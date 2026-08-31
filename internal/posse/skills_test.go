package posse

// Skills binding (docs/adr/0007-skills-binding.md): the PID names skills,
// the launch materializes them for the chosen runtime, and a runtime with
// no per-session skill surface refuses the launch like an unrealized gate.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// callsSourced is calls(t, fake) with any spilled launch script's body
// appended: a rendered line long enough to lose MAX_CANON (paneline.go,
// rangerhq-ybec) is typed as `. '<script>'` instead of verbatim, and a
// substring assertion on what the persona was launched with must still see
// it whichever way this particular rendering happened to land. Adding a
// deny rule, a longer --settings, or another mount is exactly the class of
// change paneline.go names as pushing a line over that cliff.
func callsSourced(t *testing.T, fake string) string {
	t.Helper()
	log := calls(t, fake)
	out := log
	for _, ln := range strings.Split(log, "\n") {
		idx := strings.Index(ln, ". '")
		if idx < 0 {
			continue
		}
		path, _, ok := strings.Cut(ln[idx+len(". '"):], "'")
		if !ok {
			continue
		}
		if body, err := os.ReadFile(path); err == nil {
			out += "\n" + string(body)
		}
	}
	return out
}

// mkSkill writes an Agent-Skills dir (<name>/SKILL.md) under dir.
func mkSkill(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	// description: is not decoration — codex silently skips a SKILL.md
	// without one (verified 2026-08-18), so fixtures carry it too.
	if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: the "+name+" skill\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// §1: a name resolves to RHQ_HOME/skills/<name>/SKILL.md or it is unknown —
// no index, no copy. A symlinked entry counts, which is the whole point of
// the dir (it points at ~/.claude/skills, a plugin, a repo).
func TestResolveAndListSkills(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents")}
	os.MkdirAll(a.SkillsDir(), 0o755)
	mkSkill(t, a.SkillsDir(), "dataviz")
	elsewhere := mkSkill(t, t.TempDir(), "code-review")
	if err := os.Symlink(elsewhere, a.SkillPath("code-review")); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(a.SkillPath("empty-dir"), 0o755) // no SKILL.md → not a skill

	if got := strings.Join(a.ListSkills(), ","); got != "code-review,dataviz" {
		t.Errorf("ListSkills: %q", got)
	}
	paths, unknown := a.ResolveSkills([]string{"dataviz", "ghost", "empty-dir"})
	if len(paths) != 1 || paths[0] != a.SkillPath("dataviz") {
		t.Errorf("paths: %v", paths)
	}
	if strings.Join(unknown, ",") != "ghost,empty-dir" {
		t.Errorf("unknown: %v", unknown)
	}

	// Bindings: which PIDs name each skill (posse skills list).
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "d.md"), []byte("---\nname: d\nskills: [dataviz, ghost]\n---\nYou are d.\n"), 0o644)
	os.WriteFile(filepath.Join(a.AgentsDir, "l.md"), []byte("---\nname: l\nskills:\n  - dataviz\n---\nYou are l.\n"), 0o644)
	b := a.SkillBindings()
	if strings.Join(b["dataviz"], ",") != "d,l" || strings.Join(b["ghost"], ",") != "d" {
		t.Errorf("bindings: %v", b)
	}
}

