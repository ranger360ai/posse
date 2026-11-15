package posse

import (
	"testing"
	"time"
)

// ─── the derived backstop ────────────────────────────────────────────────────
//
// This file is the house answer to one defect class, and it is here rather
// than beside its first caller because that caller's name was being read as
// the class's boundary. It began as `watchBackstop` in watch_test.go
// (ranger-base-khhnd) for the two watch-loop tests; backoffhold_qa_test.go
// took it next, and ranger-base-1hnqc took it for five launcher-lock tests,
// where "watch" named nothing in the fixture at all.
//
// THE CLASS: a test that hands work to a goroutine and waits for it to
// report, with a `select` whose other arm is a `time.After` CONSTANT. The
// constant is a guess about how fast the box is, and under `go test -race`
// it is the wrong guess — the instrumented build's cost here is WAIT and not
// CPU (13-14% CPU at ~13x the wall, MEASURED in scripts/test-race.sh's
// header), so every such constant is a budget on a wall the build stretches.
// MEASURED 2026-11-14: four tests red under `-race`, green without it, with
// `DATA RACE` 0, at the same timings run alone on a quiet box as under load
// — the deadlines firing, not contention (ranger-base-1hnqc).
//
// WHAT IS NOT IN THE CLASS, and the test that tells them apart: a deadline
// BOUNDED BY SOMETHING ELSE IN THE FIXTURE is an observable and must stay a
// constant. Three live examples, each with the bound written at the site:
// launchlock_test.go's "the gather held the launcher lock" (2s, bounded by
// the fake's 5s prompt delay — past it there is no gather left to observe)
// and launchlock_qa_test.go's "--watch holds the launcher lock across its
// backoff" (10s, bounded by the fixture's own 30s backoff — past it the next
// pass takes the lock legitimately and the assertion goes vacuous). A
// deadline asserting that something does NOT happen yet (150ms, 100ms, 2s)
// is the same: it is the observation, and widening it only slows the suite.
//
// The one in the class is the one whose whole job is to beat `go test`'s
// timeout panic with a better sentence. Nothing a caller asserts depends on
// the goroutine being faster than it.

// backstop is how long a test waits for the work it handed a goroutine to
// report before saying so in a sentence. A backstop, not a margin: a loop
// that wedges mid-pass, a launcher that never takes a released lock, or
// anything that stops honouring ctx fails with a message and the output it
// did print instead of hanging until `go test`'s global timeout.
//
// It is derived from that global timeout rather than being a constant
// (ranger-base-khhnd). The constant was 30s, which a quiet box never noticed
// and a loaded one did: MEASURED 2026-09-07 under `go test -race` in a
// package with ~750 t.Parallel tests, pass 2 of a 40ms loop landed ~21s
// behind pass 1, and TestDryRunWatchTailDoesNotSayDispatched reported "watch
// never returned" about a loop that was running correctly, just slowly
// (docs/notes.d/ranger-base-d0xvw.md). Widening the constant only moves the
// guess; the backstop's whole job is to beat `go test`'s own timeout panic
// with a better sentence, so the deadline it is racing — less the room to
// print — is the honest value, and it costs a genuinely wedged goroutine
// nothing it was not already going to cost the run.
func backstop(t *testing.T) time.Duration {
	t.Helper()
	dl, ok := t.Deadline()
	return backstopBefore(dl, ok, time.Now())
}

// backstopBefore is backstop's arithmetic, split out so it can be pinned
// over inputs a test binary cannot produce on demand (its own -timeout is
// whatever the runner passed).
func backstopBefore(deadline time.Time, ok bool, now time.Time) time.Duration {
	// Room for the Fatalf to be formatted and flushed before `go test`
	// panics over the whole binary and takes the buffered output with it.
	const reporting = 30 * time.Second
	// `-timeout 0`: nothing bounds the binary, so nothing bounds this
	// either, beyond keeping a wedge from sitting there for the afternoon.
	const unbounded = 10 * time.Minute
	if !ok {
		return unbounded
	}
	// A deadline already on top of us: go test will report first whatever we
	// choose, so choose the old floor rather than a zero or negative wait.
	if d := deadline.Sub(now) - reporting; d > reporting {
		return d
	}
	return reporting
}

// The callers cannot exercise this: a test binary's deadline is whatever
// -timeout the runner passed, and the interesting arms are the ends.
//
// Named TestWatchBackstop… while the helper was watchBackstop; the name is
// kept so docs/notes.d/ranger-base-khhnd.md's pointer to it still resolves.
func TestWatchBackstopTracksTheBinaryDeadline(t *testing.T) {
	t.Parallel()
	now := time.Now()
	for _, c := range []struct {
		name     string
		deadline time.Time
		ok       bool
		want     time.Duration
	}{
		{"no -timeout at all", time.Time{}, false, 10 * time.Minute},
		{"make test's 25m", now.Add(25 * time.Minute), true, 25*time.Minute - 30*time.Second},
		{"go test's default 10m", now.Add(10 * time.Minute), true, 10*time.Minute - 30*time.Second},
		// Below the old 30s constant the backstop stops shrinking: go test
		// reports first whatever we pick, and a zero or negative wait would
		// fire instantly and call a healthy loop wedged.
		{"a minute left", now.Add(time.Minute), true, 30 * time.Second},
		{"45s left", now.Add(45 * time.Second), true, 30 * time.Second},
		{"deadline already past", now.Add(-time.Minute), true, 30 * time.Second},
	} {
		if got := backstopBefore(c.deadline, c.ok, now); got != c.want {
			t.Errorf("%s: backstop %s, want %s", c.name, got, c.want)
		}
	}
	// And the live wiring, whatever this binary's -timeout happens to be: a
	// wait no shorter than the constant it replaced.
	if got := backstop(t); got < 30*time.Second {
		t.Errorf("backstop under this binary's own deadline: %s, want >= 30s", got)
	}
}
