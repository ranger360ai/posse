//go:build posse_arm2

package posse

// The `working`-over-a-closed-bead arm of the end-of-pass sweep —
// ranger-base-4yefx, ask 1.
//
// WHAT IT IS PINNING. A claude seat that ends its turn with a background task
// live reports `working` with no end (MEASURED 2026-11-23, 36 samples across
// one seat's own turn boundary: 36 of 36 root program_status records
// `working`, never idle, never done — reapworking.go has the table). So
// settledForReap's "working means somebody is in there" held one lane for
// 41.6h over a CLOSED bead and a clean tree, and said nothing on any of the
// hundreds of sweeps in that window.
//
// EVERY ARM HERE IS A PAIR, on reapresidue_test.go's rule: a widened sweep
// alone is "the sweep takes more", which is not the bead. The bead asked for
// a predicate, and a predicate is only pinned by what it REFUSES — so each
// reap below is followed by the nearest shape that must survive it, and the
// refusals are the arms that matter most, because the conjunction is what
// makes a screen reading safe to spend on a destructive sweep.
//
// It lives in arm 2 because its two fixtures do: residueSession/ageResidue
// (reapresidue_test.go) and armScreen, which seeds the region previews herdr
// hands panework.go (settlewaiting_qa_test.go).
//
//	go test -tags posse_arm2 -run TestAutoReapWorking ./internal/posse

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The two footers that differ only in whether a turn is running, as measured
// 2026-11-23 on this box — the in-turn one at sample 22 and the post-turn one
// at sample 24, twenty seconds apart, with the same background task live in
// both. `busyFooter` next door is the same post-turn shape measured
// 2026-09-02; this file spells its own pair so the pin reads on today's
// bytes and names the turn in both directions.
const (
	liveTurnFooter  = "  ⏵⏵ auto mode on · 1 shell · esc to interrupt · ↓ to manage\n"
	endedTurnFooter = "  ⏵⏵ auto mode on · 1 shell · ↓ to manage\n"
)

// workingSeatOverAClosedBead is the incident's own shape: dispatch's per-bead
// session, its bead closed, the occupant reading `working`, the screen
// carrying whatever the caller gives it, and both stamps aged by `by`.
func workingSeatOverAClosedBead(t *testing.T, by time.Duration, footer, box string) (*HerdrBackend, *Dispatcher, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir, Bead: "a-1"}, "closed", by)
	workingClaude(t, fake)
	armScreen(t, fake, footer, box)
	return b, d, name
}

// ─── the arm ─────────────────────────────────────────────────────────────────

func TestAutoReapWorkingTakesASeatWhoseTurnHasEndedOverAClosedBead(t *testing.T) {
	t.Parallel()
	b, d, name := workingSeatOverAClosedBead(t, DefaultWorkingReapAfter+time.Minute, endedTurnFooter, emptyBox)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); ok {
		t.Errorf("a per-bead seat whose bead is CLOSED, whose footer names live background work with no turn in flight, and whose tree holds nothing is the sweep's — this is the 41.6h:\n%s", dispatcherOut(d))
	}
	out := dispatcherOut(d)
	// The line says which arm took it and on what reading, for the same
	// reason the crew arm's does: the operator reads this to know what the
	// sweep decided and on what.
	if !strings.Contains(out, "reaped "+name+" (bead a-1 closed, holding 1 shell with no turn in flight, idle ") {
		t.Errorf("the reap line must name the evidence that took it:\n%s", out)
	}
}

// The hint, and it is the whole safety property: claude's own `esc to
// interrupt` in the same footer at the same instant says a turn IS in flight,
// and a closed bead does not make that turn disposable — a persona closes its
// bead and then writes its ORDERS.md lessons down.
func TestAutoReapWorkingKeepsASeatWithATurnInFlight(t *testing.T) {
	t.Parallel()
	b, d, name := workingSeatOverAClosedBead(t, DefaultWorkingReapAfter+time.Minute, liveTurnFooter, emptyBox)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a seat whose footer carries claude's interrupt hint has a LIVE TURN — reaping it over a closed bead kills the wrap-up:\n%s", dispatcherOut(d))
	}
	if out := dispatcherOut(d); !strings.Contains(out, "a turn is in flight") || !strings.Contains(out, footerInterruptHint) {
		t.Errorf("the keep must name the hint it read, or the reading is untraceable:\n%s", out)
	}
}

// A footer with no task vocabulary in it is not evidence of anything, and
// neither is a screen posse could not read — both answer the zero hold. The
// LINE still fires, because a closed bead's seat holding a lane is the
// condition that went 41.6h unsaid (ranger-base-fytno, ranger-base-ktiik).
func TestAutoReapWorkingKeepsASeatItCannotReadAndSaysSo(t *testing.T) {
	t.Parallel()
	b, d, name := workingSeatOverAClosedBead(t, DefaultWorkingReapAfter+time.Minute, idleFooter, emptyBox)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("with no background work in the footer there is no evidence about what `working` means, and the seat stays:\n%s", dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "posse cannot see what it is working on") {
		t.Errorf("a held lane over a closed bead is said on every pass, whatever the screen gave:\n%s", out)
	}
	if !strings.Contains(out, "posse kill "+name+" --no-land") {
		t.Errorf("the line must name the command that ends it now — the operator typed two waits it could not honour before reaching that flag:\n%s", out)
	}
}

