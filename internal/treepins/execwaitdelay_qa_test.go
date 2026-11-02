package treepins

// QA pin for ranger-base-2cttm: a context deadline without cmd.WaitDelay does
// not bound the call.
//
// THE MECHANISM, which herdrWaitDelay's doc (internal/posse/herdr.go) already
// states: os/exec's Wait does not return until the goroutines copying the
// child's stdout and stderr are done, and a GRANDCHILD that inherited those
// pipes keeps them open after the child is killed. So a context deadline
// alone does not bound a call that captures output — it bounds only the
// child's life. cmd.WaitDelay is the second half of the kill.
//
// MEASURED 2026-10-31 (ranger-base-fnn1o's fixture, darwin, in
// internal/posse): a 200ms context over `#!/bin/sh` + `echo <line>` +
// `sleep 120` — the shell forks the sleep and waits on it — returned from
// cmd.Output() after 120.16s with WaitDelay dropped, and after ~0.4s with it
// set.
//
// CORRECTED 2026-10-31 under ranger-base-0npto. This note used to add that a
// ONE-command body (`sleep 120` alone) is bounded either way, "because `sh
// -c` execs a lone command and the process the context kills IS the
// sleeper". It does not reproduce, and it was never about this fixture: the
// fixture is a script FILE, not `sh -c`, and darwin's /bin/sh forks the lone
// command rather than exec'ing it, so that body leaks a grandchild too.
// RE-MEASURED on this box (darwin 25.6.0), a 200ms limit and a 25s sleep,
// five runs of each of five bodies — `sleep` alone, echo-then-sleep,
// echo-then-background-sleep-then-sleep, background-sleep-then-wait,
// echo-then-background-sleep-then-wait: with WaitDelay dropped all 25 runs
// waited the full 25s (25.01-25.12s), and with it set all 25 returned in
// 0.20-0.41s. No body on this box is bounded by the context alone. A child
// that forks NO shell is (the `ps` ruling below), and that is a different
// claim about a different fixture.
//
// THE 200ms IN BOTH PARAGRAPHS ABOVE NEEDS A QUIET BOX, and this is the one
// sentence to carry forward into the next WaitDelay fixture built from these
// notes (ranger-base-metqp, from ranger-base-grj2i). Those readings are
// sound — a 25s wait is proof the sleeper ran — but they were taken where a
// /bin/sh starts inside 200ms, and on a loaded one it does not: MEASURED
// 2026-10-31 at loadavg 5.9/21.1, with the sleeper touching a witness file
// as its first act, a 200ms limit started it 0 times in 6 and 800ms 3 times
// in 6, while 1600ms and 3s started it 6/6. A fixture at 200ms on a loaded
// box therefore measures NOTHING about WaitDelay — the child dies before it
// forks, so there is no grandchild to hold the pipes — and it passes either
// way. internal/posse's own arm ran at 200ms for that reason and was green
// over a tree with the assignment deleted; it asserts the witness now, at a
// 4s limit.
//
// AND THE LIMIT WAS THE WRONG LEVER (ranger-base-y9q7l). What the fixture
// cannot fit inside a small deadline is the FIRST exec of the executable it
// just wrote — darwin's charge on a new binary — and not the scheduler:
// MEASURED 2026-11-02 on this box, a fresh path each time read mean
// 164-714ms and worst 2.17s, while a re-exec of the SAME path read mean
// 6.3-8.8ms and 7.8-21ms at the tail. The largest worst-case came at the
// QUIETEST loadavg of the five and the cheapest reading of all at the
// busiest, so the load column nobody had would not have warned either. At 3s
// the arm's own witness fired on a full `make test` and reddened arm 3 for a
// seat whose diff touched no production code. So build
// the next one with THREE parts: the `& wait` body, a positive control the
// fixture writes before it forks, and a PRE-FLIGHT invocation that pays the
// first exec outside the deadline — leaving the limit to cover a warm
// re-exec, which is the only part a number can honestly bound. The limit is
// still the part that goes stale; the control is what reds instead of passing
// when it does.
//
// WHY A PIN AND NOT PROSE. The rule was written out in three doc comments and
// honoured at five of fifteen call sites. ranger-base-2cttm censused the tree
// by hand and found nine sites with a deadline and no WaitDelay — four of
// them real (three `gh`, one `python3`, one `grok inspect` sitting in FRONT of
// a launch behind a CLI that carries a self-updater) and three ruled exempt.
// Nothing compared the rule with the code, so the next site was always one
// landing away. This is that comparison.
//
// THE RULE IS UNCONDITIONAL over `exec.CommandContext`, and that is narrower
// than the hazard on purpose. The hazard is capture — Wait blocks on the copy
// goroutines, which os/exec starts only for a Stdout/Stderr that is not an
// *os.File — but "does this site capture" is a second analysis, and the one
// that can be wrong while looking green. WaitDelay on a site that captures
// nothing starts no goroutine and so bounds nothing that was not already
// bounded, which makes the unconditional rule free; a capture detector that
// missed one writer would not be. So: every bounded child names its grace, or
// its binary is ruled exempt below.
//
// Non-test sources only. A test that leaks a Wait is a slow test and is the
// suite's own business (internal/posse's watchhang and runtimeprobe pins); a
// shipped call that leaks one is a wedged dispatch pass.
//
// The behaviour behind the rule is pinned where it happens, not here:
// TestReadCLIVersionGivesUpOnACLIThatWillNotAnswer (internal/posse,
// runtimeprobe_test.go) runs the measured fixture above through a real child
// and fails if the call outlives its deadline plus grace. This pin is the
// census; that one is the proof the census is about something.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ewdCorpusRoot is the root holding posse's shipped Go, for the sentences
// this pin prints. The WALK below spells it as a literal at the call rather
// than reading this variable, which is not a style choice: the tree-wide
// class is DERIVED from a WalkDir whose root is a literal relative path
// (treewidedoor_qa_test.go, twdFunc.relEnum), so a root arriving as a
// variable is a pin whose membership nothing can see and whose door nothing
// would demand.
var ewdCorpusRoot = "the repository root"

