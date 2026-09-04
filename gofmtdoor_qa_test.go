package posse

// QA pins for ranger-base-rulbl — the fast door to gofmt-cleanliness.
//
// THE GAP. `TestTreeIsGofmtClean` (internal/posse/gofmtclean_qa_test.go) is
// the pin on this property and stays the pin. It lives inside internal/posse,
// a ~950s package (943.9s on 2026-09-03, 1050.7s on 2026-09-01), well past the
// 600s a seat can spend in one foreground call — so the standing advice is a
// focused `-run` filter instead. `-run` selects by test NAME, and no seat's
// filter has ever named `TreeIsGofmtClean`, because gofmt is nobody's subject.
// Four commits reached main not gofmt-clean through exactly that door:
// ranger-base-ig1o, -d4ya, -edg8 and -4v4r6. The close that shipped the last
// one said so plainly — `-run '(Pin|PlanUsage|Override|Credential|Loopback)'`,
// ok 45.7s — and none of those five alternations matches. Nobody was careless.
// The property costs under a second to check and cost ~950s to reach, and the
// repo had no reader of it at all: `make fmt` WRITES, `vet` never asks, and
// `git config core.hooksPath` is empty in a posse worktree.
//
// `make fmt-check` is the door. These arms hold it open.
//
//  1. it is wired: `make test` depends on it, `fmt` and `fmt-check` name the
//     same files and the same gofmt, and the reader still only reads.
//  2. it is WIDE ENOUGH. A door narrower than the pin is worse than no door:
//     the seat gets a green check and the suite goes red anyway. Two-way
//     against a walk of the repo, so a new .go tree neither side can reach
//     is a red here rather than a red on main. This arm found the defect it
//     exists for on the first run — `fmt` wrote `cmd internal embed.go`
//     while the pin walked the whole repo root, so the 43 *_qa_test.go files
//     beside embed.go were pinned by a test and fixed by no command.
//  3. it can FAIL. The recipe is `make -n`'s own expansion of it, run for
//     real against a scratch tree: clean is silent at rc 0, and the drift
//     ig1o filed exits nonzero, names the file and names the fix.
//
// Arm 3 uses the drift shape from ig1o rather than "a missing blank line":
// gofmt normalises list-item separation to be CONSISTENT, so a list with no
// separators anywhere is already consistent and is left alone. A control
// built on the easy shape would pass over the bug that started this.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The one line both targets read their files from, and the one line arm 2
// measures. Kept as a prefix match rather than a full expansion: make's
// own expansion is what arm 3 runs.
const fmtRootsVar = "FMT_ROOTS := "

// fmtRoots returns the roots `$(FMT_ROOTS)` names, as written.
func fmtRoots(t *testing.T, makefile string) []string {
	t.Helper()
	for _, line := range strings.Split(makefile, "\n") {
		if strings.HasPrefix(line, fmtRootsVar) {
			return strings.Fields(strings.TrimPrefix(line, fmtRootsVar))
		}
	}
	t.Fatalf("the Makefile defines no %q — `fmt` and `fmt-check` have no shared idea of which files gofmt is asked about", strings.TrimSpace(fmtRootsVar))
	return nil
}

// Arm 1: the door exists, `make test` opens it, and it opens on the same
// files `make fmt` writes.
func TestQAMakeTestOpensTheGofmtDoor(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	recipe := makeRecipe(src, "fmt-check")
	if len(recipe) == 0 {
		t.Fatal("the Makefile has no `fmt-check` target — gofmt-cleanliness is back to having no reader under ~950s (ranger-base-rulbl)")
	}
	body := strings.Join(recipe, "\n")
	if !strings.Contains(body, "-l ") {
		t.Errorf("`make fmt-check` no longer runs gofmt in LIST mode — a door that does not report is not a door:\n%s", body)
	}
	if strings.Contains(body, "-w ") {
		t.Errorf("`make fmt-check` WRITES — the point of a second target is that a seat can ask the question without changing the tree:\n%s", body)
	}
	if !strings.Contains(body, "$(GOFMT)") || !strings.Contains(body, "$(FMT_ROOTS)") {
		t.Errorf("`make fmt-check` no longer reads $(GOFMT)/$(FMT_ROOTS) — the reader and the writer can now name different gofmts or different files:\n%s", body)
	}

	// The writer, same two variables. This is the equality that makes the
	// door's own error message ("run `make fmt`") true.
	write := strings.Join(makeRecipe(src, "fmt"), "\n")
	if write == "" {
		t.Fatal("the Makefile has no `fmt` target")
	}
	if !strings.Contains(write, "$(GOFMT)") || !strings.Contains(write, "$(FMT_ROOTS)") {
		t.Errorf("`make fmt` no longer writes the files `make fmt-check` reads, so the fix it tells you to run does not fix what it reported:\n%s", write)
	}

	// $(GOFMT) is the toolchain's, not PATH's. gofmt's output is
	// version-specific and TestTreeIsGofmtClean runs go/format out of
	// $(GOBIN)'s toolchain, so a PATH gofmt from another Go would disagree
	// with the test this is a door to — and a gated session's PATH is
	// shims before tools.
	var gofmtVar string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "GOFMT ") || strings.HasPrefix(line, "GOFMT:") || strings.HasPrefix(line, "GOFMT=") {
			gofmtVar = line
			break
		}
	}
	if !strings.Contains(gofmtVar, "GOROOT") {
		t.Errorf("$(GOFMT) is no longer resolved out of the toolchain's GOROOT, so `make fmt-check` can answer for a different Go than the pin it stands in for: %q", gofmtVar)
	}

	var deps string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "test:") {
			deps = line
			break
		}
	}
	if deps == "" {
		t.Fatal("the Makefile has no `test` target")
	}
	if !strings.Contains(deps, "fmt-check") {
		t.Errorf("`make test` no longer depends on fmt-check, so the only thing that reports a gofmt drift is again ~950s away: %q", deps)
	}
}

