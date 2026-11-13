package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVulnPrivateHitsBareIDIsClean is the compliant shape AGENTS.md
// "Landing the plane" prescribes, every form the rule's own prose and
// ranger-base-lcuec.md's own "Not closed here" use: a bullet, backticks,
// bold, numbering, a trailing colon, two bare ids sharing one bullet.
func TestVulnPrivateHitsBareIDIsClean(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c", "ranger-base-gzl2w"}
	for _, line := range []string{
		"ranger-base-l751c",
		"- ranger-base-l751c",
		"* ranger-base-l751c",
		"1. ranger-base-l751c",
		"- `ranger-base-l751c`",
		"- **ranger-base-l751c**",
		"- ranger-base-l751c:",
		"- ranger-base-l751c, ranger-base-gzl2w",
		"(ranger-base-l751c)",
		"  ranger-base-l751c  ",
	} {
		if got := vulnPrivateHits(line, ids); got != nil {
			t.Errorf("vulnPrivateHits(%q) = %v, want no hit — this is the bare-id shape the rule allows", line, got)
		}
	}
}

// TestVulnPrivateHitsDescriptiveTextIsFlagged is the shape the rule
// refuses: anything beyond decoration, including the exact "see also" case
// the bead's own description worried a mechanical rule could miss without
// reading every line that kind of carefully. It does not: a letter is
// never in the allowed decoration set, so "see" trips it the same as a
// full description would.
func TestVulnPrivateHitsDescriptiveTextIsFlagged(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c"}
	for _, line := range []string{
		"- ranger-base-l751c stays open",
		"see also ranger-base-l751c",
		"ranger-base-l751c: the remote-settings cache",
		"fixed by ranger-base-l751c and the bwrap mask",
	} {
		if got := vulnPrivateHits(line, ids); len(got) != 1 || got[0] != ids[0] {
			t.Errorf("vulnPrivateHits(%q) = %v, want [%s] — descriptive text beside an open vuln-private id must be flagged", line, got, ids[0])
		}
	}
}

// TestVulnPrivateHitsIgnoresUnknownIDs: a bead id that is not in the
// caller's open-vuln-private set is not this check's business at all —
// lcuec.md's own "Not closed here" names two ordinary beads in full
// sentences, and AGENTS.md's rule does not bind them (not vuln-private).
func TestVulnPrivateHitsIgnoresUnknownIDs(t *testing.T) {
	t.Parallel()
	if got := vulnPrivateHits("ranger-base-wuuqi stays open, the operator's call", nil); got != nil {
		t.Errorf("vulnPrivateHits with no known ids = %v, want nil", got)
	}
	if got := vulnPrivateHits("ranger-base-wuuqi stays open, the operator's call", []string{"ranger-base-lcuec"}); got != nil {
		t.Errorf("vulnPrivateHits for an id not present = %v, want nil", got)
	}
}

// TestVulnPrivateHitsMultilineOnlyFlagsTheBadLine: a message or a markdown
// diff is more than one line, and a bare id two lines above a description
// of something else entirely must not be swept up with it.
func TestVulnPrivateHitsMultilineOnlyFlagsTheBadLine(t *testing.T) {
	t.Parallel()
	text := "wire the reaper\n\n## Not closed here\n- ranger-base-l751c\n- ranger-base-gzl2w: the remote-settings cache\n"
	got := vulnPrivateHits(text, []string{"ranger-base-l751c", "ranger-base-gzl2w"})
	if len(got) != 1 || got[0] != "ranger-base-gzl2w" {
		t.Errorf("vulnPrivateHits = %v, want [ranger-base-gzl2w] — the bare line above it must not be flagged", got)
	}
}

