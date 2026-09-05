package posse

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
	ag := &AgentFile{Name: "developer", MemoryDir: t.TempDir()}
	w := strings.Join(a.SeatbeltWritable(ag, work, t.TempDir(), rt.StateDirs...), "\n")
	if !strings.Contains(w, ExpandTilde("~/.mycli")) {
		t.Errorf("declared state_dir is not writable under seatbelt:\n%s", w)
	}
	// A runtime that was not launched contributes nothing — and after
	// ranger-base-9fl that holds for the BUILT-INS too, which is the
	// reversal: this block used to require ~/.claude ~/.claude.json
	// ~/.codex ~/.grok in a no-runtime caller's set, on the ground that it
	// should get "exactly what the literal granted". The literal granted
	// every runtime's auth store to every persona, and a write to another
	// runtime's auth store is an exfil channel (ADR 0019 posture review).
	// The set is now exactly the launching runtime's declaration.
	// TestQASeatbeltGrantsOnlyTheLaunchingRuntimesStateDir is the pin.
	base := strings.Join(a.SeatbeltWritable(ag, work, t.TempDir()), "\n")
	for _, unwanted := range []string{"~/.mycli", "~/.claude", "~/.claude.json", "~/.codex", "~/.grok"} {
		if strings.Contains(base, ExpandTilde(unwanted)) {
			t.Errorf("a runtime that was not launched contributed its state dir %s:\n%s", unwanted, base)
		}
	}
}

