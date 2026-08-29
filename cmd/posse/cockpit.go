package main

// cockpit — the interactive oversight view, built to run inside a herdr
// plugin popup pane (plugin/herdr-plugin.toml). Fresh and small on purpose:
// raw-mode keys + ANSI drawing, no TUI framework. herdr itself remains the
// layout/interaction surface for everything beyond oversight.
//
// Three sections under one cursor (ADR 0004 §2), each with its own keys (§3);
// the footer shows the selected section's:
//
//   SESSIONS (blocked-first)     IN PROGRESS (stalled-first)   READY WORK
//   ─ enter focus & quit         ─ enter focus the holder      ─ c claim
//   ─ p     prompt its agent     ─ p     prompt the holder      ─ d dispatch:
//   ─ v     peek terminal tail   ─ v     peek the holder            route to a
//   ─ o     crew/fleet (0008)    ─ d     resume (--resume)          persona,
//   ─ x     kill (y confirms)    ─ u     unclaim (y confirms)       launch,
//                                                                  claim, prompt
//   j/k or arrows move · tab cycles sections · r refresh · q quit
//   ctrl-d/ctrl-u page · g/G top/bottom
//
// v2 (ADR 0004 §1, §4, §5): refresh() builds a flat row model; render(w,h)
// draws it to the terminal's size — columns are fixed, flex or droppable, and
// a single viewport scrolls the whole list under one cursor. render is a pure
// function of (rows, cursor, offset, mode, status), which is what the golden
// tests in cockpit_test.go pin.
//
// Non-tty stdin (tests, pipes) falls back to a display-only refresh loop that
// renders the same row model at 80 wide.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/ranger360ai/posse/internal/rhq"
)

type cockpitMode int

const (
	modeNormal  cockpitMode = iota
	modePrompt              // typing a prompt for the selected session
	modeConfirm             // y/n confirmation (kill a session, unclaim a bead)
	modePeek                // showing a terminal tail; any key returns
)

// What a modeConfirm y answers. Two destructive keys share the mode, and
// the footer has to name the right thing (ADR 0004 §3).
type confirmKind int

const (
	confirmKill confirmKind = iota
	confirmUnclaim
)

// section is which of the three lists a row (or the cursor) belongs to, in
// draw order: SESSIONS · IN PROGRESS · READY WORK (ADR 0004 §2). Cursor
// space runs in the same order, so the section a cursor is in is a range
// check, not a lookup.
type section int

const (
	secSessions section = iota
	secInProg
	secIssues
)

type cockpit struct {
	app  *rhq.App
	hb   *rhq.HerdrBackend
	bd   rhq.Bd
	disp *rhq.Dispatcher
	out  io.Writer

	sessions    []rhq.HerdrSession
	inprog      []rhq.RepoIssue // claimed beads, stalled-first (ADR 0004 §2)
	issues      []rhq.RepoIssue // ready work, with the claimed ones filtered out
	rows        []row           // the view model both draw paths render (ADR 0004 §1)
	cursor      int             // sessions, then inprog, then issues
	offset      int             // first row of the viewport (ADR 0004 §4)
	width       int             // terminal size at the last draw; paging keys read it
	height      int
	mode        cockpitMode
	confirm     confirmKind // what a y answers in modeConfirm
	input       []rune      // prompt buffer
	peekText    string
	status      string      // last action result, shown in the footer
	dispatching bool        // a launch goroutine is in flight (one at a time)
	results     chan string // launch goroutine → event loop status line

	// Running cost (ADR 0003 §4): a background scan of every registered cost
	// provider's transcripts, refreshed every costEvery, keyed by bead id;
	// the day total in the footer. A session on a runtime with no adapter
	// shows "uncounted" and names its runtime (ADR 0012 D4).
	costs      chan *rhq.CostReport
	costByBead map[string]float64
	costToday  float64
	costAt     time.Time
	// costUncounted counts those sessions and costUncountedRuntimes names
	// which runtimes they were on. Carrying the names, not a hardcoded
	// "codex/grok", is what keeps the footer honest on the commit that gives
	// a runtime an adapter: the label follows the registry.
	costUncounted         int
	costUncountedRuntimes []string
	costDayCap            float64 // config budget_day: (ADR 0003 Dial E); 0 = no cap
	// costUnread is how many transcripts the last scan could NOT read
	// (ADR 0018 §3) — a COUNT and not the error, for the same reason
	// govRead.failed is: the footer is one line and the cockpit owns the
	// whole terminal, so `posse cost` is where the reason is printed. Above
	// zero, costToday and its budget percentage are a floor, not a total.
	costUnread int

	// The plan's rate windows (rangerhq-jgm): the current reading in
	// the header, no history. Never a guessed number — but never silently
	// empty either when the guard is configured, because empty reads as
	// "no guard" and that is the blind window nobody can see (rangerhq-6h1).
	plans      chan planRead
	planLine   string
	planReadAt time.Time // last SUCCESSFUL reading (or cockpit start) — the header's blind clock

	// The credential-expiry segment (ADR 0019 D5, bead ranger-base-k6ha):
	// the posse-owned session mints that die inside the window, soonest
	// first, or nil. It rides the plan scan because it is the same
	// off-the-event-loop tick asking the other half of one question — is
	// this shop able to authenticate tomorrow — and because a stamp in a
	// file does not change faster than every two minutes.
	//
	// Empty is the overwhelmingly common state and draws NOTHING: the
	// header keeps the bytes it had before this existed, and the column
	// only appears when there is something to say.
	creds []rhq.CredExpiry

	// The governance surface (ADR 0029 §2, bead rangerhq-81y0): the third
	// rendering of ShopCheck, drawn as a block
	// above SESSIONS. It is scanned off the event loop like the cost and
	// plan readings — the check talks to herdr, bd, the plan snapshot and
	// the kernel, and the draw path does no I/O.
	//
	// The rows are NOT cursor items: there is no key that acts on a
	// condition, because conditions heal by themselves and the things a
	// human does about them are already keys on the sections below. So the
	// block is filler rows, cursor space is untouched, and nothing about
	// selection, tab or reselect changes.
	govs      chan govRead
	gov       rhq.GovSet
	govFailed int
	govAt     time.Time

	// now is the clock the header reads; nil = time.Now. The golden tests
	// (ADR 0004 §5) pin it so the render is byte-stable.
	now func() time.Time

	// launcher is what `d` calls; nil = the real dispatcher. The key tests
	// have no herdr to launch into, and a launch runs off the event loop
	// where a panic would take the process with it.
	launcher func(bead rhq.RepoIssue, resume bool) (string, error)
}

const (
	costEvery = 30 * time.Second
	// planEvery is how often the header REFRESHES, which since
	// rangerhq-tdy8 is no longer how often the endpoint is asked: the tick
	// reads the shared snapshot and only pays for a request when that
	// snapshot is older than `plan_usage_ttl:`. Two minutes keeps the
	// header picking up a reading a dispatch pass took, for free.
	planEvery = 2 * time.Minute
)

// govEvery is how often the GOVERNANCE block is recomputed. It is the
// heaviest of the three scans (a few bd calls per configured repo), so it
// runs at the cost scan's cadence rather than the 2s redraw's: a condition
// that has been true for thirty seconds is not less true, and none of these
// rows is one a human reacts to inside a second.
const govEvery = 30 * time.Second

// govRead is one shop check's worth of fact, computed off the event loop.
// failed is a COUNT and not the errors: the block says the set is partial,
// and `posse status` is where the reasons are printed — the cockpit owns the
// whole terminal and has nowhere to put a multi-line error.
type govRead struct {
	set    rhq.GovSet
	failed int
}

// scanGov runs off the event loop; the result lands on c.govs.
func (c *cockpit) scanGov() {
	in := rhq.StatusInputs(c.app, c.hb, io.Discard)
	in.Caller = "cockpit"
	set, failed := rhq.ShopCheck(in)
	select {
	case c.govs <- govRead{set: set, failed: len(failed)}:
	default:
	}
}

// scanCosts runs off the event loop; the result lands on c.costs.
func (c *cockpit) scanCosts() {
	rep := rhq.ScanCosts("", time.Now().Add(-14*24*time.Hour))
	// The day cap rides along with the scan: it is a config read, and the
	// draw path must not do file I/O. 0 = Dial E dormant = no budget line.
	_, rep.DayCap = c.app.BudgetCaps(io.Discard)
	if c.bd.Available() {
		rep.AttributePersonas(c.app, c.bd)
	}
	rep.CountUncounted(c.hb)
	select {
	case c.costs <- rep:
	default:
	}
}

// applyCost lands one scan on the event loop's state — the whole of what
// the footer knows about money. It is its own function, like applyGov and
// applyPlan, so the wiring can be pinned: a footer test builds the fields by
// hand and would stay green if a field stopped being carried across from the
// report (which is how Unread was missing from the display in the first
// place, ADR 0018 §3 / ranger-base-c65c).
func (c *cockpit) applyCost(rep *rhq.CostReport) {
	c.costByBead = rep.ByBead()
	c.costToday = rep.DayTotal(time.Now())
	c.costUncounted = rep.Uncounted
	c.costUncountedRuntimes = rep.UncountedRuntimes
	c.costDayCap = rep.DayCap
	c.costUnread = rep.Unread
	c.costAt = time.Now()
}

