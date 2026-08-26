package rhq

// ADR 0013 §1–2: the declared half of the dispatch contract, and the grid
// that prints it. What is pinned here is the DIRECTION each unknown falls
// in — an undeclared runtime must read typed/untrusted/uncounted/unmapped
// and say so out loud — plus the one thing a typo must never do, which is
// look like a declaration.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	return a
}

func writeRuntime(t *testing.T, a *App, name, body string) *Runtime {
	t.Helper()
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime(name)
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

// A template-only yaml with nothing but command: is dispatchable and noisy.
// Every default it lands on is the expensive-to-be-wrong-about direction.
func TestUnknownRuntimeIsNoisyNotSilent(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "mycli", "command: mycli --sys {file}\n")

	if rt.PromptMode() != PromptTyped {
		t.Errorf("undeclared prompt must be typed, not argv: %q", rt.PromptMode())
	}
	if rt.Wait() != DefaultStartupWait {
		t.Errorf("undeclared startup_wait must be the default: %s", rt.Wait())
	}
	if rt.RecordTrust() != RecordUntrusted {
		t.Errorf("undeclared record must be untrusted: %q", rt.RecordTrust())
	}
	if rt.Counted() {
		t.Error("a template-only runtime has no cost adapter")
	}
	if len(rt.NativeRules) != 0 || len(rt.Interstitials) != 0 {
		t.Errorf("nothing is declared for an unknown runtime: %+v %+v", rt.NativeRules, rt.Interstitials)
	}

	var b bytes.Buffer
	a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
	out := b.String()
	// The six stages are the grid; a missing one is a named degrade or a
	// named refuse, so both columns have to be on the screen.
	for _, want := range []string{"launch", "promptable", "work", "record", "settle", "account", "missing →", "by "} {
		if !strings.Contains(out, want) {
			t.Errorf("grid is missing %q:\n%s", want, out)
		}
	}
	for _, want := range []string{"typed", "untrusted", "UNCOUNTED", "UNMAPPED", "uncounted_cap_mycli: unset"} {
		if !strings.Contains(out, want) {
			t.Errorf("undeclared runtime must print %q loudly:\n%s", want, out)
		}
	}
	// A silence would be the failure: the grid has to say WHO left each
	// stage undeclared, not just what it fell back to.
	if !strings.Contains(out, "prompt: unset in runtimes/mycli.yaml") ||
		!strings.Contains(out, "record: unset in runtimes/mycli.yaml") {
		t.Errorf("the grid must name the yaml that declared nothing:\n%s", out)
	}
	// herdr unreachable is "unknown", never a confident "no".
	if !strings.Contains(out, "UNKNOWN") {
		t.Errorf("no herdr → unknown recognition, not a verdict:\n%s", out)
	}
}

