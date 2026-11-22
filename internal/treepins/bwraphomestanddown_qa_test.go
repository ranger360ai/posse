package treepins

// QA pins for ranger-base-jhmm7 — the door under the bwrap fixture HOME.
//
// THE DEFECT. BwrapArgs judges every write grant against the per-seat wall
// root composed from $HOME (bwrapRefuseWallRootGrant, ADR 0100 D1/D2(a)) and
// REFUSES a grant that contains it. A blanket temp grant contains any fixture
// home cut under that temp root, so a pin whose home is a plain
// `t.TempDir()` and whose subject is a render that must SUCCEED is asking the
// render to refuse itself. The refusal stands down only where the home is
// under `/tmp` (bwrapWallRoot), and that is a property of the HOST and not of
// the test: on linux `t.TempDir()` IS under /tmp and the stand-down is free;
// on darwin it is $TMPDIR under `/var/folders/…`, granted just as
// unconditionally but not `/tmp`. So the red is invisible on this box and
// lands on macos arm 1, in a sentence that names a refusal instead of the
// wall the arm came to measure.
//
// WHY THIS FILE EXISTS AND NOT A FOURTH FIX. Three fixtures in two days, each
// found by reading a macos CI log and nothing else:
//
//	bwrapstatedirs_qa_test.go   bwsHome              ranger-base-zbmxc
//	beadshomespelling_qa_test   via wgFixtureAnyBox  ranger-base-1aayx
//	storerepogitwall_qa_test    via wgFixtureAnyBox  ranger-base-1aayx
//
// Both fixes were the same answer — cut the home under `bwrapTmpMount` — and
// nothing held the rule, so the fourth fixture to drive BwrapArgs from a
// `t.TempDir()` home would have been green here and red there with no door in
// between. The deliverable is the door, not a fix.
//
// THE RULE IS STATED OVER THE FIXTURE ROOT, which is where the question is
// already answered. The alternative — a runtime assertion inside BwrapArgs'
// own test path — is not available: that refusal is a PRODUCTION rule and a
// test must be able to measure it (wallseatwall_qa_test.go, keepleaf_qa_test
// .go and wallcarvepins_qa_test.go do exactly that), so a reader has to tell
// "renders and must succeed" from "refuses and must say so", and the fixture
// root is the one thing in the source that says which. THREE roots say it,
// and a render's home has to come from one of them:
//
//   - wallStandDownDir (wall_helpers_test.go) — `os.MkdirTemp(bwrapTmpMount,
//     …)`, resolved. The home is inside the one blanket grant both hosts
//     spell the same way, so the render's stand-downs cover it and the
//     render SUCCEEDS. This is the answer for a pin whose subject is a wall.
//   - wallUngrantedDir (wall_helpers_test.go) — a root outside every blanket
//     temp grant. The wall root is then real and a grant over it is REFUSED,
//     which is the subject of the pins that drive it. Its own arms skip when
//     `bwrapWallRoot()` is empty, so it cannot quietly become the other case.
//   - sbRoot (seatbeltconstitution_qa_test.go) — the premise reached the
//     third way: HOME stays where it is and the blanket grant MOVES, TMPDIR
//     re-pointed at a sibling of the root and seatbeltTempGrants stubbed to
//     nil where the box cannot place the root outside them. No grant then
//     contains the home, so the render succeeds for a reason the host cannot
//     take away (ranger-base-u8xk4 is why it is written that way).
//
// A home from any of the three is DECLARED and this pin says nothing about
// it. A home from `t.TempDir()`, `gitTempDir(t)`, `wtqaHome(t)` or a bare
// `os.MkdirTemp` under anything but `/tmp` is the violation: it is green on
// linux by an accident of where the box's temp root is, and it is the red
// three macos legs carried. A FOURTH declared root is one row in
// bhsDeclaredRoots and nothing else — there is no exemption list anywhere
// here, because an exemption would only ever be used to re-open the class.
//
// MEASURED, and why the vocabulary is three and not one: with
// wallUngrantedDir and sbRoot left out, the scan read 26 functions where 3
// were real — 11 of the 23 false reads were wallUngrantedDir fixtures whose
// subject IS the refusal, and converting any of them would have retired the
// pin it is (2026-11-22, linux 7.2.15, go1.27.3, at 4f309bcb;
// docs/notes.d/ranger-base-jhmm7.md has every reading in this comment).
//
// WHAT THE SCAN READS, and the two properties it pairs:
//
//  1. THE RENDER, resolved by name over the test corpus: a function whose
//     body calls BwrapArgs or bwrapArgsWatched, or that reaches one through
//     identifier calls. All three live violations rendered in their own body
//     and all three took their home from a fixture two or three hops away,
//     which is the direction the reach actually has to run in.
//  2. THE HOME, resolved by SCOPE and then by that same reach: a
//     `Setenv("HOME", …)` whose value derives from a bare temp root, in the
//     body or in anything the body calls — with the three declared roots
//     SEALING the propagation. A function that calls one of them has
//     declared its own premise and is answerable for that root, which is
//     what keeps sbRoot's own internal `wtqaHome` from reading as every
//     caller's violation (sbRoot takes the blanket grant away from that home
//     deliberately, and its contract is its own pin).
//
// A METHOD CALL IS NOT AN EDGE, and that is the one place this scan is
// deliberately narrower than it looks. A receiver's type cannot be resolved
// from source alone — and source is what has to be read here, because the
// package is three build-tag arms and only parser.ParseFile sees all three at
// once — so a method edge keyed by NAME alone joins one type's `render` to
// another type's. MEASURED 2026-11-22 on this box (linux 7.2.15, go1.27.3, at
// 4f309bcb): with method names as edges the scan read 26 functions where 3
// were real, and 10 of the 23 false reads were `qcwRig.render` and a `render`
// closure in cost_test.go reaching a BwrapArgs that belongs to neither. Those
// 23 would each have been "fixed" by moving a fixture that is deliberately
// built another way. temproot_qa_test.go drops a name the package binds twice
// rather than guessing; this drops the whole namespace, for the same reason
// and with the same consequence — a render or a home that arrives only
// through a method is not read here.
//
// THE RENDER THIS PIN READS IS BwrapArgs AND ITS WATCHED TWIN, AND NOT THE
// PRODUCTION ENTRY POINTS ABOVE THEM. ranger-base-shf97 held that half open,
// measured every widening that could close it, and closed the bead on NO
// RULE: not one variant sees the two arms it was filed over without dragging
// in false reads, and the cheapest that sees them at all costs fifteen. The
// table is six rows down this comment and
// docs/notes.d/ranger-base-shf97.md has the instrument.
// This was tried and measured, same box and commit, and the widening is not
// the answer as it stands. Deriving the production functions that render
// bwrap on EVERY host — a path to BwrapArgs along uniquely-named edges with
// every `WallMechanism()` caller CUT, because a path through the mechanism
// switch renders bwrap only on linux, where `t.TempDir()` IS the stand-down
// root and this class cannot bite — gives a clean set of 10 from 3270
// production declarations (91 ambiguous names dropped, 17 mechanism-asking
// functions cut; following the ambiguous names instead puts 1651 of the 3270
// in the set, which is the measurement that says why they are dropped):
//
//	BwrapArgs BwrapReport RenderBwrap bwrapArgsWatched bwrapRender
//	mcpConfigWallRefusalOn mcpWriteSet renderBwrapWall renderWallInto
//	renderWallOn
//
// Seeded in, that set flagged 13 live tests and ALL 13 ARE GREEN under the
// TMPDIR=/var/tmp repro this class is reproduced with — because `bwrapRender`
// drops the blanket temp grants before BwrapArgs sees them
// (bwrapDropTmpGrants), so on that path the refusal needs the home to be
// granted by something else. It also still missed the two arms that ARE red
// there (wallseedprojectconfig_qa_test.go, whose seed call is inside a
// method). 13 false and 2 missed is not a door, so the door is stated over
// the render a test writes.
//
// EVERY OTHER WIDENING WAS THEN MEASURED TOO, and none of them is a door
// either (ranger-base-shf97, 2026-11-22, same box, over the live corpus with
// that file's pre-fix source overlaid so its two homes read bare again):
//
//	variant                                        reach  flagged  real  false
//	this pin as it stands                            106        0     0      0
//	+ a method edge where the name is UNIQUE         109        0     0      0
//	+ a method edge keyed by NAME ALONE (unsound)    142        2     0      2
//	the 10 production seeds, identifier edges        189       13     0     13
//	the 10 + unique-name method edges                192       13     0     13
//	the 10 + name-alone method edges (unsound)       230       17     2     15
//	file-scoped, narrow seeds (no edges at all)        —    5 fls     0      8
//	file-scoped, the 10 seeds                          —   16 fls     2     23
//
// UNIQUENESS WAS THE ONE SOUND NARROWING AND IT DOES NOT REACH, which is the
// finding. The rule would have been temproot_qa_test.go's — admit a method
// edge whose name has exactly one declaration in the tree and is bound
// nowhere else — and the blocker is not the render but the HOP: `bwrapRender`
// is unique, while `prep` is declared twice in internal/posse and bound as a
// local besides, so `f.prep(t, f.home)` is inadmissible by exactly the rule
// that makes `bwrapRender` admissible. The two the narrow seeds flag with
// name-alone edges are canonicalwall_qa_test.go's — this comment's own
// measured failure mode, at a different pair.
//
// AND THE DOOR FOR THAT PATH TURNED OUT NOT TO BE A SCAN. MEASURED the same
// day: `TMPDIR=/var/tmp/… go test ./internal/posse` (arm 1, with those two
// arms fixed) is 263.0s and reds TWO MORE live members of this class that no
// variant above flags at all — wallwitness_qa_test.go's, whose home is
// wtqaHome's and whose render is reached through `RenderWall`. Four members in
// arm 1: the best static variant flags two and costs fifteen false reads, and
// the run finds four and costs none. So this pin keeps the class it can state
// soundly, the remaining members and the venue are beads, and
// docs/notes.d/ranger-base-shf97.md has every reading.
//
// Reading the SOURCE rather than building it is also what covers all three
// arms at once: parser.ParseFile applies no build constraints, so the
// untagged, posse_arm2 and posse_arm3 files are all read by one run.
//
// WHERE IT LIVES. internal/treepins, not internal/posse: the subject is
// internal/posse's whole test corpus, ~950s is past what a seat spends in one
// foreground call, and a pin nobody's `-run` filter names is unreachable
// exactly when it matters (ranger-base-rulbl's class). Every test in this
// package is inside `make treepins-check` by construction, which is the
// complement door ADR 0098 landed, so this needs no Makefile line and no
// membership row of its own (treewidedoor_qa_test.go says so for the nine
// that preceded it).
//
// NOT CLAIMED: that the three live fixtures are the only shapes that can red
// a macos leg, or that a darwin run was made here. Nothing in this package
// renders a wall; this is a census over source. What IS claimed is the
// red-before/green-after: with ranger-base-1aayx's fix reverted
// (`git apply -R` of d8acb142's two files), arm 1 names exactly
// TestQABwrapRendersNoWriteBindInTheStoreRepoGitDir,
// TestQAEveryWallResolvesTheStoreSpellingBeadsHomeAnswers and
// TestQAWholeTreeDenyRendersAtTheBwrapMechanism and nothing else; restored, it
// is green. The first two are the two rows macos arm 1 carried at 2a50bcb5,
// word for word.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	// bhsRenderPure and bhsRenderWatched are the two spellings of the render
	// this rule is about — BwrapArgs and its third-output twin.
	bhsRenderPure    = "BwrapArgs"
	bhsRenderWatched = "bwrapArgsWatched"
	// bhsTmpMountConst is the package const naming the one blanket temp grant
	// both hosts spell the same way. Named rather than spelled, so a fixture
	// written against the const reads as declared.
	bhsTmpMountConst = "bwrapTmpMount"
	bhsHomeVar       = "HOME"
)

