package main

// ranger-base-1jmtm: the usage catalog gives a sub-verb its own header line —
// and three entries' worth of sub-verbs the switch accepts had none to print.
//
// ranger-base-rjfec makes `posse <verb> -h/--help` read the catalog once,
// ahead of main's switch, so a subcommand path answers for itself WHEN THE
// CATALOG NAMES IT AS A PATH. That reading cannot reach a sub-verb the
// catalog never spelled. MEASURED 2026-10-09 by ranger-base-rjfec, in that
// bead's own worktree against a scratch RHQ_HOME, on a binary carrying that
// reading — which is why these four are what asking for help actually got,
// and not merely what a missing entry looks like:
//
//	$ posse gates wrap --help    posse: posse gates wrap: nothing to run after `--`
//	$ posse skills list --help   <RHQ_HOME>/skills — skills bound by PIDs (…)  RAN
//	$ posse env edit --help      posse: bad env set name '--help'
//	$ posse env rm --help        posse: env set not found: --help
//
// Each had its own cause and the catalog was the single fix: `gates wrap` had
// no entry at all, `skills [list]` bracketed its only sub-verb so `skills
// list` was not a path, and `env edit|rm <name>` fused two sub-verbs behind
// one pipe so neither was. Three entries, four sub-verb spellings.
//
// ORDER: this pin and its catalog fix do not wait on that reading and are
// not waiting for it — the census here is the SWITCH against the CATALOG,
// which is a claim about this file and main.go alone, and it is already the
// claim worth holding. What ranger-base-rjfec adds is the consequence: the
// entries become what `-h/--help` prints. If you are reading this before
// that bead lands, catalogLead is not in the tree yet and nothing below
// mentions it.
//
// The convention was never in doubt — the catalog had already answered this
// question eleven times (`agent new|edit|check`, `runtime check|probe|filing`,
// `cage build|down`, `gates install-hooks|managed-hooks|adr-census`, `backup
// status|verify` each own a header). What was missing is the thing that makes
// the answer hold for the sub-verb added tomorrow, which is this pin: the
// switch's sub-verbs censused from the AST, against the catalog's own paths.
//
// Why the AST and not a probe: a sub-verb that is missing from the catalog
// does not fail — it RUNS, or it errors about the name '--help', which is
// exactly how all four survived. There is no exit code to assert on. The
// switch is the only place that knows the accepted set, so the census reads
// the switch.
//
// WHAT THIS CENSUS CANNOT SEE, stated so a later reader does not mistake its
// silence for a clean bill: a sub-verb is only visible to it when main's
// switch decides on a STRING COMPARE against args[0]. Two shapes are
// deliberately out of reach, and neither is a sub-verb today —
//
//   - a positional consumed BY VALUE rather than compared. `posse refresh
//     <runtime> [session|meter]` is the live example: its loop reads
//     `ro.Purpose = posse.CredPurpose(v)` and never compares v to "session"
//     or "meter", so no literal exists for the census to find. That is
//     correct here — those are argument values of one verb, which the
//     `posse refresh <runtime> [session|meter]` entry documents, not paths
//     of their own.
//   - a dispatch moved out of main into a helper. `runtimeProbe`'s
//     `switch args[0]` is such a switch today and is a FLAG loop, which is
//     why nothing is lost; a real sub-verb dispatch relocated there would
//     leave this pin green.
//
// So this pin holds the rule for the shapes main writes, and a sub-verb
// introduced in either shape above needs it taught, not merely re-run.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// subVerbAliases are the sub-verb spellings that are deliberately NOT catalog
// paths: a second spelling of a sub-verb the catalog names under its
// canonical one. The catalog's house style for an alias is prose on the
// canonical entry — `posse attach <name>  … (alias: focus)` — not a header of
// its own, so an alias has no block to print and wants none.
//
// This map is small on purpose and is the one place the alias/canonical call
// is recorded, because the AST cannot make it: `case "edit", "new":` and
// `case "new", "edit":` are the same tree, and `posse agent`'s `case "new",
// "edit":` is two CANONICAL sub-verbs in exactly that shape. Position in the
// case clause carries no meaning. Each entry is checked both ways below — the
// canonical spelling must be a catalog path, and the catalog must name the
// alias in that entry's prose — so an alias cannot be used here to park a
// sub-verb nobody documented.
var subVerbAliases = map[string]string{
	"env new":    "env edit",
	"env delete": "env rm",
}

