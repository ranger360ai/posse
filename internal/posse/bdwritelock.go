package posse

// ADR 0073 D1 — one flock per store, for every bd write Bd.runOnce makes.
//
// bd 0.50.3's no-db mode reads the whole jsonl, changes it in memory and
// writes the whole file back, with no lock of its own: two overlapping
// writers can both print `exit 0` while the second's write clobbers the
// first's (ADR 0073's rig, 34-36/48 creates lost). D1's answer is the field's
// answer to a lost update — a single writer — realized the way the launcher
// lock already is (ADR 0011 §1, launchlock.go): flock(2), held by the open
// file description, never a pidfile. The kernel releases it when the holder
// dies and it needs no reaper.
//
// This file is the in-process half: Bd.runOnce takes the lock itself, for
// every call this persona-free package makes on its own behalf (verify
// batches, dispatch's close/comment/claim calls, the sync self-heal). The
// shim's half — the same lock from a persona's typed `bd` — is ADR 0073 B2,
// a separate bead.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// BdWriteLockWait bounds D1's per-store lock: a write that cannot take it
// within this gives up rather than queueing forever behind a write that is
// still running. ASSUMED equal to Bd's own child timeout (BdTimeout is 3m,
// chosen to cover the slowest thing Bd.run does; this lock only ever waits
// behind ANOTHER write, which is one bd call, so the shorter of the two
// numbers is the honest bound).
const BdWriteLockWait = 30 * time.Second

// bdWriteLockPoll is how often a contended bdWriteLock retries the
// non-blocking flock while it waits out BdWriteLockWait. flock(2) has no
// "wait up to N" mode — LOCK_EX blocks forever and LOCK_EX|LOCK_NB answers at
// once — so a bounded wait is this poll, not a single syscall.
const bdWriteLockPoll = 100 * time.Millisecond

// bdWriteLockDir is ~/.cache/posse/bd-write (XDG_CACHE_HOME honoured) — the
// one directory every bd write lock file on the box lives under. "~/.cache/
// posse is where the box's other slot locks live" (ADR 0073, Alternatives
// rejected; suitelock.go's SuiteLockDir resolves the sibling the same way,
// modulo POSSE_SUITE_LOCK_DIR, which names THAT lock and not this one).
//
// "" when there is no HOME to resolve it from, ExpandTilde's reason
// (ranger-base-a3t1): an invented home would make the lock file a child of a
// directory nobody asked for, and a scrubbed environment is better served by
// the fail-open "lock dir unopenable" arm than by a path it never meant.
func bdWriteLockDir() string {
	if c := os.Getenv("XDG_CACHE_HOME"); c != "" {
		return filepath.Join(c, "posse", "bd-write")
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".cache", "posse", "bd-write")
}

// bdWriteLockPath is the lock file for store: sha256 of store's realpath so
// that a redirect and a direct spelling of the same store share one lock
// (ADR 0072 D2's own reason for realpathing the census target), truncated to
// 16 hex digits — a collision space this box's handful of stores will never
// fill, and the same width the launch lock's sibling cache keys use.
func bdWriteLockPath(dir, store string) string {
	sum := sha256.Sum256([]byte(resolveLinks(store)))
	return filepath.Join(dir, hex.EncodeToString(sum[:])[:16]+".lock")
}

// bdWriteLockHead is the argv D1 stamps into a held lock file: informational
// only (an flock records no holder at all, ADR 0059 D3), and three tokens is
// enough for an operator's eyes to recognize the call without the file
// turning into a second transcript.
func bdWriteLockHead(argv []string) []string {
	if len(argv) > 3 {
		return argv[:3]
	}
	return argv
}

// BdWriteLock is a held per-store bd write lock.
type BdWriteLock struct{ f *os.File }

// Release drops the lock by closing the fd — the same thing process death
// does (launchlock.go's reasoning applies verbatim). Idempotent, and a nil
// receiver is a no-op: callers that took no lock (a read verb, or a degrade
// to unlocked) defer Release unconditionally.
func (l *BdWriteLock) Release() {
	if l == nil || l.f == nil {
		return
	}
	l.f.Close()
	l.f = nil
}

