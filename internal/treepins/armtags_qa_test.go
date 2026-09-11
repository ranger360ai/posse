package treepins

// QA pins for ranger-base-qp1hm — internal/posse's suite is three binaries,
// and this is what keeps the partition total.
//
// Claim: every test in internal/posse runs in at least one arm, the arms are
// the three the Makefile runs, and arm 1 is the DEFAULT build.
//
// The split is a build tag per test-bearing file:
//
//	//go:build !posse_arm2 && !posse_arm3   arm 1, the default build
//	//go:build posse_arm2                   arm 2
//	//go:build posse_arm3                   arm 3
//	(no build line)                         shared — compiled into all three
//
// And one axis that is NOT the partition: a constraint naming no arm tag
// splits the package by BUILD MODE — a flag on the command line, not a tag
// — and a file behind one is in every arm, in the mode that builds it.
// `race` is the whole of it today (armModeAxis). Reading such a file as an
// unknown ARM would be two wrong answers at once: arm 1 would call it a
// file nothing runs, and arm 6 would call the partition drifted, because
// the toolchain does build it and this census would not put it anywhere.
//
// WHY IT NEEDS A PIN AT ALL, and why every arm here is about SILENCE. A
// build tag removes a file from a build; there is no diagnostic for a test
// that stopped being compiled. Four ways this rots, and none of them is
// visible in a green run:
//
//  1. a new test file lands with a tag nobody runs — `posse_arm4`, or
//     `posse_arm2 && posse_arm3`, which is satisfiable by no arm — and its
//     tests are in the tree, gofmt-clean, vetted by nothing and run by
//     nothing. Arm 1 below.
//  2. the Makefile stops running an arm (a target renamed, a recipe edited)
//     and a third of the package goes quiet while `make test` still exits 0.
//     Arm 2.
//  3. arm 1 stops being the default — somebody gives it a positive tag for
//     symmetry — and then a bare `go test ./internal/posse`, which is what
//     78% of measured suite entries actually were (ranger-base-uvzjk's
//     census), runs NO tests and says `ok`. Arm 3.
//  4. a pin named by a Makefile `-run` door drifts into arm 2 or 3. The door
//     takes no tag, so its filter selects nothing — and a `go test -run` that
//     matches no test exits 0. The door stays green over a pin that no longer
//     runs in it. Arm 4, and it is the one that would have been found last.
//
// Arm 5 is this file's own: the classifier has to be able to say no, over a
// planted file of each wrong shape, or the four arms above are a spelling
// exercise.
//
// Arm 6 is TestQAEverySuiteArmTypeChecks, at the foot of this file: the
// partition can be total by TAGS and broken by SYMBOLS, so each arm is also
// vetted for real. `go vet` over a package that matched nothing exits 0
// too, so each arm also asks the toolchain WHICH FILES it built and checks
// that set against what this census puts there — the only way to catch a
// constraint armBuildLine cannot see at all (a platform filename suffix, a
// legacy `// +build` line, a tag like `ignore`), any of which takes a file
// out of every arm while its build line still reads "shared".
//
// Arm 7 is the fifth way this rots, found once the first four were held
// (ranger-base-f8zw2). Arm 6 lives in the repo ROOT package, which only arm
// 1's `./...` line builds — so the arms a mistagged file actually reds, 2
// and 3, could not run the pin that explains it. `make test-arm2` and `make
// test-arm3` carry a `-run` door onto it, and arm 7 pins those two lines.
//
// MUTATION-CHECKED. Retagging one file `posse_arm4` reds arm 1 alone;
// dropping the `-tags posse_arm3` line from `make test`'s recipe reds arm 2
// alone, and so does a `test-arm2` recipe whose timeout stops matching
// `test`'s; moving `TestTreeIsGofmtClean` into arm 2 reds arm 4 alone.
// Giving `test-arm1` a `-tags` of its own reds arm 3 AND arm 2 — arm 2
// because the recipe then no longer matches `test`'s, which is a second true
// thing about the same edit rather than a leak between the two.
//
// MUTATION-CHECKED for the mode axis too (ranger-base-0dt50): misspelling
// internal/posse/fakecallcost_test.go's constraint as `!raced` reds arm 1
// ("no arm `make test` runs builds this file") and all three of arm 6's
// file-set halves ("built into arm N and this census does not put it
// there") — the same pair of readings the unlisted spelling `!race`
// produced before the axis was named, and worth having: `!raced` names a
// tag nobody passes, so it is SATISFIED, the toolchain builds the file into
// every arm, and only the census disagrees. Listing a mode is therefore not
// a way to silence either arm; it is a way to answer them.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The three arm expressions, in the exact spelling a file must carry. Arm 1
// is written as the negation of the other two so that it IS the default
// build; the map is keyed by expression because that is what the files hold
// and what a typo changes.
var armExpr = map[string]int{
	"!posse_arm2 && !posse_arm3": 1,
	"posse_arm2":                 2,
	"posse_arm3":                 3,
}

