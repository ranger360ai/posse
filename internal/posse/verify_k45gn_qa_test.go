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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// bdRedirectingEnvVars are the environment variables MEASURED to repoint bd
// 0.50.3's store, minus `BEADS_DIR`. A shipped file that sets any of them
// puts a redirect into an environment some bd will read.
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
//
// "A shipped file" is not "a shipped Go file", and that difference cost this
// pin a hole (ranger-base-z541t): posse ships two env sets —
// examples/envs/default.env, which config.yaml names as `default_env` and so
// applies to NEW sessions by default, and examples/envs/projA.env — whose
// plain `KEY=VALUE` lines envs.go passes to the session VERBATIM, with no key
// filter anywhere in that file. One `BEADS_DB=` line there is a session-env
// row by construction AND crosses the cage (CageEnvNames forwards any Key
// matching cageEnvName), which is both halves of the paragraph above at once,
// in a file the Go walk below cannot see. So there are two readers here: the
// Go census over cmd/ and internal/, and k45gnAssertSeedEnvSetsShipNoStoreVar
// over the seed tree posse embeds. The operator's OWN env sets stay out of
// both, and that half of the old sentence was always true — input is not
// something posse ships.
var bdRedirectingEnvVars = []string{"BEADS_DB"}

// The four SHAPES a shipped GO file can put a redirecting variable into an
// environment with, and how a hit on each reads. One line each, because a
// hit names the shape it was found in and they are not repaired the same
// way. A shipped file that is not Go has a shape of its own and a reader of
// its own: k45gnHowSeedEnv, below.
//
// ranger-base-1fjq5: only the first of these was ever censused, while the
// comment above claimed all three — so the two shapes that carry a value to
// a bd bdStoreEnv never rebuilds, which is the whole reason the list is not
// empty, were the two nothing read. Both are BARE NAMES in the source: the
// `=` is added at render time (EnvVar) or never (the cage's `-e NAME`), so
// no `NAME=` literal exists for a prefix census to find. MEASURED
// 2026-09-11: a `BEADS_DB` row appended at herdrback.go:2495 and a
// `"BEADS_DB"` added to CageEnvNames' list both built clean and left this
// pin green. The fourth shape is the same hole one hop on, closed at the
// same time because the tree already builds cage names out of constants.
const (
	// `NAME=value`: the row bdStoreEnv builds for the bd children runOnce
	// launches, and the only shape that carries its own `=`.
	k45gnHowRow = "builds %s into a child environment (a `NAME=` row)"
	// EnvVar{Key: NAME}: the shape EVERY session-env row a shipped file
	// WRITES AS A GO LITERAL is built in (herdrback.go planLaunch,
	// runtimeprobe.go). envs.go builds the same rows out of a parsed line
	// rather than a literal and so is invisible to this arm — which is
	// correct for the file, and was wrong for the ROWS: an operator's env set
	// is input, but the seed's two are shipped, and they arrive through this
	// exact code path (ranger-base-z541t). k45gnHowSeedEnv is the arm that
	// reads them. CageEnvNames forwards any such Key that is a legal env
	// name, so this one shape reaches the session AND the cage.
	k45gnHowSession = "builds %s into a session environment (an EnvVar Key)"
	// A bare name in CageEnvNames' own list: the engine forwards `-e NAME`
	// and takes the value from the pane's environment, so the name is the
	// whole row.
	k45gnHowCage = "carries %s across the cage boundary (a CageEnvNames name)"
	// The two arms above read a LITERAL in the Key or the list, and cage.go's
	// list already carries two names that are neither (EnvLaunchHome,
	// EnvPersona — declared in herdrback.go, a file away). A constant is one
	// hop of indirection past a per-file census and the tree's own idiom for
	// exactly this, so the DECLARATION is the hit: censusing where the name
	// is minted needs no cross-file resolution and cannot be walked past by
	// spelling the use site `EnvVar{EnvBeadsDB, db}`.
	//
	// bdStoreEnvShed stays green under it: a `[]string{…}` is not a
	// string-valued spec, which is what this reads (the fixture table pins
	// both halves).
	k45gnHowConst = "declares %s as a Go string constant, one hop from an EnvVar Key or the cage list"
)

