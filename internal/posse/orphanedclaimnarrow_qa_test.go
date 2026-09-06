//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0030 §1's tiebreak is a CONJUNCTION, and ranger-base-uco3m measured
// that none of its parts was pinned on its own. The close's four arms
// (orphanedclaim_test.go) all stand on the same fixture — an in_progress
// bead assigned to the persona, no holder, a crew session of that persona
// live in that repo — so every one of them stays green when a condition is
// DELETED and the gate only gets wider. Six mutants survived them:
//
//	drop s.Crew            (CrewHolder)  — a fleet session parks the bead
//	drop s.Checkout()==dir (CrewHolder)  — a chat in ANOTHER repo parks it
//	drop s.Agent==persona  (CrewHolder)  — anyone's chat parks it
//	drop is.Status         (fireLoop)    — a READY assigned bead parks
//	drop is.Assignee       (fireLoop)    — someone else's claim parks
//	drop holder==""        (fireLoop)    — a bead with a live holder parks
//
// Each is a way ADR 0030 §2 ("ready beads and evidenced runs are untouched")
// stops holding while every existing arm still passes. These six arms are
// the other side of the conjunction: the gate DECLINING to park, one
// condition at a time. They are wrong-arm tests by construction — each
// asserts the park line is absent where the close's arms assert it present.
//
// One helper, six fixtures: the only thing that varies is the condition
// under test, so a mutant that widens the gate reds exactly the arm that
// names it and no other.

import (
	"strings"
	"testing"
)

// narrowFixture builds the close's own shape with one dial moved. It
// returns the dispatcher's transcript and how many beads it dispatched.
func narrowFixture(t *testing.T, ready, show string, crewDirIsRepo bool, crewAgent string, crew bool) (string, int, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	// scout exists so a session may name it, but claims no label this
	// fixture's bead carries — it must never compete for the work.
	writePersona(t, b.App, "scout", "[rust]")
	repo := qaRepo(t, b.App, ready, show)
	crewDir := repo
	if !crewDirIsRepo {
		crewDir = t.TempDir()
	}
	mustCreate(t, b, NewSessionOpts{Name: "ranger-adhoc", Dir: crewDir, Agent: crewAgent, Crew: crew})
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	d := newTestDispatcher(t, b)
	d.Resume = true
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	return dispatcherOut(d), n, repo
}

const orphanPark = "no session posse started"

// The close's fixture, restated here so these six arms are measured against
// a rig that CAN produce the park line (rig-must-be-shown-able-to-fail): if
// this one ever stops parking, every "must not park" arm below is vacuous.
func TestQANarrowControlStillParksTheOrphanedClaim(t *testing.T) {
	t.Parallel()
	out, n, _ := narrowFixture(t,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`,
		true, "ranger", true)
	if n != 0 || !strings.Contains(out, orphanPark) {
		t.Fatalf("the control must park — every arm below is vacuous otherwise. n=%d:\n%s", n, out)
	}
}

// s.Checkout() == dir. The operator's conversation about ANOTHER repo says
// nothing about this bead; parking on it would stop a persona's whole queue
// for a chat in an unrelated tree.
func TestQAOrphanedClaimIgnoresACrewSessionInAnotherRepo(t *testing.T) {
	t.Parallel()
	out, n, _ := narrowFixture(t,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`,
		false, "ranger", true)
	if strings.Contains(out, orphanPark) || n != 1 {
		t.Errorf("a crew session in another repo must not park this bead, n=%d:\n%s", n, out)
	}
}

// s.Crew. Crew marking is what makes a session "the operator's" (ADR 0008
// §1); an unmarked fleet session is posse's own and is no reason to park.
func TestQAOrphanedClaimIgnoresANonCrewSessionInTheSameRepo(t *testing.T) {
	t.Parallel()
	out, n, _ := narrowFixture(t,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`,
		true, "ranger", false)
	if strings.Contains(out, orphanPark) || n != 1 {
		t.Errorf("an unmarked (non-crew) session must not park this bead, n=%d:\n%s", n, out)
	}
}

// s.Agent == persona. ADR 0030 asks whether THIS persona's assignee is at
// the keyboard; another persona's conversation is not an answer to it.
func TestQAOrphanedClaimIgnoresAnotherPersonasCrewSession(t *testing.T) {
	t.Parallel()
	out, n, _ := narrowFixture(t,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`,
		true, "scout", true)
	if strings.Contains(out, orphanPark) || n != 1 {
		t.Errorf("another persona's crew session must not park this bead, n=%d:\n%s", n, out)
	}
}

// is.Status == "in_progress". §2 stands in the sharper shape the close's own
// arm 3 could not measure: its ready bead carries NO assignee, so dropping
// the status test alone leaves the assignee test to refuse the gate and the
// mutant lives. This bead is ready AND assigned to the persona.
func TestQAOrphanedClaimDoesNotParkAReadyBeadAssignedToThePersona(t *testing.T) {
	t.Parallel()
	out, n, _ := narrowFixture(t,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger"}]`,
		`[{"id":"a-1","title":"t","status":"open","assignee":"ranger"}]`,
		true, "ranger", true)
	if strings.Contains(out, orphanPark) || n != 1 {
		t.Errorf("a READY bead assigned to the persona must dispatch, not park, n=%d:\n%s", n, out)
	}
}

// is.Assignee == persona. An in_progress row nobody is assigned is not this
// persona's orphaned claim, and their live conversation is no reason to
// leave it standing.
func TestQAOrphanedClaimDoesNotParkAClaimThisPersonaDoesNotHold(t *testing.T) {
	t.Parallel()
	out, n, _ := narrowFixture(t,
		`[{"id":"a-1","title":"t","labels":["go"],"status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress"}]`,
		true, "ranger", true)
	if strings.Contains(out, orphanPark) || n != 1 {
		t.Errorf("an unassigned in_progress row must not park on this persona's chat, n=%d:\n%s", n, out)
	}
}

// holder == "". ADR 0030 is explicit that the crew walk is "presence
// consulted at ambiguity, never against a fact": a bead whose own Dial F
// session is live has already been answered, and the crew session must not
// get a second, contradicting vote.
func TestQAOrphanedClaimNeverOverridesAnEvidencedHolder(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	// The bead's OWN session, under the Dial F name heldSession resolves —
	// this is the "evidenced run" §2 promises is untouched.
	held := SessionForBead("ranger", repo, "a-1")
	mustCreate(t, b, NewSessionOpts{Name: held, Dir: repo, Agent: "ranger"})
	// ...and the operator's conversation live beside it.
	mustCreate(t, b, NewSessionOpts{Name: "ranger-adhoc", Dir: repo, Agent: "ranger", Crew: true})
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	d := newTestDispatcher(t, b)
	d.Resume = true
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if strings.Contains(out, orphanPark) {
		t.Errorf("the crew walk voted against a holder the record already named:\n%s", out)
	}
	if log := calls(t, fake); !strings.Contains(log, held) {
		t.Errorf("the evidenced holder %s was never acted on:\n%s", held, log)
	}
}
