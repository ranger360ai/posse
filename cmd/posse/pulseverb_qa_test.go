package main

// The cmd/posse half of ADR 0093 D1's structural claim (ranger-base-cgr1h):
// the `pulse` verb's arm in main's switch never touches the herdr backend.
//
// internal/posse/pulsewaitpane_qa_test.go pins the same claim over the
// waiter's own source; this one is here because main.go is where a verb
// acquires a pane. Every other verb in that switch has `hb` in scope — it is
// built before the switch and most arms use it — so "the waiter writes to no
// pane" is one careless `hb.` away at this layer whatever internal/posse's
// signature says.
//
// Read from the AST and not by grep, for the reason verbhelp_qa_test.go
// reads the same switch that way: a hand-kept list of what the arm may
// mention cannot red when the arm grows.

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"
)

// pulseCaseClause is the `case "pulse":` clause of main's verb switch.
func pulseCaseClause(t *testing.T) *ast.CaseClause {
	t.Helper()
	sw, _ := mainVerbSwitch(t)
	for _, st := range sw.Body.List {
		cc, ok := st.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, e := range cc.List {
			lit, ok := e.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			if strings.Trim(lit.Value, `"`) == "pulse" {
				return cc
			}
		}
	}
	t.Fatal(`main's verb switch has no case "pulse" — the verb has been renamed or removed, so this pin is measuring an absence`)
	return nil
}

// The pin: no identifier in the arm names the backend, and none of its
// pane-writing verbs appears.
func TestPulseVerbNeverReachesTheHerdrBackend(t *testing.T) {
	// `hb` is the backend main builds before the switch. The rest are the
	// names a pane write would wear if someone reached for one without it.
	forbidden := map[string]string{
		"hb":              "the herdr backend main builds before the switch",
		"NewHerdrBackend": "a backend of this arm's own",
		"Mux":             "the pane multiplexer",
		"Prompt":          "submits text to the runtime — the splice ADR 0093 ends",
		"SendText":        "types into a pane's input",
		"SendKeys":        "presses keys in a pane",
	}
	cc := pulseCaseClause(t)
	ast.Inspect(cc, func(n ast.Node) bool {
		var name string
		switch id := n.(type) {
		case *ast.Ident:
			name = id.Name
		case *ast.SelectorExpr:
			name = id.Sel.Name
		default:
			return true
		}
		if why, bad := forbidden[name]; bad {
			t.Errorf(`case "pulse" names %s — %s. ADR 0093 D1's guarantee is that the waiter writes nothing to a pane: a delivery is this command's EXIT, and the typed fallback is the watch's (D3/D4)`, name, why)
		}
		return true
	})
}

// And the floor under it: the arm does reach the waiter. A `pulse` case that
// had stopped calling posse.PulseWait would pass the census above by doing
// nothing at all.
func TestPulseVerbReachesTheWaiter(t *testing.T) {
	cc := pulseCaseClause(t)
	found := false
	ast.Inspect(cc, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "PulseWait" {
			found = true
		}
		return true
	})
	if !found {
		t.Error(`case "pulse" does not call posse.PulseWait — the verb no longer arms the waiter ADR 0093 D1 names`)
	}
}
