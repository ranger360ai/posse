package rhq

// ranger-base-ufdy (verifying ranger-base-d4ya / ranger-base-ig1o): the tree
// is gofmt-clean, and something says so on every run.
//
// d4ya and ig1o filed the same one-line drift in internal/rhq/seatbelt.go — a
// missing bare `//` in a doc comment — and ig1o fixed it with `gofmt -w` and
// added no test, on the reasoning that "gofmt itself is the check". Nothing
// runs gofmt: the Makefile's only gofmt line is `fmt:`, which WRITES, and
// `make test` and `vet` never ask. So the deliverable of both beads was a
// STATE of the tree with nothing holding it, which is how the same drift
// arrived twice in one day and was found by hand both times.
//
// go/format is what gofmt runs; comparing its output to the file is the same
// question `gofmt -l` answers, without needing a gofmt on PATH — which in a
// gated session is a shim, not the tool.

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The roots `make fmt` writes, so the pin and its fix name the same files:
// `gofmt -w cmd internal embed.go`. The root is walked non-recursively for
// its own .go, which is embed.go and its neighbours.
var qgfRoots = []string{"cmd", "internal", "."}

func TestTreeIsGofmtClean(t *testing.T) {
	root := qibRepoRoot(t)
	var drifted []string
	scanned := 0
	for _, rel := range qgfRoots {
		err := filepath.WalkDir(filepath.Join(root, rel), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if rel == "." && path != filepath.Join(root, rel) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			want, err := format.Source(body)
			if err != nil {
				// Unparseable is the compiler's complaint, not this pin's.
				return nil
			}
			if string(want) != string(body) {
				relPath, _ := filepath.Rel(root, path)
				drifted = append(drifted, relPath)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// A pin that measures pure absence is satisfied by measuring nothing:
	// say how many files were actually read, and fail on a walk that found
	// none rather than passing by having nothing to say. The floor is well
	// under what the tree holds, so it survives ordinary growth.
	if scanned < 100 {
		t.Fatalf("only %d .go files read across %v — the walk found nothing to pin", scanned, qgfRoots)
	}
	t.Logf("read %d .go files across %v", scanned, qgfRoots)
	if len(drifted) > 0 {
		t.Errorf("not gofmt-clean (%d files) — run `make fmt`:\n  %s",
			len(drifted), strings.Join(drifted, "\n  "))
	}
}

// The comparison above reports absence, so it is worth exactly as much as
// its ability to report a presence. Feed it the drift on purpose.
//
// The shape is ig1o's, and it is narrower than "a missing blank line":
// gofmt normalises list-item separation to be CONSISTENT, so a list whose
// first items are separated by a bare `//` and whose last item is not gets
// the separator inserted — which is precisely what happened to sbSeal's
// Deny/Seal/Keep list. MEASURED against the real pre-fix file (1e86fdf^)
// while writing this: format.Source rewrites it, and its first diff is that
// inserted `//` at the "- Keep:" item. A list with NO separators anywhere is
// already consistent and is left alone, so a control built on "a blank line
// is missing" would have passed over the bug.
func TestGofmtCleanPinDetectsDrift(t *testing.T) {
	clean := "package p\n\n// A doc.\n//\n//   - one, with a body that\n//     wraps a line.\n//\n//   - two follows, separated the same way.\nfunc f() {}\n"
	got, err := format.Source([]byte(clean))
	if err != nil {
		t.Fatalf("format.Source: %v", err)
	}
	if string(got) != clean {
		t.Fatalf("the pin's own idea of clean is wrong — it would report the whole tree:\n%s", got)
	}
	// The same comment with the last item's separator dropped.
	drifted := "package p\n\n// A doc.\n//\n//   - one, with a body that\n//     wraps a line.\n//\n//   - two, with a body that\n//     wraps a line.\n//   - three follows with no separator.\nfunc f() {}\n"
	got, err = format.Source([]byte(drifted))
	if err != nil {
		t.Fatalf("format.Source: %v", err)
	}
	if string(got) == drifted {
		t.Fatal("format.Source did not rewrite the comment shape ig1o filed — the pin above cannot see the drift it exists for (ranger-base-d4ya / ranger-base-ig1o)")
	}
	if !strings.Contains(string(got), "//\n//   - three") {
		t.Errorf("the rewrite is not the separator insertion this pin is about:\n%s", got)
	}
}