// Declared keys are read; a typo in one REFUSES rather than being demoted
// to the default. `record: trused` silently reading as untrusted is exactly
// the silence ADR 0013 exists to remove.
func TestDeclaredContractKeys(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "declared", strings.Join([]string{
		"command: declared --sys {file}",
		"prompt: argv",
		"startup_wait: 90s",
		"record: trusted",
		"record_why: measured on 2026-08-24",
		"native_rules: [AGENTS.md, HOUSE.md]",
		"",
	}, "\n"))
	if rt.PromptMode() != PromptArgv {
		t.Errorf("prompt: argv not read: %q", rt.PromptMode())
	}
	if rt.Wait().String() != "1m30s" {
		t.Errorf("startup_wait: 90s not read: %s", rt.Wait())
	}
	if rt.RecordTrust() != RecordTrusted || rt.RecordWhy == "" {
		t.Errorf("record:/record_why: not read: %q %q", rt.Record, rt.RecordWhy)
	}
	if strings.Join(rt.NativeRules, ",") != "AGENTS.md,HOUSE.md" {
		t.Errorf("native_rules: not read: %v", rt.NativeRules)
	}
	var b bytes.Buffer
	a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
	if out := b.String(); !strings.Contains(out, "runtimes/declared.yaml (prompt:)") ||
		!strings.Contains(out, "runtimes/declared.yaml (record:)") {
		t.Errorf("the grid must credit the yaml for what it declared:\n%s", out)
	}

	for _, bad := range []string{"prompt: arvg", "record: trused", "startup_wait: soon"} {
		name := "bad" + strings.Fields(bad)[0]
		if err := os.WriteFile(filepath.Join(a.RuntimesDir(), name+".yaml"),
			[]byte("command: x\n"+bad+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := a.LoadRuntime(name); err == nil {
			t.Errorf("%q must refuse, not fall back to the default", bad)
		}
	}
}

// The built-ins carry the ADR's own table, and one line of it is load-
// bearing: grok and codex differ ONLY because a dispatched grok session was
// measured to close a bead and three dispatched codex sessions were
// measured not to. If this test ever has to change, a measurement changed.
func TestBuiltinContractDeclarations(t *testing.T) {
	a := checkApp(t)
	for _, c := range []struct {
		name, prompt, record string
		counted              bool
	}{
		{"claude", PromptTyped, RecordTrusted, true},
		{"codex", PromptArgv, RecordUntrusted, false},
		{"grok", PromptArgv, RecordTrusted, false},
	} {
		rt, err := a.LoadRuntime(c.name)
		if err != nil {
			t.Fatal(err)
		}
		if rt.PromptMode() != c.prompt {
			t.Errorf("%s prompt: %q want %q", c.name, rt.PromptMode(), c.prompt)
		}
		if rt.RecordTrust() != c.record {
			t.Errorf("%s record: %q want %q", c.name, rt.RecordTrust(), c.record)
		}
		if rt.Counted() != c.counted {
			t.Errorf("%s counted: %v want %v", c.name, rt.Counted(), c.counted)
		}
		if c.record == RecordTrusted && rt.RecordWhy == "" {
			t.Errorf("%s is trusted with no measurement named", c.name)
		}
		if len(rt.NativeRules) == 0 {
			t.Errorf("%s declares no native rulebooks; all three CLIs have them", c.name)
		}
	}
	// ADR 0013 §2: argv delivery was ASSUMED until ranger-base-cl7 probed
	// it, and the probe held on both non-claude runtimes on 2026-08-25
	// (docs/adr/0013-argv-prompt-probe.md). The rule the pre-probe version
	// of this test enforced — nothing declares argv on a guess — is now
	// enforced from the other side: a runtime that declares argv must have
	// a measurement behind it, and a StartupWait on an argv runtime is a
	// number for a ladder it does not use.
	for _, n := range []string{"codex", "grok"} {
		rt, _ := a.LoadRuntime(n)
		if rt.StartupWait != 0 {
			t.Errorf("%s declares argv AND a startup_wait: — the wait is the typed ladder's patience, and argv does not use it", n)
		}
	}
	// claude is the one that stays typed, and deliberately: it works, and
	// ADR 0013 calls argv there "an allowed later unify", not this bead's.
	if rt, _ := a.LoadRuntime("claude"); rt.Prompt != PromptTyped {
		t.Errorf("claude moved off typed delivery with no measurement asking for it")
	}
}

// Layer 2 of §2: posse NAMES the operator's interstitial keys and writes
// none of them. The dangerous one is codex's, whose default action runs
// `brew upgrade`, and it has to be marked as a launch refuse.
func TestInterstitialsAreNamedNotWritten(t *testing.T) {
	a := checkApp(t)
	grok, _ := a.LoadRuntime("grok")
	codex, _ := a.LoadRuntime("codex")

	var gb, cb bytes.Buffer
	a.RuntimeCheck(grok, Herdr{Bin: "no-such-herdr-binary"}, &gb)
	a.RuntimeCheck(codex, Herdr{Bin: "no-such-herdr-binary"}, &cb)

	for _, want := range []string{"privacy_banner_acked", "~/.grok/config.toml", "[Opt out]", "auto_update", "maximum_version"} {
		if !strings.Contains(gb.String(), want) {
			t.Errorf("grok grid must name %q:\n%s", want, gb.String())
		}
	}
	for _, want := range []string{"dismissed_version", "~/.codex/version.json", "LAUNCH REFUSE", "brew upgrade", "blind-sends Enter"} {
		if !strings.Contains(cb.String(), want) {
			t.Errorf("codex grid must name %q:\n%s", want, cb.String())
		}
	}
	// Never [Opt in]: that is a visibility line, not a dismissal.
	if strings.Contains(gb.String(), "click [Opt in]") {
		t.Error("posse must never suggest opting in to coding-data retention")
	}
	// Probing must not create or touch the operator's files.
	for _, in := range append(GrokInterstitials, CodexInterstitials...) {
		if in.Probe == nil {
			continue
		}
		in.Probe()
	}
	for _, p := range []string{filepath.Join(grokHome(), "config.toml"), filepath.Join(codexHome(), "version.json")} {
		before, err := os.Stat(p)
		if err != nil {
			continue // not on this box; nothing to protect
		}
		grokPrivacyProbe()
		grokAutoUpdateProbe()
		codexUpdateProbe()
		after, _ := os.Stat(p)
		if after == nil || !after.ModTime().Equal(before.ModTime()) {
			t.Errorf("a probe wrote %s — posse documents these keys, it does not set them", p)
		}
	}
}

// The probes answer the question an onboarder actually has, and they answer
// "unknown" rather than "no" when they cannot read the file. Both keys are
// shaped in ways a naive read gets wrong: grok's ack is a TIMESTAMP, not a
// bool, and codex's dismissal is only good until latest_version moves.
func TestInterstitialProbesReadRealShapes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GROK_HOME", filepath.Join(home, ".grok"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	os.MkdirAll(filepath.Join(home, ".grok"), 0o755)
	os.MkdirAll(filepath.Join(home, ".codex"), 0o755)

	if ok, why := grokPrivacyProbe(); ok || !strings.Contains(why, "unset") {
		t.Errorf("absent config must read as not-silenced: %v %q", ok, why)
	}
	if ok, why := codexUpdateProbe(); ok || !strings.Contains(why, "cannot tell") {
		t.Errorf("absent version.json must read as unknown, not as silenced: %v %q", ok, why)
	}

	cfg := filepath.Join(home, ".grok", "config.toml")
	os.WriteFile(cfg, []byte("[cli]\nauto_update = false\n\n[privacy]\nprivacy_banner_acked = \"2026-08-24T21:35:58Z\"\n"), 0o644)
	if ok, why := grokPrivacyProbe(); !ok {
		t.Errorf("an RFC3339 ack is an ack, not a false: %q", why)
	}
	if ok, why := grokAutoUpdateProbe(); !ok || !strings.Contains(why, "false") {
		t.Errorf("auto_update = false is the pin holding: %v %q", ok, why)
	}

	vj := filepath.Join(home, ".codex", "version.json")
	os.WriteFile(vj, []byte(`{"latest_version":"0.149.1","dismissed_version":"0.149.1"}`), 0o644)
	if ok, _ := codexUpdateProbe(); !ok {
		t.Error("dismissed == latest is silenced")
	}
	os.WriteFile(vj, []byte(`{"latest_version":"0.150.0","dismissed_version":"0.149.1"}`), 0o644)
	if ok, why := codexUpdateProbe(); ok || !strings.Contains(why, "the menu is back") {
		t.Errorf("a dismissal expires when latest_version moves: %v %q", ok, why)
	}
}

