package treepins

// QA census for ranger-base-s9h52 — EVERY wall-check member spends its one
// line on a cause, read as a census over the member set rather than one
// member at a time.
//
// THE CLASS, THREE TIMES BY HAND BEFORE THIS. `scripts/posse-wall-check`
// reports a failed member by `evidence()`: the first `file.go:NN:` line of
// that test's output which is not the venue marker, truncated to 300
// characters, and nothing after it (ADR 0097 D3). The launcher puts that one
// line on a P1 bead. So a member's budget is ONE line, and the two ways to
// spend it on nothing are:
//
//	ranger-base-a688i  a format string ending `:\n%s` whose `%s` is the only
//	                   account of WHY. The bead read `the seat ended before it
//	                   ran (exit 1):` and stopped there.
//	ranger-base-axt1f  a runner that reduces its child's error to `err == nil`
//	                   before any message is formatted. The exit status then
//	                   exists as text nowhere, so `exit status 128` and
//	                   `signal: killed` report identically and NO line budget
//	                   brings the difference back.
//
// Each of those three beads pinned ONE NAMED FILE, which is exactly why
// nothing read the other members: a named read is nobody's census. This pin
// reads the whole set.
//
// WHY THE RULE IS NARROW, AND WHAT IT IS NOT (the decision ranger-base-s9h52
// was filed to make: a priced exemption table, or a narrower rule). It is a
// NARROWER RULE, and there is no exemption table anywhere in this file.
//
// The wide rule — ranger-base-axt1f's own, which that bead applied to its one
// file — is "no format string in a member file carries a newline". MEASURED
// 2026-11-21 over the member set: 785 testing format calls in 24 member
// files, 204 of them carrying a newline. Narrowing it by PROSE SHAPE does not
// converge either: "the newline defers and the first line carries no format
// verb at all" reads 44, and those 44 still mix the class with messages whose
// first line is a complete claim about an artifact the reader can go and read
// (a render's argv, a verdict file, a bead's text). Pricing 44 rows by hand is
// a register that drifts, and every row of it would be a judgement about
// someone else's prose.
//
// So this census narrows by SUBJECT instead: it reads the functions that
// launch a bwrap SEAT, and the messages that carry a seat's account. That is
// the one place where the deferred value is the ONLY account of why — a
// child's status and words, unreproducible after the fact — and it is what
// both of the beads above actually hit. A message whose tail is an artifact
// the reader can re-read is out of scope and needs no row saying so, because
// the rule never reaches it.
//
// MEASURED 2026-11-21 (linux 7.2.15, go1.27.3, this tree): 24 member files,
// 30 bwrap-launching functions in them, 2 of which rendered no error at all —
// `bsLive.in` (bwrapseatstorelive_test.go) and `srLive.in`
// (storerepogitwalllive_test.go), byte-identical copies of the two runners
// ranger-base-axt1f fixed — and 19 messages reading their accounts after a
// newline. All of those are fixed in the landing that adds this file.
//
// IT ENUMERATES A DISCOVERED FILE SET (the Glob below), so it is in this
// package's one-package-census class and is doored by `make treepins-check` —
// the complement door, which is this package minus the priced table in
// treewidedoor_qa_test.go's `twdTreepinsExcluded` (ADR 0098 D1). It is in no
// such table, so it needs no hand-written Makefile entry and cannot go
// undoored (ADR 0098 D4).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// wmaCheck is the wall check, whose `member_pattern` is the only definition
// of the member set. Read, never retyped: a census whose subject set is a
// second spelling of that pattern is a census that can disagree with the
// thing it exists to serve.
const wmaCheck = "scripts/posse-wall-check"

// wmaPkgGlob is the discovered file set. internal/posse is the one package
// the check runs (`-run <member_pattern> ./internal/posse`).
const wmaPkgGlob = "internal/posse/*_test.go"

// wmaAccountFunc is the one spelling of a seat's one-line account
// (bwrapgitrefslive_test.go, ranger-base-axt1f): the exit status, folded,
// then the seat's words folded onto the same line by wwOneLine.
const wmaAccountFunc = "bgSeatAccount"

