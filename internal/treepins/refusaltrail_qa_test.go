package treepins

// QA pin for ranger-base-u5ez8 — every host-side ACCESS to the refusals
// trail goes through the guard, and the pin's subject is the SET of them.
//
// THE CLASS, and why a per-call-site pin was not enough. The host's own
// opens of the refusals trail are guarded one call site at a time, with
// symlink, dangling, hardlink and FIFO plants: MEASURED 2026-11-22, fifteen
// tests over four files — internal/posse/refusalleaf_test.go (8),
// refusalparent_test.go (3), cmd/posse/gatesrefusalread_qa_test.go (2) and
// runtimeprobe_test.go's delta pair. Not one of them has the SET as its
// subject, so a new access arrives guarded by nothing and reads green — and
// the class has now escaped by exactly that route three times:
//
//   - ranger-base-legou guarded four opens (RefusalsLog,
//     appendCanonicalRefusal, EnsureCageSpool, the fold's spool read). Its
//     census said "no other uncaged Go WRITER to either leaf" — true as
//     stated, because its subject was the writers.
//   - ranger-base-byqv0: the same leaves' PARENT directories, and the
//     cursor's own os.MkdirAll + os.WriteFile, unguarded.
//   - ranger-base-jielm: cmd/posse/main.go's os.ReadFile of the canonical
//     leaf, and internal/posse/runtimeprobe.go's fileSize/readFrom pair —
//     the second found by censusing the leaf rather than by taking the call
//     site the bead named, and the worse of the two, because that delta is
//     the entire evidence for `runtime probe`'s observable 2.
//
// THE THREE PATHS. gates/<persona>/refusals.log (the canonical trail),
// state/cages/<persona>/<session>/refusals.log (a container session's
// spool, the one file CageMounts binds rw) and
// gates/<persona>/spool-cursors/<session> (the fold's own bookkeeping).
// All three are opened UNCAGED by the host over a name a planter outside
// every wall may have replaced, so all three go through
// openRefusalLeaf / readRefusalLeaf / refusalLeafSize / refusalDeltaAfter
// (internal/posse/refusalleaf.go) and nothing else.
//
// WHY THE RULE IS THE DESIGN, said as the two rules that do NOT work:
//
//   - A string-literal grep for `refusals.log` is hopeless. MEASURED
//     2026-11-22 over non-test .go under cmd/ and internal/: 88 lines name
//     `refusals.log` and 4 name `spool-cursors`, against six real
//     accesses. Nearly all of the 92 are prose comments or the rendered
//     shim TEXT a SEAT appends with — gates.go bakes the path into shell it
//     writes at seven sites, and those lines are inside the wall and are
//     not the host's opens.
//   - Keying on the path HELPERS (RefusalsLogPath, CageSpoolPath,
//     spoolCursorPath, GatesDir) misses runtimeprobe.go, which built the
//     path with its own filepath.Join of a gatesDir it already had — i.e.
//     it misses the one that actually escaped.
//
// SO THE RULE IS ASKED FROM THE OPEN SIDE, with one closure. An access is a
// call in the os / unix / syscall / ioutil open family, or a call to a
// function of this tree's own one of whose parameters reaches one. A
// finding is such a call whose argument is a TRAIL EXPRESSION — a
// filepath.Join or a `+` one of whose elements is a trail path COMPONENT, a
// call to a function that returns one of those, a pure path wrapper over
// either, or a local bound to any of them — where the callee is not one of
// the guarded entries.
//
// A bare string literal is not a trail expression, however exactly it names
// one. That single condition is what separates this census from the grep
// above: the Join or the `+` round a component is what says a path is being
// BUILT, and without it the rule derives six more "producers" out of this
// tree, every one a renderer of prose or of shim text, and five findings
// behind them (rtlTrailComponent has the measurement).
//
// THE CLOSURE IS NOT OPTIONAL: it is what finds two of the three escapes.
// byqv0's cursor writer was `func writeSpoolCursor(path string, c
// spoolCursor)` doing os.MkdirAll + os.WriteFile on `path`, and jielm's
// probe pair was `fileSize(path)` / `readFrom(path, off)` doing os.Stat and
// os.ReadFile. In both the open is in a function that spells no trail
// literal at all; the trail path is spelled at the CALL SITE one frame up.
// A rule that reads one function body at a time is green over both, which
// is the shape this file exists to refuse.
//
// WHAT A GREEN HERE PROVES, exactly, and what it does not. This is a
// SYNTACTIC census over the parsed non-test source of internal/ and cmd/ —
// no type checker, no build — so it buys precision on the shapes it names
// and blindness to the shapes it does not:
//
//   - A trail path that never spells a trail literal anywhere is invisible.
//     A path assembled from fragments ("refusals" + ".log"), read out of a
//     config value, or handed in from outside these two trees is not found
//     by any rule here. The complementary guard for that direction is the
//     per-call-site plant tests, which judge behaviour rather than
//     spelling.
//   - Dataflow is LOCAL plus one interprocedural hop per call, repeated to
//     a fixpoint over the corpus. A trail path parked in a struct field, a
//     map, a slice or a channel and opened elsewhere is invisible — which
//     is why cage.go's `Src: a.CageSpoolPath(…)` does not appear below:
//     that is a mount the host never opens, and it would not appear if it
//     were one.
//   - Within a function, dataflow is flow-INSENSITIVE but SCOPE-sensitive:
//     an identifier that ever held a trail path counts as holding one for
//     as long as that binding is live, and a range variable's binding ends
//     with its `for`. The second half is not decoration — see trailLocal.
//   - Callees resolve by NAME, receiver ignored, so a call reaches every
//     same-named function in the tree. That OVER-approximates: the set of
//     openers is bigger than the truth and a green is therefore stronger
//     than the call graph. absencerules_qa_test.go's pin 3 reads the same
//     way for the same reason.
//   - An open's path POSITION is not modelled: every argument of an
//     open-family call is treated as a path. Harmless in this direction —
//     a mode, a []byte or an offset is never a trail expression — and it
//     costs no per-function arity table that could go stale.
//   - TEST sources are not read, deliberately. refusalleaf_test.go and
//     refusalparent_test.go plant exactly these shapes by the dozen,
//     because planting them is their whole job, so a census over tests
//     would be mostly fixtures and would stop saying anything.
//   - A FOURTH file joining the trail is invisible until rtlTrailLiterals
//     names it. The producer floor below is the nearest thing to a guard
//     on that: a path helper added beside the three is derived, and one
//     that stops being derived reds.
//
// A THIRD ANSWER, reported and never failed: a PASS-THROUGH, a call handed
// a trail path whose callee neither opens it nor is a guard. The path
// printed by `posse gates`, baked into the shim text a seat appends with,
// listed in a seatbelt carve-out, or handed to a helper that opens it
// through a guard two frames down. Those are the non-accesses the literal
// grep could not tell from an access, and they are logged rather than
// registered — what makes them safe is the arm above and not a row here, so
// there is no register to go stale and no row anybody has to write.
//
// The rule is shown able to fail by TestQARefusalTrailCensusCatchesEachEscape
// below, which runs the same scanner over a clean fixture and then over six
// plants: all four of the historical escapes, as they were actually written
// — legou's create on a plain os.OpenFile, jielm's `posse gates` read by
// concatenation and its probe pair through a parameter, byqv0's cursor
// write through a parameter — plus byqv0's unguarded parent MkdirAll and a
// two-hop chain to the open, which the four say nothing about on their own.
// That test is deliberately NOT in a `-run` door variable: it reads planted
// fixtures rather than the tree, so nothing derives it and no unrelated
// bead can red it (`make treepins-check` runs it with the rest of the
// package).

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// rtlGuards is the guarded entry set — the only callees that may be handed
// a trail path. Spelled once, and pinned to exist by the arm below: a
// rename that emptied this set would turn every access in the tree into a
// finding, which is loud, but a rename that emptied it while the openers
// were renamed with it would not be, so the names are asserted present.
//
// openRefusalLeaf, readRefusalLeaf, refusalLeafSize and refusalDeltaAfter
// are the four ranger-base-u5ez8 names. The other two are the same guard
// from the inside and the outside: openRefusalDirAt is the walk's own
// per-component openat, reached only from openRefusalLeaf, and
// ReadRefusalsLog is the exported wrapper `posse gates` reads the canonical
// trail through (ranger-base-jielm). All six are in
// internal/posse/refusalleaf.go, which is the whole of the guard.
var rtlGuards = map[string]string{
	"openRefusalLeaf":   "the walk and the O_NOFOLLOW open, judged on the fd (ranger-base-legou, ranger-base-byqv0)",
	"openRefusalDirAt":  "one component of that walk, openat(O_DIRECTORY|O_NOFOLLOW) relative to the last fd",
	"readRefusalLeaf":   "os.ReadFile through openRefusalLeaf",
	"refusalLeafSize":   "the probe's delta read, half 1 — fails closed at 0 (ranger-base-jielm)",
	"refusalDeltaAfter": "the probe's delta read, half 2 — fails closed at \"\" (ranger-base-jielm)",
	"ReadRefusalsLog":   "the canonical trail's bytes for `posse gates` (ranger-base-jielm)",
}

