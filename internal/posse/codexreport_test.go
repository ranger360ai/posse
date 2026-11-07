//go:build !posse_arm2 && !posse_arm3

package posse

// Unit pins for ranger-base-z7kle: posse's OSC 7501 reporter and the codex
// hooks declaration that installs it (ADR 0076 Amendment 2, D6).
//
// WHAT THESE PIN, and why each needs its own arm. The reporter's whole job is
// to put exact bytes on a terminal nothing in `go test` has, at a moment
// inside somebody else's turn:
//
//   - THE BYTES. Rex parses the payload by pair, and a wrong separator, a
//     missing `app=`, or BEL instead of ST is a report that lands as nothing
//     or as a record with no owner. Nothing downstream can tell those apart
//     from a runtime that went quiet, so the sequence is compared whole and
//     literally — never with a helper that would render the same mistake into
//     both sides.
//   - ONE REPORT. codex fires several events per turn and the hook runs once
//     per event; a reporter that wrote twice would be posse deciding what
//     the second one means.
//   - EXIT 0, ALWAYS, on the hook path. codex reads exit 2 as "block this",
//     and a UserPromptSubmit hook that exits 2 holds the prompt. The arm
//     below walks every way this can fail — no stdin, truncated JSON, an
//     event D6 does not name, an app name that is not one, a /dev/tty that
//     will not open, a wrong argv — and the answer is 0 for all of them.
//   - NO WRITE means the tty is never OPENED. On a seat with no controlling
//     terminal that open is the failure, so "wrote nothing" has to be pinned
//     one step earlier than the buffer.
//   - THE DECLARATION IS TOML. Nothing in Go reads TOML, so the string is
//     handed to a parser (python3's tomllib) and the shape is asserted
//     against what comes BACK — one group, one handler per event. Asserting
//     against the string would pin the renderer to itself, which is the one
//     thing that cannot drift.
//
// MEASURED facts these rest on are in docs/notes.d/ranger-base-z7kle.md, with
// their date and box: `state=clear` removes the record rather than naming a
// sixth state, and what codex 0.153.4 does with a handler path that needs
// quoting.

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qaTTY is the write target runHookReport is handed in place of /dev/tty: it
// records the bytes, whether it was opened at all, and whether it was closed.
type qaTTY struct {
	buf    bytes.Buffer
	opens  int
	closes int
	err    error // when non-nil, the open fails — the no-controlling-terminal case
}

func (q *qaTTY) open() (io.WriteCloser, error) {
	q.opens++
	if q.err != nil {
		return nil, q.err
	}
	return q, nil
}
func (q *qaTTY) Write(p []byte) (int, error) { return q.buf.Write(p) }
func (q *qaTTY) Close() error                { q.closes++; return nil }

// qaHookRun runs the reporter over one hook payload and returns what reached
// the "terminal", plus the exit code and the tty's own ledger.
func qaHookRun(t *testing.T, app, stdin string, q *qaTTY) (string, int) {
	t.Helper()
	var out, errs bytes.Buffer
	code := runHookReport([]string{"posse", HookReportVerb, app}, strings.NewReader(stdin), &out, &errs, q.open)
	// A hook's stdout and stderr are codex's, not the terminal's: anything
	// written there is the hook's ANSWER to codex, and PreToolUse reads it.
	// The reporter has no answer to give.
	if out.Len() != 0 {
		t.Errorf("the reporter wrote %q to stdout — a hook's stdout is codex's, and PreToolUse reads it as a verdict", out.String())
	}
	if errs.Len() != 0 {
		t.Errorf("the reporter wrote %q to stderr — that lands inside the operator's turn", errs.String())
	}
	return q.buf.String(), code
}

