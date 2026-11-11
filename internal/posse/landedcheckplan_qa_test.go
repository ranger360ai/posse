//go:build posse_arm2

package posse

// QA pins for scripts/posse-landed-check's PREREQUISITE DERIVATION (ADR 0088
// D1/D6, ranger-base-r3pqf).
//
// WHAT IS UNDER TEST, and it is one sentence: does the script run exactly the
// prerequisites of the Makefile's `test:` line, derived rather than listed,
// and nothing else? Every other property of the landed check belongs to its
// runner (ADR 0088 D1-D5, ranger-base-5fond) and is pinned there.
//
// THE LAND CHECK'S PIN SAYS WHY THIS EXISTS AT ALL
// (landcheckplan_qa_test.go, ranger-base-n2yaw): nothing in the suite runs
// these scripts — `make test` does not invoke them, shell-syntax.sh only
// parses them, and `make scripts-check`'s censuses read scripts/ as text — so
// the one part of each designed to outlive its author, the derivation, was
// witnessed by by-hand `--plan` readings on its own close and never again. A
// derivation that silently returned the WRONG list (as against none, which
// the script's own guard catches) would be found by a landing that went red
// unnoticed, which is the incident this whole ADR answers, one level in.
//
// OVER A PLANTED MAKEFILE AND NEVER OVER THIS TREE, deliberately. A pin that
// read this repo's own `test:` line would assert a door list every unrelated
// landing can move, and it would be a tree-wide pin living in a package
// nobody runs whole (ranger-base-rulbl's class) — reachable only behind a
// door. The subject here is the DERIVATION, so each fixture spells its own
// Makefile and the script is the only thing under test. The script's copy is
// the recorded-green tree's and the cwd is the tree being judged (D1), which
// is exactly the shape these runs have: an absolute path to this checkout's
// script, `cmd.Dir` the fixture.
//
// `--plan` FOR THE DERIVATION ARMS AND A REAL RUN FOR THE REST, which is what
// keeps this in an arm rather than behind a door: plan mode runs no make goal
// at all, and the fixtures whose doors DO run spell them `@:`. Nothing here
// compiles anything. Two arms are the witnesses that make it mean something —
// ldcPlanRunsNothing proves plan mode ran no recipe, and
// ldcSuiteArmsAreNotRun proves a REAL run leaves the `test:` recipe, which is
// about fifteen minutes and three suite slots on this repo, untouched.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// ldcScript resolves this checkout's landed check, skipping when the checkout
// does not carry it (a tarball, a worktree pruned for a build) — the same
// shape landcheckplan_qa_test.go uses, and a climb to ONE NAMED FILE rather
// than an enumeration, which is what keeps this pin out of the tree-wide
// class.
func ldcScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../scripts/posse-landed-check")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("landed check not present: %v", err)
	}
	return p
}

// ldcArmWitness is the file the fixtures' `test:` RECIPE writes. It must never
// exist after a run: the recipe is the suite arms, which this check does not
// run (D6).
const ldcArmWitness = "suite-arm-ran"

// ldcMakefile is the baseline fixture: three doors in declared order, a
// `test:` line naming them, and a `test:` RECIPE that leaves a witness behind
// if it is ever run. Doors are `@:` so a real run costs one make.
//
// Tabs matter — a Makefile recipe line is TAB-indented — so every recipe here
// is spelled with an explicit \t and never with Go source indentation.
const ldcMakefile = "test: door-a door-b door-c\n" +
	"\t@touch " + ldcArmWitness + "\n" +
	"door-a:\n\t@:\n" +
	"door-b:\n\t@:\n" +
	"door-c:\n\t@:\n"

// ldcTree plants a tree holding one Makefile and returns its root.
func ldcTree(t *testing.T, makefile string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	return root
}

// ldcRun runs the check over a planted tree and returns its combined output
// and exit code.
func ldcRun(t *testing.T, tree string, args ...string) (string, int) {
	t.Helper()
	return ldcRunEnv(t, tree, nil, args...)
}

// ldcRunEnv is ldcRun with extra environment, which is its own arm's subject:
// make reads options out of the environment, so the caller's environment is an
// input to this check whether or not anyone meant it to be.
func ldcRunEnv(t *testing.T, tree string, extra []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(ldcScript(t), args...)
	cmd.Dir = tree
	if len(extra) > 0 {
		cmd.Env = append(os.Environ(), extra...)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run the landed check over %s: %v\n%s", tree, err, out)
	}
	return string(out), code
}

