package posse

// Skills binding (ADR 0007): the PID names skills, the launch materializes
// them for the chosen runtime. posse copies nothing and indexes nothing —
// RHQ_HOME/skills/<name>/SKILL.md *is* the registry, `posse skills` is `ls`
// and `posse agent check` is `stat`. The dir holds real Agent-Skills dirs or
// symlinks to wherever they already live (~/.claude/skills/x, a plugin's
// skills/x, a repo); posse does not care which.
//
// Two shapes materialize, one per *kind* of surface (rangerhq-1qd):
//
//   - a flag pointing at a rendered tree — claude's plugin shape, a dir
//     carrying .claude-plugin/plugin.json and skills/<name> symlinks, typed
//     as `--plugin-dir` (`--add-dir` is CLAUDE.md dirs and does not load
//     skills). A template-only runtime with `skills_flag:` is handed the
//     same dir: what sits inside it is the universal Agent-Skills layout and
//     the plugin.json is inert to anything that does not read it.
//   - no flag at all — codex and grok discover skills from the session's
//     *working directory*, so the launch symlinks them into
//     `<cwd>/.agents/skills/<name>` and `{skills}` renders nothing. That is
//     the ADR's last resort, and it is the only surface those two CLIs
//     have: verified 2026-08-18 on codex-cli 0.147.0 and grok 1.0.5 (see
//     RenderAgentsSkills and NOTES.md).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// SkillsDir is RHQ_HOME/skills.
func (a *App) SkillsDir() string { return filepath.Join(a.Home, "skills") }

// SkillPath is where a name must resolve.
func (a *App) SkillPath(name string) string { return filepath.Join(a.SkillsDir(), name) }

// ResolveSkills splits names into the ones carrying a SKILL.md (as paths,
// in the PID's order) and the ones that resolve to nothing.
func (a *App) ResolveSkills(names []string) (paths, unknown []string) {
	for _, n := range names {
		p := a.SkillPath(n)
		if st, err := os.Stat(filepath.Join(p, "SKILL.md")); err == nil && !st.IsDir() {
			paths = append(paths, p)
			continue
		}
		unknown = append(unknown, n)
	}
	return paths, unknown
}

// SkillDescription is the `description:` a skill's SKILL.md frontmatter
// declares, "" when the file has no frontmatter block, no such key, or an
// empty value. Not decoration: `name` and `description` are the two
// required Agent-Skills frontmatter keys, and codex enforces that by
// silence — it renders each discovered skill as one `- <name>: <description>`
// line and drops a SKILL.md carrying no description entirely, so the
// binding materializes and the persona has nothing (verified codex-cli
// 0.147.0, rangerhq-3zr; claude and grok fall back to the body's first
// paragraph instead — rangerhq-1qd). Read
// through the same flat-YAML subset as everything else: a folded or
// literal block scalar (`description: >-`) reads as present, which is what
// a real YAML parser downstream will also make of it.
func SkillDescription(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return ""
	}
	front, _ := agentFrontmatter(string(b))
	return yamlGetLines(front, "description")
}

