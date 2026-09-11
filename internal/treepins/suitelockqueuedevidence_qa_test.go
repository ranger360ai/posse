package treepins

// QA pins for ranger-base-fhjvs — the suite-lock self-test's QUEUED arms must
// read a marker's absence only after the holder has answered.
//
// THE DEFECT, and it is the twin of ranger-base-4psg2 facing the other way.
// Three arms of `scripts/suite-lock.sh --self-test` assert that a run is
// QUEUED, which is an absence — no marker file — and asked it like this:
//
//	"$tmp/holder.sh" ... &
//	sleep 2
//	if [ -e "$tmp/m3" ]; then bad ...
//
//	arm 2   queue: a third full suite waits
//	arm 13  slots: a negative POSSE_SUITE_SLOTS does not widen the queue
//	arm 14  orphan: ... (the queued half)
//
// A fork that has not been scheduled yet has written no marker either, so on a
// loaded box those two seconds are equally satisfied by a lock that IS handing
// out the slot the arm says it withholds. ranger-base-4psg2 turned a true
// reading into a red suite; this turns a regression into a green one, which is
// the direction that does not announce itself.
//
// MEASURED 2026-09-11, darwin/arm64, on the rig arm 2 below builds: with the
// lock widened to nine slots and the three holders slowed by 3s before their
// acquire, the pre-fix self-test printed
//
//	ok    queue: a third full suite waits
//	ok    slots: a negative POSSE_SUITE_SLOTS does not widen the queue
//	ok    orphan: a dead wrapper leaves the slot held by its child, and says so
//
// over a lock with no queue in it at all. After the fix the same rig fails all
// three, each naming the slot it watched the run take.
//
// WHY THIS FILE ARRIVES AFTER THE FIX IT GUARDS (ranger-base-a6gcp). The three
// arms were repaired on main by ranger-base-1p4ig, which landed while this
// bead's branch sat merge-blocked: 1p4ig swept every arm whose deadline was a
// bare `sleep` and reached these three from the other direction. So the script
// change this pin was written alongside is already on main, in 1p4ig's
// spelling, and replaying it would have been a no-op dressed as a conflict.
// What was NOT on main is this file. That is the asymmetry worth naming: the
// fix was reachable twice and the guard only once, and a fix nobody pinned is
// a fix that comes back.
//
// WHAT THE TWO ARMS BELOW HOLD, and why it takes two:
//
//  1. source: every `-e "$tmp/m*"` read in the self-test is preceded by a
//     `wait_answer` on that same marker. That is the rule in one line — an
//     absence is evidence only after the holder has SPOKEN (its marker, or its
//     "waiting for suite lock" line) — and it catches the regression at the
//     spelling, where it is a `sleep` nobody would look twice at.
//  2. execution: the arms still FAIL when the queue really is broken, and say
//     what they saw. A rig can satisfy arm 1 by waiting for an answer and then
//     deciding nothing, and the control run guards the other side: the same
//     slowed forks against an INTACT lock must leave every arm green, so the
//     three FAILs are the widened lock and never the 3s.
//
// The control is not decoration. Without it this pin is equally satisfied by
// arms that fail on any slow fork — which is ranger-base-4psg2, the defect
// whose fix these arms were left out of.

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
	// `[ -e "$tmp/m3" ]` and `[ ! -e "$tmp/m5b" ]`. Markers only: the
	// hold-files are `$tmp/hold*` and the holder scripts' own `[ -e "$hold" ]`
	// loops are a process waiting to be told to exit, not an arm deciding.
	slMarkerRead = regexp.MustCompile(`\[\s*(?:!\s*)?-e\s+"\$tmp/(m[A-Za-z0-9]*)"\s*\]`)
	slWaitAnswer = regexp.MustCompile(`wait_answer\s+"\$tmp/(m[A-Za-z0-9]*)"`)
)

// Arm 1: no arm reads a marker's absence off a clock.
func TestQANoSuiteLockArmReadsAMarkerAbsenceBeforeTheHolderAnswers(t *testing.T) {
	lines, off := slSelfTestBody(t)

	answered := map[string]bool{}
	reads := 0
	var early []string
	for i, raw := range lines {
		l := strings.TrimLeft(raw, " \t")
		if strings.HasPrefix(l, "#") {
			continue
		}
		waits := map[string]int{}
		for _, m := range slWaitAnswer.FindAllStringSubmatchIndex(raw, -1) {
			name := raw[m[2]:m[3]]
			if at, seen := waits[name]; !seen || m[0] < at {
				waits[name] = m[0]
			}
		}
		for _, m := range slMarkerRead.FindAllStringSubmatchIndex(raw, -1) {
			reads++
			name := raw[m[2]:m[3]]
			if answered[name] {
				continue
			}
			// Same line counts only when the wait comes FIRST — the
			// `wait_answer ... && [ ! -e ... ]` shape, where the read is
			// reached only after the answer.
			if at, ok := waits[name]; ok && at < m[0] {
				continue
			}
			early = append(early, suiteLockScript+":"+strconv.Itoa(off+i+1)+": "+l)
		}
		for name := range waits {
			answered[name] = true
		}
	}

	// The positive witness: a regexp that stopped matching would leave this
	// arm green over a scan that read nothing. MEASURED against main at
	// ranger-base-a6gcp: 7 such reads.
	if reads < 5 {
		t.Fatalf("the scan found only %d marker `-e` reads in %s's self-test — it was 7, so the scan has gone blind and a clean result here means nothing",
			reads, suiteLockScript)
	}
	if len(early) > 0 {
		t.Errorf("%d marker read(s) in %s --self-test happen before the holder has answered:\n  %s\n\n"+
			"A marker that is not there yet is not evidence of a QUEUE. A fork that\n"+
			"has not been scheduled yet has written no marker either, so an arm that\n"+
			"reads the absence off a `sleep` passes over a lock that is handing out\n"+
			"the slot the arm says it withholds (ranger-base-fhjvs MEASURED arms 2, 13\n"+
			"and 14 printing ok against a lock with nine slots). Wait for the holder's\n"+
			"answer first — `wait_answer \"$tmp/<marker>\" \"$fork_s\"` — and read the\n"+
			"absence after it.",
			len(early), suiteLockScript, strings.Join(early, "\n  "))
	}
}

