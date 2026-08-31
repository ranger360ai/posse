package posse

// ADR 0021: a runtimes/<name>.yaml naming a built-in is a per-key OVERLAY,
// not a template. Mirrors runtimeyamlv2_test.go's shape — the template-only
// validation is the pattern these tests reuse against LoadRuntime's other
// branch.

import (
	"bytes"
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

// command:/skills_flag: change the launch mechanism, not a measured fact —
// ADR 0021 Decision 2. Both refuse, naming the fact/mechanism split.
func TestOverlayRefusesMechanismKeys(t *testing.T) {
	a := checkApp(t)
	for _, c := range []struct{ name, body, want string }{
		{"command", "command: claude --evil {file}\n", "launch mechanism is not overlayable"},
		{"skills_flag", "skills_flag: --plugin-dir %s\n", "verified mechanism"},
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
