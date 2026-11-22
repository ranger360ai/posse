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
// AND SINCE D6.7, THE TAIL (ranger-base-hzh0w/25kki). The body runs under
// `make -j4 --output-sync=target`, so D1's "the last lines of combined output
// are the cause" is no longer something make gives for free: make announces
// the dead door, then waits for the jobs already running, so the last lines
// are a GREEN door's output. The script makes the tail instead — it reads the
// failed doors out of make's own `*** [Makefile:N: <door>] Error` lines and
// re-runs the first one ALONE, last. Three arms here are that shape and
// nothing else: the tail is the dead door's and not a slow green sibling's,
// a door green when re-run alone is still RED and says so by name, and the
// doors behind the first batch never start (which is what "no -k" means once
// the body is parallel).
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

// ldcLateDoors are the doors BEHIND the first `-j4` batch in
// ldcBatchMakefile: five of them, each leaving a witness, so that "the run
// starts no door after the first red" is a fact about files on disk.
var ldcLateDoors = []string{"door-e", "door-f", "door-g", "door-h", "door-i"}

// ldcBatchMakefile is the fail-fast fixture in the shape D6.7 made relevant:
// door-a dies at once; door-b, door-c and door-d are SLOW and chatty, so they
// are still running when it dies and their output lands after make's error
// line; and ldcLateDoors sit behind the first batch of four. Under `-j4` with
// no `-k`, the first four start together and the five behind them never start
// at all.
func ldcBatchMakefile() string {
	mk := "test: door-a door-b door-c door-d " + strings.Join(ldcLateDoors, " ") + "\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@echo door-a is unhappy; exit 1\n" +
		"door-b:\n\t@sleep 1; echo " + ldcChatter + "; echo more from door-b\n" +
		"door-c:\n\t@sleep 1; echo door-c is fine\n" +
		"door-d:\n\t@sleep 1; echo door-d is fine\n"
	for _, d := range ldcLateDoors {
		mk += d + ":\n\t@touch " + d + "-ran\n"
	}
	return mk
}

// ldcChatter is what the slow GREEN door prints. It is the text that must not
// be in the tail: under `-j4 --output-sync=target` it is what make prints
// last, MEASURED 2026-11-22 over a planted Makefile and recorded in ADR
// 0088's "the body runs four doors at a time".
const ldcChatter = "door-b is fine and chatty"

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

// ldcParallelStep is step 1: the derived goals in ONE make, four doors at a
// time, output-synced, with no `-k` (ADR 0088 D6.7). The job count is spelled
// out here rather than read off the script, because a constant the pin takes
// from its subject is a constant the pin cannot hold.
func ldcParallelStep(goals ...string) string {
	return "make --no-print-directory -j4 --output-sync=target " + strings.Join(goals, " ")
}

// ldcRerunStep is step 2: the serial re-run that MAKES the tail. `--plan`
// prints it with the door as a placeholder, because which door it is depends
// on which one dies.
const ldcRerunStep = "make --no-print-directory <the first failed door>"

// ldcTail is the last non-empty lines of combined output, in order — the
// whole of what the runner flattens into a bead (landCheckSaid keeps three of
// them, ADR 0088 D1/D5). Fatal on nothing at all: an arm reading the tail must
// not read "it names the door" out of an empty run.
func ldcTail(t *testing.T, out string) []string {
	t.Helper()
	var tail []string
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			tail = append(tail, line)
		}
	}
	if len(tail) == 0 {
		t.Fatalf("the run printed nothing at all")
	}
	return tail
}

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

