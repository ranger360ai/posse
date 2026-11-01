package treepins

// QA pin for ranger-base-bm6j8: every binary posse execs from a launcher
// process runs outside every gates dir (ADR 0077 D3).
//
// THE MECHANISM, which internal/posse/protocolchild.go states at length. A
// posse process that dispatches, launches, checks parity or renders a wall is
// a child of whatever shell asked for it — routinely another persona's SEAT,
// whose gates bin dir leads its PATH. A binary posse execs as part of its own
// protocol is posse's, not that persona's (ADR 0009: the walls bind what a
// persona TYPES), so it is resolved and run as though no gates dir were
// there.
//
// MEASURED 2026-10-31 (ranger-base-c33j6, twice): posse took ADR 0073's
// per-store write lock, exec'd `bd` by bare name, landed on the launching
// persona's `bd` shim, and the shim took the SAME lock on a new open file
// description — posse waited BdWriteLockWait on itself and refused the write,
// every dispatch from a gated seat. ranger-base-rzjie fixed that one call;
// this is the census the ADR's own scope line asked for ("the other exec
// sites are a census bead, each judged on whether it is protocol or a
// persona's command").
//
// WHY A PIN AND NOT THE FRAGMENT. The classification is written out in
// docs/notes.d/ranger-base-bm6j8.md, site by site. A list of 46 call sites in
// prose is wrong one landing after it is written, and the two halves of the
// hazard are both invisible at the call site: a site that execs a bare name
// reads exactly like one that was resolved, and a site that binds the
// gates-free PATH and is then handed `append(os.Environ(), …)` two lines
// later has silently lost it. ARM 2 is that second half, and it is the one
// this bead nearly shipped wrong — internal/posse/cage.go assigns `.Env`
// three times after constructing its child.
//
// THE RULE IS UNCONDITIONAL over the shipped tree's `exec.Command` and
// `exec.CommandContext`, and the exemptions are keyed by the FUNCTION rather
// than by the binary — the opposite of ewdExemptBinaries next door, for a
// reason that is the whole of this classification. WaitDelay asks "can this
// child leave a descendant holding the pipes", a fact about the program being
// run. This asks "whose environment is this process", a fact about which
// posse process reaches the line: the same `docker` argv is protocol when
// parity asks the engine a question and the SEAT's own when
// `posse <runtime> --posse-cage-launch` becomes it in the pane.
//
// Non-test sources only. A test that shells a gated binary is the suite's own
// business; a shipped call that does it is a wedged dispatch pass.
//
// THE BEHAVIOUR IS PINNED WHERE IT HAPPENS, not here: this census is
// syntactic, so a protocolEnv that stopped setting PATH would leave every arm
// below green. internal/posse/bdgatesfreepath_test.go is the proof the census
// is about something — a fake gates bin dir leading PATH whose `bd` shim takes
// the real ADR 0073 store lock, and a posse write that completes instead of
// waiting it out. Since ranger-base-bm6j8 that fixture runs through these very
// two functions (Bd.bin is one call of protocolBin, bdChildEnv one of
// protocolEnv), so it is the behavioural arm for the whole rule and not only
// for bd — including ranger-base-cwsi8's half, which measured that a bare
// name nothing outside the gates answers must be an ERROR and not something
// handed to exec (TestBdBinUnresolvableOutsideTheGatesIsAnError).
//
// MEASURED 2026-10-31, each mutant against the one arm it should red:
// protocolEnv returning its argument unchanged reds that file's child-PATH
// arm; protocolBin returning the bare name instead of an error reds its
// deadlock arm with ranger-base-c33j6's sentence verbatim; dropping one
// protocolCmd in the shipped tree reds arm 1 here naming that file and line;
// dropping one protocolEnv from a `.Env` assignment reds arm 2.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// pcExemptFuncs is the ruling half: the shipped functions whose exec is NOT
// posse's protocol, keyed "<file>:<func>" and carrying why. A row is a claim
// that the process reaching this line is the seat or the wall rather than a
// launcher — and therefore that a gates dir leading its PATH is the point and
// not the bug.
var pcExemptFuncs = map[string]string{
	"internal/posse/cagelauncher.go:runCageSteps": "the engine steps of a cage " +
		"launch. RunCageLaunch is the pane's own argv0 (the launcher symlink " +
		"herdr identifies the session by), so this runs IN the seat and not in " +
		"any launcher: its environment is the one posse TYPED onto that seat " +
		"(ADR 0077 D1/D2) plus the seat's own GatePrefix, and a seat's walls " +
		"are its own PID's (ADR 0017). Reached from StartCageEgress and from " +
		"RunCageReap, the watcher this launch forks.",
	"internal/posse/cagelauncher.go:watchCageEgress": "the egress watcher, " +
		"forked by RunCageLaunch above and re-exec'ing THIS binary by the path " +
		"os.Executable named — an absolute path, so there is no name for a shim " +
		"to answer to. Same process and same reason as the row above.",
}

