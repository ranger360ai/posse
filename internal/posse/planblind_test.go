package posse

// The plan guard's blind window (rangerhq-6h1, the business manager's
// ruling on rangerhq-30m): fail-open keeps a grace period, then stops — but
// only where nobody is watching. Hermetic: a fake usage endpoint, a fake
// keychain, and an injected clock, so nothing here sleeps ten minutes or
// touches the operator's credentials.

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// blindT is the fixed instant every test's clock starts from.
var blindT = time.Date(2026, 8, 19, 20, 53, 0, 0, time.UTC)

// deadURL is an address nothing is listening on, so Read fails the way it
// fails in the wild: a real transport error, real "usage endpoint
// unreachable" text, no faked error strings.
func deadURL(t *testing.T) string {
	t.Helper()
	return "http://127.0.0.1:1"
}

// blindRig: a dispatcher with a ready bead, a clock the test drives, and a
// plan reader that can be blinded and restored between passes. Returns the
// dispatcher, its stderr, and the two knobs (clock, blind switch).
type blindRig struct {
	d     *Dispatcher
	errb  *strings.Builder
	fake  string
	repo  string
	ps    *planServer // the working endpoint, for its request count
	live  string      // its URL
	dead  string      // the one that refuses connections
	clock time.Time
}

func newBlindRig(t *testing.T, cfg string) *blindRig {
	t.Helper()
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 12, 40) // well under any threshold: only the blind window gates here
	d, errb := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, cfg)
	idleClaude(t, fake)

	r := &blindRig{d: d, errb: errb, fake: fake, repo: repo, ps: ps, live: ps.URL, dead: deadURL(t), clock: blindT}
	d.Now = func() time.Time { return r.clock }
	d.blindSince = blindT
	return r
}

func (r *blindRig) blind()               { planReaderOf(r.d).URL = r.dead }
func (r *blindRig) sighted()             { planReaderOf(r.d).URL = r.live }
func (r *blindRig) at(d time.Duration)   { r.clock = blindT.Add(d) }
func (r *blindRig) out() string          { return dispatcherOut(r.d) }
func (r *blindRig) err() string          { return r.errb.String() }
func (r *blindRig) run(t *testing.T) int { t.Helper(); n, _ := r.d.Run("", "", 0); return n }

const guardOn = "plan_guard_5h: 70\nplan_guard_7d: 85"

// A hand-run pass keeps today's unconditional fail-open, however long the
// clock says it has been blind: the stderr line has a witness when a human
// typed the command, which is the whole premise.
func TestBlindAttendedNeverSkips(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.blind()
	r.at(3 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("an attended pass must fail open at any blind age: %d dispatched\n%s", n, r.out())
	}
	if want := "plan guard: usage endpoint unreachable — pass not gated"; !strings.Contains(r.err(), want) {
		t.Errorf("want today's stderr line %q, got %q", want, r.err())
	}
	if strings.Contains(r.out(), "skipped") {
		t.Errorf("attended is never skipped:\n%s", r.out())
	}
}

// Unattended and under the budget: today's behaviour exactly — one stderr
// line, pass not gated, pass runs.
func TestBlindUnderBudgetRuns(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.blind()
	r.at(9 * time.Minute)

	if n := r.run(t); n != 1 {
		t.Fatalf("9m blind is inside the 10m budget: %d dispatched\n%s", n, r.out())
	}
	if want := "pass not gated"; !strings.Contains(r.err(), want) {
		t.Errorf("want %q, got %q", want, r.err())
	}
}

// Over the budget, unattended: the on-meter bead parks with the blind
// duration in its line. Zero dispatched still makes --watch back off.
func TestBlindOverBudgetSkips(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.blind()
	r.at(12 * time.Minute)

	n := r.run(t)
	if n != 0 {
		t.Fatalf("past the blind budget the pass must not dispatch: %d\n%s", n, r.out())
	}
	want := "plan guard: blind 12m (usage endpoint unreachable) — skipped"
	if !strings.Contains(r.out(), want) {
		t.Errorf("want %q, got:\n%s", want, r.out())
	}
	if calls := bdCalls(t, r.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a blind park must not claim the bead, got: %s", calls)
	}
	base := 30 * time.Second
	if got := NextInterval(base, base, 8*base, n); got != 2*base {
		t.Errorf("--watch must back off after a blind skip: %s, want %s", got, 2*base)
	}
}

