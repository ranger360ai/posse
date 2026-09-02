package posse

// The shop pulse (ranger-base-dwlb1, operator ruling 2026-09-02) — the bead
// numbers that mean something, in place of the raw open count.
//
// The raw count was the reading every surface printed, and it measures the
// wrong thing twice over. It is dominated by discovered debt from reviews
// the operator himself commissioned, so it grows when the shop is working
// WELL: on the day of the ruling the crew closed 86 beads by 11:30 local, a
// record, filed 111, and the "open" number went UP. A number that moves
// against the work is not a health reading, it is a discouragement with a
// decimal point.
//
// So four numbers replace it, and each answers a question the total cannot:
//
//	closes today        did the shop move today
//	7-day median        against what it usually moves
//	open by class       feature / bug / debt — the operator tracks the
//	                    three separately, because a backlog of features is
//	                    a roadmap and a backlog of bugs is a problem
//	P1 / P2             how much of the open pile is actually urgent
//
// plus created-vs-closed per day, which is the only pair that says whether
// the pile is growing or shrinking — the question the total looked like it
// was answering and never was.
//
// Class is read through BeadClass (beads.go) and nowhere else, so the
// scorecard, the pulse line and verify-after's filer cannot disagree about
// what a bead is. `unclassified` is carried all the way to the rendering
// rather than folded away: on the day this landed it was almost the whole
// store, and a census that hid that would have reported a tidy fiction.

import (
	"fmt"
	"io"
	"sort"
	"time"
)

// PulseDays is how many COMPLETE days the median and the per-day table look
// back over. Today rides along on the table as a partial day; it is kept out
// of the median on purpose (see BeadPulse.Median).
const PulseDays = 7

// ClassCensus is one class's share of the open pile.
type ClassCensus struct {
	Open int // status != closed
	P1   int
	P2   int
}

// PulseDay is one calendar day's flow: what was filed and what was closed.
// Day is that day's local midnight — the operator's day, which is the day
// "86 closes by 11:30" was counted in.
type PulseDay struct {
	Day     time.Time
	Created int
	Closed  int
	Partial bool // today: the day is not over, so the row is a running total
}

// BeadPulse is one reading over every configured beads repo.
type BeadPulse struct {
	Now         time.Time
	ClosedToday int
	// Median is the median of the PulseDays complete days ENDING YESTERDAY.
	// Today is excluded because it is partial: a median taken at 09:00 that
	// includes this morning's three closes is a bar the morning itself just
	// lowered, and "today vs typical" stops meaning anything. MedianKnown
	// says whether there were any days to take it over at all.
	Median      int
	MedianKnown bool
	Class       map[string]ClassCensus
	Days        []PulseDay // oldest first: the median's window, then today
	Repos       int        // beads repos actually read
	Unread      int        // configured repos whose scan failed
}

// Open is the whole open pile, summed from the classes rather than counted
// beside them — the raw total is not a reading anyone reports, but the
// scorecard's own table has to add up and this is what it adds up to.
func (p BeadPulse) Open() int {
	n := 0
	for _, c := range p.Class {
		n += c.Open
	}
	return n
}

// P1 and P2 are the urgent share of the open pile, across every class.
func (p BeadPulse) P1() int { return p.prio(func(c ClassCensus) int { return c.P1 }) }
func (p BeadPulse) P2() int { return p.prio(func(c ClassCensus) int { return c.P2 }) }

func (p BeadPulse) prio(f func(ClassCensus) int) int {
	n := 0
	for _, c := range p.Class {
		n += f(c)
	}
	return n
}

// Line is the one line `posse status`, the pulse tick and the cockpit all
// print — one rendering, so the three cannot drift:
//
//	closes today 86 (median 41) · open 40F/86B/0D/18U · P1 3 · P2 71
//
// The four class slots are always all four, zero or not. A slot that
// disappears when it is empty is a slot nobody notices is missing, and the
// two the shop most needs to see move — debt and unclassified — are exactly
// the two that sit at zero until a groom runs (ranger-base-ppc85).
func (p BeadPulse) Line() string {
	median := "median ?"
	if p.MedianKnown {
		median = fmt.Sprintf("median %d", p.Median)
	}
	c := p.Class
	line := fmt.Sprintf("closes today %d (%dd %s) · open %dF/%dB/%dD/%dU · P1 %d · P2 %d",
		p.ClosedToday, PulseDays, median,
		c[ClassFeature].Open, c[ClassBug].Open, c[ClassDebt].Open, c[ClassUnclassified].Open,
		p.P1(), p.P2())
	if p.Unread > 0 {
		// Same rule the scorecard's own caveat keeps: a repo that could not
		// be read holds an unknown number, not zero, and a reading that
		// does not say so is read as the whole shop.
		line += fmt.Sprintf(" · partial, %d repo(s) unread", p.Unread)
	}
	return line
}

