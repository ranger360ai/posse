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
//  1. the child's binary resolves to the bare name "git" — spelled inline, or
//     reached one hop through an identifier assigned in the same function
//     (`gitBin := "git"`), because the shape this pin MANDATES is itself
//     indirect and an indirected bare name is the same defect
//     (ranger-base-l0bep). `exec.LookPath("git")` is read as that same bare
//     name, because it IS the bare-name resolution: os/exec runs it for you
//     when you pass a name with no separator, against the same live $PATH.
//     MEASURED 2026-11-10 from this gated seat (linux 7.2.15, go1.27.3),
//     `exec.LookPath("git")` and `exec.Command("git").Path` both
//     /home/.../state/gates/<persona>/bin/git, byte-identical — an absolute
//     path, which is why it reads as compliant, and the gate, which is the
//     whole subject (ranger-base-ffojl);
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
// exactly why arm 1 counts them rather than converting them. Nor that a
// binary reached through MORE than one hop is read — a function parameter, a
// package-level var, a second identifier — which is the same depth the `.Env`
// half has and is limited for the same reason: a hop further starts reading
// other packages' helpers, and nothing in the tree needs it. What the one hop
// buys is symmetry, so neither half of the subject can be restored by moving
// a token out of the call (ranger-base-l0bep). An in-function `var` or
// `const` binding IS read, at BOTH nodes of that one hop, and neither was
// until two beads closed in sequence: a declaration is an *ast.DeclStmt and
// not an assignment, so `var gitBin = "git"` resolved to nothing where
// `gitBin := "git"` reds (ranger-base-o57rc), and `var cmd =
// exec.Command(gitBin, …)` recorded no CHILD at all — which skipped the
// function WHOLE, so the binary half of the verdict never ran however well it
// had just been taught to resolve (ranger-base-6rr2e; MEASURED 2026-11-11 at
// policypins_qa_test.go's ppRepo, reading ok with a caught `var gitBin =
// "git"` beside it as the positive control). This paragraph read as a
// complete enumeration while neither was true (ranger-base-tfqlr finding 2).
// Both walks that read a BINDING have both arms now; the `.Env` walk needs
// only the one, because `var cmd.Env = …` is not Go and a write to a field
// is always an assignment.
//
// NOR that `exec.LookPath` is read anywhere but in that one hop, on a literal
// "git", bound to a local identifier the call then execs. Three shapes in this
// tree are deliberately outside it. `_, err := exec.LookPath("git")` is the
// availability probe and names no binary — 105 of this corpus's 114 LookPath
// calls are that, so a census that does not bind the BINARY's own slot reports
// 114 for a population of 9 (MEASURED 2026-11-10). `exec.LookPath(someVar)`
// resolves a name this pin cannot read, and is left to whoever bound the
// variable. And a LookPath value carried into a STRUCT FIELD and exec'd as
// `f.gitBin` — hooksredirect_qa_test.go's hrFix, whose own field comment
// reads "git by ABSOLUTE path", which is what makes this class worth a rule —
// is two hops and a selector, past the depth both halves stop at. All three
// are in internal/posse, immune for the same reason its other 39 sites are
// (ranger-base-ffojl).

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

// fgpKid is one bare-name git child of a function, as the first pass over
// that function's body found it: the line of its exec call, the identifier
// the bare name was reached through ("" when the call spells the literal
// itself), and whether that identifier was bound by exec.LookPath rather than
// by the literal — which changes nothing about the verdict and everything
// about what the fix has to say (ranger-base-ffojl).
type fgpKid struct {
	line     int
	via      string
	lookPath bool
}

// fgpSite is one bare-name git child, with the verdict on its environment.
type fgpSite struct {
	file, fn string
	line     int
	why      string // "" when the child still carries the launcher's pin
	via      string // the identifier the bare name was reached through, "" when inline
	lookPath bool   // that identifier was bound by exec.LookPath("git")
	immune   bool   // its package steps out from behind the gates in TestMain
	local    bool   // its own function strips the gates from $PATH
}

