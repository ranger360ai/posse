//go:build posse_arm2

package posse

// ADR 0087's ratchet, measured at the site it runs: a replayed landing is
// judged by the repo's own check, and may not turn a green base red
// (ranger-base-ej47m, from ranger-base-qk7w6; landcheck.go's header has the
// incident and the whole table).
//
// THE SHAPE EVERY FIXTURE HERE IS BUILT ON is the a2327230 incident's own:
// two branches that each pass, and a replay of one onto the other that does
// not. The check reds when a.txt and b.txt are both in its cwd, so the base
// (b.txt alone) and the seat's tip (a.txt alone) are each green and only the
// replayed tree is red — which is what git's "no textual conflict" cannot
// see and what nothing compiled before this landed.
//
// MUTATION-CHECKED 2026-11-11, all three against the six landing tests this
// file held THEN — the count is that measurement's corpus and not a running
// total, and ranger-base-sjjf4 has since added five arms at the bottom, each
// naming the mutant it kills (the three that call runLandCheckWithin
// directly, and the no-check test, are deliberately insensitive to the first
// two):
//
//   - the gate not called at all reds six of them.
//   - blocking on ANY red rather than on red-over-green reds exactly
//     TestAReplayOverAnAlreadyRedBaseLandsWithAWarning — the ratchet's own
//     arm, and the one a stricter gate would break.
//   - the D5 reset not run reds four, which is the finding worth keeping:
//     TestASecondReplay… goes red because the branch is then left on the
//     replayed tip, and the NEXT call fast-forwards it — so a block without
//     the reset is not a stricter gate, it is a one-pass delay before the
//     unjudged arm lands the same red tree.
//
// IT IS A REAL CHILD, not a stubbed verdict: the thing being pinned is that
// posse runs the BASE checkout's executable with the session tree as its cwd
// and reads its exit status, and a fake would pin the plumbing and nothing
// about which bytes get judged. The script counts its own runs, which is how
// the tests below can tell "the base's verdict came off the record" from "the
// base was checked out and judged for real" — two outcomes that print the
// same sentence.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// landCheckBody is the fixture repo's check. It reds two ways: `broken` in
// the tree is a base that is red on its own, and a.txt beside b.txt is the
// semantic conflict a replay creates out of two green sides.
func landCheckBody(counter string) string {
	return "#!/bin/sh\n" +
		"echo run >> " + shq(counter) + "\n" +
		"if [ -f broken ]; then\n" +
		"  echo 'the base was already failing'\n" +
		"  exit 1\n" +
		"fi\n" +
		"if [ -f a.txt ] && [ -f b.txt ]; then\n" +
		"  echo 'two errors from the replayed tree:'\n" +
		"  echo 'a.txt and b.txt cannot both be here'\n" +
		"  exit 1\n" +
		"fi\n" +
		"exit 0\n"
}

// shipLandCheck writes a check into a tree, with the execute bit, and commits
// it path-limited the way every other fixture here commits.
func shipLandCheck(t *testing.T, dir, body, msg string) {
	t.Helper()
	p := filepath.Join(dir, landCheckScript)
	write(t, p, body)
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "--", landCheckScript)
	mustGit(t, dir, "commit", "-q", "-m", msg, "--", landCheckScript)
}

// landCheckRepo is a repo on main shipping the check, plus the path the check
// counts its runs in.
func landCheckRepo(t *testing.T) (*App, string, string) {
	t.Helper()
	a := wtApp(t)
	repo := wtRepo(t)
	counter := filepath.Join(t.TempDir(), "runs")
	shipLandCheck(t, repo, landCheckBody(counter), "ship the land check")
	return a, repo, counter
}

// landCheckRuns is how many times the check has run — the positive witness
// for D4's cache, which is otherwise invisible: a base verdict taken off the
// record and a base verdict measured by checking the base out produce the
// same landing and the same sentence.
func landCheckRuns(t *testing.T, counter string) int {
	t.Helper()
	b, err := os.ReadFile(counter)
	if err != nil {
		return 0
	}
	return len(strings.Fields(string(b)))
}

// landCheckSaysGreen runs the BASE's check over dir the way posse does, so a
// fixture can assert its own premise — that each side is green on its own —
// rather than resting on it.
func landCheckSaysGreen(t *testing.T, repo, dir string) bool {
	t.Helper()
	v, said := runLandCheck(repo, dir)
	if v == landUnanswered {
		t.Fatalf("fixture: the check would not run over %s: %s", dir, said)
	}
	return v == landGreen
}

// landCheckTree is a session tree whose branch holds one commit.
func landCheckTree(t *testing.T, a *App, repo, session, path, body string) *SessionTree {
	t.Helper()
	tr, err := a.EnsureSessionTree(repo, session, nil)
	if err != nil || tr == nil {
		t.Fatalf("session tree for %s: %v", session, err)
	}
	commitIn(t, tr.Path, path, body, session+": "+path)
	return tr
}

