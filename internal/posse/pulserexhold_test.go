//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0093 D4, ranger-base-r3qz6: the typed fallback refuses to type over
// text the operator typed, on rex only.
//
// THE FAILURE IT ANSWERS, measured. The pulse types `Pulse check: …` and
// then Enter into the coordinator's pane; the operator types into that same
// one-byte stream from a remote client. Three coordinator turns on
// 2026-11-13 came out spliced — `I think thats accurPulse check: …` — one
// message made of two, neither of them what anybody wrote.
//
// WHY IT IS NOT ranger-base-htafy's GATE BACK. That gate read herdr's
// composer region, which on 2026-09-04 carried a line the operator had
// already SENT: ~586 consecutive skips against a box `posse peek` showed
// empty, the pulse arm off for ~10 of that day's hours, 108 commits stacked
// behind it (ranger-base-wr624). A screen matcher over a region that can
// hold ghost text cannot clear itself. This reading can: it asks the
// RUNTIME'S OWN submit log first (ranger-base-qzujz, ADR 0076 Amendment 6)
// and then the dim check, and only what survives both is a hold. So the
// arms here are three — held, and the two retirements that must still
// deliver — because a gate that holds on all three is wr624 again.
//
// ARM 1, where the capture fixtures and the runtime-session store live
// (capturedcomposer_qa_test.go, runtimesession_qa_test.go). The switch's own
// arms are arm 3's (pulseswitch_test.go); nothing here takes the waiter
// lock, because on this path there is no waiter — a hold is only ever
// reached when the lock is free.

import (
	"os"
	"strings"
	"testing"
	"time"
)

// pulseRexCfg is the typed-fallback config: armed, the coordinator, the feed
// on and no waiter anywhere, which is exactly when D4 is reached.
func pulseRexCfg() PulseConfig {
	return PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute,
		Interval: 2 * time.Minute, Feed: true}
}

// pulseRexSeat is a live idle coordinator session on a `multiplexer: rex`
// instance, with one standing condition so the tick is due.
//
// The seat is rexTestSeat by construction and not by seeding: the fake's
// first `rex new` hands out session:fake-0001 / block:fake-0001, which is
// what rexTestSeat names, so the session posse creates here IS the block the
// other fixtures in this package seed. `owner` is the pid the block's
// program_status record names — the test's own where an arm needs the submit
// log joined on it (runtimesession.go's fence), and the ordinary measured 20
// otherwise.
func pulseRexSeat(t *testing.T, owner int) (*HerdrBackend, *Dispatcher, *time.Time) {
	t.Helper()
	b, _ := newTestBackend(t)
	m, _ := newTestRexMux(t)
	b.Slot = m
	writePersona(t, b.App, "coordinator", "code")
	// SEEDED BEFORE THE CREATE, because the create reads it: on Rex a
	// launch waits up to the runtime's `startup_wait:` for the block's
	// first program_status record (confirmReports, ADR 0076 §3), so a
	// fixture that seeds the record afterwards pays 45s of real time and
	// then warns that the seat reports nothing.
	rexSeedProcess(t, rexTestBlock, claudeProcTable(owner))
	rexSeedStatus(t, rexTestBlock, []RexStatusRecord{{State: "idle", App: "claude-code",
		Owner: &RexOwner{PID: owner, Name: claudeRuntimeProcName}}})
	mustCreate(t, b, NewSessionOpts{Name: "coordinator-work", Agent: "coordinator"})
	unpushedRepo(t, b)

	// The seat the pulse will resolve has to be the block these fixtures
	// seed, or every arm below would be an arm about an unseeded screen.
	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	name, seat, status, found := pulseTarget(sessions, "coordinator")
	if !found || seat.Pane != rexTestBlock || !pulseTakesPrompt(status) {
		t.Fatalf("fixture: pulseTarget = %q %+v %q found=%v, want the seeded idle block %s",
			name, seat, status, found, rexTestBlock)
	}
	clock := time.Now()
	return b, deliveryDispatcher(t, b, &clock), &clock
}

// rexCallsSince is the rex calls made after the first n — the only honest
// way to ask "did this TICK type anything", because the session's own
// creation types the runtime's command line into the block (RexMux.Start is
// `rex send` plus Enter), so the launch is in this log too.
func rexCallsSince(t *testing.T, n int) []string {
	t.Helper()
	all := rexCalls(t)
	if len(all) < n {
		t.Fatalf("the rex call log shrank from %d to %d entries", n, len(all))
	}
	return all[n:]
}

