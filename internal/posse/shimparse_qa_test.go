//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-i36bf — EVERY RENDERED GATE SHIM PARSES, AND CARRIES NO SHAPE
// darwin's /bin/sh CANNOT PARSE.
//
// THE GAP. `make verify-shell-syntax` (scripts/shell-syntax.sh,
// ranger-base-g4z8m) parses every TRACKED shell script with the shell its own
// shebang names. The L1 gate shims are RENDERED by Go at launch and are
// tracked nowhere, so that sweep cannot see them, and until this file nothing
// in this tree ever asked whether a rendered shim parses at all.
//
// WHAT IT COST. ranger-base-5blmt (9193d3b1) added renderSeatObjectEnv, whose
// argv walk was a `case` inside a command substitution:
//
//	posse_so_c=$(
//	  …
//	  case "$1" in
//	    -C) … ;;
//
// MEASURED macos-latest (image macos-26-arm64 20261117.0516.1, Darwin 25.6.0
// arm64, bash 3.2.57 as /bin/sh, git 2.56.1), CI run 44927635225 job
// 136917129851, macos arm 3:
//
//	…/state/gates/developer/bin/git: line 403: syntax error near unexpected
//	token ';;'
//
// bash 3.2 scans `$( )` for its closing paren while tracking quote state, so
// the first case pattern's own `)` ends the substitution and the `;;` after it
// is left outside any case. The FILE does not parse, so nothing in it runs:
// every git through a darwin seat's gates PATH exited 2 — the PID's deny
// rules, the alias arm and the sequencer audit included. It fails CLOSED, but
// a darwin seat could not run git at all, for six days (green at edfe7a79, run
// 43731869157; red at 9193d3b1 and every macos arm-3 leg since).
//
// AND IT IS INVISIBLE ON LINUX. bash 5 parses the same shape clean — that is
// the measurement scripts/shell-syntax.sh's header already carries for the
// heredoc half of this class (ranger-base-ftr3e, bash 5.2.21 on
// ubuntu-latest) — so the only reader was a macos CI leg, and macos arm 1 had
// been red for other reasons since 2026-11-11 (ranger-base-zcj0z). This is
// the third darwin-only bash 3.2 bite in the tree in a week.
//
// SO THE PIN IS TWO HALVES, and the second is the one that answers on a linux
// box:
//
//   - EVERY rendered shim parses under this box's /bin/sh. On darwin that IS
//     bash 3.2, so there the pin catches this class outright; everywhere else
//     it catches a shim that does not parse at all.
//   - and no rendered shim carries a `case` arm, inside a command
//     substitution, whose pattern does not open with `(`. That is a
//     STRUCTURAL reading of the hazard, so it answers the same on every box,
//     which is the whole point: the renderers are edited from linux seats.
//
// WHAT THIS PIN DOES NOT CLAIM. The leading paren (`(-C)`) is legal and
// parses — renderHookSkipGlobalScope uses it and the same shim carried it
// green on darwin through edfe7a79 — so the rule below ADMITS it. What
// renderSeatObjectEnv took instead is the function form, because a function
// body is parsed at the top level where this hazard does not exist and an arm
// added later cannot reintroduce it. The rule is the weaker of the two on
// purpose: it is a wall against the error, not a vote between the two fixes.
//
// AND ONE READING OF bash 3.2's SCANNER, taken from that error rather than
// from a darwin box, because this seat is linux. The old text's line above the
// `-C)` arm was a comment carrying an apostrophe (`this subshell's output is
// the answer`), INSIDE the same substitution. Had the 3.2 scanner tracked
// quote state through `#` comments, that apostrophe would have opened a quote
// that swallowed the arm, and the error would have been an unterminated one
// far below — the shape ranger-base-g4z8m measured for a heredoc. It is not:
// the error names `;;` at the arm's own line, which is what a scanner that
// skipped the comment and stopped at the pattern's `)` produces. So an
// apostrophe in a COMMENT inside `$( )` is not this hazard, and the rule below
// does not pretend to cover it. The heredoc half of the class stays
// scripts/shell-syntax.sh's.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the rule, as a scanner ─────────────────────────────────────────────────