// ADR 0013 §3, off-meter arm: an explicit runtime on a different meter is
// still the operator's routing decision, and a blind guarded meter cannot
// park it.
func TestBlindExplicitOffMeterRuntimeRuns(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.d.Runtime = "grok"
	r.blind()
	r.at(12 * time.Minute)

	if n := r.run(t); n != 1 {
		t.Fatalf("--runtime grok must run while the guarded meter is blind: %d dispatched\n%s", n, r.out())
	}
	if strings.Contains(r.out(), "— skipped") {
		t.Errorf("an off-meter launch must not be parked:\n%s", r.out())
	}
	if got := delivered(t, r.d.App, r.fake); !strings.Contains(got, "runtime/tier: grok/") {
		t.Errorf("the launched work prompt must name grok:\n%s", got)
	}
}

// Both blind arms of ADR 0013 §3 share one pass: blind parks the claude bead,
// the grok bead launches on its own runtime, and configured overflow is not
// consulted without a reading. The threshold arm remains pinned in the
// overflow tests.
func TestBlindGuardDecidesPerBeadAndNeverOverflows(t *testing.T) {
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 12, 40)
	d, _ := planDispatcher(t, b, ps)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	writePersona(t, b.App, "metered", "[metered]")
	offMeterPID := "---\nname: offmeter\ndescription: test\nlabels: [offmeter]\nruntime: grok\n---\nYou are offmeter.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "offmeter.md"), []byte(offMeterPID), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := planRepo(t,
		`[{"id":"a-1","title":"metered","labels":["metered"]},{"id":"b-1","title":"off meter","labels":["offmeter"]}]`,
		`[{"id":"b-1","title":"off meter","status":"closed"}]`)
	planConfig(t, b.App, repo, guardOn+"\nplan_guard_overflow: codex\nplan_guard_overflow_cap: 5")
	agentPerLaunch(t, fake)

	clock := blindT.Add(12 * time.Minute)
	d.Now = func() time.Time { return clock }
	d.blindSince = blindT
	d.Unattended = true
	planReaderOf(d).URL = deadURL(t)

	if n, err := d.Run("", "", 0); err != nil || n != 1 {
		t.Fatalf("blind mixed pass dispatched %d, err=%v:\n%s", n, err, dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "a-1") || !strings.Contains(out, "blind 12m") || !strings.Contains(out, "— skipped") {
		t.Errorf("the claude bead must park on the blind guarded meter:\n%s", out)
	}
	if got := delivered(t, b.App, fake); !strings.Contains(got, "runtime/tier: grok/") {
		t.Errorf("the grok bead must launch in the same pass:\n%s", got)
	}
	if strings.Contains(out, "← overflow") {
		t.Errorf("blind never overflows:\n%s", out)
	}
	if _, err := os.Stat(d.App.OverflowLogPath()); !os.IsNotExist(err) {
		t.Errorf("blind writes no overflow ledger (%v)", err)
	}
	if got := strings.Count(bdCalls(t, fake), "--claim"); got != 1 {
		t.Errorf("only the off-meter bead may be claimed, got %d claims:\n%s", got, bdCalls(t, fake))
	}
}

