package posse

// QA pins for ranger-base-q8dhm — three ADR rules of pure ABSENCE that the
// September 2026 adherence audit (docs/notes.d/adr-adherence-2026-09.md,
// finding 13) measured true by grep and that no test held, plus a fourth
// added by ranger-base-yi2f8 for the same reason one file over:
//
//   1. ADR 0019 D1 — "Nothing else in posse may acquire a credential except
//      through this seam." One acquirer, internal/posse/credential.go.
//   2. ADR 0017 §3 — the shadow-predicate rule. Per-runtime behaviour keyed
//      on the runtime's NAME is allowed only where the behaviour is
//      inherently that CLI's own state; five such sites are accepted and a
//      sixth must go through the declaration.
//   3. ADR 0013 §4 — the section's closing line, which names a crew seat and
//      reads here as its role (ADR 0012 D2): this is not "the harness closes
//      the bead." The bead is the store of record and the harness is not its
//      writer, so no close verb of the Bd API is reachable from the dispatch
//      path except through a caller a REGISTER names (one row since
//      2026-09-05: ci-watch closing the bead it filed that no session ever
//      claimed, ADR 0013 §4's one ruled exception — ranger-base-8fr2j).
//
// A grep is a measurement of one afternoon. Each of these is the same
// measurement wired to the build, so the day somebody adds the sixth site
// they hear about it from the suite rather than from the next audit.
//
// WHAT A GREEN HERE PROVES, exactly. All three are SYNTACTIC censuses over
// the parsed non-test source of internal/ and cmd/ — no type checker, no
// build. That buys precision on the shapes they name and blindness to the
// shapes they do not:
//
//   - pin 1 sees a token spelled in Go source. A credential acquired by
//     building the path or the argv out of fragments is invisible to it.
//     The complementary pin on the shipped SCRIPTS is credentialgate_qa_test.go.
//   - pin 2 sees an `==`/`!=` against a runtime name. ADR 0017 §3's own
//     register update says the method is incomplete in as many words: "grep
//     found three instances; the consumer-driven parity fixture found the
//     fourth." A map keyed on the name, a switch, a lookup table — all
//     invisible here. This pin holds the REGISTER, not the rule.
//   - pin 4 sees identifiers, over the same parsed tree with comments
//     dropped. A consumer reached through an untyped intermediary — a string
//     handed on, a struct copied whole and read elsewhere — is invisible to
//     it, exactly as for pin 1.
//   - pin 3 resolves a call's receiver only where the source says what it is
//     (a `Bd`-typed receiver, parameter, variable or struct field). Elsewhere
//     it OVER-approximates reachability — every method call reaches every
//     same-named function in the tree — so the reachable set is bigger than
//     the truth and a green is stronger than the graph.
//
// Each pin is shown able to fail by a companion test that runs the same
// scanner over a scratch fixture with the violation planted, plus a clean
// control arm over the same fixture rig so a green is not a rig that could
// never speak (NOTES, "a rig must be shown able to fail").

