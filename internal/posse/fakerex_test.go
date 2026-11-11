package posse

// The fake rex (ranger-base-qlxb5) — the fourth substrate the test binary
// doubles as, beside fakeHerdr, fakeGh and fakeBd. TestMain dispatches to it
// on the VERB, and this file holds the verb set, the state and the answers.
//
// NO BUILD LINE, so it is compiled into all three suite arms: TestMain is in
// every arm and the dispatcher it runs has to be able to reach this. The
// TESTS that drive it are arm 1's (muxrex_qa_test.go), which is where their
// wall clock belongs.
//
// WHAT IT ANSWERS WITH. The fake's whole state is files under fakeDir(),
// written by the parent and read by the child, because the child is a fresh
// exec of this binary and shares nothing else:
//
//	rex-sessions.json  the server's sessions, each with its blocks
//	rex-status.json    blockID -> a QUEUE of record sets, one taken per call
//	rex-process.json   blockID -> the block's process table
//	rex-capture.json   blockID -> the block's terminal text
//	rex-endpoint       the server address `whoami --json` answers
//	rex-fail           "<substring of the argv>\t<message>": that call exits 3
//	rex-calls.log      every invocation, one argv per line
//
// The status QUEUE is what makes Wait testable: a one-element queue is a
// constant answer, and a longer one steps the seat through states as the
// poller re-reads it. That is the only way to drive a poller from outside it
// without a clock a test cannot see.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── the verb set, and finding it past the globals ───────────────────────────

// rexFakeVerbs is every verb Rex (rex.go) actually spells. It is the set
// TestMain dispatches on, so it is deliberately NOT "every verb the real rex
// has": a verb nothing calls would widen the overlap with the other three
// fakes for no benefit.
var rexFakeVerbs = map[string]bool{
	"new": true, "ls": true, "kill": true, "send": true, "send-key": true,
	"capture": true, "focus": true, "block": true, "whoami": true,
}

// rexFakeGlobalValueFlags are the global flags that take a SEPARATE value, so
// that skipping the globals does not read one of their values as the verb.
// `--autostart=false` is joined and needs no entry; `--color never` and
// `--server <addr>` do.
var rexFakeGlobalValueFlags = map[string]bool{
	"--color": true, "--server": true, "-S": true, "-C": true, "--timeout": true, "--client": true,
}

// rexFakeVerb is the rex verb this argv names, or "" when it names none —
// which is TestMain's test for "this is a rex call". The globals lead (see
// Rex.rexGlobals), so this skips flags and their values until the first
// bare word.
func rexFakeVerb(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			if rexFakeGlobalValueFlags[a] {
				i++
			}
			continue
		}
		if rexFakeVerbs[a] {
			return a
		}
		return ""
	}
	return ""
}

// ─── the state on disk ───────────────────────────────────────────────────────

type fakeRexSession struct {
	SessionID string         `json:"session_id"`
	Label     string         `json:"label"`
	Blocks    []fakeRexBlock `json:"blocks"`
}

type fakeRexBlock struct {
	BlockID string `json:"block_id"`
	Label   string `json:"label"`
	Flavor  string `json:"flavor"`
}

func fakeRexPath(name string) string { return filepath.Join(fakeDir(), name) }

func fakeRexLoadSessions() []fakeRexSession {
	var ss []fakeRexSession
	if b, err := os.ReadFile(fakeRexPath("rex-sessions.json")); err == nil {
		json.Unmarshal(b, &ss)
	}
	return ss
}

func fakeRexSaveSessions(ss []fakeRexSession) {
	b, _ := json.Marshal(ss)
	os.WriteFile(fakeRexPath("rex-sessions.json"), b, 0o644)
}

func fakeRexLoadMap(name string, into any) {
	if b, err := os.ReadFile(fakeRexPath(name)); err == nil {
		json.Unmarshal(b, into)
	}
}

// ─── the child ───────────────────────────────────────────────────────────────

