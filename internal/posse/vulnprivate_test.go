package posse

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// EVERY BEAD ID IN THIS FILE'S TABLES IS CLOSED OR SPELLS NO BEAD AT ALL,
// and that is a constraint and not a style (ranger-base-ql7xy). Since
// ranger-base-cq3vi widened check 4's content arm from the markdown
// pathspecs to every staged path, `.go` is in scope — so a fixture that
// spells a LIVE open `vuln-private` id beside descriptive text is the exact
// shape the gate refuses, and whoever next edits one of these lines has
// their own commit refused over a hole that is not theirs. The commit-time
// own-bead exemption cannot help: it drops the one id the branch is
// recorded for (`branch.<branch>.posseBead`), never a fixture's.
//
// `ranger-base-fixt3` is the synthetic id, shared with
// TestVulnPrivateHitsAcceptsTheResidualLabel below: no bead has it, so the
// judge's open set never contains it and the row reads the same forever.
// `ranger-base-l751c` is a real bead and stays because it is CLOSED, which
// is what vulnPrivateHits' caller filters on (Bd.OpenIssues drops closed
// rows on both store classes) — but a closed bead is only closed once, so
// prefer the synthetic id for anything added here.

// TestVulnPrivateHitsBareIDIsClean is the compliant shape AGENTS.md
// "Landing the plane" prescribes, every form the rule's own prose and
// ranger-base-lcuec.md's own "Not closed here" use: a bullet, backticks,
// bold, numbering, a trailing colon, two bare ids sharing one bullet.
func TestVulnPrivateHitsBareIDIsClean(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c", "ranger-base-fixt3"}
	for _, line := range []string{
		"ranger-base-l751c",
		"- ranger-base-l751c",
		"* ranger-base-l751c",
		"1. ranger-base-l751c",
		"- `ranger-base-l751c`",
		"- **ranger-base-l751c**",
		"- ranger-base-l751c:",
		"- ranger-base-l751c, ranger-base-fixt3",
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
	text := "wire the reaper\n\n## Not closed here\n- ranger-base-l751c\n- ranger-base-fixt3: the remote-settings cache\n"
	got := vulnPrivateHits(text, []string{"ranger-base-l751c", "ranger-base-fixt3"})
	if len(got) != 1 || got[0] != "ranger-base-fixt3" {
		t.Errorf("vulnPrivateHits = %v, want [ranger-base-fixt3] — the bare line above it must not be flagged", got)
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
// indented bare id is the same escape upside down.
//
// EVERY ROW HERE IS PLAIN MARKDOWN, and the `+`-prefixed content-arm
// spellings that used to sit among them have moved to
// TestRunGatesVulnPrivateScanReadsTheContentArmsDiffMarker. That is
// ranger-base-gi6ge: the diff marker is discounted ONCE at the arm boundary
// (vulnPrivateUndiff) instead of inside these readers, so this function sees
// markdown and nothing else — which is what lets the `+` BULLET rows below
// be read as bullets at all.
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
		// ranger-base-ckcly finding 1: the LAZY continuation, one space
		// fewer than the row above it. Markdown reads a column-0 line under
		// a list item as a continuation of that item; the pop loop read it
		// as a unit of its own, so this passed SILENT, rc 0, through the
		// shipped subcommand.
		"- ranger-base-l751c\nthe heap overflow in the parser\n",
		"1. ranger-base-l751c\nthe heap overflow in the parser\n",
		"## Not closed here\n\n- ranger-base-l751c\nthe heap overflow in the parser\n",
		// ranger-base-gi6ge: EVERY bullet spelling vulnPrivateListItem
		// names, because the lazy continuation it carries is the list
		// item's and a spelling that is not read as a list item does not
		// carry one. `-` and `1.` are above; these two are the rest.
		//
		// `+` is the one that escaped, and it escaped in the MESSAGE arm:
		// the diff-marker strip used to run inside vulnPrivateListItem's
		// own reader, so a message line `+ <id>` lost its `+` before
		// anything could read a bullet there, read as prose at indent 1,
		// and the description below it bought a unit of its own. MEASURED
		// 2026-11-14 end-to-end through the shipped subcommand, one open
		// id with the description at column 0 on the next line: `-`, `*`
		// and `1.` each rc 1, `+` rc 0 — and `+` with the description
		// INDENTED rc 1, so one space was the whole of the difference.
		"* ranger-base-l751c\nthe heap overflow in the parser\n",
		"+ ranger-base-l751c\nthe heap overflow in the parser\n",
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
		"+ ranger-base-l751c\n+ ranger-base-wuuqi stays open, the operator's call\n",
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
//
// THE THIRD DIRECTION IS NOT HERE AND IS NOT NOWHERE
// (ranger-base-mxbq0 finding 1): a clause APPENDED to either check-4 remedy
// displaces no noun below, so nothing shaped like these rows can see it.
// TestQARemedySentencesAreClosedSets (remedyclosedset_test.go) reads all
// four of the tree's remedy constants as closed sets for that, and reds on
// an edit until its author has come back here and given the new clause a
// row. remedyPromises is that file's helper too — one spelling over both
// checks, where this file's plain strings.Contains and check 3's
// whitespace-collapsing Contains used to be two.
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
		if !remedyPromises(VulnPrivateWayThrough, row.noun) {
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
		if !remedyPromises(VulnPrivateMessageWayThrough, row.noun) {
			t.Errorf("the message remedy no longer promises %q, and this row exists because the sentence and the rule went out of step once already (ranger-base-68631 finding 6, unpinned on this arm until ranger-base-ckcly): %q", row.noun, VulnPrivateMessageWayThrough)
		}
		if got := vulnPrivateHits(row.line, ids); got != nil {
			t.Errorf("vulnPrivateHits(%q) = %v, want no hit — the message remedy promises %s is fine, so refusing it prints a way through that does not go through", row.line, got, row.noun)
		}
	}
	// And the half the sentence REFUSES, held the same way: it names a
	// letter, because that is the rule vulnPrivateBare implements. A
	// sentence that went back to enumerating bytes would drop this noun.
	if !remedyPromises(VulnPrivateMessageWayThrough, "a LETTER beside the id") {
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

// writeFakeOpenIssues seeds the fixture the scan's ONE query reads. Since
// ranger-base-yno5e that query is bd.OpenIssues — a bare `bd list --json`,
// no `--label-any` — so the fake binary serves it fake-list.json and not
// fake-list-labeled.json (herdr_test.go's `list` case: the labeled file is
// the `--label-any` query's own, with an empty default).
func writeFakeOpenIssues(t *testing.T, dir, json string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "fake-list.json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunGatesVulnPrivateScanFlagsAnOpenResidualWithText is the end-to-end
// shape: one open vuln-private bead, named with a description on stdin.
func TestRunGatesVulnPrivateScanFlagsAnOpenResidualWithText(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFakeOpenIssues(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["code","security","vuln-private"]}]`)
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader("## Not closed here\n- ranger-base-l751c: the remote-settings cache\n"), &out)
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
	writeFakeOpenIssues(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["code","security","vuln-private"]}]`)
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader("## Not closed here\n- ranger-base-l751c\n"), &out)
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
	writeFakeOpenIssues(t, dir, `[{"id":"ranger-base-l751c","status":"closed","labels":["code","security","vuln-private"]}]`)
	if err := os.WriteFile(filepath.Join(dir, "fake-list-keep-closed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader("- ranger-base-l751c: now public, the fix shipped\n"), &out)
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
	hit := RunGatesVulnPrivateScan(bd, dir, VulnPrivateMessageArm, strings.NewReader("- ranger-base-l751c: a description that would otherwise trip this\n"), &out)
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
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader(""), &out)
	if hit {
		t.Errorf("empty input must never hit, got: %q", out.String())
	}
}

// ranger-base-yno5e: check 4's id set is the label OR the marker line. One
// row per arm the bead names — (a) marker without label flagged, (b) label
// without marker still flagged, (c) the word in prose is not a request,
// (d) a closed marker-only bead is no longer a residual.
//
// NO REAL BEAD ID IS FIXTURE DATA HERE, which is ranger-base-crwxs:
// `ranger-base-zzzz1` is made up and resolves to nothing in any store, the
// same spelling parkedwork_test.go uses for the same reason. This table was
// first written against a real, OPEN `vuln-private` id, beside placeholder
// descriptions ("the mechanism, spelled out") that described nothing of it —
// and the push gate held the commit anyway, correctly: a public tree does
// not carry an open residual's id next to the shape of a disclosure, and
// nobody reading the fixture can tell a placeholder from a leak. The rule
// the fixture exercises is the rule the fixture obeys.
func TestRunGatesVulnPrivateScanMarkerOrLabel(t *testing.T) {
	t.Parallel()
	const said = "- ranger-base-zzzz1: the mechanism, spelled out\n"
	for _, c := range []struct {
		name, fixture string
		keepClosed    bool
		want          bool
	}{
		{"marker-without-label", `[{"id":"ranger-base-zzzz1","status":"open","labels":["security"],"description":"VULN-PRIVATE: name by id only in public text.\nmore"}]`, false, true},
		{"marker-after-blank-lines", `[{"id":"ranger-base-zzzz1","status":"blocked","description":"\n  \nVULN-PRIVATE: name by id only."}]`, false, true},
		{"marker-crlf", `[{"id":"ranger-base-zzzz1","status":"deferred","description":"VULN-PRIVATE: id only.\r\nmore"}]`, false, true},
		{"label-without-marker", `[{"id":"ranger-base-zzzz1","status":"in_progress","labels":["vuln-private"],"description":"a plain description"}]`, false, true},
		{"word-mid-prose", `[{"id":"ranger-base-zzzz1","status":"open","labels":["security"],"description":"A bead whose description says VULN-PRIVATE: but no label."}]`, false, false},
		{"marker-not-on-line-one", `[{"id":"ranger-base-zzzz1","status":"open","description":"Finding.\nVULN-PRIVATE: id only."}]`, false, false},
		{"word-without-colon", `[{"id":"ranger-base-zzzz1","status":"open","description":"VULN-PRIVATE beads are named by id."}]`, false, false},
		{"lowercase-marker", `[{"id":"ranger-base-zzzz1","status":"open","description":"vuln-private: id only."}]`, false, false},
		{"closed-marker-only", `[{"id":"ranger-base-zzzz1","status":"closed","description":"VULN-PRIVATE: id only."}]`, true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFakeOpenIssues(t, dir, c.fixture)
			if c.keepClosed {
				if err := os.WriteFile(filepath.Join(dir, "fake-list-keep-closed"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out strings.Builder
			hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader(said), &out)
			if hit != c.want {
				t.Fatalf("hit = %v, want %v; output: %q", hit, c.want, out.String())
			}
			if hit && strings.Contains(out.String(), "mechanism") {
				t.Errorf("output must withhold the text: %q", out.String())
			}
		})
	}
}

// ─── the two arms, which are one byte apart (ranger-base-gi6ge) ─────────────

// TestRunGatesVulnPrivateScanReadsAPlusBulletInTheMessage is
// ranger-base-gi6ge, end-to-end through the judge the hook calls: a `+`
// bullet in a COMMIT MESSAGE is a list item like any other, so the line
// below it is that item's lazy continuation.
//
// WHAT FAILED. vulnPrivateListItem names four bullet spellings — `-`, `*`,
// `+` and a number — and ranger-base-ckcly's lazy-continuation fix reached
// three of them. The diff-marker strip ran inside the line readers, so a
// message line `+ <id>` lost its `+` before anything could read a bullet
// there: it read as prose at indent 1, carried no lazy continuation, and the
// description below it bought a unit of its own and passed SILENT. The
// comment that reasoned the risk away said such a line loses "a column of
// indent, which can only make a unit narrower" — narrower by exactly the
// line that holds the description.
//
// MEASURED 2026-11-14 on the shipped tree, these five texts through `posse
// gates vuln-private-scan` against an open vuln-private id: rows 1-4 rc 1,
// row 5 rc 0. The fix is vulnPrivateUndiff — one strip at the ARM BOUNDARY
// and none inside the readers — and the arm is a parameter because a `+` is
// a bullet in one arm and a marker in the other, which the text cannot say.
func TestRunGatesVulnPrivateScanReadsAPlusBulletInTheMessage(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"- ranger-base-l751c\nthe heap overflow in the parser\n",
		"* ranger-base-l751c\nthe heap overflow in the parser\n",
		"1. ranger-base-l751c\nthe heap overflow in the parser\n",
		// The row that was already caught, and the one space that was the
		// whole of the difference from the row below it.
		"+ ranger-base-l751c\n  the heap overflow in the parser\n",
		// THIS ONE. Silent on the shipped tree.
		"+ ranger-base-l751c\nthe heap overflow in the parser\n",
	} {
		dir := t.TempDir()
		writeFakeOpenIssues(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]}]`)
		var out strings.Builder
		if !RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader(text), &out) {
			t.Errorf("scan(%q) over the MESSAGE arm = no hit, want one — every bullet spelling vulnPrivateListItem names carries a lazy continuation, `+` included", text)
		}
		if strings.Contains(out.String(), "heap") {
			t.Errorf("the output must withhold the description, class-only: %q", out.String())
		}
	}
}

// TestRunGatesVulnPrivateScanReadsTheContentArmsDiffMarker is the other arm,
// and it is what the fix above must not cost. The content arm's text is `git
// diff --cached -U0 … | grep -a '^+'` (gates.go's $posse_added), so every
// line of it carries a unified-diff `+` that is no part of the file and the
// indentation this check measures sits AFTER it — without the discount,
// `+- <id>` and `+  <description>` both read as column 0 and the
// continuation rule is dead in the one arm that reads a file.
//
// The `++ <id>` rows are the proof the discount is a DISCOUNT and not a ban
// on the byte: a markdown `+` bullet staged in a file arrives here as `++`,
// keeps one `+` after the strip, and is read as the bullet it is. Dropping
// `+` from vulnPrivateListItem instead of moving the strip would have lost
// exactly these.
func TestRunGatesVulnPrivateScanReadsTheContentArmsDiffMarker(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"+- ranger-base-l751c\n+  the heap overflow in the parser\n",
		"+- ranger-base-l751c\n+the heap overflow in the parser\n",
		"++ ranger-base-l751c\n+  the heap overflow in the parser\n",
		"++ ranger-base-l751c\n+the heap overflow in the parser\n",
		"+1. ranger-base-l751c\n+the heap overflow in the parser\n",
	} {
		dir := t.TempDir()
		writeFakeOpenIssues(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]}]`)
		var out strings.Builder
		if !RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateContentArm, strings.NewReader(text), &out) {
			t.Errorf("scan(%q) over the CONTENT arm = no hit, want one — the diff marker is discounted, so what is left is a bullet and its continuation", text)
		}
	}
	// The controls, so the discount cannot pass by flagging everything.
	for _, text := range []string{
		// A sibling bullet about another bead, which is the shape
		// ranger-base-lcuec.md's own "Not closed here" uses.
		"+- ranger-base-l751c\n+- ranger-base-wuuqi stays open, the operator's call\n",
		// A BARE id, which is what the wall's own remedy prescribes.
		"+- ranger-base-l751c\n",
		// Declared residual 3 read through this arm: the file's own lines
		// are ` <id>` and `the heap overflow …`, which markdown reads as
		// ONE paragraph and this grouping reads as two units — a prose run
		// at column 0 with no list item above it (vulnPrivateUnits' third
		// residual). Stated, not new: the same text in the message arm is
		// the `+ <id>` row of the test above, and the arm is the whole of
		// the difference between them.
		"+ ranger-base-l751c\n+the heap overflow in the parser\n",
	} {
		dir := t.TempDir()
		writeFakeOpenIssues(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]}]`)
		var out strings.Builder
		if RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateContentArm, strings.NewReader(text), &out) {
			t.Errorf("scan(%q) over the CONTENT arm = %q, want no hit", text, out.String())
		}
	}
}

