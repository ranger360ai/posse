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
	"strconv"
	"strings"
	"testing"
	"time"
)

// legPaceCeiling is how long each pin lets one bead wait. The pass ends on
// gather's WaitCeiling arm, so no pin here depends on a settle, a bd read or
// a judged bead.
const legPaceCeiling = 2 * time.Second

// legPaceSession is the one seat every fixture in this file hires. Named so
// that an assertion about what the pass SAID about it does not re-spell it.
const legPaceSession = "ranger-qa-a-1"

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
			const session = legPaceSession
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

// ---------------------------------------------------------------------------
// ranger-base-aj79z — and what the leg that did not wait came back WITH.
//
// The pins above hold the rate. These hold the one thing the rate traded
// away: with ProgressEvery at a minute the per-leg progress line is 84x
// quieter, and the per-leg line was the only signal that ever exposed this
// class — 295 lines in ~3m30s is what got ranger-base-74tkp filed at all.
//
// WHAT HAD NO SINK. Everything past gather's `r.err == nil` arm runs with a
// non-nil leg error, and on the `delivered` path neither guard that consumes
// one can fire: both are `&& !p.delivered`, because on `prompt: argv` the
// prompt landed at exec and a wait failure is never a delivery failure
// (ADR 0013 §2). So nothing printed, logged or recorded it — a seat whose
// every leg failed for a whole turn printed exactly what a healthy one
// printed. MEASURED FALSE 2026-11-07 over this file's own fixture:
// ranger-base-74tkp's close said the discriminator was already in the pass
// output ("a leg that ran out names its own deadline, and a leg that could
// not read names the rex call"), and neither half could print — the loop only
// goes round again when the status probe answers `working`, and the only arm
// that named a rex call was the `default` one, which returns after one leg.
//
// These pins run on both occupants for the reason every pin in this file
// does, and here the two occupants ARE the two halves of that discriminator:
// rex's own blown deadline, and the code herdr's server returns.

// legErrs is how many times one bead's loop named its leg error. One line per
// failing leg while ProgressEvery is 0, same as legs().
func legErrs(out string) int { return strings.Count(out, "wait leg ended:") }

// THE ERROR ITSELF REACHES THE OUTPUT, on both occupants, in the regime that
// produces the busy-wait: a leg that comes back failing while the agent reads
// WORKING throughout, which is every leg this fixture runs.
//
// Each occupant is pinned on its own text, because that is the discriminator:
// on rex the leg names the deadline it blew and the pane it blew it in, and
// on herdr it names what the server said. Both carry the code, which is what
// HerdrAPIError.Error() renders and what RexMux.Wait builds by hand for its
// own deadline.
func TestQAWaitLegErrorReachesThePassOutputOnEveryMultiplexer(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		// The lever this file arms the fake with, rendered through
		// HerdrAPIError.Error().
		MuxHerdr: {"wait leg ended:", "timed out waiting for agent status", "(timeout)"},
		// RexMux.Wait's own deadline refusal: the `until` set it was given,
		// the pane, and the code it chose over a stall (muxrex.go).
		MuxRex: {"wait leg ended:", "no " + strings.Join([]string{MuxIdle, MuxDone, MuxBlocked}, "|") + " state in " + rexTestBlock, "(timeout)"},
	}
	for mux, out := range waitLegGather(t, func(d *Dispatcher) {
		d.LegFloor = 400 * time.Millisecond
		d.ProgressEvery = 0
	}) {
		for _, probe := range want[mux] {
			if !strings.Contains(out, probe) {
				t.Errorf("%s: a pass whose every wait leg failed never printed %q, so the leg error has no sink on the delivered path:\n%s",
					mux, probe, out)
			}
		}
	}
}

// AND IT IS PACED, on its own interval and not per leg — the half of the
// done-when that keeps this from undoing ranger-base-74tkp. The floor is OFF
// so legs come back as fast as the occupant answers and ProgressEvery is the
// only thing holding the line down, exactly as the progress-line pin does it.
func TestQAWaitLegErrorLineHasItsOwnIntervalOnEveryMultiplexer(t *testing.T) {
	t.Parallel()
	const every = 400 * time.Millisecond
	// +2: the first failing leg always says so, and the window the ceiling
	// closes in.
	most := int(legPaceCeiling/every) + 2
	for mux, out := range waitLegGather(t, func(d *Dispatcher) {
		d.LegFloor = 0
		d.ProgressEvery = every
	}) {
		if n := legErrs(out); n > most {
			t.Errorf("%s: %d leg-error lines in %s at a %s interval, want at most %d — the leg error is named at the rate the box is polled:\n%s",
				mux, n, legPaceCeiling, every, most, out)
		}
		if n := legErrs(out); n == 0 {
			t.Errorf("%s: no leg-error line at all in %s, so the pacing suppressed the first one too:\n%s", mux, legPaceCeiling, out)
		}
	}
}

