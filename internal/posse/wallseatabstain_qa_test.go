//go:build !posse_arm2 && !posse_arm3

package posse

// QA, ranger-base-k2quj — EVERY SEAT RUN IN THE WALL CHECK'S MEMBER SET
// ROUTES THE ABSTENTION, and this is the pin that says so for the 23 files
// the first one did not reach.
//
// WHAT WAS WRONG. ranger-base-evx07 landed the criterion
// (bgSeatEndedFromOutside) and the abstention (bgSeatAbstain) and routed them
// in ONE file. MEASURED 2026-11-22 against the check's own definition of the
// set — wallVenueMemberPattern, read rather than retyped — 55 members in 24
// files: 8 in bwrapgitrefslive_test.go, 47 in 23 files that kept their own
// seat runners with no abstention at all. evx07's close said exactly that
// ("the other members in ADR 0097's set keep their own seat runners, so the
// same silent kill can still red them") and filed nothing; ranger-base-2unmh
// found it in verification and handed it here.
//
// WHAT IT COSTS. A seat the box ends — the fleet load guard's kill arm, an
// operator, an OOM killer — says nothing and exits 128+9 or 128+15, and
// ranger-base-evx07 MEASURED that bwrap renders both as an ordinary status.
// An unguarded runner reduces that to `err == nil` and both polarities are
// then wrong: a refusal arm shaped `if ok { fail }` reads it as the wall
// refusing, which is a FALSE PASS (ADR 0086 D5's open obligation), and a
// success arm reads it as the red that ADR 0097 D3 reports as exit 1 and ADR
// 0088 D5's ratchet files as one P1 `-l code` bead attributed to the NEWEST
// LANDING'S PERSONA. That class has already done it four times —
// ranger-base-a688i, -w6kke, -axt1f, -s9h52 — and none of those landings had
// anything to do with it.
//
// WHY A PIN AND NOT A LIVE ARM. The guarded branch is reached only by a real
// silent supervisor kill, which no fixture manufactures inside a runner:
// MEASURED under ranger-base-2unmh, deleting BOTH call sites in the one
// converted file left arm 1 green, unfiltered. So the routing has no
// observable, and what is read is the one thing that is always there — the
// call, in the function that runs the seat. Same reasoning and same `qcs*`
// helpers as callsite_qa_test.go (ranger-base-nqw8v) and
// bwrapgitrefsaccount_qa_test.go (ranger-base-axt1f).
//
// WHAT IT DOES NOT READ, said here so the next reader does not come back to
// it. wallvenue_test.go runs a bwrap of its own (bwrapNestProbe) and is NOT a
// member file, deliberately: that probe is the gate, its own comment says
// "EVERY FAILURE IS AN ABSTENTION", and a silent kill of it already becomes a
// skip rather than a measurement. And eight of the 24 member files exec
// nothing at all, so they have no runner to route.
//
// ITS DOOR IS `make pkgcensus-check`. This pin enumerates its own package
// directory and reads every `*_test.go` in it, which is the third seed of
// internal/posse's tree-wide class (twdPosseEnumOnlyTests), so it needs a
// Makefile door like every other one-package census — see
// internal/treepins/treewidedoor_qa_test.go, whose register carries the row.
// No new door: QA_PKGCENSUS_PINS' subject is a reading of this repo's own Go
// sources whose file SET is discovered, and this is that reading.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// wsaRouter is the ONE routing door every seat run must go through, and
// wsaCriterion is the escape for a test whose SUBJECT is the criterion: it
// calls the decision function directly, so it is grading the rule rather than
// spending it (TestLiveBwrapGitRefsReadsAnOutsideKillAsNothingMeasured and
// TestBgSeatEndedFromOutsideReadsOnlyASupervisorSignalOverSilence). Derived
// and not a name list, so a second such pin needs no edit here.
// The watcher-run half is the SAME rule over the other way a member runs a
// seat (ranger-base-jqdm2): runWallWatch forks bwrap in PRODUCTION code and
// hands the test an int, so there is no `exec.Command("bwrap", …)` in the
// test for arm 1 to read and no *exec.ExitError for wsaRouter to take. Seven
// members are run that way and none of them routed anything until jqdm2.
// wsaWatcherCriterion is the same escape as wsaCriterion, for the pin whose
// subject is the shared decision function.
const (
	wsaRouter    = "bgSeatAbstainIfOutside"
	wsaCriterion = "bgSeatEndedFromOutside"

	wsaWatcherRouter    = "wwAbstainIfOutside"
	wsaWatcherCriterion = "bgSeatStatusFromOutside"
	wsaWatcherRun       = "runWallWatch"
)

// wsaRunVerbs are the calls that make a function the one that RAN the seat,
// as opposed to one that merely built the *exec.Cmd. The distinction is
// load-bearing: mcpstrict_live_test.go's `walled` is a builder — it returns
// the command and runs nothing, so it has no status to read — and the rule
// steps out of it to the function that does read one.
var wsaRunVerbs = map[string]bool{
	"Run": true, "Start": true, "Wait": true, "Output": true, "CombinedOutput": true,
}

