package posse_test

// QA pin for ranger-base-hdcgb: internal/rhq leftovers sweep, code half.
//
// WHAT THE SWEEP DID. The Go package was renamed `internal/rhq` ->
// `internal/posse` in 9c00e192 (2026-08-31). Comments and docs outside
// docs/adr went on naming the retired directory: 17 live-pin guard recipes
// (`go test ./internal/rhq -run ...`) across internal/, two evergreen prose
// explanations (govern_test.go, splashwide_qa_test.go), a runbook recipe
// (docs/runbooks/release.md) and two NOTES.md passages. docs/adr is a
// separate corpus, already swept and pinned by
// TestADRRecordsNameTheLivePackageDirectory (adrpackagedirsweep_qa_test.go);
// this pin does not overlap it and does not walk docs/adr.
//
// docs/notes.d/ IS OUT, DELIBERATELY, THE SAME WAY IT IS OUT of the
// extdiff_qa_test.go population: those fragments are frozen per-bead records
// (ADR 0022, one writer per file), and a record quoting a command or a
// package path as it was AT THE TIME is accurate, not stale. Rewriting one
// would misrepresent what the fragment's own author measured. The bead body
// listed "notes.d" as an example sweep target, but that predates
// ranger-base-l1ix2 landing this exact policy in code; re-reading the pages
// this bead cites (per the 2026-09-06 pause-lift) is what surfaced the
// conflict, and the coded policy wins.
//
// THE REMAINING ~27 HITS INSIDE internal/ AND NOTES.md are frozen for the
// same reason, one citation at a time, not by directory: each is either (a)
// a `MEASURED <date>` or `SIGHTED` mutation-testing narration reporting what
// a specific historical run against the tree found, dated before or at the
// 2026-08-31 rename (so the package really was named internal/rhq when the
// narration was written), or (b) a comment naming the rename itself
// (exampledigests.go, bdflushdiscipline_qa_test.go), or (c) the
// extdiff_qa_test.go passage above explaining the notes.d policy, which
// necessarily quotes the frozen paths it is describing. rhqLeftoverExemptions
// is the explicit, one-line-per-citation list — no globs, no directory
// carve-outs beyond docs/adr and docs/notes.d, matching the
// scripts/silent-reverts.allow convention: a citation triaged and accepted,
// not a pattern that would silence a future one that happens to rhyme.
//
// WHAT THIS PIN DOES NOT COVER: scripts/, Makefile, CHANGELOG.md and
// README.md all still name internal/rhq (measured 2026-09-06) but none of
// the four are in the bead's declared census (`grep -rl internal/rhq over
// internal/ cmd/ docs/`, plus INSTALL.md and NOTES.md named explicitly) —
// left alone rather than swept unasked.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	rhqLeftoverRetired = "internal/rhq"
	rhqLeftoverLive    = "internal/posse"
)

// rhqLeftoverExempt is one citation this sweep leaves alone, and why.
type rhqLeftoverExempt struct {
	file   string // repo-relative path
	substr string // unique text on the exempted line
	reason string
}

