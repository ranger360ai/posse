package posse

// QA pin for ranger-base-exv9h. mkPrereqs and mkRuns (boxcheck_qa_test.go)
// replaced eight call sites that read a Makefile prerequisite line as BYTES
// with a tokenised read, across three beads: ranger-base-hna69 (boxcheck,
// gofmtdoor, suitelock), ranger-base-7kwb8 (treewidedoor, all four sites),
// and armtags_qa_test.go's own armTarget, converted alongside this pin.
// Every conversion left the discipline held by a COMMENT at the call site —
// "TOKENS, not the line's bytes" — and a comment does not stop a ninth call
// site being written the old way, or one of the eight being rewritten back
// to it. This is that pin.
//
// THE SHAPE. Every real instance of this defect, in three separate beads,
// was one of two things:
//
//  1. hand-rolling mkPrereqs: scanning `strings.Split(makefile, "\n")` for a
//     line found by `strings.HasPrefix(line, target+":")` (or the
//     two-result `strings.CutPrefix`), then testing membership with
//     `strings.Contains` against that line, or a `TrimPrefix`/`TrimSpace`
//     remainder of it — boxcheck, gofmtdoor and suitelock before hna69,
//     and armTarget's own `deps string` until this bead.
//  2. calling mkPrereqs (or an equivalent tokeniser) correctly and then
//     throwing away the tokenisation it bought: `strings.Join`-ing its
//     deps back into one string and asking `strings.Contains` of THAT —
//     treewidedoor's twdPrereqs callers, before ranger-base-7kwb8.
//
// Both defeat the same guarantee mkRuns exists to hold: `strings.Contains`
// treats "verify-parallel" as present in a line naming only
// "verify-parallelx", and does not stop at a Makefile `#` comment. mkRuns
// (slices.Contains over TOKENS) does both correctly, and is the only
// sanctioned way to ask the question this sweep is a fence around.
//
// THE BOUNDARY. A `strings.Contains` against a Makefile RECIPE — the
// tab-indented command lines makeRecipe returns, joined or not — is a
// different and common question ("does this target's command carry this
// flag"), asked five times in this corpus in TestQAMakeTestOpensTheTreeWideDoors
// alone, and it is not this defect: a recipe carries no token discipline to
// defeat, because nothing here ever claims a recipe line is a token list.
// The two are told apart below by PROVENANCE — did the string trace back to
// a `target:` line's PREREQUISITE remainder — not by the presence of
// `strings.Join` or `strings.Contains` alone. That boundary is what the bead
// this pin is for named as the whole design question, and getting it wrong
// in the READER's favor (flagging recipe Contains too) would have made this
// pin unlandable against the corpus as it stands today.
//
// WHAT A GREEN HERE PROVES, exactly. This is a SYNTACTIC, single-function
// dataflow, in the register of absencerules_qa_test.go's censuses: it
// tracks named identifiers through `:=`/`=` and a small allow-list of
// `strings.Trim*`/`strings.CutPrefix`/`strings.Join` wrappers, within ONE
// function body, in source order, with no type checker and no build. It
// does not follow a value through a struct field, a slice index, a closure
// capture, a map, or a second hop of re-tokenising (Fields, then Join
// again) — each is invisible to it, exactly as absencerules_qa_test.go's
// pins are blind past their own named shapes. What bounds the class is that
// every real instance of this defect was a direct local variable a few
// lines from its own `strings.Contains` call, and the corpus this sweeps —
// every repo-root `*_test.go` file — is where mkPrereqs, mkRuns and every
// known call site live; internal/posse test files that parse the Makefile
// independently (a handful do, for unrelated claims) are outside this
// sweep's corpus and are not this bead's finding.
//
// Shown able to fail, and not on the recipe boundary, by
// TestQAMkPrereqBytesSweepCatchesTheBannedShapes below: fixtures for both
// real historical shapes, plus clean controls for the recipe boundary, an
// unrelated Join+Contains, and mkPrereqs/mkRuns' own bodies.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// mkpFile is one parsed repo-root test file.
type mkpFile struct {
	rel  string
	file *ast.File
}

