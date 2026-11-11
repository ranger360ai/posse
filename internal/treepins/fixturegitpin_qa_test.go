package treepins

// QA pins for ranger-base-xa2e3 — the door under a fixture git that unpins
// git's config scope and meets the seat's own L1 shim.
//
// THE DEFECT. Ten tests in cmd/posse and internal/treepins commit a fixture
// into a throwaway repo under t.TempDir, with a hermetic environment so the
// commit cannot read the operator's git config. On a gated seat every one of
// them was refused, by posse's own git shim, before the code under test ran:
//
//	refused by posse gate: git -C /tmp/TestQAPolicyPins.../001 commit -qm qa fixture -- .
//	  — GIT_CONFIG_GLOBAL, HOME or XDG_CONFIG_HOME changes the hook config git
//	  sees in global or system scope, against what the launcher's HOME reads
//	  there (ADR 0079) (rule: hookskip:config-env, ranger-base-r3pri)
//
// The rule is right about a commit into THIS repo — a managed hook a scanner
// may live in must never be skipped (ADR 0052) — and it was reading a commit
// into a scratch fixture repo that has no hooks and is not this repo. So the
// gate that guards the tree made the suite that guards the gate unrunnable
// from a gated seat, and the ten failures read to the next person as real
// drift in the ADR census and the policy pins, which is the expensive part.
//
// TWO FACTS MAKE IT, and neither is in the test's source (MEASURED 2026-11-10
// on this box, git 2.56.0, from a session worktree):
//
//  1. `exec.Command("git", …)` resolves the BARE NAME against this process's
//     $PATH — never cmd.Env — and a dispatched seat's PATH leads with its own
//     gates bin dir. So the fixture's own `git` IS the shim, whatever PATH it
//     then hands the child. That is ranger-base-ebhdx's finding, which is why
//     both cmd/posse and internal/treepins already carry a gitOutsideGates
//     helper; these ten sites were simply not converted.
//  2. The seat's environment PINS global and system scope — a child of this
//     session reads GIT_CONFIG_GLOBAL=<the operator's own global file> and
//     GIT_CONFIG_SYSTEM=/etc/gitconfig (obdnk's pin, the claude inlet's
//     GIT_CONFIG_SYSTEM) — and this box's global scope carries four `alias.*`
//     keys, which are hook-relevant to the arm (an alias can spell `commit
//     -n`). A fixture that DROPS or OVERRIDES that pin therefore makes the
//     shim's two sides differ, and it refuses.
//
// Fact 2 is why the refusal is not universal, and the asymmetry is worth
// stating because the green half looks identical from the call site. MEASURED,
// same box, same day, a scratch repo and three commits through the shim:
//
//	HOME moved, the rest inherited              ALLOWED  (the pin still carries
//	                                                     global scope, so the
//	                                                     two sides are equal)
//	GIT_CONFIG_GLOBAL=/dev/null, HOME inherited REFUSED
//	a replacement env (HOME moved, the pin not   REFUSED
//	  carried at all), as cmd.Env=[]string{…}
//
// So internal/treepins' bottle fixture, which moves HOME with t.Setenv and
// hands its git child no cmd.Env at all, is green, and the policy pins beside
// it, which append GIT_CONFIG_GLOBAL=/dev/null, are red. hookskip.go's own
// comment reads "GIT_CONFIG_GLOBAL=/dev/null on a box with no global hook
// config hides nothing, which is every hermetic test fixture in this repo" —
// true of a box with no global alias, and this is not one.
//
// WHAT IS NOT DONE HERE. The rule is not narrowed. A carve-out keyed on the
// repo the commit is AIMED at (a -C target outside this checkout) would narrow
// an enforcement line, which wants the operator's eye (ADR 0025) and a
// security-lane verify before it closes (the routing note on the bead); and
// the way through needs neither. Nor is the
// whole package stepped out from behind the gates the way internal/posse's
// TestMain does (herdr_test.go, `os.Setenv("PATH", PathOutsideGates(""))`,
// rangerhq-8sd) — cmd/posse's PATH leak is a LIVE DEFECT PINNED GREEN in
// gateschainpathleak_qa_test.go, declined by the operator triage sweep of
// 2026-09-04 as "test hygiene at three QA sites with no observed failure", and
// that pin's own prophecy is this bead: "the day a crew deny covers one of
// those verbs, the three sites go red or, worse, green on a refusal the probes
// cannot tell from the gate chain's own." Converting the sites that went red
// answers the day without reversing the ruling or deleting that pin.
//
// WHY THIS PIN, AND WHY IT IS NOT A GREP. The sites that are red and the sites
// that are green spell the same thing — `exec.Command("git", …)` — and there
// are 66 bound ones in the tree's test corpus. What separates them is whether
// the child's env still carries the launcher's pin, which is one hop away from
// the exec call: through a local `env` slice, or through a package helper like
// pfEnv. And what excuses 45 of them is a single line in one TestMain, in
// another file. A census keyed on any of those three alone reports the wrong
// set, so the rule below reads all three:
//
//  1. the child's binary is the bare literal "git";
//  2. its `.Env` is assigned, in the function that exec'd it, by an identifier
//     match (protocolchildpath_qa_test.go's rule — two functions in one file
//     routinely both call their child `cmd`), to a value that either names
//     GIT_CONFIG_GLOBAL / GIT_CONFIG_SYSTEM / GIT_CONFIG_NOSYSTEM, or reaches
//     no os.Environ() at all and so carries nothing the launcher set; and
//  3. its package has not stepped out from behind the gates for the whole test
//     binary — derived from the package's own TestMain, never listed here, so
//     the day internal/posse's line goes away this pin names its 45 sites
//     instead of going quiet.
//
// There is no exemption roster, for gittempdirroot_qa_test.go's reason:
// complying is one token (`gitOutsideGates(t)` instead of `"git"`), the helper
// is already in both packages, and an exemption mechanism here would only ever
// be used to re-open the class. A function that strips the gates from $PATH
// for itself — internal/treepins/bdhookcommit_qa_test.go's shape — is read as
// compliant, because that is the third honest spelling of the fix and not an
// excuse for skipping it.
//
// Reading the SOURCE rather than building it covers all three suite arms at
// once: parser.ParseFile applies no build constraints, so the untagged,
// posse_arm2 and posse_arm3 files are all read by one run.
//
// NOT CLAIMED: that the 39 sites in internal/posse are safe on any other box,
// or that a seat whose PATH leads with a gates dir is the only way to reach
// the shim. Both are properties of the environment, not of this tree, which is
// exactly why arm 1 counts them rather than converting them.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// fgpUnpinVars are the environment names that change which files git reads as
// global or system scope. A fixture that sets one of these has overridden the
// launcher's pin whatever value it chose: /dev/null hides the operator's
// aliases, and any other path is a different file from the pinned one.
//
// HOME and XDG_CONFIG_HOME are deliberately NOT here. They move global scope
// only when GIT_CONFIG_GLOBAL is absent, and on a posse seat it never is, so a
// fixture that moves HOME and inherits the rest is allowed — MEASURED, see the
// table above. Listing them would flag the whole green half.
var fgpUnpinVars = []string{"GIT_CONFIG_GLOBAL=", "GIT_CONFIG_SYSTEM=", "GIT_CONFIG_NOSYSTEM="}

