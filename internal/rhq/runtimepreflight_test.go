package rhq

// ADR 0012 D4 / rangerhq-tr8k: the three preflight declarations and the
// check that reports what is missing.
//
// Everything here is hermetic. The gap machinery takes herdr as a
// parameter and the env as a parameter, so none of it depends on which CLIs
// happen to be installed on the box running the suite — which is the whole
// point of splitting a DECLARATION (the profile is right) from a PREFLIGHT
// (this machine can run it).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// state_dir: the key exists so a third-party CLI's own state survives
// `cage: seatbelt`. The pin is on the writable set, not on the getter: a
// field that parses and never reaches the profile is the bug.
func TestStateDirJoinsTheSeatbeltWritableSet(t *testing.T) {
	// The writable set is built from os.UserHomeDir(); no test in this
	// package gets a temp HOME by default, so this one takes its own rather
	// than asserting against the operator's live home (ranger-base-e06g).
	t.Setenv("HOME", t.TempDir())
	a := checkApp(t)
	rt := writeRuntime(t, a, "mycli", "command: mycli {file}\nstate_dir: ~/.mycli\n")
	if got := rt.StateDirs; len(got) != 1 || got[0] != "~/.mycli" {
		t.Fatalf("state_dir: not parsed: %v", got)
	}

	work := t.TempDir()
	ag := &AgentFile{Name: "dinesh", MemoryDir: t.TempDir()}
	w := strings.Join(a.SeatbeltWritable(ag, work, t.TempDir(), rt.StateDirs...), "\n")
	if !strings.Contains(w, ExpandTilde("~/.mycli")) {
		t.Errorf("declared state_dir is not writable under seatbelt:\n%s", w)
	}
	// The built-ins' own dirs are in the same set, and they are declarations
	// now rather than a literal in the profile builder — so a caller that
	// passes no runtime still gets exactly what the literal granted.
	base := strings.Join(a.SeatbeltWritable(ag, work, t.TempDir()), "\n")
	for _, want := range []string{"~/.claude", "~/.claude.json", "~/.codex", "~/.grok"} {
		if !strings.Contains(base, ExpandTilde(want)) {
			t.Errorf("built-in state dir %s dropped from the writable set:\n%s", want, base)
		}
	}
	if strings.Contains(base, ExpandTilde("~/.mycli")) {
		t.Error("a runtime that was not launched must not contribute its state dir")
	}
}

// A relative state_dir is refused: the session cwd is already writable, so
// the only thing a relative path can do here is grant a directory under the
// tree and leave the real state dir read-only — the bug with an extra step.
func TestStateDirRefusesARelativePath(t *testing.T) {
	a := checkApp(t)
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "bad.yaml"),
		[]byte("command: bad {file}\nstate_dir: .bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := a.LoadRuntime("bad")
	if err == nil || !strings.Contains(err.Error(), "state_dir") {
		t.Fatalf("want a refusal naming state_dir, got %v", err)
	}
}

// env_required: NAMES ONLY, and it is enforced rather than documented. A
// list an operator can put a value in is a list whose contents end up in a
// terminal, which is the one thing envs: guarantees never happens.
func TestEnvRequiredTakesNamesNeverValues(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "bedrock", "command: claude {file}\nenv_required: [AWS_REGION, AWS_PROFILE]\n")
	if got := rt.EnvRequired; len(got) != 2 || got[0] != "AWS_REGION" || got[1] != "AWS_PROFILE" {
		t.Fatalf("env_required: not parsed: %v", got)
	}
	for _, bad := range []string{"AWS_REGION=us-east-1", "$AWS_REGION", "aws region"} {
		if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "v.yaml"),
			[]byte("command: c {file}\nenv_required: ["+bad+"]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := a.LoadRuntime("v"); err == nil {
			t.Errorf("env_required: %q must refuse — it is not a name", bad)
		}
	}
}

