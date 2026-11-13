//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-v87a1 — the partition `posse upgrade`'s automatic
// rollback is computed over.
//
// WHAT IT PROTECTS, and why it is a census rather than a list somebody
// maintains. ADR 0092 D4 rolls an install back when `posse status` gains a
// condition whose code is one THE BINARY RENDERS, and excludes the codes
// that move on their own while the verb's twelve-minute `make test` runs.
// The ADR named three and said so: "That code set is ASSUMED complete here.
// The build bead must list every condition code by source and pin the set."
//
// A set like that goes wrong in one direction and in silence. The next bead
// to add a G-row to ShopCheck has no reason to think about an installer, so
// the row lands unclassified — and an unclassified row is handled by
// upgradeCondSource's unknown rule, which treats it as binary-rendered and
// rolls back. That is the SAFE direction and it is still wrong: a shop-state
// row added tomorrow would roll back every upgrade the day it first appears,
// with a report blaming a binary that did nothing. The other direction is
// worse and is the one this pin exists for at all — a row nobody classified
// is a row nobody THOUGHT about, and the thinking is the deliverable.
//
// THE CENSUS IS OVER THIS PACKAGE'S OWN SHIPPED SOURCES, with the file set
// DISCOVERED rather than spelled, so a G-row added in a file that does not
// exist today is still found. That is what puts this pin in the class
// QA_PKGCENSUS_PINS holds, and its door is `make pkgcensus-check` — type it
// when you add or edit a .go file anywhere in this module.
//
// TWO SHAPES, because ShopCheck's rows are written two ways and both are
// load-bearing:
//
//  1. `add("G12", GovLane, key, detail)` — the four-argument closure
//     ShopCheck builds and passes down to beadConditions. Matched on its
//     SIGNATURE (four args, a string literal first, GovUrgent or GovLane
//     second) rather than on the identifier `add`, which is a name five
//     other files in this package use for something else entirely.
//  2. `GovCondition{ID: "G11", Class: ..., ...}` — the composite literal the
//     four rows computed outside govern.go return (launcherlag.go,
//     memoryland.go, queuejsonl.go, verifybox.go).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// upgradeClassCensus walks every shipped .go file of this package and
// returns each condition row name it can print, mapped to where it was
// found. The key is the row as GovReport prints it, so a carry-over's empty
// ID arrives here as the em dash and is looked up in the same table a reader
// of `posse status` would use.
func upgradeClassCensus(t *testing.T, dir string) map[string][]string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	found := map[string][]string{}
	note := func(id, where string) {
		row := GovCondition{ID: id}.Row()
		for _, seen := range found[row] {
			if seen == where {
				return
			}
		}
		found[row] = append(found[row], where)
	}
	fset := token.NewFileSet()
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, dir+"/"+name, nil, 0)
		if perr != nil {
			t.Fatalf("parsing %s: %v", name, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.CallExpr:
				// Shape 1 — the four-argument row closure, identified by
				// its ARGUMENTS and not by the callee's name (`add` names
				// something else in five other files of this package).
				// Three tests, and the second is the one that does the
				// work: a row name in the first position, spelled as the
				// literal the table is keyed on. The class in the second
				// position is asked for as an IDENTIFIER rather than as
				// GovUrgent or GovLane by name, because G3 and G16 compute
				// their class into a local first — a row whose URGENCY
				// depends on the reading is still a row.
				if len(v.Args) != 4 {
					return true
				}
				if _, ok := v.Args[1].(*ast.Ident); !ok {
					return true
				}
				id, ok := upgradeClassLit(v.Args[0])
				if !ok || !upgradeClassRowLit(id) {
					return true
				}
				note(id, name)
			case *ast.CompositeLit:
				// Shape 2 — a GovCondition literal, bare, through the
				// package selector, or ELIDED inside a []GovCondition. The
				// elided form is not a corner case: G11 is written that
				// way, and a census that read only the spelled form found
				// everything except the one row `posse upgrade` is itself
				// the subject of.
				if upgradeClassIsGovCondition(v.Type) {
					upgradeClassNoteID(v, name, note)
					return true
				}
				arr, ok := v.Type.(*ast.ArrayType)
				if !ok || !upgradeClassIsGovCondition(arr.Elt) {
					return true
				}
				for _, el := range v.Elts {
					if inner, ok := el.(*ast.CompositeLit); ok && inner.Type == nil {
						upgradeClassNoteID(inner, name, note)
					}
				}
			}
			return true
		})
	}
	return found
}

