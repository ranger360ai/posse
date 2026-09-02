package posse

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ranger-base-lxkdi: scripts/cleanroom.sh's HOOK_DEPS is the list the
// clean-room probe walks — "commands the generated hooks call", and a MISSING
// on any distro is a finding rather than a setup step. It was hand-enumerated
// by READING internal/posse/gates.go for ranger-base-rmgz, and by 2026-09-01 it
// had drifted in both directions at once: `cut` and `sed` were called and never
// probed (the sed one quotes the paths in the revert-recovery paragraph rmgz
// was filed to restore — without it the paragraph prints "finish it:  git
// commit -F - -- " with no paths after it), while six names the hooks never
// call sat in the list looking probed.
//
// The fix is this file: derive the list from the RENDERED hook text instead of
// from a reading of the Go source, and fail here when the two disagree. Reading
// gates.go is what drifted; the rendered bytes are what runs on the box.
//
// SCOPE — the three files posse writes into a hooks dir:
//   - prepare-commit-msg (CommitGuardHook, via the real installer)
//   - pre-push           (PrePushHook)
//   - the chain dispatcher (chainRender), written by installHook when another
//     tool already owns the slot — a generated hook like the other two, and the
//     only reason `dirname` is a dependency at all.
//
// The gate SHIMS and the gate shell (renderShim, gateShellScript) are rendered
// shell too, but they are not hooks and are not what the probe's line claims to
// cover; they are rendered per session on a box that already ran posse. If that
// ever needs probing it is a second list, not this one.

