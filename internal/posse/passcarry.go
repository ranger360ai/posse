package posse

// The in-flight set, carried across passes (ranger-base-3ryit).
//
// MEASURED 2026-09-04 01:51Z: a `dispatch --watch` loop printed
// "── pass 1", launched four sessions, printed "4 prompt(s) in flight,
// gathering" — and printed no pass summary for 2h20m. The loop was healthy
// the whole time: the pulse ticked, settles were detected, and settle-driven
// refills launched three more sessions. Only the PASS was gone, and with it
// the merge-back sweep, the hook-wall check, the backup ticker, the plan
// read, the epoch accounting, and — the operator-visible one — any offer of
// ready work to a seat that became free with no settle to hang a refill on. A
// P1 the operator asked for by name sat ready and unhired for an hour with
// its persona's seat empty.
//
// The cause is arithmetic, not a hang. ADR 0028 §1 made `Run` long-lived and
// let every settle refill the seat it freed; the gather loop counts legs
// still outstanding and exits at zero. Each refill launches, and each launch
// joins the set it is waiting on — so on a shop busy enough to keep refilling,
// the set is fed faster than it drains and the count never reaches zero. A
// pass whose duties are time-based was gated on session-shaped waits, and
// those waits are 15m a leg with a ladder to WaitCeiling above them.
//
// THE FIX is the bead's first shape: the gather is bounded per pass. A pass
// judges what has landed inside its window, then RETURNS with the rest still
// in flight — carried, not abandoned. The next pass takes the carried set
// back at the head of its own gather, and the wait goroutines never notice:
// one goroutine per pending bead, all of them fanning into ONE channel with
// the life of the loop rather than one per pass. Nothing is judged twice,
// nothing is dropped, and the pass is a clock again.
//
// WHAT THE CARRY COSTS. Between the window closing and the next pass, a leg
// that settles is nobody's: its result sits in the channel until the next
// pass reads it, so its seat is refilled up to one interval later than it
// would have been. That is exactly the latency ADR 0016's settle hint already
// exists to remove — a settle wakes the loop out of its backoff — and ADR
// 0028 §1's own framing applies: a lost or coalesced hint costs latency,
// never correctness, because the next pass re-verifies against bd and herdr
// before it acts on anything.
//
// WHAT MUST HAVE THE SAME LIFETIME AS THE SET. The busy map (ADR 0028 §3) is
// the seats this Run fired into, released at the settle that judges them; a
// carried leg's seat is still occupied, so the map is carried with it, and so
// is the per-slot session-failure count that ADR 0013 §2's ceiling is
// denominated in. They were the Run's locals when the Run was the loop; they
// are the LOOP's now, and a one-shot Run (no Refill, no window, gathers to
// zero exactly as it always did) still gets a fresh pair per call.

import (
	"fmt"
	"strings"
	"time"
)

// gathered is one settled leg, fanned in from its own wait goroutine. It
// carries the pendingBead itself so the judging side can strike it from the
// in-flight set — the count is what a pass's window is measured against and
// what the watchdog names when a pass stops coming round.
type gathered struct {
	p       *pendingBead
	is      RepoIssue
	persona string
	working bool
	err     error
}

// DefaultGatherWindow bounds a pass's gather when the caller sets none. Watch
// sets its own (the loop's base interval, watch.go); this is for a Refill Run
// driven by anything else, so that "rolling" can never again mean "unbounded".
const DefaultGatherWindow = 3 * time.Minute

// carriedDrainBudget bounds the join at the end of a stopping loop. Every
// wait goroutine returns as soon as `stopped()` closes unless it is inside a
// herdr child, and every herdr child is bounded by HerdrControlTimeout plus
// the grace herdr gets to write its envelope — so a join that outlives this
// is not a leg finishing late, it is a leg that will not finish at all, and
// the loop says so and leaves rather than holding the operator's ctrl-c.
var carriedDrainBudget = HerdrControlTimeout + HerdrWaitGrace

