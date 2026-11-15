package treepins

// QA pin for ranger-base-7fjmv — the membership half of the WRITE-TIME GATE
// reading, derived for the same reason its three siblings are derived.
//
// THE CATEGORY. A wall is install-gated because the installed binary RENDERS
// it (wallrendersources_qa_test.go); a launch-time check because the
// installed binary DECIDES it at launch (launchchecksources_qa_test.go); a
// commit-time gate because the installed binary JUDGES it at commit time
// (commitgatesources_qa_test.go); this one because the installed binary
// JUDGES it at WRITE time, for every bd write any persona on this box types.
// The bd SHIM is a render and gates.go is already in WallRenderSources — but
// the judge its write-verb tail execs is a SUBPROCESS of whatever `posse` is
// on PATH, resolved fresh on purpose (ADR 0073 D2: the lock has to be held
// by the process that becomes bd, so the judge cannot be a render-time
// stamp). So a perfectly current shim can route every write to a judge a
// binary behind, and the consequence is the one no refusal names: a write
// that loses the race prints `Created issue: <id>` and exits 0 like any
// other (ADR 0073's rig, 34-36 of 48 concurrent creates lost).
//
// MEASURED 2026-11-14 on this box — installed 0.5.3+7d0f8c31 against main at
// 35e8cf41, 75 commits: 12 rendering a wall, 9 deciding a launch-time check,
// 4 judging a commit-time gate, ZERO judging a write-time gate. The zero is
// the point: it is now SAID, and three commits in main's 2,756 have ever
// touched these paths — 2a9f4e06 (bdwritelock.go and beads.go, in no list
// that existed, so it sat inside the headline named by no category at all),
// 21508a1a (gates.go, so it was counted under "rendering a wall" while its
// judge half was named by nothing — the mis-attribution ranger-base-m1obc
// measured, one row over) and 6b4cd64d (bdline.go's creation). The traffic
// the row guards is measured too: 3,264 bd write calls typed across this
// box's 490 retained session transcripts in the week to 2026-11-14, 1,074 of
// them in one day across 123 seats. wallrenderer.go's BdWriteSources and
// docs/notes.d/ranger-base-7fjmv.md hold the whole reading.
//
// THE DERIVED CLASS, in three clauses. The first two are the commit-time
// pin's two, one of them WIDENED and the other CALLED rather than copied;
// the third is this row's own, because its judge is three files and its
// sibling's is one.
//
//   - THE INVOCATION, read off the REAL rendered BD SHIM and not off an AST,
//     for the sibling's reason: the shim is shell, assembled from a dozen
//     consts and helpers, and the verb a bd write actually execs is a
//     property of the assembled text. So this clause renders a persona's
//     gates with posse's own RenderGates and reads the bytes.
//
//     ANCHORED ON `exec`, which is where this clause departs from the
//     sibling's. The commit-time pin reads every `posse gates <verb>` the
//     hook body names and lets the DISPATCH half sort invocations from
//     mentions — sound there, MEASURED: 8 of its 10 occurrences are
//     mentions and not one of them is dispatched to a `RunGates*` judge.
//     It is not sound here. The bd shim's own rendered comment says
//     "closing the TOCTOU the separate \"gates bd-line\" call used to
//     carry", and cmd/posse DOES dispatch bd-line to RunGatesBdLine — so a
//     mention-and-dispatch intersection derives bdlineshim.go off a comment
//     sentence, and the pin would then order a human to drop that path the
//     day the comment is reworded. Reading what the shim EXECS is the
//     spelling the render itself makes unambiguous, and ARM 4's third
//     boundary holds it.
//
//   - THE JUDGE, read off cmd/posse's own dispatch — cgsJudges and
//     cgsDeclarers, the sibling's helpers, CALLED. The clause is identical
//     for both rows (an `if <x> == "<verb>"` arm, the `RunGates*` entry
//     point it calls, the non-test internal/posse file declaring it), and a
//     second implementation of it would be a second thing to keep in step
//     by hand. Their fatals speak in the commit-time row's words because
//     that is where they live; the cause they name is the same for both.
//
//   - THE JUDGE'S OWN HELPERS, in wallrendersources_qa_test.go's call-
//     closure discipline. vulnprivate.go holds its verdict whole; this
//     judge does not — bdwriteshim.go sequences it, bdwritelock.go holds
//     the per-store flock and the write/read classification, bdline.go
//     holds the projection it refuses on, and 2a9f4e06 touched the lock
//     ALONE. So the class closes over calls, following the family the
//     entry point names: strip `RunGates` off the entry point, lower-case
//     what is left, and follow every call whose callee name carries it.
//     By NAME and not by type, the wall pin's own trade: a method and a
//     func sharing a name both count, which can only over-derive.
//
// A FLOOR, and BdWriteSources says so where it is declared. The residual is
// a helper whose name does not carry the stem: bdlineshim.go's refusal
// wording and gate log are called by this judge and are NOT in the class,
// because what they change is what a refused writer is told rather than
// whether the write is refused. Nothing keys off the list — the reading
// always reports the WHOLE lag and always names `make install`.
//
// PRICED, measured on this box, 2026-11-14: see the figures at the foot of
// docs/notes.d/ranger-base-7fjmv.md. Like the sibling it takes no suite slot
// and is nothing a door has to budget for, and `make treepins-check` is the
// door that runs it (ADR 0098 named that door by what it does NOT run, so a
// pin added here is doored the moment it is written).
//
// NOT DISJOINT from any sibling, and deliberately not pinned to be: gates.go
// renders the shim AND is in WallRenderSources, and a commit to it is
// honestly counted in both categories.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// bwsShimExec matches what the rendered bd shim EXECS: posse resolved into a
// shell variable at render time, then `gates <verb>`. Lower-case kebab only,
// which is every verb the dispatcher has ever carried. See the head comment
// on why this is anchored on `exec` and the sibling's is not.
var bwsShimExec = regexp.MustCompile(`exec "\$[A-Za-z_][A-Za-z0-9_]*" gates ([a-z0-9][a-z0-9-]*)`)