func fakeRex(args []string) int {
	if f, _ := os.OpenFile(fakeRexPath("rex-calls.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); f != nil {
		fmt.Fprintln(f, strings.Join(args, " "))
		f.Close()
	}
	// The refusal lever, in rex's own shape: `Error: <what>` on stderr with
	// a non-zero exit and nothing on stdout (MEASURED on the real CLI —
	// rex.go's head comment, item 2). One marker file so a test can red a
	// single call rather than the whole fake.
	if b, err := os.ReadFile(fakeRexPath("rex-fail")); err == nil {
		spec := strings.SplitN(strings.TrimSpace(string(b)), "\t", 2)
		if len(spec) == 2 && spec[0] != "" && strings.Contains(strings.Join(args, " "), spec[0]) {
			fmt.Fprintf(os.Stderr, "Error: %s\n\nUsage:\n  rex %s\n", spec[1], spec[0])
			return 3
		}
	}
	verb := rexFakeVerb(args)
	flags, words := fakeRexSplit(args, verb)
	switch verb {
	case "new":
		return fakeRexNew(flags, words)
	case "ls":
		return fakeRexLs()
	case "kill":
		return fakeRexKill(words)
	case "send", "send-key", "focus":
		// Recorded in the log; nothing else observable — but the block
		// target is still resolved, so a seat handed over without its
		// session is refused here the way the real CLI refuses it
		// (fakeRexScoped). `focus` spells its block as a bare word.
		bid := flags["-b"]
		if verb == "focus" && bid == "" && len(words) > 0 {
			bid = words[0]
		}
		if !fakeRexScoped(flags["-s"], bid) {
			return 3
		}
		return 0
	case "capture":
		return fakeRexCapture(flags)
	case "whoami":
		return fakeRexWhoami()
	case "block":
		return fakeRexBlockCmd(flags, words)
	}
	fmt.Fprintf(os.Stderr, "Error: unknown command %q for \"rex\"\n", verb)
	return 2
}

// fakeRexSplit is the argv past the verb, as a flag map and the bare words.
// `--k=v` and `--k v` both land in the map, which is what the real CLI
// accepts and what lets a test assert on either spelling posse might use.
func fakeRexSplit(args []string, verb string) (map[string]string, []string) {
	flags := map[string]string{}
	var words []string
	at := 0
	for i, a := range args {
		if a == verb {
			at = i + 1
			break
		}
	}
	rest := args[at:]
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "--":
			words = append(words, rest[i+1:]...)
			return flags, words
		case strings.HasPrefix(a, "--") && strings.Contains(a, "="):
			kv := strings.SplitN(a, "=", 2)
			flags[kv[0]] = kv[1]
		case strings.HasPrefix(a, "-"):
			// A value-taking flag of the verbs this fake answers: -s, -b,
			// --format, --cwd. Everything else (--json, --trim, --short)
			// is a bare switch and gets "1".
			switch a {
			case "-s", "-b", "-w", "--format", "--cwd", "-c", "--session", "--block", "--window", "--shell":
				if i+1 < len(rest) {
					flags[a] = rest[i+1]
					i++
				}
			default:
				flags[a] = "1"
			}
		default:
			words = append(words, a)
		}
	}
	return flags, words
}

func fakeRexNew(flags map[string]string, words []string) int {
	label := ""
	if len(words) > 0 {
		label = words[0]
	}
	ss := fakeRexLoadSessions()
	sid := fmt.Sprintf("session:fake-%04d", len(ss)+1)
	bid := fmt.Sprintf("block:fake-%04d", len(ss)+1)
	ss = append(ss, fakeRexSession{SessionID: sid, Label: label,
		Blocks: []fakeRexBlock{{BlockID: bid, Label: "shell", Flavor: RexTerminalFlavor}}})
	fakeRexSaveSessions(ss)
	// The command the block was created with, so a test can read the env
	// wrapper back (rexShellCommand) without parsing the call log.
	if len(words) > 1 {
		os.WriteFile(fakeRexPath("rex-new-command"), []byte(strings.Join(words[1:], "\n")), 0o644)
	}
	out, _ := json.Marshal(map[string]any{
		"session_id": sid,
		"initial_windows": []map[string]any{
			{"window_id": "window:fake", "block_ids": []string{bid}},
		},
	})
	fmt.Println(string(out))
	return 0
}

func fakeRexLs() int {
	rows := []map[string]string{}
	for _, s := range fakeRexLoadSessions() {
		rows = append(rows, map[string]string{"session_id": s.SessionID, "label": s.Label})
	}
	out, _ := json.Marshal(map[string]any{"sessions": rows})
	fmt.Println(string(out))
	return 0
}

