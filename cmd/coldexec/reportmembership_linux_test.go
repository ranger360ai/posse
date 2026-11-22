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
// WHAT IT READS, and therefore what it does NOT. A function that hands
// `exec.Command` the built door's path starts a supervised run, so it either
// calls the assertion itself — once per run it starts — or goes in the table
// below with a reason. The path is read two ways: the direct
// `exec.Command(builtDoor(t), …)`, and a local whose one assignment is that
// call (`bin := builtDoor(t)`, then `exec.Command(bin, …)`). Source and not
// behaviour because there is no observable — an unasserted run is a run that
// passes.
//
// Both of those cost a finding (ranger-base-m0ajs, from the verify of the
// close that wrote this census). As it first shipped it read the direct call
// only and answered ONE bool for the whole body, so two spellings of an
// unasserted run were green against a comment claiming it saw them all:
// `bin := builtDoor(t)` with the exec one line down — which ARM 8 already
// writes, because a function needing the path twice naturally does — and a
// function that asserts run A and then starts run B, which is the original
// defect one function over. MEASURED 2026-11-22, each probe planted alone in
// this package and graded by this test: both PASS before, both RED after,
// with the direct-and-unasserted control red throughout.
//
// What is still out of reach, and is the register for it:
//
//   - ARM 8's INNER door. It is a string inside a supervised script, so no
//     `exec.Command` in this package's source names it, and that run is a
//     CHILD of a run runDoor already holds.
//   - a path this file cannot resolve in source: a struct field, a function
//     parameter, a local assigned more than once, or any other door started
//     by a spelling of `os/exec` that is not `exec.Command` (there is none in
//     this package — MEASURED 2026-11-22, no `exec.CommandContext` and no
//     `exec.Cmd` literal in its `*_test.go`).
//   - runs PAIRED to assertions. The two are counted, not matched, so a
//     function asserting one run twice while starting a second covers the
//     count. Every site in this package today is one run per function or a
//     helper whose single run is asserted in its own body, so the counting
//     reading is the whole of what the sites need; a function that really has
//     two runs to assert should assert each where it starts it.
//
// It names its own package directory and no other, and `go test` runs a test
// binary with its package dir as the working directory, so the file set is
// this package's `*_test.go` and the parse is of files beside this one.

import (
	"fmt"
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
			starts, asserts := rmScan(fd.Body)
			if len(starts) == 0 {
				continue
			}
			sites += len(starts)
			seen[fd.Name.Name] = true
			reason, exempt := rmExempt[fd.Name.Name]
			switch {
			case asserts > 0 && exempt:
				t.Errorf("%s calls %s and is also in rmExempt (%q). Take it out of the table: an "+
					"exemption that is not exercised is a hole this pin reports as covered.",
					fd.Name.Name, rmAssertion, reason)
			case asserts >= len(starts):
			case exempt:
				if strings.TrimSpace(reason) == "" {
					t.Errorf("%s is in rmExempt with no reason. The reason is the whole entry — a "+
						"name on its own is a run nobody decided about.", fd.Name.Name)
				}
			default:
				t.Errorf("%s\n"+
					"Every supervised run that reaches the filter prints a report whose nine verdict "+
					"counts must sum to its notification count, and that number is what a seat "+
					"reconciles by hand. Three runs sat outside this assertion while its own comment "+
					"said it was held on every run (ranger-base-l9pk7), and a door spending two "+
					"verdicts per notification passed all three. Call %s(t, <the output>) for each "+
					"run, or — if a run prints no report at all — put it in rmExempt with the reason "+
					"why.", rmShortfall(fset, fd.Name.Name, starts, asserts), rmAssertion)
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

// THE CENSUS'S OWN REACH, which the census cannot witness (ranger-base-m0ajs).
// Every red above is reached through rmScan, and over this package's real
// sources rmScan answers the same thing whether or not it reads a bound door
// path or counts runs — every live site is one direct run per function. So
// the two holes the verify of ranger-base-l9pk7 measured were green against a
// green census, and would be again the moment either reading is simplified
// away.
//
// These rows are the shapes, parsed and scanned directly: the four the verify
// graded by planting a probe file (the first two of them PASSED before this
// landed), and three negatives that say the widened reach does not report a
// run that is not there. A table, because the shape is the subject — the
// snippets are literal for the reason the planted probes were literal.
func TestQATheCensusReadsABoundDoorPathAndCountsEveryRun(t *testing.T) {
	t.Parallel()

	const (
		direct  = `out, _ := exec.Command(builtDoor(t), "--", "/bin/true").CombinedOutput()` + "\n_ = out\n"
		bound   = "bin := builtDoor(t)\n" + `out, _ := exec.Command(bin, "--", "/bin/true").CombinedOutput()` + "\n_ = out\n"
		assert  = "wantReportAccountsForEveryNotification(t, \"\")\n"
		nondoor = `out, _ := exec.Command("/bin/sh", "-c", bin).CombinedOutput()` + "\n_ = out\n"
	)
	for _, tc := range []struct {
		name        string
		body        string
		runs        int
		asserts     int
		wantRedline bool
	}{
		{"the direct call, unasserted", direct, 1, 0, true},
		{"the door path bound to a local first", bound, 1, 0, true},
		{"one run, asserted", direct + assert, 1, 1, false},
		{"a bound run, asserted", bound + assert, 1, 1, false},
		{"run A asserted, then run B unasserted", direct + assert + direct, 2, 1, true},
		{"two runs, neither asserted", direct + direct, 2, 0, true},
		{"two runs, both asserted", direct + assert + direct + assert, 2, 2, false},
		// ARM 8's shape: the door's path goes into a string the script runs,
		// so this package's source holds no exec.Command of it. Registered as
		// out of reach in the head comment, and this is the row that says the
		// bound-local reading did not quietly bring it in.
		{"a bound path that is never exec'd", "bin := builtDoor(t)\n" + nondoor, 0, 0, false},
		// A name assigned twice is a path this file cannot resolve, so it is
		// not a site. The alternative — resolving the first assignment and
		// reporting the call — is a red for a run that never happened.
		{"a local reassigned to something else", "bin := builtDoor(t)\nbin = \"/bin/echo\"\n" +
			`out, _ := exec.Command(bin, "hi").CombinedOutput()` + "\n_ = out\n", 0, 0, false},
		{"a run inside a subtest closure", "t.Run(\"x\", func(t *testing.T) {\n" + direct + "})\n", 1, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := "package main\nfunc f(t *testing.T) {\n" + tc.body + "}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "snippet.go", src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parsing the snippet: %v\n%s", err, src)
			}
			starts, asserts := rmScan(file.Decls[0].(*ast.FuncDecl).Body)

			if len(starts) != tc.runs || asserts != tc.asserts {
				t.Fatalf("rmScan read %d supervised runs and %d assertions, want %d and %d — the "+
					"census's reach moved, and every red it files is reached through this:\n%s",
					len(starts), asserts, tc.runs, tc.asserts, src)
			}
			// What the census does with those two numbers, which is the half
			// that decides whether anything is reported at all.
			red := len(starts) > 0 && asserts < len(starts)
			if red != tc.wantRedline {
				t.Errorf("this body is reported = %v, want %v (%d runs, %d assertions):\n%s",
					red, tc.wantRedline, len(starts), asserts, src)
			}
			if red {
				if got := rmShortfall(fset, "f", starts, asserts); !strings.Contains(got, rmAssertion) ||
					!strings.Contains(got, "snippet.go:") {
					t.Errorf("the red names neither the assertion nor a position, so a seat meeting "+
						"it has nothing to go to: %q", got)
				}
			}
		})
	}
}

