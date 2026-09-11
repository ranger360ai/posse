//go:build posse_arm2

package posse

// ranger-base-yct7l (the verify of ranger-base-9u5zy): what
// standingMergeBlock's skip actually buys, and what it costs.
//
// THE SKIP'S OWN CLAIM IS ABOUT THE TREE, AND logs/HEAD IS NOT THE TREE.
// TestSweepStopsTouchingTheTreeOnceTheBlockStands (mergeblocked_test.go)
// pins the mtime of ONE file, the one the rebase probe wrote. What ADR
// 0058's fact 4 reads is lastTreeWrite — the newest mtime among ALL the
// files of the session's git dir — so a pass that stopped writing logs/HEAD
// and kept writing the index would pass that pin and still hold the grace
// clock open forever. MEASURED here 2026-09-10 (macOS 26.4.1/APFS, git
// 2.50.1), 10 runs of this fixture: the index moves once more after the
// block stands — the stat-cache settle of the first `git status` following
// the last aborted rebase — in 10/10, and once more again in 1/10, and
// never after that. So the property holds, with a one-to-two-pass lag
// neither the commit message nor the skip's comment mentions, and the pin
// below measures the property and that bound rather than their proxy.
//
// AND THE COST, WHICH WAS THE FINDING AND IS NOW THE REQUIREMENT
// (ranger-base-ejju3 fixed it, and these tests are inverted as this file's
// header said whoever fixed it would). The skip was keyed on the BRANCH not
// having moved, while every reason a merge-back blocks is a statement about
// the BASE. Once a block stood, a base that moved back under the branch was
// never re-read: work that would now land did not land, and work that had
// since reached the base under another sha was reported a strand forever.
// standingMergeBlock now re-reads the operands — the base, and the tree's
// dirt — on every pass, and honours the block only while none of them can
// have changed the answer.
//
// THE FOURTH OPERAND CAME LAST AND ON ITS OWN (ranger-base-c6ohn), because it
// is the one no reading can answer: a base that moved FORWARD past the
// conflict. It is asked as a cheap filter in the object store plus a record of
// the base the replay last ran against, so the answer costs one tree write per
// base MOVEMENT — TestAConflictRevertedOnTheBaseIsProbedAgainAndLands is the
// requirement and TestAFilterThatDisagreesWithTheReplayProbesOncePerBaseMove
// is the bound on it. With that, every reason MergeSessionWork blocks on has
// a pass that can see it stop being true.
//
// THE TWO PROPERTIES ARE IN TENSION AND BOTH ARE PINNED HERE, which is the
// point of keeping them in one file: the quiet tests above say the tree is
// not written over a block that still stands, and the tests below say the
// block stops standing the moment the base or the dirt says it might. A fix
// to either that breaks the other is the same bug with the operands swapped
// (ranger-base-9u5zy one way, ranger-base-ejju3 the other).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gitDirWrites is every file of the session's git dir and when it was last
// written — THE FILES and not the directory, for lastTreeWrite's own reason
// (retire.go: `index.lock` moves the directory on every `git status` and no
// tree on the board would ever read as quiet).
func gitDirWrites(t *testing.T, tr *SessionTree) map[string]time.Time {
	t.Helper()
	gd := mustGit(t, tr.Path, "rev-parse", "--absolute-git-dir")
	m := map[string]time.Time{}
	if err := filepath.Walk(gd, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		m[strings.TrimPrefix(p, gd)] = fi.ModTime()
		return nil
	}); err != nil {
		t.Fatalf("could not read %s: %v", gd, err)
	}
	if len(m) == 0 {
		t.Fatalf("fixture: %s holds no readable file, so nothing here measures a quiet tree", gd)
	}
	return m
}

