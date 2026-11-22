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
// WHAT A LAUNCH IS SPELLED LIKE, which this census got wrong once
// (ranger-base-hhps5, found by ranger-base-t0ha1's verify). The key read the
// functions that hand `exec.Command`/`CommandContext` a LITERAL `"bwrap"`
// ARGUMENT, and the prose above states no such limit — so a seat launched
// through a shell or a pty wrapper, with the word inside the command STRING,
// was invisible, and the `launchers` floor could not say so: an invisible
// launcher does not lower the count of the visible ones. MEASURED 2026-11-22
// at 2866fdf4 by a planted member file and its control, the same probe twice
// and the only difference the spelling of the argv —
// `exec.Command("bwrap", "--dev-bind", "/", "/", "/bin/true")` RED,
// `exec.Command("sh", "-c", "bwrap --dev-bind / / /bin/true")` GREEN. That
// control is what made the green a reading: arm 1 did see a planted launcher.
// The key is now the bwrap command WORD wherever it reaches the call's
// arguments, one hop included (wmaSeatLaunches), and there were already two
// such launches in the member set: grokleader_qa_test.go's pty arm, which
// joins the render's argv into a line for `script -qfec`, and
// wallunixlive_test.go's `sh -c 'bwrap "$@"; :'` arm. Both render their
// seat's error, so the widening reds nothing — it closes the route the next
// one would have escaped by. Arm 3 carries the planted pair and its negative
// cases (`bwraptest`, `bwrap-wrapper`, `bwrap.sh` and a skip message that
// says the word are none of them launches); arm 1 carries a floor on the
// wrapper route.
//
// THE OVER-READ DIRECTION IS ARM 3'S, AND ONLY ARM 3'S (ranger-base-8752f,
// from ranger-base-udagn's verify of the widening). A ceiling on the wrapper
// count — `len(wrapped) > 8` — used to sit beside that floor here, on the
// reasoning that widening a key is how a census starts grading functions that
// launch nothing. It could not perform the detection its own message claimed.
// MEASURED 2026-11-22 at 330d6940, one mutant at a time: the whole ceiling
// block deleted reds nothing, and `wmaSeatWord` rewritten as a
// `strings.Contains` — the canonical over-read, and exactly the shape the
// ceiling's message described — still reads 2 of 32 through the wrapper route,
// the same count as the unmutated key. It cannot: the neighbour names that an
// over-reading key would newly match (`bwraptest`, `bwrap-wrapper`,
// `bwrap.sh`) live in arm 3's fixtures and in no member file, so no value of
// that bound distinguishes the two keys over this tree. The SAME mutant reds
// arm 3's neighbour row, and nowhere else — so the claim is held hermetically,
// by the arm this comment already called "the precise half of this", and the
// census logs its own counts instead of asserting a bound nothing witnesses.
//
// MEASURED 2026-11-21 (linux 7.2.15, go1.27.3, this tree): 24 member files,
// 30 bwrap-launching functions in them, 2 of which rendered no error at all —
// `bsLive.in` (bwrapseatstorelive_test.go) and `srLive.in`
// (storerepogitwalllive_test.go), byte-identical copies of the two runners
// ranger-base-axt1f fixed — and 19 messages reading their accounts after a
// newline. All of those are fixed in the landing that adds this file.
// RE-MEASURED 2026-11-22 (same box, go1.27.3) with the widened key: 24 member
// files, 32 launching functions, 31 of them handing a literal `bwrap` argv
// entry and 2 taking the wrapper route (one does both), 33 child errors
// graded. The +1 over the argv-entry reading is grokleader_qa_test.go's
// function, which this census could not see at all before.
//
// THE LINE RULE IS POSITIONAL, AND THE ACCOUNT KEY READS THE FOLD
// (ranger-base-momix, from ranger-base-0b6zo's verify of ranger-base-mklu0).
// Arm 2 used to say NO NEWLINE AT ALL in a message carrying a seat's account,
// and its report said of a message that had one that "the account is dropped
// in transit". Both were wrong, and the only reason the tree was green is that
// the key read almost nothing: wmaAccountLocals recognised an account solely
// as a local assigned from a multi-return seat-LAUNCHING FuncDecl whose last
// result is a string, which is 3 of the 24 member files — and NOT
// internal/posse/wallclaudeprojectsattack_qa_test.go, the file where this
// class has now been found twice (ranger-base-eheo3's close, then
// ranger-base-mklu0), because its runner is a closure with one result and its
// account comes from wwOneLine and from an inline bgSeatAccount(...) argument.
// Neither spelling is an Ident assigned from a FuncDecl, so arm 2 read
// neither.
//
// What `scripts/posse-wall-check`'s evidence() carries is the FIRST
// `file.go:NN:` line and nothing after it (ADR 0097 D3) — so a newline costs
// the account nothing when the account is in FRONT of it, and D3 and
// ranger-base-mklu0's own fix shape both prescribe exactly that: the account
// first, the cwd and the reasoning on line 2, "where D3 says supporting prose
// costs nothing" (docs/notes.d/ranger-base-mklu0.md).
//
// MEASURED 2026-11-22 at e5221de6, which is what decided this and not the
// prose: with the key widened to read the fold, 8 of 24 member files carry an
// account and 65 messages read one — against 3 files and 32 messages for the
// old key on that same tip, so the widening roughly doubles what arm 2 grades.
// FOUR of those 65 carry a newline, and every one of them puts the account in
// front of it — wallclaudeprojectsattack_qa_test.go:386, the line mklu0
// landed on purpose, and wallunixlive_test.go:237, :526 and :1130, whose
// `the seat exited badly: %s\nargv: bwrap %s` is three more of the same shape
// from a different lineage. NOTHING in the member set puts an account after a
// newline. So the old rule was never a strictness the tree was paying for: it
// was a rule that could only red the shape the house prescribes, held off by a
// key too narrow to reach any of the four — and the three files it did reach
// folded everything onto one line for a rule the house does not hold. The two
// readings could not both stand, and that is why deciding was the same edit.
//
// AND THE BOTH-WAYS READING, nine mutants one at a time (MEASURED 2026-11-22,
// docs/notes.d/ranger-base-momix.md carries the battery and its output): the
// unmutated arms green; a graded account moved behind a newline reds with that
// file and line; the rule re-tightened to "no newline at all" reds all four
// deferred-prose messages AND the floor that witnesses them, which is the
// decision above taken twice; the key narrowed back to the runner route alone
// reds ten rows, the two route floors among them; the fold dropped from the
// vocabulary, renamed, given a second result, or no longer called by
// bgSeatAccount each reds the arm that owns it; and the four deferred messages
// folded onto one line red the witness floor and nothing else.
//
// WHAT THIS ARM STILL CANNOT SEE, and it is the OTHER half of
// ranger-base-mklu0: the 300 characters. evidence() truncates that first line
// with the `file.go:NN: ` prefix inside the 300, so an account that is on line
// 1 but behind a long preamble is cut anyway — mklu0's finding 2 exactly, 261
// of the 300 spent before the account. It is not read here because it cannot
// be read off a format string: 46 of those 261 bytes were an interpolated cwd
// whose width is the box's and 215 were literal, so a bound on the LITERAL
// prefix — 300 less the `file.go:NN: ` prefix, call it 270 — would have passed
// the one message this class has cost. ORDER WITHIN THE FIRST LINE IS
// THEREFORE UNPINNED, by measurement and not by oversight — ranger-base-uu303
// carries the reading that would decide it.
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