// recordedLandCheck is the one repo-level record D4 keeps, as git holds it.
func recordedLandCheck(t *testing.T, repo string) string {
	t.Helper()
	out, err := git(repo, "config", "--get", landCheckKey)
	if err != nil {
		return ""
	}
	return out
}

// landCheckBlockedTree is THE a2327230 SHAPE, as a fixture: two sides that
// each pass the check on their own, and a replay of one onto the other that
// does not. It asserts its own premises rather than resting on them — each
// side green, and the replay textually clean, so the conflict arm cannot be
// the one that answers.
//
// Shared with mergeBlockedCases (mergeblocked_test.go), which drives every
// o.Reason spelling the merge path can produce and needs this one reached.
func landCheckBlockedTree(t *testing.T) (*App, string, *SessionTree, string) {
	t.Helper()
	a, repo, counter := landCheckRepo(t)
	tr := landCheckTree(t, a, repo, "s-1", "a.txt", "the seat's own work\n")
	pre := mustGit(t, tr.Path, "rev-parse", "HEAD")
	// The base moves on a path the branch never touches, so the replay is
	// textually clean and the only thing that can refuse it is the check.
	commitIn(t, repo, "b.txt", "the operator's own work\n", "main: moved on")
	base := mustGit(t, repo, "rev-parse", "HEAD")
	if !landCheckSaysGreen(t, repo, tr.Path) {
		t.Fatal("fixture: the seat's own tip does not pass the check, so nothing here measures a replay that turns a green base red")
	}
	if !landCheckSaysGreen(t, repo, repo) {
		t.Fatal("fixture: the base does not pass the check, so this would be the red-over-red arm")
	}
	if !mergesCleanly(repo, base, pre) {
		t.Fatal("fixture: the branch does not merge cleanly onto the base, so the conflict arm would answer first")
	}
	return a, repo, tr, counter
}

// THE HEADLINE (a): the a2327230 shape. Two sides each green, the replay red,
// so the landing is blocked, the branch is left exactly where the seat left
// it, and the base never moves.
func TestAReplayThatWouldTurnAGreenBaseRedIsBlocked(t *testing.T) {
	t.Parallel()
	a, repo, tr, counter := landCheckBlockedTree(t)
	pre := mustGit(t, tr.Path, "rev-parse", "HEAD")
	base := mustGit(t, repo, "rev-parse", "HEAD")
	before := landCheckRuns(t, counter)

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatalf("a replay that fails the repo's own check over a green base LANDED — that is the a2327230 night again: %+v", o)
	}
	for _, want := range []string{landCheckScript, "a.txt and b.txt cannot both be here", "ADR 0087"} {
		if !strings.Contains(o.Reason, want) {
			t.Errorf("the block's reason does not name %q, and this sentence is embedded verbatim in the merge-back bead:\n%s", want, o.Reason)
		}
	}
	// D5: the branch is where the seat left it, in the tree and on the ref.
	if head := mustGit(t, tr.Path, "rev-parse", "HEAD"); head != pre {
		t.Errorf("the session tree is at %s, want the pre-replay tip %s — a block that leaves the replay in place lets the unjudged fast-forward arm land it next pass", abbrevSHA(head), abbrevSHA(pre))
	}
	if ref := refSHA(repo, tr.Branch); ref != pre {
		t.Errorf("%s is at %s, want %s", tr.Branch, abbrevSHA(ref), abbrevSHA(pre))
	}
	if cur := mustGit(t, tr.Path, "symbolic-ref", "--short", "HEAD"); cur != tr.Branch {
		t.Errorf("the session tree is on %q and not on its own branch after the block", cur)
	}
	// And the base never moved.
	if now := mustGit(t, repo, "rev-parse", "HEAD"); now != base {
		t.Errorf("main moved to %s over a landing that was refused", abbrevSHA(now))
	}
	if _, err := os.Stat(filepath.Join(repo, "a.txt")); err == nil {
		t.Error("the branch's file reached main anyway")
	}
	// Nothing is left in the tree for the next pass to read as the persona's
	// uncommitted work (ADR 0041 §1–§2).
	if dirt := dirtyPaths(tr.Path); len(dirt) > 0 {
		t.Errorf("the refused landing left %v in the session tree", dirt)
	}
	// Two runs: the replayed tip, and the base nobody had on record. The
	// base's verdict is what the ratchet turns on, so it was measured.
	if n := landCheckRuns(t, counter) - before; n != 2 {
		t.Errorf("the check ran %d times, want 2 (the replayed tip, then the base it had no record for)", n)
	}
	if got, want := recordedLandCheck(t, repo), revTreeID(repo, base)+" green"; got != want {
		t.Errorf("the record reads %q, want %q — without it every later red replay pays a second run", got, want)
	}
}