// shellCommandWords returns the words appearing in a command position in POSIX
// shell text — the name the shell would look up to run that command — in the
// order first seen, and a second list of BLIND SITES: places in shell code
// where a command runs that this scanner cannot name.
//
// It is a scanner, not a parser: it tracks quoting, comments, command
// substitution and `case` pattern lists, which is exactly what a substring grep
// cannot do. Grep is not an option here for a measured reason: "tr" occurs 68
// times in the rendered prepare-commit-msg and three of those are the command
// (the rest are "tree", "instance", "restate"), and "rm" occurs 30 times and
// none of them are the command ("form", "performed").
//
// COMPLETENESS — this paragraph is the contract, and ranger-base-h6k2r is what
// it costs to get it wrong. It used to name two blind spots when there were
// ten: `env -i awk`, `! awk`, `xargs awk`, `nice awk`, `timeout 5 awk`,
// `exec awk`, `command awk`, `>out awk`, `eval "awk"` and `VAR=v awk` all
// returned no `awk`, the last of them past an arm whose own comment said it
// kept the command position open. Nothing was missing from HOOK_DEPS at the
// time, so the file was green while telling the next reader it was complete.
//
// The absolute that used to close that paragraph — "exactly one of three
// buckets, and there is no fourth" — was itself wrong, and ranger-base-xwepd is
// what it cost the second time. There WAS a fourth bucket, SILENT, and eight
// shapes sat in it: `cat f |\<newline> awk` and the same line continuation
// after `&&`, after `then` and after `;`; `\awk`, the alias-bypass spelling;
// `trap 'awk 1' EXIT`; `${x:-$(awk 1)}`; and `$(( $(awk 1) + 1 ))`. None was a
// live miss over the three rendered hooks, exactly as with h6k2r — the
// paragraph was the defect, not the census — and the sharpest of them, a
// wrapped pipeline, is more ordinary than any of the ten h6k2r was filed over.
// All eight are TAUGHT or REPORTED now, each with a row below and each row
// mutation-checked.
//
// So the three buckets are a claim about what this file MEASURES, and the
// residue is enumerated at the end instead of asserted away. Every shape named
// in a bucket has a row in
// TestShellCommandWordsSeesEveryCommandPrefixOrReportsIt, every name in the two
// report tables is swept for actually reaching a report, and every marker the
// scanner can report is swept for having its own sentence:
//
// TAUGHT — grammar alone, no knowledge of any command's own options:
//   - the command prefixes that ARE shell syntax: `VAR=v cmd`, `! cmd`,
//     `time cmd`, a redirection written before the command (`>out cmd`,
//     `2>&1 cmd`), and the `exec` and `command` builtins.
//   - an option word: a word starting with `-` is never a command name, so it
//     is skipped without closing the command position (`time -p cmd`,
//     `command -v cmd`). An option that takes a VALUE is the limit of that —
//     `exec -a nm awk` derives "nm" and not "awk" — but it derives a name no
//     HOOK_DEPS holds, so it fails the census out loud instead of going quiet.
//   - the ordinary grammar: pipelines, `&&`, `;`, newlines, subshells, brace
//     groups, `if`/`while`/`until`/`for`/`case`, and `$( )`.
//
// COMPENSATED by a second reader:
//   - a command invoked through a VARIABLE. Both hooks do that once, in
//     posse_stamp, and shellExecProbeNames below reads that idiom's literal
//     name off the `[ -x "$dir/name" ]` test that finds it.
//
// REPORTED as a blind site, which the caller fails on:
//   - `cmd` in BACKTICKS, the other command substitution. Rather than guess at
//     it the scanner reports every backtick it meets while reading SHELL CODE.
//     That is a report from the same lexer state as the words, which is the
//     whole point (ranger-base-cx2ok): a backtick inside a comment or inside
//     single quotes runs nothing, and the rendered commit guard quotes three
//     commands for a reader that way. A whole-body strings.Contains(body, "`")
//     cannot tell the two apart and failed on the prose for a day.
//   - a wrapper whose command argument sits behind its OWN option grammar:
//     runsAnotherCommand below. `env -u NAME awk` and `nice -n 5 awk` cannot be
//     walked past without knowing that -u and -n each take an argument, and
//     holding a table of other commands' options is the one thing this scanner
//     will not do — that table is the hand-maintained list this whole file
//     exists to replace.
//   - find's inline command: `-exec`, `-ok` and their -dir forms.
//   - a HEREDOC operator. Its body is data, not code, so reading it as code
//     both invents command words and can desync the lexer for everything after
//     it — silence, not noise, which is the failure mode that matters here.
//   - a STRING RUNNER: `eval`, and `trap` which is eval with the run deferred
//     to a signal. Its argument is shell text that may not exist until runtime.
//   - a command substitution written INSIDE a region this scan steps over
//     unread — `${x:-$(awk 1)}` and `$(( $(awk 1) + 1 ))`. Not reading those
//     regions is load-bearing (`$((i + 1))` otherwise opens a subshell and `i`
//     is scanned as a command; `${rule#Bash(}` is a pattern, not syntax), so
//     the skip looks for a `$(` or a backtick inside and reports rather than
//     stepping over it in silence. See skipOver.
//   - `. file` and `source file`: the commands are in another file, which this
//     scan does not open. The hooks source nothing today; the entry costs
//     nothing and turns a prose residual into a live guard.
//
// STILL SILENT, named rather than denied, because the last two versions of this
// paragraph both proved that an absolute here is a claim the next reader stops
// checking. None is a live miss over today's three rendered hooks; each is a
// shape that would go quiet if a hook grew it:
//   - an ALIAS, or a shell FUNCTION defined outside the text being scanned. A
//     name resolved either way is not a PATH lookup, so the census erring
//     toward a name HOOK_DEPS does not need is the safe direction here — and
//     `posse_*`, the hooks' own functions, are filtered by hookCensus by name.
//   - a command run through a VARIABLE other than the posse_stamp idiom
//     shellExecProbeNames reads: `$CMD f`, or a name assembled at runtime.
//   - a name spelled so that no literal of it survives: `aw\<newline>k 1`
//     joins to `awk`, and this scan reads `aw` — which is loud (a name no
//     HOOK_DEPS holds), not silent, but it is not `awk` either.
//   - non-POSIX quoting a hook has no reason to grow, `$'...'` first among them.
//
// A report is not a bug in the hook. It is this file saying the census stopped
// being derivable there, which is the finding ranger-base-lxkdi asked for.
func shellCommandWords(src string) (words, blind []shellCall) {
	var out []shellCall
	seen := map[string]bool{}
	report := func(name string, off int) {
		blind = append(blind, shellCall{Name: name, Line: 1 + strings.Count(src[:off], "\n")})
	}
	// skipOver steps past a region the scan does not READ — a `$(( ))` or a
	// `${ }` — and reports it when a command substitution is written inside.
	// Not reading them is deliberate and load-bearing (`$((i + 1))` otherwise
	// opens a subshell and `i` is scanned as a command; `${rule#Bash(}` is a
	// pattern, not syntax), but a `$( )` or a backtick in either place runs a
	// command the scan never enters. `${x:-$(awk 1)}` and `$(( $(awk 1) + 1 ))`
	// were both silent (ranger-base-xwepd). end is the index after the region,
	// body the first index of its insides, marker the name the reader gets.
	skipOver := func(end, body int, marker string, off int) int {
		if body > end {
			body = end
		}
		if r := src[body:end]; strings.Contains(r, "$(") || strings.Contains(r, "`") {
			report(marker, off)
		}
		return end
	}
	emit := func(w string, off int) {
		if !seen[w] {
			seen[w] = true
			out = append(out, shellCall{Name: w, Line: 1 + strings.Count(src[:off], "\n")})
		}
	}
	isWord := func(c byte) bool {
		return c == '_' || c == '.' || c == '/' || c == '-' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
	}
	// Words that never resolve through PATH: shell keywords, and the builtins
	// every POSIX shell answers itself. A builtin cannot make the probe say
	// MISSING, so naming one in HOOK_DEPS is an assertion that can never fail.
	noBinary := map[string]bool{
		"if": true, "then": true, "else": true, "elif": true, "fi": true,
		"for": true, "while": true, "until": true, "do": true, "done": true,
		"case": true, "esac": true, "in": true, "select": true, "function": true,
		".": true, ":": true, "[": true, "[[": true, "alias": true, "bg": true,
		"break": true, "cd": true, "command": true, "continue": true, "echo": true,
		"eval": true, "exec": true, "exit": true, "export": true, "false": true,
		"fc": true, "fg": true, "getopts": true, "hash": true, "jobs": true,
		"kill": true, "local": true, "newgrp": true, "printf": true, "pwd": true,
		"read": true, "readonly": true, "return": true, "set": true, "shift": true,
		"source": true, "test": true, "times": true, "trap": true, "true": true,
		"type": true, "ulimit": true, "umask": true, "unalias": true,
		"unset": true, "wait": true,
	}

	// case-statement state, stacked because a case can nest inside one.
	const caseSubject, casePattern, caseBody = 1, 2, 3
	var caseState []int
	topCase := func() int {
		if len(caseState) == 0 {
			return 0
		}
		return caseState[len(caseState)-1]
	}
	setCase := func(v int) {
		if len(caseState) > 0 {
			caseState[len(caseState)-1] = v
		}
	}

	var substStack []substFrame // scanner state saved across `$( )`
	inDouble, inSingle := false, false
	cmdPos := true
	// assignVal is "the scanner is inside the VALUE of a `VAR=` prefix". Every
	// arm that would otherwise close the command position consults it, because
	// the whole point of the prefix is that the command comes after the value:
	// `AWKPATH=/x awk` and `PATH=/a:/b sh -c ...`. Before ranger-base-h6k2r the
	// word arm set out to do this and the very next byte — the `=` itself —
	// fell to the default arm and undid it.
	assignVal := false
	// endWord closes the command position unless we are mid-assignment-value.
	// Every arm that ends a word goes through it, the `=` of the prefix itself
	// included — that character has no arm of its own because it does not need
	// one, and a mutation run proved the one it used to have was dead.
	endWord := func() {
		if !assignVal {
			cmdPos = false
		}
	}

	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
			i++
		case inDouble:
			switch {
			case c == '\\' && i+1 < len(src):
				i += 2
			case c == '"':
				inDouble = false
				endWord()
				i++
			case c == '$' && i+2 < len(src) && src[i+1] == '(' && src[i+2] == '(':
				i = skipOver(skipArith(src, i), i+3, "$((", i)
			case c == '$' && i+1 < len(src) && src[i+1] == '(':
				substStack = append(substStack, substFrame{inDouble: true, assignVal: assignVal})
				inDouble, assignVal, cmdPos = false, false, true
				i += 2
			case c == '$' && i+1 < len(src) && src[i+1] == '{':
				i = skipOver(skipBraceExpansion(src, i), i+2, "${", i)
			case c == '`':
				// Substitution inside double quotes too, and here the scanner
				// does not even open a command position for it.
				report("`", i)
				i++
			default:
				i++
			}
		case c == '\\' && i+1 < len(src) && src[i+1] == '\n':
			// A backslash-newline is a LINE JOINER, not a quoted character:
			// POSIX says both bytes are removed and what follows continues the
			// same line. So it must leave cmdPos and assignVal exactly as it
			// found them — `cat f |\<newline>  awk 1` is `cat f | awk 1`, and
			// closing the command position here lost the `awk` in silence
			// (ranger-base-xwepd, four of its eight silent lines; `\awk` below is the
			// fifth).
			i += 2
		case c == '\\' && i+1 < len(src) && cmdPos && isWord(src[i+1]):
			// `\awk` is the command word `awk` with one character quoted — the
			// idiom that bypasses an alias or a function of the same name. The
			// name the shell then looks up through PATH is `awk`, so step over
			// the backslash ALONE and let the word arm read it. Closing the
			// command position here made the alias-bypass spelling of every
			// dependency invisible.
			i++
		case c == '\\' && i+1 < len(src):
			i += 2
			endWord()
		case c == '\'':
			inSingle = true
			endWord()
			i++
		case c == '"':
			inDouble = true
			i++
		case c == '#' && (i == 0 || src[i-1] == ' ' || src[i-1] == '\t' || src[i-1] == '\n' ||
			src[i-1] == ';' || src[i-1] == '|' || src[i-1] == '&' || src[i-1] == '('):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '$' && i+2 < len(src) && src[i+1] == '(' && src[i+2] == '(':
			i = skipOver(skipArith(src, i), i+3, "$((", i)
			endWord()
		case c == '$' && i+1 < len(src) && src[i+1] == '(':
			substStack = append(substStack, substFrame{assignVal: assignVal})
			assignVal, cmdPos = false, true
			i += 2
		case c == '$' && i+1 < len(src) && src[i+1] == '{':
			i = skipOver(skipBraceExpansion(src, i), i+2, "${", i)
			endWord()
		case c == '>' || c == '<':
			// A redirection and its target, wherever it appears in the simple
			// command. Written BEFORE the command name (`>out awk`, `2>&1 awk`)
			// it must not close the command position, and its target must not
			// be read as the command: both were misses before h6k2r, the second
			// one loudly — `2>/dev/null awk` emitted `2`.
			var heredoc bool
			i, heredoc = skipRedirect(src, i)
			if heredoc {
				report("<<", i)
			}
		case c == '!' && cmdPos:
			// `! cmd` — a reserved word, and the command follows it.
			i++
		case c == ')':
			// A `)` closing a command substitution we opened is unambiguous, so
			// it is popped BEFORE the case-pattern reading: `$(posse_stamp)`
			// occurs inside case bodies, and reading its `)` as the end of a
			// pattern list left the rest of the double-quoted message being
			// scanned as commands.
			if n := len(substStack); n > 0 {
				f := substStack[n-1]
				substStack = substStack[:n-1]
				inDouble, assignVal = f.inDouble, f.assignVal
				// `x=$(date) awk` is still a command prefix; `$(date) awk` is
				// not — there the substitution WAS the command.
				cmdPos = assignVal
			} else if topCase() == casePattern {
				setCase(caseBody)
				cmdPos = true
			} else {
				cmdPos = true
			}
			i++
		case c == ';' && i+1 < len(src) && src[i+1] == ';':
			if topCase() == caseBody {
				setCase(casePattern)
			}
			cmdPos, assignVal = false, false
			i += 2
		case c == '`':
			// Read as a separator so the word after it is at least scanned as a
			// command; the report is what the caller acts on, because the
			// CLOSING backtick opens a command position too and the word after
			// THAT is a false positive.
			report("`", i)
			cmdPos, assignVal = true, false
			i++
		case c == '\n' || c == ';' || c == '|' || c == '&' || c == '(' || c == '{':
			cmdPos, assignVal = true, false
			i++
		case c == ' ' || c == '\t':
			assignVal = false
			i++
		case isWord(c):
			j := i
			for j < len(src) && isWord(src[j]) {
				j++
			}
			w := src[i:j]
			assignment := j < len(src) && src[j] == '='
			i = j
			switch {
			case assignment:
				// A VAR=value prefix leaves the next word still a command; the
				// value is walked with assignVal set so nothing in it emits and
				// nothing in it closes the position.
				assignVal = true
			case assignVal:
				// A bare word inside an assignment's value.
			// `esac` is read BEFORE the pattern-list arm: `;;` returns the
			// scanner to pattern state, so the closing `esac` of a one-line
			// `case ... in p) cmd ;; esac` arrives in it. Swallowing it there
			// left the case open for the rest of the file and suppressed every
			// command after it — the whole scan went quiet with no error.
			case w == "esac":
				if len(caseState) > 0 {
					caseState = caseState[:len(caseState)-1]
				}
				cmdPos = false
			case findInlineCommand[w]:
				// find's own command argument, and it is an OPTION word, so it
				// arrives with the command position already closed by `find`.
				report(w, j-len(w))
			case allDigits(w) && j < len(src) && (src[j] == '>' || src[j] == '<'):
				// The fd of a redirection, not a command: `2>&1`. Written
				// before the command name it used to be EMITTED as one —
				// `2>/dev/null awk` derived a dependency called "2".
			case topCase() == casePattern:
				cmdPos = false
			case w == "case" && cmdPos:
				caseState = append(caseState, caseSubject)
				cmdPos = false
			case w == "in" && topCase() == caseSubject:
				setCase(casePattern)
				cmdPos = false
			case !cmdPos:
			case w[0] == '-':
				// An option word is never a command name, and the command can
				// still follow it: `time -p cmd`, `command -v cmd`. An option
				// that takes a VALUE is where this stops — `exec -a nm awk`
				// derives "nm" — and it stops LOUDLY, with a name no HOOK_DEPS
				// holds, which is the direction this file can survive.
			case w == "if" || w == "then" || w == "else" || w == "elif" ||
				w == "while" || w == "until" || w == "do" || w == "time" ||
				w == "exec" || w == "command":
				// A command follows these; stay in a command position. exec and
				// command are builtins that RUN their argument, so unlike the
				// wrappers below there is no option table to walk past.
			case w == "for" || w == "select":
				cmdPos = false // name, `in`, and a word list — not commands
			case w == "fi" || w == "done":
				cmdPos = false
			default:
				if runsAnotherCommand[w] {
					report(w, j-len(w))
				}
				if !noBinary[w] {
					emit(w, j-len(w))
				}
				cmdPos = false
			}
		default:
			endWord()
			i++
		}
	}
	return out, blind
}