// wmaFoldFunc is the fold both accounts share, and the one a caller uses
// alone when there is no status to carry — a seat still running at its
// deadline, whose words are the whole account (ranger-base-mklu0).
const wmaFoldFunc = "wwOneLine"

// wmaAccountFolds is arm 2's account vocabulary, and it is two names because
// the package says it is two: wwOneLine (wallwatchlive_test.go) is the fold,
// bgSeatAccount (bwrapgitrefslive_test.go) is the account built on it, and
// that file's own head comment is why there is no third — "THE FOLD IS
// wwOneLine's AND NOT A SECOND SPELLING OF IT … two foldings of 'put this on
// one line' are two answers waiting to disagree about what a member's line may
// contain". A typed pair of names is the register this file refuses
// everywhere else, so arm 5 reads the pair BACK out of the member set rather
// than trusting this map.
var wmaAccountFolds = map[string]bool{wmaAccountFunc: true, wmaFoldFunc: true}

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

// wmaUnreadableVerbRe matches the two format shapes whose argument POSITIONS
// cannot be read off the text: an explicit argument index (`%[2]s`) reorders
// them, and a star width (`%*d`) consumes one more argument than it looks
// like. Neither occurs in a member message today, so arm 2 holds such a
// message to the stricter rule and says so rather than guessing a position.
var wmaUnreadableVerbRe = regexp.MustCompile(`%[-+# 0-9.]*(\[[0-9]+\]|\*)`)

