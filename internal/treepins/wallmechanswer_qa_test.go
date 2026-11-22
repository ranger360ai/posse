package treepins

// QA pins for ranger-base-akwrv — an arm that ships the wall check and builds
// its own landedChecker answers ADR 0097 D2's SECOND question itself.
//
// THE CLASS. `wall` asks three questions in order (wallcheck.go's header), and
// the second — is this host's wall a bwrap — is a property of the BOX and not
// of the step. At sandbox-exec the step DOES NOT EXIST (ADR 0097 D2, ADR 0096:
// no darwin change), so an arm whose subject is what the venue does has to
// answer that question for itself or it measures the one line that declines
// it. internal/posse has three seams for the answer: the checker's own
// `wallMechFn` field (landedcheck.go, which states the trade at the field),
// the package-level `wallMechanismForTest`, and the host's own
// `WallMechanism()` read behind a skip, which is how the live pin does it
// (wallchecklive_test.go). An arm that answers through none of them is GREEN
// on every ubuntu leg of ci.yml and RED on every macos one, and the suite on
// this box cannot tell the difference.
//
// THAT HAS HAPPENED TWICE, both times found by a CI leg and a bead rather
// than by a test:
//
//   - ranger-base-jyqk4, the first wave: the whole of ADR 0097 D5's table,
//     red on macos-latest arm 2 from the day the step landed (MEASURED at
//     edfe7a79 through acdaf81f). Fixed by putting the line in `wckFixture`
//     (internal/posse/wallcheck_test.go) rather than in each arm — "so an arm
//     written tomorrow cannot forget it", which holds for every arm written
//     in that fixture's own file and for no other.
//   - ranger-base-zbmxc, the residual one file over: an arm written in
//     ANOTHER FILE forgot it.
//     TestQATheVenueCallSiteDiffersFromTheJudgesInTheKindAndTheChildOnly
//     (internal/posse/wallvenueargv_qa_test.go) built its checker with
//     newLandedChecker directly and read "1 cage(s), want 2" over a judge
//     that had answered green — found by reading macos arm 1 at b2509fba,
//     fixed with one line.
//
// A fixture's comment cannot hold a class whose members are not in its body,
// which is what this file is for.
//
// THE RULE. A census over internal/posse's test sources. A function that
// BUILDS its own checker (`newLandedChecker`, in its own body) and NAMES the
// wall check (`shipWallCheck`, the `wallCheckScript` const, or that const's
// path as a literal) must answer D2's second question through one of the
// three seams, in its own body. Functions and not tests: `wckFixture` is a
// helper and is a member, and the pin that could not see it would be the pin
// jyqk4 already wrote in prose.
//
// THE SHIP SIDE CLOSES OVER THE PACKAGE'S OWN NAMES, and that is
// ranger-base-72t07's finding against the first cut of this pin
// (ranger-base-b8igm). The ship half used to be read one body at a time, so
// the most ordinary refactor there is emptied the class with the suite green:
// `shipWallCheck` has exactly ONE call site in the tree, inside `wckFixture`,
// and extracting it — `func wckShip(t, repo) { shipWallCheck(t, repo, …) }` —
// leaves the test body `builds && !ships` and the helper `ships && !builds`,
// so NEITHER is a member and nobody is asked the question. A package var
// holding the const walks through the same gap. So `ships` is now the
// fixpoint over reference edges: a body ships if it names the wall check
// itself, or reads any declaration of this package's test sources that does,
// at any depth. Both sibling pins that landed in the same batch needed the
// same closure and measured why: ranger-base-u5ez8 computes a fixpoint over
// parameter indices because 2 of its 3 historical escapes put the open a
// frame down, and ranger-base-jhmm7 reaches the home three hops through a
// fixture.
//
// THE ANSWER SIDE DOES NOT CLOSE, DELIBERATELY, and the reason is the
// DIRECTION each mistake fails in. An answer inherited from a callee would
// make this census read green over a member whose seam it had resolved by
// name — silently, which is the whole defect this file is about. An answer
// demanded in the member's own body can only cost a RED, over a body that
// pins the mechanism one frame down; that red names the body and the fix is
// the same one line both historical fixes were. A pin that must choose
// between a false green and a false red here takes the red. Arm 3's
// "answers one frame down" row holds that choice so it cannot drift into an
// accident.
//
// THE BUILD SIDE DOES NOT CLOSE EITHER, and that one IS a false green —
// priced, measured, and held by a row rather than left to be rediscovered
// (ranger-base-8752f, from ranger-base-udagn's verify of this close).
// `builds` is set by a `newLandedChecker` call in the body itself, so the
// MIRROR of the refactor the ship fixpoint was built for walks through:
// extracting the BUILDER — `func zzBuild(t, repo) *landedChecker { return
// newLandedChecker(t, repo) }` — leaves the test `ships && !builds` and the
// helper `builds && !ships`, and nobody is asked D2's second question.
// MEASURED 2026-11-22 at 175e7c3b by planting that pair into internal/posse:
// green, no member added, against the close's own shape (a) — the SHIP one
// frame down — which reds and names the caller and its chain.
//
// CLOSING IT WAS TRIED AND PRICED, at 330d6940 the same day, and the arm
// below is the reading that stopped it. The closure cannot be the ship side's:
// `wckFixture` builds a checker for some thirty arms and the whole point of it
// is that it answers for them (jyqk4's fix), so a closure that read every
// caller of a builder as a builder reds all thirty. Narrowed to carry up only
// from a builder that SETTLES NOTHING — answers nowhere, reaches no wall check
// — it adds exactly one member to today's tree:
// TestEveryVerdictLineNamesTheLauncherThatRanTheCheck
// (internal/posse/landedlauncher_test.go:73), which ships through wckFixture
// in three arms and inherits an unanswered checker through
// landedCheckFixture in three others. That body is CORRECT on macos today —
// its wall arms take wckFixture and get its `wallMechFn` — so the finding is
// a false red, and the "cost of answering where it was not needed is one
// line" this file prices elsewhere does not hold for it: that test is
// `t.Parallel()` with parallel subtests, the package seam writes a
// package-level var and is serial-only by its own comment
// (TestATreeThatShipsNoWallCheckIsLeftAloneEntirely says so), and the host
// seam is a skip that would cost the macos leg three landed verdict lines. So
// answering it means three `c.wallMechFn` lines inside arms that never touch
// the wall step. A false red at that price buys less than it costs, and the
// build side stays one body at a time, named here and in the list below and
// held by arm 3's "the BUILD one frame down" row.
//
// THE NAMING SIDE IS BROAD ON PURPOSE, and the tree's own third member is
// why. TestATreeThatShipsNoWallCheckIsLeftAloneEntirely
// (internal/posse/wallcheck_test.go) ships no wall check at all and names the
// const only inside a failure message — and it needs the answer exactly as
// much as the other two, because its claim is that the pass says nothing
// about the wall, and a step that declines for want of a bwrap says
// `wall check not run`. It answers through the package seam
// (`wallMechanismForTest(t, "linux")`), and the census counts it a member.
// So the question this pin asks a body is not "did you ship" but "could what
// you assert move with the host's wall mechanism", and naming the script is
// the readable proxy for that. The cost of answering where it was not needed
// is one line; the cost of not answering is a macos leg and a bead, twice
// over. What the census must NOT do is reach a body that reaches no mention
// of the script at all — the 30-odd arms that take `wckFixture` and get the
// answer from it, and the live pin, which ships nothing and skips on the real
// mechanism.
//
// A PARSE AND NOT A GREP, for two reasons a `grep -l` cannot have: a mention
// in a COMMENT is not a reading (arm 3 holds that), and the answer has to be
// in the member's OWN body rather than somewhere in the file — which is the
// whole difference between wckFixture's line and the file it was supposed to
// cover.
//
// EVERY TEST FILE IS PARSED, and the prefilter this pin shipped with is
// gone, because the closure above is what killed it: a relay frame between a
// member and the ship names neither `newLandedChecker` nor the script, so a
// census of the files that name the builder cannot see the edge it hangs on.
// The whole directory costs 0.20s to parse — 763 test files, 13.5MB, 7,251
// function bodies and 509,070 identifiers (MEASURED 2026-11-22, this box,
// go1.27.3) — and arm 1 runs in 0.3s, arm 2 in 1.1s. The enumeration is still
// the directory and not a list of names in this file, which is what makes
// this pin's reading the tree's.
//
// WHAT A GREEN HERE PROVES, exactly, and what it does not. This is a
// SYNTACTIC census over parsed test sources — no type checker, no build:
//
//   - Reference edges resolve by NAME, receiver and package qualifier
//     ignored, so a body reaches every declaration of these sources that
//     shares a name with something it reads. That OVER-approximates the ship
//     side: the set of members is bigger than the truth, so a green is
//     stronger than the call graph and a false member is a red that names
//     its chain (the finding prints it).
//   - Within a body, the reading is flow-INSENSITIVE and scope-blind: an
//     identifier that appears anywhere in the body is read, local shadowing
//     included. Same direction, same reason.
//   - A ship that spells no spelling anywhere is invisible — a path built
//     from fragments, read out of a fixture's field, or handed in from
//     another package. The complementary guard for that direction is the
//     macos legs of ci.yml, which is what found both escapes above.
//   - PRODUCTION sources are not read, which is not an omission: the checker
//     itself names the script in its own body, so an edge into it would make
//     every test that calls into the wall step a shipper and the class would
//     be the package.
//   - A declaration with no initializer carries no facts, so a ship parked
//     in a struct field, a map, a slice or a channel and named elsewhere is
//     invisible.
//   - The BUILD side is read one body at a time while the ship side closes,
//     so a checker built one frame down — a builder extracted into a helper,
//     the mirror of the refactor the ship fixpoint was written for — empties
//     the class with this green. That is the one FALSE GREEN on this list
//     rather than a safe over-approximation; "THE BUILD SIDE DOES NOT CLOSE
//     EITHER" above has the measurement and what closing it costs, and arm
//     3's "the BUILD one frame down" row is its teeth.
//
// GRADED AGAINST THE REAL TREE and not only against arm 3's fixtures
// (MEASURED 2026-11-22, this box, go1.27.3). Each shape planted in
// internal/posse as a file of its own — the census parses that directory and
// never builds it, so a plant has only to parse — and arm 1 run over the
// tree with the plant in it:
//
//	plant                                            arm 1
//	control, no plant                                green, 3 members
//	the ship one frame down (72t07's shape (a))      RED, names the caller
//	the const through a package var (shape (b))      RED, names the body
//	both of those, each answering in its own body    green
//	a brand-new fourth member answering nothing      RED, names it
//	the venue arm's own `c.wallMechFn =` blanked      RED, names it
//	the BUILD one frame down (8752f's mirror of (a)) green, NO member
//
// The first cut of this pin was green over the first two of those, which is
// ranger-base-72t07's finding; the next two are the close's own mutants,
// re-measured here so the widening is not resting on them. The LAST is the
// one false green this census keeps: "THE BUILD SIDE DOES NOT CLOSE EITHER"
// prices closing it and arm 3 holds the shape, so it cannot drift back into
// being nobody's decision.
//
// Its subject is the tree, so it lives here and is doored by
// `make treepins-check`, the package minus a priced table (ADR 0098 D1/D4) —
// which is the point of a complement door: a pin written today is doored the
// moment it is written.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The subject package, as a relative path from the repo root this package's
// tests run in (root_test.go).
const wmqPkgDir = "internal/posse"