// planRead is one scan's worth of fact. The clock that turns a failed read
// into "guard blind 14m" lives on the event loop, not here, so this stays a
// pure read and nothing races the header.
type planRead struct {
	line      string    // the reading, formatted; "" when it could not be taken
	at        time.Time // when that reading was TAKEN (zero = now) — it may be a shared one, minutes old
	guarded   bool      // any plan_guard_<window>: configured — then blindness is worth saying
	noAdapter bool      // the guard is armed and nothing here can read a meter (ADR 0012 D4)
	noSource  bool      // the guard is armed, an adapter ships, and this platform holds no credential (ADR 0019 D3)
	ledger    bool      // budget_pass:/budget_day: configured — ADR 0018's fork, and what blindness COSTS
	// creds is the OTHER credential question this scan answers, and it is
	// unrelated to the guard: which posse-owned session mints expire inside
	// the window (ADR 0019 D5). It is here rather than on a fourth ticker
	// because it is read at the same cadence, off the same goroutine, and
	// lands on the same channel — nothing in the header may do I/O.
	creds []rhq.CredExpiry
}

// planOffState classifies a failed read into the two states that are NOT
// blindness. Both mean the guard is off and no clock is running; they differ
// in what the operator would do next, which is the whole reason the header
// says which one it is.
//
// It is its own function so the classification can be pinned without a
// cockpit, a cache and a machine in a particular state — a rule this thin
// tested only through a rig is a rule nobody re-checks when the error
// plumbing moves.
func planOffState(err error) (noSource, noAdapter bool) {
	if rhq.NoSourceReason(err) != nil {
		// Asked first and asked through the seam's own reader: a NoSource
		// arrives BOTH on its own (the store went away after the
		// availability check) and inside a *NoPlanAdapter (the check caught
		// it), and both are the same fact about this platform.
		return true, false
	}
	var na *rhq.NoPlanAdapter
	return false, errors.As(err, &na)
}

// scanPlan runs off the event loop; the reading lands on c.plans. Read-only
// and fail-quiet — this is a display, not a guard.
func (c *cockpit) scanPlan() {
	var r planRead
	// io.Discard: a malformed threshold is dispatch's line to print, and the
	// cockpit owns the whole terminal — it cannot write to stderr at all.
	r.guarded = len(c.app.PlanGuardThresholds(io.Discard)) > 0
	// Read whether Dial E is armed, not what it says: the header reports
	// which policy a blind guard is under (ADR 0018 §1), and the dollars are
	// the cost scan's to print.
	pass, day := c.app.BudgetCaps(io.Discard)
	r.ledger = pass > 0 || day > 0
	// Through the shared cache (rangerhq-tdy8), never the endpoint direct:
	// a cockpit open all day was ~30 requests/hour on a metering endpoint,
	// and the 429s that bought cost the fleet a three-hour blind guard.
	// Most of these ticks are now a file read, and the ones that are not
	// are the whole instance's one request per TTL.
	if u, at, err := c.app.PlanCache("cockpit").Read(c.app.PlanUsageTTL(io.Discard)); err == nil {
		r.line, r.at = u.Line(), at
	} else {
		// The failures that are not blindness: no adapter (ADR 0012 D4) and
		// no credential store on this platform (ADR 0019 D3). Neither has a
		// clock, so the header must not show a blind timer counting up
		// toward a park that will never come.
		r.noSource, r.noAdapter = planOffState(err)
	}
	// Files under the posse home, read here for the same reason the plan
	// reading is: the draw path does no I/O. Nothing about this depends on
	// the guard being armed — a mint expires on a box with no meter guard
	// at all.
	r.creds = c.app.ExpiringCredentials(c.clock())
	select {
	case c.plans <- r:
	default:
	}
}

// applyPlan lands one scan on the header. Split out of the event loop for
// takeGov's reason: the scan and the segment are each testable on their own,
// and the step that JOINS them is the one a refactor drops silently — a
// header that renders a field nothing assigns is green in every test that
// sets the field itself.
func (c *cockpit) applyPlan(r planRead) {
	c.planLine, c.creds = c.planSegment(r), r.creds
}

// planSegment is the header's plan segment, and the witness half of
// rangerhq-6h1. A good reading is the reading. A failed one is empty today
// — which is indistinguishable from "no guard configured", the exact
// ambiguity that lets an unattended loop go blind unseen. So when the guard
// IS configured, say the blind instead of nothing: `plan — · guard blind 14m`.
// The clock is time since the last successful reading, floored at cockpit
// start, the same rule the dispatcher's blind window uses.
//
// A guard with no adapter is the third state and says so: it is off, not
// blind, and no clock is running (ADR 0012 D4). A guard whose adapter ships
// and whose platform holds no credential is the fourth, and gets its own
// words rather than the third's (ADR 0019 D3) — "no adapter" on a box that
// has simply never run `claude` sends an operator looking for a missing
// feature instead of running one command.
//
// ADR 0018 §1 made blindness two outcomes, so the header names which one is
// waiting: with Dial E armed an unattended pass past `plan_guard_blind_max:`
// degrades under the ledger (`… — ledger brake`), with it unset that same
// pass parks the fleet's on-meter lanes. Same blind clock, opposite days —
// reading one as the other is the ambiguity this segment exists to close.
func (c *cockpit) planSegment(r planRead) string {
	now := c.clock()
	if c.planReadAt.IsZero() {
		c.planReadAt = now
	}
	if r.line != "" {
		// The reading's own age, not the tick's: a shared reading is a
		// snapshot, and the header's blind clock counts from when it was
		// taken (rangerhq-tdy8).
		c.planReadAt = now
		if !r.at.IsZero() && r.at.Before(now) {
			c.planReadAt = r.at
		}
		return r.line
	}
	if !r.guarded {
		return "" // no guard: nothing to be blind about, nothing to say
	}
	if r.noSource {
		// Not "no adapter": one ships, and what is missing is a credential
		// this platform has never been given. The dispatch line carries the
		// store and the command; the header carries which of the two
		// guard-off states this is, which is what a glance is for.
		return "plan — · guard off, no credential source"
	}
	if r.noAdapter {
		return "plan — · guard off, no adapter"
	}
	seg := "plan — · guard blind " + rhq.BlindFor(now.Sub(c.planReadAt))
	if r.ledger {
		seg += " — ledger brake"
	}
	return seg
}

// sessionCost is the running api-equiv $ for a persona session's bead, or
// "" when the session is not a per-bead one (or the runtime is uncounted).
func (c *cockpit) sessionCost(s rhq.HerdrSession) string {
	if s.Agent == "" {
		return ""
	}
	// An adapter is what makes a runtime countable, not its name (ADR 0012
	// D4) — grok gained one and stopped printing $uncounted here on the same
	// commit, with nothing in the cockpit to edit.
	if s.Runtime != "" {
		if _, ok := rhq.CostProviderFor(s.Runtime); !ok {
			return "$uncounted"
		}
	}
	if s.Dir == "" || c.costByBead == nil {
		return ""
	}
	prefix := rhq.SessionFor(s.Agent, s.Dir) + "-"
	if !strings.HasPrefix(s.Name, prefix) {
		return ""
	}
	bead := strings.TrimPrefix(s.Name, prefix)
	if cost, ok := c.costByBead[bead]; ok {
		return fmt.Sprintf("$%.2f", cost)
	}
	return ""
}

func runCockpit(a *rhq.App, hb *rhq.HerdrBackend, out io.Writer) error {
	c := &cockpit{app: a, hb: hb, bd: rhq.NewBd(), out: out,
		disp: rhq.NewDispatcher(a, hb, io.Discard), results: make(chan string, 4),
		costs: make(chan *rhq.CostReport, 1), plans: make(chan planRead, 1),
		govs: make(chan govRead, 1)}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return c.displayOnly()
	}

	old, err := term.MakeRaw(fd)
	if err != nil {
		return c.displayOnly()
	}
	defer term.Restore(fd, old)
	fmt.Fprint(c.out, "\033[?1049h\033[?25l")       // alt screen, hide cursor
	defer fmt.Fprint(c.out, "\033[?25h\033[?1049l") // restore

	keys := make(chan []byte, 8)
	go func() {
		buf := make([]byte, 8)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				close(keys)
				return
			}
			k := make([]byte, n)
			copy(k, buf[:n])
			keys <- k
		}
	}()

	// SIGTERM must unwind through the defers (raw-mode restore, alt-screen
	// exit) — a bare kill would leave a normal pane's terminal raw.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// The popup is resized by every herdr layout change: redraw at the new
	// size rather than waiting for the 2s tick (ADR 0004 §1).
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	c.refresh()
	c.draw()
	go c.scanCosts()
	go c.scanPlan()
	go c.scanGov()
	costTick := time.NewTicker(costEvery)
	defer costTick.Stop()
	planTick := time.NewTicker(planEvery)
	defer planTick.Stop()
	govTick := time.NewTicker(govEvery)
	defer govTick.Stop()
	for {
		select {
		case <-stop:
			return nil
		case <-winch:
			c.draw()
		case <-tick.C:
			if c.mode == modeNormal {
				c.refresh()
				c.draw()
			}
		case <-costTick.C:
			go c.scanCosts()
		case <-planTick.C:
			go c.scanPlan()
		case <-govTick.C:
			go c.scanGov()
		case g := <-c.govs:
			c.applyGov(g)
			if c.mode == modeNormal {
				c.draw()
			}
		case r := <-c.plans:
			c.applyPlan(r)
			if c.mode == modeNormal {
				c.draw()
			}
		case rep := <-c.costs:
			c.applyCost(rep)
			if c.mode == modeNormal {
				c.draw()
			}
		case msg := <-c.results:
			c.dispatching = false
			c.status = msg
			c.refresh()
			c.draw()
		case k, ok := <-keys:
			if !ok {
				return nil
			}
			quit, err := c.handleKey(k)
			if err != nil {
				return err
			}
			if quit {
				return nil
			}
			c.draw()
		}
	}
}

