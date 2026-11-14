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
// on whichever it LISTS first; dropping it from a code lane for that would
// read a FOLLOWER's frontmatter order as a decision, and a follower's order
// decides nothing (ADR 0011 §4, ranger-base-svoux). That order is a key at
// the HEAD of the candidates only, where it names that PID's own lane —
// TestQALaneWinningLabelTurnsOnTheHeadPIDsFrontmatterOrder below.
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

// ---------------------------------------------------------------------------
// ranger-base-5793u — the head PID's label order IS the second routing key.
//
// DECIDED, ADR 0011 §4 (ranger-base-svoux, commit 70ab74b7). A PID lists its
// PRIMARY lane first, and two keys decide a lane, both written by a human in
// a reviewed file: `route_order:` decides WHO heads the candidates, and the
// head's own label order decides WHICH of its labels is the lane
// (dispatch.go, `win := out[0].label`). The test above is the other half — a
// FOLLOWER's order decides nothing, it is a seat wherever it carries the
// winning label. Since ranger-base-equhp's narrowing this key decides
// MEMBERSHIP and not only the label the route report prints, which is why it
// is pinned rather than left to the reader.
//
// The three arms differ in ONE input — the head PID's two labels, swapped (A
// vs B) or a cross-cutting label listed first (C). Bead and roster are
// otherwise identical.
//
// A and B are MEANT to differ: that difference is the dial. C is the stated
// configuration error §4 names — a PID that lists a cross-cutting label first
// makes that label its lane and every PID carrying it a seat, so a code
// deliverable's lane holds the findings-only security PID and not the second
// code seat, which is equhp's own incident shape reached the other way. Its
// remedy is the PID's order, not code: a cross-cutting label and a
// legitimately shared lane label are spelled alike, so no static rule can
// refuse it, and it shows at run time in the route report's `label:` and
// roster. `team` is §4's spelling for the real label several of this
// instance's PIDs share — the instance's own vocabulary, named on the bead
// and not here (ADR 0024 D3, restate-and-cite).
//
// THAT REMEDY IS THIS ARM'S AND NOT THE WHOLE SHAPE'S (ranger-base-bskno).
// The order is a remedy here because arm C's head matches on TWO labels, so
// there is something to order. There is a second route to the same lane where
// there is not: a head whose `matched` set holds exactly ONE element takes
// that label as the lane whatever order any PID lists its labels in, and when
// that element is the cross-cutting label the lane can hold no seat that can
// do the work at all. §4 does not cover it and this arm's fixture cannot
// reach it. It is measured by
// TestQALaneIsTheCrossCuttingLabelWhenTheHeadMatchesOnlyThat below; its remedy
// is open at ranger-base-d7lsc.
//
// The bead labels are passed ALPHABETICALLY because that is the only order
// routing ever sees: MEASURED 2026-11-13, bd normalizes a bead's labels to
// alphabetical order at create — 689 of 689 multi-label beads in this
// instance's store are sorted, and two beads whose `bd create` typed the
// other order (`-l risk -l question`, `-l devops -l debt`) read back
// ['question','risk'] and ['debt','devops']. `BdIssue.Labels` is decoded
// straight from that JSON (beads.go:492). That is why §4 rejected taking the
// winning label from the BEAD's order: it would be the alphabet and not the
// filer. docs/notes.d/ranger-base-5793u.md.
func TestQALaneWinningLabelTurnsOnTheHeadPIDsFrontmatterOrder(t *testing.T) {
	t.Parallel()
	for _, arm := range []struct {
		name     string
		personas [][2]string
		bead     []string
		want     string
		label    string
	}{
		{
			"A head lists code first",
			[][2]string{{"developer", "[code, qa]"}, {"developer-2", "[code]"}, {"tester", "[qa]"}},
			[]string{"code", "qa"}, "developer,developer-2", "code",
		},
		{
			"B same roster, head's two labels swapped",
			[][2]string{{"developer", "[qa, code]"}, {"developer-2", "[code]"}, {"tester", "[qa]"}},
			[]string{"code", "qa"}, "developer,tester", "qa",
		},
		{
			"C the stated configuration error: a cross-cutting label listed first",
			[][2]string{
				{"developer", "[team, code, feature, bug]"},
				{"developer-2", "[code, feature, bug]"},
				{"watcher", "[security, audit, team]"},
			},
			[]string{"code", "team"}, "developer,watcher", "team",
		},
	} {
		t.Run(arm.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			for _, p := range arm.personas {
				writePersona(t, b.App, p[0], p[1])
			}
			names, label := laneOf(t, d, arm.bead...)
			if strings.Join(names, ",") != arm.want || label != arm.label {
				t.Errorf("lane is %v (%s), want %s (%s) — the head PID's label order is the lane (ADR 0011 §4); arm C is the stated configuration error, not a regression",
					names, label, arm.want, arm.label)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ranger-base-bskno — the SECOND route to a cross-cutting lane, and this one
// no order reaches.
//
// Arm C above is §4's stated configuration error: a PID that LISTS a
// cross-cutting label first makes that label its lane, and §4's stated remedy
// is the order. That remedy reaches arm C, where the head's `matched` set has
// two elements and the order really does decide. It does not reach this test.
// When the head candidate's `matched` set holds exactly ONE element and that
// element is the cross-cutting label, that label is the lane whatever order
// any PID lists its labels in — there is nothing to order. The roster below
// has §4's remedy already applied everywhere: every PID lists its own lane
// label first, as RE-MEASURED 2026-11-14 all 11 of this instance's PIDs do
// (and as 20 of 20 did on 2026-11-13, counting the shipped examples), and the
// misroute still happens.
//
// Sharper than arm C, and sharper than ranger-base-equhp's incident. equhp
// was a lane that SOMETIMES reached a seat which could only hand the bead
// back. Here the lane holds no seat that can do the deliverable at all:
// `ops-seat` is the only devops seat on the roster and `watcher` the only
// security seat, and neither is in its own deliverable's lane, so the bead
// cannot be seated on a capable seat on any pass.
//
// TWO CONTROLS, MEASURED 2026-11-14 through these same arms by mutating
// `labelMatches` and reverting after the reading:
//
//   - PRE-NARROWING (`return out` for `return lane`): all four arms red, with
//     the lane reading `[aaa-dev ops-seat zz-arch]` and `[aaa-dev watcher
//     zz-arch]`. Before ranger-base-equhp's narrowing the capable seat was
//     always IN the lane and only the printed label was wrong, so the
//     EXCLUSION half is new with it — what ranger-base-5793u established for
//     the frontmatter-order half.
//   - §4'S REJECTED ALTERNATIVE (`win` as the alphabetically first of
//     `out[0].matched`): all four arms green and byte-identical, because a
//     one-element matched set has the same first element in either order.
//
// So this shape is orthogonal to the (a)/(b) choice §4 settled — neither the
// decided rule nor the rejected one reaches it — and it is not a reopen of
// ranger-base-svoux.
//
// CHARACTERIZATION, not a decision — the same posture arm C's table went in
// with under ranger-base-5793u while ranger-base-svoux was out. The remedy is
// with the architecture lane as ranger-base-d7lsc, which carries the three
// shapes: (a) prefer the next candidate whose matched set holds a non-shared
// label, (b) state this as a second configuration error whose remedy is not
// the order, (c) the explicit `lane:` PID key §4 already names as its exit
// hatch. Under (b) this table stands as written. Under (a) or (c) it must be
// rewritten to the table the finding asked for — the same roster, with the
// lane HOLDING the seat that can do the work — and the red is the deliberate
// signal that the behaviour moved.
//
// NOT FIRING ON THIS INSTANCE TODAY: RE-MEASURED 2026-11-14, 0 of 3,851 beads
// in this store carry the cross-cutting label at all (0 of 3,795 a day
// earlier). It is declared on 5 of this instance's 11 PIDs, every one
// of which lists its own lane label first and none of which lists the shared
// one first, and it has never been filed on a bead. There is exactly one
// devops PID and one security PID, so both rows below are the live roster's
// shape; ONE bead carrying the label plus a lane label turns a row on.
//
// `team` is §4's spelling for the real label several of this instance's PIDs
// share — the instance's own vocabulary, named on the bead and not here (ADR
// 0024 D3, ADR 0048 D2; the code and architecture lanes were each refused by
// the visibility gate for the real literal under ranger-base-5793u and
// ranger-base-svoux).
func TestQALaneIsTheCrossCuttingLabelWhenTheHeadMatchesOnlyThat(t *testing.T) {
	t.Parallel()
	// head is aaa-dev's own labels. The default lists its lane label first,
	// which is §4's remedy already applied; the reordered arms are the proof
	// that no order is a remedy here.
	const headLaneFirst = "[code, feature, bug, team]"
	for _, arm := range []struct {
		name     string
		head     string
		bead     []string
		want     string
		label    string
		stranded string
	}{
		{
			"devops deliverable: the only devops seat is not in its lane",
			headLaneFirst, []string{"devops", "team"},
			"aaa-dev,zz-arch", "team", "ops-seat",
		},
		{
			"security deliverable: the only security seat is not in its lane",
			headLaneFirst, []string{"security", "team"},
			"aaa-dev,zz-arch", "team", "watcher",
		},
		{
			"the order is not a remedy: head lists its lane label last",
			"[team, code, feature, bug]", []string{"devops", "team"},
			"aaa-dev,zz-arch", "team", "ops-seat",
		},
		{
			"the order is not a remedy: head's labels alphabetical",
			"[bug, code, feature, team]", []string{"devops", "team"},
			"aaa-dev,zz-arch", "team", "ops-seat",
		},
	} {
		t.Run(arm.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			writePersona(t, b.App, "aaa-dev", arm.head)
			writePersona(t, b.App, "ops-seat", "[devops, infra, ci]")
			writePersona(t, b.App, "watcher", "[security, audit, vuln]")
			writePersona(t, b.App, "zz-arch", "[architecture, design, team]")

			names, label := laneOf(t, d, arm.bead...)
			if strings.Join(names, ",") != arm.want || label != arm.label {
				t.Errorf("lane is %v (%s), want %s (%s) — the head's matched set is the cross-cutting label alone, so that label is the lane and no PID's order moves it (ranger-base-bskno, remedy open at ranger-base-d7lsc)",
					names, label, arm.want, arm.label)
			}
			for _, n := range names {
				if n == arm.stranded {
					t.Fatalf("%s is in the lane now — the behaviour moved, which is what ranger-base-d7lsc's (a)/(c) would do; rewrite this table deliberately: %v (%s)",
						arm.stranded, names, label)
				}
			}

			// The control: the narrowing has not stranded the dropped seat's
			// own label. Its lane is its own, and only the deliverable that
			// ALSO carries the cross-cutting label loses it.
			solo, soloLabel := laneOf(t, d, arm.bead[0])
			if strings.Join(solo, ",") != arm.stranded || soloLabel != arm.bead[0] {
				t.Errorf("%s lane regressed: got %v (%s), want %s (%s)",
					arm.bead[0], solo, soloLabel, arm.stranded, arm.bead[0])
			}
		})
	}
}
