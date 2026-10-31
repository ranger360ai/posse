package treepins

// QA pin for ranger-base-0ckgm: in this package, a tree walk SPELLS ITS ROOT.
//
// WHY THIS IS A RULE ABOUT A SPELLING. The Makefile's tree-wide doors are
// derived, not listed (treewidedoor_qa_test.go, arm 2): the internal/treepins
// half of the class is every test that reaches a directory ENUMERATION rooted
// at a relative literal — `twdRelTreePath` over `twdWalkers` — and
// twdRelTreePath reads a literal and cannot read a variable. That is a
// deliberate refusal: following a root through a parameter is interprocedural
// constant propagation, a second thing to be wrong about. The price of the
// refusal is that the class is bounded by a SPELLING, and a tree reading
// written in another spelling is not in the class, is never asked for a door,
// and is reachable only by an unfiltered 589.965s package run. Nothing said
// so when it happened, because every signal is green: the pin passes, the
// register passes, the doors pass.
//
// It happened. ranger-base-42mv's daemon-leak census
// (TestQANoTestShellsRealBdWithoutStoppingItsDaemon, bddaemonleak_qa_test.go)
// walked the repo root and read every _test.go in the tree through
// `scanRealBdCallSites(root string)` — `filepath.Walk(root, …)`, with the one
// literal spelled at the call site in the test body. For two months it was a
// tree-wide pin with no door, and the register never asked. Its root is
// spelled at the walk now, which is the whole fix; this pin is what keeps the
// next one from arriving the same way.
//
// THE RULE: a function in this package's test sources that roots a walk at
// one of its OWN PARAMETERS — directly, `Walk(root)`, or through a local
// assigned from it, `p := filepath.Join(root, d); Walk(p)` — must not be
// handed a path INSIDE THE TREE by any call site here. A fixture root is
// fine and is most of them: a helper that walks a t.TempDir is not a reading
// of the tree and no door is owed. What is flagged is the pair — the
// parameter and the tree path that reaches it — because that pair is exactly
// a tree reading the derivation cannot see.
//
// THE CENSUS THIS RESTS ON (MEASURED 2026-10-31, this worktree, by the
// program kept at docs/notes.d/ranger-base-0ckgm.md). Over
// internal/posse/*_test.go and internal/treepins/*_test.go: 25 functions root
// a walk at one of their own parameters, and exactly 3 call sites hand one of
// them a tree path — `scanRealBdCallSites(".")`, the one this bead fixed, and
// two in internal/posse, `assertNoHintVocabulary(t, ".")` and
// `scanReExecSites(".")`. So the shape is rare, which is why a rule about it
// can be a rule and not a register.
//
// WHY internal/posse IS NOT HELD HERE. The two call sites there root at the
// PACKAGE directory — internal/posse's tests run with their own package as
// cwd and climb to the repo root through the ONE helper twdRootHelper names
// — so a relative literal in that package is not a tree path at all, and that
// half of the class is derived from the climb rather than from the root. Those two are a different question (is a census of one package's own
// sources in the class?), filed rather than decided here.
//
// THE RESIDUAL, said out loud, because it is bigger than what this pin
// holds. A parameter is one of several spellings the derivation cannot read,
// and the same census found the others live in this package today: a local
// variable assigned a literal one line above the walk
// (`dir := filepath.Join("docs", "adr"); os.ReadDir(dir)`), a named constant
// (`os.ReadDir(armPkgDir)`), a range variable over a package-level list of
// literals (`for _, d := range arRoots`), and a root returned by a helper
// (`root, _ := repoRoot()`). SEVENTEEN tests in internal/treepins read the
// tree through one of those and are neither derived nor doored, against the
// thirty that are. That is a derivation to widen, not more spellings to pin —
// intraprocedural assignment tracking, which this package already does for
// the other half of the class (`twdRootExprs`) — and it is filed with the
// list, as ranger-base-8wxbn. The two rows in rpWaived below are that bead's, named
// here so the hole is counted rather than silent.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// rpRootWalker is one (function, parameter) pair that roots a walk at that
// parameter: the spelling the register cannot read.
type rpRootWalker struct {
	fn    string
	param string
	idx   int
	walk  string
	where string
}

