package posse

// The suspend witness (ranger-base-sqxo1).
//
// A box that was asleep and a loop that hung produce the same evidence, and
// this shop has now read the first as the second twice.
//
// 2026-09-03, ranger-base-wj7e9: a `dispatch --watch` loop wrote its last
// line at 04:53:18Z and its next at 12:07Z. Filed as a 7h11m hang, with a
// herdr child at etime 07:21:37 to prove it. The laptop had been asleep with
// the lid closed for the whole window; the bead was retracted on that ground
// (watchhang_qa_test.go's head keeps the retraction).
//
// 2026-10-01, this bead: the same shape, a different reason for the sleep,
// and the same first reading. Pass 6 of watch pid 60923 printed
// "1 dispatched · next pass in 3m0s" and the log then held nothing but two
// pulse rounds for what looked like five and three quarter hours — no pass
// 7, no hires, no landings — while three seats stood "working". It was filed
// as a hang in the prompt-gathering wait, and the asks were a bound on the
// gather and a watchdog that repeats.
//
// MEASURED 2026-10-02, this box, three independent witnesses agreeing to the
// second:
//
//	pmset -g log   18:38:49 "Entering Sleep state due to 'Low Power Sleep':
//	               TCPKeepAlive=inactive Using Batt (Charge:1%) 19807 secs",
//	               after BatteryHealth warning level 3 at 18:34:01 and the
//	               display going off at 18:38:44; then 00:08:56 "Wake from
//	               Hibernate [CDNVA] : due to lid/UserActivity … Using AC
//	               (Charge:100%)", hibmode=3. 19807s is 5h30m07s.
//	plan-usage.log the pulse's own reads stop at 2026-10-01T22:34:55Z and
//	               resume at 2026-10-02T04:08:57Z — 18:34:55 and 00:08:57
//	               local, one second after the wake.
//	the same log   the COCKPIT's reads, a different process entirely, stop
//	               at 22:35:01Z and resume at 04:08:58Z. Two processes do
//	               not stall together; a box does.
//
// So nothing hung. The gather was already bounded (passcarry.go,
// ranger-base-3ryit) and pass 6 completed about four seconds before the
// sleep; the "hours of pulses" were two rounds in the seventy-four seconds
// between the wake and the operator's TERM, read as hours because pulse
// lines carry no timestamp. The one line that was a true finding — the
// pass-stall witness at 18:37:44, "no pass has completed for 14m" — was
// about a pass that really did take 12m30s on a CPU throttled to a 1%
// battery, and it completed.
//
// WHAT WAS MISSING BOTH TIMES is this reading. watchdog.go names it and
// declines it in the same breath — "Naming the wake is a DIFFERENT reading
// (wall elapsed minus monotonic elapsed) and a different line; this file is
// not it" — and that was the right call for one incident and the wrong one
// for two. Every clock in this process measures AWAKE time and therefore
// says nothing (watchdog.go's "WHAT IT CANNOT SEE"), and every clock OUTSIDE
// it measures wall time and therefore says something true and misleading:
// `guard-blind:5h` because the plan reading was 5h30m old, `loop-mute`
// (G7, URGENT since ranger-base-n00wn) because the watch log was 5h30m
// untouched, and a watch-status line reporting a log written five hours ago.
// All three were correct. None of them could say why, and the operator who
// read them was handed a muted shop with no cause in it.
//
// THE READING. time.Now() carries both a wall reading and a monotonic one.
// Across a darwin suspend the wall clock keeps time and the monotonic reading
// does not advance at all, so the gap between the two, taken over one tick,
// IS the suspend. Nothing else produces it: a tick delayed by a loaded box
// grows both gaps equally, because both readings are taken in the same call.
//
// MEASURED 2026-10-03 03:50Z on this box, and this is the number the whole
// file rests on, so it is worth having exactly:
//
//	wall uptime        2509051.5s   (now - sysctl kern.boottime)
//	CLOCK_MONOTONIC    2509051.2s   tracks wall — it INCLUDES suspend on darwin
//	CLOCK_UPTIME_RAW   1352850.5s   == mach_absolute_time, excludes suspend
//
// 1156201s between the last two: thirteen days and nine hours of suspend in a
// twenty-nine-day uptime. And it is mach_absolute_time that Go reads —
// runtime.nanotime1 for GOOS=darwin is the mach_absolute_time trampoline
// (go1.26.5, src/runtime/sys_darwin.go:304, with the numer/denom conversion
// under it), NOT clock_gettime, so a Go monotonic difference on this platform
// is awake time and the middle row above is a trap rather than an
// alternative. The 2026-09-03 reading in watchdog.go's head measured the same
// thing less directly (wall uptime 323192s against a runtime monotonic of
// 296514s) and agrees.
//
// THE WALL GAP IS COMPUTED FROM UnixNano AND NOT FROM Time.Sub, and that is
// the whole correctness of this file. Sub prefers the monotonic reading
// whenever both operands carry one, so `wall.Sub(prev)` is identically the
// monotonic gap and the difference it is subtracted from is always zero — the
// defect this reading exists to end, written in one keystroke and invisible
// in every test, because there is no way to synthesize a Time whose wall and
// monotonic readings disagree (Add moves both, and only a real suspend
// separates them). `Round(0)` would strip the reading and would be just as
// unkillable by a pin. UnixNano has no monotonic variant at all, so the class
// is structurally impossible here rather than guarded by a line no fixture
// can hold.
//
// THE FLOOR is for clock STEPS, not for skew: the two readings are taken a
// nanosecond apart, so the only other thing that moves them apart is the wall
// clock being SET — ntpd, or an operator — and a one-second correction is not
// a suspend. A suspend worth a line is minutes.
//
// WHAT IT DOES NOT DO. It does not shorten the pass this loop is waiting
// out: the next-pass timer is monotonic too, so a wake lands with whatever
// was left of it still to run — 2m50s of a 3m interval on the night this was
// filed, against the 5h30m it explains, and ADR 0028 §1's rule that a late
// wake costs latency and never correctness.
//
// WHAT IT DOES NOW THAT IT DID NOT (ADR 0064, ranger-base-m154u). Two
// clauses above were written as declines and have been re-ruled, both for
// the same reason: the reading existed and the surfaces that needed it could
// not reach it.
//
//   - It could not name the suspend to a surface outside this goroutine —
//     "`guard-blind` and `loop-mute` are built in govern.go from files, by
//     any process that asks, and teaching the governance set to subtract a
//     suspend is a change to the shop's own condition contract rather than
//     to this loop's log". That change is now made, deliberately and in one
//     place: ADR 0064 D1 rules that a wall-clock shop condition is a
//     statement about AWAKE time, so this file keeps a small ledger of the
//     suspends it witnessed and answers suspendedSince for a window. The
//     condition set subtracts it in the WATCH process only, through
//     GovInputs.Suspended (D2) — `posse status` and the cockpit are separate
//     processes with no witness and keep the wall reading (D4).
//   - It was silent on a BACKWARD step — "that is a real gap and a different
//     line". It is that line now (D6), and nothing more: a backward step is
//     REPORTED and never corrected, and never enters the ledger. A forward
//     gap leaves a hole in wall time that no timestamp falls into, so
//     overlap-with-a-window is exact; a backward step makes two frames
//     overlap for its own length, and a timestamp inside the overlap cannot
//     be told pre-step from post-step. The hazard is bounded by the step and
//     self-heals as the overlap passes. Zero backward steps in the record;
//     the first line this prints is the measurement that reopens the clause.
//
// It writes through quietf, like every other clock here and for the reason
// under LastWrite (dispatch.go): a line from a goroutine of its own is not
// the loop writing, and one that stamped the silence clock would hand the
// watchdog a fresh reading at every tick.

