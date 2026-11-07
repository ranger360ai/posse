package posse

// posse's OSC 7501 reporter, and the codex hooks declaration that installs it
// (ADR 0076 Amendment 2, D6; bead ranger-base-z7kle).
//
// WHAT THIS IS FOR. On Rex the multiplexer slot reads the runtime's OWN
// program-status record and invents nothing (ADR 0076 §3, multiplexer.go).
// codex 0.153.4 carries no `7501;` string, so it reports nothing, and D4
// refused it on Rex for the price of a hook. ADR 0080 D3 accepts that price
// and D6 chooses the route: codex's line carries `-c hooks={…}`, one group
// with one handler per event, and the handler is THIS BINARY. It reads the
// hook JSON codex writes on stdin, takes `hook_event_name`, and writes ONE
// OSC 7501 report to /dev/tty.
//
// WHY /dev/tty AND NOT STDOUT. codex captures a hook's stdout and stderr —
// they are the hook's answer to codex, and PreToolUse can use them to block a
// tool. The report is not for codex, it is for the terminal Rex is watching,
// so it goes to the controlling tty. MEASURED (ADR 0076 §Context, and the
// spike): a child with its stdout redirected that writes to /dev/tty lands a
// record; a `setsid` child gets ENXIO and lands nothing. The record Rex keeps
// is stamped with the tty's FOREGROUND program, which is why a record written
// by posse's reporter reads `owner={pid,"codex"}` and is positive evidence
// under ADR 0013 §1 (D9): posse did not write a record about codex, codex's
// own event did.
//
// WHY EVERY HOOK PATH BELOW EXITS 0. codex reads exit 2 from a hook as "block
// this", and a UserPromptSubmit hook that exits 2 holds the prompt (that is
// the lock ADR 0076's probes use to make a run unbillable). A reporter is a
// bystander: it must never be able to stop a turn. So nothing on the hook
// path returns anything but 0 — not an unreadable stdin, not malformed JSON,
// not an event the table does not name, and not a /dev/tty that will not
// open. A failure to report is read on the other side as silence, and ADR
// 0013 §1 already says what silence is worth: D2's argv arm HOLDs rather than
// settling a bead on a turn nobody saw.
//
// WHY THE ENTRY POINT IS AHEAD OF NewApp (cmd/posse/main.go). NewApp refuses
// a process whose environment names no home, by design and with exit 1
// (ranger-base-58b5, TestBinaryWithNoHomeRefusesRatherThanRootAtTheCWD). A
// hook runs in whatever environment the runtime hands it, and a reporter that
// can exit 1 on a scrubbed environment is a reporter that prints a refusal
// into codex's hook output. It reads no config, like the two cage entry
// points above it, so it has nothing to get from NewApp anyway.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// HookReportVerb is posse's fourth entry point: `posse hook-report <app>`,
// the handler D6 puts on codex's line. It is deliberately absent from the
// usage catalog — nobody types it, the launch renders it — which is also why
// it is not a case in main's verb switch (a verb there without a catalog
// entry reds cmd/posse/verbhelp_qa_test.go, and rightly: an operator-facing
// verb owes an operator a usage).
const HookReportVerb = "hook-report"

// HookReportDeclareFlag makes the same verb print the declaration it would be
// installed by, instead of reporting: `posse hook-report --declare codex`.
//
// It exists so that the probe and anyone debugging a stalled codex seat read
// the string posse's own renderer produces, rather than a second copy of it
// written in shell. A renderer whose only caller is the launch is a renderer
// nobody can check before the launch.
const HookReportDeclareFlag = "--declare"

// HookReportAskFlag makes the same verb run D7's trust ask and print the ONE
// `-c` value it builds: `posse hook-report --ask codex [-c <value>]…`
// (codextrust.go).
//
// It is here rather than on a verb of its own for the reason --declare is: a
// launch renders both halves of codex's hooks line itself, and these two arms
// are how the halves are made observable BEFORE a launch — the string that
// would be typed, and the trust codex itself would answer with. The probe
// reads posse's own renderer through them (docs/adr/0076-trust-ask.probe.sh)
// rather than carrying a second copy of either in shell, so there is no
// drift between what the probe measures and what a launch types.
//
// The `-c` values after the app name are the OTHER values the seat's line
// carries, and the ask passes them to codex ahead of the declaration. The
// hooks declaration itself is not among them and must not be: the ask renders
// it (CodexHooksAsk.Flags), because trust binds to the hash of the
// declaration asked about.
const HookReportAskFlag = "--ask"