// AND THE SECOND PASS OVER IT (e, first half): the record is what makes the
// probe cost one run per BASE MOVEMENT. MergeSessionWork is called twice over
// an unmoved base here; the dispatcher half of (e) is the sweep's own skip.
func TestASecondReplayOverTheSameBaseTakesTheBaseVerdictOffTheRecord(t *testing.T) {
	t.Parallel()
	a, _, tr, counter := landCheckBlockedTree(t)
	// From here, not from zero: the fixture asserts its own premises by
	// running the check over each side, and those runs are in the counter too.
	before := landCheckRuns(t, counter)

	if o, err := MergeSessionWork(a, tr); err != nil || o.Merged {
		t.Fatalf("fixture: the first attempt was not blocked (%v, %+v)", err, o)
	}
	first := landCheckRuns(t, counter) - before
	if first != 2 {
		t.Fatalf("fixture: the first attempt ran the check %d times, want 2", first)
	}
	if o, err := MergeSessionWork(a, tr); err != nil || o.Merged {
		t.Fatalf("the second attempt landed a replay the first one refused (%v, %+v)", err, o)
	}
	if n := landCheckRuns(t, counter) - before - first; n != 1 {
		t.Errorf("the second attempt over the same base ran the check %d times, want 1 — the base's verdict is on record (D4) and paying for it again doubles the lock time of every red replay", n)
	}
}

// (b): the base was already red. Nothing this branch did can be blamed for
// it, the fix for a red base comes through this same path, so it lands — and
// says so, because a tip that reached main unjudged is a fact about main
// nobody else on the pass will mention.
func TestAReplayOverAnAlreadyRedBaseLandsWithAWarning(t *testing.T) {
	t.Parallel()
	a, repo, counter := landCheckRepo(t)
	tr := landCheckTree(t, a, repo, "s-1", "a.txt", "the seat's own work\n")
	commitIn(t, repo, "broken", "somebody else broke the base\n", "main: moved on, red")
	base := mustGit(t, repo, "rev-parse", "HEAD")
	if landCheckSaysGreen(t, repo, repo) {
		t.Fatal("fixture: the base passes the check, so this is not the red-over-red arm")
	}
	before := landCheckRuns(t, counter)

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged {
		t.Fatalf("a branch was stranded by a base somebody else broke — that is the failure ADR 0087 D3's ratchet exists to avoid: %s", o.Reason)
	}
	if !strings.Contains(o.LandCheckWarning, "already red") {
		t.Errorf("the pass said nothing about landing over a red base: %q", o.LandCheckWarning)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "a.txt")); err != nil || string(body) != "the seat's own work\n" {
		t.Errorf("the work did not land (%v)", err)
	}
	if n := landCheckRuns(t, counter) - before; n != 2 {
		t.Errorf("the check ran %d times, want 2 (the replayed tip, then the base)", n)
	}
	// The base probe put the session tree back on its own branch, and left
	// nothing in it (the restore is the half of D4's probe that must be
	// guaranteed).
	if cur := mustGit(t, tr.Path, "symbolic-ref", "--short", "HEAD"); cur != tr.Branch {
		t.Errorf("the session tree is on %q after the base probe, not on %s", cur, tr.Branch)
	}
	if dirt := dirtyPaths(tr.Path); len(dirt) > 0 {
		t.Errorf("the base probe left %v in the session tree", dirt)
	}
	// The tip is the new base and it is red, which the record now says — so
	// the next red replay onto it is a lookup and not a second run.
	if got, want := recordedLandCheck(t, repo), revTreeID(repo, tr.Branch)+" red"; got != want {
		t.Errorf("the record reads %q, want %q", got, want)
	}
	_ = base
}

// (c), end to end: a check that cannot answer at all. D3's bottom row — land,
// print the ⚠, never block — because a gate that reads its own silence as a
// verdict strands the fleet, and ci.yml stays the backstop.
func TestACheckThatCannotRunLandsWithAWarning(t *testing.T) {
	t.Parallel()
	a, repo, _ := landCheckRepo(t)
	// A shebang naming an interpreter that is not there: execve fails, so
	// this process never learns anything about the tree. The exec bit is on,
	// so the opt-in is not in question.
	shipLandCheck(t, repo, "#!/nonexistent/interpreter\nexit 0\n", "main: a check that cannot exec")
	tr := landCheckTree(t, a, repo, "s-1", "a.txt", "the seat's own work\n")
	commitIn(t, repo, "b.txt", "the operator's own work\n", "main: moved on")

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged {
		t.Fatalf("a check that could not answer blocked a landing: %s", o.Reason)
	}
	for _, want := range []string{"could not judge", landCheckScript} {
		if !strings.Contains(o.LandCheckWarning, want) {
			t.Errorf("the ⚠ does not name %q: %q", want, o.LandCheckWarning)
		}
	}
	if got := recordedLandCheck(t, repo); got != "" {
		t.Errorf("a check that answered nothing recorded %q — a verdict nobody measured is the one thing the record must never hold", got)
	}
}

