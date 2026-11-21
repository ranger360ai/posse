//go:build posse_arm3

package posse

// ADR 0093 D3, ranger-base-r3qz6: delivery switches on the waiter's lock.
// A waiter that is LISTENING makes the feed line the delivery and nothing is
// typed; a free lock — or one held by a waiter past its own maximum life —
// falls back to the typed pulse, whose text gains the command that arms the
// feed. `pulse_feed: off` is the exit hatch and ADR 0027's path unchanged.
//
// WHY THIS MATTERS AT ALL, measured: the pulse typed into the coordinator's
// pane, the operator types into that same one-byte stream, and three
// coordinator turns on 2026-11-13 came out spliced (`I think thats
// accurPulse check: …`). D1's waiter writes to no pane, so the splice is
// impossible wherever it is armed — and THIS file is what decides whether it
// is used, so every arm here is about a `Prompt` that must not happen or
// must.
//
// ARM 3, beside pulsefeed_test.go and pulse_test.go, because what these
// drive is pulseOnce and what they assert is mostly the FEED: the entries,
// the record, the log lines. The waiter's own acquisition pins are arm 2's
// (pulsewait_test.go) and stay there. What travels with the lock rather than
// with the arm is the serial discipline: every test here that arms or probes
// the lock carries no t.Parallel and is named in cmd/testparallel's serial
// map, because two flock acquisition tests side by side have been measured
// reading a released lock as still held (ranger-base-9l77f).
//
// The lock is armed through lockPulseWait — the waiter's own code, never a
// hand-rolled flock — so the thing the watch probes is the thing a real
// `posse pulse wait` holds. flock is per open file description, so one
// process taking it twice contends exactly as two processes do.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// psCfg is the delivery config these arms share: armed, the coordinator as
// the persona, ONE renag interval and the feed on. The interval is 2m
// because the wedge bound is renag plus one tick and a test that sets no
// interval would be judging holders against a bound of exactly renag.
func psCfg() PulseConfig {
	return PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute,
		Interval: 2 * time.Minute, Feed: true}
}

// psArmWaiter arms the waiter lock with `started` inside it and releases it
// when the test ends — one listener, exactly as `posse pulse wait` holds it.
func psArmWaiter(t *testing.T, b *HerdrBackend, started time.Time) *pulseWaitLock {
	t.Helper()
	lock, held, err := lockPulseWait(b.App, started)
	if err != nil || held || lock == nil {
		t.Fatalf("arming the waiter lock: held=%v err=%v", held, err)
	}
	t.Cleanup(lock.Release)
	// The probe has to agree before anything leans on it: a fixture whose
	// lock is not actually held would make every assertion below an
	// assertion about a free lock.
	if armed, err := PulseWaiterArmed(b.App); err != nil || !armed {
		t.Fatalf("the fixture's lock does not read as armed: armed=%v err=%v", armed, err)
	}
	return lock
}

// psCoordinator is the fixture every arm starts from: a live idle
// coordinator session and one standing condition that has nothing to do with
// session status, so the tick is due and the set is non-empty.
// The standing condition's key is handed back too, because one arm asserts
// the feed's body and a hand-written key would drift from the fixture.
func psCoordinator(t *testing.T) (b *HerdrBackend, fake string, d *Dispatcher, clock *time.Time, key string) {
	t.Helper()
	b, fake = newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	repo := unpushedRepo(t, b)
	now := time.Now()
	return b, fake, deliveryDispatcher(t, b, &now), &now, "unpushed:" + repo + ":1"
}

// ─── the switch ──────────────────────────────────────────────────────────────

