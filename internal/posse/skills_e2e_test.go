//go:build posse_arm2

package posse

// The reproducible half of rangerhq-1qd: materialize a binding with the
// real code, then let the installed codex and grok say whether they see
// it. Skipped unless RHQ_E2E=1, because it needs both CLIs on PATH —
// re-run it when either updates, since the surface it leans on
// (<cwd>/.agents/skills) is a discovery convention, not a documented flag.
//
//	RHQ_E2E=1 go test ./internal/posse/ -run E2ESkillSurfaces -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2ESkillSurfaces(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_E2E") != "1" {
		t.Skip("set RHQ_E2E=1 with codex and grok installed")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	os.MkdirAll(a.SkillsDir(), 0o755)
	mkSkill(t, a.SkillsDir(), "posse-e2e-probe")
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, err := a.RenderAgentsSkills(repo, "developer", []string{"posse-e2e-probe"}); err != nil {
		t.Fatal(err)
	}
	// codex renders the model-visible prompt; grok lists what it discovered.
	// Both are read-only and neither costs an API call.
	for _, probe := range [][]string{{"codex", "debug", "prompt-input"}, {"grok", "inspect"}} {
		cmd := exec.Command(probe[0], probe[1:]...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", probe[0], err, out)
		}
		if !strings.Contains(string(out), "posse-e2e-probe") {
			t.Errorf("%s does not see the bound skill", probe[0])
		}
	}
	if out, _ := exec.Command("git", "-C", repo, "status", "--porcelain").Output(); len(out) != 0 {
		t.Errorf("the binding must stay out of git status: %s", out)
	}

	// claude's surface is a flag at a rendered plugin tree, not cwd discovery.
	// plugin details is the zero-cost probe (rangerhq-74c6); skip if the CLI
	// is not on PATH so a machine with only codex/grok still runs the rest.
	t.Run("claude-plugin-dir", func(t *testing.T) {
		if _, err := exec.LookPath("claude"); err != nil {
			t.Skip("no claude on PATH")
		}
		dir, err := a.RenderClaudeSkills("developer", []string{"posse-e2e-probe"})
		if err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("claude", "--plugin-dir", dir, "plugin", "details", "posse-developer").CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugin details: %v\n%s", err, out)
		}
		got := string(out)
		if !strings.Contains(got, "posse-e2e-probe") {
			t.Errorf("claude does not list the bound skill:\n%s", got)
		}
		if strings.Contains(got, "not found") {
			t.Errorf("plugin name drifted (want posse-<persona>):\n%s", got)
		}
	})

	// rangerhq-1qd: codex silently skips a SKILL.md with no description:
	// line. grok and claude fall back to the body. A pin that only asserts
	// presence (mkSkill always writes description:) cannot catch a drop.
	t.Run("codex-drops-missing-description", func(t *testing.T) {
		name := "posse-e2e-nodesc"
		p := filepath.Join(a.SkillsDir(), name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte("---\nname: "+name+"\n---\nbody without a description line\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		repo2 := t.TempDir()
		if out, err := exec.Command("git", "-C", repo2, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		if _, err := a.RenderAgentsSkills(repo2, "developer", []string{name}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(repo2, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Fatalf("fixture not linked: %v", err)
		}
		// grok still advertises a skill with no description: (falls back to
		// the body). If this fails, the fixture is not discoverable and the
		// codex drop cannot be judged.
		grok := exec.Command("grok", "inspect")
		grok.Dir = repo2
		gout, err := grok.CombinedOutput()
		if err != nil {
			t.Fatalf("grok inspect: %v\n%s", err, gout)
		}
		if !strings.Contains(string(gout), name) {
			t.Fatalf("grok does not see %s — the linked SKILL.md is not reaching either CLI", name)
		}
		cmd := exec.Command("codex", "debug", "prompt-input")
		cmd.Dir = repo2
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "<skills_instructions>") {
			t.Fatalf("codex prompt-input did not render skills_instructions — the drop cannot be judged:\n%s", out)
		}
		if strings.Contains(string(out), name) {
			t.Errorf("codex listed %s despite no description: line — rangerhq-1qd no longer holds, or the matcher changed", name)
		}
	})
}