// (c), the deadline itself. Asked of runLandCheckWithin rather than through a
// landing, for the reason that function's doc gives: reaching it through
// MergeSessionWork means spending generatorTimeout — two minutes of suite wall
// in a package already at ~950s — and the sentence is the same either way.
func TestACheckThatBlowsItsDeadlineIsNotAVerdict(t *testing.T) {
	t.Parallel()
	_, repo, _ := landCheckRepo(t)
	// `exec`, so the wedged process IS the child and the kill ends it with no
	// descendant holding the pipe — otherwise this test would also pay
	// generatorKillGrace, which is the test below's subject and not this
	// one's.
	shipLandCheck(t, repo, "#!/bin/sh\necho 'still working'\nexec sleep 30\n", "main: a check that wedges")
	start := time.Now()
	v, said := runLandCheckWithin(repo, repo, 300*time.Millisecond)
	if v != landUnanswered {
		t.Errorf("a check posse stopped hearing from answered %s — a blown deadline is a statement about posse's silence, never a verdict about the tree", v)
	}
	if !strings.Contains(said, "no answer in") {
		t.Errorf("the cause does not say the deadline blew: %q", said)
	}
	if waited := time.Since(start); waited > generatorTimeout {
		t.Errorf("the call took %s, which is the whole production bound — the limit was not honoured", waited)
	}
}

// (c), the other half of the bound: generatorKillGrace. The check exits and a
// descendant it started holds the output pipe open, so os/exec's Wait does not
// return on the child's exit at all — generatorTimeout bounds the child and
// this grace bounds the call. Unanswered rather than the exit status it also
// has: what was read is short by an unknown amount.
func TestACheckWhoseDescendantOutlivesItIsNotAVerdict(t *testing.T) {
	t.Parallel()
	_, repo, _ := landCheckRepo(t)
	shipLandCheck(t, repo, "#!/bin/sh\nsleep 10 &\nexit 0\n", "main: a check that leaks a child")
	v, said := runLandCheckWithin(repo, repo, generatorTimeout)
	if v != landUnanswered {
		t.Errorf("a check that leaked a child into the launcher answered %s: %q", v, said)
	}
	if !strings.Contains(said, "reaping grace") {
		t.Errorf("the cause does not name the grace that ended the wait: %q", said)
	}
}

// (d): a repo that ships no check lands exactly as it does today. Every repo
// the sweep walks but this one.
func TestARepoWithNoLandCheckLandsExactlyAsBefore(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr := landCheckTree(t, a, repo, "s-1", "a.txt", "the seat's own work\n")
	commitIn(t, repo, "b.txt", "the operator's own work\n", "main: moved on")

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged || !o.Rebased {
		t.Fatalf("a replayed landing in a repo with no check did not land: %+v", o)
	}
	if o.LandCheckWarning != "" {
		t.Errorf("a repo with no check got a ⚠: %q", o.LandCheckWarning)
	}
	if got := recordedLandCheck(t, repo); got != "" {
		t.Errorf("a repo with no check recorded %q", got)
	}
}

// (f): the check that runs is the BASE checkout's copy, which is the copy a
// human reviewed. A branch that rewrites the script to `exit 0` does not get
// to judge itself — the same decision generatedindex.go makes about whose
// generator runs, and the reason the launcher is not executing persona-authored
// code that nobody has read.
func TestTheBasesCopyOfTheCheckRunsEvenWhenTheBranchEditedIt(t *testing.T) {
	t.Parallel()
	a, repo, _ := landCheckRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil || tr == nil {
		t.Fatalf("session tree: %v", err)
	}
	commitIn(t, tr.Path, "a.txt", "the seat's own work\n", "s-1: a.txt")
	shipLandCheck(t, tr.Path, "#!/bin/sh\nexit 0\n", "s-1: this branch judges itself green")
	pre := mustGit(t, tr.Path, "rev-parse", "HEAD")
	commitIn(t, repo, "b.txt", "the operator's own work\n", "main: moved on")

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatalf("a branch that rewrote the check landed on its own verdict — the base's copy is the only one posse may run (ADR 0087 D1): %+v", o)
	}
	if !strings.Contains(o.Reason, "a.txt and b.txt cannot both be here") {
		t.Errorf("the block does not quote the BASE's check:\n%s", o.Reason)
	}
	// And the branch keeps its own edit: the base's copy judging it is not the
	// landing rewriting it.
	if head := mustGit(t, tr.Path, "rev-parse", "HEAD"); head != pre {
		t.Errorf("the branch is at %s, want its own tip %s", abbrevSHA(head), abbrevSHA(pre))
	}
	body, err := os.ReadFile(filepath.Join(tr.Path, landCheckScript))
	if err != nil || !strings.Contains(string(body), "exit 0") {
		t.Errorf("the branch's own copy of the check was changed by the landing (%v)", err)
	}
}