// bwsDenyRule is the PID rule that makes RenderGates render a bd shim at
// all. Any bd rule would do — the write-verb tail is unconditional for
// cmd=="bd" (renderShim) and carries no rule of its own — and this one is in
// every crew PID, so the fixture is a spelling an operator's tree really
// renders.
const bwsDenyRule = "Bash(bd daemon:*)"

// bwsBdShim renders a persona's gates with posse's OWN renderer and returns
// the bd shim's bytes. The PATH is this box's own minus every gates dir
// (PathOutsideGates), the way bdpin_qa_test.go's render helper does it: a
// seat runs inside a persona pane, and resolving `bd` or `posse` back
// through a shim of our own would be reading the wall through the wall.
//
// Hermetic in the one way that matters here: the write-verb tail names
// `gates bd-write` whether or not posse is found at render time (when it is
// not, posse_bw_bin renders empty and the arm falls through to bd with a
// notice), so this clause reads the same verb on a box with no posse
// installed as on this one.
func bwsBdShim(t *testing.T) string {
	t.Helper()
	t.Setenv("PATH", posse.PathOutsideGates(""))
	app := &posse.App{StateDir: filepath.Join(t.TempDir(), "state")}
	_, binDir, _, err := app.RenderGates("bdwritesourcesqa", []string{bwsDenyRule})
	if err != nil {
		t.Fatalf("RenderGates: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(binDir, "bd"))
	if err != nil {
		t.Fatalf("no bd shim rendered: %v", err)
	}
	return string(b)
}

// bwsVerbsExecd returns every `gates <verb>` the rendered text execs.
func bwsVerbsExecd(body string) map[string]bool {
	out := map[string]bool{}
	for _, m := range bwsShimExec.FindAllStringSubmatch(body, -1) {
		out[m[1]] = true
	}
	return out
}

// bwsStem is the helper family an entry point names: its own name with
// cgsRunPrefix taken off, lower-cased. `RunGatesBdWrite` -> `bdwrite`, which
// is what bdWriteLock, bdWriteStore, bdWriteSplitArgv and ProjectBdWrite all
// carry and what nothing else in internal/posse does.
func bwsStem(entry string) string {
	return strings.ToLower(strings.TrimPrefix(entry, cgsRunPrefix))
}

// bwsFunc is one func or method declaration: the file it is in, and every
// name its body calls.
type bwsFunc struct {
	file    string
	callees []string
}

// bwsDecls parses every non-test Go file in internal/posse and returns, per
// func or method NAME, its declarations. Keyed by name and not by receiver,
// for wallrendersources_qa_test.go's reason: a method and a func sharing a
// name both count, which can only over-derive and never miss.
func bwsDecls(t *testing.T, root string) map[string][]bwsFunc {
	t.Helper()
	dir := filepath.Join(root, "internal", "posse")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", dir, err)
	}
	out := map[string][]bwsFunc{}
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
		rel := "internal/posse/" + name
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			fn := bwsFunc{file: rel}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch c := call.Fun.(type) {
				case *ast.Ident:
					fn.callees = append(fn.callees, c.Name)
				case *ast.SelectorExpr:
					fn.callees = append(fn.callees, c.Sel.Name)
				}
				return true
			})
			out[fd.Name.Name] = append(out[fd.Name.Name], fn)
		}
	}
	if seen < 50 {
		// The package is ~200 files. A handful means the walk found the
		// wrong directory, and every arm below would then pass over nothing.
		t.Fatalf("only %d non-test sources under %s — this pin is reading the wrong tree", seen, dir)
	}
	return out
}

