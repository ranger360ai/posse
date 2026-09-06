package posse_test

// QA pin for ranger-base-efk14, widened to every Go file by ranger-base-tq0gx
// — an ADR that names a source file by name is making a checkable claim, and
// until now only the `_test.go` half of that claim was checked.
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
// THE GAP, which is the general one: the ADRs cite source files by name, and
// no test resolved a single citation. A pin can be renamed, moved or deleted
// and every record still names it; an auditor then has to resolve the name by
// hand, and a hand resolution can miss.
//
// WHY THE NON-TEST HALF WAITED, and what it cost. The first cut
// (ranger-base-efk14) compared base names only, because 12 citations across 7
// records still spelled the retired `internal/rhq/` directory. ranger-base-
// 1d8bk rewrote those twelve and tightened the rule to the exact path, so the
// TEST half has been pinned since. The other half is the one that then rotted
// in the open: nine non-test files across six records still spelled
// `internal/rhq/` four days after the package was renamed and nothing went
// red (found by ranger-base-3ni7p, swept under it).
//
// Widening the token regex from `_test\.go` to `\.go` was never the work,
// which is why ranger-base-tq0gx exists as its own bead. Doing only that left
// 13 tokens unresolved and not one of them a defect a pin should refuse: five
// were the extractor running through a `/`-separated pair of files or through
// the `//` of a URL; four were another repo's source; four named files this
// repo deliberately deleted and cites as history. An allowlist of thirteen
// exceptions pins nothing, so the conventions came first and are written down
// where ADR 0040 puts citation convention — ADR 0051, "Naming a source file".
// This file is their only reader.
//
// WHAT THIS PINS, which is that section, mechanised:
//
//   - LIVE. A bare name is a claim that the file is somewhere in the tree; a
//     name carrying a directory is a claim about where. A stale directory is
//     exactly the defect that made audit finding 7 — a hand resolution of a
//     wrong path.
//   - HISTORICAL. `git show <sha>:<path>` on ONE line. Nothing looks the
//     object up: ADR 0051 already rules that a clone may lack the blob and
//     that a missing object is unjudged rather than a finding, so the SHAPE
//     is the claim. A wrapped `git show` is two lines and is not a citation,
//     which is pinned below because it is a silent trap otherwise.
//   - FOREIGN. The repo name immediately before the path (`bd
//     cmd/bd/nodb.go`), the spelling 0037 and 0046 already used for
//     `ranger-base` paths. It declares scope, not that the path resolves
//     over there.
//   - BACKREFERENCE. Declared once per base name per record; a later BARE
//     mention of that name in the same record needs no repeat. A PREFIXED
//     mention never backreferences — it is a fresh claim about a path.
//
// and the tokenizer those rules need, because `cost.go/cockpit.go` is how the
// records actually write a pair of files.
//
// THE CORPUS is every record in docs/adr, and docs/adr holds two file classes
// (ranger-base-bvich). Alongside the *.md decisions sit the *.probe.sh
// executable supplements — four today — which a record hands its reproduction
// to and which cite source files in their comments and their echoed prose
// exactly the way the record does. Reading only the *.md left that class
// unpinned, and the class is not hypothetical: the launcher citation in
// 0002-container-tier.probe.sh carried `internal/rhq/cagelauncher.go` on main
// for the same four days as the nine non-test citations above, in the same
// directory, and only ranger-base-3ni7p's human sweep found it. A shell
// comment is prose like any other, so every rule above applies to a
// supplement unchanged.
//
// THE FILE NAME RECORDS WHERE IT STARTED and is deliberately not renamed:
// docs/notes.d/adr-adherence-2026-09.md is a dated note that names it, and
// renaming a file a dated note names is the stale-name problem ADR 0051 is
// about.
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
	// decl is "" for a plain citation, "historical" for one inside a
	// `git show <sha>:` and "foreign" for one behind another repo's name.
	decl string
}

