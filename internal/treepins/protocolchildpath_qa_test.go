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
//
// ARM 3 IS THE PROBE and it is a separate test function below —
// TestQAEveryProbeForAProtocolChildsNameGoesThroughProtocolAvailable
// (ranger-base-372iq), with its own ruling roster (pcProbeExemptFuncs) and its
// own planted-shape partner (TestQATheProbeCensusCanStillSayNo). Arms 1 and 2
// are about the child; this one is about the ANSWER posse gives before it runs
// one. A probe that asks exec.LookPath reads this process's gated $PATH and so
// can answer true where protocolBin refuses — a wrong yes, a plan made on it,
// and the right no arriving later (ranger-base-g1xnt, -d0nyb; five probes did
// it). Its own door row in the Makefile, because `-run` selects by test name
// and a third arm hidden inside the function above would be reachable only by
// a filter that happened to name it.
//
// It is a separate FUNCTION and not a third arm in the one above because the
// two halves it matches are a different pair: arms 1 and 2 read exec sites and
// `.Env` assignments, this one reads exec sites against PROBES, and the
// positive witness each needs is a different count. What they share is the
// file walk and pcFuncAt, which is the exemption rosters' key either way.
//
// MEASURED 2026-11-01, arm 3's mutants: Herdr.Available back to
// `resolveOutside(h.Bin, "") != ""` reds it naming herdr.go and nothing else;
// backup.go's tool gate back to `exec.LookPath(tool)` reds it naming
// backup.go, which is the row the literal-range expansion exists for; and the
// whole of ranger-base-d0nyb's pre-fix tree reds it on exactly its four
// probes (the paragraph on that function).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
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
	"cmd/coldexec/door_linux.go:supervise": "the cold-first-exec door's " +
		"supervisor (ADR 0085 D2/D6, ranger-base-97w45), which is a SEAT's " +
		"command: `make coldexec-check`, typed by a person or by a bead that " +
		"added a planted fixture. No launcher and no dispatch reaches it, and " +
		"nothing in internal/posse refers to it. Its child is os.Executable's " +
		"absolute path — this binary re-exec'd as its own supervised child — so " +
		"there is no name for a shim to answer to, the same reason as the row " +
		"above. And the PATH it hands on is the SUBJECT rather than an " +
		"accident: the door's one job is to run the command line it was given " +
		"in the environment it was given, minus the one variable it consumes, " +
		"and internal/posse's suite runs with a gates dir leading PATH on " +
		"purpose — the gated-bd arms assert what posse does there. A door that " +
		"resolved its child outside the gates would be injecting its cost into " +
		"a different suite than the one a seat runs.",
	"internal/posse/wallwatch_linux.go:runWallWatch": "the bwrap seat's " +
		"wall-watch sidecar (ranger-base-ihnnl), which the pane's launch line " +
		"runs in front of bwrap — so it runs IN the seat's line, and its child " +
		"is the seat itself. The environment it hands on is the one posse " +
		"TYPED onto that seat, gates dir leading PATH on purpose (ADR 0009), " +
		"and protocolCmd would strip the seat's own gates. The binary is the " +
		"absolute path the render resolved through protocolBin (BwrapPrefix, " +
		"bwrapBin), so there is no name for a shim to answer to.",
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

// ─── arm 3: the PROBE for such a child's name ────────────────────────────────

