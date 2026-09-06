//go:build posse_arm2

package posse

// The loop's own log (ranger-base-n00wn, watchlog.go).

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// THE BEAD. A loop started with no pipe of any kind still writes the
// record: the fixture's Out is an in-memory tap, nothing redirects anything,
// and the file on disk can only have got there from inside the process.
//
// This is the arm that was missing. Every previous pin on the watch loop
// asserted about d.Out, which is exactly the writer a hand restart keeps and
// the tee is not — so all of them stayed green through the three days the
// fleet's log was frozen at 2026-08-31 18:08 with a live loop holding the
// lock.
func TestWatchWritesItsOwnLogWithNoPipe(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	tap := newPassTap(2)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-tap.reached:
			cancel()
		case <-ctx.Done():
		}
	}()
	done := make(chan struct{})
	go func() { d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("watch never returned:\n%s", tap.String())
	}

	log, err := os.ReadFile(WatchLogPath(b.App))
	if err != nil {
		t.Fatalf("the loop kept no log of its own: %v", err)
	}
	got := string(log)
	// The generation banner, which is what tells a reader where one loop's
	// output starts — the line plugin/autostart.sh used to print through the
	// tee it no longer needs.
	if !strings.Contains(got, "== dispatch --watch armed ") {
		t.Errorf("the log has no generation banner:\n%s", got)
	}
	// And the loop's ordinary output: the pass headers and the backoff line,
	// which are the bytes every retrospective question is answered from.
	if !strings.Contains(got, passHeader) || !strings.Contains(got, "next pass in") {
		t.Errorf("the log is missing the loop's pass reports:\n%s", got)
	}
	// The pane still gets everything it got before: the log is a COPY, and a
	// MultiWriter whose second leg silenced the first would trade one
	// blindness for another.
	if !strings.Contains(tap.String(), "next pass in") {
		t.Errorf("teeing the log must not cost the pane its output:\n%s", tap.String())
	}
	// The file is closed by the time Watch returns — nothing writes an
	// instance's state/ after its caller believes the loop is over, which is
	// the rule the pulse join already keeps (watch.go).
	if _, ok := ReadWatchPid(WatchPidPath(b.App)); ok {
		t.Error("the pid record outlived the loop")
	}
}

// One generation, renamed at the size, and a LINE is never split across the
// rename: the file exists to be grepped.
func TestWatchLogRotatesAtItsGeneration(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state", "dispatch-watch.log")
	w, err := openWatchLog(path, 64, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	first := strings.Repeat("a", 40) + "\n"
	second := strings.Repeat("b", 40) + "\n"
	w.Write([]byte(first))
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Fatal("nothing has crossed the size yet — rotating here loses a generation to no purpose")
	}
	w.Write([]byte(second))

	rolled, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("the crossed generation was not kept: %v", err)
	}
	if string(rolled) != first {
		t.Errorf(".1 = %q, want the first generation whole", rolled)
	}
	cur, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(cur) != second {
		t.Errorf("the live generation = %q, want the line that crossed, whole", cur)
	}
}

// A line LARGER than the whole generation must still be written, and must
// not rotate an empty file: the alternative is a rename per line and a `.1`
// holding nothing.
func TestWatchLogNeverRotatesAnEmptyGeneration(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "dispatch-watch.log")
	w, err := openWatchLog(path, 8, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	long := strings.Repeat("x", 40) + "\n"
	w.Write([]byte(long))
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Error("an empty generation was rotated away")
	}
	if got, _ := os.ReadFile(path); string(got) != long {
		t.Errorf("a line past the generation size must still be written, got %q", got)
	}
}

