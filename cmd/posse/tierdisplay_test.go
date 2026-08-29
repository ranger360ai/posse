package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/rhq"
)

// ADR 0013 §6 in the cockpit row: the persona tag carries the DISPLAY tier,
// so a session on a runtime that maps nothing draws as @<runtime>/default
// while a claude session at fast keeps its own name. Both rows come off the
// same renderer, so the mapping — not the fixture — is what separates them.
//
// The unmapped side is a DECLARED runtime with no model_<tier>:. It used to
// be grok, which mapped nothing until rangerhq-jp6; all three built-ins map
// every tier now, so a rule about the map has to be fixtured on the map.
func TestCockpitRowShowsTheDisplayTier(t *testing.T) {
	home := t.TempDir()
	app := &rhq.App{Home: home, ConfigPath: filepath.Join(home, "config.yaml")}
	if err := os.MkdirAll(app.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app.RuntimesDir(), "blankcli.yaml"),
		[]byte("command: blankcli {model} --sys {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &cockpit{
		app: app,
		sessions: []rhq.HerdrSession{
			{Name: "dev-blank", Agent: "dev", Status: "idle", Runtime: "blankcli", Tier: "standard"},
			{Name: "dev-claude", Agent: "dev", Status: "idle", Runtime: "claude", Tier: "fast"},
			{Name: "dev-grok", Agent: "dev", Status: "idle", Runtime: "grok", Tier: "fast"},
		},
	}
	got := stripANSI(renderRow(row{kind: rowItem, cols: c.sessionCols(c.sessions[0])}, 120, false))
	if !strings.Contains(got, "@blankcli/default") || strings.Contains(got, "@blankcli/standard") {
		t.Errorf("an unmapped tier must draw as default: %q", got)
	}
	if got := stripANSI(renderRow(row{kind: rowItem, cols: c.sessionCols(c.sessions[1])}, 120, false)); !strings.Contains(got, "@claude/fast") {
		t.Errorf("a mapped tier keeps its own name: %q", got)
	}
	// grok maps fast → grok-4.5 since rangerhq-jp6, so it wears the name too.
	if got := stripANSI(renderRow(row{kind: rowItem, cols: c.sessionCols(c.sessions[2])}, 120, false)); !strings.Contains(got, "@grok/fast") {
		t.Errorf("grok maps every tier now and must keep its own name: %q", got)
	}
}

// The same rule on the `posse agents` listing, which is the other place an
// operator reads a persona's runtime/tier. Run through the built binary
// because the listing is written inline in main's command switch — there is
// no function under it to call.
func TestAgentsListingShowsTheDisplayTier(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	agents := filepath.Join(home, "agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "runtimes", "blankcli.yaml"),
		[]byte("command: blankcli {model} --sys {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, runtime string }{{"onx", "blankcli"}, {"ony", "codex"}, {"onz", "grok"}} {
		body := "---\nname: " + c.name + "\ndescription: d\nruntime: " + c.runtime + "\ntier: strong\n---\nYou are " + c.name + ", the developer of the crew.\n"
		if err := os.WriteFile(filepath.Join(agents, c.name+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(bin, "agents")
	cmd.Env = []string{"HOME=" + t.TempDir(), "RHQ_HOME=" + home, "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("posse agents: %v\n%s", err, out)
	}
	got := string(out)
	// blankcli maps no model id: the listing says so. codex and grok both
	// map strong, so they still wear the name — the discriminating half.
	if !strings.Contains(got, "[blankcli/default]") || strings.Contains(got, "[blankcli/strong]") {
		t.Errorf("a strong PID on an unmapped runtime must list as blankcli/default:\n%s", got)
	}
	for _, want := range []string{"[codex/strong]", "[grok/strong]"} {
		if !strings.Contains(got, want) {
			t.Errorf("a mapped tier keeps its own name, want %s:\n%s", want, got)
		}
	}
}

// rangerhq-jp6 item 4: `posse runtimes` must show grok's tier map the way
// it shows claude's. It does so off Runtime.TierMap, the same rendering the
// `runtime check` grid reads — so this is a pin on the CATALOG line, which
// is where an operator picking a runtime actually looks.
//
// All three built-in rows are asserted together on purpose. The line grok
// used to print was "tiers: UNMAPPED — ignores tier:, the CLI picks its own
// model"; a test that only checked grok's new row would stay green if the
// switch that chooses between the three renderings broke for everyone else.
func TestRuntimesCatalogShowsEveryBuiltinTierMap(t *testing.T) {
	bin := buildRhq(t)
	cmd := exec.Command(bin, "runtimes")
	cmd.Env = []string{"HOME=" + t.TempDir(), "RHQ_HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH"),
		"RHQ_HERDR_BIN=no-such-herdr-binary"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("posse runtimes: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{
		"claude   built-in · tiers: strong=claude-fable-5 standard=claude-opus-5 fast=claude-sonnet-5",
		"codex    built-in · tiers: strong=gpt-5.6-sol standard=gpt-5.6-sol fast=gpt-5.6-luna",
		"grok     built-in · tiers: strong=grok-4.6 standard=grok-4.6 fast=grok-4.5",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("posse runtimes missing %q in:\n%s", want, got)
		}
	}
	// No built-in may still be advertising that it ignores the key.
	if strings.Contains(got, "UNMAPPED") {
		t.Errorf("every built-in maps every tier since rangerhq-jp6:\n%s", got)
	}
}
