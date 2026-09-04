package posse

// ranger-base-3ryit: the pass must come round while prompts are outstanding.
//
// The incident is in passcarry.go's head. What is pinned here is the bead's
// third ask, both halves of it, each against a control arm that is the shape
// the shop had before the fix — an unbounded gather window, which is exactly
// what `Run` did when the settle-driven refill kept feeding the set it was
// draining:
//
//  1. a pass COMPLETES with prompts still outstanding (the carried leg is
//     still in flight, unjudged, and pass 2 has come and gone);
//  2. a seat that became free with NO settle event is offered ready work by
//     the next pass. No settle happens anywhere in this fixture, so the
//     refill cannot be what hires — only a second pass can.
//
// Plus the two things the carry itself must not break: a stopping loop still
// joins the legs it is carrying and prints their kept claims (the drain,
// ranger-base-e9d9, whose own fixture now holds its pass open deliberately),
// and a seat occupied by a carried leg is not fired into twice.
//
// MUTATIONS RUN (go test -overlay, 2026-09-04):
//   - drop the window from gatherRound (`if d.Refill` → `if false`, which is
//     the shop as it was) → "the pass comes round" and "the free seat is
//     hired" both red at their 90s budget; both control arms stay green,
//     which is what makes them controls and not decoration.
//   - carry the legs but recreate busy/sessFail per pass (seatState always
//     returns fresh maps) → "the free seat is hired" reds on a-2: pass 2
//     fires a second bead into the seat its own carried leg is sitting on.
//   - notePassStall always speaks → the witness arm reds on the second line.
//   - print the carry line without inFlightLine() → "names what it carries"
//     reds on the bead the line does not name.
//   - drop `case <-d.settled()` from Watch's timer select → the wake arm reds
//     at 90s with the bead settled, judged, and the loop asleep on a
//     five-minute backoff.
//
// NOT PINNED, measured and deliberate: removing drainCarried's defer from
// Watch leaves the drain arm GREEN. The abandoned leg's goroutine prints its
// kept claim the moment the context closes, and in a test binary that
// outlives the loop it always eventually wins the race against the assertion
// below. What the join buys is the case the suite cannot reach — a real
// process whose main returns and exits while that goroutine is still between
// the print and the send — so the arm below pins the OBSERVABLE (the loop
// ends with a carried leg, the claim is kept) and not the join.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// loopJoinWait bounds every join of the watch loop here. Generous for
// waitForOut's own reason and measured against it: each pass forks the test
// binary several times, and under -race those forks are tens of seconds — a
// loop cancelled mid-pass finishes that pass's launches before it returns.
// MEASURED 2026-09-04: this file's drain arm needs ~40s under -race on a
// box also running the rest of this package, against ~1s without it. The
// claim it bounds is "the loop ends", never "the loop ends quickly".
const loopJoinWait = 90 * time.Second

// carryRig is a watch loop over a bead whose prompt will not come back, with
// a second bead arriving in the queue behind it.
type carryRig struct {
	out  *syncBuf
	repo string
	fake string
	d    *Dispatcher
	loop <-chan struct{}
	stop context.CancelFunc
	// legs is how many prompt legs this arm expects to leave in flight, read
	// by the cleanup's join. One is the fixture's own; an arm that hires the
	// second seat sets two.
	legs int
}

