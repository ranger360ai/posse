//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-74tkp — the wait leg paces itself, on either
// multiplexer.
//
// WHAT WAS MEASURED, and where. ranger-base-8gh58's live acceptance ran two
// dispatch passes minutes apart against the same repo and the same runtime,
// differing only in the instance's `multiplexer:` key. On the herdr instance
// the "still working after Ns — waiting again" line was printed ZERO times
// over a longer wait; on the `multiplexer: rex` instance it was printed 295
// times in ~3m30s over 210 distinct one-second values — about 1.4 legs per
// second, for the whole length of one agent's turn, each iteration also
// re-reading the Rex server's program_status. A 40-minute bead is ~3,400
// lines on that arithmetic and the 4h WaitCeiling ~20,000, written into a
// state/dispatch-watch.log that rotates at 5 MiB. Nothing was misjudged: the
// claim was kept and the settle was read when it came, which is why the bead
// is P2 — it is cost and legibility.
//
// WHY IT IS NOT A REX BUG, which is what these pins exist to say. gather
// re-waited on whatever the last leg returned without ever asking how long
// that leg took, and printed a line every time it went round. On herdr a leg
// blocks for PromptWaitMS inside the server's own wait verb, so the defect
// was invisible there; on Rex there is no wait verb to delegate to, so a leg
// ends when its own deadline does and the loop went round at the speed of the
// box. MEASURED here on HERDR at 24 legs/second before the fix, with the fake
// server armed to answer every leg with the `timeout` code a real one answers
// a blown deadline with — which is also what a server replaced under an
// in-flight prompt does (TestDispatchRewaitServerGoneHandsTheBeadBack).
//
// SO EVERY PIN HERE RUNS ON BOTH OCCUPANTS and asserts the same ceiling,
// which is the bead's own ask: compare the two multiplexers in one test
// rather than asserting a rate on one.
//
// They drive gather() DIRECTLY, with the leg already in flight, because the
// subject is the loop and not the launch. A launch would put its own
// occupant-shaped settle gate in front of every assertion here — on Rex a
// `prompt: typed` runtime has to read idle before anything is typed and
// working forever after, which is a fixture about the launch path and not
// about pacing a wait.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// legPaceCeiling is how long each pin lets one bead wait. The pass ends on
// gather's WaitCeiling arm, so no pin here depends on a settle, a bd read or
// a judged bead.
const legPaceCeiling = 2 * time.Second

// waitLegGather runs one bounded gather per occupant over a seat that reads
// WORKING throughout and a wait leg that ends at once, and hands back each
// occupant's own output. tune sets the two knobs under test.
//
// THE TWO FIXTURES ARE AS ALIKE AS TWO MULTIPLEXERS CAN BE MADE, and the one
// difference is the honest one — how each occupant's leg comes back early:
//
//   - On REX nothing is injected. PromptWaitMS is 1ms, so RexMux's poller is
//     already past its own deadline when its first sample returns and it
//     answers with its own `timeout` (muxrex.go: "running out is a refusal
//     and not a reading"). That is the production shape: on this occupant any
//     leg whose deadline is shorter than the turn ends inside the turn.
//   - On HERDR the fake server's `wait-error` lever answers `agent wait` with
//     the same code. It has to be the lever, because the fake answers out of
//     a file and watches no clock, and a herdr leg that really did block for
//     PromptWaitMS is the case this defect hid behind.
//
// delivered is set on the pendingBead for the reason the measured incident
// had it set: codex is `prompt: argv`, so the prompt rode the launch line and
// a wait that fails says nothing about whether it landed (ADR 0013 §2). It is
// also what makes the loop reach the re-wait on EVERY leg error rather than
// only on a timeout, which is the arm a Rex leg can take.
func waitLegGather(t *testing.T, tune func(*Dispatcher)) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, mux := range []string{MuxHerdr, MuxRex} {
		t.Run(mux, func(t *testing.T) {
			b, fake := newTestBackend(t)
			const session = "ranger-qa-a-1"
			var target string
			if mux == MuxRex {
				m, _ := newTestRexMux(t)
				b.Slot = m
				rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: b.App.WorkspaceLabel(session),
					Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Label: "shell", Flavor: RexTerminalFlavor}}})
				rexSeedProcess(t, rexTestBlock, rexRuntimeProc())
				// One record, written by the runtime in the block's
				// foreground, saying working and going on saying it: a
				// one-element queue is a constant answer (fakerex_test.go).
				rexSeedStatus(t, rexTestBlock, []RexStatusRecord{rexRuntimeRec(MuxWorking)})
				target = rexTestBlock
				if err := b.writeMeta(&HerdrMeta{Name: session, Workspace: rexTestSession, Pane: rexTestBlock,
					Agent: "ranger", Mux: MuxRex, Socket: m.Server().Addr, Launched: time.Now()}); err != nil {
					t.Fatal(err)
				}
			} else {
				saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: b.App.WorkspaceLabel(session), AgentStatus: "working"}})
				workingClaude(t, fake)
				if err := os.WriteFile(filepath.Join(fake, "wait-error"),
					[]byte("timeout|timed out waiting for agent status"), 0o644); err != nil {
					t.Fatal(err)
				}
				target = "w1:p1"
				if err := b.writeMeta(&HerdrMeta{Name: session, Workspace: "w1", Pane: target,
					Agent: "ranger", Socket: SocketID(), Gen: ServerGen(), Launched: time.Now()}); err != nil {
					t.Fatal(err)
				}
			}
			d := newTestDispatcher(t, b)
			d.PromptWaitMS = 1
			d.WaitCeiling = legPaceCeiling
			tune(d)
			// The fixture's own control: the state this loop keeps re-waiting
			// on has to be the one that re-waits. A seat reading anything
			// else settles or holds on its first leg and pins nothing.
			if st, err := d.agentStatus(session); err != nil || st != "working" {
				t.Fatalf("%s: the fixture seat reads (%q, %v), want working — nothing below would be about pacing a wait", mux, st, err)
			}
			now := time.Now()
			p := &pendingBead{is: RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t"}, Dir: t.TempDir()},
				persona: "ranger", session: session, target: target, runtime: "claude", delivered: true,
				result: make(chan promptResult, 1), prompted: now, legAt: now, launched: now}
			d.rewait(p)
			inFlight, err := d.gather(p)
			got := dispatcherOut(d)
			if err != nil {
				t.Fatalf("%s: gather over a working seat failed: %v\n%s", mux, err, got)
			}
			if !inFlight {
				t.Fatalf("%s: gather judged a seat that never stopped working, so this did not end on the ceiling:\n%s", mux, got)
			}
			if !strings.Contains(got, "still working after") {
				t.Fatalf("%s: this gather never reached the re-wait arm, so it pins nothing about pacing it:\n%s", mux, got)
			}
			out[mux] = got
		})
	}
	return out
}

