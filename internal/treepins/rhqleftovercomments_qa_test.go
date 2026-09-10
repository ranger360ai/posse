package treepins

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
// WHAT THIS PIN DOES NOT COVER: scripts/*.sh and Makefile still name
// internal/rhq (measured 2026-09-07) but neither is *.go or *.md, so
// widening the walk to the repo root (ranger-base-0f929 finding 1) did not
// pull them in — left alone rather than swept unasked. The root widening DID
// pull in CHANGELOG.md, README.md and this pin's own root-level siblings;
// README.md is clean today, CHANGELOG.md's one hit is a dated changelog
// entry and is in rhqLeftoverExemptions, and this pin's own file
// (rhqLeftoverSelfFile) is excluded from its own corpus rather than
// exempted line by line — see the const's comment.

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
	{"docs/notes.d/notes-testing.md", "On 2026-08-29 `make test` came back exit 2 with ~80 reds in `internal/rhq`,",
		"narrates a specific 2026-08-29 incident, before the rename"},

	// The rows below are ranger-base-0f929's: rhqLeftoverCorpus now walks the
	// repo root (finding 1), which pulls this pin's own sibling QA files and
	// the top-level docs into its population for the first time.
	{"CHANGELOG.md", "The fix is `internal/rhq/credpin.go`, one rule for both readers:",
		"a changelog entry narrates the fix as it stood when it was written; rewriting it misrepresents the historical record the same way a dated MEASURED narration would"},
	{"internal/treepins/suitetimeout_qa_test.go", "DEFAULT -timeout. The default is 10m per package and internal/rhq spends",
		"prose scene-setting for the ranger-base-2ggb timeout measurement, not a citation of the directory as it exists today"},
	{"internal/treepins/suitetimeout_qa_test.go", "internal/rhq's worst measured run is 623.2s, standalone (ranger-base-2ggb,",
		"a dated measurement (ranger-base-2ggb) reporting the package name as it was when the run was measured"},
	{"internal/treepins/adrpackagedirsweep_qa_test.go", "WHAT THE SWEEP DID. The Go package was renamed `internal/rhq` ->",
		"prose narrating the 9c00e192 rename itself, part of this sibling pin's own header"},
	{"internal/treepins/adrpackagedirsweep_qa_test.go", "records. Its done-when was `grep -rn internal/rhq docs/adr` returning",
		"quotes the repro command the sibling pin's done-when was defined against"},
	{"internal/treepins/adrpackagedirsweep_qa_test.go", "number. `0046-constitution-directory-posse.md` keeps its `internal/rhq/`.",
		"describes the one frozen census row that ADR 0046 pin itself exempts; quoting the string it is a census of"},
	{"internal/treepins/adrpackagedirsweep_qa_test.go", `adrSweepRetired = "internal/rhq"`,
		"the string literal the sibling pin's own predicate matches against, not a citation of the directory"},
	{"internal/treepins/adrpackagedirsweep_qa_test.go", `"the resolver lives in `,
		"synthetic fixture text feeding TestADRPackageDirSweepCheckCanFail, invented to prove the predicate catches a stale citation"},
	{"internal/treepins/adrpackagedirsweep_qa_test.go", `"0098-invented.probe.sh": "# argv[0] reset`,
		"synthetic fixture text, same can-fail test, the probe-supplement shape"},
	{"internal/treepins/adrpackagedirsweep_qa_test.go", `below := map[string]string{adrSweepExemptRecord:`,
		"synthetic fixture proving the sibling pin's own heading-keyed exemption covers a line below it"},
	{"internal/treepins/adrpackagedirsweep_qa_test.go", `above := map[string]string{adrSweepExemptRecord:`,
		"synthetic fixture proving the sibling pin's own heading-keyed exemption does NOT cover a line above it"},
	{"internal/treepins/adrpackagedirsweep_qa_test.go", `elsewhere := map[string]string{"0097-invented.md":`,
		"synthetic fixture proving the sibling pin's exemption does not leak to a different record"},
	{"internal/treepins/adrtestcitation_qa_test.go", "records still spelled the retired `internal/rhq/` directory. ranger-base-",
		"prose narrating a sibling pin's own history (ranger-base-efk14/1d8bk), not a live citation"},
	{"internal/treepins/adrtestcitation_qa_test.go", "`internal/rhq/` four days after the package was renamed and nothing went",
		"same historical narration, continued"},
	{"internal/treepins/adrtestcitation_qa_test.go", "0002-container-tier.probe.sh carried `internal/rhq/cagelauncher.go` on main",
		"prose citing the specific stale citation ranger-base-3ni7p's human sweep found, as history"},
	{"internal/treepins/adrtestcitation_qa_test.go", `text: "internal/rhq/constitutionwall_qa_test.go"}`,
		"synthetic fixture: the stale half of a resolved/stale citation pair this pin's own test constructs"},
	{"internal/treepins/adrtestcitation_qa_test.go", `staleSource := adrCite{adr: "0017-runtime-equivalence.md", line: 35, text: "internal/rhq/runtime.go"}`,
		"synthetic fixture: a non-test-file stale citation this pin's test constructs to prove the rule widened past *_test.go"},
	{"internal/treepins/adrtestcitation_qa_test.go", "a non-test citation under the retired internal/rhq/ resolved; widening the rule to every Go file did not take",
		"a t.Fatalf failure message describing the synthetic fixture above it, not a citation of the directory"},
	{"internal/treepins/adrtestcitation_qa_test.go", `prefixed := adrCite{adr: adr, line: 60, text: "internal/rhq/herdrevents.go"}`,
		"synthetic fixture proving a prefixed mention does not resolve as a backreference"},
	{"internal/treepins/adrtestcitation_qa_test.go", "as `internal/rhq/cagelauncher.go` until 2026-09-06; the point of the",
		"prose naming the exact string a real citation carried before 2026-09-06, as history"},
	{"internal/treepins/adrtestcitation_qa_test.go", `stale.text = "internal/rhq/cagelauncher.go"`,
		"synthetic fixture reusing the same historical string as a stale-citation test case"},
	{"internal/treepins/adrtestcitation_qa_test.go", "history is `git show 495d2a6:internal/posse/overflow.go`; live is internal/rhq/runtime.go",
		"synthetic fixture: a git-show-declares-only-its-own-token test case pairing a live and a stale path"},
}