// b32 frame kinds. A frame is one open construct; `subst` is sticky down the
// stack, because the question the rule asks is whether a `case` is anywhere
// inside a command substitution.
const (
	b32Top = iota
	b32Subst
	b32Paren
	b32Case
)

// states of a b32Case frame, in the order a case statement passes through
// them: `case` → subject → `in` → pattern → `)` → body → `;;` → pattern …
const (
	b32WantIn = iota
	b32WantPat
	b32InPat
	b32InBody
)

type b32Frame struct {
	kind  int
	state int  // b32Case only
	lead  bool // b32Case only: the arm being read opened with `(`
	subst bool // this frame, or any frame below it, is a command substitution
	dq    bool // a double quote is open in this frame
	line  int
}

// b32CaseArmsInSubstitution reads rendered POSIX sh and reports every `case`
// arm that sits inside a command substitution with no leading `(` on its
// pattern — the shape that is a syntax error under darwin's bash 3.2 and
// parses clean under bash 5.
//
// It is a scanner, not a parser. It knows single quotes, double quotes,
// backslash escapes, `#` comments, `$( )`, `( )`, `case`/`in`/`esac`/`;;`, and
// nothing else — which is every construct the shim renderers emit (there is no
// heredoc and no backquote in a shim; the one rendered heredoc in this package
// is in a git hook, gates.go, and is not a shim). `$(( ))` reads as a `$(`
// with a `(` inside it, which is all this rule needs of it.
//
// TWO RETURNS, because "could not read it" is not "it is clean": findings, and
// the reason the scan gave up. A caller must fail on either — a scanner that
// lost its place has measured nothing, and a pin that read that as a pass
// would be green over exactly the renderer edit that confused it.
func b32CaseArmsInSubstitution(src string) (findings []string, unreadable string) {
	frames := []b32Frame{{kind: b32Top}}
	line := 1
	cur := func() *b32Frame { return &frames[len(frames)-1] }
	push := func(kind int) {
		f := b32Frame{kind: kind, subst: cur().subst || kind == b32Subst, line: line}
		frames = append(frames, f)
	}
	pop := func() { frames = frames[:len(frames)-1] }
	lost := func(why string) ([]string, string) {
		return nil, fmt.Sprintf("line %d: %s", line, why)
	}
	wordChar := func(b byte) bool {
		return b == '_' || b == '-' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
	}
	// word <w> whole, at a position a command or keyword can begin at.
	word := func(i int, w string) bool {
		if !strings.HasPrefix(src[i:], w) {
			return false
		}
		if j := i + len(w); j < len(src) && wordChar(src[j]) {
			return false
		}
		if i == 0 {
			return true
		}
		switch src[i-1] {
		case ' ', '\t', '\n', ';', '&', '|', '(':
			return true
		}
		return false
	}
	// The whole line index i falls on, for the finding's own evidence.
	lineText := func(i int) string {
		a := strings.LastIndexByte(src[:i], '\n') + 1
		b := strings.IndexByte(src[i:], '\n')
		if b < 0 {
			return strings.TrimSpace(src[a:])
		}
		return strings.TrimSpace(src[a : i+b])
	}

	for i := 0; i < len(src); {
		b := src[i]
		if b == '\n' {
			line++
			i++
			continue
		}
		f := cur()
		if f.dq {
			switch {
			case b == '\\' && i+1 < len(src):
				if src[i+1] == '\n' {
					line++
				}
				i += 2
			case b == '"':
				f.dq = false
				i++
			case b == '$' && i+1 < len(src) && src[i+1] == '(':
				push(b32Subst)
				i += 2
			default:
				i++
			}
			continue
		}
		// A case frame's own grammar comes first: inside it a `)` closes an
		// arm rather than a paren, and that is the whole difference between
		// this scanner and a paren counter.
		if f.kind == b32Case {
			switch f.state {
			case b32WantIn:
				if word(i, "in") {
					f.state = b32WantPat
					i += 2
					continue
				}
			case b32WantPat:
				if b == ' ' || b == '\t' {
					i++
					continue
				}
				if word(i, "esac") {
					pop()
					i += 4
					continue
				}
				if b == '(' {
					f.lead = true
					f.state = b32InPat
					i++
					continue
				}
				f.lead = false
				f.state = b32InPat
				// and the char itself falls through to the general reader.
			case b32InPat:
				if b == ')' {
					if !f.lead && f.subst {
						findings = append(findings, fmt.Sprintf(
							"line %d: a case arm inside a command substitution whose pattern does not open with `(`: %s", line, lineText(i)))
					}
					f.state = b32InBody
					i++
					continue
				}
			case b32InBody:
				if b == ';' && i+1 < len(src) && src[i+1] == ';' {
					f.state = b32WantPat
					i += 2
					continue
				}
				if word(i, "esac") {
					pop()
					i += 4
					continue
				}
			}
		}
		switch {
		case b == '\\' && i+1 < len(src):
			if src[i+1] == '\n' {
				line++
			}
			i += 2
		case b == '\'':
			j := strings.IndexByte(src[i+1:], '\'')
			if j < 0 {
				return lost("a single quote never closes")
			}
			line += strings.Count(src[i:i+1+j+1], "\n")
			i += j + 2
		case b == '`':
			return lost("a backquote substitution — this scanner reads `$( )` only, so it cannot say whether this parses")
		case b == '#' && (i == 0 || src[i-1] == ' ' || src[i-1] == '\t' || src[i-1] == '\n'):
			if j := strings.IndexByte(src[i:], '\n'); j < 0 {
				i = len(src)
			} else {
				i += j // leave the newline to the counter
			}
		case b == '"':
			f.dq = true
			i++
		case b == '$' && i+1 < len(src) && src[i+1] == '(':
			push(b32Subst)
			i += 2
		case b == '(':
			push(b32Paren)
			i++
		case b == ')':
			if k := cur().kind; k != b32Subst && k != b32Paren {
				return lost("a `)` closing nothing this scanner saw open")
			}
			pop()
			i++
		case word(i, "case"):
			push(b32Case)
			cur().state = b32WantIn
			i += 4
		case word(i, "esac"):
			return lost("an `esac` with no `case` open")
		default:
			i++
		}
	}
	if len(frames) != 1 {
		return lost(fmt.Sprintf("%d construct(s) left open, the outermost from line %d", len(frames)-1, frames[1].line))
	}
	return findings, ""
}

