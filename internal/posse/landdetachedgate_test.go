//go:build posse_arm3

package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `posse worktrees --land` reads the bead record before it merges anything,
// and a tree no record accounts for is reported rather than landed (ADR 0006,
// ranger-base-atxe). unaccountedFor asked that of `<base>..<branch>`, which is
// ZERO over a worktree whose HEAD is DETACHED — the shape a container-tier
// session is launched on ON PURPOSE (PrepareSessionHead, ranger-base-t4f1) —
// so the gate opened, and MergeSessionWork behind it SPLICES a designed
// detach's work back onto the branch before it counts. The whole of such a
// session's work went onto the operator's branch with nothing accounting for
// it (ranger-base-qihvt, from ranger-base-vavx2 which fixed the sweep's copy
// of the same question).
//
// THE ON-BRANCH CONTROL IS THE POINT OF THE TABLE. On a tree whose HEAD is on
// its own branch the two tips are the same commit, so a "fix" that changed
// nothing passes every detached arm's siblings; only a fixture that puts the
// work where the branch cannot see it can tell the two apart, and only a
// control that still reads in the BRANCH's words can tell a fix from a
// blanket refusal.
//
// Five arms, because this gate is worth nothing unless it is narrow: the
// designed detach is refused, the on-branch tree is refused in the sentence it
// always had, a RECORDED detach still lands, `--force` still lands the refused
// one (which is what its own refusal promises), and an ACCIDENTAL detach is
// left to landed()'s sentence, which carries the `git branch -f` cure this one
// cannot honestly offer.
func TestLandGateAsksTheTipADetachedTreesWorkIsOn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		detach   bool // the tree's HEAD walks off its branch before the commit
		stamp    bool // posse recorded the detach as its own design
		bead     string
		pick     bool // main moves and takes the tree's work as a new sha
		force    bool
		lands    bool
		want     []string
		unwanted []string
	}{{
		name:   "a designed detach with no record is refused and the refusal names the HEAD",
		detach: true, stamp: true,
		want:     []string{"no record says which bead", "NOT landed", "detached HEAD", "--force"},
		unwanted: []string{"log main..posse/"},
	}, {
		name:     "the on-branch control is refused in the branch's own words",
		want:     []string{"no record says which bead", "NOT landed", "log main..posse/", "--force"},
		unwanted: []string{"detached HEAD"},
	}, {
		name:   "a designed detach whose branch names its bead still lands",
		detach: true, stamp: true, bead: "a-1", lands: true,
		want: []string{"1 commit(s) onto main"},
	}, {
		name:   "--force lands the detached tree the gate refused",
		detach: true, stamp: true, force: true, lands: true,
		want: []string{"1 commit(s) onto main"},
	}, {
		name:   "the equivalence is measured on the tip that holds the work",
		detach: true, stamp: true, pick: true,
		want:     []string{"no record says which bead", "equivalent patch on main", "nothing here is unlanded", "detached HEAD"},
		unwanted: []string{"NOT landed", "--force"},
	}, {
		name:     "an accidental detach keeps landed()'s sentence and its branch -f cure",
		detach:   true,
		want:     []string{"the tree's HEAD is off its own branch", "branch -f"},
		unwanted: []string{"no record says which bead"},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := wtApp(t)
			repo := wtRepo(t)
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			if c.detach {
				mustGit(t, tr.Path, "checkout", "-q", "--detach")
			}
			if c.stamp {
				if err := recordDetached(tr.Repo, tr.Branch, true); err != nil {
					t.Fatal(err)
				}
			}
			commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
			if c.bead != "" {
				if err := recordBead(tr.Repo, tr.Branch, c.bead); err != nil {
					t.Fatal(err)
				}
			}
			// The fixture premise, asserted rather than assumed: a detached
			// tree's branch is still where it was cut, and the count this gate
			// used to ask is the 0 that opened it. Without this a fixture that
			// silently committed on the branch would pass every arm below.
			ahead := mustGit(t, repo, "rev-list", "--count", "main.."+tr.Branch)
			if c.detach && ahead != "0" {
				t.Fatalf("the fixture is not the state under test: the branch is %s ahead of main", ahead)
			}
			if !c.detach && ahead != "1" {
				t.Fatalf("the on-branch control did not commit on its branch: %s ahead of main", ahead)
			}
			head := mustGit(t, tr.Path, "rev-parse", "HEAD")
			if n := mustGit(t, repo, "rev-list", "--count", "main.."+head); n != "1" {
				t.Fatalf("the fixture holds no work on the tree's HEAD: %s ahead of main", n)
			}
			if c.pick {
				// Ahead by sha is not ahead by work, asked of the DETACHED
				// tip: main moves first so the pick is a new sha rather than
				// the identical commit object, and equivalentOnBase can only
				// see it if the gate asks the tip the work is on.
				commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
				mustGit(t, repo, "cherry-pick", "-x", head)
			}
			was := mustGit(t, repo, "rev-parse", "main")

			var out strings.Builder
			if err := LandSessionTrees(&out, a, []string{repo}, c.force); err != nil {
				t.Fatal(err)
			}
			got := out.String()
			for _, want := range c.want {
				if !strings.Contains(got, want) {
					t.Errorf("the line does not say %q:\n%s", want, got)
				}
			}
			for _, unwanted := range c.unwanted {
				if strings.Contains(got, unwanted) {
					t.Errorf("the line should not say %q:\n%s", unwanted, got)
				}
			}
			// The gate is measured by what MOVED, not only by what was said:
			// the sentence and the landing are two claims and a fix that
			// prints the right words while merging anyway is the bug.
			now := mustGit(t, repo, "rev-parse", "main")
			_, onMain := os.Stat(filepath.Join(repo, "fix.txt"))
			if c.lands {
				if now == was || onMain != nil {
					t.Errorf("the work did not reach main (%s → %s):\n%s", was[:12], now[:12], got)
				}
			} else {
				if now != was {
					t.Errorf("the gate did not hold: main moved %s → %s\n%s", was[:12], now[:12], got)
				}
				if onMain == nil && !c.pick {
					t.Errorf("work no record accounts for is on main:\n%s", got)
				}
				// Reported, never destroyed.
				if _, err := os.Stat(filepath.Join(tr.Path, "fix.txt")); err != nil {
					t.Errorf("the refused tree lost its work: %v", err)
				}
			}
		})
	}
}
