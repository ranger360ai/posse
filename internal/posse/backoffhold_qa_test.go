//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for the backoff hold — ranger-base-5tkz9.
//
// Claim: a pass that launched nothing because it was BLOCKED does not grow
// the idle backoff, and a block that names its own end schedules the next
// pass just after that end.
//
// The defect was measured twice on 2026-11-13: the launch cap exhausted 28
// seconds before the epoch turned, "0 dispatched · next pass in 40m0s", and
// a shop with three ready P1 beads idle until the operator restarted the
// unit. Three things have to hold for that not to recur, and they are three
// arms because each one of them was green on its own while the shop idled:
//
//  1. the arithmetic — a block cuts the sleep to its lift and never climbs
//     the ladder (TestNextPassIsHeldToTheBlocksLift);
//  2. the reading — a cap-exhausted pass actually records the cap and the
//     turn, and a declined pass records the pause with no lift
//     (TestQACapExhaustedPassRecordsTheEpochTurn,
//     TestQAPausedPassRecordsABlockWithNoKnownLift);
//  3. the schedule — the loop puts the two together and prints a wait at the
//     turn rather than at the end of its ladder
//     (TestQAWatchSchedulesTheNextPassAtTheLaunchCapTurn).
//
// MUTATION-CHECKED. Dropping passBlock.hold's lift arm reds arms 1 and 3;
// dropping its ladder arm reds arm 1; dropping epoch.go's noteBlocked call
// reds arms 2 and 3; dropping the pause's noteBlocked reds arm 2.

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Arm 1: the arithmetic, over the inputs the loop hands it.
func TestNextPassIsHeldToTheBlocksLift(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 13, 15, 59, 32, 0, time.UTC)
	// The incident's own numbers: the ladder at its 40m ceiling, the cap
	// spent, the epoch turning 28 seconds later.
	turn := now.Add(28 * time.Second)
	cap40 := passBlock{why: "the launch cap", until: turn}

	for _, tc := range []struct {
		name                string
		blk                 passBlock
		grown, prev         time.Duration
		wantLadder, wantNap time.Duration
	}{{
		name: "no block: the ladder is the sleep, as it always was",
		blk:  passBlock{}, grown: 40 * time.Minute, prev: 20 * time.Minute,
		wantLadder: 40 * time.Minute, wantNap: 40 * time.Minute,
	}, {
		name: "the cap, with the turn nearer than the ladder: sleep to the turn",
		blk:  cap40, grown: 40 * time.Minute, prev: 40 * time.Minute,
		wantLadder: 40 * time.Minute, wantNap: 28*time.Second + BlockLiftMargin,
	}, {
		name: "the cap, blocking a pass that would have doubled: no climb",
		blk:  cap40, grown: 40 * time.Minute, prev: 20 * time.Minute,
		wantLadder: 20 * time.Minute, wantNap: 28*time.Second + BlockLiftMargin,
	}, {
		name:  "the cap, with the turn further off than the ladder: the ladder stands",
		blk:   passBlock{why: "the launch cap", until: now.Add(time.Hour)},
		grown: 40 * time.Minute, prev: 40 * time.Minute,
		wantLadder: 40 * time.Minute, wantNap: 40 * time.Minute,
	}, {
		// The turn went by while the pass was running. One margin, and
		// never MORE than the ladder: a lift in the past must not be the
		// one input that lengthens a wait.
		name:  "the cap, with the turn already past: one margin",
		blk:   passBlock{why: "the launch cap", until: now.Add(-5 * time.Minute)},
		grown: 40 * time.Minute, prev: 40 * time.Minute,
		wantLadder: 40 * time.Minute, wantNap: BlockLiftMargin,
	}, {
		name:  "a lift in the past against a ladder under one margin: the ladder",
		blk:   passBlock{why: "the launch cap", until: now.Add(-5 * time.Minute)},
		grown: 100 * time.Millisecond, prev: 100 * time.Millisecond,
		wantLadder: 100 * time.Millisecond, wantNap: 100 * time.Millisecond,
	}, {
		// A pause is lifted by a human, so there is no instant to aim at —
		// holding the ladder where it is, is the whole of it.
		name: "the pause: held, not grown, and no lift to aim at",
		blk:  passBlock{why: "the pause"}, grown: 40 * time.Minute, prev: 5 * time.Minute,
		wantLadder: 5 * time.Minute, wantNap: 5 * time.Minute,
	}, {
		name: "the first pass of a loop, with no previous wait to hold to",
		blk:  passBlock{why: "the pause"}, grown: 5 * time.Minute, prev: 0,
		wantLadder: 5 * time.Minute, wantNap: 5 * time.Minute,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ladder, nap := tc.blk.hold(tc.grown, tc.prev, now)
			if ladder != tc.wantLadder {
				t.Errorf("ladder = %s, want %s", ladder, tc.wantLadder)
			}
			if nap != tc.wantNap {
				t.Errorf("sleep = %s, want %s", nap, tc.wantNap)
			}
			if nap > ladder {
				t.Errorf("a hold must never lengthen a wait: sleep %s > ladder %s", nap, ladder)
			}
		})
	}
}