// TestVulnPrivateChecksSpellsTheArmPerArm: the judge cannot read its arm off
// the text, so the RENDER has to say it — and only the message arm says
// anything, because an installed hook older than this binary says nothing at
// all and that window must behave as the content arm always did
// (VulnPrivateArm's own comment, ranger-base-gi6ge).
func TestVulnPrivateChecksSpellsTheArmPerArm(t *testing.T) {
	t.Parallel()
	content := vulnPrivateContentCheck()
	if strings.Contains(content, "vuln-private-scan --message") {
		t.Errorf("check 4's CONTENT arm must not pass --message — its lines each carry a diff marker:\n%s", content)
	}
	if !strings.Contains(content, "| posse gates vuln-private-scan 2>/dev/null") {
		t.Errorf("check 4's CONTENT arm must call the judge with no arm flag:\n%s", content)
	}
	msg := vulnPrivateSource().checks("  ")
	if !strings.Contains(msg, "| posse gates vuln-private-scan --message 2>/dev/null") {
		t.Errorf("check 4's MESSAGE arm must pass --message — a `+` there is a markdown bullet, not a diff marker:\n%s", msg)
	}
}

// ─── the committing tree's OWN bead (ranger-base-ed07e) ─────────────────────

// vpOwnTree is a session worktree of a one-commit repo, on a session branch
// whose `posseBead` record names bead, with the fake bd fixture staged in
// the WORKTREE — which is the dir the hook's cwd is and the dir the scan is
// handed. A real repo is the whole point: the record lives in git config and
// the branch is read with git's own plumbing, so a `mkdir .git` stand-in
// would measure nothing.
func vpOwnTree(t *testing.T, bead, issues string) string {
	t.Helper()
	repo := wtRepo(t)
	session := "ranger-posse-" + bead
	branch := SessionBranch(session)
	wt := filepath.Join(gitTempDir(t), session)
	mustGit(t, repo, "worktree", "add", "-q", "-b", branch, wt)
	if bead != "" {
		if err := recordBead(repo, branch, bead); err != nil {
			t.Fatal(err)
		}
	}
	writeFakeOpenIssues(t, wt, issues)
	return wt
}

