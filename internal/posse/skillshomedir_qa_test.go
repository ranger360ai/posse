//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-q114b: the session dir that IS the operator's home, and the
// DELIBERATE asymmetry between the two trees posse writes there.
//
// ranger-base-se81d guarded the persona-mode file against this exact input
// and ADR 0062's Consequences left the sibling open: `posse new` has no
// worktree option, its session dir is `--dir`, else `default_dir`, else
// `$HOME` (herdrback.go's `CfgGet` fallback), so one interactive launch with
// no `--dir` names the home as the workspace — and down that same default
// `~/.agents/skills/` has ALREADY landed (created 2026-09-05,
// ranger-base-4mrmc F2).
//
// THE ANSWER IS THAT THIS ONE IS FINE, and these pins are what makes that a
// position rather than an omission. The two writes differ in the one way
// that matters:
//
//   - `~/.bob/plugins/posse/custom_modes.yaml` lands in a glob bob reads
//     GLOBALLY. The write means something at the home it does not mean in
//     any other directory — one persona's whole PID becomes a selectable
//     mode in every bob session on the box — so the materializer refuses.
//   - `~/.agents/skills/` lands on a surface read at the cwd ONLY. MEASURED
//     2026-08-18 (ADR 0007 §2, codex-cli 0.147.0 and grok 1.0.5): discovery
//     is at the cwd and does not climb — and bob's own row is cwd-discovery
//     too (ADR 0060). No CLI reads `~/.agents` as a global root at all:
//     each of the three has its own and it is a different path
//     (`~/.codex/skills`, `~/.grok/skills`, `~/.bob/skills`). So the tree
//     at the home is read by a
//     cwd-at-home session, and that is exactly what the tree in any other
//     directory is: ADR 0007 §4 already declares this surface additive,
//     owned by the DIR rather than by the persona, and outliving the seat.
//     `$HOME` is a directory under that contract, not an escalation of it.
//
// And the seam, which is the second reason there is no guard here: FOUR
// writes land inside the session dir — the pre-push and commit-guard hooks
// under its `.git/hooks`, the skills tree (plus its `.git/info/exclude`
// line), and the mode file — with two more keyed on the dir without writing
// into it (the L3 hooks redirect's `core.hooksPath`, claude's directory
// trust). If a session whose dir is the home should be refused, that is one
// rule about the DIR, at the place the dir is resolved —
// ranger-base-er6mt's root cause, giving `posse new` a tree of its own —
// and not a refusal bolted into one of the four, which would stop the
// skills render while the rest proceeded. Arm 3 below is that order,
// pinned: a bob launch at the home installs the hooks, writes this tree,
// and THEN refuses, which is why the already-landed `~/.agents/skills` is
// not evidence of a stray write.
//
// WHAT WOULD FLIP IT is one fact, and arm 4 holds it: a CLI that reads
// `~/.agents` as a global root. The day a runtime declares `.agents` as its
// `PersonaModeChannel.GlobalRoot` — or any config root covering this tree —
// the write at the home becomes the mode file's case and wants the mode
// file's refusal. That is a red test, not a silent drift.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// skillsHomePID binds one skill, which is what makes RenderSkillsFor do
// anything at all (a PID with no `skills:` renders nothing and would be
// green over every arm below).
const skillsHomePID = `---
name: p
skills:
  - dataviz
deny: [Bash(git push:*)]
---
You are p, the probe persona.
`

// skillsHomeNeedsGit: two of the arms are git arithmetic (`.git/info/exclude`
// and the `git status` the exclude is for), so a box without git measures
// nothing here rather than passing.
func skillsHomeNeedsGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}
}

func TestQASkillsTreeAtASessionDirThatIsTheHomeIsWrittenOnPurpose(t *testing.T) {
	// No t.Parallel: t.Setenv. And the home has to be the PROCESS's `$HOME`
	// rather than a path that merely looks like one, because that is the
	// input every other skills pin does NOT have — they hand the writer a
	// temp dir as the session dir, which is the one input under which the
	// question this file answers is invisible.
	skillsHomeNeedsGit(t)
	rhq := gitTempDir(t)
	a := &App{Home: rhq, StateDir: filepath.Join(rhq, "state"), AgentsDir: filepath.Join(rhq, "agents")}
	if err := os.MkdirAll(a.SkillsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	mkSkill(t, a.SkillsDir(), "dataviz")

	home := gitTempDir(t)
	t.Setenv("HOME", home)
	// A home that is a git repo, because that is the real one: a dotfiles
	// repo is the common shape, and it is the arm where posse appends to
	// `~/.git/info/exclude`. The write is wanted — the tree must stay out
	// of the operator's `git status` the way it does in any other repo.
	gitRun(t, home, "init", "-b", "main")

	// ARM 1, THE MEASURED CASE: the session dir IS `$HOME`, and the tree is
	// written. A refusal here is the change this file exists to stop
	// without a reading to go with it.
	dir, err := a.RenderAgentsSkills(home, "developer", []string{"dataviz"})
	if err != nil {
		t.Fatalf("a session dir that is $HOME was refused the skills tree: %v", err)
	}
	if want := filepath.Join(home, ".agents", "skills"); dir != want {
		t.Errorf("tree at %s, want %s", dir, want)
	}
	// Bound through the link, which is the whole realization on this
	// surface — there is no flag to check instead (ADR 0007 §2).
	if _, err := os.Stat(filepath.Join(dir, "dataviz", "SKILL.md")); err != nil {
		t.Errorf("the skill is not reachable through the tree: %v", err)
	}
	// And out of git's way, at the home as anywhere else.
	out, err := exec.Command("git", "-C", home, "status", "--porcelain").Output()
	if err != nil || strings.TrimSpace(string(out)) != "" {
		t.Errorf("the tree at the home must stay out of git status: %q (%v)", out, err)
	}

	// ARM 2: the launch's own path, not just the writer's. RenderSkillsFor
	// is what herdrback.go calls, and a guard added to EITHER layer would
	// have to red something here.
	ag := loadTestAgent(t, skillsHomePID)
	for _, name := range []string{"codex", "grok"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("%s is not a built-in: %v", name, err)
		}
		if !rt.SkillsCwd {
			t.Fatalf("%s no longer discovers skills from the cwd — this arm is about the cwd surface", name)
		}
		got, err := a.RenderSkillsFor(ag, rt, home)
		if err != nil {
			t.Errorf("%s at a session dir of $HOME was refused: %v", name, err)
			continue
		}
		if got != dir {
			t.Errorf("%s rendered %q, want the home's tree %q", name, got, dir)
		}
	}
}