// wmaVerbs counts the argument-consuming verbs in a stretch of format string.
// `%%` is a literal percent and consumes nothing, and it is removed first
// because wmaVerbRe's flag class contains a space — `100%% done` would
// otherwise read as one verb.
func wmaVerbs(s string) int {
	return len(wmaVerbRe.FindAllString(strings.ReplaceAll(s, "%%", ""), -1))
}

// wmaAccountBeforeNewline answers, for a format string and the 0-based index
// of the account among the arguments that follow it, whether the account is
// rendered BEFORE the first newline — which is the whole of what reaches a
// bead (ADR 0097 D3, and see the head comment for why this is the rule and
// "no newline at all" was not). `readable` is false when the text uses a verb
// whose positions cannot be read; the caller then holds the message to the
// stricter rule. firstLine is what evidence() would carry.
func wmaAccountBeforeNewline(text string, argIdx int) (before, readable bool, firstLine string) {
	i := strings.IndexAny(text, "\n\r")
	if i < 0 {
		return true, true, text
	}
	if wmaUnreadableVerbRe.MatchString(text) {
		return false, false, text[:i]
	}
	return argIdx < wmaVerbs(text[:i]), true, text[:i]
}

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

// wmaSeatRe is the one spelling of a bwrap launch, read as a command WORD so
// that a path to the binary counts and a neighbour whose name merely carries
// the letters does not: `bwrap` and `/usr/bin/bwrap` match, `bwrap-wrapper`,
// `bwrap.sh` and `wrcNeedBwrap` do not.
var wmaSeatRe = regexp.MustCompile(`(?:^|[^\w.-])bwrap(?:$|[^\w.-])`)

// wmaSeatWord reports whether s names the bwrap binary as a command word.
func wmaSeatWord(s string) bool { return wmaSeatRe.MatchString(s) }

// wmaStringLit unquotes a string literal expression.
func wmaStringLit(x ast.Node) (string, bool) {
	lit, ok := x.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// wmaExprSeatWord reports whether any string literal anywhere under x names
// bwrap as a command word — so a line spelled across a `+` is read too.
func wmaExprSeatWord(x ast.Expr) bool {
	found := false
	ast.Inspect(x, func(n ast.Node) bool {
		if s, ok := wmaStringLit(n); ok && wmaSeatWord(s) {
			found = true
		}
		return true
	})
	return found
}

// wmaExprMentions reports whether x reads any of these names.
func wmaExprMentions(x ast.Expr, names map[string]bool) bool {
	found := false
	ast.Inspect(x, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && names[id.Name] {
			found = true
		}
		return true
	})
	return found
}

// wmaSeatWordNames are the names this function builds a bwrap command word
// INTO: `line := "bwrap …"`, `q = append(q, "bwrap")`, `argv := []string{"bwrap", …}`.
// ONE HOP, the same depth arm 2's account read uses, and it is what lets a
// seat reach the call in a variable — grokleader_qa_test.go's pty arm joins
// its argv into a line before handing it to `script -qfec`.
func wmaSeatWordNames(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	record := func(lhs, rhs []ast.Expr) {
		carries := false
		for _, r := range rhs {
			if wmaExprSeatWord(r) {
				carries = true
			}
		}
		if !carries {
			return
		}
		for _, l := range lhs {
			if id, ok := l.(*ast.Ident); ok && id.Name != "_" {
				names[id.Name] = true
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			record(s.Lhs, s.Rhs)
		case *ast.ValueSpec:
			var lhs []ast.Expr
			for _, id := range s.Names {
				lhs = append(lhs, id)
			}
			record(lhs, s.Values)
		}
		return true
	})
	return names
}

// wmaSeatLaunches reports HOW this function starts a bwrap seat, and the two
// answers are kept apart because one of them is this census's own blind spot
// made visible (ranger-base-hhps5):
//
//	direct   an exec.Command/CommandContext argument is the literal "bwrap"
//	         — the argv entry, which is all this key read before.
//	wrapped  the word reaches the call some OTHER way: inside a command LINE
//	         for a shell or a pty wrapper, or one hop away in a name the
//	         function built the line in. A launch arm 1 could not see at all.
//
// Either is a launch. `wrapped` is counted so the floor below can say the
// tree still has one, because a floor on the visible launchers cannot: an
// invisible launcher does not lower the count of the visible ones.
func wmaSeatLaunches(fn *ast.FuncDecl) (direct, wrapped bool) {
	names := wmaSeatWordNames(fn)
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
			if s, ok := wmaStringLit(a); ok {
				switch {
				case s == "bwrap":
					direct = true
				case wmaSeatWord(s):
					wrapped = true
				}
				continue
			}
			if wmaExprSeatWord(a) || wmaExprMentions(a, names) {
				wrapped = true
			}
		}
		return true
	})
	return direct, wrapped
}