// wsaSite is one seat run: where it is, and which function has to route.
//
// `top` is the TOP-LEVEL declaration the site is in, and only the watcher
// half sets it (ranger-base-jqdm2): the ROUTER has to be in the innermost
// function, so that one arm of a member cannot lose it behind a sibling that
// keeps it, while the CRITERION escape is a property of the whole test — a
// pin whose subject is the decision grades it in its arms and may transport
// the status through a closure to get there.
type wsaSite struct {
	line  int
	owner string // the routing function's label, for the message
	body  ast.Node
	top   ast.Node
}

// wsaBwrapLiteral answers whether a call is `exec.Command("bwrap", …)` or
// `exec.CommandContext(ctx, "bwrap", …)`. A LITERAL, deliberately: a site
// whose argv[0] is a variable cannot be read this way and is carried by
// wsaVariableCommandRegister below instead, which is the honest shape rather
// than a rule that guesses.
func wsaBwrapLiteral(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "exec" {
		return false
	}
	at := -1
	switch sel.Sel.Name {
	case "Command":
		at = 0
	case "CommandContext":
		at = 1
	default:
		return false
	}
	if len(c.Args) <= at {
		return false
	}
	lit, ok := c.Args[at].(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && (lit.Value == `"bwrap"` || lit.Value == "`bwrap`")
}

// wsaCommandArgIsLiteral answers whether an exec.Command/CommandContext call
// names its program with a string literal at all. Arm 2 reads the negative.
func wsaCommandArgIsLiteral(c *ast.CallExpr) (isExec, literal bool) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false, false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "exec" {
		return false, false
	}
	at := -1
	switch sel.Sel.Name {
	case "Command":
		at = 0
	case "CommandContext":
		at = 1
	default:
		return false, false
	}
	if len(c.Args) <= at {
		return true, false
	}
	lit, ok := c.Args[at].(*ast.BasicLit)
	return true, ok && lit.Kind == token.STRING
}

// wsaMembers returns the top-level test names in a parsed file that are
// members of ADR 0097 D3's set, by the pattern the check itself types.
func wsaMembers(f *ast.File) []string {
	re := wallVenueMemberRe()
	var out []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !re.MatchString(fn.Name.Name) {
			continue
		}
		out = append(out, fn.Name.Name)
	}
	return out
}

// wsaSeatSites finds every bwrap seat run in a parsed file and, for each,
// the function that has to route the abstention: the innermost enclosing
// function whose body RUNS something, stepping outward through builders, and
// the top-level declaration if none of them does.
func wsaSeatSites(f *ast.File, fset *token.FileSet) []wsaSite {
	var out []wsaSite
	// The chain of enclosing functions, outermost first, maintained as the
	// walk descends. A FuncLit inside a FuncDecl inside nothing else is the
	// shape almost every seat run has.
	type frame struct {
		node  ast.Node
		label string
	}
	var chain []frame
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		ast.Inspect(n, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.FuncDecl:
				if v.Body == nil {
					return false
				}
				chain = append(chain, frame{v.Body, qcsName(wsaRecv(v), v.Name.Name)})
				walk(v.Body)
				chain = chain[:len(chain)-1]
				return false
			case *ast.FuncLit:
				chain = append(chain, frame{v.Body, "a func literal"})
				walk(v.Body)
				chain = chain[:len(chain)-1]
				return false
			case *ast.CallExpr:
				if !wsaBwrapLiteral(v) {
					return true
				}
				// Innermost first, outward: the first frame that runs
				// something owns the routing.
				owner := -1
				for i := len(chain) - 1; i >= 0; i-- {
					if wsaRuns(chain[i].node) {
						owner = i
						break
					}
				}
				if owner < 0 && len(chain) > 0 {
					owner = 0
				}
				if owner < 0 {
					return true
				}
				out = append(out, wsaSite{
					line:  fset.Position(v.Pos()).Line,
					owner: chain[owner].label,
					body:  chain[owner].node,
				})
				return true
			}
			return true
		})
	}
	walk(f)
	return out
}

// wsaRecv is the receiver type name of a method declaration, "" for a plain
// function, so a message can name `bgLive.walled` the way the house does.
func wsaRecv(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	return qcsTypeName(fn.Recv.List[0].Type)
}

// wsaRuns: does this body run a command, rather than only build one?
func wsaRuns(n ast.Node) bool {
	for _, c := range qcsCalls(n) {
		if wsaRunVerbs[c] {
			return true
		}
	}
	return false
}

// wsaUnrouted is the rule, over one parsed source: the lines of every bwrap
// seat run whose owning function routes neither the abstention nor the
// criterion. It is a function so the controls below can drive it over planted
// source with no bwrap, no venue and no fixture.
func wsaUnrouted(t *testing.T, name, src string) (unrouted []string, sites int) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	for _, s := range wsaSeatSites(f, fset) {
		sites++
		calls := qcsCalls(s.body)
		if qcsHas(calls, wsaRouter) || qcsHas(calls, wsaCriterion) {
			continue
		}
		unrouted = append(unrouted, name+":"+strconv.Itoa(s.line)+" ("+s.owner+")")
	}
	return unrouted, sites
}

