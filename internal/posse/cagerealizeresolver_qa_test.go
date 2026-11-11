//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-dfgte — ADR 0086 D2's "ONE RESOLVER", as a sweep
// rather than as a sentence.
//
// WHAT THE DECISION ASKS FOR, verbatim: "`RealizeCage(demand)` becomes
// `RealizeCageFor(ag, rt, demand)`. Every door that reads the map calls it:
// CreateSession's launch path (herdrback.go), both halves of `CheckParity`
// (the demand and the relaunch's by-PID reading), and dispatch's `CheckTier`.
// If the launch and parity disagree, the session has a wall its record does
// not name. That is the defect to design out, so the build pins that every
// door reaches the one function."
//
// AND WHY PROSE WAS NOT ENOUGH. The defect this guards is SILENT in both
// directions and in a way no test of one door can see. A second reader of
// `cage_realize_*` would compile, pass every table in cagerealize_test.go
// (they drive the resolver directly), and produce a session that is walled at
// the tier one door resolved and recorded at the tier another did — and the
// surface an operator reads, `posse gates`, is parity's answer, which is the
// one that would be right. The wrong answer is the wall.
//
// So this file sweeps the package's own sources for the three shapes that put
// a second resolver in the tree:
//
//  1. a caller of the map's raw half, `realizeCageIn`. It is unexported for
//     exactly this reason and D2's decline is not in it, so a caller there
//     resolves the map with the decline left out.
//  2. a caller that takes `CageRealizeEntries`' ROWS outside the file that
//     owns the key. That function is the validator every door refuses on, and
//     it is reached from CheckTier and from the standing reading — but its
//     rows are the map itself, and applying them by hand is a resolver
//     spelled in a loop.
//  3. a caller of `checkParityAtRealizedTier`, the map-free parity variant
//     the decline test needs. A door that reached that instead of CheckParity
//     would read the matrix for a tier this host may never run the PID at —
//     and would not loop, so nothing else would say so.
//
// Each arm carries a FLOOR: the legitimate spelling it exempts has to be
// found, or the sweep is measuring an absence and the exemption is granted to
// nobody (ranger-base-qg6q5's ladder, the same floor
// TestQANoLaunchPathSpellsTheSeatbeltWallPredicateItself carries).

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// crrOwner is the file that owns the key family: the one place the raw map
// reading and the entry rows are legitimately handled.
const crrOwner = "cagerealize.go"

// crrResolver is the one resolver every door must reach.
const crrResolver = "RealizeCageFor"

// crrCallName is the called function's own name, whether the call is spelled
// bare (`realizeCageIn(...)`) or through a receiver or package
// (`a.RealizeCageFor(...)`). Scope-free for the reason cdmWallLookup is: a
// second spelling is free to pick either form, and the name is what both have
// in common.
func crrCallName(n ast.Node) string {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return ""
	}
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		return f.Sel.Name
	case *ast.Ident:
		return f.Name
	}
	return ""
}

// crrEnclosing walks d once and reports, for every call to one of want, the
// name of the function declaration it sits in and the line it is on.
type crrHit struct {
	file, in string
	line     int
	name     string
}

func crrCalls(t *testing.T, want map[string]bool) []crrHit {
	t.Helper()
	fset := token.NewFileSet()
	var out []crrHit
	for _, src := range cdmPackageSources(t) {
		f, err := parser.ParseFile(fset, src, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			in := cdmDeclName(d)
			ast.Inspect(d, func(n ast.Node) bool {
				if name := crrCallName(n); want[name] {
					out = append(out, crrHit{file: src, in: in, line: fset.Position(n.Pos()).Line, name: name})
				}
				return true
			})
		}
	}
	return out
}

// where is the `<file>:<line>` a failure prints. NOT `at`, which is the name
// of a package-level helper elsewhere in this test package: cmd/testparallel
// resolves a call by NAME, so a method spelled `at` made these pins read as
// readers of that helper's package var and both lost their `t.Parallel`
// clearance (`make corpus-check`, 2026-11-11).
func (h crrHit) where() string { return fmt.Sprintf("%s:%d", h.file, h.line) }