// seatState hands the pass the two maps whose lifetime is the in-flight set's
// (see this file's head). A one-shot Run drains its gather to zero before it
// returns, so it takes a fresh pair and nothing survives the call; Watch's
// rolling Run takes the loop's own, because a carried leg's seat is still
// occupied and a seat released by a pass boundary would be a second bead on a
// persona that already has one (ADR 0028 §5 observable 4).
func (d *Dispatcher) seatState() (map[string]string, map[string]int) {
	if !d.Refill {
		return map[string]string{}, map[string]int{}
	}
	if d.busySeats == nil {
		d.busySeats, d.seatFail = map[string]string{}, map[string]int{}
	}
	return d.busySeats, d.seatFail
}

// enqueue puts freshly fired prompts in flight: one wait goroutine each,
// fanning into the loop's own results channel.
//
// The channel is made once and outlives every pass, which is what lets a leg
// fired by pass 1 be judged by pass 2. Its buffer is a courtesy, not a
// contract: a send that finds it full blocks the wait goroutine — which has
// nothing left to do but exit — until the next pass reads, and no result is
// ever dropped. Called only from the pass's own goroutine (Run's head and the
// judging below), which is what makes the slice safe to append to under the
// same lock the watchdog reads it through.
func (d *Dispatcher) enqueue(pending []*pendingBead) {
	if len(pending) == 0 {
		return
	}
	d.mu.Lock()
	d.openFanin()
	results, wake := d.results, d.wake
	d.inflight = append(d.inflight, pending...)
	d.mu.Unlock()
	for _, p := range pending {
		go func(p *pendingBead) {
			working, err := d.gather(p)
			results <- gathered{p, p.is, p.persona, working, err}
			// The poke, after the send so the pass that wakes finds the
			// result already in hand (settled). Coalesced and never
			// blocking: a poke nobody has taken yet is a poke that has not
			// been acted on, which is the rule watch.go's herdr-hint
			// refresh keeps.
			select {
			case wake <- struct{}{}:
			default:
			}
		}(p)
	}
}

// openFanin makes the loop's two channels. Called under mu.
func (d *Dispatcher) openFanin() {
	if d.results == nil {
		d.results = make(chan gathered, 32)
		d.wake = make(chan struct{}, 1)
	}
}

// settled is the in-process settle hint: a leg CARRIED past the end of a
// pass landed, and the loop should take its next pass now rather than wait
// out the backoff (watch.go's timer select reads it).
//
// It is the same trigger ADR 0016 §1 ratified and the same one ADR 0028 §1
// calls the refill's first, arriving over a channel this process owns
// instead of over herdr's event socket — which matters because the carry's
// one cost is exactly this latency: a leg that settles after its pass's
// window closed is judged, and its seat refilled, by the NEXT pass. With
// this the next pass is immediate whether or not herdr's events are up; the
// backoff tick stays the backstop it always was.
func (d *Dispatcher) settled() <-chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.openFanin()
	return d.wake
}

// forget strikes a judged leg from the in-flight set.
func (d *Dispatcher) forget(p *pendingBead) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, q := range d.inflight {
		if q == p {
			d.inflight = append(d.inflight[:i], d.inflight[i+1:]...)
			return
		}
	}
}

// inFlightCount is how many prompts this loop is still waiting on, carried
// legs included.
func (d *Dispatcher) inFlightCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.inflight)
}

// inFlightLine names the set, oldest first, for the two lines that report it:
// the carry line a pass writes when it returns with legs outstanding, and the
// watchdog's finding when passes stop coming round. Bead, session and how
// long that prompt has been in flight — the three facts the operator went
// looking for by hand on the night this bead was filed.
//
// Capped at four, because eleven seats is eleven of these and a witness
// nobody can read is the failure this bead is about.
func (d *Dispatcher) inFlightLine() string {
	d.mu.Lock()
	set := append([]*pendingBead(nil), d.inflight...)
	d.mu.Unlock()
	if len(set) == 0 {
		return ""
	}
	for i := 1; i < len(set); i++ {
		for j := i; j > 0 && set[j].prompted.Before(set[j-1].prompted); j-- {
			set[j], set[j-1] = set[j-1], set[j]
		}
	}
	var parts []string
	for i, p := range set {
		if i == 4 {
			parts = append(parts, fmt.Sprintf("+%d more", len(set)-i))
			break
		}
		parts = append(parts, fmt.Sprintf("%s in %s %s", p.is.ID, p.session,
			BlindFor(time.Since(p.prompted).Round(time.Second))))
	}
	return strings.Join(parts, ", ")
}