// TestVulnPrivateHitsFlagsADescriptionOnTheContinuationLine is
// ranger-base-68631 finding 1: the check was strictly PER-LINE, so the one
// shape anyone writing a bullet with detail reaches for — the detail indented
// on the line below — passed clean, which is the escape AGENTS.md's own "Not
// closed here" section invites. MEASURED through the shipped subcommand
// before the fix: `- <id> <text>` flagged rc 1, `- <id>\n  <text>` SILENT
// rc 0.
//
// Both directions, because a unit is read from either end: the text above an
// indented bare id is the same escape upside down. And the `+`-prefixed
// spelling, because the content arm scans `git diff --cached | grep '^+'`
// (gates.go's $posse_added) and nothing else — a rule that read indentation
// from column 0 would be dead in the one arm that reads a file.
func TestVulnPrivateHitsFlagsADescriptionOnTheContinuationLine(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c"}
	for _, text := range []string{
		"- ranger-base-l751c\n  the heap overflow in the parser\n",
		"## Not closed here\n\n- ranger-base-l751c\n  the heap overflow in the parser\n",
		"1. ranger-base-l751c\n   the heap overflow in the parser\n",
		"- ranger-base-l751c\n  > the heap overflow in the parser\n",
		"- ranger-base-l751c\n  **the heap overflow** in the parser\n",
		"- the residual that stays open:\n  ranger-base-l751c\n",
		"+- ranger-base-l751c\n+  the heap overflow in the parser\n",
		// ranger-base-ckcly finding 1: the LAZY continuation, one space
		// fewer than the row above it. Markdown reads a column-0 line under
		// a list item as a continuation of that item; the pop loop read it
		// as a unit of its own, so this passed SILENT, rc 0, through the
		// shipped subcommand.
		"- ranger-base-l751c\nthe heap overflow in the parser\n",
		"1. ranger-base-l751c\nthe heap overflow in the parser\n",
		"## Not closed here\n\n- ranger-base-l751c\nthe heap overflow in the parser\n",
		"+- ranger-base-l751c\n+the heap overflow in the parser\n",
		// Lazily continuing the INNER item of a nested list, which is the
		// item markdown's paragraph is open in.
		"- ranger-base-wuuqi stays open\n  - ranger-base-l751c\nthe heap overflow in the parser\n",
	} {
		if got := vulnPrivateHits(text, ids); len(got) != 1 || got[0] != ids[0] {
			t.Errorf("vulnPrivateHits(%q) = %v, want [%s] — a description in the id's own bullet must be flagged", text, got, ids[0])
		}
	}
	// The controls, so the widening cannot pass by flagging everything: a
	// NESTED bullet is its own unit, and so is the next sibling bullet —
	// ranger-base-lcuec.md's own section names ordinary beads in full
	// sentences beside bare vuln-private ids, and those beads are not this
	// check's business (TestVulnPrivateHitsIgnoresUnknownIDs).
	for _, text := range []string{
		"- ranger-base-l751c\n  - ranger-base-wuuqi stays open, the operator's call\n",
		"- ranger-base-l751c\n- ranger-base-wuuqi stays open, the operator's call\n",
		"+- ranger-base-l751c\n+- ranger-base-wuuqi stays open, the operator's call\n",
		// And the control the lazy-continuation rule is NARROWED for
		// (ranger-base-ckcly finding 1): a HEADING wrapped onto a second
		// line does not take a lazy continuation — markdown opens a new
		// paragraph there — so a bare id on that second line is not charged
		// the heading's words. A rule that popped every opener alike reds
		// docs/notes.d/ranger-base-nr3qf.md, which is written this way, and
		// its remedy would be one the writer has already performed.
		"### Finding 1: the pin gap (escaped from\nranger-base-l751c)\n",
		"## Not closed here\nranger-base-l751c\n",
	} {
		if got := vulnPrivateHits(text, ids); got != nil {
			t.Errorf("vulnPrivateHits(%q) = %v, want no hit — a sibling or nested bullet about another bead, and the second line of a wrapped HEADING, are each a unit of their own", text, got)
		}
	}
}

// TestVulnPrivateHitsLeavesTheSeparateParagraphOpen RATIFIES the residual
// finding 1's fix deliberately leaves: a blank line closes every open unit,
// so a description in its own paragraph below a "Not closed here" section is
// not charged to the bare ids above it. MEASURED SILENT, rc 0, and it stays
// that way.
//
// This test is the decision and not an omission (vulnPrivateUnits' own
// comment carries the trade). Section scope would close it and would red
// ranger-base-lcuec.md, whose section names ordinary beads in full sentences
// — a refusal with no remedy the writer can perform, over text the rule does
// not bind, which is ADR 0068's class bought to close a shape nobody has
// typed. A later bead that decides otherwise reds HERE, with the reason in
// front of it, which is the point.
func TestVulnPrivateHitsLeavesTheSeparateParagraphOpen(t *testing.T) {
	t.Parallel()
	text := "## Not closed here\n\n- ranger-base-l751c\n\nthe heap overflow in the parser\n"
	if got := vulnPrivateHits(text, []string{"ranger-base-l751c"}); got != nil {
		t.Errorf("vulnPrivateHits = %v, want no hit — the separate-paragraph residual is DELIBERATE (ranger-base-68631 finding 1). If you widened the scope on purpose, read vulnPrivateUnits' comment first: section scope reds ranger-base-lcuec.md's own section, and then say so here rather than deleting this test", got)
	}
}

