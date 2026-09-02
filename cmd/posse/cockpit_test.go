package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

// rangerhq-5li: the cursor follows the item, not the row index, across
// refreshes that reshuffle the lists.
func TestCockpitReselect(t *testing.T) {
	ss := func(names ...string) []posse.HerdrSession {
		var out []posse.HerdrSession
		for _, n := range names {
			out = append(out, posse.HerdrSession{Name: n})
		}
		return out
	}
	is := func(ids ...string) []posse.RepoIssue {
		var out []posse.RepoIssue
		for _, id := range ids {
			out = append(out, posse.RepoIssue{BdIssue: posse.BdIssue{ID: id}, Dir: "/r"})
		}
		return out
	}
	c := &cockpit{sessions: ss("a", "b"), issues: is("x", "y", "z")}

	// A dispatch adds a session row: the selected bead stays selected.
	c.cursor = 3 // issue "y"
	sel := c.selected()
	if got := reselect(ss("a", "b", "new"), nil, is("x", "y", "z"), sel); got != 4 {
		t.Errorf("bead y after a session was added: cursor %d, want 4", got)
	}
	// Sessions re-sort blocked-first: the selected session follows.
	c.cursor = 1 // session "b"
	sel = c.selected()
	if got := reselect(ss("b", "a"), nil, c.issues, sel); got != 0 {
		t.Errorf("session b after re-sort: cursor %d, want 0", got)
	}
	// The selected bead closed: same section, same offset (clamped).
	c.cursor = 4 // issue "z", last
	sel = c.selected()
	if got := reselect(c.sessions, nil, is("x", "y"), sel); got != 3 {
		t.Errorf("last bead vanished: cursor %d, want 3 (still in READY WORK)", got)
	}
	c.cursor = 3 // issue "y", middle
	sel = c.selected()
	if got := reselect(c.sessions, nil, is("x", "z"), sel); got != 3 {
		t.Errorf("middle bead vanished: cursor %d, want 3 (next bead)", got)
	}
	// All beads gone: fall back to sessions; empty everything → 0.
	if got := reselect(c.sessions, nil, nil, sel); got != 0 {
		t.Errorf("issues section vanished: cursor %d, want 0", got)
	}
	if got := reselect(nil, nil, nil, sel); got != 0 {
		t.Errorf("empty lists: cursor %d, want 0", got)
	}
	// Nothing was selected (empty cockpit before): stays at 0.
	e := &cockpit{}
	if got := reselect(ss("a"), nil, is("x"), e.selected()); got != 0 {
		t.Errorf("empty-before: cursor %d, want 0", got)
	}

	// ADR 0004 §2: the third section shifts READY WORK down, and a bead
	// that gets claimed under the cursor is followed into IN PROGRESS
	// rather than dropped (issueKey is shared by both bead sections).
	c.cursor = 2 // issue "x", first ready bead
	sel = c.selected()
	if got := reselect(c.sessions, is("w"), c.issues, sel); got != 3 {
		t.Errorf("bead x with one in-progress row above: cursor %d, want 3", got)
	}
	if got := reselect(c.sessions, is("x"), is("y", "z"), sel); got != 2 {
		t.Errorf("bead x after it was claimed: cursor %d, want 2 (IN PROGRESS)", got)
	}
	// An in-progress bead that closed: same section, same offset (clamped).
	c.inprog = is("m", "n")
	c.cursor = 3 // in-progress "n"
	sel = c.selected()
	if sel.sec != secInProg {
		t.Fatalf("cursor %d reads as section %d, want IN PROGRESS", c.cursor, sel.sec)
	}
	if got := reselect(c.sessions, is("m"), c.issues, sel); got != 2 {
		t.Errorf("last in-progress bead vanished: cursor %d, want 2", got)
	}
	if got := reselect(c.sessions, nil, c.issues, sel); got != 0 {
		t.Errorf("IN PROGRESS emptied: cursor %d, want 0", got)
	}
}

// rangerhq-llse: a repo whose bd call failed has an unknown queue, not a
// shorter READY list. refresh puts that on the status line so the operator
// sees the miss, not a quiet cockpit that looks caught up.
func TestCockpitReadyScanFailureIsAStatus(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	herdr := filepath.Join(binDir, "herdr")
	if err := os.WriteFile(herdr, []byte(`#!/bin/sh
if [ "$1" = "workspace" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"result":{"workspaces":[]}}'
  exit 0
fi
if [ "$1" = "agent" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"result":{"agents":[]}}'
  exit 0
fi
printf '%s\n' '{"error":{"code":"no","message":"unexpected '"$1 $2"'"}}'
exit 1
`), 0o755); err != nil {
		t.Fatal(err)
	}
	bd := filepath.Join(binDir, "bd")
	if err := os.WriteFile(bd, []byte("#!/bin/sh\necho 'database is locked' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := &posse.App{
		Home:       home,
		ConfigPath: filepath.Join(home, "config.yaml"),
		StateDir:   filepath.Join(home, "state"),
	}
	c := &cockpit{
		app: a,
		hb:  &posse.HerdrBackend{App: a, H: posse.Herdr{Bin: herdr}, Warn: io.Discard},
		bd:  posse.Bd{Bin: bd},
	}
	c.refreshAll()
	if !strings.Contains(c.status, "ready scan failed") || !strings.Contains(c.status, "database is locked") {
		t.Fatalf("status must name the failed scan, got %q", c.status)
	}
	foot := stripANSI(strings.Join(c.footerLines(120), "\n"))
	if !strings.Contains(foot, "ready scan failed") {
		t.Errorf("the status must reach the footer:\n%s", foot)
	}
}

// rangerhq-jgm: the plan's rate windows sit in the header when we have a
// reading, and the header is unchanged when we do not (an unreadable
// keychain or endpoint shows nothing, never a wrong number).
func TestCockpitPlanHeader(t *testing.T) {
	var b strings.Builder
	c := &cockpit{out: &b, planLine: "5h 42% · 7d 61%"}
	c.drawPlain()
	if got := b.String(); !strings.Contains(got, "5h 42% · 7d 61%") {
		t.Errorf("header without the plan windows:\n%s", got)
	}
	b.Reset()
	c.planLine = ""
	c.drawPlain()
	if got := b.String(); strings.Contains(got, "5h") || strings.Contains(got, " · ·") {
		t.Errorf("no reading must leave the header clean:\n%s", got)
	}
}

// ADR 0008: a crew session wears 👤, and the toggle is only offered where
// it does something — on a session row, never on a bead row.
func TestCockpitCrewTagAndFooter(t *testing.T) {
	var b strings.Builder
	c := &cockpit{out: &b,
		sessions: []posse.HerdrSession{
			{Name: "developer-rangerhq-b3p", Agent: "developer", Status: "idle", Crew: true},
			{Name: "devops-rangerhq-h3n", Agent: "devops", Status: "idle"},
		},
		issues: []posse.RepoIssue{{BdIssue: posse.BdIssue{ID: "x-1"}, Dir: "/r"}},
	}
	c.draw()
	for _, ln := range strings.Split(b.String(), "\r\n") {
		if strings.Contains(ln, "developer") && !strings.Contains(ln, posse.CrewTag) {
			t.Errorf("crew row untagged: %q", ln)
		}
		if strings.Contains(ln, "devops") && strings.Contains(ln, posse.CrewTag) {
			t.Errorf("fleet row wearing the crew tag: %q", ln)
		}
	}
	if !strings.Contains(b.String(), "o crew/fleet") {
		t.Errorf("session row must offer the toggle:\n%s", b.String())
	}

	b.Reset()
	c.cursor = len(c.sessions) // the bead row
	c.draw()
	if strings.Contains(b.String(), "o crew/fleet") {
		t.Errorf("bead row must not offer the toggle:\n%s", b.String())
	}
}

func TestCockpitTurnFailureOverridesIdlePresentation(t *testing.T) {
	c := &cockpit{sessions: []posse.HerdrSession{{
		Name: "security-posse-6ne", Agent: "security", Status: "idle",
		TurnFailure: "You've reached your Fable 5 limit.",
	}}}
	got := stripANSI(renderRow(row{kind: rowItem, cols: c.sessionCols(c.sessions[0])}, 100, false))
	if !strings.Contains(got, "⛔") || !strings.Contains(got, "failed") ||
		!strings.Contains(got, posse.TurnFailureTag) || strings.Contains(got, "idle") {
		t.Errorf("turn failure rendered as a healthy idle session: %q", got)
	}
}

// ─── ADR 0004: the row model, width-aware columns, scrolling ────────────────

var updateGolden = flag.Bool("update", false, "rewrite the cockpit golden files")

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]")

// The goldens are stored without ANSI so a diff is readable; colours get
// their own assertion in TestCockpitRenderColours.
func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// fixtureBeadID is a synthetic id for a fixture's ready-work row: nothing
// resolves it, unlike the rangerhq-h3n/fei/a1a rows above, which are real
// beads. The `x` keeps the source literal in the inert `posse-<id>` marker
// form the seed preflight recognises — markers survive the posse rename
// (HISTORY.md, rangerhq-7xpn), a bare "posse-" prefix would not.
func fixtureBeadID(i int) string { return fmt.Sprintf("rangerhq-x%02d", i) }

// repoDir is the fixture's beads repo — session names are derived from its
// basename, so the holder join has something real to match.
const repoDir = "/Users/x/src/posse"

// fixture is the cockpit the golden tests draw: enough rows to overflow a
// 20-line popup, a title long enough to truncate at 60 and 80, droppable
// context (repo dir, (focused), cost) that only a wide terminal keeps, and
// an IN PROGRESS section covering all four stall ranks and both halves of
// the holder join (ADR 0004 §2) — Dial F per-bead sessions and the
// pre-Dial-F persona slot.
func fixture() *cockpit {
	at := time.Date(2026, 8, 18, 14, 5, 9, 0, time.UTC)
	c := &cockpit{
		now:      func() time.Time { return at },
		planLine: "5h 42% · 7d 61%",
		sessions: []posse.HerdrSession{
			{Name: posse.SessionForBead("devops", repoDir, "rangerhq-h3n"), Emoji: "🧛", Agent: "devops", Status: "blocked", Dir: repoDir},
			{Name: posse.SessionForBead("developer", repoDir, "rangerhq-fei"), Emoji: "🐿️", Agent: "developer", Status: "working", Dir: repoDir, Focused: true},
			{Name: posse.SessionFor("business-manager", repoDir), Emoji: "🙂", Agent: "business-manager", Status: "idle", Dir: repoDir, Runtime: "gemini", Tier: "premium"},
			{Name: "notes", Emoji: "📓", Status: "", Crew: true},
		},
		costByBead:    map[string]float64{"rangerhq-fei": 1.25},
		costToday:     4.5,
		costDayCap:    20,
		costUncounted: 1,
		// The fixture's one uncounted session is the gemini one above — a
		// runtime with no cost adapter, which is what "uncounted" means
		// (ADR 0012 D4). It was codex until codex gained one (rangerhq-0va);
		// the footer names it from here, never from a hardcoded runtime pair.
		costUncountedRuntimes: []string{"gemini"},
		costAt:                at,
		status:                "dispatched rangerhq-fei → developer-rangerhq-fei",
	}
	titles := []string{
		"cockpit v2 (b): IN PROGRESS section — Bd.InProgressAll, holder join and stalled-first sort",
		"scorecard: per-persona close rates",
		"dispatch: honour budget_day",
		"gates: L0 deny rules for git push",
		"herdr: codex detection override",
		"beads: sync before commit",
		"docs: ADR index",
		"cost: attribute grok sessions",
		"parity: cage tier matrix",
		"envs: tighten mode on write",
	}
	for i, t := range titles {
		c.issues = append(c.issues, posse.RepoIssue{
			BdIssue: posse.BdIssue{ID: fixtureBeadID(i), Title: t, Priority: i % 4},
			Dir:     "/Users/x/src/posse",
		})
	}
	c.issues[0].Assignee = "developer"

	// Deliberately filed in the *wrong* order: sortInProg is what puts the
	// stalled ones on top, and the goldens should show that it ran.
	for _, ip := range []struct {
		id, who, title string
		age            time.Duration
	}{
		{"rangerhq-fei", "developer", "cockpit v2 (a): row model and render(w,h), width-aware columns", 4 * time.Minute},
		{"rangerhq-a1a", "business-manager", "onboarding: PID doc pass", 3 * time.Hour},
		{"rangerhq-h3n", "devops", "herdr: codex detection override — the dialog reads as idle", 26 * time.Hour},
		{"rangerhq-9q2", "qa", "verify: dispatch honours budget_day", 12 * time.Minute},
	} {
		c.inprog = append(c.inprog, posse.RepoIssue{
			BdIssue: posse.BdIssue{ID: ip.id, Title: ip.title, Priority: 2,
				Status: "in_progress", Assignee: ip.who, Updated: at.Add(-ip.age)},
			Dir: repoDir,
		})
	}
	c.sortInProg()
	return c
}

