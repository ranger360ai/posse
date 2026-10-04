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
func narrowFixture(t *testing.T, ready, list, show string, crewDirIsRepo bool, crewAgent string, crew bool) (string, int, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	// scout exists so a session may name it, but claims no label this
	// fixture's bead carries — it must never compete for the work.
	writePersona(t, b.App, "scout", "[rust]")
	repo := claimedRepo(t, b.App, ready, list, show)
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
		`[]`,
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
		`[]`,
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
		`[]`,
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
		`[]`,
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
		`[]`,
		`[{"id":"a-1","title":"t","status":"open","assignee":"ranger"}]`,
		true, "ranger", true)
	if strings.Contains(out, orphanPark) || n != 1 {
		t.Errorf("a READY bead assigned to the persona must dispatch, not park, n=%d:\n%s", n, out)
	}
}

// is.Assignee == persona. An in_progress row nobody is assigned is not this
// persona's orphaned claim, and their live conversation is no reason to
// leave it standing.
//
// ASKED OF LaunchBead, not of a pass (ranger-base-bknod). ADR 0030 names two
// launchers and both carry the conjunction — fireLoop's copy at
// dispatch.go:2978, the cockpit's `d` at dispatch.go:4992 — and only one of
// them can still be HANDED an unassigned claim. A pass gets its claimed
// beads from interruptedRuns, whose heldLane rule drops a row with no
// assignee before the loop sees it (interrupted.go), and `bd ready` never
// carried an in_progress row at all; so the fixture this arm used to stand
// on — an unassigned in_progress row in fake-ready.json — was a queue no
// store can produce, and with the fake honest it dispatches nothing and the
// arm reads green either way. The cockpit's `d` takes the row the IN
// PROGRESS section displays, which is `bd list --status in_progress` whole
// (Bd.InProgressAll), unassigned rows included — an unclaim under a live
// run leaves exactly one — so that is where the condition is still
// load-bearing and still mutable.
//
// WRONG-ARM, same as its siblings: the refusal must be ABSENT. Dropping
// `is.Assignee == persona` from the cockpit's conjunction makes `d` refuse
// this launch with the orphaned-claim line, and this arm reds.
func TestQAOrphanedClaimDoesNotParkAClaimThisPersonaDoesNotHold(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	// The operator's conversation is live in the repo — the presence half of
	// the conjunction, so the only thing left refusing is the assignee test.
	repo := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "ranger-adhoc", Dir: repo, Agent: "ranger", Crew: true})
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	d := newTestDispatcher(t, b)
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t",
		Labels: []string{"go"}, Status: "in_progress"}, Dir: repo}
	session, err := d.LaunchBead(is)
	if err != nil && strings.Contains(err.Error(), orphanPark) {
		t.Fatalf("an unassigned in_progress row parked on this persona's chat: %v", err)
	}
	if err != nil {
		t.Fatalf("LaunchBead refused an unassigned claim for some other reason: %v", err)
	}
	if session == "" {
		t.Error("LaunchBead reported no session for a launch it did not refuse")
	}
	// The control that makes the absence evidence: the same crew session,
	// the same repo, one dial moved — the row is assigned to the persona —
	// and the refusal DOES fire. Without it a LaunchBead that refuses
	// nothing would satisfy the arm above.
	held := RepoIssue{BdIssue: BdIssue{ID: "a-2", Title: "t",
		Labels: []string{"go"}, Status: "in_progress", Assignee: "ranger"}, Dir: repo}
	if _, err := d.LaunchBead(held); err == nil || !strings.Contains(err.Error(), orphanPark) {
		t.Errorf("the control must park — the arm above is vacuous otherwise, got %v", err)
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
	repo := claimedRepo(t, b.App, `[]`,
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
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if strings.Contains(out, orphanPark) {
		t.Errorf("the crew walk voted against a holder the record already named:\n%s", out)
	}
	// Acted ON, not merely NAMED. `calls` already carries the holder's name
	// from this fixture's own `workspace create`, so a grep for it is
	// satisfied by the setup and says nothing about the pass — which is how
	// this arm stayed green over a queue that never offered the bead at all
	// (ranger-base-bknod). What the pass has to have done is re-prompt it.
	if n != 1 {
		t.Errorf("the evidenced holder was never resumed, n=%d:\n%s", n, out)
	}
	if log := calls(t, fake); !strings.Contains(log, "agent prompt") {
		t.Errorf("the evidenced holder %s was named but never prompted:\n%s", held, log)
	}
	// No twin either. Counted rather than matched by label: the Dial F name
	// has the slot name as a strict PREFIX (ranger-003-a-1 / ranger-003), so
	// a `--label `+slot grep is satisfied by the fixture's own create of the
	// holder. Two creates are this fixture's; a third would be the pass's.
	if log := calls(t, fake); strings.Count(log, "workspace create ") != 2 {
		t.Errorf("the pass built a session beside the evidenced holder %s (%d creates, want the fixture's 2):\n%s",
			held, strings.Count(log, "workspace create "), log)
	}
}
