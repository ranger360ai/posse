package rhq

// Hermetic tests for the plan guard's overflow pool (ADR 0010, rangerhq-r9k).
// Same substrate as the guard's own tests: a fake usage endpoint and a fake
// keychain (planusage_test.go), the test binary re-execing as fake herdr and
// fake bd. Nothing here launches a real codex or grok — the launch that is
// asserted is the command herdr was asked to type, and the eligibility that
// is asserted is the parity check's, which is the same decision dispatch
// makes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// overflowPID is the default persona for these tests: nothing declared but
// the label routing, so parity is clean on every runtime and the bead's own
// labels decide the tier.
const overflowPID = "---\nname: ranger\ndescription: test\nlabels: [go]\n---\nYou are ranger.\n"

type overflowFixture struct {
	d    *Dispatcher
	errb *strings.Builder
	b    *HerdrBackend
	fake string
	repo string
	ps   *planServer
}

// overflowPass wires one pass whose 5h reading is 78% against a 70%
// threshold — tripped — with extra config lines and a persona of the
// caller's choosing. beadLabels is the ready bead's label list.
func overflowPass(t *testing.T, cfg, pid, beadLabels string) *overflowFixture {
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
	return &overflowFixture{d: d, errb: errb, b: b, fake: fake, repo: repo, ps: ps}
}

// ledger reads the overflow log back as lines.
func (f *overflowFixture) ledger(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(f.b.App.OverflowLogPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

// The default, and the behaviour every pass had before this ADR: the guard
// trips, no overflow runtime is configured, the whole pass is skipped on the
// same line — and bd is never asked anything.
func TestOverflowUnsetSkipsWholePass(t *testing.T) {
	f := overflowPass(t, "", overflowPID, `["go","tier:standard"]`)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "plan 5h at 78% > 70%, pass skipped") {
		t.Fatalf("want the whole-pass skip, got n=%d:\n%s", n, out)
	}
	if calls := bdCalls(t, f.fake); calls != "" {
		t.Errorf("a skipped pass must not call bd at all, got: %s", calls)
	}
	if f.errb.Len() != 0 {
		t.Errorf("overflow unset is silent: %q", f.errb.String())
	}
	if l := f.ledger(t); l != nil {
		t.Errorf("nothing may be written to the ledger: %v", l)
	}
}

// §3: the cap is REQUIRED. An overflow runtime without one is overflow off
// — the pass is skipped exactly as before, and the config error is named
// once so it is not a pool that quietly never engages.
func TestOverflowWithoutCapIsOff(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\n", overflowPID, `["go","tier:standard"]`)

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "plan 5h at 78% > 70%, pass skipped") {
		t.Fatalf("no cap must skip the pass, got n=%d:\n%s", n, out)
	}
	lines := strings.Split(strings.TrimRight(f.errb.String(), "\n"), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "plan_guard_overflow_cap:") {
		t.Errorf("want exactly one stderr line naming the missing cap, got: %q", f.errb.String())
	}
	if strings.Contains(out, "overflow") {
		t.Errorf("zero overflow launches, and nothing that reads like one:\n%s", out)
	}
	if l := f.ledger(t); l != nil {
		t.Errorf("no cap, no ledger: %v", l)
	}
}

