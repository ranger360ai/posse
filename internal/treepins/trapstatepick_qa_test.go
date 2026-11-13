package treepins

// QA pin for ranger-base-wzo36 — the variable an EXIT trap reads is assigned
// in the shell that owns the trap, never inside a subshell.
//
// THE DEFECT, and why prose did not hold it. scripts/verify-bd-dep-safety.sh
// and scripts/prune-bd-relates-to.sh read the beads db through a `q` helper
// that picked its reader lazily — open the store `mode=ro` if sqlite3 can,
// else copy the db to a `mktemp -d` and read the copy, with an EXIT trap to
// give the copy back. The trap gave nothing back. EVERY caller of `q` is a
// command substitution, and a `$( … )` is a subshell, so `pick_reader` ran in
// a child: the scratch path and the memo were assigned there and died with
// it. The parent's trap then read an empty variable and removed nothing, and
// because the memo did not survive either, the NEXT query picked again — one
// leaked copy of a 69M db per QUERY, not per run. ranger-base-m3fl5 fixed it
// by hoisting the pick into the trap's own shell and making `q` refuse rather
// than pick.
//
// What that fix left behind is a rule written in prose, in two scripts, with
// nothing under it. MEASURED under ranger-base-ltfq7, 2026-11-12, on this
// box at tree 9aaef0d4: with the pick moved back into `q` — the shape at
// 197ea0b8^ — `make verify-shell-syntax`, `make verify-bd-dep-safety`, `make
// verify-bd-no-relate-pairs`, `make prune-bd-relates-to` and `make
// tree-check` were ALL green, while a fallback run leaked one copy per query
// again:
//
//	verify-bd-dep-safety (3 queries)   picks 3->1   leftovers 3->0
//	verify-bd-dep-safety --gate        picks 1->1   leftovers 1->0
//	verify-bd-dep-safety <target>      picks 1->1   leftovers 1->0
//	prune-bd-relates-to (dry run)      picks 1->1   leftovers 1->0
//	prune-bd-relates-to --apply        picks 2->2   leftovers 2->0
//
// So the fix could be undone by the next edit as silently as the defect was
// made. The two scripts say the rule themselves —
// scripts/prune-bd-relates-to.sh:161, "THE PICK HAPPENS IN THIS SHELL, NEVER
// INSIDE A `$(…)`" — and a sentence in a file is not a gate.
//
// THE RULE, stated for the class rather than for those two scripts. In a
// script under scripts/ that registers an EXIT trap, every assignment to a
// variable that trap READS happens in the shell that registered it. A
// function that makes such an assignment is a trap-state SETTER, and a setter
// must be reachable only from that shell: never called from inside a
// `$( … )`, a backtick, a `( … )` subshell, a pipeline component or a
// background job, and never called from another function that is itself only
// reached that way. The rule is TRANSITIVE because the defect was: at
// 197ea0b8^ `pick_reader` was called from `q`, at the top of `q`'s body and
// at no depth at all, and it was `q` that only ever ran inside a `$( … )`.
// A rule that looked only at the setter's own call sites would have called
// that shape clean.
//
// IT IS NOT "a trap that removes a scratch copy", though that is the member
// that cost 206M of a shared tmpfs. Any variable the handler reads is the
// same bug: a pid list the handler kills, a stage name it reports, a path it
// restores. The handler is reading what nothing in the process that owns it
// ever assigned, which is the defect whatever the variable is for.
//
// MEASURED 2026-11-13 (linux 7.2.15 x86_64, go1.27.3, this box; the readings
// and who took them are in docs/notes.d/ranger-base-wzo36.md, which the crew
// census does not scan):
// 64 files under scripts/, 48 of them shell, 25 of those registering an EXIT
// trap, 46 handler-read variables between them, 426 shell functions censused
// (55 of them nested inside another) and 16 trap-state setters — among them
// reap-test-tmp.sh's `start_fake` and `stop_fake`, which are nested in its
// self_test and both assign the pid list its trap kills. TWO of the sixteen
// are reached in a subshell and both are on tsExempt below with their
// reasons — and they are the two
// halves of this rule: `watch_once` by its own call site, `drop_pid` only
// through `watch_once`, which is the shape 197ea0b8^ had. The two scripts the
// rule was written for hold 4 setters between them (`pick_reader` and
// `cleanup_ro` in each) and all four are clean.
//
// THE READER IS A LEXER AND NOT A PARSE, and loose in the direction that
// costs a false RED rather than a false pass — brewguard_qa_test.go's rule
// one file over, for the same reason: a false FAIL names the line and a
// reader can argue with it, while a false pass is the next bead in the
// family. Three places it is deliberately loose:
//
//   - A variable the HANDLER FUNCTION merely names is read as a trap-read
//     variable, minus the names that function declares `local`. A handler
//     that mentions a name it does not remove costs an extra setter in the
//     census and nothing else, because only a TAINTED setter reds.
//   - A name in command position is read as a call. A `case` pattern
//     (`selftest) selftest ;;`) scores its own pattern word as a call at
//     depth 0, which taints nothing.
//   - A logical line is the physical lines JOINED while a quote or a frame
//     stays open, so a `$( … )` spanning lines is seen. A trailing comment
//     carrying an unbalanced bracket joins lines that are not one statement;
//     that can only move a call site to a greater depth, so it reds and says
//     where. The join is capped (tsJoinCap) and a file that runs away is
//     reported in `lost` rather than read as though it had not.
//
// AND IT SHARES THE PACKAGE'S ONE SHELL STATE MACHINE. sfWalk
// (selftestforkarm_qa_test.go) already told a `(` inside a quoted string from
// a real one, to answer a different question; ranger-base-wzo36 gave it a
// third return — the open frame count — rather than writing a second walk, so
// a repair to the nesting reaches both readers and they cannot come to
// disagree about what a subshell is. Heredoc openers are bgHeredoc's
// (brewguard) and the quoted-opener probe is sfQuoted's, the same way
// mktemproot_qa_test.go borrows them.
//
// WHAT IT DOES NOT REACH, said once so nobody has to rediscover it:
//
//   - `eval`, and a function name reached through a variable
//     (`$handler args`). Both are a call this reader cannot see; a setter
//     invoked that way is invisible to the census, not scored clean by it.
//   - A subshell made by anything but a bracket, a pipe or a `&` — a `coproc`,
//     or `set -e` arithmetic. None appears under scripts/ today.
//   - The FIRST of two definitions of one name. gotest.sh defines
//     `sha256_file` and three `stat_*` helpers twice, once per branch of a
//     portability `if`; the reader keeps the last and the lines of the first
//     fall to whatever encloses them. Both branches assign nothing a trap
//     reads (MEASURED 2026-11-13), and the honest repair if one ever does is
//     to key functions by definition rather than by name.
//   - The shell that owns the trap, when a trap is REGISTERED inside a
//     function that itself runs in a subshell. The handler and the setter
//     would then be consistent with each other and both wrong; the census
//     reads the registration, never where the registration ran.
//   - Anything outside scripts/. The subject is the directory the defect was
//     found in twice, which is also `make scripts-check`'s subject; plugin/
//     holds one script and registers no EXIT trap (MEASURED 2026-11-13).

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// tsJoinCap — how many physical lines one logical line may swallow before the
// reader calls the join lost. The longest real one under scripts/ is
// reap-test-tmp.sh's `found=$( … )` sweep at 65 lines (MEASURED 2026-11-13);
// the cap is three times that, and its only job is to stop a construct this
// reader cannot close from eating a file in silence.
const tsJoinCap = 200