// ClearCloseOnExec drops FD_CLOEXEC from l's fd, so that a later
// syscall.Exec in THIS process hands the open file description to the
// image it becomes rather than having the kernel close it at the exec
// itself (ADR 0073 D2: "bd inherits the lock's open file description, so
// the lock is held exactly as long as bd runs"). os.OpenFile has no
// portable flag to open without CLOEXEC in the first place, so this is the
// fcntl the ADR names as the alternative. Only the gates bd-write arm
// calls this — every other holder (Bd.runOnce, the sync self-heal) never
// execs, and CLOEXEC is exactly what it wants there: no bd child should
// ever inherit one of this process's own lock fds. A nil receiver is a
// no-op, matching Release.
func (l *BdWriteLock) ClearCloseOnExec() error {
	if l == nil || l.f == nil {
		return nil
	}
	_, err := unix.FcntlInt(l.f.Fd(), unix.F_SETFD, 0)
	return err
}

// BdWriteLockTimeoutError is D1's bounded wait running out: another bd write
// against the same store has held the lock for the whole of BdWriteLockWait,
// so this write is refused rather than left to queue forever. The sentence
// is ADR 0073 D1's own, verbatim — a refusal a writer can see is the entire
// point of the lock.
type BdWriteLockTimeoutError struct {
	Store string
	Wait  time.Duration
}

func (e *BdWriteLockTimeoutError) Error() string {
	return fmt.Sprintf("refused by posse gate: %s's bd write lock was held for %s — a bd write is still running against it; nothing was written (ADR 0073)",
		e.Store, e.Wait)
}

// bdWriteDegraded is D1's "lock dir unopenable" arm: one notice, then the
// write proceeds unlocked. A wall that blocks every bd write because this
// cache directory could not be reached — a seat whose cage does not grant
// ~/.cache (ranger-base-r3czg), a scrubbed environment with no HOME — would
// be worse than the race D1 exists to close; D3's read-back is the backstop
// either way (ADR 0073, Consequences).
func bdWriteDegraded(argv []string, reason string) {
	fmt.Fprintf(processNotices(), "posse: bd write lock: %s — %s runs unlocked (ADR 0073)\n",
		reason, strings.Join(bdWriteLockHead(argv), " "))
}

// bdWriteLock takes D1's per-store flock, bounded at wait (Bd.runOnce always
// passes BdWriteLockWait; a test passes a short one so the timeout arm does
// not cost the suite 30s), before a bd write runs. store is the directory
// the call resolves its store against (bdWriteStore); argv is the call the
// lock is about to guard.
//
// A nil, nil return means "proceed unlocked" — see bdWriteDegraded. A
// non-nil error is only ever *BdWriteLockTimeoutError: every other way this
// can fail degrades instead, because nothing about the lock itself is load-
// bearing (the exit hatch in ADR 0073's Consequences: "nothing is lost,
// because the lock files hold nothing").
func bdWriteLock(store string, argv []string, wait time.Duration) (*BdWriteLock, error) {
	dir := bdWriteLockDir()
	if dir == "" {
		bdWriteDegraded(argv, "no HOME to resolve the lock directory under")
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		bdWriteDegraded(argv, fmt.Sprintf("%s: %v", AbbrevHome(dir), err))
		return nil, nil
	}
	path := bdWriteLockPath(dir, store)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		bdWriteDegraded(argv, fmt.Sprintf("%s: %v", AbbrevHome(path), err))
		return nil, nil
	}
	deadline := time.Now().Add(wait)
	for {
		lerr := flock(f, syscall.LOCK_EX|syscall.LOCK_NB)
		if lerr == nil {
			break
		}
		if !errors.Is(lerr, syscall.EWOULDBLOCK) {
			f.Close()
			bdWriteDegraded(argv, fmt.Sprintf("%s: %v", AbbrevHome(path), lerr))
			return nil, nil
		}
		// time.Until, not a pre-emptive "would the next poll overshoot":
		// that version gave up up to one bdWriteLockPoll short of wait,
		// which is the caller's own bound and the number BdWriteLockTimeoutError
		// reports — a wait that ends early is a wait that lied about itself.
		remaining := time.Until(deadline)
		if remaining <= 0 {
			f.Close()
			return nil, &BdWriteLockTimeoutError{Store: store, Wait: wait}
		}
		sleep := bdWriteLockPoll
		if remaining < sleep {
			sleep = remaining
		}
		time.Sleep(sleep)
	}
	stampBdWriteLockHolder(f, argv)
	return &BdWriteLock{f: f}, nil
}