// bhsRenderNames is the render as a TEST writes it: the pure function and its
// watched twin. Bounded on purpose — see "THE RENDER THIS PIN READS" in the
// head comment for the production entry points it deliberately does not
// follow, and the bead that holds them.
var bhsRenderNames = map[string]bool{bhsRenderPure: true, bhsRenderWatched: true}

// bhsDeclaredRoots are the three fixture roots that answer "which case is
// this" in the source, and the whole of the rule's vocabulary. A home from
// one of them is declared; a home from anything else is not.
var bhsDeclaredRoots = map[string]bhsClass{
	"wallStandDownDir": bhsStandDown,
	"wallUngrantedDir": bhsUngranted,
	"sbRoot":           bhsMovedGrant,
}

type bhsClass int

const (
	bhsUnknown    bhsClass = iota // a value this rule cannot trace
	bhsStandDown                  // inside /tmp: the render's stand-downs cover it
	bhsUngranted                  // outside every blanket grant: the refusal is the subject
	bhsMovedGrant                 // the grant was moved off this home instead
	bhsBare                       // a temp root whose coverage is the host's accident
)

// bhsFunc is one function declared in internal/posse/*_test.go, read for the
// two properties the rule is stated over.
type bhsFunc struct {
	name   string
	where  string
	method bool // a method: analysed, but its name is not an edge

	renders  []string // BwrapArgs / bwrapArgsWatched call sites in this body
	homeBare []string // HOME set from a root nothing declared
	homeDecl []string // HOME set from one of the three
	homeDark []string // HOME set from a value this rule cannot trace
	// open: this body sets HOME from a value it did not produce and that is
	// not a literal — the `…FixtureIn(t, root)` idiom, where the root is the
	// CALLER's and so is the class. Its callers are classified at the call
	// site instead (bhsOpenHomes).
	open bool

	allocs map[bhsClass]int
	calls  map[string]string // identifier calls, locally shadowed names dropped
	// argClass is, per identifier call, the join of its arguments' classes in
	// THIS body — what bhsOpenHomes reads to classify an open fixture's home
	// at the call site.
	argClass map[string]bhsClass
	seals    []string // calls to a declared fixture root
	rets     bool
}