// upgradeClassRowLit is the grammar of a condition's ID as ShopCheck writes
// it: a G-number, or the empty string a pulse-era carry-over carries. It is
// what keeps shape 1's signature test from reading an unrelated
// four-argument call as a governance row.
func upgradeClassRowLit(id string) bool {
	if id == "" {
		return true
	}
	if !strings.HasPrefix(id, "G") || len(id) < 2 {
		return false
	}
	for _, r := range id[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// upgradeClassNoteID pulls the ID field out of one GovCondition literal.
func upgradeClassNoteID(cl *ast.CompositeLit, where string, note func(id, where string)) {
	for _, el := range cl.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if k, ok := kv.Key.(*ast.Ident); !ok || k.Name != "ID" {
			continue
		}
		if id, ok := upgradeClassLit(kv.Value); ok {
			note(id, where)
		}
	}
}

func upgradeClassLit(e ast.Expr) (string, bool) {
	b, ok := e.(*ast.BasicLit)
	if !ok || b.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(b.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func upgradeClassIsGovCondition(t ast.Expr) bool {
	switch v := t.(type) {
	case *ast.Ident:
		return v.Name == "GovCondition"
	case *ast.SelectorExpr:
		return v.Sel != nil && v.Sel.Name == "GovCondition"
	}
	return false
}

// TestQAEveryStatusConditionCodeHasAnUpgradeClass is the pin ADR 0092 D4
// asked the build bead for. Door: `make pkgcensus-check`.
func TestQAEveryStatusConditionCodeHasAnUpgradeClass(t *testing.T) {
	t.Parallel()
	found := upgradeClassCensus(t, ".")

	// THE CONTROL THAT SAYS THIS CENSUS CAN STILL SAY NO. A matcher that
	// stopped matching — a renamed closure, a refactor that moved the class
	// constant out of the second argument — would find nothing and pass
	// over an empty set, which is the one failure a census cannot report on
	// itself. The floor is deliberately well under the live count (18 rows
	// on 2026-11-13) so that an ordinary landing does not have to move it,
	// and well over zero so that a dead matcher cannot hide.
	if len(found) < 12 {
		t.Fatalf("the condition census found only %d row name(s): %v — it is reading nothing, so every claim below is vacuous. Check the two shapes in this file's header against govern.go.", len(found), upgradeClassRows(found))
	}

	var missing []string
	for row, where := range found {
		if _, ok := upgradeConditionSource[row]; !ok {
			missing = append(missing, row+" ("+strings.Join(where, ", ")+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf(`%d `+"`posse status`"+` condition row(s) have no class in upgradeConditionSource:
  %s

Every row this binary can print has to be classified, because `+"`posse upgrade`"+` rolls an
install back when a row it RENDERS appears between the two readings and must not when a
row that MOVES ON ITS OWN does (ADR 0092 D4). An unclassified row is handled as
binary-rendered, which is the safe answer and not the right one: a shop-state row left
unclassified rolls back every upgrade from the day it first appears.

Add it to upgradeConditionSource in upgradeverify.go, beside the measurement table in
that file's header, with the store of record it reads and whether that store moves while
a twelve-minute `+"`make test`"+` runs.`, len(missing), strings.Join(missing, "\n  "))
	}

	// And the other direction: a class for a row nothing can print is a row
	// somebody deleted, and leaving it here is how the table stops being a
	// reading of the code and starts being folklore.
	var stale []string
	for row := range upgradeConditionSource {
		if _, ok := found[row]; !ok {
			stale = append(stale, row)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("upgradeConditionSource classifies %s, which no shipped source in this package can print any more — drop the entry and the measurement table's row with it", strings.Join(stale, ", "))
	}
}

// The census must see a row added in a file that does not exist today, which
// is the whole reason its file set is discovered rather than spelled. Both
// shapes, over a fixture, because a matcher that reads only the shape
// govern.go happens to use today would go blind the first time a row moves
// out of ShopCheck.
func TestQAUpgradeClassCensusSeesBothRowShapes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write(t, dir+"/newrows.go", `package posse

func shopCheckLater(add func(id, class, key, detail string)) {
	add("G42", GovUrgent, "a-key", "a row added through the closure")
	add("", GovLane, "carry-over", "a row with no number")
	// A row whose urgency is computed first, which is how G3 and G16 are
	// written: the class arrives as a local and not as a constant.
	class := GovLane
	add("G45", class, "computed", "a row whose class is a local")
}

func anotherRow() GovCondition {
	return GovCondition{ID: "G43", Class: GovLane, Key: "k", Detail: "d"}
}

// The ELIDED shape, which is how G11 is written — and the shape a census
// that read only the spelled one missed.
func elidedRows() []GovCondition {
	return []GovCondition{{ID: "G46", Class: GovLane, Key: "k", Detail: "d"}}
}

func notARow() {
	someOtherAdd("G44", "not a class", "x", "y")
	fourArgsNoRowName("hook-wall", GovLane, "x", "y")
}
`)
	found := upgradeClassCensus(t, dir)
	for _, want := range []string{"G42", "G43", "G45", "G46", GovCondition{}.Row()} {
		if _, ok := found[want]; !ok {
			t.Errorf("the census missed %s: found %v", want, upgradeClassRows(found))
		}
	}
	// A four-argument call whose second argument is not an identifier at
	// all, and one whose first argument is not a row name: reading either as
	// a governance row would make the pin demand a class for every helper in
	// the package.
	if _, ok := found["G44"]; ok {
		t.Errorf("the census read a call with no class identifier as a condition row: %v", upgradeClassRows(found))
	}
	if len(found) != 5 {
		t.Errorf("the census found %v, want exactly the five rows the fixture declares", upgradeClassRows(found))
	}
}

func upgradeClassRows(found map[string][]string) []string {
	out := make([]string, 0, len(found))
	for row := range found {
		out = append(out, row)
	}
	sort.Strings(out)
	return out
}