// The four spellings the census reads, and the seam's own declaration site.
// Every one of them is a name in internal/posse: a rename there takes this
// census dark, so arm 1 asserts each is still findable rather than letting a
// census of nothing read green.
const (
	wmqBuilder     = "newLandedChecker"
	wmqShipper     = "shipWallCheck"
	wmqScript      = "wallCheckScript"
	wmqScriptPath  = "scripts/posse-wall-check"
	wmqCheckerSeam = "wallMechFn"
	wmqPackageSeam = "wallMechanismForTest"
	wmqHostSeam    = "WallMechanism"

	wmqSeamFile = "internal/posse/landedcheck.go"
	wmqSeamDecl = "wallMechFn func() string"
)

// wmqMemberFloor is what the class holds today, as a floor and not an
// equality: three members (wckFixture, the venue call-site arm, and the
// ships-no-wall-check arm that answers through the package seam), and the
// bead counted the first two. A floor of two, because a member may be
// legitimately deleted and the dark-census failure is held by the seam
// assertions above it, not by this number.
const wmqMemberFloor = 2

// wmqDecl is one top-level declaration of the subject package's test sources
// — a function, or a package-level var or const with an initializer — read
// for the facts this pin is about.
type wmqDecl struct {
	where  string // file:line, for a reader who has to go and look
	path   string // the file it was read from, for arm 2's mutant
	name   string // the declaration's name, receiver included
	kind   string // "func" or "var", for the finding's prose
	builds bool   // it builds a checker of its own, in its own body

	// shipsHere is "this body names the wall check itself"; ships is that
	// closed over reference edges, and shipVia is the chain it got there
	// through, empty when the body names it itself.
	shipsHere bool
	ships     bool
	shipVia   []string

	// refs is every name this body reads, deduped: the edges the ship side
	// closes over. Answers are NOT closed over (see the head comment).
	refs        []string
	answers     []string // the seams it answers D2's second question through
	answerLines []int    // and where, for arm 2's mutant
}

