package posse

// QA pin for ranger-base-acvq3 finding 1, found verifying ranger-base-btdvw's
// close under ranger-base-4m5n6.
//
// THE DEFECT, as it shipped. cmd/testparallel's gate was
//
//	case !eligible && g.hasParallel && parallelOK[g.name] == "":
//
// so a clearance was keyed on the test NAME alone. The reason
// recorded beside it — "reads blindT", "calls sandboxApplyRefusal" — is never
// compared with the var that actually holds the test back. A test cleared
// once is therefore waived for every FUTURE shared-state reason, including a
// genuinely racy one, which is the defect btdvw exists to stop. `extra`
// printed the RECORDED reason rather than the measured one, so the drift was
// invisible there too.
//
// The 40 currently cleared tests are the blind spot. None of them is racy
// today — the clearances were checked var by var on ranger-base-4m5n6 and are
// factual — so nothing here is a live defect. This pin holds the gate to
// catching the NEXT one.
//
// THE FIX (ranger-base-acvq3): the gate compares the vars a clearance names
// against the vars the test reaches (clearanceCovers over reachedVars), so a
// cleared test that grows a write to a var its line does not name is flagged
// like an uncleared one, and `extra` prints the measured var beside the
// recorded reason. Parked with a t.Skip until then; un-skipped it FAILED on
// the finding arm alone with the control arm passing, on the pre-fix tree.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// tpcCopyPackage is a scratch copy of internal/posse for the tool to scan.
// A copy rather than the real directory because this mutates a test file, and
// a sibling seat's `make test` compiles the real tree while this runs.
func tpcCopyPackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	srcs, err := filepath.Glob("internal/posse/*.go")
	if err != nil || len(srcs) == 0 {
		t.Fatalf("no sources to copy from internal/posse: %v", err)
	}
	for _, src := range srcs {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(src)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// tpcCheck runs the gate over dir and answers its exit code and output.
func tpcCheck(t *testing.T, dir string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", "run", "./cmd/testparallel", dir, "check")
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("go run ./cmd/testparallel: %v\n%s", err, out)
	}
	return string(out), code
}

// tpcAddSharedWrite puts one write of the package-level costProviders map
// immediately after fn's t.Parallel(). AFTER, not before: inserted first it
// displaces t.Parallel and the tool then reports the test as merely missing
// it, which is a different finding wearing the same exit code.
func tpcAddSharedWrite(t *testing.T, file, fn string) {
	t.Helper()
	tpcInsertAfterParallel(t, file, fn, "\tdelete(costProviders, \"acvq3-probe\")")
}

// tpcAddEnvWrite puts one process-environment write after fn's t.Parallel():
// the flag no parallelOK reason can argue away, since t.Parallel panics over
// a Setenv at run time whatever the clearance says about a var.
func tpcAddEnvWrite(t *testing.T, file, fn string) {
	t.Helper()
	tpcInsertAfterParallel(t, file, fn, "\tos.Setenv(\"ACVQ3_PROBE\", \"1\")")
}

func tpcInsertAfterParallel(t *testing.T, file, fn, stmt string) {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	head := "func " + fn + "(t *testing.T) {\n\tt.Parallel()"
	s := string(b)
	if !strings.Contains(s, head) {
		t.Fatalf("%s: %s is not a parallel test with t.Parallel as its first line; this pin is measuring nothing", file, fn)
	}
	s = strings.Replace(s, head, head+"\n"+stmt, 1)
	if err := os.WriteFile(file, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestQAParallelClearanceDoesNotWaiveAReasonNobodyCleared(t *testing.T) {
	gate, err := os.ReadFile("cmd/testparallel/main.go")
	if err != nil {
		t.Fatal(err)
	}
	// The two names this pin rests on, checked rather than assumed: one that
	// IS cleared and one that is not. A rename that walks past either would
	// otherwise make this pin pass for the wrong reason.
	const cleared, uncleared = "TestScoreIssues", "TestQACostAdapterMissDoesNotFallBackToClaudesTable"
	if !strings.Contains(string(gate), `"`+cleared+`":`) {
		t.Fatalf("%s is no longer a parallelOK entry; pick another cleared test or this pin measures nothing", cleared)
	}
	if strings.Contains(string(gate), `"`+uncleared+`":`) {
		t.Fatalf("%s has become a parallelOK entry; the control arm below can no longer trip", uncleared)
	}

	// The substrate arm. An untouched copy must read exactly as the real
	// directory does, or neither arm below says anything about the gate.
	base := tpcCopyPackage(t)
	if out, code := tpcCheck(t, base); code != 0 || !strings.Contains(out, "clean") {
		t.Fatalf("the unmutated copy is not clean (exit %d):\n%s", code, out)
	}

	// The control arm, and it has to come first: it is what says the tool can
	// see this write at all. Without it the finding arm's silence is
	// indistinguishable from a write the filter simply does not model.
	ctl := tpcCopyPackage(t)
	tpcAddSharedWrite(t, filepath.Join(ctl, "costseamarms_qa_test.go"), uncleared)
	out, code := tpcCheck(t, ctl)
	if code == 0 || !strings.Contains(out, uncleared) {
		t.Fatalf("the gate did not catch a costProviders write in the UNCLEARED %s (exit %d); this pin cannot separate a waiver from a blind filter:\n%s", uncleared, code, out)
	}

	// The finding. The identical write, in a test whose clearance says
	// "reads blindT" — a fact that has nothing to do with costProviders.
	fnd := tpcCopyPackage(t)
	tpcAddSharedWrite(t, filepath.Join(fnd, "scorecard_test.go"), cleared)
	out, code = tpcCheck(t, fnd)
	if code == 0 || !strings.Contains(out, cleared) {
		t.Fatalf("a costProviders write in %s is waived by a clearance recorded for an unrelated var (exit %d).\nparallelOK must be keyed on the vars its reason names, not the test name alone (ranger-base-acvq3).\n%s", cleared, code, out)
	}
	// And `extra` has to say WHICH var, or the message above cannot be
	// followed: the row for the cleared test names the measured var and
	// reads UNCLEARED, not the recorded reason as if it still held.
	ex := exec.Command("go", "run", "./cmd/testparallel", fnd, "extra")
	exOut, err := ex.CombinedOutput()
	if err != nil {
		t.Fatalf("extra: %v\n%s", err, exOut)
	}
	var row string
	for _, line := range strings.Split(string(exOut), "\n") {
		if strings.Contains(line, "\t"+cleared+"\t") {
			row = line
		}
	}
	if row == "" {
		t.Fatalf("extra has no row for %s after the write:\n%s", cleared, exOut)
	}
	if !strings.Contains(row, "costProviders") || !strings.Contains(row, "UNCLEARED") {
		t.Errorf("extra still reports the recorded reason as the state of %s; want the measured var (costProviders) and UNCLEARED:\n  %s", cleared, row)
	}

	// The other half of the key: a clearance argues about VARS, so a flag of
	// another kind on a cleared test — here an environment write, which
	// t.Parallel panics over at run time — is not covered by "reads blindT"
	// either. Same cleared test, same recorded reason, different hold.
	env := tpcCopyPackage(t)
	tpcAddEnvWrite(t, filepath.Join(env, "scorecard_test.go"), cleared)
	out, code = tpcCheck(t, env)
	if code == 0 || !strings.Contains(out, cleared) {
		t.Errorf("an environment write in %s is waived by a clearance recorded for a package var (exit %d); a parallelOK reason covers the vars it names and nothing else (ranger-base-acvq3).\n%s", cleared, code, out)
	}
}