// ARM 1 — the raw map reading has exactly one caller, and it is the resolver.
//
// `realizeCageIn` is ADR 0083 D7's resolution and nothing more: the config,
// the OS, the provenance and the demand. ADR 0086 D2's decline needs the PID
// and the runtime, which that signature does not take — so a door that
// resolved the map there would get a tier with the decline silently left out,
// which is a launch at one tier under a record naming another.
func TestQAOneCageRealizeResolverAndEveryDoorReachesIt(t *testing.T) {
	t.Parallel()
	raw := crrCalls(t, map[string]bool{"realizeCageIn": true})
	if len(raw) == 0 {
		t.Fatalf("nothing in this package calls realizeCageIn — the map's raw reading has moved or been renamed, so this sweep is measuring an absence that means nothing (ADR 0086 D2)")
	}
	for _, h := range raw {
		if h.file == crrOwner && h.in == "func "+crrResolver {
			continue
		}
		t.Errorf("%s calls %s inside %s — that is ADR 0083 D7's map AND NOT ADR 0086 D2's decline, so this reading resolves a tier the PID may never be launched at. Call %s.%s(ag, rt, demand) instead: it is the one resolver, and the reason it is one is that a launch and a parity reading that disagree leave a session walled at a tier its own record does not name (ADR 0086 D2)",
			h.where(), h.name, h.in, crrOwner, crrResolver)
	}
}

// ARM 2 — outside the file that owns the key, `CageRealizeEntries` is a
// VALIDATOR and its rows are not read.
//
// It is the door every reader refuses on — the launch, dispatch's CheckTier,
// `posse status` — and that use takes the error and discards the rows. The
// rows themselves are the map, so a caller that bound them outside this file
// would be resolving a demand by hand, in a loop, free to disagree with the
// resolver about an identity row, a demand the map does not name, or D2's
// decline. Inside cagerealize.go the rows are the subject (the resolution and
// the standing reading both walk them), which is what the exemption is for.
func TestQACageRealizeEntryRowsAreReadOnlyWhereTheKeyLives(t *testing.T) {
	t.Parallel()
	const fn = "CageRealizeEntries"
	fset := token.NewFileSet()
	owner, validator := 0, 0
	for _, src := range cdmPackageSources(t) {
		f, err := parser.ParseFile(fset, src, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		// A call whose rows are discarded: the FIRST result is the blank
		// identifier. Collected before the sweep for the AssignStmt reason
		// cdmWallLookup's caller gives — the statement is visited before its
		// own Rhs, so a discarded call is already marked when it is judged.
		discarded := map[ast.Node]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Rhs) != 1 || len(as.Lhs) == 0 {
				return true
			}
			if id, ok := as.Lhs[0].(*ast.Ident); !ok || id.Name != "_" {
				return true
			}
			if crrCallName(as.Rhs[0]) == fn {
				discarded[as.Rhs[0]] = true
			}
			return true
		})
		for _, d := range f.Decls {
			in := cdmDeclName(d)
			ast.Inspect(d, func(n ast.Node) bool {
				if crrCallName(n) != fn {
					return true
				}
				at := fmt.Sprintf("%s:%d", src, fset.Position(n.Pos()).Line)
				switch {
				case src == crrOwner:
					owner++
				case discarded[n]:
					validator++
				default:
					t.Errorf("%s binds %s's ROWS inside %s — outside %s that function is the validator a door refuses on, and its rows are the map itself. Applying them by hand is a second resolver spelled in a loop, free to disagree with %s about an identity row, a demand the map does not name, or ADR 0086 D2's decline. Take the error and discard the rows, then ask %s for the tier",
						at, fn, in, crrOwner, crrResolver, crrResolver)
				}
				return true
			})
		}
	}
	if owner == 0 {
		t.Fatalf("no call to %s in %s — the key's own reader has moved, so the exemption above is granted to nobody and this sweep measures an absence", fn, crrOwner)
	}
	if validator == 0 {
		t.Fatalf("no door outside %s validates the map with %s — that reading is what makes a broken map a refusal at the launch and at dispatch rather than a silent \"no mapping\" (ADR 0083 D7, ADR 0080 D9), so its absence is the finding and not a clean sweep", crrOwner, fn)
	}
}

