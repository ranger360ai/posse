//go:build !posse_arm2 && !posse_arm3

package posse

// Unit pins for ranger-base-fwwl8: D7's trust ask — posse asks codex which
// hooks would load on a seat's line, and renders the one `-c hooks.state`
// value that trusts its own and switches every other one off (ADR 0076
// Amendment 2, D7).
//
// WHAT THESE PIN, AND WHY A FAKE app-server AND NOT A FAKE READER. The thing
// under test is a conversation with a child process and a string that goes on
// a command line. A test that injected the ROWS would pin the rendering and
// nothing about the ask: not the argv, not the environment, not the order the
// two `-c` values have to be in, not that a notification arriving mid-stream
// is skipped, and not that the child is killed. So every arm below starts a
// real child — a shell script that speaks the protocol — and the rows are the
// script's own answer. The seam is the BINARY (CodexHooksAsk.Bin), which is
// the same seam a probe uses, not a function only a test can reach.
//
// WHAT THE REFUSAL ARMS ARE FOR. D7 refuses a launch in two named cases and
// posse refuses in several more it did not enumerate, and every one of them
// has the same alternative: a line that looks complete and a codex seat
// sitting on the startup review dialog — no record, no turn, nothing on the
// screen that says why. A refusal is only worth anything if it is the thing
// that happens, so each case is driven end to end and its SENTENCE is
// checked, not merely that an error came back: the sentence is what the next
// reader gets.
//
// THE THREE RENDERING CASES of the bead's done-when are the first function:
// posse's hooks alone, posse's hooks beside one foreign hook, and posse's
// hooks beside four foreign hooks from four different layers — because D7's
// rule is keyed on "not the session-flags layer" and not on the word "user",
// and a one-foreign-hook table cannot tell those apart.

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ─── the fake app-server ─────────────────────────────────────────────────────

// ctExe is the posse path the declaration under test names: absolute, with no
// byte codexHookCommand refuses, and not this binary — the handler is a
// parameter precisely so that it need not be (codexreport.go).
const ctExe = "/opt/posse-fixture/posse"

// ctHandler is the `command` codex would echo for that exe, which is what the
// drift guard compares against. Taken from the renderer rather than written
// out, because a literal here would pass a tree in which the renderer and the
// guard had drifted together.
func ctHandler(t *testing.T) string {
	t.Helper()
	cmd, err := codexHookCommand(ctExe, "codex")
	if err != nil {
		t.Fatalf("codexHookCommand(%q, codex): %v", ctExe, err)
	}
	return cmd
}

// ctRow is one hook as the fake app-server will report it. The fields are
// spelled in codex's own JSON names by ctListReply, so a rename on either
// side is a compile error here rather than a silently absent field.
type ctRow struct {
	key, source, command, hash, sourcePath, trustStatus string
	managed                                             bool
}

// ctOurs is posse's own eight, as codex reports them: the session-flags
// layer, the synthetic key path, one group and one handler per event, the
// handler posse declared. MEASURED 2026-11-06 — these are the rows the real
// `codex app-server` answered with, with the hashes shortened.
func ctOurs(t *testing.T) []ctRow {
	t.Helper()
	// The key's event segment is codex's own snake_case spelling of the
	// event, which is why it is derived here rather than taken from
	// CodexHookEvents(): posse never builds one of these keys, it echoes
	// them, and a fixture that built them from posse's own event names would
	// be asserting posse's spelling against itself.
	events := []string{"session_start", "user_prompt_submit", "pre_tool_use", "permission_request", "post_tool_use", "stop", "interrupt", "session_end"}
	if len(events) != len(CodexHookEvents()) {
		t.Fatalf("the fixture names %d codex events and posse declares %d — the fixture is the one that has to follow (CodexHookEvents, codexreport.go)", len(events), len(CodexHookEvents()))
	}
	rows := make([]ctRow, 0, len(events))
	for i, ev := range events {
		rows = append(rows, ctRow{
			key:         "/<session-flags>/config.toml:" + ev + ":0:0",
			source:      codexSessionFlagsSource,
			command:     ctHandler(t),
			hash:        "sha256:" + strings.Repeat(string(rune('a'+i)), 64),
			sourcePath:  "/<session-flags>/config.toml",
			trustStatus: "untrusted",
		})
	}
	return rows
}

