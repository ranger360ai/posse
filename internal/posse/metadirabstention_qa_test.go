//go:build posse_arm2

package posse

// ranger-base-jzxrh: the FOURTH door onto one double-seating, found while
// verifying ranger-base-3yqyg (ranger-base-n8qwj).
//
// ranger-base-6swlr closed absence-under-abstention for reconcileSeats,
// ranger-base-5kiu4 for personaActive's nil-error path, ranger-base-3yqyg
// for its err path. All three answer a listing that would not say. This is
// the same sentence one layer down: metaNames (herdrback.go) drops its own
// os.ReadDir error, so a meta DIRECTORY that cannot be read answers exactly
// like one with nothing in it — and 3yqyg's repair, which stands on
// metaNames, reports the free seat it was filed to remove.
//
// The first two tests were PARKED when this file was filed; the fix landed
// with this bead and they are live. Both were shown able to fail before
// they were unparked — see the bead's close for the mutants.
//
// The THIRD is the control, and it keeps
// the fix from being "hold the seat on any error at all". A meta dir that
// does not exist is not an abstention, it is a box with no sessions — a
// fresh install, a state dir not yet made — and freezing the shop on that
// would be ranger-base-ifjgm in the safe direction over every lane at once.
// Any fix here has to be error-KIND aware.

import (
	"os"
	"strings"
	"testing"
)

// qaUnreadableMetaDir makes the meta dir write-and-exec but not READ.
//
// 0o333 and not 0o000, and the difference is the whole measurement: at 0o000
// the same fault that blanks the listing also blocks the meta WRITE, so the
// launch aborts on `permission denied` and the harm hides behind a failure
// that looks like the guard working. At 0o333 the write succeeds and the
// seat walk is the only thing that is wrong, which is the reachable shape —
// any ReadDir error that is not also a write error (EACCES on the dir alone,
// EIO on a mount that went away, EMFILE under fd exhaustion) lands here.
//
// It skips rather than fails where the uid can still read the dir, because
// which fence works depends on who is asking (a root or a uid-0-equivalent
// reader is not fenced by mode bits at all).
func qaUnreadableMetaDir(t *testing.T, b *HerdrBackend) {
	t.Helper()
	md := b.metaDir()
	if err := os.MkdirAll(md, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(md, 0o333); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(md, 0o755) })
	if _, err := os.ReadDir(md); err == nil {
		t.Skip("this uid can still read a write-only directory — the lever cannot be armed here")
	}
	if names, err := b.metaNames(); len(names) != 0 || err == nil {
		t.Fatalf("premise: metaNames must come back empty AND say it could not read, for these assertions to be about the seat: %v, %v", names, err)
	}
}

// ranger-base-jzxrh. Both arms, at the seat walk itself.
func TestQAAnUnreadableMetaDirHoldsTheSeat(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dir := "/src/posse"
	slot := SessionFor("developer", dir)
	qaMetaNaming(t, b, slot+"-a-1", "", "w405", false)

	// The premise is the SEPARATION: while the dir is readable this seat is
	// held, so what the lever below changes is the answer and not the setup.
	if name, st := d.personaActive("developer", dir); name != slot+"-a-1" {
		t.Fatalf("premise: with the meta dir readable this seat is held: got %q %q", name, st)
	}

	qaUnreadableMetaDir(t, b)

	// ARM 2, and it pre-dates ranger-base-3yqyg: NOTHING is wrong with herdr,
	// only the dir. Before the fix listSessions walked metaNames, found
	// nothing to list and nothing to withhold, and returned a clean empty
	// answer with err == nil — so nothing above personaActive aborted
	// either, and the seat below read free with no abstention anywhere in
	// the chain. The listing must ERROR instead: that is what puts
	// personaActive on ranger-base-3yqyg's arm at all.
	if sess, withheld, err := b.listSessions(); err == nil {
		t.Errorf("the listing answered CLEAN on a meta dir it could not read: %d sessions, withheld=%v, err=<nil> — an empty herd and an unreadable one are the same answer again", len(sess), withheld)
	}
	// BOTH returns, not just "not empty" (ranger-base-eq3ba, finding 1).
	// Each of them reaches an operator: the status is seatPass.doing and
	// seatWhy prints it ("developer unreadable"), and the name rides to the
	// refill summary. The sibling arm pins both for the same reason
	// (seatunreadable_qa_test.go) and this one inherited none of it —
	// `seatUnreadable` → `seatUnlisted` and the slot name → any other string
	// both survived the whole seat suite.
	if name, st := d.personaActive("developer", dir); name != slot || st != seatUnreadable {
		t.Errorf("an unreadable meta DIR must hold the seat under the SLOT's own name and say which repair it is: personaActive = %q %q, want %q %q — the seat is reported under its slot because no session name can be read, and a status that did not move would render two different repairs as one", name, st, slot, seatUnreadable)
	}

	// ARM 1: ranger-base-3yqyg's own trigger on top of it. Its err arm
	// answers out of metaNames, which is the thing that cannot read.
	qaListError(t, fake)
	if _, _, err := b.listSessions(); err == nil {
		t.Fatalf("premise: the listing must fail")
	}
	if name, st := d.personaActive("developer", dir); name != slot || st != seatUnreadable {
		t.Errorf("the err arm must hold the seat when metaNames' own ReadDir failed: personaActive = %q %q, want %q %q — 88c2507 claims an unreadable listing holds the seat it cannot answer for", name, st, slot, seatUnreadable)
	}
}

