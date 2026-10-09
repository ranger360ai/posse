package treepins

// QA pin for ranger-base-7ebv6 (finding 6 of ranger-base-751ha, verifying
// ranger-base-wmaf9's close).
//
// THE CLAIM. G12 — the governance row for a degraded L3 hook wall — reads
// SweepHookWallIdentity and never SweepHookWall. ADR 0023's question is
// identity AND behavior; the governance block drops the behavior half because
// it runs on a TICKER, and the cost of the pair is not a price a reading may
// charge: MEASURED 2026-10-09 over the four repos this box's
// `beads_visibility:` declares, the pair costs 3.41/3.54/3.62s a sweep against
// 375/396/393ms for identity alone, paid every 30s by the cockpit's governance
// block and every two minutes by the pulse.
//
// WHY THIS IS READ FROM THE SOURCE, which is the exception in this shop and
// not the habit. ranger-base-751ha measured that swapping SweepHookWall in
// survives all sixteen G12-G15 pins, so a later hand can restore the
// 3.5s-per-tick reading with the suite green. There is no behavioural
// discriminator to pin instead, and that is a property of the two readings
// rather than a gap in the fixtures: the halves disagree only where the
// behavior half FAILS, and it fails for a renderer regression (a property of
// this binary, identical in every repo), a missing `sh`, or a scratch dir that
// cannot be made — one binary-wide and two process-global, none of them a
// thing a fixture can arrange for one repo without reaching into the
// environment every other test in internal/posse shares. Over every repo state
// a fixture CAN build, the two sweeps return the same verdict, so a pin that
// called the governance set would be green under either call. The cost is
// legible in the source and nowhere else, so the source is where it is held.
//
// It is also why this pin lives in internal/treepins and asks for the
// MEASUREMENT beside the choice: the next hand to touch that line has to walk
// past the numbers that decided it.
//
// MUTATION-CHECKED: swapping `SweepHookWallIdentity()` for `SweepHookWall()`
// in govern.go reds arm 1; deleting the measured figures from l3AskIdentity's
// doc reds arm 2.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Arm 1: the G12 loop's own reading.
func TestQAG12ReadsTheHookWallOnIdentityAlone(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("internal", "posse", "govern.go"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	const want = "for _, r := range in.App.SweepHookWallIdentity().Repos {"
	if !strings.Contains(body, want) {
		t.Errorf("internal/posse/govern.go no longer reads the hook wall as %q.\n"+
			"  G12 draws in the cockpit's governance block every 30s and in the pulse every two minutes. The ADR 0023 pair execs this binary's render once per declared repo and each render runs git over that repo's index: 3.41/3.54/3.62s a sweep against 375/396/393ms for identity alone (MEASURED 2026-10-09, four repos). If the reading legitimately changed, re-measure and re-state this arm over what it is now.", want)
	}
	// And not the pair, anywhere in the file: the G12 loop is govern.go's
	// only hook-wall reader, so a second spelling here IS the swap.
	if strings.Contains(body, "SweepHookWall()") {
		t.Errorf("internal/posse/govern.go calls SweepHookWall(), the ADR 0023 pair, on a surface that ticks.\n" +
			"  See above for the cost. The pair's callers are the one-shot ones — `posse promote`'s epilogue and the watch preamble, both through ReportHookWall.")
	}
}

// Arm 2: the measurement, and what the cheaper reading gives up, on the
// record at the place the choice is made. A number nobody can find is a
// number the next hand will not weigh.
func TestQATheIdentityOnlyAskCarriesItsMeasurement(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("internal", "posse", "gates.go"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if !strings.Contains(body, "l3AskIdentity") {
		t.Fatal("CONTROL: internal/posse/gates.go no longer declares l3AskIdentity, so this pin is aimed at something that moved — re-read the ask")
	}
	for _, want := range []string{
		// The two costs, so the comparison is a measurement and not a feel.
		"3.41/3.54/3.62s", "375/396/393ms",
		// And the honest half: what the drop does not catch, and who does.
		"RENDERER regression",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("l3AskIdentity's doc no longer carries %q.\n"+
				"  It is the whole argument for a second reading of ADR 0023's question: the cost of the pair on a ticker, and the one class of defect identity alone cannot see (which is asked at every launch and once per watch loop instead). Re-measure and re-state rather than deleting.", want)
		}
	}
}
