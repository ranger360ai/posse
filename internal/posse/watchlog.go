package posse

// The watch loop's own log (ranger-base-n00wn).
//
// $RHQ_HOME/state/dispatch-watch.log is the fleet's retrospective record —
// ADR 0041 and ADR 0028 both cite it, and every question about a pass, a
// seat, a kill or a load reading after the fact is answered out of it. Until
// this file it existed only because somebody was piping into it:
// plugin/autostart.sh built the loop's command as `... 2>&1 | tee -a $LOG`,
// and the tee was the only writer.
//
// That is a record with no owner. A loop restarted by hand — the operator's
// own habit, and the standing bounce the coordinating lane runs on his
// behalf — is a loop started without that pipe, so its output goes to
// whatever the terminal or the scratchpad file of the moment was, and the
// record simply stops. MEASURED on this box: the
// log's last line was 2026-08-31 18:08 and it was read on 09-02 with a LIVE
// loop holding the lock, eleven hand-started generations having written into
// per-session scratchpad files that die with the session that made them.
// Nothing went red for three days, because nothing was watching a file
// whose writer was a shell operator.
//
// So the loop writes it. Watch opens this at the top and tees its own Out
// and Err into it for the loop's life, which makes the record a property of
// the process rather than of how it happened to be invoked. No pipe, no
// redirect and no hand restart can end it, and the two surfaces that would
// have caught the outage — `dispatch --watch-status` and G7 — now read the
// file's age rather than assuming somebody is feeding it.
//
// ROTATION is the shape the hook already had and the `.1` on the box is
// evidence of: one generation at 5 MiB, renamed, never compressed and never
// pruned further. The hook rotated at arm time only, which is once per herdr
// start; a loop that runs for a week between starts grew a single file to
// 12.8 MB (the `.1` found on the box). Rotating on the write that would
// cross the line bounds it in the process that is doing the growing.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WatchLogPath is the loop's log, beside the lock and the pidfile it is the
// evidence half of.
func WatchLogPath(a *App) string { return filepath.Join(a.StateDir, "dispatch-watch.log") }

// WatchLogMax is the generation size, matching plugin/autostart.sh's own
// AUTOSTART_MAXLOG default and INSTALL.md's promise of "one .1 generation
// past 5 MiB". One number in two writers is a number that will drift, and
// the hook's tee is the one being retired, so this is where it now lives.
const WatchLogMax = 5 << 20

// watchLog is the loop's end of that file: an append-only writer that
// rotates itself and can never fail the loop.
//
// Every write goes through one mutex, because the thing being teed is
// d.Out, and since ADR 0028 §1 that stream has several writers at once — a
// gather goroutine per pending bead plus four clocks. Out's own writes are
// serialized by outMu, but the header lines Watch writes directly are not,
// and a rotation is a rename between two Writes either way.
type watchLog struct {
	mu   sync.Mutex
	path string
	max  int64
	f    *os.File
	n    int64
	// broken is set by the first write that fails, so a full disk or a
	// state dir that went away costs one line on stderr rather than one per
	// line of the loop's output forever.
	broken bool
	// report is the ORIGINAL error writer, captured before Watch tees this
	// log into it. Reporting through the tee would recurse into Write and
	// deadlock on mu; this is the writer that cannot.
	report io.Writer
}

// openWatchLog opens (creating) the log for append. The error is the refusal
// to open at all — every caller treats it the way stampWatchPid treats an
// unwritable state dir: it costs the record, never the loop.
func openWatchLog(path string, max int64, report io.Writer) (*watchLog, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	w := &watchLog{path: path, max: max, f: f, report: report}
	// The size is read once and then TRACKED, not re-stat'd per line: the
	// only other writer this file can have is a tee somebody started, and a
	// stat per line to notice it would cost a syscall on every line of the
	// loop's output to make the rotation slightly less late. An undercount
	// rotates late, which is the harmless direction.
	if st, err := f.Stat(); err == nil {
		w.n = st.Size()
	}
	return w, nil
}