func (s fgpSite) String() string {
	switch {
	case s.lookPath:
		return fmt.Sprintf("%s:%d (%s): its binary is %s, bound by exec.LookPath(\"git\") — an absolute path, and the gate's — and %s", s.file, s.line, s.fn, s.via, s.why)
	case s.via != "":
		return fmt.Sprintf("%s:%d (%s): its binary is the bare name \"git\" bound to %s, and %s", s.file, s.line, s.fn, s.via, s.why)
	}
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

// fgpLookPathGit reports whether e is `exec.LookPath("git")` — the one call in
// this tree that resolves a bare name against the LIVE $PATH and hands back
// what it found there.
//
// It is read as the bare name itself, and that is not a widening of this
// pin's subject but the recognition of it. os/exec resolves a name with no
// path separator by calling LookPath on it, so `exec.Command("git", …)` and
// `gitBin, _ := exec.LookPath("git"); exec.Command(gitBin, …)` are the SAME
// operation against the SAME $PATH, differing only in whether the answer is
// stored first. MEASURED 2026-11-10 from a gated seat on this box (linux
// 7.2.15, go1.27.3): `exec.LookPath("git")` and `exec.Command("git").Path`
// both returned the same <home>/state/gates/<persona>/bin/git under this
// box's own gates root, byte-identical — the seat's own L1 shim.
//
// What makes it worth its own accept test rather than a note is that the
// result LOOKS like compliance from every angle the reader has: it is an
// absolute path, it was computed rather than typed, and one site in this tree
// documents the shape as the house form for exactly that ("gitBin string //
// git by ABSOLUTE path — ADR 0052 M2's own form",
// internal/posse/hooksredirect_qa_test.go). `gitOutsideGates(t)` exists
// because LookPath is not safe here: it walks posse.PathOutsideGates("") by
// hand and takes the first git that is in no gates dir
// (internal/treepins/gitoutsidegates_test.go, cmd/posse/gateschain_qa_test.go).
//
// It is NOT the broader reading — "a call is a bare name unless it is
// gitOutsideGates" — which inverts the rule's default and would read every
// unresolvable binary as a violation. This matches one call, on the literal
// "git", and nothing else (ranger-base-ffojl).
func fgpLookPathGit(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "LookPath" {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "exec" {
		return false
	}
	return fgpStringLit(call.Args[0], "git")
}

// fgpBareGit reports whether e names the bare binary "git", the identifier it
// was reached through ("" when the literal is in the call itself), and
// whether that identifier was bound by exec.LookPath rather than by the
// literal.
//
// ONE HOP, by the same rule and to the same depth as the `.Env` half below:
// an *ast.Ident first argument is resolved through the bindings of that name
// in the exec'ing function — its assignments and its `var`/`const`
// declarations alike — and the accept test applied to each is the SAME
// fgpStringLit the inline spelling gets. So `gitBin := "git"` followed by
// `exec.Command(gitBin, …)` is read as what it is, and `gitBin :=
// gitOutsideGates(t)` — a call, not a literal — is not. ANY assignment
// resolving to the literal is enough: a name bound to the bare word on even
// one path can reach the shim on that path.
//
// This exists because the fix this pin prescribes is itself an indirection
// (`gitBin := gitOutsideGates(t)`, then exec the variable), so every converted
// site reads `exec.Command(<ident>, …)`. Without the hop, an author editing
// one of those sites — or copying the pattern into a package with no
// gitOutsideGates helper — is one token from the defect and this census stays
// green over it. MEASURED 2026-11-10 by the verify lane (ranger-base-b079k):
// a converted site reverted to `gitBin := "git"` read this census ok 0.187s
// while the tests it protects failed with ranger-base-xa2e3's original
// refusal. The same mutation reds it now, naming the binding
// (ranger-base-l0bep; docs/notes.d/ranger-base-l0bep.md has both arms).
//
// Two accept tests are applied to each assignment, not one: fgpStringLit, for
// the literal the inline spelling gets, and fgpLookPathGit, for the one call
// that is that literal's own resolution. Any OTHER call — gitOutsideGates(t)
// foremost — is not a bare name and never becomes one here
// (ranger-base-ffojl).
func fgpBareGit(e ast.Expr, local map[string][]ast.Expr) (bare bool, via string, lookPath bool) {
	if fgpStringLit(e, "git") {
		return true, "", false
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return false, "", false
	}
	// The literal first, so a name bound to it on one path and to LookPath on
	// another is reported as the plainer of the two.
	for _, rhs := range local[id.Name] {
		if fgpStringLit(rhs, "git") {
			return true, id.Name, false
		}
	}
	for _, rhs := range local[id.Name] {
		if fgpLookPathGit(rhs) {
			return true, id.Name, true
		}
	}
	return false, "", false
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
// BOTH halves of the verdict are resolved ONE HOP, over the same map of local
// assignments. The env hop is what the three sites this bead converted needed:
// policypins' `env` is a local slice holding the override, pidfence's is an
// append over a package helper (pfEnv), and adrcensus' is a local replacement
// slice naming no override at all. The BINARY hop is fgpBareGit's, added
// because the shape this pin mandates is an indirection and the two halves
// must be read to the same depth or the class re-opens through the shallower
// one (ranger-base-l0bep). A hop further, on either half, would start reading
// other packages' helpers, and nothing in the tree needs it.
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
			// Every local assignment, for the one hop. Collected BEFORE the
			// children, because the binary half of the verdict reads it too:
			// a child bound to `gitBin` is only recognised once `gitBin`'s
			// own assignment has been seen, and Go does not require that
			// assignment to come first in source order (a closure defined
			// above the binding, for one).
			//
			// TWO assignment shapes, because the only spelling of this
			// class's third form is the second of them: `gitBin, err :=
			// exec.LookPath("git")` is n-from-1, and a map built under
			// len(Lhs) == len(Rhs) alone never holds that binding at all, so
			// the identifier would resolve to nothing however the accept test
			// read it (ranger-base-ffojl). For n-from-1 only Lhs[0] is
			// recorded: it is the slot the VALUE lands in for every `v, err
			// := f()` in this corpus, and mapping `err` to f() would be a
			// false statement about what that name holds.
			//
			// AND the same two shapes spelled as a `var` DECLARATION inside
			// that function, which is an *ast.DeclStmt and not an assignment
			// at all. Until ranger-base-o57rc this walk read AssignStmt
			// alone, so `var gitBin = "git"` and `var gitBin, _ =
			// exec.LookPath("git")` resolved to nothing and read ok at a site
			// where `gitBin := "git"` reds and names the binding — MEASURED
			// 2026-11-10 at policypins_qa_test.go's ppRepo, one spelling per
			// arm, with the two `:=` arms as the positive control
			// (ranger-base-tfqlr finding 2). The pin's prose above claims
			// this depth — "the assignments to that name in the exec'ing
			// function" — and its NOT CLAIMED paragraph enumerates what is
			// outside it: a function parameter, a package-level var, a second
			// identifier. An in-function `var` is in neither list, so the
			// enumeration read as complete and was not. The accept tests are
			// untouched: a ValueSpec's value meets the same fgpStringLit and
			// fgpLookPathGit the inline spelling meets, so this is the
			// recognizer reaching a spelling, not the wider reading
			// ranger-base-ffojl declined. A `const` spec is the same node and
			// the same defect one token away, so it is read too.
			local := map[string][]ast.Expr{}
			// bind records n names from n values pairwise, or n names from a
			// single value — in which case only the FIRST name is taken, for
			// the reason above. An empty name is a target that binds no local
			// identifier (`f.gitBin`, `a[i]`); it holds its slot so the
			// pairwise case stays aligned, and is recorded under no name.
			bind := func(names []string, values []ast.Expr) {
				if len(values) == 0 {
					return
				}
				if len(names) == len(values) {
					for i, nm := range names {
						if nm != "" {
							local[nm] = append(local[nm], values[i])
						}
					}
					return
				}
				if len(values) == 1 && len(names) > 0 && names[0] != "" {
					local[names[0]] = append(local[names[0]], values[0])
				}
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch s := n.(type) {
				case *ast.AssignStmt:
					names := make([]string, len(s.Lhs))
					for i, l := range s.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							names[i] = id.Name
						}
					}
					bind(names, s.Rhs)
				case *ast.DeclStmt:
					gd, ok := s.Decl.(*ast.GenDecl)
					if !ok || (gd.Tok != token.VAR && gd.Tok != token.CONST) {
						return true
					}
					for _, sp := range gd.Specs {
						vs, ok := sp.(*ast.ValueSpec)
						if !ok {
							continue
						}
						names := make([]string, len(vs.Names))
						for i, id := range vs.Names {
							names[i] = id.Name
						}
						bind(names, vs.Values)
					}
				}
				return true
			})
			// The bound bare-name git children of this function, inline or
			// one hop away.
			//
			// TWO BINDING SHAPES, the same two the local map above reads and
			// for the same reason: a child spelled `var cmd =
			// exec.Command(gitBin, …)` is an *ast.DeclStmt and not an
			// assignment, so until ranger-base-6rr2e it entered no `kids`
			// entry at all. That is worse here than at the binary node,
			// because `len(kids) == 0` below skips the function WHOLE: the
			// env half is never looked at, and the binary half never runs
			// however well ranger-base-o57rc had just taught it to resolve
			// the name. MEASURED 2026-11-11 at policypins_qa_test.go's
			// ppRepo: `var cmd = exec.Command(gitBin, …)` read ok with a
			// caught `var gitBin = "git"` beside it as the positive control,
			// so the miss was this walk and nothing else (ranger-base-tfqlr
			// finding 2, filed as this bead). The accept tests are untouched:
			// a ValueSpec's value meets the same fgpExecBin and fgpBareGit
			// the assignment's Rhs meets. MEASURED the same day over the
			// whole corpus, 779 test files: 362 exec children bound by an
			// assignment and 0 by a declaration, so this closes before the
			// class reaches it rather than naming a live site, exactly as
			// ranger-base-l0bep's hop did.
			//
			// A `const` spec is deliberately NOT read here, where the local
			// map reads one: a constant expression cannot hold an *exec.Cmd,
			// so `const cmd = exec.Command(…)` does not compile and the arm
			// would be dead code wearing the look of symmetry. At the binary
			// node `const gitBin = "git"` compiles and is the defect, which
			// is why that map has it.
			//
			// PAIRWISE ONLY, in both shapes, which is where this walk differs
			// from the local map: n-from-1 is read there because the third
			// spelling of a BINARY is `gitBin, err := exec.LookPath("git")`,
			// and no exec.Command call binds a child and an error. Taking
			// Lhs[0] of an arbitrary n-from-1 call here would record a child
			// this function never exec'd.
			kids := map[string]fgpKid{}
			recordKids := func(names, values []ast.Expr) {
				if len(names) != len(values) {
					return
				}
				for i, v := range values {
					call, ok := v.(*ast.CallExpr)
					if !ok {
						continue
					}
					bin, ok := fgpExecBin(call)
					if !ok {
						continue
					}
					bare, via, lookPath := fgpBareGit(bin, local)
					if !bare {
						continue
					}
					if id, ok := names[i].(*ast.Ident); ok && id.Name != "_" {
						kids[id.Name] = fgpKid{
							line: fset.Position(call.Pos()).Line, via: via, lookPath: lookPath,
						}
					}
				}
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch s := n.(type) {
				case *ast.AssignStmt:
					recordKids(s.Lhs, s.Rhs)
				case *ast.DeclStmt:
					gd, ok := s.Decl.(*ast.GenDecl)
					if !ok || gd.Tok != token.VAR {
						return true
					}
					for _, sp := range gd.Specs {
						vs, ok := sp.(*ast.ValueSpec)
						if !ok {
							continue
						}
						names := make([]ast.Expr, len(vs.Names))
						for i, id := range vs.Names {
							names[i] = id
						}
						recordKids(names, vs.Values)
					}
				}
				return true
			})
			if len(kids) == 0 {
				continue
			}
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
					kid, ok := kids[recv.Name]
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
					key := fmt.Sprintf("%s:%d:%s", rel, kid.line, recv.Name)
					if prev, seen := byKey[key]; seen {
						if prev.why == "" {
							prev.why = why
						}
						prev.local = prev.local || stripsHere
						byKey[key] = prev
						continue
					}
					byKey[key] = fgpSite{
						file: rel, line: kid.line, fn: fd.Name.Name, why: why,
						via: kid.via, lookPath: kid.lookPath,
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

// ─── The ARGV half (ranger-base-pn99n, from ranger-base-ltk6x) ──────────────
//
// fixturegitpin's arm above pins ONE way a bare-name git fixture meets the
// seat's own L1 shim: an environment that drops or overrides the launcher's
// config pin (hookskip:config-env). The worked example that opened this bead
// is the OTHER way — a verb the shim refuses OUTRIGHT, whatever the
// environment: cmd/posse/version_test.go built its fixture repo with `git
// commit-tree`, which hookskip.go has refused since ranger-base-crlty
// (hookskip:plumbing-commit), and `make test` was red at arm 1 for every
// gated seat for a day while ungated CI stayed green, because that pin's own
// env check cannot see a refusal that no environment can fix.
//
// Censused 2026-11-11 over cmd/ and internal/treepins, after
// ranger-base-ltk6x's fix: ZERO sites. This census, like the one above,
// keeps it at zero rather than fixes anything live.
//
// SCOPE. Of hookskip.go's refusal classes that do not depend on the
// environment, this reads two: hookskip:plumbing-commit (commit-tree
// outright, am unless it only ends or reads a stalled one) and
// hookskip:no-verify (-n/--no-verify on the verbs that take it). It does NOT
// read hookskip:config-write (`git config`/`clone`/`init` writing a
// hook-moving key) or hookskip:template (--template, GIT_TEMPLATE_DIR):
// both need the same key-parsing hookskip.go's shell arms do (-c K=V,
// --config=K=V, a short cluster's glued value), which is a second
// implementation of that parser and not a read of a literal, and neither has
// bitten here. Nor Bash(git push:*) or Bash(git commit unless --) — PID
// denies, judged on a running seat's own argv, not on a fixture's source.
// Building those is this bead's own NOT CLAIMED paragraph's call to make,
// left for whoever reads a live site through one of them.
//
// DERIVED, NOT RETYPED (fgpHookSkipVerbs): the verb set hookskip:no-verify
// fires on, and the am exceptions, are read out of hookskip.go's own
// hookSkipNoVerifyVerbs and hookSkipAmAllowed var declarations, so a verb
// added to either there is censused here without a second edit. NOT
// derived: which of those verbs also takes the SHORT -n spelling
// (fgpArgvShortFormVerbs, below) — that split lives inside hookSkipNoVerify's
// switch statement in hookskip.go, not in a named var, and parsing a switch
// is past what this walk buys.
//
// NO ONE-HOP for the argv itself, unlike the binary and env halves above:
// the GAP this closes reads "the exec call's string-literal args", and every
// site either arm has ever found spells its verb inline
// (`exec.Command(gitBin, "-C", repo, "commit-tree", …)`). An argv assembled
// into a []string and spread in with `args...` is past this depth — the
// same boundary the other two hops stop at — and nothing in this tree does
// that today. The BINARY is still resolved through fgpBareGit's one hop,
// unchanged, because a fixture that converts its binary to
// gitOutsideGates(t) and keeps a stray commit-tree is still wrong, and the
// binary hop is what tells a live shim-reaching fixture from a scratch repo
// built with /usr/bin/git.
//
// ISOLATED FROM fgpScan ON PURPOSE. This needs only fgpBareGit's binary
// resolution and fgpStripsPath's immunity/local checks, neither of fgpScan's
// env-hop machinery — so it walks its own, duplicate, local-binding map
// (fgpArgvLocalBindings) rather than reach into fgpScan's. Four bugs in a
// row were found in that exact binding walk (ranger-base-l0bep,
// ranger-base-o57rc, ranger-base-6rr2e, ranger-base-tfqlr); sharing it would
// make a future fifth one able to move both arms' verdicts at once, and
// keeping this walk its own means it cannot.

// fgpHookSkipVerbs reads internal/posse/hookskip.go's own hookSkipNoVerifyVerbs
// and hookSkipAmAllowed var declarations, so the refused set here is derived
// from the shim's single source rather than retyped.
func fgpHookSkipVerbs(t *testing.T) (noVerify, amAllowed []string) {
	t.Helper()
	const rel = "internal/posse/hookskip.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing %s: %v", rel, err)
	}
	want := map[string]*[]string{
		"hookSkipNoVerifyVerbs": &noVerify,
		"hookSkipAmAllowed":     &amAllowed,
	}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, sp := range gd.Specs {
			vs, ok := sp.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			slot, ok := want[vs.Names[0].Name]
			if !ok {
				continue
			}
			cl, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, el := range cl.Elts {
				if bl, ok := el.(*ast.BasicLit); ok && bl.Kind == token.STRING {
					if v, err := strconv.Unquote(bl.Value); err == nil {
						*slot = append(*slot, v)
					}
				}
			}
		}
	}
	if len(noVerify) == 0 || len(amAllowed) == 0 {
		t.Fatalf("reading %s: hookSkipNoVerifyVerbs=%v hookSkipAmAllowed=%v — the shim's own var declarations moved or were renamed, and this census has nothing to derive the refused set from", rel, noVerify, amAllowed)
	}
	return noVerify, amAllowed
}

