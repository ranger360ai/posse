package posse

// ranger-base-g5xoy, verifying ranger-base-xc2s4's close: the FIFO class at
// `.beads/redirect` is guarded on the READ and not on the WRITE.
//
// xc2s4 put isRegularFile in front of the read of <repo>/.beads/redirect
// (seedBeadsRedirect, worktree.go), which is right and is pinned by
// redirectfifo_test.go. Its census then listed the next statement in the same
// function —
//
//	os.WriteFile(filepath.Join(dst, "redirect"), []byte(target+"\n"), 0o644)
//
// — as "the WRITE, not a read", i.e. as not being of the class. It is.
// os.WriteFile opens O_WRONLY|O_CREATE|O_TRUNC, and open(2) for write on a
// FIFO with no reader blocks exactly as open for read on one with no writer
// does. MEASURED 2026-09-05 on darwin 25.4.0/go1.26.5: a bare os.WriteFile of
// four bytes to a 0644 FIFO did not return in 10s, with the same call over a
// regular file returning immediately.
//
// `dst` is the SESSION TREE's own .beads, and the seed is re-run into an
// existing tree on purpose ("Seeding is idempotent and re-run on purpose",
// EnsureSessionTree's two branches), so the reachable shape is a relaunch: a
// session replaces its own tree's .beads/redirect with a pipe — a caged
// session has write access to its own worktree, which is a WEAKER precondition
// than the write-access-to-the-main-checkout the read arm needs — and every
// later dispatched launch into that tree hangs with nothing printed and no
// deadline above it. That is xc2s4's own symptom and its own EXPECTED ("the
// launch proceeds") one statement further down.
//
// Controls first, in the same rig, through the same call, so a BLOCKED
// verdict is the file type and not the harness.

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestQAASeedIntoAFifoDestinationMustNotWedgeTheLaunch(t *testing.T) {
	t.Parallel()
	t.Skip("ranger-base-lwfhe (escape from ranger-base-xc2s4, found on ranger-base-g5xoy): seedBeadsRedirect (internal/posse/worktree.go) type-checks the redirect it READS and not the one it WRITES, so a 0644 FIFO at <tree>/.beads/redirect wedges every relaunch into that tree")

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}

	// CONTROL: the relaunch shape with an ordinary regular redirect already
	// at the destination — the seed returns AND rewrites the value, so a
	// "fix" that merely skipped the write would fail here rather than pass
	// by being quiet everywhere.
	ctl := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ctl, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctl, beadsDirName, beadsRedirect), []byte("/stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !returnsWithin(t, 15*time.Second, func() { _ = seedBeadsRedirect(&SessionTree{Repo: repo, Path: ctl}) }) {
		t.Fatalf("CONTROL: a re-seed over a regular redirect blocked — the rig, not the code")
	}
	if got, want := readSeeded(t, ctl), filepath.Join(repo, beadsDirName); got != want {
		t.Errorf("CONTROL: the re-seed left %s, want the repo's own %s", got, want)
	}

	// THE DEFECT: the same re-seed into a tree whose own redirect is a pipe.
	arm := t.TempDir()
	if err := os.MkdirAll(filepath.Join(arm, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(arm, beadsDirName, beadsRedirect)
	if err := syscall.Mkfifo(dst, 0o644); err != nil {
		t.Fatal(err)
	}
	// The fixture's own witness: a pin over a plain file cannot pass here.
	if fi, err := os.Lstat(dst); err != nil || fi.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("fixture: %s is not a FIFO (err %v)", dst, err)
	}
	if !returnsWithin(t, 15*time.Second, func() { _ = seedBeadsRedirect(&SessionTree{Repo: repo, Path: arm}) }) {
		t.Fatalf("seedBeadsRedirect blocked forever WRITING to a 0644 FIFO at %s — the launch does not proceed", dst)
	}
	// And it must end up a real redirect, not merely a call that returned:
	// the pipe is not something posse wrote and cannot be left standing as
	// this tree's account of where the store is.
	if got, want := readSeeded(t, arm), filepath.Join(repo, beadsDirName); got != want {
		t.Errorf("the seed left %s at %s, want the repo's own %s", got, dst, want)
	}
}

func readSeeded(t *testing.T, tree string) string {
	t.Helper()
	p := filepath.Join(tree, beadsDirName, beadsRedirect)
	if fi, err := os.Lstat(p); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("seeded redirect at %s is not a regular file (err %v)", p, err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("seeded redirect at %s: %v", p, err)
	}
	return strings.TrimSpace(string(b))
}
