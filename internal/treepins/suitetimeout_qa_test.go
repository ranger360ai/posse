package treepins

// QA pins for ranger-base-2ggb — the suite's own timeout.
//
// Claim: no entry point that runs this repo's tests relies on `go test`'s
// DEFAULT -timeout. The default is 10m per package and internal/rhq spends
// most of it: measured on darwin between 484.6s and 623.2s standalone across
// three sessions on 2026-08-29, and at 600.8s / 601.0s / 601.1s under a plain
// `go test ./...` — which is not an assertion but the ceiling arriving as a
// timeout panic, because `./...` runs the three packages concurrently and
// starves the long one. A package at its own ceiling produces a red that
// belongs to the box, lands on whoever ran the suite to verify an unrelated
// diff, and names NO TEST AT ALL through the house filter (`go test ./... |
// grep -E '^(---|ok|FAIL)'` prints a bare `FAIL … 601.010s`).
//
// So `make test` carries `-timeout 25m` and this is the pin that keeps it
// there. Three arms, because the flag can be lost three ways:
//
//  1. the `test` target's own recipe drops it, or carries a value that does
//     not clear the measured runtime with room — a `-timeout 8m` is WORSE
//     than the default it replaced, and reads as compliance;
//  2. a NEW entry point — another Makefile target, a script, a workflow step
//     — invokes `go test` directly and inherits the default, routing around
//     the target that was fixed;
//  3. the detector itself goes blind. Arms 1 and 2 assert ABSENCE over files
//     that also DISCUSS `go test ./...` in prose, so a reader that missed the
//     Makefile's `$(GOBIN) test` spelling, or a comment rule that swallowed
//     the recipe, would leave both green over a suite on the default.
//
// THE CORPUS IS SHELL FILES, TWO RULES, not `scripts/*.sh` (ranger-base-enhtx).
// The extension was the whole of it until 2026-11-14, and scripts/ holds three
// extensionless entry points — posse-land-check, posse-landed-check and
// posse-wall-check — that the glob saw none of. The first two name `go test`
// only in comments, so the hole stood over an empty population until ADR 0097
// D3 gave posse-wall-check a live
// `go test -count=1 -json -timeout 15m -run … ./internal/posse`: arm 2 was
// green over it, and a landing that dropped the flag would have reddened
// nothing. The rule is now scripts/shell-syntax.sh's and tsIsShell's — a .sh,
// or a shell shebang — and NOT "every file under scripts/", because the .py
// censuses there carry `go test` inside string literals and regexes that no
// `#` rule can tell from code.
//
// Widening it turned up two more things, and arm 3 below carries a case for
// each: the live invocation is written `pins_argv=(go test …)`, an argv recorded on
// one line and exec'd on another, which `adjacentGoTest` did not read as a
// run; and the same file hands python a 130-line parser whose docstring says
// "not go test framing", which a line-oriented reader over shell reports as
// an entry point on the default timeout. MEASURED 2026-11-14 over the widened
// corpus: 53 files, 49 of them shell under scripts/, 31 invocations, and that
// docstring line the only piece of prose among them.
//
// The floor below is not the measurement and not the flag: it is the least
// value that is still a DECISION rather than a smaller default. Raising the
// suite's real runtime toward it is a reason to re-measure and move both
// numbers, which is exactly the conversation this pin exists to force.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// suiteTimeoutFloor is the least `-timeout` that counts as a decision.
// internal/rhq's worst measured run is 623.2s, standalone (ranger-base-2ggb,
// ranger-base-2ad3); the Makefile carries 25m, comfortably above this.
const suiteTimeoutFloor = 15 * time.Minute