// pcEnvBinders are the functions that put ADR 0077 D3's PATH onto a child's
// environment — DERIVED, never listed. protocolEnv is the rule; anything
// whose body calls it (bdChildEnv, which adds ADR 0073's store binding
// first) is a second honest route to the same answer, and a roster of them
// here would be the place that goes stale.
func pcEnvBinders(files map[string]*ast.File) map[string]bool {
	out := map[string]bool{"protocolEnv": true}
	// Transitive, so a binder written in terms of another binder counts:
	// run to a fixed point rather than one hop, which is the difference
	// between a derivation and a one-deep special case.
	for {
		grew := false
		for _, f := range files {
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Body == nil || out[fn.Name.Name] {
					continue
				}
				if pcReturnsStrings(fn) && pcCallsAny(fn.Body, out) {
					out[fn.Name.Name], grew = true, true
				}
			}
		}
		if !grew {
			return out
		}
	}
}

// pcReturnsStrings reports whether fn returns a []string — what an
// environment IS. It is the narrowing that keeps the derivation to functions
// that hand back an environment: protocolCmd calls protocolEnv too, and
// without this it would be derived as a binder and then satisfy an `.Env`
// assignment that reaches no environment of posse's at all.
func pcReturnsStrings(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, r := range fn.Type.Results.List {
		arr, ok := r.Type.(*ast.ArrayType)
		if !ok || arr.Len != nil {
			continue
		}
		if id, ok := arr.Elt.(*ast.Ident); ok && id.Name == "string" {
			return true
		}
	}
	return false
}

// pcCallsAny reports whether n holds a call to any function named in names.
func pcCallsAny(n ast.Node, names map[string]bool) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		call, ok := x.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && names[id.Name] {
			found = true
		}
		return true
	})
	return found
}

// pcSite is one exec.Command/exec.CommandContext call in the shipped tree.
type pcSite struct {
	file    string
	line    int
	fn      string // the enclosing FuncDecl's name, "" at package level
	wrapped bool   // the exec call is protocolCmd's own argument
	cmdVar  string // the identifier the child was bound to; "" when chained
	envOK   bool   // (on a watched `.Env` assignment) its RHS reaches an env binder
}

// pcFuncAt names the innermost FuncDecl containing p. A FuncLit reports the
// declaration it sits inside, which is what the exemption roster keys on: a
// closure is reached by the same process its function is.
func pcFuncAt(f *ast.File, p token.Pos) string {
	name, span := "", token.Pos(0)
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil || p < fn.Pos() || p >= fn.End() {
			continue
		}
		if span == 0 || fn.End()-fn.Pos() < span {
			name, span = fn.Name.Name, fn.End()-fn.Pos()
		}
	}
	return name
}

// pcIsExec reports whether e is an `exec.Command`/`exec.CommandContext` call.
func pcIsExec(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	return ewdIsSel(call.Fun, "exec", "Command") || ewdIsSel(call.Fun, "exec", "CommandContext")
}

