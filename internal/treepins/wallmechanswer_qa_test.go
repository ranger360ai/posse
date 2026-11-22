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
// which is what this file is for: the third one is caught by a suite.
//
// THE RULE. A census over internal/posse's test sources. A function that
// BUILDS its own checker (`newLandedChecker`) and NAMES the wall check
// (`shipWallCheck`, the `wallCheckScript` const, or that const's path as a
// literal) must answer D2's second question through one of the three seams,
// in its own body. Functions and not tests: `wckFixture` is a helper and is a
// member, and the pin that could not see it would be the pin jyqk4 already
// wrote in prose.
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
// over. What the census must NOT do is reach a body that never names the
// script — the 30-odd arms that take `wckFixture` and get the answer from it,
// and the live pin, which ships nothing and skips on the real mechanism.
//
// A PARSE AND NOT A GREP, for two reasons a `grep -l` cannot have: a mention
// in a COMMENT is not a reading (arm 3 holds that), and the answer has to be
// in the member's OWN body rather than somewhere in the file — which is the
// whole difference between wckFixture's line and the file it was supposed to
// cover. Its subject is the tree, so it lives here and is doored by
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

// wmqFile is one candidate source: a test file of the subject package whose
// bytes name the builder.
type wmqFile struct {
	path string
	src  string
}

