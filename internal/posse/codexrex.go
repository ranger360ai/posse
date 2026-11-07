package posse

// THE REX CODEX LINE (ADR 0076 Amendment 2, D6, D7 and D8; bead
// ranger-base-8gh58). On a `multiplexer: rex` instance, a codex launch line
// drops `--disable hooks` and carries posse's own reporter instead: the D6
// declaration, and the trust codex's own `hooks/list` reported for it (D7).
// This file is the one place those three edits are made, and it is a no-op
// everywhere else — on herdr, and on every runtime whose Rex record is its
// own output (`RexRoute == ""`, claude).
//
// WHY A LINE AT ALL. codex writes no OSC 7501 record of its own (MEASURED,
// ADR 0076 D4: no `7501;` string in the binary), so on Rex — where posse
// reads the runtime's record and invents no state, with no screen rules and
// no posse-written hooks to fall back on (§3) — a codex seat would be
// unreadable from its first keystroke. D4 priced the hook route and refused
// it; ADR 0080 D3 accepted the price, and Amendment 2 is how it is paid: no
// `hooks.json` anywhere, no per-seat `CODEX_HOME`, no hash computed by posse.
// Two `-c` values on the line and one `codex app-server` ask per launch.
//
// THE THREE EDITS, in the order they matter:
//
//  1. `--disable hooks` comes OFF. It switches the whole hook layer off, so
//     a line carrying it and the declaration would install a reporter and
//     then make sure nothing ran it — a seat that launches, reports nothing,
//     and reads downstream as a runtime that went silent. D8 keeps the flag
//     for herdr, where the posture it protects (in a fleet seat only posse's
//     hooks run) is bought instead by D7 switching every foreign hook off by
//     key.
//  2. The DECLARATION goes on, naming posse's own binary as the handler.
//  3. The TRUST goes on AFTER it, and the order is load-bearing: codex
//     applies `-c` overrides in order and `hooks={…}` sets the whole `hooks`
//     table, `state` subtable and all, so a `hooks.state` that arrives first
//     is wiped by the declaration that follows it (MEASURED 2026-11-06,
//     ranger-base-fwwl8; the pair is handed back ordered by
//     CodexHooksFlags for exactly this reason).
//
// SHELL-QUOTED, BOTH OF THEM, because this line is TYPED — into a pane's
// shell, or into the launch script a line over PaneLineMax spills to
// (paneline.go). shellQuote's `'\''` idiom is what carries the declaration's
// own single quotes through, and those are not optional: codex splits a hook
// `command` on whitespace with no shell and honours single quotes, so the
// handler path is quoted inside the TOML string (MEASURED, ranger-base-z7kle).
// Single quotes also make `$` and a backtick in either value inert, which a
// double-quoted spelling would not.
//
// WHAT IT COSTS THE LINE. These two values are long — roughly 800 bytes of
// declaration for eight events, and about as much again in trust once codex
// has reported a sha256 per hook. Every persona line already spills to
// state/launch/<session>.sh (measured 2026-09-01: 1867-2058 bytes against the
// 1023 limit), so this spends headroom that is already spent and changes no
// behaviour; NOTES.md keeps that accounting.
//
// WHAT IT COSTS THE LAUNCH. One `codex app-server` child per launch, bounded
// by CodexAskTimeout and killed by its pid (MEASURED 0.41 s for the whole
// three-message conversation). It is spent under the launcher lock, like
// confirmReports' wait, and it is spent AFTER every cheap refusal on this
// path — so a launch that was going to fail on its model, its roots, its PID
// channel or its version pin fails without starting it.

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// rexCodexFlags is the ask, as a seam. A var for skillsDriftVersion's reason
// (parity.go): a hermetic test must not spawn whatever codex happens to be on
// the box's PATH, and the pins for the LINE are about the three edits above,
// not about the RPC — which has its own nine pins over a fake app-server
// (codextrust_test.go) and its own probe.
var rexCodexFlags = CodexHooksAsk.Flags