// The clause is the only thing in the log that ties the cap's exhaustion
// line to the number an operator reads, and it must stay silent when the
// hold changed nothing — a pause landing on a loop already at its ceiling
// has nothing about the schedule to explain.
func TestTheHoldSaysSoOnlyWhenItMovedTheSchedule(t *testing.T) {
	t.Parallel()
	turn := time.Date(2026, 11, 13, 16, 0, 0, 0, time.UTC)
	cap := passBlock{why: "the launch cap", until: turn}
	if got := cap.clause(40*time.Minute, 40*time.Minute, 29*time.Second); !strings.Contains(got, "held to") ||
		!strings.Contains(got, "the launch cap") || !strings.Contains(got, "40m0s") {
		t.Errorf("a sleep cut to the turn must name the turn, the block and the backoff it replaced: %q", got)
	}
	pause := passBlock{why: "the pause"}
	if got := pause.clause(40*time.Minute, 20*time.Minute, 20*time.Minute); !strings.Contains(got, "the pause") ||
		!strings.Contains(got, "held at 20m0s") {
		t.Errorf("a ladder held from climbing must say so: %q", got)
	}
	if got := pause.clause(40*time.Minute, 40*time.Minute, 40*time.Minute); got != "" {
		t.Errorf("a hold that moved nothing must say nothing: %q", got)
	}
	if got := (passBlock{}).clause(time.Minute, time.Minute, time.Minute); got != "" {
		t.Errorf("an unblocked pass has no clause: %q", got)
	}
}

// Arm 2: a pass whose cap is spent records the cap and the epoch's turn, in
// the frame the schedule is kept in. Without this the arithmetic above is
// correct over inputs nothing produces.
func TestQACapExhaustedPassRecordsTheEpochTurn(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.StartupWait = 100 * time.Millisecond
	writePersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`, "")
	idleClaude(t, fake)

	// Pass 1 spends the whole cap; pass 2 is the one with nothing left.
	if _, err := d.Run("", "", 1); err != nil {
		t.Fatal(err)
	}
	if d.blocked.on() {
		t.Errorf("a pass that launched is not blocked: %+v", d.blocked)
	}
	if _, err := d.Run("", "", 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dispatcherOut(d), "launch cap: 1 of 1") {
		t.Fatalf("pass 2 should have found the cap spent:\n%s", dispatcherOut(d))
	}
	if d.blocked.why != "the launch cap" {
		t.Errorf("a cap-exhausted pass must record the cap, got %q", d.blocked.why)
	}
	if want := d.epochEnd(); !d.blocked.until.Equal(want) {
		t.Errorf("the lift must be the epoch's turn: %s, want %s", d.blocked.until, want)
	}
}

// Arm 2, the other half: a declined pass records the pause, and records
// that its end is not knowable from here. A zero lift is the whole
// difference between the two blocks.
func TestQAPausedPassRecordsABlockWithNoKnownLift(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	idleClaude(t, fake)
	if _, err := WritePause(b.App, "the operator", "the box is being rebuilt", time.Now(), d.errw()); err != nil {
		t.Fatal(err)
	}

	n, err := d.Run("", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a paused pass launches nothing, got %d", n)
	}
	if d.blocked.why != "the pause" {
		t.Errorf("a declined pass must record the pause, got %q", d.blocked.why)
	}
	if !d.blocked.until.IsZero() {
		t.Errorf("`posse resume` is a human, not a clock — the lift must be unknown, got %s", d.blocked.until)
	}
}

// Arm 4: a block does not outlive the pass that found it — added by
// ranger-base-ujp0c, verifying this bead.
//
// passBlock is pass-denominated, and the whole of that is Run clearing it
// at its head beside planTrip (dispatch.go). MEASURED 2026-11-13, D1 host,
// at 2bb926f3: with that one line dropped, every pin above stays GREEN.
// The reading does not decay on its own — hold reads it on every later
// pass, and a STALE cap block's lift is in the past, so hold clamps the
// sleep to one margin. The loop then takes a pass a second for the rest of
// its life, with the ladder pinned at whatever `prev` was and the cadence
// line naming an epoch turn that went by hours ago. That is this bead's own
// defect inverted, and no arm above could see it, because each builds its
// block inside the pass it measures.
//
// Both arms of Run that reach the backoff are here: the one that launched,
// and the one that found nothing to launch and returned before epochRoom.
func TestQAABlockDoesNotOutliveItsPass(t *testing.T) {
	t.Parallel()
	// The premise, asserted rather than assumed, so this pin says what a
	// stale block COSTS and not merely that the field was zeroed.
	stale := passBlock{why: "the launch cap", until: time.Now().Add(-2 * time.Hour)}
	if _, nap := stale.hold(40*time.Minute, 40*time.Minute, time.Now()); nap != BlockLiftMargin {
		t.Fatalf("a block whose lift went by holds the loop to %s, not one margin — this pin's premise has moved", nap)
	}

	for _, tc := range []struct{ name, ready string }{
		{"a pass that launched", `[{"id":"a-1","title":"t","labels":["go"]}]`},
		{"a pass with no ready work", `[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			d.StartupWait = 100 * time.Millisecond
			writePersona(t, b.App, "ranger", "[go]")
			qaRepo(t, b.App, tc.ready, "")
			idleClaude(t, fake)
			// The pass before this one was blocked by a cap that has long
			// since turned.
			d.blocked = stale
			if _, err := d.Run("", "", 1); err != nil {
				t.Fatal(err)
			}
			if d.blocked.on() {
				t.Errorf("a block left by an earlier pass survived this one (%+v), so every later pass is held to one margin "+
					"and the loop spins at a pass a second (ranger-base-5tkz9, ranger-base-ujp0c):\n%s",
					d.blocked, dispatcherOut(d))
			}
		})
	}
}