// wmaWaiters are the Cmd methods that produce the child's error. The error a
// bwrap-launching function must render is the one THESE return, and not
// merely some error somewhere in the body — a function that reports a
// fixture's failure and swallows its seat's is the shape this pin is for.
var wmaWaiters = map[string]bool{
	"CombinedOutput": true, "Output": true, "Run": true, "Start": true, "Wait": true,
}

// wmaSayers are the calls that put a value where a reader can see it. A
// testing method is matched by NAME on any receiver, because a fixture's
// helper takes `t` under whatever name it likes.
var wmaSayers = map[string]bool{
	"Fatal": true, "Fatalf": true, "Error": true, "Errorf": true,
	"Log": true, "Logf": true, "Skip": true, "Skipf": true,
	"Sprintf": true, "Sprint": true, "Errorf_": true,
	wmaAccountFunc: true,
}

// wmaFormatVerbs are the format-taking testing methods: the calls whose first
// argument is the format string a reader meets through evidence().
var wmaFormatVerbs = map[string]bool{"Fatalf": true, "Errorf": true, "Logf": true, "Skipf": true}

var wmaVerbRe = regexp.MustCompile(`%[-+# 0-9.*\[\]]*[a-zA-Z]`)

// wmaMemberPattern reads the member pattern out of the wall check. One line,
// one literal, and the check's own comment says it is written once there.
func wmaMemberPattern(t *testing.T) *regexp.Regexp {
	t.Helper()
	b, err := os.ReadFile(wmaCheck)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^member_pattern='([^']+)'$`).FindSubmatch(b)
	if m == nil {
		t.Fatalf("%s no longer carries a `member_pattern='…'` line, so this census cannot read the set it is a census of. That literal is the check's own definition of a member (ADR 0097 D4) and this pin reads it rather than retyping it.", wmaCheck)
	}
	re, cerr := regexp.Compile(string(m[1]))
	if cerr != nil {
		t.Fatalf("%s's member_pattern %q does not compile in Go: %v", wmaCheck, m[1], cerr)
	}
	return re
}

// wmaFile is one parsed member file.
type wmaFile struct {
	path string
	fset *token.FileSet
	ast  *ast.File
}

// wmaMemberFiles is the census's subject: every file in the package the check
// runs that declares at least one member. A Glob and not a list, which is
// what puts this pin in the one-package-census class.
func wmaMemberFiles(t *testing.T) []wmaFile {
	t.Helper()
	re := wmaMemberPattern(t)
	paths, err := filepath.Glob(wmaPkgGlob)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("%s matched no files — this census has nothing to read, so every arm below would pass for the wrong reason", wmaPkgGlob)
	}
	var out []wmaFile
	for _, p := range paths {
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", p, perr)
		}
		member := false
		for _, fn := range wmaDecls(f) {
			if fn.Recv == nil && re.MatchString(fn.Name.Name) {
				member = true
			}
		}
		if member {
			out = append(out, wmaFile{path: p, fset: fset, ast: f})
		}
	}
	return out
}

func wmaDecls(f *ast.File) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
			out = append(out, fn)
		}
	}
	return out
}

// wmaLaunchesSeat reports whether this function starts a bwrap seat.
func wmaLaunchesSeat(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext") {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "exec" {
			return true
		}
		for _, a := range c.Args {
			if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, uerr := strconv.Unquote(lit.Value); uerr == nil && s == "bwrap" {
					found = true
				}
			}
		}
		return true
	})
	return found
}

// wmaChildErrs are the names this function binds the SEAT's error to: the
// error result of a Cmd waiter. Those are the names that have to reach a
// reader.
func wmaChildErrs(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 {
			return true
		}
		c, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || !wmaWaiters[sel.Sel.Name] {
			return true
		}
		// The error is the LAST result of every waiter in wmaWaiters.
		if id, ok := as.Lhs[len(as.Lhs)-1].(*ast.Ident); ok && id.Name != "_" {
			names[id.Name] = true
		}
		return true
	})
	return names
}

