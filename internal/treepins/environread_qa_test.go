package treepins

// The fence on reading ANOTHER process's environment, held mechanically
// (ranger-base-cwboz item (d), from the security review of ranger-base-is1qp's
// box-wide loop census under ranger-base-gvrv7).
//
// WHAT IS BEING FENCED. `internal/posse/boxloops_linux.go`'s `boxHomeOf`
// reads `/proc/<pid>/environ` — a same-uid peer's whole environment, which on
// a box like this one carries credentials — because that is the only channel
// by which a process declares which RHQ_HOME it belongs to, and the box-wide
// census needs that to attribute a candidate to an instance (ADR 0092 D8).
// One variable leaves that function and it is a path.
//
// WHY A COMMENT WAS NOT ENOUGH. That discipline was written as prose in the
// function's own header, and prose is exactly what the review asked to be
// replaced: nothing mechanical stopped a later edit from widening the return
// to a map, lifting a second variable, or adding a second reader somewhere
// else in the tree. The whole fence was a hope that the next reader of that
// file would read the comment above the line they were editing. So the three
// claims the comment makes are checked here instead, and widening is a red.
//
// WHY HERE AND NOT IN internal/posse. The subject is the TREE — "no OTHER
// file does this" is a claim no single package can make — and this package is
// where a pin whose subject is the tree lives: `TestMain` chdirs to the repo
// root, so the reading is a plain relative walk, and the complement door
// (`make treepins-check`, ADR 0098) doors it the moment it is written, with no
// hand-added Makefile line.
//
// AN AST PARSE AND NOT A GREP. The claims are about a FUNCTION — which one
// holds the read, what it returns, what prefix it matches — and a grep cannot
// see a function boundary. A shell rewrite of this would be a second
// implementation to keep in sync by hand, which is the one thing AGENTS.md
// says a door must not be.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// erProcEnvironLiteral is the spelling that reaches a peer's environment: the
// PROC PATH, with its slash. Not the bare word — "environ" is a substring of
// "environment", which appears in prose and in error strings throughout the
// tree, and a census that matched it would be noise rather than a fence.
const erProcEnvironLiteral = "/environ"

// erTheOneReader is the function allowed to hold that read, and the file it
// is allowed to live in.
const (
	erTheOneReader = "boxHomeOf"
	erTheOneFile   = "internal/posse/boxloops_linux.go"
)

// erTheOnlyKey is the one environment variable this reader may lift.
const erTheOnlyKey = "RHQ_HOME="

// erRoots are the shipped Go trees. A test file is not shipped and may read
// what it likes — a pin that plants an environ fixture is not a peer read.
var erRoots = []string{"internal", "cmd"}

// erReader is one function found to contain the proc-environ literal.
type erReader struct {
	file    string
	fn      string
	results []string // the result types, as written
	keys    []string // every string literal in it that looks like an env key
}

func TestQAReadingAPeersEnvironmentHappensInExactlyOneFunction(t *testing.T) {
	t.Parallel()
	found := erFindEnvironReaders(t)
	if len(found) != 1 {
		var where []string
		for _, r := range found {
			where = append(where, r.file+":"+r.fn)
		}
		sort.Strings(where)
		t.Fatalf("%d functions in the shipped tree read %q, want exactly 1 (%s's %s):\n  %s\n\nA peer's environment carries credentials. One reader is what makes \"one variable, and it is a path\" a checkable claim; a second reader is a second fence nobody wrote (ranger-base-cwboz).",
			len(found), erProcEnvironLiteral, erTheOneFile, erTheOneReader, strings.Join(where, "\n  "))
	}
	r := found[0]
	if r.fn != erTheOneReader || r.file != erTheOneFile {
		t.Errorf("the environ read lives in %s's %s, want %s's %s — if it MOVED, move this pin with it and say why on the bead",
			r.file, r.fn, erTheOneFile, erTheOneReader)
	}
}

