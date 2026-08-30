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
	if rt.CostRead() || rt.CostPriced() || rt.CostReading() != "" {
		t.Error("a template-only runtime has no cost adapter: nothing reads it and nothing prices it")
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
		// read/priced are the account stage's two facts since
		// ranger-base-0lg6, and they come apart on codex: its rollout
		// scanner counts turns and tokens and prices none of them.
		read, priced bool
	}{
		{"claude", PromptTyped, RecordTrusted, true, true},
		{"codex", PromptArgv, RecordUntrusted, true, false},
		{"grok", PromptArgv, RecordTrusted, true, true},
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
		if rt.CostRead() != c.read {
			t.Errorf("%s cost read: %v want %v", c.name, rt.CostRead(), c.read)
		}
		if rt.CostPriced() != c.priced {
			t.Errorf("%s cost priced: %v want %v", c.name, rt.CostPriced(), c.priced)
		}
		if (rt.CostReading() != "") != c.read {
			t.Errorf("%s reading %q disagrees with read=%v — a reading that names nothing, or nothing naming a reading", c.name, rt.CostReading(), c.read)
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
	// ranger-base-poj5 adds the durable half: the grid has to name the key
	// that actually silences the menu and the target that asserts it, or an
	// operator reading this reaches for the one-release dismissal instead.
	for _, want := range []string{"dismissed_version", "~/.codex/version.json", "LAUNCH REFUSE", "brew upgrade", "blind-sends Enter",
		"check_for_update_on_startup", "etc/codex/version-pin.toml", "verify-codex-pin"} {
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

	if sil := grokPrivacyProbe(); sil.Silenced || sil.Unknown || !strings.Contains(sil.Why, "unset") {
		t.Errorf("absent config must read as not-silenced: %+v", sil)
	}
	// UNKNOWN and NOT-SILENCED are two readings, not one, and since
	// ranger-base-9r33 the difference decides a launch: DangerUnsilenced
	// refuses on the second and never on the first. A probe that returned a
	// bare "no" here would wall every codex dispatch on a box posse could
	// not read a file on.
	if sil := codexUpdateProbe(); !sil.Unknown || sil.Silenced || !strings.Contains(sil.Why, "cannot tell") {
		t.Errorf("absent version.json must read as unknown, not as silenced or as no: %+v", sil)
	}

	cfg := filepath.Join(home, ".grok", "config.toml")
	os.WriteFile(cfg, []byte("[cli]\nauto_update = false\n\n[privacy]\nprivacy_banner_acked = \"2026-08-24T21:35:58Z\"\n"), 0o644)
	if sil := grokPrivacyProbe(); !sil.Silenced {
		t.Errorf("an RFC3339 ack is an ack, not a false: %+v", sil)
	}
	if sil := grokAutoUpdateProbe(); !sil.Silenced || !strings.Contains(sil.Why, "false") {
		t.Errorf("auto_update = false is the pin holding: %+v", sil)
	}

	vj := filepath.Join(home, ".codex", "version.json")
	os.WriteFile(vj, []byte(`{"latest_version":"0.149.1","dismissed_version":"0.149.1"}`), 0o644)
	if sil := codexUpdateProbe(); !sil.Silenced {
		t.Errorf("dismissed == latest is silenced: %+v", sil)
	}
	os.WriteFile(vj, []byte(`{"latest_version":"0.150.0","dismissed_version":"0.149.1"}`), 0o644)
	if sil := codexUpdateProbe(); sil.Silenced || sil.Unknown || !strings.Contains(sil.Why, "the menu is back") {
		t.Errorf("a dismissal expires when latest_version moves, and that is a READING, not an unknown: %+v", sil)
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
//   - fully mapped (claude, and codex since this bead; grok since
//     rangerhq-jp6): the ids, and the flag they render with, so nobody
//     diffs two runtimes to learn which one honours the key;
//   - fully unmapped: UNMAPPED, "ignores tier: entirely", and the yaml key
//     that would change it — plus the BUILT-IN variant of that remedy,
//     which says runtime.go instead. No built-in is unmapped any more, so
//     that arm is driven by a Runtime value rather than by whichever
//     built-in happened to carry no map that week: it is tierLine and
//     tierFix under test, and the fixture only has to reach them.
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

	// grok: mapped since rangerhq-jp6, so the grid prints the ids and the
	// -m flag they render with, and must NOT still say UNMAPPED.
	out = grid("grok")
	for _, want := range []string{"strong=grok-4.6", "standard=grok-4.6", "fast=grok-4.5", "-m %s"} {
		if !strings.Contains(out, want) {
			t.Errorf("grok tier line must carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "UNMAPPED") || strings.Contains(out, "grok/default") {
		t.Errorf("grok maps every tier; nothing may read UNMAPPED:\n%s", out)
	}

	// Fully unmapped, and loud about it — including WHERE the mapping would
	// have to go. For a BUILT-IN that is runtime.go and NOT a yaml: naming
	// runtimes/<name>.yaml here would send an operator to a file
	// LoadRuntime never stats for a built-in, which is a remedy that
	// silently does nothing — the exact shape of bug this line exists to
	// prevent. Driven off a Runtime value because no built-in is unmapped
	// today; the two flags it sets (Builtin, no Models) are exactly the two
	// tierLine and tierFix branch on.
	out = a.tierLine(&Runtime{Name: "blankcli", Builtin: true, ModelFlag: "-m %s"})
	for _, want := range []string{"UNMAPPED", "ignores tier: entirely", "blankcli/default", "BUILT-IN", "runtime.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("an unmapped built-in tier line must carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Declare model_<tier>:") {
		t.Errorf("a yaml cannot override a built-in; the grid may not prescribe one:\n%s", out)
	}
	// The non-built-in half of the same remedy, so the branch is pinned
	// both ways: a DECLARED runtime with no map is sent to its yaml.
	out = a.tierLine(&Runtime{Name: "blankyaml", ModelFlag: "-m %s"})
	if !strings.Contains(out, "Declare model_<tier>:") || strings.Contains(out, "BUILT-IN") {
		t.Errorf("an unmapped declared runtime must be sent to its yaml:\n%s", out)
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

// ─── ADR 0017 §1/§4: the dimensions the six stages never carried ─────────
//
// skills surface, egress, cage credential, project config, sandbox/gate
// shell. Each is a Runtime field a launch changes behaviour on, and none of
// them had a row — so onboarding a fourth runtime meant reading runtime.go
// for facts the code already knew (ranger-base-qm6e measured the skills
// half: no row at all, only a {skills} placeholder inside the echoed
// template).
//
// Everything here is asserted against the RENDERED row, scoped to that row,
// for the reason ranger-base-qm6e names: the grid is a page of prose in
// which almost every word appears somewhere, so an unscoped
// `strings.Contains` over the screen answers about whichever row said it
// first. gridRow is the same shape-based reader gridStages uses
// (runtimegrid_qa_test.go), one row deep — and adding these rows proved the
// point on a neighbour, which TestRuntimeCheckPrintsTheProbeRowBothWays now
// scopes the same way.

// gridRow returns one row of the grid — its value line plus every
// continuation under it — flattened to single spaces, so an assertion reads
// the sentence and not the wrap.
func gridRow(t *testing.T, out, label string) string {
	t.Helper()
	var got []string
	in := false
	for _, line := range strings.Split(out, "\n") {
		isRow := len(line) >= 15 && strings.HasPrefix(line, "  ") && line[13] == ' ' && line[14] != ' '
		lab := ""
		if isRow {
			lab = strings.TrimSpace(line[2:13])
		}
		switch {
		case isRow && lab == label:
			in = true
			got = append(got, strings.TrimSpace(line[13:]))
		case !in:
		case isRow && lab != "", strings.TrimSpace(line) == "":
			in = false
		default:
			got = append(got, strings.TrimSpace(line))
		}
	}
	if len(got) == 0 {
		t.Fatalf("the grid has no %q row:\n%s", label, out)
	}
	return strings.Join(strings.Fields(strings.Join(got, " ")), " ")
}

// dimensionRows is the order they are drawn in, and the list every test
// below walks: a row that stops being drawn must red something.
var dimensionRows = []string{"skills", "egress", "cage_cred", "project_cfg", "sandbox"}

// Drawn as ROWS — not as a sentence inside somebody else's row — on a
// template-only yaml and on all three built-ins, after the six stages.
func TestGridDrawsTheDeclaredDimensionRows(t *testing.T) {
	a := checkApp(t)
	h := Herdr{Bin: "no-such-herdr-binary"}
	runtimes := []*Runtime{writeRuntime(t, a, "mycli", "command: mycli --sys {file}\n")}
	for _, n := range []string{"claude", "codex", "grok"} {
		rt, err := a.LoadRuntime(n)
		if err != nil {
			t.Fatal(err)
		}
		runtimes = append(runtimes, rt)
	}
	for _, rt := range runtimes {
		var b bytes.Buffer
		a.RuntimeCheck(rt, h, &b)
		out := b.String()
		rows := gridStages(out)
		// After the six stages, in the ADR's own order, and each with the
		// provenance pair every other row of this grid carries.
		var idx []int
		for _, want := range dimensionRows {
			at := -1
			for i, r := range rows {
				if r == want && i >= 6 {
					at = i
					break
				}
			}
			if at < 0 {
				t.Fatalf("%s: no %q ROW after the six stages — the grid drew %v\n%s", rt.Name, want, rows, out)
			}
			idx = append(idx, at)
			row := gridRow(t, out, want)
			if !strings.Contains(row, "by ") || !strings.Contains(row, "missing → ") {
				t.Errorf("%s: the %s row is missing its declared-by / missing-→ pair, which is what makes it a grid row rather than a remark: %s", rt.Name, want, row)
			}
		}
		for i := 1; i < len(idx); i++ {
			if idx[i] < idx[i-1] {
				t.Errorf("%s: dimension rows are out of order (%v at %v)", rt.Name, dimensionRows, idx)
			}
		}
	}
}

// A bare template-only yaml — the runtime this command exists for — prints a
// LOUD line on every one of the five. ADR 0017 §2's vocabulary is the thing
// under test as much as the values: an absence must read as UNDECLARED /
// UNDECIDED / none, and DECLARED DIFFERENCE may not appear anywhere, because
// nothing here was measured to differ.
func TestUndeclaredDimensionsAreLoudNotBlank(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "mycli", "command: mycli --sys {file}\n")
	var b bytes.Buffer
	a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
	out := b.String()

	for _, c := range []struct{ row, want, unwanted string }{
		// No surface at all is the third arm of parity.go's split, and the
		// expensive one: every `skills:` PID refuses to launch here.
		{"skills", "NO SURFACE — UNDECLARED: neither skills_flag: nor skills_cwd:", "cwd-discovery"},
		{"skills", "skills_cwd: true (the CLI walks .agents/skills", "--plugin-dir '"},
		{"egress", "UNDECLARED — no host of this runtime's own is known", "always added to a caged PID's"},
		{"cage_cred", "UNDECIDED — cage: container refuses on this runtime", "the env NAME a containerised session authenticates with"},
		{"project_cfg", "none — no repo→box config surface declared", "read from the SESSION DIR at launch"},
		{"sandbox", "posse's seatbelt wraps this runtime — self_sandbox: unset", "macOS refuses to nest seatbelts"},
		{"sandbox", "gate_shell: on — the launch points SHELL/GROK_SHELL at the gate shell", "every Bash(...) deny is UNREALIZED here"},
	} {
		row := gridRow(t, out, c.row)
		if !strings.Contains(row, c.want) {
			t.Errorf("the %s row of an undeclared runtime must say %q:\n%s", c.row, c.want, row)
		}
		if strings.Contains(row, c.unwanted) {
			t.Errorf("the %s row of an undeclared runtime must NOT say %q — that is the declared reading:\n%s", c.row, c.unwanted, row)
		}
	}
	// Every one of them must credit the yaml that declared nothing, by the
	// key an onboarder would type. A row that fell back silently is the
	// whole failure class (ADR 0013's declaredBy).
	for _, key := range []string{"egress", "cage_cred", "project_config", "self_sandbox", "gate_shell"} {
		if !strings.Contains(out, key+": unset in runtimes/mycli.yaml") {
			t.Errorf("nothing on the screen says %s: was left unset in the yaml:\n%s", key, out)
		}
	}
	// §2's presentation rule: an UNKNOWN and a measured difference may never
	// be spelled the same way, and on a runtime nobody measured there is no
	// difference to declare.
	for _, r := range dimensionRows {
		if row := gridRow(t, out, r); strings.Contains(row, "DECLARED DIFFERENCE") {
			t.Errorf("the %s row calls an UNMEASURED runtime a declared difference:\n%s", r, row)
		}
	}
}

// The other direction: a yaml that declares all five is read, and each row
// credits the key. `egress:` is written in BLOCK form on purpose — the value
// after the colon is empty there, so a provenance line built on YamlGet
// credits a built-in default the yaml had in fact overridden (declaredByList).
func TestDeclaredDimensionsAreReadAndCredited(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "fullcli", strings.Join([]string{
		"command: fullcli --sys {file} {skills}",
		`skills_flag: "--skills=%s"`,
		"egress:",
		"  - api.fullcli.example",
		"  - auth.fullcli.example",
		"cage_cred: FULLCLI_TOKEN",
		"project_config: .fullcli/settings.json",
		"project_config_keys: [hooks, mcpServers]",
		"self_sandbox: true",
		"gate_shell: false",
		"",
	}, "\n"))
	var b bytes.Buffer
	a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
	out := b.String()

	for _, c := range []struct {
		row      string
		want     []string
		unwanted []string
	}{
		{"skills",
			// The flag AND what the tree it points at is made of: a runtime
			// that installs the same tree and follows no symlink surfaces
			// zero skills while every screen says "flag" (ranger-base-65rc).
			[]string{"flag — --skills=", "SYMLINK into RHQ_HOME/skills", "runtimes/fullcli.yaml (skills_flag:)"},
			[]string{"NO SURFACE", "cwd-discovery"}},
		{"egress",
			[]string{"api.fullcli.example auth.fullcli.example", "runtimes/fullcli.yaml (egress:)"},
			[]string{"UNDECLARED", "egress: unset in runtimes/fullcli.yaml"}},
		{"cage_cred",
			[]string{"FULLCLI_TOKEN — the env NAME", "runtimes/fullcli.yaml (cage_cred:)"},
			[]string{"UNDECIDED", "cage.go cageCredential"}},
		{"project_cfg",
			[]string{".fullcli/settings.json — read from the SESSION DIR", "narrowed to top-level JSON keys: hooks, mcpServers",
				"runtimes/fullcli.yaml (project_config:)", "runtimes/fullcli.yaml (project_config_keys:)", "trust_project_config: true"},
			[]string{"none — no repo→box config surface", "the WHOLE FILE is the predicate"}},
		{"sandbox",
			[]string{"self_sandbox — a DECLARED DIFFERENCE, not a failure", "gate_shell: false — a DECLARED DIFFERENCE",
				"every Bash(...) deny is UNREALIZED here but `git push`",
				"runtimes/fullcli.yaml (self_sandbox:)", "runtimes/fullcli.yaml (gate_shell:)"},
			[]string{"posse's seatbelt wraps this runtime", "gate_shell: on"}},
	} {
		row := gridRow(t, out, c.row)
		for _, w := range c.want {
			if !strings.Contains(row, w) {
				t.Errorf("the declared %s row must carry %q:\n%s", c.row, w, row)
			}
		}
		for _, u := range c.unwanted {
			if strings.Contains(row, u) {
				t.Errorf("the declared %s row still carries the undeclared reading %q:\n%s", c.row, u, row)
			}
		}
	}
	// And the values reached the struct, not only the screen — the p84 shape
	// in reverse (a row that renders a fact no loader read).
	if strings.Join(rt.Egress, ",") != "api.fullcli.example,auth.fullcli.example" ||
		rt.CageCred != "FULLCLI_TOKEN" || !rt.SelfSandbox || !rt.NoGateShell ||
		strings.Join(rt.ProjectConfig, ",") != ".fullcli/settings.json" ||
		strings.Join(rt.ProjectConfigKeys, ",") != "hooks,mcpServers" {
		t.Errorf("the grid printed declarations LoadRuntime did not read: %+v", rt)
	}
}

// The built-ins are where the vocabulary earns its keep. codex sandboxes
// itself, which is a first-class runtime and not a broken one: ADR 0017 §2
// says nothing may render a DECLARED DIFFERENCE as a failure, and the row
// that would have to get this wrong is the one that also carries the word
// UNDECLARED for the runtime next door.
func TestBuiltinDimensionRowsSpeakTheVerdictVocabulary(t *testing.T) {
	a := checkApp(t)
	h := Herdr{Bin: "no-such-herdr-binary"}
	grid := func(name string) string {
		t.Helper()
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		a.RuntimeCheck(rt, h, &b)
		return b.String()
	}

	// codex: self_sandbox is a DECLARED DIFFERENCE, and the row may not
	// read as an absence or as a fault.
	codex := gridRow(t, grid("codex"), "sandbox")
	for _, w := range []string{"a DECLARED DIFFERENCE, not a failure", "macOS refuses to nest seatbelts", "its own sandbox is what enforces Edit/Write here"} {
		if !strings.Contains(codex, w) {
			t.Errorf("codex's sandbox row must carry %q:\n%s", w, codex)
		}
	}
	for _, u := range []string{"UNDECLARED", "UNDECIDED", "posse's seatbelt wraps this runtime"} {
		if strings.Contains(codex, u) {
			t.Errorf("codex's self_sandbox is measured, not missing; the row may not say %q:\n%s", u, codex)
		}
	}
	// claude: the flag surface, spelled as the flag the CLI reads.
	claude := gridRow(t, grid("claude"), "skills")
	for _, w := range []string{"flag — --plugin-dir ", "SYMLINK into RHQ_HOME/skills"} {
		if !strings.Contains(claude, w) {
			t.Errorf("claude's skills row must carry %q:\n%s", w, claude)
		}
	}
	// codex and grok: the cwd surface, whose binding is the links and not
	// anything on the line — the fact a "flag" row would hide.
	for _, n := range []string{"codex", "grok"} {
		row := gridRow(t, grid(n), "skills")
		for _, w := range []string{"cwd-discovery — .agents/skills under the session dir", "the LINKS are the binding", "ADDITIVE"} {
			if !strings.Contains(row, w) {
				t.Errorf("%s's skills row must carry %q:\n%s", n, w, row)
			}
		}
		if strings.Contains(row, "NO SURFACE") || strings.Contains(row, "--plugin-dir") {
			t.Errorf("%s discovers skills from the cwd; the row may not show a flag surface:\n%s", n, row)
		}
	}
	// codex and grok keep plain auth.json files, so their container
	// credential is UNDECIDED — the state that refuses `cage: container`
	// rather than starting an unauthenticated session (rangerhq-kiz).
	if row := gridRow(t, grid("claude"), "cage_cred"); !strings.Contains(row, "CLAUDE_CODE_OAUTH_TOKEN") ||
		!strings.Contains(row, "built-in table (cage.go cageCredential)") {
		t.Errorf("claude's cage_cred row must name the var and the table it came from:\n%s", row)
	}
	for _, n := range []string{"codex", "grok"} {
		if row := gridRow(t, grid(n), "cage_cred"); !strings.Contains(row, "UNDECIDED") {
			t.Errorf("%s has no decided container credential; the row must say so:\n%s", n, row)
		}
	}
	// grok declares no project config surface and codex declares one with no
	// key narrowing — the two arms claude's keyed row does not reach.
	if row := gridRow(t, grid("grok"), "project_cfg"); !strings.Contains(row, "none — no repo→box config surface") {
		t.Errorf("grok declares no project config surface:\n%s", row)
	}
	if row := gridRow(t, grid("codex"), "project_cfg"); !strings.Contains(row, ".codex/config.toml") ||
		!strings.Contains(row, "the WHOLE FILE is the predicate") {
		t.Errorf("codex's whole-file predicate must be on the row:\n%s", row)
	}
	if row := gridRow(t, grid("claude"), "project_cfg"); !strings.Contains(row, "narrowed to top-level JSON keys: hooks, mcpServers") {
		t.Errorf("claude's project config is key-narrowed; the row must name the keys:\n%s", row)
	}
}

// ranger-base-poj5: the durable silence, and the three ways a naive read of
// it would be wrong.
//
// codex has no version ceiling to set — required_maximum_version and friends
// appear zero times in the 0.150.1 binary against a positive control — so the
// fleet pin is the Homebrew cask plus check_for_update_on_startup = false,
// which stops the "1. Update now" menu being drawn at all. That is a value,
// not a presence: the same key at TRUE is the menu armed. It also outranks
// version.json, because with the startup check off what that file says is not
// a reading about any screen the operator will meet — which is the case this
// pins, since on 2026-08-30 version.json alone walled every codex dispatch
// (tap 0.151.0 against a dismissal of 0.149.1) on a box where the menu could
// no longer draw.
func TestCodexUpdateProbePrefersTheDurableSilence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".codex", "config.toml")
	vj := filepath.Join(home, ".codex", "version.json")

	// A version.json that is DUE a menu, held constant across every arm: an
	// arm that read as silenced because the dismissal happened to be current
	// would prove nothing about the key under test.
	if err := os.WriteFile(vj, []byte(`{"latest_version":"0.151.0","dismissed_version":"0.149.1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if sil := codexUpdateProbe(); sil.Silenced {
		t.Fatalf("fixture is not due a menu — the arms below would be vacuous: %+v", sil)
	}

	for _, tc := range []struct {
		name     string
		body     string
		silenced bool
	}{
		{"pin applied", "check_for_update_on_startup = false\n", true},
		{"pin applied among other keys", "model = \"gpt-5.6-sol\"\ncheck_for_update_on_startup = false\n\n[tui]\ntheme = \"dark\"\n", true},
		// The wrong arms. Each one is a config.toml that MENTIONS the key
		// and must still read as armed.
		{"key set true", "check_for_update_on_startup = true\n", false},
		{"key commented out", "# check_for_update_on_startup = false\n", false},
		{"an unrelated key", "unrelated_bogus_key_xyz = false\n", false},
		{"no config at all", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.Remove(cfg)
			if tc.body != "" {
				if err := os.WriteFile(cfg, []byte(tc.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			sil := codexUpdateProbe()
			if sil.Silenced != tc.silenced {
				t.Fatalf("silenced=%v, want %v: %+v", sil.Silenced, tc.silenced, sil)
			}
			if tc.silenced && !strings.Contains(sil.Why, "check_for_update_on_startup = false") {
				t.Errorf("the reading must name the key it read: %+v", sil)
			}
			if !tc.silenced && !strings.Contains(sil.Why, "the menu is back") {
				t.Errorf("an armed box must fall through to the version.json reading: %+v", sil)
			}
		})
	}
}
