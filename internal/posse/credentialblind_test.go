package posse

// A 401 is a credential condition, not blind weather (bead rangerhq-ytyj).
//
// Three layers, three pins: the adapter classes the auth statuses
// (planusage.go's AuthFailure), every line the guard prints names the class
// rather than burying it in blind-time accounting, and the governance row
// the pulse fingerprints carries its own key so the coordinator is told
// what it is — while the blind WINDOW itself stays the one rangerhq-e1n
// pinned, byte for byte (ADR 0018 §2: no policy fork by failure class).

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The adapter's four outcomes, and the two that are NOT this class. The
// 500 and the 429 arms are the control: a fork that swallowed every
// non-200 into the credential class would pass the first two rows alone.
func TestPlanReaderAuthFailureClasses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status int
		auth   bool // want an *AuthFailure
		rate   bool // want a *RateLimit — the class that already existed
		stale  bool // want Stale() — an interactive refresh is the move
		says   string
		never  string
	}{
		{name: "401", status: http.StatusUnauthorized, auth: true, stale: true,
			says: "credential stale — run `claude` once to refresh"},
		// MUST NOT say refresh: a setup token is not entitled to plan
		// windows and refreshing it produces the same 403 forever (ADR
		// 0019 D2 as amended, measured on ranger-base-0qp).
		{name: "403", status: http.StatusForbidden, auth: true,
			says: "not entitled to plan windows", never: "refresh"},
		{name: "500 is availability", status: http.StatusInternalServerError,
			says: "usage endpoint returned 500", never: "credential"},
		{name: "429 stays a rate limit", status: http.StatusTooManyRequests, rate: true,
			says: "usage endpoint returned 429", never: "credential"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ps := newPlanServer(t, 1, 2)
			ps.status = tc.status
			_, err := ps.reader().Read()
			if err == nil {
				t.Fatal("a non-200 is never a reading")
			}
			af := AuthFailureReason(err)
			if (af != nil) != tc.auth {
				t.Fatalf("AuthFailureReason(%v) = %+v, want auth=%v", err, af, tc.auth)
			}
			if af != nil && af.Stale() != tc.stale {
				t.Errorf("%d: Stale() = %v, want %v — 401 and only 401 is a freshness problem", tc.status, af.Stale(), tc.stale)
			}
			if af != nil && af.Status == "" {
				t.Error("the status line is what the diagnostic quotes; it must survive the class")
			}
			var rl *RateLimit
			if errors.As(err, &rl) != tc.rate {
				t.Errorf("%d: rate-limited=%v, want %v — the two classes must not overlap: %#v",
					tc.status, !tc.rate, tc.rate, err)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("%d: %q must say %q", tc.status, err.Error(), tc.says)
			}
			if tc.never != "" && strings.Contains(err.Error(), tc.never) {
				t.Errorf("%d: %q must not say %q", tc.status, err.Error(), tc.never)
			}
			if strings.Contains(err.Error(), fakeToken) {
				t.Error("the access token reached the error text")
			}
		})
	}
}

// The dispatch surface, which is where the operator met this: the 2026-08-22
// line said `plan guard: blind 40m (usage endpoint returned 401
// Unauthorized) — pass skipped` and buried a credential condition inside
// blind-time accounting. Same park, same clock, and now the line says which
// of the two states it is.
func TestBlindSkipOn401NamesTheCredentialCondition(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.ps.status = http.StatusUnauthorized
	r.ps.body = "unauthorized"

	r.at(12 * time.Minute)
	if n := r.run(t); n != 0 {
		t.Fatalf("12m blind on 401 must park exactly as any other blind read: %d\n%s", n, r.out())
	}
	out := r.out()
	if !strings.Contains(out, "— skipped") {
		t.Fatalf("a parked pass must say why:\n%s", out)
	}
	for _, want := range []string{"blind 12m", "401", "credential stale — run `claude` once to refresh"} {
		if !strings.Contains(out, want) {
			t.Errorf("the park line must carry %q, got:\n%s", want, out)
		}
	}
}

// Requirement 3, and the one that must not have moved: the blind window is
// still rangerhq-e1n's. A credential failure buys no extra tolerance and
// costs none — under the budget the pass RUNS, with the class named on
// stderr, because `claude` refreshing its own token on the next launch is a
// real self-heal and quiet tolerance is exactly what it is for.
func TestBlindUnderBudgetOn401StillRuns(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.ps.status = http.StatusUnauthorized
	r.ps.body = "unauthorized"

	r.at(9 * time.Minute)
	if n := r.run(t); n != 1 {
		t.Fatalf("9m blind is inside the 10m budget, 401 or not: %d dispatched\n%s", n, r.out())
	}
	if !strings.Contains(r.err(), "credential stale") {
		t.Errorf("the fail-open stderr line names the class too, got %q", r.err())
	}
	if strings.Contains(r.out(), "skipped") {
		t.Errorf("a class must never move the park boundary:\n%s", r.out())
	}
}

