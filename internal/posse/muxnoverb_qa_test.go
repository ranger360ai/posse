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
//     SINCE ADR 0076 AMENDMENT 12 (ranger-base-25otk) one of the three
//     cannot be: the settle ladder now ACCEPTS a settled reading the
//     occupant has evidence for, and Rex answers no other kind, so arm 4b
//     reaches this renderer through an occupant that answers §2's verb with
//     a state and nothing else — the shape herdr's own envelope has and the
//     interface permits. Arm 4a is the accept, against the real occupant.
//     SINCE AMENDMENT 13 (ranger-base-q5fee) all three are split that way,
//     because all three now ask the occupant where §4's reading is not
//     available. 3a and 5a are the accepts; 3b drives the concession with a
//     reading the REAL occupant can answer and that is not evidence — a
//     record the block's own shell wrote (§3's second guard) — and 5b needs
//     the stand-in, for a reason that is a property of the occupant and is in
//     noEvidenceMux's own doc.
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
	// One record per call, and the LAST one repeats: a one-element queue is a
	// constant answer (fakerex_test.go), which is what a seat that stays in
	// one state looks like.
	recs := make([][]RexStatusRecord, 0, len(states))
	for _, st := range states {
		recs = append(recs, []RexStatusRecord{rexRuntimeRec(st)})
	}
	return noVerbSeatReading(t, b, session, rexRuntimeProc(), recs...)
}

// noVerbSeatAtShell is the same instance holding the one reading that is NOT
// one: the block's own SHELL wrote the record, which is ADR 0076 §3's second
// guard, so the occupant answers `unknown` with no evidence behind it
// (rexReading). It is what the three ladders' concessions are driven with
// since ranger-base-q5fee — on the REAL occupant and not a stand-in, because
// this is a shape the production box has: a seat whose runtime never started,
// or whose runtime has ended and left the shell at its prompt, which is
// ranger-base-3p0's own incident.
func noVerbSeatAtShell(t *testing.T, b *HerdrBackend, session string) Seat {
	t.Helper()
	return noVerbSeatReading(t, b, session, rexShellProc(),
		[]RexStatusRecord{{State: MuxIdle, App: "zsh", Owner: rexTestShellOwner(10)}})
}

func noVerbSeatReading(t *testing.T, b *HerdrBackend, session string, proc RexProcess, recs ...[]RexStatusRecord) Seat {
	t.Helper()
	m, _ := newTestRexMux(t)
	b.Slot = m
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: b.App.WorkspaceLabel(session),
		Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Label: "shell", Flavor: RexTerminalFlavor}}})
	rexSeedProcess(t, rexTestBlock, proc)
	rexSeedStatus(t, rexTestBlock, recs...)
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

// Arm 3 — the readiness gate (promptready.go), in the two halves ADR 0076
// Amendment 13 split it into (ranger-base-q5fee), for arm 4's reason one site
// over. The gate now ASKS the occupant where §4's reading is not available,
// so a seat whose runtime is reporting no longer reaches the concession at
// all; 3b drives it with the reading that is not one — a record the block's
// own shell wrote — which is a shape the real occupant has.
//
// 3a is the accept. The gate's rule is "a CLI that holds the keyboard" and
// NOT a settled state (promptready.go's "what this gate deliberately is
// not"), so it is asserted over `working`: the one state that tells this
// gate's predicate apart from muxSettledGate's, and the state the pulse's own
// rule then has to refuse.
func TestQAThePromptGateOpensOnTheOccupantsOwnReading(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	target := noVerbSeat(t, b, "ranger-qa-a-1", MuxWorking)

	start := time.Now()
	det, note, err := b.AwaitPromptable("ranger-qa-a-1", target)
	spent := time.Since(start)
	if err != nil {
		t.Fatalf("the gate refused a keyboard its own occupant reports a runtime on: %v", err)
	}
	if !det.Seen() || det.State != MuxWorking {
		t.Fatalf("the gate opened on something other than the occupant's reading: %+v", det)
	}
	// THE READING THE PULSE'S RULE NEEDS, in the pulse's own expression
	// (pulse.go: `det.Seen() && !pulseTakesPrompt(det.State)`). That rule is
	// what keeps a shop check off a persona mid-turn, and on a §4-missing
	// instance it has never once been able to fire — 518 concessions in the
	// measured window, every one of them the pulse, each handing it the zero
	// detection (muxsettle.go's census). The other half of this property —
	// that the pulse applies that rule to THIS gate's detection — is
	// TestPulseSkipsAWorkingScreenTheListingCalledIdle.
	if !(det.Seen() && !pulseTakesPrompt(det.State)) {
		t.Errorf("the pulse's own guard cannot fire over the gate's reading, so a shop check would land mid-turn: %+v", det)
	}
	// One poll and no grace spent: the concession's bound is what the gate
	// used to pay here on every prompt, and the occupant answers before the
	// first sleep.
	if spent >= promptExplainGrace {
		t.Errorf("the gate spent %s of its %s concession grace on a reading it had from the first poll", spent, promptExplainGrace)
	}
	if note != "" {
		t.Errorf("a gate that opened on the first poll has nothing to report: %q", note)
	}
}

