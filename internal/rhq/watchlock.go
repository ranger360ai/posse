package rhq

// The watch-loop lock (rangerhq-gir5; ADR 0011 Consequences → ops): the one
// `posse dispatch --watch` loop of an RHQ_HOME holds flock(2) on
// state/dispatch-watch.lock for its whole life, and anything asking "is the
// fleet's loop running?" asks the kernel instead of inferring.
//
// What it retires is an inference. dispatch-watch.pid plus `ps -p <pid> -o
// command=` had to reconstruct liveness from a file whose truth decays, and
// the reconstruction was patched three times: a recycled pid reads as alive
// (the hole rangerhq-ct9's close left), a one-shot `dispatch --persona`
// whose argv merely contains the word reads as the watch loop
// (rangerhq-ppy9), and a `ps` that cannot answer reads as alive or dead
// depending on which arm you wrote (rangerhq-mugy, ranger-base-rmc). None
// of those is a bug in a patch. They are the standing cost of asking
// evidence a question it cannot answer — the store-disagreement class ADR
// 0011 names.
//
// An flock is held by the open file description, so the kernel drops it
// when the process dies: crash, kill -9, closed pane alike. Held means a
// loop is running, free means none is, release *is* process death. No
// staleness class, nothing to reap, no argv to match.
//
// dispatch-watch.pid stays, demoted to what it is good at: which pid, since
// when, under what argv — the identity half of the husk check, and the
// operator's companion to dispatch-watch.log. It decides nothing. A missing
// or stale one costs a phrase in a message and changes no answer.
//
// The file is created and never removed, for the same reason the launcher
// lock is: unlinking it would let the next loop create a fresh inode and
// lock *that* instead — two holders, one path, no error anywhere.

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// WatchLockPath is the one watch-loop lock of an RHQ_HOME, beside the
// pidfile whose job it took over.
func WatchLockPath(a *App) string {
	return filepath.Join(a.StateDir, "dispatch-watch.lock")
}

// WatchLock is a held watch-loop lock. Deliberately not the launcher's
// LaunchLock: the two answer different questions on different files, and one
// process holds both at once for most of a pass.
type WatchLock struct{ f *os.File }

// Release drops the lock by closing the fd — the same thing process death
// does, which is why forgetting it is a leak and never a stale lock.
// Idempotent, and nil-safe so a degraded loop can defer it unconditionally.
func (l *WatchLock) Release() {
	if l == nil || l.f == nil {
		return
	}
	l.f.Close()
	l.f = nil
}

// lockWatch takes the watch-loop lock for the life of this loop. Three
// answers, and the caller owes each a different response:
//
//	lock, false, nil  — ours. Hold it until the loop ends.
//	nil,  true,  nil  — another loop of this RHQ_HOME holds it. Do not run:
//	                    one loop per queue is the invariant, and unlike the
//	                    pidfile this is proof rather than a guess.
//	nil,  false, err  — the lock file could not be opened at all. Degraded,
//	                    not fatal: an unwritable state dir costs the record,
//	                    never the loop (the rule stampWatchPid already
//	                    follows). The loop runs unlocked and the hook reads
//	                    it as no loop — the same blindness as a posse too
//	                    old to stamp anything, and the caller says so.
func lockWatch(a *App) (*WatchLock, bool, error) {
	path := WatchLockPath(a)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, false, err
	}
	err = flock(f, syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		f.Close()
		return nil, true, nil
	}
	if err != nil {
		f.Close()
		return nil, false, err
	}
	return &WatchLock{f: f}, false, nil
}

// WatchLoopRunning asks the kernel whether a `posse dispatch --watch` loop
// of this RHQ_HOME is running. The probe takes a SHARED lock, not an
// exclusive one: shared conflicts with the loop's LOCK_EX — which is the
// question — while two probes racing each other both succeed, so a second
// hook run never reports the first one as the loop.
//
// The error is "could not ask", never "no loop". Every caller must keep
// those apart: reading an unanswerable probe as "no loop" is what
// kill-and-replaces a live loop and puts a second one on the same queue
// (rangerhq-ct9/mugy).
func WatchLoopRunning(a *App) (bool, error) {
	path := WatchLockPath(a)
	// Read-only, and it creates nothing: asking a question must not leave
	// state behind, and a file no loop has made yet is a file no loop can
	// be holding. flock is on the open file description and does not care
	// that this one is O_RDONLY.
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	err = flock(f, syscall.LOCK_SH|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	// Ours only for the instant it took to learn nothing holds it.
	_ = flock(f, syscall.LOCK_UN)
	return false, nil
}

// WatchStatusPrefix is the token `posse dispatch --watch-status` answers
// with, and the contract plugin/autostart.sh matches on. The LINE is the
// answer; the exit status says only whether the question could be asked. A
// posse too old to know the subcommand fails the flag parse, exits non-zero
// and prints nothing that starts with this — which is the "could not ask"
// case, and the hook stands down on it rather than replacing a loop it
// cannot see.
const WatchStatusPrefix = "watch-loop: "

// WatchStatus is that line. Liveness comes from the lock; the parenthetical
// comes from dispatch-watch.pid and is for the operator's eyes — an absent
// or stale record loses a pid, never the answer.
func WatchStatus(a *App) (string, error) {
	running, err := WatchLoopRunning(a)
	if err != nil {
		return "", Die("watch-loop lock %s: %v", AbbrevHome(WatchLockPath(a)), err)
	}
	if !running {
		return WatchStatusPrefix + "none (" + AbbrevHome(WatchLockPath(a)) + " is free)", nil
	}
	w, ok := ReadWatchPid(WatchPidPath(a))
	if !ok {
		return WatchStatusPrefix + "running (holder unrecorded)", nil
	}
	who := "pid " + strconv.Itoa(w.Pid)
	if !w.Started.IsZero() {
		who += ", since " + w.Started.UTC().Format(time.RFC3339)
	}
	return WatchStatusPrefix + "running (" + who + ")", nil
}
