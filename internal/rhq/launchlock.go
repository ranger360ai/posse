package rhq

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
// Run's two are strictly sequential.
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
	return &LaunchLock{f: f}, nil
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
