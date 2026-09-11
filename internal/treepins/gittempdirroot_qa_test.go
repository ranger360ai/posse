package treepins

// QA pins for ranger-base-0gp7d — the door under the git-init temp root.
//
// THE DEFECT, unchanged since ranger-base-mhxnh: a test that `git init`s
// inside a plain t.TempDir() inherits t.TempDir's own cleanup, which is a
// single os.RemoveAll that t.Errorf's on failure. On this box that goes red
// on ENOTEMPTY under .git/objects with the subtest body already returned
// clean — something outside the process writes there between RemoveAll's
// walk and its final rmdir. internal/posse's gitTempDir (commitwall_qa_test
// .go) is the tolerant wrapper: it registers a retrying removal AFTER
// t.TempDir's, and cleanups run LIFO, so the tolerant pass runs first and
// t.TempDir's own later attempt finds nothing to complain about.
//
// WHY THIS FILE EXISTS AND NOT A NINTH FIX. This is the 9th bead on one
// defect class (mhxnh -> 5onr5 -> r3t1g -> h2iib -> rgipo -> o1u5p -> 1xpm7
// -> mryfe -> 0gp7d). Every pass before it converted the call sites a grep
// had found and stopped, and each grep matched a SHAPE rather than the
// class:
//
//   - ranger-base-h2iib's wanted "git" and "init" on one source line, so
//     `for _, args := range [][]string{{"init", "-q"}, ...}` (beadloss_test
//     .go) never matched, and neither did any of the fixture runners that
//     take the root as their own first argument — mustGit(t, repo, "init",
//     …), siMust, qblGit, srGit, run(dir, "init", …).
//   - the fix at 2115efb1 converted the three sites its bead named and
//     verified them under `-tags posse_arm3`, which does not COMPILE the
//     posse_arm2 file carrying the two sites ranger-base-1xpm7 then found.
//   - ranger-base-mryfe's census read "a `:= t.TempDir()` within the 25
//     source lines above an init", which crosses function boundaries in both
//     directions: it counted gates_test.go's TestPrePushHook, already
//     converted, off a t.TempDir() belonging to the function above it, and
//     it could not see a root arriving from a helper 26 lines up or in
//     another file at all. It reported 33; the rule below found 84 sites in
//     59 init-running functions, plus 7 more in the helpers those functions
//     take a root from — 91 conversions (MEASURED 2026-09-10, darwin arm64
//     go1.26.5, at 26090db3).
//
// So the deliverable of this bead is not the 91 conversions. It is the rule
// that makes conversion 92 impossible to forget, and the rule has to be
// stated over the CLASS, which is three properties and no shapes at all:
//
//  1. Detection is over-broad ON PURPOSE. Any `"init"` string literal in an
//     argument list or at the head of a string composite literal counts as
//     running an init subcommand — `exec.Command("git", …)`, a fixture
//     runner, a [][]string table, and `bd("init", …)` alike. The only thing
//     subtracted is a commit MESSAGE (`-m`/`-qm`/`-F` immediately before),
//     because that is a different word, not a different shape. An init this
//     over-counts costs one tolerant wrapper and buys nothing; an init it
//     under-counts is the next bead in the chain.
//  2. The root is resolved by SCOPE, not by position. A function that runs
//     an init must register a tolerant removal for every temp root it
//     allocates — t.TempDir() and os.MkdirTemp both count as allocations,
//     and gitTempDir(t) is one of each, so it nets to zero. Anywhere in the
//     body: no line window, so the two directions mryfe's 25-line lookback
//     got wrong are both gone.
//  3. And one hop out, because scope alone still misses the root that
//     arrives from a helper — which is not hypothetical: cageinnerlive_test
//     .go's liveCageRepo git-inits a root from shortTempDir, an os.MkdirTemp
//     with a plain, error-discarding os.RemoveAll cleanup two functions
//     away. A function that runs an init must also not CALL a test-source
//     function that is short a tolerant removal and returns a string.
//
// There is deliberately no exemption list. Complying is one token — write
// gitTempDir(t) instead of t.TempDir() — and the tolerance is a strict
// widening (it retries, and only reports when retries do not clear), so a
// root that was never going to race pays nothing for it. An exemption
// mechanism here would only ever be used to re-open the class.
//
// WHERE IT LIVES. internal/treepins, not internal/posse: the subject is
// internal/posse's whole test corpus, ~950s is past what a seat spends in
// one foreground call, and a pin nobody's `-run` filter names is unreachable
// exactly when it matters (ranger-base-rulbl's class, doored in
// treewidedoor_qa_test.go). treepins is the package `go test ./...` runs
// whole in seconds, so this needs no Makefile door of its own — and being
// outside internal/posse keeps it outside treewidedoor's derived class,
// which is why no door variable names it.
//
// Reading the SOURCE rather than building it is also what covers all three
// arms at once: parser.ParseFile applies no build constraints, so the
// untagged, posse_arm2 and posse_arm3 files are all read by one run. That is
// 2115efb1's escape closed mechanically rather than by remembering.
//
// NOT CLAIMED: that the concurrent writer is fixed, or that every converted
// root was genuinely exposed. The writer is still open (Spotlight/mds over
// the runner's TMPDIR and an outliving git subprocess are the two live
// leads), the race is timing-dependent, and the repro here is structural —
// this scan — exactly as it was in ranger-base-mhxnh's original report.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// gtdTolerant is internal/posse's tolerant wrapper, by name — the spelling
// a caller writes instead of t.TempDir().
const gtdTolerant = "gitTempDir"

