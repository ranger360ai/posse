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

// A nested launcher lock is a deadlock, not a second guarantee: flock is
// per open file description, so a second Open+Flock in this process blocks
// on the first forever, and LOCK_NB turns that deadlock into a spurious
// EWOULDBLOCK against nobody. So a caller that can run BOTH inside another
// launcher's critical section and on its own has to be able to tell which
// it is.
//
// It used to ask a process-wide counter (launchLockDepth). That answered
// "does this PROCESS hold a lock" while the question is "does this CALLER
// hold it", and the only thing that made the two the same was a claim about
// posse's goroutines: it launched, created and pruned from one. That claim
// was already false when it was written down — cmd/posse/cockpit.go's
// launch() runs LaunchBead, which holds the lock for its whole body, on its
// own goroutine, while the cockpit's select loop lists (Sessions -> reclaim)
// on another. A create started from any second goroutine read the launch
// goroutine's lock as its own and ran its nameFree/writeMeta inside a
// critical section it did not hold — rangerhq-3a5t's window, reopened by
// the mechanism that closed it (ranger-base-deaz).
//
// So the held lock is passed down instead of looked up. The token is the
// proof: holding it is the only way to say "the exclusion f needs is
// already mine", and a goroutine that was handed nothing waits, which is
// what it should have been doing all along. What it costs is that a caller
// which really does hold the lock and passes nil deadlocks on its own open
// file description — loud, and safe: the failure it replaces destroyed
// session records silently (rangerhq-9nso).

// underLaunchLock runs f serialized against every other launcher of this
// RHQ_HOME. held is the launcher lock the CALLER already holds, or nil: a
// caller that hands one over runs f directly under it, because that lock is
// the exclusion f needed and re-taking it is the deadlock above. Every
// other caller — including one on a different goroutine of a process whose
// launcher lock is held elsewhere — waits for it.
//
// f is handed the lock it runs under so it can pass it on in turn: the
// nesting is a chain (RelaunchSession -> replace -> clearDeadMeta), and a
// link that cannot name the lock it is inside is the caller-blind question
// again, one frame down.
//
// This is the write half of rangerhq-3a5t. The prune's half deliberately
// does NOT come through here — a contended lock there means "spare the
// file", which is a safe answer a create does not have (mustNotOrphan: on
// the write side doing nothing is what destroys the record), so it takes
// tryLockLaunches directly and treats any held lock as contention.
func underLaunchLock(a *App, out io.Writer, held *LaunchLock, f func(*LaunchLock) error) error {
	if held != nil {
		return f(held)
	}
	lock, err := lockLaunches(a, out)
	if err != nil {
		return err
	}
	defer lock.Release()
	return f(lock)
}

// lockLaunches takes the launcher lock, blocking until it is ours. When
// another launcher holds it, one line says so and names its pid *before* the
// wait: a dispatch that has stopped for a reason must never look like a
// dispatch that has hung.
//
// Callers must not nest it on one goroutine. flock is per open file
// description, so a second Open+Flock in this process waits on the first
// forever. The callers are Run's fire loop, LaunchBead, and VerifyAfter
// (which acts, and so is a launcher for this purpose — rangerhq-th7l); none
// runs inside another, and Run's two are strictly sequential. A caller that
// can run BOTH inside one of those and on its own — CreateSession, which
// `posse new` reaches unlocked and LaunchBead reaches holding it — goes
// through underLaunchLock instead of here, and is handed the outer lock to
// prove which case it is in.
//
// Waiting here on a lock this process holds on ANOTHER goroutine is
// legitimate and not nesting: the cockpit lists on its select loop while
// LaunchBead launches on its own (cockpit.go). lockHolder says so when it
// happens, because the other reading of that line — a nested caller that
// was handed no lock — is a hang somebody has to recognize.
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

// tryLockLaunches takes the launcher lock only if it is free right now. It
// exists for the one caller that must never wait: `posse kill` lands a
// session's worktree branch, and the cockpit's `k` runs that on the TUI's
// single select loop, where a minutes-long wait behind a firing pass is a
// frozen cockpit (rangerhq-09o2). The caller's fallback must be to do
// nothing and say so — never to act unserialized.
//
// It returns WHY it did not take the lock, "" when it did, in the shape
// prunable and notOurWorkspace already use in this package. A bare false
// said "held" for four unrelated events — an unmakeable state dir, an
// unopenable lock file, an flock that failed for reasons of its own, and
// the one real contention — so every reader downstream, an operator line
// and a test's failure message alike, asserted a cause nobody had
// measured. ranger-base-zppcv is what that costs: one red on the free-lock
// arm reading "the non-blocking take failed on a free lock", byte-identical
// to what a plain ENOTDIR open produces, and a whole verification pass
// spent unable to get past "one of three things happened". Only the
// EWOULDBLOCK arm means the lock is busy; the other three mean this box is
// broken, which is worth saying out loud and never worth guessing.
//
// The contended arm stays silent-and-false for its callers regardless — the
// string is what it is, and nothing here waits, retries or logs.
func tryLockLaunches(a *App) (*LaunchLock, string) {
	path := LaunchLockPath(a)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Sprintf("the launcher lock's directory could not be made: %v", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Sprintf("the launcher lock file could not be opened: %v", err)
	}
	if err := flock(f, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		// EWOULDBLOCK is the only one of these that is another launcher.
		// The rest — EBADF, ENOLCK, EINVAL — are the kernel refusing to
		// answer the question, and neither caller waits on an answer: both
		// defer to "the next quiet pass", which under a told-wrong reason
		// is every pass from here on, each one spared by a launcher that
		// does not exist.
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, "a launcher is running"
		}
		return nil, fmt.Sprintf("the launcher lock could not be taken: %v", err)
	}
	stampLockHolder(f)
	return &LaunchLock{f: f}, ""
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
	if pid == os.Getpid() {
		// Our own pid: a launcher on another goroutine of this process. Two
		// things wear that shape — the cockpit's select loop meeting its own
		// launch goroutine, which is the serialization working, and a nested
		// caller that was handed no lock, which will never be released. The
		// line cannot tell them apart, so it says what it knows.
		return "this process (pid " + strconv.Itoa(pid) + ", another goroutine)"
	}
	return "pid " + strconv.Itoa(pid)
}