// theWritesStop is the reading fact 4 takes, asked over a run of passes.
// The settle window is passes 2 and 3: the first `git status` after the last
// aborted rebase rewrites the index whose stat cache that rebase left dirty,
// and on a loaded box that has been seen to take one pass more. Every pass
// after the window must write NOTHING — and THAT is the property, because
// the probe ranger-base-9u5zy removed wrote on every pass forever, so no
// settle window would ever have saved it and a grace clock can only reach
// zero once the writing ends. Read over every file of the session's git dir
// and over lastTreeWrite itself, which is the reading the retire takes.
func theWritesStop(t *testing.T, tr *SessionTree, pass func(n int)) {
	t.Helper()
	const settle, quiet = 3, 4
	prev, prevLast := gitDirWrites(t, tr), mustLastTreeWrite(t, tr)
	for n := 2; n <= settle+quiet; n++ {
		pass(n)
		cur, curLast := gitDirWrites(t, tr), mustLastTreeWrite(t, tr)
		var moved []string
		for p, ts := range cur {
			if b, had := prev[p]; !had {
				moved = append(moved, fmt.Sprintf("%s CREATED (%s)", p, ts))
			} else if !b.Equal(ts) {
				moved = append(moved, fmt.Sprintf("%s (%s -> %s)", p, b, ts))
			}
		}
		if wrote := len(moved) > 0 || !curLast.Equal(prevLast); wrote && n > settle {
			t.Errorf("pass %d wrote the tree of an unmoved, already-blocked branch: %s (lastTreeWrite %s -> %s) — that is past the %d-pass stat-cache settle, and ADR 0058's fact 4 reads exactly this, so a tree written at the sweep's own cadence never goes quiet",
				n, strings.Join(moved, ", "), prevLast, curLast, settle)
		}
		prev, prevLast = cur, curLast
	}
}

// THE WRITE THE SETTLE WINDOW ABOVE IS AN ALLOWANCE FOR, asked directly and
// answered the same on every platform (ranger-base-a8tqz). theWritesStop
// reads the property through the sweep, which makes it a reading of the
// whole pass and leaves it sensitive to how fast the box runs them: the
// window it tolerates was measured on an idle mac, and on ubuntu under the
// full suite the pins around it red — not on a regression, on `git status`
// refreshing a racy index and writing it back, which MEASURED can repeat
// without bound (three trials of the bare command over an untouched tree:
// quiet after pass 3, after pass 6, and in one trial never quiet at all in
// eight; dirtyPaths' header carries the numbers). So the window can never be
// widened into correctness, and the sweep is the wrong altitude to ask from.
// This asks the one question underneath, where the answer does not depend on
// the box: dirtyPaths over a tree whose last git operation was an aborted
// rebase must not write the index — not on a loaded runner, not once.
func TestDirtyPathsDoesNotWriteTheIndexOfABlockedTree(t *testing.T) {
	t.Parallel()
	d, _, tr := nurlBlocked(t)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	before, ok := gitDirWrites(t, tr)["/index"]
	if !ok {
		t.Fatal("fixture: the session tree has no .git/index, so nothing here measures the refresh")
	}
	for n := 1; n <= 8; n++ {
		dirtyPaths(tr.Path)
		if after := gitDirWrites(t, tr)["/index"]; !after.Equal(before) {
			t.Fatalf("dirtyPaths call %d wrote the index of a tree nobody touched (%s -> %s) — `git status` refreshed the stat cache an aborted rebase left racy and wrote it back, and lastTreeWrite reads exactly that mtime, so the retire grace clock restarts every sweep pass: ranger-base-9u5zy's bug through a second writer. --no-optional-locks is what stops it (worktree.go, dirtyPaths)",
				n, before, after)
		}
	}
}

// The property ADR 0058's fact 4 actually reads, over the blocked branch
// that motivated ranger-base-9u5zy. MEASURED 10 runs of this fixture on an
// idle box, 2026-09-10: the git dir was written on the pass after the block
// stood in 10/10, on the pass after THAT in 1/10, and never later; a run
// under the full suite took the second pass too, which is why the window
// theWritesStop allows is three and not two.
func TestTheWholeGitDirGoesQuietOnceTheBlockStands(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlBlocked(t)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if n := len(mergeBlockedBeads(t, repo)); n != 1 {
		t.Fatalf("fixture: the first pass filed %d handoffs, want 1", n)
	}
	pass := func(n int) {
		d2 := newTestDispatcher(t, d.HB)
		dispatcherErr(t, d2)
		if _, err := d2.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
		if out := dispatcherOut(d2); !strings.Contains(out, "already answered this and is still open") {
			t.Fatalf("fixture: pass %d did not take the skip, so nothing here measures it:\n%s", n, out)
		}
	}
	theWritesStop(t, tr, pass)
}