import (
	"fmt"
	"time"
)

// monoEpoch is one monotonic reading, taken when this process starts.
// time.Since of it is a monotonic difference — how long this process has been
// AWAKE — because both operands carry a monotonic reading and Sub prefers it.
var monoEpoch = time.Now()

func monoSince() time.Duration { return time.Since(monoEpoch) }

// SuspendFloor is the smallest wall-against-monotonic gap this calls a
// suspend. See THE FLOOR above: it is sized for a clock being SET, which is
// the only other thing that separates the two readings, and not for skew
// between them.
const SuspendFloor = 30 * time.Second

// SuspendTick is how often the pair is read. It is not the loop's interval
// and deliberately shorter than it: the readings this line exists to explain
// are taken by OTHER processes the moment the box comes back — the pulse's
// own plan read landed one second after the 2026-10-01 wake — so the window
// in which the log holds a five-hour gap and no reason for it is this
// number, and on the armed loop the base interval would make it 3m.
const SuspendTick = 30 * time.Second

// suspendLedgerCap bounds the ledger of witnessed suspends (ADR 0064 D2):
// past it the OLDEST pair is dropped, which is the one no window can still
// reach.
//
// Sized from the rate, MEASURED 2026-10-03 on this box over `pmset -g log`'s
// whole retention (2026-09-26 → 10-02, six days): 1127 sleeps, 751 of them at
// or over SuspendFloor, 319 over ten minutes — so about 125 pairs a day at
// this box's duty cycle, and the witness cannot record faster than one per
// SuspendTick because two sleeps inside one tick arrive as one gap. Against
// that, the LONGEST window anything asks about is `WatchLogStaleAfter` at its
// cap (85m at this box's arm) or `plan_guard_blind_max` (10m default), both
// far under a day. 256 is therefore about two days of pairs and twenty times
// the longest window's worth, at 32 bytes each.
//
//	pmset -g log | grep -c 'Entering Sleep'
//	pmset -g log | grep 'Entering Sleep' | grep -oE '[0-9]+ secs' | awk '$1>=30' | wc -l
const suspendLedgerCap = 256

