package main

import (
	"strings"
	"testing"

	posse "github.com/ranger360ai/posse/internal/posse"
)

// LIVE DEFECT, pinned GREEN — the shape a DECLINED defect gets, not a park
// (gateschainpathleak_qa_test.go is the other one).
//
// ranger-base-7bdbb, found verifying ranger-base-qmjc under
// ranger-base-37buk. qmjc closed the seven Unicode 16.0 Emoji_Presentation
// additions and stopped there, by its own scope note. Unicode 16.0 moved 198
// code points into East_Asian_Width Wide/Fullwidth, and the other 191 are
// still absent from wideRanges — same table, same undercount, same wrapped
// row. Measured on this box against the terminal's own cursor advance (tmux
// 3.7b): each of the seven sampled below advances TWO cells and dispWidth
// counts ONE.
//
// ranger-base-7bdbb was CLOSED "not doing" by the operator triage sweep of
// 2026-09-04: "cockpit cell-width for 191 rare wide code points;
// nobody has hit one". The table is therefore not moving, and this file was
// a t.Skip parked on a bead that will never land — an instruction to
// un-skip (which reds the suite: measured 7/7 failing at 374d3b8) attached
// to a decision that the rows stay wrong. Re-verified under
// ranger-base-09yjv and inverted instead: the assertion below is the
// UNDERCOUNT, so the declined hole is measured on every run rather than
// skipped, and the pin goes RED the day the table moves — which is the day
// to delete this file and restore the seven `want: 2` rows from the prose
// above.
//
// The controls are in the same table and are not decoration: without a
// glyph the table DOES carry, a pin over dispWidth would pass a build where
// every width answer was 1.
func TestQACockpitStillUndercountsTheUnicode16WideAdditions(t *testing.T) {
	for _, c := range []struct {
		in       string
		want     int // what dispWidth answers TODAY
		terminal int // what the terminal actually advances
	}{
		{"☰", 1, 2},          // trigram, U+2630..U+2637 — W in 16.0, Neutral in 15.1
		{"⚊", 1, 2},          // monogram, U+268A..U+268F
		{"㇤", 1, 2},          // U+31E4..U+31E5
		{"䷀", 1, 2},          // Yijing hexagram, U+4DC0..U+4DFF (64 of them)
		{"\U00018CFF", 1, 2}, // U+18CFF
		{"\U0001D300", 1, 2}, // Tai Xuan Jing, U+1D300..U+1D356 (87)
		{"\U0001D360", 1, 2}, // U+1D360..U+1D376 (23)

		{"x", 1, 1},          // control: narrow, and stays narrow
		{"　", 2, 2},          // control: fullwidth space, tabled since 15.1
		{"\U0001F7E1", 2, 2}, // control: the glyph rangerhq-53p was filed on
		{"\U0001FAE9", 2, 2}, // control: one of qmjc's seven
	} {
		got := dispWidth(c.in)
		if got == c.want {
			continue
		}
		if c.want != c.terminal && got == c.terminal {
			t.Errorf("FIXED: dispWidth(%q U+%04X) = %d, which is what the terminal advances — wideRanges has taken the Unicode 16.0 additions, so delete this pin and restore the `want: 2` rows (ranger-base-7bdbb, closed not doing 2026-09-04)",
				c.in, []rune(c.in)[0], got)
			continue
		}
		t.Errorf("dispWidth(%q U+%04X) = %d, not the %d this table has answered since ranger-base-qmjc (the terminal advances %d)",
			c.in, []rune(c.in)[0], got, c.want, c.terminal)
	}

	// End to end, the report's own repro with one of the missed code points:
	// the layout believes this row is exactly 60 cells and the terminal draws
	// 61, wrapping the trailing … onto the next line (measured cursor_x=1
	// cursor_y=1 in a 60-column pane).
	//
	// This arm is a LAYOUT control, not a second witness of the defect: it
	// holds on both sides of the fix (measured — with the seven ranges
	// tabled, renderRow clips to the wider glyph and dispWidth still answers
	// 60), so it says only that the row the layout builds is the width the
	// layout asked for. What differs across the fix is what the TERMINAL
	// draws, which no in-process assertion can reach; the cursor_x
	// measurement above is the record of that.
	is := posse.RepoIssue{BdIssue: posse.BdIssue{ID: "rangerhq-yel", Priority: 1,
		Title: "䷀ flaky suite " + strings.Repeat("x", 60)}, Dir: "/Users/x/src/posse"}
	if got := dispWidth(qaPlain(renderRow(row{kind: rowItem, cols: issueCols(is, 14)}, 60, false))); got != 60 {
		t.Errorf("the row the layout believes is 60 cells measures %d — the terminal draws 61 either way (ranger-base-7bdbb)", got)
	}
}