// adrToken is one citation and the byte offset it starts at on its line. The
// offset is not decoration: both declaration shapes are recognised by what
// sits immediately in front of the token, so the tokenizer has to hand it
// over.
type adrToken struct {
	start int
	text  string
}

// adrIsCiteChar reports whether b can appear inside a path token. A run of
// these is the unit the tokenizer works on; everything else — backticks,
// parentheses, spaces, `*`, `:` — ends the run.
func adrIsCiteChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	}
	return b == '_' || b == '.' || b == '-' || b == '/'
}

// adrCiteTokens extracts the Go-file citations from one line, per ADR 0051's
// "Naming a source file". The old one-line regex `[A-Za-z0-9_./-]*\.go`
// matched through a `/` and through a `//`, which is why widening it alone
// produced five tokens that were never citations at all.
func adrCiteTokens(line string) []adrToken {
	var out []adrToken
	for i := 0; i < len(line); {
		if !adrIsCiteChar(line[i]) {
			i++
			continue
		}
		start := i
		for i < len(line) && adrIsCiteChar(line[i]) {
			i++
		}
		run := line[start:i]

		// A run carrying `//` is a URL, and a URL's last component can
		// be a Go file — a link into a repository browser is the
		// obvious one. Without this the segment walk below reads
		// `example.com` as a directory and reports the link as a
		// missing file. ADR 0011's research.google link needs no help
		// from it (its `.go` is inside a hostname, so no segment ends
		// in `.go` at all), which is exactly why the arm that measures
		// this guard has to spell a URL that DOES end in one.
		if strings.Contains(run, "//") {
			continue
		}
		// Trailing sentence punctuation is not part of the name. ADR
		// 0039 ends a line with `internal/posse/modelsessiontoken_
		// test.go.` and dropping that citation would be a silent loss
		// of coverage, not a cleanup.
		run = strings.TrimRight(run, ".-")

		off, tokStart := start, start
		var dirs []string
		for _, seg := range strings.Split(run, "/") {
			if len(dirs) == 0 {
				tokStart = off
			}
			if adrIsGoFileSegment(seg) {
				out = append(out, adrToken{start: tokStart, text: strings.Join(append(dirs, seg), "/")})
				dirs = nil
			} else {
				dirs = append(dirs, seg)
			}
			off += len(seg) + 1
		}
	}
	return out
}

// adrIsGoFileSegment reports whether one `/`-separated segment is a Go file
// name rather than a directory. A segment that ends in `.go` is a file, which
// is what splits `cost.go/cockpit.go` into the pair the record means. A
// segment with no base name at all is prose — `cage*.go` leaves the run
// `.go`, and 0048 writes "prove again with a `.go` file" — and so is a bare
// suffix, since Go's own build ignores a file whose name starts with `_` or
// `.` and no record cites one as live code.
func adrIsGoFileSegment(seg string) bool {
	if !strings.HasSuffix(seg, ".go") || len(seg) <= len(".go") {
		return false
	}
	return seg[0] != '_' && seg[0] != '.'
}

// adrHistoricalRe matches the `git show <sha>:` that must sit immediately in
// front of a historical citation, on the same line. A wrapped one does not
// match, deliberately and pinned: ADR 0051 says the citation goes on one line
// precisely so this reader cannot be fooled by a 76-column wrap.
var adrHistoricalRe = regexp.MustCompile(`git show [0-9a-f]{7,40}:$`)

// adrForeignRepos are the neighbouring repositories whose paths the records
// legitimately name and this tree cannot resolve. It is a named list and not
// a pattern on purpose: an unrecognised word in front of a path exempts
// nothing, so a third repo is a deliberate edit here and in ADR 0051.
var adrForeignRepos = []string{"bd", "ranger-base"}

