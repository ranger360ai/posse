package posse

// The settle window's OTHER exit (ranger-base-hcig1).
//
// awaitSettled's window closes in two places and until this file only one of
// them was legible. The arm at the foot of that loop is the one everything is
// written about: it reaches its own deadline holding a reading, writes a D3
// record, and names the session, the patience and the `posse peek` that reads
// it. The other is one line long and three lines earlier — the occupant's own
// `Wait` refusing, because the ladder sizes that call from the SAME deadline
// and so the occupant's window and the ladder's close together. That error
// left awaitSettled raw.
//
// WHAT THAT COST. MEASURED 2026-11-23 over the live fleet's two dispatch pass
// logs (`state/dispatch-watch.log` and `.log.1`, 66,287 lines, armed
// 2026-11-10 18:53 through the reading): 46 lines of the launcher's own
// abandonment shape `✗ <bead> <reason>`, of which 21 are a bounded wait that
// ran out on the SEAT rather than on a cage, a claim or a runtime that ended.
// 16 of those 21 are awaitTarget's, and they read:
//
//	✗ <bead> no agent detected in <seat>-posse-<bead>
//	  after 45s — check the session (posse peek <seat>-posse-<bead>)
//
// The other 5 are this exit, in five different beads:
//
//	✗ ranger-base-72z36 herdr: no idle|done|blocked state in
//	  block:01a205f7-7021-75cf-9b39-b0a384535850 within 16.446s
//	  (last reading: working) (timeout)
//	✗ ranger-base-k0hd6 … within  2.203s (last reading: working)
//	✗ ranger-base-kzbg8 … within 32.763s (last reading: unknown)
//
// …and two more of that third shape, `unknown` at 42.948s and at 26.541s,
// whose beads are open and are therefore named here by id and nothing else
// (AGENTS.md, "Landing the plane"):
//
//   - ranger-base-1mxhx
//   - ranger-base-al0d0
//
// Three things are missing from each, and each is something the sibling line
// one paragraph up already has:
//
//  1. the SESSION. A block UUID is not a name any posse verb takes, and the
//     operator reading this cannot get from it to the seat, the persona, the
//     tree or the log without a listing.
//  2. the `posse peek`. The next step is the same next step as the other 16
//     and the line does not say it.
//  3. a readings-log record. `scripts/readings-census.py` counted 0 of these
//     5, so the denominator for "launches posse refused at the settle gate"
//     was missing its whole timeout half — the same unfilled corner of ADR
//     0066 D1 that logBlockedRefusal was written for, on the arm next door.
//
// The word `herdr:` leading a line printed on an instance whose multiplexer
// is Rex is the fourth thing, and it is HerdrAPIError.Error()'s prefix rather
// than this arm's doing (ranger-base-l32dl's class, one envelope lower). The
// sentence this file prints does not carry it.
//
// WHY THIS IS AN ARM AND NOT A FALL-THROUGH. The deadline arm below it makes
// a concession first: when no poll in the window ever produced a real
// detection (`lastWhy == ""`) and the last diagnostic errored, it prompts the
// pane on whatever state it has, out loud. On an instance whose multiplexer
// has no `agent explain` (ADR 0076 §4) that is EVERY launch — the diagnostic
// is permanently unavailable, so lastWhy is permanently "" — and the same two
// logs read 856 `prompting on its … anyway` lines against 0 `never became
// promptable`, which is the concession firing every time and the refusal
// never. So a timeout folded into that arm unchanged would not be refused at
// all: it would PROMPT a pane whose last reading was `working`, which is
// rangerhq-37c's race — a prompt typed at a runtime that has not taken the
// keyboard, eaten, with the claim spent. The consequence this arm keeps is
// therefore the consequence the raw error already had (the launch is
// abandoned, nothing is typed); what it adds is a record and a sentence.
//
// AND IT IS A REFUSAL, in ADR 0066 D1's sense, for logBlockedRefusal's reason
// verbatim: awaitAgent runs before `d.claim`, so there is no claim to hand
// back and nothing was ever sent.
//
// NO PANE CAPTURE, BY A RULE THIS PACKAGE ALREADY ARGUED. blockedsettle.go's
// first gate: the D1 branch — any non-settled state that is `agent wait`'s
// state and nothing else — gets no capture, "because there is no screen for a
// reader to name, and a capture there would be posse reading a pane its own
// decision never looked at". A `Wait` that ran out is that branch exactly: a
// state, from a verb that answers with a state. So no capture, and with no
// capture no `LooksLike` row — which is also what keeps Reading.LooksLike's
// documented scope (D3, and the one D2 class) true.
//
// WHAT WOULD MAKE THE 5 NOT HAPPEN AT ALL is a different bead and not this
// one: on a §4-missing instance the ladder cannot SEE a settle, so it spends
// the whole startup wait and decides on whichever reading is last — while
// `Wait` has been handing it a `Seen` the ladder drops on the floor
// (res.State is read, res.Seen is not). Two of the five had been reading
// settled for most of their window. Filed as ranger-base-25otk; this arm
// makes the abandonment legible and changes nothing about when it happens.

