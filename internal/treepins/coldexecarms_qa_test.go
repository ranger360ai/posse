package treepins

// QA pins for ranger-base-s64a5 — `make coldexec-check`'s arms are DERIVED
// from the plant census, not remembered.
//
// THE DOOR. coldexec-check (Makefile, ADR 0085 D6) runs a suite inside
// cmd/coldexec, the seccomp execve supervisor, so the FIRST exec of every
// freshly planted executable is held for COLDEXEC_DELAY. That is the whole of
// ADR 0085's move: the class is "a test whose verdict changes when the first
// exec of a planted file is slow", and it is decided by RUNNING the test with
// that cost injected. The door is therefore only as wide as its arm list, and
// the arm list was five hand-written recipe lines.
//
// THE DEFECT. The set those arms SHOULD equal is derivable and was not
// derived. A package whose tests can flake on darwin's cold first exec is a
// package that PLANTS an executable, which is a package with a
// WriteExecutable call site — the one writer every exec-bit write in this
// tree goes through, which internal/posse's
// TestQATreeGoFilesWriteExecutablesUnderTheForkLock establishes separately
// (make execwrite-check, ranger-base-o6oj4 widened by ranger-base-6cznr).
// Nothing compared the two. execwrite-check's own floors are a per-directory
// .go FILE count — a different claim, and one that is satisfied by a package
// nobody injects into — and the Makefile's coldexec block states the coupling
// in prose only. A fourth package that starts planting tomorrow gets no arm,
// no red, and a door that is green over tests it never ran.
//
// MEASURED 2026-11-11, this bead, by the census below: 331 plant call sites
// in exactly three packages — internal/posse 261, internal/treepins 53,
// cmd/posse 17. (The bead's own description says 345 in those same three,
// which was `grep -rn 'WriteExecutable('`: 331 calls, the two
// WriteExecutable DECLARATIONS — internal/posse/execwrite.go and
// internal/treepins/execwrite_test.go, the package-local copy internal/posse's
// init.go import cycle forces — and twelve quoted mentions, six in
// execwritesiblings_qa_test.go's positive-witness table and six in
// execwritegates_qa_test.go's. This census parses, so a call written inside a
// string literal is not a call site and a `func WriteExecutable` is not one
// either.)
//
// THE SHAPE, and why it is two pins and a register rather than one pin.
// (`arm` below means what it means in the Makefile and in cxaArm — one
// coldexec-check RECIPE LINE. The pins are named, not numbered, because this
// file is about arms and a second meaning for the word would be unreadable.)
//
// THE DERIVATION is TestQAEveryPlantingPackageHasAColdexecArm: every planting
// package has an arm reaching it, and no arm covers only packages that plant
// nothing. Both directions, because an arm over a package with no plants
// injects nothing and is a recipe line a reader will trust.
//
// The register (cxaUncoveredPlanters) was the live gap, and it was live
// because of WHEN this bead landed: coldexec-check had three arms, all
// internal/posse, and extending it to cmd/posse and internal/treepins was
// ranger-base-in8dr's own deliverable — after ADR 0085 D6's amendment
// (ranger-base-dacil) one Makefile line per package, inside the supervisor.
// So this pin could not demand those two arms without reddening the suite
// over work another bead held, and it must not go quiet about them either.
// A row per package, naming the bead that will cover it, is the honest third
// answer: the derivation reds for a FOURTH package immediately, and
// TestQAColdexecArmRegisterIsNotStale reds the moment a registered row stops
// being true.
//
// THE REGISTER'S PARTNER IS A COUPLING, not a courtesy. A waiver that outlives its cause is
// how this door would go back to being remembered — the register is the one
// route into arm 1 that does not go through the census, which is exactly the
// parking spot treewidedoor_qa_test.go's twdDoorHolders keeps warning about.
// So a registered package that has stopped planting, or that HAS an arm now,
// is a red with a one-line fix: delete the row.
//
// ranger-base-in8dr's seat did NOT see that red, which is the one thing this
// coupling could not arrange: that bead's worktree was based before this file
// existed, so the arms and the rows they clear were never in one tree any seat
// ran. The red landed on main instead — corpus-check, so tree-check, so
// `make test` exited 2 for every seat on the box until ranger-base-r5tnw
// deleted the rows. The coupling fired and named its own fix; what nothing
// reads between two concurrent closes is the MERGED tree, filed to the
// architecture lane as ranger-base-oe9ja.
//
// THE CONTROL is TestQAColdexecArmCensusCanStillSayNo. The two pins above
// both rest on two readers — a matcher over Go source and a reader over a
// Makefile recipe line — and the cheapest way for either to go green is to
// stop matching. A census that counts no call sites reports every package as
// a non-planter and the derivation passes with nothing to say; so does a
// recipe reader that finds no arms, in the one direction that is silent. The
// control asks both readers, on synthetic input, for both answers. Its own
// failures are not a floor and no unrelated bead can red it, so it is
// deliberately not behind a door (the reason three of the corpus-check rows
// next door give).
//
// THE FLOORS are the other half of that. A census is positively checked by
// the control and floored here: fewer than cxaSiteFloor call sites, or fewer
// than cxaPlanterFloor packages holding them, and the walk is not reading
// this tree, so the absence the derivation asserts measures nothing.
//
// Its door is `make corpus-check` — a census over this repo's own .go
// sources, which is that door's subject exactly, and no new door for the
// sentence every other corpus-check row gives. Not execwrite-check: that
// door runs internal/posse and its subject is whether a write is routed
// through WriteExecutable at all, which is not the question here.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// cxaTarget is the Makefile door whose arms these pins derive.
const cxaTarget = "coldexec-check"