// k45gnCageCarryFunc is the cage carry list, identified by the function that
// BUILDS it rather than by its shape. It has to be: the list is bare names
// in a `[]string`, and bdStoreEnvShed (beads.go) is bare names in a
// `[]string` too — and that one SHEDS what it names, which is the opposite
// of carrying it (the fixture arm below pins that it must not hit). Where
// the literal sits is the only thing that tells the two apart.
//
// A rename would silently retire this arm, so it is not left on trust:
// k45gnAssertShapesReachTheCorpus reds if no shipped function by this name
// exists, or if the one that does stops naming BEADS_DIR.
const k45gnCageCarryFunc = "CageEnvNames"

// k45gnHit is one redirecting variable, found in one Go string literal, in
// one of the three shapes above.
type k45gnHit struct {
	v   string
	how string
	pos token.Pos
}

func (h k45gnHit) what() string { return fmt.Sprintf(h.how, h.v) }

// k45gnStoreVarHits returns the redirecting variables that the Go file
// `name` puts into an environment — at most one hit per variable, at its
// first occurrence, naming the shape it was found in.
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
//
// STRUCTURE, NOT JUST THE PREFIX (ranger-base-1fjq5). A `NAME=` literal is
// one of three shapes and the least used of them; the other two are read off
// the syntax tree, because a bare `"BEADS_DB"` is a redirect in an EnvVar Key
// or the cage list and its own opposite in bdStoreEnvShed. So position in the
// tree decides, never the spelling — which is also why the shed-list arm of
// the fixture table keeps passing.
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
		// EARLIEST hit wins rather than first-seen, so which shape gets
		// reported does not depend on the walk order of four readers.
		best := k45gnHit{}
		take := func(how string, pos token.Pos) {
			if best.pos == token.NoPos || pos < best.pos {
				best = k45gnHit{v: v, how: how, pos: pos}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BasicLit:
				if s, ok := k45gnLitString(n); ok && strings.Contains(s, v+"=") {
					take(k45gnHowRow, n.Pos())
				}
			case *ast.CompositeLit:
				for _, lit := range k45gnEnvVarKeyLits(n) {
					if s, ok := k45gnLitString(lit); ok && s == v {
						take(k45gnHowSession, lit.Pos())
					}
				}
			case *ast.ValueSpec:
				for _, lit := range k45gnSpecStringLits(n) {
					if s, ok := k45gnLitString(lit); ok && s == v {
						take(k45gnHowConst, lit.Pos())
					}
				}
			case *ast.FuncDecl:
				if n.Name.Name != k45gnCageCarryFunc || n.Body == nil {
					return true
				}
				for _, lit := range k45gnBodyStringLits(n.Body) {
					if s, ok := k45gnLitString(lit); ok && s == v {
						take(k45gnHowCage, lit.Pos())
					}
				}
			}
			return true
		})
		if best.pos != token.NoPos {
			out = append(out, best)
		}
	}
	return out
}

// k45gnLitString unquotes a string literal, so an escaped or raw spelling
// reads the same as the plain one and the source form is not the subject.
func k45gnLitString(lit *ast.BasicLit) (string, bool) {
	if lit == nil || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// k45gnEnvVarKeyLits returns the Key-position string literals of cl, if cl is
// an `EnvVar{…}` composite literal or a `[]EnvVar{…}` of them. Both spellings
// the tree uses are read: positional (`EnvVar{"BEADS_DIR", home}`,
// herdrback.go:2495) and keyed (`{Key: "BASH_ENV", Value: …}`, inletpin.go),
// with the element type elided inside a slice (cageinner.go, egress.go).
func k45gnEnvVarKeyLits(cl *ast.CompositeLit) []*ast.BasicLit {
	if k45gnIsEnvVarType(cl.Type) {
		if lit := k45gnKeyLit(cl); lit != nil {
			return []*ast.BasicLit{lit}
		}
		return nil
	}
	at, ok := cl.Type.(*ast.ArrayType)
	if !ok || !k45gnIsEnvVarType(at.Elt) {
		return nil
	}
	var out []*ast.BasicLit
	for _, e := range cl.Elts {
		ecl, ok := e.(*ast.CompositeLit)
		if !ok || (ecl.Type != nil && !k45gnIsEnvVarType(ecl.Type)) {
			continue
		}
		if lit := k45gnKeyLit(ecl); lit != nil {
			out = append(out, lit)
		}
	}
	return out
}

func k45gnIsEnvVarType(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name == "EnvVar"
	case *ast.SelectorExpr: // posse.EnvVar, from cmd/
		return t.Sel.Name == "EnvVar"
	}
	return false
}

