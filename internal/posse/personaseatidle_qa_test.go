package posse

// ranger-base-ajpbe: an agent that settles idle mid-bead with a suite still
// running was offered as a free seat. ranger-base-25cit (fixed by 900b5c35)
// closed one door onto this shape — the busy MAP releasing a hold a settle
// merely judged for the seat's NAME, freeing a bead another launch held.
// This is a different door onto the same shape, and 25cit's fix does
// nothing about it.
//
// personaActive is the live-herdr read seatFor asks when deciding whether a
// DIFFERENT bead may seat onto a persona already in this lane — never the
// bead that persona is holding, always a second one. It counted only
// agent_status working/blocked as busy, so an idle session still holding
// its OWN in_progress bead read as an empty seat, and a second bead was
// routed onto the same persona: two live sessions on one seat.
//
// MEASURED 2026-09-07 on a two-seat QA lane, on the binary carrying
// 25cit's fix: one seat's dispatched session read agent_status idle behind
// "1 shell, 1 monitor still running" with its own bead still in_progress,
// and a second ready bead was fired into that same seat within the hour —
// two live sessions holding the lane's one seat for that persona.
//
// No PaneHold read is needed here, unlike ranger-base-htafy's three sites
// (gather's settle-open, the --resume skip, govern's G2 row): those ask
// whether THIS bead's own idle holder should be re-prompted, and an unsent
// composer or a live shell is the difference between a nudge and a garbled
// second message. This asks whether a DIFFERENT bead may have the seat at
// all, and the answer is no the moment the session's own claim — the bead
// it was dispatched to work — is still in_progress, whatever the pane is
// doing.

import (
	"strings"
	"testing"
)

// The bead's own shape: two QA seats, scout genuinely working so the walk
// reaches verifier, and verifier's own session idle behind its bead — which
// is still in_progress. The second ready bead must wait for the lane, not
// seat onto verifier beside the session already holding a-1.
func TestQAIdleHolderWithLiveBeadIsNotAFreeSeat(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "scout", "[qa]")
	writePersona(t, b.App, "verifier", "[qa]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["qa"],"assignee":"verifier","status":"in_progress"},`+
			`{"id":"a-2","title":"u","labels":["qa"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"verifier"}]`)
	agentPerLaunch(t, fake)

	scoutSlot := SessionFor("scout", repo)
	mustCreate(t, b, NewSessionOpts{Name: scoutSlot, Dir: repo, Agent: "scout"})
	verifierDial := SessionForBead("verifier", repo, "a-1")
	mustCreate(t, b, NewSessionOpts{Name: verifierDial, Dir: repo, Agent: "verifier", Bead: "a-1"})
	// setAgentStatus (sessionAgent's own call) rewrites the whole agents.json
	// listing, so two live sessions in one fixture must be set together or
	// the second call wipes the first's row out from under it.
	twoAgentSeats(t, fake, scoutSlot, "working", verifierDial, "idle")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out, log := dispatcherOut(d), calls(t, fake)
	if want := "workspace create --label " + SessionForBead("verifier", repo, "a-2"); strings.Contains(log, want) {
		t.Errorf("a second verifier session was fired beside the one holding a-1:\n%s\n%s", out, log)
	}
	if n != 0 {
		t.Errorf("the lane has no free seat this pass, want n=0, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "qa lane busy: scout, verifier") {
		t.Errorf("the second bead must see the whole lane busy, scout AND verifier:\n%s", out)
	}
}

// twoAgentSeats arms both sessions' herdr status in a single agents.json
// write, so neither `sessionAgent` call clobbers the other's row.
func twoAgentSeats(t *testing.T, fake, name1, status1, name2, status2 string) {
	t.Helper()
	ws := fakeLoadWSFrom(t, fake)
	var id1, id2 string
	for _, w := range ws {
		switch w.Label {
		case name1:
			id1 = w.WorkspaceID
		case name2:
			id2 = w.WorkspaceID
		}
	}
	if id1 == "" || id2 == "" {
		t.Fatalf("workspace lookup failed: %s=%q %s=%q (%+v)", name1, id1, name2, id2, ws)
	}
	setAgentStatuses(t, fake, agentState{id1, status1}, agentState{id2, status2})
}

// THE CONTROL, and the reason the fix above is a measurement and not a
// tautology: the same idle verifier session with its OWN bead closed is
// the ordinary Dial F case this must not catch — the session is
// verifier's to reuse or reap, and this seat is free for the next bead.
func TestQAIdleHolderWithClosedBeadIsStillAFreeSeat(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "scout", "[qa]")
	writePersona(t, b.App, "verifier", "[qa]")
	// BOTH beads are in the show fixture, a-2 included (ranger-base-s92di):
	// a store that names only a-1 answers `bd show a-2` about a-1 (the
	// fake's stand-in for bd's prefix resolution), and the claim preflight
	// reads that before it writes — correctly refusing to claim against a
	// store that says a-2 is really a-1. The seat, not the store, is what
	// this test is about.
	repo := qaRepo(t, b.App,
		`[{"id":"a-2","title":"u","labels":["qa"]}]`,
		`[{"id":"a-1","title":"t","status":"closed","assignee":"verifier"},
		  {"id":"a-2","title":"u","status":"open"}]`)
	agentPerLaunch(t, fake)

	scoutSlot := SessionFor("scout", repo)
	mustCreate(t, b, NewSessionOpts{Name: scoutSlot, Dir: repo, Agent: "scout"})
	verifierDial := SessionForBead("verifier", repo, "a-1")
	mustCreate(t, b, NewSessionOpts{Name: verifierDial, Dir: repo, Agent: "verifier", Bead: "a-1"})
	// scout busy too, so a-2 can only seat by way of verifier's read.
	twoAgentSeats(t, fake, scoutSlot, "working", verifierDial, "idle")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out, log := dispatcherOut(d), calls(t, fake)
	if n != 1 {
		t.Errorf("verifier's closed-bead session must still read as a free seat, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "qa lane busy") {
		t.Errorf("the control arm must not report the lane busy:\n%s", out)
	}
	if want := "workspace create --label " + SessionForBead("verifier", repo, "a-2"); !strings.Contains(log, want) {
		t.Errorf("a-2 did not seat onto verifier's freed seat:\n%s\n%s", out, log)
	}
}
