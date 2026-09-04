package posse

// QA pin for ADR 0012 App.A 5 (verifying rangerhq-24yt under rangerhq-ikx5).
// A fresh deployer's `go test ./...` must not name the originating instance's
// crew, operator, or home. rangerhq-24yt renamed the suite onto the shipped
// example roles; this pin is the invariant that commit did not encode, and
// that rangerhq-oay walked back the next day.
//
// Live since ranger-base-h6fx: skipped from cd365fa to that bead, which is
// exactly how the corpus refilled with crew names (223 hits over five days,
// 339 by the sweep). A skipped invariant is documentation, not a pin.
//
// WIDENED to the whole tree by ranger-base-he9y, after ADR 0012 App.A 5 was
// amended (ranger-base-cqbq, 272bb35) to say so in as many words: App.A 5
// "reaches every line cmd/, internal/, and etc/ ship — comments included,
// not string literals alone ... The edge is the tree, not the syntax."
// Until then this walk read *_test.go and testdata/ only, so 16 comment
// lines under the same roots named the crew where no pin could see them.
// docs/ and the root narrative files stay outside: there the crew are
// historical actors and D6's no-mass-sweep governs them, as it governs ids.
//
// WIDENED again by rangerhq-gk4k to examples/ — the seed tree embed.go ships
// inside the binary — see qibShippedRoots.
//
// WIDENED again by ranger-base-4say to the repo root's *_test.go family: the
// live-QA suite sitting beside go.mod (release_injection_qa_test.go,
// bdpin_qa_test.go and 32 more) held 7 comment-prose hits that neither this
// walk (root untouched) nor TestShippedStringsNameRolesNotThisCrew's fourth
// root (test files excluded there on purpose) could see. Fixed at the names,
// not swept: renamed to the roles rangerhq-24yt already established for the
// rest of the corpus, or dropped where the attribution read worse renamed
// than gone (ADR 0012 D2/D6). See qibRootTestFloor below.
//
// WIDENED again by ranger-base-o3g6a from lines to NAMES. Both pins below
// read file CONTENTS and nothing else: this walk computed relPath for its
// message and then matched `line` only, and the literal pin parses string
// literals, which a file name is not. A path ships in every clone exactly as
// a line does, and one file — a probe named after a QA seat, arrived at
// 95e7939 — sat on main under both greens for a day. Both walks now match the
// path they already had in hand, and say which kind of hit each one is.
//
// An archive id survives a sweep either way — "measured (rangerhq-lrnp)" is
// the shape the amendment names: D6 grandfathers *ids* (nothing promises to
// resolve one), D2 depersonalizes *names* (any deployer could have written
// the line).

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func qibRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %s has no go.mod: %v", root, err)
	}
	return root
}

// qibShippedRoots are the trees a deployer receives. Three of them are the
// code trees ADR 0012 D6's amendment names in as many words; examples/ is the
// fourth, and it is the one that ships hardest — embed.go carries it INTO the
// binary and `posse init` copies it into a fresh RHQ_HOME (ADR 0012 D5), so a
// crew name landing there is not a comment a deployer reads past, it is the
// config and the PIDs their instance starts with. D2's "persona names become
// roles" is the rule; the amendment's list was answering whether COMMENTS
// count, not carving out the seed. Measured before widening (rangerhq-gk4k):
// with `# verify_assignee: <a crew name>` appended to examples/config.yaml —
// the exact line that bead was filed for — both pins in this file passed.
var qibShippedRoots = []string{"cmd", "internal", "etc", "examples"}

// qibRootFloor is the per-root witness: how many files the walk must actually
// read there before an absence of hits means anything.
var qibRootFloor = map[string]int{"cmd": 5, "internal": 150, "etc": 1, "examples": 15}

// Assembled so this file itself does not contain the banned spellings. One
// list, two patterns below, so a name added here reaches both readers.
func qibCrewNames() []string {
	return []string{
		"di" + "nesh",
		"gil" + "foyle",
		"hoo" + "ver",
		"lau" + "rie",
		"jar" + "ed",
		"mon" + "ica",
		"rich" + "ard",
		"erl" + "ich",
		"hol" + "den",
		"gw" + "art",
		"jian" + "-yang",
		"jian" + "Yang",
		"da" + "ve",
		"david" + "stacy",
	}
}

