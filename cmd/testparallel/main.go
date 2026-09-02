// Command testparallel answers one question about a package's tests: which
// of them can take t.Parallel, and which must not. It is the tool ADR 0047
// D3 says to run before adding t.Parallel to a newly env-clean set, and the
// re-derivation of the two filters docs/notes.d/ranger-base-i7fa.md §2 and
// §4 describe but never committed.
//
//	go run ./cmd/testparallel ./internal/posse            # the counts
//	go run ./cmd/testparallel ./internal/posse eligible   # the set to mark
//	go run ./cmd/testparallel ./internal/posse d3         # the list to READ
//
// Three filters, each catching what the last one does not:
//
//  1. env taint. t.Parallel panics in a test that has called t.Setenv, so a
//     call-graph taint over the TEST files (t.Setenv/os.Setenv/Chdir at the
//     root, closure bodies included) says which tests never reach the
//     process environment. Test files only: no non-test file in the subject
//     package writes the environment, and propagating through product code
//     would taint everything.
//  2. package-level state. Env-clean is not parallel-safe: a test asserting
//     a once-per-process notice is a complete answer while tests are serial
//     and no answer at all when they are not (i7fa §4). Every package-level
//     var, narrowed to those written anywhere, then every test naming one.
//  3. the shared filesystem. Neither of the first two catches two tests in
//     one directory. The roots are the $HOME readers outside the App, and
//     the output is a list to READ rather than a verdict: a test asserting
//     a per-test key is safe, one asserting a file whole or absent is not.
//
// Over-approximating by design: the call graph is keyed on the callee's
// identifier alone, receivers ignored. Over-taint costs a test its
// t.Parallel; under-taint costs a panic.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type fn struct {
	name    string
	file    string
	isTest  bool // declared in a _test.go file
	topTest bool // func TestXxx(t *testing.T)
	calls   map[string]bool
	idents  map[string]bool
	envRoot bool
	homeRd  bool
	// claudeCfgLit: the function names the shared claude config file by
	// literal rather than through the product helper.
	claudeCfgLit bool
}

