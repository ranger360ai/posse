package main

// rangerhq-qv5: `posse new --help` used to take '--help' as the session name
// and create a real herdr workspace plus a meta file. The reading of a
// leading dashed argument lives in argLead; the end-to-end half runs the
// built binary, because "created nothing" is a claim about the process.

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestArgLead(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		rest []string
		help bool
	}{
		{"help long", []string{"--help"}, []string{"--help"}, true},
		{"help short", []string{"-h"}, []string{"-h"}, true},
		{"help before a name", []string{"--help", "proj"}, []string{"--help", "proj"}, true},
		{"separator frees a dashed name", []string{"--", "--help"}, []string{"--help"}, false},
		{"separator alone", []string{"--"}, []string{}, false},
		{"a plain name", []string{"proj"}, []string{"proj"}, false},
		{"help after the name is the subcommand's own flag", []string{"proj", "--help"}, []string{"proj", "--help"}, false},
		{"no args", nil, nil, false},
	} {
		rest, help := argLead(c.args)
		if help != c.help || !reflect.DeepEqual(rest, c.rest) {
			t.Errorf("%s: argLead(%q) = (%q, %v), want (%q, %v)", c.name, c.args, rest, help, c.rest, c.help)
		}
	}
}

// The bead's regression test: usage on stdout, exit 0, and nothing left on
// the host — no meta file, and no herdr call at all (RHQ_HERDR_BIN points
// at a binary that fails loudly if anything reaches it).
func TestNewHelpCreatesNothing(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	env := append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
	)

	for _, c := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"new", "--help"}, 0, "usage: posse new <name>"},
		{[]string{"new", "-h"}, 0, "usage: posse new <name>"},
		{[]string{"new", "-x"}, 1, "bad session name '-x'"},
		{[]string{"kill", "--help"}, 0, "usage: posse kill <name>"},
		{[]string{"attach", "--help"}, 0, "usage: posse attach <name>"},
		{[]string{"up", "--help"}, 0, "usage: posse attach <name>"},
		{[]string{"local", "--help"}, 0, "usage: posse attach <name>"},
		{[]string{"recipe", "--help"}, 0, "usage: posse recipe <name>"},
		{[]string{"prompt", "--help"}, 0, `usage: posse prompt <name> "<text>"`},
		{[]string{"relaunch", "--help"}, 0, "usage: posse relaunch <name>"},
		{[]string{"crew", "--help"}, 0, "usage: posse crew <name>"},
	} {
		cmd := exec.Command(bin, c.args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("posse %s: %v", strings.Join(c.args, " "), err)
		}
		if code != c.code || !strings.Contains(string(out), c.want) {
			t.Errorf("posse %s: exit %d, output %q; want exit %d containing %q",
				strings.Join(c.args, " "), code, out, c.code, c.want)
		}
	}

	var left []string
	filepath.WalkDir(home, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			left = append(left, p)
		}
		return nil
	})
	if len(left) > 0 {
		t.Errorf("help and a refused name must leave no state behind: %q", left)
	}
}

func buildRhq(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "posse")
	out, err := exec.Command("go", "build", "-o", bin, "github.com/ranger360ai/posse/cmd/posse").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// ranger-base-g98: a machine with neither config home seeds the new posse
// path. This runs the command, rather than only checking NewApp's string,
// because the contract includes the complete embedded instance.
func TestInitUsesPosseHomeByDefault(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	cmd := exec.Command(bin, "init")
	cmd.Env = []string{"HOME=" + home, "RHQ_HOME="}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("posse init: %v\n%s", err, out)
	}

	wantHome := filepath.Join(home, ".config", "posse")
	if !strings.Contains(string(out), "initialized "+wantHome) {
		t.Errorf("init said %q, want home %s", out, wantHome)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "examples", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(wantHome, "config.yaml"))
	if err != nil || string(got) != string(want) {
		t.Errorf("seeded config: %v (%d bytes, want %d)", err, len(got), len(want))
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "rhq")); !os.IsNotExist(err) {
		t.Errorf("init touched the legacy home: %v", err)
	}
}

// rangerhq-ytkl: `posse dispatch -n three` used to reach the pass as -n 0,
// and 0 is documented as no cap — a typo in the one flag whose job is to
// bound a pass made it unbounded, with no line said about it. --timeout
// dropped the same error. Both halves matter, so both are here: the reading
// (validCount) and the process (exit 1, the flag named, nothing dispatched).
func TestValidCount(t *testing.T) {
	for _, c := range []struct {
		arg  string
		want bool
	}{
		{"3", true},
		{"0", true},   // the deliberate escape hatch: no cap
		{"007", true}, // strconv's reading, not a new one
		{"three", false},
		{"3x", false},
		{"", false},
		{" 3", false},
		{"3.0", false},
		{"-1", false}, // fireLoop caps only on max > 0, so this was unbounded too
	} {
		if got := validCount(c.arg); got != c.want {
			t.Errorf("validCount(%q) = %v, want %v", c.arg, got, c.want)
		}
	}
}

