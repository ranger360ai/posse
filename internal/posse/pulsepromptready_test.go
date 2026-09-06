//go:build posse_arm2

package posse

// ranger-base-k99a: the pulse was the second ungated AgentPrompt caller.
// Its guard — "only prompt a session whose status is idle|done" — read that
// status off herdr's agent listing, and the listing is exactly where herdr
// GUESSES: a pane holding a known agent that no rule matched is reported
// idle with matched_rule null (ranger-base-3p0, promptready.go). So the one
// caller that already believed it was being careful was believing a guess.
//
// The lever is fakeExplain's, as in promptready_test.go: `explain-fallback`
// is the guess, `explain-state` is what a SEEN screen says.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pulseFastRuntime gives a created session a runtime with a short declared
// patience, so a test that measures the pulse's REFUSAL costs a fraction of
// a second instead of the claude-shaped 45. Same lever a slow CLI uses in
// production — the gate's wait is the session's own runtime's (promptReadyWait).
func pulseFastRuntime(t *testing.T, b *HerdrBackend, session, wait string) {
	t.Helper()
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuntime(t, b.App, "fastcli", "command: fastcli --sys {file}\nstartup_wait: "+wait+"\n")
	m, ok := b.readMeta(session)
	if !ok {
		t.Fatalf("no meta for session %q", session)
	}
	m.Runtime = "fastcli"
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
}

// The bug. herdr's listing says idle; herdr's own explain says it only
// guessed that. Nothing is typed, the skip is logged, and the bookkeeping
// is untouched so the next tick tries again — a pulse that never went out
// must not be gated behind the renag window.
func TestPulseDoesNotPromptAPaneHerdrOnlyGuessesAt(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pulseFastRuntime(t, b, "coordinator-work", "400ms")
	unpushedRepo(t, b)
	if err := os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644); err != nil {
		t.Fatal(err) // guess forever: the CLI never draws a screen posse knows
	}

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)

	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("a guessed screen was prompted anyway:\n%s", log)
	}
	out := dispatcherOut(d)
	for _, want := range []string{"pulse: skipped", "coordinator-work", "nothing was sent"} {
		if !strings.Contains(out, want) {
			t.Errorf("the skip must say %q:\n%s", want, out)
		}
	}
	// Undelivered leaves no trace: the same fingerprint is due next tick.
	if state := ReadPulseState(PulsePath(b.App)); state.PromptedFingerprint != "" || !state.PromptedAt.IsZero() {
		t.Errorf("a pulse that never went out advanced its bookkeeping: %+v", state)
	}
}

// The wrong arm, and the reason this file is not just a way to switch the
// pulse off: with no lever the fake answers the shape a settled pane really
// has — a named rule — and the pulse delivers exactly as it did before.
func TestPulseStillPromptsAPaneHerdrHasSeen(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pane := id + ":p1"
	pulseFastRuntime(t, b, "coordinator-work", "400ms")
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)

	if !strings.Contains(dispatcherOut(d), "→ prompted coordinator-work") {
		t.Errorf("a seen idle screen must still take the pulse:\n%s", dispatcherOut(d))
	}
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
		t.Errorf("want exactly one prompt, got %d:\n%s", n, calls(t, fake))
	}
}

// The half of the fix a bare AwaitPromptable call would NOT have bought.
// The gate's own rule is weaker than the pulse's: it opens on any screen
// herdr recognizes, working included, because `posse prompt` by hand may
// legitimately nudge an agent mid-turn. A shop check may not. So the pulse
// re-asks its idle|done question of the detection the gate opened on —
// here a listing that says idle over a screen herdr can see is working,
// which is the state the listing lags behind on every real turn.
func TestPulseSkipsAWorkingScreenTheListingCalledIdle(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pulseFastRuntime(t, b, "coordinator-work", "400ms")
	unpushedRepo(t, b)
	if err := os.WriteFile(filepath.Join(fake, "explain-state"), []byte("working"), 0o644); err != nil {
		t.Fatal(err)
	}

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)

	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("a shop check landed in the middle of a turn:\n%s", log)
	}
	out := dispatcherOut(d)
	for _, want := range []string{"pulse: skipped", "working", `listing said "idle"`} {
		if !strings.Contains(out, want) {
			t.Errorf("the skip must say %q:\n%s", want, out)
		}
	}
}

// The concession, one level up. An `agent explain` that never answers is a
// verb this herdr does not have, not a measurement — and a pulse that went
// silent against an older herdr would be a worse regression than the race.
// The listing's idle is all there is, and it is used.
func TestPulseStillPromptsWhenHerdrCannotExplain(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pulseFastRuntime(t, b, "coordinator-work", "400ms")
	unpushedRepo(t, b)
	if err := os.WriteFile(filepath.Join(fake, "explain-error"),
		[]byte("bad_request|unknown subcommand explain"), 0o644); err != nil {
		t.Fatal(err)
	}

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)

	out := dispatcherOut(d)
	if !strings.Contains(out, "→ prompted coordinator-work") {
		t.Errorf("a failed diagnostic silenced the pulse:\n%s", out)
	}
	if !strings.Contains(out, "cannot explain") {
		t.Errorf("the concession must be named out loud in the watch log:\n%s", out)
	}
}
