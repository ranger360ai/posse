package posse_test

// QA pin for ranger-base-efk14 — an ADR that names a pin file by name is
// making a checkable claim, and until now nothing checked it.
//
// WHAT WENT WRONG. ADR 0015 §3 says the bd-hook narrowing "is pinned twice,
// and neither pin is optional", and names bdhookcommit_qa_test.go as the
// behaviour half. The 2026-09 adherence audit (docs/notes.d/adr-adherence-
// 2026-09.md, finding 7) reported that no commit in history ever carried that
// file and marked the rule DRIFTED on the record's claim. The file exists: it
// landed 2026-08-29 in d085a96 at the repo ROOT and is on main. The audit
// looked for it under the package directories and posse keeps ~30 *_qa_test.go
// files at the root, so the search missed it and a correct record was recorded
// as a false one — which very nearly cost an ADR edit reversing a true
// sentence.
//
// THE GAP, which is the general one: the ADRs cite 36 distinct test files by
// name, and no test resolves a single citation. A pin can be renamed, moved or
// deleted and every record still names it; an auditor then has to resolve the
// name by hand, and a hand resolution can miss.
//
// WHAT THIS PINS. Every *_test.go the ADRs name must exist in the tree, by
// base name. Base name and not full path deliberately: 12 citations across 7
// records still spell the retired `internal/rhq/` directory (ranger-base-1d8bk,
// the architect lane — this lane verifies against the records and does not
// edit them), so a path-exact assertion is red on arrival for a reason that is
// a record defect and not a missing pin. When 1d8bk lands, tighten
// adrCiteResolves to compare the full path.
//
// The named regression is asserted directly as well: 0015 §3's own citation
// must resolve, and the file it resolves to must still carry the three tests
// the ADR describes it as running.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type adrCite struct {
	adr  string
	line int
	text string
}

// A citation is any token shaped like a Go test file. The ADRs write them
// bare, in backticks, and with a directory prefix; all three spellings are one
// claim about a file that must exist.
var adrCiteRe = regexp.MustCompile(`[A-Za-z0-9_./-]*_test\.go`)

func adrCitations(t *testing.T) []adrCite {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join("docs", "adr"))
	if err != nil {
		t.Fatalf("read docs/adr: %v", err)
	}
	var out []adrCite
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join("docs", "adr", e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for i, ln := range strings.Split(string(b), "\n") {
			for _, m := range adrCiteRe.FindAllString(ln, -1) {
				out = append(out, adrCite{adr: e.Name(), line: i + 1, text: m})
			}
		}
	}
	return out
}

// adrTestFileIndex maps every test file in the tree by base name to the paths
// carrying it.
func adrTestFileIndex(t *testing.T) map[string][]string {
	t.Helper()
	idx := map[string][]string{}
	err := filepath.WalkDir(".", func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			idx[d.Name()] = append(idx[d.Name()], p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return idx
}

// adrCiteResolves is the whole rule, isolated so the arm below can be shown
// able to fail over a synthetic corpus.
func adrCiteResolves(idx map[string][]string, c adrCite) bool {
	return len(idx[filepath.Base(c.text)]) > 0
}

func adrUnresolved(idx map[string][]string, cites []adrCite) []adrCite {
	var out []adrCite
	for _, c := range cites {
		if !adrCiteResolves(idx, c) {
			out = append(out, c)
		}
	}
	return out
}

// Every test file the ADRs name must be in the tree.
func TestADRCitedPinFilesExist(t *testing.T) {
	cites := adrCitations(t)
	idx := adrTestFileIndex(t)

	// Floors. A corpus that has silently emptied — the records moved, the
	// walk rooted somewhere else — would otherwise pass with nothing
	// measured. These are floors on the count, not a coupling to it.
	if len(idx) < 40 {
		t.Fatalf("the tree index holds %d test files; the walk is measuring the wrong root", len(idx))
	}
	adrs := map[string]bool{}
	for _, c := range cites {
		adrs[c.adr] = true
	}
	if len(cites) < 25 || len(adrs) < 8 {
		t.Fatalf("found %d citations across %d records; the extractor stopped matching", len(cites), len(adrs))
	}

	for _, c := range adrUnresolved(idx, cites) {
		t.Errorf("%s:%d names %s as a pin and no such file is in the tree", c.adr, c.line, c.text)
	}
}

// The rig, shown able to fail. Without this arm a resolver that answered true
// unconditionally would pass the arm above forever.
func TestADRCitationCheckCanFail(t *testing.T) {
	idx := adrTestFileIndex(t)
	real := adrCite{adr: "0015-constitution-promotion.md", line: 341, text: "bdhookcommit_qa_test.go"}
	fake := adrCite{adr: "0015-constitution-promotion.md", line: 341, text: "internal/posse/bdhookcommit_qa_test.go.nosuch_test.go"}

	if got := adrUnresolved(idx, []adrCite{fake}); len(got) != 1 {
		t.Fatalf("a citation naming a file that cannot exist was resolved; the check cannot fail and pins nothing")
	}
	if got := adrUnresolved(idx, []adrCite{real}); len(got) != 0 {
		t.Fatalf("the real citation was reported missing; the check refuses everything and separates nothing")
	}
}

// The named regression. 0015 §3 does not only name a file — it says what that
// file does. Both halves are claims, so both are pinned.
func TestADR0015NamesTheHookCommitPinAndItDoesWhatItSays(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("docs", "adr", "0015-constitution-promotion.md"))
	if err != nil {
		t.Fatalf("read ADR 0015: %v", err)
	}
	const named = "bdhookcommit_qa_test.go"
	if !strings.Contains(string(b), named) {
		t.Fatalf("ADR 0015 no longer names %s; the record and this pin have drifted apart", named)
	}

	idx := adrTestFileIndex(t)
	paths := idx[named]
	if len(paths) != 1 {
		t.Fatalf("want exactly one %s in the tree, found %v", named, paths)
	}
	pin, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read %s: %v", paths[0], err)
	}
	// The three behaviours §3 describes: the shipped PIDs may commit, the
	// broad spellings may not, and the hazard the narrowing kept is still
	// refused.
	for _, fn := range []string{
		"func TestShippedPIDsLetBeadsOwnHooksRun(",
		"func TestBroadHookDenyWallsTheCommitAndTheCheckout(",
		"func TestNarrowedHookDenyStillRefusesInstall(",
	} {
		if !strings.Contains(string(pin), fn) {
			t.Errorf("%s no longer carries %s; ADR 0015 §3 describes a pin that is no longer there", paths[0], fn)
		}
	}
}
