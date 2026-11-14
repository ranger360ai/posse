package treepins

// QA pins for ranger-base-41l0r — the two read-only modes of
// scripts/verify-gate-freshness.sh that `posse upgrade`'s gate copy arm
// reads the box through (ADR 0092 D5).
//
// WHAT CAN GO WRONG HERE AND NOWHERE ELSE. The arm asks that script two
// questions: WHERE the operator's PreToolUse hook reaches
// (`--print-wrappers`) and WHETHER the gate installed there works
// (`--behaviour-only`). Both answers are read off the script's exit code and
// stdout, and the script answers an unknown flag with a usage line and exit
// 2 — which the arm renders as `unknown`. So a rename of either mode, in
// either direction, leaves every install reporting `gate unknown` with the
// box's copy never refreshed, and nothing in Go can see it: internal/posse
// stubs the two seams (upgradegate_test.go), and no grep over two files can
// tell that one spells a flag the other does not accept.
//
// This is the shape TestQAHookFreshnessAcceptsTheFlagsTheInstallRecipePasses
// already holds one directory over, with the Makefile's recipe line in place
// of a Go call site: the flags are EXTRACTED from the caller and then handed
// to the real script, so the two cannot drift without one of them reddening.
//
// AND THE VERDICTS, because "it accepts the flag" is not the claim the arm
// rests on. `--print-wrappers` must print the wrapper the settings name and
// nothing else, so the arm has a destination; `--behaviour-only` must exit 0
// over a gate that refuses `bd daemon stop` through the parser and 1 over
// one that refuses nothing, because that second answer is what puts both
// .prev copies back.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// gamSeamFile is the arm's own source — read by NAME, because what this pin
// needs from it is the two argv it builds and not a reading of the tree.
const gamSeamFile = "internal/posse/upgradegate.go"

// gamFlag matches a quoted long flag.
var gamFlag = regexp.MustCompile(`"(--[a-z-]+)"`)

// gamSeamFlags returns the flag lists the arm passes to the verifier, in
// source order: one list per call site that names upgradeGateScript.
//
// Keyed on that helper rather than on the script's filename, because the
// helper is the one place the arm spells the path — a second call site added
// tomorrow is matched by this, and a grep for the script name would also
// match the prose in that file's header.
func gamSeamFlags(t *testing.T) [][]string {
	t.Helper()
	body, err := os.ReadFile(filepath.FromSlash(gamSeamFile))
	if err != nil {
		t.Fatalf("reading %s: %v", gamSeamFile, err)
	}
	var out [][]string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") || !strings.Contains(line, "upgradeGateScript(wt)") {
			continue
		}
		var flags []string
		for _, m := range gamFlag.FindAllStringSubmatch(line, -1) {
			flags = append(flags, m[1])
		}
		if len(flags) > 0 {
			out = append(out, flags)
		}
	}
	if len(out) != 2 {
		t.Fatalf("%s has %d flag-passing call sites of upgradeGateScript, want 2 (the resolution and the behaviour reading) — this pin is measuring something other than the arm's two modes: %v", gamSeamFile, len(out), out)
	}
	return out
}

// ─── the rig ─────────────────────────────────────────────────────────────────

// gamGate is a stub gate that refuses `bd daemon` the way the parser does —
// exit 0 with a deny decision on stdout — and stays out of the way of
// everything else. It is a stub and not the real gate because what is under
// test is the VERIFIER: a real gate would make the deny arm's green reading
// a statement about two things at once, and the one that matters here is
// which answer the script turns into which exit code.
const gamGate = `#!/bin/sh
payload=$(cat)
case "$payload" in
  *'bd daemon'*) printf '{"hookSpecificOutput":{"permissionDecision":"deny"}}' ;;
esac
exit 0
`

// gamOpenGate is the failure the deny arm exists to catch: every call comes
// back with nothing to see, so the fence is open.
const gamOpenGate = "#!/bin/sh\ncat >/dev/null\nexit 0\n"

