package posse

// The pulse waiter (ADR 0093 D1/D3, ranger-base-cgr1h): `posse pulse wait
// --after <seq>` blocks until state/pulse.feed holds an entry past <seq>,
// prints those entries, names the cursor to come back with, and exits 0.
//
// It is the channel that replaces the keyboard. The pulse used to type
// `Pulse check: …` into the coordinator's pane, into the same one-byte
// stream the operator types into, and the splice that follows from two
// writers and no lock was measured three times on 2026-11-13 alone. The
// waiter writes nothing to any pane: the coordinator runs it as a
// background command and its runtime re-invokes the session when the
// command exits, so a delivery is an exit rather than a keystroke and the
// splice class is gone by construction rather than by care.
//
// Three things this file is deliberately not:
//
//   - It does not COMPUTE conditions. The watch is the only component that
//     does (ADR 0029) and the only one that writes delivery state (ADR
//     0011); the waiter reads one text file and prints lines out of it. That
//     is what keeps the feed's reader from becoming a second, disagreeing
//     reading of the shop.
//   - It holds no STATE. The cursor lives on the command line (`--after
//     <seq>`), so a late re-arm with an old cursor gets the backlog rather
//     than losing it, and there is nothing of the waiter's to go stale.
//   - It is not a liveness heartbeat. Whether a listener exists is asked of
//     the kernel through flock, below.
//
// WHY A LOCK AND NOT A PIDFILE (ADR 0093 D3, and the canon it follows):
// "is a waiter armed?" is the question D3 switches delivery on, and a file
// that names a pid answers it badly — the file survives the process (crash,
// SIGKILL, a closed pane), and the pid gets recycled, so file-exists proves
// no liveness and pid-answers-signal-0 proves no identity. An flock is held
// by the open file description and the kernel drops it when the last such
// descriptor closes, which process death does: release IS death, with no
// staleness class and nothing to reap. That is exactly the reasoning
// watchlock.go carries for the watch loop, and this file deliberately
// repeats its discipline rather than inventing a second one — including its
// retry budget, for the same measured reason (see lockPulseWait).
//
// Identity stays a SEPARATE question, because the kernel says "a process
// holds this" and not "the process you recorded holds this". The waiter
// writes its pid and start time INSIDE the lock file, where D3 asks for
// them, and nothing reads that body to decide whether a waiter is armed: a
// missing or stale body costs a name in a message and no answer. One file
// rather than the watch's lock-plus-pidfile pair, because the body is two
// fields and the holder of the lock is the only honest writer of it.

