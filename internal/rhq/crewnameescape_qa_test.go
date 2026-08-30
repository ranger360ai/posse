package rhq

// ranger-base-ufdy (verifying the close of ranger-base-h6fx): the test
// corpus's crew-name invariant, for the one spelling neither existing pin
// can see.
//
// h6fx's sweep found two fixtures that hid a crew name behind a `\n` escape
// — the shape is a literal whose escape runs straight into the name, as in
// "base" + backslash-n + a crew name + " HALF-WRITTEN". They were renamed by
// hand, and the close said plainly that the sweep AND the pin walk straight
// past that spelling: in the source text the escape's `n` and the name's
// first letter are both word characters, so qibCrewPattern's `\b` never
// fires between them. Nothing was left holding it.
//
// The two pins that exist each cover one half and neither covers this:
//
//   - TestShippedTreeNamesRolesNotThisCrew reads every raw line under cmd/,
//     internal/, etc/ and examples/ — test files included — but reads them
//     RAW, so `\n` is two characters and the name is not at a word boundary.
//
//   - TestShippedStringsNameRolesNotThisCrew unquotes before matching, which
//     is exactly the right instrument, but skips every *_test.go on purpose:
//     its scope is SHIPPED strings (ranger-base-q3gp).
//
// So the corpus is measured raw and the escapes are measured only outside
// it. This pin is the intersection: unquoted literals, in the test corpus.
// Both fixtures h6fx renamed live in internal/rhq/*_test.go, which is to say
// the class regressed once already in the only place nothing watches.
//
// The repo ROOT's *_test.go are a different gap in the same invariant and
// are ranger-base-4say's, not swept here.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The corpus roots, matching TestShippedTreeNamesRolesNotThisCrew's.
var qceRoots = []string{"cmd", "internal", "etc", "examples"}

// qceHidden reports the crew names a literal spells only after unquoting —
// a hit its own source line does not carry, and therefore one the raw walk
// cannot report. A raw string literal is byte-for-byte its source and can
// hide nothing, so only escaped literals are asked.
func qceHidden(t *testing.T, path string, lines []string) []string {
	t.Helper()
	re := qibCrewPattern()
	var out []string
	for _, lit := range qibShippedStrings(t, path) {
		if lit.Raw {
			continue
		}
		loc := re.FindIndex([]byte(lit.Text))
		if loc == nil {
			continue
		}
		// The source line the literal starts on. If the name is visible
		// THERE, the raw walk already reports it and this is not a hit.
		if lit.Line-1 < len(lines) && re.MatchString(lines[lit.Line-1]) {
			continue
		}
		out = append(out, path+":"+strconv.Itoa(lit.Line)+": "+lit.Text[loc[0]:loc[1]])
	}
	return out
}

func TestTestCorpusHidesNoCrewNameBehindAnEscape(t *testing.T) {
	root := qibRepoRoot(t)
	var hits []string
	scanned := 0
	for _, rel := range qceRoots {
		err := filepath.WalkDir(filepath.Join(root, rel), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			// The pin file that carries the spellings, split, is its own
			// exception in the walk above and is one here for the same
			// reason. This file names no crew member at all.
			if d.Name() == "instancebound_qa_test.go" {
				return nil
			}
			if !strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			relPath, _ := filepath.Rel(root, path)
			for _, h := range qceHidden(t, path, strings.Split(string(body), "\n")) {
				hits = append(hits, strings.Replace(h, path, relPath, 1))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// A pin over pure absence is satisfied by measuring nothing: say how
	// many files were parsed, and fail a walk that found none.
	if scanned < 100 {
		t.Fatalf("only %d test files parsed across %v — the walk found nothing to pin", scanned, qceRoots)
	}
	t.Logf("parsed %d test files across %v", scanned, qceRoots)
	if len(hits) > 0 {
		t.Errorf("ADR 0012 App.A 5: a test fixture names the originating instance behind an escape (%d hits):\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}

// The sweep above reports an absence over a corpus that is already clean, so
// on its own it is a green light with nothing behind it. This is the arm
// that fails: h6fx's own fixture shape, planted in a scratch file, must be
// found here AND must be invisible to the raw-line match — the second half
// is the whole reason this pin exists rather than being a duplicate of
// TestShippedTreeNamesRolesNotThisCrew.
func TestCrewNameEscapePinSeesWhatTheRawWalkCannot(t *testing.T) {
	// Assembled, never spelled: this file is inside the walk above and is
	// not on its exception list. Same trick qibCrewPattern uses.
	name := strings.ToUpper("mon" + "ica")
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture_test.go")
	src := "package p\n\nvar x = \"base\\n" + name + " HALF-WRITTEN\"\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(src, "\n")

	// The raw walk's instrument, on the same bytes: it must find nothing.
	re := qibCrewPattern()
	for i, line := range lines {
		if re.MatchString(line) {
			t.Fatalf("the raw-line match found the name at line %d (%q) — the gap this pin is filling has closed, and the pin is now a duplicate rather than a cover", i+1, line)
		}
	}

	hits := qceHidden(t, path, lines)
	if len(hits) != 1 {
		t.Fatalf("the escape-hidden name was not found: %v\nsource:\n%s", hits, src)
	}

	// And the other direction: a name the source line DOES carry belongs to
	// the raw walk, and must not be double-reported here.
	plain := filepath.Join(dir, "plain_test.go")
	psrc := "package p\n\nvar y = \"" + strings.ToLower(name) + " wrote this\"\n"
	if err := os.WriteFile(plain, []byte(psrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if h := qceHidden(t, plain, strings.Split(psrc, "\n")); len(h) != 0 {
		t.Errorf("a name visible on its own source line is the raw walk's to report, got %v", h)
	}
}
