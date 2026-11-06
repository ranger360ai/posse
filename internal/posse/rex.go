package posse

// Every rex invocation goes through Rex.capture — the Rex twin of Herdr.Run,
// and the client half of the second occupant of ADR 0075 §2's multiplexer
// slot. muxrex.go holds the occupant; this file holds the CLI and nothing
// else, so that "which argv answers which question" is in one place and the
// ten verbs above it read as verbs.
//
// THREE WAYS IT IS NOT HERDR'S CLIENT, each measured rather than assumed:
//
//  1. NOT ON PATH. Rex ships its CLI inside the app bundle on darwin and as
//     one server-and-CLI binary from its Linux package, so there is no
//     `exec.LookPath` to ask — and off darwin there is worse than nothing to
//     ask, because `rex` on a Debian-family host is an unrelated Perl tool.
//     The binary comes from config (`rex_bin:`) or RHQ_REX_BIN, defaulting
//     to the platform's install path below (ADR 0076 Consequences, "Rex's
//     CLI is not on PATH"; ADR 0080 Context, "Rex's binary").
//  2. NO JSON ENVELOPE. herdr answers every control call with
//     {"id":…,"result":…} or {"error":{"code","message"}}, which is why
//     Herdr.Run decodes one. rex answers with the result object itself and
//     reports failure as plain text on stderr with a non-zero exit —
//     MEASURED 2026-10-31 on rex 0.0.1793377904: `block call -b <missing>
//     program_status` printed `Error: block target "…" not found` and exited
//     3, and an unknown key printed `Error: invalid key "notakey"` and
//     exited 2. So there is no code to branch on and no envelope to find on
//     stderr; the error is the text.
//  3. AUTOSTART IS A READING HAZARD, not a convenience. `--autostart`
//     defaults TRUE and starts a missing local server. A fresh server holds
//     no sessions, so a read that autostarted one would answer "no such
//     session" for every seat posse has recorded — and §5's liveness rule
//     reads absence from the listing as death, so one autostarted read
//     could land and reclaim the whole fleet. Every READ here passes
//     `--autostart=false` and fails honestly instead; only NewSession, which
//     is posse asking for a seat, lets the default stand.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// RexBinEnv overrides the configured rex binary, the way RHQ_HERDR_BIN
// overrides herdr's. It is the testing door: the fake-rex suite points it at
// a link to the test binary.
const RexBinEnv = "RHQ_REX_BIN"

// RexBinKey is the instance-config key that names the rex CLI. There is no
// PATH lookup to fall back on (see the head comment), so an instance that
// installed Rex somewhere else says so here.
const RexBinKey = "rex_bin"

// rexDefaultBinRel is where the Rex server keeps its CLI under $HOME on
// DARWIN, as MEASURED on this box 2026-10-31 (Rex Beta 0.1.0, CLI
// 0.0.1793377904). It is a RELATIVE path joined to $HOME at call time, never
// a literal home: a path with a user name baked in is what
// `make identity-check` exists to refuse, and an instance whose Rex lives
// elsewhere sets RexBinKey.
//
// `Current` is a symlink Rex maintains across its own upgrades, so this path
// survives a Rex version bump; the version in the comment is what answered
// when it was measured, not a pin.
const rexDefaultBinRel = "Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"

// rexPackageBin is where Rex's Linux package installs it: one binary that is
// both the server and the CLI (ADR 0080 Context, "Rex's binary"). REPORTED
// from Rex's own docs 2026-11-06 and not yet MEASURED here — the always-on
// host is not up (ranger-base-yyq64) and the shakedown (ranger-base-offid)
// is what reads this path on a real install.
//
// IT IS ABSOLUTE, AND THAT IS THE DECISION. There is deliberately no
// `exec.LookPath("rex")` behind it, which is the obvious fallback and the
// one thing this must not do: Debian-family hosts package an unrelated Perl
// remote-execution tool as `rex`, so on a box without Rex a LookPath would
// not fail — it would answer with a DIFFERENT program, and posse would go
// on to drive it with rex argv. A wrong absolute path fails as a missing
// path instead (Available), which is the honest answer, and an instance
// whose Rex lives elsewhere says so in RexBinKey.
const rexPackageBin = "/usr/bin/rex"

// RexDefaultBin is this platform's Rex install path: rexDefaultBinRel under
// this process's $HOME on darwin, else the Linux package's rexPackageBin.
func RexDefaultBin() string {
	return rexDefaultBinAt(runtime.GOOS, os.Getenv("HOME"))
}

