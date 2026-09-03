package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The gate `make test` depends on had no pin of its own: `go test ./...`
// reported cmd/testparallel as `[no test files]`, so a refactor that made
// `check` always exit 0 would have taken the ceiling's only deterministic
// guard with it and nothing would have said so (found verifying
// ranger-base-pj87l under ranger-base-169ft).
//
// scripts/test-times.sh carries `--self-test` for exactly this reason and
// the Makefile says why beside it: "Prove the reporter still reports". This
// is the same argument applied to the other half of the ceiling story.
//
// Every arm below is PAIRED with a control that must come out the other
// way, because a gate that exits 1 on everything and a gate that exits 0 on
// everything both satisfy a one-sided assertion. The fixtures only have to
// PARSE — the tool is a go/ast read and never builds or type-checks what it
// is pointed at — so each one is a few lines and the whole file costs one
// `go build` of the tool.

var (
	buildOnce sync.Once
	toolBin   string
	buildErr  error
)

// tool builds cmd/testparallel once per test binary and answers the path.
// `go test` runs a test binary with its own package directory as the working
// directory, so "." is this command.
func tool(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "testparallel-selftest")
		if err != nil {
			buildErr = err
			return
		}
		toolBin = filepath.Join(dir, "testparallel")
		out, err := exec.Command("go", "build", "-o", toolBin, ".").CombinedOutput()
		if err != nil {
			buildErr = err
			toolBin = string(out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building the tool under test: %v\n%s", buildErr, toolBin)
	}
	return toolBin
}

