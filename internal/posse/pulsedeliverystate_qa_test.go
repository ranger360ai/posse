//go:build posse_arm3

package posse

// The persisted pulse record, after ADR 0027's 2026-09-05 simplification
// (ranger-base-thm0j) and ADR 0093 D2 (ranger-base-hkqpq): state/pulse.yaml
// is `delivered_keys` and `prompted_at` and nothing else, and every field
// that has ever left it — `at`, `conditions`, `fingerprint`,
// `renag_interval`, and now `prompted_fingerprint` — leaves without a
// migration job, because the reader only ever asks for the two it wants.
//
// Two upgrades, one mechanism, and the arms below carry both: a six-field
// record and a `prompted_fingerprint` record each load what still exists and
// get rewritten two keys wide by the first tick after the upgrade. What the
// 0093 one costs is named where it is pinned — delivery reads as unknown,
// so the next due tick says the whole standing set as `+` once.
//
// These are the file-shape and old-file pins. The BEHAVIOUR the two fields
// buy (dedup, one fixed repeat interval, retry after a skip, reset on a
// cleared set) is pinned next door in pulse_delivery_test.go, and the delta
// the keys buy in pulsefeed_test.go.

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

// The record as the shipped code wrote it between the 2026-09-05
// simplification and ADR 0093 D2 — the shape on every operator's disk at
// the moment the delta landed.
const oldFingerprintPulseState = `prompted_fingerprint: question:ranger-base-6wqe|settled-unsent:ranger-base-gjbdl
prompted_at: 2026-09-06T02:38:17Z
`

// An old file loads every field that still EXISTS, and reads the ones that
// left as unset — both old shapes, because the mechanism is one reader
// asking for two keys and the pin has to hold for whatever is on disk.
//
// `prompted_at` is the arm that carries the claim: it survived both upgrades,
// so a reader that stumbled anywhere in either file would lose the renag
// clock and re-deliver on the first tick after the upgrade. The six-field
// file's `conditions:` block is why that is not a formality — it is the one
// non-flat shape either file has ever had, and it sits ABOVE the
// `prompted_at:` line.
//
// `delivered_keys` reading empty off both is the ADR 0093 D2 migration in
// one assertion: delivery is UNKNOWN after the upgrade, not known-empty, and
// the two render the same way, so the next due tick says the standing set as
// `+` once and then deltas forever after.
func TestQAOldPulseStateFilesLoadWhatStillExists(t *testing.T) {
	t.Parallel()
	want, err := time.Parse(time.RFC3339, "2026-09-06T02:38:17Z")
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"six-field (pre 2026-09-05)":      oldSixFieldPulseState,
		"prompted_fingerprint (pre 0093)": oldFingerprintPulseState,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "pulse.yaml")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			s := ReadPulseState(path)
			if !s.PromptedAt.Equal(want) {
				t.Errorf("prompted_at = %s, want %s", s.PromptedAt, want)
			}
			if len(s.DeliveredKeys) != 0 {
				t.Errorf("delivered_keys = %q, want none: neither old shape carries the field", s.DeliveredKeys)
			}
		})
	}
}

