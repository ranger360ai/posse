package posse

// The 2026-08-31 incident (ranger-base-c3vqe): the fleet's meter credential
// went stale, every read after it was a 401 or a 429, and for nineteen hours
// the shop kept hiring on the degraded arm of ADR 0018 §1 — because
// the instance's Dial E caps were armed and the ledger was counting fine.
// It was counting DOLLARS. The account's weekly window
// climbed 89% → 96% behind the frozen snapshot, and the operator caught it
// by hand with 4% left.
//
// So the licence to degrade is asked of the meter, from the last reading it
// managed (blindheadroom.go). Hermetic like its neighbours: blindRig's dead
// endpoint for the blindness, an injected Spend for the dollars, an injected
// clock for the age — and a seeded snapshot for the reading, because the
// whole point is a fleet that HAD a meter and lost it.

import (
	"strings"
	"testing"
	"time"
)

// blindHeadroomCfg is the operator's live shape on the day: a 7d threshold
// the last reading was UNDER (89 < 95, so the sighted pass ran and nothing
// tripped), with Dial E armed. The incident needs both — a park that came
// from a tripped threshold would prove nothing about the caps.
const blindHeadroomCfg = "plan_guard_5h: 70\nplan_guard_7d: 90\n" + ledgerCaps

// ledgerCaps is ADR 0018's own published pair, as ledgerArmedCfg uses them.
// Not the instance's live numbers: a fixture only has to be a cap, and a
// test that quotes the operator's config is instance-ops content in a public
// repo (ADR 0024 D1) that no gate is scanning Go for.
const ledgerCaps = "budget_pass: 30\nbudget_day: 250"

// seedReading puts one real reading in the shared snapshot, through the real
// cache and the rig's real endpoint. Not a hand-written file: the snapshot's
// shape is plancache.go's, and a fixture that guesses it goes green the day
// the shape moves.
//
// It does not spend the rig's one ready bead on a sighted pass, which a
// `r.run(t)` would — the pass under test has to have work to park.
func (r *blindRig) seedReading(t *testing.T, fiveH, sevenD float64) {
	t.Helper()
	r.ps.setWindows(fiveH, sevenD)
	c := r.d.App.PlanCache("seed")
	c.Reader = r.ps.reader()
	c.Now = func() time.Time { return r.clock }
	u, _, err := c.Read(0)
	if err != nil {
		t.Fatalf("seed reading: %v", err)
	}
	if got := win(u, "7d"); got != sevenD {
		t.Fatalf("seed: 7d read back %g, want %g — the rig is not seeding what the test thinks", got, sevenD)
	}
	if _, _, ok := r.d.App.PlanCache("seed").LastReading(); !ok {
		t.Fatal("seed: the snapshot was not shared — every assertion below would be about an empty file")
	}
	// Back to the reading the rig's own siblings use, so nothing downstream
	// depends on the seed still being what the endpoint would answer.
	r.ps.setWindows(12, 40)
}

// THE INCIDENT. Last reading 89% of the weekly window, caps armed, ledger
// readable and well under both — and the pass parks anyway, because a dollar
// cap is not a brake on the plan window.
func TestBlindParksWhenTheLastReadingLeftNoHeadroom(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) } // 27% of the pass cap — the ledger is happy
	r.seedReading(t, 30, 89)
	r.blind()
	r.at(19 * time.Hour)

	if n := r.run(t); n != 0 {
		t.Fatalf("blind with no headroom must park, caps or no caps: %d dispatched\n%s", n, r.out())
	}
	out := r.out()
	for _, want := range []string{
		"plan guard: blind 19h00m",
		"last reading 7d at 89% is past the 80% braking rung",
		"read 19h00m ago",
		"a dollar cap is not a brake on the plan window",
		"— skipped",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the park line must carry %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "degraded, running under ledger brake") {
		t.Errorf("this is the 19h day: the caps do not license it\n%s", out)
	}
	if calls := bdCalls(t, r.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a park claims nothing, got: %s", calls)
	}
}