// FoldBeadPulse computes the reading from issues already in hand. now is a
// parameter, not time.Now(), so a test can hold the clock still against
// fixed stamps — and so every surface in one process reads one clock.
func FoldBeadPulse(issues []BdIssue, now time.Time) BeadPulse {
	p := BeadPulse{Now: now, Class: map[string]ClassCensus{}}
	for _, cl := range BeadClasses {
		p.Class[cl] = ClassCensus{}
	}
	today := dayStart(now)
	// The window runs from the oldest complete day up to today: PulseDays
	// complete days plus the partial one. Indexed by local midnight so a
	// day with no traffic still gets its row.
	idx := map[time.Time]int{}
	for i := 0; i <= PulseDays; i++ {
		d := today.AddDate(0, 0, i-PulseDays)
		idx[d] = i
		p.Days = append(p.Days, PulseDay{Day: d, Partial: i == PulseDays})
	}
	for _, is := range issues {
		if is.Status != "closed" {
			cl := BeadClass(is)
			c := p.Class[cl]
			c.Open++
			switch is.Priority {
			case 1:
				c.P1++
			case 2:
				c.P2++
			}
			p.Class[cl] = c
		}
		if !is.Created.IsZero() {
			if i, ok := idx[dayStart(is.Created.In(now.Location()))]; ok {
				p.Days[i].Created++
			}
		}
		// A close is counted by closed_at and nothing else: `status:
		// closed` with no stamp is a close whose DAY is unknown, and
		// dropping it into today would invent a number the operator reads
		// as this morning's work.
		if is.ClosedAt == nil {
			continue
		}
		day := dayStart(is.ClosedAt.In(now.Location()))
		if day.Equal(today) {
			p.ClosedToday++
		}
		if i, ok := idx[day]; ok {
			p.Days[i].Closed++
		}
	}
	// The median is over the complete days only — every row but the last.
	daily := make([]int, 0, PulseDays)
	for _, d := range p.Days {
		if !d.Partial {
			daily = append(daily, d.Closed)
		}
	}
	if len(daily) > 0 {
		sort.Ints(daily)
		p.Median, p.MedianKnown = daily[len(daily)/2], true
	}
	return p
}

// dayStart is the local midnight of t's calendar day.
func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// ReadBeadPulse scans every configured beads repo and folds one reading.
// A repo whose scan fails is counted as UNREAD rather than dropped — the
// same rule the scorecard's table and ReadyAll's queue keep: an unreadable
// store holds an unknown number, never zero, and the line says so.
//
// One `bd list --all` per repo per call, which is what the reading costs:
// ~0.4s on this instance's store with --no-daemon (beads.go). Callers on a
// timer (the pulse tick, the cockpit's governance scan) already pay several
// list calls at that cadence; `posse status` pays it once, where a human
// asked.
func (a *App) ReadBeadPulse(bd Bd, now time.Time) (BeadPulse, []error) {
	var issues []BdIssue
	var failed []error
	repos := 0
	for _, dir := range a.BeadsDirs() {
		got, err := bd.ListAll(dir)
		if err != nil {
			failed = append(failed, ScanError{Dir: dir, Err: err})
			continue
		}
		repos++
		issues = append(issues, got...)
	}
	p := FoldBeadPulse(issues, now)
	p.Repos, p.Unread = repos, len(failed)
	return p, failed
}

// WriteBeadPulse is the scorecard's section: the line, the class table with
// P1/P2 per class, and created-vs-closed per day. The scorecard is where the
// operator goes for the arithmetic behind the one-liner, so this prints the
// inputs — including the days the median is taken over, so the median can be
// checked by eye rather than believed.
func WriteBeadPulse(w io.Writer, p BeadPulse) {
	fmt.Fprintf(w, "\nshop pulse — %s\n", p.Line())
	fmt.Fprintf(w, "%-16s %6s %5s %5s\n", "open by class", "open", "P1", "P2")
	for _, cl := range BeadClasses {
		c := p.Class[cl]
		fmt.Fprintf(w, "%-16s %6d %5d %5d\n", cl, c.Open, c.P1, c.P2)
	}
	fmt.Fprintf(w, "%-16s %6d %5d %5d\n", "total", p.Open(), p.P1(), p.P2())
	if p.Class[ClassUnclassified].Open > 0 {
		fmt.Fprintf(w, "\n%d open bead(s) carry neither -t feature/bug nor -l debt — counted as %s, never\nfolded into a class (ADR 0006 §1, amended 2026-09-02); the groom is what closes that gap\n",
			p.Class[ClassUnclassified].Open, ClassUnclassified)
	}
	fmt.Fprintf(w, "\n%-12s %8s %7s %7s\n", "day", "created", "closed", "net")
	for _, d := range p.Days {
		label := d.Day.Format("2006-01-02")
		if d.Partial {
			label += "*"
		}
		fmt.Fprintf(w, "%-12s %8d %7d %+7d\n", label, d.Created, d.Closed, d.Created-d.Closed)
	}
	median := "no complete day to take it over"
	if p.MedianKnown {
		median = fmt.Sprintf("%d closes/day", p.Median)
	}
	fmt.Fprintf(w, "\n* today, a running total. The %dd median is %s — taken over the %d COMPLETE\ndays above, never over today, so \"today vs typical\" is not a bar this morning set itself.\n",
		PulseDays, median, PulseDays)
	if p.Unread > 0 {
		fmt.Fprintf(w, "read %d of %d configured beads repo(s) — every number in this section counts only those\n", p.Repos, p.Repos+p.Unread)
	}
}

// PulseFailureLines renders the scan errors a caller wants named above
// the line. Kept beside the reading so `posse status`, the watch log and the
// cockpit say the same sentence about the same failure.
func PulseFailureLines(failed []error) []string {
	out := make([]string, 0, len(failed))
	for _, err := range failed {
		out = append(out, "shop pulse: "+err.Error())
	}
	return out
}
