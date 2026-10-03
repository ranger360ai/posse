//go:build posse_arm3

package posse

// ranger-base-sqxo1: the pins for the suspend witness (suspend.go).
//
// The bead was filed as a hang — "watch loop hung ~6h after pass 6, the
// prompt-gathering wait never returned" — and it was a laptop that went into
// Low Power Sleep at 1% battery for 19807 seconds. suspend.go's head carries
// the three measurements that settle it. So, like watchhang_qa_test.go's pins
// before them, not one arm here asserts that a hang happened: they assert
// that posse can now tell a suspended box from a stalled loop and says which,
// which was missing on 2026-09-03 and still missing on 2026-10-01.
//
// ranger-base-m154u then made the reading answerable from outside this
// goroutine (ADR 0064 D2, D3, D6): a ledger of the gaps it named, a window
// sum over it, the pulse taking its own reading first, and a backward step
// reported instead of swallowed. The last of those REVERSES a pin in this
// file: the direction arm below used to assert silence, and is flipped rather
// than deleted.
//
// Eight arms:
//
//	the reading     a wall clock that moved five and a half hours while the
//	                monotonic clock moved three seconds is named as a
//	                suspend, with both readings in the line; two clocks that
//	                moved together are not.
//	the floor       a wall step under SuspendFloor is a clock being SET, and
//	                is silent — in EITHER direction.
//	the direction   a wall clock that went BACKWARD past the floor is
//	                reported and is not written to the ledger: it is not a
//	                negative suspend and there is nothing to subtract.
//	the ledger      the window sum is each span's overlap with [since, now],
//	                for a window that starts before, inside and after a
//	                recorded suspend and for two suspends in one window; and
//	                the ledger is bounded, dropping the oldest.
//	the pulse       pulseOnce reads the pair ITSELF, before it reads
//	                anything else — the ordering ADR 0064 D3 rests on, which
//	                nothing in this slice can observe behaviourally.
//	the seam        the witness reads d.Clocks and never d.now(), or every
//	                fixture that ages the dispatcher's clock prints a suspend.
//	the subtraction the wall gaps in suspendTick AND suspendedSince are not
//	                Time.Subs — the one claim in this file no fixture can
//	                hold, pinned at the source.
//	the arming      the witness runs with neither watchdog budget set, and
//	                Watch seeds it before the first tick.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"sync"
	"testing"
	"time"
)

// pairClock is the witness's seam driven by hand: two clocks a test can move
// independently, which is the one thing a real pair cannot do and the whole
// reason the seam returns both readings at once.
type pairClock struct {
	mu   sync.Mutex
	wall time.Time
	mono time.Duration
}

func (c *pairClock) read() (time.Time, time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.wall, c.mono
}

// advance moves the two clocks by the amounts named. A suspend is wall moving
// and mono standing still; an ordinary tick is both moving together.
func (c *pairClock) advance(wall, mono time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.wall, c.mono = c.wall.Add(wall), c.mono+mono
}

// suspendRig is a dispatcher whose suspend witness reads a pairClock, seeded.
func suspendRig(t *testing.T) (*Dispatcher, *pairClock) {
	t.Helper()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	// Thirty seconds before the incident's sleep instant, so the control tick
	// every arm takes first lands the wall clock exactly on pmset's 18:38:49.
	c := &pairClock{wall: time.Date(2026, 10, 1, 18, 38, 19, 0, time.Local)}
	d.Clocks = c.read
	d.noteClock()
	return d, c
}

