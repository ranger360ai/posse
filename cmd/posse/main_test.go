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
func TestPromptWaitTimeoutRefusesBadCount(t *testing.T) {
	t.Skip("ranger-base-sknr: posse prompt/wait --timeout still drop strconv.Atoi; soon/3x/-1 become 0 = herdr default (unbounded)")
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
		{[]string{"prompt", "sess", "hi", "--wait", "--timeout", "soon"}, "--timeout needs"},
		{[]string{"prompt", "sess", "hi", "--wait", "--timeout", "3x"}, "--timeout needs"},
		{[]string{"prompt", "sess", "hi", "--wait", "--timeout", "-1"}, "--timeout needs"},
		{[]string{"prompt", "sess", "hi", "--wait", "--timeout", ""}, "--timeout needs"},
		{[]string{"prompt", "sess", "hi", "--wait", "--timeout"}, "--timeout needs"},
		{[]string{"wait", "sess", "--timeout", "soon"}, "--timeout needs"},
		{[]string{"wait", "sess", "--timeout", "3x"}, "--timeout needs"},
		{[]string{"wait", "sess", "--timeout", "-1"}, "--timeout needs"},
		{[]string{"wait", "sess", "--timeout"}, "--timeout needs"},
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
		if strings.Contains(got, "herdr workspace list") {
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
