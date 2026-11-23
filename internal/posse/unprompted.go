package posse

// G19 · a dispatched seat that never got its work prompt — ranger-base-p51zu.
//
// THE INCIDENT. MEASURED 2026-11-23 01:13-01:2xZ on the originating box (rex
// occupant, claude/standard, posse at 7dc3313e). A pass created a per-bead
// session `<persona>-<repo>-ranger-base-72z36`, its own worktree and its
// branch, and then the settle gate refused:
//
//	· ranger-base-72z36 creating session <persona>-<repo>-ranger-base-72z36 …
//	· ranger-base-72z36 own tree …/<persona>-<repo>-ranger-base-72z36 …
//	✗ ranger-base-72z36 herdr: no idle|done|blocked state in block:01a205f7-…
//	    within 16.446s (last reading: working) (timeout)
//	– ranger-base-72z36 <persona>-<repo>-ranger-base-72z36 did not take the
//	    launch — <persona> keeps its slot; the next bead gets a fresh session
//
// awaitAgent's error is raised BEFORE the claim and before the keystroke
// (dispatch.go launchSession), so nothing was typed and the bead was left
// unclaimed — correct, and the gate working. What was left behind is a live
// claude at its welcome banner, with an empty composer, a worktree and a
// branch, pointed at a bead nobody had told it about. The operator's
// delegate found it by LOOKING AT THE PANE and prompted it by hand.
//
// WHY NOTHING ELSE WAS EVER GOING TO SAY SO. The seat is outside every
// sweep and every reading there was:
//
//   - the reaper wants a bead the store calls CLOSED (autoreap.go), and this
//     bead was open — so no reap arm, narrow or widened, can see it;
//   - it does not hold the persona's slot either (personaActive's
//     in_progress-and-assigned test fails on an unclaimed bead), so the lane
//     lines never name it;
//   - `posse ls` and G1/G15 read a pane that is idle and undegraded, which
//     is what a seat that FINISHED looks like;
//   - the launch's own `✗` line is in state/dispatch-watch.log, once, in a
//     log the fleet writes some thousands of lines to a day.
//
// So the fleet had every fact and no reading: the seat's own run record said
// `bead: ranger-base-72z36` and carried no `prompted:` stamp at all, while
// the record of the seat launched 54 seconds after it carried both
// (state/herdr/*.yaml, both read at 01:21Z). That pair is the predicate.
//
// THE ROW (ADR 0029's bar for a row outside the table — predicate, owner,
// scope, class):
//
//	predicate  a live, non-foreign, non-crew session whose meta carries a
//	           `bead:` pointer, carries NO `prompted:` stamp, and whose
//	           `launched:` stamp is older than unpromptedGrace.
//	owner      the session's own run record (ADR 0011 §3) — the launcher
//	           writes both stamps and nothing else does.
//	scope      one seat. It names the session, the persona and the bead.
//	class      LANE. One bead is stopped and the rest of the shop flows,
//	           which is the split's own definition; the shop is not stopped.
//
// IT IS A FACT AND IT HEALS, which is what ADR 0029 asks of a condition
// rather than a bead. Three things clear it and all three are ordinary: the
// next pass prompts the seat it already resolves by name (launchSession's
// existing-session path), the operator prompts it by hand, or the seat is
// killed. Nothing is written down and nothing has to be closed.
//
// NO SUSPEND DISCOUNT, and that is a reading of ADR 0064 D1 rather than an
// omission of it. The grace below exists to exclude a launch that is still
// IN PROGRESS, and a launch cannot be in progress across a suspend — the
// process that was going to type is gone with it. So the awake/wall
// distinction cannot change this row's answer in the direction that matters:
// a seat whose prompt was in flight when the box went down did not get one.
//
// NO CONFIG KEY, for the same reason: the grace is not a policy about how
// long an operator should wait, it is the bound on how long a launch can
// still be mid-flight, and that bound is measured rather than chosen.
// unpromptedGrace states both floors it clears.

import (
	"fmt"
	"time"
)