// ARM 3 — the map-free parity variant is reached only from the decline test.
//
// `checkParityAtRealizedTier` exists so D2's decline can read parity at a
// tier WITHOUT re-entering the map, which would loop. The loop is the reason
// it exists and the reason it is dangerous: a door that reached it for a
// DEMAND would get the matrix for a tier this host may never launch the PID
// at — the same disagreement D2 names, and this one would not loop, so
// nothing at all would say so.
func TestQATheMapFreeParityVariantIsTheDeclineTestsAlone(t *testing.T) {
	t.Parallel()
	const fn = "checkParityAtRealizedTier"
	const caller = "func cageRulesLostByMapping"
	hits := crrCalls(t, map[string]bool{fn: true})
	if len(hits) == 0 {
		t.Fatalf("nothing in this package calls %s — ADR 0086 D2's decline test has moved or is spelled another way, so this sweep is measuring an absence (and a decline test that resolved the map would loop)", fn)
	}
	for _, h := range hits {
		if h.file == crrOwner && h.in == caller {
			continue
		}
		t.Errorf("%s calls %s inside %s — that variant reads NO `cage_realize_*` key, so it answers for a tier this host may never launch this PID at. Every other reader wants CheckParity, which resolves the map through %s; this one is %s's, where re-entering the map would loop (ADR 0086 D2)",
			h.where(), fn, h.in, crrResolver, caller)
	}
}

// ARM 4 — and the doors are THERE. The three arms above say nothing about a
// door that stopped reading the map at all: a launch that dropped its
// resolution would wall the session at the demanded tier while parity
// recorded the mapped one, which is the same defect from the other side and
// is just as silent.
//
// The floor is per DOOR and not a count, because what rots here is one door
// leaving and the count is as easily restored by a new caller somewhere else
// (ranger-base-o05yg's lesson, one file over). CheckTier is named by the
// function it reaches the resolver THROUGH, which is the honest statement of
// how dispatch's door is wired: it calls CheckParity, and CheckParity
// resolves.
func TestQAEveryCageMapDoorReachesTheOneResolver(t *testing.T) {
	t.Parallel()
	byFile := map[string][]string{}
	for _, h := range crrCalls(t, map[string]bool{crrResolver: true}) {
		byFile[h.file] = append(byFile[h.file], h.in)
	}
	// The launch door (ADR 0086 D2: "CreateSession's launch path").
	if len(byFile["herdrback.go"]) == 0 {
		t.Errorf("no door in herdrback.go calls %s — the LAUNCH no longer resolves the cage map, so the session is walled at the tier it demanded and recorded at whatever parity resolves. ADR 0086 D2 names this door by file", crrResolver)
	}
	// Both halves of CheckParity: the demand, and the relaunch's by-PID
	// reading. Two calls and not one — the second is how a mapped session's
	// waiver survives a refresh (ADR 0083 D7, ranger-base-pl3ck).
	if n := len(byFile["parity.go"]); n < 2 {
		t.Errorf("parity.go calls %s %d time(s); ADR 0086 D2 names BOTH halves of CheckParity — the demand, and the relaunch's by-PID reading, which is what keeps a mapped session's waiver from expiring at the first refresh", crrResolver, n)
	}
	// Dispatch's CheckTier, through the function it reaches the resolver by.
	// Read from the source rather than asserted in prose: a CheckTier that
	// stopped calling CheckParity would stop reading the map, and dispatch
	// would launch a bead at a tier nobody resolved.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "parity.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	reaches := false
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "CheckTier" {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			if crrCallName(n) == "CheckParity" {
				reaches = true
			}
			return true
		})
	}
	if !reaches {
		t.Errorf("CheckTier (parity.go) does not reach CheckParity, which is how dispatch's door reads the cage map (ADR 0086 D2). Either it resolves the map itself now — in which case %s is the call, and the arms above should have caught it — or dispatch launches a bead at a tier no resolver answered for", crrResolver)
	}
	// And the floor under all four: the resolver is called at all.
	if len(byFile) == 0 {
		t.Fatalf("nothing in this package calls %s — the one resolver has been renamed or removed, so every arm above is measuring an absence", crrResolver)
	}
}
