package main

import (
	"strings"
	"testing"

	posse "github.com/ranger360ai/posse/internal/posse"
)

// ranger-base-7bdbb, found verifying ranger-base-qmjc under
// ranger-base-37buk. qmjc closed the seven Unicode 16.0 Emoji_Presentation
// additions and stopped there, by its own scope note. Unicode 16.0 moved 198
// code points into East_Asian_Width Wide/Fullwidth, and the other 191 are
// still absent from wideRanges — same table, same undercount, same wrapped
// row. Measured on this box against the terminal's own cursor advance (tmux
// 3.7b): each of the seven sampled below advances two cells and counts one.
//
// The controls are in the same table and are not decoration: without a glyph
// the table DOES carry, a pin over dispWidth would pass a build where every
// width answer was 2.
func TestQACockpitDispWidthUnicode16WideAdditions(t *testing.T) {
	t.Skip("ranger-base-7bdbb: wideRanges tables the 7 Unicode 16.0 emoji and none of the other 191 Wide additions")
	for _, c := range []struct {
		in   string
		want int
	}{
		{"☰", 2},          // trigram, U+2630..U+2637 — W in 16.0, Neutral in 15.1
		{"⚊", 2},          // monogram, U+268A..U+268F
		{"㇤", 2},          // U+31E4..U+31E5
		{"䷀", 2},          // Yijing hexagram, U+4DC0..U+4DFF (64 of them)
		{"\U00018CFF", 2}, // U+18CFF
		{"\U0001D300", 2}, // Tai Xuan Jing, U+1D300..U+1D356 (87)
		{"\U0001D360", 2}, // U+1D360..U+1D376 (23)

		{"x", 1},          // control: narrow, and stays narrow
		{"　", 2},          // control: fullwidth space, tabled since 15.1
		{"\U0001F7E1", 2}, // control: the glyph rangerhq-53p was filed on
		{"\U0001FAE9", 2}, // control: one of qmjc's seven
	} {
		if got := dispWidth(c.in); got != c.want {
			t.Errorf("dispWidth(%q U+%04X) = %d, the terminal advances %d", c.in, []rune(c.in)[0], got, c.want)
		}
	}

	// End to end, the report's own repro with one of the missed code points:
	// the layout believes this row is exactly 60 cells and the terminal draws
	// 61, wrapping the trailing … onto the next line (measured cursor_x=1
	// cursor_y=1 in a 60-column pane).
	is := posse.RepoIssue{BdIssue: posse.BdIssue{ID: "rangerhq-yel", Priority: 1,
		Title: "䷀ flaky suite " + strings.Repeat("x", 60)}, Dir: "/Users/x/src/posse"}
	if got := dispWidth(qaPlain(renderRow(row{kind: rowItem, cols: issueCols(is, 14)}, 60, false))); got != 60 {
		t.Errorf("row rendered at 60 measures %d", got)
	}
}