// armModeAxis names the build constraints that split internal/posse by
// BUILD MODE rather than by arm, mapped to whether the mode THIS census
// runs in — the default one, no extra flags — compiles the file. A file
// behind one of these is in every arm; what decides whether it is built is
// a flag, so the arm partition has nothing to say about it.
//
// `race` is the whole axis: fakecallcost_test.go and its `_race_test.go`
// sibling declare one constant at two values, because a fixture budget
// bounding work done in forked fake-herdr calls is a different number of
// calls under `-race` (ranger-base-0dt50). Both files are in all three
// arms; exactly one is built in any given run.
//
// Nothing here weakens arm 1's refusal: an expression that is neither an
// arm nor a listed mode is still a file nothing runs. Adding a mode means
// adding BOTH of its spellings here, and the value is the answer for the
// census's own build — so `race` is false, and a pin that lived behind it
// would correctly read as one no untagged `go test` door can select.
var armModeAxis = map[string]bool{
	"!race": true,
	"race":  false,
}

// armPlaces answers where a build line puts a file: whether it belongs to
// arm a at all, and whether the build mode this census measures compiles it
// there. The two are separate questions and only the mode axis answers them
// differently — an arm's own files are built whenever their arm is.
func armPlaces(build string, a int) (in, built bool) {
	if build == "" {
		return true, true
	}
	if b, ok := armModeAxis[build]; ok {
		return true, b
	}
	return armExpr[build] == a, true
}

const armPkgDir = "internal/posse"

// armBuildLine returns the //go:build constraint of a Go source file, or ""
// when it has none. Only a line above the package clause counts: a
// `//go:build` further down is a comment, which is exactly the mistake worth
// catching rather than honouring.
func armBuildLine(src string) string {
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "package ") {
			return ""
		}
		if s, ok := strings.CutPrefix(t, "//go:build "); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

