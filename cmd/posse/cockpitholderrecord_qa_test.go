package main

import (
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// The bug this file pins (ranger-base-eeg0s): the cockpit's DISPLAY join read
// two name patterns and never the run record, while dispatch's `d` on the same
// row headed with the record. So a bead held by a session whose name matches
// neither pattern — the hand-dispatch route stamping `bead:` via
// NoteBeadFromPrompt, or a crew session made by hand and handed the bead —
// read "no session" in IN PROGRESS, its row sorted as an interrupted run, and
// enter/p/v hit noHolder and acted on nothing.
//
// ADR 0004 §2 as amended 2026-09-06 puts the record at the head of all three,
// the order ADR 0008's shield and ADR 0011 §3 already name.

// recordHeldFixture is a bead held by a session under NEITHER name pattern,
// carrying only the run record: `bead:` plus the persona and the checkout.
func recordHeldFixture(t *testing.T) (*cockpit, posse.RepoIssue) {
	t.Helper()
	const hand = "typed-by-hand" // no naming convention in it
	is := posse.RepoIssue{
		BdIssue: posse.BdIssue{ID: "b-handed", Title: "handed to a crew session",
			Status: "in_progress", Assignee: "devops", Updated: qaClock},
		Dir: qaDir,
	}
	if hand == posse.SessionForBead(is.Assignee, is.Dir, is.ID) || hand == posse.SessionFor(is.Assignee, is.Dir) {
		t.Fatalf("fixture session %q matches a name pattern — it must match neither", hand)
	}
	c := qaProgFixture()
	c.sessions = append(c.sessions, posse.HerdrSession{
		Name: hand, Emoji: "🙂", Agent: "devops", Status: "working",
		Dir: qaDir, Bead: is.ID, Crew: true, PaneID: "w9:p1",
	})
	c.inprog = append(c.inprog, is)
	c.sortInProg()
	c.buildRows()
	return c, is
}

// inProgIndex is the bead's position in the sorted IN PROGRESS list, which is
// also its offset in cursor space behind the sessions.
func inProgIndex(t *testing.T, c *cockpit, id string) int {
	t.Helper()
	for i := range c.inprog {
		if c.inprog[i].ID == id {
			return i
		}
	}
	t.Fatalf("no IN PROGRESS row for %s", id)
	return -1
}

func TestQAHolderJoinHeadsWithTheRunRecord(t *testing.T) {
	c, is := recordHeldFixture(t)

	s := c.holderSession(is)
	if s == nil {
		t.Fatalf("holderSession(%s) = nil: the display join is blind to the run record", is.ID)
	}
	if s.Name != "typed-by-hand" {
		t.Fatalf("holderSession(%s) = %q, want the record-held session", is.ID, s.Name)
	}
	if got := c.holderState(is); got != "working" {
		t.Errorf("holderState(%s) = %q, want %q", is.ID, got, "working")
	}

	// The holder column of the row itself names it — the operator-visible
	// half, which is what read "no session".
	at := inProgIndex(t, c, is.ID)
	line := qaPlain(renderRow(row{kind: rowItem, sec: secInProg, cols: c.inprogCols(c.inprog[at], 14)}, 200, false))
	if strings.Contains(line, noSession) {
		t.Errorf("row still reads %q: %q", noSession, line)
	}
	if !strings.Contains(line, "working") {
		t.Errorf("row does not carry the holder's state: %q", line)
	}

	// enter/p/v act on the holder, not on nothing (ADR 0004 §3).
	c.cursor = len(c.sessions) + at
	if sel := c.selected(); sel.sec != secInProg {
		t.Fatalf("cursor did not land in IN PROGRESS: %+v", sel)
	}
	if a := c.actSession(); a == nil || a.Name != "typed-by-hand" {
		t.Errorf("actSession = %+v, want the record-held session", a)
	}
}

// The record arm must be as narrow as RunHolder's: it keys on the bead, the
// persona and the CHECKOUT, and never matches a foreign row. Each of these
// would be a wrong holder — the reassignment nobody asked for — if the arm
// were widened.
func TestQAHolderRecordArmIsNarrow(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*posse.HerdrSession, *posse.RepoIssue)
	}{
		{"another bead's record", func(s *posse.HerdrSession, _ *posse.RepoIssue) { s.Bead = "b-elsewhere" }},
		{"another persona's session", func(s *posse.HerdrSession, _ *posse.RepoIssue) { s.Agent = "developer" }},
		{"another checkout", func(s *posse.HerdrSession, _ *posse.RepoIssue) { s.Dir, s.Repo = "/other/repo", "" }},
		{"a foreign row", func(s *posse.HerdrSession, _ *posse.RepoIssue) { s.Foreign = true }},
		{"no record at all", func(s *posse.HerdrSession, _ *posse.RepoIssue) { s.Bead = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, is := recordHeldFixture(t)
			tc.mut(&c.sessions[len(c.sessions)-1], &is)
			if s := c.holderSession(is); s != nil {
				t.Errorf("%s joined to %q", tc.name, s.Name)
			}
			if got := c.holderState(is); got != noSession {
				t.Errorf("%s: holderState = %q, want %q", tc.name, got, noSession)
			}
		})
	}
}

// RunHolder refuses an empty key rather than matching on it, and so does this
// arm: a bead with no id or no repo must not join to a session that carries no
// record and shares the empty checkout.
func TestQAHolderRecordArmRefusesAnEmptyKey(t *testing.T) {
	c := qaProgFixture()
	c.sessions = append(c.sessions, posse.HerdrSession{Name: "unstamped", Agent: "devops", Status: "working"})
	for _, is := range []posse.RepoIssue{
		{BdIssue: posse.BdIssue{ID: "", Assignee: "devops"}, Dir: ""},
		{BdIssue: posse.BdIssue{ID: "b-handed", Assignee: "devops"}, Dir: ""},
	} {
		if s := c.holderSession(is); s != nil && s.Name == "unstamped" {
			t.Errorf("empty key (id=%q dir=%q) joined to the unstamped session", is.ID, is.Dir)
		}
	}
}

// A per-session worktree's Dir is not the repo the bead came from
// (rangerhq-09o2), so the arm compares Checkout() — Repo when set. A worktree
// session whose Repo is the bead's checkout is still the holder.
func TestQAHolderRecordMatchesThroughAWorktree(t *testing.T) {
	c, is := recordHeldFixture(t)
	s := &c.sessions[len(c.sessions)-1]
	s.Dir, s.Repo = "/home/u/.posse/worktrees/x/devops-b-handed", qaDir
	if got := c.holderSession(is); got == nil || got.Name != "typed-by-hand" {
		t.Fatalf("worktree session not joined: %+v", got)
	}
}

// The record wins over both name patterns when more than one is live — the
// order ADR 0004 §2 names, and the reason the record is a fact where a name
// is a guess.
func TestQAHolderRecordWinsOverBothNames(t *testing.T) {
	c, is := recordHeldFixture(t)
	c.sessions = append(c.sessions,
		posse.HerdrSession{Name: posse.SessionForBead(is.Assignee, is.Dir, is.ID), Agent: is.Assignee, Status: "idle", Dir: qaDir},
		posse.HerdrSession{Name: posse.SessionFor(is.Assignee, is.Dir), Agent: is.Assignee, Status: "idle", Dir: qaDir},
	)
	if got := c.holderSession(is); got == nil || got.Name != "typed-by-hand" {
		t.Fatalf("record must head the join, got %+v", got)
	}
}
