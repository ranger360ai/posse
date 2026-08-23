package rhq

// The plan guard's blind window (rangerhq-6h1, the business manager's
// ruling on rangerhq-30m): fail-open keeps a grace period, then stops — but
// only where nobody is watching. Hermetic: a fake usage endpoint, a fake
// keychain, and an injected clock, so nothing here sleeps ten minutes or
// touches the operator's credentials.

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	u := s.URL
	s.Close()
	return u
}

// blindRig: a dispatcher with a ready bead, a clock the test drives, and a
// plan reader that can be blinded and restored between passes. Returns the
// dispatcher, its stderr, and the two knobs (clock, blind switch).
type blindRig struct {
	d     *Dispatcher
	errb  *strings.Builder
	fake  string
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

	r := &blindRig{d: d, errb: errb, fake: fake, ps: ps, live: ps.URL, dead: deadURL(t), clock: blindT}
	d.Now = func() time.Time { return r.clock }
	d.blindSince = blindT
	return r
}

func (r *blindRig) blind()               { r.d.Plan.URL = r.dead }
func (r *blindRig) sighted()             { r.d.Plan.URL = r.live }
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

// Over the budget, unattended: the pass is skipped, with the blind duration
// in a line the shape of the over-threshold one — and bd is never asked
// anything, so --watch reads it as a quiet pass and backs off.
func TestBlindOverBudgetSkips(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.blind()
	r.at(12 * time.Minute)

	n := r.run(t)
	if n != 0 {
		t.Fatalf("past the blind budget the pass must not dispatch: %d\n%s", n, r.out())
	}
	want := "plan guard: blind 12m (usage endpoint unreachable) — pass skipped"
	if !strings.Contains(r.out(), want) {
		t.Errorf("want %q, got:\n%s", want, r.out())
	}
	if calls := bdCalls(t, r.fake); calls != "" {
		t.Errorf("a blind skip must not call bd at all, got: %s", calls)
	}
	base := 30 * time.Second
	if got := NextInterval(base, base, 8*base, n); got != 2*base {
		t.Errorf("--watch must back off after a blind skip: %s, want %s", got, 2*base)
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
		t.Fatalf("setup: want the blind pass skipped, got %d dispatched", n)
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
	before := strings.Count(r.out(), "pass skipped")
	r.blind()
	r.at(23 * time.Minute)
	r.run(t)
	if got := strings.Count(r.out(), "pass skipped"); got != before {
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

// The skipped half (rangerhq-llse): a pass the guard SKIPS prints nothing
// else at all, so its silence reads as an empty queue — three hours of
// gated passes looked exactly like a loop with no work. Every skipped pass
// names the error that skipped it, with the real blind age. The hourly
// quiet applies to the fail-open note only.
func TestBlindSkipIsNeverSilent(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.blind()

	// The crossing into skipping.
	r.at(12 * time.Minute)
	if n := r.run(t); n != 0 {
		t.Fatalf("12m blind must skip: %d", n)
	}
	want := "plan guard: blind 12m (usage endpoint unreachable) — pass skipped"
	if !strings.Contains(r.out(), want) {
		t.Fatalf("want %q, got:\n%s", want, r.out())
	}

	// The next hour of skipped passes: each one skipped, each one said.
	for i, at := range []time.Duration{20 * time.Minute, 40 * time.Minute, 60 * time.Minute} {
		r.at(at)
		if n := r.run(t); n != 0 {
			t.Errorf("still blind at %s, still skipping: %d dispatched", at, n)
		}
		if got := strings.Count(r.out(), "pass skipped"); got != i+2 {
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
	if passes := r.d.Watch(ctx, "", "", 0, time.Second, time.Second); passes != 1 {
		t.Fatalf("want 1 pass, got %d", passes)
	}
	if !r.d.Unattended {
		t.Error("Watch must mark the loop unattended")
	}
	if !r.d.blindSince.Equal(blindT) {
		t.Errorf("blind clock seeded at loop start: %s, want %s", r.d.blindSince, blindT)
	}
	if strings.Contains(r.out(), "pass skipped") {
		t.Errorf("a fresh loop's first blind pass gets the full grace:\n%s", r.out())
	}
}

func TestBlindFor(t *testing.T) {
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
