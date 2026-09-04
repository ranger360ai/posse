package posse

// QA pins for ranger-base-ik44f — fast doors to the other four tree-wide pins.
//
// THE CLASS, as ranger-base-rulbl named it: "a QA test whose subject is the
// TREE, living inside a package nobody runs whole". internal/posse is ~950s,
// past the 600s a seat can spend in one foreground call, so the standing
// advice is a focused `-run` filter — and `-run` selects by test NAME. A
// tree-wide pin is nobody's subject, so no seat's filter ever names it, and
// the pin is unreachable at exactly the moment it would have mattered.
//
// rulbl shipped the first door (`make fmt-check`) and censused the rest.
// This bead is the other four:
//
//	TestTreeIsGofmtClean                          make fmt-check   ~1.5s
//	TestShippedTreeNamesRolesNotThisCrew          make crew-check  ~2.5s
//	TestShippedStringsNameRolesNotThisCrew        make crew-check
//	TestTestCorpusHidesNoCrewNameBehindAnEscape   make crew-check
//	TestHerdrSelectorsAreNamedByADR0016           make selector-check ~0.5s
//
// and `make tree-check` is all of them, which is the command a seat types
// after a filtered run.
//
// WHY THE DOOR RUNS THE PIN. fmt-check re-runs the TOOL, because gofmt is a
// tool and `gofmt -l` cannot disagree with `go/format`. These four are Go:
// their reading is an ast parse, an unquote and a case-boundary scan, and a
// shell rewrite of that would be a second implementation to keep in sync by
// hand — a door that goes NARROWER than the pin while both look green, which
// is worse than no door at all. So each door is a `-run` filter naming the
// pin, and the only thing left to hold is that the filters name ALL of them.
//
// Three arms, rulbl's three:
//
//  1. the doors are wired — `make test` depends on tree-check, tree-check
//     reaches all three doors, and each door reads only (no `-w`, no `./...`,
//     and `-count=1`, because a door that answers from cache can lie).
//  2. the doors are WIDE ENOUGH, two-way and mechanically: the class is
//     derived by parsing internal/posse/*_test.go for the tests that call
//     qibRepoRoot — the same rule the bead censused with — and every member
//     must be named by a Makefile door variable, and every name in those
//     variables must be a member. A tree-wide pin added tomorrow reds this
//     until it is given a door, which is the whole deliverable.
//  3. the doors can FAIL: `make -n`'s own expansion of each, run for real
//     against a scratch copy of the tree carrying the real drift — a crew
//     name in a shipped file, and an ADR page that stopped naming a selector
//     it subscribes to. Clean arm first, so a door that always fails is not
//     mistaken for one that detects.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The Makefile variables that hold the class. One per door, plus the pin
// whose door is a tool rather than a filter — the union is what arm 2
// measures against the tree.
var twdPinVars = []string{"QA_CREW_PINS", "QA_SELECTOR_PINS", "QA_TOOL_PINS"}

// twdVar returns the `|`-separated test names a Makefile variable holds.
func twdVar(t *testing.T, makefile, name string) []string {
	t.Helper()
	for _, line := range strings.Split(makefile, "\n") {
		if !strings.HasPrefix(line, name) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, name))
		if !strings.HasPrefix(rest, ":=") {
			continue
		}
		var out []string
		for _, f := range strings.Split(strings.TrimSpace(strings.TrimPrefix(rest, ":=")), "|") {
			if f = strings.TrimSpace(f); f != "" {
				out = append(out, f)
			}
		}
		return out
	}
	t.Fatalf("the Makefile defines no %s — a door variable the pin below reads is gone, so the class has no recorded membership", name)
	return nil
}

// twdPrereqs returns the prerequisites of a target, from its `target:` line.
func twdPrereqs(t *testing.T, makefile, target string) []string {
	t.Helper()
	for _, line := range strings.Split(makefile, "\n") {
		if strings.HasPrefix(line, target+":") && !strings.HasPrefix(line, target+":=") {
			return strings.Fields(strings.TrimPrefix(line, target+":"))
		}
	}
	t.Fatalf("the Makefile has no `%s` target", target)
	return nil
}