// THE BEAD. A listening waiter means the feed IS the delivery: the line is
// appended, the record advances with it, and NOTHING is typed into the
// pane. The last clause is the one the ADR exists for.
func TestPulseFeedIsTheDeliveryWhileAWaiterHoldsTheLock(t *testing.T) {
	b, fake, d, clock, _ := psCoordinator(t)
	psArmWaiter(t, b, *clock)

	d.pulseOnce(psCfg())

	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Fatalf("a pulse was TYPED while a waiter held the lock — this is the splice class, and D3 is what closes it:\n%s", log)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "→ feed") {
		t.Fatalf("no feed delivery logged:\n%s", out)
	}
	if strings.Contains(out, "→ prompted") {
		t.Errorf("the tick claims a typed delivery:\n%s", out)
	}
	// The line, and the record that says it was said — the two move
	// together or a delta is lost (ranger-base-hkqpq's §1).
	entries := pulseFeedEntries(PulseFeedPath(b.App))
	if len(entries) != 1 {
		t.Fatalf("feed has %d entries, want 1:\n%s", len(entries), strings.Join(entries, "\n"))
	}
	if s := ReadPulseState(PulsePath(b.App)); len(s.DeliveredKeys) == 0 || s.PromptedAt.IsZero() {
		t.Errorf("the record did not advance with the line that carried the delivery: %+v", s)
	}
}

// The arm's whole point said as its own claim: a feed delivery cannot be
// refused. The coordinator is WORKING, which is the state the typed path
// skips on — 16 of ADR 0093's 1323 ticks, beside 986 blocked — and a waiter
// is blocked on a file, so there is nothing to wait for and nothing to
// garble.
func TestPulseFeedDeliversToAWorkingCoordinator(t *testing.T) {
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	unpushedRepo(t, b)
	setAgentStatus(t, fake, id, "working")
	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	psArmWaiter(t, b, clock)

	d.pulseOnce(psCfg())

	if out := dispatcherOut(d); !strings.Contains(out, "→ feed") || strings.Contains(out, "skipped") {
		t.Fatalf("a working coordinator is not a reason to withhold a FEED delivery:\n%s", out)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("something was typed into a working session:\n%s", log)
	}
}

// A free lock means nobody is listening, so the typed pulse runs — with the
// reminder that arms the feed, carrying the cursor the coordinator should
// come back with. The cursor is the feed's newest entry at the time the text
// is composed, which is BEFORE this delivery's own line: arming with it
// replays this delta once rather than starting after it.
func TestPulseTypesAndRemindsWhenNoWaiterIsArmed(t *testing.T) {
	t.Parallel()
	b, fake, d, _, key := psCoordinator(t)

	d.pulseOnce(psCfg())

	out := dispatcherOut(d)
	if !strings.Contains(out, "→ prompted coordinator-work") {
		t.Fatalf("no typed fallback over a free lock:\n%s", out)
	}
	if !strings.Contains(out, "no waiter holds the lock") {
		t.Errorf("the fallback did not say why it was typing:\n%s", out)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "arm the feed: posse pulse wait --after 0") {
		t.Fatalf("the typed pulse carries no reminder, so a coordinator that forgot to re-arm is told nothing:\n%s", log)
	}
	// The command in the text is the one spelling of it (PulseWaitCommand),
	// which is also what the waiter's own `next:` line prints — the
	// coordinator pastes both.
	if want := "arm the feed: " + PulseWaitCommand(0); !strings.Contains(log, want) {
		t.Errorf("the reminder is not %q:\n%s", want, log)
	}
	// AND the condition: a free lock while the coordinator is live is
	// `pulse-unarmed` on the surface (govern.go), so it is in this tick's
	// set, in the text that was typed, and on the line the feed records.
	// This is the row's end-to-end arm — govern_test.go holds its gates.
	wantFeed(t, b, "+pulse-unarmed +"+key+" standing: 2")
	if !strings.Contains(log, "pulse-unarmed") {
		t.Errorf("the typed pulse does not carry the condition it is evidence of:\n%s", log)
	}
}

