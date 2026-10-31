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

// ─── the channel suffix (ranger-base-3a5z7) ──────────────────────────────────
//
// §6c's compare is on skillsVersionRelease of each side, not on the raw
// --version lines, because one CLI's line carries a field that is BOX STATE:
// grok 1.0.52 prints `grok 1.0.52 (0920c8169e27) [stable]` under the
// operator's $HOME and `grok 1.0.52 (0920c8169e27)` under a fresh one, the
// same binary both times, and the suffix comes from `~/.grok/version.json` —
// grok's own update-check cache (MEASURED 2026-10-31, bisected one file at a
// time; docs/notes.d/ranger-base-3a5z7.md). An exact compare has no right
// answer there: declaring the suffix reds every fresh-$HOME seat forever and
// dropping it reds the box that runs scripts/verify-box.sh, and no
// re-measurement clears either.

// Each case is here because it is the only one that can answer one guard in
// skillsVersionRelease: drop a trailing bracketed field, and NOTHING else.
func TestSkillsVersionReleaseDropsTheChannelFieldAndNothingElse(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want, why string }{
		{"grok 1.0.52 (0920c8169e27) [stable]", "grok 1.0.52 (0920c8169e27)",
			"the live case: the trailing channel field is dropped"},
		{"grok 1.0.52 (0920c8169e27)", "grok 1.0.52 (0920c8169e27)",
			"the same binary under a fresh $HOME is already the release"},
		{"codex-cli 0.153.4", "codex-cli 0.153.4",
			"codex carries no bracketed field, so its line is returned byte for byte"},
		{"2.0.5", "2.0.5",
			"bob's bare number is returned byte for byte"},
		{"cli [1.0] stable", "cli [1.0] stable",
			"a bracket that is not TRAILING is release identity"},
		{"cli 1.0 [stable", "cli 1.0 [stable",
			"an UNCLOSED bracket: the only case the HasSuffix guard alone can answer, since the inner-] check covers the closed-but-not-trailing one above and the slice arithmetic below is only in bounds because the line ends in ]"},
		{"[stable]", "[stable]",
			"a line that is nothing BUT the field keeps it — the i < 0 guard; dropping it would return \"\", which compares equal to every other empty read"},
		{"cli 1.0 [a] b]", "cli 1.0 [a] b]",
			"a stray ] after a field is not a field — without the inner-] guard this would swallow \" b]\" too"},
		{"cli 1.0 [a] [b]", "cli 1.0 [a]",
			"two fields: the trailing one goes, the one before it is not this rule's business"},
		{"grok 1.0.52 (0920c8169e27) [stable]  ", "grok 1.0.52 (0920c8169e27)",
			"ProbeCLIVersion trims, and so does this, so a trailing blank cannot decide a drift"},
	} {
		if got := skillsVersionRelease(c.in); got != c.want {
			t.Errorf("skillsVersionRelease(%q) = %q, want %q — %s", c.in, got, c.want, c.why)
		}
	}
}

// The walk, both directions, plus the part that must still red. A declaration
// and an installed line that differ ONLY by a trailing bracketed field are one
// release; anything ahead of the bracket moving is drift with the field
// present or absent.
func TestSkillsDriftReadsThroughTheChannelSuffixAndStillRedsOnTheRelease(t *testing.T) {
	a := checkApp(t)
	writeRuntime(t, a, "driftcli", "command: driftcli {skills} --sys {file}\nskills_cwd: true\nskills_verified: driftcli 1.0\n")

	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })

	// The grok case as it reads on the operator's box: the declaration is
	// bare and the CLI adds the field. Clean — and the tick has to name
	// BOTH strings, because "reports the same" over two different lines is
	// the reader being told something false.
	skillsDriftVersion = driftOnly("driftcli", "driftcli 1.0 [stable]")
	var b bytes.Buffer
	if st := a.ReportSkillsDrift(&b); st != SkillsDriftClean {
		t.Errorf("a trailing [field] on the installed line is box state, not drift: want exit %d, got %d:\n%s", SkillsDriftClean, st, b.String())
	}
	for _, want := range []string{"✓ driftcli", "driftcli 1.0", "driftcli 1.0 [stable]", "the same release", "box state"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("the clean-but-not-identical tick must carry %q:\n%s", want, b.String())
		}
	}
	if strings.Contains(b.String(), "reports the same") {
		t.Errorf("two different strings must not be reported as the same line:\n%s", b.String())
	}

	// And the other direction, which is the state a seat lands in when a
	// declaration was taken on a box that HAD the field: declared with it,
	// installed without. Also clean, so an old declaration does not red a
	// container tier.
	writeRuntime(t, a, "driftcli", "command: driftcli {skills} --sys {file}\nskills_cwd: true\nskills_verified: driftcli 1.0 [stable]\n")
	skillsDriftVersion = driftOnly("driftcli", "driftcli 1.0")
	b.Reset()
	if st := a.ReportSkillsDrift(&b); st != SkillsDriftClean {
		t.Errorf("a declaration carrying the field must not red a seat whose CLI omits it: want exit %d, got %d:\n%s", SkillsDriftClean, st, b.String())
	}

	// THE PART THAT MUST STILL RED, and the reason this is not just a
	// loosened compare: the release moved, and the suffix is along for the
	// ride on both sides. §6c's clock is unchanged for the only thing it was
	// built to catch.
	writeRuntime(t, a, "driftcli", "command: driftcli {skills} --sys {file}\nskills_cwd: true\nskills_verified: driftcli 1.0\n")
	for _, now := range []string{"driftcli 2.0 [stable]", "driftcli 2.0", "driftcli 1.0.1 [stable]"} {
		skillsDriftVersion = driftOnly("driftcli", now)
		b.Reset()
		if st := a.ReportSkillsDrift(&b); st != SkillsDriftFound {
			t.Errorf("installed %q against declared %q is a real release move: want exit %d, got %d:\n%s", now, "driftcli 1.0", SkillsDriftFound, st, b.String())
		}
		if got := b.String(); !strings.Contains(got, "drift:") || !strings.Contains(got, now) {
			t.Errorf("the drift sentence must name what the box actually printed (%q):\n%s", now, got)
		}
	}
}

// The declaration side of the same rule, swept over every built-in rather
// than pinned on grok: a `skills_verified:` is the release whose skill
// discovery was measured, so it may not carry a trailing bracketed field at
// all. grok's did (landed 2026-10-31 by ranger-base-9f2o6, found the same
// day), and the next `skills_cwd:` built-in someone declares off a live
// `--version` line is where it bites again.
func TestNoBuiltinDeclaresASkillsVerifiedCarryingBoxState(t *testing.T) {
	t.Parallel()
	declared := 0
	for _, rt := range builtinRuntimes {
		if rt.SkillsVerified == "" {
			continue
		}
		declared++
		if release := skillsVersionRelease(rt.SkillsVerified); release != rt.SkillsVerified {
			t.Errorf("%s declares skills_verified: %q — the trailing bracketed field is box state, not the release (%q). Declare the release; §6c's compare drops the field from the INSTALLED line on its own (ranger-base-3a5z7).",
				rt.Name, rt.SkillsVerified, release)
		}
	}
	// A positive witness that the sweep had subjects: an absence claim over
	// an empty walk is no claim (three built-ins declare one today).
	if declared < 3 {
		t.Fatalf("this sweep read %d declarations, so it cannot have measured the built-ins that carry one", declared)
	}
}
