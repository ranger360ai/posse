//go:build posse_arm3

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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// probeFixtureExe is the CLI every fixture in this file asserts is NOT
// installed here, and its spelling is the deliverable of ranger-base-mis0i.
//
// The reserved form is `posse-<bead>-no-such-exe` — a word no package
// manager will ever ship — and it is a rule rather than a taste because the
// short, plausible alternative has already cost a red suite. ProbeState's
// drift check resolves rt.Exe() on the REAL PATH (runtimeprobe.go, the
// like-for-like branch), so "a runtime that does not exist here" is not a
// property of the fixture: it is a bet that the name stays free on every box
// the suite runs on. `bob` held that bet from ADR 0017 until an npm install
// of an unrelated tool called bobshell put a real `bob` on this box
// (2026-09-28 14:31), and five arm3 tests went red at a HEAD nobody had
// touched. ranger-base-ymmiv renamed the placeholder to `carol`, which is
// the same bet at longer odds — alice/bob/carol are exactly the names a
// package manager ships.
//
// Only the EXE is reserved. The runtime NAME stays `carol`, because nothing
// ever resolves a name: it is a filename under runtimes/ and a word in the
// `posse runtime probe <name>` remedy, and the assertions below read better
// for it. internal/treepins/fixtureexe_qa_test.go is what keeps this true
// for the whole test corpus rather than for the file somebody remembered.
const probeFixtureExe = "posse-mis0i-no-such-exe"

// probeParityApp is an App with a template-only runtime declared in yaml and
// a state dir of its own. The Builtin guard below is ranger-base-ymmiv's:
// when ADR 0060 made bob a fourth BUILT-IN, every fixture in this file would
// otherwise have become a template-only assertion about a runtime that loads
// built-in, and that guard is what said so first.
func probeParityApp(t *testing.T) (*App, *Runtime) {
	t.Helper()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "carol.yaml"), []byte("command: "+probeFixtureExe+" --pid {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime("carol")
	if err != nil {
		t.Fatal(err)
	}
	if rt.Builtin {
		t.Fatal("carol must load as template-only, or this file tests the wrong thing")
	}
	return a, rt
}

// writeProbe stores a record for carol with every observable green (or one
// red), so the tests below can move the runtime between assumed and measured
// without a CLI.
func writeProbe(t *testing.T, a *App, pass bool) {
	t.Helper()
	obs := evalProbe(passingReading("/tmp/gates/bin"))
	if !pass {
		obs[0] = ProbeObservable{1, "shim-precedence", false, "command -v uname → /usr/bin/uname"}
	}
	rec := &ProbeRecord{
		Runtime: "carol", CLIPath: "/usr/local/bin/" + probeFixtureExe, LauncherPath: "/usr/local/bin/" + probeFixtureExe,
		Version: "carol 1.2.3",
		Date:    time.Now().UTC(), PosseVersion: Version, Canary: "uname", Observables: obs,
	}
	if err := a.WriteProbeRecord(rec); err != nil {
		t.Fatal(err)
	}
}

