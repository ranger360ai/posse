package posse

// QA pins for ranger-base-ik44f — fast doors to the other four tree-wide pins.
//
// THE CLASS, as ranger-base-rulbl named it: "a QA test whose subject is the
// TREE, living inside a package nobody runs whole". internal/posse is ~950s,
// past the 600s a seat can spend in one foreground call, so the standing
// advice is a focused `-run` filter — and `-run` selects by test NAME. A
// tree-wide pin is nobody's subject, so no seat's filter ever names it, and
// the pin is unreachable at exactly the moment it would have mattered.
//
// rulbl shipped the first door (`make fmt-check`) and censused the rest.
// This bead is the other four:
//
//	TestTreeIsGofmtClean                          make fmt-check   ~1.5s
//	TestShippedTreeNamesRolesNotThisCrew          make crew-check  ~2.5s
//	TestShippedStringsNameRolesNotThisCrew        make crew-check
//	TestTestCorpusHidesNoCrewNameBehindAnEscape   make crew-check
//
// and eight more the first census could not see, doored under
// ranger-base-sx2dq (see arm 2):
//
//	TestSeedSurfaceNameCountIsZero                make seed-check    ~0.2s
//	TestSeedConfigLiveKeysAreRead                 make seed-check
//	TestPublicationRootCommitOmitsExcludedPaths   make history-check ~0.5s
//	TestPublicationRootCommitADRsCarryProvenance  make history-check
//	TestPublicationHistoryNeverCarriesTheSeedScript make history-check
//	TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover
//	                                              make doc-check     ~0.1s
//	TestQACageCredDocDoesNotCallTheOnDiskCredentialStale   make doc-check
//	TestQAADR0036StatusLineDoesNotCarryTheRetractedUnbuiltStamp
//	                                              make doc-check
//
// and five more that took their root from `git rev-parse --show-toplevel`,
// doored under ranger-base-xndgk (FINDING 5 of the ranger-base-xtgvp verify):
//
//	TestQAIdentityLiteralsNeverAppearInATrackedPath make identity-check ~0.5s
//	TestIdentityLiteralsNeverAppearInTheHarnessRepoUndispositioned
//	                                              make identity-check
//	TestQAEveryOpsHitInTrackedMarkdownIsRuled     make ops-check       ~2s
//	TestQAOpsShapeTableCanStillSayNo              make ops-check
//	TestShippedExampleTableCoversEveryVersionInGitHistory
//	                                              make history-check   ~3s
//
// and three more that arrived afterwards, each doored by the bead that wrote
// it and given its own membership row in arm 2:
//
//	TestQAADR0035PaneModeSurfaceClaimIsBuilt      make doc-check
//	                                              (ranger-base-vwgt)
//	TestInstancePathFormNeverAppearsInTrackedContentUndispositioned
//	                                              make ops-check
//	TestQAInstancePathCensusCanStillSayNo         make ops-check
//	                                              (both ranger-base-l9ii)
//
// and `make tree-check` is all of them — 40-46s on this box over three runs
// at twenty pins and seven doors — which is the command a seat types after a
// filtered run. (It was 21-41s over four runs at the older, smaller class;
// re-measured under ranger-base-4jogv, because the same sentence that had the
// wrong count was also pricing a class three pins smaller than the one that
// runs. The seconds are NOT pinned — an elapsed-seconds red belongs to the
// box, per the `test` target's own note — but they are measured, not carried.)
//
// THAT SENTENCE IS THE ONLY LIVE COUNT IN THIS FILE, and arm 4 holds it to
// the Makefile, both numerals and the enumeration above it. It read seventeen
// and six until ranger-base-4jogv, and arm 4 is why it is not quoted here:
// the rule allows exactly ONE live count claim in this comment, so a second
// sentence saying a number — even a historical one — reds it. The DOOR number
// entered wrong at d189b623, which wrote "seven doors" over an enumeration of
// eight and was then faithfully decremented by one when the selector door
// went; the PIN number drifted on its own, because the three tests listed
// directly above were added to a door variable by beads that had no reason to
// read this comment. A seat prices `make tree-check` from this sentence, in
// the one file whose whole subject is "the doors are wide enough", so nothing
// but a derivation may say how many there are.
//
// WHY THE DOOR RUNS THE PIN. fmt-check re-runs the TOOL, because gofmt is a
// tool and `gofmt -l` cannot disagree with `go/format`. These four are Go:
// their reading is an ast parse, an unquote and a case-boundary scan, and a
// shell rewrite of that would be a second implementation to keep in sync by
// hand — a door that goes NARROWER than the pin while both look green, which
// is worse than no door at all. So each door is a `-run` filter naming the
// pin, and the only thing left to hold is that the filters name ALL of them.
//
// Four arms — rulbl's three, and a fourth that holds the sentence above:
//
//  1. the doors are wired — `make test` depends on tree-check, tree-check
//     reaches every door, and each door reads only (no `-w`, no `./...`,
//     and `-count=1`, because a door that answers from cache can lie).
//  2. the doors are WIDE ENOUGH, two-way and mechanically: the class is
//     derived by parsing internal/posse/*_test.go, and every member must be
//     named by a Makefile door variable, and every name in those variables
//     must be a member. A tree-wide pin added tomorrow reds this until it is
//     given a door, which is the whole deliverable.
//
//     THAT PROMISE WAS FALSE AS FIRST SHIPPED (ranger-base-sx2dq). The
//     derivation keyed on one identifier, qibRepoRoot, and the tree carried
//     a byte-identical twin of it (qspRepoRoot) plus a hand-rolled
//     `filepath.Abs("../..")`: three tree-wide pins were spelled outside the
//     class, got no door, and this arm said nothing. The twin is folded, the
//     hand-rolled climb goes through the helper, the derivation also follows
//     a test into a helper that WALKS from the root, and
//     TestQAOneRepoRootHelperInTheTestPackage below is what keeps a third
//     copy from re-opening the same hole. Five members became thirteen.
//
//     AND WAS STILL FALSE (ranger-base-xndgk FINDING 5). Both rules key on
//     GO's filesystem calls, and five pins took their root from `git
//     rev-parse --show-toplevel` — stdout from a subprocess, which neither
//     rule can see. One of them censuses every tracked PATH in the
//     repository, one `git grep`s every tracked FILE, one scans every
//     tracked markdown file, one reads the history of every shipped
//     example. All five now take the root from the one helper, that
//     spelling is fenced in TestQAOneRepoRootHelperInTheTestPackage, and
//     thirteen members became eighteen. It has moved both ways since — ADR
//     0016's socket hints and their selector pin went, three later pins
//     arrived — and this line deliberately stops counting there: the live
//     number is the one in the `make tree-check` sentence above, which arm 4
//     derives from the Makefile. The lesson the third rule is NOT:
//     the class was never bounded by how a test spells its root, so the
//     thing that bounds it is the single helper, not another rule.
//  3. the doors can FAIL: `make -n`'s own expansion of each, run for real
//     against a scratch copy of the tree carrying the real drift — a crew
//     name in a shipped file. Clean arm first, so a door that always fails is
//     not mistaken for one that detects.
//  4. the head comment's COUNT is the Makefile's — both numerals derived,
//     and every name in a door variable named up there, so the enumeration
//     the count rests on cannot go short while the count stays green
//     (ranger-base-4jogv).

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The Makefile variables that hold the class. One per door, plus the pin
// whose door is a tool rather than a filter — the union is what arm 2
// measures against the tree.
var twdPinVars = []string{"QA_CREW_PINS", "QA_TOOL_PINS", "QA_SEED_PINS", "QA_HISTORY_PINS", "QA_DOC_PINS", "QA_IDENTITY_PINS", "QA_OPS_PINS"}

