package posse

// The guard clock (ranger-base-fxs60).
//
// The load guard is read at the top of a pass (dispatch.go, Run) and the
// orphan census rides inside the refusal it prints. Both of those are the
// PASS's, and under ADR 0028 §1's rolling Run the pass stopped recurring:
// Run does not return while a bead is in flight, a wait leg is fifteen
// minutes, and the ladder above it runs to WaitCeiling. MEASURED 2026-09-02:
// one loop ran 1h40m on ONE pass — nothing but pulse lines and "still
// working after 45m — waiting again" — while the box climbed to load 75-85
// and eight orphaned gate-shell children burned ~50% of a core each for 37
// minutes. The guard never evaluated, so arm 2 (loadguardkill.go) never ran,
// and the operator ended them by hand.
//
// (ranger-base-3ryit bounded the gather, so the pass recurs again — but this
// clock is not thereby redundant, and its two arms below say why: it ticks at
// the base interval where a quiet pass backs off to `--max-interval`, it
// evaluates while a pass is inside its own window, and the orphan census it
// carries runs on ticks the pass's reading would never have taken at all.)
//
// So the reading gets its own clock, on the same shape as the pulse (ADR
// 0027) and the backup clock (ADR 0036 §4) one file over: a goroutine that
// starts with the watch loop, ticks at the operator's `--interval`
// regardless of what the pass is doing, and is JOINED on the way out so
// nothing it started outlives the loop that claims to have ended.
//
// TWO THINGS ARE DELIBERATELY DIFFERENT FROM THE PASS'S READING.
//
//  1. It launches NOTHING. The pass's reading decides whether to dispatch;
//     this one only reports, and — where `load_guard_kill: true` — ends what
//     the orphan predicate names. A clock that could launch would be a
//     second dispatcher racing the first for the launcher lock, the busy map
//     and the epoch, and none of those are shaped for two writers.
//
//  2. THE ORPHAN CENSUS RUNS WHETHER OR NOT THE BOX IS OVER THE LINE.
//     Arm 1 has always ridden "only on a pass the guard is already skipping"
//     (NOTES.md), and the 09-02 addendum is what that costs: after the loop
//     was restarted the 1-minute load had dipped to 44, under `load_guard:
//     60`, so the pass was NOT skipped — and the same eight orphans, which
//     matched the predicate the whole time, went unreported and unkilled
//     again. Load is not the predicate. The leak is: ppid 1, over
//     LoadCulpritOrphanCPU, over LoadOrphanMinAge, and the ADR 0009 gate-shell
//     preamble at the head of its argv. A census that only looks when a
//     5-minute sample happens to catch the spike misses a fan that pushes
//     load to 85 between samples and dips under at the tick.
//
// Running it off the pass makes a concurrent census possible — this clock's
// and a pass's, or this clock's and the next tick's — and arm 2 is already
// safe for that by construction rather than by luck: sysReapOrphans re-reads
// every target's row immediately before it signals and skips a pid that is
// gone, recycled, no longer ppid 1, no longer ours, or declared in between
// (loadguardkill.go, "FAIL CLOSED ON THE KILL ITSELF"). The second census to
// reach a leak finds nothing to end and says so.

import (
	"context"
	"io"
	"time"
)

// guardLoop is the clock. It owns no state; every tick is a fresh reading.
func (d *Dispatcher) guardLoop(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.guardTick()
		}
	}
}

// guardTick is one reading, printed only when it has something to say. It
// goes through d.quietf because Run's gather is writing the same stream from
// one goroutine per pending bead (outMu's doc), and through d.quietErrWriter
// for the same reason: a reading LoadHigh could not take is a line, and a
// line half-interleaved with a launch is worse than the reading it explains.
// The QUIET pair, not printf: this clock reports on the shop, not on the
// loop keeping it, and a tick that stamped LastWrite fed the watchdog a
// fresh reading every base interval for as long as the box stayed over the
// line — the condition under which a stalled pass is most likely and least
// visible (ranger-base-0fz98 finding 3; watchdog.go's head).
func (d *Dispatcher) guardTick() {
	if line := d.App.GuardTickLine(d.quietErrWriter()); line != "" {
		d.quietf("%s\n", line)
	}
}

// GuardTickLine is one guard-clock reading rendered: the LoadHigh witness
// with the culprit line under it when the box is over the limit, the orphan
// report on its own when it is not, and "" when there is nothing true to say
// — a quiet box writes no line at all, which is what makes the ones it does
// write worth reading in a log that runs for days.
//
// Both branches take ONE census (loadCensus), so the top-CPU line and the
// orphan report under it are two renderings of a single `ps` rather than two
// reads that disagree.
func (a *App) GuardTickLine(errw io.Writer) string {
	why := a.LoadHigh(errw)
	busy := a.loadCensus()
	if why != "" {
		// Deliberately not the pass's sentence: this reading skipped no
		// pass and dispatched nothing, and a log line claiming a decision
		// nobody made is how a reader learns to distrust the log.
		return "◷ " + why + " — measured off the pass (guard clock); nothing new is launched while it stands" + a.culpritLineFrom(busy)
	}
	// Under the line, and the census still ran — see this file's head. The
	// report is "" on every box that is not holding one of ours.
	if report := a.orphanReport(busy); report != "" {
		return "◷ load guard: box under the line, orphan census ran anyway (the leak is the predicate, not the load)" + report
	}
	return ""
}
