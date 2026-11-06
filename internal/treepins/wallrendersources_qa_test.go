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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
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

// wrsRenderPrefix is the naming convention of a func that returns rendered
// text. It is what the body-renderer half of the class follows.
const wrsRenderPrefix = "render"

// THE BODY RENDERERS (ranger-base-jk380). A header is written once, by the
// file that writes the wall, but the wall's BODY can be spliced in from
// elsewhere: renderShim in gates.go writes the git shim's header and calls
// renderHookSkipArm and renderHookSkipEnvCall, declared in hookskip.go. That
// file says `do not edit` nowhere, so the marker alone never derived it, and
// `posse gates` read "0 of them rendering a wall" over three commits that
// changed only hookskip.go — one closing a hole live on every installed seat.
//
// So the class is closed over calls: start from every func declared in a
// headered file, follow each call whose callee is named `render*`, and every
// file declaring such a callee is a body renderer; the walk continues through
// that callee's own `render*` calls. By NAME, not by type: a method and a
// func sharing a name both count, which can only over-derive, never miss.
// The residual is a helper spliced into a wall whose name is not `render*`
// (a quoting helper, say) — still a floor, and WallRenderSources says so.

// wrsRenderSites parses every non-test Go file in internal/posse and returns,
// per file, whether its string literals claim "do not edit", whether its body
// calls the stamp writer, and whether it is a body renderer reached from a
// headered file (see above; never a file already in renders).
func wrsRenderSites(t *testing.T, root string) (renders, stamps, bodies map[string]bool) {
	t.Helper()
	dir := filepath.Join(root, "internal", "posse")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", dir, err)
	}
	renders, stamps, bodies = map[string]bool{}, map[string]bool{}, map[string]bool{}
	type renderFunc struct {
		file    string
		callees []string // the `render*` names its body calls
	}
	decls := map[string][]renderFunc{} // func or method name -> declarations
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
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			rf := renderFunc{file: rel}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var name string
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				}
				if strings.HasPrefix(name, wrsRenderPrefix) {
					rf.callees = append(rf.callees, name)
				}
				return true
			})
			decls[fd.Name.Name] = append(decls[fd.Name.Name], rf)
		}
	}
	if seen < 50 {
		// The package is ~200 files. A handful means the walk found the
		// wrong directory, and every arm below would then pass over nothing.
		t.Fatalf("only %d non-test sources under %s — this pin is reading the wrong tree", seen, dir)
	}
	if len(renders) == 0 {
		t.Fatalf("no file under %s claims %q in a string literal — the marker this pin derives its class from is gone", dir, wrsRenderMarker)
	}
	var queue []string
	for _, ds := range decls {
		for _, d := range ds {
			if renders[d.file] {
				queue = append(queue, d.callees...)
			}
		}
	}
	visited := map[string]bool{}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if visited[name] {
			continue
		}
		visited[name] = true
		for _, d := range decls[name] {
			if !renders[d.file] {
				bodies[d.file] = true
			}
			queue = append(queue, d.callees...)
		}
	}
	return renders, stamps, bodies
}

// wrsUnlisted returns, sorted, every derived render site (headered or body)
// that list does not name.
func wrsUnlisted(renders, bodies map[string]bool, list []string) []string {
	listed := map[string]bool{}
	for _, p := range list {
		listed[p] = true
	}
	var missing []string
	for _, set := range []map[string]bool{renders, bodies} {
		for rel := range set {
			if !listed[rel] {
				missing = append(missing, rel)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

// ARM 1: every derived render site — headered or body — is in
// WallRenderSources, so the count `posse promote` prints cannot silently omit
// a wall.
func TestWallRenderSourcesNameEveryRenderedDoNotEditSite(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	renders, _, bodies := wrsRenderSites(t, root)
	for _, rel := range wrsUnlisted(renders, bodies, posse.WallRenderSources) {
		why := fmt.Sprintf("renders a file whose header says %q", wrsRenderMarker)
		if bodies[rel] {
			why = fmt.Sprintf("declares a %s* func a headered render site splices into its wall", wrsRenderPrefix)
		}
		t.Errorf("%s %s, and posse.WallRenderSources does not name it — `posse promote` and `posse gates` would count a lag over this file as touching no wall, which is the silence ranger-base-vso72 closed (and ranger-base-jk380 found again for a body renderer). Add it to WallRenderSources (wallrenderer.go).", rel, why)
	}

	// The derivation's own witness (ranger-base-jk380): hookskip.go carries
	// no header and renders the git shim's hook-skip arm, so a copy of the
	// list without it must red. If this fails the body-renderer half has
	// stopped seeing the file whose omission it exists for.
	const body = "internal/posse/hookskip.go"
	var without []string
	for _, p := range posse.WallRenderSources {
		if p != body {
			without = append(without, p)
		}
	}
	if renders[body] {
		t.Fatalf("%s now says %q itself — this witness needs a body renderer that does not; pick one from the derived bodies %v", body, wrsRenderMarker, bodies)
	}
	if got := wrsUnlisted(renders, bodies, without); len(got) != 1 || got[0] != body {
		t.Errorf("WallRenderSources minus %s should be missing exactly %s, derived as a body renderer; the derivation reports missing %v (bodies derived: %v)", body, body, got, bodies)
	}
}

// ARM 2: every derived render site carries the renderer in its header, so a
// seat reading a stale wall can tell which binary wrote it.
func TestEveryRenderedDoNotEditSiteNamesItsRenderer(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	renders, stamps, _ := wrsRenderSites(t, root)
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
