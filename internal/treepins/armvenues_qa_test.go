package treepins

// QA pins for ranger-base-c3zb8 — every venue that runs internal/posse's
// build-tag arms is held to the PARTITION, not to the number three.
//
// THE GAP. internal/posse's tests are three binaries (ranger-base-qp1hm), and
// armtags_qa_test.go registers the partition: `armExpr` maps each build-tag
// expression to its arm number and reds when a file carries a tag that is not
// in the table. Several places then RUN those arms, each as a hand-written
// list of recipe lines:
//
//	make test            arm 1 (`./...`) + the two tagged lines
//	make vet             the same three, because `go vet ./...` is arm 1 only
//	make test-arm<N>     one target per arm, for CI's three jobs
//	make pkgcensus*      one door per arm, because a door's `-run` takes the
//	                     arm's tag or selects nothing
//	make coldexec-check  three of its five arms (the other two are packages)
//	make tmpdir-check    three arms with $TMPDIR off /tmp (ranger-base-m33n5)
//	ci.yml `arm: [...]`  which job numbers exist at all
//	test-race.sh         `arm_tag`, the -race arm's per-arm tag
//
// NOTHING COMPARED ANY OF THOSE LISTS TO `armExpr`. A fourth arm tomorrow
// gets an `armExpr` row, a `//go:build posse_arm4` file, and a tag that
// armtags' own census is satisfied by — and then no venue line anywhere, no
// red, and a door reading green over an arm it never ran. That is the same
// silence the whole of armtags_qa_test.go is about, one layer out: a build tag
// removes a file from a build and there is no diagnostic for a test that
// stopped being compiled, and there is none either for an ARM that was never
// started. `go test` over a package whose tag matched no file exits 0.
//
// WHY THE EXISTING PINS DO NOT CLOSE IT. armtags' arm 2
// (TestQAMakefileRunsEverySuiteArm) does read `make test`'s and `make vet`'s
// recipes — with `for a := 2; a <= 3; a++`. The three is typed there as well,
// so the pin that reads the venue is short by exactly the arm the venue is
// short by, and both are green. coldexecarms_qa_test.go is the worked example
// of a Makefile arm list held to a derivation (ranger-base-s64a5) — but its
// subject is WHICH PACKAGES plant an executable, so internal/posse's three
// tagged lines are one entry of its five and the arm count is not its claim.
//
// THE DERIVATION is `armExpr`, read through avArms: the arm numbers that
// table declares, which must be 1..n with no gap, and arm 1 is the DEFAULT
// build (armtags' arm 3 holds that separately, and it is why an untagged line
// over internal/posse is read here as arm 1 rather than as nothing).
//
// THE ARMS HERE
//
//  1. every arm has a line in every registered venue (avVenues). Forward
//     direction: the arm the tree declares and nobody runs.
//  2. the register is not stale, and no venue runs an arm the tree does not
//     declare. Backward direction, over EVERY recipe line in the Makefile
//     rather than over the register's targets — a `-tags posse_arm*` line in
//     a target nobody registered is the register going blind, which is the
//     failure mode a hand-kept list of venues has (armDoorVars went a door
//     short for as long as its door existed, ranger-base-na4rm). An arm tag
//     outside a recipe entirely — in a Makefile variable — is refused for
//     the same reason: the register could not attribute it to a target.
//  3. ci.yml's `arm:` matrix is the partition. Six jobs run `make
//     test-arm${{ matrix.arm }}`, so the matrix IS the list of arms CI runs
//     and a fourth arm with no matrix entry is an arm no job builds. Both
//     directions, like armtags' arm 8 over the `os:` matrix one key over.
//  4. test-race.sh's `arm_tag` is the partition, and the loops that index it
//     enumerate every element. The script's census already dies when a
//     CANDIDATE test is compiled into no arm ("a fourth tag" is in its own
//     die message), which is why this venue is the best defended of the
//     eight — but only if a race-prone test happens to land in the new arm.
//     With none there, the array is short and silent.
//  5. the reader can say no. Arms 1-4 are all built on avArmOfLine, so a
//     reader that answers "arm 1" to anything, or never answers at all, is
//     four green arms over nothing.
//
// WHAT THIS DOES NOT SEE, said plainly. The per-arm SUMMARY printf in
// test-race.sh (three `%s` rows) and racearm_qa_test.go's own `want 3` on the
// census output are both still typed. Each of them REDS on a fourth arm
// rather than going quiet — a four-arm census fails `!= 3` and the printf
// drops a row that the arm-4 line would have filled — so they are a
// confusing red and not a silence, and arm 4's error message names them so
// the red is actionable.
//
// DOORED BY THE COMPLEMENT, and without being named (ADR 0098,
// ranger-base-3cpcg). Every pin here reads tracked files by relative literal
// — `Makefile`, `.github/workflows/ci.yml`, `scripts/test-race.sh` — and
// enumerates no directory, which is the treepins rule's own negative case
// (treewidedoor_qa_test.go), so no `-run` door could find them. `make
// treepins-check` is this package MINUS $(QA_TREEPINS_EXCLUDED)'s priced
// table, none of these names is in it, and that target is on `make
// tree-check`'s line.
//
// MUTATION-CHECKED 2026-11-22 at 97979d7d on linux (go1.27.3), eight mutants,
// each applied to a tracked file and restored from a byte copy:
//
//   - adding `"posse_arm4": 4` to armExpr reds arms 1, 3 AND 4 — the fourth
//     arm has no Makefile line in any of the six venues, no ci.yml matrix
//     entry and no `arm_tag`. That IS the bead, and the three reds are three
//     different venues short rather than a leak between pins: arm 2 stays
//     green, correctly, because nothing in the tree RUNS the new arm, which
//     is the whole complaint.
//   - dropping tmpdir-check's `-tags posse_arm3` recipe line reds arm 1
//     alone, on that venue's row.
//   - renaming `pkgcensus-arm2-check` reds arm 1 (no such target) and arm 2
//     (its tagged line now sits in a target no venue registers) — two true
//     things about one edit.
//   - rewriting a tagged recipe line as `-tags=posse_arm2` keeps every arm
//     green, which is the control for reading the flag in both spellings.
//   - changing ci.yml's matrix to `arm: [1, 2]` reds arm 3 alone, and `[1, 2,
//     3, 4]` reds arm 3 alone in the other direction.
//   - deleting `posse_arm3` from test-race.sh's `arm_tag` reds arm 4 alone,
//     and so does changing one of its two loops to `for i in 0 1` — the
//     control that says the array and its index set are two claims.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ─── THE DERIVATION ──────────────────────────────────────────────────────────

