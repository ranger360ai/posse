package posse

// ranger-base-fsil: two guards on the settle-hint budget, both source-level
// on purpose. What they want is "this file never waits on a bare number
// again", and a test cannot observe that by waiting — it would have to
// reproduce the flake to fail, which is the thing being fixed.
//
// The flake they close out: TestWatchSettleHintWakesTheNextPassEarly went
// red about one full `go test ./internal/rhq` run in three, always at
// 5.0x seconds. Measured on this box (1600 instrumented waits under load
// 62-89), the wait distribution is not a tail — it is bimodal, with nothing
// at all between 324ms and 5097ms, and everything above 5s is the subscribe
// handshake in the one test that goes through the production adapter. That
// gap is herdrHintRetry: when the loop's first subscribe has to be redialled
// the second attempt arrives one whole retry delay later, and the old 5s
// budget was that delay to the millisecond. A budget equal to a retry the
// code under test may legitimately spend is a coin flip, however patient it
// looks.
//
// So the first guard is a ratio, not a number: whatever the retry becomes,
// the budget has to clear it several times over.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestQAHintBudgetClearsTheAdapterRetry(t *testing.T) {
	t.Parallel()
	if hintWait < 5*herdrHintRetry {
		t.Errorf("hintWait is %s and the adapter redials after %s: a test budget "+
			"under 5x the retry it may have to wait out is the ranger-base-fsil "+
			"flake, which failed at exactly one retry delay", hintWait, herdrHintRetry)
	}
	// And the retry cannot be raised into the budget from the other side.
	if herdrHintRetry > 10*time.Second {
		t.Errorf("herdrHintRetry is %s: raising it moves the same race back under "+
			"hintWait (%s) — raise both or neither", herdrHintRetry, hintWait)
	}
}

