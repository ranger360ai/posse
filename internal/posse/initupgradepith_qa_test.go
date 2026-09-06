//go:build !posse_arm2 && !posse_arm3

package posse

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
	"os"
	"path/filepath"
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
	t.Parallel()
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

// TestQAUpgradeInitOnAPromotedHomeRefuses is the same trap on the OTHER kind
// of armed home. A `seeded` manifest gets re-stamped for exactly what this
// run wrote (the case above); a genuinely PROMOTED one (Seeded == false)
// never may be — that manifest is a claim about a commit, and only `posse
// promote` may restate it (init.go's comment on `repaired`). So an upgrade
// init that adds recipes/ or skills/ files to a promoted home has no
// re-stamp available to it at all: it used to say nothing (exit 0, the
// manifest untouched, every dispatched launch refusing from then on), then
// under ranger-base-pith it said `posse promote` on the way out — and still
// left the fleet refusing until somebody read the line.
//
// ranger-base-39jnl closes it at the source: a home carrying a PROMOTED
// manifest is not init's to write at all, so the copy that breaks the launch
// verify never happens. This pins the refusal AND what it protects — the
// home still verifies, and a dispatched launch still goes — because a
// refusal that merely trades one broken home for another is not the fix.
func TestQAUpgradeInitOnAPromotedHomeRefuses(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatal(err)
	}
	qaPID(t, b, "architect", TierStandard)
	// Stand in for `posse promote`: a manifest that is a claim about a
	// commit, Seeded left false the way CmdPromote writes one.
	m := &PromoteManifest{Version: promoteManifestVersion, Source: "/somewhere/constitution", SHA: strings.Repeat("a", 40)}
	files, err := HashPromotedSet(a.Home)
	if err != nil {
		t.Fatal(err)
	}
	m.Files = files
	if err := m.write(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Fatalf("fixture does not verify: %s", v.Line())
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s0", Dir: t.TempDir(), Agent: "architect", Bead: "x-1"}); err != nil {
		t.Fatalf("board not set up: %v", err)
	}

	// THE UPGRADE, on the promoted home this time.
	out.Reset()
	err = a.initFrom(&out, newerSeed(t), "newer")
	if err == nil {
		t.Fatalf("init wrote a promoted home — it must refuse (ranger-base-39jnl):\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "posse promote") {
		t.Errorf("the refusal does not name where the write belongs:\n%v", err)
	}
	// What the refusal is FOR: the home is untouched, so it still matches
	// its manifest and still dispatches. Before 39jnl this same call landed
	// newthing.yaml and newskill/ inside the promoted set, and every
	// dispatched launch was refused from here on.
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("the refused init still moved the home off its manifest: %s", v.Line())
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "architect", Bead: "x-1"}); err != nil {
		t.Errorf("a refused init still cost the fleet its dispatch: %v", err)
	}
}

// TestQAUpgradeInitOnUnreadableManifestNamesTheProblem — a second, smaller
// instance of the same silence (ranger-base-pith comment, 2026-08-29).
// Before 5fbb28c a promoted.json init could not read killed
// `posse init` outright; now the read error is swallowed into `manErr` and
// init exits 0 having left a home the unattended fleet cannot dispatch to,
// with nothing printed connecting the two.
func TestQAUpgradeInitOnUnreadableManifestNamesTheProblem(t *testing.T) {
	wtqaHome(t)
	// Hermetic against the operator fence (ADR 0031 §2): see newTestBackend.
	a := NewAppAt(filepath.Join(t.TempDir(), "home"))
	if err := os.MkdirAll(a.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.PromoteManifestPath(), []byte("not json{"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatal(err)
	}
	v := a.VerifyPromoted()
	if v.Err == nil {
		t.Fatal("fixture: expected the manifest to still be unreadable after init")
	}
	if !strings.Contains(out.String(), "unreadable") || !strings.Contains(out.String(), "posse promote") {
		t.Errorf("init left a home the fleet cannot dispatch to and said nothing about it:\n  verdict: %s\n  init said:\n%s", v.Line(), out.String())
	}
}
