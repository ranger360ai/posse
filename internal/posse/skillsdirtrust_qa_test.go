//go:build posse_arm3

package posse

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// QA pins for ADR 0007 §6f (ranger-base-kwbh1) — the cwd skills root's
// precondition on the SESSION DIRECTORY.
//
// What they hold, in the order the defect arrives:
//
//  1. the parse, over the bytes grok ACTUALLY prints. The trusted and
//     untrusted fixtures below are verbatim captures from the two-arm
//     measurement of 2026-10-31 (skillsdirtrust.go's header), not strings
//     written to match the parser.
//  2. the three verdicts: an untrusted directory DEGRADES and leaves
//     Realized; an unknown one changes nothing; a trusted one says so.
//     Arm 2 is the whole point of the bead — a realized binding the CLI will
//     not read is ADR 0007 §3's refusal through the accepted path.
//  3. UNKNOWN never degrades. Walling a launch because `grok inspect` would
//     not run refuses on ignorance (ranger-base-9r33), and that is the arm a
//     later "be stricter" edit is most likely to break.
//  4. posse does not WRITE the grant. That is §6f's decision rather than a
//     gap, and a census is what keeps it from being quietly reversed by a
//     bead that reads the degrade line as a TODO.

// The real thing, 2026-10-31, grok 1.0.52 (0920c8169e27): two scratch repos
// with identical content under one fixture $HOME that trusted only the first.
// Trimmed to the Environment block and the Skills count — the two facts the
// probe and the finding turn on.
const (
	grokInspectTrusted = `
  Environment
  └ Version: 1.0.52 [unknown]
  └ CWD: /private/tmp/qa/repoA
  └ Git root: /private/tmp/qa/repoA/
  └ Project trusted: yes

  Permissions
  └ Source: /private/tmp/qa/repoA/.claude/settings.json (settings)
  └ 2 loaded, 0 skipped

  Skills (1)
  └ qm-alpha  project
`
	grokInspectUntrusted = `
  Environment
  └ Version: 1.0.52 [unknown]
  └ CWD: /private/tmp/qa/repoB
  └ Git root: /private/tmp/qa/repoB/
  └ Project trusted: no

  Permissions
  └ Source: (none)
  └ 0 loaded, 0 skipped

  Skills (None)
`
)

func TestQAGrokDirTrustReadsGroksOwnAnswer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		out              string
		err              error
		trusted, unknown bool
		why              string
	}{
		{name: "trusted", out: grokInspectTrusted, trusted: true, why: "Project trusted: yes"},
		{name: "untrusted", out: grokInspectUntrusted, why: "Project trusted: no"},
		// A release that renames or drops the line is UNKNOWN, not
		// untrusted: folding it into "no" would degrade every grok launch on
		// the box the day grok redraws its own output.
		{name: "no such line", out: "  Environment\n  └ Version: 9.9.9\n", unknown: true, why: "printed no `Project trusted:` line"},
		{name: "unrecognised value", out: "  └ Project trusted: maybe\n", unknown: true, why: "neither yes nor no"},
		{name: "would not run", err: errors.New("exec: \"grok\": not found"), unknown: true, why: "would not answer"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := grokDirTrustFrom("/private/tmp/qa/repo", tc.out, tc.err)
			if got.Trusted != tc.trusted || got.Unknown != tc.unknown {
				t.Errorf("trusted=%v unknown=%v, want %v/%v (%s)", got.Trusted, got.Unknown, tc.trusted, tc.unknown, got.Why)
			}
			if !strings.Contains(got.Why, tc.why) {
				t.Errorf("the reading must say what it read: %q does not contain %q", got.Why, tc.why)
			}
			// Every arm names the directory it read, because a reading
			// without one is unactionable on a box with twenty worktrees.
			if !strings.Contains(got.Why, "repo") {
				t.Errorf("the reading must name the directory: %q", got.Why)
			}
		})
	}
}

// grokWithTrust is grok's built-in with the probe swapped for a fixed
// answer: the DECLARATION is production's, only the reading is the test's.
func grokWithTrust(t *testing.T, a *App, answer DirTrust) *Runtime {
	t.Helper()
	rt, err := a.LoadRuntime("grok")
	if err != nil {
		t.Fatal(err)
	}
	if rt.SkillsDirTrust == nil {
		t.Fatal("grok must declare a skills-dir precondition (ADR 0007 §6f) — if it was removed, the launch is back to realizing a binding grok may never read")
	}
	decl := *rt.SkillsDirTrust
	decl.Probe = func(string) DirTrust { return answer }
	rt.SkillsDirTrust = &decl
	return rt
}

