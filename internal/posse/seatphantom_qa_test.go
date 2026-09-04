package posse

// ranger-base-ifjgm: a seat that stays BUSY after its session is gone.
//
// MEASURED 2026-09-03 (state/dispatch-watch.log, seat-cadence.log). 16:04:51Z
// the watch launched a code-lane seat on ranger-base-buvq4; 16:14:07Z it
// settled "done with 1 shell, 1 monitor still running — waiting, not judged
// this pass" and was then reaped ("bead closed, 1 commit rebased and
// fast-forwarded onto main; worktree removed"). From there until the loop was
// bounced at 18:26Z — 2h12m — every refill named all three code seats busy
// and hired into 2 of 3, while `posse list` showed no pane for the reaped one
// at all and seat-cadence recorded no later launch on it. The same shape ran
// 04:53Z-12:05Z that morning (noted on ranger-base-wj7e9 as unexplained).
// Only a new process — a new busy map — ever released it.
//
// The cause is that the Run's seat map has exactly ONE release: the gather
// loop's `delete(busy, seat)`, which it reaches only for a settle it judged
// done. A settle that came back `working` drops out of `active` and is never
// looked at again, so its hold outlives the session by the whole life of the
// Run. reconcileSeats is the second release, and these are its pins: what it
// must let go of, and — the arms that matter more — the three things it must
// NOT.

import (
	"path/filepath"
	"strings"
	"testing"
)

// seatFire is one fire pass over one bead, sharing the caller's Run maps —
// what Run does at its head and what every refire does after (refire →
// fireLoop). The prompts it leaves in flight are joined by the caller.
func seatFire(t *testing.T, d *Dispatcher, repo, id string, busy map[string]string, sessFail map[string]int) []*pendingBead {
	t.Helper()
	beads := []RepoIssue{{BdIssue: BdIssue{ID: id, Title: "t", Labels: []string{"go"}}, Dir: repo}}
	_, pending, _, err := d.fireLoop(beads, "", 0, busy, sessFail)
	if err != nil {
		t.Fatal(err)
	}
	return pending
}

// The bead, in two fire passes over one busy map.
//
// a-1 fires and the Run records the hold. Its settle came back `working`, so
// the gather never released it — the fixture simply never calls the gather,
// which is exactly the state the log was in. Then the session is reaped on a
// later pass, and the next refill must find the seat free.
//
// MUTATION: drop the `d.reconcileSeats(busy)` call at the head of fireLoop →
// a-2 is skipped `lane busy: ranger` and the seat is held on a-1 forever →
// red on both assertions below.
func TestQAAReapedSeatIsReleasedByTheNextFirePass(t *testing.T) {
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
		t.Fatalf("the fixture must actually seat a-1, or the release proves nothing: busy=%v\n%s", busy, dispatcherOut(d))
	}
	// The reap, on a later pass: bead closed, session gone, worktree removed.
	if err := b.KillSession(SessionForBead("ranger", repo, "a-1")); err != nil {
		t.Fatal(err)
	}

	inFlight = append(inFlight, seatFire(t, d, repo, "a-2", busy, sessFail)...)
	out := dispatcherOut(d)
	if busy[slot] != "a-2" {
		t.Errorf("a seat whose session was reaped is not occupancy — the next fire pass must release it and seat a-2, not read `lane busy` for 2h12m: busy=%v\n%s", busy, out)
	}
	if want := "↺ seat " + slot + " released: no session (held a-1)"; !strings.Contains(out, want) {
		t.Errorf("a released seat must say so in the log — a phantom the operator cannot see is the 09-03 incident: want %q\n%s", want, out)
	}
}

