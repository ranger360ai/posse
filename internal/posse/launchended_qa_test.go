//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-ybrk6: a runtime that dies before it reports reads
// as "that seat ENDED", not as "rex would not answer".
//
// THE DEFECT. ADR 0076 §3's launch confirmation has three outcomes and only
// two of them are a judgement: a record arrived; the bound ran out with
// nothing to read (the argv/typed arms); or the read FAILED, which was all
// read as "rex could not be asked" and kept the launch. Until
// ranger-base-1l1en a runtime that died at its first instant left the pane's
// shell behind, so its block stayed open and the poll answered *no record* —
// the second outcome. Every launch line is exec'd now, so the runtime IS the
// block's program and the block closes with it: the read fails with rex's
// `block target "<id>" not found` and the third outcome fires, over a server
// that answered immediately and correctly. MEASURED 2026-11-15 on a scratch
// server (ADR 0080's amendment of that date):
//
//	posse new husk --cmd '/bin/sh -c printf-then-sleep-3'
//	  -> claude launch in husk could not be confirmed — rex would not
//	     answer the program_status read (… block target "…" not found)
//	the same launch with a command that stays up
//	  -> DEGRADED — no program_status record from claude in husk within 45s
//
// The gate's property held by another route — nothing is prompted, because
// dispatch's awaitAgent then fails and benches the slot — so what these pins
// are about is the DIAGNOSIS, which sent an operator to look at rex.
//
// THREE ARMS, BECAUSE THE VERDICT LADDER IS ONE (HerdrBackend.launchVerdict).
// The reason is new; what a miss costs is still keyed on `prompt:` and on
// whether anybody is watching, which is ADR 0076 D2's whole content. So each
// of the three verdicts is pinned over the new reason, and the fourth pin is
// the one that keeps the reasons apart: a read that failed for any OTHER
// cause is still ignorance and still keeps the launch
// (TestQARexThatCannotBeAskedIsIgnoranceAndNeverARefusal, muxslot_qa_test.go).
//
// WHY THE PREDICATE IS PINNED PER VERB. The bead was filed with a
// measurement to take with it — "rex's exact error text and shape for a gone
// BLOCK on each verb posse calls, so the predicate is not matched on one
// message" — and taking it found the row that a predicate written from
// confirmReports' own call would have missed: `rex focus` names `block or
// window target`, because it takes either in one positional. Five verbs say
// one sentence and the sixth says another.

import (
	"os"
	"strings"
	"testing"
	"time"
)

// ─── the predicate, over every sentence rex actually prints ──────────────────

