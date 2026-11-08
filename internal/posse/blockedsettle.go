package posse

// The settle refusal says WHICH screen (ranger-base-x2grk).
//
// WHAT IT COST TO NOT SAY IT. A fresh account's first dispatched claude seat
// sat on `Try the new fullscreen renderer? 1. Yes / 2. Not now`, and dispatch
// printed, whole:
//
//	agent in <session> never settled idle (status "blocked") — check the
//	session (posse peek <session>)
//
// The operator hand-launched and peeked to learn which screen it was
// (ranger-base-yk5qb, `docs/notes.d/ranger-base-yk5qb.md`) — the same
// per-event price ranger-base-3j8 measured for the unrecognized-screen line,
// paid here on a screen herdr RECOGNIZES. MEASURED under ranger-base-72oz4:
// that pane reads `blocked` from rule `live_blocked_form`, region
// `after_last_horizontal_rule`, priority 980, off the bundled claude
// manifest 2026.09.11.1. herdr had the answer and posse printed the state.
//
// SO THIS IS NOT THE D3 READER AND MUST NOT BE CONFUSED WITH IT. `WhatHerdrSaw`
// and the known-screen heading reader (unrecognized.go, knownscreen.go) speak
// only where herdr's rules said NOTHING — "it only ever speaks where that one
// said nothing" is knownscreen.go's own line. A blocked settle is the other
// case: herdr working, named by a rule, replayable at the codepoint, which is
// the diagnostic ADR 0066 D2 prefers to every report. What was missing was
// that posse never printed it.
//
// WHY ONE MORE `agent explain` (blockerEvidence). `agent wait` answers with a
// STATE and nothing else — `{"agent":{"agent_status":"blocked"}}`, no rule, no
// region, no manifest — so awaitSettled's first arm, which is the arm the
// incident went through, had no rule to return and did not pretend to: it
// handed back `AgentDetection{State: status}`. The evidence was therefore NOT
// already in hand on that path, whatever the bead that filed this said. One
// explain buys it, and this is logPromptHandBack's bargain argued in the same
// words: the arm is TERMINAL — awaitAgent dies on the next line — so the call
// happens once per refused launch and never in a poll loop, and a refused
// launch already costs a session nobody can use.
//
// It is NOT guarded by the dry-run rule the way d3Evidence's pane read is.
// That guard exists because a `pane read` is a cost no dry pass should fork;
// an `agent explain` is already made unguarded on every poll of the ladder
// directly above this, so one more on its exit adds no class of call a dry
// pass was not already making. The RECORD is guarded, by logReading itself.
//
// NOTHING IS PRESSED AT THE SCREEN, here or anywhere below it (rangerhq-4mzt,
// ADR 0013 §2, and startupscreen_test.go pins it as behaviour): a blocked pane
// is the operator's, and this file's whole output is a sentence and a log line.

import (
	"fmt"
	"strings"
)

// WhyBlocked is the clause that follows the status on awaitAgent's failure
// line: what herdr read the screen WITH, in herdr's own spelling.
//
// `rule "live_blocked_form" region=after_last_horizontal_rule priority=980`
// — the three fields `herdr agent explain` prints as
// `rule: live_blocked_form (region=after_last_horizontal_rule priority=980)`,
// in that order and under those names, so the operator can match the posse
// line against a herdr run and against the manifest without translating
// between two vocabularies. herdr's own parentheses are dropped because the
// failure line already has a pair around the status.
//
// NEVER "" — a clause that vanished would leave the reader unable to tell a
// line posse could not name from a line it did not try to, which is the
// confusion this file exists to end. Where there is no rule it says so and
// says what there was instead, through `herdrEvidence`: that reader's own
// first arm is the rule, which the branch it is called inside has already
// ruled out, so the two cannot disagree about one.
//
// A REGION OR PRIORITY OF ZERO IS LEFT OUT rather than printed as empty or
// `priority=0`. An older herdr emits neither field (AgentDetection.Rule), and
// the honest line there names the rule alone.
func (d AgentDetection) WhyBlocked() string {
	if d.Rule.ID == "" {
		switch saw := herdrEvidence(d); {
		case d.Reported != "":
			// The reported route reads no screen at all, so there is no rule
			// to have named and never will be (ADR 0061 D3.3): the evidence
			// is the other authority's word, which is what it says.
			return fmt.Sprintf("no rule: herdr reads no screen for a pane it did not detect, and the authority that labels this one %q reported the state", d.Reported)
		case saw != "":
			return "no rule, only " + saw
		}
		return "no rule: herdr named nothing that produced this state"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "rule %q", d.Rule.ID)
	if d.Rule.Region != "" {
		fmt.Fprintf(&b, " region=%s", d.Rule.Region)
	}
	if d.Rule.Priority > 0 {
		fmt.Fprintf(&b, " priority=%d", d.Rule.Priority)
	}
	return b.String()
}