// pcScan reads one parsed file for its exec sites, and for the `.Env`
// assignments that could take a bound child's PATH back off it.
func pcScan(fset *token.FileSet, rel string, f *ast.File, binders map[string]bool) (sites []pcSite, watched []pcSite) {
	// Which exec calls are protocolCmd's own argument, and which identifier
	// each was bound to. Collected first: a site's own node can see neither
	// the wrapper above it nor the assignment it is the right-hand side of.
	wrapped := map[ast.Node]bool{}
	bound := map[ast.Node]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			id, ok := node.Fun.(*ast.Ident)
			if !ok || id.Name != "protocolCmd" || len(node.Args) == 0 {
				return true
			}
			if pcIsExec(node.Args[0]) {
				wrapped[node.Args[0]] = true
			}
		case *ast.AssignStmt:
			if len(node.Lhs) != len(node.Rhs) {
				return true
			}
			for i, rhs := range node.Rhs {
				inner := rhs
				// `x := protocolCmd(exec.Command(…))` binds the same child
				// as `x := exec.Command(…)` does.
				if call, ok := rhs.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "protocolCmd" && len(call.Args) > 0 {
						inner = call.Args[0]
					}
				}
				if !pcIsExec(inner) {
					continue
				}
				if id, ok := node.Lhs[i].(*ast.Ident); ok {
					bound[inner] = id.Name
				}
			}
		}
		return true
	})

	// The sites.
	var found []pcSite
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !pcIsExec(call) {
			return true
		}
		found = append(found, pcSite{
			file:    rel,
			line:    fset.Position(call.Pos()).Line,
			fn:      pcFuncAt(f, call.Pos()),
			wrapped: wrapped[ast.Node(call)],
			cmdVar:  bound[ast.Node(call)],
		})
		return true
	})

	// Arm 2's half: an assignment `<bound ident>.Env = …` whose right-hand
	// side reaches no env binder, inside the function that bound it. Two
	// functions in one file routinely both call their child `cmd`, so the
	// match is by identifier AND by enclosing function — ewdScan's rule.
	byFunc := map[string]map[string]bool{}
	for _, s := range found {
		if s.cmdVar == "" {
			continue
		}
		if byFunc[s.fn] == nil {
			byFunc[s.fn] = map[string]bool{}
		}
		byFunc[s.fn][s.cmdVar] = true
	}
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			sel, ok := lhs.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Env" {
				continue
			}
			recv, ok := sel.X.(*ast.Ident)
			if !ok {
				continue
			}
			fn := pcFuncAt(f, as.Pos())
			if !byFunc[fn][recv.Name] {
				continue // not a child this function exec'd
			}
			watched = append(watched, pcSite{
				file: rel, line: fset.Position(as.Pos()).Line, fn: fn, cmdVar: recv.Name,
				envOK: i < len(as.Rhs) && pcCallsAny(as.Rhs[i], binders),
			})
		}
		return true
	})
	return found, watched
}

