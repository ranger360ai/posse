//go:build posse_arm3

package posse

// QA pins for ranger-base-2jl1's close (verified under ranger-base-cb5mg).
//
// The close is right; what it shipped without is a fixture that can tell an
// EPOCH sum from a DAY sum. Every GovInputs.Spend fixture in this package
// puts its whole spend at `govNow`, so `PassTotal(epochStart)` and
// `DayTotal(now)` return the same number and two mutants survive the suite:
//
//	st.PassSpend = rep.DayTotal(now)   // the epoch window is the day's spend
//	since := epochStart                // the day window loses the morning
//
// Both were run against the package at 2209f24 and no test went red. These
// two do.

import (
	"testing"
	"time"
)

// twoEpochReport is a ledger with spend in TWO windows of the same day: one
// segment inside the current epoch and one an hour before it opened. The
// day window must count both; the epoch window only the first. A fixture
// with a single segment at `now` cannot tell the two apart, which is how
// the epoch reading went in unmeasured.
func twoEpochReport(epochStart time.Time, inEpoch, earlier float64) *CostReport {
	return &CostReport{Beads: []*Segment{
		{Bead: "in-epoch", Start: epochStart, CostUSD: inEpoch},
		{Bead: "earlier-today", Start: epochStart.Add(-time.Hour), CostUSD: earlier},
	}}
}

// G6's epoch window is an EPOCH, not a second name for the day: money spent
// earlier today, in a window that has already turned, is not spend against
// the cap the operator armed for THIS epoch (ADR 0029 §1 amendment).
func TestGovG6EpochWindowExcludesSpendFromAnEarlierEpoch(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "budget_day: 100\nbudget_pass: 5\ndispatch_epoch: 2h\n")
	in := govIn(t, b)

	es := EpochStart(govNow, 2*time.Hour)
	if !es.After(startOfDay(govNow)) {
		t.Fatalf("setup: the epoch must open after midnight for this fixture to have two windows in one day (epoch start %s, midnight %s)", es, startOfDay(govNow))
	}
	in.Spend = func(time.Time) *CostReport { return twoEpochReport(es, 4, 40) }

	st := in.dialE(govNow, nil)
	if st.PassSpend != 4 {
		t.Errorf("epoch spend = %.2f, want 4 — only the segment inside the current epoch counts; %.2f means the day's total is wearing the epoch cap's name", st.PassSpend, st.DaySpend)
	}
	if st.DaySpend != 44 {
		t.Errorf("day spend = %.2f, want 44 — the day window keeps every segment since midnight", st.DaySpend)
	}
	// And the row that comes out of it: 4 of a $5 epoch cap is 80%, which
	// beats 44 of a $100 day, so the tightest window is the epoch. Read the
	// day sum into that slot and it reads 880% — the same ledger, a window
	// the operator never armed.
	if st.Window != "epoch" || st.Pct != 80 {
		t.Errorf("resolve() = %s %.0f%%, want epoch 80%%", st.Window, st.Pct)
	}
}

// The scan floor dialE hands the ledger is local MIDNIGHT — never the epoch
// start, however late in the day the epoch opened. One scan feeds both
// windows (the close's own words), and a scan that starts at the epoch
// silently loses every dollar the day spent before it: the day window is
// then a floor wearing a total's name, and nothing else in this package
// reads that argument.
func TestGovG6ScanFloorIsMidnightNotTheEpoch(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "budget_day: 100\nbudget_pass: 10\ndispatch_epoch: 2h\n")
	in := govIn(t, b)

	var since time.Time
	in.Spend = func(t time.Time) *CostReport {
		since = t
		return twoEpochReport(EpochStart(govNow, 2*time.Hour), 4, 40)
	}
	in.dialE(govNow, nil)

	if want := startOfDay(govNow); !since.Equal(want) {
		t.Errorf("dialE scanned from %s, want local midnight %s — a floor at the epoch start (%s) drops the morning out of the day window",
			since, want, EpochStart(govNow, 2*time.Hour))
	}
}
