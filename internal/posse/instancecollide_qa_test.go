//go:build posse_arm2

package posse

// ranger-base-rcwx (the escape filed against rangerhq-ouf9's close, found by
// ranger-base-hs1g). `instance:` used to free a name from another instance
// only when that instance was ALSO tagged. Against an UNTAGGED instance's
// bare label — which is what the operator's own long-running home writes,
// and what INSTALL.md §10 tells a cold installer to create by hand — the tag
// freed nothing, so the collision rangerhq-ouf9 exists to remove was still
// live in the one ordering the fleet actually has: the SECOND, tagged home
// is the one that dies on "already exists", and it could not even relaunch
// its own session.
//
// These were green defect pins asserting that behaviour with the inversion
// in each failure message; they are now the regression pins the fix owes,
// same two arms, each with the control that keeps the fix from being "delete
// the guard": a workspace wearing the label THIS home writes must still
// refuse, on both the create and the relaunch.
//
// Root cause, one line: a bare label was read as this home's own rendering
// of a name even when a tag was set (labelWearsName's bare arm, reached from
// nameWornElsewhere; and Sessions() naming a foreign row by its label, read
// through Resolve by the create's guard). Both questions now ask
// WorkspaceLabel — the string startPlanned would actually write.

import (
	"strings"
	"testing"
)

// Arm 1 — the create. The bead's observable is two instances, one with
// `instance: work`, both creating a session under one shared name, and both
// succeeding. It used to hold only when the tagged home created FIRST.
func TestInstanceTagFreesABareForeignNamesake(t *testing.T) {
	b, fake := newTestBackend(t)
	hermeticGen(t)
	setInstance(t, b, "work")

	// The other instance is UNTAGGED, so its workspace wears the bare name.
	// INSTALL.md §10 tells a cold installer to type `posse new smoke`.
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
		fakeWS{WorkspaceID: "w9", Label: "smoke"}))

	if err := b.CreateSession(NewSessionOpts{Name: "smoke", Dir: t.TempDir()}); err != nil {
		t.Fatalf("a tagged home refused over an untagged instance's bare namesake: %v", err)
	}
	if log := calls(t, fake); !strings.Contains(log, "workspace create --label work/smoke") {
		t.Errorf("the create did not go out under this home's tag:\n%s", log)
	}
	// And it is addressable: this home's own row wins the name over the
	// bare foreign one, which is why the refusal was never necessary.
	s, err := b.Resolve("smoke")
	if err != nil || s.Foreign || s.WorkspaceID == "w9" {
		t.Errorf("Resolve must prefer this home's own session: %+v, %v", s, err)
	}

	// The other half of the symmetry, which already worked: the untagged
	// home is not blocked by a tagged instance's row either.
	b2, fake2 := newTestBackend(t)
	hermeticGen(t)
	saveWSTo(t, fake2, append(fakeLoadWSFrom(t, fake2),
		fakeWS{WorkspaceID: "w9", Label: "work/smoke"}))
	if err := b2.CreateSession(NewSessionOpts{Name: "smoke", Dir: t.TempDir()}); err != nil {
		t.Errorf("the untagged home must be free of a tagged instance's row: %v", err)
	}
}

// The control the fix must not cost: a workspace wearing the label this home
// would WRITE is really in the way, and still refuses. Both spellings of it
// — the untagged home's bare namesake, which is the single-instance
// behaviour every home had before the key existed, and a tagged home meeting
// its own tag (two homes sharing a tag, or a row of ours whose meta is gone).
//
// The tagged arm is one the old guard let THROUGH: it asked Resolve, which
// names a foreign row by its whole label, so `work/smoke` never matched the
// name `smoke` and herdr took two workspaces under one label.
func TestACollidingLabelStillRefusesTheCreate(t *testing.T) {
	for _, c := range []struct{ name, tag, label string }{
		{"untagged home, bare namesake", "", "smoke"},
		{"tagged home, a row under its own tag", "work", "work/smoke"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			hermeticGen(t)
			if c.tag != "" {
				setInstance(t, b, c.tag)
			}
			saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
				fakeWS{WorkspaceID: "w9", Label: c.label}))

			err := b.CreateSession(NewSessionOpts{Name: "smoke", Dir: t.TempDir()})
			if err == nil {
				t.Fatalf("a workspace labelled %q is in the way of this home's create and must refuse it", c.label)
			}
			if !strings.Contains(err.Error(), "already exists") {
				t.Errorf("the refusal changed shape: %v", err)
			}
			// The refusal points at the row by its DISPLAYED name, which is
			// what `posse list` prints and what attach resolves — under a
			// tag that is the label, not the session name.
			if !strings.Contains(err.Error(), "posse attach "+c.label) {
				t.Errorf("the refusal must send the operator at the row that is in the way: %v", err)
			}
			if log := calls(t, fake); strings.Contains(log, "workspace create") {
				t.Errorf("a refused create still reached herdr:\n%s", log)
			}
		})
	}
}

// Arm 2 — the relaunch, and the worse half. A tagged home could not relaunch
// its OWN session while another instance held the bare name, over a
// workspace that cannot obstruct a create this home labels `fleet/s1`. The
// refusal then told the operator to "rename or close" that workspace in
// herdr — precisely the act rangerhq-selx (closed the same evening) built a
// wall against on every posse path.
func TestRelaunchIsNotBlockedByABareForeignNamesake(t *testing.T) {
	b, fake := newTestBackend(t)
	hermeticGen(t)
	setInstance(t, b, "fleet")
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")

	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
		fakeWS{WorkspaceID: "w9", Label: "s1"}))

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("a tagged home could not relaunch its own session over a bare foreign namesake: %v\n%s", err, out.String())
	}
	if !b.HasSession("s1") {
		t.Error("the session is gone after a relaunch that reported success")
	}

	// The control, kept from the pin this replaces: a row wearing THIS
	// home's label really would take the name back after the kill, so it
	// still refuses — before the kill, naming the workspace in the way.
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
		fakeWS{WorkspaceID: "w8", Label: "fleet/s1"}))
	out.Reset()
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatal("a workspace wearing this home's own label must still refuse the relaunch")
	}
	if !strings.Contains(err.Error(), "w8") || !strings.Contains(err.Error(), "was NOT closed") {
		t.Errorf("the refusal must name the workspace in the way: %v", err)
	}
}
