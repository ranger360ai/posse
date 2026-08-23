package main

// Adversarial pins for the cockpit's render contract (ADR 0004 §1, §4, §5),
// written verifying rangerhq-fei under rangerhq-mpg. The live tests here are
// invariants render(w,h) already holds and must keep holding; the skipped
// ones are escapes with a bug bead each — delete the t.Skip when the bead
// closes, not before.
//
// Self-contained on purpose: its own fixture and its own ANSI stripper, so
// it keeps compiling while cockpit_test.go grows sections of its own.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/rhq"
)

var qaANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func qaPlain(s string) string { return qaANSI.ReplaceAllString(s, "") }

// qaFixture is deliberately hostile: emoji and a runtime/tier tag inside the
// flex column, a title long enough to truncate at every width under test, and
// enough rows to overflow a small pane in both directions.
func qaFixture() *cockpit {
	c := &cockpit{
		sessions: []rhq.HerdrSession{
			{Name: "devops-rangerhq-h3n", Emoji: "🧛", Agent: "devops", Status: "blocked", Dir: "/w"},
			{Name: "developer-rangerhq-fei", Emoji: "🐿️", Agent: "developer", Status: "working", Dir: "/w", Focused: true},
			{Name: "business-manager-rangerhq-a1a", Emoji: "🙂", Agent: "business-manager", Status: "idle", Runtime: "codex", Tier: "premium"},
			{Name: "notes", Emoji: "📓", Crew: true},
		},
	}
	for i := 0; i < 12; i++ {
		c.issues = append(c.issues, rhq.RepoIssue{
			BdIssue: rhq.BdIssue{
				ID:       fixtureBeadID(i),
				Title:    fmt.Sprintf("bead %d — a title long enough that the flex column has to cut it 👨‍💻", i),
				Priority: i % 4,
			},
			Dir: "/Users/x/src/posse",
		})
	}
	return c
}

// at builds the model and scrolls to the cursor, the way draw() does.
func (c *cockpit) qaAt(cursor, h int) {
	c.cursor = cursor
	c.buildRows()
	c.offset = scrollTo(0, c.cursorRow(), len(c.rows), viewportH(h))
}

func (c *cockpit) qaItems() int { return len(c.sessions) + len(c.issues) }

// ADR 0004 §4: the cursor is kept inside the viewport, whatever the size and
// wherever it is. Two witnesses — the arithmetic, and a ▸ on the screen.
func TestCockpitCursorStaysVisible(t *testing.T) {
	// 6 and 7 join the list with rangerhq-5qm: they were the sizes where the
	// more-markers used to eat the viewport whole.
	for _, h := range []int{6, 7, 8, 9, 10, 12, 20, 24, 40} {
		for cur := 0; cur < qaFixture().qaItems(); cur++ {
			c := qaFixture()
			c.qaAt(cur, h)
			n, _, _ := visible(c.offset, len(c.rows), viewportH(h))
			if row := c.cursorRow(); row < c.offset || row >= c.offset+n {
				t.Errorf("h=%d cursor=%d: row %d outside the drawn window [%d,%d)", h, cur, row, c.offset, c.offset+n)
			}
			if !strings.Contains(qaPlain(c.render(80, h)), "▸") {
				t.Errorf("h=%d cursor=%d: no ▸ on screen", h, cur)
			}
		}
	}
}

// ADR 0004 §4: "↑ n more" / "↓ n more" — n is the count of rows that are
// really off screen, not an approximation.
func TestCockpitMoreMarkersCountTruthfully(t *testing.T) {
	for _, h := range []int{8, 12, 20, 24, 40} {
		for cur := 0; cur < qaFixture().qaItems(); cur++ {
			c := qaFixture()
			c.qaAt(cur, h)
			n, up, down := visible(c.offset, len(c.rows), viewportH(h))
			out := qaPlain(c.render(120, h))
			if want := fmt.Sprintf("↑ %d more", c.offset); up && !strings.Contains(out, want) {
				t.Errorf("h=%d cursor=%d: want %q", h, cur, want)
			}
			if want := fmt.Sprintf("↓ %d more", len(c.rows)-c.offset-n); down && !strings.Contains(out, want) {
				t.Errorf("h=%d cursor=%d: want %q", h, cur, want)
			}
			if c.offset < 0 || c.offset+n > len(c.rows) {
				t.Errorf("h=%d cursor=%d: window [%d,%d) escapes the %d-row model", h, cur, c.offset, c.offset+n, len(c.rows))
			}
		}
	}
}

