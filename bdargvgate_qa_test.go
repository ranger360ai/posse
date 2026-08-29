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
		"bd --actor someone daemon stop":            "daemon",
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

		// The SHELL's own spellings of bd, in the same table as the literal
		// one (ranger-base-hthx). The parser resolves the command word with
		// shlex before it asks whether the basename is bd, so each of these
		// is a bd call carrying no literal `bd` substring — which is exactly
		// what the wrapper's fast path used to answer on. `b\d ship --help`
		// RAN through the shipped fence, one character from a refusal.
		`b\d admin reset`:                  "admin",
		`b\d daemon stop`:                  "daemon",
		`b\d ship --help`:                  "ship",
		`b\d mail --help`:                  "mail",
		`b\d duplicates --help`:            "duplicates",
		`b\d rename-prefix old new`:        "rename-prefix",
		`b\d jira sync --push`:             "jira",
		`b\d --no-daemon daemon --help`:    "daemon",
		`b\d dep relate a b`:               "dep relate",
		`b\d sync --full`:                  "sync --full",
		`/usr/local/bin/b\d admin reset`:   "admin",
		`env b\d --db /tmp/x admin reset`:  "admin",
		`PATH=/x b\d delete ranger-base-1`: "delete",
		`echo hi | b\d import`:             "import",
		// RIG CAVEAT, measured and inherited from the pin this replaces:
		// Go's encoding/json escapes &, < and > as \u00xx, and the wrapper
		// keeps any payload containing \u on the slow path. So a row spelled
		// with those characters cannot exercise the fast path HERE however it
		// behaves in the harness (node's JSON.stringify does not escape them),
		// and `cd /tmp && b\d ...` is deliberately NOT in this table — it
		// would be green over a broken fast path. The compound and redirect
		// spellings are swept by `make verify-bd-argv-gate`, which builds its
		// payloads with python's json and does not escape them.
		"b''d daemon stop":   "daemon",
		"'b''d' daemon stop": "daemon",
		"'b'd daemon stop":   "daemon",
		"b'd' daemon stop":   "daemon",
		`b""d daemon stop`:   "daemon",
		`"b""d" daemon stop`: "daemon",
		`"b"d daemon stop`:   "daemon",
		`b"d" daemon stop`:   "daemon",
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

		// The other side of the shell spellings: widening the fast path must
		// not widen the REFUSAL. These reach the parser now and must come
		// back silent (ranger-base-hthx).
		`b\d show ranger-base-3bqn`, // an escaped spelling of an ALLOWED verb
		`b" "d daemon stop`,         // runs `b d`; the quotes do not concatenate
		`b\\d daemon stop`,          // literal b\d, which is not bd either
		`b\'d daemon stop`,          // literal b'd
		`sed 's/a/b/' f`,
		`echo "b"`,
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
	// The wrapper answers a payload with no `bd` in it from a shell builtin,
	// without starting an interpreter — that fast path is what keeps this
	// hook off the fleet's critical path. It must not be reachable by a
	// spelling the PARSER would have refused: a JSON-escaped `bd` decodes
	// after the substring test, so the escape keeps the call on the slow
	// path (delete the `*'\u'*` arm and this is waved through).
	if r := runGateRaw(t, nil, `{"tool_name":"Bash","tool_input":{"command":"\u0062d daemon stop"}}`); denied(t, r) == "" {
		t.Errorf("a JSON-escaped bd must still reach the parser, got code=%d out=%q", r.code, r.stdout)
	}

	// And a call for another tool is never this gate's business, even when
	// its arguments happen to spell bd.
	if r := runGateRaw(t, nil, `{"tool_name":"Read","tool_input":{"file_path":"/x/bd/y"}}`); r.code != 0 || r.stdout != "" {
		t.Errorf("a non-Bash tool call must pass untouched, got code=%d out=%q", r.code, r.stdout)
	}

	// With the parser unavailable, an escaped spelling must fail closed the
	// same way the literal one does — the fallback grep had the same blind
	// spot as the fast path (ranger-base-hthx).
	for _, env := range [][]string{
		{"BD_ARGV_GATE_PY=" + missing},
		{"BD_ARGV_GATE_PYTHON=" + filepath.Join(t.TempDir(), "no-python3")},
	} {
		for _, spelling := range []string{
			`b\d daemon stop`, "b''d daemon stop", `b"d" daemon stop`,
			`cd /tmp && b\d admin reset`,
		} {
			if r := runGate(t, env, spelling); r.code != 2 || !strings.Contains(r.stderr, "fail closed") {
				t.Errorf("%v: %q must be refused when the parser is unavailable: code=%d err=%q",
					env, spelling, r.code, r.stderr)
			}
		}
	}
}