import (
	"context"
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

// PulseWaitPoll is how often a blocked waiter re-reads the feed.
//
// A poll and not inotify, which is ranger-base-cgr1h's own "inotify or 2s
// poll" read the boring way: inotify is Linux-only and this tree builds on
// darwin too (ci.yml's macos-latest leg), so the watch-descriptor version
// would need a second implementation beside it or a new dependency, to buy
// latency against a feed that moves 4.6 times an HOUR (ADR 0093's measured
// 70h window). Two seconds of lag on a 2m pulse interval is below the
// resolution of anything downstream, and the cost of the poll itself is one
// stat-and-read of a file capped at 500 lines — no CPU spin, no watcher to
// leak, identical on both platforms.
const PulseWaitPoll = 2 * time.Second

// PulseWaitLockPath is state/pulse.wait.lock: the switch D3 reads, held for
// the waiter's whole life, carrying its pid and start time inside.
//
// Created and never removed, for the reason the launcher's and the watch's
// locks are: unlinking it would let the next waiter create a fresh inode and
// lock THAT instead — two holders, one path, and no error anywhere.
func PulseWaitLockPath(a *App) string {
	return filepath.Join(a.StateDir, "pulse.wait.lock")
}

// PulseWaiter is the identity half: what the lock file's body holds. Flat
// YAML, `pid:` first, so a shell reader is one `sed` — the same shape
// dispatch-watch.pid carries and for the same audience, the operator's eyes.
// It decides nothing.
type PulseWaiter struct {
	Pid     int
	Started time.Time
}

// Line is how a message names the holder. "pid N, since T" where both are
// known, and the degraded spellings where they are not — a body nobody could
// read must still produce a sentence, because the only caller is a refusal
// and a refusal that cannot finish its sentence is worse than a vague one.
func (w *PulseWaiter) Line() string {
	if w == nil || w.Pid <= 0 {
		return "holder unrecorded"
	}
	s := "pid " + strconv.Itoa(w.Pid)
	if !w.Started.IsZero() {
		s += ", since " + w.Started.UTC().Format(time.RFC3339)
	}
	return s
}

// ReadPulseWaiter returns the recorded holder, or false when there is no
// readable record. Never asked whether a waiter is ARMED — that is
// PulseWaiterArmed's question and the kernel's answer.
func ReadPulseWaiter(path string) (*PulseWaiter, bool) {
	if _, err := os.Stat(path); err != nil {
		return nil, false
	}
	pid, err := strconv.Atoi(YamlGet(path, "pid"))
	if err != nil || pid <= 0 {
		return nil, false
	}
	started, _ := time.Parse(time.RFC3339, YamlGet(path, "started"))
	return &PulseWaiter{Pid: pid, Started: started}, true
}

// writePulseWaiter stamps the body through the fd that holds the lock, not
// through a fresh os.WriteFile: the body is a statement by the holder of the
// lock, and writing it down the same descriptor is what makes that true of
// the bytes as well as of the intention. Truncate-then-write rather than
// append, so a second arming of the same path leaves one record and not two.
func writePulseWaiter(f *os.File, w PulseWaiter) error {
	var s strings.Builder
	fmt.Fprintf(&s, "pid: %d\n", w.Pid)
	if !w.Started.IsZero() {
		fmt.Fprintf(&s, "started: %s\n", w.Started.UTC().Format(time.RFC3339))
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.WriteAt([]byte(s.String()), 0); err != nil {
		return err
	}
	return f.Sync()
}

// pulseWaitLock is a held waiter lock.
type pulseWaitLock struct{ f *os.File }

// Release drops it by closing the fd — the same thing process death does,
// which is why forgetting it is a leak and never a stale lock. Idempotent
// and nil-safe.
func (l *pulseWaitLock) Release() {
	if l == nil || l.f == nil {
		return
	}
	l.f.Close()
	l.f = nil
}

// lockPulseWait arms the waiter. Three answers, each owed a different
// response:
//
//	lock, false, nil  — ours, and the body is stamped. Hold it until the
//	                    waiter exits.
//	nil,  true,  nil  — another waiter of this RHQ_HOME holds it for the
//	                    whole retry budget. Refuse: one listener per feed,
//	                    because D3 switches on "the lock is held" and two
//	                    holders would be two coordinators woken by one
//	                    delta.
//	nil,  false, err  — the lock file could not be opened or locked at all.
//	                    Refuse, and here that is a DEPARTURE from
//	                    lockWatch's degraded-but-running arm: a watch loop
//	                    that cannot record itself still dispatches the
//	                    queue, which is its whole job, but a waiter's whole
//	                    job IS to be visible to D3's probe. One that blocked
//	                    without the lock would read as unarmed, so the watch
//	                    would type as well as append — a listener nobody can
//	                    see, silent for up to pulse_renag. Saying so costs
//	                    the coordinator one turn and leaves it on the typed
//	                    channel knowingly.
//
// The retry budget is watchlock.go's, shared rather than re-chosen: D3's
// probe takes LOCK_SH for the instant it takes to learn nothing holds
// LOCK_EX, and a released flock on darwin has been measured still reading as
// held ~100ms later (planmeterspend_qa_test.go's footnote, 2 runs in 12). A
// waiter arming in that instant must outlast the probe rather than read it as
// another waiter; 250ms over ten tries is 2.5x that measurement and costs a
// genuine second waiter nothing, because a real one holds LOCK_EX for its
// whole life.
func lockPulseWait(a *App, now time.Time) (*pulseWaitLock, bool, error) {
	path := PulseWaitLockPath(a)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, false, err
	}
	deadline := time.Now().Add(watchLockRetryBudget)
	for {
		err = flock(f, syscall.LOCK_EX|syscall.LOCK_NB)
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			break
		}
		if !time.Now().Before(deadline) {
			f.Close()
			return nil, true, nil
		}
		time.Sleep(watchLockRetryPoll)
	}
	if err != nil {
		f.Close()
		return nil, false, err
	}
	// The body is identity and identity decides nothing (watchpid.go's
	// rule): a stamp that cannot be written costs the operator a name in a
	// message, and the lock — which is the switch — is already held. Failing
	// the waiter over it would turn a cosmetic loss into a silent channel.
	_ = writePulseWaiter(f, PulseWaiter{Pid: os.Getpid(), Started: now})
	return &pulseWaitLock{f: f}, false, nil
}