// The cursor in that reminder is a READING OF THE FEED, over a feed that
// already holds entries. The arm above cannot say so: its fixture's feed is
// empty at the moment the text is composed — the typed arm appends its own
// line after the Prompt — so pulseFeedCursor() answers 0 there and the
// reading and the constant 0 are the same number. Nothing else types a
// reminder over a non-empty feed.
//
// MEASURED 2026-11-14 (ranger-base-wjnfr, escaped from ranger-base-r3qz6),
// two one-line mutants of pulse.go's `PulseWaitCommand(pulseFeedCursor(...))`,
// each over an otherwise unmodified tree:
//
//	PulseWaitCommand(0)                          ok — SURVIVED the arm above
//	PulseWaitCommand(pulseFeedCursor(...) + 1)   FAIL, that arm
//
// So the off-by-one direction was caught and "no cursor at all" was not, and
// what the surviving mutant costs is not prose: over a non-empty feed
// `--after 0` hands the coordinator every kept entry — up to PulseFeedLines
// — on its first wake, instead of this delivery's own line alone.
//
// This arm catches both directions, which is why the want is computed from
// the feed rather than written out: the reading is 1 here, `+1` would make
// it the 2 the typed path's own append is about to produce, and 0 is the
// mutant said by name below.
//
// Two ticks, because a non-empty feed and a free lock is a coordinator that
// armed, was fed, and then let its waiter go — the real shape, not a planted
// file. No t.Parallel and an entry in cmd/testparallel: tick one takes the
// waiter's flock (ranger-base-9l77f).
func TestPulseReminderCursorIsTheFeedsNewestEntryNotZero(t *testing.T) {
	b, fake, d, clock, key := psCoordinator(t)
	cfg := psCfg()

	// Tick one: a listening waiter, so the feed IS the delivery and nothing
	// is typed. This is the only thing this tick is here for — to put a
	// line in the feed the way the feed really gains lines.
	lock := psArmWaiter(t, b, *clock)
	d.pulseOnce(cfg)
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Fatalf("the fixture tick typed instead of feeding, so the reminder below is composed over the wrong feed:\n%s", log)
	}
	wantFeed(t, b, "+"+key+" standing: 1")

	// The fixture's own claim, said before it is leaned on: an empty feed
	// here would make this arm the one it replaces.
	cursor := pulseFeedCursor(PulseFeedPath(b.App))
	if cursor <= 0 {
		t.Fatalf("the feed's max seq is %d, so the cursor and the constant 0 are the same number and this arm pins nothing", cursor)
	}

	// The waiter goes, and the lock going free is what it is to go: release
	// IS death (pulsewait.go). Cleanup releases it again and Release is
	// idempotent on a nil f.
	lock.Release()

	// Tick two: nobody listening, so the typed fallback runs. It is due
	// because `pulse-unarmed` is exactly what a free lock adds to the set
	// (govern.go), so the delta is non-empty without touching the clock.
	d.pulseOnce(cfg)

	if out := dispatcherOut(d); !strings.Contains(out, "→ prompted coordinator-work") {
		t.Fatalf("no typed fallback over the now-free lock:\n%s", out)
	}
	log := calls(t, fake)
	want := "arm the feed: " + PulseWaitCommand(cursor)
	if !strings.Contains(log, want) {
		t.Errorf("the reminder is not %q, so the coordinator is told to arm at the wrong place:\n%s", want, log)
	}
	// Said separately because it is the mutant: `--after 0` is not a cursor
	// that is merely off, it is a replay of every kept entry.
	if strings.Contains(log, PulseWaitCommand(0)) {
		t.Errorf("the reminder arms at 0 over a feed holding %d entries, which replays the lot:\n%s", cursor, log)
	}
	// And the append still happens after the text is composed, which is the
	// other half of why the cursor means "this delivery's own line": the
	// feed gained an entry past the one the reminder names.
	wantFeed(t, b, "+"+key+" standing: 1", "+pulse-unarmed standing: 2")
}