// pcProbeExemptFuncs is arm 3's ruling half: the shipped functions that ask
// "is there a <name>" about a name one of posse's own children execs, and
// whose answer is nonetheless NOT about that child. Keyed "<file>:<func>" the
// way pcExemptFuncs is, and carrying why.
//
// A row is one of three claims, and never that the collapse was inconvenient:
// the two spellings name two different binaries; they name two different
// PATHs, because the process that reaches the probe is not the process that
// reaches the exec; or the call is not a probe at all — a resolveOutside whose
// answer is the real target written into a rendered shim is a resolution, and
// protocolAvailable would throw away the path it needs.
var pcProbeExemptFuncs = map[string]string{
	"internal/posse/cageinner.go:PrepareGatesWrap": "the two `argv[0]`s are " +
		"two different argvs. PrepareGatesWrap's is what follows `--` INSIDE " +
		"the container (`posse gates wrap -- …`), resolved on the caller's PATH " +
		"before the shims go in front of it — which that function's own comment " +
		"states, and which is the ruling cagelauncher.go's two pcExemptFuncs " +
		"rows carry: this runs in the seat, not in a launcher. " +
		"CageInnerGatesReady's argv[0] is the host-side engine step. Same " +
		"spelling, different variable, different process.",
	"internal/posse/seatbelt.go:SeatbeltAvailable": "ranger-base-npqem's " +
		"ruling, written out at that function: SeatbeltAvailable is a HOST " +
		"question. The `sandbox-exec` it is about is the one startPlanned types " +
		"into a herdr pane, which runs in herdr's process tree carrying the " +
		"seat's PATH — so the gated lookup is the one that matches how that " +
		"binary will be resolved, and a gates-free probe here would answer a " +
		"question neither AvailableCages[CageSeatbelt] nor the launch gate " +
		"asked. The sandbox-exec posse execs in THIS process is " +
		"sandboxApplyProbe's, and since npqem that site resolves its own binary " +
		"(protocolBin at :166) rather than reading this answer as a licence.",
}

// pcProbe is one availability probe in the shipped tree: a call whose value is
// "is there a <name>".
type pcProbe struct {
	file     string
	line     int
	fn       string
	spelling string // "exec.LookPath" | "resolveOutside"
	arg      string // the argument as rendered
	name     string // the exec'd spelling it collided with; "" when in no class
}

// pcIsProbe reports whether call asks that question, and over which argument.
//
// TWO SPELLINGS, both live in this tree (the counts are on the test function
// below, MEASURED 2026-11-01):
//
//	exec.LookPath(X)    — searches THIS process's $PATH. The defect itself:
//	                      on a gated seat that PATH leads with the launching
//	                      persona's gates bin dir, so the probe answers true
//	                      on a shim the protocol resolution then refuses
//	                      (ranger-base-c33j6, -g1xnt, -d0nyb).
//	resolveOutside(X,…) — the RIGHT PATH, the WRONG spelling. Not a defect
//	                      today; a second copy of protocolAvailable's body,
//	                      which is what protocolAvailable was written to stop
//	                      being. ranger-base-d0nyb's four probes landed in
//	                      this shape — two sites, its three cage probes
//	                      sharing engineOnPath — in the hour before
//	                      ranger-base-g1xnt named the function, and a
//	                      LookPath-only census read them as clean while the
//	                      tree carried three spellings of one question.
//	                      ranger-base-372iq collapsed both onto
//	                      protocolAvailable so this arm is green over one
//	                      spelling rather than rostered around two.
//
// NOT absLookPath, which is the third thing in that neighbourhood and is not
// in the class: it takes a PATH and never a name, searches nothing, and is
// resolveOutside's own arm with no other caller in the tree (MEASURED
// 2026-11-01: two call sites, both inside resolveOutside). A probe cannot be
// written as absLookPath(X) for a bare name at all.
//
// protocolAvailable is of course not a probe to report — it is the answer.
func pcIsProbe(call *ast.CallExpr) (ast.Expr, string, bool) {
	if ewdIsSel(call.Fun, "exec", "LookPath") && len(call.Args) == 1 {
		return call.Args[0], "exec.LookPath", true
	}
	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "resolveOutside" && len(call.Args) == 2 {
		return call.Args[0], "resolveOutside", true
	}
	return nil, "", false
}