// k45gnKeyLit is the Key field of one EnvVar literal: the keyed element if
// the literal is keyed, else element 0, which is Key's position in the
// struct (envs.go: `type EnvVar struct{ Key, Value string }`).
func k45gnKeyLit(cl *ast.CompositeLit) *ast.BasicLit {
	for i, e := range cl.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			id, ok := kv.Key.(*ast.Ident)
			if !ok || id.Name != "Key" {
				continue
			}
			lit, _ := kv.Value.(*ast.BasicLit)
			return lit
		}
		if i == 0 {
			lit, _ := e.(*ast.BasicLit)
			return lit
		}
	}
	return nil
}

// k45gnSpecStringLits returns the string literals a const or var spec binds
// DIRECTLY to a name — `const EnvPersona = "RHQ_PERSONA"` (herdrback.go:725),
// the idiom cage.go's carry list is already partly built from. A spec whose
// value is a composite literal is not one of these, which is what keeps
// bdStoreEnvShed (`[]string{"BEADS_DIR", "BEADS_DB"}`) out: that names what it
// SHEDS, and a shed list is the opposite of a mint.
func k45gnSpecStringLits(spec *ast.ValueSpec) []*ast.BasicLit {
	var out []*ast.BasicLit
	for _, v := range spec.Values {
		if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			out = append(out, lit)
		}
	}
	return out
}

func k45gnBodyStringLits(body *ast.BlockStmt) []*ast.BasicLit {
	var out []*ast.BasicLit
	ast.Inspect(body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			out = append(out, lit)
		}
		return true
	})
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
			// PARSED, not read. The claim is about an env row this tree
			// BUILDS, and every one of the four shapes bottoms out in a Go
			// string literal (beads.go bdStoreEnv, herdrback.go's EnvVar
			// rows, cage.go's carry list, and the constants that list is
			// partly built from) — so only string literals are censused, and
			// prose is structurally invisible.
			rel, _ := filepath.Rel(root, p)
			for _, h := range k45gnStoreVarHits(t, fset, p, nil) {
				hits = append(hits, filepath.ToSlash(rel)+":"+strconv.Itoa(fset.Position(h.pos).Line)+
					" "+h.what())
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
	k45gnAssertShapesReachTheCorpus(t, root)
	// The same claim over the corpus a Go walk cannot read (ranger-base-z541t).
	k45gnAssertSeedEnvSetsShipNoStoreVar(t)
}

// k45gnAssertShapesReachTheCorpus is the same liveness argument as the file
// count above, one level in: a reader aimed at a shape the tree no longer
// builds is satisfied by finding nothing, and reports the same zero as a
// reader that works. The `NAME=` arm needs no witness of its own — beads.go's
// bdStoreEnvShed doc comment quotes the prefix deliberately and the prose arms
// of the fixture table below turn red if that stops being invisible — but the
// three shapes added by ranger-base-1fjq5 are structural, and every one of
// them would go quiet on an ordinary rename: the Key field, the carry-list
// function, or the tree's habit of minting env names as constants.
//
// MEASURED 2026-09-11 by raising each floor and reading the count back: 54
// EnvVar Key literals and 252 string-valued const/var specs under cmd/ and
// internal/, herdrback.go's planLaunch the largest single source of the
// first. (envs.go contributes no Key — it builds one out of a parsed line,
// not a literal; the rows that line comes from are censused by
// k45gnAssertSeedEnvSetsShipNoStoreVar instead, which has liveness floors of
// its own.) Both floors are far under the counts so ordinary
// deletion does not red this, but renaming the Key field, or moving the
// session environment off the type, does.
func k45gnAssertShapesReachTheCorpus(t *testing.T, root string) {
	t.Helper()
	keys, specs, carry := 0, 0, []string(nil)
	sawCarry := false
	fset := token.NewFileSet()
	for _, dir := range []string{"cmd", "internal"} {
		if err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasSuffix(p, "_test.go") || filepath.Ext(p) != ".go" {
				return err
			}
			f, perr := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
			if perr != nil {
				t.Fatalf("parse %s: %v", p, perr)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.CompositeLit:
					keys += len(k45gnEnvVarKeyLits(n))
				case *ast.ValueSpec:
					specs += len(k45gnSpecStringLits(n))
				case *ast.FuncDecl:
					if n.Name.Name != k45gnCageCarryFunc || n.Body == nil {
						return true
					}
					sawCarry = true
					for _, lit := range k45gnBodyStringLits(n.Body) {
						if s, ok := k45gnLitString(lit); ok {
							carry = append(carry, s)
						}
					}
				}
				return true
			})
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if keys < 20 {
		t.Errorf("the census read %d EnvVar Key literals under cmd/ and internal/, want at least 20 — the session-env arm is matching a shape this tree no longer builds, and its zero above means nothing", keys)
	}
	if specs < 50 {
		t.Errorf("the census read %d string-valued const/var specs under cmd/ and internal/, want at least 50 — the constant arm is reading nothing, and a redirect minted as a constant would reach the cage list unseen", specs)
	}
	if !sawCarry {
		t.Errorf("no shipped func %s: the cage carry-list arm of the census is reading nothing. It finds the list BY FUNCTION NAME (k45gnCageCarryFunc) because the list's own shape is bdStoreEnvShed's shape — so point the constant at whatever builds the carry list now", k45gnCageCarryFunc)
	} else if !containsString(carry, "BEADS_DIR") {
		t.Errorf("func %s no longer names BEADS_DIR (%v) — the census is reading a function by that name but not the carry list, whose whole business is forwarding store names (ADR 0055 D1)", k45gnCageCarryFunc, carry)
	}
}

