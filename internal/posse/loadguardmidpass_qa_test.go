//go:build posse_arm2

package posse

// ranger-base-etsk5 (QA, verifying the close of ranger-base-jfe5z).
//
// THE GAP. ranger-base-jfe5z threaded a `byHand` argument down the launch
// path so the load guard can tell a launch the operator typed from one the
// fleet decided on. Its crew half is pinned four ways over; its FLEET half
// was not pinned at all. Every existing load-guard pin reaches planLaunch
// either through a crew entry point (ByHand true) or through a pass that
// the pass-level gate at dispatch.go:2204 had already skipped — so nothing
// in the suite ever read the argument the fire loop passes. Mutating that
// one word, `d.launchSession(..., prompt, held, false)` to `true`, left
// every load-guard and seat pin GREEN.
//
// WHY IT IS NOT DEAD CODE. App.LoadHigh takes a fresh reading on every
// call — it caches nothing — and a pass gathers for 15m-4h (Run's own
// note above the gate). A box that crosses the ceiling AFTER the pass gate
// read it therefore meets the guard nowhere but here, at the launch, where
// `byHand false` is the only thing that still refuses. Flip that word and a
// saturating pass keeps launching into a box that can no longer fork, which
// is the whole condition the guard was cut for.
//
// THE FIXTURE is that box: the first reading is quiet, so the pass starts
// and the pass-level gate is not what we measure; every reading after it is
// saturated, so the launch meets the guard. The control arm is the same
// fixture with a quiet box throughout — it must actually launch, or a green
// here would only mean the pass never got as far as a session.

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAPassThatSaturatesMidFlightIsStillRefusedAtTheLaunch(t *testing.T) {
	t.Parallel()

	// loads[0] is what the pass gate reads; every later read gets loads[1].
	// Returning the quiet value exactly once is what puts the guard at the
	// launch instead of at the gate.
	setup := func(t *testing.T, first, rest float64) (*Dispatcher, string, *int32) {
		t.Helper()
		b, fake := newTestBackend(t)
		d := newTestDispatcher(t, b)
		writePersona(t, b.App, "ranger", "[go]")
		repo := t.TempDir()
		os.WriteFile(filepath.Join(repo, "fake-ready.json"),
			[]byte(`[{"id":"a-1","title":"fix the thing","priority":1,"labels":["go"]}]`), 0o644)
		os.WriteFile(filepath.Join(repo, "fake-show.json"),
			[]byte(`[{"id":"a-1","title":"fix the thing","status":"closed"}]`), 0o644)
		os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
		idleClaude(t, fake)
		agentPerLaunch(t, fake)
		var reads int32
		b.App.Load1 = func() (float64, error) {
			if atomic.AddInt32(&reads, 1) == 1 {
				return first, nil
			}
			return rest, nil
		}
		return d, fake, &reads
	}

	t.Run("control: a box that stays quiet launches", func(t *testing.T) {
		d, fake, reads := setup(t, 6.0, 6.0)
		n, err := d.Run("", "", 0)
		if err != nil || n != 1 {
			t.Fatalf("the fixture must be able to launch at all, or the refusal arm proves nothing: n=%d err=%v\n%s", n, err, dispatcherOut(d))
		}
		if !strings.Contains(calls(t, fake), "workspace create") {
			t.Fatalf("premise: the control arm must really create a session:\n%s", calls(t, fake))
		}
		if atomic.LoadInt32(reads) < 2 {
			t.Errorf("premise: the guard must be read again at the launch, not only at the pass gate: reads=%d", atomic.LoadInt32(reads))
		}
	})

	t.Run("the box saturates after the pass gate read it", func(t *testing.T) {
		d, fake, reads := setup(t, 6.0, 263)
		n, _ := d.Run("", "", 0)
		out := dispatcherOut(d)

		// Premise, both halves. The pass must have STARTED — otherwise this
		// measures dispatch.go's pass gate, which ranger-base-jfe5z did not
		// touch — and the load must have been read a second time, which is
		// the read that happens inside planLaunch.
		if strings.Contains(out, "pass skipped") {
			t.Fatalf("premise: the pass gate must NOT be what fired here, or the launch guard is not what is measured:\n%s", out)
		}
		if got := atomic.LoadInt32(reads); got < 2 {
			t.Fatalf("premise: the launch never took its own reading (reads=%d), so nothing here measures the fleet arm:\n%s", got, out)
		}

		// The finding itself: byHand is false down the fire loop, so the
		// guard refuses. With it true, the session is created instead.
		if log := calls(t, fake); strings.Contains(log, "workspace create") {
			t.Errorf("a pass that saturated mid-flight launched anyway — the fire loop is handing planLaunch ByHand true (ranger-base-jfe5z's fleet arm):\n%s\n%s", log, out)
		}
		if n != 0 {
			t.Errorf("want 0 dispatched into a box that crossed the ceiling, got %d:\n%s", n, out)
		}
	})
}
