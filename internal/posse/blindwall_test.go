//go:build posse_arm2

package posse

// ADR 0065 — the hiring gate ages its evidence on the WALL clock, and a
// witnessed suspend is never subtracted from its budget.
//
// The pair this pins: `plan_guard_blind_max:` has two readers over one
// timestamp and one threshold. G5's row (govern.go blindPast) subtracts the
// suspend and is awake-denominated since ADR 0064; this gate does not, for
// 0065 D1's reason — the budget is the maximum AGE OF THE EVIDENCE the guard
// will hire on, and the evidence is a reading of an account other spenders
// move while the lid is shut. The arms below are blindGuard on a Dispatcher
// whose `suspends` ledger covers the whole blind window: the subtraction the
// gate declines is in hand, in this process, and the gate still brakes.
//
// WHAT THESE CANNOT SEE, said here rather than scanned for in source text.
// The hand-driven clock pins the SUBTRACTION — that no `suspendedSince` is
// taken off the budget — and nothing more. It cannot pin the DENOMINATION:
// `blindWall` reads UnixNano precisely because `Time.Sub` would prefer a
// monotonic reading when both operands carry one, and a fixture clock built
// from `time.Date` carries none, so a Sub here would be a wall difference too
// and every arm below would pass over the defect. That is suspend.go's head
// verbatim — a Time whose wall and monotonic readings disagree cannot be
// synthesized (Add moves both; only a real suspend separates them) — and it
// is why the denomination is pinned at the source by UnixNano having no
// monotonic variant, not by a test.

import (
	"strings"
	"testing"
	"time"
)

// blindSlept hands the rig a witnessed suspend of slept that lands inside
// the blind window, and a witness clock in the rig's own frame so the ledger
// and the gate read one timeline. back sits a minute inside now so the span
// is covered at both ends and the clip in suspendedSince is inert.
func (r *blindRig) blindSlept(blind, slept time.Duration) {
	r.d.Clocks = func() (time.Time, time.Duration) { return r.clock, 0 }
	r.d.suspends = []suspendSpan{{back: blindT.Add(blind - time.Minute), slept: slept}}
}

// D1, the whole ruling: 45m of wall blindness, 40m of it witnessed sleep, a
// 10m budget — the row's own fixture numbers
// (TestGovG5SuspendedWindowIsNotBlindness, where they produce NO G5) — and
// the gate brakes anyway, naming the wall age in the line.
//
// Both sides of blindFork, because the brake's two outcomes print the
// duration from two different places: the park line per bead and the
// degraded line on d.Out.
func TestBlindGateDoesNotSubtractWitnessedSuspend(t *testing.T) {
	for _, tc := range []struct {
		name, cfg string
		spend     func(time.Time) *CostReport
		want      int
		wantLines []string
	}{{
		name: "park, the meter is the last armed brake",
		cfg:  guardOn,
		want: 0,
		// 45m and not 5m: the ledger says 40m of this window was a shut
		// lid, and the gate does not care.
		wantLines: []string{"plan guard: blind 45m", "— skipped"},
	}, {
		name:      "degrade, with Dial E armed",
		cfg:       ledgerArmedCfg,
		spend:     func(time.Time) *CostReport { return spendOf(7.50, nil) },
		want:      1,
		wantLines: []string{"plan guard: blind 45m", "degraded, running under ledger brake"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			r := newBlindRig(t, tc.cfg)
			r.d.Unattended = true // past needs it: this gate only brakes unattended
			if tc.spend != nil {
				r.d.Spend = tc.spend
			}
			r.blind()
			r.at(45 * time.Minute)
			r.blindSlept(45*time.Minute, 40*time.Minute)

			// The premise of the whole arm: the subtraction the gate
			// declines is available to it, on this Dispatcher, in this
			// process — the same ledger G5's row reads. A fixture that
			// proved nothing more than an empty ledger would pass under
			// either ruling.
			if got := r.d.suspendedSince(r.d.blindSince); got != 40*time.Minute {
				t.Fatalf("the ledger must cover the blind window, else this arm is vacuous: suspendedSince = %s", got)
			}

			if n := r.run(t); n != tc.want {
				t.Fatalf("a witnessed sleep does not extend the blind budget: %d dispatched, want %d\n%s", n, tc.want, r.out())
			}
			for _, want := range tc.wantLines {
				if !strings.Contains(r.out(), want) {
					t.Errorf("want %q in the brake's line, got:\n%s", want, r.out())
				}
			}
			// The awake reading is what (a) would have printed, and it is
			// under the budget — so its absence is the ruling, not a
			// formatting detail.
			if strings.Contains(r.out(), "blind 5m") {
				t.Errorf("the gate printed the AWAKE age; ADR 0065 D1 says wall:\n%s", r.out())
			}
		})
	}
}

// The restored line reads the same window, so it must print the same number
// (D2). Blind across the sleep, then sighted: one pass, two lines, one age.
func TestBlindRestoredLineNamesTheSameWallAge(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.blind()
	r.at(45 * time.Minute)
	r.blindSlept(45*time.Minute, 40*time.Minute)
	if n := r.run(t); n != 0 {
		t.Fatalf("the blind pass must brake first, else there is nothing to restore from: %d dispatched\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "plan guard: blind 45m") {
		t.Fatalf("want the wall age on the braking pass, got:\n%s", r.out())
	}

	// The clock does not move: the restored line measures from the seed the
	// braking pass held, so the two numbers are over the identical window
	// and any drift between them is the two expressions disagreeing.
	r.sighted()
	if n := r.run(t); n != 1 {
		t.Fatalf("the first good reading clears the clock and the same pass hires: %d dispatched\n%s", n, r.out())
	}
	if !strings.Contains(r.err(), "reading restored after 45m blind") {
		t.Errorf("the restored line must name the same wall age the brake did, got:\n%s", r.err())
	}
}