// rhqLeftoverPopFloors are the per-root floors this sweep's population must
// clear, measured 2026-09-07: 576 files under internal/, 53 under cmd/, 7
// docs/*.md outside docs/adr and docs/notes.d, and 60 at the repo root
// (non-recursive, *.go and *.md, excluding this pin's own file — NOTES.md
// and INSTALL.md are counted there, not read separately). A single floor
// over the union let one root's files vanish entirely as long as another
// root's slack covered the loss (ranger-base-0f929 finding 2: docs/ alone
// carries 38 files of slack under the old union floor); keying the floor
// per root means losing ANY single root is what trips it.
var rhqLeftoverPopFloors = map[string]int{
	"internal":          576,
	"cmd":               53,
	"docs":              7,
	".":                 6,
	"internal/treepins": 55,
}

// rhqLeftoverSelfFile is this pin's own file. It is excluded from the root
// walk below: once the root is in the corpus, every literal citation this
// file's own exemption list and doc comments carry — the string the sweep
// exists to find — would have to re-exempt itself against its own
// predicate, one line at a time, forever.
const rhqLeftoverSelfFile = "internal/treepins/rhqleftovercomments_qa_test.go"

// rhqLeftoverCorpus walks internal/, cmd/, docs/ (skipping docs/adr and
// docs/notes.d, which have their own exemption policy) and the repo root
// itself (non-recursive, so it does not re-walk internal/cmd/docs — this is
// where NOTES.md, INSTALL.md and this pin's root-level *_qa_test.go/*.md
// siblings live), and returns every .go/.md file's body keyed by
// repo-relative path.
func rhqLeftoverCorpus(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	counts := map[string]int{}
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
			if path == rhqLeftoverSelfFile {
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
			counts[root]++
			if strings.HasPrefix(path, "internal/treepins/") {
				counts["internal/treepins"]++
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read .: %v", err)
	}
	for _, e := range ents {
		if e.IsDir() || e.Name() == rhqLeftoverSelfFile {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out[e.Name()] = string(b)
		counts["."]++
	}
	// These sections retain the old NOTES.md population. Other notes.d
	// fragments keep their existing frozen-record exclusion.
	archives, err := filepath.Glob("docs/notes.d/notes-*.md")
	if err != nil || len(archives) == 0 {
		t.Fatalf("NOTES history corpus: %v, %v", archives, err)
	}
	for _, path := range archives {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		out[path] = string(body)
	}
	for root, floor := range rhqLeftoverPopFloors {
		if counts[root] < floor {
			t.Fatalf("root %q read %d files, floor is %d — that root has lost files, and every verdict below it would be a green over files it never read", root, counts[root], floor)
		}
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
