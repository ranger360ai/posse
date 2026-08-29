package rhq

// ADR 0012 D2 (rangerhq-icb3): the harness's headline feature must be
// demonstrable FROM THE SHIPPED EXAMPLES ALONE. examples/skills carries the
// generic distributed-systems canon, two shelf PIDs declare it, and the seed
// ships recipes for the two non-claude runtimes. What this file pins is the
// join: seed a scratch instance from the embed, hire a shelf PID the way
// `posse init` tells an operator to, and the skill it declares materializes
// on all three built-in runtimes — rangerhq-74c6's check with the instance
// removed.

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// hireExamplePID copies a shelf PID into agents/ — the one move `posse init`
// prints, and the only way an example ever becomes a lane (ranger-base-qajs).
func hireExamplePID(t *testing.T, a *App, name string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), name+".md"))
	if err != nil {
		t.Fatalf("shelf PID %s: %v", name, err)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExamplePIDsBindTheSeededSkill(t *testing.T) {
	b, fake := newTestBackend(t)
	a := b.App
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatal(err)
	}
	hireExamplePID(t, a, "architect")
	hireExamplePID(t, a, "developer")

	// The declaration survives the seed, and the name it declares resolves
	// against the skills/ that same seed wrote. Those are two different
	// halves of the bead and both have to hold at once: a PID naming a skill
	// nobody ships refuses every launch it is on.
	for _, name := range []string{"architect", "developer"} {
		ag, err := a.LoadAgent(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := strings.Join(ag.Skills, ","); got != "distributed-systems" {
			t.Errorf("%s skills: %q — the shelf PID must declare the seeded skill", name, got)
		}
		if _, unknown := a.ResolveSkills(ag.Skills); len(unknown) != 0 {
			t.Errorf("%s: %v resolves to nothing under %s", name, unknown, a.SkillsDir())
		}
		// The declaration must not have eaten the list beside it: skills:
		// sits between two block lists in the frontmatter, and the flat-YAML
		// reader takes them one key at a time. The counts move whenever the
		// shelf PIDs do — deny went 3 → 4 when ADR 0019 D4's
		// `Bash(posse refresh:*)` joined the promote line (ranger-base-kryn),
		// then 4 → 27 when ADR 0015 §3's amendment added bd's 23
		// destructive/egress verbs (ranger-base-u9ud); what is pinned is
		// that every list survives the key beside it, not the numbers
		// themselves.
		if len(ag.Intents) != 3 || len(ag.Metrics) != 2 || len(ag.Deny) != 27 {
			t.Errorf("%s frontmatter around skills:: intents %v metrics %v deny %v", name, ag.Intents, ag.Metrics, ag.Deny)
		}
	}
	// `posse skills` answers with both lanes: a skill nobody declares is
	// shipped weight, which is the state this bead found the seed in.
	if got := strings.Join(a.SkillBindings()["distributed-systems"], ","); got != "architect,developer" {
		t.Errorf("SkillBindings: %q — want the two shelf PIDs", got)
	}

	ag, _ := a.LoadAgent("architect")
	const claim = "skills: distributed-systems"
	for _, name := range []string{"claude", "codex", "grok"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		p := a.CheckParity(ag, rt, CageShims, ag.Tier)
		if len(p.Degraded) != 0 {
			t.Errorf("%s: degraded on the shipped example: %v", name, p.Degraded)
		}
		if p.Realized[claim] == "" {
			t.Errorf("%s: %q not realized: %+v", name, claim, p.Realized)
		}
	}

	// And it materializes for real, per surface: claude gets the rendered
	// plugin tree on its line, codex and grok get the session-dir symlink.
	tree := filepath.Join(a.StateDir, "skills", "architect", "claude")
	for _, name := range []string{"claude", "codex", "grok"} {
		dir := t.TempDir()
		if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		mustCreate(t, b, NewSessionOpts{Name: "arch-" + name, Agent: "architect", Runtime: name, Dir: dir})
		if name == "claude" {
			if log := callsSourced(t, fake); !strings.Contains(log, "--plugin-dir '"+tree+"'") {
				t.Errorf("claude must launch pointed at the tree:\n%s", log)
			}
			if _, err := os.Stat(filepath.Join(tree, "skills", "distributed-systems", "SKILL.md")); err != nil {
				t.Errorf("SKILL.md not reachable through the launched tree: %v", err)
			}
			// The canon is references/, loaded on demand — a tree that
			// reaches SKILL.md and not the files it indexes binds a stub.
			if _, err := os.Stat(filepath.Join(tree, "skills", "distributed-systems", "references", "toctou.md")); err != nil {
				t.Errorf("references/ not reachable through the launched tree: %v", err)
			}
			continue
		}
		link := filepath.Join(dir, AgentsSkillsPath, "distributed-systems")
		if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
			t.Errorf("%s: SKILL.md not reachable at %s: %v", name, link, err)
		}
		if _, err := os.Stat(filepath.Join(link, "references", "toctou.md")); err != nil {
			t.Errorf("%s: references/ not reachable at %s: %v", name, link, err)
		}
		out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
		if err != nil || len(strings.TrimSpace(string(out))) != 0 {
			t.Errorf("%s: the binding must stay out of git status: %q (%v)", name, out, err)
		}
	}
}

// The seed's recipes must cover the two non-claude runtimes, and must be
// launchable on the instance they ship to: agents/ arrives empty, so a
// recipe naming a persona names one that does not exist.
func TestExampleRecipesCoverNonClaudeRuntimes(t *testing.T) {
	a := initTestApp(t)
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatal(err)
	}
	want := map[string][3]string{ // name → purpose, command, emoji
		"codex-projA": {"codex", "codex", "🪢"},
		"grok-projB":  {"grok", "grok", "✖️"},
	}
	for name, w := range want {
		r, err := a.LoadRecipe(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if r.Name != name || r.Purpose != w[0] || r.Command != w[1] || r.Emoji != w[2] {
			t.Errorf("%s: %+v — want purpose %q, command %q, emoji %q", name, r, w[0], w[1], w[2])
		}
		if r.Dir == "" {
			t.Errorf("%s: no dir:", name)
		}
		// The emoji map in the seeded config.yaml has to agree with the
		// recipe's own glyph, or `posse list` and `posse recipes` disagree
		// about the same session.
		if got := a.EmojiFor(name); got != w[2] {
			t.Errorf("%s: config emoji map says %q, recipe says %q", name, got, w[2])
		}
	}
	for _, name := range a.ListRecipes() {
		r, err := a.LoadRecipe(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if r.Agent != "" {
			t.Errorf("%s names agent %q — a fresh instance ships no crew, so the recipe would not launch", name, r.Agent)
		}
	}
}