// ewdExemptBinaries is the ruling half of ranger-base-2cttm, keyed by the
// BINARY rather than by file and line. The question WaitDelay answers is
// "can this child leave a descendant holding the pipes", which is a fact
// about the program being run and not about the line that runs it — so a
// per-line roster would go stale on every move of a call that did not change
// its answer, and would say nothing about the next site that runs the same
// binary.
//
// A row here is a claim that the named program forks nothing that could
// inherit its stdout. It is also permission for a site to be CHAINED
// (`exec.CommandContext(...).Output()`), which cannot set WaitDelay at all
// for want of a variable.
var ewdExemptBinaries = map[string]string{
	"ps": "`ps` reads the kernel's process table and exits; it forks nothing, " +
		"so the process the context kills IS the only holder of the pipe and " +
		"the deadline bounds the whole call on its own. Three sites: the load " +
		"guard's CPU census and its argv read (internal/posse/loadguard.go) " +
		"and the kill arm's re-read (loadguardkill.go), all three chained " +
		"onto .Output().",
}

// ewdSite is one `exec.CommandContext` call in the shipped tree.
type ewdSite struct {
	file   string
	line   int
	pos    token.Pos // for resolving which function it sits in
	cmdVar string    // the identifier the *exec.Cmd was bound to; "" when chained
	binary string    // argv[0] when it is a string literal; "" when it is a value
	satBy  string    // what set its WaitDelay; "" = nothing did
}

// ewdSat is one thing that sets WaitDelay on an identifier: either the
// assignment itself or a call handing the cmd to a binder. scope is the
// function enclosing it, so a `cmd` in one function cannot satisfy a `cmd`
// in another.
type ewdSat struct {
	ident string
	pos   token.Pos
	scope [2]token.Pos
	what  string
}

// ewdGoFiles is the corpus: every non-test .go file in the shipped tree.
// This package's TestMain chdirs the binary to the repo root, so the "."
// below is a tree path.
//
// WIDENED 2026-10-31 under ranger-base-tpy6q (ranger-base-rc63f FINDING 2).
// It walked cmd/ and internal/ and nothing else, so a .go file at the
// REPOSITORY ROOT sat outside a census whose sentence is "every bounded
// child" — and the roots were spelled as literals for a reason that says
// nothing about which roots they are. The reading did not change: the tree
// has exactly one non-test .go at the root (embed.go), it runs no child,
// and the count went 159 -> 160 with the site list identical (MEASURED
// 2026-10-31, this bead). What changed is that the next root file, and the
// next top-level package directory, are inside the census rather than
// outside it. One literal relative root is still what the tree-wide class
// is derived from, so the walk still spells it at the call.
//
// Dot-directories are skipped along with node_modules and testdata: no
// shipped Go lives in one, and a seat's scratch checkout under .posse/ is
// not this tree's bounded children.
func ewdGoFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			n := d.Name()
			if path != "." && (strings.HasPrefix(n, ".") || n == "node_modules" || n == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(path)
		if strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree for shipped Go: %v", err)
	}
	sort.Strings(out)
	return out
}

