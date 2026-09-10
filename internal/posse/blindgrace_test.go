//go:build posse_arm2

package posse

// THE 2026-09-07 INCIDENT (ranger-base-vq5zz): the last good reading has to
// keep gating while the meter is blind, from the FIRST blind pass and not
// from the end of the grace.
//
// What happened. The weekly plan guard had read over the operator's own
// threshold and correctly skipped every hire for eleven passes. On the
// twelfth the usage endpoint answered 429, the guard went blind, and — still
// inside `plan_guard_blind_max:` — the pass printed
//
//	plan guard: usage endpoint rate-limited, not asking again for 57m —
//	pass not gated
//
// and hired four seats at a shop that was one pass earlier at a KNOWN
// over-threshold reading of its weekly ceiling. The operator refreshed the
// credential that evening and confirmed the reading had been real, so those
// were hires past the brake in fact and not merely in reading.
//
// The blind grace is tolerance for a pass that knows NOTHING (2026-08-26,
// where parking on ignorance cost a measured hour of zero dispatch). It was
// never meant to buy ten minutes of hiring against a reading already over
// the operator's own line, and the refusal that says so was reachable only
// past the grace, from blindFork (blindheadroom.go, 2026-08-31).
//
// Hermetic like its neighbours: the rig's fake endpoint answers 429 the way
// the real one did, an injected Spend for the dollars, an injected clock for
// the ages, and seedReading for the reading the shop already had.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// blindGraceAt is when the pass under test runs: past the shared reading's
// max age (half the blind budget, so 5m), which is what sends a probe to the
// endpoint at all, and well inside the 10m grace, which is where the
// incident happened. A test that ran at 12m would be pinning blindFork
// again — the arm that was already correct.
const blindGraceAt = 6 * time.Minute

// rateLimited is the production failure, not a dead socket: the endpoint
// answers 429 and the cache writes a cooldown, exactly as it did on the day.
func (r *blindRig) rateLimited() {
	r.ps.status = http.StatusTooManyRequests
	r.ps.body = "rate limited"
}

// twoOnMeter gives the pass two candidates on the guarded meter. The done
// condition is a skip for EVERY candidate, and over a one-bead rig "zero
// dispatched" cannot be told from a queue that ran out of work.
func (r *blindRig) twoOnMeter(t *testing.T) {
	t.Helper()
	// A lane of its own for the second bead: one seat per lane per pass, so
	// two beads on ONE label would have the pass skip the second as "lane
	// busy" and prove nothing about the guard.
	writePersona(t, r.d.App, "wren", "[docs]")
	ready := `[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"t","labels":["docs"]}]`
	show := `[{"id":"a-1","title":"t","status":"closed"},{"id":"a-2","title":"t","status":"closed"}]`
	if err := os.WriteFile(filepath.Join(r.repo, "fake-ready.json"), []byte(ready), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.repo, "fake-show.json"), []byte(show), 0o644); err != nil {
		t.Fatal(err)
	}
	agentPerLaunch(t, r.fake) // Dial F wants a fresh session per bead
}

// THE INCIDENT. 96% against a 90% threshold, then a 429, then a pass inside
// the grace: every candidate is skipped, the line names the stale reading
// and its age, and nothing is hired. The line the incident printed —
// "pass not gated" — must not appear at all: this pass IS gated, by the
// reading the guard already had.
func TestBlindInsideTheGraceStillGatesOnTheLastReading(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(7.50, nil) }
	r.twoOnMeter(t)
	r.seedReading(t, 30, 96)
	r.rateLimited()
	r.at(blindGraceAt)

	if n := r.run(t); n != 0 {
		t.Fatalf("a reading over the operator's threshold gates the blind pass too: %d hired\n%s", n, r.out())
	}
	out := r.out()
	for _, want := range []string{
		"blind 6m",
		"last reading 7d at 96% is over plan_guard_7d: 90%",
		"read 6m ago",
		"429",
		"— skipped",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the park line must carry %q, got:\n%s", want, out)
		}
	}
	if got := strings.Count(out, "— skipped"); got != 2 {
		t.Errorf("every candidate on the meter is skipped, want 2 lines, got %d:\n%s", got, out)
	}
	if strings.Contains(r.err(), "pass not gated") {
		t.Errorf("a pass with a gating reading is not an ungated pass: %q", r.err())
	}
	if calls := bdCalls(t, r.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a park claims nothing, got: %s", calls)
	}
	if strings.Contains(out, "no ready work") {
		t.Errorf("a gated pass must not read as an empty queue:\n%s", out)
	}

	// Still gated on the next pass, with the age it has THEN — the park is
	// per pass and the hourly quiet is the fail-open note's alone
	// (rangerhq-llse). Nothing was claimed, so both beads are still ready.
	r.at(8 * time.Minute)
	if n := r.run(t); n != 0 {
		t.Fatalf("still blind, still gated: %d hired\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "read 8m ago") {
		t.Errorf("the repeat carries the reading's real age:\n%s", r.out())
	}
}

