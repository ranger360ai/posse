//go:build posse_arm2

package posse

// LIVE-DEFECT PINS for ranger-base-751ha (verifying ranger-base-e01op's close).
//
// ranger-base-e01op stopped a clean seat collecting a spurious closed-dirty P1,
// and it did it at the source rather than by subtracting a set in one of the
// five readers — the right call, and
// TestFreshSessionTreeIsPorcelainCleanBeforeTheSeatTypes holds it (seven of
// eight mutations red it, the eighth a genuine no-op).
//
// What that close did not finish is the arithmetic between a PATH and a
// gitignore PATTERN. seedScaffoldExcludes writes `/`+path into the COMMON
// info/exclude, and that file is read by every worktree AND by the operator's
// main checkout. Two consequences, both MEASURED 2026-10-09 (darwin 25.4.0,
// git 2.50.1), each pinned below and each shipping GREEN over the hole:
//
//  1. A path is pasted in unescaped, so a glob metacharacter in a declared
//     `worktree_link:` becomes a PATTERN metacharacter. `cfg[1]` yields
//     `/cfg[1]`, which matches `cfg1` and not `cfg[1]` — so the seat's own
//     collateral file goes invisible to git (the silent-loss direction the
//     close's own comment calls the worse of the two) while the scaffolding it
//     was written for still reads `?? cfg[1]`. Over-exclude and under-exclude
//     from one line.
//
//  2. A declared path the MAIN CHECKOUT holds untracked and un-ignored is
//     silently ignored there from the first session tree onwards. The close
//     withheld a pattern from a TRACKED declared path for exactly this reason,
//     and the untracked-but-visible case is one step over from it: posse
//     deciding an ignore for the operator's own file, which is the thing the
//     same commit declined to do for `.beads/` on the record.
//
// Both reds are one `QuoteMeta`-shaped escape and one `--ignored` check away;
// when either lands, the arm below reds and its message says to delete it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// HOLE 1 — a glob metacharacter in a declared path.
func TestQAScaffoldExcludePastesAPathAsAPattern(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "cfg[1]", "x"), "x\n")
	commitIn(t, repo, ".gitignore", "cfg[1]/\n", "the operator ignores it as a directory, as with .bob")
	write(t, a.ConfigPath, "worktree_link:\n  - cfg[1]\n")

	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	// CONTROL: the tree really holds the scaffolding as a symlink, so what
	// follows is about the pattern and not about an unseeded tree.
	if fi, err := os.Lstat(filepath.Join(tr.Path, "cfg[1]")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("CONTROL: cfg[1] is not a symlink in the session tree: %v", err)
	}
	ex := filepath.Join(repo, ".git", "info", "exclude")
	body, err := os.ReadFile(ex)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "\n/cfg[1]\n") {
		t.Fatalf("CONTROL: the pattern this pin is about is not in info/exclude:\n%s", body)
	}

	// Under-exclude: the scaffolding the close exists to hide is still dirt,
	// because `[1]` is a character class and the literal path is not `cfg1`.
	out := mustGit(t, tr.Path, "status", "--porcelain", "--untracked-files=all")
	if !strings.Contains(out, "cfg[1]") {
		t.Errorf("HOLE CLOSED: the declared path is excluded now, so the pattern is escaped. Delete this arm and give TestFreshSessionTreeIsPorcelainCleanBeforeTheSeatTypes a metacharacter in its fixture: %q", out)
	}

	// Over-exclude, and this is the costly half: the seat's own unrelated
	// file matches the class and never reaches git status.
	write(t, filepath.Join(tr.Path, "cfg1"), "the seat wrote this\n")
	out = mustGit(t, tr.Path, "status", "--porcelain", "--untracked-files=all")
	if strings.Contains(out, "cfg1\n") || strings.Contains(out, "cfg1 ") {
		t.Errorf("HOLE CLOSED: a collateral `cfg1` now reaches git status, so the pattern is escaped. Delete this arm (see above): %q", out)
	}
}

// HOLE 2 — the shared exclude reaches the operator's main checkout.
func TestQAScaffoldExcludeHidesAnUnignoredDeclaredPathInTheMainCheckout(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	// The operator's own untracked, UN-ignored path, named in worktree_link:.
	// `worktree_link:` is documented for gitignored paths, and this is the
	// state one line of .gitignore away from that — a local file somebody
	// never bothered to ignore.
	write(t, filepath.Join(repo, ".secrets", "env"), "K=v\n")
	write(t, a.ConfigPath, "worktree_link:\n  - .secrets\n")

	// CONTROL: git reports it in the main checkout before any tree is seeded.
	before := mustGit(t, repo, "status", "--porcelain", "--untracked-files=all")
	if !strings.Contains(before, ".secrets/env") {
		t.Fatalf("CONTROL: the main checkout must report the un-ignored path first, else this measures nothing: %q", before)
	}

	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(filepath.Join(tr.Path, ".secrets")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("CONTROL: the tree must hold .secrets as posse's own symlink: %v", err)
	}

	after := mustGit(t, repo, "status", "--porcelain", "--untracked-files=all")
	if strings.Contains(after, ".secrets/env") {
		t.Errorf("HOLE CLOSED: seeding a session tree no longer hides the operator's own un-ignored path in the MAIN checkout. Delete this arm and fold the assertion into TestFreshSessionTreeIsPorcelainCleanBeforeTheSeatTypes, whose main-checkout control is scoped to `.bob`: before=%q after=%q", before, after)
	}
	// …and it is ignored, not merely unreported: a `git add -A` salvage in the
	// main checkout would skip it, which is the silent-loss direction.
	if ign := mustGit(t, repo, "status", "--porcelain", "--untracked-files=all", "--ignored", "--", ".secrets"); !strings.Contains(ign, "!!") {
		t.Errorf("the path must be IGNORED in the main checkout for this to be the hole it is — re-read the exclude: %q", ign)
	}
}