// The park is EVIDENCE-driven, and this is the other side of it: the same
// blindness, the same caps, the same dollars — a last reading with room, and
// ADR 0018 §1 stands exactly as written. Without this arm the change above
// would just be "park on blindness", which is the 2026-08-26 outage back.
func TestBlindStillDegradesWhenTheLastReadingHadHeadroom(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.seedReading(t, 30, 79) // one point under the rung
	r.blind()
	r.at(19 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("a reading with room is §1 unchanged: %d dispatched\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "degraded, running under ledger brake") {
		t.Errorf("want the degraded line, got:\n%s", r.out())
	}
	if strings.Contains(r.out(), "no headroom") || strings.Contains(r.out(), "braking rung") {
		t.Errorf("79%% is headroom:\n%s", r.out())
	}
}

// A machine with NO reading is left on §1's arm too, and on purpose: that is
// the 2026-08-26 shape (a credential posse could not read, from the first
// pass), and parking it cost a measured hour of zero dispatch. This rule
// parks on evidence, never on ignorance — pinned so the next edit cannot
// quietly widen it into the outage it is not for.
func TestBlindWithNoReadingEverIsUnchanged(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.blind()
	r.at(19 * time.Hour)

	if _, _, ok := r.d.App.PlanCache("test").LastReading(); ok {
		t.Fatal("setup: this rig must have no snapshot at all")
	}
	if n := r.run(t); n != 1 {
		t.Fatalf("no reading is no evidence, and §1's arm is unchanged: %d\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "degraded, running under ledger brake") {
		t.Errorf("want the degraded line, got:\n%s", r.out())
	}
}

// The stronger refusal, and the one the operator wrote down themselves: over
// `plan_guard_7d:` a SIGHTED pass would have skipped. Going blind is not a
// promotion from skipped to running.
func TestBlindParksOverTheOperatorsOwnThreshold(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.seedReading(t, 30, 96)
	r.blind()
	r.at(19 * time.Hour)

	if n := r.run(t); n != 0 {
		t.Fatalf("past the threshold the sighted pass skips; the blind one must not run: %d\n%s", n, r.out())
	}
	// Named by the KEY the operator would edit, not by the rung — the two
	// refusals send them to different lines of config.yaml.
	if !strings.Contains(r.out(), "last reading 7d at 96% is over plan_guard_7d: 90%") {
		t.Errorf("want the threshold refusal, got:\n%s", r.out())
	}
}

// The escape hatch is untouched: `plan_guard_blind_max: 0` never reaches the
// fork at all, so it never reaches this rule either.
func TestBlindMaxZeroIsUntouchedByTheHeadroomRule(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg+"\nplan_guard_blind_max: 0")
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.seedReading(t, 30, 89)
	r.blind()
	r.at(19 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("the hatch still never fails closed: %d\n%s", n, r.out())
	}
	if strings.Contains(r.out(), "no headroom") || strings.Contains(r.out(), "— skipped") {
		t.Errorf("under the hatch there is no fork to reach:\n%s", r.out())
	}
}

// Attended is untouched for the same reason: a hand-run pass fails open
// before the fork, and a human at the keyboard is the witness the whole
// blind window is premised on.
func TestHeadroomRuleIsUnattendedOnly(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg)
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.seedReading(t, 30, 89)
	r.blind()
	r.at(19 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("an attended pass fails open at any blind age: %d\n%s", n, r.out())
	}
	if strings.Contains(r.out(), "no headroom") {
		t.Errorf("attended blindness never reaches the fork:\n%s", r.out())
	}
}

// Self-healing, like every other part of the blind window: the first good
// reading clears the park, with no operator action and no sticky state.
// (A reading is what clears it — not the clock, and not a raised cap.)
func TestHeadroomParkClearsOnTheFirstGoodReading(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.seedReading(t, 30, 89)
	r.blind()
	r.at(19 * time.Hour)
	if n := r.run(t); n != 0 {
		t.Fatalf("setup: want the park first, got %d\n%s", n, r.out())
	}

	// The week rolled: the endpoint is up and the window is back down.
	r.ps.setWindows(12, 40)
	r.sighted()
	r.at(19*time.Hour + 2*time.Minute)
	if n := r.run(t); n != 1 {
		t.Fatalf("a good reading releases the park: %d\n%s", n, r.out())
	}
	if !strings.Contains(r.err(), "reading restored after") {
		t.Errorf("want the recovery line, got %q", r.err())
	}
}

// ── the rule itself, without a rig ─────────────────────────────────────────