// The main line: tripped, eligible, cap has room → the bead launches on the
// overflow runtime, the session is created for it, the prompt header says
// so, and one ledger line is written.
func TestOverflowLaunchesEligibleBead(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n",
		overflowPID, `["go","tier:standard"]`)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("an eligible bead must launch on the overflow pool, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "overflow grok, 0/5 in 7d") {
		t.Errorf("the pass must say the pool it is stepping over to:\n%s", out)
	}
	if !strings.Contains(out, "[grok ← overflow]") {
		t.Errorf("the launch line must name the move:\n%s", out)
	}
	// The runtime is per-launch all the way down: the typed command is
	// grok's, and the meta records it.
	log := calls(t, f.fake)
	if !strings.Contains(log, "GATES grok ") {
		t.Errorf("the session was not created on grok:\n%s", log)
	}
	m, ok := f.b.readMeta("ranger-" + filepath.Base(f.repo) + "-a-1")
	if !ok || m.Runtime != "grok" {
		t.Errorf("session meta runtime = %+v, want grok", m)
	}
	// The prompt header the persona reads names the runtime it is on.
	if !strings.Contains(log, "runtime/tier: grok/standard") {
		t.Errorf("the work prompt must name the overflow runtime:\n%s", log)
	}
	l := f.ledger(t)
	if len(l) != 1 {
		t.Fatalf("want exactly one ledger line, got %v", l)
	}
	fields := strings.Fields(l[0])
	if len(fields) != 4 || fields[1] != "grok" || fields[2] != "a-1" || fields[3] != "ranger" {
		t.Errorf("ledger line = %q, want `RFC3339 grok a-1 ranger`", l[0])
	}
	if _, err := time.Parse(time.RFC3339, fields[0]); err != nil {
		t.Errorf("ledger timestamp %q is not RFC3339: %v", fields[0], err)
	}
}

// §2(b): judged work never moves. A strong bead gets the guard's line, per
// bead, and the pool is not touched.
func TestOverflowStrongBeadSkipped(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n",
		overflowPID, `["go","tier:strong"]`)

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "plan 5h at 78% > 70%, overflow grok: strong work stays on claude — skipped") {
		t.Fatalf("strong work must not move, got n=%d:\n%s", n, out)
	}
	if l := f.ledger(t); l != nil {
		t.Errorf("a skipped bead spends nothing: %v", l)
	}
}

// §2(c): the PID's own opt-out, for what a parity check cannot see.
func TestOverflowPIDOptOut(t *testing.T) {
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\noverflow: false\n---\nYou are ranger.\n"
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n",
		pid, `["go","tier:standard"]`)

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "overflow grok: ranger says overflow: false — skipped") {
		t.Fatalf("overflow: false must hold, got n=%d:\n%s", n, out)
	}
	if l := f.ledger(t); l != nil {
		t.Errorf("a skipped bead spends nothing: %v", l)
	}
}

// §2(a): parity decides, with no runtime special-casing. The same seatbelt
// PID is refused on codex (macOS will not nest a sandbox) and admitted on
// grok (it nests) — one PID, one rule, two answers.
func TestOverflowParityDecidesPerTarget(t *testing.T) {
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\ncage: seatbelt\ndeny: [Bash(git push:*)]\n---\nYou are ranger.\n"
	for _, tc := range []struct {
		target   string
		launched bool
		want     string
	}{
		{"codex", false, "does not nest"},
		{"grok", true, "[grok ← overflow]"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			// The host's own sandbox-exec probe must not decide a parity
			// test: pin the cage available, as parity_test.go does.
			had := AvailableCages[CageSeatbelt]
			AvailableCages[CageSeatbelt] = true
			defer func() {
				if !had {
					delete(AvailableCages, CageSeatbelt)
				}
			}()
			f := overflowPass(t, "plan_guard_overflow: "+tc.target+"\nplan_guard_overflow_cap: 5\n",
				pid, `["go","tier:standard"]`)

			n, _ := f.d.Run("", "", 0)
			out := dispatcherOut(f.d)
			if tc.launched != (n == 1) {
				t.Fatalf("%s: launched=%v, want %v:\n%s", tc.target, n == 1, tc.launched, out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("%s: want %q in:\n%s", tc.target, tc.want, out)
			}
			if got := len(f.ledger(t)); (got == 1) != tc.launched {
				t.Errorf("%s: %d ledger lines, launched=%v", tc.target, got, tc.launched)
			}
		})
	}
}