// twdRootHelper is the ONE repo-root helper internal/posse's tests may use.
// It is a single identifier on purpose — the class below is derived from it,
// so a second spelling is a member of the class that the derivation cannot
// see. That is not hypothetical: the tree carried qspRepoRoot, byte-for-byte
// identical, and three tree-wide pins hid behind it (ranger-base-sx2dq).
const twdRootHelper = "qibRepoRoot"

// twdVar returns the `|`-separated test names a Makefile variable holds.
func twdVar(t *testing.T, makefile, name string) []string {
	t.Helper()
	for _, line := range strings.Split(makefile, "\n") {
		if !strings.HasPrefix(line, name) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, name))
		if !strings.HasPrefix(rest, ":=") {
			continue
		}
		var out []string
		for _, f := range strings.Split(strings.TrimSpace(strings.TrimPrefix(rest, ":=")), "|") {
			if f = strings.TrimSpace(f); f != "" {
				out = append(out, f)
			}
		}
		return out
	}
	t.Fatalf("the Makefile defines no %s — a door variable the pin below reads is gone, so the class has no recorded membership", name)
	return nil
}

// twdSameSet reports whether two name lists hold the same names, order aside.
func twdSameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	for i := range g {
		if g[i] != w[i] {
			return false
		}
	}
	return true
}

// twdPrereqs returns the prerequisites of a target, from its `target:` line.
func twdPrereqs(t *testing.T, makefile, target string) []string {
	t.Helper()
	for _, line := range strings.Split(makefile, "\n") {
		if strings.HasPrefix(line, target+":") && !strings.HasPrefix(line, target+":=") {
			return strings.Fields(strings.TrimPrefix(line, target+":"))
		}
	}
	t.Fatalf("the Makefile has no `%s` target", target)
	return nil
}

// Arm 1: the doors exist, `make test` opens them, and they only read.
func TestQAMakeTestOpensTheTreeWideDoors(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// The umbrella reaches every door. It carries no recipe of its own, so
	// `make -n tree-check` prints exactly what a seat would have to type.
	tree := strings.Join(twdPrereqs(t, src, "tree-check"), " ")
	for _, door := range []string{"fmt-check", "crew-check", "seed-check", "history-check", "doc-check", "identity-check", "ops-check"} {
		if !strings.Contains(tree, door) {
			t.Errorf("`make tree-check` no longer reaches `%s`, so one tree-wide pin is back to being ~950s away: %q", door, tree)
		}
	}

	// And `make test` reaches the umbrella, so a full run fails on the class
	// in seconds instead of at ~950 (rulbl's reason for fmt-check).
	if deps := strings.Join(twdPrereqs(t, src, "test"), " "); !strings.Contains(deps, "tree-check") {
		t.Errorf("`make test` no longer depends on tree-check: %q", deps)
	}

	if phony := strings.Join(twdPrereqs(t, src, ".PHONY"), " "); !strings.Contains(phony, "tree-check") ||
		!strings.Contains(phony, "crew-check") ||
		!strings.Contains(phony, "identity-check") || !strings.Contains(phony, "ops-check") {
		t.Errorf(".PHONY does not name the new doors — a file of that name in the tree would silence one: %q", phony)
	}

	for _, door := range []struct{ target, variable string }{
		{"crew-check", "QA_CREW_PINS"},
	} {
		recipe := strings.Join(makeRecipe(src, door.target), "\n")
		if recipe == "" {
			t.Errorf("the Makefile has no `%s` target", door.target)
			continue
		}
		// The door reads its filter from the one variable arm 2 measures.
		// Spelled here, so a door quietly narrowed to a literal subset of
		// the class fails rather than passing on a shorter list.
		if !strings.Contains(recipe, "'^($("+door.variable+"))$$'") {
			t.Errorf("`make %s` no longer runs exactly $(%s), anchored — the door and the pin it stands in for can now name different tests:\n%s", door.target, door.variable, recipe)
		}
		if !strings.Contains(recipe, "./internal/posse") {
			t.Errorf("`make %s` no longer names ./internal/posse — a package-tree run is the ~950s this door exists to avoid:\n%s", door.target, recipe)
		}
		if strings.Contains(recipe, "./...") {
			t.Errorf("`make %s` runs the package tree, which is the wall it is a door through:\n%s", door.target, recipe)
		}
		// A door that can answer from cache is a door that can lie: the
		// drift these pins are about arrives as a new FILE in a walked
		// directory, and nothing promises go's test cache key notices one.
		if !strings.Contains(recipe, "-count=1") {
			t.Errorf("`make %s` may answer from go's test cache, so it can report a tree it never read:\n%s", door.target, recipe)
		}
		if strings.Contains(recipe, " -w") || strings.Contains(recipe, "rm ") {
			t.Errorf("`make %s` writes — the point of a door is that a seat can ask the question without changing the tree:\n%s", door.target, recipe)
		}
	}
}