// gatherRound is the pass's half of the gather: judge every leg that lands
// inside this pass's window, then return with whatever is still outstanding
// left in flight for the next pass.
//
// The window is Watch's (the loop's base interval). A one-shot Run has none
// and drains to zero, which is what `posse dispatch` without --watch has
// always done and must keep doing: there is no next pass to carry anything
// into.
//
// When the window closes, everything ALREADY landed is judged before the
// pass leaves — the results are in hand, judging them costs no wait, and
// leaving them for the next pass would hold their seats over the backoff for
// nothing. Only legs still genuinely outstanding are carried.
func (d *Dispatcher) gatherRound(personaFilter, dirFilter string, max int, busy map[string]string, sessFail map[string]int) (judged, stillWorking int) {
	var window <-chan time.Time
	if d.Refill {
		w := d.GatherWindow
		if w <= 0 {
			w = DefaultGatherWindow
		}
		t := time.NewTimer(w)
		defer t.Stop()
		window = t.C
	}
	for d.inFlightCount() > 0 {
		select {
		case g := <-d.results:
			j, w := d.judge(g, personaFilter, dirFilter, max, busy, sessFail)
			judged, stillWorking = judged+j, stillWorking+w
		case <-window:
			j, w := d.judgeLanded(personaFilter, dirFilter, max, busy, sessFail)
			return judged + j, stillWorking + w
		}
	}
	return judged, stillWorking
}

// judgeLanded judges the results already in hand and returns without waiting
// for any that are not. Bounded by construction: each judgement may refill,
// and a refill's own launches settle later, so nothing it starts can land
// back in this loop.
func (d *Dispatcher) judgeLanded(personaFilter, dirFilter string, max int, busy map[string]string, sessFail map[string]int) (judged, stillWorking int) {
	for {
		select {
		case g := <-d.results:
			j, w := d.judge(g, personaFilter, dirFilter, max, busy, sessFail)
			judged, stillWorking = judged+j, stillWorking+w
		default:
			return judged, stillWorking
		}
	}
}

// judge is one settled leg: counted, swept after, and — under a rolling Run —
// the seat it freed offered work again. Unchanged from the loop body it was
// cut out of; only the in-flight bookkeeping around it is new.
func (d *Dispatcher) judge(g gathered, personaFilter, dirFilter string, max int, busy map[string]string, sessFail map[string]int) (judged, working int) {
	d.forget(g.p)
	if g.err != nil {
		d.printf("✗ %-14s %v\n", g.is.ID, g.err)
	} else {
		if g.working {
			working = 1
		}
		judged = 1
	}
	if !d.Refill {
		return judged, working
	}
	// The sweep, on the event that MAKES a session sweepable
	// (ranger-base-t8tq). Both of the other call sites — before routing, and
	// the epilogue — fire once per Run, which was once per pass until ADR
	// 0028 §1 made this Run long-lived and then became once per PROCESS.
	// MEASURED 2026-08-28: one pass ran 7h09m and 22 done-sessions piled up
	// behind it, every one of them swept in the first seconds after the loop
	// was bounced. A settle is the moment a per-bead session stops being
	// anybody's, so it is where the sweep belongs — including for a bead
	// still working (its seat frees nothing, but the graveyard behind it is
	// not about this bead). The sweep reads every bead fresh and swallows its
	// own read failures (autoreap.go), so a settle it cannot sweep after
	// costs nothing but the next settle's sweep.
	d.autoReapPass(afterRouting)
	if g.working {
		return judged, working
	}
	if d.stopping() {
		// The loop is stopping: the seat this settle just freed is still
		// recorded free (below), but nothing fires into it.
		return judged, working
	}
	seat := SessionFor(g.persona, g.is.Dir)
	delete(busy, seat)
	// The refill runs the fire path for every free seat, not only for the one
	// that just settled (ranger-base-t8tq). ADR 0028 §1 as accepted said
	// "re-runs the fire path for the freed seat", on "the level-triggered tick
	// still sweeps everything, so a lost event costs latency, never
	// correctness" — but under S4 the tick is Watch's, and Watch did not get
	// its loop back until the Run returned. Since ranger-base-3ryit it does,
	// every interval, so the two triggers now BOTH hold: this settle sweeps
	// every free seat immediately, and the pass that comes round behind it
	// sweeps them again from a fresh reading. personaFilter is still the
	// operator's --persona, the busy map still refuses a seat with a bead on
	// it, and each seat is re-read live (seatMap), so this fires no seat a
	// fresh pass would not have fired.
	more, attempts, err := d.refire(seat, g.is.ID, personaFilter, dirFilter, max, busy, sessFail)
	if err != nil {
		d.printf("✗ refill %s: %v\n", g.persona, err)
	} else if !d.DryRun {
		d.epochAttempts += attempts
	}
	d.enqueue(more)
	return judged, working
}