// TestCodexHookReportIsD6sTableByteForByte walks every event ADR 0076 D6
// names and compares the whole escape sequence.
func TestCodexHookReportIsD6sTableByteForByte(t *testing.T) {
	t.Parallel()

	// The table, spelled out rather than built: `\x1b]7501;` … `\x1b\\` is
	// what every ADR 0076 probe wrote and what every measurement on this box
	// was taken with, so it is written here the way it is written there.
	for _, tc := range []struct{ event, want string }{
		{"SessionStart", "\x1b]7501;state=idle:app=codex\x1b\\"},
		{"Stop", "\x1b]7501;state=idle:app=codex\x1b\\"},
		{"Interrupt", "\x1b]7501;state=idle:app=codex\x1b\\"},
		{"UserPromptSubmit", "\x1b]7501;state=working:app=codex\x1b\\"},
		{"PreToolUse", "\x1b]7501;state=working:app=codex\x1b\\"},
		{"PostToolUse", "\x1b]7501;state=working:app=codex\x1b\\"},
		{"PermissionRequest", "\x1b]7501;state=blocked:kind=permission:app=codex\x1b\\"},
		{"SessionEnd", "\x1b]7501;state=clear:app=codex\x1b\\"},
	} {
		t.Run(tc.event, func(t *testing.T) {
			t.Parallel()
			q := &qaTTY{}
			got, code := qaHookRun(t, "codex", `{"hook_event_name":"`+tc.event+`","cwd":"/tmp","session_id":"x"}`, q)
			if got != tc.want {
				t.Errorf("%s reported %q, want %q", tc.event, got, tc.want)
			}
			if code != 0 {
				t.Errorf("%s exited %d — a reporter that can exit nonzero can stop a turn", tc.event, code)
			}
			// ONE report, and the handle is closed before the process goes.
			if q.opens != 1 || q.closes != 1 {
				t.Errorf("%s opened the tty %d times and closed it %d — exactly one of each", tc.event, q.opens, q.closes)
			}
		})
	}
}

// TestCodexHookReportCoversEveryEventPosseDeclares is the other direction of
// the same question, and it is the arm that catches the silent half: an event
// declared on codex's line whose handler writes nothing is a hook that fires,
// costs a process, and reports no state. A table and a declaration drifting
// apart is invisible in every arm above.
func TestCodexHookReportCoversEveryEventPosseDeclares(t *testing.T) {
	t.Parallel()
	for _, ev := range CodexHookEvents() {
		if CodexHookReport(ev, "codex") == "" {
			t.Errorf("posse declares a hook on %s and CodexHookReport maps it to nothing — the hook would fire and report no state", ev)
		}
	}
	// And the control: the mapping is not a function that says yes to
	// everything, or the arm above is a tautology.
	if got := CodexHookReport("SessionNoSuchEvent", "codex"); got != "" {
		t.Errorf("an event D6's table does not name reported %q — this census cannot say no", got)
	}
}

// TestHookReportOnAnEventItDoesNotKnowWritesNothingAndNeverOpensTheTTY. codex
// may grow an event tomorrow; mapping the unknown onto a state would be posse
// inventing the reading ADR 0076 §3 exists to stop.
func TestHookReportOnAnEventItDoesNotKnowWritesNothingAndNeverOpensTheTTY(t *testing.T) {
	t.Parallel()
	for _, ev := range []string{"SessionSomethingNew", "", "sessionstart", "SessionStart ", "PreCompact"} {
		q := &qaTTY{}
		got, code := qaHookRun(t, "codex", `{"hook_event_name":"`+ev+`"}`, q)
		if got != "" || code != 0 {
			t.Errorf("event %q reported %q and exited %d, want nothing and 0", ev, got, code)
		}
		if q.opens != 0 {
			t.Errorf("event %q opened /dev/tty %d times — on a seat with no controlling terminal that open IS the failure, so an event with nothing to say must not reach it", ev, q.opens)
		}
	}
}