// rexDefaultBinAt is RexDefaultBin with the OS and the home named. They are
// parameters and not `runtime.GOOS` and `os.Getenv` for the reason
// meterStore's goos is one (credential.go): no build tags means neither arm
// is a branch only one box can reach, and a test on either box proves both
// — which is what `make test-linux` reads here.
//
// "" AND NOT A PATH ROOTED AT "/" when darwin's $HOME is unset, the same
// refusal ExpandTilde makes for the same reason (ranger-base-a3t1): an empty
// home used to turn every ~-path into a child of the filesystem root. The
// package path joins no home, so that question does not arise off darwin.
func rexDefaultBinAt(goos, home string) string {
	if goos != "darwin" {
		return rexPackageBin
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, rexDefaultBinRel)
}

// RexBin is this instance's rex CLI: the env override, else `rex_bin:`, else
// the platform's install path. Read here rather than in NewRexMux so that
// the one place an operator can move the binary is the one place posse looks
// for it.
func (a *App) RexBin() string {
	if bin := os.Getenv(RexBinEnv); bin != "" {
		return bin
	}
	return ExpandTilde(a.CfgGet(RexBinKey, RexDefaultBin()))
}

// Rex is the rex CLI, bound to one server.
type Rex struct {
	// Bin is the rex binary. App.RexBin resolves it; a test points it at a
	// fake.
	Bin string

	// Server is `-S/--server`: a host label or an address. "" is this box's
	// local server, which is what every posse instance uses today — the key
	// exists because `rex` takes it and a remote Rex is a one-line change,
	// not because anything here has one.
	Server string

	// ControlTimeout is the child deadline; 0 = RexControlTimeout. It is a
	// field for the same reason Herdr's is: the pins cannot afford the real
	// number, which is a ceiling on a hang rather than a latency budget.
	ControlTimeout time.Duration

	// Hangw takes the one line a blown deadline writes; nil is the
	// process's unowned-notice stream. See Herdr.Hangw for why the finding
	// is written where it happens rather than left to the caller.
	Hangw io.Writer
}

// RexControlTimeout bounds one rex child. Every call posse makes here is
// answered out of the server's memory and declares no deadline of its own —
// MEASURED 2026-10-31, `rex block call … program_status` five runs: 50, 60,
// 70, 60, 60 ms wall — so there is one ceiling and no per-call arithmetic,
// which is the half of Herdr.callDeadline that Rex does not need. The number
// is herdr's control ceiling quoted rather than a second one coined: it is
// three thousand times the measurement either way, and two ceilings for the
// same shape of call would be two numbers to keep true.
const RexControlTimeout = HerdrControlTimeout

// rexWaitDelay is the second half of the kill, for the reason
// herdrWaitDelay's doc gives: Wait does not return while a grandchild still
// holds the child's pipes.
const rexWaitDelay = 5 * time.Second

func (r Rex) hangw() io.Writer {
	if r.Hangw != nil {
		return r.Hangw
	}
	return processNotices()
}

// Available is whether this rex binary is there and runnable. It is a stat
// and not a LookPath, because the CLI is not on PATH (head comment, item 1)
// — so the question is about one absolute path and nothing else.
//
// UNKNOWN AND NEVER A NO when the path cannot be read at all: a stat that
// fails for any reason other than "not there" leaves this true, so a launch
// is never refused because a permission bit could not be read. That is ADR
// 0013 §1's direction, the same one KnownAgentKinds and HasReportAgent
// degrade in.
func (r Rex) Available() bool {
	if r.Bin == "" {
		return false
	}
	fi, err := os.Stat(r.Bin)
	if err != nil {
		return !os.IsNotExist(err)
	}
	return !fi.IsDir() && fi.Mode()&0o111 != 0
}

// ─── the child ───────────────────────────────────────────────────────────────

// rexGlobals are the flags that go ahead of every verb. The real CLI accepts
// them anywhere, but `rex new … -- COMMAND` ends flag parsing at the `--`, so
// they lead — one order for every call rather than one per verb.
//
// `--color never` so that nothing posse parses depends on whether a tty was
// attached, and `--autostart=false` for every read (head comment, item 3).
func (r Rex) rexGlobals(autostart bool) []string {
	g := []string{"--color", "never"}
	if r.Server != "" {
		g = append(g, "--server", r.Server)
	}
	if !autostart {
		g = append(g, "--autostart=false")
	}
	return g
}