var ldcStepLine = regexp.MustCompile(`^posse-landed-check: step ([0-9]+): (.*)$`)

// ldcPlan is the planned step list: the payload of each `step N:` line, with
// the numbering checked. A plan that failed is a fatal here rather than an
// empty list — an arm comparing step lists must not read "no steps" out of a
// red.
func ldcPlan(t *testing.T, tree string) []string {
	t.Helper()
	out, code := ldcRun(t, tree, "--plan")
	if code != 0 {
		t.Fatalf("--plan over %s exited %d, want 0\n%s", tree, code, out)
	}
	var steps []string
	for _, line := range strings.Split(out, "\n") {
		m := ldcStepLine.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("step number %q does not parse: %v", m[1], err)
		}
		if n != len(steps)+1 {
			t.Errorf("step lines are numbered %d after %d steps — the numbering is not the order\n%s", n, len(steps), out)
		}
		steps = append(steps, m[2])
	}
	if len(steps) == 0 {
		t.Fatalf("--plan over %s printed no step line at all\n%s", tree, out)
	}
	return steps
}

// TestQALandedCheckPlansMakeOverTheDerivedPrerequisites is the baseline: one
// step, `make` over the `test:` prerequisites in the order the Makefile
// declares them, and the suite arms absent from it.
func TestQALandedCheckPlansMakeOverTheDerivedPrerequisites(t *testing.T) {
	t.Parallel()
	steps := ldcPlan(t, ldcTree(t, ldcMakefile))
	want := "make --no-print-directory door-a door-b door-c"
	if len(steps) != 1 || steps[0] != want {
		t.Errorf("planned %q, want exactly [%q] — the derived prerequisites, in declared order, in one make", steps, want)
	}
	if strings.Contains(steps[0], "test") {
		t.Errorf("the plan names the `test` goal itself (%q) — the suite arms are the recipe of that goal and cost three suite slots (ADR 0088 D6)", steps[0])
	}
}

// TestQALandedCheckDerivationExpandsVariables is the first thing a scan of the
// Makefile's text would get wrong. make is asked, so a prerequisite named
// through a variable arrives expanded.
func TestQALandedCheckDerivationExpandsVariables(t *testing.T) {
	t.Parallel()
	mk := "DOORS := door-a door-b\n" +
		"test: $(DOORS) door-c\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@:\n" +
		"door-b:\n\t@:\n" +
		"door-c:\n\t@:\n"
	steps := ldcPlan(t, ldcTree(t, mk))
	want := "make --no-print-directory door-a door-b door-c"
	if len(steps) != 1 || steps[0] != want {
		t.Errorf("planned %q over a Makefile naming two doors through $(DOORS), want [%q] — a derivation reading the line's TEXT would plan `$(DOORS)`, which is not a goal", steps, want)
	}
}

// TestQALandedCheckDerivationUnionsASecondTestLine is the second: make unions
// the prerequisites of every rule line for a target, so a door added by a
// later `test:` line is a door, and a scan of the first such line alone would
// never run it.
func TestQALandedCheckDerivationUnionsASecondTestLine(t *testing.T) {
	t.Parallel()
	mk := "test: door-a\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"test: door-b\n" +
		"door-a:\n\t@:\n" +
		"door-b:\n\t@:\n"
	steps := ldcPlan(t, ldcTree(t, mk))
	if len(steps) != 1 {
		t.Fatalf("planned %d steps, want 1: %q", len(steps), steps)
	}
	for _, door := range []string{"door-a", "door-b"} {
		if !strings.Contains(steps[0], door) {
			t.Errorf("the plan %q does not name %s — make unions the prerequisites of both `test:` lines, and this one is a door that would never run", steps[0], door)
		}
	}
}

// TestQALandedCheckFindsNoPrerequisitesIsARed is the guard the script's header
// argues for: a derivation that silently returns nothing restores the exact
// hole the check exists to close.
func TestQALandedCheckFindsNoPrerequisitesIsARed(t *testing.T) {
	t.Parallel()
	mk := "test:\n\t@touch " + ldcArmWitness + "\n"
	out, code := ldcRun(t, ldcTree(t, mk), "--plan")
	if code == 0 {
		t.Errorf("a `test:` line with no prerequisites planned green (exit 0) — a derivation that finds nothing must red, not skip\n%s", out)
	}
	if !strings.Contains(out, "no prerequisites") {
		t.Errorf("the red does not say what was not found — its tail is what reaches the bead:\n%s", out)
	}
}

