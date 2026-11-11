//go:build !posse_arm2 && !posse_arm3

package posse

// LIVE-DEFECT PIN — ranger-base-md7ag, the QA lane's verify of
// ranger-base-8t6j1.
//
// ADR 0086 D3's headline is "A PID THAT DENIES THE WHOLE TREE GETS NO WRITE
// GRANT INTO ANY GIT DIR OF ITS SESSION", and ranger-base-8t6j1 told five
// doors so. There is a SIXTH, and this pin holds the hole open until it is
// closed: `beadsGitDirs`, the store-of-record grant SeatbeltWritable makes
// for every PID.
//
// THE SHAPE. D3 keeps the store of record's own git dirs granted "because bd
// commits there, and it is another repo" (seatbelt.go's own words). The code
// does not test for another repo — it tests whether the store is already
// covered by what this PID was granted (`!underDir(grantedBeadsBase, home)`,
// where grantedBeadsBase is `<cwd>/.beads` for this PID class). So a
// `.beads/redirect` that stays INSIDE the session's own repository but
// outside `<cwd>/.beads` — `<cwd>/inner/.beads`, which is ranger-base-f5dg's
// REPRO 2 and the shape 8t6j1's own recordTargets narrowing keeps "granted
// and probed" — makes the store of record's git dirs and the SESSION's git
// dirs the same directories, and they are granted WRITE.
//
// WHAT THAT REACHES, which is W14/W15 exactly as D3 states them: the grant is
// the per-worktree git dir and the checkout's `.git` WHOLE — not even
// narrowed below the common dir the way `sessionGitGrants` was
// (ranger-base-m2wf) — so `objects` and `refs/heads` are writable and a seat
// that cannot edit a file mints a commit by plumbing and moves
// `posse/<session>`, which the launcher fast-forwards onto `main`. In a main
// checkout the same grant reaches `refs/heads/main`.
//
// NOT EXPOSED ON THIS BOX TODAY, and that is why this is a pin and not a P1:
// all three stores here are reached through a redirect into ANOTHER
// repository, so every dispatched launch takes the branch where the
// exception is what D3 says it is.
//
// IT IS GREEN BECAUSE THE DEFECT IS LIVE. When the hole is closed this pin
// FAILS, and its message says so: delete it, and widen
// TestQAWholeTreeDenyGetsNoGitGrantInALinkedWorktree to drive an in-repo
// redirect beside its no-redirect fixture.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAWholeTreeDenyStillGetsItsOwnGitDirsThroughAnInRepoRedirect(t *testing.T) {
	f := wgDenyFixture(t)
	ag := wgDeny(f)

	// THE CONTROL, and it is what makes this pin a reading of the redirect
	// and not of the fixture: with no redirect at all, D3 holds — the
	// session's repo contributes `<cwd>/.beads` and nothing else.
	if got := wgRepoGrants(f.a.SeatbeltWritable(ag, f.tree, f.gates), f.repo, f.tree); len(got) != 1 ||
		got[0] != absResolve(filepath.Join(f.tree, beadsDirName)) {
		t.Fatalf("the control is wrong, so this pin measures nothing: with no redirect the repo-local set is\n  %s\nwant just %s",
			strings.Join(wgKinds(got), "\n  "), filepath.Join(f.tree, beadsDirName))
	}

	// The store of record, moved INSIDE this session's own repo.
	inner := filepath.Join(f.tree, "inner", beadsDirName)
	for _, d := range []string{inner, filepath.Join(f.tree, beadsDirName)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.tree, beadsDirName, beadsRedirect), []byte(inner+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := beadsHome(f.tree); got != inner {
		t.Fatalf("the redirect was not followed, so nothing below is about it: beadsHome = %s, want %s", got, inner)
	}

	set := f.a.SeatbeltWritable(ag, f.tree, f.gates)
	var git []string
	for _, p := range wgRepoGrants(set, f.repo, f.tree) {
		if underDir(f.common, p) || p == absResolve(filepath.Join(f.tree, ".git")) {
			git = append(git, p)
		}
	}
	if len(git) == 0 {
		t.Fatalf("ADR 0086 D3 NOW HOLDS FOR AN IN-REPO REDIRECT TOO — the hole this pin was written to hold open is closed.\n"+
			"Delete this file and widen TestQAWholeTreeDenyGetsNoGitGrantInALinkedWorktree to drive a\n"+
			"`<cwd>/.beads/redirect` naming `<cwd>/inner/.beads` beside its no-redirect fixture.\n"+
			"repo-local set was:\n  %s", strings.Join(wgKinds(wgRepoGrants(set, f.repo, f.tree)), "\n  "))
	}
	t.Logf("LIVE (ranger-base-md7ag): a whole-tree-deny PID is granted WRITE in its own session git dirs\n  %s\nbecause beadsGitDirs answers for a store of record that is inside this very repo",
		strings.Join(wgKinds(git), "\n  "))

	// And the bwrap mechanism binds them read-write: BwrapWritable returns
	// early for this PID class (ADR 0086 D3, "no git grant to swap"), so the
	// set it hands BwrapArgs is this one, unnarrowed. The claim that both
	// mechanisms agree after D3 is true, and what they agree on here is the
	// grant.
	bset, _ := BwrapWritable(ag, set, f.tree)
	for _, p := range git {
		found := false
		for _, q := range bset {
			if p == q {
				found = true
			}
		}
		if !found {
			t.Errorf("the bwrap mechanism dropped %s but the seatbelt producer kept it — the two walls now disagree for this PID, which is its own finding", p)
		}
	}

	// recordTargets probes them, so the launch gate PASSES over the grant
	// rather than catching it: 8t6j1's narrowing is conditioned on the store
	// being under `<cwd>/.beads`, which is exactly what this shape is not.
	if n := len(recordTargets(ag, f.tree, beadsHome(f.tree))); n < 2 {
		t.Errorf("recordTargets = %d target(s); this shape keeps the git dirs as targets, so a change here is part of the same finding", n)
	}
}