// mkpCorpus parses every repo-root *_test.go file — the tree mkPrereqs,
// mkRuns and every known call site of this defect live in.
func mkpCorpus(t *testing.T) (*token.FileSet, []mkpFile) {
	t.Helper()
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	// Reader sanity: a walk that opened nothing would pass the sweep below
	// on silence. 47 repo-root *_test.go files on 2026-09-06; loose on
	// purpose — this floor catches a broken glob, not a file count.
	if len(paths) < 40 {
		t.Fatalf("only %d repo-root *_test.go files found — the glob is broken, not the tree", len(paths))
	}
	fset := token.NewFileSet()
	out := make([]mkpFile, 0, len(paths))
	for _, p := range paths {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		out = append(out, mkpFile{rel: p, file: f})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return fset, out
}

// mkpSelCall reports whether e is a call `pkg.name(...)`, and returns it.
func mkpSelCall(e ast.Expr, pkg, name string) (*ast.CallExpr, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return nil, false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || x.Name != pkg {
		return nil, false
	}
	return call, true
}

// mkpBareCall reports whether e is a call to the unqualified name(...).
func mkpBareCall(e ast.Expr, name string) (*ast.CallExpr, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok || id.Name != name {
		return nil, false
	}
	return call, true
}

// mkpExprKey renders a target-tail expression to a comparable, deduplicable
// key: a string literal's own value, or `<ident>+<literal>` for a
// `target+":"` build. Two checks that key identically are the same check
// written twice, not two candidate spellings.
func mkpExprKey(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, err := strconv.Unquote(v.Value); err == nil {
				return s
			}
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			if id, ok := v.X.(*ast.Ident); ok {
				return id.Name + "+" + mkpExprKey(v.Y)
			}
		}
	}
	return ""
}

// mkpLineTargetGuard reports whether body treats `line` as found by exactly
// ONE `target:` check: a single strings.HasPrefix or strings.CutPrefix
// tail, anywhere in the loop, ending in ":" — regardless of
// whether the surrounding control flow is a match-branch or a negated
// continue-guard; both have occurred and this sweep does not need to
// resolve which arm the guard steers.
//
// More than one DISTINCT tail checked against the same line — GOFMT's three
// candidate spellings of a variable's definition line in
// TestQAMakeTestOpensTheGofmtDoor ("GOFMT ", "GOFMT:", "GOFMT=") — is
// hunting for a variable, not a rule's prerequisite line, and is out of
// this sweep's scope: Make gives a rule exactly one spelling, `name:`,
// which is why every real instance of this defect (three beads, four call
// sites the first time, one this bead) tested exactly one tail.
//
// RESIDUAL, said out loud: a hand-rolled reimplementation that also excludes
// `target:=` the way mkPrereqs's own guard does — two checks, not one —
// reads as a variable hunt under this rule and is invisible to it. This
// boundary is drawn from the shapes that have actually occurred, not a
// proof the class stops there.
func mkpLineTargetGuard(body ast.Node, line string) bool {
	seen := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, name := range []string{"HasPrefix", "CutPrefix"} {
			c, ok := mkpSelCall(call, "strings", name)
			if !ok || len(c.Args) < 2 {
				continue
			}
			if id, ok := c.Args[0].(*ast.Ident); ok && id.Name == line {
				seen[mkpExprKey(c.Args[1])] = true
			}
		}
		return true
	})
	if len(seen) != 1 {
		return false
	}
	for k := range seen {
		return strings.HasSuffix(k, ":")
	}
	return false
}

// mkpWrapsTainted reports whether e is one of the strings.Trim* calls this
// corpus uses to peel a found line down to its remainder, applied to an
// already-tainted expression.
func mkpWrapsTainted(e ast.Expr, tainted func(ast.Expr) bool) bool {
	for _, name := range []string{"TrimSpace", "TrimPrefix", "TrimSuffix", "TrimLeft", "TrimRight", "Trim"} {
		if c, ok := mkpSelCall(e, "strings", name); ok && len(c.Args) >= 1 && tainted(c.Args[0]) {
			return true
		}
	}
	return false
}

// mkpJoinsTokens reports whether e is `strings.Join(x, sep)` where x is a
// known prerequisite-TOKEN slice — the shape that undoes mkPrereqs's own
// tokenisation one call site later (ranger-base-7kwb8).
func mkpJoinsTokens(e ast.Expr, tokens map[string]bool) bool {
	c, ok := mkpSelCall(e, "strings", "Join")
	if !ok || len(c.Args) < 1 {
		return false
	}
	id, ok := c.Args[0].(*ast.Ident)
	return ok && tokens[id.Name]
}

