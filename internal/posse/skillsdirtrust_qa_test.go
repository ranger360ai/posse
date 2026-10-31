//go:build posse_arm3

package posse

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	sources := map[string]string{}
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		sources[n] = string(b)
	}
	if len(sources) == 0 {
		t.Fatal("the census read no production sources — a green here would mean nothing")
	}
	found, err := scanTrustTableWrites(sources)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range found {
		t.Errorf("%s writes grok's trust table (%s) — ADR 0007 §6f rules that grant the OPERATOR's, once per checkout, because it is box-wide, outlives the session, and opens an undeclared repo→box permissions channel (ranger-base-1g8gf). If that is now the decision, amend §6f and this test together.\n  names the table at: %s", w.Where, w.Call, w.Names)
	}
}

// trustTableWrite is one finding: a scope that both NAMES grok's trust table
// and WRITES, with the two halves quoted so the reader can see which line
// the census read as which.
type trustTableWrite struct {
	Where string // "file.go:func seedTrust" or "file.go (package scope)"
	Call  string // the write it makes
	Names string // the string literal, or the package-level name, that spells the table
}

// trustTableWriteCalls is what "writes" means here: calls that put bytes on
// disk, by the only spelling that cannot be renamed out of the census — the
// standard library's, plus this package's own two write helpers.
//
// NO SEEDER NAME IS IN THIS LIST, and that is ranger-base-0npto's second
// correction. The pin this replaced carried the literal "SeedGrokTrust", and
// the mutant its close reported as red was a func CALLED SeedGrokTrust — a
// rig whose fixture came from the list under test. A writer only has to be
// called something else.
var trustTableWriteCalls = map[string]bool{
	"os.WriteFile":     true,
	"os.Create":        true,
	"os.CreateTemp":    true,
	"os.OpenFile":      true,
	"os.Rename":        true,
	"os.Symlink":       true,
	"os.Link":          true,
	"os.Truncate":      true,
	"ioutil.WriteFile": true,
	"WriteExecutable":  true,
	"writeJSONInPlace": true,
}

// trustTableSpellings are the two ways a writer can name the thing: the
// file's basename, and the key syntax of its CONTENT (`[folders."<path>"]`,
// which is what grok writes when the operator accepts the prompt —
// qmTrustedGrokHome has the capture). The second is here so that building
// the path out of variables does not hide a func that plainly writes the
// table's body.
var trustTableSpellings = []string{"trusted_folders", "[folders."}

// scanTrustTableWrites decides per SCOPE — one function, or a file's
// package-level declarations taken together — rather than per line.
//
// The per-line census this replaced required the table's name and a write
// verb on the SAME source line, so a seeder written the ordinary way (path
// into a variable, write on the next line) was invisible to it; so was a
// path assembled from two literals. MEASURED 2026-10-31 (ranger-base-0npto):
// of three seeders appended to skillsdirtrust.go one at a time, the per-line
// census killed one — the one named after a literal in its own list — and
// the two ordinary ones survived. The fixtures are planted in
// TestQATheTrustTableCensusCanStillSayNo, so the measurement is a test and
// not a transcript.
//
// Comments are not parsed (parser mode 0), so the prose in this file and in
// skillsdirtrust.go — which names the table repeatedly, as it must — is
// invisible to the census rather than filtered out of it by a `//` prefix
// test that an indented or trailing comment would walk past.
//
// All the string literals in a scope are joined before matching, which is
// what catches `"trusted_" + "folders.toml"`, and a package-level string
// whose own value names the table makes every scope that references it a
// namer, which is what stops the hoisting of the path into a const from
// being an evasion.
func scanTrustTableWrites(sources map[string]string) ([]trustTableWrite, error) {
	fset := token.NewFileSet()
	names := make([]string, 0, len(sources))
	for n := range sources {
		names = append(names, n)
	}
	sort.Strings(names)

	parsed := make([]*ast.File, 0, len(names))
	for _, n := range names {
		f, err := parser.ParseFile(fset, n, sources[n], 0)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		parsed = append(parsed, f)
	}

	// Pass 1: package-level identifiers whose own value spells the table.
	tableIdents := map[string]bool{}
	for _, f := range parsed {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
				continue
			}
			for _, sp := range gd.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, v := range vs.Values {
					if _, hit := trustTableSpelling(joinStringLits(v)); hit {
						for _, id := range vs.Names {
							tableIdents[id.Name] = true
						}
					}
				}
			}
		}
	}

	// Pass 2: every scope, its namers and its writes.
	var found []trustTableWrite
	for i, f := range parsed {
		file := names[i]
		var pkgLevel []ast.Node
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				pkgLevel = append(pkgLevel, d)
				continue
			}
			if fn.Body == nil {
				continue
			}
			found = append(found, trustTableWritesIn(file+":func "+fn.Name.Name, tableIdents, fn.Body)...)
		}
		if len(pkgLevel) > 0 {
			found = append(found, trustTableWritesIn(file+" (package scope)", tableIdents, pkgLevel...)...)
		}
	}
	return found, nil
}