// qibCrewPattern reads LINES, where \b is right: prose puts spaces and
// punctuation around a name, and requiring them is what keeps the pin off
// every word that merely contains one.
func qibCrewPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(?:` + strings.Join(qibCrewNames(), "|") + `)\b`)
}

// qibCrewPathPattern reads PATHS, and deliberately drops \b. A path is not
// prose and has none of prose's boundaries: in Go's own file-naming
// convention the separator is `_`, which is a word character, so \b never
// fires beside it. Measured (ranger-base-o3g6a): the line pattern does NOT
// match `internal/posse/<a QA seat>_as19_probe_test.go`, which is the exact
// file this arm was written for. Requiring a boundary here would be a path
// arm blind to the one hit that motivated it — asserted, not just said, in
// TestShippedPinsSeeACrewNameInAPath.
//
// The cost is a substring match: a file named after something that merely
// CONTAINS a crew name would fail this pin. Nothing in the shipped tree does
// (censused at the fix), the failure is loud and names the file, and the fix
// for one is a rename — which is what a real hit needs anyway. A false
// negative here is a crew name riding main under a green pin, which is the
// defect this whole file exists to prevent, so the trade is not close.
func qibCrewPathPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:` + strings.Join(qibCrewNames(), "|") + `)`)
}

// qibReaders is the pair of patterns every scan needs: one for lines, one for
// paths. They are different regexps for the reason qibCrewPathPattern gives,
// and they travel together so no scanner can be handed one and quietly use it
// for both — which is how a path arm would come out blind to `_`.
type qibReaders struct{ line, path *regexp.Regexp }

func qibCrewReaders() qibReaders {
	return qibReaders{line: qibCrewPattern(), path: qibCrewPathPattern()}
}

// qibPathHit reports a crew name in a file's own PATH. The name a file ships
// UNDER is not a line of any file, which is why nothing saw it: both pins
// here read contents only (ranger-base-o3g6a). `re` must be
// qibCrewPathPattern's, not the line pattern's — see there for why the two
// are not one regexp.
//
// The hit is worded, and carries no line number, so a reader can tell it from
// a content hit without going to look for a line that is not there.
// Slash-separated so the message reads the same wherever the suite runs.
func qibPathHit(re *regexp.Regexp, relPath string) []string {
	rel := filepath.ToSlash(relPath)
	loc := re.FindStringIndex(rel)
	if loc == nil {
		return nil
	}
	return []string{rel + ": in the FILE NAME, not its content: " + rel[loc[0]:loc[1]]}
}

// qibTreeScan is what one pass of TestShippedTreeNamesRolesNotThisCrew read.
// The reading is factored out of the test (ranger-base-o3g6a) so the wrong
// arm at the bottom of this file can point the same scanner at a scratch tree
// it planted known hits in. Nothing is decided here: the floors and the
// verdict stay with the caller.
type qibTreeScan struct {
	hits          []string
	scanned       map[string]int
	rootTestFiles int
}