// Every wait in herdrevents_test.go spends the one named budget. A literal
// here is how the flake got in: four separate 5s deadlines, none of them
// reading as a decision.
//
// One exemption, added with ranger-base-7hjy4 and deliberately narrow.
// TestHerdrHintsRedialFloorBoundsAStorm does not wait for anything: it runs
// unbroken churn for a fixed stretch and COUNTS the dials the adapter gets
// out in it, so its deadline is the instrument, not patience, and spending
// hintWait on it would make the test a minute long and measure nothing new.
// The exemption is by name and the name is checked below — `stormWindow`
// must be declared under a second WHEREVER package posse binds it, because
// that is the scope the name resolves in, so nobody can grow it back into the
// patience budget this guard exists to forbid.
func TestQAHintWaitsUseTheNamedBudget(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(herdrEventsFile)
	if err != nil {
		t.Fatal(err)
	}
	// The three shapes a wait takes in that file. `within` is recvHint's own
	// parameter, which its callers fill with hintWait.
	budgets := []*regexp.Regexp{
		regexp.MustCompile(`time\.After\(([^)]*)\)`),
		regexp.MustCompile(`time\.Now\(\)\.Add\(([^)]*)\)`),
		// `recvHint(t, ` and not `recvHint(t *testing.T, `: the call sites,
		// not the declaration the third shape is named after.
		regexp.MustCompile(`recvHint\(t,\s*[^,]+,\s*([^)]*)\)`),
	}
	found := 0
	for _, re := range budgets {
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			found++
			switch arg := strings.TrimSpace(m[1]); arg {
			case "hintWait", "within":
			case "stormWindow": // a measurement window, not a budget; bounded below
			default:
				t.Errorf("a wait spends %q instead of hintWait: %s", arg, strings.TrimSpace(m[0]))
			}
		}
	}
	// Without this the guard passes a file that has no waits left in it at
	// all — renamed, rewritten, or deleted — which is the one way it could
	// go green while measuring nothing.
	if found < 12 {
		t.Errorf("only %d waits found in herdrevents_test.go; this guard has lost "+
			"its subject and needs to follow it", found)
	}

	// The exemption's own fence. `stormWindow` is allowed above only as a
	// stretch of churn to count dials in; a patience budget wearing the name
	// would be the exact defect this file was written for, so it has to
	// declare itself and it has to be short.
	//
	// FindAll and not FindString (ranger-base-43ux4, escaped from
	// ranger-base-0b0qg): the exemption above admits EVERY wait spelled
	// `stormWindow` anywhere in the file, so a fence that measures only the
	// first declaration leaves the second one exempt and unmeasured — a
	// thirty-second wait wearing the exempt name, green. The fence has to
	// cover the same set the exemption opens.
	//
	// PARSED and not matched (ranger-base-zt61m, escaped from
	// ranger-base-43ux4): FindAll reached every declaration written in ONE
	// spelling — `stormWindow = <int> * time.(Millisecond|Second|Minute)` —
	// while the exemption keys on the NAME. Three ordinary Go spellings sat
	// outside that pattern and inside the exemption, each measured green at
	// 63d44db: `stormWindow := 30 * time.Second` (the idiomatic spelling for
	// a duration local, and the one the pattern cannot reach at all), `const
	// stormWindow = 1 * time.Hour`, `const stormWindow = time.Minute`.
	// Keeping a pattern in step with a name by hand is two rules that have to
	// agree; reading the value bound to the identifier is one rule, and it is
	// the exemption's own.
	//
	// THE WHOLE PACKAGE and not this one file (ranger-base-eaq7n, escaped
	// from ranger-base-zt61m via ranger-base-f34bo): the parse read
	// herdrevents_test.go alone, and the exemption keys on an identifier,
	// which Go resolves at PACKAGE scope. A `const stormWindow = 1 *
	// time.Hour` in any sibling file of package posse is what the waits above
	// then spend, and it was exempt and unmeasured — measured green at
	// 65bcaad by putting exactly that in a sibling test file, against a
	// wrong arm (`longStorm`, the same hour under another name) that failed
	// as it should. It fails CLOSED only when the binding leaves the package
	// ENTIRELY, because then nothing compiles.
	//
	// So the name is reserved PACKAGE-WIDE, which is deliberately wider than
	// the exemption: a `stormWindow` local to some other test file is not
	// what herdrevents_test.go's waits resolve to, and it is fenced anyway.
	// The alternative is resolving each wait to its own binding — go/types
	// over the whole test package to hold one identifier — and the cheaper
	// rule is one sentence a reader can hold: in package posse this name
	// means a window a test counts in, and it is under a second. A binding
	// that means something else takes another name.
	fset := token.NewFileSet()
	clause, err := parser.ParseFile(fset, herdrEventsFile, src, parser.PackageClauseOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", herdrEventsFile, err)
	}
	pkgName := clause.Name.Name
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	decls, read := 0, 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		// A file in the external test package (`posse_test`) is a different
		// scope and its bindings are not what these waits resolve to. There
		// are none today; fencing them would be a red nobody could act on.
		if file.Name.Name != pkgName {
			continue
		}
		read++
		decls += fenceStormWindow(t, fset, file)
	}
	// The sibling of the `found < 12` guard above: a census whose file list
	// came out empty is green for the same reason a scanner with no subject
	// is, and this one takes its list from a directory read rather than a
	// literal. The floor is a FLOOR and not a count: package posse is 500
	// files on 2026-09-05, so this only fires on a fence that has stopped
	// walking the package — a cwd that is not the package dir, a filter that
	// stopped matching — and never on ordinary growth or a deletion.
	if read < 100 {
		t.Errorf("the fence read only %d file(s) of package %s; it is meant to hold the "+
			"whole package and has lost its subject", read, pkgName)
	}
	if decls == 0 {
		t.Fatalf("stormWindow is exempted from the named budget but no file of package %s declares it", pkgName)
	}
}

// herdrEventsFile is the file the exemption scan above reads. The FENCE below
// reads the whole package (ranger-base-eaq7n); this name is the scan's
// subject, not the fence's.
const herdrEventsFile = "herdrevents_test.go"