// (e): THE COST OF A BLOCK THAT STANDS. This is the one tension ADR 0087
// inherits rather than introduces — the landing sweep must not re-ask a
// question whose answer cannot have changed, because the replay that asks it
// WRITES the session tree and a tree written on every pass never goes quiet
// (ranger-base-9u5zy, ADR 0058 fact 4). A land-check block is the hardest
// case for that machinery, and it is worth saying why: every OTHER reason
// MergeSessionWork blocks is one the cheap readings can see stop being true,
// while this one is invisible to all of them. The branch still merges cleanly
// — `merge-tree` says so, because the conflict is semantic and git has no
// opinion about it — so mergesCleanly's filter opens the gate on every pass.
// What closes it is the record of the base the replay last ran against
// (probedBaseKey), and that record is the whole of the bound: one probe per
// base MOVEMENT, not one per pass.
//
// TestABaseThatMovesAndStillConflictsKeepsTheBlockAndTheTreeQuiet and
// TestAFilterThatDisagreesWithTheReplayProbesOncePerBaseMove
// (mergeblocked_qa_test.go) are the model, and the mtime half of the property
// lives there: this block takes the identical path through standingMergeBlock,
// and what is new here is only that the obstacle is the check. So what is
// measured below is the skip itself — which passes probe, which do not, and
// that six passes over one blocked branch still cost one handoff.
func TestALandCheckBlockReProbesOncePerBaseMovement(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", true)
	write(t, filepath.Join(repo, "fake-show.json"), `[{"id":"a-1","status":"closed","assignee":"ranger"}]`)
	counter := filepath.Join(t.TempDir(), "runs")
	shipLandCheck(t, repo, landCheckBody(counter), "ship the land check")
	// The seat's second commit and the operator's own, on paths that do not
	// touch each other: the replay is textually clean and only the check can
	// refuse it.
	commitIn(t, tr.Path, "a.txt", "the seat's own work\n", "a-1: a.txt")
	commitIn(t, repo, "b.txt", "the operator's own work\n", "main: moved on")
	pre := mustGit(t, tr.Path, "rev-parse", "HEAD")
	dispatcherErr(t, d)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "did NOT reach") || !strings.Contains(out, landCheckScript) {
		t.Fatalf("fixture: the first pass did not block on the check, so nothing here measures its block:\n%s", out)
	}
	// The fixture's positive witness for the mechanism being measured: the
	// cheap filter would let a probe through on every pass, and the replay the
	// pass just ran is what says it must not.
	if !mergesCleanly(repo, mustGit(t, repo, "rev-parse", "HEAD"), pre) {
		t.Fatal("fixture: merge-tree refuses this branch, so the skip would stand on the filter and nothing here measures the record")
	}
	if got, want := probedBase(tr.Repo, tr.Branch), mustGit(t, repo, "rev-parse", "HEAD"); got != want {
		t.Fatalf("fixture: the replay recorded %q as the base it ran against, want %q", got, want)
	}

	pass := func(n int, skip bool) {
		t.Helper()
		d2 := newTestDispatcher(t, d.HB)
		dispatcherErr(t, d2)
		if _, err := d2.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
		out := dispatcherOut(d2)
		if took := strings.Contains(out, "already answered this and is still open"); took != skip {
			if skip {
				t.Fatalf("pass %d re-ran the check over a base a replay already judged — the record is unread, and this branch replays and judges its tree on every pass forever:\n%s", n, out)
			}
			t.Fatalf("pass %d stood on a record taken against a base that has since moved — a base that moved past the disagreement would never be re-judged:\n%s", n, out)
		}
		if head := mustGit(t, tr.Path, "rev-parse", "HEAD"); head != pre {
			t.Fatalf("pass %d left the branch at %s, not the tip the seat committed (%s)", n, abbrevSHA(head), abbrevSHA(pre))
		}
	}
	// The base has not moved, so the record answers and the check does not
	// run at all.
	runs := landCheckRuns(t, counter)
	pass(2, true)
	pass(3, true)
	if n := landCheckRuns(t, counter) - runs; n != 0 {
		t.Errorf("two passes over an unmoved base ran the check %d times, want 0 — that is the launcher's lock held for a verdict already on record", n)
	}

	// The base MOVES, on a path that changes nothing about the
	// disagreement: a new question, worth exactly one probe.
	commitIn(t, repo, "c.txt", "the operator moves on again\n", "main: moved on again")
	pass(4, false)
	if n := landCheckRuns(t, counter) - runs; n == 0 {
		t.Error("the pass after the base moved judged nothing — a base that moved past the disagreement is a question only the replay and the check can answer")
	}
	if got, want := probedBase(tr.Repo, tr.Branch), mustGit(t, repo, "rev-parse", "HEAD"); got != want {
		t.Errorf("the probe recorded %q, want the base it ran against (%q) — a record that lags the probe re-probes on the next pass", got, want)
	}
	// And it is one probe and not a standing licence.
	runs = landCheckRuns(t, counter)
	pass(5, true)
	pass(6, true)
	if n := landCheckRuns(t, counter) - runs; n != 0 {
		t.Errorf("the two passes after the probe ran the check %d times, want 0", n)
	}
	// Six passes over one blocked branch, one handoff — the dedupe every
	// other block in this file's family keeps.
	if n := len(mergeBlockedBeads(t, repo)); n != 1 {
		t.Errorf("six passes over one land-check block left %d handoffs, want 1", n)
	}
	// And main never took the work.
	if _, err := os.Stat(filepath.Join(repo, "a.txt")); err == nil {
		t.Error("the branch's file reached main over six blocked passes")
	}
}

