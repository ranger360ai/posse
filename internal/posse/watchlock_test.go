package posse

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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// The other half of the invariant, and the one nothing pinned: a running
// loop HOLDS the lock for its whole life, so the hook's probe sees it from
// outside. TestWatchRefusesWhenAnotherLoopHoldsTheLock proves Watch TAKES
// the lock; it stays green if Watch drops it the instant after, because it
// never reaches that line. Measured (ranger-base-w5g2): turning `defer
// lock.Release()` in Watch into an immediate release left every package
// green — a live loop would then read as none, and the autostart hook would
// kill it and put a second loop on the same queue, which is the exact
// failure the lock replaced a pidfile to prevent (rangerhq-ct9/mugy).
func TestWatchHoldsTheLockForItsWholeLife(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); done <- p }()

	// Wait for the loop to be up by the fact it stamps, then ask the
	// question the hook asks — from a second open file description, which
	// is what makes this the same contention a separate process gets.
	up := false
	for i := 0; i < 200 && !up; i++ {
		if w, ok := ReadWatchPid(WatchPidPath(b.App)); ok && w.Pid == os.Getpid() {
			up = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !up {
		cancel()
		<-done
		t.Fatal("the loop never started")
	}
	running, err := WatchLoopRunning(b.App)
	if err != nil {
		cancel()
		<-done
		t.Fatalf("probing a live loop: %v", err)
	}
	if !running {
		cancel()
		<-done
		t.Fatal("a running loop must hold the lock the whole time it runs — a dropped lock reads as no loop and gets replaced")
	}
	// And the same line the hook actually matches on, not just the bool.
	line, err := WatchStatus(b.App)
	if err != nil {
		cancel()
		<-done
		t.Fatalf("watch-status against a live loop: %v", err)
	}
	if !strings.HasPrefix(line, WatchStatusPrefix+"running") {
		cancel()
		<-done
		t.Fatalf("the hook reads this line; a live loop must say running, got %q", line)
	}

	cancel()
	<-done

	if running, err := WatchLoopRunning(b.App); err != nil || running {
		t.Errorf("a loop that ended must release the lock: running=%v err=%v", running, err)
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

// The third arm, and the one the whole rangerhq-ct9 → mugy → ranger-base-rmc
// line is about: a probe that CANNOT ASK must never answer "no loop".
// WatchStatus has three outcomes and only two of them were pinned — held
// reads running, free reads none, and an unanswerable lock (the state dir
// unreadable, flock unsupported by the filesystem, the fd table full) must
// come back as an error with no line at all, because plugin/autostart.sh
// matches on the LINE: a `watch-loop: none` it cannot distinguish from a
// real one authorises kill-and-replace against a live loop and puts a
// second one on the same queue.
//
// Measured (ranger-base-fjf4): collapsing that arm to `running = false` —
// one line, the exact shape of the argv probe's old silence-reads-as-death
// bug — left ., ./cmd/posse and ./internal/rhq all green.
func TestWatchStatusNeverTurnsAnUnaskableQuestionIntoNone(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads through the mode bits, so this fixture cannot block the probe")
	}
	a := watchApp(t)
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A loop IS running — this is the case where reading the silence as
	// death costs a live loop, not merely an accurate answer.
	lock, held, err := lockWatch(a)
	if err != nil || held {
		t.Fatalf("could not arrange a running loop: held=%v err=%v", held, err)
	}
	defer lock.Release()
	if err := os.Chmod(a.StateDir, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(a.StateDir, 0o755)

	// The fixture's own witness: a probe that still answers has not been
	// blocked, and everything below it would then be measuring nothing.
	if running, err := WatchLoopRunning(a); err == nil {
		t.Fatalf("fixture did not block the probe (running=%v) — the assertions below would prove nothing", running)
	}

	line, err := WatchStatus(a)
	if err == nil {
		t.Fatalf("an unaskable question must be an error, got %q", line)
	}
	if strings.Contains(line, "none") {
		t.Errorf("the hook matches on the line: %q reads as a refutation of a live loop", line)
	}
	if line != "" {
		t.Errorf("a failed probe must emit no line at all, got %q", line)
	}
}