// pcProtocolNames are the expressions this file hands to posse's own
// resolution: the binary of a protocolCmd-wrapped exec, and every direct
// protocolBin argument.
//
// WRAPPED SITES ONLY, and that is the class and not a shortcut. An exec that
// does NOT go through protocolCmd is either arm 1's error or a pcExemptFuncs
// row — a line the SEAT reaches, whose binary is resolved on the seat's own
// gated PATH on purpose. A probe for such a binary reading that same gated
// PATH is right, so those sites must not pull their file's probes into the
// class (cagelauncher.go's two rows are exactly that case, and tree-wide they
// were two of the derivation's false positives).
func pcProtocolNames(f *ast.File) []ast.Expr {
	var out []ast.Expr
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || len(call.Args) == 0 {
			return true
		}
		switch id.Name {
		case "protocolBin":
			out = append(out, call.Args[0])
		case "protocolCmd":
			inner, ok := call.Args[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			switch {
			case ewdIsSel(inner.Fun, "exec", "Command") && len(inner.Args) > 0:
				out = append(out, inner.Args[0])
			case ewdIsSel(inner.Fun, "exec", "CommandContext") && len(inner.Args) > 1:
				out = append(out, inner.Args[1])
			}
		}
		return true
	})
	return out
}

// pcRangeStrings maps "<func>:<ident>" to the string literals ident takes when
// it is the value of a `for _, x := range []string{…}`.
//
// It is here because the match below is between RENDERED EXPRESSIONS, and one
// member of this class does not spell its name at the probe:
// internal/posse/backup.go asks `exec.LookPath(tool)` over
// `[]string{"git", "sqlite3"}` and execs `"sqlite3"` thirty lines down. An
// expression match alone reads `tool` against `"sqlite3"` and says clean,
// which is a pin missing a member it can see — so the match is between SETS of
// spellings, and a literal range is the one expansion this tree needs
// (MEASURED 2026-11-01: it adds exactly that one row and no other, on either
// side).
func pcRangeStrings(f *ast.File) map[string][]string {
	out := map[string][]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		rs, ok := n.(*ast.RangeStmt)
		if !ok || rs.Value == nil {
			return true
		}
		id, ok := rs.Value.(*ast.Ident)
		if !ok {
			return true
		}
		cl, ok := rs.X.(*ast.CompositeLit)
		if !ok {
			return true
		}
		arr, ok := cl.Type.(*ast.ArrayType)
		if !ok || arr.Len != nil {
			return true
		}
		if elt, ok := arr.Elt.(*ast.Ident); !ok || elt.Name != "string" {
			return true
		}
		key := pcFuncAt(f, rs.Pos()) + ":" + id.Name
		for _, el := range cl.Elts {
			if lit, ok := el.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				out[key] = append(out[key], lit.Value)
			}
		}
		return true
	})
	return out
}

// pcNameSet is every spelling e can be: itself as rendered, plus the literals
// of a string range it is the value of.
func pcNameSet(f *ast.File, ranges map[string][]string, e ast.Expr) []string {
	out := []string{types.ExprString(e)}
	if id, ok := e.(*ast.Ident); ok {
		out = append(out, ranges[pcFuncAt(f, e.Pos())+":"+id.Name]...)
	}
	sort.Strings(out)
	return out
}

// pcProbeScan reads one file for its probes, each marked with the exec'd
// spelling it collided with. PER FILE, which is the measured rule: a
// cross-file spelling match says nothing about the binary — `path` and `tool`
// are generic identifiers, and tree-wide the same derivation returned two rows
// that were only identifier collisions (cagelauncher.go:210 against
// runtimeprobe.go:442, gates.go:1159 against skillsdirtrust.go:168).
func pcProbeScan(fset *token.FileSet, rel string, f *ast.File) (probes []pcProbe, execd []string) {
	ranges := pcRangeStrings(f)
	seen := map[string]bool{}
	for _, e := range pcProtocolNames(f) {
		for _, s := range pcNameSet(f, ranges, e) {
			if !seen[s] {
				seen[s], execd = true, append(execd, s)
			}
		}
	}
	sort.Strings(execd)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		arg, spelling, ok := pcIsProbe(call)
		if !ok {
			return true
		}
		p := pcProbe{
			file: rel, line: fset.Position(call.Pos()).Line,
			fn: pcFuncAt(f, call.Pos()), spelling: spelling, arg: types.ExprString(arg),
		}
		for _, s := range pcNameSet(f, ranges, arg) {
			if seen[s] {
				p.name = s
				break
			}
		}
		probes = append(probes, p)
		return true
	})
	return probes, execd
}

