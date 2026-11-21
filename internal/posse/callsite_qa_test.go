//go:build !posse_arm2 && !posse_arm3

package posse

// QA, ranger-base-nqw8v — THE CALL SITE IS WHAT A FIX INTO A HELPER LEAVES
// UNWITNESSED, and these two pins are the two places this verify found it.
//
// THE CLASS. A fix whose shape is "move the behaviour into a function the pins
// can call" buys a cheap, fast, hermetic pin on that function — and leaves the
// one line nothing measures: the CALL. Every mutant inside the helper reds;
// deleting the call, or passing it the wrong thing, or making it on a path it
// was deliberately kept off, leaves the whole package green. The tell is a pin
// whose NAME says the caller while its body only exercises the callee.
//
// MEASURED 2026-11-21 (ranger-base-nqw8v, verifying ranger-base-a688i and
// ranger-base-nt5xd), one mutant per pin below, each planted alone with the
// tree asserted clean between arms, run over `-run
// 'TestQAWallWatch|TestLiveWallWatch|TestQAWallVenue'` and `-run
// 'TestUpgrade|TestQAEveryStatusConditionCodeHasAnUpgradeClass'` respectively:
//
//   - the two fatals in wwLiveSeat.launchOn reverted to their PRE-FIX shape —
//     `t.Fatalf("the seat ended before it ran (exit %d):", c)`, byte-for-byte
//     the line ranger-base-a688i was filed on — left arm 1's whole wall-watch
//     family GREEN, TestQAWallWatchFixtureFoldsWhatTheWatcherSaidOntoOneLine
//     included: that pin calls wwOneLine and never launchOn.
//   - `fmt.Fprint(w, upgradeClassNote(before, after))` added to upgradeVerify's
//     ROLLBACK path left the whole upgrade family GREEN, and so did the same
//     mutant on upgradeRenderNote and on upgradeHookWallLine. All three are
//     stand-path-only by a documented decision (a note on a run that is about
//     to be undone describes a binary that will not be there) and nothing read
//     the placement.
//
// WHY SOURCE PINS. Neither call has an observable a behavioural test can
// reach. launchOn's two fatals fire only when a live bwrap seat fails to start
// — the case the member set does not reproduce and ranger-base-a688i could not
// reproduce at -count=100 — and the three report halves print onto a verdict
// path whose rollback arm the unit fixtures reach with no gained row to report.
// So what is read is the one thing that is always there: the call, in the
// function that must make it, on the path that must make it.
//
// EACH PIN NAMES ITS ONE FILE AND ENUMERATES NOTHING, so neither is in
// internal/treepins' one-package-census class and neither needs a Makefile
// door (treewidedoor_qa_test.go's derived key: discovered file sets need a
// door, named ones do not). Same shape and same reason as
// TestQAWallUnixStepLocksItsOSThreadFirstAndNeverUnlocks.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// qcsFuncDecl is the one named function of one named file, parsed. A rename has
// to bring the pin along, which is the point: a pin that went looking for its
// subject could find a different one.
func qcsFuncDecl(t *testing.T, file, recv, name string) (*ast.FuncDecl, *token.FileSet) {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != name || fd.Body == nil {
			continue
		}
		if recv == "" && fd.Recv != nil {
			continue
		}
		if recv != "" {
			if fd.Recv == nil || len(fd.Recv.List) != 1 {
				continue
			}
			if qcsTypeName(fd.Recv.List[0].Type) != recv {
				continue
			}
		}
		return fd, fset
	}
	t.Fatalf("%s is not in %s with a body — this pin reads that function's calls, so a rename or a move has to bring it along", qcsName(recv, name), file)
	return nil, nil
}

func qcsName(recv, name string) string {
	if recv == "" {
		return name
	}
	return recv + "." + name
}

