package treepins

// QA pins for ranger-base-qbg7c — a Go or Python temp root made straight
// inside a shared temp directory names a namespace scripts/reap-test-tmp.sh
// has a class for.
//
// THE DEFECT IS A CENSUS, NOT A ROOT, and this is the fourth bead in the
// family to say so and the first to say it about these two languages.
// ranger-base-7hyn8 fixed cmd/coldexec's door build dir; ranger-base-4dw7b
// fixed cmd/testparallel's tool dir and scripts/pattern-kill-census.py's
// fixture root; ranger-base-w8n6w wrote mktemproot_qa_test.go, which holds
// `mktemp -d` under scripts/ and said in its own head comment that a root
// created by Go's os.MkdirTemp was "pinned nowhere here". Three sites found
// one at a time, by hand, over two days, and then a pin for one language of
// three. This file is the other two.
//
// WHAT THE PREFIX BUYS, since it is easy to read `posse-` as decoration.
// Every one of these roots HAS a remover — a t.Cleanup, a TestMain, a
// try/finally, a `with` block — so none of them is accumulating. What no
// remover covers is the half that matters on this box: a SIGKILL and a
// `-timeout` panic run no finally, no defer and no t.Cleanup, and this box
// kills suites (AGENTS.md, "Ending anything" — eleven confirmed suite kills
// across five personas). The root such a run leaves is then collectable only
// by scripts/reap-test-tmp.sh, whose posse-scratch class is the glob
// `posse-*` at a 120m grace; every other class of that table names Go's
// `$WORK`, t.TempDir's `Test*[0-9]`, the harness's own scratchpads, or a dead
// fixture name kept to drain a pre-fix backlog. A root outside the namespace
// is matched by no row, so it is unreapable at any age, by any spelling of
// that script, forever — and /tmp here is a 14G tmpfs with 1,048,576 inodes
// shared by every seat as one uid, which hit 100% of its INODES on
// 2026-11-11 with the byte gauge reading 29% (ranger-base-vw1cp,
// ranger-base-l23nb).
//
// THE RULE. A live `os.MkdirTemp`, `tempfile.mkdtemp` or
// `tempfile.TemporaryDirectory` whose PARENT is a shared temp directory
// carries a prefix beginning `posse-`. There is deliberately no exemption
// list: complying is six characters, the namespace constrains nothing but the
// name, and an exemption mechanism here would only ever be used to re-open
// the class. (mktemproot_qa_test.go next door has one, for a reason that does
// not arise in either of these languages: a shell root whose darwin PATH has
// to be hardcoded against a 104-byte sun_path cap.)
//
// WHAT COUNTS AS A SHARED TEMP PARENT (trTempParents). The TMPDIR-defaulting
// spelling — Go's `os.MkdirTemp("", …)` and `os.TempDir()`, Python's `dir=`
// omitted — plus the three absolute spellings of the same idea: `/tmp`,
// `/private/tmp` (what darwin's /tmp resolves to, and the one the reaper's
// own default root reaches there) and `/var/tmp`. The reaper's default roots
// are /tmp and this shell's $TMPDIR when that is somewhere else
// (reap-test-tmp.sh, "Default:"), so the first four spellings all land
// somewhere it looks. `/var/tmp` is NOT one of them and is in this list
// anyway: it is a shared temp directory on both hosts, a leak there is
// box-wide the same way, and the prefix is the whole of what would let
// `reap-test-tmp.sh --root /var/tmp` collect one. The tree's single root
// there (wallUngrantedDir, internal/posse/wall_helpers_test.go) already
// spells `posse-ungranted-`, so including it costs nothing today and closes
// the hole rather than waiting for a bead to find it.
//
// WHAT IS OUT OF SCOPE, and this is ranger-base-qbg7c's third deliverable —
// the decision said once, rather than an exemption row per site:
//
//   - A ROOT WHOSE PARENT IS NOT A SHARED TEMP DIRECTORY is covered by its
//     parent and is not this pin's business. The census reads the parent
//     argument and scores anything else out, which is the rule and not a
//     waiver. Three live roots are in that position and every one of them
//     was audited rather than assumed: internal/posse/app_test.go's
//     `abbrev-probe-` and internal/posse/readingslog_qa_test.go's `rl-repo-`
//     both sit under this test binary's temp HOME, which IS
//     `posse-testhome-*` (internal/posse/herdr_test.go:198), and
//     internal/posse/landedcheck.go's `run-` sits under the launcher's state
//     dir, which is not in any temp root at all. Collecting the parent takes
//     the child with it; naming the parent's namespace again in the child
//     would be a second place for it to drift.
//   - A TEMP FILE, not a directory: `os.CreateTemp`, `tempfile.mkstemp`,
//     `tempfile.NamedTemporaryFile`, `mktemp` with no `-d`. Every class in
//     the reaper's table finds candidates with `-type d`, so a leaked FILE is
//     collected by no row whatever it is called and a `posse-` prefix on one
//     would be a claim of coverage that does not exist. This is the same line
//     mktemproot_qa_test.go draws, drawn for the same reason.
//   - `t.TempDir()`, which the reaper's `Test*[0-9]` class already globs, and
//     which is the right answer wherever the length of the path does not
//     matter. Three of the roots this bead renamed are MkdirTemp precisely
//     because a unix socket under them has to fit in 104 or 108 bytes.
//   - SHELL. `mktemp -d` under scripts/ is mktemproot_qa_test.go's subject
//     behind `make scripts-check`, and the two pins are deliberately not one
//     reader: that one is line-based because a shell parse is not something
//     to write here, this one is a go/parser census plus a line reader for
//     Python. `mktemp -d` elsewhere in the tree is covered by NEITHER —
//     docs/adr/*.probe.sh holds a dozen of them, five bare, which are frozen
//     evidence records and a different decision (ranger-base-ym8pl, filed
//     with the census that found them and the reason they were left).
//
// MEASURED 2026-11-12 (linux, this box), after this bead's renames: the walk
// reads 1026 .go files in 8 package directories and 15 .py files; the census
// reports 15 in-scope Go roots and 7 in-scope Python roots, and every one of
// the 22 is inside the namespace. 0.30s, which is the whole cost of parsing
// those 1026 files. Before the renames EIGHT of those 22 were outside it —
// three Go (`bwh`, `posse6so`, `bwt7wdb7-`) and five Python
// (`nodb-set-defer-selftest-`, `nodb-defer-`, `nodb-defer-selftest-`,
// `page-currency-selftest-`, `d3-reader-eval.`) — beside two darwin shell
// roots this file does not read (`pselfclose.`, `pghost.`, renamed under the
// same bead and rostered next door). Each of the eight was planted back and
// re-run: the census reds naming the site and the line, eight for eight, and
// the arm is green over the tree as it stands.
//
// THE ELEVENTH SITE IS WHY THE PARENT IS RESOLVED THROUGH IDENTIFIERS.
// ranger-base-qbg7c's own hand census named ten sites and the reader found
// one more: internal/posse/bwraptmp_qa_test.go's `bwt7wdb7-`, whose parent is
// the package const `bwrapTmpMount` — `"/tmp"`, declared in a different file
// of that package (wallseatcache.go). A grep for `MkdirTemp("/tmp"` cannot
// see that line and neither can a grep for `MkdirTemp(""`. So trStrings
// resolves a name to its literal across the whole package, and drops any name
// the package binds to two different literals rather than guessing which one
// a call site meant. That resolution is what makes the census stricter: a
// parent it cannot read is scored OUT of scope, which is the one direction in
// which this reader is loose, and it is said here rather than half-handled. A
// PREFIX it cannot read is scored RED, because there the strict answer is the
// safe one and the remedy is one line at the call site.
//
// AND THE AMBIGUITY RULE COST A LIVE SITE ON ITS FIRST RUN, which is the best
// evidence of what that looseness is: internal/posse/wall_helpers_test.go's
// `/var/tmp` root held its parent in a local `const base`, and internal/posse
// also binds `base` to a seven-line yaml literal in systemdunit_test.go, so
// the name was dropped and the root was scored out of scope — 14 Go roots
// where a hand count says 15. The fix is at the call site and not in the
// reader: that const is `varTmpRoot` now, the census reads 15, and the
// alternative (resolving per function rather than per package) would have
// cost the cross-file resolution that found site eleven in the first place.
//
// (Site three of the ten was not a site: internal/treepins/
// gittempdirroot_qa_test.go:373's `os.MkdirTemp("/tmp", "x")` is TEXT inside
// a raw string literal, one of that pin's planted Go fixtures, and creates
// nothing. A go/parser census is immune to it by construction — a string
// literal yields no CallExpr — which is the second reason this half is an AST
// read and not a grep.)

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// trRequired is the one prefix a shared-temp root may carry: the reaper's
// posse-scratch glob is `posse-*`, WITH the hyphen. One of this bead's own
// ten sites was `posse6so`, which looks like it is inside the namespace and
// is matched by no class at all.
const trRequired = "posse-"

