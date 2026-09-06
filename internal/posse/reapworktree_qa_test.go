//go:build posse_arm2

package posse

// ranger-base-fytno: the reap sweep over the shape the whole fleet actually
// runs in — a session whose Dir is its own git WORKTREE and whose name was
// built from the CHECKOUT that worktree hangs off (rangerhq-09o2).
//
// MEASURED 2026-09-04: eight finished sessions held their seats for up to 9h
// — four of them for nine — while the same passes reaped six others on
// cadence and said not one word about the eight. Two silences in autoreap.go
// can produce exactly that, and both are pinned here:
//
//   - reapClassOf compared a session's name against SessionFor(agent, Dir).
//     For a worktree session Dir's basename IS the session name, so that
//     renders `ranger-<name>` and matches nothing — which made the crew arm
//     (ranger-base-f6lk) unreachable for every dispatched session on a
//     worktree fleet. Not "reaped late": never.
//   - reapWhy folded a bd read FAILURE into "the bead is not closed", both
//     returning false with nothing printed. A session whose store cannot be
//     read was therefore skipped by every sweep forever, and no line anywhere
//     said so.
//
// Every test below is a pair, the way reapresidue_test.go's are: what the
// sweep must now TAKE, and the nearest shape it must still refuse. A widened
// arm on its own is not a predicate.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// wtSession builds the live shape: a real checkout, a per-bead session whose
// name comes from the CHECKOUT (as dispatch renders it — SessionForBead(persona,
// is.Dir, is.ID), where is.Dir is the repo the bead lives in), and its own
// worktree underneath.
//
// The `bd show` fixture is COMMITTED to the checkout before the worktree is
// cut, for two reasons that are both the live shape and not a convenience:
// reapWhy runs bd in the session's own worktree, which is where a tracked
// file is and an untracked one is not; and the kill the sweep then performs
// is guarded by residueHolds, which refuses over a dirty tree — so a fixture
// left untracked would make every reap below fail for a reason that has
// nothing to do with what is being pinned. beadStatus "" commits nothing and
// leaves the store unable to answer at all.
func wtSession(t *testing.T, b *HerdrBackend, repo, bead, beadStatus string, crew bool, age time.Duration) string {
	t.Helper()
	name := SessionForBead("ranger", repo, bead)
	if beadStatus != "" {
		commitIn(t, repo, "fake-show.json", `[{"id":"`+bead+`","status":"`+beadStatus+`"}]`, "bd fixture")
	}
	if _, err := b.App.EnsureSessionTree(repo, name, nil); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateSession(NewSessionOpts{
		Name: name, Dir: repo, Agent: "ranger", Bead: bead, Crew: crew, Worktree: true,
	}); err != nil {
		t.Fatal(err)
	}
	// The record must say what the live ones say, or the pin measures a
	// shape the launcher never produces: Dir is the worktree, Repo is the
	// checkout the NAME was built from.
	m, ok := b.readMeta(name)
	if !ok {
		t.Fatalf("no run record for %s", name)
	}
	if m.Repo != repo || m.Dir == repo {
		t.Fatalf("fixture is not the worktree shape: dir=%s repo=%s", m.Dir, m.Repo)
	}
	if d := dirtyPaths(m.Dir); len(d) > 0 {
		t.Fatalf("fixture worktree is dirty (%v) — the kill guard would refuse for the wrong reason", d)
	}
	ageResidue(t, b, name, age)
	return name
}

// ─── the crew arm, on a worktree fleet ───────────────────────────────────────

// ranger-base-f6lk's population in the shape it actually occurs in: a session
// dispatch created for one bead, crew-marked afterwards (cockpit `p`, `posse
// prompt`), over a closed bead, over a tree that holds nothing, past its
// grace. On a worktree fleet the arm could not fire at all before this bead:
// the name check it is gated on was asked of the worktree instead of the
// checkout and never matched.
func TestAutoReapTakesACrewMarkedWorktreeSessionPastItsGrace(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	name := wtSession(t, b, repo, "a-1", "closed", true, DefaultCrewReapAfter+time.Minute)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); ok {
		t.Errorf("a crew-marked per-bead session in its own worktree, closed and settled past its grace, must be reaped — this is f6lk's arm on the only shape the fleet has:\n%s", dispatcherOut(d))
	}
	if !strings.Contains(dispatcherOut(d), "crew-marked on a session dispatch made") {
		t.Errorf("the line must say which arm took it:\n%s", dispatcherOut(d))
	}
}

// The refusal that keeps the arm above from being "take every crew session".
// Same worktree, same closed bead, same age — a name the OPERATOR chose, so
// it is not a session dispatch made and ADR 0008's shield is whole. Without
// this the widening is satisfied by an arm that takes anything.
//
// The shape is CONSTRUCTED, not produced: `posse new` cuts no worktree
// (ranger-base-f6lk's note), so nothing on the box makes an operator-named
// session with a Repo. That is why it is here — the arm must key on the
// NAME and not on "crew + a worktree + a closed bead", and the only way to
// say so is to build the one shape where those two answers differ.
// reapresidue_test.go pins the same boundary in the shape that IS produced.
func TestAutoReapKeepsAnOperatorNamedCrewSessionInAWorktree(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	commitIn(t, repo, "fake-show.json", `[{"id":"a-1","status":"closed"}]`, "bd fixture")
	if _, err := b.App.EnsureSessionTree(repo, "ranger-scratch", nil); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateSession(NewSessionOpts{
		Name: "ranger-scratch", Dir: repo, Agent: "ranger", Bead: "a-1", Crew: true, Worktree: true,
	}); err != nil {
		t.Fatal(err)
	}
	ageResidue(t, b, "ranger-scratch", 30*24*time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-scratch"); !ok {
		t.Errorf("a crew session the operator NAMED is his conversation, however old and however its bead reads (ADR 0008):\n%s", dispatcherOut(d))
	}
}

