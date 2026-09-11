package posse

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// rangerhq-bh8: quiet passes back off (double, capped), a busy pass snaps
// back to the base interval.
func TestNextIntervalSchedule(t *testing.T) {
	t.Parallel()
	base, max := 10*time.Second, 60*time.Second
	cur := base
	var got []time.Duration
	for i := 0; i < 5; i++ {
		cur = NextInterval(cur, base, max, 0)
		got = append(got, cur)
	}
	want := []time.Duration{20 * time.Second, 40 * time.Second, 60 * time.Second, 60 * time.Second, 60 * time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("quiet pass %d: %s, want %s", i+1, got[i], want[i])
		}
	}
	if n := NextInterval(cur, base, max, 1); n != base {
		t.Errorf("busy pass must reset to base, got %s", n)
	}
	if n := NextInterval(0, base, max, 0); n != base {
		t.Errorf("first quiet pass from zero: %s, want base", n)
	}
	if n := NextInterval(base, base, 5*time.Second, 0); n != base {
		t.Errorf("max below base must still yield base, got %s", n)
	}
}

func TestParseInterval(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]time.Duration{"30": 30 * time.Second, "30s": 30 * time.Second, "2m": 2 * time.Minute} {
		if got, err := ParseInterval(in); err != nil || got != want {
			t.Errorf("ParseInterval(%q) = %s, %v", in, got, err)
		}
	}
	for _, bad := range []string{"0", "-5", "abc", "1x"} {
		if _, err := ParseInterval(bad); err == nil {
			t.Errorf("ParseInterval(%q) should fail", bad)
		}
	}
}

// Watch runs passes until the context ends, and ends between passes.
//
// The cancel is driven by the loop's own output, not by a stopwatch
// (rangerhq-skcv). The earlier shape cancelled 120ms in and wanted two
// passes inside that window; a box under CPU load that spent the whole
// window on pass 1 then failed a test about looping with a message about
// counting. Nothing here asserts how long a pass takes.
func TestWatchStopsOnContext(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	const wantPasses = 3
	tap := newPassTap(wantPasses)
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
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); done <- p }()

	var passes int
	select {
	case passes = <-done:
	case <-time.After(watchBackstop(t)):
		t.Fatalf("watch never returned, though cancel fired on pass %d's header:\n%s", wantPasses, tap.String())
	}
	out := tap.String()
	if passes < wantPasses {
		t.Errorf("cancel only fires on pass %d's header, so the loop must reach it; got %d passes:\n%s", wantPasses, passes, out)
	}
	if !strings.Contains(out, "no ready work") || !strings.Contains(out, "next pass in") {
		t.Errorf("watch output missing pass reports:\n%s", out)
	}
}

// A dry-run watch pass launches nothing, so its tail must not claim it
// dispatched anything (ranger-base-g7ly8, escaped from rangerhq-f3wv: the
// one-shot arm was fixed in 66db3a2, cmd/posse/main.go:832, but the watch
// tail never was).
func TestDryRunWatchTailDoesNotSayDispatched(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	tap := newPassTap(2) // pass 2's header proves pass 1's tail printed
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
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); done <- p }()
	select {
	case <-done:
	case <-time.After(watchBackstop(t)):
		t.Fatalf("watch never returned:\n%s", tap.String())
	}
	out, log := tap.String(), calls(t, fake)
	if strings.Contains(log, "workspace create") {
		t.Fatalf("a dry pass must launch nothing:\n%s", log)
	}
	if !strings.Contains(out, "· next pass in") {
		t.Fatalf("the tail line never printed:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "· next pass in") &&
			strings.Contains(line, "dispatched") && !strings.Contains(line, "would") {
			t.Errorf("a dry pass that launched nothing reports %q", strings.TrimSpace(line))
		}
	}
}

// passHeader is what Watch prints at the top of every pass. Counting it is
// the only account of its progress the loop offers while it is still
// running, so it is what the cancel above waits on.
const passHeader = "\u2500\u2500 pass "

// passTap is a dispatcher sink that records the watch loop's output and
// closes reached once the loop has announced want passes. Synchronized
// because the loop writes it while the cancel goroutine reads it —
// newTestDispatcher's bare strings.Builder is not.
type passTap struct {
	mu      sync.Mutex
	buf     strings.Builder
	want    int
	reached chan struct{}
	closed  bool
}

func newPassTap(want int) *passTap {
	return &passTap{want: want, reached: make(chan struct{})}
}

func (p *passTap) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n, err := p.buf.Write(b)
	if !p.closed && strings.Count(p.buf.String(), passHeader) >= p.want {
		p.closed = true
		close(p.reached)
	}
	return n, err
}

func (p *passTap) String() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buf.String()
}

// ─── the backstop these two watch tests wait on ──────────────────────────────

// watchBackstop is how long a watch test waits for the loop to return before
// saying so in a sentence. A backstop, not a margin: a loop that wedges
// mid-pass, or one that stops honouring ctx, fails with a message and the
// passes it did print instead of hanging until go test's global timeout.
// Nothing either caller asserts depends on the loop being faster than this.
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
// print — is the honest value, and it costs a genuinely wedged loop nothing
// it was not already going to cost the run.
func watchBackstop(t *testing.T) time.Duration {
	t.Helper()
	dl, ok := t.Deadline()
	return backstopBefore(dl, ok, time.Now())
}

// backstopBefore is watchBackstop's arithmetic, split out so it can be pinned
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

// The two callers above cannot exercise this: a test binary's deadline is
// whatever -timeout the runner passed, and the interesting arms are the ends.
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
	if got := watchBackstop(t); got < 30*time.Second {
		t.Errorf("watchBackstop under this binary's own deadline: %s, want >= 30s", got)
	}
}