// ranger is the seat that never settles; scout is the seat that is free the
// whole time and can only be hired by a pass.
func carryFixture(t *testing.T, window time.Duration) *carryRig {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	rig := &carryRig{out: &syncBuf{}, fake: fake, d: d, legs: 1}
	d.Out = rig.out
	writePersona(t, b.App, "ranger", "[go]")
	writePersona(t, b.App, "scout", "[docs]")
	agentPerLaunch(t, fake)
	rig.repo = qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	// The queue MOVES between the two passes (fake-ready-next.json, the one
	// swap the fake bd makes): b-1 is filed while a-1's prompt is still in
	// flight, which is the incident's own shape — the operator's P1 arrived
	// after the pass that would have offered it had already started. a-1 is
	// still in the second answer because real bd's `ready` is a fresh
	// reading and a claimed bead does not vanish from it; the busy map is
	// what must refuse it, and the launch count below is the assertion that
	// it did.
	//
	// a-2 rides in the same answer, in a-1's own lane: it is the bead that
	// must NOT go out, because ranger's seat is held by a leg this loop is
	// carrying. An idle session does not make a persona busy (a finished
	// bead's session is left for the reap), so the busy map is the only
	// thing that knows — and its lifetime is the in-flight set's.
	os.WriteFile(filepath.Join(rig.repo, "fake-ready-next.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]},{"id":"b-1","title":"u","labels":["docs"]},`+
			`{"id":"a-2","title":"v","labels":["go"]}]`), 0o644)
	// The leg that will not come back inside this test's lifetime —
	// drainFixture's own constant, for its reason: two orders of magnitude
	// outside every window asserted below.
	os.WriteFile(filepath.Join(fake, "prompt-delay-ms"), []byte(drainLegMS), 0o644)
	d.GatherWindow = window

	ctx, cancel := context.WithCancel(context.Background())
	rig.stop = cancel
	loop := make(chan struct{})
	rig.loop = loop
	go func() {
		defer close(loop)
		d.Watch(ctx, "", "", 0, 50*time.Millisecond, 100*time.Millisecond)
	}()
	// Registered after every t.TempDir newTestBackend took, so LIFO runs it
	// first: cancel, join the loop, join the abandoned legs, and only then is
	// anything removed (drain_qa_test.go's note on the 385 stale trees).
	t.Cleanup(func() {
		cancel()
		select {
		case <-loop:
		case <-time.After(loopJoinWait):
			t.Errorf("the watch loop never returned after cancel:\n%s", rig.out.String())
		}
		joinHeldPrompts(t, fake, rig.legs)
	})
	return rig
}

// The bead's ask 1, at the loop's own surface: the pass comes round with the
// prompt still outstanding, and says what it is carrying.
func TestQAPassCompletesWithPromptsStillOutstanding(t *testing.T) {
	t.Parallel()

	t.Run("bounded gather: the pass comes round and names what it carries", func(t *testing.T) {
		t.Parallel()
		rig := carryFixture(t, 0) // 0 = the loop's own window, the production shape

		waitForOut(t, rig.out, "in flight, gathering")
		waitForOut(t, rig.out, passHeader+"2")
		waitForOut(t, rig.out, "next pass in")

		s := rig.out.String()
		// The carry line, with the set named — the bead's ask 2 in the
		// ordinary case: an operator reading this log sees which bead is
		// still out, in which session, for how long.
		if !strings.Contains(s, "still in flight, carried into the next pass: a-1 in ") {
			t.Errorf("a pass that returns with a leg outstanding must name it:\n%s", s)
		}
		// Carried, not abandoned and not judged: the agent never settled, so
		// nothing may have been decided about the bead.
		if strings.Contains(s, "closed by") || strings.Contains(s, "settled") {
			t.Errorf("the leg is still in flight — a pass that judged it is not carrying it:\n%s", s)
		}
	})

	// The control arm: the same fixture with the gather unbounded, which is
	// what a rolling Run did before this bead. The pass must NOT come round —
	// proof that the fixture really does hold a leg in flight, and that the
	// arm above measured the window and not a bead that quietly settled.
	t.Run("unbounded gather: pass 1 holds, which is the defect", func(t *testing.T) {
		t.Parallel()
		rig := carryFixture(t, time.Hour)

		waitForOut(t, rig.out, "in flight, gathering")
		// Sixty of this loop's intervals. The hour-long window makes the
		// hold structural rather than a race — a timer that cannot fire —
		// so this budget is for the fixture's own forks, not for the claim.
		time.Sleep(3 * time.Second)
		if s := rig.out.String(); strings.Contains(s, passHeader+"2") || strings.Contains(s, "next pass in") {
			t.Fatalf("with the gather unbounded, pass 1 must still be holding its leg — the arm above is not measuring the window:\n%s", s)
		}
	})
}

