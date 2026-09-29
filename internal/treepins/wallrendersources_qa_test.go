package treepins

// QA pin for ranger-base-vso72 — the membership half of the wall-renderer
// reading, held mechanically so the next render site cannot ship without it.
//
// THE TWO THINGS THAT GO STALE BY HAND, and the reason this is a pin and not
// a comment. `posse.WallRenderSources` is the pathspec list `posse promote`
// and `posse gates` narrow the launcher lag through ("40 commits behind, 2 of
// them rendering a wall"), and every wall rendered at launch carries
// `RenderedByPosse()` in its header so a seat can tell "relaunch pending"
// from "install pending". Both are lists of files kept by hand, and both fail
// SILENTLY when a new render site is added: the count under-reports, and the
// new wall says "rendered from the PID at launch" while naming nobody — the
// exact sentence ranger-base-mmhhc's twelve stale caged launches read.
//
// THE DERIVED CLASS: an internal/posse source whose own STRING LITERALS say
// `do not edit`. That is the render's own claim about itself — posse writes
// this file, a human must not — and it is what every one of the seven current
// render sites spells. String literals and not the file text, because
// renderstamp.go QUOTES a header in its doc comment and renders nothing.
//
// A FLOOR, and WallRenderSources says so where it is declared: a render site
// that carries no such header is invisible to this pin, which is why nothing
// keys off the list. The reading always reports the WHOLE lag and always
// names `make install`; the wall count only says how loud to read it.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// wrsRenderMarker is the phrase a rendered file's own header carries. Lower
// case, matched case-insensitively: cageinner.go writes "Rendered at launch
// … do not edit." and seatbelt.go writes ";; … do not edit (rangerhq-5vt)".
const wrsRenderMarker = "do not edit"

// wrsStampFunc is the one writer of the header clause. The file that DEFINES
// it is exempt from carrying a call to it, for the obvious reason.
const wrsStampFunc = "RenderedByPosse"

// wrsStampDefiner is where wrsStampFunc lives, relative to the repo root.
const wrsStampDefiner = "internal/posse/renderstamp.go"

// wrsRenderSites parses every non-test Go file in internal/posse and returns,
// per file, whether its string literals claim "do not edit" and whether its
// body calls the stamp writer.
func wrsRenderSites(t *testing.T, root string) (renders, stamps map[string]bool) {
	t.Helper()
	dir := filepath.Join(root, "internal", "posse")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", dir, err)
	}
	renders, stamps = map[string]bool{}, map[string]bool{}
	fset := token.NewFileSet()
	seen := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		seen++
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", name, err)
		}
		rel := "internal/posse/" + name
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.BasicLit:
				if v.Kind != token.STRING {
					return true
				}
				// Unquote handles both interpreted and raw strings; an
				// unparseable literal is not a finding, it is a literal this
				// pin cannot read, and go/parser already accepted the file.
				if s, err := strconv.Unquote(v.Value); err == nil &&
					strings.Contains(strings.ToLower(s), wrsRenderMarker) {
					renders[rel] = true
				}
			case *ast.Ident:
				if v.Name == wrsStampFunc {
					stamps[rel] = true
				}
			}
			return true
		})
	}
	if seen < 50 {
		// The package is ~200 files. A handful means the walk found the
		// wrong directory, and every arm below would then pass over nothing.
		t.Fatalf("only %d non-test sources under %s — this pin is reading the wrong tree", seen, dir)
	}
	if len(renders) == 0 {
		t.Fatalf("no file under %s claims %q in a string literal — the marker this pin derives its class from is gone", dir, wrsRenderMarker)
	}
	return renders, stamps
}

// ARM 1: every derived render site is in WallRenderSources, so the count
// `posse promote` prints cannot silently omit a wall.
func TestWallRenderSourcesNameEveryRenderedDoNotEditSite(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	renders, _ := wrsRenderSites(t, root)
	listed := map[string]bool{}
	for _, p := range posse.WallRenderSources {
		listed[p] = true
	}
	for rel := range renders {
		if !listed[rel] {
			t.Errorf("%s renders a file whose header says %q, and posse.WallRenderSources does not name it — `posse promote` and `posse gates` would count a lag over this file as touching no wall, which is the silence ranger-base-vso72 closed. Add it to WallRenderSources (wallrenderer.go).", rel, wrsRenderMarker)
		}
	}
}

// ARM 2: every derived render site carries the renderer in its header, so a
// seat reading a stale wall can tell which binary wrote it.
func TestEveryRenderedDoNotEditSiteNamesItsRenderer(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	renders, stamps := wrsRenderSites(t, root)
	for rel := range renders {
		if rel == wrsStampDefiner || stamps[rel] {
			continue
		}
		t.Errorf("%s renders a file whose header says %q and never calls %s() — the header names the PID and the launch and not the binary, so a seat reading it cannot tell \"relaunch pending\" from \"install pending\" (ranger-base-mmhhc: twelve caged launches read exactly that sentence off a superseded wall). Put %s() in the header.", rel, wrsRenderMarker, wrsStampFunc, wrsStampFunc)
	}
}

// ARM 3: every listed path is a real file. A pathspec that matches nothing
// makes the wall count read zero over a repo where it should read more, and
// `git log -- <nothing>` says so to nobody.
func TestWallRenderSourcesAllExist(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if len(posse.WallRenderSources) == 0 {
		t.Fatal("posse.WallRenderSources is empty")
	}
	for _, p := range posse.WallRenderSources {
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil || st.IsDir() {
			t.Errorf("posse.WallRenderSources names %s, which is not a file here (%v) — a pathspec that matches nothing renders as \"no wall-rendering commit\"", p, err)
		}
	}
}