// TestVulnPrivateHitsAcceptsABareIDOnACRLFLine is ranger-base-68631
// finding 5: `\r` was not in the old decoration whitelist, so a BARE id on a
// CRLF line was refused, and the remedy it printed ("write the bare id
// alone") was the thing the writer had already done. MEASURED: `printf --
// '- <id>\r\n' | posse gates vuln-private-scan` -> rc 1.
//
// The base rate in this tree is honestly ZERO — `git grep -lI $'\r'` finds
// no tracked file with a CR and there is no .gitattributes eol policy — but
// the live route is the MESSAGE arm, which git does not normalize.
func TestVulnPrivateHitsAcceptsABareIDOnACRLFLine(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c"}
	for _, text := range []string{
		"- ranger-base-l751c\r\n",
		"## Not closed here\r\n\r\n- ranger-base-l751c\r\n",
	} {
		if got := vulnPrivateHits(text, ids); got != nil {
			t.Errorf("vulnPrivateHits(%q) = %v, want no hit — a CRLF line carrying a bare id is the compliant shape, and refusing it prints a remedy the writer already performed", text, got)
		}
	}
	// The control: CRLF does not buy a description a pass.
	if got := vulnPrivateHits("- ranger-base-l751c: the parser\r\n", ids); len(got) != 1 {
		t.Errorf("vulnPrivateHits over a CRLF line WITH a description = %v, want the id flagged", got)
	}
}

// TestVulnPrivateHitsAcceptsEveryDecorationTheRemedyPromises is
// ranger-base-68631 finding 6, held from BOTH ends: the remedy sentence
// promised "a list bullet, backticks, bold and surrounding punctuation" over
// a whitelist of eleven punctuation bytes, and ten shapes a writer actually
// types were refused although each carried a bare id. Either the sentence or
// the set was wrong, and nothing in the tree could say which.
//
// So each row names a noun the remedy SENTENCE uses and a line that noun
// describes. The row asserts the noun is still in the sentence and the line
// is still clean, so the sentence and the behaviour cannot drift apart in
// either direction — a narrowing of the rule that leaves the sentence alone
// reds, and a rewording that drops a promise reds too.
func TestVulnPrivateHitsAcceptsEveryDecorationTheRemedyPromises(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c"}
	for _, row := range []struct{ noun, line string }{
		{"whitespace", "  ranger-base-l751c\t"},
		{"list bullet", "- ranger-base-l751c"},
		{"numbering", "1. ranger-base-l751c"},
		{"backticks", "- `ranger-base-l751c`"},
		{"bold markers", "- **ranger-base-l751c**"},
		{"brackets", "[ranger-base-l751c]"},
		{"quotes", `"ranger-base-l751c"`},
		{"punctuation", "- ranger-base-l751c;"},
	} {
		if !strings.Contains(VulnPrivateWayThrough, row.noun) {
			t.Errorf("the remedy no longer promises %q, and this row exists because the sentence and the rule went out of step once already (ranger-base-68631 finding 6): %q", row.noun, VulnPrivateWayThrough)
		}
		if got := vulnPrivateHits(row.line, ids); got != nil {
			t.Errorf("vulnPrivateHits(%q) = %v, want no hit — the remedy promises %s is fine, so refusing it prints a way through that does not go through", row.line, got, row.noun)
		}
	}
	// The ten shapes finding 6 MEASURED as refused, each carrying a bare id.
	for _, line := range []string{
		"[ranger-base-l751c]", `"ranger-base-l751c"`, "'ranger-base-l751c'",
		"_ranger-base-l751c_", "<ranger-base-l751c>", "> ranger-base-l751c",
		"| ranger-base-l751c |", "• ranger-base-l751c", "— ranger-base-l751c",
		"ranger-base-l751c/",
	} {
		if got := vulnPrivateHits(line, ids); got != nil {
			t.Errorf("vulnPrivateHits(%q) = %v, want no hit — ranger-base-68631 finding 6 MEASURED all ten of these refused although each carries a bare id", line, got)
		}
	}
	// And the rule the sentence now names, in every script: a LETTER is what
	// is refused, not a byte that happened to be left off a whitelist.
	for _, line := range []string{
		"- ranger-base-l751c see",
		"- ranger-base-l751c переполнение",
		"- ranger-base-l751c 堆溢出",
	} {
		if got := vulnPrivateHits(line, ids); len(got) != 1 || got[0] != ids[0] {
			t.Errorf("vulnPrivateHits(%q) = %v, want [%s] — a word beside the id is the description the rule refuses, whatever its script", line, got, ids[0])
		}
	}
}