// capture runs one rex child under the deadline and hands back both streams.
// Every call in this file goes through it, so the deadline has one choke
// point and no call site that can forget it (ADR 0011's loop must never
// depend on a child returning — Herdr.capture's block carries the incident
// that bought that rule).
func (r Rex) capture(args []string) (stdout []byte, stderr string, runErr error) {
	limit := r.ControlTimeout
	if limit <= 0 {
		limit = RexControlTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := protocolCmd(exec.CommandContext(ctx, r.Bin, args...))
	cmd.WaitDelay = rexWaitDelay
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	started := time.Now()
	runErr = cmd.Run()
	if ctx.Err() != nil {
		// HerdrHangError and not a type of its own, deliberately: callers
		// tell "the multiplexer said no" from "the multiplexer said nothing
		// at all" through IsHerdrHang, and gather's unclaim rules turn on
		// exactly that distinction. A second type would give a Rex instance
		// a hang no existing arm recognizes. The NAME is herdr's and the
		// meaning is the slot's; renaming it is the same mechanical bead as
		// HerdrBackend → Backend (ADR 0076 §1).
		hang := &HerdrHangError{Argv: append([]string{r.Bin}, args...), Limit: limit, Waited: time.Since(started)}
		fmt.Fprintf(r.hangw(), "◷ %s\n", hang.Error())
		return out.Bytes(), strings.TrimSpace(errb.String()), hang
	}
	return out.Bytes(), strings.TrimSpace(errb.String()), runErr
}

// RunText executes rex and returns stdout verbatim — for `capture`, whose
// output is terminal text and never JSON (terminal text that happens to look
// like JSON must not be interpreted; PaneRead carries the same rule).
func (r Rex) RunText(autostart bool, args ...string) (string, error) {
	out, errb, runErr := r.capture(append(r.rexGlobals(autostart), args...))
	if runErr != nil {
		return "", r.callErr(args, errb, runErr)
	}
	return string(out), nil
}

// Run executes rex and decodes stdout as the method's result object. There is
// no envelope to unwrap (head comment, item 2): rex prints the result and
// says no with a non-zero exit.
func (r Rex) Run(autostart bool, into any, args ...string) error {
	out, errb, runErr := r.capture(append(r.rexGlobals(autostart), args...))
	if runErr != nil {
		return r.callErr(args, errb, runErr)
	}
	if into == nil {
		return nil
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), into); err != nil {
		return Die("rex %s: cannot read the answer as JSON: %v", strings.Join(args, " "), err)
	}
	return nil
}

// callErr is what a failed rex child says. A blown deadline travels
// unwrapped — it already names the argv and the wait, and IsHerdrHang is how
// a caller tells silence from refusal — and everything else becomes the
// stderr text, which on rex is the whole of the diagnosis.
func (r Rex) callErr(args []string, errb string, runErr error) error {
	if IsHerdrHang(runErr) {
		return runErr
	}
	// `Error: <what>` plus a usage block is rex's shape for a refusal
	// (MEASURED, head comment item 2). The first line is the sentence; the
	// usage block is noise in a log.
	msg := errb
	if i := strings.Index(msg, "\nUsage:"); i >= 0 {
		msg = strings.TrimSpace(msg[:i])
	}
	if msg == "" {
		msg = runErr.Error()
	}
	return Die("rex %s: %s", strings.Join(args, " "), msg)
}

// ─── typed views of the API responses ────────────────────────────────────────

// RexSession is one row of `rex ls --json` (session.list). Two fields is all
// that method has: there is no focus and no state in it, which is why List
// reads the blocks too and why SeatInfo.Focused is false on Rex (muxrex.go).
type RexSession struct {
	SessionID string `json:"session_id"`
	Label     string `json:"label"`
}

// RexBlock is one row of `rex block ls -s <session> --json`.
type RexBlock struct {
	BlockID   string `json:"block_id"`
	Label     string `json:"label"`
	Flavor    string `json:"flavor"`
	WindowID  string `json:"window_id"`
	Placement string `json:"placement"`
}

// RexTerminalFlavor is the block flavor posse creates and the only one it can
// type into or read. A session holding none of them holds no seat.
const RexTerminalFlavor = "com.superlogical.terminal.shell"

// RexOwner is a program_status record's `owner`: "The terminal's foreground
// process when the report arrived."
//
// BOTH FIELDS ARE COMPARED, because Rex says to: "Compare it as well as pid,
// because the system reuses process IDs" (the method's own schema). A pid
// alone is the id-recycle hazard rangerhq-yt1p is about, one layer down.
type RexOwner struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
}