// ListSkills returns the names under RHQ_HOME/skills carrying a SKILL.md,
// sorted. Stat, not ReadDir modes: an entry is usually a symlink to a dir.
func (a *App) ListSkills() []string {
	ents, _ := os.ReadDir(a.SkillsDir())
	var out []string
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(a.SkillsDir(), e.Name(), "SKILL.md")); err == nil {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// SkillBindings maps a skill name to the personas whose PID binds it —
// including names no SKILL.md answers, which is what makes the listing
// worth reading.
func (a *App) SkillBindings() map[string][]string {
	out := map[string][]string{}
	for _, n := range a.ListAgents() {
		ag, err := a.LoadAgent(n)
		if err != nil {
			continue
		}
		for _, s := range ag.Skills {
			out[s] = append(out[s], ag.Name)
		}
	}
	return out
}

// pluginManifest is the .claude-plugin/plugin.json a rendered skills tree
// carries — the minimum that makes claude read its skills/ dir.
type pluginManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RenderClaudeSkills writes the persona's skills tree fresh and returns its
// path (RHQ_HOME/state/skills/<persona>/claude), or "" when the PID binds
// nothing. Like the gates it is rendered at every launch and the old tree is
// removed first, so a name dropped from the PID stops being bound.
//
// An unknown name is an error rather than a dangling symlink: `skills:` is a
// statement that the persona's work depends on them (ADR 0007 §3), and the
// one failure mode worth spending a refusal on is a persona that launches
// believing it has a skill it does not. `posse agent check` catches it before
// the launch does.
func (a *App) RenderClaudeSkills(persona string, names []string) (string, error) {
	dir := filepath.Join(a.StateDir, "skills", persona, "claude")
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", nil
	}
	paths, unknown := a.ResolveSkills(names)
	if len(unknown) > 0 {
		return "", Die("%s: skills: no such skill %s (looked in %s — a name resolves to <name>/SKILL.md there)",
			persona, strings.Join(unknown, ", "), AbbrevHome(a.SkillsDir()))
	}
	skills := filepath.Join(dir, "skills")
	if err := os.MkdirAll(filepath.Join(dir, ".claude-plugin"), 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(skills, 0o755); err != nil {
		return "", err
	}
	manifest, err := json.Marshal(pluginManifest{Name: "posse-" + persona, Description: "skills bound by posse"})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude-plugin", "plugin.json"), append(manifest, '\n'), 0o644); err != nil {
		return "", err
	}
	for i, n := range names {
		if err := os.Symlink(paths[i], filepath.Join(skills, n)); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// AgentsSkillsPath is the per-session skill surface codex and grok share,
// relative to the session's working directory. Both CLIs walk it at *that
// directory only* — neither climbs to the repo root — so the launch links
// into the dir the session actually starts in.
const AgentsSkillsPath = ".agents/skills"

// AgentsSkillsDir is AgentsSkillsPath under a session's cwd.
func AgentsSkillsDir(cwd string) string {
	return filepath.Join(cwd, filepath.FromSlash(AgentsSkillsPath))
}

// RenderAgentsSkills links the persona's skills into the session dir's
// .agents/skills/ and returns that dir. This is ADR 0007's last resort,
// taken because it is the only surface codex and grok have — verified
// 2026-08-18: codex-cli 0.147.0 and grok 1.0.5 both read
// <cwd>/.agents/skills/<name>/SKILL.md, follow symlinks out of the repo,
// and ignore git's ignore rules while doing it, so `.git/info/exclude`
// hides the dir from `git status` without hiding it from the CLI.
//
// Three rules keep a shared repo honest, and they are why this is not just
// RenderClaudeSkills with a different path:
//
//   - Never clobber. An entry we did not write — the repo's own skill of
//     that name, or anything not a symlink into RHQ_HOME/skills — refuses
//     the launch instead of being replaced. posse writes into the
//     operator's repo here; it may only ever write its own links. The one
//     exception is a DANGLING symlink: a link resolving to nothing binds
//     nothing, so it is a relic rather than the operator's work and the
//     launch replaces it (ranger-base-f6hiy).
//   - Union, not replacement. The dir belongs to the *repo*, not to the
//     persona (claude's tree is per-persona; this one cannot be), so a
//     launch adds its own names and leaves other personas' links alone.
//     Binding is additive anyway (ADR 0007 §4) — posse guarantees presence,
//     not absence — but it does mean a name dropped from a PID keeps its
//     link in a repo another persona still binds it in.
//   - Sweep the dead. A link of ours whose target has left
//     RHQ_HOME/skills is removed: a dangling skill is the one failure the
//     ADR spends a refusal on, and it must not survive in a repo.
func (a *App) RenderAgentsSkills(cwd, persona string, names []string) (string, error) {
	if len(names) == 0 {
		return "", nil
	}
	dir := AgentsSkillsDir(cwd)
	paths, unknown := a.ResolveSkills(names)
	if len(unknown) > 0 {
		return "", Die("%s: skills: no such skill %s (looked in %s — a name resolves to <name>/SKILL.md there)",
			persona, strings.Join(unknown, ", "), AbbrevHome(a.SkillsDir()))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for i, n := range names {
		link := filepath.Join(dir, n)
		switch target, ours := a.boundSkillLink(link); {
		case target == paths[i]:
			// already bound to the same skill — leave it as it is
		case ours, danglingSkillLink(link):
			if err := os.Remove(link); err != nil {
				return "", err
			}
			fallthrough
		case target == "":
			if err := os.Symlink(paths[i], link); err != nil {
				return "", err
			}
		default:
			return "", Die("%s: skills: %s exists and was not written by posse — not overwriting (move it aside, or drop %q from skills:)",
				persona, AbbrevHome(link), n)
		}
	}
	a.sweepDeadSkillLinks(dir)
	excludeFromGit(cwd, AgentsSkillsPath)
	return dir, nil
}

// boundSkillLink reports what link points at and whether posse wrote it (a
// symlink into RHQ_HOME/skills — the literal target this file writes).
// target is "" when nothing is there at all, and link itself — a value no
// skill path can equal — when something is there that is not ours.
func (a *App) boundSkillLink(link string) (target string, ours bool) {
	if _, err := os.Lstat(link); err != nil {
		return "", false
	}
	t, err := os.Readlink(link)
	if err != nil || !strings.HasPrefix(t, a.SkillsDir()+string(os.PathSeparator)) {
		return link, false // a real dir, or a symlink that is not ours
	}
	return t, true
}

// danglingSkillLink reports whether link is a symlink whose target does not
// exist. Such an entry binds nothing, so it is a post-cutover relic and not
// the operator's own file: the one this was filed for pointed into
// ~/.config/rhq — the home retired days earlier (ADR 0015) — and its only
// effect was to refuse every launch of the persona that named that skill,
// until it was removed by hand (ranger-base-f6hiy).
//
// Only "does not exist" counts. A link into a directory this uid cannot
// traverse, or a symlink loop, resolves to an error that is not
// fs.ErrNotExist, is not evidence that the target is gone, and still
// refuses — the never-clobber rule holds everywhere it can still be wrong.
func danglingSkillLink(link string) bool {
	fi, err := os.Lstat(link)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return false
	}
	_, err = os.Stat(link)
	return errors.Is(err, fs.ErrNotExist)
}

// sweepDeadSkillLinks removes our links whose skill has left
// RHQ_HOME/skills. Other personas' live links stay (see the union rule).
func (a *App) sweepDeadSkillLinks(dir string) {
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		link := filepath.Join(dir, e.Name())
		if _, ours := a.boundSkillLink(link); !ours {
			continue
		}
		if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
			os.Remove(link)
		}
	}
}