// The incident's own numbers: pmset recorded 19807s of Low Power Sleep from
// 18:38:49 and a wake at 00:08:56, and the loop was frozen for all of it, so
// one tick of the witness spans the whole thing with a few seconds of
// monotonic on either side of it.
func TestQASuspendWitnessNamesASuspendedBox(t *testing.T) {
	t.Parallel()
	d, c := suspendRig(t)

	// An ordinary tick first: both clocks move together, and nothing is said.
	// The control arm comes first on purpose — a witness that fired on this
	// would be a line printed every thirty seconds for the life of the loop,
	// which is the failure mode every clock in this package is written against.
	c.advance(30*time.Second, 30*time.Second)
	d.suspendTick()
	if out := dispatcherOut(d); out != "" {
		t.Fatalf("a tick on an awake box must be silent; got:\n%s", out)
	}

	// The suspend: pmset's own 19807s of wall — 18:38:49 to 00:08:56 — with
	// seven seconds of monotonic for the loop's own waking moments on either
	// side of it.
	c.advance(19807*time.Second, 7*time.Second)
	d.suspendTick()
	out := dispatcherOut(d)
	if !strings.Contains(out, "SUSPENDED") {
		t.Fatalf("a 5h30m wall gap against 7s of monotonic was not named a suspend:\n%s", out)
	}
	// How long. BlindFor rounds to the minute above an hour, so this is the
	// 5h30m07s pmset reported, in the shop's own units.
	if !strings.Contains(out, "5h30m") {
		t.Errorf("the line does not say how long the box was gone:\n%s", out)
	}
	// When it came back — the fact that makes an untimestamped gap in the
	// watch log reconstructable at all.
	if !strings.Contains(out, "00:08:56") {
		t.Errorf("the line does not say when the box came back:\n%s", out)
	}
	// Both readings, because the next thing anyone does with this line is
	// check it against `pmset -g log`.
	if !strings.Contains(out, "7s of monotonic") {
		t.Errorf("the line does not carry the monotonic reading it is a difference of:\n%s", out)
	}
	// And the three wall-clock readings it exists to disarm. These are the
	// ones the operator was handed on 2026-10-01 with no cause attached.
	for _, want := range []string{"loop-mute", "guard-blind"} {
		if !strings.Contains(out, want) {
			t.Errorf("the line must name %q as the suspend rather than a fault — that is what it is FOR:\n%s", want, out)
		}
	}

	// And it does not repeat: a suspend is an EVENT, so the tick after it
	// measures an awake box again.
	before := len(dispatcherOut(d))
	c.advance(30*time.Second, 30*time.Second)
	d.suspendTick()
	if len(dispatcherOut(d)) != before {
		t.Errorf("the witness spoke again about a box that is awake:\n%s", dispatcherOut(d)[before:])
	}
}

