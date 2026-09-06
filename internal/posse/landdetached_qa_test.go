//go:build posse_arm3

package posse

// ranger-base-vavx2: the pass-start landing sweep asked `<base>..<branch>`
// and nothing else about what a session tree holds — the same wrong question
// ranger-base-d8o6 fixed at the listing and ranger-base-v2rj7 at the two
// retire guards, at the one site that runs when NOBODY IS WATCHING.
//
// That count is ZERO over a worktree whose HEAD is DETACHED, which is not an
// accident and not rare: a container-tier session is launched detached ON
// PURPOSE, because on a detached HEAD a commit writes no ref at all and that
// is what buys the `:ro` git common dir (PrepareSessionHead, ranger-base-t4f1).
// So `if n, ok := unlandedCount(t); ok && n == 0 { continue }` skipped such a
// tree before its bead was read, before MergeSessionWork was called, and
// before any line was printed. MEASURED 2026-09-05: a stamped detached tree
// with one commit on it and a CLOSED bead drew the empty string out of the
// pass, and the base did not reach the commit.
//
// Not the same masking as v2rj7's: the herdr settle path splices a stamped
// detached tree, and `posse worktrees --land` reaches it too. What was
// missing is the UNATTENDED pass — the sweep's own doc calls this "the site
// most likely to be a strand's ONLY reader" — so a deferral the reap makes
// (NOTES.md: "Deferring costs one pass") was permanent over this shape.
//
// THE ARMS ARE EACH OTHER'S CONTROLS. The two tips of a tree whose HEAD is on
// its own branch are the SAME COMMIT, so a "fix" that is really a no-op
// passes every on-branch arm; and asking the HEAD *instead* of the branch
// would be a trade, not a fix — a branch holding a commit its worktree walked
// away from is landable work the head does not reach. So the table carries
// both detached shapes (stamped, which lands; unstamped, which is reported),
// both on-branch shapes, and the tree with nothing to land at all, which is
// the arm a sweep that stopped skipping would fail.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vavxClosed builds the incident nurlStranded builds, with the tree's own
// shape left to the arm: repo, a session tree, a CLOSED bead stamped on the
// branch, and nothing alive anywhere. setUp makes whatever commits the arm
// is about, so a detached arm can detach BEFORE it commits — which is the
// whole fixture.
func vavxClosed(t *testing.T, setUp func(t *testing.T, repo string, tr *SessionTree)) (*Dispatcher, string, *SessionTree) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App, `[]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	tr, err := b.App.EnsureSessionTree(repo, SessionForBead("ranger", repo, "a-1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	setUp(t, repo, tr)
	if err := recordBead(tr.Repo, tr.Branch, "a-1"); err != nil {
		t.Fatal(err)
	}
	return d, repo, tr
}

func TestSweepLandsADetachedTreesClosedWork(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setUp func(t *testing.T, repo string, tr *SessionTree)
		// detached says the fixture must BE the blind spot: the branch count
		// is zero while the tree's own HEAD holds a commit the base does not.
		// Without it an arm that quietly stayed on its branch pins nothing.
		detached bool
		// landed is whether the work must be on the repo's branch afterwards.
		landed bool
		// says is what the pass must print about this tree; silent is the
		// wrong arm, where it must print nothing about it at all.
		says   []string
		silent bool
		// skip is nothingToLand's own answer about this fixture, asked
		// BEFORE the pass runs because the pass moves the branch. The
		// printed output cannot stand in for it: a sweep that skipped
		// nothing would still say nothing about an already-landed tree
		// (MergeSessionWork answers Merged with 0 commits and no arm of the
		// switch prints), so it would cost a bd read and the launcher lock
		// on every tree on every pass and no assertion about words would
		// notice. MEASURED: with `return false` first in nothingToLand the
		// whole table stayed green until this field existed.
		skip bool
	}{{
		// The bug, in the shape the fleet runs it: a caged session, stamped
		// detached, whose bead was closed after its pass stopped watching.
		// MergeSessionWork splices it back onto the branch and lands it —
		// but only if this sweep asks the bead at all.
		name: "a caged session's detached work is landed",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			mustGit(t, repo, "config", detachedKey(tr.Branch), "1")
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: the caged fix")
		},
		detached: true,
		landed:   true,
		says:     []string{"a-1", "1 commit(s) fast-forwarded", "closed after its pass"},
	}, {
		// The same tree with no stamp: nothing will splice it, so nothing
		// lands — and THAT is the half this bead is named for. A strand the
		// pass cannot fix is still a strand the pass has to say out loud,
		// and it prescribes the `branch -f` that makes the next pass land it.
		name: "an unstamped detached tree is said out loud",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: off its own branch")
		},
		detached: true,
		says:     []string{"a-1", "did NOT reach", "branch -f "},
	}, {
		// The on-branch control for the landing arms: the shape that was
		// never blind. It passes before the fix and after it.
		name: "work on the session branch is landed",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: on the branch")
		},
		landed: true,
		says:   []string{"a-1", "1 commit(s) fast-forwarded"},
	}, {
		// The arm a HEAD-ONLY fix loses, and the reason the skip test asks
		// both tips: the branch holds a commit its own worktree walked away
		// from, so the head reaches nothing the base does not while the
		// branch is a whole close's work. MergeSessionWork lands the branch.
		name: "a branch the tree walked away from is landed",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: left on the branch")
			// Back to where the branch was cut, off the branch.
			mustGit(t, tr.Path, "checkout", "-q", "--detach", tr.Base)
		},
		landed: true,
		says:   []string{"a-1", "1 commit(s) fast-forwarded"},
	}, {
		// The wrong arm, and the only one that holds the skip itself: a tree
		// whose work is already on the base is passed over in silence and
		// without the launcher lock. A guard that answered "there is
		// something here" over everything would fail it.
		name: "a tree whose work is already on the base is skipped in silence",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: the fix")
			if _, err := MergeSessionWork(tr); err != nil {
				t.Fatal(err)
			}
		},
		landed: true,
		silent: true,
		skip:   true,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d, repo, tr := vavxClosed(t, c.setUp)

			if c.detached {
				// The fixture IS the blind spot, or this arm measures a
				// shape the sweep could always see.
				if n := mustGit(t, repo, "rev-list", "--count", tr.Base+".."+tr.Branch); n != "0" {
					t.Fatalf("rev-list --count %s..%s = %s; a detached fixture must leave the branch where it was cut", tr.Base, tr.Branch, n)
				}
				head := mustGit(t, tr.Path, "rev-parse", "HEAD")
				if reaches(repo, "refs/heads/"+tr.Branch, head) {
					t.Fatalf("the branch still reaches %s; this fixture is not detached work", abbrevSHA(head))
				}
			}

			// Asked before the pass, which moves the branch out from under
			// the question.
			trs, err := SessionTreesIn([]string{repo})
			if err != nil || len(trs) != 1 {
				t.Fatalf("fixture: session trees %v %v", len(trs), err)
			}
			if got := nothingToLand(trs[0]); got != c.skip {
				t.Errorf("nothingToLand = %v, want %v — a skip here is a tree whose bead is never read, whose merge is never attempted and whose strand is never printed", got, c.skip)
			}

			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)

			body, rerr := os.ReadFile(filepath.Join(repo, "fix.txt"))
			if landed := rerr == nil && string(body) == "the persona's work\n"; landed != c.landed {
				t.Errorf("the closed bead's work is on %s = %v, want %v — a strand is work no ref the shop reads will ever reach:\n%s",
					tr.Base, landed, c.landed, out)
			}
			if c.silent {
				if strings.Contains(out, tr.Branch) || strings.Contains(out, "a-1") {
					t.Errorf("a tree with nothing to land was reported anyway:\n%s", out)
				}
				return
			}
			for _, want := range c.says {
				if !strings.Contains(out, want) {
					t.Errorf("the pass does not say %q about this tree — the silence this bead exists to remove:\n%s", want, out)
				}
			}
		})
	}
}