// TestQABdArgvGateFastPathIsLooserThanTheParser pins the wrapper's ONE
// obligation to the parser: the fast path may answer without starting an
// interpreter only for payloads the parser would have had nothing to say
// about. Anything it refuses must reach it.
//
// The bug this pins (ranger-base-hthx): the fast path tested for a literal
// `bd` substring, on the stated premise that "a payload with no `bd` in it at
// all cannot produce any refusal below". False — the parser resolves the
// command word with shlex FIRST, then asks whether its basename is bd, so
// every spelling the shell concatenates into bd was refused by the parser and
// never reached it. MEASURED live against the installed copy: `bd ship --help`
// was refused and `b\d ship --help` ran, printing bd's help, exit 0.
//
// This asserts AGREEMENT between the two programs rather than "was refused":
// a table of expected refusals can go green because the wrapper got stricter
// in some unrelated way, while agreement can only go green if the fast path
// actually deferred. Each row also carries a positive witness — the parser
// must genuinely refuse it — so a fixture that stopped discriminating fails
// instead of passing quietly.
func TestQABdArgvGateFastPathIsLooserThanTheParser(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	// Each row names the fast-path arm that keeps it off the builtin answer.
	// Delete that arm from scripts/bd-argv-gate.sh and its rows must fail:
	// three of the four arms have a witness here, and the fourth is documented
	// in the script as unreachable under JSON escaping (a command's `"`
	// arrives as `\"`, so the backslash arm already covers it).
	for _, row := range []struct{ arm, command string }{
		{`*bd*`, "bd daemon stop"},
		{`*bd*`, "bd admin reset"},
		{`*b\\*`, `b\d daemon stop`},
		{`*b\\*`, `b\d ship --help`},
		{`*b\\*`, `/usr/local/bin/b\d admin reset`},
		{`*b\\*`, `cd /tmp && b\d daemon stop`},
		{`*b\\*`, `b"d" daemon stop`},   // a command `"` is `\"` in the payload
		{`*b\\*`, `"b""d" daemon stop`}, // ditto
		{`*b\'*`, "b''d daemon stop"},
		{`*b\'*`, "'b'd daemon stop"},
		{`*b\'*`, "b'd' daemon stop"},
	} {
		wrapper := denied(t, runGate(t, nil, row.command))
		parser := denied(t, parserGate(t, row.command))
		if parser == "" {
			t.Errorf("fixture measures nothing: the parser does not refuse %q, so the wrapper deferring to it proves nothing", row.command)
			continue
		}
		if wrapper != parser {
			t.Errorf("fast path (arm %s) answered %q itself: wrapper said %q, parser said %q",
				row.arm, row.command, wrapper, parser)
		}
	}

	// Why the fourth arm (*b\"*) has no row above, pinned rather than
	// asserted in a comment: a JSON string escapes a double quote, so a
	// command's `b"` reaches the wrapper as `b\"` and the backslash arm
	// already covers it. The arm stays as defence for a payload that is not
	// JSON-escaped. If the harness ever stops escaping, THIS fails, and the
	// arm becomes load-bearing with a witness — which is the whole point of
	// pinning the reason a redundancy is a redundancy.
	payload, err := json.Marshal(map[string]any{"command": `b"d daemon stop`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `b\"`) {
		t.Errorf("a command's double quote must arrive escaped, got %s", payload)
	}
	if strings.Contains(string(payload), `b"d`) {
		t.Errorf("an unescaped `b\"` in the payload would make the *b\\\"* arm load-bearing, got %s", payload)
	}

	// The control the whole test rests on: the fast path must still BE a fast
	// path. These carry no spelling of bd, so both programs stay silent and
	// the wrapper never starts an interpreter for them.
	for _, command := range []string{
		"go test ./...", "git status", "brew upgrade", "make verify-bd-pin",
		"grep -rn 'lib' .", `sed 's/a/b/' f`, "cd /tmp && ls",
	} {
		if wrapper, parser := denied(t, runGate(t, nil, command)), denied(t, parserGate(t, command)); wrapper != "" || parser != "" {
			t.Errorf("must be left alone: %s -> wrapper %q, parser %q", command, wrapper, parser)
		}
	}
}

// ─── ranger-base-4txk: LIVE DEFECT, GREEN ON PURPOSE ────────────────────────
//
// Found verifying the close of ranger-base-3bqn (ranger-base-7ol6). The pin
// below asserts what the shipped gate DOES, not what it should do, per
// NOTES.md on silent reverts: a defect with no pin is a defect whose fix
// nobody can date. It names the bead that closes it and says how to invert
// it, and goes red the day that bead lands, which is the point.
//
// Its sibling, TestQABdArgvGateFastPathIsReachableByAShellSpelling, pinned
// ranger-base-hthx the same way and has been INVERTED as its own comment
// instructed: the fast path defers now, so those spellings sit in
// TestQABdArgvGateResolvesTheVerb's refused table above, with
// TestQABdArgvGateFastPathIsLooserThanTheParser holding the general property
// and `make verify-bd-argv-gate` sweeping the whole quoting alphabet.

// parserGate runs the PARSER directly, skipping the sh wrapper. The two must
// agree; where they do not, the wrapper is the fence and the parser is right.
func parserGate(t *testing.T, command string) gateResult {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "-S", "-E", filepath.Join("scripts", "bd-argv-gate.py"))
	cmd.Stdin = strings.NewReader(string(payload))
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running the parser: %v", err)
		}
		code = ee.ExitCode()
	}
	return gateResult{code: code, stdout: out.String(), stderr: errb.String()}
}