// twdJoinsTwoDotDots reports whether an argument list ENDS in two ".."
// literals — `filepath.Join(dir, "..", "..")`, the climb from internal/posse
// to the repo root written out by hand.
//
// It must be the end of the list, and that is the whole distinction between
// this class and its neighbours: `filepath.Join("..", "..", "examples",
// "agents")` climbs to the root and then goes back DOWN into one directory,
// so a walk from it reads examples/agents and reds only when that directory
// changes. Nine tests do that, none of them tree-wide.
func twdJoinsTwoDotDots(args []ast.Expr) bool {
	if len(args) < 2 {
		return false
	}
	for _, a := range args[len(args)-2:] {
		lit, ok := a.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return false
		}
		if v, err := strconv.Unquote(lit.Value); err != nil || v != ".." {
			return false
		}
	}
	return true
}

// twdWalkers are the calls that turn a repo root into a reading of the whole
// tree. A function that computes the root AND makes one of these is walking
// from the root, and every file in the tree is then its subject.
var twdWalkers = map[string]bool{"WalkDir": true, "Walk": true, "ReadDir": true, "Glob": true}

// twdFunc is one function declared in internal/posse/*_test.go: who it calls,
// and whether its own body reaches outside the package.
type twdFunc struct {
	name  string
	where string
	test  bool
	calls map[string]bool
	// root: the body calls qibRepoRoot, the package's ONE repo-root helper.
	root bool
	// rootedWalk: the body computes the root AND walks from it, so anything
	// added anywhere in the tree can red whoever calls it.
	rootedWalk bool
	// walk: the body calls one of twdWalkers.
	walk bool
	// ascent: the body spells the climb from internal/posse to the repo
	// root by hand — `"..", ".."` joined, or a `"../.."` literal.
	ascent bool
	// caller: the body asks runtime.Caller where its own source lives,
	// which is how qibRepoRoot finds the root without depending on cwd.
	caller bool
	// handRolledTreeWalk: the body walks a directory whose path IS the repo
	// root reached by a hand-rolled climb — not one reached through
	// qibRepoRoot, and not a subdirectory of it.
	handRolledTreeWalk bool
	// shellRoot: the body asks GIT for this repo's root —
	// `exec.Command("git", "rev-parse", "--show-toplevel").Output()` — and
	// keeps the answer. A third spelling of the root, invisible to every
	// rule above, which is how five tree-wide pins sat undoored
	// (ranger-base-xndgk FINDING 5).
	shellRoot bool
}

// twdAsksGitForTheRoot reports whether a call is
// `exec.Command("git", ..., "--show-toplevel", ...).Output()` (or
// CombinedOutput) — git asked for this repo's root with the answer KEPT.
//
// The value being kept is the whole distinction. Four calls in
// silentrevert_qa_test.go ask git the same question and throw the answer
// away, as a probe for "is there a checkout here at all" — as does
// qibSkipUnlessCheckout, which asks --git-dir. A probe is not a second way
// to compute the root, and it is not fenced.
//
// RESIDUAL, said out loud: this matches the exec.Command spelling, which is
// what all four offending call sites wrote and what the next one would
// write. A root taken from the package's own git(dir, ...) helper with dir
// spelled "." is not matched. Nothing here can enumerate every way to name
// this repo — what bounds the class is that there is ONE helper, and the pin
// below fences the three spellings that have actually appeared: a twin
// helper, a hand-rolled climb, and this.
func twdAsksGitForTheRoot(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Output" && sel.Sel.Name != "CombinedOutput") {
		return false
	}
	inner, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	fn, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || (fn.Sel.Name != "Command" && fn.Sel.Name != "CommandContext") {
		return false
	}
	if x, ok := fn.X.(*ast.Ident); !ok || x.Name != "exec" {
		return false
	}
	for _, a := range inner.Args {
		lit, ok := a.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if v, err := strconv.Unquote(lit.Value); err == nil && v == "--show-toplevel" {
			return true
		}
	}
	return false
}

// twdRootExprs finds, inside one function body, every expression that
// evaluates to the repo root reached by hand: a bare `"../.."`, a
// `filepath.Join(..., "..", "..")`, those wrapped in Abs/Clean/EvalSymlinks,
// and the identifiers assigned from any of them. It returns a predicate over
// expressions.
//
// This is a small dataflow rather than "the body mentions `..` and walks
// something" on purpose, and the difference is nine tests: detectionRig
// climbs to the root and then ReadDirs `<root>/etc/herdr/agent-detection`,
// which reds when that one directory changes. Only a walk rooted AT the root
// is tree-wide.
func twdRootExprs(body *ast.BlockStmt) func(ast.Expr) bool {
	roots := map[string]bool{}
	var isRoot func(ast.Expr) bool
	isRoot = func(e ast.Expr) bool {
		switch v := e.(type) {
		case *ast.Ident:
			return roots[v.Name]
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				return false
			}
			s, err := strconv.Unquote(v.Value)
			if err != nil {
				return false
			}
			return filepath.ToSlash(s) == "../.." || filepath.ToSlash(s) == "../../"
		case *ast.CallExpr:
			sel, ok := v.Fun.(*ast.SelectorExpr)
			if !ok {
				return false
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok || x.Name != "filepath" {
				return false
			}
			switch sel.Sel.Name {
			case "Join":
				return twdJoinsTwoDotDots(v.Args)
			case "Abs", "Clean", "EvalSymlinks", "ToSlash":
				return len(v.Args) == 1 && isRoot(v.Args[0])
			}
		}
		return false
	}
	// Two passes, so `root := ...` before the walk and after both bind.
	for i := 0; i < 2; i++ {
		ast.Inspect(body, func(n ast.Node) bool {
			var lhs, rhs []ast.Expr
			switch st := n.(type) {
			case *ast.AssignStmt:
				lhs, rhs = st.Lhs, st.Rhs
			case *ast.ValueSpec:
				for _, id := range st.Names {
					lhs = append(lhs, id)
				}
				rhs = st.Values
			default:
				return true
			}
			// `x := expr` and `x, err := expr` both bind x.
			if len(rhs) != 1 || len(lhs) == 0 {
				if len(lhs) != len(rhs) {
					return true
				}
				for i, l := range lhs {
					if id, ok := l.(*ast.Ident); ok && isRoot(rhs[i]) {
						roots[id.Name] = true
					}
				}
				return true
			}
			if id, ok := lhs[0].(*ast.Ident); ok && isRoot(rhs[0]) {
				roots[id.Name] = true
			}
			return true
		})
	}
	return isRoot
}

