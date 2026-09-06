//go:build posse_arm2

package posse

// QA, ranger-base-xndgk FINDING 2 (verifying ranger-base-eq3ba's close).
//
// mergeblocked_test.go asserts a claim over "every o.Reason spelling in the
// merge path rather than the one a single fixture reaches", and it asserted
// it over a HAND-WRITTEN table of twelve cases with nothing tying the table
// to the code and no floor on its length. The twelve were a complete census
// of the producers on the day they were written — and a thirteenth refusal
// added tomorrow would have got no case, and nothing would have said so.
// For at least one arm the table is the ONLY holder: restore the sentence
// ranger-base-eq3ba removed and drop that one row, and the whole merge suite
// goes green over the defect it was filed for.
//
// That is ranger-base-ik44f's shape one domain over: a class derived by
// hand, with a floor that catches nothing.
//
// So the class is derived from the CODE instead. This file parses
// worktree.go, finds every place a MergeOutcome's Reason is set to something
// non-empty, follows the two indirections the file actually uses (a value
// returned by a helper, assigned through a local), and requires a two-way
// match with the table: every sentence the merge path can produce is driven
// by exactly one case, and every case drives exactly one sentence.
//
// IT FAILS CLOSED. A refusal written in a shape this census cannot follow is
// reported as such rather than counted as zero — a derivation that quietly
// stops matching is exactly the failure this pin exists to prevent
// (ranger-base-sx2dq, one package over, cost eight undoored pins).

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// mergeBlockedCase is one arm of the table in mergeblocked_test.go: a
// fixture that reaches one refusal, and the substring that says which.
type mergeBlockedCase struct {
	name, arm string
	reason    func(*testing.T) string
}

// mrfSource is the file that produces every merge refusal. That it is the
// only one is asserted by TestQAOnlyWorktreeGoWritesAMergeRefusal below.
const mrfSource = "worktree.go"

// mrfSentence is one sentence the merge path can put in MergeOutcome.Reason,
// as its format string reads in the source.
type mrfSentence struct {
	where string
	text  string
}

// mrfLiteral returns what a reason-valued expression SAYS: the format string
// of a fmt.Sprintf, or a plain string literal. ok is false for anything else,
// which the callers report rather than skip.
func mrfLiteral(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		return s, err == nil
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Sprintf" || len(v.Args) == 0 {
			return "", false
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "fmt" {
			return "", false
		}
		b, ok := v.Args[0].(*ast.BasicLit)
		if !ok || b.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(b.Value)
		return s, err == nil
	}
	return "", false
}