func bhsAt(fset *token.FileSet, p token.Pos) string {
	at := fset.Position(p)
	return fmt.Sprintf("%s:%d", at.Filename, at.Line)
}

func bhsStr(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// bhsCallee is the name a call names, whether written as an identifier or as
// a selector: `BwrapArgs(…)`, `f.a.SeatbeltWritable(…)` and `t.TempDir()` all
// answer their last component. Used for the SEEDS, which are matched by name
// wherever they are written; the call GRAPH is identifiers only.
func bhsCallee(x *ast.CallExpr) string {
	switch fun := x.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

// bhsAllocClass classifies one call that produces a temp root. `os.MkdirTemp`
// under the /tmp const or literal is a stand-down root by construction — that
// is what wallStandDownDir and bwsHome both are — and under anything else it
// is bare.
func bhsAllocClass(x *ast.CallExpr) (bhsClass, bool) {
	name := bhsCallee(x)
	if c, ok := bhsDeclaredRoots[name]; ok {
		return c, true
	}
	switch name {
	case "TempDir":
		return bhsBare, true
	case "MkdirTemp":
		if len(x.Args) > 0 {
			if id, ok := x.Args[0].(*ast.Ident); ok && id.Name == bhsTmpMountConst {
				return bhsStandDown, true
			}
			if s, ok := bhsStr(x.Args[0]); ok && (s == "/tmp" || s == "/private/tmp") {
				return bhsStandDown, true
			}
		}
		return bhsBare, true
	}
	return bhsUnknown, false
}

// bhsWorse is the join: a value that mixes a declared root with a bare one is
// bare, because the bare half is the half a macos leg reads.
func bhsWorse(a, b bhsClass) bhsClass {
	if a > b {
		return a
	}
	return b
}

// bhsExprClass classifies one expression — the second argument of a
// `Setenv("HOME", …)` — against the local bindings and the package's
// root-returning providers.
func bhsExprClass(e ast.Expr, binds map[string]bhsClass, providers map[string]bhsClass) bhsClass {
	got := bhsUnknown
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if c, ok := bhsAllocClass(x); ok {
				got = bhsWorse(got, c)
				return true
			}
			if c, ok := providers[bhsCallee(x)]; ok {
				got = bhsWorse(got, c)
			}
		case *ast.Ident:
			if c, ok := binds[x.Name]; ok {
				got = bhsWorse(got, c)
			}
		}
		return true
	})
	return got
}