// subVerbs returns every sub-verb spelling main's switch accepts, as
// "<verb> <sub-verb>", read from the AST: a string compare against args[0]
// inside a top-level case body, in any of the shapes main actually writes —
// `switch args[0] { case "x", "y": }`, `if args[0] == "x"`, `if len(args) > 0
// && args[0] == "x"`, and `args[0] != "x" && args[0] != "y"`.
func subVerbs(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	sw := mainVerbSwitch(t, f)
	var found []string
	for _, stmt := range sw.Body.List {
		cc, ok := stmt.(*ast.CaseClause)
		if !ok || cc.List == nil { // nil List is `default:`
			continue
		}
		verbs := caseLits(cc.List)
		// `posse env` reads its sub-verb through a local — `sub, name :=
		// args[0], args[1]` — and then switches on that, so the census
		// follows a one-hop binding as well as args[0] itself. Without
		// this it censused env's case as having no sub-verbs at all and
		// passed over both of the ones this bead was filed about.
		names := argZeroNames(cc)
		subs := map[string]bool{}
		for _, s := range cc.Body {
			for _, v := range argZeroLits(s, names) {
				// "" is the bare form, which the verb's own entry is; a
				// flag is not a sub-verb — argLead does not treat one as a
				// path either, and the catalog documents a verb's flags on
				// the verb's own entry (`posse scorecard --catalog`).
				if v != "" && !strings.HasPrefix(v, "-") {
					subs[v] = true
				}
			}
		}
		if len(subs) == 0 {
			continue
		}
		// A multi-spelling verb that also takes sub-verbs would make
		// "<verb> <sub-verb>" ambiguous — which spelling does the catalog
		// have to name? Nothing in main writes that shape today; if
		// something starts to, this pin must be taught the answer rather
		// than silently censusing one arbitrary half of it.
		if len(verbs) != 1 {
			t.Fatalf("case %q takes sub-verbs %v and has %d verb spellings — this pin does not know which spelling the catalog must name", verbs, keys(subs), len(verbs))
		}
		for _, s := range keys(subs) {
			found = append(found, verbs[0]+" "+s)
		}
	}
	sort.Strings(found)
	if len(found) == 0 {
		t.Fatal("censused no sub-verbs at all — this pin is reading nothing")
	}
	return found
}

// mainVerbSwitch finds main()'s `switch cmd` — the top-level verb dispatch.
func mainVerbSwitch(t *testing.T, f *ast.File) *ast.SwitchStmt {
	t.Helper()
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if d, ok := d.(*ast.FuncDecl); ok && d.Name.Name == "main" && d.Recv == nil {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("cmd/posse/main.go has no main()")
	}
	for _, stmt := range fn.Body.List {
		if sw, ok := stmt.(*ast.SwitchStmt); ok {
			if id, ok := sw.Tag.(*ast.Ident); ok && id.Name == "cmd" {
				return sw
			}
		}
	}
	t.Fatal("main() no longer dispatches on `switch cmd` — this pin is reading nothing")
	return nil
}

// argZeroLits returns every string literal compared against args[0] — or
// against a local bound to it, per names — anywhere inside stmt, in either
// direction and by == or !=, plus the case literals of such a switch.
func argZeroLits(stmt ast.Stmt, names map[string]bool) []string {
	var got []string
	ast.Inspect(stmt, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SwitchStmt:
			if isArgZero(n.Tag, names) {
				for _, s := range n.Body.List {
					if cc, ok := s.(*ast.CaseClause); ok {
						got = append(got, caseLits(cc.List)...)
					}
				}
			}
		case *ast.BinaryExpr:
			if n.Op != token.EQL && n.Op != token.NEQ {
				return true
			}
			if isArgZero(n.X, names) {
				if s, ok := strLit(n.Y); ok {
					got = append(got, s)
				}
			}
			if isArgZero(n.Y, names) {
				if s, ok := strLit(n.X); ok {
					got = append(got, s)
				}
			}
		}
		return true
	})
	return got
}

// isArgZero reports whether e is the expression `args[0]`, or an identifier
// that this case body bound to it.
func isArgZero(e ast.Expr, names map[string]bool) bool {
	if id, ok := e.(*ast.Ident); ok {
		return names[id.Name]
	}
	ix, ok := e.(*ast.IndexExpr)
	if !ok {
		return false
	}
	id, ok := ix.X.(*ast.Ident)
	if !ok || id.Name != "args" {
		return false
	}
	lit, ok := ix.Index.(*ast.BasicLit)
	return ok && lit.Kind == token.INT && lit.Value == "0"
}

