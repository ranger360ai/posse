package treepins

// QA pins for ranger-base-zb476 — a pin that asserts on a LAUNCH LINE reads
// both places the line lands in, which is `launchLog` and never `calls`.
//
// THE CLASS. A launch line over PaneLineMax (1023 bytes, the tty's MAX_CANON
// — internal/posse/paneline.go) is not typed into the pane at all: it is
// spilled to state/launch/<session>.sh and the pane types a dot-source of
// that script instead. internal/posse's fake herdr logs the argv it was
// called with, so a pin reading that log through `calls` sees the
// `. '<script>'` line and not the line it means to assert on. `launchLog`
// (internal/posse/herdr_test.go) reads BOTH places and exists precisely for
// this.
//
// WHY IT IS A LIVE BUG AND NOT HOUSEKEEPING. Whether a given pin's line is
// over the cliff is a function of its fixture PATH LENGTHS, so the verdict
// moves with TMPDIR, with the checkout path, and with any unrelated landing
// that adds bytes to a rendered line. MEASURED 2026-11-13 on this box
// (ranger-base-6ca29), internal/posse/parity_test.go's grok line: 959 bytes
// with TMPDIR unset — typed, 64 bytes of headroom — against 1325 bytes with
// TMPDIR set to a directory under the home, spilled, and the pin red with
// `bwrap wrap missing` over a launch that was correctly walled. 64 bytes.
// ranger-base-rq83c walked SIX fixtures over the same cliff when ~110 bytes
// of an unrelated credential-dir pin were added; ranger-base-90y3c is a
// third landing, where the pin died on `no such file or directory` while the
// launch it asked about was correct.
//
// And the direction that is worse than a red: in production spilling is the
// NORMAL case — paneline.go records every persona's line at 1867-2058 bytes
// against the 1023 limit, all seven spilled — so a fixture short enough to
// be typed is measuring the EXCEPTIONAL path, and a pin whose assertion is
// satisfied by the typed form may be GREEN over a spilled line it never
// read. Three beads fixed the site their own measurement reddened and left
// the class; this is the census that was supposed to end that.
//
// ─── what is in scope ───────────────────────────────────────────────────────
//
// internal/posse has 395 calls of `calls` and almost none of them are about a
// launch line (MEASURED 2026-11-14). The log holds one herdr argv per line,
// and nearly every pin reading it is asking about herdr's OWN argv —
// `workspace create`, `agent prompt`, `pane send-text`, `agent wait --until
// working`. None of that can spill: PaneLine is reached by the persona
// COMMAND alone (herdrback.go, runtimeprobe.go), so every other herdr call is
// in calls.log verbatim whatever its length, and `calls` is the right reader
// for all of them.
//
// `--env RHQ_TIER=fast` is the one that looks like a launch-line assertion
// and is not. It is herdr's own `workspace create --env K=V`
// (Herdr.CreateWorkspace, internal/posse/herdr.go) — eleven assertions name
// one, and every one of them is reading the create line.
//
// So scope is decided by the ASSERTED TEXT, and a launch-line marker is what
// decides it. The DERIVED half (llRuntimeMarkers) is read out of
// internal/posse/runtime.go and is the larger one: every built-in runtime's
// CLI name, every `-`flag in its resolved `Command` template and
// `Unattended` flags, its `ModelFlag` printf head, and every `-`flag literal
// in the bodies of the `Realize` and `Skills` functions it names. A runtime
// added to the built-ins, or a flag added to a template, widens this census
// the same day — which is the half a hand-typed list would have gone stale
// on first.
//
// The LISTED half is four markers no template spells, each one a thing posse
// puts on the line from somewhere else; llListedMarkers carries the renderer
// for each, and the pin refuses a row whose reason is too short to check.
//
// WHAT IS LOOSE, said here rather than half-handled. A launch-line assertion
// whose text carries NO marker — a bare model id, a quoted fixture path — is
// out of scope and this reader will not find it. There is no closed
// vocabulary available in the other direction either: posse renders herdr's
// argv and the launch line from the same package, so "a flag literal in
// posse's sources" separates nothing, and a default-IN rule would have to
// exempt pane ids, line counts and `--cmd` text one at a time. What covers
// this direction is the SITES FLOOR at the foot of the census — exact, not a
// margin — so a site leaving the census reds the door whatever took it out,
// including a spelling this reader never learns to read.
//
// ─── four readers, one per question ─────────────────────────────────────────
//
// ranger-base-6ca29's scope note left open whether the tree should have ONE
// reader of the fake herdr's log. It should not: there are four questions,
// and they are all in internal/posse/herdr_test.go now — the SHARED file, so
// every build arm can ask any of them. That file's own comment is the table;
// what matters here is which two this census is about.
//
// `calls` and `launchLog` are the two it reads. Everything else is a
// different reader and therefore out of scope by construction, which is the
// right answer and not an omission: `paneRunLines` is deliberately RAW,
// because the subject it was written for is how long the line is, and
// `launchLines` is already spill-aware — it resolves a `. '<script>'` to the
// script's body and collapses the gate prefix, one entry per `pane run`.
//
// That last one is why the hardest site in the class could be fixed rather
// than exempted. `TestPersonaLaunchRuntime` counts the launches that carried
// codex's line, and `launchLog` cannot answer a COUNT over one session: the
// spilled script is rewritten at every launch, so a launch and a relaunch
// leave ONE script and the count reads 1 where a count over typed lines reads
// 2. `paneRunLines` and `resolvedTypedLine` were in paneline_test.go and
// codexwritable_test.go, both arm 1 only, which is why ranger-base-6ca29
// added a third WHOLE-LOG reader rather than reach for them from a shared
// file; they were moved rather than copied, so there is still one reader per
// question and a relaunch pin in any arm can count.
//
// MEASURED 2026-11-14 (linux, this box), over internal/posse as it stands
// after this bead's fixes: 885 .go files read, 23 built-in-runtime markers
// derived over 4 runtimes (bob, claude, codex, grok) beside 4 listed, and 21
// in-scope launch-line assertions — 20 through `launchLog` and 1 on
// llExempt. 0.37s, which is the whole cost of parsing those 885 files.
//
// BEFORE the fixes the same census read 22, of which 7 went through `calls`:
// six sites and the one exemption. The six, each red with its site, its
// asserting expression and the markers that scored it:
//
//	herdr_test.go            TestPersonaLaunchRuntime
//	planguardpark_test.go    TestTripChoosesNoProviderForTheBead
//	skills_test.go           TestSkillsParity
//	skills_test.go           TestSkillsOnCodexAcceptance
//	unattended_test.go       TestNoPersonaLaunchPathsTypeTheMode
//	verify_nx85_qa_test.go   TestQANoPersonaRelaunchTypesTheModeExactlyOnce
//
// Four of the six were ABSENCES — `--plugin-dir` must not be on a plain or
// codex line, `GATES grok ` must not be anywhere after a parked bead — and
// an absence is the direction where a calls()-only reader is worst: a line
// it cannot see reads as a line that was never typed, so the pin passes over
// the launch it was written to forbid.
//
// The count goes 22 → 21 rather than staying put because
// TestPersonaLaunchRuntime was the one function asserting through BOTH
// readers, and a site is one (file, function, reader).
//
// AND THE SPILL WAS MEASURED, not argued (2026-11-14, linux, this box). With
// TMPDIR at a 138-byte path under /tmp, and with
// TestPersonaLaunchRuntime's grok recipe assertion put back to `calls` and
// nothing else changed, that test reds `recipe runtime: not applied` under
// the long TMPDIR and passes under the default — the same pair
// ranger-base-6ca29 measured at 959 against 1325 bytes, over a different
// fixture. Through `launchLog` it is green under both — which is the whole
// claim, stated over the one fixture it was measured on. The rest of the set
// — planguardpark_test.go, skills_test.go, unattended_test.go,
// verify_nx85_qa_test.go, codexwritable_test.go, paneline_test.go and
// dispatch_qa_test.go, across all three build arms — is green under both
// TMPDIRs after the fixes, which says the fixes break nothing at either
// length and does NOT say that each of those lines crosses the cliff. Which
// of them does is a fact about their fixture paths, and the point of the rule
// is that nobody has to know.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The reader that sees only what was typed, and the one that sees both
// places the line lands in. Both live in internal/posse/herdr_test.go.
const (
	llRawReader  = "calls"
	llSafeReader = "launchLog"
)

