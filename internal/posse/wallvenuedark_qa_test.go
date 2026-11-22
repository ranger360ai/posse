//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-ddn56's Finding 2 — the one direction of abstention
// that the off-/tmp venue CAUSES.
//
// THE DEFECT. `make tmpdir-check` (ADR 0098 D5, ranger-base-m33n5) runs
// internal/posse with TMPDIR at a `/var/tmp` root, because that is the
// discriminator for the bwrap fixture-home class: a render that must succeed
// is green only where the fixture `$HOME` lands under `/tmp`, which is where
// `bwrapWallRoot` stands the wall-root refusal down. The door is a plain `go
// test`, so a member that SKIPS under that venue prints `ok` — and one member
// did. MEASURED 2026-11-22 (linux 7.2.15 x86_64, go1.27.3, 16 cores, at
// 91381cab), `-run 'TestQAWallPinnedDstPrivateSetGateStandsDownWithTheWall|
// TestQAWallClaudeProjectsRendersOwnKeyInsideAReadOnlyProjects'
// ./internal/posse -v`:
//
//	TMPDIR=/var/tmp/…   SKIP: …PrivateSetGateStandsDownWithTheWall (0.00s)
//	                    package line: ok 5.376s
//	TMPDIR unset        PASS: the same test (0.00s), ok 1.125s
//
// An abstention is never a pass (ADR 0097 D5), and that record's own
// `/var/tmp` row is a fixture change that took 18 pins dark over a green exit
// code. The venue moved membership and nothing counted it either way.
//
// THE DIRECTION IS THE WHOLE OF THE RULE, and both directions exist in this
// package. A skip guarded by `bwrapWallRoot() == ""` is dark on a box whose
// home IS under `/tmp` and MEASURES under the venue: that is the venue's
// yield, and fourteen sites are spelled that way on purpose
// (wallcarvepins, wallpinnedsource, wallseatwall, wallvenueargv, keepleaf,
// wallpinneddst's live half). A skip guarded by `bwrapWallRoot() != ""` is
// the inverse — dark under the venue, measured nowhere else — and that is
// this pin's subject. There was exactly one, and its fix was a root that
// measures in both shapes rather than a door that catches the skip
// (wallpinneddst_qa_test.go).
//
// WHY A SCAN HERE AND A RUN FOR THE CLASS. ADR 0098 D5 rules that the door
// for the fixture-home class is a RUN, and ranger-base-shf97 measured why: no
// static variant finds those members without false reads. This pin is not
// that class. Its subject is one syntactic fact — which way a `t.Skip`'s own
// guard points — and the predicate is a single call compared against the
// empty string at every live site, so there is nothing to infer and no
// dataflow to follow. A condition that reads `bwrapWallRoot` in any OTHER
// shape is reported rather than passed, because a shape this rule cannot read
// is a shape it cannot say the direction of.
//
// AND THE KEY IS `bwrapWallRoot` AND NOT "ANYTHING THE VENUE MOVES", which is
// the residual, said plainly. The venue moves TMPDIR, so a gate keyed on
// `os.TempDir()` or on where `t.TempDir()` lands is in the same position and
// is not read here. What this scan can say is that `bwrapWallRoot` is the only
// key SPELLED in the package today: MEASURED 2026-11-22 over 764 test files,
// 14 guarded skips and every one of them `== ""`. What it cannot say is that
// no OTHER key darkens a member, and the thing that decides that is a RUN —
// arm 1 under both shapes, `-v`, the SKIP sets diffed — which is ADR 0098 D5's
// own answer for this class and is `make tmpdir-check`'s reason for existing.
// docs/notes.d/ranger-base-ddn56.md carries that run's readings.
//
// It reads every `*_test.go` in the package directory, so all three build-tag
// arms are in the corpus whichever arm runs it: `parser.ParseFile` applies no
// build constraints. That is also why this file is a pkgcensus member — the
// file set is DISCOVERED and not spelled (ADR 0098, the internal/posse half)
// — and `make pkgcensus-check` is its door.
//
//	go test ./internal/posse -run 'WallVenueDark|NoWallPinGoesDark' -v

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// wvdPredicate is the venue predicate this rule reads: the production
// function whose answer the venue moves. Named once, because a second
// spelling is a second rule.
const wvdPredicate = "bwrapWallRoot"