type gamRig struct {
	cfg     string // the CLAUDE_CONFIG_DIR the hook is read out of
	dir     string // the gate dir the hook names
	wrapper string
}

func gamNewRig(t *testing.T, wrapper string) *gamRig {
	t.Helper()
	r := &gamRig{cfg: t.TempDir(), dir: t.TempDir()}
	r.wrapper = filepath.Join(r.dir, "bd-argv-gate.sh")
	gamPlant(t, r.wrapper, wrapper)
	gamPlant(t, filepath.Join(r.dir, "bd-argv-gate.py"), "# a stub parser\n")

	// The settings carry a shell COMMAND, not a path, and the quoting is
	// the shape the operator's own entry uses — which is the whole reason
	// the script reads it with shlex rather than taking the string.
	settings := map[string]any{"hooks": map[string]any{"PreToolUse": []any{
		map[string]any{"matcher": "Bash", "hooks": []any{
			map[string]any{"type": "command", "command": "'" + r.wrapper + "'"},
		}},
	}}}
	body, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.cfg, "settings.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	return r
}

// gamPlant writes one of the gate's two files. Through
// posse.WriteExecutable, which is this tree's one way to write a file
// something is about to exec: the hook execs the wrapper directly, so a
// write descriptor a sibling fork inherits is an ETXTBSY the probe would
// answer with (execwrite.go, golang/go#22315).
func gamPlant(t *testing.T, path, body string) {
	t.Helper()
	if err := posse.WriteExecutable(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// gamRun invokes the real script with one of the arm's flag lists, reading
// the operator's hook out of this rig's config dir.
//
// A FUNCTION AND NOT A METHOD ON gamRig. It had to be when it was written:
// twdParseDir skipped every declaration with a receiver, so an exec.Command
// inside a method was invisible to the census that holds every tracked
// program a pin runs to a disposition — and this pin runs one. That hole is
// closed (ranger-base-b3w9x, treewidedoor_qa_test.go): methods are read and
// the census asks every declaration, not only the tests. The shape is kept
// because the OTHER half of the register still needs it — the reach closure
// follows *ast.Ident calls, so a reading propagates to its callers from a
// plain function and from nothing else.
func gamRun(t *testing.T, r *gamRig, flags ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("scripts/verify-gate-freshness.sh", flags...)
	cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+r.cfg)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running the verifier with %v: %v", flags, err)
	}
	return string(out), code
}

// ─── the modes ───────────────────────────────────────────────────────────────

// Mode 1, by execution: the resolution flag the arm passes is a flag this
// script accepts, and its answer is the wrapper path alone.
func TestQAGateArmResolutionModeAnswersTheInstalledWrapper(t *testing.T) {
	flags := gamSeamFlags(t)[0]
	r := gamNewRig(t, gamGate)
	out, code := gamRun(t, r, flags...)
	if strings.Contains(out, "usage:") {
		t.Fatalf("the gate arm passes %v, which this script does not accept:\n%s", flags, out)
	}
	if code != 0 {
		t.Fatalf("a box whose settings name an installed gate must resolve, got exit %d with %v:\n%s", code, flags, out)
	}
	// The arm reads PATHS off these lines, so a line of prose beside them
	// is a destination it might take. One line, and it is the wrapper.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 || lines[0] != r.wrapper {
		t.Errorf("%v printed %q, want exactly the one wrapper %q — the arm writes to what this prints", flags, out, r.wrapper)
	}
}

// And the no-gate box answers through the script's own "nothing measured"
// channel, which is the refusal the arm turns into `gate refused` rather
// than into a write.
func TestQAGateArmResolutionModeRefusesABoxWithNoHook(t *testing.T) {
	flags := gamSeamFlags(t)[0]
	r := gamNewRig(t, gamGate)
	if err := os.Remove(filepath.Join(r.cfg, "settings.json")); err != nil {
		t.Fatal(err)
	}
	out, code := gamRun(t, r, flags...)
	if code != 2 {
		t.Errorf("a box with no PreToolUse hook answers exit %d, want 2:\n%s", code, out)
	}
	if strings.Contains(out, r.wrapper) {
		t.Errorf("the refusal names a wrapper path, which the arm would write to:\n%s", out)
	}
	if !strings.Contains(out, "nothing measured") {
		t.Errorf("the refusal does not go through this script's own channel for it:\n%s", out)
	}
}

