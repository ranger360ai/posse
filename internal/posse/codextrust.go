package posse

// D7's ASK: posse asks codex which hooks would load on this seat's line, and
// types their trust onto the same line (ADR 0076 Amendment 2, D7; bead
// ranger-base-fwwl8). codexreport.go is the other half — it renders the
// declaration this file asks about, and is the handler those hooks name.
//
// WHAT THIS IS FOR. D6 puts posse's reporter on codex's line as session-flag
// hooks, so that a codex seat on a Rex instance writes an OSC 7501 record and
// can be admitted at all (ADR 0076 §3). codex will not run a hook it has not
// been told to trust: the startup review fires for any hook that is untrusted
// or modified, enabled or not (STATIC, hooks/src/engine/discovery.rs
// hook_needs_review at rust-v0.153.4), and that dialog holds the prompt. A
// seat behind it submits nothing, reports nothing, and reads downstream as a
// runtime that went silent — D2's argv arm HOLDs and a worktree and a claim
// are spent on a turn nobody saw. So trust is not an optimisation here. It is
// the difference between a seat and a stall.
//
// WHY POSSE ASKS INSTEAD OF COMPUTING. codex's trust identity is a sha256
// over sorted-key compact JSON of the NORMALIZED declaration (hook_hash plus
// version_for_toml, STATIC at the same tag) — undocumented, and a shape that
// moves with the version. Worse, D7 has to switch off every OTHER hook in
// reach as well, by key, and `enabled=false` ALONE still draws the review
// dialog (MEASURED, ADR 0076's `userhook` arm). So posse would need every
// FOREIGN hook's hash too, which means re-implementing codex's whole hook
// discovery and re-auditing it at every codex bump. Asking is one RPC that
// answered in 0.1-0.4 s every time it has been run, and a failed ask refuses
// safely. ADR 0076's "Alternatives rejected" has the longer argument; a
// Python model of the hash that reproduced codex 8 of 8 stays in
// docs/adr/0076-flag-trust.probe.sh as a check on the READING, not as posse's
// route.
//
// WHAT IS ASKED, AND WHAT COMES BACK. `codex app-server` speaks JSON-RPC
// lines on stdio. It is labelled `[experimental]` by codex's own help, which
// is why D9 pins the admission to the version this was measured on. Three
// messages: `initialize` with a clientInfo, the `initialized` notification,
// then `hooks/list` with the seat's cwd.
//
// MEASURED 2026-11-06, this box, codex-cli 0.153.4, a scratch CODEX_HOME with
// a dead loopback provider and no credential, this worktree's own `posse
// hook-report --declare codex` as the declaration:
//
//   - `hooks/list` answered in 0.41 s wall for the whole conversation, with
//     nine rows: posse's eight (`source: "sessionFlags"`, one group and one
//     handler each, `isManaged: false`) and a user-layer `hooks.json`
//     (`source: "user"`). Every row read `trustStatus: "untrusted"`.
//   - The answer's shape is `result.data[]`, one entry per asked cwd, each
//     `{cwd, hooks[], errors[], warnings[]}`. A row carries `key`, `source`,
//     `command`, `currentHash`, `isManaged` and `sourcePath` among others.
//   - **A NOTIFICATION ARRIVES BETWEEN THE TWO ANSWERS**:
//     `remoteControl/status/changed`, id-less, after `initialize`'s reply and
//     before `hooks/list`'s. So a reader that takes the next line as its
//     answer reads the wrong one. codexRPCAwait skips every message that is
//     not the id it asked for, which is also what the reference client in
//     ADR 0076's probe does.
//   - `command` comes back BYTE-IDENTICAL to what posse typed, single quotes
//     and all. That is what makes the drift guard in codexHooksState honest.
//   - Asking a SECOND time with the rendered `hooks.state` on the line reads
//     every row back `trustStatus: "trusted"`, posse's eight still
//     `enabled: true` and the user-layer hook `enabled: false`. codex is
//     therefore its own acceptance test for the rendering, with no Rex block
//     and no prompt: that is the `state` arm of
//     docs/adr/0076-trust-ask.probe.sh.
//
// WHY EVERY FAILURE IS A REFUSAL AND NOT A DEGRADE. D7 names two cases that
// refuse the launch before the line is typed — a managed hook, and an ask
// that fails — and the reason is the same for both: the alternative is a line
// that LOOKS complete and produces a seat sitting on a dialog. A refusal
// costs a launch and no turn; a wrong line costs a claim, a worktree and a
// billed turn. So everything this file cannot vet refuses, including the
// shapes D7 did not enumerate because they were not yet measured: an answer
// that names none of posse's own hooks, a key or a hash posse cannot put on a
// line, and the same key twice.
//
// THE ENVIRONMENT IS TYPED, NOT INHERITED (ADR 0077 D1). The ask must resolve
// the SAME codex home the seat will, or it trusts hooks from one home and the
// seat loads another. posse's own CODEX_HOME is read once, through the one
// read both directions share (runtimeHomeOverride, runtimehome.go), and the
// child gets RuntimeHomePin's two arms as environment rather than as a typed
// prefix: the absolute value when it is in force, and the name UNSET when it
// is not — never the empty value, which codex and posse read differently from
// unset.
//
// ONE CHILD, BOUNDED, AND KILLED BY ITS PID. D7 says once per launch, and the
// launch lock is held for it. The deadline is CodexAskTimeout and the kill is
// the context's — `exec.CommandContext` signals the one process posse
// started, which is also the only form of ending anything this shop allows
// (AGENTS.md, "Ending anything"). cmd.WaitDelay is the second half of it:
// `codex app-server` can spawn MCP servers of its own from a trusted
// project's config, and a grandchild holding the pipes keeps os/exec's Wait
// from returning however dead the child is.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CodexAskTimeout bounds the one child this file starts — the deadline AND
// the WaitDelay, so the whole call is bounded at twice it (grokInspectOutput's
// rule, skillsdirtrust.go).
//
// 20 s against a MEASURED 0.41 s for the whole three-message conversation,
// and 0.1-0.3 s for `hooks/list` alone in ADR 0076's own four runs. The
// headroom is not for the work; it is for a cold first exec of the codex
// binary on a loaded box (ranger-base-y9q7l measured a worst case of 2.17 s
// for a first exec alone) and for an `initialize` that may reach the network
// with a live login, which is ASSUMED harmless and still unmeasured
// (ADR 0076 Amendment 2, Consequences). A codex slower than this is not going
// to hand the seat a record inside the 45 s `startup_wait` either, so the
// bound refuses in the direction the launch would fail anyway.
const CodexAskTimeout = 20 * time.Second