// The floor and the direction. The two readings are taken a nanosecond apart,
// so the only other thing that separates them is the wall clock being SET —
// ntpd stepping a second, an operator fixing a timezone — and a correction
// under the floor is not a suspend whichever way it went.
//
// Past the floor the two directions part. A forward gap is a hole in wall
// time that no timestamp falls into, so it is recordable and subtractable; a
// backward step makes two frames live at once, so it is REPORTED and nothing
// more (ADR 0064 D6). The backward arm here used to assert silence — it is
// flipped rather than deleted, because "this witness says nothing about a
// backward step" was a real decision and the record of its reversal belongs
// where it was made.
//
// MUTATION: drop the `slept < SuspendFloor` return → the forward floor arm
// reds. Drop the `slept <= -SuspendFloor` arm → the backward arm reds on the
// line, and the backward floor arm catches the mutation that reports every
// negative gap. Write the backward step to the ledger too → the
// not-recorded half reds.
func TestQASuspendWitnessIgnoresAClockStepAndABackwardOne(t *testing.T) {
	t.Parallel()

	t.Run("a step under the floor is not a suspend", func(t *testing.T) {
		t.Parallel()
		d, c := suspendRig(t)
		c.advance(SuspendFloor-time.Second, 0)
		d.suspendTick()
		if out := dispatcherOut(d); out != "" {
			t.Errorf("a %s clock step was called a suspend:\n%s", SuspendFloor-time.Second, out)
		}
	})

	t.Run("a step under the floor BACKWARD is also silent", func(t *testing.T) {
		t.Parallel()
		d, c := suspendRig(t)
		// An ntp slew, which is what a sub-floor backward step is in the
		// record. The line below is for a clock being SET; this is not one,
		// and reporting it would be a line every time the box corrects.
		c.advance(-(SuspendFloor - time.Second), 0)
		d.suspendTick()
		if out := dispatcherOut(d); out != "" {
			t.Errorf("a %s backward slew was reported:\n%s", SuspendFloor-time.Second, out)
		}
	})

	t.Run("a wall clock set back past the floor is reported, not recorded", func(t *testing.T) {
		t.Parallel()
		d, c := suspendRig(t)
		// The wall clock set back an hour while no real time passed. Every
		// wall-clock reading in the shop now reads FRESHER than it is, so
		// `loop-mute` and `guard-blind` go quiet over a real outage for that
		// long — and nothing can correct it, because a timestamp inside the
		// overlap cannot be told pre-step from post-step (ADR 0064 D6).
		c.advance(-time.Hour, 0)
		d.suspendTick()
		out := dispatcherOut(d)
		if !strings.Contains(out, "SET BACK") {
			t.Fatalf("a wall clock set back an hour was swallowed; zero backward steps are in the record and this "+
				"line is what makes the first one countable (ADR 0064 D6):\n%s", out)
		}
		// How far, and when — the two facts that make the step reconstructable.
		if !strings.Contains(out, "1h00m") {
			t.Errorf("the line does not say how far the clock went back:\n%s", out)
		}
		if !strings.Contains(out, c.wall.Format("15:04:05")) {
			t.Errorf("the line does not say when the clock was set:\n%s", out)
		}
		// What it costs the reader: the inverse of SuspendLine's three
		// readings. A suspend makes them fire wrongly; a backward step makes
		// them stay quiet wrongly, and that is the half an operator cannot
		// guess.
		for _, want := range []string{"FRESHER", "loop-mute", "guard-blind", "NOTHING corrects it"} {
			if !strings.Contains(out, want) {
				t.Errorf("the line must say %q — a quiet row over a real outage is what this costs:\n%s", want, out)
			}
		}
		if strings.Contains(out, "SUSPENDED") {
			t.Errorf("a backward step was reported as a suspend:\n%s", out)
		}

		// And NOT recorded. There is nothing to subtract from a window: the
		// step did not remove wall time, it duplicated some.
		d.mu.Lock()
		n := len(d.suspends)
		d.mu.Unlock()
		if n != 0 {
			t.Errorf("a backward step entered the ledger (%d spans); suspendedSince would then subtract a span "+
				"that no timestamp is inside", n)
		}
		if got := d.suspendedSince(c.wall.Add(-6 * time.Hour)); got != 0 {
			t.Errorf("suspendedSince subtracted %s for a backward step", got)
		}
	})
}