// fgpArgvShortFormVerbs are the hookSkipNoVerifyVerbs members that take the
// SHORT -n spelling too (hookskip.go's hookSkipNoVerify: commit and am spell
// it "-n, --no-verify"; merge, pull, rebase and push have only the long
// form, because -n means something else on each of them — --dry-run on
// push, --no-stat on merge/rebase — and is not refused there). NOT derived;
// see the package comment above.
var fgpArgvShortFormVerbs = map[string]bool{"commit": true, "am": true}

// fgpArg is one argument of a git child's argv, read as a string literal, or
// as "not one" when the walk cannot resolve it (a variable, a spread
// []string...) — which this half of the census treats as opaque rather than
// guess at, the safe direction when the slot might be the verb itself.
type fgpArg struct {
	s  string
	ok bool
}

// fgpArgvArgs reads the literal arguments of an exec.Command/CommandContext
// call, after the binary (fgpExecBin's own slot).
func fgpArgvArgs(call *ast.CallExpr) []fgpArg {
	start := 1
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "CommandContext" {
		start = 2
	}
	if start > len(call.Args) {
		return nil
	}
	out := make([]fgpArg, 0, len(call.Args)-start)
	for _, a := range call.Args[start:] {
		if bl, ok := a.(*ast.BasicLit); ok && bl.Kind == token.STRING {
			if v, err := strconv.Unquote(bl.Value); err == nil {
				out = append(out, fgpArg{s: v, ok: true})
				continue
			}
		}
		out = append(out, fgpArg{})
	}
	return out
}

