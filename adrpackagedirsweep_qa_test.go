package posse_test

// QA pin for ranger-base-ydbs9, verifying ranger-base-3ni7p's close.
//
// WHAT THE SWEEP DID. The Go package was renamed `internal/rhq` ->
// `internal/posse` in 9c00e192 (2026-08-31). The records went on naming the
// retired directory for days: ranger-base-1d8bk rewrote the twelve `_test.go`
// citations, ranger-base-3ni7p the remaining twelve live ones, across six
// records. Its done-when was `grep -rn internal/rhq docs/adr` returning
// nothing, and it deliberately returns ONE line instead — see the exemption
// below.
//
// WHY THIS PIN EXISTS BESIDE adrtestcitation_qa_test.go, which is the other
// reader of these citations and does not overlap it. That one resolves a
// citation that names a Go FILE: a token only exists for it once a
// `/`-separated segment ends in `.go` (`adrIsGoFileSegment`). Two of the
// twelve sites the sweep rewrote are invisible to it, and both were measured
// surviving a full root-package run (397.8s, zero FAIL) with the sweep
// reverted at that site:
//
//   - `0015-constitution-promotion.md:368` cites the package DIRECTORY,
//     `internal/posse`, with no file after it. A run of pure directory
//     segments emits no token at all, so nothing judged it.
//   - `0002-container-tier.probe.sh:148` cites `cagelauncher.go` inside an
//     executable supplement. That reader's corpus was `docs/adr/*.md`.
//     ranger-base-bvich widens the corpus to both record classes and kills
//     this second mutant; it does not change the tokenizer, so it does not
//     touch the first.
//
// So this pin is not a resolver and does not try to be one. It pins the
// SWEEP: the retired directory name is gone from every record of either
// class, and the records that carried it still name the live one.
//
// THE ONE EXEMPTION, and why it is keyed on a heading rather than a line
// number. `0046-constitution-directory-posse.md` keeps its `internal/rhq/`.
// That row is a census of literal strings — it counts what the remaining
// `rhq` hits ARE — so replacing the string falsifies the count rather than
// updating a pointer. It is also, and this is what licenses the exemption
// automatically, below that record's `## Historical procedure and evidence
// (superseded in full)` heading: 0046 was retired 2026-09-05 and the section
// is frozen. If the row is ever hoisted above that heading, or the heading
// goes, the exemption stops applying and this pin refuses the line like any
// other — a claim that moves out from under its guard is the failure this
// keying is written against.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	adrSweepRetired = "internal/rhq"
	adrSweepLive    = "internal/posse"

	// adrSweepExemptRecord and adrSweepExemptHeading are the single
	// exemption. The heading is matched in full: a reworded heading is a
	// different editorial claim and must re-earn the exemption.
	adrSweepExemptRecord  = "0046-constitution-directory-posse.md"
	adrSweepExemptHeading = "## Historical procedure and evidence (superseded in full)"
)

// adrSweepRecords are the six records ranger-base-3ni7p rewrote. Each must
// still name the live package directory. Without this arm the pin is
// one-way: a citation deleted outright, or a corpus that quietly stopped
// reading a file, would pass it by naming nothing at all.
var adrSweepRecords = []string{
	"0001-persona-intent-documents.md",
	"0002-container-tier.probe.sh",
	"0013-turn-outcome-refusal-probe.md",
	"0015-constitution-promotion.md",
	"0017-runtime-equivalence.md",
	"0037-venue-restricted-runtime.md",
}

// adrSweepCorpusFloor is the number of records in docs/adr at the sweep,
// measured 2026-09-06: 58 `.md` plus 4 `.sh` supplements. A reader that
// walks fewer than this has lost a class or a directory and is reporting a
// green over a corpus it never read.
const adrSweepCorpusFloor = 62

// adrSweepRecordFile reports whether one docs/adr entry is a record. Both
// classes count: the executable supplements cite source paths in their
// comments exactly the way the prose records do, and the sweep rewrote one
// of them.
func adrSweepRecordFile(name string) bool {
	return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".sh")
}

// adrSweepStaleHits returns "<record>:<line>" for every line naming the
// retired directory that the exemption does not cover. body is keyed by
// record name so the same predicate serves the live tree and the synthetic
// corpus the can-fail arm feeds it.
func adrSweepStaleHits(bodies map[string]string) []string {
	var out []string
	for name, body := range bodies {
		lines := strings.Split(body, "\n")
		exemptFrom := -1
		if name == adrSweepExemptRecord {
			for i, ln := range lines {
				if strings.TrimSpace(ln) == adrSweepExemptHeading {
					exemptFrom = i
					break
				}
			}
		}
		for i, ln := range lines {
			if !strings.Contains(ln, adrSweepRetired) {
				continue
			}
			if exemptFrom >= 0 && i > exemptFrom {
				continue
			}
			out = append(out, name+":"+strconv.Itoa(i+1))
		}
	}
	return out
}

