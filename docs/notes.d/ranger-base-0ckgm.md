# A walk whose root is not spelled is a pin with no door: the census of every root spelling in internal/treepins (ranger-base-0ckgm)

2026-10-31. `TestQANoTestShellsRealBdWithoutStoppingItsDaemon`
(internal/treepins/bddaemonleak_qa_test.go, ranger-base-42mv) walks the repo
root and reads every `_test.go` in the tree. It is squarely in
ranger-base-rulbl's class — a QA test whose subject is the TREE, living inside
a package nobody runs whole — and for the two months it existed it had no
`make` door, so its only route was an unfiltered 589.965s run of
internal/treepins.

Nothing was broken. The register that exists to catch exactly this
(`TestQAEveryTreeWidePinHasADoor`, treewidedoor_qa_test.go, arm 2) never asked
for a door, because the internal/treepins half of the class is derived from an
enumeration **rooted at a relative literal** (`twdRelTreePath` over
`twdWalkers`), and the scan was spelled

```go
func scanRealBdCallSites(root string) (...) { filepath.Walk(root, …) }
// …and, in the test body:
sites, files, err := scanRealBdCallSites(".")
```

The one literal sat one hop from where the derivation looks. Every signal was
green: the pin passed, the register passed, the doors passed.

## The fix

The scan takes no root at all now: `scanRealBdCallSites()` spells `"."` at the
`filepath.Walk`, the fixture arms call `scanBdCallSitesUnder(root)` (a
parameter, correctly — nothing hands it a tree path), and both share one walk
function so there is still one rule. The pin is derived, and
`$(QA_CORPUS_PINS)` doors it — its subject is this repo's own test corpus,
which is `make corpus-check`'s subject. Its five control arms are
deliberately NOT doored: they drive the rule over planted fixtures, so nothing
derives them and no unrelated bead can red them, and arm 2 is two-way.

## MEASURED 2026-10-31

This worktree, go 1.26.10, darwin 25.6.0, over `internal/posse/*_test.go` and
`internal/treepins/*_test.go` with the program at the foot of this fragment.

**The parameter shape is rare.** 25 functions across the two packages root a
walk at one of their own parameters, and exactly **3** call sites hand one of
them a path inside the tree:

| call site | shape |
|---|---|
| `scanRealBdCallSites(".")`, bddaemonleak_qa_test.go | this bead; fixed |
| `assertNoHintVocabulary(t, ".")`, internal/posse/herdrhintsgone_qa_test.go | a census of internal/posse's own non-test `.go` files |
| `scanReExecSites(".")`, internal/posse/reexecenv_qa_test.go | a census of internal/posse's own `_test.go` files |

The two in internal/posse root at the PACKAGE directory — that package's tests
run with their own package as cwd and climb to the repo root through the one
helper `twdRootHelper` names — so a relative literal there is not a tree path
and that half of the class is derived from the climb, not from the root. They
are a different question (is a census of one package's own sources in the
class?) and are filed, not decided here.

Because the shape is rare, it can be held by a RULE rather than a register:
`TestQANoTreeWalkingHelperIsHandedATreePathThroughAParameter`
(internal/treepins/rootparam_qa_test.go) fails any call site that hands a
param-rooted walker a tree path — directly, or through a local assigned from
that parameter (`p := filepath.Join(root, d); filepath.Walk(p, …)`, which is
how both waived helpers below spell it). Intraprocedural only: everything it
reads is in the body in front of it, which is the whole difference from the
interprocedural propagation the register refused.

