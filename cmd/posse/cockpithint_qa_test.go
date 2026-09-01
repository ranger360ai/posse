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
//  2. A hint in normal mode runs refresh(), which forces a bead scan past
//     the cadence floor (ranger-base-u5rqp). The second pin is GREEN today
//     asserting that hole, with the inversion in its failure message — it
//     goes red the day the fix lands, which is the signal to flip it.

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

// LIVE DEFECT, ranger-base-u5rqp: a hint in normal mode calls refresh(),
// which is refreshSessions + kickBeads(TRUE), and a forced kick ignores the
// bead-scan cadence floor. So an external event stream now drives bd at
// whatever rate herdr emits — the floor's own comment says it exists to hold
// bd "to at most half the wall clock wherever the cockpit is opened", and
// lists the callers allowed past it as operator keys and a landed dispatch.
//
// This pin asserts the hole, so it is green until the fix lands.
func TestCockpitHintForcesABeadScanInsideTheFloor(t *testing.T) {
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
	if !c.beadsIn {
		t.Fatal("a hint no longer starts a bead scan inside the floor: ranger-base-u5rqp is FIXED — invert this pin to assert the floor holds, and delete the chain arm below")
	}

	// And it chains: a force landing mid-scan is remembered and spent the
	// moment the stale scan lands, on top of the floor that scan just set.
	c.applyHint(posse.HerdrHint{Kind: "pane_agent_status_changed", PaneID: "w1:p1", AgentStatus: "working"})
	if !c.beadsDirty {
		t.Fatal("a second hint mid-scan no longer remembers the force: ranger-base-u5rqp is partly fixed — re-read the chain in applyBeads")
	}
	r := <-c.beads
	r.took = 10 * time.Second
	c.applyBeads(r)
	if !c.beadsIn {
		t.Fatal("the remembered force no longer starts a scan inside the fresh floor: ranger-base-u5rqp is FIXED on the chain arm — invert this pin")
	}
	if time.Until(c.beadsNext) < 9*time.Second {
		t.Fatalf("setup: the floor applyBeads just set should be ~10s away, is %v", time.Until(c.beadsNext))
	}
}