// RexReportLine returns cmd with this runtime's Rex reporting route typed on
// it, or cmd unchanged when there is no route to type: any instance that is
// not `multiplexer: rex`, and any runtime that writes its own record.
//
// IT IS THE WHOLE OF THE DECISION, so that the two paths which render a
// persona line — planLaunch and RelaunchAgent — each ask it in one line and
// cannot drift into typing different lines for one seat. Both of them reach
// it: a relaunch re-types a full persona command into a live seat, and a
// revived codex that came back without its reporter would be a seat posse can
// no longer read at all (the same reason that path re-renders the gates, the
// skills, the mode file and the trust rather than trusting the meta).
//
// THE RUNTIME DECLARES IT AND NOTHING HERE READS A NAME (ADR 0017 §3). The
// route is `Runtime.RexRoute`, built-in only, and the only non-native value
// is RexRouteCodexHooks — codex's three mechanisms, measured.
//
// dir is the seat's working directory, which is the session's own worktree on
// every dispatched launch, and it is the whole reason the ask cannot be asked
// any earlier than this: codex discovers hooks from the cwd's project files
// and keys project trust by the resolved path, so an answer about another
// directory is an answer about another seat. Symlinks are resolved for the
// same reason every probe in ADR 0076 takes its scratch dir through `pwd -P`.
//
// A FAILURE REFUSES THE LAUNCH and is an ordinary error, which is D7's own
// posture: a managed hook posse can neither vet nor switch off, an ask that
// does not answer, a declaration posse cannot render from its own path, a key
// or hash it cannot put through both shell quoting and a TOML string. Each
// alternative is a line that looks complete and a seat on the startup review
// dialog, which on a `prompt: argv` runtime costs a claim, a worktree and a
// billed turn nobody observes. A refusal costs one launch and no turn.
func (a *App) RexReportLine(rt *Runtime, session, dir, cmd string, warn io.Writer) (string, error) {
	if rt == nil || rt.RexRoute != RexRouteCodexHooks || a.MultiplexerName() != MuxRex {
		return cmd, nil
	}
	cwd := resolveLinks(dir)
	// THE HANDLER IS THIS POSSE'S OWN PATH, and it is handed in rather than
	// read inside the renderer because the right answer is not always this
	// process's (CodexHooksDeclaration's own doc). The one case where it is
	// not is the container tier, where a seat runs the IMAGE's Linux posse at
	// /usr/local/bin/posse — and codex cannot reach that tier today: it
	// declares no `CageCred`, so `cage: container` is refused for it before
	// any of this (cage.go, CheckCageCredential). If codex ever gains one,
	// this call site is where the image's path and the cage's own cwd have to
	// be typed instead, and the trust ask has to be asked from in there: a
	// declaration rendered by a posse at another path reads back
	// `trustStatus: "modified"` (MEASURED, ranger-base-fwwl8), and foreign
	// hooks discovered from the host's cwd are not the cage's.
	//
	// The inventory codex reported goes to a BUFFER and is printed only when
	// the ask fails. codex's `errors` for a directory name the declarations
	// that failed to LOAD, which is the one thing a refusal sentence cannot
	// reconstruct — a hook that did not load is absent from the answer, and
	// absence is what the drift guard refuses over. On the happy path it is
	// nine lines of noise on every launch, so it is kept for the path that
	// needs it.
	var inventory bytes.Buffer
	flags, err := rexCodexFlags(CodexHooksAsk{
		Exe:  RunningPosse().Exe,
		App:  rt.Name,
		Cwd:  cwd,
		Conf: CodexSeatConf(cwd),
		Bin:  rt.Exe(),
		Log:  &inventory,
	})
	if err != nil {
		if inventory.Len() > 0 {
			fmt.Fprintf(warnw(warn), "posse: %s — what codex reported for %s before the refusal:\n%s", session, AbbrevHome(cwd), inventory.String())
		}
		return "", err
	}
	line, err := codexRexHooksOnLine(cmd, flags, rt)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(warnw(warn), "posse: %s — %s reports on Rex through posse's own hooks: the declaration and the trust %s itself reported are on the launch line and `%s` is off it (ADR 0076 D6/D7)\n",
		session, rt.Name, rt.Name, CodexDisableHooks)
	return line, nil
}

