package posse

// TestMain's CLI-home fence, and the one variable that was outside it
// (ranger-base-u3kft).
//
// WHAT HAPPENED. TestMain replaces $HOME with a tempdir and clears the CLI
// home overrides so that no locator in this binary reads the OPERATOR's live
// store. It named three — CLAUDE_CONFIG_DIR, CODEX_HOME, GROK_HOME — and the
// credential locator honours a FOURTH, CLAUDE_SECURESTORAGE_CONFIG_DIR,
// which credentialDirNamed reads FIRST: ahead of CLAUDE_CONFIG_DIR and ahead
// of the temp HOME. A seat whose launcher exports it ran this package's
// tests against the directory that variable names. MEASURED 2026-11-11 on
// this box, which exports it from a managed-settings file on every launch:
// TestOneEnvelopeReadsIdenticallyThroughBothStores (credseam_test.go,
// untagged, so red in all three arms) compared a keychain item name suffixed
// with the live directory's hash against one derived after the test had
// unset the variable, and TestTheReportRendersStructuralAbsenceAsSomethingToDo
// (refresh_test.go, arm 2) asserted this box has no login and read a real
// expiry out of the operator's credentials file.
//
// WHY IT WAS INVISIBLE, and why this file has a SOURCE arm as well as a
// RUNTIME one. ci.yml exports no such variable, so the runtime arm below is
// green on CI whatever the fence says, and it was green on the fleet seats
// that happened not to carry it. A pin that can only fire where the accident
// already happens is not a pin. The source arms read the fence itself: they
// hold TestMain's list against the package's own enumeration of the
// variables its credential locator honours, so a FIFTH name added to
// credential.go reds here on every box, before any seat exports it.
//
// WHY UNSET AND NOT "". The three lines TestMain already had write "" on the
// stated grounds that their resolvers treat empty as unset. This one does
// not: a present-but-empty CLAUDE_SECURESTORAGE_CONFIG_DIR is a third state
// that shadows CLAUDE_CONFIG_DIR and resolves to the home
// (credentialDirNamed, credential.go; credseam_test.go's tables are the pin
// on that behaviour itself). So clearing it would have fenced off the
// operator's directory and left the variable in force — and arm 3 below is
// what keeps "which names need Unsetenv" from being a fact only this comment
// knows.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The CLI home overrides TestMain is required to fence, derived rather than
// listed: the credential locator's own enumeration (credential.go) plus the
// two runtime homes that are not in it. CLAUDE_CONFIG_DIR is in
// credentialDirVars already, so it is not spelled twice.
func fencedCLIHomeVars() []string {
	return append(append([]string{}, credentialDirVars...), CodexHomeVar, GrokHomeVar)
}

// The names whose resolver reads PRESENCE rather than a non-empty value, and
// which TestMain must therefore os.Unsetenv rather than clear. Arm 3 holds
// this list against credentialDirNamed's own reads, so a sixth such name
// cannot arrive unclassified.
var fenceNeedsUnset = map[string]string{
	"CLAUDE_SECURESTORAGE_CONFIG_DIR": "a present-but-empty value shadows CLAUDE_CONFIG_DIR and resolves to the home, so \"\" leaves the variable in force",
}

// ─── arm 1: the running binary ───────────────────────────────────────────────

// What every test in this binary is entitled to assume: the credential
// locator answers inside the temp HOME, and names nothing.
func TestQATheCredentialLocatorIsFencedInsideTheTempHome(t *testing.T) {
	t.Parallel()

	home := os.Getenv("HOME")
	if home == "" {
		t.Fatal("no $HOME — TestMain always sets one")
	}

	for _, key := range fencedCLIHomeVars() {
		v, set := os.LookupEnv(key)
		if v != "" {
			t.Errorf("%s = %q in the test binary — TestMain's fence let a CLI home override through, and every locator that honours it reads the operator's live store past the temp HOME", key, v)
		}
		if why, ok := fenceNeedsUnset[key]; ok && set {
			t.Errorf("%s is SET (to %q) — it must be os.Unsetenv'd and not cleared: %s", key, v, why)
		}
	}

	dir, named, err := credentialDirNamed()
	if err != nil {
		t.Fatalf("credentialDirNamed: %v", err)
	}
	if named {
		t.Errorf("credentialDirNamed reports the directory %q as NAMED — some variable in the environment is moving this binary's credential reads off the temp HOME", dir)
	}
	if dir != home && !strings.HasPrefix(dir, home+string(os.PathSeparator)) {
		t.Errorf("credentialDirNamed = %q, which is outside the temp HOME %q — this binary resolves the operator's credential store", dir, home)
	}
	// The seam the two reds above actually failed on: keychainItem suffixes
	// the item with a sha256 of the directory when it is NAMED, so a fenced
	// binary must produce the bare service name.
	if got := keychainItem(); got != KeychainService {
		t.Errorf("keychainItem() = %q, want the unsuffixed %q — a named directory is being hashed into the item, and a test that derives the expected name after unsetting the variable compares two different stores", got, KeychainService)
	}
}

// ─── arm 2: the fence in TestMain's source ───────────────────────────────────

// fenceCall is one os.Setenv/os.Unsetenv on a string-literal variable name,
// as TestMain spells it.
type fenceCall struct {
	verb  string // "Setenv" | "Unsetenv"
	value string // the literal second argument, for Setenv
}

