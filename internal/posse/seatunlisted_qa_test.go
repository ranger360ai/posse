//go:build posse_arm2

package posse

// ranger-base-5kiu4: the OTHER half of ranger-base-6swlr's abstention.
//
// 6swlr closed the standing Run's door — reconcileSeats no longer releases a
// hold on a listing that withheld a meta. personaActive is the same seat walk
// with the same rule inverted: it scans the listing's rows for one in the
// seat prefix and reports ("", "") when it finds none, so a withheld session
// read as an EMPTY SEAT. A FRESH Run — new process, empty busy map, no hold
// for reconcileSeats to keep — therefore seated a second bead into a live
// session, and reconcileSeats could not help because there was nothing in its
// map to keep.
//
// MEASURED POSITIVE 2026-09-03 with the 6swlr fix already in: the probe
// reported created-a-2-session=true and busy=map[ranger-003:a-2], and the arm
// that fired was `spared` — the ordinary 5m prune grace, not a herdr restart.
//
// The answer taken is per-SEAT, not the whole-pass abstention reconcileSeats
// makes: listSessions now returns the withheld NAMES, so a seat walk can ask
// about one seat. Every test below is paired with the control that separates
// those two shapes, because a whole-pass abstention passes the first
// assertion of each and fails the second.

import (
	"os"
	"strings"
	"testing"
	"time"
)