// ctListReply renders one `hooks/list` answer, in codex's own shape:
// `result.data[]`, one entry per asked cwd, each `{cwd, hooks, errors,
// warnings}`.
func ctListReply(cwd string, rows []ctRow) string {
	var b strings.Builder
	b.WriteString(`{"id":2,"result":{"data":[{"cwd":`)
	b.WriteString(ctJSON(cwd))
	b.WriteString(`,"errors":[],"warnings":[],"hooks":[`)
	for i, r := range rows {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"key":` + ctJSON(r.key))
		b.WriteString(`,"source":` + ctJSON(r.source))
		b.WriteString(`,"command":` + ctJSON(r.command))
		b.WriteString(`,"currentHash":` + ctJSON(r.hash))
		b.WriteString(`,"sourcePath":` + ctJSON(r.sourcePath))
		b.WriteString(`,"trustStatus":` + ctJSON(r.trustStatus))
		b.WriteString(`,"isManaged":` + ctBool(r.managed))
		// Fields posse does NOT read, carried anyway: an answer narrower
		// than the real one would let a decode that depended on absence
		// pass.
		b.WriteString(`,"eventName":"sessionStart","handlerType":"command","async":false,"matcher":null,"timeoutSec":3,"pluginId":null,"displayOrder":0,"enabled":true}`)
	}
	b.WriteString(`]}]}}`)
	return b.String()
}

// ctJSON spells one string as JSON through encoding/json, so a fixture key
// holding a quote, a backslash or a newline is escaped the way codex would
// escape it rather than the way a test author remembered to.
func ctJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func ctBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ctInitReply is the app-server's answer to `initialize`, as it really comes
// back (MEASURED: userAgent, codexHome, platformFamily, platformOs).
const ctInitReply = `{"id":1,"result":{"userAgent":"fake/0 (fixture)","codexHome":"/fixture","platformFamily":"unix","platformOs":"macos"}}`

// ctNotify is the id-less message that really arrives between the two
// answers (MEASURED 2026-11-06). It is in every well-behaved arm, because a
// reader that takes the next line reads THIS and calls the answer malformed.
const ctNotify = `{"method":"remoteControl/status/changed","params":{"status":"disabled"},"emittedAtMs":1}`

// ctFake writes a fake `codex app-server` and returns the binary and the
// witness file it writes. The witness is the arm that pins the ASK rather
// than the rendering: the child's cwd, whether CODEX_HOME reached it and with
// what value, its whole argv in order, and every line posse sent it.
func ctFake(t *testing.T, body string) (bin, witness string) {
	t.Helper()
	dir := t.TempDir()
	bin, witness = filepath.Join(dir, "codex"), filepath.Join(dir, "witness")
	q := shQuote(witness)
	script := "#!/bin/sh\n" +
		"{\n" +
		"  printf 'cwd=%s\\n' \"$(pwd -P)\"\n" +
		"  if [ \"${CODEX_HOME+set}\" = set ]; then printf 'home=%s\\n' \"$CODEX_HOME\"; else printf 'home=<unset>\\n'; fi\n" +
		"  for a in \"$@\"; do printf 'argv=%s\\n' \"$a\"; done\n" +
		"} > " + q + "\n" +
		strings.ReplaceAll(body, "@W@", q)
	if err := WriteExecutable(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, witness
}

// ctTalk is the body of a fake that holds a normal conversation: it answers
// `initialize`, then pushes the measured notification, then answers
// `hooks/list` with listReply. `tail` runs after the conversation.
//
// The `initialized` NOTIFICATION matches neither case arm, which is the
// point: it is a notification and nothing answers it. The pattern for
// initialize carries its closing quote so that `"initialized"` cannot match
// it.
func ctTalk(listReply, tail string) string {
	return "while IFS= read -r line; do\n" +
		"  printf 'in=%s\\n' \"$line\" >> @W@\n" +
		"  case $line in\n" +
		"    *'\"initialize\"'*) printf '%s\\n' " + shQuote(ctInitReply) + "; printf '%s\\n' " + shQuote(ctNotify) + " ;;\n" +
		"    *'hooks/list'*) printf '%s\\n' " + shQuote(listReply) + "\n" + tail + "      ;;\n" +
		"  esac\n" +
		"done\n"
}

// ctAsk is the ask under test, pointed at a fake. Every field is spelled
// here so that an arm which needs to change one changes one.
func ctAsk(bin, cwd string, conf ...string) CodexHooksAsk {
	return CodexHooksAsk{Exe: ctExe, App: "codex", Cwd: cwd, Conf: conf, Bin: bin, Timeout: 30 * time.Second}
}

// ctCwd is a seat directory: absolute and symlink-resolved, because that is
// what codex keys project trust by and what the child's own `pwd -P` will
// report.
func ctCwd(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// ─── the three rendering cases ───────────────────────────────────────────────

func TestCodexHooksAskRendersD7sTrustFromCodexsOwnAnswer(t *testing.T) {
	t.Setenv(CodexHomeVar, "") // not in force: this arm is about the rendering
	ours := ctOurs(t)
	foreign := func(key, source, path string) ctRow {
		return ctRow{key: key, source: source, command: "/somebody/else/hook.sh", hash: "sha256:" + strings.Repeat("f", 64), sourcePath: path, trustStatus: "untrusted"}
	}
	for _, c := range []struct {
		what string
		rows []ctRow
		want string
	}{
		{
			// posse's own hooks alone: every one trusted, none disabled.
			// This is the only shape a fleet seat on a clean box produces.
			what: "posse's hooks alone",
			rows: ours,
			want: ctWantState(nil, ours),
		},
		{
			// One foreign hook — the operator's own user-layer hooks.json,
			// which is the shape the probe's `userhook` and `ask` arms both
			// measured. Disabled AND trusted: `enabled=false` alone still
			// draws the review dialog (MEASURED, ADR 0076).
			what: "posse's hooks and one user-layer hook",
			rows: append(append([]ctRow{}, foreign("/op/.codex/hooks.json:session_start:0:0", "user", "/op/.codex/hooks.json")), ours...),
			want: ctWantState(
				[]ctRow{foreign("/op/.codex/hooks.json:session_start:0:0", "user", "/op/.codex/hooks.json")},
				ours),
		},
		{
			// Four foreign hooks from four different layers, one of them a
			// `source` codex has not shipped. THE RULE IS "not the
			// session-flags layer" and never a roster of known layers: a
			// codex that grows a hook source tomorrow must switch it off,
			// not fall through the gap as posse's own.
			what: "four foreign layers, including one posse has never seen",
			rows: append(append([]ctRow{},
				foreign("/op/.codex/hooks.json:stop:0:0", "user", "/op/.codex/hooks.json"),
				foreign("/repo/.codex/config.toml:stop:0:0", "project", "/repo/.codex/config.toml"),
				foreign("/op/.codex/plugins/p/hooks.json:stop:0:0", "plugin", "/op/.codex/plugins/p/hooks.json"),
				foreign("/some/where:stop:0:0", "somethingCodexGrewLater", "/some/where")),
				ours...),
			want: ctWantState(
				[]ctRow{
					foreign("/op/.codex/hooks.json:stop:0:0", "user", "/op/.codex/hooks.json"),
					foreign("/repo/.codex/config.toml:stop:0:0", "project", "/repo/.codex/config.toml"),
					foreign("/op/.codex/plugins/p/hooks.json:stop:0:0", "plugin", "/op/.codex/plugins/p/hooks.json"),
					foreign("/some/where:stop:0:0", "somethingCodexGrewLater", "/some/where"),
				},
				ours),
		},
	} {
		t.Run(c.what, func(t *testing.T) {
			cwd := ctCwd(t)
			bin, _ := ctFake(t, ctTalk(ctListReply(cwd, c.rows), ""))
			flags, err := ctAsk(bin, cwd).Flags()
			if err != nil {
				t.Fatalf("%s: %v", c.what, err)
			}
			if flags.State != c.want {
				t.Errorf("%s: the trust line is\n  %s\nwant\n  %s", c.what, flags.State, c.want)
			}
			// The declaration comes back beside the trust, and it is D6's
			// renderer's own output: the pair is one value because trust
			// binds to the hash of THIS declaration (CodexHooksFlags).
			decl, err := CodexHooksDeclaration(ctExe, "codex")
			if err != nil {
				t.Fatal(err)
			}
			if flags.Declaration != decl {
				t.Errorf("%s: Flags returned a declaration that is not the renderer's: %q", c.what, flags.Declaration)
			}
		})
	}
}

// ctWantState spells D7's rule out by hand, in the order the rows are
// reported: disabled-and-trusted for every foreign hook, trusted alone for
// posse's own. Written here rather than by calling codexHooksState, which
// would be the implementation asserting itself.
func ctWantState(foreign, ours []ctRow) string {
	var parts []string
	for _, r := range foreign {
		parts = append(parts, `"`+r.key+`"={enabled=false,trusted_hash="`+r.hash+`"}`)
	}
	for _, r := range ours {
		parts = append(parts, `"`+r.key+`"={trusted_hash="`+r.hash+`"}`)
	}
	return "hooks.state={" + strings.Join(parts, ",") + "}"
}

// ─── the ask itself: the line, the cwd, the home, the order ──────────────────

func TestCodexHooksAskRunsWithTheSeatsOwnCwdLineAndHome(t *testing.T) {
	home := ctCwd(t)
	t.Setenv(CodexHomeVar, home)
	cwd := ctCwd(t)
	ours := ctOurs(t)
	bin, witness := ctFake(t, ctTalk(ctListReply(cwd, ours), ""))

	trust := `projects={"` + cwd + `"={trust_level="trusted"}}`
	flags, err := ctAsk(bin, cwd, "allow_login_shell=false", trust).Flags()
	if err != nil {
		t.Fatal(err)
	}
	w := ctWitness(t, witness)

	if got := w["cwd"]; len(got) != 1 || got[0] != cwd {
		t.Errorf("the ask ran in %v, want the seat's cwd %q — codex discovers a project's hooks and keys its trust by the directory it runs in, so an ask in posse's own cwd answers about the wrong seat", got, cwd)
	}
	if got := w["home"]; len(got) != 1 || got[0] != home {
		t.Errorf("the child saw CODEX_HOME=%v, want %q — ADR 0077 D1: the ask must resolve the same home the seat will, or it trusts hooks from one home while the seat loads another", got, home)
	}
	// THE ARGV, in order. The declaration leads: codex applies `-c` in
	// order and `hooks={…}` replaces the whole table, so a trust value that
	// arrives first is wiped (MEASURED 2026-11-06, both ways round).
	wantArgv := []string{"app-server", "-c", flags.Declaration, "-c", "allow_login_shell=false", "-c", trust}
	if got := w["argv"]; !ctSameLines(got, wantArgv) {
		t.Errorf("the ask's argv is\n  %q\nwant\n  %q\n— every `-c` the seat's line carries has to be on the ask's line too (the hooks codex reports depend on them), the declaration FIRST, and no hooks.state (that is the answer)", got, wantArgv)
	}
	// THE CONVERSATION, in order: initialize, the initialized
	// notification, then hooks/list asked about this one cwd.
	in := w["in"]
	if len(in) != 3 {
		t.Fatalf("posse sent %d message(s), want 3 (initialize, initialized, hooks/list): %q", len(in), in)
	}
	for i, want := range []string{`"method":"initialize"`, `"method":"initialized"`, `"method":"hooks/list"`} {
		if !strings.Contains(in[i], want) {
			t.Errorf("message %d is %s, want one carrying %s", i+1, in[i], want)
		}
	}
	if want := `"cwds":["` + cwd + `"]`; !strings.Contains(in[2], want) {
		t.Errorf("hooks/list was asked %s, want it to carry %s — the answer is only about the directories it names", in[2], want)
	}
	if !strings.Contains(in[0], `"clientInfo"`) || !strings.Contains(in[0], `"name":"`+codexAskClient+`"`) {
		t.Errorf("initialize is %s, want a clientInfo naming %q", in[0], codexAskClient)
	}
}

// A CODEX_HOME posse does not have is UNSET for the child, never set empty —
// RuntimeHomePin's second arm, as environment. An empty value is a third
// state posse and the runtimes read differently from unset, and the seat's
// line spells it `/usr/bin/env -u CODEX_HOME` for the same reason.
func TestCodexHooksAskUnsetsAHomeItDoesNotHave(t *testing.T) {
	t.Setenv(CodexHomeVar, "")
	cwd := ctCwd(t)
	bin, witness := ctFake(t, ctTalk(ctListReply(cwd, ctOurs(t)), ""))
	if _, err := ctAsk(bin, cwd).Flags(); err != nil {
		t.Fatal(err)
	}
	if got := ctWitness(t, witness)["home"]; len(got) != 1 || got[0] != "<unset>" {
		t.Errorf("with posse's own CODEX_HOME empty the child saw %v, want it UNSET — an empty value is a cwd-relative home to some readers and the real home to others, so the one thing it must never be is typed", got)
	}
}

// A RELATIVE CODEX_HOME reaches the child ABSOLUTE, resolved against POSSE's
// cwd and not the seat's. This is ranger-base-c33j6's shape surviving the fix
// built to end it (ranger-base-sczjy): the ask runs with the seat's cwd, so a
// relative value handed through verbatim would name a directory under the
// SESSION dir while posse read one under its own.
func TestCodexHooksAskMakesARelativeHomeAbsoluteBeforeTheChildSeesIt(t *testing.T) {
	t.Setenv(CodexHomeVar, "relcodex")
	cwd := ctCwd(t)
	bin, witness := ctFake(t, ctTalk(ctListReply(cwd, ctOurs(t)), ""))
	if _, err := ctAsk(bin, cwd).Flags(); err != nil {
		t.Fatal(err)
	}
	posseCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got := ctWitness(t, witness)["home"]
	if len(got) != 1 || !filepath.IsAbs(got[0]) {
		t.Fatalf("the child saw CODEX_HOME=%v, want an absolute path", got)
	}
	if want := filepath.Join(posseCwd, "relcodex"); got[0] != want {
		t.Errorf("the child saw CODEX_HOME=%q, want %q — the value is made absolute against POSSE's cwd (runtimeHomeOverride), because that is the directory posse itself read, and the child runs in the seat's", got[0], want)
	}
	if got[0] == filepath.Join(cwd, "relcodex") {
		t.Errorf("the child resolved CODEX_HOME against the SEAT's cwd (%q) — that is the divergence the pin exists to stop", got[0])
	}
}

// A notification between the answers, an answer to an id nobody asked for,
// and both before the real one. MEASURED: `remoteControl/status/changed`
// really does arrive in the middle, so this is the stream and not a hostile
// fixture.
func TestCodexHooksAskSkipsEveryMessageThatIsNotItsAnswer(t *testing.T) {
	t.Setenv(CodexHomeVar, "")
	cwd := ctCwd(t)
	ours := ctOurs(t)
	noise := `{"method":"loginChanged","params":{}}` + "\n" +
		`{"id":99,"result":{"data":[]}}` + "\n" +
		`{"method":"sessionConfigured","params":{"n":1}}`
	body := "while IFS= read -r line; do\n" +
		"  printf 'in=%s\\n' \"$line\" >> @W@\n" +
		"  case $line in\n" +
		"    *'\"initialize\"'*) printf '%s\\n' " + shQuote(ctNotify) + "; printf '%s\\n' " + shQuote(ctInitReply) + " ;;\n" +
		"    *'hooks/list'*) printf '%s\\n' " + shQuote(noise) + "; printf '%s\\n' " + shQuote(ctListReply(cwd, ours)) + " ;;\n" +
		"  esac\n" +
		"done\n"
	bin, _ := ctFake(t, body)
	flags, err := ctAsk(bin, cwd).Flags()
	if err != nil {
		t.Fatalf("the ask gave up on a stream carrying the traffic codex really sends: %v", err)
	}
	if want := ctWantState(nil, ours); flags.State != want {
		t.Errorf("the trust line is\n  %s\nwant\n  %s", flags.State, want)
	}
}

// The child is ENDED. The fake answers and then becomes a process that
// appends a byte to a file every 50 ms, so "the ask returned" and "the child
// is still doing something" are two separate readings — which is the only way
// to tell a bounded call from one that happened to be quick.
//
// TWO READINGS, because there are two ways to leave a child behind and
// neither sees the other. A process that is STILL RUNNING keeps ticking; one
// that was killed and never reaped is a zombie, which does nothing at all and
// still holds a pid and a slot. The tick file catches the first, `kill -0`
// catches the second — and the mutant that drops `cmd.Wait()` while leaving
// the deferred cancel in place is exactly the second, so an arm with only the
// tick passes over it (MEASURED 2026-11-06, which is how this arm came to
// have both).
//
// THE PID AND THE FIRST TICK ARE WRITTEN BEFORE THE CONVERSATION, and that is
// the fix for a flake rather than a detail. The first cut had the fake write
// its pid AFTER printing the answer, and it reddened once in about
// twenty-five runs of the loaded suite and never once alone (MEASURED
// 2026-11-06: 16 solo runs green, and green twice more under the unrelated
// mutant it had reddened under). posse cancels as soon as it has read the
// answer, so on a loaded box the SIGKILL can outrun the fake's own
// bookkeeping — and then the arm failed on its own precondition, with a
// sentence about the fixture, over a tree that was correct. Written at
// startup, both are there before posse has anything to cancel.
func TestCodexHooksAskEndsTheAppServerItStarted(t *testing.T) {
	t.Setenv(CodexHomeVar, "")
	cwd := ctCwd(t)
	dir := t.TempDir()
	pidfile, tick := filepath.Join(dir, "pid"), filepath.Join(dir, "tick")
	// `exec`, so the ticker IS the pid posse started: a forked one would be a
	// grandchild, which is a different claim (that one is cmd.WaitDelay's,
	// and this arm would then be pinning the fixture's shell).
	body := "printf '%s\\n' \"$$\" > " + shQuote(pidfile) + "\n" +
		"printf x >> " + shQuote(tick) + "\n" +
		ctTalk(ctListReply(cwd, ctOurs(t)),
			"      exec /bin/sh -c 'while :; do printf x >> \"$1\"; /bin/sleep 0.05; done' sh "+shQuote(tick)+"\n")
	bin, _ := ctFake(t, body)
	if _, err := ctAsk(bin, cwd).Flags(); err != nil {
		t.Fatal(err)
	}

	before := ctSize(t, tick)
	if before == 0 {
		t.Fatal("the fake app-server never wrote its first tick, so this arm measured nothing — it is the positive control, and without it a fixture that never started reads exactly like a child that was ended")
	}
	raw, err := os.ReadFile(pidfile)
	if err != nil {
		t.Fatalf("the fake app-server never wrote its own pid, so the second reading below measures nothing: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		t.Fatalf("cannot read the fake's pid from %q: %v", raw, err)
	}
	kill := func() { _ = syscall.Kill(pid, syscall.SIGKILL) } // never leave the fixture's own leak behind

	time.Sleep(400 * time.Millisecond) // eight ticks' worth
	if after := ctSize(t, tick); after != before {
		kill()
		t.Errorf("the `codex app-server` posse started is STILL RUNNING: its tick file grew from %d to %d byte(s) after the ask returned. D7 says one app-server per launch, and one that is not ended is a process leaked per seat — the launch lock is released and the child is still there",
			before, after)
	}
	// The child was killed AND reaped before Flags returned, so this is a
	// read and not a race. A pid that still answers here with a tick file
	// that stopped growing is a ZOMBIE: killed, never waited on, holding a
	// pid and a process slot for as long as posse itself lives.
	if err := syscall.Kill(pid, 0); err == nil {
		kill()
		t.Errorf("pid %d still answers `kill -0` while its tick file has stopped growing — the app-server was ended and never reaped, so each launch leaves a zombie behind (cmd.Wait is the other half of ending a child, not bookkeeping)", pid)
	} else if !errors.Is(err, syscall.ESRCH) {
		t.Errorf("kill -0 %d answered %v, want ESRCH — the process should be gone and reaped by the time Flags returns", pid, err)
	}
}

// ctSize is the tick file's size, the one reading the arm above turns on.
func ctSize(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("cannot read the fake's tick file: %v", err)
	}
	return fi.Size()
}

// ─── the refusals ────────────────────────────────────────────────────────────

func TestCodexHooksAskRefusesRatherThanTypeALineItCannotVouchFor(t *testing.T) {
	t.Setenv(CodexHomeVar, "")
	ours := ctOurs(t)
	withRow := func(r ctRow) []ctRow { return append(append([]ctRow{}, r), ours...) }
	foreign := ctRow{key: "/op/.codex/hooks.json:stop:0:0", source: "user", command: "/x.sh", hash: "sha256:" + strings.Repeat("f", 64), sourcePath: "/op/.codex/hooks.json"}

	for _, c := range []struct {
		what string
		// body: the fake's conversation, "" meaning "a normal one
		// answering rows".
		body string
		rows []ctRow
		want string // a substring of the sentence the refusal has to carry
	}{
		// D7's first named case.
		{what: "a managed hook", rows: withRow(ctRow{key: "/Library/Application Support/Codex/managed.json:stop:0:0", source: "managed", command: "/mdm.sh", hash: "sha256:" + strings.Repeat("9", 64), sourcePath: "/Library/Application Support/Codex/managed.json", managed: true}),
			want: "is a MANAGED hook"},
		// D7's second: the ask fails. Four shapes of it.
		{what: "the child never answers", body: "exec sleep 30\n", want: "codex did not answer inside the bound"},
		{what: "the child dies before answering", body: "echo 'codex: no such subcommand' >&2\nexit 7\n", want: "codex exited 7"},
		{what: "the child writes bytes that are not JSON",
			body: "while IFS= read -r line; do printf 'not json at all\\n'; done\n", want: "cannot read as JSON"},
		{what: "the child writes JSON that is not a message",
			body: ctTalkWithListReply(`[1,2,3]`), want: "cannot read as a JSON-RPC message"},
		{what: "the child answers with a JSON-RPC error",
			body: ctTalkWithListReply(`{"id":2,"error":{"code":-32601,"message":"hooks/list is not a method here"}}`),
			want: "hooks/list is not a method here"},
		{what: "the child answers with neither a result nor an error",
			body: ctTalkWithListReply(`{"id":2}`), want: "neither a result nor an error"},
		{what: "the answer is not the shape posse reads",
			body: ctTalkWithListReply(`{"id":2,"result":{"data":"nope"}}`), want: "not the shape posse reads"},
		{what: "the answer is about no directory at all",
			body: ctTalkWithListReply(`{"id":2,"result":{"data":[]}}`), want: "answered about no directory at all"},
		{what: "the child closes its stdout without answering",
			body: "while IFS= read -r line; do case $line in *'\"initialize\"'*) printf '%s\\n' " + shQuote(ctInitReply) + ";; *'hooks/list'*) exit 0;; esac; done\n",
			want: "stdout ended before it answered"},
		// The drift guard: an answer that does not account for posse's own
		// hooks is an answer about a different line.
		{what: "no session-flags hook at all", rows: []ctRow{foreign},
			want: "named 0 hook(s) from the session-flags layer and posse declared 8"},
		{what: "one of posse's hooks missing", rows: ours[:len(ours)-1],
			want: "named 7 hook(s) from the session-flags layer and posse declared 8"},
		{what: "a session-flags hook posse did not declare",
			rows: append(append([]ctRow{}, ctRow{key: "/<session-flags>/config.toml:stop:0:1", source: codexSessionFlagsSource, command: "/somebody/elses/handler", hash: "sha256:" + strings.Repeat("1", 64), sourcePath: "/<session-flags>/config.toml"}), ours...),
			want: "reports a session-flags hook posse did not declare"},
		// Shapes posse cannot put on a line.
		{what: "the same key twice", rows: append(append([]ctRow{}, ours[0]), ours...),
			want: "named the key"},
		{what: "a hook with no hash", rows: withRow(ctRow{key: "/op/h.json:stop:0:0", source: "user", command: "/x.sh", sourcePath: "/op/h.json"}),
			want: "an empty trust hash"},
		{what: "a key holding a single quote", rows: withRow(ctRow{key: "/op/it's/h.json:stop:0:0", source: "user", command: "/x.sh", hash: foreign.hash, sourcePath: "/op/it's/h.json"}),
			want: `holds '\'' at offset`},
		{what: "a key holding a double quote", rows: withRow(ctRow{key: `/op/"q"/h.json:stop:0:0`, source: "user", command: "/x.sh", hash: foreign.hash, sourcePath: "/op/q/h.json"}),
			want: `holds '"' at offset`},
		{what: "a key holding a backslash", rows: withRow(ctRow{key: `/op/b\s/h.json:stop:0:0`, source: "user", command: "/x.sh", hash: foreign.hash, sourcePath: "/op/bs/h.json"}),
			want: `holds '\\' at offset`},
		{what: "a key holding a newline", rows: withRow(ctRow{key: "/op/two\nlines/h.json:stop:0:0", source: "user", command: "/x.sh", hash: foreign.hash, sourcePath: "/op/h.json"}),
			want: "holds a control byte"},
		{what: "a hash holding a double quote", rows: withRow(ctRow{key: "/op/h.json:stop:0:0", source: "user", command: "/x.sh", hash: `sha256:"x"`, sourcePath: "/op/h.json"}),
			want: "the trust hash of hook"},
	} {
		t.Run(c.what, func(t *testing.T) {
			cwd := ctCwd(t)
			body := c.body
			if body == "" {
				body = ctTalk(ctListReply(cwd, c.rows), "")
			}
			bin, _ := ctFake(t, body)
			a := ctAsk(bin, cwd)
			if strings.Contains(body, "sleep") {
				a.Timeout = 2 * time.Second // the arm is the bound, not the wait
			}
			flags, err := a.Flags()
			if err == nil {
				t.Fatalf("%s: the ask returned a trust line instead of refusing:\n  %s", c.what, flags.State)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s: the refusal is\n  %v\nand it does not carry %q — the sentence IS the deliverable here: a launch fails before any turn, and whoever reads it next has only this", c.what, err, c.want)
			}
			if flags.State != "" || flags.Declaration != "" {
				t.Errorf("%s: a refusal handed back flags anyway (%+v) — a caller that ignores the error would type them", c.what, flags)
			}
		})
	}
}

// ctTalkWithListReply is ctTalk for an arm whose hooks/list answer is a
// literal rather than a rendered inventory.
func ctTalkWithListReply(reply string) string {
	return "while IFS= read -r line; do\n" +
		"  case $line in\n" +
		"    *'\"initialize\"'*) printf '%s\\n' " + shQuote(ctInitReply) + "; printf '%s\\n' " + shQuote(ctNotify) + " ;;\n" +
		"    *'hooks/list'*) printf '%s\\n' " + shQuote(reply) + " ;;\n" +
		"  esac\n" +
		"done\n"
}

// ctNoSuchCodex is a BARE name nothing on this box answers to — bare, because
// protocolBin hands a name that spells a path straight to exec (an explicit
// binary is an answer, not a search), so only a bare one reaches the refusal
// this table is about.
const ctNoSuchCodex = "codex-ranger-base-fwwl8-no-such-binary"

// The refusals that need no child at all, because they come before one is
// started — which is the point: a launch that cannot be asked honestly must
// not pay for a process to find out.
func TestCodexHooksAskRefusesBeforeStartingAnythingItCannotAsk(t *testing.T) {
	t.Setenv(CodexHomeVar, "")
	for _, c := range []struct {
		what string
		ask  CodexHooksAsk
		want string
	}{
		{"a relative seat cwd", CodexHooksAsk{Exe: ctExe, App: "codex", Cwd: "session", Bin: ctNoSuchCodex},
			"the seat's cwd must be an absolute path"},
		{"an exe the declaration refuses", CodexHooksAsk{Exe: "posse", App: "codex", Cwd: "/tmp", Bin: ctNoSuchCodex},
			"the handler must be an absolute path"},
		{"an app name that is not a name", CodexHooksAsk{Exe: ctExe, App: "codex:evil", Cwd: "/tmp", Bin: ctNoSuchCodex},
			"is not a runtime name posse can put in an OSC 7501 report"},
		{"a codex binary nothing outside the gates answers to", CodexHooksAsk{Exe: ctExe, App: "codex", Cwd: "/tmp", Bin: ctNoSuchCodex},
			"posse cannot ask codex which hooks would load"},
	} {
		t.Run(c.what, func(t *testing.T) {
			_, err := c.ask.Flags()
			if err == nil {
				t.Fatalf("%s: no refusal", c.what)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s: the refusal is %v, want it to carry %q", c.what, err, c.want)
			}
		})
	}
}

// ─── the --ask arm's own argv ────────────────────────────────────────────────

// The CLI arm is parsed here and no further: anything past the parse starts a
// REAL codex app-server against the operator's own home, which is not a
// hermetic test (codexInstalledVersion's rule — the seam is the reader, and
// this arm's reader is CodexHooksAsk, pinned above). What a test can and must
// hold is that a malformed `-c` pairing is refused rather than silently
// dropping a value the seat's line carries.
func TestHookReportAskArmRefusesAnUnpairedConfigValue(t *testing.T) {
	t.Parallel() // argv in, a sentence out: no env, no cwd, no child
	for _, argv := range [][]string{
		{"posse", HookReportVerb, HookReportAskFlag, "codex", "-c"},
		{"posse", HookReportVerb, HookReportAskFlag, "codex", "model=x"},
		{"posse", HookReportVerb, HookReportAskFlag, "codex", "-c", "a=1", "-x", "b=2"},
	} {
		var out, errb strings.Builder
		noTTY := func() (io.WriteCloser, error) { return nil, os.ErrNotExist }
		if code := runHookReport(argv, strings.NewReader(""), &out, &errb, noTTY); code != 1 {
			t.Errorf("%q exited %d, want 1 — a dropped `-c` is a line the ask asked about and the seat will not get", argv, code)
		}
		if !strings.Contains(errb.String(), "usage: posse "+HookReportVerb+" "+HookReportAskFlag) {
			t.Errorf("%q said %q, want a usage naming the arm", argv, errb.String())
		}
		if out.String() != "" {
			t.Errorf("%q printed %q on stdout — a refusal must print nothing a launch script could type", argv, out.String())
		}
	}
}

// ─── fixture plumbing ────────────────────────────────────────────────────────

// ctWitness reads the fake's witness file into key → values, in order.
func ctWitness(t *testing.T, path string) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the fake app-server left no witness, so this arm measured nothing: %v", err)
	}
	out := map[string][]string{}
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[k] = append(out[k], v)
	}
	return out
}

func ctSameLines(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
