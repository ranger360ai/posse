package treepins

// QA pins for ranger-base-lhaae — arm 11 of the suite-lock self-test must
// stand its wrapper in a REAL queue before asserting that a queue is
// survivable.
//
// THE DEFECT. Arm 11 of `scripts/suite-lock.sh --self-test` holds the one
// property `set -euo pipefail` wrappers depend on: "all slots busy" is this
// library's ordinary answer, and a bare non-zero return there kills the
// wrapper instead of queueing it — a busy box becomes a suite that will not
// start and says nothing about why. scripts/gotest.sh runs that way, so the
// arm is the only thing between that and the crew.
//
// It set the queue up on a clock:
//
//	( ... bash "$tmp/strict.sh" ... ) &
//	sleep 1
//	rm -f "$tmp/hold12"          # frees a slot
//	if wait_file "$tmp/strict.rc" "$fork_s" && rc = 0 && 'reached the end'
//
// A fork that had not reached its acquire inside that second found the slot
// already free, acquired at once, and reached the end — so the arm printed
// `ok` over a run that never queued. An unqueued acquire under `set -e`
// returns 0 whether or not the queued path is safe, which makes the green
// vacuous, and a vacuous green is the direction that announces nothing.
//
// MEASURED 2026-09-11, darwin/arm64, at 3268ac14, on a COPY of the script
// with two mutations — the queued path made fatal to a `set -e` caller (a
// bare `false` after the poll sleep, reached only by a wrapper that queues)
// and the rig's strict wrapper delayed 2s before its acquire, which is what
// a loaded box does for free:
//
//	old arm:  ok    set -e: a queued acquire does not kill the wrapper
//	          suite-lock --self-test: all arms ok           (exit 0)
//	new arm:  FAIL  set -e: a queued acquire does not kill the wrapper:
//	          rc=1, out: suite-lock: waiting for suite lock held by ...
//	          suite-lock --self-test: FAILED                (exit 1)
//
// Same library, same break, opposite verdicts: the clock decided the arm.
//
// THE FIX, and therefore what these arms hold: the slot is freed on the
// wrapper's OWN queued announcement (`wait_answer`, backstopped by `$fork_s`),
// and an arm 11 that finds the wrapper already through with both slots held
// fails saying it never queued. This is ranger-base-4psg2's answer and arm
// 5b's, one step further back — not an absence read off a clock, but the SETUP
// for the property arranged on one.
//
// It landed in two commits, which is why these arms hold two things. The clock
// went first, in ranger-base-1p4ig's sweep of four arms: arm 11 now waits for
// `wait_answer` before it frees the slot, so the race above is gone. But
// wait_answer returns on the queued line OR on the wrapper's exit, so what it
// establishes is `answered`, not `queued` — a wrapper already through with
// both slots held still reached `ok`, and an unqueued acquire under `set -e`
// returns 0 and reaches the end whether or not the queued path is safe. The
// same vacuous green by a different road. The queued LINE is the gate
// (ranger-base-beuza, replaying this bead's work over 1p4ig's).
//
// TWO ARMS, because the executable half cannot see every clock:
//
//  1. by EXECUTION: with the queued path made fatal and the fork late, arm 11
//     must FAIL. That is the measurement above, and a setup that freed the
//     slot on any clock shorter than the fork's delay goes green on it.
//  2. by SOURCE: between the fork and the freeing of the slot, nothing waits
//     on a clock at all and something reads the queued announcement. A
//     regression to a GENEROUS sleep would satisfy arm 1 while being the same
//     defect one loaded box later, and no run on an idle box can tell a 1 from
//     a 30; a regression to wait_answer alone would satisfy arm 1 too, because
//     that mutant's wrapper dies IN the acquire and so fails either way.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The arm, by the label it prints — the same string suitelock_qa_test.go
// requires the self-test to keep printing.
const slStrictArm = "set -e: a queued acquire does not kill the wrapper"