// AND THE CURSOR IS THE FEED'S MAX SEQ AND NOT ITS ENTRY COUNT, which the
// arm above cannot say either (ranger-base-ze7xz, finding 4 of the
// ranger-base-bfbx8 verify). Both arms that type a reminder sit on a feed
// where the two numbers are EQUAL: the free-lock arm's feed is empty when the
// text is composed (count 0, max seq 0) and the arm above drives exactly one
// tick into it (count 1, max seq 1). Nothing else in the tree types a
// reminder over a feed whose count and max seq differ.
//
// MEASURED 2026-11-14, one literal replacement of pulse.go's
// `PulseWaitCommand(pulseFeedCursor(PulseFeedPath(d.App)))` over an otherwise
// unmodified tree:
//
//	PulseWaitCommand(len(pulseFeedEntries(PulseFeedPath(d.App))))
//	    ok 133.129s — the whole arm-3 package green over the feed's entry
//	    COUNT where its MAX SEQ belongs.
//
// THE REAL FEED DIFFERS BY CONSTRUCTION, so this is not a shape only a
// fixture can have: PulseFeedLines is 500 and an append keeps the newest 500
// entries and drops the rest (pulse.go), so a long-lived feed holds 501..1000
// — count 500, max seq 1000. The count reading then pastes `--after 500` where
// the oldest kept entry is 501, and pulse.go's own note says what that gets:
// "a `--after` older than the oldest kept entry gets `gap`". The coordinator's
// first wake is handed every kept entry, up to 500, instead of this delivery's
// line alone — which is the consequence `--after 0` had and the whole of
// ranger-base-wjnfr, arriving by the other reading of the same file.
//
// HOW THE TWO NUMBERS ARE PRISED APART: the feed is SEEDED before tick one
// with one entry at a sequence the count cannot reach, so after the tick it
// holds two entries whose max seq is 8. A seeded line and not seven ticks,
// because seven ticks would put the count at 7 and the max at 7 again. The
// seed is also why wantFeed is not used below — it asserts seq == i+1 over
// the whole feed, which is true of a feed that starts at 1 and is exactly
// what this fixture must not do.
//
// No t.Parallel and an entry in cmd/testparallel: tick one takes the waiter's
// flock, the same ranger-base-9l77f shape as its sibling.
func TestPulseReminderCursorIsTheFeedsMaxSeqNotItsEntryCount(t *testing.T) {
	b, fake, d, clock, key := psCoordinator(t)
	cfg := psCfg()

	// The seed: ONE entry at seq 7, written the way pulseFeedWrite writes
	// one (`<seq> <RFC3339> <body>`), so PulseFeedSeq reads it and the next
	// append is 8. Seq 7 over 1 entry is what makes the count and the max
	// seq different numbers for the rest of this arm.
	const seedSeq = 7
	feed := PulseFeedPath(b.App)
	if err := os.MkdirAll(filepath.Dir(feed), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := fmt.Sprintf("%d %s +seeded standing: 1\n", seedSeq, clock.UTC().Format(time.RFC3339))
	if err := os.WriteFile(feed, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	// Tick one: a listening waiter, so the feed IS the delivery and nothing
	// is typed. Its append is seq 8, because pulseFeedWrite hands out the
	// MAXIMUM parseable sequence plus one.
	lock := psArmWaiter(t, b, *clock)
	d.pulseOnce(cfg)
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Fatalf("the fixture tick typed instead of feeding, so the reminder below is composed over the wrong feed:\n%s", log)
	}

	// The fixture's own claims, said before anything leans on them. Each one
	// is a way this arm could silently become the arm above it.
	entries := pulseFeedEntries(feed)
	if len(entries) != 2 {
		t.Fatalf("the feed holds %d entries, want the seed plus tick one's:\n%s", len(entries), strings.Join(entries, "\n"))
	}
	cursor := pulseFeedCursor(feed)
	if cursor != seedSeq+1 {
		t.Fatalf("the feed's max seq is %d, want %d — the seed was not read as an entry, so the append did not continue its sequence:\n%s", cursor, seedSeq+1, strings.Join(entries, "\n"))
	}
	if cursor == len(entries) {
		t.Fatalf("the feed's max seq and its entry count are both %d, so this arm cannot tell the cursor from the count and pins nothing", cursor)
	}
	// ...and tick one's own entry is really there and really the delta, so
	// the tick that armed this feed is the tick the reminder is about.
	if _, body := feedEntry(t, entries[1]); body != "+"+key+" standing: 1" {
		t.Fatalf("tick one appended %q, not the standing condition's delta", body)
	}

	// The waiter goes, and the lock going free is what it is to go.
	lock.Release()

	// Tick two: nobody listening, so the typed fallback runs and composes
	// the reminder over the two-entry feed.
	d.pulseOnce(cfg)

	if out := dispatcherOut(d); !strings.Contains(out, "→ prompted coordinator-work") {
		t.Fatalf("no typed fallback over the now-free lock:\n%s", out)
	}
	log := calls(t, fake)
	if want := "arm the feed: " + PulseWaitCommand(cursor); !strings.Contains(log, want) {
		t.Errorf("the reminder is not %q, so the coordinator is told to arm at the wrong place:\n%s", want, log)
	}
	// Said separately and by name, because it is the mutant: the ENTRY COUNT
	// is a cursor that is not merely off. On a capped feed it names a
	// sequence older than the oldest kept entry, which pulse.go answers with
	// `gap` and a replay of every kept line.
	if count := len(entries); strings.Contains(log, PulseWaitCommand(count)) {
		t.Errorf("the reminder arms at %d, which is the feed's ENTRY COUNT and not its newest entry (%d):\n%s", count, cursor, log)
	}
	// And the two readings the sibling arm already catches, kept here so one
	// feed shape rules out all three: the constant and the off-by-one.
	for _, bad := range []int{0, cursor + 1} {
		if strings.Contains(log, PulseWaitCommand(bad)) {
			t.Errorf("the reminder arms at %d over a feed whose newest entry is %d:\n%s", bad, cursor, log)
		}
	}
}

// A waiter that holds the lock past its own maximum life is WEDGED, not
// listening: the waiter self-exits after one pulse_renag with nothing to
// say (pulseWaitLife), so a holder older than that plus one tick cannot be
// a healthy wait. Without this arm a wedged process silences the typed path
// for as long as it sits there, and from outside it looks exactly like a
// quiet shop (ADR 0093, "what breaks if this is wrong").
func TestPulseTypesWhenTheWaiterIsWedged(t *testing.T) {
	b, fake, d, clock, _ := psCoordinator(t)
	cfg := psCfg()
	psArmWaiter(t, b, clock.Add(-(pulseWedgeBound(cfg) + time.Minute)))

	d.pulseOnce(cfg)

	out := dispatcherOut(d)
	if !strings.Contains(out, "wedged") {
		t.Fatalf("a waiter past its maximum life was not read as wedged:\n%s", out)
	}
	if !strings.Contains(out, "→ prompted coordinator-work") {
		t.Fatalf("the typed fallback did not run behind a wedged waiter:\n%s", out)
	}
	if log := calls(t, fake); !strings.Contains(log, "agent prompt") {
		t.Errorf("nothing was typed, so the shop check went nowhere:\n%s", log)
	}
}

// The bound is one renag plus one tick, and the extra tick is not slack for
// its own sake: a healthy waiter armed an instant before a tick is exactly
// `renag` old at the tick that would judge it, and judging it wedged there
// is judging a waiter that is at that moment exiting on purpose.
func TestPulseWaiterAtExactlyTheRenagIsStillListening(t *testing.T) {
	b, fake, d, clock, _ := psCoordinator(t)
	cfg := psCfg()
	psArmWaiter(t, b, clock.Add(-cfg.Renag))

	d.pulseOnce(cfg)

	if out := dispatcherOut(d); !strings.Contains(out, "→ feed") {
		t.Fatalf("a waiter exactly one renag old was read as wedged:\n%s", out)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("the pulse typed over a listening waiter:\n%s", log)
	}
}

// A holder whose body could not be read is NOT judged wedged. The lock file's
// body is identity and identity decides nothing (pulsewait.go): the kernel
// says a process holds this, and absence of a stamp is not evidence that the
// process is stuck. The cost is named at PulseListener — a wedged waiter whose
// stamp never landed — and it is the direction that keeps the keyboard quiet.
func TestPulseConcedesToAHolderItCannotAge(t *testing.T) {
	b, fake, d, clock, _ := psCoordinator(t)
	psArmWaiter(t, b, *clock)
	// The holder's stamp, gone — a body that was never written, or written
	// by a binary that did not stamp one. The LOCK is untouched.
	if err := os.Truncate(PulseWaitLockPath(b.App), 0); err != nil {
		t.Fatal(err)
	}
	if w, ok := ReadPulseWaiter(PulseWaitLockPath(b.App)); ok {
		t.Fatalf("the fixture still has a readable holder: %+v", w)
	}

	d.pulseOnce(psCfg())

	if out := dispatcherOut(d); !strings.Contains(out, "→ feed") {
		t.Fatalf("an unreadable body was read as a wedge, which types over a listener:\n%s", out)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("the pulse typed over a waiter it merely could not name:\n%s", log)
	}
}

// A PROBE NOBODY COULD TAKE IS NOT "NOBODY IS LISTENING", and the watch
// concedes: the delivery goes to the feed and nothing is typed. The cost of
// conceding is a delta in a feed nobody may be reading and one wait for the
// next; the cost of the other direction is typing into the pane of a session
// that IS listening, which is the measured splice. The concession is said out
// loud on the error stream, so it is never something a reader has to infer.
//
// `posse status` takes the same probe the other way — PARTIAL, not a row —
// and that half is in TestQAPulseUnarmedIsTheRowWhileNobodyIsListening.
func TestPulseConcedesAnUnaskableProbeToTheFeed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0000 file, so there is no unaskable probe to stage")
	}
	b, fake, d, _, _ := psCoordinator(t)
	errs := dispatcherErr(t, d)
	if err := os.MkdirAll(b.App.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PulseWaitLockPath(b.App), []byte("pid: 1\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := PulseWaiterArmed(b.App); err == nil {
		t.Skip("this filesystem answered the probe anyway — nothing to assert")
	}

	d.pulseOnce(psCfg())

	if out := dispatcherOut(d); !strings.Contains(out, "→ feed") {
		t.Fatalf("an unaskable probe was read as `nobody is listening`, so the pulse typed:\n%s", out)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("something was typed over a probe that proves nothing:\n%s", log)
	}
	if got := errs.String(); !strings.Contains(got, "waiter lock") {
		t.Errorf("the concession is silent, so nobody can tell it from a delivery to a real waiter:\n%s", got)
	}
}