// fakeRexKill is `rex kill <session>`, and it REFUSES an id the fake does not
// hold, in the real CLI's own words: `Error: session target "<id>" not found`
// on stderr, nothing on stdout, exit 3. MEASURED 2026-11-06 on rex
// 0.0.1793377904 against a scratch server, for an unknown UUID, a word that
// is no id at all, and a session destroyed a moment earlier
// (docs/notes.d/ranger-base-43mxx.md).
//
// It used to answer 0 for every id, which made that refusal unreachable from
// the suite — and the refusal is now a BRANCH in production code
// (rexSessionGone, ADR 0080 D10: an id the server no longer knows counts as
// already destroyed), so a fake that cannot produce it is a fake that makes
// the branch untestable and its pin vacuous.
func fakeRexKill(words []string) int {
	if len(words) == 0 {
		fmt.Fprintln(os.Stderr, "Error: kill needs a session")
		return 2
	}
	var kept []fakeRexSession
	var found bool
	for _, s := range fakeRexLoadSessions() {
		if s.SessionID != words[0] && s.Label != words[0] {
			kept = append(kept, s)
			continue
		}
		found = true
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Error: session target %q not found\n", words[0])
		return 3
	}
	fakeRexSaveSessions(kept)
	return 0
}

// ─── a block is addressed within its session (ranger-base-nfc2b) ─────────────

// fakeRexScoped is the refusal the real CLI makes for a block target that the
// `-s` session does not hold — printed and exit 3, true when the lookup may
// go ahead.
//
// IT IS WHY THIS FAKE EXISTS AT ALL, and until 2026-11-10 it was the one
// thing the fake got wrong: every block verb here resolved `-b` out of a flat
// map and ignored `-s`, so a pane-only Seat read exactly like a full one and
// the suite was green over a `posse peek` that could not read a seat on the
// host. MEASURED 2026-11-10 on this box, rex CLI 0.0.1794069975, against a
// server holding three live sessions: `capture -s <its session> -b <a live
// block>` printed the screen, and both `capture -s "" -b <that same block>`
// and `capture -b <that same block>` answered `Error: block target "…" not
// found` — as did `block call -s "" -b <it> program_status`.
//
// AN UNSCOPED `-b` IS REFUSED HERE AND THE REAL CLI SOMETIMES ANSWERS IT,
// which is deliberate and the one place this fake is STRICTER than rex. With
// no `-s` the CLI resolves the block inside one default session — `rex window
// ls` with no `-s` answered for the most recently created session on that
// same server — so an unscoped verb lands for whichever seat happens to be
// default and refuses every other one. posse must not depend on which seat
// that is, so there is nothing here for it to depend on.
func fakeRexScoped(sid, bid string) bool {
	if bid == "" {
		return true // no block target: not this check's business
	}
	for _, s := range fakeRexLoadSessions() {
		if s.SessionID != sid && s.Label != sid {
			continue
		}
		for _, b := range s.Blocks {
			if b.BlockID == bid {
				return true
			}
		}
	}
	fmt.Fprintf(os.Stderr, "Error: block target %q not found\n", bid)
	return false
}

func fakeRexCapture(flags map[string]string) int {
	if !fakeRexScoped(flags["-s"], flags["-b"]) {
		return 3
	}
	var text map[string]string
	fakeRexLoadMap("rex-capture.json", &text)
	fmt.Println(text[flags["-b"]])
	return 0
}

func fakeRexWhoami() int {
	ep := "unix:///fake/rex/server.sock"
	if b, err := os.ReadFile(fakeRexPath("rex-endpoint")); err == nil {
		ep = strings.TrimSpace(string(b))
	}
	out, _ := json.Marshal(map[string]string{"endpoint": ep})
	fmt.Println(string(out))
	return 0
}

// fakeRexBlockCmd answers `block ls` and `block call`.
func fakeRexBlockCmd(flags map[string]string, words []string) int {
	if len(words) == 0 {
		fmt.Fprintln(os.Stderr, "Error: block needs a subcommand")
		return 2
	}
	switch words[0] {
	case "ls", "list":
		sid := flags["-s"]
		if sid == "" {
			// The real CLI's own refusal, which is why Rex.Blocks is one
			// call per session (MEASURED).
			fmt.Fprintln(os.Stderr, "Error: rex block ls --json requires one session. Pass -s SESSION")
			return 2
		}
		rows := []map[string]string{}
		for _, s := range fakeRexLoadSessions() {
			if s.SessionID != sid {
				continue
			}
			for _, b := range s.Blocks {
				rows = append(rows, map[string]string{
					"block_id": b.BlockID, "label": b.Label, "flavor": b.Flavor,
					"window_id": "window:fake", "placement": "placed",
				})
			}
		}
		out, _ := json.Marshal(map[string]any{"blocks": rows})
		fmt.Println(string(out))
		return 0
	case "call":
		if len(words) < 2 {
			fmt.Fprintln(os.Stderr, "Error: call needs a method")
			return 2
		}
		if !fakeRexScoped(flags["-s"], flags["-b"]) {
			return 3
		}
		switch words[1] {
		case "program_status":
			return fakeRexProgramStatus(flags["-b"])
		case "process":
			var procs map[string]json.RawMessage
			fakeRexLoadMap("rex-process.json", &procs)
			raw, ok := procs[flags["-b"]]
			if !ok {
				raw = json.RawMessage(`{"last_exit":null}`)
			}
			fmt.Println(string(raw))
			return 0
		}
		fmt.Fprintf(os.Stderr, "Error: unknown method %q\n", words[1])
		return 3
	}
	fmt.Fprintf(os.Stderr, "Error: unknown block subcommand %q\n", words[0])
	return 2
}