// cxaPlantWriter is the ONE executable writer in this tree, and the reason a
// call site is a plant: execwrite-check holds every exec-bit write to it, so
// its call sites ARE the tree's planted executables. A second writer would be
// a planting site this census cannot see — which is why that door, not this
// one, is what keeps this census complete.
const cxaPlantWriter = "WriteExecutable"

// cxaSupervisor is the Makefile variable holding the injector each arm runs
// inside. An arm line that does not name it delays nothing, so it is not an
// arm — it is a plain `go test` wearing the door's name.
const cxaSupervisor = "$(COLDEXEC_BIN)"

// cxaRoom and cxaTemp are the other two Makefile variables every arm line
// carries, and they are named here rather than spelled inline so the pin
// below and the recipe cannot drift apart silently (ranger-base-w5z7u).
//
// cxaRoom is the refusal: the arm reads the room its tests will write into
// and refuses below a floor, rather than spending an hour and reporting the
// BOX as the class. cxaTemp is the arm's own temp directory, nested under
// $TMPDIR and cleared on the way out, so an arm that is killed does not add
// to the residue that fills the room for the next one.
const (
	cxaRoom = "$(COLDEXEC_ROOM)"
	cxaTemp = "$(COLDEXEC_TMP)"
)

// What those two variables have to still SAY, because a pin that only checks
// the arms name them goes green over `COLDEXEC_ROOM :=` emptied to nothing.
// One substring each, chosen as the part that carries the meaning: the flag
// that makes the door refuse, and the two things the prelude has to do that
// are a word rather than a shape — hand the nested directory on, and clear it
// however the arm ends.
//
// COLDEXEC_TMP's third property is WHERE the directory is, and it is held by
// cxaTempRoot below and NOT by a word here (ranger-base-nxus6). "mktemp -d"
// used to be a row and is deliberately not one: the root check cannot pass
// without finding that token, so a value that has stopped running mktemp
// would red in two places and one dropped guard would read as two findings.
var cxaVarMustHold = map[string][]string{
	"COLDEXEC_ROOM":         {"-min-temp-mb", "$(COLDEXEC_TMP_FLOOR_MB)"},
	"COLDEXEC_TMP":          {"TMPDIR=", "trap "},
	"COLDEXEC_TMP_FLOOR_MB": {},
}

// cxaTempRoot is the root COLDEXEC_TMP's directory has to be nested under,
// and it is the whole of the property cxaTemp's comment claims: $TMPDIR when
// the seat has one, a bare /tmp when it has not, and nothing else.
//
// THE DEFECT IT CLOSES (ranger-base-nxus6, from ranger-base-2smyx's verify of
// ranger-base-w5z7u). The three substrings this pin used to require —
// "mktemp -d", "TMPDIR=", "trap " — are all satisfied by a value rooted
// anywhere at all, so the pin was green against the one mutant it most needs
// to catch. MEASURED 2026-11-11:
//
//	sed -i 's|\$\${TMPDIR:-/tmp}|/var/tmp|' Makefile   # the COLDEXEC_TMP line
//	go test ./internal/treepins \
//	  -run TestQAEveryColdexecArmReadsTheRoomAndTakesItsOwnTempDir -count=1
//	# ok -- and go test ./cmd/coldexec -run 'Room|Quota|Temp' is ok too
//
// and that mutant is exactly the edit ranger-base-w5z7u argues against. A
// TMPDIR off /tmp buys the room and COSTS A CLASS: four pins whose premise is
// the unconditional bare /tmp in every rendered seatbelt go red
// (ranger-base-u8xk4 holds the defect, ranger-base-akk9f the surfaces that
// abbreviate $HOME). This pin green while four unrelated pins red is the
// confusion the close exists to prevent, so the root is the one part of this
// variable that is read as a SHAPE and not as a word.
const cxaTempRoot = "$${TMPDIR:-/tmp}"

// The floors under the census. 331 call sites in three packages on
// 2026-11-11; both are deliberately loose, because their job is to catch a
// walk that read nothing, not to track the count.
const (
	cxaSiteFloor    = 250
	cxaPlanterFloor = 3
)