// testMainEnvCalls reads TestMain's body out of the source that defines it.
// A SOURCE read and not a runtime one, because the whole point is to fire on
// a box where the variable is absent from the environment anyway.
func testMainEnvCalls(t *testing.T) map[string]fenceCall {
	t.Helper()

	const src = "herdr_test.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", src, err)
	}

	var body *ast.BlockStmt
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == "TestMain" && fn.Body != nil {
			body = fn.Body
			break
		}
	}
	if body == nil {
		t.Fatalf("no func TestMain in %s — this pin reads the fence out of that function", src)
	}

	out := map[string]fenceCall{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "os" {
			return true
		}
		if sel.Sel.Name != "Setenv" && sel.Sel.Name != "Unsetenv" {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		name, ok := stringLit(call.Args[0])
		if !ok {
			return true
		}
		fc := fenceCall{verb: sel.Sel.Name}
		if fc.verb == "Setenv" && len(call.Args) > 1 {
			v, lit := stringLit(call.Args[1])
			if !lit {
				// A computed value is a move, not a fence; record it as
				// something other than the empty clear so arm 2 reports it.
				v = "<computed>"
			}
			fc.value = v
		}
		out[name] = fc
		return true
	})
	return out
}

func stringLit(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	return strings.Trim(bl.Value, "`\""), true
}

// Every variable the credential locator and the runtime homes honour is
// named in TestMain's fence, in the shape that variable's resolver requires.
func TestQATestMainsFenceNamesEveryCLIHomeOverride(t *testing.T) {
	t.Parallel()

	calls := testMainEnvCalls(t)
	for _, key := range fencedCLIHomeVars() {
		fc, ok := calls[key]
		if !ok {
			t.Errorf("TestMain's fence does not name %s — a locator that honours it reads the operator's live store on any seat whose launcher exports it, and CI exports none of these so nothing else will say so", key)
			continue
		}
		why, needsUnset := fenceNeedsUnset[key]
		switch {
		case needsUnset && fc.verb != "Unsetenv":
			t.Errorf("TestMain fences %s with os.%s(%q) — it must be os.Unsetenv: %s", key, fc.verb, fc.value, why)
		case !needsUnset && fc.verb == "Setenv" && fc.value != "":
			t.Errorf("TestMain sets %s to %q rather than clearing it — the fence must leave every CLI home override out of force", key, fc.value)
		}
	}
}

// ─── arm 3: which names are read by PRESENCE ─────────────────────────────────

// fenceNeedsUnset is a judgement about credentialDirNamed's code, so it is
// held against that code. The resolver reads the variables that are special
// in this way with os.LookupEnv — presence and value — and the ones that are
// not with the ordinary non-empty rule, through runtimeHomeOverride. So a
// LookupEnv in that function over a name this file has not classified is a
// variable whose fence shape nobody has decided, and TestMain's "" would be
// the wrong answer for it by default.
func TestQAOnlyTheClassifiedNamesAreReadByPresence(t *testing.T) {
	t.Parallel()

	const src = "credential.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", src, err)
	}

	var body *ast.BlockStmt
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == "credentialDirNamed" && fn.Body != nil {
			body = fn.Body
			break
		}
	}
	if body == nil {
		t.Fatalf("no func credentialDirNamed in %s — this pin reads the resolver's own env reads out of it", src)
	}

	seen := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "LookupEnv" {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "os" {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		if name, lit := stringLit(call.Args[0]); lit {
			seen[name] = true
		} else {
			t.Errorf("credentialDirNamed reads os.LookupEnv over a non-literal name — this pin cannot classify it, and TestMain's fence shape for it is undecided")
		}
		return true
	})

	for name := range seen {
		if _, ok := fenceNeedsUnset[name]; !ok {
			t.Errorf("credentialDirNamed reads %s by PRESENCE (os.LookupEnv) and credseamfence_qa_test.go does not classify it — decide whether TestMain must os.Unsetenv it, add it to fenceNeedsUnset with the reason, and make sure it is in credentialDirVars so arm 2 sees it", name)
		}
	}
	for name, why := range fenceNeedsUnset {
		if !seen[name] {
			t.Errorf("%s is classified as read-by-presence (%q) and credentialDirNamed no longer reads it that way — if the resolver's rule changed, TestMain may clear it like its three neighbours and this entry should go", name, why)
		}
	}

	// And the enumeration arm 2 walks has to carry them, or a name that is
	// classified here is still not fenced there.
	for name := range fenceNeedsUnset {
		var in bool
		for _, v := range credentialDirVars {
			if v == name {
				in = true
				break
			}
		}
		if !in {
			t.Errorf("%s is classified here but is not in credentialDirVars, so TestQATestMainsFenceNamesEveryCLIHomeOverride never checks it", name)
		}
	}
}

// A guard on the pin's own reach: the two source arms open files by a
// relative path, which is the package directory under `go test`. If that
// ever stops being true both arms would fail loudly at the parse — this says
// so in one place rather than leaving the reader to infer it from a parse
// error.
func TestQAThePackageSourceIsReadableFromTheTestsCwd(t *testing.T) {
	t.Parallel()

	for _, src := range []string{"herdr_test.go", "credential.go"} {
		if _, err := os.Stat(src); err != nil {
			wd, _ := os.Getwd()
			t.Fatalf("%s is not readable from %s: %v — the source arms of this pin resolve the package directory as the test's cwd", src, filepath.Clean(wd), err)
		}
	}
}
