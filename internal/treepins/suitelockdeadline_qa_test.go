package treepins

// QA pins for ranger-base-4psg2 — the suite-lock self-test's arms must not
// decide on how fast a fork got scheduled.
//
// THE DEFECT. Arms 4 and 5 of `scripts/suite-lock.sh --self-test` asserted
// that a `-run` filtered suite and a single-package run take NO slot, and
// asked it like this:
//
//	if wait_file "$tmp/m4" 5 && [ "$(slot_of "$tmp/m4")" = none ]; then
//
// The claim is about the marker's VALUE. The measurement was about a forked
// holder being scheduled inside five wall-clock seconds, and on a loaded box
// those are different questions.
//
// MEASURED 2026-09-11, darwin/arm64, with both suite slots held by two real
// crew suites (`scripts/suite-lock.sh --status` named the two seats), `make
// test` in a third worktree:
//
//	FAIL  unlocked: a -run filtered suite takes no slot:
//	      marker 'none' after 5s with both slots held
//	suite-lock --self-test: FAILED
//	make: *** [verify-suite-lock] Error 1
//
// The marker already said `none` — the arm's own claim, TRUE. Only the
// deadline had expired, so the `&&` short-circuited and the arm reported its
// true reading as a failure. `make test` runs verify-suite-lock as a prereq,
// so that reds the WHOLE suite before a single Go test runs, with a message
// about lock slots — the least likely thing a reader connects to their diff.
// `make verify-suite-lock` alone, minutes later, same tree, slots still held:
// all arms ok.
//
// THE FIX, and therefore what these two arms hold:
//
//  1. no arm carries its own wall-clock number any more. One `fork_s`
//     backstop, generous on purpose, because nothing in that self-test TIMES
//     a lock operation — the deadline is there for a holder that never ran at
//     all, and it is spent only by an arm that is already failing. Arm 1 here
//     reads the source, because the regression is a literal digit and no
//     execution can distinguish a 5 from a 60 on an idle box.
//  2. the arms that can read BOTH answers do. A holder answers by writing its
//     marker or by announcing that it is queued; waiting for either is what
//     takes the scheduler out of the verdict. Arm 2 here proves it by
//     execution: it breaks the library so a filtered run IS queued, and
//     requires arm 4 to fail SAYING it queued — the evidence, not a timeout.
//
// Arm 2 is what keeps arm 1 from being satisfiable by a self-test that waits
// generously and then decides nothing at all.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	// A call, never the definitions: both helpers take a quoted path first.
	slWaitCall = regexp.MustCompile(`\bwait_(?:file|answer)\s+"[^"]+"\s+(\S+?)(?:;|\s|$)`)
	slForkS    = regexp.MustCompile(`^\s*local fork_s=(\d+)\s*$`)
)

// slSelfTestBody returns the lines of _suite_lock_selftest, which is where
// every arm lives. Scoped rather than whole-file so the library's own poll
// and heartbeat numbers — which ARE lock behaviour and are documented as such
// — do not read as arm deadlines.
func slSelfTestBody(t *testing.T) ([]string, int) {
	t.Helper()
	b, err := os.ReadFile(suiteLockScript)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "_suite_lock_selftest() {") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("%s has no _suite_lock_selftest() to scan — this pin is reading nothing", suiteLockScript)
	}
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "}") {
			// The offset travels with the lines: a hit reported at its
			// index in the SLICE sends the next reader 500 lines up the
			// file, which is how a good failure message stops being read.
			return lines[start:i], start
		}
	}
	t.Fatalf("%s: _suite_lock_selftest() never closes at column 0", suiteLockScript)
	return nil, 0
}