// fgpSite is one bare-name git child, with the verdict on its environment.
type fgpSite struct {
	file, fn string
	line     int
	why      string // "" when the child still carries the launcher's pin
	immune   bool   // its package steps out from behind the gates in TestMain
	local    bool   // its own function strips the gates from $PATH
}

func (s fgpSite) String() string {
	return fmt.Sprintf("%s:%d (%s): %s", s.file, s.line, s.fn, s.why)
}

// fgpTestFiles is every _test.go file in the tree, in a stable order. The cwd
// is the repo root (configdirfence_test.go's TestMain chdirs there), so the
// paths are repo-relative and the reading is of the tree and not of a package.
func fgpTestFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			n := d.Name()
			if path != "." && (strings.HasPrefix(n, ".") || n == "node_modules" || n == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if rel := filepath.ToSlash(path); strings.HasSuffix(rel, "_test.go") {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree for test sources: %v", err)
	}
	sort.Strings(out)
	return out
}

// fgpExecBin returns the expression naming the binary an exec.Command or
// exec.CommandContext call runs, and whether the call is one.
func fgpExecBin(call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "exec" {
		return nil, false
	}
	switch sel.Sel.Name {
	case "Command":
		if len(call.Args) > 0 {
			return call.Args[0], true
		}
	case "CommandContext":
		if len(call.Args) > 1 {
			return call.Args[1], true
		}
	}
	return nil, false
}