func TestQAEveryProtocolChildRunsOutsideTheGates(t *testing.T) {
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

	// The positive witness. Every arm below is an assertion of absence, and
	// a walk that read nothing says exactly what a clean tree says. The
	// floors sit under the live counts (MEASURED 2026-10-31 at
	// ranger-base-tpy6q's commit: 160 non-test .go files in the tree, 46
	// exec.Command/CommandContext sites — 44 through protocolCmd and 2 ruled
	// exempt — 5 `.Env` assignments on a child one of them exec'd, all five
	// through a binder, and 2 binders derived).
	if len(files) < 100 {
		t.Fatalf("the walk read %d non-test .go files under %s — the corpus this pin censuses is not there",
			len(files), ewdCorpusRoot)
	}

	binders := pcEnvBinders(files)
	if len(binders) < 2 {
		t.Errorf("derived %d env binder(s) %v — internal/posse's bdChildEnv calls protocolEnv and is that shape, so a derivation finding only protocolEnv itself has stopped reading rather than found a tree without binders: every `.Env` assignment routed through a helper would now read as a clobber.",
			len(binders), binders)
	}

	var all, watched []pcSite
	for _, rel := range rels {
		s, w := pcScan(fset, rel, files[rel], binders)
		all = append(all, s...)
		watched = append(watched, w...)
	}
	if len(all) < 30 {
		t.Fatalf("found %d exec.Command/CommandContext sites in %d files — the scan is not reading this tree's children",
			len(all), len(files))
	}

	// ARM 1 — every site is posse's protocol and goes through protocolCmd,
	// or its function is ruled the seat's.
	seenExempt := map[string]bool{}
	wrapped := 0
	for _, s := range all {
		if s.wrapped {
			wrapped++
			continue
		}
		key := s.file + ":" + s.fn
		if _, ok := pcExemptFuncs[key]; ok {
			seenExempt[key] = true
			continue
		}
		t.Errorf("%s:%d (%s) execs a child without protocolCmd.\n"+
			"  A posse process that launches or dispatches can have another persona's gates bin dir leading its PATH — donated by the seat it was dispatched from, and on herdr homes by a server autostarted hours earlier (MEASURED 2026-10-31, ranger-base-c33j6). A bare name then resolves to that persona's shim: posse's own `bd` call deadlocked on ADR 0073's store lock that way, every dispatch from a gated seat.\n"+
			"  Wrap the exec call — `protocolCmd(exec.Command(…))` — which resolves the binary outside every gates dir and hands the child that PATH (ADR 0077 D3, internal/posse/protocolchild.go). If this line is reached by the SEAT or by a shim rather than by a launcher, add %q to pcExemptFuncs with that reason, the way cagelauncher.go's two rows are.",
			s.file, s.line, s.fn, key)
	}
	if wrapped == 0 {
		t.Error("not one site in the tree was read as going through protocolCmd, and 44 did on 2026-10-31.\n" +
			"  Either the scan stopped resolving the wrapper or the tree really did lose it; a pin that excuses every site through the exempt roster and reports nothing wrapped is measuring the roster, not the tree.")
	}

	// ARM 2 — and nothing takes it back off. A child constructed through
	// protocolCmd and then assigned `.Env = append(os.Environ(), …)` runs on
	// the launcher's PATH again, with the wrapper still on the line above
	// saying otherwise.
	// Its own positive witness, for the same reason every other arm has
	// one: "no assignment takes the PATH back off" is exactly what a scan
	// that watches nothing reports, and this arm's whole reach is the
	// identifier-and-function match above it. Five on 2026-10-31
	// (internal/posse: adrcensus.go, beads.go, cage.go x3).
	if len(watched) < 3 {
		t.Errorf("the scan watched %d `.Env` assignment(s) on a child one of these functions exec'd, and five sit in the tree today — internal/posse/cage.go's BuildCageImage alone has three. A count this low means the identifier-and-function match has stopped resolving, and arm 2 below is then green over whatever the tree does.",
			len(watched))
	}
	for _, c := range watched {
		if c.envOK {
			continue
		}
		t.Errorf("%s:%d assigns %s.Env in %s without going through protocolEnv.\n"+
			"  cmd.Env REPLACES the environment, so this overwrites the gates-free PATH protocolCmd put on that child and the call site still reads as though it had one.\n"+
			"  Hand the list to protocolEnv — `%s.Env = protocolEnv(append(os.Environ(), …))` — or to a helper that calls it, the way internal/posse/beads.go's bdChildEnv does.",
			c.file, c.line, c.cmdVar, c.fn, c.cmdVar)
	}

	// And the ruling's own rows have to be real: a row for a function that
	// no longer execs anything is a door held open for nothing, and this
	// roster's rows are the one exemption a reader cannot see from the call
	// site.
	var keys []string
	for k := range pcExemptFuncs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !seenExempt[k] {
			t.Errorf("pcExemptFuncs rules %s exempt (%s) and no shipped exec site sits in that function any more — drop the row",
				k, pcExemptFuncs[k])
		}
	}
}