// The other reader the skip leaves in the path, and the claim its comment
// makes about it: `git status` runs on EVERY pass over a blocked tree,
// deliberately (ADR 0041 §1-§2 — the persona's uncommitted work is not the
// rebase probe), and the comment asserts it "does not reproduce the write
// the header above measures". Nothing measured that, and the tree it
// matters most for is the one with dirt in it — the shape ranger-base-wj7e9
// preserved. Held to the same window as the clean tree above: `git status`
// over modifications it reports on every pass still writes nothing once the
// stat cache has settled.
func TestADirtyBlockedTreeGoesQuietTooAlthoughEveryPassReadsIt(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", true)
	write(t, filepath.Join(repo, "fake-show.json"), `[{"id":"a-1","status":"closed","assignee":"ranger"}]`)
	commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
	// The persona's uncommitted draft, left behind by the close.
	write(t, filepath.Join(tr.Path, "draft.txt"), "half a thought\n")
	dispatcherErr(t, d)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "uncommitted changes") {
		t.Fatalf("fixture: the first pass did not block on the dirt, so nothing here measures that arm:\n%s", out)
	}
	theWritesStop(t, tr, func(n int) {
		d2 := newTestDispatcher(t, d.HB)
		dispatcherErr(t, d2)
		if _, err := d2.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
		out := dispatcherOut(d2)
		if !strings.Contains(out, "already answered this and is still open") {
			t.Fatalf("fixture: pass %d did not take the skip, so nothing here measures it:\n%s", n, out)
		}
		// THE WRITE A SKIPPED PASS MUST NOT MAKE (ranger-base-zyrr4). The
		// skip hands noteMergeBlocked an outcome whose Reason is the
		// sentence above — a report about the RECORD, not a reading of the
		// obstacle — and it names no dirt. Restating the handoff from it
		// replaces this block's real reason with a line about itself, once
		// and permanently: the description is where blockStillStands reads
		// which obstacle this was, so a body that says "already answered"
		// has forgotten it was the dirt, and cleaning the dirt afterwards
		// then un-skips nothing and the branch that would land never lands.
		// That is ranger-base-ejju3's defect restored by the fix for it.
		// MergeOutcome.Standing is what stops it; the assertion after this
		// loop is the same fact read off the store.
		if strings.Contains(out, "restated") {
			t.Errorf("pass %d restated the handoff over a tree whose dirt is exactly where it was:\n%s", n, out)
		}
	})
	if desc := mergeBlockedDescription(t, repo); !strings.Contains(desc, dirtyBlockMark) {
		t.Errorf("the handoff no longer names the dirt that is still in the tree — the skip's own sentence was written over the obstacle:\n%s", desc)
	}
}

func mustLastTreeWrite(t *testing.T, tr *SessionTree) time.Time {
	t.Helper()
	ts, ok := lastTreeWrite(tr)
	if !ok {
		t.Fatal("lastTreeWrite could not be read, so fact 4 is unanswerable and nothing here measures it")
	}
	return ts
}

// THE REQUIREMENT (ranger-base-ejju3, inverted from the pin ranger-base-yct7l
// left here). The block said "main moved on and replaying conflicts". main
// moves back — the operator resets or reverts what conflicted, which is one
// of the two ways a human answers this handoff — and the branch, untouched,
// now fast-forwards. The pass must land it, which is what it did before
// ranger-base-9u5zy and is the sequence MergeSessionWork's own header
// measures on ranger-base-c02a/59fs: a bead filed at 09:57:11 and the same
// untouched branch landed at 09:57:26.
func TestABlockThatStandsIsReconsideredWhenTheBaseMovesBack(t *testing.T) {
	t.Parallel()
	d, repo, _ := nurlBlocked(t)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dispatcherOut(d), "did NOT reach") {
		t.Fatalf("fixture: the first pass was not blocked:\n%s", dispatcherOut(d))
	}
	mustGit(t, repo, "reset", "--hard", "HEAD~1")

	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)
	if strings.Contains(out, "already answered this and is still open") {
		t.Errorf("the block was honoured over a base that moved back under it — the skip's key is the branch again, and a branch that would now land never lands:\n%s", out)
	}
	if !strings.Contains(out, "fast-forwarded") {
		t.Errorf("the pass did not land a branch that now fast-forwards:\n%s", out)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "fix.txt")); err != nil || string(body) != "the persona's work\n" {
		t.Errorf("a closed bead's work is still not on main after the base moved back (%v)\n%s", err, out)
	}
}