// §2: the rendered tree is claude's plugin shape, written fresh at every
// launch — stale names go, unknown names refuse.
func TestRenderClaudeSkills(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	os.MkdirAll(a.SkillsDir(), 0o755)
	mkSkill(t, a.SkillsDir(), "dataviz")
	mkSkill(t, a.SkillsDir(), "code-review")

	dir, err := a.RenderClaudeSkills("developer", []string{"dataviz", "code-review"})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(a.StateDir, "skills", "developer", "claude"); dir != want {
		t.Errorf("dir: %s, want %s", dir, want)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man pluginManifest
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	if man.Name != "posse-developer" || man.Description != "skills bound by posse" {
		t.Errorf("plugin.json: %+v", man)
	}
	// Each name is a symlink to RHQ_HOME/skills/<name>, so the SKILL.md the
	// session loads is the one the operator installed — nothing was copied.
	for _, n := range []string{"dataviz", "code-review"} {
		link := filepath.Join(dir, "skills", n)
		target, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("%s is not a symlink: %v", n, err)
		}
		if target != a.SkillPath(n) {
			t.Errorf("%s → %s, want %s", n, target, a.SkillPath(n))
		}
		if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
			t.Errorf("%s/SKILL.md not reachable through the tree: %v", n, err)
		}
	}

	// Dropping a name from the PID stops binding it.
	if _, err := a.RenderClaudeSkills("developer", []string{"dataviz"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "skills", "code-review")); err == nil {
		t.Error("stale skill survived a re-render")
	}
	// Binding nothing renders nothing at all.
	if d, err := a.RenderClaudeSkills("developer", nil); err != nil || d != "" {
		t.Errorf("empty skills: %q %v", d, err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("tree survived an empty skills: list")
	}
	// An unknown name refuses rather than binding a dangling symlink.
	if _, err := a.RenderClaudeSkills("developer", []string{"dataviz", "ghost"}); err == nil || !strings.Contains(err.Error(), "no such skill ghost") {
		t.Errorf("unknown skill must refuse: %v", err)
	}
}

// §2 again: {skills} renders through the runtime's realizer — claude's
// --plugin-dir, nothing (and no gap) elsewhere or when the list is empty.
func TestSkillsPlaceholderRendering(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents")}
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "p.md"), []byte("---\nname: p\nskills: [dataviz]\ndeny: [Bash(git push:*)]\n---\nYou are p.\n"), 0o644)
	os.WriteFile(filepath.Join(a.AgentsDir, "q.md"), []byte("---\nname: q\n---\nYou are q.\n"), 0o644)
	withSkills, _ := a.LoadAgent("p")
	none, _ := a.LoadAgent("q")
	tree := filepath.Join(a.StateDir, "skills", "p", "claude")

	claude, _ := a.LoadRuntime("claude")
	got := withSkills.RenderCommandFor(claude, "claude", TierStrong)
	if !strings.Contains(got, "--plugin-dir '"+tree+"' ") {
		t.Errorf("claude must be pointed at the tree:\n%s", got)
	}
	// claude's tool flags are variadic and would swallow what follows, so
	// {skills} has to land in front of them.
	if i, j := strings.Index(got, "--plugin-dir"), strings.Index(got, "--disallowedTools"); i < 0 || j < 0 || i > j {
		t.Errorf("--plugin-dir must precede the variadic tool flags:\n%s", got)
	}
	if got := none.RenderCommandFor(claude, "claude", TierStrong); strings.Contains(got, "{skills}") || strings.Contains(got, "--plugin-dir") || strings.Contains(got, "  ") {
		t.Errorf("an empty skills: must vanish with its space:\n%s", got)
	}
	// codex/grok realize the binding from the session's cwd (rangerhq-1qd),
	// so they take no flag at all: {skills} must vanish with its space, and
	// the tree path must never leak onto their line.
	for _, n := range []string{"codex", "grok"} {
		rt, _ := a.LoadRuntime(n)
		got := withSkills.RenderCommandFor(rt, "claude", TierStrong)
		if strings.Contains(got, "{skills}") || strings.Contains(got, "--plugin-dir") || strings.Contains(got, tree) || strings.Contains(got, "  ") {
			t.Errorf("%s renders no skills flag:\n%s", n, got)
		}
		if !rt.RealizesSkills(withSkills.Skills) || !rt.SkillsCwd {
			t.Errorf("%s realizes skills from the session cwd", n)
		}
		if !rt.RealizesSkills(nil) {
			t.Errorf("%s with no skills declared is realizable", n)
		}
	}
	// A template-only runtime opts in with skills_flag:.
	os.MkdirAll(a.RuntimesDir(), 0o755)
	os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"), []byte("command: mycli {skills} --sys {file}\nskills_flag: --skill-plugin\n"), 0o644)
	os.WriteFile(filepath.Join(a.RuntimesDir(), "plain.yaml"), []byte("command: plain --sys {file} {skills}\n"), 0o644)
	my, err := a.LoadRuntime("mycli")
	if err != nil {
		t.Fatal(err)
	}
	if got := withSkills.RenderCommandFor(my, "claude", TierStrong); !strings.HasPrefix(got, "mycli --skill-plugin '"+tree+"' --sys ") {
		t.Errorf("skills_flag: %s", got)
	}
	plain, _ := a.LoadRuntime("plain")
	if plain.RealizesSkills(withSkills.Skills) {
		t.Error("a template-only runtime with no skills_flag: has no surface")
	}
	if got := withSkills.RenderCommandFor(plain, "claude", TierStrong); strings.Contains(got, "{skills}") || strings.HasSuffix(got, " ") {
		t.Errorf("plain: %q", got)
	}
}

