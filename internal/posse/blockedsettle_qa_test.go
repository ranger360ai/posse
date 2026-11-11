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
//  4. the capture and the heading reader's row — ADR 0066 D2's amendment of
//     2026-11-08 (ranger-base-jnmx6, built in ranger-base-w0vz2) — are
//     dropped as a cost nobody asked for. Nothing fails: the rule and region
//     are still on the line, and on the two blocked rules that name a SHAPE
//     rather than a screen (`live_blocked_form`, `live_strong_blocker`) the
//     operator is back to peeking. Arms 4-7, which are also the arms that
//     keep the amendment's four gates from widening: the capture rides only
//     beside an agreeing explain, only on a D2 record, never on a dry pass,
//     and the census counts it apart from D3's.
//
// They live in arm 3 beside startupscreen_test.go, whose
// TestDispatchAnswersNoBlockedScreen is the pin that NOTHING HERE PRESSES A
// KEY at any of these screens (rangerhq-4mzt, ADR 0013 §2). This is a
// diagnostic bead only, and the two files are read together.

import (
	"encoding/json"
	"os"
	"os/exec"
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
//
// THE PANE SHOWS THE SHIPPED FIXTURE, read off disk and not retyped
// (bsUpsellFixture). The screen is the other half of ranger-base-72oz4's
// measurement and the input the heading reader is scored on, so a rig that
// typed its own heading would be a rig that could pass while
// `etc/herdr/agent-detection/testdata/claude/blocked-fullscreen-upsell.txt`
// and `KnownScreens`' marker had drifted apart — which is the one thing
// `claude.fullscreen_upsell` asserts about the world.
func bsIncident(t *testing.T, extra func(fake string)) (*Dispatcher, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	bsArm(t, b, fake, extra)
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a blocked pane dispatched %d beads, want 0:\n%s", n, dispatcherOut(d))
	}
	return d, dispatcherOut(d)
}

// bsArm is bsIncident without the pass — the levers alone, for the one arm
// that cannot go through `d.Run` (arm 6: a dry pass never reaches awaitAgent,
// so the guard under test is unreachable from there).
func bsArm(t *testing.T, b *HerdrBackend, fake string, extra func(fake string)) {
	t.Helper()
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
	// And the screen itself, for the capture `pane read --source detection`
	// takes (ADR 0066 D2 as amended). The fake serves pane-text/<pane>.
	write(t, filepath.Join(fake, "pane-text", "w1_p1"), bsUpsellFixture(t))
	if extra != nil {
		extra(fake)
	}
}

// bsUpsellFixture is the shipped capture of the incident's own screen.
//
// READ OFF THE TREE, never a literal here: `KnownScreens`' claude row carries
// one marker ("try the new fullscreen renderer") whose whole claim is that it
// is this file's words, and the pin that compares the two is
// d3knownscreen_qa_test.go's. What THIS rig needs is that the bytes the fleet
// would read are the bytes the reader is handed, so that a fixture recaptured
// off a reworded claude release reds here rather than passing over a heading
// nobody ships any more.
func bsUpsellFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(qibRepoRoot(t), "etc", "herdr", "agent-detection",
		"testdata", "claude", "blocked-fullscreen-upsell.txt")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v — this is ranger-base-72oz4's capture of the incident screen and the rig measures nothing without it", path, err)
	}
	return string(b)
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
	log := bsLogPath(t, d)
	if _, err := os.Stat(log); os.IsNotExist(err) {
		return nil
	}
	rs, err := ReadReadings(log)
	if err != nil {
		t.Fatalf("ReadReadings(%s): %v", log, err)
	}
	return rs
}