// llPaneLineMax is internal/posse's PaneLineMax, quoted for the diagnostic
// alone — nothing in this census branches on it. It is READ out of
// paneline.go rather than typed, because a limit this pin's whole failure
// text rests on is exactly the number that must not go stale in two places;
// a tree where it cannot be read gets the constant's name instead of a wrong
// number. A function and not a package var: this package's TestMain chdirs
// the binary to the repo root, and a var initializer runs before it.
func llPaneLineMax() string {
	b, err := os.ReadFile(filepath.Join("internal", "posse", "paneline.go"))
	if err != nil {
		return "PaneLineMax"
	}
	for _, line := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(line, "const PaneLineMax = "); ok {
			return strings.TrimSpace(rest) + " bytes"
		}
	}
	return "PaneLineMax"
}

// llMinFlag is the shortest derived `-`flag this census will take as a
// marker. `-a`, `-c` and `-m ` are codex's and grok's one-letter flags, and
// a marker that short matches text that has nothing to do with a launch —
// so they are dropped and the lines that carry them are reached by their
// runtime's name and by GATES instead. Four bytes is `--xx`, the shortest
// long flag, and `-s read-only` is twelve.
const llMinFlag = 4

// llMarker is one launch-line marker and where posse puts it on the line.
// The `why` is load-bearing for the LISTED rows — an inclusion vocabulary is
// only as good as the argument for each entry — and derived rows carry the
// field of runtime.go they were read out of.
type llMarker struct {
	Text string
	Why  string
}

// llListedMarkers — the markers no runtime template or Realize body spells,
// because posse renders them around the runtime command rather than inside
// it. Four, and that is the whole hand-kept half of the vocabulary.
var llListedMarkers = []llMarker{
	{
		"GATES ",
		"the marker `calls` and `launchLog` collapse the gate prefix to (internal/posse/herdr_test.go). GatePrefix is typed in front of a persona command and in front of nothing else (gates.go, WrapWithGates), so an assertion naming GATES is naming a launch line by construction — it is the single most common spelling in the class.",
	},
	{
		"bwrap",
		"the linux wall's wrapper binary. BwrapPrefix (bwrap.go) wraps the whole persona command, so the wrapper and the command spill together — ranger-base-6ca29's own red was `bwrap wrap missing` over a launch that was correctly walled.",
	},
	{
		"sandbox-exec",
		"the darwin wall's wrapper binary, SeatbeltPrefix (seatbelt.go). The same claim as bwrap one mechanism over, and a pin that names the mechanism by hand rather than through WallMechanism() names this.",
	},
	{
		"--settings",
		"what the `{settings}` placeholder renders to, which is a *Runtime METHOD and not a field the derivation can follow: FleetSettingsText composes `--settings ` + the quoted payload, and EnsureSettingsPin guarantees it is there (runtime.go). MEASURED 2026-11-14: sweeping every `-`flag literal in the methods on *Runtime would derive this one and six more worth having — and also `--cmd`, `--flag` and `--flag=value`, which are posse's own CLI flag and a plain session's command, and internal/posse/relaunch_test.go asserts `run-me --flag` off calls.log for a `--cmd` session that has no launch line at all. So the sweep is rejected and the one flag it was wanted for is named here.",
	},
}

// ─── the derived half ────────────────────────────────────────────────────────