// TestRunGatesVulnPrivateScanExemptsTheCommittingTreesOwnBead is the whole
// of ranger-base-ed07e: a dispatched fix must commit BEFORE its bead closes,
// because the close is what fast-forwards the branch — so for the only
// window in which the fix is committable its own bead is open, and the wall
// read the fix's own subject line as a disclosure. AGENTS.md "Landing the
// plane" already allows a commit message or notes fragment to describe the
// hole its fix CLOSES; this is where that permission is realized.
func TestRunGatesVulnPrivateScanExemptsTheCommittingTreesOwnBead(t *testing.T) {
	t.Parallel()
	dir := vpOwnTree(t, "ranger-base-l751c",
		`[{"id":"ranger-base-l751c","status":"in_progress","labels":["code","security","vuln-private"]}]`)
	for _, text := range []string{
		// The house commit subject, which is what the seven bare-id
		// commits of 2026-11-13/14 had to give up.
		"ranger-base-l751c: the seatbelt render denies the config dir\n",
		// And the fix's own notes fragment, held back on three beads.
		"## Not closed here\n\n- ranger-base-l751c: the mechanism, in a bullet\n",
	} {
		var out strings.Builder
		if RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader(text), &out) {
			t.Errorf("the committing tree's own bead must not be flagged over %q, got: %q", text, out.String())
		}
	}
}

