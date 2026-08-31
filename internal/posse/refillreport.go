package posse

// The refill's own report — ADR 0028 §1's fire path, said so an operator can
// tell it from a pass (ranger-base-59jd).
//
// MEASURED 2026-08-28 ~09:15, on the first live refill, under a watch
// narrowed to one developer with `--persona`: every settle re-runs the WHOLE
// fire path now (§1 as amended — the settle is the level-triggered tick), and
// the fire path enumerates per bead. What reached the log was a wall of
// `– <bead> … lane busy` lines followed by `– 131 ready bead(s) outside
// <that persona>'s lane — skipped by --persona`, repeated at every settle and
// attributed to nothing. The operator read it as a rogue persona-filtered
// loop holding the watch and went to an alarm footing. Every line was true;
// none of them said who was speaking, and under a rolling Run they are the
// same lines every time because the queue is the same queue.
//
// So a refill NAMES the seat whose settle it is refilling for before it
// enumerates, and its skips — the per-bead `– ` lines — are counted by
// reason and reported in ONE line after it. Nothing else is quieted:
// launches, `✗` errors, and every other report the pass makes print exactly
// as they always did, and a fire pass that is not a refill (the head of a
// Run, every one-shot `posse dispatch`, every --dry-run pass that never
// refills) still prints every skip per bead, unchanged.

import (
	"fmt"
	"sort"
	"strings"
)

// Skip reasons, as they are counted in a refill's summary line. Short
// because they are counted rather than read one at a time — the per-bead
// line still says the whole thing outside a refill, and `posse dispatch
// --dry-run` prints that enumeration on demand.
const (
	skipBadID      = "bad bead id"
	skipQuestion   = "for the operator"
	skipUnroutable = "unroutable"
	skipLaneBusy   = "lane busy"
	skipCrewHeld   = "held by a crew session"
	skipOrphaned   = "orphaned claim, assignee's crew session live"
	skipForeign    = "held by another posse"
	skipSettled    = "held, agent settled"
	skipGrace      = "inside another launcher's prompt grace"
	skipBudget     = "budget window spent"
	skipPlanGuard  = "plan guard"
	skipRuntimeCap = "runtime cap"
	skipSessFail   = "session failure"
	skipBenched    = "slot benched"
)

// outsideLaneSkip is the `--persona` filter's own reason: not a per-bead
// line even outside a refill (fireLoop already summarises it), so it is
// folded in with a count rather than one at a time.
func outsideLaneSkip(persona string) string { return "outside " + persona + "'s lane" }

// refillFor is the settle a fire pass is refilling for, and the skips that
// fire pass counted instead of printing.
//
// Written and read on ONE goroutine: refire is called from Run's own gather
// loop, and fireLoop — the only thing that counts into this — runs on the
// caller's goroutine under the launcher flock. The gather goroutines beside
// it print through d.printf, which has its own mutex, and never touch this.
type refillFor struct {
	seat    string // the seat whose settle triggered this refill
	bead    string // the bead that settled and freed it
	skipped map[string]int
	order   []string // reasons in first-seen order, to break count ties stably
}

func newRefillFor(seat, bead string) *refillFor {
	return &refillFor{seat: seat, bead: bead, skipped: map[string]int{}}
}

func (r *refillFor) note(kind string, n int) {
	if n <= 0 {
		return
	}
	if _, seen := r.skipped[kind]; !seen {
		r.order = append(r.order, kind)
	}
	r.skipped[kind] += n
}

// reasons is the summary's tail: how many beads were not taken, and why,
// commonest first with first-seen breaking ties — so the line reads as what
// mostly happened, then the rest.
func (r *refillFor) reasons() (int, string) {
	total := 0
	for _, n := range r.skipped {
		total += n
	}
	at := make(map[string]int, len(r.order))
	kinds := append([]string(nil), r.order...)
	for i, k := range r.order {
		at[k] = i
	}
	sort.SliceStable(kinds, func(i, j int) bool {
		if r.skipped[kinds[i]] != r.skipped[kinds[j]] {
			return r.skipped[kinds[i]] > r.skipped[kinds[j]]
		}
		return at[kinds[i]] < at[kinds[j]]
	})
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%d %s", r.skipped[k], k))
	}
	return total, strings.Join(parts, ", ")
}

// beginRefill opens the block: the header names the seat, the settle that
// freed it, and how much queue this refill is about to walk. Printed before
// the enumeration so everything under it is visibly one refill's, which is
// the whole point — the lines were never wrong, they were unattributed.
func (d *Dispatcher) beginRefill(seat, bead string, ready int) {
	d.refilling = newRefillFor(seat, bead)
	d.printf("↻ refill for settled seat %s (%s settled) — re-offering %d ready bead(s) to every free seat\n", seat, bead, ready)
}

// endRefill closes it and is what actually reports the skips. Always paired
// with beginRefill, including on the error path: a refill that failed still
// counted whatever it walked, and leaving d.refilling set would silence the
// next ordinary fire pass.
func (d *Dispatcher) endRefill(fired int) {
	r := d.refilling
	d.refilling = nil
	if r == nil {
		return
	}
	total, why := r.reasons()
	if total == 0 {
		d.printf("↻ refill for settled seat %s: %d launched\n", r.seat, fired)
		return
	}
	d.printf("↻ refill for settled seat %s: %d launched, %d skipped (%s)\n", r.seat, fired, total, why)
}

// skipf reports one bead this fire pass did not take: the line outside a
// refill, a count under `kind` inside one.
func (d *Dispatcher) skipf(kind, format string, args ...any) {
	d.skipNf(kind, 1, format, args...)
}

// skipNf is skipf for a reason that already speaks for several beads — the
// `--persona` filter's own tail, which is one line for N beads either way.
func (d *Dispatcher) skipNf(kind string, n int, format string, args ...any) {
	if r := d.refilling; r != nil {
		r.note(kind, n)
		return
	}
	d.printf(format, args...)
}
