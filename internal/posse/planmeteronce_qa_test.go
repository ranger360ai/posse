// QA pin for ranger-base-67mdf over ranger-base-ddivo, built by the verify of
// ranger-base-f8mqa (ranger-base-9yje2).
//
// f8mqa's whole deliverable is a COUNT: one meter read per PlanStaleness call
// instead of PlanMeterSpender x3 and PlanMeterQuiet x2. Measured both ways
// with an overlay counter — before 253cc62 x3/x2, after it x1/x0 — and the
// line the surfaces print is byte-identical in both arms. Which is the
// problem: every pin the design listed stays green byte-for-byte with 253cc62
// reverted, and so does the whole internal/posse package. Nothing in the tree
// held the number the bead was filed for.
//
// The count is not observable from outside the process — the reads are a
// config get and an flock, and neither leaves a mark a test can total — so
// this asks the source instead, the way TestQAKeychainStoreHandsTheAdapterThe
// BinaryConstant and the qib census tests already do. Comments are dropped by
// the parser (mode 0), because the doc comments in planstale.go quote
// PlanMeterSpender by name and a line scan would read the sentence about the
// call as the call.
//
// What it protects, in one line: nobody may ask the meter a second time
// inside one PlanStaleness, because two reads of a decaying state can
// disagree and the disagreeing shape prints "ruling on it under the headroom
// rule" over a box where no rule is running.
package posse

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// planMeterCalls counts calls by callee name inside one function of one file.
// Mode 0: comments are not retained, so prose naming a function cannot be
// mistaken for a call to it.
func planMeterCalls(t *testing.T, path, fn string) map[string]int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var body *ast.FuncDecl
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == fn {
			body = fd
			break
		}
	}
	if body == nil {
		t.Fatalf("%s: no func %s — this pin names the shape it guards, so a rename is a decision to make here too", path, fn)
	}
	got := map[string]int{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			got[f.Sel.Name]++
		case *ast.Ident:
			got[f.Name]++
		}
		return true
	})
	return got
}

func TestQAOneMeterReadPerPlanStalenessCall(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		file, fn string
		want     map[string]int
		why      string
	}{
		{
			file: "planstale.go", fn: "PlanStaleness",
			want: map[string]int{
				"PlanCache":           1,
				"PlanMeterQuiet":      0,
				"PlanMeterSpender":    0,
				"PlanGuardThresholds": 0,
				"planMeterState":      0,
			},
			why: "the verdict and the spender both come off the one cache this builds; asking for either again is the second read",
		},
		{
			file: "planquiet.go", fn: "PlanQuietLine",
			want: map[string]int{
				"PlanCache":        1,
				"PlanMeterQuiet":   0,
				"PlanMeterSpender": 0,
			},
			why: "the same seam and the same repeat: the cache carries the verdict this line forks on",
		},
		{
			file: "plancache.go", fn: "PlanCache",
			want: map[string]int{
				"planMeterState":   1,
				"PlanMeterQuiet":   0,
				"PlanMeterSpender": 0,
			},
			why: "one call, and Quiet and Spender are its two answers — a second call is how they come to disagree",
		},
		{
			file: "planquiet.go", fn: "planMeterState",
			want: map[string]int{
				"PlanMeterSpender": 1,
			},
			why: "the one place the spend state is read at all",
		},
	} {
		got := planMeterCalls(t, c.file, c.fn)
		for name, want := range c.want {
			if got[name] != want {
				t.Errorf("%s %s calls %s %dx, want %dx — %s (ranger-base-67mdf, built by ranger-base-f8mqa)",
					c.file, c.fn, name, got[name], want, c.why)
			}
		}
	}
}