// The arm that must not fire, and the one a release-everything fix would
// break: the seat's session is still LIVE, so the hold stands and the next
// bead waits. Same two passes, same map; only the reap is missing.
//
// MUTATION: make reconcileSeats delete every hold (drop the `live` test) →
// a-2 is seated beside a live a-1 session, one persona two beads, which is
// what ADR 0028 §3's occupancy is for → red.
func TestQAALiveSeatKeepsItsHoldAcrossFirePasses(t *testing.T) {
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
		t.Fatalf("the fixture must actually seat a-1: busy=%v\n%s", busy, dispatcherOut(d))
	}
	mark := len(dispatcherOut(d))

	inFlight = append(inFlight, seatFire(t, d, repo, "a-2", busy, sessFail)...)
	out := dispatcherOut(d)[mark:]
	if busy[slot] != "a-1" {
		t.Errorf("a seat whose session is still live is still occupied — the hold must stand: busy=%v\n%s", busy, out)
	}
	if strings.Contains(out, "released: no session") {
		t.Errorf("nothing was reaped; no seat may be released:\n%s", out)
	}
	if !strings.Contains(out, "lane busy: ranger") {
		t.Errorf("a-2 must wait for the seat, not be launched beside a live a-1:\n%s", out)
	}
	if strings.Contains(calls(t, fake), "workspace create --label "+SessionForBead("ranger", repo, "a-2")) {
		t.Errorf("a-2 got a session while a-1's was live — one persona, two beads:\n%s", out)
	}
}

// Fail-closed: a session listing that could not be READ is not an empty
// herd. Sessions() applies the same rule to its own metas (rangerhq-9nso),
// and a release taken on a failed read would empty every seat of a Run the
// first time herdr hiccupped.
//
// The break is the herdr binary itself, which is what a dead server, a bad
// socket and an unreadable listing all reach this code as: an error out of
// Sessions().
//
// MUTATION: treat the error as an empty listing (drop the `if err != nil`
// return) → the hold is released on no evidence at all → red.
func TestQAAnUnreadableHerdKeepsEverySeatHold(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	slot := SessionFor("ranger", "/repo")
	busy := map[string]string{slot: "a-1"}

	// The control: with a readable (and genuinely empty) herd the same map
	// releases, so what the arm below measures is the ERROR and not the
	// absence of sessions.
	d.reconcileSeats(busy)
	if busy[slot] != "" {
		t.Fatalf("control: a readable, empty herd releases the hold — without this the arm below proves nothing: busy=%v", busy)
	}

	busy[slot] = "a-1"
	mark := len(dispatcherOut(d)) // the control's own release line is above this
	b.H = Herdr{Bin: filepath.Join(t.TempDir(), "herdr-that-is-not-there")}
	d.reconcileSeats(busy)
	out := dispatcherOut(d)[mark:]
	if busy[slot] != "a-1" {
		t.Errorf("an unreadable herd is not an empty one: a hold may only be released on evidence, never on a failed read: busy=%v\n%s", busy, out)
	}
	if strings.Contains(out, "released: no session") {
		t.Errorf("a failed reading may not be reported as a released seat:\n%s", out)
	}
}

// --dry-run holds seats it never launched into (fireLoop's dry branch calls
// seats.hold with no session behind it), so reconciling a dry pass would
// release every one of its own holds and let it report firing the same seat
// twice — a diagnostic that lies about what the real pass would do.
//
// MUTATION: drop the `d.DryRun` arm from reconcileSeats' guard → the hold
// goes → red.
func TestQAADryRunKeepsTheSeatsItOnlyPretendedToFill(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	slot := SessionFor("ranger", "/repo")
	busy := map[string]string{slot: "a-1"}

	d.reconcileSeats(busy)
	if busy[slot] != "a-1" {
		t.Errorf("a dry pass's holds have no sessions by construction; reconciling them would let it seat the same persona twice: busy=%v\n%s", busy, dispatcherOut(d))
	}
}

// seatSession names the session a hold belongs to, from the SLOT alone.
// `seatSession(slot, id) == SessionForBead(persona, dir, id)` is true by
// construction — SessionForBead delegates here — so asserting it would
// assert nothing: mutate the body and both sides move together (measured,
// M5 survived that spelling). The pin is the LITERAL, and the literal is the
// incident's own session name.
//
// MUTATION: change the separator or drop the sanitize → red.
func TestSeatSessionSpellsTheDialFName(t *testing.T) {
	t.Parallel()
	slot := SessionFor("developer", "/src/posse")
	if got, want := slot, "developer-posse"; got != want {
		t.Fatalf("the seat's slot is %q, not %q — the rest of this pin reads the wrong name", got, want)
	}
	if got, want := seatSession(slot, "ranger-base-ifjgm"), "developer-posse-ranger-base-ifjgm"; got != want {
		t.Errorf("seatSession = %q, want the Dial F name %q", got, want)
	}
	if got, want := seatSession(slot, "a/b"), "developer-posse-a-b"; got != want {
		t.Errorf("a bead id is sanitized into the session name: %q, want %q", got, want)
	}
}
