package posse

// A seat's pane must not outlive its runtime as an interactive shell
// (ranger-base-1l1en, the implementation of ranger-base-ie4mx's decision).
//
// WHAT A SURVIVING SHELL COSTS. posse types a launch into a pane and the
// pane's shell runs it as a child. When the runtime ends — `/exit`, a crash,
// SIGTERM, the kill path — that shell comes back to its prompt and keeps
// reading the tty. Anything that types into the seat after that moment is
// typing into a shell that is outside the gate PATH (GatePrefix rides the
// typed LINE, not the shell) and outside every wall, and the text posse
// types into a seat is prompt text carrying bead titles and comments, which
// are inputs. Two readers reach a seat in that state: a prompt racing the
// watcher's own exit (ranger-base-ie4mx item 1) and, on Rex, any prompt at
// all after a normal runtime exit, because a runtime that exits clears its
// program_status record, so the owner reads unknown and Prompt types
// (refuseAtDeadOwner). Dispatch relaunches before it prompts; `posse
// prompt` and the cockpit do not.
//
// WHY exec AND NOT A TRAILING `; exit`. MEASURED 2026-11-14 on this box
// (ranger-base-ie4mx: GNU bash 5.3.20, an interactive bash on a pty — python
// pty.fork, `bash --norc -i` — the line typed as a seat's is):
//
//	sh -c 'sleep 0.3; exit 3'; exit        shell gone (a later typed command never ran)
//	sh -c 'sleep 0.3; kill -INT $$'; exit  SHELL SURVIVED and ran the next typed line
//	exec sh -c 'sleep 0.3; kill -INT $$'   shell gone
//
// An interactive bash abandons the rest of a command list when its
// foreground job dies of SIGINT, so a trailing `exit` is skipped exactly
// when the runtime is killed by a key or a signal — and the watcher's drain
// runs with ISIG on, so a 0x03 in typed text reaching it during the drain is
// one such SIGINT. exec closes both. The live pin is
// TestLiveLaunchExecLeavesNoShell (launchexeclive_test.go), which reds the
// trailing-exit form on purpose.
//
// WHERE THE exec GOES, and why it is not simply a prefix on the line. Every
// launch line posse renders is a simple command behind a run of
// NAME=VALUE assignments (GatePrefix's PATH/SHELL/GROK_SHELL, the
// runtime-home pin, the transcript ceiling, CLAUDE_CONFIG_DIR). MEASURED
// 2026-11-15, bash 5.3.20 and sh on this box:
//
//	FOO=bar exec /usr/bin/env      env prints FOO=bar; the pid is the shell's
//	exec FOO=bar /usr/bin/env      exec: FOO=bar: not found
//
// So the word `exec` has to land ON the command word, after the assignment
// prefix — which is also where it reads correctly to an operator looking at
// the pane's scrollback. The assignments are still exported to the runtime
// (first row above: the shell is replaced AND carries them), which is the
// one property a misplaced exec would have cost silently: a seat launched
// with no PATH shim and no CLAUDE_CONFIG_DIR.
//
// THE COMPOUND FALLBACK. exec takes a command, not a list, so a rendered
// line that is NOT one simple command cannot be exec'd in place. Nothing
// posse renders today is — launchexec_qa_test.go pins that over every
// built-in runtime and tier — but a renderer that grows a `cd x && cmd`
// must not silently go back to a surviving shell, so such a line is exec'd
// as `exec /bin/sh -c '<line>'` instead. That costs detection fidelity (the
// pane's foreground process is then `sh` and not the runtime, which is the
// property paneline.go's `.` sourcing was chosen to keep), so it is the
// fallback and not the rule, and PaneLine always spills it to the launch
// script rather than spend the typed line's budget on the quoting.

import "strings"

// ExecLaunchLine returns cmd as a line that REPLACES the shell that runs it:
// `exec` inserted at cmd's command word, or — when cmd is not one simple
// command — an `exec /bin/sh -c` of the whole of it. The second return is
// true in that second case, which is what PaneLine reads to decide the line
// must be spilled rather than typed.
//
// An empty cmd, and a cmd that is nothing but assignments, come back
// unchanged and wrapped=false: there is no command word to exec, and posse's
// own callers never type one (`if p.Cmd != ""`). Returning the input is what
// keeps this function total — a launch line is not the place to invent an
// error for a string nobody passes.
func ExecLaunchLine(cmd string) (line string, wrapped bool) {
	if strings.TrimSpace(cmd) == "" {
		return cmd, false
	}
	if off, simple := shCommandWord(cmd); simple {
		return cmd[:off] + "exec " + cmd[off:], false
	}
	// /bin/sh and not `sh`: the line's own PATH assignment is inside the
	// quoted string now, so the lookup would be the PANE shell's PATH — a
	// gates bin dir leading it would aim this at a shim, and this arm exists
	// for a line nobody has read.
	return "exec /bin/sh -c " + shellQuote(cmd), true
}