// bsLogPath is where that log is, whether or not anything was written to it —
// which is what arm 7 needs, since the census has to be handed a path and a
// missing file is a census over nothing rather than a census that says zero.
func bsLogPath(t *testing.T, d *Dispatcher) string {
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
	return log
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
// A PANE CAPTURE AND THE REPORT, which is the assertion ranger-base-jnmx6
// came back and changed. Until 2026-11-08 this arm asserted the opposite —
// no capture, no `looks_like` — because ADR 0066 D2 named exactly two
// consumers of the D3 report and this record was neither. The amendment
// names it the third and says it takes a capture, measured: over herdr's
// 243-character previews the heading reader names 1 of the 2 generic-rule
// blocked fixtures, over the whole pane 2 of 2.
//
// THE REGION IS THE ONLY ONE ON THIS RECORD, and that is what makes the set
// mean something here. `agent explain` is not armed with evaluated rules on
// this rig, so herdr contributes no previews at all — the capture is the one
// place the heading can have come from, and a writer that stopped taking it
// would leave a record whose `looks_like` could only be empty.
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
	if len(r.LooksLike) != 1 || r.LooksLike[0] != "claude.fullscreen_upsell" {
		t.Errorf("the record carries looks_like %v, want [claude.fullscreen_upsell] — the field is the only thing that makes the saving countable, and `scripts/readings-census.py` counts it (ADR 0066 D2 as amended, rule 4)", r.LooksLike)
	}
	capture, ok := r.ReadingRegionOf(PaneCaptureRegion)
	if !ok {
		t.Fatalf("the record carries no %s region — that is the input the heading reader names 2 of 2 generic-rule fixtures from, against 1 of 2 over herdr's previews, and without it the next KnownScreens row has to be drawn on a rig instead of copied off the fleet (ADR 0066 D2 as amended)", PaneCaptureRegion)
	}
	if capture.Bytes <= 0 || capture.Truncated {
		t.Errorf("the capture says bytes=%d truncated=%v — a capture is the screen WHOLE, in the shape `herdr agent explain --file` reads back, and a previewed one is the input this amendment measured and rejected", capture.Bytes, capture.Truncated)
	}
	for _, reg := range r.Herdr.Regions {
		if reg.Name != PaneCaptureRegion {
			t.Errorf("the record carries a %q region beside the capture — this rig arms no evaluated rules, so a second region means the set below could have been read off a preview and this arm no longer measures the capture", reg.Name)
		}
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

// Arm 4 — the heading reader's row rides on arm 1's line, AFTER the rule and
// region, and inside the same status parentheses (ADR 0066 D2 as amended
// 2026-11-08, rule 3).
//
// THE ORDER IS THE ASSERTION and not just the presence. herdr's rule is
// replayable at the codepoint and this reader's answer is not, so a line that
// led with "looks like" would be putting the weaker reader first — and one
// that REPLACED the rule with it would have thrown away the diagnostic ADR
// 0066 D2 prefers to every report. The screen is the whole point on this
// pane: `live_blocked_form` matches every form claude draws, so the rule
// alone is "a form, at the bottom" and ranger-base-yk5qb's operator still had
// to peek.
func TestQABlockedSettleRefusalCarriesTheLooksLikeRow(t *testing.T) {
	t.Parallel()
	_, out := bsIncident(t, nil)
	row := "looks like: claude.fullscreen_upsell"
	if !strings.Contains(out, row) {
		t.Fatalf("the refusal does not name the screen — `live_blocked_form` is every form claude draws, so this line sends its reader to `posse peek` exactly as the bare one did:\n%s", out)
	}
	rule := strings.Index(out, `rule "live_blocked_form"`)
	if rule < 0 {
		t.Fatalf("the rule is gone from the line, so this arm cannot say the row appended to it:\n%s", out)
	}
	if at := strings.Index(out, row); at < rule {
		t.Errorf("the row is printed BEFORE the rule — the diagnostic that can be chased to a codepoint leads, and the report follows it (ADR 0066 D2):\n%s", out)
	}
	if !strings.Contains(out, "; "+row+")") {
		t.Errorf("the row is not `; looks like: ...` LAST inside the status parentheses — it has to follow herdr's whole clause and close it, not sit loose in the sentence:\n%s", out)
	}
	if strings.Count(out, "looks like") != 1 {
		t.Errorf("the line carries %d `looks like` rows, want exactly 1:\n%s", strings.Count(out, "looks like"), out)
	}
}

// Arm 5 — a DISAGREEING explain gets neither the capture nor the row, and
// neither does a failing one (ADR 0066 D2 as amended, rule 1).
//
// WHY THIS IS THE GATE. `agent wait` settled on `blocked` and the decision is
// taken; the explain beside it is a LATER instant. Where it names a different
// state, its rule is a rule about a different screen — so a capture taken
// then, and a screen named off it, would be the wrong screen named with more
// confidence than the bare line it replaced. ranger-base-x2grk refuses that
// one field to the left, for the rule, and this arm is the same refusal for
// the capture.
//
// THE PANE IS STILL SHOWING THE UPSELL, which is the whole strength of this
// arm: a read WOULD have named the screen. What must be absent is absent
// because the gate held, not because there was nothing to find.
func TestQABlockedSettleTakesNoCaptureWithoutAnAgreeingExplain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		arm  func(fake string)
	}{{
		// explain answers, and answers a DIFFERENT state. blockerEvidence
		// drops it and the bare detection stands.
		name: "disagrees",
		arm:  func(fake string) { write(t, filepath.Join(fake, "explain-state"), "working") },
	}, {
		// explain does not answer at all — a herdr that went away between
		// the two calls, which is arm 3's lever.
		name: "fails",
		arm: func(fake string) {
			write(t, filepath.Join(fake, "explain-error"), "agent_explain_unavailable|fake herdr: gone")
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d, out := bsIncident(t, tc.arm)
			if strings.Contains(out, "looks like") {
				t.Errorf("the line names a screen read at an instant the decision was not taken at:\n%s", out)
			}
			for _, r := range bsReadings(t, d) {
				if r.Consequence != ConsequenceRefusal {
					continue
				}
				if len(r.LooksLike) != 0 {
					t.Errorf("the record carries looks_like %v over a reading with no agreeing explain", r.LooksLike)
				}
				if _, ok := r.ReadingRegionOf(PaneCaptureRegion); ok {
					t.Error("a capture was taken beside a detection herdr's second read did not agree with — the bytes are a later screen and the record would file them under `agent wait`'s verdict")
				}
				return
			}
			t.Error("no refusal record: a refusal posse could not explain is still a refusal ADR 0066 D1 records")
		})
	}
}

