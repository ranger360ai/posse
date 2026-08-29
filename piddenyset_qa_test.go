package posse

// QA pins for ranger-base-d866 — the detective control over ADR 0015 §3's
// fence, scripts/verify-pid-deny-set.sh.
//
// The defect this came from: the fence for bd's destructive and egress verbs
// lived in one repo's .claude/settings.json, which a persona dispatched into
// any other tree does not read. ADR 0015 §3 (amended 2026-08-29, ranger-base-
// u9ud) moved it into the PID, where it becomes the L1 PATH shim and claude's
// --disallowedTools and therefore travels with the session rather than with
// the repo. What that fix rests on is every PID being edited by hand and
// STAYING edited — and on 2026-08-29 the nine shipped PIDs carried the rules
// within hours while the eleven PIDs of the crew that actually dispatches
// carried none of them, with no command on the box able to say so.
//
// Two arms are the whole point:
//   - the SHIPPED arm. examples/agents/ is the half this repo can pin, and it
//     is the half that was already right; it is here so the next amendment
//     cannot quietly leave it behind the way the promoted half was left.
//   - the EMPTY arm. A home with no PIDs in it has measured nothing, and
//     "no findings" earned by looking at nothing is the negative-control trap
//     in NOTES. It must exit 2, not 0.
//
// The script's own --self-test carries the arms that need planted fixtures
// (both list spellings, the daemon/daemons substring trap, prose in the body);
// pdsSelfTest runs it here so `make test` covers the detector itself.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const pdsScript = "scripts/verify-pid-deny-set.sh"

// pdsRun executes the real script. HOME and RHQ_HOME are always overridden to
// a scratch path, so no case can fall through to the operator's live home the
// way the script's own default argument would.
func pdsRun(t *testing.T, args ...string) (string, int) {
	t.Helper()
	abs, err := filepath.Abs(pdsScript)
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	cmd := exec.Command(abs, args...)
	cmd.Env = []string{"HOME=" + scratch, "RHQ_HOME=" + scratch, "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s: %v\n%s", pdsScript, err, out)
	}
	return string(out), code
}

func TestPIDDenySetSelfTestPasses(t *testing.T) {
	out, code := pdsRun(t, "--self-test")
	if code != 0 {
		t.Fatalf("--self-test failed (code %d):\n%s", code, out)
	}
	// A self-test that printed nothing and exited 0 is the same trap the
	// EMPTY arm below guards against, one level up.
	if n := strings.Count(out, "self-test PASS:"); n < 5 {
		t.Errorf("--self-test reported only %d passing arms, want >= 5:\n%s", n, out)
	}
	if strings.Contains(out, "self-test FAIL") {
		t.Errorf("--self-test reported a failure:\n%s", out)
	}
}

// The shipped PIDs carry the set. This is the pin that would have caught the
// shipped half drifting; the promoted half lives outside this repo and is
// `scripts/verify-pid-deny-set.sh ~/.config/posse`, run by hand or at promote.
func TestShippedPIDsCarryTheADR0015Fence(t *testing.T) {
	out, code := pdsRun(t, "examples")
	if code != 0 {
		t.Fatalf("examples/agents does not carry the ADR 0015 §3 fence (code %d):\n%s", code, out)
	}
	// The positive witness: a clean report over zero PIDs is not a pass.
	if !strings.Contains(out, "scanned 9 PIDs") {
		t.Errorf("want a scan of all 9 shipped PIDs; got:\n%s", out)
	}
}

// One rule removed from one otherwise-complete PID must be named, not summed
// away. The fixture is a real shipped PID so the arm cannot pass by agreeing
// with a fixture written from the same list the script checks.
func TestPIDDenySetNamesTheOneMissingRule(t *testing.T) {
	const dropped = "  - Bash(bd admin:*)"
	src, err := os.ReadFile(filepath.Join("examples", "agents", "devops.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), dropped+"\n") {
		t.Fatalf("fixture PID no longer spells %q — the rule list moved, so this arm measures nothing", dropped)
	}
	home := t.TempDir()
	agents := filepath.Join(home, "agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	gapped := strings.Replace(string(src), dropped+"\n", "", 1)
	if err := os.WriteFile(filepath.Join(agents, "gapped.md"), []byte(gapped), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := pdsRun(t, home)
	if code != 1 {
		t.Fatalf("a PID missing one rule must exit 1, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "Bash(bd admin:*)") || !strings.Contains(out, "gapped") {
		t.Errorf("the report must name the PID and the missing rule; got:\n%s", out)
	}
}

// Nothing measured is not a pass.
func TestPIDDenySetExitsTwoWhenNothingWasRead(t *testing.T) {
	empty := t.TempDir()
	if err := os.MkdirAll(filepath.Join(empty, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		home string
	}{
		{"agents dir with no PIDs", empty},
		{"home with no agents dir", t.TempDir()},
	} {
		out, code := pdsRun(t, tc.home)
		if code != 2 {
			t.Errorf("%s: want exit 2, got %d:\n%s", tc.name, code, out)
		}
		if strings.Contains(out, "every PID carries") {
			t.Errorf("%s: reported a clean fence having read nothing:\n%s", tc.name, out)
		}
	}
}
