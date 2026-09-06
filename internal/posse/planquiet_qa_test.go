//go:build posse_arm3

package posse

// QA pins for the quiet meter (ranger-base-4rfw1).
//
// The defect: `posse cockpit` asked the usage endpoint on its own whenever
// it was open. On 2026-09-02 the operator commented out both thresholds and
// restarted the watch to drain a 429 window; the box went quiet for 94
// minutes, and then two cockpit ticks (20:13:57Z and 21:15:39Z, caller
// `cockpit` in $StateDir/plan-usage.log) each drew `429 Retry-After: 3600`
// and re-armed the hour the gap was draining.
//
// So every pin here counts REQUESTS against a fake endpoint, and every one
// of them has an arm where the count is not zero — a "no requests were
// made" assertion over a rig that could not make one is the pin that stays
// green through the fix being deleted.

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// quietRig is one caller's cache over a counting endpoint, in a home whose
// config this test writes. The App is real: `plan_usage_quiet:` and the
// thresholds are read the way a live process reads them, which is the half
// a hand-built PlanCache cannot pin.
func quietRig(t *testing.T, cfg string) (*App, *planServer) {
	t.Helper()
	b, _ := newTestBackend(t)
	ps := newPlanServer(t, 42, 61)
	planConfig(t, b.App, "", cfg)
	return b.App, ps
}

// cacheOver is that App's cache for one caller, pointed at the fake
// endpoint — the shipped construction (App.PlanCache), so the quiet
// decision under test is the one a real caller gets.
func cacheOver(a *App, ps *planServer, caller string, now time.Time) *PlanCache {
	c := a.PlanCache(caller)
	c.Reader, c.NoAdapter = ps.reader(), nil
	c.Now = func() time.Time { return now }
	return c
}

// The whole bead, at the choke point: with the meter quiet, not one of the
// three callers gets a request out — and the armed control proves the same
// rig does make them.
func TestQAQuietMeterMakesNoRequestFromAnyCaller(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 20, 13, 57, 0, time.UTC)
	for _, tc := range []struct {
		name string
		cfg  string
		want int64 // requests the three callers may make between them
	}{
		// The state the incident was in: thresholds commented out.
		{"guard off", "", 0},
		// The flag the quiet gap needed — armed guard, quiet meter, so an
		// operator can stop the polling without also switching off the
		// brake.
		{"quiet flag", "plan_guard_5h: 70\nplan_usage_quiet: true", 0},
		// The control. Same rig, same three callers, one request between
		// them (the second and third are cache hits, rangerhq-tdy8).
		{"armed and readable", "plan_guard_5h: 70", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, ps := quietRig(t, tc.cfg)
			for _, caller := range []string{"cockpit", "status", "cost"} {
				cacheOver(a, ps, caller, now).Read(5 * time.Minute)
			}
			if got := ps.hits.Load(); got != tc.want {
				t.Errorf("%s: %d requests, want %d", tc.name, got, tc.want)
			}
		})
	}
}

// Quiet is "do not ask", not "forget": a snapshot the caller would have
// taken as a cache hit anyway is still served, with the reading's own
// timestamp, so the cockpit and `posse cost` keep showing the last number
// anybody got.
func TestQAQuietMeterStillServesAFreshSnapshot(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 20, 13, 57, 0, time.UTC)
	a, ps := quietRig(t, "")
	at := now.Add(-2 * time.Minute)
	cacheOver(a, ps, "seed", now).store(planEntry{At: at, Windows: PlanUsage{{Name: "5h", Pct: 46}}})

	u, readAt, err := cacheOver(a, ps, "cockpit", now).Read(5 * time.Minute)
	if err != nil {
		t.Fatalf("a snapshot inside maxAge is still a reading: %v", err)
	}
	if len(u) != 1 || u[0].Pct != 46 {
		t.Errorf("want the stored reading, got %v", u)
	}
	if !readAt.Equal(at) {
		t.Errorf("the reading's own time, not the tick's: %s, want %s", readAt, at)
	}
	if got := ps.hits.Load(); got != 0 {
		t.Errorf("serving a snapshot cost %d requests", got)
	}
}

