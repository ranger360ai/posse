package treepins

// QA pins for ranger-base-jv2q3 — the park ref namespace is spelled one way,
// and the procedure that types it by hand says so.
//
// WHY. A push hold parks the held commits on a `refs/heads/parked/<name>` ref
// and resets the base out from under them (ranger-base-zlvyk). posse writes
// none of those refs: parking is a hand procedure at the gate, by decision,
// because it runs once per hold. Two programs nevertheless CONSUME that
// prefix — the carrier census (scripts/park-carriers.sh) and the landing
// guard that refuses a landing whose range reaches a held commit
// (ranger-base-3s2ig) — so a hand convention is now an enforcement input.
//
// The failure mode belongs to the procedure and not to either program. A park
// spelled some other way — `refs/parked/<name>`, a tag, `hold-<name>`,
// `parked-<name>` with no slash — is a park neither reader can see: the
// census prints nothing, the guard refuses nothing, every carrier lands as it
// would have before either existed, and nothing reds. MEASURED 2026-11-22:
// seven park refs in this repository's checkout and zero occurrences of the
// prefix in any tracked doc, script or runbook before ranger-base-3s2ig.
//
// So this pin holds the two halves together:
//
//  1. every spelling in a tracked PROGRAM is the one namespace, and at least
//     one program still carries it (a census over a class that has gone empty
//     passes by saying nothing);
//  2. the procedure page exists, names the prefix, states the consequence of
//     spelling it otherwise, cites paths that resolve, and is reachable from
//     the orientation page a seat is handed.
//
// Taking ranger-base-jv2q3's other option — a namespace posse writes,
// `refs/posse/parked/*` — reds arm 1 on the new constant and arm 2 on the
// page, which is the intent: that change moves the writer and this pin is
// where the two readers and the procedure are held in agreement. The bead
// asks for exactly this tie, from the lane that landed the guard, after
// measuring that the census's glob and the guard's constant agree today with
// nothing holding them there.
//
// WHAT IT DOES NOT READ, said out loud: prose outside the procedure page. A
// document must be able to NAME a wrong spelling in order to refuse it — this
// page does, and so does the runbook — so a byte census over markdown would
// red on the paragraph above that explains the rule. Enforcement lives in
// programs; arm 2 reads the one document whose job is the convention.
//
// Doored by `make treepins-check`, the package minus its priced table
// (ADR 0098 D4): a pin in this package is behind that door the moment it is
// written, with no Makefile edit.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// pnsPrefix is the one park ref namespace. Spelled here as the literal both
// readers carry, so a mutation of either is a diff against this line.
const pnsPrefix = "refs/heads/parked"

// pnsProcedure is the page the hand convention is written on, and
// pnsOrientation is the page that has to be able to send a reader there.
const (
	pnsProcedure   = "docs/runbooks/push-hold-park.md"
	pnsOrientation = "AGENTS.md"
)

// pnsFullRef matches a park-flavored ref written out from `refs/`. The class
// excludes `*` and `<`, so a glob or a placeholder match ends at the prefix,
// which is what is being judged.
var pnsFullRef = regexp.MustCompile(`refs/[A-Za-z0-9._/-]*park[A-Za-z0-9._/-]*`)

// pnsShortRef matches the short branch form — a `park…/` path component that
// is NOT already inside a full ref, since a preceding `/` disqualifies it.
// This is the form a park is actually typed with (`git branch parked/<name>`).
var pnsShortRef = regexp.MustCompile(`(^|[^A-Za-z0-9._/-])(park[A-Za-z0-9._-]*)/`)

// pnsRoster: tracked programs allowed to carry a spelling this pin refuses,
// each with its reason. Checked back against the census below — a row for a
// file that stopped carrying one is a door held open for nothing.
var pnsRoster = map[string]string{
	"internal/treepins/parknamespace_qa_test.go": "this census's own control literals — the wrong spellings it has to be able to refuse",
}

