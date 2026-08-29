package posse

// QA pins for ranger-base-3bqn — the bd argv gate (scripts/bd-argv-gate.sh
// and its parser).
//
// Claim: a `Bash(bd <verb>:*)` permission rule matches a TOKEN PREFIX of the
// typed line, so any of bd's global flags in front of the verb moves the verb
// out of the prefix and the rule misses — `bd --no-daemon daemon --help` ran,
// exit 0 (MEASURED on ranger-base-az93). The gate resolves the verb instead
// of matching a prefix, so no reordering moves it.
//
// The three arms the bead asked for, each with its wrong arm, were measured
// against the real claude 2.1.251 binary; the rig and its numbers are in the
// instance tree (ADR 0024 keeps run records out of here). What is pinned in
// this file is the part that is code: the decisions, and the failure path.
//
// Load-bearing contract, quoted from the 2.1.251 bundle: "Exit code 2 - show
// stderr to model and block tool call / Other exit codes - show stderr to
// user only but continue with tool call". A hook that dies any other way is
// FAIL-OPEN, which is why every could-not-decide path here exits 2 — and why
// it does so only for calls that name bd, so a broken interpreter degrades bd
// instead of wedging every Bash call in the fleet.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type gateResult struct {
	code   int
	stdout string
	stderr string
}

func runGate(t *testing.T, env []string, command string) gateResult {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"session_id": "qa",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	return runGateRaw(t, env, string(payload))
}

func runGateRaw(t *testing.T, env []string, payload string) gateResult {
	t.Helper()
	cmd := exec.Command("sh", "scripts/bd-argv-gate.sh")
	cmd.Stdin = strings.NewReader(payload)
	cmd.Env = append(os.Environ(), env...)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running the gate: %v", err)
	}
	return gateResult{code: code, stdout: out.String(), stderr: errb.String()}
}