// adrSweepCorpus reads every record in docs/adr, keyed by base name.
func adrSweepCorpus(t *testing.T) map[string]string {
	t.Helper()
	dir := filepath.Join("docs", "adr")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	out := map[string]string{}
	for _, e := range ents {
		if e.IsDir() || !adrSweepRecordFile(e.Name()) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out[e.Name()] = string(b)
	}
	if len(out) < adrSweepCorpusFloor {
		t.Fatalf("read %d records from %s, floor is %d — a corpus this small has lost a class or a directory, and every verdict below it would be a green over unread files", len(out), dir, adrSweepCorpusFloor)
	}
	return out
}

// TestADRRecordsNameTheLivePackageDirectory is the sweep's done-when, held.
func TestADRRecordsNameTheLivePackageDirectory(t *testing.T) {
	bodies := adrSweepCorpus(t)
	for _, hit := range adrSweepStaleHits(bodies) {
		t.Errorf("docs/adr/%s names the retired package directory %q; it was renamed to %q in 9c00e192 and the records were swept under ranger-base-3ni7p. A record that cites the string as history declares it the way %s does — below that record's %q heading",
			hit, adrSweepRetired, adrSweepLive, adrSweepExemptRecord, adrSweepExemptHeading)
	}
}

// TestADRSweptRecordsStillCiteTheLiveDirectory is the other direction. On its
// own the arm above passes over a record whose citation was deleted, and over
// a corpus that read nothing.
func TestADRSweptRecordsStillCiteTheLiveDirectory(t *testing.T) {
	bodies := adrSweepCorpus(t)
	for _, name := range adrSweepRecords {
		body, ok := bodies[name]
		if !ok {
			t.Errorf("docs/adr/%s is not in the corpus — ranger-base-3ni7p rewrote a citation in it, so either the record moved and this table is stale or the reader stopped seeing its class", name)
			continue
		}
		if !strings.Contains(body, adrSweepLive) {
			t.Errorf("docs/adr/%s names %q nowhere — ranger-base-3ni7p put one there, and a citation that vanished is not a citation that was fixed", name, adrSweepLive)
		}
	}
}

// TestADRPackageDirSweepCheckCanFail feeds the predicate a corpus it must
// refuse and one it must pass. Without it, both arms above would read exactly
// the same on a predicate that had quietly stopped matching anything.
func TestADRPackageDirSweepCheckCanFail(t *testing.T) {
	// The two shapes the live tree cannot exercise, because the live tree is
	// swept: a stale citation in an ordinary record, and one in a supplement.
	stale := map[string]string{
		"0099-invented.md":       "the resolver lives in `internal/rhq/gates.go`\n",
		"0098-invented.probe.sh": "# argv[0] reset (internal/rhq/cagelauncher.go).\n",
	}
	got := adrSweepStaleHits(stale)
	if len(got) != 2 {
		t.Fatalf("the check passed a corpus with two stale citations in it, one per record class: %v — it is judging nothing and both arms above are decoration", got)
	}

	// The exemption, both ways round. Below the heading it is covered; the
	// identical line above the heading is not, which is the whole reason the
	// exemption is keyed on the heading and not on the record's name.
	below := map[string]string{adrSweepExemptRecord: "# ADR 0046\n" + adrSweepExemptHeading + "\n| docs | … | `internal/rhq/` package paths |\n"}
	if got := adrSweepStaleHits(below); len(got) != 0 {
		t.Errorf("the exemption did not cover the census row it exists for: %v", got)
	}
	above := map[string]string{adrSweepExemptRecord: "# ADR 0046\n| docs | … | `internal/rhq/` package paths |\n" + adrSweepExemptHeading + "\n"}
	if got := adrSweepStaleHits(above); len(got) != 1 {
		t.Errorf("a stale citation ABOVE the superseded-in-full heading was exempted anyway: %v — the exemption is then the record's name, and a claim hoisted out of the frozen section walks out from under its guard", got)
	}

	// And the exemption is one record's, not every record's.
	elsewhere := map[string]string{"0097-invented.md": adrSweepExemptHeading + "\n`internal/rhq/promote.go`\n"}
	if got := adrSweepStaleHits(elsewhere); len(got) != 1 {
		t.Errorf("another record borrowed 0046's exemption by copying its heading: %v", got)
	}
}
