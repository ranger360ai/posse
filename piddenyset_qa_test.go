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
// Amended ranger-base-t2v2: the operator narrowed the two broad hook rows to
// install/uninstall in both spellings on 2026-08-29 evening (ranger-base-y5g7,
// promoted), because the broad rows deny the verb bd's OWN git hooks exec and
// so took every commit and every `git worktree add` down with them across all
// eleven crew PIDs (ranger-base-c7ek). The expected list here and the shipped
// PIDs move together; TestShippedPIDsCarryTheNarrowedHookRows is the arm that
// knows the ruling independently of the script, so they cannot revert together
// in silence.
//
// Three arms are the whole point:
//   - the SHIPPED arm. examples/agents/ is the half this repo can pin, and it
//     is the half that was already right; it is here so the next amendment
//     cannot quietly leave it behind the way the promoted half was left.
//   - the EMPTY arm. A home with no PIDs in it has measured nothing, and
//     "no findings" earned by looking at nothing is the negative-control trap
//     in NOTES. It must exit 2, not 0.
//   - the RULING arm. The shipped PIDs read directly, against the y5g7 rows
//     spelled out here, so the pin does not rest on the script and the PIDs
//     agreeing with each other.
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
	// EMPTY arm below guards against, one level up. The floor is the arm
	// count, not a token 5: ranger-base-t2v2 added six (the superseded-rows
	// arm, one per narrowed hook alternative, and the list-shape arm), and a
	// floor left behind the arms lets a deleted arm pass as a rounding error.
	if n := strings.Count(out, "self-test PASS:"); n < 11 {
		t.Errorf("--self-test reported only %d passing arms, want >= 11:\n%s", n, out)
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

// The y5g7 narrowing, pinned WITHOUT going through the script (ranger-base-
// t2v2). TestShippedPIDsCarryTheADR0015Fence runs the script against the PIDs,
// so if the expected list and the PIDs are reverted together it stays green
// over the revert — the two halves agreeing is exactly what it measures. This
// arm reads the shipped PIDs directly and knows the operator ruling on its
// own, so reverting either half alone or both together reds `make test`.
//
// Why the ruling: the broad `Bash(bd hook:*)` / `Bash(bd hooks:*)` deny the
// whole verb, and the whole verb is what beads' OWN git hooks exec — pre-commit
// on the singular spelling, the prepare-commit-msg chain on the plural. Both
// broad rows on a PID mean that persona cannot commit or add a worktree at all
// (ranger-base-c7ek), and posse closes beads by committing.
func TestShippedPIDsCarryTheNarrowedHookRows(t *testing.T) {
	want := []string{
		"\n  - Bash(bd hook install:*)\n",
		"\n  - Bash(bd hook uninstall:*)\n",
		"\n  - Bash(bd hooks install:*)\n",
		"\n  - Bash(bd hooks uninstall:*)\n",
	}
	superseded := []string{
		"\n  - Bash(bd hook:*)\n",
		"\n  - Bash(bd hooks:*)\n",
	}
	pids, err := filepath.Glob(filepath.Join("examples", "agents", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	// The positive witness again: an all-absent assertion is satisfied by
	// globbing nothing.
	if len(pids) != 9 {
		t.Fatalf("want the 9 shipped PIDs, globbed %d", len(pids))
	}
	for _, pid := range pids {
		b, err := os.ReadFile(pid)
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		for _, rule := range want {
			if !strings.Contains(body, rule) {
				t.Errorf("%s: missing the y5g7 narrowed row %q", pid, strings.TrimSpace(rule))
			}
		}
		for _, rule := range superseded {
			if strings.Contains(body, rule) {
				t.Errorf("%s: still carries the superseded row %q — that PID cannot commit", pid, strings.TrimSpace(rule))
			}
		}
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
