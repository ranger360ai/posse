//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-hcig1 — the settle window's two exits say one
// sentence, and the occupant's own refusal is one of them.
//
// WHAT WAS OBSERVED, on the live fleet's two dispatch pass logs (MEASURED
// 2026-11-23, the census and the counts are in settletimeout.go's head
// comment and docs/notes.d/ranger-base-hcig1.md): five launches abandoned by
// a line that named a Rex block UUID, no session, no `posse peek` and no
// readings-log record — against sixteen abandoned by awaitTarget's line,
// which names all three.
//
// WHAT THESE PINS HOLD, and the two that are easy to lose:
//
//  1. the reading rides on the ERROR and not in the result (arm 1), so no
//     caller can read a blown deadline as a state — and `timeout` still
//     matches through IsHerdrCode, including wrapped, because four live
//     sites branch on exactly that.
//  2. the ladder's refusal names the session, the patience and the peek, and
//     NOT the occupant's block id (arm 2).
//  3. the record exists at all (arm 3). Nothing printed is read under
//     `--watch`; the record is the only reader this refusal will ever have.
//  4. THE CONSEQUENCE DID NOT MOVE (arms 2-3 both). The raw error abandoned
//     the launch and typed nothing, and so does this: a timeout folded into
//     the deadline arm's concession would PROMPT a pane whose last reading
//     was `working`, which is rangerhq-37c's race. That is the half a
//     legibility bead can break while every assertion about wording passes.
//  5. an occupant that carries NO reading back still refuses legibly (arm
//     4), which is herdr — its wait runs out inside its own server and posse
//     never held a reading to carry.
//  6. ONE SENTENCE FOR THE OPERATOR, TWO VERDICTS FOR THE CENSUS (arm 5),
//     driven through both real exits rather than compared as literals.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

// stSession gives a session a tree (so the readings log has a home), a
// runtime and a meta, and returns the log path. It is d3capture's d3Session
// with this file's ids; that one is arm 3's and cannot be reached from here.
func stSession(t *testing.T, b *HerdrBackend, name, workspace, pane string, mux string) string {
	t.Helper()
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuntime(t, b.App, "fastcli", "command: fastcli --sys {file}\nstartup_wait: 400ms\n")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	tree, err := b.App.EnsureSessionTree(repo, name, nil)
	if err != nil || tree == nil {
		t.Fatalf("EnsureSessionTree: %v (tree=%v)", err, tree)
	}
	if err := b.writeMeta(&HerdrMeta{
		Name: name, Workspace: workspace, Pane: pane, Agent: "ranger",
		Runtime: "fastcli", Mux: mux, Dir: tree.Path, Repo: repo, Branch: tree.Branch,
	}); err != nil {
		t.Fatalf("writeMeta: %v", err)
	}
	log, err := b.App.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	return log
}

// stRexSeat is a Rex instance holding one seat whose record never settles:
// `state` forever, which is what a one-element status queue is (fakerex_test).
func stRexSeat(t *testing.T, b *HerdrBackend, name, state string) (Seat, string) {
	t.Helper()
	var sets [][]RexStatusRecord
	if state == "" {
		// The `last reading: unknown` half of the measured five: no record
		// at all, which rexReading answers MuxUnknown with Seen false.
		sets = [][]RexStatusRecord{nil}
	} else {
		sets = [][]RexStatusRecord{{rexRuntimeRec(state)}}
	}
	b.Slot = rexSeatWithRuntime(t, sets...)
	return rexTestSeat(), stSession(t, b, name, rexTestSession, rexTestBlock, MuxRex)
}

// stOneRecord reads the single reading a refusal wrote.
func stOneRecord(t *testing.T, log string) Reading {
	t.Helper()
	rs, err := ReadReadings(log)
	if err != nil {
		t.Fatalf("ReadReadings(%s): %v", log, err)
	}
	if len(rs) != 1 {
		t.Fatalf("the refusal wrote %d readings, want 1", len(rs))
	}
	return rs[0]
}

// stSettle drives the real ladder and hands back everything a pin here reads.
func stSettle(t *testing.T, b *HerdrBackend, bead, session string, target Seat, wait time.Duration) (string, AgentDetection, error, string) {
	t.Helper()
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Poll = 10 * time.Millisecond
	status, det, err := d.awaitSettled(bead, session, target,
		[]string{MuxIdle, MuxDone, MuxBlocked}, time.Now().Add(wait), wait)
	return status, det, err, out.String()
}