// bhsLocalNames is every name bound inside this function — parameters,
// results, receivers, `:=` and `var` targets, range variables. An identifier
// call to one of them is a closure or a function value, not an edge to the
// package-level function of the same name: cost_test.go's `render := func(…)`
// is why.
func bhsLocalNames(fn *ast.FuncDecl) map[string]bool {
	out := map[string]bool{}
	add := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			for _, n := range f.Names {
				out[n.Name] = true
			}
		}
	}
	add(fn.Recv)
	add(fn.Type.Params)
	add(fn.Type.Results)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if x.Tok != token.DEFINE {
				return true
			}
			for _, lhs := range x.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					out[id.Name] = true
				}
			}
		case *ast.ValueSpec:
			for _, id := range x.Names {
				out[id.Name] = true
			}
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{x.Key, x.Value} {
				if id, ok := e.(*ast.Ident); ok {
					out[id.Name] = true
				}
			}
		case *ast.FuncLit:
			add(x.Type.Params)
			add(x.Type.Results)
		}
		return true
	})
	return out
}

// bhsRead runs the detector over already-parsed files. Both live and planted
// source go through it, so the control arm reads the same instrument arm 1
// reports from.
func bhsRead(fset *token.FileSet, files []*ast.File, seeds map[string]bool) []*bhsFunc {
	type decl struct {
		fn *ast.FuncDecl
		f  *bhsFunc
	}
	var decls []decl
	// The three declared roots are seeded rather than derived: each one
	// allocates a root of its own by hand (wallUngrantedDir's is an
	// os.MkdirTemp under /var/tmp, sbRoot's is a gitTempDir), so a derived
	// reading of their own bodies would classify them as the thing they
	// exist to be the alternative to.
	providers := map[string]bhsClass{}
	for name, c := range bhsDeclaredRoots {
		providers[name] = c
	}

	for _, file := range files {
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			f := &bhsFunc{
				name:     fn.Name.Name,
				where:    bhsAt(fset, fn.Pos()),
				method:   fn.Recv != nil,
				allocs:   map[bhsClass]int{},
				calls:    map[string]string{},
				argClass: map[string]bhsClass{},
				rets:     bhsReturnsString(fn),
			}
			locals := bhsLocalNames(fn)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				x, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := bhsCallee(x)
				if seeds[name] {
					f.renders = append(f.renders, name+" at "+bhsAt(fset, x.Pos()))
				}
				if _, declared := bhsDeclaredRoots[name]; declared {
					f.seals = append(f.seals, name+" "+bhsAt(fset, x.Pos()))
				}
				if c, ok := bhsAllocClass(x); ok {
					f.allocs[c]++
				}
				if id, ok := x.Fun.(*ast.Ident); ok && !locals[id.Name] {
					if _, seen := f.calls[id.Name]; !seen {
						f.calls[id.Name] = bhsAt(fset, x.Pos())
					}
				}
				return true
			})
			decls = append(decls, decl{fn, f})
		}
	}

	// Providers: a function that allocates a root and hands a string back is
	// the same exposure as the root itself. Three passes, so a provider built
	// on a provider (claudeConfigDirForTest on gitTempDir) resolves.
	for pass := 0; pass < 3; pass++ {
		for _, d := range decls {
			if !d.f.rets {
				continue
			}
			if _, declared := bhsDeclaredRoots[d.f.name]; declared {
				continue
			}
			c := bhsUnknown
			for _, k := range []bhsClass{bhsBare, bhsMovedGrant, bhsUngranted, bhsStandDown} {
				if d.f.allocs[k] > 0 {
					c = k
					break
				}
			}
			if c == bhsUnknown {
				for name := range d.f.calls {
					if pc, ok := providers[name]; ok {
						c = bhsWorse(c, pc)
					}
				}
			}
			if c != bhsUnknown && c > providers[d.f.name] {
				providers[d.f.name] = c
			}
		}
	}

	for _, d := range decls {
		binds := map[string]bhsClass{}
		for pass := 0; pass < 3; pass++ {
			ast.Inspect(d.fn.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				// `home, err := os.MkdirTemp(…)` is one right-hand side for
				// two names, and it is the spelling every by-hand root in
				// this corpus is written in — so a 1:n assignment binds
				// every name to that one value's class.
				for i, lhs := range as.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					rhs := as.Rhs[0]
					if len(as.Rhs) == len(as.Lhs) {
						rhs = as.Rhs[i]
					} else if len(as.Rhs) != 1 {
						continue
					}
					if c := bhsExprClass(rhs, binds, providers); c > binds[id.Name] {
						binds[id.Name] = c
					}
				}
				return true
			})
		}
		ast.Inspect(d.fn.Body, func(n ast.Node) bool {
			x, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := x.Fun.(*ast.Ident); ok {
				if _, edge := d.f.calls[id.Name]; edge {
					c := d.f.argClass[id.Name]
					for _, arg := range x.Args {
						c = bhsWorse(c, bhsExprClass(arg, binds, providers))
					}
					d.f.argClass[id.Name] = c
				}
			}
			if bhsCallee(x) != "Setenv" || len(x.Args) < 2 {
				return true
			}
			if s, ok := bhsStr(x.Args[0]); !ok || s != bhsHomeVar {
				return true
			}
			at := bhsAt(fset, x.Pos())
			switch c := bhsExprClass(x.Args[1], binds, providers); {
			case c == bhsBare:
				d.f.homeBare = append(d.f.homeBare, at)
			case c != bhsUnknown:
				d.f.homeDecl = append(d.f.homeDecl, at)
			default:
				d.f.homeDark = append(d.f.homeDark, at)
				// A literal home — `t.Setenv("HOME", "")` is the whole of
				// this class, the no-home refusal pins — is not a root at
				// all and nothing upstream can make it one.
				if _, lit := bhsStr(x.Args[1]); !lit {
					d.f.open = true
				}
			}
			return true
		})
	}

	out := make([]*bhsFunc, 0, len(decls))
	for _, d := range decls {
		out = append(out, d.f)
	}
	bhsOpenHomes(out)
	return out
}

