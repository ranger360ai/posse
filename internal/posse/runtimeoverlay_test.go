package posse

// ADR 0021: a runtimes/<name>.yaml naming a built-in is a per-key OVERLAY,
// not a template. Mirrors runtimeyamlv2_test.go's shape — the template-only
// validation is the pattern these tests reuse against LoadRuntime's other
// branch.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeOverlay(t *testing.T, a *App, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Absent file: today's behaviour exactly.
func TestOverlayAbsentIsTheBuiltinExactly(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if rt.Path != "" {
		t.Errorf("no overlay file: Path must stay empty, got %q", rt.Path)
	}
	if rt.Model(TierStandard) != claudeModels[TierStandard] {
		t.Errorf("no overlay file: model must be the built-in's, got %q", rt.Model(TierStandard))
	}
}

// The overlay wins for the keys it declares; everything else — including
// the realizer, the skill surface, Egress it did not name — stays the
// built-in's.
func TestOverlayPerKeyOnABuiltin(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	writeOverlay(t, a, "claude", strings.Join([]string{
		"model_fast: claude-instance-fast",
		"startup_wait: 90s",
		"record: trusted",
		"record_why: measured on this box, 2026-08-31",
		"",
	}, "\n"))

	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if rt.Path == "" {
		t.Fatal("an overlay file must set Path")
	}
	// model_fast: alone leaves strong/standard on the built-in map.
	if rt.Model(TierFast) != "claude-instance-fast" {
		t.Errorf("model_fast: not overlaid: %q", rt.Model(TierFast))
	}
	if rt.Model(TierStrong) != claudeModels[TierStrong] || rt.Model(TierStandard) != claudeModels[TierStandard] {
		t.Errorf("strong/standard must stay the built-in's when only fast is declared: %q %q", rt.Model(TierStrong), rt.Model(TierStandard))
	}
	if rt.Wait() != 90*time.Second {
		t.Errorf("startup_wait: not overlaid: %s", rt.Wait())
	}
	if rt.RecordTrust() != RecordTrusted || rt.RecordWhy == "" {
		t.Errorf("record:/record_why: not overlaid: %q %q", rt.Record, rt.RecordWhy)
	}
	// Untouched: the realizer, the skill surface, egress.
	if rt.Realize == nil || rt.Skills == nil {
		t.Error("the built-in's realizer and skill surface must survive an overlay")
	}
	if len(rt.Egress) == 0 || rt.Egress[0] != "api.anthropic.com" {
		t.Errorf("undeclared egress: must stay the built-in's: %v", rt.Egress)
	}
	if rt.Command == "" || !strings.Contains(rt.Command, "claude") {
		t.Errorf("Command must stay the built-in's template: %q", rt.Command)
	}
}

// The overlay mutating rt.Models must not corrupt the SHARED builtinRuntimes
// entry — every other LoadRuntime("claude") call, overlaid or not, must
// keep reading the built-in's own values.
func TestOverlayDoesNotMutateTheBuiltinTable(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	writeOverlay(t, a, "claude", "model_fast: overlaid-only\n")

	if _, err := a.LoadRuntime("claude"); err != nil {
		t.Fatal(err)
	}
	// Read the package-level built-in table directly: it must be untouched.
	for _, rt := range builtinRuntimes {
		if rt.Name == "claude" && rt.Models[TierFast] == "overlaid-only" {
			t.Fatal("overlayBuiltin corrupted the shared builtinRuntimes entry")
		}
	}
	// And a fresh App with no overlay file must still see the built-in's own
	// value, not the previous overlay's.
	b := checkApp(t)
	rt, err := b.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if rt.Model(TierFast) != claudeModels[TierFast] {
		t.Errorf("a different instance's overlay leaked into this one: %q", rt.Model(TierFast))
	}
}