// Mode 2, by execution: the behaviour flags the arm passes are flags this
// script accepts, and the three arms answer 0 over a gate that works.
//
// It must also read NO reference: the arm runs this script out of a
// throwaway worktree of the commit being installed, and a mode that
// compared bytes against a checkout's HEAD would answer a question about
// that tree while claiming to answer one about the box.
func TestQAGateArmBehaviourModeHoldsOverAWorkingGate(t *testing.T) {
	flags := append(gamSeamFlags(t)[1], "") // the --wrapper value
	r := gamNewRig(t, gamGate)
	flags[len(flags)-1] = r.wrapper
	out, code := gamRun(t, r, flags...)
	if strings.Contains(out, "usage:") {
		t.Fatalf("the gate arm passes %v, which this script does not accept:\n%s", flags, out)
	}
	if code != 0 {
		t.Fatalf("the three arms do not hold over a gate that refuses `bd daemon stop` through the parser, exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "behaves") {
		t.Errorf("the report does not say the arms held:\n%s", out)
	}
	for _, unwanted := range []string{"STALE", "matches HEAD"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("behaviour-only read a reference (%q in the report) — the arm runs it from a worktree it deletes minutes later:\n%s", unwanted, out)
		}
	}
	if !strings.Contains(out, "no reference read") {
		t.Errorf("the report does not say it read no reference, so a reader cannot tell this verdict from a full one:\n%s", out)
	}
}

// And the answer the rollback hangs off: a gate that refuses nothing is
// exit 1, distinct from the exit 2 that means nothing was measured. The arm
// restores both .prev copies on 1 and leaves the install standing on 2, so a
// script that confused them would either undo a good install or leave a
// broken fence in place.
func TestQAGateArmBehaviourModeFindsAnOpenFence(t *testing.T) {
	flags := append(gamSeamFlags(t)[1], "")
	r := gamNewRig(t, gamOpenGate)
	flags[len(flags)-1] = r.wrapper
	out, code := gamRun(t, r, flags...)
	if code != 1 {
		t.Fatalf("a gate that refuses nothing answers exit %d, want 1 — this is the code that puts the box's own gate back:\n%s", code, out)
	}
	if !strings.Contains(out, "FINDING") {
		t.Errorf("the report names no finding:\n%s", out)
	}
	// The prescription must not be "reinstall these bytes": this mode never
	// compared a byte, and over a copy that IS the commit's that advice
	// cannot work.
	if strings.Contains(out, "Refresh the operator's copy") {
		t.Errorf("behaviour-only prescribes a reinstall it has not measured the need for:\n%s", out)
	}
}

// A gate whose wrapper is not there at all is a finding and not a crash:
// the arm refuses that shape before it ever gets here, but this script is
// also the detective control a human runs, and an exit it has no word for
// would read as `unknown` to the arm and as nothing at all to the human.
func TestQAGateArmBehaviourModeReportsAMissingWrapper(t *testing.T) {
	flags := append(gamSeamFlags(t)[1], "")
	r := gamNewRig(t, gamGate)
	if err := os.Remove(r.wrapper); err != nil {
		t.Fatal(err)
	}
	flags[len(flags)-1] = r.wrapper
	out, code := gamRun(t, r, flags...)
	if code != 1 {
		t.Errorf("a hook naming a file that is not there answers exit %d, want 1:\n%s", code, out)
	}
	if !strings.Contains(out, "not there") {
		t.Errorf("the report does not say the file is not there:\n%s", out)
	}
}
