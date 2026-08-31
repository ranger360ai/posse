package posse

// ADR 0020 §2, amended 2026-08-27 (ranger-base-f8m9): the cockpit's `d`
// (LaunchBead) answers the pass's own two questions — WHICH LANE, WHICH
// SEAT — instead of taking Route's single head. These pin the five cases
// the amendment named: a busy lane head overflows to the next seat, a
// fully busy lane refuses by naming the lane, an explicit assignee busy
// elsewhere is the §4 hole `d` had, an in-progress holder's resume never
// re-reads the persona's other sessions, and an unassigned in-progress
// bead seats the run record's holder over a free first seat.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAgentStatuses replaces the fake herdr's agent listing wholesale with
// one entry per named session, each carrying the given status — "pane run"
// (the fake's own launch hook) overwrites agents.json with a single fresh
// entry every time a new pane starts, so a test with more than one live
// session must write the merged listing itself or the earlier session's
// "working" mark is invisible to personaActive by the time of the assertion.
func fakeAgentStatuses(t *testing.T, fake string, statuses map[string]string) {
	t.Helper()
	ws := fakeLoadWSFrom(t, fake)
	var agents []string
	for i := range ws {
		st, ok := statuses[ws[i].Label]
		if !ok {
			continue
		}
		ws[i].AgentStatus = st
		agents = append(agents, fmt.Sprintf(`{"agent":"claude","agent_status":%q,"pane_id":%q,"workspace_id":%q}`,
			st, ws[i].WorkspaceID+":p1", ws[i].WorkspaceID))
	}
	saveWSTo(t, fake, ws)
	if err := os.WriteFile(filepath.Join(fake, "agents.json"), []byte("["+strings.Join(agents, ",")+"]"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// "unassigned bead, lane head working → seats the next free seat."
func TestLaunchBeadOverflowsToTheNextFreeSeatWhenTheLaneHeadIsBusy(t *testing.T) {
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "developer", "[code]")
	writePersona(t, b.App, "developer-2", "[code]")
	repo := t.TempDir()
	agentPerLaunch(t, fake)

	d1 := newTestDispatcher(t, b)
	seed := RepoIssue{BdIssue: BdIssue{ID: "seed", Title: "s", Assignee: "developer"}, Dir: repo}
	if _, err := d1.LaunchBead(seed); err != nil {
		t.Fatal(err)
	}
	fakeAgentStatuses(t, fake, map[string]string{
		SessionForBead("developer", repo, "seed"): "working",
	})

	d2 := newTestDispatcher(t, b)
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"code"}}, Dir: repo}
	session, err := d2.LaunchBead(is)
	if err != nil {
		t.Fatalf("developer-2 is free; the bead must overflow to it, got %v", err)
	}
	if want := SessionForBead("developer-2", repo, "a-1"); session != want {
		t.Errorf("want the free seat developer-2 (%s), got %s", want, session)
	}
}

// "lane fully busy → refusal names the lane, not one persona."
func TestLaunchBeadRefusesByLaneWhenEverySeatIsBusy(t *testing.T) {
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "developer", "[code]")
	writePersona(t, b.App, "developer-2", "[code]")
	repo := t.TempDir()
	agentPerLaunch(t, fake)

	for _, who := range []string{"developer", "developer-2"} {
		d := newTestDispatcher(t, b)
		seed := RepoIssue{BdIssue: BdIssue{ID: "seed-" + who, Title: "s", Assignee: who}, Dir: repo}
		if _, err := d.LaunchBead(seed); err != nil {
			t.Fatal(err)
		}
	}
	fakeAgentStatuses(t, fake, map[string]string{
		SessionForBead("developer", repo, "seed-developer"):     "working",
		SessionForBead("developer-2", repo, "seed-developer-2"): "working",
	})

	d := newTestDispatcher(t, b)
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"code"}}, Dir: repo}
	_, err := d.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "code lane busy") {
		t.Errorf("want a lane-busy refusal, got %v", err)
	}
	for _, who := range []string{"developer", "developer-2"} {
		if !strings.Contains(err.Error(), who) {
			t.Errorf("the lane-busy refusal must name %s, got %v", who, err)
		}
	}
}