import (
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

// arRoots are the trees the posse binary is built from. docs/, etc/ and
// scripts/ are prose and shipped scripts; credentialgate_qa_test.go is the
// pin on those.
var arRoots = []string{"internal", "cmd"}

// arFileFloor is the positive witness for every walk below: an assertion of
// absence is satisfied by a scan that opened nothing. 97 non-test .go files
// under those roots on 2026-09-01; the floor is deliberately loose, because
// its job is to catch a walk that read ZERO, not to track the file count.
const arFileFloor = 80

type arFile struct {
	rel  string // slash-separated, relative to the walk root
	file *ast.File
}

// arParse parses every non-test .go file under root/dirs. Comments are
// dropped: every claim below is about code, and prose that names a
// credential or a runtime is prose.
func arParse(t *testing.T, root string, dirs []string) (*token.FileSet, []arFile) {
	t.Helper()
	fset := token.NewFileSet()
	var out []arFile
	for _, d := range dirs {
		p := filepath.Join(root, d)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		err := filepath.Walk(p, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return err
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				t.Fatalf("parse %s: %v", path, perr)
			}
			rel, _ := filepath.Rel(root, path)
			out = append(out, arFile{rel: filepath.ToSlash(rel), file: f})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", p, err)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return fset, out
}

// arLine renders a position as file:line for a finding message.
func arLine(fset *token.FileSet, rel string, pos token.Pos) string {
	return rel + ":" + strconv.Itoa(fset.Position(pos).Line)
}

// arStr returns the value of a Go string literal, and whether e was one.
func arStr(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

// ---------------------------------------------------------------------------
// Pin 1 — ADR 0019 D1: one credential acquirer.
// ---------------------------------------------------------------------------

// arCredSeam is the one file D1 nominates. Everything below is "and nowhere
// else".
const arCredSeam = "posse/credential.go"

// arCredTokens are the three spellings a second acquirer would need. They are
// the acquisition itself, not a synonym for it:
//
//   - the keychain query verb. `security` has many verbs; this is the one
//     that hands back a secret.
//   - the binary, named absolutely. ranger-base-ypf5 moved the adapter off
//     PATH precisely so a shim could not answer for it; a second file
//     spelling the path is a second adapter.
//   - the runtime's own credentials file, D2's store 3.
//   - the keychain item's own name, which is the query's subject — both the
//     exported constant and its value, because a second acquirer that
//     imports the constant and one that retypes the string are the same
//     finding.
var arCredTokens = []string{
	"find-generic-password",
	"/usr/bin/security",
	".credentials.json",
	"Claude Code-credentials",
}

// arCredIdents are the same claim spelled as an identifier.
var arCredIdents = []string{"KeychainService"}

// arCredAllow is one accepted mention outside the seam: the FILE, a substring
// of the literal that carries it, and why the mention is not an acquisition.
// The literal substring is what keeps a row from becoming a blanket pardon
// for its file — change the line and the row stops matching, and somebody
// looks.
type arCredAllow struct{ file, lit, why string }

// The register, measured 2026-09-01, re-measured 2026-09-02. Two mentions,
// neither of them a read: one is a CONTROL on the rule, and naming the thing
// you refuse is how a refusal is written.
//
// A third row stood here until ranger-base-x5f6p: posse/seatbelt.go's
// `~/.claude/.credentials.json`, the seatbelt read-deny. That literal is
// gone — the deny follows the runtime's own directory resolution now, and
// asks credentialFileCandidates for the paths — so seatbelt.go spells no
// credential path at all and needs no pardon. The row was dropped in the
// same commit as the literal, which is the rule this register runs on:
// leave it and the "matched nothing" arm reds; keep the literal and the
// unregistered arm reds. Either way somebody looks.
var arCredAllowed = []arCredAllow{
	{
		file: "posse/visibility.go",
		lit:  "find-generic-password|credentials",
		why:  "the visibility scanner's ERE — the detective control that finds a credential token in a DIFF. It matches text; it never opens a store.",
	},
	{
		file: "posse/cage.go",
		lit:  "is not the store of record and posse never reads it",
		why:  "the cage: container refusal message. Prose in a Die() telling the operator that a caged session's credentials file is not read (rangerhq-kiz).",
	},
}

type arCredHit struct {
	where string // rel:line
	file  string
	text  string // the literal, or the identifier
	token string
}

// arCredScan reports every mention of an acquisition token outside the seam,
// and the number of files it opened. Mentions come from the AST — string
// literals and identifiers — so a comment naming the keychain is not a hit
// and cannot become one by being reworded.
func arCredScan(fset *token.FileSet, files []arFile) (hits []arCredHit, scanned int) {
	for _, af := range files {
		scanned++
		if strings.HasSuffix(af.rel, arCredSeam) {
			continue
		}
		ast.Inspect(af.file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				v, ok := arStr(x)
				if !ok {
					return true
				}
				for _, tok := range arCredTokens {
					if strings.Contains(v, tok) {
						hits = append(hits, arCredHit{arLine(fset, af.rel, x.Pos()), af.rel, v, tok})
					}
				}
			case *ast.Ident:
				for _, id := range arCredIdents {
					if x.Name == id {
						hits = append(hits, arCredHit{arLine(fset, af.rel, x.Pos()), af.rel, x.Name, id})
					}
				}
			}
			return true
		})
	}
	return hits, scanned
}

func TestOneCredentialAcquirerAndNoOther(t *testing.T) {
	fset, files := arParse(t, ".", arRoots)
	if len(files) < arFileFloor {
		t.Fatalf("parsed only %d non-test .go files under %v — the walk measured nothing, so an absence here is not evidence", len(files), arRoots)
	}

	// The seam must still be the seam. If the tokens moved out of
	// credential.go this pin covers an empty rule, which is the failure mode
	// that reads greenest.
	var seam *arFile
	for i := range files {
		if strings.HasSuffix(files[i].rel, arCredSeam) {
			seam = &files[i]
		}
	}
	if seam == nil {
		t.Fatalf("%s is not under %v — ADR 0019 D1 names it as the one acquisition seam", arCredSeam, arRoots)
	}
	present := map[string]bool{}
	ast.Inspect(seam.file, func(n ast.Node) bool {
		if v, ok := arStr(exprOf(n)); ok {
			for _, tok := range arCredTokens {
				if strings.Contains(v, tok) {
					present[tok] = true
				}
			}
		}
		return true
	})
	for _, tok := range arCredTokens {
		if !present[tok] {
			t.Errorf("%s no longer spells %q. Either the acquisition moved — in which case D1's seam is a different file and this pin now watches an empty one — or it changed shape and the token is no longer the thing to look for.", seam.rel, tok)
		}
	}

	hits, scanned := arCredScan(fset, files)
	t.Logf("scanned %d non-test .go files, %d token mentions outside the seam", scanned, len(hits))

	unregistered, stale := arCredGrade(hits, arCredAllowed)
	for _, h := range unregistered {
		{
			t.Errorf("%s acquires a credential token outside the seam: %q (matched %q).\n"+
				"ADR 0019 D1: \"Nothing else in posse may acquire a credential except through this seam\" —\n"+
				"the seam is %s and the shape is Read(runtime, purpose). If this mention is NOT an\n"+
				"acquisition (a deny, a scanner pattern, a refusal message), add a row to\n"+
				"arCredAllowed naming the file, a substring of the literal, and why. A row is a\n"+
				"sentence somebody has to write, which is the point.", h.where, h.text, h.token, arCredSeam)
		}
	}
	for _, i := range stale {
		a := arCredAllowed[i]
		t.Errorf("arCredAllowed row %s (%q) matched nothing. The mention it pardons is gone or reworded — drop the row rather than leaving a pardon standing over a line nobody can find.", a.file, a.lit)
	}
}

// arCredGrade sorts hits against a register: the ones no row pardons, and the
// rows that pardon nothing. Both halves are findings, and both are exercised
// by the fixture rig — a register only holds if a stale row is as loud as an
// unregistered site.
func arCredGrade(hits []arCredHit, allow []arCredAllow) (unregistered []arCredHit, stale []int) {
	used := make([]bool, len(allow))
	for _, h := range hits {
		matched := false
		for i, a := range allow {
			if strings.HasSuffix(h.file, a.file) && strings.Contains(h.text, a.lit) {
				used[i], matched = true, true
				break
			}
		}
		if !matched {
			unregistered = append(unregistered, h)
		}
	}
	for i := range allow {
		if !used[i] {
			stale = append(stale, i)
		}
	}
	return unregistered, stale
}

// exprOf narrows a node to an expression for arStr; a nil result is not a
// string literal and arStr says so.
func exprOf(n ast.Node) ast.Expr {
	e, _ := n.(ast.Expr)
	return e
}

// arFixture writes files into a scratch tree and returns its root. The
// contents are hand-typed source, never generated from the lists under test:
// a fixture built from arCredTokens would make every mutation of that list
// equivalent and the rig would grade itself.
func arFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// The clean arm of the credential rig: a seam that acquires, a file that
// refuses, a file that talks about it. Nothing here is a finding, and the
// rig must say so — otherwise the planted arms below prove only that the
// scanner shouts at everything.
var arCredCleanFixture = map[string]string{
	"internal/posse/credential.go": `package posse

import "os/exec"

const KeychainService = "Claude Code-credentials"

func read() *exec.Cmd { return exec.Command("/usr/bin/security", "find-generic-password", "-s", KeychainService, "-w") }

func file() string { return "~/.claude/.credentials.json" }
`,
	"internal/posse/prose.go": `package posse

// The darwin adapter execs /usr/bin/security find-generic-password against
// the "Claude Code-credentials" item; ~/.claude/.credentials.json is store 3.
// None of that is code, and a comment acquires nothing.
func nothing() {}
`,
	"cmd/posse/main.go": `package main

func main() {}
`,
}

func TestCredentialAcquirerCensusCatchesEachShape(t *testing.T) {
	// Control first: the rig can come back clean, so a finding below is the
	// plant and not the rig's opinion of ordinary source.
	root := arFixture(t, arCredCleanFixture)
	fset, files := arParse(t, root, arRoots)
	if len(files) != 3 {
		t.Fatalf("clean fixture parsed %d files, want 3 — the rig did not read what it wrote", len(files))
	}
	if hits, _ := arCredScan(fset, files); len(hits) != 0 {
		t.Fatalf("the clean arm reports %v. Every arm below would then be unfalsifiable.", hits)
	}

	// One plant per shape, each in a file the seam suffix does not pardon.
	for _, tc := range []struct {
		name, path, body, want string
	}{
		{
			name: "a second keychain query",
			path: "internal/posse/meter.go",
			body: "package posse\n\nimport \"os/exec\"\n\nfunc token() ([]byte, error) {\n\treturn exec.Command(\"security\", \"find-generic-password\", \"-s\", \"x\", \"-w\").Output()\n}\n",
			want: "find-generic-password",
		},
		{
			name: "a second adapter naming the binary",
			path: "internal/posse/plan.go",
			body: "package posse\n\nvar bin = \"/usr/bin/security\"\n",
			want: "/usr/bin/security",
		},
		{
			name: "a second reader of store 3",
			path: "internal/posse/cost.go",
			body: "package posse\n\nimport \"os\"\n\nfunc load(home string) ([]byte, error) { return os.ReadFile(home + \"/.claude/.credentials.json\") }\n",
			want: ".credentials.json",
		},
		{
			name: "the item name retyped",
			path: "cmd/posse/probe.go",
			body: "package main\n\nvar svc = \"Claude Code-credentials\"\n",
			want: "Claude Code-credentials",
		},
		{
			name: "the exported constant imported",
			path: "cmd/posse/meter.go",
			body: "package main\n\nimport \"github.com/ranger360ai/posse/internal/posse\"\n\nvar svc = posse.KeychainService\n",
			want: "KeychainService",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := map[string]string{}
			for k, v := range arCredCleanFixture {
				f[k] = v
			}
			f[tc.path] = tc.body
			root := arFixture(t, f)
			fset, files := arParse(t, root, arRoots)
			hits, _ := arCredScan(fset, files)
			var got []string
			for _, h := range hits {
				got = append(got, h.where+" ("+h.token+")")
			}
			found := false
			for _, h := range hits {
				if h.token == tc.want && strings.HasSuffix(h.file, filepath.ToSlash(tc.path[strings.Index(tc.path, "/")+1:])) {
					found = true
				}
			}
			if !found {
				t.Errorf("planted %s in %s and the census reported %v — it did not see %q, so the live green above measured nothing for that shape", tc.name, tc.path, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pin 2 — ADR 0017 §3: no new shadow predicate.
// ---------------------------------------------------------------------------
//
// The rule, verbatim from §3: "per-runtime *behaviour* keyed on `rt.Name` is
// allowed only where the behaviour is inherently that CLI's own state
// (SeedCageHome writing `~/.claude.json`, trust.go seeding claude's dialog).
// A name-keyed branch that implements a *dimension* — counted-ness,
// trust-ness, sandbox shape — must go through the declaration, or the
// declaration is scenery."
//
// Both violations the ADR recorded have since been retired: the counted-ness
// pair (cost.go/cockpit.go) by the ADR 0012 D4 cost seam, and the
// turn-outcome read by `turn_outcome:` and the turnfailure.go registry. What
// stands is the accepted class, and this is its register.

// arRuntimeNames reads the runtime names out of the source itself — the
// `builtinRuntimes` composite literal — rather than carrying a list. A
// runtime added to the built-ins widens this census the same day, which a
// hand-typed set would not.
func arRuntimeNames(files []arFile) []string {
	var names []string
	for _, af := range files {
		ast.Inspect(af.file, func(n ast.Node) bool {
			vs, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}
			named := false
			for _, id := range vs.Names {
				if id.Name == "builtinRuntimes" {
					named = true
				}
			}
			if !named {
				return true
			}
			for _, v := range vs.Values {
				ast.Inspect(v, func(m ast.Node) bool {
					kv, ok := m.(*ast.KeyValueExpr)
					if !ok {
						return true
					}
					if k, ok := kv.Key.(*ast.Ident); !ok || k.Name != "Name" {
						return true
					}
					if s, ok := arStr(kv.Value); ok {
						names = append(names, s)
					}
					return true
				})
			}
			return true
		})
	}
	sort.Strings(names)
	return names
}

type arShadowSite struct {
	where string // rel:line
	file  string
	fn    string // enclosing function, "" at file scope
	text  string // the runtime name the branch keys on
	shape string // "branch" (== / != / case) or "table" (a map literal key)
}

// arEnclosing returns the name of the FuncDecl containing pos, receiver
// included, or "" for file scope.
func arEnclosing(f *ast.File, pos token.Pos) string {
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || pos < fd.Pos() || pos > fd.End() {
			continue
		}
		if fd.Recv != nil && len(fd.Recv.List) > 0 {
			t := fd.Recv.List[0].Type
			if star, ok := t.(*ast.StarExpr); ok {
				t = star.X
			}
			if id, ok := t.(*ast.Ident); ok {
				return id.Name + "." + fd.Name.Name
			}
		}
		return fd.Name.Name
	}
	return ""
}

// arShadowScan reports every runtime-name equality branch and every map
// literal keyed on a runtime name. `DefaultRuntime` counts as a name: it is
// the constant "claude" and a branch on it is the same branch.
func arShadowScan(fset *token.FileSet, files []arFile, names []string) []arShadowSite {
	isName := func(e ast.Expr) (string, bool) {
		if id, ok := e.(*ast.Ident); ok && id.Name == "DefaultRuntime" {
			return "DefaultRuntime", true
		}
		v, ok := arStr(e)
		if !ok {
			return "", false
		}
		for _, n := range names {
			if v == n {
				return v, true
			}
		}
		return "", false
	}
	var out []arShadowSite
	for _, af := range files {
		af := af
		add := func(pos token.Pos, text, shape string) {
			out = append(out, arShadowSite{arLine(fset, af.rel, pos), af.rel, arEnclosing(af.file, pos), text, shape})
		}
		ast.Inspect(af.file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BinaryExpr:
				if x.Op != token.EQL && x.Op != token.NEQ {
					return true
				}
				if v, ok := isName(x.X); ok {
					add(x.Pos(), v, "branch")
				} else if v, ok := isName(x.Y); ok {
					add(x.Pos(), v, "branch")
				}
			case *ast.CaseClause:
				for _, e := range x.List {
					if v, ok := isName(e); ok {
						add(e.Pos(), v, "branch")
					}
				}
			case *ast.CompositeLit:
				if _, ok := x.Type.(*ast.MapType); !ok {
					return true
				}
				for _, el := range x.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if v, ok := isName(kv.Key); ok {
						add(kv.Key.Pos(), v, "table")
					}
				}
			}
			return true
		})
	}
	return out
}

// arShadowAllow is one accepted site: the file, the enclosing function, the
// shape, and why the branch is that CLI's own state rather than a dimension.
type arShadowAllow struct{ file, fn, shape, why string }

// The register, measured 2026-09-01 against HEAD and widened once since. The
// five original branch rows are the ones ADR 0017 §3's register update leaves
// standing ("cage.go seeding, credential paths, trust.go's claude dialog")
// plus the two the same section classifies as identity and display.
//
// The sixth branch row is a different KIND of row and says so: it is not
// CLI-own-state, it is a narrow exception granted to one dimension by name in
// ADR 0013 §7 (2026-09-05) and executed by ranger-base-yi2f8. Its price is
// paid by a second pin that holds the reading display-only; read its why
// before treating it as a shape anything else may borrow.
//
// The two TABLE rows are an adjacent shape this census can see and §3 does
// not rule on in those words. cage.go's side map is named by §3's own
// CageCred row ("built-ins via a side map"); refresh.go's mint table is not
// ruled anywhere, and its row says so rather than pretending to a ruling.
// They are here so a THIRD runtime-keyed table is loud, not to grade these.
var arShadowAllowed = []arShadowAllow{
	{
		file: "posse/cage.go", fn: "App.SeedCageHome", shape: "branch",
		why: "§3's own example: writing ~/.claude.json is inherently claude's own state, and there is no dimension behind it to declare.",
	},
	{
		file: "posse/trust.go", fn: "SeedClaudeTrust", shape: "branch",
		why: "§3's other example: seeding claude's own trust dialog. Accepted in the register update by name.",
	},
	{
		file: "posse/credential.go", fn: "meterStore", shape: "branch",
		why: "credential paths — the register update's third standing site. posse ships no usage-endpoint adapter for another runtime (ADR 0012 D4), so the arm returns a NoSource, not a behaviour.",
	},
	{
		file: "posse/agents.go", fn: "AgentFile.RenderCommand", shape: "branch",
		why: "identity, not behaviour: the default-runtime shortcut before looking a non-default name up in builtinRuntimes. §3 classifies Name as consumed identity.",
	},
	{
		file: "posse/herdrback.go", fn: "App.RuntimeTierTag", shape: "branch",
		why: "display: the tag is suppressed for the default runtime at the default tier so a pane title reads clean. Nothing branches on the result.",
	},
	{
		file: "posse/herdr.go", fn: "Herdr.PaneAgentSession", shape: "branch",
		why: "CLI-own state, and the narrowest kind: `agent_session` is the RUNTIME's id for its own conversation, and the only caller reads claude's own submit log with it (sentline.go, ranger-base-2hvtv). There is no dimension behind it — a codex or grok pane has no such log to join to, so the arm returns an error and every reader falls back to the behaviour it had before that bead. The day another runtime keeps one, this stops being a branch and becomes a Runtime field.",
	},
	{
		file: "posse/permissionmode.go", fn: "paneReaderFor", shape: "branch",
		why: "the NAMED NARROW EXCEPTION, and the only row here that is one: ADR 0013 §7, approved 2026-09-05 and executed 2026-09-06 (ranger-base-yi2f8) — \"0057 removes the pane-mode declaration registry; the concrete built-in readers may identify the runtime they parse while preserving their current observations. It is an observation seam, not permission to bypass turn-outcome, cost or safety declarations.\" What earns it is DISPLAY-ONLY-ness, and that is not taken on trust: pin 4 below (TestPaneModeReadingDecidesNothing) censuses the non-test source and reds if any file outside permissionmode.go and herdrback.go names the reading, or if the backend reads it as anything but a rendered token. The declaration it replaced (`pane_mode:`) was measured first — ZERO external declarations existed in its one-day life, on this box or in either repo's history — so the seam bought a fourth CLI nobody has and cost a runtime yaml load per listing. A SECOND observation wanting this shape is a decision, not a precedent: the exception is granted to this one dimension by name.",
	},
	{
		file: "posse/cage.go", fn: "", shape: "table",
		why: "cageCredential, the built-ins' side map §3's CageCred row already names (\"built-ins via a side map\"). The declaration is CageCred:; this is the built-in's value for it.",
	},
	{
		file: "posse/refresh.go", fn: "", shape: "table",
		why: "runtimeMint: `claude setup-token` is claude's own mint command. ADJACENT, NOT RULED — §3's register update does not name it. Listed so a third table is loud; if it needs a verdict that is the operator's, not this pin's.",
	},
}

func TestNoNewShadowPredicate(t *testing.T) {
	fset, files := arParse(t, ".", arRoots)
	if len(files) < arFileFloor {
		t.Fatalf("parsed only %d non-test .go files under %v — the walk measured nothing", len(files), arRoots)
	}
	names := arRuntimeNames(files)
	if len(names) < 3 {
		t.Fatalf("read %v out of builtinRuntimes — fewer than the three built-ins that have shipped since ADR 0017, so the census is keyed on almost nothing", names)
	}
	t.Logf("runtime names read from builtinRuntimes: %v", names)

	sites := arShadowScan(fset, files, names)
	t.Logf("%d runtime-name sites over %d files", len(sites), len(files))

	unregistered, stale := arShadowGrade(sites, arShadowAllowed)
	for _, s := range unregistered {
		t.Errorf("%s (%s, in %q) keys on the runtime name %q and is not in the register.\n"+
			"ADR 0017 §3: a name-keyed branch is allowed only where the behaviour is inherently\n"+
			"that CLI's own state. If it implements a DIMENSION — counted-ness, trust-ness,\n"+
			"sandbox shape, who gets asked what — it must go through a Runtime field and the\n"+
			"declaration, or the grid is scenery. If it really is CLI-own-state, add a row to\n"+
			"arShadowAllowed saying which state and why.", s.where, s.shape, s.fn, s.text)
	}
	for _, i := range stale {
		a := arShadowAllowed[i]
		t.Errorf("arShadowAllowed row %s %q (%s) matched no site. The branch moved, was renamed, or was retired — repoint the row or drop it; a register that outlives its sites is a dated snapshot wearing a test's clothes.", a.file, a.fn, a.shape)
	}
}

// arShadowGrade sorts sites against a register, both halves, same as
// arCredGrade.
func arShadowGrade(sites []arShadowSite, allow []arShadowAllow) (unregistered []arShadowSite, stale []int) {
	used := make([]bool, len(allow))
	for _, s := range sites {
		matched := false
		for i, a := range allow {
			if strings.HasSuffix(s.file, a.file) && s.fn == a.fn && s.shape == a.shape {
				used[i], matched = true, true
				break
			}
		}
		if !matched {
			unregistered = append(unregistered, s)
		}
	}
	for i := range allow {
		if !used[i] {
			stale = append(stale, i)
		}
	}
	return unregistered, stale
}

// The register's other half, shown able to fail: a row that pardons nothing
// must be as loud as a site nobody pardoned. Otherwise a register decays into
// a list of sentences about code that left, and the next reader trusts it.
func TestCredentialRegisterIsLoudBothWays(t *testing.T) {
	root := arFixture(t, arCredCleanFixture)
	fset, files := arParse(t, root, arRoots)
	hits, _ := arCredScan(fset, files)
	if len(hits) != 0 {
		t.Fatalf("clean arm reports %v", hits)
	}
	// A row over a file that is there and a literal that is not.
	stalerow := []arCredAllow{{file: "posse/prose.go", lit: "a literal nobody wrote", why: "planted"}}
	if _, stale := arCredGrade(hits, stalerow); len(stale) != 1 {
		t.Errorf("a row pardoning a literal that does not exist graded as %v, want one stale row — the live green's stale-row arm measures nothing", stale)
	}
	// And a row that DOES match must not be reported stale, so the arm above
	// is not simply "every row is stale".
	f := map[string]string{}
	for k, v := range arCredCleanFixture {
		f[k] = v
	}
	f["internal/posse/deny.go"] = "package posse\n\nvar denied = []string{\"~/.claude/.credentials.json\"}\n"
	root = arFixture(t, f)
	fset, files = arParse(t, root, arRoots)
	hits, _ = arCredScan(fset, files)
	live := []arCredAllow{{file: "posse/deny.go", lit: "~/.claude/.credentials.json", why: "planted deny"}}
	un, stale := arCredGrade(hits, live)
	if len(un) != 0 || len(stale) != 0 {
		t.Errorf("a row over a real mention graded unregistered=%v stale=%v, want both empty — the register cannot pardon anything and every live row would read as a finding", un, stale)
	}
}

// The clean arm of the shadow rig: built-ins declared, a name compared
// against a VARIABLE (a lookup, not a branch), and prose. None is a site.
var arShadowCleanFixture = map[string]string{
	"internal/posse/runtime.go": `package posse

const DefaultRuntime = "claude"

var builtinRuntimes = []Runtime{
	{Name: "claude", Builtin: true},
	{Name: "codex", Builtin: true},
	{Name: "grok", Builtin: true},
}

// A branch on the runtime name would go here; naming it in a comment does
// not make one, and "claude" inside this sentence is prose.
func lookup(own string) int {
	for i := range builtinRuntimes {
		if builtinRuntimes[i].Name == own {
			return i
		}
	}
	return -1
}
`,
	"cmd/posse/main.go": `package main

func main() {}
`,
}

func TestShadowPredicateCensusCatchesEachShape(t *testing.T) {
	root := arFixture(t, arShadowCleanFixture)
	fset, files := arParse(t, root, arRoots)
	names := arRuntimeNames(files)
	if got := strings.Join(names, ","); got != "claude,codex,grok" {
		t.Fatalf("arRuntimeNames read %q from the fixture's builtinRuntimes, want claude,codex,grok — the name set the whole census keys on is not being read out of the source", got)
	}
	if sites := arShadowScan(fset, files, names); len(sites) != 0 {
		t.Fatalf("the clean arm reports %v — a name compared against a variable is a lookup, and every plant below would be unfalsifiable", sites)
	}

	for _, tc := range []struct {
		name, path, body, wantFn, wantShape string
	}{
		{
			name: "an equality branch on a literal name", path: "internal/posse/cost.go",
			body:      "package posse\n\nfunc counted(rt *Runtime) bool {\n\tif rt.Name == \"codex\" {\n\t\treturn false\n\t}\n\treturn true\n}\n",
			wantFn:    "counted",
			wantShape: "branch",
		},
		{
			name: "an inequality branch on DefaultRuntime", path: "internal/posse/trustnew.go",
			body:      "package posse\n\nfunc trusted(name string) bool { return name != DefaultRuntime }\n",
			wantFn:    "trusted",
			wantShape: "branch",
		},
		{
			name: "the same branch spelled as a switch", path: "internal/posse/sandbox.go",
			body:      "package posse\n\nfunc shape(name string) string {\n\tswitch name {\n\tcase \"grok\":\n\t\treturn \"none\"\n\t}\n\treturn \"seatbelt\"\n}\n",
			wantFn:    "shape",
			wantShape: "branch",
		},
		{
			name: "a third runtime-keyed table", path: "internal/posse/mint.go",
			body:      "package posse\n\nvar caps = map[string]int{\"claude\": 4, \"grok\": 1}\n",
			wantFn:    "",
			wantShape: "table",
		},
		{
			name: "a branch on a method receiver's name field", path: "internal/posse/pane.go",
			body:      "package posse\n\ntype P struct{ rt *Runtime }\n\nfunc (p *P) title() string {\n\tif p.rt.Name != \"claude\" {\n\t\treturn \"@\" + p.rt.Name\n\t}\n\treturn \"\"\n}\n",
			wantFn:    "P.title",
			wantShape: "branch",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := map[string]string{}
			for k, v := range arShadowCleanFixture {
				f[k] = v
			}
			f[tc.path] = tc.body
			root := arFixture(t, f)
			fset, files := arParse(t, root, arRoots)
			sites := arShadowScan(fset, files, arRuntimeNames(files))
			found := false
			for _, s := range sites {
				if s.fn == tc.wantFn && s.shape == tc.wantShape && strings.HasSuffix(s.file, filepath.Base(tc.path)) {
					found = true
				}
			}
			if !found {
				t.Errorf("planted %s in %s and the census reported %v — it did not see a %s site in %q, so the live green measured nothing for that shape", tc.name, tc.path, sites, tc.wantShape, tc.wantFn)
			}
			// And the register's unregistered arm must call it a finding:
			// seeing a site and pardoning it silently would be the same green.
			un, _ := arShadowGrade(sites, arShadowAllowed)
			if len(un) == 0 {
				t.Errorf("the planted site graded as registered against the live register — a plant that lands on an existing row proves nothing; %v", sites)
			}
		})
	}

	// The stale half, same as the credential register.
	root = arFixture(t, arShadowCleanFixture)
	fset, files = arParse(t, root, arRoots)
	sites := arShadowScan(fset, files, arRuntimeNames(files))
	row := []arShadowAllow{{file: "posse/gone.go", fn: "retired", shape: "branch", why: "planted"}}
	if _, stale := arShadowGrade(sites, row); len(stale) != 1 {
		t.Errorf("a row over a site that does not exist graded as %v, want one stale row", stale)
	}
}