// MEASURED 2026-11-15, this box (linux/amd64, rex 0.0.1794069975-g6c6689),
// against a scratch server on a private socket with the fleet's own server
// never addressed: the full table is in rexBlockTargetGone's doc and the
// transcript in docs/notes.d/ranger-base-ybrk6.md. Each line here is wrapped
// the way Rex.callErr wraps it.
func TestQARexBlockGoneReadsEveryVerbsRefusalAndNothingElse(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		err  error
		gone bool
	}{
		{"program_status", Die(`rex block call -s session:a -b block:a program_status: Error: block target "block:a" not found`), true},
		{"process", Die(`rex block call -s session:a -b block:a process: Error: block target "block:a" not found`), true},
		{"capture", Die(`rex capture -s session:a -b block:a --format text --trim: Error: block target "block:a" not found`), true},
		{"send", Die(`rex send -s session:a -b block:a hello: Error: block target "block:a" not found`), true},
		{"send-key", Die(`rex send-key -s session:a -b block:a enter: Error: block target "block:a" not found`), true},
		// The row a predicate written from one call site misses. `rex focus`
		// takes a block OR a window in one positional and its refusal names
		// both, so `block target "` alone answers false here while
		// answering true for the five above.
		{"focus, which names a block or a window", Die(`rex focus -s session:a block:a: Error: block or window target "block:a" not found`), true},
		// The block's session destroyed under it: the same sentence with a
		// reason clause after it, which the prefix match carries unchanged.
		{"the block's session gone too", Die(`rex block call -s session:a -b block:a program_status: Error: block target "block:a" not found: session scope: session "session:a" not found`), true},
		// An id that is no id at all, and one that never existed: rex
		// answers about the target either way.
		{"a word that is no block id", Die(`rex capture -s session:a -b notablock --format text --trim: Error: block target "notablock" not found`), true},

		{"nothing failed at all", nil, false},
		// A refusal that NAMES this block and is not a `not found`. There is
		// no such sentence in rex 0.0.1794069975-g6c6689's surface above —
		// this row is written against the shape and not a reading — and it
		// is here because the mutation table found the suffix test held by
		// nothing: dropping `" not found` reds no pin without it. What it
		// buys is the reading that matters, that "rex mentioned the block"
		// is not "rex does not have the block", so a future verb that
		// refuses a block it HOLDS cannot be read as a seat that ended.
		{"a refusal that names the block without losing it", Die(`rex send -s session:a -b block:a hello: Error: block target "block:a" is not accepting input`), false},
		// The reading that must never be made, and the same arm
		// rexSessionGone wears: a hang is posse never having reached the
		// server, and read as "the seat is gone" it would refuse a launch
		// on the one answer that proves nothing about the seat. This row
		// pins the OUTCOME and not the line that produces it — removing
		// rexBlockGone's IsHerdrHang guard reds nothing, because a hang's
		// text is posse's argv and not rex's sentence (MEASURED, this
		// bead's mutation table). What it does catch is a rewrite that
		// keys on the EXIT CODE, where a killed child and a refusal are
		// the same nonzero.
		{"a blown deadline", &HerdrHangError{Argv: []string{"rex", "block", "call", "-b", "block:a", "program_status"}, Limit: time.Minute, Waited: time.Minute}, false},
		{"a server that is not running", Die(`rex block call -s session:a -b block:a program_status: Error: no server at unix:///run/user/1001/rex/server.sock`), false},
		{"another refusal, same shape", Die(`rex block call -s session:a -b block:a program_status: Error: the daemon is shutting down`), false},
		// The neighbouring sentence, which is the SESSION-addressed verbs'
		// (`block ls -s <destroyed>`, `kill`): it says nothing about a
		// block, so rexBlockGone does not read it and SeatSaysGone does.
		{"a session that is gone", Die(`rex kill session:a: Error: session target "session:a" not found`), false},
		// A sentence about a window, which posse never addresses: `block or
		// window target` is matched as a whole and not as `window target`.
		{"a window that is gone", Die(`rex focus -s session:a window:a: Error: window target "window:a" not found`), false},
	} {
		if got := rexBlockGone(c.err); got != c.gone {
			t.Errorf("%s: rexBlockGone = %v, want %v (err: %v)", c.name, got, c.gone, c.err)
		}
	}
}

// SeatSaysGone is the occupant's question and it is the UNION of the two
// sentences, because a seat whose session the server has lost is gone as
// surely as one whose block it has. CloseSaysGone stays the narrower
// question it was: a destroy that reads a gone BLOCK as a destroyed session
// would unlink a record over a seat that is still there (ADR 0080 D10).
func TestQARexSeatSaysGoneCoversBothTargetsAndCloseSaysGoneStillDoesNot(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	block := Die(`rex block call -s session:a -b block:a program_status: Error: block target "block:a" not found`)
	session := Die(`rex kill session:a: Error: session target "session:a" not found`)
	if !m.SeatSaysGone(block) || !m.SeatSaysGone(session) {
		t.Errorf("SeatSaysGone(block)=%v SeatSaysGone(session)=%v — both sentences are the server saying it does not hold this seat", m.SeatSaysGone(block), m.SeatSaysGone(session))
	}
	if m.CloseSaysGone(block) {
		t.Error("CloseSaysGone read a gone BLOCK as a destroyed session — destroyRecordedSeat unlinks the record on that answer, and a seat whose block closed still holds a session on the server (ADR 0080 D10)")
	}
	if m.SeatSaysGone(nil) {
		t.Error("SeatSaysGone(nil) is true — a call that did not fail says nothing about the seat")
	}
}

