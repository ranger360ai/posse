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
// order first seen.
//
// It is a scanner, not a parser: it tracks quoting, comments, command
// substitution and `case` pattern lists, which is exactly what a substring grep
// cannot do. Grep is not an option here for a measured reason: "tr" occurs 68
// times in the rendered prepare-commit-msg and three of those are the command
// (the rest are "tree", "instance", "restate"), and "rm" occurs 30 times and
// none of them are the command ("form", "performed").
//
// The one thing it cannot see is a command invoked through a variable. Both
// hooks do that once, in posse_stamp, and shellExecProbeNames below reads that
// idiom's literal name off the `[ -x "$dir/name" ]` test that finds it.
func shellCommandWords(src string) []shellCall {
	var out []shellCall
	seen := map[string]bool{}
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
		"test": true, "times": true, "trap": true, "true": true, "type": true,
		"ulimit": true, "umask": true, "unalias": true, "unset": true, "wait": true,
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

	var doubleStack []bool // saved inDouble across `$(`
	inDouble, inSingle := false, false
	cmdPos := true

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
				cmdPos = false
				i++
			case c == '$' && i+2 < len(src) && src[i+1] == '(' && src[i+2] == '(':
				i = skipArith(src, i)
			case c == '$' && i+1 < len(src) && src[i+1] == '(':
				doubleStack = append(doubleStack, true)
				inDouble = false
				cmdPos = true
				i += 2
			case c == '$' && i+1 < len(src) && src[i+1] == '{':
				i = skipBraceExpansion(src, i)
			default:
				i++
			}
		case c == '\\' && i+1 < len(src):
			i += 2
			cmdPos = false
		case c == '\'':
			inSingle = true
			cmdPos = false
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
			i = skipArith(src, i)
			cmdPos = false
		case c == '$' && i+1 < len(src) && src[i+1] == '(':
			doubleStack = append(doubleStack, false)
			cmdPos = true
			i += 2
		case c == '$' && i+1 < len(src) && src[i+1] == '{':
			i = skipBraceExpansion(src, i)
			cmdPos = false
		case (c == '&' || c == '>' || c == '<') && i > 0 && (src[i-1] == '>' || src[i-1] == '<'):
			// A redirection operator, not a separator: `2>&1`, `>>file`.
			i++
		case c == ')':
			// A `)` closing a command substitution we opened is unambiguous, so
			// it is popped BEFORE the case-pattern reading: `$(posse_stamp)`
			// occurs inside case bodies, and reading its `)` as the end of a
			// pattern list left the rest of the double-quoted message being
			// scanned as commands.
			if n := len(doubleStack); n > 0 {
				inDouble = doubleStack[n-1]
				doubleStack = doubleStack[:n-1]
				cmdPos = false
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
			cmdPos = false
			i += 2
		case c == '\n' || c == ';' || c == '|' || c == '&' || c == '(' || c == '{' || c == '`':
			cmdPos = true
			i++
		case c == ' ' || c == '\t':
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
				// A VAR=value prefix leaves the next word still a command.
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
			case topCase() == casePattern:
				cmdPos = false
			case w == "case" && cmdPos:
				caseState = append(caseState, caseSubject)
				cmdPos = false
			case w == "in" && topCase() == caseSubject:
				setCase(casePattern)
				cmdPos = false
			case !cmdPos:
			case w == "if" || w == "then" || w == "else" || w == "elif" ||
				w == "while" || w == "until" || w == "do" || w == "time":
				// A command follows these; stay in a command position.
			case w == "for" || w == "select":
				cmdPos = false // name, `in`, and a word list — not commands
			case w == "fi" || w == "done":
				cmdPos = false
			default:
				if !noBinary[w] {
					emit(w, j-len(w))
				}
				cmdPos = false
			}
		default:
			cmdPos = false
			i++
		}
	}
	return out
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

var hookDepsLine = regexp.MustCompile(`HOOK_DEPS="\$\{HOOK_DEPS:-([^}"]*)\}"`)

func TestHookDepsNamesEveryCommandTheRenderedHooksCall(t *testing.T) {
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

	called := map[string]string{} // command -> which hook calls it
	for name, body := range rendered {
		if strings.Contains(body, "`") {
			t.Errorf("%s: backtick substitution — shellCommandWords does not scan it", name)
		}
		for _, c := range append(shellCommandWords(body), shellExecProbeNames(body)...) {
			if notPathLookups[c.Name] || strings.HasPrefix(c.Name, "posse_") {
				continue // posse_* are the hooks' own shell functions
			}
			if _, ok := called[c.Name]; !ok {
				called[c.Name] = fmt.Sprintf("%s:%d", name, c.Line)
			}
		}
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
