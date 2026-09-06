package posse

// Helpers lifted out of worktree_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// wtApp is an App whose default worktree root is this test's own, so the
// under-$HOME placement rule is satisfied for real and a test never writes
// into the operator's ~/.posse. It used to buy that with t.Setenv("HOME"),
// which held 71 tests serial after ADR 0047 made the guarantee two cheaper
// ways: TestMain gives the binary a temp $HOME, and hermetic puts the root
// at $HOME/worktrees/<t.Name()> (ranger-base-pj87l).
func wtApp(t *testing.T) *App {
	t.Helper()
	h := t.TempDir()
	return hermetic(t, &App{
		Home: h, ConfigPath: filepath.Join(h, "config.yaml"),
		RecipesDir: filepath.Join(h, "recipes"), EnvsDir: filepath.Join(h, "envs"),
		StateDir: filepath.Join(h, "state"), AgentsDir: filepath.Join(h, "agents"),
	})
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git(dir, args...)
	if err != nil {
		t.Fatalf("git %s in %s: %v", strings.Join(args, " "), dir, err)
	}
	return out
}

// wtRepo is a real one-commit git repo on `main`.
func wtRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	write(t, filepath.Join(repo, "README.md"), "seed\n")
	mustGit(t, repo, "add", "README.md")
	mustGit(t, repo, "commit", "-q", "-m", "seed")
	return repo
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitIn makes one commit the way a persona does: the path-limited form
// the wall requires, so the test exercises the same shape it does. The `add`
// is not optional and is worth knowing — `git commit -- <path>` on a path
// git has never seen fails with "pathspec did not match any file(s) known to
// git", so the blessed form needs an `add` in front of it for a NEW file.
func commitIn(t *testing.T, dir, path, body, msg string) {
	t.Helper()
	write(t, filepath.Join(dir, path), body)
	mustGit(t, dir, "config", "user.email", "p@example.com")
	mustGit(t, dir, "config", "user.name", "p")
	mustGit(t, dir, "add", path)
	mustGit(t, dir, "commit", "-q", "-m", msg, "--", path)
}

func readRedirect(t *testing.T, tree string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tree, ".beads", "redirect"))
	if err != nil {
		t.Fatalf("no beads redirect in the session tree: %v", err)
	}
	return strings.TrimSpace(string(b))
}

// raceOnRebase installs the `post-rewrite` hook that makes the operator
// commit on the base while a rebase is finishing, `moves` times, and returns
// the path of the counter every firing bumps. The counter is the positive
// witness: an assertion that a merge did or did not happen is payable by a
// fixture that never raced, and this is the only thing that can tell them
// apart.
//
// The hook commits path-limited because the crew's L1 shim is on PATH in a
// persona's session and refuses the sweeping forms — a fixture spelled `git
// commit -qm <msg>` is red for those personas and green for everyone else.
func raceOnRebase(t *testing.T, repo string, moves int) string {
	t.Helper()
	hooks := mustGit(t, repo, "rev-parse", "--git-path", "hooks")
	if !filepath.IsAbs(hooks) {
		hooks = filepath.Join(repo, hooks)
	}
	count := filepath.Join(t.TempDir(), "rebases")
	hook := filepath.Join(hooks, "post-rewrite")
	write(t, hook, "#!/bin/sh\n"+
		"n=$(cat "+shq(count)+" 2>/dev/null || echo 0)\n"+
		"n=$((n+1))\n"+
		"echo \"$n\" > "+shq(count)+"\n"+
		"if [ \"$n\" -le "+strconv.Itoa(moves)+" ]; then\n"+
		"  unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_REFLOG_ACTION GIT_AUTHOR_DATE GIT_COMMITTER_DATE\n"+
		"  cd "+shq(repo)+" || exit 0\n"+
		"  echo \"$n\" > \"raced-$n.txt\"\n"+
		"  git add -- \"raced-$n.txt\"\n"+
		"  git commit -q -m \"the operator commits on the base mid-rebase ($n)\" -- \"raced-$n.txt\"\n"+
		"fi\n"+
		"exit 0\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	return count
}

func racedFiles(moves int) []string {
	var f []string
	for i := 1; i <= moves; i++ {
		f = append(f, "raced-"+strconv.Itoa(i)+".txt")
	}
	return f
}

func countIn(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("unreadable rebase count %q: %v", b, err)
	}
	return n
}