// unpromptedGrace is how long after its launch line a bead-carrying seat may
// hold no `prompted:` stamp before that is a condition.
//
// It clears two floors and is the smaller of nothing:
//
//   - THE LAUNCH'S OWN WORST CASE. A typed launch spends the runtime's
//     patience up to three times before a key is pressed — awaitTarget, then
//     awaitSettled, then AwaitPersonaMode (dispatch.go awaitAgent) — and the
//     default is DefaultStartupWait, 45s. 135s, plus the work prompt's own
//     bd read. 15m is more than six times it.
//   - THE PASS THAT HEALS IT. A launch that failed is retried by the next
//     pass, which resolves the same per-bead session by name and prompts the
//     live seat. So the grace has to outlast more than one pass, or the row
//     beats the fleet's own repair to the operator — and the cadence is the
//     instance's own (`autostart_interval:`, no shipped default: the key IS
//     the arm switch). 15m is several passes at the cadences this shop is
//     run at, and the reading is on the bead rather than here (ADR 0024 D3,
//     ranger-base-p51zu). A seat that reds has had its chances and taken
//     none.
//
// MEASURED 2026-11-23 against the incident: the launch line was typed at
// 01:13:51.9Z and the seat still carried no stamp at 01:21Z, 7m later, with
// the next tick still ahead of it. A 15m horizon is a row a tick or two
// after the failure, not a race with the first one.
const unpromptedGrace = 15 * time.Minute

// unpromptedSeat is G19's row for one live session, and false when that
// session is not this population.
//
// The meta is read by the CALLER and handed in, so the walk pays one file
// read per candidate and this function reads nothing: it is the predicate
// alone, which is what makes it the same answer twice (ADR 0029's "facts get
// computed").
//
// A session with no readable record raises nothing. That is the one
// abstention here and it is the listing's own to report: `prompted:` absent
// and the whole record absent are not the same claim, and reading the second
// as the first would make every meta posse cannot parse a seat the operator
// has to go and look at (ranger-base-82e40's distinction, one surface over).
func unpromptedSeat(s HerdrSession, m *HerdrMeta, now time.Time) (GovCondition, bool) {
	switch {
	case s.Foreign, s.Crew:
		// Foreign: no record of ours to read. Crew: ADR 0008 — the
		// operator's own session does not exist as far as any sweep is
		// concerned, and an unprompted one is a conversation waiting for
		// them to type.
		return GovCondition{}, false
	case s.Bead == "":
		// Not a dispatched per-bead seat. `bead:` is written at exactly two
		// call sites, both in dispatch.go, plus NoteBeadFromPrompt on the
		// hand path — so its absence means nobody ever aimed this session at
		// a bead, and a seat that was never aimed is not a seat that missed
		// its prompt. The stampless population is the reaper's
		// (autoreap.go's widened arm), judged on different evidence.
		return GovCondition{}, false
	case m == nil || m.Unreadable:
		return GovCondition{}, false
	case !m.Prompted.IsZero():
		// It got one. Which launcher sent it does not matter — the record is
		// the cross-process half of exactly that question (ADR 0011 §3).
		return GovCondition{}, false
	case m.Launched.IsZero():
		// No clock. Abstaining rather than inventing one: a record with no
		// launch stamp cannot be aged, and "0 is a long time ago" would red
		// every such seat forever.
		return GovCondition{}, false
	}
	age := now.Sub(m.Launched)
	if age < unpromptedGrace {
		return GovCondition{}, false
	}
	return GovCondition{
		ID:    "G19",
		Class: GovLane,
		Key:   "unprompted:" + s.Name,
		Detail: fmt.Sprintf("%s (%s) was launched for %s %s ago and no work prompt has ever been delivered to it — its run record carries no prompted: stamp, so dispatch created the seat and gave up before the keystroke (its own ✗ line in the watch log says why); the next pass may still prompt it, or prompt it by hand (posse prompt %s) or end it (posse kill %s)",
			s.Name, s.Agent, s.Bead, BlindFor(age), s.Name, s.Name),
	}, true
}
