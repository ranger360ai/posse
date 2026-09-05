package posse

// ranger-base-v2rj7: the two guards that stand between a session's committed
// work and its destruction — RemoveSessionTree's unforced refusal, and
// residueHolds, which is that refusal asked as a question before a reap
// kills — both asked `<base>..<branch>` and nothing else about what a tree
// holds.
//
// That count is ZERO over a worktree whose HEAD is DETACHED, which is not an
// accident and not rare: a container-tier session is launched detached ON
// PURPOSE, because on a detached HEAD a commit writes no ref at all and that
// is what buys the `:ro` git common dir (PrepareSessionHead, ranger-base-t4f1).
// So a tree holding the whole of such a session's work read to both of them
// as holding nothing. MEASURED before the fix: RemoveSessionTree(t, false)
// over a stamped, clean, detached tree with one commit on it returned nil,
// removed the worktree, deleted the branch, and left the commit referenced by
// nothing at all.
//
// Not a live loss the day it was filed, and the bead says so: both are masked
// by their one caller, the herdr settle path, which runs MergeSessionWork
// first and that splices a stamped detached tree back onto its branch before
// either guard is reached. What is wrong is the guards — neither holds on its
// own evidence, and the second caller of either loses work.
//
// THE ARMS ARE EACH OTHER'S CONTROLS, which is the whole design here. The two
// tips of a tree whose HEAD is on its own branch are the SAME COMMIT, so a
// "fix" that is really a no-op passes every on-branch arm; and a guard that
// simply refuses over anything detached passes every kept arm. So each shape
// appears twice — once holding work, once with its work measured onto the
// base — and the guards must tell those apart in both.
//
// The two guards are asked about ONE fixture and their answers compared,
// rather than pinned in two hand-written sentences: residueHolds' own doc
// says it is RemoveSessionTree's refusal asked as a question, and that claim
// is only true while they agree. residueHolds is asked FIRST in every arm,
// because RemoveSessionTree ACTS and an answer read after it would be about a
// tree the assertion had just changed.

import (
	"os"
	"strings"
	"testing"
)