// THE REQUIREMENT, the same key read from the other side (ranger-base-ejju3):
// the branch's commit reaches the base under ANOTHER sha after the block
// stands. That is the ≡ arm (equivalentOnBase, ranger-base-g2xf) and it is
// the arm that ENDS a strand — Merged with nothing left to land, which is
// what lets the tree retire by the measured path ADR 0058 D2 wants. Reported
// as a strand forever while the skip was keyed on the branch alone.
func TestWorkThatReachedTheBaseUnderAnotherShaEndsTheStrand(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlBlocked(t)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	head := mustGit(t, tr.Path, "rev-parse", "HEAD")
	mustGit(t, repo, "reset", "--hard", "HEAD~1")
	// ANOTHER SHA IS THE PREMISE, SO THE FIXTURE HAS TO GUARANTEE ONE RATHER
	// THAN HOPE FOR IT (ranger-base-f5l0d). This line was `cherry-pick head`,
	// and a cherry-pick back onto the commit's own parent reproduces every
	// field of that commit — tree, parent, author, author date, message —
	// leaving the COMMITTER date as the only difference, and git records that
	// to the second. MEASURED 2026-09-10 (macOS 26.4.1/APFS, git 2.50.1):
	// replayed inside the second the original was committed in, the replay IS
	// the original, byte for byte and sha for sha. Then `main..<branch>` is 0,
	// nothingToLand is true, and the sweep says nothing about this tree at
	// all — so the pass under test never ran, both assertions below read a
	// bare "no ready work", and neither arm of this file's subject was
	// involved in the answer.
	//
	// That is the whole of ci.yml's red on main from 26090db3 to 1e1d08d0,
	// seven runs: green wherever a pass took over a second (a loaded mac, 2.0s
	// local), red on a runner fast enough to fit the fixture inside one
	// (0.26-1.07s, ubuntu and macos alike), in and out of the suite by nothing
	// but where the second boundary fell. Reproduced on demand by pinning the
	// clock the fixture was resting on: `GIT_COMMITTER_DATE=<fixed> go test`
	// reds this test here, with the same message CI printed.
	//
	// NOT A NEW HAZARD, WHICH IS THE OTHER HALF OF WHY THIS ONE IS WORTH
	// WRITING DOWN: every other fixture that lands a branch's patch on the
	// base already defends against it, in as many words. mergebackequivnote_
	// test.go moves the base first, "or the pick rebuilds the identical commit
	// object and the base reaches it by sha with nothing measured"; worktree_
	// test.go's four arms do the same and then assert `rev-list --count
	// main..<branch>` is 1, "without this the retiring arms could be passing
	// because the guard was never reached" (both ranger-base-g2xf). This test
	// arrived later (ranger-base-ejju3) and carried neither.
	//
	// A hand-landing of the same patch under its own subject is the same ≡ arm
	// and cannot collide with anything: `git cherry` measures patch-ids, a
	// message is not in a patch-id, and a differing message is a differing
	// sha whatever the clock says. It stays off `cherry-pick -x` deliberately —
	// the trailer is equivalentOnBase's SECOND arm and is pinned elsewhere
	// (worktree_test.go), and the arm this test is about is the first.
	commitIn(t, repo, "fix.txt", "the persona's work\n", "main: the same fix, landed by hand")
	// And the assertion those two fixtures pair with the base move, which is
	// what turns the degeneracy above from seven silent runs into one loud
	// one: a pin on the ≡ arm is worth nothing if the base and the branch are
	// the same commit.
	if tip := mustGit(t, repo, "rev-parse", "HEAD"); tip == head {
		t.Fatalf("fixture: %s is the branch's own tip, so no work reached the base under ANOTHER sha and nothing here measures the ≡ arm", abbrevSHA(tip))
	}

	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)
	if strings.Contains(out, "did NOT reach") {
		t.Errorf("work already on main under another sha is still reported a strand:\n%s", out)
	}
	if !strings.Contains(out, "already on main under other sha(s)") {
		t.Errorf("the pass did not say the base already holds this work:\n%s", out)
	}
}