// llLit unquotes a string literal, and nothing else.
func llLit(e ast.Expr) (string, bool) {
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

// llStrings resolves string-literal names across one package: every `const`
// or `var` spec and every `:=`/`=` assignment whose right-hand side is a
// single string literal, package level and function level alike. A name the
// package binds to two different literals resolves to NOTHING — the reader
// does not guess which binding a call site meant — which is the same bargain
// temproot_qa_test.go's trStrings strikes next door.
//
// A flat map across the package's files is what lets a test file's assertion
// resolve a const declared in a non-test one: `ClaudeFleetFlags` is in
// runtime.go and named in four test files.
//
// AND IT RESOLVES TO A FIXPOINT, not in one pass, because this package's
// most load-bearing flag constant is a chain of other constants:
//
//	const CodexFleetFlags = CodexDisableHooks + ` -c ` + codexConfLoginShell + …
//
// A single pass binds only the names whose right-hand side IS a literal, so
// CodexFleetFlags resolved to nothing, codex's whole `Command` template
// resolved to nothing with it, and the derivation read NO flag at all off the
// one runtime whose template is three constants deep (MEASURED 2026-11-14:
// `--disable` missing from the vocabulary). Each pass re-resolves every
// binding against what the last one knew, so a chain N deep is bound after N
// passes; llStringPasses is the bound. The one-distinct-value rule is
// re-applied on every pass and never relaxed — a name two bindings disagree
// about resolves to nothing however many passes it is given.
func llStrings(files map[string]*ast.File) map[string]string {
	type binding struct {
		name string
		rhs  ast.Expr
	}
	var bs []binding
	for _, p := range cxaSorted(files) {
		ast.Inspect(files[p], func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.ValueSpec:
				for i, name := range v.Names {
					if i < len(v.Values) && name.Name != "" && name.Name != "_" {
						bs = append(bs, binding{name.Name, v.Values[i]})
					}
				}
			case *ast.AssignStmt:
				if len(v.Lhs) == len(v.Rhs) {
					for i := range v.Lhs {
						if id, ok := v.Lhs[i].(*ast.Ident); ok && id.Name != "_" {
							bs = append(bs, binding{id.Name, v.Rhs[i]})
						}
					}
				}
			}
			return true
		})
	}
	env := map[string]string{}
	for pass := 0; pass < llStringPasses; pass++ {
		all := map[string][]string{}
		for _, b := range bs {
			s, ok := llStr(b.rhs, env)
			if !ok {
				continue
			}
			dup := false
			for _, prev := range all[b.name] {
				if prev == s {
					dup = true
					break
				}
			}
			if !dup {
				all[b.name] = append(all[b.name], s)
			}
		}
		next := map[string]string{}
		for name, lits := range all {
			if len(lits) == 1 {
				next[name] = lits[0]
			}
		}
		if llSameEnv(next, env) {
			break
		}
		env = next
	}
	return env
}

// llStringPasses bounds llStrings' fixpoint. Four is well past this package's
// deepest live chain (CodexFleetFlags, two deep) and the loop stops early
// when a pass learns nothing, so the bound only ever caps a pathological
// tree.
const llStringPasses = 4

// llSameEnv reports whether two resolutions agree, which is how the fixpoint
// knows it has converged.
func llSameEnv(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// llStr resolves an expression to the whole string it names: a literal, a
// name llStrings bound, or a `+` chain of those.
func llStr(e ast.Expr, env map[string]string) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		return llLit(v)
	case *ast.Ident:
		s, ok := env[v.Name]
		return s, ok
	case *ast.ParenExpr:
		return llStr(v.X, env)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		l, lok := llStr(v.X, env)
		r, rok := llStr(v.Y, env)
		if !lok || !rok {
			return "", false
		}
		return l + r, true
	}
	return "", false
}

// llFuncDecls indexes the package's top-level functions by name, so a
// `Realize:` or `Skills:` field that names one can be followed to its body.
func llFuncDecls(files map[string]*ast.File) map[string]*ast.FuncDecl {
	out := map[string]*ast.FuncDecl{}
	for _, p := range cxaSorted(files) {
		for _, d := range files[p].Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil {
				out[fn.Name.Name] = fn
			}
		}
	}
	return out
}

// llFlagMarker turns one rendered fragment into a flag marker: trailing
// whitespace off, a leading hyphen required, llMinFlag bytes required.
//
// THE TRIM IS LOAD-BEARING. A flag literal in a Realize body carries the
// space that separates it from its value — `"--allowedTools "`,
// `"--plugin-dir "` — and so does a ModelFlag head (`--model %s` cuts to
// `--model `). The assertions do not: internal/posse/skills_test.go names
// `"--plugin-dir"` with nothing after it, and a marker keeping its space
// matches that nowhere. So the marker is the flag and the SPACE IS NOT PART
// OF IT. A runtime NAME is the opposite case and keeps its space on purpose
// — see llRuntimeMarkers.
func llFlagMarker(s string) (string, bool) {
	s = strings.TrimRight(s, " \t")
	if !strings.HasPrefix(s, "-") || len(s) < llMinFlag {
		return "", false
	}
	return s, true
}

// llFlagTokens returns the `-`flag markers in a rendered command fragment.
// `--add-dir {memory}` yields `--add-dir` and not `{memory}`.
func llFlagTokens(s string) []string {
	var out []string
	for _, tok := range strings.Fields(s) {
		if m, ok := llFlagMarker(tok); ok {
			out = append(out, m)
		}
	}
	return out
}

// llRuntimeMarkers reads the launch-line vocabulary out of
// internal/posse/runtime.go's `builtinRuntimes` composite literal, rather
// than carrying a list of it. It returns the runtime names it read beside
// the markers, so the pin can floor both.
//
// Five fields per runtime, and each is on the line for a different reason:
//
//   - Name — the CLI binary, which is the first word of every persona
//     command. Taken WITH its trailing space, so `--env RHQ_RUNTIME=claude`
//     (herdr's create line) is not read as one.
//   - Command — the template the line is rendered from. Its `-`flags, and
//     the constants it concatenates: ClaudeFleetFlags, ClaudeStrictMCPFlag,
//     CodexFleetFlags, GrokFleetFlags and GrokNoLeaderFlag all resolve
//     through the package's const env, so their flags are in the vocabulary
//     without being named here.
//   - Unattended — the flags a dispatched seat adds, asserted by name in
//     four files.
//   - ModelFlag — the printf form for `{model}`, with `%s` and anything
//     after it cut: `--model %s` yields `--model `.
//   - Realize and Skills — the two functions that turn the PID's rule lists
//     and skill bindings into this CLI's flags. Those flags are in neither
//     template: `-s read-only`, `--allowedTools `, `--plugin-dir ` and
//     three more are literals inside those bodies, and three of the five
//     sites this bead fixed were found by one of them.
func llRuntimeMarkers(files map[string]*ast.File, env map[string]string) (names []string, markers []llMarker) {
	fns := llFuncDecls(files)
	seen := map[string]bool{}
	add := func(text, why string) {
		if text == "" || seen[text] {
			return
		}
		seen[text] = true
		markers = append(markers, llMarker{text, why})
	}
	for _, p := range cxaSorted(files) {
		ast.Inspect(files[p], func(n ast.Node) bool {
			vs, ok := n.(*ast.ValueSpec)
			if !ok || !llNames(vs, "builtinRuntimes") {
				return true
			}
			for _, v := range vs.Values {
				for _, el := range llCompositeElems(v) {
					name, _ := llStr(llField(el, "Name"), env)
					if name == "" {
						continue
					}
					names = append(names, name)
					add(name+" ", "the `"+name+"` CLI binary, the first word of that runtime's Command template (builtinRuntimes, runtime.go)")
					for _, field := range []string{"Command", "Unattended"} {
						if s, ok := llStr(llField(el, field), env); ok {
							for _, tok := range llFlagTokens(s) {
								add(tok, "a flag in "+name+"'s `"+field+"` (builtinRuntimes, runtime.go)")
							}
						}
					}
					if s, ok := llStr(llField(el, "ModelFlag"), env); ok {
						if head, _, found := strings.Cut(s, "%s"); found {
							if m, ok := llFlagMarker(head); ok {
								add(m, "the head of "+name+"'s `ModelFlag` printf form, which is how {model} renders (builtinRuntimes, runtime.go)")
							}
						}
					}
					for _, field := range []string{"Realize", "Skills"} {
						id, ok := llField(el, field).(*ast.Ident)
						if !ok {
							continue
						}
						fn, ok := fns[id.Name]
						if !ok {
							continue
						}
						ast.Inspect(fn.Body, func(m ast.Node) bool {
							e, ok := m.(ast.Expr)
							if !ok {
								return true
							}
							if s, ok := llLit(e); ok {
								if m, ok := llFlagMarker(s); ok {
									add(m, "a flag literal in `"+id.Name+"`, which is "+name+"'s `"+field+"` (runtime.go)")
								}
							}
							return true
						})
					}
				}
			}
			return true
		})
	}
	sort.Strings(names)
	return names, markers
}