// tsFuncDef matches a function definition: `name() {`, `name() (` and the
// `function name() {` spelling. The second bracket is captured because a
// body written as `( … )` IS a subshell — every assignment in it dies with
// the call, whoever calls it and from where (brewguard's ranger-base-80zqb
// found that shape being used to hide an export from a static reader).
var tsFuncDef = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*([{(])`)

// tsExitWord matches the signal this pin's subject is named on. A `trap …
// INT TERM` with no EXIT is not trap state in the sense that matters: the
// shell is going down either way, and what the rule is about is the handler
// that runs on the ORDINARY exit and gives something back.
var tsExitWord = regexp.MustCompile(`\bEXIT\b`)

// tsVarRef matches a variable the handler READS: `$NAME`, `${NAME`,
// `${NAME:?}`. Positional and special parameters are not names.
var tsVarRef = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)`)

// tsLocalWords — the declarations that make the names after them local to the
// function, which is what says the trap can NEVER read them, so an assignment
// to one is not an assignment to trap state. `declare -g` declares a global
// and is dropped by the flag test in tsLocalNames rather than by this list.
var tsLocalWords = map[string]bool{"local": true, "typeset": true, "declare": true}

// tsAssignWord matches the `NAME=` or `NAME+=` of an assignment, and
// `NAME[i]=` for an array element. The command-position test below is what
// makes it an assignment rather than an `=` inside an argument.
var tsAssignWord = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)(?:\[[^\]]*\])?\+?=`)

// tsLogical is one logical line: the physical lines joined while a quote or a
// `$( … )`/backtick/`( … )` frame stays open. Line is where it starts, 1-based.
// Text is the masked, comment-stripped text the lexer works on; Raw is the
// same bytes unmasked, at the same offsets, which is where a variable NAME
// is read from — tsMaskParamExp neutralises the inside of `${ … }`, and
// `${WORK:?}` is exactly where a trap handler names what it removes.
type tsLogical struct {
	Line int
	Text string
	Raw  string
}

// At maps an offset inside Text back to a physical line.
func (l tsLogical) At(off int) int {
	if off > len(l.Text) {
		off = len(l.Text)
	}
	return l.Line + strings.Count(l.Text[:off], "\n")
}

// tsFunc is one function the reader found.
type tsFunc struct {
	Name       string
	Start, End int             // physical lines, 1-based and inclusive
	Subshell   bool            // the body is `( … )`, so it is a subshell itself
	Locals     map[string]bool // names it declares local
	Assigns    map[string]int  // global name -> the line it is assigned on
}

// tsSite is one place a function name appears in command position.
type tsSite struct {
	Line  int
	Depth int    // open `$( … )`/backtick/`( … )` frames around it
	Piped bool   // it is a component of a pipeline, or a background job
	In    string // the function whose body it sits in, "" at top level
	Name  string // the variable, for an assignment site
	Code  string
}

// At renders the site the way a diagnostic leads with it.
func (s tsSite) At(script string) string { return fmt.Sprintf("%s:%d", script, s.Line) }

// Subshell reports whether the call ran in a child of the shell that reached
// it. A bracket frame and a pipeline component are the same answer: the
// assignments made in there do not come back.
func (s tsSite) Subshell() bool { return s.Depth > 0 || s.Piped }

// tsIsShell reports whether this file is shell at all. scripts/ holds Python
// censuses, a `.allow` roster whose lines are English, and markdown; a shell
// lexer over any of them reports a file it could not read, which would make
// the honest `lost` arm below useless noise. The test is the extension or the
// shebang, the way the box's own roster reads it.
func tsIsShell(path string, body []byte) bool {
	if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".bash") {
		return true
	}
	first := string(body)
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	return strings.HasPrefix(first, "#!") && (strings.Contains(first, "sh") && !strings.Contains(first, "python"))
}

// tsScript is one script's reading.
type tsScript struct {
	Path     string
	Handlers []string        // what each `trap <h> … EXIT` named
	ReadVars map[string]bool // the names those handlers read
	Funcs    map[string]*tsFunc
	Order    []string            // function names, in file order
	Calls    map[string][]tsSite // function name -> its call sites
	Setters  []string            // functions that assign a trap-read name
	Tainted  map[string]tsSite   // setter or caller -> the site that taints it
	Loose    []tsSite            // top-level assignments to trap state inside a subshell
	Shell    bool                // it is shell at all; nothing below is read when it is not
	Lost     string              // where the reader lost the lexing, if it did
	Lines    []string            // the file as written, for diagnostics
}