// §3: the cap is a rolling 7 days of beads. Entries inside the window count
// and entries older than it do not, and a reached cap is named in the skip
// line so the operator can see which number stopped the bead.
func TestOverflowCapRolling7d(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 2\n",
		overflowPID, `["go","tier:standard"]`)
	now := time.Now()
	os.MkdirAll(f.b.App.StateDir, 0o755)
	var seed strings.Builder
	// Two inside the window — the cap — plus three that fell out of it, and
	// one for another pool, which is not this pool's week.
	for _, e := range []OverflowEntry{
		{now.Add(-2 * time.Hour), "grok", "old-1", "ranger"},
		{now.Add(-6 * 24 * time.Hour), "grok", "old-2", "ranger"},
		{now.Add(-8 * 24 * time.Hour), "grok", "old-3", "ranger"},
		{now.Add(-30 * 24 * time.Hour), "grok", "old-4", "ranger"},
		{now.Add(-time.Hour), "codex", "old-5", "ranger"},
	} {
		seed.WriteString(e.line())
	}
	if err := os.WriteFile(f.b.App.OverflowLogPath(), []byte(seed.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "overflow grok: 2/2 in 7d — skipped") {
		t.Fatalf("a reached cap must skip and say so, got n=%d:\n%s", n, out)
	}
	if got := len(f.ledger(t)); got != 5 {
		t.Errorf("a skipped bead must append nothing: %d lines", got)
	}
	// The count itself, directly: 2 in the window for grok, 1 for codex.
	if got, err := f.b.App.OverflowCount("grok", now); err != nil || got != 2 {
		t.Errorf("OverflowCount(grok) = %d, %v — want 2 (entries past 7d do not count)", got, err)
	}
	if got, _ := f.b.App.OverflowCount("codex", now); got != 1 {
		t.Errorf("OverflowCount(codex) = %d, want 1 — the cap is per pool", got)
	}
}

// The fix in passing: a lane whose PID names a runtime that is not on the
// guarded meter is launched, tripped guard or not — and it is not an
// overflow launch, so nothing is charged to the cap.
func TestOverflowUngatedRuntimeLaunches(t *testing.T) {
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: grok\n---\nYou are ranger.\n"
	f := overflowPass(t, "plan_guard_overflow: codex\nplan_guard_overflow_cap: 5\n",
		pid, `["go","tier:standard"]`)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("a runtime off the guarded meter must not be skipped by it, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "← overflow") {
		t.Errorf("its own runtime is not an overflow move:\n%s", out)
	}
	if l := f.ledger(t); l != nil {
		t.Errorf("an ungated launch owes the ledger nothing: %v", l)
	}
	if !strings.Contains(calls(t, f.fake), "runtime/tier: grok/standard") {
		t.Errorf("the launch must still be grok's:\n%s", calls(t, f.fake))
	}
}

// The guard does not trip: nothing about overflow happens at all — no
// ledger read, no extra line, and the bead launches on its own runtime.
func TestOverflowUntrippedGuardReadsNothing(t *testing.T) {
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 42, 40) // under the threshold
	d, errb := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go","tier:standard"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "plan_guard_5h: 70\nplan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n")
	idleClaude(t, fake)
	// A ledger that would refuse every bead if it were read.
	os.MkdirAll(b.App.StateDir, 0o755)
	os.WriteFile(b.App.OverflowLogPath(), []byte(OverflowEntry{time.Now(), "grok", "x", "y"}.line()), 0o644)
	before, err := os.Stat(b.App.OverflowLogPath())
	if err != nil {
		t.Fatal(err)
	}

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 {
		t.Fatalf("an untripped guard changes nothing, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "overflow") {
		t.Errorf("an untripped pass says nothing about the pool:\n%s", out)
	}
	if errb.Len() != 0 {
		t.Errorf("and nothing on stderr: %q", errb.String())
	}
	if !strings.Contains(calls(t, fake), "runtime/tier: claude/standard") {
		t.Errorf("the launch stays on claude:\n%s", calls(t, fake))
	}
	if after, _ := os.Stat(b.App.OverflowLogPath()); after.ModTime() != before.ModTime() || after.Size() != before.Size() {
		t.Error("an untripped pass must not touch the ledger")
	}
}