// TestRunGatesVulnPrivateScanStillFlagsAnotherOpenResidual is the exemption's
// control, and the only reason it is one id and not a waiver: the "Not closed
// here" section AGENTS.md prescribes names OTHER beads, and the hold-back
// incident that wrote the rule was a fix describing the NEXT hole.
func TestRunGatesVulnPrivateScanStillFlagsAnotherOpenResidual(t *testing.T) {
	t.Parallel()
	dir := vpOwnTree(t, "ranger-base-l751c", `[
		{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]},
		{"id":"ranger-base-fixt2","status":"open","labels":["vuln-private"]}]`)
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader(
		"ranger-base-l751c: the fix this commit carries\n\nNot closed here:\n- ranger-base-fixt2 the residual's mechanism\n"), &out)
	if !hit {
		t.Fatalf("another open residual named with text must still be flagged; output: %q", out.String())
	}
	if strings.Contains(out.String(), "ranger-base-l751c") {
		t.Errorf("the exempted own bead must not appear in the refusal: %q", out.String())
	}
	if !strings.Contains(out.String(), "ranger-base-fixt2") {
		t.Errorf("the refusal must name the id it flagged: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanStripsTheExemptedOwnBeadToo is ranger-base-n2veh
// finding 1, escaped from the exemption above (commit e3b41b30). The own id
// was dropped from the ids handed to vulnPrivateHits, which took it out of
// the PRESENCE test (intended) and out of the STRIP that runs before the
// bareness test (not intended) — so the own id's own letters were read as a
// description of whatever OTHER open id shared its unit, and a unit carrying
// nothing but those two ids was REFUSED naming the other one. The remedy it
// printed — "a flagged id names ANOTHER hole", "write the bare id alone" —
// had already been performed, which is ADR 0068's class and the cost
// vulnprivate.go's own comments cite as the one to avoid.
//
// MEASURED 2026-11-14 at 025d2e58, both shapes below HIT and named
// ranger-base-fixt2.
func TestRunGatesVulnPrivateScanStripsTheExemptedOwnBeadToo(t *testing.T) {
	t.Parallel()
	dir := vpOwnTree(t, "ranger-base-l751c", `[
		{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]},
		{"id":"ranger-base-fixt2","status":"open","labels":["vuln-private"]}]`)
	// Two bare ids sharing one unit read as bare for BOTH, which is the
	// invariant vulnPrivateHits' doc comment states. Exempting one of them
	// from the report must not turn the other's bullet into a hit.
	for _, text := range []string{
		"Not closed here:\n- ranger-base-l751c, ranger-base-fixt2\n",
		"ranger-base-l751c ranger-base-fixt2\n",
	} {
		var out strings.Builder
		if RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader(text), &out) {
			t.Errorf("a unit of nothing but bare ids must be clean over %q, got: %q", text, out.String())
		}
	}
	// Two controls that lived in the same MEASURED run: the exemption must
	// not have become a waiver, and the own bead is still exempt alone.
	var out strings.Builder
	if !RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader(
		"Not closed here:\n- ranger-base-fixt2 the residual's mechanism\n"), &out) {
		t.Error("another open residual named with text must still be flagged")
	}
	out.Reset()
	if RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader(
		"ranger-base-l751c: the fix this commit carries\n"), &out) {
		t.Errorf("the committing tree's own bead must still be exempt, got: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanExemptsNothingWithNoBranchRecord: a tree that
// answers for no bead gets no exemption. That is the shared checkout's
// `main` — where the committer is one of eleven personas and git cannot say
// which — and every tree made by hand. "" is never read as a bead.
func TestRunGatesVulnPrivateScanExemptsNothingWithNoBranchRecord(t *testing.T) {
	t.Parallel()
	dir := vpOwnTree(t, "", `[{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]}]`)
	var out strings.Builder
	if !RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader("ranger-base-l751c: a description\n"), &out) {
		t.Errorf("no branch record is no exemption, got no hit: %q", out.String())
	}
	// And the main checkout itself, on `main`, with the same bead open.
	repo := wtRepo(t)
	writeFakeOpenIssues(t, repo, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]}]`)
	out.Reset()
	if !RunGatesVulnPrivateScan(qaFakeBd(t), repo, VulnPrivateMessageArm, strings.NewReader("ranger-base-l751c: a description\n"), &out) {
		t.Errorf("the shared checkout's main carries no record and gets no exemption, got no hit: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanExemptsTheOwnBeadOnADetachedHead: a
// container-tier seat is launched on a detached HEAD on purpose
// (PrepareSessionHead, ranger-base-t4f1), so `symbolic-ref` answers nothing
// and the branch comes from the linked worktree's own admin dir name — which
// IS the session, and SessionBranch is the branch. The BEAD still comes from
// the record, so this measures the fallback and not a parse of the name.
func TestRunGatesVulnPrivateScanExemptsTheOwnBeadOnADetachedHead(t *testing.T) {
	t.Parallel()
	dir := vpOwnTree(t, "ranger-base-l751c",
		`[{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]}]`)
	mustGit(t, dir, "checkout", "-q", "--detach")
	if _, err := git(dir, "symbolic-ref", "--short", "HEAD"); err == nil {
		t.Fatal("this tree was supposed to be detached; the fallback is not under test")
	}
	var out strings.Builder
	if RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader("ranger-base-l751c: the fix\n"), &out) {
		t.Errorf("a detached session tree's own bead must still be exempt, got: %q", out.String())
	}
}

// TestVulnPrivateOwnBeadIsNotAParseOfTheWorktreeName is the exemption's
// narrowest pin: the admin dir name names the BRANCH, and nothing more. A
// worktree someone made by hand, on a branch with no `posseBead` record,
// answers "" even though its name ends in something id-shaped — because
// SessionForBead joins persona, repo and bead with '-' and all three may
// contain one, which is why unaccountedFor refuses the same parse.
func TestVulnPrivateOwnBeadIsNotAParseOfTheWorktreeName(t *testing.T) {
	t.Parallel()
	repo := wtRepo(t)
	wt := filepath.Join(gitTempDir(t), "ranger-posse-ranger-base-l751c")
	mustGit(t, repo, "worktree", "add", "-q", "-b", "posse/ranger-posse-ranger-base-l751c", wt)
	if got := vulnPrivateOwnBead(wt); got != "" {
		t.Errorf("vulnPrivateOwnBead over an unrecorded branch = %q, want \"\" — the name is not the record", got)
	}
	// DETACHED, which is the only path the admin-dir name is read on at
	// all: attached, `symbolic-ref` answers and the name is never consulted,
	// so a pin that stops here leaves the fallback's own parse unmeasured.
	// MEASURED: a mutant that falls back to `strings.Split(name, "-")[2:]`
	// survives the attached half of this test and reds on this line.
	mustGit(t, wt, "checkout", "-q", "--detach")
	if got := vulnPrivateOwnBead(wt); got != "" {
		t.Errorf("vulnPrivateOwnBead over an unrecorded DETACHED worktree = %q, want \"\" — the admin dir name names the BRANCH, and the bead comes from the record or from nowhere", got)
	}
	if got := vulnPrivateOwnBead(repo); got != "" {
		t.Errorf("vulnPrivateOwnBead over the main checkout = %q, want \"\"", got)
	}
	// Not a repo at all, which is every existing end-to-end test's dir.
	if got := vulnPrivateOwnBead(t.TempDir()); got != "" {
		t.Errorf("vulnPrivateOwnBead outside a repo = %q, want \"\"", got)
	}
}

// TestVulnPrivateHitsAcceptsTheResidualLabel is ranger-base-cq3vi's label
// allowance: since check 4 reads source, this tree's `// Residual: <id>.`
// shape is in scope, and the label says the id IS a residual and nothing
// about what it is. The allowance is whole words from a closed list, so the
// second table — the label with anything else beside it — must still red,
// the printed-string shape of finding F1 among them.
func TestVulnPrivateHitsAcceptsTheResidualLabel(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c", "ranger-base-fixt3"}
	for _, text := range []string{
		"// Residual: ranger-base-l751c.",
		"\t// Residual: ranger-base-l751c.",
		"// Residuals: ranger-base-l751c, ranger-base-fixt3.",
		"- Residual: ranger-base-l751c",
		"# Residual: ranger-base-l751c",
		"//\n// Residual: ranger-base-l751c.",
	} {
		if got := vulnPrivateHits(text, ids); got != nil {
			t.Errorf("vulnPrivateHits(%q) = %v, want no hit — a residual label beside a bare id describes nothing", text, got)
		}
	}
	for _, text := range []string{
		"// Residual: ranger-base-l751c. The ssh tmpfs is a mask",
		"// Residual mask: ranger-base-l751c.",
		"// residualmask ranger-base-l751c",
		"// Residual (credential): ranger-base-l751c",
		`fmt.Fprintf(out, "    · %s (NO MOUNT; ranger-base-l751c)\n", AbbrevHome(p))`,
	} {
		if got := vulnPrivateHits(text, ids); len(got) != 1 || got[0] != ids[0] {
			t.Errorf("vulnPrivateHits(%q) = %v, want [%s] — only the exact label words may stand beside an open id", text, got, ids[0])
		}
	}
}