// The two-way half: the scan has to be able to SAY NO. One planted shape per
// route a site can be satisfied or lost by, run through the same reader —
// because every arm above is an assertion of absence, and an absence is what
// a reader that resolves nothing reports.
func TestQATheProtocolChildCensusCanStillSayNo(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		src         string
		wantBare    bool // the scan must find a site outside protocolCmd
		wantClobber bool // …and/or an .Env assignment that loses the PATH
	}{
		{"bare", `package x
import "os/exec"
func f() { _ = exec.Command("git", "status") }
`, true, false},
		{"wrapped", `package x
import "os/exec"
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func f() { _ = protocolCmd(exec.Command("git", "status")) }
`, false, false},
		{"wrapped, context form", `package x
import ("context"; "os/exec")
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func f(ctx context.Context) { _ = protocolCmd(exec.CommandContext(ctx, "git", "status")) }
`, false, false},
		{"wrapped but another call's argument is not the exec call", `package x
import "os/exec"
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func f() { c := exec.Command("git", "status"); _ = protocolCmd(c) }
`, true, false},
		{"wrapped, then the env taken back off", `package x
import ("os"; "os/exec")
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func f() { c := protocolCmd(exec.Command("git", "status")); c.Env = append(os.Environ(), "GIT_DIR=x") }
`, false, true},
		{"wrapped, env through the binder", `package x
import ("os"; "os/exec")
func protocolEnv(e []string) []string { return e }
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func f() { c := protocolCmd(exec.Command("git", "status")); c.Env = protocolEnv(append(os.Environ(), "GIT_DIR=x")) }
`, false, false},
		{"wrapped, env through a derived binder", `package x
import ("os"; "os/exec")
func protocolEnv(e []string) []string { return e }
func childEnv(e []string) []string { return protocolEnv(e) }
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func f() { c := protocolCmd(exec.Command("git", "status")); c.Env = childEnv(os.Environ()) }
`, false, false},
		{"an .Env on something this function never exec'd is not a clobber", `package x
import ("os"; "os/exec")
type wrap struct{ Env []string }
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func f() { _ = protocolCmd(exec.Command("git", "status")); var w wrap; w.Env = append(os.Environ(), "A=b") }
`, false, false},
		{"another function's .Env assignment is not this one's", `package x
import ("os"; "os/exec")
func protocolEnv(e []string) []string { return e }
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func g() { cmd := protocolCmd(exec.Command("git", "log")); cmd.Env = protocolEnv(os.Environ()) }
func f() { cmd := protocolCmd(exec.Command("git", "status")); cmd.Env = append(os.Environ(), "A=b") }
`, false, true},
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
			sites, watched := pcScan(fset, "x.go", f, pcEnvBinders(files))
			if len(sites) == 0 {
				t.Fatalf("the scan found no exec site at all in:\n%s", tc.src)
			}
			var bare int
			for _, s := range sites {
				if !s.wrapped {
					bare++
				}
			}
			if got := bare > 0; got != tc.wantBare {
				t.Errorf("unwrapped sites = %d (want any = %v) over %d sites; the census reads this shape wrong, so arm 1 is measuring nothing",
					bare, tc.wantBare, len(sites))
			}
			lost := 0
			for _, w := range watched {
				if !w.envOK {
					lost++
				}
			}
			if got := lost > 0; got != tc.wantClobber {
				t.Errorf("clobbering .Env assignments = %d of %d watched (want any = %v); the census reads this shape wrong, so arm 2 is measuring nothing",
					lost, len(watched), tc.wantClobber)
			}
		})
	}
}

// And the derivation is read off THIS tree, not a fixture: internal/posse's
// own two env binders must both be found, since arm 2's whole quietness over
// the shipped tree rests on them.
func TestQATheEnvBinderDerivationFindsThisTreesBinders(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, rel := range ewdGoFiles(t) {
		f, err := parser.ParseFile(fset, rel, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parsing %s: %v", rel, err)
		}
		files[rel] = f
	}
	got := pcEnvBinders(files)
	for _, want := range []string{"protocolEnv", "bdChildEnv"} {
		if !got[want] {
			t.Errorf("the env-binder derivation did not find %s — every `.Env` assignment that goes through it would now read as a clobber (or, the other way, %s stopped putting ADR 0077 D3's PATH on its answer, which is the real defect)",
				want, want)
		}
	}
	if got["protocolCmd"] {
		t.Error("protocolCmd was derived as an env BINDER. It calls protocolEnv, so the derivation is right by its own rule — but arm 2 then reads `c.Env = protocolCmd(c).Env` as satisfied, which it is not. Narrow the derivation to functions that RETURN an environment.")
	}
}
