package treepins

// QA pins for ranger-base-nhc23 — the on-demand `-race` arm is a committed
// target, and its census can still say no.
//
// THE DEFECT THESE HOLD. ranger-base-d0xvw decided a narrow `-race` arm over
// internal/posse's concurrency-carrying tests and wrote the recipe as ONE
// `go test ./internal/posse -run <106 names>`. internal/posse had become three
// build-tag binaries the day before (ranger-base-qp1hm), and the candidate set
// straddles all three: that command compiled the default build only, 86 of the
// 106 names matched nothing, and it exited 0. The arm was priced, decided and
// recorded over 19% of itself. It is rot #4 in armtags_qa_test.go's header,
// verbatim — "a `go test -run` that matches no test exits 0" — and it had
// already eaten ranger-base-y3x6n's recorded repro, which still prints
// `ok … [no tests to run]`.
//
// scripts/test-race.sh is the recipe as a program: it reads the candidate test
// names out of the candidate FILES every run, and censuses all three arms
// before it spends a minute, so a name compiled into no arm is a loud failure
// in seconds rather than a silent skip.
//
// WHY IT NEEDS PINNING AT ALL. Nothing runs this script: it is on-demand by
// design (~11.4 minutes of wall) and no gate can afford it. That is exactly
// the state in which an instrument goes quiet without anybody noticing —
// armtags' rot #2 one level out — so what the suite holds is the two halves
// that can rot while the arm sits unused:
//
//	ARM 1  the target still exists and still runs the script. Renamed or
//	       deleted, the arm becomes a command nobody can find.
//	ARM 2  the census still reaches every candidate test, in the tree as it
//	       stands. A retagged file, a fourth tag, a renamed test file — each
//	       takes tests out of the arm, and this is the only thing that says so.
//	ARM 3  the census can FAIL. Arm 2 over a census that has gone blind is a
//	       green that measures nothing, which is the defect one level in — the
//	       same shape as the arm it is about.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const raceArmScript = "scripts/test-race.sh"

// The floor is set below today's count and not at it: the number moves with
// every test added to a candidate file, and a floor that tracked it exactly
// would red on ordinary work. 106 on 2026-09-07, 107 on 2026-09-10, 215 on
// 2026-11-14 when ranger-base-87p6i put the other fifteen launcher files in
// the arm. The floor stays at 90: it is there to catch a reader that has gone
// blind, and a set that halved would still be a set somebody shrank on purpose.
const raceArmFloor = 90

var (
	raceArmTotalRe = regexp.MustCompile(`candidate set: (\d+) tests across (\d+) files`)
	raceArmPerArm  = regexp.MustCompile(`arm (\d) \([^)]*\)\s+(\d+)`)
	raceArmAllRe   = regexp.MustCompile(`every one of the (\d+) reachable in some arm`)
)

