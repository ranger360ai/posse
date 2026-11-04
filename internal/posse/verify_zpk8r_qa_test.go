//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-zpk8r, the three LIVE findings ranger-base-u4uwg
// left behind. Each fix changes what a process PRINTS, so each pin is over a
// printed line or over the wiring that prints one.
//
// ARM 1 — THE CALL SITE (finding 1). planLaunch prints three launch-time
// readings behind one guard: GitSystemScopeDegraded (ranger-base-t301k),
// PolicyPinDegraded (ranger-base-s69ep) and GitGlobalScopeDegraded
// (ranger-base-obdnk). Every one of the three FUNCTIONS was pinned
// (inletpingitconfig_qa_test.go, inletpingitglobal_qa_test.go,
// policypin_test.go) and no pin asserted that planLaunch still printed any
// of them. MEASURED 2026-11-04, this box:
//
//	sed -i '' '2879s|.*|\t\t\t\t_ = d|' internal/posse/herdrback.go
//	go test -run 'Policy|Parity|Gate|Inlet|Wall|LaunchCheck|HookConfig|HookParity' ./internal/posse
//	-> ok. Same for the other two warn() lines.
//
// A launch that says nothing is exactly the silence all three beads were
// filed about, and the suite could not tell that state from a clean box.
//
// THE CLASS IS DERIVED, not a list of three (which is what u4uwg asked for:
// a fix for one line that leaves the other two is the shape to avoid). The
// rule is an EXPORTED, ZERO-ARGUMENT, string-returning `*Degraded` function
// in this package — the signature of "a reading of THIS BOX, taken at
// launch, with nothing to pass it". The package's other `*Degraded` functions
// all take arguments (DetectionDegraded, RexRuntimeDegraded,
// RexNoReportDegraded, PIDChannelDegraded), which is the honest tell: they
// are readings ABOUT something a caller already holds, reached from wherever
// that thing is, and they have their own pins. So a fourth zero-argument
// reading added tomorrow is in this class without anyone editing a list, and
// reds its own bead if it is declared and never printed.
//
// NOT A TREE-WIDE PIN, and deliberately: the scan is a glob of THIS
// package's non-test sources with no climb to a repo root, which is the
// derivation internal/treepins/treewidedoor_qa_test.go keys on
// (`qibRepoRoot`). Its door is $(QA_PKGCENSUS_PINS), the one-package-census
// door, and it needs no new Makefile target. What that scope does not cover,
// said rather than left silent: a launch path assembled in ANOTHER package
// that prints these readings, and a reading of this class declared outside
// internal/posse. Neither exists today — all three call sites are in
// planLaunch — and the stronger shape, a launch-level pin over a fixture
// policy dir, is not reachable from a seat at all: the policy dir is
// root-owned and claudePolicyDir takes no injection point.
//
// ARM 2 — THE INVERTED VERDICT (finding 2), and ARM 3 — the raw JSON value
// (finding 3). Both are policypin.go output; their head comments are below.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// zpkPrinters is the set of callee names that reach a seat's streams. A
// reading is "printed" when its value is an argument to one of these. The set
// is this house's spelling and nothing derives it: a new way to print adds a
// name here, and that edit is the point — it is what makes someone say out
// loud that a launch reading now goes somewhere else.
var zpkPrinters = []string{"warn", "Warn", "Warnf", "Print", "Printf", "Println", "Fprint", "Fprintf", "Fprintln"}