// codexHookTimeout is the `timeout` posse declares on every handler, in
// seconds, and it is the probe's measured value. The reporter opens one file
// and writes ~40 bytes; the budget is for a box under load, not for the work.
const codexHookTimeout = 3

// codexHookEvents is the events posse declares, in the order the probe
// declared them (docs/adr/0076-flag-trust.probe.sh, EVS) — the order that was
// MEASURED accepted by codex 0.153.4's `-c` parser. Nothing depends on the
// order (TOML tables are unordered and codex keys each hook by its own
// event), but a renderer whose output moves between runs is a renderer whose
// declaration cannot be compared with the one a seat was launched with.
var codexHookEvents = []string{
	"SessionStart",
	"UserPromptSubmit",
	"PreToolUse",
	"PermissionRequest",
	"PostToolUse",
	"Stop",
	"Interrupt",
	"SessionEnd",
}

// CodexHookEvents is codexHookEvents, copied — the declaration's event set is
// also what ADR 0076 D7's trust ask has to account for (bead
// ranger-base-fwwl8: posse's own hooks get `{trusted_hash}`), and that caller
// must not be able to reorder this one.
func CodexHookEvents() []string { return append([]string(nil), codexHookEvents...) }

// CodexHookReport is D6's table: the OSC 7501 payload for one codex hook
// event, or "" for an event the table does not name.
//
//	SessionStart, Stop, Interrupt        idle
//	UserPromptSubmit, PreTool, PostTool  working
//	PermissionRequest                    blocked, kind=permission
//	SessionEnd                           state=clear
//
// "" IS THE WHOLE ANSWER FOR AN UNKNOWN EVENT. codex may grow an event
// tomorrow, and a reporter that mapped the unknown onto a state would be
// posse inventing a reading — the thing ADR 0076 §3 exists to stop. It is
// also why there is no `default: working` here, which is what the probe's
// throwaway hook.sh did (its `*)` arm): a probe hook wants a witness line for
// everything it sees, and a shipped reporter wants to write nothing it cannot
// justify.
//
// `state=clear` is not one of the protocol's five states (`idle | working |
// done | blocked | error`), and it is not a sixth: it REMOVES the record.
// MEASURED 2026-11-06, this box, Rex CLI 0.0.1793377904, by hand into a `rex
// new` block — `state=clear` takes `program_status` from one `working` record
// to `{"records":[]}`, and does the same to a `done` record; `state=` empty is
// dropped and leaves the record it found. STATIC, same binary: the OSC 7501
// keyword set holds `idle done error working blocked`, `state`, `title`, and
// no `clear`, and the `7501;?` feature reply advertises those five states
// only. So `clear` is undocumented in the spec revision ADR 0076 read, and it
// is what SessionEnd wants: the record goes away with the session rather than
// sitting at a stale `idle` for whatever reads the block next.
//
// app is passed through, not mapped — the record says what the program called
// itself (rex.go, RexStatusRecord.App). It must be a plain token, because an
// app string carrying `:` or an escape byte would not be a name, it would be
// extra pairs in someone else's report; one that is not gets "" and no write.
func CodexHookReport(event, app string) string {
	if !plainStatusToken(app) {
		return ""
	}
	pairs := ""
	switch event {
	case "SessionStart", "Stop", "Interrupt":
		pairs = "state=idle"
	case "UserPromptSubmit", "PreToolUse", "PostToolUse":
		pairs = "state=working"
	case "PermissionRequest":
		// kind is set on `blocked` only (rex.go, RexStatusRecord.Kind). Fleet
		// codex runs `-a never`, so this should never fire; D6 maps it anyway
		// and it is still UNPROVEN live (D4 item 5).
		pairs = "state=blocked:kind=permission"
	case "SessionEnd":
		pairs = "state=clear"
	default:
		return ""
	}
	return pairs + ":app=" + app
}