**The parameter is one of SIX spellings, and the other five are live.** Of
the 40 walker calls in internal/treepins/*_test.go (after this bead's change),
23 root at a literal the derivation can read and **17 do not**:

| spelling | example | walker calls |
|---|---|---|
| a parameter | `scanBdCallSitesUnder(root)`, `hfSnapshot(dir)`, `lcsSources(dir)`, `wrsRenderSites(dir)` | 4 |
| a range variable over a list of literals | `for _, root := range []string{"cmd", "internal"}` (bdfScanGo, keychain, rhqLeftoverCorpus, suitetimeout), `filepath.Join(root, d)` over `arRoots` / `cgScannedDirs` (arParse, cgScan) | 6 |
| a root parsed out of the Makefile | `roots := fmtRoots(t, string(b))` (gofmtdoor — doored anyway, by a second walk in the same function that IS literal-rooted) | 2 |
| a local assigned a literal one line up | `dir := filepath.Join("docs", "adr"); os.ReadDir(dir)` (adrSweepCorpus) | 1 |
| a named constant | `os.ReadDir(armPkgDir)`, `armPkgDir = "internal/posse"` (armFiles) | 1 |
| a helper's return | `root, err := repoRoot()` (root_test.go) | 1 |
| a scratch root, correctly invisible | `releaseFixture(t, version)` (bottle), `filepath.Join(t.TempDir(), …)` (outguard) | 2 |

**Seventeen tests in internal/treepins read the tree through one of those and
are neither derived nor doored** — 30 are derived today, and 28 underived
tests reach a walker at all, of which these are the ones whose root is a tree
path rather than a fixture. Each reads files an unrelated bead can add, which
is the whole definition of the class:

- `TestADRRecordsNameTheLivePackageDirectory`,
  `TestADRSweptRecordsStillCiteTheLiveDirectory` — every record in `docs/adr`
  (adrSweepCorpus)
- `TestOneCredentialAcquirerAndNoOther`, `TestNoNewShadowPredicate`,
  `TestNoBdCloseVerbReachableFromDispatch`, `TestPaneModeReadingDecidesNothing`
  — every non-test `.go` file under `internal` and `cmd` (arParse)
- `TestNoShippedScriptAcquiresACredential` — every runnable file under
  `scripts`, `plugin`, `etc`, `docs` (cgScan); and
  `TestKeychainReadIsExecdFromExactlyOneFile` — every non-test `.go` file
  under `internal` and `cmd`, walked in the test body
- `TestEveryRenderedDoNotEditSiteNamesItsRenderer`,
  `TestWallRenderSourcesNameEveryRenderedDoNotEditSite` — every render site
  (wrsRenderSites)
- `TestLaunchCheckSourcesNameEveryParityDeciderAndRuntimeDeclarer`,
  `TestLaunchCheckSourcesHoldNothingTheDerivationCannotSee` — internal/posse's
  own sources through `repoRoot()` (lcsSources)
- `TestQAEveryPosseTestFileIsSharedOrInANamedArm`,
  `TestQAEveryMakefileDoorPinIsReachableInTheDefaultArm`,
  `TestQAEverySuiteArmTypeChecks` — every test file in internal/posse
  (armFiles, through the `armPkgDir` constant)
- `TestQANoEntryPointRunsGoTestOnTheDefaultTimeout` — every `scripts/*.sh` and
  `.github/workflows/*.yml`
- `TestTreePinsRunFromModuleRoot` — the module root's own listing; the
  marginal one of the seventeen, since its subject is the cwd rather than the
  files, but a root-level `_test.go` added by anyone reds it

(The other 11 of the 28 are fixture arms — the `…CatchesEachShape` family,
`TestCredentialRegisterIsLoudBothWays`, the two release arms, the hook
freshness arm and this bead's own three — which walk an `arFixture` or a
`t.TempDir` and owe no door.)

The honest answer there is not more spellings pinned one at a time: it is to
widen the derivation intraprocedurally, which this package already does for
the OTHER half of the class (`twdRootExprs` tracks locals assigned a root
expression in internal/posse), and then to door what that newly derives. That
is a bead of its own — ranger-base-8wxbn — because every one of those pins
needs a door CHOSEN — and the two helpers whose call sites this bead's new rule does
see are recorded in `rpWaived` with the reason, so the hole is a number the
pin prints rather than a silence.

## The census program

Kept here rather than in `scripts/`: it is a measurement, not a gate — the
gate is the pin. Run it from the repo root with
`go run /tmp/rootcensus.go` after writing it to a scratch path (it takes the
package as its only edit).

```go
package main

// Every walker call in one package's *_test.go, with the source text of its
// root and whether twdRelTreePath can read it.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var walkers = map[string]bool{"WalkDir": true, "Walk": true, "ReadDir": true, "Glob": true}

// relTreePath is twdRelTreePath, verbatim in rule: a string literal, or a
// filepath.Join of literals whose first element is one.
func relTreePath(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		str, err := strconv.Unquote(v.Value)
		if err != nil || str == "" {
			return "", false
		}
		slash := filepath.ToSlash(str)
		if filepath.IsAbs(str) || slash == ".." || strings.HasPrefix(slash, "../") {
			return "", false
		}
		return slash, true
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "filepath" {
			return "", false
		}
		switch sel.Sel.Name {
		case "Clean", "ToSlash", "FromSlash":
			if len(v.Args) == 1 {
				return relTreePath(v.Args[0])
			}
		case "Join":
			if len(v.Args) == 0 {
				return "", false
			}
			first, ok := relTreePath(v.Args[0])
			if !ok {
				return "", false
			}
			for _, a := range v.Args[1:] {
				if lit, ok := a.(*ast.BasicLit); !ok || lit.Kind != token.STRING {
					return "", false
				}
			}
			return first, true
		}
	}
	return "", false
}

type row struct {
	where, fn, walk, root string
	lit                   bool
}

func main() {
	const pkg = "internal/treepins" // the only edit
	fset := token.NewFileSet()
	paths, _ := filepath.Glob(filepath.Join(pkg, "*_test.go"))
	var rows []row
	for _, p := range paths {
		file, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			fmt.Println("parse", p, err)
			continue
		}
		for _, d := range file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !walkers[sel.Sel.Name] {
					return true
				}
				var b bytes.Buffer
				printer.Fprint(&b, fset, call.Args[0])
				_, lit := relTreePath(call.Args[0])
				rows = append(rows, row{
					where: fmt.Sprintf("%s:%d", filepath.Base(p), fset.Position(call.Pos()).Line),
					fn:    fd.Name.Name, walk: sel.Sel.Name, root: b.String(), lit: lit,
				})
				return true
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].where < rows[j].where })
	nlit := 0
	fmt.Println("--- roots the rule CANNOT read:")
	for _, r := range rows {
		if r.lit {
			nlit++
			continue
		}
		fmt.Printf("  %-44s %-38s %s(%s)\n", r.where, r.fn, r.walk, r.root)
	}
	fmt.Printf("=== %d walker calls: %d literal-rooted, %d not\n", len(rows), nlit, len(rows)-nlit)
}
```

The param-handoff half of the census is not kept here, because it is no longer
a measurement: `rpScan` in internal/treepins/rootparam_qa_test.go is that
census, run on every suite.
