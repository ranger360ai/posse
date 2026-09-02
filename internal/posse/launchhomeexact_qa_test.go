package posse

// ADR 0031 §1's done-when is that every created session's env carries
// RHQ_LAUNCH_HOME EQUAL TO a.Home. The close's pins
// (TestHerdrCreateSession, TestPersonaLaunchRuntime) assert
// strings.Contains(log, "--env RHQ_LAUNCH_HOME="+Home), which is a PREFIX
// test: ranger-base-uco3m measured that `EnvVar{EnvLaunchHome, a.Home+"-x"}`
// leaves both of them green, because the rendered token still begins with
// the string they look for. That matters more here than for an ordinary
// value — RHQ_LAUNCH_HOME is not an address anything resolves through, so a
// wrong value is invisible at runtime and shows up only as `posse init`
// silently declining to fence (a target that no longer resolves inside the
// origin) or fencing a scratch home it should have seeded.
//
// This pin reads the VALUE and compares it, on both paths the close claims.

import (
	"regexp"
	"strings"
	"testing"
)

// launchHomeValues returns every RHQ_LAUNCH_HOME value the rendered launch
// calls carry — the token as typed, cut at the next whitespace, so a value
// with something appended reads as the different string it is.
func launchHomeValues(log string) []string {
	re := regexp.MustCompile(`--env RHQ_LAUNCH_HOME=(\S*)`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(log, -1) {
		out = append(out, m[1])
	}
	return out
}

func assertLaunchHomeExact(t *testing.T, log, want, path string) {
	t.Helper()
	got := launchHomeValues(log)
	if len(got) == 0 {
		t.Fatalf("%s: no --env RHQ_LAUNCH_HOME in the rendered launch (ADR 0031 §1):\n%s", path, log)
	}
	for _, g := range got {
		if g != want {
			t.Errorf("%s: RHQ_LAUNCH_HOME = %q, want exactly %q (ADR 0031 §1: the origin RECORD, equal to a.Home)", path, g, want)
		}
	}
}

// The crew path. A crew session runs rhq/bd tools too and `posse init` from
// one is fenced by the same comparison, so the record rides here as well.
func TestQALaunchHomeValueIsExactOnTheCrewPath(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "proj", Dir: dir, Cmd: "npm run dev"})
	log := calls(t, fake)
	assertLaunchHomeExact(t, log, b.App.Home, "crew")
	// The record and the address start life equal — that equality is the
	// whole reason init can compare a later, overridden RHQ_HOME against it.
	if !strings.Contains(log, "--env RHQ_HOME="+b.App.Home) {
		t.Errorf("crew: RHQ_HOME missing:\n%s", log)
	}
}

// The persona path, which is the one the leak (ranger-base-x26u) was
// measured on.
func TestQALaunchHomeValueIsExactOnThePersonaPath(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "h1", Dir: dir, Agent: "ranger"})
	assertLaunchHomeExact(t, calls(t, fake), b.App.Home, "persona")
}

// CageEnvNames is a NAME list — the engine forwards `-e NAME` and takes the
// value from the pane's own environment — so the name is the whole claim at
// that tier, and it must be there exactly once.
func TestQALaunchHomeCrossesTheCageBoundaryByName(t *testing.T) {
	t.Parallel()
	names := CageEnvNames(nil)
	n := 0
	for _, s := range names {
		if s == EnvLaunchHome {
			n++
		}
	}
	if n != 1 {
		t.Errorf("CageEnvNames names %s %d times, want exactly 1 (ADR 0031 §1): %v", EnvLaunchHome, n, names)
	}
}