// TestVulnPrivateMessageRemedyPromisesOnlyWhatTheRuleAllows is
// ranger-base-ckcly finding 2, and it is the same shape as the test above
// one constant over. ranger-base-68631 finding 6 was filed against BOTH
// remedy sentences — VulnPrivateWayThrough and VulnPrivateMessageWayThrough
// — and only the first got a pin, so the message arm's sentence could drift
// back to the enumerate-the-bytes shape that finding was filed over and
// nothing in the tree would say so (reader census 2026-11-13: two comment
// lines and gates.go's one call site, no test).
//
// It is the arm that matters MORE, by finding 5's own reasoning: the content
// arm's base rate for a CR is zero because git normalizes what it stages,
// and "the live route is the MESSAGE arm, which git does not normalize".
//
// MEASURED through `go test -overlay`, tracked tree unmodified: before this
// test, re-promising what the rule refuses ("punctuation, decoration and a
// short label beside the id are fine") and reverting the sentence to a byte
// list both stayed GREEN.
func TestVulnPrivateMessageRemedyPromisesOnlyWhatTheRuleAllows(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c"}
	for _, row := range []struct{ noun, line string }{
		{"stands alone", "ranger-base-l751c"},
		{"punctuation", "- ranger-base-l751c;"},
		{"decoration", "- **`ranger-base-l751c`**"},
	} {
		if !strings.Contains(VulnPrivateMessageWayThrough, row.noun) {
			t.Errorf("the message remedy no longer promises %q, and this row exists because the sentence and the rule went out of step once already (ranger-base-68631 finding 6, unpinned on this arm until ranger-base-ckcly): %q", row.noun, VulnPrivateMessageWayThrough)
		}
		if got := vulnPrivateHits(row.line, ids); got != nil {
			t.Errorf("vulnPrivateHits(%q) = %v, want no hit — the message remedy promises %s is fine, so refusing it prints a way through that does not go through", row.line, got, row.noun)
		}
	}
	// And the half the sentence REFUSES, held the same way: it names a
	// letter, because that is the rule vulnPrivateBare implements. A
	// sentence that went back to enumerating bytes would drop this noun.
	if !strings.Contains(VulnPrivateMessageWayThrough, "a LETTER beside the id") {
		t.Errorf("the message remedy no longer names the rule it enforces — vulnPrivateBare refuses a LETTER, in any script, and a remedy that enumerates bytes instead is the shape ranger-base-68631 finding 6 was filed over: %q", VulnPrivateMessageWayThrough)
	}
	for _, line := range []string{
		"ranger-base-l751c heap overflow in the parser",
		"- ranger-base-l751c переполнение",
	} {
		if got := vulnPrivateHits(line, ids); len(got) != 1 || got[0] != ids[0] {
			t.Errorf("vulnPrivateHits(%q) = %v, want [%s] — the message remedy says a LETTER beside the id is out, so the rule must refuse one", line, got, ids[0])
		}
	}
}

