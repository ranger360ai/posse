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
	"regexp"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/rhq"
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
		// The override the ownership refusal names has to exist where the
		// refusal says it does (rangerhq-selx).
		{[]string{"kill", "--help"}, 0, "usage: posse kill <name> [--force] [--foreign]"},
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

// ranger-base-g98: an existing ~/.config/rhq is the home until the operator
// moves it. posse init must seed that directory, not mkdir ~/.config/posse
// (which would then win on the next command and hide the live instance).
func TestInitFallsBackToLegacyHome(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	legacy := filepath.Join(home, ".config", "rhq")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := "legacy-marker: keep\n"
	if err := os.WriteFile(filepath.Join(legacy, "config.yaml"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "init")
	cmd.Env = []string{"HOME=" + home, "RHQ_HOME="}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("posse init: %v\n%s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, "initialized "+legacy) {
		t.Errorf("init said %q, want legacy home %s", s, legacy)
	}
	preferred := filepath.Join(home, ".config", "posse")
	if !strings.Contains(s, preferred) || !strings.Contains(s, "nothing moved") {
		t.Errorf("legacy notice missing from %q", s)
	}
	if _, err := os.Stat(preferred); !os.IsNotExist(err) {
		t.Errorf("init created the preferred home: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(legacy, "config.yaml"))
	if err != nil || string(got) != marker {
		t.Errorf("legacy config overwritten: %v %q", err, got)
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
		{"+3", true},  // strconv's reading, same as 007
		{"three", false},
		{"3x", false},
		{"", false},
		{" 3", false},
		{"3.0", false},
		{"15m", false},                 // ParseInterval spelling; old code read as 0
		{"9223372036854775808", false}, // overflow is an Atoi error, not a wrap
		{"-1", false},                  // fireLoop caps only on max > 0, so this was unbounded too
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

// ranger-base-sknr: ytkl closed the drop on dispatch's -n and --timeout.
// prompt and wait still do `timeout, _ = strconv.Atoi(rest[1])`. Atoi's
// 0 and a parsed -1 both skip the --timeout flag in AgentPrompt/AgentWait
// (`timeoutMS > 0`), so `posse prompt sess hi --wait --timeout soon` waits
// as long as herdr will. Missing-arg already dies; the hole is the dropped
// error. Unskipped FAIL (HEAD b79e0a2): herdr is asked; the flag is not named.
//
// RHQ_HERDR_BIN points at a file that is not there, so the first check past
// the flag loop is a fork/exec failure naming it — which is the positive
// control too (the accepted-count rows): only a value the loop took can
// reach herdr at all, so a validCount that refused everything would not
// pass this test.
func TestPromptWaitTimeoutRefusesBadCount(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	env := append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
		"PATH=",
	)

	for _, c := range []struct {
		args     []string
		want     string
		accepted bool // the count is good: it must reach herdr, not die here
	}{
		{args: []string{"prompt", "sess", "hi", "--wait", "--timeout", "soon"}, want: "--timeout needs"},
		{args: []string{"prompt", "sess", "hi", "--wait", "--timeout", "3x"}, want: "--timeout needs"},
		{args: []string{"prompt", "sess", "hi", "--wait", "--timeout", "-1"}, want: "--timeout needs"},
		{args: []string{"prompt", "sess", "hi", "--wait", "--timeout", ""}, want: "--timeout needs"},
		{args: []string{"prompt", "sess", "hi", "--wait", "--timeout"}, want: "--timeout needs"},
		{args: []string{"wait", "sess", "--timeout", "soon"}, want: "--timeout needs"},
		{args: []string{"wait", "sess", "--timeout", "3x"}, want: "--timeout needs"},
		{args: []string{"wait", "sess", "--timeout", "-1"}, want: "--timeout needs"},
		{args: []string{"wait", "sess", "--timeout", ""}, want: "--timeout needs"},
		{args: []string{"wait", "sess", "--timeout"}, want: "--timeout needs"},
		// Accepted counts reach the next check instead of dying here.
		{args: []string{"prompt", "sess", "hi", "--wait", "--timeout", "500"}, want: "herdr-must-not-run", accepted: true},
		{args: []string{"prompt", "sess", "hi", "--wait", "--timeout", "0"}, want: "herdr-must-not-run", accepted: true},
		{args: []string{"wait", "sess", "--timeout", "500"}, want: "herdr-must-not-run", accepted: true},
		{args: []string{"wait", "sess", "--timeout", "0"}, want: "herdr-must-not-run", accepted: true},
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
		got := string(out)
		if code != 1 || !strings.Contains(got, c.want) {
			t.Errorf("posse %s: exit %d, output %q; want exit 1 containing %q",
				strings.Join(c.args, " "), code, out, c.want)
		}
		if !c.accepted && strings.Contains(got, "herdr workspace list") {
			t.Errorf("posse %s reached herdr — the bad count was accepted:\n%s",
				strings.Join(c.args, " "), got)
		}
	}
}

// ranger-base-oz39: peek's positional <lines> is the third site of the same
// dropped Atoi, and the only one where the bad parse reads MORE than was
// asked for — PaneRead tails only when lines > 0, so `posse peek sess 40x`
// returned the whole pane where the operator asked for 40 rows, silently.
//
// Same rig as TestPromptWaitTimeoutRefusesBadCount: RHQ_HERDR_BIN points at
// a file that is not there, so an accepted count dies naming it (Resolve
// lists workspaces through herdr). That is the positive control — a
// validCount that refused everything would never reach herdr, and the
// accepted rows would fail. The refusal rows also assert the fix runs
// BEFORE Resolve: they never mention herdr.
func TestPeekLinesRefusesBadCount(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	env := append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
		"PATH=",
	)

	for _, c := range []struct {
		args     []string
		want     string
		accepted bool // the count is good: it must reach herdr, not die here
	}{
		{args: []string{"peek", "sess", "40x"}, want: "<lines> needs"},
		{args: []string{"peek", "sess", "forty"}, want: "<lines> needs"},
		{args: []string{"peek", "sess", "-1"}, want: "<lines> needs"},
		{args: []string{"peek", "sess", ""}, want: "<lines> needs"},
		{args: []string{"peek", "sess", "40.0"}, want: "<lines> needs"},
		// Accepted counts reach Resolve instead of dying here — including
		// 0, which stays the deliberate "whole pane" escape hatch, and the
		// no-argument form that means the same thing.
		{args: []string{"peek", "sess", "40"}, want: "herdr-must-not-run", accepted: true},
		{args: []string{"peek", "sess", "0"}, want: "herdr-must-not-run", accepted: true},
		{args: []string{"peek", "sess"}, want: "herdr-must-not-run", accepted: true},
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
		got := string(out)
		if code != 1 || !strings.Contains(got, c.want) {
			t.Errorf("posse %s: exit %d, output %q; want exit 1 containing %q",
				strings.Join(c.args, " "), code, out, c.want)
		}
		if !c.accepted && strings.Contains(got, "herdr-must-not-run") {
			t.Errorf("posse %s reached herdr — the bad count was accepted:\n%s",
				strings.Join(c.args, " "), got)
		}
	}
}

// ranger-base-vlrp: the seed config ships two `beads:` example paths a fresh
// machine does not have, and `posse beads check` used to print its all-clear
// over them. The census walk is deliberately quiet where a repo has no
// census, so "the repo is not there" read exactly like "nothing was ever
// dropped here" — the same shape the ready scan had before rangerhq-llse.
func TestBeadsCheckRefusesAMissingRepoAsACleanCensus(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	gone := filepath.Join(t.TempDir(), "projA")
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads:\n  - "+gone+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bd := writeExec(t, t.TempDir(), "bd", fakeBdScript)

	cmd := exec.Command(bin, "beads", "check")
	cmd.Env = readyEnv(t, home, "RHQ_BD_BIN="+bd)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("a census that could not be taken must not exit 0:\n%s", out)
	}
	got := string(out)
	// ranger-base-33vp: every path posse prints goes through
	// rhq.AbbrevHome — here via ScanError.Error — so the assertion is that
	// rendering, not the raw absolute path. Asserting the absolute form
	// passed only where t.TempDir() landed OUTSIDE $HOME; under
	// scripts/test-linux.sh (HOME=/tmp, no TMPDIR) it never does, and the
	// leg was red on every box. readyEnv keeps os.Environ(), so the child
	// abbreviates against the same $HOME this process does.
	if want := rhq.AbbrevHome(gone); !strings.Contains(got, want) {
		t.Errorf("want the unresolvable path named as %s, got:\n%s", want, got)
	}
	if !strings.Contains(got, "unknown, not clean") {
		t.Errorf("want the census named unknown, got:\n%s", got)
	}
	if strings.Contains(got, "every id git ever carried still resolves") {
		t.Errorf("a repo that is not there must never read as an all-clear:\n%s", got)
	}
}

// The quieter variant, and the one that outlives the first day: one good
// path and one typo. Half the census is invisible, and used to stay
// invisible — the good repo answered and the command said all-clear.
func TestBeadsCheckNamesATypoBesideARealRepo(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	good := t.TempDir()
	typo := filepath.Join(t.TempDir(), "projB-typo")
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads:\n  - "+good+"\n  - "+typo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bd := writeExec(t, t.TempDir(), "bd", fakeBdScript)

	cmd := exec.Command(bin, "beads", "check")
	cmd.Env = readyEnv(t, home, "RHQ_BD_BIN="+bd)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("one unresolvable path is still an incomplete census:\n%s", out)
	}
	got := string(out)
	// ranger-base-33vp: home-abbreviated, as above.
	if want := rhq.AbbrevHome(typo); !strings.Contains(got, want) {
		t.Errorf("want the typo named as %s, got:\n%s", want, got)
	}
	if !strings.Contains(got, "1 repo(s) that resolved") {
		t.Errorf("want the partial census said out loud, got:\n%s", got)
	}
}

// The positive control: every configured path resolves, so the all-clear is
// the truth and still gets printed at exit 0.
func TestBeadsCheckStillAllClearsWhenEveryRepoResolves(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bd := writeExec(t, t.TempDir(), "bd", fakeBdScript)

	cmd := exec.Command(bin, "beads", "check")
	cmd.Env = readyEnv(t, home, "RHQ_BD_BIN="+bd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("a repo that resolves is not an error: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "every id git ever carried still resolves") {
		t.Errorf("want the all-clear intact:\n%s", out)
	}
}

// ranger-base-xotg: `posse ready` prints ONE queue, ordered by priority
// across every configured source. It used to print each repo's `bd ready`
// output concatenated, so priority held inside a source and not across it:
// the second source's P1 landed behind the first source's P3s, which is how
// an operator raising a bead to P1 watched it move BACKWARD in the list.
func TestReadyOrdersByPriorityAcrossSources(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	first, second := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"),
		[]byte("beads:\n  - "+first+"\n  - "+second+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Each source hands back its own beads in its own order — bd's order is
	// the query's, not a queue's.
	if err := os.WriteFile(filepath.Join(first, "fake-ready.json"), []byte(
		`[{"id":"one-p1","title":"first p1","priority":1},
		  {"id":"one-p3","title":"first p3","priority":3},
		  {"id":"one-p2","title":"first p2","priority":2}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "fake-ready.json"), []byte(
		`[{"id":"two-p3","title":"second p3","priority":3},
		  {"id":"two-p1","title":"second p1","priority":1},
		  {"id":"two-p2","title":"second p2","priority":2}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	bd := writeExec(t, t.TempDir(), "bd", fakeBdScript)

	cmd := exec.Command(bin, "ready")
	cmd.Env = readyEnv(t, home, "RHQ_BD_BIN="+bd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("two readable repos must list: %v\n%s", err, out)
	}
	got := string(out)

	line := regexp.MustCompile(`(?m)^(\S+)\s+p(\d)`)
	var ids []string
	var prios []int
	for _, m := range line.FindAllStringSubmatch(got, -1) {
		ids = append(ids, m[1])
		prios = append(prios, int(m[2][0]-'0'))
	}
	if len(ids) != 6 {
		t.Fatalf("want all 6 beads of both sources listed, got %v:\n%s", ids, got)
	}
	// The whole point: monotonic in priority, one list, sources interleaved.
	for i := 1; i < len(prios); i++ {
		if prios[i] < prios[i-1] {
			t.Fatalf("priority resets at %s (p%d after %s p%d) — the list is per-source, not a queue:\n%s",
				ids[i], prios[i], ids[i-1], prios[i-1], got)
		}
	}
	at := func(id string) int {
		for i, g := range ids {
			if g == id {
				return i
			}
		}
		t.Fatalf("%s missing from the list:\n%s", id, got)
		return -1
	}
	// The reported regression, exactly: a P1 in the second source outranks
	// every lower-priority bead of the first.
	if at("two-p1") > at("one-p2") || at("two-p1") > at("one-p3") {
		t.Errorf("raising a second-source bead to P1 must move it FORWARD, got %v:\n%s", ids, got)
	}
}

// ─── posse status (the governance surface, bead rangerhq-81y0) ───────────────

// emptyHerdrScript answers the two listings Sessions() makes with an empty
// herd. A status test needs herdr to SUCCEED and say nothing, so that a
// non-zero exit can only mean "a condition was found".
const emptyHerdrScript = `#!/bin/sh
case "$1 $2" in
  "workspace list") echo '{"workspaces":[]}' ;;
  "agent list")     echo '{"agents":[]}' ;;
  *)                echo '{}' ;;
esac
exit 0
`

func statusEnv(t *testing.T, home string, extra ...string) []string {
	t.Helper()
	herdr := writeExec(t, t.TempDir(), "herdr", emptyHerdrScript)
	bd := writeExec(t, t.TempDir(), "bd", fakeBdScript)
	return append(append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+herdr,
		"RHQ_BD_BIN="+bd,
	), extra...)
}

// The all-clear: nothing needs a human, and the exit code says so. This is
// the arm that makes the non-zero one below mean anything.
func TestStatusClearShopExitsZero(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "status")
	cmd.Env = statusEnv(t, home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("a clear shop must exit 0: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "nothing needs a human") {
		t.Errorf("want the all-clear, got:\n%s", out)
	}
}

// The design's own observable, in the shape a script would ask it: a shop
// with autostart armed and no watch loop shows G7 and exits non-zero.
func TestStatusNonZeroOnANonEmptySet(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"),
		[]byte("beads:\n  - "+repo+"\nautostart_interval: 5m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "status")
	cmd.Env = statusEnv(t, home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("a non-empty condition set must exit non-zero:\n%s", out)
	}
	got := string(out)
	if !strings.Contains(got, "URGENT") || !strings.Contains(got, "G7") {
		t.Errorf("want the G7 row, got:\n%s", got)
	}
	if strings.Contains(got, "nothing needs a human") {
		t.Errorf("a set with a condition in it is not an all-clear:\n%s", got)
	}
}

// An unreadable store is not an all-clear either — the same rule `posse
// beads check` keeps, and the one that stops this command being trusted
// while it is blind.
func TestStatusNonZeroWhenAStoreCannotBeRead(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// fakeBdScript fails every call while this marker is there.
	if err := os.WriteFile(filepath.Join(repo, "fake-ready-fail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "status")
	cmd.Env = statusEnv(t, home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("an unreadable store must not exit 0:\n%s", out)
	}
	if strings.Contains(string(out), "nothing needs a human") {
		t.Errorf("unknown is not clear:\n%s", out)
	}
}

// ─── pause / resume (ADR 0029 §3, bead rangerhq-a2g6) ────────────────────────

// pauseEnv is statusEnv plus an explicit RHQ_PERSONA. Explicit because
// os.Environ() carries whatever the shell running the suite had, and a
// persona session has that variable set: without this line the operator arm
// below would pass for the operator and refuse for every persona running
// the same suite (the class ranger-base-rp2y and the gate-shim-on-PATH one
// cost a day each).
func pauseEnv(t *testing.T, home, persona string) []string {
	t.Helper()
	return statusEnv(t, home, "RHQ_PERSONA="+persona)
}

// The command half of §3's first observable, end to end: the file, the two
// lines, the standing-pause rule, and an idempotent resume.
func TestPauseAndResumeCommands(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"),
		[]byte("beads:\n  - "+repo+"\ncoordinator: coordinator\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(persona string, args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Env = pauseEnv(t, home, persona)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	pausePath := filepath.Join(home, "state", "pause.yaml")

	// Resume before any pause: not an error, and it says so.
	if out, err := run("", "resume"); err != nil || !strings.Contains(out, "not paused") {
		t.Fatalf("resume on an unpaused shop = %q, %v", out, err)
	}

	// The why need not be one shell word — a stop is typed in a hurry.
	out, err := run("", "pause", "waiting on the", "operator")
	if err != nil {
		t.Fatalf("pause: %v\n%s", err, out)
	}
	for _, want := range []string{"paused", rhq.PauseOperator, "waiting on the operator", "the pulse keeps ticking"} {
		if !strings.Contains(out, want) {
			t.Errorf("pause must say %q:\n%s", want, out)
		}
	}
	file, err := os.ReadFile(pausePath)
	if err != nil {
		t.Fatalf("no pause file: %v", err)
	}
	for _, want := range []string{"by: " + rhq.PauseOperator, "at: 20", "why: waiting on the operator"} {
		if !strings.Contains(string(file), want) {
			t.Errorf("state/pause.yaml must carry %q:\n%s", want, file)
		}
	}

	// A why is mandatory, and the usage says so rather than stopping the
	// shop for an unrecorded reason.
	if out, err := run("", "pause"); err == nil || !strings.Contains(out, "why") {
		t.Errorf("pause with no reason = %q, %v — want the usage", out, err)
	}

	// The condition set sees it, which is the whole G8 row.
	if out, _ := run("", "status"); !strings.Contains(out, "G8") || !strings.Contains(out, "waiting on the operator") {
		t.Errorf("posse status must report the pause:\n%s", out)
	}

	// A second pause keeps the first: overwriting would move at: forward and
	// lose the reason the shop actually stopped for.
	out, err = run("", "pause", "something else")
	if err != nil {
		t.Fatalf("a second pause is not an error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "already paused") || !strings.Contains(out, "waiting on the operator") {
		t.Errorf("the standing pause must be kept and named:\n%s", out)
	}
	if after, _ := os.ReadFile(pausePath); string(after) != string(file) {
		t.Errorf("a second pause rewrote the file:\n%s", after)
	}

	// Only the operator and the coordinator.
	if out, err := run("developer", "pause", "no"); err == nil || !strings.Contains(out, "refused") {
		t.Errorf("a stranger paused the shop: %q, %v", out, err)
	}

	// And resume lifts it, naming what it lifted.
	out, err = run("coordinator", "resume")
	if err != nil {
		t.Fatalf("resume: %v\n%s", err, out)
	}
	if !strings.Contains(out, "resumed by coordinator") || !strings.Contains(out, "waiting on the operator") {
		t.Errorf("resume must name who lifted it and what:\n%s", out)
	}
	if _, err := os.Stat(pausePath); !os.IsNotExist(err) {
		t.Errorf("the pause file survived resume: %v", err)
	}
	if out, err := run("", "resume"); err != nil || !strings.Contains(out, "not paused") {
		t.Fatalf("the second resume = %q, %v", out, err)
	}
}

// rangerhq-sk6p: `posse prompt` on a session this home holds no meta for
// warns that the crew mark did not land. Same defect as cockpit `p`, other
// entry point — and this one runs the built binary because the contract is
// what the operator SEES on stdout, not what MarkCrew returns.
func TestPromptWarnsWhenTheCrewMarkCannotBeRecorded(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	binDir := t.TempDir()
	herdr := filepath.Join(binDir, "herdr")
	if err := os.WriteFile(herdr, []byte(`#!/bin/sh
case "$1 $2" in
"workspace list")
  printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1","label":"owned","agent_status":"idle"},{"workspace_id":"w2","label":"stranger","agent_status":"idle"}]}}'
  exit 0;;
"agent list")
  printf '%s\n' '{"result":{"agents":[{"agent":"claude","agent_status":"idle","pane_id":"p1","workspace_id":"w1"},{"agent":"claude","agent_status":"idle","pane_id":"p2","workspace_id":"w2"}]}}'
  exit 0;;
"agent explain")
  # A pane herdr HAS recognized: the readiness gate (ranger-base-3p0) opens
  # on this and the crew mark is the only thing under test here.
  printf '%s\n' '{"state":"idle","matched_rule":{"id":"live_prompt_box","state":"idle"},"visible_idle":true,"fallback_reason":null}'
  exit 0;;
"agent prompt")
  printf '%s\n' '{"result":{"submitted":true}}'
  exit 0;;
esac
printf '%s\n' '{"error":{"code":"no","message":"unexpected '"$1 $2"'"}}'
exit 1
`), 0o755); err != nil {
		t.Fatal(err)
	}
	metaDir := filepath.Join(home, "state", "herdr")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "owned.yaml"),
		[]byte("name: owned\nworkspace: w1\npane: p1\nemoji: 🙂\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(name string) string {
		t.Helper()
		cmd := exec.Command(bin, "prompt", name, "hello")
		cmd.Env = []string{
			"HOME=" + t.TempDir(),
			"RHQ_HOME=" + home,
			"RHQ_HERDR_BIN=" + herdr,
			// ServerGen() would otherwise stat the operator's live socket
			// (ranger-base-ouf9).
			"HERDR_SOCKET_PATH=" + filepath.Join(home, "no-such.sock"),
			"PATH=" + os.Getenv("PATH"),
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("posse prompt %s: %v\n%s", name, err, out)
		}
		return string(out)
	}

	got := run("stranger")
	for _, want := range []string{"warning:", "NOT recorded", "no session meta", "ADR 0008"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompting a foreign session must warn %q:\n%s", want, got)
		}
	}

	// The control: a session with a meta is marked, and warns about nothing.
	if got := run("owned"); strings.Contains(got, "warning") {
		t.Errorf("prompting an owned session must warn about nothing:\n%s", got)
	}
	if b, err := os.ReadFile(filepath.Join(metaDir, "owned.yaml")); err != nil || !strings.Contains(string(b), "crew: true") {
		t.Errorf("the control's mark was not recorded (%v): %s", err, b)
	}
}

// ranger-base-3p0: `posse prompt` into a session whose CLI had not taken the
// keyboard typed the work prompt at whatever had — a leading '/' turned the
// dispatch marker into `/Work` and the rest into its arguments, and herdr
// returned success. The gate is unit-pinned in internal/rhq
// (promptready_test.go); this runs the BINARY, because what the bead is
// about is what the operator's `posse prompt` does — a gate nothing calls
// is the regression this catches.
func TestPromptRefusesAPaneHerdrHasNotRecognized(t *testing.T) {
	bin := buildRhq(t)

	// seen: the explain shape a settled pane has (a matched rule) vs the
	// GUESS a booting one gets. The lever is a file the fake reads, so one
	// herdr script serves both arms and the difference is the only variable.
	setup := func(t *testing.T, seen bool) (string, string, []string) {
		t.Helper()
		home := t.TempDir()
		binDir := t.TempDir()
		herdr := filepath.Join(binDir, "herdr")
		log := filepath.Join(binDir, "calls.log")
		explain := `{"state":"idle","matched_rule":null,"visible_idle":false,` +
			`"fallback_reason":"default_known_agent_idle_fallback",` +
			`"evaluated_rules":[{"id":"live_prompt_box","matched":false,"region":"osc_title",` +
			`"state":"idle","evidence":{"region_bytes":0,"region_preview":""}}]}`
		if seen {
			explain = `{"state":"idle","matched_rule":{"id":"live_prompt_box","state":"idle"},` +
				`"visible_idle":true,"fallback_reason":null}`
		}
		if err := os.WriteFile(herdr, []byte(`#!/bin/sh
echo "$*" >> `+log+`
case "$1 $2" in
"workspace list")
  printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1","label":"boot","agent_status":"idle"}]}}'
  exit 0;;
"agent list")
  printf '%s\n' '{"result":{"agents":[{"agent":"claude","agent_status":"idle","pane_id":"p1","workspace_id":"w1"}]}}'
  exit 0;;
"agent explain")
  printf '%s\n' '`+explain+`'
  exit 0;;
"agent prompt")
  printf '%s\n' '{"result":{"submitted":true}}'
  exit 0;;
esac
printf '%s\n' '{"error":{"code":"no","message":"unexpected '"$1 $2"'"}}'
exit 1
`), 0o755); err != nil {
			t.Fatal(err)
		}
		// A runtime with a short declared patience, so the refusal costs the
		// suite 400ms instead of the claude-shaped 45s. Same lever a slow CLI
		// uses in production.
		if err := os.MkdirAll(filepath.Join(home, "runtimes"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, "runtimes", "fastcli.yaml"),
			[]byte("command: fastcli --sys {file}\nstartup_wait: 400ms\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		metaDir := filepath.Join(home, "state", "herdr")
		if err := os.MkdirAll(metaDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(metaDir, "boot.yaml"),
			[]byte("name: boot\nworkspace: w1\npane: p1\nemoji: 🙂\nruntime: fastcli\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return log, home, []string{
			"HOME=" + t.TempDir(),
			"RHQ_HOME=" + home,
			"RHQ_HERDR_BIN=" + herdr,
			"HERDR_SOCKET_PATH=" + filepath.Join(home, "no-such.sock"),
			"PATH=" + os.Getenv("PATH"),
		}
	}
	sent := func(t *testing.T, log string) bool {
		t.Helper()
		b, _ := os.ReadFile(log)
		return strings.Contains(string(b), "agent prompt")
	}

	t.Run("guess refuses and types nothing", func(t *testing.T) {
		log, _, env := setup(t, false)
		cmd := exec.Command(bin, "prompt", "boot", "Work beads issue x")
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("a prompt into an unrecognized pane must fail:\n%s", out)
		}
		for _, want := range []string{"nothing was sent", "boot", "posse peek boot", "--now"} {
			if !strings.Contains(string(out), want) {
				t.Errorf("the refusal must name %q:\n%s", want, out)
			}
		}
		if sent(t, log) {
			t.Errorf("the text went to herdr anyway — that is the bug:\n%s", out)
		}
	})

	// The escape hatch, for a runtime whose rules cannot see its own screen.
	t.Run("--now sends it anyway", func(t *testing.T) {
		log, _, env := setup(t, false)
		cmd := exec.Command(bin, "prompt", "boot", "Work beads issue x", "--now")
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("--now must skip the gate: %v\n%s", err, out)
		}
		if !sent(t, log) {
			t.Errorf("--now sent nothing:\n%s", out)
		}
	})

	// The wrong arm: a pane herdr HAS recognized is prompted as it always
	// was, with nothing extra said. Without this, a gate that refused
	// everything would pass the test above.
	t.Run("a seen screen is prompted", func(t *testing.T) {
		log, _, env := setup(t, true)
		cmd := exec.Command(bin, "prompt", "boot", "Work beads issue x")
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("a seen screen must be prompted: %v\n%s", err, out)
		}
		if !sent(t, log) {
			t.Errorf("the prompt never reached herdr:\n%s", out)
		}
		if strings.Contains(string(out), "waited") {
			t.Errorf("nothing was waited for; the gate must be silent:\n%s", out)
		}
	})
}