// PulseWaiterArmed asks the kernel whether a waiter of this RHQ_HOME is
// armed — D3's switch, exported here so the watch arm (ranger-base-r3qz6)
// reads the lock through this one reading rather than writing a second one.
//
// The probe takes a SHARED lock, not an exclusive one, exactly as
// WatchLoopRunning's does: shared conflicts with the waiter's LOCK_EX, which
// is the question, while two probes racing each other both succeed — so the
// watch's tick never reports another tick's probe as the listener.
//
// The error is "could not ask", never "no waiter". A caller must keep those
// apart: reading an unanswerable probe as unarmed is what types a pulse into
// a pane somebody is listening on, which is the splice this whole ADR exists
// to end.
func PulseWaiterArmed(a *App) (bool, error) {
	path := PulseWaitLockPath(a)
	// Read-only, and it creates nothing: asking a question must not leave
	// state behind, and a file no waiter has made yet is a file no waiter
	// can be holding. flock is on the open file description and does not
	// care that this one is O_RDONLY.
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

// pulseFeedAfter is the whole of the waiter's reading of the feed: which
// entries a cursor is owed, whether the feed can still account for that
// cursor, and the cursor to hand back.
//
// GAP is "the feed cannot account for --after", and it has two shapes, which
// is one more than ADR 0093's prose names:
//
//   - the cursor fell off the back. The cap keeps the newest
//     PulseFeedLines entries, so a coordinator that slept through more than
//     that comes back to a feed whose oldest entry is past its cursor + 1.
//     This is D2's case: "`--after` asking for a seq older than the oldest
//     kept line prints `gap` and then what is kept."
//   - the feed RESTARTED under the cursor. Lose state/pulse.feed — which
//     ADR 0093 says costs an armed waiter one `gap` — and the next delivery
//     starts again at 1, so every kept entry is numbered below a cursor of
//     400. Read by the first shape alone that is not a gap but a silence:
//     nothing is ever greater than 400 again and the waiter blocks until its
//     deadline, forever, on a shop that is in fact delivering. So a feed
//     whose highest entry is below the cursor is a gap too.
//
// On either, every kept entry is printed, because on both the question "what
// did I miss" has no answer narrower than "what is left". The seq filter is
// what serves the ordinary case.
//
// Entries whose first field is not a sequence number are skipped rather than
// printed or counted. One writer owns this file and rewrites it whole
// through a rename, so an unreadable line is damage and not a record; the
// writer's own next-sequence rule reads the file the same way (the MAXIMUM
// parseable seq plus one), and two readings that agreed on nothing else
// would be how a feed and its reader drift apart.
func pulseFeedAfter(entries []string, after int) (lines []string, gap bool, last int) {
	var kept []string
	oldest, newest := 0, 0
	for _, e := range entries {
		n, ok := PulseFeedSeq(e)
		if !ok {
			continue
		}
		if len(kept) == 0 || n < oldest {
			oldest = n
		}
		if n > newest {
			newest = n
		}
		kept = append(kept, e)
	}
	if len(kept) == 0 {
		// Nothing readable: a fresh instance, a feed not yet written, or one
		// that is entirely damage. None of the three is a gap — a gap is a
		// statement about entries that EXISTED and are gone — and all three
		// are "nothing new yet", which is what the waiter is for.
		return nil, false, after
	}
	if oldest > after+1 || newest < after {
		return kept, true, newest
	}
	for _, e := range kept {
		if n, ok := PulseFeedSeq(e); ok && n > after {
			lines = append(lines, e)
			if n > last {
				last = n
			}
		}
	}
	if len(lines) == 0 {
		return nil, false, after
	}
	return lines, false, last
}

// PulseWaitOptions is the waiter's whole input. After is the cursor; Renag
// and Poll are zero in every real call and set only by tests — the
// coordinator's command line carries the cursor and nothing else, because a
// waiter with tuning knobs is a waiter whose behaviour differs between the
// arming the PID documents and the arming somebody typed.
type PulseWaitOptions struct {
	After int
	Renag time.Duration
	Poll  time.Duration
}

// pulseWaitLife is how long a waiter may live with nothing to say: one
// pulse_renag, which is the interval at which the watch re-says an unchanged
// set (ADR 0027 §3).
//
// It is a BOUND and not a timeout, and the thing it bounds is wedge
// detection (ADR 0093's "what breaks if this is wrong"): a waiter that holds
// the lock but cannot print silences the typed path, and from outside there
// is no way to tell that process from a healthy one waiting on a quiet shop
// — both are "holding the lock, saying nothing". Giving the healthy one a
// maximum life makes the difference observable: a waiter older than
// pulse_renag is wedged, and nothing has to reason about its stack to say so.
//
// A pulse that is DISARMED still gets DefaultPulseRenag rather than zero.
// LoadPulseConfig answers a config with no pulse_interval: with a zero
// PulseConfig, and reading that zero as the life would exit the waiter the
// instant it armed — which, re-invoked by the coordinator's runtime, is the
// hot spin the poll above is written to avoid. The waiter's contract does
// not depend on the pulse being armed: it blocks on a feed, and a feed that
// nobody writes is a quiet shop, said once per interval.
func pulseWaitLife(a *App, opt PulseWaitOptions) time.Duration {
	if opt.Renag > 0 {
		return opt.Renag
	}
	if cfg, err := LoadPulseConfig(a); err == nil && cfg.Renag > 0 {
		return cfg.Renag
	}
	return DefaultPulseRenag
}

// PulseWait is `posse pulse wait --after <seq>` (ADR 0093 D1): arm the lock,
// block on the feed, print what the cursor is owed, name the next cursor,
// exit 0.
//
// EVERY exit prints the `next:` line last, and that is the invariant the
// coordinator's PID is written against: a delta, a gap, a renag self-exit
// and an interrupt all end by saying what to come back with, so re-arming
// never has to parse the lines above it or remember anything across the
// turn. The only exits without one are the two refusals, which return an
// error and print nothing — there is no cursor to hand back when nothing was
// ever armed.
//
// The clock is split on purpose. The lock body's `started:` is stamped from
// a.nowOr(), so a test can pin the recorded identity; the deadline and the
// poll run on real time, because a fake clock does not make a blocking wait
// finish and a waiter whose life could be faked would be a waiter whose
// wedge bound could be.
func PulseWait(ctx context.Context, a *App, opt PulseWaitOptions, out io.Writer) error {
	lock, held, err := lockPulseWait(a, a.nowOr())
	if held {
		w, _ := ReadPulseWaiter(PulseWaitLockPath(a))
		return Die("a pulse waiter is already armed (%s) — one listener per feed (%s)",
			w.Line(), AbbrevHome(PulseWaitLockPath(a)))
	}
	if err != nil {
		return Die("pulse waiter lock %s: %v — not arming, because a waiter the watch cannot see is a channel that goes quiet without saying so", AbbrevHome(PulseWaitLockPath(a)), err)
	}
	defer lock.Release()

	life := pulseWaitLife(a, opt)
	poll := opt.Poll
	if poll <= 0 {
		poll = PulseWaitPoll
	}
	deadline := time.Now().Add(life)
	for {
		lines, gap, last := pulseFeedAfter(pulseFeedEntries(PulseFeedPath(a)), opt.After)
		if gap || len(lines) > 0 {
			if gap {
				fmt.Fprintln(out, "gap")
			}
			for _, ln := range lines {
				fmt.Fprintln(out, ln)
			}
			return pulseWaitNext(out, last)
		}
		left := time.Until(deadline)
		if left <= 0 {
			// The renag self-exit. ADR 0093 calls this "printing `still:`",
			// and the word is the ADR's; what follows it is NOT the keys,
			// which is the one place this verb reads differently from the
			// page.
			//
			// WHY: the feed's own `still: <keys>` line is the watch's, and
			// it is the one body that names keys precisely because the watch
			// is what computes them (ADR 0029). The waiter computes nothing
			// and remembers nothing, so the only honest subject it has for
			// the word is the feed's silence — and in the healthy case it
			// never has to improvise one, because a live watch appends its
			// own keys-naming `still:` and the waiter wakes on that line and
			// prints it verbatim through the arm above. This arm is reached
			// when NO line arrived at all: the watch is dead, the pulse is
			// disarmed, or the feed cannot be written. A line naming keys
			// here would be naming them from nowhere.
			fmt.Fprintf(out, "still: nothing in the feed for %s\n", life)
			return pulseWaitNext(out, opt.After)
		}
		// Never sleep past the deadline, and never shrink the poll itself
		// doing it: the last nap before a self-exit is short, the ones
		// before it are not.
		nap := poll
		if left < nap {
			nap = left
		}
		t := time.NewTimer(nap)
		select {
		case <-ctx.Done():
			t.Stop()
			// SIGINT or SIGTERM: the operator, or the runtime taking its
			// background commands down with the session. Not an error — the
			// waiter was asked to stop, and it has nothing it failed to do
			// — so it exits 0 having named the cursor, exactly like the
			// other two exits.
			return pulseWaitNext(out, opt.After)
		case <-t.C:
		}
	}
}

// pulseWaitNext prints the re-arming line and is the only place its spelling
// lives. The coordinator pastes this line back, so the text is the contract:
// it must be a command that runs, with the cursor in it.
func pulseWaitNext(out io.Writer, after int) error {
	_, err := fmt.Fprintf(out, "next: posse pulse wait --after %d\n", after)
	return err
}