// ARM 3: the asymmetry in one place, and the ORDER that goes with it. bob
// has both channels, so one runtime at one session dir exercises the pair:
// the skills tree is written, the mode file refuses. The order is
// herdrback.go's (skills at 2344, the mode at 2355) and it is the reason the
// already-created `~/.agents/skills` proves nothing about a stray write —
// a bob launch at the home leaves this tree behind on its way to refusing.
func TestQAAtTheHomeTheSkillsTreeIsWrittenAndTheModeFileRefuses(t *testing.T) {
	skillsHomeNeedsGit(t)
	rhq := gitTempDir(t)
	a := &App{Home: rhq, StateDir: filepath.Join(rhq, "state"), AgentsDir: filepath.Join(rhq, "agents")}
	if err := os.MkdirAll(a.SkillsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	mkSkill(t, a.SkillsDir(), "dataviz")
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	home := gitTempDir(t)
	t.Setenv("HOME", home)
	gitRun(t, home, "init", "-b", "main")

	rt := bobRuntime(t, a)
	ag := loadTestAgent(t, skillsHomePID)

	if _, err := a.RenderSkillsFor(ag, rt, home); err != nil {
		t.Errorf("bob's skills tree at a session dir of $HOME was refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "dataviz", "SKILL.md")); err != nil {
		t.Errorf("bob's launch wrote no skills tree at the home: %v", err)
	}
	// The sibling guard, at the identical input. Without this arm every
	// green above is also consistent with ranger-base-se81d's refusal
	// having been dropped.
	if _, err := a.RenderPersonaModeFor(ag, rt, home); err == nil {
		t.Error("the mode file at a session dir of $HOME was written — ranger-base-se81d's guard is the half of this pair that refuses")
	}
}

// ARM 4: the one fact the decision rests on, inside the tree. Nothing here
// can measure what a CLI reads — that is ADR 0007 §2's 2026-08-18 run — but
// the tree's own declarations can be held against it: no runtime names a
// global config root that covers `.agents/skills`. A runtime that did would
// make the home write global, which is the mode file's case and wants the
// mode file's refusal rather than this file's blessing.
func TestQANoRuntimeDeclaresAGlobalRootOverTheAgentsSkillsTree(t *testing.T) {
	t.Parallel()
	// The tree's first segment is what a CLI config root would have to
	// collide with, and it is read off the constant so a change to the
	// surface cannot leave this arm testing a stale name.
	seg := strings.Split(AgentsSkillsPath, "/")[0]
	if seg == "" || seg == AgentsSkillsPath {
		t.Fatalf("AgentsSkillsPath is %q — this arm needs a rooted relative path to take a first segment from", AgentsSkillsPath)
	}
	// builtinRuntimes is the WHOLE set to sweep: `PersonaMode` has no yaml
	// key at all, so a template runtime cannot declare a config root, and a
	// built-in is the only place one can appear.
	//
	// By VALUE, not `&builtinRuntimes[i]`: taking the address of an element
	// puts the table in cmd/testparallel's WRITTEN set, which makes every
	// `t.Parallel` test that merely reads it uncleared — nine of them, and
	// `make verify-parallel` names all nine for one `&` here.
	for _, rt := range builtinRuntimes {
		c := rt.PersonaMode
		if c == nil || c.GlobalRoot == "" {
			continue
		}
		root := strings.Split(c.GlobalRoot, "/")[0]
		if root == seg {
			t.Errorf("runtime %s declares %q as its GLOBAL config root, which covers %s — the skills tree at a session dir of $HOME is then a global write, and wants refusePersonaModeGlobalWrite's treatment rather than TestQASkillsTreeAtASessionDirThatIsTheHomeIsWrittenOnPurpose's blessing (ranger-base-q114b)", rt.Name, c.GlobalRoot, AgentsSkillsPath)
		}
	}
}