// adrCiteDeclaration classifies what sits in front of a token on its line.
func adrCiteDeclaration(line string, start int) string {
	pre := line[:start]
	if adrHistoricalRe.MatchString(pre) {
		return "historical"
	}
	// The repo name is separated from the path by a space, or by a space
	// and the backtick that opens the code span; both are trimmed off
	// before the name is read.
	trimmed := strings.TrimRight(pre, " `")
	for _, repo := range adrForeignRepos {
		if !strings.HasSuffix(trimmed, repo) {
			continue
		}
		rest := trimmed[:len(trimmed)-len(repo)]
		if rest == "" || !adrIsWordByte(rest[len(rest)-1]) {
			return "foreign"
		}
	}
	return ""
}

// adrIsWordByte reports whether b would make the repo name part of a longer
// word. `bd` is two letters and `ranger-base` is a common enough shape that
// any word or hyphenated id ENDING in one would otherwise declare the path
// after it; the arm named for it probes exactly that.
func adrIsWordByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	}
	return b == '_' || b == '-'
}

// adrIsRecord reports whether a docs/adr entry is part of the corpus. Both
// file classes in that directory are records: the *.md decisions and the
// *.probe.sh supplements they hand their reproduction to. A supplement cites
// source files exactly the way the record does; the arm that says the
// executable half is actually being read is
// TestADRCitationCorpusReadsTheExecutableSupplements.
func adrIsRecord(name string) bool {
	return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".probe.sh")
}

