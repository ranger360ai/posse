package posse

// Hermetic tests for what a plan guard verdict does to one bead (ADR 0010
// §1, ADR 0013 §3). Same substrate as the guard's own tests: a fake usage
// endpoint and a fake keychain (planusage_test.go), the test binary
// re-execing as fake herdr and fake bd. Nothing here launches a real codex
// or grok — the launch that is asserted is the command herdr was asked to
// type.
//
// The shape these pin is the one the automatic overflow used to complicate:
// a trip and a blind read both PARK the beads that would spend the meter
// the guard read, both let every other bead launch, and neither one changes
// any bead's runtime. Where paid work continues past a trip, an operator
// said so — `runtime:` on the PID, or `--runtime` on the pass.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parkPID is the default persona for these tests: nothing declared but the
// label routing, so parity is clean on every runtime and the bead's own
// labels decide the tier.
const parkPID = "---\nname: ranger\ndescription: test\nlabels: [go]\n---\nYou are ranger.\n"

type parkFixture struct {
	d    *Dispatcher
	errb *strings.Builder
	b    *HerdrBackend
	fake string
	repo string
	ps   *planServer
}

// trippedPass wires one pass whose 5h reading is 78% against a 70%
// threshold — tripped — with extra config lines and a persona of the
// caller's choosing. beadLabels is the ready bead's label list.
func trippedPass(t *testing.T, cfg, pid, beadLabels string) *parkFixture {
	t.Helper()
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 78, 40)
	d, errb := planDispatcher(t, b, ps)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":`+beadLabels+`}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "plan_guard_5h: 70\n"+cfg)
	idleClaude(t, fake)
	return &parkFixture{d: d, errb: errb, b: b, fake: fake, repo: repo, ps: ps}
}

// The rule, on the runtime the guard meters: a tripped guard parks the bead
// on the trip's own reason and claims nothing. Silent on stderr — a trip is
// an outcome, not a misconfiguration.
func TestTripParksOnMeterBead(t *testing.T) {
	t.Parallel()
	f := trippedPass(t, "", parkPID, `["go","tier:standard"]`)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "plan 5h at 78% > 70% — skipped") {
		t.Fatalf("want the on-meter bead parked, got n=%d:\n%s", n, out)
	}
	if calls := bdCalls(t, f.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a parked bead must not be claimed, got: %s", calls)
	}
	if f.errb.Len() != 0 {
		t.Errorf("a trip is silent on stderr: %q", f.errb.String())
	}
}

// The other half of the same verdict (ADR 0013 §3): a lane whose PID names a
// runtime that is not on the guarded meter LAUNCHES through the trip, on its
// own runtime, because the reading says nothing about a pool it did not
// read. This is the whole of what a trip does — park one, run the other —
// and no configuration is needed to get it.
func TestTripLetsOffMeterBeadLaunch(t *testing.T) {
	t.Parallel()
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: grok\n---\nYou are ranger.\n"
	f := trippedPass(t, "", pid, `["go","tier:standard"]`)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("a runtime off the guarded meter must not be skipped by it, got n=%d:\n%s", n, out)
	}
	if got := delivered(t, f.b.App, f.fake); !strings.Contains(got, "runtime/tier: grok/standard") {
		t.Errorf("the launch must still be grok's:\n%s", got)
	}
}

// ADR 0010 §1's removal, pinned as the absence it is: a tripped pass does
// not substitute a provider. The bead's PID names no runtime, so the only
// runtime it may launch on is the default — and on a trip it does not launch
// at all. Nothing in the pass may name a second pool, and the one runtime
// this box has a second pool for (grok) must be absent from every herdr
// call, because a session created there is the substitution itself.
func TestTripChoosesNoProviderForTheBead(t *testing.T) {
	t.Parallel()
	f := trippedPass(t, grokPoolCfg+"grok_guard_week: 70\n", parkPID, `["go","tier:standard"]`)

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 {
		t.Fatalf("no bead may launch: the only eligible one is on the tripped meter, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "overflow") || strings.Contains(f.errb.String(), "overflow") {
		t.Errorf("nothing offers a second pool:\nout: %s\nerr: %s", out, f.errb.String())
	}
	if log := calls(t, f.fake); strings.Contains(log, "GATES grok ") || strings.Contains(log, "workspace create") {
		t.Errorf("a parked bead creates no session anywhere, least of all on a second pool:\n%s", log)
	}
	// And nothing wrote the ledger the removed mechanism kept.
	if _, err := os.Stat(filepath.Join(f.b.App.StateDir, "overflow.log")); !os.IsNotExist(err) {
		t.Errorf("$StateDir/overflow.log is no longer written (%v)", err)
	}
}

// Explicit operator runtime choice remains, and it is judged on its own
// meter rather than waved through: `--runtime grok` is the operator saying
// where this pass runs, so the trip on claude's meter does not touch it;
// `--runtime claude` is the operator saying to spend the meter that just
// tripped, and that bead parks like any other on-meter bead.
func TestExplicitRuntimeIsTheOperatorsChoice(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		runtime string
		want    int
	}{
		{"grok", 1},
		{"claude", 0},
	} {
		t.Run(tc.runtime, func(t *testing.T) {
			t.Parallel()
			f := trippedPass(t, "", parkPID, `["go","tier:standard"]`)
			f.d.Runtime = tc.runtime

			n, _ := f.d.Run("", "", 0)
			out := dispatcherOut(f.d)
			if n != tc.want {
				t.Fatalf("--runtime %s: n=%d, want %d:\n%s", tc.runtime, n, tc.want, out)
			}
			if tc.want == 0 && !strings.Contains(out, "plan 5h at 78% > 70% — skipped") {
				t.Errorf("--runtime %s spends the tripped meter, so it parks on the trip:\n%s", tc.runtime, out)
			}
		})
	}
}