// Arm 1: the doors exist, `make test` opens them, and they only read.
func TestQAMakeTestOpensTheTreeWideDoors(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// The umbrella reaches every door. It carries no recipe of its own, so
	// `make -n tree-check` prints exactly what a seat would have to type.
	tree := strings.Join(twdPrereqs(t, src, "tree-check"), " ")
	for _, door := range []string{"fmt-check", "crew-check", "selector-check"} {
		if !strings.Contains(tree, door) {
			t.Errorf("`make tree-check` no longer reaches `%s`, so one tree-wide pin is back to being ~950s away: %q", door, tree)
		}
	}

	// And `make test` reaches the umbrella, so a full run fails on the class
	// in seconds instead of at ~950 (rulbl's reason for fmt-check).
	if deps := strings.Join(twdPrereqs(t, src, "test"), " "); !strings.Contains(deps, "tree-check") {
		t.Errorf("`make test` no longer depends on tree-check: %q", deps)
	}

	if phony := strings.Join(twdPrereqs(t, src, ".PHONY"), " "); !strings.Contains(phony, "tree-check") ||
		!strings.Contains(phony, "crew-check") || !strings.Contains(phony, "selector-check") {
		t.Errorf(".PHONY does not name the new doors — a file of that name in the tree would silence one: %q", phony)
	}

	for _, door := range []struct{ target, variable string }{
		{"crew-check", "QA_CREW_PINS"},
		{"selector-check", "QA_SELECTOR_PINS"},
	} {
		recipe := strings.Join(makeRecipe(src, door.target), "\n")
		if recipe == "" {
			t.Errorf("the Makefile has no `%s` target", door.target)
			continue
		}
		// The door reads its filter from the one variable arm 2 measures.
		// Spelled here, so a door quietly narrowed to a literal subset of
		// the class fails rather than passing on a shorter list.
		if !strings.Contains(recipe, "'^($("+door.variable+"))$$'") {
			t.Errorf("`make %s` no longer runs exactly $(%s), anchored — the door and the pin it stands in for can now name different tests:\n%s", door.target, door.variable, recipe)
		}
		if !strings.Contains(recipe, "./internal/posse") {
			t.Errorf("`make %s` no longer names ./internal/posse — a package-tree run is the ~950s this door exists to avoid:\n%s", door.target, recipe)
		}
		if strings.Contains(recipe, "./...") {
			t.Errorf("`make %s` runs the package tree, which is the wall it is a door through:\n%s", door.target, recipe)
		}
		// A door that can answer from cache is a door that can lie: the
		// drift these pins are about arrives as a new FILE in a walked
		// directory, and nothing promises go's test cache key notices one.
		if !strings.Contains(recipe, "-count=1") {
			t.Errorf("`make %s` may answer from go's test cache, so it can report a tree it never read:\n%s", door.target, recipe)
		}
		if strings.Contains(recipe, " -w") || strings.Contains(recipe, "rm ") {
			t.Errorf("`make %s` writes — the point of a door is that a seat can ask the question without changing the tree:\n%s", door.target, recipe)
		}
	}
}

// twdTreeWideTests parses internal/posse/*_test.go and returns the tests whose
// subject is the TREE: the ones that call qibRepoRoot in their own body. That
// helper resolves the repo root off runtime.Caller, so calling it is what
// makes a test read files outside its package — the same rule the bead
// censused the class with (`grep -rn 'qibRepoRoot(t)'`), asked of the ast so
// a call spelled across a line break still counts.
func twdTreeWideTests(t *testing.T) (names []string, funcs int) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("internal", "posse", "*_test.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no test files found under internal/posse: %v", err)
	}
	fset := token.NewFileSet()
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			funcs++
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "qibRepoRoot" {
					names = append(names, fn.Name.Name)
					return false
				}
				return true
			})
		}
	}
	return names, funcs
}