// bwsClosure is the third clause: the files reached from entry through calls
// whose callee carries entry's own stem, entry's own file included. The
// value is the identifier that put the file in the class, so a failure can
// say WHY a path belongs rather than only that it does.
func bwsClosure(t *testing.T, root, entry string) map[string]string {
	t.Helper()
	stem := bwsStem(entry)
	if len(stem) < 4 {
		t.Fatalf("%s leaves the stem %q, which is too short to be a helper family — the third clause would close over half the package. Re-read the head comment before renaming an entry point to this.", entry, stem)
	}
	decls := bwsDecls(t, root)
	if len(decls[entry]) == 0 {
		t.Fatalf("no non-test file under internal/posse declares %s — the judge the bd shim execs lives outside the package this pin reads, so the write-time lag count cannot see it. Widen bwsDecls to the package that holds it.", entry)
	}
	out := map[string]string{}
	visited := map[string]bool{}
	queue := []string{entry}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if visited[name] {
			continue
		}
		visited[name] = true
		for _, d := range decls[name] {
			if _, ok := out[d.file]; !ok {
				out[d.file] = name
			}
			for _, c := range d.callees {
				if strings.Contains(strings.ToLower(c), stem) {
					queue = append(queue, c)
				}
			}
		}
	}
	return out
}

// bwsSources is the class: every non-test internal/posse file holding the
// judge behind a verb the rendered bd shim EXECS, or one of that judge's own
// stem-sharing helpers. Keyed by path, valued by "<verb> -> <identifier>".
func bwsSources(t *testing.T, root string) map[string]string {
	t.Helper()
	execd := bwsVerbsExecd(bwsBdShim(t))
	if len(execd) == 0 {
		t.Fatalf("the rendered bd shim execs no `gates <verb>` at all — either renderBdWriteTail stopped composing ADR 0073 D2's arm or %q stopped matching it; the class would derive EMPTY either way, and `posse gates` would read \"0 of them judging a write-time gate\" over a stale judge", bwsShimExec)
	}
	judges := cgsJudges(t, root)
	out := map[string]string{}
	for verb := range execd {
		fn, ok := judges[verb]
		if !ok {
			t.Errorf("the rendered bd shim execs `gates %s` and %s dispatches it to no %s* entry point — a judge the installed binary resolves at WRITE time that this pin cannot name, so the write-time lag count cannot see it. Widen the second clause deliberately.", verb, cgsDispatch, cgsRunPrefix)
			continue
		}
		for path, ident := range bwsClosure(t, root, fn) {
			out[path] = verb + " -> " + ident
		}
	}
	return out
}

