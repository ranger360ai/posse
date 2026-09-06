package posse

// ranger-base-av446, verifying the close of ranger-base-v62hj.
//
// v62hj's own subject — the target pool's meter read at the overflow MOVE,
// ahead of the bead cap — went with ADR 0010 §1's removal of automatic
// overflow (ranger-base-6xx37). What av446 actually bought outlives it: the
// grok pool reading is MEMOISED for the pass across every site that asks
// for it, and nothing held that.
//
// MEASURED on v62hj's commit: `grokPoolGuard`'s
// `if d.grokPool != nil { return d.grokPool }` removed — the whole of the
// memo — and the 90 grok/overflow/plan-guard tests in this package stayed
// GREEN (137.9s baseline, 104.7s mutant, both exit 0). Every assertion on
// the reading is `strings.Contains`, so a pass that scans the transcripts
// twice and prints the line twice reads exactly like one that does it once.
// What that costs is the reason the memo exists: `passBudget`'s own header
// calls rescanning 50MB of transcripts per bead "the most expensive way to
// say no".
//
// The corner v446 pinned it at was one bead crossing the two sites the
// overflow ladder created. With the ladder gone the memo is what is left, so
// this asks the guard again AFTER a pass that already read it — the same
// second site, without the mechanism that used to supply one — and counts
// the lines.
//
// MUTATION RUN: the memo's early return removed → a second `! grok pool:`
// line, this test RED.

import (
	"strings"
	"testing"
)

func TestQAGrokPoolReadingIsTakenOncePerPass(t *testing.T) {
	f := grokPoolPass(t, "grok_guard_week: 70\n"+grokPoolCfg)
	// $5 of a $50 pool: 10%, well under the 70% threshold, so the bead
	// launches and the reading is taken on the way.
	f.spend(t, "s1", 5)

	n, out := f.run(t)
	if n != 1 {
		t.Fatalf("fixture: want the bead launched on the metered pool, got %d:\n%s", n, out)
	}
	if got := strings.Count(out, "! grok pool:"); got != 1 {
		t.Fatalf("fixture: the pass must take and name exactly one reading, got %d:\n%s", got, out)
	}

	// The second site, asked directly: the pass already has its answer, and
	// a memo that stopped memoising would rescan the transcripts and print
	// the line again.
	if line := f.d.grokPoolSkip(GrokPoolRuntime); line != "" {
		t.Fatalf("fixture: 10%% of the pool is under the threshold, so this must not brake: %q", line)
	}
	if got := strings.Count(dispatcherOut(f.d), "! grok pool:"); got != 1 {
		t.Errorf("the pool reading is once per pass, memoised across call sites; printed %d times:\n%s",
			got, dispatcherOut(f.d))
	}
}
