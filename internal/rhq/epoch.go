package rhq

// The dispatch epoch — ADR 0028 §2.
//
// Before this file the accounting window was THE PASS: `budget_pass:`
// measured spend since `Run` started, and `-n`/`autostart_max_beads` bounded
// launch attempts inside one `Run`. Both readings were born with the pass and
// died with it, which was serviceable only while a pass was also the shop's
// clock. ADR 0028 §1 takes that away — the Run becomes long-lived and refills
// a seat the moment its bead settles — so a window denominated in passes
// stops denominating anything at all: one pass would be an evening, and every
// bound written per pass would silently become per evening.
//
// The epoch is that window, put back on a clock nothing in this process
// controls. It is WALL-CLOCK ALIGNED — local midnight plus whole epochs — and
// that is the property the ADR asks for by name: a Run that dies and restarts
// mid-epoch recomputes the same epoch start, so the spend the dead Run
// incurred is still inside the window the new one measures against. A window
// that opened at `time.Now()` would hand a fresh `budget_pass:` to every
// crash, which is spend authority created by a restart.
//
// Behaviour-neutral where the pass and the epoch coincide: with a `--watch`
// interval well under `dispatch_epoch:`, the first pass of an epoch reads
// exactly what a per-pass window read, and the passes after it read the spend
// their own epoch's earlier passes committed — which is the point.
//
// TWO BOUNDS, TWO DEGREES OF RESTART-PROOFNESS, and the difference is stated
// rather than papered over. Spend is restart-proof because it is not stored
// here at all: it is re-derived from the transcripts by `ScanCosts` against
// the recomputed epoch start (ADR 0011's rule — one fact, one store, and the
// transcripts are the store). The launch-attempt count has no such external
// store, so it lives in memory on the Dispatcher and a restart restores the
// full `-n`. That is the honest limit of this slice: the brake that bounds
// MONEY survives a restart, the one that bounds BLAST RADIUS bounds it per
// process. Making the attempt count durable means a new shared store for a
// number nothing else can re-derive, which is the "one more small store" ADR
// 0011 warns about, and it buys a bound that spend authority already covers.

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// DefaultDispatchEpoch is config `dispatch_epoch:` when unset.
//
// One hour, and ASSUMED rather than measured (ADR 0028 §2 says so in those
// words): it is the round wall-clock unit an operator reading `posse cost`
// already thinks in, and it is long enough that a `--watch` loop at the
// documented 5m interval fits ~12 passes inside one window — so the epoch
// really is an accounting window and not a pass by another name. It is a
// tuning decision, which is why it is a config key on its first day.
const DefaultDispatchEpoch = time.Hour

// DispatchEpoch reads config `dispatch_epoch:` as an interval ("1h", "30m",
// or bare seconds, the vocabulary `--watch` already takes). A value that is
// not a positive duration is NAMED and the default stands: a typo in the
// window that denominates both brakes must be visible, and it must not
// silently widen or narrow spend authority.
func (a *App) DispatchEpoch(errw io.Writer) time.Duration {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "dispatch_epoch"))
	if raw == "" {
		return DefaultDispatchEpoch
	}
	d, err := ParseInterval(raw)
	if err != nil || d <= 0 {
		fmt.Fprintf(errw, "dispatch: config dispatch_epoch: %q is not a positive interval — using %s\n", raw, DefaultDispatchEpoch)
		return DefaultDispatchEpoch
	}
	return d
}

// EpochStart is the opening instant of the wall-clock epoch containing now.
//
// Anchored at LOCAL MIDNIGHT — `startOfDay`, the same floor Dial E's day
// window already uses — plus whole epochs. Two consequences, both wanted:
// the epoch boundary is a time an operator can read off a clock (an hourly
// epoch turns at :00 in their own zone, not at :30 because of a UTC offset,
// which `time.Time.Truncate` would give in half-hour zones); and an epoch is
// never earlier than the day window's floor, so one transcript scan starting
// at midnight always feeds both windows.
//
// DST is not corrected for. The epochs of a day that gains or loses an hour
// stay `epoch` of ELAPSED time apart and the boundaries after the transition
// shift with it. Every reader of this — a spend total, a launch count — is a
// measure of elapsed time, and a window silently stretched to 2h of real
// spending to keep its label at :00 would be the wrong repair.
func EpochStart(now time.Time, epoch time.Duration) time.Time {
	if epoch <= 0 {
		epoch = DefaultDispatchEpoch
	}
	day := startOfDay(now)
	since := now.Sub(day)
	if since <= 0 {
		return day
	}
	return day.Add(since / epoch * epoch)
}

// rollEpoch points the dispatcher at the epoch that now falls in, and
// reports whether that is a NEW one. What the epoch denominates is reset
// exactly there: the launch attempts it has already spent.
//
// Called at the head of every pass. The malformed-config line is said once
// per PROCESS, not once per pass — a `--watch` loop must not write the same
// configuration fact into a log twelve times an hour (the rule blindWarned
// and planThreshWarned already keep).
func (d *Dispatcher) rollEpoch(now time.Time) bool {
	errw := d.errw()
	if d.epochWarned {
		errw = io.Discard
	}
	d.epochWarned = true
	start := EpochStart(now, d.App.DispatchEpoch(errw))
	if start.Equal(d.epochStart) {
		return false
	}
	d.epochStart = start
	d.epochAttempts = 0
	return true
}

// epochRoom is how many launch attempts `-n` leaves in this epoch, and
// whether there are any. max ≤ 0 is no cap and always has room.
//
// The exhausted case is a LINE and not a silent empty pass: a pass with
// ready work that launches nothing is exactly the silence ranger-base-69jo
// was filed about, and "the cap you set is spent until :00" is a fact the
// operator can act on — by raising `autostart_max_beads:`, by widening
// `dispatch_epoch:`, or by leaving it alone, which is the cap working.
func (d *Dispatcher) epochRoom(max int) (int, bool) {
	if max <= 0 {
		return max, true
	}
	room := max - d.epochAttempts
	if room > 0 {
		return room, true
	}
	d.printf("◷ launch cap: %d of %d attempt(s) spent this epoch — nothing launched until it turns at %s (ADR 0028 §2; raise autostart_max_beads:/-n or dispatch_epoch:)\n",
		d.epochAttempts, max, d.epochEnd().Local().Format("15:04:05"))
	return 0, false
}

// epochEnd is the current epoch's closing instant — what the line above
// quotes, and the only place the epoch's LENGTH is needed after the roll.
func (d *Dispatcher) epochEnd() time.Time {
	return d.epochStart.Add(d.App.DispatchEpoch(io.Discard))
}