// rtlTrailLiterals are the path components that name the refusals trail.
// `refusals.log` is both the canonical leaf and a session's spool;
// `spool-cursors` is the directory the fold's cursors live in, and the
// cursor leaf itself is a bare session id, so the DIRECTORY is the only
// thing about that path a census can spell.
var rtlTrailLiterals = []string{"refusals.log", "spool-cursors"}

// rtlOpenFamily is the open family, by package and function name. Every
// entry either resolves a path to an inode or acts on the one it resolves —
// stat, read, write, create, remove, truncate, rename, link, mkdir — which
// is the whole of what "an access to the trail" means here. filepath's own
// pure arithmetic (Join, Dir, Clean) is NOT in it; those are how a trail
// path is SPELLED and are read as such by rtlTrailExpr.
//
// MkdirAll is in it because of ranger-base-byqv0: the parents were the
// escape, and a MkdirAll that walks to a planted directory aims whatever
// opens the leaf next.
var rtlOpenFamily = map[string]map[string]bool{
	"os": rtlSet(
		"Open", "OpenFile", "OpenInRoot", "Create", "CreateTemp", "ReadFile", "WriteFile",
		"Stat", "Lstat", "Readlink", "Remove", "RemoveAll", "Truncate", "Rename",
		"Link", "Symlink", "Chmod", "Chown", "Lchown", "Chtimes", "Mkdir", "MkdirAll",
		"ReadDir", "Append", "Root",
	),
	"ioutil": rtlSet("ReadFile", "WriteFile", "ReadDir"),
	"unix": rtlSet(
		"Open", "Openat", "Openat2", "Creat", "Stat", "Lstat", "Fstatat", "Statx",
		"Access", "Faccessat", "Readlink", "Readlinkat", "Unlink", "Unlinkat",
		"Truncate", "Rename", "Renameat", "Link", "Linkat", "Symlink", "Symlinkat",
		"Mkdir", "Mkdirat", "Chmod", "Fchmodat", "Chown", "Fchownat", "Utimes",
	),
	"syscall": rtlSet(
		"Open", "Openat", "Stat", "Lstat", "Access", "Readlink", "Unlink",
		"Truncate", "Rename", "Link", "Symlink", "Mkdir", "Chmod", "Chown",
	),
}

