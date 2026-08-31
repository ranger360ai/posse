package posse

// LIVE DEFECT PIN (ranger-base-hs1g → ranger-base-rcwx, the escape filed
// against rangerhq-ouf9). `instance:` frees a name from another instance only when
// that instance is ALSO tagged. Against an UNTAGGED instance's bare label —
// which is what the operator's own long-running home writes, and what
// INSTALL.md §10 tells a cold installer to create by hand — the tag frees
// nothing, and the collision rangerhq-ouf9 exists to remove is still live
// in the one ordering the fleet actually has.
//
// These assert TODAY's behaviour on purpose, green, with the inversion in
// each failure message: a pin that went red would be deleted and a skipped
// one forgotten, so the only shape that survives a silent revert is one
// that fails the day the fix lands. When these go red, read the message —
// it says what to replace them with.
//
// Root cause, one line: a bare label is read as this home's own rendering
// of a name even when a tag is set (labelWearsName's bare arm, and
// Sessions() naming a foreign row by its label), so a tagged home cannot
// tell "my own workspace from before the key was set" from "another
// instance's untagged workspace".

import (
	"strings"
	"testing"
)

// Arm 1 — the create. The bead's own observable is two instances, one with
// `instance: work`, both creating a session under one shared name, and both
// succeeding. It holds only when the tagged home creates FIRST. Reversed —
// the untagged home already
// holds the bare label — the tagged home's create dies on the very sentence
// the bead names as the failure to remove.
func TestQAInstanceTagDoesNotFreeABareForeignNamesake(t *testing.T) {
	b, fake := newTestBackend(t)
	hermeticGen(t)
	setInstance(t, b, "work")

	// The other instance is UNTAGGED, so its workspace wears the bare name.
	// INSTALL.md §10 tells a cold installer to type `posse new smoke`.
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
		fakeWS{WorkspaceID: "w9", Label: "smoke"}))

	err := b.CreateSession(NewSessionOpts{Name: "smoke", Dir: t.TempDir()})
	if err == nil {
		t.Fatal("FIXED: a tagged home created a session over an untagged instance's bare namesake.\n" +
			"  rangerhq-ouf9's observable now holds in both orderings — replace this pin with the\n" +
			"  positive assertion: the create succeeds and goes out under `work/smoke`.")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("the refusal changed shape; re-read this pin before trusting it: %v", err)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a refused create still reached herdr:\n%s", log)
	}

	// The control that makes the finding an ASYMMETRY rather than a taste:
	// the same collision, the other way round, goes through. Only the home
	// that set the key is blocked by the home that did not.
	b2, fake2 := newTestBackend(t)
	hermeticGen(t)
	saveWSTo(t, fake2, append(fakeLoadWSFrom(t, fake2),
		fakeWS{WorkspaceID: "w9", Label: "work/smoke"}))
	if err := b2.CreateSession(NewSessionOpts{Name: "smoke", Dir: t.TempDir()}); err != nil {
		t.Errorf("the untagged home must still be free of a tagged instance's row (that half works): %v", err)
	}
}

// Arm 2 — the relaunch, and the worse half. A tagged home cannot relaunch
// its OWN session while another instance holds the bare name, over a
// workspace that could not obstruct a create this home labels `fleet/s1`.
// The refusal then tells the operator to "rename or close" that workspace
// in herdr — which is precisely the act rangerhq-selx (closed the same
// evening) built a wall against on every posse path.
func TestQAInstanceTagRelaunchBlockedByABareForeignNamesake(t *testing.T) {
	b, fake := newTestBackend(t)
	hermeticGen(t)
	setInstance(t, b, "fleet")
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")

	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
		fakeWS{WorkspaceID: "w9", Label: "s1"}))

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatal("FIXED: a tagged home relaunched its own session over an untagged instance's\n" +
			"  bare namesake. nameWornElsewhere now asks for THIS home's rendering of the name —\n" +
			"  replace this pin with the positive assertion (the relaunch succeeds), and keep the\n" +
			"  control below: a row wearing `fleet/s1` really is in the way and must still refuse.")
	}
	if !strings.Contains(err.Error(), "w9") {
		t.Fatalf("the refusal changed shape; re-read this pin before trusting it: %v", err)
	}
	// The part that makes this more than a false refusal: the advice.
	if !strings.Contains(err.Error(), "rename or close") {
		t.Errorf("expected the refusal to send the operator at the other instance's workspace: %v", err)
	}
}
