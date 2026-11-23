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
// MuxSeen is set HERE and nowhere else (muxsettle_qa_test.go pins that
// mechanically): a `Seen()` that can license a settle may only come from the
// occupant's own answer to §2's verb, never from a field some other site
// found convenient to set.
func muxSettledGate(read MuxReading) (AgentDetection, bool) {
	if !read.Seen {
		return AgentDetection{}, false
	}
	if read.State != MuxIdle && read.State != MuxDone {
		return AgentDetection{}, false
	}
	return AgentDetection{State: read.State, MuxSeen: true}, true
}
