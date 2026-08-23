package rhq

// The reproducible half of rangerhq-1qd: materialize a binding with the
// real code, then let the installed codex and grok say whether they see
// it. Skipped unless RHQ_E2E=1, because it needs both CLIs on PATH —
// re-run it when either updates, since the surface it leans on
// (<cwd>/.agents/skills) is a discovery convention, not a documented flag.
//
//	RHQ_E2E=1 go test ./internal/rhq/ -run E2ESkillSurfaces -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2ESkillSurfaces(t *testing.T) {
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
}
