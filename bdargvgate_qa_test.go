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

// runGateAt is runGate with the payload field the close arm reads: the
// directory the call is being made FROM. Separate rather than a widened
// runGate because every other pin in this file is about a command line alone,
// and a cwd on those payloads would be a fixture nothing reads
// (ranger-base-3xgc7, ADR 0041 §3).
func runGateAt(t *testing.T, env []string, command, cwd string) gateResult {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"session_id":      "qa",
		"hook_event_name": "PreToolUse",
		"cwd":             cwd,
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": command},
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

		// A FLAG'S VALUE IS PART OF THE FLAG (ranger-base-il8u). pflag takes
		// `--flag=value` as the SAME flag, and SUBDENY's opts were scanned by
		// exact token membership: `bd sync --full` was refused (the row
		// above) and `bd sync --full=true` — pull, merge, export, commit,
		// PUSH — was waved through, at the layer the operator's PreToolUse
		// hook actually runs. The property over the whole SUBDENY table is
		// TestQABdArgvGateRefusesEveryDeniedOptInBothSpellings below; these
		// rows are here because this is the table people read, and because
		// the escaped one exercises the sh WRAPPER's slow path for a `=`
		// spelling, which nothing else does.
		"bd sync --full=true":           "sync --full=true",
		"bd sync --dry-run --full=true": "sync --full=true",
		`b\d sync --full=true`:          "sync --full=true",

		// A REDIRECTION IS PUNCTUATION, AND IT MUST NOT HIDE A VERB
		// (ranger-base-4txk). The fix that stopped reading `>` as the verb had
		// to consume the redirect's TARGET too, and had to reach the command
		// word as well as the verb — `> /tmp/o bd daemon stop` resolved its
		// command word to `>` and RAN through the shipped fence (MEASURED).
		// Each row is refused for the verb bd would really have run: skip one
		// token too many and the reason names `/tmp/o` or `reset` instead, so
		// these fail on an over-eager skip as loudly as on no skip at all.
		"bd > /tmp/o daemon stop":           "daemon",
		"bd >/tmp/o admin reset":            "admin",
		"bd --json 2>/dev/null daemon stop": "daemon",
		"> /tmp/o bd daemon stop":           "daemon",
		"bd 2>&1 daemon stop":               "daemon",
		// …and the substitution that is no longer split must not swallow the
		// rest of the line with it.
		`X=$(echo hi | wc -l); bd daemon stop`: "daemon",

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

		// The `=` spelling is stripped to the flag NAME and then matched by
		// MEMBERSHIP (ranger-base-il8u) — not by prefix, and not by substring.
		// These keep the fix from degenerating into "refuse any argument with
		// --full in it", each red under a different wrong shape of the scan
		// (MUTATION-CHECKED, one arm at a time):
		//   startswith  -> `--fullish` is a DIFFERENT flag, and is refused
		//   `o in t`    -> a commit message that merely quotes the flag
		// The third row is the measurement this bead rests on, standing as a
		// regression: `--json=true` prints exactly what `--json` prints, and a
		// verb with no SUBDENY entry gets no opinion from the gate either way.
		"bd sync --fullish=1",
		"bd sync -m 'not --full but close'",
		"bd list --limit 1 --json=true",
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

		// ORDINARY LINES THE GATE USED TO REFUSE (ranger-base-4txk, inverted
		// from TestQABdArgvGateRefusesOrdinaryLines, which measured exactly
		// these). resolve_verb returned the first token that did not start
		// with `-`, and shlex hands it `>` and `2>` as ordinary tokens, so a
		// bd call whose only non-flag word was a redirect resolved to the
		// redirect. Each is paired with the control it differed from by one
		// token — the control passed all along, which is what made the pair
		// evidence about the parser rather than about the weather.
		"bd --help > /tmp/o", "bd list > /tmp/o",
		"bd --version 2>&1", "bd version 2>&1",
		"bd list >/tmp/o 2>&1",
		"bd show x 2>/dev/null | head -3",
		"bd --json --no-daemon ready >> /tmp/o",

		// And the other mechanism: segments() split on `|` without tracking
		// `$(`, so a substitution containing a pipe became a fragment whose
		// command word was `$PATH`, which verdict()'s LINE-scoped `$`-variable
		// arm then refused for any line that also named bd. The second row is
		// the fleet's standard PATH-stripping preamble, refused verbatim in
		// the session that found this. Its one-word control (`bd` -> `xx`)
		// passed, so the word bd elsewhere on the line was the whole
		// difference. The guard itself is untouched: `BD=bd; $BD daemon stop`
		// stays in the refused table above.
		`P=$(echo "$PATH" | tr ':' '\n'); grep -n bd f`,
		`NEWPATH=$(echo "$PATH" | tr ':' '\n' | grep -v gates | paste -sd: -); PATH="$NEWPATH" go test ./internal/posse/ ; grep -n '"bd": {' gates.go`,
		"echo $(date | wc -l) && grep -rn bd internal/",
		// These two are what hold segments()' `$(`-depth and backtick arms
		// specifically: the fragment a split leaves behind starts with a
		// $-variable, which masking the substitution afterwards cannot repair
		// because the split already happened. Delete either arm and only these
		// go red (MUTATION-CHECKED, one revert per arm — every arm of this fix
		// has a row here that notices it).
		"SEEN=$(git log | $PAGER -n 1); grep -n bd f",
		"X=`git log | $PAGER -n 1`; grep -n bd f",
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
		//
		// The last three rows are the ones that make this control bite. The
		// first three never reach the fail-closed test at all — the fast path
		// answers them from a shell builtin, so they would stay green against
		// a fallback that refused literally everything it saw. These carry a
		// `bd` the fast path cannot rule out (a substring, a hyphenated name)
		// and a newline, so they are decided by the fallback itself.
		for _, safe := range []string{
			"go test ./...", "git status", "echo hello",
			"./scripts/verify-bd-dep-safety.sh ranger-base-f0y3",
			"grep -rn lambda .\ncat abd.txt",
			"echo one\ngit status",
		} {
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
	//
	// The multi-line rows are ranger-base-1lvm, and they are the whole reason
	// this table now carries a newline. The fallback reasoned about the
	// payload's TEXT, where the harness writes a newline as the two characters
	// `\` `n` — so the character standing in front of `bd` was the `n` of the
	// escape, which is a word character, and the raw grep missed it; deleting
	// backslashes then joined that same `n` on to make `anbd`, and the
	// stripped grep missed it too. One newline was the entire difference
	// between a refusal and a wave-through, on exactly the destructive verbs
	// the allow-list is the only fence for. MEASURED: 228 escapes in 47275
	// real command lines, every one an ordinary multi-line command.
	//
	// Every corpus that has ever been pointed at this fallback — this table,
	// and `make verify-bd-argv-gate` — was single-line, which is why three
	// separate reviews of it agreed it was sound. Keep a newline in here.
	for _, env := range [][]string{
		{"BD_ARGV_GATE_PY=" + missing},
		{"BD_ARGV_GATE_PYTHON=" + filepath.Join(t.TempDir(), "no-python3")},
	} {
		for _, spelling := range []string{
			`b\d daemon stop`, "b''d daemon stop", `b"d" daemon stop`,
			`cd /tmp && b\d admin reset`,
			"echo hi\nbd daemon stop",
			"echo hi\nb\\d admin reset",
			"A=1\necho two\nbd jira sync",
			"echo hi\nb''d daemon stop",
			// Not a tab: `echo hi<TAB>bd daemon stop` is one segment running
			// echo with arguments, so the parser is rightly silent and such a
			// row would witness nothing (MEASURED — the guard below caught it).
		} {
			if r := runGate(t, env, spelling); r.code != 2 || !strings.Contains(r.stderr, "fail closed") {
				t.Errorf("%v: %q must be refused when the parser is unavailable: code=%d err=%q",
					env, spelling, r.code, r.stderr)
			}
			// The row is only a witness if the parser really refuses it —
			// otherwise the fallback refusing it proves nothing about the
			// fallback being no looser than the parser.
			if parser := denied(t, parserGate(t, spelling)); parser == "" {
				t.Errorf("fixture measures nothing: the parser does not refuse %q", spelling)
			}
		}
	}

	// The unicode escapes for `b` and `d` are the ONLY four-hex spellings that
	// can decode to a b or a d — the hex letters a-f never occur in either —
	// so the fallback decodes exactly those two and turns every other one into
	// a separator, rather than refusing any payload that carries a unicode
	// escape at all. That distinction is not cosmetic: node's JSON.stringify
	// leaves `&<>` alone while Go's encoding/json escapes them, so a blanket
	// refusal costs 0.06% of real commands under one encoder and 72% under the
	// other (MEASURED on 47275 commands). Deciding it per-spelling makes the
	// answer identical under both, which is the property worth having.
	//
	// esc is composed rather than written out so that nothing in the toolchain
	// can normalize the sequence under test out of this file, and the payloads
	// are raw because Go's marshaller would escape the backslash and destroy
	// the very spelling being pinned.
	esc := `\u`
	for _, row := range []struct {
		name    string
		command string // the JSON string body, escapes left undecoded
		refused bool
	}{
		{"b spelled as a unicode escape", esc + "0062" + "d daemon stop", true},
		{"d spelled as a unicode escape", "b" + esc + "0064" + " daemon stop", true},
		{"both, and on the second line", `echo hi\n` + esc + "0062" + esc + "0064" + " daemon stop", true},
		{"escapes that are neither b nor d", "echo " + esc + "0041" + esc + "0042", false},
	} {
		payload := `{"tool_name":"Bash","tool_input":{"command":"` + row.command + `"}}`
		r := runGateRaw(t, []string{"BD_ARGV_GATE_PY=" + missing}, payload)
		if got := r.code == 2; got != row.refused {
			t.Errorf("%s (%s): with the parser unavailable, refused=%v want %v (code=%d err=%q)",
				row.name, payload, got, row.refused, r.code, r.stderr)
		}
	}

	// A payload that is not JSON at all is the same question asked of text
	// nobody encoded, so a backslash there is the shell's quoting rather than
	// a JSON escape — and having failed to read the payload the gate cannot
	// tell which. It refuses under either reading (ranger-base-1lvm, qa's
	// third site: BD_WORD against raw payload text appears in the wrapper
	// fallback and in the parser's own unreadable-payload path).
	for _, row := range []struct {
		name, payload string
		refused       bool
	}{
		{"not json, bd on line 1", "this is not json, bd daemon stop", true},
		{"not json, bd on line 2 as \\n", `this is not json\nbd daemon stop`, true},
		{"not json, bd on line 2, real newline", "this is not json\nbd daemon stop", true},
		{"not json, concatenated b\\d", `this is not json, b\d daemon stop`, true},
		{"not json, no bd at all", "this is not json at all", false},
	} {
		for _, env := range [][]string{nil, {"BD_ARGV_GATE_PY=" + missing}} {
			r := runGateRaw(t, env, row.payload)
			if got := r.code == 2; got != row.refused {
				t.Errorf("%s (env %v): refused=%v want %v (code=%d err=%q)",
					row.name, env, got, row.refused, r.code, r.stderr)
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
	// No row here may be spelled with & < or >: Go's encoding/json escapes
	// those as \u00xx and the wrapper keeps any \u payload on the slow path,
	// so such a row is held by the *'\u'* arm and witnesses nothing about the
	// arm it claims (measured — `cd /tmp && b\d …` marshals with \u0026, and
	// was in the first cut of this table mislabelled as a b\ witness). The
	// compound and redirect spellings are swept by `make verify-bd-argv-gate`,
	// which builds its payloads with python's json.
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

// TestQABdArgvGateRefusesEveryDeniedOptInBothSpellings pins ranger-base-il8u:
// SUBDENY's opts were scanned by exact token membership, and pflag takes
// `--flag=value` as the same flag. MEASURED against the shipped parser, one
// JSON payload per line on stdin:
//
//	bd sync --full         -> deny
//	bd sync --push --full  -> deny        (the flag has no position, vct2)
//	bd sync --full=true    -> NO output, exit 0
//
// `bd sync --full` is the one spelling of sync that pulls, merges, exports,
// commits and PUSHES, and `--json=true` was measured printing exactly what
// `--json` prints, so the third line is that sync with the fence asleep. The
// L1 shim grew this arm in ranger-base-vct2; L2 is the layer the operator's
// PreToolUse hook actually runs, and it still had the hole — which also
// retires vct2's claim that L2 had closed it on claude. A layer is credited
// with the spellings someone tested, not with the ones nobody typed.
//
// The table is read OUT OF THE PARSER rather than restated here: an opt added
// to SUBDENY is covered the day it lands, not the day someone remembers this
// file. Every opt is asserted in five spellings and four positions, and each
// arm carries its near miss — `--fullish=1` must stay silent, so a scan
// written as a prefix or substring test fails here rather than passing by
// being indiscriminate.
func TestQABdArgvGateRefusesEveryDeniedOptInBothSpellings(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	table := subdenyOpts(t)
	// A table that came back empty — a rename, a botched edit, an import that
	// half-worked — makes every loop below vacuously green. That is the same
	// "pass" a deleted fence produces, so it is a failure here.
	if len(table) == 0 {
		t.Fatal("no SUBDENY opts came back from the parser; this test would measure nothing")
	}
	for verb, opts := range table {
		for _, opt := range opts {
			// `--full=false` DISABLES the flag and is refused anyway. Reading
			// the value means reimplementing strconv.ParseBool's spellings of
			// false, and a fence that argues about truthiness is one
			// respelling from being wrong — so the wall stands wider than the
			// rule, exactly as it does at L1. Pinned so that widening is a
			// decision someone made, not an accident someone can quietly undo.
			for _, spelling := range []string{
				opt, opt + "=true", opt + "=false", opt + "=1", opt + "=",
			} {
				// A flag has no position: before the denied one, after it, and
				// with a GLOBAL option moved in front of the verb, which is the
				// reordering this whole gate exists for (ranger-base-3bqn).
				for _, command := range []string{
					"bd " + verb + " " + spelling,
					"bd " + verb + " --dry-run " + spelling,
					"bd " + verb + " " + spelling + " --json",
					"bd --no-daemon " + verb + " " + spelling,
				} {
					reason := denied(t, runGate(t, nil, command))
					if reason == "" {
						t.Errorf("MUST be refused, was waved through: %s", command)
						continue
					}
					// Naming the token as TYPED, not the canonical flag: the
					// operator reading the refusal is looking for the thing
					// they wrote.
					if want := verb + " " + spelling; !strings.Contains(reason, want) {
						t.Errorf("refusal of %q must name %q, said: %s", command, want, reason)
					}
				}
			}
			for _, near := range []string{opt + "ish", opt + "ish=1", opt + "-x=1"} {
				command := "bd " + verb + " " + near
				if reason := denied(t, runGate(t, nil, command)); reason != "" {
					t.Errorf("%q is a DIFFERENT flag and must pass through, was refused: %s",
						command, reason)
				}
			}
		}
	}
}

// subdenyOpts reads SUBDENY's option tokens out of the parser itself. Import,
// not a regexp over the source: a table restated in a test is a table that
// drifts, and the drift is always in the direction of the test still passing.
// -B and dont_write_bytecode keep a __pycache__ out of scripts/, the same
// courtesy verify-bd-argv-gate.sh pays for the same import.
func subdenyOpts(t *testing.T) map[string][]string {
	t.Helper()
	const prog = `import importlib.util, json, sys
sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("gate", "scripts/bd-argv-gate.py")
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
json.dump({k: sorted(v["opts"]) for k, v in m.SUBDENY.items() if v["opts"]}, sys.stdout)
`
	cmd := exec.Command("python3", "-B", "-S", "-E", "-c", prog)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("reading SUBDENY out of the parser: %v (%s)", err, errb.String())
	}
	var table map[string][]string
	if err := json.Unmarshal([]byte(out.String()), &table); err != nil {
		t.Fatalf("SUBDENY did not come back as JSON: %q (%v)", out.String(), err)
	}
	return table
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

// ranger-base-1lvm, found verifying ranger-base-hthx's close (ranger-base-nn0n).
// The fast path above is fixed and I could not break it. This is the OTHER
// half of that close — the fail-closed fallback, the arm taken when the parser
// did not run at all. 1lvm is FIXED and this pin is live: it was parked while
// the defect was open, and the skip outlived the fix by four days
// (ranger-base-x3pmq lifted it).
//
// The defect it holds down: the fallback reasoned about the JSON payload's
// TEXT while the parser reasons about the DECODED command, and the two
// disagreed about a newline. A newline in a command arrives as the two
// characters backslash + n, so in `hi\nbd daemon stop` the character before
// `bd` is the `n` of the escape — which is in the excluded class, so the raw
// grep missed it; and stripping the backslash JOINS that n to bd, giving
// `hinbd`, so the stripped grep missed it too. Both forms were there precisely
// to cover each other and neither covered this. A bd call that is not on the
// FIRST LINE of the command was waved through whenever the parser is
// unavailable, which is the state the fallback exists for. The decoding arms
// above bd_word in scripts/bd-argv-gate.sh are the fix; restore that `if` to
// its two pre-fix text greps and all nine rows below go red (MEASURED under
// x3pmq, control still green).
//
// The control arm is the whole finding and must stay: the SAME verb on line
// one is refused today, so a green run of this test can only mean the newline
// case was fixed, not that the fallback started refusing everything.
func TestQABdArgvGateFallbackSeesBdPastTheFirstLine(t *testing.T) {
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
		for _, command := range []string{
			"echo hi\nbd daemon stop",
			"echo hi\nb\\d admin reset",
			"cd /tmp\nbd delete some-id",
		} {
			// Positive witness: a fixture that stopped naming a denied verb
			// would let this pass while measuring nothing.
			if denied(t, parserGate(t, command)) == "" {
				t.Fatalf("fixture: the parser must refuse %q for this to be about the fallback", command)
			}
			if r := runGate(t, env, command); r.code != 2 || !strings.Contains(r.stderr, "fail closed") {
				t.Errorf("%v: %q must be refused when the parser is unavailable: code=%d err=%q",
					env, command, r.code, r.stderr)
			}
		}
		// CONTROL: the same verb on the first line already fails closed, so
		// the newline is the entire difference.
		if r := runGate(t, env, "bd daemon stop"); r.code != 2 {
			t.Errorf("%v: control: a first-line bd call must fail closed, got code=%d", env, r.code)
		}
	}
}

// ranger-base-uxuy: A HEREDOC BODY IS DATA, NOT COMMAND LINES.
//
// segments() split the whole typed string on `\n`, so every line of every
// heredoc body became a segment with a command word of its own. A line of
// ENGLISH that opened with the tracker's name therefore resolved as an
// invocation and was refused — MEASURED live by security, twice in one session,
// appending a lesson to ORDERS.md:
//
//	cat >> ORDERS.md <<'EOF'
//	bd owns the store; nobody else writes it.
//	EOF
//	-> "`bd owns` is not on the gate's allow-list"
//
// `owns` is not one of bd's verbs, there was no bd binary on the line, and
// argv[0] was cat. The POSITION of the two words was the entire trigger: the
// same sentence with one word in front of it passed all along (both spellings
// are in the table below, so a fix that merely stopped matching the word
// `owns` goes red here). A wall that refuses legitimate work teaches the crew
// to spell around it, and this one taught that lesson twice in an hour.
//
// What replaces the line split is four questions a body can honestly answer,
// each with its own refused row: does the segment's own command word EXECUTE
// the body (`sh <<'EOF'`); does anything ELSE on the line (`cat <<'EOF' | sh`,
// where the body's consumer is a different segment from its opener); is the
// delimiter UNQUOTED, so the shell runs a substitution in the body before
// anyone reads it; and is a real invocation still standing outside the body.
//
// The shell facts these arms rest on are MEASURED, not assumed (bash 3.2.57,
// darwin 25.4.0):
//
//	cat <<'ZZZ' / echo RAN   -> printed "echo RAN". An unterminated heredoc is
//	                            read to end of input and is DATA; nothing after
//	                            it runs, so absorbing it as body is what bash
//	                            does.
//	(( 1 << 2 )) / echo RAN  -> printed RAN. `<<` inside arithmetic is a left
//	                            shift, not a heredoc — which is why segments()
//	                            consumes `(( … ))` whole. Delete that arm and
//	                            the arithmetic row below swallows the real
//	                            invocation that follows it.
//	cat <<-ZZZ               -> strips TABS only, terminator line included.
//	cat <<'A' <<'B'          -> BOTH bodies are consumed, in the order the
//	                            redirections appear, and the command still runs.
//	cat <<'Z' / $(echo X)    -> printed the substitution literally.
//	cat <<Z   / $(echo X)    -> printed X. Quoting the delimiter is the whole
//	                            difference, and it is what the `expands` arm
//	                            keys on.
func TestQABdArgvGateReadsAHeredocBodyAsData(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}

	// Each row must PASS. `witness` is a command that must be REFUSED — the
	// same text with the heredoc taken off it — so a row that passes because
	// the gate went silent about everything is not mistaken for a row that
	// passes because the body is read as data (rig must be shown able to
	// fail). A row with no witness is one that passed before this fix too,
	// and is here to hold the ground it already had.
	passes := []struct{ cmd, witness string }{
		// THE FILED REPRO, in the two positions that used to differ.
		{"cat >> /tmp/x <<'EOF'\nbd owns this file\nEOF", "bd owns this file"},
		{"cat >> ORDERS.md <<'EOF'\nlesson: only the operator promotes.\nbd owns the store; nobody else writes it.\nEOF",
			"bd owns the store"},
		{"cat >> /tmp/x <<'EOF'\nremember that bd owns the store\nEOF", ""},

		// The two other spellings the same session was refused for: a body
		// that quotes the command form, and a body with an apostrophe in it.
		// Both were "bd behind a construct this gate cannot read" and "this
		// segment could not be parsed (unterminated quote)" — refusals aimed
		// at a shell construct, delivered about a sentence.
		{"cat >> ORDERS.md <<'EOF'\nrun `bd show <id>` first\nEOF", "run `bd show <id>` first"},
		{"cat >> ORDERS.md <<'EOF'\nbd's own stderr is the evidence\nEOF", "bd's own stderr is the evidence"},

		// A runbook that writes the command form of a DENIED verb into a
		// file. Nothing on this line runs it; a script that calls bd later is
		// the indirection this layer has never held and says so in its
		// docstring. The witness is the same text as a command, which is
		// refused — so this row is exactly the data/command distinction.
		{"cat > /tmp/runbook.md <<'EOF'\nbd daemon stop\nEOF", "bd daemon stop"},

		// The redirection spellings, each of which has to survive
		// tokenization as well as segmentation: `<<-` with tabs, a space
		// before the delimiter, a double-quoted delimiter, an escaped one,
		// two heredocs on one command, and a heredoc read by bd itself.
		{"cat <<-'EOF' > /tmp/x\n\tbd owns this\n\tEOF", "bd owns this"},
		{"cat << 'EOF' > /tmp/x\nbd owns this\nEOF", "bd owns this"},
		{"cat <<\"EOF\" > /tmp/x\nbd owns this\nEOF", "bd owns this"},
		{"cat <<\\EOF > /tmp/x\nbd owns this\nEOF", "bd owns this"},
		{"cat <<'A' <<'B' > /tmp/x\nbd owns a\nA\nbd owns b\nB", "bd owns a"},
		{"bd comments add ranger-base-uxuy -f - <<'EOF'\nbd owns the store\nEOF", "bd owns the store"},

		// An UNQUOTED body is expanded, so it is refused only when it carries
		// something to expand. Plain prose in one is still prose.
		{"cat > /tmp/x <<EOF\nbd owns the store\nEOF", "bd owns the store"},

		// An unterminated heredoc absorbs the rest, because bash does: the
		// measurement above printed the trailing line instead of running it.
		{"cat > /tmp/x <<'EOF'\nbd owns the store", "bd owns the store"},

		// A NON-SHELL interpreter's heredoc. The body is a program, so this
		// looks like the strictest possible reading would refuse it — and the
		// first cut did, at a MEASURED cost of 130 real command lines off this
		// box, every one a `python3 - <<'PY'` source edit whose body quoted
		// the tracker's name inside a string it was splicing into a file. It
		// would also have been incoherent, and that is the part that decided
		// it: the two rows below already pass this gate and always have, so
		// the heredoc spelling alone would have been the strict one for no
		// reason a reader could state. The line is the shell family.
		{"python3 - <<'PY'\ns = s.replace('bd daemon stop', 'x')\nPY", ""},
		{"python3 -c \"import os; os.system('bd daemon stop')\"", ""},
		{"python3 /tmp/thing.py bd daemon stop", ""},

		// A here-STRING is not a heredoc and opens no body.
		{"cat <<< 'bd owns the store'", ""},
	}
	for _, row := range passes {
		if reason := denied(t, runGate(t, nil, row.cmd)); reason != "" {
			t.Errorf("a heredoc body is data and must pass: %q -> %s", row.cmd, reason)
		}
		if row.witness == "" {
			continue
		}
		if denied(t, runGate(t, nil, row.witness)) == "" {
			t.Errorf("this row measures nothing: its witness %q is not refused on its own, "+
				"so the row would pass over a gate that refused nothing at all", row.witness)
		}
	}

	// And the other half. Every one of these carries the same body text as a
	// row above; what differs is who reads it.
	refused := map[string]string{
		// The body IS the program for its consumer.
		"sh <<'EOF'\nbd daemon stop\nEOF":          "sh",
		"bash <<EOF\nbd admin reset\nEOF":          "bash",
		"zsh <<-'EOF'\n\tbd delete some-id\n\tEOF": "zsh",
		"dash <<'EOF'\nbd daemon stop\nEOF":        "dash",
		"env bash <<'EOF'\nbd admin reset\nEOF":    "bash",
		// …and the consumer is not always the opener.
		"cat <<'EOF' | sh\nbd daemon stop\nEOF":        "sh",
		"cat <<'EOF' | bash -s\nbd admin reset\nEOF":   "bash",
		"cat <<'EOF' > /tmp/x | zsh\nbd delete x\nEOF": "zsh",

		// An UNQUOTED delimiter expands the body, so a substitution in it runs
		// before anything reads it. The quoted spelling of this exact command
		// is a passing row above.
		"cat > /tmp/x <<EOF\n$(bd daemon stop)\nEOF": "$(",
		"cat > /tmp/x <<EOF\n`bd admin reset`\nEOF":  "`",

		// A real invocation outside the body is still a real invocation,
		// whether it comes after the terminator or before the opener.
		"cat > /tmp/x <<'EOF'\nprose\nEOF\nbd daemon stop":   "daemon",
		"bd daemon stop\ncat > /tmp/x <<'EOF'\nprose\nEOF":   "daemon",
		"cat > /tmp/x <<'EOF' && bd admin reset\nprose\nEOF": "admin",

		// `<<` in arithmetic is a left shift. Read it as a heredoc opener and
		// the "body" runs to a line reading `2` — which is never — swallowing
		// every command after it. Delete segments()' `(( … ))` arm and this
		// row is the one that notices.
		"(( a << 2 ))\nbd daemon stop": "daemon",
		"(( 1 << 2 )); bd admin reset": "admin",

		// A here-string is punctuation, not a body.
		"bd daemon stop <<< x": "daemon",
	}
	for command, want := range refused {
		reason := denied(t, runGate(t, nil, command))
		if reason == "" {
			t.Errorf("MUST be refused, was waved through: %q", command)
			continue
		}
		if !strings.Contains(reason, want) {
			t.Errorf("refusal of %q must name %q, said: %s", command, want, reason)
		}
	}
}

// ranger-base-uxuy, the second half of the bead: the refusal has to say WHAT
// was matched and WHERE, so the next false positive is diagnosable from the
// message alone.
//
// The filed one was not. It said "`bd owns` is not on the gate's allow-list"
// and then, in boilerplate, "the fence is on the RESOLVED verb" — which reads
// as a claim about a bd invocation that was nowhere on the line, and sent the
// reporter looking for one. Naming the segment and its index turns the same
// refusal into a bug report: segment 2 of 3, and here is its text.
func TestQABdArgvGateRefusalNamesWhatMatchedAndWhere(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	for _, row := range []struct {
		command string
		want    []string
	}{
		{"bd daemon stop", []string{"resolved verb", "segment 1 of 1", "bd daemon stop"}},
		{"cat > /tmp/x <<'EOF'\nprose\nEOF\nbd admin reset",
			[]string{"resolved verb", "segment 2 of 2", "bd admin reset"}},
		{"ls /tmp && bd delete some-id && echo done",
			[]string{"resolved verb", "segment 2 of 3", "bd delete some-id"}},
		// The two heredoc spellings are one arm with two messages, and the
		// message is the whole diagnostic value: "sh EXECUTES this" and "sh is
		// also on this line" send the reader to different places. Collapse
		// them into one sentence and these two rows say so.
		{"sh <<'EOF'\nbd daemon stop\nEOF",
			[]string{"heredoc body", "segment 1 of 1", "bd daemon stop", "sh EXECUTES"}},
		{"cat <<'EOF' | sh\nbd daemon stop\nEOF",
			[]string{"heredoc body", "segment 1 of 2", "bd daemon stop", "also runs sh"}},
		{"cat <<'EOF' > /tmp/x\nbd daemon stop\nEOF\nbash /tmp/x",
			[]string{"heredoc body", "segment 1 of 2", "also runs bash"}},
		{"sh -c \"bd daemon stop\"", []string{"command word", "segment 1 of 1"}},
		{"BD=bd; $BD daemon stop", []string{"command word", "segment 2 of 2", "$BD"}},
		{"bd sync --full", []string{"resolved verb", "segment 1 of 1"}},
	} {
		reason := denied(t, runGate(t, nil, row.command))
		if reason == "" {
			t.Errorf("fixture measures nothing: %q is not refused", row.command)
			continue
		}
		for _, want := range row.want {
			if !strings.Contains(reason, want) {
				t.Errorf("the refusal of %q must name %q so it can be diagnosed from the "+
					"message alone; it said: %s", row.command, want, reason)
			}
		}
	}

	// The segment text is elided, not dumped: a refusal is read in a terminal.
	long := "bd daemon stop " + strings.Repeat("x", 400)
	reason := denied(t, runGate(t, nil, long))
	if reason == "" {
		t.Fatalf("fixture measures nothing: %q is not refused", long)
	}
	if strings.Contains(reason, strings.Repeat("x", 200)) {
		t.Errorf("a 400-character argument must not be echoed whole into the refusal: %s", reason)
	}
	if !strings.Contains(reason, "…") {
		t.Errorf("an elided segment must say so with an ellipsis: %s", reason)
	}
}

// ranger-base-3sv0b (found verifying ranger-base-x3pmq): EVERY ARM OF THE
// FAIL-CLOSED FALLBACK DECIDES SOMETHING, AND ONE OF THEM WAS PINNED BY
// NOTHING.
//
// The fallback is a three-armed `if` (scripts/bd-argv-gate.sh — arms A, B and
// C in the comment above it). For a disjunction the sharpest mutant is one
// arm at a time, and MEASURED here: replacing arm B's or arm C's
// `grep -Eq "$bd_word"` with a pattern that never matches reds a pin in this
// file, while the same edit to arm A leaves every TestQABdArgvGate* test
// green. Arm A is the arm the fix's own comment leads with.
//
// It is not an equivalent mutant. Over 14046 real Bash command lines
// harvested from the fleet's transcripts — the corpus the arm was measured on
// (ranger-base-1lvm) — arm A alone decides 67, and reading them says which
// class they are: every one carries a regex like `grep -rn "bd\b" f`, where
// the payload spells the escaped backslash `\\` right in front of `bd`. Arm A
// turns it into a separator and sees the word; arms B and C delete it and see
// `bdb`. So on real traffic arm A's unique contribution is the over-refusal
// the fix priced at 0.59% and accepted (279 of 47275, and only while the
// parser is down) rather than a caught escape — for an INVOCATION the
// character in front of that backslash is a word character exactly when the
// shell reads the run together as one word, which is not a bd call. That is a
// reason to hold the arm, not to leave it unmeasured: an arm nothing measures
// is one a later simplification deletes with the suite green, and the class of
// payload it decides changes silently.
//
// The rig mutates a COPY under t.TempDir and never touches the shipped script.
// Its control is that same copy unmutated: it must refuse all three payloads,
// or "the mutant let it through" below is an artifact of a broken copy rather
// than a measurement of the arm.
func TestQABdArgvGateEveryFallbackArmDecidesSomething(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	shipped, err := os.ReadFile(filepath.Join("scripts", "bd-argv-gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	// Each arm is anchored on its own spelling, not on a line number: the
	// three greps differ by the pipeline in front of them.
	arms := []struct {
		name   string
		anchor string // must appear exactly once in the shipped script
		// payload the SHIPPED script refuses and this arm's mutant does not.
		payload string
		raw     bool // send payload as-is instead of as a Bash tool call
	}{
		{
			name:    "A (decode, then a literal bd)",
			anchor:  `sed -e 's/\\./ /g' | grep -Eq "$bd_word" ||`,
			payload: `grep -rn "bd\b" internal/posse/dispatch.go`,
		},
		{
			name:    "B (decode, then a bd the shell concatenates)",
			anchor:  `tr -d '\\'\''"' | grep -Eq "$bd_word" ||`,
			payload: "echo hi\nb\\d admin reset",
		},
		{
			name:    "C (the text test, for a payload nobody encoded)",
			anchor:  `tr -d '\\'\''"' | grep -Eq "$bd_word"; then`,
			payload: `this is not json, b\d daemon stop`,
			raw:     true,
		},
	}

	dir := t.TempDir()
	missing := filepath.Join(dir, "not-a-parser.py")
	env := []string{"BD_ARGV_GATE_PY=" + missing}
	run := func(script, payload string, raw bool) gateResult {
		t.Helper()
		if !raw {
			b, err := json.Marshal(map[string]any{
				"session_id": "qa",
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": payload},
			})
			if err != nil {
				t.Fatal(err)
			}
			payload = string(b)
		}
		cmd := exec.Command("sh", script)
		cmd.Stdin = strings.NewReader(payload)
		cmd.Env = append(os.Environ(), env...)
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		code := 0
		if err := cmd.Run(); err != nil {
			ee, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("running %s: %v", script, err)
			}
			code = ee.ExitCode()
		}
		return gateResult{code: code, stdout: out.String(), stderr: errb.String()}
	}

	// The control: an unmutated copy, run the same way, refuses every row.
	control := filepath.Join(dir, "control.sh")
	if err := os.WriteFile(control, shipped, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, a := range arms {
		if r := run(control, a.payload, a.raw); r.code != 2 {
			t.Fatalf("control: the unmutated copy must refuse %q (the row for arm %s), got code=%d err=%q — "+
				"every mutant reading below would be an artifact", a.payload, a.name, r.code, r.stderr)
		}
	}

	for _, a := range arms {
		if n := strings.Count(string(shipped), a.anchor); n != 1 {
			t.Fatalf("arm %s: its grep is spelled %q %d times in the shipped script, want 1 — "+
				"the fallback was rewritten and this pin is aimed at nothing", a.name, a.anchor, n)
		}
		dead := strings.Replace(a.anchor, `"$bd_word"`, `'ZZ_NEVER_MATCHES_ZZ'`, 1)
		script := filepath.Join(dir, "mutant.sh")
		if err := os.WriteFile(script, []byte(strings.Replace(string(shipped), a.anchor, dead, 1)), 0o755); err != nil {
			t.Fatal(err)
		}
		if r := run(script, a.payload, a.raw); r.code != 0 {
			t.Errorf("arm %s: with its grep dead, %q is still refused (code=%d) — the row does not "+
				"discriminate this arm, so a deletion of it would go unnoticed", a.name, a.payload, r.code)
		}
		// And the OTHER arms still answer: a mutant that refuses nothing at
		// all would pass the check above while saying nothing about this arm.
		for _, other := range arms {
			if other.name == a.name {
				continue
			}
			if r := run(script, other.payload, other.raw); r.code != 2 {
				t.Errorf("arm %s's mutant also stopped refusing %s's row (%q, code=%d) — the edit "+
					"broke the script rather than disabling one arm", a.name, other.name, other.payload, r.code)
			}
		}
	}
}

// ─── ADR 0041 §3: the cooperative pre-close refusal (ranger-base-3xgc7) ──────
//
// One arm of this gate is not about the verb at all. `close` is on the
// allow-list and stays there; what the close arm asks is WHERE the call is
// being made from. ranger-base-yeg1 closed naming four deliverables and not
// one of them was committed — the branch's reflog is a single "Created from
// main" — and a day later a pin file carried three claims copied out of that
// close comment, all false at HEAD. Only commits leave a session tree, so a
// close typed over uncommitted paths claims work nothing carries.
//
// It is COOPERATIVE class (ADR 0025): held in-process, on the runtime's
// ordinary path and nothing stronger. `git checkout -- .` walks around it,
// and that is an explicit act by the one actor who knows whether the paths
// are work. The realized belt is ADR 0041 §1–2 (closeddirty.go), operator-
// side and under the launcher lock. So what this pins is the refusal, not an
// enforcement claim — and the class is asserted in the refusal's own text,
// because a reader who thinks a cooperative fence is a wall is the failure
// ADR 0025 was written about.
//
// THE FOUR ROWS ARE ADR 0041's Verification 4, plus the one it implies.
// A dirty session tree denies and names the path; a dirty SHARED checkout is
// silent (there the dirt is other writers, ADR 0022); a clean session tree is
// silent. The fourth is the one the second cannot cover: a dirty LINKED
// worktree on a branch that is not `posse/` is not a posse session tree, and
// only that row tells the branch arm apart from the linked arm.
//
// AND THE CONTROLS RIDE ALONG. A gate that refused every line would pass a
// table of expected refusals, so `bd list` from the dirty tree must stay
// silent and `bd daemon stop` must still be refused BY THE VERB FENCE —
// which is also where the two tails are told apart: the verb fence points at
// the allow-list, and pointing a dirty close there would be advice about the
// wrong thing entirely (`close` is on it).
func TestQABdArgvGateRefusesACloseFromADirtySessionTree(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	fx := newCloseGateTrees(t)

	for _, row := range []struct {
		name    string
		command string
		cwd     string
		want    string // "" = the gate must stay out of the way
	}{
		{"a dirty session tree refuses the close", "bd close x", fx.dirty, fx.dirtFile},
		{"a dirty shared checkout is silent", "bd close x", fx.shared, ""},
		{"a clean session tree is silent", "bd close x", fx.clean, ""},
		{"a dirty worktree off a non-posse branch is silent", "bd close x", fx.foreign, ""},
		{"a dirty MAIN checkout on a posse/ branch is silent", "bd close x", fx.onPosse, ""},
		{"control: a read verb from the dirty tree is silent", "bd list", fx.dirty, ""},
		{"control: the verb fence still fires there", "bd daemon stop", fx.dirty, "allow-list"},
	} {
		got := denied(t, runGateAt(t, nil, row.command, row.cwd))
		if row.want == "" {
			if got != "" {
				t.Errorf("%s: want silence, got a refusal: %s", row.name, got)
			}
			continue
		}
		if !strings.Contains(got, row.want) {
			t.Errorf("%s: refusal must name %q, got: %s", row.name, row.want, got)
		}
	}

	// The dirty close's refusal has to be readable and correctly labelled:
	// the paths, both resolutions, and the class. A refusal that says only
	// "refused" is the undiagnosable message ranger-base-uxuy filed.
	reason := denied(t, runGateAt(t, nil, "bd close x", fx.dirty))
	for _, want := range []string{
		// The path — and note WHICH path: dirt.txt is UNTRACKED. The
		// launcher's own dirty set counts untracked files (a stray
		// calls.log was one of the four closes ADR 0041 measured), so a
		// close arm that read only tracked changes would be silent on a
		// quarter of the class. This row is what says it does not.
		fx.dirtFile,
		"COMMIT",      // resolution one
		"DISCARD",     // resolution two
		"COOPERATIVE", // ADR 0025's class, said out loud
		"ADR 0041",    // where the realized belt is
	} {
		if !strings.Contains(reason, want) {
			t.Errorf("the dirty-close refusal must carry %q: %s", want, reason)
		}
	}
	// And NOT the verb fence's tail: `close` is on the allow-list, so
	// "file it — the allow-list is …" would send the closer to edit a file
	// that has nothing to do with why this call stopped.
	if strings.Contains(reason, "allow-list") {
		t.Errorf("the dirty-close refusal carries the VERB fence's tail, which points at the "+
			"allow-list `close` is already on: %s", reason)
	}

	// A payload with no cwd at all — a runtime that stops sending the field —
	// must leave the gate exactly as it was before ADR 0041 §3.
	if got := denied(t, runGate(t, nil, "bd close x")); got != "" {
		t.Errorf("with no cwd on the payload the close arm must not run, got: %s", got)
	}
}

// closeGateTrees is the fixture the rows above are read against: two main
// checkouts and three linked worktrees, built so that every discriminator the
// close arm uses has a pair of trees differing in exactly that one thing —
// which is what a mutant can be aimed at. Without the second main checkout
// (`onPosse`) the linked arm has no row of its own, because `shared` differs
// from a session tree in the branch as well.
type closeGateTrees struct {
	shared   string // the main checkout, DIRTY — the ADR 0022 row
	dirty    string // a linked worktree on posse/…, dirty
	clean    string // a linked worktree on posse/…, clean
	foreign  string // a linked worktree on feature/…, dirty
	onPosse  string // a MAIN checkout whose own branch is posse/…, dirty
	dirtFile string // the path the dirty trees hold, as status prints it
}

func newCloseGateTrees(t *testing.T) closeGateTrees {
	t.Helper()
	root := t.TempDir()
	shared := filepath.Join(root, "repo")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	// -c on every call, never `git config`: a global user.email, hooksPath or
	// gpgsign on the box the suite runs on must not decide whether this
	// fixture builds (the hooksPath class is why hookfreshness_qa_test.go
	// reads git's own answer instead of deriving one).
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{
			"-c", "user.email=probe@example.invalid", "-c", "user.name=probe",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main",
			"-c", "core.hooksPath=" + filepath.Join(root, "no-hooks"),
			"-C", dir,
		}, args...)...)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git -C %s %v: %v\n%s", dir, args, err, b)
		}
	}
	git(shared, "init", "-q", ".")
	if err := os.WriteFile(filepath.Join(shared, "kept.txt"), []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(shared, "add", "--", "kept.txt")
	git(shared, "commit", "-qm", "fixture", "--", "kept.txt")

	const dirt = "dirt.txt"
	fx := closeGateTrees{shared: shared, dirtFile: dirt}
	for _, w := range []struct {
		field  *string
		dir    string
		branch string
		dirty  bool
	}{
		{&fx.dirty, "wt-dirty", "posse/probe-dirty", true},
		{&fx.clean, "wt-clean", "posse/probe-clean", false},
		{&fx.foreign, "wt-foreign", "feature/probe", true},
	} {
		path := filepath.Join(root, w.dir)
		git(shared, "worktree", "add", "-q", "-b", w.branch, path)
		if w.dirty {
			if err := os.WriteFile(filepath.Join(path, dirt), []byte("not committed\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		*w.field = path
	}
	// The shared checkout is dirty too, and it is dirty in the same way — so
	// its silence is a statement about WHERE the call came from and not about
	// there being nothing to find.
	if err := os.WriteFile(filepath.Join(shared, dirt), []byte("other writers'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A MAIN checkout that happens to sit on a posse/ branch, dirty. Odd but
	// real — a branch survives its worktree — and it is the only tree that
	// differs from the dirty session tree in the LINKED discriminator alone.
	// Without it the linked arm has no row of its own: `shared` above is on
	// `main`, so killing the linked arm there is caught by the branch arm and
	// the mutation reads as silent for the wrong reason (MEASURED here — this
	// pin's first cut passed that mutant).
	onPosse := filepath.Join(root, "repo-on-posse")
	if err := os.MkdirAll(onPosse, 0o755); err != nil {
		t.Fatal(err)
	}
	git(onPosse, "init", "-q", ".")
	if err := os.WriteFile(filepath.Join(onPosse, "kept.txt"), []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(onPosse, "add", "--", "kept.txt")
	git(onPosse, "commit", "-qm", "fixture", "--", "kept.txt")
	git(onPosse, "branch", "-M", "posse/probe-shared")
	if err := os.WriteFile(filepath.Join(onPosse, dirt), []byte("other writers'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fx.onPosse = onPosse
	return fx
}

// TestQABdArgvGateCloseArmDiscriminatesPerArm mutation-checks the close arm
// one discriminator at a time, which is the only way to know the four rows
// above are reading four different things rather than one.
//
// The seam is BD_ARGV_GATE_PY, the wrapper's own override — the same one the
// fail-closed pins use — so the mutant is a COPY and the shipped script is
// never edited. Each mutant kills exactly one arm and is required to do two
// things: flip the row that arm owns, and leave the dirty-session row still
// refusing. Without the second half a mutant that simply broke the script
// would "pass" every arm at once (the mutation-harness rule this file's
// fallback pin already follows).
//
// Each anchor is asserted to appear exactly once in the shipped script before
// it is replaced: an anchor that has been rewritten out matches nothing, and
// a mutation that changed nothing is a green that measured nothing.
func TestQABdArgvGateCloseArmDiscriminatesPerArm(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	shipped, err := os.ReadFile(filepath.Join("scripts", "bd-argv-gate.py"))
	if err != nil {
		t.Fatal(err)
	}
	fx := newCloseGateTrees(t)
	dir := t.TempDir()

	// The control first: the same copy, run through the same override, must
	// reproduce the shipped verdicts. A mutant reading taken against a broken
	// harness says nothing about the mutation.
	control := filepath.Join(dir, "control.py")
	if err := os.WriteFile(control, shipped, 0o644); err != nil {
		t.Fatal(err)
	}
	controlEnv := []string{"BD_ARGV_GATE_PY=" + control}
	if got := denied(t, runGateAt(t, controlEnv, "bd close x", fx.dirty)); !strings.Contains(got, fx.dirtFile) {
		t.Fatalf("control: the unmutated copy must refuse the dirty session tree naming %q, got: %s", fx.dirtFile, got)
	}

	for _, m := range []struct {
		name   string
		anchor string
		dead   string
		flips  string // the tree whose row this arm owns; silent -> refused
	}{
		{
			name:   "the posse/ branch arm",
			anchor: `if not branch.startswith("posse/"):`,
			dead:   `if False:`,
			flips:  fx.foreign,
		},
		{
			name:   "the porcelain arm",
			anchor: "    if not paths:",
			dead:   "    if paths is None:",
			flips:  fx.clean,
		},
		{
			name:   "the linked-worktree arm",
			anchor: "if gitdir.strip() == common.strip():",
			dead:   "if False:",
			flips:  fx.onPosse,
		},
	} {
		if n := strings.Count(string(shipped), m.anchor); n != 1 {
			t.Fatalf("%s: its anchor %q appears %d times in the shipped script, want 1 — "+
				"the arm was rewritten and this mutation is aimed at nothing", m.name, m.anchor, n)
		}
		mutant := filepath.Join(dir, "mutant.py")
		if err := os.WriteFile(mutant, []byte(strings.Replace(string(shipped), m.anchor, m.dead, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		env := []string{"BD_ARGV_GATE_PY=" + mutant}
		if got := denied(t, runGateAt(t, env, "bd close x", m.flips)); got == "" {
			t.Errorf("%s: with it dead, %s is still silent — no row discriminates this arm, "+
				"so deleting it would go unnoticed", m.name, m.flips)
		}
		// The edit disabled ONE arm and did not break the script: the row the
		// whole feature exists for still refuses.
		if got := denied(t, runGateAt(t, env, "bd close x", fx.dirty)); !strings.Contains(got, fx.dirtFile) {
			t.Errorf("%s: its mutant also stopped refusing the dirty session tree (%s) — the edit "+
				"broke the script rather than disabling one arm", m.name, got)
		}
	}
}

// TWO PARKED PINS, found verifying the close that added the close arm above
// (ranger-base-fdco0 over ranger-base-3xgc7, both findings filed as
// ranger-base-0rfce). Everything that close claimed reproduced at tree ROOTS;
// these are the two shapes it did not measure. Both are written to fail on
// their assertion and not on their control, so unparking one says what it is
// for; delete the Skip to see it.

// ADR 0041 §3 says the close arm never fires in the shared checkout, and
// Verification 4's amended row spells the case: a dirty MAIN checkout whose
// branch is under posse/ must be silent. It is — from the checkout's ROOT.
//
// session_worktree compares git's two answers as STRINGS, and git does not
// print them in one format. From a subdirectory of a main checkout
// `--git-dir` is ABSOLUTE while `--git-common-dir` is RELATIVE (MEASURED
// 2026-09-05, git 2.50.1: `/path/repo/.git` and `../.git`), so the two differ
// and the linked arm reads every subdirectory of every main checkout as a
// linked worktree. With the posse/ arm satisfied too, the gate then refuses a
// close typed in a main checkout and tells its author that posse will
// fast-forward a tree the launcher will never touch.
//
// Not live: the shared checkout is on `main`, so the branch arm carries it
// today. The row escaped because newCloseGateTrees probes every tree at its
// root, which is the one cwd shape where the two formats agree — so the
// onPosse tree, built precisely to give the linked arm a mutant of its own,
// measures that arm only where it cannot be wrong.
//
// Delete the Skip when session_worktree stops comparing two formats — either
// `rev-parse --path-format=absolute --git-dir --git-common-dir` (one format
// from any cwd, git 2.31+) or os.path.realpath on both answers.
func TestQABdArgvGateCloseArmIsSilentBelowAMainCheckout(t *testing.T) {
	t.Skip("ranger-base-0rfce: --git-common-dir prints `../.git` from a subdirectory of a main checkout while --git-dir prints an absolute path, so the linked arm's string compare reads any subdirectory of a main checkout as a session worktree")

	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	fx := newCloseGateTrees(t)
	sub := func(tree string) string {
		t.Helper()
		p := filepath.Join(tree, "sub")
		// An EMPTY directory is not a porcelain record, so making it does not
		// move either tree's dirty set: the controls below still read the
		// same trees the rows above do.
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// Control one: a SUBDIRECTORY is not what turns the arm off. The dirty
	// session tree still refuses from below its root, naming the same path.
	if got := denied(t, runGateAt(t, nil, "bd close x", sub(fx.dirty))); !strings.Contains(got, fx.dirtFile) {
		t.Fatalf("control: the dirty session tree must still refuse from its own subdirectory naming %q, got: %s", fx.dirtFile, got)
	}
	// Control two: the row as the ADR spells it, at the root, which passes.
	if got := denied(t, runGateAt(t, nil, "bd close x", fx.onPosse)); got != "" {
		t.Fatalf("control: a dirty MAIN checkout on a posse/ branch must be silent at its root, got: %s", got)
	}
	// The assertion: the same checkout, one directory down.
	if got := denied(t, runGateAt(t, nil, "bd close x", sub(fx.onPosse))); got != "" {
		t.Errorf("a dirty MAIN checkout on a posse/ branch refuses the close from a subdirectory — "+
			"the linked arm read `--git-dir` against `--git-common-dir` in two different formats, so the "+
			"shared checkout is inside the fence ADR 0041 §3 says it is outside: %s", got)
	}
}

// A bd call inside a shell compound command is invisible to the WHOLE gate —
// the close arm and the verb fence alike. segments() splits the line on its
// semicolons, so the middle segment opens with the reserved word `then` (or
// `do`), command_word vouches for that word as the command, it is not bd, and
// the segment is dropped without a question asked of it.
//
// This one PREDATES the close arm and only gets inherited by it: the denied
// verb escapes the same way, which is the second row below and the reason
// this pin is not about ADR 0041 at all. Grouping is fine — `{ bd close x; }`
// is refused — so what the parser loses is the reserved words, not the
// structure. Delete the Skip when command_word stops resolving `then`/`do`/
// `else` as a command word.
func TestQABdArgvGateReadsBdInsideACompoundCommand(t *testing.T) {
	t.Skip("ranger-base-0rfce: `if true; then bd …; fi` splits into a segment whose command word is the reserved word `then`, so every bd call inside a compound command escapes both the verb fence and the close arm")

	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	fx := newCloseGateTrees(t)

	// The controls first, and they are what makes a silence below mean
	// something: both lines are refused when they are typed bare, so a gate
	// that had simply stopped refusing anything could not pass this test.
	if got := denied(t, runGateAt(t, nil, "bd close x", fx.dirty)); !strings.Contains(got, fx.dirtFile) {
		t.Fatalf("control: the bare close from the dirty session tree must be refused naming %q, got: %s", fx.dirtFile, got)
	}
	if got := denied(t, runGate(t, nil, "bd daemon stop")); got == "" {
		t.Fatal("control: the bare denied verb must be refused by the verb fence")
	}

	if got := denied(t, runGateAt(t, nil, "if true; then bd close x; fi", fx.dirty)); got == "" {
		t.Error("a close wrapped in `if …; then …; fi` from the dirty session tree is waved through — " +
			"the segment's command word resolved to the reserved word `then`, so the close arm was never asked")
	}
	if got := denied(t, runGate(t, nil, "if true; then bd daemon stop; fi")); got == "" {
		t.Error("the denied verb wrapped in `if …; then …; fi` is waved through — the same blind spot, " +
			"one layer up, which is what says this is the parser and not ADR 0041 §3's arm")
	}
}
