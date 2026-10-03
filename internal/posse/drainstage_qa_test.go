//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-sqxo1, second occurrence: the drain reaches the pass's
// pre-routing epilogue.
//
// ranger-base-e9d9 made the GATHER read the loop's context, because a pass
// that was waiting on a session ignored two SIGTERMs and a SIGINT. This is
// the same sentence one stage earlier, and it is the second time the operator
// has had to reach for KILL.
//
// MEASURED 2026-10-02/03. Watch pid 49599 printed "── pass 7 · 23:56:42" and
// the reap sweep's two "kept" lines and then nothing at all — no land-sweep
// line, no verify-after filing, no launch-cap line, and not the "N prompt(s)
// in flight, gathering" the carried leg guaranteed. TERM, then three minutes,
// then INT, both apparently ignored; the box was shut down at 00:07
// (`last reboot`), so the pass held for at least 10m18s with its context
// already cancelled. The cancel HAD arrived — the carried leg's wait
// goroutine took gather's stop exit and printed "watch loop stopping — claim
// kept, not judged this pass", the last line in the log — so what ignored it
// was not the signal path and not the gather this time. It was the epilogue
// between the gather and the post-pass wait, which read ctx nowhere, and
// which is where a pass spends its forks: the reap sweep, the land sweep, the
// plan read, the credential read, verify-after, ci-watch, the lost-bead sweep
// and `bd ready` per dir, across four configured repos, on a box the guard
// clock had recorded at loadavg 68-95 all evening with 6.1G of swap.
//
// What the fix buys is a BOUND and not an absence: the stop is honoured within
// one stage instead of after all eight. stopHere's doc (dispatch.go) carries
// the residual and why closing it is a different change.
//
// Three arms:
//
//	honoured    a Run whose loop is already stopping returns at the first
//	            boundary, says so, and hires nobody.
//	control     the same fixture with a live context runs the pass and hires,
//	            so the arm above is about the stop and not about a fixture
//	            that could never have launched.
//	every stage the source order of the epilogue: each fork-bearing stage is
//	            followed by a stop check before the next one, and one stands
//	            between the last of them and the `bd ready` read. This is the
//	            arm that catches a NEW stage added without a check, which is
//	            how the gap existed in the first place.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// stageRig is a Run with one ready bead. `held` gives it a leg that will not
// come back inside this test's lifetime — drainLegMS, for its own reason: two
// orders of magnitude outside every window asserted here, so "the pass
// returned at a boundary" and "the pass waited something out" cannot be
// confused at any load — and the Refill shape that carries it, which is the
// only shape with a stop to honour at all. Without `held` it is the one-shot
// shape `posse dispatch` has always had: no window, a gather that drains to
// zero, and a nil stopCtx.
func stageRig(t *testing.T, held bool) (*Dispatcher, *syncBuf, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	out := &syncBuf{}
	d.Out = out
	writePersona(t, b.App, "ranger", "[go]")
	agentPerLaunch(t, fake)
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	if held {
		os.WriteFile(filepath.Join(fake, "prompt-delay-ms"), []byte(drainLegMS), 0o644)
		d.Refill = true
		d.GatherWindow = 20 * time.Millisecond
	}
	return d, out, fake
}

func TestQAStopIsHonouredInsideThePassEpilogue(t *testing.T) {
	t.Parallel()

	t.Run("a stopping loop returns at the first boundary and hires nobody", func(t *testing.T) {
		t.Parallel()
		d, out, _ := stageRig(t, true)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // the operator's SIGTERM, already delivered
		d.stopCtx = ctx

		n, err := d.Run("", "", 0)
		if err != nil {
			t.Fatalf("a stop is not an error: %v\n%s", err, out.String())
		}
		if n != 0 {
			t.Errorf("a pass that stopped at its first boundary dispatched %d", n)
		}
		s := out.String()
		if !strings.Contains(s, "stop honoured after the reap sweep") {
			t.Errorf("the stop was not honoured at the first boundary, or not said:\n%s", s)
		}
		// The whole point: nothing downstream of the boundary ran. A session
		// created, claimed and prompted by a process on its way out is a
		// launch with nobody left to gather it.
		for _, never := range []string{"creating session", "in flight, gathering", "next pass in"} {
			if strings.Contains(s, never) {
				t.Errorf("a stopping pass reached %q:\n%s", never, s)
			}
		}
	})

	t.Run("control: the same fixture with a live context hires and gathers", func(t *testing.T) {
		t.Parallel()
		d, out, fake := stageRig(t, true)
		ctx, cancel := context.WithCancel(context.Background())
		d.stopCtx = ctx
		t.Cleanup(func() {
			cancel()
			// One persona, one ready bead: exactly one leg, abandoned in
			// flight by the window closing. The fake child is the last
			// writer left to join (drain_qa_test.go's note).
			joinHeldPrompts(t, fake, 1)
		})

		if _, err := d.Run("", "", 0); err != nil {
			t.Fatalf("the control pass failed: %v\n%s", err, out.String())
		}
		s := out.String()
		if strings.Contains(s, "stop honoured") {
			t.Fatalf("the control arm's context is live; nothing may be honoured:\n%s", s)
		}
		if !strings.Contains(s, "creating session") || !strings.Contains(s, "in flight, gathering") {
			t.Errorf("the fixture must be able to HIRE, or the stopped arm above measures a dead fixture:\n%s", s)
		}
	})

	// The one-shot shape, which is every `posse dispatch` an operator types
	// and every direct Run in this package: stopCtx is nil, so there is no
	// loop to stop and every boundary above must be invisible. "No loop to
	// stop" is never "interrupt me" — stopping()'s own rule, and the only
	// direction a seven-way early return may fail in.
	//
	// MUTATION: `stopping()` → `d.stopCtx == nil || d.stopCtx.Err() != nil`
	// → this arm reds at the first boundary, and nothing else in this file
	// does, because both arms above set a context.
	t.Run("a one-shot Run has no stop to honour", func(t *testing.T) {
		t.Parallel()
		d, out, _ := stageRig(t, false)
		if d.stopCtx != nil {
			t.Fatal("premise: a one-shot Run carries no context — that is what makes it one-shot")
		}

		if _, err := d.Run("", "", 0); err != nil {
			t.Fatalf("the one-shot pass failed: %v\n%s", err, out.String())
		}
		s := out.String()
		if strings.Contains(s, "stop honoured") {
			t.Errorf("a Run with no loop behind it honoured a stop nobody asked for:\n%s", s)
		}
		if !strings.Contains(s, "creating session") {
			t.Errorf("the one-shot pass must run all the way to a hire, or this arm cannot see a spurious stop:\n%s", s)
		}
	})
}

