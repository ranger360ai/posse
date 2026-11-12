package treepins

// QA pins for ranger-base-3spp4 — a credential token is a type that cannot
// be printed (internal/posse/credtoken.go), and the ONE way to its bytes,
// CredToken.Reveal, is called from a register of sites and nowhere else.
//
// The type makes every printer render a redaction. What it cannot do on its
// own is stop a caller from taking the string out and printing THAT —
// `t.Fatalf("%q", tok.Reveal())` is the incident the bead exists to make
// impossible, one hop further down. So the hop is the register:
//
//   - every non-test call of Reveal is a row of crRevealAllowed, naming the
//     file, the function, and why that line must hold the secret itself;
//     a row that matches nothing is stale and reds as loudly as an
//     unregistered call;
//   - no _test.go anywhere under internal/ or cmd/ calls Reveal at all. A
//     test that needs to compare tokens compares fingerprints
//     (internal/posse/credtokenhelpers_test.go's sameCredToken).
//
// WHAT A GREEN HERE PROVES, exactly. This is a SYNTACTIC census: it parses
// every .go file under internal/ and cmd/ and counts selector expressions
// whose selected name is Reveal — a call and a method value alike — with no
// type checker. That is precise for this tree because Reveal is the name of
// no other method in it (TestCredTokenRevealIsTheOnlyMethodOfThatName holds
// that, so the day a second Reveal appears this census says so rather than
// pardoning or blaming the wrong type). It is blind to reflection
// (MethodByName("Reveal")) and to unsafe, neither of which a careless
// failure message reaches for.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// crRevealAllowed is the register. Each row is one call: a second Reveal in
// the same function is a second reason and gets its own row.
var crRevealAllowed = []struct{ file, fn, why string }{
	{"internal/posse/planusage_anthropic.go", "Read",
		"the usage endpoint's Authorization: Bearer header — the token is presented, after pinnedRequest has checked the URL one more time"},
	{"internal/posse/modelavail.go", "getPage",
		"the model list endpoint's Authorization: Bearer header, after pinnedRequest"},
}

// crRevealDecl is where the method is declared: the census's positive
// witness that the name it counts is still the type's one door.
const crRevealDecl = "internal/posse/credtoken.go"

type crHit struct {
	file string // slash-separated, relative to the walk root
	fn   string // enclosing func's name, "" at package level
	test bool
	pos  string
}

type crScan struct {
	hits      []crHit
	goFiles   int
	testFiles int
	// decls is every file that declares a method named Reveal, keyed by
	// file, valued by receiver type name.
	decls map[string][]string
}

// crRevealScan parses every .go file (tests INCLUDED — they are half the
// rule) under root/dirs.
func crRevealScan(t *testing.T, root string, dirs []string) crScan {
	t.Helper()
	s := crScan{decls: map[string][]string{}}
	fset := token.NewFileSet()
	for _, d := range dirs {
		p := filepath.Join(root, d)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		err := filepath.WalkDir(p, func(path string, de os.DirEntry, err error) error {
			if err != nil || de.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if perr != nil {
				t.Fatalf("parse %s: %v", path, perr)
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			isTest := strings.HasSuffix(path, "_test.go")
			s.goFiles++
			if isTest {
				s.testFiles++
			}
			record := func(fn string, n ast.Node) {
				ast.Inspect(n, func(n ast.Node) bool {
					if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Reveal" {
						s.hits = append(s.hits, crHit{file: rel, fn: fn, test: isTest, pos: fset.Position(sel.Sel.Pos()).String()})
					}
					return true
				})
			}
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok {
					record("", decl)
					continue
				}
				if fd.Name.Name == "Reveal" && fd.Recv != nil && len(fd.Recv.List) == 1 {
					s.decls[rel] = append(s.decls[rel], crRecvName(fd.Recv.List[0].Type))
				}
				if fd.Body != nil {
					record(fd.Name.Name, fd.Body)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", p, err)
		}
	}
	return s
}

func crRecvName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return crRecvName(x.X)
	case *ast.Ident:
		return x.Name
	}
	return "?"
}

// crGrade sorts hits against the register: calls from a test file, calls no
// row pardons, and rows that do not match exactly one call.
func crGrade(hits []crHit, allow []struct{ file, fn, why string }) (inTests, unregistered []crHit, offCount []string) {
	used := make([]int, len(allow))
	for _, h := range hits {
		if h.test {
			inTests = append(inTests, h)
			continue
		}
		matched := false
		for i, a := range allow {
			if h.file == a.file && h.fn == a.fn {
				used[i]++
				matched = true
				break
			}
		}
		if !matched {
			unregistered = append(unregistered, h)
		}
	}
	for i, a := range allow {
		if used[i] != 1 {
			offCount = append(offCount, a.file+" "+a.fn+": "+itoa(used[i])+" calls")
		}
	}
	return inTests, unregistered, offCount
}