// member reports whether this body is in the class: it builds its own checker
// AND reaches the wall check. Both halves, because a body that takes its
// checker from wckFixture has the answer already and a body that never
// reaches the wall check cannot be measuring the step.
func (d *wmqDecl) member() bool { return d.ships && d.builds }

// wmqCallee is a call's function name — a bare identifier, or the selector's
// final name so a qualified `posse.WallMechanism()` reads the same as a local
// one.
func wmqCallee(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}

// wmqDeclName names a declaration the way a reader would have to search for
// it. Declarations with a RECEIVER are read too (twdParseDir's own lesson,
// ranger-base-b3w9x): a rig that builds a checker in a method is the natural
// shape for a rig, and a census that skipped receivers would be one rename
// away from reading nothing.
func wmqDeclName(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) == 1 {
		var recv strings.Builder
		if err := wmqTypeName(&recv, fn.Recv.List[0].Type); err == nil {
			return "(" + recv.String() + ")." + fn.Name.Name
		}
	}
	return fn.Name.Name
}

// wmqTypeName writes an expression's source-ish spelling, which is all a
// receiver name needs: `*landedChecker`, `wckRig`.
func wmqTypeName(w *strings.Builder, e ast.Expr) error {
	switch x := e.(type) {
	case *ast.Ident:
		w.WriteString(x.Name)
		return nil
	case *ast.StarExpr:
		w.WriteString("*")
		return wmqTypeName(w, x.X)
	case *ast.IndexExpr: // a generic receiver, Rig[T]
		return wmqTypeName(w, x.X)
	}
	return fmt.Errorf("receiver type %T has no spelling here", e)
}