func TestQAThePeerEnvironmentReaderReturnsOneStringAndNothingElse(t *testing.T) {
	t.Parallel()
	found := erFindEnvironReaders(t)
	if len(found) != 1 {
		t.Skip("the one-reader pin above owns that failure")
	}
	got := found[0].results
	want := []string{"string", "bool"}
	if len(got) != len(want) {
		t.Fatalf("%s returns %v, want %v — the return type IS the fence: a map, a slice or a struct would carry a peer's other variables out of the one function that is allowed to see them (ranger-base-cwboz)",
			erTheOneReader, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s's result %d is %q, want %q", erTheOneReader, i, got[i], want[i])
		}
	}
}

func TestQAThePeerEnvironmentReaderLiftsOnlyTheInstanceHome(t *testing.T) {
	t.Parallel()
	found := erFindEnvironReaders(t)
	if len(found) != 1 {
		t.Skip("the one-reader pin above owns that failure")
	}
	keys := found[0].keys
	if len(keys) != 1 || keys[0] != erTheOnlyKey {
		t.Errorf("%s matches %v, want exactly [%q].\n\nEvery key here is a variable lifted out of a peer's environment. A second one is a second thing that can be rendered onto the install delegate's bead or opened as a path, and it would arrive without the validation the first one has (boxHomeIsWellFormed).",
			erTheOneReader, keys, erTheOnlyKey)
	}
}

// erFindEnvironReaders walks the shipped Go trees and returns every function
// whose body holds a string literal containing the proc-environ path.
func erFindEnvironReaders(t *testing.T) []erReader {
	t.Helper()
	var out []erReader
	for _, root := range erRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			f, perr := parser.ParseFile(fset, path, nil, 0) // no comments: prose may say "/environ"
			if perr != nil {
				t.Fatalf("parse %s: %v", path, perr)
			}
			out = append(out, erReadersIn(filepath.ToSlash(path), f)...)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return out
}

// erReadersIn is the per-file scan, split out so the self-test below can run
// the three predicates over source it writes itself rather than over the
// tree. A census whose predicates are only ever fed the tree is a census
// nobody has seen say no.
func erReadersIn(name string, f *ast.File) []erReader {
	var out []erReader
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if !erBodyReadsEnviron(fn.Body) {
			continue
		}
		out = append(out, erReader{
			file:    name,
			fn:      fn.Name.Name,
			results: erResultTypes(fn),
			keys:    erEnvKeys(fn.Body),
		})
	}
	return out
}