// The ledger and the window sum (ADR 0064 D2). This is the arithmetic the
// whole decision rests on: a wall-clock condition compares an awake
// threshold to a wall age, and the fix is to subtract the part of the window
// the box was provably gone. Get the overlap wrong and the shop either keeps
// the false URGENT it had or buys a new way to go quiet over a real outage.
//
// So the arms are the three positions a window can take against one recorded
// suspend — starting before it, inside it, after it — plus two suspends in
// one window, which is the case a sum that returned "the longest span" or
// "the most recent span" would pass every other arm with.
//
// MUTATION: clip only the LOW end (drop the `nowNS < hi` arm) → every arm
// here stays green, MEASURED, because a span's back is always already past;
// that clip is for the caller's invariant and not for this sum, and
// suspend.go says so. Drop the `sinceNS > lo` clip → the inside-and-after
// arms red. Return after the first overlapping span → the two-suspend arm
// reds. Both of those also red TestQASuspendLedgerIsBounded, which sums 256
// spans and is the broader net of the two.
func TestQASuspendLedgerSumsTheWindowOverlap(t *testing.T) {
	t.Parallel()
	d, c := suspendRig(t)
	base := c.wall

	// One hour asleep, from base: slept is wallGap-monoGap, so a tick with
	// no monotonic at all is a span of exactly the wall it moved.
	c.advance(time.Hour, 0)
	d.suspendTick()
	// An awake hour, which must contribute nothing.
	c.advance(time.Hour, time.Hour)
	d.suspendTick()
	// Half an hour asleep.
	c.advance(30*time.Minute, 0)
	d.suspendTick()
	// And an awake hour after it, so `now` is past both spans and every
	// window below has a real right-hand end.
	c.advance(time.Hour, time.Hour)
	d.suspendTick()

	// [base, base+1h] and [base+2h, base+2h30m], with now at base+3h30m.
	d.mu.Lock()
	spans := len(d.suspends)
	d.mu.Unlock()
	if spans != 2 {
		t.Fatalf("premise: two suspends and two awake ticks must leave exactly two spans in the ledger, got %d", spans)
	}

	for _, tc := range []struct {
		what  string
		since time.Time
		want  time.Duration
	}{
		{"a window starting before both suspends holds both", base.Add(-time.Hour), 90 * time.Minute},
		{"a window starting INSIDE the first holds the rest of it, and all of the second", base.Add(30 * time.Minute), time.Hour},
		{"a window starting after the first, in the awake gap, holds only the second", base.Add(90 * time.Minute), 30 * time.Minute},
		{"a window starting inside the second holds the rest of it", base.Add(2*time.Hour + 15*time.Minute), 15 * time.Minute},
		{"a window starting after both holds nothing", base.Add(3 * time.Hour), 0},
		{"a window exactly on the first span's end holds only the second", base.Add(time.Hour), 30 * time.Minute},
		{"a zero since is no window, not all of time", time.Time{}, 0},
	} {
		if got := d.suspendedSince(tc.since); got != tc.want {
			t.Errorf("%s: suspendedSince(%s) = %s, want %s", tc.what,
				tc.since.Format("15:04:05"), got, tc.want)
		}
	}

	// The caller's invariant: a process subtracting this from an age it
	// measured over the SAME window must not be able to go negative, which
	// is what lets govern.go subtract without a clamp of its own.
	for _, since := range []time.Time{base.Add(-time.Hour), base, base.Add(45 * time.Minute), base.Add(3 * time.Hour)} {
		window := time.Duration(c.wall.UnixNano() - since.UnixNano())
		if got := d.suspendedSince(since); got > window {
			t.Errorf("suspendedSince(%s) = %s, longer than the %s window it is inside",
				since.Format("15:04:05"), got, window)
		}
	}
}

