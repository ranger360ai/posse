package posse

// QA pins for ranger-base-jwcxu (ADR 0018 site 3, the escape a verify pass
// caught on ranger-base-99ps) and the bead folded into it, ranger-base-ch6re.
//
// WHAT ESCAPED. ranger-base-99ps scrubbed six sites of instance-ops content
// out of the public tree. Site 3 is ADR 0018 §1's rendered brake line — the
// example of what a degraded pass prints — and it rendered this instance's
// own measured pass and day spend on the left of each slash. The operator's
// bless on ranger-base-axft covered the CEILINGS (the right of each slash),
// and the close applied that ceilings-scoped ruling to the whole line, so
// the spend halves stayed. Nothing in the suite could see it: the same line
// in NOTES.md has always rendered placeholders, and the two copies of one
// render had no reader that compared them.
//
// So this file is that reader. It does not re-litigate the bless and does
// not scan the tree for cost content in general — the ops-pattern list and
// the D2 commit gate already do that, and the gate is what forced the shape
// this scrub landed in (MEASURED 2026-09-02: staging the line with only the
// spend halves replaced, the blessed ceilings kept on the right of each
// slash, is REFUSED by check 2 — the scan reads ADDED lines and cannot tell
// a line being scrubbed from a line being written, so an edit that removes
// exposure is refused for the figures it did not touch. The way through
// that does not need the operator's override is NOTES.md's own shape, both
// halves; ranger-base-jwcxu's EXPECTED asked for the ceilings kept, and the
// bless is a permission to publish them, not a requirement to).
//
// NOT COVERED HERE, and deliberately. The wider finding on ranger-base-jwcxu
// — that ranger-base-99ps's stated bar ("zero ops-pattern hits over all
// tracked markdown") was never measured, and that the tree carries 68 hits
// across all four classes at this commit — is a disposition question about
// which of those are the SOFTWARE's public vocabulary and which are one
// deployment's facts. That is a visibility ruling, not a developer's call;
// it is filed on its own bead for the security lane, ranger-base-imiif. A
// green run below is not clearance for it.

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// trackedMarkdown is the shipped set — what git tracks, not what the working
// directory holds, so a scratch file a session left behind cannot red this.
func trackedMarkdown(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("git", "ls-files", "-z", "--", "*.md", "*.markdown").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v — this pin needs the checkout to name what posse ships", err)
	}
	files := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	var keep []string
	for _, f := range files {
		if f != "" {
			keep = append(keep, f)
		}
	}
	if len(keep) < 50 {
		t.Fatalf("git tracks %d markdown files — the listing failed, so a clean result here is not evidence", len(keep))
	}
	return keep
}

// The render, as dispatch.go prints it. Docs wrap, so every assertion below
// runs over whitespace-collapsed text: the ADR's copy of this line breaks
// between "brake" and "(pass", and a per-line scan sees neither half whole.
const brakeRenderMarker = "degraded, running under ledger brake"

var (
	spaceRun    = regexp.MustCompile(`[[:space:]]+`)
	brakeRender = regexp.MustCompile(regexp.QuoteMeta(brakeRenderMarker) + ` \(([^)]*)\)`)
	liveMoney   = regexp.MustCompile(`\$[0-9]`)
)

func flat(s string) string { return spaceRun.ReplaceAllString(s, " ") }