// argZeroNames returns the identifiers this case body binds to args[0],
// so a switch on one of them is read as the sub-verb switch it is. One hop
// only, and only within the case: a local assigned from a local is not
// followed, which would be a shape nothing in main writes.
func argZeroNames(cc *ast.CaseClause) map[string]bool {
	names := map[string]bool{}
	for _, stmt := range cc.Body {
		ast.Inspect(stmt, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			// Positional: `sub, name := args[0], args[1]` binds only sub.
			// A single rhs spread over several lhs (a call, a map read)
			// has no positional reading, so it is skipped.
			if len(as.Lhs) != len(as.Rhs) {
				return true
			}
			for i, rhs := range as.Rhs {
				if !isArgZero(rhs, nil) {
					continue
				}
				if id, ok := as.Lhs[i].(*ast.Ident); ok && id.Name != "_" {
					names[id.Name] = true
				}
			}
			return true
		})
	}
	return names
}

func strLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func caseLits(list []ast.Expr) []string {
	var got []string
	for _, e := range list {
		if s, ok := strLit(e); ok {
			got = append(got, s)
		}
	}
	return got
}

func keys(m map[string]bool) []string {
	var got []string
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	return got
}

// catalogPaths returns every command path the usage catalog names with a
// header line of its own, keyed by the path, with the header line as value.
// A header is `  posse <path>…`; the path is everything up to the first
// placeholder, flag or run of two spaces.
func catalogPaths(t *testing.T, out string) map[string]string {
	t.Helper()
	paths := map[string]string{}
	for _, ln := range strings.Split(out, "\n") {
		head := strings.TrimPrefix(ln, "  posse ")
		if head == ln {
			continue
		}
		// Cut the description off first — the same rule helpcatalog_test.go's
		// commandOf uses, a run of two spaces. Without it a header whose
		// description opens on a bare word (`posse backup status   age of
		// the newest…`) reads as a path with the description inside it, and
		// every lookup of the real path misses.
		if i := strings.Index(head, "  "); i >= 0 {
			head = head[:i]
		}
		var words []string
		for _, w := range strings.Fields(head) {
			if strings.HasPrefix(w, "<") || strings.HasPrefix(w, "[") || strings.HasPrefix(w, "-") || strings.HasPrefix(w, "\"") {
				break
			}
			words = append(words, w)
		}
		if len(words) > 0 {
			paths[strings.Join(words, " ")] = ln
		}
	}
	if len(paths) == 0 {
		t.Fatal("read no command headers out of the catalog — this pin is reading nothing")
	}
	return paths
}

func TestCatalogNamesEverySubVerbTheSwitchAccepts(t *testing.T) {
	paths := catalogPaths(t, helpText(t))

	// The control: a sub-verb whose header nobody has ever disputed. If the
	// reader cannot find this one, it cannot find the ones under test.
	if _, ok := paths["agent check"]; !ok {
		t.Fatalf("control: catalog reader found no `posse agent check` header — the reader is broken, not the catalog. Found: %v", keysOf(paths))
	}

	// Say what was read. A census that silently shrinks is the failure mode
	// this whole file exists to prevent, so `-v` prints its subject.
	census := subVerbs(t)
	t.Logf("censused %d sub-verb spellings from main's switch: %s", len(census), strings.Join(census, ", "))

	for _, sv := range census {
		if _, ok := paths[sv]; ok {
			continue
		}
		if canon, ok := subVerbAliases[sv]; ok {
			// An alias earns its exemption twice over, so that this map
			// cannot become the place an undocumented sub-verb hides.
			if _, ok := paths[canon]; !ok {
				t.Errorf("`posse %s` is declared an alias of `posse %s`, which is not itself a catalog path", sv, canon)
			}
			if word := sv[strings.LastIndex(sv, " ")+1:]; !strings.Contains(paths[canon], word) {
				t.Errorf("`posse %s` is declared an alias of `posse %s`, whose catalog entry does not name %q:\n%s", sv, canon, word, paths[canon])
			}
			continue
		}
		t.Errorf("main's switch accepts `posse %s`, and the usage catalog gives it no header line — so `posse %s --help` has no block to print and runs or errors instead (ranger-base-1jmtm)", sv, sv)
	}
}

func keysOf(m map[string]string) []string {
	var got []string
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	return got
}