// The first successful reading clears the clock and the same pass proceeds
// — no manual reset, no sticky state, and one line saying it came back.
func TestBlindRecoversInTheSamePass(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.blind()
	r.at(12 * time.Minute)
	if n := r.run(t); n != 0 {
		t.Fatalf("setup: want the on-meter bead parked, got %d dispatched", n)
	}

	r.sighted()
	r.at(14 * time.Minute)
	if n := r.run(t); n != 1 {
		t.Fatalf("a good reading must let the same pass run: %d dispatched\n%s", n, r.out())
	}
	if want := "plan guard: reading restored after 14m blind"; !strings.Contains(r.err(), want) {
		t.Errorf("want %q, got %q", want, r.err())
	}

	// And the clock is genuinely reset: 9 minutes after the good reading is
	// inside the budget again, even though it is 23 minutes after the first
	// failure. (The bead itself is held by the session the last pass made,
	// so the witness here is the absence of a second skip, not a count.)
	before := strings.Count(r.out(), "— skipped")
	r.blind()
	r.at(23 * time.Minute)
	r.run(t)
	if got := strings.Count(r.out(), "— skipped"); got != before {
		t.Errorf("the clock restarts at the good reading, so 9m blind must not skip:\n%s", r.out())
	}
}

// plan_guard_blind_max: 0 is the operator's escape hatch — never fail
// closed, unattended or not.
func TestBlindMaxZeroNeverFailsClosed(t *testing.T) {
	for _, raw := range []string{"0", "0s"} {
		t.Run(raw, func(t *testing.T) {
			r := newBlindRig(t, guardOn+"\nplan_guard_blind_max: "+raw)
			r.d.Unattended = true
			r.blind()
			r.at(6 * time.Hour)

			if n := r.run(t); n != 1 {
				t.Errorf("blind_max 0 is pre-6h1 behaviour everywhere: %d dispatched\n%s", n, r.out())
			}
			if strings.Contains(r.out(), "skipped") {
				t.Errorf("the escape hatch must not skip:\n%s", r.out())
			}
		})
	}
}

// A budget the operator set is the budget that is used, in the house's
// duration forms.
func TestBlindMaxConfigured(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		at   time.Duration
		skip bool
	}{
		{"30s", 20 * time.Second, false},
		{"30s", 30 * time.Second, false}, // strictly over, same as 5h/7d
		{"30s", 31 * time.Second, true},
		{"90", 80 * time.Second, false},
		{"90", 2 * time.Minute, true},
		{"1h", 40 * time.Minute, false},
		{"1h", 61 * time.Minute, true},
	} {
		t.Run(tc.raw+"@"+tc.at.String(), func(t *testing.T) {
			r := newBlindRig(t, guardOn+"\nplan_guard_blind_max: "+tc.raw)
			r.d.Unattended = true
			r.blind()
			r.at(tc.at)

			n := r.run(t)
			if tc.skip && n != 0 {
				t.Errorf("blind_max %s at %s must skip: %d dispatched\n%s", tc.raw, tc.at, n, r.out())
			}
			if !tc.skip && n != 1 {
				t.Errorf("blind_max %s at %s must run: %d dispatched\n%s", tc.raw, tc.at, n, r.out())
			}
		})
	}
}

// A budget that is not a duration is a typo: name it, keep the default, and
// do not say it again every pass.
func TestBlindMaxMalformed(t *testing.T) {
	r := newBlindRig(t, guardOn+"\nplan_guard_blind_max: soon")
	r.d.Unattended = true
	r.blind()
	r.at(12 * time.Minute)

	if n := r.run(t); n != 0 {
		t.Errorf("a malformed budget falls back to the 10m default: %d dispatched", n)
	}
	if !strings.Contains(r.err(), "plan_guard_blind_max") || !strings.Contains(r.err(), "not a duration") {
		t.Errorf("a typo must be visible, got: %q", r.err())
	}
	r.at(30 * time.Minute)
	r.run(t)
	if n := strings.Count(r.err(), "not a duration"); n != 1 {
		t.Errorf("the typo is named once per process, not once per pass: %d times", n)
	}
}

// The guard unconfigured is the guard off: no request, no clock, no skip,
// no line — even unattended, even long past any budget.
func TestBlindNeedsAConfiguredGuard(t *testing.T) {
	r := newBlindRig(t, "")
	r.d.Unattended = true
	r.blind()
	r.at(6 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("unset thresholds are today's behaviour: %d dispatched\n%s", n, r.out())
	}
	if r.errb.Len() != 0 {
		t.Errorf("guard unset is silent: %q", r.err())
	}
}

