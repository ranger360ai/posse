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
// Across a darwin suspend the wall clock keeps time and the monotonic clock
// does not advance at all — MEASURED 2026-09-03 on this box at wall uptime
// 323192s against a runtime monotonic of 296514s, the 7h25m difference being
// that incident's sleep — so the gap between the two, taken over one tick,
// IS the suspend. Nothing else produces it: a tick delayed by a loaded box
// grows both gaps equally, because both readings are taken in the same call.
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
// WHAT IT DOES NOT DO. It does not report a BACKWARD step, which would make
// every wall-clock reading in the shop read fresher than it is; that is a
// real gap and a different line. It does not shorten the pass this loop is
// waiting out: the next-pass timer is monotonic too, so a wake lands with
// whatever was left of it still to run — 2m50s of a 3m interval on the night
// this was filed, against the 5h30m it explains, and ADR 0028 §1's rule that
// a late wake costs latency and never correctness. And it cannot name the
// suspend to a surface outside this process: `guard-blind` and `loop-mute`
// are built in govern.go from files, by any process that asks, and teaching
// the governance set to subtract a suspend is a change to the shop's own
// condition contract rather than to this loop's log.
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
	if slept < SuspendFloor {
		return
	}
	d.quietf("%s\n", SuspendLine(slept, monoGap, wallGap, wall))
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
