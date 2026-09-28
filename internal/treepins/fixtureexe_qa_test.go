package treepins

// QA pin for ranger-base-mis0i — a fixture CLI is named for nothing real.
//
// THE DEFECT. A test whose subject is "a runtime that is not installed here"
// has to name that runtime, and the name is resolved on the BOX'S PATH, not
// on one the test controls. internal/posse/runtimeprobe.go's drift check is
// the shortest path to it — ProbeState resolves rt.Exe() and compares the
// answer against the recorded launcher path — but it is not the only one:
// RuntimeGaps' `exe` row does the same lookup, and probeSessionExe types
// `command -v <exe>` into a real subprocess that inherits the ambient PATH.
// So the fixture's premise is not a property of the fixture at all. It is a
// bet that the name stays free on every box the suite ever runs on.
//
// The bet was `bob`, from ADR 0017 onward. On 2026-09-28 14:31 an npm
// install of an unrelated tool called bobshell 2.0.5 put /opt/homebrew/bin/
// bob on this box, and five arm3 tests went red at a HEAD nobody had
// touched. A second seat isolated it to that one variable on the bead: same
// tree, same command, PATH with /opt/homebrew/bin FAIL and PATH without ok.
//
// What it cost is the point. None of the five failures named a collision.
// They said `the probe measured /usr/local/bin/bob and bob now resolves to
// /opt/homebrew/bin/bob — a different binary was measured; re-run posse
// runtime probe bob`, which reads as a defect in the probe's own drift
// logic, and one of them — a test asserting that a probe which cannot name
// its binary must REFUSE — read as posse failing to refuse. Two seats
// reproduced it before the cause was a package manager.
//
// THE RULE, and it is one word at each site: a fixture CLI that must not
// exist is spelled `posse-<bead>-no-such-exe`. The tree had that spelling
// before this bead (posse-tr8k-no-such-exe, posse-8vys9-no-such-exe, both
// in runtimepreflight_test.go) and it had two other spellings of the same
// intention beside it — `definitely-not-installed-anywhere-385x`, and the
// alice/bob/carol placeholder line, which is the one that lost. A
// convention three files keep and nothing checks is not a convention.
//
// WHAT THIS PIN ACTUALLY ASSERTS is the property, not the spelling: no
// fixture exe in the test corpus resolves on THIS box right now. That makes
// it deliberately box-dependent, which is the one design choice here worth
// defending. The defect is box-dependent; a pin that were not could only
// ever check a naming habit. This one reds on the day the collision arrives,
// wherever it arrives, in a package that runs in seconds — and its message
// names the exe, the file, the binary it collided with and the remedy. That
// is the whole distance between this and the afternoon the bead describes.
//
// WHAT BEHAVES DIFFERENTLY IF THIS TEST IS DELETED (ADR 0006 §7): the next
// collision is found the way this one was. Some number of tests in some arm
// go red, at a HEAD nobody touched, with messages about probe records and
// drift that name neither the fixture nor the install that broke it, and the
// seat that reads them starts by disbelieving its own tree.
//
// WHERE IT LIVES. internal/treepins, for gittempdirroot_qa_test.go's
// reasons: the subject is the repository's whole test corpus — most of it
// internal/posse, which is ~950s, past what a seat spends in one foreground
// call — and a pin nobody's `-run` filter names is unreachable exactly when
// it matters (ranger-base-rulbl's class). treepins is run whole by `go test ./...` in seconds, so this needs
// no Makefile door, and being outside internal/posse keeps it outside
// treewidedoor's derived class. Reading the SOURCE rather than building it
// is also what covers all three arms at once: parser.ParseFile applies no
// build constraints, so the untagged, posse_arm2 and posse_arm3 files are
// all read by one run.
//
// NOT CLAIMED: that every name this scan collects is one a lookup really
// reaches. Detection is over-broad on purpose — a `command:` line is read as
// a fixture CLI wherever it appears, including in cage templates, which are
// not runtimes at all. An exe this over-counts costs one rename; an exe it
// under-counts is this bead again. The resolution is over-broad the same
// way: exec.LookPath reads the whole PATH, while the production lookup this
// stands in for (resolveOutside) drops the gates dirs first, so a name that
// resolved ONLY to a gate shim would red here and be fine in a launch. No
// fixture in the corpus is spelled like a gated command today, and the
// remedy for one that were is the same rename.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// fxeSkip are directories the walk does not descend into. Not a corpus
// choice — the corpus is every *_test.go the repository tracks, walked
// rather than listed so that a package growing runtime fixtures tomorrow is
// scanned without anyone remembering to add it here.
var fxeSkip = map[string]bool{".git": true, "vendor": true, "node_modules": true}