var armTestFunc = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(`)

// armFile is one test file of the subject package.
type armFile struct {
	name  string
	build string   // "" for shared
	tests []string // top-level Test funcs, TestMain excluded
}

func armFiles(t *testing.T) []armFile {
	t.Helper()
	ents, err := os.ReadDir(armPkgDir)
	if err != nil {
		t.Fatalf("read %s: %v", armPkgDir, err)
	}
	var out []armFile
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(armPkgDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		src := string(b)
		f := armFile{name: e.Name(), build: armBuildLine(src)}
		for _, m := range armTestFunc.FindAllStringSubmatch(src, -1) {
			if m[1] != "TestMain" {
				f.tests = append(f.tests, m[1])
			}
		}
		out = append(out, f)
	}
	if len(out) < 100 {
		t.Fatalf("%s: %d test files — the census read the wrong directory", armPkgDir, len(out))
	}
	return out
}

// classify sorts the files and returns the per-arm test counts. Unknown
// carries the files whose build line is neither empty, nor one of the three
// arms, nor one of the build modes that cut across them.
func armClassify(files []armFile) (perArm map[int]int, shared int, unknown []armFile) {
	perArm = map[int]int{1: 0, 2: 0, 3: 0}
	for _, f := range files {
		if len(f.tests) == 0 && f.build == "" {
			continue // a helper-only file, which is the shape helpers should have
		}
		if built, ok := armModeAxis[f.build]; ok {
			// Every arm, but only in one mode. The mode this census does
			// not build is the mode `make test` does not run either, so a
			// file behind it that CARRIES TESTS is rot class 1 wearing a
			// legal tag — reported, not counted. Carrying no tests is the
			// shape that mode belongs in (a constant, a helper), and is
			// what fakecallcost_race_test.go is.
			if !built {
				if len(f.tests) > 0 {
					unknown = append(unknown, f)
				}
				continue
			}
			shared += len(f.tests)
			for a := range perArm {
				perArm[a] += len(f.tests)
			}
			continue
		}
		if f.build == "" {
			shared += len(f.tests)
			for a := range perArm {
				perArm[a] += len(f.tests)
			}
			continue
		}
		a, ok := armExpr[f.build]
		if !ok {
			unknown = append(unknown, f)
			continue
		}
		perArm[a] += len(f.tests)
	}
	return perArm, shared, unknown
}

// ARM 1 — every test-bearing file is either shared or in a named arm, and
// every arm has tests. A tag outside the three is a file nothing runs.
func TestQAEveryPosseTestFileIsSharedOrInANamedArm(t *testing.T) {
	t.Parallel()
	files := armFiles(t)
	perArm, shared, unknown := armClassify(files)
	for _, f := range unknown {
		t.Errorf("%s/%s: build tag %q — no arm `make test` runs builds this file, so nothing runs its tests",
			armPkgDir, f.name, f.build)
	}
	for a := 1; a <= 3; a++ {
		if perArm[a] == 0 {
			t.Errorf("arm %d holds no tests at all", a)
		}
	}
	if shared == 0 {
		t.Error("no shared (untagged) test file — TestMain's file must be one")
	}
	t.Logf("arm1=%d arm2=%d arm3=%d tests (of which %d shared, run in each)",
		perArm[1], perArm[2], perArm[3], shared)
}

// makefileText is the Makefile, read from the repo root this package's
// tests already run in.
func makefileText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	return string(b)
}

// armTarget returns a Makefile target's prerequisites, as TOKENS through
// mkPrereqs (ranger-base-hna69) rather than the line's bytes, and its recipe
// lines through makeRecipe. Membership against the result belongs to mkRuns,
// not strings.Contains against a joined string: that shape stops at neither
// a `#` nor a superstring, and is the class ranger-base-exv9h found this
// call site still doing.
func armTarget(t *testing.T, mk, name string) (deps []string, recipe []string) {
	t.Helper()
	_, deps = mkPrereqs(t, mk, name)
	return deps, makeRecipe(mk, name)
}

// ARM 2 — every arm the files declare is a line of `make test`, and every
// per-arm target repeats one of those lines verbatim.
//
// `test` keeps its own recipe rather than becoming an aggregate of the three
// per-arm targets, because six other pins read this target's literal
// prerequisite line and its literal recipe. The price is four duplicated
// lines and this is what keeps them honest: a `make test-arm2` that drifts
// from what `make test` runs would mean CI and a seat testing two different
// things under one name.
func TestQAMakefileRunsEverySuiteArm(t *testing.T) {
	t.Parallel()
	mk := makefileText(t)
	testDeps, testRecipe := armTarget(t, mk, "test")
	joined := strings.Join(testRecipe, "\n")

	// arm 1 is `./...` — the whole tree in the default build — and the two
	// tagged arms name the one package that has arms.
	if !strings.Contains(joined, "test -timeout 25m ./...") {
		t.Errorf("`make test`'s recipe no longer runs the default build over ./...:\n%s", joined)
	}
	for a := 2; a <= 3; a++ {
		tag := armTagFor(a)
		if !strings.Contains(joined, "-tags "+tag+" ./internal/posse") {
			t.Errorf("`make test` does not run `go test -tags %s ./internal/posse` — arm %d is in the tree and run by nothing:\n%s", tag, a, joined)
		}
	}

	// each per-arm target is a subset of what `make test` runs, and arm 1
	// carries the same gates, so CI's arm 1 job is not a lighter gate than a
	// seat's.
	for a := 1; a <= 3; a++ {
		deps, recipe := armTarget(t, mk, armTargetName(a))
		for _, line := range recipe {
			if !strings.Contains(joined, line) {
				t.Errorf("%s runs %q and `make test` does not — CI and a seat would be testing different things",
					armTargetName(a), line)
			}
		}
		if a != 1 {
			continue
		}
		for _, gate := range deps {
			if !mkRuns(testDeps, gate) {
				t.Errorf("`test-arm1:` names the gate %s and `test:` does not — the two lines have drifted", gate)
			}
		}
		for _, gate := range testDeps {
			if !mkRuns(deps, gate) {
				t.Errorf("`test:` names the gate %s and `test-arm1:` does not — CI's arm 1 job runs test-arm1 and would skip it", gate)
			}
		}
	}

	// and `make vet` has to see each arm, because `go vet ./...` is arm 1 only
	_, vet := armTarget(t, mk, "vet")
	vetJoined := strings.Join(vet, "\n")
	for a := 2; a <= 3; a++ {
		if !strings.Contains(vetJoined, "vet -tags "+armTagFor(a)+" ./internal/posse") {
			t.Errorf("`make vet` does not vet arm %d — two thirds of the test tree would go unvetted:\n%s", a, vetJoined)
		}
	}
}

func armTargetName(a int) string { return "test-arm" + string(rune('0'+a)) }
func armTagFor(a int) string     { return "posse_arm" + string(rune('0'+a)) }

// ARM 3 — arm 1 is the DEFAULT build. Its expression must be the negation of
// every other arm's tag, so that a bare `go test ./internal/posse` runs an
// arm rather than reporting `ok` over nothing.
func TestQAArmOneIsTheDefaultBuild(t *testing.T) {
	t.Parallel()
	var one string
	for expr, a := range armExpr {
		if a == 1 {
			one = expr
		}
	}
	for a := 2; a <= 3; a++ {
		tag := armTagFor(a)
		if !strings.Contains(one, "!"+tag) {
			t.Errorf("arm 1's expression %q does not exclude %s: with a positive tag on arm 1, "+
				"a bare `go test ./internal/posse` runs no tests and still says ok", one, tag)
		}
	}
	if strings.Contains(strings.ReplaceAll(one, "!posse_arm", ""), "posse_arm") {
		t.Errorf("arm 1's expression %q carries a positive tag", one)
	}
	// and the operational half: arm 1's own target runs `go test` with no
	// -tags at all. An arm 1 that had to be asked for by name would leave the
	// bare `go test ./internal/posse` a seat actually types running nothing.
	mk := makefileText(t)
	_, recipe := armTarget(t, mk, "test-arm1")
	m := strings.Join(recipe, "\n")
	if strings.Contains(m, "-tags ") {
		t.Errorf("test-arm1's recipe passes -tags — arm 1 is not the default build:\n%s", m)
	}
	if !strings.Contains(m, "./...") {
		t.Errorf("test-arm1 does not run `./...` — the other packages lose their run:\n%s", m)
	}
}

// armDoorVars are the Makefile variables holding the `-run` door filters. A
// door runs `go test -run` with NO tag, so every name in one has to be
// reachable in the default build.
var armDoorVars = []string{
	"QA_CREW_PINS", "QA_SEED_PINS", "QA_HISTORY_PINS",
	"QA_DOC_PINS", "QA_IDENTITY_PINS", "QA_OPS_PINS",
	// Missing until ranger-base-6cznr, which renamed this door's pin — the
	// exact drift this arm exists to catch, on the one door it could not see.
	"QA_EXECWRITE_PINS",
}

// ARM 4 — every pin a Makefile door names is in arm 1 or shared. This is the
// silent one: `go test -run` over a name that is not in the build exits 0, so
// a pin that drifts into arm 2 takes its door with it and both stay green.
func TestQAEveryMakefileDoorPinIsReachableInTheDefaultArm(t *testing.T) {
	t.Parallel()
	mk := makefileText(t)
	files := armFiles(t)

	home := map[string]armFile{}
	for _, f := range files {
		for _, name := range f.tests {
			home[name] = f
		}
	}

	var names []string
	for _, v := range armDoorVars {
		m := regexp.MustCompile(`(?m)^` + v + `\s*:?=\s*(.*)$`).FindStringSubmatch(mk)
		if m == nil {
			t.Errorf("Makefile has no %s — the door census reads a variable that is gone", v)
			continue
		}
		names = append(names, strings.Split(strings.TrimSpace(m[1]), "|")...)
	}
	// the two doors whose filter is written inline rather than in a variable
	names = append(names, "TestTreeIsGofmtClean", "TestLiveRuntimeContractWalk")
	sort.Strings(names)

	seen := 0
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		f, ok := home[n]
		if !ok {
			continue // the pin lives in another package; not this arm's subject
		}
		seen++
		// The door is a bare `go test -run`: no tag and no mode flag. A pin
		// it names has to be in arm 1 AND built by the default mode, or the
		// filter selects nothing.
		if in, built := armPlaces(f.build, 1); !in || !built {
			t.Errorf("door pin %s lives in %s (%q): the door runs `go test -run` with no tag and no mode flag, "+
				"so it selects nothing and exits 0", n, f.name, f.build)
		}
	}
	if seen < 15 {
		t.Fatalf("only %d door pins resolved to a file in %s — the census is reading the wrong names", seen, armPkgDir)
	}
}