// ─── the watcher-run half (ranger-base-jqdm2) ────────────────────────────────

// wsaWatcherLiveSeat answers three things about a call: whether it is
// runWallWatch, whether the seat it starts is a LIVE bwrap, and whether that
// was readable at all.
//
// THE KEY IS THE RENDERED PROGRAM, as arm 1's is: a `Bwrap:` field whose
// value is a []string literal whose first element is the string "bwrap". The
// three fixtures that pass `[]string{"/bin/false"}` start no sandbox and have
// no seat for the box to kill — their subject is what the watcher does when
// bwrap never reports a child — so they are read and correctly found to be
// nobody's seat, exactly as `exec.Command("git", …)` is in arm 1. A `Bwrap:`
// whose value is a VARIABLE cannot be read this way and is carried by
// wsaWatcherSiteRegister instead, which is the honest shape rather than a
// rule that guesses.
func wsaWatcherLiveSeat(c *ast.CallExpr) (isRun, live, readable bool) {
	id, ok := c.Fun.(*ast.Ident)
	if !ok || id.Name != wsaWatcherRun {
		return false, false, false
	}
	// Bwrap sits inside wallWatchOpts inside the wallWatchRun literal, so
	// the whole argument subtree is searched for the key rather than a fixed
	// path through it: a field order or a nesting change must not silently
	// stop this rule matching.
	for _, a := range c.Args {
		var found *ast.KeyValueExpr
		ast.Inspect(a, func(x ast.Node) bool {
			kv, ok := x.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Bwrap" {
				found = kv
			}
			return true
		})
		if found == nil {
			continue
		}
		cl, ok := found.Value.(*ast.CompositeLit)
		if !ok || len(cl.Elts) == 0 {
			return true, false, false
		}
		lit, ok := cl.Elts[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true, false, false
		}
		return true, lit.Value == `"bwrap"` || lit.Value == "`bwrap`", true
	}
	return true, false, false
}

// wsaChanIntNames is every identifier this file makes a `chan int`: the
// watcher's exit status in transit. A watcher runs in a goroutine and sends
// its code down one of these, so the place that READS the status is a receive
// and not the run — and the read is what has to route, because it is the line
// that compares the code to a number the fixture expected.
//
// RESOLUTION IS BY NAME, within the file, and deliberately crude: the type of
// `code` at `c := <-code` cannot be resolved from an AST alone (it is the
// result of a call, in a different function, in the same package). A file
// that declares `code` as a chan int anywhere therefore makes every `<-code`
// in it a status read. The cost is a name collision inside one test file; the
// alternative is a rule that reads no receive at all, which is the hole
// ranger-base-jqdm2 came through — the only member whose seat run arm 1 could
// have seen is launchOn's, and every one of the members that actually red
// reads its status somewhere else.
func wsaChanIntNames(f *ast.File) map[string]bool {
	isChanInt := func(e ast.Expr) bool {
		ct, ok := e.(*ast.ChanType)
		if !ok {
			return false
		}
		id, ok := ct.Value.(*ast.Ident)
		return ok && id.Name == "int"
	}
	out := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range v.Rhs {
				call, ok := rhs.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					continue
				}
				if fun, ok := call.Fun.(*ast.Ident); !ok || fun.Name != "make" {
					continue
				}
				if !isChanInt(call.Args[0]) || i >= len(v.Lhs) {
					continue
				}
				if id, ok := v.Lhs[i].(*ast.Ident); ok {
					out[id.Name] = true
				}
			}
		case *ast.Field: // a parameter, or a struct field holding one
			if isChanInt(v.Type) {
				for _, n := range v.Names {
					out[n.Name] = true
				}
			}
		case *ast.ValueSpec:
			if v.Type != nil && isChanInt(v.Type) {
				for _, n := range v.Names {
					out[n.Name] = true
				}
			}
		}
		return true
	})
	return out
}