// selection is what the cursor points at, independent of row position:
// the item's key (session name, or repo+bead id) plus its section and
// offset as a fallback. The list reshuffles under the cursor — the 2s
// refresh re-sorts sessions blocked-first, a dispatch adds a session row,
// a close removes a bead row — so an absolute index would silently land on
// a different row (rangerhq-5li).
type selection struct {
	key     string
	sec     section // which list it was in
	offset  int     // index within the section
	present bool    // there was a row under the cursor
}

func sessionKey(s rhq.HerdrSession) string { return "s:" + s.Name }

// issueKey is deliberately the same for both bead sections: a claim moves a
// bead from READY WORK to IN PROGRESS under the operator's cursor, and the
// cursor should follow it there rather than snap to whatever took its place.
func issueKey(is rhq.RepoIssue) string { return "i:" + is.Dir + "|" + is.ID }

func (c *cockpit) selected() selection {
	if c.cursor < len(c.sessions) {
		return selection{key: sessionKey(c.sessions[c.cursor]), sec: secSessions, offset: c.cursor, present: true}
	}
	if i := c.cursor - len(c.sessions); i < len(c.inprog) {
		return selection{key: issueKey(c.inprog[i]), sec: secInProg, offset: i, present: true}
	}
	if i := c.cursor - len(c.sessions) - len(c.inprog); i < len(c.issues) {
		return selection{key: issueKey(c.issues[i]), sec: secIssues, offset: i, present: true}
	}
	return selection{}
}

// reselect finds sel's cursor position in the freshly loaded lists: the
// same item if it still exists (in any section — a bead changes section
// when it is claimed or unclaimed), else the same offset in the same
// section (clamped), else 0.
func reselect(sessions []rhq.HerdrSession, inprog, issues []rhq.RepoIssue, sel selection) int {
	if !sel.present {
		return 0
	}
	for i, s := range sessions {
		if sessionKey(s) == sel.key {
			return i
		}
	}
	for i, is := range inprog {
		if issueKey(is) == sel.key {
			return len(sessions) + i
		}
	}
	for i, is := range issues {
		if issueKey(is) == sel.key {
			return len(sessions) + len(inprog) + i
		}
	}
	base := [3]int{0, len(sessions), len(sessions) + len(inprog)}
	size := [3]int{len(sessions), len(inprog), len(issues)}
	if n := size[sel.sec]; n > 0 {
		off := sel.offset
		if off >= n {
			off = n - 1
		}
		if off < 0 {
			off = 0
		}
		return base[sel.sec] + off
	}
	return 0 // section vanished — top of whatever is left
}

func (c *cockpit) refresh() {
	sel := c.selected()
	defer func() { c.cursor = reselect(c.sessions, c.inprog, c.issues, sel) }()
	herdrDown := false
	if s, err := c.hb.Sessions(); err == nil {
		c.sessions = s
		// Blocked first — the whole point of the oversight view.
		rank := func(s rhq.HerdrSession) int {
			switch s.Status {
			case "blocked":
				return 0
			case "working":
				return 1
			case "idle", "done":
				return 2
			}
			return 3
		}
		for i := 1; i < len(c.sessions); i++ {
			for j := i; j > 0 && rank(c.sessions[j]) < rank(c.sessions[j-1]); j-- {
				c.sessions[j], c.sessions[j-1] = c.sessions[j-1], c.sessions[j]
			}
		}
	} else {
		c.status = err.Error()
		herdrDown = true
	}
	if c.bd.Available() {
		// The in-progress join reads c.sessions, so it runs after them.
		c.inprog = c.bd.InProgressAll(c.app)
		c.sortInProg()
		ready, failed := c.bd.ReadyAll(c.app, "")
		c.issues = readyOnly(ready, c.inprog)
		// An unreadable repo has an unknown queue, and a READY list that is
		// merely shorter is all the operator would otherwise see
		// (rangerhq-llse). Herdr being down is the more basic failure and
		// keeps the line when both happen in the same refresh.
		if len(failed) > 0 && !herdrDown {
			c.status = "ready scan failed: " + failed[0].Error()
		}
	}
	c.buildRows()
}

// stallRank is ADR 0004 §2's stalled-first order — blocked, no session,
// idle, working. The operator's eye goes to what needs a hand; a persona
// that is actually typing needs nothing and sorts last.
func stallRank(state string) int {
	switch state {
	case "blocked":
		return 0
	case noSession, "shell": // nobody (or no agent) is on it
		return 1
	case "idle", "done":
		return 2
	case "working":
		return 3
	}
	return 4
}

// sortInProg puts the stalled beads on top. Stable, so bd's own order —
// which is priority-ish — breaks ties inside a rank.
func (c *cockpit) sortInProg() {
	state := make(map[string]string, len(c.inprog))
	for _, is := range c.inprog {
		state[issueKey(is)] = c.holderState(is)
	}
	sort.SliceStable(c.inprog, func(i, j int) bool {
		return stallRank(state[issueKey(c.inprog[i])]) < stallRank(state[issueKey(c.inprog[j])])
	})
}

// readyOnly drops the beads the IN PROGRESS section already shows: a bead
// belongs to one section only (ADR 0004 §2), and `bd ready` today can hand
// back an in_progress bead whose claim never blocked it.
func readyOnly(ready, inprog []rhq.RepoIssue) []rhq.RepoIssue {
	held := make(map[string]bool, len(inprog))
	for _, is := range inprog {
		held[issueKey(is)] = true
	}
	out := make([]rhq.RepoIssue, 0, len(ready))
	for _, is := range ready {
		if is.Status == "in_progress" || held[issueKey(is)] {
			continue
		}
		out = append(out, is)
	}
	return out
}

func (c *cockpit) selSession() *rhq.HerdrSession {
	if c.cursor < len(c.sessions) {
		return &c.sessions[c.cursor]
	}
	return nil
}

// selInProg is the claimed bead under the cursor, or nil.
func (c *cockpit) selInProg() *rhq.RepoIssue {
	if i := c.cursor - len(c.sessions); i >= 0 && i < len(c.inprog) {
		return &c.inprog[i]
	}
	return nil
}

// selIssue is the *ready* bead under the cursor, or nil — claimed beads are
// selInProg's, and the two sections answer to different keys (ADR 0004 §3).
func (c *cockpit) selIssue() *rhq.RepoIssue {
	if i := c.cursor - len(c.sessions) - len(c.inprog); i >= 0 && i < len(c.issues) {
		return &c.issues[i]
	}
	return nil
}

// items is how many selectable rows the cursor has to walk.
func (c *cockpit) items() int { return len(c.sessions) + len(c.inprog) + len(c.issues) }

// selSection is the section the cursor is in; an empty cockpit reads as
// SESSIONS, which is what its footer should offer.
func (c *cockpit) selSection() section {
	if sel := c.selected(); sel.present {
		return sel.sec
	}
	return secSessions
}

// nextSection is where tab lands: the first row of the next non-empty
// section, cycling SESSIONS → IN PROGRESS → READY WORK → SESSIONS.
func (c *cockpit) nextSection() int {
	base := [3]int{0, len(c.sessions), len(c.sessions) + len(c.inprog)}
	size := [3]int{len(c.sessions), len(c.inprog), len(c.issues)}
	cur := int(c.selSection())
	for i := 1; i <= 3; i++ {
		if s := (cur + i) % 3; size[s] > 0 {
			return base[s]
		}
	}
	return c.cursor
}

// noSession is the holder-state of a claimed bead with no live session —
// an interrupted run, or a persona that was killed mid-bead. `d` is the
// answer, which is why the status line says so.
const noSession = "no session"

// holderSession is the live session working an in-progress bead: its Dial F
// per-bead session first, then the persona's slot session. ADR 0004 §2 names
// only the slot (SessionFor) — but under Dial F the holder is almost always
// SessionForBead, so the slot alone would report "no session" for nearly
// every claimed bead. These are the same two names dispatch checks when it
// decides whether a bead is held (dispatch.go, the --resume path).
func (c *cockpit) holderSession(is rhq.RepoIssue) *rhq.HerdrSession {
	if is.Assignee == "" {
		return nil
	}
	for _, name := range []string{
		rhq.SessionForBead(is.Assignee, is.Dir, is.ID),
		rhq.SessionFor(is.Assignee, is.Dir),
	} {
		for i := range c.sessions {
			if c.sessions[i].Name == name {
				return &c.sessions[i]
			}
		}
	}
	return nil
}

// holderState is the herdr status of the holder's session, or noSession.
// A session with no agent detected reads "shell" — the same word the
// sessions section uses for it.
func (c *cockpit) holderState(is rhq.RepoIssue) string {
	s := c.holderSession(is)
	switch {
	case s == nil:
		return noSession
	case s.Status == "":
		return "shell"
	}
	return s.Status
}

// actSession is the session a key acts on: the selected session row, or the
// holder of the selected in-progress bead — enter/p/v on an in-progress row
// act on the holder (ADR 0004 §3).
func (c *cockpit) actSession() *rhq.HerdrSession {
	if s := c.selSession(); s != nil {
		return s
	}
	if is := c.selInProg(); is != nil {
		return c.holderSession(*is)
	}
	return nil
}

// noHolder sets the status line for enter/p/v on a claimed bead nobody has
// a session for, and reports that it did.
func (c *cockpit) noHolder() bool {
	is := c.selInProg()
	if is == nil {
		return false
	}
	who := is.Assignee
	if who == "" {
		who = "nobody"
	}
	c.status = fmt.Sprintf("%s: %s has no session — d re-dispatches", is.ID, who)
	return true
}

