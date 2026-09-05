package main

// QA pins for ranger-base-iyuc's close (verified under ranger-base-cb5mg).
//
// Two things the close left unmeasured, one of them a live defect:
//
//  1. TestCockpitEventLoopsWireTheHintChannel greps the WHOLE file for six
//     strings, and "c.startHints(hintCtx)" appears twice — once per loop.
//     Deleting the display-only loop's call leaves it green, so that loop
//     can stop subscribing entirely without a red: its select case is still
//     in the source, reading a nil channel that never fires. Measured: the
//     mutant survives the whole cmd/posse suite. The pin below reads each
//     loop's own body instead.
//
//  2. A hint in normal mode ran refresh(), which forces a bead scan past the
//     cadence floor (ranger-base-u5rqp). The second pin asserted that hole
//     with the inversion in its failure message; the fix landed, the pin went
//     red, and it is inverted below — it now asserts the floor holds under an
//     event stream, over a rig shown able to start a scan.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

// loopBody returns one function's source, from its header to the next
// top-level func. A pin that reads the whole file cannot tell which of two
// loops carries a line that appears in both.
func loopBody(t *testing.T, src, header string) string {
	t.Helper()
	i := strings.Index(src, header)
	if i < 0 {
		t.Fatalf("cockpit.go no longer has %q — re-aim this pin", header)
	}
	rest := src[i+len(header):]
	if end := strings.Index(rest, "\nfunc "); end >= 0 {
		return rest[:end]
	}
	return rest
}

// ADR 0016 §2 and ranger-base-iyuc's done-when: BOTH cockpit loops select on
// the hint channel — which starts with both of them subscribing. A loop that
// keeps its select case but never calls startHints reads a nil channel: the
// case is in the source, the events never arrive, and the operator's ⛔ is
// back at tick latency in that mode with nothing red to say so.
func TestBothCockpitLoopsSubscribeAndSelectOnHints(t *testing.T) {
	b, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, loop := range []struct {
		header string
		wants  []string
	}{
		// The tty loop: subscribes, selects, applies through its own filter,
		// and spends a deferred hint on the return to normal mode.
		{"func runCockpit(", []string{"c.startHints(hintCtx)", "<-c.hints", "c.applyHint(h)", "c.consumeHintDirty()"}},
		// The display-only loop: subscribes and selects. It has no modes to
		// draw over, so the hint only brings the next full frame forward.
		{"func (c *cockpit) displayOnly()", []string{"c.startHints(hintCtx)", "<-c.hints"}},
	} {
		body := loopBody(t, src, loop.header)
		for _, want := range loop.wants {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not %s — that loop is not on the ADR 0016 hint channel, whatever the rest of the file does", loop.header, want)
			}
		}
	}
}

