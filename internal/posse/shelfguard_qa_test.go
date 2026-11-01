package posse

// ranger-base-r0dp. Three sites globbed examples/agents and then did
// `if len(names) < 9 { t.Skipf(...) }`: TestExampleAgentsArePIDs and
// TestExampleAgentsHandoffsAreShapes (agents_test.go), and
// TestShelfPIDsLintCleanAsASet (pidcheck_test.go then,
// shelfpidlint_qa_test.go since ranger-base-nt0pi). The guard sits ON the
// shelf's actual size (nine), so retiring or renaming any one shelf PID
// turned every invariant those three pins hold into a silent pass instead
// of a red build.
//
// (The finding that filed this bead named five sites; measured directly
// against 2d8ccc9, the commit it cites, the other two —
// internal/rhq/initseed_qa_test.go:48 and
// internal/rhq/seedcrewrgx0_qa_test.go:79 — were already t.Fatalf, not
// t.Skipf, at that commit. Only the three above needed the fix.)
//
// shelfPIDs is that fix, shared: skip only when examples/agents itself is
// not checked out (the directory does not exist at all — e.g. a sparse
// checkout that omitted it); a directory that IS present but short of its
// committed nine is a regression and must fail the test.

import (
	"os"
	"path/filepath"
	"testing"
)

type shelfStatus int

const (
	shelfOK      shelfStatus = iota
	shelfMissing             // directory itself absent: skip
	shelfShort               // directory present, below the committed count: fail
)

// statShelf is the policy shelfPIDsIn enforces, pulled out from *testing.T
// so the pin below (TestShelfPIDsGuardFailsRatherThanSkipsWhenShort) can
// assert on the verdict directly instead of having to run a subtest whose
// deliberate failure would drag the pin itself red (t.Run reports the
// parent failed whenever a subtest does, independent of what the parent
// goes on to check).
func statShelf(dir string) (paths []string, status shelfStatus, err error) {
	if _, statErr := os.Stat(dir); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, shelfMissing, statErr
		}
		return nil, shelfShort, statErr
	}
	paths, err = filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, shelfShort, err
	}
	if len(paths) < 9 {
		return paths, shelfShort, nil
	}
	return paths, shelfOK, nil
}

func shelfPIDsIn(t *testing.T, dir string) []string {
	t.Helper()
	paths, status, err := statShelf(dir)
	switch status {
	case shelfMissing:
		t.Skipf("%s not checked out: %v", dir, err)
	case shelfShort:
		if err != nil {
			t.Fatalf("stat/glob %s: %v", dir, err)
		}
		t.Fatalf("%s holds %d reference PID(s), want the committed nine — the shelf shrank", dir, len(paths))
	}
	return paths
}

// shelfPIDs is examples/agents itself: the committed reference PIDs (ADR
// 0001) every shelf-wide PID contract pin reads.
//
// THE ROOT ARRIVES AS A PARAMETER, and the caller spells it `qibRepoRoot(t)`
// (ranger-base-8yv7z). It used to be
// `shelfPIDsIn(t, filepath.Join("..", "..", "examples", "agents"))` — a climb
// out of the package, handed to a helper that globs at its own parameter,
// which globs at ITS parameter in turn. That reads the whole shelf while
// computing no root, two call sites from the walk, so no rule in
// internal/treepins/treewidedoor_qa_test.go could see it: the three tests
// that read the shelf through here were outside the tree-wide class, were
// never asked for a Makefile door, and were reachable only by the ~950s
// internal/posse run. The register's own lesson is that the class is not
// bounded by how a test spells its root but by there being ONE — so the one
// helper is named in the test body that reads the tree, which puts the test
// in the class by the rule that half of it is derived from, with no rule
// widened and no chain followed.
func shelfPIDs(t *testing.T, root string) []string {
	t.Helper()
	return shelfPIDsIn(t, filepath.Join(root, "examples", "agents"))
}

// The pin: shrink a COPY of the shelf by one file (never the real shelf —
// [[mutation-harness-golden-copy]]) and require the guard to classify it as
// a failure, not a skip — and require a wholly absent directory to still
// classify as a skip, which is the case the guard exists to keep quiet.
func TestShelfPIDsGuardFailsRatherThanSkipsWhenShort(t *testing.T) {
	t.Parallel()
	shelf := shelfPIDs(t, qibRepoRoot(t))
	tmp := t.TempDir()
	for _, p := range shelf[1:] {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, filepath.Base(p)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, status, _ := statShelf(tmp); status != shelfShort {
		t.Errorf("a shelf present but short of its committed size (%d files here) must classify shelfShort (fail), got %v", len(shelf)-1, status)
	}

	if _, status, _ := statShelf(filepath.Join(tmp, "does-not-exist")); status != shelfMissing {
		t.Errorf("a wholly absent shelf directory must classify shelfMissing (skip), got %v", status)
	}
}