// 3b — the concession, with the one reading the occupant can answer that is
// not evidence of anything. The §4 sentence is unmoved.
func TestQAThePromptGateStandsDownNamingTheRuleAndNotHerdr(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	target := noVerbSeatAtShell(t, b, "ranger-qa-a-1")

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

// Arm 4 — the settle ladder (dispatch.go's awaitSettled), in the two halves
// ADR 0076 Amendment 12 split it into (ranger-base-25otk).
//
// IT USED TO BE ONE ARM AND IT COULD NOT STAY ONE. This arm asserted that a
// real Rex occupant answering `idle` drove the ladder into the §4 concession
// — which it did, for the whole window, on every launch. Amendment 12 made
// that reading an ACCEPT: Rex's `Wait` only ever answers a settled state it
// has positive evidence for (`rexReading` maps a state in its `Seen` branch
// and nowhere else), so the production occupant can no longer produce the
// shape the degrade needs, and a fixture that claimed it does would be
// pinning a path the box does not have.
//
// So 4a asserts the ACCEPT against the real occupant — the degrade is not
// reached, and the ladder opens on the occupant's own evidence — and 4b keeps
// ranger-base-l32dl's own property, that this SITE still reaches the one
// renderer, against an occupant that can still get there: one that answers
// §2's verb with a state and no evidence, which the interface permits and
// which herdr's own envelope is ("Seen is never derived from State").
func TestQATheSettleLadderOpensOnTheOccupantsOwnSettle(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	target := noVerbSeat(t, b, "ranger-qa-a-1", MuxIdle)
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Poll = 10 * time.Millisecond
	wait := 150 * time.Millisecond

	status, det, err := d.awaitSettled("ranger-base-25otk", "ranger-qa-a-1", target,
		[]string{MuxIdle, MuxDone, MuxBlocked}, time.Now().Add(wait), wait)
	if err != nil || status != MuxIdle {
		t.Fatalf("the ladder refused a settle its own occupant reported: %q %v\n%s", status, err, out.String())
	}
	// The ACCEPT's evidence, which is the half that makes this a settle and
	// not the concession wearing one: the gate reading posse carries into a
	// stall record says it was seen (ADR 0066 D1 as amended, ranger-base-o1aoi).
	if !det.Seen() {
		t.Errorf("the gate opened on a reading that does not read as seen, so the record it types under cannot say what opened it: %+v", det)
	}
	if line := out.String(); strings.Contains(line, "prompting on its") || strings.Contains(line, "ADR 0076 §4") {
		t.Errorf("the ladder conceded over a settle it was handed — that is the bug, not the degrade:\n%s", line)
	}
}

// noEvidenceMux is 4b's and 5b's occupant: not herdr, not Rex, and its `Wait`
// and `State` both answer a settled state with no positive evidence behind it
// — the shape herdr's own wait envelope has ("Seen is never derived from
// State") and one ADR 0076 §2 permits. Every other verb is the embedded nil
// interface and is never called; the ladders' only other call is the §4 door,
// which decides by type assertion (noVerbMux's own note,
// runtimesession_qa_test.go).
//
// IT IS WHAT THE TWO CONCESSIONS NEED SINCE ranger-base-q5fee, and `List` is
// why 5b cannot use the real occupant the way 3b does. On Rex a no-evidence
// reading is not a listed seat at all — `RexMux.List` drops a block whose
// reading has no `Seen` ("the Rex form of the unknown MEANS NONE rule") — so
// `awaitTarget` fails first and `awaitDelivered`'s concession is unreachable
// on that occupant from either side: with evidence it takes the new
// observation arm, and without it there is no target to observe. Keeping the
// arm pinned therefore takes an occupant that can still get there, which is
// the same argument Amendment 12 made for 4b.
type noEvidenceMux struct{ Mux }

func (noEvidenceMux) Wait(Seat, []string, int) (MuxReading, error) {
	return MuxReading{State: MuxIdle}, nil
}

func (noEvidenceMux) State(Seat) (MuxReading, error) {
	return MuxReading{State: MuxIdle}, nil
}

// Server is ADR 0076 §5's triple, which is not a verb and is asked of every
// occupant before a listing may name a seat (listSessions): a record whose
// server this pass cannot name is withheld, so a stub without it is a stub
// whose seat no ladder can resolve.
func (noEvidenceMux) Server() MuxServer {
	return MuxServer{Name: MuxRex, Addr: noEvidenceAddr}
}

const noEvidenceAddr = "qa-no-evidence"

func (noEvidenceMux) List() ([]SeatInfo, error) {
	return []SeatInfo{{
		Seat:  Seat{Session: rexTestSession, Pane: rexTestBlock},
		State: MuxIdle,
		Panes: []SeatPane{{Pane: rexTestBlock, Kind: "claude-code", State: MuxIdle}},
	}}, nil
}

// noEvidenceSeat is that occupant in the slot, with the meta a ladder needs to
// resolve the seat by session name.
func noEvidenceSeat(t *testing.T, b *HerdrBackend, session string) Seat {
	t.Helper()
	b.Slot = noEvidenceMux{}
	if err := b.writeMeta(&HerdrMeta{Name: session, Workspace: rexTestSession, Pane: rexTestBlock,
		Agent: "ranger", Runtime: "fastcli", Mux: MuxRex, Socket: noEvidenceAddr, Launched: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return Seat{Session: rexTestSession, Pane: rexTestBlock}
}

func TestQATheSettleLadderConcedesNamingTheRuleAndNotHerdr(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	b.Slot = noEvidenceMux{}
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Poll = 10 * time.Millisecond
	wait := 150 * time.Millisecond

	status, det, err := d.awaitSettled("ranger-base-l32dl", "ranger-qa-a-1", Seat{Session: "s", Pane: "p"},
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

// Arm 5 — the delivered wait (dispatch.go's awaitDelivered), split the same
// way for the same reason (ranger-base-q5fee).
//
// 5a is the OBSERVATION. The question this site asks is "has a runtime been
// seen in this seat", any state, so the occupant's own reading answers it —
// and the arm returns silently, because a reading that answered is not news.
// MEASURED: this site's concession has fired 0 times on the live instance (no
// `prompt: argv` runtime has launched here), so unlike arm 3 this half is a
// shape closed before it was paid.
func TestQATheDeliveredWaitObservesTheOccupantsOwnReading(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	noVerbSeat(t, b, "ranger-qa-a-1", MuxIdle)
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Poll = 10 * time.Millisecond

	target, seen, err := d.awaitDelivered("ranger-base-q5fee", "ranger-qa-a-1", "fastcli", 150*time.Millisecond)
	if want := (Seat{Session: rexTestSession, Pane: rexTestBlock}); err != nil || target != want || !seen {
		t.Fatalf("the delivered wait did not observe the occupant's own reading: %+v seen=%v %v\n%s", target, seen, err, out.String())
	}
	if line := out.String(); line != "" {
		t.Errorf("the observation printed — it is a reading that answered, not a concession:\n%s", line)
	}
}

// 5b — the concession. This is the one the incident's log was full of: it
// runs per seat on the gather path, so on an instance with no reading to take
// it is the line that printed forever. Driven through noEvidenceMux, whose
// own doc says why the real occupant can no longer reach it.
func TestQATheDeliveredWaitConcedesNamingTheRuleAndNotHerdr(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	noEvidenceSeat(t, b, "ranger-qa-a-1")
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