// substFrame is the scanner state a `$( )` saves and its `)` restores.
type substFrame struct {
	inDouble  bool
	assignVal bool
}

// runsAnotherCommand are the words that run a command this scan cannot reach.
// Two kinds, reported the same way because the reader's next move is the same:
//   - WRAPPERS that put their OWN options in front of the command. Walking past
//     `env -u NAME awk` or `nice -n 5 awk` to reach `awk` needs a table of
//     which of those options take a value — a hand-maintained list of other
//     commands' interfaces, which is the species of thing this file exists to
//     delete.
//   - STRING RUNNERS, whose argument is shell text the shell parses later:
//     `eval "awk 1"`, and `trap 'awk 1' EXIT` which is the same claim with the
//     run deferred to a signal. Both were reachable only by re-entering the
//     scanner on a string that may not exist until runtime.
//
// So they are reported instead: a hook that grows one is a finding, not a
// silence. Several of them are real binaries and are emitted as commands too;
// being a dependency and hiding a dependency are not the same claim, and the
// builtins among them (eval, trap) are reported without being emitted at all.
var runsAnotherCommand = map[string]bool{
	"env": true, "xargs": true, "nice": true, "ionice": true, "timeout": true,
	"nohup": true, "setsid": true, "chroot": true, "stdbuf": true, "flock": true,
	"watch": true, "sudo": true, "doas": true, "su": true, "eval": true,
	"sh": true, "bash": true, "dash": true, "ksh": true, "zsh": true,
	"trap": true, ".": true, "source": true,
}