// ─── the governance row, which is what the pulse fingerprints ────────────────

// G5 stays G5 — ADR 0029's table is closed at nine — but a credential
// failure is a different INSTANCE of it and says so in the key. The last
// two rows are the control: an availability failure keeps the old key, so
// this pin fails if the fork ever collapses in either direction.
func TestGovG5CredentialBlindGetsItsOwnKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		key  string
		says string
	}{
		{"401", &AuthFailure{Status: "401 Unauthorized", Code: http.StatusUnauthorized}, "guard-credential:401",
			"credential stale — run `claude` once to refresh"},
		{"403", &AuthFailure{Status: "403 Forbidden", Code: http.StatusForbidden}, "guard-credential:403",
			"not entitled to plan windows"},
		// Not a credential condition, so still the guard-blind key — which
		// since ranger-base-lpoui carries the blind stretch's hour bucket
		// (45m is its zeroth hour) and, where the failure has one, its
		// class. The last row is the control for the class half: an
		// unclassed error appends no token rather than inventing one.
		{"429", &RateLimit{Status: "429 Too Many Requests"}, "guard-blind:0h:429", "monitoring itself is broken"},
		{"unreachable", Die("usage endpoint unreachable"), "guard-blind:0h", "monitoring itself is broken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			appendConfig(t, b.App, govGuardCfg+"plan_guard_blind_max: 10m\n")
			seedPlanSnapshot(t, b.App, govNow.Add(-45*time.Minute))
			in := govIn(t, b)
			in.Plan = &fakePlanReader{err: tc.err}

			g := find(shopSet(t, in), "G5")
			if g == nil || g.Key != tc.key || g.Class != GovUrgent {
				t.Fatalf("G5 = %+v, want %s URGENT", g, tc.key)
			}
			if !strings.Contains(g.Detail, tc.says) {
				t.Errorf("the row's line must say %q, got %q", tc.says, g.Detail)
			}
			if !strings.Contains(g.Detail, "blind 45m") {
				t.Errorf("the row still carries the blind age: %q", g.Detail)
			}
		})
	}
}

// Inside the budget there is no row at all, credential or not: the row is
// the guard's park made visible, and raising one while the shop still
// dispatches would make URGENT — "the shop is stopped" — a lie.
func TestGovCredentialBlindInsideTheBudgetIsNotACondition(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, govGuardCfg+"plan_guard_blind_max: 1h\n")
	seedPlanSnapshot(t, b.App, govNow.Add(-5*time.Minute))
	in := govIn(t, b)
	in.Plan = &fakePlanReader{err: &AuthFailure{Status: "401 Unauthorized", Code: http.StatusUnauthorized}}
	if g := find(shopSet(t, in), "G5"); g != nil {
		t.Errorf("5m blind against a 1h budget is quiet tolerance: %+v", *g)
	}
}

// Delivery, which is the half the bead asked for: ONE pulse to the
// coordinator naming the credential condition, in minutes rather than on
// the operator's next log read — and one only, because the fingerprint
// dedup that bounds every other condition bounds this one too.
func TestPulseDeliversTheCredentialConditionOnce(t *testing.T) {
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pane := id + ":p1"
	govRepo(t, b) // a fake bd over an empty queue: G5 is the only row
	appendConfig(t, b.App, govGuardCfg+"plan_guard_blind_max: 10m\n")

	clock := govNow
	seedPlanSnapshot(t, b.App, clock.Add(-45*time.Minute))
	d := deliveryDispatcher(t, b, &clock)
	d.Plan = &fakePlanReader{err: &AuthFailure{Status: "401 Unauthorized", Code: http.StatusUnauthorized}}
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute, RenagMax: 4 * time.Hour}

	d.pulseOnce(cfg)

	log := calls(t, fake)
	if !strings.Contains(log, "agent prompt "+pane+" Pulse check:") {
		t.Fatalf("a parked-blind fleet must reach the coordinator:\n%s\n%s", dispatcherOut(d), log)
	}
	if !strings.Contains(log, "guard-credential:401") {
		t.Errorf("the prompt carries KEYS, so the key is where the class has to be:\n%s", log)
	}
	if strings.Contains(log, "guard-blind") {
		t.Errorf("a credential condition delivered as weather is the whole bug:\n%s", log)
	}

	// No storm: the same condition on the next tick is inside renag.
	clock = clock.Add(2 * time.Minute)
	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
		t.Errorf("want exactly one pulse for one blind stretch, got %d:\n%s", n, calls(t, fake))
	}
}
