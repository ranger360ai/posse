//go:build posse_arm3

package posse

// ranger-base-aojiu, fixing ranger-base-d14e1: seedBeadsRedirect must never
// OPEN the redirect it writes into the session tree.
//
// The destination is the session's OWN worktree, which every caged session can
// write by construction, and the writer is the LAUNCHER, outside the cage. So
// the session picks the file that sits at <tree>/.beads/redirect and an uncaged
// process writes to it. os.WriteFile opens it, and an open resolves both things
// the session controls: the file's TYPE (a FIFO blocks the launcher forever —
// ranger-base-lwfhe, pinned in seedfifowrite_qa_test.go) and, through a symlink,
// its PATH (the write lands wherever the link points — ranger-base-d14e1, P1).
// An isRegularFile guard in front of it does not help: os.Stat FOLLOWS, so the
// symlink answers true and is written through.
//
// The fix is one primitive rather than a check: write a sibling temp and
// os.Rename it over the name. rename(2) replaces the destination's last
// component without following it and without opening it, so the symlink and the
// FIFO fall together and there is no check-then-write window to race.
//
// Every arm below was shown able to fail before it was parked; the table at the
// foot of this file says which revision or mutant reds which arm, and no arm is
// in it that nothing reds. Arms an earlier guard already closed are kept anyway
// — they are what a later "simplification" back to check-then-write breaks
// first, and the table is what says so.
//
// The CONTROL is load-bearing in the other direction. A rename replaces the
// INODE at that path, and a fix that simply removed everything unconditionally
// would pass every escape arm by being destructive — so the control asserts the
// ordinary relaunch still ends with the repo's own .beads at that path.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seedRenameTree builds the two directories seedBeadsRedirect wants: a repo
// with a .beads (so the function does not return early) and a session tree
// with one (so the arm decides what is at <tree>/.beads/redirect, not whether
// the directory exists).
func seedRenameTree(t *testing.T) (repo, tree string) {
	t.Helper()
	repo = t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	tree = t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo, tree
}

func seedRenameRun(t *testing.T, repo, tree string) error {
	t.Helper()
	var err error
	if !returnsWithin(t, 15*time.Second, func() {
		err = seedBeadsRedirect(&SessionTree{Repo: repo, Path: tree})
	}) {
		t.Fatalf("seedBeadsRedirect blocked — it opened the destination")
	}
	return err
}

// CONTROL. Not a formality: it is the arm that a destructive "fix" fails.
func TestSeedRedirectRewritesARegularRedirectInPlace(t *testing.T) {
	t.Parallel()
	repo, tree := seedRenameTree(t)
	p := filepath.Join(tree, beadsDirName, beadsRedirect)
	if err := os.WriteFile(p, []byte("/stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := seedRenameRun(t, repo, tree); err != nil {
		t.Fatalf("CONTROL: the relaunch shape errored: %v", err)
	}
	if got, want := readSeeded(t, tree), filepath.Join(repo, beadsDirName); got != want {
		t.Errorf("CONTROL: the re-seed left %q, want the repo's own %q", got, want)
	}
	// The mode is part of the contract: beadsHome reads this on the launch
	// path, and os.CreateTemp opens at 0600.
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("CONTROL: the seeded redirect is %v, want 0644", fi.Mode().Perm())
	}
}

// THE ESCAPE. A symlink at the seeded path aimed at a regular file OUTSIDE the
// session tree: the launcher must not write through it.
func TestSeedRedirectMustNotWriteThroughASymlinkOutOfTheTree(t *testing.T) {
	t.Parallel()
	repo, tree := seedRenameTree(t)
	outside := filepath.Join(t.TempDir(), "precious.txt")
	if err := os.WriteFile(outside, []byte("PRECIOUS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(tree, beadsDirName, beadsRedirect)
	if err := os.Symlink(outside, p); err != nil {
		t.Fatal(err)
	}
	// The fixture's own witness: over a plain file this arm proves nothing.
	if fi, err := os.Lstat(p); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("fixture: %s is not a symlink (err %v)", p, err)
	}
	if err := seedRenameRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a symlink it should simply have replaced: %v", err)
	}
	b, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("the file outside the tree: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "PRECIOUS" {
		t.Errorf("ESCAPE: the file OUTSIDE the session tree now reads %q — the launcher wrote THROUGH the session's symlink", got)
	}
	// And the tree must carry a redirect of its own afterwards. Leaving the
	// link standing is the second half of the same defect: beadsHome would
	// then answer from a file the session picked, and the seatbelt writable
	// set and the launch line are built from what it answers (ADR 0012 D3-C).
	if got, want := readSeeded(t, tree), filepath.Join(repo, beadsDirName); got != want {
		t.Errorf("the seeded path holds %q, want the repo's own %q", got, want)
	}
}

// A DANGLING symlink. Under main this CREATED a file at the target path
// outside the tree — a create primitive, not just a clobber.
func TestSeedRedirectMustNotCreateAFileThroughADanglingSymlink(t *testing.T) {
	t.Parallel()
	repo, tree := seedRenameTree(t)
	outside := filepath.Join(t.TempDir(), "created-by-the-launcher.txt")
	p := filepath.Join(tree, beadsDirName, beadsRedirect)
	if err := os.Symlink(outside, p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(outside); err == nil {
		t.Fatalf("fixture: %s exists already, so the arm cannot tell a create", outside)
	}
	if err := seedRenameRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a dangling symlink: %v", err)
	}
	if _, err := os.Lstat(outside); err == nil {
		t.Errorf("ESCAPE: %s was CREATED outside the session tree through a dangling symlink", outside)
	}
	if got, want := readSeeded(t, tree), filepath.Join(repo, beadsDirName); got != want {
		t.Errorf("the seeded path holds %q, want the repo's own %q", got, want)
	}
}