import (
	"fmt"
	"time"
)

// refuseUnsettled is awaitSettled's refusal when the occupant's own Wait ran
// the window out: the record ADR 0066 D1 asks for, and the sentence the other
// exit prints.
//
// ONE SENTENCE FOR BOTH EXITS, which is the point. It is the deadline arm's
// line — `agent in <session> never became promptable within <wait> — <why>;
// check the session (posse peek <session>)` — with this exit's own `why`.
// "Never became promptable" is the true claim on both: this gate's standard is
// a settle it can accept, and a pane that reported `idle` at a poll it could
// not confirm never met it. A second spelling of one fact is what the bead
// that filed this was about, so the DIVERGENCE is put where a census reads
// and an operator does not — the record's verdict, prefixed `never settled:`,
// so `scripts/readings-census.py` can price the two exits apart.
//
// THE PATIENCE NAMED IS THE LADDER'S AND NOT THE CALL'S. `wait` is the window
// awaitAgent gave the settle (a fresh `time.Now().Add(wait)`); the occupant's
// own message names only what was left of it when its last `Wait` began,
// which is smaller every poll and is a fact about this loop's arithmetic
// rather than about the seat. The ladder's number is the one the operator set
// and the one the sibling line prints.
//
// IT RETURNS THE ERROR RATHER THAN DYING, because awaitSettled's contract is
// (status, detection, error) and awaitAgent is the function that ends the
// launch. The status it returns beside this is "" and the detection is the
// zero one, exactly as the deadline arm's refusal does: there is no state
// this gate accepted, and a caller must not be handed one it could branch on.
//
// IT TAKES NO TARGET, and that is the no-capture rule above read as a
// signature: the only use a pane would have here is a `pane read` this arm is
// not allowed to take, so the seat is not an argument to a decision that does
// not read it.
func (d *Dispatcher) refuseUnsettled(id, session string, wait time.Duration, waitErr error) error {
	last, carried := MuxWaitLastReading(waitErr)
	ev := ReadingEvidence{}
	why := fmt.Sprintf("the multiplexer ran the whole window out and carried no reading back (%v)", waitErr)
	verdict := fmt.Sprintf("never settled: the wait ran out with no reading (%v)", waitErr)
	if carried {
		// Seen rides across because the two occupants answer the same
		// question from different facts and this field is where that answer
		// is recorded (ADR 0076 §3, "the Rex form of herdr's Seen()"):
		// `working` with Seen is a runtime that holds the keyboard and is
		// busy, `unknown` without it is a seat that has reported nothing at
		// all, and those are the two halves of the measured five.
		ev = ReadingEvidence{State: last.State, Seen: last.Seen}
		why = fmt.Sprintf("the multiplexer ran the whole window out and the last state it read was %q", orUnknown(last.State))
		verdict = fmt.Sprintf("never settled: the wait ran out, last reading %q", orUnknown(last.State))
	}
	// RuleSettleGate, by its own doc: "posse refused a launch on a state
	// herdr answered and no rule was in hand". A timeout is the one shape of
	// that where there was never a rule to have — the verb answers with a
	// state — so the gate names itself, which is what every refusal in this
	// package does when no manifest rule produced the verdict
	// (blockedRefusalRule's second arm, and the D3 record on this ladder).
	//
	// No Runtime: this ladder takes a target and never a runtime, which is
	// the same reason the deadline arm's record carries none.
	d.logReading(session, Reading{
		Decision:    DecisionPaneState,
		Verdict:     verdict,
		Consequence: ConsequenceRefusal,
		Rule:        RuleSettleGate,
		Bead:        id,
		Herdr:       ev,
	})
	return Die("agent in %s never became promptable within %s — %s; check the session (posse peek %s)", session, wait, why, session)
}