// tsMaskParamExp replaces the INSIDE of every `${ … }` with a neutral byte,
// keeping the braces, the length and every newline — so an offset into the
// masked text is the same offset into the original, and a diagnostic can
// print the real line.
//
// WHY, and why here rather than in sfWalk. A parameter expansion carries its
// own quoting rules, and a double-quoted one carries rules that are not the
// enclosing string's:
//
//	printf '%s' "${rest%'"'*}"      scripts/verify-detection.sh:182
//
// is a valid trim whose pattern is a quoted `"`. To a lexer that knows only
// strings, that inner `"` CLOSES the enclosing one, the `"` after `}` opens a
// new one, and everything to the end of the file is a string: MEASURED
// 2026-11-13, that one line made this reader swallow 54 lines and report
// scripts/verify-detection.sh as a file it could not read, and
// scripts/verify-herdr-bob-rules.sh has the same shape. The content of an
// expansion is never a command position, never an assignment and never a
// call, so masking it loses this census nothing.
//
// It is masked HERE and not taught to sfWalk because sfWalk answers a
// question for three other readers whose subject is what is inside a quoted
// string; widening it would change their answers to serve mine.
func tsMaskParamExp(text string) string {
	out := []byte(text)
	for i := 0; i+1 < len(out); i++ {
		if out[i] != '$' || out[i+1] != '{' {
			continue
		}
		depth, j := 0, i+1
		for ; j < len(out); j++ {
			if out[j] == '{' {
				depth++
			} else if out[j] == '}' {
				if depth--; depth == 0 {
					break
				}
			}
		}
		if j >= len(out) {
			continue // nothing closes it; leave the text alone
		}
		for k := i + 2; k < j; k++ {
			if out[k] != '\n' {
				out[k] = 'x'
			}
		}
		i = j
	}
	return string(out)
}

// tsStripComment cuts a logical line at its first unquoted `#` that starts a
// word, counting from `after` — the offset the last physical line was
// appended at, so the quote state carried in from the lines above is the
// state the scan starts from.
//
// IT IS NOT A TIDINESS PASS, it is what keeps the lexer from running away.
// MEASURED 2026-11-13: without it, scripts/verify-pid-deny-set.sh's comment
// "a pattern kill matches every seat's identical argv" opened a single quote
// that nothing closed, the join ate the next 60 lines and the reader reported
// the whole file lost; scripts/verify-herdr-bob-rules.sh ("the dialog's top
// rule") and scripts/verify-detection.sh did the same, three files of the
// seventy-six, and every apostrophe in prose is the same trap. A comment is
// not code and the shell never reads one, so cutting it is the reading and
// not a convenience.
//
// `${x#y}`, `${#arr[@]}` and `$#` are not comments: the `#` in each is
// preceded by a byte that is not a word break. That is also why the test is
// the PRECEDING byte and not the `#` alone.
// It returns the OFFSET to cut at rather than the cut text, because the
// masked text and the raw text are cut at the same place and must stay the
// same length (see tsLogical).
func tsCommentCut(text string, after int) int {
	for i := after; i < len(text); i++ {
		if text[i] != '#' {
			continue
		}
		if i > 0 {
			switch text[i-1] {
			case ' ', '\t', '\n', ';', '&', '|', '(', '`':
			default:
				continue
			}
		}
		if quote, _, _ := sfWalk(text, 0, i); quote != 0 {
			continue
		}
		return i
	}
	return len(text)
}

// tsReadLines splits a file into logical lines, dropping comment lines,
// blank lines and heredoc BODIES.
//
// A heredoc body is data this script hands to something else, so nothing in
// it is a call, an assignment or structure. The opener is bgHeredoc's and the
// "is the opener itself inside a quoted string" probe is sfQuoted's — a
// phantom heredoc would swallow real lines, which is this reader's one loose
// direction that could go quiet, so it is the one borrowed with a guard.
func tsReadLines(masked, raw []string) (out []tsLogical, lost string) {
	hereTerm, hereDash := "", false
	for i := 0; i < len(masked); i++ {
		line := raw[i]
		code := strings.TrimSpace(line)
		if hereTerm != "" {
			if line == hereTerm || (hereDash && code == hereTerm) {
				hereTerm = ""
			}
			continue
		}
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		start := i
		text, rawText := masked[i], raw[i]
		if cut := tsCommentCut(text, 0); cut < len(text) {
			text, rawText = text[:cut], rawText[:cut]
		}
		for n := 0; ; n++ {
			quote, _, depth := sfWalk(text, 0, len(text))
			if quote == 0 && depth == 0 {
				break
			}
			if i+1 >= len(masked) || n >= tsJoinCap {
				if lost == "" {
					lost = fmt.Sprintf("%d: %q", start+1, strings.TrimSpace(raw[start]))
				}
				break
			}
			i++
			at := len(text) + 1
			text, rawText = text+"\n"+masked[i], rawText+"\n"+raw[i]
			if cut := tsCommentCut(text, at); cut < len(text) {
				text, rawText = text[:cut], rawText[:cut]
			}
		}
		out = append(out, tsLogical{Line: start + 1, Text: text, Raw: rawText})
		if m := bgHeredoc.FindStringSubmatchIndex(text); m != nil && !sfQuoted(text[:m[0]]) {
			hereTerm, hereDash = rawText[m[4]:m[5]], rawText[m[2]:m[3]] == "-"
		}
	}
	return out, lost
}

// tsBraceDelta counts the block braces on a logical line — the ones outside
// any quoted string. `${VAR}` contributes both of its braces or neither, so
// it nets to zero either way.
func tsBraceDelta(text string) int {
	delta := 0
	for i := 0; i < len(text); i++ {
		if text[i] != '{' && text[i] != '}' {
			continue
		}
		if quote, _, _ := sfWalk(text, 0, i); quote != 0 {
			continue
		}
		if text[i] == '{' {
			delta++
		} else {
			delta--
		}
	}
	return delta
}

// tsStatementEnd returns the offset where the statement starting at `at`
// ends: the next unquoted `;`, newline, `|`, `&` or `}` at the same frame
// depth, the `)` that closes the frame the statement is IN, or the end of
// the logical line.
//
// The closing `)` is a terminator and not a word, which is the difference
// between `( WORK=$(mktemp -d) )` — an assignment in a subshell, this pin's
// own loose arm — and `WORK=x cmd`, an environment prefix on a command. Read
// without it the first is a prefix whose command is `)`.
func tsStatementEnd(text string, at int) int {
	_, _, depth := sfWalk(text, 0, at)
	for i := at; i < len(text); i++ {
		switch text[i] {
		case ';', '\n', '|', '&', '}', ')':
		default:
			continue
		}
		if q, _, d := sfWalk(text, 0, i); q == 0 && d == depth {
			return i
		}
	}
	return len(text)
}

