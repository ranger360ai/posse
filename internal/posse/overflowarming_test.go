package posse

// ADR 0010 §3, amended 2026-08-29 (ranger-base-qs0z, built on
// ranger-base-gxgc): the requirement for an overflow move is AT LEAST ONE
// ARMED BRAKE on the target pool, not `plan_guard_overflow_cap:`
// specifically. The bead cap was written as the stand-in for a meter that
// did not exist; where the target now has one of its own — today only grok,
// keyed by name and deliberately not a registry — the meter arms the move
// on its own.
//
// The four corners, and what each one is here to catch:
//
//	cap  meter  →
//	 ✓     ✗    armed, cap applies                 (shipped; overflow_test.go)
//	 ✗     ✓    armed, the METER is the brake      (this file)
//	 ✓     ✓    armed, both apply, reading first   (verifyesa0j_qa_test.go)
//	 ✗     ✗    off, one line naming both ways     (this file)
//
// "Armed" is an arming test over three config keys and never a reading
// (PoolMeterArming): a HALF-armed meter is off under §6 and arms nothing,
// which is the case that decides whether this is a brake or a hole.
//
// MUTATIONS RUN — eight, all killed (each reds at least the tests named):
//   - `Overflow.On` without `|| o.Meter` → every meter-only test reds.
//   - `PoolMeterArming` returning (true, "") unconditionally → the
//     half-armed table reds, and so do TestOverflowWithoutCapIsOff and the
//     three fail-closed ledger pins in overflow_qa_test.go.
//   - `PoolMeterArming` without its `runtime != GrokPoolRuntime` guard →
//     "unmetered target, grok's keys armed" reds.
//   - `overflowFor`'s cap check without `ov.Capped()` → the meter-only
//     launches red on a "0/0 in 7d" park.
//   - `PlanGuardOverflow` dropping `Meter:` from the both-set return →
//     the ledger-fault test reds (the cap goes and takes overflow with it),
//     and so does TestPlanGuardOverflowConfig.
//   - `dropCap` never keeping the Overflow (`if false`) → same test reds.
//   - `grokPoolSkip` never braking (`!st.Read || true`) → the
//     over-threshold park reds, with TestGrokPool* and the both-brakes
//     ordering pin.
//   - `overThreshold` always taking the capped branch (`if false`) →
//     TestOverflowArmedByMeterAloneLaunches reds on the header.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// armedMeterCfg is the target pool's own meter, fully armed, at the round
// test factor: $0.50 per point, so the full pool is $50 and $5 is 10%.
const armedMeterCfg = "grok_guard_week: 70\n" + grokPoolCfg

// meterPass is an overflowPass on the fixture's grok clock with one session's
// worth of pool spend already on disk. dollars is what the meter will read:
// $5 → 10%, $40 → 80%, against the 70% threshold above.
func meterPass(t *testing.T, cfg string, dollars float64) *overflowFixture {
	t.Helper()
	f := overflowPass(t, cfg, overflowPID, `["go","tier:standard"]`)
	f.d.Now = func() time.Time { return grokPoolNow }
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	at := grokPoolLastReset.Add(time.Hour)
	grokPoolSession(t, home, "s1",
		grokPoolUser(at, "Work beads issue ranger-base-gxgc (t)")+
			grokPoolTurn(at, "p-s1", usdTicks(dollars)))
	return f
}

