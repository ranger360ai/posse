//go:build posse_arm3

package posse

// ranger-base-2yj5 — the routing order among label matches is a stated
// decision, not the order os.ReadDir hands the agents dir back.
//
// The finding these tests pin: Route returned the FIRST persona whose
// labels overlapped the bead's, walking ListAgents, which is alphabetical.
// On the crew it was found in, every unassigned `code` bead went to the
// seeded `developer` (14 lifetime closes) rather than the lane the
// operator wrote, purely on the alphabet, and 11 of 37 unassigned open
// beads sat on PIDs with 14 and 0 lifetime closes. Retiring the unused
// generics is config and the operator's call; what
// is code's is that the ordering be sayable (`route_order:`), that the
// default change nothing, and that a pass line say who else was in the
// race.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeOrderedPersona is writePersona plus a `route_order:` line. Passing
// order == "" writes no key at all — the case that must behave exactly as
// it did before this bead.
func writeOrderedPersona(t *testing.T, a *App, name, labels, order string) {
	t.Helper()
	os.MkdirAll(a.AgentsDir, 0o755)
	md := "---\nname: " + name + "\ndescription: test\nlabels: " + labels + "\n"
	if order != "" {
		md += "route_order: " + order + "\n"
	}
	md += "---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

func routeCode(t *testing.T, d *Dispatcher) (string, string) {
	t.Helper()
	return d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-1", Labels: []string{"code"}}})
}

// No route_order anywhere: the winner is what it was before this bead, so
// an instance that never touches the key sees no change. What IS new is
// the why — the line now says who else matched, which is the whole audit
// this bead came out of.
func TestQARouteOrderAbsentKeepsTodaysWinnerAndNamesTheRace(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeOrderedPersona(t, b.App, "developer", "[code, feature]", "")
	writeOrderedPersona(t, b.App, "hopper", "[code, feature]", "")

	p, why := routeCode(t, d)
	if p != "developer" {
		t.Errorf("absent route_order must not move the winner: got %q (%s)", p, why)
	}
	for _, want := range []string{"label:code", "first of 2", "developer, hopper"} {
		if !strings.Contains(why, want) {
			t.Errorf("why %q missing %q", why, want)
		}
	}
}

// One persona matching is the common case and must stay a short line: no
// roster, because there was no race.
func TestQARouteWhyStaysBareWhenOnlyOneMatches(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeOrderedPersona(t, b.App, "hopper", "[code]", "")
	writeOrderedPersona(t, b.App, "devops", "[infra]", "")

	p, why := routeCode(t, d)
	if p != "hopper" || why != "label:code" {
		t.Errorf("single match: got %q (%s), want hopper (label:code)", p, why)
	}
}

// The fix, both directions: promote the working lane, or demote the
// generic. Either edit is one line in one PID and no code change.
func TestQARouteOrderPromotesAndDemotes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, devOrder, hopOrder string }{
		{"promote the named lane", "", "10"},
		{"demote the generic", "90", ""},
		{"both stated", "90", "10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			writeOrderedPersona(t, b.App, "developer", "[code]", tc.devOrder)
			writeOrderedPersona(t, b.App, "hopper", "[code]", tc.hopOrder)

			p, why := routeCode(t, d)
			if p != "hopper" {
				t.Errorf("route_order ignored: got %q (%s)", p, why)
			}
			// The roster is in the order routing preferred them, so the
			// operator reads the effect of the key, not a sorted list.
			if !strings.Contains(why, "first of 2: hopper, developer") {
				t.Errorf("why %q does not show the resolved order", why)
			}
		})
	}
}

// A tie is broken by persona name — the documented tiebreak. Same order as
// the old readdir accident, said out loud, so the next PID someone adds
// cannot jump the queue by being named `aaa` without also saying so.
func TestQARouteOrderTieBreaksOnPersonaName(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	// Written youngest-name-first: nothing about creation order may leak in.
	writeOrderedPersona(t, b.App, "zulu", "[code]", "7")
	writeOrderedPersona(t, b.App, "alpha", "[code]", "7")

	if p, why := routeCode(t, d); p != "alpha" {
		t.Errorf("tie must break on persona name: got %q (%s)", p, why)
	}
	// And an explicit order still beats the name, or the key is decoration.
	writeOrderedPersona(t, b.App, "zulu", "[code]", "1")
	if p, why := routeCode(t, d); p != "zulu" {
		t.Errorf("route_order must outrank the name tiebreak: got %q (%s)", p, why)
	}
}