func adrCitations(t *testing.T) []adrCite {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join("docs", "adr"))
	if err != nil {
		t.Fatalf("read docs/adr: %v", err)
	}
	var out []adrCite
	for _, e := range ents {
		if e.IsDir() || !adrIsRecord(e.Name()) {
			continue
		}
		b, err := os.ReadFile(filepath.Join("docs", "adr", e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for i, ln := range strings.Split(string(b), "\n") {
			for _, tok := range adrCiteTokens(ln) {
				out = append(out, adrCite{
					adr:  e.Name(),
					line: i + 1,
					text: tok.text,
					decl: adrCiteDeclaration(ln, tok.start),
				})
			}
		}
	}
	return out
}

// adrGoFileIndex maps every Go file in the tree by base name to the paths
// carrying it.
func adrGoFileIndex(t *testing.T) map[string][]string {
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
		if strings.HasSuffix(d.Name(), ".go") {
			idx[d.Name()] = append(idx[d.Name()], p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return idx
}

// adrCiteResolves is the LIVE rule, unchanged from the `_test.go` cut and
// isolated so the arms below can be shown able to fail over a synthetic
// corpus. A bare name resolves by base name; a name carrying a directory
// resolves only if a file sits at that exact path. The index is keyed by base
// name and holds the tree paths as WalkDir reported them (relative, no
// leading "./"), so the citation is compared cleaned.
func adrCiteResolves(idx map[string][]string, c adrCite) bool {
	paths := idx[filepath.Base(c.text)]
	if !strings.Contains(c.text, "/") {
		return len(paths) > 0
	}
	want := filepath.Clean(c.text)
	for _, p := range paths {
		if filepath.Clean(p) == want {
			return true
		}
	}
	return false
}

// adrDeclaredNames is the backreference radius: per record, the base names
// that record has declared historical or foreign at least once, in full.
func adrDeclaredNames(cites []adrCite) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, c := range cites {
		if c.decl == "" {
			continue
		}
		if out[c.adr] == nil {
			out[c.adr] = map[string]bool{}
		}
		out[c.adr][filepath.Base(c.text)] = true
	}
	return out
}

func adrUnresolved(idx map[string][]string, cites []adrCite) []adrCite {
	declared := adrDeclaredNames(cites)
	var out []adrCite
	for _, c := range cites {
		if c.decl != "" {
			continue
		}
		if adrCiteResolves(idx, c) {
			continue
		}
		// A BARE name the record has already declared is a
		// backreference. A prefixed one is not: it claims a path.
		if !strings.Contains(c.text, "/") && declared[c.adr][filepath.Base(c.text)] {
			continue
		}
		out = append(out, c)
	}
	return out
}

// Every Go file the ADRs name must be in the tree, or be declared historical
// or another repo's under ADR 0051.
func TestADRCitedGoFilesResolveOrAreDeclared(t *testing.T) {
	cites := adrCitations(t)
	idx := adrGoFileIndex(t)

	// Floors. A corpus that has silently emptied — the records moved, the
	// walk rooted somewhere else, the tokenizer stopped matching — would
	// otherwise pass with nothing measured. These are floors on the count,
	// not a coupling to it (measured 2026-09-06: 620 base names over 641
	// files, 378 citations, 142 distinct, across 51 records).
	if len(idx) < 300 {
		t.Fatalf("the tree index holds %d Go base names; the walk is measuring the wrong root", len(idx))
	}
	adrs := map[string]bool{}
	distinct := map[string]bool{}
	tests := 0
	for _, c := range cites {
		adrs[c.adr] = true
		distinct[c.text] = true
		if strings.HasSuffix(c.text, "_test.go") {
			tests++
		}
	}
	if len(cites) < 250 || len(distinct) < 100 || len(adrs) < 20 {
		t.Fatalf("found %d citations (%d distinct) across %d records; the extractor stopped matching",
			len(cites), len(distinct), len(adrs))
	}
	// The half this file started as, floored separately: widening to every
	// Go file must never be the trade that loses the test-file coverage
	// ranger-base-efk14 and -1d8bk built (measured 2026-09-06: 63 mentions
	// of 52 distinct test files).
	if tests < 40 {
		t.Fatalf("found %d *_test.go citations; the widened tokenizer has lost the half this pin started as", tests)
	}

	for _, c := range adrUnresolved(idx, cites) {
		t.Errorf("%s:%d names %s and no such file is in the tree; ADR 0051 'Naming a source file' says a removed file is cited `git show <sha>:<path>` and another repo's is prefixed with its repo name",
			c.adr, c.line, c.text)
	}
}

// The rig, shown able to fail. Without this arm a resolver that answered true
// unconditionally would pass the arm above forever.
func TestADRCitationCheckCanFail(t *testing.T) {
	idx := adrGoFileIndex(t)
	const adr = "0015-constitution-promotion.md"
	cite := func(text string) adrCite { return adrCite{adr: adr, line: 341, text: text} }

	real := cite("bdhookcommit_qa_test.go")
	fake := cite("internal/posse/bdhookcommit_qa_test.go.nosuch_test.go")

	if got := adrUnresolved(idx, []adrCite{fake}); len(got) != 1 {
		t.Fatalf("a citation naming a file that cannot exist was resolved; the check cannot fail and pins nothing")
	}
	if got := adrUnresolved(idx, []adrCite{real}); len(got) != 0 {
		t.Fatalf("the real citation was reported missing; the check refuses everything and separates nothing")
	}

	// The directory half (ranger-base-1d8bk). The base name exists in the
	// tree in every one of these; only the directory differs, which is the
	// shape of the stale citations this arm exists to keep out.
	realPath := adrCite{adr: "0002-runtimes-and-gates.md", line: 864, text: "internal/posse/constitutionwall_qa_test.go"}
	stalePath := adrCite{adr: "0002-runtimes-and-gates.md", line: 864, text: "internal/rhq/constitutionwall_qa_test.go"}
	rootedElsewhere := cite("internal/posse/bdhookcommit_qa_test.go")

	if got := adrUnresolved(idx, []adrCite{realPath}); len(got) != 0 {
		t.Fatalf("a citation spelling the path the file is at was reported missing")
	}
	if got := adrUnresolved(idx, []adrCite{stalePath}); len(got) != 1 {
		t.Fatalf("a citation spelling a directory that does not exist resolved on its base name; the rule is still base-name-only")
	}
	if got := adrUnresolved(idx, []adrCite{rootedElsewhere}); len(got) != 1 {
		t.Fatalf("a citation placing a root-level pin under a package resolved; the rule is still base-name-only")
	}

	// The non-test half (ranger-base-tq0gx). The nine files that rotted in
	// the open were source, not pins, and the widened rule must refuse the
	// same shape there.
	staleSource := adrCite{adr: "0017-runtime-equivalence.md", line: 35, text: "internal/rhq/runtime.go"}
	liveSource := adrCite{adr: "0017-runtime-equivalence.md", line: 35, text: "internal/posse/runtime.go"}
	if got := adrUnresolved(idx, []adrCite{staleSource}); len(got) != 1 {
		t.Fatalf("a non-test citation under the retired internal/rhq/ resolved; widening the rule to every Go file did not take")
	}
	if got := adrUnresolved(idx, []adrCite{liveSource}); len(got) != 0 {
		t.Fatalf("a non-test citation at the path the file is at was reported missing")
	}
}

// The declaration half, and the two escapes it could open. A declaration is
// the reason twelve non-defects are no longer an allowlist, so what it does
// NOT exempt is the whole of its safety.
func TestADRCitationDeclarationsExemptOnlyWhatTheyDeclare(t *testing.T) {
	idx := adrGoFileIndex(t)
	const adr = "0016-herdr-event-hints.md"

	declared := adrCite{adr: adr, line: 45, text: "internal/posse/herdrevents.go", decl: "historical"}
	backref := adrCite{adr: adr, line: 55, text: "herdrevents.go"}
	if got := adrUnresolved(idx, []adrCite{declared, backref}); len(got) != 0 {
		t.Fatalf("a declared removal and its bare backreference were refused: %v", got)
	}
	// Without the declaration, both are defects. This is the arm that says
	// the pair above passed on the declaration and not on nothing.
	undeclared := declared
	undeclared.decl = ""
	if got := adrUnresolved(idx, []adrCite{undeclared, backref}); len(got) != 2 {
		t.Fatalf("an undeclared deleted file and its bare mention were not both refused, got %d", len(got))
	}

	// ESCAPE 1: the radius is one base name, not the record. A record that
	// declares herdrevents.go must not thereby exempt a stale runtime.go.
	other := adrCite{adr: adr, line: 60, text: "runtimeprobe.go.nosuch.go"}
	if got := adrUnresolved(idx, []adrCite{declared, other}); len(got) != 1 {
		t.Fatalf("a declaration in a record exempted a DIFFERENT base name in it; the backreference radius is the record, not the name")
	}

	// ESCAPE 3: the radius is one RECORD, not the corpus. Every arm above
	// keeps both cites in the SAME record, so a guard widened from
	// declared[c.adr][name] to "any record declared this name" — the natural
	// simplification, and the shape a reader who forgot the per-record half
	// would write — leaves the whole root package green (ranger-base-9ycqa
	// finding 2). ADR 0051 says "Declare once per base name per record", and
	// the cost of losing it is that one `git show` of overflow.go in 0010
	// exempts a bare, unresolvable overflow.go in all fifty other records.
	elsewhere := adrCite{adr: "0010-plan-guard-overflow.md", line: 150, text: "internal/posse/herdrevents.go", decl: "historical"}
	acrossRecords := adrCite{adr: adr, line: 55, text: "herdrevents.go"}
	if got := adrUnresolved(idx, []adrCite{elsewhere, acrossRecords}); len(got) != 1 {
		t.Fatalf("a declaration in %s exempted a bare mention in %s; the backreference radius is the corpus, not the record, got %v",
			elsewhere.adr, acrossRecords.adr, got)
	}
	// And its control, one field apart: the SAME pair inside one record is
	// the backreference the rule allows, so the arm above failed on the
	// record boundary and not on the name.
	sameRecord := elsewhere
	sameRecord.adr = adr
	if got := adrUnresolved(idx, []adrCite{sameRecord, acrossRecords}); len(got) != 0 {
		t.Fatalf("the same declaration and bare mention inside one record were refused: %v", got)
	}

	// ESCAPE 2: a prefixed mention never backreferences. It is a fresh
	// claim about a path and must resolve or declare itself.
	prefixed := adrCite{adr: adr, line: 60, text: "internal/rhq/herdrevents.go"}
	if got := adrUnresolved(idx, []adrCite{declared, prefixed}); len(got) != 1 {
		t.Fatalf("a PREFIXED citation resolved as a backreference to a declaration; a stale directory can now ride in behind a git show")
	}

	// A foreign declaration behaves the same way, and does not claim the
	// path resolves over there.
	foreign := adrCite{adr: "0055-store-of-record-rides-the-session-env.md", line: 32, text: "cmd/bd/nodb.go", decl: "foreign"}
	foreignBare := adrCite{adr: "0055-store-of-record-rides-the-session-env.md", line: 130, text: "nodb.go"}
	if got := adrUnresolved(idx, []adrCite{foreign, foreignBare}); len(got) != 0 {
		t.Fatalf("a declared foreign path and its bare backreference were refused: %v", got)
	}
}

// The executable half of docs/adr, floored separately because widening the
// corpus to it is the whole of ranger-base-bvich and a count that quietly
// returns to zero is how it would be lost again. Three of the four
// supplements carry Go citations today — 0002-container-tier,
// 0014-l4-worktree-narrowing and 0014-path-scoped-writes; 0009-gate-shell
// names none and is not floored, because a supplement is not required to
// cite anything.
func TestADRCitationCorpusReadsTheExecutableSupplements(t *testing.T) {
	cites := adrCitations(t)

	from := map[string]int{}
	for _, c := range cites {
		if strings.HasSuffix(c.adr, ".probe.sh") {
			from[c.adr]++
		}
	}
	// Floor, not a coupling (measured 2026-09-06: 5 citations across 3
	// supplements). At zero the *.md-only corpus is back and this arm is
	// the only thing that says so.
	if len(from) < 3 {
		t.Fatalf("the corpus carries citations from %d executable supplements (%v); docs/adr/*.probe.sh is being skipped and the class that rotted is unpinned again", len(from), from)
	}

	// The named regression, asserted at its own path. This citation stood
	// as `internal/rhq/cagelauncher.go` until 2026-09-06; the point of the
	// widening is that the NEXT rename of that directory reds here instead
	// of waiting for a sweep.
	const (
		adr  = "0002-container-tier.probe.sh"
		want = "internal/posse/cagelauncher.go"
	)
	found := false
	for _, c := range cites {
		if c.adr == adr && c.text == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s no longer contributes %s to the corpus; either the supplement stopped naming the launcher or it is no longer being read", adr, want)
	}

	// And the supplements are held to the rule, not merely collected: a
	// stale directory in one is refused exactly as it is in a record.
	idx := adrGoFileIndex(t)
	stale := adrCite{adr: adr, line: 148, text: "internal/rhq/cagelauncher.go"}
	live := adrCite{adr: adr, line: 148, text: want}
	if got := adrUnresolved(idx, []adrCite{stale}); len(got) != 1 {
		t.Fatalf("the supplement's own stale spelling resolved; reading the file buys nothing if the rule does not apply to it")
	}
	if got := adrUnresolved(idx, []adrCite{live}); len(got) != 0 {
		t.Fatalf("the supplement's live citation was reported missing")
	}
}

// The tokenizer, over the exact lines the records actually carry. Five of the
// thirteen leftovers ranger-base-tq0gx measured were the old regex reading
// through a separator, and one real citation sits at the end of a sentence.
func TestADRCitationTokenizerReadsTheRecordsAsWritten(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		want []string
	}{
		{"a pair of files, 0003:21", "model ids and exact price rows live in runtime.go/cost.go; an instance may",
			[]string{"runtime.go", "cost.go"}},
		{"a pair of files, 0041:184", "  in herdrback.go/runtimeprobe.go), so this is a new launcher-to-gate",
			[]string{"herdrback.go", "runtimeprobe.go"}},
		{"a URL whose host holds a .go, 0011:482", "  https://research.google/pubs/the-chubby-lock-service-for-loosely-coupled/",
			nil},
		{"a URL whose last component IS a Go file", "the shape is at https://github.com/ranger360ai/posse/blob/main/embed.go today",
			nil},
		{"a glob, 0047:123", "cage*.go, dispatch.go, relaunch.go and everything the launch path",
			[]string{"dispatch.go", "relaunch.go"}},
		{"an extension in prose, 0048:220", "   installed, re-run step 3 and prove again with a `.go` file; expect the",
			nil},
		{"a suffix in prose, 0051", "records where it started, at `_test.go` citations alone.",
			nil},
		{"end of a sentence, 0039:202", "  internal/posse/modelsessiontoken_test.go.",
			[]string{"internal/posse/modelsessiontoken_test.go"}},
		{"a path in a code span", "`internal/posse/gates.go`: `adrShaGuardBody` and its assembly path",
			[]string{"internal/posse/gates.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, tok := range adrCiteTokens(tc.line) {
				got = append(got, tok.text)
			}
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Errorf("tokens %v, want %v", got, tc.want)
			}
		})
	}
}