// Arm 2: the door reaches every .go file in the repo. Two-way — a file no
// root reaches is the failure this bead is about arriving one directory
// over, and a root that reaches nothing is a line nobody notices is dead.
func TestQATheGofmtDoorReachesEveryGoFile(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	roots := fmtRoots(t, string(b))
	if len(roots) == 0 {
		t.Fatal("$(FMT_ROOTS) is empty — `make fmt-check` asks about nothing and passes")
	}

	reached := map[string]bool{}
	for _, root := range roots {
		var got []string
		if strings.ContainsAny(root, "*?[") {
			// A glob: exactly the files it matches, no recursion. This is
			// how the shell hands them to gofmt.
			m, err := filepath.Glob(root)
			if err != nil {
				t.Fatalf("bad glob %q in $(FMT_ROOTS): %v", root, err)
			}
			for _, p := range m {
				if strings.HasSuffix(p, ".go") {
					got = append(got, p)
				}
			}
		} else {
			// A directory: gofmt walks it whole.
			err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(path, ".go") {
					got = append(got, path)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("$(FMT_ROOTS) names %q, which gofmt cannot walk: %v", root, err)
			}
		}
		if len(got) == 0 {
			t.Errorf("$(FMT_ROOTS) names %q and it holds no .go file — a dead root reads as coverage", root)
		}
		for _, p := range got {
			reached[filepath.Clean(p)] = true
		}
	}

	// The other direction, which is the one that matters: every .go file
	// in the tree, from a walk that knows nothing about the roots.
	var missed []string
	total := 0
	err = filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		total++
		if !reached[filepath.Clean(path)] {
			missed = append(missed, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// A walk that found nothing would agree with a door that reaches
	// nothing. The floor is far under what the tree holds (576 on
	// 2026-09-04), so it survives ordinary growth in both directions.
	if total < 200 {
		t.Fatalf("only %d .go files found in the tree — the walk this arm compares against found nothing", total)
	}
	if len(missed) > 0 {
		t.Errorf("%d of %d .go files are outside $(FMT_ROOTS) %v — `make fmt-check` reports them clean and `make fmt` does not fix them, which is ranger-base-rulbl one directory over:\n  %s",
			len(missed), total, roots, strings.Join(missed, "\n  "))
	}
	t.Logf("$(FMT_ROOTS) %v reaches all %d .go files in the tree", roots, total)
}

// Arm 3: the door can fail. `make -n` expands the recipe with make's own
// variables — no second copy of it here to drift — and it is run for real
// against a scratch tree, clean arm first so a recipe that always fails is
// not mistaken for one that detects.
func TestQATheGofmtDoorReportsRealDrift(t *testing.T) {
	t.Parallel()
	out, err := exec.Command("make", "-n", "fmt-check").Output()
	if err != nil {
		t.Fatalf("make -n fmt-check: %v", err)
	}
	recipe := strings.TrimSpace(string(out))
	if recipe == "" {
		t.Fatal("`make -n fmt-check` expands to nothing")
	}
	// `$(` alone would match the shell's own command substitution, which
	// this recipe uses; the make variables are named.
	for _, v := range []string{"$(GOFMT)", "$(FMT_ROOTS)"} {
		if strings.Contains(recipe, v) {
			t.Fatalf("the expanded recipe still holds %s — this arm would be measuring the wrong text:\n%s", v, recipe)
		}
	}

	// The scratch tree carries the same shape $(FMT_ROOTS) names, so the
	// recipe runs against it unmodified.
	dir := t.TempDir()
	for _, sub := range []string{"cmd", "internal"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	clean := "package p\n\n// A doc.\n//\n//   - one, with a body that\n//     wraps a line.\n//\n//   - two follows, separated the same way.\nfunc f() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "clean.go"), []byte(clean), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func() (string, error) {
		cmd := exec.Command("sh", "-c", recipe)
		cmd.Dir = dir
		b, err := cmd.CombinedOutput()
		return string(b), err
	}

	got, err := run()
	if err != nil {
		t.Fatalf("the door failed a gofmt-clean tree — it reports drift that is not there:\n%s", got)
	}
	if strings.TrimSpace(got) != "" {
		t.Errorf("a clean tree is not silent, so a seat learns to ignore the output:\n%s", got)
	}

	// The same comment with the last item's separator dropped — the shape
	// ig1o filed, verified against the real pre-fix file while
	// gofmtclean_qa_test.go was written.
	drifted := "package p\n\n// A doc.\n//\n//   - one, with a body that\n//     wraps a line.\n//\n//   - two, with a body that\n//     wraps a line.\n//   - three follows with no separator.\nfunc f() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "drifted.go"), []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = run()
	if err == nil {
		t.Fatalf("the door passed a tree holding the exact drift that reddened main four times — it reads nothing (ranger-base-ig1o / -d4ya / -edg8 / -4v4r6):\n%s", got)
	}
	if !strings.Contains(got, "drifted.go") {
		t.Errorf("the door failed without naming the file, so it says a seat is wrong and not where:\n%s", got)
	}
	if strings.Contains(got, "clean.go") {
		t.Errorf("the door named a file that is already gofmt-clean:\n%s", got)
	}
	if !strings.Contains(got, "make fmt") {
		t.Errorf("the door does not name the fix, and the fix is the whole reason a seat can act on it in one line:\n%s", got)
	}
}