// excludeFromGit adds rel to the repo's .git/info/exclude — never the
// repo's own .gitignore, which is the operator's file and shows in a diff
// (ADR 0007's alternatives). Best effort: a dir that is not a repo has
// nothing to pollute. The pattern is anchored at the repo root, so a
// session started in a subdirectory excludes its own path and not another
// one that happens to share the name. git's own --show-prefix does that
// arithmetic: filepath.Rel against --show-toplevel gets it wrong wherever
// a path component is a symlink (on macOS, every /var/… temp dir).
func excludeFromGit(dir, rel string) {
	prefix, err := exec.Command("git", "-C", dir, "rev-parse", "--show-prefix").Output()
	if err != nil {
		return
	}
	common, err := exec.Command("git", "-C", dir, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return
	}
	gitDir := strings.TrimSpace(string(common))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}
	pattern := "/" + strings.TrimSpace(string(prefix)) + rel + "/"
	p := filepath.Join(gitDir, "info", "exclude")
	if b, err := os.ReadFile(p); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(line) == pattern {
				return
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "\n# posse: skills bound into this session (ADR 0007) — session-local, not the repo's\n%s\n", pattern)
}

// RenderSkillsFor materializes the PID's skills for the runtime it is
// launching on in the session's working directory dir, and returns the dir
// that was written ("" when the PID binds none, or when this runtime has
// no surface at all — the parity check has already made that a refusal
// unless the operator waived it).
func (a *App) RenderSkillsFor(ag *AgentFile, rt *Runtime, dir string) (string, error) {
	if len(ag.Skills) == 0 || !rt.RealizesSkills(ag.Skills) {
		return "", nil
	}
	if rt.SkillsCwd {
		if dir == "" {
			return "", Die("%s: skills: %s discovers skills from the session's working directory and this session has none", ag.Name, rt.Name)
		}
		return a.RenderAgentsSkills(dir, ag.Name, ag.Skills)
	}
	return a.RenderClaudeSkills(ag.Name, ag.Skills)
}