// ewdIsSel reports whether e is the selector `pkg.name`.
func ewdIsSel(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

// ewdFuncRanges returns every FuncDecl and FuncLit span in f, so the
// innermost function enclosing a position can be found by taking the
// smallest span that contains it.
func ewdFuncRanges(f *ast.File) [][2]token.Pos {
	var out [][2]token.Pos
	ast.Inspect(f, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Body != nil {
				out = append(out, [2]token.Pos{fn.Pos(), fn.End()})
			}
		case *ast.FuncLit:
			out = append(out, [2]token.Pos{fn.Pos(), fn.End()})
		}
		return true
	})
	return out
}

// ewdEnclosing is the smallest span in ranges that contains p, or the whole
// file's span when p sits in no function at all.
func ewdEnclosing(ranges [][2]token.Pos, p token.Pos, file [2]token.Pos) [2]token.Pos {
	best := file
	for _, r := range ranges {
		if p < r[0] || p >= r[1] {
			continue
		}
		if r[1]-r[0] < best[1]-best[0] {
			best = r
		}
	}
	return best
}

// ewdBinders are the helper functions that set WaitDelay on an *exec.Cmd
// their caller hands them — DERIVED, never listed. internal/posse has one
// (bindGitDeadline: "there is one choke point and no call site that can
// forget the clock"), and a roster of them here would be the second place
// that fact is written and the one that goes stale. A function qualifies
// when it takes an *exec.Cmd parameter and assigns WaitDelay on it.
//
// The value is WHICH PARAMETER POSITIONS it assigns on, not a bool
// (ranger-base-tpy6q, ranger-base-rc63f FINDING 1c). A bool said "this
// function binds a child", and ewdScan then read every identifier argument
// of a call to it as bound — so `bindIt(nil, cmd)`, where bindIt touches
// only its first parameter, satisfied cmd. The position is the whole of what
// makes a binder call a satisfier of THIS child, and it is derived from the
// same body read.
//
// Positions are flat and count unnamed parameters, because a call's argument
// list is flat: `func f(ctx context.Context, cmd *exec.Cmd)` binds position
// 1, and `func f(a, b *exec.Cmd)` has positions 0 and 1 in one field.
//
// Methods are skipped: a call to one is a SelectorExpr (`x.bind(cmd)`), which
// ewdScan does not read as a binder call at all, so registering one could
// only ever mark a same-named plain function's call satisfied.
//
// Still keyed by NAME across the whole corpus, deliberately — an unqualified
// call resolves in its own package, so the tight key would be the package
// directory, and that is a type question this pin does not ask. It is reached
// only by a non-binder sharing a binder's name in another package; the tree
// has one binder and no such twin (MEASURED 2026-10-31).
func ewdBinders(files map[string]*ast.File) map[string]map[int]bool {
	out := map[string]map[int]bool{}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil || fn.Type.Params == nil {
				continue
			}
			pos := 0
			for _, p := range fn.Type.Params.List {
				isCmd := false
				if star, ok := p.Type.(*ast.StarExpr); ok && ewdIsSel(star.X, "exec", "Cmd") {
					isCmd = true
				}
				if len(p.Names) == 0 {
					pos++ // an unnamed parameter holds a position and can be assigned nothing
					continue
				}
				for _, nm := range p.Names {
					if isCmd && ewdAssignsWaitDelay(fn.Body, nm.Name) {
						if out[fn.Name.Name] == nil {
							out[fn.Name.Name] = map[int]bool{}
						}
						out[fn.Name.Name][pos] = true
					}
					pos++
				}
			}
		}
	}
	return out
}

