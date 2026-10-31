package treepins

// QA pin for ranger-base-0ckgm, widened by ranger-base-pyl22: in this
// package, a tree path handed to a walking helper THROUGH A PARAMETER is a
// tree path the register derives.
//
// WHY THERE IS A PIN HERE AT ALL. The Makefile's tree-wide doors are derived,
// not listed (treewidedoor_qa_test.go, arm 2): the internal/treepins half of
// the class is every test that reaches a directory ENUMERATION rooted at a
// relative tree path. For as long as that rule read a literal and nothing
// else, the class was bounded by a SPELLING — a tree reading written another
// way was not in the class, was never asked for a door, and was reachable
// only by an unfiltered 589.965s package run. Nothing said so when it
// happened, because every signal is green: the pin passes, the register
// passes, the doors pass.
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
// WHAT CHANGED, AND WHY THIS PIN IS NOW A COUPLING (ranger-base-pyl22). The
// escape above was closed by respelling one helper's root, and this pin was
// the rule that kept the next one from arriving: a param-rooted walk handed a
// tree path was reported as a defect, with the two helpers it could not
// respell — arParse and cgScan, both genuinely polymorphic — carried in a
// waiver that named the hole. That waiver is gone, because the derivation
// reads the pair now. twdParamHops resolves ONE argument at ONE call site,
// against the walker table rpParamRootedWalkers builds BELOW, and the five
// pins that sat behind those two helpers are derived and doored
// ($(QA_CORPUS_PINS)).
//
// So the pair is no longer a defect and the pin is no longer a prohibition.
// THE RULE NOW: every pair this file finds — a function that roots a walk at
// one of its OWN PARAMETERS, directly or through a local assigned from it,
// and a call site handing it a path INSIDE THE TREE — must be a pair the
// register's hop READ, as evidence in the caller's relEnum. A fixture root is
// fine and is most of them: a helper that walks a t.TempDir is not a reading
// of the tree and no door is owed.
//
// It is a coupling and it is written down as one. rpScan resolves the call
// site's argument with twdRelTreePath and the hop with twdRelTreePaths, which
// is a superset today, so the live answer is that every pair is read — and
// the day somebody narrows the hop (to one walker, to argument zero, to a
// bare identifier) this reds with the pair it dropped, instead of five pins
// quietly leaving the class the way they quietly sat outside it. That is the
// same thing arm 2 does for doors, one level down: the rule is wide enough.
//
// THE CENSUS THIS RESTS ON (MEASURED 2026-10-31, this worktree, by the
// program kept at docs/notes.d/ranger-base-0ckgm.md). Over
// internal/posse/*_test.go and internal/treepins/*_test.go: 25 functions root
// a walk at one of their own parameters, and exactly 3 call sites hand one of
// them a tree path — `scanRealBdCallSites(".")`, the one this bead fixed, and
// two in internal/posse, `assertNoHintVocabulary(t, ".")` and
// `scanReExecSites(".")`. So the shape is rare, which is why a rule about it
// can be a rule and not a register. (That census counted the pairs it
// reported as NEWS. The arm below counts every pair it finds, waiving
// nothing, which in this package is six: arParse at four call sites and
// cgScan at two.)
//
// WHY internal/posse IS NOT HELD HERE. The two call sites there root at the
// PACKAGE directory — internal/posse's tests run with their own package as
// cwd and climb to the repo root through the ONE helper twdRootHelper names
// — so a relative literal in that package is not a tree path at all, and that
// half of the class is derived from the climb rather than from the root. Those two are a different question (is a census of one package's own
// sources in the class?), filed rather than decided here.
//
// THE RESIDUAL, said out loud, because it is what is left after three
// widenings rather than what one of them missed. The other five spellings
// this census found — a local assigned a literal one line above the walk, a
// package const, a range variable over a literal list, a root returned by
// repoRoot(), and the parameter — are all read now (ranger-base-8wxbn for
// the first four, this bead for the last). What neither this rule nor the
// hop reads is a CHAIN: a helper that takes a root as its own parameter and
// hands it on to a param-rooted walker, with the literal two call sites away.
// Both stop at one hop, deliberately and identically, so neither can be
// green over something the other would catch — and the thing that keeps the
// chain from being written is this pin's floor plus the fact that nothing in
// the package is spelled that way (MEASURED 2026-10-31). Following it would
// be the transitive propagation twdRelEnv's head comment refuses, and the
// refusal stands: what was revisited here is the ONE-step resolution, which
// is not propagation and which rpScan had already shipped under a fixture
// arm.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"sort"
	"strings"
	"testing"
)

// rpRootWalker is one (function, parameter) pair that roots a walk at that
// parameter: the spelling the register reads one hop later, at the call site,
// and not from the body in front of it.
type rpRootWalker struct {
	fn    string
	param string
	idx   int
	walk  string
	where string
}

// rpHandoff is one call site that hands such a function a path inside the
// tree — a reading of the tree whose door can only be derived through the
// hop, which is what the arm below holds.
type rpHandoff struct {
	caller string
	helper string
	param  string
	// walk is the walker the helper roots at that parameter (Walk, WalkDir,
	// ReadDir, Glob). It is here because the register's parameter hop names
	// it in the evidence it records, and the pin below compares the two.
	walk  string
	path  string
	where string
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

// rpDecl is one function declaration of the parsed sources, with the
// file:line its name sits on.
type rpDecl struct {
	fd    *ast.FuncDecl
	where string
}

// rpDecls flattens the parsed files into declarations, in filename order so
// that two runs over the same sources report in the same order.
func rpDecls(fset *token.FileSet, files map[string]*ast.File) []rpDecl {
	var decls []rpDecl
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
			decls = append(decls, rpDecl{fd, fmt.Sprintf("%s:%d", name, fset.Position(fd.Pos()).Line)})
		}
	}
	return decls
}