// ─── arm 1: the reading rides on the error, and the code survives ────────────

func TestQAARexWaitTimeoutCarriesItsLastReadingOnTheError(t *testing.T) {
	t.Parallel()
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec(MuxWorking)})

	read, err := m.Wait(rexTestSeat(), []string{MuxIdle, MuxDone, MuxBlocked}, 30)
	if err == nil {
		t.Fatal("the wait returned on a seat whose record never settles — this pin's premise")
	}
	// THE RESULT IS STILL EMPTY. "Running out is a refusal and not a
	// reading" is the verb's contract and the whole reason the reading moved
	// onto the error instead of into this value.
	if read != (MuxReading{}) {
		t.Errorf("a blown deadline answered with a reading — a caller could open a gate on it: %+v", read)
	}
	// The code four live sites branch on, directly and wrapped. IsHerdrCode
	// is errors.As over HerdrAPIError, so the wrap is the whole question.
	if !IsHerdrCode(err, "timeout") {
		t.Fatalf("the refusal lost its `timeout` code — the gather wait leg, the stall probe, relaunch's landing and RexMux.Prompt all branch on it: %v", err)
	}
	if !IsHerdrCode(fmt.Errorf("waiting on %s: %w", rexTestBlock, err), "timeout") {
		t.Error("the code does not survive a wrap — errors.As is why this is a type and not a string")
	}
	if got := HerdrCodeOf(err); got != "timeout" {
		t.Errorf("HerdrCodeOf = %q, want \"timeout\" — a readings row records the code through it", got)
	}

	last, ok := MuxWaitLastReading(err)
	if !ok {
		t.Fatalf("the refusal carried no reading: %v", err)
	}
	if last.State != MuxWorking || !last.Seen {
		t.Errorf("the reading rode back as %+v, want state %q with Seen — Seen is the occupant's own answer to \"does a runtime hold this keyboard\" (ADR 0076 §3)", last, MuxWorking)
	}
	if _, ok := MuxWaitLastReading(fmt.Errorf("wrapped: %w", err)); !ok {
		t.Error("the reading does not survive a wrap, for arm 1's reason one assertion up")
	}
	// And it is not claimed for errors that never carried one: herdr's own
	// timeout envelope is the shape that matters, because it is live.
	if _, ok := MuxWaitLastReading(HerdrAPIError{Code: "timeout", Message: "herdr's own"}); ok {
		t.Error("an envelope with no reading in it answered true — a caller would then print a state nobody read")
	}
	if _, ok := MuxWaitLastReading(errors.New("unrelated")); ok {
		t.Error("an unrelated error answered true")
	}
	// The MESSAGE is unchanged: `posse wait` prints this error as text and
	// names the block, which is the seat the occupant was addressing.
	for _, want := range []string{rexTestBlock, "last reading: " + MuxWorking, "no idle|done|blocked state"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the occupant's own sentence lost %q:\n%s", want, err)
		}
	}
}

// ─── arm 2: the ladder's refusal, and nothing typed ──────────────────────────

func TestQATheSettleTimeoutRefusalNamesTheSessionAndNotTheBlock(t *testing.T) {
	t.Parallel()
	// Both halves of the measured five: a seat reporting `working` to the
	// end, and a seat that reported nothing at all.
	for _, tc := range []struct {
		name, state, reads string
	}{
		{"working", MuxWorking, MuxWorking},
		{"no record", "", MuxUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			session := "stx-" + strings.ReplaceAll(tc.name, " ", "-")
			target, _ := stRexSeat(t, b, session, tc.state)

			wait := 120 * time.Millisecond
			status, det, err, out := stSettle(t, b, "ranger-base-hcig1", session, target, wait)
			if err == nil {
				t.Fatalf("the gate opened on a seat that never settled: status %q\n%s", status, out)
			}
			// Nothing this gate accepted, so nothing a caller can branch on
			// — the deadline arm's refusal returns the same pair.
			if status != "" || det.State != "" || det.Seen() {
				t.Errorf("the refusal handed back a state: %q %+v", status, det)
			}
			line := err.Error()
			for _, want := range []string{
				session,
				"posse peek " + session,
				"never became promptable within " + wait.String(),
				fmt.Sprintf("the last state it read was %q", tc.reads),
			} {
				if !strings.Contains(line, want) {
					t.Errorf("the refusal must name %q — it is what the operator reading a pass has to get to the seat with:\n%s", want, line)
				}
			}
			// The three things the raw error said instead.
			for _, never := range []string{rexTestBlock, "herdr:", "(timeout)"} {
				if strings.Contains(line, never) {
					t.Errorf("the refusal still says %q — a block UUID is not a name any posse verb takes, and herdr is not the multiplexer here:\n%s", never, line)
				}
			}
			// THE CONSEQUENCE DID NOT MOVE. The concession prompts; this
			// refuses. A timeout that reached the concession would type at a
			// pane whose last reading was `working`.
			if strings.Contains(out, "prompting on its") {
				t.Errorf("the timeout took the deadline arm's concession and the launch was PROMPTED — rangerhq-37c's race:\n%s", out)
			}
		})
	}
}