// ─── the decisions the first round of pins left to a reader ──────────────────
//
// The five arms below are ranger-base-sjjf4, out of ranger-base-7urt1's
// attack on this file's own close: every one of them is a decision
// landcheck.go states in prose and nothing compiled. Each names the mutant it
// kills, because a pin over a decision that reds nothing is the shape the
// decision escaped in.

// THE OPT-IN IS THE FILE AND NOT THE BIT (landCheckShipped), which is this
// file's one stated divergence from ADR 0087 D1 ("a repo opts in by shipping
// an EXECUTABLE at …") — and the close that landed it named the literal
// reading as a one-liner the architecture lane may want, so the gap sat
// exactly where the next reader would act.
//
// MUTANT KILLED: `&& fi.Mode().Perm()&0o111 != 0` appended to
// landCheckShipped's return. Nothing else in the file sees it, because
// shipLandCheck chmods every fixture 0755.
//
// NON-REGULAR IS NOT SHIPPED is the other half, and it is the loud direction
// the other way: a FIFO at the path would block the launcher for
// generatorTimeout while holding the fleet lock and answer nothing
// (ranger-base-gs9r's shape one function over), and a directory would be
// exec'd. MUTANT KILLED: `.IsRegular()` dropped for `err == nil`.
func TestLandCheckShippedReadsTheFileAndNotItsMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if landCheckShipped(dir) {
		t.Fatal("a repo with nothing at the path reads as opted in — that is every repo that has not shipped a check")
	}
	path := filepath.Join(dir, landCheckScript)

	// A regular file with no execute bit is an OPT-IN. The mode is left to
	// exec, whose refusal is a "could not answer" that lands with a ⚠ naming
	// the file (D3's bottom row); reading the missing bit as "this repo has
	// no check" is the one failure that silently restores the a2327230 hole
	// — judged nothing, said nothing.
	write(t, path, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if !landCheckShipped(dir) {
		t.Error("a check shipped without the execute bit reads as no check at all — a file at a path with posse's own prefix in it is an opt-in whatever its mode, and the quiet reading is the one that restores the hole")
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if !landCheckShipped(dir) {
		t.Error("the ordinary shape — a regular file with the bit — does not read as shipped")
	}

	// And the two shapes that are NOT an opt-in, for the reason the bit is
	// not asked: both of them would be run.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o755); err != nil {
		t.Skipf("NOTHING MEASURED about the non-regular arm: this filesystem will not hold a FIFO (%v)", err)
	}
	if landCheckShipped(dir) {
		t.Error("an executable FIFO reads as a shipped check — opening it blocks the launcher for the whole generator timeout while it holds the fleet lock, and answers nothing")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if landCheckShipped(dir) {
		t.Error("a DIRECTORY at the path reads as a shipped check, and the launcher would exec it")
	}
}

// The same divergence through a LANDING, which is what makes it a decision
// rather than a predicate: a base whose copy of the check cannot be exec'd
// LANDS, and says so. The loud direction, end to end — ⚠ naming the file,
// nothing recorded, and the work on main.
//
// MUTANT KILLED: the same exec-bit conjunct. With it, landCheckShipped reads
// false, landCheckGate returns all three zero, and the landing is silent —
// which is the a2327230 night with a check in the repo.
func TestABaseWhoseCheckHasNoExecuteBitLandsAndSaysSo(t *testing.T) {
	t.Parallel()
	a, repo, counter := landCheckRepo(t)
	tr := landCheckTree(t, a, repo, "s-1", "a.txt", "the seat's own work\n")
	commitIn(t, repo, "b.txt", "the operator's own work\n", "main: moved on")
	// The BASE's copy only, which is the copy posse runs (D1). The branch is
	// untouched, so nothing here is about whose bytes get judged.
	if err := os.Chmod(filepath.Join(repo, landCheckScript), 0o644); err != nil {
		t.Fatal(err)
	}
	if !landCheckShipped(repo) {
		t.Fatal("fixture: the base reads as shipping no check, so this measures the arm where no check exists")
	}
	before := landCheckRuns(t, counter)

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged {
		t.Fatalf("a check posse could not exec blocked a landing: %s", o.Reason)
	}
	for _, want := range []string{"could not judge", landCheckScript, "permission denied", "landed unjudged"} {
		if !strings.Contains(o.LandCheckWarning, want) {
			t.Errorf("the ⚠ does not say %q — a repo that opted in and got no verdict is the one thing this must not do quietly:\n%q", want, o.LandCheckWarning)
		}
	}
	if n := landCheckRuns(t, counter) - before; n != 0 {
		t.Errorf("the check ran %d times over a file nothing can exec", n)
	}
	if got := recordedLandCheck(t, repo); got != "" {
		t.Errorf("a check that answered nothing recorded %q", got)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "a.txt")); err != nil || string(body) != "the seat's own work\n" {
		t.Errorf("the work did not land (%v)", err)
	}
}

