package treepins

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"testing"
)

// checkCredentialFence holds a TestMain's env fence against credential.go's
// credentialDirVars. CLAUDE_CONFIG_DIR is the one name the fence SETS (to its
// tempdir); every other name must be os.Unsetenv'd, so the locator falls
// through to that tempdir. Unset and never "": the resolver reads presence,
// and an empty CLAUDE_SECURESTORAGE_CONFIG_DIR resolves to $HOME/.claude.
//
// cmd/posse/credentialfencecheck_test.go carries the same code;
// the two packages cannot share test code.
func checkCredentialFence(t *testing.T, credentialGo, fenceFile string) {
	t.Helper()
	vars := credentialDirVarsFrom(t, credentialGo)
	calls := testMainEnvVerbs(t, fenceFile)
	for _, name := range vars {
		want := "Unsetenv"
		if name == "CLAUDE_CONFIG_DIR" {
			want = "Setenv"
		}
		if got := calls[name]; got != want {
			t.Errorf("%s's TestMain answers %s with %q, want os.%s — the credential locator honours it (credentialDirVars), and a child built from os.Environ() resolves the operator's live credential store through it", fenceFile, name, got, want)
		}
		// Runtime arm: green on CI whatever the source says, red on any
		// seat that exports the variable past a broken fence.
		if name == "CLAUDE_CONFIG_DIR" {
			continue
		}
		if _, set := os.LookupEnv(name); set {
			t.Errorf("%s survived into this test binary — TestMain's fence did not unset it", name)
		}
	}
}

// credentialDirVarsFrom reads the string literals of credentialDirVars out of
// credential.go. Fails rather than returning nothing, so a renamed or
// reshaped declaration cannot leave the pin checking an empty list.
func credentialDirVarsFrom(t *testing.T, path string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "credentialDirVars" || len(vs.Values) != 1 {
			return true
		}
		lit, ok := vs.Values[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, e := range lit.Elts {
			bl, ok := e.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				t.Fatalf("credentialDirVars in %s carries a non-literal element — this pin cannot read it", path)
			}
			s, err := strconv.Unquote(bl.Value)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, s)
		}
		return false
	})
	if len(out) == 0 {
		t.Fatalf("no credentialDirVars string list in %s — this pin derives the fence's class from it", path)
	}
	return out
}

// testMainEnvVerbs maps each literal env name TestMain passes to os.Setenv or
// os.Unsetenv to the verb used.
func testMainEnvVerbs(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var body *ast.BlockStmt
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "TestMain" && fn.Body != nil {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatalf("no func TestMain in %s", path)
	}
	out := map[string]string{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Setenv" && sel.Sel.Name != "Unsetenv") {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "os" {
			return true
		}
		bl, ok := call.Args[0].(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			return true
		}
		if s, err := strconv.Unquote(bl.Value); err == nil {
			out[s] = sel.Sel.Name
		}
		return true
	})
	return out
}
