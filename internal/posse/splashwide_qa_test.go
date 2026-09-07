//go:build !posse_arm2 && !posse_arm3

package posse

// rangerhq-ydfw / the close of rangerhq-1xsj: grok's startup_splash is idle
// on purpose, keyed on the New worktree / Resume session menu PLUS a footer
// line `Grok Build <ver> ([channel])?`. That footer is a NARROW-pane layout.
// A production-width herdr pane draws Grok Build inside the boxed logo, with
// a bare `[stable]` under the composer, so line_regex misses and the named
// rule 1xsj kept does not fire. File-explain of this capture is fallback idle;
// the live pane is rescued only by osc_title_idle when grok has emitted OSC.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestQAGrokWideBoxedSplashIsNamedIdle(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// go test ./internal/posse runs with cwd = this package. The capture lives
	// with the other detection fixtures so scripts/verify-detection.sh — the
	// gate that runs against the INSTALLED manifest after `make install-detection`
	// or a `herdr update` — covers the wide layout too (ranger-base-neyn).
	file := filepath.Join(root, "..", "..", "etc", "herdr", "agent-detection",
		"testdata", "grok", "idle-startup-splash-wide-boxed.txt")
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
	// Execute the manifest in this checkout, not the installed fleet override:
	// a committed detection fix must prove itself before an operator deploys it.
	config := t.TempDir()
	overrides := filepath.Join(config, "herdr", "agent-detection")
	if err := os.MkdirAll(overrides, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "..", "..", "etc", "herdr", "agent-detection", "grok.toml")
	b, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overrides, "grok.toml"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("herdr", "agent", "explain", "--file", file, "--agent", "grok", "--json")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+config, "XDG_STATE_HOME="+filepath.Join(config, "state"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("herdr agent explain: %v\n%s", err, out)
	}
	// --file is a bare detection object (top-level state). A pane explain
	// through the CLI wraps the same object in {"result":...}. Accept both.
	var det struct {
		State       string `json:"state"`
		Fallback    string `json:"fallback_reason"`
		MatchedRule *struct {
			ID string `json:"id"`
		} `json:"matched_rule"`
	}
	var wrap struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(out, &wrap); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	raw := out
	if len(wrap.Result) > 0 && wrap.Result[0] == '{' {
		raw = wrap.Result
	}
	if err := json.Unmarshal(raw, &det); err != nil {
		t.Fatalf("detection json: %v\n%s", err, out)
	}
	got := ""
	if det.MatchedRule != nil {
		got = det.MatchedRule.ID
	}
	if det.State != "idle" {
		t.Errorf("state=%q, want idle (the 1xsj decision: this screen is not a blocker)", det.State)
	}
	if got != "startup_splash" {
		t.Errorf("rule=%q fallback=%q, want startup_splash — production-width boxed splash is the screen 1xsj kept this rule to name", got, det.Fallback)
	}
}