// raceArmCensus runs the script's census and answers its combined output plus
// the error. The census is the cheap half — it builds the three arms without
// `-race` and asks each binary for its test list — so the suite can afford it
// where it cannot afford the arm itself.
func raceArmCensus(t *testing.T, files string) (string, error) {
	t.Helper()
	cmd := exec.Command(raceArmScript, "--census")
	if files != "" {
		cmd.Env = append(os.Environ(), "POSSE_RACE_FILES="+files)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ARM 1 — `make test-race` is the way in, and it runs the script.
func TestQATheRaceArmHasATargetThatRunsTheScript(t *testing.T) {
	t.Parallel()
	mk := makefileText(t)
	recipe := makeRecipe(mk, "test-race")
	if len(recipe) == 0 {
		t.Fatalf("the Makefile has no `test-race` target — the on-demand `-race` arm is a script nobody can find, which is how it was prose in a notes file (ranger-base-nhc23)")
	}
	var runs bool
	for _, line := range recipe {
		if !isComment(line) && strings.Contains(line, raceArmScript) {
			runs = true
		}
	}
	if !runs {
		t.Errorf("`make test-race` does not run %s:\n%s", raceArmScript, strings.Join(recipe, "\n"))
	}
	phonyLine, phony := mkPrereqs(t, mk, ".PHONY")
	if !mkRuns(phony, "test-race") {
		t.Errorf(".PHONY does not name test-race — a file of that name in the tree would silence the target: %q", phonyLine)
	}
	info, err := os.Stat(raceArmScript)
	if err != nil {
		t.Fatalf("%s: %v", raceArmScript, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("%s is not executable (%v) — the recipe runs it by path", raceArmScript, info.Mode().Perm())
	}
}

// ARM 2 — every test in the candidate set is compiled into some arm, in the
// tree as it stands today. This is the arm that reds on a retag.
func TestQAEveryRaceArmCandidateTestIsCompiledIntoSomeArm(t *testing.T) {
	t.Parallel()
	out, err := raceArmCensus(t, "")
	if err != nil {
		t.Fatalf("%s --census: %v\n%s", raceArmScript, err, out)
	}

	m := raceArmTotalRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("the census printed no candidate-set line, so nothing below reads anything:\n%s", out)
	}
	total, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	if total < raceArmFloor {
		t.Errorf("the candidate set is %d tests, under the floor of %d — it was 106 when ranger-base-d0xvw priced it and 107 on 2026-09-10, so either a candidate file lost most of its tests or the reader that finds them has gone blind:\n%s", total, raceArmFloor, out)
	}

	perArm := raceArmPerArm.FindAllStringSubmatch(out, -1)
	if len(perArm) != 3 {
		t.Fatalf("the census named %d arms, want 3:\n%s", len(perArm), out)
	}
	for _, a := range perArm {
		n, err := strconv.Atoi(a[2])
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Errorf("arm %s compiles NONE of the candidate set — a whole partition of the arm would run nothing and exit 0:\n%s", a[1], out)
		}
	}

	// The census's own verdict line, which is the one the script exits 0 on.
	if a := raceArmAllRe.FindStringSubmatch(out); a == nil || a[1] != m[1] {
		t.Errorf("the census did not report all %d candidate tests reachable:\n%s", total, out)
	}
	t.Logf("candidate set %d tests: arm1=%s arm2=%s arm3=%s", total, perArm[0][2], perArm[1][2], perArm[2][2])
}

// ARM 3 — the control, in both shapes the census has to be able to refuse. A
// census that cannot say no leaves arm 2 green over an arm that runs nothing,
// which is the defect this whole file is about, one level in.
func TestQATheRaceArmCensusRefusesTheWrongShapes(t *testing.T) {
	t.Parallel()

	// A test file whose tests are in another package entirely: every name is
	// compiled into no arm of internal/posse. This is the planted rot #4.
	out, err := raceArmCensus(t, "internal/treepins/root_test.go")
	if err == nil {
		t.Errorf("the census passed over a candidate file whose tests are in NO arm — it cannot detect the silent skip it exists for:\n%s", out)
	}
	if !strings.Contains(out, "compiled into NO arm") || !strings.Contains(out, "TestTreePinsRunFromModuleRoot") {
		t.Errorf("the census refused, and its message neither says why nor names the unreachable test:\n%s", out)
	}

	// And a candidate file that is not in the tree, which is what a rename
	// looks like from here. It must not be read as "no tests, nothing to do".
	out, err = raceArmCensus(t, "internal/posse/nosuch_qa_test.go")
	if err == nil {
		t.Errorf("the census passed over a candidate file that does not exist:\n%s", out)
	}
	if !strings.Contains(out, "not in the tree") {
		t.Errorf("the census refused a missing candidate file without saying so:\n%s", out)
	}
}

// ─── ARM 4 — the candidate FILE set itself ───────────────────────────────────
//
// ranger-base-87p6i. Arms 1-3 hold the script and its census: that the arm is
// findable, that every NAMED test is compiled into some arm, and that the
// census can say no. None of them says the right FILES are named, and on
// 2026-11-14 they were not.
//
// WHAT HAPPENED. ranger-base-np54s was a data race in
// internal/posse/suspend_qa_test.go — a `strings.Builder` the test polled while
// `watchdogLoop`, started on a goroutine four lines above, appended to it.
// Deterministic on the first iteration under `-race`, and a panic rather than a
// failure, so it took the whole arm-3 binary down and every other test in that
// run reported nothing. `make test` found it once, by luck, under load. The
// instrument built for exactly that class could not see the file:
// suspend_qa_test.go was not in CANDIDATE_FILES, and neither were fourteen
// other files that run one of those loops on a goroutine of their own, nor a
// fifteenth that runs the pass itself. The arm was green over the file that
// had the race — armtags' rot #2 one level further out, and the same shape as
// the defect arms 1-3 are about: an instrument that covers less than it says
// it does and exits 0.
//
// WHAT THIS HOLDS. The script's comment states a criterion — the arm covers the
// tests that carry dispatch.go's gather fan-in, passcarry.go's fan-out, and
// watch.go's pulse/backup/guard/watchdog goroutines — and that criterion has a
// mechanical half nobody was checking: a test file that puts one of those four
// loops, or `Watch` (which starts all four), on a goroutine of its own is
// carrying that concurrency whatever the file is named and whatever it asserts.
// This derives that half from the test sources and holds the file list against
// it. Every such file is in CANDIDATE_FILES, or carries a row in
// raceArmExcluded saying what it would cost and why it is out.
//
// IT IS NOT A DERIVED LIST, and the difference is the judgement the script's
// comment defends. A file can carry this concurrency with no `go` statement of
// its own — dispatch_qa_test.go and dispatchparity_qa_test.go are both in the
// arm and neither launches a loop — so the derivation is a FLOOR on the list
// and never the whole of it. What it ends is the silent half: a file written
// tomorrow that launches one of these loops is in the arm the day it lands, or
// it is a row somebody had to type.
//
// KNOWN LIMIT. The transitive half reaches a test helper called by name
// (runBackupLoop, drainFixture, guardClockFixture, runLagWatch, carryFixture —
// all five of today's are plain funcs). A helper with a RECEIVER would be
// called through a selector and this would not follow it. The floor below is
// what guards that: a derivation that goes blind reads as "nothing to report",
// which is this file's own subject.

// raceArmLaunchers are the product entry points that put one of the goroutines
// this arm exists for on a goroutine of its own. The four loops are watch.go's
// (`pulseLoop`, `backupLoop`, `guardLoop`, `watchdogLoop`, each started by
// Watch); `Watch` is in the set because a test that runs IT on a goroutine is
// running all four.
var raceArmLaunchers = []string{"Watch", "backupLoop", "guardLoop", "pulseLoop", "watchdogLoop"}

// And `Run`, the pass itself: dispatch.go's gather fan-in lives inside it, so a
// test that launches it on a goroutine is observing that fan-in while it runs
// — this arm's subject as squarely as a loop is. It needs the receiver as well
// as the name, which the five above do not (MEASURED 2026-11-14 over
// internal/posse/*_test.go: all 35 `.Watch(` calls and both loop calls there
// have receiver `d`, and nothing else in the tree answers to those names).
// `Run` is nobody's unique name — of its 1,262 calls there, 806 are `t.Run`
// subtests and 25 are `cmd.Run` on an `exec.Cmd` — so the name alone would
// propose every test that runs a child process on a goroutine. The
// dispatcher's three spellings account for 383 of them.
//
// A dispatcher spelled some other way is a false NEGATIVE here, which is the
// direction this pin is allowed to miss in: it is a floor under a list a
// person maintains, and `launchlock_test.go` is in CANDIDATE_FILES because a
// person put it there. The floor exists so that the ordinary case is not left
// to somebody noticing.
var raceArmRunReceivers = []string{"d", "d2", "d3"}

// The floor is set below today's count for the same reason raceArmFloor is:
// the number moves with ordinary work. 21 files on 2026-11-14.
const raceArmLaunchFloor = 15

// raceArmExcluded prices a file the derivation proposes and the arm
// deliberately does not run. A row is the MEASURED seconds it would add to the
// arm, the box and the date, and why that buys nothing — the same currency
// ADR 0098 D2 puts on a treepins exclusion, because this arm's cost is also
// wall. Empty on 2026-11-14: every file the derivation proposes is in the arm
// (ranger-base-87p6i priced the whole set at a doubling of a 7.7-minute
// on-demand arm and paid it). A row is not "its subject is not concurrency" —
// that was the reading this pin exists to correct.
var raceArmExcluded = map[string]string{}

// raceArmSite is where a file launches one of raceArmLaunchers, and which one.
type raceArmSite struct {
	what  string // the launcher
	where string // "file:line", or "via <helper>"
}

// raceArmScanParsed derives the launcher files from parsed test sources. It is
// separated from the read so the control arm can run it over a fixture.
func raceArmScanParsed(fset *token.FileSet, files map[string]*ast.File) map[string]raceArmSite {
	launches := func(n ast.Node) (string, bool) {
		var what string
		ast.Inspect(n, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || what != "" {
				return true
			}
			if slices.Contains(raceArmLaunchers, sel.Sel.Name) {
				what = sel.Sel.Name
				return true
			}
			recv, isIdent := sel.X.(*ast.Ident)
			if sel.Sel.Name == "Run" && isIdent && slices.Contains(raceArmRunReceivers, recv.Name) {
				what = recv.Name + ".Run"
			}
			return true
		})
		return what, what != ""
	}

	out := map[string]raceArmSite{}
	helpers := map[string]raceArmSite{} // helper func name -> the site inside it
	calls := map[string][]string{}      // file -> idents it calls
	for name, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if g, ok := n.(*ast.GoStmt); ok {
					if what, yes := launches(g); yes {
						site := raceArmSite{what: what, where: fmt.Sprintf("%s:%d", name, fset.Position(g.Go).Line)}
						if _, seen := out[name]; !seen {
							out[name] = site
						}
						if fn.Recv == nil && !strings.HasPrefix(fn.Name.Name, "Test") {
							if _, seen := helpers[fn.Name.Name]; !seen {
								helpers[fn.Name.Name] = site
							}
						}
					}
				}
				if call, ok := n.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok {
						calls[name] = append(calls[name], id.Name)
					}
				}
				return true
			})
		}
	}

	// The transitive half, to a fixed point: a file that calls a helper which
	// launches is running the loop too. watchhang_qa_test.go reaches backupLoop
	// this way and nothing else does today.
	for changed := true; changed; {
		changed = false
		for name, called := range calls {
			if _, have := out[name]; have {
				continue
			}
			for _, c := range called {
				if site, ok := helpers[c]; ok {
					out[name] = raceArmSite{what: site.what, where: "via " + c + " (" + site.where + ")"}
					changed = true
					break
				}
			}
		}
	}
	return out
}