// fxeReal are the words a fixture names on purpose because it wants the real
// binary. Each one is a reason, not an exemption: nothing here is a runtime
// the test claims is absent.
//
// It is a map and not a list so that adding a row costs writing the reason
// down, which is the only thing standing between this and an allowlist that
// grows whenever the pin is inconvenient.
var fxeReal = map[string]string{
	// The four built-in runtimes. A test naming one is about a runtime that
	// legitimately exists — posse ships its template and an onboarder is
	// expected to have the CLI. `bob` is here as of ADR 0060, which is the
	// same fact that ended its career as the placeholder.
	"claude": "built-in runtime (ADR 0017)",
	"codex":  "built-in runtime (ADR 0017)",
	"grok":   "built-in runtime (ADR 0017)",
	"bob":    "built-in runtime (ADR 0060, ranger-base-ymmiv)",
	// Cage templates, whose command: really is a program the test runs.
	// cages/*.yaml is not a runtime: the word is the engine, and these two
	// are chosen because every box has them.
	"env":  "cage template engine (cage_test.go, cageinner_test.go)",
	"echo": "cage template engine (cage_test.go's routeless arm)",
	// cmd/posse/runtimeprobe_qa_test.go's `shell` runtime. Its subject is
	// that `runtime probe` is ROUTED and parses its flags, and it refuses on
	// a missing herdr long before anything resolves the exe — so naming a
	// program every box has is not a bet, it is the opposite of one.
	"sh": "POSIX shell, named on purpose (cmd/posse/runtimeprobe_qa_test.go)",
}

// fxeReserved is the spelling the remedy names. `posse-` scopes it to this
// project and `-no-such-exe` says what it is for; the bead in the middle
// says who needed it, which is how the two that predate this pin read.
const fxeReserved = "posse-<bead>-no-such-exe"

// fxeCandidate is one fixture CLI word and where it was written.
type fxeCandidate struct {
	exe   string
	where string
}

// fxeFlatten renders a string expression to the text it will hold at run
// time, substituting package-level string constants (which is how a fixture
// that already obeys this rule is written — `"command: "+probeFixtureExe+…`).
// A part it cannot read becomes a NUL, so the words either side of it stay
// separate and the unreadable part cannot be mistaken for a name.
func fxeFlatten(e ast.Expr, consts map[string]string) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "\x00"
		}
		s, err := strconv.Unquote(v.Value)
		if err != nil {
			return "\x00"
		}
		return s
	case *ast.Ident:
		if s, ok := consts[v.Name]; ok {
			return s
		}
		return "\x00"
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			return fxeFlatten(v.X, consts) + fxeFlatten(v.Y, consts)
		}
	case *ast.ParenExpr:
		return fxeFlatten(v.X, consts)
	}
	return "\x00"
}

// fxeExeOf takes the command word out of a rendered template line: the first
// field after `command:`, anchored to the START of a line so that prose
// mentioning the word ("the whole point of the command: every gap by name")
// is not read as a declaration.
func fxeExeOf(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimLeft(line, " \t")
		const key = "command:"
		if !strings.HasPrefix(line, key) {
			continue
		}
		if f := strings.Fields(line[len(key):]); len(f) > 0 {
			out = append(out, f[0])
		}
	}
	return out
}

// fxeLookable filters to words a PATH search can actually be asked about. A
// placeholder, a path, a shell fragment and an unreadable concatenation are
// all "not a bare command name", and none of them is what this pin is about.
func fxeLookable(w string) bool {
	if w == "" || strings.ContainsAny(w, "\x00{}$\"'`\\%*|<>;&()[]") {
		return false
	}
	if strings.ContainsRune(w, filepath.Separator) {
		// An absolute or relative path names one file, not a PATH lookup.
		return false
	}
	return true
}