// avArms is the arm numbers the tree declares, from armExpr — sorted, and
// required to be 1..n with no gap. The contiguity is not pedantry: arm 1 is
// read here as "an untagged line over internal/posse", so a partition whose
// numbering started at 2 would make every venue's arm-1 line unattributable
// and every pin below green by vacuity.
func avArms(t *testing.T) []int {
	t.Helper()
	seen := map[int]bool{}
	for _, a := range armExpr {
		seen[a] = true
	}
	var out []int
	for a := range seen {
		out = append(out, a)
	}
	sort.Ints(out)
	if len(out) == 0 {
		t.Fatal("armExpr declares no arm: the partition this file holds every venue to is empty, " +
			"so every arm below would pass over nothing")
	}
	for i, a := range out {
		if a != i+1 {
			t.Fatalf("armExpr's arm numbers are %v, which is not 1..%d — arm 1 is read here as the "+
				"DEFAULT build (an untagged line), so a numbering with a gap or a different base "+
				"leaves the venues' arm-1 lines unattributable", out, len(out))
		}
	}
	return out
}

// ─── READING ONE RECIPE LINE ─────────────────────────────────────────────────

// avPosseArmPkg are the package patterns that select internal/posse's own
// build. All three are in live use and each is deliberate: `./...` is what
// `make test`'s and `make vet`'s arm-1 lines run (arm 1 is the tree's default
// build, so its line covers every other package too), `./internal/posse/...`
// is coldexec-check's and tmpdir-check's spelling because only a package-TREE
// pattern takes a suite slot, and the bare `./internal/posse` is what the
// tagged lines and the pkgcensus doors name.
var avPosseArmPkg = map[string]bool{
	"./...":                true,
	"./internal/posse":     true,
	"./internal/posse/...": true,
}

// avTagRe reads a `-tags` flag in every spelling the toolchain accepts:
// `-tags X`, `-tags=X`, and the double-dash forms. scripts/suite-lock.sh:243
// reads the same two shapes for the same reason — a flag spelled with `=` is
// one word, and a reader that only splits on space sees no tag at all.
var avTagRe = regexp.MustCompile(`--?tags[= ]+([^\s]+)`)