// OSC7501 wraps one payload in the escape sequence Rex reads:
// `ESC ]7501;<payload> ESC \`, string-terminated.
//
// ST and not BEL: `ESC \` is what the spec's own examples use and what every
// ADR 0076 probe wrote (`printf '\033]7501;…\033\\'`), so it is the form
// every measurement on this box was taken with.
func OSC7501(payload string) string { return "\x1b]7501;" + payload + "\x1b\\" }

// plainStatusToken is the character set an app name may use: a name, nothing
// else. `:` separates pairs, `=` separates a pair, ESC ends the sequence, and
// a byte outside this set is either one of those or something Rex's own
// payload validator may drop the whole report over ("payload is not escape
// code safe UTF-8", its words).
//
// A LEADING DASH IS REFUSED, and that is an argv rule rather than a payload
// one (ranger-base-pjui5, found verifying ranger-base-5x8xi under
// ranger-base-r7fyk). `-` is otherwise a legal byte here — `grok-build` is a
// runtime name — but the app name is not only the `app=` a report carries: it
// is also the last word of the command CodexHooksDeclaration renders, which
// codex splits into an argv and runs. runHookReport reads a leading dash in
// that position as "this is not a hook argv" and gets out of codex's way at
// exit 0, before it knows anything about app names at all — so an app called
// `-c`, `-`, `--declare` or `--ask` USED TO render a declaration whose
// handler this very file then refused: a reporter that opens no tty and
// writes nothing, on every event, forever, while codex trusts it and Rex
// reads no record for the seat. MEASURED 2026-11-07: the declaration rendered
// and the command it installed wrote 0 bytes for all four.
//
// REFUSED HERE AND NOT IN THE DECLARATION because this is the one rule both
// sides already read, and a second copy beside the first is two rules to keep
// in step by hand. runHookReport's own dash guard stays where it is: it is
// reached before any name rule and its job is the exit code codex sees, not
// what a name may be.
func plainStatusToken(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return true
}

// ─── the declaration (D6: one group, one handler per event) ──────────────────

// CodexHooksDeclaration renders the `-c` VALUE that installs the reporter:
//
//	hooks={SessionStart=[{hooks=[{type="command",command="'<exe>' hook-report codex",timeout=3}]}],…}
//
// exe is posse's own binary — on a host seat RunningPosse().Exe, an absolute
// path with symlinks resolved, which is also why no gate shim can answer for
// it (ADR 0077 D3's posture; the same reason cagelauncher.go re-execs by the
// path os.Executable named). app is the runtime name, and it is both the
// reporter's argument and the `app=` the record carries.
//
// IT IS A PARAMETER AND NOT A READ OF os.Executable, because the right answer
// is not always this process's path. A container seat (ADR 0002 §3 L4) runs
// the IMAGE's Linux posse, /usr/local/bin/posse (etc/cage/Dockerfile), and the
// host's absolute path does not exist in there — a declaration rendered from
// it would give a handler that cannot exec, which from outside looks like one
// line inside the turn and a runtime that went silent. Whoever types the line
// decides which posse it names; this function refuses to guess.
//
// It is one group of one handler per event rather than one group listing
// every event, because that is the shape the probe measured codex accept and
// the shape codex keys its trust by: `/<session-flags>/config.toml:<event>:<group>:<handler>`
// (D6, D7). A second handler in a group would move every key after it.
//
// IT RETURNS AN ERROR RATHER THAN A BEST-EFFORT STRING. `command` is a single
// TOML string that codex splits into an argv itself, and posse does not own
// that split — so a path which cannot survive it refuses here, before the
// line is typed, which is D7's own posture for the two cases it names. A
// refusal costs a launch; a declaration that mis-splits costs a seat that
// starts, reports nothing, and is read as a runtime that went silent.
func CodexHooksDeclaration(exe, app string) (string, error) {
	cmd, err := codexHookCommand(exe, app)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("hooks={")
	for i, ev := range codexHookEvents {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `%s=[{hooks=[{type="command",command=%q,timeout=%d}]}]`, ev, cmd, codexHookTimeout)
	}
	b.WriteString("}")
	return b.String(), nil
}