func TestQAAnUntrustedSessionDirDegradesASkillsLaunch(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	ag := loadTestAgent(t, "---\nname: prober\nskills: [qm-alpha, qm-beta]\n---\nYou are prober.\n")
	dir := t.TempDir()
	key := "skills: qm-alpha, qm-beta"

	// TRUSTED — realized, and the detail carries the reading. This is the
	// fact ranger-base-9f2o6 had to measure by hand to learn why every grok
	// launch on this box gets its skills.
	rt := grokWithTrust(t, a, DirTrust{Trusted: true, Why: "`grok inspect` reports `Project trusted: yes`"})
	p := a.CheckParityIn(ag, rt, CageSeatbelt, TierStandard, dir)
	if got, ok := p.Realized[key]; !ok || !strings.Contains(got.Detail, "Project trusted: yes") {
		t.Errorf("a trusted dir must stay realized AND print the reading: %+v", p.Realized)
	}
	if degradedAbout(p, "ADR 0007 §6f") != "" {
		t.Errorf("a trusted dir degrades nothing: %q", degradedAbout(p, "ADR 0007 §6f"))
	}

	// UNTRUSTED — the row LEAVES Realized and the launch degrades. The
	// removal is the pin: a Realized row over a root grok will not read is
	// exactly the clean launch with an empty binding that ADR 0007 §3 calls
	// a refusal arriving through the accepted path.
	rt = grokWithTrust(t, a, DirTrust{Why: "`grok inspect` reports `Project trusted: no`"})
	p = a.CheckParityIn(ag, rt, CageSeatbelt, TierStandard, dir)
	if _, ok := p.Realized[key]; ok {
		t.Errorf("an untrusted dir must not read as a realized binding: %+v", p.Realized)
	}
	line := degradedAbout(p, "ADR 0007 §6f")
	if line == "" {
		t.Fatalf("an untrusted dir must degrade the launch: %+v", p.Degraded)
	}
	// The line has to carry all four things, or it is a wall with no door:
	// what was read, what it costs, that posse will not write the grant, and
	// who can.
	for _, want := range []string{
		"Project trusted: no",
		"reads NONE of them",
		"posse never writes it",
		"the OPERATOR accepts grok's folder trust once per checkout",
		"~/.grok/trusted_folders.toml",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the degrade line must say %q:\n%s", want, line)
		}
	}

	// UNKNOWN — the reading prints and the verdict is untouched. posse
	// refuses on a reading, never on ignorance (ranger-base-9r33).
	rt = grokWithTrust(t, a, DirTrust{Unknown: true, Why: "`grok inspect` would not answer"})
	p = a.CheckParityIn(ag, rt, CageSeatbelt, TierStandard, dir)
	if got, ok := p.Realized[key]; !ok || !strings.Contains(got.Detail, "would not answer") {
		t.Errorf("an unknown reading stays realized and still prints: %+v", p.Realized)
	}
	if line := degradedAbout(p, "ADR 0007 §6f"); line != "" {
		t.Errorf("UNKNOWN must never degrade a launch — that refuses on ignorance: %q", line)
	}
}

// Two silences, both deliberate, and both the kind that would otherwise come
// back as a duplicate finding or a line about nobody's launch.
func TestQASkillsDirTrustIsSilentWhereItHasNothingToAdd(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	dir := t.TempDir()
	never := DirTrust{Why: "`grok inspect` reports `Project trusted: no`"}

	// A PID that binds no skills has no binding to be empty.
	bare := loadTestAgent(t, "---\nname: bare\n---\nYou are bare.\n")
	rt := grokWithTrust(t, a, never)
	if p := a.CheckParityIn(bare, rt, CageSeatbelt, TierStandard, dir); degradedAbout(p, "ADR 0007 §6f") != "" {
		t.Errorf("a PID with no skills: is unaffected: %+v", p.Degraded)
	}

	// §6b outranks §6f: an UNPROVEN root has already degraded this launch
	// with the louder sentence, and a second line about a precondition on a
	// root nobody has measured reading is noise over the top of it.
	ag := loadTestAgent(t, "---\nname: prober\nskills: [qm-alpha]\n---\nYou are prober.\n")
	unproven := grokWithTrust(t, a, never)
	unproven.SkillsVerified = ""
	p := a.CheckParityIn(ag, unproven, CageSeatbelt, TierStandard, dir)
	if degradedAbout(p, "UNPROVEN") == "" {
		t.Fatalf("§6b must still degrade an unverified root: %+v", p.Degraded)
	}
	if line := degradedAbout(p, "ADR 0007 §6f"); line != "" {
		t.Errorf("§6f must not stack a second line onto §6b: %q", line)
	}

	// No dir, no reading: CheckParity's dir-independent matrix never invents
	// a cwd, and this must not be the arm that teaches it to.
	if p := a.CheckParity(ag, rt, CageSeatbelt, TierStandard); degradedAbout(p, "ADR 0007 §6f") != "" {
		t.Errorf("the dir-independent matrix says nothing about a directory: %+v", p.Degraded)
	}
}