// Arm 1: every arm's deadline comes from the one backstop, and the backstop is
// generous.
func TestQANoSuiteLockArmCarriesItsOwnWallClockBudget(t *testing.T) {
	lines, off := slSelfTestBody(t)

	var budget int
	var calls int
	var literal []string
	for i, raw := range lines {
		l := strings.TrimLeft(raw, " \t")
		if strings.HasPrefix(l, "#") {
			continue
		}
		if m := slForkS.FindStringSubmatch(raw); m != nil {
			budget, _ = strconv.Atoi(m[1])
		}
		for _, m := range slWaitCall.FindAllStringSubmatch(l, -1) {
			calls++
			if m[1] != `"$fork_s"` {
				literal = append(literal, suiteLockScript+":"+strconv.Itoa(off+i+1)+": "+l)
			}
		}
	}

	// The positive witness. A regexp that stopped matching, or a body finder
	// that stopped finding the body, would leave this arm green over a scan
	// that read nothing. There were 19 such calls when this pin was written.
	if calls < 12 {
		t.Fatalf("the scan found only %d wait_file/wait_answer calls in %s's self-test — it was 19, so the scan has gone blind and a clean result here means nothing",
			calls, suiteLockScript)
	}
	if len(literal) > 0 {
		t.Errorf("%d arm(s) of %s --self-test carry their own wall-clock deadline:\n  %s\n\n"+
			"An arm's deadline must be `\"$fork_s\"`, the one backstop. A hand-written\n"+
			"number there is a budget for the SCHEDULER, not for the lock: on a box\n"+
			"already running two suites a fork does not get five seconds, and the arm\n"+
			"then reports its own true reading as a failure (ranger-base-4psg2 MEASURED\n"+
			"arm 4 printing FAIL over a marker that already said 'none'). `make test`\n"+
			"gates on verify-suite-lock, so that reds a whole suite before a package\n"+
			"builds.",
			len(literal), suiteLockScript, strings.Join(literal, "\n  "))
	}
	if budget == 0 {
		t.Fatalf("%s's self-test has no `local fork_s=<n>` — the arms have no backstop to share", suiteLockScript)
	}
	// Not a measured optimum and nothing here claims otherwise: 60 is what
	// the fix chose, and this floor only refuses a return to the budgets that
	// were measured losing.
	if budget < 30 {
		t.Errorf("%s's self-test backstop is fork_s=%d. It was 60, and 5 is what ranger-base-4psg2 measured a loaded box blowing; a backstop is spent only by an arm that is already failing, so there is nothing to buy by cutting it",
			suiteLockScript, budget)
	}
}

// Arm 2: the filtered-run arm fails on the EVIDENCE that the run queued, and
// says so — not on a deadline that expired.
//
// The mutation is the one that makes the property false while leaving the rig
// intact: suite_lock_wanted answers yes for everything, so a `-run` filtered
// suite queues behind the two held slots exactly like a full one. Arm 4 must
// then fail, and its message must be the queued announcement it read. An arm
// that had gone back to waiting out a timeout would fail too — with a
// different sentence, and one backstop later.
func TestQATheFilteredRunArmFailsOnEvidenceAndNotOnATimeout(t *testing.T) {
	src, err := os.ReadFile(suiteLockScript)
	if err != nil {
		t.Fatal(err)
	}
	const wantedFn = "suite_lock_wanted() {\n\tlocal a tree=0"
	if !strings.Contains(string(src), wantedFn) {
		t.Fatalf("%s no longer contains the function this arm mutates (%q) — the arm cannot fail and is therefore not evidence",
			suiteLockScript, wantedFn)
	}
	broken := strings.Replace(string(src), wantedFn,
		"suite_lock_wanted() {\n\treturn 0\n\tlocal a tree=0", 1)

	path := filepath.Join(t.TempDir(), "suite-lock-everything-queues.sh")
	if err := WriteExecutable(path, []byte(broken), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", path, "--self-test").CombinedOutput()
	got := string(out)
	if err == nil {
		t.Fatalf("%s --self-test passed with every run queued: the arm that says a filtered run takes no slot cannot fail\n%s",
			suiteLockScript, got)
	}
	const armFail = "FAIL  unlocked: a -run filtered suite takes no slot"
	if !strings.Contains(got, armFail) {
		t.Fatalf("the self-test failed, but not on the arm that was broken — it is refusing for some other reason and is not measuring this\n%s", got)
	}
	for _, l := range strings.Split(got, "\n") {
		if !strings.HasPrefix(l, armFail) {
			continue
		}
		if !strings.Contains(l, "it queued behind the held slots") {
			t.Errorf("arm 4 failed without saying WHY it decided:\n  %s\n\n"+
				"It must fail on the holder's own queued announcement, which is the\n"+
				"answer a loaded box cannot fake either way. A failure phrased as a\n"+
				"marker that did not arrive is a deadline deciding the arm again\n"+
				"(ranger-base-4psg2).", l)
		}
	}
}