// ---------------------------------------------------------------------------
// Pin 3 — ADR 0013 §4: the harness never closes a bead.
// ---------------------------------------------------------------------------
//
// §4 makes the bead the store of record and the runtime a hint: "`record:
// untrusted` — default for every other runtime ... Dispatch still launches.
// Gather never prints ✓ on settle-without-close", and then the line this pin
// is named for, whose subject is a crew seat read here as its role (ADR 0012
// D2): **this is not "the harness closes the bead."** A dispatch pass that
// could close a bead itself would
// make the record stage unfalsifiable — every settle would look like a close
// and the one signal the section nominates as truth would be written by the
// thing it grades.
//
// The claim, in three arms:
//
//  1. every call to a close verb of the Bd API is in the caller register —
//     the operator's `posse done` in cmd/posse/main.go, and ci-watch's
//     green half;
//  2. no function reachable from the dispatch path contains one of those
//     calls, and no close verb is reachable from it, except through a
//     caller ARM 2'S OWN register names;
//  3. no code outside beads.go builds a bd argv with a close verb in it —
//     the escape hatch around the Bd API entirely.
//
// Arm 2's positive control is what makes it evidence: the same graph, from
// the same roots, DOES reach `Bd.Claim` and `Bd.Comment`. A traversal that
// reached no Bd verb at all would report the same green over any code.
//
// ARM 2 HAS A REGISTER SINCE 2026-09-05 and it is the same shape arm 1's is:
// a file, a function and a sentence saying why that caller is not the
// agent's-behalf case §4 rejects. The exception is the operator's ruling on
// ranger-base-8fr2j, not this file's reading — and the verb half of the arm
// cuts the registered caller's close edge before it asks its question, so
// "a close verb is reachable" keeps meaning "by a route nobody wrote a
// sentence for" rather than going quiet the day the first row landed.

