package posse

// ADR 0019 D3 (ranger-base-vmqg): a (runtime, purpose, platform) with no
// credential store is UNCONFIGURED, and unconfigured is guard-OFF — never
// blind.
//
// The distinction is not cosmetic. Blindness has a clock on it (ADR 0018),
// and past `plan_guard_blind_max:` an unattended pass parks every on-meter
// bead until a reading succeeds. On a machine that has no store at all, no
// reading is ever coming: reported as blindness, structural absence is a
// brake with no release. So the tests here all ask the same two questions of
// every arrival — did the pass still dispatch, and did the clock stay
// stopped — and then ask whether the one line an operator gets names the
// platform, the store and the command.
//
// Nothing here reads the operator's home or their credentials: the absence
// is the fixture, and *NoSource is a value.

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// linuxNoSource is what the non-darwin adapter hands back on a box where
// `claude` has never logged in — the store named, the fix named, and no
// mention of a keychain (ADR 0019 V2). Written out rather than produced by
// credentialsFileStore so the path in it is nobody's real home.
func linuxNoSource() *NoSource {
	return &NoSource{
		Runtime: "claude", Purpose: CredMeter, GOOS: "linux",
		Store: "the Claude Code credentials file /home/op/.claude/.credentials.json",
		Arm:   "log in once with `claude` — its own login loop writes that file and posse reads it there",
	}
}

// noSourceAdapter installs the shipped adapter as it behaves on a platform
// that holds no credential for it: available in the binary, unavailable on
// this machine, and unavailable for a reason with a type.
//
// It is the counterpart of planseam_test.go's noAdapters, and the pair is
// the point: two adapters that both refuse, one whose refusal a login fixes
// and one whose does not, and a guard that says which is which.
//
// Its callers must be SERIAL: this replaces the package-level slice
// planAdapters, which PlanAdapter ranges over for every other test in the
// binary (ranger-base-btdvw).
func noSourceAdapter(t *testing.T, ns *NoSource) {
	t.Helper()
	saved := planAdapters
	planAdapters = []planAdapter{{
		Name:        "anthropic",
		Unavailable: func() error { return ns },
		New:         func() PlanReader { t.Fatal("an unavailable adapter must not be built"); return nil },
	}}
	t.Cleanup(func() { planAdapters = saved })
}

// noSourceRig is the guard armed, unattended, and three hours past a ten
// minute blind budget — the state in which blindness parks the fleet. If
// anything here treats structural absence as blindness, this pass parks.
func noSourceRig(t *testing.T) (*Dispatcher, *strings.Builder) {
	t.Helper()
	b, fake := newTestBackend(t)
	d, errb := planDispatcher(t, b, nil)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, guardOn+"\nplan_guard_blind_max: 10m")
	idleClaude(t, fake)
	d.Unattended = true
	d.Now = func() time.Time { return blindT.Add(3 * time.Hour) }
	d.blindSince = blindT
	return d, errb
}

// assertUnconfigured is the whole invariant, asked of one pass: the bead
// went out, the clock never started, and the line the operator gets is the
// actionable one.
func assertUnconfigured(t *testing.T, d *Dispatcher, errb *strings.Builder, n int, ns *NoSource) {
	t.Helper()
	if n != 1 {
		t.Fatalf("structural absence must never park a pass: %d dispatched\n%s", n, dispatcherOut(d))
	}
	out, errs := dispatcherOut(d), errb.String()
	if strings.Contains(out, "skipped") || strings.Contains(out, "blind") {
		t.Errorf("no credential source must not park or degrade a bead:\n%s", out)
	}
	// The blind window's own two lines are what a running clock looks like.
	if strings.Contains(errs, "pass not gated") || strings.Contains(errs, "guard blind") {
		t.Errorf("no credential source must not start the blind clock: %q", errs)
	}
	// ...and its two pieces of state, which is what the park reads.
	if d.blindFailed {
		t.Error("blindFailed set: the guard recorded a failed reading where there was no reading to fail")
	}
	if d.planBlind != "" {
		t.Errorf("planBlind set to %q — that string is the per-bead park reason", d.planBlind)
	}
	if !strings.Contains(errs, "the guard is UNCONFIGURED") {
		t.Errorf("the operator armed a guard with no credential and must be told which state that is: %q", errs)
	}
	for _, want := range []string{ns.GOOS, ns.Store, ns.Arm} {
		if !strings.Contains(errs, want) {
			t.Errorf("the witness line must name the platform, the store and what would arm it — missing %q in %q", want, errs)
		}
	}
	// "No adapter" is the other guard-off state and sends an operator
	// looking for a missing feature instead of running one command.
	if strings.Contains(errs, "no plan-window adapter serves this machine") {
		t.Errorf("a missing credential must not be reported as a missing adapter: %q", errs)
	}
}