// The other side of it, and the reason this is not "park whenever blind":
// the same blindness, the same 429, the same grace — a last reading with
// room, and the grace is exactly what it was.
func TestBlindInsideTheGraceRunsWhenTheLastReadingHadRoom(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(7.50, nil) }
	r.twoOnMeter(t)
	r.seedReading(t, 30, 61)
	r.rateLimited()
	r.at(blindGraceAt)

	if n := r.run(t); n != 2 {
		t.Fatalf("a reading with room leaves the grace untouched: %d hired\n%s", n, r.out())
	}
	if !strings.Contains(r.err(), "pass not gated") {
		t.Errorf("want today's fail-open line, got %q", r.err())
	}
	if strings.Contains(r.out(), "— skipped") {
		t.Errorf("61%% is room:\n%s", r.out())
	}
}

// …and neither does ignorance become evidence. A machine with no reading at
// all keeps the whole grace: that is the 2026-08-26 shape, and parking it
// cost a measured hour of zero dispatch.
func TestBlindInsideTheGraceWithNoReadingEverIsUnchanged(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(7.50, nil) }
	r.twoOnMeter(t)
	r.rateLimited()
	r.at(blindGraceAt)

	if _, _, ok := r.d.App.PlanCache("test").LastReading(); ok {
		t.Fatal("setup: this rig must have no snapshot at all")
	}
	if n := r.run(t); n != 2 {
		t.Fatalf("no reading is no evidence: %d hired\n%s", n, r.out())
	}
	if !strings.Contains(r.err(), "pass not gated") {
		t.Errorf("want today's fail-open line, got %q", r.err())
	}
}

// The gate is the READING's, not the ledger's, so it holds with no cap
// armed — and then it must not talk about caps. "a dollar cap is not a brake
// on the plan window" is the sentence for a pass where a cap is the thing a
// reader would otherwise take for the brake (2026-08-31); on a shop that
// armed none it would name a brake nobody set.
func TestBlindGraceParkWithNoCapsSaysNothingAboutDollars(t *testing.T) {
	r := newBlindRig(t, "plan_guard_5h: 70\nplan_guard_7d: 90")
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(7.50, nil) }
	r.twoOnMeter(t)
	r.seedReading(t, 30, 96)
	r.rateLimited()
	r.at(blindGraceAt)

	if n := r.run(t); n != 0 {
		t.Fatalf("the reading gates whether or not Dial E is armed: %d hired\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "last reading 7d at 96% is over plan_guard_7d: 90%") {
		t.Errorf("want the threshold refusal, got:\n%s", r.out())
	}
	if strings.Contains(r.out(), "dollar cap") {
		t.Errorf("no cap is armed here, so none is being overridden:\n%s", r.out())
	}
}

// `plan_guard_blind_max: 0` is the operator's escape hatch and it still
// means what it says: never fail closed while blind, this brake included.
// It is the one way to hire against a stale braking reading, and it is a
// sentence the operator wrote in their own config — pinned so that widening
// this rule into the hatch takes a decision instead of a diff.
func TestBlindGraceHatchIsUntouched(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg+"\nplan_guard_blind_max: 0")
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(7.50, nil) }
	r.twoOnMeter(t)
	r.seedReading(t, 30, 96)
	r.rateLimited()
	r.at(6 * time.Hour)

	if n := r.run(t); n != 2 {
		t.Fatalf("the hatch never fails closed: %d hired\n%s", n, r.out())
	}
	if strings.Contains(r.out(), "— skipped") {
		t.Errorf("under the hatch there is no blind brake to reach:\n%s", r.out())
	}
}

// Attended is untouched too (TestHeadroomRuleIsUnattendedOnly's rule, and
// the same reason): a hand-run pass fails open, because a human typed the
// command. What it gains is the number the human is failing open ON — a
// witness told "rate-limited" and not "96% six minutes ago" is watching the
// wrong one, and that was true of the incident's line as well.
func TestBlindGraceAttendedRunsButNamesTheStaleReading(t *testing.T) {
	r := newBlindRig(t, blindHeadroomCfg)
	r.d.Spend = func(time.Time) *CostReport { return spendOf(7.50, nil) }
	r.twoOnMeter(t)
	r.seedReading(t, 30, 96)
	r.rateLimited()
	r.at(blindGraceAt)

	if n := r.run(t); n != 2 {
		t.Fatalf("an attended pass fails open at any blind age: %d hired\n%s", n, r.out())
	}
	if strings.Contains(r.out(), "— skipped") {
		t.Errorf("attended is never skipped:\n%s", r.out())
	}
	for _, want := range []string{
		"pass not gated",
		"last reading 7d at 96% is over plan_guard_7d: 90%",
		"read 6m ago",
	} {
		if !strings.Contains(r.err(), want) {
			t.Errorf("the attended line must carry %q, got %q", want, r.err())
		}
	}
}