// THE TWO CLOCKS ARE SEPARATE, which is the whole of why sayLegError does not
// read saidWorking. Both lines are printed in the same iteration, so one
// shared timestamp means whichever printed first suppresses the other for a
// whole ProgressEvery — and the leg error is printed FIRST, so the sharing
// mutant costs the progress line, in the regime where the agent IS working.
//
// ProgressEvery is ten minutes against a 2s ceiling, so each line can only be
// the first one: exactly one of each, on a box of any speed. With a shared
// clock the progress line never prints and waitLegGather's own guard fails
// first, which is the mutant reddening rather than this assertion.
func TestQAWaitLegErrorDoesNotSuppressTheProgressLine(t *testing.T) {
	t.Parallel()
	for mux, out := range waitLegGather(t, func(d *Dispatcher) {
		d.LegFloor = 0
		d.ProgressEvery = 10 * time.Minute
	}) {
		if n := legs(out); n != 1 {
			t.Errorf("%s: %d progress lines inside one ProgressEvery, want 1:\n%s", mux, n, out)
		}
		if n := legErrs(out); n != 1 {
			// 0 = the progress line's clock suppressed it; more than 1 = it
			// is not paced at all. Either way the two clocks are not the two
			// this pin is about.
			t.Errorf("%s: %d leg-error lines inside one ProgressEvery, want 1:\n%s", mux, n, out)
		}
	}
}

// ─── and WHERE the sink is: before the probe, not inside an arm of it ────────
//
// ranger-base-obaim. The three pins above are all driven by waitLegGather,
// whose seat reads WORKING throughout — it hard-requires that ("this gather
// never reached the re-wait arm"), because the busy-wait regime it was written
// for IS the `working` arm. So all three are satisfied by a sink that only
// fires inside `case "working":`, and the PLACEMENT the close states as the
// decision — "printed BEFORE the probe, not inside an arm of it, because the
// leg error is a fact about the leg and not about what the status probe then
// says" — was unpinned.
//
// MEASURED 2026-11-07 with the sink moved into `case "working":`: `blocked`,
// the WaitCeiling arm and the `default` arm each print no leg error, and the
// reach pin stays green because earlier `working` legs printed one. Three of
// the four arms the close enumerates, silently back to what ranger-base-aj79z
// was filed off — a leg that ran out, a leg the occupant refused and a leg
// whose read failed all reaching the probe indistinguishable from each other.
//
// WHY A SECOND FIXTURE AND NOT A TUNE KNOB. These arms end the pass on the
// FIRST leg — `blocked` and `default` both `return true, nil` — so there is no
// re-wait to pace and nothing here is about a rate. waitLegGather's own
// control would fail first.

