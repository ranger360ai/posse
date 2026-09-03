package main

// QA pin from verify bead ranger-base-qca6i, measured against the close of
// ranger-base-lpoui (b1eb2bf).

import (
	"strings"
	"testing"
)

// The narrow pane is the whole reason this line is a ROW and not a header
// column. cockpit.go's planStale field says so: the header's flex column
// already carries the clock, the version, the guard's reading and the codex
// hint and truncates from its tail, so a 130-character sentence appended
// there is either everything else thrown away or itself thrown away — while
// "on a row of its own the same 80-column pane keeps the age, the timestamp
// and the reading and clips only the streak clause."
//
// That is the argument the design rests on and it was asserted at 200
// columns, where nothing clips at all. MEASURED at 80 (verify
// ranger-base-qca6i) the claim HOLDS, exactly as written — the age, the
// timestamp and both windows survive and the ellipsis lands inside "ruling
// on it under the headroom rule":
//
//	plan meter BLIND 10h09m: last reading 2026-09-02T03:23Z (5h 41% · 7d 89%) — rul…
//
// Pinned at the width the claim is about, because a pane on this box IS 80
// columns and the three facts this bead was filed to make loud are the
// three that a wider fixture can never prove survive.
func TestQACockpitStaleRowKeepsItsFactsAtEightyColumns(t *testing.T) {
	c := fixture()
	c.planStale = lpouiLine
	c.buildRows()

	for _, w := range []int{80, 100, 120} {
		var got string
		for _, l := range strings.Split(stripANSI(strings.Join(c.renderLines(w, 40), "\n")), "\n") {
			if strings.Contains(l, "plan meter BLIND") {
				got = l
			}
		}
		if got == "" {
			t.Fatalf("width %d: the row is not drawn at all", w)
		}
		// The age, when the reading was taken, and the reading itself. The
		// tail clause ("ruling on it…; N consecutive 429") may clip; these
		// four may not.
		for _, must := range []string{"BLIND 10h09m", "2026-09-02T03:23Z", "5h 41%", "7d 89%"} {
			if !strings.Contains(got, must) {
				t.Errorf("width %d lost %q — the row has become as narrow as the header column it was kept out of:\n%s", w, must, got)
			}
		}
	}
}