// rpHandoff is one call site that hands such a function a path inside the
// tree — a reading of the tree in a spelling no door can be derived for.
type rpHandoff struct {
	caller string
	helper string
	param  string
	path   string
	where  string
}

// rpWaived are the param-rooted helpers this pin reports as known rather
// than as news, keyed by helper name. One row per helper, not per call site:
// the question is about the helper's root, and a second call site with the
// same tree path says nothing new.
//
// A row here is a DEFECT RECORDED, never a shape blessed: both of these are
// real tree readings that no Makefile door names, and both are listed on
// ranger-base-8wxbn, which widens the derivation to reach them and takes
// these rows out when it lands. They are here so that the hole is a number
// this file prints rather than a silence — and so that a NEW pair reds,
// which is the whole point of a rule whose live answer is zero.
var rpWaived = map[string]string{
	"arParse": "absencerules_qa_test.go: joins its `root` parameter with each entry of the " +
		"package-level arRoots (`internal`, `cmd`) and walks every non-test .go file under " +
		"them; four pins call it with `.` and are tree-wide and undoored (ranger-base-8wxbn)",
	"cgScan": "credentialgate_qa_test.go: joins its `root` parameter with each of " +
		"cgScannedDirs (`scripts`, `plugin`, `etc`, `docs`) and walks every runnable file " +
		"under them; TestNoShippedScriptAcquiresACredential passes `.` at two call " +
		"sites, and that pin is tree-wide and undoored (ranger-base-8wxbn)",
}

// rpParamIndex maps each named parameter of a function to its position in the
// flattened parameter list, which is what a call site's argument list is
// indexed by.
func rpParamIndex(fd *ast.FuncDecl) map[string]int {
	out := map[string]int{}
	if fd.Type.Params == nil {
		return out
	}
	i := 0
	for _, field := range fd.Type.Params.List {
		if len(field.Names) == 0 {
			i++
			continue
		}
		for _, n := range field.Names {
			out[n.Name] = i
			i++
		}
	}
	return out
}

// rpRootParam resolves the walk root e to the parameter it carries, if it
// carries one: the parameter itself, a local known to carry it, or either of
// those inside a filepath.Join/Clean/ToSlash/FromSlash. The path-joining
// cases are the point — `filepath.Join(root, d)` is how both waived helpers
// above spell it, and a rule that read only the bare identifier would call
// them clean.
func rpRootParam(e ast.Expr, params map[string]int, carries map[string]string) (string, bool) {
	switch v := e.(type) {
	case *ast.Ident:
		if _, ok := params[v.Name]; ok {
			return v.Name, true
		}
		if p, ok := carries[v.Name]; ok {
			return p, true
		}
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok || len(v.Args) == 0 {
			return "", false
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "filepath" {
			return "", false
		}
		switch sel.Sel.Name {
		case "Join", "Clean", "ToSlash", "FromSlash", "Abs":
			return rpRootParam(v.Args[0], params, carries)
		}
	}
	return "", false
}

// rpCarriers reads one function body for the locals that carry a parameter:
// `p := filepath.Join(root, d)` makes p carry root. Iterated to a fixed
// point, so a second hop costs nothing — one hop is what the tree writes and
// two is what a refactor writes.
//
// INTRAPROCEDURAL, which is the whole difference from the propagation the
// register refused: everything read here is in the body in front of it.
func rpCarriers(body *ast.BlockStmt, params map[string]int) map[string]string {
	carries := map[string]string{}
	for changed := true; changed; {
		changed = false
		ast.Inspect(body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, lhs := range as.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || i >= len(as.Rhs) {
					continue
				}
				if _, already := carries[id.Name]; already {
					continue
				}
				if _, isParam := params[id.Name]; isParam {
					continue
				}
				if p, ok := rpRootParam(as.Rhs[i], params, carries); ok {
					carries[id.Name] = p
					changed = true
				}
			}
			return true
		})
	}
	return carries
}