// RexStatusRecord is one OSC 7501 record (spec revision 0.4,
// superlogical.com/rex/docs/build/program-status).
//
// ID is "" for the ROOT record — the reports that carry no id — which is the
// one posse reads: a child record like `build/test` is a task inside the
// program, not the program's own state.
type RexStatusRecord struct {
	ID    string `json:"id"`
	State string `json:"state"`
	// Kind is what a BLOCKED program is waiting for (permission | question
	// | auth), and the protocol sets it on `blocked` only.
	Kind string `json:"kind"`
	// App is the program's name as IT reported it — `claude-code`, not
	// posse's runtime name. It is passed through rather than mapped: the
	// record says what the runtime called itself, and posse's own meta
	// (Agent, Runtime) is the authority on what it launched. A mapping here
	// would be posse guessing which app string is which runtime.
	App          string    `json:"app"`
	EffectiveApp string    `json:"effective_app"`
	Title        string    `json:"title"`
	Msg          string    `json:"msg"`
	UpdatedAt    string    `json:"updated_at"`
	Owner        *RexOwner `json:"owner"`
}

// RexProc is one process in a block's `process` answer.
type RexProc struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
}

// RexProcess is `block.method process`: what is running in the terminal.
//
// The three fields State reads are Child, Foreground and Ancestors, and what
// each proves is MEASURED (2026-10-31, rex 0.0.1793377904, this box):
//
//   - a record written BY THE SHELL carries `owner={82975,"zsh"}`, which is
//     exactly Child — and Foreground too, because the shell was at its
//     prompt. That is the "owner must not be the pane's shell" case ADR 0076
//     §3 names, and comparing against Child is how it is caught.
//   - a record written by a program the shell started carries
//     `owner={91369,"Python"}` == Foreground, with Child the shell and
//     Ancestors [the shell]. That is positive evidence that something other
//     than the shell has the keyboard.
//   - after that program exits, the record SURVIVES (Rex drops only working
//     and blocked at exit) with its dead pid, while Foreground falls back to
//     the shell. Nothing live matches the owner, which is the case that must
//     read `unknown` and never the record's last state (ranger-base-ch2rb).
type RexProcess struct {
	Child      *RexProc  `json:"child"`
	Foreground *RexProc  `json:"foreground"`
	Ancestors  []RexProc `json:"ancestors"`
	LastExit   *RexExit  `json:"last_exit"`
}

// RexExit is how the block's program ended; nil while it is still running.
type RexExit struct {
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error"`
}

// ─── the verbs ───────────────────────────────────────────────────────────────

// NewSession is `rex new <label> --cwd <cwd> --json -- <command…>`
// (session.create with one terminal block), and the ONE call here that lets
// `--autostart` stand: posse is asking for a seat, so bringing a missing
// local server up is the answer rather than a hazard.
//
// No `--keep-open`, and that is load-bearing. With it, a block whose program
// exits stays open forever, and a seat posse can never see the end of is a
// seat it can never land. Without it the block closes — MEASURED 2026-10-31:
// the session then stays in `session.list` with `windows: 0`, which is why
// List drops a session holding no terminal block (muxrex.go).
func (r Rex) NewSession(label, cwd string, command []string) (sessionID, blockID string, err error) {
	args := []string{"new"}
	if label != "" {
		args = append(args, label)
	}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	args = append(args, "--json")
	if len(command) > 0 {
		args = append(args, "--")
		args = append(args, command...)
	}
	var res struct {
		SessionID      string `json:"session_id"`
		InitialWindows []struct {
			WindowID string   `json:"window_id"`
			BlockIDs []string `json:"block_ids"`
		} `json:"initial_windows"`
	}
	if err := r.Run(true, &res, args...); err != nil {
		return "", "", err
	}
	if res.SessionID == "" {
		return "", "", Die("rex new: no session id in the answer")
	}
	for _, w := range res.InitialWindows {
		if len(w.BlockIDs) > 0 {
			return res.SessionID, w.BlockIDs[0], nil
		}
	}
	// A session with no block is a seat with nothing to type into. It is
	// returned rather than swallowed so the caller can clean it up: a
	// workspace that exists while posse believes nothing does is the
	// rangerhq-v52t orphan.
	return res.SessionID, "", Die("rex new: the session was created with no block")
}

// KillSession is `rex kill <session>` (session.destroy).
func (r Rex) KillSession(sessionID string) error {
	return r.Run(false, nil, "kill", sessionID)
}