// mkpScanFunc walks one function body and returns the position of every
// strings.Contains call whose haystack is, by the dataflow this file's doc
// comment describes, an untokenised Makefile prerequisite-line string.
// mkPrereqs's own body is exempt — it IS the sanctioned reader, and this
// sweep is a fence around call sites, not a claim about its internals.
func mkpScanFunc(fn *ast.FuncDecl) []token.Pos {
	if fn.Body == nil || fn.Name.Name == "mkPrereqs" {
		return nil
	}
	line := map[string]bool{}      // holds an untokenised prereq-line STRING
	tokens := map[string]bool{}    // holds prereq TOKENS ([]string): safe until Joined
	splitVars := map[string]bool{} // holds strings.Split(x, "\n") of a would-be Makefile text
	var bad []token.Pos

	tainted := func(e ast.Expr) bool {
		id, ok := e.(*ast.Ident)
		return ok && line[id.Name]
	}
	taintedRHS := func(e ast.Expr) bool {
		return tainted(e) || mkpWrapsTainted(e, tainted) || mkpJoinsTokens(e, tokens)
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.RangeStmt:
			lineID, ok := node.Value.(*ast.Ident)
			if !ok || lineID.Name == "_" {
				break
			}
			isSplit := false
			if _, ok := mkpSelCall(node.X, "strings", "Split"); ok {
				isSplit = true
			} else if id, ok := node.X.(*ast.Ident); ok && splitVars[id.Name] {
				isSplit = true
			}
			if isSplit && mkpLineTargetGuard(node.Body, lineID.Name) {
				line[lineID.Name] = true
			}
		case *ast.AssignStmt:
			if len(node.Rhs) == 1 && len(node.Lhs) == 2 {
				if _, ok := mkpBareCall(node.Rhs[0], "mkPrereqs"); ok {
					if id, ok := node.Lhs[0].(*ast.Ident); ok && id.Name != "_" {
						line[id.Name] = true
					}
					if id, ok := node.Lhs[1].(*ast.Ident); ok && id.Name != "_" {
						tokens[id.Name] = true
					}
				}
				if c, ok := mkpSelCall(node.Rhs[0], "strings", "CutPrefix"); ok && len(c.Args) == 2 && tainted(c.Args[0]) {
					if id, ok := node.Lhs[0].(*ast.Ident); ok && id.Name != "_" {
						line[id.Name] = true
					}
				}
			}
			if len(node.Rhs) != len(node.Lhs) {
				break
			}
			for i, rhs := range node.Rhs {
				id, ok := node.Lhs[i].(*ast.Ident)
				if !ok || id.Name == "_" {
					continue
				}
				if c, ok := mkpSelCall(rhs, "strings", "Split"); ok && len(c.Args) == 2 {
					if lit, ok := c.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if v, err := strconv.Unquote(lit.Value); err == nil && v == "\n" {
							splitVars[id.Name] = true
						}
					}
				}
				if taintedRHS(rhs) {
					line[id.Name] = true
				}
			}
		case *ast.CallExpr:
			if c, ok := mkpSelCall(node, "strings", "Contains"); ok && len(c.Args) == 2 && taintedRHS(c.Args[0]) {
				bad = append(bad, node.Pos())
			}
		}
		return true
	})
	return bad
}

// TestQANoMakefilePrereqLineReadAsBytesOutsideMkPrereqs is the sweep itself.
func TestQANoMakefilePrereqLineReadAsBytesOutsideMkPrereqs(t *testing.T) {
	fset, files := mkpCorpus(t)
	var bad []string
	for _, f := range files {
		for _, decl := range f.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			for _, pos := range mkpScanFunc(fn) {
				bad = append(bad, f.rel+":"+strconv.Itoa(fset.Position(pos).Line)+" in "+fn.Name.Name+"()")
			}
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("strings.Contains against an untokenised Makefile prerequisite line, outside mkPrereqs — read it through mkPrereqs/mkRuns instead (ranger-base-exv9h):\n  %s",
			strings.Join(bad, "\n  "))
	}
}