// The mechanism keys change the launch, not a measured fact — ADR 0021
// Decision 2. Every one of them refuses, naming the fact/mechanism split.
//
// Driven FROM builtinMechanismKeys with a body per key, and the table is
// asserted to cover the list: a key added to the refused set with no case
// here reds rather than shipping a wall nothing walked into. Six of these
// eight were declarable and did NOTHING on a built-in until
// ranger-base-otoq8 — the silence this pins shut.
func TestOverlayRefusesMechanismKeys(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	bodies := map[string]string{
		"command":      "command: claude --evil {file}\n",
		"skills_flag":  "skills_flag: --plugin-dir %s\n",
		"skills_cwd":   "skills_cwd: true\n",
		"self_sandbox": "self_sandbox: true\n",
		// A file claude really does read, declared by hand: the guard parity
		// builds on is the built-in's measured list, not this one.
		"project_config":      "project_config: .instance/settings.json\n",
		"project_config_keys": "project_config_keys: [hooks]\n",
		"unattended":          "unattended: --dangerously-skip-permissions\n",
		// The sharp arm: a REGISTERED reader name, so nothing about the
		// value is wrong — it is refused for being mechanism, which is the
		// whole distinction D2 draws.
		"turn_outcome": "turn_outcome: " + TurnOutcomeClaudeTranscript + "\n",
	}
	// Two legacy messages other readers already quote; the rest are checked
	// by shape below.
	legacy := map[string]string{
		"command":     "launch mechanism is not overlayable",
		"skills_flag": "verified mechanism",
	}
	for _, m := range builtinMechanismKeys {
		body, ok := bodies[m.key]
		if !ok {
			t.Errorf("%s: refused by builtinMechanismKeys and probed by nothing — add a case, or the wall ships unwalked", m.key)
			continue
		}
		writeOverlay(t, a, "claude", body)
		_, err := a.LoadRuntime("claude")
		if err == nil {
			t.Errorf("%s: an overlay declaring it loaded clean — that is the ranger-base-otoq8 silence, not a refusal", m.key)
			continue
		}
		for _, want := range []string{m.key + ":", "ADR 0021 Decision 2", "may only overlay:", legacy[m.key]} {
			if want != "" && !strings.Contains(err.Error(), want) {
				t.Errorf("%s: err = %v, want one containing %q", m.key, err, want)
			}
		}
	}
	for k := range bodies {
		found := false
		for _, m := range builtinMechanismKeys {
			if m.key == k {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is probed here but is not in builtinMechanismKeys — the case passes over a wall that moved", k)
		}
	}
}

// Presence, not value: a bare `unattended:` is still this file deciding the
// launch mechanism, and YamlGet-style "is it non-empty" reading would have
// let it through in silence.
func TestOverlayRefusesAValuelessMechanismKey(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	writeOverlay(t, a, "claude", "unattended:\n")
	if _, err := a.LoadRuntime("claude"); err == nil || !strings.Contains(err.Error(), "unattended:") {
		t.Errorf("a valueless mechanism key must refuse, got %v", err)
	}
}

// Every declarable key sits on exactly ONE side of the ADR 0021 split.
//
// This is the pin the finding asked for: `runtimeYamlKeys()` is the surface
// a yaml may declare, and until ranger-base-otoq8 thirteen of its entries
// were in neither builtinOverlayKeys nor the refusal — so an overlay
// declaring `state_dir:` or `unattended:` on a built-in loaded clean, warned
// nothing (warnUnknownRuntimeKeys knows every key here) and changed nothing.
// A new declarable key now reds until someone decides which it is, the same
// shape TestEveryRuntimeFieldIsClassified holds for the struct.
func TestEveryDeclarableKeyIsClassifiedForTheOverlay(t *testing.T) {
	t.Parallel()
	overlay := map[string]bool{}
	for _, k := range builtinOverlayKeys {
		if overlay[k] {
			t.Errorf("builtinOverlayKeys names %s twice", k)
		}
		overlay[k] = true
	}
	mech := map[string]bool{}
	for _, m := range builtinMechanismKeys {
		if mech[m.key] {
			t.Errorf("builtinMechanismKeys names %s twice", m.key)
		}
		if strings.TrimSpace(m.why) == "" {
			t.Errorf("%s: refused with no why — a wall that cannot say what it stopped is a list, not a decision", m.key)
		}
		mech[m.key] = true
	}
	// model_<tier> is expanded from Tiers on both sides, so it is neither
	// spelled nor a decision: the overlay loop applies whichever tiers exist.
	tier := map[string]bool{}
	for _, t2 := range Tiers {
		tier["model_"+t2] = true
	}

	for _, k := range runtimeYamlKeys() {
		n := 0
		for _, in := range []bool{overlay[k], mech[k], tier[k]} {
			if in {
				n++
			}
		}
		switch {
		case n == 0:
			t.Errorf("%s: declarable and classified by nothing — an overlay on a built-in declaring it loads clean and changes nothing (ADR 0021 D1: overlay it if it names a measured instance fact, refuse it if it changes the launch mechanism)", k)
		case n > 1:
			t.Errorf("%s: classified %d ways — a key cannot both overlay and refuse", k, n)
		}
	}
	// And the other direction: a classification for a key no yaml may
	// declare is a decision about nothing, and would warn on its own file.
	declarable := map[string]bool{}
	for _, k := range runtimeYamlKeys() {
		declarable[k] = true
	}
	for k := range overlay {
		if !declarable[k] {
			t.Errorf("builtinOverlayKeys names %s, which runtimeYamlKeys() does not — the overlay applies a key the loader warns is unknown", k)
		}
	}
	for k := range mech {
		if !declarable[k] {
			t.Errorf("builtinMechanismKeys refuses %s, which runtimeYamlKeys() does not — a wall in front of a key nobody may write", k)
		}
	}
}

// Classified is not applied. Every key on the overlay side gets a body and
// a reading here, driven from builtinOverlayKeys so a key added to the set
// and wired to nothing reds — which is exactly the defect this bead was
// filed for: `state_dir:` was declarable, known, and read by no overlay.
//
// Each row carries a CONTROL: the same reading off a built-in with no
// overlay file must differ from what the overlay declares. Without it a row
// asserting claude's own value would be green over an overlay that did
// nothing at all.
func TestOverlayAppliesEveryInstanceFactKey(t *testing.T) {
	t.Parallel()
	const tierFamily = "model_<tier>"
	rows := map[string]struct {
		body string
		get  func(*Runtime) string
		want string
	}{
		tierFamily:     {"model_fast: instance-fast\n", func(r *Runtime) string { return r.Model(TierFast) }, "instance-fast"},
		"model_flag":   {"model_flag: -m %s\n", func(r *Runtime) string { return r.ModelFlag }, "-m %s"},
		"prompt":       {"prompt: " + PromptArgv + "\n", func(r *Runtime) string { return r.Prompt }, PromptArgv},
		"startup_wait": {"startup_wait: 90s\n", func(r *Runtime) string { return r.Wait().String() }, "1m30s"},
		"record":       {"record: " + RecordUntrusted + "\n", func(r *Runtime) string { return r.Record }, RecordUntrusted},
		"record_why": {"record: trusted\nrecord_why: measured on this box 2026-09-02\n",
			func(r *Runtime) string { return r.RecordWhy }, "measured on this box 2026-09-02"},
		"native_rules": {"native_rules: [ONLY.md]\n", func(r *Runtime) string { return strings.Join(r.NativeRules, ",") }, "ONLY.md"},
		"egress":       {"egress: [instance.example.com]\n", func(r *Runtime) string { return strings.Join(r.Egress, ",") }, "instance.example.com"},
		"cage_cred":    {"cage_cred: INSTANCE_TOKEN\n", func(r *Runtime) string { return r.CageCred }, "INSTANCE_TOKEN"},
		"gate_shell":   {"gate_shell: false\n", func(r *Runtime) string { return fmt.Sprint(r.NoGateShell) }, "true"},
		"rules_precedence": {"rules_precedence: " + RulesPrecedenceNative + "\n",
			func(r *Runtime) string { return r.RulesPrecedence }, RulesPrecedenceNative},
		"rules_precedence_why": {"rules_precedence: " + RulesPrecedenceNative + "\nrules_precedence_why: probed here, ranger-base-otoq8\n",
			func(r *Runtime) string { return r.RulesPrecedenceWhy }, "probed here, ranger-base-otoq8"},
		"state_dir":    {"state_dir: ~/.instance-claude\n", func(r *Runtime) string { return strings.Join(r.StateDirs, ",") }, "~/.instance-claude"},
		"env_required": {"env_required: AWS_REGION\n", func(r *Runtime) string { return strings.Join(r.EnvRequired, ",") }, "AWS_REGION"},
	}

	bare, err := checkApp(t).LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range append([]string{tierFamily}, builtinOverlayKeys...) {
		row, ok := rows[k]
		if !ok {
			t.Errorf("%s: on the overlay side and read by no case here — the key this bead exists about was classified and wired to nothing", k)
			continue
		}
		a := checkApp(t)
		writeOverlay(t, a, "claude", row.body)
		rt, err := a.LoadRuntime("claude")
		if err != nil {
			t.Errorf("%s: %v", k, err)
			continue
		}
		if got := row.get(rt); got != row.want {
			t.Errorf("%s: overlay declared %q, loaded runtime reads %q — the declaration never arrived", k, row.want, got)
		}
		if got := row.get(bare); got == row.want {
			t.Errorf("%s: the built-in already reads %q with no overlay file — this row would be green over an overlay that did nothing; pick a value the built-in does not carry", k, got)
		}
	}
	for k := range rows {
		if k == tierFamily {
			continue
		}
		found := false
		for _, o := range builtinOverlayKeys {
			if o == k {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is probed here but is not in builtinOverlayKeys — the case passes over a key the overlay no longer applies", k)
		}
	}
}

// state_dir:/env_required: REPLACE when present and keep the built-in's
// when absent — the native_rules:/egress: rule. The empty-list arm is the
// one that needs saying out loud: it moves the seatbelt's writable grant,
// so `state_dir: []` on claude is an operator declaring that this install
// keeps no state where posse thinks it does, and the launch shows it by
// re-running a first-run flow rather than by granting something quietly.
func TestOverlayStateDirAndEnvRequiredReplace(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	writeOverlay(t, a, "claude", "env_required: [AWS_REGION, AWS_PROFILE]\n")
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(rt.StateDirs, ",") != strings.Join(claudeStateDirsForTest(t), ",") {
		t.Errorf("an overlay naming no state_dir: must keep the built-in's: %v", rt.StateDirs)
	}
	if len(rt.EnvRequired) != 2 {
		t.Errorf("env_required: list not overlaid: %v", rt.EnvRequired)
	}

	writeOverlay(t, a, "claude", "state_dir: []\n")
	rt2, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if rt2.StateDirs != nil {
		t.Errorf("an explicitly empty state_dir: must clear the built-in's list, got %v", rt2.StateDirs)
	}
}

// claudeStateDirsForTest reads the built-in's own list through a home with
// no overlay file, rather than spelling it: a fixture that repeats the
// value under test pins the copy, not the code.
func claudeStateDirsForTest(t *testing.T) []string {
	t.Helper()
	rt, err := checkApp(t).LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	return rt.StateDirs
}

// Present-but-wrong still refuses on the keys that joined the overlay —
// the ADR 0013 §2 rule the nine already wore. `rules_precedence: pdi` on a
// built-in must not read as "unmeasured" and `state_dir: .claude` must not
// silently grant a directory under the session tree.
func TestOverlayInstanceFactKeysRefuseAWrongValue(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	for _, c := range []struct{ name, body, want string }{
		{"rules_precedence", "rules_precedence: pdi\n", "want pid or native"},
		{"state_dir", "state_dir: .claude\n", "must be absolute or ~-prefixed"},
		{"env_required", "env_required: AWS_REGION=us-east-1\n", "names only, never values"},
	} {
		writeOverlay(t, a, "claude", c.body)
		_, err := a.LoadRuntime("claude")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one containing %q", c.name, err, c.want)
		}
	}
}

// record: trusted with no record_why: refuses — overlay and template-only
// alike (ADR 0021 Decision 4). Nothing promotes a runtime's own contract
// without naming the measurement behind it.
func TestOverlayAndTemplateRequireRecordWhy(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	writeOverlay(t, a, "claude", "record: trusted\n")
	if _, err := a.LoadRuntime("claude"); err == nil || !strings.Contains(err.Error(), "no record_why:") {
		t.Errorf("overlay record: trusted with no record_why: must refuse, got %v", err)
	}

	writeOverlay(t, a, "tplonly", "command: tplonly --sys {file}\nrecord: trusted\n")
	if _, err := a.LoadRuntime("tplonly"); err == nil || !strings.Contains(err.Error(), "no record_why:") {
		t.Errorf("template-only record: trusted with no record_why: must refuse, got %v", err)
	}
}

// List-valued keys REPLACE, no merge: an overlay's native_rules: drops the
// built-in's list rather than adding to it, and an explicit empty list
// clears it — told apart from the key being absent by yamlHasKey, not by
// length.
func TestOverlayListKeysReplace(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	writeOverlay(t, a, "claude", "native_rules: [ONLY.md]\negress: [instance.example.com]\n")
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.NativeRules) != 1 || rt.NativeRules[0] != "ONLY.md" {
		t.Errorf("native_rules: must REPLACE the built-in's list: %v", rt.NativeRules)
	}
	if len(rt.Egress) != 1 || rt.Egress[0] != "instance.example.com" {
		t.Errorf("egress: must REPLACE the built-in's list: %v", rt.Egress)
	}

	writeOverlay(t, a, "codex", "native_rules: []\n")
	rt2, err := a.LoadRuntime("codex")
	if err != nil {
		t.Fatal(err)
	}
	if rt2.NativeRules != nil {
		t.Errorf("an explicitly empty native_rules: must clear the built-in's list, got %v", rt2.NativeRules)
	}
}

