package main

// The header's governance segment, and the non-tty rendering that had none
// (bead rangerhq-mgvx, on rangerhq-81y0's surface).
//
// The governance-surface ADR §2 gives the cockpit header one job the block
// cannot do: "a dead loop pulses nobody, and the residual witness is the
// operator's glance at the cockpit header." The block is body — it scrolls
// away, and nothing scrolls it back — so the answer lives in the header and
// the reasons live in the block. Measured on the scratch rig
// (scripts/verify-govern-honesty.sh) before this: `posse status` said
// `URGENT G7` and exited 1 while the piped cockpit drew a clean shop, because
// displayOnly never ran the check at all.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/rhq"
)

// headerOf is the first line of the render — the one line that is not in the
// viewport and cannot scroll.
func headerOf(c *cockpit, w, h int) string {
	return stripANSI(strings.SplitN(strings.Join(c.renderLines(w, h), "\n"), "\n", 2)[0])
}

// Nothing scanned yet is UNKNOWN, and unknown is not clear. Reading a silent
// header as an all-clear is the same mistake as reading an unprobeable lock
// as "no loop", one rendering down.
func TestCockpitGovHeaderIsUnknownBeforeTheFirstScan(t *testing.T) {
	c := fixture()
	c.buildRows()
	got := headerOf(c, 140, 40)
	if !strings.Contains(got, "gov …") {
		t.Errorf("an unscanned header must say so, got %q", got)
	}
	if strings.Contains(got, "clear") {
		t.Errorf("a check that has not run has not found nothing: %q", got)
	}
}

func TestCockpitGovHeaderSaysClearWhenTheCheckFoundNothing(t *testing.T) {
	c := fixture()
	c.govAt = c.clock()
	c.buildRows()
	if got := headerOf(c, 140, 40); !strings.Contains(got, "gov clear") {
		t.Errorf("want `gov clear`, got %q", got)
	}
}

// G7 is named, not counted. Every other row in that count is a condition
// somebody still has to be told about and the loop is what tells them, so
// "1 URGENT" alone understates by exactly the row that matters most.
func TestCockpitGovHeaderNamesTheDeadLoop(t *testing.T) {
	c := govFixture()
	c.govAt = c.clock()
	c.buildRows()
	got := headerOf(c, 140, 40)
	if !strings.Contains(got, "gov 1 URGENT · 2 LANE") {
		t.Errorf("want the summary in the header, got %q", got)
	}
	if !strings.Contains(got, "loop dead") {
		t.Errorf("G7 must be named in the header, got %q", got)
	}
}

// A set with conditions but no G7 must not say it: the phrase is the probe's
// answer, never decoration on a busy shop.
func TestCockpitGovHeaderSaysLoopDeadOnlyForG7(t *testing.T) {
	c := fixture()
	c.govAt = c.clock()
	c.gov = rhq.GovSet{
		{ID: "G1", Class: rhq.GovLane, Key: "blocked:devops-x", Detail: "devops-x is blocked"},
		{ID: "G6", Class: rhq.GovUrgent, Key: "budget-stop:day", Detail: "budget stop"},
	}
	c.buildRows()
	got := headerOf(c, 140, 40)
	if strings.Contains(got, "loop dead") {
		t.Errorf("no G7 in the set, so no dead loop in the header: %q", got)
	}
	if !strings.Contains(got, "gov 1 URGENT · 1 LANE") {
		t.Errorf("want the summary, got %q", got)
	}
}

// An unreadable store is not an all-clear in the header either.
func TestCockpitGovHeaderSaysPartial(t *testing.T) {
	c := fixture()
	c.govAt = c.clock()
	c.govFailed = 2
	c.buildRows()
	if got := headerOf(c, 140, 40); !strings.Contains(got, "gov clear · partial") {
		t.Errorf("want the partial marker, got %q", got)
	}
}

// The load-bearing one. Scroll the viewport past the GOVERNANCE block — one
// press of `down` from a full queue does it — and the block is gone while
// the header still answers. This is the whole reason the segment exists: the
// ADR gives the header the residual-witness job, and a witness that scrolls
// out of the frame is not one.
func TestCockpitGovHeaderSurvivesTheBlockScrollingAway(t *testing.T) {
	c := govFixture()
	c.buildRows()
	c.govAt = c.clock()
	const w, h = 140, 16
	c.cursor = c.items() - 1
	c.offset = scrollTo(0, c.cursorRow(), len(c.rows), viewportH(h))
	if c.offset == 0 {
		t.Fatal("fixture did not scroll; the test would pass without proving anything")
	}
	out := stripANSI(strings.Join(c.renderLines(w, h), "\n"))
	if strings.Contains(out, "GOVERNANCE") || strings.Contains(out, "no watch loop holds") {
		t.Fatalf("the block was still on screen — this test needs it gone to mean anything:\n%s", out)
	}
	if !strings.Contains(headerOf(c, w, h), "loop dead") {
		t.Errorf("the header lost the answer when the block scrolled away:\n%s", out)
	}
}

// The non-tty rendering: a landed shop check becomes a drawn line, header
// included. displayOnly's own scan is pinned live, by the script — this is
// the half that can be pinned hermetically.
func TestCockpitDisplayFrameDrawsALandedShopCheck(t *testing.T) {
	c := govFixture()
	var buf bytes.Buffer
	c.out = &buf
	c.govs = make(chan govRead, 1)
	c.govs <- govRead{set: c.gov, failed: 1}
	c.gov = nil

	if !c.takeGov() {
		t.Fatal("a landed check was not taken")
	}
	if c.govAt.IsZero() {
		t.Error("takeGov must stamp govAt, or the header keeps saying unknown")
	}
	c.drawPlain()
	out := stripANSI(buf.String())
	if !strings.Contains(out, "loop dead") {
		t.Errorf("the non-tty header did not name the dead loop:\n%s", out)
	}
	if !strings.Contains(out, "GOVERNANCE") {
		t.Errorf("the non-tty rendering drew no block:\n%s", out)
	}
	// Second take with nothing waiting must not block or clear the set.
	if c.takeGov() {
		t.Error("takeGov invented a read")
	}
	if len(c.gov) == 0 {
		t.Error("an empty channel must not erase the last answer")
	}
}