// ─── the scanner's own both-polarity arms ───────────────────────────────────

// A pin over the renders alone is satisfied by a scanner that finds nothing
// ever, so the scanner is measured against fixtures first — the pre-fix text
// verbatim on the red side, and every shape the shim renderers really emit on
// the green one.
func TestQABash32CaseArmScannerSaysNoToTheShapeItWasWrittenFor(t *testing.T) {
	t.Parallel()
	// The red side: renderSeatObjectEnv as ranger-base-5blmt wrote it,
	// trimmed to the lines that matter and otherwise verbatim — the comment's
	// apostrophe included, because a scanner that choked on that would be
	// reporting the wrong defect.
	preFix := "if [ -n \"${POSSE_SEAT_OBJECTS:-}\" ]; then\n" +
		"  posse_so_c=$(\n" +
		"    unset CDPATH # a CDPATH hit makes cd print, and this subshell's output is the answer\n" +
		"    while [ $# -gt 0 ]; do\n" +
		"      case \"$1\" in\n" +
		"        -C) [ $# -ge 2 ] || exit 1; shift 2 ;;\n" +
		"        --git-dir=*) posse_so_gd=${1#--git-dir=}; shift ;;\n" +
		"        --bare) posse_so_gd=$(pwd); shift ;;\n" +
		"        clone|init) exit 1 ;;\n" +
		"        *) break ;;\n" +
		"      esac\n" +
		"    done\n" +
		"    pwd -P\n" +
		"  )\n" +
		"fi\n"
	got, unreadable := b32CaseArmsInSubstitution(preFix)
	if unreadable != "" {
		t.Fatalf("the scanner could not read the text it exists for: %s", unreadable)
	}
	// Five arms, and every one of them is the error: a pin that caught the
	// first and stopped would go quiet the day the first arm gets a paren.
	if len(got) != 5 {
		t.Errorf("the pre-fix shape has five unparenthesised arms inside $( ); the scanner found %d:\n  %s", len(got), strings.Join(got, "\n  "))
	}
	if len(got) > 0 && !strings.Contains(got[0], "-C)") {
		t.Errorf("the first finding does not name the arm darwin's bash reported: %s", got[0])
	}

	// The green side. Each of these is a shape the renderers in this package
	// really emit, and none of them is this hazard.
	for _, c := range []struct{ name, src string }{
		{"the fix: the walk is a function and the substitution calls it",
			"posse_so_target() {\n  while [ $# -gt 0 ]; do\n    case \"$1\" in\n      -C) shift 2 ;;\n      *) break ;;\n    esac\n  done\n  pwd -P\n}\nposse_so_c=$(posse_so_target \"$@\")\n"},
		{"the leading paren, which parses and which renderHookSkipGlobalScope uses",
			"x=$(\n  for l in $y; do\n    case \"$l\" in (system\"\t\"*|global\"\t\"*) printf '%s\\n' \"$l\" ;; esac\n  done\n)\n"},
		{"a case at the top level of a function, which is every other argv walk in the shim",
			"posse_verb_match() {\n  while [ $# -gt 0 ]; do\n    case \"$1\" in\n      -C) shift 2 ;;\n      -*) shift ;;\n      *) break ;;\n    esac\n  done\n  return 0\n}\n"},
		{"a comment carrying parens and an apostrophe, inside a substitution",
			"x=$(\n  # the seat's own store (ranger-base-5blmt): not a case arm\n  pwd\n)\n"},
		{"a quoted paren and an escaped one, inside a substitution",
			"x=$(\n  printf '%s\\n' \")\" \"a)b\"\n  echo \\)\n)\n"},
		{"arithmetic expansion, which is a `$(` with a `(` in it",
			"x=$(\n  n=$((n+1))\n  echo $n\n)\n"},
		{"a nested case in the body of a case inside a substitution, arms parenthesised",
			"x=$(\n  case \"$1\" in (a) case \"$2\" in (b) echo b ;; esac ;; (*) echo z ;; esac\n)\n"},
	} {
		got, unreadable := b32CaseArmsInSubstitution(c.src)
		if unreadable != "" {
			t.Errorf("%s: the scanner lost its place: %s", c.name, unreadable)
			continue
		}
		if len(got) != 0 {
			t.Errorf("%s: invented %d finding(s):\n  %s", c.name, len(got), strings.Join(got, "\n  "))
		}
	}

	// And the third state says so rather than passing. Without this arm every
	// green row above is also satisfied by a scanner that gives up silently.
	for _, c := range []struct{ name, src, want string }{
		{"an unterminated single quote", "x=$(echo 'oops\n)\n", "single quote"},
		{"a backquote substitution", "x=`pwd`\n", "backquote"},
		{"a stray close paren", "echo hi)\n", "closing nothing"},
		{"a substitution left open", "x=$(pwd\n", "left open"},
		{"an esac with no case", "esac\n", "no `case` open"},
	} {
		got, unreadable := b32CaseArmsInSubstitution(c.src)
		if unreadable == "" {
			t.Errorf("%s: the scanner reported a clean read (%d findings) over text it cannot judge", c.name, len(got))
			continue
		}
		if !strings.Contains(unreadable, c.want) {
			t.Errorf("%s: the reason does not name it (%q): %s", c.name, c.want, unreadable)
		}
		if got != nil {
			t.Errorf("%s: an unreadable text returned findings as well: %v", c.name, got)
		}
	}
}