// pcProtocolAvailableCalls counts the calls to the sanctioned spelling.
func pcProtocolAvailableCalls(files map[string]*ast.File) int {
	n := 0
	for _, f := range files {
		ast.Inspect(f, func(x ast.Node) bool {
			call, ok := x.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "protocolAvailable" {
				n++
			}
			return true
		})
	}
	return n
}

// TestQAEveryProbeForAProtocolChildsNameGoesThroughProtocolAvailable is arm 3
// of ranger-base-bm6j8's census — ranger-base-372iq, asked for by
// ranger-base-d0nyb rather than fixing three call sites by hand, "since the
// next one lands the same way".
//
// THE RULE. A probe for a name that one of posse's own children execs must ask
// protocolAvailable, which asks protocolBin's question over ADR 0077 D3's
// gates-free PATH. A probe that asks exec.LookPath instead reads THIS
// process's $PATH — on a gated seat, the launching persona's gates bin dir
// first — so it answers TRUE on a shim that the resolution one function away
// then refuses: a wrong yes followed by a right no, with the plan already made
// on the yes. ranger-base-c33j6 measured the gated PATH; ranger-base-g1xnt
// found the first probe (Bd.Available), ranger-base-d0nyb the next four
// (Herdr.Available and cage's three engine probes, which WrapInCage had
// already cleared and then refused).
//
// THE DERIVATION, measured rather than listed, and the reason this is an arm
// and not five fixes:
//
//	probe      = exec.LookPath(X) | resolveOutside(X, …)   (pcIsProbe)
//	exec site  = protocolCmd(exec.Command(Y, …))           (pcProtocolNames)
//	           | protocolCmd(exec.CommandContext(ctx, Y, …))
//	           | protocolBin(Y)
//	in class   = some exec site in the SAME FILE spells Y the way this probe
//	             spells X (pcNameSet, which is the rendered expression plus the
//	             literals of a string range it is the value of)
//
// MEASURED 2026-11-01 (darwin 25.6.0, a developer seat's ranger-base-372iq worktree,
// at main 2225b0b0 + this change): 21 probes in the shipped tree — 7
// exec.LookPath and 14 resolveOutside — against 36 exec'd spellings over 28
// files, and 2 of the 21 in class, both ruled below. A third was in class when
// this bead opened and is collapsed by it: internal/posse/backup.go's
// backup-tool gate, which is the whole reason pcNameSet expands a literal
// range (it spells `tool` and execs `"sqlite3"`).
//
// PER FILE is the rule and it was measured against the alternative: tree-wide,
// the same derivation adds two rows that are identifier collisions and nothing
// else (cagelauncher.go:210's `path` against runtimeprobe.go:442's,
// gates.go:1159's against skillsdirtrust.go:168's).
//
// AND IT FINDS THE CLASS IT WAS DERIVED FROM, which is the test of a
// derivation. MEASURED the same day over the tree at 2225b0b0^ — the four
// probes ranger-base-d0nyb had not yet fixed, restored into this worktree and
// read by this very scan: cage.go:266 (ContainerAvailable), :1277
// (cageNoBinary), :1430 (BuildCageImage) and herdr.go:59 (Herdr.Available),
// each reported against the protocolBin site in its own file, and nothing
// else. (ranger-base-g1xnt's internal/posse/beads.go:51 is the fifth and was
// not re-measured here: that revision of beads.go does not compile against
// today's internal/posse, so the test binary cannot be built over it. The
// derivation that found it is on ranger-base-372iq, measured by
// ranger-base-d0nyb's seat.)
//
// WHAT THIS ARM IS NOT. It is syntactic, like arms 1 and 2: a protocolAvailable
// that stopped reading the gates-free PATH would leave it green.
// internal/posse/probegatesfreepath_test.go is the behavioural proof, both
// directions, on a fixture PATH whose only answer for the name is a gates bin
// dir — with its own positive controls, because "unavailable" is what a probe
// reading a PATH it can resolve nothing on reports.
func TestQAEveryProbeForAProtocolChildsNameGoesThroughProtocolAvailable(t *testing.T) {
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
	if len(files) < 100 {
		t.Fatalf("the walk read %d non-test .go files under %s — the corpus this pin censuses is not there",
			len(files), ewdCorpusRoot)
	}

	var probes []pcProbe
	execd := 0
	for _, rel := range rels {
		p, e := pcProbeScan(fset, rel, files[rel])
		probes = append(probes, p...)
		execd += len(e)
	}

	// The positive witnesses. This arm is an assertion of absence like every
	// other one here, and a scan that resolves neither side reports exactly
	// what a clean tree reports. Floors under the live counts (MEASURED
	// 2026-11-01, at main 2225b0b0 + this change): 21 probes, 36 exec'd
	// spellings over 28 files, 4 calls to protocolAvailable.
	if len(probes) < 12 {
		t.Errorf("the scan found %d probe(s) in the shipped tree and 21 sit there today — 7 exec.LookPath (cageinner.go, cagelauncher.go, gates.go x2, possebinary.go, runtimepreflight.go, seatbelt.go) and 14 resolveOutside. A count this low means pcIsProbe has stopped resolving one of its two spellings, and this arm is then green over whatever the tree does with it.",
			len(probes))
	}
	if execd < 20 {
		t.Errorf("the scan read %d exec'd spelling(s) out of protocolCmd/protocolBin sites and 36 sit in the tree today, over 28 files — pcProtocolNames has stopped resolving the wrapper, so no probe can collide with anything and this arm is measuring nothing.",
			execd)
	}
	if n := pcProtocolAvailableCalls(files); n < 2 {
		t.Errorf("the tree holds %d call(s) to protocolAvailable and four sit there today (beads.go Bd.Available, herdr.go Herdr.Available, cage.go engineOnPath, backup.go's backup-tool gate). The sanctioned spelling having emptied out is how this class comes back: a tree with no protocolAvailable in it and no probe in class is also a tree where every probe was rewritten as something this scan cannot see.",
			n)
	}

	// THE ARM.
	seen := map[string]bool{}
	inClass := 0
	for _, p := range probes {
		if p.name == "" {
			continue
		}
		inClass++
		key := p.file + ":" + p.fn
		if _, ok := pcProbeExemptFuncs[key]; ok {
			seen[key] = true
			continue
		}
		t.Errorf("%s:%d (%s) probes %s with %s(%s), and a protocolCmd/protocolBin site in this same file execs %s.\n"+
			"  A probe for a name posse's own children exec has to ask the question the resolution asks. exec.LookPath searches THIS process's $PATH, which on a gated seat leads with the launching persona's gates bin dir — so it answers true on that persona's shim while protocolBin, one function away, refuses the same name (MEASURED 2026-10-31, ranger-base-c33j6; ranger-base-g1xnt and -d0nyb are the five probes that did it). A bare resolveOutside reads the right PATH and is a second copy of protocolAvailable's body, which is the copy that goes stale.\n"+
			"  Spell it `protocolAvailable(%s)` (internal/posse/protocolchild.go) — never wider than protocolBin, and narrower only on an explicit path that is not there, which is a probe saying no rather than a gate opening onto a refusal. If the two names are two different binaries, or two different PATHs because the process that reaches this line is not the one that reaches the exec, or this call is a resolution whose ANSWER is the path (a shim's real target) rather than a yes/no, add %q to pcProbeExemptFuncs with that reason — the way cageinner.go's and seatbelt.go's rows are.",
			p.file, p.line, p.fn, p.name, p.spelling, p.arg, p.name, p.arg, key)
	}
	if inClass == 0 {
		t.Error("not one probe in the tree was read as naming a binary one of posse's own children execs, and two did on 2026-11-01 — cageinner.go:PrepareGatesWrap and seatbelt.go:SeatbeltAvailable, the two rows pcProbeExemptFuncs rules — with a third, backup.go's backup-tool gate, collapsed onto protocolAvailable by ranger-base-372iq.\n" +
			"  Both halves have to resolve for this arm to mean anything, and a scan that resolves the probes but matches none of them reports exactly what a clean tree reports. Either the match broke or the tree really did lose the class; check pcNameSet against a file you know holds one.")
	}

	// And the ruling's rows have to be real.
	var keys []string
	for k := range pcProbeExemptFuncs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !seen[k] {
			t.Errorf("pcProbeExemptFuncs rules %s exempt (%s) and no probe there names a binary one of posse's own children execs any more — drop the row",
				k, pcProbeExemptFuncs[k])
		}
	}
}

