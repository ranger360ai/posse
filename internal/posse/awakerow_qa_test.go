package posse

// ranger-base-awt98, verifying ranger-base-szzoj: two gaps in the awake-time
// reading's own pins, both found by mutation and both green across the whole
// corpus before this file.
//
// ADR 0064 D2 says the `guard-blind:Nh` BUCKET is the awake age, and
// suspendDiscount's doc says a row on a box that did not sleep is
// "byte-for-byte what it was before this reading existed". Neither sentence
// had a pin. The reason for the first is a fixture that cannot reach it:
// TestGovG5SuspendedWindowIsNotBlindness measures 45m of wall against 25m of
// awake, and blindHours renders BOTH as "0h" — so the key is the same string
// whichever age reaches it. MEASURED here 2026-10-03: feeding the wall age to
// guardBlindRow for the key alone, leaving its detail on the awake age,
// survived every arm of every build tag, and the key it produced is the
// 2026-10-01 incident's own `guard-blind:5h`. The second is the same shape one
// level down: making suspendDiscount render for a ZERO sleep put the clause
// "the other 0s was a witnessed SUSPEND" on every G5 and G7 row on every box,
// including the processes ADR 0064 D4 leaves with no witness at all, and
// nothing reds.
//
// No t.Parallel in this file, for the reason
// TestGovSuspendedReadingIsClippedToItsWindow states: these read govNow, and
// cmd/testparallel does not clear that package var for a parallel test.
// Untagged, like govern_test.go whose fixtures these use, so the arms all
// carry them.

import (
	"strings"
	"testing"
	"time"
)

// THE BUCKET IS THE AWAKE AGE (ADR 0064 D1-D2), and the bucket is not
// decoration: it is the identity the pulse FINGERPRINTS, the escalation is
// hourly BY the bucket changing (guardBlindRow's own head, ranger-base-lpoui),
// and the pulse's prompt carries keys and not details. So a row that fires
// with the wall bucket wakes the coordinator URGENT with the exact string the
// 2026-10-01 wake delivered — `guard-blind:5h` — while every detail on it is
// correct.
//
// The fixture has to separate the two ages by a whole hour, which is what the
// existing 45m/25m arm cannot do. Ten hours blind, eight and a half of them a
// witnessed sleep: awake 1h30m, past a 10m budget either way, so the row fires
// in BOTH arms and only the key moves.
//
// MUTATION: compute the key from `blindFor+slept` and leave the detail on
// `blindFor` → the second arm reds naming guard-blind:10h.
func TestQAGuardBlindBucketIsTheAwakeAgeAndNotTheWallAge(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, govGuardCfg+"plan_guard_blind_max: 10m\n")
	seedPlanSnapshot(t, b.App, govNow.Add(-10*time.Hour))

	in := govIn(t, b)
	in.Plan = &fakePlanReader{err: Die("usage endpoint: 429")}

	// The control, and the incident's own key: with no witness the bucket is
	// the wall age, which is what every process without a ledger still reports
	// (ADR 0064 D4) and what the pulse delivered on 2026-10-01.
	g := find(shopSet(t, in), "G5")
	if g == nil || g.Key != "guard-blind:10h" {
		t.Fatalf("premise: a witnessless 10h blind window is guard-blind:10h, got %+v", g)
	}

	in.Suspended = govSuspended(govNow, 8*time.Hour+30*time.Minute)
	g = find(shopSet(t, in), "G5")
	if g == nil {
		t.Fatal("1h30m of awake blindness past a 10m budget is still G5, got no row")
	}
	if g.Key != "guard-blind:1h" {
		t.Errorf("the bucket is %q, want \"guard-blind:1h\" — the key is what the pulse fingerprints and "+
			"prompts on, so a wall bucket wakes the coordinator with the 2026-10-01 string over a box that "+
			"was merely asleep (ADR 0064 D1-D2)", g.Key)
	}
	// And the two numbers the row carries are still the awake age and the
	// discount, so this arm cannot pass on a row that stopped subtracting.
	for _, want := range []string{"blind 1h30m", "awake for 1h30m", "8h30m"} {
		if !strings.Contains(g.Detail, want) {
			t.Errorf("a discounted row must name %q: %q", want, g.Detail)
		}
	}
}

// A BOX THAT DID NOT SLEEP GETS THE ROW IT ALWAYS GOT. suspendDiscount's doc
// promises it ("Empty for a zero discount, so every row on a box that did not
// sleep is byte-for-byte what it was before this reading existed") and ADR
// 0064 D1 rests on it: `posse status` and the cockpit pass no witness at all,
// so every row they render goes through this path with a zero discount.
//
// Both spellings of "did not sleep", because they reach the clause by
// different routes: a nil field (every process but the pulse) short-circuits
// in GovInputs.suspended, and a witness that answers zero (the pulse on a box
// that has not slept) reaches suspendDiscount with slept == 0.
//
// MUTATION: relax suspendDiscount's guard to `slept < 0` → both arms red.
func TestQAWallClockRowsCarryNoSuspendClauseWhenNothingSlept(t *testing.T) {
	zero := func(time.Time) time.Duration { return 0 }

	t.Run("G5", func(t *testing.T) {
		b, _ := newTestBackend(t)
		appendConfig(t, b.App, govGuardCfg+"plan_guard_blind_max: 10m\n")
		seedPlanSnapshot(t, b.App, govNow.Add(-45*time.Minute))
		in := govIn(t, b)
		in.Plan = &fakePlanReader{err: Die("usage endpoint: 429")}

		g := find(shopSet(t, in), "G5")
		if g == nil {
			t.Fatal("premise: a 45m blind window against a 10m budget must be G5")
		}
		noWitness := g.Detail
		if strings.Contains(noWitness, "witnessed SUSPEND") {
			t.Errorf("a process with no witness rendered a suspend clause: %q", noWitness)
		}

		in.Suspended = zero
		g = find(shopSet(t, in), "G5")
		if g == nil {
			t.Fatal("a witness that saw no sleep must not clear the row")
		}
		if g.Detail != noWitness {
			t.Errorf("a zero discount is not byte-for-byte the row before this reading existed:\n no witness: %q\n zero sleep: %q",
				noWitness, g.Detail)
		}
	})

	t.Run("G7", func(t *testing.T) {
		b, _ := newTestBackend(t)
		appendConfig(t, b.App, "autostart_interval: 5m\n")
		lock, held, err := lockWatch(b.App)
		if err != nil || held {
			t.Fatalf("could not take the watch lock: held=%v err=%v", held, err)
		}
		defer lock.Release()
		writeWatchLog(t, b.App, govNow.Add(-72*time.Hour))

		in := govIn(t, b)
		g := find(shopSet(t, in), "G7")
		if g == nil || g.Key != "loop-mute" {
			t.Fatalf("premise: a witnessless 72h quiet must be loop-mute, got %+v", g)
		}
		noWitness := g.Detail
		if strings.Contains(noWitness, "witnessed SUSPEND") {
			t.Errorf("a process with no witness rendered a suspend clause: %q", noWitness)
		}

		in.Suspended = zero
		g = find(shopSet(t, in), "G7")
		if g == nil {
			t.Fatal("a witness that saw no sleep must not clear the row")
		}
		if g.Detail != noWitness {
			t.Errorf("a zero discount is not byte-for-byte the row before this reading existed:\n no witness: %q\n zero sleep: %q",
				noWitness, g.Detail)
		}
	})
}