// LIVE DEFECT ranger-base-4txk, the other direction: the gate refuses ordinary
// lines. resolve_verb returns the first token that does not start with "-",
// and shlex hands it ">" and "2>" as ordinary tokens, so a bd call whose only
// non-flag word is a redirect resolves to the redirect. And segments() splits
// on "|" without tracking "$(", so a substitution containing a pipe becomes a
// fragment whose command word is a $-variable — refused whenever bd appears
// ANYWHERE on the line, because that guard is line-scoped. Each row carries
// the control it differs from by one thing, so this measures the mechanism
// rather than the weather.
//
// TO INVERT when ranger-base-4txk lands: move the refusedToday rows into
// TestQABdArgvGateResolvesTheVerb's allowed list and delete this test.
func TestQABdArgvGateRefusesOrdinaryLines(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	for _, c := range []struct{ refusedToday, control string }{
		{`bd --help > /tmp/o`, `bd list > /tmp/o`},
		{`bd --version 2>&1`, `bd version 2>&1`},
		{`P=$(echo "$PATH" | tr ':' '\n'); grep -n bd f`, `P=$(echo "$PATH" | tr ':' '\n'); grep -n xx f`},
	} {
		if denied(t, runGate(t, nil, c.control)) != "" {
			t.Errorf("control %q must pass — the pair is only discriminating while it does", c.control)
		}
		if denied(t, runGate(t, nil, c.refusedToday)) == "" {
			t.Errorf("ranger-base-4txk looks FIXED for %q — invert this test: move these rows into "+
				"TestQABdArgvGateResolvesTheVerb's allowed list and delete it", c.refusedToday)
		}
	}
}