// twdParse reads every function declared in internal/posse/*_test.go.
func twdParse(t *testing.T) (map[string]*twdFunc, int) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("internal", "posse", "*_test.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no test files found under internal/posse: %v", err)
	}
	fset := token.NewFileSet()
	out := map[string]*twdFunc{}
	tests := 0
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			f := &twdFunc{
				name:  fn.Name.Name,
				where: fmt.Sprintf("%s:%d", path, fset.Position(fn.Pos()).Line),
				test:  strings.HasPrefix(fn.Name.Name, "Test"),
				calls: map[string]bool{},
			}
			if f.test {
				tests++
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if v, err := strconv.Unquote(lit.Value); err == nil {
						switch filepath.ToSlash(v) {
						case "../..", "../../":
							f.ascent = true
						}
					}
				}
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if twdJoinsTwoDotDots(call.Args) {
					f.ascent = true
				}
				if twdAsksGitForTheRoot(call) {
					f.shellRoot = true
				}
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					f.calls[fun.Name] = true
					if fun.Name == twdRootHelper {
						f.root = true
					}
				case *ast.SelectorExpr:
					if twdWalkers[fun.Sel.Name] {
						f.walk = true
					}
					if x, ok := fun.X.(*ast.Ident); ok && x.Name == "runtime" && fun.Sel.Name == "Caller" {
						f.caller = true
					}
				}
				return true
			})
			f.rootedWalk = f.root && f.walk
			if !f.root {
				isRoot := twdRootExprs(fn.Body)
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok || len(call.Args) == 0 {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || !twdWalkers[sel.Sel.Name] {
						return true
					}
					if isRoot(call.Args[0]) {
						f.handRolledTreeWalk = true
					}
					return true
				})
			}
			out[f.name] = f
		}
	}
	return out, tests
}