// THE OTHER OPERAND, and the findings bead's second trigger for it
// (ranger-base-ejju3): the block was "main moved on and <tree> has
// uncommitted changes, which git will not rebase over". The dirt is then
// cleaned WITHOUT a commit — dirtyPaths reads empty, the branch is where it
// was — and the rebase that would now succeed must be attempted. The base
// here moves on a DIFFERENT path, so nothing conflicts and the only thing
// that ever blocked this branch is the dirt.
func TestCleaningTheDirtWithoutCommittingIsReconsidered(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", true)
	write(t, filepath.Join(repo, "fake-show.json"), `[{"id":"a-1","status":"closed","assignee":"ranger"}]`)
	commitIn(t, repo, "elsewhere.txt", "the operator's own line\n", "main: moved on")
	draft := filepath.Join(tr.Path, "draft.txt")
	write(t, draft, "half a thought\n")
	dispatcherErr(t, d)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "uncommitted changes") {
		t.Fatalf("fixture: the first pass did not block on the dirt:\n%s", out)
	}
	if err := os.Remove(draft); err != nil {
		t.Fatal(err)
	}

	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)
	if strings.Contains(out, "already answered this and is still open") {
		t.Errorf("the block was honoured over a tree whose dirt is gone — the rebase that would now succeed is never attempted:\n%s", out)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "fix.txt")); err != nil || string(body) != "the persona's work\n" {
		t.Errorf("the work did not land after the dirt was cleaned (%v)\n%s", err, out)
	}
}

// THE REQUIREMENT, inverted from the pin ranger-base-ejju3 left here against
// today's behaviour — the fourth operand, and the last one (ranger-base-c6ohn).
//
// The operator answers the handoff by REVERTING the conflicting commit with a
// new one rather than by resetting main. The base is then not an ancestor of
// the branch (no fast-forward), the work is on main under no sha at all (no
// equivalence), the tree is clean (not the dirt arm) — so every operand the
// three arms above read says the block stands, and only the replay can say the
// conflict is gone. The replay in the session tree is the write ADR 0058's
// fact 4 reads, so it is not run on a hunch: `git merge-tree` answers the
// merge in the object store with no worktree at all (mergesCleanly), and the
// base it answered about is recorded when the replay runs (probedBaseKey), so
// the branch where merge-tree and the rebase disagree costs one probe per base
// MOVEMENT rather than one per pass. The test below this one is that half.
//
// What must happen here is what happened before ranger-base-9u5zy and what
// MergeSessionWork's own header measures on ranger-base-c02a/59fs: the
// untouched branch lands on the pass after the operator's answer, with nobody
// closing the handoff by hand.
func TestAConflictRevertedOnTheBaseIsProbedAgainAndLands(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlBlocked(t)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dispatcherOut(d), "did NOT reach") {
		t.Fatalf("fixture: the first pass was not blocked:\n%s", dispatcherOut(d))
	}
	// The fixture's positive witness for the OTHER half: the pass that
	// actually replayed recorded the base it replayed onto, or the record
	// below can only ever be empty and nothing here measures it.
	if got, want := probedBase(tr.Repo, tr.Branch), mustGit(t, repo, "rev-parse", "HEAD"); got != want {
		t.Fatalf("fixture: the replay recorded %q as the base it ran against, want %q — the record the filter is gated on was never written", got, want)
	}
	// REVERTED and not reset: the conflicting commit stays in main's history
	// and a new commit undoes it, which is the shape no arm but the replay can
	// read. `--no-commit` plus the path-limited commit, because a plain
	// `git revert` names no paths and the crew's commit wall refuses it.
	mustGit(t, repo, "revert", "--no-commit", "HEAD")
	mustGit(t, repo, "commit", "-q", "-m", "main: revert the conflicting line", "--", "fix.txt")
	if reaches(repo, mustGit(t, tr.Path, "rev-parse", "HEAD"), "main") {
		t.Fatal("fixture: main is an ancestor of the branch, so this would land by fast-forward and nothing here measures the replay arm")
	}

	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)
	if strings.Contains(out, "already answered this and is still open") {
		t.Errorf("the block was honoured over a base that moved past the conflict — a branch that would now land never lands:\n%s", out)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "fix.txt")); err != nil || string(body) != "the persona's work\n" {
		t.Errorf("a closed bead's work is still not on main after the conflict was reverted (%v)\n%s", err, out)
	}
}

