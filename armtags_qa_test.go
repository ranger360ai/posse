package posse

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
//  5. a file compiled into an arm calls a declaration that is not in that
//     arm, and the arm stops BUILDING. Every arm above stays green, because
//     each of them reads build LINES: an untagged file is a legal shape, and
//     "shared, compiled into all three" is exactly what it says. The
//     partition is total by tags and broken by symbols. Arm 6.
//
// Arm 5 is this file's own: the classifier has to be able to say no, over a
// planted file of each wrong shape, or the four arms above are a spelling
// exercise.
//
// MUTATION-CHECKED. Retagging one file `posse_arm4` reds arm 1 alone;
// dropping the `-tags posse_arm3` line from `make test`'s recipe reds arm 2
// alone, and so does a `test-arm2` recipe whose timeout stops matching
// `test`'s; moving `TestTreeIsGofmtClean` into arm 2 reds arm 4 alone.
// Giving `test-arm1` a `-tags` of its own reds arm 3 AND arm 2 — arm 2
// because the recipe then no longer matches `test`'s, which is a second true
// thing about the same edit rather than a leak between the two.
//
// Arm 6's four, measured on ranger-base-pv5vt's tree. Putting back either
// half of that bead's fix — the tag on `retirekept_qa_test.go`, or
// `containsStr`'s move out of the arm-3 `pulse_test.go` — reds its compile
// half for arms 1 and 2 and reds no other arm in this file, which is the
// whole reason it exists. Renaming a shared file to `*_linux_test.go` reds
// the file-set half alone, in all three arms, with the compile half and all
// five arms above still green — that is the constraint armBuildLine cannot
// see. Giving a shared file a build line satisfied by two arms
// (`!posse_arm3`) reds the OTHER direction of that set, and arm 1 with it:
// two true statements about one edit, not a leak.

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
// carries the files whose build line is neither empty nor one of the three.
func armClassify(files []armFile) (perArm map[int]int, shared int, unknown []armFile) {
	perArm = map[int]int{1: 0, 2: 0, 3: 0}
	for _, f := range files {
		if len(f.tests) == 0 && f.build == "" {
			continue // a helper-only file, which is the shape helpers should have
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
		t.Errorf("%s/%s: build tag %q is not one of the three arms — nothing runs this file",
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

// makefileText is the Makefile, read from the repo root the root package's
// tests already run in.
func makefileText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	return string(b)
}

// armTarget returns a Makefile target's prerequisite text and its recipe
// lines. A recipe line is a tab-indented line after the target header; the
// first line that is neither tab-indented nor blank ends it.
func armTarget(t *testing.T, mk, name string) (deps string, recipe []string) {
	t.Helper()
	lines := strings.Split(mk, "\n")
	for i, line := range lines {
		s, ok := strings.CutPrefix(line, name+":")
		if !ok || strings.HasPrefix(s, "=") {
			continue
		}
		deps = strings.TrimSpace(s)
		for _, r := range lines[i+1:] {
			if strings.HasPrefix(r, "\t") {
				recipe = append(recipe, strings.TrimSpace(r))
				continue
			}
			if strings.TrimSpace(r) == "" {
				continue
			}
			break
		}
		return deps, recipe
	}
	t.Fatalf("the Makefile has no `%s:` target", name)
	return "", nil
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
		for _, gate := range strings.Fields(deps) {
			if !strings.Contains(testDeps, gate) {
				t.Errorf("`test-arm1:` names the gate %s and `test:` does not — the two lines have drifted", gate)
			}
		}
		for _, gate := range strings.Fields(testDeps) {
			if !strings.Contains(deps, gate) {
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
		if f.build != "" && armExpr[f.build] != 1 {
			t.Errorf("door pin %s lives in %s (arm %d): the door runs `go test -run` with no tag, "+
				"so it selects nothing and exits 0", n, f.name, armExpr[f.build])
		}
	}
	if seen < 15 {
		t.Fatalf("only %d door pins resolved to a file in %s — the census is reading the wrong names", seen, armPkgDir)
	}
}

// ARM 6 — every arm COMPILES, and the files the toolchain puts in it are the
// files this census says are in it.
//
// The five arms above pin the partition BY TAGS, and a partition total by
// tags can still be broken by SYMBOLS. ranger-base-pv5vt is the shape:
// `retirekept_qa_test.go` landed with no build line — legal, and read
// everywhere here as "shared, compiled into all three" — while the helpers
// it calls live in `retiresweep_qa_test.go` and `retirecmd_qa_test.go`,
// which a later branch tagged `posse_arm3`. Each branch was green alone.
// Together, arms 1 and 2 of internal/posse did not build: `make test`'s
// first line failed, two thirds of the package ran NOTHING, the root
// package's TestQATheTreeWideDoorsReportRealDrift went red reporting drift
// that was not there — and every arm above passed over that tree, because
// every build line in it was one of the three legal spellings. A second
// instance was in the same tree and had never been reported at all
// (`containsStr`, defined in the arm-3 `pulse_test.go` and called from four
// arm-1 files and one shared one), which is what a class with no pin looks
// like.
//
// So this asks the toolchain instead of the file text. CI already vets each
// arm in its own job and would have caught it there; the point of asking
// here is WHEN — inside `go test ./...`, which is `make test`'s first line,
// for ~4s per tagged arm on top of a run that has already built arm 1, in
// front of an arm that costs ~236s. A seat learns in the first seconds that
// another arm does not build, instead of after the one arm that does.
//
// THE FILE-SET HALF IS WHAT KEEPS THE COMPILE HALF HONEST. `go vet` over a
// package that matched nothing exits 0, so a census reading the wrong
// directory would pass this three times. Comparing the set `go list` builds
// for each arm against the set armClassify would put there makes the two
// readers check each other, and it catches the constraints armBuildLine
// cannot see at all: a legacy `// +build` line, a platform filename suffix,
// a tag like `ignore` — any of which takes a file out of every arm while
// leaving its "shared" build line in place.
func TestQAEverySuiteArmCompiles(t *testing.T) {
	t.Parallel()
	files := armFiles(t)

	for a := 1; a <= 3; a++ {
		// arm 1 is the default build and is asked for with no -tags at all,
		// which is the same thing arm 3 above pins about test-arm1's recipe.
		var tags []string
		if a != 1 {
			tags = []string{"-tags", armTagFor(a)}
		}
		run := func(verb string, extra ...string) ([]byte, []string, error) {
			argv := []string{verb}
			argv = append(argv, tags...)
			argv = append(argv, extra...)
			argv = append(argv, "./"+armPkgDir)
			out, err := exec.Command("go", argv...).CombinedOutput()
			return out, argv, err
		}

		// the set the toolchain actually builds for this arm
		out, argv, err := run("list", "-f", `{{range .TestGoFiles}}{{.}}
{{end}}`)
		if err != nil {
			t.Fatalf("arm %d: `go %s` failed: %v\n%s", a, strings.Join(argv, " "), err, out)
		}
		built := map[string]bool{}
		for _, line := range strings.Split(string(out), "\n") {
			if n := strings.TrimSpace(line); n != "" {
				built[n] = true
			}
		}
		if len(built) < 100 {
			t.Fatalf("arm %d: `go %s` named %d test files — the toolchain and this census are not reading the same package",
				a, strings.Join(argv, " "), len(built))
		}

		// the set this file's own classifier says is in it
		want := map[string]bool{}
		for _, f := range files {
			if f.build == "" || armExpr[f.build] == a {
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
			t.Errorf("%s/%s is built into arm %d and this census does not put it there — "+
				"the two readers of the partition have drifted", armPkgDir, n, a)
		}

		// and it compiles
		if out, argv, err := run("vet"); err != nil {
			t.Errorf("arm %d does not build — `go %s` failed (%v):\n%s\n"+
				"a file compiled into this arm calls a declaration that is not in it. Either give the "+
				"caller the tag its helpers carry, or lift the helpers into an untagged `*_helpers_test.go` "+
				"so every arm compiles them (ranger-base-qp1hm, ranger-base-pv5vt). Every other pin in "+
				"this file stays green over exactly this, which is why arm 6 exists.",
				a, strings.Join(argv, " "), err, out)
		}
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
	for a := 1; a <= 3; a++ {
		t.Run(armTargetName(a), func(t *testing.T) {
			t.Parallel()
			args := []string{"vet"}
			if a != 1 {
				// arm 1 is the default build and takes no tag; asking for
				// one by name here would stop measuring the build a bare
				// `go test ./internal/posse` actually runs.
				args = append(args, "-tags", armTagFor(a))
			}
			args = append(args, "./"+armPkgDir)
			cmd := exec.Command(goBin, args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("arm %d does not type-check: `go %s` exited %v.\n"+
					"A file compiled into this arm names something no file in this arm declares — "+
					"most often an UNTAGGED file reaching for a helper that lives behind one arm's tag.\n%s",
					a, strings.Join(args, " "), err, out)
			}
		})
	}
}
