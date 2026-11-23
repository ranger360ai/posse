package posse

// A `working` reading over a CLOSED bead — ranger-base-4yefx, ask 1.
//
// THE INCIDENT. MEASURED 2026-11-23 by the coordinating persona on this
// instance: <persona>-<repo>-ranger-base-tvokd's bead closed 2026-11-21T09:22:47-05:00 and
// the seat was retired ~41.6h later, by hand, after `posse kill` refused it
// twice. Its pane held three claude background tasks and its tree held
// nothing. What held the lane for 41.6h was one line of this sweep —
// settledForReap's `return false // working, blocked: somebody is in there`
// — and the sweep said NOTHING about it on any pass. From the launcher's own
// log, over that window: 16 gather legs reading `working` (15m to the 4h
// WaitCeiling, which then stopped judging it), 252 land-sweep "workspace is
// alive" lines, 53 refill lines reporting that persona's lane busy, and zero
// lines of any kind naming a reap.
//
// WHY `working` WAS TRUE AND MEANT NOTHING. On this instance the state is the
// runtime's own OSC 7501 report (ADR 0076 §3, rexReading), and MEASURED
// 2026-11-23 on this box by one seat sampling its OWN block every 20s across
// its own turn boundary with a background task live — 36 samples, 22 in-turn
// then 14 post-turn:
//
//   - 36 of 36 root records read `working`. Not one `idle`, not one `done`.
//     A claude seat that ends its turn with ONE background task live reports
//     `working` with no end, which is the whole of tvokd's 41.6h.
//   - post-turn the root record's `msg` reads "1 shell still running" — the
//     runtime's own words for what the `working` is about — and its
//     `updated_at` freezes at the turn's end (one distinct value in all 14).
//   - the FOOTER tells the two apart cleanly and one-way: `esc to interrupt`
//     in 22 of 22 in-turn footers, 0 of 14 post-turn, switching once and
//     never flapping. All 36 carried `· 1 shell ·`.
//
// So the fact the sweep needed was on the screen the whole time, in the one
// line claude publishes about itself, and the vocabulary for reading it has
// been in panework.go since ranger-base-htafy. (capturedHold's doc records
// that on 2026-11-10 this same store read `done` after a turn ended, with no
// child record; today's build reads `working` and publishes a child record
// per task. The footer reading is the one of the four that did not move.)
//
// WHAT THIS ARM IS. The narrowest conjunction that makes "working" stop
// being an answer, and it adds exactly one reading to the evidence the sweep
// already demands:
//
//	class reapFleetClosed      dispatch's own per-bead session, no crew mark,
//	                           carrying the `bead:` pointer
//	the store of record        that bead reads CLOSED (reapWhy, read fresh)
//	the screen                 the footer names live background work AND does
//	                           not carry claude's interrupt hint, off ONE
//	                           hold taken at one instant
//	the composer               holds nothing unsent (somebody may be typing)
//	the records                residueHolds is empty — a kill takes nothing
//	a grace                    `reap_working_after:`, the dial below
//
// WHY NOT AT THE READING INSTEAD. The screen could have been used to call
// the seat settled, which would route it into the existing idle/done arm and
// fix this in one line. panework.go's standing rule refuses that, and it is
// right: "the reading is only ever allowed to say 'this settle is not one',
// never to invent one." A screen that can manufacture a settled state can
// close a bead and judge a turn nobody saw. Here nothing is judged — the
// bead is ALREADY closed by the store of record, there is no turn left to
// judge, and the only question is whether the workspace may go.
//
// WHAT IT IS WILLING TO BE WRONG ABOUT. claude drawing no interrupt hint
// during a turn. Then this reaps a live turn — the same consequence
// settledForReap's `""` arm already accepts on a thinner reading (detection
// blinking on a live agent, autoreap.go), over a bead the store calls closed
// and a tree that provably holds nothing. What it CANNOT be wrong about is
// the direction the guards fail in: an unreadable screen, a herdr that will
// not explain, a footer with no vocabulary in it and a capture that failed
// all answer the zero hold, whose Work is "" — and this arm needs Work
// non-empty, so every one of them keeps the seat exactly as today.
//
// AND IT SAYS ITSELF, WHICH IS HALF THE BEAD. Every pass it declines to act
// prints one line naming what it read and the command that ends it now —
// ranger-base-fytno and ranger-base-ktiik's rule for this sweep's other
// standing declines, and the rule the 41.6h silence broke.

import (
	"fmt"
	"time"
)

// DefaultWorkingReapAfter is the grace this arm waits past residueIdle. It
// is a POLICY DIAL and no number here was read off the fleet — the same
// footing as the two graces beside it in autoreap.go — and it is the
// unpointed arm's hour rather than the crew arm's four, on the unpointed
// arm's own argument.
//
// What the grace buys here is not doubt about the bead (the store of record
// has spoken) nor about the tree (residueHolds has). It is "not racing a
// seat somebody is about to use", which is exactly what
// DefaultUnpointedReapAfter is priced for. The crew grace is longer because
// it covers a CONVERSATION's gaps, and this arm is not that population: a
// crew-marked session reaches reapCrewClosed and pays four hours.
//
// It is measured against residueIdle — the later of the launch and the last
// work prompt — which is the only clock any record here carries, and the one
// the two arms beside it already use. On a dispatched bead that is the launch
// in almost every case: tvokd was launched at 08:24 and closed at 09:22, so
// an hour's grace was already spent before the bead closed and this arm
// would have taken it on the first sweep past the close.
const DefaultWorkingReapAfter = time.Hour

