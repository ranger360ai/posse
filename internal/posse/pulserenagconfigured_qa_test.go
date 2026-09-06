package posse

// Two arms of ADR 0027's delivery rule that the 2026-09-05 simplification
// (ranger-base-thm0j) left unmeasured, found while verifying its close under
// ranger-base-4m5tq. Both are pins over code that is already correct; each is
// written as the arm that dies when the code stops being correct.
//
//  1. "Use that one CONFIGURED repeat interval, default 30m" — every existing
//     delivery fixture in the package passes Renag: 30 * time.Minute, which is
//     DefaultPulseRenag to the nanosecond, so no arm could tell a delivery
//     that reads cfg.Renag from one that hardcodes the constant. Measured:
//     replacing cfg.Renag with DefaultPulseRenag in deliverPulse's due rule
//     survived the whole `-run 'Pulse|pulse'` sweep. Both directions here,
//     because a fixture SHORTER than the default only catches the hardcode
//     one way round.
//
//  2. "Only a successful prompt advances delivery state; skipped or failed
//     attempts remain eligible on the next tick" — the package pinned the
//     SKIPS (idle-only, no live session, no target) and not the FAILURE. The
//     fake herdr has carried the lever for it all along (`prompt-error`, used
//     15 times in dispatch_qa_test.go and by no pulse test). Measured:
//     advancing the two fields inside deliverPulse's prompt-failed branch
//     survived the same sweep.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The repeat interval is the one the operator configured, not the constant
// the default happens to equal. ADR 0027 leaves tuning it as the remedy for a
// condition that repeats too often ("the operator may tune the existing
// interval after seeing the result"), so a delivery that quietly ignored
// pulse_renag: would take that remedy away with the suite green.
func TestQAPulseRepeatUsesTheConfiguredIntervalNotTheDefault(t *testing.T) {
	t.Parallel()

	// SHORTER than DefaultPulseRenag: a hardcoded 30m would still be
	// suppressing at +6m.
	t.Run("shorter than the default repeats sooner", func(t *testing.T) {
		t.Parallel()
		b, fake := newTestBackend(t)
		id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
		pane := id + ":p1"
		unpushedRepo(t, b)

		clock := time.Now()
		d := deliveryDispatcher(t, b, &clock)
		cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 5 * time.Minute}
		if cfg.Renag >= DefaultPulseRenag {
			t.Fatalf("fixture: Renag %s must differ from DefaultPulseRenag %s or this pin measures nothing",
				cfg.Renag, DefaultPulseRenag)
		}

		d.pulseOnce(cfg)
		if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
			t.Fatalf("setup: want exactly one prompt, got %d", n)
		}

		// Inside the configured window, so still suppressed — without this
		// the arm below would pass for a delivery that ignored the clock.
		clock = clock.Add(4 * time.Minute)
		d.pulseOnce(cfg)
		if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
			t.Fatalf("4m into a 5m window must stay suppressed, got %d prompts", n)
		}

		clock = clock.Add(2 * time.Minute) // +6m total: past pulse_renag, well inside 30m
		d.pulseOnce(cfg)
		if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 2 {
			t.Errorf("a 5m pulse_renag: must repeat at +6m, got %d prompts — delivery is reading %s, not the configured interval",
				n, DefaultPulseRenag)
		}
	})

	// LONGER than DefaultPulseRenag: a hardcoded 30m would prompt at +31m
	// when the operator asked for two hours of quiet.
	t.Run("longer than the default stays quiet", func(t *testing.T) {
		t.Parallel()
		b, fake := newTestBackend(t)
		id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
		pane := id + ":p1"
		unpushedRepo(t, b)

		clock := time.Now()
		d := deliveryDispatcher(t, b, &clock)
		cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 2 * time.Hour}
		if cfg.Renag <= DefaultPulseRenag {
			t.Fatalf("fixture: Renag %s must exceed DefaultPulseRenag %s or this pin measures nothing",
				cfg.Renag, DefaultPulseRenag)
		}

		d.pulseOnce(cfg)
		if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
			t.Fatalf("setup: want exactly one prompt, got %d", n)
		}

		clock = clock.Add(31 * time.Minute) // past the DEFAULT, nowhere near the configured 2h
		d.pulseOnce(cfg)
		if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
			t.Errorf("a 2h pulse_renag: must still be quiet at +31m, got %d prompts — delivery is reading %s, not the configured interval",
				n, DefaultPulseRenag)
		}

		// ...and the configured window does end, so "quiet" above is the
		// interval and not a delivery that stopped working.
		clock = clock.Add(90 * time.Minute) // +2h1m total
		d.pulseOnce(cfg)
		if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 2 {
			t.Errorf("past the configured 2h the set must repeat once, got %d prompts", n)
		}
	})
}

// A prompt that FAILED is not a prompt that went out: the bookkeeping stays
// where it was, so the same fingerprint is retried on the very next tick
// rather than gated behind the renag interval for a hint nobody received
// (ADR 0027; the bead's "skips/failures retry on later ticks").
//
// The skips were already pinned three ways. This is the failure — herdr
// accepted the pane and then refused the prompt, which is the shape a
// `--wait` timeout or an agent_not_ready leaves.
func TestQAPulseFailedPromptRetriesOnTheNextTick(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pane := id + ":p1"
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	var errOut strings.Builder
	d.Err = &errOut
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	// herdr refuses the prompt itself, after the target has been found.
	errFile := filepath.Join(fake, "prompt-error")
	if err := os.WriteFile(errFile, []byte("agent_not_ready|fake herdr: agent not ready"), 0o644); err != nil {
		t.Fatal(err)
	}

	d.pulseOnce(cfg)

	// The fixture bit: the attempt was made and it failed. Without both
	// halves this pin would pass for a tick that never reached delivery.
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
		t.Fatalf("fixture: delivery must have been ATTEMPTED once, got %d", n)
	}
	if got := errOut.String(); !strings.Contains(got, "pulse: prompt failed for") {
		t.Fatalf("fixture: the failure must be reported on Err:\n%s", got)
	}
	if s := ReadPulseState(PulsePath(b.App)); s.PromptedFingerprint != "" || !s.PromptedAt.IsZero() {
		t.Fatalf("a failed prompt must not advance the delivery record: %+v", s)
	}

	// One tick later — far inside the 30m window — with herdr healthy again.
	if err := os.Remove(errFile); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(2 * time.Minute) // one pulse_interval, not one pulse_renag
	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 2 {
		t.Errorf("the tick after a failed prompt must retry immediately, not wait out the renag interval: %d attempts", n)
	}
	if s := ReadPulseState(PulsePath(b.App)); s.PromptedFingerprint == "" || s.PromptedAt.IsZero() {
		t.Errorf("the retry that succeeded must record the delivery: %+v", s)
	}
}