// trTempParents — the parent directories that make a root this pin's
// business. The empty string is Go's "use $TMPDIR else /tmp"; the three
// absolute spellings are the same idea written out. See the head comment for
// why /var/tmp is in here and what the reaper does and does not reach.
var trTempParents = map[string]bool{
	"":             true,
	"/tmp":         true,
	"/private/tmp": true,
	"/var/tmp":     true,
}

// trRoot is one temp-root creation the census found in scope.
type trRoot struct {
	File   string // repo-relative, slash-separated
	Line   int
	Code   string // the call as the source spells it, for the diagnostic
	Parent string // how the parent was read, for the diagnostic
	Prefix string // the resolved prefix
	Known  bool   // false when the prefix is an expression the reader cannot read
}

// At returns the `path:line` a diagnostic leads with.
func (r trRoot) At() string { return fmt.Sprintf("%s:%d", r.File, r.Line) }

// ─── the Go half ─────────────────────────────────────────────────────────────

// trStrings resolves string-literal names across one package: every `const`
// or `var` spec and every `:=`/`=` assignment in the package's files whose
// right-hand side is a single string literal, package level and function
// level alike.
//
// A name the package binds to two DIFFERENT literals is dropped rather than
// resolved to either. A flat map is what lets the parent resolve across files
// — `bwrapTmpMount` is declared in wallseatcache.go and used in
// bwraptmp_qa_test.go — and dropping the ambiguous names is what keeps that
// flatness from inventing an answer when two functions both have a local
// called `dir`.
func trStrings(files map[string]*ast.File) map[string]string {
	out := map[string]string{}
	bad := map[string]bool{}
	bind := func(name string, rhs ast.Expr) {
		if name == "" || name == "_" {
			return
		}
		lit, ok := trLit(rhs)
		if !ok {
			return
		}
		if prev, seen := out[name]; seen && prev != lit {
			bad[name] = true
			return
		}
		out[name] = lit
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		ast.Inspect(files[p], func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.ValueSpec:
				for i, name := range v.Names {
					if i < len(v.Values) {
						bind(name.Name, v.Values[i])
					}
				}
			case *ast.AssignStmt:
				if len(v.Lhs) == len(v.Rhs) {
					for i := range v.Lhs {
						if id, ok := v.Lhs[i].(*ast.Ident); ok {
							bind(id.Name, v.Rhs[i])
						}
					}
				}
			}
			return true
		})
	}
	for name := range bad {
		delete(out, name)
	}
	return out
}

