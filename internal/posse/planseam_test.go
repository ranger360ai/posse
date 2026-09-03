package posse

// The plan-window seam (ADR 0012 D4, rangerhq-ys7x): the guard's harness
// half decides passes without knowing what a rate window is called, which
// provider reports one, or whether this machine has an adapter at all.
//
// Every test here runs the SAME guard the shipped adapter runs, against a
// provider that does not exist — two windows named nothing like Anthropic's,
// then no provider at all. What passes here is what makes planusage.go a
// seam rather than a rename.

import (
	"strings"
	"testing"
	"time"
)

// fakePlanReader is a two-window adapter for a provider posse does not
// ship. Its window names share no substring with "5h"/"7d" on purpose: a
// label that leaked into the harness would land in an assertion here.
type fakePlanReader struct {
	windows PlanUsage
	err     error
	reads   int
}

func (f *fakePlanReader) Read() (PlanUsage, error) {
	f.reads++
	if f.err != nil {
		return nil, f.err
	}
	return f.windows, nil
}

// MayShare: true, so the cache behaves exactly as it does for the shipped
// endpoint. A reading nobody vouched for takes a different path (credpin.go
// rule 5) and that path has its own tests.
func (f *fakePlanReader) MayShare() bool { return true }

func fakeWindows(burst, month float64) PlanUsage {
	return PlanUsage{{Name: "burst", Pct: burst}, {Name: "month", Pct: month}}
}

// noAdapters is an instance no shipped adapter can serve: the shipped one
// is there and cannot run here — posse on a platform whose credential store
// nothing reads, which is the real shape of this state and not a
// hypothetical one.
func noAdapters(t *testing.T) {
	t.Helper()
	saved := planAdapters
	planAdapters = []planAdapter{{
		Name:        "anthropic",
		Unavailable: func() error { return Die("its credential store is not on this platform") },
		New:         func() PlanReader { t.Fatal("an unavailable adapter must not be built"); return nil },
	}}
	t.Cleanup(func() { planAdapters = saved })
}

// seamRig is the plan-guard rig with an arbitrary adapter behind it.
func seamRig(t *testing.T, r PlanReader, cfg string) (*Dispatcher, *strings.Builder, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d, errb := planDispatcher(t, b, nil)
	d.Plan = r
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, cfg)
	idleClaude(t, fake)
	return d, errb, fake
}

