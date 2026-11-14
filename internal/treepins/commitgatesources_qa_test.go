package treepins

// QA pin for ranger-base-m1obc — the membership half of the COMMIT-TIME GATE
// reading, derived for the same reason its two siblings are derived.
//
// THE CATEGORY. A wall is install-gated because the installed binary RENDERS
// it (wallrendersources_qa_test.go); a launch-time check because the
// installed binary DECIDES it at launch (launchchecksources_qa_test.go);
// this one because the installed binary JUDGES it at COMMIT time. The hook
// BODY is a render and gates.go is already in WallRenderSources — but the
// judge the body pipes through is a SUBPROCESS of whatever `posse` is on
// PATH, resolved fresh at commit time on purpose (vulnPrivateChecks: "an
// id's open/closed status is exactly the live fact a render-time stamp
// cannot hold"). So a perfectly current hook body can carry a verdict from a
// binary 59 commits behind, and the file that holds the verdict is in
// neither sibling list: vulnprivate.go writes no `do not edit` header, which
// is the wall class's key, and declares no func naming *Parity and
// constructs no Runtime, which are the launch-check class's two keys. It is
// CORRECTLY outside both, and hand-adding it to either would red that list's
// pin AND attach the wrong cost sentence.
//
// MEASURED 2026-11-14 on this box over its own exposure window — installed
// 0.5.3+7d0f8c31 against origin/main at 1d03f9d1, 59 commits: 8 rendering a
// wall, 7 deciding a launch-time check, 4 judging a commit-time gate
// (e3b41b30, 6067b1d7, e208dba9, 72fe7738). Exactly ONE of the four appeared
// in the printed reading at all — 6067b1d7, which also touched gates.go and
// so was counted under "rendering a wall", beside a cost sentence about
// seatbelts, gate shims, cage mounts and hooks that says nothing about why
// the seat was being refused. ranger-base-ed07e's own-bead exemption is one
// of the four, so until it was installed an open vuln-private bead's own fix
// could carry no descriptive commit subject and its notes fragment could not
// land at all. docs/notes.d/ranger-base-v81vo.md section 3 has the reading.
//
// THE DERIVED CLASS, in two clauses, because a commit-time gate has two
// halves on opposite sides of a process boundary and they share no spelling:
//
//   - THE INVOCATION, read off the REAL RENDER and not off an AST. The hook
//     body is shell, so this clause calls the renderer posse installs —
//     (*App).CommitGuardHook — and reads the bytes for every `posse gates
//     <verb>` in them. An AST walk over gates.go could not do this: the body
//     is assembled from a dozen consts and helpers, and the verb a commit
//     actually pipes to is a property of the assembled text.
//
//   - THE JUDGE, read off cmd/posse's own dispatch. For each verb, the
//     `if <x> == "<verb>"` arm in cmd/posse/main.go, and the `RunGates*`
//     entry point called inside it; the non-test internal/posse file
//     declaring that entry point is the member. `RunGates*` is the whole
//     family of hidden `posse gates <verb>` subcommands something posse
//     RENDERS invokes — RunGatesWrap, RunGatesBdLine, RunGatesBdWrite,
//     RunGatesRenderProbe, RunGatesVulnPrivateScan — so the prefix is the
//     claim and no go/types load or call-graph walk is needed.
//
// WHY AN INTERSECTION AND NOT A REGEX OVER THE RENDER. The rendered hook
// body NAMES three verbs and invokes one: `posse gates install-hooks` and
// `posse gates adr-census` appear in its own comments and in its refusal
// text ("re-run posse gates install-hooks"), which a shell-mention filter
// cannot separate from an invocation — MEASURED 2026-11-14 on this tree, 8
// of the 10 occurrences are mentions (seven `install-hooks`, one
// `adr-census`) and SIX of those eight are inside `echo` refusal text, so a
// drop-the-comment-lines filter would still leave six. The dispatch half is
// what sorts them: install-hooks goes to
// an installer and adr-census to RunAdrCensus, neither a `RunGates*` judge,
// and only vuln-private-scan is both named by the body and dispatched to
// one. Over-derivation (a refusal text that one day quotes `posse gates
// bd-write`) reds this pin and asks a human, which is the safe direction and
// the same trade the wall pin's call closure makes.
//
// PRICED, as the bead asked: 0.196s, 0.197s and 0.216s for all four arms over
// three runs on this box, 2026-11-14, at a one-minute load average of 1.98
// with neither suite slot held by anyone (`go test -count=1 -run CommitGate
// ./internal/treepins`, warm build cache). It takes no suite slot and is
// nothing a door has to budget for — one hook render in memory, one
// go/parser pass over cmd/posse/main.go, one os.ReadDir plus a parse per
// non-test file under internal/posse, and no git, no exec and no tempdir.
// `make treepins-check` is the door that runs it, since ADR 0098 named that
// door by what it does NOT run: a pin added here is doored the moment it is
// written, with no Makefile edit, and a fifth of a second does not go near
// the priced-row table it would need an exemption from.
//
// A FLOOR, and CommitGateSources says so where it is declared: a commit-time
// judge reached by neither clause — a verb invoked from a hook body posse
// does NOT render, a judge dispatched under a name that is not `RunGates*` —
// is invisible here, which is why nothing keys off the list. The reading
// always reports the WHOLE lag and always names `make install`; the
// per-category counts only say how loud.
//
// NOT bd-write, and arm 4 is why it stays out by construction rather than by
// hand: its invoking render is the bd gate SHIM and not a hook body, its
// decision is taken at WRITE time, and its consequence is a lost or
// duplicated bead write (ADR 0073) rather than a refused commit — a fourth
// row by wallrenderer.go's own rule, carried by ranger-base-7fjmv.
//
// NOT DISJOINT from either sibling, and deliberately not pinned to be: a
// file can render a wall and hold a judge, and a commit to it is honestly
// counted in both categories.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// cgsRunPrefix is the naming convention of a hidden `posse gates <verb>`
// subcommand's entry point. It is the family, not a guess: every one in this
// tree spells it, and cgsDeclarers fails outright if the family is ever gone
// rather than deriving an empty class. No count is written here — the number
// drifts with every verb added, and a stale one in a comment is worse than
// none.
const cgsRunPrefix = "RunGates"