// legs is how many wait legs one bead's loop ran. It is read off the progress
// line, which is one per leg while ProgressEvery is 0 — the only
// per-iteration observable both occupants share, and exact.
func legs(out string) int { return strings.Count(out, "still working after") }

// THE POLL RATE. ProgressEvery is 0, so every leg says so and the line count
// IS the leg count; LegFloor is the number under test.
//
// A CEILING AND NEVER A FLOOR, which is what makes this safe on a loaded box:
// the floor can only make a loop take FEWER legs in a window, so a slow box
// moves the reading away from the assertion rather than into it. Before the
// fix this read in the hundreds on both occupants.
func TestQAWaitLegPollsOnAnIntervalOnEveryMultiplexer(t *testing.T) {
	t.Parallel()
	const floor = 400 * time.Millisecond
	// +2: the leg the ceiling lands on, and one for a sleep that rounds up.
	most := int(legPaceCeiling/floor) + 2
	for mux, out := range waitLegGather(t, func(d *Dispatcher) {
		d.LegFloor = floor
		d.ProgressEvery = 0
	}) {
		if n := legs(out); n > most {
			t.Errorf("%s: %d wait legs in %s at a %s floor, want at most %d — the loop re-waited without asking how long the last leg took:\n%s",
				mux, n, legPaceCeiling, floor, most, out)
		}
	}
}

// THE PROGRESS RATE, pinned with the floor OFF so the line rate cannot be
// riding on the poll rate: legs come back as fast as the occupant answers and
// the only thing holding the line down is ProgressEvery. That is the bead's
// own ask — "a progress line rate that does not track the poll rate".
func TestQAWaitLegProgressLineHasItsOwnIntervalOnEveryMultiplexer(t *testing.T) {
	t.Parallel()
	const every = 400 * time.Millisecond
	// +2: the first leg always says so, and the window the ceiling closes in.
	most := int(legPaceCeiling/every) + 2
	for mux, out := range waitLegGather(t, func(d *Dispatcher) {
		d.LegFloor = 0
		d.ProgressEvery = every
	}) {
		if n := legs(out); n > most {
			t.Errorf("%s: %d progress lines in %s at a %s interval, want at most %d — the record is written at the rate the box is polled:\n%s",
				mux, n, legPaceCeiling, every, most, out)
		}
	}
}

// AND THE TWO NUMBERS THEMSELVES, which is the half of the done-when no rate
// assertion can hold: a pass built the production way carries both, and they
// stand in the relation the defaults were chosen in.
func TestQAProductionDispatchCarriesBothPacingNumbers(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := NewDispatcher(b.App, b, nil)
	if d.LegFloor != DefaultLegFloor {
		t.Errorf("a production pass polls on %s, not DefaultLegFloor (%s)", d.LegFloor, DefaultLegFloor)
	}
	if d.ProgressEvery != DefaultProgressEvery {
		t.Errorf("a production pass prints progress every %s, not DefaultProgressEvery (%s)", d.ProgressEvery, DefaultProgressEvery)
	}
	// The numbers, said once here so that moving either is a change to this
	// pin: four readings a minute per waiting seat, one line a minute per
	// waiting bead.
	if DefaultLegFloor != 15*time.Second || DefaultProgressEvery != time.Minute {
		t.Errorf("the pacing numbers moved: floor %s, progress %s — the field docs price 15s and 1m", DefaultLegFloor, DefaultProgressEvery)
	}
	// A floor UNDER one leg is what keeps herdr byte-identical: its leg
	// blocks for PromptWaitMS inside `agent wait`, so the remainder is always
	// negative and nothing on that path ever sleeps.
	if leg := time.Duration(DefaultPromptWaitMS) * time.Millisecond; DefaultLegFloor >= leg {
		t.Errorf("DefaultLegFloor (%s) is not under one wait leg (%s) — on herdr the floor would start adding wall to a leg that already spent it", DefaultLegFloor, leg)
	}
	// And a line no more often than a poll: a progress interval shorter than
	// the floor is a number that can never be reached, which reads as a
	// chosen rate and is not one.
	if DefaultProgressEvery < DefaultLegFloor {
		t.Errorf("DefaultProgressEvery (%s) is shorter than DefaultLegFloor (%s), so the line rate is the poll rate wearing another name", DefaultProgressEvery, DefaultLegFloor)
	}
}
