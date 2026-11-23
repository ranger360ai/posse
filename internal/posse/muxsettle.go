package posse

// The occupant's own settle is the gate's evidence (ranger-base-25otk, ADR
// 0076 Amendment 12).
//
// WHAT THE LADDER WAS DOING WITH IT. `awaitSettled` reads `res.State` off
// `Mux().Wait` and drops `res.Seen`. On herdr that costs nothing — herdr's
// `waitReading` leaves Seen false on purpose, because an `agent wait`
// envelope reports a STATE and never what recognized it, so there is nothing
// in it to drop. On a Rex instance it costs the whole gate: `agent explain`
// is a §4 verb that occupant does not have (muxVerbMissing), so the
// confirmation the ladder waits for can never arrive, `lastWhy` stays empty
// for the entire window, and the launch is decided by the deadline arm's
// concession on whichever reading was LAST.
//
// WHAT THAT COST, MEASURED 2026-11-23 (UTC; 2026-11-22 -05:00 local) over the
// live fleet's two dispatch pass logs — `state/dispatch-watch.log` and
// `.log.1`, 66,476 lines, the earliest `== dispatch --watch armed ==` line
// 2026-11-10 18:53:28. Re-read of hcig1's census on a log 189 lines longer:
//
//   - 857 `prompting on its … anyway` lines (583 + 274) against 0 `never
//     became promptable` and 0 `herdr never saw a screen it recognizes`. The
//     concession is the only way this arm has ever exited on this instance,
//     and every one of those 857 launches spent its runtime's whole
//     `startup_wait:` — 45s for claude here, which declares none and so takes
//     DefaultStartupWait — before typing on a state nothing had confirmed.
//   - 5 launches abandoned by the occupant's own `Wait` timeout instead
//     (settletimeout.go's census, unchanged: 46 abandonments, 21 of them a
//     bounded wait that ran out on the seat). Two of the five —
//     ranger-base-72z36 at 16.446s left and ranger-base-k0hd6 at 2.203s —
//     read `working` only at the END of a window they had been reading
//     settled through. They were promptable tens of seconds before they were
//     abandoned, and the reading that said so was handed to the ladder and
//     dropped.
//
// WHY THE FIELD IS EVIDENCE AND NOT A GUESS. ADR 0013 §1's settle row's
// observable is `idle`/`done` *Seen()* — "a matched rule, not the
// idle-fallback" — and ADR 0076 §3 declares `MuxReading.Seen` to be "the Rex
// form of herdr's `Seen()`": a program_status record whose owner is not the
// block's own shell and is still a live process in it (rexReading's three
// guards). So the gate's own standard is this field, and the occupant answers
// it on every settled Wait. The `explain` was herdr's CONFIRMATION of a fact
// herdr's wait envelope cannot carry, not a second condition the grid asks
// for — which is why its permanent absence had to be read as "confirm it
// another way" and never as "concede".
//
// WHAT IT STILL DOES NOT PROVE, and why this is not the last word before a
// keystroke. A record says what the runtime claims about its TURN and nothing
// about what it has DRAWN (ADR 0076 Amendment 7: `program_status`'s fields
// are state, app, effective_app, title, msg, updated_at, owner — MEASURED —
// and claude's `/model` picker and its folder-trust prompt can be up with
// nothing running at all). A settle is therefore a settle and not a licence
// to type: the keystroke's own guards are at the delivery verb, where
// Amendment 7's composer-absence refusal and Amendment 9's dead-owner refusal
// stand unchanged — "on a rex instance this verb's own refusals ARE the gate"
// (refuseAtDeadOwner). This arm moves no part of that.
//
// WHY A GATE MAY OPEN ON IT AT ALL, which is the ruling the bead asked for.
// Amendment 5 licensed a reading whose only output can be "this settle is not
// one", Amendment 7 added "do not send this keystroke", and both say such a
// reading may never LICENSE one. Neither reaches this: both are about
// readings posse COMPOSES off a screen, and this is §2's own verb answering
// with §3's own declared field. The direction of risk is also the opposite of
// what a loosening would be — what this replaces is a concession that types
// on `status` with no positive evidence whatsoever, 857 times out of 857, so
// accepting the field is strictly more evidence and strictly less patience
// than the arm it short-circuits.

// muxSettledGate is the whole of that rule, in one place: the occupant
// reported a SETTLED state and said it has positive evidence for it.
//
// BOTH HALVES, AND THE STATE HALF IS NOT REDUNDANT. awaitSettled has already
// handed `blocked` back by name one arm above this, so by here the state can
// only be settled — but the rule a reader needs to see is "settled AND seen",
// and a predicate that read only the second half would be one edit away from
// opening this gate on `working`. ADR 0076 §2's own note says Seen "is never
// derived from State", and the converse holds here: neither field stands in
// for the other.
//
// `ok` false is not a refusal. It is "this reading is not one this arm can
// act on", and the ladder goes on to the confirmation it has always taken —
// which is every herdr reading, because a herdr wait envelope carries no
// evidence to read (muxherdr.go's waitReading).
//
// MuxSeen is set in this FILE and nowhere else (muxsettle_qa_test.go pins
// that mechanically): a `Seen()` that can license a settle may only come from
// the occupant's own answer to §2's verb, never from a field some other site
// found convenient to set.
//
// THE EVIDENCE HALF IS READ THROUGH muxSeenGate and not spelled again here
// (ranger-base-q5fee). Two gates now read that half — this one and the
// readiness gate, which asks for it alone — and a second spelling of "the
// occupant says it has evidence" is two places to disagree about what
// evidence is. What this function owns is the state half, which is its own.
func muxSettledGate(read MuxReading) (AgentDetection, bool) {
	det, ok := muxSeenGate(read)
	if !ok {
		return AgentDetection{}, false
	}
	if read.State != MuxIdle && read.State != MuxDone {
		return AgentDetection{}, false
	}
	return det, true
}