// planHeadroomRefusal is thin enough that testing it only through a
// dispatcher is how it stops being re-checked when the plumbing moves
// (cockpit.go planOffState's rule, applied here).
func TestPlanHeadroomRefusal(t *testing.T) {
	th := map[string]float64{"5h": 70, "7d": 90}
	for _, tc := range []struct {
		name string
		th   map[string]float64
		last PlanUsage
		want string
	}{
		{"room everywhere", th, PlanUsage{{Name: "5h", Pct: 30}, {Name: "7d", Pct: 61}}, ""},
		{"the incident", th, PlanUsage{{Name: "5h", Pct: 30}, {Name: "7d", Pct: 89}},
			"last reading 7d at 89% is past the 80% braking rung"},
		{"over the operator's threshold", th, PlanUsage{{Name: "5h", Pct: 30}, {Name: "7d", Pct: 96}},
			"last reading 7d at 96% is over plan_guard_7d: 90%"},
		// The threshold refusal is the stronger statement and comes first
		// even when an EARLIER window is already in the braking band — the
		// two loops are not interleaved, and this is which one wins.
		{"threshold beats rung", map[string]float64{"5h": 95, "7d": 90},
			PlanUsage{{Name: "5h", Pct: 85}, {Name: "7d", Pct: 96}},
			"last reading 7d at 96% is over plan_guard_7d: 90%"},
		// …and within the rung loop it is the adapter's order, which is
		// meaning: the window whose exhaustion hurts most is listed first.
		{"rung in adapter order", map[string]float64{"5h": 95, "7d": 90},
			PlanUsage{{Name: "5h", Pct: 82}, {Name: "7d", Pct: 88}},
			"last reading 5h at 82% is past the 80% braking rung"},
		// Boundaries, each matching the code that already owns it:
		// planGuard trips strictly ABOVE a threshold…
		{"exactly at the threshold", map[string]float64{"7d": 89}, PlanUsage{{Name: "7d", Pct: 89}},
			"last reading 7d at 89% is past the 80% braking rung"},
		// …and Dial E's step-down is >=, so 80 is already braking.
		{"exactly at the rung", th, PlanUsage{{Name: "7d", Pct: 80}},
			"last reading 7d at 80% is past the 80% braking rung"},
		{"just under the rung", th, PlanUsage{{Name: "7d", Pct: 79.9}}, ""},
		// A window nobody set a threshold for still gets the rung: the
		// braking band is Dial E's and needs no plan_guard_ key.
		{"unthresholded window", map[string]float64{}, PlanUsage{{Name: "30d", Pct: 90}},
			"last reading 30d at 90% is past the 80% braking rung"},
		{"empty reading", th, PlanUsage{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := planHeadroomRefusal(tc.th, tc.last); got != tc.want {
				t.Errorf("planHeadroomRefusal = %q, want %q", got, tc.want)
			}
		})
	}
}

// LastReading is LastReadAt's other half and keeps its rules: a snapshot
// holding only a cooldown is not a reading, and neither is a missing file.
// The blind park reads this — a "reading" conjured out of a 429 entry would
// park the fleet on windows nobody ever measured.
func TestLastReadingIsAReadingOrNothing(t *testing.T) {
	dir := t.TempDir()
	c := &PlanCache{Path: dir + "/plan-usage.json"}

	if _, _, ok := c.LastReading(); ok {
		t.Error("no file is no reading")
	}
	c.store(planEntry{RetryAt: time.Now().Add(time.Hour)})
	if u, _, ok := c.LastReading(); ok {
		t.Errorf("a cooldown-only entry is not a reading, got %v", u)
	}
	at := time.Date(2026, 8, 30, 23, 9, 0, 0, time.UTC)
	c.store(planEntry{At: at, Windows: PlanUsage{{Name: "7d", Pct: 89}}})
	u, got, ok := c.LastReading()
	if !ok || win(u, "7d") != 89 || !got.Equal(at) {
		t.Errorf("LastReading = %v, %v, %v; want the 89%% snapshot at %v", u, got, ok, at)
	}
	// And the two halves agree, because one is the other.
	if gotAt, okAt := c.LastReadAt(); !okAt || !gotAt.Equal(got) {
		t.Errorf("LastReadAt = %v, %v; want %v, true", gotAt, okAt, got)
	}
}