// arGraph is a name-keyed call graph over the parsed tree. Method calls whose
// receiver the source names as a `Bd` resolve to `Bd.<method>`; every other
// method call fans out to every same-named function in the tree, which
// over-approximates reachability — deliberately, since the claim is absence.
type arGraph struct {
	nodes map[string][]*arNode // key → declarations sharing it
	edges map[string]map[string]bool
	// The spellings the source itself gives a Bd, read in arBuildGraph.
	bdFields, bdFuncs, bdMethods map[string]bool
}

type arNode struct {
	key  string
	rel  string
	decl *ast.FuncDecl
	bd   map[string]bool // identifiers this function binds to a Bd
}

// arTypeIsBd reports whether a type expression names posse's Bd, qualified
// (`posse.Bd`, from cmd/) or not.
func arTypeIsBd(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name == "Bd"
	case *ast.SelectorExpr:
		return x.Sel.Name == "Bd"
	case *ast.StarExpr:
		return arTypeIsBd(x.X)
	}
	return false
}

func arFuncKey(fd *ast.FuncDecl) string {
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		t := fd.Recv.List[0].Type
		if star, ok := t.(*ast.StarExpr); ok {
			t = star.X
		}
		if id, ok := t.(*ast.Ident); ok {
			return id.Name + "." + fd.Name.Name
		}
	}
	return fd.Name.Name
}

