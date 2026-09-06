package posse

// The pin for execwrite.go. The defect is a WINDOW, not a value, so every arm
// here is two-sided: the wrong arm — the same measurement with no lock — is
// run for real and has to come out the other way, or "green" here would only
// mean the probe cannot see a fork at all.
//
// Nothing in this file calls t.Parallel, deliberately. It takes ForkLock,
// which every fork in this test binary needs, and it reads whether ForkLock
// is free — a question no sibling fork may be answering at the same moment.
// Go releases the parallel batch only after the sequential pass finishes, so
// a test without t.Parallel has the process to itself; adding it here would
// make the readings below somebody else's.

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// forkLockHeld is the question forkExec's own acquireForkLock asks: is
// ForkLock held for writing right now? A fork cannot start while it is, and
// a fork that cannot start is a fork that cannot inherit a write descriptor.
func forkLockHeld() bool {
	if syscall.ForkLock.TryRLock() {
		syscall.ForkLock.RUnlock()
		return false
	}
	return true
}

// waitForkLockFree polls rather than reading once. A goroutine left running
// by an earlier test can hold ForkLock for the length of one fork, and that
// is not this file's lock — a single read would call it ours and red on
// somebody else's subprocess.
func waitForkLockFree(t *testing.T, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if !forkLockHeld() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: ForkLock is still held for writing after 5s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestUnderForkLockHoldsTheLockForTheWriteAndNoLonger(t *testing.T) {
	// The control for every reading below: free before the window opens, so
	// a "held" reading inside it cannot come from the lock simply always
	// being held.
	waitForkLockFree(t, "the rig proves nothing")

	var inside bool
	if err := underForkLock(func() error { inside = forkLockHeld(); return nil }); err != nil {
		t.Fatal(err)
	}
	if !inside {
		t.Error("the write ran with ForkLock free: a sibling fork could land inside it and inherit the write descriptor")
	}
	waitForkLockFree(t, "ForkLock outlived the write — every fork in this binary would queue behind it")

	// The write's error is the caller's, and the lock comes back either way.
	want := errors.New("disk on fire")
	if err := underForkLock(func() error { return want }); !errors.Is(err, want) {
		t.Errorf("underForkLock must return the write's own error: got %v, want %v", err, want)
	}
	waitForkLockFree(t, "ForkLock outlived a FAILED write")
}

// forkInsideWindow opens a window with run, starts a real fork/exec inside
// it, and reports whether that fork COMPLETED before the window closed. That
// is the whole question: a fork that lands inside the window is a fork whose
// child holds a duplicate of the write descriptor until its own execve.
//
// wait is asymmetric on purpose. In the arm that expects the fork to land,
// it is a red-safety margin that costs nothing when the arm passes — the
// window closes the instant the fork reports. In the arm that expects it to
// be held out, it is the hold itself, paid in full and paid by every other
// fork in the process, so it is short: shortening it can only make this pin
// MISS a defect, never invent one.
func forkInsideWindow(t *testing.T, run func(func() error) error, wait time.Duration) bool {
	t.Helper()
	open := make(chan struct{})
	forked := make(chan error, 1)
	go func() {
		<-open
		cmd := exec.Command("/bin/sh", "-c", ":")
		err := cmd.Start() // this call is the fork
		forked <- err
		if err == nil {
			_ = cmd.Wait()
		}
	}()

	landed := false
	if err := run(func() error {
		close(open)
		select {
		case err := <-forked:
			if err != nil {
				return err
			}
			landed = true
		case <-time.After(wait):
		}
		return nil
	}); err != nil {
		t.Fatalf("the fork could not be started at all: %v", err)
	}
	if landed {
		return true
	}
	// It has to run once the window closes, or the rig measured a fork that
	// was never going to happen and both arms would read the same.
	select {
	case err := <-forked:
		if err != nil {
			t.Fatalf("the fork failed rather than waiting: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the rig proves nothing: the fork never completed even after the window closed")
	}
	return false
}

func TestUnderForkLockKeepsAConcurrentForkOutOfTheWriteWindow(t *testing.T) {
	// The wrong arm, run for real: with no lock the fork lands inside the
	// window every time. Without this the arm below would pass on a rig
	// where the fork simply never happened.
	bare := func(write func() error) error { return write() }
	if !forkInsideWindow(t, bare, 5*time.Second) {
		t.Fatal("the rig proves nothing: with no lock at all the fork did not land inside the window, so nothing below can be the lock's doing")
	}
	if forkInsideWindow(t, underForkLock, 250*time.Millisecond) {
		t.Error("a fork landed inside the write window: its child holds a duplicate of the write descriptor until execve, and an execve of the written file answers ETXTBSY while it does (golang/go#22315, ranger-base-d26ak)")
	}
}

func TestWriteExecutableWritesAFileThatRuns(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fake")
	if err := WriteExecutable(p, []byte("#!/bin/sh\necho first\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o755 {
		t.Errorf("perm must be the caller's: got %04o, want 0755", got)
	}
	out, err := exec.Command(p).Output()
	if err != nil {
		t.Fatalf("the file it wrote does not run: %v", err)
	}
	if string(out) != "first\n" {
		t.Errorf("content: got %q, want %q", out, "first\n")
	}
	// It truncates like the os.WriteFile it stands in for, or a shorter
	// second body would run with the tail of the first still attached.
	if err := WriteExecutable(p, []byte("#!/bin/sh\necho 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(p).Output(); err != nil || string(out) != "2\n" {
		t.Errorf("a rewrite must truncate: got %q, %v", out, err)
	}
	if err := WriteExecutable(filepath.Join(dir, "no", "such", "dir", "x"), nil, 0o755); err == nil {
		t.Error("a write into a missing directory must fail like os.WriteFile does")
	}
	waitForkLockFree(t, "ForkLock outlived a write that failed on a missing directory")
}

// Every arm above is aimed at underForkLock, so a WriteExecutable that had
// quietly gone back to calling os.WriteFile would pass all of them. This one
// is aimed at WriteExecutable itself, and it needs the write to be STOPPABLE
// to ask the question at all: a FIFO gives that for free, because
// os.WriteFile's open(2) of one blocks until a reader arrives, so the window
// stays open exactly as long as this test declines to open it.
func TestWriteExecutableWritesUnderTheForkLock(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Fatal(err)
	}
	waitForkLockFree(t, "the rig proves nothing")

	done := make(chan error, 1)
	go func() { done <- WriteExecutable(p, []byte("#!/bin/sh\nexit 0\n"), 0o755) }()

	// The write is parked in open(2). If it took the lock it is holding it
	// now; if it did not, it never will, and this poll spends its deadline.
	held := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if forkLockHeld() {
			held = true
			break
		}
		time.Sleep(time.Millisecond)
	}

	// Release it either way. A goroutine that outlives this test still
	// parked on the FIFO would hold ForkLock, and every later test's fork
	// in this binary would queue behind it — a red nobody could read.
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, f); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("the write itself failed: %v", err)
	}

	if !held {
		t.Error("WriteExecutable wrote with ForkLock free: a sibling fork can land inside its window and inherit the write descriptor")
	}
	waitForkLockFree(t, "ForkLock outlived WriteExecutable")
}
