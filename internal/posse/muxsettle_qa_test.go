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

import (
	"encoding/json"
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