// §3: declared means required — every runtime either materializes the
// binding (claude by flag, codex/grok by the session cwd) or degrades the
// launch, and the refusal names the skills and the runtime.
func TestSkillsParity(t *testing.T) {
	b, fake := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.AgentsDir, 0o755)
	os.MkdirAll(a.SkillsDir(), 0o755)
	mkSkill(t, a.SkillsDir(), "dataviz")
	mkSkill(t, a.SkillsDir(), "code-review")
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"),
		[]byte("---\nname: developer\nskills: [dataviz, code-review]\n---\nYou are developer.\n"), 0o644)
	ag, _ := a.LoadAgent("developer")

	claude, _ := a.LoadRuntime("claude")
	if p := a.CheckParity(ag, claude, CageShims, TierStrong); len(p.Degraded) != 0 ||
		!strings.Contains(p.Realized["skills: dataviz, code-review"].Detail, "--plugin-dir") {
		t.Errorf("claude realizes the binding: %+v", p)
	}
	for _, n := range []string{"codex", "grok"} {
		rt, _ := a.LoadRuntime(n)
		p := a.CheckParity(ag, rt, CageShims, TierStrong)
		if len(p.Degraded) != 0 || !strings.Contains(p.Realized["skills: dataviz, code-review"].Detail, AgentsSkillsPath) {
			t.Errorf("%s realizes the binding from the session dir: %+v", n, p)
		}
	}

	// A template-only runtime that names no skills_flag: is the case with
	// no surface at all — the launch refuses, and says why.
	os.MkdirAll(a.RuntimesDir(), 0o755)
	os.WriteFile(filepath.Join(a.RuntimesDir(), "plain.yaml"), []byte("command: plain --sys {file} {skills}\n"), 0o644)
	plain, _ := a.LoadRuntime("plain")
	p := a.CheckParity(ag, plain, CageShims, TierStrong)
	want := "skills: dataviz, code-review — plain has no per-session skill surface"
	if len(p.Degraded) != 1 || p.Degraded[0] != want {
		t.Errorf("plain must degrade with %q: %+v", want, p)
	}
	if len(p.Unrealized) != 0 {
		t.Errorf("skills are not a wall claim — Unrealized must stay empty: %v", p.Unrealized)
	}
	err := b.CreateSession(NewSessionOpts{Name: "dp", Agent: "developer", Runtime: "plain", Dir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("launch must refuse: %v", err)
	}
	// --allow-degraded launches it marked, with no skills on the line but
	// the env exit hatch still there.
	mustCreate(t, b, NewSessionOpts{Name: "dp", Agent: "developer", Runtime: "plain", AllowDegraded: true})
	m, _ := b.readMeta("dp")
	if m == nil || !strings.Contains(m.Degraded, "no per-session skill surface") {
		t.Errorf("meta must record the degradation: %+v", m)
	}
	log := calls(t, fake)
	if strings.Contains(log, "--plugin-dir") {
		t.Errorf("plain gets no skills flag:\n%s", log)
	}
	if !strings.Contains(log, "--env RHQ_SKILLS_DIR="+a.SkillsDir()) || !strings.Contains(log, "--env RHQ_SKILLS=dataviz") {
		t.Errorf("the env exit hatch must ride anyway:\n%s", log)
	}
}