// TestVulnPrivateContentArmReadsEveryStagedFileButTheBeadsJsonl is finding
// F2 of ranger-base-cq3vi, measured through real git rather than read off the
// render: check 4's content arm used to diff the markdown pathspecs alone, so
// a residual described in a .go comment or a printed string never reached the
// judge. The rendered arm must carry vulnPrivateContentPathspec, and that
// pathspec, run through the arm's own diff shape from a SUBDIRECTORY (the
// `:(top)` reason, ceilingRefusePathspec's comment), must yield the .go and
// .md additions and not the beads jsonl's.
func TestVulnPrivateContentArmReadsEveryStagedFileButTheBeadsJsonl(t *testing.T) {
	t.Parallel()
	arm := vulnPrivateContentCheck()
	if !strings.Contains(arm, "-- "+vulnPrivateContentPathspec+" ") {
		t.Fatalf("check 4's content arm no longer diffs vulnPrivateContentPathspec:\n%s", arm)
	}
	if strings.Contains(arm, markdownPathspecArgs()) {
		t.Fatalf("check 4's content arm is back to the markdown pathspecs — a .go comment or printed string is invisible to it again:\n%s", arm)
	}

	repo := gitTempDir(t)
	qblGit(t, repo, "init", "-q")
	qblGit(t, repo, "config", "user.email", "t@example.com")
	qblGit(t, repo, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "seed"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	qblGit(t, repo, "add", "--", "seed")
	qblGit(t, repo, "commit", "-q", "-m", "seed")
	files := map[string]string{
		"internal/x.go": "// GO-LINE\n",
		"docs/x.md":     "MD-LINE\n",
		// ranger-base-pdcf6's rows, end to end: a bare id, then content
		// that LOOKS like a file header, then the description. The file is
		// named to sort after docs/x.md so the id is not the last thing in
		// the staged stream either.
		"docs/y.md":           "- ranger-base-l751c\n++ a content line\n\tthe heap overflow\n+++ b/bin/bob-watch\n",
		".beads/issues.jsonl": "JSONL-LINE\n",
	}
	for p, body := range files {
		full := filepath.Join(repo, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		qblGit(t, repo, "add", "--", p)
	}
	// THE PIPELINE IS THE ARM'S OWN, not a second spelling of it: the grep
	// is built from the same constant the render uses and the render is
	// asserted to carry it, so a change to one of them cannot leave this
	// test measuring the other (ranger-base-pdcf6).
	grep := `grep -a -e '^+' -e '^` + vulnPrivateDiffHeader + `'`
	if !strings.Contains(arm, grep) {
		t.Fatalf("check 4's content arm no longer greps %s, so the stream below is not the one the judge reads:\n%s", grep, arm)
	}
	cmd := exec.Command("sh", "-c", `git diff --cached -U0 `+diffReaderShape+` HEAD -- `+vulnPrivateContentPathspec+` | `+grep)
	cmd.Dir = filepath.Join(repo, "internal")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("diff: %v\n%s", err, out)
	}
	got := string(out)
	// And the FILE BOUNDARY comes through the same pipeline
	// (ranger-base-4og5l): the `+++ b/<path>` header is the only thing left
	// in a -U0 stream that says where one file's added lines end, and this
	// arm's judge groups a line with the lines that CONTINUE it. The second
	// grep that used to drop it is gone from this arm and must not come
	// back; vulnPrivateUndiff blanks the header, path and all.
	if strings.Contains(arm, `grep -av '^+++'`) {
		t.Errorf("check 4's content arm drops the '+++' file boundaries again — a bare id last in its file is continued by the next file's added line:\n%s", arm)
	}
	for _, want := range []string{"+++ b/internal/x.go", "+++ b/docs/x.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("check 4's reader lost the %q file boundary:\n%s", want, got)
		}
	}
	// And the line that says WHICH header is a header (ranger-base-pdcf6):
	// `+++ …` is also what an added line beginning `++` looks like here, so
	// the judge reads the boundary as a `diff --git ` line plus the `+++`
	// line after it. Without these in the stream nothing is blanked at all.
	for _, want := range []string{
		vulnPrivateDiffHeader + "a/internal/x.go b/internal/x.go",
		vulnPrivateDiffHeader + "a/docs/x.md b/docs/x.md",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("check 4's reader lost the %q line, which is the only boundary an added line cannot imitate:\n%s", want, got)
		}
	}
	for _, want := range []string{"+// GO-LINE", "+MD-LINE"} {
		if !strings.Contains(got, want) {
			t.Errorf("check 4's staged set is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "JSONL-LINE") {
		t.Errorf("check 4's staged set reads the beads jsonl, which is the bead's own record and bd stages into every commit:\n%s", got)
	}

	// END TO END, which is the whole point of running real git here: the
	// stream above, judged. docs/y.md holds a bare open id, a content line
	// beginning `++` and a tab-indented description — the shape
	// ranger-base-pdcf6 measured passing, because the `++` line arrived as
	// `+++` and was blanked, closing the id's unit. It is a HIT again.
	writeFakeOpenIssues(t, repo, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]}]`)
	var judged strings.Builder
	if !RunGatesVulnPrivateScan(qaFakeBd(t), repo, VulnPrivateContentArm, strings.NewReader(got), &judged) {
		t.Errorf("the real pipeline's own stream reads clean over a described open residual (%q):\n%s", judged.String(), got)
	}
}

// vpFile renders what the content arm's pipeline actually feeds the judge
// for one file: the `diff --git` line, the `+++` header that follows it in
// the FILTERED stream — `index`, `--- a/…` and `@@` are all dropped by the
// grep, so the two are adjacent for every file git emits — and then the
// file's added lines, each carrying the `+` that makes it an added line.
//
// Spelling it once is what keeps these fixtures honest about the one fact
// ranger-base-pdcf6 turned on: a header is known by its POSITION, because
// `+++ …` is also what an added line beginning `++` looks like here. A
// fixture that writes a bare `+++ b/<path>` is writing content.
func vpFile(path string, added ...string) string {
	s := vulnPrivateDiffHeader + "a/" + path + " b/" + path + "\n+++ b/" + path + "\n"
	for _, l := range added {
		s += "+" + l + "\n"
	}
	return s
}

// TestRunGatesVulnPrivateScanClosesEveryUnitAtAFileBoundary is
// ranger-base-4og5l's first arm. The content arm's text was one stream of
// every staged file's added lines with no file boundary left in it, so the
// unit grouping ran ACROSS files: a list item opened in an early file (by
// path order) kept its unit open over every later file's added lines until a
// blank added line turned up. The shape AGENTS.md's own ruling prescribes is
// a "Not closed here" section of bare ids, and such a section is usually the
// last thing in its file — so the last bare id in it was continued by
// whatever the NEXT file added, and the remedy the refusal printed had
// already been performed (ADR 0068's class). MEASURED 2026-11-21 through the
// shipped subcommand before the fix: `- <open id>` then a later file's
// `\t// …` line, rc 1.
//
// The boundary is the diff's own header, which the arm's pipeline used to
// discard with a second grep and now lets through for vulnPrivateUndiff to
// blank. Since ranger-base-pdcf6 that is BOTH header lines and the reading is
// positional — see vpFile, and the collision rows in the middle of this table.
//
// THE TABLE HAS A SECOND HALF SINCE ranger-base-cso5z, which re-reported this
// bug against an installed hook older than this tree and asked for the one
// direction 4og5l's rows do not cover: the id as the FIRST added line of its
// file rather than the last (the grouping charges a bare id to the text above
// it too), and whether the same joining could SUPPRESS a hit rather than
// invent one. The last three rows are that false-negative arm and their
// comment carries the measurement; the answer is no, structurally, because
// blanking a boundary only ever closes a unit.
func TestRunGatesVulnPrivateScanClosesEveryUnitAtAFileBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		text string
		want bool
	}{{
		// The bead's own stdin (i), with the headers restored.
		name: "a bare id last in its file, continued by the next file",
		text: vpFile("docs/x.md", "- ranger-base-l751c") +
			vpFile("internal/posse/x.go", "\tcarveOut.heapOverflowInTheParser = true"),
	}, {
		name: "a whole second file under the id's section",
		text: vpFile("docs/w.md", "## Not closed here", "- ranger-base-l751c") +
			vpFile("docs/x.md", "the heap overflow in the parser"),
	}, {
		// The header's PATH goes with it. Blanked and not merely
		// `+`-stripped, so a path that happens to carry the id and a word
		// is not a unit of its own — which `+++ b/<id>-notes.md` would be.
		name: "the header's own path is discarded with it",
		text: vpFile("docs/notes.d/ranger-base-l751c.md", "- ranger-base-l751c"),
	}, {
		// A file with no `+++` line at all — a pure rename, a mode change,
		// an empty new file — is why the second blanking is conditional:
		// the next thing in the stream is another `diff --git`, and the
		// file after it must still be read.
		name: "a file whose header has no `+++` line does not eat the next file",
		text: "diff --git a/docs/renamed.md b/docs/moved.md\n" +
			vpFile("docs/x.md", "- ranger-base-l751c", "  the heap overflow in the parser"),
		want: true,
	}, {
		// The controls: a boundary must not blank the judge.
		name: "a description in the SAME file still reds",
		text: vpFile("docs/x.md", "- ranger-base-l751c", "  the heap overflow in the parser"),
		want: true,
	}, {
		name: "a description on the id's own line still reds",
		text: vpFile("docs/x.md", "- ranger-base-l751c the heap overflow") +
			vpFile("docs/y.md", "ordinary prose"),
		want: true,
	}, {
		// ranger-base-pdcf6's rows 1 and 3: an ADDED LINE whose own text
		// begins with two plus signs arrives here as `+++…` and used to be
		// blanked, which CLOSED the unit and bought a pass for the shape
		// below. A prefix test cannot tell these from a header; position
		// can, and content `+++ b/<path>` (row 3) is a shape a tracked file
		// already holds — etc/herdr/herdr-bob/darwin-portability.patch.
		name: "a `++` content line between a bare id and its description",
		text: vpFile("docs/x.md", "- ranger-base-l751c", "++ a content line", "\tthe heap overflow"),
		want: true,
	}, {
		name: "a `+++ b/` content line between a bare id and its description",
		text: vpFile("docs/x.md", "- ranger-base-l751c", "+++ b/bin/bob-watch", "\tthe heap overflow"),
		want: true,
	}, {
		// The row-2 control pdcf6 asks for: the plain shape still reds, so
		// the two rows above are not passing because the judge is wired to
		// nothing. "No hit" is also what that says.
		name: "control: the same two lines with no `++` line at all",
		text: vpFile("docs/x.md", "- ranger-base-l751c", "\tthe heap overflow"),
		want: true,
	}, {
		// ranger-base-cso5z: the SAME joining read the other way up. The
		// grouping charges a bare id to the text ABOVE it as well as below
		// (vulnPrivateUnits, "in BOTH directions"), so the boundary has to
		// hold for a file that OPENS with the id too — the shape that bead
		// described, a bare id as the first added line of its file. Nothing
		// pinned this direction: the rows above all put the id in the EARLIER
		// file.
		name: "the upwards escape does not cross a boundary either",
		text: vpFile("docs/w.md", "- the heap overflow in the parser") +
			vpFile("docs/x.md", "  ranger-base-l751c"),
	}, {
		// Its control, paired as the downwards one is: the same two lines in
		// ONE file still reds, so the row above is clean because of the
		// boundary and not because nothing is read.
		name: "the upwards escape in the SAME file still reds",
		text: vpFile("docs/x.md", "- the heap overflow in the parser", "  ranger-base-l751c"),
		want: true,
	}, {
		// ranger-base-cso5z's FALSE-NEGATIVE arm, which that bead raised as
		// arguable and unmeasured: two files' lines joined for the
		// continuation rule are joined for every other paragraph-shaped rule
		// this judge applies, so a hit could in principle be SUPPRESSED
		// across a boundary rather than invented. It is not, and the reason
		// is structural — the boundary only ever CLOSES units, so joining can
		// widen a unit and never narrow one, and a narrower unit is the only
		// thing that can hide a letter. The one narrowing shape is a header
		// landing MID-file, which is ranger-base-pdcf6's collision and the
		// three rows above this block.
		//
		// MEASURED 2026-11-21 (Linux 7.2.15-1-omarchy, this tree at 32bce0e4)
		// through RunGatesVulnPrivateScan over 8 candidate shapes, each fed
		// twice — once as a header-less stream of added lines, which is
		// exactly the pre-fix text, and once with the boundaries: NO shape
		// read clean without the boundary and red with it, which is what a
		// suppressed hit would look like. Three read the other way round
		// (hit without the boundary, clean with it) and those are the false
		// positives ranger-base-4og5l's fix removes — this bead's own repro
		// among them. The three rows below are the arm's own pins.
		name: "a described id in the file AFTER a boundary still reds",
		text: vpFile("docs/w.md", "## Not closed here", "- ranger-base-l751c") +
			vpFile("docs/x.md", "- ranger-base-l751c", "  the heap overflow in the parser"),
		want: true,
	}, {
		// The workaround shape a56098a9 landed with: one blank added line at
		// the end of the previous file, which is what a writer who has not
		// read the hook arrives at. The boundary makes it redundant and must
		// not make it blinding.
		name: "a blank added line before the boundary does not blind the next file",
		text: vpFile("docs/w.md", "- ranger-base-l751c", "") +
			vpFile("docs/x.md", "- ranger-base-l751c", "  the heap overflow in the parser"),
		want: true,
	}, {
		// Every id in a unit is stripped from it before the bareness test
		// (vulnPrivateHits), so a joined unit stripped the id from the
		// PREVIOUS file's lines as well. Blanking the boundary leaves each
		// file's own unit to be judged on its own: w's is bare, x's is not,
		// and the hit is named from x.
		name: "the id stripping does not cross a boundary",
		text: vpFile("docs/w.md", "- ranger-base-l751c") +
			vpFile("docs/x.md", "ranger-base-l751c is a heap overflow"),
		want: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFakeOpenIssues(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]}]`)
			var out strings.Builder
			got := RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateContentArm, strings.NewReader(tc.text), &out)
			if got != tc.want {
				t.Errorf("scan(%q) = %v (%q), want %v", tc.text, got, out.String(), tc.want)
			}
		})
	}
}

// TestRunGatesVulnPrivateScanPaysNoBdQueryForFileBoundariesAlone: keeping the
// `+++` headers makes $posse_added non-empty for a commit that stages only
// DELETIONS — every line of it a header — where the shell's own
// `[ -n "$posse_added" ]` used to skip the arm outright. The judge answers
// that text before it queries bd, so such a commit pays no subprocess at
// all: the same "found nothing costs nothing" property the arm had before
// ranger-base-4og5l, asserted on the bd call log rather than on the verdict,
// because a header-only text answers "no hit" either way.
func TestRunGatesVulnPrivateScanPaysNoBdQueryForFileBoundariesAlone(t *testing.T) {
	// Parallel-safe although it reads the fake bd's call log: fakeBinFor
	// symlinks the fake into fakeDirOf(t), which is keyed on this test, and
	// the fake resolves that log off its own argv[0] — so the log is this
	// test's alone.
	t.Parallel()
	dir := t.TempDir()
	writeFakeOpenIssues(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]}]`)
	bd := qaFakeBd(t)
	log := filepath.Join(fakeDirOf(t), "bd-calls.log")
	var out strings.Builder
	if RunGatesVulnPrivateScan(bd, dir, VulnPrivateContentArm, strings.NewReader(vpFile("docs/x.md")+vpFile("internal/posse/y.go")), &out) {
		t.Errorf("a text of nothing but file boundaries = %q, want no hit", out.String())
	}
	if b, err := os.ReadFile(log); err == nil && len(b) > 0 {
		t.Errorf("a text of nothing but file boundaries queried bd:\n%s", b)
	}
	// The control, so the early answer cannot pass by never querying at all.
	if !RunGatesVulnPrivateScan(bd, dir, VulnPrivateContentArm, strings.NewReader(vpFile("docs/x.md", "- ranger-base-l751c the heap overflow")), &out) {
		t.Fatalf("the control must still be judged: %q", out.String())
	}
	b, err := os.ReadFile(log)
	if err != nil || !strings.Contains(string(b), "list") {
		t.Errorf("an added line past the boundary must still reach bd's open-issue query: %q %v", b, err)
	}
}

