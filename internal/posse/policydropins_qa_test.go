//go:build !posse_arm2 && !posse_arm3

package posse

// TestQAThePolicyDropInsDoNotSetTheSameKeyTwice, moved out of
// policyhooks_qa_test.go by ranger-base-8yv7z. IT IS HERE FOR ITS ARM, the
// same reason shelfpidlint_qa_test.go exists one file over
// (ranger-base-nt0pi).
//
// The pin's subject is the DIRECTORY, not the file its old neighbours read:
// it enumerates `etc/claude/managed-settings.d` and holds every drop-in in it
// to setting no key another one sets. So a bead that adds a fourth drop-in
// joins its population and can red it, which is what the tree-wide class in
// internal/treepins/treewidedoor_qa_test.go means, and what a Makefile door
// ($(QA_SEED_PINS), `make seed-check`) is for. A door's `-run` runs with NO
// `-tags` (TestQAEveryMakefileDoorPinIsReachableInTheDefaultArm), and a
// `go test -run` that matches nothing exits 0 — so behind
// `//go:build posse_arm3`, where the rest of that file sits, a door would
// have been green over a pin it never ran. The pin moves to arm 1 rather
// than the door mechanism learning to pass a tag, which no door does today;
// that tag question is ranger-base-m94g2's and nothing here prejudges it.
//
// Nothing else in policyhooks_qa_test.go moved. Those pins read
// `30-posse-hooks.json` BY NAME and red only when that one file changes,
// which is not this class — and they need a live $HOME and a real hook run,
// which is why that file is in arm 3 at all.
//
// AND ITS ROOT COMES FROM qibRepoRoot. It used to spell
// `const dir = "../../etc/claude/managed-settings.d"` and read the directory
// through that identifier, which reaches the tree while computing no root at
// all: every rule the register derives internal/posse's half of the class
// from keys on that one helper, so the reading was invisible, the pin was
// never asked for a door, and its only route was the ~950s package run.
// ranger-base-nt0pi fixed three pins that spelled the climb AT the walker
// and fenced that spelling; this is the same defect one hop out, through an
// identifier, and the fence reads that spelling too now.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Two policy sources setting one key make the winner an ordering accident —
// the same argument the inlet pin's drop-in test makes, now that the
// directory holds three files.
func TestQAThePolicyDropInsDoNotSetTheSameKeyTwice(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(qibRepoRoot(t), "etc", "claude", "managed-settings.d")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("the policy drop-in directory is missing: %v", err)
	}
	owner := map[string]string{}
	seen := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s is not valid JSON: %v", e.Name(), err)
		}
		seen++
		for k := range m {
			if prev, dup := owner[k]; dup {
				t.Errorf("%q is set by both %s and %s — two policy sources for one key make the winner an ordering accident", k, prev, e.Name())
				continue
			}
			owner[k] = e.Name()
		}
	}
	// Without this the loop above passes on an empty directory.
	if seen < 3 {
		t.Errorf("read %d drop-ins, want at least 3 (inlet, field, hooks) — a sweep over nothing agrees with everything", seen)
	}
}