// On the feed arm the append IS the delivery, so an append that fails said
// NOTHING: the record must not advance, and the next tick retries the same
// delta. The typed arm is the other way round (the prompt has already landed
// by then) and pulse_delivery_test.go pins that.
func TestPulseFeedArmDoesNotAdvanceTheRecordWhenTheAppendFails(t *testing.T) {
	b, fake, d, clock, _ := psCoordinator(t)
	errs := dispatcherErr(t, d)
	psArmWaiter(t, b, *clock)
	// The writer renames a temp file into place; a DIRECTORY in its way
	// fails the write without touching the feed itself.
	if err := os.MkdirAll(PulseFeedPath(b.App)+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}

	d.pulseOnce(psCfg())

	if got := errs.String(); !strings.Contains(got, "cannot append") {
		t.Fatalf("a failed append left no diagnostic:\n%s", got)
	}
	out := dispatcherOut(d)
	if strings.Contains(out, "→ feed") {
		t.Errorf("the tick claimed a delivery it could not write:\n%s", out)
	}
	if s := ReadPulseState(PulsePath(b.App)); len(s.DeliveredKeys) != 0 || !s.PromptedAt.IsZero() {
		t.Errorf("the record advanced over a line that never landed, so the delta is lost: %+v", s)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("a failed feed append fell back to typing, which is a second delivery of one delta:\n%s", log)
	}
}

