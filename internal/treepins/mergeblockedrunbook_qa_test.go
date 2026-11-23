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
	"strconv"
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

// QA pin for ranger-base-uc3ng — THE EDGE: one claim, two copies, and until
// this pin nothing read the second.
//
// The pin above keeps the cited fragments in the tree. What it cannot see is
// what they SAY, and both of them carry their own copy of the replay recipe:
// docs/notes.d/ranger-base-4r8fw.md under the heading "## The recipe, per
// commit", docs/notes.d/ranger-base-xea2y.md as the single-commit worked
// example. Both were written before ranger-base-ghtov put `git merge
// --ff-only <base>` at the top of the dispatched copy and `git merge-base
// --is-ancestor <base> HEAD` at the bottom — the two steps that exist because
// a dispatched seat's HEAD is its LAUNCH base, so a replay without them is
// faithful, pairs, reads back EMPTY and lands nowhere (MEASURED under
// ranger-base-56gd0: replay 1c9a6f26 on a base four commits stale, re-filed
// as ranger-base-zr2mf).
//
// So the block's last line sent a seat to the pre-fix procedure, and the two
// families of pin that existed could not say so: internal/posse's read the
// rendered STRING and cannot open a file, and the notes pins (`make
// notes-check`, adrtestcitation_qa_test.go) resolve the PATH and never the
// content. MEASURED 2026-11-23, main d13842d4: `grep -n` over both fragments
// finds `ff-only` and `is-ancestor` in neither.
//
// This reads both halves and compares them. It is deliberately a DISJUNCTION
// — a cited copy of the recipe may carry the head step itself, or the citation
// may name it — so it holds whichever way a later bead closes the gap, and it
// reds if the citation drops the scope while the fragments still lack the
// steps. Reading named files and enumerating nothing, it is the enumeration
// rule's own negative case, so it needs no door row of its own: `make
// treepins-check` is this package minus $(QA_TREEPINS_EXCLUDED).
var headSteps = []string{"merge --ff-only", "merge-base --is-ancestor"}

// recipeMark is how a fragment says it carries a copy of the replay: the
// launcher's own operand, which is what makes the text a procedure a seat can
// follow rather than a note about one. A fragment without it prescribes
// nothing and can disagree with nothing.
const recipeMark = "merge-tree --write-tree"