// allDigits reports whether w is a file-descriptor number.
func allDigits(w string) bool {
	for i := 0; i < len(w); i++ {
		if w[i] < '0' || w[i] > '9' {
			return false
		}
	}
	return w != ""
}

// findInlineCommand are find's option words whose argument is a command line.
var findInlineCommand = map[string]bool{
	"-exec": true, "-execdir": true, "-ok": true, "-okdir": true,
}

// blindSiteDescription turns a reported site into the sentence a reader needs,
// because "`" and "env" and "<<" fail for three different reasons.
func blindSiteDescription(name string) string {
	switch {
	case name == "`":
		return "a backtick command substitution in shell code"
	case name == "<<":
		return "a heredoc: its body is data, and this scanner is reading it as code"
	case name == "$((":
		return "a command substitution inside a $(( )) arithmetic expansion, which this scanner steps over unread"
	case name == "${":
		return "a command substitution inside a ${ } parameter expansion, which this scanner steps over unread"
	case name == "trap":
		return "trap: its handler is shell text the shell parses later, like eval"
	case name == "." || name == "source":
		return "`" + name + " file`: the commands are in another file, which this scanner does not open"
	case findInlineCommand[name]:
		return "find's " + name + ", whose argument is a command line"
	default:
		return name + " runs the command that follows its own options"
	}
}