// rtlSet is a name set from a list, spelled once for the tables above.
func rtlSet(names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

// ---------------------------------------------------------------------------
// The corpus: one record per function, with the local dataflow already run.
// ---------------------------------------------------------------------------

// rtlFunc is one function or method of the corpus.
type rtlFunc struct {
	rel    string   // slash-separated path, relative to the walk root
	name   string   // bare name; a method's receiver is not part of it
	params []string // parameter names in order, receiver excluded

	// paramOf maps an identifier in the body to the parameter indices its
	// value may have come from: the parameters themselves, plus every local
	// assigned from an expression mentioning one, to a fixpoint. This is
	// what lets the closure below ask "which of MY parameters reach an
	// open" rather than "does this function open anything".
	paramOf map[string]map[int]bool

	// trailLocal is the identifiers in the body holding a trail path:
	// assigned from a producer call, from a Join naming a trail component,
	// or from another trail local. SCOPED, by the source range the binding
	// is live over — the function body for an assignment, the `for`
	// statement for a range variable. Without that scope one `for _, f :=
	// range []string{"refusals.log", "shell.log"}` made every OTHER `f` in
	// the same function a trail path: MEASURED 2026-11-22, six spurious
	// pass-through rows in seatbelt.go from one name collision, and had any
	// of those six callees been an opener they would have been six
	// findings.
	trailLocal []rtlBinding

	// calls is every call in the body, in source order.
	calls []rtlCall

	// opens is the parameter indices that reach an UNGUARDED open — filled
	// by the fixpoint in rtlScan, empty for a guard and for anything that
	// only ever opens what it spells itself.
	opens map[int]bool
}

// rtlBinding is one identifier holding a trail path, live over [from, to).
type rtlBinding struct {
	name     string
	from, to token.Pos
}

// rtlHoldsTrail reports whether an identifier at a position is one of this
// function's trail locals.
func (f *rtlFunc) rtlHoldsTrail(name string, at token.Pos) bool {
	for _, b := range f.trailLocal {
		if b.name == name && at >= b.from && at < b.to {
			return true
		}
	}
	return false
}

// rtlCall is one call site.
type rtlCall struct {
	pkg  string // "os", "unix", … when the callee is <ident>.<Sel> and ident is a known open-family package
	name string // the bare callee name: an Ident, or a selector's Sel
	args []ast.Expr
	pos  token.Pos
}

// rtlCorpus is the parsed tree plus what is derived from it.
type rtlCorpus struct {
	fset  *token.FileSet
	files []arFile
	funcs []*rtlFunc

	// byName is every function of that name — plural on purpose, because
	// callees resolve by name and receivers are ignored (see the head
	// comment's over-approximation note).
	byName map[string][]*rtlFunc

	// producers is the functions whose RETURN value is a trail path:
	// derived from the body, so a renamed helper is still found. Today
	// RefusalsLogPath, CageSpoolPath and spoolCursorPath.
	producers map[string]bool
}

// rtlPathWrappers are the pure path functions a trail path survives: a
// trail expression wrapped in one of these is still a trail expression.
// filepath.Dir is the one that matters — byqv0's unguarded parent create
// was os.MkdirAll(filepath.Dir(cursorPath), 0o755).
var rtlPathWrappers = rtlSet("Dir", "Clean", "Abs", "ToSlash", "FromSlash", "EvalSymlinks", "Base", "Localize")

// rtlJoinFuncs are the path-composing calls a trail literal can hide in.
var rtlJoinFuncs = rtlSet("Join")

// rtlTrailComponent reports whether a string value is a trail path
// COMPONENT — the literal on its own, with or without separators round it,
// and nothing else.
//
// This is the whole of what separates this census from the literal grep the
// head comment refuses, and it is not a detail: MEASURED 2026-11-22, a rule
// that took any string CONTAINING `refusals.log` for a path derived six
// more "producers" out of this tree — usageCatalog, render,
// CommitGuardHook, constitutionGuardBody, dataCeilingCheck,
// dataCeilingReport — every one of them a function whose return value is
// prose or the rendered shim TEXT a seat appends with
// (`… >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null`), and five unguarded
// findings behind them: four a commit-guard render handed to installHook or
// to execOwnRenders, and the fifth cockpit.go's `write(c.render(w, h))`,
// which is the name resolution above compounding the over-match — a second
// `render` entirely, matched because one of that name returns shim text.
// A component is a path; a sentence mentioning one is not.
func rtlTrailComponent(s string) bool {
	trimmed := strings.Trim(s, "/")
	for _, lit := range rtlTrailLiterals {
		if trimmed == lit {
			return true
		}
	}
	return false
}

// rtlSpellsTrail reports whether an expression SPELLS a trail path on its
// own, with no help from a local binding: a Join or a concatenation one of
// whose elements is a trail COMPONENT, a call to a producer, or either of
// those inside a pure path wrapper. This is the half of rtlTrailExpr that
// needs no function context, so it is also what makes a local a trail
// local.
//
// A bare string literal is deliberately NOT a trail path, however exactly
// it names one. A component is where a path is spelled, never the path: the
// Join or the `+` round it is what says a path is being built, and without
// that condition this census is the literal grep (see rtlTrailComponent).
func (c *rtlCorpus) rtlSpellsTrail(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.ParenExpr:
		return c.rtlSpellsTrail(v.X)
	case *ast.BinaryExpr:
		// `gatesDir + "/refusals.log"`, either way round.
		return v.Op == token.ADD && (rtlIsTrailComponent(v.X) || rtlIsTrailComponent(v.Y) ||
			c.rtlSpellsTrail(v.X) || c.rtlSpellsTrail(v.Y))
	case *ast.CallExpr:
		name := rtlCalleeName(v.Fun)
		if name == "" {
			return false
		}
		if c.producers[name] {
			return true
		}
		if rtlJoinFuncs[name] {
			for _, a := range v.Args {
				if rtlIsTrailComponent(a) || c.rtlSpellsTrail(a) {
					return true
				}
			}
			return false
		}
		if rtlPathWrappers[name] && len(v.Args) == 1 {
			return c.rtlSpellsTrail(v.Args[0])
		}
	}
	return false
}