// The log-noise rule, fail-open half: a blind pass that still RUNS says so
// when the reading first fails and at most once an hour after that. It can
// afford to be quiet — the pass prints its own routing lines either way, so
// nobody reads the silence as an idle loop.
func TestBlindLogNoiseWhileRunning(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.blind()

	// First failure, still inside the budget: today's line, pass runs.
	r.at(2 * time.Minute)
	if n := r.run(t); n != 1 {
		t.Fatalf("2m blind still runs: %d", n)
	}
	if got := strings.Count(r.err(), "pass not gated"); got != 1 {
		t.Fatalf("the first failure is said once: %d", got)
	}

	// Still blind, still under budget, three passes later: nothing new.
	for _, at := range []time.Duration{3 * time.Minute, 5 * time.Minute, 8 * time.Minute} {
		r.at(at)
		r.run(t)
	}
	if got := strings.Count(r.err(), "pass not gated"); got != 1 {
		t.Errorf("the same blind line must not repeat every pass: %d", got)
	}
}

// The parked half (rangerhq-llse): every on-meter bead names the error that
// parked it, with the real blind age. The hourly quiet applies to the
// fail-open note only.
func TestBlindSkipIsNeverSilent(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.blind()

	// The crossing into skipping.
	r.at(12 * time.Minute)
	if n := r.run(t); n != 0 {
		t.Fatalf("12m blind must skip: %d", n)
	}
	want := "plan guard: blind 12m (usage endpoint unreachable) — skipped"
	if !strings.Contains(r.out(), want) {
		t.Fatalf("want %q, got:\n%s", want, r.out())
	}

	// The next hour of skipped passes: each one skipped, each one said.
	for i, at := range []time.Duration{20 * time.Minute, 40 * time.Minute, 60 * time.Minute} {
		r.at(at)
		if n := r.run(t); n != 0 {
			t.Errorf("still blind at %s, still skipping: %d dispatched", at, n)
		}
		if got := strings.Count(r.out(), "— skipped"); got != i+2 {
			t.Fatalf("a skipped pass must say why, every pass: %d lines after %s\n%s", got, at, r.out())
		}
	}
	// …carrying the real age, not the crossing's.
	for _, age := range []string{"blind 20m", "blind 40m", "blind 1h00m"} {
		if !strings.Contains(r.out(), age) {
			t.Errorf("the repeat carries the real age (%s), got:\n%s", age, r.out())
		}
	}
	// And it never reads as an empty queue: bd is not asked, so "no ready
	// work" — the line that made this bug invisible — is never printed.
	if strings.Contains(r.out(), "no ready work") {
		t.Errorf("a skipped pass must not read as an empty queue:\n%s", r.out())
	}
}

// The blind reason must survive the shared read and reach every on-meter
// bead. Dropping it is rangerhq-llse: Watch reports only 0 dispatched.
func TestBlindParkReasonIsNeverEmpty(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.blind()

	r.at(12 * time.Minute)
	r.d.planGuard()
	if r.d.planBlind == "" {
		t.Fatal("empty park reason is rangerhq-llse: Watch would print only 0 dispatched")
	}
	if !strings.Contains(r.d.planBlind, "blind 12m") {
		t.Errorf("park reason must name the blind age, got %q", r.d.planBlind)
	}

	// Inside the hourly quiet window: the per-bead reason is still refreshed.
	r.at(20 * time.Minute)
	r.d.planGuard()
	if !strings.Contains(r.d.planBlind, "blind 20m") {
		t.Fatalf("a second park must carry its current age: %q", r.d.planBlind)
	}
}

