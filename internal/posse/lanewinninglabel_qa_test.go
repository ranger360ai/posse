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
// (dispatch.go, `labelMatches`: `win` is the head's anchor, which on a head
// that lists its own lane label and matches it IS that first label). The test
// above is the other half — a
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
// THAT REMEDY IS THIS ARM'S AND NOT THE WHOLE SHAPE'S (ranger-base-bskno,
// decided at ranger-base-d7lsc). The order is a remedy here because arm C's
// head matches on TWO labels, so there is something to order — and under the
// anchor rule it still is: this head lists `team` FIRST, so `team` is the lane
// it DECLARES, it anchors on it, and arm C routes exactly as it did. There is
// a second route to the same lane where there is nothing to order: a head
// whose `matched` set holds exactly ONE element, and that element neither its
// own first label nor a single lane's. That head took the label as the lane
// whatever order any PID listed its labels in, and where the element was the
// cross-cutting label the lane held no seat that could do the work at all.
// §4 did not cover it and this arm's fixture cannot reach it. It is now
// decided as code: such a candidate does not head.
// TestQAHeadAnchorsOnItsOwnLaneOrASingleLaneLabel below is that rule, and
// carries this arm's shape as a control.
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
// ranger-base-d7lsc — A CANDIDATE HEADS ONLY ON AN ANCHOR, so the capable
// seat is the lane on the route no order reaches.
//
// THE SHAPE (found as ranger-base-bskno, characterized there over the
// behaviour this table replaces). Arm C above is ADR 0011 §4's stated
// configuration error: a PID that LISTS a cross-cutting label first makes
// that label its lane, and §4's stated remedy is the order. That remedy
// reaches arm C, where the head's `matched` set has two elements and the
// order really does decide. It did not reach this one. When the head
// candidate's `matched` set held exactly ONE element and that element was the
// cross-cutting label, that label was the lane whatever order any PID listed
// its labels in — there was nothing to order. The roster below has §4's
// remedy already applied everywhere: every PID lists its own lane label
// first, as RE-MEASURED 2026-11-14 all 11 of this instance's PIDs do (and as
// 20 of 20 did on 2026-11-13, counting the shipped examples), and the
// misroute still happened.
//
// Sharper than arm C, and sharper than ranger-base-equhp's incident. equhp
// was a lane that SOMETIMES reached a seat which could only hand the bead
// back. There the lane held no seat that could do the deliverable at all:
// `ops-seat` is the only devops seat on the roster and `watcher` the only
// security seat, and neither was in its own deliverable's lane, so the bead
// could not be seated on a capable seat on any pass.
//
// THE RULE, decided at ranger-base-d7lsc and written into ADR 0011 §4 as
// amended 2026-11-14 (commit 8a0a93d1), built in `labelMatches` under
// ranger-base-5ex01. A label's LANES are the first labels of every
// non-coordinator PID carrying it. A candidate's ANCHOR is the first of its
// matched labels, in its PID's order, that is its own first label or belongs
// to exactly one lane. The head is the first candidate in the `route_order`
// sort that HAS an anchor, and the lane is that anchor. Seats are unchanged:
// every candidate carrying the lane label. Where no candidate has an anchor,
// out[0]'s head label is the lane as before.
//
// WHAT THIS TABLE HOLDS, row by row — the two stranded rows now seat the
// capable PID, and everything the rule must NOT move is a control beside
// them:
//
//   - rows 1-4: the finding's two rows, over the head orders where the head
//     does not declare the shared label. `aaa-dev` matches the bead on
//     `team` alone, which is neither its own lane nor one lane's, so it does
//     not head; the only devops seat heads a devops bead and the only
//     security seat a security one. These are the rows that moved.
//   - row 5: the FIRST configuration error, which keeps the remedy it has
//     (§4, ranger-base-svoux). A head that lists `team` first DECLARED that
//     as its lane, so it anchors on it and the lane is `team` — arm C's
//     shape reached through a one-element matched set, and still an error
//     whose remedy is the PID's order and not code.
//   - row 6: nothing to anchor on. A bead carrying ONLY the shared label has
//     no capable-seat answer to find, so the rule leaves it where it was: a
//     `team` lane of the two carriers.
//   - rows 7-8: the head's own lane still wins when it matches it, with and
//     without another lane's label on the bead.
//   - row 9: the ALIAS arm, and the reason primary-first was rejected
//     (ranger-base-d4bk4's shape: a code fix labelled `[bug, security]`).
//     `bug` is not `aaa-dev`'s declared lane, but every PID carrying `bug`
//     is in the code lane, so matching it is a code match and the fix stays
//     with the code seat. Primary-first would hand it to the findings-only
//     security PID, which is equhp's own incident.
//
// MEASURED 2026-11-14 (linux, scratch census over `bd list --all --json
// --limit 6000` and this instance's 10 non-coordinator PIDs): 3,856 beads,
// 3,474 with a candidate, and the anchor changes 0 of those routes.
// Primary-first changes 2. Exactly one label on the live roster spans more
// than one lane and 0 beads carry it, so rows 1-5 are a shape the roster
// permits and no bead has yet reached.
//
// THE DELIBERATE RED. This table is the rewrite ranger-base-bskno's
// characterization pin asked for — same roster, the lane HOLDING the seat
// that can do the work. It reds if `win` reverts to `out[0].label`: rows 1-4
// read `aaa-dev,zz-arch` (`team`) under the old body, which is the measured
// before-state, and `capable` is asserted by name so the message says which
// seat went missing rather than only that a set differed. VERIFIED by that
// mutation, 2026-11-14 (ranger-base-5ex01).
//
// `team` is §4's spelling for the real label several of this instance's PIDs
// share — the instance's own vocabulary, named on the bead and not here (ADR
// 0024 D3, ADR 0048 D2; the code and architecture lanes were each refused by
// the visibility gate for the real literal under ranger-base-5793u and
// ranger-base-svoux).
func TestQAHeadAnchorsOnItsOwnLaneOrASingleLaneLabel(t *testing.T) {
	t.Parallel()
	// head is aaa-dev's own labels. The default lists its lane label first,
	// which is §4's remedy already applied; the reordered arms are the proof
	// that the anchor does not need an order to be the remedy.
	const headLaneFirst = "[code, feature, bug, team]"
	for _, arm := range []struct {
		name     string
		head     string
		bead     []string
		want     string
		label    string
		capable  string // must be IN the lane, by name
		stranded string // must NOT be in the lane, by name
		solo     string // the lane for bead[0] alone; "" skips the control
	}{
		{
			"1 a devops deliverable heads on the only devops seat",
			headLaneFirst, []string{"devops", "team"},
			"ops-seat", "devops", "ops-seat", "", "ops-seat",
		},
		{
			"2 a security deliverable heads on the only security seat",
			headLaneFirst, []string{"security", "team"},
			"watcher", "security", "watcher", "", "watcher",
		},
		{
			"3 the head's order is not the remedy: shared label last, lane label second",
			"[bug, code, feature, team]", []string{"devops", "team"},
			"ops-seat", "devops", "ops-seat", "", "ops-seat",
		},
		{
			"4 the same, on the security deliverable",
			"[bug, code, feature, team]", []string{"security", "team"},
			"watcher", "security", "watcher", "", "watcher",
		},
		{
			"5 control: a head that lists the shared label FIRST declared it, and anchors on it",
			"[team, code, feature, bug]", []string{"devops", "team"},
			"aaa-dev,zz-arch", "team", "", "ops-seat", "ops-seat",
		},
		{
			"6 control: nothing to anchor on, so the shared label is still the lane",
			headLaneFirst, []string{"team"},
			"aaa-dev,zz-arch", "team", "", "", "",
		},
		{
			"7 control: the head's own lane wins when the bead carries it",
			headLaneFirst, []string{"code", "team"},
			"aaa-dev", "code", "aaa-dev", "zz-arch", "",
		},
		{
			"8 control: [code, security] is the code lane",
			headLaneFirst, []string{"code", "security"},
			"aaa-dev", "code", "aaa-dev", "watcher", "",
		},
		{
			"9 control, the alias arm: a [bug, security] code fix stays with the code seat",
			headLaneFirst, []string{"bug", "security"},
			"aaa-dev", "bug", "aaa-dev", "watcher", "",
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
				t.Errorf("lane is %v (%s), want %s (%s) — a candidate heads only on an anchor: the first matched label that is its own first label or belongs to one lane (ADR 0011 §4, ranger-base-d7lsc)",
					names, label, arm.want, arm.label)
			}
			seated := false
			for _, n := range names {
				if n == arm.capable {
					seated = true
				}
				if arm.stranded != "" && n == arm.stranded {
					t.Fatalf("%s is in the lane and must not be: %v (%s)", arm.stranded, names, label)
				}
			}
			if arm.capable != "" && !seated {
				t.Fatalf("%s is not in the lane — the seat that can do this deliverable must be the one that heads it: %v (%s)",
					arm.capable, names, label)
			}

			// The control ranger-base-bskno's pin carried, kept: the
			// narrowing never stranded the dropped seat's own LABEL, only
			// the deliverable that also carried the shared one. Still true,
			// and on rows 1-4 it is now the same lane either way.
			if arm.solo != "" {
				solo, soloLabel := laneOf(t, d, arm.bead[0])
				if strings.Join(solo, ",") != arm.solo || soloLabel != arm.bead[0] {
					t.Errorf("%s lane regressed: got %v (%s), want %s (%s)",
						arm.bead[0], solo, soloLabel, arm.solo, arm.bead[0])
				}
			}
		})
	}
}