// wvdFloor is the number of `bwrapWallRoot`-guarded skips this package held
// when the rule landed, less a margin. Every arm below reports ABSENCE, and a
// scan that has quietly stopped parsing reports absence too — so this floor is
// what separates "no site points the wrong way" from "no site was read".
// MEASURED 2026-11-22 over 764 test files: 14 guarded skips, all of them
// `== ""`; 15 before ranger-base-ddn56's fix, the fifteenth being the one
// `!= ""` this rule exists for.
const wvdFloor = 10

// wvdSite is one `t.Skip` whose guard reads the venue predicate.
type wvdSite struct {
	file, test string
	line       int
	// dark is the direction: "offTmp" when the guard is `!= ""` — the skip
	// fires where the box has a wall root, which is the venue — "onTmp" when
	// it is `== ""`, and "" when this rule could not read the comparison.
	dark string
	cond string // the condition as source, for the unreadable case
}

// wvdSkips finds every `if <cond that reads the predicate> { t.Skip… }` in a
// parsed test file and says which way each one points.
//
// THE GUARD AND NOT THE SKIP'S OWN ARGUMENT: a skip's message is prose and a
// bead may word it any way at all, while the condition is the thing that
// decides whether the test runs. ranger-base-ddn56's member said "this box's
// TMPDIR is not under /tmp" and the site two hundred lines on says "ADR
// 0100's rules stand down for a home under /tmp"; those are the same fact
// from opposite sides, and only the condition tells them apart.
func wvdSkips(fset *token.FileSet, file string, f *ast.File) []wvdSite {
	var out []wvdSite
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok || stmt.Cond == nil || !wvdSkipsIn(stmt.Body) {
				return true
			}
			// THE INIT STATEMENT COUNTS AS READING IT — `if root :=
			// bwrapWallRoot(); root != ""` is the same guard one hop on, and
			// a rule that read only the condition would call that site
			// invisible rather than unreadable. Its DIRECTION is still "",
			// because the comparison is against a local and not against the
			// call, so the site is reported and the author spells it the one
			// way this rule reads.
			if !wvdReadsNode(stmt.Cond) && (stmt.Init == nil || !wvdReadsNode(stmt.Init)) {
				return true
			}
			out = append(out, wvdSite{
				file: file, test: fn.Name.Name,
				line: fset.Position(stmt.Pos()).Line,
				dark: wvdDirection(stmt.Cond),
				cond: wvdExprText(fset, file, stmt.Cond),
			})
			return true
		})
	}
	return out
}

// wvdReadsNode is whether a node calls the venue predicate anywhere in it —
// asked of the condition and of the if-statement's own init statement.
func wvdReadsNode(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if c, ok := x.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == wvdPredicate {
				found = true
			}
		}
		return !found
	})
	return found
}

// wvdDirection reads the comparison. It is deliberately ONE shape —
// `bwrapWallRoot() <op> ""` in either operand order, with no && and no ! — so
// that anything else comes back "" and is reported rather than classified.
func wvdDirection(e ast.Expr) string {
	bin, ok := e.(*ast.BinaryExpr)
	if !ok {
		return ""
	}
	var call, lit ast.Expr
	switch {
	case wvdIsPredicateCall(bin.X) && wvdIsEmptyString(bin.Y):
		call, lit = bin.X, bin.Y
	case wvdIsPredicateCall(bin.Y) && wvdIsEmptyString(bin.X):
		call, lit = bin.Y, bin.X
	}
	if call == nil || lit == nil {
		return ""
	}
	switch bin.Op {
	case token.NEQ:
		return "offTmp"
	case token.EQL:
		return "onTmp"
	}
	return ""
}

func wvdIsPredicateCall(e ast.Expr) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return false
	}
	id, ok := c.Fun.(*ast.Ident)
	return ok && id.Name == wvdPredicate
}

func wvdIsEmptyString(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && (lit.Value == `""` || lit.Value == "``")
}