// Text in the box is somebody mid-composition, which the pulse's own G2 row
// treats as a hold (panework.go). It keeps the seat here too.
func TestAutoReapWorkingKeepsASeatWithAnUnsentPrompt(t *testing.T) {
	t.Parallel()
	b, d, name := workingSeatOverAClosedBead(t, DefaultWorkingReapAfter+time.Minute, endedTurnFooter, unsentBox)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a box holding an unsent prompt is somebody typing — not a seat to close:\n%s", dispatcherOut(d))
	}
	if out := dispatcherOut(d); !strings.Contains(out, "UNSENT") {
		t.Errorf("the keep must name what is in the box:\n%s", out)
	}
}

// Inside the grace the seat stays, and the line is self-terminating: it says
// how much is left and the command that ends it now, which is the shape the
// crew and unpointed arms' own keeps already print.
func TestAutoReapWorkingKeepsASeatInsideItsGrace(t *testing.T) {
	t.Parallel()
	b, d, name := workingSeatOverAClosedBead(t, DefaultWorkingReapAfter-30*time.Minute, endedTurnFooter, emptyBox)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("inside `reap_working_after` the seat stays:\n%s", dispatcherOut(d))
	}
	if out := dispatcherOut(d); !strings.Contains(out, "left of the `reap_working_after` grace") {
		t.Errorf("the keep must name the dial and what is left of it:\n%s", out)
	}
}

// `off` disarms the KILL and not the line — the one place this dial differs
// from the two beside it, and deliberately (reapworking.go): the lane being
// held in silence was half this bug.
func TestAutoReapWorkingOffKeepsTheSeatAndStillNamesIt(t *testing.T) {
	t.Parallel()
	b, d, name := workingSeatOverAClosedBead(t, DefaultWorkingReapAfter+time.Minute, endedTurnFooter, emptyBox)
	write(t, b.App.ConfigPath, "reap_working_after: off\n")

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("`off` must restore the kill-nothing behaviour:\n%s", dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "the `reap_working_after` arm is off in config") {
		t.Errorf("a disarmed arm still names the held lane — the operator who disarmed it is the one who most needs to see it:\n%s", out)
	}
	if strings.Contains(out, "reaped "+name) {
		t.Errorf("`off` reaped the seat anyway:\n%s", out)
	}
}

// ─── the boundaries: who this arm is NOT for ─────────────────────────────────

// An OPEN bead over the same `working` reading is the ordinary case — a
// persona working — and nothing here may touch it. The screen is the same
// screen the positive arm reaps on, so this pin is the one that would catch a
// conjunction wired up with the bead read dropped.
func TestAutoReapWorkingKeepsASeatWhoseBeadIsOpen(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir, Bead: "a-1"},
		"in_progress", DefaultWorkingReapAfter+time.Hour)
	workingClaude(t, fake)
	armScreen(t, fake, endedTurnFooter, emptyBox)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a seat whose bead is still OPEN is a persona at work, whatever its footer says — the store of record is what licenses this arm:\n%s", dispatcherOut(d))
	}
	if out := dispatcherOut(d); strings.Contains(out, "NOT reaped over bead") {
		t.Errorf("an open bead is the ordinary case and costs no line at all — this sweep runs over every live seat on the box:\n%s", out)
	}
}

// `blocked` is the one unsettled state where somebody is demonstrably needed,
// and the screen posse would read is the dialog rather than the footer under
// it. It keeps exactly the behaviour it had before this arm existed, silently.
func TestAutoReapWorkingDoesNotTakeABlockedSeat(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir, Bead: "a-1"}, "closed", DefaultWorkingReapAfter+time.Hour)
	write(t, filepath.Join(fake, "agents.json"),
		`[{"agent":"claude","agent_status":"blocked","pane_id":"w1:p1","workspace_id":"w1"}]`)
	armScreen(t, fake, endedTurnFooter, emptyBox)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a blocked seat is a persona waiting on its own dialog — this arm is `working`'s alone:\n%s", dispatcherOut(d))
	}
}

// And the crew mark: a seat the operator stepped into reads `working` the
// same way, and it is not this arm's. It reaches reapCrewClosed, pays four
// hours, and is refused here whatever its screen says — a screen reading is
// not the thing to spend on a population the operator did not previously
// trust the sweep to decide anything about.
func TestAutoReapWorkingDoesNotTakeACrewMarkedSeat(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir, Bead: "a-1", Crew: true},
		"closed", DefaultCrewReapAfter+time.Hour)
	workingClaude(t, fake)
	armScreen(t, fake, endedTurnFooter, emptyBox)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a crew-marked seat reading `working` stays the operator's, past any grace:\n%s", dispatcherOut(d))
	}
}