// wmqRead fills one declaration's facts from the node that carries them — a
// function body, or a var's initializer.
func wmqRead(d *wmqDecl, fset *token.FileSet, node ast.Node) {
	seam := map[string]bool{}
	ref := map[string]bool{}
	answer := func(n ast.Node, s string) {
		d.answerLines = append(d.answerLines, fset.Position(n.Pos()).Line)
		if !seam[s] {
			seam[s] = true
			d.answers = append(d.answers, s)
		}
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			// The const, wherever it is used: a path argument, a wanted
			// string, a failure message. See the head comment on why the
			// naming side is broad.
			if x.Name == wmqScript {
				d.shipsHere = true
			}
			// And every name this body reads, which is the edge set the
			// ship side closes over. A call's callee is an Ident (or a
			// SelectorExpr's Sel, which Inspect visits too), so this
			// covers calls and plain reads of a package var alike.
			if !ref[x.Name] {
				ref[x.Name] = true
				d.refs = append(d.refs, x.Name)
			}
		case *ast.BasicLit:
			// And the path typed out, which is the spelling a body that
			// does not import the const would use.
			if x.Kind == token.STRING && strings.Contains(x.Value, wmqScriptPath) {
				d.shipsHere = true
			}
		case *ast.CallExpr:
			switch wmqCallee(x.Fun) {
			case wmqBuilder:
				d.builds = true
			case wmqShipper:
				d.shipsHere = true
			case wmqPackageSeam:
				answer(x, wmqPackageSeam+"()")
			case wmqHostSeam:
				answer(x, wmqHostSeam+"()")
			}
		case *ast.AssignStmt:
			// The checker's own seam, set or CLEARED: a body that assigns
			// nil has taken a position on D2's second question too — it is
			// the shape TestAHostWhoseWallIsNotABwrapRunsNoVenueAndCounts-
			// NoSilence uses, paired with the package seam, and "nil means
			// the real one" is a reading of the mechanism and not a
			// failure to make one.
			for _, lhs := range x.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == wmqCheckerSeam {
					answer(x, "the checker's own "+wmqCheckerSeam)
				}
			}
		}
		return true
	})
}

// wmqScan reads one file's declarations. The source is a parameter rather
// than a path so arm 2 can hand it a mutant of the real thing.
//
// Package-level vars and consts are declarations too, and that is the second
// half of ranger-base-72t07's finding: `var qa72t07script = wallCheckScript`
// read from a body that builds its own checker is the const reached through
// one hop, and a census of function bodies alone sees neither end of it.
func wmqScan(path, src string) ([]*wmqDecl, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, err
	}
	var out []*wmqDecl
	at := func(n ast.Node, name, kind string) *wmqDecl {
		return &wmqDecl{
			where: fmt.Sprintf("%s:%d", path, fset.Position(n.Pos()).Line),
			path:  path,
			name:  name,
			kind:  kind,
		}
	}
	for _, decl := range file.Decls {
		switch x := decl.(type) {
		case *ast.FuncDecl:
			if x.Body == nil {
				continue
			}
			d := at(x, wmqDeclName(x), "func")
			wmqRead(d, fset, x.Body)
			out = append(out, d)
		case *ast.GenDecl:
			if x.Tok != token.VAR && x.Tok != token.CONST {
				continue
			}
			for _, spec := range x.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue // no initializer, so no fact to read
					}
					d := at(name, name.Name, "var")
					wmqRead(d, fset, vs.Values[i])
					out = append(out, d)
				}
			}
		}
	}
	return out, nil
}

// wmqCorpus is every declaration of the subject package's test sources, with
// the ship side already closed over its reference edges.
type wmqCorpus struct {
	decls   []*wmqDecl
	files   int // test files read
	named   int // of them, the ones whose bytes name the builder
	parsed  int // files parsed (every one of them; the prefilter is gone)
	byName  map[string][]*wmqDecl
	members []*wmqDecl
}

// wmqReadDir parses every test file of the subject package and closes the
// ship side. `mutant` replaces a file's source by path, which is how arm 2
// puts a member's own answer back in the state zbmxc left it in — the whole
// corpus has to be rebuilt for that, because a member's ship may be several
// frames and several files away.
func wmqReadDir(t *testing.T, mutant map[string]string) *wmqCorpus {
	t.Helper()
	ents, err := os.ReadDir(wmqPkgDir)
	if err != nil {
		t.Fatalf("read %s: %v", wmqPkgDir, err)
	}
	c := &wmqCorpus{byName: map[string][]*wmqDecl{}}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		c.files++
		path := filepath.Join(wmqPkgDir, e.Name())
		src, ok := mutant[path]
		if !ok {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			src = string(b)
		}
		if strings.Contains(src, wmqBuilder) {
			c.named++
		}
		c.parsed++
		decls, err := wmqScan(path, src)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		c.add(decls)
	}
	c.closeShips()
	return c
}

