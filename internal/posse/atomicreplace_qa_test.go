package posse

// QA pin for the OTHER half of ranger-base-5qnt / ranger-base-5cv7.
//
// Both beads asked for two things, and only one of them was pinned. The
// lock is pinned hard (trustlock_qa_test.go, cagehomelock_qa_test.go: an
// unlocked seeder loses 5–7 of 8 dirs, 3/3). The temp-file-and-rename was
// not pinned at all: reverting either writeJSONInPlace call site to the
// plain os.WriteFile it replaced — lock left in place — was GREEN across
// all three packages when this file was written. That is the half both
// beads named as the reason a losing writer could leave a TRUNCATED
// config rather than merely a merged-away one ("invalid character '6'
// after top-level value", measured on ranger-base-5cv7's pre-fix code),
// and the half trust.go's own doc comment promises in prose.
//
// The property, stated as something observable rather than as a mechanism:
// a reader that opened the config BEFORE a launch seeds it still reads the
// complete file it opened. A rename gives it that — the reader's fd holds
// the old inode, whole, until it closes. A truncate-in-place does not: the
// same fd is the file being rewritten, so that reader sees the new content
// if it is lucky about timing and a half-written prefix if it is not, and
// the unlucky case is exactly the corrupt config the beads describe.
//
// Driven through the CONSUMERS (SeedClaudeTrust and SeedCageHome), not
// through writeJSONInPlace, per the standard ranger-base-p84 set and ADR
// 0017 §3 inherited: a helper that is correct and unreached is the defect
// class this suite exists to catch.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// heldReaderSeesTheWholeOldFile opens p, has seed() replace it, and reads
// what the already-open fd still holds. Returns the bytes that reader got.
func heldReaderSeesTheWholeOldFile(t *testing.T, p string, seed func()) []byte {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	seed()
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read through the held fd: %v", err)
	}
	return b
}

// assertWholeOldFile is the acceptance criterion: what the held reader got
// is the complete file it opened, byte for byte — not empty, not a prefix,
// not the new content.
func assertWholeOldFile(t *testing.T, what string, before, got []byte) {
	t.Helper()
	var state map[string]any
	if err := json.Unmarshal(got, &state); err != nil {
		t.Fatalf("%s: a reader holding the config open across a seed read %d bytes that are not JSON (%v) — the live file was truncated in place instead of replaced by rename (ranger-base-5qnt / ranger-base-5cv7):\n%q",
			what, len(got), err, got)
	}
	if string(got) != string(before) {
		t.Errorf("%s: a reader holding the config open across a seed saw the file change under it (%d bytes -> %d) — the seed rewrote the live inode instead of renaming a finished file over it; the half-written window that costs is the same one",
			what, len(before), len(got))
	}
}

// The host config (ranger-base-5qnt): `posse new` seeds directory trust
// into the operator's whole claude state while claude itself may have the
// file open.
func TestQASeedTrustReplacesByRenameNotTruncate(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".claude.json")
	rt := claudeRuntime(t)
	root := t.TempDir()

	if _, err := SeedClaudeTrust(cfg, rt, qaSeedDir(root, 0)); err != nil {
		t.Fatal(err)
	}
	// Enough prior state that a truncate-in-place has a window at all: the
	// operator's config is their whole project history, not two keys.
	state := readConfig(t, cfg)
	projects, _ := state["projects"].(map[string]any)
	for i := 0; i < 400; i++ {
		projects[qaSeedDir(root, 1000+i)] = map[string]any{"hasTrustDialogAccepted": true}
	}
	writeConfig(t, cfg, state)

	before, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := heldReaderSeesTheWholeOldFile(t, cfg, func() {
		d := qaSeedDir(root, 1)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := SeedClaudeTrust(cfg, rt, d); err != nil {
			t.Fatal(err)
		}
	})
	assertWholeOldFile(t, "host config", before, got)

	// And the seed did land — otherwise this pin is green on a no-op.
	if !ClaudeTrusted(readConfig(t, cfg), qaSeedDir(root, 1)) {
		t.Fatal("the seed under test never wrote its dir — this pin measured nothing")
	}
}

// The cage HOME's config (ranger-base-5cv7): same shape, the file the
// container tier seeds instead.
func TestQASeedCageHomeReplacesByRenameNotTruncate(t *testing.T) {
	a := cageApp(t)
	ag := cageAgent(t, a, "")
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()

	home, err := a.SeedCageHome(ag, rt, qaSeedDir(root, 0))
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".claude.json")
	state := readConfig(t, cfg)
	projects, _ := state["projects"].(map[string]any)
	for i := 0; i < 400; i++ {
		projects[qaSeedDir(root, 1000+i)] = map[string]any{"hasTrustDialogAccepted": true}
	}
	writeConfig(t, cfg, state)

	before, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := heldReaderSeesTheWholeOldFile(t, cfg, func() {
		d := qaSeedDir(root, 1)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := a.SeedCageHome(ag, rt, d); err != nil {
			t.Fatal(err)
		}
	})
	assertWholeOldFile(t, "cage HOME config", before, got)

	after := readConfig(t, cfg)
	ap, _ := after["projects"].(map[string]any)
	proj, _ := ap[qaSeedDir(root, 1)].(map[string]any)
	if proj["hasTrustDialogAccepted"] != true {
		t.Fatal("the seed under test never wrote its dir — this pin measured nothing")
	}
}