// wvdSkipsIn is whether a block's own statements call t.Skip or t.Skipf. A
// nested function literal does not count: its skip belongs to whatever drives
// it, not to this guard.
func wvdSkipsIn(b *ast.BlockStmt) bool {
	found := false
	for _, s := range b.List {
		ast.Inspect(s, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok &&
					(sel.Sel.Name == "Skip" || sel.Sel.Name == "Skipf" || sel.Sel.Name == "SkipNow") {
					found = true
				}
			}
			return !found
		})
	}
	return found
}

func wvdExprText(fset *token.FileSet, file string, e ast.Expr) string {
	src, err := os.ReadFile(file)
	if err != nil {
		return "<unreadable>"
	}
	a, b := fset.Position(e.Pos()).Offset, fset.Position(e.End()).Offset
	if a < 0 || b > len(src) || a >= b {
		return "<out of range>"
	}
	return string(src[a:b])
}

// wvdPosseTestFiles is the corpus: every `*_test.go` beside this one. `go
// test` runs with the package directory as cwd, so the relative read needs no
// root (ADR 0098, the internal/posse half — the file set is discovered, which
// is what makes this a pkgcensus member).
func wvdPosseTestFiles(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.go") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// The pin. No test in this package may abstain BECAUSE the box it runs on has
// a wall root — that is the one venue `make tmpdir-check` exists to be, and a
// skip there is a pass printed over nothing measured.
func TestQANoWallPinGoesDarkUnderTheOffTmpVenue(t *testing.T) {
	t.Parallel()
	names := wvdPosseTestFiles(t)
	if len(names) < 400 {
		t.Fatalf("read %d *_test.go files beside this one — the corpus this pin reads is not there", len(names))
	}
	var sites []wvdSite
	for _, name := range names {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, name, src, 0)
		if perr != nil {
			t.Fatalf("%s: %v", name, perr)
		}
		sites = append(sites, wvdSkips(fset, name, f)...)
	}
	if len(sites) < wvdFloor {
		t.Fatalf("only %d skip(s) in this package are guarded by %s — this rule's whole subject is that guard, so a corpus with none of them is a rule deriving nothing rather than a package with nothing to say", len(sites), wvdPredicate)
	}
	onTmp, offTmp, unread := 0, 0, 0
	for _, s := range sites {
		switch s.dark {
		case "onTmp":
			onTmp++
		case "offTmp":
			offTmp++
			t.Errorf("%s:%d: %s abstains when %s is NON-EMPTY, which is every box whose temp root is outside %s — `make tmpdir-check`'s venue, and ci.yml's macos legs, which ADR 0098 D5 makes the venue of record. So the one run built to measure this test prints `ok` over a skip, and an abstention is never a pass (ADR 0097 D5). Give the fixture a root that measures in BOTH shapes — wallStandDownDir (wall_helpers_test.go) is one under %s whatever TMPDIR says — and make the guard a CONTROL that reds (ranger-base-ddn56)",
				s.file, s.line, s.test, wvdPredicate, bwrapTmpMount, bwrapTmpMount)
		default:
			unread++
			t.Errorf("%s:%d: %s guards a t.Skip with a condition this rule cannot read the direction of: %s. It reads `%s() != \"\"` and `%s() == \"\"` in either operand order and nothing else, because a condition whose direction is unreadable is a skip nobody can say which venue darkens. Spell it as that comparison, or widen wvdDirection and say here what the new shape means (ranger-base-ddn56)",
				s.file, s.line, s.test, s.cond, wvdPredicate, wvdPredicate)
		}
	}
	// The counts, all three of them, so the log cannot read green over the
	// errors above it.
	t.Logf("%d skip(s) guarded by %s over %d test files: %d dark on a %s box (the venue's yield), %d dark UNDER the venue, %d unreadable",
		len(sites), wvdPredicate, len(names), onTmp, bwrapTmpMount, offTmp, unread)
}

