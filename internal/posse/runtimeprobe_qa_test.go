package posse

// ADR 0032 §1 rule 1, assumed-until-probed: the parity wiring, which is the
// half that changes what a launch does. The probe command is the unlock;
// this is the lock, and the two ship together on purpose — refusing without
// offering the unlock is the alternative the ADR rejected.
//
// Every arm here drives production CheckParity / RuntimeCheck. The point of
// the file is that deleting the parity clause must turn one of these red.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// probeParityApp is an App with a template-only runtime declared in yaml and
// a state dir of its own. `bob` is the ADR's own name for a CLI the harness
// has never seen.
func probeParityApp(t *testing.T) (*App, *Runtime) {
	t.Helper()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "bob.yaml"), []byte("command: bob --pid {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime("bob")
	if err != nil {
		t.Fatal(err)
	}
	if rt.Builtin {
		t.Fatal("bob must load as template-only, or this file tests the wrong thing")
	}
	return a, rt
}

// writeProbe stores a record for bob with every observable green (or one
// red), so the tests below can move the runtime between assumed and measured
// without a CLI.
func writeProbe(t *testing.T, a *App, pass bool) {
	t.Helper()
	obs := evalProbe(passingReading("/tmp/gates/bin"))
	if !pass {
		obs[0] = ProbeObservable{1, "shim-precedence", false, "command -v uname → /usr/bin/uname"}
	}
	rec := &ProbeRecord{
		Runtime: "bob", CLIPath: "/usr/local/bin/bob", LauncherPath: "/usr/local/bin/bob",
		Version: "bob 1.2.3",
		Date:    time.Now().UTC(), PosseVersion: Version, Canary: "uname", Observables: obs,
	}
	if err := a.WriteProbeRecord(rec); err != nil {
		t.Fatal(err)
	}
}

func TestTemplateBashDenyIsAssumedUntilProbed(t *testing.T) {
	t.Parallel()
	a, bob := probeParityApp(t)
	dev := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(rm -rf /)\n---\nYou are dev.\n")

	// Unprobed: BOTH shell-verb denies land in Degraded, and neither is
	// Realized. This is the whole change — before it, parity counted them
	// realized on the strength of three behaviours nobody had measured.
	p := a.CheckParity(dev, bob, CageShims, TierStrong)
	if len(p.Unrealized) != 2 {
		t.Fatalf("both Bash denies must be unrealized on an unprobed template runtime: %+v", p)
	}
	for _, rule := range []string{"Bash(git push:*)", "Bash(rm -rf /)"} {
		if p.Realized[rule].Detail != "" {
			t.Errorf("%s must not read as realized before a probe: %q", rule, p.Realized[rule].Detail)
		}
	}
	joined := strings.Join(p.Degraded, "\n")
	for _, want := range []string{"assumed, not measured", "posse runtime probe bob", "no probe record"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the degrade line must carry %q — a refusal that does not name its unlock trains the operator to waive by habit:\n%s", want, joined)
		}
	}

	// A recorded FAILURE is not a record that unlocks anything, and it must
	// say what failed rather than repeating "no probe record".
	writeProbe(t, a, false)
	p = a.CheckParity(dev, bob, CageShims, TierStrong)
	if len(p.Unrealized) != 2 {
		t.Fatalf("a FAILED probe leaves the claim assumed: %+v", p)
	}
	if j := strings.Join(p.Degraded, "\n"); !strings.Contains(j, "FAILED") || !strings.Contains(j, "/usr/bin/uname") {
		t.Errorf("a failed probe must be reported as measured-and-broken, with the observable:\n%s", j)
	}

	// Probed and passing: the claim flips to realized, exactly as it reads
	// on a built-in, and the launch stops degrading.
	writeProbe(t, a, true)
	p = a.CheckParity(dev, bob, CageShims, TierStrong)
	if len(p.Degraded) != 0 {
		t.Fatalf("a passing probe must clear the degradation: %+v", p.Degraded)
	}
	if p.Realized["Bash(rm -rf /)"].Detail != "L1 shim (literal argv prefix)" || p.Realized["Bash(git push:*)"].Detail != "L1 shim (subcommand, option-aware)" {
		t.Errorf("after a passing probe the wall reads as it does on a built-in: %+v", p.Realized)
	}
}