// tsLocalNames returns the names a logical line declares local. The
// declaration is read wherever it sits in command position — `set_it() {
// local tmp; tmp=$(…); }` is one line and is how half this tree spells a
// short helper, so a rule anchored at the start of a line would miss it and
// then read `tmp` as trap state.
func tsLocalNames(text string) []string {
	var out []string
	for off := 0; off < len(text); off++ {
		if !isTsNameStart(text, off) || !tsCommandWord(text, off) {
			continue
		}
		end := off
		for end < len(text) && isTsNameByte(text[end]) {
			end++
		}
		if !tsLocalWords[text[off:end]] {
			continue
		}
		fields := strings.Fields(text[end:tsStatementEnd(text, off)])
		global := false
		for _, w := range fields {
			if strings.HasPrefix(w, "-") {
				global = global || strings.ContainsRune(strings.TrimLeft(w, "-"), 'g')
				continue
			}
			if global {
				break
			}
			if n := strings.SplitN(w, "=", 2)[0]; n != "" && tsAssignWord.MatchString(n+"=") {
				out = append(out, n)
			}
		}
	}
	return out
}

// tsIsEnvPrefix reports whether the assignment starting at `at` is an
// environment prefix on a command — `GIT_COMMITTER_DATE=… git commit …` —
// rather than an assignment to this shell. The shell puts a prefixed name in
// the CHILD's environment and never in its own variables, so a trap in this
// shell could not read it whatever the name is, and counting one would red
// eight real lines under scripts/ that are doing exactly the right thing
// (MEASURED 2026-11-13: verify-herdr-bob.sh, verify-hook-freshness.sh,
// verify-id-recycle.sh, verify-prune-guard.sh, release-artifacts.sh,
// queue-cutover.sh).
//
// A word follows the assignment, inside the same statement and at the same
// frame depth, exactly when it is a prefix: the spaces inside a `"…"` or a
// `$( … )` are at a quote or a depth of their own and are not word breaks.
func tsIsEnvPrefix(text string, at int) bool {
	_, _, depth := sfWalk(text, 0, at)
	end := tsStatementEnd(text, at)
	i := at
	for ; i < end; i++ {
		if text[i] != ' ' && text[i] != '\t' {
			continue
		}
		if q, _, d := sfWalk(text, 0, i); q == 0 && d == depth {
			break
		}
	}
	rest := strings.TrimSpace(text[min(i, end):end])
	return rest != "" && !strings.HasPrefix(rest, "\\")
}

// tsCommandWord reports whether the name starting at `at` is in command
// position on this logical line: outside every quoted string, and with
// nothing before it since the last separator. `export NAME=` and `! cmd`
// are command positions; `local NAME=` is NOT, because the name it declares
// is not trap state.
func tsCommandWord(text string, at int) bool {
	if quote, _, _ := sfWalk(text, 0, at); quote != 0 {
		return false
	}
	prefix := strings.TrimRight(text[:at], " \t")
	if prefix == "" {
		return true
	}
	switch prefix[len(prefix)-1] {
	case ';', '&', '|', '(', '`', '\n', '{', '!':
		return true
	}
	for _, w := range []string{"then", "else", "elif", "do", "in", "export", "readonly"} {
		if strings.HasSuffix(prefix, w) && (len(prefix) == len(w) || isTsBoundary(prefix[len(prefix)-len(w)-1])) {
			return true
		}
	}
	return false
}

func isTsBoundary(c byte) bool {
	return c == ' ' || c == '\t' || c == ';' || c == '&' || c == '|' || c == '(' || c == '\n'
}

// tsPiped reports whether the command starting at `at` is a pipeline
// component or a background job at this frame depth — both of which bash runs
// in a child, so an assignment made there is lost exactly the way one made
// inside a `$( … )` is.
//
// It reads the separator IMMEDIATELY on either side of the command and not
// "is there a `|` on this line". `a | b && set_it x` has a pipe on it and
// `set_it` is not in that pipeline; `[ -z "$X" ] || set_it x` has a `|` on it
// twice over and is an ordinary conditional call. The nearest separator is
// what says which command this is, and `||` and `&&` are read before `|` and
// `&` so the doubled spellings never answer for the single ones.
func tsPiped(text string, at int) bool {
	_, _, depth := sfWalk(text, 0, at)
	for i := at - 1; i >= 0; i-- {
		c := text[i]
		if c == ' ' || c == '\t' {
			continue
		}
		if c != '|' {
			break // any other byte: a separator that is not a pipe, or a word
		}
		if q, _, d := sfWalk(text, 0, i); q != 0 || d != depth {
			break
		}
		return !(i > 0 && text[i-1] == '|') && !(i+1 < len(text) && text[i+1] == '|')
	}
	for i := at; i < len(text); i++ {
		switch text[i] {
		case ';', '\n', '|', '&', ')':
		default:
			continue
		}
		if q, _, d := sfWalk(text, 0, i); q != 0 || d != depth {
			continue
		}
		switch text[i] {
		case '|', '&':
			doubled := (i > 0 && text[i-1] == text[i]) || (i+1 < len(text) && text[i+1] == text[i])
			return !doubled
		}
		return false
	}
	return false
}

