package posse

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pairSeed writes dir/.beads/beads.db from the given SQL and returns dir —
// the same shape bdrelatesprune_qa_test.go's prunSeed uses for the shell
// script, so the two fixtures stay comparable.
func pairSeed(t *testing.T, dir, seed string) {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not on PATH")
	}
	beads := filepath.Join(dir, ".beads")
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
}

func TestPairCheckNoDatabaseIsNilNotAFinding(t *testing.T) {
	t.Parallel()
	pairs, err := PairCheck(t.TempDir())
	if err != nil || pairs != nil {
		t.Fatalf("a dir with no beads.db yet must read as no-finding, not a failure: pairs=%v err=%v", pairs, err)
	}
}

func TestPairCheckFindsASymmetricPairAndIgnoresAMereReacher(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pairSeed(t, dir, `
INSERT INTO dependencies VALUES ('a','b','relates-to'), ('b','a','relates-to');
INSERT INTO dependencies VALUES ('c','a','blocks'), ('d','e','blocks');
`)
	pairs, err := PairCheck(dir)
	if err != nil {
		t.Fatalf("PairCheck: %v", err)
	}
	var ids []string
	for _, p := range pairs {
		ids = append(ids, p.ID)
	}
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("want [a b], got %v", ids)
	}
}

func TestPairCheckCleanWithOnlyAReacher(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pairSeed(t, dir, `INSERT INTO dependencies VALUES ('c','a','blocks');`)
	pairs, err := PairCheck(dir)
	if err != nil {
		t.Fatalf("PairCheck: %v", err)
	}
	if len(pairs) != 0 {
		t.Fatalf("want no pairs, got %v", pairs)
	}
}

func TestPairCheckDifferentTypesDoNotPair(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Same two nodes, opposite directions, but NOT the same type — the gate
	// query requires a.type = b.type, so this must not read as a pair.
	pairSeed(t, dir, `INSERT INTO dependencies VALUES ('a','b','blocks'), ('b','a','discovered-from');`)
	pairs, err := PairCheck(dir)
	if err != nil {
		t.Fatalf("PairCheck: %v", err)
	}
	if len(pairs) != 0 {
		t.Fatalf("a pair of differing types must not be reported, got %v", pairs)
	}
}

// A WAL-mode db whose -shm is gone cannot be opened `mode=ro` at all; sqlite
// returns CANTOPEN(14). bd leaves the store in exactly that state after a
// write when no daemon holds it (verify-bd-dep-safety.sh's own comment on
// the same fallback). PairCheck must fall back to a copy rather than answer
// "clean" over a store it never actually read (ranger-base-z3s3's own
// requirement, "what it must not do").
func TestPairCheckReadsAWALDBWithNoShm(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pairSeed(t, dir, `
INSERT INTO dependencies VALUES ('a','b','relates-to'), ('b','a','relates-to');
`)
	db := filepath.Join(dir, ".beads", "beads.db")
	if out, err := exec.Command("sqlite3", db, "PRAGMA journal_mode=WAL; INSERT INTO dependencies VALUES ('x','y','blocks');").CombinedOutput(); err != nil {
		t.Fatalf("switch to WAL: %v\n%s", err, out)
	}
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

	pairs, err := PairCheck(dir)
	if err != nil {
		t.Fatalf("a shm-less WAL db must still answer via the copy fallback, got error: %v", err)
	}
	var ids []string
	for _, p := range pairs {
		ids = append(ids, p.ID)
	}
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("want [a b] read through the copy fallback, got %v", ids)
	}
}

// No sqlite3 on PATH is could-not-check, never clean.
func TestPairCheckUnavailableWithoutSqlite3(t *testing.T) {
	dir := t.TempDir()
	pairSeed(t, dir, `INSERT INTO dependencies VALUES ('a','b','relates-to'), ('b','a','relates-to');`)

	empty := t.TempDir()
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", empty)
	defer os.Setenv("PATH", oldPath)

	pairs, err := PairCheck(dir)
	if err == nil {
		t.Fatalf("no sqlite3 on PATH must not read as clean, got pairs=%v", pairs)
	}
	var unavail PairCheckUnavailableError
	if !errors.As(err, &unavail) {
		t.Fatalf("want PairCheckUnavailableError, got %T: %v", err, err)
	}
}