// workingSeatWorthAStoreRead is the one shape that pays for the `bd show`
// reapWhy opens with: the class and the state the arm below can act on. It is
// asked in the sweep, before that read, and it is deliberately the same two
// questions the arm asks again afterwards — the sweep's copy is about COST and
// the arm's is about correctness, and an arm that trusted its caller to have
// filtered would be one call away from acting on a population nobody checked.
func workingSeatWorthAStoreRead(s HerdrSession, class reapClass) bool {
	return class == reapFleetClosed && s.Status == "working"
}

// mayReapWorkingSeat is the arm: whether a session the occupant calls
// `working`, whose bead the store of record calls CLOSED, may be reaped
// anyway. It appends its own clause to `why` so the reap line says which
// evidence took the session, and it prints the standing line on every pass
// it declines.
//
// It is called from reapWhy, after the bead has been read closed and with
// that read not repeated: this is the one place in the sweep where "the work
// is finished" is already known, which is the whole of what makes the screen
// reading below spendable.
func (d *Dispatcher) mayReapWorkingSeat(s HerdrSession, class reapClass, pol reapPolicy, why *string) bool {
	// reapFleetClosed only. The other two classes rest on LESS evidence
	// about whether the work is finished — a crew mark the operator's own
	// hands put there, or no bead at all — and a screen reading is not the
	// thing to spend on a population the operator did not previously trust
	// the sweep to decide anything about (autoreap.go's own note on those
	// two arms). A crew-marked seat in this state is still the operator's,
	// and `posse kill` is still theirs to type.
	if class != reapFleetClosed {
		return false
	}
	// `working` ONLY, and the other two unsettled readings are deliberately
	// not this arm's (settledForReap names all three). `blocked` is a
	// persona waiting on its own dialog — the one unsettled state where
	// somebody is demonstrably needed, and where the screen posse would read
	// is the dialog rather than the footer under it. `""` is a CLI that may
	// still be starting, which has its own evidence and its own grace in
	// settledForReap. Both reach here and both are refused, so each keeps
	// exactly the behaviour it had.
	if s.Status != "working" {
		return false
	}
	m, ok := d.HB.readMeta(s.Name)
	if !ok {
		return false // no record is no age, and no age is not old enough
	}
	hold := d.HB.sessionHolding(s.Name)
	if hold.Work == "" {
		// Nothing was read, or what was read named no background work. Both
		// are "this sweep has no evidence about what `working` means here",
		// and the seat stays — but the line is printed either way, because
		// a bead the store calls closed over a seat nobody can release is
		// the condition that went 41.6h unsaid.
		// Said with no claim about WHY the reading was empty: on the
		// capture route a footer that named no background work and a
		// screen posse never read answer the same zero hold, and
		// capturedHold records its region only where the vocabulary
		// matched — so "the footer said nothing" and "there was no footer"
		// are not distinguishable here, and a line that implied either
		// would be inventing the difference.
		d.printf("◑ %s NOT reaped over %s: its agent reads `working` and posse cannot see what it is working on — `posse kill %s --no-land` takes it now\n",
			s.Name, *why, s.Name)
		return false
	}
	if hold.LiveTurn {
		// claude's own interrupt hint, in the same footer, at the same
		// instant: a turn IS in flight. The bead being closed does not make
		// that turn disposable — a persona closes its bead and then writes
		// its ORDERS.md lessons down.
		d.printf("◑ %s NOT reaped over %s: a turn is in flight (its footer carries claude's `%s` beside %s)\n",
			s.Name, *why, footerInterruptHint, hold.Work)
		return false
	}
	if hold.Typed != "" {
		d.printf("◑ %s NOT reaped over %s: its box holds %s\n", s.Name, *why, hold.Why())
		return false
	}
	// `off` disarms the KILL and not the saying, which is the one place this
	// arm's dial differs from the two beside it (where `off` puts the whole
	// population back outside the sweep and nothing is printed). The reason
	// is that the two halves of this bead are separable: the 41.6h cost one
	// lane and it cost it in SILENCE, and an operator who has disarmed the
	// kill — most likely because they were burned by a reading — is the
	// reader who most needs the line that says a closed bead's seat is still
	// holding one. It is cheap and bounded: it fires only for a per-bead
	// session whose bead the store calls CLOSED and which still reads
	// `working`, which is a condition that should be rare and is exactly
	// what wants naming when it is not.
	if pol.working <= 0 {
		d.printf("◑ %s NOT reaped over %s, holding %s with no turn in flight: the `reap_working_after` arm is off in config — `posse kill %s --no-land` takes it now\n",
			s.Name, *why, hold.Work, s.Name)
		return false
	}
	idle, dated := residueIdle(m)
	if !dated {
		d.printf("◑ %s NOT reaped over %s: nothing in its record says when it was last touched, so it can never be old enough for the `reap_working_after` grace — `posse kill %s --no-land` takes it now\n",
			s.Name, *why, s.Name)
		return false
	}
	if idle < pol.working {
		d.printf("◑ %s NOT reaped over %s, holding %s with no turn in flight: %s left of the `reap_working_after` grace — `posse kill %s --no-land` takes it now\n",
			s.Name, *why, hold.Work, (pol.working - idle).Round(time.Minute), s.Name)
		return false
	}
	if held := residueHolds(m); held != "" {
		d.printf("◑ %s NOT reaped over %s, holding %s with no turn in flight: %s\n",
			s.Name, *why, hold.Work, held)
		return false
	}
	*why += fmt.Sprintf(", holding %s with no turn in flight, idle %s", hold.Work, idle.Round(time.Minute))
	return true
}
