package posse

// The silence watchdog (ranger-base-wj7e9).
//
// On 2026-09-03 a `posse dispatch --watch` loop wrote its last line at
// 04:53:18Z and its next at 12:07Z. The cause was a laptop asleep with the
// lid closed for that whole window and NOT a hang — herdr.go's block says
// why the first reading got that wrong — so this file fixes no defect that
// gap proves, and the bead it was filed under was retracted.
//
// It fixes what the gap made plain instead. For seven hours a loop that was
// not running looked exactly like a loop that was, and no clock in this
// process could tell the difference or say so to anyone. The three that
// might have — the pulse (ADR 0027), the backup clock (ADR 0036 §4) and the
// guard clock (ranger-base-fxs60) — each already runs on a goroutine of its
// own, and none of them reads whether this loop is writing: they report on
// the shop, not on the process keeping it. The operator found the gap by
// reading the log himself, which is the only instrument that saw it.
//
// So there are two halves and this is the second. The first bounds every
// child, which makes a hang impossible to INHERIT from one. This one
// reports the silence itself, whatever its cause, because a deadline can
// only bound the failures we have already met.
//
// WHAT IT CANNOT SEE is a sleep, and that is deliberate but worth naming
// because this bead is where the question came up. LastWrite and d.now()
// both carry Go monotonic readings, so their Sub is a monotonic difference,
// and MEASURED on this box 2026-09-03 the darwin monotonic clock does not
// advance across a sleep: wall uptime 323192s against a runtime monotonic of
// 296514s, the 7h25m difference being the sleep itself. So this watchdog
// measures AWAKE silence. A frozen process did not stall — it was suspended
// — and reporting an hours-long stall on wake for a box that behaved would
// be a false alarm. Naming the wake is a DIFFERENT reading (wall elapsed
// minus monotonic elapsed) and a different line; this file is not it.
//
// WHY SILENCE AND NOT "NO PASS", as this file was built. "No pass in N
// intervals" was the reading the bead asked for and it was the wrong one, for
// the reason ADR 0028 §1 made true: under a rolling Run a pass legitimately
// lasted hours, so a pass boundary was not a heartbeat and a loop working
// perfectly would have tripped it all day. What a healthy loop DOES do,
// always, is write. A gather writes a check-in line per pending bead per wait
// leg; an idle loop writes its pass header and its "next pass in" line every
// interval. The log stopping is the observable the operator actually used to
// find this bead, and it is the one reading that catches a stall wherever it
// is — in a child, in a lock, in a goroutine nobody thought about.
//
// AND NOW BOTH (ranger-base-3ryit). That premise expired the day the gather
// became bounded: a pass is once again a thing that must come round on the
// interval, so "no pass in N intervals" is a legitimate reading and the one
// the 2026-09-04 incident needed. For 2h20m a loop held pass 1 open while
// settle-driven refills scrolled past — every one of them a WRITE, so the
// silence reading above was satisfied the whole time and correctly said
// nothing, while the sweep, the tickers, the plan read and every seat that
// freed without a settle simply did not run. Silence catches a loop that
// stopped; the pass clock catches a loop that is busy at half its duties. The
// two readings are independent and neither subsumes the other, so this
// watchdog takes both, off one tick.
//
// The pass reading says it ONCE per stall, where silence repeats. That is the
// bead's ask and it is the difference between the two: a stalled pass
// coexists with a log that is scrolling, so a line repeated every interval
// would be one more thing scrolling past, and the reader who comes back to
// the log wants the moment it started. It re-arms the moment a pass
// completes, so a loop that stalls, recovers and stalls again says so twice.
//
// THE BUDGET is built from the longest quiet a healthy loop can have, not
// from a number that sounded right. Two candidates:
//
//	one wait leg      PromptWaitMS (15m in production) plus the grace herdr
//	                  gets to write its timeout envelope — the gap between
//	                  two "still working after Nm" lines on a pending bead.
//	one pass interval the gap between two pass headers on an idle loop, at
//	                  the backed-off maximum rather than the base.
//
// The budget is WatchdogFactor times the larger. With production numbers
// that is 2 x 16m = 32m: a healthy leg reports at half the budget, and a
// loop that goes quiet while the box is AWAKE is named within one interval
// of the half hour rather than whenever somebody next reads the log. It
// would NOT have named the 09-03 gap — see WHAT IT CANNOT SEE above: the
// process was frozen for it, and the clock this reads skipped it too.
//
// It is LEVEL-TRIGGERED and it repeats — the same rule backupTick keeps, and
// for the same reason: a line printed once, hours ago, in a scrollback
// nobody is watching is the silence this bead was about. Each tick reprints
// with the number grown, so a reader coming back to the log sees an
// escalation rather than one stale sentence.
//
// It writes through quietf, which deliberately does NOT stamp LastWrite. A
// watchdog that reset its own clock would report a stall exactly once and
// then fall silent with the loop. The guard clock and the backup clock write
// the same way, for the reason under LastWrite (dispatch.go): a line from a
// clock on its own goroutine is not the loop writing, and one that stamped
// the reading kept this budget unreachable for as long as the box was over
// the load line (ranger-base-0fz98 finding 3).

import (
	"context"
	"fmt"
	"os"
	"time"
)

// WatchdogFactor is the multiple of the longest legitimate quiet that counts
// as a stall. Two, not one: a healthy wait leg reports at exactly one
// multiple, so a factor of one would call every ordinary leg a stall.
const WatchdogFactor = 2

