package posse

// cutRule lifted out of codexupdatemenu_qa_test.go so every suite arm compiles
// it (the shape ranger-base-qp1hm set): a file with a build tag is absent from
// the arms it does not name, and this one has readers in two of them now —
// codexupdatemenu_qa_test.go under posse_arm3, and the verify-detection pins
// which moved to arm 1 under ranger-base-00fyg (ADR 0078 D3) to get a door.

import (
	"strings"
	"testing"
)

// cutRule removes one `[[rules]]` block by id, with the comment block that
// introduces it — the manifest's own layout, one blank-line-separated stanza
// per rule.
func cutRule(t *testing.T, toml, id string) string {
	t.Helper()
	stanzas := strings.Split(toml, "\n\n")
	kept := make([]string, 0, len(stanzas))
	cut := 0
	for _, s := range stanzas {
		if strings.Contains(s, "[[rules]]") && strings.Contains(s, `id = "`+id+`"`) {
			cut++
			continue
		}
		kept = append(kept, s)
	}
	if cut != 1 {
		t.Fatalf("cutRule(%q): removed %d stanzas, want exactly 1 — the manifest's layout moved under this test", id, cut)
	}
	return strings.Join(kept, "\n\n")
}