// TestHookReportNeverExitsNonZeroOnTheHookPath is the exit-2 rule, walked
// over every failure the reporter can actually have.
func TestHookReportNeverExitsNonZeroOnTheHookPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, stdin string }{
		{"empty stdin — codex closed the pipe with nothing in it", ""},
		{"not json at all", "SessionStart\n"},
		{"json that is not an object", `["SessionStart"]`},
		{"an object with no hook_event_name", `{"cwd":"/tmp"}`},
		{"hook_event_name is not a string", `{"hook_event_name":17}`},
		{"truncated mid-object", `{"hook_event_name":"SessionSt`},
		{"a valid event with trailing garbage", `{"hook_event_name":"SessionStart"} }}`},
	} {
		q := &qaTTY{}
		if _, code := qaHookRun(t, "codex", tc.stdin, q); code != 0 {
			t.Errorf("%s exited %d — codex reads exit 2 as a block, and no exit from here may be able to hold a prompt", tc.name, code)
		}
	}

	// The /dev/tty that will not open: D6's silent exit 0. The sequence was
	// rendered and there was nowhere to put it.
	q := &qaTTY{err: os.ErrPermission}
	got, code := qaHookRun(t, "codex", `{"hook_event_name":"SessionStart"}`, q)
	if code != 0 || got != "" {
		t.Errorf("an unopenable /dev/tty exited %d and wrote %q, want 0 and nothing", code, got)
	}
	if q.opens != 1 {
		t.Errorf("the reporter did not even try the tty for a known event (%d opens)", q.opens)
	}

	// A wrong argv, which can only be a hand run or a declaration that
	// drifted from this binary: it says so on stderr and still gets out of
	// the way. Checked through runHookReport directly, because the stderr
	// qaHookRun forbids is the point of this one.
	for _, args := range [][]string{
		{"posse", HookReportVerb},
		{"posse", HookReportVerb, "codex", "extra"},
		{"posse", HookReportVerb, "--nope"},
		{"posse", HookReportVerb, HookReportDeclareFlag},
	} {
		q := &qaTTY{}
		var out, errs bytes.Buffer
		code := runHookReport(args, strings.NewReader(`{"hook_event_name":"SessionStart"}`), &out, &errs, q.open)
		if code != 0 {
			t.Errorf("argv %v exited %d — a malformed handler line must not be able to block a prompt", args[1:], code)
		}
		if !strings.Contains(errs.String(), "usage") {
			t.Errorf("argv %v said nothing an operator could act on: %q", args[1:], errs.String())
		}
		if q.opens != 0 || q.buf.Len() != 0 {
			t.Errorf("argv %v still reported something: %d opens, %q", args[1:], q.opens, q.buf.String())
		}
	}
}

// TestHookReportCannotExitTwoByCrashing. A Go panic exits with status 2, and
// 2 is exactly the status codex reads as "block this prompt" — so the last
// route from a reporter bug to a held prompt is a crash, and it is a route
// nothing above the recover would have chosen. The arm plants the crash where
// the reporter has no control over what it is handed: the tty opener.
//
// It also pins the SHORT ARGV, which is the other way this function could
// have panicked rather than answered: `posse hook-report` with nothing after
// it is argv[2:] on a two-element slice today, and one element short of a
// panic if the verb were ever reached another way.
func TestHookReportCannotExitTwoByCrashing(t *testing.T) {
	t.Parallel()
	var out, errs bytes.Buffer
	boom := func() (io.WriteCloser, error) { panic("z7kle: the tty opener blew up") }
	code := runHookReport([]string{"posse", HookReportVerb, "codex"}, strings.NewReader(`{"hook_event_name":"SessionStart"}`), &out, &errs, boom)
	if code != 0 {
		t.Errorf("a crash in the reporter exited %d; Go panics exit 2, and 2 is codex's block", code)
	}
	for _, argv := range [][]string{{"posse"}, {"posse", HookReportVerb}} {
		var o, e bytes.Buffer
		q := &qaTTY{}
		if code := runHookReport(argv, strings.NewReader(""), &o, &e, q.open); code != 0 {
			t.Errorf("argv %v exited %d, want 0", argv, code)
		}
	}
}

