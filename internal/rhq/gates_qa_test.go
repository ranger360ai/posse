package rhq

// QA, rangerhq-40ig — adversarial verification of the shared-index
// commit wall (rangerhq-lmq9). What is pinned here is what I attacked and
// what survived; self-contained (own repo, own fixtures) so it stands
// whatever the next persona does to gates_test.go.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qaCommitRepo is a fresh repo with one commit and a git runner whose env is
// explicit — no RHQ_PERSONA leaks in from the pane the suite runs in, and
// PathOutsideGates drops this session's own shims (rangerhq-8sd).
func qaCommitRepo(t *testing.T) (repo string, git func(env []string, args ...string) (string, error), write func(string, string)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git = func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	write = func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "a")
	write("b.txt", "b")
	git(nil, "add", "a.txt", "b.txt")
	if out, err := git(nil, "commit", "-qm", "init"); err != nil {
		t.Fatalf("init commit: %v %s", err, out)
	}
	return repo, git, write
}

// `git commit -i -- <paths>` (--include) is a FOURTH sweeping form: it
// commits the named paths ON TOP OF whatever is already in the shared
// index, and it carries a pathspec, so the L1 rule's qualifier is
// satisfied. Measured (git 2.39.3): it gets .git/index.lock, so the L3
// hook refuses it through its `-a` arm. This test asserts both halves —
// that the form really does sweep, and that the hook stops it.
func TestQACommitWallIncludeFormSweepsAndIsRefused(t *testing.T) {
	// Half one: without the guard, -i takes the other persona's staged file.
	_, git, write := qaCommitRepo(t)
	write("a.txt", "theirs") // another persona's staged work
	git(nil, "add", "a.txt")
	write("b.txt", "mine")
	if out, err := git(nil, "commit", "-i", "-m", "mine", "--", "b.txt"); err != nil {
		t.Fatalf("unguarded -i commit: %v %s", err, out)
	}
	if out, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(out, "a.txt") {
		t.Fatalf("premise: `git commit -i -- b.txt` must sweep a.txt, got %q", out)
	}

	// Half two: with the guard installed, the same argv is refused.
	repo2, git2, write2 := qaCommitRepo(t)
	gates := t.TempDir()
	if _, err := installCommitGuard(repo2); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + gates}
	write2("a.txt", "theirs")
	git2(nil, "add", "a.txt")
	write2("b.txt", "mine")
	for _, argv := range [][]string{
		{"commit", "-i", "-m", "x", "--", "b.txt"},
		{"commit", "--include", "-m", "x", "--", "b.txt"},
	} {
		out, err := git2(persona, argv...)
		if err == nil || !strings.Contains(out, "refused by posse gate") {
			t.Errorf("git %s must be refused (it sweeps): %v %s", strings.Join(argv, " "), err, out)
		}
	}
	if out, _ := git2(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "a.txt" {
		t.Errorf("the other persona's staged entry must survive, got %q", out)
	}
}

// A refusal must leave the shared tree exactly as it found it: another
// persona's staged entry intact (content included) and no index.lock or
// next-index-* left behind. A wall that wedges the shared index for the
// whole crew would be worse than the sweep it prevents.
func TestQACommitWallRefusalLeavesSharedTreeIntact(t *testing.T) {
	repo, git, write := qaCommitRepo(t)
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}
	write("a.txt", "theirs")
	git(nil, "add", "a.txt")
	write("b.txt", "mine")
	for _, argv := range [][]string{
		{"commit", "-m", "x"},
		{"commit", "-am", "x"},
		{"commit", "--amend", "--no-edit"},
		{"commit", "-m", "x", "--"},
	} {
		if out, err := git(persona, argv...); err == nil {
			t.Fatalf("git %s must be refused: %s", strings.Join(argv, " "), out)
		}
		if out, _ := git(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "a.txt" {
			t.Errorf("after refused `git %s`, staged set changed: %q", strings.Join(argv, " "), out)
		}
		if out, _ := git(nil, "show", ":a.txt"); strings.TrimSpace(out) != "theirs" {
			t.Errorf("after refused `git %s`, staged content changed: %q", strings.Join(argv, " "), out)
		}
		leftovers, _ := filepath.Glob(filepath.Join(repo, ".git", "*.lock"))
		next, _ := filepath.Glob(filepath.Join(repo, ".git", "next-index-*"))
		if len(leftovers)+len(next) != 0 {
			t.Errorf("after refused `git %s`, lock residue: %v %v", strings.Join(argv, " "), leftovers, next)
		}
		if out, err := git(nil, "status", "--short"); err != nil {
			t.Errorf("tree wedged after refused `git %s`: %v %s", strings.Join(argv, " "), err, out)
		}
	}
	// And the way through still works after all that.
	if out, err := git(persona, "commit", "-m", "safe", "--", "b.txt"); err != nil {
		t.Errorf("path-limited commit must still pass: %v %s", err, out)
	}
}

// The L1 half does not catch --include: the rule asks only for `--` with
// an operand, and `git commit -i -- <paths>` has one while still committing
// the whole shared index. L3 catches it (test above), so the wall holds
// where the hook is installed — but L1 is the layer that travels with the
// session into a repo that has no hook.
func TestQACommitWallL1IncludeForm(t *testing.T) {
	t.Skip("rangerhq-ojnw: L1 asks only for `--` with an operand; `-i` carries one and still commits the shared index. L3 catches it, so the wall holds where the hook is installed.")
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	realBin := t.TempDir()
	os.WriteFile(filepath.Join(realBin, "git"), []byte("#!/bin/sh\necho \"real git $*\"\n"), 0o755)
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, binDir, _, err := a.RenderGates("qa", []string{"Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	for _, argv := range [][]string{
		{"commit", "-i", "-m", "x", "--", "a.go"},
		{"commit", "--include", "-m", "x", "--", "a.go"},
	} {
		cmd := exec.Command(filepath.Join(binDir, "git"), argv...)
		out, _ := cmd.CombinedOutput()
		if !strings.Contains(string(out), "refused by posse gate") {
			t.Errorf("git %s must be refused at L1 (it commits the shared index): %s", strings.Join(argv, " "), out)
		}
	}
}