// wmaSaid reports whether name is handed to something that shows it.
func wmaSaid(fn *ast.FuncDecl, name string) bool {
	said := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := ""
		switch f := c.Fun.(type) {
		case *ast.Ident:
			callee = f.Name
		case *ast.SelectorExpr:
			callee = f.Sel.Name
			// `err.Error()` is a rendering of err all by itself.
			if f.Sel.Name == "Error" {
				if id, ok := f.X.(*ast.Ident); ok && id.Name == name {
					said = true
				}
			}
		}
		if !wmaSayers[callee] {
			return true
		}
		for _, a := range c.Args {
			if id, ok := a.(*ast.Ident); ok && id.Name == name {
				said = true
			}
		}
		return true
	})
	return said
}

// wmaAccountLocals are the locals a call site binds to a seat runner's
// account — the LAST string result of a function that launches a seat. One
// hop, which is every spelling the member set uses and the same depth
// treewidedoor_qa_test.go's parameter hop reads.
func wmaAccountLocals(f wmaFile) map[string]bool {
	runners := map[string]bool{}
	for _, fn := range wmaDecls(f.ast) {
		if !wmaLaunchesSeat(fn) || fn.Type.Results == nil {
			continue
		}
		var res []string
		for _, r := range fn.Type.Results.List {
			n := 1
			if len(r.Names) > 1 {
				n = len(r.Names)
			}
			name := "?"
			if id, ok := r.Type.(*ast.Ident); ok {
				name = id.Name
			}
			for i := 0; i < n; i++ {
				res = append(res, name)
			}
		}
		if len(res) >= 2 && res[len(res)-1] == "string" {
			runners[fn.Name.Name] = true
		}
	}
	locals := map[string]bool{}
	if len(runners) == 0 {
		return locals
	}
	ast.Inspect(f.ast, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 || len(as.Lhs) < 2 {
			return true
		}
		c, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := ""
		switch fun := c.Fun.(type) {
		case *ast.Ident:
			callee = fun.Name
		case *ast.SelectorExpr:
			callee = fun.Sel.Name
		}
		if !runners[callee] {
			return true
		}
		if id, ok := as.Lhs[len(as.Lhs)-1].(*ast.Ident); ok && id.Name != "_" {
			locals[id.Name] = true
		}
		return true
	})
	return locals
}

// wmaFormat reconstructs a format string from a literal or a concatenation of
// literals, so a format spelled across a `+` is read too.
func wmaFormat(x ast.Expr) (string, bool) {
	switch e := x.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(e.Value)
		if err != nil {
			return "", false
		}
		return s, true
	case *ast.ParenExpr:
		return wmaFormat(e.X)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		l, ok := wmaFormat(e.X)
		if !ok {
			return "", false
		}
		r, ok := wmaFormat(e.Y)
		if !ok {
			return "", false
		}
		return l + r, true
	}
	return "", false
}