// codexRexHooksOnLine is the three edits, as a function of the rendered line
// alone: `--disable hooks` removed, the declaration appended, the trust
// appended after it.
//
// APPENDED AT THE END, not spliced in where the flag was, for two reasons.
// codex parses options anywhere on the line and ends a variadic list at the
// next option token (EnsureUnattended's own measured rule), so position buys
// nothing; and the alternative — rebuilding the line around an index — would
// make the result depend on how a PID's `command:` happens to be spelled.
// The work prompt is appended AFTER this by planLaunch, so these two flags
// still come before the positional prompt.
//
// THE REFUSAL IS THE POINT OF SPLITTING THIS OUT. A PID's own `command:` is
// the one template posse did not write, and a spelling this removal cannot
// see — `--disable hooks,mcp`, `--disable=hooks`, the flag at the very start
// of a line — would leave the hook layer switched off over a declaration
// posse had just typed and trusted. That is a seat that launches, runs no
// hook, writes no record, and HOLDs its bead over a turn nobody can read.
// Asked of the line AFTER the removal, so it is one question — "is this flag
// still here" — and not a list of spellings to keep up with.
func codexRexHooksOnLine(cmd string, f CodexHooksFlags, rt *Runtime) (string, error) {
	line := codexWithoutDisableHooks(cmd)
	if strings.Contains(line, CodexDisableHooks) {
		return "", Die("%s launch refused: the rendered line still names `%s` after posse removed it, so the hooks posse is about to declare and trust would be switched off by the same line — a seat that starts, runs no hook, writes no program_status record, and holds its bead over a turn nobody can read (ADR 0076 D6, D8)\n"+
			"  the line is: %s\n"+
			"  posse removes the flag only where it types it (CodexFleetFlags' own spelling, with a leading space); a `command:` template that spells it another way has to drop it itself for this instance, or run this persona on a herdr instance where the flag belongs",
			rt.Name, CodexDisableHooks, line)
	}
	if f.Declaration == "" || f.State == "" {
		return "", Die("%s launch refused: posse has only half of the hooks line (declaration %q, trust %q) and both are required — the declaration alone draws the startup review dialog, and the trust alone names hooks that are not there (ADR 0076 D6, D7)",
			rt.Name, f.Declaration, f.State)
	}
	// Declaration FIRST, trust after it. MEASURED both ways round
	// (ranger-base-fwwl8): the wrong order is a line carrying every byte of
	// the right answer, a launch that completes, and a seat on the review
	// dialog with no record and no turn.
	return line + " -c " + shellQuote(f.Declaration) + " -c " + shellQuote(f.State), nil
}

// codexWithoutDisableHooks removes `--disable hooks` from a rendered line, and
// only where it is a WHOLE word: ` --disable hooks` followed by a space or by
// the end of the line.
//
// THE WHOLE-WORD RULE IS THE GUARD'S PRECONDITION, not a nicety. A plain
// substring removal takes the same bytes out of `--disable hooks,mcp` and
// leaves `,mcp` behind — a line with codex's hook layer still switched off,
// through a flag the refusal above can no longer SEE, because the string it
// looks for is the string that was just removed. So anything this does not
// recognise is left in place, where that refusal finds it and says so.
func codexWithoutDisableHooks(cmd string) string {
	for {
		i := strings.Index(cmd, " "+CodexDisableHooks)
		if i < 0 {
			return cmd
		}
		end := i + 1 + len(CodexDisableHooks)
		if end < len(cmd) && cmd[end] != ' ' {
			// Not posse's own spelling. Left for the refusal; a second,
			// legitimate occurrence after it is left too, because a line
			// carrying both is not a line to type either way.
			return cmd
		}
		cmd = cmd[:i] + cmd[end:]
	}
}
