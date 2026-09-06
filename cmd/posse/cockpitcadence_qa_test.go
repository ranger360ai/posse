package main

// ADR 0016's retained-cadence acceptance row, for ranger-base-4dxpo: with
// the event subscription removed, the 2s tick is the ONLY thing that brings
// a herdr change onto the screen, in both modes. Before the removal it was
// the completeness path beside the hint channel; now it is the whole path,
// and a mode that quietly lost it would show the operator a frame as stale
// as its next keystroke with nothing red to say so.
//
// The pin reads each loop's OWN body and not the whole file, because the
// tick is declared twice — once per loop — and a file-wide grep stays green
// over a loop that dropped its own (ranger-base-iyuc's escape, measured:
// that mutant survived the whole cmd/posse suite).
//
// MUTATION, both arms, each restored: lower either loop's ticker to any
// other period, or delete it, and this reds naming that loop.

import (
	"os"
	"strings"
	"testing"
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

func TestBothCockpitLoopsKeepTheTwoSecondRefresh(t *testing.T) {
	b, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	const tick = "time.NewTicker(2 * time.Second)"
	for _, header := range []string{"func runCockpit(", "func (c *cockpit) displayOnly()"} {
		if !strings.Contains(loopBody(t, src, header), tick) {
			t.Errorf("%s no longer declares %s — ADR 0016 keeps both cockpit modes on their current "+
				"cadence, and since the socket hints were removed this tick is the only path a herdr "+
				"change has onto the screen in that mode", header, tick)
		}
	}
}

// The other half of the same row: the bead-scan floor stays a floor in both
// modes. ADR 0016 keeps "the bead-scan cadence floor and protection of modal
// input", and refresh()'s force past that floor belongs to the closed list of
// callers that just WROTE to bd — c, u, x, o, r, a landed dispatch
// (ranger-base-u5rqp). Neither loop's cadence path is on that list, and the
// hint path that used to be tempted onto it is gone; nothing may take its
// place.
//
// MUTATION, each arm restored: point either loop's cadence path at refresh()
// (or at kickBeads(true)) and the arm that owns it reds.
func TestNeitherCockpitCadencePathForcesTheBeadScan(t *testing.T) {
	b, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	// The tty loop's 2s tick kicks the scan, and must kick it BEHIND the
	// floor.
	tty := loopBody(t, src, "func runCockpit(")
	if !strings.Contains(tty, "c.kickBeads(false)") {
		t.Error("runCockpit's tick no longer kicks the bead scan behind the floor (c.kickBeads(false)) — " +
			"either the scan stopped tracking the fleet under an open prompt, or it is forcing past the floor")
	}
	if strings.Contains(tty, "c.kickBeads(true)") {
		t.Error("runCockpit's own body forces a bead scan past the floor — forcing belongs to refresh(), " +
			"whose callers are exactly the ones that just wrote to bd (ranger-base-u5rqp)")
	}
	// The display-only loop scans synchronously and must consult the floor
	// itself, since it has no kickBeads path.
	frame := loopBody(t, src, "func (c *cockpit) displayFrame()")
	if !strings.Contains(frame, "time.Now().Before(c.beadsNext)") {
		t.Error("displayFrame no longer consults c.beadsNext — the non-tty loop is back to a bd scan per frame")
	}
	// And refreshSessions, which BOTH cadence paths call, touches the bead
	// lists not at all: it is the herdr half.
	if sessions := loopBody(t, src, "func (c *cockpit) refreshSessions()"); strings.Contains(sessions, "kickBeads") {
		t.Error("refreshSessions now kicks the bead scan — it is on both loops' cadence path, so that is a bd " +
			"read per tick in the tty loop and per frame in the other")
	}
}

// Modal input keeps its protection, the third thing ADR 0016 retains by name
// where it removed the socket. The tty loop's tick is now the ONLY thing that
// repaints on its own, so its mode gate is the whole of that protection: an
// ungated tick draws over a half-typed prompt every two seconds, and no
// golden-frame test sees it because the frame it draws is correct — it is
// only wrong about WHEN.
//
// MUTATION: drop the `if c.mode == modeNormal` around the tick's redraw and
// this reds.
func TestTheCockpitTickRedrawsOnlyInNormalMode(t *testing.T) {
	b, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	tty := loopBody(t, string(b), "func runCockpit(")
	i := strings.Index(tty, "case <-tick.C:")
	if i < 0 {
		t.Fatal("runCockpit has no tick case — re-aim this pin")
	}
	gated := tty[i:]
	if j := strings.Index(gated, "\n\t\t\tc.kickBeads(false)"); j > 0 {
		gated = gated[:j]
	}
	if !strings.Contains(gated, "if c.mode == modeNormal {") {
		t.Errorf("runCockpit's tick redraws without checking c.mode — since ADR 0016 removed the hint "+
			"channel this tick is the only self-starting repaint, so its mode gate IS the modal-input "+
			"protection:\n%s", gated)
	}
}

// The cockpit half of ADR 0016's fourth done-when row: no event subscription
// actor or state remains in this package either. internal/posse owns its own
// directory's census (herdrhintsgone_qa_test.go); this is cockpit.go's, and
// they are separate so neither needs a Makefile tree-door.
//
// MUTATION: name any of these in cockpit.go and this reds with the line.
func TestQANoHintStateRemainsInTheCockpit(t *testing.T) {
	b, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		"HerdrHint", "HerdrAllHints", "startHints", "applyHint", "spendHint",
		"consumeHintDirty", "refreshHint", "pokeHintsIfPanesMoved", "samePaneSet",
		"hintFloor", "hintDirty", "hintPending", "hintNext", "hintPanes",
		"hintRefresh", "hintReports", "AgentPanes",
	} {
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, bad) {
				t.Errorf("cockpit.go:%d names %q — ADR 0016 removed the hint channel and its floor, "+
					"dirty/pending bits and pane-set poke; the 2s tick is the refresh path:\n\t%s",
					i+1, bad, strings.TrimSpace(line))
			}
		}
	}
}