// wsaWatcherSites is every place a watcher's exit status is READ, owned by
// the innermost function that reads it.
//
// WHAT IS A SITE, and it is not simply "a runWallWatch call". These fixtures
// run the watcher in a `go func()` that sends its status down a channel and
// read it somewhere else entirely — a `t.Run` arm, a helper taking the
// channel, the launch loop's own non-blocking select. So a run whose value is
// SENT to a channel is a producer and no site at all: it hands the status on,
// the way arm 1's builders hand on an *exec.Cmd, and the receive is where an
// arm can be wrong about it. A run whose value is used in place
// (`got := runWallWatch(…)`) is both at once and is a site.
//
// THE OWNER IS THE INNERMOST FUNCTION, literals included, which is what makes
// this rule bite inside a member that has several arms: a test whose two
// `t.Run` closures each read a status must route in BOTH, and attributing the
// pair to their parent would let either one lose its routing silently.
func wsaWatcherSites(f *ast.File, fset *token.FileSet) []wsaSite {
	chans := wsaChanIntNames(f)
	// The runs whose value is handed to a channel: producers, not sites.
	handedOn := map[ast.Node]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.SendStmt); ok {
			if c, ok := s.Value.(*ast.CallExpr); ok {
				handedOn[c] = true
			}
		}
		return true
	})

	type frame struct {
		node  ast.Node
		label string
	}
	var chain []frame
	var out []wsaSite
	add := func(n ast.Node) {
		f := chain[len(chain)-1]
		label := f.label
		if len(chain) > 1 {
			label = chain[0].label + "'s " + label
		}
		out = append(out, wsaSite{line: fset.Position(n.Pos()).Line, owner: label, body: f.node, top: chain[0].node})
	}
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		ast.Inspect(n, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.FuncDecl:
				if v.Body == nil {
					return false
				}
				chain = append(chain, frame{v.Body, qcsName(wsaRecv(v), v.Name.Name)})
				walk(v.Body)
				chain = chain[:len(chain)-1]
				return false
			case *ast.FuncLit:
				chain = append(chain, frame{v.Body, "func literal"})
				walk(v.Body)
				chain = chain[:len(chain)-1]
				return false
			case *ast.CallExpr:
				if len(chain) == 0 || handedOn[v] {
					return true
				}
				if isRun, live, _ := wsaWatcherLiveSeat(v); isRun && live {
					add(v)
				}
			case *ast.UnaryExpr:
				if v.Op != token.ARROW || len(chain) == 0 {
					return true
				}
				if id, ok := v.X.(*ast.Ident); ok && chans[id.Name] {
					add(v)
				}
			}
			return true
		})
	}
	walk(f)
	return out
}

// wsaWatcherUnrouted is the watcher half of the rule, over one parsed source,
// and a function for the same reason wsaUnrouted is: the controls drive it
// over planted source with no bwrap and no fixture.
func wsaWatcherUnrouted(t *testing.T, name, src string) (unrouted []string, sites int) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	for _, s := range wsaWatcherSites(f, fset) {
		sites++
		if qcsHas(qcsCalls(s.body), wsaWatcherRouter) {
			continue
		}
		// THE CRITERION ESCAPE IS READ ON THE WHOLE TEST, and the router on
		// the innermost function (wsaSite.top says why). The live half of
		// this bead's own criterion is the shape that needs it: it IS the
		// outside killer, so routing it would abstain on every run and hold
		// the check at exit 3 forever (the grokleader row below is the same
		// family), and it grades the decision in its two arms while a
		// transport closure carries the status out to them.
		top := qcsCalls(s.top)
		if qcsHas(top, wsaWatcherCriterion) || qcsHas(top, wsaCriterion) {
			continue
		}
		unrouted = append(unrouted, name+":"+strconv.Itoa(s.line)+" ("+s.owner+")")
	}
	return unrouted, sites
}

// wsaWatcherSiteRegister is the priced table of watcher-run seats whose
// program this rule cannot read: a `Bwrap:` field whose value is a variable.
// Same job, same arm shape and same reason as wsaVariableCommandRegister one
// table down — the hole is named rather than silently covered, and a NEW
// invisible watcher fixture cannot arrive without a reader.
var wsaWatcherSiteRegister = []struct {
	file, test, why string
	routed          bool
}{
	{
		file: "wallwatchtty_linux_test.go", test: "TestWallWatchARefusedSeatLeavesItsReasonOnDisk",
		routed: false,
		why: "EXEMPT: the table's `Bwrap` is the case's own field, and every case is a bwrap that " +
			"CANNOT RUN — the subject is the refusal runWallWatch writes to the verdict file before " +
			"any seat exists (ranger-base-r9slg), so there is no sandboxed seat for the box to kill. " +
			"It is also not a member: the ratchet reds nothing here.",
	},
}

// wsaVariableCommandRegister is the priced table of seat runs this pin's
// bwrap LITERAL cannot see: an `exec.Command` whose program is a variable.
// Each row is file · the top-level test it is in · what was decided and why.
// It exists so that the hole is named rather than silently covered, and so a
// NEW invisible seat runner cannot arrive without a reader: arm 2 reds on a
// variable-program site this table does not name, and on a row that names
// nothing.
//
// The disposition is not a flag this pin acts on — every ROUTED row below is
// already routed in its file and arm 2 says so by reading the owning
// function, exactly as arm 1 does. An EXEMPT row is the one shape where
// routing would be WRONG, and there is one: a seat THIS PROCESS kills. The
// criterion cannot tell our own SIGKILL from the box's, so routing it would
// abstain on every ordinary run and hold the wall check at exit 3 forever
// (ADR 0097 D3) — which is the one direction D5's watchdog reports as silence
// rather than as a wall regression.
var wsaVariableCommandRegister = []struct {
	file, test, why string
	routed          bool
}{
	{
		file: "bwraphide_qa_test.go", test: "TestLiveBwrapHideIsNotWalkedAroundThroughProcRoot",
		routed: true,
		why: "argv[0] is the rendered bwrap argv for the walled arm and `sh` for the control, " +
			"so the program is a variable. The fatal it guards reads the status as the probe failing, " +
			"which is the same false red a literal site has.",
	},
	{
		file: "wallwatchbackstoplive_linux_test.go", test: "TestLiveWallWatchBackstopKillsASeatWhoseWatcherIsGone",
		routed: true,
		why: "the seat runner is posse's own wall-watch verb, exec'd as os.Executable(), not bwrap — " +
			"bwrap is its argument. The fatal reads a missing bgpid file as the seat never running, " +
			"so a watcher the box ended arrives as that finding; the status is read before this " +
			"process's own Kill and non-blocking, since a slow watcher must still reach the fatal.",
	},
	{
		file: "wallwatchbackstoplive_linux_test.go", test: "TestLiveWallWatchBackstopFilesAWatcherThatDiedUnderARunningSeat",
		routed: true,
		why:    "the same runner and the same fatal as the arm above, in the other D3 arm.",
	},
	{
		file: "grokleader_qa_test.go", test: "TestLiveWallGrokLeaderUseLeaderTUIStartsNoLeader",
		routed: false,
		why: "EXEMPT: the program is util-linux `script`, which holds the bwrap line under a pty, and " +
			"THIS TEST kills it — `cmd.Process.Kill()` on both arms, by design, after the socket " +
			"deadline. Every run therefore ends `signal: killed` and the criterion cannot tell that " +
			"from the box's kill, so routing it would abstain always. Nothing here reads the exit as " +
			"a verdict either: the arms read whether leader.sock appeared and whether any leader " +
			"process is still alive.",
	},
}