// llNames reports whether a ValueSpec declares the given name.
func llNames(vs *ast.ValueSpec, want string) bool {
	for _, id := range vs.Names {
		if id.Name == want {
			return true
		}
	}
	return false
}

// llCompositeElems returns the elements of a composite literal, through a
// `[]Runtime{…}` the way builtinRuntimes spells it.
func llCompositeElems(e ast.Expr) []ast.Expr {
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	return cl.Elts
}

// llField returns a composite-literal element's named field, or nil.
func llField(el ast.Expr, name string) ast.Expr {
	cl, ok := el.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	for _, kv := range cl.Elts {
		pair, ok := kv.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if k, ok := pair.Key.(*ast.Ident); ok && k.Name == name {
			return pair.Value
		}
	}
	return nil
}

// ─── the census ──────────────────────────────────────────────────────────────

// llSite is one launch-line assertion the census found: the reader it goes
// through, the subject it reads into, and the markers that put it in scope.
type llSite struct {
	File    string // repo-relative, slash-separated
	Line    int    // the reader call
	Func    string // the enclosing top-level function — the exemption key
	Reader  string // llRawReader or llSafeReader
	Markers []string
	Code    string // the asserting expression, one line, for the diagnostic
}

// At returns the `path:line` a diagnostic leads with.
func (s llSite) At() string { return fmt.Sprintf("%s:%d", s.File, s.Line) }

// llExempt — the live launch-line assertions that read `calls` on purpose,
// with the reason. Keyed on (file, function) rather than on a line number: a
// line number is stale on the next edit above it, and the function is the
// thing the row is about.
//
// An exemption is a claim that the spill cannot reach this assertion, and
// the two rows here make it for two different reasons — which is the
// argument for a roster with prose in it rather than a wider marker rule.
var llExempt = []struct{ File, Func, Reason string }{
	{
		"internal/posse/bwraplaunchline_qa_test.go", "TestQABothMultiplexersTypeTheWallLineVerbatim",
		"nothing can spill here: this pin calls HerdrMux.Start directly with a line it rendered itself, and PaneLine is never on that path — Start types what it is handed (muxherdr.go), and the spill decision is made one layer up in planLaunch and RelaunchAgent. The assertion is `pane run w1:p1 ` + the line, i.e. the herdr argv INCLUDING the command, which is the subject: that the multiplexer slot's occupant does not rewrite, re-quote or re-split what posse gave it. launchLog would answer a different question and there is no App in this fixture to ask it with.",
	},
}

// NOT ON THAT LIST, and worth saying because it is the file a reader of this
// census will look for: internal/posse/paneline_test.go, whose whole subject
// IS which of the two places a line landed in. It needs no exemption because
// it reads neither reader — `paneRunLines` for the typed bytes and
// `os.ReadFile(a.LaunchScript(…))` for the spilled ones, compared against
// each other. A pin whose subject is the spill reads the two halves apart,
// which is exactly what keeps it out of scope here.

// llIsReaderCall reports whether e is a call of one of the two calls.log
// readers, and which.
func llIsReaderCall(e ast.Expr) (string, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return "", false
	}
	if id.Name == llRawReader || id.Name == llSafeReader {
		return id.Name, true
	}
	return "", false
}

// llIsMessageCall reports whether a call's arguments are a MESSAGE rather
// than an assertion — `t.Errorf`, `t.Fatal`, `fmt.Sprintf` and their
// relatives. Such a call is skipped whole: a failure message naming `claude`
// is prose, and a reader that read one as an assertion would score half the
// package in scope.
func llIsMessageCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "Errorf", "Fatalf", "Logf", "Skipf", "Error", "Fatal", "Log", "Skip",
		"Sprintf", "Printf", "Fprintf", "WriteString":
		return true
	}
	return false
}

// llComparisons are the operators that compare the log with something: a
// `log != "…"` is an assertion the same way `strings.Contains(log, "…")` is.
var llComparisons = map[token.Token]bool{
	token.EQL: true, token.NEQ: true,
	token.LSS: true, token.LEQ: true, token.GTR: true, token.GEQ: true,
}

// llAssertedText is the text one expression compares the log against: the
// string literals it spells, plus — for every name it mentions — whatever
// string literals that name's binding spells.
//
// The second half is why the reader can see
// `"claude " + ClaudeStrictMCPFlag + " " + ClaudeFleetFlags`: the two
// constants resolve through the package env, and a local bound to a
// concatenation contributes the literal pieces of it even when the whole
// does not resolve.
func llAssertedText(e ast.Expr, env map[string]string, locals map[string][]string) []string {
	var out []string
	seen := map[string]bool{}
	keep := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	ast.Inspect(e, func(n ast.Node) bool {
		x, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		if s, ok := llLit(x); ok {
			keep(s)
			return true
		}
		if id, ok := x.(*ast.Ident); ok {
			keep(env[id.Name])
			for _, s := range locals[id.Name] {
				keep(s)
			}
		}
		return true
	})
	return out
}