// cxaUncoveredPlanters are the planting packages coldexec-check does not
// cover yet, each naming the bead that will cover it.
// TestQAEveryPlantingPackageHasAColdexecArm exempts these from needing an
// arm; TestQAColdexecArmRegisterIsNotStale reds the moment a row stops being
// true, so a row cannot outlive its cause.
//
// EMPTY since ranger-base-r5tnw, and that is the register working rather than
// the register being unused: ranger-base-in8dr landed the arms its two rows
// named, the stale-register pin red naming both, and the rows went. Every
// planting package has an arm now, so the derivation exempts nothing and
// holds all three of them on its own.
//
// TO ADD A ROW: only for a package that plants and whose arm belongs to
// another bead — key the package directory, and let the value name that bead.
// TO CLEAR A ROW: add the arm to coldexec-check's recipe and delete the row;
// the stale-register pin reds, names the row and says so.
var cxaUncoveredPlanters = map[string]string{}

// cxaPlantSites returns the number of plant call sites per package directory,
// slash-separated and relative to the repository root. TestMain chdirs this
// binary to that root, so the walk is rooted at a plain `.`.
//
// Test files are IN, and they are nearly all of it — a plant is a fixture and
// fixtures live in tests. testdata, vendor, node_modules and dotted
// directories are out: a .go file there is compiled into no package, so a
// call site in one is not a call site at all.
func cxaPlantSites(t *testing.T) (map[string]int, int) {
	t.Helper()
	per := map[string]int{}
	total := 0
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			n := d.Name()
			if p != "." && (strings.HasPrefix(n, ".") || n == "vendor" || n == "node_modules" || n == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(p)
		if !strings.HasSuffix(rel, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("parse %s: %v", rel, perr)
		}
		if n := cxaPlantCalls(f); n > 0 {
			per[path.Dir(rel)] += n
			total += n
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree for plant call sites: %v", err)
	}
	return per, total
}

// cxaPlantCalls counts calls to the plant writer in one parsed file — bare
// (`WriteExecutable(...)`, inside internal/posse and internal/treepins, which
// each hold a copy) and qualified (`posse.WriteExecutable(...)`, everywhere
// else). A `func WriteExecutable` declaration is not a call and neither is the
// name inside a string literal; both are why this reads an AST and not a grep.
func cxaPlantCalls(f *ast.File) int {
	n := 0
	ast.Inspect(f, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			if fn.Name == cxaPlantWriter {
				n++
			}
		case *ast.SelectorExpr:
			if fn.Sel.Name == cxaPlantWriter {
				n++
			}
		}
		return true
	})
	return n
}

// cxaArm is one recipe line of the door, read.
type cxaArm struct {
	line       string
	pattern    string // the `./…` package pattern as the recipe spells it
	dir        string // that pattern's directory, slash-separated and `./` stripped
	tree       bool   // the `/...` form: the directory AND every package under it
	supervised bool   // the line runs that pattern inside cxaSupervisor
}

// covers reports whether this arm runs a package directory, read the way `go
// test` reads its argument: `./internal/posse` is that package alone and
// `./internal/posse/...` is it and everything below it.
//
// A PREFIX AND NOT AN EQUALITY, and the control below is why. The reader
// this replaced stripped `/...` and compared directories, which made
// `./internal/...` — one pattern over both planting packages under internal/
// — read as an arm over a directory called `internal` that plants nothing:
// two false findings, in both directions at once, over a recipe line that is
// correct. The two spellings are not distinguishable without resolving the
// path, and they do not need to be: a prefix is what the pattern MEANS.
func (a cxaArm) covers(pkg string) bool {
	if !a.tree {
		return pkg == a.dir
	}
	return a.dir == "" || pkg == a.dir || strings.HasPrefix(pkg, a.dir+"/")
}

// cxaReadArm reads one recipe line. It reports ok=false for a line that runs
// no `go test` at all — such a line is not an arm and is not this pin's
// business — and an error for a line that DOES run one and names no package
// pattern at all, because that is an arm whose coverage cannot be read and
// skipping it silently is the whole defect this pin exists for.
func cxaReadArm(line string) (cxaArm, bool, error) {
	fields := strings.Fields(line)
	at := -1
	for i, f := range fields {
		if f != "test" || i == 0 {
			continue
		}
		if prev := fields[i-1]; prev == "$(GOBIN)" || prev == "go" {
			at = i
			break
		}
	}
	if at < 0 {
		return cxaArm{}, false, nil
	}
	arm := cxaArm{line: line, supervised: strings.Contains(line, cxaSupervisor)}
	for _, f := range fields[at+1:] {
		if !strings.HasPrefix(f, "./") {
			continue
		}
		arm.pattern = f
		rest := strings.TrimPrefix(f, "./")
		if rest == "..." {
			arm.dir, arm.tree = "", true
			return arm, true, nil
		}
		if d, ok := strings.CutSuffix(rest, "/..."); ok {
			arm.dir, arm.tree = d, true
			return arm, true, nil
		}
		arm.dir = rest
		return arm, true, nil
	}
	return cxaArm{}, true, &cxaUnreadableArm{line: line}
}