// cgsDispatch is the file that maps a gates verb to its entry point. One
// file, because `posse gates` has one dispatcher and a second would be a
// finding of its own.
const cgsDispatch = "cmd/posse/main.go"

// cgsInvocation matches `posse gates <verb>` in rendered shell. Lower-case
// kebab only, which is every verb this dispatcher has ever carried.
var cgsInvocation = regexp.MustCompile(`posse gates ([a-z0-9][a-z0-9-]*)`)

// cgsCommitHookBody is the rendered COMMIT-time hook body — the real bytes
// posse installs, assembled from every const and helper that composes it.
//
// Public visibility and a bare OpsPatternSet, with no identity literals:
// check 4's content arm is unconditional and its message arm rides check 3's
// reader, so neither needs this box's own values, and a fixture that needed
// them would be a fixture that could disagree with what an operator's tree
// renders. An App with no Home renders no home stamp (commitGuardHomeStamp),
// which is the one difference from an installed body and touches no verb.
func cgsCommitHookBody() string {
	return (&posse.App{}).CommitGuardHook(posse.VisibilityPublic, posse.OpsPatternSet{})
}

// cgsVerbsNamed returns every `posse gates <verb>` the rendered text names,
// mentions included — the dispatch clause is what sorts invocations from
// mentions (see the head comment).
func cgsVerbsNamed(body string) map[string]bool {
	out := map[string]bool{}
	for _, m := range cgsInvocation.FindAllStringSubmatch(body, -1) {
		out[m[1]] = true
	}
	return out
}

// cgsJudges parses cmd/posse/main.go and returns verb -> the `RunGates*`
// entry point its dispatch arm calls. The shape is uniform and syntactic: an
// `if` whose condition compares something to a string literal, and a call to
// a `RunGates*` func or method somewhere in its body.
func cgsJudges(t *testing.T, root string) map[string]string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(cgsDispatch))
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("cannot parse %s: %v", cgsDispatch, err)
	}
	out := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		ifs, ok := n.(*ast.IfStmt)
		if !ok || ifs.Body == nil {
			return true
		}
		bin, ok := ifs.Cond.(*ast.BinaryExpr)
		if !ok || bin.Op != token.EQL {
			return true
		}
		verb := ""
		for _, side := range []ast.Expr{bin.X, bin.Y} {
			if lit, ok := side.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				// An unparseable literal is not a finding — go/parser
				// already accepted the file — it is a literal this clause
				// cannot read.
				if s, err := strconv.Unquote(lit.Value); err == nil {
					verb = s
				}
			}
		}
		if verb == "" {
			return true
		}
		ast.Inspect(ifs.Body, func(m ast.Node) bool {
			call, ok := m.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			}
			if strings.HasPrefix(name, cgsRunPrefix) {
				out[verb] = name
			}
			return true
		})
		return true
	})
	if len(out) == 0 {
		t.Fatalf("no `if … == \"<verb>\"` arm in %s calls a %s* entry point — the dispatch this pin derives its second clause from has changed shape, so the commit-gate class would derive EMPTY and `posse gates` would read \"0 of them judging a commit-time gate\" over a stale judge", cgsDispatch, cgsRunPrefix)
	}
	return out
}