// The guard trips on a window it has never heard of, names it in the skip
// line, and says nothing about 5h or 7d — because with this adapter
// installed neither exists.
func TestPlanGuardTripsOnAnotherProvidersWindow(t *testing.T) {
	t.Parallel()
	f := &fakePlanReader{windows: fakeWindows(78, 40)}
	d, errb, fake := seamRig(t, f, "plan_guard_burst: 70\nplan_guard_month: 85")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("want 0 dispatched above the burst threshold, got %d\n%s", n, dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if want := "plan burst at 78% > 70% — skipped"; !strings.Contains(out, want) {
		t.Errorf("want %q, got:\n%s", want, out)
	}
	if strings.Contains(out, "5h") || strings.Contains(out, "7d") {
		t.Errorf("the shipped adapter's vocabulary reached a pass it is not serving:\n%s", out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "--claim") {
		t.Errorf("a parked bead must not be claimed, got: %s", calls)
	}
	if errb.Len() != 0 {
		t.Errorf("a working guard says nothing on stderr: %q", errb.String())
	}
}

// Under both of that provider's thresholds the pass runs, and it costs one
// reading — the guard's cadence does not change with its vocabulary.
func TestPlanGuardBelowAnotherProvidersThresholdsRuns(t *testing.T) {
	t.Parallel()
	f := &fakePlanReader{windows: fakeWindows(12, 40)}
	d, errb, _ := seamRig(t, f, "plan_guard_burst: 70\nplan_guard_month: 85")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 dispatched below both thresholds, got %d\n%s", n, dispatcherOut(d))
	}
	if f.reads != 1 {
		t.Errorf("want exactly 1 usage read per pass, got %d", f.reads)
	}
	if errb.Len() != 0 {
		t.Errorf("a working guard says nothing on stderr: %q", errb.String())
	}
}

// Dial E takes whatever the adapter names too (rangerhq-25p): the plan
// window is the tightest, so a standard bead steps down to fast without any
// dollar cap being near.
func TestBudgetStepsDownOnAnotherProvidersWindow(t *testing.T) {
	t.Parallel()
	f := &fakePlanReader{windows: fakeWindows(84, 30)}
	d, _, _ := seamRig(t, f, "plan_guard_burst: 95\nbudget_day: 100")
	d.Spend = func(time.Time) *CostReport { return &CostReport{} }

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	st := d.budget()
	if st.Window != "plan burst" || st.Pct != 84 {
		t.Fatalf("tightest window = %q at %v, want \"plan burst\" at 84", st.Window, st.Pct)
	}
	if !st.StepDown() {
		t.Errorf("84%% of the tightest window is Dial E's step-down rung: %+v", st)
	}
}

// A threshold that names a window this provider does not report gates
// nothing. That is the malformed-threshold failure wearing a different hat,
// so it is said out loud — once — rather than silently ignored.
func TestPlanGuardNamesAThresholdWithNoWindow(t *testing.T) {
	t.Parallel()
	f := &fakePlanReader{windows: fakeWindows(12, 40)}
	d, errb, _ := seamRig(t, f, "plan_guard_burst: 70\nplan_guard_5h: 70")

	if n, err := d.Run("", "", 0); err != nil || n != 1 {
		t.Fatalf("an unmatched threshold gates nothing and must not stop the pass: %d %v\n%s", n, err, dispatcherOut(d))
	}
	want := `plan guard: config plan_guard_5h: this provider reports no window by that name (it reports burst, month) — that threshold gates nothing`
	if !strings.Contains(errb.String(), want) {
		t.Errorf("want %q, got %q", want, errb.String())
	}
	// The one that DOES match is still live, and still silent.
	if strings.Contains(errb.String(), "plan_guard_burst") {
		t.Errorf("a matched threshold must not be reported: %q", errb.String())
	}
}

// The non-window `plan_guard_` settings are not thresholds and must never be
// read as one — a guard armed by `plan_guard_overflow:` alone would fetch a
// reading nobody asked for.
func TestPlanGuardReservedKeysAreNotWindows(t *testing.T) {
	t.Parallel()
	f := &fakePlanReader{windows: fakeWindows(99, 99)}
	d, errb, _ := seamRig(t, f, "plan_guard_blind_max: 10m\nplan_guard_overflow: grok\nplan_guard_overflow_cap: 5")

	if n, err := d.Run("", "", 0); err != nil || n != 1 {
		t.Fatalf("no threshold set means no guard: %d %v\n%s", n, err, dispatcherOut(d))
	}
	if f.reads != 0 {
		t.Errorf("an unarmed guard must not read a meter, got %d reads", f.reads)
	}
	if errb.Len() != 0 {
		t.Errorf("an unarmed guard says nothing: %q", errb.String())
	}
}

// No adapter: the guard is OFF, not blind. The pass runs, one line says why,
// and the blind clock never starts — so an unattended loop hours past
// `plan_guard_blind_max:` still dispatches, because no reading is ever
// coming to release a park.
func TestPlanGuardWithNoAdapterIsOffNotBlind(t *testing.T) {
	noAdapters(t)
	b, fake := newTestBackend(t)
	d, errb := planDispatcher(t, b, nil)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "plan_guard_5h: 70\nplan_guard_7d: 85")
	idleClaude(t, fake)
	d.Unattended = true
	d.Now = func() time.Time { return blindT.Add(3 * time.Hour) }
	d.blindSince = blindT

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("no adapter must never park a pass: %d dispatched\n%s", n, dispatcherOut(d))
	}
	out, errs := dispatcherOut(d), errb.String()
	if strings.Contains(out, "skipped") || strings.Contains(out, "blind") {
		t.Errorf("no adapter must not park or degrade a bead:\n%s", out)
	}
	// The blind window's own two lines — the fail-open note and the park —
	// are what "the clock is running" looks like. Neither may appear.
	if strings.Contains(errs, "pass not gated") || strings.Contains(errs, "guard blind") {
		t.Errorf("no adapter must not start the blind clock: %q", errs)
	}
	if !strings.Contains(errs, "no plan-window adapter serves this machine") ||
		!strings.Contains(errs, "anthropic (its credential store is not on this platform)") ||
		!strings.Contains(errs, "the guard is OFF, not blind") {
		t.Errorf("the operator armed a guard that cannot run and must be told which state that is, and why: %q", errs)
	}
}