// The bound. A laptop sleeps often enough that an unbounded ledger is a slow
// leak in a process meant to run for days: MEASURED 2026-10-03, 751 of this
// box's 1127 sleeps in six days of `pmset -g log` were at or over
// SuspendFloor. Dropping the OLDEST is what makes the bound harmless — the
// longest window anything asks about is under two hours, so the pairs that
// fall off are the ones no window can reach.
//
// MUTATION: drop the re-slice → the length arm reds. Drop the NEWEST instead
// of the oldest (`d.suspends[:suspendLedgerCap]`) → the sum arm reds, because
// the window then holds the cap's worth of ancient spans and none of the
// recent ones.
func TestQASuspendLedgerIsBounded(t *testing.T) {
	t.Parallel()
	d, c := suspendRig(t)
	base := c.wall

	// One minute asleep, one minute awake, past the cap.
	const over = 5
	for i := 0; i < suspendLedgerCap+over; i++ {
		c.advance(time.Minute, 0)
		d.suspendTick()
		c.advance(time.Minute, time.Minute)
		d.suspendTick()
	}

	d.mu.Lock()
	n := len(d.suspends)
	d.mu.Unlock()
	if n != suspendLedgerCap {
		t.Errorf("the ledger holds %d spans after %d suspends; suspendLedgerCap is %d and a loop that runs for days "+
			"must not grow one entry per sleep forever", n, suspendLedgerCap+over, suspendLedgerCap)
	}
	// Every surviving span is a whole minute and the window covers all of
	// them, so the sum names exactly how many survived — and the `over`
	// spans that fell off are the oldest, not the newest.
	if got, want := d.suspendedSince(base.Add(-time.Hour)), time.Duration(suspendLedgerCap)*time.Minute; got != want {
		t.Errorf("a window over the whole ledger sums to %s, want %s (%d spans of a minute)", got, want, suspendLedgerCap)
	}
	if got := d.suspendedSince(c.wall.Add(-2*over*time.Minute - time.Second)); got != over*time.Minute {
		t.Errorf("a window over the last %d suspends sums to %s, want %s — the ledger dropped the NEWEST spans, "+
			"which are the only ones a window can still reach", over, got, over*time.Minute)
	}
}

// The pulse reads the pair itself, before it reads anything else (ADR 0064
// D3). After a wake the witness's 30s ticker and the pulse's 2m one resume
// with whatever was left on each, so the witness has not necessarily seen the
// gap when the pulse computes its set — and a pulse that computes
// `guard-blind:5h; loop-mute` from a ledger nobody has told yet is the
// 2026-10-01 false URGENT exactly as it happened.
//
// The arming fixture's pattern: a pairClock moved by hand with the witness's
// ticker never started at all, so the ONLY thing that can have recorded the
// gap is the pulse tick.
//
// MUTATION: delete `d.suspendTick()` from pulseOnce → the first arm reds.
func TestQASuspendPulseReadsThePairFirst(t *testing.T) {
	t.Parallel()

	t.Run("a pulse tick records the gap with no witness running", func(t *testing.T) {
		t.Parallel()
		d, c := suspendRig(t)
		base := c.wall
		// The incident's own sleep, and no watchdogLoop anywhere: nothing
		// else in this process is reading the clocks.
		c.advance(19807*time.Second, time.Second)

		d.pulseOnce(PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute})

		if out := dispatcherOut(d); !strings.Contains(out, "SUSPENDED") {
			t.Fatalf("the pulse computed a condition set without reading the pair; after a wake it is the pulse "+
				"that lands first about one time in eight, and this is that time:\n%s", out)
		}
		// Not just the line — the LEDGER, which is what the set is built
		// from. A line the pulse printed and did not record would read as a
		// fix and subtract nothing.
		if got := d.suspendedSince(base); got != 19806*time.Second {
			t.Errorf("the pulse's own reading left %s in the ledger, want %s", got, 19806*time.Second)
		}
	})

	// And the ORDER, pinned at the source because nothing in this slice can
	// observe it: the field the set is built from lands in govern.go, so
	// until it does, a suspendTick moved below the ShopCheck call is green
	// everywhere and wrong on exactly the wake this exists for.
	//
	// MUTATION: move `d.suspendTick()` below `ShopCheck(…)` → this reds.
	t.Run("the call is before the shop check, not after it", func(t *testing.T) {
		t.Parallel()
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "pulse.go", nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		var body *ast.BlockStmt
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "pulseOnce" {
				body = fn.Body
			}
		}
		if body == nil {
			t.Fatal("pulseOnce is gone from pulse.go; this sweep is not reading the file it thinks it is")
		}
		// stmtAt, not `at`: this package has a package-level `at` and
		// cmd/testparallel reads the NAME, so a local that shadows it reads
		// as shared state and refuses this test's t.Parallel.
		stmtAt := func(name string) int {
			for i, stmt := range body.List {
				found := false
				ast.Inspect(stmt, func(n ast.Node) bool {
					if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
						found = true
					} else if id, ok := n.(*ast.Ident); ok && id.Name == name {
						found = true
					}
					return !found
				})
				if found {
					return i
				}
			}
			return -1
		}
		tick, shop := stmtAt("suspendTick"), stmtAt("ShopCheck")
		if tick < 0 {
			t.Fatal("pulseOnce does not call suspendTick: after a wake the witness's ticker is not guaranteed to " +
				"have fired, and every wall-clock row on that tick is then a fault the shop did not have (ADR 0064 D3)")
		}
		if shop < 0 {
			t.Fatal("pulseOnce does not call ShopCheck; this sweep cannot say what the pair is read before")
		}
		if tick > shop {
			t.Errorf("pulseOnce reads the suspend pair at statement %d, AFTER the shop check at %d: the set is "+
				"built from the ledger, so a reading taken afterwards arrives one tick too late", tick, shop)
		}
	})
}