func fxeCollect(t *testing.T) []fxeCandidate {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if fxeSkip[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", path, perr)
		}
		files = append(files, f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// A corpus that silently reads zero files is a green pin watching
	// nothing — the same silence this file exists to end.
	if len(files) == 0 {
		t.Fatal("the walk found no *_test.go at all — the scan's corpus is empty")
	}

	// Package-level string constants first, across every file, so a const
	// declared in one arm's file is readable from another's.
	consts := map[string]string{}
	for _, f := range files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, sp := range gd.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, id := range vs.Names {
					if i < len(vs.Values) {
						if s := fxeFlatten(vs.Values[i], nil); !strings.Contains(s, "\x00") {
							consts[id.Name] = s
						}
					}
				}
			}
		}
	}

	seen := map[string]bool{}
	var out []fxeCandidate
	add := func(exe string, pos token.Pos) {
		if !fxeLookable(exe) || seen[exe] {
			return
		}
		seen[exe] = true
		p := fset.Position(pos)
		out = append(out, fxeCandidate{exe: exe, where: fmt.Sprintf("%s:%d", p.Filename, p.Line)})
	}

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			// Shape 1: a rendered yaml template, anywhere a string is
			// built. Every level of a concatenation is read, not just the
			// outermost: flattening `"command: carol" + someCall()` yields
			// `command: carol\x00`, whose first field carries the NUL and
			// is discarded as unreadable — while the inner literal on its
			// own reads cleanly as `carol`. Visiting both is what makes an
			// unreadable neighbour cost nothing, and `seen` makes the
			// overlap free.
			case *ast.BasicLit, *ast.BinaryExpr:
				e := n.(ast.Expr)
				for _, exe := range fxeExeOf(fxeFlatten(e, consts)) {
					add(exe, e.Pos())
				}
			// Shape 2: a Runtime/Cage struct built in Go rather than yaml.
			case *ast.KeyValueExpr:
				if id, ok := v.Key.(*ast.Ident); ok && id.Name == "Command" {
					if f := strings.Fields(fxeFlatten(v.Value, consts)); len(f) > 0 {
						add(f[0], v.Value.Pos())
					}
				}
			}
			return true
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].exe < out[j].exe })
	return out
}

// The pin. Every fixture CLI the test corpus names either is a word the tree
// deliberately wants to be real (fxeReal, with its reason) or resolves to
// nothing on this box.
func TestQAFixtureRuntimeExesResolveToNothingOnThisBox(t *testing.T) {
	t.Parallel()
	cands := fxeCollect(t)
	// The scan finding nothing at all would be a green pin over an empty
	// set — the failure mode this whole file is about, reached by a
	// refactor that changes how fixtures are spelled rather than by an
	// install.
	if len(cands) < 20 {
		t.Fatalf("the scan collected only %d fixture exes; it collected 55 when this pin was written (MEASURED 2026-09-28, ranger-base-mis0i), so the extraction has stopped matching how fixtures are written: %+v", len(cands), cands)
	}

	var bad []string
	for _, c := range cands {
		if _, ok := fxeReal[c.exe]; ok {
			continue
		}
		path, err := exec.LookPath(c.exe)
		if err != nil {
			continue
		}
		bad = append(bad, fmt.Sprintf("  %s (%s) resolves to %s", c.exe, c.where, path))
	}
	if len(bad) > 0 {
		t.Errorf(`%d fixture CLI name(s) in the test corpus resolve on this box:
%s

A test whose subject is a runtime that is NOT installed here must not name it
with a word a package manager can ship. Nothing is wrong with the binary that
arrived — the fixture name was a bet that it never would.

Fix it at the fixture, not here: rename the exe to the reserved form
%s (the runtime's NAME can stay what it is — nothing ever
resolves a name, only the first word of its command:). If a fixture above
really does mean the installed binary, add it to fxeReal WITH its reason.

This is ranger-base-mis0i, which is the same defect as an npm install of
bobshell reddening five arm3 probe tests at an untouched HEAD.`,
			len(bad), strings.Join(bad, "\n"), fxeReserved)
	}
}