func golden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test ./cmd/posse -run TestCockpitGolden -update)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs (-want +got):\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}

// versionMask is the release version, blanked to a token of its own LENGTH,
// so the goldens pin the header's shape without pinning the number
// (ranger-base-qlrx: bumping to 0.4.0 reddened all three). Same length on
// purpose — the header is width-truncated, so a substitution that changed
// the line's length would move every column after it and the goldens would
// stop meaning anything. What the version actually IS is
// cmd/posse.TestVersionNamesTheCommitWithoutTheLdflag's question.
func versionMask() (from, to string) {
	return posse.Version, strings.Repeat("V", len(posse.Version))
}

// ADR 0004 §5: render(w,h) is a pure function of the row model, drawn to a
// size. Three sizes, one fixed clock, byte-for-byte.
func TestCockpitGolden(t *testing.T) {
	defer func(b string) { posse.Build = b }(posse.Build)
	posse.Build = "test"
	from, to := versionMask()
	for _, sz := range []struct {
		name string
		w, h int
	}{
		{"60x20", 60, 20},
		{"80x24", 80, 24},
		{"140x40", 140, 40},
	} {
		t.Run(sz.name, func(t *testing.T) {
			c := fixture()
			c.buildRows()
			c.offset = scrollTo(0, c.cursorRow(), len(c.rows), viewportH(sz.h))
			out := stripANSI(c.render(sz.w, sz.h))
			for _, ln := range strings.Split(out, "\r\n") {
				if n := dispWidth(ln); n > sz.w {
					t.Errorf("line wider than %d (%d runes): %q", sz.w, n, ln)
				}
			}
			body := strings.ReplaceAll(out, "\r\n", "\n")
			if !strings.Contains(body, from+"+test") {
				t.Fatalf("the header does not carry %q — the mask below would be a no-op and the golden would pin nothing", from+"+test")
			}
			golden(t, "cockpit-"+sz.name, strings.ReplaceAll(body, from, to))
		})
	}
}

// The header/footer are fixed and the rows in between get exactly h−5
// (ADR 0004 §4) — the invariant every golden above silently depends on.
func TestCockpitViewportHeight(t *testing.T) {
	for _, h := range []int{20, 24, 40} {
		c := fixture()
		c.buildRows()
		if got, want := len(c.renderLines(80, h)), h; got != want {
			t.Errorf("h=%d: rendered %d lines, want %d", h, got, want)
		}
		if got, want := viewportH(h), h-5; got != want {
			t.Errorf("viewportH(%d) = %d, want %d", h, got, want)
		}
	}
	// Non-tty: 80×∞ — no padding, no clipping, every row drawn.
	c := fixture()
	c.buildRows()
	if got, want := len(c.renderLines(80, 0)), 2+len(c.rows)+3; got != want {
		t.Errorf("unbounded height: %d lines, want %d", got, want)
	}
}