// declaredBy() already source-splits per key once Path is set — ADR 0021
// says "with no new code", so this pins that it really does for an
// overlaid built-in: an overlaid key names the file, an untouched one
// still names "built-in default".
func TestOverlayDeclaredBy(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	writeOverlay(t, a, "claude", "prompt: argv\n")
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if by := rt.declaredBy("prompt"); !strings.Contains(by, "runtimes/claude.yaml (prompt:)") {
		t.Errorf("an overlaid key must be credited to the yaml: %q", by)
	}
	if by := rt.declaredBy("record"); by != "built-in default" {
		t.Errorf("an untouched key on an overlaid built-in must still read built-in default: %q", by)
	}
}

// ListRuntimes: an overlaid built-in lists once, not twice.
func TestOverlaidBuiltinListsOnce(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	writeOverlay(t, a, "claude", "prompt: argv\n")
	writeRuntime(t, a, "extra", "command: extra --sys {file}\n")

	names := a.ListRuntimes()
	count := 0
	for _, n := range names {
		if n == "claude" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("claude must list exactly once, got %d in %v", count, names)
	}
	found := false
	for _, n := range names {
		if n == "extra" {
			found = true
		}
	}
	if !found {
		t.Errorf("a template-only runtime must still list: %v", names)
	}
}