// TestVulnPrivateHitsReadsAGoCommentsOwnMarkdown is ranger-base-4og5l's
// second arm. vulnPrivateListItem reads a bullet after leading whitespace,
// and a comment-marker line starts with a slash — so no line of a Go comment
// opened a block: it could not POP an earlier list item, and the tidy
// spelling `//   - <id>` was a CONTINUATION rather than a list item of its
// own. Since ranger-base-cq3vi widened this arm from markdown to every
// staged path, that left a description under a bare id in a `.go` comment
// passing SILENT — the one escape the unit grouping exists to close
// (ranger-base-68631 finding 1) — and made the only reliable way to name an
// open residual's id in Go source not naming it there.
//
// THE EDGE OF THE BLOCK IS A BOUNDARY, which is the half that cost
// (MEASURED 2026-11-21, census over the tracked tree): without it the
// compliant `// Not closed here:\n// - <id>` ends a comment on a list item
// and the `func` line below it reads as that bullet's lazy continuation, so
// the reading that fixes a false positive buys one — at 10 sites, this
// file's neighbour bwraphide_qa_test.go among them. A `//` block is a
// document of its own and the code around it is no part of it.
func TestVulnPrivateHitsReadsAGoCommentsOwnMarkdown(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c"}
	for _, tc := range []struct {
		name string
		text string
		want bool
	}{{
		name: "a description continuing a bulleted id in a comment",
		text: "//   - ranger-base-l751c\n//     the heap overflow in the parser\n",
		want: true,
	}, {
		name: "a bulleted id in a comment, written above a description",
		text: "// the heap overflow in the parser:\n//\n//   - ranger-base-l751c\n//     in the parser\n",
		want: true,
	}, {
		name: "the compliant section, at the end of a comment block",
		text: "// Not closed here:\n// - ranger-base-l751c\nfunc TestLiveBwrapHideIsNotWalkedAround(t *testing.T) {\n",
	}, {
		name: "the compliant section, numbered",
		text: "//\n// Not closed here:\n//  1. ranger-base-l751c\n//  2. ranger-base-fixt3\nvar heapOverflowInTheParser = true\n",
	}, {
		// The shape this tree already writes, inside a struct body: the
		// labelled residual (vulnPrivateLabels) on its own comment line,
		// with field names above and below it and no blank line anywhere —
		// red before this reading, because the struct's own opening line
		// took every tab-indented line under it (MEASURED 2026-11-21: three
		// LIVE open vuln-private ids were in exactly this shape, their
		// sites on the bead rather than here — a still-open residual is
		// named by its id and nothing else).
		name: "a labelled residual inside a struct body",
		text: "type carveOut struct {\n\tDeny []string\n\t// an empty tmpfs over it would hide all of that to cover one file.\n\t// Residual: ranger-base-l751c.\n\tReadMaskDir []string\n}\n",
	}, {
		name: "a bare id on its own comment line",
		text: "// wire the reaper\n// ranger-base-l751c\n",
	}, {
		// THE GUTTER SPACE, and the one shape whose verdict turns on it:
		// vulnPrivateUncomment strips the marker and ONE space — the
		// reading go/doc/comment itself gives a doc comment, where `// `
		// is the gutter and `//\t` opens a code block. So a tab-indented
		// comment line sits DEEPER than the prose above it and is charged
		// to it, which is ranger-base-68631 finding 1's escape read
		// upwards ("the text above an indented bare id is the same escape
		// upside down") finally reaching Go comments. Strip the space and
		// nothing else, and prose and tab land in the same column, each
		// its own unit, and the escape is open again.
		name: "a bare id tab-indented under a comment's prose",
		text: "// the heap overflow in the parser\n//\tranger-base-l751c\n",
		want: true,
	}, {
		// The control for the whole reading: a marker counts at the head of
		// the line and nowhere else, so a quoted one is not a comment.
		name: "a marker inside a string is not a comment",
		text: "\tfmt.Fprintln(out, \"//   - ranger-base-l751c the heap overflow\")\n",
		want: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := vulnPrivateHits(tc.text, append(ids, "ranger-base-fixt3"))
			if (got != nil) != tc.want {
				t.Errorf("vulnPrivateHits(%q) = %v, want hit=%v", tc.text, got, tc.want)
			}
		})
	}
}