// The epilogue's stages, in the order Run runs them. Each is a stage that
// forks — herdr, git, bd or the network — and each is therefore a stage the
// stop has to be readable after.
//
// autoReapPass appears three times in Run; only the beforeRouting call is in
// the epilogue, which is why this sweep keys on the ARGUMENT and not the name.
var epilogueStages = []string{
	"autoReapPass",
	"landClosedTrees",
	"credentialExpiry",
	"VerifyAfter",
	"CIWatch",
	"WarnLostBeads",
	// The queue read is a stage like the rest — a bd child per configured
	// dir — and it is the last one before anything is spent, which is why
	// the boundary after it is the one that keeps a stopping loop from
	// hiring. "ReadyAll" is the loop's own call; "Ready" is the --dir arm
	// beside it, and either may come first in source order.
	"ReadyAll",
}

// What ends the epilogue: the fire loop, which creates a session, claims a
// bead and prompts it. Everything above must have a stop check between it and
// this call.
var epilogueEnd = []string{"fireLoop"}

// Every fork-bearing stage of Run's epilogue is followed by a stop check
// before the next stage begins — and one stands between the last of them and
// the `bd ready` read, because the fire loop behind it creates sessions.
//
// A source-order sweep rather than eight fixtures, for the reason the clock
// sweep in watchhang_qa_test.go gives: the defect is a MISSING check, so what
// has to be pinned is the shape of the sequence and not the behaviour of any
// one stage. A fixture per stage would also be eight fixtures that each have
// to get the box into that stage, which is how a pin ends up measuring its own
// rig.
//
// MUTATION: delete any one `d.stopHere(...)` from Run → this names the gap it
// leaves. Delete them all → it names the first.
func TestQAEveryEpilogueStageIsFollowedByAStopCheck(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "dispatch.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "Run" && fn.Recv != nil {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("Dispatcher.Run is gone from dispatch.go; this sweep is not reading the function it thinks it is")
	}

	stageAt := map[string]int{}
	var stops []int
	endAt := 0
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name, pos := sel.Sel.Name, int(sel.Pos())
		if name == "stopHere" {
			stops = append(stops, pos)
			return true
		}
		if name == "autoReapPass" {
			// Only the epilogue's call; the two past routing are not it.
			if len(call.Args) != 1 {
				return true
			}
			if id, ok := call.Args[0].(*ast.Ident); !ok || id.Name != "beforeRouting" {
				return true
			}
		}
		for _, s := range epilogueStages {
			if s == name {
				if _, seen := stageAt[name]; !seen {
					stageAt[name] = pos
				}
			}
		}
		for _, e := range epilogueEnd {
			if e == name && (endAt == 0 || pos < endAt) {
				endAt = pos
			}
		}
		return true
	})

	for _, s := range epilogueStages {
		if _, ok := stageAt[s]; !ok {
			t.Fatalf("Run no longer calls %s; the stage list in this pin is stale and the sweep is measuring nothing", s)
		}
	}
	if endAt == 0 {
		t.Fatal("Run no longer reads bd ready; this sweep cannot find where the epilogue ends")
	}
	if len(stops) == 0 {
		t.Fatalf("Run takes no stop check at all — the operator's TERM is honoured only after the whole epilogue, " +
			"which is the second occurrence on this bead (at least 10m18s, TERM and INT both apparently ignored)")
	}
	sort.Ints(stops)

	// The boundary after each stage is the window up to the NEXT stage, and
	// for the last stage up to the ready read.
	bounds := append([]int(nil), stops...)
	for i, s := range epilogueStages {
		from := stageAt[s]
		to := endAt
		if i+1 < len(epilogueStages) {
			to = stageAt[epilogueStages[i+1]]
		}
		found := false
		for _, at := range bounds {
			if at > from && at < to {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no stop check between %s (%s) and the next stage: a TERM delivered inside %s is not readable "+
				"until the stage after it finishes, and every stage here forks per configured repo (ranger-base-sqxo1)",
				s, fset.Position(token.Pos(stageAt[s])), s)
		}
	}
	// No separate "before any hire" assertion: the last stage above IS the
	// queue read and the end marker IS fireLoop, so that claim is the last
	// iteration of the loop above rather than a second check in the same
	// window. It was a second check once, over a window with no fork in it,
	// and deleting it reddened nothing — which is what moved the boundary in
	// dispatch.go to the far side of the read.
	t.Logf("%d epilogue stage(s), %d stop check(s) in Run", len(epilogueStages), len(stops))
}