// Cells, not runes: the ADR's rune count under-measures 🎭/👤 inside the flex
// column and a 60-column popup wraps. See the widths block in cockpit.go.
func TestCockpitDispWidth(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		{"", 0},
		{"plain", 5},
		{"⚡", 2},  // Emoji_Presentation by default
		{"⚙️", 2}, // ...and by variation selector
		{"⚙", 1},  // ...text presentation without one
		{"○", 1},  // a geometric shape stays narrow
		{"·", 1},
		{"…", 1},
		{"🐿️", 2},
		{"👨‍💻", 2}, // ZWJ sequence: one glyph
		{"🎭developer", 11},
		{"  ⚡ 🤖 working  x 🎭developer", 30},
	} {
		if got := dispWidth(c.in); got != c.want {
			t.Errorf("dispWidth(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// wideRune binary-searches wideRanges, which is only correct while the table
// stays sorted, non-empty and disjoint — a merged-in range in the wrong place
// would go silently unfound and the row would wrap again (rangerhq-53p).
func TestCockpitWideRangesSorted(t *testing.T) {
	prev := rune(-1)
	for i, g := range wideRanges {
		if g[0] > g[1] {
			t.Errorf("wideRanges[%d] = %04X-%04X is inverted", i, g[0], g[1])
		}
		if g[0] <= prev {
			t.Errorf("wideRanges[%d] = %04X-%04X overlaps or follows %04X", i, g[0], g[1], prev)
		}
		prev = g[1]
	}
	// ...and the search agrees with a scan of the same table, at every edge
	// and either side of it.
	for _, g := range wideRanges {
		for _, r := range []rune{g[0] - 1, g[0], g[1], g[1] + 1} {
			want := false
			for _, h := range wideRanges {
				if r >= h[0] && r <= h[1] {
					want = true
					break
				}
			}
			if got := wideRune(r); got != want {
				t.Errorf("wideRune(%04X) = %v, the table says %v", r, got, want)
			}
		}
	}
}

// The regression the smoke test caught: a session row with emoji in both the
// fixed marks and the flex column must still fit a 60-column popup.
func TestCockpitRowFitsNarrow(t *testing.T) {
	c := &cockpit{sessions: []posse.HerdrSession{
		{Name: "developer-posse-rangerhq-fei", Emoji: "🤖", Agent: "developer",
			Status: "working", Runtime: "claude", Tier: "standard", Crew: true, Focused: true},
	}}
	for _, w := range []int{40, 60, 80, 100, 140} {
		got := stripANSI(renderRow(row{kind: rowItem, cols: c.sessionCols(c.sessions[0])}, w, true))
		if n := dispWidth(got); n > w {
			t.Errorf("w=%d: row is %d cells wide: %q", w, n, got)
		}
	}
}

// ADR 0004 §1, one case per column kind.
func TestCockpitColumnKinds(t *testing.T) {
	is := posse.RepoIssue{
		BdIssue: posse.BdIssue{ID: "rangerhq-fei", Priority: 2, Assignee: "developer",
			Title: "cockpit v2 (a): row model and render(w,h), width-aware columns, SIGWINCH"},
		Dir: "/Users/x/src/posse",
	}
	wide := layout(issueCols(is, 14), 140)
	if !strings.Contains(stripANSI(wide), "src/posse") {
		t.Errorf("140 cols must keep the droppable repo dir: %q", stripANSI(wide))
	}
	if strings.Contains(stripANSI(wide), "…") {
		t.Errorf("140 cols must not truncate the title: %q", stripANSI(wide))
	}

	narrow := stripANSI(layout(issueCols(is, 14), 99))
	if strings.Contains(narrow, "src/posse") {
		t.Errorf("droppables go under %d cols: %q", dropAt, narrow)
	}
	if !strings.Contains(narrow, "developer") {
		t.Errorf("the holder survives at 99 cols: %q", narrow)
	}
	if !strings.HasSuffix(narrow, "…") {
		t.Errorf("the flex title truncates with …: %q", narrow)
	}

	tiny := stripANSI(layout(issueCols(is, 14), 69))
	if strings.Contains(tiny, "developer") {
		t.Errorf("the holder column goes under %d cols: %q", dropHolderAt, tiny)
	}
	// Fixed columns are never truncated by the layout: the id and priority
	// survive every width that can hold them.
	for _, w := range []int{69, 99, 140} {
		if got := stripANSI(layout(issueCols(is, 14), w)); !strings.HasPrefix(got, "rangerhq-fei   p2") {
			t.Errorf("w=%d: fixed columns must print whole: %q", w, got)
		}
	}
	if n := dispWidth(tiny); n > 69 {
		t.Errorf("layout overran its width: %d runes", n)
	}
}

// rangerhq-zag6: the holder column's 12-cell pad was a minimum only, so a
// longer name (the shipped example crew's `business-manager`, 16 cells)
// pushed the title and dir right for that row alone and the section stopped
// reading as a table. It clips now — in both sections that draw a holder.
func TestCockpitHolderColumnClips(t *testing.T) {
	const w = 140
	c := fixture()
	issue := func(who string) posse.RepoIssue {
		return posse.RepoIssue{BdIssue: posse.BdIssue{ID: "rangerhq-fei", Priority: 2,
			Assignee: who, Title: "cockpit v2 (a): row model and render(w,h)"},
			Dir: repoDir}
	}
	for _, tc := range []struct {
		name string
		cols func(posse.RepoIssue) []col
	}{
		{"ready", func(is posse.RepoIssue) []col { return issueCols(is, 14) }},
		{"inprog", func(is posse.RepoIssue) []col { return c.inprogCols(is, 14) }},
	} {
		short := stripANSI(layout(tc.cols(issue("qa")), w))
		long := stripANSI(layout(tc.cols(issue("business-manager")), w))
		if !strings.Contains(long, "business-ma…") {
			t.Errorf("%s: a 16-cell holder must clip to its 12-cell pad: %q", tc.name, long)
		}
		if strings.Contains(long, "business-manager") {
			t.Errorf("%s: the holder printed whole and shifted the row: %q", tc.name, long)
		}
		// Cells, not bytes: the … that marks the clip is three bytes wide.
		at := func(s string) int {
			i := strings.Index(s, "cockpit v2")
			if i < 0 {
				return -1
			}
			return dispWidth(s[:i])
		}
		if a, b := at(short), at(long); a != b || a < 0 {
			t.Errorf("%s: the title must start in the same column for every row: %d vs %d\n%q\n%q",
				tc.name, a, b, short, long)
		}
		if n := dispWidth(long); n != dispWidth(short) {
			t.Errorf("%s: rows must be the same width: %d vs %d", tc.name, n, dispWidth(short))
		}
	}
}

// ranger-base-g48p: the id column had the same pad-is-a-minimum defect the
// holder column did (rangerhq-zag6) — pad 14 fits an id shaped like
// rangerhq-fei (12-13 cells) but not one shaped like ranger-base-2jl5 (16),
// so that row alone pushed its flex column right. Clipping is wrong here,
// unlike the holder: a truncated id can't be copied, which is what the
// column is for. The fix widens the whole section's id column to its
// widest id (idColPad) instead.
func TestCockpitIDColumnWidensToSection(t *testing.T) {
	const w = 140
	c := fixture()
	issue := func(id string) posse.RepoIssue {
		return posse.RepoIssue{BdIssue: posse.BdIssue{ID: id, Priority: 2,
			Assignee: "qa", Title: "cockpit v2 (a): row model and render(w,h)"},
			Dir: repoDir}
	}
	for _, tc := range []struct {
		name string
		cols func(posse.RepoIssue, int) []col
	}{
		{"ready", issueCols},
		{"inprog", c.inprogCols},
	} {
		short, long := issue("rangerhq-fei"), issue("ranger-base-2jl5")
		pad := idColPad([]posse.RepoIssue{short, long})
		if want := dispWidth(long.ID); pad != want {
			t.Fatalf("%s: idColPad = %d, want the widest id's %d cells", tc.name, pad, want)
		}
		shortRow := stripANSI(layout(tc.cols(short, pad), w))
		longRow := stripANSI(layout(tc.cols(long, pad), w))
		if !strings.Contains(longRow, "ranger-base-2jl5") {
			t.Errorf("%s: the id must print whole, never clipped — it's what the column is for: %q", tc.name, longRow)
		}
		at := func(s string) int {
			i := strings.Index(s, "cockpit v2")
			if i < 0 {
				return -1
			}
			return dispWidth(s[:i])
		}
		if a, b := at(shortRow), at(longRow); a != b || a < 0 {
			t.Errorf("%s: every row's title must start in the same column: %d vs %d\n%q\n%q",
				tc.name, a, b, shortRow, longRow)
		}
	}
}

// The section-wide pass is per section, not global: READY WORK and IN
// PROGRESS each get an over-long id at a *different* width, so a pad shared
// across sections (or hardcoded, or computed but not plumbed to one of
// them) shows up as a mismatch here — not just as "still 14".
func TestCockpitBuildRowsWidensIDColumnPerSection(t *testing.T) {
	c := fixture()
	const issueID, inprogID = "ranger-base-2jl5", "ranger-base-longid" // 16, 18 cells
	c.issues[0].ID = issueID
	c.inprog[0].ID = inprogID
	c.buildRows()
	wantPad := map[section]int{secIssues: dispWidth(issueID), secInProg: dispWidth(inprogID)}
	if wantPad[secIssues] == wantPad[secInProg] {
		t.Fatalf("fixture setup: the two ids must differ in width to tell the sections apart")
	}
	seen := map[section]bool{}
	for _, r := range c.rows {
		if r.kind != rowItem || (r.sec != secIssues && r.sec != secInProg) {
			continue
		}
		if got, want := r.cols[0].pad, wantPad[r.sec]; got != want {
			t.Errorf("sec=%d row (id %q): id column pad = %d, want %d", r.sec, r.cols[0].text, got, want)
		}
		seen[r.sec] = true
	}
	if !seen[secIssues] || !seen[secInProg] {
		t.Fatalf("fixture setup: seen=%v", seen)
	}
}

func TestTruncCells(t *testing.T) {
	for _, c := range []struct {
		in   string
		n    int
		want string
	}{
		{"abcdef", 10, "abcdef"},
		{"abcdef", 6, "abcdef"},
		{"abcdef", 5, "abcd…"},
		{"abcdef", 1, "…"},
		{"abcdef", 0, ""},
		{"héllo·wörld", 6, "héllo…"},
		{"⚙️ posse", 8, "⚙️ posse"}, // VS16: ⚙️ is two cells, not one — 8 is an exact fit
		{"🎭developer", 5, "🎭de…"},   // a wide glyph is never split
		{"🎭developer", 2, "…"},      // ...so the cut lands before it
		{"👨‍💻 ops", 6, "👨‍💻 ops"},   // a ZWJ sequence is one glyph
	} {
		if got := truncCells(c.in, c.n); got != c.want {
			t.Errorf("truncCells(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

// rangerhq-5qm: below h=6 the ADR's fixed 2+3 chrome no longer fits around a
// viewport, so the chrome yields — but the render is still exactly h lines,
// still shows the row under the cursor, and still keeps the line the operator
// acts on. Every mode, because each has its own footer.
func TestCockpitShortTerminalShedsChrome(t *testing.T) {
	modes := []struct {
		name   string
		set    func(*cockpit)
		action string // the footer line that must survive to h=2
	}{
		{"normal", func(*cockpit) {}, "enter focus"},
		{"prompt", func(c *cockpit) { c.mode, c.input = modePrompt, []rune("go") }, "prompt"},
		{"confirm", func(c *cockpit) { c.mode, c.confirm = modeConfirm, confirmUnclaim }, "(y/n)"},
		{"peek", func(c *cockpit) { c.mode, c.peekText = modePeek, "a\nb\nthe tail\n" }, "any key returns"},
	}
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			for _, w := range []int{60, 140} {
				for h := 1; h <= 12; h++ {
					c := fixture()
					c.buildRows()
					c.offset = scrollTo(0, c.cursorRow(), len(c.rows), viewportH(h))
					m.set(c)
					lines := c.renderLines(w, h)
					if len(lines) != h {
						t.Errorf("%dx%d: %d lines rendered, want %d", w, h, len(lines), h)
					}
					out := stripANSI(strings.Join(lines, "\n"))
					if h >= 2 && !strings.Contains(out, m.action) {
						t.Errorf("%dx%d: no %q line to act on:\n%s", w, h, m.action, out)
					}
					if m.name == "normal" && !strings.Contains(out, "▸") {
						t.Errorf("%dx%d: the cursor row is not drawn:\n%s", w, h, out)
					}
					if m.name == "peek" && !strings.Contains(out, "the tail") {
						t.Errorf("%dx%d: peek shows no pane content:\n%s", w, h, out)
					}
				}
			}
		})
	}
}

// ADR 0004 §4: one viewport, a 2-row margin, ↑/↓ n more at the edges.
func TestCockpitScrolling(t *testing.T) {
	c := fixture()
	c.buildRows()
	total := len(c.rows)
	vh := viewportH(20) // 15 body lines for 19 rows

	if total <= vh {
		t.Fatalf("fixture must overflow a 20-line popup: %d rows, %d lines", total, vh)
	}
	// Top of the list: no up-marker, a down-marker with the right count.
	c.offset = scrollTo(0, c.cursorRow(), total, vh)
	if c.offset != 0 {
		t.Errorf("cursor at the top: offset %d, want 0", c.offset)
	}
	top := strings.Join(c.bodyLines(80, vh), "\n")
	if strings.Contains(top, "↑") || !strings.Contains(stripANSI(top), "↓ ") {
		t.Errorf("top of the list wants a ↓ marker only:\n%s", stripANSI(top))
	}
	// G: the last item pulls the viewport to the bottom.
	c.cursor = c.items() - 1
	c.offset = scrollTo(c.offset, c.cursorRow(), total, vh)
	if c.offset == 0 {
		t.Errorf("last item must scroll the viewport")
	}
	n, up, down := visible(c.offset, total, vh)
	if !up || down {
		t.Errorf("bottom of the list wants an ↑ marker only (up=%v down=%v)", up, down)
	}
	if c.offset+n != total {
		t.Errorf("bottom must show the last row: offset %d + %d != %d", c.offset, n, total)
	}
	// ...and fill the window doing it: the ↓ marker's line goes back to a row.
	if n != vh-1 {
		t.Errorf("bottom window shows %d rows, want %d (vh %d less the ↑ marker)", n, vh-1, vh)
	}
	body := c.bodyLines(80, vh)
	if last := body[len(body)-1]; last == "" {
		t.Errorf("padding under the last row of a full viewport:\n%s",
			stripANSI(strings.Join(body, "\n")))
	}
	// The cursor is inside the window at every position, with the margin
	// respected wherever the list is long enough to give it.
	for i := 0; i < c.items(); i++ {
		c.cursor = i
		c.offset = scrollTo(c.offset, c.cursorRow(), total, vh)
		n, _, _ := visible(c.offset, total, vh)
		if cr := c.cursorRow(); cr < c.offset || cr >= c.offset+n {
			t.Errorf("item %d (row %d) outside the viewport [%d,%d)", i, cr, c.offset, c.offset+n)
		}
	}
	// The markers cost viewport lines, never extra ones.
	c.offset = 2
	if got := len(c.bodyLines(80, vh)); got != vh {
		t.Errorf("body with both markers: %d lines, want %d", got, vh)
	}
}

// ctrl-d/ctrl-u page, g/G jump, and the cursor only ever lands on item rows.
func TestCockpitPagingKeys(t *testing.T) {
	c := fixture()
	c.buildRows()
	c.height, c.width = 20, 80
	last := c.items() - 1

	if _, err := c.handleKey([]byte("G")); err != nil {
		t.Fatal(err)
	}
	if c.cursor != last {
		t.Errorf("G: cursor %d, want %d", c.cursor, last)
	}
	if _, err := c.handleKey([]byte("g")); err != nil {
		t.Fatal(err)
	}
	if c.cursor != 0 {
		t.Errorf("g: cursor %d, want 0", c.cursor)
	}
	before := c.cursorRow()
	if _, err := c.handleKey([]byte("\x04")); err != nil { // ctrl-d
		t.Fatal(err)
	}
	if c.cursorRow() <= before {
		t.Errorf("ctrl-d must move down: row %d, was %d", c.cursorRow(), before)
	}
	if r := c.rows[c.cursorRow()]; r.kind != rowItem {
		t.Errorf("ctrl-d landed on a non-item row: %+v", r)
	}
	down := c.cursor
	if _, err := c.handleKey([]byte("\x15")); err != nil { // ctrl-u
		t.Fatal(err)
	}
	if c.cursor >= down {
		t.Errorf("ctrl-u must move up: cursor %d, was %d", c.cursor, down)
	}
}

// Peek shares the viewport instead of appending below the list (§4): the
// tail is clipped to the window and the chrome keeps its size.
func TestCockpitPeekClipped(t *testing.T) {
	c := fixture()
	c.buildRows()
	var tail []string
	for i := 0; i < 40; i++ {
		tail = append(tail, fmt.Sprintf("line %d", i))
	}
	c.peekText = strings.Join(tail, "\n")
	c.mode = modePeek
	lines := c.renderLines(80, 24)
	if len(lines) != 24 {
		t.Fatalf("peek rendered %d lines, want 24", len(lines))
	}
	body := stripANSI(strings.Join(lines[2:2+viewportH(24)], "\n"))
	if !strings.Contains(body, "line 39") {
		t.Errorf("peek must show the tail:\n%s", body)
	}
	if strings.Contains(body, "line 20\n") && strings.Contains(body, "line 0\n") {
		t.Errorf("peek must be clipped to the viewport:\n%s", body)
	}
}

// The goldens are colour-stripped; this is where the colours are asserted.
func TestCockpitRenderColours(t *testing.T) {
	c := fixture()
	c.buildRows()
	out := c.render(80, 24)
	for _, want := range []string{aRed, aGrn, aYlw, aDim, aBold, aRev} {
		if !strings.Contains(out, want) {
			t.Errorf("render dropped %q", strings.TrimPrefix(want, "\x1b"))
		}
	}
}

// ─── ADR 0004 §2–3: the IN PROGRESS section ─────────────────────────────────

// The holder join reads both session names a claimed bead can live under:
// its Dial F per-bead session, and the pre-Dial-F persona slot. ADR 0004 §2
// names only the slot; on a Dial F fleet that alone finds nothing.
func TestCockpitHolderJoin(t *testing.T) {
	c := fixture()
	for _, tc := range []struct{ id, want string }{
		{"rangerhq-fei", "working"}, // per-bead session
		{"rangerhq-a1a", "idle"},    // persona slot session
		{"rangerhq-h3n", "blocked"}, //
		{"rangerhq-9q2", noSession}, // holder has no session at all
	} {
		is := posse.RepoIssue{}
		for _, ip := range c.inprog {
			if ip.ID == tc.id {
				is = ip
			}
		}
		if is.ID == "" {
			t.Fatalf("fixture lost %s", tc.id)
		}
		if got := c.holderState(is); got != tc.want {
			t.Errorf("holderState(%s) = %q, want %q", tc.id, got, tc.want)
		}
	}
	// No assignee, no holder — and never a panic looking for one.
	if got := c.holderState(posse.RepoIssue{BdIssue: posse.BdIssue{ID: "x"}, Dir: repoDir}); got != noSession {
		t.Errorf("unassigned bead: holderState = %q, want %q", got, noSession)
	}
}

// Stalled-first: blocked, no session, idle, working (ADR 0004 §2).
func TestCockpitInProgressStalledFirst(t *testing.T) {
	c := fixture()
	var got []string
	for _, is := range c.inprog {
		got = append(got, is.ID)
	}
	want := []string{"rangerhq-h3n", "rangerhq-9q2", "rangerhq-a1a", "rangerhq-fei"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("stalled-first order: %v, want %v", got, want)
	}
	// Ties keep bd's order: two blocked beads stay as they came.
	c.sessions = nil
	c.inprog = []posse.RepoIssue{
		{BdIssue: posse.BdIssue{ID: "b"}, Dir: repoDir},
		{BdIssue: posse.BdIssue{ID: "a"}, Dir: repoDir},
	}
	c.sortInProg()
	if c.inprog[0].ID != "b" {
		t.Errorf("stable sort must keep bd's order within a rank: %s first", c.inprog[0].ID)
	}
	for state, rank := range map[string]int{"blocked": 0, noSession: 1, "shell": 1, "idle": 2, "done": 2, "working": 3, "wat": 4} {
		if got := stallRank(state); got != rank {
			t.Errorf("stallRank(%q) = %d, want %d", state, got, rank)
		}
	}
}

// A bead appears in one section only (ADR 0004 §2).
func TestCockpitReadyFilter(t *testing.T) {
	mk := func(id, status string) posse.RepoIssue {
		return posse.RepoIssue{BdIssue: posse.BdIssue{ID: id, Status: status}, Dir: repoDir}
	}
	ready := []posse.RepoIssue{mk("a", "open"), mk("b", "in_progress"), mk("c", "open"), mk("d", "open")}
	inprog := []posse.RepoIssue{mk("d", "in_progress")}
	got := readyOnly(ready, inprog)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("readyOnly kept %v, want [a c]", got)
	}
	// Same id, different repo: a different bead, and it stays.
	other := posse.RepoIssue{BdIssue: posse.BdIssue{ID: "d", Status: "open"}, Dir: "/other"}
	if got := readyOnly([]posse.RepoIssue{other}, inprog); len(got) != 1 {
		t.Errorf("a same-id bead in another repo must survive: %v", got)
	}
}

func TestShortAge(t *testing.T) {
	now := time.Date(2026, 8, 18, 14, 5, 9, 0, time.UTC)
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{3 * time.Minute, "3m"},
		{119 * time.Minute, "1h"},
		{2 * time.Hour, "2h"},
		{26 * time.Hour, "1d"},
		{9 * 24 * time.Hour, "9d"},
		{-time.Minute, "0s"}, // a clock skew reads as now, never as -1m
	} {
		if got := shortAge(now, now.Add(-tc.d)); got != tc.want {
			t.Errorf("shortAge(-%s) = %q, want %q", tc.d, got, tc.want)
		}
	}
	if got := shortAge(now, time.Time{}); got != "-" {
		t.Errorf("no timestamp: shortAge = %q, want %q", got, "-")
	}
}

// ADR 0004 §1–2: the holder *name* drops below dropHolderAt with the ready
// section's; the holder *state* does not — it is why the section exists.
func TestCockpitInProgressColumns(t *testing.T) {
	c := fixture()
	is := c.inprog[0] // devops, blocked, 1d old
	wide := stripANSI(layout(c.inprogCols(is, 14), 140))
	for _, want := range []string{"rangerhq-h3n", "p2", "devops", "blocked", "1d", "src/posse"} {
		if !strings.Contains(wide, want) {
			t.Errorf("140 cols must show %q: %q", want, wide)
		}
	}
	narrow := stripANSI(layout(c.inprogCols(is, 14), 99))
	if strings.Contains(narrow, "src/posse") {
		t.Errorf("droppables go under %d cols: %q", dropAt, narrow)
	}
	tiny := stripANSI(layout(c.inprogCols(is, 14), 69))
	if strings.Contains(tiny, "devops") {
		t.Errorf("the holder name goes under %d cols: %q", dropHolderAt, tiny)
	}
	if !strings.Contains(tiny, "blocked") {
		t.Errorf("the holder state stays — it is the stall signal: %q", tiny)
	}
	for _, w := range []int{40, 60, 69, 80, 99, 140} {
		got := stripANSI(renderRow(row{kind: rowItem, sec: secInProg, cols: c.inprogCols(is, 14)}, w, true))
		if n := dispWidth(got); n > w {
			t.Errorf("w=%d: in-progress row is %d cells wide: %q", w, n, got)
		}
	}
}

// tab cycles all three sections in draw order, skipping empty ones.
func TestCockpitTabCyclesThreeSections(t *testing.T) {
	c := fixture()
	c.buildRows()
	tab := func() int {
		if _, err := c.handleKey([]byte("\t")); err != nil {
			t.Fatal(err)
		}
		return c.cursor
	}
	c.cursor = 0
	if got, want := tab(), len(c.sessions); got != want {
		t.Errorf("tab from SESSIONS: cursor %d, want %d (IN PROGRESS)", got, want)
	}
	if got, want := tab(), len(c.sessions)+len(c.inprog); got != want {
		t.Errorf("tab from IN PROGRESS: cursor %d, want %d (READY WORK)", got, want)
	}
	if got := tab(); got != 0 {
		t.Errorf("tab from READY WORK: cursor %d, want 0 (SESSIONS)", got)
	}
	// An empty section is skipped, never landed on.
	c.inprog = nil
	c.buildRows()
	c.cursor = 0
	if got, want := tab(), len(c.sessions); got != want {
		t.Errorf("tab past an empty IN PROGRESS: cursor %d, want %d (READY WORK)", got, want)
	}
}

// ADR 0004 §3: the footer offers the selected section's keys only.
func TestCockpitPerSectionFooter(t *testing.T) {
	c := fixture()
	c.buildRows()
	foot := func() string { return stripANSI(strings.Join(c.footerLines(120), "\n")) }

	c.cursor = 0 // a session
	if got := foot(); !strings.Contains(got, "x kill") || !strings.Contains(got, "o crew/fleet") ||
		strings.Contains(got, "u unclaim") || strings.Contains(got, "c claim") {
		t.Errorf("SESSIONS footer:\n%s", got)
	}
	c.cursor = len(c.sessions) // an in-progress bead
	if got := foot(); !strings.Contains(got, "d resume") || !strings.Contains(got, "u unclaim") ||
		!strings.Contains(got, "enter focus holder") || strings.Contains(got, "x kill") {
		t.Errorf("IN PROGRESS footer:\n%s", got)
	}
	c.cursor = len(c.sessions) + len(c.inprog) // a ready bead
	if got := foot(); !strings.Contains(got, "c claim") || !strings.Contains(got, "d dispatch") ||
		strings.Contains(got, "u unclaim") || strings.Contains(got, "v peek") {
		t.Errorf("READY WORK footer:\n%s", got)
	}
}

// enter/p/v act on the holder; d resumes; u unclaims behind a y (§3).
func TestCockpitInProgressKeys(t *testing.T) {
	c := fixture()
	c.buildRows()
	c.results = make(chan string, 4)
	var launched []string
	c.launcher = func(bead posse.RepoIssue, resume bool) (string, error) {
		launched = append(launched, fmt.Sprintf("%s resume=%v", bead.ID, resume))
		return "session", nil
	}
	press := func(k string) {
		t.Helper()
		if _, err := c.handleKey([]byte(k)); err != nil {
			t.Fatal(err)
		}
	}

	// rangerhq-9q2: claimed by qa, no session. enter/p/v say so rather
	// than acting on nothing.
	c.cursor = len(c.sessions) + 1
	if is := c.selInProg(); is == nil || is.ID != "rangerhq-9q2" {
		t.Fatalf("cursor %d is not rangerhq-9q2: %+v", c.cursor, c.selInProg())
	}
	for _, k := range []string{"\r", "p", "v"} {
		c.status, c.mode = "", modeNormal
		press(k)
		if c.mode != modeNormal {
			t.Errorf("%q with no holder must not change mode: %v", k, c.mode)
		}
		if !strings.Contains(c.status, "qa has no session") {
			t.Errorf("%q with no holder: status %q", k, c.status)
		}
	}
	// d on that row is dispatch --resume.
	press("d")
	if !strings.Contains(c.status, "resuming rangerhq-9q2") {
		t.Errorf("d on an in-progress row: status %q", c.status)
	}
	if msg := <-c.results; !strings.Contains(msg, "resumed rangerhq-9q2") {
		t.Errorf("launch result: %q", msg)
	}
	c.dispatching = false
	if len(launched) != 1 || launched[0] != "rangerhq-9q2 resume=true" {
		t.Errorf("launched %v, want [rangerhq-9q2 resume=true]", launched)
	}
	// ...and on a ready row it is a plain dispatch.
	c.cursor = len(c.sessions) + len(c.inprog)
	press("d")
	<-c.results
	c.dispatching = false
	if got := launched[len(launched)-1]; !strings.HasSuffix(got, "resume=false") {
		t.Errorf("d on a ready row: %q, want resume=false", got)
	}

	// p on a row whose holder is alive opens the prompt for that session.
	c.cursor = len(c.sessions) + 3 // rangerhq-fei, held by developer (working)
	c.status, c.mode = "", modeNormal
	press("p")
	if c.mode != modePrompt {
		t.Fatalf("p with a live holder: mode %v, want modePrompt", c.mode)
	}
	if got := stripANSI(strings.Join(c.footerLines(120), "\n")); !strings.Contains(got, posse.SessionForBead("developer", repoDir, "rangerhq-fei")) {
		t.Errorf("the prompt names the holder's session:\n%s", got)
	}
	press("\033") // esc
	if c.mode != modeNormal {
		t.Errorf("esc must leave the prompt: mode %v", c.mode)
	}

	// u confirms before it unclaims, and n backs out without touching bd.
	c.status = ""
	press("u")
	if c.mode != modeConfirm || c.confirm != confirmUnclaim {
		t.Fatalf("u: mode %v confirm %v, want modeConfirm/confirmUnclaim", c.mode, c.confirm)
	}
	if got := stripANSI(strings.Join(c.footerLines(120), "\n")); !strings.Contains(got, "unclaim rangerhq-fei? (y/n)") {
		t.Errorf("unclaim confirmation:\n%s", got)
	}
	press("n")
	if c.mode != modeNormal || c.status != "unclaim cancelled" {
		t.Errorf("n: mode %v status %q", c.mode, c.status)
	}
	// u is an in-progress key only.
	for _, cur := range []int{0, len(c.sessions) + len(c.inprog)} {
		c.cursor, c.mode = cur, modeNormal
		press("u")
		if c.mode != modeNormal {
			t.Errorf("u at cursor %d must do nothing: mode %v", cur, c.mode)
		}
	}
	// x stays a sessions key, and it confirms a kill, not an unclaim.
	c.cursor, c.mode = 0, modeNormal
	press("x")
	if c.mode != modeConfirm || c.confirm != confirmKill {
		t.Errorf("x: mode %v confirm %v", c.mode, c.confirm)
	}
	if got := stripANSI(strings.Join(c.footerLines(120), "\n")); !strings.Contains(got, "kill ") {
		t.Errorf("kill confirmation:\n%s", got)
	}
}

// rangerhq-tdy8: the header's reading may be a SHARED one, taken minutes
// ago by a dispatch pass. The blind clock counts from when it was taken —
// otherwise a snapshot nobody can refresh keeps resetting the witness and
// the header says "fine" through an outage.
func TestCockpitPlanClockFollowsTheReading(t *testing.T) {
	at := time.Date(2026, 8, 19, 20, 53, 0, 0, time.UTC)
	c := &cockpit{now: func() time.Time { return at }}

	// A four-minute-old snapshot: shown as the reading it is.
	if got := c.planSegment(planRead{line: "5h 42% · 7d 61%", at: at.Add(-4 * time.Minute), guarded: true}); got != "5h 42% · 7d 61%" {
		t.Fatalf("a shared reading is still the reading, got %q", got)
	}
	// Seven minutes later it cannot be refreshed: eleven minutes blind,
	// counted from the reading, not from the tick that displayed it.
	at = at.Add(7 * time.Minute)
	if got := c.planSegment(planRead{guarded: true}); got != "plan — · guard blind 11m" {
		t.Errorf("blind is time since the reading was taken, got %q", got)
	}
	// A reading with no time on it (nothing shared it) is taken as now.
	if got := c.planSegment(planRead{line: "5h 9% · 7d 30%", guarded: true}); got != "5h 9% · 7d 30%" {
		t.Fatalf("recovery = %q", got)
	}
	at = at.Add(2 * time.Minute)
	if got := c.planSegment(planRead{guarded: true}); got != "plan — · guard blind 2m" {
		t.Errorf("an untimed reading clocks from now, got %q", got)
	}
}

// rangerhq-6h1, the witness half: an empty plan segment reads exactly like
// "no guard configured", which is what makes a blind guard invisible. When
// the guard IS configured and the reading failed, the header says so and
// carries the age; when it is not, the header stays clean as before.
func TestCockpitPlanBlindWitness(t *testing.T) {
	at := time.Date(2026, 8, 19, 20, 53, 0, 0, time.UTC)
	c := &cockpit{now: func() time.Time { return at }}

	// A good reading is the reading, and it is what the clock counts from.
	if got := c.planSegment(planRead{line: "5h 42% · 7d 61%", guarded: true}); got != "5h 42% · 7d 61%" {
		t.Fatalf("a good reading is the segment, got %q", got)
	}

	// Fourteen minutes later the reading fails and the guard is on: say it.
	at = at.Add(14 * time.Minute)
	got := c.planSegment(planRead{guarded: true})
	if want := "plan — · guard blind 14m"; got != want {
		t.Errorf("blind segment = %q, want %q", got, want)
	}
	var b strings.Builder
	c.out, c.planLine = &b, got
	c.drawPlain()
	if !strings.Contains(b.String(), "guard blind 14m") {
		t.Errorf("the witness must reach the header:\n%s", b.String())
	}

	// The clock keeps counting from the last good reading, not from the
	// last failure — two failed scans in a row are one blind window.
	at = at.Add(6 * time.Minute)
	if got := c.planSegment(planRead{guarded: true}); got != "plan — · guard blind 20m" {
		t.Errorf("blind is time since the last reading, got %q", got)
	}

	// A reading back clears it, and no guard means nothing to be blind
	// about: the header stays exactly as clean as it was before 6h1.
	if got := c.planSegment(planRead{line: "5h 9% · 7d 30%", guarded: true}); got != "5h 9% · 7d 30%" {
		t.Errorf("recovery = %q", got)
	}
	at = at.Add(3 * time.Hour)
	if got := c.planSegment(planRead{}); got != "" {
		t.Errorf("unguarded and unreadable must stay silent, got %q", got)
	}
}

// rangerhq-6h1: the first failed scan, before any successful reading, is
// the case the empty header used to hide. Floor the clock at cockpit start
// and say so — 0s is a witness, empty is "guard not configured".
func TestCockpitPlanBlindWitnessFromStart(t *testing.T) {
	at := time.Date(2026, 8, 19, 20, 53, 0, 0, time.UTC)
	c := &cockpit{now: func() time.Time { return at }}

	got := c.planSegment(planRead{guarded: true})
	if want := "plan — · guard blind 0s"; got != want {
		t.Fatalf("first guarded failure = %q, want %q (empty is the original bug)", got, want)
	}
	at = at.Add(14 * time.Minute)
	if got := c.planSegment(planRead{guarded: true}); got != "plan — · guard blind 14m" {
		t.Errorf("blind is time since cockpit start when there was never a reading, got %q", got)
	}
}

// ADR 0018 §1: blindness is now two different days, and the header must not
// read them as one. Unarmed, a blind guard is about to park the on-meter
// lanes; armed, the same blind guard is a declared degrade with a spend
// ceiling under it. Same clock, opposite outcomes.
func TestCockpitPlanBlindNamesTheLedgerBrake(t *testing.T) {
	at := time.Date(2026, 8, 19, 20, 53, 0, 0, time.UTC)
	c := &cockpit{now: func() time.Time { return at }}

	// A reading first, so the blind clock counts from something real.
	c.planSegment(planRead{line: "5h 42% · 7d 61%", guarded: true, ledger: true})
	at = at.Add(14 * time.Minute)
	if got := c.planSegment(planRead{guarded: true, ledger: true}); got != "plan — · guard blind 14m — ledger brake" {
		t.Errorf("an armed ledger must show as the brake that is holding, got %q", got)
	}
	if got := c.planSegment(planRead{guarded: true}); got != "plan — · guard blind 14m" {
		t.Errorf("unarmed is today's line, unchanged: %q", got)
	}
	// A good reading is a good reading either way — the clause is about
	// blindness, not about Dial E being configured.
	if got := c.planSegment(planRead{line: "5h 9% · 7d 30%", guarded: true, ledger: true}); got != "5h 9% · 7d 30%" {
		t.Errorf("a reading is the segment, got %q", got)
	}
	// No guard, no blindness to qualify.
	at = at.Add(time.Hour)
	if got := c.planSegment(planRead{ledger: true}); got != "" {
		t.Errorf("unguarded stays silent whatever Dial E says, got %q", got)
	}
}

// ADR 0018 §3 (ranger-base-3nvt): the THIRD outcome, which the two-state
// segment above rendered as the degrade. Caps armed over a ledger the cost
// scan came back short of is a brake counting nothing, and the dispatcher
// parks on it (internal/posse, TestBlindDegradeParksWhenTheLedgerCannotBeRead)
// — so a header reading " — ledger brake" there tells the operator the
// opposite of what the pass did.
func TestCockpitPlanBlindUnreadableLedgerIsParkedNotBraked(t *testing.T) {
	at := time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC)
	// costUnread is the fact, and it is the cost scan's: three transcripts
	// the last scan could not read, which is what the footer beside this
	// segment is already hedging its dollars with.
	c := &cockpit{now: func() time.Time { return at }, costUnread: 3}

	// A reading first, so the blind clock counts from something real.
	c.planSegment(planRead{line: "5h 42% · 7d 61%", guarded: true, ledger: true})
	at = at.Add(4 * time.Hour)

	got := c.planSegment(planRead{guarded: true, ledger: true})
	if want := "plan — · guard blind 4h00m — ledger unreadable, parked"; got != want {
		t.Fatalf("an unreadable armed ledger = %q, want %q", got, want)
	}
	if strings.Contains(got, "brake") {
		t.Errorf("the parked state must not name a brake that is not holding: %q", got)
	}
	// Dial E unset is the unarmed park, which has no clause and gains none
	// from a scan that could not read the ledger it is not keeping.
	if got := c.planSegment(planRead{guarded: true}); got != "plan — · guard blind 4h00m" {
		t.Errorf("unarmed says what it always said, got %q", got)
	}
	// A reading is the segment either way — this clause is about blindness.
	if got := c.planSegment(planRead{line: "5h 9% · 7d 30%", guarded: true, ledger: true}); got != "5h 9% · 7d 30%" {
		t.Errorf("a reading is the segment, got %q", got)
	}
	// WITHOUT-ARM, and the one that matters: a READABLE armed ledger is
	// still the degrade. Delete the brake clause outright and every
	// assertion above still passes — this is what refuses that fix.
	c.costUnread = 0
	at = at.Add(4 * time.Hour) // the reading above reset the blind clock
	if got := c.planSegment(planRead{guarded: true, ledger: true}); got != "plan — · guard blind 4h00m — ledger brake" {
		t.Errorf("a readable armed ledger is still the brake, got %q", got)
	}
}

// The wiring pin. planSegment is exercised over a hand-built planRead, so a
// clause that renders a field nothing ever assigns is green there and dead
// on the screen — which is precisely how Unread came to be missing from the
// display in the first place (ADR 0018 §3 / ranger-base-c65c). The fact is
// the COST scan's, so this walks it from a *posse.CostReport to c.planLine.
func TestCockpitApplyPlanTakesTheLedgerFailureFromTheCostScan(t *testing.T) {
	at := time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC)
	c := &cockpit{now: func() time.Time { return at }}
	c.applyPlan(planRead{line: "5h 42% · 7d 61%", guarded: true, ledger: true})
	at = at.Add(4 * time.Hour)

	// A clean scan: the degraded day, unchanged.
	c.applyCost(&posse.CostReport{DayCap: 250})
	c.applyPlan(planRead{guarded: true, ledger: true})
	if want := "plan — · guard blind 4h00m — ledger brake"; c.planLine != want {
		t.Fatalf("clean scan = %q, want %q", c.planLine, want)
	}

	// The same blind read over a scan that lost three transcripts.
	c.applyCost(&posse.CostReport{DayCap: 250, Unread: 3})
	c.applyPlan(planRead{guarded: true, ledger: true})
	if want := "plan — · guard blind 4h00m — ledger unreadable, parked"; c.planLine != want {
		t.Fatalf("unreadable scan = %q, want %q", c.planLine, want)
	}
	// And the header does not wait out a plan tick for it: the cost scan is
	// 30s and the plan scan 2m, so the tick that LEARNS it re-renders.
	c.applyCost(&posse.CostReport{DayCap: 250})
	if want := "plan — · guard blind 4h00m — ledger brake"; c.planLine != want {
		t.Errorf("the cost tick must re-render the segment it feeds, got %q", c.planLine)
	}
	c.applyCost(&posse.CostReport{DayCap: 250, Unread: 1})
	if want := "plan — · guard blind 4h00m — ledger unreadable, parked"; c.planLine != want {
		t.Errorf("re-render, other direction: %q", c.planLine)
	}

	// A SUCCESSFUL reading is not re-rendered by the cost tick: it is the
	// segment, and re-running it would move planReadAt every 30s and reset
	// the blind clock the header is counting.
	c.applyPlan(planRead{line: "5h 9% · 7d 30%", guarded: true, ledger: true})
	c.applyCost(&posse.CostReport{DayCap: 250, Unread: 3})
	if c.planLine != "5h 9% · 7d 30%" {
		t.Errorf("a reading must survive the cost tick, got %q", c.planLine)
	}
	at = at.Add(30 * time.Minute)
	if got := c.planSegment(planRead{guarded: true, ledger: true}); got != "plan — · guard blind 30m — ledger unreadable, parked" {
		t.Errorf("the blind clock must count from the reading, got %q", got)
	}
}

