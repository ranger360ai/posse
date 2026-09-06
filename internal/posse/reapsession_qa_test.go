//go:build posse_arm2

package posse

// ranger-base-ktiik: what the reaper's candidate set actually is, and the
// silence that was read as it being the wrong one.
//
// THE REPORT. Three sessions on 2026-09-05 — `<persona>-<repo>-ranger-base-1se2l`,
// `<persona>-<repo>-ranger-base-dr0fu` and `<persona>-<repo>-ranger-base-rflee` — sat
// `done` over a CLOSED bead for 2-5 passes each and were killed by hand. Each
// of their beads had been claimed twice (an older session branch under
// another persona, released that morning), and the log showed the landing
// sweep retiring that OTHER branch for the same bead id, so the reading filed
// with the bead was that the sweep finds one branch per bead id, judges it,
// and stops.
//
// IT DOES NOT, and the first test here is that measurement: the sweep walks
// SESSIONS, and two live sessions holding one bead id are both taken in one
// pass. Keying candidates on the bead would take exactly one of them.
//
// WHAT THE OPERATOR WAS ACTUALLY READING is the rest of this file. Running
// the sweep backwards over the records that survived those three kills leaves
// one path standing (autoreap.go's note on the block this pins): all three
// wore a CREW mark, and a crew-marked per-bead session over a closed bead is
// held for the whole of `reap_crew_after` — four hours by default — silently,
// on every pass. From outside, a keep nothing says is indistinguishable from
// a reaper that does not run, which is what three hand-reaps and this bead
// cost. So each grace keep now says itself, carrying what is LEFT of the
// grace so the line reads as transient, and each is paired below with the
// nearest shape that must stay silent — a keep said about every session would
// be no more readable than a keep said about none.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// idleFleet gives every workspace the fake herdr hands out an idle agent.
// idleClaude answers for w1 alone, which is one session; every test here
// needs at least two on the board at once.
func idleFleet(t *testing.T, fake string) {
	t.Helper()
	var rows []string
	for _, w := range []string{"w1", "w2", "w3", "w4"} {
		rows = append(rows, `{"agent":"claude","agent_status":"idle","pane_id":"`+w+`:p1","workspace_id":"`+w+`"}`)
	}
	if err := os.WriteFile(filepath.Join(fake, "agents.json"), []byte("["+strings.Join(rows, ",")+"]"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// claimSession is wtSession with the persona as a parameter and the bd
// fixture left to the caller: two claims of one bead are two sessions under
// two personas over ONE store, so the fixture has to be committed once,
// before either worktree is cut, or the second tree does not carry it.
func claimSession(t *testing.T, b *HerdrBackend, repo, persona, bead string, crew bool, age time.Duration) string {
	t.Helper()
	name := SessionForBead(persona, repo, bead)
	if _, err := b.App.EnsureSessionTree(repo, name, nil); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateSession(NewSessionOpts{
		Name: name, Dir: repo, Agent: persona, Bead: bead, Crew: crew, Worktree: true,
	}); err != nil {
		t.Fatal(err)
	}
	ageResidue(t, b, name, age)
	return name
}

// ─── the candidate set is the session, not the bead ──────────────────────────

// The bead's own asked-for pin: two sessions and two branches for one bead
// id, the bead closed, both reaped in one pass. A sweep that found one
// session per bead id would take whichever it reached first and leave the
// other standing forever — which is the shape the released stale claims
// produced nine times over on 2026-09-05.
func TestAutoReapTakesBothSessionsOfATwiceClaimedBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	writePersona(t, b.App, "scout", "[go]")
	repo := wtRepo(t)
	commitIn(t, repo, "fake-show.json", `[{"id":"a-1","status":"closed"}]`, "bd fixture")
	first := claimSession(t, b, repo, "scout", "a-1", false, time.Hour)
	second := claimSession(t, b, repo, "ranger", "a-1", false, time.Hour)
	idleFleet(t, fake)

	d.autoReapPass(afterRouting)

	out := dispatcherOut(d)
	if _, ok := b.readMeta(first); ok {
		t.Errorf("the first claim's session holds one bead id and one session name — it is the sweep's:\n%s", out)
	}
	if _, ok := b.readMeta(second); ok {
		t.Errorf("a bead claimed twice has TWO sessions, and a candidate set keyed on the bead id would leave this one standing forever (ranger-base-ktiik):\n%s", out)
	}
}

// ─── the crew grace says itself ──────────────────────────────────────────────

// The measured silence. A per-bead session the operator stepped into, over a
// bead the store of record calls CLOSED, inside `reap_crew_after`: the keep
// is correct — ADR 0008 says the operator may still be talking to it — and
// saying nothing about it for four hours is not, because the only reading
// left is "the reaper is not running".
func TestAutoReapNamesTheCrewGraceItIsHoldingASessionFor(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	commitIn(t, repo, "fake-show.json", `[{"id":"a-1","status":"closed"}]`, "bd fixture")
	name := claimSession(t, b, repo, "ranger", "a-1", true, DefaultCrewReapAfter-time.Hour)
	idleFleet(t, fake)

	d.autoReapPass(afterRouting)

	out := dispatcherOut(d)
	if _, ok := b.readMeta(name); !ok {
		t.Fatalf("inside its grace the session is KEPT — that half is ADR 0008's and does not change:\n%s", out)
	}
	for _, want := range []string{name, "NOT reaped", "reap_crew_after", "bead a-1 closed"} {
		if !strings.Contains(out, want) {
			t.Errorf("the keep must name the session, the bead it is over and the dial that spells it — no %q in:\n%s", want, out)
		}
	}
	// What is LEFT, not how long it has been idle: a grace keep stops being
	// true by itself, and the number that says when is the one the operator
	// can act on.
	if !strings.Contains(out, "1h0m0s left") {
		t.Errorf("the line must carry what is left of the grace, so it reads as transient rather than as a standing refusal:\n%s", out)
	}
}

// The refusal that keeps the line above from being "say something about
// every crew session". Same session, same mark, same grace — an OPEN bead,
// which is a persona still working and not a keep anybody is waiting on. The
// sweep has always been silent there and stays silent: a keep printed for
// every live session is exactly as unreadable as no keep at all.
func TestAutoReapSaysNothingAboutACrewSessionWhoseBeadIsStillOpen(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	commitIn(t, repo, "fake-show.json", `[{"id":"a-1","status":"in_progress"}]`, "bd fixture")
	name := claimSession(t, b, repo, "ranger", "a-1", true, DefaultCrewReapAfter-time.Hour)
	idleFleet(t, fake)

	d.autoReapPass(afterRouting)

	out := dispatcherOut(d)
	if _, ok := b.readMeta(name); !ok {
		t.Fatalf("an open bead is never reaped:\n%s", out)
	}
	if strings.Contains(out, name) {
		t.Errorf("an open bead's session is a persona at work, not a keep the operator is waiting on — the sweep says nothing:\n%s", out)
	}
}

// The unpointed arm's own grace, on the same rule: kftx tagged this
// population 🏷️no-bead so the boundary would be visible instead of silent,
// and the grace it then waits out was the silence again.
func TestAutoReapNamesTheUnpointedGraceItIsHoldingASessionFor(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	name := SessionForBead("ranger", repo, "a-1")
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: repo, Agent: "ranger"}); err != nil {
		t.Fatal(err)
	}
	ageResidue(t, b, name, DefaultUnpointedReapAfter/2)
	idleFleet(t, fake)

	d.autoReapPass(afterRouting)

	out := dispatcherOut(d)
	if _, ok := b.readMeta(name); !ok {
		t.Fatalf("inside its grace the session is KEPT:\n%s", out)
	}
	for _, want := range []string{name, "NOT reaped", "no bead pointer", "reap_unpointed_after"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in the unpointed keep:\n%s", want, out)
		}
	}
}

// ─── a keep that will never expire says so ───────────────────────────────────

// The other silent return in the same block, and the one that is not
// transient at all: a record carrying neither stamp residueIdle reads has no
// age, is therefore never old enough, and was skipped by every sweep forever
// with nothing said. Fail-closed is unchanged — no age never licenses a kill
// — and the sentence is what is new.
func TestAutoReapNamesASessionWhoseIdleCannotBeRead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	commitIn(t, repo, "fake-show.json", `[{"id":"a-1","status":"closed"}]`, "bd fixture")
	name := claimSession(t, b, repo, "ranger", "a-1", true, 0)
	m, ok := b.readMeta(name)
	if !ok {
		t.Fatalf("no run record for %s", name)
	}
	m.Launched, m.Prompted = time.Time{}, time.Time{}
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	idleFleet(t, fake)

	d.autoReapPass(afterRouting)

	out := dispatcherOut(d)
	if _, ok := b.readMeta(name); !ok {
		t.Fatalf("no age is not old enough — the session is kept:\n%s", out)
	}
	if !strings.Contains(out, "when it was last touched") {
		t.Errorf("a keep nothing will ever expire is the one that most needs saying:\n%s", out)
	}
}