func main() {
	dir := os.Args[1]
	fset := token.NewFileSet()
	ents, _ := os.ReadDir(dir)
	funcs := map[string][]*fn{}
	pkgVars := map[string]bool{}
	writtenVars := map[string]bool{}
	files := map[string]*ast.File{}
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			panic(err)
		}
		files[e.Name()] = f
	}
	// package-level vars
	for _, f := range files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				for _, n := range vs.Names {
					if n.Name != "_" {
						pkgVars[n.Name] = true
					}
				}
			}
		}
	}
	// which package-level vars are written anywhere
	// i7fa's set, kept as it was rather than narrowed. It over-reads: a
	// package-level time.Time fixture given .Add counts as written, which
	// is why backupAt and blindT are on the list and 56 tests that could
	// take t.Parallel do not. Narrowing it to Store/Delete/Do alone frees
	// those 56 — measured — but this bead is not the place to relax a
	// filter whose whole job is to be conservative, and 56 of 1975 is
	// noise against the 700 it is here to free. Filed, not taken.
	writeMeth := map[string]bool{"Store": true, "Delete": true, "Do": true, "LoadOrStore": true,
		"Add": true, "Set": true, "Put": true, "Reset": true, "Push": true, "Clear": true}
	for name, f := range files {
		_ = name
		ast.Inspect(f, func(n ast.Node) bool {
			mark := func(e ast.Expr) {
				for {
					switch x := e.(type) {
					case *ast.Ident:
						if pkgVars[x.Name] {
							writtenVars[x.Name] = true
						}
						return
					case *ast.IndexExpr:
						e = x.X
					case *ast.SelectorExpr:
						e = x.X
					case *ast.StarExpr:
						e = x.X
					default:
						return
					}
				}
			}
			switch x := n.(type) {
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					mark(l)
				}
			case *ast.IncDecStmt:
				mark(x.X)
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					mark(x.X)
				}
			case *ast.CallExpr:
				if se, ok := x.Fun.(*ast.SelectorExpr); ok && writeMeth[se.Sel.Name] {
					mark(se.X)
				}
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "append" && len(x.Args) > 0 {
					mark(x.Args[0])
				}
			}
			return true
		})
	}

	envSetter := map[string]bool{"Setenv": true, "Unsetenv": true, "Chdir": true, "Clearenv": true}
	for name, f := range files {
		isTest := strings.HasSuffix(name, "_test.go")
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			g := &fn{name: fd.Name.Name, file: name, isTest: isTest,
				calls: map[string]bool{}, idents: map[string]bool{}}
			if isTest && strings.HasPrefix(fd.Name.Name, "Test") && fd.Recv == nil &&
				len(fd.Type.Params.List) == 1 && isT(fd.Type.Params.List[0].Type) {
				g.topTest = true
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					switch c := x.Fun.(type) {
					case *ast.Ident:
						g.calls[c.Name] = true
					case *ast.SelectorExpr:
						g.calls[c.Sel.Name] = true
						// Receiver-blind on purpose: t, tb, a *testing.TB
						// parameter under any name, or os itself — over-taint
						// costs a test its t.Parallel, under-taint costs a panic.
						if envSetter[c.Sel.Name] {
							g.envRoot = true
						}
						if c.Sel.Name == "UserHomeDir" {
							g.homeRd = true
						}
						if c.Sel.Name == "Getenv" && len(x.Args) == 1 {
							if bl, ok := x.Args[0].(*ast.BasicLit); ok && bl.Value == `"HOME"` {
								g.homeRd = true
							}
						}
					}
				case *ast.Ident:
					g.idents[x.Name] = true
				case *ast.BasicLit:
					if strings.Contains(x.Value, ".claude.json") || strings.Contains(x.Value, ".config.json") {
						g.claudeCfgLit = true
					}
				}
				return true
			})
			funcs[fd.Name.Name] = append(funcs[fd.Name.Name], g)
		}
	}

	// ── filter 1: env taint over the test-file call graph ────────────────
	inTests := func(g *fn) bool { return g.isTest }
	envTainted := propagateIn(funcs, func(g *fn) bool { return g.envRoot }, inTests)
	// The one exemption, and it is the harness's own: fakeDirs is written
	// only by setFakeDir, only ever as Store(t.Name(), …)/Delete(t.Name()),
	// and read only as Load(t.Name()) — a key space partitioned by the
	// test's own name over a sync.Map, so no two tests touch one entry.
	// That is the same argument ADR 0047 D3 makes for SeedClaudeTrust's
	// per-repo key. Left in, it taints 663 of the tests this bead exists to
	// free, so it is named here rather than waived silently.
	// operatorHome is the second and last: written once in TestMain, before
	// m.Run, and read-only for the whole of the run.
	exempt := map[string]bool{"fakeDirs": true, "operatorHome": true}
	// Named serial, for reasons no static filter can see. Kept here rather
	// than as a comment on each test so that `eligible` is the whole answer
	// and a later pass cannot re-add t.Parallel by running this tool.
	serial := map[string]string{
		// The child halves of the cross-process tests: re-exec'd with
		// -test.run=<name> and read for one line on stdout against a
		// deadline. Pausing them into the parallel phase moves that line.
		"TestQASeedTrustChildSeeder":    "child of a cross-process test",
		"TestQASeedTrustChildHoldLock":  "child of a cross-process test",
		"TestQASeedCageHomeChildSeeder": "child of a cross-process test",
		"TestLaunchLockChildHolder":     "child of a cross-process test",
		"TestWatchLockHolderChild":      "child of a cross-process test",
		// Tests that assert on flock ACQUISITION. Two of them side by side
		// read a released lock as still held, on lock files that are per
		// test — 3-6 failures in 60 at -parallel 8 over launchlock_test.go
		// alone (ranger-base-9l77f, filed off ranger-base-aupee). Until
		// that is understood, anything asserting who holds a lock is
		// serial; the hundreds of tests that merely TAKE the launcher lock
		// on their way through a pass are not affected and stay parallel.
		"TestLaunchLockSecondLauncherWaits":                    "asserts flock acquisition",
		"TestLaunchLockFreeAfterRelease":                       "asserts flock acquisition",
		"TestLaunchLockHolderNamesOurOwnProcess":               "asserts flock acquisition",
		"TestLaunchLockHoldsAcrossProcesses":                   "asserts flock acquisition",
		"TestDispatchFireLoopWaitsForTheLock":                  "asserts flock acquisition",
		"TestDispatchGatherRunsUnlocked":                       "asserts flock acquisition",
		"TestDispatchDryRunDoesNotTakeTheLock":                 "asserts flock acquisition",
		"TestLaunchBeadWaitsForTheLock":                        "asserts flock acquisition",
		"TestLaunchBeadReportsTheLockWaitToProgress":           "asserts flock acquisition",
		"TestTwoPassesDoNotInterleaveLaunches":                 "asserts flock acquisition",
		"TestWatchLoopRunningTracksTheLock":                    "asserts flock acquisition",
		"TestLockWatchRefusesASecondHolder":                    "asserts flock acquisition",
		"TestWatchLockDiesWithItsProcess":                      "asserts flock acquisition",
		"TestWatchStatusReadsLockThenPidfile":                  "asserts flock acquisition",
		"TestWatchHoldsTheLockForItsWholeLife":                 "asserts flock acquisition",
		"TestWatchRefusesWhenAnotherLoopHoldsTheLock":          "asserts flock acquisition",
		"TestWatchStatusNeverTurnsAnUnaskableQuestionIntoNone": "asserts flock acquisition",
		"TestWatchReleasesLockBetweenPasses":                   "asserts flock acquisition",
	}
	// ── filter 2: written package-level vars named anywhere reachable ────
	varTainted := propagateIn(funcs, func(g *fn) bool {
		for id := range g.idents {
			if writtenVars[id] && !exempt[id] {
				return true
			}
		}
		return false
	}, inTests)
	// ── filter 3: $HOME readers outside the App, reached from a test ─────
	homeRoots := map[string]bool{}
	for n, gs := range funcs {
		for _, g := range gs {
			if !g.isTest && g.homeRd {
				homeRoots[n] = true
			}
		}
	}
	homeTainted := propagate(funcs, func(g *fn) bool { return !g.isTest && g.homeRd })
	// D3 (ADR 0047): the shared $HOME is a shared FILE only where something
	// writes under it, and the one writer is SeedClaudeTrust ->
	// ClaudeConfigFile() under a flock, keyed on the per-test repo path. So
	// the tests to read are the ones whose own test-file code reaches that
	// file — a per-key assertion is safe, the file whole or absent is not.
	trustNames := map[string]bool{
		"ClaudeConfigFile": true, "SeedClaudeTrust": true, "lockClaudeConfig": true,
		"claudeTrustProbe": true, "claudeOutsideReadProbe": true,
		"ClaudeTrusted": true, "claudeOutsideReadSeen": true, "SeedClaudeOutsideRead": true,
	}
	d3 := propagateIn(funcs, func(g *fn) bool {
		for id := range g.idents {
			if trustNames[id] {
				return true
			}
		}
		for c := range g.calls {
			if trustNames[c] {
				return true
			}
		}
		return g.claudeCfgLit
	}, inTests)
	// D3, second half and the one the claude-config pass missed: a TEST
	// that reaches $HOME from its own code. The shared home is only shared
	// where something writes into it, and the writers the product owns are
	// keyed (SeedClaudeTrust per repo path) — but a fixture that plants
	// files under $HOME/.grok, $HOME/.codex or $HOME/.claude is writing to
	// one directory every other test also reads. grokPoolPassFull is the
	// measured instance: os.UserHomeDir(), then a session transcript under
	// it, then a guard that sums EVERY session there.
	// hermetic is not a root: it reads $HOME only to Join the per-test
	// worktrees root under it (ADR 0047 D2), and every backend test calls it.
	homeInTest := propagateIn(funcs, func(g *fn) bool { return g.homeRd && g.name != "hermetic" }, inTests)
	// the one WRITER under $HOME plus its lock
	writeRoots := map[string]bool{"SeedClaudeTrust": true, "lockClaudeConfig": true}
	writeTainted := propagate(funcs, func(g *fn) bool { return writeRoots[g.name] && !g.isTest })

	var tests []*fn
	for _, gs := range funcs {
		for _, g := range gs {
			if g.topTest {
				tests = append(tests, g)
			}
		}
	}
	sort.Slice(tests, func(i, j int) bool { return tests[i].name < tests[j].name })

	mode := "summary"
	if len(os.Args) > 2 {
		mode = os.Args[2]
	}
	switch mode {
	case "summary":
		fmt.Printf("package vars: %d, written: %d\n", len(pkgVars), len(writtenVars))
		fmt.Printf("$HOME readers outside a test file: %d\n", len(homeRoots))
		nEnv, nVar, nClean, nHome, nWrite, nD3, nHT, nNamed := 0, 0, 0, 0, 0, 0, 0, 0
		for _, g := range tests {
			if envTainted[g.name] {
				nEnv++
				continue
			}
			nClean++
			if serial[g.name] != "" {
				nNamed++
			}
			if varTainted[g.name] {
				nVar++
			}
			if homeTainted[g.name] {
				nHome++
			}
			if writeTainted[g.name] {
				nWrite++
			}
			if !varTainted[g.name] && d3[g.name] {
				nD3++
			}
			if !varTainted[g.name] && homeInTest[g.name] {
				nHT++
			}
		}
		fmt.Printf("top-level tests: %d\n", len(tests))
		fmt.Printf("  env-tainted (must stay serial):        %d\n", nEnv)
		fmt.Printf("  env-clean:                             %d\n", nClean)
		fmt.Printf("    of those, name a written pkg var:    %d\n", nVar)
		fmt.Printf("    of those, reach a $HOME reader:      %d\n", nHome)
		fmt.Printf("    of those, reach SeedClaudeTrust/lock:%d\n", nWrite)
		fmt.Printf("  D3 to read by hand (eligible AND touch the claude config in test code): %d\n", nD3)
		fmt.Printf("  D3b eligible AND read $HOME from test code: %d\n", nHT)
		fmt.Printf("    named serial by hand (see the map): %d\n", nNamed)
	case "eligible": // env-clean AND no written pkg var AND not named serial
		for _, g := range tests {
			if !envTainted[g.name] && !varTainted[g.name] && serial[g.name] == "" {
				fmt.Printf("%s\t%s\n", g.file, g.name)
			}
		}
	case "named-serial":
		var ks []string
		for k, why := range serial {
			ks = append(ks, k+"\t"+why)
		}
		sort.Strings(ks)
		for _, k := range ks {
			fmt.Println(k)
		}
	case "varserial":
		for _, g := range tests {
			if !envTainted[g.name] && varTainted[g.name] {
				fmt.Printf("%s\t%s\n", g.file, g.name)
			}
		}
	case "d3":
		for _, g := range tests {
			if !envTainted[g.name] && !varTainted[g.name] && d3[g.name] {
				fmt.Printf("%s\t%s\n", g.file, g.name)
			}
		}
	case "hometestroots":
		var ks []string
		for n, gs := range funcs {
			for _, g := range gs {
				if g.isTest && g.homeRd {
					ks = append(ks, g.file+"\t"+n)
				}
			}
		}
		sort.Strings(ks)
		for _, k := range ks {
			fmt.Println(k)
		}
	case "hometest":
		for _, g := range tests {
			if !envTainted[g.name] && !varTainted[g.name] && homeInTest[g.name] {
				fmt.Printf("%s\t%s\n", g.file, g.name)
			}
		}
	case "homereach":
		for _, g := range tests {
			if !envTainted[g.name] && !varTainted[g.name] && writeTainted[g.name] {
				fmt.Printf("%s\t%s\n", g.file, g.name)
			}
		}
	case "homeroots":
		var ks []string
		for k := range homeRoots {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			for _, g := range funcs[k] {
				if !g.isTest && g.homeRd {
					fmt.Printf("%s\t%s\n", g.file, k)
				}
			}
		}
	case "varimpact":
		for v := range writtenVars {
			one := map[string]bool{v: true}
			tt := propagateIn(funcs, func(g *fn) bool {
				for id := range g.idents {
					if one[id] {
						return true
					}
				}
				return false
			}, inTests)
			n := 0
			for _, g := range tests {
				if !envTainted[g.name] && tt[g.name] {
					n++
				}
			}
			if n > 0 {
				fmt.Printf("%5d  %s\n", n, v)
			}
		}
	case "writtenvars":
		var ks []string
		for k := range writtenVars {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		fmt.Println(strings.Join(ks, "\n"))
	}
}

func isT(e ast.Expr) bool {
	se, ok := e.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := se.X.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "T"
}

// propagate marks every function that transitively calls a root.
func propagate(funcs map[string][]*fn, root func(*fn) bool) map[string]bool {
	return propagateIn(funcs, root, func(*fn) bool { return true })
}

// propagateIn is propagate restricted to the functions consider admits: the
// env and package-var filters run over the TEST files alone, because no
// non-test file in this package writes the environment (verified) and every
// product function names its own package vars, which would taint the world.
func propagateIn(funcs map[string][]*fn, root func(*fn) bool, consider func(*fn) bool) map[string]bool {
	tainted := map[string]bool{}
	for n, gs := range funcs {
		for _, g := range gs {
			if consider(g) && root(g) {
				tainted[n] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for n, gs := range funcs {
			if tainted[n] {
				continue
			}
			for _, g := range gs {
				if !consider(g) {
					continue
				}
				for c := range g.calls {
					if tainted[c] {
						tainted[n] = true
						changed = true
						break
					}
				}
				if tainted[n] {
					break
				}
			}
		}
	}
	return tainted
}
