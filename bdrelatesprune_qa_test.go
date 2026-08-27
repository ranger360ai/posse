package posse

// QA pins for ranger-base-nusr.
//
// The finding: bd 0.49.1's non-atomic `create --deps` is not a socket problem.
// Measured on a VACUUM INTO snapshot of the fleet db, direct storage mode, no
// daemon and therefore no socket at all: `bd --no-daemon create ... --deps
// discovered-from:ranger-base-okbr` was killed at 90s with the issue row
// COMMITTED and the dependency absent. The 30s timeout dinesh saw through the
// daemon is incidental — raising it buys a longer hang, not an edge. The one
// thing that fixes it is removing the symmetric pairs: after the prune, the
// same create against okbr, x6ic and cpyb ran in 0.38-0.41s with the edge
// present.
//
// So the prune is the fix, and these pin the two tools that make it hold:
// `verify-bd-dep-safety.sh --gate` (drift detector) and
// `prune-bd-relates-to.sh` (the prune itself, dry by default).
//
// Also pinned here: the prune is DRY unless `--apply` is typed. Pruning the
// fleet store is a deletion on live state; a `make` target must never do it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// prunRun runs a repo script with cwd=root and BEADS_DB stripped, like bdsRun.
func prunRun(t *testing.T, script, root string, env []string, args ...string) (string, int) {
	t.Helper()
	abs, err := filepath.Abs(script)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(abs, args...)
	cmd.Dir = root
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
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("%s: %v\n%s", script, err, out)
		}
		code = ee.ExitCode()
	}
	return string(out), code
}

// prunSeed writes root/.beads/beads.db from the given SQL and returns root.
func prunSeed(t *testing.T, seed string) string {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not on PATH")
	}
	root := t.TempDir()
	beads := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beads, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sqlite3", filepath.Join(beads, "beads.db"))
	cmd.Stdin = strings.NewReader(`
CREATE TABLE dependencies (
  issue_id TEXT NOT NULL,
  depends_on_id TEXT NOT NULL,
  type TEXT NOT NULL DEFAULT 'blocks',
  PRIMARY KEY (issue_id, depends_on_id, type)
);
` + seed)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seed: %v\n%s", err, out)
	}
	return root
}

const prunPair = `
INSERT INTO dependencies VALUES ('a','b','relates-to'), ('b','a','relates-to');
INSERT INTO dependencies VALUES ('c','a','blocks'), ('d','e','blocks');
`

func TestQABdDepSafetyGateFailsOnAnyPair(t *testing.T) {
	const script = "scripts/verify-bd-dep-safety.sh"

	out, code := prunRun(t, script, prunSeed(t, prunPair), nil, "--gate")
	if code != 1 {
		t.Fatalf("--gate must fail while a pair exists: exit %d\n%s", code, out)
	}
	// It has to name the nodes, or nobody can act on it.
	for _, want := range []string{"UNSAFE:", "\n  a\n", "\n  b\n", "prune-bd-relates-to.sh"} {
		if !strings.Contains(out, want) {
			t.Errorf("--gate output missing %q\n%s", want, out)
		}
	}
	// c only *reaches* the pair; it is not in one, so the gate does not list it.
	if strings.Contains(out, "\n  c\n") {
		t.Errorf("--gate listed a reacher as a pair member\n%s", out)
	}

	// The pruned store is the steady state this whole bead exists to reach.
	out, code = prunRun(t, script, prunSeed(t, "INSERT INTO dependencies VALUES ('c','a','blocks');"), nil, "--gate")
	if code != 0 {
		t.Fatalf("--gate must pass with no pair: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "clean:") {
		t.Errorf("--gate did not report clean\n%s", out)
	}
}

// A WAL-mode db whose -shm is gone cannot be opened `mode=ro` at all; sqlite
// returns CANTOPEN(14). bd leaves the store in exactly that state after a
// write when no daemon holds it, and the audit used to die with a bare sqlite
// error there — a checker that errors instead of answering is a checker that
// gets ignored.
func TestQABdDepSafetyReadsAWALDBWithNoShm(t *testing.T) {
	root := prunSeed(t, prunPair)
	db := filepath.Join(root, ".beads", "beads.db")
	if out, err := exec.Command("sqlite3", db, "PRAGMA journal_mode=WAL; INSERT INTO dependencies VALUES ('x','y','blocks');").CombinedOutput(); err != nil {
		t.Fatalf("switch to WAL: %v\n%s", err, out)
	}
	// sqlite3 checkpoints and removes -wal/-shm on a clean close; assert the
	// db really is WAL-mode and that a read-only open of it fails, so this
	// test is pinning the fallback and not a no-op.
	mode, err := exec.Command("sqlite3", db, "PRAGMA journal_mode;").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(mode)) != "wal" {
		t.Skipf("db did not stay in WAL mode (%q); nothing to pin", strings.TrimSpace(string(mode)))
	}
	if err := exec.Command("sqlite3", "file:"+db+"?mode=ro", "SELECT 1").Run(); err == nil {
		t.Skip("this sqlite build opens a shm-less WAL db read-only; nothing to pin")
	}

	out, code := prunRun(t, "scripts/verify-bd-dep-safety.sh", root, nil, "--gate")
	if code != 1 {
		t.Fatalf("gate on a shm-less WAL db: exit %d, want 1\n%s", code, out)
	}
	if strings.Contains(out, "unable to open database") {
		t.Errorf("gate leaked a raw sqlite error instead of reading a copy\n%s", out)
	}
}

