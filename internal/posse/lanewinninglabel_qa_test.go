//go:build posse_arm2

package posse

// ranger-base-equhp — a lane is ONE label, so a bead that carries several
// does not merge several lanes into one.
//
// The finding: laneFor took every persona whose PID labels INTERSECT the
// bead's as one candidate list, and routeWhy then named the lane after the
// first candidate's label alone. A bead carries the labels of all the work it
// touches, so `[code, debt, security, vuln-private]` — a code deliverable that
// came out of a security finding, which is the shape every such finding has by
// convention — read as a single four-seat `label:code` lane containing the
// security PID.
//
// MEASURED 2026-11-13 in the dispatch watch log, over ranger-base-g8fmb, on
// an instance with three code seats and one security seat: a four-name `code
// lane busy:` line, then — with the three code seats busy — `label:code (seat
// 3/4: …)` onto the SECURITY seat, and a session created on it. Twice in one
// night, at strong tier both times. That PID denies Edit and Write — findings
// only — so the seat cannot do a code deliverable. It can only hand the bead
// back, which spends a strong-tier seat and requeues the bead to lose the same
// race again on the next pass.
//
// What the fix is NOT: a rule about Edit/Write, or about the security label.
// The seat was wrong before the PID's denies were read, because the lane was
// wrong — ADR 0020 §1 (folded into ADR 0011 §4) says a lane is a set of
// LABELS and a seat is a persona whose PID labels intersect THEM, and the
// bead's own label set is not that lane. All seats busy means the bead waits
// (§2), which is a pass of latency; seating it on a PID that cannot do it is
// a spent seat AND the same latency.

import (
	"strings"
	"testing"
)

// laneOf is the lane as routing sees it — the names, in the order it prefers
// them, and the label that made it a lane.
func laneOf(t *testing.T, d *Dispatcher, labels ...string) ([]string, string) {
	t.Helper()
	l := d.laneFor(RepoIssue{BdIssue: BdIssue{ID: "a-1", Labels: labels}})
	if l.deny != "" {
		t.Fatalf("lane denied for %v: %s", labels, l.deny)
	}
	var names []string
	for _, s := range l.seats {
		names = append(names, s.name)
	}
	return names, l.label
}

// The finding itself, at the lane. A bead labelled [code, security] is a code
// lane of the code seats — the security PID is not in it, free or busy.
func TestQALaneIsOneLabelNotTheUnionOfTheBeadsLabels(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "developer", "[code, feature, bug]")
	writePersona(t, b.App, "developer-2", "[code, feature, bug]")
	// Sorts AFTER the code seats on the name tiebreak, exactly as the crew
	// PID this was found on does: the union put it at the END of the code
	// lane, which is why only a busy lane reached it.
	writePersona(t, b.App, "watcher", "[security, audit, vuln, deps]")

	names, label := laneOf(t, d, "code", "debt", "security", "vuln-private")
	if label != "code" {
		t.Errorf("the lane's label must be the preferred candidate's, got %q", label)
	}
	if strings.Join(names, ",") != "developer,developer-2" {
		t.Errorf("a code bead's lane is the code seats, got %v", names)
	}

	// And the security PID still has its own lane — the narrowing must not
	// strand the label it was the only match for.
	names, label = laneOf(t, d, "security", "vuln-private")
	if label != "security" || strings.Join(names, ",") != "watcher" {
		t.Errorf("security lane regressed: got %v (%s)", names, label)
	}
}

// The route report says the same thing: the roster is the lane, so "first of
// N" counts the seats that can actually take the bead. An operator reading
// "first of 3" over a two-seat lane is reading a hiring signal that is wrong
// by one (ADR 0020 §4: lane concurrency is seat count).
func TestQARouteWhyCountsTheLaneNotTheOverlap(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "developer", "[code]")
	writePersona(t, b.App, "developer-2", "[code]")
	writePersona(t, b.App, "watcher", "[security]")

	p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-1", Labels: []string{"code", "security"}}})
	if p != "developer" {
		t.Errorf("got %q (%s), want developer", p, why)
	}
	if !strings.Contains(why, "first of 2: developer, developer-2") {
		t.Errorf("the roster must be the lane: %q", why)
	}
	if strings.Contains(why, "watcher") {
		t.Errorf("the security PID was never in this race: %q", why)
	}
}

// The pin the bead names, over a whole PASS: every code seat busy, and the
// bead WAITS. The lane-busy line is also the assertion that the lane-busy
// report names the right roster — it is the hiring signal, and naming a seat
// that cannot do the work reads as "the lane is one wider than it is".
func TestQABusyCodeLaneWaitsAndDoesNotSeatTheSecurityPID(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "developer", "[code]")
	writePersona(t, b.App, "developer-2", "[code]")
	writePersona(t, b.App, "watcher", "[security]")
	// a-3 is ranger-base-g8fmb's shape: a code deliverable discovered from a
	// security finding. a-1 and a-2 spend both code seats first.
	qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["code"]},`+
			`{"id":"a-2","title":"u","labels":["code"]},`+
			`{"id":"a-3","title":"v","labels":["code","debt","security","vuln-private"]}]`,
		`[{"id":"x","status":"closed"}]`)
	agentPerLaunch(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 2 {
		t.Errorf("a two-seat lane fires two, got %d:\n%s", n, out)
	}
	if strings.Contains(out, "watcher") {
		t.Errorf("the security PID must not be seated or counted on a code bead:\n%s", out)
	}
	if !strings.Contains(out, "code lane busy: developer, developer-2") {
		t.Errorf("a-3 must wait on the code lane, named as the two seats it has:\n%s", out)
	}
}

// Which label wins is `route_order:` then persona name — the dial ADR 0011 §4
// already states, and the only one. Narrowing makes that choice decide the
// whole lane rather than only its head, so the dial is pinned here: an
// instance whose security PID must take a `[code, security]` bead says so with
// the key, and gets a lane of its own seats rather than a merged one.
func TestQARouteOrderPicksWhichLabelWinsTheBead(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeOrderedPersona(t, b.App, "developer", "[code]", "")
	writeOrderedPersona(t, b.App, "developer-2", "[code]", "")
	writeOrderedPersona(t, b.App, "watcher", "[security]", "10")

	names, label := laneOf(t, d, "code", "security")
	if label != "security" || strings.Join(names, ",") != "watcher" {
		t.Errorf("route_order must decide the winning label: got %v (%s)", names, label)
	}
}

// The narrowing reads the PID's whole matching set, not the one label the
// walk happened to record first. A PID carrying both `qa` and `code` matches
// on whichever it LISTS first; dropping it from a code lane for that would be
// an ordering accident inside a PID's frontmatter, which is nobody's decision.
func TestQALaneKeepsAPIDThatCarriesTheWinningLabelSecond(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "developer", "[code]")
	// Lists `qa` first, so the walk records `qa` — and the bead carries it.
	writePersona(t, b.App, "developer-2", "[qa, code]")
	writePersona(t, b.App, "watcher", "[security]")

	names, label := laneOf(t, d, "code", "qa", "security")
	if label != "code" || strings.Join(names, ",") != "developer,developer-2" {
		t.Errorf("a PID that carries the winning label is a seat wherever it lists it: got %v (%s)", names, label)
	}
}