// The unknown-key warning still fires over an overlay file — a typo on a
// built-in's overlay is exactly as silent a dead wall as one on a
// template-only yaml.
func TestOverlayWarnsUnknownKeys(t *testing.T) {
	a := checkApp(t)
	var buf bytes.Buffer
	old := runtimeNoticeWriter
	runtimeNoticeWriter = &buf
	defer func() { runtimeNoticeWriter = old }()

	writeOverlay(t, a, "claude", "sartup_wait: 90s\n")
	if _, err := a.LoadRuntime("claude"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "sartup_wait:") {
		t.Errorf("a typo'd overlay key must warn: %q", buf.String())
	}
}

// ADR 0021 D5: `runtime check`'s built-in footer names the overlayable keys.
// It is the screen an onboarder reads the rule off, and while it was spelled
// by hand it said "command: and skills_flag: REFUSE" over eight refusals and
// nine overlay keys over fourteen. Rendered from the code that applies them,
// this pin is what keeps the rendering honest — and the width check is the
// price of rendering: a list that grows has to keep fitting the screen the
// grid above it is measured against.
func TestBuiltinFooterNamesBothSidesOfTheSplit(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
	out := b.String()
	i := strings.Index(out, "is a BUILT-IN")
	j := strings.Index(out, "Onboarding your OWN CLI")
	if i < 0 || j < i {
		t.Fatalf("the built-in overlay footer is not on the screen any more:\n%s", out)
	}
	footer := out[i:j]

	for _, k := range append([]string{"model_<tier>"}, builtinOverlayKeys...) {
		if !strings.Contains(footer, k+":") {
			t.Errorf("the built-in footer never names the overlayable key %s: — a rule an onboarder cannot read is one they find by having a launch refused:\n%s", k, footer)
		}
	}
	for _, m := range builtinMechanismKeys {
		if !strings.Contains(footer, m.key+":") {
			t.Errorf("the built-in footer never names the REFUSED key %s: — the wall is undiscoverable until it fires:\n%s", m.key, footer)
		}
	}
	for _, ln := range strings.Split(footer, "\n") {
		if n := len([]rune(ln)); n > 100 {
			t.Errorf("footer line is %d columns — the one-screen rule the grid keeps (gridWidth) dies at the footer: %q", n, ln)
		}
	}
}