// gtdRemove is the tolerant removal itself, and it is what the rule is
// actually stated over: gitTempDir is just the common way to register one.
// A root allocated by hand — cageinnerlive_test.go's shortTempDir needs an
// os.MkdirTemp under /tmp, because a unix socket has to fit in 104 bytes and
// the go test temp dir alone spends most of that — is tolerant when its own
// cleanup goes through this, and the rule must be able to say so without an
// exemption.
const gtdRemove = "removeAllTolerant"

// gtdFunc is one function declared in internal/posse/*_test.go, read for
// the three properties the rule above is stated over.
type gtdFunc struct {
	name  string
	where string
	// inits: positions of `"init"` argv literals that are not a commit
	// message — property 1.
	inits []string
	// bare: positions of temp-root allocations whose cleanup is not
	// tolerant — t.TempDir() and os.MkdirTemp — property 2.
	bare []string
	// calls: every package-level function this body calls, by name, at the
	// first position it is called from — property 3's edge.
	calls map[string]string
	// tolerated: how many tolerant removals this body registers. Compared
	// against len(bare) rather than treated as a flag, so a function that
	// tolerates one root and allocates two is still short by one.
	tolerated int
	// provider: this function allocates a bare root AND hands a string back,
	// so calling it is the same exposure as allocating one — property 3's
	// node.
	provider bool
}

// gtdRead runs the detector over already-parsed files. Both arms below go
// through it: arm 2 feeds it planted source, so the control is reading the
// same instrument arm 1 reports from.
func gtdRead(fset *token.FileSet, files []*ast.File) []*gtdFunc {
	var out []*gtdFunc
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			f := &gtdFunc{
				name:  fn.Name.Name,
				where: gtdAt(fset, fn.Pos()),
				calls: map[string]string{},
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					switch fun := x.Fun.(type) {
					case *ast.Ident:
						if fun.Name == gtdRemove {
							f.tolerated++
						}
						if _, seen := f.calls[fun.Name]; !seen {
							f.calls[fun.Name] = gtdAt(fset, x.Pos())
						}
					case *ast.SelectorExpr:
						if id, ok := fun.X.(*ast.Ident); ok {
							switch {
							case id.Name == "t" && fun.Sel.Name == "TempDir",
								id.Name == "os" && fun.Sel.Name == "MkdirTemp":
								f.bare = append(f.bare, gtdAt(fset, x.Pos()))
							}
						}
					}
					for i, arg := range x.Args {
						s, ok := gtdStr(arg)
						if !ok || s != "init" {
							continue
						}
						// A commit message is a different word, not a
						// different shape: `git commit -qm init`.
						if i > 0 {
							if prev, ok := gtdStr(x.Args[i-1]); ok && (prev == "-m" || prev == "-qm" || prev == "-F") {
								continue
							}
						}
						f.inits = append(f.inits, gtdAt(fset, arg.Pos()))
					}
				case *ast.CompositeLit:
					// `[][]string{{"init", "-q"}, …}` — the shape
					// ranger-base-h2iib's one-line grep could not see.
					if len(x.Elts) > 0 {
						if s, ok := gtdStr(x.Elts[0]); ok && s == "init" {
							f.inits = append(f.inits, gtdAt(fset, x.Pos()))
						}
					}
				}
				return true
			})
			f.provider = f.short() > 0 && gtdReturnsString(fn)
			out = append(out, f)
		}
	}
	return out
}

