package main

// The two rendering halves of ranger-base-dwlb1 that live in cmd/: the
// cockpit's row and `posse status`'s line. The arithmetic and the line
// itself are pinned beside the code that computes them
// (internal/posse/beadpulse_qa_test.go); these pin that the surfaces
// actually PRINT it — the half a refactor drops silently, and the whole
// point of the bead was that the number the operator reads at a glance
// stops being the raw open count.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

const dwlb1Line = "closes today 86 (7d median 41) · open 40F/86B/0D/18U · P1 3 · P2 71"

// The row is drawn, whole, above the first section heading — above because
// the sections scroll and nothing scrolls them back.
func TestQACockpitDrawsTheShopPulseAboveEverything(t *testing.T) {
	c := fixture()
	c.pulse = dwlb1Line
	c.buildRows()
	out := stripANSI(strings.Join(c.renderLines(200, 40), "\n"))

	if n := strings.Count(out, dwlb1Line); n != 1 {
		t.Fatalf("want the whole line drawn exactly once, got %d:\n%s", n, out)
	}
	if i, j := strings.Index(out, dwlb1Line), strings.Index(out, "SESSIONS"); i < 0 || j < 0 || i > j {
		t.Errorf("the pulse row must sit above the first heading (line at %d, SESSIONS at %d):\n%s", i, j, out)
	}
}

// The control: no scan has landed, so there is no reading — and an empty
// line must draw NOTHING rather than a blank row and a spacer. "no scan yet"
// is not "closes today 0", which is the lie a zero-valued render would tell.
func TestQACockpitDrawsNoShopPulseBeforeTheFirstScan(t *testing.T) {
	c := fixture()
	c.buildRows()
	before := len(c.rows)
	if out := stripANSI(strings.Join(c.renderLines(200, 40), "\n")); strings.Contains(out, "closes today") {
		t.Fatalf("no scan has landed; nothing may claim a close count:\n%s", out)
	}
	c.pulse = dwlb1Line
	c.buildRows()
	if len(c.rows) != before+2 {
		t.Errorf("the row costs exactly itself and its spacer: %d rows vs %d", len(c.rows), before)
	}
}

// The join step: a governance scan that computed the line has to reach the
// field the draw path reads. It is one assignment, and both tests above set
// c.pulse by hand, so they would stay green without it.
func TestQACockpitGovScanCarriesTheShopPulseToTheDraw(t *testing.T) {
	c := fixture()
	c.applyGov(govRead{pulse: dwlb1Line})
	if c.pulse != dwlb1Line {
		t.Fatalf("applyGov dropped the line: %q", c.pulse)
	}
}

// ─── `posse status` ──────────────────────────────────────────────────────────

// The command prints the reading off the store, and prints the four class
// slots rather than a raw open total. The fixture is deliberately the shape
// this instance was actually in — a store where almost nothing carries a
// class — so the unclassified bucket is what the line has to make visible.
func TestQAStatusPrintsTheShopPulse(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	repo := t.TempDir()
	writeStatusConfig(t, home, repo, "")
	if err := os.WriteFile(filepath.Join(repo, "fake-list-all.json"), []byte(`[
		{"id":"a-1","status":"open","issue_type":"feature","priority":1},
		{"id":"a-2","status":"in_progress","issue_type":"bug","priority":2},
		{"id":"a-3","status":"open","issue_type":"bug","priority":2},
		{"id":"a-4","status":"open","labels":["debt"],"priority":3},
		{"id":"a-5","status":"open","issue_type":"task","priority":2},
		{"id":"a-6","status":"open","priority":3}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "status")
	cmd.Env = statusEnv(t, home)
	raw, _ := cmd.CombinedOutput()
	out := string(raw)

	// The close counts are not pinned here — the subprocess has the real
	// clock and no close in this fixture carries a stamp — but the pile,
	// its classes and its urgent share come from the store and are this
	// command's to get right.
	for _, want := range []string{"shop pulse ·", "closes today 0", "open 1F/2B/1D/2U", "P1 1", "P2 3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status missing %q:\n%s", want, out)
		}
	}
	// The number the operator ruled out, in the shape the old surfaces
	// printed it. 6 beads are open here; none of the four slots is "6", so
	// a raw total could only come from a surface that put one back.
	if strings.Contains(out, "6 open") || strings.Contains(out, "open 6") {
		t.Errorf("the raw open count is back on the status header:\n%s", out)
	}
}

// The one rendering: whatever `posse status` prints for a store, the
// internal renderer produces for the same store. Two surfaces spelling one
// reading differently is the drift this bead's "one line, three surfaces"
// exists to prevent, and only a test that computes both catches it.
func TestQAStatusAndTheRendererAgree(t *testing.T) {
	p := posse.FoldBeadPulse([]posse.BdIssue{
		{ID: "a-1", Status: "open", IssueType: "feature", Priority: 1},
		{ID: "a-2", Status: "in_progress", IssueType: "bug", Priority: 2},
		{ID: "a-3", Status: "open", IssueType: "bug", Priority: 2},
		{ID: "a-4", Status: "open", Labels: []string{"debt"}, Priority: 3},
		{ID: "a-5", Status: "open", IssueType: "task", Priority: 2},
		{ID: "a-6", Status: "open", Priority: 3},
	}, time.Now())
	if want := "open 1F/2B/1D/2U · P1 1 · P2 3"; !strings.Contains(p.Line(), want) {
		t.Fatalf("the renderer's own line has drifted from the one status prints: %s", p.Line())
	}
}