// citationGaps names the head steps a cited fragment's own recipe does not
// carry AND the citing text does not name — the gap a seat falls through.
func citationGaps(fragment, citation string) []string {
	if !strings.Contains(fragment, recipeMark) {
		return nil
	}
	var out []string
	for _, s := range headSteps {
		if strings.Contains(fragment, s) || strings.Contains(citation, s) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// goStringSegment matches one interpreted string literal in Go source.
var goStringSegment = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// goConstLiteral returns the value of one `const <name> = "..." + "..."`
// declaration: the concatenation's own lines, which are the ones ending in
// `+`. The citation is a const inside mergeBlockedReplay and the rest of that
// function names both head steps as commands, so a pin that read the whole
// body would be green over a citation that said nothing.
func goConstLiteral(t *testing.T, src, name string) string {
	t.Helper()
	i := strings.Index(src, "const "+name+" = ")
	if i < 0 {
		t.Fatalf("no `const %s = ` in the source read — the pin has stopped reading its subject", name)
	}
	var out strings.Builder
	for _, line := range strings.Split(src[i:], "\n") {
		for _, seg := range goStringSegment.FindAllString(line, -1) {
			s, err := strconv.Unquote(seg)
			if err != nil {
				t.Fatalf("const %s: %q does not unquote (%v) — the pin has stopped reading its subject", name, seg, err)
			}
			out.WriteString(s)
		}
		if !strings.HasSuffix(strings.TrimRight(line, " \t"), "+") {
			break
		}
	}
	return out.String()
}

func TestMergeBlockedRunbookCitationNamesTheStepsItsFragmentsLack(t *testing.T) {
	b, err := os.ReadFile("internal/posse/dispatch.go")
	if err != nil {
		t.Fatalf("read dispatch.go: %v", err)
	}
	citation := goConstLiteral(t, goFuncBody(t, string(b), "mergeBlockedReplay"), "runbooks")

	cites := notesFragmentCite.FindAllString(citation, -1)
	if len(cites) == 0 {
		t.Fatalf("the replay copy's runbook citation names no fragment, so this pin reads nothing (ranger-base-uc3ng):\n%s", citation)
	}
	for _, c := range cites {
		f, err := os.ReadFile(c)
		if err != nil {
			// The pin above is the one that owns this; here it means the
			// comparison could not be made.
			t.Errorf("read the cited fragment %s: %v", c, err)
			continue
		}
		if gaps := citationGaps(string(f), citation); len(gaps) > 0 {
			t.Errorf("%s carries a copy of the replay recipe (%q) that names %q nowhere, and the citation that sends a seat to it does not say so either (ranger-base-uc3ng) — a seat that follows the citation writes a replay that pairs, reads back clean and lands nowhere. Put the step in the fragment, or say in the citation that the fragment lacks it.\nthe citation reads:\n%s",
				c, recipeMark, gaps, citation)
		}
	}
}

// The control: the comparison must be red on the pre-fix pair and quiet on
// each of the three ways the gap can be closed. Without it this is a green
// function nobody has seen refuse anything — and the pre-fix pair is the real
// one, taken from the text this bead replaced.
func TestMergeBlockedRunbookCitationGapCheckCanFail(t *testing.T) {
	const preFixCitation = "docs/notes.d/ranger-base-xea2y.md is the worked example and\n" +
		"docs/notes.d/ranger-base-4r8fw.md the per-commit loop, both MEASURED."
	const fragmentWithRecipe = "## The recipe, per commit\n" +
		"TREE=$(git merge-tree --write-tree --merge-base=$C^ HEAD $C | head -1)\n"

	if got := citationGaps(fragmentWithRecipe, preFixCitation); len(got) != len(headSteps) {
		t.Errorf("the comparison finds %q to object to in the pre-fix pair, want all of %q — it would pass on the defect (ranger-base-uc3ng)", got, headSteps)
	}
	// Closed in the citation: this bead's route.
	scoped := preFixCitation + " Neither names `git merge --ff-only` or `git merge-base --is-ancestor`."
	if got := citationGaps(fragmentWithRecipe, scoped); len(got) > 0 {
		t.Errorf("a citation that names the missing steps is still read as a gap: %q", got)
	}
	// Closed in the fragment: the route this pin must not forbid, since a
	// later bead may add the steps to the recipe block itself.
	fixed := fragmentWithRecipe + "git merge --ff-only <base>\ngit merge-base --is-ancestor <base> HEAD\n"
	if got := citationGaps(fixed, preFixCitation); len(got) > 0 {
		t.Errorf("a fragment that carries the steps itself is still read as a gap: %q", got)
	}
	// And a fragment that prescribes no replay at all disagrees with nothing.
	if got := citationGaps("a note about the merge-back, with no recipe in it", preFixCitation); len(got) > 0 {
		t.Errorf("a fragment carrying no copy of the recipe is read as a gap: %q", got)
	}

	// The reader, over planted source: it must take the concatenation and
	// stop at the end of it, or the citation it compares is the whole
	// function body — which names both steps as commands and is green over
	// anything.
	const src = "func x() {\n\tconst runbooks = \"a\\n\" +\n\t\t\"b `git merge --ff-only`\"\n" +
		"\treturn warn + \"  git merge-base --is-ancestor main HEAD\\n\"\n}\n"
	if got, want := goConstLiteral(t, src, "runbooks"), "a\nb `git merge --ff-only`"; got != want {
		t.Errorf("goConstLiteral read %q, want %q", got, want)
	}
}