// ARM 1 — THE ACCOUNT. Every function in a member file that launches a bwrap
// seat renders that seat's error. This is ranger-base-axt1f's half: a runner
// that answers `err == nil` has destroyed the exit status before any message
// exists, and the line budget is then irrelevant.
func TestQAEveryWallMemberSeatLaunchRendersItsChildsError(t *testing.T) {
	t.Parallel()
	files := wmaMemberFiles(t)
	launchers, graded := 0, 0
	for _, f := range files {
		for _, fn := range wmaDecls(f.ast) {
			if !wmaLaunchesSeat(fn) {
				continue
			}
			launchers++
			errs := wmaChildErrs(fn)
			if len(errs) == 0 {
				// A launch whose waiter's error is dropped at the
				// assignment is the same hole one step earlier.
				t.Errorf("%s:%d: %s launches a bwrap seat and binds its waiter's error to nothing a reader can be shown. `scripts/posse-wall-check` carries a failed member's FIRST `file.go:NN:` line and nothing after it (ADR 0097 D3), so the status is the one row that discriminates `exit status 128` (the wall or the command) from `signal: killed` (the box) — and an error that is never bound cannot be rendered by any message. Build the seat's answer with %s, as bgLive.walled does.",
					f.path, f.fset.Position(fn.Pos()).Line, fn.Name.Name, wmaAccountFunc)
				continue
			}
			for name := range errs {
				graded++
				if wmaSaid(fn, name) {
					continue
				}
				t.Errorf("%s:%d: %s launches a bwrap seat and renders %s nowhere — it is reduced to a bool or dropped before any message is formatted. That is ranger-base-axt1f exactly: a git that exited 128 at `cannot lock ref` and a seat ended from outside then produce byte-identical reports, and the one line `scripts/posse-wall-check` carries to a P1 bead (ADR 0097 D3) cannot say which it was. No line budget brings a destroyed status back. Return %s(%s, out) beside the raw output, as bsLive.in and srLive.in do.",
					f.path, f.fset.Position(fn.Pos()).Line, fn.Name.Name, name, wmaAccountFunc, name)
			}
		}
	}
	// THE CONTROLS. Both directions are the bug: a census that found no
	// member files, or no seat launches in them, would pass having read
	// nothing — which is the failure mode an absence pin has (see this
	// package's absencerules_qa_test.go).
	if len(files) < 20 {
		t.Errorf("this census found %d member file(s) in %s, want at least 20 (24 on 2026-11-21) — the member pattern read from %s has stopped matching this package and the arms above graded almost nothing", len(files), wmaPkgGlob, wmaCheck)
	}
	if launchers < 25 {
		t.Errorf("this census found %d bwrap-launching function(s) across those files, want at least 25 (30 on 2026-11-21) — a seat is started some way this pin cannot see, so the class has members it is not reading", launchers)
	}
	if graded == 0 {
		t.Errorf("this census graded 0 child errors over %d launching function(s) — the waiter set %v no longer names how this package waits on a seat", launchers, wmaWaiters)
	}
}

// ARM 2 — THE LINE. A message carrying a seat's account carries it where the
// bead will read it. This is ranger-base-a688i's half, scoped to the one
// value that cannot be recovered after the fact.
//
// NO NEWLINE AT ALL in such a message, rather than "the account is before the
// first newline": everything after that newline is dropped in transit anyway
// (ADR 0097 D3), so a message that needs a tail is a message whose tail the
// bead will not get. That is the same rule ranger-base-axt1f pinned for its
// own file, here applied to every member file that has an account to carry.
func TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine(t *testing.T) {
	t.Parallel()
	files := wmaMemberFiles(t)
	withAccounts, read := 0, 0
	for _, f := range files {
		locals := wmaAccountLocals(f)
		if len(locals) == 0 {
			continue
		}
		withAccounts++
		ast.Inspect(f.ast, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) < 2 {
				return true
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok || !wmaFormatVerbs[sel.Sel.Name] {
				return true
			}
			text, ok := wmaFormat(c.Args[0])
			if !ok {
				return true
			}
			for _, a := range c.Args[1:] {
				id, ok := a.(*ast.Ident)
				if !ok || !locals[id.Name] {
					continue
				}
				read++
				if !strings.ContainsAny(text, "\n\r") {
					continue
				}
				i := strings.IndexAny(text, "\n\r")
				t.Errorf("%s:%d: this t.%s carries the seat's account %s in a format string that has a newline in it. `scripts/posse-wall-check` reports a failed member by its FIRST `file.go:NN:` line and nothing after it (ADR 0097 D3, 300 chars), and the launcher puts that one line on a P1 bead — so this message reaches the bead as %q and the account is dropped in transit. That is ranger-base-a688i and ranger-base-axt1f, both of which arrived as a symptom, a colon and nothing. Fold the message onto one line.",
					f.path, f.fset.Position(c.Pos()).Line, sel.Sel.Name, id.Name, text[:i])
			}
			return true
		})
	}
	// The controls, again both ways.
	if withAccounts < 3 {
		t.Errorf("this arm found seat accounts in %d member file(s), want at least 3 (3 on 2026-11-21: bwrapgitrefslive_test.go, bwrapseatstorelive_test.go, storerepogitwalllive_test.go) — the runner shape this arm reads has changed and it is grading almost nothing", withAccounts)
	}
	if read < 15 {
		t.Errorf("this arm read %d message(s) interpolating a seat account, want at least 15 (24 on 2026-11-21) — an account reaches a reader by some route this arm cannot see", read)
	}
}