// fgpArgvFirstPositional returns the first argument that is not -C (and its
// value) or another flag with no separate value — the shape every fixture in
// this tree spells its verb with. An unresolved slot in that position stops
// the walk rather than guesses past it.
func fgpArgvFirstPositional(args []fgpArg) (verb string, ok bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !a.ok {
			return "", false
		}
		switch {
		case a.s == "-C":
			i++
		case strings.HasPrefix(a.s, "-"):
		default:
			return a.s, true
		}
	}
	return "", false
}

// fgpArgvWhy reports why a git child's argv is refused outright, or "" when
// it is not — the ARGV counterpart to fgpScan's env-only "why".
func fgpArgvWhy(args []fgpArg, amAllowed []string, noVerifyVerbs map[string]bool) string {
	verb, ok := fgpArgvFirstPositional(args)
	if !ok {
		return ""
	}
	switch verb {
	case "commit-tree":
		return "its verb is commit-tree, which hookskip.go refuses outright (hookskip:plumbing-commit) — no pre-commit or commit-msg hook ever sees that commit"
	case "am":
		for _, a := range args {
			if !a.ok {
				continue
			}
			if a.s == "--" {
				break // the shim itself refuses at "--" too; an allowed word must come before it
			}
			for _, allow := range amAllowed {
				if a.s == allow || strings.HasPrefix(a.s, allow+"=") {
					return ""
				}
			}
		}
		return "its verb is am, which hookskip.go refuses outright (hookskip:plumbing-commit) unless its own argv only ends or reads a stalled am"
	}
	if !noVerifyVerbs[verb] {
		return ""
	}
	for _, a := range args {
		if !a.ok {
			continue
		}
		if a.s == "--no-verify" || (a.s == "-n" && fgpArgvShortFormVerbs[verb]) {
			return fmt.Sprintf("its verb %s carries %s, which hookskip.go refuses outright (hookskip:no-verify)", verb, a.s)
		}
	}
	return ""
}