// ranger-base-u5rqp, FIXED: a hint reaches the bead lists the way the 2s tick
// does — refreshHint is refreshSessions + kickBeads(FALSE) — so an external
// event stream cannot drive bd past the cadence floor whose own comment says
// it holds bd "to at most half the wall clock wherever the cockpit is
// opened". The callers allowed past it stay what that comment lists: operator
// keys and a landed dispatch, each of which just changed the bead lists.
//
// This pin is the inversion of the one that asserted the defect. It asserts
// that nothing happened, so it carries the arm that shows the rig CAN make it
// happen: refresh() on the same cockpit, at the same point in the same floor,
// must still start a scan.
func TestCockpitHintKeepsTheBeadScanFloor(t *testing.T) {
	home := t.TempDir()
	hb := fakeAgentBackend(t, home, [2]string{"w1", "w1:p1"})
	c := &cockpit{app: &posse.App{Home: home}, hb: hb, mode: modeNormal,
		beads: make(chan beadRead, 8)}
	// As if a scan just landed and cost ten seconds, which is this store's
	// measured figure: nothing may scan again for ten seconds.
	c.beadsAt = time.Now()
	c.beadsNext = time.Now().Add(10 * time.Second)

	// The control arm, and the reason this is a floor and not a cadence: the
	// two-second tick asks the same question and is held.
	c.kickBeads(false)
	if c.beadsIn {
		t.Fatal("the 2s tick's kick must be held by the floor — this pin measures nothing if it is not")
	}

	c.applyHint(posse.HerdrHint{Kind: "pane_agent_status_changed", PaneID: "w1:p1", AgentStatus: "blocked"})
	if c.beadsIn {
		t.Fatal("a hint started a bead scan inside the floor: ranger-base-u5rqp is back — applyHint must reach the lists through refreshHint (kickBeads(false)), not refresh()")
	}
	// The session half is what the event is about, and it must NOT be on the
	// floor: a pin that let both halves go quiet would be green on a hint
	// path that had stopped working altogether.
	if len(c.sessions) == 0 {
		t.Fatal("a hint must still re-read the sessions at event latency (ADR 0016 §2) — refreshHint dropped the half it is for")
	}

	// And there is no force left over to chain with: a second event mid-floor
	// remembers nothing, because it never forced in the first place.
	c.applyHint(posse.HerdrHint{Kind: "pane_agent_status_changed", PaneID: "w1:p1", AgentStatus: "working"})
	if c.beadsIn || c.beadsDirty {
		t.Fatalf("a second hint must neither start nor remember a scan: beadsIn=%v beadsDirty=%v", c.beadsIn, c.beadsDirty)
	}

	// consumeHintDirty is the same event arriving under a mode and spent on
	// the return to normal — the other half of applyHint, and the other call
	// site that used to force. A pin on one of them leaves the other free.
	c.mode = modePrompt
	c.applyHint(posse.HerdrHint{Kind: "workspace_closed", WorkspaceID: "w9"})
	if !c.hintDirty {
		t.Fatal("setup: a hint under a mode must defer, or the arm below spends nothing")
	}
	c.mode = modeNormal
	c.consumeHintDirty()
	if c.beadsIn || c.beadsDirty {
		t.Fatalf("a deferred hint spent on the return to normal must keep the floor too: beadsIn=%v beadsDirty=%v", c.beadsIn, c.beadsDirty)
	}

	// The rig CAN start a scan from exactly here. Without this arm every
	// assertion above is green on a cockpit that simply cannot scan.
	c.refresh()
	if !c.beadsIn {
		t.Fatal("rig: an operator refresh must still force past the floor — if it does not, the arms above measured nothing")
	}
	r := <-c.beads
	r.took = 10 * time.Second
	c.applyBeads(r)
	if time.Until(c.beadsNext) < 9*time.Second {
		t.Fatalf("setup: the floor applyBeads just set should be ~10s away, is %v", time.Until(c.beadsNext))
	}
	if c.beadsIn {
		t.Fatal("nothing forced a scan mid-flight, so none may start inside the fresh floor")
	}
}

// The other end of a coupling that shipped with only one end read
// (ranger-base-43ux4, escaped from ranger-base-0b0qg finding 3).
//
// internal/posse's TestQAHerdrRedialFloorStaysUnderItsSweep bounds
// herdrRedialFloor above by ADR 0016 §1's sweep — "the cockpit's two-second
// completeness tick" — and spells that bound as a literal, because the tick
// is cmd/posse's and the pin is internal/posse's. That literal is a hand-copy
// of the constant below, and until this pin nothing in the tree read the
// SHIPPED tick: lower cockpit.go to a one-second tick and the ceiling pin
// stays GREEN over a floor that now equals the sweep, which is the state its
// own failure message forbids. Measured before this pin existed.
//
// So this is the N-1 edge, not a second copy of the claim: the ceiling pin
// owns "floor < 2s", this one owns "2s is still what ships". Both loops
// carry the tick and both are read here, because a sweep that covers the
// delayed pane in one mode and not the other is the same hole one mode over.
//
// If the tick moves on purpose, this pin and the ceiling pin's literal move
// together — that is the coupling, and it is meant to be felt.
func TestCockpitCompletenessTickIsTheSweepTheRedialFloorIsBoundedBy(t *testing.T) {
	b, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	const tick = "time.NewTicker(2 * time.Second)"
	for _, header := range []string{"func runCockpit(", "func (c *cockpit) displayOnly()"} {
		if !strings.Contains(loopBody(t, src, header), tick) {
			t.Errorf("%s no longer declares %s — ADR 0016 §1 bounds herdrRedialFloor above by this sweep, "+
				"and internal/posse's TestQAHerdrRedialFloorStaysUnderItsSweep spells that bound as a literal 2s. "+
				"Move both, or the floor outlives the timer that covers it with nothing red to say so", header, tick)
		}
	}
}