var avArmTagRe = regexp.MustCompile(`^posse_arm([0-9]+)$`)

// avArmOfLine reports which internal/posse build arm one recipe line runs.
//
// A line runs an arm when it names a `go test` or `go vet` over a package
// pattern that selects internal/posse. The arm is the `posse_arm<N>` tag it
// passes, or 1 when it passes none — which is the whole reason armtags' arm 3
// exists: arm 1 must BE the default build, or an untagged line runs nothing
// and says ok.
//
// The verb matters. Without it a `go build ./...` or a `gofmt -l ./...` in a
// registered venue's recipe would satisfy that venue's arm-1 line, and the
// forward direction would be green over an arm nothing tests.
func avArmOfLine(line string) (arm int, ok bool, err error) {
	code := strings.TrimSpace(line)
	for _, p := range []string{"@", "-", "+"} {
		code = strings.TrimPrefix(code, p)
	}
	code = strings.TrimSpace(code)
	if code == "" || strings.HasPrefix(code, "#") {
		return 0, false, nil
	}

	var arms []int
	for _, m := range avTagRe.FindAllStringSubmatch(code, -1) {
		for _, w := range strings.FieldsFunc(strings.Trim(m[1], `"'`), func(r rune) bool { return r == ',' }) {
			if n := avArmTagRe.FindStringSubmatch(w); n != nil {
				v, convErr := strconv.Atoi(n[1])
				if convErr != nil {
					return 0, false, fmt.Errorf("unreadable arm tag %q", w)
				}
				arms = append(arms, v)
			}
		}
	}

	verb, pkg := "", ""
	for _, f := range strings.Fields(code) {
		f = strings.Trim(f, `"'`)
		switch f {
		case "test", "vet":
			if verb == "" {
				verb = f
			}
		}
		if avPosseArmPkg[f] {
			pkg = f
		}
	}

	switch {
	case len(arms) > 1:
		return 0, false, fmt.Errorf("names %d arm tags %v in one command — this reader attributes a line to ONE arm, "+
			"so a line running two of them would be counted for whichever was read first", len(arms), arms)
	case len(arms) == 1 && (verb == "" || pkg == ""):
		return 0, false, fmt.Errorf("passes -tags posse_arm%d and this reader cannot attribute it to an arm "+
			"(verb %q, internal/posse package pattern %q) — an arm tag it cannot read is an arm it cannot hold",
			arms[0], verb, pkg)
	case len(arms) == 1:
		return arms[0], true, nil
	case verb != "" && pkg != "":
		return 1, true, nil // the default build
	}
	return 0, false, nil
}

// ─── THE REGISTER ────────────────────────────────────────────────────────────

// avVenue is one place that runs internal/posse's arms. target names the
// Makefile target that runs a given arm: the same name for every arm where
// one recipe runs them all, and a different name per arm where the venue is
// one target per arm (which is what CI and the `-run` doors need).
type avVenue struct {
	name   string
	target func(arm int) string
	why    string
}

func avOneTarget(name string) func(int) string { return func(int) string { return name } }

// avPkgcensusTarget is the pkgcensus door for an arm. Arm 1's is spelled
// without an arm in its name because it is the default build and was the only
// door until the split; the tagged ones have to pass `-tags` or their `-run`
// selects nothing and prints ok.
func avPkgcensusTarget(a int) string {
	if a == 1 {
		return "pkgcensus-check"
	}
	return fmt.Sprintf("pkgcensus-arm%d-check", a)
}

var avVenues = []avVenue{
	{
		"make test", avOneTarget("test"),
		"the house command and the whole suite — an arm missing here is a third of the package that " +
			"`make test` exits 0 without having compiled",
	},
	{
		"make vet", avOneTarget("vet"),
		"`go vet ./...` builds the DEFAULT build of every package, so with the suite split by tag it " +
			"sees arm 1 and nothing else (ranger-base-qp1hm)",
	},
	{
		"make test-arm<N>", armTargetName,
		"one target per arm, for ci.yml, which runs them as separate jobs — an arm with no target is " +
			"an arm with no job",
	},
	{
		"make pkgcensus[-arm<N>]-check", avPkgcensusTarget,
		"the one-package censuses over internal/posse's own sources: a door takes the arm's tag or its " +
			"`-run` selects nothing, and a `go test -run` that matches no test exits 0",
	},
	{
		"make coldexec-check", avOneTarget("coldexec-check"),
		"the cold-first-exec venue (ADR 0085). Three of its five arms are internal/posse's; the other " +
			"two are cmd/posse and internal/treepins, which coldexecarms_qa_test.go derives from the " +
			"plant census",
	},
	{
		"make tmpdir-check", avOneTarget("tmpdir-check"),
		"the off-/tmp venue (ranger-base-m33n5) — the bead's own door, and the one whose three " +
			"hand-written lines this file was filed from",
	},
}

