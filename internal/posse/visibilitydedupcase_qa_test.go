//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-y5vud — the gap in ranger-base-99gww's close.
//
// That bead replaced a string compare with samePath (identity: same device,
// same inode) so a `beads_visibility:` key the operator spelled `~/src/hcn`
// meets the directory git spells `~/src/HCN`. It carried the SAME comparison
// to two dedup sites through anySamePath — SweepHookWall and
// SweepSecondStores, where two spellings of one checkout must be one row —
// and visibilitypathcase_qa_test.go pins only the lookup and the stamp.
//
// WHY THAT LEFT THE DEDUP SITES UNPINNED. The one dedup test that existed,
// TestQASecondStoreDeduplicatesByPath, spells its two entries `work` and
// `filepath.Join(work, ".")`, and filepath.Clean folds those into one
// string — so it is killed by samePath's STRING fast path and passes
// unchanged over the pre-fix compare. MEASURED 2026-10-03: the only
// case-differing `beads_visibility:` fixture anywhere in the package's test
// corpus is visibilitypathcase_qa_test.go's, and it reaches neither sweep,
// so anySamePath's identity half could regress to `resolvedPath(a) ==
// resolvedPath(b)` with every arm of every suite still green.
//
// WHAT THAT COSTS, which is why it is a pin and not a note: both sweeps
// report to a person. SweepHookWall would count one checkout as two
// `Declared` repos and print two rows probing one wall, and "0 of N present"
// would name a repo that was never a separate repo. SweepSecondStores would
// hand an operator the same `rm` twice — the thing its own dedup test says in
// its first line it exists to prevent.
//
// MUTATION-CHECKED (2026-10-03, this box, APFS case-insensitive): anySamePath
// back to a string compare reds both arms below and nothing else in arm 1.
// Each arm carries a DISTINCT-repo control in the same fixture, so a dedup
// that always answers "one" reds too.
//
// Both arms skip on a case-sensitive volume, where the two spellings are two
// directories and there is nothing here to fold — the same split, and the
// same reason, as the file this one sits beside.

import (
	"os"
	"path/filepath"
	"testing"
)

// SweepHookWall: two spellings of one checkout are one wall and one row.
func TestQAHookWallSweepCountsTwoCaseSpellingsOfOneRepoOnce(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	onDisk, lower := otherCaseDir(t, home, "HCN")

	a := hermetic(t, NewAppAt(t.TempDir()))
	if err := os.WriteFile(a.ConfigPath,
		[]byte("beads_visibility:\n  "+lower+": private\n  "+onDisk+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := a.SweepHookWall()
	if s.Declared != 1 || len(s.Repos) != 1 {
		t.Errorf("two spellings of one checkout declared %d repo(s) in %d row(s), want 1 and 1 — one wall probed twice is two rows about one repo",
			s.Declared, len(s.Repos))
	}

	// CONTROL, same shape and a genuinely second directory: the dedup must
	// still be able to count two, or the assertion above is satisfied by a
	// sweep that always answers one.
	other := filepath.Join(home, "second-checkout")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath,
		[]byte("beads_visibility:\n  "+lower+": private\n  "+other+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := a.SweepHookWall(); s.Declared != 2 || len(s.Repos) != 2 {
		t.Errorf("two DIFFERENT checkouts declared %d repo(s) in %d row(s), want 2 and 2", s.Declared, len(s.Repos))
	}
}

// SweepSecondStores: and one finding, so the operator is handed one `rm`.
func TestQASecondStoreDeduplicatesTwoCaseSpellingsOfOneCheckout(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	onDisk, lower := otherCaseDir(t, home, "HCN")
	instance := t.TempDir()

	a := hermetic(t, NewAppAt(t.TempDir()))
	mkdirAllOrFatal(t, filepath.Join(onDisk, beadsDirName))
	mkdirAllOrFatal(t, filepath.Join(instance, beadsDirName))
	writeBeadsFile(t, onDisk, beadsRedirect, filepath.Join(instance, beadsDirName)+"\n")
	writeBeadsFile(t, onDisk, "beads.db", "SQLite format 3\x00")

	if err := os.WriteFile(a.ConfigPath,
		[]byte("beads:\n  - "+lower+"\n  - "+onDisk+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := a.SweepSecondStores(); len(got) != 1 {
		t.Errorf("two spellings of one checkout are one store and one finding, got %d: %v", len(got), SecondStoreLines(got))
	}

	// CONTROL: a genuinely second checkout, redirected and carrying its own
	// store, is a second finding.
	other := filepath.Join(home, "second-checkout")
	mkdirAllOrFatal(t, filepath.Join(other, beadsDirName))
	writeBeadsFile(t, other, beadsRedirect, filepath.Join(instance, beadsDirName)+"\n")
	writeBeadsFile(t, other, "beads.db", "SQLite format 3\x00")
	if err := os.WriteFile(a.ConfigPath,
		[]byte("beads:\n  - "+lower+"\n  - "+other+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := a.SweepSecondStores(); len(got) != 2 {
		t.Errorf("two DIFFERENT checkouts are two findings, got %d: %v", len(got), SecondStoreLines(got))
	}
}
