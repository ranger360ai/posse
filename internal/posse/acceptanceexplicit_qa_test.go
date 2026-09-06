//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0006 §4 as simplified 2026-09-05, for ranger-base-0ezn7: acceptance is
// the closed bead's own, and no guessed PID row stands in for it.
//
// A removal needs a pin for the same reason a fix does, and this one needs it
// more than most, because what was removed LOOKED useful: a "done when" line
// in a verify bead reads like exactly what a verifier wants. The measurement
// (verifyAcceptanceLine's comment, and the numbers below) is what says it was
// not — and a green suite says nothing about a line nobody renders, so this
// is what stops it being rebuilt.
//
// MEASURED 2026-09-06 over the operator's whole queue (2229 beads, 2100
// closed, 1048 carrying a verify label, closed 2026-08-12 .. 2026-09-06):
// the 203 verify beads this rule filed in that window carried the row ZERO
// times and no close of one cites it; re-run against the matcher as it stood,
// 379 of the 1048 closes would have earned a row, and those 379 rows are TWO
// distinct sentences (359 of them one sentence). The cell is a property of
// the closer's PID and the bead's type, never of the task.
//
// The behavioural half is in verifyafter_test.go —
// TestVerifyDescriptionCarriesCloserAndPointsAtSourceAcceptance,
// TestVerifyDescriptionNamesTheLimitWhenTheSourceHasNoAcceptance,
// TestVerifyDescriptionNeverQuotesAPidIntentTable and
// TestVerifyAcceptanceLineQuotesNoOperatorProse. This file owns the three
// claims those cannot make: that the mechanism is GONE from the package
// rather than merely unreached, that the fields §4 keeps beside the
// acceptance line are still there in the order it names them, and (added by
// ranger-base-0r61m) that the two DOCUMENTS which prescribe this rule to a
// human — ADR 0005 §3's QA recommended text and the shipped QA PID's
// `## Work prompt` — still agree with §4 and with each other.
//
// MUTATION, each restored: name any banned symbol in a non-test file here and
// the census reds with path, line and symbol; drop or reorder any of closer /
// close_reason / closed / labels / acceptance in verifySection and the order
// arm reds naming the pair that moved.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse"
)

// inferredIntentVocabulary is the removed mechanism, spelled as the
// identifiers and the rendered wire strings that only it ever had. Each
// entry is a substring; none is a substring of anything that survives, so a
// hit is a hit.
//
// `IntentRow`, `IntentRows` and `markdownRows` are here because the caller
// census after the two verifyafter.go callers went showed nothing else read
// them — the PID's `## Intents` table is still the PID's (pidcheck.go still
// requires its header, and the frontmatter `intents:` list is untouched); it
// is only the HARNESS that stopped mining it.
var inferredIntentVocabulary = []string{
	"closerDoneWhen",
	"closerIntentRows",
	"IntentDoneWhen",
	"intentMatchesLabel",
	"IntentRows(",
	"IntentRow{",
	"markdownRows(",
	"done when (",
	"unmatched; every intent",
}

func TestQANoInferredIntentMechanismRemainsInInternalPosse(t *testing.T) {
	t.Parallel()
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(inferredIntentVocabulary) == 0 {
		t.Fatal("the vocabulary is empty, so this census can only ever pass")
	}
	// Test files are excluded because THIS file names every banned symbol,
	// and a census that could not name its own subject would have to spell
	// it in pieces — which is how a census stops being readable and starts
	// being wrong.
	scanned, sawOwner := 0, false
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		if name == "verifyafter.go" {
			sawOwner = true
		}
		for i, line := range strings.Split(string(b), "\n") {
			for _, bad := range inferredIntentVocabulary {
				if strings.Contains(line, bad) {
					t.Errorf("%s:%d names %q — ADR 0006 §4 makes acceptance explicit; the closed bead's "+
						"own description is the checklist, and a guessed PID row may not stand in for it:\n\t%s",
						filepath.Join(".", name), i+1, bad, strings.TrimSpace(line))
				}
			}
		}
	}
	// The scan must have REACHED this package's own source: a census that
	// walked an empty directory passes for the wrong reason (a pass count is
	// not a coverage floor, so this asks for the file that owns the subject
	// rather than for a number).
	if scanned == 0 {
		t.Fatal("the census read no non-test .go file at all")
	}
	if !sawOwner {
		t.Fatal("the census never opened verifyafter.go — the file that HELD the mechanism was not " +
			"among the files it read, so it graded nothing (point the walk elsewhere and this is what reds)")
	}
}