// ─── READING THE MAKEFILE ────────────────────────────────────────────────────

// avTargetRe matches a Makefile target line. The `(?:[^=]|$)` after the colon
// is what keeps a variable assignment out: `X := ...` and `X ?= ...` put an
// `=` immediately after the separator, and a recipe-less `.PHONY:` line ends
// there.
var avTargetRe = regexp.MustCompile(`^([A-Za-z0-9._$(){}/-]+)[ \t]*:(?:[^=]|$)`)

// avRecipes is every Makefile target's recipe lines, keyed by target. A
// recipe is the tab-indented run after a target line; makeRecipe does the
// same for ONE named target, and this is the sweep the staleness arm needs.
func avRecipes(mk string) map[string][]string {
	out := map[string][]string{}
	cur := ""
	for _, line := range strings.Split(mk, "\n") {
		if strings.HasPrefix(line, "\t") {
			if cur != "" {
				out[cur] = append(out[cur], strings.TrimPrefix(line, "\t"))
			}
			continue
		}
		cur = ""
		if m := avTargetRe.FindStringSubmatch(line); m != nil {
			cur = m[1]
		}
	}
	return out
}

// ─── ARM 1 ───────────────────────────────────────────────────────────────────

// ARM 1 — every arm the tree declares has a line in every registered venue.
//
// The forward direction, and the bead's own defect: an arm that exists, is
// compiled, is tagged, passes armtags' census, and is RUN by nobody.
func TestQAEveryPosseSuiteArmHasALineInEveryArmListVenue(t *testing.T) {
	t.Parallel()
	arms := avArms(t)
	recipes := avRecipes(makefileText(t))

	for _, v := range avVenues {
		for _, a := range arms {
			tgt := v.target(a)
			rec, ok := recipes[tgt]
			if !ok {
				t.Errorf("%s: the Makefile has no target %q, so arm %d is in the tree and this venue "+
					"never runs it (%s)", v.name, tgt, a, v.why)
				continue
			}
			found := false
			for _, line := range rec {
				n, ok, err := avArmOfLine(line)
				if err != nil {
					t.Errorf("%s: recipe line %q: %v", v.name, line, err)
					continue
				}
				if ok && n == a {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: target %q runs no line for arm %d (%s). armExpr declares %d arms %v; "+
					"this venue's recipe is:\n\t%s",
					v.name, tgt, a, v.why, len(arms), arms, strings.Join(rec, "\n\t"))
			}
		}
	}
	t.Logf("%d arms %v × %d venues held", len(arms), arms, len(avVenues))
}

// ─── ARM 2 ───────────────────────────────────────────────────────────────────

