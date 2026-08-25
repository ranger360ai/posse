package rhq

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
	t.Skip("ranger-base-z6n: production-width boxed splash must resolve to startup_splash, not fallback idle")
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// go test ./internal/rhq runs with cwd = this package.
	file := filepath.Join(root, "testdata", "grok-startup-splash-wide-boxed.txt")
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("herdr", "agent", "explain", "--file", file, "--agent", "grok", "--json").CombinedOutput()
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