// cxaUnreadableArm is an arm line whose package coverage cannot be read.
type cxaUnreadableArm struct{ line string }

func (e *cxaUnreadableArm) Error() string {
	return "`make " + cxaTarget + "` has a `go test` recipe line that names no `./…` package pattern:\n  " + e.line +
		"\nAn arm of this door has to say which packages it runs, because that is what the census below is compared against. Spell the pattern — `./<dir>`, or `./<dir>/...` for the package-tree form that takes a suite slot — and an arm nothing can read becomes an arm something can derive."
}

// cxaArms reads the door's arms out of the Makefile.
func cxaArms(t *testing.T, makefile string) []cxaArm {
	t.Helper()
	lines := mkRecipeCode(makeRecipe(makefile, cxaTarget))
	if len(lines) == 0 {
		t.Fatalf("the Makefile has no `%s` recipe — the door these pins derive is gone, and with it every arm injecting the cold first exec (ADR 0085 D6)", cxaTarget)
	}
	var out []cxaArm
	for _, line := range lines {
		arm, runs, err := cxaReadArm(line)
		if !runs {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, arm)
	}
	if len(out) == 0 {
		t.Fatalf("the `%s` recipe runs no `go test` at all:\n  %s\nThe door exists to run a suite with the first exec of every planted executable held, so a recipe with no arm in it is a green door over nothing.", cxaTarget, strings.Join(lines, "\n  "))
	}
	return out
}

