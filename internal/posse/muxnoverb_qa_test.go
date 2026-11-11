//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-l32dl — a §4 reading that is unavailable BY DESIGN
// does not print as a herdr that failed.
//
// WHAT WAS OBSERVED, on the ADR 0080 host (ranger-base-yk5qb). A
// `multiplexer: rex` instance's dispatch log carried, per seat:
//
//	herdr cannot explain <seat> (agent explain is herdr's own verb and this
//	instance's multiplexer is not herdr — the reading is not available
//	(ADR 0076 §4)) — …
//
// The INNER sentence is muxNotHerdr's and was never wrong. The OUTER one was
// wrong twice: it named herdr as the party that failed on a box where herdr
// is not the multiplexer, and it was shaped like news for a reading that is
// unavailable on every launch by design — so it fired once per seat, on every
// pass, forever, and a reader who learns to skip it has learned to skip the
// line a herdr that really failed prints too.
//
// WHAT THESE PINS HOLD, and the thing that is easy to lose:
//
//  1. the two causes render as two sentences (arm 1), and the failure one is
//     unchanged word for word — the §4 one is additive, not a rewrite.
//  2. THE SENTENCE HAS ONE HOME (arm 2). Three call sites print this, each
//     with the error already in hand, and each one spelled the line itself.
//     A fourth site tomorrow would spell it the old way and nothing would
//     say so, which is how the first three came to disagree with the box.
//     Mechanical, over this package's own non-test sources.
//  3. all three sites reach it (arms 3-5), asserted through the real ladders
//     against a real non-herdr occupant rather than through the renderer —
//     a pin that called explainDegrade directly would prove nothing about
//     who reaches it.
//  4. THE DEGRADE DID NOT MOVE. ADR 0076 §4's reading is "not available" and
//     never a "no", so each arm below asserts the concession still happens:
//     the gather goes on, the prompt goes out, the gate stands down. That is
//     the half a wording bead can break without anybody noticing until a
//     Rex instance stops dispatching.
//
// The herdr-FAILURE end of arms 3-5 is pinned where it always was and in the
// arms it always ran in — bootrace_test.go:116 and bootrace_qa_test.go:96
// (arm 2, dispatch), pulsepromptready_test.go:162 (arm 2, the pulse),
// promptready_test.go:196 (arm 3, the gate) — all three over the fake
// herdr's `explain-error` lever, all three still asserting "cannot explain".
// Nothing here duplicates them; arm 1 holds that wording as a value, and arm
// 2 is what keeps those three assertions pointed at this one renderer.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ─── arm 1: two causes, two sentences ────────────────────────────────────────

func TestQAAVerbThisMultiplexerDoesNotHaveIsNotAHerdrThatFailed(t *testing.T) {
	t.Parallel()

	// The §4 cause. It is the error muxNotHerdr answers with, reached through
	// the door a call site really calls, so the pin cannot pass over a
	// backend that stopped producing it.
	b, _ := newTestBackend(t)
	b.Slot = rexSeatWithRuntime(t)
	_, err := b.explain(Seat{Session: rexTestSession, Pane: rexTestBlock})
	if err == nil {
		t.Fatal("the herdr-only door answered on a non-herdr occupant — §4's whole premise")
	}
	verb, missing := muxVerbMissing(err)
	if !missing || verb != "agent explain" {
		t.Fatalf("muxVerbMissing(%v) = (%q, %v), want (\"agent explain\", true) — the verb is the subject of the sentence", err, verb, missing)
	}
	line := explainDegrade("ranger-qa-a-1", err, "gathering anyway")
	for _, want := range []string{"this instance's multiplexer has no agent explain", "ADR 0076 §4", "gathering anyway"} {
		if !strings.Contains(line, want) {
			t.Errorf("the §4 line must name %q — the rule is what tells a reader this is the design and not an outage:\n%s", want, line)
		}
	}
	// The two false claims, asserted as absences because that is what was
	// wrong with the old line and nothing else about it was.
	for _, never := range []string{"cannot explain", "herdr cannot"} {
		if strings.Contains(line, never) {
			t.Errorf("the §4 line still says %q — herdr is not the multiplexer here and nothing failed:\n%s", never, line)
		}
	}

	// The FAILURE cause, unchanged word for word. This is the sentence the
	// three end-to-end pins named in the head comment assert, so a change
	// here is a change to them.
	failed := Die("agent_not_found: no detection for w1:p1")
	got := explainDegrade("ranger-qa-a-1", failed, "prompting on its \"idle\" anyway")
	want := "herdr cannot explain ranger-qa-a-1 (agent_not_found: no detection for w1:p1) — prompting on its \"idle\" anyway"
	if got != want {
		t.Errorf("the failure wording moved:\n got %s\nwant %s", got, want)
	}

	// And the discriminator is the error VALUE, not its text: a §4 answer
	// wrapped on its way up is still one, and an unrelated error whose text
	// happens to quote §4 is not. A string compare would have both backwards.
	if _, ok := muxVerbMissing(fmt.Errorf("gathering %s: %w", "ranger-qa-a-1", err)); !ok {
		t.Error("a wrapped §4 answer read as a verb that failed — errors.As is the whole reason this is a type")
	}
	if _, ok := muxVerbMissing(Die("herdr: %s", err.Error())); ok {
		t.Error("an error that merely QUOTES §4's sentence read as a §4 answer — that is a string compare, and herdr's own errors can carry that text")
	}
}

