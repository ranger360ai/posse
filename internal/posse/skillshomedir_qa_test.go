//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-q114b asked whether a session dir that IS the operator's home
// is a special case for the skills tree, and answered no: `~/.agents/skills`
// is read by a cwd-at-home session, which is the session that asked for it,
// and (it said) no CLI reads `~/.agents` as a GLOBAL root — each of codex,
// grok and bob has its own, and none of them was that path. That was the
// measurement the blessing rested on.
//
// ranger-base-q1w0h's operator-run static read of bob 2.0.5's own bundle
// (ADR 0007 §6, Evidence) found that measurement FALSE: bob reads
// `~/.agents/skills` globally after all — and `~/.bob/plugins/*/skills`,
// which is where bob's OWN posse-owned root (BobSkillsRoot,
// `.bob/plugins/posse/skills`, §6d) sits. So the write this file used to
// bless is, today, exactly the mode file's case one path over
// (personamode.go's refusePersonaModeGlobalWrite): a session dir of `$HOME`
// turns a session-local render into a write under a root bob reads in
// EVERY bob session on the box, in every workspace, trusted or not,
// outliving the seat. ADR 0007 §6e is the reversal, THIS FILE's new answer:
// each built-in declares its measured global skill roots
// (`Runtime.SkillsGlobalRoots`, runtime.go, beside
// `PersonaModeChannel.GlobalRoot`), and a cwd render refuses before writing
// when its target lies under ANY declared runtime's root — not only the
// launching runtime's, because bob reads `~/.agents/skills` whoever wrote
// it (refuseSkillsGlobalWrite, skills.go).
//
// THE OLD PIN READ A DECLARATION NOBODY HAD MADE, so it stayed green while
// the fact flipped under it: TestQANoRuntimeDeclaresAGlobalRootOverTheAgentsSkillsTree
// swept `builtinRuntimes` for a `PersonaMode.GlobalRoot` covering
// `.agents/skills` and found none, because the covering root was never a
// *mode* channel's root — it was a *skills* channel's, a dimension that pin
// never read. Its replacement below, TestQAEveryDeclaredSkillsGlobalRootRefusesTheCwdRender,
// reads SkillsGlobalRoots instead, and is a positive assertion (the render
// DOES refuse) rather than the old pin's negative one (no declaration
// exists) — a positive pin cannot go quietly stale the way a "nothing found"
// one did, because removing the declaration it depends on reds it instead
// of leaving it green over nothing.
//
// ARM 3's ordering note moves with the fact: a bob launch at the home used
// to write this tree and THEN refuse at the mode file (herdrback.go, skills
// at 2405, mode at 2416) — which was why the already-landed
// `~/.agents/skills` was not evidence of a stray write. Today the skills
// render is the one that refuses, first, over its OWN declared root, and a
// real launch never reaches the mode file at all. Arm 3 below pins that:
// bob's skills render refuses, AND (independently, calling
// RenderPersonaModeFor directly, the way herdrback.go never gets to) the
// mode guard still refuses the identical input — two guards, either one
// sufficient, so a landing that quietly dropped one would not go unnoticed
// behind the other.
//
// MEASURED 2026-10-12: `~/.agents/skills/distributed-systems` exists on the
// operator's box, a posse link created 2026-09-05 (ranger-base-4mrmc F2) —
// under this file's rule it sits inside a global bob skill root today. It
// is the operator's home: reported in the ADR, not removed (the same
// reasoning ranger-base-se81d gave for the mode-file sibling: deleting a
// file posse cannot prove it wrote is a larger liberty than the write being
// undone).

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

// ARM 1: codex's and grok's shared surface, AgentsSkillsPath, rendered at a
// session dir that IS `$HOME`. It refuses, naming the global root that
// covers it — `.agents/skills`. Until ranger-base-bdias this was read as
// "bob's declaration, not codex's or grok's own" (the "any runtime" half of
// ADR 0007 §6e); the live re-measurement there found codex-cli 0.153.4 DOES
// read `~/.agents/skills` globally (union with the cwd root, not override),
// so codex now declares it too (runtime.go). `builtinRuntimes` sweeps
// claude, codex, grok, bob in that order and the guard returns on the FIRST
// match, so codex's own declaration — not bob's — is the one the message
// below names, for a codex render AND for a grok render (grok still
// declares no root of its own; it is caught by codex's, the same "any
// runtime" clause one owner over). grok's own live probe is a separate
// measurement and stays ASSUMED.
func TestQASkillsTreeAtASessionDirThatIsTheHomeRefusesUnderADeclaredGlobalRoot(t *testing.T) {
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
	// repo is the common shape. This arm asserts a REFUSAL, so there is
	// nothing for `.git/info/exclude` to receive — the repo is here only to
	// make the fixture the realistic one, not because this arm exercises it.
	gitRun(t, home, "init", "-b", "main")

	_, err := a.RenderAgentsSkills(home, "p", []string{"dataviz"})
	if err == nil {
		t.Fatal("a session dir that is $HOME was NOT refused the skills tree — ADR 0007 §6e reverses ranger-base-q114b's blessing exactly here")
	}
	for _, want := range []string{"skills", AgentsSkillsPath, "codex", "GLOBAL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q — the operator needs to see WHICH global root this tree fell under and WHICH runtime declared it", err, want)
		}
	}
	// And nothing was written: a refusal that still leaves the tree behind
	// is the mode-file guard's own distinction (refuse, don't sweep) read
	// backwards.
	if _, statErr := os.Stat(AgentsSkillsDir(home)); statErr == nil {
		t.Error("the render refused but the tree exists anyway — a refusal must mean no write, not a write it disclaims")
	}

	// ARM 1b: the launch's own path, not just the writer's. RenderSkillsFor
	// is what herdrback.go calls, and a guard added to EITHER layer would
	// have to red something here.
	ag := loadTestAgent(t, skillsHomePID)
	for _, name := range []string{"codex", "grok"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("%s is not a built-in: %v", name, err)
		}
		if rt.SkillsRoot == "" {
			t.Fatalf("%s no longer discovers skills from the cwd — this arm is about the cwd surface", name)
		}
		if _, err := a.RenderSkillsFor(ag, rt, home); err == nil {
			t.Errorf("%s at a session dir of $HOME was NOT refused — RenderSkillsFor must reach the same guard RenderAgentsSkills does", name)
		}
	}
}