// Arm 2: every tree-wide pin has a door, and every door names a real one.
// Two-way, because both directions are the bug: a pin with no door is
// ranger-base-ik44f arriving again, and a door naming a test that no longer
// exists is a green `-run` filter that runs nothing.
func TestQAEveryTreeWidePinHasADoor(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	doored := map[string]string{}
	for _, v := range twdPinVars {
		for _, name := range twdVar(t, src, v) {
			if prev, dup := doored[name]; dup {
				t.Errorf("%s is named by both $(%s) and $(%s) — one pin, two doors, and the two can drift apart", name, prev, v)
			}
			doored[name] = v
		}
	}

	// QA_TOOL_PINS is the one membership this arm cannot check by wiring: a
	// tool door (`gofmt -l`) names no test, so a pin listed there is a CLAIM
	// that some tool reads it. Exactly one pin makes that claim today, and
	// its door is held open by gofmtdoor_qa_test.go. A second name arriving
	// here would be a pin declaring itself doored with nothing to show —
	// the cheapest way to silence the check below — so it fails until this
	// arm is taught what the new tool door is and who pins it.
	if tool := twdVar(t, src, "QA_TOOL_PINS"); len(tool) != 1 || tool[0] != "TestTreeIsGofmtClean" {
		t.Errorf("$(QA_TOOL_PINS) = %v, want exactly [TestTreeIsGofmtClean] — that variable records the pins whose door is a TOOL rather than a `-run` filter, which is a claim nothing here can verify. A new entry needs its own arm; a pin moved here to quiet this test has no door at all.", tool)
	}

	names, funcs := twdTreeWideTests(t)
	// A pin over a derived set is satisfied by deriving nothing: say how
	// many test functions were actually parsed, and fail a walk that found
	// far fewer than internal/posse holds (1000+ on 2026-09-04).
	if funcs < 300 {
		t.Fatalf("only %d test functions parsed under internal/posse — the walk this arm derives the class from found nothing", funcs)
	}
	if len(names) < 5 {
		t.Fatalf("only %d tree-wide tests found (5 on 2026-09-04) — qibRepoRoot has been renamed or wrapped, and this arm is now deriving a class that is missing members", len(names))
	}
	t.Logf("parsed %d test functions under internal/posse, %d of them tree-wide", funcs, len(names))

	found := map[string]bool{}
	for _, name := range names {
		found[name] = true
		if doored[name] == "" {
			t.Errorf("%s reads the repo root and no Makefile door names it — it is reachable only by a `-run` filter that happens to spell it, which is ranger-base-ik44f arriving again. Add it to $(QA_CREW_PINS), $(QA_SELECTOR_PINS) or a new door.", name)
		}
	}
	for name, v := range doored {
		if !found[name] {
			t.Errorf("$(%s) names %s, which is not a tree-wide test in internal/posse — the door's `-run` filter matches nothing there and passes in silence", v, name)
		}
	}
}

// twdSourceFiles lists what to copy: git's idea of the working tree, which
// leaves out bin/ and dist/ and every other build output.
//
// It falls back to a walk when git cannot answer, and the fallback is not
// decoration: a `git archive | tar -x` scratch tree is the house rig for
// mutation runs on this repo, it has no .git, and four root/internal pins
// already red on that alone in every arm including the control. A fifth
// would be a tax on every future rig, paid to save ten lines here.
func twdSourceFiles(t *testing.T) []string {
	t.Helper()
	if out, err := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output(); err == nil {
		var files []string
		for _, rel := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
			if rel != "" {
				files = append(files, rel)
			}
		}
		return files
	}
	var files []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "dist", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// twdSeedTree copies the working tree into a scratch directory: the door's
// recipe compiles internal/posse from there, so qibRepoRoot — which resolves
// off the compiled source's own path — answers the scratch root and the pins
// walk the copy. The WORKING tree and not HEAD, so a pin renamed in one
// commit with its door does not fail this arm against a stale HEAD.
func twdSeedTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copied := 0
	for _, rel := range twdSourceFiles(t) {
		info, err := os.Lstat(rel)
		if err != nil || !info.Mode().IsRegular() {
			// A symlink (plugin/bin/posse points at the installed binary)
			// or a path git knows and the tree does not. Neither compiles
			// and neither is read by the pins.
			continue
		}
		body, err := os.ReadFile(rel)
		if err != nil {
			continue
		}
		dst := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, body, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
		copied++
	}
	if copied < 500 {
		t.Fatalf("only %d files copied into the scratch tree — the pins below have file-count floors and would fail on the rig rather than on the drift", copied)
	}
	return dir
}