// D4 IS KEYED ON THE TREE AND NOT THE COMMIT, because the check reads BYTES —
// stated in landcheck.go, and true of every fixture either way until this
// one: a fast-forward landing leaves the branch tip and the new base at the
// same commit, so the two keys are the same string and both readings agree.
//
// The fixture that discriminates is a base that MOVES to a commit whose tree
// was already judged — an `--allow-empty` commit here, a revert-and-reapply
// in the field. The tree-keyed lookup hits; the commit-keyed one pays a full
// base probe (checkout --detach, run, restore) for bytes it has already read.
//
// MUTANT KILLED: the `^{tree}` suffix dropped from revTreeID's rev-parse. The
// run count is the discriminator — the record's own first field is written
// and read by the mutated function, so an assertion spelled through revTreeID
// moves with it and measures nothing.
func TestABaseThatMovesToAnAlreadyJudgedTreeIsNotJudgedAgain(t *testing.T) {
	t.Parallel()
	a, repo, tr, counter := landCheckBlockedTree(t)
	before := landCheckRuns(t, counter)

	if o, err := MergeSessionWork(a, tr); err != nil || o.Merged {
		t.Fatalf("fixture: the first attempt was not blocked (%v, %+v)", err, o)
	}
	if first := landCheckRuns(t, counter) - before; first != 2 {
		t.Fatalf("fixture: the first attempt ran the check %d times, want 2 (the replayed tip, then the base nobody had on record)", first)
	}
	base := mustGit(t, repo, "rev-parse", "HEAD")
	// The record is a TREE id, which is readable without revTreeID: it is
	// not the commit the base is at.
	rec := strings.Fields(recordedLandCheck(t, repo))
	if len(rec) != 2 || rec[0] == base {
		t.Fatalf("the record reads %q over a base at %s — D4 keys on the tree, and a record keyed on the commit re-probes a tree it has already read", recordedLandCheck(t, repo), abbrevSHA(base))
	}
	if rec[0] != mustGit(t, repo, "rev-parse", base+"^{tree}") {
		t.Fatalf("the record's key is %q, which is neither the base's commit nor its tree", rec[0])
	}

	// The base moves to a NEW COMMIT with the SAME TREE. Nothing about the
	// bytes the check reads has changed, so nothing is worth re-running.
	mustGit(t, repo, "commit", "-q", "--allow-empty", "-m", "main: a commit that changes no bytes")
	moved := mustGit(t, repo, "rev-parse", "HEAD")
	if moved == base {
		t.Fatal("fixture: the base did not move, so this is the unmoved-base arm one test up")
	}
	if mustGit(t, repo, "rev-parse", moved+"^{tree}") != rec[0] {
		t.Fatal("fixture: the empty commit changed the tree, so the lookup would miss for a reason that is not the key")
	}
	runs := landCheckRuns(t, counter)

	if o, err := MergeSessionWork(a, tr); err != nil || o.Merged {
		t.Fatalf("the second attempt landed a replay the first one refused (%v, %+v)", err, o)
	}
	if n := landCheckRuns(t, counter) - runs; n != 1 {
		t.Errorf("the attempt over a base whose TREE was already judged ran the check %d times, want 1 (the replayed tip alone) — the record is keyed on the tree because the check reads bytes, and a commit-keyed record pays a whole base probe per empty commit, revert-and-reapply or amended base", n)
	}
}