// qaWithheldMeta plants a session meta the next listing will withhold with a
// NIL error: it names a workspace no board holds and a socket that is not
// this server's, which is cannotAnswerFor. Unlike qaStaleMeta the agent and
// the crew mark are the caller's, because they are what personaActive reads
// off the meta when the listing will not answer for it.
func qaWithheldMeta(t *testing.T, b *HerdrBackend, name, agent string, crew bool) {
	t.Helper()
	meta := "name: " + name + "\nworkspace: w405\npane: w405:p1\nemoji: x\nagent: " + agent + "\nsocket: /tmp/5kiu4/theirs.sock\n"
	if crew {
		meta += "crew: true\n"
	}
	if err := os.MkdirAll(b.metaDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.metaPath(name), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The bug, end to end: two Runs over one herd, the second one fresh.
//
// a-1 is seated by the first Run. Then the listing goes short with every meta
// intact — nothing died — and a NEW dispatcher with a NEW busy map fires a-2.
// The seat walk is the only thing standing between a-2 and a-1's live
// session, and it used to stand aside.
//
// MUTATION: drop the withheld loop from personaActive → a-2 gets a workspace
// under a-1's live seat, which is the reported ACTUAL to the character.
func TestQAAFreshRunFindsNoFreeSeatUnderAnAbstention(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	agentPerLaunch(t, fake)
	d.PromptGrace = 0

	var inFlight []*pendingBead
	t.Cleanup(func() { joinPrompts(t, inFlight) })

	slot := SessionFor("ranger", repo)
	busy, sessFail := map[string]string{}, map[string]int{}
	inFlight = append(inFlight, seatFire(t, d, repo, "a-1", busy, sessFail)...)
	if busy[slot] != "a-1" {
		t.Fatalf("premise: a-1 must be seated, or the fresh Run below proves nothing: busy=%v\n%s", busy, dispatcherOut(d))
	}
	if _, withheld, err := b.listSessions(); err != nil || len(withheld) != 0 {
		t.Fatalf("premise: before the short listing the herd answers in full: withheld=%v err=%v", withheld, err)
	}
	saveWSTo(t, fake, nil) // the listing goes short; every meta stays on disk
	sess, withheld, err := b.listSessions()
	if err != nil || len(withheld) == 0 {
		t.Fatalf("premise: the short listing must WITHHOLD with a nil error — that is the whole defect: n=%d withheld=%v err=%v", len(sess), withheld, err)
	}

	// A fresh Run: another process over the same herd. Nothing in its map to
	// reconcile, so 6swlr's abstention has nothing to hold.
	d2 := newTestDispatcher(t, b)
	d2.PromptGrace = 0
	fresh, freshFail := map[string]string{}, map[string]int{}
	inFlight = append(inFlight, seatFire(t, d2, repo, "a-2", fresh, freshFail)...)
	out := dispatcherOut(d2)

	if strings.Contains(calls(t, fake), "workspace create --label "+SessionForBead("ranger", repo, "a-2")) {
		t.Errorf("a fresh Run seated a-2 while a-1's session was live: one persona, two beads, two worktrees on one seat\n%s", out)
	}
	if fresh[slot] != "" {
		t.Errorf("the fresh Run took the seat on a listing that refused to answer for it: busy=%v\n%s", fresh, out)
	}
	if !strings.Contains(out, "lane busy") {
		t.Errorf("a bead that cannot be seated must be reported as a busy lane and left for a later pass:\n%s", out)
	}
}

// The same fresh Run through the arm the incident actually fired.
//
// The test above goes short by emptying the board, which is `emptyBoard` — a
// herdr that just came up. The measurement on this bead fired through
// `spared` instead: the seat's workspace is simply not on a board that still
// holds others, and prunable() will not call a meta younger than the 5m grace
// dead. So this is not only the herdr-restart shape; it is reachable on an
// ordinary pass, and the premise below pins WHICH arm withheld the meta
// rather than trusting the count.
//
// MUTATION: drop the withheld loop from personaActive → a-2 takes the seat
// here too.
func TestQAAFreshRunFindsNoFreeSeatUnderTheSparedArm(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/5kiu4/ours.sock") // the meta and the pass agree
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	agentPerLaunch(t, fake)
	d.PromptGrace = 0

	var inFlight []*pendingBead
	t.Cleanup(func() { joinPrompts(t, inFlight) })

	slot := SessionFor("ranger", repo)
	busy, sessFail := map[string]string{}, map[string]int{}
	inFlight = append(inFlight, seatFire(t, d, repo, "a-1", busy, sessFail)...)
	if busy[slot] != "a-1" {
		t.Fatalf("premise: a-1 must be seated: busy=%v\n%s", busy, dispatcherOut(d))
	}

	// The seat's workspace leaves a board that is still not empty. The id has
	// to be one herdr did NOT hand this launch: reusing it puts the fixture on
	// the `strangers` arm instead (measured while writing this), which is a
	// real abstention but not the one the incident fired.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w99", Label: "someone-else"}})
	w := warnBuf(t, b)
	_, withheld, err := b.listSessions()
	if err != nil || len(withheld) == 0 {
		t.Fatalf("premise: the meta must be withheld with a nil error: withheld=%v err=%v", withheld, err)
	}
	if !strings.Contains(w.String(), "not dead") {
		t.Fatalf("premise: this fixture must fire the SPARED arm, not emptyBoard — that is what makes it an ordinary pass: warn=%q", w.String())
	}

	d2 := newTestDispatcher(t, b)
	d2.PromptGrace = 0
	fresh, freshFail := map[string]string{}, map[string]int{}
	inFlight = append(inFlight, seatFire(t, d2, repo, "a-2", fresh, freshFail)...)
	if strings.Contains(calls(t, fake), "workspace create --label "+SessionForBead("ranger", repo, "a-2")) {
		t.Errorf("a fresh Run seated a-2 on a seat whose session was only SPARED — the prune grace is not a death certificate\n%s", dispatcherOut(d2))
	}
	if fresh[slot] != "" {
		t.Errorf("the fresh Run took the seat: busy=%v\n%s", fresh, dispatcherOut(d2))
	}
}

// The seat walk answers for ONE seat, and the control is the seat next door.
//
// A whole-pass abstention — "the listing withheld something, so no seat is
// free" — passes the first half of this and fails the second: it would trade
// the double-seating for a fleet that stops hiring on one stale meta, which
// is precisely the ranger-base-ifjgm phantom in the safe direction.
//
// MUTATION: return on `len(withheld) > 0` instead of matching the prefix →
// red on the free seat. Drop the withheld loop → red on the held one.
func TestPersonaActiveHoldsTheWithheldSeatAndOnlyThatSeat(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/5kiu4/ours.sock")
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	// A live workspace of somebody else's: NOT an empty board, so what
	// follows is cannotAnswerFor and not emptyBoard by another name.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "live"}})

	dir := "/src/posse"
	slot := SessionFor("developer", dir)
	if name, st := d.personaActive("developer", dir); name != "" {
		t.Fatalf("premise: nothing is planted yet and the seat must read free: %q %q", name, st)
	}

	// The agent field is deliberately EMPTY — the shape of a meta written
	// before the field existed, and the fixture that makes the seat PREFIX
	// the only thing that can decide the control below. With an agent on it,
	// a walk that dropped the prefix match would still be caught by the
	// agent filter, and the control would pass for the wrong reason.
	qaWithheldMeta(t, b, slot+"-a-1", "", false)
	sess, withheld, err := b.listSessions()
	if err != nil || len(withheld) != 1 {
		t.Fatalf("premise: the meta must be withheld with a nil error: withheld=%v err=%v", withheld, err)
	}
	for _, s := range sess {
		if s.Name == slot+"-a-1" {
			t.Fatalf("premise: the withheld session must be ABSENT from the rows — that absence is what the seat walk misread")
		}
	}

	name, st := d.personaActive("developer", dir)
	if name != slot+"-a-1" {
		t.Errorf("a session this listing could not answer for read as an empty seat: personaActive = %q, want %q", name, slot+"-a-1")
	}
	if st != seatUnlisted {
		t.Errorf("status = %q, want %q: the seat is taken and nobody can currently say by what — reporting herdr's %q would be the listing's lie in the other direction", st, seatUnlisted, "working")
	}

	// The control: one seat's unanswerable meta is not the whole shop's.
	if name, st := d.personaActive("hopper", dir); name != "" {
		t.Errorf("a meta withheld in one seat froze another: personaActive(hopper) = %q %q, want free — the abstention is per-seat", name, st)
	}
}

// The three row filters are facts about the META, not about the listing, so a
// herdr that cannot answer does not change them.
//
// Crew is the one that matters (ADR 0008): dispatch treats the operator's
// conversation as no session at all, and without the skip a herdr restart
// would freeze every lane holding a crew-marked seat — the whole-pass failure
// again, arriving one meta at a time.
//
// MUTATION: drop `m.Crew` from the skip → red on the first; drop the agent
// term → red on the second.
func TestPersonaActiveSkipsAWithheldCrewOrForeignMeta(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/5kiu4/ours.sock")
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "live"}})
	dir := "/src/posse"
	slot := SessionFor("developer", dir)

	qaWithheldMeta(t, b, slot, "developer", true) // the operator's own session
	if _, withheld, err := b.listSessions(); err != nil || len(withheld) != 1 {
		t.Fatalf("premise: the crew meta must be withheld with a nil error: withheld=%v err=%v", withheld, err)
	}
	if name, st := d.personaActive("developer", dir); name != "" {
		t.Errorf("a withheld CREW session held the seat: %q %q — ADR 0008 keeps dispatch out of the operator's conversation, and a listing that cannot answer does not change whose session it is", name, st)
	}

	// Same seat prefix, another persona's agent: not this persona working,
	// listed or withheld (the same read the rows get, ranger-base-p6no).
	qaWithheldMeta(t, b, slot+"-a-1", "hopper", false)
	if name, st := d.personaActive("developer", dir); name != "" {
		t.Errorf("a withheld meta belonging to another agent held this persona's seat: %q %q", name, st)
	}
}