// skipRedirect steps over one redirection operator and its target, starting at
// the `>` or `<`, and reports whether the operator was a heredoc. It leaves the
// command position exactly as it found it: a redirection can be written before
// the command name, and POSIX says the command is still what follows.
func skipRedirect(src string, i int) (int, bool) {
	start := i
	for i < len(src) && (src[i] == '>' || src[i] == '<' || src[i] == '&') {
		i++
	}
	if strings.HasPrefix(src[start:i], "<<") {
		return i, true // the body is data; the caller is told, not guessed at
	}
	for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
		i++
	}
	// The target word, quoted or not. Stopping at the operators keeps a bare
	// `>&1` or a `>out;cmd` from swallowing what follows.
	for i < len(src) {
		switch c := src[i]; {
		case c == '\'' || c == '"':
			for i++; i < len(src) && src[i] != c; i++ {
			}
			if i < len(src) {
				i++
			}
		case c == ' ' || c == '\t' || c == '\n' || c == ';' || c == '|' ||
			c == '&' || c == '(' || c == ')' || c == '<' || c == '>':
			return i, false
		default:
			i++
		}
	}
	return i, false
}

// shellCall is one command word and the 1-based line of the rendered hook it
// was found on, so a finding names where to look rather than only what.
type shellCall struct {
	Name string
	Line int
}