// rtlIsTrailComponent is rtlTrailComponent over an expression: a string
// literal naming one component of the trail.
func rtlIsTrailComponent(e ast.Expr) bool {
	s, ok := arStr(e)
	return ok && rtlTrailComponent(s)
}

// rtlTrailExpr is rtlSpellsTrail plus the local bindings of one function:
// an identifier the function assigned a trail path to, and the pure path
// wrappers and Joins over it.
func (c *rtlCorpus) rtlTrailExpr(f *rtlFunc, e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.ParenExpr:
		return c.rtlTrailExpr(f, v.X)
	case *ast.Ident:
		return f.rtlHoldsTrail(v.Name, v.Pos())
	case *ast.BinaryExpr:
		// `gatesDir + "/refusals.log"` — the spelling `posse gates`
		// actually used, which is why the component arm is here and not
		// only in rtlSpellsTrail.
		return v.Op == token.ADD && (rtlIsTrailComponent(v.X) || rtlIsTrailComponent(v.Y) ||
			c.rtlTrailExpr(f, v.X) || c.rtlTrailExpr(f, v.Y))
	case *ast.CallExpr:
		name := rtlCalleeName(v.Fun)
		if name != "" && (rtlJoinFuncs[name] || (rtlPathWrappers[name] && len(v.Args) == 1)) {
			for _, a := range v.Args {
				if c.rtlTrailExpr(f, a) {
					return true
				}
			}
		}
	}
	return c.rtlSpellsTrail(e)
}

// rtlCalleeName is the bare name a call expression names: an Ident, or the
// Sel of a selector. The receiver or package qualifier is dropped — see the
// head comment on why resolving by name over-approximates in the safe
// direction.
func rtlCalleeName(fun ast.Expr) string {
	switch v := fun.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	case *ast.ParenExpr:
		return rtlCalleeName(v.X)
	case *ast.IndexExpr: // a generic instantiation
		return rtlCalleeName(v.X)
	}
	return ""
}

// rtlOpenPkg is the package qualifier of a call, when the callee is spelled
// `<ident>.<Sel>` and that ident is one of the open-family packages. A
// method call on a value named `os` is not a thing this tree has.
func rtlOpenPkg(fun ast.Expr) string {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	if _, known := rtlOpenFamily[id.Name]; !known {
		return ""
	}
	return id.Name
}

// rtlIsStdOpen reports whether a call is an open-family call of the
// standard library or of golang.org/x/sys/unix.
func rtlIsStdOpen(c rtlCall) bool {
	if c.pkg == "" {
		return false
	}
	return rtlOpenFamily[c.pkg][c.name]
}

// ---------------------------------------------------------------------------
// Building the corpus.
// ---------------------------------------------------------------------------

// rtlScan parses root/arRoots and returns the corpus with the producer set
// derived and the open-reaching parameter sets closed over the call graph.
func rtlScan(t *testing.T, root string) *rtlCorpus {
	t.Helper()
	fset, files := arParse(t, root, arRoots)
	c := &rtlCorpus{fset: fset, files: files, byName: map[string][]*rtlFunc{}, producers: map[string]bool{}}

	// Pass 1 — the producers, derived from what a function RETURNS. A Join
	// naming a trail literal inside a composite literal or an argument is
	// not a producer: cage.go's CageMounts spells
	// `Dst: filepath.Join(CageGatesDir(ag.Name), "refusals.log")` for a
	// mount the host never opens, and reading that as a producer would make
	// every caller of CageMounts a trail caller.
	for _, af := range files {
		for _, d := range af.file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				ret, ok := n.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				for _, r := range ret.Results {
					if c.rtlSpellsTrail(r) {
						c.producers[fd.Name.Name] = true
					}
				}
				return true
			})
		}
	}

	// Pass 2 — one record per function, with the local dataflow run twice:
	// once for "which parameters could this identifier have come from" and
	// once for "does this identifier hold a trail path".
	for _, af := range files {
		for _, d := range af.file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			f := &rtlFunc{
				rel:     af.rel,
				name:    fd.Name.Name,
				paramOf: map[string]map[int]bool{},
				opens:   map[int]bool{},
			}
			if fd.Type.Params != nil {
				for _, fl := range fd.Type.Params.List {
					if len(fl.Names) == 0 {
						f.params = append(f.params, "")
						continue
					}
					for _, nm := range fl.Names {
						f.params = append(f.params, nm.Name)
					}
				}
			}
			for i, p := range f.params {
				if p == "" || p == "_" {
					continue
				}
				if f.paramOf[p] == nil {
					f.paramOf[p] = map[int]bool{}
				}
				f.paramOf[p][i] = true
			}
			rtlLocalFlow(c, f, fd.Body)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				f.calls = append(f.calls, rtlCall{
					pkg:  rtlOpenPkg(call.Fun),
					name: rtlCalleeName(call.Fun),
					args: call.Args,
					pos:  call.Lparen,
				})
				return true
			})
			c.funcs = append(c.funcs, f)
			c.byName[f.name] = append(c.byName[f.name], f)
		}
	}

	// Pass 3 — the closure. A function's parameter reaches an unguarded
	// open if it is an argument of a standard open, or an argument of a
	// call to a function one of whose own open-reaching parameters sits at
	// that position. Iterated to a fixpoint, because the chain can be any
	// depth: jielm's probe pair was one hop, and nothing stops the next one
	// being three.
	for changed := true; changed; {
		changed = false
		for _, f := range c.funcs {
			if rtlGuards[f.name] != "" {
				// A guard is never an opener: that is what makes it the
				// guard. openRefusalLeaf does hold the tree's only
				// unix.Openat of a leaf, and reading it as an opener would
				// red every guarded call site in the tree.
				continue
			}
			for _, call := range f.calls {
				for i, arg := range call.args {
					if !c.rtlOpensAt(call, i) {
						continue
					}
					for p := range rtlIdents(arg) {
						for idx := range f.paramOf[p] {
							if !f.opens[idx] {
								f.opens[idx] = true
								changed = true
							}
						}
					}
				}
			}
		}
	}
	return c
}