// short is how many of this function's own temp roots have no tolerant
// removal registered for them. gitTempDir itself reads as zero — it
// allocates one bare root and registers one tolerant removal over it, which
// is exactly what it is for.
func (f *gtdFunc) short() int {
	if n := len(f.bare) - f.tolerated; n > 0 {
		return n
	}
	return 0
}

// gtdViolations is the rule itself, over the functions gtdRead produced:
// every function that runs an init, paired with why its root is not
// tolerant. One line per reason, sorted, so a red names every site at once
// rather than one per run.
func gtdViolations(funcs []*gtdFunc) []string {
	// Keyed by name, because the call edge is a name — and the three arms can
	// each declare their own <name>, so the worst of them wins rather than
	// the last one parsed. An arm-local helper that is short a removal is the
	// exposure whichever arm compiles it.
	providers := map[string]*gtdFunc{}
	for _, f := range funcs {
		if !f.provider {
			continue
		}
		if prev, ok := providers[f.name]; !ok || f.short() > prev.short() {
			providers[f.name] = f
		}
	}
	var bad []string
	for _, f := range funcs {
		if len(f.inits) == 0 {
			continue
		}
		if n := f.short(); n > 0 {
			bad = append(bad, fmt.Sprintf("%s (%s) runs an init (%s) and allocates %d temp root(s) with no tolerant removal, from %s — use %s(t), or register %s over the root by hand",
				f.name, f.where, f.inits[0], n, strings.Join(f.bare, ", "), gtdTolerant, gtdRemove))
		}
		for name, at := range f.calls {
			p, ok := providers[name]
			if !ok || name == f.name {
				continue
			}
			bad = append(bad, fmt.Sprintf("%s (%s) runs an init (%s) and takes a root from %s at %s — %s allocates %d untolerated temp root(s) (%s) and returns a string; give %s a %s(t) root",
				f.name, f.where, f.inits[0], name, at, name, p.short(), strings.Join(p.bare, ", "), name, gtdTolerant))
		}
	}
	sort.Strings(bad)
	return bad
}

// gtdParsePosse reads internal/posse/*_test.go with no build constraints
// applied, which is what puts all three arms under one run.
func gtdParsePosse(t *testing.T) (*token.FileSet, []*ast.File) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("internal", "posse", "*_test.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no test files found under internal/posse: %v", err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, file)
	}
	return fset, files
}

// Arm 1: the class, over the live corpus.
func TestQAEveryGitInitInThePosseTestsSitsOnATolerantRoot(t *testing.T) {
	t.Parallel()
	fset, files := gtdParsePosse(t)
	funcs := gtdRead(fset, files)

	// Floors. Every arm below reports absence, and absence is exactly what a
	// scan that has quietly stopped reading also reports. These sit well
	// under the live figures (MEASURED 2026-09-10: 470 files, 4237 functions,
	// 111 of them running an init, 61 bare-root providers) and exist only to
	// separate "no violations" from "no input".
	initFns, providers := 0, 0
	for _, f := range funcs {
		if len(f.inits) > 0 {
			initFns++
		}
		if f.provider {
			providers++
		}
	}
	if len(files) < 200 || len(funcs) < 1000 {
		t.Fatalf("parsed %d files and %d functions under internal/posse — the corpus this pin reads is not there", len(files), len(funcs))
	}
	if initFns < 60 {
		t.Fatalf("only %d functions run an init subcommand — the detector has stopped seeing the thing it is a census of", initFns)
	}
	if providers < 30 {
		t.Fatalf("only %d functions allocate a bare temp root and return it — the one-hop arm is deriving nothing", providers)
	}
	t.Logf("%d files, %d functions, %d running an init, %d bare-root providers", len(files), len(funcs), initFns, providers)

	for _, v := range gtdViolations(funcs) {
		t.Errorf("%s", v)
	}
}