// tsRead reads one script and returns everything the rule is decided from.
// It is a plain function and not the assertion so the control below can run
// it over planted fixtures and read its answer — an assertion that only ever
// ran over a tree that passes is a census nobody has shown can say no.
func tsRead(path string) (*tsScript, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := &tsScript{
		Path:     path,
		ReadVars: map[string]bool{},
		Funcs:    map[string]*tsFunc{},
		Calls:    map[string][]tsSite{},
		Tainted:  map[string]tsSite{},
		Shell:    tsIsShell(path, b),
	}
	if !s.Shell {
		return s, nil
	}
	s.Lines = strings.Split(string(b), "\n")
	logical, lost := tsReadLines(strings.Split(tsMaskParamExp(string(b)), "\n"), s.Lines)
	s.Lost = lost

	// 1. functions, by brace count over the logical lines.
	for i := 0; i < len(logical); i++ {
		m := tsFuncDef.FindStringSubmatch(logical[i].Text)
		if m == nil {
			continue
		}
		f := &tsFunc{
			Name:    m[1],
			Start:   logical[i].Line,
			Locals:  map[string]bool{},
			Assigns: map[string]int{},
		}
		if m[2] == "(" {
			// The joiner already closed the body's parentheses, so a subshell
			// body is this one logical line.
			f.Subshell = true
			f.End = logical[i].At(len(logical[i].Text))
		} else {
			depth, j := tsBraceDelta(logical[i].Text), i
			for depth > 0 && j+1 < len(logical) {
				j++
				depth += tsBraceDelta(logical[j].Text)
			}
			if depth > 0 {
				if s.Lost == "" {
					s.Lost = fmt.Sprintf("%d: %s() never closes", f.Start, f.Name)
				}
				continue
			}
			f.End = logical[j].At(len(logical[j].Text))
			// Deliberately NOT `i = j`: a definition nested inside this
			// body is a function too, and skipping to the end of the outer
			// one loses it. MEASURED 2026-11-13: 55 indented definitions in
			// 8 files under scripts/, gotest.sh's twelve among them. Each
			// def's extent is computed from its own brace count, so finding
			// them is all the nesting costs, and tsSite.In then resolves to
			// the innermost body a call sits in.
		}
		if _, dup := s.Funcs[f.Name]; !dup {
			s.Order = append(s.Order, f.Name)
		}
		s.Funcs[f.Name] = f
	}

	// 2. what each function declares local, and what it assigns.
	for _, l := range logical {
		for _, f := range s.Funcs {
			if l.Line < f.Start || l.Line > f.End {
				continue
			}
			for _, n := range tsLocalNames(l.Text) {
				f.Locals[n] = true
			}
		}
	}
	for _, l := range logical {
		for off := 0; off < len(l.Text); off++ {
			if !isTsNameStart(l.Text, off) {
				continue
			}
			m := tsAssignWord.FindStringSubmatch(l.Text[off:])
			if m == nil || !tsCommandWord(l.Text, off) || tsIsEnvPrefix(l.Text, off) {
				continue
			}
			name, line := m[1], l.At(off)
			var inner *tsFunc
			for _, f := range s.Funcs {
				if line < f.Start || line > f.End {
					continue
				}
				if inner == nil || f.Start > inner.Start {
					inner = f
				}
			}
			if inner != nil {
				if !inner.Locals[name] {
					if _, seen := inner.Assigns[name]; !seen {
						inner.Assigns[name] = line
					}
				}
				continue
			}
			// Top level. An assignment made inside a bracket frame or a
			// pipeline component here is the same defect with no function in
			// the middle, so it is kept for its own arm — filtered to the
			// names a handler actually reads once the traps are known, which
			// is after this pass.
			_, _, depth := sfWalk(l.Text, 0, off)
			if depth > 0 || tsPiped(l.Text, off) {
				s.Loose = append(s.Loose, tsSite{Line: line, Depth: depth, Piped: tsPiped(l.Text, off), Name: name, Code: s.CodeAt(line)})
			}
		}
	}

	// 3. the EXIT traps and the names their handlers read.
	for _, l := range logical {
		code := strings.TrimSpace(l.Text)
		if !strings.HasPrefix(code, "trap ") || !tsExitWord.MatchString(code) {
			continue
		}
		// The handler is taken from the RAW text: a trap written inline is
		// `rm -rf -- "${RO_TMP:?}"`, and the names in it are exactly what
		// the mask neutralises.
		rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l.Raw), "trap "))
		handler := tsFirstWord(rest)
		if handler == "" || handler == "-" {
			continue
		}
		s.Handlers = append(s.Handlers, handler)
		body := handler
		if f, ok := s.Funcs[handler]; ok {
			var parts []string
			for _, h := range logical {
				if h.Line >= f.Start && h.Line <= f.End {
					parts = append(parts, h.Raw)
				}
			}
			body = strings.Join(parts, "\n")
			for _, m := range tsVarRef.FindAllStringSubmatch(body, -1) {
				if !f.Locals[m[1]] {
					s.ReadVars[m[1]] = true
				}
			}
			continue
		}
		for _, m := range tsVarRef.FindAllStringSubmatch(body, -1) {
			s.ReadVars[m[1]] = true
		}
	}
	if len(s.Handlers) == 0 {
		s.Loose = nil
		return s, nil
	}
	kept := s.Loose[:0]
	for _, site := range s.Loose {
		if s.ReadVars[site.Name] {
			kept = append(kept, site)
		}
	}
	s.Loose = kept

	// 4. the setters: a function that assigns a name a handler reads.
	for _, name := range s.Order {
		f := s.Funcs[name]
		for v := range f.Assigns {
			if s.ReadVars[v] {
				s.Setters = append(s.Setters, name)
				break
			}
		}
	}

	// 5. the call sites of every function, and the taint closure over them.
	for _, name := range s.Order {
		f := s.Funcs[name]
		word := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
		for _, l := range logical {
			if strings.HasPrefix(strings.TrimSpace(l.Text), "trap ") {
				continue // a registration is not a call
			}
			for _, loc := range word.FindAllStringIndex(l.Text, -1) {
				line := l.At(loc[0])
				if line >= f.Start && line <= f.End {
					continue // its own body
				}
				if !tsCommandWord(l.Text, loc[0]) {
					continue
				}
				_, _, depth := sfWalk(l.Text, 0, loc[0])
				site := tsSite{Line: line, Depth: depth, Piped: tsPiped(l.Text, loc[0]), Code: s.CodeAt(line)}
				for _, g := range s.Funcs {
					if line < g.Start || line > g.End {
						continue
					}
					if site.In == "" || s.Funcs[site.In].Start < g.Start {
						site.In = g.Name
					}
				}
				s.Calls[name] = append(s.Calls[name], site)
			}
		}
	}
	for _, name := range s.Order {
		if s.Funcs[name].Subshell {
			s.Tainted[name] = tsSite{Line: s.Funcs[name].Start, Code: name + "() ( … )"}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, name := range s.Order {
			if _, done := s.Tainted[name]; done {
				continue
			}
			for _, site := range s.Calls[name] {
				if _, up := s.Tainted[site.In]; site.Subshell() || (site.In != "" && up) {
					s.Tainted[name] = site
					changed = true
					break
				}
			}
		}
	}
	return s, nil
}

func isTsNameStart(text string, off int) bool {
	c := text[off]
	if !(c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		return false
	}
	return off == 0 || !isTsNameByte(text[off-1])
}

func isTsNameByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// tsFirstWord takes the handler argument off a `trap` line: a quoted string
// whole, or the first bare word.
func tsFirstWord(rest string) string {
	if rest == "" {
		return ""
	}
	if q := rest[0]; q == '\'' || q == '"' {
		if end := strings.IndexByte(rest[1:], q); end >= 0 {
			return rest[1 : 1+end]
		}
		return rest[1:]
	}
	return strings.Fields(rest)[0]
}

