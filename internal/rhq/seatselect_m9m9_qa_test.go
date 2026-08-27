package rhq

// ADR 0020 §2 — a lane is a set of labels, a seat is a persona, and
// selection among peers is AVAILABILITY-FIRST at dispatch.
//
// These four started life as holden's green pins asserting the hole
// (ranger-base-69jo, escape from the ranger-base-m9m9 verify of
// ranger-base-2yj5): the fire loop took Route's single head and skipped the
// BEAD when that one persona was busy, so a free seat beside it was never
// offered the work. Each carried its own inversion in its failure message.
// §2 landed, all four went red on the same run, and each is now the
// assertion its own message named. The fifth was never a hole — it is the
// correct half of §2.4, green before and after, and it is what stops §2.4
// widening --persona into "seat it anywhere".

import (
	"strings"
	"testing"
)

// ADR 0020, Consequences, verbatim: "A pass with three unassigned code
// beads and three free code seats fires all three; today it fires one and
// marks the lane's only routable persona busy."
//
// Two beads, two free seats, two fires. And §4 in the same assertion: each
// persona takes exactly ONE of them, so a seat walk that fell through into
// fanning one persona N-wide fails here rather than in review.
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
	if n != 2 {
		t.Errorf("two free seats and two beads must fire two, got %d:\n%s", n, out)
	}
	// The two shapes a busy SKIP can take. "dinesh busy" inside a seat
	// clause is the opposite fact — it is the report saying the walk moved
	// on to a free seat — so match the skip lines, not the word.
	if strings.Contains(out, "lane busy") || strings.Contains(out, "busy this pass") {
		t.Errorf("no bead may report busy while a seat is free:\n%s", out)
	}
	for _, who := range []string{"dinesh", "gwart"} { // ADR 0020 §4: a persona stays strictly serial
		if strings.Count(out, "persona "+who) != 1 {
			t.Errorf("%s must take exactly one bead:\n%s", who, out)
		}
	}
}

// ADR 0020 §2: every seat busy is the LANE being busy — "All seats busy →
// the bead waits for a later pass, and the report names the lane ('code
// lane busy: dinesh, gwart, jian-yang'), not one persona."
//
// The line the single-seat model printed named Route's head, which reads as
// "dinesh IS the code lane". Naming the lane is also the hiring signal ADR
// 0020 §4 turns on: lane concurrency is seat count, so an operator who sees
// the whole lane spent knows the answer is a PID, not a longer wait.
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
	if !strings.Contains(out, "code lane busy") {
		t.Errorf("a full lane must report the lane, not one persona:\n%s", out)
	}
	for _, who := range []string{"dinesh", "gwart"} {
		if !strings.Contains(out, who) {
			t.Errorf("the lane-busy line must name every seat it tried, missing %q:\n%s", who, out)
		}
	}
	if !strings.Contains(out, "code lane busy: dinesh, gwart") {
		t.Errorf("the lane-busy line must name the seats in routing order:\n%s", out)
	}
}

// ADR 0020 §2: "The route report must say why a seat won — 'label:code
// (seat 2/3: gwart; dinesh busy)' — the audit line ranger-base-2yj5 asked
// for."
//
// Route alone cannot see availability, so the seat clause is composed where
// availability is read — inside the fire loop, under the launcher lock —
// and the assertion is over a whole PASS. Route keeps ca0c00e's roster
// clause, which is the honest answer to a question asked with no pass
// running (the cockpit's `d`, `--dry-run` before the loop starts).
func TestQARouteWhyNamesTheRaceNotTheSeat(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "dinesh", "[code]")
	writePersona(t, b.App, "gwart", "[code]")
	writePersona(t, b.App, "jian-yang", "[code]")
	qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["code"]},{"id":"a-2","title":"u","labels":["code"]}]`,
		`[{"id":"x","status":"closed"}]`)
	agentPerLaunch(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "seat 1/3") || !strings.Contains(out, "seat 2/3") {
		t.Errorf("the pass report must say which seat took each bead:\n%s", out)
	}
	if !strings.Contains(out, "dinesh busy") {
		t.Errorf("the seat clause must say why the earlier seat did not take it:\n%s", out)
	}

	// The roster clause ca0c00e landed must not regress: Route is still the
	// single-answer API both launchers share, and it answers about the race.
	p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-1", Labels: []string{"code"}}})
	if p != "dinesh" || !strings.Contains(why, "first of 3: dinesh, gwart, jian-yang") {
		t.Errorf("Route's roster clause regressed: got %q (%s)", p, why)
	}
}

// ADR 0020 §2: "--persona X restricts seating to X: a bead whose lane
// contains X may seat only there, others are skipped as today."
//
// The bug this pins was the SILENCE: the bead was dropped at
// `persona != personaFilter` before any line was printed, so `posse
// dispatch --persona gwart` reported NOTHING while gwart sat idle beside
// ready work in gwart's own lane.
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
	if n != 1 {
		t.Errorf("gwart is in a-1's lane, so --persona gwart must seat it there, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "gwart") {
		t.Errorf("--persona must never be silent about a bead in that persona's lane:\n%s", out)
	}
}

// Not a hole — the correct half of §2.4, pinned green so building the other
// half cannot widen --persona into "seat it anywhere". A bead whose lane
// EXCLUDES the filtered persona stays skipped. It gets no line of its own
// (one per filtered-out bead would bury the ones that matter), but the pass
// says at the end how many there were, so a filtered pass that dispatches
// nothing can still be told from an empty queue.
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
	out := dispatcherOut(d)
	if n != 0 {
		t.Errorf("gilfoyle is not in a-1's lane; --persona gilfoyle must dispatch nothing, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "a-1") {
		t.Errorf("a bead outside the filtered persona's lane gets no line of its own:\n%s", out)
	}
	if !strings.Contains(out, "outside gilfoyle's lane") {
		t.Errorf("a filtered pass must say a ready bead was filtered out, not go silent:\n%s", out)
	}
}