// D5's RESET ASKS THE HEAD FIRST, and landcheck.go calls that "not a
// formality": `reset --hard` moves whatever HEAD names, so over a DETACHED
// session tree it would move HEAD and leave the BRANCH on the replayed tip —
// the undo silently not happening, which is the one outcome worse than not
// attempting it.
//
// IT IS REACHABLE, which is why this is a pin and not a comment:
// baseLandCheck returns the base's verdict off the RECORD before reaching its
// own copy of this guard, so a detached session tree whose base tree is on
// record green with a red replay arrives here detached.
//
// MUTANT KILLED: resetReplay's symbolic-ref guard replaced by a dead branch.
// Both halves red — the refusal sentence the caller lands with, and the
// branch ref that a reset over a detached HEAD leaves standing on the replay.
func TestResetReplayRefusesATreeThatIsNotOnItsOwnBranch(t *testing.T) {
	t.Parallel()
	a, repo, _ := landCheckRepo(t)
	tr := landCheckTree(t, a, repo, "s-1", "a.txt", "the seat's own work\n")
	pre := mustGit(t, tr.Path, "rev-parse", "HEAD")
	// A second commit, so a reset that DID happen would move something and
	// the assertion below is not vacuous.
	commitIn(t, tr.Path, "c.txt", "what a replay would have written\n", "s-1: the replayed tip")
	tip := mustGit(t, tr.Path, "rev-parse", "HEAD")
	mustGit(t, tr.Path, "checkout", "--detach", "-q")
	if _, err := git(tr.Path, "symbolic-ref", "--short", "HEAD"); err == nil {
		t.Fatal("fixture: the session tree is still on its branch, so nothing here measures the detached arm")
	}

	why := resetReplay(tr, pre)
	if why == "" {
		t.Fatal("a reset over a DETACHED session tree reported success — it moved HEAD and left the branch on the replayed tip, which is the undo silently not happening (ADR 0087 D5)")
	}
	for _, want := range []string{AbbrevHome(tr.Path), tr.Branch, "a reset would not move the branch"} {
		if !strings.Contains(why, want) {
			t.Errorf("the refusal does not name %q — the caller lands with this sentence instead of reporting a block it did not make:\n%s", want, why)
		}
	}
	if ref := refSHA(repo, tr.Branch); ref != tip {
		t.Errorf("%s is at %s after a refused reset, want %s", tr.Branch, abbrevSHA(ref), abbrevSHA(tip))
	}
	if head := mustGit(t, tr.Path, "rev-parse", "HEAD"); head != tip {
		t.Errorf("the detached HEAD moved to %s, want %s — the refusal is "+
			"that nothing was touched", abbrevSHA(head), abbrevSHA(tip))
	}

	// baseLandCheck's own copy of the same guard, unpinned for the same
	// reason: the one tree the launcher may write is the session tree, and
	// nothing may detach a tree whose HEAD is not where this function would
	// put it back. A container-tier session works detached on purpose.
	v, said := baseLandCheck(tr, tip)
	if v != landUnanswered {
		t.Errorf("the base probe answered %s over a tree it must not detach — restoring is the half that must be guaranteed, and it cannot be", v)
	}
	for _, want := range []string{AbbrevHome(tr.Path), tr.Branch, "could not be checked out there to judge it"} {
		if !strings.Contains(said, want) {
			t.Errorf("the base probe's cause does not name %q:\n%s", want, said)
		}
	}
	if head := mustGit(t, tr.Path, "rev-parse", "HEAD"); head != tip {
		t.Errorf("the refused base probe moved the tree to %s", abbrevSHA(head))
	}
}

// AND THE ⚠ REACHES A SURFACE. The warning was pinned on the MergeOutcome
// FIELD alone — four print sites and no test over any of them, so deleting
// the block that prints it red nothing (ranger-base-sjjf4 finding 6). Two of
// the four are cheap to reach and are reached here: the operator's own
// `posse worktrees --land`, and the kill's line set.
//
// The other two (dispatch.go, landsweep.go) print the same field from the
// same outcome one call later; they are named on the bead rather than pinned,
// because the gap they share is this package's shape and not this decision's
// — RegeneratedNote, the sibling note these four were modelled on, is
// asserted at no print surface either.
func TestTheLandCheckWarningReachesTheSurfacesThatOweIt(t *testing.T) {
	t.Parallel()
	// A kill's line set (KillLanding.Lines): the fourth sentence, in the
	// order it happened — after the generated-file note, before what became
	// of the tree.
	l := &KillLanding{Merge: MergeOutcome{
		Branch:           "posse/s-1",
		Regenerated:      []string{"docs/notes.d/README.md"},
		LandCheckWarning: "the land check could not judge the replay (fixture)",
	}}
	lines := l.Lines()
	at := -1
	for i, ln := range lines {
		if strings.Contains(ln, l.Merge.LandCheckWarning) {
			at = i
		}
	}
	if at < 0 {
		t.Errorf("a kill that landed a replay nothing judged green said nothing about it — the kill's line set is the one record of it:\n%v", lines)
	} else if at == 0 || !strings.Contains(lines[at-1], "reproduced on") {
		t.Errorf("the ⚠ is not in the order it happened (the generated-file note, then the check):\n%v", lines)
	}

	// And the operator's own landing command, which prints it beside the
	// outcome it belongs to.
	a, repo, _ := landCheckRepo(t)
	landCheckTree(t, a, repo, "s-1", "a.txt", "the seat's own work\n")
	commitIn(t, repo, "broken", "somebody else broke the base\n", "main: moved on, red")
	var out bytes.Buffer
	// force, because the gate `posse worktrees --land` reads is the branch's
	// bead record and this fixture has none — which is unaccountedFor's
	// subject and not this one's.
	if err := LandSessionTrees(&out, a, []string{repo}, true); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "already red") || !strings.Contains(got, landCheckScript) {
		t.Errorf("`posse worktrees --land` landed over a red base and said nothing about it — the human reading this terminal is the whole audience this surface has:\n%s", got)
	}
}
