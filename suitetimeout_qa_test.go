package posse

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

// adjacentGoTest returns the arguments of a `go test` standing in one word
// list, or nil. It accepts the Makefile's `$(GOBIN)` spelling as well as a
// literal `go`, because the recipe under test runs the former and a pin that
// only knew the word `go` would be green over the very line it is about.
func adjacentGoTest(words []string) []string {
	for i := 0; i+1 < len(words); i++ {
		switch words[i] {
		case "go", "$(GOBIN)", "${GOBIN}", "$GOBIN":
		default:
			continue
		}
		if words[i+1] != "test" {
			continue
		}
		return words[i+2:]
	}
	return nil
}

// reparsesItsArgument reports whether words[i] is handed to something that
// runs its argument as a command line rather than reading it as text. Two
// do: a shell's `-c`, and a workflow step's `run:`. Both are plausible
// homes for a new entry point, which is what arm 2 exists to catch, so a
// quoted word in either place is opened rather than trusted.
func reparsesItsArgument(words []string, i int) bool {
	if i == 0 {
		return false
	}
	switch words[i-1] {
	case "-c", "run:":
		return true
	}
	return false
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
		t.Fatalf("`make test` runs `go test` on the DEFAULT 10m timeout (args %v) — internal/rhq's worst measured run is 623.2s, so the suite is a coin flip on a loaded box and the red it throws names no test (ranger-base-2ggb)", args)
	}
	if err != nil {
		t.Fatalf("-timeout value does not parse: %v", err)
	}
	if d < suiteTimeoutFloor {
		t.Errorf("-timeout %s is below the %s floor — internal/rhq alone has been measured at 623.2s, and a ceiling under the measurement is worse than the default because it reads as a decision (ranger-base-2ggb)", d, suiteTimeoutFloor)
	}
}

// Arm 2: the flag cannot be routed around. A second entry point that runs
// `go test` itself gets the 10m default back, and it would be green here
// while `make test` stayed correct.
func TestQANoEntryPointRunsGoTestOnTheDefaultTimeout(t *testing.T) {
	var files []string
	files = append(files, "Makefile")
	for _, glob := range []string{
		filepath.Join("scripts", "*.sh"),
		filepath.Join(".github", "workflows", "*.yml"),
	} {
		matched, err := filepath.Glob(glob)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matched...)
	}
	if len(files) < 10 {
		t.Fatalf("corpus is %d files — the globs found nothing to check, so a green here measures nothing", len(files))
	}

	checked := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for n, line := range strings.Split(string(b), "\n") {
			if isComment(line) {
				continue
			}
			args := goTestArgs(line)
			if args == nil {
				continue
			}
			checked++
			d, ok, err := timeoutOf(args)
			switch {
			case !ok:
				t.Errorf("%s:%d runs `go test` on the default 10m timeout: %s\n\tinternal/rhq's worst measured run is 623.2s — route this through `make test` or give it its own -timeout (ranger-base-2ggb)", f, n+1, strings.TrimSpace(line))
			case err != nil:
				t.Errorf("%s:%d: -timeout does not parse: %v", f, n+1, err)
			case d < suiteTimeoutFloor:
				t.Errorf("%s:%d: -timeout %s is below the %s floor: %s", f, n+1, d, suiteTimeoutFloor, strings.TrimSpace(line))
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
		{"go test -timeout=25m ./internal/rhq", true, "25m"},
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
}