// trLit unquotes a string literal, and nothing else.
func trLit(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// trStr resolves an expression to the whole string it names: a literal, a
// name trStrings bound, or a `+` chain of those.
func trStr(e ast.Expr, env map[string]string) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		return trLit(v)
	case *ast.Ident:
		s, ok := env[v.Name]
		return s, ok
	case *ast.ParenExpr:
		return trStr(v.X, env)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		l, lok := trStr(v.X, env)
		r, rok := trStr(v.Y, env)
		if !lok || !rok {
			return "", false
		}
		return l + r, true
	}
	return "", false
}

// trHead resolves the LEADING text of an expression — the part a prefix check
// needs. `"posse-probe-" + rt.Name + "-"` is a prefix inside the namespace
// even though its tail is a runtime value (internal/posse/runtimeprobe.go),
// so the rule is stated over what the string STARTS with and not over what it
// is.
func trHead(e ast.Expr, env map[string]string) (string, bool) {
	switch v := e.(type) {
	case *ast.ParenExpr:
		return trHead(v.X, env)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		return trHead(v.X, env)
	}
	return trStr(e, env)
}

// trIsTempDirCall reports whether e is `os.TempDir()`, which is the other
// spelling of "straight into $TMPDIR".
func trIsTempDirCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	return ok && ewdIsSel(call.Fun, "os", "TempDir")
}