// ARM 2: bob's OWN posse-owned root, BobSkillsRoot, at the identical input.
// Unlike codex and grok, bob's declaration covers its own render directly —
// `.bob/plugins/posse/skills` matches `.bob/plugins/*/skills` with `posse`
// standing for the wildcard segment — so this is not the "any runtime"
// clause, it is bob refusing itself.
func TestQABobsSkillsTreeAtTheHomeRefusesUnderItsOwnGlobalRoot(t *testing.T) {
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

	if _, err := a.RenderSkillsFor(ag, rt, home); err == nil {
		t.Error("bob's own skills tree at a session dir of $HOME was NOT refused — BobSkillsRoot sits inside bob's own declared .bob/plugins/*/skills")
	}
	if _, statErr := os.Stat(filepath.Join(home, ".bob", "plugins", "posse", "skills", "dataviz", "SKILL.md")); statErr == nil {
		t.Errorf("bob's launch wrote a skills tree at %s despite the refusal", BobSkillsRoot)
	}
	// The sibling guard, at the identical input, called the way herdrback.go
	// never gets to call it now (the skills refusal above stops the launch
	// first — herdrback.go's order is skills, then mode). Still pinned
	// directly so a landing that dropped THIS guard would not hide behind
	// the one above: either refusal is independently sufficient.
	if _, err := a.RenderPersonaModeFor(ag, rt, home); err == nil {
		t.Error("the mode file at a session dir of $HOME was written — ranger-base-se81d's guard is the other half of this pair and must refuse on its own")
	}
}

// ARM 3: the one fact both refusals rest on, inside the tree, asked with
// the PRODUCTION PREDICATE (ranger-base-szxlx's lesson, carried over from
// the pin this replaces) — not a first segment or a hand-rolled prefix
// test. Every entry any built-in declares in SkillsGlobalRoots must cover
// SOME real render target (the FLOOR: a sweep over nothing is not an
// answer), and skillsGlobalRootCovers — the function refuseSkillsGlobalWrite
// actually calls — must say yes to it. The CONTROL is the negative half:
// a path that is NOT under any declared root must not be refused, so this
// arm cannot pass by reading every path as covered.
func TestQAEveryDeclaredSkillsGlobalRootRefusesTheCwdRender(t *testing.T) {
	t.Parallel()
	// The home is a real directory because the resolution inside
	// refuseSkillsGlobalWrite follows symlinks over the deepest existing
	// ancestor — on darwin a `/var` home is a link, and a textual test reads
	// it as outside itself.
	home := t.TempDir()

	// By VALUE, not `&builtinRuntimes[i]`: taking the address of an element
	// puts the table in cmd/testparallel's WRITTEN set, which makes every
	// `t.Parallel` test that merely reads it uncleared (ranger-base-q114b's
	// own finding, the reason this loop is still shaped this way).
	declared := 0
	for _, rt := range builtinRuntimes {
		for _, pattern := range rt.SkillsGlobalRoots {
			declared++
			// A render target that matches the pattern literally — a `*`
			// segment stands for one directory, here spelled "x" — is the
			// CONTROL: the predicate must say yes to the exact shape it was
			// written for.
			target := filepath.Join(home, filepath.FromSlash(strings.ReplaceAll(pattern, "*", "x")))
			if !skillsGlobalRootCovers(home, pattern, target) {
				t.Errorf("control: runtime %s declares %q, and %s does not read as covered by it — the sweep below cannot catch what it is for", rt.Name, pattern, AbbrevHome(target))
				continue
			}
			if err := refuseSkillsGlobalWrite("p", home, strings.ReplaceAll(pattern, "*", "x"), home); err == nil {
				t.Errorf("runtime %s declares %q as a GLOBAL skill root, and a cwd render whose root IS that path was not refused at a session dir of $HOME (ranger-base-6i5tr, ADR 0007 §6e)", rt.Name, pattern)
			}
		}
	}
	if declared == 0 {
		t.Fatal("no built-in runtime declares a SkillsGlobalRoots entry, so the sweep above compared nothing — bob has declared six since ADR 0007 §6e (ranger-base-6i5tr)")
	}

	// The negative half of the control: a root with no relation to any
	// declared global root is not refused, so `refuseSkillsGlobalWrite` is
	// not a function that says yes to everything.
	if err := refuseSkillsGlobalWrite("p", home, "project/.agents/skills", home); err != nil {
		t.Errorf("control: a cwd root of %q covers no declared global root and must NOT be refused — the guard is answering yes to everything: %v", "project/.agents/skills", err)
	}
	// And the ordinary case this whole file exists beside: a session dir
	// that is NOT the home is never checked against a home-relative root at
	// all, whatever the rendered root is named.
	if err := refuseSkillsGlobalWrite("p", t.TempDir(), AgentsSkillsPath, home); err != nil {
		t.Errorf("control: a session dir that is not $HOME must not be refused by a home-relative global root: %v", err)
	}
}