// pnsHit is one refused spelling, with where it was read.
//
// The walk roots below are spelled as literals in their ranges, not composed
// and not taken as a parameter, because that is the spelling the tree-wide
// derivation can read (treewidedoor_qa_test.go's twdRelTreePath is
// literal-only — a root behind a variable leaves the pin outside the class).
type pnsHit struct {
	file  string
	line  int
	found string
	why   string
}

// pnsJudge returns the park ref spellings in one chunk of program text that
// are NOT the one namespace, with the reason each is refused. A chunk with no
// park ref in it at all returns nothing and counts nothing.
func pnsJudge(text string) (bad []string, seen int) {
	for _, m := range pnsFullRef.FindAllString(text, -1) {
		seen++
		if m == pnsPrefix || strings.HasPrefix(m, pnsPrefix+"/") {
			continue
		}
		bad = append(bad, m)
	}
	for _, m := range pnsShortRef.FindAllStringSubmatch(text, -1) {
		seen++
		if m[2] == "parked" {
			continue
		}
		bad = append(bad, m[2]+"/")
	}
	return bad, seen
}

// pnsScanGo parses the tree's Go sources and judges their string literals.
// Literals and not bytes, for the reason the head comment gives: a comment
// explaining the rule names the spellings the rule refuses.
func pnsScanGo(t *testing.T) (hits []pnsHit, parsed, seen int) {
	t.Helper()
	fset := token.NewFileSet()
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == "testdata" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			rel := filepath.ToSlash(path)
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			parsed++
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil || !strings.Contains(v, "park") {
					return true
				}
				bad, n2 := pnsJudge(v)
				seen += n2
				for _, b := range bad {
					hits = append(hits, pnsHit{file: rel, line: fset.Position(lit.Pos()).Line, found: b, why: "Go string literal"})
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return hits, parsed, seen
}

// pnsScanScripts judges the runnable half — scripts/ — by line, with whole
// comment lines dropped for the same reason the Go half reads literals.
func pnsScanScripts(t *testing.T) (hits []pnsHit, read, seen int) {
	t.Helper()
	for _, root := range []string{"scripts"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			switch {
			case strings.HasSuffix(path, ".sh"), strings.HasSuffix(path, ".py"):
			default:
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			read++
			rel := filepath.ToSlash(path)
			for i, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				bad, n := pnsJudge(line)
				seen += n
				for _, s := range bad {
					hits = append(hits, pnsHit{file: rel, line: i + 1, found: s, why: "script line"})
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return hits, read, seen
}

// Arm 1. Every park ref spelling a tracked program carries is the one
// namespace — and the class is not empty, which is the half a census cannot
// prove by passing.
func TestQAEveryParkRefSpellingInATrackedProgramIsTheOneNamespace(t *testing.T) {
	t.Parallel()

	goHits, parsed, goSeen := pnsScanGo(t)
	shHits, read, shSeen := pnsScanScripts(t)

	if parsed < 200 {
		t.Fatalf("only %d Go files parsed under cmd/ and internal/ — the walk this arm censuses found nothing", parsed)
	}
	if read < 20 {
		t.Fatalf("only %d scripts read under scripts/ — the walk this arm censuses found nothing", read)
	}

	// The positive witness. "No program spells it wrong" is also what a
	// census reading an empty class says, and the class is one prefix in two
	// programs: if it reads zero, either the readers are gone — in which case
	// this pin and the procedure page are describing machinery that no longer
	// exists — or the matcher stopped matching.
	if seen := goSeen + shSeen; seen < 1 {
		t.Fatalf("no park ref spelling found in any tracked program (scripts/park-carriers.sh carried one on 2026-11-22, and ranger-base-3s2ig's guard adds a second) — either the readers of %s are gone, and %s with them, or this census has stopped seeing its own class", pnsPrefix, pnsProcedure)
	}

	carried := map[string]bool{}
	for _, h := range append(goHits, shHits...) {
		carried[h.file] = true
		if _, ok := pnsRoster[h.file]; ok {
			continue
		}
		t.Errorf("%s:%d (%s) spells a park ref %q, and the one namespace is %s/<name> — a park spelled otherwise is invisible to the carrier census AND to the landing guard, so the landing they exist to refuse goes through and nothing reds (ranger-base-jv2q3; the procedure is %s)", h.file, h.line, h.why, h.found, pnsPrefix, pnsProcedure)
	}
	for file, why := range pnsRoster {
		if !carried[file] {
			t.Errorf("pnsRoster exempts %s (%s) and it no longer carries a refused spelling — drop the row; an exemption nobody needs is a hole held open", file, why)
		}
	}
	t.Logf("parsed %d Go files and %d scripts; %d park ref spellings judged", parsed, read, goSeen+shSeen)
}

// pnsInAFencedBlock reports whether want appears inside a ``` fence in doc.
// Fences are counted by their opening line, so an unclosed one takes the rest
// of the document, which is what a renderer does too.
func pnsInAFencedBlock(doc, want string) bool {
	in := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			in = !in
			continue
		}
		if in && strings.Contains(line, want) {
			return true
		}
	}
	return false
}

// pnsRepoPath matches a repository path as the procedure page cites one.
var pnsRepoPath = regexp.MustCompile(`(?:docs|scripts|internal|cmd|etc|examples)/[A-Za-z0-9._/-]+`)

// Arm 2. The procedure page is where the convention is written, so it has to
// exist, name the prefix, say what spelling it otherwise costs, cite paths a
// reader can open, and be reachable from the page a seat is handed.
func TestQATheParkProcedureNamesTheNamespaceAndIsReachable(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile(pnsProcedure)
	if err != nil {
		t.Fatalf("read %s: %v — the park namespace is an enforcement input typed by hand, and this page is the only place it is written for the person typing it (ranger-base-jv2q3)", pnsProcedure, err)
	}
	doc := string(b)

	if !strings.Contains(doc, pnsPrefix+"/") {
		t.Errorf("%s does not name %s/<name> — the page is the procedure and the prefix is what the procedure gets wrong silently", pnsProcedure, pnsPrefix)
	}

	// Named in a FENCED BLOCK, not merely somewhere on the page. MEASURED
	// while inverting this arm: with the prefix constant moved to
	// `refs/posse/parked` — ranger-base-jv2q3's other option — the
	// whole-document check above stayed green, because the page's own closing
	// section names that namespace as the alternative it is NOT using. A
	// counterexample in prose satisfied a check meant to read a prescription,
	// so the prescription has to be read where the procedure shows what to
	// type.
	if !pnsInAFencedBlock(doc, pnsPrefix+"/") {
		t.Errorf("%s names %s/<name> nowhere in a fenced block — the page has to SHOW the spelling as the thing to type, and a prefix mentioned only in prose is as likely to be the spelling it is warning about", pnsProcedure, pnsPrefix)
	}

	// The consequence, not just the prefix. A page that names the namespace
	// and not what the other spellings cost reads as a style preference, and
	// the whole finding is that getting it wrong is silent. Matched over
	// whitespace-folded text, because a sentence in a hard-wrapped document
	// is not a line and a quote pinned to one wrapping breaks on a reflow.
	folded := strings.Join(strings.Fields(doc), " ")
	for _, quoted := range []string{"spelled any other way", "nothing reds"} {
		if !strings.Contains(folded, quoted) {
			t.Errorf("%s does not say %q — a park spelled otherwise is seen by neither reader and nothing reds, and that sentence is why the prefix is written down at all", pnsProcedure, quoted)
		}
	}

	// Every path it cites resolves. The page sends its reader to the census
	// and to this pin by path, and a path inside a document is checked by
	// nothing (the same class as mergeblockedrunbook_qa_test.go's).
	cites := map[string]bool{}
	for _, c := range pnsRepoPath.FindAllString(doc, -1) {
		cites[strings.TrimRight(c, ".,)")] = true
	}
	if !cites["scripts/park-carriers.sh"] {
		t.Errorf("%s cites no carrier census — the procedure's step 3 is the census, and the page is what names it", pnsProcedure)
	}
	paths := make([]string, 0, len(cites))
	for c := range cites {
		paths = append(paths, c)
	}
	sort.Strings(paths)
	for _, c := range paths {
		if _, err := os.Stat(c); err != nil {
			t.Errorf("%s sends its reader to %s, which is not in the tree (%v)", pnsProcedure, c, err)
		}
	}

	// Reachability. A procedure nobody is sent to is the state this bead
	// found: the convention existed in seven live refs and in no page.
	o, err := os.ReadFile(pnsOrientation)
	if err != nil {
		t.Fatalf("read %s: %v", pnsOrientation, err)
	}
	if !strings.Contains(string(o), pnsProcedure) {
		t.Errorf("%s does not cite %s — the procedure page is reachable only by whoever already knows it is there, which is the reader who did not need it", pnsOrientation, pnsProcedure)
	}
}

// Arm 3. The control: drive the matcher over planted spellings, both ways.
// A census that cannot be shown to refuse anything is a green test nobody has
// seen work (internal/treepins/README.md, "Adding a pin", step 4).
func TestQAParkNamespaceScannerCanStillSayNo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text    string
		wantBad []string
		wantSee int
	}{
		// The one namespace, in the three spellings the tree uses.
		{"park_glob='refs/heads/parked/*'", nil, 1},
		{"git branch parked/held main", nil, 1},
		{`git for-each-ref refs/heads/parked`, nil, 1},
		// The spellings the finding names. Each is a park the census and the
		// guard both read past.
		{"refs/parked/oe884", []string{"refs/parked/oe884"}, 1},
		{"refs/tags/parked/oe884", []string{"refs/tags/parked/oe884"}, 1},
		{"refs/posse/parked/oe884", []string{"refs/posse/parked/oe884"}, 1},
		{"git branch parked-oe884/x main", []string{"parked-oe884/"}, 1},
		{"git branch parking/oe884 main", []string{"parking/"}, 1},
		// Neither a ref nor a park: the word in its other sense, and a
		// shell variable named for one. A census that scored these would be
		// unusable in a tree whose CHANGELOG parks beads on every page.
		{"a question or risk bead parked with no end date", nil, 0},
		{"the plan guard parks the work", nil, 0},
		{`parked="$parked$short"`, nil, 0},
		{`case "$ref" in $park_glob)`, nil, 0},
		{`short="${park#refs/heads/}"`, nil, 0},
	}
	for _, c := range cases {
		bad, seen := pnsJudge(c.text)
		if seen != c.wantSee {
			t.Errorf("pnsJudge(%q) saw %d park refs, want %d — the matcher's reach decides what the census can refuse", c.text, seen, c.wantSee)
		}
		if strings.Join(bad, ",") != strings.Join(c.wantBad, ",") {
			t.Errorf("pnsJudge(%q) refused %v, want %v", c.text, bad, c.wantBad)
		}
	}

	// And the judgement itself, over the prefix constant rather than over
	// text: a mutation that accepted any prefix would pass every row above
	// that carries a park ref at all, because they all contain the word.
	if bad, _ := pnsJudge("refs/heads/parkedx/oe884"); len(bad) != 1 {
		t.Errorf("a ref whose component merely STARTS with the namespace is accepted — %s/<name> is a path component and not a prefix match", pnsPrefix)
	}

	// And the fence reader arm 2 leans on, both ways: it is the difference
	// between a page PRESCRIBING a spelling and a page mentioning one.
	const fenced = "text\n\n```\n" + pnsPrefix + "/<name>\n```\n\nmore text\n"
	if !pnsInAFencedBlock(fenced, pnsPrefix+"/") {
		t.Errorf("pnsInAFencedBlock cannot see a spelling inside a fence")
	}
	if pnsInAFencedBlock("a namespace posse writes, "+pnsPrefix+"/x, would not\n", pnsPrefix+"/") {
		t.Errorf("pnsInAFencedBlock scores a mention in prose, which is the hole that MEASURED pass came through")
	}
	if pnsInAFencedBlock("```\nnothing here\n```\nand "+pnsPrefix+"/x in prose after it\n", pnsPrefix+"/") {
		t.Errorf("pnsInAFencedBlock does not close a fence — everything after the first block scores")
	}
}