// The upgrade is one tick wide: the first write after it replaces the old
// record with the two-field one, whole-file, so nothing that left is still
// sitting on disk pretending to be state. The six-field file is the harder
// input — it carries four removed keys and a block list — and dropping
// `prompted_fingerprint` is checked by the same key census.
func TestQAWritingOverAnOldRecordDropsEveryRemovedField(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pulse.yaml")
	if err := os.WriteFile(path, []byte(oldSixFieldPulseState+oldFingerprintPulseState), 0o644); err != nil {
		t.Fatal(err)
	}
	in := ReadPulseState(path)
	in.DeliveredKeys = []string{"question:ranger-base-6wqe", "settled-unsent:ranger-base-gjbdl"}
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
	if want := "delivered_keys,prompted_at"; got != want {
		t.Errorf("state/pulse.yaml keys = %q, want exactly %q:\n%s", got, want, b)
	}
	// And it still round-trips: a rewrite that dropped the two survivors
	// alongside the casualties would pass the key check above and lose the
	// dedup on every restart. The keys go in a `|`-joined scalar, so this
	// is also the pin that the join and the split are inverses — a reader
	// that split on the wrong byte would hand the next tick one long key
	// and re-announce the whole set as `+`.
	out := ReadPulseState(path)
	if out.Fingerprint() != in.Fingerprint() || !out.PromptedAt.Equal(in.PromptedAt) {
		t.Errorf("round trip lost the delivery record: wrote %+v, read %+v", in, out)
	}
	if len(out.DeliveredKeys) != len(in.DeliveredKeys) {
		t.Errorf("round trip changed the key COUNT: wrote %q, read %q", in.DeliveredKeys, out.DeliveredKeys)
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

// The reset says nothing about the shop, and since ADR 0093 D2 that rule
// has teeth it did not have before: an empty computed set clears the
// delivery bookkeeping and appends `cleared all` to the feed — an all-clear
// in as many words, said to the coordinator — so it may only run when the
// read was COMPLETE. A tick that could not reach a store holds everything
// and says nothing (ADR 0027 done-when 3's "reset never claims an
// unobserved condition cleared", now enforced rather than argued).
//
// Before D2 this arm asserted the opposite — the record cleared on a partial
// tick too — and what made that safe was "it forgets a delivery, it does not
// assert an all-clear, and nothing downstream reads this file for one". Both
// halves of that are now false: the next tick's delta reads the field, and
// the line this arm would write IS the all-clear.
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
	// Feed on, which is the shipped default (`pulse_feed:` absent —
	// LoadPulseConfig) and what this arm needs: the `cleared all` line is
	// the feed's, and the hatch writes no feed at all (ADR 0093 D2's exit
	// hatch, wired in ranger-base-r3qz6). The waiter lock is left alone —
	// `pulse-unarmed` cannot fire with no persona named, and an empty set
	// types nothing either way, so the channel switch never reaches this
	// tick.
	cfg := PulseConfig{Armed: true, Persona: "", Renag: 30 * time.Minute, Feed: true}

	// A delivery record from before the outage, planted rather than earned:
	// what is under test is the hold, not how the record got there.
	// Truncated to the second, because the record round-trips through
	// RFC3339 and a planted stamp with nanoseconds would read back one
	// truncation short of itself — a difference in the WRITER's precision
	// showing up as a failure of the hold this test is about.
	planted := PulseState{
		DeliveredKeys: []string{"question:ranger-base-old"},
		PromptedAt:    clock.Add(-time.Minute).UTC().Truncate(time.Second),
	}
	if err := WritePulseState(PulsePath(b.App), planted); err != nil {
		t.Fatal(err)
	}
	errFile := filepath.Join(fake, "list-error")
	if err := os.WriteFile(errFile, []byte("500|fake herdr: listing refused"), 0o644); err != nil {
		t.Fatal(err)
	}

	d.pulseOnce(cfg)

	if got := errOut.String(); !strings.Contains(got, "pulse: shop check partial") {
		t.Fatalf("an unreadable store must be reported on the tick that holds:\n%s", got)
	}
	out := dispatcherOut(d)
	// Nothing that reads as a verdict about the shop. The tick logs its
	// partial and stops; the condition line and the shop line are for
	// non-empty sets only, and `cleared all` is the verdict D2 added.
	for _, never := range []string{"→ prompted", "pulse: shop ", "undeliverable", "cleared all"} {
		if strings.Contains(out, never) {
			t.Errorf("a partial, empty tick must not print %q:\n%s", never, out)
		}
	}
	if n := len(pulseFeedEntries(PulseFeedPath(b.App))); n != 0 {
		t.Errorf("a partial, empty tick must append nothing to the feed, got %d entries", n)
	}
	s := ReadPulseState(PulsePath(b.App))
	if s.Fingerprint() != planted.Fingerprint() || !s.PromptedAt.Equal(planted.PromptedAt) {
		t.Errorf("a partial, empty tick must HOLD the delivery record: planted %+v, read %+v", planted, s)
	}

	// ...and the other half of the sentence, which is the arm that keeps
	// this from passing for "the reset is gone": with the store readable the
	// same empty set IS a clear, it says so once, and the record resets so
	// that an identical later episode is a fresh delivery.
	if err := os.Remove(errFile); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	d.pulseOnce(cfg)

	if got := dispatcherOut(d); !strings.Contains(got, "cleared all") {
		t.Errorf("a COMPLETE empty tick over a standing record must say cleared all:\n%s", got)
	}
	feed := pulseFeedEntries(PulseFeedPath(b.App))
	if len(feed) != 1 || !strings.HasSuffix(feed[0], " cleared all") {
		t.Errorf("want exactly one `cleared all` entry, got %q", feed)
	}
	if s := ReadPulseState(PulsePath(b.App)); len(s.DeliveredKeys) != 0 || !s.PromptedAt.IsZero() {
		t.Errorf("a complete empty set must clear the delivery bookkeeping: %+v", s)
	}

	// And it says it ONCE: a healthy shop that re-announced its all-clear
	// every two minutes forever would fill the 500-line cap in a day and
	// leave an armed waiter waking for nothing.
	clock = clock.Add(time.Minute)
	d.pulseOnce(cfg)
	if n := len(pulseFeedEntries(PulseFeedPath(b.App))); n != 1 {
		t.Errorf("a cleared shop must append nothing further, got %d entries", n)
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
	fp := ReadPulseState(PulsePath(b.App)).Fingerprint()
	if fp == "" {
		t.Fatalf("setup: a delivered pulse left no keys in %s", PulsePath(b.App))
	}
	if err := os.WriteFile(PulsePath(b.App),
		[]byte("delivered_keys: "+fp+"\nprompted_at: not-a-time\n"), 0o644); err != nil {
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
