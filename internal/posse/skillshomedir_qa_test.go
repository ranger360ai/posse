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
//
// THE QUESTION IS ASKED WITH THE PRODUCTION PREDICATE, not with a first
// segment (ranger-base-szxlx, verifying this file's close). What makes a
// declared root cover this tree is the same arithmetic
// refusePersonaModeGlobalWrite does — `underDir(filepath.Join(home,
// GlobalRoot), …)` — and `filepath.Join` CLEANS, so four spellings resolve
// onto `<home>/.agents` while their first segment is not `.agents` at all:
// `/.agents`, `./.agents`, `.` (the home itself, which contains everything
// under it) and `../<home>/.agents`. MEASURED 2026-10-03: all four cover the
// tree under `underDir` and a `strings.Split(GlobalRoot, "/")[0] == seg`
// test flags none of them. A root is a hand-typed table literal, so the
// spelling is exactly the kind of thing that arrives by hand.
//
// The sweep therefore carries its own two halves, because a loop over a
// table can go green by reaching nothing: the FLOOR (at least one runtime
// declares a root at all, else the comparison below never runs) and the
// CONTROL (the same predicate, over a planted root, must say yes).
func TestQANoRuntimeDeclaresAGlobalRootOverTheAgentsSkillsTree(t *testing.T) {
	t.Parallel()
	// The home is a real directory because `underDir` resolves symlinks over
	// the deepest existing ancestor — on darwin a `/var` home is a link, and
	// a textual test reads it as outside itself.
	home := t.TempDir()
	tree := AgentsSkillsDir(home)
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	if tree == home {
		t.Fatalf("AgentsSkillsPath is %q — this arm needs a path under the session dir to ask containment about", AgentsSkillsPath)
	}
	// covers is refusePersonaModeGlobalWrite's own reading of a declared
	// root, with the glob left out exactly as the production guard leaves it
	// out: anywhere under `<home>/<GlobalRoot>` is inside.
	covers := func(globalRoot string) bool {
		if globalRoot == "" {
			return false
		}
		return underDir(filepath.Join(home, filepath.FromSlash(globalRoot)), tree)
	}

	// CONTROL first: the predicate must say yes to a root that does cover
	// the tree, in every spelling that resolves onto it. Without this the
	// whole sweep is green over a predicate that answers no to everything.
	for _, spelling := range []string{
		AgentsSkillsPath,                               // `.agents/skills`
		strings.Split(AgentsSkillsPath, "/")[0],        // `.agents`
		"/" + strings.Split(AgentsSkillsPath, "/")[0],  // `/.agents`
		"./" + strings.Split(AgentsSkillsPath, "/")[0], // `./.agents`
		".", // the home itself
		"../" + filepath.Base(home) + "/" + strings.Split(AgentsSkillsPath, "/")[0],
	} {
		if !covers(spelling) {
			t.Errorf("control: a GlobalRoot of %q resolves onto %s and must read as covering it — this arm cannot catch what it is for", spelling, tree)
		}
	}
	// And the negative half of the control, so `covers` is not a function
	// that says yes to everything: bob's real root does not cover the tree.
	if covers(".bob") {
		t.Error("control: a GlobalRoot of \".bob\" must NOT read as covering the skills tree — the predicate is answering yes to everything")
	}

	// builtinRuntimes is the WHOLE set to sweep: `PersonaMode` has no yaml
	// key at all, so a template runtime cannot declare a config root, and a
	// built-in is the only place one can appear.
	//
	// By VALUE, not `&builtinRuntimes[i]`: taking the address of an element
	// puts the table in cmd/testparallel's WRITTEN set, which makes every
	// `t.Parallel` test that merely reads it uncleared — nine of them, and
	// `make verify-parallel` names all nine for one `&` here.
	declared := 0
	for _, rt := range builtinRuntimes {
		c := rt.PersonaMode
		if c == nil || c.GlobalRoot == "" {
			continue
		}
		declared++
		if covers(c.GlobalRoot) {
			t.Errorf("runtime %s declares %q as its GLOBAL config root, which covers %s — the skills tree at a session dir of $HOME is then a global write, and wants refusePersonaModeGlobalWrite's treatment rather than TestQASkillsTreeAtASessionDirThatIsTheHomeIsWrittenOnPurpose's blessing (ranger-base-q114b)", rt.Name, c.GlobalRoot, AgentsSkillsPath)
		}
	}
	// FLOOR: the sweep above is a loop, and a loop that iterates over
	// nothing is indistinguishable from a loop that found nothing. bob has
	// declared `.bob` since ADR 0060; if no runtime declares a root any
	// more, this arm has stopped asking the question rather than answering
	// it — and the sibling that would notice,
	// TestQAAtTheHomeTheSkillsTreeIsWrittenAndTheModeFileRefuses, notices
	// only for the runtime its fixture uses.
	if declared == 0 {
		t.Errorf("no built-in runtime declares a PersonaMode.GlobalRoot, so the sweep above compared nothing — %s is unheld, not clear (ranger-base-q114b)", AgentsSkillsPath)
	}
}
