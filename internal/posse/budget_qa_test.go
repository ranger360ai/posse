//go:build posse_arm3

package posse

// QA pin for rangerhq-9qho (verify of rangerhq-kiai, duplicate of rangerhq-dsk).
// TestPassAndDayWindows builds 'earlier-today' at startOfDay(now)+1m and
// asserts PassTotal(now-10m)==5. Between local 00:00 and 00:11 inclusive
// that bead sits inside the pass window (PassTotal uses !Before, i.e. >=).
// In the first minute this-pass is yesterday, so DayTotal is 3 not 8 too.
// Production is fine: after midnight a pass that started before midnight
// contains the whole day window, so the two subsets cannot differ.
// dsk's alternative (min(startOfDay+1m, passStart-1m)) moves earlier-today
// into yesterday and DayTotal becomes 5. The fix is a fabricated now
// far from midnight (noon), not a relative placement.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func qaBudgetTestSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "budget_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}

func qaExtractTest(t *testing.T, src, name string) string {
	t.Helper()
	start := strings.Index(src, "func "+name+"(")
	if start < 0 {
		t.Fatalf("%s not in budget_test.go", name)
	}
	rest := src[start:]
	next := strings.Index(rest[1:], "\nfunc ")
	if next < 0 {
		return rest
	}
	return rest[:next+1]
}

// The shape TestPassAndDayWindows should have taken: same three beads,
// clock pinned at local noon so earlier-today (00:01) is before passStart
// (11:50) on the same calendar day.
func TestQAPassAndDayWindowsAtFabricatedNoon(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.Local)
	passStart := now.Add(-10 * time.Minute)
	rep := &CostReport{Beads: []*Segment{
		{Bead: "old-day", Start: now.AddDate(0, 0, -2), CostUSD: 7},
		{Bead: "earlier-today", Start: startOfDay(now).Add(time.Minute), CostUSD: 3},
		{Bead: "this-pass", Start: now.Add(-time.Minute), CostUSD: 5},
	}}
	if got := rep.DayTotal(now); got != 8 {
		t.Errorf("day total %v, want 8 (today's beads only)", got)
	}
	if got := rep.PassTotal(passStart); got != 5 {
		t.Errorf("pass total %v, want 5 (this pass's beads only)", got)
	}
	if got := rep.PassTotal(time.Time{}); got != 0 {
		t.Errorf("no pass in flight must be an empty window, got %v", got)
	}
}

// Live since ranger-base-tvmh: the fixture clock is fabricated and must
// stay that way. A wall clock here makes the suite red for eleven minutes a
// night, which is worse than a plain bug — it makes "suite green" mean "and
// it was not just after midnight", which nobody writes down.
func TestQAPassAndDayWindowsDoesNotUseWallClock(t *testing.T) {
	t.Parallel()
	body := qaExtractTest(t, qaBudgetTestSource(t), "TestPassAndDayWindows")
	if strings.Contains(body, "time.Now()") {
		t.Fatal("TestPassAndDayWindows calls time.Now() again; fails local 00:00–00:11 inclusive (pass total 8, want 5). Pin the fixture clock (noon), do not min(startOfDay+1m, passStart-1m) — that drops DayTotal to 5 at 00:05")
	}
	// The pin is only a pin if it can see the body it names. An extractor
	// that returned "" would pass over any fixture at all.
	if !strings.Contains(body, "startOfDay(now)") || !strings.Contains(body, "PassTotal(passStart)") {
		t.Fatalf("extractor did not return the fixture body — this pin is blind:\n%s", body)
	}
}
