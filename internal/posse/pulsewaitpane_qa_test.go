//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0093 D1's load-bearing claim, pinned structurally: the waiter writes
// NOTHING to any pane. "Nothing is written to the pane's input, so a splice
// is impossible by construction, not merely unlikely" — and a claim of
// construction has to be read off the construction, because no behavioural
// test can prove a pane write did not happen on a path it did not take.
//
// ARM 1, unlike the waiter's behavioural pins next door in pulsewait_test.go
// (arm 2): this one is an AST read of one file in this directory, it takes
// milliseconds, and arm 1 is what a bare `go test ./...` builds — a pin
// whose whole job is to red when someone reaches for the keyboard again
// should be in the arm that runs most often.
//
// It reads THIS package's directory and never climbs to the repo root, so it
// is not a tree-wide pin and needs no Makefile door of its own (AGENTS.md,
// "A `-run` filter cannot reach a tree-wide pin"). The cmd/posse half of the
// same claim — that the `pulse` verb's arm never touches the herdr backend —
// is pinned beside main.go in cmd/posse/pulseverb_qa_test.go, for the same
// reason: each half is read in the package that holds it.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// pwPaneWriters are the identifiers that reach a pane, or reach the thing
// that reaches one. Chosen because none of them has an innocent use in a
// file whose whole job is one lock and one text file — unlike the mux method
// names this list deliberately omits (`Read`, `Close`, `Wait`, `State`,
// `Start`), each of which is also a perfectly ordinary call on an *os.File
// or a timer. A census that banned those would red on `l.f.Close()` and
// would then be loosened by whoever met it, which is how a pin becomes a
// comment.
var pwPaneWriters = map[string]string{
	"Prompt":          "submits text to the runtime — the splice (ADR 0093 Context)",
	"SendText":        "types into the pane's input",
	"SendKeys":        "presses keys in the pane",
	"AgentPrompt":     "herdr's own prompt verb",
	"AwaitPromptable": "a pane reading, which only a path that is about to type needs",
	"Mux":             "the pane multiplexer itself",
	"HerdrBackend":    "the backend that owns the mux",
	"Dispatcher":      "the watch loop's type — delivery state and the typed path are ITS (ADR 0011/0029)",
	"deliverPulse":    "the typed delivery",
	"pulsePromptText": "the typed pulse's text",
}

// The census.
func TestQAThePulseWaiterReachesNoPane(t *testing.T) {
	t.Parallel()
	const src = "pulsewait.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, src, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", src, err)
	}
	// Comments are excluded from the walk on purpose: this file's header
	// EXPLAINS the splice by naming Prompt, and a census that read prose
	// would make the explanation the violation.
	found := 0
	ast.Inspect(f, func(n ast.Node) bool {
		var name string
		switch id := n.(type) {
		case *ast.Ident:
			name = id.Name
		case *ast.SelectorExpr:
			name = id.Sel.Name
		default:
			return true
		}
		if why, bad := pwPaneWriters[name]; bad {
			found++
			t.Errorf("%s names %s at %s — %s. ADR 0093 D1 is that the waiter writes nothing to a pane, which is what makes the splice impossible by construction rather than unlikely; delivery and the typed fallback are the watch's (D3/D4)",
				src, name, fset.Position(n.Pos()), why)
		}
		return true
	})
	if found == 0 && len(pwPaneWriters) == 0 {
		t.Fatal("the forbidden set is empty — this census is checking nothing")
	}
}

// The other direction, and the one that cannot be satisfied by renaming:
// PulseWait's signature gives it no way to REACH a pane. A context, the app,
// its options and one io.Writer — no backend, no mux, no seat. A pane write
// added tomorrow would have to add a parameter or a package-level backend to
// get there, and this reds on the parameter.
func TestQAPulseWaitTakesNoBackend(t *testing.T) {
	t.Parallel()
	const src = "pulsewait.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", src, err)
	}
	want := []string{"context.Context", "*App", "PulseWaitOptions", "io.Writer"}
	var got []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "PulseWait" || fn.Recv != nil {
			continue
		}
		for _, p := range fn.Type.Params.List {
			var b strings.Builder
			if err := printType(&b, p.Type); err != nil {
				t.Fatal(err)
			}
			for range max(1, len(p.Names)) {
				got = append(got, b.String())
			}
		}
	}
	if len(got) == 0 {
		t.Fatal("pulsewait.go declares no PulseWait — this pin is reading the wrong file or the verb has been renamed")
	}
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("PulseWait takes (%s), want (%s) — a waiter that can reach a pane is one that can splice, and ADR 0093 D1's guarantee is that it cannot",
			strings.Join(got, ", "), strings.Join(want, ", "))
	}
}

// printType renders a parameter's type the way the want list above spells
// it. Only the three forms that list uses, and a Fatal for anything else:
// a pin that silently rendered an unknown type as "" would read a new
// parameter as no parameter.
func printType(b *strings.Builder, e ast.Expr) error {
	switch t := e.(type) {
	case *ast.Ident:
		b.WriteString(t.Name)
	case *ast.StarExpr:
		b.WriteString("*")
		return printType(b, t.X)
	case *ast.SelectorExpr:
		if err := printType(b, t.X); err != nil {
			return err
		}
		b.WriteString(".")
		b.WriteString(t.Sel.Name)
	default:
		return Die("pulsewait.go: PulseWait has a parameter of an unrendered type %T — widen printType rather than letting it read as absent", e)
	}
	return nil
}