func (c *cockpit) handleKey(k []byte) (quit bool, err error) {
	key := string(k)
	isUp := key == "k" || key == "\033[A"
	isDown := key == "j" || key == "\033[B"

	switch c.mode {
	case modePeek:
		c.mode = modeNormal
		return false, nil

	case modeConfirm:
		yes := key == "y" || key == "Y"
		if c.confirm == confirmUnclaim {
			is := c.selInProg()
			switch {
			case !yes:
				c.status = "unclaim cancelled"
			case is == nil:
			default:
				// ADR 0004 §3: actor none — bd's default actor is the
				// operator, and this is the operator's hand, attributed as
				// such. The assignee is cleared with the claim: taking a
				// bead off a stalled holder puts it back in the pool, which
				// is the whole point of the key.
				if err := c.bd.Unclaim(is.Dir, is.ID, "", false); err != nil {
					c.status = err.Error()
				} else {
					c.status = "unclaimed " + is.ID
					c.refresh()
				}
			}
			c.mode = modeNormal
			return false, nil
		}
		if yes {
			if s := c.selSession(); s != nil {
				landing, err := c.hb.KillSessionAndLand(s.Name)
				switch {
				case err != nil:
					// Including the ADR 0013 §4 reap guard: a session still
					// holding an open bead over an uncommitted tree is not
					// killed here either, and the refusal is one line
					// because this is the one line there is room for. The
					// way through it is `posse kill <name> --force`, which
					// the refusal names.
					c.status = err.Error()
				case landing.Line() != "":
					// The worktree's fate is the half of a kill that can
					// lose work, so it goes on the status line rather than
					// into a stream the cockpit does not show
					// (rangerhq-09o2).
					c.status = "killed " + s.Name + " — " + landing.Line()
				default:
					c.status = "killed " + s.Name
				}
				c.refresh()
			}
		} else {
			c.status = "kill cancelled"
		}
		c.mode = modeNormal
		return false, nil

	case modePrompt:
		switch {
		case key == "\033": // esc
			c.mode, c.input, c.status = modeNormal, nil, "prompt cancelled"
		case key == "\r" || key == "\n":
			text := strings.TrimSpace(string(c.input))
			c.mode, c.input = modeNormal, nil
			if text == "" {
				c.status = "empty prompt — nothing sent"
				break
			}
			s := c.actSession()
			if s == nil {
				break
			}
			target, err := c.hb.AgentTarget(s.Name)
			if err != nil {
				c.status = err.Error()
				break
			}
			if _, err := c.hb.H.AgentPrompt(target, text, false, 0); err != nil {
				c.status = err.Error()
			} else {
				// The operator just started a conversation here: the session
				// is theirs until they hand it back with `o` (ADR 0008).
				name := s.Name
				c.hb.MarkCrew(name)
				c.refresh() // c.sessions is replaced — s is stale past here
				c.status = "prompted " + name
			}
		case key == "\x7f" || key == "\b":
			if len(c.input) > 0 {
				c.input = c.input[:len(c.input)-1]
			}
		default:
			for _, r := range key {
				if r >= 32 && r != 127 {
					c.input = append(c.input, r)
				}
			}
		}
		return false, nil
	}

	// modeNormal
	switch {
	case key == "q" || key == "\x03": // q or ctrl-c
		return true, nil
	case isDown:
		if c.cursor < c.items()-1 {
			c.cursor++
		}
	case isUp:
		if c.cursor > 0 {
			c.cursor--
		}
	case key == "\x04": // ctrl-d: page down
		c.moveRows(c.pageStep())
	case key == "\x15": // ctrl-u: page up
		c.moveRows(-c.pageStep())
	case key == "g":
		c.cursor = 0
	case key == "G":
		if n := c.items(); n > 0 {
			c.cursor = n - 1
		}
	case key == "\t": // cycle the three sections (ADR 0004 §2)
		c.cursor = c.nextSection()
	case key == "r":
		c.refresh()
		c.status = "refreshed"
	case key == "\r" || key == "\n":
		if s := c.actSession(); s != nil {
			if err := c.hb.FocusSession(s.Name); err != nil {
				c.status = err.Error()
				break
			}
			return true, nil // popup closes, revealing the focused workspace
		}
		c.noHolder()
	case key == "p":
		if c.actSession() != nil {
			c.mode, c.input = modePrompt, nil
		} else {
			c.noHolder()
		}
	case key == "o":
		// ADR 0008: hand the selected session to the operator or back to
		// the fleet. The only way back for a session dispatch must stop
		// skipping — there is no timer.
		if s := c.selSession(); s != nil {
			name, crew := s.Name, !s.Crew
			if err := c.hb.SetCrew(name, crew); err != nil {
				c.status = err.Error()
				break
			}
			c.refresh()
			if crew {
				c.status = name + " is crew (yours) — dispatch skips it"
			} else {
				c.status = name + " is fleet — dispatch may use it"
			}
		}
	case key == "x":
		if s := c.selSession(); s != nil {
			// A foreign row is somebody else's to end (rangerhq-selx), and
			// the cockpit shows the whole herd on purpose — including rows
			// this home holds no meta for. Refused at the key rather than
			// after the y/n, so the operator is not asked to confirm a kill
			// that will not happen; the backend refuses it too, which is
			// what makes this a wall and not a hint. There is no cockpit
			// override: the way through is the CLI's --foreign, which the
			// refusal names.
			if err := rhq.ForeignKillRefusal(s); err != nil {
				c.status = err.Error()
				break
			}
			c.mode, c.confirm = modeConfirm, confirmKill
		}
	case key == "u":
		// ADR 0004 §3: claimed beads only — there is nothing to unclaim on
		// a session or a ready row.
		if c.selInProg() != nil {
			c.mode, c.confirm = modeConfirm, confirmUnclaim
		}
	case key == "v":
		s := c.actSession()
		if s == nil {
			c.noHolder()
			break
		}
		if s.PaneID != "" {
			text, err := c.hb.H.PaneRead(s.PaneID, 15)
			if err != nil {
				c.status = err.Error()
				break
			}
			c.peekText = text
			c.mode = modePeek
		}
	case key == "c":
		if is := c.selIssue(); is != nil {
			resumed, err := c.bd.Claim(is.Dir, is.ID, "")
			switch {
			case err != nil:
				c.status = err.Error()
			case resumed:
				c.status = is.ID + " was already yours — resumed"
				c.refresh()
			default:
				c.status = "claimed " + is.ID
				c.refresh()
			}
		}
	case key == "d":
		if c.dispatching {
			c.status = "a dispatch is already in flight"
			break
		}
		if is := c.selInProg(); is != nil {
			c.launch(*is, true)
		} else if is := c.selIssue(); is != nil {
			c.launch(*is, false)
		}
	}
	return false, nil
}

// dispatchBead is the real launcher. Only one launch is ever in flight
// (c.dispatching), so setting the shared dispatcher's --resume flag per
// call is safe.
func (c *cockpit) dispatchBead(bead rhq.RepoIssue, resume bool) (string, error) {
	c.disp.Resume = resume
	return c.disp.LaunchBead(bead)
}

// launch runs a launcher off the event loop — session create + agent
// startup blocks for up to StartupWait, and the UI has to stay live.
// resume is `d` on an in-progress row: dispatch's --resume semantics, which
// LaunchBead already realizes (re-prompt the holder, or launch it if the
// session is gone).
func (c *cockpit) launch(bead rhq.RepoIssue, resume bool) {
	verb, done := "dispatching", "dispatched "
	if resume {
		verb, done = "resuming", "resumed "
	}
	fn := c.launcher
	if fn == nil {
		fn = c.dispatchBead
	}
	c.dispatching = true
	c.status = verb + " " + bead.ID + "…"
	go func() {
		session, err := fn(bead, resume)
		if err != nil {
			c.results <- err.Error()
			return
		}
		c.results <- done + bead.ID + " → " + session
	}()
}

// ─── row model ───────────────────────────────────────────────────────────────

const (
	aDim, aBold, aRev, aRst = "\033[2m", "\033[1m", "\033[7m", "\033[0m"
	aRed, aGrn, aYlw        = "\033[31m", "\033[32m", "\033[33m"
)

// The view is a flat slice of rows built by buildRows() and drawn by
// render(w,h) — ADR 0004 §1. Rows know nothing about the cursor or the
// terminal size: selection, scrolling and truncation are all decided at
// render time, which is what makes the draw path a pure function of
// (rows, cursor, offset, mode, status) and so golden-testable (§5).
type rowKind int

const (
	rowHeading rowKind = iota
	rowItem
	rowFiller // "(none)" markers and the blank line between sections
)

// Column kinds (ADR 0004 §1). Widths are rune counts, not display widths:
// the flex column is last and emoji live in fixed columns at the left, so a
// wide glyph misaligns nothing the eye reads. That is the deliberate cost of
// "no new dep" — there is no go-runewidth here.
type colKind int

const (
	colFixed colKind = iota // natural width, never truncated
	colFlex                 // exactly one per row; takes what is left, truncated with …
	colDrop                 // trailing dim context; dropped whole on a narrow terminal
)

const (
	dropAt       = 100 // droppable columns vanish below this width
	dropHolderAt = 70  // ...and the holder column below this one
	chromeLines  = 5   // header (2) + footer (3); everything else is the viewport
	minViewport  = 1   // ...but the cursor's row outranks the chrome (rangerhq-5qm)
	scrollMargin = 2   // rows kept between the cursor and a viewport edge
)