// A relative state_dir is refused: the session cwd is already writable, so
// the only thing a relative path can do here is grant a directory under the
// tree and leave the real state dir read-only — the bug with an extra step.
func TestStateDirRefusesARelativePath(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// ...and the refusal does not print what it refused. The surface it prints
// on is `posse runtime check`, which is output meant to be pasted into a
// bead, and beads sync to a store repo — so a refusal that echoes
// `AWS_SECRET_ACCESS_KEY=wJalr…` whole makes the visibility hop the key
// exists to prevent, in the sentence promising it never happens
// (ranger-base-60lj). Both arms are pinned, and both directions: the value
// is absent AND the elided entry is present, because a refusal that says
// nothing at all costs the operator the line they have to edit.
func TestEnvRequiredRefusalElidesTheValue(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	const secret = "wJalrXUtnFEMI0EXAMPLEKEY"
	for _, c := range []struct{ entry, want string }{
		{"AWS_SECRET_ACCESS_KEY=" + secret, `"AWS_SECRET_ACCESS_KEY=..."`}, // the = arm
		{"AWS_SECRET_ACCESS_KEY " + secret, `"AWS_SECRET_ACCESS_KEY ..."`}, // whitespace
		{"$" + secret, `"$..."`}, // a reference
		{"AWS_SECRET_ACCESS_KEY:" + secret, `"AWS_SECRET_ACCESS_KEY:..."`}, // the bad-character arm
		{"AWS_SECRET_ACCESS_KEY-" + secret, `"AWS_SECRET_ACCESS_KEY-..."`}, // same arm, another separator
		{"AWS_REGION=", `"AWS_REGION="`},                                   // nothing follows: nothing elided
	} {
		if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "leaky.yaml"),
			[]byte("command: c {file}\nenv_required: ["+c.entry+"]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := a.LoadRuntime("leaky")
		if err == nil {
			t.Errorf("env_required: %q must refuse", c.entry)
			continue
		}
		if msg := err.Error(); strings.Contains(msg, secret) {
			t.Errorf("env_required: %q — the refusal printed the value it exists to keep out of print: %s", c.entry, msg)
		} else if !strings.Contains(msg, c.want) {
			t.Errorf("env_required: %q — want the entry elided as %s, got: %s", c.entry, c.want, msg)
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	// exe is NOT in that list: it is asked on posse's own PATH, which is not
	// the one a launch resolves in, so it reports and never refuses
	// (ranger-base-8vys9 —
	// TestExeGapSaysWhichPATHItLookedOnAndDoesNotRefuse).
	if by["exe"].Blocking {
		t.Errorf("the exe gap answers about posse's PATH, not the pane's — it cannot refuse on it: %q", by["exe"].Line)
	}
	for _, name := range []string{"yaml", "env_required"} {
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
	t.Parallel()
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
	t.Parallel()
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

// The exe gap answers a question posse cannot ask from here. exec.LookPath
// runs in the POSSE process; the pane a launch opens is a child of the herdr
// DAEMON and resolves in the daemon's environment — measured 2026-09-05 with
// a scratch herdr, where a CLI planted only on the server's PATH is what the
// pane resolves and RUNS and one planted only on the client's is absent from
// the pane's PATH entirely (ranger-base-385x). herdr publishes no route to
// that environment, so this surface cannot close the gap; what it can do is
// stop refusing on the wrong PATH and say which one it looked on.
//
// Two facts, and the second is the one that cost: a CLI the daemon has and
// posse does not launches fine, and while the gap BLOCKED it also refused
// `posse runtime probe` — the one command that types the lookup into a pane
// and could have measured which of the two shapes a box is in. A runtime in
// that shape could not be onboarded at all.
func TestExeGapSaysWhichPATHItLookedOnAndDoesNotRefuse(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := writeRuntime(t, a, "fake", "command: posse-8vys9-no-such-exe {file}\n")
	gaps := a.RuntimeGaps(rt, Herdr{Bin: "no-such-herdr-binary"})
	var exe *RuntimeGap
	for i := range gaps {
		if gaps[i].Name == "exe" {
			exe = &gaps[i]
		}
	}
	if exe == nil {
		t.Fatalf("a CLI posse cannot resolve is still worth a line: %+v", gaps)
	}
	if exe.Blocking {
		t.Error("posse's own PATH is not the one that decides a launch — a miss on it cannot refuse one")
	}
	// Which PATH was looked on, and which one decides: a gap naming the
	// wrong PATH is worse than no gap, because the operator acts on it.
	for _, want := range []string{"posse's own PATH", "herdr daemon"} {
		if !strings.Contains(exe.Line, want) {
			t.Errorf("the line must name both PATHs — missing %q: %q", want, exe.Line)
		}
	}
	// And the route to the answer it cannot give, named with the runtime so
	// it can be pasted.
	if !strings.Contains(exe.Line, "posse runtime probe fake") {
		t.Errorf("the line must name the command that measures the session's own answer: %q", exe.Line)
	}

	// The consequence, at the predicate that used to refuse: RuntimeProbe
	// dies on the first BLOCKING gap (runtimeprobe.go, "there is nothing to
	// measure past one"), so a runtime whose only defect is an exe posse
	// cannot see must leave that loop with nothing to fire on. This is what
	// makes 385x's ProbeExeUnresolved record reachable.
	for _, g := range gaps {
		if g.Blocking {
			t.Errorf("nothing here blocks a probe of a runtime posse merely cannot see: %s: %s", g.Name, g.Line)
		}
	}
	// …and the same thing at `posse runtime check`'s verdict, which is its
	// exit status.
	var b strings.Builder
	if !a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b) {
		t.Errorf("a runtime posse cannot resolve but the daemon may is a named degrade, not a refusal:\n%s", b.String())
	}
	if !strings.Contains(b.String(), "nothing blocking") {
		t.Errorf("the verdict line must say so out loud:\n%s", b.String())
	}

	// An empty command: is a different fact and still refuses: no PATH
	// anywhere makes an absent argv0 launchable, so this one is not about
	// which PATH was asked.
	empty := &Runtime{Name: "empty"}
	var found bool
	for _, g := range a.RuntimeGaps(empty, Herdr{Bin: "no-such-herdr-binary"}) {
		if g.Name == "exe" {
			found = true
			if !g.Blocking {
				t.Errorf("a command: that renders no executable blocks whatever any PATH holds: %q", g.Line)
			}
		}
	}
	if !found {
		t.Error("a command: that renders no executable is still an exe gap")
	}
}
