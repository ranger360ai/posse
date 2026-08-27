package posse

// QA pins for ranger-base-pkqn.
//
// Claim: bd 0.49.1 `dep add` does not terminate when the TARGET of the edge
// can reach a symmetric `relates-to` pair, because its cycle check is a
// UNION ALL recursive CTE — walks, not nodes, depth 100, every edge type —
// and bd writes `relates-to` in both directions, so every one is a 2-cycle.
// scripts/verify-bd-dep-safety.sh names the pairs and the unsafe targets, and
// gates a single id. See NOTES.md, *beads (bd) substrate*.
//
// The fixture below is a four-node graph, not the fleet's: a<->b is the
// planted pair, c reaches it through a plain `blocks` edge, d and e do not.
// So {a,b} are the cycle nodes and {a,b,c} the unsafe targets — the arithmetic
// the script has to get right, hermetically, with no beads db in sight.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bdsFixture writes a beads-shaped sqlite db holding just the dependencies
// table the script reads, and returns its path.
func bdsFixture(t *testing.T, dir string) string {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not on PATH")
	}
	db := filepath.Join(dir, "beads.db")
	const seed = `
CREATE TABLE dependencies (
  issue_id TEXT NOT NULL,
  depends_on_id TEXT NOT NULL,
  type TEXT NOT NULL DEFAULT 'blocks',
  PRIMARY KEY (issue_id, depends_on_id, type)
);
-- the landmine: bd writes relates-to in both directions, so it is a 2-cycle
INSERT INTO dependencies VALUES ('a','b','relates-to'), ('b','a','relates-to');
-- c can reach the pair; d and e cannot
INSERT INTO dependencies VALUES ('c','a','blocks'), ('d','e','blocks');
`
	cmd := exec.Command("sqlite3", db)
	cmd.Stdin = strings.NewReader(seed)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seed fixture db: %v\n%s", err, out)
	}
	return db
}

// bdsRun runs the script from the repo with cwd=root, so `.beads` discovery is
// the fixture's and never the fleet's.
func bdsRun(t *testing.T, root string, env []string, args ...string) (string, int) {
	t.Helper()
	script, err := filepath.Abs("scripts/verify-bd-dep-safety.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(script, args...)
	cmd.Dir = root
	// BEADS_DB from a pane would retarget the fleet db; strip it unless the
	// case under test sets it back deliberately.
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "BEADS_DB=") {
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("verify-bd-dep-safety.sh: %v\n%s", err, out)
		}
	}
	return string(out), code
}

// bdsRoot lays out root/.beads/beads.db around the fixture.
func bdsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	beads := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beads, 0o755); err != nil {
		t.Fatal(err)
	}
	bdsFixture(t, beads)
	return root
}

func TestQABdDepSafetyNamesThePairAndTheUnsafeTargets(t *testing.T) {
	out, code := bdsRun(t, bdsRoot(t), nil)
	if code != 0 {
		t.Fatalf("audit mode must exit 0, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "nodes in a symmetric (2-cycle) dependency pair: 2") {
		t.Errorf("audit did not count the planted pair\n%s", out)
	}
	if !strings.Contains(out, "unsafe as a 'dep add' / '--deps' TARGET: 3") {
		t.Errorf("audit did not reach c through the plain blocks edge\n%s", out)
	}
	// d and e are downstream of nothing cyclic and must not be swept in.
	for _, safe := range []string{"\n  d\n", "\n  e\n"} {
		if strings.Contains(out, safe) {
			t.Errorf("audit listed a safe node %q as unsafe\n%s", strings.TrimSpace(safe), out)
		}
	}
}

func TestQABdDepSafetyGatesASingleTarget(t *testing.T) {
	root := bdsRoot(t)
	for _, tc := range []struct {
		id   string
		want int
	}{
		{"a", 1}, // in the pair
		{"b", 1}, // in the pair
		{"c", 1}, // reaches the pair — the case that actually surprises people
		{"d", 0},
		{"e", 0},
		{"never-created", 0}, // a fresh bead has no outgoing edges: always safe
	} {
		out, code := bdsRun(t, root, nil, tc.id)
		if code != tc.want {
			t.Errorf("target %q: exit %d, want %d\n%s", tc.id, code, tc.want, out)
		}
		if tc.want == 1 && !strings.Contains(out, "UNSAFE: "+tc.id) {
			t.Errorf("target %q: refusal does not name it\n%s", tc.id, out)
		}
	}
}

// The fleet's .beads is a one-hop redirect (the worktrees all point at the
// instance repo), so discovery has to follow it or the audit reads nothing.
func TestQABdDepSafetyFollowsTheBeadsRedirect(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	bdsFixture(t, real)
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".beads", "redirect"), []byte(real+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := bdsRun(t, root, nil, "c")
	if code != 1 {
		t.Fatalf("redirect not followed: exit %d, want 1\n%s", code, out)
	}
}

func TestQABdDepSafetyBeadsDBOverrideAndMissingDB(t *testing.T) {
	root := bdsRoot(t)
	db := filepath.Join(root, ".beads", "beads.db")

	// BEADS_DB wins over discovery, from any cwd.
	out, code := bdsRun(t, t.TempDir(), []string{"BEADS_DB=" + db}, "a")
	if code != 1 {
		t.Fatalf("BEADS_DB override: exit %d, want 1\n%s", code, out)
	}

	// No db is exit 2 — "I could not check", never a green "safe".
	out, code = bdsRun(t, t.TempDir(), nil, "a")
	if code != 2 {
		t.Fatalf("missing db: exit %d, want 2\n%s", code, out)
	}
	if strings.Contains(out, "safe:") {
		t.Errorf("missing db must not report safe\n%s", out)
	}
}

func TestQABdDepSafetyMakefileWiringAndMode(t *testing.T) {
	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mk), "verify-bd-dep-safety:\n\tscripts/verify-bd-dep-safety.sh\n") {
		t.Error("Makefile lost the verify-bd-dep-safety target")
	}
	info, err := os.Stat("scripts/verify-bd-dep-safety.sh")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatal("scripts/verify-bd-dep-safety.sh is not executable")
	}
}