// ewdPassThrough names the helpers that take an *exec.Cmd and hand the SAME
// child back — DERIVED, never listed, on ewdBinders' own reasoning. A roster
// here would be the second place that fact is written and the one that goes
// stale.
//
// It exists because `cmd := protocolCmd(exec.CommandContext(…))` binds exactly
// the child `cmd := exec.CommandContext(…)` bound, and without one unwrap the
// scan reads every wrapped site as CHAINED — which is "no variable to set
// cmd.WaitDelay on", an unsatisfiable finding about a site that sets it on the
// next line. MEASURED 2026-10-31 (ranger-base-bm6j8, the landing that put ADR
// 0077 D3's choke point in front of every exec site in the tree): twelve
// sites at once, and this pin's own "not one site was read as naming its
// grace" arm beside them.
//
// Taking the *exec.Cmd back out is the whole of the rule. A helper that takes
// one and returns a bool or an error has not handed the caller a child to set
// anything on, so it binds nothing — which is the shape the two-way arm
// plants.
func ewdPassThrough(files map[string]*ast.File) map[string]bool {
	out := map[string]bool{}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Type.Params == nil || fn.Type.Results == nil {
				continue
			}
			takes := false
			for _, p := range fn.Type.Params.List {
				if star, ok := p.Type.(*ast.StarExpr); ok && ewdIsSel(star.X, "exec", "Cmd") {
					takes = true
				}
			}
			gives := false
			for _, r := range fn.Type.Results.List {
				if star, ok := r.Type.(*ast.StarExpr); ok && ewdIsSel(star.X, "exec", "Cmd") {
					gives = true
				}
			}
			if takes && gives {
				out[fn.Name.Name] = true
			}
		}
	}
	return out
}

// ewdAssignsWaitDelay reports whether body holds `<ident>.WaitDelay = …`.
func ewdAssignsWaitDelay(body ast.Node, ident string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range as.Lhs {
			if ewdIsSel(lhs, ident, "WaitDelay") {
				found = true
			}
		}
		return true
	})
	return found
}