// THE OTHER HALF OF THE FILTER, and the reason it is a filter behind a record
// rather than a filter alone (ranger-base-c6ohn). `git merge-tree` answers
// about the WHOLE branch merged at once; the rebase replays commit by commit,
// and the two disagree over a branch whose own commits cancel out — here the
// second commit puts fix.txt back to exactly what main holds, so the merge is
// clean while replaying the FIRST commit is the same add/add conflict it
// always was.
//
// On such a branch a filter with no memory lets the replay through on every
// pass, and that is ranger-base-9u5zy's every-pass tree write with the
// operands swapped — silent, because the handoff is deduped, so nothing is
// filed while lastTreeWrite advances at the sweep's own cadence forever and
// ADR 0058 D2 is never reached. What the record buys is ONE probe per base
// movement: the base moves, the filter says clean, the replay runs and
// conflicts and writes down the base it ran against, and every pass after that
// stands on the record until the base moves again.
func TestAFilterThatDisagreesWithTheReplayProbesOncePerBaseMove(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", true)
	write(t, filepath.Join(repo, "fake-show.json"), `[{"id":"a-1","status":"closed","assignee":"ranger"}]`)
	commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
	// The branch's second commit, which nets its own first one out: the tip's
	// fix.txt is byte-identical to main's, so the whole-branch merge is clean.
	commitIn(t, tr.Path, "fix.txt", "the operator's line\n", "a-1: the operator's line after all")
	dispatcherErr(t, d)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "did NOT reach") {
		t.Fatalf("fixture: the first pass was not blocked, so nothing here measures the disagreement:\n%s", out)
	}
	// The fixture's positive witness, both halves of it: the cheap filter says
	// this branch would merge clean, and the replay the pass just ran says it
	// does not. Without this the test could pass over a branch the filter
	// refuses, where no probe was ever on the table.
	head := mustGit(t, tr.Path, "rev-parse", "HEAD")
	if !mergesCleanly(repo, mustGit(t, repo, "rev-parse", "HEAD"), head) {
		t.Fatal("fixture: merge-tree refuses this branch, so the filter would never let a probe through and nothing here measures the record")
	}

	// pass runs one sweep and says whether it took the skip. skip=true is
	// "the record answered and no replay ran".
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
				t.Fatalf("pass %d re-asked a base a replay already ran against — the filter is unrecorded again, and this branch writes its tree on every pass forever:\n%s", n, out)
			}
			t.Fatalf("pass %d stood on a record taken against a base that has since moved — a base that moved past the conflict would never be re-read:\n%s", n, out)
		}
	}
	// quietPass is the same with the reading ADR 0058's fact 4 takes wrapped
	// around it. Taken only after the stat-cache settle the file's header
	// measures has been spent, for theWritesStop's reason: the first `git
	// status` runs after the last aborted rebase can still write the index,
	// and that window is an allowance, never the property.
	quietPass := func(n int) {
		t.Helper()
		prev, prevLast := gitDirWrites(t, tr), mustLastTreeWrite(t, tr)
		pass(n, true)
		cur, curLast := gitDirWrites(t, tr), mustLastTreeWrite(t, tr)
		for p, ts := range cur {
			if b, had := prev[p]; !had || !b.Equal(ts) {
				t.Errorf("pass %d wrote %s over a tree whose base nobody moved (%s -> %s)", n, p, b, ts)
			}
		}
		if !curLast.Equal(prevLast) {
			t.Errorf("pass %d moved lastTreeWrite over a tree whose base nobody moved (%s -> %s)", n, prevLast, curLast)
		}
	}
	// The base has NOT moved, so the record answers: the one probe this base
	// was worth was pass 1's, and by pass 4 the tree is quiet.
	pass(2, true)
	pass(3, true)
	quietPass(4)

	// The base MOVES — on a path nothing here touches, so the filter still
	// says clean and the replay still conflicts. That is a new question and it
	// is worth exactly one probe.
	commitIn(t, repo, "elsewhere.txt", "the operator's own line\n", "main: moved on")
	pass(5, false)
	if got, want := probedBase(tr.Repo, tr.Branch), mustGit(t, repo, "rev-parse", "HEAD"); got != want {
		t.Errorf("the probe recorded %q, want the base it ran against (%q) — a record that lags the probe re-probes on the next pass", got, want)
	}
	// And it is one probe and not a standing licence: the base is where pass 5
	// left it, so the record answers again and the tree goes quiet again.
	pass(6, true)
	pass(7, true)
	quietPass(8)

	// The whole run costs the persona one handoff, as every other pass over
	// this file's fixtures does.
	if n := len(mergeBlockedBeads(t, repo)); n != 1 {
		t.Errorf("six passes over one blocked branch left %d handoffs, want 1", n)
	}
}