type col struct {
	kind colKind
	text string // plain text — ANSI lives in ansi so widths stay countable
	pad  int    // minimum rune width (fixed columns)
	ansi string // colour prefix; reset is appended after the padded text
	drop int    // drop below this terminal width (0 = never, or dropAt for colDrop)
}

type row struct {
	kind rowKind
	sec  section // which of the three lists this row belongs to
	item int     // index in cursor space; only meaningful for rowItem
	cols []col
}

// ─── widths ─────────────────────────────────────────────────────────────────
//
// ADR 0004 §1 (as amended 2026-08-23, rangerhq-swh) specifies display cells
// measured in-tree. As accepted it said rune counts, on the reading that emoji
// sit in fixed columns at the left where a wide glyph misaligns nothing — but
// 🎭persona and 👤 ride inside the *flex* column, so counting them as one cell
// under-truncates the row and a 60-column popup wraps, which is the very thing
// v2 exists to stop. So: cells, not runes, from a table. Still no dependency;
// go-runewidth stays rejected.
//
// The first cut of this table covered the glyphs the cockpit draws today and
// left everything else unknown — and unknown counted as one cell. That is the
// wrong default: under-counting wraps the row, over-counting only leaves a
// blank cell, and a table with holes plus a narrow default fails in the
// wrapping direction. 🟡 in a bead title did exactly that (rangerhq-53p), as
// did 🀄, 🃏 and 🈚. So this table has no holes to fall through: it is every
// code point a terminal draws two cells wide, which is East_Asian_Width
// Wide/Fullwidth ∪ Emoji_Modifier_Base.
//
// The second half of that union is not redundant. ☝ ⛹ ✌ ✍ 🏋 🕴 🖐 are
// East_Asian_Width Neutral and a terminal still gives them two cells, because
// a skin tone may follow — a width library that only reads EastAsianWidth.txt
// gets these wrong, and so did we.
//
// Widths were taken from the terminal's own cursor advance rather than another
// library's opinion — a pane printing the glyph, then `tmux display-message -p
// '#{cursor_x}'`. 60 code points on both sides of this change, 0 mismatches
// (tmux 3.7b, darwin 25.4.0). The narrow default stays: a regional indicator
// is one cell and a flag is a pair of them, so widening the unknown tail — the
// other shape this fix could have taken — would have counted 🇺🇸 as four.

// wideRanges are the code points a terminal draws two cells wide. Sorted and
// disjoint: wideRune binary-searches it, and TestCockpitWideRangesSorted holds
// that invariant.
var wideRanges = [...][2]rune{
	{0x1100, 0x115F}, {0x231A, 0x231B}, {0x2329, 0x232A}, {0x23E9, 0x23EC},
	{0x23F0, 0x23F0}, {0x23F3, 0x23F3}, {0x25FD, 0x25FE}, {0x2614, 0x2615},
	{0x261D, 0x261D}, {0x2648, 0x2653}, {0x267F, 0x267F}, {0x2693, 0x2693},
	{0x26A1, 0x26A1}, {0x26AA, 0x26AB}, {0x26BD, 0x26BE}, {0x26C4, 0x26C5},
	{0x26CE, 0x26CE}, {0x26D4, 0x26D4}, {0x26EA, 0x26EA}, {0x26F2, 0x26F3},
	{0x26F5, 0x26F5}, {0x26F9, 0x26FA}, {0x26FD, 0x26FD}, {0x2705, 0x2705},
	{0x270A, 0x270D}, {0x2728, 0x2728}, {0x274C, 0x274C}, {0x274E, 0x274E},
	{0x2753, 0x2755}, {0x2757, 0x2757}, {0x2795, 0x2797}, {0x27B0, 0x27B0},
	{0x27BF, 0x27BF}, {0x2B1B, 0x2B1C}, {0x2B50, 0x2B50}, {0x2B55, 0x2B55},
	{0x2E80, 0x2E99}, {0x2E9B, 0x2EF3}, {0x2F00, 0x2FD5}, {0x2FF0, 0x303E},
	{0x3041, 0x3096}, {0x3099, 0x30FF}, {0x3105, 0x312F}, {0x3131, 0x318E},
	{0x3190, 0x31E3}, {0x31EF, 0x321E}, {0x3220, 0x3247}, {0x3250, 0x4DBF},
	{0x4E00, 0xA48C}, {0xA490, 0xA4C6}, {0xA960, 0xA97C}, {0xAC00, 0xD7A3},
	{0xF900, 0xFAFF}, {0xFE10, 0xFE19}, {0xFE30, 0xFE52}, {0xFE54, 0xFE66},
	{0xFE68, 0xFE6B}, {0xFF01, 0xFF60}, {0xFFE0, 0xFFE6}, {0x16FE0, 0x16FE4},
	{0x16FF0, 0x16FF1}, {0x17000, 0x187F7}, {0x18800, 0x18CD5},
	{0x18D00, 0x18D08}, {0x1AFF0, 0x1AFF3}, {0x1AFF5, 0x1AFFB},
	{0x1AFFD, 0x1AFFE}, {0x1B000, 0x1B122}, {0x1B132, 0x1B132},
	{0x1B150, 0x1B152}, {0x1B155, 0x1B155}, {0x1B164, 0x1B167},
	{0x1B170, 0x1B2FB}, {0x1F004, 0x1F004}, {0x1F0CF, 0x1F0CF},
	{0x1F18E, 0x1F18E}, {0x1F191, 0x1F19A}, {0x1F200, 0x1F202},
	{0x1F210, 0x1F23B}, {0x1F240, 0x1F248}, {0x1F250, 0x1F251},
	{0x1F260, 0x1F265}, {0x1F300, 0x1F320}, {0x1F32D, 0x1F335},
	{0x1F337, 0x1F37C}, {0x1F37E, 0x1F393}, {0x1F3A0, 0x1F3CC},
	{0x1F3CF, 0x1F3D3}, {0x1F3E0, 0x1F3F0}, {0x1F3F4, 0x1F3F4},
	{0x1F3F8, 0x1F43E}, {0x1F440, 0x1F440}, {0x1F442, 0x1F4FC},
	{0x1F4FF, 0x1F53D}, {0x1F54B, 0x1F54E}, {0x1F550, 0x1F567},
	{0x1F574, 0x1F575}, {0x1F57A, 0x1F57A}, {0x1F590, 0x1F590},
	{0x1F595, 0x1F596}, {0x1F5A4, 0x1F5A4}, {0x1F5FB, 0x1F64F},
	{0x1F680, 0x1F6C5}, {0x1F6CC, 0x1F6CC}, {0x1F6D0, 0x1F6D2},
	{0x1F6D5, 0x1F6D7}, {0x1F6DC, 0x1F6DF}, {0x1F6EB, 0x1F6EC},
	{0x1F6F4, 0x1F6FC}, {0x1F7E0, 0x1F7EB}, {0x1F7F0, 0x1F7F0},
	{0x1F90C, 0x1F93A}, {0x1F93C, 0x1F945}, {0x1F947, 0x1F9FF},
	{0x1FA70, 0x1FA7C}, {0x1FA80, 0x1FA88}, {0x1FA90, 0x1FABD},
	{0x1FABF, 0x1FAC5}, {0x1FACE, 0x1FADB}, {0x1FAE0, 0x1FAE8},
	{0x1FAF0, 0x1FAF8}, {0x20000, 0x2FFFD}, {0x30000, 0x3FFFD},
}

// wideRune reports whether the terminal draws r in two cells. The ASCII
// short-circuit takes the common case out of the search entirely.
func wideRune(r rune) bool {
	if r < wideRanges[0][0] {
		return false
	}
	lo, hi := 0, len(wideRanges)-1
	for lo <= hi {
		mid := int(uint(lo+hi) >> 1)
		switch {
		case r < wideRanges[mid][0]:
			hi = mid - 1
		case r > wideRanges[mid][1]:
			lo = mid + 1
		default:
			return true
		}
	}
	return false
}

// cellScan turns a rune stream into per-rune cell deltas. It is a scanner
// rather than a pure function because three cases need the rune before:
// VS16 promotes it to emoji (⚙️ = 2 cells, ⚙ = 1), VS15 demotes it, and a
// zero-width joiner folds it into the glyph that follows (👨‍💻 = 2, not 4).
type cellScan struct{ prev int }

func (sc *cellScan) next(r rune) int {
	switch {
	case r == 0xFE0F: // variation selector-16: emoji presentation
		if sc.prev == 1 {
			sc.prev = 2
			return 1
		}
		return 0
	case r == 0xFE0E: // variation selector-15: text presentation
		if sc.prev == 2 {
			sc.prev = 1
			return -1
		}
		return 0
	case r == 0x200D: // zero-width joiner
		d := -sc.prev
		sc.prev = 0
		return d
	case r >= 0x1F3FB && r <= 0x1F3FF, // skin tone modifiers
		r >= 0x0300 && r <= 0x036F, // combining marks
		r >= 0x20D0 && r <= 0x20FF:
		sc.prev = 0
		return 0
	case wideRune(r):
		sc.prev = 2
		return 2
	}
	sc.prev = 1
	return 1
}

// dispWidth is the terminal cells s occupies. Every width in the layout is
// measured with it — padding, truncation, and the viewport's own clip.
func dispWidth(s string) int {
	var sc cellScan
	n := 0
	for _, r := range s {
		n += sc.next(r)
	}
	return n
}