// waitLegGatherEndingOn runs one bounded gather per occupant over a seat the
// status probe reads as `st`, with the wait leg failing, and hands back each
// occupant's output. `st` is a state gather does NOT loop on, so the pass ends
// on its first leg.
//
// THE LEG FAILS ON EACH OCCUPANT THE WAY waitLegGather's DOES, and the
// difference between the two fixtures is the same honest one:
//
//   - On HERDR the fake's `wait-error` lever answers `agent wait` with the
//     `timeout` code, independent of what the seat reads — so the seat reads
//     `st` from the start.
//   - On REX nothing is injected: the status QUEUE steps. Its head is
//     `working`, which is not in rewait's until set, so RexMux's poller is
//     already past its own 1ms deadline when that first sample returns and it
//     answers with its own `timeout` (the file head's note). The next element
//     is `st` and the fake leaves the last one in place forever
//     (fakeRexProgramStatus), so every later read — the probe's, and
//     statusAfterTimeout's re-reads on the `default` arm — is `st`.
//
// That is also the production sequence rather than a contrivance: the leg ran
// out while the agent was working, and by the time the probe asked, the seat
// had moved.
func waitLegGatherEndingOn(t *testing.T, st string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, mux := range []string{MuxHerdr, MuxRex} {
		t.Run(mux, func(t *testing.T) {
			b, fake := newTestBackend(t)
			const session = legPaceSession
			var target string
			if mux == MuxRex {
				m, _ := newTestRexMux(t)
				b.Slot = m
				rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: b.App.WorkspaceLabel(session),
					Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Label: "shell", Flavor: RexTerminalFlavor}}})
				rexSeedProcess(t, rexTestBlock, rexRuntimeProc())
				rexSeedStatus(t, rexTestBlock,
					[]RexStatusRecord{rexRuntimeRec(MuxWorking)}, []RexStatusRecord{rexRuntimeRec(st)})
				target = rexTestBlock
				if err := b.writeMeta(&HerdrMeta{Name: session, Workspace: rexTestSession, Pane: rexTestBlock,
					Agent: "ranger", Mux: MuxRex, Socket: m.Server().Addr, Launched: time.Now()}); err != nil {
					t.Fatal(err)
				}
			} else {
				saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: b.App.WorkspaceLabel(session), AgentStatus: st}})
				if err := os.WriteFile(filepath.Join(fake, "agents.json"),
					[]byte(`[{"agent":"claude","agent_status":"`+st+`","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644); err != nil {
					t.Fatal(err)
				}
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
			d.LegFloor = 0
			d.ProgressEvery = 0
			now := time.Now()
			p := &pendingBead{is: RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t"}, Dir: t.TempDir()},
				persona: "ranger", session: session, target: target, runtime: "claude", delivered: true,
				result: make(chan promptResult, 1), prompted: now, legAt: now, launched: now}
			d.rewait(p)
			inFlight, err := d.gather(p)
			got := dispatcherOut(d)
			if err != nil {
				t.Fatalf("%s: gather over a %s seat failed: %v\n%s", mux, st, err, got)
			}
			// Both arms under test KEEP the claim, and a pass that judged the
			// bead reached neither of them — on rex that is the leg having
			// succeeded (its poller got a second sample inside 1ms and read
			// `st` off the queue), which would make everything below vacuous.
			if !inFlight {
				t.Fatalf("%s: gather judged a %s seat, so the pass never reached the arm this pin is about:\n%s", mux, st, got)
			}
			out[mux] = got
		})
	}
	return out
}

// legErrorBefore is the placement itself: the leg error is in the output AND
// it is above `arm`, which is the one thing a sink moved inside an arm of the
// probe cannot do.
func legErrorBefore(t *testing.T, mux, out, arm string) {
	t.Helper()
	i, j := strings.Index(out, "wait leg ended:"), strings.Index(out, arm)
	if j < 0 {
		t.Fatalf("%s: this gather never printed %q, so it did not end on the arm this pin is about:\n%s", mux, arm, out)
	}
	if i < 0 {
		t.Errorf("%s: a failed wait leg reached %q and never said it failed — the leg error is printed inside an arm of the status probe and not before it, so a leg that ran out, one the occupant refused and one whose read failed are indistinguishable here (ranger-base-aj79z):\n%s",
			mux, arm, out)
		return
	}
	if i > j {
		t.Errorf("%s: the leg error is printed AFTER %q — it is a fact about the leg and not about what the probe then says:\n%s", mux, arm, out)
	}
}

// THE `blocked` ARM. The occupant says the agent is waiting on a human, and
// the leg that just failed is the other half of that sentence: an operator
// told to attach needs to know the harness also could not read the seat.
func TestQAWaitLegErrorIsSaidOnTheBlockedArmOnEveryMultiplexer(t *testing.T) {
	t.Parallel()
	for mux, out := range waitLegGatherEndingOn(t, MuxBlocked) {
		legErrorBefore(t, mux, out, "blocked in ")
		// And the occupant's OWN text, which is the discriminator these pins
		// exist for — not just that something was said.
		want := "timed out waiting for agent status"
		if mux == MuxRex {
			want = "no " + strings.Join([]string{MuxIdle, MuxDone, MuxBlocked}, "|") + " state in " + rexTestBlock
		}
		for _, probe := range []string{want, "(timeout)"} {
			if !strings.Contains(out, probe) {
				t.Errorf("%s: the leg error on the blocked arm does not carry %q:\n%s", mux, probe, out)
			}
		}
	}
}

// THE `default` ARM — a state nobody names, which is the arm whose own line
// says the LEAST about the leg: statusPhrase names the PROBE's error and never
// this one, so without the sink the only thing printed about a leg that failed
// is a sentence about what the occupant could not say.
func TestQAWaitLegErrorIsSaidOnTheUnnamedStateArmOnEveryMultiplexer(t *testing.T) {
	t.Parallel()
	// Not one of the four gather branches on, and not "" either: an empty
	// reading is statusPhrase's "detects no agent" arm, which is a different
	// sentence. This is a state the occupant does name and posse does not know.
	const unnamed = "pondering"
	for mux, out := range waitLegGatherEndingOn(t, unnamed) {
		legErrorBefore(t, mux, out, "wait timed out after")
		// What the arm prints back is the OCCUPANT's reading, and the two
		// occupants honestly differ: herdr reports the agent_status string it
		// was given, while RexMux maps any record state it does not know to
		// MuxUnknown before posse ever sees it (muxrex.go, ADR 0076 §3 —
		// never to MuxIdle). Either way it is statusPhrase's `default` arm,
		// which is what this pin needs to have reached.
		state := unnamed
		if mux == MuxRex {
			state = MuxUnknown
		}
		if !strings.Contains(out, "herdr reports "+legPaceSession+" "+strconv.Quote(state)) {
			t.Errorf("%s: the default arm did not print the state it could not read (%q), so this may be some other arm:\n%s", mux, state, out)
		}
	}
}