// fenceStormWindow holds every binding of the exempt name in one file under a
// second, and reports how many it found. Every shape Go can bind a name in is
// answered, and the ones whose value is chosen somewhere this fence does not
// read are errors rather than skips — the exemption admits the NAME however
// it is bound, so an unreadable binding is a wait this guard cannot price.
func fenceStormWindow(t *testing.T, fset *token.FileSet, file *ast.File) int {
	t.Helper()
	decls := 0
	ast.Inspect(file, func(n ast.Node) bool {
		switch d := n.(type) {
		case *ast.ValueSpec: // `const stormWindow = ...`, `var stormWindow = ...`
			for i, name := range d.Names {
				if name.Name != "stormWindow" {
					continue
				}
				decls++
				if i >= len(d.Values) {
					t.Errorf("%s: stormWindow is declared with no value of its own (an iota or a grouped spec), "+
						"which this fence cannot read — the exemption admits the name however it is bound",
						stormWindowAt(fset, name.Pos()))
					continue
				}
				stormWindowUnderASecond(t, fset, d.Values[i])
			}
		case *ast.AssignStmt: // `stormWindow := ...`, `stormWindow = ...`
			for i, lhs := range d.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name != "stormWindow" {
					continue
				}
				decls++
				if len(d.Rhs) != len(d.Lhs) {
					t.Errorf("%s: stormWindow is bound by a multi-value assignment, which this fence cannot read — "+
						"the exemption admits the name however it is bound", stormWindowAt(fset, id.Pos()))
					continue
				}
				stormWindowUnderASecond(t, fset, d.Rhs[i])
			}
		case *ast.Field: // a parameter, a result or a struct field
			for _, name := range d.Names {
				if name.Name != "stormWindow" {
					continue
				}
				decls++
				t.Errorf("%s: stormWindow is bound as a parameter or a field, whose value is chosen "+
					"somewhere this fence does not read — the exemption above admits the NAME, so a "+
					"caller could spend a minute through it and nothing here would say so. Take a "+
					"`time.Duration` under another name, or declare the window as a constant",
					stormWindowAt(fset, name.Pos()))
			}
		case *ast.RangeStmt: // `for stormWindow := range ...`
			for _, key := range []ast.Expr{d.Key, d.Value} {
				if id, ok := key.(*ast.Ident); ok && id.Name == "stormWindow" {
					decls++
					t.Errorf("%s: stormWindow is bound by a range, whose value this fence cannot read — "+
						"the exemption admits the name however it is bound", stormWindowAt(fset, id.Pos()))
				}
			}
		}
		return true
	})
	return decls
}

// stormWindowAt names a line of the file under census, the way the sibling QA
// tests in this package do. The file comes off the position and is not a
// literal: the census is the whole package now, so a message naming
// herdrevents_test.go would send a reader to the wrong file
// (ranger-base-eaq7n).
func stormWindowAt(fset *token.FileSet, p token.Pos) string {
	pos := fset.Position(p)
	return fmt.Sprintf("%s:%d", filepath.Base(pos.Filename), pos.Line)
}

// stormWindowUnderASecond holds one binding of the exempt name under a second.
// It fails CLOSED on an expression it cannot read: the exemption admits the
// name whatever it is bound to, so a value this fence cannot evaluate is a
// failure here and not a skip — reading the unreadable as zero is the widening
// the parse replaced the pattern for.
func stormWindowUnderASecond(t *testing.T, fset *token.FileSet, expr ast.Expr) {
	t.Helper()
	got, ok := constDuration(expr)
	if !ok {
		t.Errorf("%s: stormWindow is bound to `%s`, which this fence cannot read as a duration constant — "+
			"spell it as one (an integer, a `time.<Unit>`, or a product of those) or take the name out of "+
			"the exemption above", stormWindowAt(fset, expr.Pos()), types.ExprString(expr))
		return
	}
	// `>=` and not `>`: the doc comment above and this message both say
	// "under a second" / "a second or longer", and an exemption fence that
	// admits the first value it forbids is not a fence (ranger-base-0b0qg).
	if got >= time.Second {
		t.Errorf("%s: stormWindow is declared %s (`%s`): the exemption is for a window a test COUNTS in, "+
			"and anything a second or longer is patience wearing its name — spend hintWait",
			stormWindowAt(fset, expr.Pos()), got, types.ExprString(expr))
	}
}

// timeDurationUnits is every unit `time` names, and not only the three
// herdrevents_test.go happens to use today: the fence covers the set the
// exemption opens, and the exemption opens the name.
var timeDurationUnits = map[string]time.Duration{
	"Nanosecond":  time.Nanosecond,
	"Microsecond": time.Microsecond,
	"Millisecond": time.Millisecond,
	"Second":      time.Second,
	"Minute":      time.Minute,
	"Hour":        time.Hour,
}