// ARM 3 — THE RULES SAY NO. Both rules driven over planted source, each with
// its own negative case, through the SAME functions the two arms above read
// the tree with. A rule that quietly stopped meaning what this file says it
// means reds here with no bwrap, no venue and no member set.
func TestQATheWallMemberAccountRulesMatchWhatTheyClaim(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name            string
		src             string
		launches, rends bool
	}{
		{"a seat runner that renders its status", `func run(t *testing.T) (bool, string) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	return err == nil, bgSeatAccount(err, string(out))
}`, true, true},
		{"the ranger-base-axt1f shape, reduced to a bool", `func run(t *testing.T) (bool, string) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	return err == nil, strings.TrimSpace(string(out))
}`, true, false},
		{"a seat run whose error reaches a fatal", `func run(t *testing.T) {
	cmd := exec.Command("bwrap", "x")
	if err := cmd.Run(); err != nil {
		t.Fatalf("the seat refused: %v", err)
	}
}`, true, true},
		{"a seat run whose error is dropped at the assignment", `func run(t *testing.T) {
	cmd := exec.Command("bwrap", "x")
	out, _ := cmd.CombinedOutput()
	t.Logf("%s", out)
}`, true, false},
		{"a seat run that says a DIFFERENT error and swallows its own", `func run(t *testing.T) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	if ferr := os.WriteFile("x", out, 0o644); ferr != nil {
		t.Fatalf("the fixture: %v", ferr)
	}
	_ = err
}`, true, false},
		{"a run of something that is not a seat", `func run(t *testing.T) {
	cmd := exec.Command("git", "status")
	out, _ := cmd.CombinedOutput()
	t.Logf("%s", out)
}`, false, false},
		{"a seat started through CommandContext", `func run(t *testing.T, ctx context.Context) {
	cmd := exec.CommandContext(ctx, "bwrap", "x")
	if err := cmd.Start(); err != nil {
		t.Fatalf("the seat never started: %v", err)
	}
}`, true, true},
		{"an err.Error() is a rendering of its own", `func run(t *testing.T) string {
	cmd := exec.Command("bwrap", "x")
	err := cmd.Run()
	if err != nil {
		return err.Error()
	}
	return ""
}`, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			fn, _ := wmaFixture(t, c.src, "run")
			if got := wmaLaunchesSeat(fn); got != c.launches {
				t.Fatalf("wmaLaunchesSeat = %v, want %v — the seat-launch key no longer selects what this file says it selects:\n%s", got, c.launches, c.src)
			}
			if !c.launches {
				return
			}
			errs := wmaChildErrs(fn)
			rendered := false
			for name := range errs {
				if wmaSaid(fn, name) {
					rendered = true
				}
			}
			if rendered != c.rends {
				t.Errorf("the child-error reading says rendered=%v, want %v (bound names %v) — arm 1's key no longer means what this file says it means:\n%s", rendered, c.rends, errs, c.src)
			}
		})
	}

	// And arm 2's one hop, the same way: a local bound to a runner's account
	// is found, and a local bound to something else is not.
	for _, c := range []struct {
		name string
		src  string
		want []string
	}{
		{"the account local of a three-result runner", `func (f fx) in(t *testing.T) (bool, string, string) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	return err == nil, string(out), bgSeatAccount(err, string(out))
}
func TestX(t *testing.T) { ok, out, acct := f.in(t); _, _, _ = ok, out, acct }`, []string{"acct"}},
		{"the account local of a two-result runner", `func (f fx) in(t *testing.T) (bool, string) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	return err == nil, bgSeatAccount(err, string(out))
}
func TestX(t *testing.T) { ok, acct := f.in(t); _, _ = ok, acct }`, []string{"acct"}},
		{"a blanked account binds nothing", `func (f fx) in(t *testing.T) (bool, string) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	return err == nil, bgSeatAccount(err, string(out))
}
func TestX(t *testing.T) { ok, _ := f.in(t); _ = ok }`, nil},
		{"a helper that launches no seat is not a runner", `func (f fx) in(t *testing.T) (bool, string) {
	cmd := exec.Command("git", "status")
	out, err := cmd.CombinedOutput()
	return err == nil, string(out)
}
func TestX(t *testing.T) { ok, acct := f.in(t); _, _ = ok, acct }`, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, f := wmaFixture(t, c.src, "")
			got := wmaAccountLocals(f)
			if len(got) != len(c.want) {
				t.Fatalf("wmaAccountLocals found %v, want %v — arm 2's one hop no longer binds what this file says it binds:\n%s", got, c.want, c.src)
			}
			for _, w := range c.want {
				if !got[w] {
					t.Errorf("wmaAccountLocals found %v, want it to name %q:\n%s", got, w, c.src)
				}
			}
		})
	}

	// The format reader, including the shape a `+` concatenation hides a
	// newline in: a format spelled in two literals is one message.
	for _, c := range []struct {
		src  string
		want string
		ok   bool
	}{
		{`"one line"`, "one line", true},
		{`"first:\n" + "second"`, "first:\nsecond", true},
		{"`a raw\nstring`", "a raw\nstring", true},
		{`"a " + name`, "", false},
	} {
		expr, perr := parser.ParseExpr(c.src)
		if perr != nil {
			t.Fatalf("parse %s: %v", c.src, perr)
		}
		got, ok := wmaFormat(expr)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("wmaFormat(%s) = %q,%v want %q,%v", c.src, got, ok, c.want, c.ok)
		}
	}
}