// ...and told exactly once, however many passes a --watch loop makes.
func TestPlanGuardNoAdapterIsSaidOncePerProcess(t *testing.T) {
	noAdapters(t)
	b, fake := newTestBackend(t)
	d, errb := planDispatcher(t, b, nil)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "plan_guard_5h: 70")
	idleClaude(t, fake)

	for i := 0; i < 3; i++ {
		if _, err := d.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Count(errb.String(), "the guard is OFF"); got != 1 {
		t.Errorf("a --watch loop must not repeat a configuration fact: said %d times\n%s", got, errb.String())
	}
}

// The cache refuses with the typed reason, so every caller — the header,
// `posse cost --plan`, the guard — can tell "no meter here" from "a meter
// that would not answer". It refuses a SNAPSHOT too: an instance that
// cannot refresh a reading has no business acting on one.
func TestPlanCacheWithNoAdapterRefusesEvenASnapshot(t *testing.T) {
	noAdapters(t)
	b, _ := newTestBackend(t)
	c := b.App.PlanCache("cost")
	c.Path = ""
	// A meter this instance may ask: the subject here is WHICH refusal a
	// missing adapter produces, and a quiet cache (no thresholds in this
	// backend's config, planquiet.go) would refuse one step earlier with a
	// different type — correctly, and not the fact under test.
	c.Quiet = nil
	if _, _, err := c.Read(time.Hour); err == nil {
		t.Fatal("want a refusal with no adapter")
	} else if _, ok := err.(*NoPlanAdapter); !ok {
		t.Errorf("want *NoPlanAdapter so callers can tell it from blindness, got %T: %v", err, err)
	}

	r := newCacheRig(t)
	c = r.caller("cockpit")
	c.Reader, c.NoAdapter = nil, &NoPlanAdapter{Why: "nothing here reads a meter"}
	if _, _, err := c.Read(time.Hour); err == nil {
		t.Error("a snapshot nobody can refresh is not a reading")
	}
}

// PlanAdapter's refusal names the adapter it could not use and why, so an
// operator reading one line knows what would arm the guard.
func TestPlanAdapterRefusalNamesTheAdapterAndTheReason(t *testing.T) {
	saved := planAdapters
	planAdapters = []planAdapter{{
		Name:        "someprovider",
		Unavailable: func() error { return Die("no credential store on this platform") },
		New:         func() PlanReader { t.Fatal("an unavailable adapter must not be built"); return nil },
	}}
	t.Cleanup(func() { planAdapters = saved })

	r, err := PlanAdapter()
	if r != nil {
		t.Fatal("want no reader")
	}
	if !strings.Contains(err.Error(), "someprovider") || !strings.Contains(err.Error(), "no credential store") {
		t.Errorf("want the adapter and its reason, got %q", err)
	}

	// A build with no adapter at all is the same answer with nothing to
	// name — still a *NoPlanAdapter, never a nil the guard could swallow.
	planAdapters = nil
	if r, err := PlanAdapter(); r != nil || err == nil {
		t.Fatalf("empty registry: reader=%v err=%v", r, err)
	} else if _, ok := err.(*NoPlanAdapter); !ok {
		t.Errorf("want *NoPlanAdapter, got %T", err)
	}
}

// A snapshot written before this seam existed carries the old two-field
// shape and decodes to zero windows. That must be a cache MISS: read as a
// reading it is a plan with no limits, which is the one wrong number this
// file could produce.
func TestPlanSnapshotFromBeforeTheSeamIsAMiss(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	c := r.caller("dispatch")
	c.store(planEntry{At: r.clock})
	u, _, err := c.Read(time.Hour)
	if err != nil {
		t.Fatalf("a windowless snapshot must read through to the endpoint: %v", err)
	}
	if win(u, "5h") != 42 {
		t.Errorf("want the fetched reading, got %+v", u)
	}
}

// The rendering is the adapter's vocabulary end to end — nothing between
// the endpoint and the header knows how many windows there are, or what a
// provider calls them.
func TestPlanUsageLineRendersWhateverTheAdapterNames(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		u    PlanUsage
		want string
	}{
		{PlanUsage{{"5h", 42.4}, {"7d", 61.4}}, "5h 42% · 7d 61%"},
		{fakeWindows(78, 40), "burst 78% · month 40%"},
		{PlanUsage{{"rolling", 5}}, "rolling 5%"},
		{nil, "no windows"},
	} {
		if got := tc.u.Line(); got != tc.want {
			t.Errorf("Line(%+v) = %q, want %q", tc.u, got, tc.want)
		}
	}
}