// stampBdWriteLockHolder records who holds the lock, the way
// stampLockHolder (launchlock.go) does: a courtesy for the operator's eyes,
// never evidence — the lock is the kernel's, and failing to write these
// bytes costs the next waiter a name, not the lock.
func stampBdWriteLockHolder(f *os.File, argv []string) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return
	}
	if err := f.Truncate(0); err != nil {
		return
	}
	fmt.Fprintf(f, "pid: %d\nargv: %s\n", os.Getpid(), strings.Join(bdWriteLockHead(argv), " "))
}

// bdWriteStore resolves the store a Bd.runOnce(dir, ...) call will actually
// read and write, the way bdStoreEnv and ADR 0072 D2's own judge do:
//
//   - dir == "" leaves the inherited environment and cwd untouched
//     (bdStoreEnv sets no Env at all in that case), so the store is this
//     process's own binding — $BEADS_DIR if the session set one, else
//     beadsHome(cwd). bdLineCensusHome is that exact resolution, already
//     built for the shim's own process to apply to itself.
//   - dir != "" is a call that NAMES a directory, and bdStoreEnv is about to
//     set the child's BEADS_DIR to beadsHome(dir) (shedding whatever this
//     process inherited) when that resolves to a real directory — so that
//     is the store the lock has to key on.
//
// Store resolution, not the realpath hop: bdWriteLockPath does that, once,
// over whichever of the two this returns.
func bdWriteStore(dir string) string {
	if dir == "" {
		return bdLineCensusHome()
	}
	return beadsHome(dir)
}

// bdKnownReadVerbs is ADR 0073 D1's read list: the bd verbs that never
// change the store, so a call of theirs never takes the write lock. Every
// verb NOT named here — including one bdVerbWrites has never heard of — is a
// write. That is the fail-safe direction D1 chose deliberately: a verb
// nobody listed is serialized, and costs only a wait; a verb wrongly read as
// safe costs the lost update this ADR exists to fix.
var bdKnownReadVerbs = map[string]bool{
	"show":    true,
	"list":    true,
	"ready":   true,
	"blocked": true,
	"info":    true,
	"where":   true,
	"stats":   true,
	"search":  true,
	"count":   true,
	"version": true,
	"help":    true,
	"prime":   true,
	"onboard": true,
}

// bdVerbWrites classifies one bd call — its full argv, bd's own global
// options included — as a write, for Bd.runOnce to decide whether the call
// needs D1's lock. It skips those global options the same way
// ProjectBdWrite (ADR 0072 D1's judge) does — stripBdGlobalOpts, over
// globalValueOpts["bd"] — then reads the verb that is left.
//
// Two verbs carry a subcommand that changes the answer, and bd's own --help
// names exactly one subcommand of each (MEASURED, bd 0.50.3):
//
//   - `comments` alone, or `comments <id>`, lists and is a read; only
//     `comments add <id> <text>` writes. bd ships no third shape.
//   - `dep list <id>` and `dep tree <id>` read the graph; every other `dep`
//     subcommand (`add`, and anything this classifier has not been told
//     about) is a write, by the same fail-safe default as the rest.
//
// Everything else is answered by bdKnownReadVerbs alone, including no verb
// at all (an empty rest): that shape is unreachable from this package's own
// callers today, and treating it as a write rather than guessing costs
// nothing when it never happens.
func bdVerbWrites(argv []string) bool {
	rest, _ := stripBdGlobalOpts(argv)
	if len(rest) == 0 {
		return true
	}
	switch rest[0] {
	case "comments":
		return len(rest) > 1 && rest[1] == "add"
	case "dep":
		return !(len(rest) > 1 && (rest[1] == "list" || rest[1] == "tree"))
	default:
		return !bdKnownReadVerbs[rest[0]]
	}
}

// IsBdWriteLockTimeout reports whether err is D1's write lock giving up
// after BdWriteLockWait — the one failure shape bdWriteLock does not
// degrade past, and the one a caller may want to tell apart from every
// other way a bd write can fail.
func IsBdWriteLockTimeout(err error) bool {
	var te *BdWriteLockTimeoutError
	return errors.As(err, &te)
}