// ─── arm 3: the record ───────────────────────────────────────────────────────

func TestQATheSettleTimeoutWritesItsRefusalRecord(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, state, reads string
		seen               bool
	}{
		{"working", MuxWorking, MuxWorking, true},
		{"no record", "", MuxUnknown, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			session := "str-" + strings.ReplaceAll(tc.name, " ", "-")
			target, log := stRexSeat(t, b, session, tc.state)

			if _, _, err, _ := stSettle(t, b, "ranger-base-hcig1", session, target, 120*time.Millisecond); err == nil {
				t.Fatal("the gate opened on a seat that never settled")
			}

			r := stOneRecord(t, log)
			if r.Decision != DecisionPaneState || r.Consequence != ConsequenceRefusal {
				t.Errorf("want a D1 refusal, got %s/%s — the reading this spent is a pane state, and nothing was typed or claimed", r.Decision, r.Consequence)
			}
			if r.Rule != RuleSettleGate {
				t.Errorf("Rule = %q, want %q: a `Wait` answers with a state and nothing else, so there was never a manifest rule to name", r.Rule, RuleSettleGate)
			}
			if r.Bead != "ranger-base-hcig1" || r.Session != session {
				t.Errorf("the record does not say which launch it was: bead %q session %q", r.Bead, r.Session)
			}
			if !strings.HasPrefix(r.Verdict, "never settled: the wait ran out") {
				t.Errorf("the verdict must say WHICH exit closed the window, so a census can price the two apart: %q", r.Verdict)
			}
			if !strings.Contains(r.Verdict, tc.reads) {
				t.Errorf("the verdict does not name the last reading %q: %q", tc.reads, r.Verdict)
			}
			if r.Herdr.State != tc.reads || r.Herdr.Seen != tc.seen {
				t.Errorf("the evidence is %+v, want state %q seen=%v — `working` with Seen is a busy runtime and `unknown` without it is a seat that reported nothing, which are the two halves of the measured five",
					r.Herdr, tc.reads, tc.seen)
			}
			// NO CAPTURE, by blockedsettle.go's first gate: this is the D1
			// branch, a state from a verb that answers with a state, so
			// there is no screen a reader could name and no `LooksLike` row
			// — which is also what keeps Reading.LooksLike's documented
			// scope (D3, and the one D2 class) true.
			if len(r.Herdr.Regions) != 0 || len(r.LooksLike) != 0 {
				t.Errorf("the record carries a screen reading its own decision never looked at: regions %+v looks_like %v", r.Herdr.Regions, r.LooksLike)
			}
		})
	}
}

// ─── arm 4: an occupant that carries no reading back ─────────────────────────