// rmScan answers EVERY exec.Command whose first argument is the built door's
// path in a function body, and how many times that body calls the assertion.
// Both are counted rather than answered as one bool, because the shape this
// pin exists to catch is a run nobody asserted and a function can hold more
// than one run (ranger-base-m0ajs: "run A asserted, then run B unasserted"
// was green while one bool stood for the whole body).
//
// A nested function literal is part of the body it is written in, which is
// what `ast.Inspect` gives and what this pin wants: a run started inside a
// t.Run closure is started by the function the closure is in.
func rmScan(body *ast.BlockStmt) (starts []ast.Node, asserts int) {
	doors := rmDoorLocals(body)
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == rmAssertion {
			asserts++
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Command" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "exec" {
			return true
		}
		if len(call.Args) == 0 || !rmIsDoorPath(call.Args[0], doors) {
			return true
		}
		starts = append(starts, call)
		return true
	})
	return starts, asserts
}

// rmIsDoorPath reads the first argument of an exec.Command: the door's path
// is either the starter call itself or a local rmDoorLocals resolved to one.
func rmIsDoorPath(arg ast.Expr, doors map[string]bool) bool {
	if id, ok := arg.(*ast.Ident); ok {
		return doors[id.Name]
	}
	return rmIsStarterCall(arg)
}

// rmIsStarterCall is `builtDoor(…)` and nothing else.
func rmIsStarterCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == rmStarter
}

// rmDoorLocals names the locals in a body that hold the built door's path:
// assigned exactly once, by a call to rmStarter, and never assigned anything
// else. A name with a second assignment is left OUT rather than guessed at —
// this pin reads source, so a path decided at run time is out of its reach by
// construction, and a name it resolved wrongly would report a run that is not
// there. A declaration with no value (`var bin string`) is not an assignment
// and does not disqualify the name.
func rmDoorLocals(body *ast.BlockStmt) map[string]bool {
	door, other := map[string]int{}, map[string]int{}
	note := func(lhs, rhs []ast.Expr) {
		if len(rhs) == 0 {
			return
		}
		for i, l := range lhs {
			id, ok := l.(*ast.Ident)
			if !ok || id.Name == "_" {
				continue
			}
			// `a, b := f()` pairs no expression with either name, so
			// neither is resolvable and both are counted as assigned
			// otherwise.
			if len(lhs) != len(rhs) || !rmIsStarterCall(rhs[i]) {
				other[id.Name]++
				continue
			}
			door[id.Name]++
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			note(s.Lhs, s.Rhs)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, 0, len(s.Names))
			for _, nm := range s.Names {
				lhs = append(lhs, nm)
			}
			note(lhs, s.Values)
		}
		return true
	})
	out := map[string]bool{}
	for name, n := range door {
		if n == 1 && other[name] == 0 {
			out[name] = true
		}
	}
	return out
}

// rmShortfall is the first sentence of the red: what the function starts and
// what it holds. The one-run case keeps the sentence this pin shipped with,
// because that is the red a seat writing a fourth supervised run meets.
func rmShortfall(fset *token.FileSet, name string, starts []ast.Node, asserts int) string {
	where := make([]string, 0, len(starts))
	for _, s := range starts {
		where = append(where, fset.Position(s.Pos()).String())
	}
	if len(starts) == 1 && asserts == 0 {
		return fmt.Sprintf("%s starts a supervised run at %s and never calls %s.",
			name, where[0], rmAssertion)
	}
	return fmt.Sprintf("%s starts %d supervised runs (%s) and calls %s %d time(s), so at least "+
		"one of those runs is asserted by nobody.",
		name, len(starts), strings.Join(where, ", "), rmAssertion, asserts)
}