func TestCredTokenRevealIsCalledOnlyFromTheRegister(t *testing.T) {
	t.Parallel()
	s := crRevealScan(t, ".", []string{"internal", "cmd"})
	if s.goFiles < 500 || s.testFiles < 300 {
		t.Fatalf("parsed %d .go files (%d tests) under internal/ and cmd/ — the walk measured nothing, so an absence here is not evidence", s.goFiles, s.testFiles)
	}
	if got := s.decls[crRevealDecl]; len(got) != 1 || got[0] != "CredToken" {
		t.Fatalf("%s declares Reveal on %v, want exactly [CredToken] — the door this census counts moved or was renamed, and the census now covers nothing", crRevealDecl, got)
	}
	inTests, unregistered, offCount := crGrade(s.hits, crRevealAllowed)
	t.Logf("parsed %d .go files (%d tests); %d Reveal sites", s.goFiles, s.testFiles, len(s.hits))
	for _, h := range inTests {
		t.Errorf("%s: a test reaches a credential's bytes (in %s). No _test.go may call CredToken.Reveal — a test holding the revealed string is one t.Fatalf away from printing it into a transcript, which is the leak ranger-base-3spp4 made structurally impossible. Compare tokens with sameCredToken (a fingerprint).", h.pos, h.fn)
	}
	for _, h := range unregistered {
		t.Errorf("%s: %s reveals a credential's bytes and no crRevealAllowed row names it. Reveal is the one door out of CredToken (internal/posse/credtoken.go); if this line must hold the secret itself — it hands it to a header or a pipe — add a row naming the file, the func, and why. If it only needs to compare or log, use Fingerprint, Len or the token itself, which print the redaction.", h.pos, h.fn)
	}
	for _, o := range offCount {
		t.Errorf("crRevealAllowed row %s, want exactly 1. Zero is a stale pardon — drop the row; two is a second reason that needs its own sentence.", o)
	}
}

// Reveal is precise as a NAME only while no other method in the tree has
// it. The day one does, this reds, and the census above needs a type
// checker or a narrower match rather than a pardon.
func TestCredTokenRevealIsTheOnlyMethodOfThatName(t *testing.T) {
	t.Parallel()
	s := crRevealScan(t, ".", []string{"internal", "cmd"})
	var files []string
	for f, recvs := range s.decls {
		for _, r := range recvs {
			files = append(files, f+" ("+r+")")
		}
	}
	sort.Strings(files)
	if len(files) != 1 || files[0] != crRevealDecl+" (CredToken)" {
		t.Errorf("methods named Reveal: %v, want only %s (CredToken). The register census matches by selector NAME, so a second Reveal makes it count calls of the wrong type.", files, crRevealDecl)
	}
}

// The scanner is not asleep: one planted file per shape, graded against a
// hand-typed register.
func TestCredTokenRevealScannerCatchesEachShape(t *testing.T) {
	t.Parallel()
	root := arFixture(t, map[string]string{
		"internal/p/tok.go": "package p\ntype CredToken struct{ s string }\nfunc (c CredToken) Reveal() string { return c.s }\n",
		"internal/p/ok.go":  "package p\nfunc header(c CredToken) string { return \"Bearer \" + c.Reveal() }\n",
		"internal/p/bad.go": "package p\nimport \"fmt\"\nfunc logIt(c CredToken) { fmt.Println(c.Reveal()) }\n",
		"internal/p/val.go": "package p\nvar sneak = CredToken{}.Reveal\n",
		"internal/p/two.go": "package p\nfunc twice(c CredToken) string { return c.Reveal() + c.Reveal() }\n",
		"cmd/x/x_test.go":   "package x\nimport \"testing\"\nfunc TestX(t *testing.T) { var c interface{ Reveal() string }; t.Log(c.Reveal()) }\n",
	})
	s := crRevealScan(t, root, []string{"internal", "cmd"})
	allow := []struct{ file, fn, why string }{
		{"internal/p/ok.go", "header", "fixture"},
		{"internal/p/two.go", "twice", "fixture"},
		{"internal/p/gone.go", "vanished", "fixture"},
	}
	inTests, unregistered, offCount := crGrade(s.hits, allow)
	if len(inTests) != 1 || inTests[0].file != "cmd/x/x_test.go" || inTests[0].fn != "TestX" {
		t.Errorf("test-file calls = %+v, want exactly the one in cmd/x/x_test.go TestX", inTests)
	}
	var un []string
	for _, h := range unregistered {
		un = append(un, h.file+":"+h.fn)
	}
	sort.Strings(un)
	if strings.Join(un, ",") != "internal/p/bad.go:logIt,internal/p/val.go:" {
		t.Errorf("unregistered = %v, want the call in logIt and the package-level method value", un)
	}
	sort.Strings(offCount)
	if strings.Join(offCount, "|") != "internal/p/gone.go vanished: 0 calls|internal/p/two.go twice: 2 calls" {
		t.Errorf("off-count rows = %v, want the stale row at 0 and the doubled one at 2", offCount)
	}
	if got := s.decls["internal/p/tok.go"]; len(got) != 1 || got[0] != "CredToken" {
		t.Errorf("declaration witness = %v, want [CredToken]", got)
	}
}