// codexHookCommand is the handler's `command`: posse's own path, the hidden
// verb, and the app name.
//
// HOW CODEX READS THIS STRING, MEASURED 2026-11-06 (this box, codex-cli
// 0.153.4, docs/adr/0076-reporter.probe.sh arm `space`, both spellings of a
// handler under a directory called `a dir`):
//
//   - UNQUOTED, the split is on whitespace and there is no shell: codex tried
//     to exec the first word and the turn printed `hook exited with code
//     127`. The hook never ran, and nothing but that one line said so.
//   - SINGLE-QUOTED, the quotes are honoured: the hook RAN, and its `$0` was
//     the whole path, space included.
//
// So the path is quoted, always — one spelling, the one that was measured,
// rather than a branch on whether today's install happens to hold a space.
// `exec.Command` is not available to us here: the string is config codex
// parses later, not an argv posse hands to a process.
//
// THE REFUSALS, each naming what it protects:
//   - an empty exe. posse could not name its own binary, so there is no
//     handler to declare at all.
//   - a relative exe. The hook runs with codex's cwd, which is the seat's,
//     and a relative handler would resolve against whatever that is.
//   - a single quote, a double quote or a backslash in exe. Each is read by
//     BOTH layers — codex's splitter and, for the quotes, the TOML string
//     around it — and neither layer's escaping is measured. The close-escape-
//     reopen idiom that would carry a single quote through is a POSIX-shell
//     rule, and what codex splits with is not a POSIX shell. UNMEASURED is
//     not "probably fine"
//     here: it is a seat that starts, runs no hook, and reads downstream as a
//     runtime that went silent. D7's posture for anything posse cannot vet is
//     to refuse before the line is typed, where it costs a launch and no
//     turn. The `space` arm above is the re-measurement that would lift any
//     of these.
//   - a control byte in exe. It would reach the terminal through codex's own
//     error reporting before it reached anything else.
func codexHookCommand(exe, app string) (string, error) {
	if !plainStatusToken(app) {
		return "", Die("codex hooks declaration: %q is not a runtime name posse can put in an OSC 7501 report, nor one it can pass as the handler's own argument (letters, digits, '-', '_', '.', and never a leading '-' — a name that looks like a flag installs a reporter this binary refuses to run)", app)
	}
	if exe == "" {
		return "", Die("codex hooks declaration: posse cannot name its own binary, so there is no handler to declare (os.Executable failed — see RunningPosse)")
	}
	if !strings.HasPrefix(exe, "/") {
		return "", Die("codex hooks declaration: the handler must be an absolute path and %q is not — a hook runs with the seat's cwd, and a relative handler resolves against whatever that is", exe)
	}
	for i := 0; i < len(exe); i++ {
		switch c := exe[i]; {
		case c == '\'', c == '"', c == '\\':
			return "", Die("codex hooks declaration: posse's own path %q holds %s at offset %d. codex's hook `command` is one string that codex splits itself (MEASURED: it honours single quotes and runs no shell), and both that splitter and the TOML string around it would read this byte — posse refuses the line rather than guess at the escaping, so the launch fails before a turn is spent. docs/adr/0076-reporter.probe.sh arm `space` is the re-measurement that would lift this",
				exe, strconv.QuoteRune(rune(c)), i)
		case c < ' ', c == 0x7f:
			return "", Die("codex hooks declaration: posse's own path %q holds a control byte (%s) at offset %d, which reaches the terminal through codex's own error reporting before it reaches anything else", exe, strconv.QuoteRune(rune(c)), i)
		}
	}
	// The verb and the app need no quoting and get none: both are plain
	// tokens by construction (HookReportVerb is a constant, app is held to
	// plainStatusToken above), and a quote around a word that cannot need one
	// is a second spelling of the same command for nobody's benefit.
	return "'" + exe + "' " + HookReportVerb + " " + app, nil
}

// ─── the entry point ─────────────────────────────────────────────────────────

// IsHookReport reports whether this argv is the reporter's. It reads the
// verb, and it is checked ahead of NewApp for the reason at the head of this
// file.
func IsHookReport(argv []string) bool {
	return len(argv) > 1 && argv[1] == HookReportVerb
}

// RunHookReport is the reporter. It returns an exit code rather than an
// error, because the ONLY code it may return on the hook path is 0 and a
// caller holding an error has to decide that again.
//
//	posse hook-report <app>                     read the hook JSON on stdin, write one report
//	posse hook-report --declare <app>           print the declaration that installs the above
//	posse hook-report --ask <app> [-c <v>]…     ask codex for the trust that declaration needs
//
// Only the declare and ask arms can fail, and they can because nothing they
// do is inside a turn: they are things posse or an operator asks before a
// launch.
func RunHookReport(argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runHookReport(argv, stdin, stdout, stderr, openTTY)
}