// "ready ASSIGNED bead, assignee working another bead in the repo → refused
// busy (the §4 hole d had)." An assignee is a lane of one, but ADR 0020 §2
// puts §4 in front of `d` for a fresh launch: today `d` only read the
// TARGET session, so a persona busy on another bead in this repo went
// unnoticed and got fanned two-wide.
func TestLaunchBeadRefusesAnAssignedBeadWhenTheAssigneeIsBusyElsewhere(t *testing.T) {
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "developer", "[code]")
	repo := t.TempDir()
	agentPerLaunch(t, fake)

	d1 := newTestDispatcher(t, b)
	seed := RepoIssue{BdIssue: BdIssue{ID: "seed", Title: "s", Assignee: "developer"}, Dir: repo}
	if _, err := d1.LaunchBead(seed); err != nil {
		t.Fatal(err)
	}
	fakeAgentStatuses(t, fake, map[string]string{
		SessionForBead("developer", repo, "seed"): "working",
	})

	d2 := newTestDispatcher(t, b)
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"code"}, Assignee: "developer"}, Dir: repo}
	if _, err := d2.LaunchBead(is); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Errorf("an assignee busy on another bead in this repo must refuse, got %v", err)
	}
}

// "in_progress assigned, holder idle, persona busy on another bead → still
// resumes the holder (behaviour preserved)." An in-progress assigned bead
// is a lane of one and `d` there is resume, not a seat question (§2): the
// persona's OTHER sessions never enter it.
func TestLaunchBeadResumesAnIdleHolderEvenWhenThePersonaIsBusyElsewhere(t *testing.T) {
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "developer", "[code]")
	repo := t.TempDir()
	agentPerLaunch(t, fake)

	d1 := newTestDispatcher(t, b)
	a1 := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Assignee: "developer"}, Dir: repo}
	if _, err := d1.LaunchBead(a1); err != nil {
		t.Fatal(err)
	}

	d2 := newTestDispatcher(t, b)
	seed := RepoIssue{BdIssue: BdIssue{ID: "seed", Title: "s", Assignee: "developer"}, Dir: repo}
	if _, err := d2.LaunchBead(seed); err != nil {
		t.Fatal(err)
	}

	// developer is busy on `seed` now, visibly so — both sessions carry a
	// live agent — but a-1's own holder is left idle.
	fakeAgentStatuses(t, fake, map[string]string{
		SessionForBead("developer", repo, "a-1"):  "idle",
		SessionForBead("developer", repo, "seed"): "working",
	})

	d3 := newTestDispatcher(t, b)
	d3.PromptGrace = 0 // this test is not about PromptGrace; the record from d1's own launch would still be within it
	resume := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Status: "in_progress", Assignee: "developer"}, Dir: repo}
	session, err := d3.LaunchBead(resume)
	if err != nil {
		t.Fatalf("resuming an assigned holder must not re-check the persona's other sessions: %v", err)
	}
	if want := SessionForBead("developer", repo, "a-1"); session != want {
		t.Errorf("want resume into the holder %s, got %s", want, session)
	}
}

// "in_progress unassigned with a run record for seat 2 → seats the record
// holder, not the first free seat." An unclaim erased the assignee under a
// live run; the run record answers before availability does.
func TestLaunchBeadSeatsUnassignedInProgressBeadOnItsRunRecordHolder(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "developer", "[code]")
	writePersona(t, b.App, "developer-2", "[code]")
	repo := t.TempDir()
	agentPerLaunch(t, fake)

	holder := SessionForBead("developer-2", repo, "a-1")
	mustCreate(t, b, NewSessionOpts{Name: holder, Dir: repo, Agent: "developer-2", Bead: "a-1"})

	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"code"}, Status: "in_progress"}, Dir: repo}
	session, err := d.LaunchBead(is)
	if err != nil {
		t.Fatal(err)
	}
	if session != holder {
		t.Errorf("want the run record's holder %s (seat 2), not the first free seat, got %s", holder, session)
	}
}
