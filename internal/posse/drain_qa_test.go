package posse

// ranger-base-e9d9: the drain. On 2026-08-30 a `dispatch --watch` loop
// survived two SIGTERMs 40s apart and a SIGINT while it was between launches
// and only waiting on sessions; SIGKILL was what stopped it — the one exit
// that risks landing mid-reap, which is why a graceful stop exists at all.
//
// The signals were delivered and the context WAS cancelled. What ignored it
// was the gather: `Watch` checks ctx between passes, and under ADR 0028 §1's
// rolling Run there is no "between passes" while a bead is in flight. A wait
// leg is PromptWaitMS — fifteen minutes in production — with a ladder above
// it that re-waits to WaitCeiling, four hours, and none of it consulted the
// loop's context.
//
// So the pin is at the loop's own surface: cancel while a leg is genuinely in
// flight, and the loop must come back. The control arm is the same fixture
// left alone — without it, a loop that returned because the fixture never got
// a bead in flight would read as the fix working.
//
// MUTATIONS RUN:
//   - drop the stop case from gather's select (back to `r := <-p.result`) →
//     the cancelled arm reds after 30s with the pass still gathering, which
//     is the bug's own shape.
//   - make `stopped()` return an always-closed channel → the control arm reds
//     (the loop leaves pass 1 with the leg still in flight).
//
// NOT PINNED, and deliberately: gather's second stop check, the one above
// `statusAfterTimeout`. It takes the same exit the select already guarantees,
// so no observable of the LOOP can tell it apart — what it saves is a status
// probe and an orphaned `herdr agent wait` on the way out, and the only
// fixture that reaches it is a leg erroring in a spin. Its own comment says
// it is defensive.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The leg the drain has to abandon. Not a margin: it is two orders of
// magnitude outside every window asserted below, so "the loop returned" and
// "the loop waited the leg out" cannot be confused for one another at any
// load. Production's is PromptWaitMS (15m); this is the same shape, shorter
// only so an abandoned fake outlives the suite by seconds instead of hours.
const drainLegMS = "120000"

func TestQADrainEndsAWatchLoopWaitingOnASession(t *testing.T) {
	// Cancelled: the operator's SIGTERM, which is all signal.NotifyContext
	// hands the loop (cmd/posse/main.go).
	t.Run("cancelled while a leg is in flight", func(t *testing.T) {
		out, done, cancel := drainFixture(t)

		waitForOut(t, out, "in flight, gathering")
		cancel()

		var passes int
		select {
		case passes = <-done:
		case <-time.After(30 * time.Second):
			t.Fatalf("TERM/INT must end the loop while a wait leg is in flight — this one waited it out, which is the SIGKILL the drain needed:\n%s", out.String())
		}
		s := out.String()
		if passes < 1 {
			t.Errorf("the loop must have run the pass it was cancelled inside, got %d passes:\n%s", passes, s)
		}
		// Ended, not abandoned: the bead stays claimed, so the next loop
		// finds it held rather than free and nothing re-fires a session that
		// is still working.
		if !strings.Contains(s, "watch loop stopping") || !strings.Contains(s, "claim kept") {
			t.Errorf("a leg abandoned by the drain keeps its claim and says so:\n%s", s)
		}
		if strings.Contains(s, "closed by") || strings.Contains(s, "settled") {
			t.Errorf("the agent never settled, so the drain must judge nothing:\n%s", s)
		}
	})

	// The control arm: the same fixture, no cancel. The loop must still be
	// inside the pass — proof that the 120s leg really does hold it, and
	// that the arm above measured the cancel and not the fixture running out
	// of work. A false failure here needs the leg to end 60x early.
	t.Run("left alone, the loop stays in the pass", func(t *testing.T) {
		out, done, _ := drainFixture(t)

		waitForOut(t, out, "in flight, gathering")
		select {
		case <-done:
			t.Fatalf("the loop returned with a %sms leg still in flight and nobody asking it to stop — the fixture is not holding it, so the cancelled arm proves nothing:\n%s", drainLegMS, out.String())
		case <-time.After(2 * time.Second):
		}
		// Held INSIDE the pass, not merely still alive: a loop that dropped
		// the leg and went round again is also "not returned", and it is the
		// gather this bead is about. The pass is over the instant a second
		// header or a backoff line appears, and neither may.
		if s := out.String(); strings.Contains(s, passHeader+"2") || strings.Contains(s, "next pass in") {
			t.Fatalf("pass 1 must still be gathering its one leg — the loop left it, so the cancelled arm is not measuring the drain:\n%s", s)
		}
	})
}

// drainFixture is one persona, one ready bead, a prompt leg that will not
// come back inside this test's lifetime — and the loop running over it.
//
// It owns the loop because it owns the JOIN, and the join is the part that
// was missing (ranger-base-06bvw). Cancelling the context ends the loop and
// nothing else: by design the drain abandons the in-flight `agent prompt`,
// and that leg is a forked posse.test still sleeping out drainLegMS with
// this subtest's t.TempDir as its RHQ_FAKE_DIR. Nothing waited for it, so
// t.TempDir removed the tree minutes before the child MkdirAll'd it back to
// write its window — 385 of the 769 stale `Test*` trees in the operator's
// $TMPDIR on 2026-09-02 were these two subtests, at 0755 rather than
// t.TempDir's 0700, which is what proves they were recreated and not merely
// left. The claims above are unchanged: the leg is still abandoned where
// each arm measures it, and only afterwards is it joined.
//
// Cleanup order is the whole trick. This t.Cleanup is registered after every
// t.TempDir newTestBackend took, so LIFO runs it FIRST — cancel, join the
// loop, join the child, and only then does anything get removed.
func drainFixture(t *testing.T) (out *syncBuf, done <-chan int, cancel context.CancelFunc) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	out = &syncBuf{}
	d.Out = out
	writePersona(t, b.App, "ranger", "[go]")
	agentPerLaunch(t, fake)
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	os.WriteFile(filepath.Join(fake, "prompt-delay-ms"), []byte(drainLegMS), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	passes := make(chan int, 1)
	// Two signals, not one: an arm may or may not read the pass count, and
	// the cleanup must be able to wait for the loop either way.
	loop := make(chan struct{})
	go func() {
		p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond)
		passes <- p
		close(loop)
	}()
	t.Cleanup(func() {
		cancel() // idempotent: the cancelled arm has already called it
		select {
		case <-loop:
		case <-time.After(30 * time.Second):
			t.Errorf("the watch loop never returned after cancel:\n%s", out.String())
		}
		// One persona, one ready bead: exactly one leg is in flight, and
		// the abandoned prompt goroutine does nothing after the child but
		// send into a buffered channel nobody reads (dispatch.go), so the
		// child is the last writer left to join.
		joinHeldPrompts(t, fake, 1)
	})
	return out, passes, cancel
}
