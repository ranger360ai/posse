//go:build posse_arm3

package posse

// QA pins for ranger-base-x2grk — a settle refusal names the screen.
//
// THE INCIDENT (ranger-base-yk5qb). A fresh account's first dispatched claude
// seat sat on `Try the new fullscreen renderer? 1. Yes / 2. Not now` and
// dispatch printed `never settled idle (status "blocked")`, whole. The
// operator hand-launched and peeked to learn which screen it was — on a pane
// herdr had already matched to a rule, MEASURED under ranger-base-72oz4 as
// `live_blocked_form`, region `after_last_horizontal_rule`, priority 980.
//
// WHY A PIN AND NOT JUST THE FIX. Every arm here is about a SILENCE. A
// diagnostic that goes missing fails nothing: the launch still refuses, the
// pass still prints a line, the suite still passes, and the cost lands on
// whoever reads the line next and cannot act on it. Three ways this rots:
//
//  1. the explain on awaitSettled's refusing arm is dropped as redundant —
//     `agent wait` already said `blocked`, and the detection it hands back
//     looks complete — and the line silently goes back to the state string.
//     Arm 1, which asserts the three fields by name.
//  2. the record stops being written, and `scripts/readings-census.py`'s
//     denominator silently loses the recognized half of the gate's refusals
//     again. Arm 2. A census over a log cannot tell a decision nobody made
//     from a decision nobody recorded.
//  3. the line starts naming a rule it did not read — the one outcome worse
//     than the bare line, because it is wrong with more confidence. Arm 3
//     takes the rule away and asks what the line says then.
//
// They live in arm 3 beside startupscreen_test.go, whose
// TestDispatchAnswersNoBlockedScreen is the pin that NOTHING HERE PRESSES A
// KEY at any of these screens (rangerhq-4mzt, ADR 0013 §2). This is a
// diagnostic bead only, and the two files are read together.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bsIncident arms the fake herdr with ranger-base-72oz4's measured reading of
// the claude fullscreen-upsell pane, and runs one pass over it.
//
// THE REPO IS A REAL GIT REPO and that is load-bearing, for fiSeat's reason
// (falseidle_qa_test.go): LogReading resolves the log through the session
// meta's Dir and a session with no tree gets no log at all, so a pass over a
// plain temp dir would write nothing and arm 2 would be an assertion about an
// absent file. splashRepo next door does not make one, which is why this rig
// is its own rather than that one with two levers added.
func bsIncident(t *testing.T, extra func(fake string)) (*Dispatcher, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	write(t, filepath.Join(fake, "agents.json"),
		`[{"agent":"claude","agent_status":"blocked","pane_id":"w1:p1","workspace_id":"w1"}]`)
	// The measured reading: herdr 0.9.1, bundled claude manifest
	// 2026.09.11.1, over the fullscreen-upsell pane (ranger-base-72oz4).
	// Named like the real thing rather than left at the fake's defaults,
	// because a pin asserting `fake_blocked` and `fake_region` would be
	// asserting the fixture.
	write(t, filepath.Join(fake, "wait-status"), "blocked")
	write(t, filepath.Join(fake, "explain-rule"), "live_blocked_form")
	write(t, filepath.Join(fake, "explain-region"), "after_last_horizontal_rule")
	write(t, filepath.Join(fake, "explain-priority"), "980")
	if extra != nil {
		extra(fake)
	}
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a blocked pane dispatched %d beads, want 0:\n%s", n, dispatcherOut(d))
	}
	return d, dispatcherOut(d)
}

// bsReadings is every reading the pass wrote under the session it tried to
// dispatch into. An absent log is no readings, which is what arm 2 would see
// if the record stopped being written.
//
// Not fiReadings: that helper is arm 2's (falseidle_qa_test.go) and these
// pins are arm 3's, so the two cannot share one. Its shape is deliberately
// the same, down to the fatal on a session with no meta — a pass that left no
// tree logged nothing, and reading that as "no records" would make arm 2 pass
// for the wrong reason.
func bsReadings(t *testing.T, d *Dispatcher) []Reading {
	t.Helper()
	sessions, err := d.HB.Sessions()
	if err != nil || len(sessions) != 1 {
		t.Fatalf("the pass left %d sessions (%v), want the one it refused to prompt", len(sessions), err)
	}
	m, ok := d.HB.readMeta(sessions[0].Name)
	if !ok || m.Dir == "" {
		t.Fatalf("no meta with a dir for %s — nothing could have been logged, so this arm measures nothing", sessions[0].Name)
	}
	log, err := d.App.ReadingsLogPath(m.Dir)
	if err != nil {
		t.Fatalf("ReadingsLogPath(%s): %v", m.Dir, err)
	}
	if _, err := os.Stat(log); os.IsNotExist(err) {
		return nil
	}
	rs, err := ReadReadings(log)
	if err != nil {
		t.Fatalf("ReadReadings(%s): %v", log, err)
	}
	return rs
}