// pulseRexBox seeds the block's screen with `composer` in the prompt box.
func pulseRexBox(t *testing.T, line string) {
	t.Helper()
	rexSeedCapture(t, rexTestBlock, claudeComposerScreen([]string{line}, idleFooter))
}

// THE BEAD. A box holding text nobody can account for is a prompt sitting
// unsent, and the pulse does not type after it: the tick is SKIPPED, the
// delivery record is left where it was, and the next tick retries the same
// delta.
func TestPulseHoldsWhenTheRexBoxHoldsOperatorText(t *testing.T) {
	t.Parallel()
	const typed = "actually, hold off on the migration until"
	b, d, _ := pulseRexSeat(t, 20)
	pulseRexBox(t, mark+typed)
	launched := len(rexCalls(t))

	d.pulseOnce(pulseRexCfg())

	out := dispatcherOut(d)
	if !strings.Contains(out, "pulse: held — coordinator-work box holds operator text") {
		t.Fatalf("the hold did not happen, or did not say so — the log line is what explains a garbled turn after the fact:\n%s", out)
	}
	if !strings.Contains(out, typed[:20]) {
		t.Errorf("the hold names no text, so it is not evidence of a reading:\n%s", out)
	}
	// Nothing was typed. This is the whole claim: the splice is two writers
	// on one stream, and the second one just declined.
	typedCalls := rexCallsSince(t, launched)
	for _, verb := range []string{" send ", " send-key "} {
		if containsSubstr(typedCalls, verb) {
			t.Errorf("the pulse typed into a box holding operator text (rex%s):\n%s", verb, strings.Join(typedCalls, "\n"))
		}
	}
	// A skip and not a delivery: unadvanced record, and no feed line either
	// — a tick that said nothing must not record that it did.
	if s := ReadPulseState(PulsePath(b.App)); len(s.DeliveredKeys) != 0 || !s.PromptedAt.IsZero() {
		t.Errorf("the record advanced over a delivery that never went out: %+v", s)
	}
	if entries := pulseFeedEntries(PulseFeedPath(b.App)); len(entries) != 0 {
		t.Errorf("a held tick wrote to the feed: %q", entries)
	}
}

// The hatch does NOT take this with it. `pulse_feed: off` turns off the
// channel — feed, probe, reminder, condition — and D4 is a guard on the
// KEYBOARD: an operator reaching for the hatch is an operator whose pulse is
// typing, which is when the hold matters most. Said as its own arm because
// ADR 0093's word for the hatch is "unchanged" and this is a deliberate
// reading of it.
func TestPulseHoldsOverTheRexBoxWithTheFeedHatchSet(t *testing.T) {
	t.Parallel()
	b, d, _ := pulseRexSeat(t, 20)
	pulseRexBox(t, mark+"wait — I'm mid-sentence")
	cfg := pulseRexCfg()
	cfg.Feed = false

	d.pulseOnce(cfg)

	if out := dispatcherOut(d); !strings.Contains(out, "box holds operator text") {
		t.Fatalf("the hatch disarmed the composer hold, so the exit hatch is a splice regression:\n%s", out)
	}
	if s := ReadPulseState(PulsePath(b.App)); !s.PromptedAt.IsZero() {
		t.Errorf("the record advanced: %+v", s)
	}
}