// trGoPkg reads one package directory's files and returns every in-scope
// os.MkdirTemp in them.
//
// parser.ParseFile applies no build constraints, so the untagged, posse_arm2
// and posse_arm3 files of internal/posse are all read by one call — which is
// what keeps this census out of the escape 2115efb1 fell into, where a fix
// was verified under one tag and the sites under another were never compiled
// (gittempdirroot_qa_test.go's head comment tells that story).
func trGoPkg(files []string) ([]trRoot, error) {
	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}
	for _, p := range files {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		parsed[p] = f
	}
	env := trStrings(parsed)
	var out []trRoot
	paths := make([]string, 0, len(parsed))
	for p := range parsed {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		f := parsed[p]
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !ewdIsSel(call.Fun, "os", "MkdirTemp") || len(call.Args) != 2 {
				return true
			}
			parent, inScope := "", false
			switch {
			case trIsTempDirCall(call.Args[0]):
				parent, inScope = "os.TempDir()", true
			default:
				if s, ok := trStr(call.Args[0], env); ok && trTempParents[s] {
					parent, inScope = strconv.Quote(s), true
				}
			}
			if !inScope {
				return true
			}
			pos := fset.Position(call.Lparen)
			prefix, known := trHead(call.Args[1], env)
			out = append(out, trRoot{
				File:   filepath.ToSlash(p),
				Line:   pos.Line,
				Code:   trExprText(call, fset),
				Parent: parent,
				Prefix: prefix,
				Known:  known,
			})
			return true
		})
	}
	return out, nil
}

// trExprText renders a call the way a diagnostic can print it: the function
// and its two argument expressions, source-spelled, with no newlines.
func trExprText(call *ast.CallExpr, fset *token.FileSet) string {
	var parts []string
	for _, a := range call.Args {
		parts = append(parts, trOneLine(a, fset))
	}
	return "os.MkdirTemp(" + strings.Join(parts, ", ") + ")"
}

// trOneLine prints one expression. A literal and a name print themselves; an
// expression the reader did not resolve prints its own source span, which is
// what tells the author which line to change.
func trOneLine(e ast.Expr, fset *token.FileSet) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		return v.Value
	case *ast.Ident:
		return v.Name
	case *ast.CallExpr:
		return trOneLine(v.Fun, fset) + "(…)"
	case *ast.SelectorExpr:
		return trOneLine(v.X, fset) + "." + v.Sel.Name
	case *ast.BinaryExpr:
		return trOneLine(v.X, fset) + " " + v.Op.String() + " " + trOneLine(v.Y, fset)
	case *ast.ParenExpr:
		return "(" + trOneLine(v.X, fset) + ")"
	}
	return fmt.Sprintf("<%T at %s>", e, fset.Position(e.Pos()))
}

// ─── the Python half ─────────────────────────────────────────────────────────

// trPyCall matches a directory-making tempfile call, as a word so that
// neither `mkstemp` nor a sentence about `mkdtemp` in prose is read as one.
// The `(` is required, which is also what keeps the f-string in
// scripts/pattern-kill-census.py's own assertion ("handing mkdtemp another
// literal reds here") out of the census.
var trPyCall = regexp.MustCompile(`\b(?:tempfile\.)?(mkdtemp|TemporaryDirectory)\s*\(`)

// trPyKwarg reads one keyword argument's value: a single-quoted or
// double-quoted literal, or a bare name.
var trPyKwarg = func(kw string) *regexp.Regexp {
	return regexp.MustCompile(`\b` + kw + `\s*=\s*(?:(?:"((?:[^"\\]|\\.)*)")|(?:'((?:[^'\\]|\\.)*)')|([A-Za-z_][A-Za-z0-9_]*))`)
}

var (
	trPyPrefix = trPyKwarg("prefix")
	trPyDir    = trPyKwarg("dir")
)

// trPyAssign matches a module-level `NAME = "literal"`, which is how
// scripts/pattern-kill-census.py holds its own prefix (SELFTEST_DIR_PREFIX).
// Anchored at column 0 so a local inside a function is not read as one — a
// local is bound at the call site's own indentation and a reader that took
// both would have the same collision problem the Go half solves by dropping
// ambiguous names.
var trPyAssign = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(?:(?:"((?:[^"\\]|\\.)*)")|(?:'((?:[^'\\]|\\.)*)'))\s*(?:#.*)?$`)