// twdExpand returns `make -n`'s own expansion of a door's recipe, so nothing
// here is a second copy of the command to drift from it.
func twdExpand(t *testing.T, target string) string {
	t.Helper()
	out, err := exec.Command("make", "-n", target).Output()
	if err != nil {
		t.Fatalf("make -n %s: %v", target, err)
	}
	recipe := strings.TrimSpace(string(out))
	if recipe == "" {
		t.Fatalf("`make -n %s` expands to nothing", target)
	}
	if strings.Contains(recipe, "$(") {
		t.Fatalf("the expanded recipe still holds a make variable — this arm would be measuring the wrong text:\n%s", recipe)
	}
	return recipe
}

// Arm 3: the doors can fail, on the drift they exist for.
func TestQATheTreeWideDoorsReportRealDrift(t *testing.T) {
	t.Parallel()
	crew := twdExpand(t, "crew-check")
	selector := twdExpand(t, "selector-check")
	dir := twdSeedTree(t)

	run := func(recipe string) (string, error) {
		cmd := exec.Command("sh", "-c", recipe)
		cmd.Dir = dir
		b, err := cmd.CombinedOutput()
		return string(b), err
	}

	// The clean arm first, both doors: a door that always fails detects
	// nothing, and a filter that matches nothing passes in silence.
	for _, door := range []struct{ name, recipe string }{{"crew-check", crew}, {"selector-check", selector}} {
		got, err := run(door.recipe)
		if err != nil {
			t.Fatalf("`make %s` failed on a clean copy of this tree — it reports drift that is not there:\n%s", door.name, got)
		}
		if strings.Contains(got, "no tests to run") {
			t.Fatalf("`make %s`'s filter matched no test at all, so its green says nothing:\n%s", door.name, got)
		}
	}

	// The crew door's drift: a crew name in a file the shipped tree walk
	// reads. Assembled, never spelled — this file is inside that walk's
	// repo-root pass, the same reason instancebound_qa_test.go assembles.
	name := "gw" + "art"
	probe := filepath.Join("internal", "twd_drift_probe.txt")
	if err := os.WriteFile(filepath.Join(dir, probe), []byte("seat: "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := run(crew)
	if err == nil {
		t.Fatalf("`make crew-check` passed a tree naming the originating instance's crew — the door reads nothing:\n%s", got)
	}
	for _, want := range []string{filepath.ToSlash(probe), name, "TestShippedTreeNamesRolesNotThisCrew"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make crew-check` failed without naming %q, so it says a seat is wrong and not where:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, probe)); err != nil {
		t.Fatal(err)
	}

	// The selector door's drift: the ADR page stops naming a selector posse
	// subscribes to. Same length, so the pin's own byte floor on the page is
	// not what fails.
	const adr = "docs/adr/0016-herdr-event-hints.md"
	const sel = "pane.agent_detected"
	page, err := os.ReadFile(filepath.Join(dir, adr))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), sel) {
		t.Fatalf("%s does not name %q even before this arm edits it — the drift is already here and selector-check should have caught it", adr, sel)
	}
	drifted := strings.ReplaceAll(string(page), sel, "pane.agent_renamed")
	if err := os.WriteFile(filepath.Join(dir, adr), []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = run(selector)
	if err == nil {
		t.Fatalf("`make selector-check` passed an ADR 0016 that no longer names %q — losing that selector strands every seat that appears after the dial:\n%s", sel, got)
	}
	for _, want := range []string{sel, "TestHerdrSelectorsAreNamedByADR0016"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make selector-check` failed without naming %q:\n%s", want, got)
		}
	}
}