// Arm 3: the loop puts the reading and the arithmetic together.
//
// The fixture is the incident in miniature — the cap spent a few seconds
// before the turn, with a base interval far longer than that — and what it
// asserts is the SCHEDULE the loop prints, not when a later pass actually
// arrives. A wall-clock arrival is a reading about the box: under -race on a
// loaded box a single pass of this package has been measured tens of seconds
// behind its tick (watchBackstop's own doc), and the number this bead is
// about is the one the loop chose.
func TestQAWatchSchedulesTheNextPassAtTheLaunchCapTurn(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.StartupWait = 100 * time.Millisecond
	writePersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	idleClaude(t, fake)

	// An epoch is midnight-aligned (epoch.go), so one exactly `turnIn`
	// longer than the day so far is one that turns `turnIn` from now. This
	// is how a fixture gets "seconds before the turn" off the real clock,
	// without an injected one: the schedule below is kept in the same frame
	// as the timer that runs it, and injecting a clock into one half of that
	// would be a fixture about nothing.
	const turnIn = 8 * time.Second
	now := time.Now()
	appendConfig(t, b.App, fmt.Sprintf("dispatch_epoch: %dms\n", (now.Sub(startOfDay(now))+turnIn).Milliseconds()))

	// The cap, already spent. rollEpoch resets the count only when the
	// computed opening differs from the stored one, so the stored one is
	// that opening.
	d.epochStart = startOfDay(now)
	d.epochAttempts = 1

	// Far longer than the turn is away: this is the 40m the incident waited.
	const base = 10 * time.Minute
	var out syncBuf
	d.Out = &out
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.Watch(ctx, "", "", 1, base, base)
	}()
	waitForOut(t, &out, "next pass in")
	cancel()
	select {
	case <-done:
	case <-time.After(watchBackstop(t)):
		t.Fatal("the loop never returned")
	}

	log := out.String()
	if !strings.Contains(log, "launch cap: 1 of 1") {
		t.Fatalf("the fixture's first pass must find the cap spent — if the epoch turned before the pass ran, "+
			"raise turnIn:\n%s", log)
	}
	// Read with the regex `scripts/pass-census.py` reads it with, character
	// for character (its NEXT). That script is the only instrument that
	// measures what a pass on this box costs (ADR 0063 D4), and a clause
	// inserted between the number and the ctrl-c hint would take every
	// cap-blocked pass out of it silently — which is what the first draft of
	// this line did.
	m := regexp.MustCompile(`next pass in (\S+) \(ctrl-c`).FindStringSubmatch(log)
	if m == nil {
		t.Fatalf("the cadence line must still parse as scripts/pass-census.py reads it:\n%s", log)
	}
	got, err := time.ParseDuration(m[1])
	if err != nil {
		t.Fatalf("cadence line %q: %v", m[0], err)
	}
	// At or just after the turn, and never the ladder: the turn is at most
	// turnIn away, and a pass slow enough to run past it is held to one
	// margin instead.
	if got <= 0 || got > turnIn+BlockLiftMargin {
		t.Errorf("a pass blocked by a cap that turns in %s scheduled the next one in %s, with a base of %s — "+
			"the backoff grew past the turn (ranger-base-5tkz9):\n%s", turnIn, got, base, log)
	}
	if !strings.Contains(log, "held to") || !strings.Contains(log, "the launch cap") {
		t.Errorf("the cadence line must say the wait was held to the cap's turn:\n%s", log)
	}
}