// TestHookReportRefusesAnAppNameThatIsNotOne. `:` separates pairs and `=`
// separates one, so an app string carrying either is not a name — it is extra
// pairs in somebody else's report. Nothing is written at all, rather than a
// report with the name stripped out of it.
func TestHookReportRefusesAnAppNameThatIsNotOne(t *testing.T) {
	t.Parallel()
	for _, app := range []string{
		"",
		// A LONE COLON, and it is the row that was missing (ranger-base-5x8xi).
		// Every other row below carries a `=`, a space, an escape, a newline
		// or a slash BESIDE its colon, so each is refused for the other byte
		// and the colon — the first byte this pin's doc names — was never
		// isolated: MEASURED 2026-11-07, admitting `:` in plainStatusToken
		// left this test green.
		"codex:x",
		"codex:state=idle",
		"codex=x",
		"co dex",
		"codex\x1b\\",
		"codex\n",
		"codex/1",
	} {
		q := &qaTTY{}
		var out, errs bytes.Buffer
		code := runHookReport([]string{"posse", HookReportVerb, app}, strings.NewReader(`{"hook_event_name":"SessionStart"}`), &out, &errs, q.open)
		if code != 0 {
			t.Errorf("app %q exited %d", app, code)
		}
		if q.opens != 0 || q.buf.Len() != 0 {
			t.Errorf("app %q reached the terminal as %q — an app name is a name, never a pair", app, q.buf.String())
		}
	}
	// The control: the names posse actually uses are names.
	for _, app := range []string{"codex", "claude", "grok-build", "claude-code", "bob_2", "v0.153.4"} {
		if CodexHookReport("SessionStart", app) == "" {
			t.Errorf("%q is a runtime name posse may have to report and the token rule rejects it", app)
		}
	}
}

// TestCodexHookEventsCannotBeReorderedByItsCaller. ADR 0076 D7's trust ask
// (ranger-base-fwwl8) reads this list to build `hooks.state`, and codex keys
// each hook by its own event — but a caller that sorted the slice in place
// would move what the DECLARATION renders, which is the string a launched
// seat is compared against.
func TestCodexHookEventsCannotBeReorderedByItsCaller(t *testing.T) {
	t.Parallel()
	first := CodexHookEvents()
	if len(first) == 0 {
		t.Fatal("posse declares no events")
	}
	got := CodexHookEvents()
	got[0] = "Clobbered"
	if again := CodexHookEvents(); again[0] != first[0] {
		t.Errorf("CodexHookEvents handed out its own slice: after a caller wrote to it, it answers %q and not %q", again[0], first[0])
	}
}

// ─── the declaration ─────────────────────────────────────────────────────────