// skipArith steps over a $((...)) arithmetic expansion, whose contents are not
// shell words at all. Without it `$((i + 1))` reads as a command substitution
// opening a subshell and `i` is scanned as a command name.
func skipArith(src string, i int) int {
	depth := 0
	for i < len(src) {
		if src[i] == '(' {
			depth++
		} else if src[i] == ')' {
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return len(src)
}

// skipBraceExpansion steps over a ${...} parameter expansion. Its insides are
// a variable name and, in the trim forms, a PATTERN: `${rule#Bash(}` and
// `${RHQ_PERSONA:-operator}` both read as shell syntax opening a command
// position otherwise.
func skipBraceExpansion(src string, i int) int {
	depth := 0
	for i < len(src) {
		if src[i] == '{' {
			depth++
		} else if src[i] == '}' {
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return len(src)
}

// shellExecProbeNames reads the one idiom shellCommandWords cannot: a command
// found by walking PATH by hand and then run through a variable. Both hooks do
// it in posse_stamp, which resolves `date` that way on purpose — a bare `date`
// in a persona session is answered by that session's own gate shim
// (ranger-base-l97n) — and the literal name survives in the `-x` test.
var shellExecProbe = regexp.MustCompile(`\[ -x "\$[A-Za-z_][A-Za-z0-9_]*/([A-Za-z0-9_.-]+)"`)

func shellExecProbeNames(src string) []shellCall {
	var out []shellCall
	for _, m := range shellExecProbe.FindAllStringSubmatchIndex(src, -1) {
		out = append(out, shellCall{Name: src[m[2]:m[3]], Line: 1 + strings.Count(src[:m[0]], "\n")})
	}
	return out
}

// hookCensus derives the commands a set of rendered hooks calls, and the blind
// sites where one runs that shellCommandWords could not name. It is a function
// and not a paragraph inside the test so that the REPORTING path is reachable
// from a pin: a guard whose failure branch no test has ever entered is a guard
// nobody has measured (ranger-base-h6k2r).
func hookCensus(rendered map[string]string, notPathLookups map[string]bool) (called map[string]string, blind []string) {
	called = map[string]string{} // command -> which hook calls it
	for _, name := range sortedKeys(rendered) {
		body := rendered[name]
		words, sites := shellCommandWords(body)
		for _, b := range sites {
			blind = append(blind, fmt.Sprintf("%s:%d: %s", name, b.Line, blindSiteDescription(b.Name)))
		}
		for _, c := range append(words, shellExecProbeNames(body)...) {
			if notPathLookups[c.Name] || strings.HasPrefix(c.Name, "posse_") {
				continue // posse_* are the hooks' own shell functions
			}
			if _, ok := called[c.Name]; !ok {
				called[c.Name] = fmt.Sprintf("%s:%d", name, c.Line)
			}
		}
	}
	return called, blind
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var hookDepsLine = regexp.MustCompile(`HOOK_DEPS="\$\{HOOK_DEPS:-([^}"]*)\}"`)

func TestHookDepsNamesEveryCommandTheRenderedHooksCall(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	cmd := exec.Command("git", "-C", repo, "init", "-q", "-b", "main")
	cmd.Env = []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo}
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, b)
	}
	hookPath, err := installCommitGuard(repo)
	if err != nil {
		t.Fatalf("install prepare-commit-msg: %v", err)
	}
	commitGuard, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}

	// The dispatcher's two operands are hook FILES addressed by path in the
	// same hooks dir, not PATH lookups: rendering it with names of our
	// choosing is also what makes them recognizable here.
	const neighbour = "theirs-prepare-commit-msg"
	rendered := map[string]string{
		"prepare-commit-msg": string(commitGuard),
		"pre-push":           PrePushHook,
		"chain dispatcher":   chainHookDispatcherWith("prepare-commit-msg", neighbour),
	}
	notPathLookups := map[string]bool{
		// git is what RUNS the hooks; deliberately absent from HOOK_DEPS.
		"git": true,
		// hook files in the dispatcher's own dir, reached as "$d/<name>".
		neighbour: true, "posse-prepare-commit-msg": true,
	}

	called, blind := hookCensus(rendered, notPathLookups)
	for _, b := range blind {
		t.Errorf("%s — shellCommandWords cannot name the command that runs there, so it is missing "+
			"from this census and the clean-room probe reports every distro clean for it. Add the "+
			"command to HOOK_DEPS and say here why the scanner cannot see the call, or rewrite the "+
			"hook line in a shape it can. See the COMPLETENESS paragraph above.", b)
	}

	b, err := os.ReadFile("../../scripts/cleanroom.sh")
	if err != nil {
		t.Fatalf("read cleanroom.sh: %v", err)
	}
	m := hookDepsLine.FindSubmatch(b)
	if m == nil {
		t.Fatal("scripts/cleanroom.sh: no HOOK_DEPS=\"${HOOK_DEPS:-...}\" line — the probe this test pins is gone or renamed")
	}
	listed := map[string]bool{}
	for _, d := range strings.Fields(string(m[1])) {
		listed[d] = true
	}

	if testing.Verbose() {
		var all []string
		for c, where := range called {
			all = append(all, c+" ("+where+")")
		}
		sort.Strings(all)
		t.Logf("derived from the rendered hooks: %s", strings.Join(all, ", "))
	}
	var missing, spurious []string
	for c := range called {
		if !listed[c] {
			missing = append(missing, c+" (called by "+called[c]+")")
		}
	}
	for d := range listed {
		if _, ok := called[d]; !ok {
			spurious = append(spurious, d)
		}
	}
	sort.Strings(missing)
	sort.Strings(spurious)
	if len(missing) > 0 {
		t.Errorf("scripts/cleanroom.sh HOOK_DEPS does not name %d command(s) the rendered hooks call: %s\n"+
			"The clean-room probe reports every distro as clean for these. Add them.", len(missing), strings.Join(missing, ", "))
	}
	if len(spurious) > 0 {
		t.Errorf("scripts/cleanroom.sh HOOK_DEPS names %d command(s) no rendered hook calls: %s\n"+
			"A name in that list is a claim the hooks need it. Remove them, or say here why the scanner cannot see the call.",
			len(spurious), strings.Join(spurious, ", "))
	}
}

// ranger-base-cx2ok: the guard above used to be a whole-body
// strings.Contains(body, "`"), which reds on a backtick ANYWHERE in the
// rendered hook. Three of them are prose in comments — the commit guard quotes
// `rm -rf <marker>` and two `git mv` lines for a reader, from the marker and
// --no-renames paragraphs — and the whole internal/posse package was red on
// them, so nobody could honestly claim a green suite.
//
// A backtick is only a command substitution where the shell is reading CODE.
// The scanner already knows where that is: it tracks comments and single
// quotes to find command words at all. This pin is that discrimination —
// reported where a substitution would RUN, silent where the shell would read a
// literal.
func TestShellCommandWordsReportsBackticksInCodeNotInProse(t *testing.T) {
	const bt = "`"
	for _, tc := range []struct {
		name string
		src  string
		want bool // a backtick is reported
	}{
		// Reported: the shell would substitute here, and the scan of what is
		// inside is exactly what shellCommandWords cannot do.
		{"bare command substitution", "x=" + bt + "date" + bt, true},
		{"inside double quotes", "x=\"" + bt + "date" + bt + "\"", true},
		{"in a command position", bt + "hostname" + bt + " -f", true},
		{"after a code line's trailing comment", "ls # note\ny=" + bt + "date" + bt, true},
		// Silent: nothing runs, so the census is not missing anything.
		{"whole-line comment", "# quotes " + bt + "rm -rf rhq/agents" + bt + " for a reader", false},
		{"trailing comment", "ls -l  # see " + bt + "git mv a b" + bt + " above", false},
		{"single quotes", "echo '" + bt + "date" + bt + "'", false},
		{"escaped, unquoted", "echo \\" + bt, false},
		{"escaped inside double quotes", "echo \"\\" + bt + "\"", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, backticks := shellCommandWords(tc.src)
			if got := len(backticks) > 0; got != tc.want {
				t.Errorf("shellCommandWords(%q) reported %d backtick(s), want reported=%v",
					tc.src, len(backticks), tc.want)
			}
		})
	}

	// The live witness. The rendered commit guard carries backticks today and
	// they are all prose; splicing ONE into shell code is the only difference
	// between silence and a finding. Without this arm the table above is a
	// scanner unit test that no rendered hook has to keep satisfying.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	cmd := exec.Command("git", "-C", repo, "init", "-q", "-b", "main")
	cmd.Env = []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo}
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, b)
	}
	hookPath, err := installCommitGuard(repo)
	if err != nil {
		t.Fatalf("install prepare-commit-msg: %v", err)
	}
	b, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, bt) {
		t.Errorf("the rendered prepare-commit-msg no longer contains a backtick at all: " +
			"this arm witnesses that the guard tolerates PROSE backticks, and it now witnesses nothing. " +
			"If the template deliberately dropped them, delete this paragraph and say so.")
	}
	if _, backticks := shellCommandWords(body); len(backticks) > 0 {
		var where []string
		for _, c := range backticks {
			where = append(where, fmt.Sprintf("line %d", c.Line))
		}
		t.Errorf("rendered prepare-commit-msg: %d backtick(s) read as shell code (%s)",
			len(backticks), strings.Join(where, ", "))
	}
	// Two, not one: the scanner reports SITES, and a substitution opens and
	// closes with one each. Both are worth naming — the closing backtick is
	// where the scan resumes reading words as commands.
	if _, backticks := shellCommandWords(body + "\nposse_x=" + bt + "date" + bt + "\n"); len(backticks) != 2 {
		t.Errorf("a real substitution spliced into the rendered hook was reported at %d site(s), want 2 — "+
			"the guard would not catch the thing it exists for", len(backticks))
	}
}