// ARM 5 — the classifier can say no. Every arm above asserts an ABSENCE over
// files it read off disk, so a reader that silently classified everything as
// shared would leave all four green over any partition at all.
func TestQAArmClassifierRefusesTheWrongShapes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want string // expected build line
	}{
		{"arm 2", "//go:build posse_arm2\n\npackage posse\n\nfunc TestX(t *testing.T) {}\n", "posse_arm2"},
		{"arm 1", "//go:build !posse_arm2 && !posse_arm3\n\npackage posse\n", "!posse_arm2 && !posse_arm3"},
		{"shared", "package posse\n\nfunc TestX(t *testing.T) {}\n", ""},
		{"a tag below the package clause is a comment", "package posse\n\n//go:build posse_arm2\n", ""},
		{"an unknown arm", "//go:build posse_arm4\n\npackage posse\n", "posse_arm4"},
		{"an unsatisfiable pair", "//go:build posse_arm2 && posse_arm3\n\npackage posse\n", "posse_arm2 && posse_arm3"},
		{"a build mode, not an arm", "//go:build !race\n\npackage posse\n", "!race"},
		{"the other half of that mode", "//go:build race\n\npackage posse\n", "race"},
	}
	for _, c := range cases {
		if got := armBuildLine(c.src); got != c.want {
			t.Errorf("%s: build line %q, want %q", c.name, got, c.want)
		}
	}
	// and the two shapes arm 1 must reject are rejected
	for _, bad := range []string{"posse_arm4", "posse_arm2 && posse_arm3"} {
		if _, ok := armExpr[bad]; ok {
			t.Errorf("%q reads as a known arm", bad)
		}
	}
	// a file with tests and an unknown tag lands in `unknown`, not silently in an arm
	_, _, unknown := armClassify([]armFile{{name: "x_test.go", build: "posse_arm4", tests: []string{"TestX"}}})
	if len(unknown) != 1 {
		t.Errorf("a file tagged posse_arm4 was not reported as unknown: %v", unknown)
	}

	// THE MODE AXIS says no the same way. A listed mode is every arm — and
	// only the half this census's own build compiles counts as running —
	// while a near-miss spelling is still a file nothing runs.
	for _, c := range []struct {
		build                string
		wantArm1, wantShared int
		wantUnknown          int
		wantIn, wantBuilt    bool
		what                 string
	}{
		{"!race", 1, 1, 0, true, true, "the mode this census builds is in every arm and counted"},
		{"race", 0, 0, 1, true, false, "a TEST behind the mode make test never builds is rot class 1 in a legal tag"},
		{"!raced", 0, 0, 1, false, true, "a near-miss mode tag is a file nothing runs"},
	} {
		perArm, shared, unknown := armClassify([]armFile{{name: "x_test.go", build: c.build, tests: []string{"TestX"}}})
		if perArm[1] != c.wantArm1 || shared != c.wantShared || len(unknown) != c.wantUnknown {
			t.Errorf("%q: arm1=%d shared=%d unknown=%d, want %d/%d/%d — %s",
				c.build, perArm[1], shared, len(unknown), c.wantArm1, c.wantShared, c.wantUnknown, c.what)
		}
		// and the placement the type-check arm reads off the same line —
		// arm 2, so that "every arm" is asserted somewhere other than the
		// default one.
		if in, built := armPlaces(c.build, 2); in != c.wantIn || built != c.wantBuilt {
			t.Errorf("armPlaces(%q, 2) = in=%v built=%v, want in=%v built=%v", c.build, in, built, c.wantIn, c.wantBuilt)
		}
	}
}