// ADR 0019 D3 (ranger-base-vmqg): the fourth state. An adapter ships, this
// platform holds no credential for it, and the header must say so as itself
// — no blind timer counting up toward a park that will never come, and not
// the third state's words either. "No adapter" reads as "posse does not
// support your provider"; what is actually missing is one login.
func TestCockpitPlanNoCredentialSourceIsOffNotBlind(t *testing.T) {
	at := time.Date(2026, 8, 19, 20, 53, 0, 0, time.UTC)
	c := &cockpit{now: func() time.Time { return at }}

	got := c.planSegment(planRead{guarded: true, noSource: true})
	if want := "plan — · guard off, no credential source"; got != want {
		t.Fatalf("no credential source = %q, want %q", got, want)
	}
	// An hour later it is the same line: nothing is counting.
	at = at.Add(time.Hour)
	if got := c.planSegment(planRead{guarded: true, noSource: true}); got != "plan — · guard off, no credential source" {
		t.Errorf("no clock may run on structural absence, got %q", got)
	}
	// Dial E does not qualify it — the ledger brake is about blindness, and
	// this is not blindness.
	if got := c.planSegment(planRead{guarded: true, noSource: true, ledger: true}); strings.Contains(got, "ledger brake") {
		t.Errorf("guard-off is not a degrade, got %q", got)
	}
	// The third state keeps its own words.
	if got := c.planSegment(planRead{guarded: true, noAdapter: true}); got != "plan — · guard off, no adapter" {
		t.Errorf("no adapter = %q, want the unchanged line", got)
	}
	// Unarmed says nothing, as ever: there is no guard to be off.
	if got := c.planSegment(planRead{noSource: true}); got != "" {
		t.Errorf("unguarded stays silent, got %q", got)
	}
}

