package main

// ADR 0034 D3, the cockpit half: codex's on-disk meter reaches the header as
// a HINT — rendered only when this box has a reading, always with the
// reading's age, and never touching a single state the plan guard's own
// segment renders.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

// seedCodexRollout writes one rollout line carrying a rate_limits reading,
// the way codex itself appends it. Hand-written JSON rather than built
// through the reader's own types on purpose: this pins the bytes on disk,
// which is the only contract this feature has with codex.
func seedCodexRollout(t *testing.T, home string, at time.Time, windows string) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions",
		at.Format("2006"), at.Format("01"), at.Format("02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"timestamp":"` + at.Format(time.RFC3339Nano) + `","type":"event_msg",` +
		`"payload":{"type":"token_count","rate_limits":{` + windows +
		`,"credits":{"has_credits":false,"unlimited":false,"balance":"0"}}}}` + "\n"
	name := "rollout-" + at.Format("2006-01-02T15-04-05") + "-fixture.jsonl"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
}

// oneWindow is a single 7d window at pct, resetting at resets.
func oneWindow(pct float64, resets time.Time) string {
	return fmt.Sprintf(`"primary":{"used_percent":%g,"window_minutes":10080,"resets_at":%d}`, pct, resets.Unix())
}

// The header carries the hint after the guard's own reading, and carries
// nothing at all when there is no reading — the two states that make this a
// hint rather than a row.
func TestCockpitHeaderCarriesTheCodexHint(t *testing.T) {
	at := time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC)
	c := fixture()
	c.now = func() time.Time { return at }
	c.planHint = &posse.PlanHint{
		At:      at.Add(-3 * time.Hour),
		Windows: []posse.HintWindow{{Name: "codex_7d", UsedPercent: 62, ResetsAt: at.Add(24 * time.Hour)}},
	}
	head := stripANSI(c.renderLines(200, 24)[0])
	if want := "codex 7d 62%, as of 3h00m ago"; !strings.Contains(head, want) {
		t.Errorf("the hint never reached the header (want %q):\n%s", want, head)
	}
	// After the guard's reading, not before it: the flex column truncates
	// from its tail, so the segment that gates nothing must be the one a
	// narrow pane loses first.
	if i, j := strings.Index(head, "5h 42%"), strings.Index(head, "codex 7d"); i < 0 || j < i {
		t.Errorf("the hint must follow the guard's reading, not lead it:\n%s", head)
	}
	// The other state, and the common one.
	c.planHint = nil
	if head := stripANSI(c.renderLines(200, 24)[0]); strings.Contains(head, "codex") || strings.Contains(head, " · ·") {
		t.Errorf("no reading must leave the header clean:\n%s", head)
	}
}

// The age is rendered from the clock at DRAW time, not frozen at the
// two-minute plan tick that read it. A segment formatted once in applyPlan
// would pass every assertion above and then sit on the header claiming to be
// up to two minutes younger than it is — and the age is the load-bearing
// half of a hint whose file only moves when codex takes a turn on this box.
func TestCockpitCodexHintAgesBetweenPlanTicks(t *testing.T) {
	read := time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC)
	c := fixture()
	c.planHint = &posse.PlanHint{
		At:      read,
		Windows: []posse.HintWindow{{Name: "codex_7d", UsedPercent: 62, ResetsAt: read.Add(24 * time.Hour)}},
	}
	for _, w := range []struct {
		after time.Duration
		want  string
	}{
		{time.Minute, "codex 7d 62%, as of 1m ago"},
		{97 * time.Minute, "codex 7d 62%, as of 1h37m ago"},
		// And past the window's own reset, with the hint untouched: the
		// percent stops being shown because the clock moved, not because
		// anything re-read the file.
		{25 * time.Hour, "codex 7d reset, as of 25h00m ago"},
	} {
		c.now = func() time.Time { return read.Add(w.after) }
		if head := stripANSI(c.renderLines(200, 24)[0]); !strings.Contains(head, w.want) {
			t.Errorf("%s after the reading, want %q:\n%s", w.after, w.want, head)
		}
	}
}

// The wiring pin. The two tests above set c.planHint by hand and would stay
// green with the scan deleted, the apply deleted, or both — so this walks
// the whole path: a rollout on disk → scanPlan → the channel → applyPlan →
// the drawn header. It also pins the half of D3 that is an absence: a box
// with a codex reading and no plan guard configured still says nothing about
// the guard, because a hint has no clock and starts no blind window.
func TestCockpitCodexHintComesFromTheRolloutsOnDisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	at := time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC)
	seedCodexRollout(t, home, at.Add(-42*time.Minute), oneWindow(62, at.Add(24*time.Hour)))

	c := fixture()
	c.app, c.now = posse.NewAppAt(filepath.Join(home, "config")), func() time.Time { return at }
	c.planLine = ""
	c.plans = make(chan planRead, 1)
	c.scanPlan()
	select {
	case r := <-c.plans:
		c.applyPlan(r)
	default:
		t.Fatal("the scan landed nothing on the channel")
	}
	head := stripANSI(c.renderLines(200, 24)[0])
	if want := "codex 7d 62%, as of 42m ago"; !strings.Contains(head, want) {
		t.Errorf("the reading on disk never reached the header (want %q):\n%s", want, head)
	}
	// No guard configured on this box, and the hint may not invent one: no
	// blind clock, no "guard off", nothing.
	if strings.Contains(head, "guard") || strings.Contains(head, "plan —") {
		t.Errorf("a hint must not put the guard in any state:\n%s", head)
	}
}