// The new arm, on the real launch path: `plan_guard_overflow: grok` with NO
// cap and the pool's own meter armed and under threshold moves the bead.
// Before this it was overflow off — §3 made the cap required — and the pass
// parked every on-meter bead.
//
// The assertions are the whole move, not just the count: the session is
// created on the overflow runtime, the prompt says so, and the ledger line
// is written anyway (§3: it feeds the metric, and a cap set later).
func TestOverflowArmedByMeterAloneLaunches(t *testing.T) {
	f := meterPass(t, "plan_guard_overflow: grok\n"+armedMeterCfg, 5)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("an armed pool meter arms the move with no cap, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "[grok ← overflow]") {
		t.Errorf("the bead must be shown moving to the second pool:\n%s", out)
	}
	if got := delivered(t, f.b.App, f.fake); !strings.Contains(got, "runtime/tier: grok/standard") {
		t.Errorf("the launch itself must go to the overflow runtime:\n%s", got)
	}
	// The arming notice names the brake that is actually holding the pool,
	// and does not invent a cap of zero to report a count against.
	if !strings.Contains(out, "overflow grok, armed by grok's own pool meter (no bead cap set)") {
		t.Errorf("the trip header must name the brake that armed the move:\n%s", out)
	}
	if strings.Contains(out, "in 7d") {
		t.Errorf("no cap is set, so no bead count may be reported as one:\n%s", out)
	}
	// The reading is still taken and named — it is the brake, so the
	// operator gets the number it decided on.
	if !strings.Contains(out, "! grok pool: estimated 10%") || !strings.Contains(out, "grok_guard_week: 70%") {
		t.Errorf("the meter that armed the move must report its reading:\n%s", out)
	}
	if l := f.ledger(t); len(l) != 1 || !strings.Contains(l[0], " grok a-1 ranger") {
		t.Errorf("the ledger is written on every overflow launch, cap or no cap: %v", l)
	}
	if s := f.errb.String(); s != "" {
		t.Errorf("a fully armed brake is not a config complaint: %q", s)
	}
}

// The same arm through --dry-run, which is the other place the move is
// rendered: the decision is shown and nothing is acted on. A render that
// disagreed with the launch about whether the move is armed would be a
// dry-run that lies about the pass it is previewing.
func TestOverflowArmedByMeterAloneDryRunShowsTheMove(t *testing.T) {
	f := meterPass(t, "plan_guard_overflow: grok\n"+armedMeterCfg, 5)
	f.d.DryRun = true

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 || !strings.Contains(out, "[grok ← overflow]") {
		t.Fatalf("--dry-run must show the meter-armed move, got n=%d:\n%s", n, out)
	}
	if l := f.ledger(t); l != nil {
		t.Errorf("--dry-run writes no ledger: %v", l)
	}
	if log := calls(t, f.fake); strings.Contains(log, "workspace create") {
		t.Errorf("--dry-run creates nothing:\n%s", log)
	}
}