// codexSessionFlagsSource is the `source` codex reports for a hook that came
// from the command line — the session-flags layer, keyed under a synthetic
// path no directory appears in (STATIC, config/src/config.rs at
// rust-v0.153.4; MEASURED as the literal below).
//
// IT IS THE WHOLE OF "posse's own" IN THIS FILE, and it is sound because
// posse types the only session flags there are: a hook in this layer arrived
// on the argv this very call just built. The drift guard in codexHooksState
// checks the `command` as well, so the claim is confirmed by codex's own echo
// rather than resting on that reasoning alone.
const codexSessionFlagsSource = "sessionFlags"

// codexAskClient is the clientInfo name posse introduces itself with. It
// lands in the app-server's `userAgent` (MEASURED: `posse/0.153.4 (Mac OS …)`)
// and nowhere else posse reads, so it is a label for whoever reads codex's
// own logs and not a protocol value.
const codexAskClient = "posse"

// CodexHooksFlags is the PAIR the launch types, and it is one value on
// purpose. The trust codex reports binds to the HASH of the declaration it
// was asked about, so a line that types a declaration other than the one the
// ask asked about is a line whose hooks are untrusted — the dialog, the
// stall, the silent runtime. Handing back both halves together is what makes
// "the line is typed from what was asked" true by construction instead of
// true as long as two call sites keep rendering the same string.
//
// THE TWO ARE ORDERED, AND THE ORDER IS LOAD-BEARING: Declaration must be
// typed BEFORE State. codex applies `-c` overrides in order, and
// `hooks={…}` sets the whole `hooks` table, `state` subtable and all — so a
// `hooks.state` that arrives first is wiped by the declaration that follows
// it. MEASURED 2026-11-06 (this box, codex-cli 0.153.4, the same two values
// both ways round, one scratch home): declaration first, codex read every
// hook back `trusted` with the foreign one `enabled: false`; trust first,
// all nine read `untrusted` and the foreign one `enabled: true`. The wrong
// order is a line carrying every byte of the right answer and a seat sitting
// on the startup review dialog — no record, no turn, and nothing on the
// screen that says why. docs/notes.d/ranger-base-fwwl8.md has the table.
type CodexHooksFlags struct {
	// Declaration is the `hooks={…}` value (D6) — the `-c` that installs
	// posse's reporter. It is typed FIRST.
	Declaration string
	// State is the `hooks.state={…}` value (D7) — the `-c` that trusts
	// posse's hooks and switches every other one off. It is typed AFTER
	// Declaration.
	State string
}