func arBuildGraph(files []arFile) *arGraph {
	g := &arGraph{nodes: map[string][]*arNode{}, edges: map[string]map[string]bool{}}

	// Every spelling that hands back a Bd, read out of the source: struct
	// fields typed Bd (Dispatcher.Bd, HerdrBackend.Bd, cockpit's lowercase
	// bd), and functions whose result is one (NewBd, needBd, the reap
	// guard's accessor).
	bdFields := map[string]bool{}
	bdFuncs := map[string]bool{}
	bdMethods := map[string]bool{}
	for _, af := range files {
		ast.Inspect(af.file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.StructType:
				for _, f := range x.Fields.List {
					if !arTypeIsBd(f.Type) {
						continue
					}
					for _, nm := range f.Names {
						bdFields[nm.Name] = true
					}
				}
			case *ast.FuncDecl:
				if x.Type.Results == nil {
					return true
				}
				for _, r := range x.Type.Results.List {
					if !arTypeIsBd(r.Type) {
						continue
					}
					if x.Recv != nil {
						bdMethods[x.Name.Name] = true
					} else {
						bdFuncs[x.Name.Name] = true
					}
				}
			}
			return true
		})
	}

	g.bdFields, g.bdFuncs, g.bdMethods = bdFields, bdFuncs, bdMethods

	for _, af := range files {
		for _, d := range af.file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			nd := &arNode{key: arFuncKey(fd), rel: af.rel, decl: fd, bd: map[string]bool{}}
			arBindBd(nd, bdFields, bdFuncs, bdMethods)
			g.nodes[nd.key] = append(g.nodes[nd.key], nd)
		}
	}

	// Edges, once every node knows its own Bd identifiers.
	byMethod := map[string][]string{}
	for key := range g.nodes {
		if i := strings.LastIndex(key, "."); i >= 0 {
			m := key[i+1:]
			if !strings.HasPrefix(key, "Bd.") {
				byMethod[m] = append(byMethod[m], key)
			}
		} else {
			byMethod[key] = append(byMethod[key], key)
		}
	}
	for key, nds := range g.nodes {
		out := g.edges[key]
		if out == nil {
			out = map[string]bool{}
			g.edges[key] = out
		}
		for _, nd := range nds {
			ast.Inspect(nd.decl.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					for _, k := range byMethod[fn.Name] {
						out[k] = true
					}
					out[fn.Name] = true
				case *ast.SelectorExpr:
					if arIsBdExpr(fn.X, nd.bd, bdFields, bdFuncs, bdMethods) {
						out["Bd."+fn.Sel.Name] = true
						return true
					}
					for _, k := range byMethod[fn.Sel.Name] {
						out[k] = true
					}
				}
				return true
			})
		}
	}
	return g
}

// arBindBd finds the identifiers this function binds to a Bd: its receiver,
// parameters and results declared as one, `var x Bd`, and `x := <a Bd>`. The
// loop runs to a fixed point so `bd := d.Bd; b := bd` resolves.
func arBindBd(nd *arNode, fields, funcs, methods map[string]bool) {
	bindField := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			if !arTypeIsBd(f.Type) {
				continue
			}
			for _, nm := range f.Names {
				nd.bd[nm.Name] = true
			}
		}
	}
	bindField(nd.decl.Recv)
	bindField(nd.decl.Type.Params)
	bindField(nd.decl.Type.Results)
	for pass := 0; pass < 3; pass++ {
		before := len(nd.bd)
		ast.Inspect(nd.decl.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.ValueSpec:
				if arTypeIsBd(x.Type) {
					for _, nm := range x.Names {
						nd.bd[nm.Name] = true
					}
				}
				for i, v := range x.Values {
					if i < len(x.Names) && arIsBdExpr(v, nd.bd, fields, funcs, methods) {
						nd.bd[x.Names[i].Name] = true
					}
				}
			case *ast.AssignStmt:
				for i, rhs := range x.Rhs {
					if i >= len(x.Lhs) || !arIsBdExpr(rhs, nd.bd, fields, funcs, methods) {
						continue
					}
					if id, ok := x.Lhs[i].(*ast.Ident); ok {
						nd.bd[id.Name] = true
					}
				}
			}
			return true
		})
		if len(nd.bd) == before {
			return
		}
	}
}

// arIsBdExpr reports whether the source says e is a Bd.
func arIsBdExpr(e ast.Expr, local, fields, funcs, methods map[string]bool) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return local[x.Name]
	case *ast.SelectorExpr:
		return fields[x.Sel.Name]
	case *ast.CompositeLit:
		return arTypeIsBd(x.Type)
	case *ast.CallExpr:
		switch fn := x.Fun.(type) {
		case *ast.Ident:
			return funcs[fn.Name]
		case *ast.SelectorExpr:
			return methods[fn.Sel.Name] || funcs[fn.Sel.Name]
		}
	case *ast.ParenExpr:
		return arIsBdExpr(x.X, local, fields, funcs, methods)
	}
	return false
}

// arReachCut is reach with ONE kind of edge removed: the edge from a
// registered caller (arCloseReachAllowed) to a close verb. Everything else
// is followed, including every other edge out of that same caller.
//
// It is what keeps arm 2's verb assertion meaning what it meant before the
// register existed. Plain reachability answers "is a close verb reachable
// from the dispatch path", and once ONE pardoned caller is on that path the
// answer is yes forever — so the assertion would go quiet over a second,
// unregistered closer added next door. With the pardoned edges cut, a
// reachable close verb again means "reachable by a route nobody wrote a
// sentence for", which is the claim §4 actually makes.
//
// A ROW THAT NAMES THE WRONG FILE CUTS NOTHING: the caller is matched by
// node key AND by the file the node was parsed from, so a stale row cannot
// silently pardon a same-named function somewhere else.
func (g *arGraph) reachCut(roots []string, cut map[string]bool, verbs []string) map[string]bool {
	isVerb := map[string]bool{}
	for _, v := range verbs {
		isVerb[v] = true
	}
	seen := map[string]bool{}
	var walk func(string)
	walk = func(k string) {
		if seen[k] {
			return
		}
		seen[k] = true
		for next := range g.edges[k] {
			if cut[k] && isVerb[next] {
				continue
			}
			walk(next)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return seen
}

// arCutKeys is the set of function keys a register pardons, resolved against
// the tree that was parsed: a row whose file does not hold a declaration of
// that name pardons nothing, and arm 2's stale half is what says so out loud.
func arCutKeys(g *arGraph, allow []arCloseCallerAllow) map[string]bool {
	cut := map[string]bool{}
	for _, a := range allow {
		for _, nd := range g.nodes[a.fn] {
			if strings.HasSuffix(nd.rel, a.file) {
				cut[a.fn] = true
			}
		}
	}
	return cut
}

// arReach is the transitive closure from roots.
func (g *arGraph) reach(roots []string) map[string]bool {
	seen := map[string]bool{}
	var walk func(string)
	walk = func(k string) {
		if seen[k] {
			return
		}
		seen[k] = true
		for next := range g.edges[k] {
			walk(next)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return seen
}

// arCloseVerbs reads the close verbs out of the Bd API rather than naming
// one: any Bd method that hands bd's argv builders a close literal. A verb
// added later joins the census on the day it is written.
var arCloseLiterals = []string{"close", "closed"}

func arCloseVerbs(g *arGraph) []string {
	argvBuilder := map[string]bool{"run": true, "runOnce": true, "bdArgs": true}
	var out []string
	for key, nds := range g.nodes {
		if !strings.HasPrefix(key, "Bd.") {
			continue
		}
		for _, nd := range nds {
			found := false
			ast.Inspect(nd.decl.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := ""
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				}
				if !argvBuilder[name] {
					return true
				}
				for _, a := range call.Args {
					v, ok := arStr(a)
					if !ok {
						continue
					}
					for _, lit := range arCloseLiterals {
						if v == lit {
							found = true
						}
					}
				}
				return true
			})
			if found {
				out = append(out, key)
			}
		}
	}
	sort.Strings(out)
	return out
}

// arCallSites finds every call to one of the named Bd verbs, with the
// function it sits in.
type arCallSite struct{ where, rel, in, verb string }

func arCallSites(fset *token.FileSet, files []arFile, g *arGraph, verbs []string) []arCallSite {
	want := map[string]bool{}
	for _, v := range verbs {
		want[strings.TrimPrefix(v, "Bd.")] = true
	}
	var out []arCallSite
	byRel := map[string][]*arNode{}
	for _, nds := range g.nodes {
		for _, nd := range nds {
			byRel[nd.rel] = append(byRel[nd.rel], nd)
		}
	}
	for _, af := range files {
		for _, nd := range byRel[af.rel] {
			ast.Inspect(nd.decl.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !want[sel.Sel.Name] {
					return true
				}
				// The receiver decides it: only a Bd's Close is Bd.Close.
				if !arIsBdExpr(sel.X, nd.bd, g.bdFields, g.bdFuncs, g.bdMethods) {
					return true
				}
				out = append(out, arCallSite{arLine(fset, af.rel, call.Pos()), af.rel, nd.key, "Bd." + sel.Sel.Name})
				return true
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].where < out[j].where })
	return out
}

// arDispatchRoots is "the dispatch path": everything the pass is, plus its
// constructor. Read out of the tree — every method on Dispatcher — so a
// method added tomorrow is a root the same day.
func arDispatchRoots(g *arGraph) []string {
	var roots []string
	for key := range g.nodes {
		if strings.HasPrefix(key, "Dispatcher.") || key == "NewDispatcher" {
			roots = append(roots, key)
		}
	}
	sort.Strings(roots)
	return roots
}

// arRawBdClose finds a close verb built into a bd argv OUTSIDE the Bd API —
// the escape hatch around arm 1 and arm 2 entirely.
func arRawBdClose(fset *token.FileSet, files []arFile) []string {
	var out []string
	for _, af := range files {
		if strings.HasSuffix(af.rel, "posse/beads.go") {
			continue // the API itself; arms 1 and 2 grade its callers
		}
		ast.Inspect(af.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			}
			if name != "bdArgs" && name != "run" && name != "runOnce" {
				return true
			}
			for _, a := range call.Args {
				v, ok := arStr(a)
				if !ok {
					continue
				}
				for _, lit := range arCloseLiterals {
					if v == lit {
						out = append(out, arLine(fset, af.rel, call.Pos()))
					}
				}
			}
			return true
		})
	}
	return out
}