// Sessions is `rex ls --json` (session.list). `--short=false` because the
// short ids this prints by default are a display convenience and the ids
// posse records are the full ones.
func (r Rex) Sessions() ([]RexSession, error) {
	var res struct {
		Sessions []RexSession `json:"sessions"`
	}
	if err := r.Run(false, &res, "ls", "--json", "--short=false"); err != nil {
		return nil, err
	}
	return res.Sessions, nil
}

// Blocks is `rex block ls -s <session> --json` (session.list_blocks). One
// session per call, because the CLI refuses the listing otherwise — MEASURED:
// `block ls -A --json` answers "rex block ls --json requires one session".
func (r Rex) Blocks(sessionID string) ([]RexBlock, error) {
	var res struct {
		Blocks []RexBlock `json:"blocks"`
	}
	if err := r.Run(false, &res, "block", "ls", "-s", sessionID, "--json", "--short=false"); err != nil {
		return nil, err
	}
	return res.Blocks, nil
}

// FocusBlock is `rex focus -s <session> <block>` (session.focus_block), which
// also makes the block's window the active one.
func (r Rex) FocusBlock(sessionID, blockID string) error {
	return r.Run(false, nil, "focus", "-s", sessionID, blockID)
}

// Send is `rex send -b <block> <text>`: the keystrokes with no Enter behind
// them. The text rides the argv, the way herdr's `pane send-text` does.
func (r Rex) Send(sessionID, blockID, text string) error {
	return r.Run(false, nil, "send", "-s", sessionID, "-b", blockID, text)
}

// SendKey is `rex send-key -b <block> <key…>`, in Rex's own key syntax.
//
// The names are passed through rather than translated. Rex refuses one it
// does not know — MEASURED: `send-key notakey` printed `invalid key
// "notakey"` and exited 2 — so a caller spelling a key herdr's way gets a
// refusal rather than a silent no-op, which is the only property a
// translation table would have been buying. (`esc` and `escape` both land.)
func (r Rex) SendKey(sessionID, blockID string, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	args := append([]string{"send-key", "-s", sessionID, "-b", blockID}, keys...)
	return r.Run(false, nil, args...)
}

// Capture is `rex capture -b <block> --format <format> --trim`: the block's
// screen AND scrollback, which is what rex's capture is (herdr's `pane read`
// is the pane). --trim drops the trailing blank rows, so a tail taken over
// this reads the last lines of OUTPUT and not of padding — the client-side
// tail PaneRead already applies for the same reason.
func (r Rex) Capture(sessionID, blockID, format string) (string, error) {
	if format == "" {
		format = "text"
	}
	return r.RunText(false, "capture", "-s", sessionID, "-b", blockID, "--format", format, "--trim")
}

// ProgramStatus is `rex block call -b <block> program_status`: every OSC 7501
// record this block holds, root first.
//
// `block call` and not `api call block.method`, which is the same request one
// layer down: `api` is the verb the fake-substrate dispatcher already gives
// gh (herdr_test.go's TestMain), and a second substrate answering to it would
// make the three fakes overlap on argv alone.
func (r Rex) ProgramStatus(sessionID, blockID string) ([]RexStatusRecord, error) {
	var res struct {
		Records []RexStatusRecord `json:"records"`
	}
	if err := r.Run(false, &res, "block", "call", "-s", sessionID, "-b", blockID, "program_status"); err != nil {
		return nil, err
	}
	return res.Records, nil
}

// Process is `rex block call -b <block> process`: what is running in the
// block, which is what turns a record into a reading (RexProcess's doc has
// the three measurements).
func (r Rex) Process(sessionID, blockID string) (RexProcess, error) {
	var res RexProcess
	if err := r.Run(false, &res, "block", "call", "-s", sessionID, "-b", blockID, "process"); err != nil {
		return RexProcess{}, err
	}
	return res, nil
}

// Endpoint is the server address this client is talking to — `rex whoami
// --json`'s `endpoint`, e.g. `unix:///…/rex/server.sock`. It is what ADR 0076
// §5 puts in a session record's `socket:`.
//
// "" on any failure, which MuxServer.Addr documents as "this pass cannot
// name its server" and which is never read as a match (cannotAnswerFor). A
// prune that could not name its own server must not act on a record's.
func (r Rex) Endpoint() string {
	var res struct {
		Endpoint string `json:"endpoint"`
	}
	if err := r.Run(false, &res, "whoami", "--json"); err != nil {
		return ""
	}
	return res.Endpoint
}