var rhqLeftoverExemptions = []rhqLeftoverExempt{
	{"internal/posse/exampledigests.go", "// 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)",
		"names the rename itself, both sides of the arrow; nine identical digest comments"},
	{"internal/posse/bdflushdiscipline_qa_test.go", "reported this pin as landed at internal/rhq/bdflushdiscipline_qa_test.go.",
		"quotes yeg1's 2026-08-31 close comment, which named the file's actual path in that bead's worktree"},
	{"internal/posse/bdflushdiscipline_qa_test.go", "bead's worktree, on a base older than the internal/rhq -> internal/posse",
		"names the rename itself as the reason the file was orphaned"},
	{"internal/posse/codexwritable_test.go", "internal/rhq package green. Both of the bead's own pins judge the redirect",
		"MEASURED 2026-08-29, before the rename — the package really was named internal/rhq that day"},
	{"internal/posse/credentialdegrade_qa_test.go", "leaves `go test ./internal/rhq -run 'TestBlind|TestQABlind|Degrade|",
		"the command as actually run and captured 2026-08-30, before the rename"},
	{"internal/posse/dispatch_qa_test.go", "claimLost arm increment sessFail and the ENTIRE internal/rhq package",
		"MEASURED 2026-08-30, before the rename"},
	{"internal/posse/extdiff_qa_test.go", "fragments are frozen per-bead records (they still name `internal/rhq/`",
		"explains the docs/notes.d/ frozen-record policy this pin also follows; quoting the paths it describes"},
	{"internal/posse/gatedkeychain_test.go", "internal/rhq package stays green.",
		"blames 2026-08-28, before the rename"},
	{"internal/posse/gateschain_qa_test.go", "of ./internal/rhq — stays green.",
		"measured 2026-08-28, before the rename"},
	{"internal/posse/jeu2halves_qa_test.go", "./internal/rhq green (measured 2026-08-29, verifying the close).",
		"the text names its own measurement date, before the rename"},
	{"internal/posse/landsweep_qa_test.go", "count as an empty one — left the whole internal/rhq package green, and it",
		"blames 2026-08-28, before the rename"},
	{"internal/posse/reachability_qa_test.go", "whole internal/rhq package green, 562s.",
		"blames 2026-08-29, before the rename"},
	{"internal/posse/seatheldbyrun_qa_test.go", "the whole internal/rhq package green (-count=1, 457s). With that mutation a",
		"blames 2026-08-28, before the rename"},
	{"internal/posse/seatheldbyrun_qa_test.go", "1/3 full internal/rhq runs, green alone and green on a second full",
		"same historical narrative block as the line above (ranger-base-qhf8 sighting)"},
	{"internal/posse/shelfguard_qa_test.go", "internal/rhq/initseed_qa_test.go:48 and",
		"cites file paths as they existed at commit 2d8ccc9, ahead of the rename"},
	{"internal/posse/shelfguard_qa_test.go", "internal/rhq/seedcrewrgx0_qa_test.go:79 — were already t.Fatalf, not",
		"same citation, second file"},
	{"internal/posse/statedirlaunch_qa_test.go", "whole internal/rhq package green — the ranger-base-unzn shape, an arm nothing",
		"blames 2026-08-29, before the rename"},
	{"internal/posse/watchlock_test.go", "bug — left ., ./cmd/posse and ./internal/rhq all green.",
		"blames 2026-08-28, before the rename"},
	{"NOTES.md", "On 2026-08-29 `make test` came back exit 2 with ~80 reds in `internal/rhq`,",
		"narrates a specific 2026-08-29 incident, before the rename"},
}

// rhqLeftoverPopFloor is the number of files this sweep's population reads,
// measured 2026-09-06: 574 under internal/, 53 under cmd/, 7 docs/*.md
// outside docs/adr and docs/notes.d, plus NOTES.md and INSTALL.md. A reader
// that walks fewer than this has lost a directory and is reporting a green
// over files it never read.
const rhqLeftoverPopFloor = 600

// rhqLeftoverCorpus walks internal/, cmd/ and docs/ (skipping docs/adr and
// docs/notes.d, which have their own exemption policy), plus NOTES.md and
// INSTALL.md, and returns every .go/.md file's body keyed by repo-relative
// path.
func rhqLeftoverCorpus(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, root := range []string{"internal", "cmd", "docs"} {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path == filepath.Join("docs", "adr") || path == filepath.Join("docs", "notes.d") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".md") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[path] = string(b)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	for _, f := range []string{"NOTES.md", "INSTALL.md"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		out[f] = string(b)
	}
	if len(out) < rhqLeftoverPopFloor {
		t.Fatalf("read %d files, floor is %d — a corpus this small has lost a directory, and every verdict below it would be a green over unread files", len(out), rhqLeftoverPopFloor)
	}
	return out
}