// arCloseCallerAllow is one accepted caller of a Bd close verb: the file, the
// function, and why a human is behind it. A register rather than a count, for
// the same reason as the other two — the day a second caller is deliberate,
// somebody writes the sentence.
type arCloseCallerAllow struct{ file, fn, why string }

// The register, measured 2026-09-01, re-measured 2026-09-04. Two rows.
var arCloseCallerAllowed = []arCloseCallerAllow{
	{
		file: "posse/main.go", fn: "main",
		why: "`posse done <id> [--as <persona>]` — the operator verb, in the command switch a human types at. It sits ABOVE the dispatch path, not on it: no Dispatcher method reaches main, which is what arm 2 measures.",
	},
	{
		file: "posse/ciwatch.go", fn: "App.ciClear",
		why: "ci-watch's green half (ranger-base-4gy4i). It IS on the dispatch path, which is arm 2's question and not this arm's — arCloseReachAllowed carries the sentence about why §4 admits it. Here it is simply the second caller in the tree, and it is written down.",
	},
}

// arCloseReachAllowed is ARM 2's register, and it is the same type and the
// same grader as arm 1's over a different question: arm 1 asks who calls a
// close verb anywhere in the tree, this asks who calls one from a function
// the DISPATCH PATH REACHES. §4's prohibition is a reachability claim, so
// the exception to it needs a reachability register; a row here is a
// sentence saying why this caller is not the "harness closes the bead on the
// agent's behalf" case the section rejects.
//
// The register, measured 2026-09-04. One row.
//
// Adding a second is the point of the shape: a harness close that nobody
// wrote a sentence for does not compile green, and a row that stops matching
// a real call site is as loud as an unregistered one — otherwise this decays
// into a permanent pardon for a call that moved.
var arCloseReachAllowed = []arCloseCallerAllow{
	{
		file: "posse/ciwatch.go", fn: "App.ciClear",
		why: "ci-watch closing the bead IT FILED that NO SESSION EVER CLAIMED — status still `open`, no assignee (ciwatch.go, ciHolder). OPERATOR RULING ranger-base-8fr2j, 2026-09-05: this is the one exception ADR 0013 §4 admits, and it is not the agent's-behalf case the section rejects, because there is no agent. §4's harm is a record graded by the thing that writes it — settle-without-close made unobservable — and a bead nothing was ever dispatched onto grades nobody: no session's settle is measured by it, no persona's close count moves. The guard is read off the bead rather than remembered, and every other shape (in_progress, assigned, blocked, deferred) keeps the shipped behaviour: a comment saying CLOSE IT, and the close left to the seat. Measured cost this removes: 7 red episodes in 6.6 days on ci.yml's own history, 6 of them self-healed — ~6 dispatched sessions a week spent closing beads nobody worked (ranger-base-x9e34).",
	},
}

// arCloseCallerGrade sorts call sites against the register, both halves.
func arCloseCallerGrade(sites []arCallSite, allow []arCloseCallerAllow) (unregistered []arCallSite, stale []int) {
	used := make([]bool, len(allow))
	for _, s := range sites {
		matched := false
		for i, a := range allow {
			if strings.HasSuffix(s.rel, a.file) && s.in == a.fn {
				used[i], matched = true, true
				break
			}
		}
		if !matched {
			unregistered = append(unregistered, s)
		}
	}
	for i := range allow {
		if !used[i] {
			stale = append(stale, i)
		}
	}
	return unregistered, stale
}

func TestNoBdCloseVerbReachableFromDispatch(t *testing.T) {
	fset, files := arParse(t, ".", arRoots)
	if len(files) < arFileFloor {
		t.Fatalf("parsed only %d non-test .go files under %v — the walk measured nothing", len(files), arRoots)
	}
	g := arBuildGraph(files)
	if len(g.nodes) < 500 {
		t.Fatalf("the graph holds %d function keys — too few for this tree, so the traversal below is not measuring the binary", len(g.nodes))
	}

	verbs := arCloseVerbs(g)
	t.Logf("close verbs read out of the Bd API: %v", verbs)
	if len(verbs) == 0 {
		t.Fatalf("no Bd method hands bd's argv builders %v. Either the close verb moved out of beads.go — in which case every arm below watches nothing — or the argv is built some way this reader does not see.", arCloseLiterals)
	}

	// Arm 1 — the whole tree. Every Bd close call, wherever it is.
	sites := arCallSites(fset, files, g, verbs)
	t.Logf("Bd close call sites in the tree: %v", sites)
	unregistered, stale := arCloseCallerGrade(sites, arCloseCallerAllowed)
	for _, s := range unregistered {
		t.Errorf("%s (in %s) calls %s and is not in the register.\n"+
			"ADR 0013 §4 makes the bead the store of record and the harness a reader of it,\n"+
			"and its closing line says in as many words that the harness is not what closes\n"+
			"the bead. The only registered caller is the operator verb `posse done <id>`,\n"+
			"which a human types. A harness that can close the bead cannot then be graded by\n"+
			"it: settle-without-close is the one signal §4 nominates as truth. If this caller\n"+
			"is deliberate, add a row saying who types it and why they are not the harness.",
			s.where, s.in, s.verb)
	}
	for _, i := range stale {
		a := arCloseCallerAllowed[i]
		t.Errorf("arCloseCallerAllowed row %s %q matched no call site. The operator verb moved or was renamed — repoint the row; a register that pardons a caller nobody can find is how this pin goes quiet.", a.file, a.fn)
	}

	// Arm 2 — reachability from the dispatch path.
	roots := arDispatchRoots(g)
	if len(roots) < 50 {
		t.Fatalf("found %d dispatch roots (*Dispatcher methods + NewDispatcher) — the roots did not resolve, so an empty reachable set below would say nothing", len(roots))
	}
	seen := g.reach(roots)
	t.Logf("%d dispatch roots reach %d of %d function keys", len(roots), len(seen), len(g.nodes))

	// The positive control, and it is the whole evidence: the SAME traversal
	// from the SAME roots reaches other Bd verbs. A graph that reached no Bd
	// verb at all would print this same green over a harness that closed
	// every bead it dispatched.
	for _, want := range []string{"Bd.Claim", "Bd.Comment", "Bd.Ready"} {
		if !seen[want] {
			t.Fatalf("the dispatch traversal does not reach %s. A pass that neither claims nor comments is not this harness — the graph is broken, and its silence about the close verb is the graph's, not the code's.", want)
		}
	}
	// The verb half, with the registered callers' close edges cut: a close
	// verb still reachable is one reachable by a route nobody wrote a
	// sentence for.
	cut := arCutKeys(g, arCloseReachAllowed)
	if len(cut) != len(arCloseReachAllowed) {
		t.Errorf("arCloseReachAllowed has %d row(s) and %d of them resolved to a function in the file they name — a row that resolves to nothing pardons nothing, and the assertions below are then measuring a register that is not there", len(arCloseReachAllowed), len(cut))
	}
	unpardoned := g.reachCut(roots, cut, verbs)
	for _, v := range verbs {
		if unpardoned[v] {
			t.Errorf("%s is reachable from the dispatch path by a route no arCloseReachAllowed row names. ADR 0013 §4: the bead is the store of record and the runtime is not — a pass that can close a bead makes settle-without-close unobservable, and the record grade it hands each runtime is then a grade of itself. The one ruled exception (ranger-base-8fr2j) is a bead the HARNESS filed that NO SESSION EVER CLAIMED; if this route is another one, it needs the row that says whose record it does not grade.", v)
		}
	}
	// And the sites that sit in a reachable function, graded against arm 2's
	// own register — the same grader as arm 1, over the reachable subset.
	var onPath []arCallSite
	for _, s := range sites {
		if seen[s.in] {
			onPath = append(onPath, s)
		}
	}
	t.Logf("Bd close call sites the dispatch path reaches: %v", onPath)
	reachUnreg, reachStale := arCloseCallerGrade(onPath, arCloseReachAllowed)
	for _, s := range reachUnreg {
		t.Errorf("%s sits in %s, which the dispatch path reaches, and is not in arCloseReachAllowed.\n"+
			"ADR 0013 §4 rejects the harness closing a bead on the agent's behalf; its one\n"+
			"exception (ranger-base-8fr2j) is a bead the harness itself filed that no session\n"+
			"ever claimed. If this caller is that case, add the row saying so — who filed the\n"+
			"bead, how the caller knows nobody claimed it, and whose record it therefore does\n"+
			"not grade.", s.where, s.in)
	}
	for _, i := range reachStale {
		a := arCloseReachAllowed[i]
		t.Errorf("arCloseReachAllowed row %s %q matched no reachable call site. Either the caller left the dispatch path — in which case the exception is over and the row goes with it — or it moved and the row must be repointed; a register pardoning a call nobody can find is how this pin goes quiet.", a.file, a.fn)
	}

	// Arm 3 — the escape hatch.
	if raw := arRawBdClose(fset, files); len(raw) != 0 {
		t.Errorf("a bd close argv is built outside the Bd API at %v. Arms 1 and 2 grade calls to Bd's methods; this is the way around them.", raw)
	}
}