// Arm 6 — a dry pass forks no `pane read` and writes no record, and the set
// it hands back is the preview-only one (ADR 0066 D2 as amended, rule 4's
// dry-run clause, carried by d3Evidence).
//
// NOT THROUGH `d.Run`, and that is deliberate rather than a shortcut: a dry
// pass prints its routing and `continue`s before `launchSession`
// (dispatch.go's `if d.DryRun` in the launch loop), so awaitAgent is
// unreachable and the guard under test would never be asked. That earlier
// return is a SECOND guard; this arm pins the one that has to survive its
// removal, which is where d3Evidence's own comment says the cost lives — a Go
// argument is evaluated before the function that would have discarded it, so
// a capture wrapped at the call site would be forked by a pass that then
// refuses to write the record.
//
// THE SAME CALL TWICE, live then dry, with the flag the only difference: the
// live half is what says the rig can produce a capture and a set at all, so
// the dry half's two zeroes are a guard holding rather than a rig that was
// never armed.
func TestQABlockedSettleTakesNoCaptureOnADryPass(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	bsArm(t, b, fake, nil)
	if n, err := d.Run("", "", 0); err != nil || n != 0 {
		t.Fatalf("the live pass dispatched %d beads (%v), want 0 — it is what creates the session this arm logs under", n, err)
	}
	live := bsReadings(t, d)
	if len(live) != 1 || len(live[0].LooksLike) != 1 {
		t.Fatalf("the live pass wrote %d reading(s) and named %v — this arm's dry half measures nothing unless the live one takes a capture", len(live), live)
	}
	readsBefore, _ := os.ReadFile(filepath.Join(fake, "pane-read-log"))
	if len(readsBefore) == 0 {
		t.Fatal("the live pass read no pane, so a dry pass reading none says nothing")
	}

	d.DryRun = true
	det := AgentDetection{State: "blocked"}
	det.Rule.ID = "live_blocked_form"
	det.Rule.Region = "after_last_horizontal_rule"
	det.Rule.Priority = 980
	looksLike := d.logBlockedRefusal("a-1", "ranger-qa", "claude", fakeHerdrSeat, "blocked", det)

	if len(looksLike) != 0 {
		t.Errorf("the dry pass named %v — with no capture the only regions are herdr's previews, and this rig arms none, so a non-empty set here means a `pane read` was forked for a record the dry run then refused to write", looksLike)
	}
	readsAfter, _ := os.ReadFile(filepath.Join(fake, "pane-read-log"))
	if len(readsAfter) != len(readsBefore) {
		t.Errorf("a dry pass read the pane:\n%s", strings.TrimPrefix(string(readsAfter), string(readsBefore)))
	}
	if rs := bsReadings(t, d); len(rs) != len(live) {
		t.Errorf("a dry pass wrote %d reading(s) on top of the live pass's %d — a census over a log that counted them would price a decision nobody made (logReading)", len(rs)-len(live), len(live))
	}
}

