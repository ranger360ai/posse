//go:build posse_arm3

package posse

// Live pin for ranger-base-xdaw9, the herdr twin of
// TestLiveRexPromptSeesThisSeatsOwnComposer (capturedhold_live_test.go):
//
//	RHQ_LIVE_HERDR=1 go test -tags posse_arm3 ./internal/posse -run TestLiveHerdrPaneSeesThisPanesOwnComposer -v
//
// WHAT IT GUARDS is the hazard ADR 0076 Amendment 8 names out loud, which is
// Amendment 7's: `refuseWithoutComposerPane` refuses a typed prompt into a
// pane whose screen carries claude's chrome and no box, so if a later claude
// stops drawing the two rules around its box, every typed prompt into such a
// pane is refused. The hermetic arms cannot see that — their screens are
// quoted bytes from 2026-11-11 (promptnocomposerpane_qa_test.go) — and this
// one can, because on a herdr instance the suite itself runs in a claude pane
// with its composer drawn.
//
// AND IT IS NOT MADE REDUNDANT BY THE REX PIN, which reads the same function.
// What differs is the SOURCE: this route judges `pane read --source
// detection` and that one judges `rex capture --format vt --trim`, and the
// source is exactly what Amendment 8 D2 had to measure. A claude that changed
// its drawing would red either pin, on whichever occupant's box the suite ran;
// a herdr whose `pane read` output shape moved would red only this one, and
// neither box can run the other's.
//
// READ-ONLY, and nothing is typed: this is `captureComposerSight` over one
// read of this pane's own screen, not a `Prompt`. A pin that prompted would be
// typing into the pane running it.

import (
	"os"
	"strings"
	"testing"
)

func TestLiveHerdrPaneSeesThisPanesOwnComposer(t *testing.T) {
	t.Parallel()

	if os.Getenv("RHQ_LIVE_HERDR") == "" {
		t.Skip("set RHQ_LIVE_HERDR=1 (reads the pane this suite is running in off the operator's herdr)")
	}
	pane := os.Getenv("HERDR_PANE_ID")
	if pane == "" {
		t.Skip("HERDR_PANE_ID unset: not running inside a herdr pane, so there is no live screen to read")
	}
	h := NewHerdr()
	if !h.Available() {
		t.Skip("no herdr on this host")
	}

	screen, err := h.PaneReadDetection(pane)
	if err != nil {
		t.Fatalf("herdr could not read this pane's own screen %s: %v", pane, err)
	}
	sight := captureComposerSight(screen)
	t.Logf("live sight = %d (open=%d), last line = %q", sight, composerOpen, captureLastLine(screen))
	if det, derr := h.AgentExplain(pane); derr == nil {
		t.Logf("herdr detects %q, state %q", det.Agent, det.State)
		if det.Agent != claudeDetectedLabel {
			t.Skipf("herdr detects %q in this pane, and this reading may only decide about %s",
				det.Agent, claudeDetectedLabel)
		}
	}
	if sight != composerOpen {
		t.Fatalf("this pane's own live screen does not read as a composer posse may type into (sight %d).\n"+
			"If this pane really is holding a dialog, clear it and run this again. If it is not, claude has changed "+
			"what it draws and refuseWithoutComposerPane now refuses every typed prompt into a reported pane on this "+
			"instance — ADR 0076 Amendment 8's stated hazard. The screen was:\n%s",
			sight, strings.TrimRight(screen, "\n"))
	}
}