// rtlOpensAt reports whether argument i of a call lands on a path an
// UNGUARDED open will resolve. For a standard open that is every argument
// (see the head comment: a mode or a []byte is never a trail expression,
// so the over-approximation costs nothing and saves an arity table). For a
// call to this tree's own code it is the callee's own open-reaching
// parameter set, which is what pass 3 is closing over.
func (c *rtlCorpus) rtlOpensAt(call rtlCall, i int) bool {
	if rtlGuards[call.name] != "" {
		return false
	}
	if rtlIsStdOpen(call) {
		return true
	}
	if call.pkg != "" {
		// A non-open function of an open-family PACKAGE — os.Getenv,
		// unix.Getpid. The name is qualified, so it is not one of this
		// tree's own functions and the closure below must not be asked
		// about it: `byName` would answer for any same-named function the
		// tree happens to declare.
		return false
	}
	for _, callee := range c.byName[call.name] {
		if callee.opens[i] {
			return true
		}
	}
	return false
}

// rtlLocalFlow fills a function's paramOf and trailLocal from its
// assignments, to a fixpoint over the body. Both halves are deliberately
// flow-INSENSITIVE — an identifier that ever held a trail path counts as
// holding one — because a census that reasoned about order would be a
// second, weaker type checker and the direction of its mistakes would be
// the unsafe one. They are SCOPED, though, which is a different question
// and the one that bit: see trailLocal's own comment.
func rtlLocalFlow(c *rtlCorpus, f *rtlFunc, body *ast.BlockStmt) {
	// scope is the source range a binding is live over. The function body
	// for an assignment or a var, the `for` statement for a range variable.
	type binding struct {
		lhs      []ast.Expr
		rhs      []ast.Expr
		from, to token.Pos
	}
	var bindings []binding
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			bindings = append(bindings, binding{lhs: v.Lhs, rhs: v.Rhs, from: body.Pos(), to: body.End()})
		case *ast.ValueSpec:
			var lhs []ast.Expr
			for _, nm := range v.Names {
				lhs = append(lhs, nm)
			}
			bindings = append(bindings, binding{lhs: lhs, rhs: v.Values, from: body.Pos(), to: body.End()})
		case *ast.RangeStmt:
			// `for _, p := range []string{…, "refusals.log"}` — the range
			// variable holds whatever the operand spells, and a LIST of
			// components is the one place a bare literal does make its
			// binding a trail path. seatbelt.go's carve-out is spelled this
			// way: `for _, f := range []string{"refusals.log",
			// "shell.log"} { c.Keep = append(c.Keep, filepath.Join(gd, f)) }`.
			// It is a list, not an access — but the census has to SEE it,
			// or an open added inside that loop tomorrow is invisible.
			//
			// Scoped to this statement, which is what keeps the next `for
			// _, f := range …` in the same function out of it.
			if v.Value != nil {
				bindings = append(bindings, binding{lhs: []ast.Expr{v.Value}, rhs: []ast.Expr{v.X}, from: v.Pos(), to: v.End()})
				if lit, ok := v.X.(*ast.CompositeLit); ok {
					for _, el := range lit.Elts {
						if !rtlIsTrailComponent(el) {
							continue
						}
						if id, ok := v.Value.(*ast.Ident); ok && id.Name != "_" {
							f.trailLocal = append(f.trailLocal, rtlBinding{name: id.Name, from: v.Pos(), to: v.End()})
						}
					}
				}
			}
		}
		return true
	})
	for changed := true; changed; {
		changed = false
		for _, b := range bindings {
			// One-to-one when the arities match; otherwise every left-hand
			// name takes every right-hand expression, which is what a
			// multi-value call returns anyway.
			for li, l := range b.lhs {
				id, ok := l.(*ast.Ident)
				if !ok || id.Name == "_" {
					continue
				}
				rhs := b.rhs
				if len(b.lhs) == len(b.rhs) {
					rhs = b.rhs[li : li+1]
				}
				for _, r := range rhs {
					if !f.rtlHoldsTrail(id.Name, id.Pos()) && c.rtlTrailExpr(f, r) {
						f.trailLocal = append(f.trailLocal, rtlBinding{name: id.Name, from: b.from, to: b.to})
						changed = true
					}
					for p := range rtlIdents(r) {
						for idx := range f.paramOf[p] {
							if f.paramOf[id.Name] == nil {
								f.paramOf[id.Name] = map[int]bool{}
							}
							if !f.paramOf[id.Name][idx] {
								f.paramOf[id.Name][idx] = true
								changed = true
							}
						}
					}
				}
			}
		}
	}
}

// rtlIdents is every identifier named anywhere in an expression. Used for
// taint rather than for resolution, so a selector's field name coming along
// is harmless: a field called `path` is not a parameter.
func rtlIdents(e ast.Expr) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			out[id.Name] = true
		}
		return true
	})
	return out
}

// ---------------------------------------------------------------------------
// The census.
// ---------------------------------------------------------------------------

