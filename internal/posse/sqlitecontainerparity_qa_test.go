//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0074 D1: at `cage: container`, over a store holding a SQLite
// database, the tier is a gate CheckParityIn cannot realize — a caged bd
// there is `--no-db`, and nothing imports its writes since bd 0.50 deleted
// the daemon (ADR 0056), so the host's next flush erases them. These pin
// applyContainerSQLiteGate's one fact (parity.go) against the four shapes
// the bead names: a SQLite store under container, a no-db store under
// container, metadata.json naming another file, and a tier where the gate
// does not apply at all.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adr0074Row pulls the ADR 0074 Degraded line out of a matrix, "" when it
// is not there.
func adr0074Row(p Parity) string {
	for _, line := range p.Degraded {
		if strings.HasPrefix(line, "container tier: ") {
			return line
		}
	}
	return ""
}

func writeNoDBStore(t *testing.T, dir string) {
	t.Helper()
	home := filepath.Join(dir, beadsDirName)
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("no-db: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, beadsJSONL), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSQLiteStore(t *testing.T, dir, dbFile string, meta map[string]string) {
	t.Helper()
	home := filepath.Join(dir, beadsDirName)
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, dbFile), []byte("not a real sqlite file, just a stat target"), 0o644); err != nil {
		t.Fatal(err)
	}
	if meta != nil {
		b, err := json.Marshal(meta)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, "metadata.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// container cage + a store holding beads.db -> the Degraded row carries
// ADR 0074's wording, and dispatch-never-degrades (TestDispatchRefusesDegradedUnlessAllowed's
// shape) applies the same way it does to every other unrealized gate.
func TestContainerSQLiteGateDegradesOverABeadsDB(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeSQLiteStore(t, dir, "beads.db", nil)
	ag := cageAgent(t, a, "cage: container\n")

	p := a.CheckParityIn(ag, claude, CageContainer, TierStrong, dir)
	row := adr0074Row(p)
	if row == "" {
		t.Fatalf("a store holding beads.db must carry the ADR 0074 row: %+v", p.Degraded)
	}
	for _, want := range []string{"holds a SQLite database (beads.db)", "--no-db", "erased by the host's next flush", "ADR 0074", "cage: seatbelt", "no-db"} {
		if !strings.Contains(row, want) {
			t.Errorf("row missing %q: %s", want, row)
		}
	}
	if err := degradedError(degradedError{p: p}); !strings.Contains(err.Error(), "--allow-degraded") {
		t.Errorf("a degraded launch over this gate must still point at the flag: %v", err)
	}
}

// container cage + a no-db store (config.yaml + issues.jsonl only, no
// beads.db, no metadata.json) -> no such row: a no-db store is exactly the
// shape the ADR's remedy produces, and it must not re-trip the gate it was
// the fix for.
func TestContainerSQLiteGateIsSilentOverANoDBStore(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeNoDBStore(t, dir)
	ag := cageAgent(t, a, "cage: container\n")

	p := a.CheckParityIn(ag, claude, CageContainer, TierStrong, dir)
	if row := adr0074Row(p); row != "" {
		t.Errorf("a no-db store must not carry the ADR 0074 row: %s", row)
	}
}

// metadata.json naming another database file: when that file exists, the
// row names IT, not beads.db; when it does not exist, and beads.db does not
// exist either, there is no row at all — the answer to "does a SQLite file
// sit here", not to "does metadata.json's claim resolve".
func TestContainerSQLiteGateFollowsMetadataJSONsDatabaseName(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	ag := cageAgent(t, a, "cage: container\n")

	named := t.TempDir()
	writeSQLiteStore(t, named, "custom.db", map[string]string{"database": "custom.db"})
	if row := adr0074Row(a.CheckParityIn(ag, claude, CageContainer, TierStrong, named)); !strings.Contains(row, "custom.db") {
		t.Errorf("a metadata.json name that exists must be the file the row names: %q", row)
	}

	missing := t.TempDir()
	if err := os.MkdirAll(filepath.Join(missing, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]string{"database": "ghost.db"})
	if err := os.WriteFile(filepath.Join(missing, beadsDirName, "metadata.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if row := adr0074Row(a.CheckParityIn(ag, claude, CageContainer, TierStrong, missing)); row != "" {
		t.Errorf("a metadata.json name that does not exist, with no beads.db either, must carry no row: %q", row)
	}
}

// seatbelt/shims cage + a SQLite store -> no row at all: the gate is the
// container tier's only, because a seatbelt or shims bd is never run
// `--no-db` (CageBdFlags is a container-only render) and so never loses a
// write this way.
func TestContainerSQLiteGateIsContainerOnly(t *testing.T) {
	// No t.Parallel(): this mutates AvailableCages, a package-level map
	// other tests read (testparallel's census, ranger-base-kffdz).
	a := cageApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeSQLiteStore(t, dir, "beads.db", nil)

	shims := cageAgent(t, a, "")
	if row := adr0074Row(a.CheckParityIn(shims, claude, CageShims, TierStrong, dir)); row != "" {
		t.Errorf("cage: shims must carry no ADR 0074 row: %q", row)
	}

	seatbelt := cageAgent(t, a, "cage: seatbelt\n")
	had := AvailableCages[CageSeatbelt]
	AvailableCages[CageSeatbelt] = true
	defer func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	}()
	if row := adr0074Row(a.CheckParityIn(seatbelt, claude, CageSeatbelt, TierStrong, dir)); row != "" {
		t.Errorf("cage: seatbelt must carry no ADR 0074 row: %q", row)
	}
}
