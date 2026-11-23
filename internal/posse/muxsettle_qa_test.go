//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-25otk — the settle gate opens on the settle its own
// occupant handed it (ADR 0076 Amendment 12).
//
// WHAT WAS DROPPED. `awaitSettled` read `res.State` off `Mux().Wait` and not
// `res.Seen`. On a §4-missing instance the confirming `agent explain` can
// never answer, so the ladder spent the runtime's whole `startup_wait:` and
// then conceded on whichever reading was LAST — 857 launches of 857 on the
// live fleet, and five abandoned outright by the occupant's own `Wait`
// timeout, two of which had been reading settled for most of the window they
// were abandoned in (muxsettle.go carries the census, the dates and the risk
// argument; docs/notes.d/ranger-base-25otk.md carries the readings).
//
// WHAT THESE ARMS HOLD, and the two that are easy to lose:
//
//  1. the RULE is both halves — settled AND seen (arm 1). A predicate that
//     read only the evidence half would open this gate on `working`, which is
//     the race the gate exists for (rangerhq-3hb5).
//  2. the gate returns on the FIRST settled reading and spends no window
//     (arm 2). The accept is worth nothing as a correctness claim if the
//     ladder still sits out the deadline before honouring it, and the whole
//     cost being paid was the sitting.
//  3. THE MEASURED ABANDONMENT CANNOT HAPPEN AGAIN (arm 3). A seat that
//     reports settled and then goes `working` is the shape of
//     ranger-base-72z36 and ranger-base-k0hd6, whose launches were thrown
//     away 16.4s and 2.2s after they were promptable.
//  4. `blocked` IS UNMOVED (arm 4). It is a settled state in `until` and it
//     is still handed back by name for the caller to refuse, evidence or no
//     evidence — the accept arm may never swallow a blocker.
//  5. HERDR CANNOT REACH THE ARM (arm 5), and that is the whole blast radius.
//     An `agent wait` envelope reports a state and never what recognized it,
//     so `waitReading` leaves Seen false and every herdr launch takes the
//     ladder it always took.
//  6. the licensing field has ONE HOME (arm 6), mechanically, over this
//     package's own non-test sources. A `Seen()` that can open a gate may
//     only come from the occupant's own answer to §2's verb.
//
// ARMS 7-9 ARE ranger-base-q5fee's, the same field at the other two ladders
// (ADR 0076 Amendment 13). The readiness gate and the delivered wait took the
// §4 concession with no occupant reading at all, and the verb that answers
// them — `Mux().State` — is one of §2's ten rather than a §4 one, so it
// answers on every occupant:
//
//  7. the readiness half is the EVIDENCE ALONE (arm 7), which is not a
//     loosening of arm 1 but the other gate's own rule: it opens on a CLI
//     that holds the keyboard whatever the state, `working` included, and
//     arm 1's own table is what keeps the settle gate from doing the same.
//  8. THE DOOR IS §4's "NOT AVAILABLE" and not "the explain errored" (arm 8).
//     `HerdrMux.State` IS `agent explain` flattened, so on herdr the read
//     would be a retry of the call that just failed — asserted as a call that
//     is never made, over the occupant's own call log.
//  9. HERDR CANNOT REACH IT (arm 9), and it is the same blast radius as arm 5
//     reached by the other route: herdr's §4 door never answers "unavailable"
//     at all, so no herdr ladder takes the arm whatever its explain did.

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── arm 1: the rule is both halves ──────────────────────────────────────────