// wmaLaunchesSeat reports whether this function starts a bwrap seat, by
// either route.
func wmaLaunchesSeat(fn *ast.FuncDecl) bool {
	direct, wrapped := wmaSeatLaunches(fn)
	return direct || wrapped
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

// wmaCallee names the function a call expression calls, by its last
// identifier: a package-level func, a method, a field holding a closure.
func wmaCallee(c *ast.CallExpr) string {
	switch fun := c.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

// wmaIsFoldCall reports whether x is a call of one of the account folds.
func wmaIsFoldCall(x ast.Expr) bool {
	c, ok := x.(*ast.CallExpr)
	return ok && wmaAccountFolds[wmaCallee(c)]
}

// wmaFoldLocals are the locals a call site binds to a FOLDED account, which is
// the route wmaAccountLocals cannot see and the one the twice-found file uses
// (ranger-base-momix): the account is produced by wwOneLine or bgSeatAccount
// rather than returned by a seat runner this census can recognise as one.
// Reading the fold instead of the runner is what makes the key independent of
// how the seat was launched — a closure, a one-result helper, a bare inline
// call — and the runner route stays beside it because a runner that returns
// its account unfolded is still an account.
//
// ONE HOP, the same depth as every other reading in this file. An account the
// fold produces three assignments away is not read, and the floors in arm 2
// are what notice a route going quiet.
func wmaFoldLocals(f wmaFile) map[string]bool {
	locals := map[string]bool{}
	record := func(lhs, rhs []ast.Expr) {
		for i, r := range rhs {
			if i >= len(lhs) || !wmaIsFoldCall(r) {
				continue
			}
			if id, ok := lhs[i].(*ast.Ident); ok && id.Name != "_" {
				locals[id.Name] = true
			}
		}
	}
	ast.Inspect(f.ast, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			record(s.Lhs, s.Rhs)
		case *ast.ValueSpec:
			var lhs []ast.Expr
			for _, id := range s.Names {
				lhs = append(lhs, id)
			}
			record(lhs, s.Values)
		}
		return true
	})
	return locals
}