// wmqFixture is the same corpus over one in-memory file and nothing else,
// which is what arm 3's shapes are: a row's verdict must not depend on what
// the tree happens to hold.
func wmqFixture(t *testing.T, src string) *wmqCorpus {
	t.Helper()
	decls, err := wmqScan("fixture_test.go", "package posse\n\n"+src+"\n")
	if err != nil {
		t.Fatalf("the fixture does not parse, so this row measures nothing: %v", err)
	}
	c := &wmqCorpus{byName: map[string][]*wmqDecl{}, files: 1, parsed: 1}
	c.add(decls)
	c.closeShips()
	return c
}

func (c *wmqCorpus) add(decls []*wmqDecl) {
	for _, d := range decls {
		d.ships = d.shipsHere
		c.decls = append(c.decls, d)
		c.byName[d.name] = append(c.byName[d.name], d)
	}
}

// closeShips propagates "reaches the wall check" along reference edges to a
// fixpoint, and records the chain each member got there through so a finding
// can print it. A fixpoint and not one hop: nothing stops the next refactor
// putting the ship three frames down, which is the lesson both sibling pins
// in this batch wrote down (ranger-base-u5ez8, ranger-base-jhmm7).
func (c *wmqCorpus) closeShips() {
	for changed := true; changed; {
		changed = false
		for _, d := range c.decls {
			if d.ships {
				continue
			}
			for _, name := range d.refs {
				for _, target := range c.byName[name] {
					if target == d || !target.ships {
						continue
					}
					d.ships = true
					d.shipVia = append([]string{name}, target.shipVia...)
					changed = true
					break
				}
				if d.ships {
					break
				}
			}
		}
	}
	for _, d := range c.decls {
		if d.member() {
			c.members = append(c.members, d)
		}
	}
}

// wmqVia is how a member reaches the wall check, for a reader who has to
// decide whether a finding is a real member or this census over-approximating
// a name.
func (d *wmqDecl) wmqVia() string {
	if len(d.shipVia) == 0 {
		return "in its own body"
	}
	return "through " + strings.Join(d.shipVia, " -> ")
}

// wmqFindings is every complaint ADR 0097 D2's second question makes about a
// corpus.
//
// A function and not inline, because arms 2 and 3 require it to be NON-empty
// over mutants and shapes — so the check arm 1 makes and the check the other
// two prove can fail are the same code and cannot drift apart. A closed-set
// check nobody has seen red in the direction it exists for is a comment
// (ranger-base-ietka).
func (c *wmqCorpus) wmqFindings() []string {
	var out []string
	for _, d := range c.members {
		if len(d.answers) > 0 {
			continue
		}
		out = append(out, fmt.Sprintf(
			"%s (%s, %s) names the wall check (%s) and builds its own checker with %s, and answers ADR 0097 D2's "+
				"SECOND question — is this host's wall a bwrap — through none of this package's three seams "+
				"(`c.%s = func() string { return WallBwrap }`, `%s(t, \"linux\")`, or a `%s()` read behind a "+
				"skip), in its own body. At sandbox-exec the wall step DOES NOT EXIST (ADR 0096: no darwin "+
				"change), so this body measures the one line that declines it: green on every ubuntu leg and "+
				"red on every macos one, which is how ranger-base-jyqk4 and ranger-base-zbmxc were both found "+
				"by a CI leg instead of by a suite. Arms that take wckFixture "+
				"(internal/posse/wallcheck_test.go) get the answer from it; an arm that builds its own checker "+
				"answers for itself, and an answer a frame down is not one this census can read (ranger-base-b8igm).",
			d.name, d.kind, d.where, d.wmqVia(), wmqBuilder, wmqCheckerSeam, wmqPackageSeam, wmqHostSeam))
	}
	return out
}

// wmqMemberLines is the member set as a reader wants to see it.
func (c *wmqCorpus) wmqMemberLines() []string {
	var out []string
	for _, d := range c.members {
		answers := strings.Join(d.answers, ", ")
		if answers == "" {
			answers = "NO ANSWER"
		}
		out = append(out, fmt.Sprintf("%s — ships %s — %s", d.name, d.wmqVia(), answers))
	}
	sort.Strings(out)
	return out
}

// wmqNames is the member names of a corpus, sorted, which is what arm 3's
// rows assert against.
func (c *wmqCorpus) wmqNames() []string {
	var out []string
	for _, d := range c.members {
		out = append(out, d.name)
	}
	sort.Strings(out)
	return out
}