// constDuration evaluates the shapes a duration constant is written in: an
// integer literal, a `time.<Unit>`, `time.Duration(x)`, parentheses, and
// arithmetic over those. Anything else — an identifier from elsewhere in the
// file, a function call — answers false, and the caller reports it.
func constDuration(expr ast.Expr) (time.Duration, bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return constDuration(e.X)
	case *ast.BasicLit:
		if e.Kind != token.INT {
			return 0, false
		}
		n, err := strconv.ParseInt(e.Value, 0, 64)
		if err != nil {
			return 0, false
		}
		return time.Duration(n), true
	case *ast.SelectorExpr: // `time.Second`
		pkg, ok := e.X.(*ast.Ident)
		if !ok || pkg.Name != "time" {
			return 0, false
		}
		d, ok := timeDurationUnits[e.Sel.Name]
		return d, ok
	case *ast.CallExpr: // `time.Duration(x)`
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok || len(e.Args) != 1 {
			return 0, false
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "time" || sel.Sel.Name != "Duration" {
			return 0, false
		}
		return constDuration(e.Args[0])
	case *ast.BinaryExpr:
		x, okx := constDuration(e.X)
		y, oky := constDuration(e.Y)
		if !okx || !oky {
			return 0, false
		}
		switch e.Op {
		case token.MUL:
			return x * y, true
		case token.ADD:
			return x + y, true
		case token.SUB:
			return x - y, true
		case token.QUO:
			if y == 0 {
				return 0, false
			}
			return x / y, true
		}
	}
	return 0, false
}

// The redial floor's ceiling, the herdrHintRetry check above in the shape it
// already uses: a constant in herdrevents.go whose bound ADR 0016 states as a
// number gets that bound pinned here, because nothing else reads the SHIPPED
// value — both of ranger-base-7hjy4's new pins take the floor as a parameter,
// so the constant itself shipped unmeasured (ranger-base-0b0qg, from
// ranger-base-8ouj8). Measured before this pin existed: the constant at 3s and
// at 10s ran `go test ./internal/posse -run "Herdr|Hint|Budget|Watch"` green
// both times.
//
// The bound is the ADR's and not a fresh number: §1 prices the floor's cost as
// "a pane that appears and settles inside the wait is missed by the stream and
// swept by the timer", and bounds it above by that sweep — the cockpit's
// two-second completeness tick (cmd/posse/cockpit.go, `time.NewTicker(2 *
// time.Second)`), "so the floor never outlives the timer that covers it". A
// literal here and not a reference: the tick is cmd/posse's, this is
// internal/posse, and the sibling check spells its bound the same way. The
// other end of the copy is read by cmd/posse's
// TestCockpitCompletenessTickIsTheSweepTheRedialFloorIsBoundedBy — without
// it the tick could drop to a second and this pin would stay green over a
// floor that equals the sweep, which is exactly what its message forbids
// (ranger-base-43ux4).
//
// `>=` because a floor equal to the sweep does not sit under it — the two land
// in the same instant and which goes first is scheduling, which is the
// stormWindow fence's defect one guard up.
//
// This is the CEILING only. §1's lower edge is the dial's own cost ("anything
// under ~33 ms would be decorative"), stated as an approximation rather than a
// bound, and one second's place inside the band is ASSUMED by the ADR in as
// many words — so a bead that moves the floor within the band moves it without
// touching this test, and only a floor that outlives its sweep reds.
func TestQAHerdrRedialFloorStaysUnderItsSweep(t *testing.T) {
	t.Parallel()
	const cockpitCompletenessTick = 2 * time.Second
	if herdrRedialFloor >= cockpitCompletenessTick {
		t.Errorf("herdrRedialFloor is %s and the cockpit's completeness tick is %s: "+
			"ADR 0016 §1 bounds the floor above by the sweep that covers the pane it "+
			"delays, and a floor at or past that tick outlives it — move the tick with "+
			"it, or bring the measurement the ADR asks for", herdrRedialFloor, cockpitCompletenessTick)
	}
}