// Arm 2: the three queued arms fail on the evidence that a run took a slot,
// and do not fail merely because a fork was slow.
//
// Two runs of the same rig, both with the three holders those arms watch
// slowed by 3s — longer than the `sleep 2` this bead removed, which is the
// load symptom without the load:
//
//	control  the lock untouched: every arm must still be ok.
//	mutant   the lock widened to nine slots, so nothing ever queues: those
//	         three arms must FAIL, each saying which slot it watched the run
//	         take.
func TestQATheQueuedArmsFailOnEvidenceAndNotOnASlowFork(t *testing.T) {
	src, err := os.ReadFile(suiteLockScript)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)

	// The holder heredoc's acquire — the FIRST of the three copies (holder,
	// releaser, orphaner), which is the one arms 2, 13 and 14 fork.
	const acquire = ". \"$lib\"\nsuite_lock_acquire \"$@\" 2>\"$marker.log\"\n"
	at := strings.Index(s, acquire)
	hs, rs := strings.Index(s, `cat >"$tmp/holder.sh"`), strings.Index(s, `cat >"$tmp/releaser.sh"`)
	if at < 0 || hs < 0 || rs < 0 || at < hs || at > rs {
		t.Fatalf("%s: the holder heredoc this pin slows is not where it was (acquire at %d, holder.sh at %d, releaser.sh at %d) — the pin cannot build its rig and is therefore not evidence",
			suiteLockScript, at, hs, rs)
	}
	// Only the markers arms 2, 13 and 14 wait on, so the other twenty forks
	// cost nothing: this is about THEIR latency, not the box's.
	slow := s[:at] + ". \"$lib\"\ncase ${marker##*/} in m3 | m17 | m19) sleep 3 ;; esac\nsuite_lock_acquire \"$@\" 2>\"$marker.log\"\n" + s[at+len(acquire):]

	const slots = "\tlocal n=${POSSE_SUITE_SLOTS:-2}\n"
	if strings.Count(slow, slots) != 1 {
		t.Fatalf("%s no longer contains the slot-count line this pin widens (%q) — the mutant cannot break the queue and the arm is not evidence",
			suiteLockScript, slots)
	}
	wide := strings.Replace(slow, slots, "\tlocal n=9\n", 1)

	selfTest := func(name, body string) (string, error) {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if err := WriteExecutable(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("bash", path, "--self-test").CombinedOutput()
		return string(out), err
	}

	ctl, err := selfTest("suite-lock-slow-forks.sh", slow)
	if err != nil || !strings.Contains(ctl, "all arms ok") {
		t.Fatalf("the CONTROL failed: with the lock untouched and the three holders 3s slow, %s --self-test must still be green — read the arm that failed, because it is one of two things:\n\n"+
			"  an arm decided on WHEN the fork got scheduled instead of on what the\n"+
			"  marker says (ranger-base-4psg2, and it reds a whole suite through\n"+
			"  verify-suite-lock); or\n\n"+
			"  an arm read a queued holder's LOG before the holder had written it —\n"+
			"  arm 3 does that to arm 2's log, and on the pre-fix script it is exactly\n"+
			"  what this control caught: `FAIL  queue: the waiting line names the\n"+
			"  holding worktree: got: ` with an empty file.\n\n%s",
			suiteLockScript, ctl)
	}

	got, err := selfTest("suite-lock-slow-forks-nine-slots.sh", wide)
	if err == nil {
		t.Fatalf("%s --self-test passed with nine slots handed out and nothing ever queued: no arm of it can tell a queue from no queue\n%s",
			suiteLockScript, got)
	}

	// Each arm's own FAIL line, and the words that say it decided on what it
	// SAW. An arm that had gone back to `sleep 2` prints `ok` here, over a
	// lock with no queue in it.
	for _, want := range []struct{ arm, evidence string }{
		{"FAIL  queue: a third full suite waits", "it started anyway, on slot "},
		{"FAIL  slots: a negative POSSE_SUITE_SLOTS does not widen the queue", "a third suite ran anyway, on slot "},
		{"FAIL  orphan: a dead wrapper leaves the slot held by its child, and says so", "queued=0"},
	} {
		found := false
		for _, l := range strings.Split(got, "\n") {
			if strings.HasPrefix(l, want.arm) && strings.Contains(l, want.evidence) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("with nine slots handed out and nothing ever queued, %s --self-test did not fail saying %q:\n\nmissing: %s ... %s\n\nfull output:\n%s",
				suiteLockScript, want.evidence, want.arm, want.evidence, got)
		}
	}
}