// …and PAST that age it refuses rather than hand back a stale number as a
// fresh one. Read's guarantee — nothing older than maxAge — is what every
// caller has built on, and widening it for this flag is how a guard ends up
// ruling on a nineteen-hour-old reading it believes is current
// (ranger-base-c3vqe). Surfaces that want the old reading ask for it by
// name, with its age (LastReading), which is what the cockpit does.
func TestQAQuietMeterRefusesAStaleSnapshotRatherThanServeIt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 20, 13, 57, 0, time.UTC)
	a, ps := quietRig(t, "")
	at := now.Add(-6 * time.Hour)
	cacheOver(a, ps, "seed", now).store(planEntry{At: at, Windows: PlanUsage{{Name: "5h", Pct: 46}}})

	c := cacheOver(a, ps, "cost", now)
	_, _, err := c.Read(5 * time.Minute)
	if q := PlanQuietReason(err); q == nil {
		t.Fatalf("want the quiet refusal, got %v", err)
	}
	if ps.hits.Load() != 0 {
		t.Error("the refusal asked the endpoint anyway")
	}
	if _, gotAt, ok := c.LastReading(); !ok || !gotAt.Equal(at) {
		t.Errorf("the old reading is still there for a surface that asks by name: %v %s", ok, gotAt)
	}
}

// The two causes stay apart, because the operator's next move differs: one
// is a threshold to set, the other a flag to unset.
func TestQAQuietNamesWhichQuietItIs(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 20, 13, 57, 0, time.UTC)
	for _, tc := range []struct {
		name, cfg, why, says string
	}{
		{"guard off", "", "guard off", "plan_guard_"},
		{"flag", "plan_guard_5h: 70\nplan_usage_quiet: true", "meter quiet", "plan_usage_quiet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, ps := quietRig(t, tc.cfg)
			_, _, err := cacheOver(a, ps, "cockpit", now).Read(5 * time.Minute)
			q := PlanQuietReason(err)
			if q == nil {
				t.Fatalf("want a quiet refusal, got %v", err)
			}
			if q.Why() != tc.why {
				t.Errorf("Why() = %q, want %q", q.Why(), tc.why)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("the sentence must name what to change: %q", err)
			}
		})
	}
}

// A quiet a nobody can parse is not a quiet: `plan_usage_quiet: banana`
// leaves the meter readable and says so. The plan guard's rule for a
// malformed setting, and here the visible failure is the safe one — a typo
// that silently muted the shop's only meter is the failure mode this whole
// file is about, arriving by the other door.
func TestQAQuietMalformedFlagLeavesTheMeterReadable(t *testing.T) {
	t.Parallel()
	a, _ := quietRig(t, "plan_guard_5h: 70\nplan_usage_quiet: banana")
	var errb strings.Builder
	if q := a.PlanMeterQuiet(&errb); q != nil {
		t.Errorf("a malformed flag must not mute the meter: %v", q)
	}
	if !strings.Contains(errb.String(), "plan_usage_quiet") || !strings.Contains(errb.String(), "banana") {
		t.Errorf("the typo must be named: %q", errb.String())
	}
	// And `false` is a value, not a typo: it says nothing and mutes nothing.
	a2, _ := quietRig(t, "plan_guard_5h: 70\nplan_usage_quiet: false")
	var errb2 strings.Builder
	if q := a2.PlanMeterQuiet(&errb2); q != nil {
		t.Errorf("false is not quiet: %v", q)
	}
	if errb2.Len() != 0 {
		t.Errorf("false is silent: %q", errb2.String())
	}
}

// A watch pass is the heaviest reader of this endpoint, so a quiet gap it
// does not honour is not a quiet gap. With the flag set and thresholds
// armed the pass makes no request, runs anyway, and says once that the
// guard is OFF — guard-off, not guard-blind: no clock, nothing parked.
func TestQAQuietDispatchPassIsGuardOffAndAsksNothing(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 99, 99) // would skip every pass if it were read
	d, errb := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "plan_guard_5h: 70\nplan_usage_quiet: true")
	idleClaude(t, fake)

	for i := 0; i < 3; i++ {
		n, err := d.Run("", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && n != 1 {
			t.Fatalf("a quiet meter is guard-off, so the pass runs: %d dispatched\n%s", n, dispatcherOut(d))
		}
	}
	if got := ps.hits.Load(); got != 0 {
		t.Errorf("three quiet passes made %d requests", got)
	}
	if !strings.Contains(errb.String(), "the guard is OFF") || !strings.Contains(errb.String(), "plan_usage_quiet") {
		t.Errorf("the pass must say which state it is in: %q", errb.String())
	}
	if got := strings.Count(errb.String(), "the guard is OFF"); got != 1 {
		t.Errorf("a --watch loop must not repeat a configuration fact: said %d times\n%s", got, errb.String())
	}
	if strings.Contains(errb.String(), "guard blind") || strings.Contains(dispatcherOut(d), "park") {
		t.Errorf("quiet is not blindness — no clock, nothing parked: %q\n%s", errb.String(), dispatcherOut(d))
	}
}

