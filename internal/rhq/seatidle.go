package rhq

// Idle-to-next per seat — ADR 0028 §5 observable 1, the control arm.
//
// A seat is one (persona, repo): SessionFor's key, the unit ADR 0020 §2
// seats a bead into and the unit ADR 0028 §3 hands back at settle. Its
// IDLE-TO-NEXT is the wall-clock between a seat's bead settling and the
// next bead being prompted into that same seat. Today that window is the
// gather barrier plus the --watch sleep; ADR 0028 §1 claims to shrink it to
// seconds. A claim nobody measured before the change is unfalsifiable
// after it, so this ships FIRST and alone.
//
// It DECIDES NOTHING. No guard reads this ledger, no launch is refused on
// it, no cap counts it — it is a measurement, and the whole point of the
// slice is that the "before" number was taken by a dispatch that behaves
// exactly as it always did. That is also what keeps it from being the
// "one more small store" ADR 0011 warns about: a store that is never read
// back into a decision adds no way for two stores to disagree about a fact.
//
// WHY A LEDGER AND NOT THE SESSION META. The two facts come from where the
// bead says they do — the settle is herdr's (the state the gather's
// AgentWait returned), the launch is the same instant dispatch stamps
// `Prompted:` into the session meta — but neither store KEEPS the pair.
// Dial F gives every bead its own session, and the end-of-pass auto-reap
// deletes that session's meta once the bead closes (herdrback.go), which is
// exactly the moment before the seat's next launch. So the settle timestamp
// evaporates precisely when the next launch would want to subtract it. The
// ledger is the seat-scoped, reap-surviving half, on the same append-only
// shape as `overflow.log` and `uncounted.log` (overflow.go): O_APPEND from
// several launchers, one short line per event, never rotated by posse.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SeatCadenceLogPath is the ledger: `$StateDir/seat-cadence.log`,
// append-only, one line per seat event.
func (a *App) SeatCadenceLogPath() string {
	return filepath.Join(a.StateDir, "seat-cadence.log")
}

// Seat event kinds.
const (
	SeatSettle = "settle" // the seat's bead settled and the seat came free
	SeatLaunch = "launch" // a bead was prompted into the seat
)

// SeatEvent is one line: `RFC3339 kind seat bead detail`.
//
// Detail is the kind's own fifth column — the herdr state for a settle, the
// runtime for a launch — rather than two record types, because the readers
// below only ever need the first four fields and a fifth column keeps a
// line an operator greps readable without a parser.
type SeatEvent struct {
	At     time.Time
	Kind   string
	Seat   string
	Bead   string
	Detail string
}

func (e SeatEvent) line() string {
	return fmt.Sprintf("%s %s %s %s %s\n", e.At.UTC().Format(time.RFC3339), e.Kind, e.Seat, e.Bead, e.Detail)
}