// openTTY is the controlling terminal, as the write target. A hook's stdout
// is codex's, so this is the only handle that reaches the terminal Rex reads.
func openTTY() (io.WriteCloser, error) { return os.OpenFile("/dev/tty", os.O_WRONLY, 0) }

// runHookReport is RunHookReport with the tty handed in, so a test can pin
// the BYTES rather than the fact that something was written: there is no
// controlling terminal in `go test`, and a reporter whose only arm is "it did
// not crash" pins nothing (the same reason runningPosse takes its two
// lookups).
func runHookReport(argv []string, stdin io.Reader, stdout, stderr io.Writer, tty func() (io.WriteCloser, error)) (code int) {
	// A GO PANIC EXITS WITH STATUS 2, which is the one status this must never
	// return: codex reads 2 from a hook as "block this", and from
	// UserPromptSubmit that holds the prompt. So the last way the reporter
	// could still stop a turn is by crashing, and nothing above would have
	// decided to. The named return is what makes the recovery an answer and
	// not just a swallowed trace — a bare `recover()` in a deferred closure
	// leaves the function's result wherever it was. Recovered HERE and not in
	// the exported wrapper so the guard is reachable from a test: an
	// unreachable guard against a coincidence of exit codes is a comment, not
	// a control.
	defer func() {
		if recover() != nil {
			code = 0
		}
	}()

	var args []string
	if len(argv) > 2 {
		args = argv[2:]
	}

	// THE ASK ARM OWNS ITS OWN ARITY, at any length (ranger-base-5x8xi, found
	// verifying ranger-base-fwwl8 under ranger-base-rucsu). It used to be
	// gated on `len(args) >= 2`, so a bare `--ask` with no app name fell
	// past it into the wrong-argv arm below, whose `return 0` is the HOOK
	// path's rule — and exit 0 over an empty stdout is the one answer this
	// arm must never give, because a launch script reads it as a green light
	// over an empty trust line (runCodexHooksAsk's own doc). A short argv
	// reaches that arm's usage instead, which exits 1 like every other
	// malformed ask.
	if len(args) >= 1 && args[0] == HookReportAskFlag {
		return runCodexHooksAsk(args[1:], stdout, stderr)
	}
	// AND SO DOES THE DECLARE ARM, at any length (ranger-base-pjui5, found
	// verifying ranger-base-5x8xi under ranger-base-r7fyk). It was routed on
	// `len(args) == 2`, so an argv one word short (`--declare` alone) and one
	// word long (`--declare codex extra`) both fell past it into the shared
	// wrong-argv arm below and were answered with that arm's `return 0` — the
	// HOOK path's rule, over an empty stdout, which is the same wrong answer
	// the ask arm above stopped giving and for the same reason. The reader is
	// shipped and is not a hand run: docs/adr/0076-trust-ask.probe.sh reads
	// this arm at lines 122 and 171 as
	//
	//	DECL=$("$POSSE" hook-report --declare codex) || exit 1
	//
	// so an exit 0 leaves DECL empty, the `|| exit 1` unfired, and the script
	// carries on and installs nothing.
	//
	// EXIT 1 AND NOT 0 is what this arm already answers when
	// CodexHooksDeclaration refuses the app name, which is also what says the
	// finding was the arity and not the validation — an app name the token
	// rule refuses DOES reach here and DOES exit 1. And it blocks nothing: the
	// status codex reads from a hook as "block this prompt" is 2, never 1
	// (this file's header), and a declaration never installs this argv anyway.
	if len(args) >= 1 && args[0] == HookReportDeclareFlag {
		if len(args) != 2 {
			fmt.Fprintf(stderr, "posse: usage: posse %s %s <app>  — one runtime name and nothing else (posse renders this line itself; a launch script reads the declaration off stdout, so an empty stdout at exit 0 would be a green light over nothing)\n",
				HookReportVerb, HookReportDeclareFlag)
			return 1
		}
		exe := RunningPosse().Exe
		decl, err := CodexHooksDeclaration(exe, args[1])
		if err != nil {
			fmt.Fprintln(stderr, "posse: "+err.Error())
			return 1
		}
		fmt.Fprintln(stdout, decl)
		return 0
	}
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		// Not exit 2, and not a usage an operator is meant to read: posse
		// renders this command itself, so a wrong argv here is either a hand
		// run or a declaration that drifted from this binary. Say which, on
		// stderr, and still get out of codex's way.
		fmt.Fprintf(stderr, "posse: usage: posse %s <app>  |  posse %s %s <app>  |  posse %s %s <app> [-c <value>]…  (posse's OSC 7501 hook reporter, rendered onto a runtime's launch line — ADR 0076 D6/D7 — not typed by hand)\n",
			HookReportVerb, HookReportVerb, HookReportDeclareFlag, HookReportVerb, HookReportAskFlag)
		return 0
	}
	app := args[0]

	// The payload is codex's own, and it is read whole. NO SIZE CAP, on
	// purpose: a cap is a number that turns a legitimate report into a silent
	// no-write the day a hook payload grows past it, and nothing has measured
	// what codex's largest is. A decoder that fails for any reason — a short
	// read, a truncated object, something that is not JSON — falls through to
	// the same exit 0 as everything else here.
	var hook struct {
		Event string `json:"hook_event_name"`
	}
	if err := json.NewDecoder(stdin).Decode(&hook); err != nil {
		return 0
	}
	payload := CodexHookReport(hook.Event, app)
	if payload == "" {
		// An event D6's table does not name, or an app name that is not one.
		// Nothing is written and nothing is said: a hook that chatted at
		// codex about every event it did not recognize would be noise inside
		// every turn.
		return 0
	}
	f, err := tty()
	if err != nil {
		// A silent exit 0, per D6. There is no controlling terminal — a
		// `setsid` child, a pane that has gone, a cage without one — so there
		// is nothing to report TO, and saying so on codex's stderr would put
		// posse's plumbing inside the operator's turn.
		return 0
	}
	io.WriteString(f, OSC7501(payload))
	f.Close()
	return 0
}