// AND THE OTHER HALF OF THE TENSION, which is what keeps the fix above from
// being ranger-base-9u5zy's bug again: a base that moves and STILL conflicts
// must leave the block standing and the tree untouched. main moves on every
// few minutes in the field, so a skip re-keyed on the base's sha alone would
// put the every-pass rebase probe — and ADR 0058's unreachable fact 4 — back
// exactly as they were.
func TestABaseThatMovesAndStillConflictsKeepsTheBlockAndTheTreeQuiet(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlBlocked(t)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dispatcherOut(d), "did NOT reach") {
		t.Fatalf("fixture: the first pass was not blocked:\n%s", dispatcherOut(d))
	}
	// Two more passes first, so the stat-cache settle the header measures is
	// spent before anything here is read (settle = 3 passes).
	for n := 2; n <= 3; n++ {
		d2 := newTestDispatcher(t, d.HB)
		dispatcherErr(t, d2)
		if _, err := d2.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
	}
	prev, prevLast := gitDirWrites(t, tr), mustLastTreeWrite(t, tr)
	for n := 4; n <= 6; n++ {
		// main moves on every pass, and every move still conflicts with the
		// branch's own edit of the same path.
		commitIn(t, repo, "fix.txt", fmt.Sprintf("the operator's line %d\n", n), "main: conflicting again")
		d2 := newTestDispatcher(t, d.HB)
		dispatcherErr(t, d2)
		if _, err := d2.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
		if out := dispatcherOut(d2); !strings.Contains(out, "already answered this and is still open") {
			t.Fatalf("pass %d re-asked a question whose answer cannot have changed — the base moved, and it still conflicts:\n%s", n, out)
		}
		cur, curLast := gitDirWrites(t, tr), mustLastTreeWrite(t, tr)
		for p, ts := range cur {
			if b, had := prev[p]; !had || !b.Equal(ts) {
				t.Errorf("pass %d wrote %s over a still-blocked tree nobody touched (%s -> %s) — re-reading the base must not cost the tree write ranger-base-9u5zy removed", n, p, b, ts)
			}
		}
		if !curLast.Equal(prevLast) {
			t.Errorf("pass %d moved lastTreeWrite over a still-blocked tree (%s -> %s)", n, prevLast, curLast)
		}
		prev, prevLast = cur, curLast
	}
	if n := len(mergeBlockedBeads(t, repo)); n != 1 {
		t.Errorf("six passes over one blocked branch left %d handoffs, want 1", n)
	}
}

// The skip's own escape hatch, which its doc claims and no test measured: an
// OPEN block whose pin cannot be read is a question nobody can answer from
// the record, so the probe runs for real — and noteMergeBlocked's OPEN arm
// re-pins, so the pass after it is quiet again. This is every block filed
// before pinBlockedWork existed, and they are the oldest ones.
func TestAnOpenBlockWithNoPinProbesOnceAndSelfHeals(t *testing.T) {
	t.Parallel()
	d, _, tr := nurlBlocked(t)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	mustGit(t, tr.Repo, "update-ref", "-d", blockedPinRef(tr.Branch))

	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d2); strings.Contains(out, "already answered this and is still open") {
		t.Errorf("a block with no pin skipped the probe anyway — an OPEN block's pin is its only evidence that the branch has not moved:\n%s", out)
	}
	pin, err := git(tr.Repo, "rev-parse", "--verify", "--quiet", blockedPinRef(tr.Branch))
	if err != nil || pin == "" {
		t.Fatalf("the pin did not self-heal (%v, %q), so this branch pays the tree-writing probe on every pass forever", err, pin)
	}
	if head := mustGit(t, tr.Path, "rev-parse", "HEAD"); pin != head {
		t.Errorf("the healed pin is at %s, the tree's head is %s", pin, head)
	}
	d3 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d3)
	if _, err := d3.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d3); !strings.Contains(out, "already answered this and is still open") {
		t.Errorf("the pass after the self-heal probed the tree again:\n%s", out)
	}
}