// fgpArgvLocalBindings is fgpScan's local-binding walk, duplicated rather
// than shared; see the package comment above for why.
func fgpArgvLocalBindings(body *ast.BlockStmt) map[string][]ast.Expr {
	local := map[string][]ast.Expr{}
	bind := func(names []string, values []ast.Expr) {
		if len(values) == 0 {
			return
		}
		if len(names) == len(values) {
			for i, nm := range names {
				if nm != "" {
					local[nm] = append(local[nm], values[i])
				}
			}
			return
		}
		if len(values) == 1 && len(names) > 0 && names[0] != "" {
			local[names[0]] = append(local[names[0]], values[0])
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			names := make([]string, len(s.Lhs))
			for i, l := range s.Lhs {
				if id, ok := l.(*ast.Ident); ok {
					names[i] = id.Name
				}
			}
			bind(names, s.Rhs)
		case *ast.DeclStmt:
			gd, ok := s.Decl.(*ast.GenDecl)
			if !ok || (gd.Tok != token.VAR && gd.Tok != token.CONST) {
				return true
			}
			for _, sp := range gd.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok {
					continue
				}
				names := make([]string, len(vs.Names))
				for i, id := range vs.Names {
					names[i] = id.Name
				}
				bind(names, vs.Values)
			}
		}
		return true
	})
	return local
}