// And the predicate against what the SERVER says, per verb, rather than
// against a line a test wrote. The fake refuses a block its `-s` session does
// not hold in the real CLI's own three spellings (fakeRexScoped, MEASURED on
// the same run), so this arm is what reds a predicate that reads one of them:
// every verb posse calls, aimed at a seat the server does not have.
func TestQAEveryRexVerbsOwnRefusalForAGoneBlockReadsAsGone(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	// The server holds the session and NOT the block — a husk, which is the
	// state an exec'd runtime's exit leaves (ADR 0080 D9 as amended).
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: "box:dev"})
	seat := rexTestSeat()

	calls := []struct {
		verb string
		run  func() error
	}{
		{"program_status", func() error { _, err := m.State(seat); return err }},
		{"capture", func() error { _, err := m.Read(seat, 5, false); return err }},
		{"send", func() error { return m.SendText(seat, "y") }},
		{"send-key", func() error { return m.SendKeys(seat, "enter") }},
		{"focus", func() error { return m.Focus(seat) }},
		{"start (send then the key)", func() error { return m.Start(seat, "claude") }},
	}
	for _, c := range calls {
		err := c.run()
		if err == nil {
			t.Errorf("%s over a block the server does not hold answered without an error — the fake is not refusing it and every arm below is passing over a live wire", c.verb)
			continue
		}
		if !m.SeatSaysGone(err) {
			t.Errorf("%s: SeatSaysGone read rex's own refusal as something else, so this verb's failure would be reported as a server that would not answer:\n%v", c.verb, err)
		}
	}
}

// ─── the three verdicts, over the new reason ─────────────────────────────────

// rexGoneSeatBackend is a backend whose Rex slot holds the seat's SESSION and
// not its block: the husk an exec'd runtime leaves when it ends, which is the
// state confirmReports reads when a launch dies before it reports.
func rexGoneSeatBackend(t *testing.T) (*HerdrBackend, Seat) {
	t.Helper()
	b, _ := newTestBackend(t)
	m, _ := newTestRexMux(t)
	b.Slot = m
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: "box:dev"})
	return b, rexTestSeat()
}