// The control: the same rig, unattended and three hours past a ten minute
// blind budget, with an ORDINARY read failure instead of a NoSource — the
// same shape TestGovNoCredentialSourceIsNotGuardBlind carries for G5, but for
// the dispatch surface. This is what noSourceRig is supposed to do when
// nothing about the absence is structural: park. Without this test, all four
// NoSource pins above are satisfied just as well by a rig that can never
// park in the first place, and a change to noSourceRig, planDispatcher or
// the blind budget that broke parking would leave every one of them green.
func TestNoSourceRigWithAnOrdinaryFailureDoesPark(t *testing.T) {
	d, _ := noSourceRig(t)
	d.Plan = &fakePlanReader{err: Die("usage endpoint: 500")}

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("an ordinary blind read past budget must park the pass: %d dispatched\n%s", n, dispatcherOut(d))
	}
	if d.planBlind == "" {
		t.Error("planBlind unset: an ordinary read failure past the blind budget must set the per-bead park reason")
	}
	if !d.blindFailed {
		t.Error("blindFailed unset: an ordinary read failure must be recorded as a failed reading")
	}
}

// Arrival one: the availability check caught it, so no reader was ever
// built. This is the shape of every real linux box that has not logged in.
func TestPlanGuardWithNoCredentialSourceIsUnconfiguredNotBlind(t *testing.T) {
	ns := linuxNoSource()
	noSourceAdapter(t, ns)
	d, errb := noSourceRig(t)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	assertUnconfigured(t, d, errb, n, ns)
}

// Arrival two: the READ found no store — the check passed and the file was
// gone by the time the token was wanted, or a caller supplied the reader and
// no check was ever made. Same platform, same store, same fix, so the guard
// must give the same answer; deciding a fleet's fate on which of two code
// paths noticed is exactly the race this outcome class exists to remove.
func TestPlanGuardNoSourceFromTheReadIsUnconfiguredNotBlind(t *testing.T) {
	ns := linuxNoSource()
	d, errb := noSourceRig(t)
	d.Plan = &fakePlanReader{err: ns}

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	assertUnconfigured(t, d, errb, n, ns)
}

// A NoSource wrapped on its way up is still a NoSource: the guard reads the
// type, and a caller that adds context to an error must not change what the
// fleet does.
func TestPlanGuardWrappedNoSourceIsStillUnconfigured(t *testing.T) {
	ns := linuxNoSource()
	d, errb := noSourceRig(t)
	d.Plan = &fakePlanReader{err: fmt.Errorf("reading the meter: %w", ns)}

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	assertUnconfigured(t, d, errb, n, ns)
}

