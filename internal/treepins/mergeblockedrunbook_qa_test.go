package treepins

// QA pin for ranger-base-vh2o8.
//
// The merge-back block body now ends by sending its reader to a runbook
// fragment BY PATH — the measured replay that a block's real conflict needs.
// A path inside a string constant is checked by nothing: rename or remove
// that fragment and every open block still points at it, and the reader is
// back where the bead started, with a recipe it cannot find.
//
// Narrow on purpose. `docs/notes.d/` paths appear all over internal/posse —
// in comments, and in test fixtures that deliberately name fragments no tree
// has (hookcluster_qa_test.go's, notesguard_qa_test.go's) — so a tree-wide
// version of this would need a register of exemptions before it could be
// green. This reads ONE function: the copy a seat is dispatched onto.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// notesFragmentCite matches a docs/notes.d fragment path as it is written in
// Go source.
var notesFragmentCite = regexp.MustCompile(`docs/notes\.d/[A-Za-z0-9._-]+\.md`)

// goFuncBody returns the source of one top-level function: its `func
// <name>(` line through the closing brace, which for a top-level function is
// the first line that is a bare `}`. Nothing around it — a doc comment
// belongs to whatever it precedes, and a pin that swallowed the next
// function's would read a citation this one does not make.
func goFuncBody(t *testing.T, src, name string) string {
	t.Helper()
	i := strings.Index(src, "\nfunc "+name+"(")
	if i < 0 {
		t.Fatalf("no func %s in the source read — the pin has stopped reading its subject", name)
	}
	rest := src[i+1:]
	j := strings.Index(rest, "\n}\n")
	if j < 0 {
		t.Fatalf("func %s does not close — the pin has stopped reading its subject", name)
	}
	return rest[:j+3]
}

func TestMergeBlockedReplayCitesARunbookThatExists(t *testing.T) {
	b, err := os.ReadFile("internal/posse/dispatch.go")
	if err != nil {
		t.Fatalf("read dispatch.go: %v", err)
	}
	body := goFuncBody(t, string(b), "mergeBlockedReplay")

	cites := notesFragmentCite.FindAllString(body, -1)
	if len(cites) == 0 {
		t.Fatalf("the merge-back block's replay copy cites no runbook (ranger-base-vh2o8) — it was the only pointer to the measured recipe:\n%s", body)
	}
	for _, c := range cites {
		if _, err := os.Stat(c); err != nil {
			t.Errorf("the merge-back block sends its reader to %s, which is not in the tree (%v) — every open block carries that path", c, err)
		}
	}
}

// The control: the checker must be red on a fragment that is not there, and
// on a copy that cites none. Without it this is a green function nobody has
// seen refuse anything.
func TestMergeBlockedRunbookCheckCanFail(t *testing.T) {
	const gone = "docs/notes.d/ranger-base-no-such-fragment.md"
	if _, err := os.Stat(gone); err == nil {
		t.Fatalf("%s exists, so the control measures nothing", gone)
	}
	if got := notesFragmentCite.FindAllString("...see "+gone+" for it.", -1); len(got) != 1 || got[0] != gone {
		t.Errorf("the citation scanner does not read a fragment path out of prose: %q", got)
	}
	if got := notesFragmentCite.FindAllString("no runbook here at all", -1); len(got) != 0 {
		t.Errorf("the citation scanner invents a citation: %q", got)
	}
}