// ARM 6 — every arm TYPE-CHECKS, its test files included.
//
// The five arms above are structural: they read build lines off disk and
// Makefile text, and every one of them asserts a SILENCE. None of them
// compiles anything, so all five were green at a main whose arms 1 and 2 did
// not build at all — ranger-base-gxfd4, out of ranger-base-i469g's verify of
// the ranger-base-g4s6o landing.
//
// The class they missed: an UNTAGGED file — compiled into all three arms —
// referring to a declaration that only ONE arm provides. Two files did it,
// against helpers a landing had just moved behind `//go:build posse_arm3`:
// resolved in arm 3, `undefined:` in arms 1 and 2. The partition was still
// total, every tag was still one of the three, every arm still had a Makefile
// line — arms 1 through 5 were all telling the truth. The tree simply did not
// compile.
//
// It is also a class no seat's own green can rule out, which is why it needs
// a pin rather than care. Each half was a legal partition alone; the two only
// meet at the landing, and the launcher rebases the bead branch onto main's
// tip and fast-forwards AFTER the close's suite run — so the tree that lands
// is not the tree that was measured whenever main moved during the seat's run.
//
// `go vet` is the type-check, and the cheapest one that sees test files: it
// builds the package under the tag it is given, with its `_test.go` files, so
// an undefined identifier in any arm is a nonzero exit here. It is what `make
// vet` already runs as three lines — this is the pin and that target is the
// door, the shape `TestTreeIsGofmtClean` and `make fmt-check` already have.
//
// Arms 2 and 3 are the two only this pin can see: `go test ./...`, which is
// what runs THIS file, builds arm 1 and nothing else. Arm 1 is checked too so
// that the claim is total and so that a break names its arm here, instead of
// arriving as a `[build failed]` line beside an `ok` for this package.
//
// MUTATION-CHECKED, with both halves of the escape this bead fixed. Deleting
// `//go:build posse_arm3` from internal/posse/retirekept_qa_test.go reds
// arm1 and arm2 and leaves arm3 green; putting internal/posse's shared
// verifybox_qa_test.go back on `containsStr`, which pulse_test.go declares
// behind `posse_arm3`, reds the same two. Neither mutant moves any of arms
// 1-5, which is the finding: a build line can be legal and the arm still not
// exist.
//
// And the control that proves the `-tags` is live rather than three spellings
// of one build: an undefined identifier appended to an arm-2-only file
// (accountstage_qa_test.go) reds test-arm2 alone, and the same line appended
// to an arm-3-only file (pulse_test.go) reds test-arm3 alone. Without it the
// two mutants above are equally explained by a pin that compiles arm 1 three
// times.
//
// THE FILE-SET HALF, and why the compile half alone is not enough
// (ranger-base-6m7rd): `go vet` over a package that matched NOTHING also
// exits 0, so if armPkgDir ever stopped naming a real package this arm would
// stay green while reporting that every arm type-checks. The same hole is a
// live class, not just a floor: every arm above decides which arm a file is
// in by reading its `//go:build` line, and Go has other ways to take a file
// out of a build that leave that line untouched and legal — a platform
// filename suffix (`x_linux_test.go`), a legacy `// +build` line with no
// `//go:build` beside it, a tag nothing passes (`//go:build ignore`). The
// filename-suffix case is silent in every direction: armBuildLine reads ""
// (shared, legal), arm 1 is green, and the file's tests run nowhere at all —
// the same shape as the defect this arm exists for. So each arm also asks
// `go list` which test files it actually built and compares that set both
// ways against what armClassify puts there: a name in the census but not in
// the build is a constraint this file cannot see; a name in the build but not
// the census means the two readers have drifted.
//
// MUTATION-CHECKED (ranger-base-6m7rd), on ranger-base-pv5vt at 55af5461.
// Renaming a shared file to `*_linux_test.go` fails the
// file-set half for all three arms while the vet half stays green — the
// finding in one mutant. Giving a shared file `//go:build !posse_arm3` fires
// the "built but not in the census" direction for arms 1 and 2 (armClassify
// puts an unrecognized expression in `unknown`, not in any arm's set).