// CodeAt returns one physical line of the script as it is written, for a
// diagnostic that should print what a reader will find at that line and not
// the masked, comment-stripped, sixty-line join the lexer worked on.
func (s *tsScript) CodeAt(line int) string {
	if line >= 1 && line <= len(s.Lines) {
		return strings.TrimSpace(s.Lines[line-1])
	}
	return ""
}

// tsExempt — a trap-state setter the census finds in a subshell, with the
// reason it is there. Keyed by (script, function), the way mtExempt is keyed
// by what its row is ABOUT rather than by a line number that is stale on the
// next edit above it.
//
// An exemption is a claim that the trap reading nothing costs nothing, and
// that is a claim about the script, so it carries prose and not a flag.
var tsExempt = []struct{ Script, Func, Reason string }{
	{
		"scripts/verify-herdr-bob-rules.sh", "watch_once",
		"Called as `got=\"$(watch_once …)\"` twice, so its `WATCH_PIDS=\"$WATCH_PIDS $p\"` lands in the subshell and the parent's `cleanup` has an empty list. It costs nothing on the ordinary path: the same subshell kills AND `wait`s for that pid before it returns, so by the time the assignment is lost there is nothing for the trap to kill. What it does not cover is a run killed BETWEEN the `&` and the `kill` — the parent's trap then cannot name a bob-watch it never heard of, and removes $T out from under it. That window is the subject of ranger-base-myjcz; it is not fixed here because this script SKIPS on this box (herdr-bob is not installed) and a fix nothing here can execute is a fix nobody has run.",
	},
	{
		"scripts/verify-herdr-bob-rules.sh", "drop_pid",
		"Same call, one frame further in: `drop_pid` is called only from `watch_once`, so it inherits that subshell and its `WATCH_PIDS=$keep` dies with it too. Harmless for the same reason and open for the same one — it is removing a pid from a list the parent will never read. Filed with its caller under ranger-base-myjcz. This row is also what keeps the TRANSITIVE arm of the census live over the real tree: it is the only member outside the control fixtures whose taint comes from its caller rather than from its own call site, which is exactly the shape 197ea0b8^ had.",
	},
}