// bhsOpenHomes attributes an open fixture's home at the CALL SITE, which is
// where its class actually lives.
//
// THE IDIOM THIS EXISTS FOR is `…FixtureIn(t, root)`: the fixture sets HOME
// to a path under a root its CALLER chose, and this corpus writes that
// eighteen ways over (wgFixtureIn, wrcFixtureIn, weFixtureIn, wspFixtureIn).
// Most call sites hand it `wallUngrantedDir(t)` — declared, and the whole
// point of the idiom — and some hand it `t.TempDir()`, which is the defect.
// Read inside the fixture alone, both are the same untraceable `f.home`.
//
// THE ARGUMENT IS NOT MATCHED TO THE PARAMETER, deliberately: the home
// reaches that Setenv through a struct field and a closure, so which
// parameter it came from is not a thing source can answer here. The join over
// ALL arguments is over-broad in the one direction that is safe — a caller
// that hands such a fixture a bare temp root at all is a caller whose home
// may be that root, and the remedy is the same declared root either way.
func bhsOpenHomes(funcs []*bhsFunc) {
	for pass := 0; pass < 3; pass++ {
		open := map[string]bool{}
		for _, f := range funcs {
			if f.open && !f.method {
				open[f.name] = true
			}
		}
		grew := false
		for _, f := range funcs {
			for name, at := range f.calls {
				if !open[name] || name == f.name {
					continue
				}
				switch c := f.argClass[name]; {
				case c == bhsBare:
					if !containsStringBhs(f.homeBare, at) {
						f.homeBare = append(f.homeBare, at)
						grew = true
					}
				case c == bhsUnknown && !f.open:
					// The root this caller hands on is its own caller's.
					f.open = true
					grew = true
				}
			}
		}
		if !grew {
			return
		}
	}
}

func containsStringBhs(set []string, s string) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}

func bhsReturnsString(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, r := range fn.Type.Results.List {
		if id, ok := r.Type.(*ast.Ident); ok && id.Name == "string" {
			return true
		}
	}
	return false
}

