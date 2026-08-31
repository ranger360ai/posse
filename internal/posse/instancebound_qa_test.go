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

// Assembled so this file itself does not contain the banned spellings.
func qibCrewPattern() *regexp.Regexp {
	names := []string{
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
	return regexp.MustCompile(`(?i)\b(?:` + strings.Join(names, "|") + `)\b`)
}

// TestShippedTreeNamesRolesNotThisCrew reads EVERY line of EVERY file under
// the three shipped trees — production Go, test Go, testdata, and the toml,
// md and Dockerfiles that ship beside them. There is no per-file rule to get
// wrong because the amendment left none: a file is in scope if it is in the
// tree.
//
// The repo root's directories are not walked here — its non-test .go is
// TestShippedStringsNameRolesNotThisCrew's fourth root below, and its .md is
// the root narrative the amendment excludes on purpose. Its *_test.go family
// WAS a real gap (ranger-base-4say) and gets its own non-recursive pass
// below, after the three trees: root is where the QA suite lives, not a tree
// this walk should recurse into (that would re-read cmd/, internal/, etc/
// and examples/ a second time and double-count their hits).
func TestShippedTreeNamesRolesNotThisCrew(t *testing.T) {
	root := qibRepoRoot(t)
	re := qibCrewPattern()
	var hits []string
	scanned := map[string]int{}
	for _, rel := range qibShippedRoots {
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
			scanned[rel]++
			relPath, _ := filepath.Rel(root, path)
			for i, line := range bytes.Split(body, []byte("\n")) {
				if loc := re.FindIndex(line); loc != nil {
					hits = append(hits, relPath+":"+strconv.Itoa(i+1)+": "+string(line[loc[0]:loc[1]]))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// The repo root's *_test.go family (ranger-base-4say): the live-QA suite
	// sitting beside go.mod, outside every tree walked above. Non-recursive —
	// os.ReadDir does not descend, so cmd/, internal/, etc/ and examples/ are
	// not re-read here, and docs/ and the root .md narrative (D6's excluded
	// class) are never reached either.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	rootTestFiles := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		rootTestFiles++
		body, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range bytes.Split(body, []byte("\n")) {
			if loc := re.FindIndex(line); loc != nil {
				hits = append(hits, e.Name()+":"+strconv.Itoa(i+1)+": "+string(line[loc[0]:loc[1]]))
			}
		}
	}
	// Same fm4p-lesson witness as the per-root floors below, sized off the 34
	// *_test.go files at root when this pass was added — comfortably under
	// that, so ordinary growth doesn't flake it, but nowhere near 0.
	const qibRootTestFloor = 20
	if rootTestFiles < qibRootTestFloor {
		t.Fatalf("only %d _test.go files read at the repo root (floor %d) — the walk found nothing to pin there",
			rootTestFiles, qibRootTestFloor)
	}
	t.Logf("read %d _test.go files at repo root", rootTestFiles)

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
		if scanned[rel] < qibRootFloor[rel] {
			t.Fatalf("only %d files read under %s/ (floor %d) — the walk found nothing to pin there",
				scanned[rel], rel, qibRootFloor[rel])
		}
		t.Logf("read %d files under %s/", scanned[rel], rel)
	}
	if len(hits) > 0 {
		t.Errorf("ADR 0012 App.A 5: the shipped tree names the originating instance (%d hits):\n  %s",
			len(hits), strings.Join(hits, "\n  "))
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

func TestShippedStringsNameRolesNotThisCrew(t *testing.T) {
	root := qibRepoRoot(t)
	re := qibCrewPattern()
	var hits []string
	scanned := 0
	// The repo root is walked NON-recursively as a fourth root: its own .go
	// files ship too (embed.go), and every subdirectory that holds shipped
	// Go is already named above. The test files sitting there are outside
	// TestShippedTreeNamesRolesNotThisCrew's three roots — that gap is the
	// test corpus's, filed as ranger-base-4say and not swept here.
	for _, rel := range []string{"cmd", "internal", "etc", "."} {
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
			for _, lit := range qibShippedStrings(t, path) {
				for i, line := range strings.Split(lit.Text, "\n") {
					loc := re.FindIndex([]byte(line))
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
					hits = append(hits, relPath+":"+strconv.Itoa(at)+": "+line[loc[0]:loc[1]])
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
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
