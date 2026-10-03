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
// Six arms:
//
//	the reading     a wall clock that moved five and a half hours while the
//	                monotonic clock moved three seconds is named as a
//	                suspend, with both readings in the line; two clocks that
//	                moved together are not.
//	the floor       a wall step under SuspendFloor is a clock being SET, and
//	                is silent.
//	the direction   a wall clock that went BACKWARD is silent, not a negative
//	                suspend.
//	the seam        the witness reads d.Clocks and never d.now(), or every
//	                fixture that ages the dispatcher's clock prints a suspend.
//	the subtraction suspendTick's wall gap is not a Time.Sub — the one claim
//	                in this file no fixture can hold, pinned at the source.
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

// The floor. The two readings are taken a nanosecond apart, so the only other
// thing that separates them is the wall clock being SET — ntpd stepping a
// second, an operator fixing a timezone — and that is not a suspend.
//
// MUTATION: drop the `slept < SuspendFloor` return → the first arm reds.
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

	t.Run("a wall clock that went backward is silent", func(t *testing.T) {
		t.Parallel()
		d, c := suspendRig(t)
		// Monotonic ahead of wall: the wall clock was set back an hour. That
		// makes every wall-clock reading in the shop read FRESHER than it is,
		// which is a real gap and a different line (suspend.go's head) — what
		// is pinned here is that this witness says nothing rather than
		// reporting a negative suspend as a positive one.
		c.advance(-time.Hour, 30*time.Second)
		d.suspendTick()
		if out := dispatcherOut(d); out != "" {
			t.Errorf("a backward wall step reached the suspend line:\n%s", out)
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
// MUTATION: rewrite the wallGap line as `wall.Sub(prevWall)` → this reds,
// and every other arm in this file stays green, which is why it exists.
func TestQASuspendWallGapIsNotAMonotonicSubtraction(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "suspend.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "suspendTick" {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("suspendTick is gone from suspend.go; this sweep is not reading the file it thinks it is")
	}
	unix := 0
	ast.Inspect(body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "Sub":
			t.Errorf("suspendTick calls .Sub: Time.Sub prefers the MONOTONIC reading when both operands carry one, "+
				"so the wall gap becomes the monotonic gap and the witness is silent forever (%s)", fset.Position(sel.Pos()))
		case "UnixNano":
			unix++
		}
		return true
	})
	if unix < 2 {
		t.Errorf("suspendTick must take its wall gap from two UnixNano readings; found %d", unix)
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