// ─── arm 2: the sentence has one home ────────────────────────────────────────

// TestQATheExplainDegradeSentenceHasOneHome is the arm that makes "one
// renderer" a property of the package rather than of this bead's diff.
//
// The claim is narrow and mechanical: no non-test file in this package may
// SPELL "cannot explain" except the renderer's own. Three sites spelled it,
// each with the error in hand and none with the distinction, and a fourth
// would be written the same way — the §4 cause is invisible from a call site
// that is only formatting a string it was handed.
//
// Comments are invisible to the scan (it is an ast walk over string
// literals), which is why this file and the renderer may discuss the old
// wording at length while no code may print it.
func TestQATheExplainDegradeSentenceHasOneHome(t *testing.T) {
	t.Parallel()
	const phrase = "cannot explain"
	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var spells []string
	seen := map[string]bool{}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if v, err := strconv.Unquote(lit.Value); err == nil && strings.Contains(v, phrase) && !seen[name] {
				seen[name], spells = true, append(spells, name)
			}
			return true
		})
	}
	sort.Strings(spells)
	if len(spells) != 1 || spells[0] != "muxherdr.go" {
		t.Errorf("%q is spelled in %v, want only muxherdr.go — a second speller is a site with no way to tell a herdr that failed from a multiplexer that has no such verb (ranger-base-l32dl)",
			phrase, spells)
	}
}

// ─── arms 3-5: the three sites, on a real non-herdr occupant ─────────────────

