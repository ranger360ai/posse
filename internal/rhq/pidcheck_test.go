package rhq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckAgent(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	os.MkdirAll(a.AgentsDir, 0o755)

	// The scaffold and the reference PIDs are clean.
	if _, err := a.ScaffoldAgent("fresh"); err != nil {
		t.Fatal(err)
	}
	if fs, _, _ := a.CheckAgent("fresh"); len(fs) != 0 {
		t.Errorf("scaffold has findings: %v", fs)
	}
	ref, _ := os.ReadFile(filepath.Join("..", "..", "examples", "agents", "architect.md"))
	if len(ref) > 0 {
		os.WriteFile(filepath.Join(a.AgentsDir, "architect.md"), ref, 0o644)
		// The shelf PID declares skills: [distributed-systems] (ADR 0012 D2),
		// and `posse init` seeds that skill from examples/skills — so "clean"
		// is judged in a home that HAS been seeded, not in a bare directory.
		mkSkill(t, a.SkillsDir(), "distributed-systems")
		if fs, _, _ := a.CheckAgent("architect"); len(fs) != 0 {
			t.Errorf("reference PID has findings: %v", fs)
		}
		// The other side of that same coin: copy the shelf PID into a home
		// whose skills/ does not carry it and the lint says so by name. That
		// is the whole point of declaring it — the binding is required, and a
		// missing skill is a finding rather than a silent drop.
		os.RemoveAll(a.SkillPath("distributed-systems"))
		fs, _, _ := a.CheckAgent("architect")
		if len(fs) != 1 || !strings.Contains(fs[0], `unknown skill "distributed-systems"`) {
			t.Errorf("a home without the seeded skill must name it: %v", fs)
		}
	}

	// One of everything wrong.
	bad := `---
name: bad
command: claude {allow} {deny} --verbose
allow: [Bash(git commit -m a,b), Edit]
metrics: [closed-no-reopen, made-up-metric]
---
Hello, I am bad.

## Metrics
- whatever

## Who you are
x

## Intents
no table here

## Guardrails
no risk lines

## Handoffs
## Done
## Blocked
`
	os.WriteFile(filepath.Join(a.AgentsDir, "bad.md"), []byte(bad), 0o644)
	fs, _, err := a.CheckAgent("bad")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(fs, "\n")
	for _, want := range []string{
		`allow: inline list split "Bash(git commit -m a" on a comma inside a permission rule`,
		"command: {allow} must be last",
		"body must open with the identity line",
		"missing section ## How you work",
		"missing section ## Memory",
		"section ## Metrics out of contract order", // it came before ## Who you are
		"## Intents lacks the table header",
		"## Guardrails does not carry the four hard risk lines",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing finding %q in:\n%s", want, joined)
		}
	}
	// The catalog is derived from the PIDs now (ADR 0001 amendment): a
	// persona naming how it is judged is never "unknown".
	if strings.Contains(joined, "unknown id") {
		t.Errorf("metric ids are no longer rejected:\n%s", joined)
	}
	// A balanced rule in inline form is fine — the crew writes them that way.
	if strings.Contains(joined, `"Edit"`) {
		t.Errorf("inline form flagged for its own sake:\n%s", joined)
	}
	if _, _, err := a.CheckAgent("nope"); err == nil {
		t.Error("unknown persona must error")
	}
}

