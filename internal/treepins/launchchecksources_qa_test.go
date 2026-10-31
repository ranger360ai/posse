package treepins

// QA pin for ranger-base-atvd4 — the membership half of the LAUNCH-CHECK
// reading, derived for the same reason its wall sibling is derived.
//
// THE CATEGORY. A wall is install-gated because the installed binary renders
// it; a launch-time check is install-gated because the installed binary
// DECIDES it. Same consequence — a landed fix that does not guard live seats
// — and until this bead only the wall half was counted. The wall class is
// keyed on a `do not edit` header a render writes about itself
// (wallrendersources_qa_test.go), and a check writes no file to put a header
// on, so that derivation cannot reach this one. MEASURED 2026-10-31 over
// ranger-base-qr7me's own exposure window (installed fe3b8036, main at
// d4d105d2, 29 commits): 2 rendering a wall, 8 deciding a launch-time check,
// two of the eight being the grok project-config detector that was
// measurably guarding no seat.
//
// THE DERIVED CLASS, in two clauses, because a launch-time check has two
// halves and they share no spelling:
//
//   - THE DECIDERS: a non-test internal/posse source declaring a func or
//     method whose signature names `*posse.Parity`. Parity IS the registry
//     every launch-time verdict is reported from, and a file that takes a
//     pointer to it is a file that writes one. Purely syntactic, so no
//     go/types load and no call-graph walk: the signature is the claim.
//
//   - THE DECLARATIONS THEY READ: a non-test internal/posse source that
//     constructs a `posse.Runtime` composite literal. The verdicts read a
//     Runtime — `ProjectConfig`, `ProjectConfigKeys`, `SkillsRoot`,
//     `SkillsVerified` — so a commit that changes what a Runtime SAYS
//     changes what a launch checks just as surely as one that changes the
//     checking code. This clause is why f083e140 counts: it touched
//     runtime.go and nothing else outside tests.
//
// A FLOOR, and LaunchCheckSources says so where it is declared: a launch
// check reached by neither spelling is invisible here, which is why nothing
// keys off the list. The reading always reports the WHOLE lag and always
// names `make install`; the per-category counts only say how loud.
//
// NOT DISJOINT from WallRenderSources, and deliberately not pinned to be — a
// file can render a wall and decide a check, and a commit to it is honestly
// counted in both categories. There is no arm here asserting the two lists
// do not overlap, because the overlap would be a fact about the tree and not
// a defect.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// lcsParityType is the registry a launch-time verdict is reported from. A
// func naming a POINTER to it is writing one; `Parity` by value is a reader
// (Parity.String) and not a decider, so the clause is the pointer.
const lcsParityType = "Parity"

// lcsRuntimeType is the declaration those verdicts read.
const lcsRuntimeType = "Runtime"

// lcsSources derives the two clauses over every non-test Go file in
// internal/posse and returns, per file, which clause put it in the class.
func lcsSources(t *testing.T, root string) (deciders, declarers map[string]bool) {
	t.Helper()
	dir := filepath.Join(root, "internal", "posse")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", dir, err)
	}
	deciders, declarers = map[string]bool{}, map[string]bool{}
	fset := token.NewFileSet()
	seen := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		seen++
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", name, err)
		}
		rel := "internal/posse/" + name
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if lcsNamesParityPointer(fn) {
				deciders[rel] = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if id, ok := cl.Type.(*ast.Ident); ok && id.Name == lcsRuntimeType {
				declarers[rel] = true
			}
			return true
		})
	}
	if seen < 50 {
		// The package is ~200 files. A handful means the walk found the
		// wrong directory, and every arm below would then pass over nothing.
		t.Fatalf("only %d non-test sources under %s — this pin is reading the wrong tree", seen, dir)
	}
	if len(deciders) == 0 {
		t.Fatalf("no file under %s declares a func naming *%s — the registry this pin derives its first clause from is gone", dir, lcsParityType)
	}
	if len(declarers) == 0 {
		t.Fatalf("no file under %s constructs a %s — the declaration this pin derives its second clause from is gone", dir, lcsRuntimeType)
	}
	return deciders, declarers
}

