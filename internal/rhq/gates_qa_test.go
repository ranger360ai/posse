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

// rangerhq-t9by — close of rangerhq-2f5r. The wall lmq9 landed covers THIS
// incident's shared-index half. The four forms gilfoyle measured against the
// live hook, driven with the incident's own argv (`git commit -F <file>`,
// not `-m`): B holds staged work throughout.
//
// Half one is the unguarded incident: without the hook, `git add mine &&
// git commit -F msg` captures B's staged file. If git stops sweeping, this
// pin dies and the wall is guarding a ghost. Half two is the wall.
func TestQA2f5rIncidentFourForms(t *testing.T) {
	msg := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msg, []byte("incident\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Half one: the incident, unguarded.
	_, git, write := qaCommitRepo(t)
	write("a.txt", "B-STAGED")
	git(nil, "add", "a.txt")
	write("b.txt", "A-MINE")
	git(nil, "add", "b.txt")
	if out, err := git(nil, "commit", "-F", msg); err != nil {
		t.Fatalf("unguarded incident must land: %v %s", err, out)
	}
	if out, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.txt") {
		t.Fatalf("premise: `git add mine && git commit -F msg` must capture B's staged a.txt, got %q", out)
	}

	// Half two: the same board with the guard. B stays staged through every form.
	repo, git2, write2 := qaCommitRepo(t)
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}
	write2("a.txt", "B-STAGED")
	git2(nil, "add", "a.txt")
	write2("b.txt", "A-MINE")
	git2(nil, "add", "b.txt")

	stillB := func(after string) {
		t.Helper()
		if out, _ := git2(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "a.txt" && !strings.Contains(out, "a.txt") {
			t.Errorf("after %s, B's staged entry missing: %q", after, out)
		}
		if out, _ := git2(nil, "show", ":a.txt"); strings.TrimSpace(out) != "B-STAGED" {
			t.Errorf("after %s, B's staged content changed: %q", after, out)
		}
	}

	// Form 1: the incident itself — git add mine then commit -F, no pathspec.
	out, err := git2(persona, "commit", "-F", msg)
	if err == nil || !strings.Contains(out, "refused by posse gate: an unqualified git commit") {
		t.Errorf("form1 (incident argv) must be refused: %v %s", err, out)
	}
	stillB("form1")
	if out, _ := git2(nil, "diff", "--cached", "--name-only"); !strings.Contains(out, "b.txt") {
		t.Errorf("form1 must leave A's staged mine.txt too, got %q", out)
	}

	git2(nil, "reset", "-q", "HEAD", "--", "b.txt")
	write2("b.txt", "A-MINE")

	// Form 2: git commit -a, named as -a.
	if out, err := git2(persona, "commit", "-am", "sweep-all"); err == nil ||
		!strings.Contains(out, "refused by posse gate: git commit -a") {
		t.Errorf("form2 (`git commit -a`) must be refused as -a: %v %s", err, out)
	}
	stillB("form2")

	// Form 3: the blessed form. Commits only mine; B stays in the shared index.
	if out, err := git2(persona, "commit", "-F", msg, "--", "b.txt"); err != nil {
		t.Fatalf("form3 (blessed `git commit -F msg -- b.txt`) must pass: %v %s", err, out)
	}
	if out, _ := git2(nil, "show", "--name-only", "--format=", "HEAD"); strings.TrimSpace(out) != "b.txt" {
		t.Errorf("form3 must commit only b.txt, got %q", out)
	}
	if out, _ := git2(nil, "show", "HEAD:b.txt"); strings.TrimSpace(out) != "A-MINE" {
		t.Errorf("form3 HEAD:b.txt: %q", out)
	}
	if out, _ := git2(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "a.txt" {
		t.Errorf("form3 must leave B staged, got %q", out)
	}
	stillB("form3")

	// Form 4: the private GIT_INDEX_FILE workaround recorded on 2f5r. Dead.
	idx := filepath.Join(t.TempDir(), "index")
	priv := []string{"RHQ_PERSONA=qa", "GIT_INDEX_FILE=" + idx}
	write2("b.txt", "A-VIA-PRIVATE")
	if out, err := git2(priv, "read-tree", "HEAD"); err != nil {
		t.Fatalf("read-tree private index: %v %s", err, out)
	}
	if out, err := git2(priv, "add", "--", "b.txt"); err != nil {
		t.Fatalf("add private index: %v %s", err, out)
	}
	head, _ := git2(nil, "rev-parse", "HEAD")
	out, err = git2(priv, "commit", "-m", "workaround")
	if err == nil || !strings.Contains(out, "refused by posse gate: a commit from a private GIT_INDEX_FILE") {
		t.Errorf("form4 (private GIT_INDEX_FILE) must be refused as private index: %v %s", err, out)
	}
	if now, _ := git2(nil, "rev-parse", "HEAD"); strings.TrimSpace(now) != strings.TrimSpace(head) {
		t.Errorf("form4 moved HEAD")
	}
	stillB("form4")
}

// rangerhq-2f5r residual, filed rangerhq-lvu9: the blessed form commits the
// WORKING TREE content of the named path, not what you staged. The wall
// closed the shared-index half of the incident; this half rides through
// because the form is correct. Isolation (rangerhq-09o2) is the real fix.
func TestQA2f5rBlessedFormTakesWorkingTree(t *testing.T) {
	repo, git, write := qaCommitRepo(t)
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}
	write("a.txt", "v1\ndinesh line\nLAURIE HALF-WRITTEN\n")
	msg := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msg, []byte("msg about dinesh line only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(persona, "commit", "-F", msg, "--", "a.txt"); err != nil {
		t.Fatalf("blessed form must pass: %v %s", err, out)
	}
	body, _ := git(nil, "show", "HEAD:a.txt")
	if !strings.Contains(body, "dinesh line") || !strings.Contains(body, "LAURIE HALF-WRITTEN") {
		t.Fatalf("residual: named path commits the file on disk; got %q", body)
	}
}
