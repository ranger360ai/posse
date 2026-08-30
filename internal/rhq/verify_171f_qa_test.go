package rhq

// QA pin for ranger-base-171f, found verifying rangerhq-a2g6 under
// ranger-base-cld5.
//
// The pause gate returns from the whole pass (dispatch.go, `return 0, nil`),
// not from the fire loop — so while the shop is paused the reap, the land
// sweep, the plan-guard reading, verify-after and the bead-loss census never
// run. Three documents and the close comment say the opposite ("Nothing else
// is stopped ... they sit below this line on purpose"), and the shipped pin
// asserts today's behaviour indirectly, by requiring that a declined pass
// forks no bd at all.
//
// This is a GREEN pin over a live defect, deliberately: it asserts the hole
// as it stands and carries the inversion in its own failure message, so the
// day the epilogue starts running while paused this test says so instead of
// quietly agreeing. It is not a claim that the current behaviour is right.
//
// Its own witness is the first arm: the identical rig, unpaused, reaps the
// session. That arm holds on both sides of the fix, so a rig that stopped
// being able to reap at all fails here rather than passing this pin for the
// wrong reason.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQAPausedPassStillSkipsTheEpilogue(t *testing.T) {
	// The witness: unpaused, this rig reaps a closed bead's idle session and
	// runs the epilogue. Without it, "still standing" below would be green
	// over a rig that never reaps anything.
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("the unpaused control pass failed: %v\n%s", err, dispatcherOut(d))
	}
	if _, alive := b.readMeta("ranger-repo-a-1"); alive {
		t.Fatalf("the witness arm did not reap — this rig cannot show the epilogue running, so the paused arm below would measure nothing:\n%s",
			dispatcherOut(d))
	}

	// The pin: the same rig, paused.
	pb, pfake := newTestBackend(t)
	pd := newTestDispatcher(t, pb)
	writePersona(t, pb.App, "ranger", "[go]")
	reapCandidate(t, pb, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, pfake)
	pausedShop(t, pb.App, "coordinator", "waiting on the operator")

	n, err := pd.Run("", "", 0)
	if err != nil || n != 0 {
		t.Fatalf("a declined pass is not a failed pass and dispatches nothing: n=%d err=%v", n, err)
	}
	if _, alive := pb.readMeta("ranger-repo-a-1"); !alive {
		t.Fatalf("ranger-base-171f LOOKS FIXED: a paused pass reaped the closed session, which is what the gate's own comment always claimed it did.\n"+
			"That is the wanted behaviour, so: delete this pin, and invert the bd-fork assertion in TestDispatchDeclinesThePassWhilePaused, which today requires the opposite.\n%s",
			dispatcherOut(pd))
	}
	if _, err := os.Stat(filepath.Join(pfake, "bd-calls.log")); err == nil {
		t.Fatalf("ranger-base-171f LOOKS PARTLY FIXED: a paused pass forked the tracker, so some of the epilogue is running again — re-read this pin and the one it names.\n%s",
			dispatcherOut(pd))
	}
}