// Arm 2: the control. A census that cannot say no is a census that reports
// green over a tree it has stopped reading — and this class has already been
// closed eight times by a reading that was narrower than it looked. Each
// case below is a shape that ESCAPED a previous pass, planted on purpose.
func TestQAGitInitRootCensusCanStillSayNo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want bool
	}{{
		name: "tolerant root is fine",
		src: `func f(t *testing.T) {
			repo := gitTempDir(t)
			exec.Command("git", "-C", repo, "init", "-q").Run()
		}`,
		want: false,
	}, {
		name: "bare root, git and init on one line",
		src: `func f(t *testing.T) {
			repo := t.TempDir()
			exec.Command("git", "-C", repo, "init", "-q").Run()
		}`,
		want: true,
	}, {
		name: "argv table — ranger-base-h2iib's escape",
		src: `func f(t *testing.T) {
			repo := t.TempDir()
			for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "t"}} {
				exec.Command("git", append([]string{"-C", repo}, args...)...).Run()
			}
		}`,
		want: true,
	}, {
		name: "fixture runner takes the root as its own argument",
		src: `func f(t *testing.T) {
			repo := t.TempDir()
			mustGit(t, repo, "init", "-q", "-b", "main", ".")
		}`,
		want: true,
	}, {
		name: "root and init 26 lines apart — past mryfe's window",
		src: `func f(t *testing.T) {
			repo := t.TempDir()
			` + strings.Repeat("_ = repo\n", 30) + `
			mustGit(t, repo, "init", "-q")
		}`,
		want: true,
	}, {
		name: "closure captures the root",
		src: `func f(t *testing.T) {
			dir := t.TempDir()
			run := func(args ...string) { exec.Command("git", args...).Run() }
			run("init", "-b", "main")
		}`,
		want: true,
	}, {
		name: "root arrives from a helper — the one-hop arm",
		src: `func helper(t *testing.T) string {
			dir, _ := os.MkdirTemp("/tmp", "x")
			t.Cleanup(func() { os.RemoveAll(dir) })
			return dir
		}
		func f(t *testing.T) {
			dir := helper(t)
			gitRun(t, dir, "init", "-b", "main")
		}`,
		want: true,
	}, {
		name: "helper that hands back a tolerant root is fine",
		src: `func helper(t *testing.T) string {
			return gitTempDir(t)
		}
		func f(t *testing.T) {
			dir := helper(t)
			gitRun(t, dir, "init", "-b", "main")
		}`,
		want: false,
	}, {
		name: "a commit message is not an init",
		src: `func f(t *testing.T) {
			repo := t.TempDir()
			exec.Command("git", "-C", repo, "commit", "-qm", "init").Run()
		}`,
		want: false,
	}, {
		name: "a bare root with no init at all is not this pin's business",
		src: `func f(t *testing.T) {
			home := t.TempDir()
			_ = home
		}`,
		want: false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "planted_test.go", "package posse\n"+tc.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse planted source: %v", err)
			}
			got := gtdViolations(gtdRead(fset, []*ast.File{file}))
			if (len(got) > 0) != tc.want {
				t.Fatalf("violations = %v; want flagged = %v", got, tc.want)
			}
		})
	}
}

// Arm 3: the wrapper is reachable from every arm. The fix at 2115efb1 was
// verified under `-tags posse_arm3` alone and left two sites in the
// posse_arm2 file it never compiled; the mirror of that mistake is putting
// gitTempDir itself behind a tag, which would compile in one arm and leave
// the other two with no wrapper to convert to. Arm 1 reads source and so
// could not notice.
func TestQATheTolerantTempDirWrapperCompilesInEveryArm(t *testing.T) {
	t.Parallel()
	fset, files := gtdParsePosse(t)
	found := ""
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name != gtdTolerant {
				continue
			}
			at := fset.Position(fn.Pos())
			if found != "" {
				t.Fatalf("%s is declared twice, at %s and %s:%d — one of them is a tagged copy, and a copy is how the arms drift apart", gtdTolerant, found, at.Filename, at.Line)
			}
			found = fmt.Sprintf("%s:%d", at.Filename, at.Line)
			for _, group := range file.Comments {
				for _, c := range group.List {
					if strings.HasPrefix(c.Text, "//go:build") {
						t.Errorf("%s is declared in %s, which carries %q — the other arms have no tolerant wrapper to convert to", gtdTolerant, at.Filename, c.Text)
					}
				}
			}
		}
	}
	if found == "" {
		t.Fatalf("internal/posse's tests no longer declare %s — every arm of this pin is stated over it", gtdTolerant)
	}
	t.Logf("%s declared once, untagged, at %s", gtdTolerant, found)
}

func gtdAt(fset *token.FileSet, p token.Pos) string {
	at := fset.Position(p)
	return fmt.Sprintf("%s:%d", at.Filename, at.Line)
}

func gtdStr(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

func gtdReturnsString(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, r := range fn.Type.Results.List {
		if id, ok := r.Type.(*ast.Ident); ok && id.Name == "string" {
			return true
		}
	}
	return false
}