// rhqLeftoverExempt reports whether the given line in the given file is
// covered by an explicit exemption.
func rhqLeftoverIsExempt(file, line string) bool {
	for _, e := range rhqLeftoverExemptions {
		if e.file == file && strings.Contains(line, e.substr) {
			return true
		}
	}
	return false
}

// rhqLeftoverStaleHits returns "<file>:<line>" for every line naming the
// retired directory that no exemption covers.
func rhqLeftoverStaleHits(bodies map[string]string) []string {
	var out []string
	for name, body := range bodies {
		for i, ln := range strings.Split(body, "\n") {
			if !strings.Contains(ln, rhqLeftoverRetired) {
				continue
			}
			if rhqLeftoverIsExempt(name, ln) {
				continue
			}
			out = append(out, name+":"+strconv.Itoa(i+1))
		}
	}
	return out
}

// TestNoUnswappedInternalRhqCommentsOutsideFrozenRecords is the sweep's
// done-when, held: every remaining `internal/rhq` citation in this pin's
// population is on the explicit exemption list.
func TestNoUnswappedInternalRhqCommentsOutsideFrozenRecords(t *testing.T) {
	bodies := rhqLeftoverCorpus(t)
	for _, hit := range rhqLeftoverStaleHits(bodies) {
		t.Errorf("%s names the retired package directory %q; it was renamed to %q in 9c00e192 (ranger-base-hdcgb sweep). If this citation is a frozen historical record (a dated MEASURED/SIGHTED narration, or a rename citation), add it to rhqLeftoverExemptions with a reason — otherwise fix the comment", hit, rhqLeftoverRetired, rhqLeftoverLive)
	}
}

// TestRhqLeftoverExemptionsStillNameRealLines guards the exemption list from
// the other direction: an exemption whose substring no longer appears in its
// file is a dead entry hiding nothing, and a moved or reworded line under it
// would pass this pin unexamined instead of being re-triaged.
func TestRhqLeftoverExemptionsStillNameRealLines(t *testing.T) {
	bodies := rhqLeftoverCorpus(t)
	for _, e := range rhqLeftoverExemptions {
		body, ok := bodies[e.file]
		if !ok {
			t.Errorf("exemption names %s, which is not in the walked corpus — moved, renamed, or the walk lost it", e.file)
			continue
		}
		if !strings.Contains(body, e.substr) {
			t.Errorf("exemption for %s (%q) matches no line in the current file — remove the stale exemption, or the line it was written for moved/changed and needs re-triage", e.file, e.substr)
		}
	}
}

// TestRhqLeftoverSweepCheckCanFail feeds the predicate corpora it must
// refuse and must pass, so the two tests above are not decoration over a
// predicate that quietly stopped matching anything.
func TestRhqLeftoverSweepCheckCanFail(t *testing.T) {
	// An ordinary unlisted citation must be caught.
	stale := map[string]string{"internal/posse/invented_test.go": "//\tgo test ./internal/rhq -run TestInvented -v\n"}
	if got := rhqLeftoverStaleHits(stale); len(got) != 1 {
		t.Fatalf("an unlisted internal/rhq citation was not caught: %v — the check is judging nothing", got)
	}

	// A real exemption, under its real file and real substring, must be
	// suppressed.
	exempt := map[string]string{
		"internal/posse/gatedkeychain_test.go": "// the whole\n// internal/rhq package stays green.\n",
	}
	if got := rhqLeftoverStaleHits(exempt); len(got) != 0 {
		t.Errorf("a real exemption did not cover the line it exists for: %v", got)
	}

	// The exemption is keyed on file AND substring, not the file alone: a
	// second, different internal/rhq line in an exempted file must still be
	// caught.
	fileWide := map[string]string{
		"internal/posse/gatedkeychain_test.go": "// internal/rhq package stays green.\n// a brand new go test ./internal/rhq recipe nobody triaged\n",
	}
	if got := rhqLeftoverStaleHits(fileWide); len(got) != 1 {
		t.Errorf("exemption leaked file-wide instead of line-by-line: %v", got)
	}
}