// ─── the renders ────────────────────────────────────────────────────────────

// spDenyMatrix is a deny set per renderer branch the shim builder has: no rule
// at all (the git shim every seat gets regardless), a verb rule, a negative
// `unless` rule and its qualified-helper, a flag rule matched by membership,
// bd's write tail and ceiling arm, a non-git non-bd command, and a whole-tree
// deny. The shims are rendered, never run, so the rules' own refusals are not
// under test here — their SYNTAX is.
var spDenyMatrix = [][]string{
	nil,
	{"Bash(git push:*)"},
	{"Bash(git commit unless --)"},
	{"Bash(git push --force:*)", "Bash(git push:*)", "Bash(git commit unless --)", "Bash(git daemon:*)"},
	{"Bash(bd daemon:*)", "Bash(bd sync --full:*)", "Bash(bd config set:*)", "Bash(bd dep relate:*)"},
	{"Bash(security:*)", "Bash(pkill:*)", "Bash(killall:*)"},
	{"Edit", "Write", "Bash(git push:*)"},
	{"Bash(nosuchtool danger:*)"},
}

// spShims renders one persona's gates and returns every shim in its bin dir.
func spShims(t *testing.T, a *App, persona string, deny []string) map[string]string {
	t.Helper()
	_, binDir, _, err := a.RenderGates(persona, deny)
	if err != nil {
		t.Fatalf("RenderGates(%s, %v): %v", persona, deny, err)
	}
	ents, err := os.ReadDir(binDir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(binDir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[p] = string(b)
	}
	if len(out) == 0 {
		t.Fatalf("no shim was rendered into %s for deny %v — this pin would be green over nothing", binDir, deny)
	}
	return out
}

func TestQAEveryRenderedShimParsesAndCarriesNoBash32SubstitutionHazard(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on this box")
	}
	// The ceiling is config, not a deny, so one render carries it: without it
	// renderDataCeilingArgvGuard's arm is in no shim this pin reads.
	home := t.TempDir()
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte(qaCeilingCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), ConfigPath: cfg}
	if set := a.OpsPatternSet(); len(set.Ceiling) == 0 {
		t.Fatal("fixture premise: the ceiling is empty, so no shim here carries its argv guard")
	}
	// Two PATHs: one where every shimmed command resolves to a real binary
	// (which is what puts renderSeatObjectEnv and the hook-skip arm's execs
	// in the git shim at all), and one where nothing does — the shim's
	// run-time PATH search tail, and the renderers' `real == ""` branches.
	real := t.TempDir()
	for _, c := range []string{"git", "bd", "security", "date", "posse", "nosuchtool"} {
		WriteExecutable(filepath.Join(real, c), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	}
	if g, err := exec.LookPath("git"); err == nil {
		// A real git for the one renderer that ASKS one (the hook-skip arm's
		// global-scope reference execs `git var`), so the rendered text is
		// the text a seat gets and not the text a stub's silence produces.
		b, err := os.ReadFile(g)
		if err == nil {
			WriteExecutable(filepath.Join(real, "git"), b, 0o755)
		}
	}
	empty := t.TempDir()

	scanned := 0
	for _, pathDir := range []string{real, empty} {
		t.Setenv("PATH", pathDir)
		for i, deny := range spDenyMatrix {
			persona := fmt.Sprintf("developer%d", i)
			for p, body := range spShims(t, a, persona, deny) {
				scanned++
				// Half 1: it parses under this box's /bin/sh. On darwin that
				// is bash 3.2, which is where this class is an error.
				if out, err := exec.Command(sh, "-n", p).CombinedOutput(); err != nil {
					t.Errorf("the rendered shim %s does not parse under %s (deny %v, PATH %s): %v\n%s",
						filepath.Base(p), sh, deny, pathDir, err, out)
				}
				// Half 2: and it carries no shape a bash 3.2 would refuse,
				// which is the half that answers on a linux box.
				found, unreadable := b32CaseArmsInSubstitution(body)
				if unreadable != "" {
					t.Errorf("the rendered shim %s (deny %v) could not be read for the bash 3.2 substitution rule, so it is not measured: %s",
						filepath.Base(p), deny, unreadable)
					continue
				}
				for _, f := range found {
					t.Errorf("the rendered shim %s (deny %v) carries a shape darwin's /bin/sh refuses — %s\n"+
						"  put the walk in a function the substitution CALLS (renderSeatObjectEnv, ranger-base-i36bf), or give every arm the POSIX leading paren `(pattern)` (renderHookSkipGlobalScope)",
						filepath.Base(p), deny, f)
				}
			}
		}
	}
	if scanned < 2*len(spDenyMatrix) {
		t.Fatalf("only %d shims were read across %d deny sets and two PATHs — the matrix measured less than one shim per render", scanned, len(spDenyMatrix))
	}
}