// trPyRoots reads one .py file and returns every in-scope mkdtemp /
// TemporaryDirectory in it.
//
// LINE-BASED AND LOOSE IN THE SAFE DIRECTION, the way mktemproot_qa_test.go's
// shell reader is. A line whose stripped text starts with `#` is prose and
// creates nothing. Everything else that names one of the two calls counts,
// including a line the interpreter would never reach; over-counting costs the
// author six characters and under-counting is the next bead in this family.
// All 7 live in-scope calls open and close on one line (MEASURED
// 2026-11-12), and a call whose arguments run past the line is read to the
// end of the line, which can only lose a `prefix=` and therefore only ever
// reds.
func trPyRoots(path string) ([]trRoot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	env := map[string]string{}
	bad := map[string]bool{}
	for _, line := range lines {
		if m := trPyAssign.FindStringSubmatch(line); m != nil {
			val := m[2] + m[3]
			if prev, seen := env[m[1]]; seen && prev != val {
				bad[m[1]] = true
				continue
			}
			env[m[1]] = val
		}
	}
	for name := range bad {
		delete(env, name)
	}

	var out []trRoot
	for i, line := range lines {
		code := strings.TrimSpace(line)
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		loc := trPyCall.FindStringIndex(code)
		if loc == nil {
			continue
		}
		args := trPyArgs(code[loc[1]:])
		// Scope: `dir=` omitted is tempfile's own "use the default temp
		// directory", which is $TMPDIR else /tmp.
		parent, inScope := "dir= omitted", true
		if m := trPyDir.FindStringSubmatch(args); m != nil {
			val, name := m[1]+m[2], m[3]
			switch {
			case name != "":
				if s, ok := env[name]; ok && trTempParents[s] {
					parent = "dir=" + name + " (" + strconv.Quote(s) + ")"
				} else {
					inScope = false
				}
			case trTempParents[val]:
				parent = "dir=" + strconv.Quote(val)
			default:
				inScope = false
			}
		}
		if !inScope {
			continue
		}
		r := trRoot{File: filepath.ToSlash(path), Line: i + 1, Code: code, Parent: parent}
		if m := trPyPrefix.FindStringSubmatch(args); m != nil {
			if name := m[3]; name != "" {
				r.Prefix, r.Known = env[name], env[name] != ""
			} else {
				r.Prefix, r.Known = m[1]+m[2], true
			}
		} else {
			// No prefix at all. tempfile's default is `tmp`, which the
			// reaper's bare-`tmp.??????????` class globs only at mktemp's
			// template width — so this is a root outside the namespace and
			// not a root with a name someone might argue about.
			r.Prefix, r.Known = "", true
		}
		out = append(out, r)
	}
	return out, nil
}

// trPyArgs returns the argument text of a call whose opening paren has
// already been consumed, stopping at the paren that closes it. Quotes are
// tracked so a `)` inside a string literal does not end the list.
func trPyArgs(s string) string {
	depth := 1
	var quote rune
	escaped := false
	for i, c := range s {
		if escaped {
			escaped = false
			continue
		}
		switch {
		case quote != 0:
			// The flag, not a `continue` on the backslash: a bare continue
			// advances to the escaped character and then judges it, so `\'`
			// inside a single-quoted literal would close the quote and the
			// next `)` would end the argument list early.
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth == 0 {
				return s[:i]
			}
		}
	}
	return s
}

// ─── the census ──────────────────────────────────────────────────────────────