// RETIREMENT 1 — the box is echoing the line this pane's own runtime says it
// already submitted. That is not a hold, and this is the arm wr624's gate
// could not take: the store is asked FIRST and it is the authority for "was
// this submitted".
func TestPulseDeliversOverARexBoxEchoingItsLastSubmit(t *testing.T) {
	t.Parallel()
	start := ownProcStart(t)
	pid := os.Getpid()
	const sent = "work the pulse bead and keep the suite green"

	b, d, _ := pulseRexSeat(t, pid)
	history := armClaudeSessionRow(t, b, pid, map[string]any{
		"pid": pid, "sessionId": qzujzSession, "cwd": "/w", "procStart": start,
	})
	writeSubmitRows(t, history, qzujzSession, "how are we doing?", sent)
	pulseRexBox(t, mark+sent)
	launched := len(rexCalls(t))

	d.pulseOnce(pulseRexCfg())

	out := dispatcherOut(d)
	if !strings.Contains(out, "echoes its own last submit") {
		t.Errorf("the echo reading left no trace, so a later skip cannot be told from this:\n%s", out)
	}
	if !strings.Contains(out, "→ prompted coordinator-work") {
		t.Fatalf("the pulse skipped on a box echoing a line the runtime says was SENT — this is the ~586 skips of ranger-base-wr624:\n%s", out)
	}
	if s := ReadPulseState(PulsePath(b.App)); len(s.DeliveredKeys) == 0 {
		t.Errorf("a delivered pulse left no record: %+v", s)
	}
	// And the keystrokes themselves, which is what `→ prompted` claims.
	if calls := rexCallsSince(t, launched); !containsSubstr(calls, " send ") || !containsSubstr(calls, " send-key ") {
		t.Errorf("nothing was typed into the pane:\n%s", strings.Join(calls, "\n"))
	}
}

// RETIREMENT 2 — the box is drawing claude's own suggested next prompt,
// faint. Nobody typed it, no store claims it, and the only thing that tells
// it from typed input is that it is dim (ranger-base-6o7wm).
func TestPulseDeliversOverARexGhostBox(t *testing.T) {
	t.Parallel()
	const suggestion = `Try "create a util logging.py that..."`
	_, d, _ := pulseRexSeat(t, 20)
	pulseRexBox(t, mark+suggestion)
	// The same screen as the renderer spells it: the box, drawn dim. The
	// plain capture above is what the composer is READ off; this is what the
	// ghost check decides on.
	rexSeedCaptureAs(t, rexTestBlock, "vt", claudeComposerScreen(
		[]string{mark + "\x1b[0m\x1b[2m" + suggestion + "\x1b[0m"}, idleFooter))

	d.pulseOnce(pulseRexCfg())

	out := dispatcherOut(d)
	if !strings.Contains(out, "claude's own suggestion") {
		t.Errorf("the ghost reading left no trace:\n%s", out)
	}
	if !strings.Contains(out, "→ prompted coordinator-work") {
		t.Fatalf("a suggestion nobody typed held the shop check — and nothing can clear it, which is the hold that cannot go false:\n%s", out)
	}
}

// THE SCOPE, at the door: herdr stays report-only (ADR 0093 D4, and
// ranger-base-wr624's measurement is why). `false` is "this reading was
// never taken", which is the one thing an empty hold cannot say — and the
// pulse has to be able to tell those apart or it would read herdr's
// phantom-prone composer as an empty box and call that evidence.
func TestQARexComposerHoldIsRexOnly(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	if _, ok := b.rexComposerHold(Seat{Session: "ws", Pane: "ws:p1"}); ok {
		t.Fatal("the door answered on a herdr backend — herdr's composer reading is the one that muted the pulse for a day (ranger-base-wr624) and D4 leaves it report-only")
	}

	m, _ := newTestRexMux(t)
	b.Slot = m
	rexSeedTestSeat(t)
	rexSeedProcess(t, rexTestBlock, claudeProcTable(20))
	rexSeedStatus(t, rexTestBlock, []RexStatusRecord{{State: "idle", App: "claude-code",
		Owner: &RexOwner{PID: 20, Name: claudeRuntimeProcName}}})
	pulseRexBox(t, mark+"a prompt sitting unsent")
	hold, ok := b.rexComposerHold(rexTestSeat())
	if !ok {
		t.Fatal("the door refused on a rex backend")
	}
	if hold.Typed != "a prompt sitting unsent" {
		t.Errorf("Typed = %q, want the box's text", hold.Typed)
	}
	// Narrower than capturedHold on purpose: the pulse's question is about
	// the keyboard, and whether this seat has a background shell running is
	// both answered upstream and none of this delivery's business.
	if hold.Work != "" {
		t.Errorf("Work = %q — the door reads the composer only", hold.Work)
	}
	// A screen rex will not answer is "available, read nothing", which is
	// this family's standing direction and here means the pulse types.
	rexSeedFail(t, "capture", "no such block")
	if hold, ok := b.rexComposerHold(rexTestSeat()); !ok || hold.Typed != "" {
		t.Errorf("an unreadable screen = %+v, ok=%v; want the empty hold with the door still open", hold, ok)
	}
}
