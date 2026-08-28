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
// so a grok session dispatched at standard draws as @grok/default while a
// claude session at fast keeps its own name. Both rows come off the same
// renderer, so the mapping — not the fixture — is what separates them.
func TestCockpitRowShowsTheDisplayTier(t *testing.T) {
	home := t.TempDir()
	c := &cockpit{
		app: &rhq.App{Home: home, ConfigPath: filepath.Join(home, "config.yaml")},
		sessions: []rhq.HerdrSession{
			{Name: "dev-grok", Agent: "dev", Status: "idle", Runtime: "grok", Tier: "standard"},
			{Name: "dev-claude", Agent: "dev", Status: "idle", Runtime: "claude", Tier: "fast"},
		},
	}
	got := stripANSI(renderRow(row{kind: rowItem, cols: c.sessionCols(c.sessions[0])}, 120, false))
	if !strings.Contains(got, "@grok/default") || strings.Contains(got, "@grok/standard") {
		t.Errorf("an unmapped tier must draw as default: %q", got)
	}
	if got := stripANSI(renderRow(row{kind: rowItem, cols: c.sessionCols(c.sessions[1])}, 120, false)); !strings.Contains(got, "@claude/fast") {
		t.Errorf("a mapped tier keeps its own name: %q", got)
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
	for _, c := range []struct{ name, runtime string }{{"onx", "grok"}, {"ony", "codex"}} {
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
	// grok maps no model id: the listing says so. codex maps strong, so it
	// still wears the name — the discriminating half.
	if !strings.Contains(got, "[grok/default]") || strings.Contains(got, "[grok/strong]") {
		t.Errorf("a strong PID on grok must list as grok/default:\n%s", got)
	}
	if !strings.Contains(got, "[codex/strong]") {
		t.Errorf("a mapped tier keeps its own name:\n%s", got)
	}
}
