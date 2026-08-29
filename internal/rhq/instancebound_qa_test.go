package rhq

// QA pin for ADR 0012 App.A 5 (verifying rangerhq-24yt under rangerhq-ikx5).
// A fresh deployer's `go test ./...` must not name the originating instance's
// crew, operator, or home. rangerhq-24yt renamed the suite onto the shipped
// example roles; this pin is the invariant that commit did not encode, and
// that rangerhq-oay walked back the next day.
//
// Live since ranger-base-h6fx: skipped from cd365fa to that bead, which is
// exactly how the corpus refilled with crew names (223 hits over five days,
// 339 by the sweep). A skipped invariant is documentation, not a pin.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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

func TestFixturesNameRolesNotThisCrew(t *testing.T) {
	root := qibRepoRoot(t)
	re := qibCrewPattern()
	var hits []string
	for _, rel := range []string{"cmd", "internal", "etc"} {
		err := filepath.WalkDir(filepath.Join(root, rel), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			base := d.Name()
			if base == "instancebound_qa_test.go" {
				return nil
			}
			inTestdata := false
			for _, p := range strings.Split(path, string(os.PathSeparator)) {
				if p == "testdata" {
					inTestdata = true
					break
				}
			}
			if !inTestdata && !strings.HasSuffix(base, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
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
	if len(hits) > 0 {
		t.Errorf("ADR 0012 App.A 5: fixture names the originating instance (%d hits):\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}

// ─── the shipped half of the same invariant (ranger-base-q3gp) ───────────────
//
// TestFixturesNameRolesNotThisCrew above reads test files and testdata only,
// so non-test .go was outside it by construction. Two shipped strings were
// living in that blind spot at 116288e: `DefaultPulsePersona = "monica"` (the
// default a deployer's pulse prompts) and a `measured by laurie` line inside
// the gate-shell template, which is written into every persona's rendered
// hook file rather than merely read here.
//
// WHERE THE LINE IS, decided once so the next sweeper does not re-litigate
// it: this pin reads STRING LITERALS and not comments. A string is what a
// deployer's install and output are made of — a default they inherit, a file
// posse renders into their home. A comment naming who measured a thing is an
// inert provenance marker, the same class ADR 0012 D6 keeps deliberately for
// private-tracker ids ("no mass sweep") and the same class the crew's names
// occupy throughout docs/ and NOTES.md. Sweeping 15 attributions out of
// internal/rhq would cost the only record of who measured what and buy a
// fresh deployer nothing they can run into.
//
// The two roots differ in the same way: over test files every line counts
// (rangerhq-24yt rewrote prose there, and h6fx kept that), over shipped code
// only what ships does.
//
// What it cannot see, stated rather than implied: a name assembled from
// fragments ("mon"+"ica") is two literals to the parser and passes. That is
// the trick qibCrewPattern itself uses, and it is deliberate — a regression
// that reintroduces a crew name spells it whole; a concatenation is somebody
// working around the pin, which is a different problem from an accident.
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
	// TestFixturesNameRolesNotThisCrew's three roots — that gap is the test
	// corpus's and is filed as its own bead, not swept here.
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