// ADR 0004 §4: a 2-row margin between the cursor and a viewport edge, except
// where the model itself ends.
func TestCockpitScrollMargin(t *testing.T) {
	for _, h := range []int{20, 24, 40} {
		for cur := 0; cur < qaFixture().qaItems(); cur++ {
			c := qaFixture()
			c.qaAt(cur, h)
			n, _, _ := visible(c.offset, len(c.rows), viewportH(h))
			if n >= len(c.rows) {
				continue // the whole model fits; no edge to keep away from
			}
			row := c.cursorRow()
			if top := row - c.offset; top < scrollMargin && c.offset > 0 {
				t.Errorf("h=%d cursor=%d: %d rows above the cursor, want >= %d", h, cur, top, scrollMargin)
			}
			if bot := c.offset + n - 1 - row; bot < scrollMargin && c.offset+n < len(c.rows) {
				t.Errorf("h=%d cursor=%d: %d rows below the cursor, want >= %d", h, cur, bot, scrollMargin)
			}
		}
	}
}

// ADR 0004 §1: no row is ever wider than the terminal. The ↑/↓ markers and
// peek mode are excluded here and pinned below — they are rangerhq-nm8, not
// an invariant the code holds yet.
func TestCockpitNoLineExceedsWidth(t *testing.T) {
	modes := []struct {
		name string
		set  func(*cockpit)
	}{
		{"normal", func(*cockpit) {}},
		{"prompt", func(c *cockpit) { c.mode, c.input = modePrompt, []rune(strings.Repeat("ぬ", 60)) }},
		{"confirm", func(c *cockpit) { c.mode = modeConfirm }},
	}
	for _, m := range modes {
		for w := 1; w <= 200; w++ {
			c := qaFixture()
			m.set(c)
			c.qaAt(0, 24)
			for i, ln := range strings.Split(qaPlain(c.render(w, 24)), "\r\n") {
				if strings.Contains(ln, "more") && (strings.Contains(ln, "↑") || strings.Contains(ln, "↓")) {
					continue // rangerhq-nm8
				}
				if n := dispWidth(ln); n > w {
					t.Fatalf("%s w=%d line %d: %d cells wide: %q", m.name, w, i, n, ln)
				}
			}
		}
	}
}

// truncCells is the only thing standing between a hostile title and a wrapped
// row: it must never exceed n cells and never return a non-prefix.
func TestCockpitTruncCellsNeverOverflows(t *testing.T) {
	for _, s := range []string{
		"plain ascii title", "日本語のタイトルです", "👨‍💻👩‍🚀 emoji crew", "⚙️⚙ mixed selectors",
		"🎭developer@codex/premium ⚠️degraded", "a👩‍👩‍👧‍👦b", strings.Repeat("🧛", 20), "👍🏽 skin tone", "",
	} {
		for n := 0; n <= dispWidth(s)+3; n++ {
			got := truncCells(s, n)
			if w := dispWidth(got); w > n {
				t.Errorf("truncCells(%q, %d) = %q — %d cells", s, n, got, w)
			}
			if got != "" && !strings.HasPrefix(s, strings.TrimSuffix(got, "…")) {
				t.Errorf("truncCells(%q, %d) = %q — not a prefix of the input", s, n, got)
			}
		}
	}
}

// The paging keys are index arithmetic over a model that reshuffles under
// them (rangerhq-5li): walk them and check nothing escapes the model.
func TestCockpitKeysNeverEscapeTheModel(t *testing.T) {
	keys := []string{"G", "\x04", "\x04", "\x04", "g", "\x15", "j", "j", "k", "\t", "G", "\x15", "\x04", "\t", "g"}
	for _, h := range []int{6, 7, 12, 24, 40} {
		c := qaFixture()
		c.width, c.height = 80, h
		c.buildRows()
		for i, k := range keys {
			if _, err := c.handleKey([]byte(k)); err != nil {
				t.Fatalf("h=%d key %q: %v", h, k, err)
			}
			c.buildRows()
			c.offset = scrollTo(c.offset, c.cursorRow(), len(c.rows), viewportH(h))
			if c.cursor < 0 || c.cursor >= c.qaItems() {
				t.Errorf("h=%d step %d key %q: cursor %d outside [0,%d)", h, i, k, c.cursor, c.qaItems())
			}
			if c.offset < 0 || c.offset > len(c.rows) {
				t.Errorf("h=%d step %d key %q: offset %d outside [0,%d]", h, i, k, c.offset, len(c.rows))
			}
		}
	}
}