// AppendSeatEvent records one seat event.
func (a *App) AppendSeatEvent(e SeatEvent) error {
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(a.SeatCadenceLogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(e.line()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// SeatFreeing says whether a settle in this herdr state actually handed the
// seat back. Only idle and done do: `personaActive` reads working AND
// BLOCKED as the persona being busy, so a bead that settles blocked keeps
// its claim, keeps its seat, and is waiting on a human — counting the wait
// for that human as dispatch latency would put the operator's response time
// in the harness's number.
func SeatFreeing(state string) bool { return state == "idle" || state == "done" }

// seatEvents reads the ledger back for one seat, oldest first. A missing
// ledger is no events, not an error: the first launch creates it. A line
// that does not parse is skipped — the file is ours to write, and a line
// nobody can date is not a measurement.
func seatEvents(path, seat string) ([]SeatEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []SeatEvent
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 || fields[2] != seat {
			continue
		}
		at, err := time.Parse(time.RFC3339, fields[0])
		if err != nil {
			continue
		}
		e := SeatEvent{At: at, Kind: fields[1], Seat: fields[2], Bead: fields[3]}
		if len(fields) > 4 {
			e.Detail = fields[4]
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// SeatRefill is one measured idle-to-next window: the seat, the bead whose
// settle freed it, the bead that took it back, and the gap.
//
// Unmeasured is not zero. Why is set — and Idle left alone — whenever the
// ledger cannot honestly subtract, and those cases are reported as what
// they are rather than folded into the figures as a fast refill.
type SeatRefill struct {
	Seat     string
	Bead     string // the bead this launch put into the seat
	After    string // the bead whose settle freed it ("" = none in the ledger)
	Settled  time.Time
	Launched time.Time
	Idle     time.Duration
	Why      string // "" = measured; else why there is no figure
}

// Measured says this refill carries a real number.
func (r SeatRefill) Measured() bool { return r.Why == "" }

// SeatIdleAt is the subtraction, for a launch into seat at time at.
//
// Two guards, both of which produce a NAMED non-figure rather than a wrong
// one. The seat's first launch since the ledger began has nothing to
// subtract from. And a seat whose newest event is a LAUNCH was refilled
// without its previous settle ever being recorded — a settle observed by
// nobody (the pass died mid-gather; the bead settled blocked and the
// operator freed it; the session was reaped between passes) — so the last
// settle on file is older than a whole bead and the gap across it is not
// this seat's idle window.
func SeatIdleAt(path, seat, bead string, at time.Time) SeatRefill {
	r := SeatRefill{Seat: seat, Bead: bead, Launched: at}
	events, err := seatEvents(path, seat)
	if err != nil {
		r.Why = fmt.Sprintf("seat ledger unreadable (%v)", err)
		return r
	}
	newest := -1
	for i := range events {
		if events[i].Kind == SeatSettle {
			newest = i
		}
	}
	if newest < 0 {
		r.Why = "first launch into this seat on record — no settle to measure from"
		return r
	}
	// FILE ORDER, not timestamps, decides whether that settle is the seat's
	// newest event. The ledger is append-only and one seat is one persona in
	// one repo, so its lines are already in the order the events happened —
	// while the timestamps are second-granularity, and a bead that settles
	// inside the second it launched in (a stranded pane, a refused turn)
	// would tie and read as unordered.
	if last := events[len(events)-1]; last.Kind != SeatSettle {
		r.Why = fmt.Sprintf("previous settle not observed (last event is %s's %s) — no honest window", last.Bead, last.Kind)
		return r
	}
	settle := events[newest]
	r.After, r.Settled = settle.Bead, settle.At
	r.Idle = at.Sub(settle.At)
	if r.Idle < 0 {
		// Clocks, not seats: an event stamped in the future is a machine
		// problem and a negative idle would poison every aggregate taken
		// off this log.
		r.Idle = 0
		r.Why = fmt.Sprintf("settle at %s is after this launch — clock skew, not idle", settle.At.UTC().Format(time.RFC3339))
	}
	return r
}

// Line is observable 1's account line for one seat. It carries the raw
// material and not only the difference — both endpoints and both beads —
// because the aggregate that answers the ADR is taken over an evening of
// these, and a summary nobody can re-derive is not a baseline.
func (r SeatRefill) Line() string {
	if !r.Measured() {
		return fmt.Sprintf("◷ idle-to-next %-14s —       %s: %s", r.Seat, r.Bead, r.Why)
	}
	return fmt.Sprintf("◷ idle-to-next %-14s %s  (%s settled %s → %s launched %s) [ADR 0028 §5 obs.1 baseline]",
		r.Seat, r.Idle.Round(time.Second),
		r.After, r.Settled.Local().Format("15:04:05"),
		r.Bead, r.Launched.Local().Format("15:04:05"))
}

// ─── the pass's half (ADR 0028 §5 observable 1) ──────────────────────────────

// noteSeatLaunch measures the window this launch closes and records the
// launch that opens the next one.
//
// The measurement is taken BEFORE the append, or the launch it just wrote
// would be the "previous settle not observed" case for itself. Both are
// skipped under --dry-run: a dry pass launched nothing, so it freed no seat
// and refilled none, and writing either half would leave the next real
// launch subtracting from a window that never happened.
func (d *Dispatcher) noteSeatLaunch(is RepoIssue, seat, runtime string, at time.Time) {
	if d.DryRun {
		return
	}
	path := d.App.SeatCadenceLogPath()
	d.seatRefills = append(d.seatRefills, SeatIdleAt(path, seat, is.ID, at))
	if err := d.App.AppendSeatEvent(SeatEvent{At: at, Kind: SeatLaunch, Seat: seat, Bead: is.ID, Detail: runtime}); err != nil {
		d.eprintf("seat cadence: launch of %s into %s not recorded (%v) — the next refill of this seat has no window to measure (ADR 0028 §5)\n", is.ID, seat, err)
	}
}

// noteSeatSettle records that a seat's bead settled. A settle that did not
// free the seat is recorded WITH ITS STATE and not silently dropped: a
// blocked seat is a real thing that happened to it, and a reader who cannot
// see the blocked settles cannot tell a seat nobody refilled from a seat
// waiting on a human.
//
// A settle whose state does not free the seat is written and then ignored
// by SeatIdleAt, which reads only the newest settle of any kind — so a
// blocked settle that is later freed by hand fails into the "previous
// settle not observed" branch rather than charging the operator's response
// time to dispatch.
func (d *Dispatcher) noteSeatSettle(p *pendingBead, state string, at time.Time) {
	if d.DryRun || !SeatFreeing(state) {
		return
	}
	seat := SessionFor(p.persona, p.is.Dir)
	if err := d.App.AppendSeatEvent(SeatEvent{At: at, Kind: SeatSettle, Seat: seat, Bead: p.is.ID, Detail: state}); err != nil {
		d.eprintf("seat cadence: settle of %s in %s not recorded (%v) — this seat's next refill has no window to measure (ADR 0028 §5)\n", p.is.ID, seat, err)
	}
}

// seatIdleReport is observable 1's obligation: every pass that refilled a
// seat says how long that seat sat empty first.
//
// A pass that refilled nothing prints nothing — there is no window to
// report and a standing line meaning "fine" is the noise ADR 0013 §5's own
// report rule already refused. The unmeasured refills ARE printed: the
// count of windows this instrument could not close is how anyone judges
// whether the baseline is worth anything.
func (d *Dispatcher) seatIdleReport() {
	if len(d.seatRefills) == 0 {
		return
	}
	refills := append([]SeatRefill(nil), d.seatRefills...)
	sort.SliceStable(refills, func(i, j int) bool { return refills[i].Seat < refills[j].Seat })
	measured := make([]time.Duration, 0, len(refills))
	for _, r := range refills {
		d.println(r.Line())
		if r.Measured() {
			measured = append(measured, r.Idle)
		}
	}
	if len(measured) == 0 {
		return
	}
	sort.Slice(measured, func(i, j int) bool { return measured[i] < measured[j] })
	d.printf("◷ idle-to-next: %d of %d refill(s) measured — median %s, max %s (ADR 0028 §5 observable 1; no refill has shipped, this is the control arm)\n",
		len(measured), len(refills), medianDuration(measured).Round(time.Second), measured[len(measured)-1].Round(time.Second))
}

// medianDuration is the middle of a SORTED slice, the low middle of an even
// one. The median rather than the mean because one seat waiting on a
// 75-minute bead is the distribution this measures, not an outlier to
// average away.
func medianDuration(sorted []time.Duration) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[(len(sorted)-1)/2]
}