// Write appends, rotating first when this line would cross the generation
// size. It ALWAYS reports success to its caller: this writer sits on the
// end of an io.MultiWriter whose first leg is the operator's own terminal,
// and a MultiWriter stops at the first error — a log that returned one
// would silence the pane it is a copy of.
func (w *watchLog) Write(p []byte) (int, error) {
	w.mu.Lock()
	err := w.append(p)
	first := err != nil && !w.broken
	if first {
		w.broken = true
	}
	w.mu.Unlock()
	// Outside the lock and NOT through the tee — see the report field.
	if first && w.report != nil {
		fmt.Fprintf(w.report, "warning: the watch log %s cannot be written: %v — this loop's record stops here\n", AbbrevHome(w.path), err)
	}
	return len(p), nil
}

// append is Write's body, under the lock.
func (w *watchLog) append(p []byte) error {
	if w.f == nil {
		return os.ErrClosed
	}
	// Rotate BEFORE the write that would cross, never after, so a line is
	// never split across two generations — the reader of a retrospective
	// record is grepping for a whole line.
	if w.n > 0 && w.n+int64(len(p)) > w.max {
		if err := w.rotate(); err != nil {
			return err
		}
	}
	n, err := w.f.Write(p)
	w.n += int64(n)
	return err
}

// rotate renames the current generation to `.1` and opens a fresh one. The
// previous `.1` is replaced: one generation, which is what the hook did and
// what the box's own files show.
func (w *watchLog) rotate() error {
	if err := w.f.Close(); err != nil {
		w.f = nil
		return err
	}
	w.f = nil
	if err := os.Rename(w.path, w.path+".1"); err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	w.f, w.n = f, 0
	return nil
}

// Close closes the current generation. Registered by Watch as its FIRST
// defer so it runs LAST (LIFO), after every clock has been joined: a
// backup tick or a watchdog line written while the loop was shutting down
// belongs in the record like any other.
func (w *watchLog) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// WatchLogMtime is when the log was last written, and whether there is one
// to read. It is a stat and not a read: the question both callers ask is
// "is the record still arriving", and the answer is the file's age.
func WatchLogMtime(path string) (time.Time, bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return time.Time{}, false
	}
	return st.ModTime(), true
}

// WatchLogStaleAfter is how long a LIVE loop's log may go untouched before
// the silence is a broken record rather than a quiet shop.
//
// It is not a number somebody liked. A healthy loop is GUARANTEED to write
// within it, by the watchdog (watchdog.go): a loop that has written nothing
// for watchdogBudget prints a line saying so, and keeps reprinting once per
// base interval. So the longest a working loop can leave this file alone is
// the watchdog's own budget plus the tick that notices — and anything past
// that, with the lock still held, is output going somewhere other than here.
//
// The wait-leg term is DefaultPromptWaitMS because this is asked about the
// ARMED loop and plugin/autostart.sh passes no --timeout, so the loop on the
// box carries the constructor's own fifteen minutes. A loop an operator
// started by hand with a longer --timeout has a longer legitimate quiet than
// this computes; that is the limit of reading a running process's cadence
// out of the config describing a different launch, and it is named here
// rather than papered over with slack.
//
// It is also blind to a SLEEP, and deliberately. The watchdog measures awake
// silence (its own "WHAT IT CANNOT SEE"), and this file's mtime is wall
// time, so a box asleep for longer than the budget wakes with a log older
// than this threshold and a loop that did nothing wrong. The loop's next
// pass timer fires within one base interval of the wake and the file goes
// fresh again, so the window is one interval wide and transient — a cost
// worth naming, not one worth a second clock.
func WatchLogStaleAfter(base, maxInterval time.Duration) time.Duration {
	if maxInterval < base {
		maxInterval = base
	}
	return watchdogBudget(maxInterval, DefaultPromptWaitMS) + base
}

// WatchLogNote is the ` · log <path>, written <age> ago` half of the
// `--watch-status` line (ranger-base-n00wn). The path is there because the
// operator's next question after "is it running" is "where is it writing",
// and the ANSWER used to depend on how it was started; the age is there
// because a path with a three-day-old file behind it is the outage this
// bead is about, and naming the path alone would have hidden it.
func WatchLogNote(a *App, now time.Time) string {
	path := WatchLogPath(a)
	mt, ok := WatchLogMtime(path)
	if !ok {
		return " · log " + AbbrevHome(path) + ", absent"
	}
	return " · log " + AbbrevHome(path) + ", written " + BlindFor(now.Sub(mt)) + " ago"
}

// nowOr is the App's clock, or the wall. Tests freeze App.Now; every real
// caller leaves it nil.
func (a *App) nowOr() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}