// ewdScan reads one parsed file for sites and satisfiers.
func ewdScan(fset *token.FileSet, rel string, f *ast.File, binders map[string]map[int]bool, through map[string]bool) ([]ewdSite, []ewdSat) {
	ranges := ewdFuncRanges(f)
	whole := [2]token.Pos{f.Pos(), f.End()}

	// Which CommandContext calls were bound to an identifier. Collected
	// first, because a site's own node cannot see the assignment above it.
	bound := map[ast.Node]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != len(as.Rhs) {
			return true
		}
		for i, rhs := range as.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok {
				continue
			}
			if !ewdIsSel(call.Fun, "exec", "CommandContext") {
				// One unwrap through a pass-through helper, and one only:
				// `cmd := protocolCmd(exec.CommandContext(…))` is the same
				// binding, and nothing deeper is a shape this tree has.
				id, ok := call.Fun.(*ast.Ident)
				if !ok || !through[id.Name] || len(call.Args) == 0 {
					continue
				}
				inner, ok := call.Args[0].(*ast.CallExpr)
				if !ok || !ewdIsSel(inner.Fun, "exec", "CommandContext") {
					continue
				}
				call = inner
			}
			if id, ok := as.Lhs[i].(*ast.Ident); ok {
				bound[call] = id.Name
			}
		}
		return true
	})

	var sites []ewdSite
	var sats []ewdSat
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if ewdIsSel(node.Fun, "exec", "CommandContext") {
				s := ewdSite{file: rel, line: fset.Position(node.Pos()).Line, pos: node.Pos(), cmdVar: bound[node]}
				if len(node.Args) >= 2 {
					if lit, ok := node.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if v, err := strconv.Unquote(lit.Value); err == nil {
							s.binary = v
						}
					}
				}
				sites = append(sites, s)
				return true
			}
			// A binder call: `bindGitDeadline(ctx, cmd)`. Only the
			// ARGUMENTS in the positions the binder assigns on are
			// satisfied — a cmd handed to a parameter it never touches is
			// not bound by it.
			id, ok := node.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			at, ok := binders[id.Name]
			if !ok {
				return true
			}
			for i, a := range node.Args {
				if !at[i] {
					continue
				}
				if arg, ok := a.(*ast.Ident); ok {
					sats = append(sats, ewdSat{
						ident: arg.Name,
						pos:   node.Pos(),
						scope: ewdEnclosing(ranges, node.Pos(), whole),
						what:  id.Name + "()",
					})
				}
			}
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "WaitDelay" {
					continue
				}
				recv, ok := sel.X.(*ast.Ident)
				if !ok {
					continue
				}
				sats = append(sats, ewdSat{
					ident: recv.Name,
					pos:   node.Pos(),
					scope: ewdEnclosing(ranges, node.Pos(), whole),
					what:  "a WaitDelay assignment",
				})
			}
		}
		return true
	})

	// Resolve each site against the satisfiers PER SITE: the grace has to
	// reach THIS child. Three conditions, and each of the three is a hole
	// the per-name-per-function rule this replaces had
	// (ranger-base-tpy6q, from ranger-base-rc63f FINDING 1 — the holes are
	// still planted, flipped, in execwaitdelayholes_qa_test.go):
	//
	//  1. SAME IDENTIFIER. Unchanged, and the reason is unchanged: a `cmd`
	//     is one child only within one scope.
	//  2. THE SAME INNERMOST FUNCTION, exactly — not "in a function that
	//     encloses the site", and not "anywhere inside the site's own
	//     function" either. Two functions in one file routinely both call
	//     their child `cmd` (internal/posse/ciwatch.go has three), and a
	//     CLOSURE is a function: an inner `cmd := …` shadows the outer name
	//     and is a different child, so an outer grace does not reach it and
	//     a grace inside a closure does not reach a site outside it. Both
	//     directions fail CLOSED — a shape this rule cannot see is reported,
	//     never excused.
	//  3. AFTER THE SITE, AND BEFORE THE NEXT CHILD on that identifier. A
	//     grace is written after the child it binds, so the next event on
	//     the name after the site has to be one: the `cmd = exec.…` below it
	//     is a SECOND child, and a grace past that point is the second
	//     child's and not this one's. This is the shape an ordinary landing
	//     reaches — a second bounded child added to a function that has one,
	//     assigned to the same `cmd` — and the old rule read both sites as
	//     satisfied whichever side of the one assignment they sat.
	//
	// Source order is the whole of the reaching analysis, which is why 3
	// fails closed rather than open: a grace that sits textually BEFORE its
	// site (a loop body that assigns the child at the bottom) is reported
	// although it has one. No shipped site is that shape, and reporting a
	// bound child costs a reader one look while excusing an unbound one
	// costs a wedged dispatch pass.
	siteFn := make([][2]token.Pos, len(sites))
	for i := range sites {
		siteFn[i] = ewdEnclosing(ranges, sites[i].pos, whole)
	}
	for i := range sites {
		if sites[i].cmdVar == "" {
			continue // chained: nothing to set it on, so nothing can satisfy it
		}
		// Where this site's reach ends: the next child bound to the same
		// identifier in the same function, or the end of the file.
		limit := whole[1]
		for j := range sites {
			if j == i || sites[j].cmdVar != sites[i].cmdVar || siteFn[j] != siteFn[i] {
				continue
			}
			if sites[j].pos > sites[i].pos && sites[j].pos < limit {
				limit = sites[j].pos
			}
		}
		for _, s := range sats {
			if s.ident != sites[i].cmdVar || s.scope != siteFn[i] {
				continue
			}
			if s.pos > sites[i].pos && s.pos < limit {
				sites[i].satBy = s.what
				break
			}
		}
	}
	return sites, sats
}