// raceArmScan parses the subject package's test files and derives the launcher
// set from the tree as it stands.
func raceArmScan(t *testing.T) map[string]raceArmSite {
	t.Helper()
	ents, err := os.ReadDir(armPkgDir)
	if err != nil {
		t.Fatalf("read %s: %v", armPkgDir, err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		rel := filepath.Join(armPkgDir, e.Name())
		f, err := parser.ParseFile(fset, rel, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", rel, err)
		}
		files[e.Name()] = f
	}
	if len(files) < 100 {
		t.Fatalf("%s: %d test files parsed — the derivation read the wrong directory", armPkgDir, len(files))
	}
	return raceArmScanParsed(fset, files)
}

var raceArmCandidateRe = regexp.MustCompile(`(?s)CANDIDATE_FILES=\$\{POSSE_RACE_FILES:-"(.*?)"\}`)

// raceArmCandidateFiles reads the script's own list, by the spelling the script
// runs. A list the pin could not find would leave this arm asserting nothing.
func raceArmCandidateFiles(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(raceArmScript)
	if err != nil {
		t.Fatalf("%s: %v", raceArmScript, err)
	}
	m := raceArmCandidateRe.FindStringSubmatch(string(b))
	if m == nil {
		t.Fatalf("%s has no `CANDIDATE_FILES=${POSSE_RACE_FILES:-\"…\"}` assignment — either the list moved or this reader has gone blind, and a reader that finds nothing says every file is covered", raceArmScript)
	}
	files := strings.Fields(m[1])
	if len(files) < 8 {
		t.Fatalf("%s names %d candidate files, want at least the 8 ranger-base-nhc23 priced: %v", raceArmScript, len(files), files)
	}
	return files
}

// ARM 4 — every test file that launches one of those loops, or a dispatcher
// pass, on a goroutine of its own is in the arm or is a priced row. This is the
// arm that reds on a test file written tomorrow.
func TestQAEveryTestFileThatLaunchesAWatchLoopIsACandidateOrPriced(t *testing.T) {
	t.Parallel()

	launchers := raceArmScan(t)
	if len(launchers) < raceArmLaunchFloor {
		t.Fatalf("the derivation found %d test files launching one of %v, or Run on one of %v, on a goroutine, under the floor of %d (21 on 2026-11-14) — a renamed package directory or a scan that stopped seeing `go` statements reads as 'every file is covered', which is the green that measures nothing this file is about", len(launchers), raceArmLaunchers, raceArmRunReceivers, raceArmLaunchFloor)
	}

	candidates := raceArmCandidateFiles(t)
	for _, name := range slices.Sorted(maps.Keys(launchers)) {
		rel := filepath.Join(armPkgDir, name)
		if slices.Contains(candidates, rel) {
			continue
		}
		if why, priced := raceArmExcluded[rel]; priced {
			t.Logf("%s is priced out of the arm: %s", rel, why)
			continue
		}
		t.Errorf("%s launches %s on a goroutine (%s) and is neither in %s's CANDIDATE_FILES nor priced in raceArmExcluded — the on-demand `-race` arm does not cover it, and nothing but this pin says so. ranger-base-np54s's race landed in exactly such a file (ranger-base-87p6i)", rel, launchers[name].what, launchers[name].where, raceArmScript)
	}

	// A row for a file the derivation does not propose is a row nobody can
	// read: it prices out a file that was never in by this criterion, and it
	// goes stale silently when the file stops launching.
	for _, rel := range slices.Sorted(maps.Keys(raceArmExcluded)) {
		why := raceArmExcluded[rel]
		if _, ok := launchers[filepath.Base(rel)]; !ok {
			t.Errorf("raceArmExcluded prices %s (%q) and the derivation does not propose it — either the file stopped launching one of %v or the row is stale; delete the row", rel, why, raceArmLaunchers)
		}
		if slices.Contains(candidates, rel) {
			t.Errorf("raceArmExcluded prices %s (%q) and %s runs it anyway — a row and a candidate are contradictory halves of one decision", rel, why, raceArmScript)
		}
	}
	t.Logf("%d test files launch %v (or Run, on %v) on a goroutine; %d candidate files, %d priced out", len(launchers), raceArmLaunchers, raceArmRunReceivers, len(candidates), len(raceArmExcluded))
}

// ARM 5 — the control for arm 4's derivation, which is the half that can go
// blind. A scan that finds nothing leaves arm 4 green over a list that covers
// nothing, the same relationship arm 3 has to arm 2.
func TestQATheRaceArmFileDerivationFindsTheShapesItMustFind(t *testing.T) {
	t.Parallel()

	const direct = `package posse

func TestDirect(t *testing.T) {
	go func() { p, _ := d.Watch(ctx, "", "", 0, base, base); done <- p }()
}
`
	const bare = `package posse

func TestBare(t *testing.T) {
	go d.watchdogLoop(ctx, time.Millisecond, 0, 0)
}
`
	const viaHelper = `package posse

func runIt(t *testing.T, d *Dispatcher) {
	go func() { defer close(done); d.backupLoop(ctx, cfg) }()
}

func TestViaHelper(t *testing.T) { runIt(t, d) }
`
	const notLaunched = `package posse

func TestSynchronous(t *testing.T) {
	d.Watch(ctx, "", "", 0, base, base)
	go func() { close(done) }()
}
`
	const dispatcherRun = `package posse

func TestPassOnAGoroutine(t *testing.T) {
	go func() {
		n, err := d.Run("", "", 0)
		done <- n
		_ = err
	}()
}
`
	// The two shapes the receiver has to separate `Run` from, both of which a
	// name-only rule would propose: a child process launched in a goroutine,
	// and a subtest.
	const otherRuns = `package posse

func TestOtherRuns(t *testing.T) {
	cmd := exec.Command("/usr/bin/true")
	go func() { _ = cmd.Run() }()
	go func() { _ = exec.Command("/usr/bin/true").Run() }()
	go func() { t.Run("sub", func(t *testing.T) {}) }()
}
`
	for _, tc := range []struct {
		name string
		src  map[string]string
		want map[string]string // file -> launcher, absent means not proposed
	}{
		{"a go statement whose literal calls Watch", map[string]string{"a_test.go": direct}, map[string]string{"a_test.go": "Watch"}},
		{"a bare `go d.watchdogLoop(...)`", map[string]string{"b_test.go": bare}, map[string]string{"b_test.go": "watchdogLoop"}},
		{"a helper called by name", map[string]string{"c_test.go": viaHelper}, map[string]string{"c_test.go": "backupLoop"}},
		{"Watch called synchronously, and a goroutine that launches nothing", map[string]string{"d_test.go": notLaunched}, map[string]string{}},
		{"a dispatcher pass on a goroutine", map[string]string{"g_test.go": dispatcherRun}, map[string]string{"g_test.go": "d.Run"}},
		{"exec.Cmd.Run and t.Run on goroutines, which are not the pass", map[string]string{"h_test.go": otherRuns}, map[string]string{}},
		{"the helper's own file and its caller, together", map[string]string{"e_test.go": viaHelper, "f_test.go": "package posse\n\nfunc TestCaller(t *testing.T) { runIt(t, d) }\n"}, map[string]string{"e_test.go": "backupLoop", "f_test.go": "backupLoop"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			files := map[string]*ast.File{}
			for name, src := range tc.src {
				f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
				if err != nil {
					t.Fatalf("parsing the fixture: %v", err)
				}
				files[name] = f
			}
			got := raceArmScanParsed(fset, files)
			if len(got) != len(tc.want) {
				t.Fatalf("the derivation proposed %d files, want %d: %+v", len(got), len(tc.want), got)
			}
			for name, what := range tc.want {
				site, ok := got[name]
				if !ok {
					t.Fatalf("the derivation did not propose %s: %+v", name, got)
				}
				if site.what != what {
					t.Errorf("%s: proposed for %q, want %q (%s)", name, site.what, what, site.where)
				}
			}
		})
	}
}
