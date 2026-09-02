package main

// The two rendering halves of ranger-base-lpoui that live in cmd/: the
// cockpit's loud row and `posse status`'s line. The line itself, its
// arithmetic and its threshold are pinned next to the code that computes
// them (internal/posse/planstale_qa_test.go); these pin that the surfaces
// actually PRINT it, which is the half a refactor drops silently — the
// cockpit's own applyPlan comment says so about the join step, and this
// bead exists because a fact nothing printed was a fact nobody had.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// lpouiLine is the measured line, as the internal renderer produces it.
const lpouiLine = "plan meter BLIND 10h09m: last reading 2026-09-02T03:23Z (5h 41% · 7d 89%) — " +
	"ruling on it under the headroom rule; 10 consecutive 429"

// The row is drawn, whole, above the first section heading. Above because
// the sections scroll and nothing scrolls them back — the same reason the
// GOVERNANCE block sits where it does.
func TestQACockpitDrawsTheStaleRowAboveEverything(t *testing.T) {
	c := fixture()
	c.planStale = lpouiLine
	c.buildRows()
	out := stripANSI(strings.Join(c.renderLines(200, 40), "\n"))

	if n := strings.Count(out, lpouiLine); n != 1 {
		t.Fatalf("want the whole line drawn exactly once, got %d:\n%s", n, out)
	}
	if i, j := strings.Index(out, lpouiLine), strings.Index(out, "SESSIONS"); i < 0 || j < 0 || i > j {
		t.Errorf("the loud row must sit above the first heading (line at %d, SESSIONS at %d):\n%s", i, j, out)
	}
}

// The control: no staleness, no row, no blank line spent on it. A screen
// that gave this two lines every healthy day would be a screen an operator
// learns to read past, which is how the header's own "guard blind" became
// furniture.
func TestQACockpitDrawsNoStaleRowWhenTheReadingIsFresh(t *testing.T) {
	c := fixture()
	c.buildRows()
	fresh := len(c.rows)
	out := stripANSI(strings.Join(c.renderLines(200, 40), "\n"))
	if strings.Contains(out, "plan meter BLIND") {
		t.Fatalf("a fresh reading must draw nothing:\n%s", out)
	}
	c.planStale = lpouiLine
	c.buildRows()
	if len(c.rows) != fresh+2 {
		t.Errorf("the row costs exactly itself and its spacer: %d rows vs %d", len(c.rows), fresh)
	}
}

// The join step: a scan that found staleness has to reach the field the
// draw path reads. It is one assignment, it is the assignment applyPlan's
// own comment warns is the one a refactor drops, and a test that set
// c.planStale by hand (both of the ones above) would stay green without it.
func TestQACockpitPlanScanCarriesTheStaleLineToTheDraw(t *testing.T) {
	c := fixture()
	c.applyPlan(planRead{stale: lpouiLine})
	if c.planStale != lpouiLine {
		t.Fatalf("applyPlan dropped the line: %q", c.planStale)
	}
	c.applyPlan(planRead{})
	if c.planStale != "" {
		t.Fatalf("and it heals when the next scan is fresh: %q", c.planStale)
	}
}

// ─── `posse status` ──────────────────────────────────────────────────────────

// The command prints it, off the files, without asking the endpoint
// anything. Hermetic by RHQ_PLAN_USAGE_URL: a loopback override is asked
// WITHOUT a credential (credpin.go rule 4), so the guard is armed, the read
// fails against a closed port, and no keychain on the box running this
// suite is touched.
//
// The age is not asserted — the subprocess has the real clock — but the
// timestamp, the reading and the streak clause are, and those are the parts
// that come from the stores rather than from `now`.
func TestQAStatusPrintsTheStaleLine(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	repo := t.TempDir()
	at := time.Date(2026, 9, 2, 3, 23, 0, 0, time.UTC)
	writeStatusConfig(t, home, repo, "plan_guard_5h: 70\nplan_guard_7d: 85\n")
	seedStatusReading(t, home, at)
	seedStatusLog(t, home, at, 10)

	out := runStatus(t, bin, home)
	// The COUNT is not pinned here and the exact age is not either: this
	// command's own shop check reads the meter before the line is printed,
	// so its failure joins the streak, and the subprocess has the real
	// clock. Both are pinned where nothing else is asking —
	// internal/posse/planstale_qa_test.go. What is this command's to get
	// right is that it prints the sentence at all, off the stores, with the
	// reading it is ruling on in it.
	const want = "last reading 2026-09-02T03:23Z (5h 41% · 7d 89%) — ruling on it under the headroom rule; "
	if !strings.Contains(out, "plan meter BLIND ") || !strings.Contains(out, want) {
		t.Fatalf("want the loud line, got:\n%s", out)
	}
	if !strings.Contains(out, " consecutive 429") {
		t.Errorf("the streak clause must name the class the log recorded, got:\n%s", out)
	}
}

// Two controls, because a line printed unconditionally would pass the pin
// above. Both are states where the sentence would be a lie: an armed guard
// whose reading is minutes old, and a shop with no meter guard at all —
// where nothing is ruling on the snapshot a cockpit happened to write.
func TestQAStatusQuietWhereTheLineWouldLie(t *testing.T) {
	for _, tc := range []struct {
		name    string
		guard   string
		reading time.Time
	}{
		{"fresh reading", "plan_guard_5h: 70\nplan_guard_7d: 85\n", time.Now().UTC().Add(-time.Minute)},
		{"no meter guard", "", time.Date(2026, 9, 2, 3, 23, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := buildRhq(t)
			home := t.TempDir()
			repo := t.TempDir()
			writeStatusConfig(t, home, repo, tc.guard)
			seedStatusReading(t, home, tc.reading)
			if out := runStatus(t, bin, home); strings.Contains(out, "plan meter BLIND") {
				t.Fatalf("%s must print nothing about blindness:\n%s", tc.name, out)
			}
		})
	}
}

func writeStatusConfig(t *testing.T, home, repo, guard string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"),
		[]byte("beads:\n  - "+repo+"\n"+guard), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedStatusReading(t *testing.T, home string, at time.Time) {
	t.Helper()
	state := filepath.Join(home, "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"at": at, "windows": []map[string]any{
		{"name": "5h", "pct": 41}, {"name": "7d", "pct": 89},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "plan-usage.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedStatusLog(t *testing.T, home string, at time.Time, n int) {
	t.Helper()
	var b strings.Builder
	b.WriteString(at.UTC().Format(time.RFC3339) + " dispatch ok\n")
	for i := 1; i <= n; i++ {
		b.WriteString(at.Add(time.Duration(i)*time.Hour).UTC().Format(time.RFC3339) + " dispatch 429 cooldown=5m\n")
	}
	if err := os.WriteFile(filepath.Join(home, "state", "plan-usage.log"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runStatus runs the command and returns everything it said. The exit code
// is not asserted: with the guard armed and blind, `posse status` is
// SUPPOSED to exit non-zero, and a test that demanded 0 would be asserting
// the opposite of the condition it set up.
func runStatus(t *testing.T, bin, home string) string {
	t.Helper()
	cmd := exec.Command(bin, "status")
	// A loopback usage URL: the adapter serves it without reading any
	// credential store, and port 1 refuses, so the read fails the way a
	// blind box's does.
	cmd.Env = statusEnv(t, home, "RHQ_PLAN_USAGE_URL=http://127.0.0.1:1")
	out, _ := cmd.CombinedOutput()
	return string(out)
}
