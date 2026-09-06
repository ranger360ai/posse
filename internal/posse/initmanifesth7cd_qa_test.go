//go:build posse_arm2

package posse

// ranger-base-h7cd: `posse init` must not arm ADR 0015 §3 on a home that
// never promoted anything.
//
// The bug had no symptom at init time, which is what made it P1. Init stamped
// promoted.json over whatever constitution it found — on the live box that
// was 11 personas, config.yaml, 10 recipes and the skills tree — and printed
// one line about seeding. Nothing failed. The operator's NEXT edit to any of
// those files was the trigger, and what it broke was the unattended half:
// dispatch refuses on a mismatch, an interactive launch only warns. So the
// fleet stopped launching at a time unrelated to the init that caused it,
// with nothing connecting the two.
//
// Both arms are here because either one alone is green for the wrong reason:
// the populated home must come up launching after an edit, and the fresh
// install must still refuse — otherwise "no refusal" would only mean the
// launch verify had been broken outright.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// preADR0015Home is a home as an upgrade finds it: a constitution the
// operator wrote, and no manifest — the state VerifyPromoted reads as
// "nothing was promoted here", and the state every install predating ADR
// 0015 is in.
func preADR0015Home(t *testing.T, a *App) {
	t.Helper()
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_dir: ~\ncoordinator: ranger\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\n---\nthe operator's own prose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.RecipesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RecipesDir, "mine.yaml"), []byte("purpose: mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// editConfig is the routine operator act the bug turned into a fleet outage:
// one line appended to config.yaml, long after the init.
func editConfig(t *testing.T, a *App) {
	t.Helper()
	body, err := os.ReadFile(a.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, append(body, []byte("budget_day: 250\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestQAInitDoesNotArmTheLaunchVerifyOnAHomeThatNeverPromoted(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	preADR0015Home(t, a)

	var out bytes.Buffer
	if err := a.CmdInit(&out); err != nil {
		t.Fatalf("init on an existing home: %v\n%s", err, out.String())
	}
	// Errorf, not Fatalf: the stamp and the outage it causes are two separate
	// claims, and the second is the one the fleet felt. A Fatal here would
	// hide it, and a pin that only ever reports the cause cannot say the
	// effect is still gone.
	if m, err := ReadPromoteManifest(a.PromoteManifestPath()); err != nil || m != nil {
		n, seeded := 0, false
		if m != nil {
			n, seeded = len(m.Files), m.Seeded
		}
		t.Errorf("init stamped a manifest over a constitution nobody promoted: seeded=%v, %d files, err=%v", seeded, n, err)
	}
	// Said out loud, with the one command that would arm it — the operator
	// otherwise cannot tell an armed home from an unarmed one.
	if s := out.String(); !strings.Contains(s, "posse promote") || !strings.Contains(s, "ADR 0015") {
		t.Errorf("init did not say the verify is off and what arms it:\n%s", s)
	}

	// The trigger, and the launch that used to die on it.
	editConfig(t, a)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger", Bead: "x-1"}); err != nil {
		t.Fatalf("a dispatched launch was refused after an ordinary config edit on an unpromoted home: %v", err)
	}
}

// The control, and the half of init that must NOT change: a genuinely fresh
// install is its own anchor from the first launch, so §3 is live there and a
// mismatch still refuses a dispatch.
func TestQAInitStillStampsAFreshInstallAndSaysWhatThatArms(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App

	var out bytes.Buffer
	if err := a.CmdInit(&out); err != nil {
		t.Fatalf("init on a fresh home: %v\n%s", err, out.String())
	}
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil || !m.Seeded {
		t.Fatalf("a fresh install got no seeded manifest: %+v %v", m, err)
	}
	if s := out.String(); !strings.Contains(s, "promoted.json") || !strings.Contains(s, "ADR 0015") {
		t.Errorf("init stamped a manifest and did not name it:\n%s", s)
	}

	// A crew of one, hired the way INSTALL.md says (copy off the shelf) —
	// which is itself an edit to the promoted set, so re-stamp the anchor
	// the way an operator would run `posse promote`, then drift it.
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Files, err = HashPromotedSet(a.Home)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.write(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger", Bead: "x-1"}); err != nil {
		t.Fatalf("a matching constitution refused a dispatch: %v", err)
	}
	editConfig(t, a)
	_, err = b.planLaunch(NewSessionOpts{Name: "s2", Dir: t.TempDir(), Agent: "ranger", Bead: "x-1"})
	if err == nil {
		t.Fatal("dispatch launched on a seeded home whose config.yaml no longer matches its manifest")
	}
	if !strings.Contains(err.Error(), "config.yaml") || !strings.Contains(err.Error(), "posse promote") {
		t.Errorf("the refusal does not name the drift and the fix: %v", err)
	}
}