// bhsReach is the fixpoint over the identifier call graph: which functions
// have a seeded property of their own or reach one. `sealed` names the
// functions the propagation stops at — a function that declared its own
// fixture root is answerable for that root and not for one three hops down.
//
// A METHOD'S NAME IS NOT EXPORTED INTO THE GRAPH (see the head comment): its
// own properties are read, and a caller reaches them through nothing.
func bhsReach(funcs []*bhsFunc, seed func(*bhsFunc) bool, sealed func(*bhsFunc) bool) map[*bhsFunc]bool {
	has := map[*bhsFunc]bool{}
	byName := map[string]bool{}
	mark := func(f *bhsFunc) {
		has[f] = true
		if !f.method {
			byName[f.name] = true
		}
	}
	for _, f := range funcs {
		if seed(f) {
			mark(f)
		}
	}
	for {
		grew := false
		for _, f := range funcs {
			if has[f] || (sealed != nil && sealed(f)) {
				continue
			}
			for name := range f.calls {
				if byName[name] && name != f.name {
					mark(f)
					grew = true
					break
				}
			}
		}
		if !grew {
			return has
		}
	}
}

// bhsViolations is the rule itself: a function that drives the bwrap render
// from a home nothing declared. One line per function, sorted, so a red names
// every site at once rather than one per run.
func bhsViolations(funcs []*bhsFunc) []string {
	renders := bhsReach(funcs, func(f *bhsFunc) bool { return len(f.renders) > 0 }, nil)
	bare := bhsReach(funcs,
		func(f *bhsFunc) bool { return len(f.homeBare) > 0 },
		func(f *bhsFunc) bool { return len(f.seals) > 0 && len(f.homeBare) == 0 })

	setters := map[string]string{}
	for _, f := range funcs {
		if len(f.homeBare) > 0 && !f.method {
			setters[f.name] = f.homeBare[0]
		}
	}

	var bad []string
	for _, f := range funcs {
		if !renders[f] || !bare[f] {
			continue
		}
		via := "its own body, at " + strings.Join(f.homeBare, ", ")
		if len(f.homeBare) == 0 {
			var out []string
			for name, at := range f.calls {
				if site, ok := setters[name]; ok {
					out = append(out, name+" ("+site+", called at "+at+")")
				}
			}
			sort.Strings(out)
			via = "a fixture it calls: " + strings.Join(out, ", ")
			if len(out) == 0 {
				via = "a fixture further down its own call graph"
			}
		}
		where := "it reaches that render through its own call graph"
		if len(f.renders) > 0 {
			where = "it calls " + f.renders[0]
		}
		bad = append(bad, fmt.Sprintf("%s (%s) drives the bwrap render — %s — from a $HOME set by %s. On linux that home is under %s and the wall-root refusal stands down for free; on darwin it is $TMPDIR under /var/folders and the render REFUSES its own fixture, so this arm reads a refusal instead of the wall it measures (ADR 0100 D1/D2(a), ranger-base-jhmm7). Take the home from one of %s",
			f.name, f.where, where, via, bhsTmpMountConst, bhsDeclaredRootNames()))
	}
	sort.Strings(bad)
	return bad
}

func bhsDeclaredRootNames() string {
	var out []string
	for name := range bhsDeclaredRoots {
		out = append(out, name)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// bhsParsePosse reads internal/posse/*_test.go with no build constraints
// applied, which is what puts all three arms under one run. Relative, because
// internal/treepins' TestMain chdirs this binary to the repo root.
func bhsParsePosse(t *testing.T) (*token.FileSet, []*ast.File) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("internal", "posse", "*_test.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no test files found under internal/posse: %v", err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, file)
	}
	return fset, files
}

// Arm 1: the class, over the live corpus.
func TestQAEveryBwrapRenderInThePosseTestsSitsOnADeclaredFixtureHome(t *testing.T) {
	t.Parallel()
	fset, files := bhsParsePosse(t)
	funcs := bhsRead(fset, files, bhsRenderNames)

	// Floors. Every arm here reports absence, and absence is what a scan that
	// has quietly stopped reading also reports. These sit well under the live
	// figures (MEASURED 2026-11-22, linux 7.2.15, go1.27.3, at 4f309bcb: 762
	// files, 7234 functions, 106 reaching the render, and of the $HOME sites
	// 17 declared, 147 bare, 34 untraced) and exist only to separate "no
	// violations" from "no input".
	//
	// THE BARE FLOOR IS THE OPEN-FIXTURE MECHANISM'S TOO. 27 of those 147 are
	// attributed at a call site by bhsOpenHomes rather than read in a body, so
	// a floor above 120 is also what says that half is still deriving — the
	// half that cannot be seen any other way, because an open fixture handed a
	// declared root produces no record at all.
	renders := bhsReach(funcs, func(f *bhsFunc) bool { return len(f.renders) > 0 }, nil)
	declared, barely, dark := 0, 0, 0
	for _, f := range funcs {
		declared += len(f.homeDecl)
		barely += len(f.homeBare)
		dark += len(f.homeDark)
	}
	if len(files) < 400 || len(funcs) < 3000 {
		t.Fatalf("parsed %d files and %d functions under internal/posse — the corpus this pin reads is not there", len(files), len(funcs))
	}
	if len(renders) < 50 {
		t.Fatalf("only %d functions reach %s — the detector has stopped seeing the render it is a census of", len(renders), bhsRenderPure)
	}
	if declared < 10 {
		t.Fatalf("only %d $HOME sites resolve to one of the declared fixture roots (%s) — the rule's whole vocabulary is deriving nothing, so every render would read as a violation or none would", declared, bhsDeclaredRootNames())
	}
	if barely < 120 {
		t.Fatalf("only %d $HOME sites resolve to a bare temp root — this scan sees %d functions and a corpus that sets HOME %d times by that reading, which is not this package", barely, len(funcs), barely+declared+dark)
	}
	t.Logf("%d files, %d functions, %d reaching %s; $HOME sites: %d declared, %d bare, %d untraced",
		len(files), len(funcs), len(renders), bhsRenderPure, declared, barely, dark)

	for _, v := range bhsViolations(funcs) {
		t.Errorf("%s", v)
	}
}

