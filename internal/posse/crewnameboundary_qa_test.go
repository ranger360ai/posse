package posse

// QA probe for the residual ranger-base-o3g6a left behind (found verifying
// that close under ranger-base-han3i).
//
// That close fixed the NAME axis: both ADR 0012 App.A 5 pins now match the
// path a file ships under, and they match it with qibCrewPathPattern, which
// drops `\b` on purpose — because `\b` never fires beside `_`, and `_` is
// Go's own file-name separator.
//
// The same boundary argument applies to the LINE pattern the pins use on file
// CONTENTS, and there it was not made. `\b` fires only between a word
// character and a non-word one, so a crew name used as part of a Go
// IDENTIFIER — `<seat>Probe`, `<seat>_probe`, `fake<Seat>` — is invisible to
// qibCrewPattern, while the very same string in a path is caught. A name
// entering the shipped tree as prose is held; a name entering it as code is
// not, and code is the likelier way for one to arrive in cmd/ or internal/.
//
// CENSUSED at the time of writing (2026-09-04, HEAD 9920e75): a
// case-insensitive SUBSTRING sweep of qibCrewNames over the contents of the
// four shipped roots returns ZERO hits, so nothing is escaping today. This is
// a pin gap, not a live defect — which is exactly why it ships as a green
// test that asserts the gap and names the inversion, rather than as prose in
// a bead nobody re-reads.
//
// WHEN THIS GOES RED the line pattern has learned the boundary, and that is
// the fix: delete this file. Do not "fix" it by loosening the assertion.

import (
	"strings"
	"testing"
)

func TestQACrewNamePinIsBlindToAnIdentifier(t *testing.T) {
	t.Parallel()
	re := qibCrewReaders()
	// Built from the pin's own list, so a name added there is measured here
	// too and this file still contains none of the banned spellings.
	names := qibCrewNames()
	if len(names) < 5 {
		t.Fatalf("qibCrewNames returned %d names — this probe measured almost nothing", len(names))
	}

	// A control first: the shape the line pattern DOES hold. Without it, a
	// pattern that matched nothing at all would satisfy every case below.
	held := 0
	for _, n := range names {
		if re.line.MatchString("// measured by " + n + " on a Tuesday") {
			held++
		}
	}
	if held != len(names) {
		t.Fatalf("the line pattern held only %d of %d names in prose — the control failed, so the blind-class result below means nothing",
			held, len(names))
	}

	// The blind class: the same name inside an identifier.
	for _, shape := range []struct{ label, tmpl string }{
		{"a lower-snake identifier", "const x = \"%s_probe\""},
		{"an exported camel identifier", "func %sProbe() {}"},
		{"a suffix camel identifier", "var fake%s = 1"},
		{"a path quoted in a comment", "// see internal/posse/%s_as19_probe_test.go"},
	} {
		for _, n := range names {
			line := strings.Replace(shape.tmpl, "%s", n, 1)
			if strings.Contains(shape.tmpl, "fake%s") {
				line = strings.Replace(shape.tmpl, "%s", strings.ToUpper(n[:1])+n[1:], 1)
			}
			if re.line.MatchString(line) {
				t.Errorf("FIXED for %s (%s): the line pattern now sees %q. "+
					"The gap this file pins is closed — delete internal/posse/crewnameboundary_qa_test.go.",
					shape.label, n, line)
				continue
			}
			// The path pattern sees it, which is what makes this a gap in the
			// LINE arm specifically and not a limit of the name list.
			if !re.path.MatchString(line) {
				t.Errorf("neither pattern sees %q — the fixture stopped naming a crew seat, so this row measures nothing", line)
			}
		}
	}
}