// TestQATheEnvironFenceCensusCanStillSayNo drives the three predicates over
// source this test writes, because the tree is green by construction and a
// predicate that silently stopped matching would look exactly the same.
//
// The widened-RETURN case is why this exists rather than a tree mutation: a
// `boxHomeOf` edited to return a map does not compile — every caller takes a
// string — so mutating the real file proves only that Go has a type checker.
// Here the shape is reachable.
func TestQATheEnvironFenceCensusCanStillSayNo(t *testing.T) {
	t.Parallel()
	parse := func(src string) *ast.File {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "fake.go", src, 0)
		if err != nil {
			t.Fatalf("fixture does not parse: %v", err)
		}
		return f
	}

	// (1) a reader is FOUND by the proc-path literal, however it is built.
	for name, src := range map[string]string{
		"concatenated": `package p
func r(pid int) (string, bool) { b, _ := f("/proc/" + itoa(pid) + "/environ"); return g(b, "RHQ_HOME=") }`,
		"formatted": `package p
func r(pid int) (string, bool) { b, _ := f(sprintf("/proc/%d/environ", pid)); return g(b, "RHQ_HOME=") }`,
	} {
		got := erReadersIn("fake.go", parse(src))
		if len(got) != 1 || got[0].fn != "r" {
			t.Errorf("%s: the census did not find the environ read: %+v", name, got)
		}
	}

	// (2) prose alone is NOT a reader — comments are not parsed, and the
	// bare word "environment" must not match.
	noise := `package p
// this reads /proc/<pid>/environ one day
func r() string { return "resolved from this shell's environment" }`
	if got := erReadersIn("fake.go", parse(noise)); len(got) != 0 {
		t.Errorf("a comment and the word \"environment\" are not a peer read: %+v", got)
	}

	// (3) a WIDENED RETURN is rejected. This is the arm no tree mutation can
	// reach.
	wide := `package p
func r(pid int) (map[string]string, bool) { b, _ := f("/proc/" + itoa(pid) + "/environ"); return parse(b), true }`
	got := erReadersIn("fake.go", parse(wide))
	if len(got) != 1 {
		t.Fatalf("fixture should be one reader: %+v", got)
	}
	if got[0].results[0] != "map[string]string" {
		t.Errorf("the census read the widened result list as %v — it cannot tell a map from a string, so widening the reader would pass the fence", got[0].results)
	}

	// (4) a SECOND lifted variable is rejected.
	two := `package p
func r(pid int) (string, bool) {
	b, _ := f("/proc/" + itoa(pid) + "/environ")
	if v, ok := cut(b, "RHQ_HOME="); ok { return v, true }
	if v, ok := cut(b, "ANTHROPIC_API_KEY="); ok { return v, true }
	return "", false
}`
	got = erReadersIn("fake.go", parse(two))
	if len(got) != 1 {
		t.Fatalf("fixture should be one reader: %+v", got)
	}
	if len(got[0].keys) != 2 {
		t.Errorf("the census found %v, so a second lifted variable would pass", got[0].keys)
	}

	// (5) a value that is not an env key is not counted as one, or the arm
	// above would red on ordinary code.
	plain := `package p
func r(pid int) (string, bool) {
	b, _ := f("/proc/" + itoa(pid) + "/environ")
	_ = "pid: "
	_ = "some sentence = with an equals"
	return cut(b, "RHQ_HOME=")
}`
	got = erReadersIn("fake.go", parse(plain))
	if len(got) != 1 || len(got[0].keys) != 1 || got[0].keys[0] != erTheOnlyKey {
		t.Errorf("ordinary literals were read as lifted variables: %+v", got)
	}
}

func erBodyReadsEnviron(body *ast.BlockStmt) bool {
	hit := false
	ast.Inspect(body, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if v, err := strconv.Unquote(lit.Value); err == nil && strings.Contains(v, erProcEnvironLiteral) {
			hit = true
		}
		return true
	})
	return hit
}

// erResultTypes is the function's result list as source text, which for the
// shapes that matter here ("string", "bool", "map[string]string") is the
// identifier or expression printed back.
func erResultTypes(fn *ast.FuncDecl) []string {
	var out []string
	if fn.Type.Results == nil {
		return out
	}
	for _, fld := range fn.Type.Results.List {
		name := erTypeString(fld.Type)
		n := len(fld.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, name)
		}
	}
	return out
}

func erTypeString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + erTypeString(t.X)
	case *ast.ArrayType:
		return "[]" + erTypeString(t.Elt)
	case *ast.MapType:
		return "map[" + erTypeString(t.Key) + "]" + erTypeString(t.Value)
	case *ast.SelectorExpr:
		return erTypeString(t.X) + "." + t.Sel.Name
	default:
		return "?"
	}
}

// erEnvKeys is every string literal in the body that has the shape of an
// environment-variable key being matched: it ends in "=" and is otherwise an
// upper-case name. That is how a lift is spelled whether the call is
// strings.CutPrefix, strings.HasPrefix, bytes.HasPrefix or a hand-rolled
// index — so the pin does not have to enumerate the call sites, and a new
// spelling of the same lift is caught by the key rather than by the verb.
func erEnvKeys(body *ast.BlockStmt) []string {
	var out []string
	seen := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil || !strings.HasSuffix(v, "=") || len(v) < 2 {
			return true
		}
		name := strings.TrimSuffix(v, "=")
		if name != strings.ToUpper(name) || strings.ContainsAny(name, " /\t\n") {
			return true
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
		return true
	})
	sort.Strings(out)
	return out
}
