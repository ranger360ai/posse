package posse

// The backoff hold — ranger-base-5tkz9.
//
// `dispatch --watch`'s backoff is a reading about THE QUEUE: a pass that
// dispatched nothing found nothing to do, so the next one can wait longer
// (watch.go, NextInterval). A pass that was BLOCKED is not that reading at
// all. The launch cap being spent, or a pause being in force, means the pass
// found work and declined to take it — and the backoff read that decline as
// an empty shop and waited the whole ladder out over it.
//
// MEASURED 2026-11-13 on the host, twice in one day:
//
//	pass 13, 15:59:32  "launch cap: 6 of 6 spent this epoch, nothing
//	                    launched until it turns at 16:00:00", then
//	                    "0 dispatched, next pass in 40m0s". The epoch
//	                    turned 28 seconds later with three P1 beads ready
//	                    (b0qxo, 9v2xo, qfhph) and the next pass was not due
//	                    until 16:39. The coordinator restarted the unit at
//	                    16:2x to force one.
//	pass 11, 17:53     the same shape against the 18:00 turn; the shop idled
//	                    until the unit was restarted at 18:07.
//
// So the cap, which exists to bound how fast the shop hires inside one
// epoch, was idling it for most of the NEXT one — and it does that every
// epoch the cap binds, which is what made it a P1.
//
// TWO ARMS, and the difference between them is whether the block's end is
// knowable from here.
//
//   - THE LADDER DOES NOT CLIMB on a blocked pass, whatever the block. The
//     ladder's input is "was there anything to do", and a blocked pass has
//     no opinion on that: it is the launcher's own refusal. Holding is the
//     whole of what a block with no knowable end gets — a pause is lifted by
//     a human typing `posse resume` and nothing here can say when that is.
//   - THE SLEEP IS CUT TO THE LIFT when the block names one. The launch cap
//     does: it is spent until the epoch turns, and the turn is a wall-clock
//     instant this process already computes (epoch.go, epochEnd). So the
//     next pass is scheduled just after the turn rather than at the end of a
//     ladder that has nothing to do with it.
//
// The sleep is cut below `base` where the turn is nearer than that, and that
// is deliberate: there is ready work the cap refused, and the first instant
// it can be taken is the turn. It costs at most one extra pass per turn the
// cap binds, and only on an epoch that bound it.
//
// WHAT THE LADDER ARM COSTS, said plainly because it is the one arm that
// buys latency with passes. Under the cap it is close to free: the pass that
// spent the cap launched something, so the ladder is already at `base` when
// the next one is refused. Under a PAUSE it is the only arm there is, and it
// holds the loop at whatever its ladder was — at `base`, that is a pass every
// interval for the length of the pause instead of one every ceiling, and a
// pass under a pause is not free: it reaps, lands, runs the guard readings,
// files verify beads and makes ci-watch's `gh` call (2.8-4.2s). That is the
// trade the bead asked for in those words — "a pass that dispatched 0
// because of the cap (or a pause) should not grow the idle backoff" — and it
// is the right one on the shop's own terms, because a pause keeps oversight
// running on purpose (ADR 0029 §3) and the thing it was costing was a resume
// nobody noticed for forty minutes. Anyone reversing it should reverse the
// PAUSE arm and keep the cap's.
//
// The LOAD GUARD is deliberately not a block here. It returns from the whole
// pass for the same "nothing launched" reading, but a saturated box is a
// reason to come back LATER rather than sooner, and its end is not knowable
// either — so it keeps the ordinary ladder it has always had.

import (
	"fmt"
	"time"
)

// BlockLiftMargin is how far PAST a block's lift the next pass is scheduled.
//
// It is there so the pass that is meant to see the block lifted actually
// does. Scheduled exactly at the turn, a pass whose timer fires a
// nanosecond early — a monotonic timer against a wall-clock epoch, a pass
// that starts a hair before it reads the clock — recomputes the SAME epoch,
// stays blocked, and then waits out the held ladder for real. One second
// is far inside the resolution of anything this schedule is read at (the
// incident above was about forty minutes) and far outside any clock
// difference between a timer and a `time.Now`.
const BlockLiftMargin = time.Second

// passBlock is why a pass launched nothing while it had something to launch,
// and when that reason lifts. The zero value is "not blocked", which is
// every ordinary pass.
//
// Pass-denominated like planTrip and budgetStopped beside it: Run clears it
// at its head and whatever the pass finds it sets. Written and read on the
// pass goroutine only — Run is called synchronously from Watch, and the one
// other writer (refire, from the gather's drain loop) is on that goroutine
// too — so it needs no lock, exactly as its neighbours do not.
type passBlock struct {
	// why names the block in the operator's words — "the launch cap",
	// "the pause". Empty means no block, and is the whole of the test.
	why string
	// until is the instant the block lifts, in the wall-clock frame the
	// schedule is kept in. Zero means it is not knowable from here, which
	// is a pause: `posse resume` is a human, not a clock.
	until time.Time
}

func (b passBlock) on() bool { return b.why != "" }

// noteBlocked records that this pass declined to launch for a reason that
// is not an empty queue. until is zero when the lift is not knowable.
//
// Last writer wins, and the writers cannot disagree about anything that
// matters: the fire pass and any refill behind it in the same pass read the
// same cap against the same epoch.
func (d *Dispatcher) noteBlocked(why string, until time.Time) {
	d.blocked = passBlock{why: why, until: until}
}

// hold applies the block to one round of the backoff. grown is what
// NextInterval made of prev, and now is the clock the lift is measured
// against.
//
// It returns the ladder to carry into the next pass and the wait to actually
// sleep, which are the same thing on an unblocked pass. The sleep never
// exceeds the ladder: a lift already in the past cuts the wait to the
// margin, it never extends one.
func (b passBlock) hold(grown, prev time.Duration, now time.Time) (ladder, sleep time.Duration) {
	if !b.on() {
		return grown, grown
	}
	ladder = grown
	if prev > 0 && ladder > prev {
		ladder = prev
	}
	sleep = ladder
	if !b.until.IsZero() {
		lift := b.until.Sub(now) + BlockLiftMargin
		if lift < BlockLiftMargin {
			lift = BlockLiftMargin
		}
		if lift < sleep {
			sleep = lift
		}
	}
	return ladder, sleep
}

// clause is what the cadence line says about the hold, and it says something
// only when the hold CHANGED the schedule. A block that moved neither the
// ladder nor the sleep — a pause landing on a loop already at its ceiling —
// gets no clause, because there is nothing about the schedule to explain.
//
// The block itself has already printed its own line this pass (the cap's
// exhaustion line, the pause's decline line), so this is the clause that
// ties that line to the number an operator actually reads off the log.
func (b passBlock) clause(grown, ladder, sleep time.Duration) string {
	switch {
	case !b.on():
		return ""
	case sleep < ladder:
		return fmt.Sprintf(" — held to %s, when %s lifts, rather than backed off to %s (ranger-base-5tkz9)",
			b.until.Local().Format("15:04:05"), b.why, grown.Round(time.Second))
	case ladder < grown:
		return fmt.Sprintf(" — %s is in force, which is not a quiet queue, so the backoff is held at %s rather than grown to %s (ranger-base-5tkz9)",
			b.why, ladder.Round(time.Second), grown.Round(time.Second))
	}
	return ""
}