// Arm 2: the control. A census that cannot say no is a census that reports
// green over a tree it has stopped reading. Each case below is a shape that
// either IS the live defect or escaped a reading of it — the three live
// fixtures' own shape first, then every reason a function in this corpus is
// legitimately not a violation.
func TestQABwrapFixtureHomeCensusCanStillSayNo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want bool
	}{{
		name: "the live defect: a bare home and a render in one body",
		src: `func f(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			BwrapArgs(dir, grants, SeatbeltCarveOut{})
		}`,
		want: true,
	}, {
		name: "the home three hops down a fixture — ranger-base-1aayx's shape",
		src: `func wtqaHome(t *testing.T) { t.Setenv("HOME", t.TempDir()) }
		func fixtureAnyBox(t *testing.T) string {
			wtqaHome(t)
			return gitTempDir(t)
		}
		func newRig(t *testing.T) string { return fixtureAnyBox(t) }
		func f(t *testing.T) {
			tree := newRig(t)
			BwrapArgs(tree, grants, SeatbeltCarveOut{})
		}`,
		want: true,
	}, {
		name: "the fix: the same fixture on a stand-down root",
		src: `func fixtureAnyBox(t *testing.T) string {
			root := wallStandDownDir(t)
			t.Setenv("HOME", sbMkdir(t, filepath.Join(root, "seat-home")))
			return sbMkdir(t, filepath.Join(root, "work"))
		}
		func f(t *testing.T) {
			tree := fixtureAnyBox(t)
			BwrapArgs(tree, grants, SeatbeltCarveOut{})
		}`,
		want: false,
	}, {
		name: "bwsHome's spelling: os.MkdirTemp under the /tmp const by hand",
		src: `func bwsHome(t *testing.T) string {
			home, _ := os.MkdirTemp(bwrapTmpMount, "posse-bwsd-")
			home = absResolve(home)
			t.Setenv("HOME", home)
			return home
		}
		func f(t *testing.T) {
			home := bwsHome(t)
			BwrapArgs(home, grants, SeatbeltCarveOut{})
		}`,
		want: false,
	}, {
		name: "the refusal's own pin, from wallUngrantedDir",
		src: `func f(t *testing.T) {
			root := wallUngrantedDir(t)
			home := filepath.Join(root, "home")
			t.Setenv("HOME", home)
			if _, _, err := BwrapArgs(root, []string{home}, SeatbeltCarveOut{}); err == nil {
				t.Error("a grant over the wall root rendered")
			}
		}`,
		want: false,
	}, {
		name: "sbRoot: the grant moves instead, and its own wtqaHome is sealed",
		src: `func wtqaHome(t *testing.T) { t.Setenv("HOME", t.TempDir()) }
		func sbRoot(t *testing.T) string {
			wtqaHome(t)
			tmp := gitTempDir(t)
			root := gitTempDir(t)
			t.Setenv("TMPDIR", tmp)
			sbNoBlanketTempGrants(t, root)
			return root
		}
		func newFixture(t *testing.T) string { return sbRoot(t) }
		func f(t *testing.T) {
			tree := newFixture(t)
			BwrapArgs(tree, grants, SeatbeltCarveOut{})
		}`,
		want: false,
	}, {
		name: "a closure shadowing a package function is not an edge",
		src: `func render(t *testing.T) { BwrapArgs(dir, grants, SeatbeltCarveOut{}) }
		func f(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			render := func(r *CostReport) string { return r.String() }
			_ = render(nil)
		}`,
		want: false,
	}, {
		name: "a bare home with no render anywhere is not this pin's business",
		src: `func f(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			_ = SeatbeltProfile("dev", nil, nil, SeatbeltCarveOut{})
		}`,
		want: false,
	}, {
		name: "a render with no fixture home at all is not this pin's business",
		src: `func f(t *testing.T) {
			BwrapArgs("/proj", []string{"/proj"}, SeatbeltCarveOut{})
		}`,
		want: false,
	}, {
		name: "a bare os.MkdirTemp root is the same defect as t.TempDir",
		src: `func f(t *testing.T) {
			home, _ := os.MkdirTemp("/var/tmp", "x")
			t.Setenv("HOME", home)
			BwrapArgs(home, grants, SeatbeltCarveOut{})
		}`,
		want: true,
	}, {
		name: "the watched render is the same render, and gitTempDir is derived",
		src: `func gitTempDir(t *testing.T) string {
			d := t.TempDir()
			t.Cleanup(func() { removeAllTolerant(t, d) })
			return d
		}
		func f(t *testing.T) {
			t.Setenv("HOME", gitTempDir(t))
			bwrapArgsWatched(dir, grants, SeatbeltCarveOut{})
		}`,
		want: true,
	}, {
		name: "the open-fixture idiom: the root is the caller's, and it is bare",
		src: `func feFixtureIn(t *testing.T, root string) feFixture {
			at := func(p ...string) string { return filepath.Join(append([]string{root}, p...)...) }
			f := feFixture{home: at("home")}
			t.Setenv("HOME", f.home)
			return f
		}
		func f(t *testing.T) {
			fx := feFixtureIn(t, t.TempDir())
			BwrapArgs(fx.dir, grants, SeatbeltCarveOut{})
		}`,
		want: true,
	}, {
		name: "the same idiom handed a declared root is the idiom working",
		src: `func feFixtureIn(t *testing.T, root string) feFixture {
			at := func(p ...string) string { return filepath.Join(append([]string{root}, p...)...) }
			f := feFixture{home: at("home")}
			t.Setenv("HOME", f.home)
			return f
		}
		func f(t *testing.T) {
			fx := feFixtureIn(t, wallUngrantedDir(t))
			BwrapArgs(fx.dir, grants, SeatbeltCarveOut{})
		}`,
		want: false,
	}, {
		name: "the open fixture's own callers are read, not the fixture",
		src: `func feFixtureIn(t *testing.T, root string) feFixture {
			f := feFixture{home: filepath.Join(root, "home")}
			t.Setenv("HOME", f.home)
			BwrapArgs(f.home, grants, SeatbeltCarveOut{})
			return f
		}
		func ok(t *testing.T) { feFixtureIn(t, wallStandDownDir(t)) }`,
		want: false,
	}, {
		name: "a declared root joined with a bare one is still bare",
		src: `func f(t *testing.T) {
			root := wallStandDownDir(t)
			t.Setenv("HOME", filepath.Join(root, t.TempDir()))
			BwrapArgs(root, grants, SeatbeltCarveOut{})
		}`,
		want: true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "planted_test.go", "package posse\n"+tc.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse planted source: %v", err)
			}
			got := bhsViolations(bhsRead(fset, []*ast.File{file}, bhsRenderNames))
			if (len(got) > 0) != tc.want {
				t.Fatalf("violations = %v; want flagged = %v", got, tc.want)
			}
		})
	}
}