// mrfSentences is the census: every sentence worktree.go can leave in
// MergeOutcome.Reason.
//
// Three shapes are followed, and they are the three the file writes:
//
//   - `o.Reason = fmt.Sprintf(...)` — the sentence is right there.
//   - `o.Reason = helper(...)` — a locally declared function's returns.
//   - `if why := helper(...); why != "" { o.Reason = why }` — the same,
//     through a local. The result INDEX is carried across, so
//     `hit, why := constitutionOnBranch(t)` reads that function's second
//     result and not its first.
//
// `o.Reason = ""` is not a refusal: it is the assignment that CLEARS one,
// and MergeOutcome.Blocked's own doc says why both halves are load-bearing.
func mrfSentences(t *testing.T) []mrfSentence {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mrfSource, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", mrfSource, err)
	}
	funcs := map[string]*ast.FuncDecl{}
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
			funcs[fn.Name.Name] = fn
		}
	}
	at := func(p token.Pos) string { return fmt.Sprintf("%s:%d", mrfSource, fset.Position(p).Line) }

	var out []mrfSentence
	// fromFunc collects the sentences a producer returns in result index idx.
	fromFunc := func(name string, idx int, where string) {
		fn := funcs[name]
		if fn == nil {
			t.Errorf("%s sets a merge refusal from %s(), which is not declared in %s — this census reads one file and cannot follow it. Produce the sentence here, or teach this pin where it lives.", where, name, mrfSource)
			return
		}
		found := 0
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok || idx >= len(ret.Results) {
				return true
			}
			text, ok := mrfLiteral(ret.Results[idx])
			if !ok {
				t.Errorf("%s returns a merge refusal this census cannot read (result %d of %s) — it is neither a fmt.Sprintf nor a string literal, so the sentence it produces has no case in mergeBlockedCases and nothing would say so. Write it as one, or teach this pin the shape.", at(ret.Pos()), idx, name)
				return true
			}
			if text == "" {
				return true
			}
			out = append(out, mrfSentence{where: at(ret.Pos()), text: text})
			found++
			return true
		})
		if found == 0 {
			t.Errorf("%s takes its refusal from %s() result %d, and this census found no sentence there — the derivation is reading nothing where the code produces something", where, name, idx)
		}
	}

	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, l := range as.Lhs {
				sel, ok := l.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Reason" {
					continue
				}
				where := at(as.Pos())
				var rhs ast.Expr
				switch {
				case len(as.Rhs) == len(as.Lhs):
					rhs = as.Rhs[i]
				case len(as.Rhs) == 1:
					rhs = as.Rhs[0]
				}
				if rhs == nil {
					t.Errorf("%s assigns a merge refusal in a shape this census cannot follow (%d values into %d targets)", where, len(as.Rhs), len(as.Lhs))
					continue
				}
				if text, ok := mrfLiteral(rhs); ok {
					if text != "" {
						out = append(out, mrfSentence{where: where, text: text})
					}
					continue
				}
				// `o.Reason = helper(...)`.
				if call, ok := rhs.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok {
						fromFunc(id.Name, 0, where)
						continue
					}
				}
				// `why := helper(...)` somewhere above, then `o.Reason = why`.
				id, ok := rhs.(*ast.Ident)
				if !ok {
					t.Errorf("%s assigns a merge refusal from an expression this census cannot follow — it is not a literal, a fmt.Sprintf, a call to a function in this file, or a local bound from one. Every refusal must be reachable from here or it has no case in mergeBlockedCases and nothing says so.", where)
					continue
				}
				bound := false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					bind, ok := n.(*ast.AssignStmt)
					if !ok || len(bind.Rhs) != 1 {
						return true
					}
					call, ok := bind.Rhs[0].(*ast.CallExpr)
					if !ok {
						return true
					}
					callee, ok := call.Fun.(*ast.Ident)
					if !ok {
						return true
					}
					for k, bl := range bind.Lhs {
						if bid, ok := bl.(*ast.Ident); ok && bid.Name == id.Name {
							fromFunc(callee.Name, k, where)
							bound = true
						}
					}
					return true
				})
				if !bound {
					t.Errorf("%s assigns a merge refusal from the local %q, and nothing in %s binds it from a call this census can follow", where, id.Name, fn.Name.Name)
				}
			}
			return true
		})
	}
	// Dedupe by SITE: two assignments both take their sentence from
	// notOnBase, so its two returns are reached twice. One sentence is one
	// sentence however many places embed it — a case drives the wording,
	// not the assignment.
	sort.Slice(out, func(i, j int) bool { return out[i].where < out[j].where })
	var uniq []mrfSentence
	seen := map[string]bool{}
	for _, s := range out {
		if seen[s.where+"\x00"+s.text] {
			continue
		}
		seen[s.where+"\x00"+s.text] = true
		uniq = append(uniq, s)
	}
	return uniq
}

