package treepins

// QA pin for ranger-base-csnzn — the multiplexer slot's boundary holds.
//
// ADR 0076 §1 put the seam at the CLIENT: posse drives a seat through the ten
// verbs of `Mux` (internal/posse/mux.go), herdr's occupant of that slot wraps
// the `Herdr` client (muxherdr.go), and verbs only herdr has are reached by a
// type assertion on the occupant and never through the interface (§4). The
// extraction itself was one bead. Keeping it is this pin.
//
// WHY A PIN AND NOT THE PROBE. The ADR ships its own census —
// `sh docs/adr/0076-multiplexer-slot.probe.sh`, part 1 — and that is what
// MEASURED the seam before and after: fourteen files calling the client
// directly, down to four. But it is a shell script a seat has to remember to
// type, and it matches receivers by their SPELLING (`.H`, `h`, `herdr*`), so
// it counts a method on the occupant as a call on the client whenever the
// local happens to be named `h`, and misses a client call through a local
// named anything else. ADR 0075 §2 already measured what an unenforced seam
// costs while it waits: the surface widened from 13 files to 21. The second
// occupant arrives behind this boundary, so the boundary needs a reader that
// cannot be forgotten and cannot be fooled by a variable name.
//
// THE RULE IS ONE STEP BACK FROM THE CALL, for the reason bddbflag_qa_test.go
// gives for its own Go arm: a call is built from a value, and no line says
// what a value's type is without a type checker. So this asks which files can
// HOLD a `Herdr` at all. There are exactly three routes to one —
//
//	the type name `Herdr`     a composite literal, a field, a parameter,
//	                          a method receiver, a conversion
//	`NewHerdr`                the constructor, in either spelling
//	                          (`NewHerdr(…)` or `posse.NewHerdr(…)`)
//	the selector `.H`         the one field either holder exposes
//	                          (HerdrBackend.H, HerdrMux.H)
//
// — and a file that names none of them cannot reach a client verb, whatever
// it calls its locals. Over-approximate on purpose: naming `Herdr` in a
// signature is not calling it, so a roster entry is a claim about a FILE and
// not about a line, which is the grain a seam is kept at.
//
// Parsed identifiers, never file bytes. `HerdrMux`, `HerdrBackend`,
// `HerdrWorkspace` and `NewHerdrMux` are their own identifiers and are not
// hits, and a comment about the client — this file is mostly that — is not a
// hit either. The byte census that would red on its own explanation is the
// mistake bddbflag's head comment already names.
//
// WHAT IS OUT OF SCOPE, said out loud. Test sources are not read. Tests build
// the bare client on purpose and must keep being able to: `b.H =
// Herdr{Bin: …}` is how a seat-phantom pin points a backend at a herdr that
// is not there, and the client's own tests are the only place its argv and
// its child deadline are exercised. A test that reaches past the slot is
// therefore not caught here; what catches a PRODUCTION path doing it is this
// census, and that is the path a second occupant runs.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// muxClientHolders is every non-test Go file that may hold a `Herdr`, with
// the reason it may. A new entry is a claim that the file is part of herdr's
// own occupant of the slot — not that it found the seam inconvenient.
var muxClientHolders = map[string]string{
	"internal/posse/herdr.go": "the client itself: every herdr invocation, the API envelope, and the child deadline (ranger-base-wj7e9) that no call site may forget.",

	"internal/posse/reportedagent.go": "the report-agent route, which ADR 0076 §4 keeps herdr's. It is the client's own internals: AgentPrompt chooses it for a pane herdr LABELS but does not detect, so it cannot be expressed as the slot's Prompt and cannot be reached from outside the client.",

	"internal/posse/muxherdr.go": "herdr's occupant of the slot (ADR 0076 §1): the ten verbs over the client, plus the §4 surface that only herdr has. This is the file the seam exists to create.",

	"internal/posse/herdrback.go": "the backend holds the client in its H field and builds the occupant from it (HerdrBackend.Mux, and HerdrBackend.herdrMux for §4's own doors). ADR 0076 §1 keeps the type name for a later, mechanical bead, and the field for every caller — tests included — that pointed a backend at a particular herdr before the slot existed. One read of it in non-test code, and this pin's arm 2 is what says so.",
}

// muxSeamSkipDir names the directories the walk does not enter.
func muxSeamSkipDir(name string) bool {
	return name == ".git" || name == ".beads" || name == "node_modules"
}

type muxSeamHit struct {
	file string
	line int
	what string
}

