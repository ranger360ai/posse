package posse

// Helpers lifted out of autoreap_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// reapCandidateIn is reapCandidate over a caller-chosen dir, so a test can
// hand it a real git checkout rather than a bare temp dir.
func reapCandidateIn(t *testing.T, b *HerdrBackend, dir, name, bead, show string) {
	t.Helper()
	// The kill's own reap guard (reapguard.go) reads the bead through the
	// BACKEND's runner, not the dispatcher's; without this it shells out to
	// the ambient bd and refuses every dirty tree it cannot ask about.
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	if show != "" {
		os.WriteFile(filepath.Join(dir, "fake-show.json"), []byte(show), 0o644)
	}
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: dir, Agent: "ranger", Bead: bead}); err != nil {
		t.Fatal(err)
	}
}

// dispatcherErr captures the sweep's stderr, which errw() otherwise sends
// to the process's own.
func dispatcherErr(t *testing.T, d *Dispatcher) *strings.Builder {
	t.Helper()
	var e strings.Builder
	d.Err = &e
	return &e
}

// fakeBdInTree lets the fake bd answer from inside a session worktree: the
// sweep reads the bead from the SESSION's dir (ADR 0011 — fresh, at reap
// time), and for a worktree session that dir is the tree, not the repo. In
// production the tree's own .beads/redirect carries that (worktree_qa_test
// pins it); the fake reads a file, so it needs one — kept out of git so it
// is not the uncommitted work under test.
func fakeBdInTree(t *testing.T, repo, tree, show string) {
	t.Helper()
	write(t, filepath.Join(tree, "fake-show.json"), show)
	write(t, filepath.Join(repo, ".git", "info", "exclude"), "fake-show.json\nfake-ready.json\n")
}

// ageLaunch moves a session's `launched:` stamp back, which is the only way
// a test can say "and this CLI has been gone a while": settledForReap reads
// that stamp through RelaunchGrace to tell a CLI that has EXITED from one
// that has not finished starting. RelaunchGrace is deliberately not
// shortened for tests (ranger-base-ze9p — it is measured against a session's
// real age), so the session has to be aged instead.
func ageLaunch(t *testing.T, b *HerdrBackend, session string, by time.Duration) {
	t.Helper()
	m, ok := b.readMeta(session)
	if !ok {
		t.Fatalf("no meta for %s to age", session)
	}
	m.Launched = m.Launched.Add(-by)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
}
