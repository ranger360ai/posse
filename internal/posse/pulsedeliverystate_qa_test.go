//go:build posse_arm3

package posse

// The persisted pulse record, after ADR 0027's 2026-09-05 simplification
// (ranger-base-thm0j): state/pulse.yaml is `prompted_fingerprint` and
// `prompted_at` and nothing else, and the four fields that left — `at`,
// `conditions`, `fingerprint`, `renag_interval` — leave without a migration
// job, because the reader only ever asks for the two it wants.
//
// These are the file-shape and old-file pins. The BEHAVIOUR the two fields
// buy (dedup, one fixed repeat interval, retry after a skip, reset on a
// cleared set) is pinned next door in pulse_delivery_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The six-field record as the shipped code wrote it up to 2026-09-05 —
// copied from a real state/pulse.yaml rather than re-derived, since the
// point of the pin is the bytes that are already on operators' disks.
const oldSixFieldPulseState = `at: 2026-09-06T02:40:20Z
fingerprint: question:ranger-base-6wqe|settled-unsent:ranger-base-gjbdl
conditions:
  - question:ranger-base-6wqe
  - settled-unsent:ranger-base-gjbdl
prompted_fingerprint: question:ranger-base-6wqe|settled-unsent:ranger-base-gjbdl
prompted_at: 2026-09-06T02:38:17Z
renag_interval: 30m0s
`

// An old six-field file loads its two useful fields, so the upgrade does not
// re-prompt every shop on its first tick. The `conditions:` block is the arm
// that makes this more than a formality: it is the one non-flat shape in the
// file, and a reader that stumbled on its list items would answer "" for the
// `prompted_fingerprint:` line printed after it.
func TestQAOldSixFieldPulseStateLoadsTheTwoDeliveryFields(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pulse.yaml")
	if err := os.WriteFile(path, []byte(oldSixFieldPulseState), 0o644); err != nil {
		t.Fatal(err)
	}
	s := ReadPulseState(path)
	if want := "question:ranger-base-6wqe|settled-unsent:ranger-base-gjbdl"; s.PromptedFingerprint != want {
		t.Errorf("prompted_fingerprint = %q, want %q", s.PromptedFingerprint, want)
	}
	want, err := time.Parse(time.RFC3339, "2026-09-06T02:38:17Z")
	if err != nil {
		t.Fatal(err)
	}
	if !s.PromptedAt.Equal(want) {
		t.Errorf("prompted_at = %s, want %s", s.PromptedAt, want)
	}
}

// The upgrade is one tick wide: the first write after it replaces the old
// six-field record with the two-field one, whole-file, so nothing that left
// is still sitting on disk pretending to be state.
func TestQAWritingOverAnOldRecordDropsTheFourRemovedFields(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pulse.yaml")
	if err := os.WriteFile(path, []byte(oldSixFieldPulseState), 0o644); err != nil {
		t.Fatal(err)
	}
	in := ReadPulseState(path)
	if err := WritePulseState(path, in); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, ln := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		k, _, ok := strings.Cut(ln, ":")
		if !ok {
			t.Fatalf("state/pulse.yaml line is not a flat key: %q\n%s", ln, b)
		}
		keys = append(keys, k)
	}
	got := strings.Join(keys, ",")
	if want := "prompted_fingerprint,prompted_at"; got != want {
		t.Errorf("state/pulse.yaml keys = %q, want exactly %q:\n%s", got, want, b)
	}
	// And it still round-trips: a rewrite that dropped the two survivors
	// alongside the four casualties would pass the key check above and lose
	// the dedup on every restart.
	out := ReadPulseState(path)
	if out.PromptedFingerprint != in.PromptedFingerprint || !out.PromptedAt.Equal(in.PromptedAt) {
		t.Errorf("round trip lost the delivery record: wrote %+v, read %+v", in, out)
	}
}

// Restart dedup, end to end and through the file rather than through the
// struct: a tick that delivers, then a FRESH Dispatcher reading the same
// state dir, must not re-prompt the same set inside the window. This is the
// pin that a smaller file did not cost the thing the file is for.
func TestQAPulseDedupSurvivesARestart(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pane := id + ":p1"
	unpushedRepo(t, b)

	clock := time.Now()
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	deliveryDispatcher(t, b, &clock).pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
		t.Fatalf("setup: want exactly one prompt, got %d", n)
	}

	// A new Dispatcher is the restart: nothing carries over but the file.
	clock = clock.Add(10 * time.Minute)
	deliveryDispatcher(t, b, &clock).pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
		t.Errorf("a restart inside the renag window re-prompted: %d prompts", n)
	}

	// ...and the window still ends. Without this arm a reader that answered
	// "already prompted, forever" would pass the line above.
	clock = clock.Add(21 * time.Minute)
	deliveryDispatcher(t, b, &clock).pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 2 {
		t.Errorf("past the window a restarted watch must re-prompt once, got %d", n)
	}
}