// §2 for codex and grok (rangerhq-1qd): the binding materializes as
// symlinks in the session dir, never overwrites what posse did not write,
// leaves another persona's links alone, and sweeps its own dead ones.
func TestRenderAgentsSkills(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	os.MkdirAll(a.SkillsDir(), 0o755)
	mkSkill(t, a.SkillsDir(), "dataviz")
	mkSkill(t, a.SkillsDir(), "code-review")
	mkSkill(t, a.SkillsDir(), "doomed")
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}

	dir, err := a.RenderAgentsSkills(repo, "developer", []string{"dataviz", "doomed"})
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(repo, ".agents", "skills") {
		t.Errorf("dir: %s", dir)
	}
	// The CLI finds SKILL.md through the link, and git does not find the dir.
	if _, err := os.Stat(filepath.Join(dir, "dataviz", "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not reachable: %v", err)
	}
	out, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output()
	if err != nil || len(strings.TrimSpace(string(out))) != 0 {
		t.Errorf("the session dir must stay out of git status: %q (%v)", out, err)
	}
	if b, _ := os.ReadFile(filepath.Join(repo, ".gitignore")); len(b) != 0 {
		t.Error("the repo's own .gitignore must never be touched")
	}
	// Idempotent: a second launch appends no second exclude line.
	if _, err := a.RenderAgentsSkills(repo, "developer", []string{"dataviz", "doomed"}); err != nil {
		t.Fatal(err)
	}
	ex, _ := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if n := strings.Count(string(ex), AgentsSkillsPath); n != 1 {
		t.Errorf("exclude written %d times:\n%s", n, ex)
	}

	// Another persona in the same repo adds its own name and keeps ours:
	// the dir belongs to the repo, and binding is additive (ADR 0007 §4).
	if _, err := a.RenderAgentsSkills(repo, "qa", []string{"code-review"}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"dataviz", "doomed", "code-review"} {
		if _, err := os.Stat(filepath.Join(dir, n, "SKILL.md")); err != nil {
			t.Errorf("%s must still be bound: %v", n, err)
		}
	}

	// A skill that leaves RHQ_HOME/skills leaves the repo too — a dangling
	// link is the one thing the ADR spends a refusal on.
	os.RemoveAll(a.SkillPath("doomed"))
	if _, err := a.RenderAgentsSkills(repo, "developer", []string{"dataviz"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "doomed")); err == nil {
		t.Error("a dead link must be swept")
	}

	// Never clobber what posse did not write.
	os.MkdirAll(filepath.Join(dir, "mine"), 0o755)
	mkSkill(t, a.SkillsDir(), "mine")
	if _, err := a.RenderAgentsSkills(repo, "developer", []string{"mine"}); err == nil || !strings.Contains(err.Error(), "not overwriting") {
		t.Errorf("a foreign entry must refuse the launch: %v", err)
	}
	// An unknown name refuses before anything is written.
	if _, err := a.RenderAgentsSkills(repo, "developer", []string{"ghost"}); err == nil || !strings.Contains(err.Error(), "no such skill ghost") {
		t.Errorf("unknown name: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "ghost")); err == nil {
		t.Error("nothing may be written for an unknown name")
	}
}