// wmaAccountArg reports whether one argument of a format call IS a seat's
// account, and by which route — which is the whole of arm 2's key. The three
// routes are kept apart so arm 2 can floor each of them: a route that has
// gone quiet is a route a later narrowing could delete while the census stays
// green, which is ranger-base-hhps5's lesson one arm over.
func wmaAccountArg(x ast.Expr, runner, folded map[string]bool) (name, route string, ok bool) {
	if id, isID := x.(*ast.Ident); isID {
		switch {
		case runner[id.Name]:
			return id.Name, "runner local", true
		case folded[id.Name]:
			return id.Name, "folded local", true
		}
		return "", "", false
	}
	if c, isCall := x.(*ast.CallExpr); isCall && wmaAccountFolds[wmaCallee(c)] {
		return wmaCallee(c) + "(…)", "inline fold", true
	}
	return "", "", false
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
	launchers, graded, argvEntry := 0, 0, 0
	var wrapped []string
	for _, f := range files {
		for _, fn := range wmaDecls(f.ast) {
			direct, viaWrapper := wmaSeatLaunches(fn)
			if !direct && !viaWrapper {
				continue
			}
			launchers++
			if direct {
				argvEntry++
			}
			if viaWrapper {
				wrapped = append(wrapped, f.path+":"+strconv.Itoa(f.fset.Position(fn.Pos()).Line)+" "+fn.Name.Name)
			}
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
	// AND THE CONTROL FOR THE WIDENING (ranger-base-hhps5). The floor above
	// cannot ask whether the launch KEY still reads every spelling of a
	// launch, for the reason that finding exists: an invisible launcher does
	// not lower the count of the visible ones, so arm 1 was green over the
	// wrapper route for as long as it could not see it. This floor asks the
	// one question that floor cannot — does the tree still carry a launch the
	// old literal-argv key could not see — and arm 3's planted pair is the
	// hermetic half of the same claim.
	if len(wrapped) == 0 {
		t.Errorf("this census found %d launch(es) of a seat spelled some way other than a literal `bwrap` argv entry, want at least 1 (2 on 2026-11-22: grokleader_qa_test.go's pty arm, which joins the render's argv into a line for `script -qfec`, and wallunixlive_test.go's `sh -c 'bwrap \"$@\"; :'` arm). Those two are why the key is not the argv entry alone (ranger-base-hhps5); with neither in the tree, nothing but arm 3's planted pair holds the widening, and a later narrowing of wmaSeatLaunches would be green here. If the routes really have gone, say so and take this floor out — do not lower it.", len(wrapped))
	}
	// The other direction — a key that has stopped meaning `this function
	// starts a seat` — is arm 3's and not a bound here: see the head comment,
	// which carries the measurement that no bound over this tree can read it.
	// What this arm owes a reader is the census it actually took.
	t.Logf("%d member file(s), %d launching function(s) (%d handing exec.Command a literal `bwrap` argv entry, %d through the wrapper route), %d child error(s) graded. The wrapper reads were: %v",
		len(files), launchers, argvEntry, len(wrapped), graded, wrapped)
}

// ARM 2 — THE LINE. A message carrying a seat's account carries it where the
// bead will read it. This is ranger-base-a688i's half, scoped to the one
// value that cannot be recovered after the fact.
//
// THE ACCOUNT IS RENDERED BEFORE THE FIRST NEWLINE, and not "the message has
// no newline at all". `scripts/posse-wall-check`'s evidence() carries that
// first line and nothing after it (ADR 0097 D3), so the newline is what costs
// a reader the TAIL and never the account in front of it — and the account in
// front with the supporting prose behind is the shape D3 and
// ranger-base-mklu0's fix both prescribe. The head comment carries the
// measurement that decided this against the rule that stood here before, and
// names the one thing this rule still does not read (the 300 characters,
// ranger-base-uu303).
func TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine(t *testing.T) {
	t.Parallel()
	files := wmaMemberFiles(t)
	withAccounts, read, deferred := 0, 0, 0
	routes := map[string]int{}
	for _, f := range files {
		// EVERY member file, and not only the ones that bind an account to a
		// local: an inline fold IS an account, and a file that hands one
		// straight to a verb binds nothing. Gating the walk on the locals was
		// the narrowness this arm was found with one layer down — it hid
		// wallunixlive_test.go's three deferred-prose messages, which are
		// three quarters of what witnesses the rule (ranger-base-momix).
		runner, folded := wmaAccountLocals(f), wmaFoldLocals(f)
		fileRead := 0
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
			for i, a := range c.Args[1:] {
				name, route, isAccount := wmaAccountArg(a, runner, folded)
				if !isAccount {
					continue
				}
				read++
				fileRead++
				routes[route]++
				// A variadic spread reorders nothing but hides how many
				// arguments the verbs before the newline consumed, so it is
				// the same unreadable case as an indexed verb.
				before, readable, firstLine := wmaAccountBeforeNewline(text, i)
				if c.Ellipsis.IsValid() {
					readable = false
				}
				switch {
				case !readable:
					t.Errorf("%s:%d: this t.%s carries the seat's account %s in a format string that has a newline in it AND a verb whose argument positions cannot be read off the text (an indexed `%%[n]s`, a star width, or a variadic spread). `scripts/posse-wall-check` carries the FIRST `file.go:NN:` line and nothing after it (ADR 0097 D3), so whether the account survives depends on where it lands in %q — and this census will not guess. Spell the verbs in order, or fold the message onto one line.",
						f.path, f.fset.Position(c.Pos()).Line, sel.Sel.Name, name, firstLine)
				case !before:
					t.Errorf("%s:%d: this t.%s renders the seat's account %s AFTER the first newline of its format string. `scripts/posse-wall-check` reports a failed member by its FIRST `file.go:NN:` line and nothing after it (ADR 0097 D3), and the launcher puts that one line on a P1 bead — so this message reaches the bead as %q and the account really is dropped in transit. That is ranger-base-a688i exactly: a symptom, a colon and nothing. Move the account in front of the newline; the prose behind it costs nothing.",
						f.path, f.fset.Position(c.Pos()).Line, sel.Sel.Name, name, firstLine)
				case strings.ContainsAny(text, "\n\r"):
					// The shape the rule before this one refused, and the
					// reason the floor below exists.
					deferred++
				}
			}
			return true
		})
		if fileRead > 0 {
			withAccounts++
		}
	}
	// The controls, again both ways.
	if withAccounts < 6 {
		t.Errorf("this arm found seat accounts in %d member file(s), want at least 6 (8 on 2026-11-22: bwrapgitrefslive_test.go, bwrapseatstorelive_test.go, storerepogitwalllive_test.go, wallclaudeprojects_qa_test.go, wallclaudeprojectsattack_qa_test.go, wallunixlive_test.go, wallwatchbackstopwiring_linux_test.go and wallwatchlive_test.go — 3 of them under the key before ranger-base-momix widened it) — the shapes this arm reads have changed and it is grading almost nothing", withAccounts)
	}
	if read < 45 {
		t.Errorf("this arm read %d message(s) interpolating a seat account, want at least 45 (65 on 2026-11-22, of which 32 were all the key read before ranger-base-momix) — an account reaches a reader by some route this arm cannot see", read)
	}
	// AND A FLOOR PER ROUTE, for the reason arm 1 floors the wrapper route
	// (ranger-base-hhps5): a key that reads three spellings and finds two of
	// them in the tree is a key whose third arm a later narrowing could
	// delete with every arm here still green. An account this census cannot
	// see does not lower the count of the ones it can.
	for _, route := range []string{"runner local", "folded local", "inline fold"} {
		if routes[route] == 0 {
			t.Errorf("this arm read 0 account(s) by the %q route, want at least 1 (on 2026-11-22: 32 runner local, 8 folded local, 25 inline fold). That route is in the key because the member set used it — the folded routes are ranger-base-momix's, which is how wallclaudeprojectsattack_qa_test.go came to be graded by nothing after this class had been found in it twice. If the spelling really has gone from the tree, say so and take the route out of wmaAccountArg; do not leave an arm of the key with nothing witnessing it.", route)
		}
	}
	// THE WITNESS FOR THE RULE ITSELF. The rule above is POSITIONAL, and the
	// only thing in this tree that tells a positional rule from the "no
	// newline at all" rule it replaced is a message that HAS a newline with
	// its account in front of it. With none in the member set, a later
	// re-tightening would be green here and would red four messages whose
	// shape ADR 0097 D3 prescribes — which is the state ranger-base-momix
	// found and the reason that re-tightening is easy to type.
	if deferred == 0 {
		t.Errorf("this arm read %d account message(s) that defer their prose to a second line, want at least 1 (4 on 2026-11-22: wallclaudeprojectsattack_qa_test.go:386 and wallunixlive_test.go:237, :526, :1130). Those four are what distinguish this arm's rule from `no newline at all`; with none of them in the tree, nothing here witnesses that the rule is positional and arm 3's planted rows are the whole of it.", deferred)
	}
	t.Logf("%d member file(s), %d of them carrying a seat account, %d account message(s) read (%d by a runner local, %d by a folded local, %d by an inline fold), %d of which defer prose to a second line with the account in front of it.",
		len(files), withAccounts, read, routes["runner local"], routes["folded local"], routes["inline fold"], deferred)
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
		// ranger-base-hhps5's planted pair, which is the whole of that
		// finding: the same probe twice, the only difference being how the
		// seat's argv is spelled. The first was RED before the widening and
		// the second GREEN, with nothing else between them (MEASURED
		// 2026-11-22 at 2866fdf4 by planting both into internal/posse as a
		// member file). Both are launches now.
		{"a seat launched through a shell, the literal inside the command line", `func run(t *testing.T) {
	cmd := exec.Command("sh", "-c", "bwrap --dev-bind / / /bin/true")
	out, _ := cmd.CombinedOutput()
	t.Logf("%s", out)
}`, true, false},
		{"a seat launched through a pty wrapper, the argv joined into a line one hop away", `func run(t *testing.T) {
	var q []string
	q = append(q, "bwrap")
	for _, a := range argv {
		q = append(q, shellQuote(a))
	}
	cmd := exec.Command(script, "-qfec", strings.Join(q, " ")+" "+line, "/dev/null")
	if err := cmd.Start(); err != nil {
		t.Fatalf("the seat never started: %v", err)
	}
	_ = cmd.Wait()
}`, true, true},
		{"a seat launched by its absolute path inside a command line", `func run(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exec /usr/bin/bwrap --dev-bind / / /bin/true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("the seat refused: %v", err)
	}
}`, true, true},
		// And the negative cases the widening needs, because a key read as a
		// SUBSTRING would put every one of these under arm 1's rule:
		{"a shell line that is not a seat", `func run(t *testing.T) {
	cmd := exec.Command("sh", "-c", "git status --porcelain")
	out, _ := cmd.CombinedOutput()
	t.Logf("%s", out)
}`, false, false},
		{"a shell line naming a neighbour of bwrap", `func run(t *testing.T) {
	cmd := exec.Command("sh", "-c", "bwraptest && bwrap-wrapper && echo bwrap.sh")
	out, _ := cmd.CombinedOutput()
	t.Logf("%s", out)
}`, false, false},
		{"a skip message that says bwrap is absent", `func run(t *testing.T) {
	why := "NOTHING MEASURED: no bwrap on this host"
	cmd := exec.Command("git", "status")
	out, _ := cmd.CombinedOutput()
	t.Skipf("%s: %s", why, out)
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

	// And arm 2's OTHER two routes, which are ranger-base-momix's: an account
	// the fold produced, bound to a local or handed to the verb inline. The
	// rows are the two spellings
	// internal/posse/wallclaudeprojectsattack_qa_test.go actually uses, and
	// the negatives are the ones that make the key a key: a fold of something
	// that is not a seat's words is still read (the fold exists to make a
	// value fit a member's one line, so the rule is right for it too), but a
	// local bound to anything else is not.
	for _, c := range []struct {
		name  string
		src   string
		local []string
		arg   []bool
	}{
		{"an account folded into a local, from a runner this census cannot recognise", `func TestX(t *testing.T) {
	out := run(t, nonOwn)
	said := wwOneLine(out)
	t.Fatalf("claude -p under the wall did not end in 90s; it said %s\ncwd %s", said, cwd)
}`, []string{"said"}, []bool{true, false}},
		{"an account built by the account func and bound to a local", `func TestX(t *testing.T) {
	acct := bgSeatAccount(runErr, outBuf.String())
	t.Logf("the seat ended badly: %s", acct)
}`, []string{"acct"}, []bool{true}},
		{"an account handed to the verb inline", `func TestX(t *testing.T) {
	t.Logf("the seat ended badly, which this pin does not assert on: %s", bgSeatAccount(runErr, outBuf.String()))
}`, nil, []bool{true}},
		{"a fold handed to the verb inline", `func TestX(t *testing.T) {
	t.Fatalf("the seat exited badly: %s\nargv: bwrap %s", wwOneLine(string(out)), argv)
}`, nil, []bool{true, false}},
		{"a local bound to something that is not a fold", `func TestX(t *testing.T) {
	out := strings.TrimSpace(string(raw))
	t.Fatalf("the seat said:\n%s", out)
}`, nil, []bool{false}},
		{"a blanked fold binds nothing", `func TestX(t *testing.T) {
	_ = wwOneLine(out)
	t.Logf("%s", out)
}`, nil, []bool{false}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, f := wmaFixture(t, c.src, "")
			got := wmaFoldLocals(f)
			if len(got) != len(c.local) {
				t.Fatalf("wmaFoldLocals found %v, want %v — arm 2's folded-local route no longer binds what this file says it binds:\n%s", got, c.local, c.src)
			}
			for _, w := range c.local {
				if !got[w] {
					t.Errorf("wmaFoldLocals found %v, want it to name %q:\n%s", got, w, c.src)
				}
			}
			// And the per-argument reading, which is what arm 2 keys on.
			runner := wmaAccountLocals(f)
			var isAccount []bool
			ast.Inspect(f.ast, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !wmaFormatVerbs[sel.Sel.Name] {
					return true
				}
				for _, a := range call.Args[1:] {
					_, _, is := wmaAccountArg(a, runner, got)
					isAccount = append(isAccount, is)
				}
				return true
			})
			if len(isAccount) != len(c.arg) {
				t.Fatalf("wmaAccountArg read %d format argument(s), want %d:\n%s", len(isAccount), len(c.arg), c.src)
			}
			for i := range c.arg {
				if isAccount[i] != c.arg[i] {
					t.Errorf("wmaAccountArg says argument %d is an account = %v, want %v — arm 2's key no longer means what this file says it means:\n%s", i, isAccount[i], c.arg[i], c.src)
				}
			}
		})
	}

	// THE POSITIONAL RULE, driven over the text (ranger-base-momix). The row
	// that matters most is the third: an account in FRONT of a newline is
	// accepted, which is the shape ADR 0097 D3 prescribes and the shape the
	// rule that stood here before refused.
	for _, c := range []struct {
		name, text       string
		argIdx           int
		before, readable bool
		firstLine        string
	}{
		{"one line, one verb", "the seat refused: %s", 0, true, true, "the seat refused: %s"},
		{"ranger-base-a688i's own shape, the account after the newline", "the seat ended before it ran (exit 1):\n%s", 0, false, true, "the seat ended before it ran (exit 1):"},
		{"the account in front, the prose behind", "it said %s\ncwd %s — D3 assumes an unwritable transcript dir is survivable", 0, true, true, "it said %s"},
		{"the second argument is behind the newline", "it said %s\ncwd %s", 1, false, true, "it said %s"},
		{"a literal percent consumes no argument", "the wall took 100%% of it:\n%s", 0, false, true, "the wall took 100%% of it:"},
		{"a carriage return is a newline too", "the seat refused:\r%s", 0, false, true, "the seat refused:"},
		{"an indexed verb cannot be read positionally", "it said %[1]s\ncwd %[2]s", 0, false, false, "it said %[1]s"},
		{"a star width cannot be read positionally", "it said %*s\ncwd %s", 1, false, false, "it said %*s"},
		{"an unreadable verb in a ONE-LINE message is no finding", "it said %[1]s and %[1]s again", 0, true, true, "it said %[1]s and %[1]s again"},
	} {
		t.Run(c.name, func(t *testing.T) {
			before, readable, firstLine := wmaAccountBeforeNewline(c.text, c.argIdx)
			if before != c.before || readable != c.readable || firstLine != c.firstLine {
				t.Errorf("wmaAccountBeforeNewline(%q, %d) = %v,%v,%q want %v,%v,%q — arm 2's line rule no longer reads what this file says it reads",
					c.text, c.argIdx, before, readable, firstLine, c.before, c.readable, c.firstLine)
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

// ARM 5 — THE ACCOUNT VOCABULARY IS THE TREE'S AND NOT THIS FILE'S
// (ranger-base-momix). Arm 2's key reads a value folded by one of two names,
// and a typed pair of names is exactly the register this file's head comment
// refuses when it declines an exemption table: a list nobody reads back drifts
// quietly, and a drifted list here does not red — it GRADES LESS, which is the
// state that let this class be found twice in one file.
//
// So the pair is read back out of the member set. Each name is declared there
// as a function returning exactly one string, and bgSeatAccount's body calls
// wwOneLine — which is the sentence bwrapgitrefslive_test.go writes about
// itself ("THE FOLD IS wwOneLine's AND NOT A SECOND SPELLING OF IT"). What it
// cannot see is a THIRD fold nobody told it about; arm 2's per-route floors
// are what notice a route going quiet, and this arm is what notices the two
// names it does have coming apart.
func TestQATheWallMemberAccountFoldsAreTheOnesThePackageDeclares(t *testing.T) {
	t.Parallel()
	files := wmaMemberFiles(t)
	decls := map[string]*ast.FuncDecl{}
	for _, f := range files {
		for _, fn := range wmaDecls(f.ast) {
			if fn.Recv != nil || !wmaAccountFolds[fn.Name.Name] {
				continue
			}
			if prev, seen := decls[fn.Name.Name]; seen {
				t.Errorf("%s declares a second %s (the first is at %s) — two foldings of \"put this on one line\" are two answers waiting to disagree about what a member's line may contain (bwrapgitrefslive_test.go), and arm 2 reads them as one",
					f.path, fn.Name.Name, prev.Name.Name)
			}
			decls[fn.Name.Name] = fn
		}
	}
	for name := range wmaAccountFolds {
		fn, ok := decls[name]
		if !ok {
			t.Errorf("no member file declares %s, which arm 2's key names as a spelling of a seat's account. Either it was renamed — in which case arm 2 is now grading the messages that used it by nothing, silently, which is the shape ranger-base-momix found — or the fold has moved out of the member set and this census can no longer see it.", name)
			continue
		}
		res := fn.Type.Results
		if res == nil || len(res.List) != 1 || len(res.List[0].Names) > 1 {
			t.Errorf("%s no longer returns exactly one value, so what arm 2 reads at its call sites is not the one-line account this file says it is", name)
			continue
		}
		if id, isID := res.List[0].Type.(*ast.Ident); !isID || id.Name != "string" {
			t.Errorf("%s no longer returns a string, so arm 2's reading of its call sites as a rendered account is wrong", name)
		}
	}
	// AND THE PAIR IS ONE FOLD AND ONE ACCOUNT BUILT ON IT, which is the whole
	// reason this list is two names long and not one or three.
	if fn, ok := decls[wmaAccountFunc]; ok && !wmaBodyCalls(fn, wmaFoldFunc) {
		t.Errorf("%s no longer calls %s, so this package has two independent foldings of \"put this on one line\" and arm 2 reads them as one vocabulary. bwrapgitrefslive_test.go's own head comment is the contract: the fold is %s's and not a second spelling of it.",
			wmaAccountFunc, wmaFoldFunc, wmaFoldFunc)
	}
}

// wmaBodyCalls reports whether fn's body calls the named function.
func wmaBodyCalls(fn *ast.FuncDecl, name string) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok && wmaCallee(c) == name {
			found = true
		}
		return true
	})
	return found
}