// ranger-base-h6k2r: the paragraph on shellCommandWords named two blind spots
// when there were ten, and a reader takes that paragraph as the completeness
// statement for the census — which is the one property this file exists to give
// back (ranger-base-lxkdi). None of the ten was a live miss, so nothing was red
// while the claim was false; that is exactly the silence this pin ends.
//
// The standard the two named blind spots were already held to, now applied to
// all of them: a command-prefix shape is either SEEN — the command word comes
// out of the census — or REPORTED, which the caller above fails on. Nothing is
// allowed to be quiet. The wrong arm is what makes this a measurement: every
// "reported" row returned no report before the fix, and every "seen" row
// returned no `awk`.
func TestShellCommandWordsSeesEveryCommandPrefixOrReportsIt(t *testing.T) {
	const seen, reported, loud = "seen", "reported", "loud"
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		// Shapes the scanner already read, kept here so a rewrite of the
		// command-prefix arms cannot quietly cost one of them.
		{"plain", "awk '{print}' f", seen},
		{"pipeline", "cat f | awk '{print}'", seen},
		{"and-and", "true && awk '{print}'", seen},
		{"command substitution", "x=$(awk '{print}' f)", seen},
		{"case body", "case $x in a) awk '{print}' ;; esac", seen},
		{"brace group", "{ awk '{print}' f; }", seen},
		{"subshell", "( awk '{print}' f )", seen},
		{"while condition", "while awk '{print}' f; do :; done", seen},
		{"time prefix", "time awk '{print}' f", seen},

		// Taught by h6k2r. Each returned no `awk` before it.
		{"assignment prefix", "AWKPATH=/x awk '{print}'", seen},
		{"assignment with a colon in the value", "PATH=/a:/b awk '{print}'", seen},
		{"assignment with a quoted value", "X=\"a b\" awk '{print}'", seen},
		{"two assignment prefixes", "A=1 B=2 awk '{print}'", seen},
		{"assignment from a substitution", "X=$(date) awk '{print}'", seen},
		{"negation", "! awk '{print}' f", seen},
		{"negation inside if", "if ! awk '{print}' f; then :; fi", seen},
		{"leading redirection", ">out awk '{print}'", seen},
		{"leading fd redirection", "2>/dev/null awk '{print}'", seen},
		{"exec", "exec awk '{print}'", seen},
		{"command builtin", "command awk '{print}'", seen},
		{"valueless option before the command", "command -v awk", seen},
		{"option taking a value", "exec -a nm awk '{print}'", loud},

		// Reported by h6k2r: reaching the command needs a table of ANOTHER
		// command's options, which is the hand-maintained list this file exists
		// to delete. Each was silent before.
		{"env with an assignment", "env AWKPATH=/x awk '{print}'", reported},
		{"env -i", "env -i awk '{print}'", reported},
		{"xargs", "cat f | xargs awk '{print}'", reported},
		{"nice", "nice awk '{print}'", reported},
		{"timeout", "timeout 5 awk '{print}'", reported},
		{"nohup", "nohup awk '{print}' f", reported},
		{"stdbuf", "stdbuf -oL awk '{print}' f", reported},
		{"sudo", "sudo awk '{print}' f", reported},
		{"eval", "eval \"awk '{print}'\"", reported},
		{"sh -c", "sh -c 'awk 1'", reported},
		{"find -exec", "find . -exec awk {} ;", reported},
		{"heredoc", "cat <<EOF\nawk\nEOF\n", reported},

		// The FOURTH bucket ranger-base-xwepd found: eight shapes that ran a
		// command and produced neither a word nor a report, under a paragraph
		// asserting there was no fourth. The first five are one arm — a
		// backslash used to close the command position, which is right for
		// neither of the two things a backslash does at that spot.
		{"continuation in a pipeline", "cat f |\\\n  awk 1", seen},
		{"continuation after &&", "grep -q x f && \\\n awk 1", seen},
		{"continuation after then", "if grep -q x f; then \\\n awk 1\nfi", seen},
		{"continuation after ;", "true; \\\n awk 1", seen},
		{"escaped command name", "\\awk 1", seen},
		// The control the continuation rows are read against: one backslash is
		// the whole difference, and this arm was always right.
		{"pipeline across a plain newline", "cat f |\n  awk 1", seen},
		// The other three: the string runner that is eval with the run
		// deferred, and the two regions this scan steps OVER unread — one
		// row each for the `$(` and the backtick spelling inside them,
		// because the two are separate checks in skipOver.
		{"trap handler", "trap 'awk 1' EXIT", reported},
		{"substitution in a parameter expansion", "printf %s \"${x:-$(awk 1)}\"", reported},
		{"backtick in a parameter expansion", "printf %s \"${x:-`awk 1`}\"", reported},
		{"substitution in arithmetic", "n=$(( $(awk 1) + 1 ))", reported},
		// Not one of the eight; the same reasoning applied while it was
		// open, and free because no hook sources anything today.
		{"dot script", ". lib.sh", reported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			words, blind := shellCommandWords(tc.src)
			var got []string
			sees := false
			for _, w := range words {
				got = append(got, w.Name)
				if w.Name == "awk" {
					sees = true
				}
			}
			switch tc.want {
			case seen:
				if !sees {
					t.Errorf("shellCommandWords(%q) derived %v — `awk` runs there and the census "+
						"does not name it, so HOOK_DEPS can go short of it in silence", tc.src, got)
				}
				if len(blind) > 0 {
					t.Errorf("shellCommandWords(%q) reported %q as a blind site but the command "+
						"IS derivable here; a report the reader cannot act on trains them to ignore reports",
						tc.src, blind[0].Name)
				}
			case loud:
				// The limit of the option-word arm. Not silence: the census
				// derives a name nothing can satisfy, so it FAILS rather than
				// under-reporting, which is the direction that is survivable.
				if sees || len(got) == 0 {
					t.Errorf("shellCommandWords(%q) derived %v — this row exists to hold the "+
						"failure DIRECTION at the option-word arm's limit: a spurious name the "+
						"census reds on, never a quiet miss", tc.src, got)
				}
			case reported:
				if len(blind) == 0 {
					t.Errorf("shellCommandWords(%q) derived %v and reported nothing — the command "+
						"behind that prefix is invisible to the census AND to the reader", tc.src, got)
				}
			}
		})
	}

	// The report tables are not decoration: every name in them has to actually
	// produce a report from a command position. A name added to the map and
	// never reached is a blind spot wearing a fix.
	for w := range runsAnotherCommand {
		if _, blind := shellCommandWords(w + " awk 1"); len(blind) == 0 {
			t.Errorf("runsAnotherCommand names %q but %q reported nothing", w, w+" awk 1")
		}
	}
	for w := range findInlineCommand {
		if _, blind := shellCommandWords("find . " + w + " awk {} ;"); len(blind) == 0 {
			t.Errorf("findInlineCommand names %q but `find . %s awk {} ;` reported nothing", w, w)
		}
	}

	// Every marker the scanner can report needs its OWN sentence.
	// blindSiteDescription's default arm is prose about a WRAPPER's option
	// grammar; a marker falling through to it sends the reader looking past
	// options that do not exist. `${` and `$((` are reachable from neither
	// report table, so this is the only sweep that reaches them.
	//
	// The default's text is read off a name that must hit it rather than
	// copied here, so rewording that arm cannot leave this sweep green over
	// a marker that still falls into it.
	fallthroughText := strings.TrimPrefix(blindSiteDescription("env"), "env")
	if fallthroughText == blindSiteDescription("env") {
		t.Fatalf("`env` no longer reaches blindSiteDescription's default arm, so this sweep has no "+
			"reference text and cannot tell a marker's own sentence from the wrapper default: %q",
			blindSiteDescription("env"))
	}
	for _, marker := range []string{"`", "<<", "$((", "${", "trap", ".", "source", "-exec"} {
		if d := blindSiteDescription(marker); d == marker+fallthroughText {
			t.Errorf("blindSiteDescription(%q) = %q — that is the WRAPPER default, and %q is not a "+
				"wrapper; the reader is told to look past options that do not exist", marker, d, marker)
		}
	}

	// The live witness, and it goes through hookCensus rather than the scanner
	// alone: a table over string literals is a scanner unit test no rendered
	// hook has to keep satisfying, and a report the census does not act on is
	// no better than a silence. Both arms are the rendered pre-push; the only
	// difference is one line of the shape the bead says this hook cluster is
	// one commit from growing.
	clean, blind := hookCensus(map[string]string{"pre-push": PrePushHook}, nil)
	if len(blind) > 0 {
		t.Errorf("rendered pre-push already reports a blind site (%s) — the splices below then "+
			"witness nothing", blind[0])
	}
	if _, ok := clean["awk"]; ok {
		t.Fatalf("the rendered pre-push already calls awk; pick another command for this witness")
	}
	for _, line := range []string{
		"env -i awk '{print}' \"$f\"", // constitutionGuardBody's own residual paragraph names env -i
		"timeout 5 awk '{print}' \"$f\"",
		"! awk '{print}' \"$f\"",
		// ranger-base-xwepd's shapes, spliced the same way: a table over string
		// literals is a scanner unit test no rendered hook has to keep
		// satisfying, and a wrapped pipeline is the most ordinary line here.
		"cat \"$f\" |\\\n  awk '{print}'",
		"trap 'awk 1' EXIT",
		"printf %s \"${x:-$(awk 1)}\"",
	} {
		called, blind := hookCensus(map[string]string{"pre-push": PrePushHook + "\n" + line + "\n"}, nil)
		_, named := called["awk"]
		if !named && len(blind) == 0 {
			t.Errorf("%q spliced into the rendered pre-push: the census neither named awk nor "+
				"reported a blind site. A hook growing that line leaves HOOK_DEPS short of awk and "+
				"the clean-room probe calling every distro clean for it — the silence lxkdi was "+
				"filed to end, now with a green test over it", line)
		}
	}
}