var (
	// The fork of the strict wrapper, and the line that frees the slot it is
	// meant to be queued behind.
	slStrictFork = regexp.MustCompile(`bash "\$tmp/strict\.sh"`)
	slFreesSlot  = regexp.MustCompile(`^\s*rm -f "\$tmp/hold12"`)
	// A wall-clock wait: `sleep 1` on its own, or after a `;`/`&&`. The
	// library's own poll sleep is not in scope — this scan reads one region of
	// the self-test body.
	slSleepWait = regexp.MustCompile(`(^|[;&|(]\s*)sleep\s`)
	// The evidence the arm is allowed to wait on instead — and it takes BOTH.
	// wait_answer returns on the queued line OR on the wrapper's exit, so on
	// its own it establishes "answered", not "queued".
	slAnswerWait = regexp.MustCompile(`\bwait_answer\s+"`)
	slQueuedLine = regexp.MustCompile(`waiting for suite lock`)
)

// The two mutations, applied to a copy. Each is checked for presence first: a
// mutation that no longer matches leaves the arm passing over a script it
// never changed.
const (
	// Reached only by a wrapper that QUEUES — a wrapper that gets a slot on
	// its first sweep never runs this line — and fatal only to a caller under
	// `set -e`, which is arm 11's wrapper and no other fork in the rig. That
	// is the regression class the arm exists for, narrowed to the path the
	// arm is supposed to be standing in.
	slPollSleep = "\t\tsleep \"${POSSE_SUITE_LOCK_POLL:-5}\"\n"
	slPollFatal = slPollSleep + "\t\tfalse\n"
	// And the loaded box, made deterministic: the strict wrapper reaches its
	// acquire two seconds after it is forked.
	slStrictBody = ". \"$1\"\nsuite_lock_acquire go test"
	slStrictSlow = ". \"$1\"\nsleep 2\nsuite_lock_acquire go test"
)