// The bead's ask 3, second half, and the operator-visible cost: a seat that
// became free with no settle behind it is offered ready work by the next
// pass. Nothing settles in this fixture, so no refill can fire — the only
// thing that can hire scout is a pass that comes round.
func TestQAFreeSeatIsHiredByTheNextPassWithNoSettle(t *testing.T) {
	t.Parallel()

	t.Run("bounded gather: the free seat is hired", func(t *testing.T) {
		t.Parallel()
		rig := carryFixture(t, 0)
		rig.legs = 2 // a-1's leg, and b-1's once it launches

		waitForOut(t, rig.out, "in flight, gathering")
		waitForOut(t, rig.out, "creating session "+SessionForBead("scout", rig.repo, "b-1"))
		// Read the log only once pass 2 is OVER. Its enumeration is still
		// running when b-1's launch line lands, and a-2 comes after b-1 —
		// asserting a-2's absence against a half-written pass would assert
		// the read order, not the seat.
		waitForCount(t, rig.out, "dispatched · next pass in", 2)

		s := rig.out.String()
		if strings.Contains(s, "closed by") || strings.Contains(s, "settled") {
			t.Errorf("no bead settled here, so the hire must be the PASS's and not a refill's:\n%s", s)
		}
		// ADR 0028 §5 observable 4 across the carry: ranger's seat is held by
		// a leg this loop is carrying, so the pass that hires scout must not
		// hire ranger for a-2 — the busy map's lifetime is the in-flight
		// set's, not the pass's. Read off the pass that hired scout: the
		// enumeration named a-2 in the same fireLoop.
		log := calls(t, rig.fake)
		if strings.Contains(log, SessionForBead("ranger", rig.repo, "a-2")) {
			t.Errorf("a-2 is in a-1's lane and a-1's leg is still in flight — two beads on one seat:\n%s\n%s", s, log)
		}
		if n := strings.Count(log, "workspace create --label "+SessionForBead("ranger", rig.repo, "a-1")); n != 1 {
			t.Errorf("a-1's seat is occupied by a carried leg; want exactly 1 session for it, got %d:\n%s", n, log)
		}
	})

	// The control arm: unbounded, which is the incident. b-1 is ready, the
	// scout seat is empty, and nothing ever offers it the work.
	t.Run("unbounded gather: the ready bead sits out, which is the incident", func(t *testing.T) {
		t.Parallel()
		rig := carryFixture(t, time.Hour)

		waitForOut(t, rig.out, "in flight, gathering")
		time.Sleep(3 * time.Second)
		if s := rig.out.String(); strings.Contains(s, "b-1") {
			t.Fatalf("with the gather unbounded there is no second pass, so nothing can hire the free seat — the arm above is not measuring the fix:\n%s", s)
		}
	})
}

// The drain, over a CARRIED leg (ranger-base-e9d9 under this bead's fix).
// Before the carry, every leg was joined by the Run that fired it; now the
// pass returns first and Watch joins them on its way out (drainCarried). The
// observable is unchanged and is the one that matters: the loop ends, and the
// bead's claim is kept rather than handed back.
func TestQADrainJoinsTheLegsAPassCarried(t *testing.T) {
	t.Parallel()
	rig := carryFixture(t, 0)

	waitForOut(t, rig.out, "in flight, gathering")
	waitForOut(t, rig.out, passHeader+"2") // the leg is genuinely carried now
	rig.stop()

	select {
	case <-rig.loop:
	case <-time.After(loopJoinWait):
		t.Fatalf("the loop must end while a carried leg is in flight:\n%s", rig.out.String())
	}
	if s := rig.out.String(); !strings.Contains(s, "watch loop stopping") || !strings.Contains(s, "claim kept") {
		t.Errorf("a carried leg abandoned by the drain keeps its claim and says so, in this loop's own log:\n%s", s)
	}
}