// The launch preflight's arithmetic, without a launch: what the session
// receives wins, the launcher's own environment counts, and present-but-
// empty is missing (an empty AWS_REGION is the same dead pane).
func TestMissingEnvLooksInBothPlaces(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "bedrock", "command: claude {file}\nenv_required: [POSSE_TR8K_A, POSSE_TR8K_B]\n")

	if got := MissingEnv(rt, nil); len(got) != 2 {
		t.Fatalf("nothing set: want both missing, got %v", got)
	}
	// An env set the session receives supplies it.
	vars := []EnvVar{{"POSSE_TR8K_A", "x"}, {"POSSE_TR8K_B", ""}}
	got := MissingEnv(rt, vars)
	if len(got) != 1 || got[0] != "POSSE_TR8K_B" {
		t.Errorf("an env-set value counts and an EMPTY one does not: %v", got)
	}
	// The operator's own exported environment supplies it just as
	// legitimately — the launch inherits it.
	t.Setenv("POSSE_TR8K_B", "y")
	if got := MissingEnv(rt, vars); len(got) != 0 {
		t.Errorf("an exported name is supplied: %v", got)
	}
	// And the refusal names the missing variables and nothing else: no
	// value, and not the names that WERE found.
	err := EnvRequiredError(rt, []string{"POSSE_TR8K_B"}).Error()
	if !strings.Contains(err, "POSSE_TR8K_B") || strings.Contains(err, "POSSE_TR8K_A") || strings.Contains(err, "\"y\"") {
		t.Errorf("the refusal is names-only, and only the missing ones:\n%s", err)
	}
}

// Dismissals are DECLARABLE (ADR 0012 D4) and they are never keystrokes.
// rangerhq-6723 retired the launcher's one keystroke table and rangerhq-4mzt
// ruled that no drawn dialog is the launcher's to answer, so what a third
// party can declare is the screen and the operator-owned key that silences
// it — with no probe, because posse cannot read an unknown CLI's config.
func TestDeclaredInterstitialsAreNamedNotPressed(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "mycli", `command: mycli {file}
interstitial_update:
  screen: "Update available! 1. Update now"
  where: ~/.mycli/version.json
  key: dismissed_version
  silence: "the OPERATOR picks 3"
  danger: "the default option upgrades the machine's tooling"
`)
	if len(rt.Interstitials) != 1 {
		t.Fatalf("interstitial_<name>: not parsed: %+v", rt.Interstitials)
	}
	in := rt.Interstitials[0]
	if in.Key != "dismissed_version" || in.Where != "~/.mycli/version.json" || in.Danger == "" {
		t.Errorf("declared fields did not arrive: %+v", in)
	}
	if in.Probe != nil {
		t.Error("a DECLARED interstitial must carry no probe — posse cannot read an unknown CLI's config, and a guessed probe answers no for a screen silenced years ago")
	}
	if in.Seeded {
		t.Error("Seeded is never declarable: it is posse WRITING the operator's config")
	}

	// The grid names the key and never a keystroke.
	var b strings.Builder
	a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
	out := b.String()
	if !strings.Contains(out, "dismissed_version") || !strings.Contains(out, "state unknown") {
		t.Errorf("the grid must name the key and say the state is unknown:\n%s", out)
	}
	for _, never := range []string{"send-keys", "press Esc", "Enter is sent"} {
		if strings.Contains(out, never) {
			t.Errorf("the grid offered a keystroke (%q) — rangerhq-4mzt:\n%s", never, out)
		}
	}
}

// A half-declared screen refuses rather than printing a blank: an entry with
// no key silences nothing, and one with no file is not a place.
func TestIncompleteInterstitialRefuses(t *testing.T) {
	a := checkApp(t)
	for _, body := range []string{
		"command: c {file}\ninterstitial_x:\n  screen: \"S\"\n",
		"command: c {file}\ninterstitial_x:\n  screen: \"S\"\n  key: k\n",
		"command: c {file}\ninterstitial_x:\n  screen: \"S\"\n  where: ~/f\n  key: k\n  silense: oops\n",
	} {
		if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "x.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := a.LoadRuntime("x"); err == nil {
			t.Errorf("a half-declared screen must refuse:\n%s", body)
		}
	}
}

