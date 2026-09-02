package posse

// ranger-base-av446, verifying the close of ranger-base-v62hj.
//
// v62hj moved the target pool's meter into `overflowFor`, ahead of the bead
// cap (ADR 0010 §3). One bead can now take that reading at TWO sites in one
// pass: at the overflow move, and again in the launch loop's `grokPoolSkip`
// on the runtime it moved to. The close says the second one is "a memoised
// no-op on the moved ones", and dispatch.go's comment block says the same.
// Nothing held it.
//
// MEASURED on the close's own commit: `grokPoolGuard`'s
// `if d.grokPool != nil { return d.grokPool }` removed — the whole of the
// memo — and the 90 grok/overflow/plan-guard tests in this package stay
// GREEN (137.9s baseline, 104.7s mutant, both exit 0). Every assertion on
// the reading is `strings.Contains`, so a pass that scans the transcripts
// twice and prints the line twice reads exactly like one that does it once.
// What that costs is the reason the memo exists: `passBudget`'s own header
// calls rescanning 50MB of transcripts per bead "the most expensive way to
// say no", and it is now per bead at two sites rather than one.
//
// So this pins the count, at the corner where both sites actually run: under
// the threshold and under the cap, the meter lets the move through, the bead
// launches on the overflow pool, and the launch loop asks the same question
// again about the runtime it landed on.
//
// MUTATION RUN: the memo's early return removed → two `! grok pool:` lines,
// this test RED. It is the only test in the package that reds on it.
import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestQAGrokPoolReadingIsTakenOncePerPassOverBothSites(t *testing.T) {
	f := overflowPass(t,
		"plan_guard_overflow: grok\nplan_guard_overflow_cap: 2\ngrok_guard_week: 70\n"+grokPoolCfg,
		overflowPID, `["go","tier:standard"]`)
	f.d.Now = func() time.Time { return grokPoolNow }
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// $5 of a $50 pool: 10%, well under the 70% threshold, so the meter
	// passes the move and the empty ledger passes the cap.
	at := grokPoolLastReset.Add(time.Hour)
	grokPoolSession(t, home, "s1",
		grokPoolUser(at, "Work beads issue ranger-base-esa0j (t)")+
			grokPoolTurn(at, "p-s1", usdTicks(5)))

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("fixture: want the bead launched, got %d:\n%s", n, out)
	}
	// The witness that BOTH sites ran on this bead: it took the overflow
	// move (overflowFor, where the meter is now consulted) and then went
	// through the launch loop on the runtime it moved to, where
	// grokPoolSkip asks the same question again.
	if !strings.Contains(out, "[grok ← overflow]") {
		t.Fatalf("fixture: the bead never moved onto the metered pool, so only one site read it:\n%s", out)
	}
	if got := strings.Count(out, "! grok pool:"); got != 1 {
		t.Errorf("the pool reading is once per pass, memoised across both call sites; printed %d times:\n%s", got, out)
	}
}
