package rhq

// The watch-loop lock (rangerhq-gir5). Real flock in a temp RHQ_HOME — no
// fake: flock is per open file description, so two Open+Flock pairs inside
// one test process contend exactly as two processes do, and a faked lock
// would only test the fake.
//
// One test does fork, because one claim here is specifically about a
// process that dies: the kernel drops the lock, which is the whole reason
// this replaced a pidfile nobody could tell from a stale one.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func watchApp(t *testing.T) *App {
	t.Helper()
	return &App{StateDir: filepath.Join(t.TempDir(), "state")}
}

// The two states, and nothing between them.
func TestWatchLoopRunningTracksTheLock(t *testing.T) {
	a := watchApp(t)

	if running, err := WatchLoopRunning(a); err != nil || running {
		t.Fatalf("a fresh RHQ_HOME has no loop: running=%v err=%v", running, err)
	}
	lock, held, err := lockWatch(a)
	if err != nil || held || lock == nil {
		t.Fatalf("the first loop must get the lock: held=%v err=%v", held, err)
	}
	if running, err := WatchLoopRunning(a); err != nil || !running {
		t.Fatalf("a held lock is a running loop: running=%v err=%v", running, err)
	}
	// Two probes must not read each other as the loop — that is why the
	// probe takes LOCK_SH and not LOCK_EX.
	if _, err := WatchLoopRunning(a); err != nil {
		t.Fatalf("second probe: %v", err)
	}
	if running, _ := WatchLoopRunning(a); !running {
		t.Error("a probe consumed the answer")
	}
	lock.Release()
	if running, err := WatchLoopRunning(a); err != nil || running {
		t.Fatalf("a released lock is no loop: running=%v err=%v", running, err)
	}
	lock.Release() // idempotent
}

// One loop per queue, proved rather than guessed: the second one cannot
// take the lock and must not run.
func TestLockWatchRefusesASecondHolder(t *testing.T) {
	a := watchApp(t)
	first, held, err := lockWatch(a)
	if err != nil || held {
		t.Fatalf("first: held=%v err=%v", held, err)
	}
	defer first.Release()

	second, held, err := lockWatch(a)
	if !held || second != nil || err != nil {
		t.Fatalf("a second loop must be refused, got held=%v lock=%v err=%v", held, second, err)
	}
}

// The claim the pidfile could never make: liveness ends when the process
// does. A killed holder leaves the lock free with nothing to reap — no
// staleness window, no `kill -0`, no argv to match.
func TestWatchLockDiesWithItsProcess(t *testing.T) {
	a := watchApp(t)
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestWatchLockHolderChild$", "-test.v")
	child.Env = append(os.Environ(), "POSSE_WATCHLOCK_HOLD="+a.StateDir)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = child.Process.Kill()
		_, _ = child.Process.Wait()
	}()
	// The child says so once the lock is its own.
	buf := make([]byte, 64)
	deadline := time.Now().Add(30 * time.Second)
	got := ""
	for !strings.Contains(got, "held") && time.Now().Before(deadline) {
		n, err := stdout.Read(buf)
		got += string(buf[:n])
		if err != nil {
			break
		}
	}
	if !strings.Contains(got, "held") {
		t.Fatalf("child never took the lock: %q", got)
	}
	if running, err := WatchLoopRunning(a); err != nil || !running {
		t.Fatalf("another process's loop must read as running: running=%v err=%v", running, err)
	}

	// SIGKILL is the pane-killed case: no shutdown, no chance to clean up.
	if err := child.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if _, err := child.Process.Wait(); err != nil {
		t.Fatal(err)
	}
	running, err := WatchLoopRunning(a)
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Error("a killed loop must read as dead immediately — that is the whole point of the lock")
	}
}

// The child half of TestWatchLockDiesWithItsProcess. Inert unless the env
// var selects it, so a plain `go test` never runs it.
func TestWatchLockHolderChild(t *testing.T) {
	dir := os.Getenv("POSSE_WATCHLOCK_HOLD")
	if dir == "" {
		t.Skip("child of TestWatchLockDiesWithItsProcess")
	}
	lock, held, err := lockWatch(&App{StateDir: dir})
	if err != nil || held {
		t.Fatalf("child could not take the lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	os.Stdout.WriteString("held\n")
	time.Sleep(60 * time.Second) // killed by the parent long before this
}

// The status line: liveness from the lock, identity from the pidfile, and a
// missing record costs a name and not the answer.
func TestWatchStatusReadsLockThenPidfile(t *testing.T) {
	a := watchApp(t)

	line, err := WatchStatus(a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "watch-loop: none") {
		t.Errorf("no lock holder must read none, got %q", line)
	}

	lock, _, err := lockWatch(a)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	// Held, with nobody having written the identity half.
	line, err = WatchStatus(a)
	if err != nil {
		t.Fatal(err)
	}
	if line != "watch-loop: running (holder unrecorded)" {
		t.Errorf("a held lock with no pidfile is still a running loop, got %q", line)
	}

	started := time.Date(2026, 8, 26, 9, 30, 0, 0, time.UTC)
	if err := WriteWatchPid(WatchPidPath(a), WatchPid{Pid: 4242, Started: started}); err != nil {
		t.Fatal(err)
	}
	line, err = WatchStatus(a)
	if err != nil {
		t.Fatal(err)
	}
	if line != "watch-loop: running (pid 4242, since 2026-08-26T09:30:00Z)" {
		t.Errorf("the pidfile is what names the holder, got %q", line)
	}

	// And the identity half decides nothing: a stale record whose pid the
	// kernel has handed to somebody else does not make a free lock look
	// held. This is rangerhq-ppy9's whole class, gone.
	lock.Release()
	line, err = WatchStatus(a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "watch-loop: none") {
		t.Errorf("a stale pidfile must not read as a running loop, got %q", line)
	}
}

// The loop refuses to become the second loop, before it has touched the
// queue at all.
func TestWatchRefusesWhenAnotherLoopHoldsTheLock(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	lock, held, err := lockWatch(b.App)
	if err != nil || held {
		t.Fatalf("arranging the other loop: held=%v err=%v", held, err)
	}
	defer lock.Release()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	passes, err := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond)
	if err == nil {
		t.Fatal("a second loop on one queue must refuse")
	}
	if passes != 0 {
		t.Errorf("a refused loop must not have run a pass, ran %d", passes)
	}
	if !strings.Contains(err.Error(), "one loop per queue") {
		t.Errorf("the refusal must name the invariant, got %q", err)
	}
	if _, ok := ReadWatchPid(WatchPidPath(b.App)); ok {
		t.Error("a refused loop must not stamp itself over the running one's record")
	}
}