// qcsTypeName is the receiver's type name, value or pointer.
func qcsTypeName(e ast.Expr) string {
	if s, ok := e.(*ast.StarExpr); ok {
		e = s.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// qcsCalls names every function called anywhere under n, by its last
// identifier — so `wwOneLine(x)` is "wwOneLine" and `t.Fatalf(...)` is
// "Fatalf".
func qcsCalls(n ast.Node) []string {
	var out []string
	ast.Inspect(n, func(x ast.Node) bool {
		c, ok := x.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := c.Fun.(type) {
		case *ast.Ident:
			out = append(out, fun.Name)
		case *ast.SelectorExpr:
			out = append(out, fun.Sel.Name)
		}
		return true
	})
	return out
}

func qcsHas(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// TestQAWallWatchLaunchFatalsCarryTheFoldedWordsAtTheCallSite is the call-site
// half of ranger-base-a688i, whose fix was the two fatals and whose pin is on
// the fold.
//
// WHAT THE WALL CHECK CAN READ. scripts/posse-wall-check reports a failed
// member by evidence(): the first `file.go:NN:` line that is not the venue
// marker, 300 characters, and nothing after it (ADR 0097 D3). So a member's
// budget is ONE line and it is the member's job to spend it on a cause. Both
// of launchOn's fatals fire with the watcher's words already in the buffer,
// and the whole of ranger-base-a688i was that they put those words on line 2.
//
// THE LOOP IS THE KEY, not the sentence. Every t.Fatalf inside launchOn's wait
// loop is reached with the watcher goroutine already running, so each one has
// words to spend its line on and none of them may spend it on a symptom. A
// fatal ABOVE the loop is setup — no watcher, nothing said — and is not this
// pin's subject, which is why the pin reads the loop and not the function.
//
// MEASURED 2026-11-21 against the fourth of runWallWatch's four exit-1 causes,
// the one the fix's own close did not reproduce (a wall missing at launch,
// respond(missing, true) — the only cause that writes a multi-line verdict and
// files a bead): the folded fatal is 370 characters, so evidence() does drop
// its tail, and what survives inside the 300 is `KILLED seat <seat>: the wall
// over <path> came off` — the cause, at the front, where the budget is. The
// timestamp and the filed bead id are what the truncation takes.
func TestQAWallWatchLaunchFatalsCarryTheFoldedWordsAtTheCallSite(t *testing.T) {
	t.Parallel()
	const (
		file = "wallwatchlive_test.go"
		fold = "wwOneLine"
	)
	fn, fset := qcsFuncDecl(t, file, "wwLiveSeat", "launchOn")

	var loops []ast.Node
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			loops = append(loops, n)
		}
		return true
	})
	if len(loops) == 0 {
		t.Fatalf("%s.launchOn has no loop — this pin reads the fatals inside the wait loop, because those are the ones reached with the watcher already running and therefore the ones with words to report (ranger-base-a688i)", file)
	}

	found := 0
	for _, loop := range loops {
		ast.Inspect(loop, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Fatalf" {
				return true
			}
			found++
			if !qcsHas(qcsCalls(c), fold) {
				t.Errorf("%s:%d: this t.Fatalf is inside launchOn's wait loop and does not call %s. The watcher's goroutine is already running here, so its words — and its bwrap's — are in the buffer, and `scripts/posse-wall-check` carries only this member's FIRST `file.go:NN:` line (ADR 0097 D3). A fatal that names the symptom and leaves the reason on line 2 reaches the bead as `the seat ended before it ran (exit 1):` with four possible causes and no way to tell which fired, which is ranger-base-a688i exactly.",
					file, fset.Position(c.Pos()).Line, fold)
			}
			return true
		})
	}
	// TWO, because exit-1-before-the-seat-ran and the-seat-never-ran are
	// different facts about a launch and each spends its own line. A count
	// that fell to one would mean a branch was dropped rather than fixed.
	if found < 2 {
		t.Errorf("%s.launchOn's wait loop makes %d t.Fatalf call(s), want at least 2 — the loop refuses a launch in two ways (the watcher returned early, and the seat never ran inside the deadline) and each is a separate member failure with its own one-line budget to spend on a cause (ranger-base-a688i)", file, found)
	}
}