// TestQALandedCheckWithNoMakefileIsARed: the contract's cwd IS the tree to
// judge, so a cwd that holds no Makefile is this script pointed at the wrong
// place, and it says so rather than deriving nothing.
func TestQALandedCheckWithNoMakefileIsARed(t *testing.T) {
	t.Parallel()
	out, code := ldcRun(t, t.TempDir(), "--plan")
	if code == 0 {
		t.Errorf("a tree with no Makefile planned green (exit 0)\n%s", out)
	}
	if !strings.Contains(out, "no Makefile here") {
		t.Errorf("the red does not name the missing Makefile:\n%s", out)
	}
}

// TestQALandedCheckTakesNoArguments: the contract is no arguments (D1), and an
// argument this script does not know is a caller that thinks it is talking to
// something else. Exit 2, and name the argument.
func TestQALandedCheckTakesNoArguments(t *testing.T) {
	t.Parallel()
	out, code := ldcRun(t, ldcTree(t, ldcMakefile), "--tree-check")
	if code != 2 {
		t.Errorf("an unknown argument exited %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "--tree-check") {
		t.Errorf("the refusal does not name the argument it refused:\n%s", out)
	}
}

// TestQALandedCheckPlanRunsNothing is the witness for every arm above: plan
// mode prints the make line and does not run it. The fixture's doors each
// leave a file and the first one FAILS, so a plan that ran its step would be
// both red and visible.
func TestQALandedCheckPlanRunsNothing(t *testing.T) {
	t.Parallel()
	mk := "test: door-a\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@touch door-a-ran; exit 1\n"
	tree := ldcTree(t, mk)
	out, code := ldcRun(t, tree, "--plan")
	if code != 0 {
		t.Fatalf("--plan over a fixture whose only door fails exited %d, want 0 — plan mode runs nothing\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(tree, "door-a-ran")); err == nil {
		t.Errorf("--plan ran the door: %s exists", "door-a-ran")
	}
	// And the same fixture WITHOUT --plan is red, which is what makes the
	// green above a fact about plan mode rather than about the fixture.
	out, code = ldcRun(t, tree)
	if code == 0 {
		t.Fatalf("a real run over a fixture whose only door fails exited 0\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(tree, "door-a-ran")); err != nil {
		t.Errorf("a real run did not run the door (%v) — then the red above is not the door's", err)
	}
}

// TestQALandedCheckRunsThePrerequisitesAndNotTheSuiteArms is D6 itself: the
// prerequisites run, the `test:` RECIPE does not. On this repo that recipe is
// about fifteen minutes and three suite slots per landing, and the check holds
// no slot and no lock.
func TestQALandedCheckRunsThePrerequisitesAndNotTheSuiteArms(t *testing.T) {
	t.Parallel()
	mk := "test: door-a door-b\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@touch door-a-ran\n" +
		"door-b:\n\t@touch door-b-ran\n"
	tree := ldcTree(t, mk)
	out, code := ldcRun(t, tree)
	if code != 0 {
		t.Fatalf("a real run over a green fixture exited %d, want 0\n%s", code, out)
	}
	for _, ran := range []string{"door-a-ran", "door-b-ran"} {
		if _, err := os.Stat(filepath.Join(tree, ran)); err != nil {
			t.Errorf("the run did not run a prerequisite: %s missing (%v)", ran, err)
		}
	}
	if _, err := os.Stat(filepath.Join(tree, ldcArmWitness)); err == nil {
		t.Errorf("the run executed the `test:` recipe (%s exists) — those are the suite arms, which ADR 0088 D6 does not run", ldcArmWitness)
	}
}

// TestQALandedCheckStopsAtTheFirstRedAndTheTailNamesIt: the last lines of
// combined output are the cause (D1), and a door after the failing one must
// not run — the tail would then be a green door's output.
func TestQALandedCheckStopsAtTheFirstRedAndTheTailNamesIt(t *testing.T) {
	t.Parallel()
	mk := "test: door-a door-b\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@echo door-a is unhappy; exit 1\n" +
		"door-b:\n\t@touch door-b-ran\n"
	tree := ldcTree(t, mk)
	out, code := ldcRun(t, tree)
	if code == 0 {
		t.Fatalf("a run over a fixture whose first door fails exited 0\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(tree, "door-b-ran")); err == nil {
		t.Errorf("the run carried on past the failing door — door-b ran, so the tail is a GREEN door's output")
	}
	var tail []string
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			tail = append(tail, line)
		}
	}
	if len(tail) == 0 {
		t.Fatalf("a red printed nothing at all")
	}
	last := tail[len(tail)-1]
	if !strings.Contains(last, "door-a") {
		t.Errorf("the last non-empty line is %q — it does not name the door that died, and the last lines are the whole of what reaches the bead (ADR 0088 D1/D5)\nfull output:\n%s", last, out)
	}
}

// TestQALandedCheckIgnoresAnInheritedMakeEnvironment: make takes options out
// of MAKEFLAGS and GNUMAKEFLAGS, so a caller that is itself a make — or any
// shell that exported one — would otherwise decide what this check does. An
// inherited `-n` is the bad one: every door prints and none runs, and the
// check exits 0, which is a green over an empty room.
func TestQALandedCheckIgnoresAnInheritedMakeEnvironment(t *testing.T) {
	t.Parallel()
	mk := "test: door-a\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@touch door-a-ran\n"
	for _, env := range []string{"MAKEFLAGS=-n", "GNUMAKEFLAGS=-n", "MAKEFLAGS=--dry-run"} {
		tree := ldcTree(t, mk)
		out, code := ldcRunEnv(t, tree, []string{env})
		if code != 0 {
			t.Errorf("a run under %s exited %d, want 0\n%s", env, code, out)
			continue
		}
		if _, err := os.Stat(filepath.Join(tree, "door-a-ran")); err != nil {
			t.Errorf("a run under %s ran no door (%v) and still exited 0 — the environment turned the check into a green over an empty room", env, err)
		}
	}
}

// TestQALandedCheckIgnoresAnInheritedKeepGoing is the other half: `-k` carries
// on past the first red, so the tail that reaches the bead would be a GREEN
// door's output instead of the cause (ADR 0088 D1/D5).
func TestQALandedCheckIgnoresAnInheritedKeepGoing(t *testing.T) {
	t.Parallel()
	mk := "test: door-a door-b\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@echo door-a is unhappy; exit 1\n" +
		"door-b:\n\t@touch door-b-ran\n"
	tree := ldcTree(t, mk)
	out, code := ldcRunEnv(t, tree, []string{"MAKEFLAGS=-k"})
	if code == 0 {
		t.Fatalf("a run under MAKEFLAGS=-k over a fixture whose first door fails exited 0\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(tree, "door-b-ran")); err == nil {
		t.Errorf("MAKEFLAGS=-k carried the run past the failing door — the tail is then a green door's output")
	}
}

// TestQALandedCheckTailNamesTheDoorUnderAParentMake: MAKELEVEL set makes make
// announce the directory it enters and, last of all, the one it leaves — so
// the last line of output is a PATH where the door's name belongs. This pin
// red exactly that way when the suite first ran it under `make test-arm2`,
// which is the only reason the scrub and `--no-print-directory` are in the
// script.
func TestQALandedCheckTailNamesTheDoorUnderAParentMake(t *testing.T) {
	t.Parallel()
	mk := "test: door-a\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@echo door-a is unhappy; exit 1\n"
	out, code := ldcRunEnv(t, ldcTree(t, mk), []string{"MAKELEVEL=1"})
	if code == 0 {
		t.Fatalf("a run under MAKELEVEL=1 over a failing fixture exited 0\n%s", out)
	}
	var tail []string
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			tail = append(tail, line)
		}
	}
	last := tail[len(tail)-1]
	if strings.Contains(last, "irectory") || !strings.Contains(last, "door-a") {
		t.Errorf("under MAKELEVEL=1 the last non-empty line is %q — it must name the door that died, not the directory make left\nfull output:\n%s", last, out)
	}
}

// TestQALandedCheckReadsADoubleColonTestRule: make prints a double-colon rule
// as `test:: <prerequisites>`, and a scan that stripped one colon would plan
// the goal `:`. That reds loudly rather than quietly, which is why this arm
// asserts the planned goals and not merely the exit code.
func TestQALandedCheckReadsADoubleColonTestRule(t *testing.T) {
	t.Parallel()
	mk := "test:: door-a door-b\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@:\n" +
		"door-b:\n\t@:\n"
	steps := ldcPlan(t, ldcTree(t, mk))
	want := "make --no-print-directory door-a door-b"
	if len(steps) != 1 || steps[0] != want {
		t.Errorf("planned %q over a `test::` rule, want [%q] — a derivation stripping one colon plans the goal `:`", steps, want)
	}
}
