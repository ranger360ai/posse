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
// SO THIS IS NOT THE D3 READER — AND SINCE ranger-base-jnmx6 IT CARRIES THE
// D3 REPORT ANYWAY. The two halves of that reader have to be kept apart here.
// `WhatHerdrSaw` — herdr's whole working, every rule it tried and a preview
// of each region — is D3's and is NOT printed on this line: a blocked settle
// has a rule, so there is no working to explain, which is what "it only ever
// speaks where that one said nothing" (knownscreen.go's own line about the
// heading reader) was written against. What rides here is that reader's one
// ROW, `LooksLikeLine`, and ADR 0066 D2 as amended 2026-11-08 names this
// refusal its THIRD consumer for a measured reason: two of herdr's blocked
// rules name a SHAPE and not a screen — `live_blocked_form` (claude, every
// form it draws, the trust dialog included) and `live_strong_blocker`
// (codex) — so on those two the rule says "a form, at the bottom" and the
// operator still has to peek. MEASURED 2026-11-08 over every shipped fixture
// that reads `blocked` (ten): eight match a screen-specific rule whose id
// already names the screen, two match a generic one, and those two are where
// the row is the difference between this line and ranger-base-yk5qb's
// hand-launch.
//
// THE RULE AND THE REGION STAY FIRST AND THE ROW ONLY APPENDS (ADR 0066 D2
// rule 3). herdr's rule is replayable at the codepoint and this reader's
// answer is not, so the diagnostic that can be chased leads and the report
// follows it — the same order, and the same reason, as the D3 block next
// door.
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
// The EXPLAIN is NOT guarded by the dry-run rule the way d3Evidence's pane
// read is. That guard exists because a `pane read` is a cost no dry pass
// should fork; an `agent explain` is already made unguarded on every poll of
// the ladder directly above this, so one more on its exit adds no class of
// call a dry pass was not already making. The RECORD is guarded, by
// logReading itself — and the CAPTURE this file now takes beside the record
// is guarded too, by going through d3Evidence and nothing else.
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
//
// THE HEADING READER'S ROW FOLLOWS THIS CLAUSE and never replaces it
// (awaitAgent, ADR 0066 D2 rule 3): `; looks like: <set>`, inside the same
// status parentheses, for the two blocked rules that name a shape rather than
// a screen. This function neither knows nor asks about it.
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
// A PANE CAPTURE AND THE D3 REPORT, ON THE D2 BRANCH ONLY — which is the
// 2026-11-08 amendment and was a refusal in this file until it (ADR 0066 D2,
// ranger-base-jnmx6, from ranger-base-w0vz2). The capture is what the
// known-screen heading reader needs: over herdr's 243-character previews
// alone it names 1 of the 2 generic-rule blocked fixtures, over the whole
// pane 2 of 2 — the same input bottleneck ranger-base-qk9tr measured at 5 of
// 11 against 11 of 11 on the D3 path. It does a second job the report does
// not: a blocked refusal on a screen `KnownScreens` has no row for leaves a
// fixture-shaped capture in the log, so the next row is copied from the fleet
// rather than drawn on a rig (the step ranger-base-72oz4 had to do by hand).
//
// THE FOUR GATES, in the order the code applies them — which is not the
// amendment's numbering, so each one cites the rule it is (rule 3, the line's
// order, is awaitAgent's and is argued at the head of this file):
//
//  1. `status "blocked"` only — the D2 branch (rule 2). The D1 branch (any
//     other non-idle status) is not a screen reading at all: it is `agent
//     wait`'s state and nothing else, so there is no screen for a reader to
//     name, and a capture there would be posse reading a pane its own
//     decision never looked at.
//  2. An AGREEING explain only (rule 1), and `det.Seen()` is the test. The
//     bare detection blockerEvidence synthesizes when explain disagreed or
//     failed carries no rule, no label and no visible-idle, so it fails
//     Seen() by construction — and a later instant's screen attached to
//     `agent wait`'s state would be the wrong screen named with confidence,
//     which is what ranger-base-x2grk already refuses one field to the left,
//     for the rule. Seen() and not a flag threaded out of the ladder because
//     it is the reader this package already has for "herdr's own read said
//     something positive about this pane", and because it makes the two
//     D3-report consumers in dispatch.go the two sides of one question:
//     awaitDelivered speaks where `det.Seen()` is false and herdr recognized
//     nothing, this speaks where it is true and the rule it recognized names
//     a shape.
//     NARROWER THAN THE RULE'S LETTER IN ONE SHAPE, said out loud: an
//     explain that agreed on `blocked` while naming no rule at all fails
//     Seen() and gets no capture. herdr does not produce that shape — its
//     fallback state is `idle` — and where it ever does, withholding is the
//     safe direction and the one this whole ADR takes.
//  3. The reported route gets neither (rule 2's second half), and no gate
//     here says so: withPaneCaptureNamed refuses a pane herdr does not
//     address, so the evidence comes back with no capture and the reader
//     reads no regions (ADR 0061 D3.3).
//  4. The dry-run rule, carried by d3Evidence and not restated — a dry pass
//     forks no `pane read`, exactly as on the D3 path. Not one of the
//     numbered rules: it is the amendment's "through `d3Evidence` (dry-run
//     guarded)" and it is a gate because that is the function the capture
//     goes through.
//
// IT RETURNS THE SET, for logUnrecognized's reason: the line and the record
// have to carry the SAME reading, the evidence is built here and the capture
// is taken here, and a caller recomputing it off the detection would get the
// preview-only reader this amendment measured and rejected. On a dry pass no
// record is written and no capture is taken, and the set returned is then the
// preview-only one — the best reading the evidence in hand supports, which is
// the answer logUnrecognized gives in the same case and for the same reason.
func (d *Dispatcher) logBlockedRefusal(id, session, runtime, target, status string, det AgentDetection) []string {
	decision := DecisionPaneState
	ev := ReadingEvidenceOf(det)
	var looksLike []string
	if status == "blocked" {
		decision = DecisionDialog
		if det.Seen() {
			ev = d.d3Evidence(target, det)
			looksLike = ev.LooksLike()
		}
	}
	d.logReading(session, Reading{
		Decision:    decision,
		Verdict:     fmt.Sprintf("never settled idle: status %q", status),
		Consequence: ConsequenceRefusal,
		Rule:        blockedRefusalRule(ev),
		Bead:        id,
		Runtime:     runtime,
		Herdr:       ev,
		LooksLike:   looksLike,
	})
	return looksLike
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
