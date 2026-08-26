package rhq

// Live pin for rangerhq-09o2's beads clause, run against the real bd rather
// than the fake:
//
//	a persona in its session worktree reads and writes the SAME work graph
//	as the main checkout — no forked database, no staleness warning, and no
//	--allow-stale.
//
//	RHQ_LIVE_BD=1 go test ./internal/rhq -run TestLiveWorktreeSharesOneGraph -v
//
// Env-gated and skipped by default, like the other live pins: it shells out
// to the operator's bd, which has a version, a daemon and a cache, and none
// of those belong in a hermetic suite. Everything it does happens inside one
// t.TempDir — the `bd init` here is the throwaway-database case, never a
// repo anybody keeps.
//
// Measured 2026-08-25, bd 0.49.1, git 2.39.3:
//   - with the redirect this code writes, `bd list` in the worktree returns
//     the main database's rows, including one created in the main checkout a
//     moment earlier, and creates no database of its own;
//   - a bead created FROM the worktree is in the main checkout's database;
//   - the worktree's own checked-out issues.jsonl, materialized with a fresh
//     mtime, raises no staleness warning however far forward its mtime is
//     moved: bd checks the jsonl beside the database it RESOLVED to. Touching
//     the MAIN repo's jsonl forward is what raises "Database out of sync with
//     JSONL" — which is a fact about the main checkout, not about worktrees.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveWorktreeSharesOneGraph(t *testing.T) {
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("no bd on PATH")
	}
	a := wtApp(t)
	repo := wtRepo(t)

	bd := func(dir string, args ...string) (string, error) {
		cmd := exec.Command("bd", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := bd(repo, "init"); err != nil {
		t.Skipf("bd init did not take in a throwaway repo: %v %s", err, out)
	}
	if out, err := bd(repo, "create", "in the main checkout", "-t", "task"); err != nil {
		t.Fatalf("bd create in the main checkout: %v %s", err, out)
	}
	mustGit(t, repo, "add", "-A")
	mustGit(t, repo, "commit", "-q", "-m", "beads")

	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}

	// The control, and it is load-bearing: bd's own pre-commit hook rewrites
	// the main jsonl during the commit above, so the MAIN checkout can be
	// mid-import at this moment. If it is, that is a fact about the main
	// checkout and says nothing about worktrees — settle it there, the way
	// bd tells you to, and then ask the worktree the real question.
	if out, err := bd(repo, "list"); err != nil && strings.Contains(out, "out of sync") {
		if out, err := bd(repo, "sync", "--import-only"); err != nil {
			t.Fatalf("bd sync --import-only in the main checkout: %v %s", err, out)
		}
	}
	if out, err := bd(repo, "list"); err != nil {
		t.Fatalf("the main checkout itself is not readable, so nothing below means anything: %v %s", err, out)
	}

	// One graph: the row created in the main checkout is visible here, and
	// nothing about the read suggests a sync.
	out, err := bd(tr.Path, "list")
	if err != nil {
		t.Fatalf("bd list in the session worktree: %v %s", err, out)
	}
	if !strings.Contains(out, "in the main checkout") {
		t.Errorf("the worktree does not see the main database's rows:\n%s", out)
	}
	for _, bad := range []string{"out of sync", "--import-only", "allow-stale", "Fresh clone"} {
		if strings.Contains(out, bad) {
			t.Errorf("bd said %q in the worktree — the redirect is not holding:\n%s", bad, out)
		}
	}
	if _, err := os.Stat(filepath.Join(tr.Path, ".beads", "beads.db")); err == nil {
		t.Error("the worktree built a database of its own — the graph forked")
	}

	// Writes go to the one database too.
	if out, err := bd(tr.Path, "create", "from the worktree", "-t", "task"); err != nil {
		t.Fatalf("bd create in the worktree: %v %s", err, out)
	}
	if out, err := bd(repo, "list"); err != nil || !strings.Contains(out, "from the worktree") {
		t.Errorf("a bead filed in the worktree is not in the main database: %v\n%s", err, out)
	}

	// The trap named in the bead: the worktree's checked-out jsonl has a
	// fresh mtime by construction. Push it far forward and the read is still
	// clean, because bd checks the jsonl beside the database it resolved to.
	jsonl := filepath.Join(tr.Path, ".beads", "issues.jsonl")
	if _, err := os.Stat(jsonl); err != nil {
		t.Skipf("this bd does not track issues.jsonl here, so there is no trap to spring: %v", err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(jsonl, future, future); err != nil {
		t.Fatal(err)
	}
	out, err = bd(tr.Path, "list")
	if err != nil || strings.Contains(out, "out of sync") {
		t.Errorf("the worktree's own stale jsonl fired bd's staleness check: %v\n%s", err, out)
	}
}