// ARM 2 — the register is not stale, and no venue runs an arm the tree does
// not declare.
//
// The backward direction, swept over EVERY recipe in the Makefile rather than
// over the register's own targets. A hand-kept list of venues goes blind in
// one direction only: it cannot name the venue nobody added to it, and that
// venue is then unheld while arm 1 stays green. armDoorVars had exactly this
// shape and went a door short for as long as that door existed
// (ranger-base-na4rm); the fix there was to derive membership, and the
// derivation available here is the tag itself — a `-tags posse_arm*` line IS
// a declaration that something runs an arm, wherever it is typed.
func TestQAEveryMakefileArmLineIsInARegisteredVenue(t *testing.T) {
	t.Parallel()
	arms := avArms(t)
	declared := map[int]bool{}
	for _, a := range arms {
		declared[a] = true
	}
	mk := makefileText(t)
	recipes := avRecipes(mk)

	// Which target each venue owns for each arm, so a tagged line can be
	// attributed back.
	owner := map[string]map[int]string{} // target → arm → venue name
	for _, v := range avVenues {
		for _, a := range arms {
			tgt := v.target(a)
			if owner[tgt] == nil {
				owner[tgt] = map[int]string{}
			}
			owner[tgt][a] = v.name
		}
	}

	// cxaSorted so a finding reads the same twice — a map sweep that reports
	// in range order prints a different error list on every run.
	for _, tgt := range cxaSorted(recipes) {
		for _, line := range recipes[tgt] {
			if !strings.Contains(line, "posse_arm") {
				continue
			}
			n, ok, err := avArmOfLine(line)
			if err != nil {
				t.Errorf("target %q: recipe line %q: %v", tgt, line, err)
				continue
			}
			if !ok {
				continue
			}
			if !declared[n] {
				t.Errorf("target %q runs `-tags posse_arm%d` and armExpr declares only %v — the venue "+
					"runs an arm no file in the tree carries, so that line compiles the DEFAULT build "+
					"under another name and tests the same third twice:\n\t%s", tgt, n, arms, line)
			}
			if owner[tgt][n] == "" {
				t.Errorf("target %q runs arm %d and no venue in avVenues names that target for that arm — "+
					"an unregistered venue is unheld by arm 1, which is how a hand-kept list of venues "+
					"goes blind. Register it (or say on the row why it is not a venue):\n\t%s",
					tgt, n, line)
			}
		}
	}

	// And no arm tag outside a recipe at all. A Makefile VARIABLE holding
	// `-tags posse_arm2` would be expanded into some target's recipe by make
	// and read by nothing here, so the register could not attribute it.
	for i, line := range strings.Split(mk, "\n") {
		if strings.HasPrefix(line, "\t") {
			continue
		}
		code := strings.TrimSpace(line)
		if strings.HasPrefix(code, "#") || !strings.Contains(code, "posse_arm") {
			continue
		}
		if avTagRe.MatchString(code) {
			t.Errorf("Makefile:%d passes an arm tag outside any recipe — make would expand it into some "+
				"target's command line and this register, which reads recipes, could not attribute it to "+
				"a venue:\n\t%s", i+1, code)
		}
	}
}

// ─── ARM 3 ───────────────────────────────────────────────────────────────────

var avCIArmMatrixRe = regexp.MustCompile(`(?m)^[\t ]*arm:[\t ]*\[([^\]]*)\]`)

// ARM 3 — ci.yml's `arm:` matrix is the partition.
//
// The six jobs run `make test-arm${{ matrix.arm }}` and `go vet -tags
// posse_arm${{ matrix.arm }}`, so unlike every other venue here CI's arm list
// is DERIVED from one place — and that place is this matrix line. Arm 1 of
// this file holds the `test-arm<N>` targets; nothing held the list of N.
//
// Both directions, because each is its own mistake: a declared arm with no
// matrix entry is an arm no job builds, vets or runs, and a matrix entry with
// no declared arm is six jobs spending a runner on `make test-arm4`, which
// make would refuse outright.
func TestQACIArmMatrixIsTheSuiteArmPartition(t *testing.T) {
	t.Parallel()
	arms := avArms(t)
	b, err := os.ReadFile(filepath.Join(".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read ci.yml: %v", err)
	}
	m := avCIArmMatrixRe.FindStringSubmatch(string(b))
	if m == nil {
		t.Fatal("ci.yml has no `arm: [...]` matrix line: the arms CI runs cannot be held to a list this " +
			"pin cannot read, and an unheld matrix is how a declared arm gets no job")
	}
	var got []int
	for _, f := range strings.Split(m[1], ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		n, convErr := strconv.Atoi(f)
		if convErr != nil {
			t.Errorf("ci.yml's matrix line %q names %q, which is not an arm number", m[0], f)
			continue
		}
		got = append(got, n)
	}
	sort.Ints(got)

	has := func(set []int, a int) bool {
		for _, v := range set {
			if v == a {
				return true
			}
		}
		return false
	}
	for _, a := range arms {
		if !has(got, a) {
			t.Errorf("armExpr declares arm %d and ci.yml's matrix is %v — no CI job builds, vets or runs "+
				"that arm, so a third of internal/posse's tests would reach main having run on one seat "+
				"at most", a, got)
		}
	}
	for _, a := range got {
		if !has(arms, a) {
			t.Errorf("ci.yml's matrix runs arm %d and armExpr declares only %v — six jobs would call "+
				"`make test-arm%d`, which does not exist", a, arms, a)
		}
	}
}

// ─── ARM 4 ───────────────────────────────────────────────────────────────────

const avRaceScript = "scripts/test-race.sh"