// The control the launch above rests on: the meter is a BRAKE and not a
// permission slip. Same config, same pass, $40 of the $50 pool spent — 80%
// against a 70% threshold — and the bead parks on the pool's own line with
// nothing claimed and nothing ledgered.
//
// Without this, "meter-armed overflow launches" would pass just as happily
// over a meter that reads nothing and stops nothing.
func TestOverflowArmedByMeterAloneParksOverThreshold(t *testing.T) {
	f := meterPass(t, "plan_guard_overflow: grok\n"+armedMeterCfg, 40)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 0 {
		t.Fatalf("the meter is the brake here and it is over threshold, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "grok pool: estimated 80% of the weekly pool used > grok_guard_week: 70% — skipped") {
		t.Errorf("the bead must park on the pool's own reading:\n%s", out)
	}
	if calls := bdCalls(t, f.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a parked bead must not be claimed, got: %s", calls)
	}
	if l := f.ledger(t); l != nil {
		t.Errorf("nothing moved, so nothing is ledgered: %v", l)
	}
}

// §6: a local meter is armed or off, and a HALF-armed one arms nothing. Each
// row below sets `plan_guard_overflow: grok` with no cap and one of the three
// meter inputs missing or unusable — overflow is off, the bead parks on the
// plain trip line, and the stderr line names both ways to arm the move plus
// which input this meter is short of.
//
// This is the row that decides whether the amendment is a brake or a hole: an
// arming test that accepted two keys out of three would arm overflow on a
// meter that can never take a reading.
func TestOverflowHalfArmedMeterArmsNothing(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cfg   string
		names string
	}{
		{"no threshold", grokPoolCfg, "grok_guard_week: + grok_pool_reset: + grok_pool_usd_per_point:"},
		{"no reset", "grok_guard_week: 70\ngrok_pool_usd_per_point: 0.50\n", "grok_pool_reset: is unset or unusable"},
		{"no factor", "grok_guard_week: 70\ngrok_pool_reset: " + grokPoolReset + "\n", "grok_pool_usd_per_point: is unset or unusable"},
		{"neither", "grok_guard_week: 70\n", "grok_pool_reset: and grok_pool_usd_per_point: are both unset or unusable"},
		{"malformed reset", "grok_guard_week: 70\ngrok_pool_reset: someday\ngrok_pool_usd_per_point: 0.50\n", "grok_pool_reset: is unset or unusable"},
		{"malformed factor", "grok_guard_week: 70\ngrok_pool_reset: " + grokPoolReset + "\ngrok_pool_usd_per_point: lots\n", "grok_pool_usd_per_point: is unset or unusable"},
		{"malformed threshold", "grok_guard_week: soon\n" + grokPoolCfg, "grok_guard_week: + grok_pool_reset: + grok_pool_usd_per_point:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// $5 is well under the threshold: if a half-armed meter DID arm
			// the move, this bead would launch, so the park below is the
			// arming test's verdict and not the reading's.
			f := meterPass(t, "plan_guard_overflow: grok\n"+tc.cfg, 5)

			n, err := f.d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(f.d)
			if n != 0 {
				t.Fatalf("a half-armed meter arms nothing (ADR 0010 §6), got n=%d:\n%s", n, out)
			}
			if !strings.Contains(out, "plan 5h at 78% > 70% — skipped") {
				t.Errorf("the on-meter bead parks on the plain trip line:\n%s", out)
			}
			errs := f.errb.String()
			if !strings.Contains(errs, "needs an armed brake on the target pool") ||
				!strings.Contains(errs, "plan_guard_overflow_cap:") {
				t.Errorf("the off line must name both ways to arm the move: %q", errs)
			}
			if !strings.Contains(errs, tc.names) {
				t.Errorf("want the meter half named %q, got: %q", tc.names, errs)
			}
			if l := f.ledger(t); l != nil {
				t.Errorf("overflow off writes no ledger: %v", l)
			}
		})
	}
}

// Neither brake, said in full. The line an operator gets when they point
// `plan_guard_overflow:` at a pool and stop has to name BOTH ways to arm it,
// because "set a cap" alone hides that the pool they chose has a meter that
// would have done — and on a target with no meter at all it must say that
// too, rather than advertising three keys that will never arm anything
// there.
func TestOverflowNeitherBrakeNamesBothWaysToArmIt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		cfg    string
		want   []string
		absent string
	}{
		{"metered target, meter unset", "plan_guard_overflow: grok\n",
			[]string{"needs an armed brake on the target pool",
				"plan_guard_overflow_cap: N (beads per rolling 7 days",
				"grok's own pool meter fully armed (grok_guard_week: + grok_pool_reset: + grok_pool_usd_per_point:)"}, ""},
		// The tripwire, from the other side: the meter is keyed on the one
		// runtime that has one. Another target is not armed by grok's keys,
		// and is not told to go and set them.
		{"unmetered target, grok's keys armed", "plan_guard_overflow: codex\n" + armedMeterCfg,
			[]string{"needs an armed brake on the target pool",
				"codex has no pool meter posse can read, so the cap is its only brake"}, "grok_guard_week:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{ConfigPath: filepath.Join(t.TempDir(), "config.yaml")}
			if err := os.WriteFile(a.ConfigPath, []byte(tc.cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			var errb strings.Builder
			if got := a.PlanGuardOverflow(&errb); got.On() {
				t.Errorf("neither brake is overflow off, got %+v", got)
			}
			for _, w := range tc.want {
				if !strings.Contains(errb.String(), w) {
					t.Errorf("want %q in the off line, got: %q", w, errb.String())
				}
			}
			if tc.absent != "" && strings.Contains(errb.String(), tc.absent) {
				t.Errorf("must not name %q on a target that has no meter: %q", tc.absent, errb.String())
			}
		})
	}
}