// The floor mergeblocked_test.go never had: every sentence the merge path can
// produce is driven by exactly one case, and every case drives exactly one
// sentence.
//
// Two-way, because the two failures are different and both are silent. A
// sentence with no case is a refusal nobody asserts the claim over — the
// thirteenth-refusal hole. A case with no sentence is an arm whose substring
// has drifted off the code: the fixture still runs, the `arm` check still
// passes on whatever sentence it does reach, and the row measures a
// different refusal than it says it does.
//
// NOT t.Parallel, and for a reason the tool prints rather than one I picked:
// this test reads mergeBlockedCases, whose fixtures reach the package var
// `blindT`, so `go run ./cmd/testparallel internal/posse` calls it UNCLEARED.
// It never RUNS a fixture — it reads the table's `arm` strings — but the
// clearance is over what a test reaches, not what it executes, and widening
// parallelOK for a test that gains nothing from parallelism would be paying
// in the wrong currency. It parses one file and takes 0.00s.
func TestQAEveryMergeRefusalSentenceHasACase(t *testing.T) {
	sentences := mrfSentences(t)
	// A pin over a derived set is satisfied by deriving nothing.
	if len(sentences) < 10 {
		t.Fatalf("only %d merge-refusal sentences derived from %s (12 on 2026-09-04) — the census is reading nothing, and every check below would pass on an empty class:\n%v", len(sentences), mrfSource, sentences)
	}
	cases := mergeBlockedCases()
	if len(cases) < 10 {
		t.Fatalf("mergeBlockedCases holds %d cases (12 on 2026-09-04) — rows have gone, and this pin is the thing that was supposed to notice", len(cases))
	}

	for _, s := range sentences {
		var hit []string
		for _, c := range cases {
			if strings.Contains(s.text, c.arm) {
				hit = append(hit, c.name)
			}
		}
		switch {
		case len(hit) == 0:
			t.Errorf("%s produces a merge refusal no case in mergeBlockedCases drives:\n\t%q\nAdd a case whose fixture reaches it and whose `arm` is a substring only this sentence carries. The claim in TestMergeBlockedReasonsNeverPromiseTheBranch is asserted over the arms that ARE there, so an undriven refusal is one this file promises to check and does not.", s.where, s.text)
		case len(hit) > 1:
			t.Errorf("%s is driven by %d cases (%v) — each `arm` must be a substring only ONE refusal carries, or a fixture that fell through to a neighbouring refusal passes as this one:\n\t%q", s.where, len(hit), hit, s.text)
		}
	}

	for _, c := range cases {
		var hit []string
		for _, s := range sentences {
			if strings.Contains(s.text, c.arm) {
				hit = append(hit, s.where)
			}
		}
		if len(hit) == 0 {
			t.Errorf("the %q case looks for %q, which no refusal in %s contains any more — the sentence was reworded or removed, and this row now measures whatever refusal its fixture happens to reach", c.name, c.arm, mrfSource)
		}
	}
	t.Logf("%d refusal sentences in %s, %d cases", len(sentences), mrfSource, len(cases))
}

// And the census reads the whole class: worktree.go is the only file in this
// package that WRITES a MergeOutcome's Reason. Everything else — dispatch.go's
// noteMergeBlocked, closeddirty.go, herdrback.go — reads the sentence and
// embeds it. A refusal produced anywhere else would be outside the census
// above, which is the ranger-base-sx2dq failure (a derivation keyed on one
// place while the tree carried a second) rather than a hypothetical.
func TestQAOnlyWorktreeGoWritesAMergeRefusal(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("*.go")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no .go files in this package: %v", err)
	}
	fset := token.NewFileSet()
	read := 0
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") || path == mrfSource {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "MergeOutcome") {
			continue
		}
		read++
		file, err := parser.ParseFile(fset, path, b, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, l := range as.Lhs {
				if sel, ok := l.(*ast.SelectorExpr); ok && sel.Sel.Name == "Reason" {
					t.Errorf("%s:%d writes a .Reason in a file that handles MergeOutcome — every merge refusal must be produced in %s, which is the file the census in this pin reads. A sentence written here has no case in mergeBlockedCases and nothing would say so.", path, fset.Position(as.Pos()).Line, mrfSource)
				}
			}
			return true
		})
	}
	if read < 2 {
		t.Fatalf("only %d non-test file besides %s names MergeOutcome — this pin is reading nothing (3 on 2026-09-04: closeddirty.go, dispatch.go, herdrback.go)", read, mrfSource)
	}
}