// ─── the shape that is not Go (ranger-base-z541t) ───────────────────────────

// k45gnHowSeedEnv is the fifth shape and the only one with no Go literal
// behind it: a plain `KEY=VALUE` line in an env set posse SHIPS.
const k45gnHowSeedEnv = "ships %s in a seeded env set (a KEY=VALUE line posse init lays down)"

// k45gnEnvHit is one redirecting variable found in one seeded env set, at
// the line that sets it. Separate from k45gnHit because there is no
// token.Pos here and inventing one would mean lying about a file the Go
// parser never read.
type k45gnEnvHit struct {
	v    string
	line int
}

func (h k45gnEnvHit) what() string { return fmt.Sprintf(k45gnHowSeedEnv, h.v) }

// k45gnSeedEnvHits returns the redirecting variables that the env set text
// `data` SETS — at most one hit per variable, at the first line that sets it.
//
// POSSE'S OWN PARSER, one line at a time. parseEnvLines (envs.go) is what
// actually turns these files into session rows, so it is what decides here:
// a census with its own idea of the format would red on lines posse ignores
// and, worse, stay green on lines it honours (`export BEADS_DB=…` is the
// standing example — a hand-rolled `strings.HasPrefix(line, v+"=")` walks
// straight past it, and envs.go does not). parseEnvLines splits on "\n" and
// carries nothing across lines, so feeding it one line yields exactly the row
// the whole file would, plus the number to report it at.
//
// The KEY is compared with surrounding space trimmed, which is WIDER than
// what posse would set. Deliberate: whether the `--env KEY=VALUE` argv
// herdr.CreateWorkspace builds treats " BEADS_DB" as BEADS_DB is not measured
// here, and a guard that has not measured the difference fails closed. A
// leading space is not a spelling any seeded row has a reason to carry, so
// the cost of the wider match is zero and the cost of the narrow one is a
// redirect this pin would call green. (The cage half needs no such care:
// cageEnvName rejects the space outright.)
func k45gnSeedEnvHits(data string) []k45gnEnvHit {
	var out []k45gnEnvHit
	seen := map[string]bool{}
	for i, line := range strings.Split(data, "\n") {
		for _, row := range parseEnvLines(line) {
			for _, v := range bdRedirectingEnvVars {
				if strings.TrimSpace(row.Key) == v && !seen[v] {
					seen[v] = true
					out = append(out, k45gnEnvHit{v: v, line: i + 1})
				}
			}
		}
	}
	return out
}

