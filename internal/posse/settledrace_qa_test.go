//go:build posse_arm2

package posse

// ranger-base-9c5bh: the fire loop's settled skip, after ranger-base-eh1kr
// left it with no store-driven caller.
//
// ranger-base-bknod stopped the suite's fake bd from serving an in_progress
// row out of `bd ready`, which is what real bd has always done, and that
// turned the skip's reachability into a measurement: `holder` is non-empty
// only for an in_progress bead assigned to this persona; no store answers
// `ready` with one (MEASURED 2026-10-03, bd 0.50.3, both classes); the
// claimed half reaches the loop through interruptedRuns, which declines a
// settled holder itself unless `--resume` asked; and under `--resume` the
// branch's own `!d.Resume` is false. FOUR pins asserted its line and every
// one of them could only reach it through the old fixture, so the decision
// was the code lane's to take: the branch STAYS and says NOTHING.
//
// The two halves of that decision are pinned here — the race that is the one
// caller left, and the counted-reason table the line used to be tallied in.
// docs/notes.d/ranger-base-bknod.md has the per-test evidence.

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

// The one path left: a holder that settles BETWEEN the scan's heldSession
// walk and the loop's. interruptedRuns offers the bead because no session
// holds it under any of the three names; by the time the loop asks, a live
// session with a settled agent wears one — and on a pass with no `--resume`
// nothing may be typed into it (rangerhq-zom: re-prompting a persona that
// stopped on purpose is a token loop under --watch).
//
// Staged with the lever ranger-base-rrg2 built for exactly this window
// (`unhide-when-locked`, fakeHerdr): the holder is in the fake's world from
// the start, hidden from `workspace list` until the launcher lock is held.
// The scan runs before fireLoop takes that lock, so it reads a listing
// without the holder; everything from reconcileSeats down reads one with it.
//
// Two guards keep this from being a sticker, because the branch's whole
// output is silence and so is a pass that never reached it: the lever must
// have FIRED (a listing was taken under the lock, so the holder really did
// appear mid-pass), and the pass must not have said "no ready work" (the
// queue was not empty, so the bead really did reach the fire loop). The arm
// TestDispatchHeldBeadNotReprompted pins is the other one — the scan
// declining a settled holder before the loop ever sees it.
//
// MUTATION-CHECKED 2026-10-03: deleting the `holder != "" && holderStatus !=
// "" && !d.Resume` branch from fireLoop reds this on the re-prompt
// assertion (`agent prompt` typed into the holder's pane, n=1).
func TestQASettledHolderRacingTheScanIsSkipped(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := claimedRepo(t, b.App, `[]`,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	// Nothing may re-claim a bead this persona already holds; if anything
	// tries, the fake refuses loudly rather than letting it pass.
	if err := os.WriteFile(filepath.Join(repo, "fake-claim-fail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// The holder: the bead's own Dial F session, agent idle — the same
	// fixture TestDispatchHeldBeadNotReprompted carries. The race is the
	// only difference.
	session := SessionForBead("ranger", repo, "a-1")
	mustCreate(t, b, NewSessionOpts{Name: session, Dir: repo, Agent: "ranger"})
	idleClaude(t, fake)
	ws := metaOf(t, b, session).Workspace
	if ws == "" {
		t.Fatalf("the holder %s has no workspace to hide", session)
	}
	hideFromTheListing(t, fake, ws)
	if err := os.MkdirAll(filepath.Dir(LaunchLockPath(b.App)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "unhide-when-locked"),
		[]byte(LaunchLockPath(b.App)), 0o644); err != nil {
		t.Fatal(err)
	}
	// Everything the setup itself said to herdr is not this pass's doing.
	if err := os.Remove(filepath.Join(fake, "calls.log")); err != nil {
		t.Fatal(err)
	}

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out, log := dispatcherOut(d), calls(t, fake)

	if _, err := os.Stat(filepath.Join(fake, "unhide-when-locked")); !os.IsNotExist(err) {
		t.Fatalf("the lever never fired: no workspace listing was taken under the launcher lock, so no holder ever appeared under the scan and this pass measured nothing:\n%s", out)
	}
	if strings.Contains(out, "no ready work") {
		t.Fatalf("the queue was empty, so the bead never reached the fire loop and the branch under test was not the reason for the silence:\n%s", out)
	}
	if n != 0 {
		t.Errorf("a holder that settled under the scan must not be fired into on a pass with no --resume, got n=%d:\n%s", n, out)
	}
	if strings.Contains(log, "agent prompt") {
		t.Errorf("the settled holder was re-prompted on an unattended pass (rangerhq-zom's token loop):\n%s", log)
	}
	if strings.Contains(log, "workspace create") {
		t.Errorf("a twin was built beside the holder (ranger-base-6bu):\n%s", log)
	}
	if bd := bdCalls(t, fake); strings.Contains(bd, "update") {
		t.Errorf("the claim was touched by a pass that took no action on it:\n%s", bd)
	}
	// And it is SILENT: the line this branch used to print was a per-pass
	// report of what is now a race, and the surface that reports a holder
	// which stopped without closing is the governance surface's G2 row
	// (`settled:<bead>`, govern.go) — once per finding, not once per pass.
	for _, said := range []string{"stopped on purpose", "held, agent settled", "– a-1"} {
		if strings.Contains(out, said) {
			t.Errorf("the settled skip still reports %q per pass (ranger-base-9c5bh):\n%s", said, out)
		}
	}

	// The control: the same fixture, the same holder, `--resume`. The flag's
	// own population IS a settled holder, so the pass types into it — which
	// is what says the silence above was this branch's decision and not a
	// fixture nothing could ever have fired.
	d2 := newTestDispatcher(t, b)
	d2.Resume = true
	n2, err := d2.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out2 := dispatcherOut(d2)
	if n2 != 1 || !strings.Contains(out2, "resuming") {
		t.Errorf("--resume must re-prompt the settled holder, got n=%d:\n%s", n2, out2)
	}
	if !strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("--resume typed nothing into the holder:\n%s", calls(t, fake))
	}
}

// The other half of the decision: a refill counts a skip by REASON, and a
// reason in that table is a promise that some site reports it. `skipSettled`
// outlived its line — the settled skip stopped printing when eh1kr left it
// with no store-driven caller, and the table went on offering a tally
// nothing could add to, which is a summary line that can never be wrong
// because it can never be written.
//
// Read from the source rather than asserted through a refill, because that
// is the direction the rot runs: a reason with no caller produces no output
// to assert on. Parsed and not grepped — a `skipFoo` in a comment or in a
// test is not a site that reports it.
//
// MUTATION-CHECKED 2026-10-03: putting `skipSettled = "held, agent settled"`
// back in the table reds this; so does deleting any live reason's `d.skipf`
// call while leaving its constant standing.
func TestQAEveryCountedSkipReasonHasAReportingSite(t *testing.T) {
	t.Parallel()
	const table = "refillreport.go"
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = f
	}
	if files[table] == nil {
		t.Fatalf("%s is not in this package any more — this pin reads its const block", table)
	}

	// The table: every `skip*` constant declared in refillreport.go.
	declared := map[string]bool{}
	for _, decl := range files[table].Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, id := range vs.Names {
				if strings.HasPrefix(id.Name, "skip") {
					declared[id.Name] = true
				}
			}
		}
	}
	if len(declared) < 2 {
		t.Fatalf("read %d skip reasons out of %s — the const block moved and this pin is measuring nothing", len(declared), table)
	}

	// The sites: the first argument of every skipf/skipNf call in the
	// package. Those two are the whole of the reporting path — skipf is
	// skipNf for one bead, and skipNf is what counts into a refill.
	reported := map[string]bool{}
	for name, f := range files {
		if name == table {
			continue // the declarations themselves are not a report
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "skipf" && sel.Sel.Name != "skipNf") {
				return true
			}
			if id, ok := call.Args[0].(*ast.Ident); ok {
				reported[id.Name] = true
			}
			return true
		})
	}

	var orphans []string
	for name := range declared {
		if !reported[name] {
			orphans = append(orphans, name)
		}
	}
	sort.Strings(orphans)
	for _, name := range orphans {
		t.Errorf("%s names a counted skip reason no site reports: pass it to d.skipf/d.skipNf, or take it out of the table (ranger-base-9c5bh — `skipSettled` outlived its line by one bead)", name)
	}
}