// A pulse whose delivery record cannot be read is a pulse that prompts
// again: ADR 0027 says best-effort deduplication, not exactly-once, and
// this is the sentence run rather than quoted. The lever is the record
// itself — a corrupt file reads as every field's zero value, which is
// exactly what a crash before the write leaves behind.
func TestQAUnreadableDeliveryRecordRepeatsRatherThanSilences(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pane := id + ":p1"
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
		t.Fatalf("setup: want exactly one prompt, got %d", n)
	}
	if err := os.WriteFile(PulsePath(b.App), []byte("prompted_at: not-a-time\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	clock = clock.Add(time.Minute) // well inside the renag window
	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 2 {
		t.Errorf("a lost delivery record must cost a repeat, not a silence: %d prompts", n)
	}
}

// The reset says nothing about the shop. An empty computed set clears the
// delivery bookkeeping — that is what makes an identical later episode
// fresh — and the tick that does it must still be the tick that says a
// store could not be read, so the clearing is never mistaken for an
// all-clear (ADR 0027; done-when 3's "reset never claims an unobserved
// condition cleared").
//
// The lever is herdr's `list-error`, the fake's whole-shop refusal. Persona
// is "" on purpose: with a target named, an unreadable listing raises the
// `no-live:` carry-over and the set is not empty, which is a different
// tick from the one under test here.
func TestQAEmptySetResetIsNotAnAllClear(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writeBeadsDirs(b.App, []string{t.TempDir()})
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	// The partial line is a DIAGNOSTIC (equietf), so it lands on Err, not
	// on the log the operator's watch tees — captured here rather than
	// asserted against dispatcherOut, which would pass for the wrong
	// reason if the line moved to stdout and would fail forever if it did
	// not exist at all.
	var errOut strings.Builder
	d.Err = &errOut
	cfg := PulseConfig{Armed: true, Persona: "", Renag: 30 * time.Minute}

	// A delivery record from before the outage, planted rather than earned:
	// what is under test is the reset, not how the record got there.
	if err := WritePulseState(PulsePath(b.App), PulseState{
		PromptedFingerprint: "question:ranger-base-old",
		PromptedAt:          clock.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "list-error"), []byte("500|fake herdr: listing refused"), 0o644); err != nil {
		t.Fatal(err)
	}

	d.pulseOnce(cfg)

	if got := errOut.String(); !strings.Contains(got, "pulse: shop check partial") {
		t.Fatalf("an unreadable store must be reported on the tick that resets:\n%s", got)
	}
	out := dispatcherOut(d)
	// Nothing that reads as a verdict about the shop. The tick logs its
	// partial and stops; the condition line and the shop line are for
	// non-empty sets only.
	for _, never := range []string{"→ prompted", "pulse: shop ", "undeliverable"} {
		if strings.Contains(out, never) {
			t.Errorf("a partial, empty tick must not print %q:\n%s", never, out)
		}
	}
	if s := ReadPulseState(PulsePath(b.App)); s.PromptedFingerprint != "" || !s.PromptedAt.IsZero() {
		t.Errorf("an empty computed set must clear the delivery bookkeeping: %+v", s)
	}
}

// The HALF-readable record, which is the other half of the sentence above
// and the one the arm before it cannot reach: `prompted_fingerprint` parses
// and `prompted_at` does not. That record has a standing fingerprint, so
// the dedup arm answers "already prompted" while the renag arm has no
// timestamp to measure a window from — and a reader that took the missing
// timestamp as "no window has elapsed" would go silent for that condition
// set until the set itself changed, which is the suppression ADR 0027 rules
// out by name (a reset/error mistake must never suppress an undelivered
// hint).
//
// Both arms matter. Without the second one, "always due" passes the first
// and turns a hint that repeats every 30m into one that repeats every tick.
//
// The name carries "Pulse" on purpose: `-run 'Pulse|pulse'` is the filter
// this package's pulse work is measured with, and a name outside it is a
// pin that no mutation sweep run that way will ever consult.
func TestQAHalfReadablePulseRecordRepeatsRatherThanSilences(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pane := id + ":p1"
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
		t.Fatalf("setup: want exactly one prompt, got %d", n)
	}
	// The fingerprint is read back from the record the tick just wrote
	// rather than spelled out here: what is under test is a record whose
	// dedup half still MATCHES the standing set, and a hand-written
	// fingerprint would decay into a mismatch the moment a condition key
	// is renamed, passing for the wrong reason.
	fp := ReadPulseState(PulsePath(b.App)).PromptedFingerprint
	if fp == "" {
		t.Fatalf("setup: a delivered pulse left no fingerprint in %s", PulsePath(b.App))
	}
	if err := os.WriteFile(PulsePath(b.App),
		[]byte("prompted_fingerprint: "+fp+"\nprompted_at: not-a-time\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	clock = clock.Add(time.Minute)
	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 2 {
		t.Errorf("a delivery record with no readable timestamp must cost a repeat, not a silence: %d prompts", n)
	}

	// ...and exactly one repeat: the tick above rewrote the record whole,
	// so the renag window it starts is the ordinary one.
	clock = clock.Add(time.Minute)
	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 2 {
		t.Errorf("the repeat must restart the renag window, not prompt every tick: %d prompts", n)
	}
}
