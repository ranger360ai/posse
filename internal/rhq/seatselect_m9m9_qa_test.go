package rhq

// ranger-base-69jo (escape from the ranger-base-m9m9 verify of
// ranger-base-2yj5) — ADR 0020 §2: a lane is a set of labels, a seat is a
// persona, and selection among peers is AVAILABILITY-FIRST at dispatch.
//
// ca0c00e landed the roster and a stated `route_order:` tiebreak — real
// work, and its sort is already §2's "order by key, then name". What it did
// NOT land is the half the ADR decided: the fire loop still takes Route's
// single head and skips the BEAD when that one persona is busy, so a free
// seat beside it is never offered the work.
//
// THESE PINS ARE GREEN AND THEY ASSERT THE HOLE. That is deliberate and it
// is the shop's standard (NOTES.md §"Why this one is nastier than the
// mislabel", rangerhq-th7l): a `t.Skip` pin is how a live defect stayed
// green through a silent revert, because the unskip rode in the same commit
// as the fix and went out with it. A red pin gets deleted and a skipped one
// gets forgotten; a green pin that FAILS THE DAY THE FIX LANDS is the only
// shape that survives. Each failure message below carries its own inversion,
// so whoever builds §2 is handed the assertion to replace it with.

import (
	"strings"
	"testing"
)

// ADR 0020, Consequences, verbatim: "A pass with three unassigned code
// beads and three free code seats fires all three; today it fires one and
// marks the lane's only routable persona busy."
//
// Today it fires one. Pinned as such.
func TestQASeatSelectionMissingSoAFreeSeatIsNeverOffered(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "dinesh", "[code]")
	writePersona(t, b.App, "gwart", "[code]")
	qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["code"]},{"id":"a-2","title":"u","labels":["code"]}]`,
		`[{"id":"x","status":"closed"}]`)
	agentPerLaunch(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 || !strings.Contains(out, "busy this pass") {
		t.Fatalf(`ranger-base-69jo LOOKS FIXED — two free code seats now fire n=%d (was 1).

If ADR 0020 §2 has landed, INVERT THIS PIN to the ADR's assertion and delete it from 69jo:
    if n != 2 { t.Errorf("two free seats and two beads must fire two, got %%d", n) }
    if strings.Contains(out, "busy this pass") { t.Errorf("no bead may report busy while a seat is free") }
    for _, who := range []string{"dinesh", "gwart"} {   // ADR 0020 §4: a persona stays strictly serial
        if strings.Count(out, "persona "+who) != 1 { t.Errorf("%%s must take exactly one bead", who) }
    }
If it has NOT landed, this is a behaviour change nobody asked for.

pass report:
%s`, n, out)
	}
	// The seat that was never offered the work.
	if strings.Contains(out, "persona gwart") {
		t.Errorf("gwart was seated — §2 may have landed; invert this pin:\n%s", out)
	}
}

// ADR 0020 §2: every seat busy is the LANE being busy — "All seats busy →
// the bead waits for a later pass, and the report names the lane ('code
// lane busy: dinesh, gwart, jian-yang'), not one persona."
//
// Today the skip line names Route's single head, which reads as "dinesh IS
// the code lane" — the single-seat model this ADR retired.
func TestQALaneBusySkipStillNamesOnePersonaNotTheLane(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "dinesh", "[code]")
	writePersona(t, b.App, "gwart", "[code]")
	qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["code"]},{"id":"a-2","title":"u","labels":["code"]},{"id":"a-3","title":"v","labels":["code"]}]`,
		`[{"id":"x","status":"closed"}]`)
	agentPerLaunch(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if strings.Contains(out, "code lane busy") {
		t.Fatalf(`ranger-base-69jo LOOKS FIXED — the pass now reports the lane.

INVERT THIS PIN:
    if !strings.Contains(out, "code lane busy") { t.Errorf("a full lane must report the lane, not one persona") }
    for _, who := range []string{"dinesh", "gwart"} {
        if !strings.Contains(out, who) { t.Errorf("the lane-busy line must name every seat it tried, missing %%q", who) }
    }

pass report:
%s`, out)
	}
	if !strings.Contains(out, "busy this pass") {
		t.Errorf("expected the single-persona busy skip that 69jo pins:\n%s", out)
	}
}

// ADR 0020 §2: "The route report must say why a seat won — 'label:code
// (seat 2/3: gwart; dinesh busy)' — the audit line ranger-base-2yj5 asked
// for."
//
// ca0c00e's line names the RACE ("first of 3: dinesh, gwart, jian-yang"),
// which answers the original bead's cheap-guard ask and is worth keeping.
// It cannot answer §2's, because nothing on this path reads availability.
func TestQARouteWhyNamesTheRaceNotTheSeat(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "dinesh", "[code]")
	writePersona(t, b.App, "gwart", "[code]")
	writePersona(t, b.App, "jian-yang", "[code]")

	p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-1", Labels: []string{"code"}}})
	if strings.Contains(why, "seat") {
		t.Fatalf(`ranger-base-69jo LOOKS FIXED — why now names the seat: %q

INVERT THIS PIN to assert §2's line over a whole pass (Route alone cannot see
availability, so assert on the pass report, not here):
    if !strings.Contains(out, "seat 1/3") || !strings.Contains(out, "seat 2/3") { ... }
    if !strings.Contains(out, "dinesh busy") { ... }`, why)
	}
	if p != "dinesh" || !strings.Contains(why, "first of 3: dinesh, gwart, jian-yang") {
		t.Errorf("the roster clause ca0c00e landed must not regress: got %q (%s)", p, why)
	}
}

// ADR 0020 §2: "--persona X restricts seating to X: a bead whose lane
// contains X may seat only there, others are skipped as today."
//
// Today the bead is dropped at dispatch.go's `persona != personaFilter`
// before any line is printed, so `posse dispatch --persona gwart` reports
// NOTHING while gwart sits idle beside ready work in gwart's own lane. The
// silence is the worst part: there is no line to grep.
func TestQAPersonaFilterSilentlyDropsABeadInThatPersonasLane(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "dinesh", "[code]")
	writePersona(t, b.App, "gwart", "[code]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["code"]}]`, `[{"id":"x","status":"closed"}]`)
	agentPerLaunch(t, fake)

	n, err := d.Run("", "gwart", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 || strings.Contains(out, "gwart") {
		moved := "the bead is still not seated, but the pass now says so — the silence went first"
		if n != 0 {
			moved = "the bead is now seated under --persona gwart"
		}
		t.Fatalf(`ranger-base-69jo MOVED (%s) — n=%d

INVERT THIS PIN:
    if n != 1 { t.Errorf("gwart is in a-1's lane, so --persona gwart must seat it there, got %%d", n) }
    if !strings.Contains(out, "gwart") { t.Errorf("--persona must never be silent about a bead in that persona's lane") }

pass report:
%s`, moved, n, out)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("69jo pins this as SILENT; a line appeared, so the shape moved:\n%s", out)
	}
}

// Not a hole — the correct half of §2.4, pinned green so building the other
// half cannot widen --persona into "seat it anywhere". A bead whose lane
// EXCLUDES the filtered persona stays skipped.
func TestQAPersonaFilterSkipsBeadsOutsideTheLane(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "dinesh", "[code]")
	writePersona(t, b.App, "gilfoyle", "[infra]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["code"]}]`, `[{"id":"x","status":"closed"}]`)
	agentPerLaunch(t, fake)

	n, err := d.Run("", "gilfoyle", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("gilfoyle is not in a-1's lane; --persona gilfoyle must dispatch nothing, got n=%d:\n%s", n, dispatcherOut(d))
	}
}