// The clean arm of the dispatch rig: a Bd with four verbs, a Dispatcher that
// claims, gathers and comments across three hops, and an operator `done`
// above it. This is the shape the live tree has, small enough to read.
var arDispatchCleanFixture = map[string]string{
	"internal/posse/beads.go": `package posse

type Bd struct{ Bin string }

func NewBd() Bd { return Bd{} }

func bdArgs(actor string, rest ...string) []string { return rest }

func (b Bd) run(dir string, args ...string) ([]byte, error) { return nil, nil }

func (b Bd) Ready(dir, assignee string) ([]byte, error) {
	return b.run(dir, bdArgs("", "ready")...)
}

func (b Bd) Claim(dir, id, actor string) error {
	_, err := b.run(dir, bdArgs(actor, "update", id, "--claim")...)
	return err
}

func (b Bd) Comment(dir, id, text, actor string) error {
	_, err := b.run(dir, bdArgs(actor, "comments", "add", id, text)...)
	return err
}

func (b Bd) Close(dir, id, actor string) error {
	_, err := b.run(dir, bdArgs(actor, "close", id)...)
	return err
}
`,
	"internal/posse/dispatch.go": `package posse

type Dispatcher struct{ Bd Bd }

func NewDispatcher() *Dispatcher { return &Dispatcher{Bd: NewBd()} }

func (d *Dispatcher) Run(dir string) error {
	if err := d.Bd.Claim(dir, "x-1", "qa"); err != nil {
		return err
	}
	return d.settle(dir)
}

func (d *Dispatcher) settle(dir string) error {
	d.gather(dir)
	return d.Bd.Comment(dir, "x-1", "settled", "harness")
}

func (d *Dispatcher) gather(dir string) { _, _ = d.Bd.Ready(dir, "") }
`,
	"cmd/posse/main.go": `package main

import "github.com/ranger360ai/posse/internal/posse"

func needBd() posse.Bd { return posse.NewBd() }

func main() {
	bd := needBd()
	_ = bd.Close("", "x-1", "operator")
}
`,
}

// arDispatchGrade runs every arm of pin 3 over a tree. The graph and its
// roots ride along so the register arms below can re-ask arm 2's question
// with a register of their own — a register that pardons nothing is the only
// way to show that the live one pardons something.
type arDispatchResult struct {
	verbs []string
	sites []arCallSite
	seen  map[string]bool
	raw   []string
	g     *arGraph
	roots []string
}

// onPath is the call sites arm 2 grades: the ones in a function the dispatch
// path reaches.
func (r arDispatchResult) onPath() []arCallSite {
	var out []arCallSite
	for _, s := range r.sites {
		if r.seen[s.in] {
			out = append(out, s)
		}
	}
	return out
}

// pardoned is arm 2's verb half under an arbitrary register: whether a close
// verb is still reachable once that register's callers' close edges are cut.
func (r arDispatchResult) pardoned(allow []arCloseCallerAllow) bool {
	left := r.g.reachCut(r.roots, arCutKeys(r.g, allow), r.verbs)
	for _, v := range r.verbs {
		if left[v] {
			return false
		}
	}
	return true
}

func arDispatchGrade(t *testing.T, root string) arDispatchResult {
	t.Helper()
	fset, files := arParse(t, root, arRoots)
	g := arBuildGraph(files)
	verbs := arCloseVerbs(g)
	roots := arDispatchRoots(g)
	return arDispatchResult{
		verbs: verbs,
		sites: arCallSites(fset, files, g, verbs),
		seen:  g.reach(roots),
		raw:   arRawBdClose(fset, files),
		g:     g,
		roots: roots,
	}
}

func TestDispatchCloseCensusCatchesEachShape(t *testing.T) {
	// Control. The rig must read the fixture the way it reads the tree: one
	// close verb, one operator call site, the other verbs reachable, the
	// close verb not.
	got := arDispatchGrade(t, arFixture(t, arDispatchCleanFixture))
	if strings.Join(got.verbs, ",") != "Bd.Close" {
		t.Fatalf("clean arm read close verbs %v, want [Bd.Close] — the verb is derived from the argv, and the rig cannot see it", got.verbs)
	}
	if un, _ := arCloseCallerGrade(got.sites, arCloseCallerAllowed); len(got.sites) != 1 || len(un) != 0 {
		t.Fatalf("clean arm found call sites %v (unregistered %v), want the one operator verb in cmd/posse/main.go and the live register pardoning it", got.sites, un)
	}
	for _, want := range []string{"Bd.Claim", "Bd.Comment", "Bd.Ready"} {
		if !got.seen[want] {
			t.Fatalf("clean arm's dispatch roots do not reach %s — the traversal is dead, and every plant below would fail for the wrong reason", want)
		}
	}
	if got.seen["Bd.Close"] || len(got.raw) != 0 {
		t.Fatalf("clean arm already reports a violation (reachable=%v raw=%v)", got.seen["Bd.Close"], got.raw)
	}

	plant := func(rel, body string) string {
		f := map[string]string{}
		for k, v := range arDispatchCleanFixture {
			f[k] = v
		}
		f[rel] = body
		return arFixture(t, f)
	}

	t.Run("a dispatch method closes the bead itself", func(t *testing.T) {
		got := arDispatchGrade(t, plant("internal/posse/settleopen.go", `package posse

func (d *Dispatcher) autoClose(dir string) error { return d.Bd.Close(dir, "x-1", "harness") }
`))
		if !got.seen["Bd.Close"] {
			t.Errorf("planted a Dispatcher method calling d.Bd.Close and the traversal did not reach it — arm 2's live green measures nothing")
		}
		if un, _ := arCloseCallerGrade(got.sites, arCloseCallerAllowed); len(un) != 1 {
			t.Errorf("arm 1 graded %v as %v unregistered, want the plant called out", got.sites, un)
		}
	})

	t.Run("two hops from the dispatch path", func(t *testing.T) {
		got := arDispatchGrade(t, plant("internal/posse/reap.go", `package posse

func (d *Dispatcher) reapPass(dir string) error { return reapOne(d.Bd, dir) }

func reapOne(bd Bd, dir string) error { return bd.Close(dir, "x-1", "harness") }
`))
		if !got.seen["Bd.Close"] {
			t.Errorf("planted a two-hop route (Dispatcher.reapPass → reapOne → Bd.Close) and the traversal did not reach it — the graph resolves a direct field selector and nothing else, so arm 2 only ever saw one-line violations")
		}
	})

	t.Run("a close off the dispatch path is still a second caller", func(t *testing.T) {
		got := arDispatchGrade(t, plant("internal/posse/lint.go", `package posse

func sweep(bd Bd, dir string) { _ = bd.Close(dir, "x-1", "sweeper") }
`))
		if got.seen["Bd.Close"] {
			t.Errorf("sweep is called by nobody — the traversal should not reach Bd.Close through it, and a graph that reaches everything grades nothing")
		}
		if un, _ := arCloseCallerGrade(got.sites, arCloseCallerAllowed); len(un) != 1 {
			t.Errorf("arm 1 graded %v as %v unregistered, want the plant — a second closer outside the pass is still a second writer of the store of record", got.sites, un)
		}
	})

	t.Run("the argv built around the Bd API", func(t *testing.T) {
		got := arDispatchGrade(t, plant("internal/posse/nudge.go", `package posse

func nudge(b Bd, dir, id, actor string) { _, _ = b.run(dir, bdArgs(actor, "close", id)...) }
`))
		if len(got.raw) != 1 {
			t.Errorf("planted a bd close argv outside beads.go and arm 3 reported %v — the escape hatch around arms 1 and 2 is unwatched", got.raw)
		}
	})

	// ─── arm 2's register, both ways ─────────────────────────────────────
	//
	// The live green above says "the one reachable closer is the one row
	// names it". These say the register is what made that green: the same
	// planted closer, graded against four registers, is pardoned by exactly
	// the one that names it in the file it is in.
	t.Run("arm 2's register pardons the caller it names", func(t *testing.T) {
		got := arDispatchGrade(t, plant("internal/posse/settleopen.go", `package posse

func (d *Dispatcher) autoClose(dir string) error { return d.Bd.Close(dir, "x-1", "harness") }
`))
		if len(got.onPath()) != 1 {
			t.Fatalf("the planted closer sits in %v on the dispatch path, want exactly one site — the arms below would then grade nothing", got.onPath())
		}
		row := []arCloseCallerAllow{{file: "posse/settleopen.go", fn: "Dispatcher.autoClose", why: "planted"}}

		// Pardoned: no unregistered site, no stale row, no reachable verb.
		if un, stale := arCloseCallerGrade(got.onPath(), row); len(un) != 0 || len(stale) != 0 {
			t.Errorf("a row over the reachable closer graded unregistered=%v stale=%v, want both empty — arm 2's register cannot pardon anything, so the live green means only that nothing closes at all", un, stale)
		}
		if !got.pardoned(row) {
			t.Errorf("the close verb is still reachable with the registered caller's close edge cut — the cut does not reach the edge it names, so the live arm is green for a reason the register did not buy")
		}

		// Unpardoned: the empty register is the shipped rule, and it must
		// still be loud.
		if un, _ := arCloseCallerGrade(got.onPath(), nil); len(un) != 1 {
			t.Errorf("with no register the planted closer graded %v unregistered, want 1 — arm 2 pardons by default and grades nobody", un)
		}
		if got.pardoned(nil) {
			t.Errorf("with no register the close verb read as unreachable from the dispatch path — the verb half is blind, and every unregistered closer would ride in behind the live row")
		}

		// A row naming the right function in the WRONG file pardons
		// nothing: this is the register decaying into a name match, which
		// is how one pardoned closer would cover a same-named one next door.
		wrongFile := []arCloseCallerAllow{{file: "posse/reap.go", fn: "Dispatcher.autoClose", why: "planted, wrong file"}}
		if got.pardoned(wrongFile) {
			t.Errorf("a row naming %s in a file that does not declare it still cut the close edge — the register pardons by name alone", wrongFile[0].fn)
		}
		if un, stale := arCloseCallerGrade(got.onPath(), wrongFile); len(un) != 1 || len(stale) != 1 {
			t.Errorf("a wrong-file row graded unregistered=%v stale=%v, want one of each", un, stale)
		}

		// And a row over a caller that is not there at all is stale, arm
		// 1's rule: a register that outlives its site is a dated snapshot
		// wearing a test's clothes.
		gone := []arCloseCallerAllow{{file: "posse/gone.go", fn: "Dispatcher.retired", why: "planted, retired"}}
		if _, stale := arCloseCallerGrade(got.onPath(), gone); len(stale) != 1 {
			t.Errorf("a row over a caller that does not exist graded stale=%v, want one", stale)
		}
	})

	t.Run("a close verb spelled --status closed", func(t *testing.T) {
		got := arDispatchGrade(t, plant("internal/posse/status.go", `package posse

func (b Bd) Finish(dir, id, actor string) error {
	_, err := b.run(dir, bdArgs(actor, "update", id, "--status", "closed")...)
	return err
}
`))
		if len(got.verbs) != 2 {
			t.Errorf("a second Bd method writing --status closed read as verbs %v — the verb set is derived so a close by another spelling joins it, and it did not", got.verbs)
		}
	})
}

