package treepins

// QA witness for ranger-base-rc63f, verifying ranger-base-2cttm: three
// shapes the WaitDelay census (execwaitdelay_qa_test.go,
// TestQAEveryBoundedChildNamesItsWaitDelay) reads as SATISFIED while the
// site names no grace of its own.
//
// It ships GREEN, asserting the hole, because that is the only shape of
// witness that survives the fix: a `t.Skip` dies silently and an assertion
// of the right answer would red the suite over a defect nobody has agreed
// to fix yet. Each arm's failure message spells the INVERSION — when the
// resolver is corrected the arm reds, and the fix is to flip `wantUnmet`
// for that row and delete this comment's row from the list below.
//
// WHY A WITNESS AND NOT A FIX. The census resolves a site against the
// satisfiers by IDENTIFIER and by enclosing FUNCTION (ewdScan's final
// loop), which is the right call for the tree it was written over —
// internal/posse/ciwatch.go has three children all called `cmd`, in three
// functions. The three shapes below are where that resolution is coarser
// than the rule it enforces, and narrowing it is a change to what the pin
// MEANS (per-site reaching rather than per-name-per-function), with its own
// mutation pass. That is the code lane's, not this file's.
//
// MEASURED 2026-10-31 at a72dfaf8, by running ewdBinders/ewdScan over these
// exact fixtures: 3 of 8 hostile shapes resolve as satisfied with no grace
// at the site. The other five fail CLOSED and are kept below as the control
// — an arm that reports a hole has to be read beside the arms that report
// none, or it is measuring a reader that resolves nothing.
//
// NOT REACHED BY ANY SITE IN THE TREE TODAY, and the margin is one
// identifier wide. At this commit the census reads 15 exec.CommandContext
// sites, 12 naming a grace and 3 ruled exempt. ONE shipped function already
// holds two of them — internal/posse/worktree.go:2966 and :2967, the
// `git log` and `git patch-id` halves of patchIDsVerbatimWithin — and it is
// safe only because they are bound to DISTINCT identifiers (logCmd, idCmd)
// and each is handed to bindGitDeadline separately. Give both the name
// `cmd`, or add a third child to any of the twelve functions and reuse its
// variable, and the extra site stops being censused while the pin stays
// green. So this is a hole in the pin rather than a live unbounded call,
// and it is not a hypothetical shape: the tree is already one rename away
// from it.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

// ewdhCase is one planted shape and what the census says about it TODAY.
type ewdhCase struct {
	name      string
	src       string
	wantUnmet bool   // what the census reports now
	should    string // what it would report if it resolved the rule exactly
}

var ewdhCases = []ewdhCase{
	{
		name: "HOLE two sites in one function share an identifier and one grace",
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
		wantUnmet: false,
		should:    "report the SECOND site, which names no grace: one assignment to `cmd` satisfies every site in the function bound to `cmd`, whichever side of it they sit",
	},
	{
		name: "HOLE an outer grace satisfies a site in a closure that shadows the name",
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
		wantUnmet: false,
		should:    "report the site in the closure: the satisfier's scope ENCLOSES that site's function, which is the clause that lets a wrapper's grace count, and a shadowed `cmd` is a different variable",
	},
	{
		name: "HOLE a binder that sets WaitDelay on one of its two *exec.Cmd parameters",
		src: `package x
import ("context"; "os/exec"; "time")
func bindIt(a, b *exec.Cmd) { a.WaitDelay = time.Second }
func f(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "gh", "api")
	bindIt(nil, cmd)
	_, _ = cmd.Output()
}
`,
		wantUnmet: false,
		should:    "report the site: the binder derivation is keyed by FUNCTION NAME, and a call to it marks every identifier argument satisfied, so a cmd passed in the parameter the binder never touches reads as bound",
	},
	// The control: five shapes that fail CLOSED today. They are what makes
	// the three rows above a reading of the resolver rather than of a
	// reader that resolves nothing.
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
		wantUnmet: true,
		should:    "report the second site",
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
		wantUnmet: true,
		should:    "report the site (the binding is invisible to the scan, so it reads as chained — fail-closed)",
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
		wantUnmet: true,
		should:    "report the site — fail-closed",
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
		wantUnmet: true,
		should:    "report the site: a grace on a field is not read as a satisfier, so the site is reported although it HAS one — fail-closed, and the direction to keep if this row is ever revisited",
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
		wantUnmet: true,
		should:    "report the site: argv[0] is read only as a string LITERAL, so the `ps` exemption does not reach a constant — fail-closed",
	},
}

func TestQATheWaitDelayCensusResolvesByNameAndFunction(t *testing.T) {
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
			binders := ewdBinders(map[string]*ast.File{path: f})
			sites, _ := ewdScan(fset, "x.go", f, binders)
			if len(sites) == 0 {
				t.Fatalf("the scan found no exec.CommandContext site in this fixture — it is reading neither the hole nor the control")
			}
			var unmet int
			for _, s := range sites {
				if s.satBy == "" {
					unmet++
				}
			}
			if got := unmet > 0; got != tc.wantUnmet {
				t.Errorf("the census now reports %d unsatisfied site(s) of %d here, and this arm was written when it reported %v.\n"+
					"  What it SHOULD report: %s.\n"+
					"  If the resolver was narrowed to close this (ranger-base-rc63f, from ranger-base-2cttm), flip wantUnmet for this row — do not delete the row, it is the witness that the narrowing reached this shape.",
					unmet, len(sites), tc.wantUnmet, tc.should)
			}
		})
	}
}