var (
	avRaceArmTagRe = regexp.MustCompile(`(?m)^arm_tag=\(([^)]*)\)`)
	avRaceLoopRe   = regexp.MustCompile(`(?m)^[\t ]*for i in ([^;]*); do`)
)

// ARM 4 — test-race.sh's `arm_tag` is the partition, and the loops that index
// it enumerate every element.
//
// The on-demand `-race` arm runs internal/posse's concurrency-carrying tests
// as the three tag-aware commands it actually is (ranger-base-nhc23): one
// command compiled arm 1 and the shared files only, and 86 of the 106 names
// in its regex matched nothing while it printed ok. `arm_tag` is that fix's
// hand-written list, and the two `for i in 0 1 2` loops its hand-written
// index set.
//
// This venue is the best defended of the eight and still not defended enough.
// The script's census fails loudly when a CANDIDATE test is compiled into no
// arm — "a fourth tag" is named in its own die message — so a fourth arm with
// a race-prone test in it reds there. With none in it, the array is short, the
// loops are short, and the arm is simply not raced.
func TestQATheRaceArmTagListIsTheSuiteArmPartition(t *testing.T) {
	t.Parallel()
	arms := avArms(t)
	b, err := os.ReadFile(avRaceScript)
	if err != nil {
		t.Fatalf("read %s: %v", avRaceScript, err)
	}
	src := string(b)

	m := avRaceArmTagRe.FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("%s has no `arm_tag=(...)` line: the -race arm's per-arm tags cannot be held to a list "+
			"this pin cannot read", avRaceScript)
	}
	var got []string
	for _, w := range strings.Fields(m[1]) {
		got = append(got, strings.Trim(w, `"'`))
	}
	if len(got) != len(arms) {
		t.Errorf("%s's arm_tag is %q — %d entries for %d declared arms %v. Every arm needs its tag here, "+
			"and a fourth arm also needs the `for i in` loops below, the per-arm summary printf, and "+
			"racearm_qa_test.go's `want 3` on the census output",
			avRaceScript, m[1], len(got), len(arms), arms)
	}
	for i, a := range arms {
		if i >= len(got) {
			break
		}
		want := ""
		if a != 1 {
			want = armTagFor(a)
		}
		if got[i] != want {
			t.Errorf("%s's arm_tag[%d] is %q, want %q — the array is indexed by arm-1, so entry 0 is the "+
				"DEFAULT build (the empty tag) and entry n-1 is arm n's tag",
				avRaceScript, i, got[i], want)
		}
	}

	// And the loops over it. An array with every arm's tag in it is still
	// short by an arm if the loop that reads it stops at the old last index.
	var want []string
	for i := range arms {
		want = append(want, strconv.Itoa(i))
	}
	wantList := strings.Join(want, " ")
	loops := avRaceLoopRe.FindAllStringSubmatch(src, -1)
	if len(loops) == 0 {
		t.Errorf("%s has no `for i in ...; do` loop over arm_tag — the census and the three arms are both "+
			"such a loop, so a reader that finds neither holds neither", avRaceScript)
	}
	for _, l := range loops {
		if got := strings.Join(strings.Fields(l[1]), " "); got != wantList {
			t.Errorf("%s loops `for i in %s` over an arm_tag of %d arms %v — want the indices %q, or the "+
				"loop skips an arm whose tag is right there in the array",
				avRaceScript, got, len(arms), arms, wantList)
		}
	}
}

// ─── ARM 5 ───────────────────────────────────────────────────────────────────