// ─── the exit hatch ──────────────────────────────────────────────────────────

// `pulse_feed: off` is ADR 0027's path unchanged: the typed pulse goes out
// even while a waiter holds the lock, there is no feed file, and the text
// carries no reminder to arm a channel the operator switched off.
func TestPulseFeedOffKeepsTheTypedPathUnchanged(t *testing.T) {
	b, fake, d, clock, _ := psCoordinator(t)
	psArmWaiter(t, b, *clock)
	cfg := psCfg()
	cfg.Feed = false

	d.pulseOnce(cfg)

	out := dispatcherOut(d)
	if !strings.Contains(out, "→ prompted coordinator-work") {
		t.Fatalf("the hatch did not leave the typed path running:\n%s", out)
	}
	if strings.Contains(out, "→ feed") || strings.Contains(out, "pulse: feed") {
		t.Errorf("the hatch is set and the feed still took the delivery:\n%s", out)
	}
	if entries := pulseFeedEntries(PulseFeedPath(b.App)); len(entries) != 0 {
		t.Errorf("the hatch is set and a feed was written: %q", entries)
	}
	if log := calls(t, fake); strings.Contains(log, "arm the feed") {
		t.Errorf("the typed pulse reminds the coordinator to arm a feed that is off:\n%s", log)
	}
}