// The seam, and the escape it closes. d.Now is aged by the fixtures in this
// package by hours, and watchhang_qa_test.go runs one at 10000x real time to
// reach a watchdog budget — which is wall moving while monotonic does not,
// the exact signature of a suspend. A witness reading d.now() would print a
// suspend line into every one of them, and the lines would be about nothing.
//
// MUTATION: make clocks() fall back to `d.now(), monoSince()` → this reds.
func TestQASuspendWitnessReadsItsOwnSeamAndNotTheDispatcherClock(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)

	// The accelerated-clock fixture's own shape (watchhang_qa_test.go's
	// stalled-pass arm): the dispatcher's view of time runs 10000x, and the
	// witness is left on the real pair.
	epoch := time.Now()
	t0 := time.Date(2026, 10, 1, 18, 38, 49, 0, time.Local)
	d.Now = func() time.Time { return t0.Add(time.Since(epoch) * 10000) }

	d.noteClock()
	// Ten milliseconds of real time is 100 seconds of this dispatcher's,
	// three times the floor.
	time.Sleep(10 * time.Millisecond)
	d.suspendTick()
	if out := dispatcherOut(d); out != "" {
		t.Errorf("aging the DISPATCHER's clock is a test reaching a budget, never a claim that the box slept; the witness spoke:\n%s", out)
	}
	if got := d.now().Sub(t0); got < 3*SuspendFloor {
		t.Fatalf("premise: the fixture's clock must have moved far past the floor for this to measure anything (moved %s)", got)
	}
}