// The waiver semantics are the standard ones, which means tier fast is not
// on offer (ADR 0003 §3). Pinned here because "standard waiver semantics"
// is a sentence in the ADR and a property of the code only as long as the
// degrade goes through p.Degraded rather than through a bespoke field.
func TestAssumedProbeIsNotWaivableAtTierFast(t *testing.T) {
	t.Parallel()
	a, bob := probeParityApp(t)
	dev := loadTestAgent(t, "---\nname: dev\ndeny: [Bash(rm -rf /)]\n---\nYou are dev.\n")
	if p := a.CheckParity(dev, bob, CageShims, TierFast); !p.NoDegrade {
		t.Errorf("an unprobed template runtime at tier fast must refuse without a waiver: %+v", p)
	}
	writeProbe(t, a, true)
	if p := a.CheckParity(dev, bob, CageShims, TierFast); p.NoDegrade || len(p.Degraded) != 0 {
		t.Errorf("a probed runtime at tier fast is clean: %+v", p)
	}
}

// Built-ins are exempt by MEASUREMENT, not by privilege — ADR 0009's argv
// table (rangerhq-e43) — and no yaml is read for them, so there is nobody to
// author a probe for. The arm matters: if the clause keyed on "has a Path"
// or on "is not claude" instead, this is where it would show.
func TestBuiltinRuntimesDoNotWaitOnAProbe(t *testing.T) {
	t.Parallel()
	a, _ := probeParityApp(t)
	dev := loadTestAgent(t, "---\nname: dev\ndeny: [Bash(rm -rf /)]\n---\nYou are dev.\n")
	for _, name := range []string{"claude", "codex", "grok"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatal(err)
		}
		p := a.CheckParity(dev, rt, CageShims, TierStrong)
		if len(p.Degraded) != 0 || p.Realized["Bash(rm -rf /)"].Detail == "" {
			t.Errorf("%s is probe-backed by ADR 0009 and must not degrade: %+v", name, p)
		}
	}
}

// A template runtime that also declares `gate_shell: false` was already
// unrealized for a different reason, and it must keep saying that reason —
// an operator sent to `posse runtime probe` for a runtime whose wrapper is
// switched off would probe, pass three observables, and still have no wall.
func TestGateShellFalseKeepsItsOwnDiagnosis(t *testing.T) {
	t.Parallel()
	a, _ := probeParityApp(t)
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "odd.yaml"), []byte("command: odd --pid {file}\ngate_shell: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	odd, err := a.LoadRuntime("odd")
	if err != nil {
		t.Fatal(err)
	}
	dev := loadTestAgent(t, "---\nname: dev\ndeny: [Bash(rm -rf /)]\n---\nYou are dev.\n")
	p := a.CheckParity(dev, odd, CageShims, TierStrong)
	j := strings.Join(p.Unrealized, "\n")
	if !strings.Contains(j, "gate_shell: false") {
		t.Errorf("gate_shell: false owns its own diagnosis: %s", j)
	}
	if strings.Contains(j, "posse runtime probe") {
		t.Errorf("a runtime with no gate shell must not be sent to the probe — it would pass and still have no wall: %s", j)
	}
}

// `posse runtime check` is where an onboarder learns the claim is
// conditional, so the probe row prints whether or not it is a gap, and it
// prints the OPPOSITE word once a record lands.
func TestRuntimeCheckPrintsTheProbeRowBothWays(t *testing.T) {
	t.Parallel()
	a, bob := probeParityApp(t)
	var out bytes.Buffer
	a.RuntimeCheck(bob, Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}, &out)
	got := out.String()
	// Scoped to the probe ROW, not to the screen. ASSUMED and MEASURED are
	// ordinary English on a grid that spends most of its width telling an
	// onboarder which facts were measured — the skills row says "declare the
	// one you MEASURED" — so an unscoped Contains here answers about
	// whichever row said the word first, and would have called this runtime
	// probed on the strength of a sentence about skills (ranger-base-bcpa).
	row := gridRow(t, got, "probe")
	for _, want := range []string{"ASSUMED", "posse runtime probe bob"} {
		if !strings.Contains(row, want) {
			t.Errorf("the unprobed grid's probe row must carry %q:\n%s", want, row)
		}
	}
	if strings.Contains(row, "MEASURED") {
		t.Errorf("an unprobed runtime must not print MEASURED:\n%s", row)
	}

	writeProbe(t, a, true)
	out.Reset()
	a.RuntimeCheck(bob, Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}, &out)
	got = out.String()
	row = gridRow(t, got, "probe")
	if !strings.Contains(row, "MEASURED") || strings.Contains(row, "ASSUMED") {
		t.Errorf("a probed runtime prints MEASURED and not ASSUMED:\n%s", row)
	}
	if !strings.Contains(got, a.ProbeRecordPath("bob")) && !strings.Contains(got, AbbrevHome(a.ProbeRecordPath("bob"))) {
		t.Errorf("the grid must name the record it read:\n%s", got)
	}

	// And a built-in gets the honest version rather than a remedy that
	// unlocks nothing (the same rule the onboarding footer already follows).
	claude, _ := a.LoadRuntime("claude")
	out.Reset()
	a.RuntimeCheck(claude, Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}, &out)
	if got := out.String(); !strings.Contains(got, "not applicable") {
		t.Errorf("a built-in's probe row says the probe does not apply:\n%s", got)
	}
}