// noVerbSeat is the fixture the three ladders run against: this instance's
// multiplexer is Rex, one seat holds a runtime, and its status queue answers
// `states` one call at a time.
//
// A REAL OCCUPANT and not a stub: the §4 door is a type assertion on
// whatever is in the slot (HerdrBackend.herdrMux), so a stub that happened
// not to be HerdrMux would shut the door for a reason the production box does
// not have. Rex is the occupant the incident was observed on.
func noVerbSeat(t *testing.T, b *HerdrBackend, session string, states ...string) Seat {
	t.Helper()
	m, _ := newTestRexMux(t)
	b.Slot = m
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: b.App.WorkspaceLabel(session),
		Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Label: "shell", Flavor: RexTerminalFlavor}}})
	rexSeedProcess(t, rexTestBlock, rexRuntimeProc())
	// One record per call, and the LAST one repeats: a one-element queue is a
	// constant answer (fakerex_test.go), which is what a seat that stays in
	// one state looks like.
	for _, st := range states {
		rexSeedStatus(t, rexTestBlock, []RexStatusRecord{rexRuntimeRec(st)})
	}
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuntime(t, b.App, "fastcli", "command: fastcli --sys {file}\nstartup_wait: 400ms\n")
	if err := b.writeMeta(&HerdrMeta{Name: session, Workspace: rexTestSession, Pane: rexTestBlock,
		Agent: "ranger", Runtime: "fastcli", Mux: MuxRex, Socket: m.Server().Addr, Launched: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return Seat{Session: rexTestSession, Pane: rexTestBlock}
}

// Arm 3 — the readiness gate (promptready.go). The gate stands down and says
// so, and what it says is the §4 sentence.
func TestQAThePromptGateStandsDownNamingTheRuleAndNotHerdr(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	target := noVerbSeat(t, b, "ranger-qa-a-1", MuxIdle)

	det, note, err := b.AwaitPromptable("ranger-qa-a-1", target)
	// The degrade, unmoved: a reading nobody could take is never a refusal.
	if err != nil {
		t.Fatalf("the gate REFUSED over a verb this multiplexer does not have — §4 says not available, never no: %v", err)
	}
	if det.Seen() {
		t.Errorf("the concession must hand back the zero detection — a diagnostic that is not there is evidence of no state at all: %+v", det)
	}
	for _, want := range []string{"this instance's multiplexer has no agent explain", "ADR 0076 §4", "prompting without the readiness gate"} {
		if !strings.Contains(note, want) {
			t.Errorf("the gate's note must name %q:\n%s", want, note)
		}
	}
	if strings.Contains(note, "cannot explain") {
		t.Errorf("the gate still reports a herdr that failed on a box where herdr is not the multiplexer:\n%s", note)
	}
}

// Arm 4 — the settle ladder (dispatch.go's awaitSettled). `agent wait`'s
// occupant answered idle; the explain that would have confirmed it is a verb
// this multiplexer does not have, so the ladder prompts on the state it has
// and names the rule it is standing on.
func TestQATheSettleLadderConcedesNamingTheRuleAndNotHerdr(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	target := noVerbSeat(t, b, "ranger-qa-a-1", MuxIdle)
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Poll = 10 * time.Millisecond
	wait := 150 * time.Millisecond

	status, det, err := d.awaitSettled("ranger-base-l32dl", "ranger-qa-a-1", target,
		[]string{MuxIdle, MuxDone, MuxBlocked}, time.Now().Add(wait), wait)
	// The degrade, unmoved.
	if err != nil || status != MuxIdle {
		t.Fatalf("the ladder refused the launch over a verb this multiplexer does not have: %q %v\n%s", status, err, out.String())
	}
	if det.Seen() {
		t.Errorf("the concession carries no screen evidence, because no screen was read: %+v", det)
	}
	line := out.String()
	for _, want := range []string{"ranger-base-l32dl", "this instance's multiplexer has no agent explain", "ADR 0076 §4", `prompting on its "idle" anyway`} {
		if !strings.Contains(line, want) {
			t.Errorf("the ladder's line must name %q:\n%s", want, line)
		}
	}
	if strings.Contains(line, "cannot explain") {
		t.Errorf("the ladder still reports a herdr that failed:\n%s", line)
	}
}

// Arm 5 — the delivered wait (dispatch.go's awaitDelivered). This is the one
// the incident's log was full of: it runs per seat on the gather path, so on
// a Rex instance it is the line that printed forever.
func TestQATheDeliveredWaitConcedesNamingTheRuleAndNotHerdr(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	noVerbSeat(t, b, "ranger-qa-a-1", MuxIdle)
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Poll = 10 * time.Millisecond

	target, seen, err := d.awaitDelivered("ranger-base-l32dl", "ranger-qa-a-1", "fastcli", 150*time.Millisecond)
	// The degrade, unmoved: it takes the occupant at its word that an agent
	// is there and gathers.
	// BOTH HALVES of the seat (ranger-base-k0hd6): awaitDelivered hands back
	// what AgentTarget resolved, and that is a Seat the occupant can address
	// without resolving anything of its own.
	if want := (Seat{Session: rexTestSession, Pane: rexTestBlock}); err != nil || target != want || !seen {
		t.Fatalf("the delivered wait failed the bead over a verb this multiplexer does not have: %+v seen=%v %v\n%s", target, seen, err, out.String())
	}
	line := out.String()
	for _, want := range []string{"ranger-base-l32dl", "this instance's multiplexer has no agent explain", "ADR 0076 §4", "the work prompt is on its launch line; gathering anyway"} {
		if !strings.Contains(line, want) {
			t.Errorf("the delivered wait's line must name %q:\n%s", want, line)
		}
	}
	if strings.Contains(line, "cannot explain") {
		t.Errorf("the delivered wait still reports a herdr that failed:\n%s", line)
	}
}