// rpParamRootedWalkers finds every (function, parameter) pair that roots a
// walk at that parameter — directly, or through a local assigned from it.
//
// IT IS THE REGISTER'S WALKER TABLE TOO, which is why it is a function of
// its own rather than a loop inside rpScan (ranger-base-pyl22). The
// derivation in treewidedoor_qa_test.go resolves such a parameter ONE STEP,
// at the call site, and it reads the pairs from here — so the pin below and
// the derivation cannot disagree about what a param-rooted walk is. A second
// copy of this loop, written to be easy to read from over there, is a copy
// that can go narrower than this one while both look green.
func rpParamRootedWalkers(fset *token.FileSet, files map[string]*ast.File) (walkers []rpRootWalker, byName map[string][]rpRootWalker) {
	byName = map[string][]rpRootWalker{}
	for _, d := range rpDecls(fset, files) {
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
	return walkers, byName
}

// rpScan is the rule, over parsed sources: the param-rooted walkers, and the
// call sites that hand one of them a tree path. The fixture arm below drives
// THIS function, so a fixture that passes cannot be passing against a
// second, friendlier copy of the rule.
func rpScan(fset *token.FileSet, files map[string]*ast.File) (walkers []rpRootWalker, handoffs []rpHandoff) {
	walkers, byName := rpParamRootedWalkers(fset, files)
	for _, d := range rpDecls(fset, files) {
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
					walk:   w.walk,
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

// TestQAEveryTreePathHandedThroughAParameterIsDerived is the coupling the
// head comment describes: every param-rooted walker handed a tree path here
// is a pair twdParamHops read, recorded as evidence in the caller's relEnum.
//
// Both halves come from the same sources and the same walker table, so what
// can differ between them is the resolution of the call site's ARGUMENT —
// twdRelTreePath here, twdRelTreePaths there — and that is exactly the thing
// a narrowing of the hop would change. The door this buys is arm 2's: once
// the hop has read the pair, the caller is in the derived class and
// TestQAEveryTreeWidePinHasADoor asks the Makefile for a door. This arm does
// not re-ask that question; it holds the step before it.
func TestQAEveryTreePathHandedThroughAParameterIsDerived(t *testing.T) {
	t.Parallel()
	fset, files := rpPackageSources(t)
	walkers, handoffs := rpScan(fset, files)

	// The positive witness, both ways. An assertion about pairs is satisfied
	// by a rule that matched no pair at all — a renamed walker set, a parse
	// that read no bodies — and by 2026-10-31 this package held 6
	// param-rooted walks and 6 handoffs over two helpers (arParse at four
	// call sites, cgScan at two). A package with this many fixture helpers
	// will always have some of the first; the second is what the coupling is
	// about, so a scan that finds none of those is reporting nothing.
	if len(walkers) == 0 {
		t.Fatal("no walk in this package is rooted at a parameter at all (6 were on 2026-10-31) — this rule is matching nothing, not passing")
	}
	if len(handoffs) == 0 {
		t.Fatal("no call site in this package hands a param-rooted walker a tree path (6 did on 2026-10-31, over arParse and cgScan) — so this arm is comparing an empty set with the register's hop and would pass however narrow that hop became")
	}
	t.Logf("read %d param-rooted walks in %s, %d of them handed a tree path", len(walkers), twdTreepinsPkg, len(handoffs))

	all, _ := twdParseTreepins(t)
	for _, h := range handoffs {
		f := all[h.caller]
		if f == nil {
			t.Errorf("%s hands %s the tree path %q at %s, and the register's own parse of this package declares no function of that name — the two readings have come apart, so nothing here can say whether the hop saw this pair", h.caller, h.helper, h.path, h.where)
			continue
		}
		want := twdHopEntry(rpRootWalker{fn: h.helper, walk: h.walk}, h.path)
		if slices.Contains(f.relEnum, want) {
			continue
		}
		t.Errorf("%s hands %s the tree path %q at %s, and %s roots a %s at that very parameter (%s) — "+
			"so this is a reading of the tree, and the register's parameter hop did not record it "+
			"(%s's enumerations are %v, wanted one of them to be %q). Until it does, no Makefile door "+
			"will ever be asked for and %s is reachable only by an unfiltered %s run. Either widen the "+
			"hop in treewidedoor_qa_test.go to read this spelling, or spell the root at the walk the way "+
			"scanRealBdCallSites does (bddaemonleak_qa_test.go, ranger-base-0ckgm).",
			h.caller, h.helper, h.path, h.where, h.helper, h.walk, h.param, h.caller, f.relEnum, want, h.caller, twdTreepinsPkg)
	}
}

// The fixture arm. The rule above is a rule about a SPELLING, and a rule that
// stops matching leaves a green pin over the hole it was written for, so each
// shape it must and must not match is typed here and driven through the same
// rpScan the live arm uses. Both directions, because either is the bug: a
// rule that matches nothing passes over the escape, and a rule that matches
// everything makes every fixture helper in the package a false red.
//
// rpScan is unchanged by ranger-base-pyl22 and so is every case below — what
// changed is the VERDICT the live arm reaches on a pair, and the register
// reading the same pairs rather than nothing. The hop's own edges are typed
// on the other side of the coupling, in
// TestQATheTreepinsEnumerationRuleMatchesWhatItClaims, where the parameter
// case flipped from false to true.
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