func TestQATheSettleAcceptAsksForASettledStateAndEvidenceBoth(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		read MuxReading
		want bool
	}{
		{"idle with evidence", MuxReading{State: MuxIdle, Seen: true}, true},
		{"done with evidence", MuxReading{State: MuxDone, Seen: true}, true},
		// The race this gate exists for. `working` is not a settle however
		// well it is evidenced, and `blocked` is the caller's to refuse by
		// name one arm earlier (arm 4).
		{"working with evidence", MuxReading{State: MuxWorking, Seen: true}, false},
		{"blocked with evidence", MuxReading{State: MuxBlocked, Seen: true}, false},
		{"unknown with evidence", MuxReading{State: MuxUnknown, Seen: true}, false},
		{"no state at all, with evidence", MuxReading{Seen: true}, false},
		// The other half: a settled state nothing says the runtime is behind
		// is herdr's idle-fallback guess in the slot's vocabulary, and it is
		// what ADR 0013 §1's settle row refuses.
		{"idle without evidence", MuxReading{State: MuxIdle}, false},
		{"done without evidence", MuxReading{State: MuxDone}, false},
	} {
		det, ok := muxSettledGate(c.read)
		if ok != c.want {
			t.Errorf("%s: muxSettledGate = %v, want %v", c.name, ok, c.want)
		}
		if !ok {
			if det.State != "" || det.Seen() {
				t.Errorf("%s: a reading this arm cannot act on handed back a detection a caller could branch on: %+v", c.name, det)
			}
			continue
		}
		if det.State != c.read.State || !det.Seen() {
			t.Errorf("%s: the accepted detection must carry the occupant's state and read as seen: %+v", c.name, det)
		}
	}
}

// ─── arm 2: the first settled reading, and no window spent ───────────────────

// TestQATheSettleGateReturnsOnTheFirstSettleItIsHanded is the arm that prices
// the fix. Before it, this fixture — a seat whose occupant answers `idle` on
// every call — drove the ladder through its whole window at `d.Poll` and then
// conceded; the bound below is the assertion that it no longer does.
//
// THE BOUND IS A FRACTION OF THE WINDOW AND NOT A WALL-CLOCK FIGURE. What is
// being pinned is "one poll, not the deadline", and the fake occupant is a
// real subprocess per call, so a number tight enough to be interesting on an
// idle box would be a flake on a loaded one. Half the window separates the
// two behaviours by a factor of four at this fixture's poll.
func TestQATheSettleGateReturnsOnTheFirstSettleItIsHanded(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	target := noVerbSeat(t, b, "ranger-qa-a-1", MuxIdle)
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Poll = 250 * time.Millisecond // the production poll, not a test one
	const wait = 4 * time.Second

	start := time.Now()
	status, det, err := d.awaitSettled("ranger-base-25otk", "ranger-qa-a-1", target,
		[]string{MuxIdle, MuxDone, MuxBlocked}, start.Add(wait), wait)
	spent := time.Since(start)
	if err != nil || status != MuxIdle {
		t.Fatalf("the gate did not open on the settle it was handed: %q %v\n%s", status, err, out.String())
	}
	if !det.Seen() {
		t.Errorf("the gate opened on a reading that does not read as seen, so the record it types under cannot name what opened it: %+v", det)
	}
	if spent > wait/2 {
		t.Errorf("the gate spent %s of a %s window on a settle it had from the first poll — the whole cost of this bug was the sitting", spent, wait)
	}
	if line := out.String(); line != "" {
		t.Errorf("the accept path printed, and an opened gate is not one of ADR 0066 D1's consequences:\n%s", line)
	}
}

// ─── arm 3: the measured abandonment ─────────────────────────────────────────

// TestQAASeatThatSettlesThenWorksIsNotAbandoned is ranger-base-72z36's and
// ranger-base-k0hd6's shape, hermetically: the occupant answers settled once
// and `working` for the rest of the window.
//
// WHAT HAPPENED BEFORE. The first `Wait` answered `idle`, the ladder dropped
// the evidence and asked for a confirmation this instance's multiplexer has
// no verb for, slept its poll, and re-waited — and the second `Wait` found
// `working`, ran the rest of the window out and REFUSED (settletimeout.go).
// The launch was thrown away over a settle that had already been reported,
// and the live log says 16.446s and 2.203s of window were left when the last
// wait began.
func TestQAASeatThatSettlesThenWorksIsNotAbandoned(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	// The queue's last entry repeats (noVerbSeat): settled once, then
	// working forever.
	target := noVerbSeat(t, b, "ranger-qa-a-1", MuxIdle, MuxWorking)
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Poll = 250 * time.Millisecond
	const wait = 4 * time.Second

	status, det, err := d.awaitSettled("ranger-base-72z36", "ranger-qa-a-1", target,
		[]string{MuxIdle, MuxDone, MuxBlocked}, time.Now().Add(wait), wait)
	if err != nil {
		t.Fatalf("the launch was abandoned over a settle the occupant had already reported: %v\n%s", err, out.String())
	}
	if status != MuxIdle || !det.Seen() {
		t.Fatalf("the gate opened on something other than the reported settle: %q %+v", status, det)
	}
}

