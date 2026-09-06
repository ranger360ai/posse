//go:build posse_arm3

package posse

// ADR 0031 §2–3: `posse init` joins the operator fence, keyed on the target
// home rather than blanket-refused under RHQ_PERSONA (the promote/refresh
// shape) — the harm is a property of WHICH home is written, and a persona's
// `RHQ_HOME=<scratch> posse init` is how QA seeds fixtures and how the leak
// this ADR fixes (ranger-base-x26u) was itself measured. Six arms, each
// standing for one branch of the fence in initFrom (init.go):
//
//  1. persona + origin=live + target=live            → refused, nothing written
//  2. persona + origin=live + target=scratch          → seeds fully
//  3. persona + origin absent                         → refused, names relaunch
//  4. no persona                                       → seeds (operator unchanged)
//  5. persona + target a symlink resolving into origin → refused
//  6. persona + target a not-yet-existing dir under origin → refused
//
// All hermetic: t.Setenv for HOME/RHQ_HOME/RHQ_PERSONA/RHQ_LAUNCH_HOME, temp
// dirs only — no arm here may touch the operator's real $HOME.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// fenceFixture roots HOME and RHQ_HOME in a scratch tree so nothing here can
// reach the invoking operator's real home, then returns an App at target
// with persona/origin set as asked. Each caller drives initFrom itself, since
// the assertion differs per arm (refused vs seeded).
func fenceFixture(t *testing.T, target, persona, origin string) *App {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "operator-home"))
	t.Setenv("RHQ_HOME", target)
	t.Setenv(EnvPersona, persona)
	if persona != "" {
		// Set explicitly so an empty string (arm 3's "absent") is distinct
		// from never having called t.Setenv at all — both read "" from
		// os.Getenv, which is exactly the point of arm 3.
		if origin == "" {
			// t.Setenv first so the ambient value (a real persona session
			// has one) is registered for restore; os.Unsetenv alone leaks
			// the absence into every test that runs after this one.
			t.Setenv(EnvLaunchHome, "")
			os.Unsetenv(EnvLaunchHome)
		} else {
			t.Setenv(EnvLaunchHome, origin)
		}
	}
	return NewAppAt(target)
}

func assertRefused(t *testing.T, a *App, wantSubstr string) {
	t.Helper()
	var out strings.Builder
	err := a.initFrom(&out, posse.Seed, "embedded")
	if err == nil {
		t.Fatalf("initFrom(%s) succeeded, want refusal", a.Home)
	}
	if !strings.Contains(err.Error(), "ADR 0031") {
		t.Errorf("refusal does not name ADR 0031: %v", err)
	}
	if !strings.Contains(err.Error(), "RHQ_HOME=<scratch> posse init") {
		t.Errorf("refusal does not print the scratch working form: %v", err)
	}
	if wantSubstr != "" && !strings.Contains(err.Error(), wantSubstr) {
		t.Errorf("refusal = %q, want it to mention %q", err.Error(), wantSubstr)
	}
	if _, statErr := os.Stat(a.ConfigPath); statErr == nil {
		t.Errorf("initFrom wrote %s despite refusing", a.ConfigPath)
	}
}

func assertSeeded(t *testing.T, a *App) {
	t.Helper()
	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("initFrom(%s) = %v, want it to seed", a.Home, err)
	}
	if _, err := os.Stat(a.ConfigPath); err != nil {
		t.Errorf("initFrom did not seed %s: %v", a.ConfigPath, err)
	}
}

// Arm 1: the exact leak ranger-base-x26u measured — a persona session whose
// target IS the home it was launched from.
func TestQAInitFenceRefusesLiveHomeEqualToOrigin(t *testing.T) {
	live := filepath.Join(t.TempDir(), "live-home")
	a := fenceFixture(t, live, "developer", live)
	assertRefused(t, a, live)
}

// Arm 2: the measured QA path (h7cd) and scratch seeding must stay open —
// same persona, an origin that is NOT the target.
func TestQAInitFenceAllowsScratchTargetUnderPersona(t *testing.T) {
	live := filepath.Join(t.TempDir(), "live-home")
	scratch := filepath.Join(t.TempDir(), "scratch-home")
	a := fenceFixture(t, scratch, "developer", live)
	assertSeeded(t, a)
}

// Arm 3: fail closed. A session that cannot prove where it came from does
// not get to write anywhere it might have come from.
func TestQAInitFenceRefusesWhenOriginAbsent(t *testing.T) {
	target := filepath.Join(t.TempDir(), "some-home")
	a := fenceFixture(t, target, "developer", "")
	assertRefused(t, a, "relaunch")
}

// Arm 4: the operator's own path is unchanged — no persona, no fence, even
// pointed straight at what would otherwise be a live-equals-target refusal.
func TestQAInitFenceIgnoresOperatorSessions(t *testing.T) {
	live := filepath.Join(t.TempDir(), "live-home")
	a := fenceFixture(t, live, "", live)
	assertSeeded(t, a)
}

// Arm 5: the target is reached through a symlink whose real path resolves
// inside the origin — the pre-cutover shape (ADR 0015 §2), where the home is
// a symlink onto the instance repo. underDir must resolve it, not compare
// the alias path literally.
func TestQAInitFenceRefusesSymlinkedTargetIntoOrigin(t *testing.T) {
	root := t.TempDir()
	live := filepath.Join(root, "live-home")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias-home")
	if err := os.Symlink(live, alias); err != nil {
		t.Fatal(err)
	}
	a := fenceFixture(t, alias, "developer", live)
	assertRefused(t, a, "")
}

// Arm 6: the throwaway-target ordinary case — a target that does not exist
// YET, nested under the origin. The longest-EXISTING-prefix resolution must
// still catch it; a naive EvalSymlinks(target) would error on a path that
// isn't there and silently fail open.
func TestQAInitFenceRefusesNotYetExistingTargetUnderOrigin(t *testing.T) {
	live := filepath.Join(t.TempDir(), "live-home")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(live, "not-yet-created")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("fixture bug: %s already exists", target)
	}
	a := fenceFixture(t, target, "developer", live)
	assertRefused(t, a, "")
}