// TestQAEveryWallMemberSeatRunRoutesTheAbstention is the widening of
// ranger-base-evx07's one file to the whole of ADR 0097 D3's member set.
func TestQAEveryWallMemberSeatRunRoutesTheAbstention(t *testing.T) {
	t.Parallel()
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.go") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	members, memberFiles, sites := 0, 0, 0
	var unrouted []string
	// Arm 2's two halves, collected over the same walk: the
	// variable-program sites found, and the register rows they match.
	found := map[string]bool{}
	matched := map[int]bool{}
	// Arm 3 is the watcher-run half (ranger-base-jqdm2), collected the same
	// way: its own unrouted sites, and its own register.
	wSites := 0
	var wUnrouted []string
	wFound := map[string]bool{}
	wMatched := map[int]bool{}
	for _, name := range names {
		src, rerr := os.ReadFile(name)
		if rerr != nil {
			t.Fatal(rerr)
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, name, src, 0)
		if perr != nil {
			t.Fatalf("%s: %v", name, perr)
		}
		mem := wsaMembers(f)
		if len(mem) == 0 {
			continue
		}
		members += len(mem)
		memberFiles++

		bad, n := wsaUnrouted(t, name, string(src))
		sites += n
		unrouted = append(unrouted, bad...)

		wBad, wn := wsaWatcherUnrouted(t, name, string(src))
		wSites += wn
		wUnrouted = append(wUnrouted, wBad...)

		// The watcher half's register: a runWallWatch whose Bwrap is a
		// variable, so this rule cannot tell whether it starts a seat.
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(x ast.Node) bool {
				c, ok := x.(*ast.CallExpr)
				if !ok {
					return true
				}
				isRun, _, readable := wsaWatcherLiveSeat(c)
				if !isRun || readable {
					return true
				}
				wFound[name+"\x00"+fn.Name.Name] = true
				for i, row := range wsaWatcherSiteRegister {
					if row.file == name && row.test == fn.Name.Name {
						wMatched[i] = true
						if row.routed && !qcsHas(qcsCalls(fn.Body), wsaWatcherRouter) {
							t.Errorf("%s: %s is registered as routing the abstention for a watcher-run seat whose bwrap argv is a variable, and it does not call %s. %s", name, fn.Name.Name, wsaWatcherRouter, row.why)
						}
						if !row.routed && qcsHas(qcsCalls(fn.Body), wsaWatcherRouter) {
							t.Errorf("%s: %s is registered EXEMPT and now calls %s. Read the row before you keep it: %s", name, fn.Name.Name, wsaWatcherRouter, row.why)
						}
						return true
					}
				}
				t.Errorf("%s:%d: %s runs a seat through %s whose bwrap argv is a VARIABLE, and no row of wsaWatcherSiteRegister names it. This rule reads the literal `Bwrap: []string{\"bwrap\", …}`, so a fixture spelled this way is invisible to it. Add a row saying whether this site routes %s (ranger-base-jqdm2's reason: a seat the box SIGKILLs is silent and the watcher hands that back as 128+9, which every arm here compares to a number it expected) or why routing it would be wrong.",
					name, fset.Position(c.Pos()).Line, fn.Name.Name, wsaWatcherRun, wsaWatcherRouter)
				return true
			})
		}

		// The variable-program half, in the same member file.
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(x ast.Node) bool {
				c, ok := x.(*ast.CallExpr)
				if !ok {
					return true
				}
				isExec, literal := wsaCommandArgIsLiteral(c)
				if !isExec || literal {
					return true
				}
				key := name + "\x00" + fn.Name.Name
				found[key] = true
				for i, row := range wsaVariableCommandRegister {
					if row.file == name && row.test == fn.Name.Name {
						matched[i] = true
						if row.routed && !qcsHas(qcsCalls(fn.Body), wsaRouter) {
							t.Errorf("%s: %s is registered as routing the abstention for a seat run whose program is a variable, and it does not call %s. %s", name, fn.Name.Name, wsaRouter, row.why)
						}
						if !row.routed && qcsHas(qcsCalls(fn.Body), wsaRouter) {
							t.Errorf("%s: %s is registered EXEMPT and now calls %s. Read the row before you keep it: %s", name, fn.Name.Name, wsaRouter, row.why)
						}
						return true
					}
				}
				t.Errorf("%s:%d: %s runs a command whose program is a VARIABLE, and no row of wsaVariableCommandRegister names it. This pin's rule reads the literal `exec.Command(\"bwrap\", …)`, so a seat run spelled this way is invisible to it — which is the hole the register exists to keep named. Add a row saying whether this site routes the abstention (ranger-base-k2quj's reason: a seat the box SIGKILLs is silent and exits 128+9, which a success arm reds and a refusal arm PASSES) or why routing it would be wrong.",
					name, fset.Position(c.Pos()).Line, fn.Name.Name)
				return true
			})
		}
	}

	for _, u := range unrouted {
		t.Errorf("%s runs a seat inside a rendered wall and does not route the abstention. Call %s(t, <what ran>, err, out) with the run's status before anything reads it: a seat the box ends says NOTHING and exits 128+9 or 128+15, so an unguarded runner reduces it to `err == nil` and a refusal arm PASSES on it while a success arm REDS — and that red is ADR 0097 D3's exit 1, which ADR 0088 D5's ratchet files as one P1 `-l code` bead against whoever landed last (ranger-base-a688i, -w6kke, -axt1f, -s9h52 were all this). The abstention is D3's exit 3: never a pass, never a red. bwrapgitrefslive_test.go holds the criterion and what was measured of it.",
			u, wsaRouter)
	}

	for _, u := range wUnrouted {
		t.Errorf("%s runs a seat through %s, or reads the status of one, and does not route the abstention. Call %s(t, <what ran>, code, said) with the watcher's exit code before anything compares it to a number: a seat the box ends says NOTHING and the watcher hands that back as 128+9 or 128+15, so a `want 1`, a `want 7` or a `want 137` reads a death that measured nothing as a wall regression — ADR 0097 D3's exit 1, which ADR 0088 D5's ratchet files as one P1 `-l code` against whoever landed last. ranger-base-jqdm2 was exactly that: `watcher exit = 137, want 7 (the seat's own): NOTHING`, charged to a landing that compiles no watcher. The abstention is D3's exit 3: never a pass, never a red. wallwatchlive_test.go holds the door and bwrapgitrefslive_test.go the criterion.",
			u, wsaWatcherRun, wsaWatcherRouter)
	}

	for i, row := range wsaWatcherSiteRegister {
		if !wMatched[i] {
			t.Errorf("wsaWatcherSiteRegister row %d names %s in %s, and this walk found no watcher-run seat with a variable bwrap argv there. A stale row is a hole nobody is reading any more: delete it, or follow the site to its new home.", i, row.test, row.file)
		}
	}

	for i, row := range wsaVariableCommandRegister {
		if !matched[i] {
			t.Errorf("wsaVariableCommandRegister row %d names %s in %s, and this walk found no seat run with a variable program there. A stale row is a hole nobody is reading any more: delete it, or follow the site to its new home.", i, row.test, row.file)
		}
	}

	// THE CONTROLS. A pin over a derived set is satisfied by deriving
	// nothing, so each count is floored at well under today's reading and
	// the whole census is logged. 55 members in 24 files with 33 literal
	// bwrap seat runs on 2026-11-22; the floors are there to catch a seed
	// that stopped matching, not a bead that deletes a test.
	if members < 40 || memberFiles < 18 {
		t.Errorf("this pin found %d members in %d files (55 in 24 on 2026-11-22) — the `%s` reading has stopped matching and the class this holds is losing members silently",
			members, memberFiles, wallVenueMemberPattern)
	}
	if sites < 25 {
		t.Errorf("this pin found %d bwrap seat runs in the member set (33 on 2026-11-22) — the literal reading has stopped matching, and an unrouted runner would now pass", sites)
	}
	if len(found) < 3 {
		t.Errorf("this pin found %d member tests running a command with a variable program (4 rows on 2026-11-22) — arm 2's reading has stopped matching and the register is holding nothing", len(found))
	}
	// The watcher half's floors (ranger-base-jqdm2). MEASURED 2026-11-22: 8
	// sites in the three member files that run a seat that way — 2 runs read
	// in place and 6 reads of a status a producer handed to a channel (the
	// three other live runWallWatch fixtures ARE those producers, so they are
	// counted at their read and not twice).
	if wSites < 6 {
		t.Errorf("this pin found %d watcher-run status sites in the member set (8 on 2026-11-22) — the `%s` reading or the chan-int reading has stopped matching, and an unrouted watcher member would now pass", wSites, wsaWatcherRun)
	}
	if len(wFound) < 1 {
		t.Errorf("this pin found %d member tests running a watcher seat with a variable bwrap argv (1 row on 2026-11-22) — arm 3's register is holding nothing", len(wFound))
	}
	t.Logf("%d members in %d files, %d literal bwrap seat runs, %d variable-program member tests, %d watcher-run sites, %d variable-argv watcher tests", members, memberFiles, sites, len(found), wSites, len(wFound))
}