// zpkCalleeName is the last component of a call's callee: `warn` for
// `warn(...)`, `Fprintf` for `fmt.Fprintf(...)`, `warn` for `b.warn(...)`.
// Scope-free on purpose — the claim is about the verb, not about who holds it.
func zpkCalleeName(c *ast.CallExpr) string {
	switch fn := c.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

func zpkIsPrinter(name string) bool {
	for _, p := range zpkPrinters {
		if p == name {
			return true
		}
	}
	return false
}

// zpkEmptyStringLit reports whether e is the literal "".
func zpkEmptyStringLit(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	v, err := strconv.Unquote(lit.Value)
	return err == nil && v == ""
}

func TestQAEveryZeroArgDegradedLaunchReadingIsPrintedAtItsCallSite(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("no non-test .go source in this package — a census over nothing passes for a reason that is not the property")
	}

	// The class, derived from the signature.
	class := map[string]string{} // reading -> the file that declares it
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() || !strings.HasSuffix(fn.Name.Name, "Degraded") {
				continue
			}
			if fn.Type.Params != nil && len(fn.Type.Params.List) != 0 {
				continue
			}
			if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
				continue
			}
			id, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || id.Name != "string" {
				continue
			}
			class[fn.Name.Name] = fset.Position(fn.Pos()).Filename
		}
	}

	// The floor, against a reading LEAVING the class rather than going
	// unprinted inside it: the three the beads above were filed about. A
	// reading that is genuinely retired is removed from this list by the
	// bead that retires it, which is the whole of the ceremony.
	for _, want := range []string{"GitSystemScopeDegraded", "PolicyPinDegraded", "GitGlobalScopeDegraded"} {
		if _, ok := class[want]; !ok {
			t.Errorf("%s is no longer an exported zero-argument string reading in this package — either it was retired (remove it here, in the bead that retires it) or it grew a parameter and left the class this pin censuses, taking its only call-site witness with it", want)
		}
	}
	if len(class) < 3 {
		t.Fatalf("the launch-reading class holds %d member(s): %v. There are three (ranger-base-t301k, ranger-base-s69ep, ranger-base-obdnk) and a census that finds fewer is watching a class that has emptied out under it", len(class), class)
	}

	// Every member has at least one call site that GUARDS its value and
	// PRINTS it. The shape is `if d := R(); d != "" { …(…, d…) }`, which is
	// how all three are spelled; a different shape reds here and the message
	// says which, because "the reading reaches a stream" is the property and
	// a second spelling of it is a second thing to keep true.
	printed := map[string]string{} // reading -> where it is printed
	guarded := map[string]string{} // reading -> where its value was bound
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			ifs, ok := n.(*ast.IfStmt)
			if !ok || ifs.Init == nil {
				return true
			}
			as, ok := ifs.Init.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			bound, ok := as.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			call, ok := as.Rhs[0].(*ast.CallExpr)
			if !ok || len(call.Args) != 0 {
				return true
			}
			reading := zpkCalleeName(call)
			if _, ok := class[reading]; !ok {
				return true
			}
			// The guard is `bound != ""`. Without it the reading's clean
			// answer ("") would be printed as a line on every launch.
			cmp, ok := ifs.Cond.(*ast.BinaryExpr)
			if !ok || cmp.Op != token.NEQ {
				return true
			}
			x, ok := cmp.X.(*ast.Ident)
			if !ok || x.Name != bound.Name || !zpkEmptyStringLit(cmp.Y) {
				return true
			}
			guarded[reading] = fset.Position(ifs.Pos()).String()
			// …and the body hands that value to a printer.
			ast.Inspect(ifs.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok || !zpkIsPrinter(zpkCalleeName(c)) {
					return true
				}
				for _, a := range c.Args {
					if id, ok := a.(*ast.Ident); ok && id.Name == bound.Name {
						printed[reading] = fset.Position(c.Pos()).String()
						return false
					}
				}
				return true
			})
			return true
		})
	}

	for reading, decl := range class {
		if printed[reading] != "" {
			continue
		}
		if where := guarded[reading]; where != "" {
			t.Errorf("%s (declared in %s) is read at %s and its value goes to no printer — the guard fires and the launch says NOTHING, which is the state this reading was filed to end. Hand the bound value to one of %v, or add the spelling you used to zpkPrinters", reading, filepath.Base(decl), where, zpkPrinters)
			continue
		}
		t.Errorf("%s (declared in %s) has NO guarded call site in this package's non-test sources — nothing reads it, so a launch on a box in that state prints nothing and the suite cannot tell it from a clean box (ranger-base-zpk8r finding 1). The shape is `if d := %s(); d != \"\" { warn(\"posse: %%s\\n\", d) }`", reading, filepath.Base(decl), reading)
	}
}