// planOffState is the classification the header forks on, pinned without a
// cockpit: both ways a *NoSource arrives read as the same state, and a
// refusal that is not one reads as the adapter state it is.
func TestCockpitPlanOffStateReadsBothArrivals(t *testing.T) {
	ns := &posse.NoSource{Runtime: "claude", Purpose: posse.CredMeter, GOOS: "linux",
		Store: "the Claude Code credentials file", Arm: "log in once with `claude`"}
	cases := []struct {
		name               string
		err                error
		source, no_adapter bool
	}{
		// Caught by the availability check, so the reason arrives flattened
		// into a sentence AND kept as a value.
		{"availability check", &posse.NoPlanAdapter{Why: "no adapter serves this machine", Errs: []error{ns}}, true, false},
		// Caught by the read: the store went away after that check.
		{"the read", ns, true, false},
		{"wrapped", fmt.Errorf("reading the meter: %w", ns), true, false},
		// A refusal with no credential in it is the third state, unchanged.
		{"no adapter", &posse.NoPlanAdapter{Why: "no plan-window adapter is compiled in"}, false, true},
		// Everything else is blindness and keeps its clock.
		{"ordinary failure", errors.New("usage endpoint unreachable"), false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source, noAdapter := planOffState(c.err)
			if source != c.source || noAdapter != c.no_adapter {
				t.Errorf("planOffState = (noSource %v, noAdapter %v), want (%v, %v)",
					source, noAdapter, c.source, c.no_adapter)
			}
		})
	}
}

