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
// The first two tests are PARKED (t.Skip) because they fail on main today:
// they are the shape of the fix, not a claim about it. Unpark by deleting
// the t.Skip line. Both were shown able to fail before they were parked —
// run unparked through `go test -overlay`, both fail with the message they
// carry.
//
// The THIRD is not parked and must stay green: it is the control that keeps
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
	if names := b.metaNames(); len(names) != 0 {
		t.Fatalf("premise: metaNames must come back EMPTY for these assertions to be about the seat: %v", names)
	}
}

// PARKED (ranger-base-jzxrh). Both arms, at the seat walk itself.
func TestQAAnUnreadableMetaDirHoldsTheSeat(t *testing.T) {
	t.Parallel()
	// PARKED, and t.Parallel stays ABOVE it so cmd/testparallel counts this
	// test as parallel-safe the day it is unparked (make verify-parallel
	// reads the first line and does not follow a skip).
	t.Skip("PARKED: fails on main — ranger-base-jzxrh. Unpark by deleting this line.")
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

	// ARM 2, and it pre-dates ranger-base-3yqyg: NO listing error at all.
	// listSessions walks metaNames, finds nothing to list and nothing to
	// withhold, and returns a clean empty answer with err == nil — so
	// nothing above personaActive aborts either.
	if sess, withheld, err := b.listSessions(); err != nil || len(sess) != 0 || len(withheld) != 0 {
		t.Fatalf("premise: the listing answers CLEAN and empty on an unreadable meta dir: %d sessions, withheld=%v, err=%v", len(sess), withheld, err)
	}
	if name, st := d.personaActive("developer", dir); name == "" {
		t.Errorf("an unreadable meta DIR read as a FREE seat while the session is live: personaActive = %q %q, want the seat held — the listing did not answer for it, it could not be asked", name, st)
	}

	// ARM 1: ranger-base-3yqyg's own trigger on top of it. Its err arm
	// answers out of metaNames, which is the thing that cannot read.
	qaListError(t, fake)
	if _, _, err := b.listSessions(); err == nil {
		t.Fatalf("premise: the listing must fail")
	}
	if name, st := d.personaActive("developer", dir); name == "" {
		t.Errorf("the err arm reported a FREE seat when metaNames' own ReadDir failed: personaActive = %q %q — 88c2507 claims an unreadable listing holds the seat it cannot answer for", name, st)
	}
}

// PARKED (ranger-base-jzxrh). The harm, end to end, on the shape that is
// actually reachable — the same pass ranger-base-3yqyg pins for its own
// lever, with the meta dir as this one.
func TestQAAFreshRunFindsNoFreeSeatUnderAnUnreadableMetaDir(t *testing.T) {
	t.Parallel()
	// PARKED, and t.Parallel stays ABOVE it so cmd/testparallel counts this
	// test as parallel-safe the day it is unparked (make verify-parallel
	// reads the first line and does not follow a skip).
	t.Skip("PARKED: fails on main — ranger-base-jzxrh. Unpark by deleting this line.")
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

// THE CONTROL, and it is NOT parked: it passes today and must keep passing.
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
	if names := b.metaNames(); len(names) != 0 {
		t.Fatalf("premise: metaNames over a missing dir is empty: %v", names)
	}

	if name, st := d.personaActive("developer", dir); name != "" {
		t.Errorf("a box with no meta dir at all froze a seat: personaActive = %q %q, want free — a directory that was never written is no sessions, not an unanswered question (ranger-base-jzxrh's control)", name, st)
	}
}
