//go:build !posse_arm2 && !posse_arm3

package posse

// Live pin for ranger-base-r5131, run against the real rex server and a real
// claude seat rather than the fake — because the fake is a file this repo
// writes, and the thing being claimed is that CLAUDE puts a sentence on a
// screen that REX will hand back.
//
//	RHQ_LIVE_REX=1 go test ./internal/posse/ -run TestLiveCapturedHoldReadsThisSeatsOwnFooter -v
//
// It reads the seat it is RUNNING IN: `REX_SESSION` and `REX_BLOCK` are in
// every Rex seat's environment (MEASURED 2026-11-10 in this one), so the pin
// needs no fixture, no launch and no cleanup — the live claude whose footer
// it reads is the one running the test.
//
// WHAT IT PROVES THAT THE HERMETIC ARMS CANNOT. capturedhold_qa_test.go pins
// the route and the anchor over a capture this repo wrote. Three things
// outside this repo have to hold for that to mean anything, and all three
// are somebody else's to change: claude has to put its background-task
// summary in the footer, rex's `capture --format text` has to return a
// screen with the rules in it, and the footer has to survive the end of the
// turn. The first and third were MEASURED 2026-11-10 by sampling this seat
// across a turn boundary (panework.go's capturedHold note); this pin is how
// the first two get re-checked on a later claude or a later rex, by hand,
// in one command.
//
// It asserts the SHAPE and not the work, because a seat with nothing running
// has an honest empty footer and that is not a failure. Set
// RHQ_LIVE_REX_WORK=1 when something IS running in the seat — a background
// shell, a monitor — and the pin then also requires the hold to see it,
// which is the arm that would have caught this bead.
import (
	"os"
	"strings"
	"testing"
)

func TestLiveCapturedHoldReadsThisSeatsOwnFooter(t *testing.T) {
	// Read-only throughout: three rex calls that capture a screen and read a
	// status, no seat created, nothing typed, no env mutated.
	t.Parallel()

	if os.Getenv("RHQ_LIVE_REX") == "" {
		t.Skip("RHQ_LIVE_REX unset: this pin shells out to the operator's rex and reads the seat it runs in")
	}
	block, bin := os.Getenv("REX_BLOCK"), os.Getenv(RexBinEnv)
	if block == "" {
		t.Skip("REX_BLOCK unset: not running inside a Rex seat, so there is no live footer to read")
	}
	// BOTH HALVES of the seat, which is what the verbs under test are
	// addressed with since ranger-base-k0hd6 — a Rex block lives under
	// `session.*`, so an occupant handed a pane-only Seat resolves the
	// session half out of the server's own listing first (RexMux.hold,
	// ranger-base-nfc2b). The environment carries both, so this pin pays
	// neither that listing nor the chance of resolving to the wrong seat.
	// REX_SESSION empty is not a skip: the occupant still answers for half
	// a Seat it is handed, and that licence is not withdrawn.
	seat := Seat{Session: os.Getenv("REX_SESSION"), Pane: block}
	if bin == "" {
		bin = RexDefaultBin()
	}

	b := &HerdrBackend{Slot: RexMux{R: Rex{Bin: bin}}}

	// The §4 door must still refuse, or the fallback under test is not the
	// code path this seat takes.
	if _, err := b.explain(seat); err == nil {
		t.Fatal("`agent explain` answered on a Rex slot — this pin is about the route taken when it cannot")
	} else if verb, missing := muxVerbMissing(err); !missing {
		t.Fatalf("explain failed with something other than §4's refusal (%v) — the capture route is not reached and PaneHolding answers the empty hold", err)
	} else if verb != "agent explain" {
		t.Fatalf("§4 refusal named %q, want \"agent explain\"", verb)
	}

	screen, err := b.Mux().Read(seat, captureFooterLines, false)
	if err != nil {
		t.Fatalf("rex could not capture this seat's own block %s: %v", block, err)
	}
	footer := captureFooter(screen)
	t.Logf("captured %d bytes of screen tail; footer = %q", len(screen), footer)
	if footer == "" {
		t.Fatalf("no footer after the last horizontal rule in a live claude screen — either claude stopped drawing the rules or rex stopped returning them, and either way the WORK half of htafy's hold is off on this instance again. Screen tail:\n%s", screen)
	}

	hold := b.PaneHolding(seat)
	// Whatever the footer says, these two must agree: the hold is read off
	// the footer and nothing else on this route.
	if (hold.Work != "") != (backgroundWorkIn(footer) != "") {
		t.Errorf("PaneHolding().Work = %q but the footer's own vocabulary reading is %q — the hold is no longer the footer's", hold.Work, backgroundWorkIn(footer))
	}
	if hold.Typed != "" || hold.Sent != "" || hold.Ghost != "" {
		t.Errorf("the capture route claimed a composer reading (Typed=%q Sent=%q Ghost=%q); it has no store to tell a held prompt from an echoed one (ranger-base-qzujz)", hold.Typed, hold.Sent, hold.Ghost)
	}

	if os.Getenv("RHQ_LIVE_REX_WORK") == "" {
		t.Logf("RHQ_LIVE_REX_WORK unset: not requiring live work. Work = %q", hold.Work)
		return
	}
	// The arm that reproduces the bead. Something is running in this seat,
	// the turn it was started from may well have ended, and the hold has to
	// see it — or dispatch counts a settle-open and re-prompts a seat that is
	// doing exactly what it was told.
	if hold.Work == "" {
		t.Fatalf("RHQ_LIVE_REX_WORK is set and the hold sees no live work. footer = %q", footer)
	}
	if !hold.Waiting() {
		t.Fatalf("Waiting() = false with Work = %q", hold.Work)
	}
	if !strings.Contains(hold.Why(), "still running") {
		t.Errorf("Why() = %q, want it to say what is still running", hold.Why())
	}
	t.Logf("live hold: Work = %q, Why = %q", hold.Work, hold.Why())
}