// llLiteralParts collects every string literal an expression spells, at any
// depth — the pieces of a concatenation the whole of which does not resolve.
func llLiteralParts(e ast.Expr) []string {
	var out []string
	ast.Inspect(e, func(n ast.Node) bool {
		x, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		if s, ok := llLit(x); ok {
			out = append(out, s)
		}
		return true
	})
	return out
}

// llOneLine renders an expression for a diagnostic, with no newlines.
func llOneLine(e ast.Expr, fset *token.FileSet) string {
	if e == nil {
		return "<no single expression>"
	}
	var buf strings.Builder
	if err := printer.Fprint(&buf, fset, e); err != nil {
		return fmt.Sprintf("<%T>", e)
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// llPkgSites reads one parsed package and returns every launch-line
// assertion in its _test.go files.
//
// THE ASSOCIATION IS PER CALL, not per statement. A `log := calls(t, fake)`
// binds `log`; the assertion is the `strings.Contains(log, "GATES claude")`
// that names it, and the text it asserts against is that call's OTHER
// arguments. A comparison — `log != "…"`, `strings.Count(log, x) != 2` — is
// read the same way, one operand against the other.
//
// It was statement-level first and that was wrong in both directions
// (MEASURED 2026-11-14). internal/posse/dispatch_qa_test.go:427 is the shape
// that matters: `strings.Count(c, "pane run") != 2 ||
// !strings.Contains(launchLog(t, b.App, fake), "GATES claude")` — one `if`
// whose author had already made exactly this distinction, the COUNT off
// calls.log and the CONTENT through launchLog, with a comment saying so. A
// statement-level reader attributed `GATES ` to `c` and reported the one site
// in the tree that had got it right. Per-call attribution reads it correctly:
// `c` is compared with `pane run`, which is herdr argv.
//
// WHAT THAT COSTS, said out loud: an assertion whose text is not an argument
// of the same call — `regexp.MustCompile(lit).MatchString(log)`, or a literal
// handed to a helper that does the comparing — is not associated and the
// site does not enter scope. No live site is spelled either way, and the
// sites floor is what notices if one stops being.
//
// A site is one (file, function, reader): a function that rebinds `log`
// three times asserts on the launch line once, and the exemption key is the
// same pair. The line reported is the first assertion, which is what an
// author needs to find it.
func llPkgSites(files map[string]*ast.File, fset *token.FileSet, env map[string]string, markers []llMarker) []llSite {
	var out []llSite
	for _, p := range cxaSorted(files) {
		if !strings.HasSuffix(p, "_test.go") {
			continue
		}
		for _, d := range files[p].Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			// The readers' own declarations are not assertions.
			// `launchLog`'s body reads `calls` and installs the `GATES `
			// marker itself, so a reader that did not skip it reported the
			// helper this whole pin exists to recommend.
			if fn.Name.Name == llRawReader || fn.Name.Name == llSafeReader {
				continue
			}
			binds := map[string][]llBind{}
			locals := map[string][]string{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != len(as.Rhs) {
					return true
				}
				for i := range as.Lhs {
					id, ok := as.Lhs[i].(*ast.Ident)
					if !ok {
						continue
					}
					if reader, ok := llIsReaderCall(as.Rhs[i]); ok {
						binds[id.Name] = append(binds[id.Name], llBind{as.Rhs[i].Pos(), reader})
						continue
					}
					if parts := llLiteralParts(as.Rhs[i]); len(parts) > 0 {
						locals[id.Name] = append(locals[id.Name], parts...)
					}
				}
				return true
			})
			for name := range binds {
				sort.Slice(binds[name], func(i, j int) bool { return binds[name][i].at < binds[name][j].at })
			}

			hits := map[string][]string{} // reader → markers
			line := map[string]int{}
			code := map[string]string{}
			record := func(reader string, at int, where ast.Expr, found []string) {
				hits[reader] = append(hits[reader], found...)
				if l, seen := line[reader]; !seen || at < l {
					line[reader] = at
					code[reader] = llOneLine(where, fset)
				}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				var operands []ast.Expr
				switch v := n.(type) {
				case *ast.CallExpr:
					if llIsMessageCall(v) {
						return false
					}
					operands = v.Args
				case *ast.BinaryExpr:
					if !llComparisons[v.Op] {
						return true
					}
					operands = []ast.Expr{v.X, v.Y}
				default:
					return true
				}
				for i, subj := range operands {
					readers := llReadersIn(subj, binds)
					if len(readers) == 0 {
						continue
					}
					var found []string
					for j, other := range operands {
						if i == j {
							continue
						}
						for _, text := range llAssertedText(other, env, locals) {
							for _, m := range markers {
								if strings.Contains(text, m.Text) {
									found = append(found, m.Text)
								}
							}
						}
					}
					if len(found) == 0 {
						continue
					}
					for _, reader := range readers {
						record(reader, fset.Position(n.Pos()).Line, n.(ast.Expr), found)
					}
				}
				return true
			})
			for _, reader := range cxaSorted(hits) {
				out = append(out, llSite{
					File:    filepath.ToSlash(p),
					Line:    line[reader],
					Func:    fn.Name.Name,
					Reader:  reader,
					Markers: llUniq(hits[reader]),
					Code:    code[reader],
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Reader < out[j].Reader
	})
	return out
}

// llBind is one binding of a name to a calls.log reader, at a position.
type llBind struct {
	at     token.Pos
	reader string
}

// llReadersIn returns the calls.log readers an expression reads from: a name
// bound to one, or a direct call of one.
//
// A NAME IS RESOLVED AT THE POSITION IT IS USED, to the latest binding at or
// before it. internal/posse/herdr_test.go's TestPersonaLaunchRuntime is why:
// it binds `got` to `launchLog` once and to `calls` twice, over four
// assertions, and a map of name → reader holding only the last one blamed
// `calls` for the assertion that had correctly used `launchLog`. Lexical
// order and not control flow — which is exact for a straight-line test body
// and is what every one of these is.
func llReadersIn(e ast.Expr, binds map[string][]llBind) []string {
	seen := map[string]bool{}
	ast.Inspect(e, func(n ast.Node) bool {
		x, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		if id, ok := x.(*ast.Ident); ok {
			if reader, ok := llReaderAt(binds[id.Name], id.Pos()); ok {
				seen[reader] = true
			}
			return true
		}
		if reader, ok := llIsReaderCall(x); ok {
			seen[reader] = true
		}
		return true
	})
	return cxaSorted(seen)
}

// llReaderAt resolves one name's reader at a use position: the latest
// binding at or before it, else — for a use that lexically precedes every
// binding, which a loop body can spell — the first.
func llReaderAt(bs []llBind, use token.Pos) (string, bool) {
	if len(bs) == 0 {
		return "", false
	}
	reader := bs[0].reader
	for _, b := range bs {
		if b.at > use {
			break
		}
		reader = b.reader
	}
	return reader, true
}

// llUniq sorts and de-duplicates a marker list.
func llUniq(in []string) []string {
	sort.Strings(in)
	var out []string
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// llParsePkg parses every .go file in one package directory with no build
// constraints applied, so the untagged, posse_arm2 and posse_arm3 files of
// internal/posse are all read by one call. A fix verified under one tag with
// the sites under another never compiled is the escape this census has to be
// out of — the same reason temproot_qa_test.go's trGoPkg reads them together.
func llParsePkg(dir string) (map[string]*ast.File, *token.FileSet, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", p, perr)
		}
		parsed[p] = f
	}
	return parsed, fset, nil
}

// llSites is the live count of in-scope launch-line assertions, and the
// floor below is the count ITSELF rather than a margin under it — the shape
// ranger-base-sxez7 argued for temproot_qa_test.go's roots floor and for the
// same reason. A margin is right for a file count, which moves with every
// landing and teaches nothing; this number moves only when somebody adds or
// removes a launch-line assertion, and NOTICING ONE LEAVE is the whole job of
// the floor. It is also the one thing covering the direction the head comment
// calls loose: a site that drops out because its assertion was respelled past
// the vocabulary reds here rather than going quiet.
//
// So a landing that legitimately removes one reds this arm and edits this
// line, which is the intended cost. It is the number the t.Logf prints, so a
// run with -v re-measures it.
const llSites = 21

// Every pin in internal/posse that asserts on a LAUNCH LINE reads it through
// launchLog, which reads both places the line lands in — not through calls,
// which reads only what was typed.
func TestQAEveryLaunchLineAssertionReadsBothPlacesTheLineLandsIn(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("internal", "posse")
	files, fset, err := llParsePkg(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	// Reader sanity before the arms trust it. An assertion that every
	// launch-line pin uses the safe reader is satisfied by a census that
	// found none, and a census that found none is silent about it. The file
	// floor sits well under the live count (885 .go files, MEASURED
	// 2026-11-14) so an ordinary landing does not move it; the SITES floor at
	// the foot of this function is exact.
	if len(files) < 600 {
		t.Fatalf("the read found %d .go file(s) in %s — the reader is broken, not the tree (885, MEASURED 2026-11-14)", len(files), dir)
	}
	env := llStrings(files)

	names, derived := llRuntimeMarkers(files, env)
	// The derived half is the one that must keep growing with the tree, so
	// its floors are over what it READ rather than over what it produced: a
	// derivation that stopped finding builtinRuntimes, or stopped following
	// Realize, goes quiet in exactly the way a vocabulary cannot afford.
	if len(names) < 3 {
		t.Fatalf("the derivation read %v out of builtinRuntimes — fewer than the three built-ins that have shipped since ADR 0017, so the launch-line vocabulary is keyed on almost nothing", names)
	}
	if len(derived) < 15 {
		t.Fatalf("the derivation produced %d marker(s) over runtimes %v, want at least 15 (23, MEASURED 2026-11-14)\n"+
			"Every one is read out of internal/posse/runtime.go: a Command or Unattended flag, a ModelFlag head, or a `-`flag literal in a Realize or Skills body. A collapse here does not red the census — it empties it.", len(derived), names)
	}
	markers := append(append([]llMarker(nil), llListedMarkers...), derived...)
	for _, m := range llListedMarkers {
		if len(strings.TrimSpace(m.Why)) < 60 {
			t.Errorf("the listed marker %q carries no argument worth reading (%q) — the hand-kept half of an inclusion vocabulary is only as good as the reason for each entry", m.Text, m.Why)
		}
	}

	type exemptKey struct{ file, fn string }
	exempt := map[exemptKey]string{}
	for _, e := range llExempt {
		exempt[exemptKey{e.File, e.Func}] = e.Reason
	}
	seenExempt := map[exemptKey]bool{}

	sites := llPkgSites(files, fset, env, markers)
	for _, s := range sites {
		k := exemptKey{s.File, s.Func}
		if reason, ok := exempt[k]; ok {
			seenExempt[k] = true
			if len(strings.TrimSpace(reason)) < 60 {
				t.Errorf("%s (%s) is exempted from the launch-line reader rule with no reason worth reading (%q) — an exemption nobody can check is the rule being dropped quietly", s.At(), s.Func, reason)
			}
			continue
		}
		if s.Reader == llSafeReader {
			continue
		}
		t.Errorf("%s asserts on a launch line through %s, which reads only what was TYPED:\n"+
			"  in:      %s\n"+
			"  asserts: %s\n"+
			"  markers: %s\n"+
			"  want:    %s(t, <app>, fake), or a row in llExempt (internal/treepins/launchlineread_qa_test.go) with the reason\n"+
			"A rendered line over PaneLineMax (%s, the tty's MAX_CANON) is not typed into the pane at all: it is spilled to state/launch/<session>.sh and the pane types `. '<script>'` instead (internal/posse/paneline.go). Which of the two places it lands in is a fact about the line's LENGTH — so about this fixture's path lengths, about TMPDIR, and about any unrelated landing that adds bytes — and never about the launch. %s reads both. MEASURED 2026-11-13: parity_test.go's grok line was 959 bytes with TMPDIR unset and 1325 with TMPDIR under the home, 64 bytes of headroom, and the pin red over a launch that was correct (ranger-base-6ca29). In production every persona's line spills, so the typed form this reader sees is the exceptional path and an assertion satisfied by it can be green over a spilled line nobody read.",
			s.At(), s.Reader, s.Func, s.Code, strings.Join(s.Markers, " "), llSafeReader, llPaneLineMax(), llSafeReader)
	}
	for _, e := range llExempt {
		if !seenExempt[exemptKey{e.File, e.Func}] {
			t.Errorf("llExempt names %s (%s) and the census finds no launch-line assertion there — a stale exemption is a rule nobody is keeping. Delete the row, or find out what took the site out of the census.", e.File, e.Func)
		}
	}

	// And the floor on the SITES, not just the files: a reader that silently
	// stopped associating an assertion with its subject would pass every arm
	// above.
	var safe, raw int
	for _, s := range sites {
		if s.Reader == llSafeReader {
			safe++
		} else {
			raw++
		}
	}
	t.Logf("in-scope launch-line assertions: %d (%d through %s, %d through %s), over %d .go files, %d derived markers over runtimes %v and %d listed",
		len(sites), safe, llSafeReader, raw, llRawReader, len(files), len(derived), names, len(llListedMarkers))
	t.Logf("derived markers: %v", llMarkerTexts(derived))
	for _, s := range sites {
		t.Logf("  %s %s in %s — %s", s.At(), s.Reader, s.Func, strings.Join(s.Markers, " "))
	}
	if len(sites) < llSites {
		t.Fatalf("the census found %d in-scope launch-line assertion(s), want at least %d (MEASURED 2026-11-14, linux, this box)\n"+
			"A site LEAVING this census is what this floor is for, and it does not care why: a reader that stopped associating a subject with its assertion, an assertion respelled past the marker vocabulary, or a pin genuinely deleted all read the same here. Check the diff for an assertion whose text changed; if one really was removed, llSites at the head of this file is the line to lower, and lowering it is a decision rather than a formality — the vocabulary is an inclusion list and this floor is the only thing watching the direction it cannot cover.", len(sites), llSites)
	}
}

// The reader and the rule over planted sources, because an assertion that
// only ever ran over a tree that passes is a census nobody has shown can say
// no — and "it reds on a launch-line pin moved back to calls()" is this
// bead's own done-when.
//
// It reads one directory under t.TempDir(), so this arm is not in the
// tree-wide class and needs no door of its own (the shape
// temproot_qa_test.go's control arm uses next door).
func TestQATheLaunchLineAssertionCensusCanStillSayNo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name string, lines ...string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// A non-test file of the same package, holding the declarations the
	// derivation reads. Two runtimes, so "it read more than the first" is
	// observable, and the Realize/Skills hop is spelled the way runtime.go
	// spells it: a named function in another file of the package.
	write("runtime.go",
		"package probe",
		``,
		"const ProbeFleetFlags = `--permission-mode auto`",
		``,
		"var builtinRuntimes = []Runtime{",
		"	{Name: \"vim\", ModelFlag: \"--model %s\", Realize: realizeVim, Skills: skillsVim, Unattended: ProbeFleetFlags,",
		"		Command: `vim {model} ` + ProbeFleetFlags + ` --append-system-prompt \"$(cat {file})\"`},",
		"	{Name: \"nano\", ModelFlag: \"-c model=%s\", Realize: realizeNano,",
		"		Command: `nano {model} -a never --nanoflag {deny}`},",
		"}",
		``,
		"func realizeVim() string { return \"--allowedTools \" + \"-s read-only\" }",
		"func skillsVim() string { return \"--plugin-dir \" }",
		"func realizeNano() string { return \"--nanorules \" }",
	)

	// The pins. The number on each `calls`/`launchLog` line is the line the
	// census reports for it, which is the ASSERTION's line and not the
	// reader call's — a site is one (file, function, reader), and the line
	// is where the claim is made, because that is what an author needs in
	// order to find it.
	write("probe_test.go",
		"package probe",
		``,
		"func TestTypedOnly(t *testing.T) {",
		"	log := calls(t, fake)", // 5  RED: GATES
		"	if !strings.Contains(log, \"GATES claude\") {",
		"		t.Errorf(\"no:\\n%s\", log)",
		"	}",
		"}",
		``,
		"func TestHerdrArgvIsNotALaunchLine(t *testing.T) {",
		"	log := calls(t, fake)", // 12 out: herdr's own create line
		"	if !strings.Contains(log, \"workspace create\") || !strings.Contains(log, \"--env RHQ_TIER=fast\") {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestSafeReader(t *testing.T) {",
		"	log := launchLog(t, a, fake)", // 19 in scope, and compliant
		"	if !strings.Contains(log, \"GATES vim\") {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestDerivedTemplateFlag(t *testing.T) {",
		"	log := calls(t, fake)", // 26 RED: --append-system-prompt
		"	if !strings.Contains(log, \"--append-system-prompt\") {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestDerivedRealizeFlag(t *testing.T) {",
		"	log := calls(t, fake)", // 33 RED: -s read-only, from realizeVim
		"	if !strings.Contains(log, \"-s read-only\") {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestDerivedSkillsFlag(t *testing.T) {",
		"	log := calls(t, fake)", // 40 RED: --plugin-dir, from skillsVim
		"	if !strings.Contains(log, \"--plugin-dir\") {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestDerivedModelFlagHead(t *testing.T) {",
		"	log := calls(t, fake)", // 47 RED: -c model=, nano's ModelFlag
		"	if !strings.Contains(log, \"-c model='nano-1'\") {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestShortFlagIsNotAMarker(t *testing.T) {",
		"	log := calls(t, fake)", // 54 out: `-a never` is two bytes of flag
		"	if !strings.Contains(log, \"-a never\") {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestInlineReader(t *testing.T) {",
		"	if !strings.Contains(calls(t, fake), \"GATES nano\") {", // 60 RED
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestThroughAConstant(t *testing.T) {",
		"	log := calls(t, fake)", // 67 RED: ProbeFleetFlags resolves
		"	if !strings.Contains(log, \"vim \"+ProbeFleetFlags) {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestThroughALocalConcatenation(t *testing.T) {",
		"	log := calls(t, fake)", // 75 RED: the local's literal pieces
		"	want := prefix + \"--nanorules \" + suffix",
		"	if !strings.Contains(log, want) {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestOnlyInAFailureMessage(t *testing.T) {",
		"	log := calls(t, fake)", // 82 out: prose, not an assertion
		"	if !strings.Contains(log, \"workspace close\") {",
		"		t.Errorf(\"GATES vim never ran:\\n%s\", log)",
		"	}",
		"}",
		``,
		"func TestRuntimeNameNeedsItsSpace(t *testing.T) {",
		"	log := calls(t, fake)", // 89 out: `=vim` is not `vim `
		"	if !strings.Contains(log, \"--env RHQ_RUNTIME=vim\") {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func TestNotAReaderAtAll(t *testing.T) {",
		"	lines := paneRunLines(t, fake)", // 96 out: a different reader
		"	if !strings.Contains(resolvedTypedLine(t, lines[0]), \"GATES vim\") {",
		"		t.Errorf(\"no\")",
		"	}",
		"}",
	)

	files, fset, err := llParsePkg(dir)
	if err != nil {
		t.Fatal(err)
	}
	env := llStrings(files)
	names, derived := llRuntimeMarkers(files, env)
	if got := strings.Join(names, ","); got != "nano,vim" {
		t.Errorf("the derivation read runtimes %q out of the fixture's builtinRuntimes, want nano,vim — the half of the vocabulary that has to grow with the tree is not being read out of the source", got)
	}
	// The markers the fixture's declarations must yield, and nothing of the
	// one-letter flags: `-a` and `-m` are under llMinFlag and a marker that
	// short matches text with nothing to do with a launch.
	wantMarkers := []string{
		"--allowedTools", "--append-system-prompt", "--model", "--nanoflag",
		"--nanorules", "--permission-mode", "--plugin-dir", "-c model=",
		"-s read-only", "nano ", "vim ",
	}
	got := map[string]bool{}
	for _, m := range derived {
		got[m.Text] = true
		if strings.TrimSpace(m.Why) == "" {
			t.Errorf("the derived marker %q says nothing about where it came from", m.Text)
		}
	}
	for _, w := range wantMarkers {
		if !got[w] {
			t.Errorf("the derivation missed %q — derived %d marker(s): %v", w, len(derived), llMarkerTexts(derived))
		}
	}
	for _, never := range []string{"-a", "-m", "-m ", "{model}", "{deny}", "never"} {
		if got[never] {
			t.Errorf("the derivation took %q as a launch-line marker — a token that short or that generic scores unrelated text in scope", never)
		}
	}

	markers := append(append([]llMarker(nil), llListedMarkers...), derived...)
	sites := llPkgSites(files, fset, env, markers)
	type seen struct {
		line   int
		reader string
	}
	var gotSeen []seen
	for _, s := range sites {
		gotSeen = append(gotSeen, seen{s.Line, s.Reader})
	}
	// The ASSERTION's line, not the reader call's: a site is one (file,
	// function, reader) and the line reported is where the claim is made,
	// which is what an author needs in order to find it.
	wantSeen := []seen{
		{5, llRawReader},
		{19, llSafeReader},
		{26, llRawReader},
		{33, llRawReader},
		{40, llRawReader},
		{47, llRawReader},
		{60, llRawReader},
		{67, llRawReader},
		{75, llRawReader},
	}
	if fmt.Sprint(gotSeen) != fmt.Sprint(wantSeen) {
		t.Errorf("the census found\n  %v\nwant\n  %v\n"+
			"IN: the GATES marker (5), the safe reader's own site (19), a Command flag (26), a Realize flag (33), a Skills flag (40), a ModelFlag head (47), an inline reader (60), a marker reached through a package constant (67) and one reached through a local concatenation's pieces (75).\n"+
			"OUT: herdr's own `workspace create --env` line (12), a flag under llMinFlag (54), a marker that appears only in a failure MESSAGE (82), a runtime name without its trailing space (89), and a pin that reads paneRunLines/resolvedTypedLine rather than either reader (96).", gotSeen, wantSeen)
	}
	var reds int
	for _, s := range sites {
		if s.Reader == llRawReader {
			reds++
		}
	}
	if reds != 8 {
		t.Errorf("the rule reds %d of the planted pins, want 8 (assertions at lines 5, 26, 33, 40, 47, 60, 67 and 75) — a census that cannot say no is not a census", reds)
	}

	// And the two readings the live census needed and the planted pins above
	// do not reach, both over one function: a name rebound from one reader
	// to the other resolves AT EACH USE (internal/posse/herdr_test.go's
	// TestPersonaLaunchRuntime), and the readers' OWN declarations are not
	// assertions (launchLog's body reads calls and installs `GATES ` itself).
	write("rebind_test.go",
		"package probe",
		``,
		"func TestRebound(t *testing.T) {",
		"	got := launchLog(t, a, fake)", // 4
		"	if !strings.Contains(got, \"GATES vim\") {", // 5  launchLog
		"		t.Errorf(\"no\")",
		"	}",
		"	got = calls(t, fake)", // 8
		"	if !strings.Contains(got, \"GATES nano\") {", // 9  calls
		"		t.Errorf(\"no\")",
		"	}",
		"}",
		``,
		"func launchLog(t *testing.T, a *App, fake string) string {",
		"	out := calls(t, fake)",                                 // 15
		"	return gatePrefixRe.ReplaceAllString(out, \"GATES \")", // 16 not a site
		"}",
	)
	files, fset, err = llParsePkg(dir)
	if err != nil {
		t.Fatal(err)
	}
	env = llStrings(files)
	_, derived = llRuntimeMarkers(files, env)
	markers = append(append([]llMarker(nil), llListedMarkers...), derived...)
	var rebound []seen
	for _, s := range llPkgSites(files, fset, env, markers) {
		if strings.HasSuffix(s.File, "rebind_test.go") {
			rebound = append(rebound, seen{s.Line, s.Reader})
		}
	}
	wantRebound := []seen{{5, llSafeReader}, {9, llRawReader}}
	if fmt.Sprint(rebound) != fmt.Sprint(wantRebound) {
		t.Errorf("over a rebound subject the census found\n  %v\nwant\n  %v\n"+
			"Line 5 asserts on `got` while it holds launchLog's result and line 9 after it was reassigned from calls — a reader keyed on the name alone resolves both to the LAST binding and blames `calls` for the assertion that got it right, which is what it did to internal/posse/herdr_test.go's TestPersonaLaunchRuntime. And launchLog's own declaration must contribute nothing: its body reads `calls` and installs the `GATES ` marker, so a census that did not skip the readers' own declarations reported the helper it exists to recommend.", rebound, wantRebound)
	}
}

// llMarkerTexts is the marker texts alone, for a diagnostic.
func llMarkerTexts(in []llMarker) []string {
	var out []string
	for _, m := range in {
		out = append(out, m.Text)
	}
	sort.Strings(out)
	return out
}
