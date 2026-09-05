package posse

// QA pins for ranger-base-wr624: the pulse's unsent-box gate false-fired on
// a box that was empty, and a matcher that cannot go false took the pulse
// arm off for ~10 hours of 2026-09-04 (~586 consecutive skips over three
// episodes, each naming a line the operator had already SENT and the
// persona had already answered, while 108 commits stacked behind it).
//
// The gate is gone from the delivery path; the READING is not, and the
// distinction is what these pin. The screen fixtures are settlewaiting's
// (armScreen, idleFooter, emptyBox) — the shapes measured on the live shop.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ghostBox is the failure verbatim in shape: the operator's last sent line,
// answered hours ago, still previewing in herdr's composer region.
const ghostBox = "❯ how did the day go?\n"

// pulseAgainstScreen runs one pulse tick over an idle coordinator whose
// pane previews the given footer and composer, and hands back the pass
// output and the fake's call log.
func pulseAgainstScreen(t *testing.T, footer, box string) (out, log string) {
	t.Helper()
	b, fake := newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pulseFastRuntime(t, b, "coordinator-work", "400ms")
	unpushedRepo(t, b)
	armScreen(t, fake, footer, box)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	d.pulseOnce(PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute, RenagMax: 4 * time.Hour})
	return dispatcherOut(d), calls(t, fake)
}

// THE BEAD. Text in the box no longer stops the shop check. The line that
// reports it is asserted too, and it is not decoration: it is what proves
// this pass READ a non-empty composer rather than never reaching the
// reading at all — without it, "the pulse was delivered" is also what a
// test whose fixture never armed would print.
func TestQAPulseIsNotGatedByTextInTheCoordinatorsBox(t *testing.T) {
	t.Parallel()
	out, log := pulseAgainstScreen(t, idleFooter, ghostBox)

	if !strings.Contains(out, "→ prompted coordinator-work") {
		t.Fatalf("the pulse skipped on a box reading (ranger-base-wr624 is the ~586 skips):\n%s", out)
	}
	if !strings.Contains(log, "agent prompt") {
		t.Errorf("no prompt reached herdr:\n%s", log)
	}
	if !strings.Contains(out, "how did the day go?") || !strings.Contains(out, "ghost text") {
		t.Errorf("the delivery over a non-empty box left no trace to explain a garbled turn by:\n%s", out)
	}
}

// The reading is still a reading. An empty box says nothing at all, so the
// line above is evidence of a real composer preview and not a banner every
// tick prints.
func TestQAPulseSaysNothingAboutAnEmptyBox(t *testing.T) {
	t.Parallel()
	out, _ := pulseAgainstScreen(t, idleFooter, emptyBox)

	if !strings.Contains(out, "→ prompted coordinator-work") {
		t.Fatalf("the control arm did not deliver:\n%s", out)
	}
	if strings.Contains(out, "ghost text") || strings.Contains(out, "box previews") {
		t.Errorf("an empty box was reported as holding something:\n%s", out)
	}
}

// THE WRONG ARM, and the reason this is a narrowing rather than a way to
// switch the pulse's care off: the gate that was measured — herdr can SEE
// this screen is working — still refuses, with the very same text in the
// box. Only the composer stopped deciding.
func TestQAPulseStillRefusesAWorkingScreenWithTextInTheBox(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pulseFastRuntime(t, b, "coordinator-work", "400ms")
	unpushedRepo(t, b)
	armScreen(t, fake, idleFooter, ghostBox)
	if err := os.WriteFile(filepath.Join(fake, "explain-state"), []byte("working"), 0o644); err != nil {
		t.Fatal(err)
	}

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	d.pulseOnce(PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute, RenagMax: 4 * time.Hour})

	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("a shop check landed in the middle of a turn:\n%s", log)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "pulse: skipped") {
		t.Errorf("the working screen must still skip:\n%s", out)
	}
}

// The other half of the narrowing, at the unit: dispatch's --resume skip
// and govern's G2 row read the same composer through PaneHold, and this
// bead did not touch them. A pane holding this exact ghost text is still
// Waiting() — those two callers are about a DISPATCHED holder's pane and
// fail towards not acting, and changing them was not measured here
// (ranger-base-2hvtv carries the discriminator that would).
func TestQATextInABoxIsStillAHoldForTheDispatchReaders(t *testing.T) {
	t.Parallel()
	h := detectionWith("idle", idleFooter, ghostBox).Hold()
	if !h.Waiting() || h.Typed == "" {
		t.Fatalf("PaneHold stopped seeing composer text; dispatch and govern read it: %+v", h)
	}
	if !strings.Contains(h.Why(), "UNSENT") {
		t.Errorf("the hold's clause no longer names the unsent prompt: %q", h.Why())
	}
}