// cxaSorted returns a map's keys, sorted, so a finding reads the same twice.
func cxaSorted[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestQAEveryPlantingPackageHasAColdexecArm is the derivation: coldexec-check
// covers every package that plants an executable, and nothing else.
func TestQAEveryPlantingPackageHasAColdexecArm(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	arms := cxaArms(t, string(b))

	per, total := cxaPlantSites(t)
	if total < cxaSiteFloor {
		t.Fatalf("the census found %d plant call sites in the tree, want at least %d — %s is the one executable writer here (make execwrite-check), and a count this low means this walk is not reading this tree, so the coverage below measures nothing", total, cxaSiteFloor, cxaPlantWriter)
	}
	if len(per) < cxaPlanterFloor {
		t.Fatalf("the census found plant call sites in %d packages (%v), want at least %d — same reason as the site floor above", len(per), cxaSorted(per), cxaPlanterFloor)
	}

	var covering []cxaArm
	for _, arm := range arms {
		if !arm.supervised {
			t.Errorf("`make %s` runs %s outside %s:\n  %s\nThat line delays no exec, so every planted fixture it reaches execs warm and the arm reports a green suite over exactly the cost the door exists to charge (ADR 0085 D6: POSSE_COLD_EXEC_DELAY reaches the supervisor alone).", cxaTarget, arm.pattern, cxaSupervisor, arm.line)
			continue
		}
		covering = append(covering, arm)
	}
	patterns := func() []string {
		var out []string
		for _, a := range covering {
			if !slices.Contains(out, a.pattern) {
				out = append(out, a.pattern)
			}
		}
		return out
	}()

	for _, pkg := range cxaSorted(per) {
		if slices.ContainsFunc(covering, func(a cxaArm) bool { return a.covers(pkg) }) {
			continue
		}
		if why, waived := cxaUncoveredPlanters[pkg]; waived {
			t.Logf("%s has no %s arm, registered (%s call sites: %d): %s", pkg, cxaTarget, cxaPlantWriter, per[pkg], why)
			continue
		}
		t.Errorf("%s plants an executable and `make %s` has no arm reaching it.\n"+
			"  %s call sites in %s: %d\n"+
			"  the door's patterns: %v\n"+
			"  the packages that plant: %v\n"+
			"A package that plants an executable and execs it under a deadline is in ADR 0085's class, and the only thing that decides whether a test of it flakes on darwin's cold first exec is running it with that cost injected. Add an arm — one recipe line, `./%s` inside %s — or, if that work belongs to another bead, add a row to cxaUncoveredPlanters naming it.",
			pkg, cxaTarget, cxaPlantWriter, pkg, per[pkg], patterns, cxaSorted(per), pkg, cxaSupervisor)
	}

	for _, arm := range covering {
		if slices.ContainsFunc(cxaSorted(per), arm.covers) {
			continue
		}
		t.Errorf("`make %s` runs an arm over %s, and the census finds no %s call site under it:\n  %s\n"+
			"  the packages that plant: %v\n"+
			"The arm costs a suite slot and up to COLDEXEC_DELAY per planted file to inject a cost into packages that plant nothing, and a reader counting the door's reach from its recipe counts this line. Either what it covers stopped planting and the arm goes, or the census stopped seeing it — and the second is the one worth checking first.",
			cxaTarget, arm.pattern, cxaPlantWriter, arm.line, cxaSorted(per))
	}
}

// TestQAColdexecArmRegisterIsNotStale holds cxaUncoveredPlanters to being
// true. It is the one route into the arm above that does not go through the
// census, so a row that outlives its cause is this door going back to a
// remembered list — with the register's own silence in front of it.
func TestQAColdexecArmRegisterIsNotStale(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	var covering []cxaArm
	for _, arm := range cxaArms(t, string(b)) {
		if arm.supervised {
			covering = append(covering, arm)
		}
	}
	per, _ := cxaPlantSites(t)

	for _, pkg := range cxaSorted(cxaUncoveredPlanters) {
		if i := slices.IndexFunc(covering, func(a cxaArm) bool { return a.covers(pkg) }); i >= 0 {
			t.Errorf("cxaUncoveredPlanters still waives %s, and `make %s` has an arm reaching it now:\n  %s\nThe row said that package has no arm; it has one. Delete the row — it is the reason the derivation arm exempts %s, so while it sits there a REGRESSION in that arm is silent.\n  the row reads: %s", pkg, cxaTarget, covering[i].line, pkg, cxaUncoveredPlanters[pkg])
			continue
		}
		if per[pkg] == 0 {
			t.Errorf("cxaUncoveredPlanters waives %s, and the census finds no %s call site there — that package plants nothing, so there is nothing to waive and nothing for %s to inject. Delete the row.\n  the row reads: %s\n  the packages that plant: %v", pkg, cxaPlantWriter, cxaTarget, cxaUncoveredPlanters[pkg], cxaSorted(per))
		}
	}
}

// TestQAEveryColdexecArmReadsTheRoomAndTakesItsOwnTempDir holds the other
// half of an arm: not WHICH packages it runs, which the derivation above
// holds, but whether it can honestly run them at all (ranger-base-w5z7u).
//
// THE DEFECT. The five arms were run back to back on 2026-11-11 from a seat
// with TMPDIR unset. This box's /tmp is a 14G tmpfs mounted `usrquota`, the
// quota is per UID and every seat runs as the same user, so it is one
// BOX-WIDE budget concurrent suites spend together — and 5.26 GB of it was
// already held by three abandoned `posse-testhome-*` trees 3-7 hours old.
// The suite red with `disk quota exceeded` on the TESTS: 112 FAILs in the
// cmd/posse arm alone, and that arm held 34 cold first execs against the 231
// it holds with room, because its tests died before they could plant. A door
// whose number measures the box, over a log that reads like a broken change.
//
// THE TWO THINGS AN ARM THEREFORE CARRIES, and both are a recipe line away
// from being dropped by a bead editing an arm for some other reason:
//
//   - cxaRoom, the refusal. The door reads the binding number — the QUOTA
//     share where the filesystem has one, which `df` cannot see at all
//     (ranger-base-akk9f) — and refuses below the floor, in milliseconds
//     instead of at the end of an hour. An arm without it is ADR 0085 D5's
//     own case: a run that is not covered and does not say so.
//   - cxaTemp, the arm's own temp directory. Nested under $TMPDIR, never
//     replacing it, and cleared however the arm ends. An arm without it
//     writes into the shared directory and leaves its residue there when it
//     is killed, which is exactly the ratchet that filled the room above.
//
// It reads the recipe the way the derivation does, and then reads the two
// variables themselves: a pin that only checks the arms NAME them is one
// that `COLDEXEC_ROOM :=` emptied to nothing passes.
//
// NO DOOR, unlike its two neighbours, and the register says why rather than
// leaving it to be noticed: this one reads the Makefile and nothing else, so
// it enumerates no directory and is not tree-wide under
// treewidedoor_qa_test.go's rule. A door variable here is reserved for pins
// that ARE — `make tree-check` exists so a filtered run still opens the pins
// whose subject is the whole tree, and a Makefile reader is not one of them.
// It runs with the package, which is where a pin about one file belongs.
func TestQAEveryColdexecArmReadsTheRoomAndTakesItsOwnTempDir(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	mk := string(b)

	for _, arm := range cxaArms(t, mk) {
		if !arm.supervised {
			// Not an arm of this door at all; the derivation above
			// reports that, and reporting it twice would make one
			// dropped supervisor read as two findings.
			continue
		}
		if !strings.Contains(arm.line, cxaRoom) {
			t.Errorf("`make %s` has an arm that does not read the room before it runs:\n  %s\nAdd %s. Without it the arm starts on a box it does not fit in, its tests die on their writes, and what the log holds is a hundred unrelated red names and a held-exec count that measures the box rather than the class (ranger-base-w5z7u). ADR 0085 D5 is that this door refuses rather than running a suite uncovered, and a box too small is the same kind of edge as a kernel it cannot cover.", cxaTarget, arm.line, cxaRoom)
		}
		if !strings.Contains(arm.line, cxaTemp) {
			t.Errorf("`make %s` has an arm that writes into the shared temp directory:\n  %s\nAdd %s. It nests a per-run directory under $TMPDIR and clears it however the arm ends — so an arm killed by a timeout, a panic or a signal still answers for its own residue. internal/posse's TestMain removes its home after m.Run() and therefore cannot, and on a box whose temp room is a shared quota that residue is spent out of everyone's budget (ranger-base-w5z7u).", cxaTarget, arm.line, cxaTemp)
		}
	}

	for _, name := range cxaSorted(cxaVarMustHold) {
		def, ok := cxaMakeVar(mk, name)
		if !ok {
			t.Errorf("the Makefile no longer defines %s, and `make %s`'s arms are written in terms of it — an undefined make variable expands to NOTHING, so every arm would still read as an arm and would carry neither the refusal nor its own temp directory (ranger-base-w5z7u).", name, cxaTarget)
			continue
		}
		for _, want := range cxaVarMustHold[name] {
			if !strings.Contains(def, want) {
				t.Errorf("%s no longer carries %q:\n  %s = %s\nThe arms name this variable, so emptying or narrowing it drops the guard from all five of them at once with every arm line still looking right (ranger-base-w5z7u).", name, want, name, def)
			}
		}
	}

	// And COLDEXEC_TMP's root, which is a shape and not a word: the
	// directory the arms run in has to be NESTED under $TMPDIR, with a bare
	// /tmp as its only fallback.
	if def, ok := cxaMakeVar(mk, "COLDEXEC_TMP"); ok {
		tmpl, found := cxaMktempTemplate(def)
		switch {
		case !found:
			t.Errorf("COLDEXEC_TMP no longer runs `mktemp -d` on a template this pin can read:\n  COLDEXEC_TMP = %s\nThe arms carry this variable as their whole prelude, so where its directory IS cannot be read off the arm lines — it is read here or nowhere (ranger-base-nxus6).", def)
		case !strings.HasPrefix(tmpl, cxaTempRoot+"/"):
			t.Errorf("COLDEXEC_TMP's directory is rooted at %q, not nested under %s:\n  COLDEXEC_TMP = %s\n"+
				"The arm's temp directory goes UNDER $TMPDIR, never beside it and never at a path of its own. A root anywhere else buys the room and costs a class: four pins whose premise is the unconditional bare /tmp in every rendered seatbelt go red, and this pin green while they are red is the confusion ranger-base-w5z7u's close exists to prevent (ranger-base-u8xk4 holds that defect, ranger-base-akk9f the surfaces that abbreviate $HOME). A seat that has already moved its TMPDIR keeps it — the arm nests inside it. If the room on /tmp is the problem, that is the operator's to clear and the door's to REFUSE over, not this variable's to route around (ranger-base-nxus6).",
				tmpl, cxaTempRoot, def)
		}
	}

	// The floor is the one of the three that is a NUMBER, and a floor of
	// zero turns the refusal off while every line above still passes.
	if def, ok := cxaMakeVar(mk, "COLDEXEC_TMP_FLOOR_MB"); ok {
		n, err := strconv.Atoi(strings.TrimSpace(def))
		if err != nil {
			t.Errorf("COLDEXEC_TMP_FLOOR_MB is %q, which is not a number of megabytes — the door takes it as -min-temp-mb, where anything it cannot parse is a flag error and anything <= 0 is the check turned off (ranger-base-w5z7u).", def)
		} else if n <= 0 {
			t.Errorf("COLDEXEC_TMP_FLOOR_MB is %d, which turns the refusal off: `-min-temp-mb 0` is the door's own spelling for \"do not check\", and every arm line still names $(COLDEXEC_ROOM) and reads as guarded (ranger-base-w5z7u).", n)
		}
	}
}

// cxaMktempTemplate returns the template argument of the first `mktemp -d` in
// a Makefile variable's value, with one layer of shell quoting removed.
//
// Deliberately the simplest reader that can answer "what is this directory
// rooted at", for the same reason cxaMakeVar is: the one value it is asked
// about is a single `mktemp -d "<template>"` and a reader that handled word
// splitting, nested substitution and `$(...)` would be a second shell nobody
// has a use for. It answers false rather than guessing when the token is not
// there, and the caller reports that as its own finding.
func cxaMktempTemplate(def string) (string, bool) {
	_, rest, ok := strings.Cut(def, "mktemp -d")
	if !ok {
		return "", false
	}
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" {
		return "", false
	}
	if q := rest[0]; q == '"' || q == '\'' {
		arg, _, ok := strings.Cut(rest[1:], string(q))
		return arg, ok
	}
	words := strings.FieldsFunc(rest, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ')' || r == ';'
	})
	if len(words) == 0 {
		return "", false
	}
	return words[0], true
}