// --dry-run acts on nothing: the move is shown, the ledger is not written,
// and no session is created.
func TestOverflowDryRun(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n",
		overflowPID, `["go","tier:standard"]`)
	f.d.DryRun = true

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 1 || !strings.Contains(out, "[grok ← overflow]") {
		t.Fatalf("--dry-run must show the move, got n=%d:\n%s", n, out)
	}
	if l := f.ledger(t); l != nil {
		t.Errorf("--dry-run writes no ledger: %v", l)
	}
	if log := calls(t, f.fake); strings.Contains(log, "workspace create") {
		t.Errorf("--dry-run creates nothing:\n%s", log)
	}
}

// §1's precedence: a --runtime the operator gave this pass is their decision
// about where these sessions run, and dispatch's own step-over never
// overrides one. On the guarded runtime that means the guard's skip line.
func TestOverflowExplicitRuntimePins(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n",
		overflowPID, `["go","tier:standard"]`)
	f.d.Runtime = "claude"

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "--runtime claude pins this pass — skipped") {
		t.Fatalf("an explicit --runtime must not be overridden, got n=%d:\n%s", n, out)
	}
}

// A template-only runtime is UNKNOWN to the meter, and unknown is gated:
// such a bead faces the ladder like any claude one rather than being waved
// through as "not on the meter".
func TestOnGuardedMeter(t *testing.T) {
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

// The config reader's own edges, without a pass around them.
func TestPlanGuardOverflowConfig(t *testing.T) {
	for _, tc := range []struct {
		cfg  string
		want Overflow
		says string
	}{
		{"", Overflow{}, ""},
		{"plan_guard_overflow: grok\nplan_guard_overflow_cap: 20\n", Overflow{"grok", 20}, ""},
		{"plan_guard_overflow: grok\n", Overflow{}, "plan_guard_overflow_cap:"},
		{"plan_guard_overflow: grok\nplan_guard_overflow_cap: lots\n", Overflow{}, "plan_guard_overflow_cap:"},
		{"plan_guard_overflow: grok\nplan_guard_overflow_cap: 0\n", Overflow{}, "plan_guard_overflow_cap:"},
		{"plan_guard_overflow_cap: 20\n", Overflow{}, "with no plan_guard_overflow:"},
	} {
		a := &App{ConfigPath: filepath.Join(t.TempDir(), "config.yaml")}
		if err := os.WriteFile(a.ConfigPath, []byte(tc.cfg), 0o644); err != nil {
			t.Fatal(err)
		}
		var errb strings.Builder
		got := a.PlanGuardOverflow(&errb)
		if got != tc.want {
			t.Errorf("%q → %+v, want %+v", tc.cfg, got, tc.want)
		}
		if tc.says == "" && errb.Len() != 0 {
			t.Errorf("%q must be silent, said %q", tc.cfg, errb.String())
		}
		if tc.says != "" && !strings.Contains(errb.String(), tc.says) {
			t.Errorf("%q must name %q, said %q", tc.cfg, tc.says, errb.String())
		}
	}
}

// ADR 0010 §5: a blind guard skips and never overflows. The `--watch` skip
// past `plan_guard_blind_max:` is not an over-threshold trip — there is no
// reading to judge which beads are on which meter, so nothing is moved and
// the ledger stays empty, cap or no cap.
func TestOverflowBlindGuardNeverOverflows(t *testing.T) {
	r := newBlindRig(t, guardOn+"\nplan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n")
	r.d.Unattended = true
	r.blind()
	r.at(30 * time.Minute) // well past the 10m default

	if n := r.run(t); n != 0 {
		t.Fatalf("a blind unattended pass is skipped, got n=%d:\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "pass skipped") || strings.Contains(r.out(), "overflow") {
		t.Errorf("a blind skip is a park, not a step-over:\n%s", r.out())
	}
	if _, err := os.Stat(r.d.App.OverflowLogPath()); !os.IsNotExist(err) {
		t.Errorf("a blind pass writes nothing to the ledger (%v)", err)
	}
}
