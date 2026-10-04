package main

// cockpitNotices — where the SHARED backend's OWN diagnostic lines go while
// the cockpit holds the alternate screen (ranger-base-2vhqo).
//
// THE DEFECT. `posse cockpit` shares one HerdrBackend with a dispatcher of
// its own (cockpit.go: c.hb and c.disp) and left HerdrBackend.Warn nil, so
// every b.warn site outside planLaunch/createSession wrote the process's
// stderr — with the alt screen up and the cockpit homing and overwriting
// every frame (runCockpit's one clear, ranger-base-w2uoe). A line written
// there lands ON the frame the cockpit is drawing and is gone with the next
// redraw: worse than the /dev/null a watch loop had (ranger-base-ws20a),
// because it also corrupts the only surface the operator is reading. The
// lines, all of them reachable from this screen:
//
//   - listSessions' five "session meta file(s) kept, not listed"
//     abstentions, two of which carry a two-line repair recipe. The cockpit
//     takes this path on EVERY tick (refreshSessions), so one withheld meta
//     wrote one of them every two seconds;
//   - killAndLand's "fold refusals spool for <name>" and noteUnlandedOnKill's
//     "<bead> left work unlanded in <tree> … and bd could not say whether it
//     is closed", from the cockpit's own `x` (c.hb.KillSessionAndLand).
//
// WHY NOT RouteBackendWarnings(), which is what `posse dispatch` calls
// (ranger-base-ws20a): the cockpit's dispatcher writes io.Discard, and its
// Progress is c.note — a one-field status line fed by a NON-BLOCKING send
// with a bare `default:` arm, so it does not merely show one line at a time,
// it DROPS what will not fit. A listing's five abstentions and their recipes
// pushed through it would arrive as one surviving line and four silent
// discards, and the one line that did survive would be competing with "did
// my keypress land?". That is the decision this bead was filed to make, and
// the answer is a surface of its own.
//
// THE SURFACE, on the two patterns this screen already has for a diagnostic
// that does not fit one field:
//
//   - a COUNT where the operator is looking, like costUnread's "N
//     transcript(s) unreadable" and govFailed — a filler row of its own
//     above GOVERNANCE, on planStale's reasoning (the header's flex column
//     truncates from its tail, and this line must survive an 80-column
//     popup). Zero notices draws nothing, so a clean shop keeps every byte
//     it had;
//   - the full text behind `w`, in modePeek — which is the only surface here
//     that can hold a TWO-LINE repair recipe, and the half c.note could
//     never carry.
//
// DISTINCT lines only, and that is load-bearing twice over. refreshSessions
// writes the same abstention every tick, so a sink that appended would hold
// 1,800 identical lines an hour; and a count that ticked up every two
// seconds would change the frame on every tick, which is exactly what the
// identical-frame drop (c.lastFrame, cockpitflicker_test.go) exists to
// avoid. A repeat is therefore a no-op: the count moves when something NEW
// happens and at no other time.
//
// NOTHING IS LOST ON QUIT: flush() writes whatever was collected to stderr
// after the alt screen is restored, which is where these lines were always
// going and where the scrollback keeps them.

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// cockpitNoticeCap bounds the distinct lines kept. The five listing
// abstentions are bounded by their shapes, but a kill refusal names its
// session, so a long-lived cockpit over a sick board can mint new ones —
// hence a cap, and a count of what it refused rather than a silent drop.
const cockpitNoticeCap = 200

// cockpitNotices collects the backend's own diagnostic lines. Every method
// is nil-safe: the cockpit's own tests build the struct literally and leave
// this field nil, and a nil sink is the pre-fix behaviour with no row.
//
// It is written from goroutines (a launch runs off the event loop, and
// c.disp's gathers run beside it) and read on the event loop at draw time,
// so the mutex is not decoration.
type cockpitNotices struct {
	mu      sync.Mutex
	seen    map[string]bool
	lines   []string
	dropped int
}

// Write is the io.Writer the backend holds. b.warn does one Fprintf per
// call, so one Write is one whole notice — which may be two lines, and both
// are kept as entries of their own so the recipe's second line survives on
// its own merits. A short write would split one notice in two and lose
// nothing, which is the only failure this split can have.
func (n *cockpitNotices) Write(p []byte) (int, error) {
	if n == nil {
		return len(p), nil
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, ln := range strings.Split(string(p), "\n") {
		// Trailing only: the continuation line of a repair recipe is
		// indented by two spaces and that indent is what makes it read as
		// the recipe it is.
		ln = strings.TrimRight(ln, " \t\r")
		if ln == "" {
			continue
		}
		if n.seen[ln] {
			continue
		}
		if len(n.lines) >= cockpitNoticeCap {
			n.dropped++
			continue
		}
		if n.seen == nil {
			n.seen = map[string]bool{}
		}
		n.seen[ln] = true
		n.lines = append(n.lines, ln)
	}
	return len(p), nil
}

// count is the number of distinct notices — what the row shows.
func (n *cockpitNotices) count() int {
	if n == nil {
		return 0
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.lines)
}

// text is every notice in the order it first arrived, newest last, for the
// peek. The dropped tail is part of the text and not a separate field: a
// reader of a capped list has to be told it is capped.
func (n *cockpitNotices) text() string {
	if n == nil {
		return ""
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.lines) == 0 {
		return ""
	}
	out := strings.Join(n.lines, "\n")
	if n.dropped > 0 {
		out += fmt.Sprintf("\n… and %d more notice(s) past the %d this screen keeps", n.dropped, cockpitNoticeCap)
	}
	return out
}

// noticeRow is the line the row model shows, or "" when there is nothing to
// say. It names the key rather than leaving `w` to the footer: the footer
// offers the SELECTED section's keys (ADR 0004 §3) and this is not one of
// them, so the row that reports the notices is the thing that has to say how
// to read them.
func (n *cockpitNotices) noticeRow() string {
	c := n.count()
	if c == 0 {
		return ""
	}
	// The three sources named in the order an operator meets them, and the
	// dispatch one is why this sentence changed (ranger-base-jqe3b): the
	// cockpit's dispatcher writes its errors here too, so a hint naming only
	// the listing and the kill sent a reader looking in the wrong place. Still
	// inside 80 columns, which is what this row has to survive.
	return fmt.Sprintf("%d backend notice(s) — a listing, a kill or a dispatch said so; w reads them", c)
}

// flush hands the notices back to stderr, where they were going before this
// sink existed. runCockpit calls it on the way out, AFTER the alt screen is
// restored: the screen that was being protected is gone by then, and a line
// printed here is in the terminal's scrollback rather than in a frame that
// was overwritten. A cockpit the operator quit having never pressed `w` is
// the case this exists for.
func (n *cockpitNotices) flush(w io.Writer) {
	body := n.text()
	if body == "" || w == nil {
		return
	}
	fmt.Fprintf(w, "posse: %d backend notice(s) while the cockpit held the screen:\n%s\n", n.count(), body)
}