// THE REQUIREMENT, inverted from the pin ranger-base-z8xx1 filed against
// today's behaviour (196ab095, carried onto this branch so the defect and its
// fix read as one history). What that pin measured: the dirt arm reads the
// obstacle out of prior.Why — the OPEN handoff's description — and
// noteMergeBlocked never rewrote one, so a block first filed on the dirt
// carried dirtyBlockMark for the rest of its life whatever the obstacle became.
// Clean the dirt off such a tree while the base still CONFLICTS and every
// operand read un-skip — no fast-forward, no equivalence, no dirt,
// dirtyBlockMark still in Why — so the replay ran on EVERY pass and conflicted
// on every one of them. That is ranger-base-9u5zy's every-pass tree write, and
// nothing surfaced it: the handoff is deduped, so each pass filed nothing while
// lastTreeWrite advanced at the sweep's own cadence forever, the tree never
// went quiet, and ADR 0058 D2 was never reached.
//
// WHAT THE FIX BUYS AND WHAT IT COSTS (ranger-base-zyrr4). restateMergeBlocked
// rewrites an open block's description when the obstacle stops being the dirt,
// so the un-skip is SELF-LIMITING rather than removed: pass 2 probes for real,
// comes back with the conflict, restates the handoff to it, and every pass
// after that reads a body that no longer names the dirt and stands. One probe
// per obstacle change, not one per pass — which is why this is theWritesStop
// (the writing ENDS) and not "the tree is never written again".
//
// THE COST IS PAID IN BD AND NOT IN THE TREE, and this test pins both halves:
// the handoff count stays at 1 (the dedupe is untouched — no bead is re-filed),
// and the one bead's description now names the conflict instead of dirt that
// is not there. That second assertion is also the human-facing half of the
// bug: before this, the bead a persona opened said "uncommitted changes" over
// a tree that had none.
//
// TestCleaningTheDirtWithoutCommittingIsReconsidered is the arm that must keep
// working, and it is the same shape with the base moved on a DIFFERENT path:
// there the one un-skip LANDS the work, so there is no second pass to limit.
// The difference between the two is only whether the replay conflicts, which
// is why the un-skip cannot be conditioned on the dirt alone.
func TestTheDirtArmProbesOnceOnceTheDirtIsGoneAndTheBaseStillConflicts(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", true)
	write(t, filepath.Join(repo, "fake-show.json"), `[{"id":"a-1","status":"closed","assignee":"ranger"}]`)
	// The base moves on the SAME path the branch touches, so the replay this
	// un-skip lets through conflicts.
	commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
	draft := filepath.Join(tr.Path, "draft.txt")
	write(t, draft, "half a thought\n")
	dispatcherErr(t, d)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	// The fixture's positive witness: the block has to be the DIRT one, or
	// this measures some other arm entirely.
	if out := dispatcherOut(d); !strings.Contains(out, dirtyBlockMark) {
		t.Fatalf("fixture: pass 1 did not block on the dirt, so nothing here measures the dirt arm:\n%s", out)
	}
	if filed := mergeBlockedBeads(t, repo); len(filed) != 1 {
		t.Fatalf("fixture: pass 1 filed %d handoffs, want 1", len(filed))
	}
	if err := os.Remove(draft); err != nil {
		t.Fatal(err)
	}

	// Pass 2 is the probe the un-skip is for and it writes the tree; it is
	// inside theWritesStop's settle window, which is what makes "one probe"
	// and "the stat-cache settles" one reading rather than two.
	restated := ""
	theWritesStop(t, tr, func(n int) {
		d2 := newTestDispatcher(t, d.HB)
		dispatcherErr(t, d2)
		if _, err := d2.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
		out := dispatcherOut(d2)
		if strings.Contains(out, "not re-filed, and restated") {
			if restated != "" {
				t.Errorf("the handoff was restated twice (passes %s and %d) — one obstacle change, one bd write:\n%s", restated, n, out)
			}
			restated = fmt.Sprint(n)
		}
	})
	if restated != "2" {
		t.Errorf("the open handoff was restated on pass %q, want pass 2 — the probe the un-skip allows must correct the body it read, or nothing ends it", restated)
	}

	// The dedupe is untouched: the cost of the fix is one bd write, not a
	// second handoff for a persona to read.
	again := mergeBlockedBeads(t, repo)
	if len(again) != 1 {
		t.Fatalf("the passes left %d handoffs, want the one filed on pass 1", len(again))
	}
	desc, _ := again[0]["description"].(string)
	if strings.Contains(desc, dirtyBlockMark) {
		t.Errorf("the open handoff still says the block is the dirt over a tree that is clean — that body is what blockStillStands re-reads, so the probe never stops:\n%s", desc)
	}
	if !strings.Contains(desc, "conflicts") {
		t.Errorf("the restated handoff does not name the obstacle that is actually there — a persona opening it is told to go clear paths that do not exist:\n%s", desc)
	}
}