// ─── arm 4: a blocker is still a blocker ─────────────────────────────────────

// TestQAABlockedSettleIsStillHandedBackByName keeps the accept arm behind the
// blocker arm. `blocked` is one of the states dispatch asks `Wait` for, on
// purpose — so a blocker fails the launch immediately and by name instead of
// sitting out the whole window behind one (awaitAgent) — and on Rex it
// carries evidence like any other reported state. An accept arm that read the
// evidence before the state would swallow it.
func TestQAABlockedSettleIsStillHandedBackByName(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	target := noVerbSeat(t, b, "ranger-qa-a-1", MuxBlocked)
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Poll = 10 * time.Millisecond
	const wait = 200 * time.Millisecond

	status, _, err := d.awaitSettled("ranger-base-25otk", "ranger-qa-a-1", target,
		[]string{MuxIdle, MuxDone, MuxBlocked}, time.Now().Add(wait), wait)
	if err != nil {
		t.Fatalf("the blocker arm stopped handing the state back for the caller to refuse: %v\n%s", err, out.String())
	}
	if status != MuxBlocked {
		t.Errorf("a blocked reading came back as %q — the accept arm may never swallow a blocker", status)
	}
}

// ─── arm 5: herdr cannot reach the arm ───────────────────────────────────────

// TestQAAHerdrWaitEnvelopeCannotOpenTheAcceptArm is the blast radius, asserted
// through the real mapper rather than by reading muxherdr.go's comment: every
// envelope herdr's `agent wait` can answer maps to a reading with no evidence
// in it, so the accept arm is unreachable on that occupant and a herdr launch
// takes the ladder it always took.
//
// The envelopes are herdr's own two shapes — the `agent_prompted` wrapper and
// the bare status — in the four settled states and the one empty answer.
func TestQAAHerdrWaitEnvelopeCannotOpenTheAcceptArm(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{"type":"agent_prompted","agent":{"agent_status":"idle"}}`,
		`{"type":"agent_prompted","agent":{"agent_status":"done"}}`,
		`{"type":"agent_prompted","agent":{"agent_status":"working"}}`,
		`{"type":"agent_prompted","agent":{"agent_status":"blocked"}}`,
		`{"status":"idle"}`,
		`{"status":"done"}`,
		`{}`,
	} {
		read := waitReading(json.RawMessage(raw))
		if read.Seen {
			t.Errorf("waitReading(%s) carried evidence — an envelope reports a STATE and never what recognized it, and this field is what licenses a settle", raw)
		}
		if _, ok := muxSettledGate(read); ok {
			t.Errorf("a herdr wait envelope opened the accept arm (%s) — the arm's whole blast radius is occupants that answer §3's field", raw)
		}
	}
}

// ─── arm 6: the licensing field has one home ─────────────────────────────────

// TestQATheMuxSeenFieldHasOneHome makes "only the occupant's own answer can
// license a settle" a property of the package rather than of this bead's
// diff. `AgentDetection.MuxSeen` is an arm of `Seen()`, and `Seen()` is what
// the settle gate, the readiness gate, the delivered wait, the probe and the
// readings log all read — so a site that set this field for its own
// convenience would be licensing a keystroke from wherever it stood.
//
// Mechanical, over this package's own non-test sources, and an ast walk so
// that this file and muxsettle.go may discuss the field at length while no
// other code may set it. Reads are not the subject: the field is evidence
// only where it was written from a MuxReading, and writes are what that
// claim is about.
func TestQATheMuxSeenFieldHasOneHome(t *testing.T) {
	t.Parallel()
	const field = "MuxSeen"
	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var writers []string
	seen := map[string]bool{}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		note := func() {
			if !seen[name] {
				seen[name], writers = true, append(writers, name)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.KeyValueExpr:
				// `AgentDetection{… MuxSeen: …}`
				if id, ok := v.Key.(*ast.Ident); ok && id.Name == field {
					note()
				}
			case *ast.AssignStmt:
				// `det.MuxSeen = …`
				for _, lhs := range v.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == field {
						note()
					}
				}
			case *ast.Field:
				// the declaration itself, which is herdr.go's.
				for _, id := range v.Names {
					if id.Name == field {
						note()
					}
				}
			}
			return true
		})
	}
	want := map[string]bool{"herdr.go": true, "muxsettle.go": true}
	for _, w := range writers {
		if !want[w] {
			t.Errorf("%s writes AgentDetection.%s — a Seen() that can open the settle gate may only be set from the occupant's own Wait reading (muxSettledGate, ADR 0076 Amendment 12)", w, field)
		}
		delete(want, w)
	}
	for w := range want {
		t.Errorf("%s no longer declares or sets AgentDetection.%s — if the field moved, this pin's subject moved with it", w, field)
	}
}

// ─── arm 7: the readiness half is the evidence alone ─────────────────────────

// TestQATheSeenAcceptAsksForTheEvidenceAndNotTheState is arm 1's table over
// the other predicate. The two gates differ in exactly one clause and the
// difference is deliberate: `working` with evidence is a pane whose CLI has
// the keyboard (open the readiness gate) and is not a settle (refuse the
// settle gate), which is the whole of ADR 0076 Amendment 13's D1.
//
// A state this package does not know, and no state at all, are accepted here
// for the same reason: the question asked is about the keyboard, and the gates
// that care about the state ask it themselves. Both of this arm's callers
// deliberately do not — and what reads the state afterwards is the pulse's own
// idle|done rule, which refuses both of those readings on its own (arm 3a of
// muxnoverb_qa_test.go pins that expression over the gate's real output).
func TestQATheSeenAcceptAsksForTheEvidenceAndNotTheState(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		read MuxReading
		want bool
	}{
		{"idle with evidence", MuxReading{State: MuxIdle, Seen: true}, true},
		{"done with evidence", MuxReading{State: MuxDone, Seen: true}, true},
		// The clause the two gates do not share. muxSettledGate answers
		// false for these three (arm 1) and must keep doing so.
		{"working with evidence", MuxReading{State: MuxWorking, Seen: true}, true},
		{"blocked with evidence", MuxReading{State: MuxBlocked, Seen: true}, true},
		{"unknown with evidence", MuxReading{State: MuxUnknown, Seen: true}, true},
		{"no state at all, with evidence", MuxReading{Seen: true}, true},
		// The half both gates share, and the only one either may refuse on:
		// a state nothing says the runtime is behind.
		{"idle without evidence", MuxReading{State: MuxIdle}, false},
		{"working without evidence", MuxReading{State: MuxWorking}, false},
		{"nothing at all", MuxReading{}, false},
	} {
		det, ok := muxSeenGate(c.read)
		if ok != c.want {
			t.Errorf("%s: muxSeenGate = %v, want %v", c.name, ok, c.want)
		}
		if !ok {
			if det.State != "" || det.Seen() {
				t.Errorf("%s: a reading this arm cannot act on handed back a detection a caller could branch on: %+v", c.name, det)
			}
			continue
		}
		if det.State != c.read.State || !det.Seen() {
			t.Errorf("%s: the accepted detection must carry the occupant's state and read as seen: %+v", c.name, det)
		}
		// And the settle gate is unmoved by this file's refactor: it is the
		// same evidence half plus its own state half, so exactly the two
		// settled states pass it.
		_, settled := muxSettledGate(c.read)
		if want := c.read.State == MuxIdle || c.read.State == MuxDone; settled != want {
			t.Errorf("%s: muxSettledGate = %v, want %v — the two gates share the evidence half and nothing else", c.name, settled, want)
		}
	}
}

// ─── arm 8: the door is §4's "not available" ─────────────────────────────────

// TestQATheOccupantIsAskedOnlyWhereTheVerbIsUnavailable pins the door as a
// property of the ERROR value and, for the negative, as a call that is never
// made. `HerdrMux.State` is `agent explain` flattened — the same subprocess,
// the same answer — so a read taken because an explain FAILED would be a retry
// of that failure at the same instant: no second reading, one more call per
// poll, and a fake herdr's `explain-error-after N` / `explain-error-for N`
// countdowns (which count CALLS) silently off by one.
func TestQATheOccupantIsAskedOnlyWhereTheVerbIsUnavailable(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	b.Slot = rexSeatWithRuntime(t,
		[]RexStatusRecord{rexRuntimeRec(MuxIdle)},
		[]RexStatusRecord{rexRuntimeRec(MuxIdle)},
		[]RexStatusRecord{rexRuntimeRec(MuxIdle)})
	target := Seat{Session: rexTestSession, Pane: rexTestBlock}

	// The door open: §4's own answer, and the same answer wrapped on its way
	// up — errors.As and not a string compare, which is what muxVerbMissing
	// is a type for.
	for _, err := range []error{
		muxNotHerdr("agent explain"),
		fmt.Errorf("gathering %s: %w", "ranger-qa-a-1", muxNotHerdr("agent explain")),
	} {
		det, ok := b.muxSeen(target, err)
		if !ok || !det.Seen() || det.State != MuxIdle {
			t.Errorf("muxSeen over %v = (%+v, %v), want the occupant's own idle reading", err, det, ok)
		}
	}

	// The door shut, and no call made. A nil error is "the explain answered,
	// so this is not the ladder's §4 arm at all"; the rest are verbs that
	// were asked and failed.
	before := len(rexCalls(t))
	for _, err := range []error{
		nil,
		Die("agent_not_found: no detection for w1:p1"),
		HerdrAPIError{Code: "internal", Message: "fake herdr: gone"},
	} {
		if det, ok := b.muxSeen(target, err); ok {
			t.Errorf("muxSeen asked the occupant over %v and answered %+v — the door is the verb being UNAVAILABLE, not a verb that failed", err, det)
		}
	}
	if after := len(rexCalls(t)); after != before {
		t.Errorf("the occupant was called %d time(s) for an error that is not §4's — on herdr that call is a retry of the explain that just failed:\n%s",
			after-before, strings.Join(rexCalls(t)[before:], "\n"))
	}
}

// ─── arm 9: herdr cannot reach the arm ───────────────────────────────────────

// TestQAAHerdrExplainFailureIsNeverTheUnavailableAnswer is the blast radius,
// asserted through the real door rather than by reading a comment: with herdr
// in the slot, every way `b.explain` can fail is a verb that was ASKED, so
// muxVerbMissing answers false and the arm is unreachable on that occupant —
// the ladder, the grace and the concession are the ones it always had.
//
// Arm 5 makes the same claim about the settle gate from the other side (a
// herdr wait envelope carries no evidence to accept). Both are needed: that
// one is about the reading, this one is about the door.
func TestQAAHerdrExplainFailureIsNeverTheUnavailableAnswer(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	write(t, filepath.Join(fake, "explain-error"), "agent_explain_unavailable|fake herdr: gone")

	_, err := b.explain(Seat{Session: "w1", Pane: "p1"})
	if err == nil {
		t.Fatal("the fake herdr's explain-error lever did not fire — this arm asserts about a failure")
	}
	if verb, missing := muxVerbMissing(err); missing {
		t.Errorf("a herdr explain that FAILED read as %q being unavailable — the two causes are what ranger-base-l32dl split and what this arm's door reads", verb)
	}
	if det, ok := b.muxSeen(Seat{Session: "w1", Pane: "p1"}, err); ok {
		t.Errorf("the occupant arm fired on a herdr failure and answered %+v — on herdr State IS that same `agent explain`", det)
	}
}