// The derived catalog: the union of the PIDs' metrics: plus config
// metric_ids:, with two spellings of one metric flagged as a near-duplicate
// (ADR 0001 amendment 2026-08-18).
func TestMetricCatalogAndNearDuplicates(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	os.MkdirAll(a.AgentsDir, 0o755)
	pid := func(name, metrics string) {
		md := "---\nname: " + name + "\ndescription: t\nmetrics: " + metrics + "\n---\nYou are " + name + ".\n"
		os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644)
	}
	pid("security", "[findings-surviving-triage, severity-honesty]")
	pid("qa", "[findings-survive-triage, closed-no-reopen]")
	pid("developer", "[closed-no-reopen, suite-green-on-close]")
	os.WriteFile(a.ConfigPath, []byte("metric_ids: [escapes-caught]\n"), 0o644)

	cat := a.MetricCatalog()
	for id, want := range map[string]string{
		"closed-no-reopen":          "developer, qa",
		"findings-surviving-triage": "security",
		"suite-green-on-close":      "developer",
		"escapes-caught":            "config metric_ids:",
	} {
		if got := MetricDeclaredBy(cat, id); got != want {
			t.Errorf("catalog[%s] declared by %q, want %q", id, got, want)
		}
	}
	if _, ok := cat["made-up"]; ok {
		t.Error("catalog invented an id")
	}

	fs, _, err := a.CheckAgent("qa")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(fs, "\n")
	want := `metrics: "findings-survive-triage" is near "findings-surviving-triage" in security — one spelling`
	if !strings.Contains(joined, want) {
		t.Errorf("missing near-duplicate finding %q in:\n%s", want, joined)
	}
	// An id every persona spells the same way is not a near-duplicate of
	// itself, and unrelated ids stay quiet.
	if strings.Contains(joined, `"closed-no-reopen" is near`) {
		t.Errorf("a shared id flagged against itself:\n%s", joined)
	}
	for _, f := range mustCheck(t, a, "developer") {
		if strings.HasPrefix(f, "metrics:") {
			t.Errorf("developer's own vocabulary is clean, got: %s", f)
		}
	}
	// Config ids join the vocabulary check too.
	pid("qa", "[escape-caught]")
	fs, _, _ = a.CheckAgent("qa")
	if !strings.Contains(strings.Join(fs, "\n"), `is near "escapes-caught" in config metric_ids:`) {
		t.Errorf("config metric_ids not in the near-duplicate check: %v", fs)
	}
}

func mustCheck(t *testing.T, a *App, name string) []string {
	t.Helper()
	fs, _, err := a.CheckAgent(name)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func TestMetricKey(t *testing.T) {
	same := [][2]string{
		{"findings-survive-triage", "findings-surviving-triage"},
		{"bugs-with-repros", "bug-with-repro"},
		{"escapes-caught", "escape-caught"},
		{"spec-clarity", "specs-clarity"},
		{"Findings_Survive_Triage", "findings-survive-triage"},
	}
	for _, p := range same {
		if metricKey(p[0]) != metricKey(p[1]) {
			t.Errorf("%q and %q should key the same (%q vs %q)", p[0], p[1], metricKey(p[0]), metricKey(p[1]))
		}
	}
	differ := [][2]string{
		{"closed-no-reopen", "suite-green-on-close"},
		{"queue-honesty", "severity-honesty"},
		{"blocked-honestly", "blocked-time-to-intervention"},
		{"crew-throughput", "interrupts-worth-it"},
	}
	for _, p := range differ {
		if metricKey(p[0]) == metricKey(p[1]) {
			t.Errorf("%q and %q must not collapse (both %q)", p[0], p[1], metricKey(p[0]))
		}
	}
}

// ADR 0018 §5: the parity lint's drift alarm — advisory only (a warning,
// never a finding), and it fires on both arms: a push-granting PID that is
// not the named coordinator, and a push-granting PID when no coordinator is
// named at all.
func TestCheckAgentCoordinatorParity(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	os.MkdirAll(a.AgentsDir, 0o755)
	pid := func(name string) {
		md := "---\nname: " + name + "\ndescription: t\nallow: [Bash(git push:*)]\n---\nYou are " + name + ", the role of the crew.\n"
		os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644)
	}
	pid("business-manager")
	const want = "grants the coordinator's defining permission"

	// No coordinator: configured at all — the important arm.
	_, ws, err := a.CheckAgent("business-manager")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(ws, "\n")
	if !strings.Contains(joined, want) || !strings.Contains(joined, "no coordinator: is configured") {
		t.Errorf("missing no-coordinator-named warning:\n%s", joined)
	}

	// coordinator: names someone else — drift.
	os.WriteFile(a.ConfigPath, []byte("coordinator: coordinator\n"), 0o644)
	_, ws, err = a.CheckAgent("business-manager")
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(ws, "\n")
	if !strings.Contains(joined, want) || !strings.Contains(joined, "not the coordinator") {
		t.Errorf("missing drift warning:\n%s", joined)
	}

	// coordinator: names this PID — no warning.
	os.WriteFile(a.ConfigPath, []byte("coordinator: business-manager\n"), 0o644)
	_, ws, err = a.CheckAgent("business-manager")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(ws, "\n"), want) {
		t.Errorf("the coordinator itself must not warn on its own grant: %v", ws)
	}
	// Case/path spelling still resolves to the same identity (isCoordinator).
	os.WriteFile(a.ConfigPath, []byte("coordinator: Business-Manager\n"), 0o644)
	_, ws, err = a.CheckAgent("business-manager")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(ws, "\n"), want) {
		t.Errorf("a case-different spelling of the same coordinator must not warn: %v", ws)
	}

	// A PID that does not grant push stays quiet regardless of config.
	os.WriteFile(a.ConfigPath, []byte("coordinator: someone-else\n"), 0o644)
	if _, err := a.ScaffoldAgent("quiet"); err != nil {
		t.Fatal(err)
	}
	_, ws, err = a.CheckAgent("quiet")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(ws, "\n"), want) {
		t.Errorf("a PID that grants no push must not get the ADR 0018 §5 warning: %v", ws)
	}
}