func TestTemplateBashDenyIsAssumedUntilProbed(t *testing.T) {
	t.Parallel()
	a, carol := probeParityApp(t)
	dev := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(rm -rf /)\n---\nYou are dev.\n")

	// Unprobed: BOTH shell-verb denies land in Degraded, and neither is
	// Realized. This is the whole change — before it, parity counted them
	// realized on the strength of three behaviours nobody had measured.
	p := a.CheckParity(dev, carol, CageShims, TierStrong)
	if len(p.Unrealized) != 2 {
		t.Fatalf("both Bash denies must be unrealized on an unprobed template runtime: %+v", p)
	}
	for _, rule := range []string{"Bash(git push:*)", "Bash(rm -rf /)"} {
		if p.Realized[rule].Detail != "" {
			t.Errorf("%s must not read as realized before a probe: %q", rule, p.Realized[rule].Detail)
		}
	}
	joined := strings.Join(p.Degraded, "\n")
	for _, want := range []string{"assumed, not measured", "posse runtime probe carol", "no probe record"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the degrade line must carry %q — a refusal that does not name its unlock trains the operator to waive by habit:\n%s", want, joined)
		}
	}

	// A recorded FAILURE is not a record that unlocks anything, and it must
	// say what failed rather than repeating "no probe record".
	writeProbe(t, a, false)
	p = a.CheckParity(dev, carol, CageShims, TierStrong)
	if len(p.Unrealized) != 2 {
		t.Fatalf("a FAILED probe leaves the claim assumed: %+v", p)
	}
	if j := strings.Join(p.Degraded, "\n"); !strings.Contains(j, "FAILED") || !strings.Contains(j, "/usr/bin/uname") {
		t.Errorf("a failed probe must be reported as measured-and-broken, with the observable:\n%s", j)
	}

	// Probed and passing: the claim flips to realized, exactly as it reads
	// on a built-in, and the launch stops degrading.
	writeProbe(t, a, true)
	p = a.CheckParity(dev, carol, CageShims, TierStrong)
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
	a, carol := probeParityApp(t)
	dev := loadTestAgent(t, "---\nname: dev\ndeny: [Bash(rm -rf /)]\n---\nYou are dev.\n")
	if p := a.CheckParity(dev, carol, CageShims, TierFast); !p.NoDegrade {
		t.Errorf("an unprobed template runtime at tier fast must refuse without a waiver: %+v", p)
	}
	writeProbe(t, a, true)
	if p := a.CheckParity(dev, carol, CageShims, TierFast); p.NoDegrade || len(p.Degraded) != 0 {
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
	a, carol := probeParityApp(t)
	var out bytes.Buffer
	a.RuntimeCheck(carol, Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}, &out)
	got := out.String()
	// Scoped to the probe ROW, not to the screen. ASSUMED and MEASURED are
	// ordinary English on a grid that spends most of its width telling an
	// onboarder which facts were measured — the skills row says "declare the
	// one you MEASURED" — so an unscoped Contains here answers about
	// whichever row said the word first, and would have called this runtime
	// probed on the strength of a sentence about skills (ranger-base-bcpa).
	row := gridRow(t, got, "probe")
	for _, want := range []string{"ASSUMED", "posse runtime probe carol"} {
		if !strings.Contains(row, want) {
			t.Errorf("the unprobed grid's probe row must carry %q:\n%s", want, row)
		}
	}
	if strings.Contains(row, "MEASURED") {
		t.Errorf("an unprobed runtime must not print MEASURED:\n%s", row)
	}

	writeProbe(t, a, true)
	out.Reset()
	a.RuntimeCheck(carol, Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}, &out)
	got = out.String()
	row = gridRow(t, got, "probe")
	if !strings.Contains(row, "MEASURED") || strings.Contains(row, "ASSUMED") {
		t.Errorf("a probed runtime prints MEASURED and not ASSUMED:\n%s", row)
	}
	if !strings.Contains(got, a.ProbeRecordPath("carol")) && !strings.Contains(got, AbbrevHome(a.ProbeRecordPath("carol"))) {
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
	a, carol := probeParityApp(t)
	h := Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}
	var found *RuntimeGap
	for _, g := range a.RuntimeGaps(carol, h) {
		if g.Name == "probe" {
			gg := g
			found = &gg
		}
	}
	if found == nil {
		t.Fatalf("the preflight must report the probe gap by name: %+v", a.RuntimeGaps(carol, h))
	}
	if found.Blocking {
		t.Error("an unprobed runtime is a named degrade, not a refusal")
	}
	if !strings.Contains(found.Line, "--allow-degraded") || !strings.Contains(found.Line, "tier fast") {
		t.Errorf("the gap must state the waiver semantics it costs: %q", found.Line)
	}
	writeProbe(t, a, true)
	for _, g := range a.RuntimeGaps(carol, h) {
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
	a, carol := probeParityApp(t)
	persona := probeAgentName("carol")
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + persona + "\ndeny: [Bash(git push:*)]\n---\nYou are a real lane.\n"
	if err := os.WriteFile(filepath.Join(a.AgentsDir, persona+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := a.RuntimeProbe(carol, Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}, ProbeOpts{})
	if err == nil || !strings.Contains(err.Error(), "would overwrite its wall") {
		t.Fatalf("the probe must refuse a name collision with a real PID: %v", err)
	}
}

// herdr missing is a refusal, not three-of-four. Observable 4 is the one a
// probe cannot fake, and a probe that reported PASS on three would put a
// realized mark on a runtime dispatch is blind on.
func TestProbeRefusesWithoutHerdr(t *testing.T) {
	t.Parallel()
	a, carol := probeParityApp(t)
	rec, err := a.RuntimeProbe(carol, Herdr{Bin: filepath.Join(t.TempDir(), "definitely-not-herdr")}, ProbeOpts{})
	if err == nil || !strings.Contains(err.Error(), "herdr") {
		t.Fatalf("no herdr, no probe: %v %v", rec, err)
	}
	if rec != nil {
		t.Error("a probe that could not run must write no record — an absent record is the honest state")
	}
	if _, statErr := os.Stat(a.ProbeRecordPath("carol")); statErr == nil {
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
	a, carol := probeParityApp(t)
	repo := gitTempDir(t)
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	dev := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(rm -rf /)\n---\nYou are dev.\n")
	p := a.CheckParityIn(dev, carol, CageShims, TierStrong, repo)

	got := p.Realized["Bash(git push:*)"].Detail
	if !strings.Contains(got, "L3 pre-push hook") {
		t.Fatalf("L3 does not depend on the shim's PATH race and must still count: %q / %+v", got, p.Unrealized)
	}
	if !strings.Contains(got, "posse runtime probe carol") {
		t.Errorf("the L3-only line must name the probe as the reason L1 is not counted: %q", got)
	}
	if strings.Contains(got, "gate_shell: false") {
		t.Errorf("carol never declared gate_shell: false — naming that key sends the operator to a fix that is not theirs: %q", got)
	}
	// And the gate that L3 cannot reach stays assumed, or the recovery
	// would be reading as a wall for every shell verb.
	if p.Realized["Bash(rm -rf /)"].Detail != "" {
		t.Errorf("L3 recovers git, not every shell verb: %q", p.Realized["Bash(rm -rf /)"].Detail)
	}
}

// ADR 0061 D2 property 2 asked of the PROBE — ranger-base-5jjtn.
//
// `posse runtime probe` is the one surface ADR 0061 D2 property 3 keeps
// RUNNING on a reported runtime: its detection gap is a non-blocking degrade
// precisely so the probe can measure what detection here actually reads. So
// the probe is also the surface most likely to be the one an operator meets
// first — and it kept the DETECTED route's wording for both of its no-label
// failures, because evalProbe is pure and had no declaration in hand.
//
// MEASURED 2026-10-01, the live record at
// ~/.config/posse/state/runtimes/bob/probe.json with `detection: reported`
// declared in runtimes/bob.yaml: observable 4 read "herdr saw no agent in the
// probe pane — agent_not_found … Author a detection manifest", and observable
// 3 read "herdr saw no agent in the probe pane within the timeout
// (startup_wait 45s on bob)". Both are the wrong door twice over: the
// manifest is upstream's to ship and not posse's to write (ADR 0060 D2), and
// the cause was an authority whose watcher had never once started — so the
// reader was sent to author a manifest and to raise a wait, and the one
// command that would have answered (`herdr plugin list`) was on neither line.
//
// Both arms go through NoAgentLine, so the probe cannot drift from the
// launch's own wait: there is one copy of each sentence and this pin reads it
// through the probe's own evaluator rather than restating it.
func TestQAProbeNoLabelNamesTheAuthorityOnAReportedRuntime(t *testing.T) {
	t.Parallel()
	// This arm's own copy of the declaration text. The launch-wait pin that
	// shares the sentence lives in arm 1 (reporteddetection_qa_test.go), and
	// hoisting one const across the build-tag split would buy nothing: what
	// keeps the two pins from drifting is that both read the same FUNCTION,
	// not that they spell one fixture the same way.
	const qaReportedWhy = "herdr-bob plugin MartinLoeper/herdr-bob, installed 2026-09-28"
	rr := ManifestReading{Argv0: "mycli", State: ManifestUnknownAgent, Declared: DetectionReported, Why: qaReportedWhy, Surface: ReportSurfacePresent}

	r := passingReading("/tmp/gates/bin")
	r.AgentKind, r.Detection = "", AgentDetection{}
	r.Manifest, r.Wait = rr, 45*time.Second
	// Observable 3's reason travels in SettleWhy, which Run composes where
	// the runtime is in hand. Driven through probeNoAgentWhy — the production
	// branch itself — rather than composed here: a pin that wrote the sentence
	// for itself would be green over a Run that never took the reported arm
	// (ranger-base-mmvrh).
	bob := &Runtime{Name: "mycli", StartupWait: 45 * time.Second}
	r.Settled, r.SettleWhy = "", probeNoAgentWhy(rr, bob, fmt.Errorf("herdr saw no agent in the probe pane within the timeout"))

	four := obs(t, r, 4)
	if four.OK {
		t.Fatal("no label at all cannot be a passing detection observable")
	}
	for _, want := range []string{qaReportedWhy, "herdr plugin list", "45s"} {
		if !strings.Contains(four.Detail, want) {
			t.Errorf("observable 4 on a reported runtime must carry %q:\n%s", want, four.Detail)
		}
	}
	for _, bad := range []string{"Author a detection manifest", detectionDoc, "agent_not_found"} {
		if strings.Contains(four.Detail, bad) {
			t.Errorf("observable 4 on a reported runtime must not say %q — the manifest is upstream's (ADR 0060 D2) and the absent label is the authority's silence:\n%s", bad, four.Detail)
		}
	}
	if three := obs(t, r, 3); strings.Contains(three.Detail, "startup_wait") || !strings.Contains(three.Detail, "herdr plugin list") {
		t.Errorf("observable 3's reason on a reported runtime must put `herdr plugin list` first and must not offer a larger startup_wait:\n%s", three.Detail)
	}

	// THE CONTROL, and it is the whole value of this pin: a probe that
	// printed the reported sentence over every runtime would satisfy every
	// assertion above. On a runtime herdr DETECTS, "author a manifest" is
	// the correct door and must survive.
	d := passingReading("/tmp/gates/bin")
	d.AgentKind, d.Detection = "", AgentDetection{}
	d.Manifest = ManifestReading{Argv0: "carol", State: ManifestKnown, Declared: DetectionHerdr, Version: "2026.09.28.1"}
	detail := obs(t, d, 4).Detail
	if !strings.Contains(detail, "Author a detection manifest") {
		t.Errorf("on a runtime herdr detects, authoring the manifest IS the remedy and that sentence must survive:\n%s", detail)
	}
	if strings.Contains(detail, "herdr plugin list") || strings.Contains(detail, qaReportedWhy) {
		t.Errorf("a detected runtime's observable must not quote a declaration it does not carry:\n%s", detail)
	}
	// The same control on observable 3's branch, through the same production
	// function: on a detected runtime the raw timeout IS the news, and the
	// runtime's own wait is what bounds it.
	plain := probeNoAgentWhy(d.Manifest, bob, fmt.Errorf("herdr saw no agent in the probe pane within the timeout"))
	if !strings.Contains(plain, "startup_wait 45s on mycli") {
		t.Errorf("on a runtime herdr detects, observable 3's reason must stay the timeout and name the wait it was given:\n%s", plain)
	}
	if strings.Contains(plain, "herdr plugin list") {
		t.Errorf("a detected runtime's observable 3 must not send the reader to the plugin list:\n%s", plain)
	}
}