// The lane's label is the ANCHOR, and the anchor can belong to a candidate
// the sort does not offer first. `aaa-dev` matches this bead on `team` and
// `docs`, neither its own lane nor one lane's, so it does not head; `zz-docs`
// heads on `docs`, which IS its own lane. Both carry `docs`, so both are
// seats and `aaa-dev` is still the preferred one — but its own head label is
// `team`, and `team` is not what this lane was chosen on.
//
// Pinned because `routeLane.label` and `routeWhy` read that label off
// `seats[0]` until ranger-base-5ex01, where they could only ever agree: the
// winning label WAS out[0]'s own. Under the anchor they can differ, and a
// route report naming a label the lane was not chosen on is the same defect
// ranger-base-2yj5 built the why line to answer — "why did that persona get
// this bead", answered with the wrong label.
func TestQALanesLabelIsTheAnchorNotThePreferredSeatsOwnLabel(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "aaa-dev", "[code, team, docs]")
	writePersona(t, b.App, "zz-docs", "[docs, team]")

	names, label := laneOf(t, d, "docs", "team")
	if strings.Join(names, ",") != "aaa-dev,zz-docs" {
		t.Errorf("both PIDs carry the anchor, so both are seats: got %v (%s)", names, label)
	}
	if label != "docs" {
		t.Errorf("the lane's label is the anchor the head was chosen on, got %q — aaa-dev's own head label is `team`", label)
	}

	p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-1", Labels: []string{"docs", "team"}}})
	if p != "aaa-dev" {
		t.Errorf("the preferred seat is unchanged by which candidate anchored: got %q (%s)", p, why)
	}
	if !strings.Contains(why, "label:docs (first of 2: aaa-dev, zz-docs)") {
		t.Errorf("the route report must name the label the lane was chosen on: %q", why)
	}
}