// LaunchLineIsExecd reports whether line replaces the shell that runs it:
// `exec` at its command word, in either of ExecLaunchLine's two forms. It
// is the reader the render pins ask, so that "what posse types" and "what a
// pin accepts" are one rule and not two (launchexec_qa_test.go,
// internal/treepins).
//
// It is deliberately a question about the LINE and not about ExecLaunchLine:
// a pin that re-ran the renderer would pass over a call site that never
// reached it, which is the whole class ranger-base-ie4mx item 3 is about.
func LaunchLineIsExecd(line string) bool {
	off, simple := shCommandWord(line)
	if !simple || !strings.HasPrefix(line[off:], "exec ") {
		return false
	}
	// And the word exec is ABOUT must be a program, not an assignment:
	// `exec FOO=bar cmd` is "exec: FOO=bar: not found" (MEASURED, the head
	// of this file), so a line of that shape carries the word and launches
	// nothing. Answering yes to it would make this predicate agree with
	// every renderer that got the placement wrong.
	rest := strings.TrimLeft(line[off+len("exec "):], " \t")
	word, _, _ := strings.Cut(rest, " ")
	return word != "" && !shAssignment(word)
}

// shCommandWord returns the offset of cmd's command word — the first word
// that is not a NAME=VALUE assignment — and whether cmd is one simple
// command: no unquoted control operator anywhere in it.
//
// It is SOUND and not complete, on purpose. Every answer of `true` is a line
// this reader took apart whole; everything it cannot take apart whole — an
// unterminated quote, a backquote, a construct it does not model — answers
// `false`, and false costs a caller the `/bin/sh -c` fallback and never a
// wrong insertion point. A line that is only assignments answers false too:
// there is no command word in it to exec.
func shCommandWord(cmd string) (off int, simple bool) {
	cmdWord := -1
	for i, n := 0, len(cmd); i < n; {
		if c := cmd[i]; c == ' ' || c == '\t' {
			i++
			continue
		}
		start := i
		for i < n {
			c := cmd[i]
			if c == ' ' || c == '\t' {
				break
			}
			switch {
			case shControl(c):
				return 0, false
			case c == '\'' || c == '"' || c == '`':
				j := shSkipQuoted(cmd, i)
				if j < 0 {
					return 0, false
				}
				i = j
			case c == '\\':
				i += 2
			case c == '$' && i+1 < n && cmd[i+1] == '(':
				j := shSkipParens(cmd, i+1)
				if j < 0 {
					return 0, false
				}
				i = j
			default:
				i++
			}
		}
		if i > n {
			return 0, false // a trailing backslash: the line is not whole
		}
		if cmdWord < 0 && !shAssignment(cmd[start:i]) {
			cmdWord = start
		}
	}
	if cmdWord < 0 {
		return 0, false
	}
	return cmdWord, true
}

// shControl is the characters that make a line more than one simple command.
// `<` and `>` are NOT here: a redirection is part of a simple command, and
// the bwrap launch line carries one in the middle of itself (`3<'…args'`,
// BwrapPrefix) — exec passes redirections to the process it becomes.
func shControl(c byte) bool {
	switch c {
	case ';', '&', '|', '(', ')', '\n', '\r':
		return true
	}
	return false
}

// shAssignment reports whether w is a NAME=VALUE word — the prefix every
// posse launch line carries and the one thing `exec` must be typed AFTER.
func shAssignment(w string) bool {
	eq := strings.IndexByte(w, '=')
	if eq <= 0 {
		return false
	}
	for i := 0; i < eq; i++ {
		c := w[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// shSkipQuoted advances past the quoting construct at cmd[i], or returns -1
// for one this reader will not model: an unterminated quote, or a backquote
// (nothing posse renders uses one, and reading it would be a second
// substitution grammar for no caller).
func shSkipQuoted(cmd string, i int) int {
	switch cmd[i] {
	case '\'':
		j := strings.IndexByte(cmd[i+1:], '\'')
		if j < 0 {
			return -1
		}
		return i + j + 2
	case '"':
		for i++; i < len(cmd); {
			switch c := cmd[i]; {
			case c == '\\':
				i += 2
			case c == '"':
				return i + 1
			case c == '`':
				return -1
			case c == '$' && i+1 < len(cmd) && cmd[i+1] == '(':
				j := shSkipParens(cmd, i+1)
				if j < 0 {
					return -1
				}
				i = j
			default:
				i++
			}
		}
		return -1
	}
	return -1
}

// shSkipParens advances past the `( … )` at cmd[i], honouring nesting and
// the quotes inside it. This is the arm every claude launch line needs:
// `--append-system-prompt "$(cat '<agents>/<persona>.md')"`.
func shSkipParens(cmd string, i int) int {
	depth := 0
	for i < len(cmd) {
		switch c := cmd[i]; {
		case c == '(':
			depth++
			i++
		case c == ')':
			depth--
			i++
			if depth == 0 {
				return i
			}
		case c == '\'' || c == '"' || c == '`':
			j := shSkipQuoted(cmd, i)
			if j < 0 {
				return -1
			}
			i = j
		case c == '\\':
			i += 2
		default:
			i++
		}
	}
	return -1
}