// wmqSubjectFiles enumerates the subject package's test files and returns the
// ones that could hold a member at all, with the number of files read.
//
// THE PREFILTER IS SOUND AND IT IS WHY THIS PIN IS CHEAP. A member CALLS
// wmqBuilder by that name, so a file whose bytes never carry it declares no
// member — and internal/posse is 761 test files and 15MB of source
// (MEASURED 2026-11-22, this box), which is a parse nobody needs to pay for:
// 9 of those files name the builder, and all three arms of this file run in
// 0.02s each. Every file's bytes are still read, which is both what makes the
// filter a fact rather than a guess and what makes this pin's reading the
// tree's (the enumeration is the directory, not a list of names in this
// file).
func wmqSubjectFiles(t *testing.T) (files []wmqFile, read int) {
	t.Helper()
	ents, err := os.ReadDir(wmqPkgDir)
	if err != nil {
		t.Fatalf("read %s: %v", wmqPkgDir, err)
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		read++
		path := filepath.Join(wmqPkgDir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if src := string(b); strings.Contains(src, wmqBuilder) {
			files = append(files, wmqFile{path, src})
		}
	}
	return files, read
}

// wmqFunc is one function declaration of the subject package, read for the
// three facts this pin is about.
type wmqFunc struct {
	where       string   // file:line, for a reader who has to go and look
	name        string   // the declaration's name, receiver included
	ships       bool     // it names the wall check
	builds      bool     // it builds a checker of its own
	answers     []string // the seams it answers D2's second question through
	answerLines []int    // and where, for arm 2's mutant
}

// member reports whether this body is in the class: it builds its own checker
// AND names the wall check. Both halves, because a body that takes its
// checker from wckFixture has the answer already and a body that never
// mentions the wall check cannot be measuring the step.
func (f wmqFunc) member() bool { return f.ships && f.builds }

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

// wmqScan reads one file's declarations. The source is a parameter rather
// than a path so arm 2 can hand it a mutant of the real thing.
func wmqScan(path, src string) ([]wmqFunc, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, err
	}
	var out []wmqFunc
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		f := wmqFunc{
			where: fmt.Sprintf("%s:%d", path, fset.Position(fn.Pos()).Line),
			name:  wmqDeclName(fn),
		}
		seen := map[string]bool{}
		answer := func(n ast.Node, seam string) {
			f.answerLines = append(f.answerLines, fset.Position(n.Pos()).Line)
			if !seen[seam] {
				seen[seam] = true
				f.answers = append(f.answers, seam)
			}
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				// The const, wherever it is used: a path argument, a wanted
				// string, a failure message. See the head comment on why the
				// naming side is broad.
				if x.Name == wmqScript {
					f.ships = true
				}
			case *ast.BasicLit:
				// And the path typed out, which is the spelling a body that
				// does not import the const would use.
				if x.Kind == token.STRING && strings.Contains(x.Value, wmqScriptPath) {
					f.ships = true
				}
			case *ast.CallExpr:
				switch wmqCallee(x.Fun) {
				case wmqBuilder:
					f.builds = true
				case wmqShipper:
					f.ships = true
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
		out = append(out, f)
	}
	return out, nil
}

// wmqFindings is every complaint ADR 0097 D2's second question makes about a
// set of scanned declarations.
//
// A function and not inline, because arms 2 and 3 require it to be NON-empty
// over mutants and shapes — so the check arm 1 makes and the check the other
// two prove can fail are the same code and cannot drift apart. A closed-set
// check nobody has seen red in the direction it exists for is a comment
// (ranger-base-ietka).
func wmqFindings(funcs []wmqFunc) []string {
	var out []string
	for _, f := range funcs {
		if !f.member() || len(f.answers) > 0 {
			continue
		}
		out = append(out, fmt.Sprintf(
			"%s (%s) names the wall check and builds its own checker with %s, and answers ADR 0097 D2's "+
				"SECOND question — is this host's wall a bwrap — through none of this package's three seams "+
				"(`c.%s = func() string { return WallBwrap }`, `%s(t, \"linux\")`, or a `%s()` read behind a "+
				"skip). At sandbox-exec the wall step DOES NOT EXIST (ADR 0096: no darwin change), so this "+
				"body measures the one line that declines it: green on every ubuntu leg and red on every "+
				"macos one, which is how ranger-base-jyqk4 and ranger-base-zbmxc were both found by a CI leg "+
				"instead of by a suite. Arms that take wckFixture (internal/posse/wallcheck_test.go) get the "+
				"answer from it; an arm that builds its own checker answers for itself.",
			f.name, f.where, wmqBuilder, wmqCheckerSeam, wmqPackageSeam, wmqHostSeam))
	}
	return out
}

// Arm 1: the class over the real tree, and the census is reading something.
//
// The three assertions in front of the findings are the dark-census half, and
// they are not decoration: every name this pin reads lives in another package
// and a rename there is a rename nobody would think to bring here. A census
// that finds no candidate file, no member, or a seam that is no longer
// declared has stopped asking the question, and a pin in that state prints
// `ok` forever — which is the exact shape of the hole this file closes, one
// layer up.
func TestQAEveryWallArmThatBuildsItsOwnCheckerAnswersTheMechanismQuestion(t *testing.T) {
	t.Parallel()
	files, read := wmqSubjectFiles(t)
	if read < 100 {
		t.Fatalf("%d test file(s) under %s — the census read the wrong directory", read, wmqPkgDir)
	}
	if len(files) == 0 {
		t.Fatalf("no test file under %s names %s, so this census has no candidate and no member — the builder was renamed and the class is unread (ADR 0097 D2, ranger-base-akwrv)", wmqPkgDir, wmqBuilder)
	}
	seam, err := os.ReadFile(wmqSeamFile)
	if err != nil {
		t.Fatalf("read %s: %v", wmqSeamFile, err)
	}
	if !strings.Contains(string(seam), wmqSeamDecl) {
		t.Fatalf("%s no longer declares `%s` — the seam this pin requires an arm to set is gone or renamed, so the rule is about nothing (ADR 0097 D2)", wmqSeamFile, wmqSeamDecl)
	}

	var funcs []wmqFunc
	for _, f := range files {
		got, err := wmqScan(f.path, f.src)
		if err != nil {
			t.Fatalf("parse %s: %v", f.path, err)
		}
		funcs = append(funcs, got...)
	}

	var members []string
	for _, f := range funcs {
		if f.member() {
			members = append(members, fmt.Sprintf("%s — %s", f.name, strings.Join(f.answers, ", ")))
		}
	}
	sort.Strings(members)
	if len(members) < wmqMemberFloor {
		t.Errorf("%d member(s) of the class, and this file's floor is %d — a census that finds nobody reads green whatever the tree does. The two members ranger-base-akwrv was filed over are wckFixture and TestQATheVenueCallSiteDiffersFromTheJudgesInTheKindAndTheChildOnly; if one was deleted on purpose, lower the floor in the same commit and say which bead:\n  %s",
			len(members), wmqMemberFloor, strings.Join(members, "\n  "))
	}
	for _, finding := range wmqFindings(funcs) {
		t.Error(finding)
	}
	t.Logf("%d test file(s) under %s, %d of them naming %s; %d member(s):\n  %s",
		read, wmqPkgDir, len(files), wmqBuilder, len(members), strings.Join(members, "\n  "))
}

// Arm 2: the census reds on the regression it was filed for — every member's
// own answer, removed.
//
// The mutant is the tree's, not a fixture's: take each member's answer lines
// out of the real file and require the census to name that member. That is
// ranger-base-zbmxc's defect put back, member by member, for however many
// members the class grows — a hand-written mutant would be one body's and
// would go stale the day a fourth arrives.
func TestQATheWallMechanismCensusRedsWhenAMembersAnswerGoes(t *testing.T) {
	t.Parallel()
	files, _ := wmqSubjectFiles(t)
	mutated := 0
	for _, f := range files {
		funcs, err := wmqScan(f.path, f.src)
		if err != nil {
			t.Fatalf("parse %s: %v", f.path, err)
		}
		for _, fn := range funcs {
			if !fn.member() || len(fn.answerLines) == 0 {
				continue
			}
			got, err := wmqScan(f.path, wmqBlank(f.src, fn.answerLines))
			if err != nil {
				t.Errorf("the mutant that removes %s's answer does not parse (%v) — this arm removes whole LINES, so an answer that spans several needs a mutation this arm does not have, and until it has one that member's finding is unproved", fn.name, err)
				continue
			}
			var named bool
			for _, finding := range wmqFindings(got) {
				if strings.Contains(finding, fn.name) {
					named = true
					break
				}
			}
			if !named {
				t.Errorf("removing %s's answer to ADR 0097 D2's second question (%s, line(s) %v) leaves the census with no finding about it — the arm above is decoration and the third wave of ranger-base-jyqk4 is found by a macos leg again", fn.name, fn.where, fn.answerLines)
			}
			mutated++
		}
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
func TestQATheWallMechanismCensusRefusesTheWrongShapes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		body    string
		member  bool
		finding bool
	}{{
		name: "builds its own checker, ships, and says nothing — ranger-base-zbmxc's defect",
		body: `func TestForgot(t *testing.T) {
	repo := wtRepo(t)
	write(t, filepath.Join(repo, wallCheckScript), "#!/bin/sh\nexit 0\n")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(repo, "main")
}`,
		member:  true,
		finding: true,
	}, {
		name: "the checker's own seam — wckFixture's line, and zbmxc's fix",
		body: `func TestSetsTheCheckerSeam(t *testing.T) {
	repo := wtRepo(t)
	write(t, filepath.Join(repo, wallCheckScript), "#!/bin/sh\nexit 0\n")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.wallMechFn = func() string { return WallBwrap }
	c.once(repo, "main")
}`,
		member: true,
	}, {
		name: "the package seam — TestATreeThatShipsNoWallCheckIsLeftAloneEntirely's",
		body: `func TestWritesThePackageVar(t *testing.T) {
	wallMechanismForTest(t, "linux")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	if strings.Contains(out.String(), "wall") {
		t.Errorf("a repo shipping no %s hears about the wall", wallCheckScript)
	}
	_ = c
}`,
		member: true,
	}, {
		name: "the host's own reading behind a skip — the live pin's shape, with a ship",
		body: `func TestLiveSomething(t *testing.T) {
	if m := WallMechanism(); m != WallBwrap {
		t.Skipf("NOTHING MEASURED: this host realizes its wall with %s", m)
	}
	shipWallCheck(t, wtRepo(t), "#!/bin/sh\nexit 0\n", "ship it")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	_ = c
}`,
		member: true,
	}, {
		name: "clearing the seam to nil is an answer too, not the absence of one",
		body: `func TestClearsTheSeam(t *testing.T) {
	shipWallCheck(t, wtRepo(t), "#!/bin/sh\nexit 0\n", "ship it")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.wallMechFn = nil
	wallMechanismForTest(t, "darwin")
}`,
		member: true,
	}, {
		name: "a method, which the census reads for twdParseDir's reason",
		body: `func (r *wckRig) start(t *testing.T) {
	shipWallCheck(t, r.repo, "#!/bin/sh\nexit 0\n", "ship it")
	r.c = newLandedChecker(r.a, r.out)
}`,
		member:  true,
		finding: true,
	}, {
		name: "an arm that takes the fixture builds no checker of its own",
		body: `func TestTakesTheFixture(t *testing.T) {
	c, out, repo, _ := wckFixture(t)
	wckGreen(t, c, repo)
	if !strings.Contains(out.String(), "is judged by its OWN "+wallCheckScript) {
		t.Error("no sentence")
	}
}`,
	}, {
		name: "a checker built over a tree that has nothing to do with the wall check",
		body: `func TestTheJudgeAlone(t *testing.T) {
	repo := wtRepo(t)
	shipLandedCheck(t, repo, "#!/bin/sh\nexit 0\n", "ship the judge")
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(repo, "main")
}`,
	}, {
		name: "a mention in a COMMENT is not a reading — what a grep cannot tell",
		body: `func TestOnlyACommentNamesIt(t *testing.T) {
	// wallCheckScript is the wall check, and this arm is not about it:
	// shipWallCheck is one file over.
	c := newLandedChecker(wtApp(t), &bytes.Buffer{})
	c.once(wtRepo(t), "main")
}`,
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			funcs, err := wmqScan("fixture_test.go", "package posse\n\n"+c.body+"\n")
			if err != nil {
				t.Fatalf("the fixture does not parse, so this row measures nothing: %v", err)
			}
			if len(funcs) != 1 {
				t.Fatalf("the fixture declares %d function(s), want 1", len(funcs))
			}
			f := funcs[0]
			if f.member() != c.member {
				t.Errorf("member() = %v (ships=%v builds=%v), want %v — the class is the thing this pin is about, so a row on the wrong side of it is either a red somebody will price out or a hole nobody will see",
					f.member(), f.ships, f.builds, c.member)
			}
			findings := wmqFindings(funcs)
			if got := len(findings) > 0; got != c.finding {
				t.Errorf("findings = %v, want %v: %s", findings, c.finding, strings.Join(findings, "\n"))
			}
			if c.finding && len(findings) > 0 && !strings.Contains(findings[0], f.name) {
				t.Errorf("the finding does not name %s, so a reader cannot tell which body to go and fix:\n%s", f.name, findings[0])
			}
		})
	}
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