// mkpScanSrc parses src as a standalone file and returns the violations
// mkpScanFunc finds in it. Purely syntactic, like the sweep itself: src need
// not import or declare everything it references.
func mkpScanSrc(t *testing.T, src string) int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, 0)
	if err != nil {
		t.Fatalf("parse fixture: %v\n%s", err, src)
	}
	n := 0
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			n += len(mkpScanFunc(fn))
		}
	}
	return n
}

// TestQAMkPrereqBytesSweepCatchesTheBannedShapes shows the sweep able to
// fail on both real historical shapes, and clean on the boundary the bead
// named as the false-positive risk: a recipe Contains, an unrelated
// Join+Contains, and mkRuns' own membership test.
func TestQAMkPrereqBytesSweepCatchesTheBannedShapes(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want int
	}{
		{
			name: "raw line kept and Contains'd — boxcheck/gofmtdoor/suitelock before hna69",
			src: `package p
import "strings"
func f(mk string) {
	var deps string
	for _, line := range strings.Split(mk, "\n") {
		if strings.HasPrefix(line, "test:") {
			deps = line
			break
		}
	}
	_ = strings.Contains(deps, "fmt-check")
}
`,
			want: 1,
		},
		{
			name: "CutPrefix remainder trimmed and Contains'd — armTarget before this bead",
			src: `package p
import "strings"
func f(mk, name string) {
	lines := strings.Split(mk, "\n")
	for _, line := range lines {
		s, ok := strings.CutPrefix(line, name+":")
		if !ok {
			continue
		}
		deps := strings.TrimSpace(s)
		_ = strings.Contains(deps, "verify-parallel")
	}
}
`,
			want: 1,
		},
		{
			name: "mkPrereqs tokens rejoined and Contains'd — treewidedoor before 7kwb8",
			src: `package p
import "strings"
func f(t *testing.T, mk string) {
	_, tree := mkPrereqs(t, mk, "tree-check")
	joined := strings.Join(tree, " ")
	_ = strings.Contains(joined, "fmt-check")
}
`,
			want: 1,
		},
		{
			name: "the mkPrereqs line itself Contains'd, no intermediate copy",
			src: `package p
import "strings"
func f(t *testing.T, mk string) {
	treeLine, _ := mkPrereqs(t, mk, "tree-check")
	_ = strings.Contains(treeLine, "fmt-check")
}
`,
			want: 1,
		},
		{
			name: "control: mkRuns is the sanctioned membership test",
			src: `package p
func f(t *testing.T, mk string) {
	_, tree := mkPrereqs(t, mk, "tree-check")
	_ = mkRuns(tree, "fmt-check")
}
`,
			want: 0,
		},
		{
			name: "control: a recipe Contains is not this defect",
			src: `package p
import "strings"
func f(mk, target string) {
	recipe := strings.Join(makeRecipe(mk, target), "\n")
	_ = strings.Contains(recipe, "-count=1")
}
`,
			want: 0,
		},
		{
			name: "control: an unrelated Join+Contains",
			src: `package p
import "strings"
func f(got []string, want string) {
	_ = strings.Contains(strings.Join(got, "\n"), want)
}
`,
			want: 0,
		},
		{
			name: "control: hunting a variable's definition by several candidate spellings is not a rule's prerequisite line",
			src: `package p
import "strings"
func f(mk string) {
	var gofmtVar string
	for _, line := range strings.Split(mk, "\n") {
		if strings.HasPrefix(line, "GOFMT ") || strings.HasPrefix(line, "GOFMT:") || strings.HasPrefix(line, "GOFMT=") {
			gofmtVar = line
			break
		}
	}
	_ = strings.Contains(gofmtVar, "GOROOT")
}
`,
			want: 0,
		},
		{
			name: "control: the line's error-message use is not membership",
			src: `package p
import "strings"
func f(mk string) {
	var deps string
	for _, line := range strings.Split(mk, "\n") {
		if strings.HasPrefix(line, "test:") {
			deps = line
		}
	}
	_ = len(deps)
}
`,
			want: 0,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := mkpScanSrc(t, c.src); got != c.want {
				t.Errorf("mkpScanSrc found %d violation(s), want %d:\n%s", got, c.want, c.src)
			}
		})
	}
}