// TestQALandedCheckPlansMakeOverTheDerivedPrerequisites is the baseline: TWO
// steps since D6.7 — the derived `test:` prerequisites in the order the
// Makefile declares them, four at a time in one make, and the serial re-run
// that makes the tail on a red — and the suite arms absent from both.
func TestQALandedCheckPlansMakeOverTheDerivedPrerequisites(t *testing.T) {
	t.Parallel()
	steps := ldcPlan(t, ldcTree(t, ldcMakefile))
	want := []string{ldcParallelStep("door-a", "door-b", "door-c"), ldcRerunStep}
	if len(steps) != len(want) || steps[0] != want[0] || steps[1] != want[1] {
		t.Errorf("planned %q, want exactly %q — the derived prerequisites four at a time, then the first failed door alone (ADR 0088 D6.7)", steps, want)
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
	want := ldcParallelStep("door-a", "door-b", "door-c")
	if len(steps) != 2 || steps[0] != want {
		t.Errorf("planned %q over a Makefile naming two doors through $(DOORS), want step 1 %q — a derivation reading the line's TEXT would plan `$(DOORS)`, which is not a goal", steps, want)
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
	if len(steps) != 2 {
		t.Fatalf("planned %d steps, want 2 (the parallel body, then the re-run): %q", len(steps), steps)
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

// TestQALandedCheckStopsAtTheFirstRedAndTheTailNamesIt: no `-k`, so the run
// stops at the first red — which under `-j4` means it STARTS no door it had
// not already started, rather than "nothing runs after the failing one"
// (D6.7). The five doors behind the first batch of four must leave no witness,
// and the tail must still name the door that died.
func TestQALandedCheckStopsAtTheFirstRedAndTheTailNamesIt(t *testing.T) {
	t.Parallel()
	tree := ldcTree(t, ldcBatchMakefile())
	out, code := ldcRun(t, tree)
	if code == 0 {
		t.Fatalf("a run over a fixture whose first door fails exited 0\n%s", out)
	}
	for _, door := range ldcLateDoors {
		if _, err := os.Stat(filepath.Join(tree, door+"-ran")); err == nil {
			t.Errorf("%s ran — it is behind the first batch of four, so the run carried on past the red and this is `-k` by accident (ADR 0088 D6.7)\nfull output:\n%s", door, out)
		}
	}
	tail := ldcTail(t, out)
	last := tail[len(tail)-1]
	if !strings.Contains(last, "door-a") {
		t.Errorf("the last non-empty line is %q — it does not name the door that died, and the last lines are the whole of what reaches the bead (ADR 0088 D1/D5)\nfull output:\n%s", last, out)
	}
}

// TestQALandedCheckTailIsTheFailedDoorsAndNotASlowGreenSiblings is the arm
// D6.7 exists for, and the one a serial runner could not have had. Under `-j4
// --output-sync=target` make prints the dead door's error line, then `Waiting
// for unfinished jobs....`, then the output of every green door that was
// already running — so the last lines of a `-j` run are a GREEN door's
// chatter, and the bead would quote that as the cause. The check answers by
// re-running the first failed door ALONE, last.
//
// IT ASSERTS MAKE'S OWN COMPLAINT AND NOT MERELY THE DOOR'S NAME, which is
// not pedantry: this script PRINTS a line naming the door before it re-runs
// it, so a check for the name alone is satisfied by that line and the arm
// survives the removal of the whole re-run (MEASURED 2026-11-22, mutation A
// below, before this was tightened). What D6.7 promises is the door's own
// complaint plus make's line naming it — the sentence a seat would read — so
// that is what is pinned.
//
// MUTATIONS (MEASURED 2026-11-22, this file's arms, `-run LandedCheck`):
//   - dropping the `serial_rerun` call from the script's red path reds this
//     arm and `…ADoorGreenWhenReRunAloneIsStillRed`, and nothing else;
//   - putting the body back to a serial `make` with the re-run kept reds this
//     arm too, at its fixture guard: with no `-j` the slow green sibling never
//     runs at all, so the shape this arm is about does not exist.
//
// `--output-sync=target` is NOT pinned by behaviour, and the comment says so
// rather than implying it: removing it reds only the four plan arms, which
// hold the flag as text. What it buys is each door's output in one block
// instead of four interleaved, which is legibility rather than verdict.
func TestQALandedCheckTailIsTheFailedDoorsAndNotASlowGreenSiblings(t *testing.T) {
	t.Parallel()
	out, code := ldcRun(t, ldcTree(t, ldcBatchMakefile()))
	if code == 0 {
		t.Fatalf("a run over a fixture whose first door fails exited 0\n%s", out)
	}
	if !strings.Contains(out, ldcChatter) {
		t.Fatalf("the slow green door never printed %q — then this fixture does not have the shape the arm is about\n%s", ldcChatter, out)
	}
	tail := ldcTail(t, out)
	if n := len(tail); n > landCheckTailLines {
		tail = tail[n-landCheckTailLines:]
	}
	said := strings.Join(tail, "; ")
	last := tail[len(tail)-1]
	if !strings.Contains(last, "door-a") || !strings.Contains(last, "*** [") {
		t.Errorf("the last non-empty line is %q — the tail the runner flattens into a bead is %q, and it must END on make's own complaint about the door that died (`*** [Makefile:N: door-a] Error`), not on this script's narration about it (ADR 0088 D1/D6.7)\nfull output:\n%s", last, said, out)
	}
	if !strings.Contains(said, "door-a is unhappy") {
		t.Errorf("the tail the runner flattens into a bead is %q — it does not carry the dead door's OWN output, which is the cause somebody reads (ADR 0088 D1/D6.7)\nfull output:\n%s", said, out)
	}
	if strings.Contains(said, ldcChatter) {
		t.Errorf("the tail the runner flattens into a bead is %q — it carries a GREEN door's output, which is what `-j` puts last and what the serial re-run exists to displace (ADR 0088 D6.7)\nfull output:\n%s", said, out)
	}
}

// ldcSecondDead is what the SECOND dead door prints, and ldcLateChatter what
// the slow GREEN door in the same fixture prints. Both are text that must not
// be in the tail: the first is a cause but not the one D6.7 names, and the
// second is no cause at all.
const (
	ldcSecondDead  = "door-b is unhappy too"
	ldcLateChatter = "door-c is fine and chatty"
)

// ldcTwoDeadMakefile is the shape every other fixture in this file misses: TWO
// doors die in one `-j4` batch. With no `-k` make starts no door after the
// first red, but it does WAIT for the ones already running, so a second door
// that dies while it waits gets its own `*** [Makefile:N: door] Error` line.
// MEASURED 2026-11-22 over this fixture, this box, GNU Make 4.4.1: both lines
// printed, in completion order, and the script narrates `2 door(s) died
// (door-a door-b)`.
//
// door-a dies at once and door-b a second later, so the ORDER of those two
// lines is the fixture's rather than the scheduler's; door-c and door-d are
// slower still and green, which keeps the tail hazard the arm above is about
// present in this one too.
func ldcTwoDeadMakefile() string {
	return "test: door-a door-b door-c door-d\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@echo door-a is unhappy; exit 1\n" +
		"door-b:\n\t@sleep 1; echo " + ldcSecondDead + "; exit 1\n" +
		"door-c:\n\t@sleep 2; echo " + ldcLateChatter + "; echo more from door-c\n" +
		"door-d:\n\t@sleep 2; echo door-d is fine\n"
}

// ldcStepsIn is ldcPlan's reader over a run that actually RAN: the payload of
// each `step N:` line, in order. Separate from ldcPlan because that one runs
// `--plan` and fatals on a nonzero exit, and every step line worth reading
// here comes out of a run that is red on purpose.
func ldcStepsIn(t *testing.T, out string) []string {
	t.Helper()
	var steps []string
	for _, line := range strings.Split(out, "\n") {
		m := ldcStepLine.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err != nil || n != len(steps)+1 {
			t.Errorf("step lines are numbered %q after %d steps — the numbering is not the order\n%s", m[1], len(steps), out)
		}
		steps = append(steps, m[2])
	}
	if len(steps) == 0 {
		t.Fatalf("the run printed no step line at all\n%s", out)
	}
	return steps
}

// TestQALandedCheckReRunsTheFirstFailedDoorAloneWhenTwoDied grades the two
// words of D6.7 step 2 that no other fixture in this file can reach: the
// re-run takes the FIRST failed door, and it takes it ALONE.
//
// Both survived mutation until this arm existed (MEASURED 2026-11-22,
// ranger-base-5jpdo, verifying ranger-base-25kki's close): every other
// fixture here kills exactly ONE door, and over a one-door transcript
// `${doors%%$'\n'*}` and `${doors##*$'\n'}` are the same string — so a check
// that re-ran the LAST dead door read `ok` through the whole 18-arm filter.
//
// Why the two words are worth an arm rather than a comment. The re-run is
// what MAKES the tail (D6.7, D1), so which door it names is which cause
// reaches the bead; and re-running more than one door puts the second door's
// output after the first's, which is exactly the interleaving the re-run
// exists to remove, reintroduced from the other end.
//
// MUTATIONS (MEASURED 2026-11-22, `-run LandedCheck`):
//   - `first_door=${doors##*$'\n'}` — the LAST dead door — reds this arm and
//     nothing else. Before this arm it reddened nothing at all.
//   - ALONE needs a TWO-line mutant, and that is worth saying because the
//     one-line one is EQUIVALENT: `serial_rerun $doors` changes nothing,
//     since `serial_rerun`'s body is `step make … "$1"` and drops every word
//     after the first whatever the caller passes. Widening that to `"$@"` as
//     well — the pair — reds this arm and nothing else. So "alone" is held
//     here by one arm and by that helper's own `"$1"`, which is the cheaper
//     of the two and the reason the one-line mutant reads green.
func TestQALandedCheckReRunsTheFirstFailedDoorAloneWhenTwoDied(t *testing.T) {
	t.Parallel()
	out, code := ldcRun(t, ldcTree(t, ldcTwoDeadMakefile()))
	if code == 0 {
		t.Fatalf("a run over a fixture whose first two doors fail exited 0\n%s", out)
	}
	// THE FIXTURE CONTROLS. Two doors have to have died, and in this order,
	// or "the FIRST failed door" names nothing and everything below is an
	// assertion about a one-door run.
	a := strings.Index(out, "door-a] Error")
	b := strings.Index(out, "door-b] Error")
	if a < 0 || b < 0 || a > b {
		t.Fatalf("fixture: make's complaints about door-a (at %d) and door-b (at %d) are not both present in that order, so this run has no FIRST failed door to pick\n%s", a, b, out)
	}
	// And the SCRIPT saw both of them. Without this, a parser that read one
	// door out of a transcript holding two would satisfy the step assertion
	// below by accident — it would name door-a because door-a is all it found.
	if !strings.Contains(out, "2 door(s) died (door-a door-b)") {
		t.Errorf("the run does not narrate two dead doors in the order make reported them — the parser read fewer doors than the transcript holds, and `the first` is then whichever one it happened to see (ADR 0088 D6.7)\nfull output:\n%s", out)
	}

	steps := ldcStepsIn(t, out)
	if len(steps) != 2 {
		t.Fatalf("the run took %d step(s) (%q), want 2 — the parallel body, then the re-run that makes the tail\n%s", len(steps), steps, out)
	}
	if want := "make --no-print-directory door-a"; steps[1] != want {
		t.Errorf("step 2 is %q, want exactly %q — D6.7 re-runs the FIRST failed door and re-runs it ALONE: the last one would make the tail the wrong cause, and all of them would put one door's output after another's, which is the interleaving the re-run exists to remove\nfull output:\n%s", steps[1], want, out)
	}

	tail := ldcTail(t, out)
	if n := len(tail); n > landCheckTailLines {
		tail = tail[n-landCheckTailLines:]
	}
	said := strings.Join(tail, "; ")
	if last := tail[len(tail)-1]; !strings.Contains(last, "door-a") || !strings.Contains(last, "*** [") {
		t.Errorf("the last non-empty line is %q — the tail the runner flattens into a bead is %q, and with two doors dead it must still END on make's own complaint about the FIRST of them (ADR 0088 D1/D6.7)\nfull output:\n%s", last, said, out)
	}
	if !strings.Contains(said, "door-a is unhappy") {
		t.Errorf("the tail the runner flattens into a bead is %q — it does not carry the first dead door's OWN output, which is the cause somebody reads (ADR 0088 D1/D6.7)\nfull output:\n%s", said, out)
	}
	for _, displaced := range []string{ldcSecondDead, ldcLateChatter} {
		if strings.Contains(said, displaced) {
			t.Errorf("the tail the runner flattens into a bead is %q — it carries %q, which belongs to another door: the re-run is what displaces everything the `-j4` body printed after the first red, and a tail holding two doors' output is the shape D6.7 named (ADR 0088 D6.7)\nfull output:\n%s", said, displaced, out)
		}
	}
}

// TestQALandedCheckADoorGreenWhenReRunAloneIsStillRed: the one new failure
// mode the parallel body creates. A door that dies beside its siblings and
// passes alone is either not safe beside them or a flake, and both are
// findings — so the run is RED and its last line says which door (D6.7).
// Reading that serial green as the verdict would hide it.
//
// The fixture is the FLAKE half, because it is the half that can be made
// deterministic: door-a fails the first time it is ever run in a tree and
// passes afterwards, so the `-j4` body reds and the re-run alone is green,
// with no timing anywhere in it.
//
// MUTATION (MEASURED 2026-11-22): exiting with the re-run's own status — the
// one-line change that reads serial green as green — reds this arm alone.
// Dropping the re-run entirely reds it as well, and that is the right
// coupling: with no second make there is no serial verdict to misread.
func TestQALandedCheckADoorGreenWhenReRunAloneIsStillRed(t *testing.T) {
	t.Parallel()
	mk := "test: door-a door-b\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@if [ -e door-a-ran-once ]; then echo door-a is fine now; else touch door-a-ran-once; echo door-a is unhappy; exit 1; fi\n" +
		"door-b:\n\t@echo door-b is fine\n"
	out, code := ldcRun(t, ldcTree(t, mk))
	if code == 0 {
		t.Fatalf("a door that died under -j4 and passed when re-run alone was read as GREEN — that is the one failure mode this shape creates, and it is a finding (ADR 0088 D6.7)\n%s", out)
	}
	last := ldcTail(t, out)
	if l := last[len(last)-1]; !strings.Contains(l, "door-a") || !strings.Contains(l, "green alone") {
		t.Errorf("the last non-empty line is %q — it must name the door and say it was red under -j4 and green alone, because that sentence is the whole of what reaches the bead (ADR 0088 D1/D6.7)\nfull output:\n%s", l, out)
	}
}

// TestQALandedCheckDoesNotReadAnIgnoredErrorAsTheCause: a recipe line
// prefixed `-` whose command fails is reported by make as `[Makefile:N: door]
// Error 1 (ignored)` — no `***`, and the door did not fail. Picking that line
// as the cause would re-run a door that PASSED, which then reports as "red
// under -j4 and green alone": a wrong finding on a bead rather than a missed
// one, which is why the script guards it twice (the `***` in the pattern and
// an `(ignored)` exclusion) and why this arm pins the pair rather than either.
//
// MUTATION (MEASURED 2026-11-22): dropping EITHER guard alone survives this
// arm, because the other still excludes the line; dropping both reds it, and
// reds nothing else. That is the arm doing what its last sentence claims.
func TestQALandedCheckDoesNotReadAnIgnoredErrorAsTheCause(t *testing.T) {
	t.Parallel()
	mk := "test: door-ignored door-real\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-ignored:\n\t-@echo door-ignored shrugs; exit 1\n" +
		"door-real:\n\t@sleep 1; echo door-real is unhappy; exit 1\n"
	out, code := ldcRun(t, ldcTree(t, mk))
	if code == 0 {
		t.Fatalf("a run whose second door really failed exited 0\n%s", out)
	}
	if !strings.Contains(out, "(ignored)") {
		t.Fatalf("make printed no `(ignored)` line — then this fixture is not the shape the arm is about\n%s", out)
	}
	if strings.Contains(out, "green alone") {
		t.Errorf("the check re-ran the IGNORED door, found it green, and reported a door red under -j4 and green alone — that is a wrong finding, not a missed one\n%s", out)
	}
	tail := ldcTail(t, out)
	if l := tail[len(tail)-1]; !strings.Contains(l, "door-real") {
		t.Errorf("the last non-empty line is %q — the cause is `door-real`, the only door that actually failed\nfull output:\n%s", l, out)
	}
}

// TestQALandedCheckARedThatNamesNoDoorIsStillRed: the parser is now between
// the body's red and the verdict, so it is a place a green could come back
// from. A goal with no rule is the live shape of it — make refuses it and
// prints no `*** [Makefile:N: <door>] Error` line at all — and the run must
// still be red, and say that nothing could be named.
func TestQALandedCheckARedThatNamesNoDoorIsStillRed(t *testing.T) {
	t.Parallel()
	mk := "test: door-missing\n\t@touch " + ldcArmWitness + "\n"
	out, code := ldcRun(t, ldcTree(t, mk))
	if code == 0 {
		t.Fatalf("a red body from which no door could be parsed exited 0 — never green (ADR 0088 D6.7)\n%s", out)
	}
	if !strings.Contains(out, "no door can be named") {
		t.Errorf("the red does not say that no door could be named, so the bead would carry make's refusal with no sentence about why nothing was re-run:\n%s", out)
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

// TestQALandedCheckIgnoresAnInheritedKeepGoing is the other half: `-k` runs
// every remaining door after the first red. The tail no longer turns on it —
// since D6.7 the tail is made by the serial re-run — so what an inherited
// `-k` costs is the body's own fail-fast: the doors behind the first batch
// would all run, for no verdict, on a box the judge shares with the seats
// this exists to protect. Scrubbed, they do not.
func TestQALandedCheckIgnoresAnInheritedKeepGoing(t *testing.T) {
	t.Parallel()
	tree := ldcTree(t, ldcBatchMakefile())
	out, code := ldcRunEnv(t, tree, []string{"MAKEFLAGS=-k"})
	if code == 0 {
		t.Fatalf("a run under MAKEFLAGS=-k over a fixture whose first door fails exited 0\n%s", out)
	}
	for _, door := range ldcLateDoors {
		if _, err := os.Stat(filepath.Join(tree, door+"-ran")); err == nil {
			t.Errorf("MAKEFLAGS=-k carried the run past the failing door — %s ran, and it is behind the first batch of four (ADR 0088 D6.7)\nfull output:\n%s", door, out)
		}
	}
	tail := ldcTail(t, out)
	if l := tail[len(tail)-1]; !strings.Contains(l, "door-a") {
		t.Errorf("the last non-empty line under MAKEFLAGS=-k is %q — the re-run's own tail must survive an inherited -k (ADR 0088 D1/D6.7)\nfull output:\n%s", l, out)
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
	tail := ldcTail(t, out)
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
	want := ldcParallelStep("door-a", "door-b")
	if len(steps) != 2 || steps[0] != want {
		t.Errorf("planned %q over a `test::` rule, want step 1 %q — a derivation stripping one colon plans the goal `:`", steps, want)
	}
}

// TestQALandedCheckDropsTheOrderOnlySeparator: `test: door-a | door-b` makes
// door-b an ORDER-ONLY prerequisite — still a prerequisite, still a goal this
// check must run — but `|` is punctuation and not one. make's own data base
// prints the separator verbatim (`test: door-a | door-b`), so a derivation
// that scanned the line as plain words would plan `make … door-a | door-b`,
// and that is not three words to a real make invocation: it is `make
// door-a`, a shell pipe, and a bare `|` make reads as a goal of its own,
// which dies at "No rule to make target '|'. Stop." — a landing read red
// over a Makefile spelling, the one failure mode this script's header says
// the derivation exists to avoid (ranger-base-nr3qf finding 1, escaped from
// ranger-base-r3pqf).
func TestQALandedCheckDropsTheOrderOnlySeparator(t *testing.T) {
	t.Parallel()
	mk := "test: door-a | door-b\n" +
		"\t@touch " + ldcArmWitness + "\n" +
		"door-a:\n\t@:\n" +
		"door-b:\n\t@:\n"
	steps := ldcPlan(t, ldcTree(t, mk))
	want := ldcParallelStep("door-a", "door-b")
	if len(steps) != 2 || steps[0] != want {
		t.Errorf("planned %q over a `test: door-a | door-b` rule, want step 1 %q — the order-only separator is not a goal and a derivation that plans it fails `make` outright", steps, want)
	}
}