// The hatch is read off the config, and a value that is not one of the
// spellings for `off` leaves the channel ON. Both directions cost something
// and this is the cheaper one: a mistyped hatch leaves the new channel
// running, where "anything but `on` is off" would let one character take the
// feed, the waiter and the condition out silently.
func TestPulseFeedConfigSpellings(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		value string
		off   bool
	}{
		{"off", true}, {"never", true}, {"0", true}, {"false", true},
		{" off ", true},
		{"", false}, {"on", false}, {"true", false}, {"ofc", false}, {"OFF", false},
	} {
		if got := pulseFeedOff(c.value); got != c.off {
			t.Errorf("pulseFeedOff(%q) = %v, want %v", c.value, got, c.off)
		}
	}
}

// And the key reaches PulseConfig, which is what the watch loop and `posse
// status` both read it through.
func TestLoadPulseConfigReadsTheFeedKey(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	if err := os.WriteFile(b.App.ConfigPath, []byte("pulse_interval: 2m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadPulseConfig(b.App, io.Discard)
	if err != nil || !cfg.Feed {
		t.Fatalf("the feed is ON by default: cfg=%+v err=%v", cfg, err)
	}
	if err := os.WriteFile(b.App.ConfigPath, []byte("pulse_interval: 2m\npulse_feed: off\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadPulseConfig(b.App, io.Discard)
	if err != nil || cfg.Feed {
		t.Fatalf("pulse_feed: off did not reach the config: cfg=%+v err=%v", cfg, err)
	}
	if !cfg.Armed {
		t.Error("the hatch disarmed the pulse — it switches a channel, not the shop check")
	}
}

// The wedge bound, at the unit: one renag plus one tick, and zero — which
// ages nothing — for a config that cannot say.
func TestPulseWedgeBound(t *testing.T) {
	t.Parallel()
	if got := pulseWedgeBound(psCfg()); got != 32*time.Minute {
		t.Errorf("pulseWedgeBound = %s, want 32m (renag 30m + one 2m tick)", got)
	}
	if got := pulseWedgeBound(PulseConfig{}); got != 0 {
		t.Errorf("a zero config bounds nothing, got %s — a holder judged against a default this instance does not run", got)
	}
}

// ─── the condition (ADR 0093 D3) ─────────────────────────────────────────────

// `pulse-unarmed` joins the condition set while the lock is free and a
// coordinator session is live, so `posse status` shows what the watch log
// says. Every gate is an arm, because each one is a way for this row to be
// wrong in a direction somebody pays for: a row nobody can act on (no live
// session, or the hatch set), a row that fires over a listening waiter (the
// pulse then looks broken while it is working), and a row that goes quiet
// over a probe nobody could take (reported as PARTIAL, which is this
// surface's whole discipline — unknown is never an all-clear).
func TestQAPulseUnarmedIsTheRowWhileNobodyIsListening(t *testing.T) {
	cfg := psCfg()
	unarmedIn := func(t *testing.T) (*HerdrBackend, GovInputs) {
		t.Helper()
		b, fake := newTestBackend(t)
		personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
		in := pulseIn(t, b, nil, "coordinator")
		in.PulseFeed, in.PulseWedge = true, pulseWedgeBound(cfg)
		return b, in
	}

	t.Run("a free lock while the coordinator is live", func(t *testing.T) {
		b, in := unarmedIn(t)
		set := shopSet(t, in)
		var row *GovCondition
		for i := range set {
			if set[i].Key == "pulse-unarmed" {
				row = &set[i]
			}
		}
		if row == nil {
			t.Fatalf("no pulse-unarmed row: %v", set.Keys())
		}
		if row.Class != GovLane {
			t.Errorf("class = %s, want LANE — the shop is not stopped and the typed fallback is live", row.Class)
		}
		if row.Row() != "—" {
			t.Errorf("Row() = %q, want the carry-over's dash: a row name promises ADR 0029's table describes it", row.Row())
		}
		// The detail has to carry the command, because the row's whole
		// content is that a human has to arm something.
		if !strings.Contains(row.Detail, PulseWaitCommand(pulseFeedCursor(PulseFeedPath(b.App)))) {
			t.Errorf("the detail does not name the command that clears it: %q", row.Detail)
		}
	})

	t.Run("a listening waiter clears it", func(t *testing.T) {
		b, in := unarmedIn(t)
		psArmWaiter(t, b, time.Now())
		if keys := shopKeys(t, in); containsStr(keys, "pulse-unarmed") {
			t.Errorf("the row fired over an armed waiter, so a working pulse reads as a broken one: %v", keys)
		}
	})

	t.Run("a wedged waiter is still unarmed", func(t *testing.T) {
		b, in := unarmedIn(t)
		psArmWaiter(t, b, time.Now().Add(-(pulseWedgeBound(cfg) + time.Minute)))
		if keys := shopKeys(t, in); !containsStr(keys, "pulse-unarmed") {
			t.Errorf("a wedged holder reads as a listener, which is the silence ADR 0093 names: %v", keys)
		}
	})

	t.Run("the hatch takes the row with the channel", func(t *testing.T) {
		_, in := unarmedIn(t)
		in.PulseFeed = false
		if keys := shopKeys(t, in); containsStr(keys, "pulse-unarmed") {
			t.Errorf("`pulse_feed: off` and the surface still tells the operator to arm it: %v", keys)
		}
	})

	t.Run("a disarmed pulse reports nothing", func(t *testing.T) {
		_, in := unarmedIn(t)
		in.Pulsing = false
		if keys := shopKeys(t, in); containsStr(keys, "pulse-unarmed") {
			t.Errorf("an unarmed pulse delivers through neither channel and is missing nothing: %v", keys)
		}
	})

	t.Run("no live coordinator is the other row", func(t *testing.T) {
		b, _ := newTestBackend(t)
		in := pulseIn(t, b, nil, "coordinator")
		in.PulseFeed, in.PulseWedge = true, pulseWedgeBound(cfg)
		keys := shopKeys(t, in)
		if containsStr(keys, "pulse-unarmed") {
			t.Errorf("two rows for one stopped delivery — `no-live:` is the bigger fact: %v", keys)
		}
		if !containsStr(keys, "no-live:coordinator") {
			t.Errorf("the carry-over that IS true went missing: %v", keys)
		}
	})

	t.Run("a probe nobody can take is PARTIAL and not a row", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a 0000 file, so there is no unaskable probe to stage")
		}
		b, in := unarmedIn(t)
		path := PulseWaitLockPath(b.App)
		if err := os.MkdirAll(b.App.StateDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("pid: 1\n"), 0o000); err != nil {
			t.Fatal(err)
		}
		if _, err := PulseWaiterArmed(b.App); err == nil {
			t.Skip("this filesystem answered the probe anyway — nothing to assert")
		}
		set, failed := ShopCheck(in)
		if containsStr(set.Keys(), "pulse-unarmed") {
			t.Errorf("ignorance was rendered as a condition: %v", set.Keys())
		}
		var said bool
		for _, err := range failed {
			if strings.Contains(err.Error(), "pulse-unarmed unknown") {
				said = true
			}
		}
		if !said {
			t.Errorf("a probe that could not be taken reads as an all-clear: %v", failed)
		}
	})
}
