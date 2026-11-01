package treepins

// QA witness for ranger-base-rc63f FINDING 1, verifying ranger-base-2cttm
// and now pinning its fix (ranger-base-tpy6q): WHICH site a grace reaches
// in the WaitDelay census (execwaitdelay_qa_test.go,
// TestQAEveryBoundedChildNamesItsWaitDelay).
//
// WHAT IT WAS. The census resolved a site against the satisfiers by
// IDENTIFIER and by enclosing FUNCTION, and three shapes read as SATISFIED
// with no grace at the site: two children in one function bound to the same
// identifier with one assignment between them (both satisfied, whichever
// side of it they sat); a site in a closure that SHADOWS the name (the outer
// assignment's scope encloses the closure); and a cmd handed to a binder in
// the parameter that binder never touches (the derivation keyed on the
// function NAME, and a call marked every identifier argument satisfied).
// MEASURED 2026-10-31 at a72dfaf8 by running ewdBinders/ewdScan over these
// exact fixtures: 3 of 8 hostile shapes satisfied with no grace at the site.
//
// The file shipped GREEN then, asserting the hole, because that is the only
// shape of witness that survives the fix — and its instruction was to FLIP
// the rows rather than delete them when the resolver was narrowed. That is
// what happened: ewdScan resolves per site now (same identifier, the same
// INNERMOST function exactly, and after the site but before the next child
// on that identifier), the three rows below read the right site, and they
// are kept as the witness that the narrowing reached these three shapes and
// not merely something near them. The file keeps its name for the same
// reason: it is the record of what the holes were.
//
// WHY THE ROWS ARE LINES AND NOT A COUNT. A per-site rule is a claim about
// WHICH child is unbound, and "some site in this fixture is unsatisfied" is
// satisfied by a reader that reports the wrong one — the hole in shape (a)
// was exactly a reading that got the count right over two sites and the
// site wrong. Each row names every line the census reports, in source
// order, and the site total beside it, so a scan that stopped seeing a site
// cannot read as a clean one.
//
// THE CONTROL half is the five shapes that failed closed before the fix and
// still do, plus nine added with it: a binder call that satisfies the
// parameter it DOES touch; a method, whose name is not one a plain call can
// reach; an enclosing grace below a shadowed site; a grace inside a closure
// the site is outside of; a grace above its site; a grace below BOTH
// children; and the shipped two-children-in-one-function shape three ways
// (distinct identifiers with a binder call on each, one identifier with a
// binder call on each, and that same rename with the second call missing).
// An arm that reports a hole has to be read beside arms that report none, or
// it is measuring a reader that resolves nothing.
//
// MUTATION-CHECKED BOTH WAYS, MEASURED 2026-10-31 (ranger-base-tpy6q): each
// of the seven ways to put the old reading back reds this test and names its
// own row — the ordering dropped (3 rows), the next-child limit dropped (1),
// the scope test relaxed to "or a function that encloses the site" (1) or to
// "anywhere inside the site's own function" (1), the binder's parameter
// position ignored (1), methods registered as binders (1) — and the corpus
// narrowed back to a package directory reds the census's own root arm. One
// row below is deliberately over-determined and reds under none of them: the
// closure-shadow HOLE as it was measured, which two of the conditions close.
//
// THE MARGIN IS THE TREE'S, NOT A FIXTURE'S. internal/posse/worktree.go:2966
// and :2967 are two exec.CommandContext sites in ONE function — the `git
// log` and `git patch-id` halves of patchIDsVerbatimWithin — and under the
// old rule they were censused correctly only because they carry DISTINCT
// identifiers (logCmd, idCmd) with a separate bindGitDeadline call on each.
// MEASURED 2026-10-31 (ranger-base-tpy6q) by renaming both to `cmd` in the
// shipped tree (`cmd :=` then `cmd =`, which is what that rename compiles
// to): the old per-name-per-function census stayed GREEN, and the narrowed
// one reports internal/posse/worktree.go:2966 — the FIRST site, because both
// bindGitDeadline calls sit below both children, so the child the first `cmd`
// held is overwritten before anything binds it and the only grace that
// reaches it is the one nobody wrote. Shape (a) was never waiting on a
// landing that merges two functions; the tree was one rename away from it.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// ewdhCase is one planted shape and the census's whole reading of it: the
// number of exec.CommandContext sites found, and the LINE of each site that
// names no grace, in source order.
type ewdhCase struct {
	name  string
	src   string
	sites int
	unmet []int  // nil = every site in the fixture names a grace
	why   string // what this row is holding the resolver to
}