// watchdogBudget is how long this loop may write nothing before the silence
// is a finding. See this file's head for both terms.
func watchdogBudget(maxInterval time.Duration, promptWaitMS int) time.Duration {
	quiet := maxInterval
	if leg := time.Duration(promptWaitMS)*time.Millisecond + HerdrWaitGrace; leg > quiet {
		quiet = leg
	}
	if quiet <= 0 {
		// No interval and no wait leg is a caller with no cadence at all;
		// fall back to the herdr control ceiling so the watchdog is armed
		// rather than silently disabled by a zero.
		quiet = HerdrControlTimeout
	}
	return WatchdogFactor * quiet
}

// watchdogPassBudget is how long the loop may go without COMPLETING a pass
// before that is a finding (ranger-base-3ryit).
//
// Built the same way the silence budget is: the longest a healthy pass can
// legitimately take, times WatchdogFactor. That length is now knowable, which
// is the whole reason this reading exists — a pass gathers for at most its
// window (`GatherWindow`) and then waits out at most one backed-off interval
// before the next one starts. With production's 3m/3m that is 2 x 6m = 12m,
// close under the ~15m the operator's standing mitigation used by hand
// against the log.
//
// `window` is the caller's GatherWindow, which Watch defaults to the base
// interval and otherwise leaves alone — so it is NOT interchangeable with
// base, and passing base here made the witness fire on a healthy pass for
// every caller that set a longer one (ranger-base-nzzuz finding 1).
func watchdogPassBudget(maxInterval, window time.Duration) time.Duration {
	quiet := maxInterval + window
	if quiet <= 0 {
		return 0
	}
	return WatchdogFactor * quiet
}

// watchdogLoop is the clock. It owns no state beyond the budgets; every tick
// is a fresh reading of LastWrite and of the last completed pass.
//
// It ticks at `every` (the loop's base interval) rather than at the budget,
// so the first report lands within one interval of the budget expiring and
// the repeats read as a cadence an operator can time.
func (d *Dispatcher) watchdogLoop(ctx context.Context, every, budget, passBudget time.Duration) {
	if every <= 0 || (budget <= 0 && passBudget <= 0) {
		return
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.watchdogTick(budget)
			d.passStallTick(passBudget)
		}
	}
}

// watchdogTick is one reading, printed only when the loop has been quiet
// past the budget.
func (d *Dispatcher) watchdogTick(budget time.Duration) {
	last := d.LastWrite()
	if last.IsZero() {
		return
	}
	silent := d.now().Sub(last)
	if silent < budget {
		return
	}
	d.quietf("%s\n", WatchdogLine(silent, budget, last, os.Getpid()))
}

// passStallTick is the second reading: the loop is writing, and the PASS has
// not come round (ranger-base-3ryit). Once per stall, re-armed by the next
// completed pass — see this file's head for why this one does not repeat.
func (d *Dispatcher) passStallTick(budget time.Duration) {
	if budget <= 0 {
		return
	}
	last := d.lastPassAt()
	if last.IsZero() {
		return
	}
	stalled := d.now().Sub(last)
	if stalled < budget {
		return
	}
	if !d.notePassStall() {
		return
	}
	d.quietf("%s\n", PassStallLine(stalled, budget, last, d.inFlightLine()))
}

// PassStallLine is the pass-clock finding, rendered. It names how long, the
// budget it broke, when the last pass ended, and — the part the operator
// reconstructed by hand on the night this was filed — the in-flight set that
// is holding it, so the next question ("which sessions?") is already answered
// in the line.
//
// It says what the stall is NOT: the loop is writing, or the silence watchdog
// above would have spoken first. So this is a loop that is refilling seats
// and running its clocks while the duties that live in the pass — the
// merge-back sweep, the hook wall, the backup ticker, the plan read, the
// epoch accounting, and any offer of ready work to a seat that freed with no
// settle behind it — are not running.
func PassStallLine(stalled, budget time.Duration, last time.Time, inFlight string) string {
	held := inFlight
	if held == "" {
		held = "nothing in flight — the pass is held by something else"
	}
	return fmt.Sprintf("◷ watchdog: no pass has completed for %s — past its %s budget "+
		"(last pass %s). The loop is still writing, so the sweep, the tickers and any seat that "+
		"freed without a settle are the part that is not running; in flight: %s (ranger-base-3ryit)",
		BlindFor(stalled.Round(time.Second)), BlindFor(budget), last.Format("15:04:05"), held)
}

// WatchdogLine is the finding, rendered. It names the silence, the budget it
// broke, when the last line was, and the pid — because the next question an
// operator asks is what that process is doing, and the answer is one command
// away only if the log carries the number.
//
// It also says what the silence is NOT. Every herdr child is bounded by its
// own declared timeout plus HerdrWaitGrace, or by HerdrControlTimeout, and
// every bd child by BdTimeout; a silence longer than all of those is
// therefore not a child exec waiting to return, which is the first thing
// anyone reading this line would otherwise go and check.
func WatchdogLine(silent, budget time.Duration, last time.Time, pid int) string {
	return fmt.Sprintf("◷ watchdog: this loop has written nothing for %s — past its %s budget "+
		"(last line %s). Every herdr and bd child is bounded, so this is the LOOP and not a child "+
		"exec waiting to return: sample %d (ranger-base-wj7e9)",
		BlindFor(silent.Round(time.Second)), BlindFor(budget), last.Format("15:04:05"), pid)
}