// suspendSpan is one witnessed suspend: the box came back at back having
// been gone for slept, so the hole it left in wall time is the interval
// [back-slept, back]. Two spans cannot overlap — a span's start is
// the wall reading of the tick BEFORE the one that recorded it, which is at
// or after the previous span's back — so summing overlaps double-counts
// nothing.
type suspendSpan struct {
	back  time.Time
	slept time.Duration
}

// suspendEvery is the witness's cadence: SuspendTick, or the caller's own
// interval when that is shorter, so a fixture running a 20ms loop is not
// waiting half a minute for a reading.
func suspendEvery(every time.Duration) time.Duration {
	if every <= 0 || every > SuspendTick {
		return SuspendTick
	}
	return every
}

// clocks is the witness's one reading of both clocks, from d.Clocks or the
// real pair.
func (d *Dispatcher) clocks() (time.Time, time.Duration) {
	if d.Clocks != nil {
		return d.Clocks()
	}
	return time.Now(), monoSince()
}

// noteClock seeds the witness without reporting anything, so its first tick
// measures from the loop's start rather than skipping a suspend that lands in
// the first SuspendTick. Watch calls it beside noteWrite and notePass, which
// seed the other two readings about this loop for the same reason.
func (d *Dispatcher) noteClock() {
	wall, mono := d.clocks()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.clockWall, d.clockMono = wall, mono
}

// suspendTick is one reading of the pair, printed only when the two have come
// apart by more than the floor.
//
// Under mu even though only this goroutine touches the two fields today: they
// are state about the loop kept beside lastPass, read the way lastPass is,
// and a reader on another goroutine — the pulse, if the governance half of
// this is ever built — must not be the thing that discovers the lock was
// missing.
func (d *Dispatcher) suspendTick() {
	wall, mono := d.clocks()
	d.mu.Lock()
	prevWall, prevMono := d.clockWall, d.clockMono
	d.clockWall, d.clockMono = wall, mono
	d.mu.Unlock()
	if prevWall.IsZero() {
		// Unseeded: this tick is the seed. Reachable only for a caller that
		// starts the witness without noteClock.
		return
	}
	// UnixNano, not Sub — see this file's head. This is the one subtraction
	// in the package that must be a WALL difference, and Sub cannot give one.
	wallGap := time.Duration(wall.UnixNano() - prevWall.UnixNano())
	monoGap := mono - prevMono
	slept := wallGap - monoGap
	if slept <= -SuspendFloor {
		// The same reading with the sign flipped: the wall clock was SET
		// BACK by this much (ADR 0064 D6). Reported, never corrected, and
		// never written to the ledger — see this file's head for why the
		// correction is not computable. -slept is exactly the step, because
		// a clock set back by S over an interval in which mono really moved
		// m leaves wallGap = m - S.
		d.quietf("%s\n", ClockSetBackLine(-slept, monoGap, wallGap, wall))
		return
	}
	if slept < SuspendFloor {
		return
	}
	// The ledger before the line, so a reader of the pair never sees a
	// SUSPENDED line the window sum does not know about.
	d.mu.Lock()
	d.suspends = append(d.suspends, suspendSpan{back: wall, slept: slept})
	if n := len(d.suspends); n > suspendLedgerCap {
		// Overlapping copy in place, so the backing array stays the cap's
		// size rather than creeping forward one header at a time.
		d.suspends = append(d.suspends[:0], d.suspends[n-suspendLedgerCap:]...)
	}
	d.mu.Unlock()
	d.quietf("%s\n", SuspendLine(slept, monoGap, wallGap, wall))
}