// ...and said exactly once, however many passes a --watch loop makes. A
// configuration fact repeated every pass is a log nobody reads.
func TestPlanGuardUnconfiguredIsSaidOncePerProcess(t *testing.T) {
	noSourceAdapter(t, linuxNoSource())
	d, errb := noSourceRig(t)

	for i := 0; i < 3; i++ {
		if _, err := d.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Count(errb.String(), "the guard is UNCONFIGURED"); got != 1 {
		t.Errorf("a --watch loop must not repeat a configuration fact: said %d times\n%s", got, errb.String())
	}
}

// codex and grok have no usage-endpoint adapter, so their meter credential
// is structurally absent everywhere (ADR 0012 D4 / ADR 0019 D6) — and the
// error the seam really produces for them renders as guard-off, not as an
// outage. The value under test comes out of MeterToken rather than a
// literal, so a change to what the seam returns lands here.
func TestCodexAndGrokMeterReadsAreNoSourceAndRenderGuardOff(t *testing.T) {
	for _, rt := range []string{"codex", "grok"} {
		t.Run(rt, func(t *testing.T) {
			tok, _, err := MeterToken(rt)()
			if tok != "" {
				t.Error("a runtime with no meter store must hand back no token")
			}
			ns := NoSourceReason(err)
			if ns == nil {
				t.Fatalf("want a *NoSource for %s's meter, got %T: %v", rt, err, err)
			}
			if ns.Runtime != rt || ns.Purpose != CredMeter {
				t.Errorf("the absence must name what is absent: %+v", *ns)
			}
			if ns.Arm == "" {
				t.Error("a witness with nothing to arm is a wall; ADR 0019 D3 wants the operator's next move")
			}

			d, errb := noSourceRig(t)
			d.Plan = &fakePlanReader{err: err}
			n, runErr := d.Run("", "", 0)
			if runErr != nil {
				t.Fatal(runErr)
			}
			if n != 1 {
				t.Fatalf("a runtime posse cannot meter must not park a pass: %d dispatched\n%s", n, dispatcherOut(d))
			}
			errs := errb.String()
			if !strings.Contains(errs, "the guard is UNCONFIGURED") || strings.Contains(errs, "guard blind") {
				t.Errorf("want guard-off with a witness, got %q", errs)
			}
		})
	}
}

// NoSourceReason is the single reader of both arrivals, and the table is
// where its rule is stated: structural absence is the answer only when it is
// the WHOLE answer.
func TestNoSourceReasonReadsBothArrivalsAndRefusesTheMixedOne(t *testing.T) {
	t.Parallel()
	ns := linuxNoSource()
	other := Die("its endpoint is not reachable from this network")
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"bare", ns, true},
		{"wrapped", fmt.Errorf("reading the meter: %w", ns), true},
		{"ordinary failure", Die("usage endpoint unreachable"), false},
		// The availability check's arrival: PlanAdapter flattens the reason
		// into a sentence AND keeps it as a value, and this is why.
		{"sole reason of a NoPlanAdapter", &NoPlanAdapter{Why: "no adapter serves this machine", Errs: []error{ns}}, true},
		// Two adapters, and a login only fixes one of them: the guard is
		// still off for something no login touches, so the generic sentence
		// is the honest one.
		{"one reason of several", &NoPlanAdapter{Why: "no adapter serves this machine", Errs: []error{ns, other}}, false},
		{"no reasons at all", &NoPlanAdapter{Why: "no plan-window adapter is compiled in"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NoSourceReason(c.err) != nil; got != c.want {
				t.Errorf("NoSourceReason(%v) structural=%v, want %v", c.err, got, c.want)
			}
		})
	}
}

// PlanAdapter keeps the adapters' reasons as values, not only as a
// substring of Why. Without that, the availability arrival is unreadable and
// the guard's answer depends on which code path noticed the absence.
// Serial: noSourceAdapter replaces the planAdapters slice (ranger-base-btdvw).
func TestPlanAdapterKeepsTheTypedReason(t *testing.T) {
	ns := linuxNoSource()
	noSourceAdapter(t, ns)

	r, err := PlanAdapter()
	if r != nil {
		t.Fatal("want no reader")
	}
	var na *NoPlanAdapter
	if !errors.As(err, &na) {
		t.Fatalf("want *NoPlanAdapter, got %T", err)
	}
	if got := NoSourceReason(err); got != ns {
		t.Errorf("the adapter's own reason must survive as a value, got %v", got)
	}
	// It still reads as one sentence for anyone who only prints it.
	if !strings.Contains(err.Error(), ns.Store) {
		t.Errorf("the flattened sentence must still name the store: %q", err)
	}
}

// The governance surface reads the same fact and must reach the same
// verdict: G5 is "monitoring itself is broken", and a machine that was never
// logged in has nothing broken about it. The snapshot here is stale by 45
// minutes against a 10 minute budget, so a guard that read this as blindness
// would raise an URGENT.
func TestGovNoCredentialSourceIsNotGuardBlind(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, govGuardCfg+"plan_guard_blind_max: 10m\n")
	seedPlanSnapshot(t, b.App, govNow.Add(-45*time.Minute))
	in := govIn(t, b)
	in.Plan = &fakePlanReader{err: linuxNoSource()}

	if g := find(shopSet(t, in), "G5"); g != nil {
		t.Errorf("a platform with no credential store is not a broken monitor: %+v", *g)
	}

	// The control: the same rig with an ordinary read failure DOES raise it,
	// so the absence above is this test's subject and not its fixture.
	in.Plan = &fakePlanReader{err: Die("usage endpoint: 429")}
	if g := find(shopSet(t, in), "G5"); g == nil {
		t.Error("an ordinary blind read past its budget must still raise G5")
	}
}