func padCells(s string, n int) string {
	if d := n - dispWidth(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// truncCells clips s to n cells, marking the cut with … (ADR 0004 §1). A
// wide glyph is never split: the cut lands before it, leaving a cell of
// slack rather than half a character.
func truncCells(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if dispWidth(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	var sc cellScan
	w, cut := 0, 0
	for i, r := range s {
		d := sc.next(r)
		if d > 0 && w+d > n-1 { // no room left once … is paid for
			break
		}
		w += d
		cut = i + utf8.RuneLen(r)
	}
	return s[:cut] + "…"
}

func paint(s, ansi string) string {
	if ansi == "" || s == "" {
		return s
	}
	return ansi + s + aRst
}

// layout draws one row's columns into w columns: droppables go first (they
// are dropped whole, never squeezed), the fixed columns take their natural
// width, and the single flex column gets whatever is left.
func layout(cols []col, w int) string {
	keep := make([]col, 0, len(cols))
	for _, cl := range cols {
		at := cl.drop
		if at == 0 && cl.kind == colDrop {
			at = dropAt
		}
		if at > 0 && w < at {
			continue
		}
		keep = append(keep, cl)
	}
	if len(keep) == 0 {
		return ""
	}
	fixed, flexAt := 0, -1
	for i, cl := range keep {
		if cl.kind == colFlex && flexAt < 0 {
			flexAt = i
			continue
		}
		n := dispWidth(cl.text)
		if cl.pad > n {
			n = cl.pad
		}
		fixed += n
	}
	flexW := w - fixed - (len(keep) - 1)

	var b strings.Builder
	used := 0
	for i, cl := range keep {
		if i > 0 {
			if used+1 >= w {
				break
			}
			b.WriteString(" ")
			used++
		}
		want := dispWidth(cl.text)
		if cl.pad > want {
			want = cl.pad
		}
		if i == flexAt {
			want = flexW
		}
		if want < 0 {
			want = 0
		}
		// A fixed column is never truncated by the layout — only by the
		// terminal edge, which nothing can help.
		if want > w-used {
			want = w - used
		}
		text := truncCells(cl.text, want)
		if i != len(keep)-1 {
			text = padCells(text, want)
		}
		b.WriteString(paint(text, cl.ansi))
		used += dispWidth(text)
	}
	return b.String()
}

// renderRow draws one row. The cursor marker is prepended as a column, not
// as a prefix, so the drop thresholds are measured against the terminal's
// real width rather than two columns short of it.
func renderRow(r row, w int, selected bool) string {
	if r.kind != rowItem {
		return layout(r.cols, w)
	}
	mark := col{text: " "}
	if selected {
		mark = col{text: "▸", ansi: aRev}
	}
	return layout(append([]col{mark}, r.cols...), w)
}

// sessionCols is a session row: mark · emoji · status · name+persona (flex)
// · cost and (focused) as droppable context.
func (c *cockpit) sessionCols(s rhq.HerdrSession) []col {
	mark, color := "·", aDim
	switch s.Status {
	case "blocked":
		mark, color = "⛔", aRed
	case "working":
		mark, color = "⚡", aGrn
	case "idle", "done":
		mark, color = "○", aYlw
	}
	status := s.Status
	if s.TurnFailure != "" {
		mark, color, status = "⛔", aRed, "failed"
	}
	if status == "" {
		status = "shell"
	}
	name := s.Name
	if s.Agent != "" {
		name += " 🎭" + s.Agent + c.app.RuntimeTierTag(s.Runtime, s.Tier)
		// The cage above the default tier, and the host sockets it was
		// opened for: `container+herdr` says the session is caged AND that
		// it holds a capability over the rest of the herd (ADR 0002 §3).
		if tag := rhq.CageTag(s.Cage, s.Sockets); tag != "" {
			name += " " + tag
		}
		if s.Degraded != "" {
			name += " ⚠️degraded"
		}
		// The tag above says the session runs at the tier it names; this one
		// says that tier is not the one its PID asked for (rangerhq-oay).
		if s.Fallback != "" {
			name += " " + rhq.FallbackTag
		}
		if s.TurnFailure != "" {
			name += " " + rhq.TurnFailureTag
		}
	}
	if s.Crew {
		name += " " + rhq.CrewTag
	}
	cols := []col{
		{text: mark},
		{text: s.Emoji, pad: 1},
		{text: status, pad: 8, ansi: color},
		{kind: colFlex, text: name},
	}
	if cost := c.sessionCost(s); cost != "" {
		cols = append(cols, col{kind: colDrop, text: cost, ansi: aDim})
	}
	if s.Focused {
		cols = append(cols, col{kind: colDrop, text: "(focused)", ansi: aDim})
	}
	return cols
}

// issueCols is a ready-work row: id · priority · holder · title (flex) ·
// repo dir as droppable context.
func issueCols(is rhq.RepoIssue) []col {
	who := is.Assignee
	if who == "" {
		who = "unassigned"
	}
	return []col{
		{text: is.ID, pad: 14},
		{text: fmt.Sprintf("p%d", is.Priority)},
		{text: who, pad: 12, ansi: aDim, drop: dropHolderAt},
		{kind: colFlex, text: is.Title},
		{kind: colDrop, text: rhq.AbbrevHome(is.Dir), ansi: aDim},
	}
}

// holderAnsi paints the holder-state the way the sessions section paints
// the same words, so one glance reads both: red is blocked, green is
// working, yellow is anything nobody is currently typing into.
func holderAnsi(state string) string {
	switch state {
	case "blocked":
		return aRed
	case "working":
		return aGrn
	case noSession, "shell", "idle", "done":
		return aYlw
	}
	return aDim
}

// shortAge is how long ago t was in one unit — 45s, 3m, 2h, 1d (ADR 0004
// §2: age since the bead's updated_at). No timestamp shows "-": the column
// keeps its width and claims nothing it does not know.
func shortAge(now, t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// inprogCols is a claimed-bead row (ADR 0004 §2): id · p · holder ·
// holder-state · age · title (flex) · repo dir as droppable context. The
// holder *name* drops with the sessions' holder column below dropHolderAt;
// the state does not — it is the stall signal this whole section exists to
// show, and it costs at most ten cells.
func (c *cockpit) inprogCols(is rhq.RepoIssue) []col {
	who := is.Assignee
	if who == "" {
		who = "unassigned"
	}
	state := c.holderState(is)
	return []col{
		{text: is.ID, pad: 14},
		{text: fmt.Sprintf("p%d", is.Priority)},
		{text: who, pad: 12, ansi: aDim, drop: dropHolderAt},
		{text: state, pad: 10, ansi: holderAnsi(state)},
		{text: shortAge(c.clock(), is.Updated), pad: 3, ansi: aDim},
		{kind: colFlex, text: is.Title},
		{kind: colDrop, text: rhq.AbbrevHome(is.Dir), ansi: aDim},
	}
}

// govHeading is the block's own summary, and it says PARTIAL when a store
// could not be read — an unreadable store is not an all-clear, and a count
// rendered without that word would read as one.
func (c *cockpit) govHeading() string {
	h := rhq.GovSummary(c.gov)
	if c.govFailed > 0 {
		h += fmt.Sprintf(" · partial, %d store(s) unread", c.govFailed)
	}
	return h
}

// govSegment is the header's governance segment, and the residual witness
// the governance-surface ADR §2 names: "a dead loop pulses nobody, and the
// residual witness is the operator's glance at the cockpit header."
//
// The GOVERNANCE block below is the detail, and it is not enough on its own
// for that job: it is body, so it scrolls out of the viewport the moment the
// cursor walks down into READY WORK, and there is no key that scrolls back
// to it. The header does not scroll. So the header carries the answer and
// the block carries the reasons (bead rangerhq-mgvx).
//
// Three states, kept apart for planSegment's reason — an empty segment is
// indistinguishable from "this rendering does not do governance", which is
// exactly the silence a dead loop must not be able to hide in:
//
//	gov …            no scan has landed yet. Unknown, not clear.
//	gov clear        the check ran and found nothing.
//	gov 1 URGENT …   the summary, plus PARTIAL when a store could not be read.
//
// G7 is named rather than counted. Every other row in that count is a
// condition somebody still has to be told about, and the loop is what tells
// them: "1 URGENT" with delivery dead understates by exactly the row that
// matters most. The block spells out which lock is free.
func (c *cockpit) govSegment() string {
	if c.govAt.IsZero() {
		return "gov …"
	}
	seg := "gov " + rhq.GovSummary(c.gov)
	if c.gov.Has("G7") {
		seg += " · loop dead"
	}
	if c.govFailed > 0 {
		seg += " · partial"
	}
	return seg
}

// govCols is one condition's row: class, G-row, and the detail in the flex
// column. URGENT is red because the shop is stopped; LANE is plain because
// the rest of the shop is still flowing and a colour that shouts at both
// tells the eye nothing.
func govCols(g rhq.GovCondition) []col {
	color := ""
	if g.Class == rhq.GovUrgent {
		color = aRed
	}
	return []col{
		{kind: colFixed, text: "  " + g.Class, pad: 9, ansi: color},
		{kind: colFixed, text: g.Row(), pad: 4, ansi: aDim},
		{kind: colFlex, text: g.Detail},
	}
}

// buildRows turns the three lists into the flat row model. Item rows carry
// their index in cursor space, so the cursor keeps meaning what it always
// meant (sessions, then issues) and reselect (rangerhq-5li) is untouched.
func (c *cockpit) buildRows() {
	rows := make([]row, 0, c.items()+8)
	heading := func(text string, sec section) row {
		return row{kind: rowHeading, sec: sec, cols: []col{{text: text, ansi: aDim}}}
	}
	none := func(sec section) row {
		return row{kind: rowFiller, sec: sec, cols: []col{{text: "  (none)", ansi: aDim}}}
	}
	// One pass per section, in cursor order; item rows carry their index in
	// cursor space, so the cursor keeps meaning what it always meant and
	// reselect (rangerhq-5li) needs nothing from the row model.
	// GOVERNANCE, above everything, and only when it has something to say —
	// a clear shop must not spend two lines of a 20-line popup saying so,
	// and the header already carries the shop's other standing numbers.
	// These rows are filler: they are not in cursor space (see the field's
	// comment), so tab, reselect and every key below are untouched.
	if len(c.gov) > 0 || c.govFailed > 0 {
		rows = append(rows, heading(fmt.Sprintf("GOVERNANCE (%s)", c.govHeading()), secSessions))
		for _, g := range rhq.GovOrdered(c.gov) {
			rows = append(rows, row{kind: rowFiller, sec: secSessions, cols: govCols(g)})
		}
		rows = append(rows, row{kind: rowFiller, sec: secSessions})
	}
	rows = append(rows, heading(fmt.Sprintf("SESSIONS (%d)", len(c.sessions)), secSessions))
	if len(c.sessions) == 0 {
		rows = append(rows, none(secSessions))
	}
	for i, s := range c.sessions {
		rows = append(rows, row{kind: rowItem, sec: secSessions, item: i, cols: c.sessionCols(s)})
	}
	rows = append(rows, row{kind: rowFiller, sec: secInProg})
	rows = append(rows, heading(fmt.Sprintf("IN PROGRESS (%d)", len(c.inprog)), secInProg))
	if len(c.inprog) == 0 {
		rows = append(rows, none(secInProg))
	}
	for i, is := range c.inprog {
		rows = append(rows, row{kind: rowItem, sec: secInProg,
			item: len(c.sessions) + i, cols: c.inprogCols(is)})
	}
	rows = append(rows, row{kind: rowFiller, sec: secIssues})
	rows = append(rows, heading(fmt.Sprintf("READY WORK (%d)", len(c.issues)), secIssues))
	if len(c.issues) == 0 {
		rows = append(rows, none(secIssues))
	}
	for i, is := range c.issues {
		rows = append(rows, row{kind: rowItem, sec: secIssues,
			item: len(c.sessions) + len(c.inprog) + i, cols: issueCols(is)})
	}
	c.rows = rows
}

// ─── scrolling (ADR 0004 §4) ─────────────────────────────────────────────────

// viewportH is the number of body lines for a terminal of h lines; 0 means
// unbounded (the non-tty fallback renders at 80×∞).
//
// h−5 is ADR 0004 §4's promise and holds down to h=6. Below that the floor
// wins and the chrome yields instead (chromeFor): a pane too short for the
// header and footer still shows the row the cursor is on, which is the one
// thing the operator came for.
func viewportH(h int) int {
	if h <= 0 {
		return 0
	}
	if n := h - chromeLines; n > minViewport {
		return n
	}
	return minViewport
}

// chromeFor splits the lines left over after the viewport between the header
// and the footer. The full 2+3 whenever it fits, then shed one at a time in
// the order of what carries the least: the header's blank spacer, the cost
// line, the status line, and last the title — so the footer's final line
// (keys, prompt, or the kill/unclaim question) and one body row survive to
// h=1.
func chromeFor(budget int) (head, foot int) {
	switch {
	case budget >= chromeLines:
		return 2, 3
	case budget == 4:
		return 1, 3
	case budget == 3:
		return 1, 2
	case budget == 2:
		return 1, 1
	case budget == 1:
		return 0, 1
	}
	return 0, 0
}

// visible reports how many rows are drawn at offset and whether the ↑/↓ more
// markers are shown — each marker costs a viewport line, and a marker only
// gets one while a row is still left over. A viewport that spends itself
// entirely on "there is more above and below" says nothing (rangerhq-5qm).
func visible(offset, total, vh int) (n int, up, down bool) {
	if vh <= 0 {
		return total - offset, false, false
	}
	n = vh
	if offset > 0 && n > minViewport {
		up, n = true, n-1
	}
	if offset+n < total && n > minViewport {
		down, n = true, n-1
	}
	if n < 0 {
		n = 0
	}
	if offset+n > total {
		n = total - offset
	}
	return n, up, down
}

// scrollTo keeps the cursor row inside the viewport with a 2-row margin. It
// budgets a line for each more-marker whether or not both are drawn: one
// spare row is cheaper than an offset that oscillates with its own markers.
func scrollTo(offset, cursorRow, total, vh int) int {
	if vh <= 0 || total <= vh {
		return 0
	}
	n := vh - 2
	if n < 1 {
		n = 1
	}
	margin := scrollMargin
	if max := (n - 1) / 2; margin > max {
		margin = max
	}
	if cursorRow-margin < offset {
		offset = cursorRow - margin
	}
	if cursorRow+margin > offset+n-1 {
		offset = cursorRow - n + 1 + margin
	}
	if offset > total-n {
		offset = total - n
	}
	// At the bottom the ↓ marker is gone, so one more row fits: snap the
	// window down to fill it rather than leaving a blank line under the
	// last row. (The cursor stays inside — offset only ever shrinks here.)
	if fill := total - (vh - 1); offset > fill {
		offset = fill
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

// pageStep is how far ctrl-d/ctrl-u move: one viewport, or a screenful's
// worth when the height is unknown (before the first draw, or non-tty).
func (c *cockpit) pageStep() int {
	if n := viewportH(c.height); n > 0 {
		return n
	}
	return 10
}

// cursorRow is where the cursor sits in the row model (0 when nothing is
// selectable — an empty cockpit points at the first heading).
func (c *cockpit) cursorRow() int {
	for i, r := range c.rows {
		if r.kind == rowItem && r.item == c.cursor {
			return i
		}
	}
	return 0
}

// moveRows walks the cursor n rows through the model and lands on the
// nearest item row — headings and markers are not selectable.
func (c *cockpit) moveRows(n int) {
	target := c.cursorRow() + n
	best, bestDist := c.cursor, 1<<30
	for i, r := range c.rows {
		if r.kind != rowItem {
			continue
		}
		d := i - target
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			best, bestDist = r.item, d
		}
	}
	c.cursor = best
}

// ─── drawing ─────────────────────────────────────────────────────────────────

// planSuffix appends the plan windows to the header when we have a reading.
// headerCols is the top line's columns, in drawing order. It is a function
// rather than a literal inside renderLines because whether the credential
// column EXISTS is a rule and not a rendering, and a rule that only shows up
// as a byte or two of padding at one terminal width is a rule no test
// notices being deleted.
func (c *cockpit) headerCols() []col {
	cols := []col{
		{text: "🤠 posse", ansi: aBold},
		{kind: colFlex, text: c.clock().Format("15:04:05") + " · " + rhq.VersionString() + planSuffix(c.planLine), ansi: aDim},
	}
	// The credential warning goes between them, fixed, and ONLY when there
	// is one. Appended unconditionally it would spend a separator cell of
	// the flex column on every frame of every day the shop is healthy —
	// invisible on a wide pane, one character off the plan reading on a
	// narrow one — and a header permanently narrower to make room for a
	// fortnight every few months is the wrong trade (ADR 0019 D5).
	if cd, ok := credCol(c.creds, c.clock()); ok {
		cols = append(cols, cd)
	}
	// A FIXED column, at the right edge, deliberately: the flex column
	// truncates from its tail, so a governance segment appended there is
	// the first thing an 80-column pane throws away — and the row it throws
	// away is the one that says nothing is being delivered. Fixed, it costs
	// the clock and the version instead, which is the correct exchange.
	return append(cols, col{text: c.govSegment(), ansi: aDim})
}

// credCol is the header's credential-expiry segment (ADR 0019 D5): the
// posse-owned session mint that dies soonest, once it is inside the window.
// ok is false when there is nothing to say, and nothing to say is the
// normal state — see renderLines for why that matters.
//
// It shows ONE, with a count of the rest. A header column is glanced at,
// not read: what it owes the operator is that something needs re-minting
// and roughly when, and `posse refresh` is four keystrokes away with the
// dates, the env sets and the verb.
//
// Red for already-expired and yellow for approaching, on govCols' rule: the
// two states cost different things, and one colour for both tells the eye
// nothing. Neither is a stop — the shop below the header is running, and
// expiry parks nothing.
func credCol(ex []rhq.CredExpiry, now time.Time) (col, bool) {
	if len(ex) == 0 {
		return col{}, false
	}
	e := ex[0]
	text := "cred: " + e.Runtime + " " + e.Brief(now)
	if n := len(ex) - 1; n > 0 {
		text += fmt.Sprintf(" +%d", n)
	}
	ansi := aYlw
	if e.Expired(now) {
		ansi = aRed
	}
	return col{text: text, ansi: ansi}, true
}

func planSuffix(line string) string {
	if line == "" {
		return ""
	}
	return " · " + line
}

// size is the terminal we are drawing into. A non-tty (tests, pipes) gets
// 80×∞: unbounded height means no scrolling and no more-markers.
func (c *cockpit) size() (w, h int) {
	for _, fd := range []int{int(os.Stdout.Fd()), int(os.Stdin.Fd())} {
		if w, h, err := term.GetSize(fd); err == nil && w > 0 && h > 0 {
			return w, h
		}
	}
	return 80, 0
}

func (c *cockpit) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *cockpit) draw() {
	w, h := c.size()
	c.width, c.height = w, h
	c.buildRows()
	c.offset = scrollTo(c.offset, c.cursorRow(), len(c.rows), viewportH(h))
	fmt.Fprint(c.out, c.render(w, h))
}

// render is the whole view as a string, a pure function of (rows, cursor,
// offset, mode, status) and the size it is asked for (ADR 0004 §5).
func (c *cockpit) render(w, h int) string {
	var b strings.Builder
	b.WriteString("\033[2J\033[H")
	if c.mode == modePrompt {
		b.WriteString("\033[?25h") // the prompt line is last: the cursor lands on it
	} else {
		b.WriteString("\033[?25l")
	}
	b.WriteString(strings.Join(c.renderLines(w, h), "\r\n"))
	return b.String()
}

func (c *cockpit) renderLines(w, h int) []string {
	header := []string{layout(c.headerCols(), w), ""}
	footer := c.footerLines(w)
	vh := viewportH(h)
	head, foot := len(header), len(footer)
	if h > 0 {
		head, foot = chromeFor(h - vh)
	}
	lines := append([]string{}, header[:head]...)
	if c.mode == modePeek {
		lines = append(lines, c.peekLines(w, vh)...)
	} else {
		lines = append(lines, c.bodyLines(w, vh)...)
	}
	return append(lines, trimFooter(footer, foot)...)
}

// trimFooter keeps foot of the footer's three lines, dropping the cost line
// before the status line: on a pane this short, "did my keypress land?" beats
// today's spend, which is standing background either way.
func trimFooter(footer []string, foot int) []string {
	status, cost, keys := footer[0], footer[1], footer[2]
	switch foot {
	case 0:
		return nil
	case 1:
		return []string{keys}
	case 2:
		return []string{status, keys}
	}
	return []string{status, cost, keys}
}

// bodyLines is the viewport over the row model: a window of vh lines, with a
// line spent on each more-marker that is showing.
func (c *cockpit) bodyLines(w, vh int) []string {
	total := len(c.rows)
	n, up, down := visible(c.offset, total, vh)
	out := make([]string, 0, vh)
	if up {
		out = append(out, paint(truncCells(fmt.Sprintf("  ↑ %d more", c.offset), w), aDim))
	}
	for i := c.offset; i < c.offset+n && i < total; i++ {
		r := c.rows[i]
		out = append(out, renderRow(r, w, r.kind == rowItem && r.item == c.cursor))
	}
	if down {
		out = append(out, paint(truncCells(fmt.Sprintf("  ↓ %d more", total-c.offset-n), w), aDim))
	}
	return fitLines(out, vh)
}

// peekLines shows the tail of a pane inside the same viewport, clipped to it
// rather than appended below the list (ADR 0004 §4).
func (c *cockpit) peekLines(w, vh int) []string {
	out := []string{paint(truncCells("── peek (any key to return) ──", w), aDim)}
	if vh == minViewport {
		// One line: spend it on the pane. The footer already says "any key
		// returns", so the banner is the redundant half of the pair.
		out = out[:0]
	}
	body := strings.Split(strings.TrimRight(c.peekText, "\n"), "\n")
	if room := vh - len(out); vh > 0 && len(body) > room {
		body = body[len(body)-room:]
	}
	for _, ln := range body {
		out = append(out, truncCells(ln, w))
	}
	return fitLines(out, vh)
}

// fitLines makes the body exactly vh lines: padding anchors the footer at the
// bottom of a sized terminal, clipping keeps a viewport that was handed more
// than it has room for from pushing the footer off the screen. An unbounded
// height (non-tty) is left as-is.
func fitLines(lines []string, vh int) []string {
	if vh <= 0 {
		return lines
	}
	for len(lines) < vh {
		lines = append(lines, "")
	}
	return lines[:vh]
}

// footerLines is always three lines; renderLines keeps as many as the
// terminal can afford (chromeFor, trimFooter), which is all three above h=5.
func (c *cockpit) footerLines(w int) []string {
	name := ""
	if s := c.actSession(); s != nil {
		name = s.Name
	}
	switch c.mode {
	case modePrompt:
		return []string{"", "", layout([]col{
			{text: "prompt " + name + " ›", ansi: aBold},
			{kind: colFlex, text: string(c.input)},
		}, w)}
	case modeConfirm:
		ask := fmt.Sprintf("kill %s? (y/n)", name)
		if c.confirm == confirmUnclaim {
			id := ""
			if is := c.selInProg(); is != nil {
				id = is.ID
			}
			ask = fmt.Sprintf("unclaim %s? (y/n)", id)
		}
		return []string{"", "", layout([]col{{kind: colFlex, text: ask, ansi: aRed}}, w)}
	case modePeek:
		return []string{"", "", layout([]col{{kind: colFlex, text: "any key returns", ansi: aDim}}, w)}
	}
	status := ""
	if c.status != "" {
		status = layout([]col{{kind: colFlex, text: c.status, ansi: aYlw}}, w)
	}
	cost := ""
	if !c.costAt.IsZero() {
		unc := ""
		if c.costUncounted > 0 {
			if which := strings.Join(c.costUncountedRuntimes, "/"); which != "" {
				unc = fmt.Sprintf(" · %d %s session(s) uncounted", c.costUncounted, which)
			} else {
				unc = fmt.Sprintf(" · %d session(s) uncounted (no adapter)", c.costUncounted)
			}
		}
		// ADR 0018 §3: transcripts the scan could not read are spend that is
		// missing from this number, not spend that did not happen — so the
		// day total and the percentage computed from it are a floor. The
		// marker rides in FRONT of both, because this line is one flex
		// column and a narrow terminal truncates it from the right; the
		// count follows at the end, where losing it costs only the detail.
		ge, floor := "", ""
		if c.costUnread > 0 {
			ge = "≥"
			floor = fmt.Sprintf(" · %d transcript(s) unreadable — a floor, not a total", c.costUnread)
		}
		cap_ := ""
		if c.costDayCap > 0 {
			// Dial E's day window, in the operator's eye before dispatch
			// acts on it: 80% is where standard starts stepping down.
			cap_ = fmt.Sprintf(" of $%.0f budget_day (%s%.0f%%)", c.costDayCap, ge, 100*c.costToday/c.costDayCap)
		}
		cost = layout([]col{{kind: colFlex, ansi: aDim, text: fmt.Sprintf(
			"today %s$%.2f%s api-equiv (counted runtimes, beads only; refreshed %s)%s%s",
			ge, c.costToday, cap_, c.costAt.Format("15:04:05"), unc, floor)}}, w)
	}
	// ADR 0004 §3: the footer offers the selected section's keys only —
	// `c` on a session does nothing, and `x` on a bead has nothing to kill.
	line := ""
	switch c.selSection() {
	case secInProg:
		line = "enter focus holder · p prompt · v peek · d resume · u unclaim · tab section · q quit"
	case secIssues:
		line = "c claim · d dispatch · tab section · q quit"
	default:
		crewKey := ""
		if c.selSession() != nil {
			crewKey = "o crew/fleet · "
		}
		line = "enter focus · p prompt · v peek · " + crewKey + "x kill · tab section · q quit"
	}
	keys := layout([]col{{kind: colFlex, ansi: aDim, text: line}}, w)
	return []string{status, cost, keys}
}

// displayOnly is the non-tty fallback: the refresh loop, drawing the same
// row model at 80 wide (ADR 0004 §5).
//
// It scans governance on the tty path's own cadence, because it draws the
// tty path's own rows. Without it this rendering answered every governance
// question with the zero value — no block, and a header that would say
// nothing at all — so a fleet whose cockpit is piped or logged (the pane is
// a pipe often enough) reported a dead watch loop as an all-clear. Measured
// on the scratch rig before the fix: `posse status` said `URGENT G7` and
// exited 1 while this loop drew a clean shop (bead rangerhq-mgvx).
//
// refresh() stays synchronous — it is this loop's whole job — but the shop
// check talks to bd and the kernel and takes seconds, so it runs off the
// draw path exactly as it does under the tty, and the header says `gov …`
// until the first one lands.
func (c *cockpit) displayOnly() error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	govTick := time.NewTicker(govEvery)
	defer govTick.Stop()
	go c.scanGov()
	for {
		c.displayFrame()
		select {
		case <-stop:
			return nil
		case <-govTick.C:
			go c.scanGov()
		case <-tick.C:
		}
	}
}

// displayFrame is one non-tty frame: re-read the stores this loop reads
// synchronously, take whatever the last shop check left on the channel, and
// draw. Split out of displayOnly so the drain is testable without a signal
// — the frame, not the loop, is where a landed check becomes a drawn line.
func (c *cockpit) displayFrame() {
	c.refresh()
	c.takeGov()
	fmt.Fprint(c.out, "\033[2J\033[H")
	c.drawPlain()
}

// takeGov applies the last shop check to have landed, if one has. Never
// waits: a check still running leaves govAt where it was, and the header
// keeps saying `gov …` rather than drawing an all-clear it has not earned.
func (c *cockpit) takeGov() bool {
	select {
	case g := <-c.govs:
		c.applyGov(g)
		return true
	default:
		return false
	}
}

// applyGov is where a landed check becomes the drawn answer, for both
// loops. govAt is stamped here and nowhere else: it is what separates "the
// check found nothing" from "no check has run", and a path that set the set
// without stamping it would render an all-clear it never earned.
func (c *cockpit) applyGov(g govRead) {
	c.gov, c.govFailed, c.govAt = g.set, g.failed, time.Now()
}

func (c *cockpit) drawPlain() {
	c.buildRows()
	fmt.Fprintln(c.out, strings.Join(c.renderLines(80, 0), "\n"))
}
