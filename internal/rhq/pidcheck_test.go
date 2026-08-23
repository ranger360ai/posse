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
		if fs, _, _ := a.CheckAgent("architect"); len(fs) != 0 {
			t.Errorf("reference PID has findings: %v", fs)
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