// rangerhq-5qm (fixed): below h=8 the chrome outgrew the terminal (7 lines
// into 6, the header scrolled off) and the more-markers ate the whole
// viewport (zero rows at h=7). Reproduced live at 60x5, 60x6 and 60x7.
func TestCockpitFitsShortTerminal(t *testing.T) {
	for h := 1; h <= 45; h++ {
		c := qaFixture()
		c.qaAt(0, h)
		if got := len(c.renderLines(80, h)); got != h {
			t.Errorf("h=%d: %d lines rendered", h, got)
		}
		if n, _, _ := visible(c.offset, len(c.rows), viewportH(h)); n < 1 && len(c.rows) > 0 {
			t.Errorf("h=%d: %d rows drawn out of %d — nothing to look at", h, n, len(c.rows))
		}
	}
	empty := &cockpit{}
	for h := 1; h <= 10; h++ {
		empty.qaAt(0, h)
		if got := len(strings.Split(qaPlain(empty.render(60, h)), "\r\n")); got != h {
			t.Errorf("empty cockpit h=%d: %d lines rendered", h, got)
		}
	}
}

// ─── escapes, pinned ─────────────────────────────────────────────────────────

// rangerhq-nm8: the peek banner and the ↑/↓ markers are painted, never
// measured — the only lines in a width-aware view that do not know the width.
// The markers reach normal mode too, wherever the model overflows the pane.
func TestCockpitChromeLinesRespectWidth(t *testing.T) {
	t.Skip("rangerhq-nm8: peek banner is 30 cells at any width; markers overflow below w=12")
	narrow := qaFixture()
	for w := 1; w <= 12; w++ {
		narrow.qaAt(0, 12) // small enough that both markers show
		for _, ln := range strings.Split(qaPlain(narrow.render(w, 12)), "\r\n") {
			if n := dispWidth(ln); n > w {
				t.Errorf("normal-mode marker w=%d: %d cells: %q", w, n, ln)
			}
		}
	}
	c := qaFixture()
	c.mode = modePeek
	c.peekText = strings.Repeat("a peeked line\n", 20)
	for w := 1; w <= 60; w++ {
		c.qaAt(0, 24)
		for i, ln := range strings.Split(qaPlain(c.render(w, 24)), "\r\n") {
			if n := dispWidth(ln); n > w {
				t.Errorf("peek w=%d line %d: %d cells: %q", w, i, n, ln)
			}
		}
	}
	empty := &cockpit{}
	for w := 1; w <= 12; w++ {
		empty.qaAt(0, 24)
		for _, ln := range strings.Split(qaPlain(empty.render(w, 24)), "\r\n") {
			if n := dispWidth(ln); n > w {
				t.Errorf("marker w=%d: %d cells: %q", w, n, ln)
			}
		}
	}
}