// Arm 1 — the line names the rule, the region and the priority, in herdr's
// own spelling.
//
// THE THREE FIELDS BY NAME, not "the line is longer than it was". herdr prints
// `rule: live_blocked_form (region=after_last_horizontal_rule priority=980)`,
// and the whole value of matching it is that an operator can carry the posse
// line to `herdr agent explain` and to `etc/herdr/agent-detection/*.toml`
// without translating between two vocabularies — so the keys are asserted as
// herdr writes them.
//
// It also asserts the OLD line is gone. `status "blocked")` — the closing
// paren hard against the status — is exactly what ranger-base-yk5qb read, and
// a fix that appended its clause somewhere else while leaving that shape
// intact would satisfy every other assertion here.
func TestQABlockedSettleRefusalNamesTheRuleAndRegion(t *testing.T) {
	t.Parallel()
	_, out := bsIncident(t, nil)
	for _, want := range []string{
		`never settled idle`,
		`status "blocked"`,
		`rule "live_blocked_form"`,
		`region=after_last_horizontal_rule`,
		`priority=980`,
		`posse peek`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal does not carry %s — that is the hand-launch and the peek ranger-base-yk5qb paid for, back:\n%s", want, out)
		}
	}
	if strings.Contains(out, `status "blocked")`) {
		t.Errorf("the status clause still closes on the state alone, which IS the line the incident could not act on:\n%s", out)
	}
}

// Arm 2 — the refusal is recorded (ADR 0066 D1), as a D2 reading whose rule is
// herdr's own.
//
// D2 AND NOT D3: this screen is one herdr RECOGNIZED, so the unknown-screen
// decision is not what took the reading. It is logPromptHandBack's taxonomy
// one keystroke earlier — herdr read a blocker on the screen and posse
// refused — and a record filed under D3 would put a recognized blocker in the
// census row that counts the screens posse cannot name.
//
// REFUSAL AND NOT HAND-BACK: awaitAgent runs before `d.claim`, so there is no
// claim to give back. The bead that filed this called it a hand-back; the code
// says otherwise and this is where that is written down.
//
// NO PANE CAPTURE, which is the assertion that keeps this inside ADR 0066 D2.
// A capture is what the known-screen heading reader needs, and a `looks_like`
// on this record would be a THIRD consumer of the D3 report where D2 names
// exactly two. If that is ever wanted it is an architecture ruling first
// (filed as ranger-base-jnmx6), and this arm is what it has to come back and
// change.
func TestQABlockedSettleRefusalIsRecorded(t *testing.T) {
	t.Parallel()
	d, _ := bsIncident(t, nil)
	rs := bsReadings(t, d)
	var rec []Reading
	for _, r := range rs {
		if r.Consequence == ConsequenceRefusal {
			rec = append(rec, r)
		}
	}
	if len(rec) != 1 {
		t.Fatalf("the pass wrote %d refusal records over %d readings, want the one blocked settle — an unrecorded refusal is a census denominator short by every recognized blocker (ADR 0066 D1)", len(rec), len(rs))
	}
	r := rec[0]
	if r.Decision != DecisionDialog {
		t.Errorf("the record is %s, want %s: herdr read a blocker on the screen and posse refused, which is logPromptHandBack's own D2 line", r.Decision, DecisionDialog)
	}
	if r.Rule != "live_blocked_form" {
		t.Errorf("the record's rule is %q, want herdr's matched rule — that is the field a census groups by and the one a reader takes to the manifest", r.Rule)
	}
	if r.Herdr.State != "blocked" {
		t.Errorf("the record's herdr state is %q, want %q", r.Herdr.State, "blocked")
	}
	if !r.Herdr.Seen {
		t.Error("the record says herdr saw nothing, over a reading that matched a rule")
	}
	if len(r.LooksLike) != 0 {
		t.Errorf("the record carries looks_like %v — ADR 0066 D2 names exactly two consumers of that report and a blocked hand-back is neither (ranger-base-jnmx6 is the ruling that would change this)", r.LooksLike)
	}
	if _, ok := r.ReadingRegionOf(PaneCaptureRegion); ok {
		t.Error("the record carries a pane capture, which is the D3 reader's input — a D1/D2 record carries herdr's own regions and nothing else (logPromptHandBack)")
	}
}

// Arm 3 — with no rule to be had, the line says so and invents nothing.
//
// The lever is a herdr that will not answer `explain <pane>` at all, which is
// the one way the second read comes back empty on this arm: `agent wait` has
// already settled, so there is no screen for it to disagree with except a
// herdr that went away between the two calls. The line must then read as a
// line posse COULD NOT name, not as one it did not try to — and the record's
// rule must be the codepoint that refused rather than a blank field a census
// cannot group.
func TestQABlockedSettleNamesNoRuleItDidNotRead(t *testing.T) {
	t.Parallel()
	d, out := bsIncident(t, func(fake string) {
		write(t, filepath.Join(fake, "explain-error"), "agent_explain_unavailable|fake herdr: gone")
	})
	if !strings.Contains(out, `status "blocked"`) || !strings.Contains(out, "no rule") {
		t.Errorf("a refusal posse could not name must say so beside the status:\n%s", out)
	}
	if strings.Contains(out, "live_blocked_form") || strings.Contains(out, "region=") {
		t.Errorf("the line names a rule no read produced, which is worse than the bare line it replaced:\n%s", out)
	}
	rs := bsReadings(t, d)
	for _, r := range rs {
		if r.Consequence != ConsequenceRefusal {
			continue
		}
		if r.Rule != RuleSettleGate {
			t.Errorf("the record's rule is %q, want %q — a census row under the gate's own name is the finding \"posse refused and no rule was in hand\", and an empty field is a row it drops", r.Rule, RuleSettleGate)
		}
		return
	}
	t.Errorf("no refusal record over %d readings: a refusal posse could not explain is still a refusal ADR 0066 D1 records", len(rs))
}