// Bead item (3): rwwp6's escalation is not the watch's, it is the
// instance's. The Nth 429 in a storm is usually a different process from
// the first — the measured storm was cockpit, cockpit, pulse, cockpit — so
// a streak each process counted for itself would escalate nowhere. Four
// callers, one storm, and the wait doubles across them.
func TestQAQuiet429EscalationSpansCallers(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	r.ps.status, r.ps.retry = http.StatusTooManyRequests, "3600"
	// The callers the live log named, in the order it named them.
	for i, caller := range []string{"cockpit", "cockpit", "pulse", "dispatch"} {
		r.at(time.Duration(i) * 24 * time.Hour) // well past any cooldown
		r.caller(caller).Read(5 * time.Minute)
		want := time.Duration(1<<uint(i)) * time.Hour
		if want > planCooldownCeiling {
			want = planCooldownCeiling
		}
		e, ok := r.caller(caller).load()
		if !ok {
			t.Fatalf("%s: no snapshot after the 429", caller)
		}
		if e.Wait != want {
			t.Errorf("429 #%d (%s): honoured wait %s, want %s — the escalation is the instance's, not one process's",
				i+1, caller, e.Wait, want)
		}
		if e.Streak != i+1 {
			t.Errorf("429 #%d (%s): streak %d — a per-process streak escalates nowhere", i+1, caller, e.Streak)
		}
	}
	// The control: a caller inside the escalated wait asks nothing at all.
	hits := r.hits()
	r.at(3*24*time.Hour + time.Hour)
	if _, _, err := r.caller("cost").Read(5 * time.Minute); err == nil {
		t.Error("an 8h cooldown one hour in is not a reading")
	}
	if r.hits() != hits {
		t.Error("a fourth caller asked inside the wait the first three bought")
	}
}

// The guard's own path never reaches the cache while the guard is unarmed —
// that has always been true and stays true. This is the pin that the fix
// did not move the check INTO the cache and leave the guard reading config
// twice: an unarmed guard is quiet at the choke point, whoever holds it.
func TestQAQuietIsDecidedAtTheCacheNotTheCaller(t *testing.T) {
	t.Parallel()
	a, ps := quietRig(t, "")
	if c := a.PlanCache("anybody"); c.Quiet == nil {
		t.Fatal("an unarmed guard must build a quiet cache")
	}
	// Even a caller that overrides the reader — which every test rig and
	// the dispatcher do — gets the refusal.
	c := a.PlanCache("anybody")
	c.Reader, c.NoAdapter = ps.reader(), nil
	if _, _, err := c.Read(0); PlanQuietReason(err) == nil {
		t.Errorf("maxAge 0 is 'fresh only' and must not outrank quiet: %v", err)
	}
	if ps.hits.Load() != 0 {
		t.Error("a fresh-only read asked a quiet endpoint")
	}
}

// The loud stale line (ranger-base-lpoui) says the shop is "ruling on it
// under the headroom rule". While the meter is quiet nothing is ruling on
// anything — the guard is off for the duration — so the line would name a
// rule that is not running, and a nag nobody can act on is how the header's
// own blind line became furniture.
//
// The control is the same snapshot at the same age with the flag off: the
// line fires, loudly, exactly as it did before this bead.
func TestQAQuietSilencesTheLoudStaleLine(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 20, 13, 57, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		cfg   string
		stale bool
	}{
		{"quiet", "plan_guard_5h: 70\nplan_usage_quiet: true\nplan_usage_stale_after: 2h", false},
		{"guard off", "plan_usage_stale_after: 2h", false},
		{"armed and asking", "plan_guard_5h: 70\nplan_usage_stale_after: 2h", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, ps := quietRig(t, tc.cfg)
			cacheOver(a, ps, "seed", now).store(planEntry{
				At: now.Add(-10 * time.Hour), Windows: PlanUsage{{Name: "5h", Pct: 46}},
			})
			if got := a.PlanStaleness("status", now, io.Discard).Stale; got != tc.stale {
				t.Errorf("%s: stale = %v, want %v", tc.name, got, tc.stale)
			}
		})
	}
}