// TestQAUpgradeVerifyReportHalvesArePrintedOnTheStandPathOnly is the
// placement half of ADR 0092 D7, amendment 3 and D4 rule 5. Each of those
// three landed a NOTE, and all three are deliberately printed only where the
// install stands.
//
// WHY PLACEMENT IS BEHAVIOUR HERE. A run that is about to be undone is a run
// whose binary will not be there: D7's hook-wall line prescribes a re-render
// against it, amendment 3's note says the behaviour probe could not be taken
// of it, and rule 5's note says a row stood on ITS class. Printed under a
// rollback, each of the three tells the delegate something about a binary the
// next line of the same report removes — the one wrong instruction the block
// can give, which is the reason all three carry the same comment.
//
// MEASURED 2026-11-21 (ranger-base-nqw8v): all three, added one at a time to
// the rollback path, left `-run 'TestUpgrade|TestQAEveryStatusConditionCode...'`
// GREEN. The three presence pins measure that the note is computed and
// printed; nothing measured WHERE.
func TestQAUpgradeVerifyReportHalvesArePrintedOnTheStandPathOnly(t *testing.T) {
	t.Parallel()
	const file = "upgradeverify.go"
	fn, fset := qcsFuncDecl(t, file, "", "upgradeVerify")

	// The stand path is the block guarded by "nothing is newly red", which
	// the verdict spells as an empty reason list. Keyed on that shape and
	// not on a line number: the one `if` in this function whose body ends
	// in `return nil` is the arm where the install survives.
	var stand *ast.BlockStmt
	for _, st := range fn.Body.List {
		ifst, ok := st.(*ast.IfStmt)
		if !ok || ifst.Body == nil || len(ifst.Body.List) == 0 {
			continue
		}
		ret, ok := ifst.Body.List[len(ifst.Body.List)-1].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		if id, ok := ret.Results[0].(*ast.Ident); ok && id.Name == "nil" {
			stand = ifst.Body
			break
		}
	}
	if stand == nil {
		t.Fatalf("upgradeVerify in %s has no top-level `if … { … return nil }` — this pin reads that block as the STAND path (the install survived, so the verdict had nothing to say), and the three report halves below belong to it alone. A change to the function's shape has to bring the pin along", file)
	}

	inStand := qcsCalls(stand)
	whole := qcsCalls(fn.Body)
	for _, half := range []struct{ fn, why string }{
		{"upgradeHookWallLine", "ADR 0092 D7: it prescribes a re-render against the binary that was just installed"},
		{"upgradeClassNote", "ADR 0092 D4 rule 5: it reports a gained row that stood on the TARGET binary's own class"},
		{"upgradeRenderNote", "ADR 0092 D4 amendment 3: it reports that the behaviour probe could not be taken of the new binary"},
	} {
		n := 0
		for _, c := range whole {
			if c == half.fn {
				n++
			}
		}
		if n == 0 {
			t.Errorf("upgradeVerify does not call %s at all — %s, and a note that is computed and never printed is a note the delegate reading the install bead does not get (ADR 0092)", half.fn, half.why)
			continue
		}
		if !qcsHas(inStand, half.fn) {
			t.Errorf("upgradeVerify calls %s, but not from the stand path — %s, so printed from anywhere else it describes a binary this same report is about to roll back", half.fn, half.why)
		}
		if n > 1 {
			t.Errorf("upgradeVerify calls %s %d times, want 1 (at %s). All three of this block's report halves are stand-path-only by the same decision: on a run that is about to be undone they describe a binary that will not be there, which is the one wrong instruction this block can give.",
				half.fn, n, fset.Position(fn.Pos()).String())
		}
	}
}
