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

// goTestArgs returns the arguments of a `go test` invocation on one line of
// shell/make, or nil when the line does not invoke one. It accepts the
// Makefile's `$(GOBIN)` spelling as well as a literal `go`, because the
// recipe under test runs the former and a pin that only knows the word `go`
// would be green over the very line it is about.
func goTestArgs(line string) []string {
	fields := strings.Fields(line)
	for i := 0; i+1 < len(fields); i++ {
		cmd := strings.Trim(fields[i], `"'`)
		if cmd != "go" && cmd != "$(GOBIN)" && cmd != "${GOBIN}" && cmd != "$GOBIN" {
			continue
		}
		if strings.Trim(fields[i+1], `"'`) != "test" {
			continue
		}
		return fields[i+2:]
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