// Arm 3: every declared root is reachable from every arm. The rule's whole
// vocabulary is three helper names, and a copy of one behind a build tag
// would compile in one arm and leave the other two with nothing to convert
// to — which arm 1 reads source and so could not notice. ranger-base-qp1hm's
// own escape, one namespace over (2115efb1 verified a fix under `-tags
// posse_arm3` and left two sites in the posse_arm2 file it never compiled).
func TestQATheDeclaredBwrapFixtureRootsCompileInEveryArm(t *testing.T) {
	t.Parallel()
	fset, files := bhsParsePosse(t)
	found := map[string]string{}
	for _, file := range files {
		var tags []string
		// file.Comments is populated only because bhsParsePosse passes
		// parser.ParseComments; without it this loop never runs and the tag
		// half of this arm silently cannot fail (ranger-base-3klcb).
		for _, group := range file.Comments {
			for _, c := range group.List {
				if strings.HasPrefix(c.Text, "//go:build") {
					tags = append(tags, c.Text)
				}
			}
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if _, declared := bhsDeclaredRoots[fn.Name.Name]; !declared {
				continue
			}
			at := bhsAt(fset, fn.Pos())
			if prev, seen := found[fn.Name.Name]; seen {
				t.Errorf("%s is declared twice, at %s and %s — one of them is a tagged or hand-copied twin, and a copy is how the arms drift apart", fn.Name.Name, prev, at)
			}
			found[fn.Name.Name] = at
			for _, tag := range tags {
				t.Errorf("%s is declared in a file carrying %q — the other arms then have no declared fixture root to take a home from, and every render in them reads as a violation of a rule they cannot satisfy", fn.Name.Name, tag)
			}
		}
	}
	for name := range bhsDeclaredRoots {
		if found[name] == "" {
			t.Errorf("internal/posse's tests no longer declare %s, which this rule's vocabulary is stated over — a root that is gone silently reclassifies every home that came from it as bare", name)
		}
	}
	if !t.Failed() {
		var rows []string
		for name, at := range found {
			rows = append(rows, name+" "+at)
		}
		sort.Strings(rows)
		t.Logf("declared fixture roots, each once and untagged:\n  %s", strings.Join(rows, "\n  "))
	}
}
