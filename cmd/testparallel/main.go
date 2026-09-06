// Command testparallel answers one question about a package's tests: which
// of them can take t.Parallel, and which must not. It is the tool ADR 0047
// D3 says to run before adding t.Parallel to a newly env-clean set, and the
// re-derivation of the two filters docs/notes.d/ranger-base-i7fa.md §2 and
// §4 describe but never committed.
//
//	go run ./cmd/testparallel ./internal/posse            # the counts
//	go run ./cmd/testparallel ./internal/posse eligible   # the set to mark
//	go run ./cmd/testparallel ./internal/posse d3         # the list to READ
//	go run ./cmd/testparallel ./internal/posse extra      # parallel-but-ineligible
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
	"unicode"
)

type fn struct {
	name    string
	file    string
	isTest  bool // declared in a _test.go file
	topTest bool // func TestXxx(t *testing.T)
	calls   map[string]bool
	idents  map[string]bool
	envRoot bool
	// bareFakeDir: calls fakeDir() with no argument — the binary-wide fake
	// dir, not the per-test fakeDirOf(t).
	bareFakeDir bool
	// hasParallel: the body's first statement is t.Parallel().
	hasParallel bool
	// envVars: the variables this function sets by literal name, for the
	// `envroots` census. A setter whose name is computed shows as "?".
	envVars map[string]bool
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
				calls: map[string]bool{}, idents: map[string]bool{}, envVars: map[string]bool{}}
			if len(fd.Body.List) > 0 {
				if es, ok := fd.Body.List[0].(*ast.ExprStmt); ok {
					if ce, ok := es.X.(*ast.CallExpr); ok {
						if se, ok := ce.Fun.(*ast.SelectorExpr); ok && se.Sel.Name == "Parallel" {
							g.hasParallel = true
						}
					}
				}
			}
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
						if c.Name == "fakeDir" && len(x.Args) == 0 {
							g.bareFakeDir = true
						}
					case *ast.SelectorExpr:
						g.calls[c.Sel.Name] = true
						// Receiver-blind on purpose: t, tb, a *testing.TB
						// parameter under any name, or os itself — over-taint
						// costs a test its t.Parallel, under-taint costs a panic.
						if envSetter[c.Sel.Name] {
							g.envRoot = true
							name := "?" + c.Sel.Name
							if len(x.Args) > 0 {
								if bl, ok := x.Args[0].(*ast.BasicLit); ok {
									name = strings.Trim(bl.Value, `"`)
								}
							}
							g.envVars[name] = true
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
	// only by setFakeDir, only ever as Store(t, …)/Delete(t), and read only
	// as Load(t) — a key space partitioned by the *testing.T POINTER over a
	// sync.Map, so no two tests touch one entry. That is the same argument
	// ADR 0047 D3 makes for SeedClaudeTrust's per-repo key.
	// The key is the T and NOT t.Name(): `go test -count=N` gives N live
	// copies of a test one name, and t.Parallel resumes them together — one
	// key, N tests, each Cleanup deleting the others' entry. Measured as
	// five FAILs at -count=3 that were green at -count=1 (ranger-base-pj87l,
	// the same lesson that moved the key); a T pointer is unique per run by
	// construction. internal/posse/herdr_test.go:1461 carries that argument
	// at the declaration.
	// Left in, it taints 663 of the tests this bead exists to free, so it is
	// named here rather than waived silently.
	// operatorHome is the second: written once in TestMain, before m.Run,
	// and read-only for the whole of the run.
	// hermeticRun is the third: an atomic.Int64 that only ever takes Add(1),
	// read once per call as the caller's own number. Concurrency is the
	// type's whole job, and every test calls hermetic — left in, it taints
	// 722 of the 1606 this bead just freed, which is the exact shape the
	// fakeDirs note above warns about (ranger-base-pj87l).
	exempt := map[string]bool{"fakeDirs": true, "operatorHome": true, "hermeticRun": true}
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
		"TestWatchStatusNamesTheLogAndItsAge":                  "asserts flock acquisition",
		"TestWatchReleasesLockBetweenPasses":                   "asserts flock acquisition",
		// The two that ranger-base-zppcv moved into launchlock_test.go. The
		// first carried t.Parallel in worktree_test.go and is the only
		// acquisition test the rule above had missed: it releases a lock and
		// asserts the release read as free, which is the 9l77f shape exactly,
		// and its free-lock arm is the line that failed a pass unreproducibly.
		// Both are INERT today and named anyway — they go through wtApp, so
		// filter 3 calls them ineligible and `check` is quiet with or without
		// these two lines (measured both ways). The judgment is what is being
		// recorded: the day wtApp stops reading $HOME, the filter lets go and
		// this is the only thing left holding them serial.
		"TestTryLockLaunchesDoesNotWait":       "asserts flock acquisition",
		"TestTryLockLaunchesNamesWhichFailure": "asserts flock acquisition",
		// The opt-in live probe: it creates a real herdr workspace, types a
		// launch line into its pane and spends a model turn in it. One pane
		// and one CLI at a time is the whole shape of the measurement, and
		// it was only ever "eligible" here because ranger-base-385x took
		// away the t.Setenv that used to hold it out (the shim it installs
		// now reaches the pane by absolute path, not through PATH).
		"TestLiveRuntimeProbe": "drives a real herdr pane and one model turn",
		// Drives a REAL backupLoop goroutine and times it: its treatment arm
		// asserts the second archive lands in under the 60s interval, and its
		// absence arm waits 3x that measured time. Both readings are wall
		// clock on a loop with its own ticker, so a parallel phase that
		// stretches the run stretches what it measures (ranger-base-wj7e9;
		// it takes ~11s serial). The pure-arithmetic half of the same fix,
		// TestQABackupLevelIsSampledFasterThanItFires, is parallel.
		"TestQABackupLoopSamplesTheLevelBetweenIntervals": "times a real loop against its own interval",
		// Env-tainted through a FUNC VALUE, which the call graph cannot
		// see: both reach unreadableKeychain (and its t.Setenv) as
		// `tc.err(t)` / `mk(t)` out of a table, and an identifier that is
		// never the callee of a CallExpr taints nothing. The taint is real —
		// since the darwin adapter became a composite (ADR 0019 D2 as
		// amended, ranger-base-5jdzh) a `security` exiting 44 consults the
		// credentials file, so that helper has to name a config directory
		// with no file in it or the row reads whatever the box has. Adding
		// t.Parallel here panics; this map is where the tool is told.
		"TestPlanReadHasFourCredentialFailureClasses": "env-tainted through a table's func value (unreadableKeychain)",
		"TestTheFourCredentialClassesAreDistinct":     "env-tainted through a table's func value (unreadableKeychain)",
		// The ForkLock pins (execwrite.go, ranger-base-d26ak). Same shape as
		// the flock rows above, one level down: they READ whether
		// syscall.ForkLock is held for writing, and that lock is the one
		// every fork in this binary takes. A parallel sibling's subprocess
		// answers that question for them — the rig's own "free before the
		// window" control would read somebody else's fork as our lock — and
		// two of them HOLD it, for 250ms and for as long as a FIFO stays
		// unopened, which every other test's fork would then queue behind.
		// Serial, they have the process to themselves and cost ~1.5s.
		"TestUnderForkLockHoldsTheLockForTheWriteAndNoLonger":      "reads and holds the process-wide syscall.ForkLock",
		"TestUnderForkLockKeepsAConcurrentForkOutOfTheWriteWindow": "reads and holds the process-wide syscall.ForkLock",
		"TestWriteExecutableWritesAFileThatRuns":                   "reads and holds the process-wide syscall.ForkLock",
		"TestWriteExecutableWritesUnderTheForkLock":                "reads and holds the process-wide syscall.ForkLock",
	}
	// Named parallel, and the counterpart of serial above: a test the three
	// filters call ineligible, that a human has READ and cleared. These are
	// the 45 ranger-base-btdvw went through one at a time; six of that 45 were
	// real and lost their t.Parallel in the same pass (costProviders,
	// AvailableCages x2, planAdapters, RHQ_PLAN_USAGE_URL x2). The rest are
	// here, per test and never per file, because a clearance is an argument
	// about ONE test's body and does not survive being generalised:
	//
	//   - "reads <clock>": blindT, lpouiT, pulseNow and catalogAt are package-level
	//     time.Time fixtures declared once and never assigned. The only
	//     "write" the filter sees is `.Add`, which on a time.Time returns a
	//     new value and mutates nothing — the over-read the writeMeth comment
	//     above describes. NOT fixed by exempting the three vars: MEASURED,
	//     that frees 64 further tests, so `check` would then demand
	//     t.Parallel on 64 tests nobody has read. That is a sweep, and this
	//     is not it.
	//   - "reads OpsPatterns": the shipped table is written once, by init(),
	//     compiling each ERE — before any test runs. Every test here ranges
	//     it read-only.
	//   - "calls sandboxApplyRefusal": the seam is a func var, and its one
	//     writer (rchFakeApplyRefusal in reachability_qa_test.go) is serial.
	//     Go resumes paused parallel tests only after the whole sequential
	//     pass is done, so a serial writer never overlaps a parallel reader.
	//
	// A clearance is not permanent: it says what the test's body did when it
	// was read. Add a write to one of these and the reason above stops being
	// true — which is why each line names the fact it rests on, and why the
	// gate READS that fact rather than the test's name: a clearance covers
	// exactly the written vars its reason names (clearanceCovers). A test
	// that reaches a var nobody cleared is flagged again, with the same
	// message as an uncleared one, however old its line here is. Before
	// ranger-base-acvq3 the key was the name alone, so one clearance waived
	// every future reason — measured: a costProviders write in a "reads
	// blindT" test passed `check` clean.
	parallelOK := map[string]string{
		// blindT (23)
		"TestEpochStartIsWallClockAligned":             "reads blindT",
		"TestHarnessRatios":                            "reads blindT",
		"TestPlanCache429WithoutRetryAfter":            "reads blindT",
		"TestPlanCacheCapsALongRetryAfter":             "reads blindT",
		"TestPlanCacheCorruptSnapshotRefetches":        "reads blindT",
		"TestPlanCacheHonoursRetryAfterAcrossCallers":  "reads blindT",
		"TestPlanCacheLineRendersNothingOnAFailedRead": "reads blindT",
		"TestPlanCacheLineSaysHowOldTheReadingIs":      "reads blindT",
		"TestPlanCacheLogsAFailedRead":                 "reads blindT",
		"TestPlanCacheRefetchesPastTheAgeAsked":        "reads blindT",
		"TestPlanCacheSuccessClearsTheCooldown":        "reads blindT",
		"TestPlanCacheWithoutAStateDirStillReads":      "reads blindT",
		"TestPlanCacheZeroAgeAlwaysAsks":               "reads blindT",
		"TestPlanSnapshotFromBeforeTheSeamIsAMiss":     "reads blindT",
		"TestProbeStateDriftAndCurrency":               "reads blindT",
		"TestQAPlan429BackoffAsksTheBoundaryOnce":      "reads blindT",
		"TestQAPlan429BackoffEscalatesAcrossAStorm":    "reads blindT",
		"TestQAPlan429BackoffLogStaysReadable":         "reads blindT",
		"TestQAPlan429BackoffLogsTheRawRetryAfter":     "reads blindT",
		"TestQAPlan429BackoffReadsAPreUpgradeSnapshot": "reads blindT",
		"TestQAPlan429BackoffResetsOnSuccess":          "reads blindT",
		"TestQAQuiet429EscalationSpansCallers":         "reads blindT",
		"TestScoreIssues":                              "reads blindT",
		// lpouiT (7)
		"TestQAPlan429BackoffIsOnTheLoudLine":           "reads lpouiT",
		"TestQAPlanStaleClassIgnoresABracketedSentence": "reads lpouiT",
		"TestQAPlanStaleLineIsTheMeasuredHour":          "reads lpouiT",
		"TestQAPlanStaleQuietWhereTheLineWouldLie":      "reads lpouiT",
		"TestQAPlanStaleStreakAndClass":                 "reads lpouiT",
		"TestQAPlanStaleStreakStopsAtATornLine":         "reads lpouiT",
		"TestQAPlanStaleThresholdIsAnEdge":              "reads lpouiT",
		// pulseNow (5)
		"TestQAScorecardCarriesTheShopPulseSection":            "reads pulseNow",
		"TestQAShopPulseArithmetic":                            "reads pulseNow",
		"TestQAShopPulseCountsTheUnclassifiedBucketSeparately": "reads pulseNow",
		"TestQAShopPulseLineReplacesTheRawOpenCount":           "reads pulseNow",
		"TestQAShopPulseNamesARepoItCouldNotRead":              "reads pulseNow",
		// catalogAt (4): the frozen instant the catalog-age pins date their
		// reading at, so a whole-minute render is exact (ranger-base-5hjyh).
		"TestAReadingInsideItsLeaseCarriesNoAgeClause":               "reads catalogAt",
		"TestProbeHonoursALiveCooldown":                              "reads catalogAt",
		"TestQA7vpAStaleCatalogLaunchesTheAskedForIdAndMarksNothing": "reads catalogAt",
		"TestVerdictNamesTheAgeOfTheReadingAndTheProbeOutcome":       "reads catalogAt",
		// ddivoT (3): the frozen instant ranger-base-ddivo's pins date the
		// incident's reading at — read only, and the reading itself is
		// seeded per test under a t.TempDir.
		"TestQAStaleLineNamesTheUnarmedGuardInsteadOfTheHeadroomRule": "reads ddivoT",
		"TestQAUnarmedGuardStillAsksWhileTheShopSpends":               "reads ddivoT",
		"TestQAWatchLoopRunningUnmutesTheMeter":                       "reads ddivoT",
		// OpsPatterns (2)
		"TestQAEveryOpsHitInTrackedMarkdownIsRuled": "reads OpsPatterns",
		"TestQAOpsShapeTableCanStillSayNo":          "reads OpsPatterns",
		// sandboxApplyRefusal (3)
		"TestQASandboxApplyProbeAgreesWithARenderedProfile": "calls sandboxApplyRefusal",
		"TestQASandboxApplyProbeGrid":                       "calls sandboxApplyRefusal",
		"TestQASandboxExecStaysOnPathInsideTheCage":         "calls sandboxApplyRefusal",
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
	// ── filter 2b: the PARENT-side fake dir ──────────────────────────────
	// fakeDir() with no argument answers $RHQ_FAKE_DIR, and after ADR 0047
	// D1 that variable is not set in the parent — so in a test it resolves
	// to the binary's own directory, which every test in the binary shares.
	// fakeDirOf(t) is the per-test one. This is not the fakeDirs exemption
	// above: that map IS partitioned per test (by the *testing.T — see the
	// exemption note above), and this call is the one way to read past the
	// partition. MEASURED (ranger-base-pj87l): three sites in
	// closeddirty_test.go read a bd call log this way, and two of them
	// failed in two different full runs once the tests around them went
	// parallel — "no closed-dirty create in the bd log" is a read of the
	// wrong log, not a sweep that did not run.
	fakeDirTainted := propagateIn(funcs, func(g *fn) bool { return g.bareFakeDir }, inTests)

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

	// reached: per test, the written package vars it names anywhere in the
	// test-file functions it calls — filter 2's verdict with the var kept,
	// which is what a clearance has to be compared against.
	reached := map[string][]string{}
	// otherFlag: held back by something a parallelOK line cannot argue away
	// — the environment, the binary-wide fakeDir(), or a serial entry.
	otherFlag := func(name string) bool {
		return envTainted[name] || fakeDirTainted[name] || serial[name] != ""
	}
	var tests []*fn
	for _, gs := range funcs {
		for _, g := range gs {
			if g.topTest {
				tests = append(tests, g)
				reached[g.name] = reachedVars(funcs, g.name, writtenVars, exempt)
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
		nFake := 0
		for _, g := range tests {
			if !envTainted[g.name] && fakeDirTainted[g.name] {
				nFake++
			}
		}
		fmt.Printf("  reach the binary-wide fakeDir() rather than fakeDirOf(t): %d\n", nFake)
		fmt.Printf("    named serial by hand (see the map): %d\n", nNamed)
	case "eligible": // env-clean AND no written pkg var AND not named serial
		for _, g := range tests {
			if !envTainted[g.name] && !varTainted[g.name] && !fakeDirTainted[g.name] && serial[g.name] == "" {
				fmt.Printf("%s\t%s\n", g.file, g.name)
			}
		}
	case "check":
		// The gate `make test` runs (ranger-base-pj87l). It is deterministic
		// — a static read of the same files, no clock and no box — which is
		// the only kind of red this package's timing story can afford: see
		// the charter in scripts/test-times.sh, which refuses to fail on a
		// wall clock for exactly that reason. What it catches is the DECAY
		// that produced the wall twice: a test lands, is eligible, carries
		// no t.Parallel, and nothing says so until the package outruns its
		// ceiling four days later.
		var unmarked, extra, shared []string
		for _, g := range tests {
			eligible := !envTainted[g.name] && !varTainted[g.name] && !fakeDirTainted[g.name] && serial[g.name] == ""
			switch {
			case eligible && !g.hasParallel:
				unmarked = append(unmarked, g.file+"\t"+g.name)
			case fakeDirTainted[g.name] && g.hasParallel:
				// This one IS a failure, unlike the note below: a parallel
				// test reading the binary-wide fakeDir() reads a log every
				// other test writes, and it fails as "the thing under test
				// did not happen" (ranger-base-pj87l).
				shared = append(shared, g.file+"\t"+g.name)
			case !eligible && g.hasParallel && !clearanceCovers(parallelOK[g.name], reached[g.name], otherFlag(g.name)):
				extra = append(extra, g.file+"\t"+g.name)
			}
		}
		if len(extra) > 0 {
			// This WAS a note, for as long as the set was a backlog nobody
			// had read: taking t.Parallel off a green test is a decision, not
			// a sweep, and the 45 that predated the filter (i7fa) were not
			// this tool's to decide. ranger-base-btdvw read all 45 — six lost
			// their t.Parallel, 39 are cleared in parallelOK above — so the
			// backlog is gone and what is left is decay: a test arriving with
			// t.Parallel over shared state and no argument for it. That is
			// the same failure the unmarked list below catches, in the other
			// direction, and it gets the same treatment.
			fmt.Fprintf(os.Stderr, "testparallel: %d test(s) carry t.Parallel that this tool would not give:\n", len(extra))
			for _, u := range extra {
				fmt.Fprintf(os.Stderr, "  %s\n", u)
			}
			fmt.Fprintf(os.Stderr, "\nRun `go run ./cmd/testparallel %s extra` for the var or env root behind\n"+
				"each. Then either drop t.Parallel and say why at the test, or — if the\n"+
				"state is read-only, per-test-keyed, or written only by serial tests — add\n"+
				"a line to parallelOK in cmd/testparallel/main.go naming that argument. The\n"+
				"line covers the vars its reason names and no other, so name every one.\n", dir)
			os.Exit(1)
		}
		if len(shared) > 0 {
			fmt.Fprintf(os.Stderr, "testparallel: %d parallel test(s) read the binary-wide fakeDir() instead of fakeDirOf(t):\n", len(shared))
			for _, u := range shared {
				fmt.Fprintf(os.Stderr, "  %s\n", u)
			}
			fmt.Fprintf(os.Stderr, "\nfakeDirOf(t) is the per-test one. fakeDir() answers the binary's own\n"+
				"directory in the parent since ADR 0047 D1, so a parallel test reading a bd\n"+
				"or herdr call log through it reads every other test's calls too.\n")
			os.Exit(1)
		}
		if len(unmarked) == 0 {
			fmt.Printf("testparallel: %s clean — every one of the %d eligible tests carries t.Parallel\n", dir, countEligible(tests, envTainted, varTainted, fakeDirTainted, serial))
			return
		}
		fmt.Fprintf(os.Stderr, "testparallel: %d tests in %s can take t.Parallel and do not:\n", len(unmarked), dir)
		for _, u := range unmarked {
			fmt.Fprintf(os.Stderr, "  %s\n", u)
		}
		fmt.Fprintf(os.Stderr, "\nAdd `t.Parallel()` as the first line of each, or give it a reason in the\n"+
			"serial map in cmd/testparallel/main.go. This package is one test binary on\n"+
			"one clock and has outrun that clock twice (ranger-base-2ggb, ranger-base-pj87l).\n")
		os.Exit(1)
	case "extra":
		// Every test carrying t.Parallel the three filters would not give it,
		// with what holds it back and whether a human has cleared it. The
		// list `check` fails on is this one, minus the CLEARED rows.
		for _, g := range tests {
			if !g.hasParallel {
				continue
			}
			var why []string
			if envTainted[g.name] {
				why = append(why, "env")
			}
			if varTainted[g.name] {
				// The MEASURED vars, not the class word: a clearance is an
				// argument about these names, so the line has to carry them.
				why = append(why, "pkgvar("+strings.Join(reached[g.name], " ")+")")
			}
			if fakeDirTainted[g.name] {
				why = append(why, "fakeDir")
			}
			if serial[g.name] != "" {
				why = append(why, "named-serial")
			}
			if len(why) == 0 {
				continue
			}
			state := "UNCLEARED"
			switch ok := parallelOK[g.name]; {
			case ok == "":
			case clearanceCovers(ok, reached[g.name], otherFlag(g.name)):
				state = "cleared: " + ok
			default:
				// A clearance that no longer describes the test: the gate
				// fails this row exactly as it fails an uncleared one.
				state = "UNCLEARED (recorded \"" + ok + "\" does not cover it)"
			}
			fmt.Printf("%-14s %s\t%s\t%s\n", strings.Join(why, ","), g.file, g.name, state)
		}
		// A clearance for a test that is no longer flagged is dead weight: it
		// reads as an argument someone is relying on. Named, not fatal —
		// removing a t.Parallel should not have to touch this file in the
		// same commit.
		flagged := map[string]bool{}
		for _, g := range tests {
			if g.hasParallel && (envTainted[g.name] || varTainted[g.name] || fakeDirTainted[g.name] || serial[g.name] != "") {
				flagged[g.name] = true
			}
		}
		var stale []string
		for n := range parallelOK {
			if !flagged[n] {
				stale = append(stale, n)
			}
		}
		sort.Strings(stale)
		for _, n := range stale {
			fmt.Printf("%-14s %s\t%s\n", "stale", "parallelOK", n)
		}
	case "envroots":
		// The census the next round of this work needs: every function that
		// writes the process environment, with the number of top-level tests
		// it holds serial that NO OTHER root also holds. That second number
		// is the one to sort by — a root sharing all its tests with another
		// root buys nothing on its own.
		var roots []string
		rootVars := map[string]string{}
		for n, gs := range funcs {
			for _, g := range gs {
				if !g.isTest || !g.envRoot {
					continue
				}
				var vs []string
				for v := range g.envVars {
					vs = append(vs, v)
				}
				sort.Strings(vs)
				if _, seen := rootVars[n]; !seen {
					roots = append(roots, n)
				}
				rootVars[n] = strings.Join(vs, ",")
			}
		}
		sort.Strings(roots)
		countTests := func(t map[string]bool) int {
			n := 0
			for _, g := range tests {
				if t[g.name] {
					n++
				}
			}
			return n
		}
		total := countTests(envTainted)
		type row struct {
			name, vars  string
			alone, only int
		}
		var rows []row
		for _, r := range roots {
			one := propagateIn(funcs, func(g *fn) bool { return g.name == r && g.envRoot }, inTests)
			rest := propagateIn(funcs, func(g *fn) bool { return g.envRoot && g.name != r }, inTests)
			rows = append(rows, row{r, rootVars[r], countTests(one), total - countTests(rest)})
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].only != rows[j].only {
				return rows[i].only > rows[j].only
			}
			return rows[i].alone > rows[j].alone
		})
		fmt.Printf("%d env roots hold %d of %d top-level tests serial\n", len(rows), total, len(tests))
		fmt.Printf("%-6s %-6s %s\n", "ONLY", "REACH", "root / variables")
		for _, r := range rows {
			if r.alone == 0 {
				continue
			}
			fmt.Printf("%-6d %-6d %s\t%s\n", r.only, r.alone, r.name, r.vars)
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

// reachedVars answers the written, non-exempt package vars a test names in
// its own body or in any test-file function it transitively calls. It is
// filter 2 (varTainted) with the var kept: the same walk over the same
// functions, so a test is in reached iff it is varTainted.
func reachedVars(funcs map[string][]*fn, test string, writtenVars, exempt map[string]bool) []string {
	seen := map[string]bool{test: true}
	queue := []string{test}
	vars := map[string]bool{}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, g := range funcs[n] {
			if !g.isTest {
				continue
			}
			for id := range g.idents {
				if writtenVars[id] && !exempt[id] {
					vars[id] = true
				}
			}
			for c := range g.calls {
				if !seen[c] {
					seen[c] = true
					queue = append(queue, c)
				}
			}
		}
	}
	var out []string
	for v := range vars {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// clearanceCovers answers whether a parallelOK reason waives a test AS IT IS
// TODAY. A clearance is an argument about named state, so it covers a test
// only when (a) written package vars are the whole of what holds the test
// back — no reason string argues away the environment, the binary-wide
// fakeDir() or a serial entry — and (b) every var the test reaches is named
// in the reason, as a whole identifier. A cleared test that grows a write to
// a var its line does not name is therefore a NEW finding, not a waived one
// (ranger-base-acvq3). Matching is by identifier so "reads blindT" covers
// blindT and nothing that merely contains it.
func clearanceCovers(reason string, vars []string, otherFlag bool) bool {
	if reason == "" || otherFlag || len(vars) == 0 {
		return false
	}
	named := map[string]bool{}
	for _, w := range strings.FieldsFunc(reason, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_')
	}) {
		named[w] = true
	}
	for _, v := range vars {
		if !named[v] {
			return false
		}
	}
	return true
}

func countEligible(tests []*fn, envTainted, varTainted, fakeDirTainted map[string]bool, serial map[string]string) int {
	n := 0
	for _, g := range tests {
		if !envTainted[g.name] && !varTainted[g.name] && !fakeDirTainted[g.name] && serial[g.name] == "" {
			n++
		}
	}
	return n
}

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
