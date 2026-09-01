package posse

// A blind degrade runs from HEADROOM, never from a cap.
//
// ADR 0018 §1 licensed the degrade on Dial E being armed: "with one set the
// pass runs loudly under the ledger instead", because "Dial E computes from
// posse's own transcripts and needs no credential, so it works exactly when
// the plan guard cannot". True, and it measured the wrong thing.
//
// 2026-08-31 (ranger-base-c3vqe) measured what that floor is made of. The
// fleet's meter credential went stale at 23:09 and every read after it was a
// 401 or a 429 — nineteen hours blind. The instance's Dial E caps were
// armed, so every pass took the degraded arm and kept hiring. The
// operator caught it by hand at 96% of the weekly window with 4% left; the
// fleet's own file still read 89%, the last thing it had ever seen. Nothing
// in that day was a bug in the ledger: the dollars were counted correctly,
// under their caps, the whole time. The ledger counts SPEND. The thing at
// risk was the account's weekly window, which the ledger has never been able
// to see and does not know the ceiling of. A cap on one store is not a brake
// on another store's ceiling — "every incident in the class is one store's
// momentary reading taken as evidence about another store's durable fact"
// (ADR 0011's diagnosis, and this is that class again).
//
// So the licence has to come from the meter that went blind, and the only
// thing a blind meter still has to say is its last reading. That reading is
// a snapshot and is treated as one: it is never aged forward, never scaled
// by spend, never turned into an estimate of now — ADR 0018 rejected exactly
// that ("estimate the plan window while blind"), and this invents none of
// it. It is asked one question, about the past, that the past can answer:
//
//	when the meter went dark, was there room left to spend into?
//
// Two ways the answer is no, and both are numbers already in force on a
// sighted pass — neither is a new knob and neither needs a ratio:
//
//  1. The reading is over one of the operator's own `plan_guard_<window>:`
//     thresholds. A sighted pass would have SKIPPED on it (dispatch.go
//     planGuard's loop). Going blind is not a promotion from skipped to
//     running.
//  2. The reading is at or past Dial E's step-down rung. When the guard is
//     armed the plan windows join the ledger's tightest-window comparison
//     (budget.go resolve, examples/config.yaml: "the plan's 5h/7d
//     utilization joins the comparison as a third and fourth window"), so
//     that reading already had the ledger braking on it. Blindness drops
//     those windows out of resolve() silently, and a pass that was braking
//     becomes a pass that is not. The rung is the one that was in force.
//
// Under both, the last reading showed headroom, and ADR 0018 §1 stands
// unchanged: the ledger is a floor for spend and the pass degrades under it.
//
// A machine with NO reading at all is left on §1's arm too, deliberately.
// That is the 2026-08-26 outage's own shape — a credential posse could not
// read, from the first pass, on a fleet whose 17 PIDs are all on-meter — and
// parking it cost a measured hour of zero dispatch. No reading is no
// evidence of being near a ceiling; a reading in the braking band is
// evidence. This parks on evidence and not on ignorance.

import (
	"fmt"
	"io"
	"time"
)

// PlanBlindRefusal is why the meter's last reading refuses to license a
// degraded blind pass, as a clause to hang on the line that reports it, or
// "" when it licenses one.
//
// It makes no request and reads no credential: the snapshot is a file, and
// this is the same instance-wide record G5's blind clock is read from
// (govern.go blindPast), so a cockpit, a `posse status` and the watch loop
// all answer this the same way rather than one answer per process.
func (a *App) PlanBlindRefusal(caller string, now time.Time) string {
	last, at, ok := a.PlanCache(caller).LastReading()
	if !ok {
		return ""
	}
	// io.Discard: a malformed threshold is the guard's line to print, once,
	// on the pass that is using it — not a second copy from the brake.
	why := planHeadroomRefusal(a.PlanGuardThresholds(io.Discard), last)
	if why == "" {
		return ""
	}
	return fmt.Sprintf("%s, read %s ago — a dollar cap is not a brake on the plan window", why, BlindFor(now.Sub(at)))
}

// planHeadroomRefusal is the rule itself, over a reading and the thresholds
// in force: the window that refuses the degrade, or "" when the reading left
// room. Pure, and separate from the file read, so the rule can be pinned
// without a cache, a clock and a machine in a particular state.
//
// Thresholds first: over one of those, a sighted pass would have skipped,
// which is the stronger statement and the one the operator wrote down. In
// the adapter's reading order for both, matching planGuard — the adapter
// lists the window whose exhaustion hurts most first, and that is the one to
// name.
func planHeadroomRefusal(th map[string]float64, last PlanUsage) string {
	for _, w := range last {
		if t := th[w.Name]; t > 0 && w.Pct > t {
			return fmt.Sprintf("last reading %s at %.0f%% is over plan_guard_%s: %.0f%%", w.Name, w.Pct, w.Name, t)
		}
	}
	for _, w := range last {
		if w.Pct >= BudgetStepDownPct {
			return fmt.Sprintf("last reading %s at %.0f%% is past the %.0f%% braking rung", w.Name, w.Pct, BudgetStepDownPct)
		}
	}
	return ""
}