// TestQATheProbeCensusCanStillSayNo is arm 3's two-way half: one planted shape
// per route a probe can be in class, out of class, or satisfied — run through
// the same reader, because the arm above is an assertion of absence and an
// absence is what a reader that resolves nothing reports.
func TestQATheProbeCensusCanStillSayNo(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		src      string
		inClass  bool
		noProbes bool // the shape holds no probe at all, which is its point
	}{
		{"LookPath for a name this file's protocolCmd execs", `package x
import "os/exec"
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func avail() bool { _, err := exec.LookPath("bd"); return err == nil }
func run() error { return protocolCmd(exec.Command("bd", "ready")).Run() }
`, true, false},
		{"the context form's binary is its SECOND argument", `package x
import ("context"; "os/exec")
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func avail() bool { _, err := exec.LookPath("bd"); return err == nil }
func run(ctx context.Context) error { return protocolCmd(exec.CommandContext(ctx, "bd", "ready")).Run() }
`, true, false},
		{"a direct protocolBin is an exec site too", `package x
import "os/exec"
func protocolBin(n string) (string, error) { return n, nil }
func avail() bool { _, err := exec.LookPath("bd"); return err == nil }
func run() (string, error) { return protocolBin("bd") }
`, true, false},
		{"resolveOutside is the right PATH and still the wrong spelling", `package x
import "os/exec"
func resolveOutside(c, b string) string { return c }
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func avail() bool { return resolveOutside("bd", "") != "" }
func run() error { return protocolCmd(exec.Command("bd", "ready")).Run() }
`, true, false},
		{"protocolAvailable is the answer, not a probe", `package x
import "os/exec"
func protocolAvailable(n string) bool { return n != "" }
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func avail() bool { return protocolAvailable("bd") }
func run() error { return protocolCmd(exec.Command("bd", "ready")).Run() }
`, false, true},
		{"a literal range reaches the name the probe does not spell", `package x
import "os/exec"
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func avail() bool {
	for _, tool := range []string{"git", "sqlite3"} {
		if _, err := exec.LookPath(tool); err != nil { return false }
	}
	return true
}
func run() error { return protocolCmd(exec.Command("sqlite3", "x")).Run() }
`, true, false},
		{"a different name in the same file is not this file's class", `package x
import "os/exec"
func protocolCmd(c *exec.Cmd) *exec.Cmd { return c }
func avail() bool { _, err := exec.LookPath("true"); return err == nil }
func run() error { return protocolCmd(exec.Command("git", "status")).Run() }
`, false, false},
		{"an UNWRAPPED exec is the seat's own, and pulls nothing into the class", `package x
import "os/exec"
func avail() bool { _, err := exec.LookPath("docker"); return err == nil }
func run() error { return exec.Command("docker", "ps").Run() }
`, false, false},
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
			probes, _ := pcProbeScan(fset, "x.go", f)
			if (len(probes) == 0) != tc.noProbes {
				t.Fatalf("the scan found %d probe(s) (want none = %v) in:\n%s", len(probes), tc.noProbes, tc.src)
			}
			got := false
			for _, p := range probes {
				if p.name != "" {
					got = true
				}
			}
			if got != tc.inClass {
				t.Errorf("in class = %v (want %v) over %d probe(s); the census reads this shape wrong, so arm 3 is measuring nothing",
					got, tc.inClass, len(probes))
			}
		})
	}
}
