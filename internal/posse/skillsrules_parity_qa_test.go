//go:build posse_arm3

package posse

// Runtime parity, fifth measurement lane (ranger-base-qm6e): the two
// dimensions the four areas orphaned — INSTRUCTION & SKILL SURFACES.
// ADR 0017 §2 verdicts; ADR 0007 for the binding, ADR 0013 §4 for the
// rulebooks.
//
// Two halves, and they are instrumented differently on purpose:
//
//   - The HERMETIC half drives production (RenderSkillsFor, CheckParity,
//     CreateSession) once per runtime DECLARATION — the three built-ins
//     plus both template-only shapes — because the whole class of bug the
//     parity track keeps finding is a pin that drives one runtime and
//     reads as if it covered all of them (ranger-base-unzn's busy-key
//     split, ranger-base-ntsz's third arm). A fourth runtime is onboarded
//     here by adding a row.
//
//   - The LIVE half asks the installed CLIs what they actually loaded, and
//     is gated on RHQ_PARITY_LIVE=1 because it needs codex, grok and
//     claude on PATH. Every probe it uses is free — `codex debug
//     prompt-input`, `grok inspect`, `claude plugin details` — so it can be
//     re-run on a CLI upgrade without spending a turn. The billed
//     invocation turns behind the matrix on ranger-base-qm6e were run by
//     hand; what is automated here is DISCOVERY, which is the half that
//     drifts when a CLI updates.
//
// Measured 2026-08-29 at claude 2.1.251 / codex-cli 0.150.1 / grok 1.0.5.
//
// WHAT IS NOT AUTOMATED HERE, and how it was measured, so the next person
// does not re-derive it (full matrix on ranger-base-qm6e):
//
//   - INVOCATION, as opposed to discovery. claude and codex each spent one
//     turn opening two bound skills and returning tokens that exist only in
//     the skill BODIES, with a third registry skill the PID does not name
//     correctly reported MISSING. grok could not: its account answers 402,
//     so its invocation cell is UNKNOWN (ADR 0017 §2's loud state) and the
//     one command that closes it is on ranger-base-u4y0.
//   - HOME-SCOPE rulebooks. `~/.codex/AGENTS.md` and `~/.grok/rules/*.md`
//     were both measured loading, with a redirected CODEX_HOME / HOME so
//     nothing was written into the operator's own config. claude declares
//     no user-scope rulebook at all and could not be measured the same way:
//     both CLAUDE_CONFIG_DIR and a redirected HOME cost it its keychain
//     auth ("Not logged in"), and planting a file in the real ~/.claude
//     would change what every session on the box reads.
//   - THE CASE-FOLD. This volume is case-insensitive (measured), so grok's
//     declared Agents.md/AGENTS.md and Claude.md/CLAUDE.md are one file
//     each here and no probe on this box can separate them. Hence
//     qmFoldCase.

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// qmSkill writes an Agent-Skills dir whose BODY carries a token no prompt
// mentions. Body, not frontmatter: the description reaches the model in
// every runtime's skills listing, so a token there proves discovery only.
// A token from the body proves the file was opened.
func qmSkill(t *testing.T, dir, name, token string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: the " + name + " probe skill\n---\n" +
		"When you use this skill, reply with exactly " + token + "\n"
	if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// qmSurface is what one runtime declaration is expected to do with a
// two-skill binding: where the binding lands, and what {skills} renders.
type qmSurface struct {
	runtime string
	yaml    string // template-only runtimes/<name>.yaml body; "" for a built-in
	// cwd: the binding materializes under <session dir>/.agents/skills.
	// flag: it materializes in the rendered tree and {skills} names it.
	// none: there is no surface and the launch refuses.
	shape string
	flag  string // substring {skills} must render to under shape "flag"
}

// THE SKILLS TABLE. One row per runtime DECLARATION, not per built-in:
// `skills_flag:` and a bare yaml are two different production paths and
// each has to be driven, or the surface-refusal arm is held by nothing.
var qmSurfaces = []qmSurface{
	{runtime: "claude", shape: "flag", flag: "--plugin-dir"},
	{runtime: "codex", shape: "cwd"},
	{runtime: "grok", shape: "cwd"},
	// A template-only runtime pointed at the SAME rendered tree claude
	// gets. ADR 0007's first shape: what sits inside it is the universal
	// Agent-Skills layout, the plugin.json is inert to anything that does
	// not read it.
	{runtime: "tmplflag", yaml: "command: tmpl --sys {file} {skills}\nskills_flag: --skills %s\n", shape: "flag", flag: "--skills "},
	// The cwd shape, declared in yaml. ADR 0017 §4 listed `skills_cwd:` as
	// NOT declarable ("a yaml runtime is either skills_flag: or no
	// surface"); it has since shipped, and a row is how that stops being
	// prose (runtime.go's skillsCwdDecl branch).
	{runtime: "tmplcwd", yaml: "command: tmpl --sys {file}\nskills_cwd: true\n", shape: "cwd"},
	// The same yaml with both keys left out: no surface at all.
	{runtime: "tmplnone", yaml: "command: tmpl --sys {file}\n", shape: "none"},
}

// §1 — the binding materializes for every declaration, with MORE THAN ONE
// skill, and a skill the PID does not name is not bound. Two skills
// because the entire parity evidence for this dimension was one skill on
// one day (rangerhq-74c6), and one skill cannot see an ordering or a
// truncation bug.
func TestQASkillSurfacePerRuntimeDeclaration(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.SkillsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	qmSkill(t, a.SkillsDir(), "qm-alpha", "TOKALPHA")
	qmSkill(t, a.SkillsDir(), "qm-beta", "TOKBETA")
	// The negative control: present in the registry, absent from the PID.
	// Binding is additive (ADR 0007 §4) but it is not "everything in
	// RHQ_HOME/skills" — a surface that hands over the whole registry
	// would pass every presence assertion above.
	qmSkill(t, a.SkillsDir(), "qm-unbound", "TOKUNBOUND")
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "prober.md"),
		[]byte("---\nname: prober\nskills: [qm-alpha, qm-beta]\n---\nYou are prober.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ag, err := a.LoadAgent("prober")
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range qmSurfaces {
		t.Run(s.runtime, func(t *testing.T) {
			if s.yaml != "" {
				if err := os.WriteFile(filepath.Join(a.RuntimesDir(), s.runtime+".yaml"), []byte(s.yaml), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			rt, err := a.LoadRuntime(s.runtime)
			if err != nil {
				t.Fatal(err)
			}
			p := a.CheckParity(ag, rt, CageShims, TierStandard)
			key := "skills: qm-alpha, qm-beta"

			if s.shape == "none" {
				// The one failure ADR 0007 spends a refusal on: a persona
				// that launches believing it has a skill it does not.
				want := key + " — " + s.runtime + " has no per-session skill surface"
				if len(p.Degraded) != 1 || p.Degraded[0] != want {
					t.Fatalf("must degrade with %q, got %v", want, p.Degraded)
				}
				err := b.CreateSession(NewSessionOpts{Name: "qm-" + s.runtime, Agent: "prober", Runtime: s.runtime, Dir: t.TempDir()})
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("the launch must refuse, got %v", err)
				}
				return
			}

			if len(p.Degraded) != 0 {
				t.Fatalf("binding must be realizable: %v", p.Degraded)
			}
			dir := t.TempDir()
			if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v %s", err, out)
			}
			written, err := a.RenderSkillsFor(ag, rt, dir)
			if err != nil {
				t.Fatal(err)
			}
			var root string
			switch s.shape {
			case "cwd":
				root = AgentsSkillsDir(dir)
				if written != root {
					t.Errorf("cwd surface must land in the session dir: %s", written)
				}
				if !strings.Contains(p.Realized[key].Detail, AgentsSkillsPath) {
					t.Errorf("parity must name the cwd surface: %q", p.Realized[key].Detail)
				}
				if flag, _ := rt.SkillsText(ag.SkillsStateDir, ag.Skills); flag != "" {
					t.Errorf("a cwd runtime types no skills flag, got %q", flag)
				}
			case "flag":
				root = filepath.Join(written, "skills")
				if written != ag.SkillsStateDir {
					t.Errorf("flag surface must be the rendered tree: %s", written)
				}
				flag, ok := rt.SkillsText(ag.SkillsStateDir, ag.Skills)
				if !ok || !strings.Contains(flag, s.flag) || !strings.Contains(flag, written) {
					t.Errorf("{skills} must render %q at %s, got %q", s.flag, written, flag)
				}
				if !strings.Contains(p.Realized[key].Detail, s.flag) {
					t.Errorf("parity must name the flag: %q", p.Realized[key].Detail)
				}
			}
			// MULTIPLE, not one — and only what the PID named.
			for _, name := range []string{"qm-alpha", "qm-beta"} {
				if _, err := os.Stat(filepath.Join(root, name, "SKILL.md")); err != nil {
					t.Errorf("%s must be bound at %s: %v", name, root, err)
				}
			}
			if _, err := os.Lstat(filepath.Join(root, "qm-unbound")); err == nil {
				t.Errorf("qm-unbound is in the registry but not in the PID — it must not be bound")
			}
			// The env exit hatch rides on every runtime, surface or not
			// (ADR 0007 §2): it is what a persona reads when the flag is
			// not there.
			mustCreate(t, b, NewSessionOpts{Name: "qm-" + s.runtime, Agent: "prober", Runtime: s.runtime, Dir: dir})
			log := calls(t, fake)
			if !strings.Contains(log, "--env RHQ_SKILLS=qm-alpha") {
				t.Errorf("RHQ_SKILLS must ride:\n%s", log)
			}
		})
	}
}

// §2 — the rendered tree is a claude-PLUGIN-shaped dir whose name leaks
// into every flag runtime's command line (ADR 0017 §3 notes and accepts
// it). Accepted is not the same as unmeasured: what has to hold is that
// the tree a NON-claude runtime is handed is the universal Agent-Skills
// layout, so reading it is not a coincidence.
func TestQARenderedTreeIsUniversalAgentSkills(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	if err := os.MkdirAll(a.SkillsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	qmSkill(t, a.SkillsDir(), "qm-alpha", "TOKALPHA")
	qmSkill(t, a.SkillsDir(), "qm-beta", "TOKBETA")
	dir, err := a.RenderClaudeSkills("prober", []string{"qm-alpha", "qm-beta"})
	if err != nil {
		t.Fatal(err)
	}
	// The leak, pinned so a rename is a decision and not an accident.
	if filepath.Base(dir) != "claude" || filepath.Base(filepath.Dir(dir)) != "prober" {
		t.Errorf("the tree is state/skills/<persona>/claude (ADR 0017 §3): %s", dir)
	}
	// Universal half: <root>/<name>/SKILL.md resolves, with frontmatter
	// carrying the two required Agent-Skills keys. This is all any of the
	// three CLIs needs; nothing here is claude-specific.
	for _, name := range []string{"qm-alpha", "qm-beta"} {
		b, err := os.ReadFile(filepath.Join(dir, "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		front, _ := agentFrontmatter(string(b))
		if yamlGetLines(front, "name") == "" || yamlGetLines(front, "description") == "" {
			t.Errorf("%s: name+description are the two required keys (codex drops a skill without a description)", name)
		}
	}
	// claude-specific half: exactly one file, and it is inert to a reader
	// that does not know it.
	mb, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("plugin.json: %v", err)
	}
	var man pluginManifest
	if err := json.Unmarshal(mb, &man); err != nil || man.Name != "posse-prober" {
		t.Errorf("plugin.json must name posse-<persona>: %s %v", mb, err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var top []string
	for _, e := range ents {
		top = append(top, e.Name())
	}
	sort.Strings(top)
	if strings.Join(top, ",") != ".claude-plugin,skills" {
		t.Errorf("a non-claude reader sees exactly one foreign entry: %v", top)
	}
}

// §2b — THE READER, not the layout. §2 above asserts that
// <root>/<name>/SKILL.md resolves, and that assertion was green on the day
// grok installed this very tree and listed Skills (0): os.Stat follows a
// symlink, so a pin written with the Go stdlib's defaults measures a
// dereferencing reader whatever the tree holds (ranger-base-65rc). What has
// to hold is stronger and is what a plugin loader that does not follow a
// link out of its root actually does — walk the tree, refuse every symlink,
// and still find every bound skill whole.
//
// The fixture is the shape ADR 0007 §1 licenses and the operator's own
// registry uses: the registry entry is ITSELF a symlink to where the skill
// lives, and inside it a reference file that is a symlink too. Both are
// links this render has to resolve rather than reproduce — and both are the
// control, since a rig whose fixture had no link to lose could not tell a
// copy from the symlinks it replaced.
func TestQARenderedTreeNeedsNoSymlinkFollowed(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	if err := os.MkdirAll(a.SkillsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// qm-alpha lives elsewhere and is reached through a symlinked registry
	// entry; it carries a nested dir, a script whose execute bit is part of
	// the skill (ADR 0007: skills that ship scripts run inside the cage like
	// anything else), and a reference file that is itself a link.
	elsewhere := t.TempDir()
	alpha := qmSkill(t, elsewhere, "qm-alpha", "TOKALPHA")
	if err := os.Symlink(alpha, a.SkillPath("qm-alpha")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(alpha, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "leases.md")
	if err := os.WriteFile(outside, []byte("REFBODY\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(alpha, "references", "leases.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alpha, "run.sh"), []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	qmSkill(t, a.SkillsDir(), "qm-beta", "TOKBETA")
	qmSkill(t, a.SkillsDir(), "qm-unbound", "TOKUNBOUND")

	dir, err := a.RenderClaudeSkills("prober", []string{"qm-alpha", "qm-beta"})
	if err != nil {
		t.Fatal(err)
	}

	// The reader: a walk that refuses to leave the tree through a link, the
	// way grok's plugin loader was measured to behave. It reads nothing
	// through os.Stat — every decision is made on the entry's own type.
	var links []string
	files := map[string]string{}
	modes := map[string]fs.FileMode{}
	if err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return rerr
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			links = append(links, rel)
		case d.Type().IsRegular():
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			fi, rerr := d.Info()
			if rerr != nil {
				return rerr
			}
			files[filepath.ToSlash(rel)] = string(b)
			modes[filepath.ToSlash(rel)] = fi.Mode().Perm()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(links) > 0 {
		t.Errorf("a reader that does not dereference finds nothing behind these: %v", links)
	}

	// Every bound skill, whole: the body token the fixture hides in the
	// SKILL.md, the nested reference that was a link out of the tree, and
	// the script's execute bit.
	for name, want := range map[string]string{
		"skills/qm-alpha/SKILL.md": "TOKALPHA",
		"skills/qm-beta/SKILL.md":  "TOKBETA",
	} {
		if !strings.Contains(files[name], want) {
			t.Errorf("%s must carry the skill body (%s): %q", name, want, files[name])
		}
	}
	if got := files["skills/qm-alpha/references/leases.md"]; got != "REFBODY\n" {
		t.Errorf("a reference file behind a symlink must be copied through: %q", got)
	}
	if m := modes["skills/qm-alpha/run.sh"]; m&0o111 == 0 {
		t.Errorf("a skill's script must keep its execute bit: %s", m)
	}
	// The registry is not touched: the flag tree is a render, and
	// RHQ_HOME/skills/<name> is still the operator's own entry, still a
	// symlink to where the skill actually lives (ADR 0007 §1).
	if fi, err := os.Lstat(a.SkillPath("qm-alpha")); err != nil || fi.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("the registry entry must stay as the operator wrote it: %v %v", fi, err)
	}
	// And nothing but the skill came with it: the registry holds a third
	// skill the PID never named.
	if _, err := os.Lstat(filepath.Join(dir, "skills", "qm-unbound")); err == nil {
		t.Error("the copy must be of the bound names, not of the registry")
	}
}

// §2c — a skill dir that cannot be copied refuses the launch rather than
// binding half of one. The shape reachable through the operator's own
// registry is a link that points back up its own tree, which a copy that
// followed it would never finish. ADR 0007 §3 spends its refusal on a
// persona that launches believing it has a skill it does not, and half a
// skill is that persona.
func TestQAUncopyableSkillRefusesTheLaunch(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	if err := os.MkdirAll(a.SkillsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	loop := qmSkill(t, a.SkillsDir(), "qm-loop", "TOKLOOP")
	if err := os.Symlink(loop, filepath.Join(loop, "self")); err != nil {
		t.Fatal(err)
	}
	_, err := a.RenderClaudeSkills("prober", []string{"qm-loop"})
	if err == nil || !strings.Contains(err.Error(), "finite tree") {
		t.Errorf("a self-referencing skill must refuse: %v", err)
	}

	// A dangling link INSIDE a skill is the one shape that does not refuse:
	// under the symlinks this render replaced it reached the session as an
	// entry resolving to nothing, and it still does. The skill itself must
	// still arrive.
	qmSkill(t, a.SkillsDir(), "qm-gap", "TOKGAP")
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), filepath.Join(a.SkillPath("qm-gap"), "missing.md")); err != nil {
		t.Fatal(err)
	}
	dir, err := a.RenderClaudeSkills("prober", []string{"qm-gap"})
	if err != nil {
		t.Fatalf("a dangling entry inside a skill must not refuse: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "skills", "qm-gap", "SKILL.md")); err != nil || !strings.Contains(string(b), "TOKGAP") {
		t.Errorf("the skill must still arrive whole: %v %q", err, b)
	}
	if _, err := os.Lstat(filepath.Join(dir, "skills", "qm-gap", "missing.md")); err == nil {
		t.Error("an entry that resolves to nothing must not be reproduced")
	}
}

// §3 — NATIVE RULEBOOKS, the declaration side. `runtime check` prints
// NativeRules verbatim and posse consumes it nowhere else (ADR 0017 §3,
// display-by-design), so the only thing that can be wrong with it is that
// it disagrees with the CLI — in either direction. Over-declaring makes an
// operator write guidance into a file nothing reads; UNDER-declaring puts a
// file in front of the model that the grid does not name, which is the
// direction ADR 0013 §4 cares about.
//
// The pin is the CONSUMER — the rendered `runtime check` rulebooks line,
// read back out of RuntimeCheck's own output — not the slice behind it. A
// slice assertion is green over a display that drops, truncates or
// re-labels the row, and the display IS the whole point of the field: it is
// the only thing that reads it. ranger-base-x7m1.
//
// qmRulebookTruth is what the CLIs were MEASURED to load on 2026-08-29,
// project scope, on a case-insensitive APFS volume. It is a fixture of
// fact: when a CLI updates, run TestQALiveNativeRulesDiscovery and change
// this table, never the assertion.
var qmRulebookTruth = map[string][]string{
	"claude": {".claude/CLAUDE.md", ".claude/rules/*.md", "CLAUDE.local.md", "CLAUDE.md"},
	"codex":  {"AGENTS.md", "AGENTS.override.md", "~/.codex/AGENTS.md"},
	"grok": {".claude/CLAUDE.md", ".claude/rules/*.md", ".cursor/rules/*.md", ".grok/rules/*.md",
		"AGENT.md", "AGENTS.md", "CLAUDE.local.md", "CLAUDE.md", "~/.grok/rules/*.md"},
}

// qmKnownMismatch is every disagreement between a NativeRules declaration
// and what the CLI was measured to do, with the bead that owns it. It is a
// deliberate second table so this test does two jobs at once: it holds the
// open defects to a named owner, and it goes red the day a NEW one
// appears. When a bead lands, its entry is deleted and the test says so.
// EMPTY since ranger-base-x7m1 landed: claude no longer declares AGENTS.md
// (2.1.251 reads none of it) and both claude and grok now declare
// .claude/CLAUDE.md (both load it). All three built-ins agree with the
// measurement, so every entry here was deleted, which is how this table
// says a fix landed.
var qmKnownMismatch = map[string][]string{}

func TestQANativeRulesDeclarationMatchesMeasurement(t *testing.T) {
	a := checkApp(t)
	for _, name := range []string{"claude", "codex", "grok"} {
		t.Run(name, func(t *testing.T) {
			declared := map[string]bool{}
			for _, f := range qmRenderedRulebooks(t, a, name) {
				declared[qmFoldCase(f)] = true
			}
			loaded := map[string]bool{}
			for _, f := range qmRulebookTruth[name] {
				loaded[qmFoldCase(f)] = true
			}
			var found []string
			for f := range declared {
				if !loaded[f] {
					found = append(found, "over:"+f)
				}
			}
			for f := range loaded {
				if !declared[f] {
					found = append(found, "under:"+f)
				}
			}
			sort.Strings(found)
			known := append([]string(nil), qmKnownMismatch[name]...)
			sort.Strings(known)
			if strings.Join(found, " ") == strings.Join(known, " ") {
				if len(known) > 0 {
					t.Logf("%s: known-open, owned by a bead: %v (over: declared, not loaded — an operator writes into a file nothing reads; under: loaded, not declared — a file talking to the model the grid does not name, ADR 0013 §4)", name, known)
				}
				return
			}
			t.Errorf("%s: the rendered rulebooks line vs measurement changed.\n  now:   %v\n  known: %v\nEither the CLI moved (re-measure with RHQ_PARITY_LIVE=1 and update qmRulebookTruth) or a declaration was edited (update qmKnownMismatch, or delete its entry if the bead landed).", name, found, known)
		})
	}
}

// qmRenderedRulebooks is the rulebooks row as an operator reads it: the
// output of production RuntimeCheck, sliced between the "rulebooks" lead
// and the fixed note that follows it, un-wrapped and split on the comma the
// renderer joins with. Anchoring on the note is what makes the slice exact
// — the row's continuation lines and the note's own lines indent
// identically, so a prefix rule cannot tell them apart.
func qmRenderedRulebooks(t *testing.T, a *App, name string) []string {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // preflight reads a home; never the operator's
	rt, err := a.LoadRuntime(name)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
	_, after, ok := strings.Cut(b.String(), "\n  rulebooks   ")
	if !ok {
		t.Fatalf("%s: `runtime check` printed no rulebooks row at all — the field's only consumer is gone", name)
	}
	row, _, ok := strings.Cut(after, "posse loads none of these")
	if !ok {
		t.Fatalf("%s: the rulebooks row lost the note that ends it; re-anchor this pin", name)
	}
	var files []string
	for _, f := range strings.Split(strings.Join(strings.Fields(row), " "), ",") {
		if f = strings.TrimSpace(f); f != "" {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		t.Fatalf("%s: rulebooks row is empty", name)
	}
	return files
}

// qmFoldCase collapses the spellings that are the same file on a
// case-insensitive volume. grok declares Agents.md AND AGENTS.md, Claude.md
// AND CLAUDE.md; on APFS those are one file each and no probe on this box
// can tell them apart, so the comparison is case-blind and the fact is
// recorded rather than asserted away.
func qmFoldCase(s string) string { return strings.ToLower(s) }

// §4 — LIVE. What did the installed CLI actually load? Free probes only.
//
// Two fixtures, because one cannot answer both questions: codex's
// AGENTS.override.md REPLACES the AGENTS.md beside it, so a tree carrying
// both measures the override and says nothing about AGENTS.md. A shared
// fixture that one probe mutates is how ranger-base-h15 got a wrong answer;
// here it would be a shared fixture whose CONTENTS make two declared rows
// mutually unobservable.
func TestQALiveNativeRulesDiscovery(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_PARITY_LIVE") != "1" {
		t.Skip("set RHQ_PARITY_LIVE=1 with codex and grok installed (claude's arm is a billed turn and stays manual)")
	}
	// Every candidate rulebook this fleet has ever seen named, each with a
	// token of its own, plus files nobody declares. A probe that plants
	// only the declared names cannot find an UNDER-declaration, which is
	// the direction that matters.
	main := qmPlantRulebooks(t, map[string]string{
		"CLAUDE.md": "TOKCLAUDEMD", "CLAUDE.local.md": "TOKCLAUDELOCAL", "AGENTS.md": "TOKAGENTSMD",
		"AGENT.md": "TOKAGENTMD", "AGENTS.local.md": "TOKAGENTSLOCAL",
		".claude/CLAUDE.md": "TOKCLAUDEDOT", ".claude/rules/r.md": "TOKCLAUDERULES",
		".grok/rules/r.md": "TOKGROKRULES", ".cursor/rules/r.md": "TOKCURSORRULES",
		".agents/rules/r.md": "TOKAGENTSRULES", ".codex/AGENTS.md": "TOKCODEXDOT",
		".github/copilot-instructions.md": "TOKCOPILOT", ".cursorrules": "TOKCURSORRC",
		".windsurfrules": "TOKWINDSURFRC", ".clinerules": "TOKCLINERC", ".rules": "TOKDOTRULES",
		"GEMINI.md": "TOKGEMINI", "GROK.md": "TOKGROKMD", "NOTES.md": "TOKNOTES", "RULES.md": "TOKRULES",
	})
	ovr := qmPlantRulebooks(t, map[string]string{
		"AGENTS.md": "TOKPLAIN", "AGENTS.override.md": "TOKOVERRIDE",
	})

	for _, runtime := range []string{"codex", "grok"} {
		t.Run(runtime, func(t *testing.T) {
			loaded, raw := qmDiscovered(t, runtime, main)
			t.Logf("%s loaded: %v", runtime, loaded)
			// The instrument must be able to say YES before a NO means
			// anything.
			if len(loaded) == 0 {
				t.Fatalf("%s probe measured nothing (%d bytes of output)", runtime, len(raw))
			}
			in := func(set []string, f string) bool {
				for _, g := range set {
					if qmFoldCase(g) == qmFoldCase(f) {
						return true
					}
				}
				return false
			}
			for _, undeclared := range []string{"NOTES.md", "RULES.md", "GEMINI.md", ".rules", ".clinerules", ".windsurfrules", ".github/copilot-instructions.md"} {
				if in(loaded, undeclared) {
					t.Errorf("%s loaded %s, which no runtime declares", runtime, undeclared)
				}
			}
			for _, want := range qmRulebookTruth[runtime] {
				switch {
				case strings.HasPrefix(want, "~"):
					continue // home scope: measured with a redirected home, by hand
				case want == "AGENTS.override.md":
					continue // its own fixture, below
				}
				probe := strings.Replace(want, "*.md", "r.md", 1)
				if !in(loaded, probe) {
					t.Errorf("%s no longer loads %s — re-measure and update qmRulebookTruth", runtime, want)
				}
			}
		})
	}

	// codex only: the override replaces rather than adds, which the
	// declaration (a flat comma-joined list in `runtime check`) does not
	// say.
	t.Run("codex-override", func(t *testing.T) {
		if _, err := exec.LookPath("codex"); err != nil {
			t.Skip("no codex on PATH")
		}
		_, raw := qmDiscovered(t, "codex", ovr)
		if !strings.Contains(raw, "TOKOVERRIDE") {
			t.Errorf("codex must load AGENTS.override.md")
		}
		if strings.Contains(raw, "TOKPLAIN") {
			t.Errorf("AGENTS.override.md no longer replaces AGENTS.md — the two are additive now, which changes what the rulebooks line means")
		}
	})
}

// qmPlantRulebooks writes rel→token files into a fresh git repo.
func qmPlantRulebooks(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	for rel, tok := range files {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# fixture\n\nThe token for this file is "+tok+".\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	return repo
}

// qmDiscovered asks one CLI what it loaded in repo and returns the
// repo-relative paths plus the raw output. codex renders the model-visible
// prompt (so the file BODY is the evidence); grok lists the paths it
// discovered (so the PATH is). Both are free and neither bills a turn.
func qmDiscovered(t *testing.T, runtime, repo string) ([]string, string) {
	t.Helper()
	argv := map[string][]string{
		"codex": {"codex", "debug", "prompt-input", "hi"},
		"grok":  {"grok", "inspect"},
	}[runtime]
	if _, err := exec.LookPath(argv[0]); err != nil {
		t.Skipf("no %s on PATH", argv[0])
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v (%d bytes)", argv, err, len(out))
	}
	got := string(out)
	var loaded []string
	err = filepath.Walk(repo, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(repo, p)
		if strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		_, tok, ok := strings.Cut(string(b), "The token for this file is ")
		tok, _, _ = strings.Cut(tok, ".")
		// codex carries the body; grok names the path in its own preferred
		// spelling, which a case-insensitive volume makes ambiguous.
		if (ok && strings.Contains(got, tok)) || strings.Contains(qmFoldCase(got), qmFoldCase(p)+" ") {
			loaded = append(loaded, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(loaded)
	return loaded, got
}

// §5 — LIVE skills. The binding is materialized by PRODUCTION and the CLI
// is asked what it sees. rangerhq-74c6's evidence was one skill; this is
// two, plus a registry entry the PID does not name.
func TestQALiveSkillDiscoveryPerRuntime(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_PARITY_LIVE") != "1" {
		t.Skip("set RHQ_PARITY_LIVE=1 with codex, grok and claude installed")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	if err := os.MkdirAll(a.SkillsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	qmSkill(t, a.SkillsDir(), "qm-alpha", "TOKALPHA")
	qmSkill(t, a.SkillsDir(), "qm-beta", "TOKBETA")
	qmSkill(t, a.SkillsDir(), "qm-unbound", "TOKUNBOUND")
	bound := []string{"qm-alpha", "qm-beta"}

	t.Run("cwd-runtimes", func(t *testing.T) {
		repo := t.TempDir()
		if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		if _, err := a.RenderAgentsSkills(repo, "prober", bound); err != nil {
			t.Fatal(err)
		}
		for _, probe := range [][]string{{"codex", "debug", "prompt-input", "hi"}, {"grok", "inspect"}} {
			if _, err := exec.LookPath(probe[0]); err != nil {
				t.Skipf("no %s on PATH", probe[0])
			}
			cmd := exec.Command(probe[0], probe[1:]...)
			cmd.Dir = repo
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s: %v\n%s", probe[0], err, out)
			}
			got := string(out)
			for _, n := range bound {
				if !strings.Contains(got, n) {
					t.Errorf("%s does not see %s:\n%s", probe[0], n, got)
				}
			}
			if strings.Contains(got, "qm-unbound") {
				t.Errorf("%s sees a skill the PID never bound", probe[0])
			}
		}
		// The links must stay out of the operator's diff (ADR 0007).
		if out, _ := exec.Command("git", "-C", repo, "status", "--porcelain").Output(); len(out) != 0 {
			t.Errorf("the binding must not show in git status: %s", out)
		}
	})

	t.Run("flag-runtimes", func(t *testing.T) {
		dir, err := a.RenderClaudeSkills("prober", bound)
		if err != nil {
			t.Fatal(err)
		}
		// claude: the surface it was built for.
		if _, err := exec.LookPath("claude"); err == nil {
			out, err := exec.Command("claude", "--plugin-dir", dir, "plugin", "details", "posse-prober").CombinedOutput()
			if err != nil {
				t.Fatalf("claude plugin details: %v\n%s", err, out)
			}
			for _, n := range bound {
				if !strings.Contains(string(out), n) {
					t.Errorf("claude does not list %s:\n%s", n, out)
				}
			}
			if strings.Contains(string(out), "qm-unbound") {
				t.Errorf("claude lists a skill the PID never bound:\n%s", out)
			}
		}
		// The bead's question: does a NON-claude CLI read the
		// claude-plugin-shaped tree correctly, or only coincidentally?
		// grok reads claude plugins and will say so itself — and it is the
		// only non-claude CLI on this box that does, which is exactly what
		// makes it the instrument for the reader question claude cannot be
		// asked (it dereferences).
		//
		// validate is NOT the pin and never was: the tree validated,
		// installed, and surfaced ZERO skills, because every skill under it
		// was a symlink out of the plugin root and grok's loader does not
		// follow one (ranger-base-65rc). So the probe goes all the way to
		// `inspect`, which is the first command that answers what the
		// PERSONA would have. HOME is redirected: `plugin install` writes a
		// registry, and never into the operator's own ~/.grok.
		if _, err := exec.LookPath("grok"); err == nil {
			grokHome := t.TempDir()
			grok := func(arg ...string) (string, error) {
				cmd := exec.Command("grok", arg...)
				cmd.Env = append(os.Environ(), "HOME="+grokHome)
				cmd.Dir = grokHome
				out, err := cmd.CombinedOutput()
				t.Logf("grok %s: err=%v\n%s", strings.Join(arg, " "), err, out)
				return string(out), err
			}
			if out, err := grok("plugin", "validate", dir); err != nil {
				t.Errorf("a non-claude CLI must be able to read the rendered tree: %v\n%s", err, out)
			}
			if out, err := grok("plugin", "install", dir, "--trust"); err != nil {
				t.Errorf("grok plugin install: %v\n%s", err, out)
			}
			out, err := grok("inspect")
			if err != nil {
				t.Errorf("grok inspect: %v\n%s", err, out)
			}
			for _, n := range bound {
				if !strings.Contains(out, n) {
					t.Errorf("grok installed the tree and does not see %s — the binding is realized and the persona has nothing:\n%s", n, out)
				}
			}
			if strings.Contains(out, "qm-unbound") {
				t.Errorf("grok sees a skill the PID never bound:\n%s", out)
			}
		}
	})
}