// CodexHooksAsk is one run of D7's ask. Every field is a SEAT fact, because
// the answer is only valid for the line the seat will actually get: codex
// discovers hooks from the cwd's project files and from the home, and keys
// their trust by a hash of the declaration.
type CodexHooksAsk struct {
	// Exe is the posse binary the handler names — the parameter
	// CodexHooksDeclaration refuses to guess (codexreport.go: a container
	// seat runs the image's Linux posse, and the host's path does not exist
	// in there).
	Exe string
	// App is the runtime name, which is both the reporter's argument and the
	// `app=` the record carries.
	App string
	// Cwd is the seat's working directory — the session dir. It is the
	// child's cwd AND the `cwds` hooks/list is asked about, which must be
	// the same directory for the answer to be about this seat.
	Cwd string
	// Conf are the OTHER `-c` values the seat's line types, in its order —
	// `allow_login_shell=false` and the `projects={…trusted}` grant today
	// (CodexFleetFlags), with `$PWD` already resolved to Cwd, because this
	// child has no shell to expand it. The hooks declaration is NOT among
	// them: Flags renders it and appends it, so the ask and the line cannot
	// disagree about it. Neither is `hooks.state` — that is the answer.
	Conf []string
	// Bin is the codex binary. "" asks for `codex` resolved against the
	// gates-free PATH (ADR 0077 D3), which is what a launch wants; a test
	// and a probe name a path.
	Bin string
	// Timeout overrides CodexAskTimeout. 0 is the constant.
	Timeout time.Duration
	// Log, when set, receives what codex reported: one line per hook, plus
	// any load error or warning codex named for the asked directory. It is
	// the reference client's own diagnostic (ADR 0076's ask.py), and the
	// thing to read first when a codex seat stalls — it says which hooks
	// codex would load, from where, and whether posse's own are among them.
	//
	// nil is silent. A launch path is worth pointing at its own log for
	// exactly one reason: codex's `errors` for a directory name the hook
	// declarations that FAILED to load, which is the one thing a refusal
	// sentence here cannot reconstruct — a hook that did not load is absent
	// from the answer, and absence is what the drift guard refuses over.
	Log io.Writer
}

// Flags runs the ask and renders both halves of the line. It is the whole of
// this file's exported surface.
//
// Order of operations matters and is D7's: the declaration is rendered FIRST,
// so a path posse cannot put in a `command` refuses before a child is
// started; then the ask; then the state, so a managed hook or an unusable key
// refuses before anything is typed.
func (a CodexHooksAsk) Flags() (CodexHooksFlags, error) {
	decl, err := CodexHooksDeclaration(a.Exe, a.App)
	if err != nil {
		return CodexHooksFlags{}, err
	}
	// Already vetted by the line above — this is the same renderer, asked
	// for the handler alone, because the drift guard compares codex's echo
	// with it.
	handler, err := codexHookCommand(a.Exe, a.App)
	if err != nil {
		return CodexHooksFlags{}, err
	}
	rows, err := a.ask(decl)
	if err != nil {
		return CodexHooksFlags{}, err
	}
	a.logRows(rows)
	state, err := codexHooksState(rows, handler)
	if err != nil {
		return CodexHooksFlags{}, err
	}
	return CodexHooksFlags{Declaration: decl, State: state}, nil
}

// logRows prints the inventory to a.Log, in the order codex reported it.
func (a CodexHooksAsk) logRows(rows []codexHookRow) {
	if a.Log == nil {
		return
	}
	fmt.Fprintf(a.Log, "codex hooks/list: %d hook(s)\n", len(rows))
	for _, h := range rows {
		fmt.Fprintf(a.Log, "  %-13s %-9s managed=%-5s %s\n", h.Source, h.TrustStatus, strconv.FormatBool(h.IsManaged), h.Key)
	}
}

// ─── the row ─────────────────────────────────────────────────────────────────

// codexHookRow is one hook of codex's `hooks/list` answer, narrowed to what
// D7 reads. The answer carries a dozen more fields per hook (matcher,
// displayOrder, pluginId, async…); an unread field here would be a second
// thing to keep in step with codex's protocol for no decision's sake.
type codexHookRow struct {
	// Key is codex's own identity for this hook, and it is what the trust
	// table is keyed by. posse echoes it and never builds one.
	Key string `json:"key"`
	// Source is the layer the hook came from. codexSessionFlagsSource is
	// posse's own; everything else is somebody else's and gets switched off.
	Source string `json:"source"`
	// Command is the handler codex parsed, echoed back verbatim (MEASURED).
	// The drift guard reads it.
	Command string `json:"command"`
	// CurrentHash is the trust identity posse types. posse never computes
	// one (D7).
	CurrentHash string `json:"currentHash"`
	// IsManaged is the box's own policy saying this hook is not posse's to
	// switch off or vet. One of these refuses the launch.
	IsManaged bool `json:"isManaged"`
	// SourcePath and TrustStatus are read for the sentences only — the
	// refusal that names the file an operator has to look at, and the
	// inventory line that says what codex thought of each hook before posse
	// typed anything.
	SourcePath  string `json:"sourcePath"`
	TrustStatus string `json:"trustStatus"`
}

// ─── the rendering (D7: posse's trusted, everyone else's disabled) ───────────