func TestQAPruneRelatesToIsDryUnlessApplyIsTyped(t *testing.T) {
	const script = "scripts/prune-bd-relates-to.sh"
	root := prunSeed(t, prunPair)
	db := filepath.Join(root, ".beads", "beads.db")

	out, code := prunRun(t, script, root, nil)
	if code != 0 {
		t.Fatalf("dry run: exit %d\n%s", code, out)
	}
	for _, want := range []string{"symmetric relates-to pairs: 1", "rows to delete: 2", "bd dep unrelate a b", "dry run — nothing changed"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run missing %q\n%s", want, out)
		}
	}

	// The pair is still there. A dry run that mutates is the whole nightmare.
	left, err := exec.Command("sqlite3", db, "SELECT count(*) FROM dependencies WHERE type='relates-to';").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(left)) != "2" {
		t.Errorf("dry run changed the db: %s relates-to rows left, want 2", strings.TrimSpace(string(left)))
	}

	// Nothing to do is success, not an error — the steady state after a prune.
	out, code = prunRun(t, script, prunSeed(t, "INSERT INTO dependencies VALUES ('c','a','blocks');"), nil)
	if code != 0 || !strings.Contains(out, "nothing to prune") {
		t.Errorf("clean store: exit %d\n%s", code, out)
	}
}

// `bd dep unrelate` only speaks relates_to. A symmetric pair of another type is
// the same landmine but not this script's to guess at, so it refuses loudly
// rather than reporting a prune it did not do.
func TestQAPruneRelatesToRefusesAForeignPairAndABadFlag(t *testing.T) {
	const script = "scripts/prune-bd-relates-to.sh"

	out, code := prunRun(t, script, prunSeed(t, "INSERT INTO dependencies VALUES ('a','b','blocks'), ('b','a','blocks');"), nil)
	if code != 2 {
		t.Fatalf("foreign pair: exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "blocks a b") {
		t.Errorf("refusal does not name the pair\n%s", out)
	}

	out, code = prunRun(t, script, prunSeed(t, prunPair), nil, "--yolo")
	if code != 2 || !strings.Contains(out, "usage:") {
		t.Errorf("unknown flag: exit %d\n%s", code, out)
	}

	// No db is "I could not check", never a green "nothing to prune".
	out, code = prunRun(t, script, t.TempDir(), nil)
	if code != 2 {
		t.Fatalf("missing db: exit %d, want 2\n%s", code, out)
	}
	if strings.Contains(out, "nothing to prune") {
		t.Errorf("missing db reported clean\n%s", out)
	}
}

func TestQAPruneRelatesToMakefileWiring(t *testing.T) {
	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"verify-bd-no-relate-pairs:\n\tscripts/verify-bd-dep-safety.sh --gate\n",
		"prune-bd-relates-to:\n\tscripts/prune-bd-relates-to.sh\n",
	} {
		if !strings.Contains(string(mk), want) {
			t.Errorf("Makefile lost %q", strings.SplitN(want, ":", 2)[0])
		}
	}
	// The make target must stay dry: --apply deletes rows from the live store.
	// Recipe lines only — the surrounding comment names --apply on purpose.
	for _, line := range strings.Split(string(mk), "\n") {
		if strings.HasPrefix(line, "\t") && strings.Contains(line, "prune-bd-relates-to.sh") && strings.Contains(line, "--apply") {
			t.Errorf("a make recipe must never run the prune with --apply: %q", line)
		}
	}
	info, err := os.Stat("scripts/prune-bd-relates-to.sh")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatal("scripts/prune-bd-relates-to.sh is not executable")
	}
}
