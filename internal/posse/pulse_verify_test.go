//go:build posse_arm3

package posse

// Verification-pass additions for the rangerhq-4ish close (ranger-base-360s).
// The bead's config contract says a bad pulse_interval: "disarms just this
// run with a stderr warning rather than failing the watch loop". That was
// asserted only at the config layer (TestLoadPulseConfigBadInterval) and by
// reading watch.go — nothing proved the LOOP survives it, which is the half
// that matters: a watch that dies on a typo'd config key is a fleet that
// stops dispatching.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWatchSurvivesABadPulseInterval(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	blockedSession(t, b, fake, "coordinator-shop", "coordinator")

	repo := wtRepo(t)
	os.WriteFile(b.App.ConfigPath, []byte(
		"pulse_interval: not-a-duration\npulse_persona: coordinator\nbeads:\n  - "+repo+"\n"), 0o644)

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

// ─── rangerhq-44w1 close (ranger-base-k19q) ──────────────────────────────

// TestQAPulseBlockedSessionYieldsExactlyOnePromptWithTheMarker is
// rangerhq-44w1's DONE WHEN sentence, run as written: "fake-herdr blocked
// session yields exactly one prompt with the marker, renag honors backoff".
//
// The delivery tests carry every mechanism (due-ness, the one fixed renag
// interval, idle-only, the crew seam) but all of them raise the condition set with an
// unpushed repo, deliberately, so the target session's status can be held
// fixed. Nothing exercised the shape the bead names and the incident is
// about: a DIFFERENT persona's session goes blocked, and coordinator — idle, and
// the only session that must be prompted — gets exactly one prompt naming
// it. Two live sessions with two different herdr statuses is also the only
// arrangement in which pulseTarget's "first match by agent" can pick wrong.
func TestQAPulseBlockedSessionYieldsExactlyOnePromptWithTheMarker(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	// beads: must name a scratch dir — without it BeadsDirs answers [""] and
	// pulseOnce below reads whatever directory the process started in, which
	// for `go test ./internal/posse` is this checkout and resolves to the
	// operator's live fleet queue (ranger-base-vcj8j, same class as uk0v).
	// The dispatcher's Bd is the test binary here, so today the only cost is
	// the resolution itself; the pin is for the day it is not.
	scratch := t.TempDir()
	writeBeadsDirs(b.App, []string{scratch})
	// Assert the write took rather than trusting it: a missing or misspelled
	// key is not an error and not a red, and the only symptom is that the
	// unit test reads the operator's queue.
	if got := b.App.BeadsDirs(); len(got) != 1 || got[0] != scratch {
		t.Fatalf("fixture is not hermetic: BeadsDirs() = %q, want [%q]", got, scratch)
	}
	writePersona(t, b.App, "developer", "code")
	writePersona(t, b.App, "coordinator", "code")
	mustCreate(t, b, NewSessionOpts{Name: "developer-work", Agent: "developer"})
	mustCreate(t, b, NewSessionOpts{Name: "coordinator-work", Agent: "coordinator"})

	ids := map[string]string{}
	for _, w := range fakeLoadWSFrom(t, fake) {
		ids[w.Label] = w.WorkspaceID
	}
	if ids["developer-work"] == "" || ids["coordinator-work"] == "" {
		t.Fatalf("fixture: both sessions must exist, got %v", ids)
	}
	// One listing, two agents, two statuses — blocked developer, idle coordinator.
	agents := fmt.Sprintf(
		`[{"agent":"claude","agent_status":"blocked","pane_id":%q,"workspace_id":%q},`+
			`{"agent":"claude","agent_status":"idle","pane_id":%q,"workspace_id":%q}]`,
		ids["developer-work"]+":p1", ids["developer-work"], ids["coordinator-work"]+":p1", ids["coordinator-work"])
	if err := os.WriteFile(filepath.Join(fake, "agents.json"), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}

	coordinatorPane := ids["coordinator-work"] + ":p1"
	developerPane := ids["developer-work"] + ":p1"

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)

	log := calls(t, fake)
	if n := strings.Count(log, "agent prompt "+coordinatorPane); n != 1 {
		t.Fatalf("a blocked session must yield exactly one prompt, got %d:\n%s", n, log)
	}
	if !strings.Contains(log, "agent prompt "+coordinatorPane+" Pulse check:") {
		t.Errorf("the prompt must carry the fixed marker:\n%s", log)
	}
	if !strings.Contains(log, "blocked:developer-work") {
		t.Errorf("the prompt must name the condition it was raised by:\n%s", log)
	}
	if strings.Contains(log, "agent prompt "+developerPane) {
		t.Errorf("the blocked session is the SUBJECT of the pulse, never its target:\n%s", log)
	}

	// Renag honours the backoff on the same set, and releases after it.
	clock = clock.Add(10 * time.Minute)
	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+coordinatorPane); n != 1 {
		t.Errorf("inside the renag window the same set must not re-prompt, got %d", n)
	}
	clock = clock.Add(21 * time.Minute)
	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+coordinatorPane); n != 2 {
		t.Errorf("past the renag window the same set must re-prompt once, got %d", n)
	}
}