// fgpStringLit reports whether e is the string literal want.
func fgpStringLit(e ast.Expr, want string) bool {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return false
	}
	v, err := strconv.Unquote(bl.Value)
	return err == nil && v == want
}

// fgpNamesUnpin reports whether any string literal under n starts with one of
// fgpUnpinVars.
func fgpNamesUnpin(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		bl, ok := x.(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			return true
		}
		v, err := strconv.Unquote(bl.Value)
		if err != nil {
			return true
		}
		for _, p := range fgpUnpinVars {
			if strings.HasPrefix(v, p) {
				found = true
			}
		}
		return true
	})
	return found
}

// fgpReachesEnviron reports whether n calls os.Environ or cmd.Environ — the
// two ways a child env is BUILT FROM the launcher's rather than replacing it.
// A value that reaches neither carries nothing posse set, the pin included.
func fgpReachesEnviron(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		call, ok := x.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Environ" {
			found = true
		}
		return true
	})
	return found
}

// fgpStripsPath reports whether n strips the gates from $PATH for its own
// process — `os.Setenv("PATH", PathOutsideGates(…))` or the t.Setenv spelling
// of it. Used twice: over a TestMain, for a whole package's immunity, and over
// one function, for its own.
func fgpStripsPath(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		call, ok := x.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Setenv" {
			return true
		}
		if !fgpStringLit(call.Args[0], "PATH") {
			return true
		}
		ast.Inspect(call.Args[1], func(y ast.Node) bool {
			switch v := y.(type) {
			case *ast.Ident:
				if v.Name == "PathOutsideGates" {
					found = true
				}
			case *ast.SelectorExpr:
				if v.Sel.Name == "PathOutsideGates" {
					found = true
				}
			}
			return true
		})
		return true
	})
	return found
}