// reportGather is the pass's two closing lines about what it was waiting on:
// how many beads it left with their agent (unchanged), and what it is
// carrying into the next pass.
//
// The carry line is this bead's first ask made visible. A pass that returns
// with prompts outstanding is now the ordinary case, and the log has to say
// so or the operator is back to reading a scroll of refills and inferring the
// shop from them. One line, with the set named (inFlightLine), every pass
// that carries anything.
func (d *Dispatcher) reportGather(stillWorking int) {
	if stillWorking > 0 {
		d.printf("◷ %d bead(s) still with their agent — claims kept; a later pass sees them held, not free\n", stillWorking)
	}
	if n := d.inFlightCount(); n > 0 {
		d.printf("… %d prompt(s) still in flight, carried into the next pass: %s\n", n, d.inFlightLine())
	}
}

// drainCarried joins the legs a stopping loop is carrying.
//
// Before the carry, every fired prompt was joined by the Run that fired it —
// the gather loop counted down to zero — so a drain (ranger-base-e9d9) always
// left with its abandoned legs' verdicts already printed: "claim kept, not
// judged this pass", the line that tells the next loop the bead is held and
// not free. A pass that returns with legs outstanding has nobody left to join
// them, so Watch joins them here, on its way out: each wait goroutine takes
// gather's stop exit the instant the context closes, prints that same line,
// and reports. Nothing is judged and nothing is refilled — the loop is over.
//
// Bounded (carriedDrainBudget) because a join with no bound is the shape this
// whole bead is about. What it gives up is the verdict LINE for a leg stuck
// inside a herdr child past its own timeout; the claim is kept either way,
// because keeping it is what NOT unclaiming means.
func (d *Dispatcher) drainCarried() {
	if d.inFlightCount() == 0 {
		return
	}
	deadline := time.NewTimer(carriedDrainBudget)
	defer deadline.Stop()
	for d.inFlightCount() > 0 {
		select {
		case g := <-d.results:
			d.forget(g.p)
		case <-deadline.C:
			d.printf("◷ %d prompt(s) still in flight at the stop — claims kept, not judged (%s)\n",
				d.inFlightCount(), d.inFlightLine())
			return
		}
	}
}

// notePass stamps a completed pass. The watchdog's second reading is the gap
// between two of these (watchdog.go); it is also what re-arms the stall
// witness, so a loop that comes back reports again if it stalls again.
func (d *Dispatcher) notePass() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastPass = d.now()
	d.passStallSaid = false
}

// lastPassAt is the reading, for the watchdog's goroutine.
func (d *Dispatcher) lastPassAt() time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastPass
}

// notePassStall arms the once-per-stall rule and reports whether this caller
// is the one that gets to speak.
func (d *Dispatcher) notePassStall() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.passStallSaid {
		return false
	}
	d.passStallSaid = true
	return true
}