func TestQAEveryBoundedChildNamesItsWaitDelay(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	rels := ewdGoFiles(t)
	files := map[string]*ast.File{}
	for _, rel := range rels {
		f, err := parser.ParseFile(fset, rel, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parsing %s: %v", rel, err)
		}
		files[rel] = f
	}

	// The positive witness. An assertion of absence is satisfied by a walk
	// that read nothing — a renamed root, a suffix list that stopped
	// matching — and "no bounded child is missing its grace" is exactly
	// what a walk over zero files says. The floors sit well under the live
	// counts (MEASURED 2026-10-31 at ranger-base-tpy6q's commit: 160
	// non-test .go files in the tree, 15 exec.CommandContext sites — 12
	// naming a grace, 3 ruled exempt, 0 bare — and 1 derived binder,
	// bindGitDeadline, on parameter 1) so an ordinary landing does not move
	// them.
	if len(files) < 100 {
		t.Fatalf("the walk read %d non-test .go files under %s — the corpus this pin censuses is not there",
			len(files), ewdCorpusRoot)
	}

	// And the corpus has to reach the tree ROOT, not only its package
	// directories (ranger-base-tpy6q, ranger-base-rc63f FINDING 2). This
	// arm exists because NOTHING ELSE in this file reds when the walk
	// narrows back: the tree's one root .go is embed.go, it runs no child,
	// so the census's reading is identical with the root in or out — which
	// is what let a census titled "every bounded child" walk cmd/ and
	// internal/ only. The arm is a second enumeration rather than a literal
	// "embed.go", so a root file added tomorrow is covered by it.
	root, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the repository root: %v", err)
	}
	inCorpus := map[string]bool{}
	for _, rel := range rels {
		inCorpus[rel] = true
	}
	for _, e := range root {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		if !inCorpus[n] {
			t.Errorf("%s is a non-test .go file at the tree root and the corpus this pin censuses does not hold it.\n"+
				"  The walk is rooted at the repository root for that reason: a child run from a root file is as bounded-or-not as one run from internal/, and the pin's sentence is \"every bounded child\". If the walk was narrowed back to package directories, widen it again rather than narrowing the sentence.", n)
		}
	}

	binders := ewdBinders(files)
	through := ewdPassThrough(files)
	if len(through) == 0 {
		t.Error("no function in the shipped tree takes an *exec.Cmd and returns one.\n" +
			"  internal/posse/protocolchild.go's protocolCmd is that shape and every exec site in the tree is wrapped in it (ADR 0077 D3), so a derivation that finds nothing has stopped reading — every wrapped site would then be read as CHAINED and reported for a grace it sets on the next line.")
	}
	if len(binders) == 0 {
		t.Error("no function in the shipped tree takes an *exec.Cmd and sets WaitDelay on it.\n" +
			"  internal/posse/githang.go's bindGitDeadline is that shape and three call sites depend on it, so a derivation that finds nothing has stopped reading rather than found a tree without binders — a site satisfied through a helper would now read as a bare one.")
	}

	var all []ewdSite
	for _, rel := range rels {
		sites, _ := ewdScan(fset, rel, files[rel], binders, through)
		all = append(all, sites...)
	}
	if len(all) < 10 {
		t.Fatalf("found %d exec.CommandContext sites in %d files — the scan is not reading this tree's bounded children",
			len(all), len(files))
	}

	// Every site either names its grace, or runs a binary ruled exempt.
	seenExempt := map[string]bool{}
	satisfied := 0
	for _, s := range all {
		if s.satBy != "" {
			satisfied++
			continue
		}
		if _, ok := ewdExemptBinaries[s.binary]; ok {
			seenExempt[s.binary] = true
			continue
		}
		where := "sets no cmd.WaitDelay"
		if s.cmdVar == "" {
			where = "is chained onto its result, so it has no variable to set cmd.WaitDelay on"
		}
		t.Errorf("%s:%d runs a child under a context deadline and %s.\n"+
			"  A deadline alone bounds the CHILD's life, not this call: os/exec's Wait does not return while a grandchild holds the killed child's pipes (internal/posse/herdr.go, herdrWaitDelay). MEASURED: 120.16s against a 200ms context (ranger-base-2cttm).\n"+
			"  Set cmd.WaitDelay to a named grace beside the deadline — or, if the binary forks nothing, add it to ewdExemptBinaries with that reason, the way `ps` is.",
			s.file, s.line, where)
	}
	if satisfied == 0 {
		t.Error("not one site in the tree was read as naming its grace, and twelve did on 2026-10-31.\n" +
			"  Either the scan stopped resolving WaitDelay assignments or the tree really did lose them; a pin that excuses every site through the exempt roster and reports nothing satisfied is measuring the roster, not the tree.")
	}

	// And the ruling's own rows have to be real. A row for a binary no
	// shipped site runs any more is a door held open for nothing — and
	// because the row also licenses a CHAINED call, a stale one is the one
	// kind of exemption a reader cannot see from the call site.
	var keys []string
	for b := range ewdExemptBinaries {
		keys = append(keys, b)
	}
	sort.Strings(keys)
	for _, b := range keys {
		if !seenExempt[b] {
			t.Errorf("ewdExemptBinaries rules %q exempt (%s) and no shipped site runs it under a context any more — drop the row",
				b, ewdExemptBinaries[b])
		}
	}
}

