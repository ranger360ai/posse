//go:build posse_arm3

package posse

// ranger-base-efe36, fixing ranger-base-42au9: seedBeadsRedirect's write
// closed the LEAF (ranger-base-aojiu, seedredirectrename_test.go) but still
// resolved the DIRECTORY above it — <tree>/.beads itself — through
// os.MkdirAll, which Stats each component and so FOLLOWS a symlink a caged
// seat planted right there. seedWorktreeLinks carries the identical shape one
// level up: os.MkdirAll(filepath.Dir(dst)) follows a symlink at the declared
// parent (`<tree>/plugin` for `worktree_link: plugin/bin`) and the
// create-only os.Symlink then lands the link OUTSIDE, in whatever directory
// the parent link points at.
//
// The fix resolves every seed write against the tree's own directory fd
// (os.Root = openat/renameat with the kernel's own escape check) instead of
// through a path a session can redirect with a link: an escaping component is
// refused AT THE SYSCALL, so there is no check-then-write window a leftover
// session process can race, and a link INSIDE the tree is still followed by
// design (arm G here, and TestWorktreeLinkSeedsDeclaredPaths for the ordinary
// case).
//
// Arms A-I mirror the nine parent-component probes filed against
// ranger-base-42au9 (the security lane's own probe file, kept outside the
// shipped tree) one for one, turned from logging into pins: every arm that probe only
// logged an ESCAPE line for now fails the build if the escape recurs, and
// every arm asserts the tree's own account of where the store is afterward —
// <tree>/.beads is a real directory and <tree>/.beads/redirect reads the
// repo's own .beads path — because a fix that refuses instead of seeding
// must not pass this file by being quiet (the same discipline
// seedredirectrename_test.go's CONTROL enforces one level down). Arm J is the
// seedWorktreeLinks sibling.
//
// MEASURED 2026-09-07 (go1.26.5 darwin/arm64, go test -overlay against this
// revision and against a revert to the plain os.MkdirAll/os.Symlink shape):
// arms A, B, D and J are RED under the revert (a symlink or dangling link at
// the parent is followed, and worktree_link's outside directory gains a
// `bin` entry) and every arm here is green under this file's fix.

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// spTree builds the two directories seedBeadsRedirect wants: a repo with a
// .beads (so the function does not return early) and a fresh session tree
// directory (NOT pre-seeded with .beads — every arm here plants its own
// shape at <tree>/.beads before the run).
func spTree(t *testing.T) (repo, tree string) {
	t.Helper()
	repo = t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	tree = t.TempDir()
	return repo, tree
}

func spNames(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

func spRun(t *testing.T, repo, tree string) error {
	t.Helper()
	var err error
	if !returnsWithin(t, 15*time.Second, func() {
		err = seedBeadsRedirect(&SessionTree{Repo: repo, Path: tree})
	}) {
		t.Fatalf("seedBeadsRedirect BLOCKED — it opened something at the tree's .beads parent")
	}
	return err
}

// spAssertSeeded is the after-every-arm check the probe only logged: the
// tree's own account of where the store is must be a REAL directory holding
// a REAL redirect to the repo's own .beads, whatever adversarial shape was at
// <tree>/.beads (or in some directory it pointed at) beforehand.
func spAssertSeeded(t *testing.T, tree, repo string) {
	t.Helper()
	b := filepath.Join(tree, beadsDirName)
	fi, err := os.Lstat(b)
	if err != nil || !fi.IsDir() {
		t.Fatalf("after the seed, %s is not a real directory (Lstat %v, err %v)", b, fi, err)
	}
	got := readSeeded(t, tree)
	if want := filepath.Join(repo, beadsDirName); got != want {
		t.Errorf("after the seed, %s reads %q, want the repo's own %q", filepath.Join(b, beadsRedirect), got, want)
	}
}

// A: .beads -> an EMPTY directory outside the tree.
func TestSeedRedirectParentSymlinkToEmptyDirOutsideDoesNotWiden(t *testing.T) {
	t.Parallel()
	repo, tree := spTree(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(tree, beadsDirName)); err != nil {
		t.Fatal(err)
	}
	if err := spRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a symlink it should simply have replaced: %v", err)
	}
	if n := spNames(t, outside); len(n) != 0 {
		t.Errorf("ESCAPE: the directory outside the tree gained %v", n)
	}
	spAssertSeeded(t, tree, repo)
}