// shellWords splits a line the way a shell splits a command line: blanks
// separate words, and a quoted run — `'…'` or `"…"` — joins the word it
// touches instead of breaking it. It returns each word with its quotes
// removed, a parallel slice saying whether any part of that word came from
// inside quotes, and whether every quote on the line closed.
//
// This is what tells an invocation from a mention of one. `probe "$w" "go
// test ./..."` is three words, the third a single opaque string, and no `go`
// stands next to a `test` anywhere in the argv.
func shellWords(line string) (words []string, quoted []bool, balanced bool) {
	var cur strings.Builder
	var q rune // the open quote, or 0
	inWord, sawQuote := false, false
	flush := func() {
		words = append(words, cur.String())
		quoted = append(quoted, sawQuote)
		cur.Reset()
		inWord, sawQuote = false, false
	}
	for _, r := range line {
		switch {
		case q != 0:
			if r == q {
				q = 0
				continue
			}
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			q, inWord, sawQuote = r, true, true
		case r == ' ' || r == '\t':
			if inWord {
				flush()
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		flush()
	}
	return words, quoted, q == 0
}

// goWord reports whether this word is the `go` of a `go test`.
//
// Several spellings, and each of them is a RUN. The Makefile's `$(GOBIN)`
// is here because the recipe under test runs it and a pin that only knew the
// word `go` would be green over the very line it is about. And a leading `(` or
// `NAME=(` is stripped, because bash's array-assignment form
//
//	pins_argv=(go test -count=1 -json -timeout 15m -run "$pat" ./internal/posse)
//
// is scripts/posse-wall-check:186 — an argv written down on one line and
// exec'd on another, which is the house shape for a child a `--plan` has to
// be able to print. It is as much an invocation of `go test` as a bare one,
// and it is the live line this pin's corpus was widened to reach
// (ranger-base-enhtx).
func goWord(w string) bool {
	if i := strings.Index(w, "=("); i >= 0 {
		w = w[i+2:]
	} else {
		w = strings.TrimPrefix(w, "(")
	}
	switch w {
	case "go", "$(GOBIN)", "${GOBIN}", "$GOBIN":
		return true
	}
	return false
}

// adjacentGoTest returns the arguments of a `go test` standing in one word
// list, or nil.
func adjacentGoTest(words []string) []string {
	for i := 0; i+1 < len(words); i++ {
		if !goWord(words[i]) {
			continue
		}
		if words[i+1] != "test" {
			continue
		}
		return words[i+2:]
	}
	return nil
}

// isShellCommand reports whether this command word is a SHELL — the thing
// whose `-c` argument is a command line. `$SHELL` is here because a script
// that spells it that way is still running a shell.
func isShellCommand(w string) bool {
	if i := strings.LastIndexByte(w, '/'); i >= 0 {
		w = w[i+1:]
	}
	switch w {
	case "sh", "bash", "dash", "ksh", "zsh", "$SHELL", "${SHELL}":
		return true
	}
	return false
}

// commandBefore returns the command word that the flag at words[at] belongs
// to: the nearest earlier word that is not itself a flag, or "" when there
// is none. The walk-back is what `python3 -I -c` needs — two words between
// the command and its `-c`, and the live spelling at
// scripts/posse-wall-check:246.
func commandBefore(words []string, at int) string {
	for j := at - 1; j >= 0; j-- {
		if strings.HasPrefix(words[j], "-") {
			continue
		}
		return words[j]
	}
	return ""
}

// reparsesItsArgument reports whether words[i] is handed to something that
// runs its argument as a command line rather than reading it as text. Two
// do: a SHELL's `-c`, and a workflow step's `run:`. Both are plausible
// homes for a new entry point, which is what arm 2 exists to catch, so a
// quoted word in either place is opened rather than trusted.
//
// A `-c` only counts when the command it belongs to is a shell, because
// `-c` is not a shell's flag alone. `python3 -I -c '<program>'` is the live
// one — scripts/posse-wall-check hands python a 130-line parser — and its
// argument is a program in another language, where `go test` can appear in a
// docstring. Reading that as argv is a false positive over prose, which is
// the same mistake ranger-base-quqn fixed for a quoted word and this fixes
// for a quoted PROGRAM (ranger-base-enhtx).
func reparsesItsArgument(words []string, i int) bool {
	if i == 0 {
		return false
	}
	if words[i-1] == "run:" {
		return true
	}
	if words[i-1] != "-c" {
		return false
	}
	return isShellCommand(commandBefore(words, i-1))
}

// goTestArgs returns the arguments of a `go test` invocation on one line of
// shell/make/yml, or nil when the line does not invoke one.
//
// A quoted word is DATA, not argv — the string an `echo` prints, a usage
// message, the JSON payload a gate probe hands to a gate that answers it
// without running anything. That last one is ranger-base-quqn: `probe "$w"
// "go test ./..."` in scripts/verify-gate-freshness.sh, and the `echo`
// reporting its result, both read as entry points under a plain field scan
// and reddened main for every branch that merged it. Neither runs the suite,
// so neither can be running it on the default timeout.
func goTestArgs(line string) []string {
	words, quoted, balanced := shellWords(line)
	if !balanced {
		// The quoting does not close, so no split of this line is honest.
		// Fall back to blank-separated fields — the sensitive direction: a
		// line the tokenizer cannot read is still read for an invocation.
		words = strings.Fields(line)
		quoted = make([]bool, len(words))
		for i := range words {
			words[i] = strings.Trim(words[i], `"'`)
		}
	}
	if args := adjacentGoTest(words); args != nil {
		return args
	}
	for i, w := range words {
		if !quoted[i] || !reparsesItsArgument(words, i) {
			continue
		}
		if args := goTestArgs(w); args != nil {
			return args
		}
	}
	return nil
}

// timeoutOf returns the value of a `-timeout` flag in either spelling
// (`-timeout 20m` / `-timeout=20m`, one dash or two), and whether one is
// present at all.
func timeoutOf(args []string) (time.Duration, bool, error) {
	for i, a := range args {
		a = strings.Trim(a, `"'`)
		flag := strings.TrimLeft(a, "-")
		var raw string
		switch {
		case flag == "timeout" && i+1 < len(args):
			raw = strings.Trim(args[i+1], `"'`)
		case strings.HasPrefix(flag, "timeout="):
			raw = strings.TrimPrefix(flag, "timeout=")
		default:
			continue
		}
		d, err := time.ParseDuration(raw)
		return d, true, err
	}
	return 0, false, nil
}

// isComment reports whether a line is prose. Every file in the corpus below
// — Makefile, sh, yml — comments with `#`, and every one of them DISCUSSES
// `go test ./...` in prose, so a pin that could not tell the two apart would
// fail on its own documentation.
func isComment(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "#")
}

// embeddedProgramQuote reports the quote that OPENS a program written in
// another language on this line and does not close on it, or 0.
//
// This is the only cross-line state the reader keeps, and it is armed by one
// recognisable shape: a quote opened immediately after the `-c` of a command
// that is NOT a shell. scripts/posse-wall-check:246 is it —
//
//	printf '%s\n' "$pins_json" | python3 -I -c '
//
// and the 130 lines under it are python, not shell. One of them reads
//
//	not go test framing. One rule for a failure line and for a skip line,
//
// inside a docstring: a line-oriented reader over shell sees `go` next to
// `test` and reports an entry point running on the default timeout. That was
// the one false positive the widened corpus turned up (MEASURED 2026-11-14,
// ranger-base-enhtx: 49 shell files under scripts/, 31 invocations, this the
// only line of prose among them).
//
// WHY ARMED RATHER THAN TRACKED. Carrying quote state across every line of
// every script would desync on the first apostrophe in a heredoc body — the
// English in scripts/verify-box.sh's tables is full of them — and a desynced
// tracker SKIPS, which is blindness with no symptom. So the default stays
// "read this line for an invocation", and only an explicit non-shell `-c`
// opener suspends it. A shell's `-c` is deliberately not armed: those lines
// ARE shell, and hiding an entry point in one is exactly the route-around
// arm 2 exists to catch.
//
// A single-quoted body cannot contain a single quote — the shell would end
// the word — so for the live spelling the body's extent is exact. A
// double-quoted body can carry an escaped `\"`, which ends the skip early;
// that is the sensitive direction, so it is left alone.
func embeddedProgramQuote(line string) rune {
	var q rune
	open := 0
	for i, r := range line {
		switch {
		case q != 0:
			if r == q {
				q = 0
			}
		case r == '\'' || r == '"':
			q, open = r, i
		}
	}
	if q == 0 {
		return 0
	}
	words := strings.Fields(line[:open])
	if len(words) == 0 || words[len(words)-1] != "-c" {
		return 0
	}
	if cmd := commandBefore(words, len(words)-1); cmd == "" || isShellCommand(cmd) {
		return 0
	}
	return q
}

// afterEmbeddedProgram returns what is left of a line after the quote that
// closes an embedded program, and whether that quote was on it. The
// remainder is shell again — `' "$go_rc" "$marker" "$pattern" || rc=$?` is
// the live one — so it is handed back to be read rather than dropped.
func afterEmbeddedProgram(line string, q rune) (string, bool) {
	if i := strings.IndexRune(line, q); i >= 0 {
		return line[i+len(string(q)):], true
	}
	return "", false
}

// goTestHit is one `go test` invocation the reader found in a file.
type goTestHit struct {
	Line int // 1-based, the physical line
	Args []string
	Code string // the line as written, trimmed, for a diagnostic
}

// scanEntryPoints reads one file of shell/make/yml and returns every
// `go test` invocation in it.
//
// One function, used by arm 2's sweep and by the reader's own arm below, so
// the two cannot come to disagree about what a line is — the same reason
// trapstatepick and trapsignalexit share tsRead rather than walking scripts/
// twice.
func scanEntryPoints(body string) []goTestHit {
	var hits []goTestHit
	var program rune // the open quote of an embedded non-shell program
	for n, line := range strings.Split(body, "\n") {
		if program != 0 {
			rest, closed := afterEmbeddedProgram(line, program)
			if !closed {
				continue
			}
			program, line = 0, rest
		}
		if isComment(line) {
			continue
		}
		program = embeddedProgramQuote(line)
		if args := goTestArgs(line); args != nil {
			hits = append(hits, goTestHit{Line: n + 1, Args: args, Code: strings.TrimSpace(line)})
		}
	}
	return hits
}

// The `test` target is what CI, `make test-linux` and every persona run, so
// its recipe is the one line that has to carry the flag.
func TestQASuiteTestTargetCarriesATimeoutAboveTheMeasuredRuntime(t *testing.T) {
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	recipe := makeRecipe(string(makefile), "test")
	if len(recipe) == 0 {
		t.Fatal("the Makefile has no `test` target — the suite command is the thing under test")
	}

	var args []string
	for _, line := range recipe {
		if isComment(line) {
			continue
		}
		if a := goTestArgs(line); a != nil {
			args = a
			break
		}
	}
	if args == nil {
		t.Fatalf("`make test`'s recipe invokes no `go test`:\n%s", strings.Join(recipe, "\n"))
	}

	d, ok, err := timeoutOf(args)
	if !ok {
		t.Fatalf("`make test` runs `go test` on the DEFAULT 10m timeout (args %v) — internal/posse's worst measured run is 623.2s, so the suite is a coin flip on a loaded box and the red it throws names no test (ranger-base-2ggb)", args)
	}
	if err != nil {
		t.Fatalf("-timeout value does not parse: %v", err)
	}
	if d < suiteTimeoutFloor {
		t.Errorf("-timeout %s is below the %s floor — internal/posse alone has been measured at 623.2s, and a ceiling under the measurement is worse than the default because it reads as a decision (ranger-base-2ggb)", d, suiteTimeoutFloor)
	}
}

// Arm 2: the flag cannot be routed around. A second entry point that runs
// `go test` itself gets the 10m default back, and it would be green here
// while `make test` stayed correct.
func TestQANoEntryPointRunsGoTestOnTheDefaultTimeout(t *testing.T) {
	var walked []string
	if err := filepath.WalkDir("scripts", mtWalkInto(&walked)); err != nil {
		t.Fatalf("walk scripts: %v", err)
	}
	files := []string{"Makefile"}
	shell, extensionless := 0, 0
	for _, p := range walked {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if !tsIsShell(p, b) {
			continue
		}
		shell++
		if !strings.HasSuffix(p, ".sh") {
			extensionless++
		}
		files = append(files, p)
	}
	matched, err := filepath.Glob(filepath.Join(".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, matched...)

	// Floors, under the live reading rather than on it: an assertion that no
	// entry point runs on the default is satisfied by a corpus that found no
	// entry points. MEASURED 2026-11-14 (ranger-base-enhtx): 49 shell files
	// of the 64 under scripts/, 3 of them extensionless, 3 workflows, 53
	// files in all.
	if len(files) < 45 || shell < 40 {
		t.Fatalf("corpus is %d files, %d of them shell under scripts/ — the walk found nothing to check, so a green here measures nothing", len(files), shell)
	}
	// THE HOLE THIS FLOOR HOLDS SHUT. This corpus was `scripts/*.sh` until
	// ranger-base-enhtx, and scripts/ holds three extensionless entry points
	// — posse-land-check, posse-landed-check, posse-wall-check — that the
	// glob saw none of. The first two name `go test` only in comments, so
	// the hole was over an empty population until posse-wall-check landed a
	// LIVE `go test -count=1 -json -timeout 15m -run … ./internal/posse`
	// (ADR 0097 D3). A reader that goes back to keying on the extension
	// reads 0 here.
	if extensionless < 1 {
		t.Fatalf("no shell file under scripts/ lacks a .sh extension — scripts/ carries 3 extensionless entry points (MEASURED 2026-11-14), so a corpus that finds none is keyed on the extension again and is blind to every one of them (ranger-base-enhtx)")
	}

	checked := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, h := range scanEntryPoints(string(b)) {
			checked++
			d, ok, err := timeoutOf(h.Args)
			switch {
			case !ok:
				t.Errorf("%s:%d runs `go test` on the default 10m timeout: %s\n\tinternal/posse's worst measured run is 623.2s — route this through `make test` or give it its own -timeout (ranger-base-2ggb)", f, h.Line, h.Code)
			case err != nil:
				t.Errorf("%s:%d: -timeout does not parse: %v", f, h.Line, err)
			case d < suiteTimeoutFloor:
				t.Errorf("%s:%d: -timeout %s is below the %s floor: %s", f, h.Line, d, suiteTimeoutFloor, h.Code)
			}
		}
	}
	if checked == 0 {
		t.Error("no `go test` invocation found in the whole corpus — the detector is blind (it must see at least the Makefile's `test` recipe), so absence here is not evidence")
	}
}

// The detector's own arms, because both tests above are assertions of
// ABSENCE over lines this repo also DISCUSSES in prose: a `goTestArgs` that
// missed `$(GOBIN) test`, or an `isComment` that swallowed the recipe, would
// leave every arm green over a suite running on the default.
func TestQAGoTestDetectorReadsBothSpellingsAndSkipsProse(t *testing.T) {
	for _, c := range []struct {
		line string
		want bool // is this an invocation?
		dur  string
	}{
		{"\t$(GOBIN) test -timeout 25m ./...", true, "25m"},
		{"go test -timeout=25m ./internal/posse", true, "25m"},
		{"go test --timeout 20m ./...", true, "20m"},
		{"GOBIN=go go test ./...", true, ""},
		{"# `make test` is `go test ./...` and nothing lighter", false, ""},
		{"      # The repo's own gate. `go test -timeout 25m ./...`", false, ""},
		{"gate='go vet ./... && make test'", false, ""},
		{"go build ./...", false, ""},

		// ranger-base-quqn: a `go test` inside a quoted word is a string a
		// command is HANDED, not a command that runs. All four are lines this
		// repo really carries — the first two reddened main at 25503c1, and
		// the third has been a false positive since it was written, green only
		// because the text it prints happens to carry a 40m.
		{`      probe "$w" "go test ./..."`, false, ""},
		{`        echo "    behaves  bd list allowed · bd daemon stop denied by the parser · go test untouched"`, false, ""},
		{`    echo "      go test $pkg -count=1 -timeout 40m"`, false, ""},
		{`warn 'usage: scripts/test-times.sh <go test command...>'`, false, ""},

		// ...but a quoted word that something RE-PARSES as a command line is
		// an entry point, and hiding one there is exactly the route-around
		// arm 2 exists to catch.
		{`sh -c "go test ./..."`, true, ""},
		{`bash -c 'go test -timeout 25m ./...'`, true, "25m"},
		{"      run: go test ./...", true, ""},
		{`      - run: "go test -timeout 20m ./..."`, true, "20m"},

		// A line whose quoting never closes cannot be split honestly, so the
		// detector falls back to blank-separated fields rather than going
		// blind on it.
		{"echo it's fine && go test ./...", true, ""},

		// ranger-base-enhtx: bash's array-assignment form is an argv written
		// down before it is exec'd, and scripts/posse-wall-check:186 is the
		// live one. A reader that only knew a bare command word would be
		// green over the extensionless entry point this pin's corpus was
		// widened to reach.
		{`pins_argv=(go test -count=1 -json -timeout 15m -run "$member_pattern" ./internal/posse)`, true, "15m"},
		{`argv=($(GOBIN) test -timeout 25m ./...)`, true, "25m"},
		{`( go test -timeout 20m ./... )`, true, "20m"},

		// ...and `-c` is a shell's flag only when the command is a shell.
		// python's `-c` argument is a program in another language, where a
		// `go test` is prose until proven otherwise.
		{`python3 -I -c "go test ./..."`, false, ""},
		{`python3 -c 'go test ./...'`, false, ""},
		{`env bash -c "go test -timeout 30m ./..."`, true, "30m"},
		{`/bin/sh -c "go test ./..."`, true, ""},
	} {
		if isComment(c.line) {
			if c.want {
				t.Errorf("%q was read as prose but invokes go test", c.line)
			}
			continue
		}
		args := goTestArgs(c.line)
		if (args != nil) != c.want {
			t.Errorf("goTestArgs(%q) = %v, want invocation=%v", c.line, args, c.want)
			continue
		}
		if !c.want {
			continue
		}
		d, ok, err := timeoutOf(args)
		if err != nil {
			t.Errorf("timeoutOf(%q): %v", c.line, err)
			continue
		}
		if c.dur == "" {
			if ok {
				t.Errorf("timeoutOf(%q) found %s, want none", c.line, d)
			}
			continue
		}
		want, _ := time.ParseDuration(c.dur)
		if !ok || d != want {
			t.Errorf("timeoutOf(%q) = %s,%v, want %s", c.line, d, ok, want)
		}
	}

	// The cross-line half, which no per-line case above can state: a program
	// handed to a non-shell `-c` spans lines, and its prose is not shell.
	// Both fixtures are scripts/posse-wall-check's own shape — the python
	// parser whose docstring says "not go test framing", and the real
	// invocation two steps above it (ranger-base-enhtx).
	const fixture = `#!/usr/bin/env bash
pins_argv=(go test -count=1 -json -timeout 15m -run "$pat" ./internal/posse)
pins_json=$("${pins_argv[@]}" 2>&1)
printf '%s\n' "$pins_json" | python3 -I -c '
def evidence(top):
    """The first line that is EVIDENCE: the first file.go:NN: line that is
    not the marker, else the first line that is
    not go test framing. One rule for a failure line and for a skip line."""
    return ""
' "$go_rc" || rc=$?
go test -timeout 25m ./...
`
	hits := scanEntryPoints(fixture)
	var got []int
	for _, h := range hits {
		got = append(got, h.Line)
	}
	if len(got) != 2 || got[0] != 2 || got[1] != 11 {
		t.Errorf("scanEntryPoints found invocations on lines %v, want [2 11] — line 2 is the array argv and line 11 the bare run; line 8 is a docstring inside a python program and reads as an entry point to a line-oriented reader (%v)", got, hits)
	}
	for _, h := range hits {
		if _, ok, _ := timeoutOf(h.Args); !ok {
			t.Errorf("fixture line %d reads as an invocation with no -timeout, so the skip is off by a line: %s", h.Line, h.Code)
		}
	}
	// And the control, so the arm above is not satisfied by a reader that
	// drops every quoted body: a SHELL's `-c` body is shell, and an entry
	// point hidden in one is what arm 2 exists to catch.
	const shellFixture = `bash -c '
go test ./...
'`
	switch hits := scanEntryPoints(shellFixture); {
	case len(hits) != 1:
		t.Errorf("a multi-line `bash -c` body yielded %d invocation(s), want 1: %v", len(hits), hits)
	case hits[0].Line != 2:
		t.Errorf("the `bash -c` body's `go test` was found on line %d, want 2", hits[0].Line)
	}
}