// rangerhq-53p: dispWidth's table had holes, and an unknown glyph counted as
// one cell — under-counting, so the row wrapped. Closed by rebuilding the
// table as East_Asian_Width Wide/Fullwidth ∪ Emoji_Modifier_Base, so there is
// no unknown tail left to fall through. Widths below are the terminal's own
// cursor advance (tmux 3.7b), not another library's opinion.
func TestCockpitDispWidthTableGaps(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		// The filed gaps.
		{"🟠", 2}, {"🟡", 2}, {"🟢", 2}, {"🟣", 2}, {"🟤", 2},
		{"🟥", 2}, {"🟩", 2}, {"🟦", 2},
		{"🀄", 2}, {"🃏", 2},
		{"🈚", 2}, {"🈯", 2}, {"🉐", 2}, {"🈁", 2}, {"🈳", 2},
		// Found closing them, same class, not in the report: these are
		// East_Asian_Width Neutral and the terminal still draws them wide,
		// because they are Emoji_Modifier_Base — a skin tone may follow.
		{"☝", 2}, {"⛹", 2}, {"✌", 2}, {"✍", 2},
		{"🏋", 2}, {"🏌", 2}, {"🕴", 2}, {"🕵", 2}, {"🖐", 2},
		// The other side of the same rebuild: text-presentation code points
		// inside the emoji blocks the old coarse ranges counted as two. One
		// cell bare, two once VS16 promotes them.
		{"🌡", 1}, {"🌶", 1}, {"🍽", 1}, {"🏔", 1}, {"🏳", 1}, {"🐿", 1},
		{"👁", 1}, {"📽", 1}, {"🖥", 1}, {"🛠", 1},
		{"🌡️", 2}, {"🐿️", 2}, {"👁️", 2}, {"🛠️", 2},
		// Narrow neighbours of the new ranges, so the fix does not overshoot:
		// a regional indicator is one cell and a flag is a pair of them (the
		// reason the unknown tail stayed narrow), and the alchemical, arrow
		// and legacy-computing blocks above U+1F000 are one cell each.
		{"🇺", 1}, {"🇺🇸", 2}, {"🇯🇵", 2},
		{"🜁", 1}, {"🝥", 1}, {"🠀", 1}, {"🡀", 1}, {"🬀", 1},
		{"㉈", 1}, {"🨀", 1},
		// Still true after the rebuild.
		{"🎭developer", 11}, {"👨‍💻", 2}, {"👍🏽", 2}, {"中", 2}, {"ａ", 2},
	} {
		if got := dispWidth(c.in); got != c.want {
			t.Errorf("dispWidth(%q) = %d, the terminal advances %d", c.in, got, c.want)
		}
	}
	// End to end, the repro from the report: the layout believed this row was
	// exactly 60 cells and the terminal drew 61, wrapping the trailing … onto
	// the next line and pushing the footer off a full pane.
	is := rhq.RepoIssue{BdIssue: rhq.BdIssue{ID: "rangerhq-yel", Priority: 1,
		Title: "🟡 flaky suite " + strings.Repeat("x", 60)}, Dir: "/Users/x/src/posse"}
	if got := dispWidth(qaPlain(renderRow(row{kind: rowItem, cols: issueCols(is)}, 60, false))); got != 60 {
		t.Errorf("row rendered at 60 measures %d", got)
	}
	// ...and no title built from the newly-tabled glyphs may overflow its
	// terminal at any width — the wrap invariant, not just the arithmetic.
	for _, title := range []string{"🟡 flaky suite", "☝ a pointed remark", "🐿 bare, 🐿️ promoted"} {
		is := rhq.RepoIssue{BdIssue: rhq.BdIssue{ID: "rangerhq-yel", Priority: 1,
			Title: title + strings.Repeat(" x", 40)}, Dir: "/Users/x/src/posse"}
		for _, w := range []int{40, 60, 80, 100, 140} {
			line := qaPlain(renderRow(row{kind: rowItem, cols: issueCols(is)}, w, false))
			if n := dispWidth(line); n > w {
				t.Errorf("title %q at w=%d: %d cells: %q", title, w, n, line)
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Adversarial pins for the IN PROGRESS section (ADR 0004 §2–3), written
// verifying rangerhq-ehu under rangerhq-hkz. Same rule as above: the live
// tests are invariants that hold today, the skipped one is a filed escape.
const qaDir = "/Users/x/src/posse"

var qaClock = time.Date(2026, 8, 18, 14, 5, 9, 0, time.UTC)

// qaProgFixture is a cockpit with all four stall ranks and both halves of the
// holder join, built without touching cockpit_test.go's fixture().
func qaProgFixture() *cockpit {
	c := &cockpit{
		now: func() time.Time { return qaClock },
		sessions: []rhq.HerdrSession{
			{Name: rhq.SessionForBead("devops", qaDir, "b-blocked"), Emoji: "🧛", Agent: "devops", Status: "blocked", Dir: qaDir, PaneID: "w1:p1"},
			{Name: rhq.SessionForBead("developer", qaDir, "b-working"), Emoji: "🐿️", Agent: "developer", Status: "working", Dir: qaDir, PaneID: "w2:p1"},
			{Name: rhq.SessionFor("business-manager", qaDir), Emoji: "🙂", Agent: "business-manager", Status: "idle", Dir: qaDir, PaneID: "w3:p1"},
		},
		status: "dispatched b-working → developer",
	}
	for _, ip := range []struct {
		id, who string
		age     time.Duration
	}{
		{"b-working", "developer", 4 * time.Minute},
		{"b-slot", "business-manager", 3 * time.Hour},
		{"b-blocked", "devops", 26 * time.Hour},
		{"b-none", "qa", 12 * time.Minute},
	} {
		c.inprog = append(c.inprog, rhq.RepoIssue{
			BdIssue: rhq.BdIssue{ID: ip.id, Title: "a claimed bead with a reasonably long title so the flex column has work to do",
				Status: "in_progress", Priority: 2, Assignee: ip.who, Updated: qaClock.Add(-ip.age)},
			Dir: qaDir,
		})
	}
	c.sortInProg()
	for i := 0; i < 6; i++ {
		c.issues = append(c.issues, rhq.RepoIssue{
			BdIssue: rhq.BdIssue{ID: "r-" + string(rune('a'+i)), Title: "ready work item", Priority: i % 4},
			Dir:     qaDir,
		})
	}
	c.buildRows()
	return c
}

// ── geometry ────────────────────────────────────────────────────────────────

// The invariant the whole v2 layout rests on, now that a seven-column row
// type exists: render must produce exactly h lines, none wider than w, at
// every size and in every mode.
func TestQAInProgressGeometrySweep(t *testing.T) {
	modes := []struct {
		name string
		set  func(*cockpit)
	}{
		{"normal", func(c *cockpit) {}},
		{"prompt", func(c *cockpit) { c.mode = modePrompt; c.input = []rune(strings.Repeat("x", 120)) }},
		{"confirm-unclaim", func(c *cockpit) { c.mode, c.confirm = modeConfirm, confirmUnclaim }},
		{"confirm-kill", func(c *cockpit) { c.mode, c.confirm = modeConfirm, confirmKill }},
		{"peek", func(c *cockpit) { c.mode = modePeek; c.peekText = strings.Repeat("peek line that is quite long\n", 40) }},
	}
	for _, m := range modes {
		for w := 1; w <= 200; w++ {
			for h := 8; h <= 40; h++ { // h<8 is rangerhq-5qm, a part-(a) escape, still open
				c := qaProgFixture()
				// Put the cursor on an in-progress row: that is the row
				// type this bead adds, and the one the footer keys off.
				c.cursor = len(c.sessions)
				m.set(c)
				c.buildRows()
				c.offset = scrollTo(0, c.cursorRow(), len(c.rows), viewportH(h))
				lines := c.renderLines(w, h)
				if len(lines) != h {
					t.Fatalf("%s %dx%d: %d lines, want %d", m.name, w, h, len(lines), h)
				}
				for i, ln := range lines {
					if n := dispWidth(qaPlain(ln)); n > w {
						if m.name == "peek" || strings.Contains(qaPlain(ln), "more") {
							continue // rangerhq-nm8, a part-(a) escape, still open
						}
						t.Fatalf("%s %dx%d: line %d is %d cells wide: %q", m.name, w, h, i, n, qaPlain(ln))
					}
				}
			}
		}
	}
}

// Every in-progress row must survive the same sweep with hostile content in
// the fields the join controls: a long assignee, a long id, an empty one.
func TestQAInProgressHostileFields(t *testing.T) {
	for _, bad := range []rhq.RepoIssue{
		{BdIssue: rhq.BdIssue{ID: strings.Repeat("i", 80), Title: "t", Assignee: strings.Repeat("a", 60), Updated: qaClock}, Dir: qaDir},
		{BdIssue: rhq.BdIssue{ID: "", Title: "", Assignee: "", Priority: -1}, Dir: ""},
		{BdIssue: rhq.BdIssue{ID: "e-1", Title: "🧛🐿️🙂 emoji title 日本語テキスト", Assignee: "🎭persona", Updated: qaClock.Add(-time.Hour)}, Dir: qaDir},
	} {
		c := qaProgFixture()
		c.inprog = append(c.inprog, bad)
		c.buildRows()
		for w := 1; w <= 200; w++ {
			for _, r := range c.rows {
				ln := qaPlain(renderRow(r, w, false))
				if n := dispWidth(ln); n > w {
					t.Fatalf("w=%d: row %d cells wide: %q", w, n, ln)
				}
			}
		}
	}
}

// ── the holder join (ADR 0004 §2, as amended by rangerhq-gdc) ───────────────

func TestQAHolderJoinPrecision(t *testing.T) {
	c := qaProgFixture()

	// The per-bead session of a DIFFERENT bead must not be mistaken for
	// this bead's holder — that is the whole point of joining on the id.
	other := rhq.RepoIssue{BdIssue: rhq.BdIssue{ID: "b-other", Assignee: "devops"}, Dir: qaDir}
	if got := c.holderState(other); got != noSession {
		t.Errorf("bead with only another bead's session: state %q, want %q", got, noSession)
	}

	// Same persona, same bead id, DIFFERENT repo: not the holder.
	elsewhere := rhq.RepoIssue{BdIssue: rhq.BdIssue{ID: "b-blocked", Assignee: "devops"}, Dir: "/other/repo"}
	if got := c.holderState(elsewhere); got != noSession {
		t.Errorf("same bead id in another repo: state %q, want %q", got, noSession)
	}

	// An unassigned in-progress bead has no holder, and must not join to
	// whatever session happens to be first.
	orphan := rhq.RepoIssue{BdIssue: rhq.BdIssue{ID: "b-orphan"}, Dir: qaDir}
	if s := c.holderSession(orphan); s != nil {
		t.Errorf("unassigned bead joined to %q", s.Name)
	}

	// Dial F wins over the slot when both exist.
	c.sessions = append(c.sessions, rhq.HerdrSession{Name: rhq.SessionFor("devops", qaDir), Status: "idle"})
	held := rhq.RepoIssue{BdIssue: rhq.BdIssue{ID: "b-blocked", Assignee: "devops"}, Dir: qaDir}
	if got := c.holderState(held); got != "blocked" {
		t.Errorf("per-bead session must win over the slot: state %q, want blocked", got)
	}
}

// ── stalled-first (ADR 0004 §2) ─────────────────────────────────────────────

func TestQAStalledFirstExactOrder(t *testing.T) {
	c := qaProgFixture()
	var got []string
	for _, is := range c.inprog {
		got = append(got, is.ID)
	}
	want := []string{"b-blocked", "b-none", "b-slot", "b-working"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("stalled-first order: %v, want %v", got, want)
	}
	// A holder whose session reports an unknown status must not outrank a
	// blocked one.
	if stallRank("blocked") >= stallRank("wat") {
		t.Errorf("unknown status outranks blocked")
	}
	// Ties keep bd's order (SliceStable).
	c2 := qaProgFixture()
	c2.sessions = nil // everything is "no session" — one rank, all ties
	before := append([]rhq.RepoIssue(nil), c2.inprog...)
	c2.sortInProg()
	for i := range before {
		if c2.inprog[i].ID != before[i].ID {
			t.Errorf("all-ties sort reordered: %v", c2.inprog)
			break
		}
	}
}

// ── one bead, one section (ADR 0004 §2) ─────────────────────────────────────

func TestQAReadyFilterBothWays(t *testing.T) {
	inprog := []rhq.RepoIssue{{BdIssue: rhq.BdIssue{ID: "x", Status: "in_progress"}, Dir: "/r"}}
	ready := []rhq.RepoIssue{
		{BdIssue: rhq.BdIssue{ID: "x", Status: "in_progress"}, Dir: "/r"}, // both signals
		{BdIssue: rhq.BdIssue{ID: "y", Status: "in_progress"}, Dir: "/r"}, // status only (repo bd failed)
		{BdIssue: rhq.BdIssue{ID: "z", Status: "open"}, Dir: "/r"},
		{BdIssue: rhq.BdIssue{ID: "x", Status: "open"}, Dir: "/other"}, // same id, other repo: keep
	}
	got := readyOnly(ready, inprog)
	var ids []string
	for _, is := range got {
		ids = append(ids, is.Dir+"|"+is.ID)
	}
	want := "/r|z,/other|x"
	if strings.Join(ids, ",") != want {
		t.Errorf("readyOnly = %v, want %s", ids, want)
	}
}

// ── per-section keys (ADR 0004 §3) ──────────────────────────────────────────

// u must be offered on claimed beads only, and x on sessions only.
func TestQAKeyIsolation(t *testing.T) {
	c := qaProgFixture()
	nSess, nProg := len(c.sessions), len(c.inprog)

	c.cursor = 0 // a session
	c.handleKey([]byte("u"))
	if c.mode != modeNormal {
		t.Errorf("u on a session row entered mode %d", c.mode)
	}
	c.mode = modeNormal

	c.cursor = nSess + nSess // an in-progress row
	c.cursor = nSess
	c.handleKey([]byte("x"))
	if c.mode != modeNormal {
		t.Errorf("x on an in-progress row entered mode %d", c.mode)
	}
	c.mode = modeNormal

	c.handleKey([]byte("u"))
	if c.mode != modeConfirm || c.confirm != confirmUnclaim {
		t.Errorf("u on an in-progress row: mode %d confirm %d", c.mode, c.confirm)
	}
	c.mode = modeNormal

	c.cursor = nSess + nProg // a ready row
	c.handleKey([]byte("u"))
	if c.mode != modeNormal {
		t.Errorf("u on a ready row entered mode %d", c.mode)
	}
}

// d on an in-progress row resumes; on a ready row it does not.
func TestQADResumeFlag(t *testing.T) {
	c := qaProgFixture()
	c.results = make(chan string, 4)
	var gotResume []bool
	var gotID []string
	done := make(chan struct{}, 4)
	c.launcher = func(b rhq.RepoIssue, resume bool) (string, error) {
		gotResume = append(gotResume, resume)
		gotID = append(gotID, b.ID)
		done <- struct{}{}
		return "sess", nil
	}
	c.cursor = len(c.sessions) // in-progress
	c.handleKey([]byte("d"))
	<-done
	c.dispatching = false
	c.cursor = len(c.sessions) + len(c.inprog) // ready
	c.handleKey([]byte("d"))
	<-done
	if len(gotResume) != 2 || !gotResume[0] || gotResume[1] {
		t.Errorf("resume flags %v for %v, want [true false]", gotResume, gotID)
	}
}

// rangerhq-lwx lives on this seam: `d` on the slot-held IN PROGRESS row must
// hand LaunchBead the same bead the row joined (status, assignee, dir), not a
// stripped copy. LaunchBead's slot fallback keys off those three fields.
func TestQADOnSlotHeldRowPassesHolderFields(t *testing.T) {
	c := qaProgFixture()
	c.results = make(chan string, 2)
	done := make(chan rhq.RepoIssue, 1)
	c.launcher = func(b rhq.RepoIssue, resume bool) (string, error) {
		if !resume {
			t.Errorf("d on slot-held row: resume=%v, want true", resume)
		}
		done <- b
		return "sess", nil
	}
	var idx int
	found := false
	for i, is := range c.inprog {
		if is.ID == "b-slot" {
			idx, found = i, true
			break
		}
	}
	if !found {
		t.Fatal("qaProgFixture lost the slot-held row")
	}
	held := c.inprog[idx]
	if s := c.holderSession(held); s == nil || s.Name != rhq.SessionFor("business-manager", qaDir) {
		t.Fatalf("b-slot holder = %+v, want the business-manager slot", c.holderSession(held))
	}
	c.cursor = len(c.sessions) + idx
	c.handleKey([]byte("d"))
	bead := <-done
	if bead.ID != "b-slot" || bead.Status != "in_progress" || bead.Assignee != "business-manager" || bead.Dir != qaDir {
		t.Errorf("d passed %+v, want id=b-slot status=in_progress assignee=business-manager dir=%s", bead, qaDir)
	}
}

// The footer offers the selected section's keys only.
func TestQAFooterPerSection(t *testing.T) {
	c := qaProgFixture()
	for _, tc := range []struct {
		name   string
		cursor int
		want   []string
		absent []string
	}{
		{"sessions", 0, []string{"x kill", "enter focus"}, []string{"u unclaim", "c claim"}},
		{"inprog", len(c.sessions), []string{"u unclaim", "d resume"}, []string{"x kill", "c claim"}},
		{"ready", len(c.sessions) + len(c.inprog), []string{"c claim", "d dispatch"}, []string{"u unclaim", "x kill"}},
	} {
		c.cursor = tc.cursor
		f := qaPlain(strings.Join(c.footerLines(200), "\n"))
		for _, w := range tc.want {
			if !strings.Contains(f, w) {
				t.Errorf("%s footer missing %q:\n%s", tc.name, w, f)
			}
		}
		for _, a := range tc.absent {
			if strings.Contains(f, a) {
				t.Errorf("%s footer offers %q:\n%s", tc.name, a, f)
			}
		}
	}
}

// enter/p/v on a claimed bead with no holder must say so, not fall silent
// and not act on some other session.
func TestQANoHolderStatusLine(t *testing.T) {
	for _, key := range []string{"\r", "p", "v"} {
		c := qaProgFixture()
		c.sessions = nil // nobody holds anything
		c.sortInProg()
		c.buildRows()
		c.cursor = 0 // first in-progress row
		c.status = ""
		if _, err := c.handleKey([]byte(key)); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(c.status, "no session") {
			t.Errorf("key %q on a holderless bead: status %q", key, c.status)
		}
		if c.mode != modeNormal {
			t.Errorf("key %q on a holderless bead entered mode %d", key, c.mode)
		}
	}
}

// ── age ─────────────────────────────────────────────────────────────────────

func TestQAShortAgeEdges(t *testing.T) {
	for _, tc := range []struct{ d, want string }{
		{"0s", "0s"}, {"59s", "59s"}, {"60s", "1m"}, {"59m59s", "59m"},
		{"1h", "1h"}, {"23h59m", "23h"}, {"24h", "1d"}, {"400h", "16d"},
	} {
		d, _ := time.ParseDuration(tc.d)
		if got := shortAge(qaClock, qaClock.Add(-d)); got != tc.want {
			t.Errorf("shortAge(-%s) = %q, want %q", tc.d, got, tc.want)
		}
	}
	if got := shortAge(qaClock, time.Time{}); got != "-" {
		t.Errorf("no timestamp: %q, want -", got)
	}
	// A clock skew (updated_at in the future) must not print a negative age.
	if got := shortAge(qaClock, qaClock.Add(time.Hour)); strings.HasPrefix(got, "-") {
		t.Errorf("future updated_at: %q", got)
	}
}

// The `u` key is destructive and confirmed a keystroke later. Between the two
// keystrokes the only thing that refreshes is the results channel (a launch
// finishing), which calls refresh() in any mode. Does the confirm keep
// pointing at the bead the operator aimed at?
func TestQAUnclaimTargetSurvivesRefresh(t *testing.T) {
	t.Skip("rangerhq-sei: the confirm re-reads the cursor, so a refresh mid-confirm retargets the unclaim")
	c := qaProgFixture()
	c.cursor = len(c.sessions) + 1 // second in-progress row
	aimed := c.selInProg().ID
	c.handleKey([]byte("u"))
	if c.mode != modeConfirm || c.confirm != confirmUnclaim {
		t.Fatalf("u did not arm the confirm")
	}

	// A refresh in which the aimed bead still exists but the list re-sorts
	// (its holder went from working to blocked, say).
	sel := c.selected()
	reordered := []rhq.RepoIssue{c.inprog[3], c.inprog[1], c.inprog[0], c.inprog[2]}
	c.inprog = reordered
	c.cursor = reselect(c.sessions, c.inprog, c.issues, sel)
	if got := c.selInProg().ID; got != aimed {
		t.Errorf("confirm retargeted after a re-sort: aimed %s, now %s", aimed, got)
	}
	if !strings.Contains(qaPlain(strings.Join(c.footerLines(120), "\n")), "unclaim "+aimed) {
		t.Errorf("footer no longer names the aimed bead")
	}

	// A refresh in which the aimed bead is gone (the holder closed it).
	sel = c.selected()
	var left []rhq.RepoIssue
	for _, is := range c.inprog {
		if is.ID != aimed {
			left = append(left, is)
		}
	}
	c.inprog = left
	c.cursor = reselect(c.sessions, c.inprog, c.issues, sel)
	if is := c.selInProg(); is != nil && is.ID != aimed {
		t.Errorf("ESCAPE: confirm now aims at %s, the operator aimed at %s", is.ID, aimed)
	}
}
