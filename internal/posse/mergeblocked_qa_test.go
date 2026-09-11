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
		if out := dispatcherOut(d2); !strings.Contains(out, "already answered this and is still open") {
			t.Fatalf("fixture: pass %d did not take the skip, so nothing here measures it:\n%s", n, out)
		}
	})
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
	mustGit(t, repo, "cherry-pick", head)

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

// TODAY'S BEHAVIOUR AND NOT THE REQUIREMENT, and the one arm ranger-base-ejju3
// deliberately did not close — the idiom this file already uses twice, for its
// reason: a defect that no test names gets fixed by nobody.
//
// The operator answers the handoff by REVERTING the conflicting commit with a
// new one rather than by resetting main. The base is then not an ancestor of
// the branch (no fast-forward), the work is on main under no sha at all (no
// equivalence), the tree is clean (not the dirt arm) — and only the replay can
// say the conflict is gone. The replay in the session tree is the write ADR
// 0058's fact 4 reads, so it cannot be run on a hunch; `git merge-tree` can
// answer it in the object store with no worktree at all, but a branch where
// merge-tree and rebase disagree would then be probed on EVERY pass, which is
// ranger-base-9u5zy's bug again. Closing this needs a record of the base a
// probe last ran against, so it costs one probe per base movement — filed as
// ranger-base-c6ohn. Whoever lands that inverts this test.
func TestAConflictRevertedOnTheBaseIsStillNotRetried(t *testing.T) {
	t.Parallel()
	d, repo, _ := nurlBlocked(t)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dispatcherOut(d), "did NOT reach") {
		t.Fatalf("fixture: the first pass was not blocked:\n%s", dispatcherOut(d))
	}
	mustGit(t, repo, "revert", "--no-edit", "HEAD")

	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)
	if !strings.Contains(out, "already answered this and is still open") {
		t.Errorf("the pass re-asked a base whose conflict was reverted — if that is the fix, this test is the one to invert:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); !os.IsNotExist(err) {
		t.Errorf("the branch landed after the conflict was reverted (%v) — the gap this pins is closed, so invert it", err)
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
