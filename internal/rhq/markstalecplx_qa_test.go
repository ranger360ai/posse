package rhq

// ranger-base-cplx, the arm ranger-base-twaq's condition does not reach.
//
// twaq carries the availability mark across `posse relaunch` and drops it
// again "once the PID asks for what is running" — an EQUALITY test on the
// pair. Any OTHER edit to the PID leaves the pair differing, so the mark
// rides through with its original sentence, naming a tier and a model the
// PID no longer asks for. TestQA7vpTheCarriedMarkIsDroppedOnceThePIDAsksFor
// WhatIsRunning is the arm that works; this is the third-tier one.

import (
	"strings"
	"testing"
)

func TestQAACarriedMarkNamesThePIDAsItIsNow(t *testing.T) {
	t.Skip("ranger-base-cplx: the carried mark keeps the tier it fell FROM, so a PID edited to a third tier is described by a sentence that is false of it")
	b, _ := qaFellSession(t, "cu") // architect: tier strong, fell to standard
	qaPID(t, b, "architect", TierFast)

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "cu", NoLand: true}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	m, _ := b.readMeta("cu")
	if m.Tier != TierStandard {
		t.Fatalf("board not set up, the refresh must keep running the substitute: %+v", m)
	}
	// The mark is still EARNED — the session is not running the pair its PID
	// names — so the tag and effectiveTier stay right. What must not survive
	// is a sentence about a tier this PID no longer asks for.
	if strings.Contains(m.Fallback, TierStrong) {
		t.Errorf("the mark still says the PID asks for %s; it asks for %s: %q", TierStrong, TierFast, m.Fallback)
	}
	if strings.Contains(out.String(), TierStrong) {
		t.Errorf("the receipt repeats it:\n%s", out.String())
	}
}