// codexHooksState renders D7's one `-c` value from codex's own answer:
//
//	hooks.state={"<ours>"={trusted_hash="sha256:…"},"<theirs>"={enabled=false,trusted_hash="sha256:…"}}
//
// handler is the `command` posse declared, for the drift guard below.
//
// THE INLINE-TABLE FORM, and not the dotted one. codex's `-c` parser takes
// `hooks.state={…}` with the hook keys quoted INSIDE the table; the fully
// dotted spelling (`hooks.state."<key>".trusted_hash=…`) is silently ignored,
// as unknown `-c` keys generally are — the same trap CodexFleetFlags' own
// `projects={…}` grant carries, and the reason that one is written the way it
// is (runtime.go). Silently ignored is the worst shape available here: a line
// that parses, a launch that completes, and a dialog nobody is watching.
//
// ONE VALUE AND NOT ONE PER HOOK, for the same reason the probe's `userhook`
// arm had to put both spellings in one table: a second `-c hooks.state=`
// REPLACES the first rather than merging into it.
//
// WHY A DISABLED HOOK IS ALSO TRUSTED. `enabled=false` stops the hook from
// running (STATIC, hook_enabled) and does NOT stop the review dialog
// (MEASURED, ADR 0076's `userhook` arm). Trust is what stops the dialog.
// Neither alone is enough, so a foreign hook gets both: switched off, and
// vouched for to the extent of "posse has seen this exact declaration and it
// is not going to run".
//
// THE REFUSALS, each naming what it protects:
//   - a MANAGED hook (D7). posse can neither switch it off nor vet it, so
//     either it runs code nobody here chose, or the dialog fires. Refuse,
//     and name the file, because the remedy is the operator's.
//   - an answer that does not name posse's OWN hooks, as many as were
//     declared, each with the `command` posse typed. This is not in D7's
//     list because it was not yet measurable: it is the drift guard, and
//     what it catches is an ask asked about a line other than the one the
//     seat will get — Conf carrying its own `hooks=`, a codex whose session
//     flags layer stopped being called `sessionFlags`, a declaration codex
//     parsed into something else. In every one of those the rendered trust
//     is for hooks nobody is about to install, posse's own hooks stay
//     untrusted, and the seat stalls on the dialog. Refusing here costs one
//     launch and reads as what it is.
//   - a key, a hash, or a whole value posse cannot put on a line: a single
//     quote (the value rides inside shell single quotes), a double quote or
//     a backslash (it rides inside a TOML basic string), or a control byte.
//     Each would be read by a layer whose escaping nothing here has
//     measured, and the failure mode is the dialog again. A foreign hook in
//     such a path is somebody's to move; posse says which byte and where.
//   - the same key twice. A duplicate key makes the inline table invalid
//     TOML, so codex would reject the whole value — and posse asks about one
//     cwd, so a repeat is codex contradicting itself rather than two
//     legitimate rows.
func codexHooksState(rows []codexHookRow, handler string) (string, error) {
	seen := make(map[string]bool, len(rows))
	ours, typed := 0, 0
	var b strings.Builder
	b.WriteString("hooks.state={")
	for _, h := range rows {
		if h.IsManaged {
			return "", Die("codex hooks trust: %q is a MANAGED hook (source %q, from %s). posse can neither switch it off nor vet what it runs, so the launch is refused before codex's line is typed — the alternative is a seat that runs a hook nobody here chose, or one sitting on the startup review dialog with no record and no turn. Managed hooks are the box's policy (system, MDM, enterprise); the remedy is the operator's (ADR 0076 Amendment 2, D7)",
				h.Key, h.Source, codexAskWhere(h))
		}
		if err := codexFlagToken(h.Key, "hook key", h); err != nil {
			return "", err
		}
		if err := codexFlagToken(h.CurrentHash, "trust hash", h); err != nil {
			return "", err
		}
		if seen[h.Key] {
			return "", Die("codex hooks trust: codex's hooks/list named the key %q twice. posse asks about ONE cwd, so this is not two rows about two seats — and a key repeated inside one inline table is invalid TOML, which codex would answer by dropping the whole `hooks.state` value and drawing the review dialog over hooks it then considers untrusted (ADR 0076 Amendment 2, D7)", h.Key)
		}
		seen[h.Key] = true

		off := "enabled=false,"
		if h.Source == codexSessionFlagsSource {
			if h.Command != handler {
				return "", Die("codex hooks trust: codex reports a session-flags hook posse did not declare. Its `command` is %q and posse's handler is %q (key %q). The session-flags layer is the command line, so this is a line other than the one this ask was built from — and trust binds to the hash of the declaration asked about, so typing this answer would leave posse's own hooks untrusted and the seat on the startup review dialog. Refused before the line is typed (ADR 0076 Amendment 2, D7)",
					h.Command, handler, h.Key)
			}
			off = ""
			ours++
		}
		if typed > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `"%s"={%strusted_hash="%s"}`, h.Key, off, h.CurrentHash)
		typed++
	}
	b.WriteString("}")

	// CodexHookEvents() and not the package variable, because this is the
	// seam codexreport.go built for this caller: one declared event set,
	// copied, so the count cannot be read off something a caller reordered.
	if want := len(CodexHookEvents()); ours != want {
		return "", Die("codex hooks trust: codex's hooks/list named %d hook(s) from the session-flags layer and posse declared %d (%d hook(s) in all). posse's own hooks are the ones that have to end up trusted, so an answer that does not account for all of them is an answer about a different line — the launch is refused before that line is typed, because typing it would leave posse's reporter untrusted, codex on the startup review dialog, and the seat silent. docs/adr/0076-trust-ask.probe.sh is the re-measurement (ADR 0076 Amendment 2, D7, D9)",
			ours, want, len(rows))
	}
	return b.String(), nil
}