// Arm 1: with the queued path fatal and the fork late, arm 11 fails.
//
// An arm that freed the slot on a clock shorter than those two seconds would
// pass this self-test — the wrapper would find the slot free, never queue,
// never reach the fatal line, and reach the end (MEASURED above, `ok` and exit
// 0 on the same mutant).
func TestQASuiteLockStrictArmFailsWhenTheQueuedPathKillsTheWrapper(t *testing.T) {
	src, err := os.ReadFile(suiteLockScript)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if n := strings.Count(s, slPollSleep); n != 1 {
		t.Fatalf("%s: the queued poll sleep this arm makes fatal appears %d times, want 1 — the mutation lands somewhere else or nowhere, so a green here is not evidence",
			suiteLockScript, n)
	}
	if n := strings.Count(s, slStrictBody); n != 1 {
		t.Fatalf("%s: the strict wrapper this arm delays appears %d times, want 1 — the mutation lands somewhere else or nowhere, so a green here is not evidence",
			suiteLockScript, n)
	}
	broken := strings.Replace(s, slPollSleep, slPollFatal, 1)
	broken = strings.Replace(broken, slStrictBody, slStrictSlow, 1)

	path := filepath.Join(t.TempDir(), "suite-lock-queued-acquire-is-fatal.sh")
	if err := WriteExecutable(path, []byte(broken), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", path, "--self-test").CombinedOutput()
	got := string(out)

	// The positive witness first, and it is the whole reason this arm can
	// distinguish anything: the mutation is invisible to every other fork in
	// the rig (none of them runs under `set -e`), so a run that broke
	// wholesale is measuring something else.
	if !strings.Contains(got, "ok    slots: two concurrent full suites both run") {
		t.Fatalf("the mutant self-test did not get as far as arm 1, so nothing it says about arm 11 is evidence\n%s", got)
	}
	if err == nil {
		t.Fatalf("%s --self-test passed with a queued acquire that KILLS a `set -e` wrapper.\n\n"+
			"Arm 11 is the only thing holding that property, and the mutant's wrapper was\n"+
			"forked 2s before it reached its acquire — a loaded box does that for free.\n"+
			"An arm that frees the slot on a clock lets the wrapper through unqueued, and\n"+
			"an unqueued acquire under `set -e` returns 0 and reaches the end whether or\n"+
			"not the queued path is safe (ranger-base-lhaae).\n\n%s", suiteLockScript, got)
	}
	armFail := "FAIL  " + slStrictArm
	if !strings.Contains(got, armFail) {
		t.Fatalf("the mutant self-test failed, but not on arm 11 — it is refusing for some other reason and is not measuring this\n%s", got)
	}
	for _, l := range strings.Split(got, "\n") {
		if !strings.HasPrefix(l, armFail) {
			continue
		}
		// The wrapper's exit status, which is what separates "it died in the
		// acquire" from "it queued and then hung": a failure phrased as a
		// deadline that expired is a clock deciding the arm again.
		if !strings.Contains(l, "rc=") {
			t.Errorf("arm 11 failed without saying what the wrapper did:\n  %s\n\n"+
				"It must report the wrapper's rc and its output, which is the evidence a\n"+
				"loaded box cannot fake either way (ranger-base-lhaae).", l)
		}
	}
}

// Arm 2: and the setup waits on the wrapper, not on a clock.
func TestQASuiteLockStrictArmQueuesOnEvidenceAndNotOnAClock(t *testing.T) {
	lines, off := slSelfTestBody(t)

	fork := -1
	for i, raw := range lines {
		if strings.HasPrefix(strings.TrimLeft(raw, " \t"), "#") {
			continue
		}
		if slStrictFork.MatchString(raw) {
			if fork >= 0 {
				t.Fatalf("%s: the strict wrapper is forked on more than one line (%d and %d) — this scan reads the first region and would sleep through the other",
					suiteLockScript, off+fork+1, off+i+1)
			}
			fork = i
		}
	}
	if fork < 0 {
		t.Fatalf("%s: no fork of $tmp/strict.sh in the self-test body — arm 11 is gone or renamed, and this pin is reading nothing (suitelock_qa_test.go holds the arm's existence by name)",
			suiteLockScript)
	}

	free := -1
	for i := fork + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimLeft(lines[i], " \t"), "#") {
			continue
		}
		if slFreesSlot.MatchString(lines[i]) {
			free = i
			break
		}
	}
	if free < 0 {
		t.Fatalf("%s: nothing after the strict fork frees $tmp/hold12 — the arm cannot be testing a queue that drains, and this pin is reading nothing",
			suiteLockScript)
	}
	if free == fork+1 {
		t.Fatalf("%s:%d: the slot is freed on the line after the fork, so nothing the WRAPPER did can be what freed it",
			suiteLockScript, off+free+1)
	}

	var sleeps []string
	var answered, queued bool
	for i, raw := range lines[fork+1 : free] {
		l := strings.TrimLeft(raw, " \t")
		if strings.HasPrefix(l, "#") {
			continue
		}
		where := suiteLockScript + ":" + strconv.Itoa(off+fork+1+i+1) + ": " + l
		if slSleepWait.MatchString(l) {
			sleeps = append(sleeps, where)
		}
		if slAnswerWait.MatchString(l) {
			answered = true
		}
		if slQueuedLine.MatchString(l) {
			queued = true
		}
	}
	if len(sleeps) > 0 {
		t.Errorf("arm 11 of %s --self-test waits on a clock before freeing the slot:\n  %s\n\n"+
			"That is a scheduler budget in the SETUP for the property rather than in an\n"+
			"assertion. A fork that has not reached its acquire inside it finds the slot\n"+
			"already free, acquires at once, and reaches the end — and the arm reports `ok`\n"+
			"over a run that never queued, which is the one path it exists to test\n"+
			"(ranger-base-lhaae MEASURED that green over a queued acquire that killed the\n"+
			"wrapper). Wait for the wrapper's own queued announcement instead.",
			suiteLockScript, strings.Join(sleeps, "\n  "))
	}
	if !answered {
		t.Errorf("arm 11 of %s --self-test frees the slot without waiting for the wrapper to answer at all.\n\n"+
			"The wrapper's own answer is the only positive evidence that it is where the\n"+
			"assertion needs it: `wait_answer \"$tmp/strict.rc\" \"$fork_s\"`. Without it the\n"+
			"arm is deciding on when the scheduler got round to a fork (ranger-base-lhaae).",
			suiteLockScript)
	}
	if !queued {
		t.Errorf("arm 11 of %s --self-test frees the slot on an answer without checking it is the QUEUED one.\n\n"+
			"wait_answer returns on the 'waiting for suite lock' line OR on the wrapper's\n"+
			"exit, so alone it establishes `answered`, not `queued`. A wrapper already\n"+
			"through with both slots held never stood in the queue this arm exists to test,\n"+
			"and an unqueued acquire under `set -e` returns 0 and reaches the end whether or\n"+
			"not the queued path is safe — the same vacuous green the `sleep 1` produced, by\n"+
			"a different road. Gate on the line: `log_has \"$tmp/strict.rc.log\" 'waiting for\n"+
			"suite lock'` (ranger-base-lhaae).",
			suiteLockScript)
	}
}