// B: .beads -> a directory outside holding a regular file named redirect.
func TestSeedRedirectParentSymlinkToDirOutsideWithForeignRedirectNotClobbered(t *testing.T) {
	t.Parallel()
	repo, tree := spTree(t)
	outside := t.TempDir()
	foreign := filepath.Join(outside, beadsRedirect)
	if err := os.WriteFile(foreign, []byte("FOREIGN\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tree, beadsDirName)); err != nil {
		t.Fatal(err)
	}
	if err := spRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a symlink it should simply have replaced: %v", err)
	}
	b, err := os.ReadFile(foreign)
	if err != nil {
		t.Fatalf("the file outside the tree: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "FOREIGN" {
		t.Errorf("ESCAPE: the file named redirect OUTSIDE the tree was CLOBBERED, now reads %q", got)
	}
	spAssertSeeded(t, tree, repo)
}

// C: a DANGLING symlink at .beads.
func TestSeedRedirectParentDanglingSymlinkDoesNotCreateOutside(t *testing.T) {
	t.Parallel()
	repo, tree := spTree(t)
	target := filepath.Join(t.TempDir(), "never-made")
	if err := os.Symlink(target, filepath.Join(tree, beadsDirName)); err != nil {
		t.Fatal(err)
	}
	if err := spRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a dangling symlink: %v", err)
	}
	if _, err := os.Lstat(target); err == nil {
		t.Errorf("ESCAPE: the seed CREATED %s outside the tree", target)
	}
	spAssertSeeded(t, tree, repo)
}

// D: .beads -> a REGULAR FILE outside.
func TestSeedRedirectParentSymlinkToRegularFileOutsideNotChanged(t *testing.T) {
	t.Parallel()
	repo, tree := spTree(t)
	outside := filepath.Join(t.TempDir(), "precious.txt")
	if err := os.WriteFile(outside, []byte("PRECIOUS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tree, beadsDirName)); err != nil {
		t.Fatal(err)
	}
	if err := spRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a symlink to a regular file: %v", err)
	}
	b, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("the file outside the tree: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "PRECIOUS" {
		t.Errorf("ESCAPE: the regular file outside was changed, now reads %q", got)
	}
	spAssertSeeded(t, tree, repo)
}

// E: a FIFO at .beads itself.
func TestSeedRedirectParentFIFOAtBeadsItselfReplacedNotOpened(t *testing.T) {
	t.Parallel()
	repo, tree := spTree(t)
	p := filepath.Join(tree, beadsDirName)
	if err := syscall.Mkfifo(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := spRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a FIFO at .beads itself: %v", err)
	}
	spAssertSeeded(t, tree, repo)
}

// F: .beads -> a directory outside where redirect is a DIRECTORY. Nothing
// touches the outside directory at all under the fix (the parent link is
// removed, never followed), so no litter appears there either.
func TestSeedRedirectParentSymlinkToDirOutsideWhereRedirectIsADirLeavesNoLitter(t *testing.T) {
	t.Parallel()
	repo, tree := spTree(t)
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, beadsRedirect), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tree, beadsDirName)); err != nil {
		t.Fatal(err)
	}
	if err := spRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a symlink to a directory outside: %v", err)
	}
	n := spNames(t, outside)
	if len(n) != 1 || n[0] != beadsRedirect {
		t.Errorf("ESCAPE: litter left outside the tree: %v", n)
	}
	spAssertSeeded(t, tree, repo)
}

// G: .beads -> a directory INSIDE the tree. Not an escape — os.Root follows
// an in-tree link by design — but recorded because a check-then-Remove guard
// would have left this arm alone too, and this fix's line is not that: any
// non-directory at .beads, in or out, is REPLACED, the same answer the leaf
// already gives a FIFO or a symlink at the redirect itself.
func TestSeedRedirectParentSymlinkToDirInsideTreeReplacedWithRealDir(t *testing.T) {
	t.Parallel()
	repo, tree := spTree(t)
	inside := filepath.Join(tree, "elsewhere")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", filepath.Join(tree, beadsDirName)); err != nil {
		t.Fatal(err)
	}
	if err := spRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a symlink to a directory inside the tree: %v", err)
	}
	// The link itself is replaced with a fresh real directory (the same
	// REPLACE answer every other non-directory shape gets), so "elsewhere"
	// is left standing, untouched, just no longer aliased as .beads.
	if fi, err := os.Lstat(inside); err != nil || !fi.IsDir() {
		t.Errorf("the directory the symlink pointed at was disturbed: %v (err %v)", fi, err)
	}
	spAssertSeeded(t, tree, repo)
}