// rtlHit is one call handed a trail path, whichever of the three answers
// below it got.
type rtlHit struct {
	where  string // file:line
	callee string
	arg    string // the trail expression as source-ish text
}

// rtlCensus sorts every call that is handed a trail path into three:
//
//	unguarded  — the callee opens it and is not a guard. The finding.
//	guarded    — the callee IS a guarded entry. The positive witness: an
//	             assertion that nothing is unguarded is satisfied by a
//	             scanner that found no accesses at all.
//	through    — the callee neither opens it nor is a guard, so it is a
//	             pass-through: a path printed, bound as a mount, baked into
//	             the rendered shim text a SEAT appends with, or handed to a
//	             helper that opens it through a guard (readSpoolCursor,
//	             writeSpoolCursor). Reported, never failed — these are the
//	             non-accesses the literal grep's 92 lines could not be told
//	             apart from an access, and what makes them safe is the arm
//	             above, not a row in a register here.
func (c *rtlCorpus) rtlCensus() (unguarded, guarded, through []rtlHit) {
	for _, f := range c.funcs {
		for _, call := range f.calls {
			if rtlJoinFuncs[call.name] || rtlPathWrappers[call.name] {
				// Pure path arithmetic is how a trail path is SPELLED, not
				// something it is handed to: `filepath.Join(gatesDir,
				// "refusals.log")` would otherwise report itself, once per
				// producer, as a consumer of its own literal.
				continue
			}
			for i, arg := range call.args {
				if !c.rtlTrailExpr(f, arg) {
					continue
				}
				h := rtlHit{
					where:  arLine(c.fset, f.rel, call.pos),
					callee: call.name,
					arg:    rtlExprText(arg),
				}
				switch {
				case rtlGuards[call.name] != "":
					guarded = append(guarded, h)
				case c.rtlOpensAt(call, i):
					unguarded = append(unguarded, h)
				default:
					through = append(through, h)
				}
			}
		}
	}
	// Sorted, so a red names the same call sites in the same order on every
	// box.
	byWhere := func(hits []rtlHit) {
		sort.Slice(hits, func(i, j int) bool { return hits[i].where < hits[j].where })
	}
	byWhere(unguarded)
	byWhere(guarded)
	byWhere(through)
	return unguarded, guarded, through
}

// rtlExprText renders an expression for a finding message. Not a formatter:
// enough of the shape to find the line by eye.
func rtlExprText(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.BasicLit:
		return v.Value
	case *ast.SelectorExpr:
		return rtlExprText(v.X) + "." + v.Sel.Name
	case *ast.ParenExpr:
		return "(" + rtlExprText(v.X) + ")"
	case *ast.BinaryExpr:
		return rtlExprText(v.X) + " " + v.Op.String() + " " + rtlExprText(v.Y)
	case *ast.CallExpr:
		var args []string
		for _, a := range v.Args {
			args = append(args, rtlExprText(a))
		}
		return rtlExprText(v.Fun) + "(" + strings.Join(args, ", ") + ")"
	}
	return "…"
}

