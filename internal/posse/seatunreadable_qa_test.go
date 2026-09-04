package posse

// ranger-base-3yqyg: the THIRD door onto one double-seating.
//
// ranger-base-6swlr closed absence-under-abstention for reconcileSeats and
// ranger-base-5kiu4 closed it for personaActive; both are nil-error paths,
// where the listing answered and withheld a meta. This is the same sentence
// with the loudest cause: the listing could not be read AT ALL, and
// personaActive's `err != nil` arm reported ("", "") — the answer a
// genuinely idle persona gives. One failed `workspace list` therefore made
// EVERY seat in the shop read free and the pass fired into all of them,
// with nothing above it to abort: reconcileSeats returns early on the same
// error without touching its map, and no other listing reader sits on the
// fire path.
//
// The reachable shape is a TRANSIENT read failure over a LIVE herd — with
// herdr genuinely down the create fails anyway — which is what the fake's
// `list-error` lever reproduces: the listing refuses, `workspace create`
// still works, so a seat walk that reads through the error really does put
// a second bead in a live session.
//
// The answer is per-SEAT, by the same loop the withheld case uses: an error
// is the widest abstention there is, so every meta on disk is a session
// this listing declined to answer for. Every test below is paired with the
// control that separates that from a whole-shop freeze, because holding
// every seat in the fleet on one unreadable read passes the first assertion
// of each and fails the second.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaListError arms the fake herdr to fail `workspace list`, which is the
// first of the two reads listSessions takes and the one whose error every
// caller sees as a bare `err != nil`.
func qaListError(t *testing.T, fake string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fake, "list-error"), []byte("timeout|no response from the herdr server"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// qaMetaNaming plants a session meta with the workspace field the caller
// asks for. Unlike qaWithheldMeta it says nothing about sockets or boards:
// under an unreadable listing none of the withholding guards ever run, and
// what personaActive reads is exactly this file.
func qaMetaNaming(t *testing.T, b *HerdrBackend, name, agent, workspace string, crew bool) {
	t.Helper()
	meta := "name: " + name + "\nworkspace: " + workspace + "\npane: " + workspace + ":p1\nemoji: x\nagent: " + agent + "\nsocket: " + SocketID() + "\n"
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

// The bug end to end, on the shape that is actually reachable: a-1 seated by
// a real Run into a real session, then ONE listing read fails over a herd
// that is still there, and a fresh Run — new process, empty busy map, no
// hold for reconcileSeats to keep — offers a-2 the same seat.
//
// MUTATION: put `return "", ""` back on personaActive's err arm → a-2 gets a
// workspace under a-1's live seat, one persona with two beads in two
// worktrees, which is ADR 0022's single writer defeated.
func TestQAAFreshRunFindsNoFreeSeatUnderAnUnreadableListing(t *testing.T) {
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
	if _, _, err := b.listSessions(); err != nil {
		t.Fatalf("premise: the herd answers in full before the lever is armed: err=%v", err)
	}

	qaListError(t, fake) // one read fails; every session is still alive
	if _, _, err := b.listSessions(); err == nil {
		t.Fatalf("premise: the listing must FAIL — an error, not a short answer, is this bead's whole trigger")
	}

	d2 := newTestDispatcher(t, b)
	d2.PromptGrace = 0
	fresh, freshFail := map[string]string{}, map[string]int{}
	inFlight = append(inFlight, seatFire(t, d2, repo, "a-2", fresh, freshFail)...)
	out := dispatcherOut(d2)

	if strings.Contains(calls(t, fake), "workspace create --label "+SessionForBead("ranger", repo, "a-2")) {
		t.Errorf("a fresh Run seated a-2 on a listing it could not read while a-1's session was live: one persona, two beads, two worktrees on one seat\n%s", out)
	}
	if fresh[slot] != "" {
		t.Errorf("the fresh Run took the seat on a listing that could not answer at all: busy=%v\n%s", fresh, out)
	}
	if !strings.Contains(out, "lane busy") {
		t.Errorf("a bead that cannot be seated must be reported as a busy lane and left for a later pass:\n%s", out)
	}
}

// The seat walk answers for ONE seat, and the control is the seat next door.
//
// "The listing failed, so no seat is free" passes the first half of this and
// fails the second: it would trade the double-seating for a fleet that stops
// hiring whenever one herdr read times out — the ranger-base-ifjgm phantom in
// the safe direction, and this time over the whole shop at once rather than
// one meta at a time.
//
// MUTATION: return the slot unconditionally on the err arm → red on the free
// seat. Put `return "", ""` back → red on the held one.
func TestPersonaActiveHoldsAnUnreadableSeatAndOnlyThatSeat(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dir := "/src/posse"
	slot := SessionFor("developer", dir)

	// The agent field is deliberately EMPTY — a meta written before the
	// field existed, so the seat PREFIX is the only thing that can decide
	// the control below. With an agent on it a walk that dropped the prefix
	// match would still be caught by the agent filter, and the control would
	// pass for the wrong reason.
	qaMetaNaming(t, b, slot+"-a-1", "", "w405", false)
	// The premise is the SEPARATION, not a free seat: while the listing
	// still answers, this meta is withheld by name and the seat is held as
	// `unlisted`. What the lever below changes is the CAUSE, and a status
	// that did not move would be the two repairs rendered as one.
	if name, st := d.personaActive("developer", dir); name != slot+"-a-1" || st != seatUnlisted {
		t.Fatalf("premise: with the listing readable this seat is held as %q: got %q %q", seatUnlisted, name, st)
	}

	qaListError(t, fake)
	if _, _, err := b.listSessions(); err == nil {
		t.Fatalf("premise: the listing must fail")
	}

	name, st := d.personaActive("developer", dir)
	if name != slot+"-a-1" {
		t.Errorf("a seat whose session the herd could not be asked about read as an EMPTY seat: personaActive = %q, want %q", name, slot+"-a-1")
	}
	if st != seatUnreadable {
		t.Errorf("status = %q, want %q: the seat is taken and the herd could not be listed — reporting %q would say a stale meta was the repair, and the repair is at herdr", st, seatUnreadable, seatUnlisted)
	}

	// The control: a persona with no meta at all has no session to be
	// unreadable, and stalling the whole shop on a failed read would stop
	// hiring in every lane.
	if name, st := d.personaActive("hopper", dir); name != "" {
		t.Errorf("an unreadable listing froze a seat with no session in it: personaActive(hopper) = %q %q, want free — the abstention is per-seat", name, st)
	}
}

// The meta filters are facts read off the FILE, not off the listing, so a
// herdr that cannot answer does not change them.
//
// Crew is the one that matters (ADR 0008): dispatch treats the operator's
// conversation as no session at all, and without the skip a single failed
// read would freeze every lane holding a crew-marked seat. A recipe — a meta
// naming no workspace, kept for `posse relaunch` (rangerhq-v52t) — is the
// one this path has to sort out for itself: listSessions drops recipes
// before it withholds anything, so on the error path, where the names come
// straight off disk, a walk that did not check would hold a seat for a
// session that is already gone.
//
// MUTATION: drop `m.Crew` from the skip → red on the first; drop the agent
// term → red on the second; drop the `m.Workspace == ""` skip → red on the
// third.
func TestPersonaActiveSkipsCrewForeignAndRecipeMetasUnderAnUnreadableListing(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dir := "/src/posse"
	slot := SessionFor("developer", dir)
	qaListError(t, fake)
	if _, _, err := b.listSessions(); err == nil {
		t.Fatalf("premise: the listing must fail")
	}

	qaMetaNaming(t, b, slot, "developer", "w405", true) // the operator's own session
	if name, st := d.personaActive("developer", dir); name != "" {
		t.Errorf("an unreadable CREW session held the seat: %q %q — ADR 0008 keeps dispatch out of the operator's conversation, and a herd that cannot be listed does not change whose session it is", name, st)
	}
	os.Remove(b.metaPath(slot))

	qaMetaNaming(t, b, slot+"-a-1", "hopper", "w406", false) // same prefix, another persona
	if name, st := d.personaActive("developer", dir); name != "" {
		t.Errorf("a meta belonging to another agent held this persona's seat: %q %q", name, st)
	}
	os.Remove(b.metaPath(slot + "-a-1"))

	qaMetaNaming(t, b, slot+"-a-2", "developer", "", false) // a recipe: no workspace
	if name, st := d.personaActive("developer", dir); name != "" {
		t.Errorf("a RECIPE held the seat: %q %q — a meta naming no workspace is a session already gone, and holding a seat for one would stop hiring until the operator deletes the file", name, st)
	}
}

// A session this pass already launched and gave up on is not the persona
// working (ADR 0013 §2), and that stays true when the listing fails: the
// strand is this pass's own record, not something read off the herd.
//
// MUTATION: drop the `d.stranded[name]` skip on the withheld/err loop → the
// seat this pass just sterilised holds itself, and the very next bead is
// told the lane is busy by a pane the pass abandoned.
func TestPersonaActiveSkipsAStrandedSessionUnderAnUnreadableListing(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dir := "/src/posse"
	slot := SessionFor("developer", dir)
	qaMetaNaming(t, b, slot+"-a-1", "developer", "w405", false)
	qaListError(t, fake)

	if name, _ := d.personaActive("developer", dir); name != slot+"-a-1" {
		t.Fatalf("premise: the seat must be held before the strand, or the strand proves nothing: %q", name)
	}
	d.strand(slot + "-a-1")
	if name, st := d.personaActive("developer", dir); name != "" {
		t.Errorf("a session this pass stranded held the seat under an unreadable listing: %q %q", name, st)
	}
}

// ranger-base-wq1aq, the diagnosis half of the same bead. The seat verdicts
// above are right and fail-closed; what an operator READ when they fired was
// ADR 0020 §2's lane line and nothing else — "code lane busy: ranger", the
// same sentence an honestly full shop prints. Neither half of the seat walk
// said the listing had failed: laneBusyLine drops the status by design (§2
// spells its shape by example), and reconcileSeats, which prints on every
// other pass it declines, returned on `err` in silence.
//
// The line goes on reconcileSeats' error arm because that runs ONCE, at the
// head of the fire loop, above the lane lines it explains. Where in that
// function is decided by the FRESH RUN below: its busy map is empty, so a
// diagnosis placed under the old `d.DryRun || len(busy) == 0` guard would
// never run on the one shape ranger-base-3yqyg measured — a new process
// offering a-2 the seat a-1 is live in.
//
// MUTATION: drop the printf from reconcileSeats' err arm → red on the fresh
// Run's diagnosis assertion. Move the listSessions read back below the
// `d.DryRun || len(busy) == 0` guard → red on the same one (the fresh Run
// never takes the reading). Print the line whatever the listing said → red
// on the control. Drop the `%v` and print a bare sentence → red on the
// error-text assertion, which is the half that names the repair's target.
func TestQAAnUnreadableListingSaysSoAboveTheLaneBusyLine(t *testing.T) {
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
		t.Fatalf("premise: a-1 must be seated, or neither arm below has a busy lane to report: busy=%v\n%s", busy, dispatcherOut(d))
	}

	// The CONTROL, and it is the whole reason the line is worth anything: a
	// lane that is honestly full, read through a listing that answered in
	// full, says nothing about herdr. A diagnosis printed on every busy lane
	// would be the ADR's own line with noise on top and would tell an
	// operator exactly as much as the silence did.
	mark := len(dispatcherOut(d))
	inFlight = append(inFlight, seatFire(t, d, repo, "a-2", busy, sessFail)...)
	ctrl := dispatcherOut(d)[mark:]
	if !strings.Contains(ctrl, "lane busy: ranger") {
		t.Fatalf("premise: a-2 must find the lane full while a-1's session is live:\n%s", ctrl)
	}
	if strings.Contains(ctrl, "herd unreadable") {
		t.Errorf("a listing that answered in full was reported as unreadable — the line must be off the ERROR, or it says nothing about which shop the operator has:\n%s", ctrl)
	}

	// The bug's own shape: one read fails over a herd that is still there,
	// and a NEW process — empty busy map, no hold to reconcile — walks the
	// same seats.
	qaListError(t, fake)
	d2 := newTestDispatcher(t, b)
	d2.PromptGrace = 0
	fresh, freshFail := map[string]string{}, map[string]int{}
	inFlight = append(inFlight, seatFire(t, d2, repo, "a-3", fresh, freshFail)...)
	out := dispatcherOut(d2)

	lane := strings.Index(out, "lane busy")
	if lane < 0 {
		t.Fatalf("premise: the seat walk must still report the lane busy — the correctness half is ranger-base-3yqyg's and is not what this pins:\n%s", out)
	}
	said := strings.Index(out, "herd unreadable")
	if said < 0 {
		t.Fatalf("a lane held by a listing that could not be read printed only the busy line: an operator reading it cannot tell a full shop from a herdr that will not answer, and the two are repaired in different places:\n%s", out)
	}
	if said > lane {
		t.Errorf("the cause printed BELOW the symptom it explains: the reconcile runs at the head of the fire loop and its line must sit above every lane line of the pass it describes:\n%s", out)
	}
	// The error itself, not just a sentence about one: "timeout" is the
	// difference between a herdr that is wedged and one that is not there,
	// and it is the only part of this line that says what to go and look at.
	if !strings.Contains(out, "no response from the herdr server") {
		t.Errorf("the line did not carry the listing's own error, so it names no target for the repair it asks for:\n%s", out)
	}
}

// The same line on the arm that HAS holds to keep, paired with the claim it
// must not make. reconcileSeats' error arm returns without touching the map
// (TestQAAnUnreadableHerdKeepsEverySeatHold pins that); this pins that it
// says so rather than leaving the operator to infer it from a seat that
// never frees.
//
// MUTATION: drop the printf → red. Say "released" instead of "kept" → red on
// the second assertion, which is the sentence a phantom seat would read as
// permission.
func TestQAReconcileSeatsNamesTheFailedReadItKeptTheHoldsFor(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	slot := SessionFor("ranger", "/repo")
	busy := map[string]string{slot: "a-1"}

	b.H = Herdr{Bin: filepath.Join(t.TempDir(), "herdr-that-is-not-there")}
	d.reconcileSeats(busy)
	out := dispatcherOut(d)

	if busy[slot] != "a-1" {
		t.Fatalf("premise: the hold must survive the failed read: busy=%v\n%s", busy, out)
	}
	if !strings.Contains(out, "herd unreadable") {
		t.Errorf("the pass that declined to reconcile printed nothing: `seats kept` is printed on the withheld decline for exactly this reason, and the error arm is the WIDER decline of the two:\n%s", out)
	}
	if !strings.Contains(out, "no hold released this pass") {
		t.Errorf("the line must say what it did NOT do: a hold kept and a hold released are the two states a phantom seat sits between:\n%s", out)
	}
}