// ranger-base-jzxrh. The harm, end to end, on the shape that is
// actually reachable — the same pass ranger-base-3yqyg pins for its own
// lever, with the meta dir as this one.
func TestQAAFreshRunFindsNoFreeSeatUnderAnUnreadableMetaDir(t *testing.T) {
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

	qaUnreadableMetaDir(t, b)

	d2 := newTestDispatcher(t, b)
	d2.PromptGrace = 0
	fresh, freshFail := map[string]string{}, map[string]int{}
	inFlight = append(inFlight, seatFire(t, d2, repo, "a-2", fresh, freshFail)...)
	out := dispatcherOut(d2)

	if strings.Contains(calls(t, fake), "workspace create --label "+SessionForBead("ranger", repo, "a-2")) {
		t.Errorf("a fresh Run seated a-2 on a meta dir it could not read while a-1's session was live: one persona, two beads, two worktrees on one seat\n%s", out)
	}
	if fresh[slot] != "" {
		t.Errorf("the fresh Run took the seat on a listing built out of a directory it could not read: busy=%v\n%s", fresh, out)
	}
}

// THE CONTROL: it passed before the fix and must keep passing after it.
//
// A meta dir that does not exist is not an abstention. It is a box that
// holds no sessions — a fresh install, a state dir nothing has written yet —
// and the honest answer is the free seat it gives now. The cheapest wrong
// fix for the two tests above is `if err != nil { hold }` in metaNames'
// callers, and that fix reds this one: every lane in a fresh shop would
// stop hiring.
func TestQAAMissingMetaDirIsStillAFreeSeat(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dir := "/src/posse"

	if err := os.RemoveAll(b.metaDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadDir(b.metaDir()); err == nil {
		t.Fatalf("premise: the meta dir must be GONE — a present dir makes this control measure nothing")
	}
	if names, err := b.metaNames(); len(names) != 0 || err != nil {
		t.Fatalf("premise: metaNames over a missing dir is empty and NOT an error — a dir that was never written is no sessions: %v, %v", names, err)
	}

	if name, st := d.personaActive("developer", dir); name != "" {
		t.Errorf("a box with no meta dir at all froze a seat: personaActive = %q %q, want free — a directory that was never written is no sessions, not an unanswered question (ranger-base-jzxrh's control)", name, st)
	}
}

// ranger-base-eq3ba (finding 1b): the seat status is one word for both
// arms, so the LINE above the lane lines is where an operator learns which
// reading failed — and it used to prescribe one repair for both.
//
// `seatUnreadable` has two producers: ranger-base-3yqyg's (herdr declined to
// list) and this bead's (the meta directory the walk starts from could not be
// read). reconcileSeats told every one of them "repair at herdr, not at a
// meta", which sends an operator whose state dir is unreadable to restart a
// herdr that is working. The listing's error names the meta dir when that is
// the reading that failed, so the fix is that the line points AT the error
// rather than asserting a place.
//
// Two arms, because "names the state dir" alone is satisfied by a line that
// names it always: the herdr arm must still be about herdr.
func TestQAAnUnreadableMetaDirIsRepairedWhereTheErrorPoints(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	slot := SessionFor("ranger", "/repo")
	busy := map[string]string{slot: "a-1"}
	md := b.metaDir()
	qaUnreadableMetaDir(t, b)

	d.reconcileSeats(busy)
	out := dispatcherOut(d)
	if !strings.Contains(out, "herd unreadable") {
		t.Fatalf("premise: the unreadable-listing line did not print at all, so nothing below is measured:\n%s", out)
	}
	if !strings.Contains(out, md) {
		t.Errorf("the line does not name %s — the operator is told a listing failed and not which reading of it, and this is the one the error can name:\n%s", md, out)
	}
	if strings.Contains(out, "repair at herdr") {
		t.Errorf("a meta DIRECTORY that could not be read was reported as a repair at herdr: the operator restarts a working herdr and the state dir stays unreadable:\n%s", out)
	}
	// And the holds are kept whatever the line says — the wording is the
	// only thing this test is about (TestQAAnUnreadableHerdKeepsEverySeatHold
	// owns the other half).
	if busy[slot] != "a-1" {
		t.Errorf("reconcileSeats released a hold under an unreadable meta dir: busy=%v", busy)
	}

	// THE OTHER ARM: herdr's own failure still reports herdr's own error,
	// and says nothing about a directory it read fine.
	if err := os.Chmod(md, 0o755); err != nil {
		t.Fatal(err)
	}
	qaListError(t, fake)
	d2 := newTestDispatcher(t, b)
	d2.reconcileSeats(map[string]string{slot: "a-1"})
	herd := dispatcherOut(d2)
	if !strings.Contains(herd, "herd unreadable") {
		t.Fatalf("premise: the herdr arm printed no line:\n%s", herd)
	}
	if strings.Contains(herd, md) {
		t.Errorf("a herdr that would not list was reported against the meta dir, which read fine — the line names a place off the error or it names one always:\n%s", herd)
	}
}