// A symlink to a DIRECTORY. rename(2) does not follow the final component, so
// this is the same replacement as any other link — and in particular the
// directory must not gain a `redirect` of its own, and the call must return.
func TestSeedRedirectMustNotReachIntoADirectorySymlink(t *testing.T) {
	t.Parallel()
	repo, tree := seedRenameTree(t)
	outside := t.TempDir()
	p := filepath.Join(tree, beadsDirName, beadsRedirect)
	if err := os.Symlink(outside, p); err != nil {
		t.Fatal(err)
	}
	if err := seedRenameRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a symlink to a directory: %v", err)
	}
	ents, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		names := make([]string, 0, len(ents))
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Errorf("ESCAPE: the directory outside the tree gained %v", names)
	}
	if got, want := readSeeded(t, tree), filepath.Join(repo, beadsDirName); got != want {
		t.Errorf("the seeded path holds %q, want the repo's own %q", got, want)
	}
}

// A real DIRECTORY at the seeded path. rename over one fails with "file
// exists", so the seed must RETURN an error rather than hang — the property
// the WriteFile it replaced happened to have and the one a reader of this fix
// would most easily lose.
func TestSeedRedirectReturnsOverADirectoryAtTheSeededPath(t *testing.T) {
	t.Parallel()
	repo, tree := seedRenameTree(t)
	p := filepath.Join(tree, beadsDirName, beadsRedirect)
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := seedRenameRun(t, repo, tree); err == nil {
		t.Errorf("a directory at %s seeded without error — the seed must say so, not pretend", p)
	}
	// And it must still be there. The guard this fix replaced answered
	// !isRegularFile and called os.Remove, which SUCCEEDS on an empty
	// directory — so it deleted whatever the session had put there and then
	// reported nothing. A rename declines instead.
	if fi, err := os.Lstat(p); err != nil || !fi.IsDir() {
		t.Errorf("the directory at %s was removed on the way past (err %v)", p, err)
	}
	// And it must not have left its temp behind on the way out.
	assertNoRedirectTemp(t, filepath.Join(tree, beadsDirName))
}

// A session's own redirect chmodded 0444. Same class and the same precondition
// as the FIFO — a property of its own file that the session sets and the
// launcher then trips over — and the one shape here that is a plain SELF-DoS
// rather than an escape: on both earlier revisions this killed every later
// relaunch into that tree with "permission denied" and nothing else printed.
// The rename closes it without being aimed at it, because rename(2) needs write
// on the DIRECTORY and not on the file it replaces. Kept as an arm because a
// reader who moved this back to an open would silently reopen it.
func TestSeedRedirectReplacesAReadOnlyRedirect(t *testing.T) {
	t.Parallel()
	repo, tree := seedRenameTree(t)
	p := filepath.Join(tree, beadsDirName, beadsRedirect)
	if err := os.WriteFile(p, []byte("/stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}
	// The fixture's own witness: over a 0644 file this arm proves nothing.
	if fi, err := os.Lstat(p); err != nil || fi.Mode().Perm() != 0o444 {
		t.Fatalf("fixture: %s is not 0444 (err %v)", p, err)
	}
	if err := seedRenameRun(t, repo, tree); err != nil {
		t.Fatalf("a 0444 redirect the session owns killed the relaunch: %v", err)
	}
	if got, want := readSeeded(t, tree), filepath.Join(repo, beadsDirName); got != want {
		t.Errorf("the seeded path holds %q, want the repo's own %q", got, want)
	}
}

// The temp is litter if it outlives the call. It is dot-prefixed and named so
// nothing reading this directory for a redirect can mistake one for it, but a
// leaked temp still dirties a tree the persona is about to commit from.
func TestSeedRedirectLeavesNoTempBehind(t *testing.T) {
	t.Parallel()
	repo, tree := seedRenameTree(t)
	if err := seedRenameRun(t, repo, tree); err != nil {
		t.Fatal(err)
	}
	assertNoRedirectTemp(t, filepath.Join(tree, beadsDirName))
}

func assertNoRedirectTemp(t *testing.T, dir string) {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.Name() != beadsRedirect {
			t.Errorf("the seed left %s in %s", e.Name(), dir)
		}
	}
}

// WHICH REVISION REDS WHICH ARM, measured 2026-09-05 through `go test -overlay`
// against real revisions of worktree.go and two mutants of the fix:
//
//	                        18e8a34   c86a6b8   remove-only   link+leak
//	                        (main)    (lwfhe)   mutant        mutant
//	symlink → regular       RED       RED       RED           .
//	dangling symlink        RED       .         RED           .
//	symlink → directory     RED       .         RED           .
//	directory at the path   .         RED       RED           .
//	CONTROL (regular)       .         .         RED           .
//	0444 redirect           RED       RED       .             .
//	no temp left behind     .         .         .             RED
//	FIFO (seedfifowrite)    RED       .         .             .
//
// The remove-only mutant is the destructive "fix" the security lane's ruling
// named: it passes every escape arm by deleting rather than replacing, and
// only the CONTROL catches it. The link+leak mutant drops the deferred Remove and links
// instead of renaming, which is the only way the temp arm can be reached.