// muxSeamScan returns every route to a `Herdr` value in the non-test Go
// sources under cmd/ and internal/, and the number of files parsed — so a
// walk that read nothing cannot pass as a tree with no hits.
func muxSeamScan(t *testing.T) (hits []muxSeamHit, parsed int) {
	t.Helper()
	fset := token.NewFileSet()
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if muxSeamSkipDir(d.Name()) {
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
		var visit func(n ast.Node) bool
		visit = func(n ast.Node) bool {
			at := func(pos token.Pos, what string) {
				hits = append(hits, muxSeamHit{rel, fset.Position(pos).Line, what})
			}
			switch x := n.(type) {
			case *ast.SelectorExpr:
				// The qualified type, field and constructor are named
				// here. The receiver is then walked by this same visitor
				// and the selected NAME is not, so `posse.NewHerdr` is
				// one hit rather than two.
				switch x.Sel.Name {
				case "Herdr":
					// The type, qualified from another package, and ONLY
					// that: `r.Herdr` is a field read of the readings-log
					// record, and no line says which of the two a
					// selector is without a type checker. The client
					// lives in package posse, so the qualified type use
					// has exactly one spelling.
					if id, ok := x.X.(*ast.Ident); ok && id.Name == "posse" {
						at(x.Sel.Pos(), "the type name "+muxSeamExpr(x))
					}
				case "H":
					at(x.Sel.Pos(), "the client field "+muxSeamExpr(x))
				case "NewHerdr":
					at(x.Sel.Pos(), "the constructor "+muxSeamExpr(x))
				}
				ast.Inspect(x.X, visit)
				return false
			case *ast.Field:
				// A field's NAMES are not type uses, and this tree has a
				// `Herdr` field of its own: ReadingEvidence sits on
				// `Reading.Herdr` (readingslog.go, ADR 0066 D1). Walk the
				// type and skip the names.
				ast.Inspect(x.Type, visit)
				return false
			case *ast.KeyValueExpr:
				// Same field name on the other side of the colon, in
				// `Reading{Herdr: ev}`. A key that is a bare identifier is
				// a field name and never an expression that could build a
				// client; anything else is walked.
				if _, bare := x.Key.(*ast.Ident); !bare {
					ast.Inspect(x.Key, visit)
				}
				ast.Inspect(x.Value, visit)
				return false
			case *ast.Ident:
				switch x.Name {
				case "Herdr":
					at(x.Pos(), "the type name Herdr")
				case "NewHerdr":
					at(x.Pos(), "the constructor NewHerdr")
				}
			}
			return true
		}
		ast.Inspect(f, visit)
		return nil
	}
	// Two roots, each SPELLED. A `range` over a slice of them would hand
	// WalkDir a variable, and this class's membership is derived from an
	// enumeration rooted at a literal relative path — so a loop here would
	// take the pin out of the class its door is in
	// (treewidedoor_qa_test.go's twdRelTreePath).
	if err := filepath.WalkDir("cmd", walk); err != nil {
		t.Fatalf("walking cmd for the multiplexer seam: %v", err)
	}
	if err := filepath.WalkDir("internal", walk); err != nil {
		t.Fatalf("walking internal for the multiplexer seam: %v", err)
	}
	return hits, parsed
}

// muxSeamExpr renders a selector as its source spelling, for a failure line
// that names what it found rather than only where.
func muxSeamExpr(x *ast.SelectorExpr) string {
	if id, ok := x.X.(*ast.Ident); ok {
		return id.Name + "." + x.Sel.Name
	}
	return "….." + x.Sel.Name
}

// Arm 1: no production file outside the roster reaches the herdr client.
func TestQATheMultiplexerSlotIsTheOnlyRouteToTheHerdrClient(t *testing.T) {
	t.Parallel()
	hits, parsed := muxSeamScan(t)
	if parsed < 100 {
		t.Fatalf("parsed only %d non-test Go files under cmd/ and internal/ — the walk read almost nothing, so an empty result says nothing about the seam", parsed)
	}
	byFile := map[string][]muxSeamHit{}
	for _, h := range hits {
		if _, ok := muxClientHolders[h.file]; ok {
			continue
		}
		byFile[h.file] = append(byFile[h.file], h)
	}
	if len(byFile) == 0 {
		return
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		var where []string
		for _, h := range byFile[f] {
			where = append(where, f+":"+strconv.Itoa(h.line)+" "+h.what)
		}
		t.Errorf("%s reaches the herdr client directly, past the multiplexer slot (ADR 0076 §1):\n  %s\n"+
			"  Drive the seat through the ten verbs — b.Mux() / HerdrBackend.Mux(), internal/posse/mux.go — or, for a verb only herdr has (§4), through the door that takes the type assertion: b.explain, b.workspaces, b.workspaceGet, b.manifests, b.paneAgentSession, b.paneReadDetection.\n"+
			"  If the file genuinely belongs to herdr's occupant, add it to muxClientHolders with the reason.",
			f, strings.Join(where, "\n  "))
	}
}

// Arm 2: every roster entry still names a real file that still holds the
// client. A roster that outlives its reason is a hole nobody can see — the
// same rule the ADR exemptions pin applies to its own citations.
func TestQAEveryHerdrClientHolderStillHoldsIt(t *testing.T) {
	t.Parallel()
	hits, _ := muxSeamScan(t)
	holds := map[string]bool{}
	for _, h := range hits {
		holds[h.file] = true
	}
	names := make([]string, 0, len(muxClientHolders))
	for f := range muxClientHolders {
		names = append(names, f)
	}
	sort.Strings(names)
	for _, f := range names {
		if !holds[f] {
			t.Errorf("muxClientHolders names %s and that file does not hold the herdr client any more — drop the row, so the roster keeps saying what it is for", f)
		}
	}
}