// ─── the GOVERNANCE block (bead rangerhq-81y0) ──────────────────────────────

// govFixture is the fixture with a condition set on it — one URGENT, one
// LANE, and a carry-over with no G-row.
func govFixture() *cockpit {
	c := fixture()
	c.gov = posse.GovSet{
		{ID: "G1", Class: posse.GovLane, Key: "blocked:devops-x", Detail: "devops-x (devops) is blocked on an approval"},
		{ID: "G7", Class: posse.GovUrgent, Key: "loop-dead", Detail: "autostart is armed and no watch loop holds the lock"},
		{Class: posse.GovLane, Key: "no-live:coordinator", Detail: "no live session for coordinator"},
	}
	return c
}

// A clear shop must not spend two lines of a 20-line popup saying so — and
// the goldens, whose fixture has no conditions, are the proof that it does
// not.
func TestCockpitGovBlockHiddenWhenClear(t *testing.T) {
	c := fixture()
	c.buildRows()
	if strings.Contains(stripANSI(c.render(140, 40)), "GOVERNANCE") {
		t.Error("a clear shop draws no GOVERNANCE block")
	}
}

func TestCockpitGovBlockDrawsUrgentFirst(t *testing.T) {
	c := govFixture()
	c.buildRows()
	out := stripANSI(c.render(140, 40))
	if !strings.Contains(out, "GOVERNANCE (1 URGENT · 2 LANE)") {
		t.Errorf("want the summary heading, got:\n%s", out)
	}
	urgent := strings.Index(out, "loop-dead")
	if urgent < 0 {
		urgent = strings.Index(out, "no watch loop")
	}
	lane := strings.Index(out, "is blocked on an approval")
	sessions := strings.Index(out, "SESSIONS (")
	if urgent < 0 || lane < 0 || sessions < 0 {
		t.Fatalf("block did not render:\n%s", out)
	}
	if !(urgent < lane) {
		t.Errorf("URGENT must come first:\n%s", out)
	}
	if !(lane < sessions) {
		t.Errorf("the block belongs above SESSIONS:\n%s", out)
	}
	// The carry-over keeps its em dash rather than being given a G-row.
	if !strings.Contains(out, "—") {
		t.Errorf("a carry-over row must not be given a G-id:\n%s", out)
	}
}

// The rows are filler, not cursor items: the block must not move the cursor
// or change what tab and the keys act on. This is the whole reason it is
// drawn as a banner rather than as a fourth section.
func TestCockpitGovBlockIsNotCursorSpace(t *testing.T) {
	plain, gov := fixture(), govFixture()
	plain.buildRows()
	gov.buildRows()
	if plain.items() != gov.items() {
		t.Errorf("cursor space changed: %d → %d", plain.items(), gov.items())
	}
	if len(gov.rows) <= len(plain.rows) {
		t.Error("the block drew no rows")
	}
	for _, r := range gov.rows[:len(gov.rows)-len(plain.rows)] {
		if r.kind == rowItem {
			t.Errorf("a governance row must not be selectable: %+v", r)
		}
	}
}

// An unreadable store is not an all-clear here either: the heading says so
// in the one word a count alone cannot.
func TestCockpitGovBlockSaysPartial(t *testing.T) {
	c := fixture()
	c.govFailed = 2
	c.buildRows()
	out := stripANSI(c.render(140, 40))
	if !strings.Contains(out, "partial, 2 store(s) unread") {
		t.Errorf("want the partial heading, got:\n%s", out)
	}
}

// ─── the credential-expiry segment (ADR 0019 D5, ranger-base-k6ha) ──────────

// The header's fourth state about credentials, and the only one that is not
// about the plan guard: a posse-owned session mint dies inside a fortnight.
// It is a WARNING — the shop below it is running, nothing is parked — so it
// says how long and gets out of the way.
func TestCockpitCredSegment(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	mint := func(set string, at time.Time) posse.CredExpiry {
		return posse.CredExpiry{Runtime: "claude", Purpose: "session", Set: set,
			Key: "CLAUDE_CODE_OAUTH_TOKEN", At: at}
	}

	// Nothing to say draws NOTHING — not an empty column, not a separator.
	// The golden render (§5) is the control on that: it has no creds and
	// must keep the header it had before this segment existed.
	if _, ok := credCol(nil, now); ok {
		t.Error("an empty warning list drew a column")
	}

	c, ok := credCol([]posse.CredExpiry{mint("container", now.Add(6*24*time.Hour))}, now)
	if !ok || c.text != "cred: claude in 6d" {
		t.Errorf("approaching: %q ok=%v", c.text, ok)
	}
	if c.ansi != aYlw {
		t.Errorf("approaching is yellow, got %q", c.ansi)
	}

	// Expired is a different colour AND different words: they cost
	// different things, and one rendering for both tells the eye nothing.
	c, _ = credCol([]posse.CredExpiry{mint("container", now.Add(-time.Hour))}, now)
	if c.text != "cred: claude EXPIRED" || c.ansi != aRed {
		t.Errorf("expired: %q %q", c.text, c.ansi)
	}

	// Several: the soonest is named and the rest are COUNTED. A header
	// column that listed them would push the shop's own status off an
	// 80-column pane to report a fortnight of notice.
	c, _ = credCol([]posse.CredExpiry{
		mint("alpha", now.Add(2*24*time.Hour)),
		mint("zulu", now.Add(9*24*time.Hour)),
	}, now)
	if c.text != "cred: claude in 2d +1" {
		t.Errorf("two warnings: %q", c.text)
	}
}

// In the header, in front of the governance segment, and only when there is
// one. Governance keeps the right edge: it is the row that says whether the
// shop is delivering, and the credential warning is about a fortnight from
// now.
func TestCockpitCredSegmentRendersInTheHeader(t *testing.T) {
	c := fixture()
	head := func() string { return stripANSI(c.renderLines(120, 24)[0]) }

	clean := head()
	if strings.Contains(clean, "cred:") {
		t.Fatalf("a shop with no expiring credential says nothing:\n%s", clean)
	}
	// "Says nothing" is a COLUMN COUNT, not an empty string. An empty
	// column still costs the flex a separator cell — invisible on a wide
	// pane and one character off the plan reading on a narrow one — so the
	// rule is that the column is absent, and that is what is pinned.
	if n := len(c.headerCols()); n != 3 {
		t.Errorf("a healthy shop draws %d header columns, want 3", n)
	}
	c.creds = []posse.CredExpiry{{Runtime: "claude", Purpose: "session", Set: "container",
		Key: "CLAUDE_CODE_OAUTH_TOKEN", At: c.clock().Add(3 * 24 * time.Hour)}}
	warned := head()
	i, g := strings.Index(warned, "cred: claude in 3d"), strings.Index(warned, "gov ")
	if i < 0 {
		t.Fatalf("the warning is not in the header:\n%s", warned)
	}
	if g < 0 || g < i {
		t.Errorf("governance must keep the right edge:\n%s", warned)
	}
	// The plan segment is untouched: two different questions, two columns.
	if !strings.Contains(warned, "5h 42%") {
		t.Errorf("the credential warning ate the plan reading:\n%s", warned)
	}
	if n := len(c.headerCols()); n != 4 {
		t.Errorf("a warning adds exactly one column, got %d", n)
	}
}

// The scan is what makes the segment true of this box, and the apply is what
// makes it reach the header. Both tests above set c.creds by hand and would
// stay green with the scan deleted, the apply deleted, or both — so this
// walks the whole path: env set on disk → scanPlan → the channel → applyPlan
// → the drawn header.
func TestCockpitCredSegmentComesFromTheEnvSetsOnDisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := posse.NewAppAt(filepath.Join(home, "config"))
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	body := "# expires=" + at.AddDate(0, 0, 5).Format("2006-01-02") + "\n" +
		"CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-FIXTURE\n"
	if err := os.WriteFile(filepath.Join(a.EnvsDir, "container.env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	c := fixture()
	c.app, c.now = a, func() time.Time { return at }
	c.plans = make(chan planRead, 1)
	c.scanPlan()
	select {
	case r := <-c.plans:
		c.applyPlan(r)
	default:
		t.Fatal("the scan landed nothing on the channel")
	}
	if got := stripANSI(c.renderLines(120, 24)[0]); !strings.Contains(got, "cred: claude in 4d") {
		t.Errorf("the stamp on disk never reached the header:\n%s", got)
	}
}

// The cockpit's money line is read as the day's spend against the cap. When
// the cost scan could not read every transcript (ADR 0018 §3) that reading
// is a floor, and the header already distinguishes parked from degraded —
// the dollars must not be the one line that still claims a total. Both arms,
// and the narrow one: the marker must survive the truncation that eats the
// count.
func TestCockpitCostLineMarksAnUnreadableScanAFloor(t *testing.T) {
	at := time.Date(2026, 8, 18, 14, 5, 9, 0, time.UTC)
	mk := func(unread int) string {
		c := &cockpit{
			now:        func() time.Time { return at },
			costToday:  4.5,
			costDayCap: 20,
			costAt:     at,
			costUnread: unread,
		}
		return c.footerLines(200)[1]
	}

	// Without-arm: nothing unread, nothing hedged.
	clean := mk(0)
	if !strings.Contains(clean, "today $4.50 of $20 budget_day (22%)") {
		t.Errorf("clean cost line changed shape: %q", clean)
	}
	for _, unwanted := range []string{"≥", "unreadable", "floor"} {
		if strings.Contains(clean, unwanted) {
			t.Errorf("a clean scan must not hedge, found %q: %q", unwanted, clean)
		}
	}

	got := mk(3)
	for _, want := range []string{
		// The floor marker in front of BOTH numbers the scan is short on.
		"today ≥$4.50 of $20 budget_day (≥22%)",
		"3 transcript(s) unreadable — a floor, not a total",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("degraded cost line missing %q: %q", want, got)
		}
	}
	// The count is the last thing on the line and a narrow terminal drops
	// it; the marker is in front precisely so it cannot be dropped with it.
	narrow := (&cockpit{
		now: func() time.Time { return at }, costToday: 4.5, costDayCap: 20,
		costAt: at, costUnread: 3,
	}).footerLines(30)[1]
	if strings.Contains(narrow, "unreadable") {
		t.Fatalf("width 30 was meant to truncate the count away: %q", narrow)
	}
	if !strings.Contains(narrow, "≥$4.50") {
		t.Errorf("the floor marker must survive truncation: %q", narrow)
	}
}