// §4 names what stands beside the acceptance line — "closer, close reason,
// close time and labels" — and the removal is only correct if it took the
// row and nothing else. Order is part of it: the section is read top to
// bottom by a person, and the checklist comes after the facts about the
// close, not before them.
func TestQAVerifySectionKeepsTheFieldsBesideTheAcceptanceLine(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	is := BdIssue{ID: "a-1", Title: "gate shell", Assignee: "developer",
		Labels: []string{"code", "debt"}, IssueType: "bug", CloseReason: "fixed",
		Description: "what it does", ClosedAt: &closed}
	got := a.verifySection(t.TempDir(), is, verifyCloser(is))

	prev := -1
	for _, want := range []string{
		"- closer: developer",
		"- close_reason: fixed",
		"- closed: 2026-08-18T09:20:06Z",
		"- labels: code, debt",
		"- acceptance: ",
	} {
		i := strings.Index(got, want)
		if i < 0 {
			t.Fatalf("the section lost %q — ADR 0006 §4 keeps it beside the acceptance line:\n%s", want, got)
		}
		if i < prev {
			t.Errorf("%q moved above the field before it; §4's order is closer, close reason, close "+
				"time, labels, then the checklist:\n%s", want, got)
		}
		prev = i
	}
}

// §5, which the removal must not disturb: a commit naming the id is CITATION
// and says so, and the branch-landing block is separate evidence beside it.
// They are the two lines nearest the one that changed.
func TestQAVerifySectionKeepsCitationAndLandingApart(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("verifyafter.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"a commit may merely CITE the bead",
		"session branches cut for %s (branch.<b>.posseBead)",
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("verifyafter.go no longer carries %q — ADR 0006 §5 keeps citation labelled as "+
				"citation and landing evidence independent of it", want)
		}
	}
}

// §3, the queue guarantees the ADR forbids trading for this simplification.
// The behavioural pins are the batching tests; this holds the two numbers a
// removal could quietly retune while it was in the file.
func TestQABatchingDefaultsSurviveTheAcceptanceChange(t *testing.T) {
	t.Parallel()
	if DefaultVerifyBatch != 1 {
		t.Errorf("DefaultVerifyBatch = %d, want 1 — ADR 0006 §3's 1:1 default", DefaultVerifyBatch)
	}
	if DefaultVerifyBatchAge != 24*time.Hour {
		t.Errorf("DefaultVerifyBatchAge = %s, want 24h — ADR 0006 §3's tail flush", DefaultVerifyBatchAge)
	}
}

// ---------------------------------------------------------------------------
// ranger-base-0r61m, finding 1 of ranger-base-ps10r: the DOC half.
//
// The removal above landed in the code and in `examples/agents/qa.md`, and
// left docs/adr/0005-work-prompt-blueprints.md §3 — the page whose whole job
// is to carry those recommended texts ("an instance's PIDs are the operator's;
// `examples/agents/*` carry these") — still prescribing the mechanism §4
// deletes, and still telling QA to verify "not the description" when §4 makes
// the description the checklist. Nothing caught it because no test read that
// ADR (`git grep -ln "0005-work-prompt" -- '*_test.go'` was empty) and the
// shipped-example digest table pins the example's BYTES, never the ADR text it
// is supposed to be a copy of. So the two drifted apart silently for the one
// commit that changed both halves of the same sentence.
//
// The banned vocabulary below is PRESCRIPTIVE spellings only. `qa.md` says
// "never guessed from the closer's PID", and a negated mention is the opposite
// of prescribing one — banning the bare token "PID" would red the correct text
// and reward deleting the warning, which is backwards.
//
// MUTATION, each restored: put the pre-f2f3c6ab sentence back in either text
// and arm 1 reds naming the file and the phrase; drop "acceptance" or "bead"
// from either and arm 2 reds; drop the "ADR 0006 §4" citation from either and
// arm 3 reds. Point either reader at the wrong text and the extraction guards
// fatal rather than passing on an empty string.