// qaTOMLToJSON parses s as TOML with python3's tomllib and returns the JSON
// it decodes to — a real parser, so the arm below is about TOML and not about
// posse's idea of it. The error is returned rather than fataled: one arm
// needs the parser to say NO.
func qaTOMLToJSON(t *testing.T, s string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	py := filepath.Join(dir, "toml2json.py")
	if err := os.WriteFile(py, []byte("import json,sys,tomllib\nprint(json.dumps(tomllib.loads(open(sys.argv[1],encoding='utf-8').read())))\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(dir, "decl.toml")
	if err := os.WriteFile(in, []byte(s+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("python3", "-I", py, in).CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

// TestCodexHooksDeclarationParsesAsTOMLWithTheShapeD6Names.
//
// `-c hooks={…}` is read by codex's own TOML parser (D6: config.toml carries
// hooks as well as hooks.json does, HookEventsToml), so the first question is
// whether the string IS TOML. The second is the shape, and it is asserted
// against the parse rather than against the string: one group per event, one
// handler per group, `type="command"`, the handler posse's own binary, and
// the probe's timeout.
func TestCodexHooksDeclarationParsesAsTOMLWithTheShapeD6Names(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("no python3 on PATH, and nothing in Go reads TOML — this arm cannot be run here: %v", err)
	}
	const exe = "/Users/nobody/.local/bin/posse"
	decl, err := CodexHooksDeclaration(exe, "codex")
	if err != nil {
		t.Fatalf("a plain absolute path was refused: %v", err)
	}
	if !strings.HasPrefix(decl, "hooks={") {
		t.Fatalf("the declaration is the whole `-c` value and must be the `hooks` key: %q", decl)
	}

	raw, err := qaTOMLToJSON(t, decl)
	if err != nil {
		t.Fatalf("codex's parser is a TOML parser and this is not TOML: %v\n%s\n%s", err, raw, decl)
	}
	var got struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("the parsed declaration is not the shape codex's HookEventsToml takes: %v\n%s", err, raw)
	}

	events := CodexHookEvents()
	if len(got.Hooks) != len(events) {
		t.Errorf("the declaration carries %d events, posse declares %d: %v", len(got.Hooks), len(events), raw)
	}
	for _, ev := range events {
		groups, ok := got.Hooks[ev]
		if !ok {
			t.Errorf("%s is declared by posse and absent from the rendered TOML", ev)
			continue
		}
		// One group and one handler, because codex keys trust by position —
		// `/<session-flags>/config.toml:<event>:<group>:<handler>` (D6, D7).
		// A second handler in a group moves every key after it, and a key
		// that moved is a hook whose trust no longer matches: untrusted, so
		// the review dialog holds the prompt and no record arrives.
		if len(groups) != 1 {
			t.Errorf("%s renders %d groups, want 1 — the trust key carries the group index", ev, len(groups))
			continue
		}
		if len(groups[0].Hooks) != 1 {
			t.Errorf("%s renders %d handlers in its group, want 1 — the trust key carries the handler index", ev, len(groups[0].Hooks))
			continue
		}
		h := groups[0].Hooks[0]
		if h.Type != "command" {
			t.Errorf("%s declares type %q, want \"command\"", ev, h.Type)
		}
		// The path is single-quoted and the two plain tokens are not — see
		// codexHookCommand for what was measured about codex's own split.
		if want := "'" + exe + "' " + HookReportVerb + " codex"; h.Command != want {
			t.Errorf("%s declares command %q, want %q", ev, h.Command, want)
		}
		if h.Timeout != 3 {
			t.Errorf("%s declares timeout %d, want the probe's 3", ev, h.Timeout)
		}
	}
}

// TestTheTOMLArmCanStillSayNo is qaTOMLToJSON's own control. Without it, a
// parser that had quietly stopped being run — a python3 that answers 0 on
// anything, an exec that failed in a way the arm read as success — would make
// the arm above green over any string at all.
func TestTheTOMLArmCanStillSayNo(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("no python3 on PATH: %v", err)
	}
	for _, bad := range []string{
		`hooks={SessionStart=[{hooks=[{type="command"`,
		`hooks={SessionStart=[{hooks=[{type=command}]}]}`,
		"hooks=",
	} {
		if out, err := qaTOMLToJSON(t, bad); err == nil {
			t.Errorf("tomllib accepted %q, so this arm cannot tell TOML from anything: %s", bad, out)
		}
	}
}

// TestCodexHooksDeclarationQuotesTheHandlerPathAndRefusesWhatItCannotQuote.
//
// codex's hook `command` is ONE string that codex splits into an argv itself,
// and posse does not own that split. MEASURED 2026-11-06 (codex-cli 0.153.4,
// docs/adr/0076-reporter.probe.sh arm `space`): unquoted, the split is on
// whitespace with no shell and a path holding a space dies as `hook exited
// with code 127`; single-quoted, the quotes are honoured and the hook runs
// with the whole path as its `$0`. So the path is quoted — always, one
// spelling.
//
// What is NOT measured is refused rather than guessed at, which is D7's
// posture for anything posse cannot vet: a refusal costs a launch, where a
// mis-split costs a seat that starts, reports nothing, and reads downstream
// as a runtime that went silent.
func TestCodexHooksDeclarationQuotesTheHandlerPathAndRefusesWhatItCannotQuote(t *testing.T) {
	t.Parallel()

	// A space is the case this is all for, and it is a case posse's own
	// binary can really be in on macOS — anything under `Application
	// Support`. It renders, and it renders QUOTED.
	const spaced = "/Users/nobody/Library/Application Support/posse/posse"
	decl, err := CodexHooksDeclaration(spaced, "codex")
	if err != nil {
		t.Fatalf("a path holding a space was refused, and single-quoting it is MEASURED to work: %v", err)
	}
	if want := `command="'` + spaced + `' ` + HookReportVerb + ` codex"`; !strings.Contains(decl, want) {
		t.Errorf("the handler is not quoted as measured.\n got: %s\nwant it to contain: %s", decl, want)
	}

	for _, tc := range []struct{ name, exe string }{
		{"posse cannot name its own binary", ""},
		{"a relative path, which would resolve against the seat's cwd", "bin/posse"},
		{"a bare name, which a gate shim would answer for", "posse"},
		{"a single quote, which would close the quoting posse just opened", "/Users/nobody/po'sse"},
		{"a double quote, which the TOML string itself would read", `/Users/nobody/po"sse`},
		{"a backslash, which codex's splitter may read as an escape", `/Users/nobody/po\sse`},
		{"a tab", "/Users/nobody/\tposse"},
		{"a newline", "/Users/nobody/posse\n"},
		{"a control byte", "/Users/nobody/po\x01sse"},
		{"DEL", "/Users/nobody/po\x7fsse"},
	} {
		got, err := CodexHooksDeclaration(tc.exe, "codex")
		if err == nil {
			t.Errorf("%s: %q was accepted and rendered %q", tc.name, tc.exe, got)
		}
	}
	for _, app := range []string{"", "codex:state=idle", "co dex", "codex/1"} {
		if got, err := CodexHooksDeclaration("/Users/nobody/.local/bin/posse", app); err == nil {
			t.Errorf("app %q was accepted and rendered %q — the app name is also the `app=` in every report", app, got)
		}
	}
	// The control: the paths posse's own binary actually has on this fleet —
	// a brew keg, ~/.local/bin, a checkout build, a session worktree — all
	// render.
	for _, exe := range []string{
		"/opt/homebrew/bin/posse",
		"/Users/nobody/.local/bin/posse",
		"/Users/nobody/src/posse/posse",
		"/Users/nobody/.posse/worktrees/posse/persona-posse-ranger-base-z7kle/posse",
		"/usr/local/bin/posse",
	} {
		if _, err := CodexHooksDeclaration(exe, "codex"); err != nil {
			t.Errorf("%q is a path posse's binary really has here and the declaration refused it: %v", exe, err)
		}
	}
}

// TestOSC7501IsTheSequenceEveryProbeMeasured. ST (`ESC \`) and not BEL, and
// the 7501 introducer spelled once: every MEASURED record on this box was
// written with `printf '\033]7501;…\033\\'`, so a renderer that drifted to
// BEL would make every measurement in ADR 0076 about a different sequence
// than the one posse ships.
func TestOSC7501IsTheSequenceEveryProbeMeasured(t *testing.T) {
	t.Parallel()
	if got, want := OSC7501("state=working:app=codex"), "\x1b]7501;state=working:app=codex\x1b\\"; got != want {
		t.Errorf("OSC7501 rendered %q, want %q", got, want)
	}
	if got := OSC7501("state=idle"); strings.ContainsRune(got, '\a') {
		t.Errorf("the sequence is BEL-terminated (%q) — the probes measured ST", got)
	}
}