// codexAskWhere is the file a refusal points an operator at: codex's own
// sourcePath, or the key when there is none to name.
func codexAskWhere(h codexHookRow) string {
	if h.SourcePath != "" {
		return h.SourcePath
	}
	return h.Key
}

// codexFlagToken is what a string taken from codex's answer must survive to
// reach codex again: shell single quotes on the launch line, and a TOML basic
// string inside the value. It is codexHookCommand's rule (codexreport.go) for
// the other direction, and the same posture — refuse before the line is
// typed, say which byte and where, rather than guess at an escaping nobody
// has measured through both layers.
func codexFlagToken(s, what string, h codexHookRow) error {
	if s == "" {
		return Die("codex hooks trust: codex reported a hook with an empty %s (key %q, source %q, from %s). posse echoes codex's own identity and hash and builds neither, so there is nothing to type for this hook and no way to trust it; the launch is refused rather than typed with the row dropped, which would leave that hook untrusted and codex on the review dialog",
			what, h.Key, h.Source, codexAskWhere(h))
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\'', c == '"', c == '\\':
			return Die("codex hooks trust: the %s of hook %q holds %s at offset %d (source %q, from %s). The value posse types rides inside shell single quotes on the launch line AND inside a TOML basic string within it, and neither layer's escaping for this byte has been measured — so posse refuses the line rather than guess, which costs one launch instead of a seat stalled on the startup review dialog. A hook in a path like this is the operator's to move (ADR 0076 Amendment 2, D7)",
				what, h.Key, strconv.QuoteRune(rune(c)), i, h.Source, codexAskWhere(h))
		case c < ' ', c == 0x7f:
			return Die("codex hooks trust: the %s of hook %q holds a control byte (%s) at offset %d (source %q, from %s). It would end the TOML string, the shell word, or the line itself before it reached codex's parser",
				what, h.Key, strconv.QuoteRune(rune(c)), i, h.Source, codexAskWhere(h))
		}
	}
	return nil
}

// ─── the child ───────────────────────────────────────────────────────────────