// Arm 7 — the census counts this record under its OWN row, and the D3 row
// does not move (ADR 0066 D2 as amended, rule 4, verbatim: the "unknown
// screens NAMED" line "stays D3-only").
//
// WHY THE TWO ROWS CANNOT BE ONE. On a D3 record a non-empty set is, by
// construction, a screen herdr's rules did not name at all — that is what
// makes the D3 line a true statement about unknown screens. Here a rule DID
// match; what it did not do is name a screen, because `live_blocked_form` is
// every form claude draws. The two findings have two different fixes — a new
// `KnownScreens` row for the first, a narrower herdr rule for the second —
// and a single rate over both would price them as one. So the pin asserts
// both halves: the new count IS counted, and the old count is still zero.
//
// THE REAL SCRIPT, for the reason the D3 arms give (d3knownscreen_qa_test.go,
// falseidle_qa_test.go): ADR 0066 D1 names an instrument, and a pin that
// reimplemented its arithmetic would be measuring a second copy.
func TestQABlockedSettleRefusalIsCountedApartFromD3(t *testing.T) {
	t.Parallel()
	script := filepath.Join("..", "..", "scripts", "readings-census.py")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("%s: %v — ADR 0066 D1 names a census script and this arm is the only thing that runs the new count", script, err)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatalf("no python3 on PATH, and %s is python3: %v", script, err)
	}
	d, _ := bsIncident(t, nil)
	out, err := exec.Command("python3", script, "--log", bsLogPath(t, d), "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --json: %v\n%s", script, err, out)
	}
	var c struct {
		Named struct {
			D3    int `json:"d3"`
			Named int `json:"named"`
		} `json:"named_screens"`
		Blocked struct {
			Refusals int            `json:"refusals"`
			Named    int            `json:"named"`
			Screens  map[string]int `json:"screens"`
		} `json:"blocked_screens"`
	}
	if err := json.Unmarshal(out, &c); err != nil {
		t.Fatalf("census --json is not json: %v\n%s", err, out)
	}
	if c.Blocked.Refusals != 1 || c.Blocked.Named != 1 || c.Blocked.Screens["claude.fullscreen_upsell"] != 1 {
		t.Errorf("the census says refusals=%d named=%d screens=%v, want 1, 1 and one claude.fullscreen_upsell — without this row the saving the amendment bought is uncounted, which is the state the D3 reader was built out of",
			c.Blocked.Refusals, c.Blocked.Named, c.Blocked.Screens)
	}
	if c.Named.D3 != 0 || c.Named.Named != 0 {
		t.Errorf("the D3 line moved: d3=%d named=%d over a log holding one D2 refusal and no D3 record at all — that line's claim is \"herdr's rules named no screen\", and a blocked refusal matched a rule", c.Named.D3, c.Named.Named)
	}
}