// The declaration shapes, over the lines the records carry — including the
// wrap, which is the one trap ADR 0051's "on ONE line" exists to close.
func TestADRCitationDeclarationShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		want []string // one verdict per token on the line, in order
	}{
		{"0010:150, a removed file at a sha",
			"The removed mechanism's own code is `git show 495d2a6:internal/posse/overflow.go`.",
			[]string{"historical"}},
		{"0055:32, another repo inside the code span",
			"- The mechanism is in bd's source, not in a version: `bd cmd/bd/nodb.go`",
			[]string{"foreign"}},
		{"0046:23, another repo outside the code span",
			"(ranger-base-4pjnm, ranger-base `docs/runbooks/cutover.go` @ fc7f13c)",
			[]string{"foreign"}},
		{"a git show declares only the token it is glued to",
			"history is `git show 495d2a6:internal/posse/overflow.go`; live is internal/rhq/runtime.go",
			[]string{"historical", ""}},
		{"a wrapped git show declares nothing",
			"39ce664:internal/posse/herdrevents.go` (the commit before the removal)",
			[]string{""}},
		{"a repo name that is only the tail of a word declares nothing",
			"the herdbd internal/posse/watch.go seam is unchanged",
			[]string{""}},
		{"a bead id is not a repo name",
			"the seam and the pins (ranger-base-4dxpo internal/posse/watch.go)",
			[]string{""}},
		{"a plain live citation",
			"real list twice: `internal/posse/runtime.go`'s `Runtime` struct — 23",
			[]string{""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, tok := range adrCiteTokens(tc.line) {
				got = append(got, adrCiteDeclaration(tc.line, tok.start))
			}
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("verdicts %q, want %q", got, tc.want)
			}
		})
	}
}