// armGoTool is the `go` this arm shells out to: the one on PATH, else the one
// beside the GOROOT the running binary was built against. A pin that skipped
// when it could not find one would be a pin that reports nothing on the box
// where it matters, so a missing tool is fatal.
func armGoTool(t *testing.T) string {
	t.Helper()
	if p, err := exec.LookPath("go"); err == nil {
		return p
	}
	if root := runtime.GOROOT(); root != "" {
		p := filepath.Join(root, "bin", "go")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Fatal("no `go` on PATH and none in GOROOT: this arm cannot type-check anything")
	return ""
}

func TestQAEverySuiteArmTypeChecks(t *testing.T) {
	t.Parallel()
	goBin := armGoTool(t)
	files := armFiles(t)
	for a := 1; a <= 3; a++ {
		a := a
		t.Run(armTargetName(a), func(t *testing.T) {
			t.Parallel()
			var tags []string
			if a != 1 {
				// arm 1 is the default build and takes no tag; asking for
				// one by name here would stop measuring the build a bare
				// `go test ./internal/posse` actually runs.
				tags = []string{"-tags", armTagFor(a)}
			}
			run := func(verb string, extra ...string) (out []byte, argv []string, err error) {
				argv = append([]string{verb}, tags...)
				argv = append(argv, extra...)
				argv = append(argv, "./"+armPkgDir)
				out, err = exec.Command(goBin, argv...).CombinedOutput()
				return out, argv, err
			}

			// which files the toolchain actually built for this arm — see
			// "THE FILE-SET HALF" above for why the vet call below cannot
			// stand on its own.
			out, argv, err := run("list", "-f", "{{range .TestGoFiles}}{{.}}\n{{end}}")
			if err != nil {
				t.Fatalf("`go %s` failed: %v\n%s", strings.Join(argv, " "), err, out)
			}
			built := map[string]bool{}
			for _, line := range strings.Split(string(out), "\n") {
				if n := strings.TrimSpace(line); n != "" {
					built[n] = true
				}
			}
			if len(built) < 100 {
				t.Fatalf("`go %s` named %d test files — the toolchain and this census are not reading the same package",
					strings.Join(argv, " "), len(built))
			}
			want := map[string]bool{}
			for _, f := range files {
				if in, built := armPlaces(f.build, a); in && built {
					want[f.name] = true
				}
			}
			var missing, extra []string
			for n := range want {
				if !built[n] {
					missing = append(missing, n)
				}
			}
			for n := range built {
				if !want[n] {
					extra = append(extra, n)
				}
			}
			sort.Strings(missing)
			sort.Strings(extra)
			for _, n := range missing {
				t.Errorf("%s/%s reads as arm %d here and the toolchain does not build it there — "+
					"a constraint this census cannot see (a `// +build` line, a platform filename suffix, a tag like `ignore`) "+
					"has taken the file out of the arm while its build line still says it is in",
					armPkgDir, n, a)
			}
			for _, n := range extra {
				t.Errorf("%s/%s is built into arm %d and this census does not put it there — the two readers of the partition have drifted",
					armPkgDir, n, a)
			}

			if out, argv, err := run("vet"); err != nil {
				t.Errorf("arm %d does not type-check: `go %s` exited %v.\n"+
					"A file compiled into this arm names something no file in this arm declares — "+
					"most often an UNTAGGED file reaching for a helper that lives behind one arm's tag.\n%s",
					a, strings.Join(argv, " "), err, out)
			}
		})
	}
}

// armRunFilter returns the value of a `-run` flag in either spelling
// (`-run X` / `-run=X`, one dash or two), and whether one is present. The
// value arrives with its shell quotes already stripped by shellWords and
// with make's `$$` still doubled, which armDoorMatches undoubles.
func armRunFilter(args []string) (string, bool) {
	for i, a := range args {
		flag := strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-")
		switch {
		case flag == "run" && i+1 < len(args):
			return args[i+1], true
		case strings.HasPrefix(flag, "run="):
			return strings.TrimPrefix(flag, "run="), true
		}
	}
	return "", false
}

// armDoorMatches reports whether a `-run` filter selects a test by name. It
// undoubles make's `$$` first, because the regexp under test is the one the
// SHELL is handed, not the one the Makefile spells.
func armDoorMatches(t *testing.T, filter, name string) bool {
	t.Helper()
	re, err := regexp.Compile(strings.ReplaceAll(filter, "$$", "$"))
	if err != nil {
		t.Errorf("`-run %s` is not a regexp: %v — go test would refuse the door outright", filter, err)
		return false
	}
	return re.MatchString(name)
}

// armDoorBuildsTreePinsPkg reports whether a `go test` argument list names the
// treepins package, which is where the type-check pin lives. `./...`
// counts: it is arm 1's spelling and it builds treepins too.
func armDoorBuildsTreePinsPkg(args []string) bool {
	for _, a := range args {
		if a == "./internal/treepins" || a == "./..." {
			return true
		}
	}
	return false
}

// ARM 7 — the two tagged arms open a DOOR onto arm 6 above
// (ranger-base-f8zw2).
//
// TestQAEverySuiteArmTypeChecks vets all three arms, and it lives in THIS
// package — internal/treepins — which only arm 1's `./...` line builds. So the
// pin whose whole job is catching an untagged file that reaches behind one
// arm's tag ran in exactly one of CI's three jobs, and never in `go test
// -tags posse_arm3 ./internal/posse`, which is the command a seat verifying
// an arm actually types. `make test-arm2` and `make test-arm3` therefore
// carry a `-run` door onto it, and a door is a Makefile line, which is a
// thing that gets edited.
//
// It is arm 4's silence one level up, so the check is that the filter
// MATCHES the name rather than that the line mentions it: a `-run` selecting
// nothing exits 0, and so does a door pointed at ./internal/posse, where
// this pin does not live. Both are read here.
//
// MUTATION-CHECKED, on 2026-09-07. Dropping either door line reds this arm
// alone, and so does dropping the `-run` flag from one (arm 2 reads the
// recipe by substring, and the shorter line is still a substring of `make
// test`'s). Typoing the name inside the filter and repointing the door at
// ./internal/posse red this arm AND arm 2 — arm 2 because the line then no
// longer appears in `make test`'s recipe, which is a second true thing
// about the same edit. Dropping the door line from `make test`'s recipe
// alone reds arm 2 alone.
//
// The counts above were read before arm 8 below, which reads the same door
// and so joins several of them. RE-MEASURED 2026-09-10 (ranger-base-w9ihu),
// two of the four: dropping `test-arm2`'s door line reds this arm AND arm 8,
// and dropping `make test`'s reds arm 2 AND arm 8. The other two were not
// re-run; read every "alone" above as "alone among arms 1-7".
// armTypeCheckPin is the pin every door below is a door ONTO. Named once so
// arms 7 and 8 cannot drift onto two different tests.
const armTypeCheckPin = "TestQAEverySuiteArmTypeChecks"

// armDoorIndex returns the index in a recipe of the first line that is a
// type-check door — a `go test` over the ROOT package whose `-run` filter
// selects armTypeCheckPin — or -1 when the recipe carries none.
//
// isComment first: a recipe comment DISCUSSING the door is not the door, and
// this file's head comment quotes the very line.
func armDoorIndex(t *testing.T, recipe []string) int {
	t.Helper()
	for i, line := range recipe {
		if isComment(line) {
			continue
		}
		args := goTestArgs(line)
		if args == nil || !armDoorBuildsTreePinsPkg(args) {
			continue
		}
		filter, ok := armRunFilter(args)
		if !ok || !armDoorMatches(t, filter, armTypeCheckPin) {
			continue
		}
		return i
	}
	return -1
}

func TestQAEachTaggedArmTargetOpensTheTypeCheckDoor(t *testing.T) {
	t.Parallel()
	mk := makefileText(t)
	for a := 2; a <= 3; a++ {
		name := armTargetName(a)
		_, recipe := armTarget(t, mk, name)
		if armDoorIndex(t, recipe) < 0 {
			t.Errorf("`make %s` runs no `go test` over the treepins package whose `-run` filter selects %s.\n"+
				"That pin type-checks all three arms and lives in the ROOT package, which arm %d's recipe does not build, "+
				"so without this door an untagged file reaching behind one arm's tag reds only in a full `make test`:\n%s",
				name, armTypeCheckPin, a, strings.Join(recipe, "\n"))
		}
	}
}

// ARM 8 — the door runs BEFORE the lines it guards, in every recipe that
// carries one. That is ranger-base-p1r17 for `test-arm2`/`test-arm3` and
// ranger-base-sp5z2 for `test` itself, and until this arm neither fix was
// held by anything: arm 7 above reads only that a door is PRESENT, and both
// recipes carried a present door for the whole time the bug was live. Arm 2
// (TestQAMakefileRunsEverySuiteArm) reads each per-arm line as a SUBSTRING of
// `test`'s recipe, which is order-independent by construction. So a reorder
// — the entire content of both fixes — was a silent revert twice over.
//
// The mechanism: GNU make aborts a recipe on its first non-zero exit (no `-`
// prefix, no `.IGNORE`, and CI passes no `-k`), and the lines the door guards
// — `go test -tags posse_armN ./internal/posse` — are exactly the ones an
// untagged file reaching behind an arm's tag fails to COMPILE. A door written
// after them never runs for the one bug shape it exists to name.
//
// MEASURED 2026-09-10, on this tree, with an untagged
// internal/posse/w9ihuplant_test.go calling i9dbbRead (which lives behind
// `!posse_arm2 && !posse_arm3`): `make test` reds AT the door, naming
// test-arm2 and test-arm3 and the file:line. With the pre-e8f62920 line order
// replayed, the arm-2 line aborts the recipe first and only one `go test`
// runs at all.
//
// MUTATION-CHECKED 2026-09-10. Moving the door to the end of `test`'s recipe
// reds this arm alone; the same move in `test-arm2` reds this arm alone;
// deleting `test`'s door line reds this arm and arm 2, and deleting
// `test-arm2`'s reds this arm and arm 7 (not arm 2, which reads each per-arm
// line as a substring of `test`'s recipe and not the other way round).
func TestQAEveryTypeCheckDoorRunsBeforeTheLinesItGuards(t *testing.T) {
	t.Parallel()
	mk := makefileText(t)
	// `test` is here as well as the two tagged targets: it repeats both
	// tagged lines, so it has the same shape one level up.
	for _, name := range []string{"test", armTargetName(2), armTargetName(3)} {
		_, recipe := armTarget(t, mk, name)
		door := armDoorIndex(t, recipe)
		if door < 0 {
			t.Errorf("`make %s` carries no `-run` door onto %s — arm 7 above says why every recipe here needs one:\n%s",
				name, armTypeCheckPin, strings.Join(recipe, "\n"))
			continue
		}
		for i, line := range recipe {
			if i >= door || isComment(line) || goTestArgs(line) == nil {
				continue
			}
			t.Errorf("`make %s` runs a `go test` at recipe line %d and opens the %s door only at line %d:\n"+
				"\tline %d: %s\n\tline %d: %s\n"+
				"make aborts a recipe on its first non-zero exit, and an untagged file reaching behind an arm's tag "+
				"fails the earlier line's COMPILE — so the door never runs for the shape it exists to name. "+
				"ranger-base-p1r17 and ranger-base-sp5z2 each fixed this once.",
				name, i+1, armTypeCheckPin, door+1, i+1, line, door+1, recipe[door])
		}
	}
}
