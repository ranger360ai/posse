//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-k45gn, verifying the close of ranger-base-ub2x9.
//
// ub2x9 bound every bd child to the store of the directory its caller named:
// bdStoreEnv drops an inherited `BEADS_DIR` and sets beadsHome(dir). The
// done-when was "so an inherited environment cannot redirect a query", and
// `BEADS_DIR` is not the only environment variable that redirects one.
//
// MEASURED 2026-09-10 against the pinned bd 0.50.3, production argv
// (`--no-daemon`), from a git root holding no `.beads`, each arm with a
// baseline and a control:
//
//	BEADS_DIR unset, no store           -> "Error: no beads database found"   (baseline)
//	BEADS_DB=<another repo>/beads.db    -> that repo's rows                   (REDIRECTS)
//	BEADS_DB=<a corrupt file>           -> "sqlite3: file is not a database"  (path honoured)
//	BEADS_JSONL=<another repo>/*.jsonl  -> baseline error                     (inert)
//	--db <another repo>/beads.db        -> that repo's rows                   (flag control)
//
// So an inherited `BEADS_DB` survives bdStoreEnv's rebuild and points the
// child at another repository's store, which ReadyAll then labels
// `RepoIssue{Dir: dir}` with the directory it meant to query — ub2x9's own
// masquerade, one spelling over. This tree already knows the precedence:
// bddepsafety_qa_test.go says "BEADS_DB wins over discovery, from any cwd"
// and both it and bdrelatesprune_qa_test.go strip the variable from their
// child environments for exactly this reason.
//
// The hole is DEBT rather than a live defect for one reason only: no Go
// file posse ships ever puts `BEADS_DB` into a child environment, so the
// value can only arrive from outside the process tree. THAT is what this pin
// holds. It is deliberately not a pin on bdStoreEnv's shed list — widening
// that list is the code lane's fix and would move such a pin — but on the
// reason the gap is currently harmless. The day a shipped Go file builds one
// of these into a child env, this reds and names it, and the failure message
// spells the fix.
//
// FIXED IN THE CODE LANE, ranger-base-tqcrp (2026-09-11): bdStoreEnv now
// sheds `BEADS_DB` beside `BEADS_DIR` (bdStoreEnvShed, beads.go), pinned by
// TestBdStoreEnvReplacesEveryInheritedBinding and
// TestBdChildrenNeverInheritAStoreDatabase in beadsstorebind_test.go. This
// pin did not move and its list did not shrink; bdRedirectingEnvVars below
// says why, and what it still guards that the runner cannot.
//
// SCOPE, and why it is Go only. bdStoreEnv governs exactly one thing: the
// environment runOnce hands the bd children posse launches. A shell script
// that runs bd itself never passes through it. `scripts/prune-bd-relates-to.sh`
// and `scripts/verify-bd-dep-safety.sh` both READ `${BEADS_DB:-}` as their
// own documented operator input, which is correct and is not this claim —
// censusing them would red on the day someone documents a variable.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// bdRedirectingEnvVars are the environment variables MEASURED to repoint bd
// 0.50.3's store, minus `BEADS_DIR`. A shipped file that sets any of them
// puts a redirect into a child environment.
//
// BEADS_DIR is off the list because beads.go legitimately BUILDS that row:
// bdStoreEnv SETS it, which is ranger-base-ub2x9's own fix, so censusing it
// would name beads.go every run. That is also how this pin was
// control-checked — aimed at BEADS_DIR it reds, and names beads.go.
//
// Do NOT shorten this list on the grounds that bdStoreEnv now sheds
// BEADS_DB too (ranger-base-tqcrp). That runner governs exactly one thing:
// the environment runOnce hands the bd children posse launches. A shipped
// file that builds one of these into a SESSION environment (planLaunch,
// herdrback.go) or into cage.go's carry list reaches a bd that never passes
// through bdStoreEnv at all, and this census is the only thing watching for
// it. Emptying the list would leave the pin green over nothing.
var bdRedirectingEnvVars = []string{"BEADS_DB"}

// k45gnHit is one redirecting variable, found in one Go string literal.
type k45gnHit struct {
	v   string
	pos token.Pos
}

// k45gnStoreVarHits returns the redirecting variables whose child-env
// spelling, `NAME=`, appears in a STRING LITERAL of the Go file `name` — at
// most one hit per variable, at its first occurrence.
//
// PARSED, NOT SCANNED FOR BYTES (ranger-base-15txf). This census used to be
// strings.Contains over the file bytes for a quoted `"NAME=` prefix, and its
// own comment claimed what that cannot deliver: "A bare mention in a comment
// must not red this pin". It did. MEASURED 2026-09-11: a doc comment on
// bdStoreEnvShed (beads.go) explaining why the shed list is spelled as names
// — prose, no code — turned the pin red, naming a row beads.go does not
// build. The workaround then was to reword the prose; the fix is here, and
// that paragraph now quotes the prefix on purpose as the corpus witness.
//
// Reading literals rather than bytes is the documented claim exactly, and it
// is also WIDER in the one direction that matters: the byte scan needed the
// opening quote immediately before the name, so `"--setenv=BEADS_DB="`, the
// shape cage.go's carry list is built from, walked straight past it. Any
// literal containing the prefix hits now, however it is assembled.
func k45gnStoreVarHits(t *testing.T, fset *token.FileSet, name string, src any) []k45gnHit {
	t.Helper()
	// The READ is inside this helper on purpose. It is the whole subject —
	// "parsed literals, not file bytes" — and a caller that handed in an
	// already-parsed file would make the prose arms of
	// TestQAStoreVarCensusReadsParsedLiteralsNotFileBytes unfailable: no
	// mutation of a census that never touches the source can red them.
	// src is nil to read `name` off disk, or the source itself for a fixture.
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var out []k45gnHit
	for _, v := range bdRedirectingEnvVars {
		var at token.Pos
		ast.Inspect(f, func(n ast.Node) bool {
			if at != token.NoPos {
				return false
			}
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			// Unquote, so an escaped or raw spelling reads the same as the
			// plain one and the source form is not the subject.
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if strings.Contains(s, v+"=") {
				at = lit.Pos()
			}
			return true
		})
		if at != token.NoPos {
			out = append(out, k45gnHit{v: v, pos: at})
		}
	}
	return out
}