// ---------------------------------------------------------------------------
// Pin 4 — ADR 0013 §7: the pane-mode reading decides nothing.
// ---------------------------------------------------------------------------
//
// The price of the one NAMED NARROW EXCEPTION in the register above. ADR 0057
// lets the concrete pane readers key on the runtime's name, and ADR 0013 §7
// grants that on a stated condition: "It is an observation seam, not
// permission to bypass turn-outcome, cost or safety declarations." A DISPLAY
// observation is what earns the exception, so display-only is what has to be
// checkable — otherwise the grant is prose and the branch is a shadow
// predicate wearing a citation.
//
// The rule: only the file that produces the reading and the file that writes
// it onto a session may name these symbols at all, and inside the backend the
// only READ is the token a listing prints. A launch, guard, dispatch or
// preflight path that starts consulting a pane's mode reds here the day it is
// written — which is also the day the exception above would need re-deciding.
//
// Same census shape and the same stated blindness as pins 1–3: it sees
// identifiers spelled in Go source, over the parsed non-test tree with
// comments dropped. A consumer that took the value through an untyped
// intermediary — a string handed on, a whole struct copied and read
// elsewhere — is invisible to it. It holds the register, not the rule.

// arPaneOwners is the two files allowed to name the reading, and why each is
// allowed to. A third owner is a decision about ADR 0013 §7, not a detail.
var arPaneOwners = map[string]string{
	"internal/posse/permissionmode.go": "the readers themselves, the states they return, and `posse gates <persona>`'s per-session report",
	"internal/posse/herdrback.go":      "the listing backend: the only WRITER of HerdrSession.PermissionMode, and the row that renders one token from it",
}

// arPaneSymbol reports whether an identifier names the pane-mode reading.
func arPaneSymbol(name string) bool {
	return strings.HasPrefix(name, "PaneMode") || strings.HasPrefix(name, "paneMode") ||
		strings.HasPrefix(name, "paneReader") || name == "ReadPaneMode" || name == "PermissionMode"
}

func TestPaneModeReadingDecidesNothing(t *testing.T) {
	fset, files := arParse(t, ".", arRoots)
	if len(files) < arFileFloor {
		t.Fatalf("parsed only %d non-test .go files under %v — the walk measured nothing, so an absence here is not evidence", len(files), arRoots)
	}
	named := 0
	for _, af := range files {
		af := af
		_, owned := arPaneOwners[af.rel]
		ast.Inspect(af.file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || !arPaneSymbol(id.Name) {
				return true
			}
			named++
			if !owned {
				t.Errorf("%s names the pane-mode reading (%s), and only %v may.\n"+
					"ADR 0013 §7 grants the name-keyed reader selection its narrow exception BECAUSE this is a display\n"+
					"observation. A launch, guard, dispatch or preflight path that branches on what a pane's footer says\n"+
					"makes it a shadow predicate after all (ADR 0017 §3) — declare the dimension instead. If this really\n"+
					"is a third display surface, add it to arPaneOwners saying what it renders.",
					arLine(fset, af.rel, id.Pos()), id.Name, arPaneOwnerNames())
			}
			return true
		})
	}
	// A pin over a derived set is satisfied by deriving nothing: the two
	// owners together spell these symbols dozens of times, so a census that
	// matched a handful is a census whose matcher stopped working.
	if named < 20 {
		t.Fatalf("the census matched only %d pane-mode identifiers across %d files — the symbols were renamed and this pin is now guarding nothing", named, len(files))
	}
	// The backend half: it WRITES the field, and the one thing it may do
	// with the value is render it. Reads are collected first because an
	// assignment's left-hand side is the same selector shape as a read.
	arPaneBackendReadsAreRendersOnly(t, fset, files)
}

func arPaneOwnerNames() []string {
	out := make([]string, 0, len(arPaneOwners))
	for f := range arPaneOwners {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// arPaneBackendReadsAreRendersOnly holds the listing backend to rendering:
// every `x.PermissionMode` that is not an assignment target must be the
// receiver of the token/sentence renderer. ADR 0035 §4 is the reason — a mode
// is a default DISPOSITION, never a promise a session cannot block — and a
// backend that compared one, or passed one on, would be the first branch.
func arPaneBackendReadsAreRendersOnly(t *testing.T, fset *token.FileSet, files []arFile) {
	const backend = "internal/posse/herdrback.go"
	var af *arFile
	for i := range files {
		if files[i].rel == backend {
			af = &files[i]
		}
	}
	if af == nil {
		t.Fatalf("%s is not in the walk — the file moved and this arm is checking nothing", backend)
	}
	written, rendered := map[token.Pos]bool{}, map[token.Pos]bool{}
	ast.Inspect(af.file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "PermissionMode" {
					written[sel.Pos()] = true
				}
			}
		case *ast.SelectorExpr:
			// x.PermissionMode.Tag() — the outer selector's receiver is the
			// inner one, which is the read being rendered.
			if x.Sel.Name != "Tag" && x.Sel.Name != "Line" {
				return true
			}
			if inner, ok := x.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "PermissionMode" {
				rendered[inner.Pos()] = true
			}
		}
		return true
	})
	seen := 0
	ast.Inspect(af.file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "PermissionMode" {
			return true
		}
		seen++
		if written[sel.Pos()] || rendered[sel.Pos()] {
			return true
		}
		t.Errorf("%s reads the pane mode as something other than a rendered token.\n"+
			"ADR 0035 §4: a mode is a default DISPOSITION, never a promise a session cannot block. The listing\n"+
			"prints it; nothing may decide on it, and a value passed on is a decision waiting to be made\n"+
			"somewhere this census cannot see.", arLine(fset, af.rel, sel.Pos()))
		return true
	})
	if seen < 5 || len(written) == 0 || len(rendered) == 0 {
		t.Errorf("the backend arm saw %d PermissionMode selectors, %d written and %d rendered — it must see both halves, or a green here is a matcher that stopped matching", seen, len(written), len(rendered))
	}
}