// twdTreeWideTests returns the tests whose subject is the TREE. Two rules,
// unioned, and the union is deliberate — ranger-base-sx2dq is what a single
// rule cost:
//
//   - DIRECT: the test's own body calls qibRepoRoot, so it reads outside its
//     package. This was the whole rule, and it is kept because it is what
//     the five original members are doored on.
//   - WALK-REACHING: the test reaches, through this package's own test
//     helpers, a function that computes the repo root and WALKS from it.
//     TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover names no
//     root of its own — it calls coxnGoSources, which walks every shipped
//     .go file — and any file added anywhere can red it. The direct rule
//     never saw it.
//
// The reach is only propagated from a ROOTED WALK, not from any function
// that touches the root: qspSeedScript looks up one path at the root and its
// callers red only when that script changes, which is not this class.
//
// What keeps a third spelling from slipping past both rules is not a rule
// here at all — it is TestQAOneRepoRootHelperInTheTestPackage below, which
// holds internal/posse to ONE repo-root helper. That is the pin
// ranger-base-sx2dq was filed for: the tree carried a byte-identical twin of
// qibRepoRoot (qspRepoRoot) and a hand-rolled `..`/`..` ascent, and pins
// spelled either way were outside the class and got no door, silently.
func twdTreeWideTests(t *testing.T) (names []string, funcs int) {
	t.Helper()
	all, funcs := twdParse(t)

	// Seed the reach with the rooted walkers, then close it over callers.
	reaches := map[string]bool{}
	for name, f := range all {
		if f.rootedWalk {
			reaches[name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for name, f := range all {
			if reaches[name] {
				continue
			}
			for callee := range f.calls {
				if reaches[callee] {
					reaches[name] = true
					changed = true
					break
				}
			}
		}
	}

	for name, f := range all {
		if f.test && (f.root || reaches[name]) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, funcs
}

// Arm 2: every tree-wide pin has a door, and every door names a real one.
// Two-way, because both directions are the bug: a pin with no door is
// ranger-base-ik44f arriving again, and a door naming a test that no longer
// exists is a green `-run` filter that runs nothing.
func TestQAEveryTreeWidePinHasADoor(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	doored := map[string]string{}
	for _, v := range twdPinVars {
		for _, name := range twdVar(t, src, v) {
			if prev, dup := doored[name]; dup {
				t.Errorf("%s is named by both $(%s) and $(%s) — one pin, two doors, and the two can drift apart", name, prev, v)
			}
			doored[name] = v
		}
	}

	// QA_TOOL_PINS is the one membership this arm cannot check by wiring: a
	// tool door (`gofmt -l`) names no test, so a pin listed there is a CLAIM
	// that some tool reads it. Exactly one pin makes that claim today, and
	// its door is held open by gofmtdoor_qa_test.go. A second name arriving
	// here would be a pin declaring itself doored with nothing to show —
	// the cheapest way to silence the check below — so it fails until this
	// arm is taught what the new tool door is and who pins it.
	if tool := twdVar(t, src, "QA_TOOL_PINS"); len(tool) != 1 || tool[0] != "TestTreeIsGofmtClean" {
		t.Errorf("$(QA_TOOL_PINS) = %v, want exactly [TestTreeIsGofmtClean] — that variable records the pins whose door is a TOOL rather than a `-run` filter, which is a claim nothing here can verify. A new entry needs its own arm; a pin moved here to quiet this test has no door at all.", tool)
	}

	// QA_HISTORY_PINS is the second membership this arm cannot check by
	// wiring, for a different reason than QA_TOOL_PINS: its three pins read
	// `git log` in THIS repo, so arm 3 cannot plant drift for them in a
	// copied tree (a `git archive` scratch tree has no .git, and reds them
	// in every arm including the control). They are doored and run clean;
	// what is NOT proven for them is that their door can fail. So the
	// membership is named here, and a fourth pin moved into this variable to
	// dodge arm 3's drift plant fails until this arm is taught why.
	wantHistory := []string{
		"TestPublicationRootCommitOmitsExcludedPaths",
		"TestPublicationRootCommitADRsCarryProvenance",
		"TestPublicationHistoryNeverCarriesTheSeedScript",
		// The fourth, taught here rather than moved in quietly
		// (ranger-base-xndgk FINDING 5): it reads THIS repo's `git rev-list`
		// and `git show` for every version of every shipped example, which
		// is the same reason as the three above — a copied tree has no
		// history to plant drift in. It was undoored because its root came
		// from `git rev-parse --show-toplevel`, a spelling neither class
		// rule could see.
		"TestShippedExampleTableCoversEveryVersionInGitHistory",
	}
	if got := twdVar(t, src, "QA_HISTORY_PINS"); !twdSameSet(got, wantHistory) {
		t.Errorf("$(QA_HISTORY_PINS) = %v, want exactly %v — that variable records the tree-wide pins whose subject is this repo's git HISTORY, which is the one thing a copied tree does not have, so arm 3 runs them clean and plants no drift. A new entry needs its own arm; a pin moved here is a pin whose door nothing has shown can fail.", got, wantHistory)
	}

	// The same membership-by-name treatment for the two doors
	// ranger-base-xndgk FINDING 5 added, and for the same reason as
	// QA_HISTORY_PINS rather than a new one: all four pins READ THE TREE
	// THROUGH GIT — `git ls-files`, `git grep` — so in a copied tree they
	// find no checkout and skip, and arm 3 cannot plant drift for them
	// there. Their membership is therefore named here, so neither variable
	// can become the quiet place to park a pin whose door nothing has shown
	// can fail.
	//
	// These four were the finding: every one of them took its root from
	// `git rev-parse --show-toplevel`, which is stdout from a subprocess and
	// invisible to both class rules, so none of them was in any door
	// variable at all. TestQAIdentityLiteralsNeverAppearInATrackedPath
	// censuses EVERY TRACKED PATH in the repository.
	wantIdentity := []string{
		"TestQAIdentityLiteralsNeverAppearInATrackedPath",
		"TestIdentityLiteralsNeverAppearInTheHarnessRepoUndispositioned",
	}
	if got := twdVar(t, src, "QA_IDENTITY_PINS"); !twdSameSet(got, wantIdentity) {
		t.Errorf("$(QA_IDENTITY_PINS) = %v, want exactly %v — this box's identity literals asked of the tree twice, once of tracked PATHS and once of tracked CONTENT. Both read the tree with git, so arm 3 cannot plant drift for them in a copied tree; a new entry needs its own arm.", got, wantIdentity)
	}
	wantOps := []string{
		"TestQAEveryOpsHitInTrackedMarkdownIsRuled",
		"TestQAOpsShapeTableCanStillSayNo",
		// The instance path-form census and its control, taught here rather
		// than moved in quietly (ranger-base-l9ii). Same door because it is
		// the same question — ADR 0024 D1 ops residue in the public tree —
		// asked of every tracked FILE rather than of tracked markdown: this
		// deployment's constitution checkout written as a live path
		// (`~/src/…`, `$HOME/src/…`), dispositioned per file. And it belongs
		// in a named membership for the same reason the four above do: it
		// reads the tree through `git ls-files`, so in the `git archive`
		// scratch tree arm 3 works in it finds no checkout and skips, and
		// arm 3 cannot plant drift for it there. Its door is shown able to
		// fail by its own control instead, which drives the matcher on
		// planted lines.
		"TestInstancePathFormNeverAppearsInTrackedContentUndispositioned",
		"TestQAInstancePathCensusCanStillSayNo",
	}
	if got := twdVar(t, src, "QA_OPS_PINS"); !twdSameSet(got, wantOps) {
		t.Errorf("$(QA_OPS_PINS) = %v, want exactly %v — the ops-residue census over every tracked markdown file, the instance path-form census over every tracked file, and the control beside each that says it can still say no. Both censuses read the tree with git, so arm 3 cannot plant drift for them in a copied tree; a new entry needs its own arm.", got, wantOps)
	}

	names, funcs := twdTreeWideTests(t)
	// A pin over a derived set is satisfied by deriving nothing: say how
	// many test functions were actually parsed, and fail a walk that found
	// far fewer than internal/posse holds (1000+ on 2026-09-04).
	if funcs < 300 {
		t.Fatalf("only %d test functions parsed under internal/posse — the walk this arm derives the class from found nothing", funcs)
	}
	// Two floors, because the two rules fail apart. The union's floor
	// catches a derivation that collapsed; it is NOT what catches a member
	// going invisible one at a time — the old floor here was len < 5, which
	// fired only when qibRepoRoot was renamed and never when a pin was
	// simply spelled with the twin helper (ranger-base-sx2dq). That is
	// TestQAOneRepoRootHelperInTheTestPackage's job, below.
	if len(names) < 14 {
		t.Fatalf("only %d tree-wide tests found (18 on 2026-09-04) — %s has been renamed or wrapped, and this arm is now deriving a class that is missing members: %v", len(names), twdRootHelper, names)
	}
	// And the walk-reaching rule specifically: it is the half that catches a
	// pin reading the tree through a helper, and a rule that silently stops
	// matching leaves the union looking healthy on the direct half alone.
	if reach := twdWalkReachingTests(t); len(reach) < 1 {
		t.Errorf("no test reaches a rooted walk through a helper (at least TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover did on 2026-09-04) — half the class rule is matching nothing")
	}
	t.Logf("parsed %d test functions under internal/posse, %d of them tree-wide", funcs, len(names))

	found := map[string]bool{}
	for _, name := range names {
		found[name] = true
		if doored[name] == "" {
			t.Errorf("%s reads the repo root and no Makefile door names it — it is reachable only by a `-run` filter that happens to spell it, which is ranger-base-ik44f arriving again. Add it to $(QA_CREW_PINS) or a new door.", name)
		}
	}
	for name, v := range doored {
		if !found[name] {
			t.Errorf("$(%s) names %s, which is not a tree-wide test in internal/posse — the door's `-run` filter matches nothing there and passes in silence", v, name)
		}
	}
}

// twdWalkReachingTests is the walk-reaching half of the class on its own:
// tests that are members ONLY because they reach a rooted walk through a
// helper, with no qibRepoRoot call of their own.
func twdWalkReachingTests(t *testing.T) []string {
	t.Helper()
	all, _ := twdParse(t)
	reaches := map[string]bool{}
	for name, f := range all {
		if f.rootedWalk {
			reaches[name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for name, f := range all {
			if reaches[name] {
				continue
			}
			for callee := range f.calls {
				if reaches[callee] {
					reaches[name] = true
					changed = true
					break
				}
			}
		}
	}
	var out []string
	for name, f := range all {
		if f.test && reaches[name] && !f.root {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// The pin ranger-base-sx2dq was filed for. The class above is DERIVED, and a
// derivation is only as wide as the thing it keys on: internal/posse carried
// two byte-identical repo-root helpers, qibRepoRoot and qspRepoRoot, and a
// pin spelled with the second one was outside the class, got no door, and
// nothing said so. Folding the twin made the rule true again; this is what
// keeps a third copy from arriving and quietly undoing it.
//
// Two shapes, both measured against the tree rather than imagined:
//
//   - a TWIN HELPER: a function that asks runtime.Caller where it lives and
//     climbs two levels, which is qibRepoRoot's body written again under
//     another name. Whether or not it walks, its callers are invisible.
//   - a HIDDEN WALKER: a function that walks a tree AND spells the climb to
//     the repo root by hand (`filepath.Abs("../..")`,
//     `filepath.Join(dir, "..", "..")`). TestSeedConfigLiveKeysAreRead was
//     exactly this: a WalkDir over every non-test .go file in the repo,
//     reached by no helper at all, so no identifier match could ever find
//     it.
//   - a SHELLED ROOT: a body that asks git — `exec.Command("git",
//     "rev-parse", "--show-toplevel").Output()` — and keeps the answer.
//     ranger-base-xndgk FINDING 5: FOUR tree-wide pins were spelled this
//     way, one of them a census of EVERY TRACKED PATH in the repository, and
//     none of them was in any door variable. Both rules above key on Go's
//     own filesystem calls, so neither could ever have seen a root that
//     arrives as the stdout of a subprocess. A probe that throws the answer
//     away is not this shape and is not fenced.
//
// Reading ONE file at the repo root is not either shape and is not fenced —
// ~48 test files do it, they red only when that one file changes, and that
// is not this class.
func TestQAOneRepoRootHelperInTheTestPackage(t *testing.T) {
	t.Parallel()
	all, funcs := twdParse(t)
	if funcs < 300 {
		t.Fatalf("only %d test functions parsed under internal/posse — the walk this pin reads is finding nothing", funcs)
	}
	if _, ok := all[twdRootHelper]; !ok {
		t.Fatalf("internal/posse's tests no longer declare %s — the class in this file is derived from it, so every derived arm above is now deriving nothing", twdRootHelper)
	}

	var twins, hidden []string
	for name, f := range all {
		if name == twdRootHelper || f.root {
			continue
		}
		if f.caller && f.ascent {
			twins = append(twins, f.where+" "+name)
		}
		if f.handRolledTreeWalk {
			hidden = append(hidden, f.where+" "+name)
		}
	}
	sort.Strings(twins)
	sort.Strings(hidden)
	for _, w := range twins {
		t.Errorf("%s asks runtime.Caller and climbs two levels — that is a second %s under another name, and every pin spelled with it is outside the tree-wide class this file derives, so it gets no door and nothing says so. Call %s instead. (ranger-base-sx2dq: the tree carried exactly this, byte for byte.)", w, twdRootHelper, twdRootHelper)
	}
	for _, h := range hidden {
		t.Errorf("%s walks a tree from a hand-rolled climb to the repo root — a tree-wide pin no identifier match can reach, which is how TestSeedConfigLiveKeysAreRead went undoored. Take the root from %s.", h, twdRootHelper)
	}

	var shelled []string
	for name, f := range all {
		if f.shellRoot {
			shelled = append(shelled, f.where+" "+name)
		}
	}
	sort.Strings(shelled)
	for _, sh := range shelled {
		t.Errorf("%s asks git for this repo's root and keeps the answer — `exec.Command(\"git\", \"rev-parse\", \"--show-toplevel\").Output()` is a root no rule in this file can see, so a pin spelled with it is outside the tree-wide class and gets no door. Take the root from %s and, if the reading needs a checkout, skip with qibSkipUnlessCheckout. (ranger-base-xndgk FINDING 5: four pins were spelled this way, one of them a census of every tracked path in the repository.)", sh, twdRootHelper)
	}
}

// twdSourceFiles lists what to copy: git's idea of the working tree, which
// leaves out bin/ and dist/ and every other build output.
//
// It falls back to a walk when git cannot answer, and the fallback is not
// decoration: a `git archive | tar -x` scratch tree is the house rig for
// mutation runs on this repo, it has no .git, and four root/internal pins
// already red on that alone in every arm including the control. A fifth
// would be a tax on every future rig, paid to save ten lines here.
func twdSourceFiles(t *testing.T) []string {
	t.Helper()
	if out, err := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output(); err == nil {
		var files []string
		for _, rel := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
			if rel != "" {
				files = append(files, rel)
			}
		}
		return files
	}
	var files []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "dist", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// twdSeedTree copies the working tree into a scratch directory: the door's
// recipe compiles internal/posse from there, so qibRepoRoot — which resolves
// off the compiled source's own path — answers the scratch root and the pins
// walk the copy. The WORKING tree and not HEAD, so a pin renamed in one
// commit with its door does not fail this arm against a stale HEAD.
func twdSeedTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copied := 0
	for _, rel := range twdSourceFiles(t) {
		info, err := os.Lstat(rel)
		if err != nil || !info.Mode().IsRegular() {
			// A symlink (plugin/bin/posse points at the installed binary)
			// or a path git knows and the tree does not. Neither compiles
			// and neither is read by the pins.
			continue
		}
		body, err := os.ReadFile(rel)
		if err != nil {
			continue
		}
		dst := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, body, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
		copied++
	}
	if copied < 500 {
		t.Fatalf("only %d files copied into the scratch tree — the pins below have file-count floors and would fail on the rig rather than on the drift", copied)
	}
	return dir
}

// twdExpand returns `make -n`'s own expansion of a door's recipe, so nothing
// here is a second copy of the command to drift from it.
func twdExpand(t *testing.T, target string) string {
	t.Helper()
	out, err := exec.Command("make", makeExpandFlag, "-n", target).Output()
	if err != nil {
		t.Fatalf("make -n %s: %v", target, err)
	}
	recipe := strings.TrimSpace(string(out))
	if recipe == "" {
		t.Fatalf("`make -n %s` expands to nothing", target)
	}
	if strings.Contains(recipe, "$(") {
		t.Fatalf("the expanded recipe still holds a make variable — this arm would be measuring the wrong text:\n%s", recipe)
	}
	assertRecipeIsOnlyRecipe(t, target, recipe)
	return recipe
}

// Arm 3: the doors can fail, on the drift they exist for.
func TestQATheTreeWideDoorsReportRealDrift(t *testing.T) {
	t.Parallel()
	crew := twdExpand(t, "crew-check")
	seed := twdExpand(t, "seed-check")
	doc := twdExpand(t, "doc-check")
	dir := twdSeedTree(t)

	run := func(recipe string) (string, error) {
		cmd := exec.Command("sh", "-c", recipe)
		cmd.Dir = dir
		b, err := cmd.CombinedOutput()
		return string(b), err
	}

	// The clean arm first, both doors: a door that always fails detects
	// nothing, and a filter that matches nothing passes in silence.
	for _, door := range []struct{ name, recipe string }{{"crew-check", crew}, {"seed-check", seed}, {"doc-check", doc}} {
		got, err := run(door.recipe)
		if err != nil {
			t.Fatalf("`make %s` failed on a clean copy of this tree — it reports drift that is not there:\n%s", door.name, got)
		}
		if strings.Contains(got, "no tests to run") {
			t.Fatalf("`make %s`'s filter matched no test at all, so its green says nothing:\n%s", door.name, got)
		}
	}

	// history-check is the one door this arm cannot copy: its three pins read
	// `git log` in THIS repo, and the scratch tree has no .git — all three
	// red there in every arm including the control, so a copied run would
	// measure the copy and not the door. Run it where it lives instead. It
	// only reads (`git log`, `git rev-list`), and this proves the two halves
	// a filter can get wrong: the recipe runs, and it names real tests.
	{
		out, err := exec.Command("sh", "-c", twdExpand(t, "history-check")).CombinedOutput()
		if err != nil {
			t.Errorf("`make history-check` failed in this repo — the door reports drift that is not there:\n%s", out)
		}
		if strings.Contains(string(out), "no tests to run") {
			t.Errorf("`make history-check`'s filter matched no test at all, so its green says nothing:\n%s", out)
		}
	}

	// The crew door's drift: a crew name in a file the shipped tree walk
	// reads. Assembled, never spelled — this file is inside that walk's
	// repo-root pass, the same reason instancebound_qa_test.go assembles.
	name := "gw" + "art"
	probe := filepath.Join("internal", "twd_drift_probe.txt")
	if err := os.WriteFile(filepath.Join(dir, probe), []byte("seat: "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := run(crew)
	if err == nil {
		t.Fatalf("`make crew-check` passed a tree naming the originating instance's crew — the door reads nothing:\n%s", got)
	}
	for _, want := range []string{filepath.ToSlash(probe), name, "TestShippedTreeNamesRolesNotThisCrew"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make crew-check` failed without naming %q, so it says a seat is wrong and not where:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, probe)); err != nil {
		t.Fatal(err)
	}

	// The seed door's drift: the retired harness name on the published
	// surface. Assembled for the same reason the crew probe is — this file
	// is inside the walk that reads it.
	stale := "ranger" + "hq"
	seedProbe := filepath.Join("internal", "twd_seed_drift_probe.txt")
	if err := os.WriteFile(filepath.Join(dir, seedProbe), []byte("harness: "+stale+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = run(seed)
	if err == nil {
		t.Fatalf("`make seed-check` passed a tree carrying the retired harness name on the seed surface — the door reads nothing, or its members all SKIPPED, which is the same green:\n%s", got)
	}
	for _, want := range []string{filepath.ToSlash(seedProbe), "TestSeedSurfaceNameCountIsZero"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make seed-check` failed without naming %q, so it says the surface is dirty and not where:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, seedProbe)); err != nil {
		t.Fatal(err)
	}

	// The doc door's drift: the ADR 0019 framing that was retired, back in a
	// shipped .go file — the exact way it walked in before, as a comment
	// nobody's test looks at. Assembled, so this file is not itself a hit
	// when a wider scanner than coxnGoSources is pointed at the tree.
	dead := "stale " + "leftover"
	docProbe := filepath.Join("internal", "twd_doc_drift_probe.go")
	body := "package internal\n\n// the on-disk credential file is a " + dead + " of a keychain login\n"
	if err := os.WriteFile(filepath.Join(dir, docProbe), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = run(doc)
	if err == nil {
		t.Fatalf("`make doc-check` passed a shipped source carrying the framing ADR 0019's amendment retired — the door reads nothing:\n%s", got)
	}
	for _, want := range []string{filepath.ToSlash(docProbe), dead, "TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make doc-check` failed without naming %q:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, docProbe)); err != nil {
		t.Fatal(err)
	}
}

// twdSpelled spells a non-negative integer the way this file's head comment
// does. The comment is prose and says "twenty pins", not "20 pins", so the
// pin below has to compare against a WORD; the alternative — rewriting the
// sentence in digits so a test can read it — would let the test choose how
// the file reads, which is backwards.
func twdSpelled(n int) string {
	ones := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
		"eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens := []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	switch {
	case n < 0 || n > 99:
		return strconv.Itoa(n) // out of the prose range; the pin will name the mismatch
	case n < 20:
		return ones[n]
	case n%10 == 0:
		return tens[n/10]
	default:
		return tens[n/10] + "-" + ones[n%10]
	}
}

// twdHead returns this file's own head comment as flowed prose — `//` markers
// stripped, lines joined with a space. Flowed, not per-line, because the
// claim it holds WRAPS (today "... over three runs" then "at twenty pins and
// seven doors", and the break moves whenever the sentence is rewrapped), and
// a per-line scan is blind to exactly that.
func twdHead(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("treewidedoor_qa_test.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "package posse" || line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			break // the head comment is over; `import (` is the first line here
		}
		out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "//")))
	}
	if len(out) == 0 {
		t.Fatal("this file has no head comment — the sentence arm 4 holds is gone, and with it the number a seat prices `make tree-check` from")
	}
	return strings.Join(out, " ")
}

// twdCountClaim matches the ONE sentence in the head comment that says how
// big the class is. Deliberately narrow: the comment is full of correctly
// frozen history ("five pins took their root from ...", "five members became
// thirteen"), and a rule that read those as live claims would red on prose
// that is true.
var twdCountClaim = regexp.MustCompile(`([a-z]+(?:-[a-z]+)?) pins and ([a-z]+(?:-[a-z]+)?) doors`)

// Arm 4: the head comment's count is the Makefile's count (ranger-base-4jogv).
//
// WHY THIS IS A PIN AND NOT A ONE-TIME CORRECTION. Both numerals were wrong
// at main, and they went wrong by two different routes, neither of which any
// existing arm can see:
//
//   - the DOORS number entered wrong (d189b623 wrote "seven doors" over an
//     enumeration of eight) and was then faithfully decremented to six when
//     the selector door and its pin were removed — a correct edit applied to
//     a wrong number stays wrong;
//   - the PINS number drifted with no edit at all, because a bead that adds
//     a name to $(QA_DOC_PINS) has no reason to read this file's prose.
//
// Arm 2 is two-way and mechanical, so no pin is undoored and no door is
// empty — the MECHANISM was never wrong. What was wrong is the sentence a
// seat reads to decide whether `make tree-check` is worth typing, in the one
// file whose entire subject is "the doors are wide enough". So the count is
// derived here, and the enumeration it rests on is held one-way: every name
// in a door variable must be named up there. One-way on purpose — the head
// also names tests that are NOT members (TestQAOneRepoRootHelperInTheTestPackage
// is the fence, not a pin), and a two-way rule would red on those.
func TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// The pins, deduped across door variables: arm 2 already fails a name
	// carried by two doors, so counting the union rather than the sum keeps
	// this arm from reporting a second, derived symptom of that one bug.
	seen := map[string]bool{}
	var pins []string
	for _, v := range twdPinVars {
		for _, name := range twdVar(t, src, v) {
			if seen[name] {
				continue
			}
			seen[name] = true
			pins = append(pins, name)
		}
	}
	doors := twdPrereqs(t, src, "tree-check")

	head := twdHead(t)

	// One live claim, and it is the derived one. Two matches means a second
	// sentence started counting and the two can now disagree; zero means the
	// claim moved out from under the pin that guards it.
	claims := twdCountClaim.FindAllStringSubmatch(head, -1)
	if len(claims) != 1 {
		t.Fatalf("the head comment carries %d `<n> pins and <n> doors` claims, want exactly 1 — the count of this class is said in one place on purpose, because two places drift apart and the reader cannot tell which is stale: %v", len(claims), claims)
	}
	gotPins, gotDoors := claims[0][1], claims[0][2]
	wantPins, wantDoors := twdSpelled(len(pins)), twdSpelled(len(doors))
	if gotPins != wantPins || gotDoors != wantDoors {
		t.Errorf("the head comment says %q pins and %q doors; the Makefile has %s (%d) and %s (%d).\n"+
			"  door variables: %v\n"+
			"  tree-check prerequisites: %v\n"+
			"A seat prices `make tree-check` from that sentence. Fix the sentence, not this test — and if a pin or a door really did come or go, the enumeration above the sentence needs the same edit.",
			gotPins, gotDoors, wantPins, len(pins), wantDoors, len(doors), twdPinVars, doors)
	}

	// And the enumeration the count rests on. Without this, the count stays
	// green while the list under it goes short — which is how the pins
	// number drifted in the first place: three tests were doored by beads
	// that never touched this comment.
	for _, name := range pins {
		if !strings.Contains(head, name) {
			t.Errorf("$(%s) door variable names %s, and this file's head comment does not — the enumeration the count sentence rests on is short by at least one, so the next reader counts a smaller class than `make tree-check` runs", twdVarOf(t, src, name), name)
		}
	}
}

// twdVarOf names the door variable that carries a test, for the message above.
func twdVarOf(t *testing.T, makefile, name string) string {
	t.Helper()
	for _, v := range twdPinVars {
		for _, n := range twdVar(t, makefile, v) {
			if n == name {
				return v
			}
		}
	}
	return "unknown"
}