// The other half of the same one-line change, and the half that is a
// behaviour CHANGE rather than a repair: the pre-Dial-F slot session — the
// persona's reusable `<persona>-<repobase>`, which the next resume rejoins
// (rangerhq-v330) — was rendered from the worktree too, so a slot session
// with a worktree fell straight through the guard meant to protect it and
// was taken as ordinary fleet residue. Rendered from the checkout it is
// recognised for what it is and survives, closed bead and all.
func TestAutoReapKeepsTheSlotSessionWhenItHasAWorktree(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	slot := SessionFor("ranger", repo)
	commitIn(t, repo, "fake-show.json", `[{"id":"a-1","status":"closed"}]`, "bd fixture")
	if _, err := b.App.EnsureSessionTree(repo, slot, nil); err != nil {
		t.Fatal(err)
	}
	// `bead:` on the slot is NoteBead's stamp — whichever bead last resumed
	// into it (ADR 0004 §2) — which is exactly why a pointer alone must not
	// make a name disposable.
	if err := b.CreateSession(NewSessionOpts{
		Name: slot, Dir: repo, Agent: "ranger", Bead: "a-1", Worktree: true,
	}); err != nil {
		t.Fatal(err)
	}
	ageResidue(t, b, slot, 30*24*time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(slot); !ok {
		t.Errorf("the persona's reusable slot is what the next resume rejoins and is never Dial F's to reap, however its last bead reads:\n%s", dispatcherOut(d))
	}
}

// ─── an unreadable store is not an open bead ─────────────────────────────────

// The other silence. `bd show` failing in the session's worktree — a stale
// or missing store, a bd that refuses, a timeout — used to return exactly
// what "the bead is still open" returns: false, and no line. So the session
// sat over a question nobody could answer, on every pass, forever, and the
// log the operator reads said only what it reaped.
//
// Fail-closed is unchanged and is the point of the first assertion: an
// unanswerable store never licenses a kill. What is new is the second.
func TestAutoReapNamesASessionWhoseBeadCannotBeRead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	name := wtSession(t, b, repo, "a-1", "", false, time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a store that cannot answer must never license a kill:\n%s", dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, name) || !strings.Contains(out, "NOT reaped") || !strings.Contains(out, "cannot be read") {
		t.Errorf("the skip must NAME the session and say the store could not be read, every pass it is true — a silent forever-skip is indistinguishable from a reaper that does not run:\n%s", out)
	}
}

// And the pair: the same session once the store answers. Without this the
// test above is satisfied by a sweep that refuses everything and prints a
// line about it.
func TestAutoReapTakesTheSameSessionOnceItsBeadCanBeRead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	name := wtSession(t, b, repo, "a-1", "closed", false, time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); ok {
		t.Errorf("a fleet session in its own worktree, bead closed and agent idle, is the sweep's:\n%s", dispatcherOut(d))
	}
	if strings.Contains(dispatcherOut(d), "NOT reaped") {
		t.Errorf("a readable store must cost no refusal line:\n%s", dispatcherOut(d))
	}
}

// ─── the bounce ──────────────────────────────────────────────────────────────

// ranger-base-fytno's own pin, in its own words: start a session under one
// Run, close its bead, bounce the watch, assert the next pass reaps it.
//
// The bounce is a SECOND Dispatcher over the same home, holding none of the
// first one's memory — which is what `--resume` and a watch restart both
// leave behind. The sweep's candidate set is `d.HB.Sessions()`, every meta on
// disk plus every workspace herdr holds, so nothing here should depend on who
// fired the session; this asserts that, so that no future narrowing of the
// candidate set to "sessions this Run fired" can land green. (The Run-scoped
// set that exists — the seat map reconcileSeats reconciles — is a different
// question and stays where it is: ranger-base-6swlr's abstention.)
func TestAutoReapTakesASessionAnotherRunFiredAndBounced(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)

	// Run A fires it and prompts it: its own sweep must leave it alone.
	runA := newTestDispatcher(t, b)
	name := wtSession(t, b, repo, "a-1", "closed", false, 0)
	runA.notePrompted(name)
	idleClaude(t, fake)
	runA.autoReapPass(afterRouting)
	if _, ok := b.readMeta(name); !ok {
		t.Fatalf("a session prompted seconds ago survives its own Run's sweep (PromptGrace):\n%s", dispatcherOut(runA))
	}

	// The bounce: a reboot, a `make install`, a wedge — the watch comes back
	// as a new process and re-seats this session by name. Past PromptGrace,
	// with the bead closed, the very next pass is the one that takes it.
	agePrompt(t, b, name, runA.PromptGrace+time.Minute)
	ageLaunch(t, b, name, runA.PromptGrace+time.Minute)
	runB := newTestDispatcher(t, b)
	runB.autoReapPass(beforeRouting)

	if _, ok := b.readMeta(name); ok {
		t.Errorf("a session THIS Run never fired is still the sweep's — the candidate set is every live session over a closed bead, not the ones this process launched (ranger-base-fytno: 8 seats held up to 9h):\n%s", dispatcherOut(runB))
	}
	if !strings.Contains(dispatcherOut(runB), "reaped "+name) {
		t.Errorf("the bounced Run must say what it took:\n%s", dispatcherOut(runB))
	}
	// And the seat is free because the session is gone, not because a
	// listing abstained: nothing is left on disk to hold it.
	if ents, _ := os.ReadDir(filepath.Join(b.App.StateDir, "herdr")); len(ents) != 0 {
		t.Errorf("the reap must leave no record holding the seat, got %d", len(ents))
	}
}