func TestQANoShippedFileSetsAStoreVarBdStoreEnvDoesNotShed(t *testing.T) {
	t.Parallel()
	root := k45gnRepoRoot(t)
	fset := token.NewFileSet()
	var hits []string
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			// Test files are exempt BY DESIGN: two of them strip these
			// variables on purpose, and one asserts the precedence this pin
			// is derived from. The claim is about what posse SHIPS.
			if strings.HasSuffix(p, "_test.go") {
				return nil
			}
			if filepath.Ext(p) != ".go" {
				return nil
			}
			// PARSED, not read. The claim is about a child-env row this
			// tree BUILDS, and a row is built out of a Go string literal
			// (beads.go bdStoreEnv, cage.go's carry list) — so only string
			// literals are censused, and prose is structurally invisible.
			rel, _ := filepath.Rel(root, p)
			for _, h := range k45gnStoreVarHits(t, fset, p, nil) {
				hits = append(hits, filepath.ToSlash(rel)+":"+strconv.Itoa(fset.Position(h.pos).Line)+
					" builds "+h.v+" into a child environment")
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(hits) > 0 {
		t.Errorf(`a shipped Go file now builds an environment variable that repoints bd's
store and that bdStoreEnv (beads.go) does not shed:

  %s

That is ranger-base-ub2x9 through a second spelling. bdStoreEnv (beads.go)
sheds these for the bd children runOnce launches, so a row built for THAT
environment is already covered — but a row built for a session environment
(planLaunch, herdrback.go), or carried into the cage (cage.go), reaches a bd
this runner never rebuilds, and ReadyAll then labels the rows it returns with
the directory it MEANT to query. Either do not build the row, or shed it
where that environment is assembled. (MEASURED on bd 0.50.3: BEADS_DB
redirects; BEADS_JSONL does not.)`, strings.Join(hits, "\n  "))
	}
	// LIVENESS. A walk that read nothing is satisfied by sweeping nothing,
	// so prove the corpus was actually reached before believing the zero.
	// 122 shipped .go files at 2026-09-10; a floor well under it so an
	// ordinary deletion does not red this, but a walk that landed in the
	// package directory (0) or on the wrong tree does.
	if n := k45gnCountScanned(t, root); n < 80 {
		t.Errorf("the walk read %d shipped .go files under cmd/ and internal/, want at least 80 — it is reading the wrong tree and the census above measured nothing", n)
	}
}

func k45gnRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test working directory")
		}
		dir = parent
	}
}

func k45gnCountScanned(t *testing.T, root string) int {
	t.Helper()
	n := 0
	for _, dir := range []string{"cmd", "internal"} {
		if err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasSuffix(p, "_test.go") {
				return err
			}
			if filepath.Ext(p) == ".go" {
				n++
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return n
}

// The census's claim, in a table: a shipped file that BUILDS a child-env row
// hits; prose that merely quotes the prefix does not.
//
// ranger-base-15txf. The two prose arms are the defect this pin was rebuilt
// for — a byte census reds on both — and the remaining arms are what the byte
// census caught and must keep catching, plus the two shapes it MISSED (the
// prefix not at the start of the literal, and an escaped spelling).
func TestQAStoreVarCensusReadsParsedLiteralsNotFileBytes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		src  string
		want bool
	}{{
		name: "a doc comment quoting the prefix is prose, not a build",
		src:  "// spelled as names, never as \"BEADS_DB=\", on purpose.\nvar shed = []string{\"BEADS_DB\"}\n",
		want: false,
	}, {
		name: "a comment inside a function body is prose too",
		src:  "func f(env []string) {\n\t// strip \"BEADS_DB=\" rows from the child env\n\t_ = env\n}\n",
		want: false,
	}, {
		name: "the shed list names the variable without the = and does not hit",
		src:  "var shed = []string{\"BEADS_DIR\", \"BEADS_DB\"}\n",
		want: false,
	}, {
		name: "the plain child-env row hits",
		src:  "func f(env []string, db string) []string { return append(env, \"BEADS_DB=\"+db) }\n",
		want: true,
	}, {
		name: "a Sprintf template hits",
		src:  "func f(db string) string { return fmt.Sprintf(\"BEADS_DB=%s\", db) }\n",
		want: true,
	}, {
		name: "the prefix inside a longer literal hits: the cage carry-list shape the byte scan walked past",
		src:  "func f(db string) string { return \"--setenv=BEADS_DB=\" + db }\n",
		want: true,
	}, {
		name: "a raw string literal hits",
		src:  "func f(db string) string { return `BEADS_DB=` + db }\n",
		want: true,
	}, {
		name: "an escaped spelling hits: the literal is unquoted before it is read",
		src:  "func f(db string) string { return \"\\x42EADS_DB=\" + db }\n",
		want: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hits := k45gnStoreVarHits(t, token.NewFileSet(), "fixture.go", "package posse\n"+tc.src)
			if got := len(hits) > 0; got != tc.want {
				t.Errorf("k45gnStoreVarHits = %v, want hit=%v, over:\n%s", hits, tc.want, tc.src)
			}
		})
	}
}