// The two-way half: the scan has to be able to SAY NO. Three planted shapes,
// one per route a site can be satisfied by, run through the same reader —
// because every arm above is an assertion of absence, and an absence is what
// a reader that resolves nothing reports.
func TestQATheWaitDelayCensusCanStillSayNo(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		src  string
		want bool // true = the scan must find an unsatisfied site
	}{
		{"bare", `package x
import ("context"; "os/exec")
func f(ctx context.Context) { cmd := exec.CommandContext(ctx, "gh", "api"); _, _ = cmd.Output() }
`, true},
		{"direct assignment", `package x
import ("context"; "os/exec"; "time")
func f(ctx context.Context) { cmd := exec.CommandContext(ctx, "gh", "api"); cmd.WaitDelay = time.Second; _, _ = cmd.Output() }
`, false},
		{"through a binder", `package x
import ("context"; "os/exec")
func bindIt(cmd *exec.Cmd) { cmd.WaitDelay = 0 }
func f(ctx context.Context) { cmd := exec.CommandContext(ctx, "gh", "api"); bindIt(cmd); _, _ = cmd.Output() }
`, false},
		{"a helper that takes an *exec.Cmd and sets nothing is not a binder", `package x
import ("context"; "os/exec")
func logIt(cmd *exec.Cmd) { _ = cmd.Args }
func f(ctx context.Context) { cmd := exec.CommandContext(ctx, "gh", "api"); logIt(cmd); _, _ = cmd.Output() }
`, true},
		{"chained, no variable to set it on", `package x
import ("context"; "os/exec")
func f(ctx context.Context) { _, _ = exec.CommandContext(ctx, "gh", "api").Output() }
`, true},
		{"another function's grace is not this one's", `package x
import ("context"; "os/exec"; "time")
func g(ctx context.Context) { cmd := exec.CommandContext(ctx, "gh", "api"); cmd.WaitDelay = time.Second; _ = cmd }
func f(ctx context.Context) { cmd := exec.CommandContext(ctx, "gh", "run", "list"); _, _ = cmd.Output() }
`, true},
		{"through a pass-through wrapper, grace on the result", `package x
import ("context"; "os/exec"; "time")
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func f(ctx context.Context) { cmd := protocolCmd(exec.CommandContext(ctx, "gh", "api")); cmd.WaitDelay = time.Second; _, _ = cmd.Output() }
`, false},
		{"through a pass-through wrapper, no grace", `package x
import ("context"; "os/exec")
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func f(ctx context.Context) { cmd := protocolCmd(exec.CommandContext(ctx, "gh", "api")); _, _ = cmd.Output() }
`, true},
		{"a helper that does not hand the child back is not a pass-through", `package x
import ("context"; "os/exec"; "time")
func okCmd(c *exec.Cmd) bool { return c != nil }
func f(ctx context.Context) { ok := okCmd(exec.CommandContext(ctx, "gh", "api")); _ = ok; var cmd exec.Cmd; cmd.WaitDelay = time.Second }
`, true},
		{"set inside a closure the site is in", `package x
import ("context"; "os/exec"; "time")
var f = func(ctx context.Context) { cmd := exec.CommandContext(ctx, "gh", "api"); cmd.WaitDelay = time.Second; _, _ = cmd.Output() }
`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "x.go")
			if err := os.WriteFile(path, []byte(tc.src), 0o644); err != nil {
				t.Fatal(err)
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				t.Fatalf("parsing the fixture: %v", err)
			}
			files := map[string]*ast.File{path: f}
			sites, _ := ewdScan(fset, "x.go", f, ewdBinders(files), ewdPassThrough(files))
			var unmet int
			for _, s := range sites {
				if s.satBy == "" {
					unmet++
				}
			}
			if got := unmet > 0; got != tc.want {
				t.Errorf("unsatisfied sites = %d (want any = %v) over %d sites found; the census reads this shape wrong, so the arm above is measuring nothing",
					unmet, tc.want, len(sites))
			}
		})
	}
}