// The carry's one cost, paid back: a leg that lands AFTER its pass's window
// closed must not wait out the backoff to be judged. The wait goroutine pokes
// the loop when it reports (settled/enqueue), so the next pass is immediate —
// the same trigger ADR 0016 §1 ratified, over this process's own channel
// rather than herdr's event socket, which is nil in every test here
// (newTestDispatcher) and can be down in production.
//
// The margin is the measurement: the backoff here is five minutes and the
// bead must be judged inside waitForOut's 90s, so the arm can only pass by
// the wake. MUTATION: drop the `case <-d.settled()` from Watch's timer select
// → red at 90s with the bead settled, unjudged, and the loop asleep.
func TestQACarriedSettleWakesTheNextPassAtOnce(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	out := &syncBuf{}
	d.Out = out
	writePersona(t, b.App, "ranger", "[go]")
	agentPerLaunch(t, fake)
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	// Long enough that the prompt is still in flight when the window closes,
	// short enough that it lands well inside the budget below.
	os.WriteFile(filepath.Join(fake, "prompt-delay-ms"), []byte("400"), 0o644)
	// The window closes almost immediately; the backoff is five minutes.
	d.GatherWindow = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	loop := make(chan struct{})
	go func() {
		defer close(loop)
		d.Watch(ctx, "", "", 0, 5*time.Minute, 5*time.Minute)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-loop:
		case <-time.After(loopJoinWait):
			t.Errorf("the watch loop never returned after cancel:\n%s", out.String())
		}
	})

	// The leg is genuinely carried first — otherwise this measures a pass
	// that judged its own prompt and never needed waking.
	waitForOut(t, out, "still in flight, carried into the next pass: a-1")
	// The bead's own verdict is the wait goroutine's and lands whenever the
	// agent settles, carry or no carry (gather does the judging; the pass
	// counts it, sweeps and refills the seat). So the ✓ is the fixture's
	// witness that the leg really landed, and the PASS HEADER is the
	// discriminator: with the wake it follows in milliseconds, and without it
	// the loop is asleep for five minutes.
	waitForOut(t, out, "closed by ranger")
	waitForOut(t, out, passHeader+"2")
}

// The bead's ask 2 as a witness of last resort: whatever holds a pass open in
// future, the log says so once and names the set holding it. Off the clock
// rather than off a loop, because the reading is a subtraction and the shape
// worth pinning is the once-per-stall rule and what the line carries.
func TestQAPassStallWitnessSaysItOnceAndNamesTheSet(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	var out syncBuf
	d.Out = &out
	now := time.Now()
	d.Now = func() time.Time { return now }
	d.notePass()
	d.inflight = []*pendingBead{{
		is:       RepoIssue{BdIssue: BdIssue{ID: "a-1"}},
		session:  "ranger-posse-a-1",
		prompted: time.Now().Add(-40 * time.Minute),
	}}
	budget := watchdogPassBudget(3*time.Minute, 3*time.Minute)

	// Inside the budget: nothing to say.
	now = now.Add(budget - time.Second)
	d.passStallTick(budget)
	if s := out.String(); s != "" {
		t.Fatalf("a pass inside its budget is not a finding:\n%s", s)
	}

	// Past it: the finding, naming the set.
	now = now.Add(2 * time.Second)
	d.passStallTick(budget)
	first := out.String()
	for _, want := range []string{"no pass has completed for", "a-1 in ranger-posse-a-1", "ranger-base-3ryit"} {
		if !strings.Contains(first, want) {
			t.Errorf("the stall witness must carry %q:\n%s", want, first)
		}
	}

	// And once, not once per tick: a stalled pass coexists with a log that
	// is scrolling, which is the whole reason it was invisible.
	now = now.Add(10 * budget)
	d.passStallTick(budget)
	if got := strings.Count(out.String(), "no pass has completed for"); got != 1 {
		t.Errorf("the stall is said once per stall, got %d lines:\n%s", got, out.String())
	}

	// Re-armed by a pass that completes, so a loop that stalls twice says so
	// twice.
	d.notePass()
	now = now.Add(2 * budget)
	d.passStallTick(budget)
	if got := strings.Count(out.String(), "no pass has completed for"); got != 2 {
		t.Errorf("a completed pass re-arms the witness, got %d lines:\n%s", got, out.String())
	}
}