// The record and this reader are one rule in two places, so the record has to
// still say it. ADR 0040 puts citation convention in 0051; if that section
// goes, this file is enforcing a convention nothing declares.
func TestADR0051CarriesTheSourceFileCitationConvention(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("docs", "adr", "0051-landed-is-a-bead-id.md"))
	if err != nil {
		t.Fatalf("read ADR 0051: %v", err)
	}
	// The record is prose at ~76 columns and a phrase that outlives an
	// edit can still move across a line break, so the body is read with
	// its whitespace collapsed. A wrap is not a drift; a deleted sentence
	// still is, and still reds here.
	body := strings.Join(strings.Fields(string(b)), " ")
	for _, want := range []string{
		"## Naming a source file",
		"`git show <sha>:<path>`, on ONE line",
		"the repo name goes immediately before the",
		"Declare once per base name per record",
		"A PREFIXED mention never is",
		"`adrtestcitation_qa_test.go`",
		// The corpus itself (ranger-base-bvich). adrIsRecord takes two
		// file classes and the record has to be the one that says so,
		// or this file is reading a directory on its own authority.
		"reads every record in `docs/adr`",
		"the `*.md` decisions and the `*.probe.sh` supplements",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ADR 0051 no longer says %q; the record and its only reader have drifted apart", want)
		}
	}
	// The foreign-repo list is named in both places, and an unlisted name
	// exempts nothing — so a repo added here without the record saying so
	// is a rule with no home.
	for _, repo := range adrForeignRepos {
		if !adrForeignRepoDeclared(body, repo) {
			t.Errorf("adrForeignRepos carries %q and ADR 0051's \"Another repo's\" sentence does not name it", repo)
		}
	}
}

