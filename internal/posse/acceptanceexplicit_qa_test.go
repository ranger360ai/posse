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
// TestVerifyAcceptanceLineQuotesNoOperatorProse. This file owns the two
// claims those cannot make: that the mechanism is GONE from the package
// rather than merely unreached, and that the fields §4 keeps beside the
// acceptance line are still there in the order it names them.
//
// MUTATION, each restored: name any banned symbol in a non-test file here and
// the census reds with path, line and symbol; drop or reorder any of closer /
// close_reason / closed / labels / acceptance in verifySection and the order
// arm reds naming the pair that moved.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