// trustTableWritesIn is the one decision, over one scope: does it name the
// table at all, and does it write.
func trustTableWritesIn(where string, tableIdents map[string]bool, scope ...ast.Node) []trustTableWrite {
	var namer string
	var writes []string
	for _, n := range scope {
		if lit, hit := trustTableSpelling(joinStringLits(n)); hit && namer == "" {
			namer = strconv.Quote(lit)
		}
		ast.Inspect(n, func(node ast.Node) bool {
			switch x := node.(type) {
			case *ast.Ident:
				if tableIdents[x.Name] && namer == "" {
					namer = "the package-level " + x.Name
				}
			case *ast.CallExpr:
				if c := calledName(x.Fun); trustTableWriteCalls[c] {
					writes = append(writes, c)
				}
			}
			return true
		})
	}
	if namer == "" || len(writes) == 0 {
		return nil
	}
	return []trustTableWrite{{Where: where, Call: strings.Join(writes, ", "), Names: namer}}
}

// trustTableSpelling reports whether text names the table, and returns the
// joined text so the finding can quote what it read.
func trustTableSpelling(text string) (string, bool) {
	for _, sp := range trustTableSpellings {
		if strings.Contains(text, sp) {
			return text, true
		}
	}
	return "", false
}

// joinStringLits concatenates every string literal under n, in source order
// and with nothing between them, so a path split across a `+` chain reads as
// the path it builds.
func joinStringLits(n ast.Node) string {
	var b strings.Builder
	ast.Inspect(n, func(node ast.Node) bool {
		if l, ok := node.(*ast.BasicLit); ok && l.Kind == token.STRING {
			if s, err := strconv.Unquote(l.Value); err == nil {
				b.WriteString(s)
			} else {
				b.WriteString(l.Value)
			}
		}
		return true
	})
	return b.String()
}

// calledName is "os.WriteFile" or "WriteExecutable" — the spelling the
// census keys on. A call through a value (a field, an element, a func
// returned by a call) answers "", which the list would not match anyway.
func calledName(fun ast.Expr) string {
	switch x := fun.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return id.Name + "." + x.Sel.Name
		}
		return x.Sel.Name
	}
	return ""
}

// The two-way half of the census above, and the permanent record of the
// mutation run that condemned its predecessor (ranger-base-0npto). It reads
// PLANTED sources rather than the tree, so no unrelated landing can red it
// and nothing derives it — it reds only when the reader itself changes, and
// whoever changes the reader is in this file.
//
// Rows 2-4 are the three seeders the per-line census let through, in the
// spellings MEASURED against it on 2026-10-31. Rows 5-7 are the other half:
// a census that answered YES to every file mentioning the table would pass
// rows 1-4 and hold nothing, so the reads and the unrelated writer are here
// too.
func TestQATheTrustTableCensusCanStillSayNo(t *testing.T) {
	t.Parallel()
	const head = "package posse\n\nimport \"os\"\n\n"
	for _, tc := range []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "the shipped shape: it names the table in prose and writes nothing",
			src: head + `// Where: "~/.grok/trusted_folders.toml — OPERATOR-owned, and posse never
// writes it", and os.WriteFile is named in this comment too.
func trustWhere() string { return "the operator's own table" }
`,
		},
		{
			name: "a seeder on one line, which is all the per-line census ever caught",
			src: head + `func seedOnOneLine(home string) error {
	return os.WriteFile(home+"/.grok/trusted_folders.toml", []byte("x"), 0o644)
}
`,
			want: true,
		},
		{
			name: "the path on one line and the write on the next",
			src: head + `func seedTrustForDir(home string) error {
	p := home + "/.grok/trusted_folders.toml"
	body := "[folders]\ntrusted = true\n"
	return os.WriteFile(p, []byte(body), 0o644)
}
`,
			want: true,
		},
		{
			name: "the name split across a + chain",
			src: head + `func seedTrustSplit(home string) error {
	p := home + "/.grok/" + "trusted_" + "folders.toml"
	return os.WriteFile(p, []byte("x"), 0o644)
}
`,
			want: true,
		},
		{
			name: "the path hoisted into a package-level const",
			src: head + `const groksFolderTable = ".grok/trusted_folders.toml"

func seedHoisted(home string) error {
	return os.WriteFile(home+"/"+groksFolderTable, []byte("x"), 0o644)
}
`,
			want: true,
		},
		{
			name: "the table's CONTENT, written to a path built from variables",
			src: head + `func seedByContent(dir, key string) error {
	return os.WriteFile(dir+"/"+trustFile(key), []byte("[folders.\""+key+"\"]\ntrusted = true\n"), 0o644)
}

func trustFile(string) string { return "t.toml" }
`,
			want: true,
		},
		{
			name: "READING the table is the whole mechanism and must stay green",
			src: head + `func readTrust(home string) ([]byte, error) {
	return os.ReadFile(home + "/.grok/trusted_folders.toml")
}
`,
		},
		{
			name: "a writer that has nothing to do with the table",
			src: head + `func writeSomethingElse(p string) error {
	return os.WriteFile(p+"/.grok/lsp.json", []byte("{}"), 0o644)
}
`,
		},
		{
			name: "the table named in one func and a write in another",
			src: head + `func trustPath(home string) string { return home + "/.grok/trusted_folders.toml" }

func writeNote(p string) error { return os.WriteFile(p+"/note", nil, 0o644) }
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			found, err := scanTrustTableWrites(map[string]string{"planted.go": tc.src})
			if err != nil {
				t.Fatal(err)
			}
			if got := len(found) > 0; got != tc.want {
				t.Errorf("the census found %v, want %v: %+v", got, tc.want, found)
			}
			for _, w := range found {
				if !strings.Contains(w.Where, "planted.go") || w.Call == "" || w.Names == "" {
					t.Errorf("a finding has to say where and what it read: %+v", w)
				}
			}
		})
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