// The footer can only mark a floor if the scan's read failures reach it.
// This is the wiring pin: footerLines is exercised over hand-built fields,
// so a dropped assignment here is invisible to it — and a dropped assignment
// here is exactly the defect (the report carried Unread; no display path
// read it).
func TestCockpitApplyCostCarriesTheScansReadFailures(t *testing.T) {
	seg := &posse.Segment{Bead: "a-1", Model: "claude-sonnet-5", Start: time.Now(), Msgs: map[string]*posse.Usage{"a": {Model: "claude-sonnet-5"}}}
	seg.CostUSD = 4.5
	rep := &posse.CostReport{Beads: []*posse.Segment{seg}, Uncounted: 1, DayCap: 20, Unread: 3}
	c := &cockpit{}
	c.applyCost(rep)
	if c.costUnread != 3 {
		t.Errorf("costUnread = %d, want the scan's 3", c.costUnread)
	}
	// The neighbours, so a rewrite that keeps Unread and loses one of them
	// is caught here rather than by a golden nobody re-reads.
	if c.costToday != 4.5 || c.costDayCap != 20 || c.costUncounted != 1 || c.costByBead["a-1"] != 4.5 || c.costAt.IsZero() {
		t.Errorf("applyCost dropped a field: today=%v cap=%v unc=%d byBead=%v at=%v",
			c.costToday, c.costDayCap, c.costUncounted, c.costByBead, c.costAt)
	}
	// And the footer says so, end to end from the report.
	if line := c.footerLines(200)[1]; !strings.Contains(line, "≥$4.50") || !strings.Contains(line, "3 transcript(s) unreadable") {
		t.Errorf("footer did not mark the floor from the scan: %q", line)
	}
}

// A counted runtime whose dollars are not a number is the third state, and
// the cockpit had only two: `$uncounted` (no adapter) and a figure. codex has
// an adapter and no rate card — a plan seat reports no cost — so its beads
// land in ByBead at 0.00, and rendering that as money says the bead was free.
//
// Both arms, because a lookup cannot tell this 0 from a real one: the codex
// bead reads $unpriced, and a claude bead that genuinely cost $0.00 still
// prints its zero.
func TestCockpitSessionCostIsBlankForACountedRuntimeWithNoRate(t *testing.T) {
	cx := &posse.Segment{Bead: "c-1", Runtime: "codex", Model: "gpt-5.6-sol", Start: time.Now(),
		Msgs: map[string]*posse.Usage{"turn-0": {Model: "gpt-5.6-sol", In: 1000, Out: 200}}}
	free := &posse.Segment{Bead: "a-1", Runtime: "claude", Model: "claude-opus-5", Start: time.Now(),
		Msgs: map[string]*posse.Usage{"m1": {Model: "claude-opus-5"}}}
	cx.Total()
	free.Total()
	c := &cockpit{}
	c.applyCost(&posse.CostReport{Beads: []*posse.Segment{cx, free}})
	if !c.costBlankBeads["c-1"] || c.costBlankBeads["a-1"] {
		t.Fatalf("applyCost did not carry the blank beads: %v", c.costBlankBeads)
	}
	cost := func(agent, bead string) string {
		return c.sessionCost(posse.HerdrSession{
			Name: posse.SessionForBead(agent, repoDir, bead), Agent: agent, Dir: repoDir, Runtime: "codex"})
	}
	if got := cost("dev", "c-1"); got != "$unpriced" {
		t.Errorf("codex bead cost %q, want $unpriced — 0.00 would say it was free", got)
	}
	if got := cost("qa", "a-1"); got != "$0.00" {
		t.Errorf("a bead that really cost nothing keeps its zero: %q", got)
	}
}

// The same wiring one field over, and the reason it is its own test: the
// uncounted label used to be the literal string "codex/grok", written when
// neither had an adapter. Both have one now (ADR 0012 D4), so that label
// named runtimes whose spend IS in the number beside it and sent an operator
// looking for the wrong gap. The names come from the report, which builds
// them from the registry, so the footer follows the adapters — which is why
// the negative arm here asks the registry rather than spelling a name that
// will be wrong again on the next adapter.
func TestCockpitFooterNamesTheUncountedRuntimes(t *testing.T) {
	rep := &posse.CostReport{Uncounted: 1, UncountedRuntimes: []string{"gemini"}}
	c := &cockpit{}
	c.applyCost(rep)
	if len(c.costUncountedRuntimes) != 1 || c.costUncountedRuntimes[0] != "gemini" {
		t.Fatalf("applyCost dropped the runtime names: %v", c.costUncountedRuntimes)
	}
	line := c.footerLines(200)[1]
	if !strings.Contains(line, "1 gemini session(s) uncounted") {
		t.Errorf("the footer must name the runtime the scan could not count: %q", line)
	}
	for _, counted := range posse.CountedRuntimes() {
		if strings.Contains(line, counted) {
			t.Errorf("%s has an adapter; naming it as uncounted is the defect: %q", counted, line)
		}
	}
}

// rangerhq-ecl2: a launch queued behind a running pass says so. The
// cockpit's dispatcher writes to io.Discard, so the ADR 0011 §1 wait line
// reaches the operator only through the Progress sink newCockpit wires —
// without it `d` sits on "dispatching <id>…" for the length of the other
// launcher's hold with nothing saying why.
func TestCockpitStatusShowsTheLauncherLockWait(t *testing.T) {
	c := newCockpit(nil, nil, io.Discard)
	if c.disp.Progress == nil {
		t.Fatal("the cockpit's dispatcher has no progress sink: a blocked launch says nothing")
	}
	c.dispatching = true
	c.status = "dispatching a-1…"

	// What lockLaunches writes, from the launch goroutine.
	c.disp.Progress("⏳ launcher lock held by pid 4711 — waiting (ADR 0011 §1)")

	select {
	case msg := <-c.progress:
		c.applyProgress(msg)
	default:
		t.Fatal("the wait line never reached the event loop's channel")
	}
	if !strings.Contains(c.status, "launcher lock held by pid 4711") {
		t.Errorf("status line is %q, want the lock wait naming the holder", c.status)
	}
	// The launch is still in flight — this line is progress, not its result.
	if !c.dispatching {
		t.Error("a progress line cleared c.dispatching, which would let a second `d` in")
	}
}

// note runs on the launch goroutine, where blocking on a busy event loop
// would hold the launch itself. A full channel drops the line instead.
func TestCockpitProgressNeverBlocksTheLauncher(t *testing.T) {
	c := newCockpit(nil, nil, io.Discard)
	for i := 0; i < cap(c.progress)+3; i++ {
		done := make(chan struct{})
		go func() { c.note("⏳ waiting"); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("note blocked on write %d of %d — a launch would wait on the event loop", i+1, cap(c.progress)+3)
		}
	}
}

// A channel is only a status line if something drains it, and the drain is
// one case in runCockpit's select — which no test runs, because it wants a
// raw-mode terminal. So this pin is on the source: without that case the
// wait line lands in a buffer nobody reads, which is exactly the bug
// rangerhq-ecl2 opened on, and both tests above stay green over it. It says
// the reader exists; the tests above say what it does with the line.
func TestCockpitEventLoopDrainsProgress(t *testing.T) {
	src, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"case msg := <-c.progress:", "c.applyProgress(msg)"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("runCockpit's event loop does not %s — nothing drains the progress channel", want)
		}
	}
}

// rangerhq-sk6p: `p` on a row posse holds no session meta for cannot record
// the crew mark, and the status line says so. The operator who just started
// a conversation there would otherwise read "prompted <name>" and believe
// ADR 0008 engaged; the only other tell is a 👤 that is not in a list they
// may not be looking at.
//
// Both arms in one cockpit, over one herdr: the owned row is the control —
// the same keystrokes on a session with a meta say nothing extra, so the
// warning cannot be an unconditional suffix.
func TestCockpitPromptWarnsWhenTheCrewMarkCannotBeRecorded(t *testing.T) {
	home := t.TempDir()
	binDir := t.TempDir()
	herdr := filepath.Join(binDir, "herdr")
	if err := os.WriteFile(herdr, []byte(`#!/bin/sh
case "$1 $2" in
"workspace list")
  printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1","label":"owned","agent_status":"idle"},{"workspace_id":"w2","label":"stranger","agent_status":"idle"}]}}'
  exit 0;;
"agent list")
  printf '%s\n' '{"result":{"agents":[{"agent":"claude","agent_status":"idle","pane_id":"p1","workspace_id":"w1"},{"agent":"claude","agent_status":"idle","pane_id":"p2","workspace_id":"w2"}]}}'
  exit 0;;
"agent prompt")
  printf '%s\n' '{"result":{"submitted":true}}'
  exit 0;;
"agent explain")
  # A screen herdr has SEEN — a named rule, not the idle guess. The
  # readiness gate (ranger-base-k99a) stands in front of every p now,
  # and this test is about the crew mark behind it, not about the gate.
  printf '%s\n' '{"state":"idle","matched_rule":{"id":"live_prompt_box"},"visible_idle":true,"fallback_reason":null}'
  exit 0;;
esac
printf '%s\n' '{"error":{"code":"no","message":"unexpected '"$1 $2"'"}}'
exit 1
`), 0o755); err != nil {
		t.Fatal(err)
	}
	// The meta IS the ownership record: `owned` has one, `stranger` does
	// not, and nothing else about the two rows differs.
	metaDir := filepath.Join(home, "state", "herdr")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "owned.yaml"),
		[]byte("name: owned\nworkspace: w1\npane: p1\nemoji: 🙂\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// ServerGen() stats the operator's live herdr socket unless this points
	// it somewhere else (ranger-base-ouf9): a test must not read the box.
	t.Setenv("HERDR_SOCKET_PATH", filepath.Join(home, "no-such.sock"))

	a := &posse.App{
		Home:       home,
		ConfigPath: filepath.Join(home, "config.yaml"),
		StateDir:   filepath.Join(home, "state"),
	}
	c := &cockpit{
		app:     a,
		hb:      &posse.HerdrBackend{App: a, H: posse.Herdr{Bin: herdr}, Warn: io.Discard},
		prompts: make(chan string, 4),
	}
	c.refresh()
	c.buildRows()

	rowFor := func(name string) int {
		t.Helper()
		for i, s := range c.sessions {
			if s.Name == name {
				return i
			}
		}
		t.Fatalf("no session row %q in %+v", name, c.sessions)
		return -1
	}
	// The prompt runs off the event loop now (ranger-base-k99a): the keys
	// only start it, and the line it produces arrives on c.prompts, which
	// is where runCockpit's select puts it on the status line. So the test
	// plays the event loop's one relevant case.
	prompt := func(name string) string {
		t.Helper()
		c.cursor, c.mode, c.status = rowFor(name), modeNormal, ""
		for _, k := range []string{"p", "h", "i", "\r"} {
			if _, err := c.handleKey([]byte(k)); err != nil {
				t.Fatal(err)
			}
		}
		select {
		case msg := <-c.prompts:
			c.prompting = false
			c.status = msg
		case <-time.After(30 * time.Second):
			t.Fatalf("the prompt goroutine never reported; status stuck at %q", c.status)
		}
		return c.status
	}

	// The foreign row: prompted, and told the mark did not land.
	got := prompt("stranger")
	for _, want := range []string{"prompted stranger", "NOT recorded", "no session meta", "ADR 0008"} {
		if !strings.Contains(got, want) {
			t.Errorf("p on a foreign row must say %q: %q", want, got)
		}
	}

	// The control: a session with a meta is marked, and says only that it
	// was prompted.
	if got := prompt("owned"); got != "prompted owned" {
		t.Errorf("p on an owned row: status %q, want %q", got, "prompted owned")
	}
	if b, err := os.ReadFile(filepath.Join(metaDir, "owned.yaml")); err != nil || !strings.Contains(string(b), "crew: true") {
		t.Errorf("the control's mark was not recorded (%v): %s", err, b)
	}
}