// The runtime the meter question is asked about is the runtime this launch
// would actually SPEND, and for a session that already exists that is the
// one it was created with — launchSession prompts a live session as it
// stands and only ever uses the resolved runtime to CREATE one. So a PID
// that says `runtime: grok` over a session already running on claude parks
// on a claude trip: reading the PID alone would wave through the exact work
// the guard is holding back.
func TestTripAsksAboutTheRuntimeTheLaunchWouldSpend(t *testing.T) {
	t.Parallel()
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: grok\n---\nYou are ranger.\n"
	f := trippedPass(t, "", pid, `["go","tier:standard"]`)
	session := SessionForBead("ranger", f.repo, "a-1")
	if err := f.b.CreateSession(NewSessionOpts{
		Name: session, Dir: f.repo, Agent: "ranger", Runtime: GuardedRuntime, Tier: TierStandard,
	}); err != nil {
		t.Fatal(err)
	}

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "plan 5h at 78% > 70% — skipped") {
		t.Fatalf("a live session on the guarded meter parks whatever the PID now resolves to; n=%d:\n%s", n, out)
	}
	if calls := bdCalls(t, f.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a parked bead must not be claimed, got: %s", calls)
	}
}

// The guard does not trip: nothing happens at all — no extra line, nothing
// on stderr, and the bead launches on its own runtime.
func TestUntrippedGuardChangesNothing(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 42, 40) // under the threshold
	d, errb := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go","tier:standard"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "plan_guard_5h: 70\n")
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 {
		t.Fatalf("an untripped guard changes nothing, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "plan 5h") {
		t.Errorf("an untripped pass says nothing about the window:\n%s", out)
	}
	if errb.Len() != 0 {
		t.Errorf("and nothing on stderr: %q", errb.String())
	}
	if got := delivered(t, b.App, fake); !strings.Contains(got, "runtime/tier: claude/standard") {
		t.Errorf("the launch stays on claude:\n%s", got)
	}
}

// A template-only runtime is UNKNOWN to the meter, and unknown is gated: it
// parks on a trip like any claude bead rather than being waved through as
// "not on the meter". "this runtime is free" is the expensive guess to get
// wrong.
func TestOnGuardedMeter(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"":       true,
		"claude": true,
		"codex":  false,
		"grok":   false,
		"gemini": true, // a template-only runtimes/gemini.yaml — unknown, so gated
	} {
		if got := OnGuardedMeter(name); got != want {
			t.Errorf("OnGuardedMeter(%q) = %v, want %v", name, got, want)
		}
	}
}

// The removed keys (ADR 0010 §1). `plan_guard_` is a prefix every unknown
// suffix is read as a window threshold under, so a stale `plan_guard_overflow:
// grok` left in a config would otherwise arm a guard on a window named
// "overflow" that no provider reports. It must be neither read nor silent:
// the operator who set it believed in a brake they no longer have, and the
// line is where they find that out.
func TestRemovedPlanGuardKeysAreNamedAndNotThresholds(t *testing.T) {
	t.Parallel()
	a := &App{ConfigPath: filepath.Join(t.TempDir(), "config.yaml")}
	if err := os.WriteFile(a.ConfigPath, []byte(
		"plan_guard_5h: 70\nplan_guard_blind_max: 10m\nplan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var errb strings.Builder

	th := a.PlanGuardThresholds(&errb)
	if len(th) != 1 || th["5h"] != 70 {
		t.Fatalf("thresholds = %v; want only the 5h window — a removed key is not a threshold", th)
	}
	for _, key := range []string{"plan_guard_overflow:", "plan_guard_overflow_cap:"} {
		if !strings.Contains(errb.String(), key) {
			t.Errorf("stderr must name %s as no longer read; got:\n%s", key, errb.String())
		}
	}
	if !strings.Contains(errb.String(), "is no longer read") {
		t.Errorf("the line must say the key is not read, not that it is malformed:\n%s", errb.String())
	}
	if strings.Contains(errb.String(), "plan_guard_blind_max") {
		t.Errorf("a reserved key that IS still read stays silent:\n%s", errb.String())
	}
}