// rtlNames is a sorted list of a name set, so a red reads the same on every
// box.
func rtlNames(m map[string]bool) []string {
	var out []string
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func TestQAEveryHostSideRefusalTrailAccessGoesThroughTheGuard(t *testing.T) {
	t.Parallel()
	c := rtlScan(t, ".")

	// The walk read something. An assertion of absence over zero files is
	// the one failure this whole class refuses.
	if len(c.files) < arFileFloor {
		t.Fatalf("parsed %d non-test .go files under %v, floor %d — the corpus this pin censuses is not there", len(c.files), arRoots, arFileFloor)
	}

	// The guard exists, by every name this file knows it by. A guard
	// renamed out from under rtlGuards would make its call sites read as
	// unguarded accesses, which is loud — but a guard renamed while the
	// openers moved with it would not be, and this is what says so.
	declared := map[string]bool{}
	for _, f := range c.funcs {
		declared[f.name] = true
	}
	for _, name := range rtlNames(rtlGuardNames(rtlGuards)) {
		if !declared[name] {
			t.Errorf("rtlGuards names %s (%s) and no function of that name is declared under %v — either the guard was renamed, in which case rename it here too, or it is gone, in which case every access this pin calls guarded is now going through something nothing here has read (internal/posse/refusalleaf.go)", name, rtlGuards[name], arRoots)
		}
	}

	// The producers are derived, not spelled, so that a renamed path helper
	// is still found — and a derivation is satisfied by deriving nothing,
	// so it carries a floor. Three on 2026-11-22: RefusalsLogPath (the
	// canonical leaf), CageSpoolPath (a session's spool) and
	// spoolCursorPath (the fold's cursor), which is one per trail path.
	if len(c.producers) < 3 {
		t.Errorf("only %d function(s) under %v return a path naming %v (3 on 2026-11-22: RefusalsLogPath, CageSpoolPath, spoolCursorPath) — so either the producer rule has stopped matching, and every call site that reaches the trail through a helper is now invisible to this census, or a path really did leave the trail, in which case re-state this floor and rtlTrailLiterals over what the trail is now: %v",
			len(c.producers), arRoots, rtlTrailLiterals, rtlNames(c.producers))
	}

	unguarded, guarded, through := c.rtlCensus()

	// The positive witness, and the half that catches a scanner which
	// stopped seeing the trail. The floor is loose on purpose: its job is
	// to fail a census that reads nothing, not to track the count, which an
	// ordinary landing may move either way.
	if len(guarded) < 4 {
		t.Errorf("the census found %d call(s) handing a trail path to a guarded entry (6 on 2026-11-22) — a green below would then say only that this scanner cannot see the refusals trail at all: %v", len(guarded), guarded)
	}
	t.Logf("%d non-test .go files, %d trail-path producers %v, %d guarded accesses, %d pass-throughs", len(c.files), len(c.producers), rtlNames(c.producers), len(guarded), len(through))
	for _, h := range guarded {
		t.Logf("  guarded: %s %s(%s)", h.where, h.callee, h.arg)
	}
	for _, h := range through {
		t.Logf("  through: %s %s(%s)", h.where, h.callee, h.arg)
	}

	for _, h := range unguarded {
		t.Errorf("%s hands a refusals-trail path to %s, which opens it: %s(%s)\n"+
			"  The host's own accesses to gates/<p>/refusals.log, state/cages/<p>/<s>/refusals.log and gates/<p>/spool-cursors/<s> run UNCAGED over a name a planter outside every wall may have replaced. Every one of them goes through openRefusalLeaf, readRefusalLeaf, refusalLeafSize or refusalDeltaAfter (internal/posse/refusalleaf.go): O_NOFOLLOW on the leaf, each parent below StateDir opened with openat(O_DIRECTORY|O_NOFOLLOW), and the judgement made on the fd that was opened rather than on an earlier lstat.\n"+
			"  A read must FAIL CLOSED, not fall back: refusalLeafSize answers 0 and refusalDeltaAfter answers \"\" on a refusal, so a reader of the probe's observable 2 is told no refusal reached the log rather than being handed a planter's bytes (ranger-base-jielm).\n"+
			"  If what you need is the path and not its bytes, don't pass it to an opener — RefusalsLogPath and CageSpoolPath are string helpers and printing one, or binding it as a mount, is not an access (ranger-base-u5ez8).",
			h.where, h.callee, h.callee, h.arg)
	}
}

// rtlGuardNames is the key set of a reason map.
func rtlGuardNames(m map[string]string) map[string]bool {
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// ---------------------------------------------------------------------------
// The rig, shown able to fail.
// ---------------------------------------------------------------------------

// rtlCleanFixture is a miniature of the live tree: the three path
// producers, the guard, and one guarded access of each kind — a create, a
// read through a helper that takes the path as a PARAMETER, and the probe's
// delta pair. Nothing here is a finding, and the rig must say so, or the
// planted arms below would prove only that the scanner shouts at
// everything.
var rtlCleanFixture = map[string]string{
	"internal/posse/refusalleaf.go": `package posse

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func openRefusalLeaf(root, path string, flag int, create bool) (*os.File, error) {
	fd, err := unix.Open(path, flag|unix.O_NOFOLLOW, 0o644)
	if err != nil {
		return nil, err
	}
	_ = root
	_ = create
	return os.NewFile(uintptr(fd), path), nil
}

func readRefusalLeaf(root, path string) ([]byte, error) {
	f, err := openRefusalLeaf(root, path, os.O_RDONLY, false)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func refusalLeafSize(root, path string) int64 {
	f, err := openRefusalLeaf(root, path, os.O_RDONLY, false)
	if err != nil {
		return 0
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0
	}
	return st.Size()
}

func refusalDeltaAfter(root, path string, off int64) string {
	b, err := readRefusalLeaf(root, path)
	if err != nil {
		return ""
	}
	if off < 0 || off > int64(len(b)) {
		return string(b)
	}
	return string(b[off:])
}
`,
	"internal/posse/trail.go": `package posse

import (
	"os"
	"path/filepath"
)

type App struct{ StateDir string }

func (a *App) GatesDir(persona string) string { return filepath.Join(a.StateDir, "gates", persona) }
func (a *App) CageDir(persona string) string  { return filepath.Join(a.StateDir, "cages", persona) }

func (a *App) RefusalsLogPath(persona string) string {
	return filepath.Join(a.GatesDir(persona), "refusals.log")
}

func (a *App) CageSpoolPath(persona, session string) string {
	return filepath.Join(a.CageDir(persona), session, "refusals.log")
}

func (a *App) spoolCursorPath(persona, session string) string {
	return filepath.Join(a.GatesDir(persona), "spool-cursors", session)
}

func (a *App) EnsureCageSpool(persona, session string) (string, error) {
	p := a.CageSpoolPath(persona, session)
	f, err := openRefusalLeaf(a.StateDir, p, os.O_APPEND|os.O_WRONLY, true)
	if err != nil {
		return "", err
	}
	return p, f.Close()
}

func readSpoolCursor(root, path string) ([]byte, error) {
	return readRefusalLeaf(root, path)
}

func writeSpoolCursor(root, path string, b []byte) error {
	f, err := openRefusalLeaf(root, path, os.O_WRONLY, true)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (a *App) Fold(persona, session string) error {
	cursorPath := a.spoolCursorPath(persona, session)
	if _, err := readSpoolCursor(a.StateDir, cursorPath); err != nil {
		return err
	}
	return writeSpoolCursor(a.StateDir, cursorPath, nil)
}
`,
	"internal/posse/probe.go": `package posse

import "path/filepath"

func (a *App) Probe(gatesDir string) string {
	logPath := filepath.Join(gatesDir, "refusals.log")
	at := refusalLeafSize(a.StateDir, logPath)
	return refusalDeltaAfter(a.StateDir, logPath, at)
}
`,
	"cmd/posse/main.go": `package main

import (
	"fmt"

	"github.com/ranger360ai/posse/internal/posse"
)

func main() {
	a := &posse.App{}
	fmt.Println(posse.AbbrevHome(a.RefusalsLogPath("p")))
}
`,
	"internal/posse/print.go": `package posse

import "strings"

// A path PRINTED is not a path opened: the one shape the census must leave
// alone, and the reason it asks the question from the open side.
func AbbrevHome(p string) string { return strings.TrimPrefix(p, "/home/") }
`,
}

func TestQARefusalTrailCensusCatchesEachEscape(t *testing.T) {
	t.Parallel()

	// Control first: the rig comes back clean over a tree that reaches the
	// trail at five guarded call sites and prints the path at a sixth.
	root := arFixture(t, rtlCleanFixture)
	c := rtlScan(t, root)
	if len(c.files) != len(rtlCleanFixture) {
		t.Fatalf("the clean fixture parsed %d files, want %d — the rig did not read what it wrote", len(c.files), len(rtlCleanFixture))
	}
	if len(c.producers) != 3 {
		t.Fatalf("the clean fixture derived %v as trail-path producers, want the three helpers — the producer rule is not reading this rig, so every arm below would measure the wrong thing", rtlNames(c.producers))
	}
	unguarded, guarded, through := c.rtlCensus()
	if len(unguarded) != 0 {
		t.Fatalf("the clean arm reports %v unguarded. Every arm below would then be unfalsifiable.", unguarded)
	}
	// Three guarded entries (EnsureCageSpool's create, the probe's size and
	// its delta) and three pass-throughs (readSpoolCursor, writeSpoolCursor
	// and the printed path in cmd/), which is the live tree's own shape in
	// miniature.
	if len(guarded) != 3 || len(through) != 3 {
		t.Fatalf("the clean arm found %d guarded and %d pass-through accesses, want 3 and 3 — the scanner is not reading this rig the way it reads the tree, so every arm below measures the wrong thing: guarded=%v through=%v", len(guarded), len(through), guarded, through)
	}

	// One plant per escape. The first three are the three this class has
	// actually escaped by, re-introduced as they were written; the last two
	// are the shapes those three say nothing about on their own.
	for _, tc := range []struct {
		name, path, body string
		wantCallee       string
		why              string
	}{
		{
			name: "ranger-base-legou: the spool create on a plain os.OpenFile",
			path: "internal/posse/trail.go",
			body: strings.Replace(rtlCleanFixture["internal/posse/trail.go"],
				"f, err := openRefusalLeaf(a.StateDir, p, os.O_APPEND|os.O_WRONLY, true)",
				"f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)", 1),
			wantCallee: "OpenFile",
			why:        "the path is spelled one line up by a producer call, in the same function as the open",
		},
		{
			name: "ranger-base-jielm: `posse gates` reading the leaf by concatenation",
			path: "cmd/posse/main.go",
			body: `package main

import (
	"fmt"
	"os"
)

func main() {
	gatesDir := os.Getenv("RHQ_GATES_DIR")
	b, _ := os.ReadFile(gatesDir + "/refusals.log")
	fmt.Println(string(b))
}
`,
			wantCallee: "ReadFile",
			why:        "the path is a `+` and not a Join, which is how the one call site that bead named was actually spelled",
		},
		{
			name: "ranger-base-byqv0: the cursor write, path through a parameter",
			path: "internal/posse/trail.go",
			body: strings.Replace(rtlCleanFixture["internal/posse/trail.go"],
				`func writeSpoolCursor(root, path string, b []byte) error {
	f, err := openRefusalLeaf(root, path, os.O_WRONLY, true)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}`,
				`func writeSpoolCursor(root, path string, b []byte) error {
	return os.WriteFile(path, b, 0o644)
}`, 1),
			wantCallee: "writeSpoolCursor",
			why:        "the open is in a function that spells no trail literal; the trail path is at the call site one frame up",
		},
		{
			name: "ranger-base-byqv0: the unguarded parent create",
			path: "internal/posse/trail.go",
			body: strings.Replace(rtlCleanFixture["internal/posse/trail.go"],
				"	cursorPath := a.spoolCursorPath(persona, session)",
				`	cursorPath := a.spoolCursorPath(persona, session)
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil {
		return err
	}`, 1),
			wantCallee: "MkdirAll",
			why:        "O_NOFOLLOW guards the last component only, so a MkdirAll that walks to a planted directory aims whatever opens the leaf next",
		},
		{
			name: "ranger-base-jielm: the probe's delta pair, path through a parameter",
			path: "internal/posse/probe.go",
			body: `package posse

import (
	"os"
	"path/filepath"
)

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func (a *App) Probe(gatesDir string) string {
	logPath := filepath.Join(gatesDir, "refusals.log")
	_ = fileSize(logPath)
	return ""
}
`,
			wantCallee: "fileSize",
			why:        "the trail path is an inline Join of a gatesDir the function already had — the shape keying on the path HELPERS cannot see",
		},
		{
			name: "two hops to the open",
			path: "internal/posse/probe.go",
			body: `package posse

import (
	"os"
	"path/filepath"
)

func slurp(p string) []byte {
	b, _ := os.ReadFile(p)
	return b
}

func tail(p string) []byte { return slurp(p) }

func (a *App) Probe(gatesDir string) string {
	return string(tail(filepath.Join(gatesDir, "refusals.log")))
}
`,
			wantCallee: "tail",
			why:        "the closure is a fixpoint, not one hop — nothing stops the next escape being three frames deep",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := map[string]string{}
			for k, v := range rtlCleanFixture {
				f[k] = v
			}
			if f[tc.path] == tc.body {
				t.Fatalf("the plant for %s did not change %s — the strings.Replace matched nothing, so this arm is re-running the clean control and calling it a catch", tc.name, tc.path)
			}
			f[tc.path] = tc.body
			root := arFixture(t, f)
			c := rtlScan(t, root)
			unguarded, _, _ := c.rtlCensus()
			found := false
			var got []string
			for _, h := range unguarded {
				got = append(got, h.where+" "+h.callee+"("+h.arg+")")
				if h.callee == tc.wantCallee {
					found = true
				}
			}
			if !found {
				t.Errorf("planted %s in %s and the census reported %v — it did not name %s as the opener, so the live green measures nothing for that shape (%s)", tc.name, tc.path, got, tc.wantCallee, tc.why)
			}
		})
	}
}