func TestQARexLaunchRefusesASeatWhoseRuntimeEndedAndSaysSoOfTheSeat(t *testing.T) {
	t.Parallel()
	b, seat := rexGoneSeatBackend(t)
	rt := builtinRuntime(t, "claude")
	if rt.PromptMode() != PromptTyped {
		t.Fatalf("this pin is the TYPED arm and claude reads %q — re-aim it", rt.PromptMode())
	}
	o := NewSessionOpts{Name: "seat-x", Bead: "ranger-base-x", Runtime: "claude"}
	err := b.confirmReports(o, &launchPlan{Runtime: "claude"}, seat)
	if err == nil {
		t.Fatal("a dispatched launch whose runtime ENDED before it reported was allowed to proceed — the seat is provably gone, which is more than the no-record arm has and is refused there (ADR 0076 §3)")
	}
	for _, want := range []string{"claude", "seat-x", "ENDED", "NOTHING WAS TYPED", "block target", "posse kill"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
	// The whole bead: the sentence must blame the seat and not the server.
	if strings.Contains(err.Error(), "would not answer") {
		t.Errorf("the refusal still says rex would not answer, over a server that answered with its own `not found` — an operator reading that goes looking at rex:\n%v", err)
	}
	// And it must not send anybody to a screen that went with the block.
	// MEASURED: `rex capture` of a closed block is the same `not found`.
	if strings.Contains(err.Error(), "posse peek") {
		t.Errorf("the refusal advises `posse peek` of a closed block, which answers rex's same refusal — the scrollback went with the block (ADR 0080's 2026-11-15 amendment, the `rex capture` row of its measured table):\n%v", err)
	}
}

func TestQAInteractiveRexLaunchOfASeatThatEndedIsToldAndKept(t *testing.T) {
	t.Parallel()
	b, seat := rexGoneSeatBackend(t)
	warn := warnBuf(t, b)
	// No Bead and no Recreate: `posse new`. The commonest thing on this arm
	// is an operator's own `--cmd` of a command that was always going to
	// finish — the MEASURED case on this bead — so it is not a refusal.
	o := NewSessionOpts{Name: "scratch", Runtime: "claude"}
	if err := b.confirmReports(o, &launchPlan{Runtime: "claude"}, seat); err != nil {
		t.Fatalf("an interactive launch was refused: %v", err)
	}
	got := warn.String()
	for _, want := range []string{"ENDED", "scratch", "block target"} {
		if !strings.Contains(got, want) {
			t.Errorf("the interactive line does not carry %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "would not answer") {
		t.Errorf("the interactive line blames the server for a seat that ended:\n%s", got)
	}
}

func TestQARexLaunchHoldsAnArgvRuntimeWhoseSeatEnded(t *testing.T) {
	t.Parallel()
	b, seat := rexGoneSeatBackend(t)
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuntime(t, b.App, "argvcli", "command: argvcli {file}\nprompt: argv\n")
	warn := warnBuf(t, b)
	o := NewSessionOpts{Name: "s-argv", Bead: "ranger-base-x", Runtime: "argvcli"}
	// The prompt rode the launch line, so the claim is KEPT — and here for
	// one reason more than D2's: a CLI that took the prompt, did the work
	// and exited leaves this exact state, because its records closed with
	// its block. Refusing would judge a turn that may have completed.
	if err := b.confirmReports(o, &launchPlan{Runtime: "argvcli"}, seat); err != nil {
		t.Fatalf("an argv runtime whose seat ended refused the launch; ADR 0076 D2 keys the cost on `prompt:` and the reason does not change the key: %v", err)
	}
	if got := warn.String(); !strings.Contains(got, "HOLD") || !strings.Contains(got, "s-argv") || !strings.Contains(got, "ENDED") {
		t.Errorf("the hold said nothing legible about what ended:\n%s", got)
	}
}

// ─── the wire ────────────────────────────────────────────────────────────────

// The pins above call b.confirmReports directly, so none of them would red a
// tree whose call site is gone — that is
// TestQARexFirstReportGateIsWiredIntoStartPlanned's job
// (muxslot_qa_test.go), measured there. What this one adds is that the NEW
// arm is reached through a real launch: plan it, start it, and read the
// verdict off startPlanned, with rex refusing the program_status read in its
// own words for a block that is gone.
func TestQARexRuntimeEndedArmIsReachedFromARealLaunch(t *testing.T) {
	t.Parallel()
	b := rexLaunchBackend(t, "50ms")
	// The read fails the way a closed block makes it fail. The create and
	// the typed line go through untouched, which is what makes the error
	// arrive where confirmReports reads it and not earlier.
	rexSeedFail(t, "program_status", `block target "`+rexTestBlock+`" not found`)
	o := NewSessionOpts{Name: "s-ended", Dir: t.TempDir(), Agent: "ranger", Runtime: "claude", Bead: "ranger-base-x"}
	p, err := b.planLaunch(o)
	if err != nil {
		t.Fatalf("planLaunch refused an ADMITTED runtime on Rex: %v", err)
	}
	ws, err := b.startPlanned(o, p)
	if err == nil {
		t.Fatal("startPlanned opened a Rex seat whose block was gone by the time the gate read it and returned no error — the ENDED arm is not on this path")
	}
	for _, want := range []string{"ENDED", "s-ended", "NOTHING WAS TYPED"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal startPlanned returned does not name %q — it may be some other launch failure standing in for this arm:\n%v", want, err)
		}
	}
	// rangerhq-v52t's contract, which this arm shares with the no-record
	// one: the workspace rides back WITH the error, because "a seat is up
	// and the gate refused it" is not "nothing exists".
	if ws == "" {
		t.Error("startPlanned refused and returned no workspace id — a caller cleaning up cannot then tell a refused launch from one that never opened a seat")
	}
}