// The whole point of the command: every gap by name, and the split between
// what blocks a launch and what is a named degrade.
func TestRuntimeGapsReportEachGapByName(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "fake", `command: posse-tr8k-no-such-exe {file}
env_required: [POSSE_TR8K_ABSENT]
skils_flag: --oops
`)
	// A herdr that answers nothing is UNKNOWN, never a wrong "no".
	gaps := a.RuntimeGaps(rt, Herdr{Bin: "no-such-herdr-binary"})
	by := map[string]RuntimeGap{}
	for _, g := range gaps {
		by[g.Name] = g
	}
	for _, want := range []string{"exe", "detection", "yaml", "env_required"} {
		if _, ok := by[want]; !ok {
			t.Errorf("gap %q not reported; got %+v", want, gaps)
		}
	}
	if by["detection"].Blocking {
		t.Error("a herdr that cannot be asked yields UNKNOWN — non-blocking — not a wrong 'no'")
	}
	for _, name := range []string{"exe", "yaml", "env_required"} {
		if !by[name].Blocking {
			t.Errorf("%s is a blocking gap: a launch on it cannot work", name)
		}
	}
	if !strings.Contains(by["yaml"].Line, "skils_flag") {
		t.Errorf("the yaml gap must name the key nothing reads: %q", by["yaml"].Line)
	}
	if !strings.Contains(by["env_required"].Line, "POSSE_TR8K_ABSENT") {
		t.Errorf("the env gap must name the variable: %q", by["env_required"].Line)
	}
	// And RuntimeCheck's verdict follows the blocking gaps.
	var b strings.Builder
	if a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b) {
		t.Errorf("a runtime with blocking gaps must not report clean:\n%s", b.String())
	}
	if !strings.Contains(b.String(), "preflight — ") {
		t.Errorf("the preflight section is missing:\n%s", b.String())
	}
}

// The interstitial_<name>: family is open by construction — only the
// profile's author knows the slug — so it must not warn as an unknown key,
// while a typo outside the family still does.
func TestInterstitialFamilyIsNotAnUnknownKey(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "mycli", `command: mycli {file}
interstitial_consent:
  screen: "Help improve MyCLI"
  where: ~/.mycli/config.toml
  key: consent_acked
`)
	if got := unknownRuntimeKeys(rt); len(got) != 0 {
		t.Errorf("interstitial_<name>: is a known family, not an unknown key: %v", got)
	}
	rt2 := writeRuntime(t, a, "typo", "command: c {file}\nintersitial_x: 1\n")
	if got := unknownRuntimeKeys(rt2); len(got) != 1 || got[0] != "intersitial_x" {
		t.Errorf("a near-miss of the family name is still unknown: %v", got)
	}
}

// The three built-ins declare everything the preflight reads, so the only
// gaps they can ever have are environmental (the CLI not installed, herdr
// not running). Hermetic half: nothing structural is missing.
func TestBuiltinsDeclareTheirPreflight(t *testing.T) {
	a := checkApp(t)
	for _, name := range []string{"claude", "codex", "grok"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatal(err)
		}
		if len(rt.StateDirs) == 0 {
			t.Errorf("%s declares no state_dir — `cage: seatbelt` would make its own config read-only", name)
		}
		if len(rt.EnvRequired) != 0 {
			t.Errorf("%s declares env_required: %v — the built-ins authenticate from their state dir", name, rt.EnvRequired)
		}
		if len(unknownRuntimeKeys(rt)) != 0 {
			t.Errorf("%s is a built-in and reads no yaml", name)
		}
	}
}

// The launch half of env_required: planLaunch refuses, so the missing name
// is a refusal at the top of the launch rather than a pane that opens,
// fails to authenticate, and reads to herdr as an agent sitting idle.
func TestLaunchRefusesWhenEnvRequiredIsMissing(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "bedrock.yaml"),
		[]byte("command: bedrockcli --sys \"$(cat {file})\"\nenv_required: [POSSE_TR8K_LAUNCH]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(a.AgentsDir, 0o755)
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndescription: t\nruntime: bedrock\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"})
	if err == nil || !strings.Contains(err.Error(), "POSSE_TR8K_LAUNCH") {
		t.Fatalf("want a refusal naming the missing variable, got %v", err)
	}
	// Supplied — by the operator's own environment, which the session
	// inherits — and the launch plans.
	t.Setenv("POSSE_TR8K_LAUNCH", "us-east-1")
	if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("a supplied env_required must not refuse: %v", err)
	}
}