// TestVulnPrivateHitsIsCaseInsensitive is ranger-base-68631 finding 7: the
// presence test was strings.Contains over the text as typed, so an
// UPPERCASED id with a description beside it was invisible. MEASURED:
// vulnPrivateHits("- RANGER-BASE-<ID> heap overflow", ids) -> nil.
//
// The id it REPORTS is the caller's own spelling — the hit line is the whole
// of what prints, and it has to name an id a reader can look up.
func TestVulnPrivateHitsIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c"}
	for _, line := range []string{
		"- RANGER-BASE-L751C heap overflow",
		"- Ranger-Base-L751c heap overflow",
		"- RANGER-BASE-L751C\n  heap overflow",
	} {
		got := vulnPrivateHits(line, ids)
		if len(got) != 1 || got[0] != ids[0] {
			t.Errorf("vulnPrivateHits(%q) = %v, want [%s] spelled as the caller spells it — a mechanical reading of an id has no business caring how it was shifted", line, got, ids[0])
		}
	}
	if got := vulnPrivateHits("- RANGER-BASE-L751C", ids); got != nil {
		t.Errorf("vulnPrivateHits over a bare uppercased id = %v, want no hit", got)
	}
	// And the caller's own ids may be shifted too: the fold is on both sides.
	if got := vulnPrivateHits("- ranger-base-l751c heap overflow", []string{"RANGER-BASE-L751C"}); len(got) != 1 || got[0] != "RANGER-BASE-L751C" {
		t.Errorf("vulnPrivateHits with an uppercased id from the caller = %v, want [RANGER-BASE-L751C]", got)
	}
}

// qaFakeBd points a Bd at the package's fake-binary harness (herdr_test.go)
// so RunGatesVulnPrivateScan reads fixture JSON instead of a real store.
func qaFakeBd(t *testing.T) Bd {
	t.Helper()
	return Bd{Bin: fakeBinFor(t, "bd")}
}

func writeFakeOpenLabeled(t *testing.T, dir, json string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "fake-list-labeled.json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunGatesVulnPrivateScanFlagsAnOpenResidualWithText is the end-to-end
// shape: one open vuln-private bead, named with a description on stdin.
func TestRunGatesVulnPrivateScanFlagsAnOpenResidualWithText(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFakeOpenLabeled(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["code","security","vuln-private"]}]`)
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, strings.NewReader("## Not closed here\n- ranger-base-l751c: the remote-settings cache\n"), &out)
	if !hit {
		t.Fatalf("want a hit, got none; output: %q", out.String())
	}
	if !strings.Contains(out.String(), "ranger-base-l751c") {
		t.Errorf("output must name the flagged id: %q", out.String())
	}
	if strings.Contains(out.String(), "remote-settings") {
		t.Errorf("output must withhold the description, class-only: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanBareIDIsClean: the same bead, named bare —
// exactly what the wall's own remedy prescribes — must not be flagged.
func TestRunGatesVulnPrivateScanBareIDIsClean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFakeOpenLabeled(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["code","security","vuln-private"]}]`)
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, strings.NewReader("## Not closed here\n- ranger-base-l751c\n"), &out)
	if hit {
		t.Errorf("a bare id must not be flagged, got: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanClosedResidualIsNotFlagged: the rule's own
// words are "until that residual's own fix lands" — a closed bead is no
// longer a residual, and OpenLabeledAny's whole job is dropping it on
// every store class (openlabeledany_storeclass_test.go).
func TestRunGatesVulnPrivateScanClosedResidualIsNotFlagged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFakeOpenLabeled(t, dir, `[{"id":"ranger-base-l751c","status":"closed","labels":["code","security","vuln-private"]}]`)
	if err := os.WriteFile(filepath.Join(dir, "fake-list-keep-closed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, strings.NewReader("- ranger-base-l751c: now public, the fix shipped\n"), &out)
	if hit {
		t.Errorf("a closed vuln-private residual is no longer this check's business, got: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanFailsOpenWithNoBd is ADR 0068's lesson,
// applied: this check's own answer to "what does it do when it cannot
// reach the store" is never refuse — a refusal with no remedy the
// committer can perform is a lockout, not a gate.
func TestRunGatesVulnPrivateScanFailsOpenWithNoBd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bd := Bd{Bin: filepath.Join(dir, "no-such-bd-binary")}
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(bd, dir, strings.NewReader("- ranger-base-l751c: a description that would otherwise trip this\n"), &out)
	if hit {
		t.Errorf("no bd on PATH must fail open, got: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanEmptyStdinIsClean: nothing to scan is nothing
// to flag, and the fake binary must never even be asked.
func TestRunGatesVulnPrivateScanEmptyStdinIsClean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No fixture written at all: if this call reaches bd, the fake answers
	// with its own usage error rather than an issue list, and a bug that
	// skipped the empty-input guard would surface as a false hit here.
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, strings.NewReader(""), &out)
	if hit {
		t.Errorf("empty input must never hit, got: %q", out.String())
	}
}