// ask runs `codex app-server` once and hands back the hooks it reports.
//
// decl is the declaration Flags rendered; it goes on the ask's own `-c` line
// LAST, after Conf, because the hooks codex reports have to be the hooks the
// seat will get.
func (a CodexHooksAsk) ask(decl string) ([]codexHookRow, error) {
	if !strings.HasPrefix(a.Cwd, "/") {
		return nil, Die("codex hooks trust: the seat's cwd must be an absolute path and %q is not. It is the child's working directory AND the directory hooks/list is asked about, and codex keys project trust and project hook discovery by the resolved path — a relative one would be resolved against posse's own cwd, which is not the seat's", a.Cwd)
	}
	name := a.Bin
	if name == "" {
		name = "codex"
	}
	bin, err := protocolBin(name)
	if err != nil {
		return nil, Die("codex hooks trust: %v. posse cannot ask codex which hooks would load, so it does not type a trust line it has not been given (ADR 0076 Amendment 2, D7)", err)
	}

	// THE DECLARATION LEADS, then Conf. codex applies `-c` overrides in
	// ORDER, and `hooks={…}` sets the whole `hooks` table — including the
	// `state` subtable under it. So a `hooks.state` that arrives BEFORE the
	// declaration is wiped by it. MEASURED 2026-11-06 (this box, codex-cli
	// 0.153.4, one scratch home, the same two values both ways round):
	// declaration first, every hook read back `trustStatus: "trusted"` with
	// the foreign one `enabled: false`; trust first, all nine read
	// `untrusted` and the foreign one `enabled: true` — a seat on the
	// startup review dialog, from a line holding every byte of the right
	// answer in the wrong order. This order is the SEAT's order too
	// (CodexHooksFlags), and the ask uses it so that a round trip through
	// `posse hook-report --ask … -c <the state it printed>` asks codex the
	// same question a launch does.
	argv := []string{"app-server", "-c", decl}
	for _, c := range a.Conf {
		argv = append(argv, "-c", c)
	}

	limit := a.Timeout
	if limit <= 0 {
		limit = CodexAskTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()

	cmd := protocolCmd(exec.CommandContext(ctx, bin, argv...))
	cmd.Dir = a.Cwd
	// WaitDelay as well as the context, herdrWaitDelay's rule: `codex
	// app-server` can spawn MCP servers from a trusted project's config
	// (CodexProjectConfig, runtime.go), and a grandchild holding the child's
	// pipes keeps os/exec's Wait from returning however dead the child is.
	cmd.WaitDelay = limit
	cmd.Env = codexAskEnv(cmd.Env)

	// STDOUT IS AN *os.File POSSE OWNS, not cmd.StdoutPipe(). os/exec's own
	// pipe is closed by Wait, which is documented as incorrect to call while
	// a read is still in flight — and here a read always is, because the
	// reader is a goroutine draining a stream whose end posse decides. An
	// *os.File is handed to the child directly: os/exec starts no copy
	// goroutine for it and closes nothing of posse's, so the ONE close that
	// unblocks the reader is the deferred one below, after the child is
	// reaped. A grandchild that inherited the write end cannot wedge that
	// either, for the same reason: nothing posse waits on is reading it.
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, Die("codex hooks trust: cannot open a pipe to read codex's answer: %v", err)
	}
	defer pr.Close()
	cmd.Stdout = pw
	in, err := cmd.StdinPipe()
	if err != nil {
		pw.Close()
		return nil, Die("codex hooks trust: cannot open a pipe to speak to codex: %v", err)
	}
	// STDERR IS A FILE POSSE OWNS TOO, for a reason that is about the
	// SUCCESS path and not about tidiness. Handing os/exec an io.Writer
	// makes it start a copy goroutine, and Wait does not return while that
	// goroutine is running — so a GRANDCHILD holding the pipe makes every
	// launch pay cmd.WaitDelay, and `codex app-server` spawns MCP servers of
	// its own from a trusted project's config. This child is killed on the
	// success path BY DESIGN, so that grace would be paid per launch rather
	// than only on a hang. With an *os.File there is no goroutine: Wait
	// returns when the process is reaped, and the reader below is posse's to
	// stop. The text is wanted for one thing — when the ask fails, what
	// codex said is usually the whole diagnosis, and a refusal naming only a
	// timeout sends the next reader to the wrong question.
	er, ew, err := os.Pipe()
	if err != nil {
		pw.Close()
		return nil, Die("codex hooks trust: cannot open a pipe to read codex's diagnostics: %v", err)
	}
	defer er.Close()
	cmd.Stderr = ew

	started := time.Now()
	if err := cmd.Start(); err != nil {
		pw.Close()
		ew.Close()
		return nil, Die("codex hooks trust: cannot start `%s app-server`: %v", bin, err)
	}
	// Both write ends are the child's now, so EOF means EOF.
	pw.Close()
	ew.Close()
	said := make(chan string, 1)
	go func() {
		// The error is discarded on purpose: ReadAll hands back what it
		// accumulated either way, and "posse closed the pipe" is the
		// ordinary end of this read, not a failure to report.
		b, _ := io.ReadAll(er)
		said <- string(b)
	}()

	rows, askErr := codexAskOver(ctx, a.Cwd, in, codexRPCLines(ctx, pr), a.Log)
	in.Close()
	cancel()
	waitErr := cmd.Wait()
	if askErr == nil {
		return rows, nil
	}
	// Only now is the stderr text wanted, and only now is it safe to stop
	// waiting for it: the child is reaped, so whatever still holds that pipe
	// is a grandchild posse is not waiting on.
	er.Close()
	return nil, Die("codex hooks trust: the ask failed after %s (bound %s), so posse does not type a trust line: %v%s%s. `codex app-server` is experimental and the whole route rests on it (ADR 0076 Amendment 2, D7, D9); docs/adr/0076-trust-ask.probe.sh is the re-measurement",
		time.Since(started).Round(time.Millisecond), limit, askErr,
		codexAskExit(waitErr), codexAskSaid(<-said))
}

// codexAskExit is the child's own verdict, for the refusal sentence. An exit
// STATUS is a reading — codex looked at the line and said no — and a SIGNAL is
// not: on this path the signal is almost always posse's own kill, arriving
// after the ask had already failed, and `exited -1` would read as something
// codex did. Go spells a signalled child's ExitCode as -1, which is the whole
// of the branch.
func codexAskExit(waitErr error) string {
	var ee *exec.ExitError
	if errors.As(waitErr, &ee) && ee.ExitCode() >= 0 {
		return " (codex exited " + strconv.Itoa(ee.ExitCode()) + ")"
	}
	if waitErr == nil {
		return " (codex exited 0)"
	}
	return ""
}

// codexAskSaid is the stderr tail, trimmed. codex's stderr here is diagnostic
// text, never a credential: the ask sends no prompt and reads no provider.
func codexAskSaid(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 400 {
		s = s[:400] + "…"
	}
	return "; codex said: " + strings.ReplaceAll(s, "\n", " | ")
}