// Arm 2: the control. A census that cannot say no reports green over a tree it
// has stopped reading. Each case is a shape that either IS the live defect or
// is a reason a site in this corpus is legitimately not one.
func TestQAWallVenueDarkCensusCanStillSayNo(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, src string
		want      []string // "<test>:<direction>" per site found, in order
	}{
		{
			name: "the live defect: dark where the box has a wall root",
			src: `package posse
func TestX(t *testing.T) {
	f := wrcFixtureIn(t, t.TempDir())
	if bwrapWallRoot() != "" {
		t.Skip("NOTHING MEASURED: this box's TMPDIR is not under /tmp")
	}
}`,
			want: []string{"TestX:offTmp"},
		},
		{
			name: "the yield: dark on a /tmp box, measured under the venue",
			src: `package posse
func TestX(t *testing.T) {
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under /tmp")
	}
}`,
			want: []string{"TestX:onTmp"},
		},
		{
			name: "the operands the other way round, both directions",
			src: `package posse
func TestA(t *testing.T) {
	if "" != bwrapWallRoot() {
		t.Skip("x")
	}
}
func TestB(t *testing.T) {
	if "" == bwrapWallRoot() {
		t.Skip("x")
	}
}`,
			want: []string{"TestA:offTmp", "TestB:onTmp"},
		},
		{
			name: "a negation is a direction this rule will not guess",
			src: `package posse
func TestX(t *testing.T) {
	if !(bwrapWallRoot() == "") {
		t.Skip("x")
	}
}`,
			want: []string{"TestX:"},
		},
		{
			name: "a conjunction is unreadable too, however it points",
			src: `package posse
func TestX(t *testing.T) {
	if bwrapWallRoot() != "" && runtime.GOOS == "linux" {
		t.Skip("x")
	}
}`,
			want: []string{"TestX:"},
		},
		{
			name: "a guard that does not skip is not a site",
			src: `package posse
func TestX(t *testing.T) {
	if bwrapWallRoot() != "" {
		t.Fatalf("CONTROL: this fixture's home is not where it meant to be")
	}
}`,
			want: nil,
		},
		{
			name: "a skip guarded by something else is not this rule's subject",
			src: `package posse
func TestX(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism")
	}
}`,
			want: nil,
		},
		{
			name: "the predicate read for a value, with no skip under it",
			src: `package posse
func TestX(t *testing.T) {
	if root := bwrapWallRoot(); root != "" {
		t.Logf("wall root %s", root)
	}
}`,
			want: nil,
		},
		{
			name: "an init-statement comparison: reads the predicate, direction unread",
			src: `package posse
func TestX(t *testing.T) {
	if root := bwrapWallRoot(); root != "" {
		t.Skip("x")
	}
}`,
			want: []string{"TestX:"},
		},
		{
			name: "a skip inside a nested func literal belongs to its driver",
			src: `package posse
func TestX(t *testing.T) {
	if bwrapWallRoot() != "" {
		t.Run("sub", func(t *testing.T) { t.Skip("x") })
	}
}`,
			want: nil,
		},
		{
			name: "Skipf and SkipNow are skips as surely as Skip is",
			src: `package posse
func TestA(t *testing.T) {
	if bwrapWallRoot() != "" {
		t.Skipf("NOTHING MEASURED: %s", "why")
	}
}
func TestB(t *testing.T) {
	if bwrapWallRoot() != "" {
		t.Log("x")
		t.SkipNow()
	}
}`,
			want: []string{"TestA:offTmp", "TestB:offTmp"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := dir + "/planted_test.go"
			if err := os.WriteFile(path, []byte(tc.src), 0o644); err != nil {
				t.Fatal(err)
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, tc.src, 0)
			if err != nil {
				t.Fatalf("parse planted source: %v", err)
			}
			var got []string
			for _, s := range wvdSkips(fset, path, f) {
				got = append(got, s.test+":"+s.dark)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("sites = %v; want %v", got, tc.want)
			}
		})
	}
}

// Arm 3: the predicate is still the production function this rule names. A
// rule stated over a name that no longer exists derives nothing and says
// nothing, and `go build` cannot tell the difference because the name lives
// in a string const.
func TestQATheVenuePredicateThisRuleNamesIsTheProductionOne(t *testing.T) {
	t.Parallel()
	// The binding is what ties the const below to a real function: it does
	// not compile if bwrapWallRoot is gone, renamed, or takes an argument.
	var _ func() string = bwrapWallRoot
	if wvdPredicate != "bwrapWallRoot" {
		t.Fatalf("this rule is stated over %q, and the function it compiles against is bwrapWallRoot — a rule reading one name and pinned to another reports green over every site of the one it means", wvdPredicate)
	}
}