// rpScan is the rule, over parsed sources: the param-rooted walkers, and the
// call sites that hand one of them a tree path. The fixture arm below drives
// THIS function, so a fixture that passes cannot be passing against a
// second, friendlier copy of the rule.
func rpScan(fset *token.FileSet, files map[string]*ast.File) (walkers []rpRootWalker, handoffs []rpHandoff) {
	type decl struct {
		fd    *ast.FuncDecl
		where string
	}
	var decls []decl
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, d := range files[name].Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Body == nil {
				continue
			}
			decls = append(decls, decl{fd, fmt.Sprintf("%s:%d", name, fset.Position(fd.Pos()).Line)})
		}
	}

	byName := map[string][]rpRootWalker{}
	for _, d := range decls {
		params := rpParamIndex(d.fd)
		if len(params) == 0 {
			continue
		}
		carries := rpCarriers(d.fd.Body, params)
		seen := map[string]bool{}
		ast.Inspect(d.fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !twdWalkers[sel.Sel.Name] {
				return true
			}
			param, ok := rpRootParam(call.Args[0], params, carries)
			if !ok {
				return true
			}
			key := param + "/" + sel.Sel.Name
			if seen[key] {
				return true
			}
			seen[key] = true
			w := rpRootWalker{
				fn:    d.fd.Name.Name,
				param: param,
				idx:   params[param],
				walk:  sel.Sel.Name,
				where: fmt.Sprintf("%s:%d", strings.SplitN(d.where, ":", 2)[0], fset.Position(call.Pos()).Line),
			}
			walkers = append(walkers, w)
			byName[w.fn] = append(byName[w.fn], w)
			return true
		})
	}

	for _, d := range decls {
		ast.Inspect(d.fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			for _, w := range byName[id.Name] {
				if len(call.Args) <= w.idx {
					continue
				}
				rel, ok := twdRelTreePath(call.Args[w.idx])
				if !ok {
					continue
				}
				handoffs = append(handoffs, rpHandoff{
					caller: d.fd.Name.Name,
					helper: w.fn,
					param:  w.param,
					path:   rel,
					where:  fmt.Sprintf("%s:%d", strings.SplitN(d.where, ":", 2)[0], fset.Position(call.Pos()).Line),
				})
			}
			return true
		})
	}
	return walkers, handoffs
}

// rpPackageSources parses this package's test files, through the same glob
// the register derives the class from — spelled as a literal there, which is
// also what makes this pin a member of the class it guards.
func rpPackageSources(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, path := range twdTestFiles(t, twdTreepinsPkg) {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files[path] = f
	}
	return fset, files
}