// Every declaration has to be READABLE — the grid prints What, Where and
// Grant verbatim, and a nil Probe is a permanent UNKNOWN that degrades
// nothing, i.e. a sentence that reads like a guarantee and changes no launch
// (skillsdirtrust.go's reason for having no yaml key).
func TestQAEverySkillsDirTrustDeclarationIsReadable(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	declared := 0
	for _, name := range a.ListRuntimes() {
		rt, err := a.LoadRuntime(name)
		if err != nil || rt.SkillsDirTrust == nil {
			continue
		}
		declared++
		d := rt.SkillsDirTrust
		if d.Probe == nil {
			t.Errorf("%s: a declaration with no Probe is a permanent UNKNOWN, which degrades nothing — it reads like a guarantee and changes no launch", name)
		}
		if rt.SkillsRoot == "" {
			t.Errorf("%s: a precondition on a cwd skills root this runtime does not have", name)
		}
		for field, v := range map[string]string{"What": d.What, "Where": d.Where, "Grant": d.Grant} {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: %s is empty, and `posse runtime check` prints it verbatim", name, field)
			}
		}
		if !strings.Contains(d.Grant, "OPERATOR") {
			t.Errorf("%s: Grant is the remedy a degrade line prints, so it names WHOSE act it is: %q", name, d.Grant)
		}
	}
	if declared == 0 {
		t.Error("no runtime declares a skills-dir precondition — grok's was measured on 2026-10-31 (ADR 0007 §6f) and dropping it returns the launch to realizing a binding grok may never read")
	}
}

// §6f's decision, with teeth. posse names grok's trust table and never
// writes it — the grant is box-wide, outlives the session, and also decides
// whether grok reads the repo's own .claude/settings.json as a permissions
// source (ranger-base-1g8gf). A bead that reads the degrade line as a TODO
// reds here and has to read §6f first.
//
// Scoped to THIS package's own sources, read through a relative path: the
// writer would live beside trust.go if it landed, and a census that climbed
// to the repo root would be a tree-wide pin owing a Makefile door
// (AGENTS.md, "the DOORS are held here instead").
func TestQAPosseNeverWritesGroksTrustTable(t *testing.T) {
	t.Parallel()
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	// os.WriteFile / os.Create / os.OpenFile reaching a path built from the
	// table's name. Matched on the two spellings a writer must use — the
	// basename, or grok's own key syntax — in a line that also names a
	// write, so the prose above (which says the name repeatedly) does not
	// red it.
	writes := []string{"os.WriteFile", "os.Create", "os.OpenFile", "SeedGrokTrust", "ioutil.WriteFile"}
	checked := 0
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for i, ln := range strings.Split(string(b), "\n") {
			code := strings.TrimSpace(ln)
			if strings.HasPrefix(code, "//") || !strings.Contains(code, "trusted_folders") {
				continue
			}
			for _, w := range writes {
				if strings.Contains(code, w) {
					t.Errorf("%s:%d writes grok's trust table (%s) — ADR 0007 §6f rules that grant the OPERATOR's, once per checkout, because it is box-wide, outlives the session, and opens an undeclared repo→box permissions channel (ranger-base-1g8gf). If that is now the decision, amend §6f and this test together:\n  %s", n, i+1, w, code)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("the census read no production sources — a green here would mean nothing")
	}
}

// degradedAbout returns the one Degraded line mentioning want, or "".
func degradedAbout(p Parity, want string) string {
	for _, d := range p.Degraded {
		if strings.Contains(d, want) {
			return d
		}
	}
	return ""
}