// suspendedSince is GovInputs.Suspended's answer (ADR 0064 D2): how much of
// the window from since to now THIS process witnessed the box suspended,
// summed over the ledger as each span's overlap with [since, now].
//
// Two properties the caller leans on. The result never exceeds now-since,
// because every overlap is clipped to both ends — so a caller subtracting it
// from an age it measured over the same window cannot go negative. And it is
// exact rather than an estimate: a forward gap is wall time no timestamp
// falls into, and the spans are disjoint (see suspendSpan).
//
// now comes from the witness's own pair seam and never from d.now(), for the
// reason the whole file reads that seam: the dispatcher's clock is aged by
// hours and run at 10000x by the fixtures in this package, and the ledger is
// written in the pair's frame. The clip against now is close to inert either
// way — a span's back is the wall reading of a tick that has already
// happened — and it is here so the sum is one frame end to end.
//
// A zero since is no window rather than all of time, and answers zero: the
// caller with no timestamp to measure from (an absent file) has no age to
// subtract from either.
func (d *Dispatcher) suspendedSince(since time.Time) time.Duration {
	if since.IsZero() {
		return 0
	}
	now, _ := d.clocks()
	// UnixNano, not Sub — see this file's head, and here for a sharper
	// reason than there. This function mixes two kinds of instant: the
	// LEDGER's came from time.Now() through the pair seam and carry a
	// monotonic reading, while the WINDOW's come from outside this process
	// (a file mtime) and do not. So `now.Sub(mtime)` is a true wall
	// difference and reads like a licence, while a Sub between two ledger
	// instants, or between the ledger and now, is a MONOTONIC difference
	// that excludes every suspend after the span being measured — the
	// answer comes back short by the next sleep, on the box that sleeps. No
	// fixture can see it (a hand-driven clock's Times carry no monotonic
	// reading at all), so it is pinned at the source.
	sinceNS, nowNS := since.UnixNano(), now.UnixNano()
	d.mu.Lock()
	defer d.mu.Unlock()
	var total time.Duration
	for _, sp := range d.suspends {
		backNS := sp.back.UnixNano()
		lo, hi := backNS-int64(sp.slept), backNS
		if sinceNS > lo {
			lo = sinceNS
		}
		if nowNS < hi {
			hi = nowNS
		}
		if hi > lo {
			total += time.Duration(hi - lo)
		}
	}
	return total
}

// SuspendLine is the finding, rendered. It names how long the box was gone,
// when it came back, and the pair of readings the number is a difference of
// — because the next thing anyone does with this line is check it against
// `pmset -g log`, and the two readings are what make it checkable.
//
// Then it says what the line is FOR, in the words of the three readings it
// disarms. A reader arriving at a five-hour hole in this log has `loop-mute`
// in a pulse, `guard-blind:5h` beside it and a watch-status line reporting a
// log written hours ago, all three correct and none of them the cause; the
// whole value of this line is that it is in the log between them.
func SuspendLine(slept, mono, wall time.Duration, back time.Time) string {
	return fmt.Sprintf("◷ the box was SUSPENDED for %s and this loop was frozen with it — back at %s "+
		"(%s of monotonic across %s of wall). Every wall-clock reading taken now names that window and not a "+
		"fault: `loop-mute`, `guard-blind` and a watch log whose last line is hours old are the suspend. This "+
		"loop's own clocks did not advance either, so its next pass is still due on the interval it was "+
		"waiting out (ranger-base-sqxo1)",
		BlindFor(slept.Round(time.Second)), back.Format("15:04:05"),
		BlindFor(mono.Round(time.Second)), BlindFor(wall.Round(time.Second)))
}

// ClockSetBackLine is the backward half of the same reading, and the whole of
// what ADR 0064 D6 decided to do about it: say it, once, and correct nothing.
//
// It is the inverse of SuspendLine's three readings. A forward gap makes
// every wall-clock reading in the shop name a window and not a fault; a
// backward step makes every one of them read FRESHER than it is by the step,
// so `loop-mute` and `guard-blind` go quiet over a real outage for that long
// and the shop says nothing is wrong when something may be. The line says
// which way it went and for how long, because that is the whole of what is
// knowable: inside the overlap the step created, two wall frames are live at
// once and a timestamp in it cannot be told pre-step from post-step, so there
// is no correction to apply (this file's head; D6's rejected alternative).
//
// The step is in the shop's own units. The pair it is a difference of is
// rendered as signed Durations rather than through BlindFor, which floors at
// zero: across a backward step the wall gap is usually negative and may be
// positive (a step of 35s inside a 45s tick leaves it at +10s), while the
// monotonic gap never is — and a pair printed as "0s" would be the
// one number on the line that is not checkable.
func ClockSetBackLine(step, mono, wall time.Duration, at time.Time) string {
	return fmt.Sprintf("◷ the wall clock was SET BACK by %s at %s — every wall-clock reading in this shop now names a "+
		"time %s FRESHER than it is, and will until that much wall time has passed: `loop-mute` and `guard-blind` "+
		"are quiet over that window whether or not anything is wrong. NOTHING corrects it — inside the overlap a "+
		"step makes, a timestamp cannot be told pre-step from post-step, so the step is reportable and the "+
		"correction is not computable (ADR 0064 D6). The box was not suspended; this is a clock being set "+
		"(%s of monotonic across %s of wall)",
		BlindFor(step.Round(time.Second)), at.Format("15:04:05"), BlindFor(step.Round(time.Second)),
		mono.Round(time.Second), wall.Round(time.Second))
}