// trSources walks the tree and returns the .go files grouped by package
// directory, plus the .py files.
//
// It WALKS from a relative literal, which is what puts the pin below in the
// tree-wide class the register derives (treewidedoor_qa_test.go) — and it is
// the right reading rather than an accident of spelling: the file set is
// discovered, so a .go or .py file an unrelated bead adds joins this census
// without anyone editing it. That is the whole of what the three hand censuses
// before it did not have.
//
// `testdata` is skipped, the way ewdGoFiles skips it: a .go file under there
// is a planted fixture that no build compiles, and gittempdirroot_qa_test.go
// keeps its fixtures as raw strings rather than files for the same reason.
// MEASURED 2026-11-12: this tree holds no .go or .py file under any testdata
// directory at all, so the skip subtracts nothing today and is there for the
// day a planted fixture that does not compile would otherwise fail the
// parse.
func trSources(t *testing.T) (goPkgs map[string][]string, pys []string) {
	t.Helper()
	goPkgs = map[string][]string{}
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
		if !d.Type().IsRegular() {
			return nil
		}
		rel := filepath.ToSlash(path)
		switch {
		case strings.HasSuffix(rel, ".go"):
			dir := filepath.Dir(path)
			goPkgs[dir] = append(goPkgs[dir], path)
		case strings.HasSuffix(rel, ".py"):
			pys = append(pys, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree for Go and Python sources: %v", err)
	}
	for dir := range goPkgs {
		sort.Strings(goPkgs[dir])
	}
	sort.Strings(pys)
	return goPkgs, pys
}

// Every Go or Python temp root made straight inside a shared temp directory
// is inside posse's own temp namespace.
//
// This is the pin ranger-base-qbg7c exists for: three beads before it renamed
// the roots they found by hand and left nothing in the tree that asserts the
// property, so site eleven was found by this reader and site twelve would
// have been found by the next leak.
func TestQAEveryTempRootInTheTreeIsInPossesTempNamespace(t *testing.T) {
	t.Parallel()
	goPkgs, pys := trSources(t)

	// Reader sanity before the arms trust it. An assertion that every root
	// is in the namespace is satisfied by a walk that found none, and a walk
	// that found none is silent about it. The floors sit well under the live
	// counts (1026 .go files in 8 package directories, 15 .py files, MEASURED
	// 2026-11-12) so an ordinary landing does not move them.
	goFiles := 0
	for _, files := range goPkgs {
		goFiles += len(files)
	}
	if goFiles < 500 || len(goPkgs) < 5 {
		t.Fatalf("the walk found %d .go file(s) in %d package directory(ies) — the walk is broken, not the tree (1026 in 8, MEASURED 2026-11-12)", goFiles, len(goPkgs))
	}
	if len(pys) < 10 {
		t.Fatalf("the walk found %d .py file(s) — the walk is broken, not the tree (15, MEASURED 2026-11-12): %v", len(pys), pys)
	}

	var roots []trRoot
	dirs := make([]string, 0, len(goPkgs))
	for dir := range goPkgs {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		found, err := trGoPkg(goPkgs[dir])
		if err != nil {
			t.Errorf("reading %s: %v", dir, err)
			continue
		}
		roots = append(roots, found...)
	}
	for _, p := range pys {
		found, err := trPyRoots(p)
		if err != nil {
			t.Errorf("reading %s: %v", p, err)
			continue
		}
		roots = append(roots, found...)
	}

	for _, r := range roots {
		if !r.Known {
			t.Errorf("%s makes a temp root under a shared temp directory (%s) with a prefix this census cannot read:\n  %s\n"+
				"Spell the prefix as a string literal, or as a const this package binds to one — the census resolves a name across the whole package (trStrings) and drops a name bound to two different literals. A prefix nobody can read is a prefix nobody can check, and what is being checked is that it begins %q.",
				r.At(), r.Parent, r.Code, trRequired)
			continue
		}
		if strings.HasPrefix(r.Prefix, trRequired) {
			continue
		}
		t.Errorf("%s makes a temp root under a shared temp directory (%s) as\n  %s\n"+
			"  prefix: %q\n"+
			"  want:   a prefix beginning %q\n"+
			"Only scripts/reap-test-tmp.sh's posse-scratch class (glob `posse-*`, 120m grace) reaches a root of this shape, and its remover — a t.Cleanup, a TestMain, a try/finally — runs on no SIGKILL and no -timeout panic, which is how suites end on this box. A root outside the namespace is therefore collected by no class at any age, against a /tmp that hit 100%% of its 1,048,576 inodes on 2026-11-11. This hole has now been found by hand four times (ranger-base-7hyn8, -4dw7b, -w8n6w, -qbg7c); internal/treepins/temproot_qa_test.go is the census that was supposed to end that.",
			r.At(), r.Parent, r.Code, r.Prefix, trRequired)
	}

	// And the floor on the ROOTS, not just the files: a reader that silently
	// stopped recognising either call would pass every arm above.
	var gos, pyn int
	for _, r := range roots {
		if strings.HasSuffix(r.File, ".py") {
			pyn++
		} else {
			gos++
		}
	}
	// Logged as well as floored, so the numbers this file's head comment and
	// these floors rest on are re-measurable by anyone who runs the door with
	// -v, rather than being a date and a claim.
	t.Logf("in-scope temp roots: %d Go, %d Python, over %d .go files in %d package directories and %d .py files",
		gos, pyn, goFiles, len(goPkgs), len(pys))
	if gos < 8 || pyn < 5 {
		t.Fatalf("the census found %d in-scope Go root(s) and %d Python root(s) — the reader is broken, not the tree (11 and 7, MEASURED 2026-11-12)", gos, pyn)
	}
}

// The reader and the rule, over planted sources, because an assertion that
// only ever ran over a tree that passes is a census nobody has shown can say
// no — and "it reds on a root renamed back out of the namespace" is this
// bead's own done-when.
//
// It walks nothing: the fixtures are absolute paths under t.TempDir(), so
// this arm is not in the tree-wide class and needs no door of its own (the
// shape mktemproot_qa_test.go's control arm uses next door).
func TestQATheTempRootCensusCanStillSayNo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// ─── Go ───
	// Two files of one package, because resolving the parent ACROSS files is
	// what found site eleven.
	consts := write("consts.go", strings.Join([]string{
		"package probe",
		``,
		`const tmpMount = "/tmp"`,
		`const goodPrefix = "posse-probe-"`,
		`const badPrefix = "probe-"`,
		`const ambiguous = "/tmp"`,
		"",
	}, "\n"))
	body := write("probe.go", strings.Join([]string{
		"package probe",
		``,
		`func f(parent, runtimeName string) {`,
		`	os.MkdirTemp("", "posse-a-")`,                    // 4  ok
		`	os.MkdirTemp("", "a-")`,                          // 5  RED
		`	os.MkdirTemp("/tmp", goodPrefix)`,                // 6  ok
		`	os.MkdirTemp(tmpMount, badPrefix)`,               // 7  RED, the site-11 shape
		`	os.MkdirTemp(os.TempDir(), "posse-b-")`,          // 8  ok
		`	os.MkdirTemp("/var/tmp", "c-")`,                  // 9  RED
		`	os.MkdirTemp("/private/tmp", "posse-d-")`,        // 10 ok
		`	os.MkdirTemp("", "posse-e-"+runtimeName+"-")`,    // 11 ok, head-resolved
		`	os.MkdirTemp("", "e-"+runtimeName)`,              // 12 RED, head-resolved
		`	os.MkdirTemp("", runtimeName)`,                   // 13 RED, unreadable prefix
		`	os.MkdirTemp(parent, "f-")`,                      // 14 out: parent unreadable
		`	os.MkdirTemp(someHome, "g-")`,                    // 15 out: parent unbound
		`	os.MkdirTemp(ambiguous, "h-")`,                   // 16 out: two bindings, dropped
		`	os.CreateTemp("", "i-")`,                         // 17 out: a FILE
		`	_ = "os.MkdirTemp(\"/tmp\", \"x\") in a string"`, // 18 out: text, not a call
		`	t.TempDir()`,                                     // 19 out: the Test*[0-9] class
		`	ambiguous := "/var/tmp"`,
		`	_ = ambiguous`,
		`}`,
		"",
	}, "\n"))
	got, err := trGoPkg([]string{consts, body})
	if err != nil {
		t.Fatal(err)
	}
	type seen struct {
		line   int
		prefix string
		known  bool
	}
	var gotSeen []seen
	for _, r := range got {
		gotSeen = append(gotSeen, seen{r.Line, r.Prefix, r.Known})
	}
	wantSeen := []seen{
		{4, "posse-a-", true},
		{5, "a-", true},
		{6, "posse-probe-", true},
		{7, "probe-", true},
		{8, "posse-b-", true},
		{9, "c-", true},
		{10, "posse-d-", true},
		{11, "posse-e-", true},
		{12, "e-", true},
		{13, "", false},
	}
	if fmt.Sprint(gotSeen) != fmt.Sprint(wantSeen) {
		t.Errorf("the Go reader found\n  %v\nwant\n  %v\n"+
			"The lines that must be IN are the four parent spellings and the two the head resolution reaches; the ones that must be OUT are an unreadable parent, an unbound name, a name the package binds twice, os.CreateTemp, a call inside a string literal, and t.TempDir().", gotSeen, wantSeen)
	}
	var reds int
	for _, r := range got {
		if !r.Known || !strings.HasPrefix(r.Prefix, trRequired) {
			reds++
		}
	}
	if reds != 5 {
		t.Errorf("the Go rule reds %d of the planted roots, want 5 (lines 5, 7, 9, 12 and 13) — a census that cannot say no is not a census", reds)
	}

	// ─── Python ───
	py := write("probe.py", strings.Join([]string{
		"import tempfile",
		``,
		`GOOD = "posse-probe-"`,
		`BAD = "probe-"`,
		`TMP = "/tmp"`,
		`ELSEWHERE = "/opt/scratch"`,
		``,
		`def f(parent):`,
		`    # tempfile.mkdtemp(prefix="in-a-comment-") creates nothing`,
		`    a = tempfile.mkdtemp(prefix="posse-a-")`,                   // 10 ok
		`    b = tempfile.mkdtemp(prefix="b-")`,                         // 11 RED
		`    c = tempfile.mkdtemp(prefix=GOOD)`,                         // 12 ok
		`    d = tempfile.mkdtemp(prefix=BAD)`,                          // 13 RED
		`    e = tempfile.mkdtemp()`,                                    // 14 RED, no prefix
		`    with tempfile.TemporaryDirectory(prefix="posse-f-") as g:`, // 15 ok
		`        pass`,
		`    h = tempfile.mkdtemp(dir=TMP, prefix="h-")`,                  // 17 RED
		`    i = tempfile.mkdtemp(dir="/var/tmp", prefix="posse-i-")`,     // 18 ok
		`    j = tempfile.mkdtemp(dir=ELSEWHERE, prefix="j-")`,            // 19 out: not a temp parent
		`    k = tempfile.mkdtemp(dir=parent, prefix="k-")`,               // 20 out: unresolved parent
		`    fd, m = tempfile.mkstemp(prefix="m-")`,                       // 21 out: a FILE
		`    n = 'tempfile.mkdtemp(prefix="n-") inside a string'`,         // 22 IN (loose, and reds)
		`    o = tempfile.mkdtemp(prefix="posse-o-")  # trailing comment`, // 23 ok
		`    p = tempfile.mkdtemp(prefix="posse-p-(1)")`,                  // 24 ok, paren in a literal
		// An escaped quote inside the literal, with a trailing comment that
		// names `dir=`. Mishandle the escape and the quote closes early, the
		// argument text runs to the end of the line, the comment's
		// `dir="/opt"` is read as this call's parent, and the root drops OUT
		// of the census silently. This is the arm that discriminates.
		`    q = tempfile.mkdtemp(prefix='posse-q\'s-')  # not dir="/opt"`,
		"",
	}, "\n"))
	pyGot, err := trPyRoots(py)
	if err != nil {
		t.Fatal(err)
	}
	var pySeen []seen
	for _, r := range pyGot {
		pySeen = append(pySeen, seen{r.Line, r.Prefix, r.Known})
	}
	pyWant := []seen{
		{10, "posse-a-", true},
		{11, "b-", true},
		{12, "posse-probe-", true},
		{13, "probe-", true},
		{14, "", true},
		{15, "posse-f-", true},
		{17, "h-", true},
		{18, "posse-i-", true},
		{22, `n-`, true},
		{23, "posse-o-", true},
		{24, "posse-p-(1)", true},
		{25, `posse-q\'s-`, true},
	}
	if fmt.Sprint(pySeen) != fmt.Sprint(pyWant) {
		t.Errorf("the Python reader found\n  %v\nwant\n  %v\n"+
			"The comment line, the mkstemp, the non-temp dir= and the unresolved dir= must be OUT; a mkdtemp inside a string literal is deliberately IN, because a line-based reader that tried to tell those apart would need a Python parse and the cost of being wrong here is six characters.", pySeen, pyWant)
	}
	var pyReds int
	for _, r := range pyGot {
		if !r.Known || !strings.HasPrefix(r.Prefix, trRequired) {
			pyReds++
		}
	}
	if pyReds != 5 {
		t.Errorf("the Python rule reds %d of the planted roots, want 5 (lines 11, 13, 14, 17 and 22) — including the one with no prefix= at all, whose default is tempfile's own `tmp`", pyReds)
	}
}