// cgsDeclarers returns, per `RunGates*` entry point, the non-test
// internal/posse file that declares it.
func cgsDeclarers(t *testing.T, root string) map[string]string {
	t.Helper()
	dir := filepath.Join(root, "internal", "posse")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", dir, err)
	}
	out := map[string]string{}
	fset := token.NewFileSet()
	seen := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		seen++
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", name, err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fd.Name.Name, cgsRunPrefix) {
				continue
			}
			out[fd.Name.Name] = "internal/posse/" + name
		}
	}
	if seen < 50 {
		// The package is ~200 files. A handful means the walk found the
		// wrong directory, and every arm below would then pass over nothing.
		t.Fatalf("only %d non-test sources under %s — this pin is reading the wrong tree", seen, dir)
	}
	if len(out) == 0 {
		t.Fatalf("no file under %s declares a %s* entry point — the family this pin derives its second clause from is gone", dir, cgsRunPrefix)
	}
	return out
}

// cgsSources is the class: the non-test internal/posse file holding the
// judge behind every verb the rendered commit-guard hook body names, keyed by
// path and valued by the verb that put it there.
func cgsSources(t *testing.T, root string) map[string]string {
	t.Helper()
	named := cgsVerbsNamed(cgsCommitHookBody())
	if len(named) == 0 {
		t.Fatalf("the rendered commit-guard hook body names no `posse gates <verb>` at all — either the render stopped composing check 4's arm or %q stopped matching it; the class would derive EMPTY either way", cgsInvocation)
	}
	judges, declarers := cgsJudges(t, root), cgsDeclarers(t, root)
	out := map[string]string{}
	for verb := range named {
		fn, ok := judges[verb]
		if !ok {
			continue // named but not dispatched to a judge: a mention.
		}
		path, ok := declarers[fn]
		if !ok {
			t.Errorf("%s dispatches `posse gates %s` to %s and no non-test file under internal/posse declares it — the judge the commit-guard hook pipes to lives outside the package this pin reads, so the commit-gate lag count cannot see it. Widen cgsDeclarers to the package that holds it.", cgsDispatch, verb, fn)
			continue
		}
		out[path] = verb
	}
	return out
}

// ARM 1: every derived commit-gate source is in CommitGateSources, so the
// count `posse gates` and the promote epilogue print cannot silently omit a
// commit-time judge — the silence ranger-base-m1obc was filed for, where 4
// of 59 lagging commits changed check 4's judge and the printed reading
// named one of them, as a wall.
func TestCommitGateSourcesNameEveryJudgeTheCommitHookInvokes(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, p := range posse.CommitGateSources {
		listed[p] = true
	}
	for rel, verb := range cgsSources(t, root) {
		if listed[rel] {
			continue
		}
		t.Errorf("%s holds the judge behind `posse gates %s`, which the rendered commit-guard hook body invokes, and posse.CommitGateSources does not name it — so `posse gates` and `posse promote` would count a lag over this file as judging no commit-time gate. That is exactly ranger-base-m1obc's reading: 4 of 59 lagging commits changed this judge and the printed line named one of them, under the WALL cost sentence. Add it to CommitGateSources (wallrenderer.go).", rel, verb)
	}
}

// ARM 2: every listed path is a real file. A pathspec that matches nothing
// makes the commit-gate count read zero over a repo where it should read
// more, and `git log -- <nothing>` says so to nobody. The same arm both
// siblings carry, against a third list.
func TestCommitGateSourcesAllExist(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if len(posse.CommitGateSources) == 0 {
		t.Fatal("posse.CommitGateSources is empty")
	}
	for _, p := range posse.CommitGateSources {
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil || st.IsDir() {
			t.Errorf("posse.CommitGateSources names %s, which is not a file here (%v) — a pathspec that matches nothing renders as \"0 of them judging a commit-time gate\"", p, err)
		}
	}
}