// The subtraction, pinned at the SOURCE because no fixture can hold it.
//
// time.Time carries a wall reading and a monotonic one, and Sub prefers the
// monotonic whenever both operands have it — so `wall.Sub(prev)` is identically
// the monotonic gap, the difference it is subtracted from is always zero, and
// the witness goes permanently silent. It is one keystroke and it is invisible
// to every arm above, because there is no way to build a Time whose two
// readings disagree: Add moves both, and only a real suspend separates them.
//
// So this arm reads the file, the way TestQAClockFilesUseOnlyTheQuietPair
// does for the writers. The wall gap must come from UnixNano, which has no
// monotonic variant at all.
//
// It covers suspendedSince for the same reason and a sharper case
// (ranger-base-m154u). That function mixes two kinds of instant: the
// ledger's, which came from time.Now() through the pair seam and therefore
// CARRY a monotonic reading, and the window's, which come from outside this
// process — a file mtime — and do not. So `now.Sub(mtime)` is a true wall
// difference and looks like a licence, while `now.Sub(span.back)` and
// `hi.Sub(lo)` between two ledger instants are monotonic differences that
// exclude every suspend that happened AFTER the span being measured. The
// answer is then short by the next sleep, on exactly the box that sleeps.
//
// MEASURED 2026-10-03: the idiomatic Time-based rewrite of this function —
// `sp.back.Add(-sp.slept)`, `since.After(lo)`, `hi.Sub(lo)`, the same logic
// throughout — passes every behavioural arm in this file. It has to: a
// pairClock's wall is built from time.Date and carries no monotonic reading
// at all, so in the fixture the two differences are equal by construction,
// and only a real suspend separates them. That is the whole reason this arm
// reads the file instead of running the code.
//
// MUTATION: rewrite the wallGap line as `wall.Sub(prevWall)`, or suspendedSince
// in Time arithmetic as above → this reds and every other arm in this file
// stays green, which is why it exists.
func TestQASuspendWallGapIsNotAMonotonicSubtraction(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "suspend.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string]*ast.BlockStmt{}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			bodies[fn.Name.Name] = fn.Body
		}
	}
	// Both wall-difference functions in the file. Each must take its
	// difference from at least two UnixNano readings, which is the lower
	// bound for a difference of two instants in either.
	for _, fn := range []string{"suspendTick", "suspendedSince"} {
		body := bodies[fn]
		if body == nil {
			t.Fatalf("%s is gone from suspend.go; this sweep is not reading the file it thinks it is", fn)
		}
		unix := 0
		ast.Inspect(body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "Sub":
				t.Errorf("%s calls .Sub: Time.Sub prefers the MONOTONIC reading when both operands carry one, so a "+
					"wall difference becomes a monotonic one — the witness goes silent forever, or a window sum "+
					"returns zero over a suspend it recorded (%s)", fn, fset.Position(sel.Pos()))
			case "UnixNano":
				unix++
			}
			return true
		})
		if unix < 2 {
			t.Errorf("%s must take its wall difference from UnixNano readings; found %d", fn, unix)
		}
	}
}

// The arming. The two readings watchdogLoop already took are each armed by a
// budget, and on both the nights this witness exists for every budgeted
// reading was correctly silent — so this one must not be behind them. And
// Watch must SEED it, or a box suspended inside the first SuspendTick is the
// one suspend nothing names, which is what a laptop started on a dying
// battery does.
//
// MUTATION: restore watchdogLoop's `if every <= 0 || (budget <= 0 &&
// passBudget <= 0) { return }` → the first arm reds. Drop `d.noteClock()`
// from watch.go → the second reds.
func TestQASuspendWitnessRunsWithNoBudgetAndIsSeededByWatch(t *testing.T) {
	t.Parallel()

	t.Run("no budget, still a witness", func(t *testing.T) {
		t.Parallel()
		d, c := suspendRig(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			d.watchdogLoop(ctx, time.Millisecond, 0, 0)
		}()
		c.advance(6*time.Hour, time.Second)
		deadline := time.After(5 * time.Second)
		for !strings.Contains(dispatcherOut(d), "SUSPENDED") {
			select {
			case <-deadline:
				t.Fatalf("a watchdog loop with neither budget set never read the clocks:\n%s", dispatcherOut(d))
			default:
				time.Sleep(time.Millisecond)
			}
		}
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("watchdogLoop did not return after cancel — the witness's ticker outlives the loop")
		}
	})

	t.Run("Watch seeds the witness", func(t *testing.T) {
		t.Parallel()
		b, _ := newTestBackend(t)
		d := newTestDispatcher(t, b)
		// An empty queue of its own, and a context already dead: the loop ends
		// before its first pass, which is enough — the seeding is in Watch's
		// preamble beside noteWrite and notePass.
		write(t, b.App.ConfigPath, "beads:\n  - "+t.TempDir()+"\n")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := d.Watch(ctx, "", "", 1, 20*time.Millisecond, 20*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		d.mu.Lock()
		seeded := d.clockWall
		d.mu.Unlock()
		if seeded.IsZero() {
			t.Error("Watch did not seed the suspend witness: suspendTick returns on an unseeded reading, " +
				"so a box suspended inside the loop's first SuspendTick would be named by nothing")
		}
	})
}