// A cap that is SET and unusable is a typo, and a typo stays visible even
// when the pool is held by something else. The meter keeps overflow armed;
// the line says the cap is the part that is off, so the operator is not left
// believing in a bead count that is not counting.
func TestOverflowMalformedCapNamedButMeterStillArms(t *testing.T) {
	f := meterPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: lots\n"+armedMeterCfg, 5)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("the meter still arms the move, got n=%d:\n%s", n, out)
	}
	errs := f.errb.String()
	if !strings.Contains(errs, `plan_guard_overflow_cap: "lots" is not a bead count`) ||
		!strings.Contains(errs, "overflow stays armed on grok's own pool meter") {
		t.Errorf("the typo must be named, and the line must say what is still holding the pool: %q", errs)
	}
}

// The ledger fault, under §3's either-brake rule. An unreadable or
// unappendable `overflow.log` is a fault in the CAP's instrument: the count
// is unknown or unrecordable, so that brake goes. It is not a fault in the
// meter, which is a config key and a transcript scan, so where the meter is
// armed the move survives — and where it is not, this is the pre-existing
// refusal unchanged (TestQAOverflowRefusesAReadableButUnwritableLedger,
// TestQAOverflowCorruptTargetLedgerLineFailsClosed).
func TestOverflowLedgerFaultDropsTheCapNotTheMeter(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mode  os.FileMode
		body  string
		names string
	}{
		// A line nothing can date: the count is UNKNOWN, which is exactly
		// what takes the cap out (ranger-base-lasj).
		{"unreadable", 0o644, "not a ledger entry at all\n", "unreadable"},
		// Readable and empty, but nothing can be added to it: a cap counted
		// off it would be spent again every pass and record none of it
		// (ranger-base-2y96). The meter has no such dependency.
		{"unappendable", 0o444, "", "cannot be appended to"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := meterPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 2\n"+armedMeterCfg, 5)
			if err := os.MkdirAll(f.b.App.StateDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(f.b.App.OverflowLogPath(), []byte(tc.body), tc.mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(f.b.App.OverflowLogPath(), tc.mode); err != nil {
				t.Fatal(err)
			}
			if tc.mode == 0o444 {
				// The hostile condition has to be real: root defeats 0444
				// and would turn this into a false pass (ranger-base-c00).
				if err := f.b.App.AppendOverflow(LedgerEntry{Runtime: "grok"}); err == nil {
					t.Skip("test process can append to a 0444 ledger")
				}
			}

			n, err := f.d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(f.d)
			if n != 1 {
				t.Fatalf("the meter is untouched by a file the cap could not use, got n=%d:\n%s", n, out)
			}
			errs := f.errb.String()
			if !strings.Contains(errs, tc.names) ||
				!strings.Contains(errs, "the bead cap is off for this pass; overflow stays armed on grok's own pool meter") {
				t.Errorf("the fault must be named, and the line must say which brake survived it: %q", errs)
			}
			if strings.Contains(errs, "overflow off this pass") {
				t.Errorf("overflow is not off — the meter armed it: %q", errs)
			}
			if !strings.Contains(out, "overflow grok, armed by grok's own pool meter; eligible beads step over") {
				t.Errorf("a pass that steps over still says so on the report:\n%s", out)
			}
			if strings.Contains(out, "in 7d") {
				t.Errorf("the cap is off, so no bead count may be reported as a brake:\n%s", out)
			}
		})
	}
}
