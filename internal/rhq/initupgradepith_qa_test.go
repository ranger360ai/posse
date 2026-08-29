package rhq

// ranger-base-pith: the OTHER side of ranger-base-h7cd. h7cd stopped `posse
// init` ARMING ADR 0015 §3 on a home that never promoted. This is init
// TRIPPING §3 on a home that did.
//
// initFrom copies with copyIfMissing, so it never overwrites — but it does
// ADD, and recipes/ and skills/ are both in PromotedPaths. A release that
// ships a new generic lands files in the promoted set of a home that already
// carries promoted.json, and nothing re-stamps it. VerifyPromoted then
// reports them as `unpromoted` and every DISPATCHED launch is refused, while
// an interactive one only warns — so the failure lands on the unattended
// fleet, triggered by the upgrade INSTALL.md §7 advertises, with nothing
// printed connecting the two. That is h7cd's own P1 argument, reached from
// the other end.
//
// The one path that re-stamps is retireExamplePIDs, and only when it actually
// retired something (that re-stamp is ranger-base-9afo's subject). An
// instance past its first upgrade has nothing left to retire.

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ranger360ai/posse"
)

// newerSeed is posse.Seed plus one recipe and one skill — what a release that
// ships a new generic looks like to a home that already exists.
func newerSeed(t *testing.T) fs.FS {
	t.Helper()
	m := fstest.MapFS{}
	if err := fs.WalkDir(posse.Seed, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := fs.ReadFile(posse.Seed, p)
		if rerr != nil {
			return rerr
		}
		m[p] = &fstest.MapFile{Data: b, Mode: 0o644}
		return nil
	}); err != nil {
		t.Fatalf("building the newer seed: %v", err)
	}
	m["recipes/newthing.yaml"] = &fstest.MapFile{Data: []byte("purpose: new\n"), Mode: 0o644}
	m["skills/newskill/SKILL.md"] = &fstest.MapFile{Data: []byte("---\nname: newskill\ndescription: a skill a later release ships\n---\n"), Mode: 0o644}
	return m
}

func TestQAUpgradeInitDoesNotBreakTheLaunchVerifyItCannotSee(t *testing.T) {
	t.Skip("ranger-base-pith: an upgrade init adds seed files to the promoted set of an armed home and never re-stamps, so every dispatched launch is refused from then on")
	wtqaHome(t)
	b, _ := newTestBackend(t)
	a := b.App

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatal(err)
	}
	// An ARMED home: what a fresh install is, and what every home an older
	// posse initialised is. newTestBackend's home is already populated, so
	// today's init leaves it unstamped — arm it the way those two are.
	if m, _ := ReadPromoteManifest(a.PromoteManifestPath()); m == nil {
		if err := a.SeedPromoteManifest(); err != nil {
			t.Fatal(err)
		}
	}
	qaPID(t, b, "architect", TierStandard)
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil {
		t.Fatalf("board not set up, the home must be armed: %+v %v", m, err)
	}
	if m.Files, err = HashPromotedSet(a.Home); err != nil { // what `posse promote` does after hiring
		t.Fatal(err)
	}
	if err := m.write(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s0", Dir: t.TempDir(), Agent: "architect", Bead: "x-1"}); err != nil {
		t.Fatalf("board not set up, a dispatch must launch on a home that matches its manifest: %v", err)
	}

	// THE UPGRADE, exactly as INSTALL.md §7 advertises it: re-run init with a
	// newer binary.
	out.Reset()
	if err := a.initFrom(&out, newerSeed(t), "newer"); err != nil {
		t.Fatal(err)
	}
	// Errorf, not Fatalf: the silence and the outage are two claims, and the
	// second is the one the fleet feels.
	if v := a.VerifyPromoted(); !v.OK() && !strings.Contains(out.String(), "posse promote") {
		t.Errorf("init broke the launch verify and said nothing about it:\n  verdict: %s\n  init said:\n%s", v.Line(), out.String())
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "architect", Bead: "x-1"}); err != nil {
		t.Errorf("the advertised upgrade refuses every dispatched launch: %v", err)
	}
}