// Acceptance: a PID with skills: on claude launches with --plugin-dir at a
// tree whose skills/<name>/SKILL.md is the installed one; relaunch keeps it.
func TestSkillsLaunchAndRelaunch(t *testing.T) {
	b, fake := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.AgentsDir, 0o755)
	os.MkdirAll(a.SkillsDir(), 0o755)
	mkSkill(t, a.SkillsDir(), "dataviz")
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"),
		[]byte("---\nname: developer\nskills: [dataviz]\ndeny: [Bash(git push:*)]\n---\nYou are developer.\n"), 0o644)

	mustCreate(t, b, NewSessionOpts{Name: "dc", Agent: "developer", Dir: t.TempDir()})
	tree := filepath.Join(a.StateDir, "skills", "developer", "claude")
	log := callsSourced(t, fake)
	if !strings.Contains(log, "--plugin-dir '"+tree+"'") {
		t.Errorf("the typed line must point claude at the tree:\n%s", log)
	}
	if !strings.Contains(log, "--env RHQ_SKILLS_DIR="+a.SkillsDir()) || !strings.Contains(log, "--env RHQ_SKILLS=dataviz") {
		t.Errorf("skills env missing:\n%s", log)
	}
	// /dataviz resolves in the session because the tree reaches the file.
	if _, err := os.Stat(filepath.Join(tree, "skills", "dataviz", "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not reachable through the launched tree: %v", err)
	}

	// A crash restart re-renders and re-points at the same tree.
	os.Remove(filepath.Join(fake, "agents.json"))
	m, _ := b.readMeta("dc")
	m.Launched = m.Launched.Add(-time.Hour)
	b.writeMeta(m)
	os.RemoveAll(tree)
	if ok, err := b.RelaunchAgent("dc", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	if got := callsSourced(t, fake); strings.Count(got, "--plugin-dir '"+tree+"'") != 2 {
		t.Errorf("relaunch must keep the binding:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(tree, "skills", "dataviz", "SKILL.md")); err != nil {
		t.Errorf("relaunch must re-render the tree: %v", err)
	}

	// A name that resolves to nothing refuses the launch outright.
	os.WriteFile(filepath.Join(a.AgentsDir, "ghosty.md"),
		[]byte("---\nname: ghosty\nskills: [ghost]\n---\nYou are ghosty.\n"), 0o644)
	if err := b.CreateSession(NewSessionOpts{Name: "gh", Agent: "ghosty", Dir: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "no such skill ghost") {
		t.Errorf("unknown skill must refuse the launch: %v", err)
	}
}

// §5: the linter findings for a name that resolves to nothing and a PID
// whose own command: forgot {skills}.
func TestCheckAgentSkills(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	os.MkdirAll(a.AgentsDir, 0o755)
	os.MkdirAll(a.SkillsDir(), 0o755)
	mkSkill(t, a.SkillsDir(), "dataviz")
	os.WriteFile(filepath.Join(a.AgentsDir, "bad.md"),
		[]byte("---\nname: bad\ncommand: claude {model} --sys {file} {allow} {deny}\nskills: [dataviz, ghost]\n---\nYou are bad.\n"), 0o644)
	fs, _, err := a.CheckAgent("bad")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(fs, "\n")
	for _, want := range []string{`unknown skill "ghost"`, "command: has no {skills}"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing finding %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, `"dataviz"`) {
		t.Errorf("a resolving skill is not a finding:\n%s", joined)
	}
	// The scaffold's commented-out skills: block parses to nothing and stays clean.
	if _, err := a.ScaffoldAgent("fresh"); err != nil {
		t.Fatal(err)
	}
	ag, _ := a.LoadAgent("fresh")
	if len(ag.Skills) != 0 {
		t.Errorf("scaffold skills: %v", ag.Skills)
	}
	if fs, _, _ := a.CheckAgent("fresh"); len(fs) != 0 {
		t.Errorf("scaffold has findings: %v", fs)
	}
}

// §5, the third finding of the same kind (rangerhq-3zr): the SKILL.md
// resolves and carries no `description:`, which codex drops in silence.
//
// Measured on codex-cli 0.147.0 with `codex debug prompt-input` over a
// throwaway cwd carrying .agents/skills/probeskill: with a description the
// dumped prompt carries one `- probeskill: <description> (file: …)` row,
// and with the key absent, empty, or the whole frontmatter block missing
// the name does not appear anywhere in the 29 KB of model-visible input.
//
// The two arms that pin the *reader* rather than the outcome are `folded`
// and `bodyonly`: a block scalar is a description a real YAML parser
// downstream will render, and a `description:` line in the body is not one
// — a check that read the whole file instead of the frontmatter block
// would call that PID clean and drop the finding.
func TestCheckAgentSkillNeedsDescription(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	os.MkdirAll(a.AgentsDir, 0o755)
	os.MkdirAll(a.SkillsDir(), 0o755)
	for _, tc := range []struct {
		name    string
		skill   string
		finding bool
	}{
		{"described", "---\nname: described\ndescription: what it is for\n---\nbody\n", false},
		{"folded", "---\nname: folded\ndescription: >-\n  what it is for\n---\nbody\n", false},
		{"nokey", "---\nname: nokey\n---\nA first paragraph claude and grok fall back to.\n", true},
		{"empty", "---\nname: empty\ndescription:\n---\nbody\n", true},
		{"nulled", "---\nname: nulled\ndescription: ~\n---\nbody\n", true},
		{"nofront", "A first paragraph and no frontmatter block at all.\n", true},
		{"bodyonly", "---\nname: bodyonly\n---\ndescription: this line is body, not frontmatter\n", true},
	} {
		dir := filepath.Join(a.SkillsDir(), tc.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(tc.skill), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a.AgentsDir, tc.name+".md"),
			[]byte("---\nname: "+tc.name+"\nskills: ["+tc.name+"]\n---\nYou are "+tc.name+", the tester of the crew.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		fs, _, err := a.CheckAgent(tc.name)
		if err != nil {
			t.Fatal(err)
		}
		var got string
		for _, f := range fs {
			if strings.Contains(f, "has no description:") {
				got = f
			}
			if strings.Contains(f, "unknown skill") {
				t.Fatalf("%s: the fixture must resolve: %v", tc.name, fs)
			}
		}
		switch {
		case tc.finding && got == "":
			t.Errorf("%s: no description: is a finding, got %v", tc.name, fs)
		case tc.finding && !strings.Contains(got, strconv.Quote(tc.name)):
			t.Errorf("%s: the finding must name the skill: %s", tc.name, got)
		case tc.finding && !strings.Contains(got, AbbrevHome(filepath.Join(dir, "SKILL.md"))):
			t.Errorf("%s: the finding must name the file to fix: %s", tc.name, got)
		case !tc.finding && got != "":
			t.Errorf("%s: a described skill is not a finding: %s", tc.name, got)
		}
	}
}

// Acceptance (rangerhq-1qd): a security-style PID — denies Edit and Write,
// binds skills — launches on codex with the skills actually resolvable in
// the session dir, nothing about them typed on the line, and a crash
// restart re-materializes them.
func TestSkillsOnCodexAcceptance(t *testing.T) {
	b, fake := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.AgentsDir, 0o755)
	os.MkdirAll(a.SkillsDir(), 0o755)
	mkSkill(t, a.SkillsDir(), "dataviz")
	os.WriteFile(filepath.Join(a.AgentsDir, "security.md"),
		[]byte("---\nname: security\nruntime: codex\nskills: [dataviz]\ndeny: [Edit, Write]\n---\nYou are security.\n"), 0o644)
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}

	mustCreate(t, b, NewSessionOpts{Name: "hc", Agent: "security", Dir: repo})
	bound := filepath.Join(repo, AgentsSkillsPath, "dataviz", "SKILL.md")
	if _, err := os.Stat(bound); err != nil {
		t.Fatalf("codex must find the skill at %s: %v", bound, err)
	}
	log := calls(t, fake)
	if strings.Contains(log, "--plugin-dir") || strings.Contains(log, "{skills}") {
		t.Errorf("codex takes no skills flag:\n%s", log)
	}
	if !strings.Contains(log, "-s read-only") {
		t.Errorf("the PID's own gates still render:\n%s", log)
	}
	if !strings.Contains(log, "--env RHQ_SKILLS_DIR="+a.SkillsDir()) || !strings.Contains(log, "--env RHQ_SKILLS=dataviz") {
		t.Errorf("the env exit hatch must ride anyway:\n%s", log)
	}

	// A crash restart re-materializes the binding it just removed.
	os.RemoveAll(filepath.Join(repo, ".agents"))
	os.Remove(filepath.Join(fake, "agents.json"))
	m, _ := b.readMeta("hc")
	m.Launched = m.Launched.Add(-time.Hour)
	b.writeMeta(m)
	if ok, err := b.RelaunchAgent("hc", time.Second); err != nil || !ok {
		t.Fatal(err)
	}
	if _, err := os.Stat(bound); err != nil {
		t.Errorf("relaunch must re-bind: %v", err)
	}
}