// ARM 1: every derived write-time source is in BdWriteSources, so the count
// `posse gates` and the promote epilogue print cannot silently omit a
// write-time judge — the silence ranger-base-m1obc found for the commit-time
// row and left for this one, where 2a9f4e06 changed the bd write lock and no
// category named it.
func TestBdWriteSourcesNameEveryJudgeTheBdShimExecs(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, p := range posse.BdWriteSources {
		listed[p] = true
	}
	var missing []string
	for rel, why := range bwsSources(t, root) {
		if !listed[rel] {
			missing = append(missing, rel+" ("+why+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("the write-time judge is derived into %d file(s) posse.BdWriteSources does not name:\n  %s\nAdd them (wallrenderer.go), or — if that is a long list rather than a file or two — the third clause's stem has stopped being a helper family and has closed over the package, which is a finding about the derivation and not about the list.",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// ARM 2: every listed path is a real file. A pathspec that matches nothing
// makes the write-time count read zero over a repo where it should read
// more, and `git log -- <nothing>` says so to nobody. The same arm all three
// siblings carry, against a fourth list.
func TestBdWriteSourcesAllExist(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if len(posse.BdWriteSources) == 0 {
		t.Fatal("posse.BdWriteSources is empty")
	}
	for _, p := range posse.BdWriteSources {
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil || st.IsDir() {
			t.Errorf("posse.BdWriteSources names %s, which is not a file here (%v) — a pathspec that matches nothing renders as \"0 of them judging a write-time gate\"", p, err)
		}
	}
}

// ARM 3: the list holds nothing the derivation does not reach. The launch-
// check and commit-gate lists keep this arm and the wall list does not, and
// this one keeps it for their reason: all three clauses here are mechanical,
// so an entry none of them finds is an entry that will be wrong one refactor
// from now and reassure the operator while it is. A write-time judge the
// derivation cannot see belongs in a widened clause, named here, or in the
// FLOOR sentence BdWriteSources already carries — never in a hand-appended
// path.
func TestBdWriteSourcesHoldNothingTheDerivationCannotSee(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	derived := bwsSources(t, root)
	for _, p := range posse.BdWriteSources {
		if _, ok := derived[p]; ok {
			continue
		}
		t.Errorf("posse.BdWriteSources names %s and this pin's derivation does not reach it: it holds no %s* entry point for a verb the rendered bd shim execs, and nothing reaches it through that entry point's own stem family. A hand-held entry goes stale in the direction that prints reassurance, which is the whole reason this class is derived — widen a clause here, or drop the path and let the FLOOR sentence carry it.", p, cgsRunPrefix)
	}
}

// ARM 4: the derivation's own witness and its three boundaries.
//
// The witness, in the discipline wallrendersources_qa_test.go's arm 1 keeps:
// all three halves of ADR 0073 D2's judge must be DERIVED and not merely
// listed, each through the clause that is supposed to reach it. If this
// fails, the class has stopped seeing the files it exists for and arm 1
// would pass over a short set.
//
// The boundaries are what keep this row from quietly eating or being eaten
// by the row beside it:
//
//   - the COMMIT-guard hook body names no bd-write verb. If it ever does,
//     bd-write IS also a commit-time gate and the two rows need re-deciding
//     — not merging by accident, which is what the sibling's own arm 4 says
//     from the other side.
//
//   - the two lists share no path, and the commit row's judge is not derived
//     here. Both rows are install-gated through the same callback shape, and
//     the one thing that must not happen is a lost bead write being counted
//     under a cost sentence about refused commits.
//
//   - bdlineshim.go is NOT in the class, which is the first clause's `exec`
//     anchor made a test. The shim's rendered COMMENT names `gates bd-line`
//     and cmd/posse dispatches that verb to a RunGatesBdLine judge, so a
//     mention-and-dispatch first clause — the sibling's, widened to this
//     render — derives that file off a comment sentence, and would then
//     order a human to drop the path when the comment is reworded. If this
//     arm fails, read the head comment before changing the list.
func TestBdWriteDerivationWitnessAndBoundaries(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	const verb, entry = "bd-write", "RunGatesBdWrite"
	derived := bwsSources(t, root)
	for _, want := range []struct{ path, why string }{
		{"internal/posse/bdwriteshim.go", "ADR 0073 D2's entry point, reached by the second clause"},
		{"internal/posse/bdwritelock.go", "D1's per-store flock and the write/read classification, reached by the third clause — and the one file 2a9f4e06 changed alone"},
		{"internal/posse/bdline.go", "ADR 0072's projection the judge refuses on, reached by the third clause through ProjectBdWrite"},
	} {
		if _, ok := derived[want.path]; !ok {
			t.Errorf("the derivation does not reach %s (%s); it derived %v — a class that misses a half of this judge leaves arm 1 passing over a short set", want.path, want.why, derived)
		}
	}

	// The shim execs the verb AND the dispatch sends it to the entry point:
	// asserted apart, because each half fails for its own reason and a
	// combined check cannot say which half went.
	if !bwsVerbsExecd(bwsBdShim(t))[verb] {
		t.Errorf("the rendered bd shim no longer execs `gates %s` — ADR 0073 D2's arm is not in the shim posse installs, so there is no write-time gate to be behind on and this whole class is moot; delete it deliberately rather than letting it derive empty", verb)
	}
	if got := cgsJudges(t, root)[verb]; got != entry {
		t.Errorf("%s dispatches `posse gates %s` to %q, not %s — the second clause cannot name the judge, so the class derives empty and the reading reads zero over a stale one", cgsDispatch, verb, got, entry)
	}

	// BOUNDARY 1: the commit-time hook body is not a bd-write invoker.
	if cgsVerbsNamed(cgsCommitHookBody())[verb] {
		t.Errorf("the rendered commit-guard hook body names `posse gates %s`. This row exists because its decision is taken at WRITE time, its consequence is a lost or duplicated bead write and its invoking render is the bd shim (ranger-base-7fjmv, from ranger-base-m1obc done-when 4). If a commit really does pipe to it now, decide the rows again — do not let the two merge because a render grew an arm.", verb)
	}

	// BOUNDARY 2: the commit-time row stays the commit-time row.
	for _, p := range posse.CommitGateSources {
		if _, ok := derived[p]; ok {
			t.Errorf("%s is in posse.CommitGateSources and this pin derives it too — a commit to it would be counted under both cost sentences, and one of them says a commit was refused when what was lost is a bead write. The two rows are allowed to overlap in a file that really holds both judges; if that is what happened, say so here deliberately.", p)
		}
		for _, q := range posse.BdWriteSources {
			if p == q {
				t.Errorf("%s is in both posse.CommitGateSources and posse.BdWriteSources — see above", p)
			}
		}
	}

	// BOUNDARY 3: the `exec` anchor, held as a claim about the class.
	const mentionOnly = "internal/posse/bdlineshim.go"
	if why, ok := derived[mentionOnly]; ok {
		t.Errorf("%s derived into the write-time class (%s). The bd shim MENTIONS `gates bd-line` in its own comment about the TOCTOU ADR 0073 D2 closed, and %s dispatches that verb to a %s* judge, so a first clause that reads mentions rather than what the shim EXECS derives this file off a comment sentence — and reds the day the comment is reworded. That file's refusal wording and gate log are a FLOOR residual (BdWriteSources' own comment), not a member. If the live judge really execs bd-line again, widen the first clause deliberately and say so.", mentionOnly, why, cgsDispatch, cgsRunPrefix)
	}
}