// ranger-base-jwcxu. Every copy of the degraded-pass render in the shipped
// tree shows the operator what the line LOOKS like; none of them may show
// what this instance SPENT. The rule is the whole parenthetical, not just
// the halves that escaped: a figure inside it is either spend (never public)
// or a cap (the operator's to publish, and not from a rendered example — the
// ADR's Consequences section is where the caps are stated and dated).
func TestTheRenderedBrakeLineCarriesNoLiveFigures(t *testing.T) {
	found := 0
	for _, f := range trackedMarkdown(t) {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, m := range brakeRender.FindAllStringSubmatch(flat(string(body)), -1) {
			found++
			if liveMoney.MatchString(m[1]) {
				t.Errorf("%s renders the degraded brake line with a live figure in it: (%s)\n"+
					"the spend halves are this instance's, the caps belong to ADR 0018's Consequences — "+
					"render both as the placeholders NOTES.md uses (ranger-base-jwcxu, ranger-base-99ps site 3)",
					f, m[1])
			}
		}
	}
	// Two copies exist and are the point of the pin — NOTES.md's, which was
	// always right, and ADR 0018 §1's, which was not. A matcher that found
	// neither would report a clean tree while measuring nothing.
	if found < 2 {
		t.Fatalf("found %d copies of the degraded brake render in tracked markdown, want at least 2 "+
			"(NOTES.md and docs/adr/0018) — the scan above measured nothing", found)
	}

	// The control. Plant the line exactly as it stood before the scrub and
	// require the same matcher, over the same wrapping, to see it.
	planted := "  `plan guard: blind 4h (…) — " + brakeRenderMarker + "\n  (pass $8.20/$30, day $146/$250)`.\n"
	m := brakeRender.FindStringSubmatch(flat(planted))
	if m == nil {
		t.Fatal("control: the matcher does not see the pre-scrub line at all — every clean verdict above is empty")
	}
	if !liveMoney.MatchString(m[1]) {
		t.Errorf("control: the pre-scrub parenthetical %q read as clean — the rule below it measures nothing", m[1])
	}
}

// ranger-base-jwcxu, second reader. A docs example that no code path prints
// is not a scrub problem but it is the reason one goes unnoticed: nobody
// diffs prose against a format string. dispatch.go owns the render; the ADR
// and NOTES.md quote it. If the printf moves, this reds and someone looks.
func TestTheRenderedBrakeLineMatchesTheCodeThatPrintsIt(t *testing.T) {
	src, err := os.ReadFile("internal/posse/dispatch.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(flat(string(src)), brakeRenderMarker+" (%s)") {
		t.Errorf("dispatch.go no longer prints %q followed by a parenthetical — "+
			"the docs quote a render nothing produces, and the scrub pin above is aimed at prose only",
			brakeRenderMarker)
	}
}

// ranger-base-ch6re (from ranger-base-vi67), folded into ranger-base-jwcxu.
// The bless on the ceilings is a VISIBILITY ruling and the vi67 ruling is an
// ACCURACY one; both hold at once, so Consequences keeps the pair it was
// written with AND names the pair in force. This is a regression pin against
// a silent deletion, not a measurement: the ADR does not read the live
// config, which is exactly why its claim is dated and cited.
func TestADR0018ConsequencesNamesTheReaffirmedBlindDayBound(t *testing.T) {
	const path = "docs/adr/0018-blind-meter-armed-ledger.md"
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sec := section(t, string(body), "## Consequences")

	// Append-only: the sentence the amendment qualifies is still there. An
	// amendment that overwrote it would satisfy every other check here.
	const original = "degraded-loud day bounded at"
	if !strings.Contains(flat(sec), original) {
		t.Errorf("%s: the 2026-08-26 bound is gone from Consequences — this ADR amends, it does not rewrite", path)
	}
	orig := strings.Index(flat(sec), original)

	for _, want := range []string{
		"ranger-base-vi67",
		"`budget_pass` 150 / `budget_day` 3000",
	} {
		at := strings.Index(flat(sec), want)
		if at < 0 {
			t.Errorf("%s: Consequences does not carry %q — the blind-day bound the operator re-affirmed is unstated (ranger-base-ch6re)", path, want)
			continue
		}
		if at < orig {
			t.Errorf("%s: %q appears before the sentence it amends — amendments are appended, in date order", path, want)
		}
	}
}

// section returns the body of a `## `-headed section, so an assertion about
// Consequences cannot be satisfied by a string somewhere else in the file.
func section(t *testing.T, doc, head string) string {
	t.Helper()
	i := strings.Index(doc, head+"\n")
	if i < 0 {
		t.Fatalf("no %q section — the document was restructured and this pin is aimed at nothing", head)
	}
	rest := doc[i+len(head)+1:]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		return rest[:j]
	}
	return rest
}