// fakeRexProgramStatus takes the head of this block's queue and leaves the
// LAST entry in place forever — so a one-element queue is a constant answer
// and a longer one is a sequence a poller walks (head comment).
func fakeRexProgramStatus(block string) int {
	var queues map[string][]json.RawMessage
	fakeRexLoadMap("rex-status.json", &queues)
	q := queues[block]
	if len(q) == 0 {
		fmt.Println(`{"records":[]}`)
		return 0
	}
	fmt.Printf("{\"records\":%s}\n", string(q[0]))
	if len(q) > 1 {
		queues[block] = q[1:]
		b, _ := json.Marshal(queues)
		os.WriteFile(fakeRexPath("rex-status.json"), b, 0o644)
	}
	return 0
}

// ─── the parent's side ───────────────────────────────────────────────────────

// newTestRexMux is a RexMux wired to the fake, with a poll interval a suite
// can afford (RexMux.Poll's doc says why that cannot be the production one).
func newTestRexMux(t *testing.T) (RexMux, string) {
	t.Helper()
	dir := fakeDirOf(t)
	return RexMux{R: Rex{Bin: fakeBinFor(t, "rex")}, Poll: time.Millisecond}, dir
}

// rexSeedSessions writes the server's sessions.
func rexSeedSessions(t *testing.T, ss ...fakeRexSession) {
	t.Helper()
	b, err := json.Marshal(ss)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeDirOf(t), "rex-sessions.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// rexSeedStatus writes one block's program_status QUEUE: each element is one
// call's record set.
func rexSeedStatus(t *testing.T, block string, sets ...[]RexStatusRecord) {
	t.Helper()
	queues := map[string][]json.RawMessage{}
	p := filepath.Join(fakeDirOf(t), "rex-status.json")
	if b, err := os.ReadFile(p); err == nil {
		json.Unmarshal(b, &queues)
	}
	for _, set := range sets {
		if set == nil {
			set = []RexStatusRecord{}
		}
		raw, err := json.Marshal(set)
		if err != nil {
			t.Fatal(err)
		}
		queues[block] = append(queues[block], raw)
	}
	b, err := json.Marshal(queues)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// rexSeedProcess writes one block's process table.
func rexSeedProcess(t *testing.T, block string, proc RexProcess) {
	t.Helper()
	procs := map[string]json.RawMessage{}
	p := filepath.Join(fakeDirOf(t), "rex-process.json")
	if b, err := os.ReadFile(p); err == nil {
		json.Unmarshal(b, &procs)
	}
	raw, err := json.Marshal(proc)
	if err != nil {
		t.Fatal(err)
	}
	procs[block] = raw
	b, err := json.Marshal(procs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func rexSeedCapture(t *testing.T, block, text string) {
	t.Helper()
	m := map[string]string{}
	p := filepath.Join(fakeDirOf(t), "rex-capture.json")
	if b, err := os.ReadFile(p); err == nil {
		json.Unmarshal(b, &m)
	}
	m[block] = text
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// rexSeedFail makes every call whose argv contains `match` exit 3 with
// rex's own error shape.
func rexSeedFail(t *testing.T, match, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fakeDirOf(t), "rex-fail"), []byte(match+"\t"+msg), 0o644); err != nil {
		t.Fatal(err)
	}
}

// rexCalls is every rex invocation this test has made, one argv per line.
func rexCalls(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fakeDirOf(t), "rex-calls.log"))
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// rexNewCommand is the block command the last `rex new` was given, one argv
// word per element.
func rexNewCommand(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fakeDirOf(t), "rex-new-command"))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}