// fgpArgvScan returns every bound bare-name git child whose own argv names a
// verb or flag hookskip.go refuses outright.
func fgpArgvScan(t *testing.T, files []string) []fgpSite {
	t.Helper()
	noVerify, amAllowed := fgpHookSkipVerbs(t)
	noVerifyVerbs := make(map[string]bool, len(noVerify))
	for _, v := range noVerify {
		noVerifyVerbs[v] = true
	}

	fset := token.NewFileSet()
	parsed := make(map[string]*ast.File, len(files))
	immune := map[string]bool{}
	for _, rel := range files {
		f, err := parser.ParseFile(fset, rel, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", rel, err)
		}
		parsed[rel] = f
		dir := filepath.ToSlash(filepath.Dir(rel))
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if ok && fd.Recv == nil && fd.Body != nil && fd.Name.Name == "TestMain" && fgpStripsPath(fd.Body) {
				immune[dir] = true
			}
		}
	}

	var out []fgpSite
	for _, rel := range files {
		dir := filepath.ToSlash(filepath.Dir(rel))
		for _, d := range parsed[rel].Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			local := fgpArgvLocalBindings(fd.Body)
			stripsHere := fgpStripsPath(fd.Body)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				bin, ok := fgpExecBin(call)
				if !ok {
					return true
				}
				bare, via, lookPath := fgpBareGit(bin, local)
				if !bare {
					return true
				}
				why := fgpArgvWhy(fgpArgvArgs(call), amAllowed, noVerifyVerbs)
				if why == "" {
					return true
				}
				out = append(out, fgpSite{
					file: rel, line: fset.Position(call.Pos()).Line, fn: fd.Name.Name,
					why: why, via: via, lookPath: lookPath,
					immune: immune[dir], local: stripsHere,
				})
				return true
			})
		}
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
	//
	// The binary hop ranger-base-l0bep added moved NONE of these: re-read
	// 2026-11-10 at 6365a923, 773 test files, 66 / 39 / 39, identical to the
	// reading above. The indirected spelling exists nowhere in the tree today,
	// so the hop is a guard closing before the class reaches it and not a
	// finding — which is also why these floors did not need raising.
	//
	// ranger-base-ffojl's LookPath arm DID move them, and that is the one
	// reading on this page a future bead is likely to mis-predict, so it is
	// spelled out. The same corpus reads 66 / 39 / 39 without the arm and
	// 68 / 41 / 41 with it (MEASURED 2026-11-10, this box, 775 test files,
	// the two readings taken from the same tree a minute apart and the site
	// lists diffed): exactly +2, and the two are
	// internal/posse/commitwallowner_qa_test.go:448 and
	// internal/posse/hooksredirect_qa_test.go:654, both excused by their
	// package's TestMain like the other 39. No PRE-EXISTING site's verdict
	// changed — the diff of the two site lists is those two lines and nothing
	// else, which is what rules out the n-from-1 assignment form having
	// widened the ENV half it also feeds.
	//
	// The bead predicted the totals would NOT move, reasoning that the two
	// were already counted through their `.Env`. They were not, and the
	// ordering is why: the binary half gates the env half. A child whose
	// binary is not read as a bare name never enters `kids`, so its `.Env` is
	// never looked at and it is not a site at all. Anything that widens the
	// binary half therefore moves these counts, and anything that widens only
	// the env half cannot.
	//
	// The floors still did not need raising. They sit under the counts on
	// purpose, at roughly half: they are there to catch a walk that has
	// stopped reading the tree, not to pin an exact census, and a floor moved
	// up to the live number would red on every unrelated bead that converts a
	// fixture.
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
		fix := "  Resolve the binary past the gates — `gitBin := gitOutsideGates(t)`, then exec that — which both cmd/posse and internal/treepins already carry for this reason (ranger-base-ebhdx). Keep the hermetic env exactly as it is: it is not the part that was wrong."
		switch {
		case s.lookPath:
			// This one needs its own sentence. The site already holds an
			// absolute path, computed rather than typed, so every
			// prescription above reads as already done — the thing that is
			// wrong is WHICH absolute path, and nothing at the site says
			// (ranger-base-ffojl).
			fix = fmt.Sprintf("  exec.LookPath is the bare-name resolution, not an escape from it: os/exec calls it for you for any name with no path separator, so `%s` holds whatever leads this process's $PATH — the seat's own gates dir, hence the shim's absolute path. The value being absolute is why this reads as compliant and is not what makes it safe.\n"+
				"  The line to change is the BINDING: `%s := gitOutsideGates(t)`, which walks posse.PathOutsideGates(\"\") and takes the first git in no gates dir, and which both cmd/posse and internal/treepins already carry for this reason (ranger-base-ebhdx). Keep the hermetic env exactly as it is: it is not the part that was wrong.", s.via, s.via)
		case s.via != "":
			// The call already execs a variable, so the line to change is
			// the binding and not the call: say so, or the prescription
			// above reads as already done (ranger-base-l0bep).
			fix = fmt.Sprintf("  The call already execs a variable, so the line to change is the BINDING: `%s := gitOutsideGates(t)` in place of the bare name, which both cmd/posse and internal/treepins already carry for this reason (ranger-base-ebhdx). Keep the hermetic env exactly as it is: it is not the part that was wrong.", s.via)
		}
		t.Errorf("%s\n"+
			"  exec.Command resolves the bare name \"git\" against THIS process's $PATH — never cmd.Env — and a dispatched seat's PATH leads with its own gates bin dir, so this fixture's git is posse's L1 shim. With the launcher's GIT_CONFIG_GLOBAL dropped or overridden, the shim's global-scope compare differs from the launcher's and it refuses the commit outright: hookskip:config-env, ADR 0079, ranger-base-r3pri. The fixture never runs and the failure reads as drift in whatever the test was about (ranger-base-xa2e3).\n"+
			"%s",
			s, fix)
	}
}

