package treepins

// QA pin for ranger-base-1l1en — every line posse starts a seat's RUNTIME
// with is exec'd, so no interactive shell outlives that runtime.
//
// THE RULE IS A PROPERTY OF ONE FUNCTION, and this census is what keeps it
// one. `App.PaneLine` (internal/posse/paneline.go) applies the exec, to the
// typed line and to the spilled script's body alike, and
// internal/posse/launchexec_qa_test.go pins what it produces over every
// built-in runtime and both launch paths. What a behavioural pin cannot see
// is a FIFTH caller of the slot's `Start` verb typing a launch of its own
// without going through it — green in every arm, and a seat whose pane comes
// back to a prompt when its runtime dies, which is the whole of
// ranger-base-ie4mx item 3.
//
// So the roster below is every non-test call of `Start(seat, line)` in the
// tree, each with what it types. A launch entry must name `PaneLine` in the
// same function; the one entry that is NOT a launch must not, and must not
// be exec'd either — probeSessionExe asks the pane which binary it resolves
// and then needs that same shell to run the launch line after it. A new call
// site reds this and the author says which of the two it is.
//
// ONE STEP BACK FROM THE CALL, the way muxseam_qa_test.go is and for its
// reason: no line says what a receiver's type is without a type checker, so
// the discriminator here is the SHAPE — two arguments. `exec.Cmd.Start()`
// takes none, which is every other `.Start(` in the tree (cagelauncher.go,
// codextrust.go, wallwatch_linux.go, worktree.go, cmd/coldexec).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// lxLaunch and lxLookup are what a roster entry claims about its call.
const (
	lxLaunch = "launch" // the runtime's own line: PaneLine renders it, exec'd
	lxLookup = "lookup" // a line typed INTO the pane's shell, which must stay
)

// lxStartSites is every non-test `Start(seat, line)` in the tree, keyed
// `<file>:<enclosing func>`, with its kind and the reason.
var lxStartSites = map[string]struct{ kind, why string }{
	"internal/posse/herdrback.go:startPlanned": {lxLaunch,
		"the create path: the command typed into a workspace's root pane an instant after herdr made it (ADR 0013 §1 property 3)."},

	"internal/posse/herdrback.go:RelaunchAgent": {lxLaunch,
		"the re-type path (rangerhq-vk2). Since this bead it is the one launch that types into a pane posse did NOT just create — and a pane that still reads a tty after its runtime died is the state exec removes, so what reaches here is a seat from before the change, or an occupant that keeps a dead pane open. It renders its own line and spills it through the same PaneLine."},

	"internal/posse/runtimeprobe.go:RuntimeProbe": {lxLaunch,
		"the runtime probe's own launch (ADR 0013 §4): a real launch of the same kind, measured rather than dry-rendered, so it takes the same line the fleet takes."},

	"internal/posse/runtimeprobe.go:probeSessionExe": {lxLookup,
		"NOT a launch: `command -v <exe> > <part>; mv <part> <file>` typed into the probe pane to record which binary that session resolves (ranger-base-385x). It is a compound line BY DESIGN, it is the pane's own shell that must answer it, and that same shell has to survive to run the launch line that follows — so this is the one Start the exec must not reach, and the reason the exec lives in PaneLine and not in the slot's Start."},
}

type lxSite struct {
	key, file string
	line      int
	panelined bool
}

// lxScan returns every two-argument `.Start(` call in the non-test Go
// sources under cmd/ and internal/, with the enclosing function and whether
// that function also calls PaneLine — and the number of files parsed, so a
// walk that read nothing cannot pass as a tree with no call sites.
func lxScan(t *testing.T) (sites []lxSite, parsed int) {
	t.Helper()
	fset := token.NewFileSet()
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n == ".git" || n == ".beads" || n == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(path)
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		parsed++
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			var starts []int
			panelined := false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "Start":
					if len(call.Args) == 2 {
						starts = append(starts, fset.Position(sel.Sel.Pos()).Line)
					}
				case "PaneLine":
					panelined = true
				}
				return true
			})
			for _, ln := range starts {
				sites = append(sites, lxSite{rel + ":" + fn.Name.Name, rel, ln, panelined})
			}
		}
		return nil
	}
	// Two roots, each SPELLED, for muxseam_qa_test.go's reason: a `range`
	// over a slice of them would hand WalkDir a variable, and this class's
	// membership is derived from an enumeration rooted at a literal
	// relative path (treewidedoor_qa_test.go's twdRelTreePath).
	if err := filepath.WalkDir("cmd", walk); err != nil {
		t.Fatalf("walking cmd for the launch sites: %v", err)
	}
	if err := filepath.WalkDir("internal", walk); err != nil {
		t.Fatalf("walking internal for the launch sites: %v", err)
	}
	return sites, parsed
}

// Arm 1: the roster is the tree. Every two-argument Start is on it, and
// every entry on it is still in the tree.
func TestQAEveryLineStartedInAPaneIsOnTheLaunchRoster(t *testing.T) {
	t.Parallel()
	sites, parsed := lxScan(t)
	if parsed < 100 {
		t.Fatalf("parsed only %d non-test Go files — the walk read almost nothing, so no arm here means anything", parsed)
	}
	if len(sites) == 0 {
		t.Fatal("found no two-argument Start call at all: either the slot's verb was renamed or this reader stopped reading, and both are silent")
	}
	seen := map[string]bool{}
	for _, s := range sites {
		seen[s.key] = true
		if _, ok := lxStartSites[s.key]; !ok {
			t.Errorf("%s:%d types a line into a pane and is not on the roster in this file.\n"+
				"If it starts a seat's RUNTIME, render its line through App.PaneLine so it is exec'd (ranger-base-1l1en) and add a %q entry.\n"+
				"If it types into the pane's own SHELL, say so with a %q entry and why that shell must survive.",
				s.file, s.line, lxLaunch, lxLookup)
		}
	}
	var gone []string
	for k := range lxStartSites {
		if !seen[k] {
			gone = append(gone, k)
		}
	}
	sort.Strings(gone)
	if len(gone) > 0 {
		t.Errorf("the roster names call sites the tree no longer has — delete the entries, do not leave a roster that is read as coverage: %s", strings.Join(gone, ", "))
	}
}

// Arm 2: a launch's line comes from PaneLine, which is where the exec is
// applied; a lookup's does not, because a lookup must leave a shell behind.
func TestQAALaunchLineIsRenderedByPaneLineAndALookupIsNot(t *testing.T) {
	t.Parallel()
	sites, _ := lxScan(t)
	checked := 0
	for _, s := range sites {
		e, ok := lxStartSites[s.key]
		if !ok {
			continue // arm 1 is what says so
		}
		checked++
		switch e.kind {
		case lxLaunch:
			if !s.panelined {
				t.Errorf("%s:%d starts a seat's runtime and its function never calls App.PaneLine — so that line is not exec'd, and the pane will come back to a shell prompt when the runtime dies (ranger-base-1l1en, ranger-base-ie4mx item 3).\nroster reason: %s", s.file, s.line, e.why)
			}
		case lxLookup:
			if s.panelined {
				t.Errorf("%s:%d is on the roster as a line typed into the pane's own shell, but its function renders a PaneLine too — one of the two claims is wrong, and the one that costs something is a lookup that exec'd the shell it needed.\nroster reason: %s", s.file, s.line, e.why)
			}
		default:
			t.Errorf("roster entry %s has kind %q, which is neither %q nor %q", s.key, e.kind, lxLaunch, lxLookup)
		}
	}
	if checked == 0 {
		t.Fatal("no roster entry was reached, so this arm asserted nothing")
	}
}
