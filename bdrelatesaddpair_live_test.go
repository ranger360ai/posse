package posse

// Live pin for ranger-base-uw8g, run against the real bd rather than a
// fixture:
//
//	two `bd dep add -t relates-to` calls, in opposite directions, plant a
//	full symmetric pair. bd 0.49.1's cycle check does not consult direction,
//	so the second call is accepted.
//
//	RHQ_LIVE_BD=1 go test . -run TestLiveBdDepAddTwiceInOppositeDirectionsPlantsAPair -v
//
// This corrects the opposite claim that shipped with ranger-base-nusr and was
// written into NOTES.md, scripts/prune-bd-relates-to.sh and
// scripts/verify-bd-dep-safety.sh: "`bd dep add -t relates-to` writes a
// single row and is harmless." The first half of that measurement was right —
// one call writes one row — but the inference was not: nothing stops a
// second call in the reverse direction, and bd accepts it. See
// docs/notes.d/ranger-base-ytqd.md for the fleet-db measurement this pin
// reproduces hermetically.
//
// Env-gated and skipped by default, like the other live pins: it shells out
// to the operator's bd, which has a version and a daemon, neither of which
// belongs in a hermetic suite. Everything happens inside one t.TempDir — the
// `bd init` here is the throwaway-database case, never a repo anybody keeps.

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLiveBdDepAddTwiceInOppositeDirectionsPlantsAPair(t *testing.T) {
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("no bd on PATH")
	}
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("no sqlite3 on PATH")
	}

	root := t.TempDir()
	bd := func(args ...string) (string, error) {
		cmd := exec.Command("bd", append([]string{"--no-daemon"}, args...)...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	if out, err := bd("init", "--prefix", "uw8g"); err != nil {
		t.Skipf("bd init did not take in a throwaway repo: %v %s", err, out)
	}

	create := func(title string) string {
		out, err := bd("create", title, "-t", "task")
		if err != nil {
			t.Fatalf("bd create %q: %v\n%s", title, err, out)
		}
		for _, line := range strings.Split(out, "\n") {
			if id, ok := strings.CutPrefix(line, "✓ Created issue: "); ok {
				return strings.TrimSpace(id)
			}
		}
		t.Fatalf("bd create %q: no issue id in output\n%s", title, out)
		return ""
	}
	a := create("a")
	b := create("b")

	// The claim under test: bd accepts BOTH directions, not just the first.
	if out, err := bd("dep", "add", a, b, "-t", "relates-to"); err != nil {
		t.Fatalf("dep add %s %s -t relates-to: %v\n%s", a, b, err, out)
	}
	if out, err := bd("dep", "add", b, a, "-t", "relates-to"); err != nil {
		t.Fatalf("dep add %s %s -t relates-to (the reverse call) was refused, contradicting the pin: %v\n%s", b, a, err, out)
	}

	out, err := exec.Command("sqlite3", root+"/.beads/beads.db",
		"SELECT issue_id, depends_on_id FROM dependencies WHERE type='relates-to' ORDER BY 1;").Output()
	if err != nil {
		t.Fatalf("sqlite3 read: %v", err)
	}
	got := strings.TrimSpace(string(out))
	want := a + "|" + b + "\n" + b + "|" + a
	if got != want {
		t.Errorf("two opposite-direction dep adds did not plant a full symmetric pair:\ngot:  %q\nwant: %q", got, want)
	}
}