// ─── the readiness half: evidence alone (ranger-base-q5fee, Amendment 13) ────
//
// TWO MORE LADDERS TOOK THE §4 CONCESSION WITH NO OCCUPANT READING AT ALL,
// and the verb that answers them is not a §4 verb. `Mux().State` is one of
// ADR 0076 §2's ten, so every occupant answers it — and MEASURED 2026-11-23
// (UTC; 2026-11-22 -05:00 local) on the HEAD that landed Amendment 12,
// nothing outside internal/posse/muxrex.go called it at all: the verb existed
// and no ladder read it. The two:
//
//   - AwaitPromptable (promptready.go), the hand-prompt readiness gate. On a
//     §4-missing instance every poll errors, so it concedes after
//     promptExplainGrace with the ZERO detection — no reading, no state, and
//     `Seen()` false. MEASURED over the live fleet's two dispatch pass logs
//     (`state/dispatch-watch.log` and `.log.1`, 66,766 lines, the earliest
//     `== dispatch --watch armed ==` line 2026-11-10 18:53:28): **518**
//     `prompting without the readiness gate` lines, and every one of the 518
//     is the PULSE. The pulse's own rule — "a shop check is a nudge nobody
//     asked for and must not land on a persona mid-turn" (ranger-base-k99a)
//     — is written as `det.Seen() && !pulseTakesPrompt(det.State)` over this
//     gate's reading, so on this instance it has never once been able to
//     fire. The listing's cheap pre-filter is evidence-backed on Rex and
//     caught the visibly-working seats; what was missing is the RE-READ at
//     the instant of typing, which is the whole reason the gate is a second
//     reader (`agent wait` and `agent explain` "read the same detection at
//     two different instants, and the second one wins").
//   - awaitDelivered (dispatch.go), the argv path's observation. It concedes
//     and gathers, so the consequence was already the generous one; what it
//     loses is the OBSERVATION — `launched.unseen`, which decides whether the
//     bead is judged this pass. MEASURED in the same window: **0**. No
//     `prompt: argv` runtime has launched on this instance, so this half is
//     the shape being closed before it is paid rather than a cost already
//     counted. Said as zero rather than left out: a reader comparing the two
//     sites deserves to know which one the box has exercised.
//
// THE PREDICATE IS THE EVIDENCE HALF ALONE AT BOTH SITES, and that is not a
// loosening of muxSettledGate's "both halves" — it is the other gate's own
// rule, which promptready.go has stated since it was written: it waits for a
// screen herdr has SEEN "and NOT for a particular state", because the
// operator prompting by hand may well be nudging an agent that is mid-turn,
// and "a working pane herdr recognizes is a pane whose CLI has the keyboard,
// which is the only question this bug asks". awaitDelivered asks the same
// question of the same field — its `det.Seen()` arm takes any state too,
// because detectability is what says the launch line ran.
//
// IT STILL LICENSES NO KEYSTROKE, and at these two sites that sentence has a
// second half: the refusal that WOULD be new here is already somewhere else.
// A §4-missing instance's reading with no evidence is not read as "no" —
// unknown is never no (ADR 0013 §1) — so each caller's own concession stands
// where it stood and the gate still prompts. What stops the keystroke when nothing holds
// that keyboard is the delivery verb's own refusals, where Amendment 7's and
// Amendment 9's live: "on a rex instance this verb's own refusals ARE the
// gate" (refuseAtDeadOwner, refuseWithoutComposer). So this arm adds a
// positive and invents no negative.

// muxSeenGate is the evidence half on its own: the occupant says it has
// positive evidence that the runtime holds this seat's keyboard, whatever
// state it is in.
//
// The state rides into the detection unexamined, which is the difference from
// muxSettledGate and the whole of it. A state the occupant named and this
// package does not know is still a reading (`rexMapState` answers MuxUnknown
// for one), and an occupant that answered with evidence and no state at all
// has still answered THIS question — the gates that care about the state ask
// it themselves, and both of this arm's callers deliberately do not.
func muxSeenGate(read MuxReading) (AgentDetection, bool) {
	if !read.Seen {
		return AgentDetection{}, false
	}
	return AgentDetection{State: read.State, MuxSeen: true}, true
}

// muxSeen is that rule asked of the occupant, through the one door it may be
// asked through: a §4 reading that was NOT AVAILABLE.
//
// THE DOOR IS `muxVerbMissing` AND NOT "the explain errored", and that is the
// whole blast radius (Amendment 13 D3). `HerdrMux.State` IS `agent explain`
// flattened — the same call, the same answer — so on herdr this would be a
// retry of the call that just failed at the same instant, buying no second
// reading and spending a second subprocess per poll. §4's door answers
// "unavailable" only where the slot holds something that is not herdr
// (HerdrBackend.explain), so herdr never reaches the read at all: no extra
// call, no changed ladder, and the fake herdr's own `explain-error-after N`
// and `explain-error-for N` countdowns — which count CALLS — keep meaning
// what they say.
//
// An error from the occupant's own verb is ignorance and never a refusal,
// which is every other reader of this verb (AwaitFirstReport's three
// outcomes, RexMux.Wait's "an error ends the wait rather than being
// retried"). The caller's concession is what it always was.
func (b *HerdrBackend) muxSeen(target Seat, explainErr error) (AgentDetection, bool) {
	if _, missing := muxVerbMissing(explainErr); !missing {
		return AgentDetection{}, false
	}
	read, err := b.Mux().State(target)
	if err != nil {
		return AgentDetection{}, false
	}
	return muxSeenGate(read)
}