// runCodexHooksAsk is the --ask arm: D7's trust ask, over this process's cwd
// and this process's CODEX_HOME, for a declaration naming THIS binary.
//
// It prints the `hooks.state` value on stdout and the inventory codex
// reported on stderr, which is the reference client's own split (ADR 0076's
// ask.py): the one is a thing to put on a line, the other a thing to read.
//
// EXIT 1 ON A REFUSAL, unlike every other path in this file. The reporter's
// arms must never return anything but 0 because codex reads a hook's exit
// code (see this file's header); this arm is not a hook — nobody's turn is
// waiting on it — and a refusal that exited 0 would be a launch script's
// green light over an empty trust line.
func runCodexHooksAsk(args []string, stdout, stderr io.Writer) int {
	usage := func() int {
		fmt.Fprintf(stderr, "posse: usage: posse %s %s <app> [-c <value>]…  (ADR 0076 D7's trust ask: the -c values are the OTHER values the seat's line carries, with $PWD already resolved; the hooks declaration is rendered here and must not be among them)\n",
			HookReportVerb, HookReportAskFlag)
		return 1
	}
	// The app name, and NOT `args[0]` straight off: this arm is entered on the
	// flag alone now (ranger-base-5x8xi), and an index into an empty slice
	// would panic — which RunHookReport's recovery turns into exit 0, the
	// very answer the arity change exists to stop.
	if len(args) == 0 {
		return usage()
	}
	app, rest := args[0], args[1:]
	var conf []string
	for len(rest) > 0 {
		if rest[0] != "-c" || len(rest) < 2 {
			return usage()
		}
		conf, rest = append(conf, rest[1]), rest[2:]
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "posse: cannot read this process's working directory, which is the seat's cwd the ask is about: %v\n", err)
		return 1
	}
	// Symlinks resolved, because codex keys project trust and project hook
	// discovery by the RESOLVED path (ADR 0076's probes all take their
	// scratch dir through `pwd -P` for that reason).
	flags, err := CodexHooksAsk{Exe: RunningPosse().Exe, App: app, Cwd: resolveLinks(cwd), Conf: conf, Log: stderr}.Flags()
	if err != nil {
		fmt.Fprintln(stderr, "posse: "+err.Error())
		return 1
	}
	fmt.Fprintln(stdout, flags.State)
	return 0
}
