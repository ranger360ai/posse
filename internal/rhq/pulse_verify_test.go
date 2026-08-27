package rhq

// Verification-pass additions for the rangerhq-4ish close (ranger-base-360s).
// The bead's config contract says a bad pulse_interval: "disarms just this
// run with a stderr warning rather than failing the watch loop". That was
// asserted only at the config layer (TestLoadPulseConfigBadInterval) and by
// reading watch.go — nothing proved the LOOP survives it, which is the half
// that matters: a watch that dies on a typo'd config key is a fleet that
// stops dispatching.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWatchSurvivesABadPulseInterval(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	blockedSession(t, b, fake, "monica-shop", "monica")

	repo := wtRepo(t)
	os.WriteFile(b.App.ConfigPath, []byte(
		"pulse_interval: not-a-duration\npulse_persona: monica\nbeads:\n  - "+repo+"\n"), 0o644)

	const wantPasses = 2
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
	case <-time.After(30 * time.Second):
		t.Fatal("watch never returned: a bad pulse_interval must disarm the pulse, not wedge the loop")
	}
	if passes < wantPasses {
		t.Errorf("watch made %d passes, want >= %d — the bad key must not stop dispatch", passes, wantPasses)
	}
	if strings.Contains(tap.String(), "pulse: blocked:") {
		t.Errorf("a disarmed pulse must never log a condition line:\n%s", tap.String())
	}
	if _, err := os.Stat(PulsePath(b.App)); err == nil {
		t.Error("a disarmed pulse must never write state/pulse.yaml")
	}
}