// tsRoots — the directory this census walks, as a LITERAL at the walk, because
// treewidedoor_qa_test.go derives the tree-wide class from an enumeration
// rooted at a relative path spelled in the test. A walk moved behind a
// parameter is a spelling that rule cannot see, and the pin would silently
// lose its door (mktemproot_qa_test.go's mtFileSet carries the measurement).
//
// Every trap-state setter under scripts/ is reachable only from the shell
// that owns the trap, or is on tsExempt with its reason. Two-way: a row the
// census no longer finds reds as stale, so neither the rule nor the roster
// can go quiet while staying green.
func TestQAEveryTrapStateSetterRunsInTheTrapsOwnShell(t *testing.T) {
	t.Parallel()
	walkRoots := []string{"scripts"}
	var walked []string
	for _, root := range []string{"scripts"} {
		if err := filepath.WalkDir(root, mtWalkInto(&walked)); err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	paths := mtFileSet(t, walkRoots, walked)

	type key struct{ script, fn string }
	exempt := map[key]string{}
	for _, e := range tsExempt {
		exempt[key{e.Script, e.Func}] = e.Reason
	}
	seen := map[key]bool{}
	shell, trapped, vars, funcs, setters := 0, 0, 0, 0, 0
	for _, path := range paths {
		s, err := tsRead(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if !s.Shell {
			continue
		}
		shell++
		funcs += len(s.Funcs)
		if s.Lost != "" {
			t.Errorf("%s: the reader lost the lexing at %s — it cannot say where a call runs in this file, and a census that cannot read a file must not report it clean. Fix the line (an unbalanced bracket in a trailing comment is the usual cause) or say why the reader cannot be expected to.", path, s.Lost)
		}
		if len(s.Handlers) == 0 {
			continue
		}
		trapped++
		vars += len(s.ReadVars)
		setters += len(s.Setters)
		for _, site := range s.Loose {
			t.Errorf("%s assigns trap state inside a subshell, at top level:\n  %s\n"+
				"The EXIT trap registered in this script reads that name in the shell this script started in, and an assignment made in a `$( … )`, a backtick, a `( … )` or a pipeline component dies with the child that made it. Hoist it.",
				site.At(path), site.Code)
		}
		for _, name := range s.Setters {
			site, bad := s.Tainted[name]
			k := key{path, name}
			if reason, ok := exempt[k]; ok {
				seen[k] = true
				if !bad {
					t.Errorf("tsExempt names %s() in %s and the census finds it reachable only from the trap's own shell — the row is an exemption over nothing, and it is also how this census would go blind without saying so", name, path)
				}
				if len(strings.TrimSpace(reason)) < 60 {
					t.Errorf("%s:%d: %s() is exempted from the trap-state rule with no reason worth reading (%q) — an exemption nobody can check is the rule being dropped quietly", path, s.Funcs[name].Start, name, reason)
				}
				continue
			}
			if !bad {
				continue
			}
			var sets []string
			for v := range s.Funcs[name].Assigns {
				if s.ReadVars[v] {
					sets = append(sets, v)
				}
			}
			sort.Strings(sets)
			t.Errorf("%s:%d: %s() assigns %s, which this script's EXIT trap reads — and it is reached in a subshell:\n"+
				"  %s\n"+
				"  %s\n"+
				"A `$( … )`, a backtick, a `( … )` and a pipeline component are all children: the assignment is made there and dies there, the trap in the parent then reads a name nothing it owns ever set, and whatever the handler was going to give back is not given back. Call it from the shell that registered the trap — unconditionally, before the first reader — or put a row in tsExempt (%s) saying why the trap reading nothing costs nothing here.\n"+
				"This is ranger-base-m3fl5, which leaked one copy of a 69M beads db per QUERY while five doors stayed green, and ranger-base-wzo36, which is this pin.",
				path, s.Funcs[name].Start, name, strings.Join(sets, ", "),
				site.At(path)+": "+site.Code,
				tsWhy(s, name, site),
				"internal/treepins/trapstatepick_qa_test.go")
		}
	}
	// Floors, so a reader that stopped reading cannot pass quietly. An
	// assertion that every setter is clean is satisfied by a census that
	// found none, and a census that found none says nothing about it.
	// MEASURED 2026-11-13 (see the head comment): 48 shell files of the 64
	// under scripts/, 25 with an EXIT trap, 46 handler-read names, 426
	// functions, 16 setters. Each floor sits well under its reading, so an
	// ordinary landing does not move it.
	if shell < 40 || trapped < 18 || vars < 32 || funcs < 300 || setters < 10 {
		t.Fatalf("the census found %d shell file(s), %d with an EXIT trap, %d handler-read name(s), %d function(s) and %d trap-state setter(s) — the reader is broken, not the tree (MEASURED 2026-11-13: see the head comment)", shell, trapped, vars, funcs, setters)
	}
	for _, e := range tsExempt {
		if !seen[key{e.Script, e.Func}] {
			t.Errorf("tsExempt names %s() in %s and the census finds no such trap-state setter there — a stale roster row is an exemption over nothing, and over a population this small it is also the only way this census can say it has gone blind", e.Func, e.Script)
		}
	}
}

// tsWhy renders the second line of a diagnostic: WHY the setter is tainted,
// which is the half 197ea0b8^ would have been called clean without. A setter
// whose own call site is in a subshell is one sentence; one tainted through
// its caller is the sentence the transitive rule exists for.
func tsWhy(s *tsScript, name string, site tsSite) string {
	if s.Funcs[name].Subshell {
		return "its BODY is a `( … )` subshell, so every assignment in it dies with the call whoever makes it"
	}
	if site.Subshell() {
		if site.Piped && site.Depth == 0 {
			return "that call is a pipeline component or a background job, which bash runs in a child"
		}
		return "that call sits inside a `$( … )`, a backtick or a `( … )`"
	}
	chain := []string{name}
	for cur := site.In; cur != ""; {
		chain = append(chain, cur)
		next, ok := s.Tainted[cur]
		if !ok {
			break
		}
		if next.Subshell() {
			return fmt.Sprintf("reached only through %s, whose own call at %s:%d is in a subshell — the chain is %s", cur, s.Path, next.Line, strings.Join(chain, " <- "))
		}
		cur = next.In
	}
	return fmt.Sprintf("reached only through %s", strings.Join(chain[1:], " <- "))
}

// The reader and the rule over planted scripts, because the one thing this
// bead was asked for is that the pin reds on the shape at 197ea0b8^ — and a
// rule that has only ever run over a tree that passes has not been shown to
// say no. The fixtures are absolute paths under t.TempDir(), so this arm
// enumerates nothing and is not in the tree-wide class.
func TestQATheTrapStateReaderCanStillSayNo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	read := func(name, body string) *tsScript {
		t.Helper()
		s, err := tsRead(write(name, body))
		if err != nil {
			t.Fatal(err)
		}
		if s.Lost != "" {
			t.Errorf("%s: reader lost the lexing at %s", name, s.Lost)
		}
		return s
	}
	tainted := func(s *tsScript) []string {
		var out []string
		for _, n := range s.Setters {
			if _, bad := s.Tainted[n]; bad {
				out = append(out, n)
			}
		}
		sort.Strings(out)
		return out
	}

	// THE DEFECT ITSELF, as scripts/verify-bd-dep-safety.sh spelled it at
	// 197ea0b8^: `q` picks lazily, and every caller of `q` is a `$( … )`.
	// Nothing here is at a depth of its own — `pick_reader` is called at the
	// top of `q`'s body — so this is the shape a non-transitive rule calls
	// clean.
	before := read("before.sh", strings.Join([]string{
		"#!/usr/bin/env bash",
		`RO_TMPS=""`,
		`cleanup_ro() { [ -z "$RO_TMPS" ] || rm -rf $RO_TMPS; }`,
		"trap cleanup_ro EXIT",
		`DB_READ=""`,
		"pick_reader() {",
		`	local tmp`,
		`	tmp=$(mktemp -d "${TMPDIR:-/tmp}/posse-x.XXXXXX") || exit 2`,
		`	RO_TMPS="$RO_TMPS $tmp"`,
		`	DB_READ="$tmp/beads.db"`,
		"}",
		"q() {",
		`	[ -n "$DB_READ" ] || pick_reader`,
		`	sqlite3 "$DB_READ" "$1"`,
		"}",
		`pairs=$(q "SELECT 1")`,
		`echo "$pairs"`,
		"",
	}, "\n"))
	if got, want := before.Setters, []string{"pick_reader"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the 197ea0b8^ shape has setters %v, want %v — RO_TMPS is what the trap reads, `pick_reader` is the one function that assigns it, and `cleanup_ro` only reads it (at HEAD it assigns it too, which is why the live census finds two in each of these scripts)", got, want)
	}
	if got := tainted(before); len(got) != 1 || got[0] != "pick_reader" {
		t.Errorf("the 197ea0b8^ shape reports tainted setters %v, want [pick_reader] — this is the bead's own done-when: the pick moved back inside `q`, whose every caller is a command substitution. A rule that reads only pick_reader's OWN call site sees a call at depth 0 and says nothing, which is how the defect was made and how five doors stayed green over it.", got)
	}
	if site := before.Tainted["pick_reader"]; site.In != "q" || site.Subshell() {
		t.Errorf("pick_reader is tainted by %+v, want the call inside q()'s body (In=q) at no depth of its own — the taint has to arrive through the CALLER or the diagnostic will name the wrong line", site)
	}

	// AND THE FIX, which is HEAD: the pick is hoisted into the shell that
	// owns the trap and `q` refuses rather than picks.
	after := read("after.sh", strings.Join([]string{
		"#!/usr/bin/env bash",
		`RO_TMP=""`,
		"cleanup_ro() {",
		`	[ -z "$RO_TMP" ] || rm -rf -- "${RO_TMP:?}"`,
		`	RO_TMP=""`,
		"}",
		"trap cleanup_ro EXIT",
		`DB_READ=""`,
		"pick_reader() {",
		"	cleanup_ro",
		`	local tmp`,
		`	tmp=$(mktemp -d "${TMPDIR:-/tmp}/posse-x.XXXXXX") || exit 2`,
		`	RO_TMP=$tmp`,
		`	DB_READ="$tmp/beads.db"`,
		"}",
		"q() {",
		`	[ -n "$DB_READ" ] || { echo "internal: q ran before pick_reader" >&2; exit 2; }`,
		`	sqlite3 "$DB_READ" "$1"`,
		"}",
		"pick_reader",
		`pairs=$(q "SELECT 1")`,
		`echo "$pairs"`,
		"",
	}, "\n"))
	if got := tainted(after); len(got) != 0 {
		t.Errorf("the HEAD shape reports tainted setters %v, want none — the pick is called at top level, in the shell that registered the trap, and `q` no longer picks at all", got)
	}

	// Every other way the same assignment can end up in a child. One script,
	// one setter per spelling, so a reader that stops seeing one of them is
	// named by the row it dropped and not by a count.
	for _, tc := range []struct {
		name, body string
		want       string
	}{
		{"sub.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nx=$(set_it /tmp/a)\n", "a call inside a `$( … )`"},
		{"tick.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nx=`set_it /tmp/a`\n", "a call inside a backtick"},
		{"paren.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\n( set_it /tmp/a )\n", "a call inside a `( … )` subshell"},
		{"pipe.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nset_it /tmp/a | cat\n", "a call piped into something"},
		{"bg.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nset_it /tmp/a &\n", "a backgrounded call"},
		{"body.sh", "set_it() (\n	WORK=$1\n)\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nset_it /tmp/a\n", "a function whose BODY is a subshell"},
		{"multi.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nx=$(\n	set_it /tmp/a\n)\n", "a `$( … )` that spans lines"},
		{"deep.sh", "set_it() { WORK=$1; }\nmid() { set_it \"$1\"; }\nouter() { mid \"$1\"; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nx=$(outer /tmp/a)\n", "a call three functions deep"},
	} {
		s := read(tc.name, tc.body)
		if got := tainted(s); len(got) == 0 {
			t.Errorf("%s: %s is reported clean, and it is a child — calls=%v setters=%v", tc.name, tc.want, s.Calls, s.Setters)
		}
	}

	// And what it must NOT call a violation. Each of these is a shape the
	// tree holds today, and a false red on one is a reader a seat learns to
	// route around.
	for _, tc := range []struct {
		name, body string
		why        string
	}{
		{"top.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nset_it /tmp/a\n", "a plain top-level call"},
		{"or.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\n[ -n \"$WORK\" ] || set_it /tmp/a\n", "`||` is not a pipe"},
		{"and.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\n[ -z \"$WORK\" ] && set_it /tmp/a\n", "`&&` is not a background job"},
		{"afterpipe.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nls | grep -q x && set_it /tmp/a\n", "a pipe earlier in the line is somebody else's pipeline"},
		{"beforepipe.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nset_it /tmp/a; ls | cat\n", "a pipe LATER in the line, past a `;`, is somebody else's too"},
		{"case.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\ncase ${1:-} in\n	set_it) set_it /tmp/a ;;\nesac\n", "a case pattern that happens to be the name"},
		{"arg.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nset_it /tmp/a\necho \"$(grep -c set_it \"$0\")\"\n", "the name as an ARGUMENT inside a `$( … )` is not a call"},
		{"local.sh", "set_it() { local WORK; WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nx=$(set_it /tmp/a)\n", "a name the function declares local is not trap state"},
		{"here.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nset_it /tmp/a\ncat <<'EOF'\nx=$(set_it /tmp/b)\nEOF\n", "a heredoc BODY is data, not a call"},
		{"quoted.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nset_it /tmp/a\necho \"run set_it in a subshell and it is lost\"\n", "the name inside a quoted string is prose"},
		{"comment.sh", "set_it() { WORK=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nset_it /tmp/a\n# never x=$(set_it /tmp/b)\n", "a comment creates nothing"},
		{"notrap.sh", "set_it() { WORK=$1; }\nx=$(set_it /tmp/a)\n", "no EXIT trap, so there is no trap state to lose"},
		{"othervar.sh", "set_it() { OTHER=$1; }\ntrap 'rm -rf -- \"${WORK:?}\"' EXIT\nx=$(set_it /tmp/a)\n", "the handler does not read what this function sets"},
	} {
		s := read(tc.name, tc.body)
		if got := tainted(s); len(got) != 0 {
			t.Errorf("%s: %s is reported as a violation (%v) — a false red here is a reader the next seat learns to route around", tc.name, tc.why, got)
		}
	}

	// A top-level assignment to trap state inside a subshell, with no
	// function in the middle — the same defect one layer shallower, and the
	// arm that catches it when somebody inlines the setter instead of
	// hoisting the call.
	loose := read("loose.sh", "trap 'rm -rf -- \"${WORK:?}\"' EXIT\n( WORK=$(mktemp -d) )\n")
	if len(loose.Loose) != 1 || loose.Loose[0].Line != 2 {
		t.Errorf("a top-level `( WORK=… )` is reported as %v, want one site on line 2 — an assignment made inside a bracket frame dies there whether a function is involved or not", loose.Loose)
	}
	clean := read("notloose.sh", "trap 'rm -rf -- \"${WORK:?}\"' EXIT\nWORK=$(mktemp -d)\n")
	if len(clean.Loose) != 0 {
		t.Errorf("`WORK=$(mktemp -d)` is reported as a subshell assignment (%v) — the `$( … )` is the VALUE; the assignment itself is made in this shell, and reading it the other way would red the ordinary spelling of every scratch root in the tree", clean.Loose)
	}

	// The handler's own reading: a function handler contributes the names it
	// reads minus the ones it declares local, an inline handler contributes
	// the names in the string, and `trap - EXIT` contributes nothing.
	fn := read("handler.sh", "cleanup() {\n	local p\n	for p in $PIDS; do kill \"$p\"; done\n	rm -rf \"$ROOT\"\n}\ntrap cleanup EXIT\n")
	if !fn.ReadVars["PIDS"] || !fn.ReadVars["ROOT"] || fn.ReadVars["p"] {
		t.Errorf("a function handler reads %v, want PIDS and ROOT and not its own local p — a local name cannot be trap state, and counting it would red whoever assigns that name elsewhere", fn.ReadVars)
	}
	off := read("untrap.sh", "cleanup() { rm -rf \"$ROOT\"; }\ntrap - EXIT\n")
	if len(off.Handlers) != 0 {
		t.Errorf("`trap - EXIT` registers %v, want nothing — that line REMOVES a handler", off.Handlers)
	}
}