// The cockpit half of ranger-base-pjoy: sessionCost decided with
// `s.Runtime != "claude"` — an ADR 0017 §3 shadow predicate, a runtime NAME
// standing in for "does anything read this runtime's spend". It asks the
// adapter registry now, and this drives it, because nothing did: the
// $uncounted arm had no test at all, so the name could go back in and the
// suite would stay green.
//
// grok is the row that matters. It has an adapter and is not claude, which
// is exactly what the old predicate got wrong — it gained one in
// ranger-base-k7nb and every grok pane in the cockpit said $uncounted about
// spend `posse cost` was already totalling.
func TestCockpitSessionCostAsksTheAdapterNotTheRuntimeName(t *testing.T) {
	c := &cockpit{}
	c.applyCost(&posse.CostReport{})
	cost := func(runtime string) string {
		return c.sessionCost(posse.HerdrSession{
			Name: posse.SessionForBead("dev", repoDir, "a-1"), Agent: "dev", Dir: repoDir, Runtime: runtime})
	}
	// Derived from the registry, not spelled: a runtime gaining an adapter
	// must move this test's expectation with it and not silently become an
	// assertion that the adapter is ignored.
	counted := posse.CountedRuntimes()
	if len(counted) < 2 {
		t.Fatalf("this pin needs a counted runtime that is not claude; registry has %v", counted)
	}
	for _, r := range counted {
		if got := cost(r); got == "$uncounted" {
			t.Errorf("%s has a cost adapter; the cockpit must not call its spend uncounted (got %q)", r, got)
		}
	}
	for _, r := range []string{"mycli", "gemini"} {
		if _, ok := posse.CostProviderFor(r); ok {
			t.Fatalf("%s gained an adapter; this arm needs a runtime nothing reads", r)
		}
		if got := cost(r); got != "$uncounted" {
			t.Errorf("nothing reads %s; its pane must say $uncounted, got %q", r, got)
		}
	}
	// A session with no persona is not a per-bead one whatever its runtime,
	// and "" is a pane whose runtime was never recorded — neither may be
	// labelled with a claim about spend.
	for _, s := range []posse.HerdrSession{
		{Name: "shell", Runtime: "mycli"},
		{Name: posse.SessionForBead("dev", repoDir, "a-1"), Agent: "dev", Dir: repoDir},
	} {
		if got := c.sessionCost(s); got != "" {
			t.Errorf("%+v must carry no cost label, got %q", s, got)
		}
	}
}

// ─── ADR 0016 §2 — the cockpit's own hint filter ─────────────────────────────

// fakeAgentBackend is a HerdrBackend backed by a herdr binary that reports
// one workspace-with-agent for every (workspace, pane) id pair given.
// Sessions() only lists a workspace herdr's listing actually carries — an
// agent with no matching workspace row is invisible to it — so this keeps
// both lists in step rather than reporting agents alone, and every pane
// shows up as a foreign session (no meta file), which is enough for tests
// that only care about the pane set moving under the hint subscription.
func fakeAgentBackend(t *testing.T, home string, panes ...[2]string) *posse.HerdrBackend {
	t.Helper()
	t.Setenv("HERDR_SOCKET_PATH", filepath.Join(home, "no-such.sock"))
	binDir := t.TempDir()
	herdr := filepath.Join(binDir, "herdr")
	var workspaces, agents []string
	for _, p := range panes {
		ws, pane := p[0], p[1]
		workspaces = append(workspaces, fmt.Sprintf(`{"workspace_id":%q,"label":%q,"agent_status":"working"}`, ws, ws))
		agents = append(agents, fmt.Sprintf(`{"agent":"claude","agent_status":"working","pane_id":%q,"workspace_id":%q}`, pane, ws))
	}
	script := `#!/bin/sh
case "$1 $2" in
"workspace list")
  printf '%s\n' '{"result":{"workspaces":[` + strings.Join(workspaces, ",") + `]}}'
  exit 0;;
"agent list")
  printf '%s\n' '{"result":{"agents":[` + strings.Join(agents, ",") + `]}}'
  exit 0;;
esac
exit 1
`
	if err := os.WriteFile(herdr, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return &posse.HerdrBackend{App: &posse.App{Home: home}, H: posse.Herdr{Bin: herdr}, Warn: io.Discard}
}

// In normal mode a hint re-reads the sessions at once — any subscribed
// event, blocked included, which is the whole reason the cockpit carries
// its own filter instead of the watch's settle gate (ADR 0016 §2).
// sessions starting empty and landing non-empty is the refresh's evidence.
// The bead half of that refresh stays on its cadence floor
// (ranger-base-u5rqp); TestCockpitHintKeepsTheBeadScanFloor is where that
// half is pinned.
func TestCockpitApplyHintRefreshesInNormalMode(t *testing.T) {
	home := t.TempDir()
	hb := fakeAgentBackend(t, home, [2]string{"w1", "w1:p1"})
	c := &cockpit{app: &posse.App{Home: home}, hb: hb, mode: modeNormal}

	c.applyHint(posse.HerdrHint{Kind: "pane_agent_status_changed", PaneID: "w1:p1", AgentStatus: "blocked"})
	if len(c.sessions) == 0 {
		t.Error("a hint in normal mode must run a full refresh — sessions is still empty")
	}
	if c.hintDirty {
		t.Error("normal mode redraws immediately; there is nothing left to defer")
	}
}

// In prompt/confirm/peek mode the same hint must NOT refresh — drawing
// over operator input mid-keystroke is exactly what those modes exist to
// prevent — and must instead set the dirty bit for consumeHintDirty to
// spend on the return to normal.
func TestCockpitApplyHintDefersInModalMode(t *testing.T) {
	home := t.TempDir()
	hb := fakeAgentBackend(t, home, [2]string{"w1", "w1:p1"})
	for _, mode := range []cockpitMode{modePrompt, modeConfirm, modePeek} {
		c := &cockpit{app: &posse.App{Home: home}, hb: hb, mode: mode}
		c.applyHint(posse.HerdrHint{Kind: "workspace_closed", WorkspaceID: "w9"})
		if len(c.sessions) != 0 {
			t.Errorf("mode %v: a hint must not refresh over operator input", mode)
		}
		if !c.hintDirty {
			t.Errorf("mode %v: a hint under a mode must set the dirty bit", mode)
		}
	}
}

// consumeHintDirty is where a deferred hint is actually spent: on the
// return to normal mode, which is every path handleKey has out of
// prompt/confirm/peek.
func TestCockpitConsumeHintDirtyRefreshesOnReturnToNormal(t *testing.T) {
	home := t.TempDir()
	hb := fakeAgentBackend(t, home, [2]string{"w1", "w1:p1"})
	c := &cockpit{app: &posse.App{Home: home}, hb: hb, mode: modePrompt}
	c.applyHint(posse.HerdrHint{Kind: "workspace_closed", WorkspaceID: "w9"})
	if len(c.sessions) != 0 || !c.hintDirty {
		t.Fatal("setup: the hint above must have deferred, not refreshed")
	}

	c.mode = modeNormal // what handleKey does on esc, enter, y and n
	c.consumeHintDirty()
	if len(c.sessions) == 0 {
		t.Error("the deferred hint must be spent as a refresh on the return to normal")
	}
	if c.hintDirty {
		t.Error("consumeHintDirty must clear the bit it spent")
	}
}

// pokeHintsIfPanesMoved is the cockpit's own truth path for ADR 0016 §1's
// refresh poke: it must fire only when the agent pane set actually differs
// from what the subscription was last dialled or poked with, never on
// every refreshSessions — poking at the cockpit's 2s cadence would redial
// the subscription at 0.5 Hz (§1).
func TestCockpitPokesOnlyWhenPaneSetMoves(t *testing.T) {
	home := t.TempDir()
	hb := fakeAgentBackend(t, home, [2]string{"w1", "w1:p1"})
	c := &cockpit{app: &posse.App{Home: home}, hb: hb, mode: modeNormal}
	c.hintRefresh = make(chan struct{}, 1)
	c.hintPanes = hb.AgentPanes() // as if the subscription was just dialled with this set

	poked := func() bool {
		select {
		case <-c.hintRefresh:
			return true
		default:
			return false
		}
	}

	c.refreshSessions()
	if poked() {
		t.Fatal("an unchanged pane set must not poke the subscription")
	}

	// The herdr binary is replaced with one reporting a second pane —
	// exactly what a newly detected agent looks like from this consumer's
	// truth path.
	moved := fakeAgentBackend(t, home, [2]string{"w1", "w1:p1"}, [2]string{"w2", "w2:p1"})
	c.hb = moved
	c.refreshSessions()
	if !poked() {
		t.Fatal("a pane set that grew must poke the subscription")
	}

	// It settled at the new set: the very next refresh must not poke again.
	c.refreshSessions()
	if poked() {
		t.Fatal("the same pane set twice must not poke twice")
	}
}

// startHints wires whatever Hints returns into c.hints, and its report
// callback reaches c.hintReports without blocking — the same shape
// Dispatcher.Hints gives the watch loop (watch.go), so a test can drive
// the cockpit's hint handling without a socket at all.
func TestCockpitStartHintsWiresOverrideAndReports(t *testing.T) {
	hintsCh := make(chan posse.HerdrHint, 1)
	c := &cockpit{
		Hints: func(ctx context.Context, report func(string)) <-chan posse.HerdrHint {
			report("herdr events unavailable — polling (dial unix: no such file)")
			return hintsCh
		},
		hintReports: make(chan string, 4),
	}
	c.startHints(context.Background())
	if c.hints == nil {
		t.Fatal("startHints must set c.hints from the Hints override")
	}
	select {
	case line := <-c.hintReports:
		if !strings.Contains(line, "unavailable") {
			t.Errorf("report line = %q", line)
		}
	default:
		t.Fatal("the report callback must reach c.hintReports without blocking")
	}
	hintsCh <- posse.HerdrHint{Kind: "workspace_closed"}
	select {
	case h := <-c.hints:
		if h.Kind != "workspace_closed" {
			t.Errorf("hint = %+v", h)
		}
	default:
		t.Fatal("a hint sent on the override's channel must be readable through c.hints")
	}
}

// Neither loop can be run under go test — the tty loop needs a raw
// terminal and the display-only loop waits on an OS signal — so, like
// TestCockpitEventLoopDrainsProgress above, this pins the source: the
// select statements actually feed a hint to applyHint/consumeHintDirty
// (tty) or at least wake the loop early (display-only), which is what the
// behavior tests above exercise once a hint arrives.
func TestCockpitEventLoopsWireTheHintChannel(t *testing.T) {
	src, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	for _, want := range []string{
		"c.startHints(hintCtx)",
		"case h, ok := <-c.hints:",
		"c.applyHint(h)",
		"c.consumeHintDirty()",
		"case line := <-c.hintReports:",
		"case _, ok := <-c.hints:",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the event loops do not %s — the hint channel is not wired in", want)
		}
	}
}
