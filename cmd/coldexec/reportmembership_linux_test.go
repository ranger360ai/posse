//go:build linux

package main

// QA, ranger-base-l9pk7 — THE MEMBERSHIP, not the invariant.
//
// wantReportAccountsForEveryNotification's own comment says it is "held on
// every supervised run", and for three runs that was the comment's claim
// alone: ARM 11 (TestTheDelayVariableReachesTheSupervisorAndNothingUnderIt)
// and runRoom's two arms where the door runs to the end each built their own
// exec.Command(builtDoor(t), …) for a real reason and were asserted by
// nobody. Each one prints a reconcilable report, and a door that double-spent
// every notification — `d.record(pending, int(n.Pid), pendingTarget)` written
// twice in answer() — redded 20 tests in this package with 19 distinct
// messages from that assertion while passing all three (MEASURED 2026-11-22,
// this box, linux 7.2.15 x86_64, go1.27.3).
//
// Routing those three fixed the three. This pin is the structural half: a
// fourth supervised run written outside runDoor is red here the moment it is
// written, which is the thing that was missing — nothing held the membership,
// which is how three of them accumulated without a line of output saying so.
//
// WHAT IT READS, and therefore what it does NOT. The shape is the direct one:
// a function that calls `exec.Command(builtDoor(t), …)` is a function that
// starts a supervised run, so it either calls the assertion itself or goes in
// the table below with a reason. A door started any other way is out of
// reach: ARM 8's inner door is a string inside a supervised script, and that
// run is a CHILD of a run runDoor already holds. Source and not behaviour
// because there is no observable — an unasserted run is a run that passes.
//
// It names its own package directory and no other, and `go test` runs a test
// binary with its package dir as the working directory, so the file set is
// this package's `*_test.go` and the parse is of files beside this one.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rmExempt names every function that starts a supervised run and does NOT
// hold the report arithmetic, with the reason it cannot. An entry whose
// function has stopped starting a run, or has started holding the assertion,
// reds as stale: the table is not allowed to outlive its reason.
var rmExempt = map[string]string{
	"TestNoCommandRefuses": "ARM 10 runs the door with no argv at all. It refuses with the usage " +
		"before it installs a filter, so it takes no notification and prints no report — there is " +
		"no arithmetic to read, which is that pin's whole subject.",
}

// rmAssertion is the function every other such site must call, and
// rmStarter is the helper whose call marks a site as a supervised run. Both
// are spelled once: a rename that does not bring this pin along reds at the
// "found no sites" check below rather than quietly censusing nothing.
const (
	rmAssertion = "wantReportAccountsForEveryNotification"
	rmStarter   = "builtDoor"
)

func TestQAEverySupervisedRunHoldsTheReportArithmetic(t *testing.T) {
	t.Parallel()

	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	sites := 0
	seen := map[string]bool{}

	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			starts, held := rmScan(fd.Body)
			if starts == nil {
				continue
			}
			sites++
			seen[fd.Name.Name] = true
			reason, exempt := rmExempt[fd.Name.Name]
			switch {
			case held && exempt:
				t.Errorf("%s calls %s and is also in rmExempt (%q). Take it out of the table: an "+
					"exemption that is not exercised is a hole this pin reports as covered.",
					fd.Name.Name, rmAssertion, reason)
			case held:
			case exempt:
				if strings.TrimSpace(reason) == "" {
					t.Errorf("%s is in rmExempt with no reason. The reason is the whole entry — a "+
						"name on its own is a run nobody decided about.", fd.Name.Name)
				}
			default:
				t.Errorf("%s starts a supervised run at %s and never calls %s.\n"+
					"Every supervised run that reaches the filter prints a report whose nine verdict "+
					"counts must sum to its notification count, and that number is what a seat "+
					"reconciles by hand. Three runs sat outside this assertion while its own comment "+
					"said it was held on every run (ranger-base-l9pk7), and a door spending two "+
					"verdicts per notification passed all three. Call %s(t, <the output>), or — if "+
					"this run prints no report at all — put it in rmExempt with the reason why.",
					fd.Name.Name, fset.Position(starts.Pos()), rmAssertion, rmAssertion)
			}
		}
	}

	// The census reads one source shape, so a rename of that shape would
	// empty it silently and leave every one of the reds above unreachable.
	if sites == 0 {
		dir, _ := os.Getwd()
		t.Fatalf("this census found no call to exec.Command(%s(…), …) in %s's *_test.go, so it is "+
			"asserting nothing. Either the helper that starts a supervised run was renamed — teach "+
			"rmStarter the new name — or the runs moved to a shape this pin cannot see, and the "+
			"membership it holds has to move with them.", rmStarter, filepath.Base(dir))
	}
	for name := range rmExempt {
		if !seen[name] {
			t.Errorf("rmExempt names %s, which no longer starts a supervised run in this package. "+
				"Drop the entry: a stale exemption is an exemption nobody is reading.", name)
		}
	}
}

// rmScan answers the first exec.Command(builtDoor(…), …) in a function body,
// and whether that body also calls the assertion. A nested function literal
// is part of the body it is written in, which is what `ast.Inspect` gives and
// what this pin wants: a run started inside a t.Run closure is started by the
// function the closure is in.
func rmScan(body *ast.BlockStmt) (starts ast.Node, held bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == rmAssertion {
			held = true
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Command" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "exec" {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		inner, ok := call.Args[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := inner.Fun.(*ast.Ident); ok && id.Name == rmStarter && starts == nil {
			starts = call
		}
		return true
	})
	return starts, held
}