// fgpScan reads the whole test corpus and returns every bound bare-name git
// child, with the verdict on its environment.
//
// The env verdict is resolved ONE HOP, which is what the three sites this bead
// converted needed: policypins' `env` is a local slice holding the override,
// pidfence's is an append over a package helper (pfEnv), and adrcensus' is a
// local replacement slice naming no override at all. A hop further would start
// reading other packages' helpers, and nothing in the tree needs it.
func fgpScan(t *testing.T, files []string) []fgpSite {
	t.Helper()
	fset := token.NewFileSet()
	parsed := make(map[string]*ast.File, len(files))
	// Package-scope helpers, per directory: a same-package call in an env
	// expression is expanded through these.
	helpers := map[string]map[string]*ast.FuncDecl{}
	immune := map[string]bool{}
	for _, rel := range files {
		f, err := parser.ParseFile(fset, rel, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", rel, err)
		}
		parsed[rel] = f
		dir := filepath.ToSlash(filepath.Dir(rel))
		if helpers[dir] == nil {
			helpers[dir] = map[string]*ast.FuncDecl{}
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Body == nil {
				continue
			}
			helpers[dir][fd.Name.Name] = fd
			// Immunity is a property of the package's TestMain and of
			// nothing else: a per-test strip is credited per function below.
			if fd.Name.Name == "TestMain" && fgpStripsPath(fd.Body) {
				immune[dir] = true
			}
		}
	}

	byKey := map[string]fgpSite{}
	var order []string
	for _, rel := range files {
		dir := filepath.ToSlash(filepath.Dir(rel))
		for _, d := range parsed[rel].Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			// The bound bare-name git children of this function.
			kids := map[string]int{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != len(as.Rhs) {
					return true
				}
				for i, rhs := range as.Rhs {
					call, ok := rhs.(*ast.CallExpr)
					if !ok {
						continue
					}
					bin, ok := fgpExecBin(call)
					if !ok || !fgpStringLit(bin, "git") {
						continue
					}
					if id, ok := as.Lhs[i].(*ast.Ident); ok && id.Name != "_" {
						kids[id.Name] = fset.Position(call.Pos()).Line
					}
				}
				return true
			})
			if len(kids) == 0 {
				continue
			}
			// Every local assignment, for the one hop.
			local := map[string][]ast.Expr{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != len(as.Rhs) {
					return true
				}
				for i, l := range as.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						local[id.Name] = append(local[id.Name], as.Rhs[i])
					}
				}
				return true
			})
			stripsHere := fgpStripsPath(fd.Body)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for i, l := range as.Lhs {
					sel, ok := l.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "Env" {
						continue
					}
					recv, ok := sel.X.(*ast.Ident)
					if !ok || i >= len(as.Rhs) {
						continue
					}
					line, ok := kids[recv.Name]
					if !ok {
						continue // not a git child this function exec'd
					}
					// The value, plus one hop through local identifiers and
					// same-package helper calls.
					nodes := []ast.Node{as.Rhs[i]}
					ast.Inspect(as.Rhs[i], func(y ast.Node) bool {
						switch v := y.(type) {
						case *ast.Ident:
							for _, r := range local[v.Name] {
								nodes = append(nodes, r)
							}
						case *ast.CallExpr:
							if id, ok := v.Fun.(*ast.Ident); ok {
								if h := helpers[dir][id.Name]; h != nil {
									nodes = append(nodes, h.Body)
								}
							}
						}
						return true
					})
					unpins, environ := false, false
					for _, nd := range nodes {
						if fgpNamesUnpin(nd) {
							unpins = true
						}
						if fgpReachesEnviron(nd) {
							environ = true
						}
					}
					why := ""
					switch {
					case unpins:
						why = "its env overrides GIT_CONFIG_GLOBAL / GIT_CONFIG_SYSTEM, which is the pin the launcher set"
					case !environ:
						why = "its env replaces the launcher's rather than building on os.Environ(), so it carries no GIT_CONFIG_GLOBAL at all"
					}
					// One finding per CHILD, not per .Env assignment: a
					// function that rebinds the same identifier — cmd/posse's
					// adrcensus fixture assigns `cmd.Env` three times — would
					// otherwise report its one git child three times, at a
					// line none of those assignments is on.
					key := fmt.Sprintf("%s:%d:%s", rel, line, recv.Name)
					if prev, seen := byKey[key]; seen {
						if prev.why == "" {
							prev.why = why
						}
						prev.local = prev.local || stripsHere
						byKey[key] = prev
						continue
					}
					byKey[key] = fgpSite{
						file: rel, line: line, fn: fd.Name.Name, why: why,
						immune: immune[dir], local: stripsHere,
					}
					order = append(order, key)
				}
				return true
			})
		}
	}
	out := make([]fgpSite, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

// Arm 1 — the class, over the live corpus.
func TestQAEveryFixtureGitThatUnpinsGitsConfigRunsOutsideTheGates(t *testing.T) {
	t.Parallel()

	files := fgpTestFiles(t)
	sites := fgpScan(t, files)

	// The positive witnesses. Every arm below asserts an absence, and a walk
	// that has quietly stopped reading says exactly what a clean tree says.
	// The floors sit under the live counts (MEASURED 2026-11-10 at
	// ranger-base-xa2e3's commit: 768 test files, 66 bound bare-name git
	// children, 39 of them handed an env that unpins git's config scope — all
	// 39 in internal/posse, every one excused by that package's TestMain, and
	// 0 anywhere else. The unit is one finding per CHILD and not per `.Env`
	// assignment, so these read lower than a per-assignment census of the same
	// tree, which double-counted every rebound identifier).
	if len(files) < 400 {
		root, _ := repoRoot()
		t.Fatalf("the walk read %d _test.go files from %q — the corpus this pin censuses is not there", len(files), root)
	}
	if len(sites) < 40 {
		t.Fatalf("found %d bound bare-name git children in %d test files, and 66 sat in the tree on 2026-11-10 — the scan is not reading this tree's fixtures, and arm 2 is then green over whatever they do",
			len(sites), len(files))
	}
	var unpinned, immuneUnpinned int
	for _, s := range sites {
		if s.why == "" {
			continue
		}
		unpinned++
		if s.immune {
			immuneUnpinned++
		}
	}
	if unpinned < 20 {
		t.Errorf("only %d of %d bare-name git children were read as unpinning git's config scope, and 39 did on 2026-11-10.\n"+
			"  That half of the rule is what tells the refused fixture from the allowed one — a count this low means the env hop has stopped resolving and this pin now excuses the class it is a census of.",
			unpinned, len(sites))
	}
	if immuneUnpinned < 20 {
		t.Errorf("only %d unpinning child(ren) were read as excused by their package's TestMain, and 39 were on 2026-11-10 — internal/posse's `os.Setenv(\"PATH\", PathOutsideGates(\"\"))` (herdr_test.go, rangerhq-8sd) is the one line holding them, and a derivation that has stopped finding it would report them as violations rather than as what they are.",
			immuneUnpinned)
	}
	t.Logf("%d test files, %d bound bare-name git children, %d unpinning git's config scope, %d of those excused by their package's TestMain",
		len(files), len(sites), unpinned, immuneUnpinned)

	// Arm 1 proper.
	for _, s := range sites {
		if s.why == "" || s.immune || s.local {
			continue
		}
		t.Errorf("%s\n"+
			"  exec.Command resolves the bare name \"git\" against THIS process's $PATH — never cmd.Env — and a dispatched seat's PATH leads with its own gates bin dir, so this fixture's git is posse's L1 shim. With the launcher's GIT_CONFIG_GLOBAL dropped or overridden, the shim's global-scope compare differs from the launcher's and it refuses the commit outright: hookskip:config-env, ADR 0079, ranger-base-r3pri. The fixture never runs and the failure reads as drift in whatever the test was about (ranger-base-xa2e3).\n"+
			"  Resolve the binary past the gates — `gitBin := gitOutsideGates(t)`, then exec that — which both cmd/posse and internal/treepins already carry for this reason (ranger-base-ebhdx). Keep the hermetic env exactly as it is: it is not the part that was wrong.",
			s)
	}
}

// Arm 2 — the control. A census that cannot say no reports green over a tree
// it has stopped reading, and the shapes below are the ones that actually
// separate this class: the same exec call is a violation or not depending on
// an env expression one hop away and a line in another file.
func TestQAFixtureGitCensusCanStillSayNo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		src     string
		testMan string // a TestMain for the same synthetic package, or ""
		want    bool
	}{{
		name: "the policy-pins shape: an override in a local env slice",
		src: `func f(t *testing.T) {
			env := append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd := exec.Command("git", "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = env
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "the pidfence shape: an override appended over a package helper",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", root, "commit", "-qm", "seed", "--", ".")
			cmd.Env = append(pfEnv(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "the adrcensus shape: a replacement env naming no override",
		src: `func f(t *testing.T) {
			env := []string{"PATH=" + outside, "HOME=" + repo}
			cmd := exec.Command("git", "-C", repo, "commit", "-qm", "seed", "--", "f")
			cmd.Env = env
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "the fix: the binary resolved past the gates",
		src: `func f(t *testing.T) {
			gitBin := gitOutsideGates(t)
			env := append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd := exec.Command(gitBin, "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = env
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "the green half: HOME moved, the launcher's pin still carried",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+xdg)
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "the same violation, excused by the package's TestMain",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		testMan: `func TestMain(m *testing.M) {
			os.Setenv("PATH", PathOutsideGates(""))
			os.Exit(m.Run())
		}`,
		want: false,
	}, {
		name: "a TestMain that strips something else is not immunity",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		testMan: `func TestMain(m *testing.M) {
			os.Setenv("CLAUDE_CONFIG_DIR", dir)
			os.Exit(m.Run())
		}`,
		want: true,
	}, {
		name: "a function that strips the gates for itself is compliant",
		src: `func f(t *testing.T) {
			t.Setenv("PATH", posse.PathOutsideGates(""))
			cmd := exec.Command("git", "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "an .Env on another function's child is not this child's env",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.CombinedOutput()
		}
		func g(t *testing.T) {
			cmd := exec.Command(posseBin, "gates")
			cmd.Env = []string{"PATH=" + outside}
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "a git child with no .Env at all carries the launcher's pin",
		src: `func f(t *testing.T) {
			t.Setenv("HOME", home)
			cmd := exec.Command("git", "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "CommandContext is read on its second argument",
		src: `func f(t *testing.T) {
			cmd := exec.CommandContext(ctx, "git", "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = []string{"PATH=" + outside}
			cmd.CombinedOutput()
		}`,
		want: true,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			write := func(name, body string) string {
				p := filepath.Join(dir, name)
				src := "package p\n\n" + body + "\n"
				if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
				return p
			}
			files := []string{write("a_test.go", c.src)}
			if c.testMan != "" {
				files = append(files, write("main_test.go", c.testMan))
			}
			sort.Strings(files)
			got := false
			for _, s := range fgpScan(t, files) {
				if s.why != "" && !s.immune && !s.local {
					got = true
				}
			}
			if got != c.want {
				t.Errorf("violation=%v, want %v — for %q", got, c.want, c.name)
			}
		})
	}
}
