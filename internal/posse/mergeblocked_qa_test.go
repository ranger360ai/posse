//go:build posse_arm2

package posse

// ranger-base-yct7l (laurie's verify of ranger-base-9u5zy): what
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
// AND THE COST, pinned below as it currently stands: the skip is keyed on
// the BRANCH not having moved, while every reason a merge-back blocks is a
// statement about the BASE. Once a block stands, a base that moves back
// under the branch is never re-read, so work that would now land does not
// land and work that has since reached the base under another sha is still
// reported as a strand. The two tests after the quiet one assert TODAY'S
// behaviour and not the desired behaviour — the idiom of
// TestAWriterFasterThanTheGraceKeepsTheTreeForever, for the same reason: a
// defect that no test names gets fixed by nobody. Whoever fixes it inverts
// them; the finding is filed at ranger-base-yct7l's escape bundle.

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

// The property ADR 0058's fact 4 actually reads: the writes STOP. Not
// "the next pass writes nothing" — the settle above is a write and it is
// sometimes two (MEASURED over 10 runs of this fixture on 2026-09-10: the
// git dir was written on the pass after the block stood in 10/10, and on
// the pass after THAT in 1/10, and never later). What fact 4 needs is that
// the writing ends while the grace still has room, so this pins a BOUND on
// the settle and then requires the tree to be still — every file of the git
// dir and lastTreeWrite itself, which is the reading the retire takes.
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
	// The settle window: passes 2 and 3 may still write, because the first
	// `git status` after the last aborted rebase rewrites the index whose
	// stat cache that rebase left dirty. Every pass after them must write
	// NOTHING — which is the whole difference between a bounded settle and
	// the removed probe, whose write landed on every pass forever.
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
			t.Errorf("pass %d wrote the tree of an unmoved, already-blocked branch: %s (lastTreeWrite %s -> %s) — that is past the %d-pass stat-cache settle, and fact 4 reads exactly this, so a tree written at the sweep's own cadence never goes quiet",
				n, strings.Join(moved, ", "), prevLast, curLast, settle)
		}
		prev, prevLast = cur, curLast
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

// TODAY'S BEHAVIOUR AND NOT THE REQUIREMENT (ranger-base-yct7l). The block
// said "main moved on and replaying conflicts". main moves back — the
// operator reverts the commit that conflicted, which is one of the two ways
// a human answers this handoff — and the branch, untouched, now
// fast-forwards. The skip is keyed on the branch, so the question is never
// asked again and the work never lands. Before ranger-base-9u5zy the very
// next pass landed it (measured by reverting standingMergeBlock's call:
// `⤴ a-1 1 commit(s) fast-forwarded`), which is also the sequence
// MergeSessionWork's own header measures on ranger-base-c02a/59fs — a bead
// filed at 09:57:11 and the same untouched branch landed at 09:57:26.
func TestABlockThatStandsIsNotReconsideredWhenTheBaseMovesBack(t *testing.T) {
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
	if !strings.Contains(out, "already answered this and is still open") {
		t.Errorf("the skip no longer holds over a base that moved back — if that is the fix, this test is the one to invert:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); !os.IsNotExist(err) {
		t.Errorf("the branch landed after the base moved back (%v) — the defect this pins is fixed, so invert it", err)
	}
}

// TODAY'S BEHAVIOUR AND NOT THE REQUIREMENT (ranger-base-yct7l), the same
// key read from the other side: the branch's commit reaches the base under
// ANOTHER sha after the block stands. That is the ≡ arm (equivalentOnBase,
// ranger-base-g2xf) and it is the arm that ENDS a strand — Merged with
// nothing left to land, which is what lets the tree retire by the measured
// path ADR 0058 D2 wants. Skipped now, so the strand is reported forever
// over work that is already on main and the tree stays kept.
func TestWorkThatReachedTheBaseUnderAnotherShaIsStillReportedAStrand(t *testing.T) {
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
	if out := dispatcherOut(d2); !strings.Contains(out, "did NOT reach") {
		t.Errorf("the pass read the base again and saw the equivalence — the defect this pins is fixed, so invert it:\n%s", out)
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