var ewdhCases = []ewdhCase{
	{
		name: "two sites in one function share an identifier and one grace",
		src: `package x
import ("context"; "os/exec"; "time")
func f(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "gh", "api")
	cmd.WaitDelay = time.Second
	_, _ = cmd.Output()
	cmd = exec.CommandContext(ctx, "gh", "run", "list")
	_, _ = cmd.Output()
}
`,
		sites: 2,
		unmet: []int{7},
		why: "the SECOND child names no grace, and one assignment to `cmd` must not satisfy every site in the function bound to `cmd`." +
			" WAS A HOLE until ranger-base-tpy6q: both sites read satisfied, whichever side of the assignment they sat",
	},
	{
		name: "an outer grace does not reach a site in a closure that shadows the name",
		src: `package x
import ("context"; "os/exec"; "time")
func f(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "gh", "api")
	cmd.WaitDelay = time.Second
	_, _ = cmd.Output()
	go func() {
		cmd := exec.CommandContext(ctx, "gh", "run", "list")
		_, _ = cmd.Output()
	}()
}
`,
		sites: 2,
		unmet: []int{8},
		why: "the shadowed `cmd` in the closure is a different variable and a different child." +
			" WAS A HOLE until ranger-base-tpy6q: the satisfier's scope ENCLOSED the closure, which was the clause that let a wrapper's grace count." +
			" As written, TWO of the narrowed conditions close it — the outer grace is in another function AND above the site — so the row below carries the scope half on its own",
	},
	{
		name: "a binder satisfies only the parameter it touches",
		src: `package x
import ("context"; "os/exec"; "time")
func bindIt(a, b *exec.Cmd) { a.WaitDelay = time.Second }
func f(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "gh", "api")
	bindIt(nil, cmd)
	_, _ = cmd.Output()
}
`,
		sites: 1,
		unmet: []int{5},
		why: "bindIt assigns on its FIRST parameter and this cmd is handed to the second." +
			" WAS A HOLE until ranger-base-tpy6q: the derivation keyed on the function NAME, so a call to it marked every identifier argument satisfied",
	},
	{
		name: "CONTROL a binder satisfies the parameter it does touch",
		src: `package x
import ("context"; "os/exec"; "time")
func bindIt(a, b *exec.Cmd) { a.WaitDelay = time.Second }
func f(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "gh", "api")
	bindIt(cmd, nil)
	_, _ = cmd.Output()
}
`,
		sites: 1,
		unmet: nil,
		why:   "the mirror of the row above: keying by parameter position has to still SATISFY the position the binder assigns on, or the narrowing bought a pin that reports every binder call",
	},
	{
		name: "CONTROL an enclosing grace BELOW a shadowed site does not reach it",
		src: `package x
import ("context"; "os/exec"; "time")
func f(ctx context.Context) {
	go func() {
		cmd := exec.CommandContext(ctx, "gh", "run", "list")
		_, _ = cmd.Output()
	}()
	cmd := exec.CommandContext(ctx, "gh", "api")
	cmd.WaitDelay = time.Second
	_, _ = cmd.Output()
}
`,
		sites: 2,
		unmet: []int{5},
		why: "the scope half of the rule, alone: the enclosing grace sits BELOW the shadowed site, so ordering does not refuse it and only 'the same innermost function, exactly' does." +
			" MEASURED 2026-10-31 (ranger-base-tpy6q): relaxing the scope test back to 'or a function that encloses the site' reds this row and NO other, which is why it is here and not folded into the closure row above",
	},
	{
		name: "CONTROL a grace inside a closure does not reach the site outside it",
		src: `package x
import ("context"; "os/exec"; "time")
func f(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "gh", "api")
	go func() { cmd.WaitDelay = time.Second }()
	_, _ = cmd.Output()
}
`,
		sites: 1,
		unmet: []int{4},
		why:   "the other direction of the same rule: a closure is a function, and a grace set inside one is not read as this site's — fail-closed, reported although it HAS one",
	},
	{
		name: "CONTROL a grace after the second child is not the first child's",
		src: `package x
import ("context"; "os/exec"; "time")
func f(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "gh", "api")
	cmd = exec.CommandContext(ctx, "gh", "run", "list")
	cmd.WaitDelay = time.Second
	_, _ = cmd.Output()
}
`,
		sites: 2,
		unmet: []int{4},
		why:   "the second half of the ordering rule: the next child on the identifier ENDS the first site's reach, so a grace below both of them belongs to the second one only",
	},
	{
		name: "CONTROL a method is not a binder a plain call can reach",
		src: `package x
import ("context"; "os/exec"; "time")
type h struct{}
func (x *h) bindIt(cmd *exec.Cmd) { cmd.WaitDelay = time.Second }
func bindIt(cmd *exec.Cmd) { _ = cmd.Args }
func f(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "gh", "api")
	bindIt(cmd)
	_, _ = cmd.Output()
}
`,
		sites: 1,
		unmet: []int{7},
		why: "the binder derivation is keyed by name, and a METHOD's name lives in its type's namespace: a call to one is a SelectorExpr ewdScan never reads as a binder call," +
			" so registering the method could only ever satisfy this same-named plain function's call — which binds nothing",
	},
	{
		name: "CONTROL a grace before the site does not reach it",
		src: `package x
import ("context"; "os/exec"; "time")
func f(ctx context.Context) {
	cmd := new(exec.Cmd)
	cmd.WaitDelay = time.Second
	cmd = exec.CommandContext(ctx, "gh", "api")
	_, _ = cmd.Output()
}
`,
		sites: 1,
		unmet: []int{6},
		why:   "source order is the whole of the reaching analysis, so a grace above its site (a loop body that binds the child at the bottom) is reported — fail-closed, and the direction to keep if this row is ever revisited",
	},
	{
		name: "CONTROL two children in one function, distinct identifiers, a binder call on each",
		src: `package x
import ("context"; "os/exec"; "time")
func bindIt(ctx context.Context, cmd *exec.Cmd) { cmd.WaitDelay = time.Second }
func f(ctx context.Context) {
	logCmd := exec.CommandContext(ctx, "git", "log")
	idCmd := exec.CommandContext(ctx, "git", "patch-id")
	bindIt(ctx, logCmd)
	bindIt(ctx, idCmd)
	_, _ = logCmd.Output()
	_, _ = idCmd.Output()
}
`,
		sites: 2,
		unmet: nil,
		why:   "internal/posse/worktree.go:2966-2967 reduced to a fixture — the shipped shape the margin above is about, and the two binder calls interleave with neither site",
	},
	{
		name: "CONTROL two children in one function on ONE identifier, a binder call on each",
		src: `package x
import ("context"; "os/exec"; "time")
func bindIt(ctx context.Context, cmd *exec.Cmd) { cmd.WaitDelay = time.Second }
func f(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "git", "log")
	bindIt(ctx, cmd)
	_, _ = cmd.Output()
	cmd = exec.CommandContext(ctx, "git", "patch-id")
	bindIt(ctx, cmd)
	_, _ = cmd.Output()
}
`,
		sites: 2,
		unmet: nil,
		why:   "the same rename the margin above measures, done CORRECTLY: reuse of the identifier is not the defect, so a rule that reports this one is over-reporting rather than narrowed",
	},
	{
		name: "CONTROL the same rename with no second grace",
		src: `package x
import ("context"; "os/exec"; "time")
func bindIt(ctx context.Context, cmd *exec.Cmd) { cmd.WaitDelay = time.Second }
func f(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "git", "log")
	bindIt(ctx, cmd)
	_, _ = cmd.Output()
	cmd = exec.CommandContext(ctx, "git", "patch-id")
	_, _ = cmd.Output()
}
`,
		sites: 2,
		unmet: []int{8},
		why:   "and the rename done WRONG, which is the live hazard in one fixture: the first child's binder call must not reach the second child",
	},
	{
		name: "CONTROL two sites in one function with distinct identifiers",
		src: `package x
import ("context"; "os/exec"; "time")
func f(ctx context.Context) {
	a := exec.CommandContext(ctx, "gh", "api")
	a.WaitDelay = time.Second
	_, _ = a.Output()
	b := exec.CommandContext(ctx, "gh", "run", "list")
	_, _ = b.Output()
}
`,
		sites: 2,
		unmet: []int{7},
		why:   "the identifier half, which never had a hole",
	},
	{
		name: "CONTROL a var declaration rather than an assignment",
		src: `package x
import ("context"; "os/exec")
func f(ctx context.Context) {
	var cmd = exec.CommandContext(ctx, "gh", "api")
	_, _ = cmd.Output()
}
`,
		sites: 1,
		unmet: []int{4},
		why:   "the binding is invisible to the scan, so it reads as chained — fail-closed",
	},
	{
		name: "CONTROL bound to a struct field",
		src: `package x
import ("context"; "os/exec")
type h struct{ cmd *exec.Cmd }
func (x *h) f(ctx context.Context) {
	x.cmd = exec.CommandContext(ctx, "gh", "api")
	_, _ = x.cmd.Output()
}
`,
		sites: 1,
		unmet: []int{5},
		why:   "fail-closed",
	},
	{
		name: "CONTROL a grace set through a field selector",
		src: `package x
import ("context"; "os/exec"; "time")
type h struct{ cmd *exec.Cmd }
func (x *h) f(ctx context.Context) {
	x.cmd = exec.CommandContext(ctx, "gh", "api")
	x.cmd.WaitDelay = time.Second
	_, _ = x.cmd.Output()
}
`,
		sites: 1,
		unmet: []int{5},
		why:   "a grace on a field is not read as a satisfier, so the site is reported although it HAS one — fail-closed, and the direction to keep if this row is ever revisited",
	},
	{
		name: "CONTROL an exempt binary reached through a constant",
		src: `package x
import ("context"; "os/exec")
const psBin = "ps"
func f(ctx context.Context) {
	out, _ := exec.CommandContext(ctx, psBin, "-ax").Output()
	_ = out
}
`,
		sites: 1,
		unmet: []int{5},
		why:   "argv[0] is read only as a string LITERAL, so the `ps` exemption does not reach a constant — fail-closed",
	},
}

func TestQATheWaitDelayCensusResolvesPerSite(t *testing.T) {
	t.Parallel()
	for _, tc := range ewdhCases {
		tc := tc
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
			if len(sites) != tc.sites {
				t.Fatalf("the scan found %d exec.CommandContext site(s) in this fixture and the row names %d — it is reading neither the rule nor the control here",
					len(sites), tc.sites)
			}
			var unmet []int
			for _, s := range sites {
				if s.satBy == "" {
					unmet = append(unmet, s.line)
				}
			}
			if !reflect.DeepEqual(unmet, tc.unmet) {
				t.Errorf("the census reports no grace at line(s) %v of this fixture, and this row says %v.\n"+
					"  What the row holds the resolver to: %s.\n"+
					"  Three rows here were HOLES — the census read them as satisfied with no grace at the site — until ranger-base-tpy6q narrowed ewdScan to per-site reaching (ranger-base-rc63f FINDING 1). They are flipped and kept as the witness that the narrowing reached these shapes. If this arm moved, the resolver moved: widen it back only deliberately, and never by deleting a row.",
					unmet, tc.unmet, tc.why)
			}
		})
	}
}