// H: after arm A's shape, does beadsHome widen to a third directory? (the
// no-widening claim ranger-base-42au9 made about a root-relative fix.)
func TestSeedRedirectParentBeadsHomeAfterSymlinkedParentDoesNotWiden(t *testing.T) {
	t.Parallel()
	repo, tree := spTree(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(tree, beadsDirName)); err != nil {
		t.Fatal(err)
	}
	if err := spRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored: %v", err)
	}
	got := beadsHome(tree)
	want := filepath.Join(repo, beadsDirName)
	if got != want && got != filepath.Join(tree, beadsDirName) {
		t.Errorf("WIDENING: beadsHome answers a third directory %q", got)
	}
	if got != want {
		t.Errorf("beadsHome(tree) = %q, want the repo's own %q now that the tree carries a real redirect", got, want)
	}
	spAssertSeeded(t, tree, repo)
}

// I: .beads -> dir outside where redirect is a SYMLINK to a second file
// outside. The parent link is never followed at all under the fix, so the
// second link is never reached either.
func TestSeedRedirectParentSymlinkToDirOutsideWhereRedirectIsALinkNotFollowed(t *testing.T) {
	t.Parallel()
	repo, tree := spTree(t)
	outside := t.TempDir()
	second := filepath.Join(t.TempDir(), "second.txt")
	if err := os.WriteFile(second, []byte("SECOND\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(second, filepath.Join(outside, beadsRedirect)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tree, beadsDirName)); err != nil {
		t.Fatal(err)
	}
	if err := spRun(t, repo, tree); err != nil {
		t.Fatalf("the seed errored over a symlink to a directory outside: %v", err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("the second file outside the tree: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "SECOND" {
		t.Errorf("ESCAPE: written THROUGH the second link, now reads %q", got)
	}
	n := spNames(t, outside)
	if len(n) != 1 || n[0] != beadsRedirect {
		t.Errorf("ESCAPE: outside dir now holds %v", n)
	}
	spAssertSeeded(t, tree, repo)
}

// J: seedWorktreeLinks' sibling of the same class (ranger-base-42au9 arm J).
// `worktree_link: plugin/bin` and `<tree>/plugin` a symlink a caged seat
// planted, aimed at an empty directory outside the tree: the pre-fix
// os.MkdirAll(filepath.Dir(dst)) followed it and the create-only os.Symlink
// then landed `bin` OUT THERE. A second, unattacked declaration ahead of it
// in the list (`safe/dir`, ordinary — ranger-base-42au9 did not ask for the
// attacked entry's own link to land anywhere, only that nothing escapes)
// proves the fix's refusal is local to the attacked entry and does not take
// the ordinary case down with it.
func TestSeedWorktreeLinksParentSymlinkOutsideDoesNotWiden(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, "safe", "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "safe", "dir", "marker"), []byte("SAFE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repo, "plugin", "bin", "posse"), "#!/bin/sh\n")
	write(t, a.ConfigPath, "worktree_link:\n  - safe/dir\n  - plugin/bin\n")

	tree := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(tree, "plugin")); err != nil {
		t.Fatal(err)
	}

	err := seedWorktreeLinks(&SessionTree{Repo: repo, Path: tree}, a)
	if err == nil {
		t.Errorf("seedWorktreeLinks over a symlinked parent succeeded silently — a refusal, not a widened link, is the acceptable outcome")
	}

	if n := spNames(t, outside); len(n) != 0 {
		t.Errorf("ESCAPE: the directory outside the tree gained %v", n)
	}

	link := filepath.Join(tree, "safe", "dir")
	fi, lerr := os.Lstat(link)
	if lerr != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the ordinary declaration ahead of the attacked one was not linked inside the tree: %v (err %v)", fi, lerr)
	}
	b, rerr := os.ReadFile(filepath.Join(link, "marker"))
	if rerr != nil || strings.TrimSpace(string(b)) != "SAFE" {
		t.Errorf("the link inside the tree does not reach the repo's own safe/dir: %v (err %v)", string(b), rerr)
	}
}