// codexAskEnv is the child's environment: ADR 0077 D3's gates-free PATH
// (protocolEnv — posse's own children run outside every gates dir), with
// RuntimeHomePin's two arms for CODEX_HOME applied as environment rather than
// as a typed prefix.
//
// Both arms, and the second is the one that needs saying: when posse's own
// CODEX_HOME is NOT in force, the name is UNSET for the child rather than set
// empty. An empty value is a third state posse and the runtimes read
// differently from unset (runtimeHomeOverride's doc), and the whole point of
// the pin is that the ask and the seat resolve the same home — so the ask
// unsets it exactly where the line types `/usr/bin/env -u CODEX_HOME`.
func codexAskEnv(env []string) []string {
	env = protocolEnv(env)
	if v, ok := runtimeHomeOverride(CodexHomeVar); ok {
		return envPut(env, CodexHomeVar, v)
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if name, _, ok := strings.Cut(kv, "="); ok && name == CodexHomeVar {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// ─── the conversation ────────────────────────────────────────────────────────

// codexAskOver speaks D7's three messages and decodes the answer.
//
// NO `jsonrpc` MEMBER, which is what the reference client sends and what was
// MEASURED accepted (ADR 0076's ask.py, and this bead's own runs). Adding one
// would be a protocol detail nobody here has measured codex's tolerance for.
func codexAskOver(ctx context.Context, cwd string, in io.Writer, msgs <-chan codexRPCMsg, log io.Writer) ([]codexHookRow, error) {
	if err := codexRPCSend(in, map[string]any{
		"id":     1,
		"method": "initialize",
		"params": map[string]any{"clientInfo": map[string]any{"name": codexAskClient, "version": VersionString()}},
	}); err != nil {
		return nil, fmt.Errorf("sending initialize: %w", err)
	}
	if _, err := codexRPCAwait(ctx, msgs, 1); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	if err := codexRPCSend(in, map[string]any{"method": "initialized"}); err != nil {
		return nil, fmt.Errorf("sending the initialized notification: %w", err)
	}
	if err := codexRPCSend(in, map[string]any{
		"id":     2,
		"method": "hooks/list",
		"params": map[string]any{"cwds": []string{cwd}},
	}); err != nil {
		return nil, fmt.Errorf("sending hooks/list: %w", err)
	}
	result, err := codexRPCAwait(ctx, msgs, 2)
	if err != nil {
		return nil, fmt.Errorf("hooks/list: %w", err)
	}

	var answer struct {
		Data []struct {
			Cwd   string         `json:"cwd"`
			Hooks []codexHookRow `json:"hooks"`
			// codex's own reading of the files it found. A declaration that
			// failed to load is NOT in Hooks, so these two are the only
			// account of it there will be (see CodexHooksAsk.Log).
			Errors   []json.RawMessage `json:"errors"`
			Warnings []json.RawMessage `json:"warnings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result, &answer); err != nil {
		return nil, Die("codex's hooks/list answer is not the shape posse reads (result.data[].hooks[]): %v", err)
	}
	if len(answer.Data) == 0 {
		return nil, Die("codex's hooks/list answered about no directory at all, having been asked about %q", cwd)
	}
	var rows []codexHookRow
	for _, inv := range answer.Data {
		// THE ANSWER HAS TO BE ABOUT THE DIRECTORY POSSE ASKED ABOUT
		// (ranger-base-5x8xi, found verifying ranger-base-fwwl8 under
		// ranger-base-rucsu). `data[]` carries one entry per asked cwd and
		// posse asks about exactly one, so an entry naming another directory
		// — or naming none — is an inventory of somebody else's seat. What it
		// costs is what every refusal in this file exists to prevent: the
		// hooks the seat's OWN project declares are absent from such an
		// answer, so they stay untrusted, codex draws the startup review
		// dialog, and the seat submits nothing and reports nothing. The
		// duplicate-key refusal below already says the reasoning in words
		// ("posse asks about ONE cwd") and until this it was the one place
		// the reasoning was written and the cwd was not read.
		//
		// A BYTE COMPARE, and that is MEASURED and not assumed. codex-cli
		// 0.153.4 on darwin 25.6.0, 2026-11-07: `hooks/list` echoes the asked
		// path back verbatim with no normalization of any kind — a trailing
		// slash survives, an uncollapsed `..` survives, a symlinked path is
		// NOT resolved, and the echo is identical whether the answer carries
		// no hooks, a discovered hook, or a hooks.json codex could not parse.
		// So equality cannot refuse a launch that was fine, and anything
		// weaker would admit a neighbouring path. posse hands over an
		// absolute, symlink-resolved cwd (runCodexHooksAsk, RexReportLine);
		// docs/notes.d/ranger-base-5x8xi.md has the six readings.
		if inv.Cwd != cwd {
			return nil, Die("codex hooks trust: posse asked hooks/list about %q and codex answered about %q. posse asks about ONE cwd, so an answer about another directory is an inventory of a different seat — the hooks this seat's own project declares are not in it, so they would stay untrusted, codex would draw the startup review dialog, and the seat would submit nothing (ADR 0076 Amendment 2, D7)", cwd, inv.Cwd)
		}
		rows = append(rows, inv.Hooks...)
		codexAskNotes(log, inv.Cwd, "error", inv.Errors)
		codexAskNotes(log, inv.Cwd, "warning", inv.Warnings)
	}
	return rows, nil
}

// codexAskNotes prints codex's own reading of the hook files it found for one
// directory. Nothing refuses over these: a foreign hooks.json that will not
// parse declares no hook, so it neither runs nor draws the review dialog, and
// a launch that failed over the tidiness of somebody else's config file would
// be a refusal posse cannot justify. They are printed because they are the
// diagnosis for the one refusal that looks like nothing at all — posse's own
// declaration missing from the answer.
func codexAskNotes(log io.Writer, cwd, kind string, notes []json.RawMessage) {
	if log == nil {
		return
	}
	for _, n := range notes {
		fmt.Fprintf(log, "codex hooks/list %s for %s: %s\n", kind, cwd, codexAskSnippet(n))
	}
}

// codexRPCSend writes one message as a line. The app-server reads
// line-delimited JSON on stdin.
func codexRPCSend(w io.Writer, msg map[string]any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// codexRPCMsg is one thing read off the child's stdout: a message, or the
// reason there will not be another. The ERROR TRAVELS, rather than ending the
// stream silently, because the two ways a stream can stop want different
// sentences — codex exiting without answering is a different diagnosis from
// codex writing bytes that are not JSON, and a reader that swallows the error
// has to report the first for both.
type codexRPCMsg struct {
	raw json.RawMessage
	err error
}

// codexRPCLines decodes the child's stdout as a stream of JSON values and
// hands them on.
//
// A json.Decoder AND NOT A bufio.Scanner, for one reason: a Scanner needs a
// maximum line length, and that number is a silent no-answer the day an
// inventory grows past it — a box with enough plugin hooks, in paths long
// enough, and the launch refuses over a cap posse chose. The decoder reads
// whatever the value is. It also happens to be the right reader for a
// protocol that is only incidentally line-oriented.
//
// The goroutine ends either way: on a decode error (which is also what the
// pipe being closed looks like), or on the context, which covers a send
// nobody is draining.
func codexRPCLines(ctx context.Context, r io.Reader) <-chan codexRPCMsg {
	msgs := make(chan codexRPCMsg, 8)
	go func() {
		defer close(msgs)
		dec := json.NewDecoder(r)
		for {
			var m codexRPCMsg
			m.err = dec.Decode(&m.raw)
			select {
			case msgs <- m:
			case <-ctx.Done():
				return
			}
			if m.err != nil {
				return
			}
		}
	}()
	return msgs
}

// codexRPCAwait reads until the answer to id arrives.
//
// IT SKIPS WHAT IS NOT ITS ANSWER, and that is not defensiveness: MEASURED
// 2026-11-06, `remoteControl/status/changed` arrives id-less BETWEEN
// initialize's reply and hooks/list's, so a reader that takes the next line
// reads a notification and calls the answer malformed. A message carrying
// another id is skipped for the same reason.
//
// What it does NOT skip is a message it cannot read as an envelope at all.
// That is the "answers malformed" refusal D7 names, and refusing on it beats
// skipping: skipping would turn a codex talking nonsense into a 20 s timeout
// whose sentence names the wrong cause.
func codexRPCAwait(ctx context.Context, msgs <-chan codexRPCMsg, id int) (json.RawMessage, error) {
	for {
		select {
		case m, ok := <-msgs:
			if !ok {
				return nil, Die("codex's stdout ended before it answered")
			}
			if m.err != nil {
				if errors.Is(m.err, io.EOF) {
					return nil, Die("codex's stdout ended before it answered")
				}
				return nil, Die("codex wrote bytes posse cannot read as JSON: %v", m.err)
			}
			raw := m.raw
			var env struct {
				ID     *int            `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return nil, Die("codex wrote something posse cannot read as a JSON-RPC message (%v): %s", err, codexAskSnippet(raw))
			}
			if env.ID == nil || *env.ID != id {
				continue // a notification, or an answer to something else
			}
			if env.Error != nil {
				return nil, Die("codex refused: %s (code %d)", env.Error.Message, env.Error.Code)
			}
			if len(env.Result) == 0 {
				return nil, Die("codex answered with neither a result nor an error: %s", codexAskSnippet(raw))
			}
			return env.Result, nil
		case <-ctx.Done():
			return nil, Die("codex did not answer inside the bound")
		}
	}
}

// codexAskSnippet is a message, trimmed for a sentence.
func codexAskSnippet(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