// blockerEvidence is the reading awaitSettled hands back when it refuses: the
// state `agent wait` settled on, with the rule that produced it where one
// explain can still name it.
//
// THE DECISION IS NOT RE-TAKEN AND CANNOT BE. The status returned is `agent
// wait`'s, byte for byte, and the caller's branch is unchanged — this only
// ever fills in evidence beside it. A second instant that said `idle` must
// not open a gate the first instant closed: that is the loosening direction,
// and the ladder's own comment above only ever lets the second read win the
// other way (a fresh claude drawing its trust dialog in 30ms, which is the
// failure the gate exists for).
//
// WHICH IS ALSO WHY A DISAGREEMENT DROPS THE EVIDENCE rather than reporting
// it. `explain` is a later instant; if it names a different state, its rule
// is a rule about a different screen, and a failure line that attached it to
// `agent wait`'s status would be naming the wrong screen with more confidence
// than the bare line it replaced — the one outcome worse than saying nothing.
// The bare detection then stands and WhyBlocked says there is no rule.
func (d *Dispatcher) blockerEvidence(target, status string) AgentDetection {
	bare := AgentDetection{State: status}
	if target == "" {
		return bare
	}
	det, err := d.HB.explain(paneSeat(target))
	if err != nil || det.State != status {
		return bare
	}
	return det
}

// logBlockedRefusal records the refusal (ADR 0066 D1: "every pane-state,
// dialog … reading that ends in a refusal … is recorded with the region bytes
// it read"). Until this landed a blocked hand-back wrote NO record at all, so
// `scripts/readings-census.py` counted every launch posse refused on a screen
// herdr DID recognize as nothing — the denominator was missing the whole
// recognized half of the gate's refusals. That is an unfilled corner of D1
// rather than a new decision, and no ADR moves for it.
//
// WHICH DECISION IT WAS, by logPromptHandBack's taxonomy and in its words,
// because the two are the same reading reaching the same consequence one
// keystroke apart:
//
//	blocked        herdr read a blocker on the screen and posse refused   → D2
//	anything else  not a screen reading — `agent wait`'s state alone      → D1
//
// REFUSAL AND NOT HAND-BACK, which is the one place the bead that filed this
// had the shape wrong: awaitAgent runs BEFORE `d.claim` (launchSession), so
// there is no claim yet to give back. Nothing was sent and nothing was
// claimed, which is ConsequenceRefusal's own definition and what the D3
// record on the same ladder already uses.
//
// NO PANE CAPTURE, deliberately, and it is the line that keeps this inside
// ADR 0066 D2. A capture is what the known-screen heading reader needs
// (PaneCaptureRegion: over herdr's previews alone it names 5 of 11 residue
// cases instead of 11), and a `looks_like` on a blocked hand-back would be a
// THIRD consumer of the D3 report where D2 names exactly two. That is the
// architect's call, not this file's — filed, not assumed. A D1/D2 record
// carries herdr's own regions and nothing else, which is also what
// logPromptHandBack writes.
func (d *Dispatcher) logBlockedRefusal(id, session, runtime, status string, det AgentDetection) {
	decision := DecisionPaneState
	if status == "blocked" {
		decision = DecisionDialog
	}
	ev := ReadingEvidenceOf(det)
	d.logReading(session, Reading{
		Decision:    decision,
		Verdict:     fmt.Sprintf("never settled idle: status %q", status),
		Consequence: ConsequenceRefusal,
		Rule:        blockedRefusalRule(ev),
		Bead:        id,
		Runtime:     runtime,
		Herdr:       ev,
	})
}

// blockedRefusalRule is herdrRefusalRule's question with the other answer
// available: herdr's matched rule where the second read named one, and the
// posse codepoint that refused where it did not. There is no herdr error code
// to fall back on here — the refusal is posse's own branch on a state herdr
// answered successfully — so the gate names itself, which is what every D4-D6
// record's rule already is and what the D3 record on this same ladder uses.
func blockedRefusalRule(ev ReadingEvidence) string {
	if ev.Rule != "" {
		return ev.Rule
	}
	return RuleSettleGate
}