// k45gnAssertSeedEnvSetsShipNoStoreVar is the Go census's claim over the one
// corpus the Go census cannot read: the env sets posse SHIPS.
//
// It reads posse.Seed — the embedded tree, not examples/ on disk — because
// the embed is what a release binary lays down on a laptop with no repo
// (embed.go, ADR 0012 D5), and because reading it by its own root means no
// path here to drift out of step with a rename of the directory.
//
// Every `*.env` in the seed is censused, not just envs/: init copies envs/
// (init.go seedRoots) and that is where these two live, but a set that ships
// anywhere in the tree is a set somebody can point `default_env` at, and the
// walk costs the same.
func k45gnAssertSeedEnvSetsShipNoStoreVar(t *testing.T) {
	t.Helper()
	var hits []string
	files, rows := 0, 0
	err := fs.WalkDir(posse.Seed, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".env" {
			return err
		}
		b, rerr := fs.ReadFile(posse.Seed, p)
		if rerr != nil {
			t.Fatalf("read seed %s: %v", p, rerr)
		}
		files++
		rows += len(parseEnvLines(string(b)))
		for _, h := range k45gnSeedEnvHits(string(b)) {
			hits = append(hits, p+":"+strconv.Itoa(h.line)+" "+h.what())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) > 0 {
		t.Errorf(`a SEEDED env set now sets an environment variable that repoints bd's
store and that bdStoreEnv (beads.go) does not shed:

  %s

`+"`posse init`"+` copies these into RHQ_HOME/envs, config.yaml names default.env
as `+"`default_env`"+`, and envs.go passes every row to the session VERBATIM — there
is no key filter in that file. So the row reaches a session bd that never
passes through bdStoreEnv, and CageEnvNames (cage.go) forwards the same Key
across the cage boundary. Delete the line; an operator who wants this writes
it in their OWN env set, which is input and is not censused.`, strings.Join(hits, "\n  "))
	}
	// LIVENESS, the same argument the Go walk makes: a reader that found no
	// files reports the same zero as a reader that works. MEASURED 2026-09-11
	// (darwin/arm64, go1.26.5) by raising each floor and reading the count
	// back: 2 seeded .env files, 4 KEY=VALUE rows between them. The floors sit
	// under both so deleting an example set does not red this, but an empty
	// embed, a seed with no env sets left, or a parseEnvLines that stops
	// yielding rows does.
	if files < 1 {
		t.Errorf("the census read %d seeded .env files, want at least 1 — posse.Seed carries none, so the zero above is a census of nothing", files)
	}
	if rows < 2 {
		t.Errorf("the census read %d KEY=VALUE rows across the seeded env sets, want at least 2 — the reader is parseEnvLines (envs.go) and it is yielding nothing, so a BEADS_DB= line would read as no row at all", rows)
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
	}, {
		// ranger-base-1fjq5, from here down: the two shapes that carry a
		// redirect to a bd bdStoreEnv never rebuilds. Both are BARE names —
		// which is why the shed-list arm above has to keep passing, and why
		// position in the tree, not spelling, is what decides.
		name: "a session-env row hits: the positional EnvVar shape herdrback.go builds every row in",
		src:  "func f(vars []EnvVar, db string) []EnvVar { return append(vars, EnvVar{\"BEADS_DB\", db}) }\n",
		want: true,
	}, {
		name: "a keyed EnvVar row hits",
		src:  "func f(db string) []EnvVar { return []EnvVar{{Key: \"BEADS_DB\", Value: db}} }\n",
		want: true,
	}, {
		name: "an EnvVar row with the element type elided inside a slice hits",
		src:  "func f(db string) []EnvVar { return []EnvVar{{\"RHQ_HOME\", db}, {\"BEADS_DB\", db}} }\n",
		want: true,
	}, {
		name: "the qualified spelling hits: a cmd/ file builds posse.EnvVar",
		src:  "func f(db string) posse.EnvVar { return posse.EnvVar{\"BEADS_DB\", db} }\n",
		want: true,
	}, {
		name: "the VALUE half of an EnvVar row does not hit: it is a value, not a variable name",
		src:  "func f() EnvVar { return EnvVar{\"BD_ACTOR\", \"BEADS_DB\"} }\n",
		want: false,
	}, {
		name: "a bare name in some other []string does not hit: that is bdStoreEnvShed's shape",
		src:  "func f() []string { return []string{\"BEADS_DIR\", \"BEADS_DB\"} }\n",
		want: false,
	}, {
		name: "the cage carry list hits: a bare name in CageEnvNames",
		src:  "func CageEnvNames(vars []EnvVar) []string { return []string{\"BD_ACTOR\", \"BEADS_DIR\", \"BEADS_DB\"} }\n",
		want: true,
	}, {
		name: "the same list under any other function name does not hit",
		src:  "func someOtherList(vars []EnvVar) []string { return []string{\"BD_ACTOR\", \"BEADS_DIR\", \"BEADS_DB\"} }\n",
		want: false,
	}, {
		name: "a constant minting the name hits: the EnvPersona idiom the cage list already uses",
		src:  "const EnvBeadsDB = \"BEADS_DB\"\n",
		want: true,
	}, {
		name: "a package var minting the name hits too",
		src:  "var envBeadsDB = \"BEADS_DB\"\n",
		want: true,
	}, {
		name: "a typed const spec hits: the type is not the subject",
		src:  "type envName string\n\nconst beadsDB envName = \"BEADS_DB\"\n",
		want: true,
	}, {
		name: "the shed list is a spec too and still does not hit: its value is a composite, not a string",
		src:  "var bdStoreEnvShed = []string{\"BEADS_DIR\", \"BEADS_DB\"}\n",
		want: false,
	}, {
		name: "prose inside CageEnvNames is still prose",
		src:  "func CageEnvNames() []string {\n\t// BEADS_DB is deliberately not carried\n\treturn nil\n}\n",
		want: false,
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

// The seed-env census's claim, in a table (ranger-base-z541t): a line that
// SETS the variable hits; a line posse's own parser does not turn into a row
// does not.
//
// The arms are chosen against the two ways a hand-rolled reader gets this
// wrong. Too narrow: `export BEADS_DB=` and a row further down the file are
// both real rows envs.go builds, and a `HasPrefix(line, "BEADS_DB=")` census
// misses the first. Too wide: a commented-out line is the shape
// examples/envs/projA.env already ships (`# API_TOKEN=…`), so a census that
// reds on it reds on the seed as it stands today.
func TestQASeedEnvCensusReadsPosseOwnEnvParser(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		src  string
		want int // 0 = no hit, else the line the hit must name
	}{{
		name: "a plain row hits",
		src:  "BEADS_DB=/tmp/other/beads.db\n",
		want: 1,
	}, {
		name: "the export spelling hits: parseEnvLines tolerates the prefix, so a prefix census would miss a live row",
		src:  "export BEADS_DB=/tmp/other/beads.db\n",
		want: 1,
	}, {
		name: "a row further down names its own line",
		src:  "EDITOR=vim\n\n# a comment\nBEADS_DB=/tmp/other/beads.db\n",
		want: 4,
	}, {
		name: "a commented row is prose: the shape projA.env already ships",
		src:  "# BEADS_DB=/tmp/other/beads.db\n",
		want: 0,
	}, {
		name: "an indented comment is prose too",
		src:  "   # BEADS_DB=/tmp/other/beads.db\n",
		want: 0,
	}, {
		name: "the name in a VALUE does not hit: the key is what gets set",
		src:  "NOTE=BEADS_DB=/tmp/other/beads.db\n",
		want: 0,
	}, {
		name: "a key that merely contains the name does not hit",
		src:  "MY_BEADS_DB=/tmp/other/beads.db\n",
		want: 0,
	}, {
		name: "a leading space still hits: the census fails closed on whitespace it has not measured herdr's argv against",
		src:  " BEADS_DB=/tmp/other/beads.db\n",
		want: 1,
	}, {
		name: "a line with no = is not a row",
		src:  "BEADS_DB\n",
		want: 0,
	}, {
		name: "the seeded sets as they stand today do not hit",
		src:  "EDITOR=vim\nPAGER=less\nNODE_ENV=development\nAPI_BASE=http://localhost:8080\n# API_TOKEN=put-real-secrets-in-your-own-copy\n",
		want: 0,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hits := k45gnSeedEnvHits(tc.src)
			if tc.want == 0 {
				if len(hits) > 0 {
					t.Errorf("k45gnSeedEnvHits = %v, want no hit, over:\n%s", hits, tc.src)
				}
				return
			}
			if len(hits) != 1 || hits[0].line != tc.want {
				t.Errorf("k45gnSeedEnvHits = %v, want one hit at line %d, over:\n%s", hits, tc.want, tc.src)
			}
		})
	}
}