// wmaFixture parses one planted snippet and hands back the named func (when
// name is set) and the file. It goes through the same parser the tree arms
// use, so a fixture cannot pass against a second, friendlier copy of a rule.
func wmaFixture(t *testing.T, src, name string) (*ast.FuncDecl, wmaFile) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture_test.go", "package posse\n"+src+"\n", 0)
	if err != nil {
		t.Fatalf("parse fixture: %v\n%s", err, src)
	}
	f := wmaFile{path: "fixture_test.go", fset: fset, ast: file}
	if name == "" {
		return nil, f
	}
	for _, fn := range wmaDecls(file) {
		if fn.Name.Name == name {
			return fn, f
		}
	}
	t.Fatalf("fixture declares no %s:\n%s", name, src)
	return nil, f
}

// ARM 4 — the member pattern this census reads is the one the check runs.
// Read back as a REGEXP against the real member names, because a pattern that
// compiled but matched nothing would leave every arm above green over an
// empty set.
func TestQATheWallMemberCensusReadsTheChecksOwnPattern(t *testing.T) {
	t.Parallel()
	re := wmaMemberPattern(t)
	for _, name := range []string{
		"TestLiveWallWatchKillsASeatWhoseWallComesOffAndFilesOneBead",
		"TestLiveBwrapGitRefsCommitsAfterAnOutsidePackRefs",
		"TestQALiveWallSomethingNobodyHasWrittenYet",
	} {
		if !re.MatchString(name) {
			t.Errorf("the pattern read from %s does not match %q, which is the shape of a wall-check member", wmaCheck, name)
		}
	}
	// And it refuses what is not one. bwrapgitrefsaccount_qa_test.go's own
	// pins are deliberately NOT named like members: they measure no wall.
	for _, name := range []string{
		"TestQABwrapGitRefsSeatAccountNamesItsStatusOnOneLine",
		"TestQAWallWatchFixtureFoldsWhatTheWatcherSaidOntoOneLine",
		"TestLiveSomethingThatIsNoWall",
	} {
		if re.MatchString(name) {
			t.Errorf("the pattern read from %s matches %q, which is not a wall-check member — the set this census reads has widened past the set the check measures", wmaCheck, name)
		}
	}
}