// TestQAWallMemberAbstentionRuleMatchesWhatItClaims drives the rule above
// over planted source, positive and negative. The live answer is zero
// unrouted sites once this bead lands, which is exactly when a rule that
// matches nothing looks identical to a tree that is clean.
func TestQAWallMemberAbstentionRuleMatchesWhatItClaims(t *testing.T) {
	t.Parallel()
	const head = "package posse\n"
	for _, c := range []struct {
		name string
		src  string
		want int // unrouted sites
	}{
		{"a runner that routes is clean", head + `
func TestLiveWallX(t *testing.T) {
	out, err := exec.Command("bwrap", "sh", "-c", "x").CombinedOutput()
	bgSeatAbstainIfOutside(t, "x", err, string(out))
	_ = out
}`, 0},
		{"a runner that does not route is the finding", head + `
func TestLiveWallX(t *testing.T) {
	out, err := exec.Command("bwrap", "sh", "-c", "x").CombinedOutput()
	if err != nil { t.Fatal(string(out)) }
}`, 1},
		{"the criterion's own pin is not a finding", head + `
func TestLiveWallX(t *testing.T) {
	out, err := exec.Command("bwrap", "sh", "-c", "x").CombinedOutput()
	if !bgSeatEndedFromOutside(err, string(out)) { t.Fatal("x") }
}`, 0},
		{"a closure runner is read, not its test", head + `
func TestLiveWallX(t *testing.T) {
	try := func(s string) bool {
		out, err := exec.Command("bwrap", "/bin/sh", "-c", s).CombinedOutput()
		_ = out
		return err == nil
	}
	bgSeatAbstainIfOutside(t, "x", nil, "")
	_ = try
}`, 1},
		{"routing inside the closure is what counts", head + `
func TestLiveWallX(t *testing.T) {
	try := func(s string) bool {
		out, err := exec.Command("bwrap", "/bin/sh", "-c", s).CombinedOutput()
		bgSeatAbstainIfOutside(t, "x", err, string(out))
		return err == nil
	}
	_ = try
}`, 0},
		{"a BUILDER steps out to the function that runs", head + `
func TestLiveWallX(t *testing.T) {
	walled := func(s string) *exec.Cmd { return exec.Command("bwrap", "/bin/sh", "-c", s) }
	out, err := walled("x").CombinedOutput()
	bgSeatAbstainIfOutside(t, "x", err, string(out))
	_ = out
}`, 0},
		{"a builder whose caller does not route is the finding", head + `
func TestLiveWallX(t *testing.T) {
	walled := func(s string) *exec.Cmd { return exec.Command("bwrap", "/bin/sh", "-c", s) }
	out, err := walled("x").CombinedOutput()
	if err != nil { t.Fatal(string(out)) }
}`, 1},
		{"CommandContext is read too", head + `
func TestLiveWallX(t *testing.T) {
	out, err := exec.CommandContext(ctx, "bwrap", "/bin/sh", "-c", "x").CombinedOutput()
	if err != nil { t.Fatal(string(out)) }
}`, 1},
		{"a seat run started and waited is read", head + `
func TestLiveWallX(t *testing.T) {
	cmd := exec.Command("bwrap", "sh", "-c", "x")
	if err := cmd.Start(); err != nil { t.Fatal(err) }
	if err := cmd.Wait(); err != nil { t.Fatal(err) }
}`, 1},
		{"a command that is not bwrap is nobody's seat", head + `
func TestLiveWallX(t *testing.T) {
	out, err := exec.Command("git", "status").CombinedOutput()
	if err != nil { t.Fatal(string(out)) }
}`, 0},
		{"a variable program is invisible to this rule", head + `
func TestLiveWallX(t *testing.T) {
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil { t.Fatal(string(out)) }
}`, 0},
	} {
		got, _ := wsaUnrouted(t, "planted.go", c.src)
		if len(got) != c.want {
			t.Errorf("%s: the rule found %d unrouted site(s) %v, want %d", c.name, len(got), got, c.want)
		}
	}

	// THE WATCHER-RUN HALF's controls (ranger-base-jqdm2), over the same
	// planted-source rule. Every row is a shape the three real member files
	// have, plus the two negatives that say what this rule must NOT claim.
	for _, c := range []struct {
		name string
		src  string
		want int // unrouted sites
	}{
		{"a live watcher fixture that routes is clean", head + `
func TestLiveWallX(t *testing.T) {
	got := runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: []string{"bwrap", "--args", "3"}}, Stdout: &out})
	wwAbstainIfOutside(t, "x", got, out.String())
	if got != 7 { t.Errorf("x") }
}`, 0},
		{"a live watcher fixture that does not route is the finding", head + `
func TestLiveWallX(t *testing.T) {
	got := runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: []string{"bwrap", "--args", "3"}}, Stdout: &out})
	if got != 7 { t.Errorf("x") }
}`, 1},
		{"the run in a goroutine and the read in a closure are ONE owner", head + `
func TestLiveWallX(t *testing.T) {
	code := make(chan int, 1)
	go func() { code <- runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: []string{"bwrap", "--args", "3"}}}) }()
	t.Run("arm", func(t *testing.T) {
		c := <-code
		wwAbstainIfOutside(t, "x", c, "")
		if c != 1 { t.Errorf("x") }
	})
}`, 0},
		{"the same shape with the routing deleted is the finding, at the READ", head + `
func TestLiveWallX(t *testing.T) {
	code := make(chan int, 1)
	go func() { code <- runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: []string{"bwrap", "--args", "3"}}}) }()
	t.Run("arm", func(t *testing.T) {
		c := <-code
		if c != 1 { t.Errorf("x") }
	})
}`, 1},
		{"TWO arms, and an arm that loses its routing is found although its sibling keeps one", head + `
func TestLiveWallX(t *testing.T) {
	code := make(chan int, 1)
	go func() { code <- runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: []string{"bwrap", "--args", "3"}}}) }()
	t.Run("killed", func(t *testing.T) {
		c := <-code
		wwAbstainIfOutside(t, "x", c, "")
		if c != 137 { t.Errorf("x") }
	})
	t.Run("control", func(t *testing.T) {
		c := <-code
		if c != 7 { t.Errorf("x") }
	})
}`, 1},
		{"a transport closure in a test whose subject is the criterion is not a finding", head + `
func TestLiveWallX(t *testing.T) {
	run := func(script string) (int, string) {
		code := make(chan int, 1)
		go func() { code <- runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: []string{"bwrap", "--args", "3"}}}) }()
		c := <-code
		return c, ""
	}
	got, said := run("x")
	if !bgSeatStatusFromOutside(got, false, said) { t.Fatal("x") }
}`, 0},
		{"the SAME closure in a test that grades nothing is still the finding", head + `
func TestLiveWallX(t *testing.T) {
	run := func(script string) (int, string) {
		code := make(chan int, 1)
		go func() { code <- runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: []string{"bwrap", "--args", "3"}}}) }()
		c := <-code
		return c, ""
	}
	got, said := run("x")
	if got != 7 { t.Fatal(said) }
}`, 1},
		{"a run whose value is handed to a channel is a producer, not a site", head + `
func TestLiveWallX(t *testing.T) {
	code := make(chan int, 1)
	go func() { code <- runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: []string{"bwrap", "--args", "3"}}}) }()
	_ = code
}`, 0},
		{"a helper that takes the channel is read by its own parameter", head + `
func wwWaitX(t *testing.T, code chan int, said *wwSaid) {
	if c := <-code; c != 1 { t.Errorf("x") }
}
func TestLiveWallX(t *testing.T) { wwWaitX(t, nil, nil) }`, 1},
		{"the criterion's own pin is not a finding", head + `
func TestLiveWallX(t *testing.T) {
	got := runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: []string{"bwrap", "--args", "3"}}})
	if !bgSeatStatusFromOutside(got, false, "") { t.Fatal("x") }
}`, 0},
		{"/bin/false starts no seat, so it is nobody's", head + `
func TestLiveWallX(t *testing.T) {
	got := runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: []string{"/bin/false"}}})
	if got == 0 { t.Errorf("x") }
}`, 0},
		{"a variable bwrap argv is invisible to this rule (the register carries it)", head + `
func TestLiveWallX(t *testing.T) {
	got := runWallWatch(wallWatchRun{Opts: wallWatchOpts{Bwrap: c.bwrap}})
	if got == 0 { t.Errorf("x") }
}`, 0},
		{"a receive from a channel that is not a chan int is not a status", head + `
func TestLiveWallX(t *testing.T) {
	waited := make(chan struct{})
	<-waited
}`, 0},
		{"a SEND of a status is not a read", head + `
func TestLiveWallX(t *testing.T) {
	code := make(chan int, 1)
	code <- 7
}`, 0},
	} {
		got, _ := wsaWatcherUnrouted(t, "planted.go", c.src)
		if len(got) != c.want {
			t.Errorf("%s: the watcher rule found %d unrouted site(s) %v, want %d", c.name, len(got), got, c.want)
		}
	}

	// And the member reading, which is the OTHER half: a file with no member
	// in it is not read at all, so a pattern that stopped matching would
	// take the whole census out while every arm above stayed green.
	for _, c := range []struct {
		name, src string
		want      int
	}{
		{"a live bwrap member", head + "func TestLiveBwrapX(t *testing.T) {}", 1},
		{"a live wall member", head + "func TestLiveWallX(t *testing.T) {}", 1},
		{"a QA live member", head + "func TestQALiveWallX(t *testing.T) {}", 1},
		{"an ordinary test is no member", head + "func TestWallX(t *testing.T) {}", 0},
		{"a helper named like one is no member", head + "func TestLiveWallX(t *testing.T) {}\nfunc (f fx) TestLiveWallY() {}", 1},
		{"a member named past the anchor is no member", head + "func TestXLiveWall(t *testing.T) {}", 0},
	} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "planted.go", c.src, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := wsaMembers(f); len(got) != c.want {
			t.Errorf("%s: wsaMembers found %v, want %d", c.name, got, c.want)
		}
	}
}