// The log sits on the end of a MultiWriter whose first leg is the operator's
// terminal, and a MultiWriter stops at the first error. So this writer never
// reports one — a full disk must cost the record and not the pane — and it
// says so ONCE rather than per line.
func TestWatchLogFailureIsQuietAndCostsNothingElse(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "dispatch-watch.log")
	var said strings.Builder
	w, err := openWatchLog(path, 1<<20, &said)
	if err != nil {
		t.Fatal(err)
	}
	w.Close() // every write from here on fails

	pane := &strings.Builder{}
	out := io.MultiWriter(pane, w)
	if _, err := out.Write([]byte("first\n")); err != nil {
		t.Errorf("a broken log must not fail the write: %v", err)
	}
	out.Write([]byte("second\n"))
	if pane.String() != "first\nsecond\n" {
		t.Errorf("the pane lost output to a broken log: %q", pane.String())
	}
	if n := strings.Count(said.String(), "cannot be written"); n != 1 {
		t.Errorf("a broken log is reported %d times, want exactly once:\n%s", n, said.String())
	}
	if !strings.Contains(said.String(), "dispatch-watch.log") {
		t.Errorf("the report must name the file: %q", said.String())
	}
}

// The staleness threshold is the watchdog's own guarantee and not a number
// somebody liked: a healthy loop writes within watchdogBudget because the
// watchdog itself prints past it, and the extra base interval is the tick
// that notices.
func TestWatchLogStaleAfterIsTheWatchdogBudgetPlusATick(t *testing.T) {
	t.Parallel()
	base, max := 5*time.Minute, 40*time.Minute
	if got, want := WatchLogStaleAfter(base, max), watchdogBudget(max, DefaultPromptWaitMS)+base; got != want {
		t.Errorf("WatchLogStaleAfter = %s, want %s", got, want)
	}
	// The production shape, spelled out: 2 x 40m + 5m.
	if got, want := WatchLogStaleAfter(base, max), 85*time.Minute; got != want {
		t.Errorf("a 5m/40m loop may be quiet for %s, want %s", got, want)
	}
	// A wait leg longer than the backoff cap is what bounds the quiet, so a
	// tight cap does not make the threshold shorter than one leg.
	if got, want := WatchLogStaleAfter(30*time.Second, 2*time.Minute), 2*(15*time.Minute+HerdrWaitGrace)+30*time.Second; got != want {
		t.Errorf("a 30s/2m loop may be quiet for %s, want %s", got, want)
	}
	// A cap below the base is the caller's own normalisation (Watch does the
	// same), not a shorter budget.
	if got, want := WatchLogStaleAfter(time.Hour, time.Minute), WatchLogStaleAfter(time.Hour, time.Hour); got != want {
		t.Errorf("a cap under the base = %s, want the base's own budget %s", got, want)
	}
}

// `--watch-status` names the log and its age, on both the running and the
// none arm. The hook reads the same line to decide whether to tee, so the
// token is a contract and not decoration.
func TestWatchStatusNamesTheLogAndItsAge(t *testing.T) {
	a := watchApp(t)
	a.Now = func() time.Time { return time.Date(2026, 9, 2, 23, 45, 0, 0, time.UTC) }

	line, err := WatchStatus(a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, " · log ") || !strings.Contains(line, "dispatch-watch.log") {
		t.Errorf("a status line that names no log is the line that hid the outage: %q", line)
	}
	if !strings.Contains(line, "absent") {
		t.Errorf("no log at all must say so, got %q", line)
	}

	// The outage's own reading: a loop holding the lock, and a log last
	// written three days ago.
	lock, held, err := lockWatch(a)
	if err != nil || held {
		t.Fatalf("could not arrange a running loop: held=%v err=%v", held, err)
	}
	defer lock.Release()
	path := WatchLogPath(a)
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stopped := time.Date(2026, 8, 31, 18, 8, 0, 0, time.UTC)
	if err := os.Chtimes(path, stopped, stopped); err != nil {
		t.Fatal(err)
	}
	line, err = WatchStatus(a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, WatchStatusPrefix+"running") {
		t.Fatalf("the lock is held: %q", line)
	}
	if !strings.Contains(line, "written 53h37m ago") {
		t.Errorf("the age is the whole point of naming the log: %q", line)
	}
}