// The preflight reports the probe as a NON-BLOCKING gap: an unprobed
// template runtime still takes work, it just takes it degraded. Blocking it
// would make `runtime check` exit 1 for every freshly authored profile,
// which turns ADR 0032's goal into a requirement by accident of exit status.
func TestProbeGapIsNamedAndNonBlocking(t *testing.T) {
	t.Parallel()
	a, bob := probeParityApp(t)
	h := Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}
	var found *RuntimeGap
	for _, g := range a.RuntimeGaps(bob, h) {
		if g.Name == "probe" {
			gg := g
			found = &gg
		}
	}
	if found == nil {
		t.Fatalf("the preflight must report the probe gap by name: %+v", a.RuntimeGaps(bob, h))
	}
	if found.Blocking {
		t.Error("an unprobed runtime is a named degrade, not a refusal")
	}
	if !strings.Contains(found.Line, "--allow-degraded") || !strings.Contains(found.Line, "tier fast") {
		t.Errorf("the gap must state the waiver semantics it costs: %q", found.Line)
	}
	writeProbe(t, a, true)
	for _, g := range a.RuntimeGaps(bob, h) {
		if g.Name == "probe" {
			t.Errorf("a passing probe leaves no gap: %q", g.Line)
		}
	}
}

// The scratch persona the probe launches as renders gates under its own
// name. If it collided with a real PID the probe would re-render that
// persona's wall from a canary deny — disarming the operator's own gates for
// as long as the session lives.
func TestProbeRefusesToOverwriteALivePersonasGates(t *testing.T) {
	t.Parallel()
	a, bob := probeParityApp(t)
	persona := probeAgentName("bob")
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + persona + "\ndeny: [Bash(git push:*)]\n---\nYou are a real lane.\n"
	if err := os.WriteFile(filepath.Join(a.AgentsDir, persona+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := a.RuntimeProbe(bob, Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}, ProbeOpts{})
	if err == nil || !strings.Contains(err.Error(), "would overwrite its wall") {
		t.Fatalf("the probe must refuse a name collision with a real PID: %v", err)
	}
}

// herdr missing is a refusal, not three-of-four. Observable 4 is the one a
// probe cannot fake, and a probe that reported PASS on three would put a
// realized mark on a runtime dispatch is blind on.
func TestProbeRefusesWithoutHerdr(t *testing.T) {
	t.Parallel()
	a, bob := probeParityApp(t)
	rec, err := a.RuntimeProbe(bob, Herdr{Bin: filepath.Join(t.TempDir(), "definitely-not-herdr")}, ProbeOpts{})
	if err == nil || !strings.Contains(err.Error(), "herdr") {
		t.Fatalf("no herdr, no probe: %v %v", rec, err)
	}
	if rec != nil {
		t.Error("a probe that could not run must write no record — an absent record is the honest state")
	}
	if _, statErr := os.Stat(a.ProbeRecordPath("bob")); statErr == nil {
		t.Error("a refused probe left a record behind")
	}
}

// L3 is a git hook, not a PATH lookup, so it survives the assumed-until-
// probed verdict exactly as it survives `gate_shell: false` — and the line
// it prints has to name the RIGHT reason L1 is not carrying the gate. A
// template runtime that never said `gate_shell: false` must not be told it
// did: that sends the operator to a key their yaml does not set instead of
// to the probe that would actually fix it.
func TestL3StillRecoversGitPushOnAnUnprobedTemplateRuntime(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	a, bob := probeParityApp(t)
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	dev := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(rm -rf /)\n---\nYou are dev.\n")
	p := a.CheckParityIn(dev, bob, CageShims, TierStrong, repo)

	got := p.Realized["Bash(git push:*)"].Detail
	if !strings.Contains(got, "L3 pre-push hook") {
		t.Fatalf("L3 does not depend on the shim's PATH race and must still count: %q / %+v", got, p.Unrealized)
	}
	if !strings.Contains(got, "posse runtime probe bob") {
		t.Errorf("the L3-only line must name the probe as the reason L1 is not counted: %q", got)
	}
	if strings.Contains(got, "gate_shell: false") {
		t.Errorf("bob never declared gate_shell: false — naming that key sends the operator to a fix that is not theirs: %q", got)
	}
	// And the gate that L3 cannot reach stays assumed, or the recovery
	// would be reading as a wall for every shell verb.
	if p.Realized["Bash(rm -rf /)"].Detail != "" {
		t.Errorf("L3 recovers git, not every shell verb: %q", p.Realized["Bash(rm -rf /)"].Detail)
	}
}