// denied reports the refusal reason, or "" when the gate stayed out of the way.
func denied(t *testing.T, r gateResult) string {
	t.Helper()
	if r.code == 2 {
		return "exit2: " + strings.TrimSpace(r.stderr)
	}
	if r.code != 0 {
		t.Fatalf("the gate must never exit %d — every code but 0 and 2 is FAIL-OPEN: %q", r.code, r.stderr)
	}
	if strings.TrimSpace(r.stdout) == "" {
		return ""
	}
	var decision struct {
		Hook struct {
			Event  string `json:"hookEventName"`
			Decide string `json:"permissionDecision"`
			Reason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &decision); err != nil {
		t.Fatalf("the gate wrote something that is not a hook decision: %q (%v)", r.stdout, err)
	}
	if decision.Hook.Event != "PreToolUse" {
		t.Errorf("hookEventName must be PreToolUse, got %q", decision.Hook.Event)
	}
	// The gate must never emit "allow": an allow decision would take the
	// call OUT of the normal permission pipeline and widen every persona's
	// grants to every bd verb on the allow-list.
	if decision.Hook.Decide != "deny" {
		t.Fatalf("the only decision this gate may emit is deny, got %q", decision.Hook.Decide)
	}
	return decision.Hook.Reason
}

func TestQABdArgvGateResolvesTheVerb(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	// The pair from az93 — one token apart, and the second one is the
	// spelling the fleet is taught to type — plus every other way the verb
	// has been seen to move.
	refused := map[string]string{
		"bd daemon --help":                          "daemon",
		"bd --no-daemon daemon --help":              "daemon",
		"bd --json --no-daemon daemon stop":         "daemon",
		"bd --db /tmp/x daemon stop":                "daemon",
		"bd --actor someone daemon stop":           "daemon",
		"bd --lock-timeout 5s daemon stop":          "daemon",
		"cd /tmp && bd --no-daemon daemons killall": "daemons",
		"/Users/x/.local/bin/bd daemon stop":        "daemon",
		"./bd admin reset":                          "admin",
		"env bd --db /tmp/x admin reset":            "admin",
		"BD_ACTOR=x bd delete ranger-base-1":        "delete",
		"bd doctor --fix":                           "doctor",
		"bd rename-prefix old new":                  "rename-prefix",
		"bd jira sync":                              "jira",
		"bd setup claude":                           "setup",
		"bd show x && bd hook pre-commit":           "hook",
		"bd sync --full":                            "sync --full",
		"bd dep relate a b":                         "dep relate",
		"bd config unset sync.branch":               "config",
		"sh -c \"bd daemon stop\"":                  "sh",
		"cat ids | xargs bd delete":                 "xargs",
		"eval \"bd daemon stop\"":                   "eval",
		"$(command -v bd) daemon stop":              "$(",
		"BD=bd; $BD daemon stop":                    "$BD",
		"bd --no-daemon 'daemon' stop":              "daemon",
	}
	for command, want := range refused {
		reason := denied(t, runGate(t, nil, command))
		if reason == "" {
			t.Errorf("MUST be refused, was waved through: %s", command)
			continue
		}
		if !strings.Contains(reason, want) {
			t.Errorf("refusal of %q must name %q, said: %s", command, want, reason)
		}
	}

	// The other half: a gate that refuses everything is not a gate. Nothing
	// here may be touched — including the lines that merely mention bd.
	allowed := []string{
		"bd show ranger-base-3bqn",
		"bd --no-daemon ready --json",
		"bd --db /tmp/x list --status open",
		"bd comments add ranger-base-3bqn 'done'",
		"bd close ranger-base-3bqn",
		"bd dep add a b",
		"bd sync",
		"bd version",
		"bd",        // bd's own usage
		"bd --json", // options only, still usage
		"bd help daemon",
		"go test ./...",
		"git status",
		"grep -rn bd internal/",
		"echo 'bd daemon stop'",
		"cd /tmp/bd && ls",
		"$PYTHON script.py", // a variable command word, no bd in the line
	}
	for _, command := range allowed {
		if reason := denied(t, runGate(t, nil, command)); reason != "" {
			t.Errorf("must pass through, was refused: %s -> %s", command, reason)
		}
	}
}

func TestQABdArgvGateFailsClosedOnlyForBd(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	missing := filepath.Join(t.TempDir(), "not-a-parser.py")
	broken := filepath.Join(t.TempDir(), "broken.py")
	if err := os.WriteFile(broken, []byte("def (\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, env := range [][]string{
		{"BD_ARGV_GATE_PY=" + missing},
		{"BD_ARGV_GATE_PY=" + broken},
		{"BD_ARGV_GATE_PYTHON=" + filepath.Join(t.TempDir(), "no-python3")},
	} {
		// bd is refused when the parser cannot speak for it…
		r := runGate(t, env, "bd --no-daemon daemon stop")
		if r.code != 2 || !strings.Contains(r.stderr, "fail closed") {
			t.Errorf("%v: a bd call must be refused when the parser is unavailable: code=%d err=%q", env, r.code, r.stderr)
		}
		// …and NOTHING else is, or one bad path in settings.json wedges every
		// Bash call for every persona. `python3 <missing file>` exits 2 all
		// by itself, which is why the wrapper does not key on the code alone
		// (MEASURED: the first cut of this gate denied `go test` here).
		for _, safe := range []string{"go test ./...", "git status", "echo hello"} {
			if r := runGate(t, env, safe); r.code != 0 || strings.TrimSpace(r.stdout) != "" {
				t.Errorf("%v: %q must be untouched by a broken parser: code=%d out=%q err=%q",
					env, safe, r.code, r.stdout, r.stderr)
			}
		}
	}

	// A payload the gate cannot read at all is the same rule: refuse where bd
	// is in play, keep quiet where it is not.
	if r := runGateRaw(t, nil, "this is not json, bd daemon stop"); r.code != 2 {
		t.Errorf("unreadable payload naming bd must be refused, got code=%d", r.code)
	}
	if r := runGateRaw(t, nil, "this is not json at all"); r.code != 0 {
		t.Errorf("unreadable payload with no bd in it must be left alone, got code=%d %q", r.code, r.stderr)
	}
	// And a call for another tool is never this gate's business, even when
	// its arguments happen to spell bd.
	if r := runGateRaw(t, nil, `{"tool_name":"Read","tool_input":{"file_path":"/x/bd/y"}}`); r.code != 0 || r.stdout != "" {
		t.Errorf("a non-Bash tool call must pass untouched, got code=%d out=%q", r.code, r.stdout)
	}
}