// cxaMakeVar reads one `NAME ?= value` or `NAME := value` assignment out of a
// Makefile. Deliberately the simplest reader that can answer this question —
// the three variables it is asked about are single-line assignments at the
// left margin, and a reader that handled continuations and overrides would be
// a second make nobody has a use for.
func cxaMakeVar(mk, name string) (string, bool) {
	for _, line := range strings.Split(mk, "\n") {
		rest, ok := strings.CutPrefix(line, name)
		if !ok {
			continue
		}
		rest = strings.TrimLeft(rest, " \t")
		for _, op := range []string{"?=", ":=", "="} {
			if v, ok := strings.CutPrefix(rest, op); ok {
				return strings.TrimSpace(v), true
			}
		}
	}
	return "", false
}

// TestQAColdexecArmCensusCanStillSayNo drives both readers the two arms above
// rest on, for both answers. Deliberately not behind a door: it reads planted
// source and synthetic recipe lines rather than the tree, so nothing derives
// it and no unrelated bead can red it.
func TestQAColdexecArmCensusCanStillSayNo(t *testing.T) {
	t.Parallel()

	t.Run("the plant matcher", func(t *testing.T) {
		// The plain write is ASSEMBLED rather than spelled, for the reason
		// execwritesiblings_qa_test.go's own control gives: that census is a
		// BYTE matcher over every .go file in the tree, this file is inside
		// its walk, and an exec-bit `os.WriteFile` spelled in full here is a
		// hit on this file's own source whatever the Go around it says. It
		// cost one red of `make execwrite-check` to learn, which is the door
		// working.
		plainWrite := "os.Write" + "File(p, b, 0o755)"
		for _, c := range []struct {
			name string
			src  string
			want int
		}{
			{"a bare call", "package p\nfunc f() { WriteExecutable(p, b, 0o755) }\n", 1},
			{"a qualified call", "package p\nfunc f() { posse.WriteExecutable(p, b, 0o755) }\n", 1},
			{"two on one line", "package p\nfunc f() { WriteExecutable(a, b, 0o755); WriteExecutable(c, d, 0o755) }\n", 2},
			// The two shapes grep counts and this must not: the writer's
			// own declaration (two in the tree) and the name quoted in a
			// positive-witness table (twelve in the tree).
			{"the declaration is not a call", "package p\nfunc WriteExecutable(path string, b []byte, m uint32) error { return nil }\n", 0},
			{"a quoted mention is not a call", "package p\nvar want = `WriteExecutable(p, b, 0o755)`\n", 0},
			{"a plain write is not a plant", "package p\nfunc f() { " + plainWrite + " }\n", 0},
			{"no plant at all", "package p\nfunc f() {}\n", 0},
		} {
			t.Run(c.name, func(t *testing.T) {
				f, err := parser.ParseFile(token.NewFileSet(), "x.go", c.src, parser.SkipObjectResolution)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				if got := cxaPlantCalls(f); got != c.want {
					t.Errorf("cxaPlantCalls = %d, want %d, over:\n%s", got, c.want, c.src)
				}
			})
		}
	})

	t.Run("the arm reader", func(t *testing.T) {
		const supervised = "POSSE_COLD_EXEC_DELAY='$(COLDEXEC_DELAY)' ./$(COLDEXEC_BIN) -- scripts/test-times.sh $(GOBIN) test "
		for _, c := range []struct {
			name string
			line string
			want cxaArm
			runs bool
			err  bool
		}{
			{"arm 1, the package-tree form", supervised + "./internal/posse/... -timeout 25m -count=1", cxaArm{dir: "internal/posse", tree: true, supervised: true}, true, false},
			{"a tagged arm, flags before the package", supervised + "-tags posse_arm2 ./internal/posse -timeout 25m -count=1", cxaArm{dir: "internal/posse", supervised: true}, true, false},
			{"an arm in a directory of its own", supervised + "./cmd/posse -count=1", cxaArm{dir: "cmd/posse", supervised: true}, true, false},
			{"a bare `go test` is an arm that injects nothing", "$(GOBIN) test ./cmd/posse -count=1", cxaArm{dir: "cmd/posse"}, true, false},
			{"building the supervisor runs no test", "$(GOBIN) build -o $@ ./cmd/coldexec", cxaArm{}, false, false},
			{"a recipe line that is not go test", "mkdir -p plugin/bin", cxaArm{}, false, false},
			{"a subtree pattern is a prefix, not a directory", supervised + "./internal/... -count=1", cxaArm{dir: "internal", tree: true, supervised: true}, true, false},
			{"the whole module is a prefix of everything", supervised + "./... -count=1", cxaArm{dir: "", tree: true, supervised: true}, true, false},
			{"a `go test` naming no package is unreadable", supervised + "-count=1", cxaArm{}, true, true},
		} {
			t.Run(c.name, func(t *testing.T) {
				arm, runs, err := cxaReadArm(c.line)
				if runs != c.runs {
					t.Fatalf("cxaReadArm(%q) runs = %v, want %v", c.line, runs, c.runs)
				}
				if (err != nil) != c.err {
					t.Fatalf("cxaReadArm(%q) err = %v, want err %v", c.line, err, c.err)
				}
				if err != nil {
					return
				}
				if arm.dir != c.want.dir || arm.tree != c.want.tree || arm.supervised != c.want.supervised {
					t.Errorf("cxaReadArm(%q) = {dir:%q tree:%v supervised:%v}, want {dir:%q tree:%v supervised:%v}", c.line, arm.dir, arm.tree, arm.supervised, c.want.dir, c.want.tree, c.want.supervised)
				}
			})
		}
	})

	// And the prefix itself, which is the reading the table above records and
	// the one the first reader got wrong.
	t.Run("an arm's coverage", func(t *testing.T) {
		for _, c := range []struct {
			arm  cxaArm
			pkg  string
			want bool
		}{
			{cxaArm{dir: "internal/posse"}, "internal/posse", true},
			{cxaArm{dir: "internal/posse"}, "internal/treepins", false},
			{cxaArm{dir: "internal/posse"}, "internal/posse/sub", false},
			{cxaArm{dir: "internal/posse", tree: true}, "internal/posse", true},
			{cxaArm{dir: "internal/posse", tree: true}, "internal/posse/sub", true},
			{cxaArm{dir: "internal", tree: true}, "internal/posse", true},
			{cxaArm{dir: "internal", tree: true}, "internal/treepins", true},
			{cxaArm{dir: "internal", tree: true}, "cmd/posse", false},
			// A prefix of a NAME is not a prefix of a path.
			{cxaArm{dir: "internal/poss", tree: true}, "internal/posse", false},
			{cxaArm{dir: "", tree: true}, "cmd/posse", true},
		} {
			if got := c.arm.covers(c.pkg); got != c.want {
				t.Errorf("cxaArm{dir:%q tree:%v}.covers(%q) = %v, want %v", c.arm.dir, c.arm.tree, c.pkg, got, c.want)
			}
		}
	})

	t.Run("the make variable reader", func(t *testing.T) {
		// cxaMakeVar is the second reader the room arm rests on, and it fails
		// the same way the others would: a reader that stops matching reports
		// "the Makefile no longer defines this", which reds — so the direction
		// worth proving here is the quiet one, a reader that answers for a
		// variable it was not asked about.
		const mk = "COLDEXEC_DELAY ?= 2.5s\n" +
			"COLDEXEC_TMP_FLOOR_MB ?= 2048\n" +
			"COLDEXEC_TMP   := T=$$(mktemp -d x) && trap 'rm -rf \"$$T\"' EXIT && TMPDIR=$$T\n" +
			"COLDEXEC_ROOM  := -min-temp-mb $(COLDEXEC_TMP_FLOOR_MB)\n" +
			"COLDEXEC_BIN   := bin/coldexec\n"
		for _, c := range []struct {
			name string
			var_ string
			want string
			ok   bool
		}{
			{"a ?= assignment", "COLDEXEC_TMP_FLOOR_MB", "2048", true},
			{"a := assignment, padded", "COLDEXEC_ROOM", "-min-temp-mb $(COLDEXEC_TMP_FLOOR_MB)", true},
			{"a value carrying its own operators", "COLDEXEC_TMP", "T=$$(mktemp -d x) && trap 'rm -rf \"$$T\"' EXIT && TMPDIR=$$T", true},
			// The one that would make the room arm silent: a prefix of a
			// real name must not answer with that name's value, or deleting
			// COLDEXEC_ROOM reads as COLDEXEC_ROOM still being there.
			{"a variable that is not there", "COLDEXEC_FLOOR", "", false},
			{"a name that is a prefix of a real one", "COLDEXEC_TM", "", false},
		} {
			t.Run(c.name, func(t *testing.T) {
				got, ok := cxaMakeVar(mk, c.var_)
				if ok != c.ok || got != c.want {
					t.Errorf("cxaMakeVar(%q) = %q, %v; want %q, %v", c.var_, got, ok, c.want, c.ok)
				}
			})
		}
	})

	t.Run("the recipe reader finds the live door", func(t *testing.T) {
		// The floor under the arm reader: a recipe this reader stopped
		// understanding reports no arms, and "no arms" is the one reading
		// under which the coverage arm has nothing to say.
		b, err := os.ReadFile("Makefile")
		if err != nil {
			t.Fatal(err)
		}
		var pkgs []string
		for _, arm := range cxaArms(t, string(b)) {
			if !slices.Contains(pkgs, arm.pattern) {
				pkgs = append(pkgs, arm.pattern)
			}
		}
		if len(pkgs) == 0 {
			t.Fatalf("the reader found no arm in the live `%s` recipe", cxaTarget)
		}
		t.Logf("`make %s` arms cover: %v", cxaTarget, pkgs)
	})
}
