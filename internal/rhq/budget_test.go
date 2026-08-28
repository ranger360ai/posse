package rhq

// Dial E (ADR 0003 §4, rangerhq-25p): budget caps, the 80% step-down and
// the 100% stop. Hermetic — the dollars come from an injected Spend, so no
// test reads the operator's transcripts.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// budgetConfig writes scalars first, then the beads list: the flat-YAML
// block list runs to the end of its key (the planConfig lesson).
func budgetConfig(t *testing.T, a *App, repo, scalars string) {
	t.Helper()
	cfg := scalars
	if cfg != "" && !strings.HasSuffix(cfg, "\n") {
		cfg += "\n"
	}
	if repo != "" {
		cfg += "beads:\n  - " + repo + "\n"
	}
	if err := os.WriteFile(a.ConfigPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
}

// budgetPersona is writePersona with extra PID frontmatter (tier:, deny:).
func budgetPersona(t *testing.T, a *App, name, labels, extra string) {
	t.Helper()
	os.MkdirAll(a.AgentsDir, 0o755)
	md := "---\nname: " + name + "\ndescription: test\nlabels: " + labels + "\n" + extra + "---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// spendStub makes every scan report one bead segment costing usd, started
// now (so it lands in both the day and the pass window), and counts scans.
func spendStub(d *Dispatcher, usd float64, scans *int) {
	d.Spend = func(since time.Time) *CostReport {
		if scans != nil {
			*scans++
		}
		return &CostReport{Beads: []*Segment{{Bead: "spent", Start: time.Now(), CostUSD: usd}}}
	}
}

func TestBudgetCapsConfig(t *testing.T) {
	b, _ := newTestBackend(t)
	a := b.App
	var errs strings.Builder

	budgetConfig(t, a, "", "")
	if p, d := a.BudgetCaps(&errs); p != 0 || d != 0 {
		t.Errorf("unset must be no cap, got pass=%v day=%v", p, d)
	}
	budgetConfig(t, a, "", "budget_pass: 25\nbudget_day: $100.50")
	if p, d := a.BudgetCaps(&errs); p != 25 || d != 100.50 {
		t.Errorf("pass=%v day=%v (a leading $ is allowed)", p, d)
	}
	if errs.String() != "" {
		t.Errorf("clean config complained: %s", errs.String())
	}
	// A typo must be visible, not a silently dead cap.
	budgetConfig(t, a, "", "budget_day: lots")
	if _, d := a.BudgetCaps(&errs); d != 0 {
		t.Errorf("garbage must read as no cap, got %v", d)
	}
	if !strings.Contains(errs.String(), "budget_day") {
		t.Errorf("bad value not named on errw: %q", errs.String())
	}
	budgetConfig(t, a, "", "budget_pass: 0")
	errs.Reset()
	if p, _ := a.BudgetCaps(&errs); p != 0 || !strings.Contains(errs.String(), "budget_pass") {
		t.Errorf("zero is not a cap and must be named: p=%v errs=%q", p, errs.String())
	}
}

// The tightest window drives both rungs, and plan utilization competes with
// the dollar windows on equal terms.
func TestBudgetStateResolve(t *testing.T) {
	cases := []struct {
		name   string
		st     BudgetState
		window string
		pct    float64
		step   bool
		stop   bool
	}{
		{"dormant", BudgetState{}, "", 0, false, false},
		{"day quiet", BudgetState{DayCap: 100, DaySpend: 40}, "day", 40, false, false},
		{"day steps", BudgetState{DayCap: 100, DaySpend: 80}, "day", 80, true, false},
		{"day stops", BudgetState{DayCap: 100, DaySpend: 103}, "day", 103, true, true},
		{"epoch is tighter", BudgetState{PassCap: 25, PassSpend: 24, DayCap: 100, DaySpend: 40}, "epoch", 96, true, false},
		{"plan is tighter", BudgetState{DayCap: 100, DaySpend: 10, Plan: PlanUsage{{"5h", 84}, {"7d", 30}}}, "plan 5h", 84, true, false},
		{"plan alone is not a cap", BudgetState{Plan: PlanUsage{{"5h", 99}}}, "plan 5h", 99, false, false},
		// The arithmetic never learned what a window is called (ADR 0012
		// D4): a provider that names its windows something else gets the
		// same tightest-window answer, with its own label on the line.
		{"another provider's windows", BudgetState{DayCap: 100, DaySpend: 10, Plan: PlanUsage{{"burst", 20}, {"month", 91}}}, "plan month", 91, true, false},
	}
	for _, c := range cases {
		st := c.st
		st.resolve()
		if st.Window != c.window || st.Pct != c.pct {
			t.Errorf("%s: window=%q pct=%v, want %q %v", c.name, st.Window, st.Pct, c.window, c.pct)
		}
		if st.StepDown() != c.step || st.Stop() != c.stop {
			t.Errorf("%s: step=%v stop=%v, want %v %v", c.name, st.StepDown(), st.Stop(), c.step, c.stop)
		}
	}
	st := BudgetState{DayCap: 100, DaySpend: 84.2}
	st.resolve()
	if got := st.Line(); got != "day $84.20 of $100.00 (84%)" {
		t.Errorf("Line: %q", got)
	}
	if got := st.Short(); got != "day 84%" {
		t.Errorf("Short: %q", got)
	}
}

// The two windows are cut from one scan: day by calendar day, pass by the
// moment the pass began.
func TestPassAndDayWindows(t *testing.T) {
	now := time.Now()
	passStart := now.Add(-10 * time.Minute)
	rep := &CostReport{Beads: []*Segment{
		{Bead: "old-day", Start: now.AddDate(0, 0, -2), CostUSD: 7},
		{Bead: "earlier-today", Start: startOfDay(now).Add(time.Minute), CostUSD: 3},
		{Bead: "this-pass", Start: now.Add(-time.Minute), CostUSD: 5},
	}}
	if got := rep.DayTotal(now); got != 8 {
		t.Errorf("day total %v, want 8 (today's beads only)", got)
	}
	if got := rep.PassTotal(passStart); got != 5 {
		t.Errorf("pass total %v, want 5 (this pass's beads only)", got)
	}
	if got := rep.PassTotal(time.Time{}); got != 0 {
		t.Errorf("no pass in flight must be an empty window, got %v", got)
	}
	// Interactive never reaches Beads (Dial G: shown, never gated).
	if len(rep.Beads) != 3 {
		t.Fatal("fixture drifted")
	}
}

// Dormant is free: with no cap set nothing is scanned at all.
func TestBudgetDormantNeverScans(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	scans := 0
	spendStub(d, 999, &scans)
	budgetPersona(t, b.App, "ranger", "[go]", "tier: standard\n")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	budgetConfig(t, b.App, repo, "")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if scans != 0 {
		t.Errorf("caps unset but transcripts scanned %d time(s) — Dial E must be dormant", scans)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "[standard via PID]") {
		t.Errorf("tier moved with no cap set:\n%s", out)
	}
}

// ≥80% of a window: a standard-by-default session runs at fast, strong holds.
func TestBudgetStepDownAt80(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	spendStub(d, 85, nil)
	budgetPersona(t, b.App, "ranger", "[go]", "tier: standard\n")
	budgetPersona(t, b.App, "judge", "[adr]", "tier: strong\n")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["adr"]}]`, "")
	budgetConfig(t, b.App, repo, "budget_day: 100")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "[fast via budget step-down at day 85%, was standard via PID]") {
		t.Errorf("standard did not step down at 85%%:\n%s", out)
	}
	if !strings.Contains(out, "[strong via tier_by_label adr]") {
		t.Errorf("strong must hold — judged work is never traded silently:\n%s", out)
	}
}

// The step-down yields to every rule that outranks a budget: a pinned tier,
// the PID's floor, and the wall's inability to realize a gate at fast.
func TestBudgetStepDownYields(t *testing.T) {
	for _, c := range []struct {
		name, persona, extra, bead, want string
	}{
		{"label pin", "ranger", "tier: standard\n",
			`{"id":"a-1","title":"t","labels":["go","tier:standard"]}`, "[standard via label tier:standard]"},
		{"tier_floor", "ranger", "tier: standard\ntier_floor: standard\n",
			`{"id":"a-1","title":"t","labels":["go"]}`, "[standard via PID]"},
		{"unrealized gate at fast", "ranger", "tier: standard\ndeny: [Edit]\n",
			`{"id":"a-1","title":"t","labels":["go"]}`, "[standard via PID]"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			d.DryRun = true
			spendStub(d, 95, nil)
			budgetPersona(t, b.App, c.persona, "[go]", c.extra)
			repo := qaRepo(t, b.App, "["+c.bead+"]", "")
			budgetConfig(t, b.App, repo, "budget_day: 100")

			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			if !strings.Contains(out, c.want) {
				t.Errorf("want %s, got:\n%s", c.want, out)
			}
			if strings.Contains(out, "step-down") {
				t.Errorf("stepped down anyway:\n%s", out)
			}
		})
	}
}

// --tier is the operator's own decision; a budget is not an argument
// against it.
func TestBudgetStepDownYieldsToTierFlag(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	d.Tier = TierStandard
	spendStub(d, 95, nil)
	budgetPersona(t, b.App, "ranger", "[go]", "")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	budgetConfig(t, b.App, repo, "budget_day: 100")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "[standard via --tier]") {
		t.Errorf("--tier must hold:\n%s", out)
	}
}

// ≥100%: nothing launches, every skipped bead gets a line, and the reading
// is taken once for the pass — not once per bead.
func TestBudgetStopAt100(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	scans := 0
	spendStub(d, 120, &scans)
	budgetPersona(t, b.App, "ranger", "[go]", "tier: standard\n")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`, "")
	budgetConfig(t, b.App, repo, "budget_day: 100")
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 {
		t.Errorf("dispatched %d over a spent budget:\n%s", n, out)
	}
	for _, id := range []string{"a-1", "a-2"} {
		if !strings.Contains(out, id) {
			t.Errorf("no line for skipped bead %s:\n%s", id, out)
		}
	}
	if !strings.Contains(out, "budget: day $120.00 of $100.00 (120%)") {
		t.Errorf("the stop must name the window and the numbers:\n%s", out)
	}
	if scans != 1 {
		t.Errorf("stop must be sticky for the pass: %d scans for 2 beads", scans)
	}
	if strings.Contains(bdCalls(t, fake), "--claim") {
		t.Errorf("bead claimed although nothing was launched:\n%s", bdCalls(t, fake))
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("prompt fired over a spent budget:\n%s", calls(t, fake))
	}
}

// The cockpit's one-off dispatch is fleet work spending fleet money: the
// same cap holds there.
func TestBudgetStopHoldsLaunchBead(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	spendStub(d, 120, nil)
	budgetPersona(t, b.App, "ranger", "[go]", "tier: standard\n")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	budgetConfig(t, b.App, repo, "budget_day: 100")
	idleClaude(t, fake)

	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}
	_, err := d.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "budget: day") {
		t.Fatalf("launch over a spent budget: err=%v", err)
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("prompt fired:\n%s", calls(t, fake))
	}
	// …and the refusal is not remembered: a raised cap (or a new day) must
	// be picked up on the next keystroke, not after a restart.
	spendStub(d, 10, nil)
	if _, err := d.LaunchBead(is); err != nil && strings.Contains(err.Error(), "budget") {
		t.Errorf("cockpit went on refusing under the cap: %v", err)
	}
}