// ARM 2 (finding 2): the DEGRADED wording must not invert when the launch pin
// itself fell back to /dev/null.
//
// policyPinDegraded compares each policy file against gitSystemScopePinValue()
// — git's resolved system-scope path, OR gitSystemScopeFallback when
// resolution failed. In the fallback case a policy file pinning a REAL path
// is Stale, and the s69ep sentence then reads "git system scope is NOT
// carried into this session" and "A commit scanner or hook configured only in
// system scope will not run here". Both are false there: the policy tier is
// CARRYING system scope and it is the launch pin that would have dropped it.
//
// Reach is low — resolution must fail AND a policy file must pin a
// non-/dev/null value, which the shipped drop-in has not done since
// ranger-base-t301k — so this is a wording bead and not a defect bead. The
// test is a positive AND a negative arm over the same reading, because a fix
// that made both configurations read the same sentence would satisfy either
// arm alone.
func TestQAZpk8rPolicyTierWordingFollowsWhichTierCarriesSystemScope(t *testing.T) {
	t.Parallel()

	// The sentences that are true only when the LAUNCH pin carries a real
	// path and the policy tier takes it away.
	launchPinLost := []string{
		"git system scope is NOT carried into this session",
		"A commit scanner or hook configured only in system scope will not run here",
	}

	t.Run("launch pin resolved: the policy tier takes system scope away", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writePolicyFixture(t, dir, "managed-settings.d/10-posse-inlet-pin.json",
			`{"env":{"GIT_CONFIG_SYSTEM":"/dev/null"}}`)
		fs := policyGitSystemScopeFindings(dir, policyPinFixtureWant)
		d := policyPinDegraded(fs, policyPinFixtureWant)
		for _, w := range append([]string{"DEGRADED", `"/dev/null"`, policyPinFixtureWant}, launchPinLost...) {
			if !strings.Contains(d, w) {
				t.Errorf("the ordinary wording must still say %q — that is the reading ranger-base-s69ep ships:\n%s", w, d)
			}
		}
	})

	t.Run("launch pin fell back: the policy tier is what carries it", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writePolicyFixture(t, dir, "managed-settings.d/10-posse-inlet-pin.json",
			`{"env":{"GIT_CONFIG_SYSTEM":"/etc/gitconfig"}}`)
		fs := policyGitSystemScopeFindings(dir, gitSystemScopeFallback)
		if len(fs) != 1 || !fs[0].Stale || fs[0].Value != "/etc/gitconfig" {
			t.Fatalf("findings = %+v; want the real path named stale against the fallback", fs)
		}
		d := policyPinDegraded(fs, gitSystemScopeFallback)
		if d == "" {
			t.Fatal("a policy file disagreeing with the fallback still earns a line: the tier that reaches git is not the one this launch pinned")
		}
		for _, bad := range launchPinLost {
			if strings.Contains(d, bad) {
				t.Errorf("the line says %q with the launch pin itself at %s — the policy tier is CARRYING system scope here and the launch pin is what would have dropped it. A detective row that inverts its verdict is the failure mode this reading exists to end (ranger-base-zpk8r finding 2):\n%s", bad, gitSystemScopeFallback, d)
			}
		}
		for _, w := range []string{"DEGRADED", "POLICY TIER", `"/etc/gitconfig"`, gitSystemScopeFallback} {
			if !strings.Contains(d, w) {
				t.Errorf("the fallback wording must name %q:\n%s", w, d)
			}
		}
		if row := policyPinRowDetail(dir, gitSystemScopeFallback, fs); !strings.HasPrefix(row, "DEGRADED") {
			t.Errorf("the `posse gates` row reads this through the same line and must carry it: %q", row)
		}
	})
}

// ARM 3 (finding 3): a non-string JSON value is named as the token it is.
//
// json.Unmarshal of JSON `null` into a string is a no-op that returns NO
// error, so `"GIT_CONFIG_SYSTEM": null` took the v=="" path and the line said
// the file pinned "" — a value it does not carry. A number, an object and an
// array each DID reach the raw branch one line above, so the branch was never
// dead; null alone slipped past it. Cosmetic, and listed because it is output
// and because the arm was otherwise unpinned.
//
// The raw branch's own wording moved with the fix: %q over a JSON token reads
// as a string the file does not carry either, so a raw value is named as a
// JSON value and a string stays quoted. The string arm is the control, and it
// is here because this pin changed the formatter both arms go through.
func TestQAZpk8rNonStringPolicyValueIsNamedAsTheTokenItIs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		body  string
		value string
		raw   bool
		says  string
	}{
		{"null", `{"env":{"GIT_CONFIG_SYSTEM":null}}`, "null", true, "pins the JSON value null"},
		{"number", `{"env":{"GIT_CONFIG_SYSTEM":0}}`, "0", true, "pins the JSON value 0"},
		{"object", `{"env":{"GIT_CONFIG_SYSTEM":{"a":1}}}`, `{"a":1}`, true, `pins the JSON value {"a":1}`},
		{"array", `{"env":{"GIT_CONFIG_SYSTEM":[]}}`, "[]", true, "pins the JSON value []"},
		{"a string is still quoted", `{"env":{"GIT_CONFIG_SYSTEM":"/dev/null"}}`, "/dev/null", false, `pins "/dev/null"`},
		{"the empty string is still quoted, which is what tells it from an absent row", `{"env":{"GIT_CONFIG_SYSTEM":""}}`, "", false, `pins ""`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writePolicyFixture(t, dir, "managed-settings.d/10-posse-inlet-pin.json", tc.body)
			fs := policyGitSystemScopeFindings(dir, policyPinFixtureWant)
			if len(fs) != 1 || !fs[0].Stale {
				t.Fatalf("findings = %+v; want one stale finding — none of these is the launch pin's value", fs)
			}
			if fs[0].Value != tc.value || fs[0].Raw != tc.raw {
				t.Errorf("finding = {Value:%q Raw:%v}; want {Value:%q Raw:%v}", fs[0].Value, fs[0].Raw, tc.value, tc.raw)
			}
			d := policyPinDegraded(fs, policyPinFixtureWant)
			if !strings.Contains(d, tc.says) {
				t.Errorf("the line must say %q over a file carrying %s:\n%s", tc.says, tc.body, d)
			}
			if tc.raw && strings.Contains(d, `pins ""`) {
				t.Errorf("the line names the empty string over a file that carries no string at all (ranger-base-zpk8r finding 3):\n%s", d)
			}
		})
	}
}