// The roster is capped so a pass line stays a line — and the cap is never
// silent: the count is the true count and the remainder is named.
func TestQARouteWhyCapsTheRosterWithoutHidingTheCount(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	for _, n := range []string{"p1", "p2", "p3", "p4", "p5", "p6"} {
		writeOrderedPersona(t, b.App, n, "[code]", "")
	}
	_, why := routeCode(t, d)
	if !strings.Contains(why, "first of 6:") {
		t.Errorf("why %q must count every match", why)
	}
	if !strings.Contains(why, "p1, p2, p3, p4, +2 more") {
		t.Errorf("why %q must cap the roster and say what it dropped", why)
	}
}

// The coordinator is not a lane (ADR 0033 §2) and so is not in the race —
// not as the winner, and not in the count either: a roster that included
// her would read as "she was considered".
func TestQARouteRosterExcludesTheCoordinator(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeOrderedPersona(t, b.App, "coordinator", "[code]", "1")
	writeOrderedPersona(t, b.App, "developer", "[code]", "")
	writeOrderedPersona(t, b.App, "hopper", "[code]", "")
	cfg(t, b.App, "coordinator: coordinator\n")

	p, why := routeCode(t, d)
	if p != "developer" {
		t.Errorf("coordinator won the label race at route_order 1: got %q (%s)", p, why)
	}
	if !strings.Contains(why, "first of 2: developer, hopper") || strings.Contains(why, "coordinator") {
		t.Errorf("why %q must not count the coordinator", why)
	}
}

// An assignee is an explicit choice and route_order has no business in it.
func TestQARouteOrderDoesNotTouchAnAssignedBead(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeOrderedPersona(t, b.App, "developer", "[code]", "1")
	writeOrderedPersona(t, b.App, "hopper", "[code]", "99")

	p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-2", Assignee: "hopper", Labels: []string{"code"}}})
	if p != "hopper" || why != "assignee" {
		t.Errorf("assignee rerouted by route_order: got %q (%s)", p, why)
	}
}

// A mistyped ordering hint must not take a lane off the board: it loads at
// the default. It must not be silent either — `posse agent check` says so.
func TestQARouteOrderMalformedTakesTheDefaultAndIsReported(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeOrderedPersona(t, b.App, "developer", "[code]", "high")
	writeOrderedPersona(t, b.App, "hopper", "[code]", "")

	ag, err := b.App.LoadAgent("developer")
	if err != nil {
		t.Fatalf("a bad route_order must not stop the PID loading: %v", err)
	}
	if ag.RouteOrder != RouteOrderDefault {
		t.Errorf("route_order: high should fall back to %d, got %d", RouteOrderDefault, ag.RouteOrder)
	}
	if p, why := routeCode(t, d); p != "developer" {
		t.Errorf("lane went silent over an ordering hint: got %q (%s)", p, why)
	}

	findings, _, err := b.App.CheckAgent("developer")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range findings {
		if strings.Contains(f, "route_order") && strings.Contains(f, "not an integer") {
			found = true
		}
	}
	if !found {
		t.Errorf("posse agent check said nothing about route_order: high; findings: %v", findings)
	}
}

// Negative values work — an instance that would rather promote below zero
// than renumber its crew is not wrong, it is just using ints.
func TestQARouteOrderAcceptsNegative(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeOrderedPersona(t, b.App, "developer", "[code]", "")
	writeOrderedPersona(t, b.App, "hopper", "[code]", "-5")

	if p, why := routeCode(t, d); p != "hopper" {
		t.Errorf("negative route_order ignored: got %q (%s)", p, why)
	}
}