// adrForeignRepoDeclared: does ADR 0051 name this repo WHERE it declares what
// a foreign repo is? The check this replaces asked whether the word appeared
// in a code span anywhere in the record, which is satisfied by prose about
// something else entirely (ranger-base-9ycqa finding 3): 0051:24 carries the
// span `posse gates adr-census` about the census command, so adding "posse"
// to adrForeignRepos — the likeliest third entry anyone would ever add, and
// the one that would silently exempt every path in this repo — passed green.
//
// The record's own structure is the radius. "Another repo's" opens the
// paragraph that defines the convention, and that paragraph is where a third
// repo has to be argued for, which is the close's stated safety property: a
// deliberate edit in BOTH places.
func adrForeignRepoDeclared(body, repo string) bool {
	const opens = "**Another repo's**"
	i := strings.Index(body, opens)
	if i < 0 {
		return false // the sentence itself is gone; every repo fails, loudly
	}
	para := body[i:]
	if j := strings.Index(para, "\n\n"); j >= 0 {
		para = para[:j]
	}
	return strings.Contains(para, "`"+repo+" ") || strings.Contains(para, "`"+repo+"`")
}

// The narrowing, shown able to fail — and shown to still pass what it must.
// Without the second arm a predicate that refused everything would satisfy
// the first, and the record would be unmaintainable in the other direction.
func TestADR0051ForeignRepoCheckReadsTheForeignPathSentence(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("docs", "adr", "0051-landed-is-a-bead-id.md"))
	if err != nil {
		t.Fatalf("read ADR 0051: %v", err)
	}
	body := string(b)

	// The escape, by name. `posse` IS in the record, in a code span, and it
	// says nothing about foreign paths.
	if !strings.Contains(body, "`posse gates adr-census`") {
		t.Fatalf("0051 no longer carries the span this arm is built on; the escape it measures may have moved")
	}
	if adrForeignRepoDeclared(body, "posse") {
		t.Error("`posse` counted as a declared foreign repo; the check is still reading the whole record, and this tree's own paths would be exempt")
	}
	if adrForeignRepoDeclared(body, "zzrepo") {
		t.Error("a name the record does not carry at all read as declared")
	}
	// Both live entries pass, and they pass in the two spellings the
	// sentence actually uses: `bd cmd/bd/nodb.go` and `ranger-base`.
	for _, repo := range adrForeignRepos {
		if !adrForeignRepoDeclared(body, repo) {
			t.Errorf("%q is declared in the foreign-path sentence and read as undeclared", repo)
		}
	}
	// The sentence going away is not a pass. Nothing else in the record
	// declares the convention, so a body without it must fail every entry.
	gutted := strings.Replace(body, "**Another repo's**", "**Another repository is**", 1)
	for _, repo := range adrForeignRepos {
		if adrForeignRepoDeclared(gutted, repo) {
			t.Errorf("%q still read as declared with the foreign-path sentence gone", repo)
		}
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

	idx := adrGoFileIndex(t)
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