// qibScanShippedTree reads EVERY line of EVERY file under the given roots —
// production Go, test Go, testdata, and the toml, md and Dockerfiles that
// ship beside them — plus the NAME each of them ships under. There is no
// per-file rule to get wrong because the amendment left none: a file is in
// scope if it is in the tree.
//
// The root's directories are not walked here — its non-test .go is
// TestShippedStringsNameRolesNotThisCrew's fourth root below, and its .md is
// the root narrative the amendment excludes on purpose. Its *_test.go family
// WAS a real gap (ranger-base-4say) and gets its own non-recursive pass at
// the end: root is where the QA suite lives, not a tree this walk should
// recurse into (that would re-read cmd/, internal/, etc/ and examples/ a
// second time and double-count their hits).
func qibScanShippedTree(t *testing.T, root string, roots []string, re qibReaders) qibTreeScan {
	t.Helper()
	scan := qibTreeScan{scanned: map[string]int{}}
	for _, rel := range roots {
		err := filepath.WalkDir(filepath.Join(root, rel), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if d.Name() == "instancebound_qa_test.go" {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scan.scanned[rel]++
			relPath, _ := filepath.Rel(root, path)
			scan.hits = append(scan.hits, qibPathHit(re.path, relPath)...)
			for i, line := range bytes.Split(body, []byte("\n")) {
				if loc := re.line.FindIndex(line); loc != nil {
					scan.hits = append(scan.hits, filepath.ToSlash(relPath)+":"+strconv.Itoa(i+1)+": "+string(line[loc[0]:loc[1]]))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// The root's *_test.go family (ranger-base-4say): the live-QA suite
	// sitting beside go.mod, outside every tree walked above. Non-recursive —
	// os.ReadDir does not descend, so cmd/, internal/, etc/ and examples/ are
	// not re-read here, and docs/ and the root .md narrative (D6's excluded
	// class) are never reached either.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		scan.rootTestFiles++
		scan.hits = append(scan.hits, qibPathHit(re.path, e.Name())...)
		body, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range bytes.Split(body, []byte("\n")) {
			if loc := re.line.FindIndex(line); loc != nil {
				scan.hits = append(scan.hits, e.Name()+":"+strconv.Itoa(i+1)+": "+string(line[loc[0]:loc[1]]))
			}
		}
	}
	return scan
}

func TestShippedTreeNamesRolesNotThisCrew(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)
	scan := qibScanShippedTree(t, root, qibShippedRoots, qibCrewReaders())

	// Same fm4p-lesson witness as the per-root floors below, sized off the 34
	// *_test.go files at root when this pass was added — comfortably under
	// that, so ordinary growth doesn't flake it, but nowhere near 0.
	const qibRootTestFloor = 20
	if scan.rootTestFiles < qibRootTestFloor {
		t.Fatalf("only %d _test.go files read at the repo root (floor %d) — the walk found nothing to pin there",
			scan.rootTestFiles, qibRootTestFloor)
	}
	t.Logf("read %d _test.go files at repo root", scan.rootTestFiles)

	// A pin that measures pure absence is satisfied by measuring nothing
	// (the fm4p lesson, and the guard q3gp put on the pin below): say how
	// many files were actually read — and say it PER ROOT, because a total
	// is a witness the big roots can pay on their own: examples/ is 25 files
	// against ~285, so a walk that read none of it still clears any total
	// floor. Each floor is well under what its root holds, so it fails on a
	// broken walk and not on ordinary growth.
	//
	// The floors are read off qibRootFloor and not off the walk list, so
	// deleting a root from qibShippedRoots — the cheapest way to make this
	// pin stop complaining — fails it at 0 files rather than passing it by
	// having nothing to say.
	for _, rel := range qibShippedRoots {
		if _, ok := qibRootFloor[rel]; !ok {
			t.Fatalf("root %s/ is walked with no floor in qibRootFloor — an unwitnessed root", rel)
		}
	}
	floored := make([]string, 0, len(qibRootFloor))
	for rel := range qibRootFloor {
		floored = append(floored, rel)
	}
	sort.Strings(floored)
	for _, rel := range floored {
		if scan.scanned[rel] < qibRootFloor[rel] {
			t.Fatalf("only %d files read under %s/ (floor %d) — the walk found nothing to pin there",
				scan.scanned[rel], rel, qibRootFloor[rel])
		}
		t.Logf("read %d files under %s/", scan.scanned[rel], rel)
	}
	if len(scan.hits) > 0 {
		t.Errorf("ADR 0012 App.A 5: the shipped tree names the originating instance (%d hits):\n  %s",
			len(scan.hits), strings.Join(scan.hits, "\n  "))
	}
}

// ─── the string-literal pin (ranger-base-q3gp) ───────────────────────────────
//
// q3gp landed this to reach non-test .go, which the walk above could not see
// at the time, and drew the line at STRING LITERALS on the reasoning that a
// comment naming who measured a thing is inert provenance. **The amendment
// overruled that line** (ranger-base-cqbq): the edge is the tree, so the walk
// above now reads every line of every file under the same three roots, and
// every literal it finds there is a line that walk already read.
//
// This pin is kept anyway, because it is not the subset that makes it. Two
// things it sees that a raw-line walk cannot:
//
//  1. **The repo root.** Its fourth root (below) reads root non-test .go —
//     embed.go and its neighbours — which is outside the three trees.
//  2. **A name split by an escape.** It unquotes before matching, so
//     "base\nMONICA" is two lines to the regexp and the name is at a line
//     start. In the raw source `n` and `M` are adjacent word characters and
//     \b never fires between them, so the line walk above reads straight
//     past it — the h6fx trap, and the reason deleting this test as
//     redundant would quietly reopen it.
//
// What NEITHER sees, stated rather than implied: a name assembled from
// fragments ("mon"+"ica") is two literals to the parser and two word-parts to
// the regexp. That is the trick qibCrewPattern itself uses, and it is
// deliberate — a regression that reintroduces a crew name spells it whole; a
// concatenation is somebody working around the pin, which is a different
// problem from an accident.
func qibShippedStrings(t *testing.T, path string) []struct {
	Line int
	Text string
	Raw  bool
} {
	t.Helper()
	fset := token.NewFileSet()
	// Mode 0: comments are not retained, so they cannot be read here even by
	// accident — the scope decision above is enforced by the parser.
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []struct {
		Line int
		Text string
		Raw  bool
	}
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		text := lit.Value
		// Unquote so a name split by an escape ("base\nMONICA") is one
		// string to the regexp: in the raw source `n` and `M` are adjacent
		// word characters and \b never fires between them (the h6fx trap).
		if s, err := strconv.Unquote(lit.Value); err == nil {
			text = s
		}
		out = append(out, struct {
			Line int
			Text string
			Raw  bool
		}{fset.Position(lit.Pos()).Line, text, strings.HasPrefix(lit.Value, "`")})
		return true
	})
	return out
}

// qibScanShippedStringFiles is TestShippedStringsNameRolesNotThisCrew's
// reading, factored out of it (ranger-base-o3g6a) for the same reason as the
// tree scanner above: the wrong arm at the bottom of this file points it at a
// scratch tree with known hits planted in it. The floors and the verdict stay
// with the caller.
//
// The repo root is walked NON-recursively as a fourth root: its own .go files
// ship too (embed.go), and every subdirectory that holds shipped Go is already
// named. The test files sitting there are outside qibScanShippedTree's roots —
// that gap is the test corpus's, filed as ranger-base-4say and not swept here.
func qibScanShippedStringFiles(t *testing.T, root string, roots []string, re qibReaders) (hits []string, scanned int) {
	t.Helper()
	for _, rel := range roots {
		err := filepath.WalkDir(filepath.Join(root, rel), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || (rel == "." && path != filepath.Join(root, rel)) {
					return filepath.SkipDir
				}
				return nil
			}
			base := d.Name()
			if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") {
				return nil
			}
			scanned++
			relPath, _ := filepath.Rel(root, path)
			// The file's own name (ranger-base-o3g6a). This walk is the only
			// reader of the repo root's non-test .go, so a crew name arriving
			// as embed.go's neighbour is held here or nowhere.
			hits = append(hits, qibPathHit(re.path, relPath)...)
			for _, lit := range qibShippedStrings(t, path) {
				for i, line := range strings.Split(lit.Text, "\n") {
					loc := re.line.FindIndex([]byte(line))
					if loc == nil {
						continue
					}
					// A raw string is byte-for-byte its source, so the line
					// within it is a real source line — which is the whole
					// point for a template like sharedIndexBody, 60 lines
					// long. An escape-bearing string is one source line.
					at := lit.Line
					if lit.Raw {
						at += i
					}
					hits = append(hits, filepath.ToSlash(relPath)+":"+strconv.Itoa(at)+": "+line[loc[0]:loc[1]])
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return hits, scanned
}

func TestShippedStringsNameRolesNotThisCrew(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)
	hits, scanned := qibScanShippedStringFiles(t, root, []string{"cmd", "internal", "etc", "."}, qibCrewReaders())
	// A pin that measures pure absence is satisfied by measuring nothing
	// (the fm4p lesson): say how many files were actually parsed.
	if scanned < 20 {
		t.Fatalf("only %d shipped .go files scanned — the walk found nothing to pin", scanned)
	}
	t.Logf("scanned %d shipped .go files", scanned)
	if len(hits) > 0 {
		t.Errorf("ADR 0012 App.A 5: shipped string names the originating instance (%d hits):\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}

// ─── the wrong arm for both path arms (ranger-base-o3g6a) ────────────────────
//
// Both scanners above now match the NAME a file ships under as well as its
// lines. An arm nobody has shown able to fail is an absence, not a pin — and
// this one WAS an absence for as long as it existed: the tree walk built
// relPath for its message and then matched `line` only, so a probe named
// after a QA seat rode main green under both pins for a day.
//
// So: a scratch tree with the four cases that separate the arms — a name in
// the path, a name in a comment, a name in a string literal, and a file that
// is clean both ways — read by the same two scanners the pins above call.
// Each must report exactly the hits planted and no others, and must say which
// KIND each hit is, because a reader who cannot tell a name hit from a
// content hit cannot act on either.
func TestShippedPinsSeeACrewNameInAPath(t *testing.T) {
	t.Parallel()
	re := qibCrewReaders()
	root := t.TempDir()
	// Assembled the way qibCrewNames is, so this file still does not contain
	// the spellings the pins ban.
	crew := "lau" + "rie"
	clean := "package main\n\n// a comment naming nobody\n"
	inComment := "package main\n\n// measured by " + crew + " on a Tuesday\n"
	inLiteral := "package main\n\nvar who = \"measured by " + crew + "\"\n"
	if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	plant := map[string]string{
		// Under a walked root: clean, name-in-path, name-in-comment,
		// name-in-literal.
		filepath.Join("cmd", "innocent.go"):    clean,
		filepath.Join("cmd", crew+"_probe.go"): clean,
		filepath.Join("cmd", "incomment.go"):   inComment,
		filepath.Join("cmd", "inliteral.go"):   inLiteral,
		// At the root: the *_test.go family the tree scanner reads, and the
		// non-test .go only the literal scanner reaches (embed.go's class).
		crew + "_root_test.go": clean,
		"clean_root_test.go":   clean,
		crew + "_embed.go":     clean,
	}
	for rel, body := range plant {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// The reason the two patterns are not one pattern, asserted rather than
	// left in a comment: `\b` does not fire before `_`, so the LINE pattern
	// cannot see the planted path. Anyone who "simplifies" qibCrewPathPattern
	// back to the line pattern fails here instead of shipping a blind arm.
	planted := "cmd/" + crew + "_probe.go"
	if re.line.MatchString(planted) {
		t.Fatalf("the line pattern now matches %q — this fixture no longer measures the boundary trap that motivated the path arm", planted)
	}
	if !re.path.MatchString(planted) {
		t.Fatalf("the path pattern does not match %q — the path arm is blind to the very shape it was written for", planted)
	}

	// file [kind], so the assertion reads on the two things that matter: which
	// file, and whether the scanner called it a name or a content hit.
	summarise := func(hits []string) []string {
		out := make([]string, 0, len(hits))
		for _, h := range hits {
			kind := "content"
			if strings.Contains(h, "in the FILE NAME") {
				kind = "name"
			}
			out = append(out, strings.SplitN(h, ":", 2)[0]+" ["+kind+"]")
		}
		sort.Strings(out)
		return out
	}
	same := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// The tree scanner reads every line of every file under cmd/, plus the
	// root's *_test.go family. It does NOT read the root's non-test .go —
	// that is the literal scanner's fourth root — so <crew>_embed.go is
	// correctly absent here.
	scan := qibScanShippedTree(t, root, []string{"cmd"}, re)
	wantTree := []string{
		crew + "_root_test.go [name]",
		"cmd/" + crew + "_probe.go [name]",
		"cmd/incomment.go [content]",
		"cmd/inliteral.go [content]",
	}
	sort.Strings(wantTree)
	if got := summarise(scan.hits); !same(got, wantTree) {
		t.Errorf("tree scanner hits:\n  got  %v\n  want %v", got, wantTree)
	}
	if scan.scanned["cmd"] != 4 || scan.rootTestFiles != 2 {
		t.Errorf("tree scanner read cmd/=%d root _test.go=%d; want 4 and 2 — a control that read the wrong files proves nothing",
			scan.scanned["cmd"], scan.rootTestFiles)
	}

	// The literal scanner parses with comments OFF, so incomment.go is
	// correctly absent; it skips *_test.go, so the root test file is too. It
	// is the only reader of the root's non-test .go, which is why the path
	// arm belongs there as well as on the tree walk.
	hits, scanned := qibScanShippedStringFiles(t, root, []string{"cmd", "."}, re)
	wantLit := []string{
		crew + "_embed.go [name]",
		"cmd/" + crew + "_probe.go [name]",
		"cmd/inliteral.go [content]",
	}
	sort.Strings(wantLit)
	if got := summarise(hits); !same(got, wantLit) {
		t.Errorf("literal scanner hits:\n  got  %v\n  want %v", got, wantLit)
	}
	if scanned != 5 {
		t.Errorf("literal scanner parsed %d files; want 5 — a control that read the wrong files proves nothing", scanned)
	}
}
