package rhq

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rangerhq-bh8: quiet passes back off (double, capped), a busy pass snaps
// back to the base interval.
func TestNextIntervalSchedule(t *testing.T) {
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
func TestWatchStopsOnContext(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(120 * time.Millisecond)
		cancel()
	}()
	passes := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond)
	out := dispatcherOut(d)
	if passes < 2 {
		t.Errorf("want several passes before cancel, got %d:\n%s", passes, out)
	}
	if !strings.Contains(out, "no ready work") || !strings.Contains(out, "next pass in") {
		t.Errorf("watch output missing pass reports:\n%s", out)
	}
}
