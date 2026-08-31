package posse

// The launcher lock (ADR 0011 §1): one launcher at a time per RHQ_HOME.
//
// Nothing serialized two launchers before this. A hand-run `posse dispatch`,
// an autostart `--watch` loop and the cockpit's `d` are three processes over
// one bd queue, one meta dir and one herdr, and every guard dispatch has —
// crew-held, working/blocked, prompted-recently, the busy map — is a check
// against state the other two are mutating between the check and the act.
// rangerhq-9nso is what that cost: concurrent passes, two sessions' metas
// gone. The guards are fine; they just were not atomic with the launch they
// authorize. This makes them so.
//
// flock(2), and never a second pidfile. A pidfile records liveness in a file
// whose truth decays — the reader has to infer, and rangerhq-ct9/ppy9 are
// what that inference costs. An flock is held by the open file description,
// so the kernel releases it when the process dies: crash, kill -9, closed
// pane alike. Release *is* process death, which leaves no staleness class to
// detect and nothing to reap. The pid written into the file is a courtesy
// for the operator's eyes only — nothing reads it to decide anything.
//
// The file is created and never removed. Unlinking it would let the next
// launcher create a fresh inode and lock *that* instead: two holders, one
// path, no error anywhere.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// LaunchLockPath is the one launcher lock of an RHQ_HOME, beside the watch
// pidfile it deliberately does not resemble.
func LaunchLockPath(a *App) string {
	return filepath.Join(a.StateDir, "dispatch-launch.lock")
}

// LaunchLock is a held launcher lock.
type LaunchLock struct{ f *os.File }

// Release drops the lock by closing the fd — the same thing process death
// does, which is why forgetting it is a leak and never a stale lock.
// Idempotent, so a caller may both defer it and drop it early.
func (l *LaunchLock) Release() {
	if l == nil || l.f == nil {
		return
	}
	l.f.Close()
	l.f = nil
	launchLockDepth.Add(-1)
}

// launchLockDepth is how many launcher locks THIS process is inside. flock
// is per open file description, so the kernel cannot tell a caller "you
// already hold this" — a second Open+Flock in the same process blocks on
// the first forever, and LOCK_NB turns that deadlock into a spurious
// EWOULDBLOCK against nobody. The two callers that must tell the two apart
// read it here.
//
// It is a fact about the PROCESS, and posse only ever launches, creates and
// prunes from one goroutine: the only goroutines in the non-test code are
// the herdr RPC waiters dispatch parks a `--wait` leg on (dispatch.go), and
// none of them touches a meta file or the lock. If that ever stops being
// true this becomes a lie about the CALLER, and the fix is to pass the held
// lock down rather than to look it up.
var launchLockDepth atomic.Int32

// launchLockMine reports whether this process is already inside the
// launcher lock.
func launchLockMine() bool { return launchLockDepth.Load() > 0 }

// underLaunchLock runs f serialized against every other launcher of this
// RHQ_HOME, and takes the lock only if this process is not already inside
// it. A nested call runs f directly: the lock we hold is the exclusion f
// needed, and re-taking it is the deadlock above, not a second guarantee.
//
// This is the write half of rangerhq-3a5t. The prune's half deliberately
// does NOT come through here — a contended lock there means "spare the
// file", which is a safe answer a create does not have (mustNotOrphan: on
// the write side doing nothing is what destroys the record), so it takes
// tryLockLaunches directly and treats its own process's lock as contention.
func underLaunchLock(a *App, out io.Writer, f func() error) error {
	if launchLockMine() {
		return f()
	}
	lock, err := lockLaunches(a, out)
	if err != nil {
		return err
	}
	defer lock.Release()
	return f()
}

// lockLaunches takes the launcher lock, blocking until it is ours. When
// another launcher holds it, one line says so and names its pid *before* the
// wait: a dispatch that has stopped for a reason must never look like a
// dispatch that has hung.
//
// Callers must not nest it. flock is per open file description, so a second
// Open+Flock in this process waits on the first forever. The callers are
// Run's fire loop, LaunchBead, and VerifyAfter (which acts, and so is a
// launcher for this purpose — rangerhq-th7l); none runs inside another, and
// Run's two are strictly sequential. A caller that can run BOTH inside one
// of those and on its own — CreateSession, which `posse new` reaches
// unlocked and LaunchBead reaches holding it — goes through
// underLaunchLock instead of here.
func lockLaunches(a *App, out io.Writer) (*LaunchLock, error) {
	path := LaunchLockPath(a)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, Die("launcher lock: %v", err)
	}
	// A launcher that cannot take the lock does not launch unserialized:
	// this is the invariant the bead exists for, so its failure is the
	// pass's failure and not a warning line.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, Die("launcher lock: %v", err)
	}
	err = flock(f, syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		fmt.Fprintf(out, "⏳ launcher lock held by %s — waiting (ADR 0011 §1)\n", lockHolder(path))
		err = flock(f, syscall.LOCK_EX)
	}
	if err != nil {
		f.Close()
		return nil, Die("launcher lock %s: %v", AbbrevHome(path), err)
	}
	stampLockHolder(f)
	launchLockDepth.Add(1)
	return &LaunchLock{f: f}, nil
}

// tryLockLaunches takes the launcher lock only if it is free right now. It
// exists for the one caller that must never wait: `posse kill` lands a
// session's worktree branch, and the cockpit's `k` runs that on the TUI's
// single select loop, where a minutes-long wait behind a firing pass is a
// frozen cockpit (rangerhq-09o2). The caller's fallback must be to do
// nothing and say so — never to act unserialized.
func tryLockLaunches(a *App) (*LaunchLock, bool) {
	path := LaunchLockPath(a)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, false
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, false
	}
	if err := flock(f, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, false
	}
	stampLockHolder(f)
	launchLockDepth.Add(1)
	return &LaunchLock{f: f}, true
}

// flock is syscall.Flock with the EINTR retry a blocking LOCK_EX needs in a
// Go process: the runtime's own preemption signal lands on it, and a lock
// wait abandoned by SIGURG would be a launcher that skipped the queue.
func flock(f *os.File, how int) error {
	for {
		err := syscall.Flock(int(f.Fd()), how)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

// stampLockHolder records who holds the lock. A hint for the waiting line,
// never evidence: the lock is the kernel's, and these bytes are read for no
// other purpose. Failing to write them costs the next waiter a pid, not the
// lock.
func stampLockHolder(f *os.File) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return
	}
	if err := f.Truncate(0); err != nil {
		return
	}
	cmd := strings.Join(os.Args, " ")
	cmd = strings.NewReplacer("\n", " ", "\r", " ").Replace(cmd)
	fmt.Fprintf(f, "pid: %d\nsince: %s\ncmd: %s\n",
		os.Getpid(), time.Now().UTC().Format(time.RFC3339), cmd)
}

// lockHolder names the process the waiting line points at. The pid is read
// from the file, so it can be a hair stale — a holder that has just taken
// the lock and not yet stamped it, or the previous one — and a number that
// names nobody is worse than no number. An unreadable or dead pid gets the
// honest generic phrasing instead.
func lockHolder(path string) string {
	pid, err := strconv.Atoi(YamlGet(path, "pid"))
	if err != nil || pid <= 0 || !pidAlive(pid) {
		return "another launcher"
	}
	return "pid " + strconv.Itoa(pid)
}