func TestRetireGuardsSeeADetachedTreesWork(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setUp func(t *testing.T, repo string, tr *SessionTree)
		// detached says the fixture must be the blind spot itself: the
		// branch count is zero while the tree holds a commit. Without this
		// an arm that quietly stayed on its branch would pin nothing.
		detached bool
		// kept is what BOTH guards owe about it.
		kept bool
		// says is what the refusal must spell out. Empty on a taken arm.
		says []string
	}{{
		// The bug, in the shape the fleet runs: no stamp, so nothing will
		// splice this back either.
		name: "a detached tree's unlanded work is held by both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: off its own branch")
		},
		detached: true,
		kept:     true,
		says:     []string{"commit(s) not on main", "branch -f " + SessionBranch("s-1")},
	}, {
		// The same, with the record a caged launch leaves. The stamp is what
		// MergeSessionWork reads to splice — and these guards must hold
		// whether or not anything ever splices, because the splice is the
		// caller's act and not their evidence.
		name: "a caged session's detached work is held by both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			mustGit(t, repo, "config", detachedKey(tr.Branch), "1")
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the caged work")
		},
		detached: true,
		kept:     true,
		says:     []string{"commit(s) not on main", "branch -f " + SessionBranch("s-1")},
	}, {
		// The on-branch control for the kept arms: the shape that was never
		// blind. It passes before the fix and after it, and it is what goes
		// red if the branch tip stops being asked at all — asking the head
		// INSTEAD of the branch would be a trade, not a fix.
		name: "work on the session branch is held by both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: on the branch")
		},
		kept: true,
		says: []string{"commit(s) not on main"},
	}, {
		// A branch holding a commit its own worktree's HEAD does not reach:
		// the tree walked away from it. Asking the head alone answers
		// "nothing held" here, and `branch -D` below would take the branch
		// anyway — so this is the arm the head-only fix loses.
		name: "a branch the tree walked away from is held by both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: left on the branch")
			// Back to where the branch was cut, off the branch: HEAD reaches
			// nothing the base does not, the branch still holds the commit.
			mustGit(t, tr.Path, "checkout", "-q", "--detach", "main")
		},
		kept: true,
		says: []string{"commit(s) not on main"},
	}, {
		// The on-branch control for the taken arms: nothing here is the last
		// copy of anything, and a guard that answered "held" over everything
		// would fail it.
		name: "an on-branch tree whose work is on the base is taken by both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the fix")
			if _, err := MergeSessionWork(tr); err != nil {
				t.Fatal(err)
			}
		},
	}, {
		// The DETACHED control, and the one that stops "refuse over anything
		// detached" from passing this file: the work is off every branch and
		// it is also on the base by measurement, so nothing a ref could name
		// would be lost. A pick, because a merge cannot reach a detached
		// tree's HEAD — which is the case in the first place.
		name: "a detached tree whose work is measured on the base is taken by both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the fix, off its branch")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			// The base moves first or the pick rebuilds the identical commit
			// object and the base reaches it by SHA, which measures nothing
			// (ranger-base-g2xf's fixture).
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			mustGit(t, repo, "cherry-pick", "-x", sha)
		},
		detached: true,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := wtApp(t)
			repo := wtRepo(t)
			commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			c.setUp(t, repo, tr)
			head := mustGit(t, tr.Path, "rev-parse", "HEAD")

			if c.detached {
				// The fixture IS the blind spot, or this arm measures a
				// shape the guards could always see: the branch count is
				// zero while the tree's own HEAD holds a commit the base
				// does not.
				if n := mustGit(t, repo, "rev-list", "--count", "main.."+tr.Branch); n != "0" {
					t.Fatalf("rev-list --count main..%s = %s; a detached fixture must leave the branch where it was cut", tr.Branch, n)
				}
				if reaches(repo, "refs/heads/"+tr.Branch, head) {
					t.Fatalf("the branch still reaches %s; this fixture is not detached work", abbrevSHA(head))
				}
			}

			// residueHolds first: RemoveSessionTree below ACTS.
			held := residueHolds(&HerdrMeta{
				Name: "s-1", Dir: tr.Path, Repo: repo, Branch: tr.Branch,
			})
			err = RemoveSessionTree(tr, false)

			if (held != "") != c.kept {
				t.Errorf("residueHolds = %q, want held=%v — the reap kills on this answer", held, c.kept)
			}
			if (err != nil) != c.kept {
				t.Errorf("RemoveSessionTree = %v, want a refusal=%v", err, c.kept)
			}
			// The agreement itself, stated once: residueHolds' doc says it
			// IS this refusal asked as a question, and that is the claim a
			// second caller of either would rest on.
			if (held != "") != (err != nil) {
				t.Errorf("one guard, two answers about one tree — residueHolds says %q, RemoveSessionTree says %v", held, err)
			}

			if !c.kept {
				if _, serr := os.Stat(tr.Path); serr == nil {
					t.Error("the worktree directory survived a removal nothing refused")
				}
				return
			}
			for _, want := range c.says {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not say %q, got: %v", want, err)
				}
			}
			// What the refusal is FOR: the commit is still reachable. A tree
			// removed here leaves a detached HEAD's commit named by nothing
			// — no branch, no worktree, no reflog anyone will read — which is
			// what the pre-fix measurement produced.
			if _, serr := os.Stat(tr.Path); serr != nil {
				t.Errorf("the refused tree was removed anyway: %v", serr)
			}
			if !branchExists(repo, tr.Branch) {
				t.Error("the refused branch was deleted anyway")
			}
			if _, gerr := git(repo, "rev-parse", "--verify", "--quiet", head+"^{commit}"); gerr != nil {
				t.Errorf("the work %s is gone from the object store: %v", abbrevSHA(head), gerr)
			}
		})
	}
}