// prescribedInferenceVocabulary is how the deleted mechanism reads when a
// document TELLS a verifier to use it. Each entry is a substring.
var prescribedInferenceVocabulary = []string{
	"closing persona's PID row",
	"PID row for this intent",
	`"done when" in the closing`,
	"not the description",
}

// adrQARecommendedText is ADR 0005 §3's QA bullet, anchored on the paragraph
// that declares the texts recommended — `- **QA**` also opens rows elsewhere
// on the page, and reading one of those would grade the wrong sentence.
func adrQARecommendedText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../docs/adr/0005-work-prompt-blueprints.md")
	if err != nil {
		t.Fatal(err)
	}
	adr := string(b)
	const anchor = "Recommended texts"
	i := strings.Index(adr, anchor)
	if i < 0 {
		t.Fatalf("docs/adr/0005-work-prompt-blueprints.md §3 no longer says %q — this pin is reading nothing", anchor)
	}
	for _, line := range strings.Split(adr[i:], "\n") {
		if strings.HasPrefix(line, "- **QA**") {
			return line
		}
	}
	t.Fatal("docs/adr/0005-work-prompt-blueprints.md §3 has no `- **QA**` recommended text after the anchor")
	return ""
}

// qaWorkPromptText is what the shipped QA PID actually carries. It reads the
// EMBED for the same reason commitwallseed_qa_test.go does: posse.Seed is what
// a release binary hands `posse init`, and in a checkout it is the same bytes.
func qaWorkPromptText(t *testing.T) string {
	t.Helper()
	b, err := fs.ReadFile(posse.Seed, "agents/qa.md")
	if err != nil {
		t.Fatal(err)
	}
	sec := BodySection(string(b), "## Work prompt")
	if strings.TrimSpace(sec) == "" {
		t.Fatal("examples/agents/qa.md has no `## Work prompt` section — this pin is reading nothing")
	}
	return sec
}

// TestQAAcceptanceTextsAgreeAcrossTheADRAndTheShippedPID is the coupling
// ADR 0005 §3 asserts and nothing enforced: the page that says which text a
// PID should carry, and the PID that ships it, must not disagree about where a
// verify checklist comes from.
func TestQAAcceptanceTextsAgreeAcrossTheADRAndTheShippedPID(t *testing.T) {
	t.Parallel()
	texts := map[string]string{
		"docs/adr/0005-work-prompt-blueprints.md §3 (QA)": adrQARecommendedText(t),
		"examples/agents/qa.md `## Work prompt`":          qaWorkPromptText(t),
	}
	if len(prescribedInferenceVocabulary) == 0 {
		t.Fatal("the vocabulary is empty, so arm 1 can only ever pass")
	}
	for where, text := range texts {
		// Arm 1: neither prescribes the mechanism ADR 0006 §4 deleted.
		for _, bad := range prescribedInferenceVocabulary {
			if strings.Contains(text, bad) {
				t.Errorf("%s names %q — ADR 0006 §4 makes the closed bead's own acceptance the "+
					"checklist and forbids a guessed PID row standing in for it:\n\t%s", where, bad, text)
			}
		}
		// Arm 2: both name the source §4 does name.
		for _, want := range []string{"checklist", "acceptance", "bead"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s no longer says %q — ADR 0006 §4's checklist is the closed bead's own "+
					"acceptance, and a text that does not name it points a verifier nowhere:\n\t%s",
					where, want, text)
			}
		}
		// Arm 3: both cite the section that decides it. This is the coupling
		// itself — the citation is what makes the two traceably one rule, so a
		// future edit to either has somewhere to look before it diverges.
		if !strings.Contains(text, "ADR 0006 §4") {
			t.Errorf("%s dropped its `ADR 0006 §4` citation — that section is what decides the "+
				"checklist's source, and without it neither text says which rule it is a copy of:\n\t%s",
				where, text)
		}
	}
}