// Arm 1: the class over the real tree, and the census is reading something.
//
// The four assertions in front of the findings are the dark-census half, and
// they are not decoration: every name this pin reads lives in another package
// and a rename there is a rename nobody would think to bring here. A census
// that finds no candidate file, no builder, no ship or a seam that is no
// longer declared has stopped asking the question, and a pin in that state
// prints `ok` forever — which is the exact shape of the hole this file closes,
// one layer up.
func TestQAEveryWallArmThatBuildsItsOwnCheckerAnswersTheMechanismQuestion(t *testing.T) {
	t.Parallel()
	c := wmqReadDir(t, nil)
	if c.files < 100 {
		t.Fatalf("%d test file(s) under %s — the census read the wrong directory", c.files, wmqPkgDir)
	}
	var builds, ships int
	for _, d := range c.decls {
		if d.builds {
			builds++
		}
		if d.shipsHere {
			ships++
		}
	}
	if builds == 0 {
		t.Fatalf("no declaration under %s builds a checker with %s, so this census has no member — the builder was renamed and the class is unread (ADR 0097 D2, ranger-base-akwrv)", wmqPkgDir, wmqBuilder)
	}
	if ships == 0 {
		t.Fatalf("no declaration under %s names the wall check (%s, %s, or %q), so every member of this class reaches nothing and the census reads green over all of them — the ship spellings were renamed (ranger-base-b8igm)", wmqPkgDir, wmqShipper, wmqScript, wmqScriptPath)
	}
	seam, err := os.ReadFile(wmqSeamFile)
	if err != nil {
		t.Fatalf("read %s: %v", wmqSeamFile, err)
	}
	if !strings.Contains(string(seam), wmqSeamDecl) {
		t.Fatalf("%s no longer declares `%s` — the seam this pin requires an arm to set is gone or renamed, so the rule is about nothing (ADR 0097 D2)", wmqSeamFile, wmqSeamDecl)
	}

	members := c.wmqMemberLines()
	if len(members) < wmqMemberFloor {
		t.Errorf("%d member(s) of the class, and this file's floor is %d — a census that finds nobody reads green whatever the tree does. The two members ranger-base-akwrv was filed over are wckFixture and TestQATheVenueCallSiteDiffersFromTheJudgesInTheKindAndTheChildOnly; if one was deleted on purpose, lower the floor in the same commit and say which bead:\n  %s",
			len(members), wmqMemberFloor, strings.Join(members, "\n  "))
	}
	for _, finding := range c.wmqFindings() {
		t.Error(finding)
	}
	t.Logf("%d test file(s) under %s, all parsed, %d of them naming %s; %d declaration(s) build a checker and %d name the wall check themselves; %d member(s):\n  %s",
		c.files, wmqPkgDir, c.named, wmqBuilder, builds, ships, len(members), strings.Join(members, "\n  "))
}

// Arm 2: the census reds on the regression it was filed for — every member's
// own answer, removed.
//
// The mutant is the tree's, not a fixture's: take each member's answer lines
// out of the real file and require the census to name that member. That is
// ranger-base-zbmxc's defect put back, member by member, for however many
// members the class grows — a hand-written mutant would be one body's and
// would go stale the day a fourth arrives.
//
// The whole corpus is rebuilt per member rather than the one file rescanned,
// because a member's ship can be several frames and several files away now
// (ranger-base-b8igm). 1.1s for today's three members, and the cost is one
// 0.20s parse of the directory each.
func TestQATheWallMechanismCensusRedsWhenAMembersAnswerGoes(t *testing.T) {
	t.Parallel()
	base := wmqReadDir(t, nil)
	mutated := 0
	for _, d := range base.members {
		if len(d.answerLines) == 0 {
			continue // arm 1 is already reporting this one
		}
		src, err := os.ReadFile(d.path)
		if err != nil {
			t.Fatalf("read %s: %v", d.path, err)
		}
		got := wmqReadDir(t, map[string]string{d.path: wmqBlank(string(src), d.answerLines)})
		var named bool
		for _, finding := range got.wmqFindings() {
			if strings.Contains(finding, d.name) {
				named = true
				break
			}
		}
		if !named {
			t.Errorf("removing %s's answer to ADR 0097 D2's second question (%s, line(s) %v) leaves the census with no finding about it — the arm above is decoration and the third wave of ranger-base-jyqk4 is found by a macos leg again", d.name, d.where, d.answerLines)
		}
		mutated++
	}
	if mutated == 0 {
		t.Fatal("no member of the class carried an answer to remove, so this arm measured nothing — either the class is empty (arm 1 says so too) or the seams this pin reads are no longer the seams the arms use")
	}
	t.Logf("%d member(s) reddened the census with their own answer removed", mutated)
}