// ARM 5 — the reader can say no.
//
// Arms 1 through 4 are all avArmOfLine, and its two ways of going wrong are
// opposite and both silent. A reader that answers "arm 1" to any line leaves
// arm 1 satisfied by a `go build`, a `gofmt` or a comment; a reader that
// answers nothing leaves arm 2's sweep finding no tagged line to attribute
// and every venue's arm-1 line missing — which would red arm 1 rather than
// silence it, so it is the less dangerous half, and still a lie about why.
func TestQATheArmLineReaderRefusesTheWrongShapes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		line string
		arm  int
		ok   bool
		bad  bool
	}{
		// what the live venues actually type
		{"test's arm 1", "scripts/test-times.sh $(GOBIN) test -timeout 25m ./...", 1, true, false},
		{"test's arm 2", "scripts/test-times.sh $(GOBIN) test -timeout 25m -tags posse_arm2 ./internal/posse", 2, true, false},
		{"vet's arm 1", "$(GOBIN) vet ./...", 1, true, false},
		{"vet's arm 3", "$(GOBIN) vet -tags posse_arm3 ./internal/posse", 3, true, false},
		{"tmpdir-check's arm 1", "$(TMPDIR_CHECK_TMP) scripts/test-times.sh $(GOBIN) test ./internal/posse/... -timeout 25m -count=1", 1, true, false},
		{"a pkgcensus door", "$(GOBIN) test -tags posse_arm2 ./internal/posse -timeout 15m -count=1 -run '^($(QA_PKGCENSUS_ARM2_PINS))$$'", 2, true, false},
		{"a coldexec arm through the supervisor", "$(COLDEXEC_TMP) ./$(COLDEXEC_BIN) -- scripts/test-times.sh $(GOBIN) test -tags posse_arm3 ./internal/posse -timeout 60m", 3, true, false},
		{"the = spelling", "$(GOBIN) test -tags=posse_arm2 ./internal/posse", 2, true, false},
		{"a silenced line", "@$(GOBIN) test -tags posse_arm2 ./internal/posse", 2, true, false},

		// not an arm of internal/posse
		{"another package", "scripts/test-times.sh $(GOBIN) test ./internal/treepins -timeout 15m -count=1 -run '^TestQAEverySuiteArmTypeChecks$$'", 0, false, false},
		{"coldexec's cmd/posse arm", "$(GOBIN) test ./cmd/posse/... -timeout 60m -count=1", 0, false, false},
		{"a build is not a test", "$(GOBIN) build -o $@ ./...", 0, false, false},
		{"a formatter is not a test", "gofmt -l ./...", 0, false, false},
		{"a comment in a recipe", "# $(GOBIN) test -tags posse_arm2 ./internal/posse", 0, false, false},
		{"an empty line", "", 0, false, false},
		{"an unrelated tag", "$(GOBIN) test -tags integration ./internal/posse", 1, true, false},

		// shapes the reader must refuse rather than guess at
		{"two arms in one command", "$(GOBIN) test -tags posse_arm2 -tags posse_arm3 ./internal/posse", 0, false, true},
		{"an arm tag over no known package", "$(GOBIN) test -tags posse_arm2 ./internal/treepins", 0, false, true},
		{"an arm tag with no verb", "$(GOBIN) build -tags posse_arm2 ./internal/posse", 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			arm, ok, err := avArmOfLine(tc.line)
			if tc.bad {
				if err == nil {
					t.Errorf("avArmOfLine(%q) = (%d, %v, nil), want an error: a line this reader cannot "+
						"attribute must be refused, not read as arm %d", tc.line, arm, ok, arm)
				}
				return
			}
			if err != nil {
				t.Fatalf("avArmOfLine(%q): unexpected error: %v", tc.line, err)
			}
			if ok != tc.ok || arm != tc.arm {
				t.Errorf("avArmOfLine(%q) = (%d, %v), want (%d, %v)", tc.line, arm, ok, tc.arm, tc.ok)
			}
		})
	}
}

// And the Makefile reader, whose own failure is to attribute a recipe line to
// the wrong target — or to read a variable assignment as a target, which
// would put `TMPDIR_CHECK_TMP`'s shell in some recipe and give arm 2 a line
// it cannot place.
func TestQATheMakefileRecipeReaderRefusesTheWrongShapes(t *testing.T) {
	t.Parallel()
	const mk = "# a comment: not a target\n" +
		"VAR := value: with a colon\n" +
		"OTHER ?= x\n" +
		"one: dep\n" +
		"\tline one\n" +
		"\tline two\n" +
		"\n" +
		".PHONY: one two\n" +
		"two:\n" +
		"\tline three\n" +
		"$(BIN): $(wildcard cmd/x/*.go)\n" +
		"\tbuild it\n"
	got := avRecipes(mk)
	for target, want := range map[string][]string{
		"one":    {"line one", "line two"},
		"two":    {"line three"},
		"$(BIN)": {"build it"},
	} {
		if g := strings.Join(got[target], "|"); g != strings.Join(want, "|") {
			t.Errorf("avRecipes: target %q has recipe %q, want %q", target, g, strings.Join(want, "|"))
		}
	}
	for _, notATarget := range []string{"VAR", "OTHER", "# a comment", ".PHONY"} {
		if len(got[notATarget]) != 0 {
			t.Errorf("avRecipes: %q is not a target with a recipe and was read with %q",
				notATarget, got[notATarget])
		}
	}
}