func writeExec(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func readyEnv(t *testing.T, home string, extra ...string) []string {
	t.Helper()
	return append(append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
	), extra...)
}

const fakeBdScript = `#!/bin/sh
cmd=""
for a in "$@"; do
  case "$a" in
    -*) ;;
    *) cmd=$a; break ;;
  esac
done
if [ -f fake-ready-fail ]; then
  echo "database is locked" >&2
  exit 1
fi
if [ "$cmd" = "ready" ] && [ -f fake-ready.json ]; then
  cat fake-ready.json
  exit 0
fi
echo '[]'
exit 0
`

// rangerhq-llse: `posse ready` must not print "no ready work" over a scan
// that failed. An unreadable repo is an unknown queue.
func TestReadyRefusesAFailedScanAsEmpty(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "fake-ready-fail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bd := writeExec(t, t.TempDir(), "bd", fakeBdScript)

	cmd := exec.Command(bin, "ready")
	cmd.Env = readyEnv(t, home, "RHQ_BD_BIN="+bd)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("a failed scan must fail the command:\n%s", out)
	}
	got := string(out)
	if !strings.Contains(got, "unknown, not empty") {
		t.Errorf("want the queue named unknown, got:\n%s", got)
	}
	if !strings.Contains(got, "ready scan failed") {
		t.Errorf("want the failed repo named, got:\n%s", got)
	}
	if strings.Contains(got, "no ready work") {
		t.Errorf("a failed scan must never read as an empty queue:\n%s", got)
	}
}

func TestReadyNamesAPartialScanAndStillLists(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	good := t.TempDir()
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads:\n  - "+good+"\n  - "+bad+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(good, "fake-ready.json"), []byte(`[{"id":"a-1","title":"one","priority":1}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "fake-ready-fail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bd := writeExec(t, t.TempDir(), "bd", fakeBdScript)

	cmd := exec.Command(bin, "ready")
	cmd.Env = readyEnv(t, home, "RHQ_BD_BIN="+bd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("one readable repo is enough to list: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "a-1") {
		t.Errorf("want the readable repo's bead listed:\n%s", got)
	}
	if !strings.Contains(got, "ready scan failed") || !strings.Contains(got, "database is locked") {
		t.Errorf("the failed repo must be named:\n%s", got)
	}
	if strings.Contains(got, "no ready work") {
		t.Errorf("a partial scan must not read as empty:\n%s", got)
	}
}

func TestReadyEmptyQueueStillSaysNoReadyWork(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	bd := writeExec(t, t.TempDir(), "bd", fakeBdScript)

	cmd := exec.Command(bin, "ready")
	cmd.Env = readyEnv(t, home, "RHQ_BD_BIN="+bd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("a genuine empty queue is not an error: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "no ready work") {
		t.Errorf("a successful empty scan must still say so:\n%s", out)
	}
}

// The end-to-end half: a bad count is refused by the process before the
// pass starts. PATH is emptied so the first thing past the flag loop is
// "bd not found" — which is also the positive control, since only a count
// the loop accepted can get that far.
func TestDispatchRefusesBadCount(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	env := append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
		"PATH=",
	)

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"dispatch", "-n", "three"}, "-n needs a count"},
		{[]string{"dispatch", "-n", "3x"}, "-n needs a count"},
		{[]string{"dispatch", "-n", ""}, "-n needs a count"},
		{[]string{"dispatch", "-n", "-1"}, "-n needs a count"},
		{[]string{"dispatch", "-n"}, "-n needs a count"},
		{[]string{"dispatch", "--timeout", "soon"}, "--timeout needs a value in ms"},
		{[]string{"dispatch", "--timeout"}, "--timeout needs a value in ms"},
		// Accepted counts reach the next check instead of dying here.
		{[]string{"dispatch", "-n", "3", "--timeout", "0", "--dry-run"}, "bd not found in PATH"},
		{[]string{"dispatch", "-n", "0", "--dry-run"}, "bd not found in PATH"},
	} {
		cmd := exec.Command(bin, c.args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("posse %s: %v", strings.Join(c.args, " "), err)
		}
		if code != 1 || !strings.Contains(string(out), c.want) {
			t.Errorf("posse %s: exit %d, output %q; want exit 1 containing %q",
				strings.Join(c.args, " "), code, out, c.want)
		}
	}
}