// Arm 3: the shapes, in both directions.
//
// Arm 2 proves the census reds where it must; this one proves what it leaves
// alone, which is the half a broad rule has to earn. The rows that are NOT
// members are the ones a looser pin would red on: the 30-odd arms that take
// wckFixture, the live pin that ships nothing and skips on the real
// mechanism, and a body whose only mention of the script is in a comment —
// which is the one row a `grep -l` could never get right, and the reason this
// census is a parse.
//
// A row is a whole fixture FILE and not one body, because the shapes
// ranger-base-72t07 found walk between two declarations: the ship extracted
// one frame down, the const parked in a package var. Each row names every
// member it expects and every member the census must complain about, so a
// shape that moves membership from one declaration to another cannot read as
// a pass on the strength of the other one.
func TestQATheWallMechanismCensusRefusesTheWrongShapes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		src      string
		members  []string
		findings []string
	}{{
		name: "builds its own checker, ships, and says nothing — ranger-base-zbmxc's defect",
		src: `func TestForgot(t *testing.T) {
	repo := wtRepo(t)
	write(t, filepath.Join(repo, wallCheckScript), "#!/bin/sh\nexit 0\n")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(repo, "main")
}`,
		members:  []string{"TestForgot"},
		findings: []string{"TestForgot"},
	}, {
		name: "the checker's own seam — wckFixture's line, and zbmxc's fix",
		src: `func TestSetsTheCheckerSeam(t *testing.T) {
	repo := wtRepo(t)
	write(t, filepath.Join(repo, wallCheckScript), "#!/bin/sh\nexit 0\n")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.wallMechFn = func() string { return WallBwrap }
	c.once(repo, "main")
}`,
		members: []string{"TestSetsTheCheckerSeam"},
	}, {
		name: "the package seam — TestATreeThatShipsNoWallCheckIsLeftAloneEntirely's",
		src: `func TestWritesThePackageVar(t *testing.T) {
	wallMechanismForTest(t, "linux")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	if strings.Contains(out.String(), "wall") {
		t.Errorf("a repo shipping no %s hears about the wall", wallCheckScript)
	}
	_ = c
}`,
		members: []string{"TestWritesThePackageVar"},
	}, {
		name: "the host's own reading behind a skip — the live pin's shape, with a ship",
		src: `func TestLiveSomething(t *testing.T) {
	if m := WallMechanism(); m != WallBwrap {
		t.Skipf("NOTHING MEASURED: this host realizes its wall with %s", m)
	}
	shipWallCheck(t, wtRepo(t), "#!/bin/sh\nexit 0\n", "ship it")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	_ = c
}`,
		members: []string{"TestLiveSomething"},
	}, {
		name: "clearing the seam to nil is an answer too, not the absence of one",
		src: `func TestClearsTheSeam(t *testing.T) {
	shipWallCheck(t, wtRepo(t), "#!/bin/sh\nexit 0\n", "ship it")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.wallMechFn = nil
	wallMechanismForTest(t, "darwin")
}`,
		members: []string{"TestClearsTheSeam"},
	}, {
		name: "a method, which the census reads for twdParseDir's reason",
		src: `func (r *wckRig) start(t *testing.T) {
	shipWallCheck(t, r.repo, "#!/bin/sh\nexit 0\n", "ship it")
	r.c = newLandedChecker(r.a, r.out)
}`,
		members:  []string{"(*wckRig).start"},
		findings: []string{"(*wckRig).start"},
	}, {
		// ranger-base-72t07's shape (a): the ONE call site of shipWallCheck
		// in the tree is inside wckFixture, so this is the refactor that
		// emptied the class. The caller is the member — the helper builds
		// nothing — and the census has to follow the edge to see it at all.
		name: "the ship extracted one frame down — the refactor the first cut went dark on",
		src: `func wckShip(t *testing.T, repo string) {
	shipWallCheck(t, repo, "#!/bin/sh\nexit 0\n", "ship it")
}

func TestShipsOneFrameDown(t *testing.T) {
	repo := wtRepo(t)
	wckShip(t, repo)
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(repo, "main")
}`,
		members:  []string{"TestShipsOneFrameDown"},
		findings: []string{"TestShipsOneFrameDown"},
	}, {
		name: "the same, answered: a ship a frame down does not excuse the member, and an answer in its own body settles it",
		src: `func wckShip(t *testing.T, repo string) {
	shipWallCheck(t, repo, "#!/bin/sh\nexit 0\n", "ship it")
}

func TestShipsOneFrameDownAndAnswers(t *testing.T) {
	repo := wtRepo(t)
	wckShip(t, repo)
	wallMechanismForTest(t, "linux")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(repo, "main")
}`,
		members: []string{"TestShipsOneFrameDownAndAnswers"},
	}, {
		// THE DIRECTION THIS PIN TAKES, held by a row so it cannot drift
		// into an accident: the answer is demanded in the member's own body
		// even when the helper it calls has one, because an inherited
		// answer would be a false GREEN and this is a false red. The head
		// comment argues it; this row is the argument's teeth.
		name: "an answer one frame down is NOT this census's answer — a red, deliberately",
		src: `func wckStart(t *testing.T, repo string) {
	shipWallCheck(t, repo, "#!/bin/sh\nexit 0\n", "ship it")
	wallMechanismForTest(t, "linux")
}

func TestAnswersOneFrameDown(t *testing.T) {
	repo := wtRepo(t)
	wckStart(t, repo)
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(repo, "main")
}`,
		members:  []string{"TestAnswersOneFrameDown"},
		findings: []string{"TestAnswersOneFrameDown"},
	}, {
		// ranger-base-8752f: the MIRROR of shape (a) — the BUILDER extracted
		// one frame down instead of the ship. The test is `ships && !builds`
		// and the helper `builds && !ships`, so NEITHER is a member and
		// nobody is asked D2's second question. This census reads that
		// GREEN, which is a false green and the only one on the blindness
		// list; the row is here so it is a decision somebody made and not an
		// accident, and "THE BUILD SIDE DOES NOT CLOSE EITHER" in the head
		// comment prices what closing it costs (one false red over a
		// `t.Parallel()` member that cannot use the package seam).
		//
		// A row with no member and no finding is the strongest statement
		// available here: the day the build side closes, this row reds and
		// its replacement says so.
		name: "the BUILD one frame down — the mirror refactor, read green and said so",
		src: `func wckBuild(t *testing.T) *landedChecker {
	return newLandedChecker(wtApp(t), &bytes.Buffer{})
}

func TestBuildsOneFrameDown(t *testing.T) {
	repo := wtRepo(t)
	shipWallCheck(t, repo, "#!/bin/sh\nexit 0\n", "ship it")
	c := wckBuild(t)
	c.once(repo, "main")
}`,
	}, {
		// ranger-base-72t07's shape (b): the const reached through a
		// package var, which is why var and const declarations are
		// declarations here.
		name: "the const parked in a package var — shape (b)",
		src: `var qa72t07script = wallCheckScript

func TestReadsThePackageVar(t *testing.T) {
	repo := wtRepo(t)
	write(t, filepath.Join(repo, qa72t07script), "#!/bin/sh\nexit 0\n")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(repo, "main")
}`,
		members:  []string{"TestReadsThePackageVar"},
		findings: []string{"TestReadsThePackageVar"},
	}, {
		name: "three hops, because a fixpoint and not one hop is the claim",
		src: `var qa72t07script = wallCheckScript

func wckPath(repo string) string { return filepath.Join(repo, qa72t07script) }

func wckShipFar(t *testing.T, repo string) {
	write(t, wckPath(repo), "#!/bin/sh\nexit 0\n")
}

func TestThreeHops(t *testing.T) {
	repo := wtRepo(t)
	wckShipFar(t, repo)
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(repo, "main")
}`,
		members:  []string{"TestThreeHops"},
		findings: []string{"TestThreeHops"},
	}, {
		name: "an arm that takes the fixture builds no checker of its own",
		src: `func TestTakesTheFixture(t *testing.T) {
	c, out, repo, _ := wckFixture(t)
	wckGreen(t, c, repo)
	if !strings.Contains(out.String(), "is judged by its OWN "+wallCheckScript) {
		t.Error("no sentence")
	}
}`,
	}, {
		name: "a checker built over a tree that has nothing to do with the wall check",
		src: `func TestTheJudgeAlone(t *testing.T) {
	repo := wtRepo(t)
	shipLandedCheck(t, repo, "#!/bin/sh\nexit 0\n", "ship the judge")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(repo, "main")
}`,
	}, {
		name: "a mention in a COMMENT is not a reading — what a grep cannot tell",
		src: `func TestOnlyACommentNamesIt(t *testing.T) {
	// wallCheckScript is the wall check, and this arm is not about it:
	// shipWallCheck is one file over.
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(wtRepo(t), "main")
}`,
	}, {
		// The negative half of the closure: an edge is not a ship. A body
		// that calls a helper reaching no mention of the script is no more
		// a member than it was before the fixpoint existed, and a package
		// var that holds something else is not the const.
		name: "a helper that reaches no wall check carries nothing up to its caller",
		src: `var qa72t07other = "scripts/posse-land-check"

func wckHelper(t *testing.T, repo string) {
	write(t, filepath.Join(repo, qa72t07other), "#!/bin/sh\nexit 0\n")
}

func TestCallsAHelperThatShipsNothing(t *testing.T) {
	repo := wtRepo(t)
	wckHelper(t, repo)
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(repo, "main")
}`,
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			corpus := wmqFixture(t, c.src)
			want := append([]string(nil), c.members...)
			sort.Strings(want)
			if got := corpus.wmqNames(); !wmqSameNames(got, want) {
				t.Errorf("member(s) = %v, want %v — the class is the thing this pin is about, so a row on the wrong side of it is either a red somebody will price out or a hole nobody will see", got, want)
				for _, d := range corpus.decls {
					t.Logf("  %s (%s): ships=%v (%s) builds=%v answers=%v", d.name, d.kind, d.ships, d.wmqVia(), d.builds, d.answers)
				}
			}
			findings := corpus.wmqFindings()
			if len(findings) != len(c.findings) {
				t.Errorf("%d finding(s), want %d: %s", len(findings), len(c.findings), strings.Join(findings, "\n"))
			}
			for _, name := range c.findings {
				var named bool
				for _, f := range findings {
					if strings.Contains(f, name) {
						named = true
						break
					}
				}
				if !named {
					t.Errorf("no finding names %s, so a reader cannot tell which body to go and fix:\n%s", name, strings.Join(findings, "\n"))
				}
			}
		})
	}
}

func wmqSameNames(got, want []string) bool {
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

// wmqBlank empties the given 1-based lines of a source file. Emptied rather
// than deleted, so every other line keeps its number and a finding's
// `file:line` still names where a reader should look.
func wmqBlank(src string, lines []int) string {
	out := strings.Split(src, "\n")
	for _, n := range lines {
		if n >= 1 && n <= len(out) {
			out[n-1] = ""
		}
	}
	return strings.Join(out, "\n")
}