// The production error was 429, not a dead socket. After rangerhq-tdy8 a
// 429 also writes a cooldown, so later passes fail with a *different*
// string ("not asking again") without hitting the endpoint. Both must
// still name the skip — silence after the first 429 was the bug.
func TestBlindSkipOn429IsNeverSilent(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.ps.status = http.StatusTooManyRequests
	r.ps.body = "rate limited"

	r.at(12 * time.Minute)
	if n := r.run(t); n != 0 {
		t.Fatalf("12m blind on 429 must skip: %d\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "— skipped") {
		t.Fatalf("the first 429 skip must say why:\n%s", r.out())
	}
	if !strings.Contains(r.out(), "429") {
		t.Errorf("the first skip must name the 429, got:\n%s", r.out())
	}

	// 14m/16m are inside the default 5m cooldown: no second request, a
	// different error. 20m is past it: another 429. All four must speak.
	for i, at := range []time.Duration{14 * time.Minute, 16 * time.Minute, 20 * time.Minute} {
		r.at(at)
		if n := r.run(t); n != 0 {
			t.Errorf("still blind at %s, still skipping: %d dispatched", at, n)
		}
		if got := strings.Count(r.out(), "— skipped"); got != i+2 {
			t.Fatalf("a skipped pass must say why, every pass: %d lines after %s\n%s", got, at, r.out())
		}
	}
	if !strings.Contains(r.out(), "not asking again") && !strings.Contains(r.out(), "rate-limited") {
		t.Errorf("a cooldown skip must still name the rate limit:\n%s", r.out())
	}
	if strings.Contains(r.out(), "no ready work") {
		t.Errorf("a 429 skip must not read as an empty queue:\n%s", r.out())
	}
	if calls := bdCalls(t, r.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a blind park must not claim the bead, got: %s", calls)
	}
}

// Watch is the discriminator and the seed: it marks the loop unattended and
// starts the blind clock at loop start, so the first pass of a fresh
// --watch gets the whole grace instead of an instant skip.
func TestWatchSeedsTheBlindClock(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.blind()
	// A stale clock from before the loop existed — an instant skip if Watch
	// did not reseed it.
	r.d.blindSince = blindT.Add(-6 * time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // one pass, then the loop ends
	passes, err := r.d.Watch(ctx, "", "", 0, time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if passes != 1 {
		t.Fatalf("want 1 pass, got %d", passes)
	}
	if !r.d.Unattended {
		t.Error("Watch must mark the loop unattended")
	}
	if !r.d.blindSince.Equal(blindT) {
		t.Errorf("blind clock seeded at loop start: %s, want %s", r.d.blindSince, blindT)
	}
	if strings.Contains(r.out(), "— skipped") {
		t.Errorf("a fresh loop's first blind pass gets the full grace:\n%s", r.out())
	}
}

func TestBlindFor(t *testing.T) {
	t.Parallel()
	for in, want := range map[time.Duration]string{
		-time.Second:                    "0s",
		0:                               "0s",
		45 * time.Second:                "45s",
		time.Minute:                     "1m",
		12*time.Minute + 30*time.Second: "12m",
		59 * time.Minute:                "59m",
		time.Hour:                       "1h00m",
		80 * time.Minute:                "1h20m",
		25 * time.Hour:                  "25h00m",
	} {
		if got := BlindFor(in); got != want {
			t.Errorf("BlindFor(%s) = %q, want %q", in, got, want)
		}
	}
}

// rangerhq-6h1 / rangerhq-e1n: the budget is strictly over, the same rule
// as the 5h/7d thresholds. At exactly 10m the pass still runs; one
// nanosecond past it, it skips. Two rigs — a pass that ran has claimed
// the bead, so a second run's n=0 would not prove a skip.
func TestBlindBudgetIsStrictlyOver(t *testing.T) {
	t.Run("at 10m", func(t *testing.T) {
		r := newBlindRig(t, guardOn)
		r.d.Unattended = true
		r.blind()
		r.at(10 * time.Minute)
		if n := r.run(t); n != 1 {
			t.Fatalf("exactly 10m is not over the budget: %d dispatched\n%s", n, r.out())
		}
		if strings.Contains(r.out(), "skipped") {
			t.Errorf("exactly at the budget must not skip:\n%s", r.out())
		}
	})
	t.Run("1ns over", func(t *testing.T) {
		r := newBlindRig(t, guardOn)
		r.d.Unattended = true
		r.blind()
		r.at(10*time.Minute + time.Nanosecond)
		if n := r.run(t); n != 0 {
			t.Fatalf("one step past 10m must skip: %d dispatched\n%s", n, r.out())
		}
		if !strings.Contains(r.out(), "— skipped") {
			t.Errorf("want a blind skip, got:\n%s", r.out())
		}
	})
}

// Either threshold alone arms the clock. Unset means off for that window,
// not "both required".
func TestBlindOneThresholdStillArms(t *testing.T) {
	for _, cfg := range []string{"plan_guard_5h: 70", "plan_guard_7d: 85"} {
		t.Run(cfg, func(t *testing.T) {
			r := newBlindRig(t, cfg)
			r.d.Unattended = true
			r.blind()
			r.at(12 * time.Minute)
			if n := r.run(t); n != 0 {
				t.Fatalf("one threshold still arms the clock: %d dispatched\n%s", n, r.out())
			}
			if !strings.Contains(r.out(), "— skipped") {
				t.Errorf("want a blind skip, got:\n%s", r.out())
			}
		})
	}
}

// The production failure is often the keychain, not a dead socket
// (ranger-base-r64). Same park: unattended, over budget, no bd.
func TestBlindSkipOnKeychainUnreadable(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	keychainOnly(planReaderOf(r.d), func() (string, CredMeta, error) {
		return "", CredMeta{}, Die("keychain item %q unreadable", KeychainService)
	})
	r.at(12 * time.Minute)

	if n := r.run(t); n != 0 {
		t.Fatalf("a locked keychain past the budget must skip: %d\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "— skipped") || !strings.Contains(r.out(), "keychain") {
		t.Errorf("the skip must name the keychain, got:\n%s", r.out())
	}
	if calls := bdCalls(t, r.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a blind park must not claim the bead, got: %s", calls)
	}
}

// --dry-run still applies the per-bead guard after routing.
func TestBlindOverBudgetDryRunStillSkips(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.d.DryRun = true
	r.blind()
	r.at(12 * time.Minute)

	if n := r.run(t); n != 0 {
		t.Fatalf("a dry-run unattended pass past the budget must skip: %d\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "— skipped") {
		t.Errorf("want a blind skip, got:\n%s", r.out())
	}
	if calls := bdCalls(t, r.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a blind dry-run must not claim the bead, got: %s", calls)
	}
}

func TestPlanGuardBlindMaxParse(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	var errb strings.Builder
	if got := a.PlanGuardBlindMax(&errb); got != PlanGuardBlindMaxDefault {
		t.Errorf("unset: %s, want %s", got, PlanGuardBlindMaxDefault)
	}
	if errb.Len() != 0 {
		t.Errorf("unset is silent: %q", errb.String())
	}

	for _, tc := range []struct {
		raw  string
		want time.Duration
		warn bool
	}{
		{"0", 0, false},
		{"0s", 0, false},
		{"10m", 10 * time.Minute, false},
		{"90", 90 * time.Second, false},
		{"null", PlanGuardBlindMaxDefault, false}, // YamlGet: null/~ are unset
		{"~", PlanGuardBlindMaxDefault, false},
		{"-1", PlanGuardBlindMaxDefault, true},
		{"-1s", PlanGuardBlindMaxDefault, true},
		{"false", PlanGuardBlindMaxDefault, true},
		{"10 minutes", PlanGuardBlindMaxDefault, true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			if err := os.WriteFile(a.ConfigPath, []byte("plan_guard_blind_max: "+tc.raw+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			errb.Reset()
			got := a.PlanGuardBlindMax(&errb)
			if got != tc.want {
				t.Errorf("plan_guard_blind_max %q: %s, want %s", tc.raw, got, tc.want)
			}
			warned := strings.Contains(errb.String(), "not a duration")
			if warned != tc.warn {
				t.Errorf("plan_guard_blind_max %q: warned=%v, want %v (%q)", tc.raw, warned, tc.warn, errb.String())
			}
		})
	}
}