// ARM 3: the list holds nothing the derivation does not reach. The launch-
// check list keeps this arm and the wall list does not, and this one keeps
// it for the launch-check list's reason: both clauses here are mechanical,
// so an entry neither clause finds is an entry that will be wrong one
// refactor from now and reassure the operator while it is. A commit-time
// judge the derivation cannot see belongs in a widened clause, named here,
// not in a hand-appended path.
func TestCommitGateSourcesHoldNothingTheDerivationCannotSee(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	derived := cgsSources(t, root)
	for _, p := range posse.CommitGateSources {
		if _, ok := derived[p]; ok {
			continue
		}
		t.Errorf("posse.CommitGateSources names %s and this pin's derivation does not reach it: no verb the rendered commit-guard hook body names is dispatched by %s to a %s* entry point declared there. A hand-held entry goes stale in the direction that prints reassurance, which is the whole reason this class is derived — widen a clause here, or drop the path.", p, cgsDispatch, cgsRunPrefix)
	}
}

// ARM 4: the derivation's own witness and its two boundaries.
//
// The witness, in the discipline wallrendersources_qa_test.go's arm 1 keeps:
// vulnprivate.go must be DERIVED and not merely listed, and the verb that
// derives it must be check 4's. If this fails, the class has stopped seeing
// the one file it exists for and arm 1 would pass over an empty set.
//
// The two boundaries are what keep the class from quietly eating the rows
// either side of it:
//
//   - PrePushHook names no `posse gates` judge. A push-time judge is a
//     different consequence sentence and a different row; counting one here
//     under "judging a commit-time gate" would be the mis-attribution
//     ranger-base-m1obc was filed about, one category over.
//
//   - bd-write is not in the class. ADR 0073 D2's write-time judge is
//     install-gated through the same callback shape, but its invoking render
//     is the bd gate shim and not a hook body, so it must fall out by
//     construction — ranger-base-7fjmv carries it as a fourth row. If this
//     fails, either the hook body has grown a bd-write arm (then it IS a
//     commit-time gate and 7fjmv is moot) or the dispatch moved, and the two
//     rows need re-deciding rather than merging by accident.
func TestCommitGateDerivationWitnessAndBoundaries(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	const judge, verb = "internal/posse/vulnprivate.go", "vuln-private-scan"
	derived := cgsSources(t, root)
	if got, ok := derived[judge]; !ok || got != verb {
		t.Errorf("the derivation does not reach %s through `posse gates %s` (it derived %v) — this is ADR 0024 D2 check 4's judge and the one file this class exists for, so a derivation that misses it leaves arm 1 passing over nothing", judge, verb, derived)
	}

	// The commit hook names the verb AND the dispatch sends it to a judge:
	// asserted apart, because each half fails for its own reason and a
	// combined check cannot say which half went.
	if !cgsVerbsNamed(cgsCommitHookBody())[verb] {
		t.Errorf("the rendered commit-guard hook body no longer names `posse gates %s` — check 4's arm is not in the body posse installs, so there is no commit-time gate to be behind on and this whole class is moot; delete it deliberately rather than letting it derive empty", verb)
	}
	if got := cgsJudges(t, root)[verb]; !strings.HasPrefix(got, cgsRunPrefix) {
		t.Errorf("%s no longer dispatches `posse gates %s` to a %s* entry point (got %q) — the second clause cannot name the judge, so the class derives empty and the reading reads zero over a stale one", cgsDispatch, verb, cgsRunPrefix, got)
	}

	// BOUNDARY 1: the push-time hook invokes no judge.
	for v := range cgsVerbsNamed(posse.PrePushHook) {
		if fn, ok := cgsJudges(t, root)[v]; ok {
			t.Errorf("PrePushHook names `posse gates %s`, which %s dispatches to %s — a judge the installed binary resolves at PUSH time. That is install-gated like this class and is NOT this class: its consequence sentence is about a push and not a commit. Give it its own lagCategory row (wallrenderer.go) and widen this pin's first clause deliberately, rather than letting a push-time judge be counted as a commit-time one.", v, cgsDispatch, fn)
		}
	}

	// BOUNDARY 2: bd-write is a fourth row (ranger-base-7fjmv), not this one.
	const bdWrite = "internal/posse/bdwritelock.go"
	if v, ok := derived[bdWrite]; ok {
		t.Errorf("%s derived into the commit-time gate class through `posse gates %s`. ADR 0073 D2's judge is taken at WRITE time and its invoking render is the bd gate shim, not a hook body, so ranger-base-m1obc put it in a fourth row (ranger-base-7fjmv) rather than this one. If the commit-guard hook really does pipe to it now, decide the rows again — do not let the two merge because a clause widened.", bdWrite, v)
	}
	for _, p := range posse.CommitGateSources {
		if p == bdWrite {
			t.Errorf("posse.CommitGateSources names %s by hand. It is a FOURTH category (ranger-base-7fjmv): write time, not commit time; a lost or duplicated bead write, not a refused commit; and the bd shim, not a hook body. Give it its own row and its own cost sentence.", p)
		}
	}
}