// pkg writes one fixture package — file name to source — and answers its dir.
func pkg(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// run answers the tool's exit code and its two streams joined, which is what
// a reader of the gate's failure actually sees.
func run(t *testing.T, dir string, verb ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(tool(t), append([]string{dir}, verb...)...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running the tool: %v\n%s", err, out)
	}
	return code, string(out)
}

const fixtureHeader = "package p\n\nimport \"testing\"\n\n"

// TestCheckPassesAMarkedPackageAndFailsTheSameOneUnmarked is the pair that
// decides whether the gate is a gate at all: the ONLY difference between the
// two fixtures is the `t.Parallel()` line.
func TestCheckPassesAMarkedPackageAndFailsTheSameOneUnmarked(t *testing.T) {
	t.Parallel()
	marked := pkg(t, map[string]string{"a_test.go": fixtureHeader +
		"func TestOne(t *testing.T) {\n\tt.Parallel()\n\t_ = 1\n}\n"})
	if code, out := run(t, marked, "check"); code != 0 || !strings.Contains(out, "clean") {
		t.Errorf("a fully marked package must pass: exit %d\n%s", code, out)
	}
	unmarked := pkg(t, map[string]string{"a_test.go": fixtureHeader +
		"func TestOne(t *testing.T) {\n\t_ = 1\n}\n"})
	code, out := run(t, unmarked, "check")
	if code != 1 {
		t.Fatalf("an eligible test with no t.Parallel must fail the gate: exit %d\n%s", code, out)
	}
	for _, want := range []string{"TestOne", "a_test.go", "can take t.Parallel and do not"} {
		if !strings.Contains(out, want) {
			t.Errorf("the failure must name %q so the reader can act on it:\n%s", want, out)
		}
	}
}

// TestCheckDoesNotDemandParallelOfATestThatWritesTheEnvironment is the
// control for the arm above: the red there must come from ELIGIBILITY and
// not from "carries no t.Parallel", which every serial test would trip.
// t.Parallel panics in a test that has called t.Setenv, so demanding it here
// would be the tool asking for a panic.
func TestCheckDoesNotDemandParallelOfATestThatWritesTheEnvironment(t *testing.T) {
	t.Parallel()
	direct := pkg(t, map[string]string{"a_test.go": fixtureHeader +
		"func TestOne(t *testing.T) {\n\tt.Setenv(\"K\", \"v\")\n}\n"})
	if code, out := run(t, direct, "check"); code != 0 {
		t.Errorf("a t.Setenv test is not eligible and must not be demanded of: exit %d\n%s", code, out)
	}
	// And the taint has to travel: the same write behind a helper, which is
	// the shape 198 roots in internal/posse actually take.
	viaHelper := pkg(t, map[string]string{"a_test.go": fixtureHeader +
		"func setup(t *testing.T) {\n\tt.Setenv(\"K\", \"v\")\n}\n\n" +
		"func TestOne(t *testing.T) {\n\tsetup(t)\n}\n"})
	if code, out := run(t, viaHelper, "check"); code != 0 {
		t.Errorf("the env taint must propagate through a helper: exit %d\n%s", code, out)
	}
	// The control for THAT: the identical shape with the one env write
	// removed is eligible again, so the pass above is the taint and not the
	// helper call swallowing the test.
	noWrite := pkg(t, map[string]string{"a_test.go": fixtureHeader +
		"func setup(t *testing.T) {\n\t_ = t\n}\n\n" +
		"func TestOne(t *testing.T) {\n\tsetup(t)\n}\n"})
	if code, out := run(t, noWrite, "check"); code != 1 || !strings.Contains(out, "TestOne") {
		t.Errorf("without the env write the same test is eligible again: exit %d\n%s", code, out)
	}
}

// TestCheckRefusesAParallelTestReadingTheBinaryWideFakeDir pins the filter
// added for ranger-base-1y3dp: fakeDir() with no argument answers the test
// BINARY's own directory in the parent since ADR 0047 D1, so a parallel test
// reading a call log through it reads every other test's calls. That escape
// read as "the thing under test did not happen" twice before it was named.
func TestCheckRefusesAParallelTestReadingTheBinaryWideFakeDir(t *testing.T) {
	t.Parallel()
	bare := pkg(t, map[string]string{"a_test.go": fixtureHeader +
		"func fakeDir() string { return \"\" }\n\n" +
		"func TestOne(t *testing.T) {\n\tt.Parallel()\n\t_ = fakeDir()\n}\n"})
	code, out := run(t, bare, "check")
	if code != 1 {
		t.Fatalf("a parallel test reading the binary-wide fakeDir() must fail the gate: exit %d\n%s", code, out)
	}
	for _, want := range []string{"TestOne", "fakeDirOf(t)", "binary-wide fakeDir()"} {
		if !strings.Contains(out, want) {
			t.Errorf("the failure must name %q:\n%s", want, out)
		}
	}
	// Through a helper — the shape closeddirty_test.go had, where the read
	// was inside bdCalls's caller and not in the test's own body.
	viaHelper := pkg(t, map[string]string{"a_test.go": fixtureHeader +
		"func fakeDir() string { return \"\" }\n\n" +
		"func calls() string { return fakeDir() }\n\n" +
		"func TestOne(t *testing.T) {\n\tt.Parallel()\n\t_ = calls()\n}\n"})
	if code, out := run(t, viaHelper, "check"); code != 1 || !strings.Contains(out, "TestOne") {
		t.Errorf("the fakeDir taint must propagate through a helper: exit %d\n%s", code, out)
	}
	// Two controls, both of which must come out the other way.
	//
	// One: fakeDirOf(t) is the per-test directory and is the whole point of
	// the distinction — the same test reading THAT is clean.
	perTest := pkg(t, map[string]string{"a_test.go": fixtureHeader +
		"func fakeDirOf(t *testing.T) string { return \"\" }\n\n" +
		"func TestOne(t *testing.T) {\n\tt.Parallel()\n\t_ = fakeDirOf(t)\n}\n"})
	if code, out := run(t, perTest, "check"); code != 0 {
		t.Errorf("fakeDirOf(t) is the per-test read and must be clean: exit %d\n%s", code, out)
	}
	// Two: the filter keys on the CALL SHAPE, not the name — fakeDir with an
	// argument is a different function and the tool says so in as many words
	// (`len(x.Args) == 0`). A pin on the name alone would stay green if that
	// condition were dropped.
	withArg := pkg(t, map[string]string{"a_test.go": fixtureHeader +
		"func fakeDir(s string) string { return s }\n\n" +
		"func TestOne(t *testing.T) {\n\tt.Parallel()\n\t_ = fakeDir(\"x\")\n}\n"})
	if code, out := run(t, withArg, "check"); code != 0 {
		t.Errorf("fakeDir(arg) is not the bare binary-wide read: exit %d\n%s", code, out)
	}
}

// TestCheckHoldsAWrittenPackageVarSerialAndLetsTheThreeExemptionsThrough
// pins the exemption list by name. Each of the three buys a large slice of
// the marked set — fakeDirs alone would taint 663 tests — so a fourth name
// added here without an argument beside it is how the gate quietly stops
// covering the thing it exists for.
func TestCheckHoldsAWrittenPackageVarSerialAndLetsTheThreeExemptionsThrough(t *testing.T) {
	t.Parallel()
	written := pkg(t, map[string]string{"a_test.go": fixtureHeader + "import \"sync\"\n\n" +
		"var other sync.Map\n\n" +
		"func TestOne(t *testing.T) {\n\tother.Store(1, 2)\n}\n"})
	if code, out := run(t, written, "check"); code != 0 {
		t.Errorf("a test naming a written package var is not eligible: exit %d\n%s", code, out)
	}
	for _, name := range []string{"fakeDirs", "operatorHome", "hermeticRun"} {
		ex := pkg(t, map[string]string{"a_test.go": fixtureHeader + "import \"sync\"\n\n" +
			"var " + name + " sync.Map\n\n" +
			"func TestOne(t *testing.T) {\n\t" + name + ".Store(1, 2)\n}\n"})
		code, out := run(t, ex, "check")
		if code != 1 || !strings.Contains(out, "TestOne") {
			t.Errorf("%s is exempt, so a test naming it stays eligible and must be demanded of: exit %d\n%s", name, code, out)
		}
	}
}

// TestEligibleListsTheSetToMarkAndLeavesTheTaintedOut pins the other verb
// `make verify-parallel`'s reader reaches for: the gate says WHICH tests are
// wrong and `eligible` says which set to mark, and the two have to agree.
func TestEligibleListsTheSetToMarkAndLeavesTheTaintedOut(t *testing.T) {
	t.Parallel()
	dir := pkg(t, map[string]string{"a_test.go": fixtureHeader +
		"func TestClean(t *testing.T) {\n\tt.Parallel()\n}\n\n" +
		"func TestTainted(t *testing.T) {\n\tt.Setenv(\"K\", \"v\")\n}\n"})
	code, out := run(t, dir, "eligible")
	if code != 0 {
		t.Fatalf("eligible is a listing, not a gate: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "TestClean") {
		t.Errorf("eligible must list the env-clean test:\n%s", out)
	}
	if strings.Contains(out, "TestTainted") {
		t.Errorf("eligible must not list a t.Setenv test — t.Parallel panics in one:\n%s", out)
	}
}

// A parallelOK reason covers the vars it names as whole identifiers, and
// nothing that merely contains one of them (ranger-base-acvq3). The scan pin
// over internal/posse (testparallelclearancescope_qa_test.go at the repo
// root) cannot see this edge — no cleared var there is a prefix of another —
// so it is held here, against the function itself.
func TestClearanceCoversMatchesIdentifiersNotSubstrings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		reason string
		vars   []string
		other  bool
		want   bool
	}{
		{"reads blindT", []string{"blindT"}, false, true},
		{"reads blindT and pulseNow", []string{"blindT", "pulseNow"}, false, true},
		{"reads blindT", []string{"blindT", "costProviders"}, false, false},
		{"reads blindTX", []string{"blindT"}, false, false}, // substring, not the identifier
		{"reads blindT", []string{"blind"}, false, false},
		{"reads blindT", []string{"blindT"}, true, false}, // an env/fakeDir/serial hold is not a var
		{"", []string{"blindT"}, false, false},
		{"reads blindT", nil, false, false},
	}
	for _, c := range cases {
		if got := clearanceCovers(c.reason, c.vars, c.other); got != c.want {
			t.Errorf("clearanceCovers(%q, %v, %v) = %v, want %v", c.reason, c.vars, c.other, got, c.want)
		}
	}
}
