package posse

// Helpers lifted out of launchhookpreheal_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// lhpFixture gives a backend a home with one declared repo, an agent it can
// launch, and returns the backend and the repo's path.
func lhpFixture(t *testing.T, visibility string) (*HerdrBackend, string) {
	t.Helper()
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"), []byte("---\nname: ranger\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := hwsRepo(t, t.TempDir(), "declared")
	if err := os.WriteFile(a.ConfigPath, []byte("beads_visibility:\n  "+repo+": "+visibility+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return b, repo
}

// lhpWorktreeFixture is lhpFixture with a repo a worktree can be cut from: one
// commit on `main`, which `git worktree add` requires and `git init` alone
// does not give.
func lhpWorktreeFixture(t *testing.T, visibility string) (*HerdrBackend, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"), []byte("---\nname: ranger\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := wtRepo(t)
	if err := os.WriteFile(a.ConfigPath, []byte("beads_visibility:\n  "+repo+": "+visibility+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return b, repo
}
