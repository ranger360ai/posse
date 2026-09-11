package treepins

// QA pins for ranger-base-nhc23 — the on-demand `-race` arm is a committed
// target, and its census can still say no.
//
// THE DEFECT THESE HOLD. ranger-base-d0xvw decided a narrow `-race` arm over
// internal/posse's concurrency-carrying tests and wrote the recipe as ONE
// `go test ./internal/posse -run <106 names>`. internal/posse had become three
// build-tag binaries the day before (ranger-base-qp1hm), and the candidate set
// straddles all three: that command compiled the default build only, 86 of the
// 106 names matched nothing, and it exited 0. The arm was priced, decided and
// recorded over 19% of itself. It is rot #4 in armtags_qa_test.go's header,
// verbatim — "a `go test -run` that matches no test exits 0" — and it had
// already eaten ranger-base-y3x6n's recorded repro, which still prints
// `ok … [no tests to run]`.
//
// scripts/test-race.sh is the recipe as a program: it reads the candidate test
// names out of the candidate FILES every run, and censuses all three arms
// before it spends a minute, so a name compiled into no arm is a loud failure
// in seconds rather than a silent skip.
//
// WHY IT NEEDS PINNING AT ALL. Nothing runs this script: it is on-demand by
// design (~11.4 minutes of wall) and no gate can afford it. That is exactly
// the state in which an instrument goes quiet without anybody noticing —
// armtags' rot #2 one level out — so what the suite holds is the two halves
// that can rot while the arm sits unused:
//
//	ARM 1  the target still exists and still runs the script. Renamed or
//	       deleted, the arm becomes a command nobody can find.
//	ARM 2  the census still reaches every candidate test, in the tree as it
//	       stands. A retagged file, a fourth tag, a renamed test file — each
//	       takes tests out of the arm, and this is the only thing that says so.
//	ARM 3  the census can FAIL. Arm 2 over a census that has gone blind is a
//	       green that measures nothing, which is the defect one level in — the
//	       same shape as the arm it is about.

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const raceArmScript = "scripts/test-race.sh"

// The floor is set below today's count and not at it: the number moves with
// every test added to a candidate file, and a floor that tracked it exactly
// would red on ordinary work. 106 on 2026-09-07, 107 on 2026-09-10.
const raceArmFloor = 90

var (
	raceArmTotalRe = regexp.MustCompile(`candidate set: (\d+) tests across (\d+) files`)
	raceArmPerArm  = regexp.MustCompile(`arm (\d) \([^)]*\)\s+(\d+)`)
	raceArmAllRe   = regexp.MustCompile(`every one of the (\d+) reachable in some arm`)
)

// raceArmCensus runs the script's census and answers its combined output plus
// the error. The census is the cheap half — it builds the three arms without
// `-race` and asks each binary for its test list — so the suite can afford it
// where it cannot afford the arm itself.
func raceArmCensus(t *testing.T, files string) (string, error) {
	t.Helper()
	cmd := exec.Command(raceArmScript, "--census")
	if files != "" {
		cmd.Env = append(os.Environ(), "POSSE_RACE_FILES="+files)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ARM 1 — `make test-race` is the way in, and it runs the script.
func TestQATheRaceArmHasATargetThatRunsTheScript(t *testing.T) {
	t.Parallel()
	mk := makefileText(t)
	recipe := makeRecipe(mk, "test-race")
	if len(recipe) == 0 {
		t.Fatalf("the Makefile has no `test-race` target — the on-demand `-race` arm is a script nobody can find, which is how it was prose in a notes file (ranger-base-nhc23)")
	}
	var runs bool
	for _, line := range recipe {
		if !isComment(line) && strings.Contains(line, raceArmScript) {
			runs = true
		}
	}
	if !runs {
		t.Errorf("`make test-race` does not run %s:\n%s", raceArmScript, strings.Join(recipe, "\n"))
	}
	phonyLine, phony := mkPrereqs(t, mk, ".PHONY")
	if !mkRuns(phony, "test-race") {
		t.Errorf(".PHONY does not name test-race — a file of that name in the tree would silence the target: %q", phonyLine)
	}
	info, err := os.Stat(raceArmScript)
	if err != nil {
		t.Fatalf("%s: %v", raceArmScript, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("%s is not executable (%v) — the recipe runs it by path", raceArmScript, info.Mode().Perm())
	}
}

// ARM 2 — every test in the candidate set is compiled into some arm, in the
// tree as it stands today. This is the arm that reds on a retag.
func TestQAEveryRaceArmCandidateTestIsCompiledIntoSomeArm(t *testing.T) {
	t.Parallel()
	out, err := raceArmCensus(t, "")
	if err != nil {
		t.Fatalf("%s --census: %v\n%s", raceArmScript, err, out)
	}

	m := raceArmTotalRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("the census printed no candidate-set line, so nothing below reads anything:\n%s", out)
	}
	total, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	if total < raceArmFloor {
		t.Errorf("the candidate set is %d tests, under the floor of %d — it was 106 when ranger-base-d0xvw priced it and 107 on 2026-09-10, so either a candidate file lost most of its tests or the reader that finds them has gone blind:\n%s", total, raceArmFloor, out)
	}

	perArm := raceArmPerArm.FindAllStringSubmatch(out, -1)
	if len(perArm) != 3 {
		t.Fatalf("the census named %d arms, want 3:\n%s", len(perArm), out)
	}
	for _, a := range perArm {
		n, err := strconv.Atoi(a[2])
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Errorf("arm %s compiles NONE of the candidate set — a whole partition of the arm would run nothing and exit 0:\n%s", a[1], out)
		}
	}

	// The census's own verdict line, which is the one the script exits 0 on.
	if a := raceArmAllRe.FindStringSubmatch(out); a == nil || a[1] != m[1] {
		t.Errorf("the census did not report all %d candidate tests reachable:\n%s", total, out)
	}
	t.Logf("candidate set %d tests: arm1=%s arm2=%s arm3=%s", total, perArm[0][2], perArm[1][2], perArm[2][2])
}

// ARM 3 — the control, in both shapes the census has to be able to refuse. A
// census that cannot say no leaves arm 2 green over an arm that runs nothing,
// which is the defect this whole file is about, one level in.
func TestQATheRaceArmCensusRefusesTheWrongShapes(t *testing.T) {
	t.Parallel()

	// A test file whose tests are in another package entirely: every name is
	// compiled into no arm of internal/posse. This is the planted rot #4.
	out, err := raceArmCensus(t, "internal/treepins/root_test.go")
	if err == nil {
		t.Errorf("the census passed over a candidate file whose tests are in NO arm — it cannot detect the silent skip it exists for:\n%s", out)
	}
	if !strings.Contains(out, "compiled into NO arm") || !strings.Contains(out, "TestTreePinsRunFromModuleRoot") {
		t.Errorf("the census refused, and its message neither says why nor names the unreachable test:\n%s", out)
	}

	// And a candidate file that is not in the tree, which is what a rename
	// looks like from here. It must not be read as "no tests, nothing to do".
	out, err = raceArmCensus(t, "internal/posse/nosuch_qa_test.go")
	if err == nil {
		t.Errorf("the census passed over a candidate file that does not exist:\n%s", out)
	}
	if !strings.Contains(out, "not in the tree") {
		t.Errorf("the census refused a missing candidate file without saying so:\n%s", out)
	}
}