// This is herdr, and it is not a hypothetical: `agent wait` runs the window
// out inside herdr's own server and answers with an envelope, so posse never
// held a reading to carry. The refusal must still name the session and the
// peek rather than leaving by the raw error.
func TestQAASettleTimeoutWithNoReadingStillNamesTheSessionAndThePeek(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	session := "stn"
	log := stSession(t, b, session, "w1", "w1:p1", "")
	// No pipe in the message: the fake cuts "code|message" at the first one.
	write(t, filepath.Join(fake, "wait-error"), "timeout|no settled state in w1:p1 within 45s")

	wait := 120 * time.Millisecond
	status, _, err, out := stSettle(t, b, "ranger-base-hcig1", session, fakeHerdrSeat, wait)
	if err == nil {
		t.Fatalf("the gate opened on a wait that refused: %q\n%s", status, out)
	}
	line := err.Error()
	for _, want := range []string{
		session,
		"posse peek " + session,
		"never became promptable within " + wait.String(),
		"carried no reading back",
		"no settled state in w1:p1", // herdr's own words, which is all there is
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the refusal must name %q:\n%s", want, line)
		}
	}
	// It must not invent one either.
	if strings.Contains(line, "the last state it read was") {
		t.Errorf("the refusal named a state nobody read:\n%s", line)
	}
	r := stOneRecord(t, log)
	if r.Herdr.State != "" || r.Herdr.Seen || len(r.Herdr.Regions) != 0 {
		t.Errorf("the record carries evidence posse never held: %+v", r.Herdr)
	}
	if !strings.Contains(r.Verdict, "no reading") {
		t.Errorf("the verdict does not say the wait carried nothing: %q", r.Verdict)
	}
}

// ─── arm 5: one sentence, two verdicts ───────────────────────────────────────

// The bead's own complaint, pinned by driving BOTH exits of one window: the
// operator gets one sentence from either, and the census gets two verdicts.
// Compared as literals this would prove nothing about which exit reaches
// which, so each half runs its real ladder — the timeout against a Rex seat
// that never settles, the deadline against a herdr that guesses forever
// (matched_rule null, the idle fallback: ADR 0013 §1's own case).
func TestQABothSettleWindowExitsSayOneSentenceAndRecordTwoVerdicts(t *testing.T) {
	t.Parallel()
	wait := 120 * time.Millisecond

	// Exit 1: the occupant's wait refused.
	bTimeout, _ := newTestBackend(t)
	target, timeoutLog := stRexSeat(t, bTimeout, "stb-timeout", MuxWorking)
	_, _, timeoutErr, _ := stSettle(t, bTimeout, "ranger-base-hcig1", "stb-timeout", target, wait)

	// Exit 2: the ladder's own deadline, with a real answer in hand.
	bDeadline, fake := newTestBackend(t)
	deadlineLog := stSession(t, bDeadline, "stb-deadline", "w1", "w1:p1", "")
	write(t, filepath.Join(fake, "explain-fallback"), "") // guess forever
	_, _, deadlineErr, _ := stSettle(t, bDeadline, "ranger-base-hcig1", "stb-deadline", fakeHerdrSeat, wait)

	if timeoutErr == nil || deadlineErr == nil {
		t.Fatalf("both exits must refuse: timeout=%v deadline=%v", timeoutErr, deadlineErr)
	}
	// ONE SENTENCE. The shared part is the whole of what the operator needs
	// and the whole of what the raw error was missing.
	for _, exit := range []struct {
		name    string
		err     error
		session string
	}{
		{"the occupant's wait", timeoutErr, "stb-timeout"},
		{"the ladder's deadline", deadlineErr, "stb-deadline"},
	} {
		for _, want := range []string{
			"agent in " + exit.session + " never became promptable within " + wait.String(),
			"check the session (posse peek " + exit.session + ")",
		} {
			if !strings.Contains(exit.err.Error(), want) {
				t.Errorf("%s does not say %q — the two exits are one fact and must read as one:\n%s", exit.name, want, exit.err)
			}
		}
	}
	// TWO VERDICTS. Same consequence, different cause: a census that folded
	// them would price "the occupant never answered a settle" and "posse
	// never confirmed the settle it was handed" as one class.
	tr, dr := stOneRecord(t, timeoutLog), stOneRecord(t, deadlineLog)
	if tr.Consequence != ConsequenceRefusal || dr.Consequence != ConsequenceRefusal {
		t.Errorf("both exits refuse: %s / %s", tr.Consequence, dr.Consequence)
	}
	if !strings.HasPrefix(tr.Verdict, "never settled: the wait ran out") {
		t.Errorf("the timeout exit's verdict moved: %q", tr.Verdict)
	}
	if !strings.HasPrefix(dr.Verdict, "never promptable:") {
		t.Errorf("the deadline exit's verdict moved: %q", dr.Verdict)
	}
	if tr.Decision == dr.Decision && tr.Verdict == dr.Verdict {
		t.Errorf("the two exits are indistinguishable in the log: %s %q", tr.Decision, tr.Verdict)
	}
}