func TestBalancedParens(t *testing.T) {
	for _, ok := range []string{"Edit", "Bash(git push:*)", "Bash(posse:*)", "a(b(c))"} {
		if !balancedParens(ok) {
			t.Errorf("%q should be balanced", ok)
		}
	}
	for _, bad := range []string{"Bash(git commit -m a", "b)", ")Bash("} {
		if balancedParens(bad) {
			t.Errorf("%q should be unbalanced", bad)
		}
	}
}

// The checker must stay in step with the fleet it ships with (rangerhq-h44:
// `agent check` reported `missing section ## Role` against PIDs that say
// `## Who you are`, and `metrics: unknown id` for a persona's own ids —
// both since ruled on by the ADR 0001 amendment, neither pinned as a fact
// about the *shelf*). So: every examples/agents PID, in one home posse init
// has seeded, lints clean. A one-file pin (TestCheckAgent, architect.md
// alone) cannot see a finding computed across personas — the derived
// catalog's near-duplicate check is one — and a headings-only pin
// (TestExampleAgentsArePIDs) cannot see anything the linter checks that is
// not a heading.
func TestShelfPIDsLintCleanAsASet(t *testing.T) {
	shelf, _ := filepath.Glob(filepath.Join("..", "..", "examples", "agents", "*.md"))
	if len(shelf) < 9 {
		t.Skipf("reference PIDs not present (%d found)", len(shelf))
	}
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range shelf {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a.AgentsDir, filepath.Base(p)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The shelf declares skills: [distributed-systems]; `posse init` seeds
	// examples/skills into the home, so "clean" is judged in a seeded home.
	seeded, _ := filepath.Glob(filepath.Join("..", "..", "examples", "skills", "*"))
	for _, p := range seeded {
		mkSkill(t, a.SkillsDir(), filepath.Base(p))
	}

	// Positive witness: a green loop over an empty set proves nothing.
	names := a.ListAgents()
	if len(names) != len(shelf) {
		t.Fatalf("seeded %d shelf PIDs, ListAgents sees %d: %v", len(shelf), len(names), names)
	}
	for _, n := range names {
		fs, _, err := a.CheckAgent(n)
		if err != nil {
			t.Errorf("%s: %v", n, err)
			continue
		}
		if len(fs) != 0 {
			t.Errorf("%s: shelf PID has findings: %v", n, fs)
		}
	}

	// Control: put one shelf PID back on the ADR's first-draft heading and
	// the lint must name it. Without this the loop above is green whether
	// CheckAgent looked at the bodies or not.
	raw, err := os.ReadFile(filepath.Join(a.AgentsDir, "developer.md"))
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(raw), "\n## Who you are\n", "\n## Role\n", 1)
	if drifted == string(raw) {
		t.Fatal("control did not plant: developer.md has no ## Who you are heading")
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"), []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, _, err := a.CheckAgent("developer")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(fs, "\n"), "missing section ## Who you are") {
		t.Errorf("control: a drifted identity heading must be a finding, got %v", fs)
	}
}
