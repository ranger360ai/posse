//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-f5jbs: the walk behind `posse runtime drift`. Every arm here is
// about the difference between a reading and the absence of one — the defect
// this surface closes is a detector that was RIGHT and exited 0, so "clean"
// and "nothing was measured" have to be two different answers or the exit
// code is worth no more than the line that was already there.
//
// skillsDriftVersion is the hermetic seam, and the fixtures below key on the
// exe name: this box's own built-ins are in ListRuntimes' walk, so a seam
// that answered one version for every exe would let whatever CLI is
// installed here decide these statuses.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// driftOnly answers version for exe and nothing for any other CLI, which is
// how the built-ins read as unmeasured rather than as this test's subject.
func driftOnly(exe, version string) func(string) string {
	return func(got string) string {
		if got == exe {
			return version
		}
		return ""
	}
}

func TestSkillsDriftExitStatusSeparatesCleanFromUnmeasured(t *testing.T) {
	a := checkApp(t)
	writeRuntime(t, a, "driftcli", "command: driftcli {skills} --sys {file}\nskills_cwd: true\nskills_verified: driftcli 1.0\n")

	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })

	// NOTHING MEASURED. No CLI on the box answers --version, so there is no
	// profile this walk read — which is not a pass, and the exit code and
	// the line both have to say so.
	skillsDriftVersion = func(string) string { return "" }
	var b bytes.Buffer
	if st := a.ReportSkillsDrift(&b); st != SkillsDriftUnmeasured {
		t.Errorf("no CLI answering --version must exit %d (nothing measured), got %d:\n%s", SkillsDriftUnmeasured, st, b.String())
	}
	if !strings.Contains(b.String(), "NOTHING WAS MEASURED") {
		t.Errorf("a walk that measured nothing must say so:\n%s", b.String())
	}
	// And the profile that COULD have been measured says why it was not,
	// naming the CLI — an unknown version is unknown, never unchanged.
	if got := b.String(); !strings.Contains(got, "· driftcli") || !strings.Contains(got, "answers no --version") {
		t.Errorf("the unmeasured profile must name itself and the reason:\n%s", got)
	}

	// CLEAN. The one profile with a declaration reads back the version it
	// was measured on.
	skillsDriftVersion = driftOnly("driftcli", "driftcli 1.0")
	b.Reset()
	if st := a.ReportSkillsDrift(&b); st != SkillsDriftClean {
		t.Errorf("a matching version must exit %d, got %d:\n%s", SkillsDriftClean, st, b.String())
	}
	if got := b.String(); !strings.Contains(got, "✓ driftcli") || strings.Contains(got, "drift:") {
		t.Errorf("a clean walk must tick the profile and report no drift:\n%s", got)
	}

	// DRIFTED. The sentence is skillsVerifiedDrift's own, so this surface
	// and the two display readers cannot disagree about what drift is.
	skillsDriftVersion = driftOnly("driftcli", "driftcli 2.0")
	b.Reset()
	st := a.ReportSkillsDrift(&b)
	if st != SkillsDriftFound {
		t.Errorf("a moved version must exit %d, got %d:\n%s", SkillsDriftFound, st, b.String())
	}
	for _, want := range []string{"✗ driftcli", "drift:", "driftcli 1.0", "driftcli 2.0", "FINDING", "RE-MEASUREMENT"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("drift report missing %q:\n%s", want, b.String())
		}
	}
}

// A profile posse cannot READ is a finding and not a skip: the yaml may
// declare a skills_verified: nobody can reach, and a walk that exits 0 over
// one is green over a profile it never saw. (`skills_verified:` with no
// `skills_cwd:` is an existing load refusal — runtime.go — so this plants a
// real one rather than inventing a shape.)
func TestSkillsDriftRedsOnAProfileItCannotRead(t *testing.T) {
	a := checkApp(t)
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "broken.yaml"),
		[]byte("command: broken {skills} --sys {file}\nskills_verified: broken 1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })
	skillsDriftVersion = func(string) string { return "" }

	var b bytes.Buffer
	if st := a.ReportSkillsDrift(&b); st != SkillsDriftFound {
		t.Errorf("a profile that will not load must exit %d, got %d:\n%s", SkillsDriftFound, st, b.String())
	}
	for _, want := range []string{"✗ broken", "REFUSED"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("the refusal must be named on the row: missing %q:\n%s", want, b.String())
		}
	}
}

// ADR 0007 §6b is a DIFFERENT finding and already a loud one — a `skills:`
// PID on an UNPROVEN root degrades the launch. There is no version to
// compare, so this walk reports that it did not cover the profile and does
// not red: a §6b profile reddening the §6c clock would make the clock
// permanently red on a box that is behaving exactly as declared.
func TestSkillsDriftDoesNotRedAnUnprovenRoot(t *testing.T) {
	a := checkApp(t)
	writeRuntime(t, a, "unproven", "command: unproven {skills} --sys {file}\nskills_cwd: true\n")
	writeRuntime(t, a, "driftcli", "command: driftcli {skills} --sys {file}\nskills_cwd: true\nskills_verified: driftcli 1.0\n")

	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })
	skillsDriftVersion = driftOnly("driftcli", "driftcli 1.0")

	var b bytes.Buffer
	if st := a.ReportSkillsDrift(&b); st != SkillsDriftClean {
		t.Errorf("an UNPROVEN root is §6b's finding, not this clock's: want exit %d, got %d:\n%s", SkillsDriftClean, st, b.String())
	}
	for _, want := range []string{"· unproven", "UNPROVEN", "ADR 0007 §6b"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("the UNPROVEN profile must still be named and explained: missing %q:\n%s", want, b.String())
		}
	}
}

// The walk covers the ROSTER, not its findings alone. A profile with a flag
// surface (claude's shape) measures no version at all, and printing nothing
// about it is how a reader comes to go looking for a finding that was never
// owed.
func TestSkillsDriftNamesEveryProfileItWalked(t *testing.T) {
	a := checkApp(t)
	writeRuntime(t, a, "flagcli", "command: flagcli {skills} --sys {file}\nskills_flag: --plugin-dir %s\n")

	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })
	skillsDriftVersion = func(string) string { return "" }

	var b bytes.Buffer
	a.ReportSkillsDrift(&b)
	for _, name := range a.ListRuntimes() {
		if !strings.Contains(b.String(), name) {
			t.Errorf("profile %q is in the walk and not on the screen:\n%s", name, b.String())
		}
	}
	if !strings.Contains(b.String(), "· flagcli") {
		t.Errorf("a flag-surface profile must say it is not this clock's subject:\n%s", b.String())
	}
}