func TestQANoTreeWalkingHelperIsHandedATreePathThroughAParameter(t *testing.T) {
	t.Parallel()
	fset, files := rpPackageSources(t)
	walkers, handoffs := rpScan(fset, files)

	// The positive witness. This pin's live answer is zero handoffs, and an
	// assertion of absence is satisfied by a rule that matched nothing at
	// all — a renamed walker set, a parse that read no bodies. So say what
	// was read, and fail a scan that found no param-rooted walker anywhere:
	// there were 6 in this package on 2026-10-31, and a package with this
	// many fixture helpers will always have some.
	if len(walkers) == 0 {
		t.Fatal("no walk in this package is rooted at a parameter at all (6 were on 2026-10-31) — this rule is matching nothing, not passing")
	}
	t.Logf("read %d param-rooted walks in %s, %d of them handed a tree path", len(walkers), twdTreepinsPkg, len(handoffs))

	waivedSeen := map[string]bool{}
	for _, h := range handoffs {
		if reason, ok := rpWaived[h.helper]; ok {
			waivedSeen[h.helper] = true
			t.Logf("known, and recorded rather than reported: %s(%q) at %s — %s", h.helper, h.path, h.where, reason)
			continue
		}
		t.Errorf("%s hands %s the tree path %q, and %s roots a walk at that very parameter (%s) — "+
			"so this is a reading of the tree in the one spelling the register's derivation cannot see, "+
			"which means no Makefile door will ever be asked for and %s is reachable only by an "+
			"unfiltered %s run. Spell the root at the walk, the way scanRealBdCallSites does "+
			"(bddaemonleak_qa_test.go, ranger-base-0ckgm), and give the pin a door.",
			h.caller, h.helper, h.path, h.helper, h.param, h.caller, twdTreepinsPkg)
	}

	// And the waiver cannot keep rows for pairs that are gone: a recorded
	// defect that has been fixed is a reason carried for a decision nobody
	// has to take any more, and the next reader cannot tell it from a live
	// one.
	var dead []string
	for helper := range rpWaived {
		if !waivedSeen[helper] {
			dead = append(dead, helper)
		}
	}
	sort.Strings(dead)
	for _, helper := range dead {
		t.Errorf("rpWaived records %s and nothing in %s hands it a tree path any more — drop the row rather than leave a waiver for a reading that does not happen", helper, twdTreepinsPkg)
	}
}

// The fixture arm. The rule above is a rule about a SPELLING, and a rule that
// stops matching leaves a green pin over the hole it was written for, so each
// shape it must and must not match is typed here and driven through the same
// rpScan the live arm uses. Both directions, because either is the bug: a
// rule that matches nothing passes over the escape, and a rule that matches
// everything makes every fixture helper in the package a false red.
func TestQATheRootParameterRuleMatchesWhatItClaims(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		src  string
		want int // handoffs
	}{
		{"a param root handed a tree path", `func scan(root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { scan(".") }`, 1},
		{"a param root handed a scratch dir", `func scan(root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { scan(t.TempDir()) }`, 0},
		{"a param root reached through a local join", `func scan(root string, dirs []string) {
	for _, d := range dirs {
		p := filepath.Join(root, d)
		filepath.Walk(p, nil)
	}
}
func TestX(t *testing.T) { scan(".", nil) }`, 1},
		{"a param root reached through two locals", `func scan(root string) {
	a := filepath.Clean(root)
	b := filepath.Join(a, "docs")
	filepath.WalkDir(b, nil)
}
func TestX(t *testing.T) { scan("internal") }`, 1},
		{"the root spelled at the walk is not a handoff", `func scan() { filepath.Walk(".", nil) }
func TestX(t *testing.T) { scan() }`, 0},
		{"a local from t.TempDir is not a param root", `func scan(t *testing.T) { dir := t.TempDir(); filepath.Walk(dir, nil) }
func TestX(t *testing.T) { scan(t) }`, 0},
		{"a climb out of the tree is not a tree path", `func scan(root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { scan("../..") }`, 0},
		{"an absolute path is not a tree path", `func scan(root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { scan("/etc") }`, 0},
		{"the tree path must reach the walked parameter", `func scan(t *testing.T, name string, root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { scan(t, ".", t.TempDir()) }`, 0},
		{"and is found at the parameter's own position", `func scan(t *testing.T, name string, root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { scan(t, "name", "docs") }`, 1},
		{"a reader of one named file is not a walk", `func scan(root string) { os.ReadFile(root) }
func TestX(t *testing.T) { scan("Makefile") }`, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "fixture_test.go", "package treepins\n"+c.src+"\n", 0)
			if err != nil {
				t.Fatalf("parse fixture: %v\n%s", err, c.src)
			}
			_, handoffs := rpScan(fset, map[string]*ast.File{"fixture_test.go": f})
			if len(handoffs) != c.want {
				t.Errorf("the rule finds %d handoffs in this source, want %d — the key the pin above rests on no longer means what this file says it means:\n%s\ngot %+v", len(handoffs), c.want, c.src, handoffs)
			}
		})
	}
}