// The names are the change ranger-base-5kiu4 needed from ranger-base-6swlr's
// count, and a count cannot be asserted into a name: this is the pin that
// separates "something was withheld" from "THIS seat's session was withheld".
//
// MUTATION: return a name for a listed session too, or return the spared/
// strangers strings (which carry a `name: why` suffix) instead of the bare
// name → red, and the prefix match in personaActive silently stops matching.
func TestListSessionsNamesWhatItWithheld(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/5kiu4/ours.sock")
	b, fake := newTestBackend(t)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "live"}})

	qaWithheldMeta(t, b, "developer-posse-a-1", "developer", false)
	sess, withheld, err := b.listSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(withheld) != 1 || withheld[0] != "developer-posse-a-1" {
		t.Fatalf("the withheld list must carry the meta NAME, in the same namespace as the listing's own rows and every seat prefix: %q", withheld)
	}
	if len(sess) != 1 || sess[0].Name != "live" {
		t.Errorf("premise: the board's own workspace is still listed; only the meta was withheld: %+v", sess)
	}
	// A meta this server CAN answer for is not withheld, whatever else is.
	if strings.HasPrefix(withheld[0], "live") {
		t.Errorf("a listed session must never appear in the withheld list: %q", withheld)
	}

	// The `spared` arm carries its reason to the warning as "name: why", and
	// the withheld list must carry the NAME: a prefix match against
	// "developer-posse-a-2: its meta is younger than…" still matches here by
	// accident and stops matching the moment a seat asks about the exact
	// name, so the pin is equality.
	meta := "name: developer-posse-a-2\nworkspace: w406\npane: w406:p1\nemoji: x\nagent: developer\n" +
		"socket: /tmp/5kiu4/ours.sock\nlaunched: " + time.Now().UTC().Format(time.RFC3339) + "\n"
	if err := os.WriteFile(b.metaPath("developer-posse-a-2"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, withheld, err = b.listSessions(); err != nil {
		t.Fatal(err)
	}
	if len(withheld) != 2 || withheld[1] != "developer-posse-a-2" {
		t.Errorf("a SPARED meta is withheld under its bare name, not the warning string: %q", withheld)
	}
}

// ranger-base-ox49o (verify of ranger-base-5kiu4): the FOURTH filter on the
// withheld walk — this pass's own stranded launches — held nothing up.
//
// personaActive skips a withheld meta on four facts read off the meta file:
// the seat prefix, `d.stranded`, the crew mark and the agent. Three were
// pinned at the close and mutation-checked; `d.stranded` was neither, and
// dropping it left every pin in this file green (measured 2026-09-04,
// go test -overlay).
//
// It is reachable and it is not the safe direction. `strand` records a
// session this pass launched and could not prompt (dispatch.go, ADR 0013 §2)
// so the rest of the pass may try that seat again — and the launch failures
// that strand a session are exactly the ones that also take its workspace
// out of the next listing, which is what makes the same meta withheld.
// Without the skip the seat this pass just gave up on reads as OCCUPIED for
// the rest of the pass, by a session nobody can address, and the retry the
// ceiling deliberately grants it never happens.
//
// MUTATION: drop `if d.stranded[name] { continue }` from the withheld walk →
// the second assertion reds; the first is the positive control that the seat
// really was held before the strand, so an arm that holds nothing cannot
// pass this by asserting an absence that is true of nothing.
func TestQAAStrandedLaunchDoesNotHoldItsSeatThroughTheWithheldWalk(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/5kiu4/ours.sock")
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	// A live workspace of somebody else's, so what follows is cannotAnswerFor
	// and not emptyBoard by another name.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "live"}})

	dir := "/src/posse"
	slot := SessionFor("developer", dir)
	session := slot + "-a-1"
	qaWithheldMeta(t, b, session, "", false)
	if _, withheld, err := b.listSessions(); err != nil || len(withheld) != 1 {
		t.Fatalf("premise: the meta must be withheld with a nil error: withheld=%v err=%v", withheld, err)
	}

	// POSITIVE CONTROL: before the strand the withheld meta holds the seat.
	// Without this the assertion below would be an absence that is true of
	// nothing at all.
	if name, st := d.personaActive("developer", dir); name != session {
		t.Fatalf("premise: an unstranded withheld meta must hold its seat: %q %q, want %q", name, st, session)
	}

	// This pass launched that session and could not use it (ADR 0013 §2).
	d.strand(session)
	if name, st := d.personaActive("developer", dir); name != "" {
		t.Errorf("a session THIS PASS stranded held its own seat through the withheld walk: %q %q — "+
			"the strand exists so the slot stays free for the retry the ceiling grants it, and the rows "+
			"already read it that way", name, st)
	}
}