// Arm 2 — the control. A census that cannot say no reports green over a tree
// it has stopped reading, and the shapes below are the ones that actually
// separate this class: the same exec call is a violation or not depending on
// an env expression one hop away, a BINARY expression one hop away, and a line
// in another file.
//
// Nine of the last twelve cases are the binary hop's. The first of those is
// the shape this pin was green over until ranger-base-l0bep: the bare name
// moved out of the call and into a local binding, which is one token from the
// prescribed fix and was MEASURED reproducing ranger-base-xa2e3's refusal
// while the census read ok. The last six of the nine are
// ranger-base-ffojl's — the third spelling
// (`gitBin, err := exec.LookPath("git")`) and, deliberately outnumbering it,
// the five boundaries it must NOT cross. Reading a CALL as a bare name is the
// one change here that could invert this rule's default, so the cases that
// say where it stops are the load-bearing half: the availability probe that
// binds no binary, another binary's LookPath, a LookPath of a variable, an
// n-from-1 binding to an arbitrary call, and the struct-field carry that is
// two hops out.
//
// The final three are the DECLARATION spelling of a binding — an
// *ast.DeclStmt, not an assignment — at the child node
// (ranger-base-6rr2e), at both nodes at once (which is also the only accept
// test over ranger-base-o57rc's arm, measured at a live site and not here),
// and in the PRESCRIBED form, so neither widening can be read as having
// inverted the default for the sites that already comply.
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
	}, {
		// The gap ranger-base-l0bep closed: byte for byte the shape the fix
		// mandates, with the one token that makes it the defect again.
		name: "the blind spot: the bare name moved into a local binding",
		src: `func f(t *testing.T) {
			gitBin := "git"
			env := append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd := exec.Command(gitBin, "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = env
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		// The hop must not over-match: an identifier is only a bare name
		// when it resolves to the bare literal, and an absolute path reaches
		// no $PATH and so no shim.
		name: "an absolute path in a local binding is not a bare name",
		src: `func f(t *testing.T) {
			gitBin := "/usr/bin/git"
			cmd := exec.Command(gitBin, "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		// The same binding in ANOTHER function is not this function's: the
		// hop is local, as the env hop is, so a sibling's `gitBin := "git"`
		// cannot make a compliant site read as a violation.
		name: "a bare-name binding in another function does not reach this one",
		src: `func f(t *testing.T) {
			gitBin := gitOutsideGates(t)
			cmd := exec.Command(gitBin, "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}
		func g(t *testing.T) {
			gitBin := "git"
			_ = gitBin
		}`,
		want: false,
	}, {
		// ranger-base-ffojl's shape. Two misses had to be fixed for this to
		// read: the n-from-1 AssignStmt had to enter the local map at all,
		// and the accept test had to read a CALL. Either miss alone keeps it
		// green, so a regression in either is caught here.
		name: "the third spelling: the binary bound by exec.LookPath(\"git\")",
		src: `func f(t *testing.T) {
			gitBin, err := exec.LookPath("git")
			if err != nil {
				t.Skip("no git")
			}
			cmd := exec.Command(gitBin, "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		// The availability probe, which is 105 of this corpus's 114
		// LookPath("git") calls (MEASURED 2026-11-10). It binds the error and
		// discards the path, so it names no binary and must not drag a
		// compliant site in beside it — the whole reason the rule binds the
		// BINARY's own slot and not merely "an identifier in the LHS".
		name: "the availability probe binds no binary",
		src: `func f(t *testing.T) {
			if _, err := exec.LookPath("git"); err != nil {
				t.Skip("no git")
			}
			gitBin := gitOutsideGates(t)
			cmd := exec.Command(gitBin, "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		// The accept test is keyed on the literal "git" and on nothing else:
		// another binary's LookPath is another binary's problem, and this pin
		// claims one subject.
		name: "LookPath of another binary is not this class",
		src: `func f(t *testing.T) {
			bin, err := exec.LookPath("bd")
			if err != nil {
				t.Skip("no bd")
			}
			cmd := exec.Command(bin, "list")
			cmd.Env = []string{"PATH=" + outside}
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		// A LookPath whose argument this pin cannot read is left to whoever
		// bound the argument. Reading it as a violation would be the inverted
		// default the bead declined (ranger-base-ffojl).
		name: "LookPath of a variable is not read as the bare name",
		src: `func f(t *testing.T) {
			gitBin, err := exec.LookPath(name)
			if err != nil {
				t.Skip("no " + name)
			}
			cmd := exec.Command(gitBin, "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		// The n-from-1 binding had to enter the local map for the case above
		// to read, and this is the half of that change that must NOT follow:
		// an n-from-1 binding to an arbitrary call is no more a bare name
		// than a 1-from-1 one is. The prescribed fix itself takes this shape
		// wherever the helper returns an error.
		name: "an n-from-1 binding to an arbitrary call is not a bare name",
		src: `func f(t *testing.T) {
			gitBin, err := gitOutsideGatesOrErr(t)
			if err != nil {
				t.Skip("no git outside the gates")
			}
			cmd := exec.Command(gitBin, "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		// The depth limit, stated as a test rather than only in NOT CLAIMED:
		// a LookPath value carried into a struct field and exec'd as
		// `f.gitBin` is two hops and a selector out. This is a real shape in
		// this tree (internal/posse/hooksredirect_qa_test.go's hrFix, whose
		// field comment reads "git by ABSOLUTE path"), immune there for the
		// same reason that package's other sites are. If the depth is ever
		// raised, this case is the one that must flip, deliberately.
		name: "a LookPath value carried through a struct field is past the depth",
		src: `type fix struct{ gitBin, repo string }

		func newFix(t *testing.T) *fix {
			gitBin, err := exec.LookPath("git")
			if err != nil {
				t.Skip("no git")
			}
			return &fix{gitBin: gitBin, repo: t.TempDir()}
		}

		func f(t *testing.T) {
			fx := newFix(t)
			cmd := exec.Command(fx.gitBin, "-C", fx.repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		// ranger-base-6rr2e: the same class at the CHILD node. A child bound
		// by a declaration entered no `kids` entry, so `len(kids) == 0`
		// skipped the function whole and its env was never read. The binary
		// here is the inline literal, so this case is the child walk and
		// nothing else — the same isolation the live-site measurement used.
		name: "a git child bound by an in-function `var` declaration",
		src: `func f(t *testing.T) {
			var cmd = exec.Command("git", "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		// Both halves spelled as declarations: the shape MEASURED at
		// policypins_qa_test.go's ppRepo, and the only accept test over the
		// local map's declaration arm (ranger-base-o57rc measured that one at
		// a live site and left no case here). Either walk regressing to
		// *ast.AssignStmt alone reads this green.
		name: "both the binary and the child bound by `var` declarations",
		src: `func f(t *testing.T) {
			var gitBin = "git"
			var cmd = exec.Command(gitBin, "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		// The boundary. Now that a declared child is visible, the PRESCRIBED
		// fix spelled the same way must still read as compliant: a widening
		// that cannot say no here would report every converted site the day
		// one of them is written with a `var`.
		name: "the fix spelled as declarations is still compliant",
		src: `func f(t *testing.T) {
			var gitBin = gitOutsideGates(t)
			var cmd = exec.Command(gitBin, "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			cmd.CombinedOutput()
		}`,
		want: false,
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

// Arm 3 — the ARGV class, over the live corpus. Ranger-base-ltk6x already
// converted the one worked example, so this reads as an invariant rather
// than a floor: ZERO bound bare-name git children with a refused verb in
// their own argv, measured 2026-11-11 over cmd/ and internal/treepins.
func TestQANoFixtureGitChildCallsAVerbTheShimRefusesOutright(t *testing.T) {
	t.Parallel()

	files := fgpTestFiles(t)
	if len(files) < 400 {
		root, _ := repoRoot()
		t.Fatalf("the walk read %d _test.go files from %q — the corpus this pin censuses is not there", len(files), root)
	}

	for _, s := range fgpArgvScan(t, files) {
		if s.immune || s.local {
			continue
		}
		t.Errorf("%s\n"+
			"  exec.Command resolves the bare name \"git\" against THIS process's $PATH — never cmd.Env — and a dispatched seat's PATH leads with its own gates bin dir, so this fixture's git is posse's L1 shim. The refusal below does not depend on any environment, so no hermetic env fixes it; cmd/posse/version_test.go hit this exact shape with `git commit-tree` (ranger-base-ltk6x).\n"+
			"  Resolve the binary past the gates (`gitBin := gitOutsideGates(t)`) is not the fix here — the verb itself is refused. Build the fixture a different way: `git apply` then `git commit` in place of `git am`, update-ref or a fast-forward in place of `git commit-tree`, and drop the -n/--no-verify the fixture does not need.",
			s)
	}
}

// Arm 4 — the ARGV census's own control, same shape as arm 2 above: the
// cases that separate "refused outright" from everything that is not,
// including the two negative controls that matter most — -n meaning
// --dry-run on push and --no-stat on merge, never --no-verify — because
// those are exactly where an over-eager derivation would invert this
// census's own default and start refusing fixtures that do nothing wrong.
func TestQAArgvRefusalCensusCanStillSayNo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		src     string
		testMan string
		want    bool
	}{{
		name: "commit-tree is refused outright",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "commit-tree", tree)
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "a bare am is refused outright",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "am", patch)
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "am --abort ends a stalled am and is allowed",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "am", "--abort")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "am --show-current-patch=diff reads a stalled am and is allowed",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "am", "--show-current-patch=diff")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "am stops at -- and refuses: a mailbox literally named --abort buys no pass",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "am", "--", "--abort")
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "commit -n is refused outright",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "commit", "-qm", "x", "-n")
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "commit --no-verify is refused outright",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "commit", "--no-verify", "-qm", "x")
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "a plain commit with no -n/--no-verify is not refused",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "commit", "-qm", "x", "--", ".")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		// The load-bearing negative control: push -n is --dry-run, never
		// --no-verify, and must stay green or this census manufactures its
		// own over-refusal on its own subject.
		name: "push -n is --dry-run, not --no-verify, and is not refused",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "push", "-n", "origin", "HEAD")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "push --no-verify is refused outright",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "push", "--no-verify", "origin", "HEAD")
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		// The second negative control: merge -n is --no-stat on that verb.
		name: "merge -n is --no-stat, not --no-verify, and is not refused",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "merge", "-n", "branch")
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "rebase --no-verify is refused outright",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "rebase", "--no-verify", "main")
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "the binary resolved past the gates is compliant whatever its verb",
		src: `func f(t *testing.T) {
			gitBin := gitOutsideGates(t)
			cmd := exec.Command(gitBin, "-C", repo, "commit-tree", tree)
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "the bare name one hop through a local binding is still read",
		src: `func f(t *testing.T) {
			gitBin := "git"
			cmd := exec.Command(gitBin, "-C", repo, "commit-tree", tree)
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "CommandContext is read on its own argument offset",
		src: `func f(t *testing.T) {
			cmd := exec.CommandContext(ctx, "git", "-C", repo, "commit-tree", tree)
			cmd.CombinedOutput()
		}`,
		want: true,
	}, {
		name: "a verb carried through a variable is past this depth and not read",
		src: `func f(t *testing.T) {
			verb := "commit-tree"
			cmd := exec.Command("git", "-C", repo, verb, tree)
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "another binary's commit-tree-shaped argv is not this class",
		src: `func f(t *testing.T) {
			cmd := exec.Command(posseBin, "-C", repo, "commit-tree", tree)
			cmd.CombinedOutput()
		}`,
		want: false,
	}, {
		name: "excused by the package's TestMain",
		src: `func f(t *testing.T) {
			cmd := exec.Command("git", "-C", repo, "commit-tree", tree)
			cmd.CombinedOutput()
		}`,
		testMan: `func TestMain(m *testing.M) {
			os.Setenv("PATH", PathOutsideGates(""))
			os.Exit(m.Run())
		}`,
		want: false,
	}, {
		name: "a function that strips the gates for itself is compliant",
		src: `func f(t *testing.T) {
			t.Setenv("PATH", posse.PathOutsideGates(""))
			cmd := exec.Command("git", "-C", repo, "commit-tree", tree)
			cmd.CombinedOutput()
		}`,
		want: false,
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
			for _, s := range fgpArgvScan(t, files) {
				if !s.immune && !s.local {
					got = true
				}
			}
			if got != c.want {
				t.Errorf("violation=%v, want %v — for %q", got, c.want, c.name)
			}
		})
	}
}
