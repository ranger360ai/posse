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

	// A backstop, not a margin: a loop that wedges mid-pass, or one that
	// stops honouring ctx, fails with a sentence instead of hanging until
	// go test's global timeout. No assertion below depends on the loop
	// being faster than this.
	var passes int
	select {
	case passes = <-done:
	case <-time.After(30 * time.Second):
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