// rangerhq-y7jr: --no-auto-update is real (hidden from --help, accepted) but
// per-session. Putting it on GrokFleetFlags would leave the operator's
// interactive grok and the shared leader unpinned. The config pin covers
// every entry point; the launch line must not pretend it does.
func TestGrokFleetFlagsDoNotCarryPerSessionUpdateKill(t *testing.T) {
	if strings.Contains(GrokFleetFlags, "--no-auto-update") {
		t.Fatal("GrokFleetFlags must not carry --no-auto-update; it is per-session and would leave the shared leader unpinned (rangerhq-y7jr)")
	}
	if GrokFleetFlags != `--permission-mode auto` {
		t.Errorf("GrokFleetFlags drifted: %q", GrokFleetFlags)
	}
}

// ranger-base-arm: a runtime that ignores `tier:` has to SAY so in the
// grid. Until this landed, `tier: strong` on codex was inert — no Models
// map at all — and the only way to find that out was to read runtime.go.
// Three readings are pinned here, because they are three different facts
// to an operator choosing a tier:
//
//   - fully mapped (claude, and codex since this bead): the ids, and the
//     flag they render with, so nobody diffs two runtimes to learn which
//     one honours the key;
//   - fully unmapped (grok): UNMAPPED, "ignores tier: entirely", and the
//     yaml key that would change it;
//   - PARTIAL: the mapped tiers AND the unmapped ones. A partial map shown
//     as a list of what is mapped reads as complete, which is the silence.
func TestTierLineNamesWhatTheRuntimeIgnores(t *testing.T) {
	a := checkApp(t)
	h := Herdr{Bin: "no-such-herdr-binary"}

	// The tier line is read unwrapped: the grid wraps at word boundaries
	// (gridWidth), so a substring assertion on the rendered screen would be
	// testing the wrap, not the sentence. That the line reaches the screen
	// at all is asserted separately, below.
	grid := func(name string) string {
		t.Helper()
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		a.RuntimeCheck(rt, h, &b)
		if !strings.Contains(b.String(), "tier ") {
			t.Errorf("the grid must carry a tier row:\n%s", b.String())
		}
		return a.tierLine(rt)
	}

	// codex: mapped, and the grid says so with the ids and the flag.
	out := grid("codex")
	for _, want := range []string{"strong=gpt-5.6-sol", "standard=gpt-5.6-sol", "fast=gpt-5.6-luna", "-c model=%s"} {
		if !strings.Contains(out, want) {
			t.Errorf("codex tier line must carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "UNMAPPED") {
		t.Errorf("codex maps every tier; nothing may read UNMAPPED:\n%s", out)
	}

	// grok: unmapped, and loud about it — including WHERE the mapping would
	// have to go. For a built-in that is runtime.go and NOT a yaml: naming
	// runtimes/grok.yaml here would send an operator to a file LoadRuntime
	// never stats for a built-in, which is a remedy that silently does
	// nothing — the exact shape of bug this line exists to prevent.
	out = grid("grok")
	for _, want := range []string{"UNMAPPED", "ignores tier: entirely", "grok/default", "BUILT-IN", "runtime.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("grok tier line must carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Declare model_<tier>:") {
		t.Errorf("a yaml cannot override a built-in; the grid may not prescribe one:\n%s", out)
	}

	// Partial: only model_standard: declared. fast falls back to standard,
	// so the honest reading is "standard and fast render; strong does not".
	writeRuntime(t, a, "halfcli", "command: halfcli {model} --sys {file}\nmodel_standard: mid\n")
	out = grid("halfcli")
	for _, want := range []string{"standard=mid", "fast=mid", "UNMAPPED: strong", "halfcli/default", "Declare model_<tier>:"} {
		if !strings.Contains(out, want) {
			t.Errorf("partial tier line must carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "strong=") {
		t.Errorf("strong is unmapped on halfcli; the grid may not show it as mapped:\n%s", out)
	}
}
