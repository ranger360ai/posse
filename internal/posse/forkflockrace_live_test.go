package posse

// The rig ranger-base-9l77f's folded-in ranger-base-fwuck asked for first:
// something that reds this family on demand, instead of costing a whole
// loaded `make test` pass per data point.
//
// ROOT CAUSE, MEASURED here (darwin/arm64, go1.26.5), not the KeepAlive/Fd
// theory this bead's own description opens with — that one was tried and
// rejected on ranger-base-9l77f, 2026-09-02 (bd comments). flock(2) locks are held by the OPEN
// FILE DESCRIPTION and are inherited by fork(2): "locks are not inherited
// by [execve], and are only released by close" — but fork DOES inherit the
// descriptor, and CLOEXEC only fires AT exec, not at fork. go1.26.5's
// darwin syscall.forkAndExecInChild (src/syscall/exec_libc2.go) calls the
// raw fork trampoline and then does setsid/setpgid/dup2/chroot/credential
// work in the CHILD, all in Go-runtime-owned code, BEFORE it ever reaches
// execve. Any fd open in the PARENT at the instant of fork — including
// another goroutine's held flock — is duplicated into the child and keeps
// that lock alive for as long as the child takes to reach exec (or exit),
// regardless of what the ORIGINAL holder does with its own copy meanwhile.
// So: goroutine A holds the lock, goroutine B forks (any exec.Command,
// anywhere in the same test binary), A releases — and a probe run before
// B's child execs reads the just-freed lock as still held. This needs a
// second forking goroutine in the same process; it is not a property of
// the lock code, and no CLOEXEC flag on the lock fd changes it, because
// the window is fork-to-exec, not open-to-exec.
//
// That is also why "serial, no t.Parallel" (this file's siblings, and
// cmd/testparallel's map) is a correct fix for the SUITE and not a
// papering-over: Go's testing package never runs a non-parallel test
// concurrently with a running batch of parallel ones, so a test that
// itself never calls t.Parallel cannot straddle another test's fork().
// What is NOT yet answered by that mitigation is production: dispatch.go
// runs one launching goroutine per pending bead (ADR 0028 §1), each of
// which takes the launch lock, forks the agent (herdr.go), and releases —
// in the one process `posse dispatch` is. Whether a concurrent
// tryLockLaunches probe (posse kill, herdrback.go) can observe the same
// false-busy read in that shape is not measured here and is filed
// separately (see the bead comment this file's commit lands with).
//
// RUN IT:
//
//	RHQ_FORKRACE_PROBE=1 go test ./internal/posse -run TestForkFlockRaceReadsAReleasedLockAsHeld -v -timeout 3m
//
// Opt-in: it spends real wall time forking real processes to manufacture
// the race, which is wasted cost on every ordinary pass. RHQ_FORKRACE_SECONDS
// overrides the default budget (20s) if the default's rate looks too low
// on a quiet box — the race widens with contention, same as every sighting
// on this bead measured it doing.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestForkFlockRaceReadsAReleasedLockAsHeld(t *testing.T) {
	if os.Getenv("RHQ_FORKRACE_PROBE") == "" {
		t.Skip("opt-in fork/flock stress rig for ranger-base-9l77f — set RHQ_FORKRACE_PROBE=1 to run; see file comment")
	}
	budget := 20 * time.Second
	if s := os.Getenv("RHQ_FORKRACE_SECONDS"); s != "" {
		if n, err := time.ParseDuration(s + "s"); err == nil {
			budget = n
		}
	}

	dir := t.TempDir()

	// Forking goroutines: the only thing this rig needs to manufacture the
	// window is SOME other goroutine in this process calling fork(2), so a
	// no-op child is enough — the race is in the fork, never the exec.
	stop := make(chan struct{})
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = exec.Command("/usr/bin/true").Run()
			}
		}()
	}
	defer close(stop)

	deadline := time.Now().Add(budget)

	// Watch-lock family: lockWatch + WatchLoopRunning, the exact shape
	// TestWatchStatusReadsLockThenPidfile and TestWatchLoopRunningTracksTheLock
	// pin.
	watchAttempts, watchReds := 0, 0
	for time.Now().Before(deadline) {
		a := &App{StateDir: filepath.Join(dir, fmt.Sprintf("watch-%d", watchAttempts%64), "state")}
		lock, held, err := lockWatch(a)
		watchAttempts++
		if err != nil {
			t.Fatalf("lockWatch: %v", err)
		}
		if held {
			// A fresh, never-contended path read as already held: the
			// same phenomenon, caught on the acquire side instead of the
			// reprobe side.
			watchReds++
			continue
		}
		lock.Release()
		running, err := WatchLoopRunning(a)
		if err != nil {
			t.Fatalf("WatchLoopRunning: %v", err)
		}
		if running {
			watchReds++
		}
	}

	// Launch-lock family: lockLaunches + tryLockLaunches, the shape
	// TestLaunchLockFreeAfterRelease and TestTryLockLaunchesDoesNotWait pin.
	deadline = time.Now().Add(budget)
	launchAttempts, launchReds := 0, 0
	for time.Now().Before(deadline) {
		a := &App{StateDir: filepath.Join(dir, fmt.Sprintf("launch-%d", launchAttempts%64), "state")}
		held, err := lockLaunches(a, io.Discard)
		launchAttempts++
		if err != nil {
			t.Fatalf("lockLaunches: %v", err)
		}
		held.Release()
		got, why := tryLockLaunches(a)
		if got == nil {
			launchReds++
			t.Logf("launch red at attempt %d: %s", launchAttempts, why)
			continue
		}
		got.Release()
	}

	t.Logf("watch:  %d attempts, %d reds (%.4f%%)", watchAttempts, watchReds, 100*float64(watchReds)/float64(max(watchAttempts, 1)))
	t.Logf("launch: %d attempts, %d reds (%.4f%%)", launchAttempts, launchReds, 100*float64(launchReds)/float64(max(launchAttempts, 1)))

	if watchReds > 0 {
		t.Errorf("watch lock: a released lock read as held %d/%d times — fork()-duplicates-flocked-fd (ranger-base-9l77f)", watchReds, watchAttempts)
	}
	if launchReds > 0 {
		t.Errorf("launch lock: a released lock read as held %d/%d times — fork()-duplicates-flocked-fd (ranger-base-9l77f)", launchReds, launchAttempts)
	}
}