// lcsNamesParityPointer reports whether a func's receiver, parameters or
// results name *Parity. Results count: a func HANDING BACK a verdict is the
// entry point of the whole reading (CheckParity, CheckParityIn), and a file
// holding one is a file whose commits change what a launch decides.
func lcsNamesParityPointer(fn *ast.FuncDecl) bool {
	lists := []*ast.FieldList{fn.Recv, fn.Type.Params, fn.Type.Results}
	for _, fl := range lists {
		if fl == nil {
			continue
		}
		for _, f := range fl.List {
			if lcsIsParity(f.Type) {
				return true
			}
		}
	}
	return false
}

// lcsIsParity reports whether a type expression is Parity or *Parity. Both,
// because a method on the value receiver that returns one (Parity.String) is
// still the reading's own code, and excluding it would key the class on a
// star rather than on the registry.
func lcsIsParity(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.StarExpr:
		return lcsIsParity(v.X)
	case *ast.Ident:
		return v.Name == lcsParityType
	}
	return false
}

// ARM 1: every derived launch-check source is in LaunchCheckSources, so the
// count `posse gates` and the promote epilogue print cannot silently omit a
// check — the silence ranger-base-atvd4 was filed for.
func TestLaunchCheckSourcesNameEveryParityDeciderAndRuntimeDeclarer(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	deciders, declarers := lcsSources(t, root)
	listed := map[string]bool{}
	for _, p := range posse.LaunchCheckSources {
		listed[p] = true
	}
	for rel := range deciders {
		if !listed[rel] {
			t.Errorf("%s declares a func naming *%s — it writes a launch-time verdict — and posse.LaunchCheckSources does not name it, so `posse gates` and `posse promote` would count a lag over this file as deciding no check. That is the gap ranger-base-atvd4 closed for the grok project-config detector. Add it to LaunchCheckSources (wallrenderer.go).", rel, lcsParityType)
		}
	}
	for rel := range declarers {
		if !listed[rel] {
			t.Errorf("%s constructs a %s — it declares what the launch-time checks READ — and posse.LaunchCheckSources does not name it. f083e140 (ranger-base-l12j9, grok's project-config channel) touched exactly this kind of file and nothing else outside tests, so a list without it reads zero over a live gap. Add it to LaunchCheckSources (wallrenderer.go).", rel, lcsRuntimeType)
		}
	}
}

// ARM 2: every listed path is a real file. A pathspec that matches nothing
// makes the launch-check count read zero over a repo where it should read
// more, and `git log -- <nothing>` says so to nobody. Same arm the wall list
// carries, for the same reason and against a second list.
func TestLaunchCheckSourcesAllExist(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if len(posse.LaunchCheckSources) == 0 {
		t.Fatal("posse.LaunchCheckSources is empty")
	}
	for _, p := range posse.LaunchCheckSources {
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil || st.IsDir() {
			t.Errorf("posse.LaunchCheckSources names %s, which is not a file here (%v) — a pathspec that matches nothing renders as \"0 of them deciding a launch-time check\"", p, err)
		}
	}
}

// ARM 3: the list holds nothing the derivation does not reach. The wall list
// has a hand half by design (a render site carrying no header), and this one
// does NOT: both clauses are mechanical, so an entry neither clause finds is
// an entry that will be wrong one refactor from now and reassure the
// operator while it is. A launch check the derivation cannot see belongs in
// a widened clause, named here, not in a hand-appended path.
func TestLaunchCheckSourcesHoldNothingTheDerivationCannotSee(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	deciders, declarers := lcsSources(t, root)
	for _, p := range posse.LaunchCheckSources {
		if deciders[p] || declarers[p] {
			continue
		}
		t.Errorf("posse.LaunchCheckSources names %s and neither clause of this pin reaches it: it declares no func naming *%s and constructs no %s. A hand-held entry goes stale in the direction that prints reassurance, which is the whole reason this class is derived — widen a clause here, or drop the path.", p, lcsParityType, lcsRuntimeType)
	}
}
