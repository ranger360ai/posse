//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-qlxb5 — Rex's occupant of the multiplexer slot
// (ADR 0076 §2 and §3), over the fake rex of fakerex_test.go.
//
// Claim: each of the ten verbs reaches the argv ADR 0076 §2 names it with,
// and State is the OSC 7501 record judged against the block's process table
// — which means every reading that is NOT positive evidence about a runtime
// reads MuxUnknown and never MuxIdle.
//
// THE FOUR UNKNOWN CASES ARE THE POINT OF THIS FILE, and three of them are
// records that exist:
//
//	no record at all                     ADR 0076 §3
//	a record whose owner is the shell    ADR 0076 §3
//	a record whose owner is not live     ranger-base-ch2rb, MEASURED
//	a state string nothing here knows    the protocol's advice, refused
//
// The third is the one a reader would not think to ask for: Rex drops only
// `working` and `blocked` at process exit, so an `idle` record outlives its
// writer and a State without the liveness check settles a bead on a dead
// seat.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

const (
	rexTestSession = "session:fake-0001"
	rexTestBlock   = "block:fake-0001"
)

func rexTestSeat() Seat { return Seat{Session: rexTestSession, Pane: rexTestBlock} }

// rexRuntimeProc is the measured shape of a block running a runtime: the
// shell is `child`, the runtime is `foreground`, and `ancestors` holds the
// shell between them (RexProcess's doc carries the measurement).
func rexRuntimeProc() RexProcess {
	return RexProcess{
		Child:      &RexProc{PID: 10, Name: "zsh"},
		Foreground: &RexProc{PID: 20, Name: "claude"},
		Ancestors:  []RexProc{{PID: 10, Name: "zsh"}},
	}
}

// rexShellProc is a block whose shell is at its prompt: foreground IS child,
// and `ancestors` is left out entirely (MEASURED).
func rexShellProc() RexProcess {
	return RexProcess{
		Child:      &RexProc{PID: 10, Name: "zsh"},
		Foreground: &RexProc{PID: 10, Name: "zsh"},
	}
}

// rexRuntimeRec is a record the runtime wrote: owner is the foreground of
// rexRuntimeProc.
func rexRuntimeRec(state string) RexStatusRecord {
	return RexStatusRecord{State: state, App: "claude-code", UpdatedAt: "2026-10-31T04:12:42-04:00",
		Owner: &RexOwner{PID: 20, Name: "claude"}}
}

// rexSeatWithRuntime seeds one session holding one terminal block, with the
// runtime-in-foreground process table and `sets` as the status queue.
func rexSeatWithRuntime(t *testing.T, sets ...[]RexStatusRecord) RexMux {
	t.Helper()
	m, _ := newTestRexMux(t)
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: "box:dev",
		Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Label: "shell", Flavor: RexTerminalFlavor}}})
	rexSeedProcess(t, rexTestBlock, rexRuntimeProc())
	rexSeedStatus(t, rexTestBlock, sets...)
	return m
}

func rexCallWith(t *testing.T, want ...string) string {
	t.Helper()
	for _, c := range rexCalls(t) {
		hit := true
		for _, w := range want {
			if !strings.Contains(c, w) {
				hit = false
				break
			}
		}
		if hit {
			return c
		}
	}
	t.Fatalf("no rex call contains all of %q; calls were:\n%s", want, strings.Join(rexCalls(t), "\n"))
	return ""
}

// ─── Create and Start (§2's Create row, split) ───────────────────────────────

func TestQARexCreateOpensAShellBlockWithTheEnvInItsArgv(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	seat, err := m.Create("box:dev", "/tmp/work", []EnvVar{{Key: "A", Value: "1"}, {Key: "B", Value: "two"}})
	if err != nil {
		t.Fatal(err)
	}
	if seat.Session == "" || seat.Pane == "" {
		t.Fatalf("Create gave a half seat: %+v", seat)
	}
	call := rexCallWith(t, "new", "box:dev")
	if !strings.Contains(call, "--cwd /tmp/work") {
		t.Errorf("the cwd did not reach `rex new`: %s", call)
	}
	// ADR 0076 §2: blocks take no env, so it rides the argv as `env K=V …`
	// — and it rides the SHELL's argv, so a later Start or RelaunchAgent
	// inherits it the way a herdr pane does (RexMux.Create's doc).
	cmd := rexNewCommand(t)
	want := []string{"/usr/bin/env", "A=1", "B=two", rexShell(), "-i"}
	if len(cmd) != len(want) {
		t.Fatalf("block command = %q, want %q", cmd, want)
	}
	for i := range want {
		if cmd[i] != want[i] {
			t.Fatalf("block command = %q, want %q", cmd, want)
		}
	}
	// --keep-open would make a seat whose program exited immortal: the
	// block would stay open, so nothing could ever read the end of it
	// (Rex.NewSession's doc).
	if strings.Contains(call, "--keep-open") {
		t.Errorf("`rex new` carried --keep-open: %s", call)
	}
}

func TestQARexCreateYieldsNoSeatWhenRexRefuses(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	rexSeedFail(t, "new", "no space left on device")
	seat, err := m.Create("box:dev", "/tmp/work", nil)
	if err == nil {
		t.Fatal("Create reported success over a refused `rex new`")
	}
	if seat != (Seat{}) {
		t.Errorf("Create returned a seat beside its error: %+v", seat)
	}
	if !strings.Contains(err.Error(), "no space left on device") {
		t.Errorf("rex's own sentence did not survive: %v", err)
	}
}

// The shell posse names when the launcher has none. Not parallel: it sets a
// process-wide variable (t.Setenv).
//
// Since ADR 0077 D2 (surface 4) the answer is realShell's, so an unset
// $SHELL is realShell's SEARCH — zsh, then bash, then /bin/sh — and not a
// bare /bin/sh. What this arm still pins is that the seat gets a shell posse
// can name whatever the launcher carries, including nothing: the whole
// reason rexShell exists rather than letting `rex new` pick. That the
// refusal of a gate wrapper and the honest-$SHELL arm belong to the same
// answer is pinned in seatbasepath_qa_test.go (ranger-base-zfljf), with the
// wrapper fixture.
func TestQARexShellIsNamedEvenWhenTheLauncherHasNone(t *testing.T) {
	t.Setenv("SHELL", "")
	got := rexShell()
	if got == "" {
		t.Fatal("rexShell() named no shell with no $SHELL — the seat would be opened by whatever rex picks, which is not an answer posse can type into")
	}
	if real, _ := realShell(""); got != real {
		t.Errorf("rexShell() = %q, want realShell(\"\")'s answer %q", got, real)
	}
	if st, err := os.Stat(got); err != nil || st.IsDir() {
		t.Errorf("rexShell() = %q, which is not an executable file here: %v", got, err)
	}
}

func TestQARexStartTypesTheLineAndEnter(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	if err := m.Start(rexTestSeat(), "claude --resume"); err != nil {
		t.Fatal(err)
	}
	rexCallWith(t, "send", "-b "+rexTestBlock, "claude --resume")
	rexCallWith(t, "send-key", "-b "+rexTestBlock, "enter")
}

func TestQARexCloseKillsTheSession(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: "box:dev",
		Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Flavor: RexTerminalFlavor}}})
	if err := m.Close(rexTestSeat()); err != nil {
		t.Fatal(err)
	}
	rexCallWith(t, "kill "+rexTestSession)
	rows, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("the session survived Close: %+v", rows)
	}
}

func TestQARexFocusAddressesTheBlockInItsSession(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	if err := m.Focus(rexTestSeat()); err != nil {
		t.Fatal(err)
	}
	rexCallWith(t, "focus", "-s "+rexTestSession, rexTestBlock)
}

func TestQARexSendTextAndSendKeysAddressTheBlock(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	if err := m.SendText(rexTestSeat(), "y"); err != nil {
		t.Fatal(err)
	}
	if err := m.SendKeys(rexTestSeat(), "esc", "enter"); err != nil {
		t.Fatal(err)
	}
	rexCallWith(t, "send", "-b "+rexTestBlock, " y")
	rexCallWith(t, "send-key", "-b "+rexTestBlock, "esc enter")
	// No keys is no call, not an empty send-key the CLI would refuse.
	before := len(rexCalls(t))
	if err := m.SendKeys(rexTestSeat()); err != nil {
		t.Fatal(err)
	}
	if after := len(rexCalls(t)); after != before {
		t.Errorf("SendKeys with no keys made a call")
	}
}

// ─── Read (§2's Read row) ────────────────────────────────────────────────────

func TestQARexReadTailsTextAndTakesVTWhole(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	rexSeedCapture(t, rexTestBlock, "one\ntwo\nthree\nfour")
	got, err := m.Read(rexTestSeat(), 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "three\nfour" {
		t.Errorf("Read(2) = %q, want the last two rows", got)
	}
	if call := rexCallWith(t, "capture", "--format text"); !strings.Contains(call, "--trim") {
		t.Errorf("the text capture did not --trim, so a tail reads padding: %s", call)
	}
	// ansi IGNORES lines, which is herdr's shape kept so that a caller
	// cannot get a different snapshot depending on who answered.
	got, err = m.Read(rexTestSeat(), 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "one\ntwo\nthree\nfour" {
		t.Errorf("Read(2, ansi) = %q, want the whole capture", got)
	}
	rexCallWith(t, "capture", "--format vt")
}

// ─── State (§3) ──────────────────────────────────────────────────────────────

func TestQARexStateMapsTheProtocolOntoTheSlotVocabulary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		reported string
		want     string
		why      string
	}{
		{"idle", MuxIdle, "the four posse already uses map to themselves"},
		{"working", MuxWorking, ""},
		{"done", MuxDone, ""},
		{"blocked", MuxBlocked, ""},
		{"error", MuxDone, "ADR 0076 §2: the turn ended, and the screen readers judge it"},
		{"paused", MuxUnknown, "a state nothing here knows is NOT idle, whatever the protocol advises a UI"},
	} {
		tc := tc
		t.Run(tc.reported, func(t *testing.T) {
			t.Parallel()
			m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec(tc.reported)})
			read, err := m.State(rexTestSeat())
			if err != nil {
				t.Fatal(err)
			}
			if read.State != tc.want {
				t.Errorf("state %q read as %q, want %q — %s", tc.reported, read.State, tc.want, tc.why)
			}
			if tc.want == MuxUnknown {
				return
			}
			if !read.Seen {
				t.Errorf("state %q: Seen is false over a record owned by the block's foreground, which is ADR 0076 §3's positive evidence", tc.reported)
			}
			if read.Owner != "claude(20)" {
				t.Errorf("Owner = %q, want the record's owner name and pid", read.Owner)
			}
			if !strings.Contains(read.Evidence, "app=claude-code") {
				t.Errorf("Evidence does not quote the record: %q", read.Evidence)
			}
		})
	}
}

func TestQARexStateCarriesTheBlockedKind(t *testing.T) {
	t.Parallel()
	rec := rexRuntimeRec("blocked")
	rec.Kind = "permission"
	m := rexSeatWithRuntime(t, []RexStatusRecord{rec})
	read, err := m.State(rexTestSeat())
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxBlocked || read.Kind != "permission" {
		t.Errorf("reading = %+v, want blocked/permission", read)
	}
}

func TestQARexStateIsUnknownWithNoRecord(t *testing.T) {
	t.Parallel()
	// ADR 0076 §3: no record maps to unknown, NEVER to idle.
	m := rexSeatWithRuntime(t)
	read, err := m.State(rexTestSeat())
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxUnknown {
		t.Errorf("no record read as %q, want %q", read.State, MuxUnknown)
	}
	if read.Seen {
		t.Error("Seen is true over a block that has never reported")
	}
	// And the process table is not read for a block that has never
	// reported: no record is already the answer, and that is most of the
	// blocks a List walks (RexMux.reading's doc).
	for _, c := range rexCalls(t) {
		if strings.Contains(c, " process") {
			t.Errorf("the process table was read for a block with no record: %s", c)
		}
	}
}

// ranger-base-ch2rb, MEASURED 2026-10-31: Rex drops only working and blocked
// at process exit, so an `idle` record stays behind with a dead owner pid
// while the block's foreground falls back to the shell. Reading it as idle
// settles a bead on a seat whose runtime is gone.
func TestQARexStateIsUnknownWhenTheRecordOutlivedItsWriter(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: "box:dev",
		Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Flavor: RexTerminalFlavor}}})
	// The record names a pid nothing in this block is any more.
	rexSeedStatus(t, rexTestBlock, []RexStatusRecord{rexRuntimeRec("idle")})
	rexSeedProcess(t, rexTestBlock, rexShellProc())
	read, err := m.State(rexTestSeat())
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxUnknown {
		t.Errorf("a record with a dead owner read as %q, want %q (ranger-base-ch2rb)", read.State, MuxUnknown)
	}
	if read.Seen {
		t.Error("Seen is true over a record whose writer has exited")
	}
	if !strings.Contains(read.Evidence, "outlived its writer") {
		t.Errorf("Evidence does not say why: %q", read.Evidence)
	}
}

func TestQARexStateIsUnknownForTheShellsOwnRecord(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: "box:dev",
		Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Flavor: RexTerminalFlavor}}})
	// MEASURED: a report written from the shell's prompt carries the shell
	// as its owner. ADR 0076 §3's "owner must not be the pane's shell".
	rexSeedStatus(t, rexTestBlock, []RexStatusRecord{{State: "idle", App: "shellwriter",
		Owner: &RexOwner{PID: 10, Name: "zsh"}}})
	rexSeedProcess(t, rexTestBlock, rexShellProc())
	read, err := m.State(rexTestSeat())
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxUnknown || read.Seen {
		t.Errorf("the shell's own record read as %+v, want unknown and unseen", read)
	}
	if !strings.Contains(read.Evidence, "own shell") {
		t.Errorf("Evidence does not say why: %q", read.Evidence)
	}
}

func TestQARexStateIsUnknownForAnOwnerlessRecord(t *testing.T) {
	t.Parallel()
	// ADR 0076's spike: a hook that writes to /dev/tty while the shell is
	// in the foreground lands a record with no owner at all. Nothing in it
	// says a runtime wrote it, so there is no reading in it.
	m := rexSeatWithRuntime(t, []RexStatusRecord{{State: "working", App: "codex-hook"}})
	read, err := m.State(rexTestSeat())
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxUnknown || read.Seen {
		t.Errorf("an ownerless record read as %+v, want unknown and unseen", read)
	}
}

func TestQARexStateReadsTheRootRecordAndNotAChild(t *testing.T) {
	t.Parallel()
	// A child record (`build/test`) is a task INSIDE the program, not the
	// program's own state. The root is selected by its empty id rather than
	// by position, so a listing that stops being sorted cannot promote one.
	child := rexRuntimeRec("working")
	child.ID = "build/test"
	root := rexRuntimeRec("blocked")
	m := rexSeatWithRuntime(t, []RexStatusRecord{child, root})
	read, err := m.State(rexTestSeat())
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxBlocked {
		t.Errorf("State = %q, want the root record's %q", read.State, MuxBlocked)
	}
}

func TestQARexStateIsUnknownForABlockWithOnlyChildRecords(t *testing.T) {
	t.Parallel()
	// A seat whose ONLY records are children has reported no state of its
	// own, which reads as no record — MuxUnknown, like every other gap.
	child := rexRuntimeRec("working")
	child.ID = "build/test"
	m := rexSeatWithRuntime(t, []RexStatusRecord{child})
	read, err := m.State(rexTestSeat())
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxUnknown || read.Seen {
		t.Errorf("a child-only block read as %+v, want unknown and unseen", read)
	}
}

func TestQARexStateErrorsWhenRexCannotBeAsked(t *testing.T) {
	t.Parallel()
	// MuxReading's rule: an occupant that was asked and named no state
	// reads State == ""; one that could not be asked returns an error.
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("idle")})
	rexSeedFail(t, "program_status", "block target not found")
	if _, err := m.State(rexTestSeat()); err == nil {
		t.Fatal("State reported a reading over a refused program_status")
	}
}

// ─── List (§2's List row) ────────────────────────────────────────────────────

func TestQARexListJoinsTheStatusOfEveryTerminalBlock(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	rexSeedSessions(t,
		fakeRexSession{SessionID: "session:a", Label: "box:dev", Blocks: []fakeRexBlock{
			{BlockID: "block:a1", Flavor: RexTerminalFlavor}}},
		fakeRexSession{SessionID: "session:b", Label: "box:shell", Blocks: []fakeRexBlock{
			{BlockID: "block:b1", Flavor: RexTerminalFlavor}}},
		// A session whose last block exited. MEASURED: it STAYS in
		// session.list with windows: 0, so a row for it would make the
		// seat immortal under §5's absent-from-the-listing liveness rule.
		fakeRexSession{SessionID: "session:gone", Label: "box:finished"},
	)
	rexSeedProcess(t, "block:a1", rexRuntimeProc())
	rexSeedStatus(t, "block:a1", []RexStatusRecord{rexRuntimeRec("working")})
	// b1 holds nothing but a shell, and its only record is the shell's.
	rexSeedProcess(t, "block:b1", rexShellProc())
	rexSeedStatus(t, "block:b1", []RexStatusRecord{{State: "idle", Owner: &RexOwner{PID: 10, Name: "zsh"}}})

	rows, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("List gave %d rows, want 2 (the blockless session must not be listed): %+v", len(rows), rows)
	}
	a, b := rows[0], rows[1]
	if a.Session != "session:a" || a.Label != "box:dev" {
		t.Errorf("row 0 = %+v, want session:a / box:dev", a)
	}
	if a.State != MuxWorking || a.Pane != "block:a1" || a.Kind != "claude-code" {
		t.Errorf("row 0 = %+v, want working in block:a1, kind from the record's app", a)
	}
	if len(a.Panes) != 1 || a.Panes[0].State != MuxWorking {
		t.Errorf("row 0 panes = %+v, want the one detected block", a.Panes)
	}
	// "unknown MEANS NONE", Rex's form: a block whose only record is its own
	// shell's holds no runtime, so State is "" and Panes is empty — the same
	// fact herdr's List applies to `agent_status: "unknown"`.
	if b.State != "" || b.Pane != "" || len(b.Panes) != 0 {
		t.Errorf("row 1 = %+v, want no runtime detected at all", b)
	}
	// Focused is false on every Rex row: the server has no notion of which
	// session a UI is pointed at (RexMux.List's doc).
	if a.Focused || b.Focused {
		t.Error("a Rex row claimed focus, which no Rex call can answer")
	}
}

func TestQARexListFailsRatherThanReportEverySeatEmpty(t *testing.T) {
	t.Parallel()
	// herdr's rule here for herdr's reason: a listing missing half of
	// itself would report every seat as holding no runtime, and that is the
	// one reading that must never be guessed (ADR 0013 §1).
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("working")})
	rexSeedFail(t, "block ls", "session not found")
	if _, err := m.List(); err == nil {
		t.Fatal("List answered over a block listing it could not take")
	}
}

// ─── Wait (§2's Wait row) ────────────────────────────────────────────────────

func TestQARexWaitPollsUntilTheStateArrives(t *testing.T) {
	t.Parallel()
	// Three samples: working, working, then done. The first is taken before
	// the first sleep, because `agent wait` returns on a state that is
	// already there.
	m := rexSeatWithRuntime(t,
		[]RexStatusRecord{rexRuntimeRec("working")},
		[]RexStatusRecord{rexRuntimeRec("working")},
		[]RexStatusRecord{rexRuntimeRec("done")},
	)
	read, err := m.Wait(rexTestSeat(), []string{MuxIdle, MuxDone, MuxBlocked}, 10_000)
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxDone {
		t.Errorf("Wait returned %q, want done", read.State)
	}
}

func TestQARexWaitRunningOutIsTheTimeoutCodeAndNotAReading(t *testing.T) {
	t.Parallel()
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("working")})
	start := time.Now()
	read, err := m.Wait(rexTestSeat(), []string{MuxIdle, MuxDone}, 30)
	if err == nil {
		t.Fatal("Wait returned a reading for a deadline it blew")
	}
	// The code herdr's own wait returns, because promptstall.go,
	// relaunch.go and reportedagent.go already branch on it as "this says
	// nothing about whether anything landed" (rangerhq-1z0).
	if !IsHerdrCode(err, "timeout") {
		t.Errorf("Wait's blown deadline = %v, want the `timeout` code", err)
	}
	if read.State != "" {
		t.Errorf("Wait returned a state beside its timeout: %+v", read)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("Wait took %s over a 30ms deadline", took)
	}
	if !strings.Contains(err.Error(), "working") {
		t.Errorf("the timeout does not name the last reading: %v", err)
	}
}

func TestQARexWaitWithNoUntilWaitsForTheSettledTriple(t *testing.T) {
	t.Parallel()
	// `posse wait <name>` with no --until, and `--timeout 0` beside it.
	// herdr passes no --until and lets its server decide; a poller with an
	// empty set would watch the whole deadline out and report a timeout
	// over a seat that had already settled.
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("idle")})
	start := time.Now()
	read, err := m.Wait(rexTestSeat(), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxIdle {
		t.Errorf("Wait(nil, 0) = %q, want the settled reading that was already there", read.State)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("Wait(nil, 0) took %s — it watched its default deadline out instead of matching", took)
	}
}

func TestQARexWaitEndsOnARexThatCannotBeAsked(t *testing.T) {
	t.Parallel()
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("working")})
	rexSeedFail(t, "program_status", "server not running")
	_, err := m.Wait(rexTestSeat(), []string{MuxIdle}, 10_000)
	if err == nil {
		t.Fatal("Wait answered over a rex it could not ask")
	}
	if IsHerdrCode(err, "timeout") {
		t.Error("a rex that could not be asked was reported as a blown deadline — the two mean opposite things")
	}
}

// ─── Prompt (§2's Prompt row) ────────────────────────────────────────────────

func TestQARexPromptTypesThenWaitsForTheSettle(t *testing.T) {
	t.Parallel()
	m := rexSeatWithRuntime(t,
		[]RexStatusRecord{rexRuntimeRec("working")}, // the stall window's sample
		[]RexStatusRecord{rexRuntimeRec("idle")},    // the settle
	)
	read, err := m.Prompt(rexTestSeat(), "do the thing", true, 10_000)
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxIdle {
		t.Errorf("Prompt settled on %q, want idle", read.State)
	}
	rexCallWith(t, "send", "-b "+rexTestBlock, "do the thing")
	rexCallWith(t, "send-key", "-b "+rexTestBlock, "enter")
}

func TestQARexPromptWithoutWaitClaimsOnlyTheKeystrokes(t *testing.T) {
	t.Parallel()
	m := rexSeatWithRuntime(t)
	read, err := m.Prompt(rexTestSeat(), "do the thing", false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if read.State != "" {
		t.Errorf("a no-wait prompt reported state %q; nothing has been observed since the keystroke", read.State)
	}
	if !strings.Contains(read.Evidence, "enter") {
		t.Errorf("Evidence does not name the delivery: %q", read.Evidence)
	}
}

func TestQARexPromptStallIsAgentPromptStalled(t *testing.T) {
	t.Parallel()
	// No turn inside herdr's own five seconds. The code matters: dispatch
	// reads it as "the submission was ACCEPTED and what is missing is an
	// observation" (promptstall.go's taxonomy), which is exactly true here.
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("idle")})
	// A caller patience larger than the stall window, so the stall arm is
	// the one that fires.
	if _, err := m.Prompt(rexTestSeat(), "do the thing", true, 60_000); !IsHerdrCode(err, "agent_prompt_stalled") {
		t.Fatalf("a prompt that started no turn = %v, want agent_prompt_stalled", err)
	}
}

func TestQARexPromptACallersShorterPatienceIsATimeoutNotAStall(t *testing.T) {
	t.Parallel()
	// reportedagent.go's rule, kept: "a caller timeout that expires first
	// returns timeout". The two mean opposite things to dispatch and the
	// shorter of the two is what actually expired.
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("idle")})
	_, err := m.Prompt(rexTestSeat(), "do the thing", true, 30)
	if IsHerdrCode(err, "agent_prompt_stalled") {
		t.Fatalf("a caller's own 30ms deadline was reported as a stall: %v", err)
	}
	if !IsHerdrCode(err, "timeout") {
		t.Fatalf("Prompt = %v, want the caller's own `timeout`", err)
	}
}

func TestQARexPromptBlockedShortCircuitsTheSettleWait(t *testing.T) {
	t.Parallel()
	// A turn that went straight to blocked has settled on the one state the
	// second wait would answer instantly anyway. One status read, not two.
	rec := rexRuntimeRec("blocked")
	rec.Kind = "permission"
	m := rexSeatWithRuntime(t, []RexStatusRecord{rec})
	read, err := m.Prompt(rexTestSeat(), "do the thing", true, 60_000)
	if err != nil {
		t.Fatal(err)
	}
	if read.State != MuxBlocked || read.Kind != "permission" {
		t.Errorf("Prompt = %+v, want the blocked reading that saw it", read)
	}
}

func TestQARexPromptRefusesALeadingSlashAndSendsNothing(t *testing.T) {
	t.Parallel()
	// ADR 0060 D5: the picker eats the Enter and the text sits in the
	// composer while the call reports success. On Rex there is no envelope
	// and no detection to catch it with, so the refusal is the whole guard.
	m := rexSeatWithRuntime(t)
	if _, err := m.Prompt(rexTestSeat(), "  /compact", true, 10_000); err == nil {
		t.Fatal("a '/'-leading prompt was accepted")
	}
	for _, c := range rexCalls(t) {
		if strings.Contains(c, "send") {
			t.Errorf("the refusal still typed: %s", c)
		}
	}
}

// ─── Server (§5) ─────────────────────────────────────────────────────────────

func TestQARexServerIsTheEndpointWithNoGeneration(t *testing.T) {
	t.Parallel()
	m, dir := newTestRexMux(t)
	if err := os.WriteFile(filepath.Join(dir, "rex-endpoint"), []byte("unix:///x/rex/server.sock"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := m.Server()
	if srv.Name != MuxRex {
		t.Errorf("Server().Name = %q, want %q — it is what a record's `mux:` carries", srv.Name, MuxRex)
	}
	if srv.Addr != "unix:///x/rex/server.sock" {
		t.Errorf("Server().Addr = %q, want the rex server endpoint", srv.Addr)
	}
	// ADR 0076 §5: Rex's ids are UUIDs and are never reused, so there is
	// nothing for an identity fence to tell apart — and a fence posse does
	// not need is a second reason a live seat can be withheld from its own
	// name (ranger-base-q5gc2 §3).
	if srv.Gen != "" {
		t.Errorf("Server().Gen = %q, want empty on Rex", srv.Gen)
	}
}

func TestQARexServerCannotNameItsServerRatherThanGuessOne(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	rexSeedFail(t, "whoami", "server not running")
	if addr := m.Server().Addr; addr != "" {
		t.Errorf("Server().Addr = %q over a rex that would not answer; "+
			"MuxServer documents \"\" as \"this pass cannot name its server\", and it is never read as a match", addr)
	}
}

// ─── the autostart hazard ────────────────────────────────────────────────────

func TestQARexReadsRefuseAutostartAndOnlyCreateAllowsIt(t *testing.T) {
	t.Parallel()
	// `--autostart` defaults TRUE and starts a MISSING server, which holds
	// no sessions. A read that autostarted one would answer "no such
	// session" for every seat posse has recorded — and §5 reads absence
	// from the listing as death, so one such read could land and reclaim a
	// whole fleet. Every read says --autostart=false; only Create, which is
	// posse asking for a seat, lets the default stand (rex.go's head
	// comment, item 3).
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("idle")})
	rexSeedCapture(t, rexTestBlock, "screen")
	if _, err := m.List(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.State(rexTestSeat()); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Read(rexTestSeat(), 10, false); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(rexTestSeat()); err != nil {
		t.Fatal(err)
	}
	m.Server()
	for _, c := range rexCalls(t) {
		if !strings.Contains(c, "--autostart=false") {
			t.Errorf("a read ran with autostart left on: %s", c)
		}
	}
	if _, err := m.Create("box:dev", "/tmp", nil); err != nil {
		t.Fatal(err)
	}
	create := rexCallWith(t, "new", "box:dev")
	if strings.Contains(create, "--autostart=false") {
		t.Errorf("Create refused to start a missing server: %s", create)
	}
}

// ─── the binary, which is not on PATH ────────────────────────────────────────

// Not parallel: it sets a process-wide variable (t.Setenv).
func TestQARexBinPrefersTheEnvThenTheConfigKeyThenTheInstallPath(t *testing.T) {
	home := t.TempDir()
	a := NewAppAt(home)

	t.Setenv(RexBinEnv, "")
	if got, want := a.RexBin(), RexDefaultBin(); got != want {
		t.Errorf("RexBin with nothing set = %q, want the install path %q", got, want)
	}
	if err := os.WriteFile(a.ConfigPath, []byte(RexBinKey+": ~/elsewhere/rex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := a.RexBin(), ExpandTilde("~/elsewhere/rex"); got != want {
		t.Errorf("RexBin from %s: = %q, want %q (the ~ expanded)", RexBinKey, got, want)
	}
	t.Setenv(RexBinEnv, "/tmp/fake-rex")
	if got := a.RexBin(); got != "/tmp/fake-rex" {
		t.Errorf("RexBin with %s set = %q, want the env override", RexBinEnv, got)
	}
}

// BOTH ARMS OF THE PLATFORM DEFAULT, BY PARAMETER (ranger-base-izgm6, ADR
// 0080 Context, "Rex's binary"). rexDefaultBinAt takes the OS and the home
// rather than reading runtime.GOOS and $HOME, so there is no build tag here
// and neither arm is reachable from only one box: this test proves the Linux
// arm on darwin, and proves darwin's arm under `make test-linux`. Before
// that split the off-darwin default was the macOS app bundle path under
// $HOME — a path no Linux box has, on the host ADR 0080 moves posse to.
func TestQARexDefaultBinIsThePlatformsInstallPath(t *testing.T) {
	t.Parallel()
	home := t.TempDir()

	if got, want := rexDefaultBinAt("darwin", home), filepath.Join(home, rexDefaultBinRel); got != want {
		t.Errorf("darwin default = %q, want the app bundle's CLI under $HOME %q", got, want)
	}
	if got := rexDefaultBinAt("darwin", ""); got != "" {
		t.Errorf("darwin with no $HOME = %q, want \"\" and not a path rooted at the filesystem root", got)
	}

	for _, goos := range []string{"linux", "freebsd", "openbsd", "windows"} {
		got := rexDefaultBinAt(goos, home)
		if got != rexPackageBin {
			t.Errorf("%s default = %q, want the Linux package's path %q", goos, got, rexPackageBin)
		}
		if strings.Contains(got, home) {
			t.Errorf("%s default joined $HOME: %q", goos, got)
		}
	}
	// Off darwin $HOME cannot matter, because the package path is absolute
	// and joins nothing: an unset home is not the empty-path refusal darwin
	// needs, it is simply unread.
	if got := rexDefaultBinAt("linux", ""); got != rexPackageBin {
		t.Errorf("linux with no $HOME = %q, want %q", got, rexPackageBin)
	}

	// A PATH, NOT A NAME TO LOOK UP. `rex` on a Debian-family host is an
	// unrelated Perl remote-execution tool, so an exec.LookPath fallback
	// would not fail on a box without Rex — it would hand posse a different
	// program. The default must be absolute for the stat in Available to be
	// the whole question.
	if !filepath.IsAbs(rexPackageBin) {
		t.Errorf("the off-darwin default is not absolute: %q — a bare name would resolve through PATH", rexPackageBin)
	}

	// And the exported door is that same function at this box's GOOS and
	// $HOME, so RexDefaultBin cannot drift from the arms above.
	if got, want := RexDefaultBin(), rexDefaultBinAt(runtime.GOOS, os.Getenv("HOME")); got != want {
		t.Errorf("RexDefaultBin() = %q, want this platform's arm %q", got, want)
	}
}

func TestQARexAvailableIsAStatAndDegradesToUnknown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if (Rex{}).Available() {
		t.Error("an unset binary reported available")
	}
	missing := filepath.Join(dir, "nope")
	if (Rex{Bin: missing}).Available() {
		t.Error("a binary that is not there reported available")
	}
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if (Rex{Bin: plain}).Available() {
		t.Error("a non-executable file reported available")
	}
	if err := os.Chmod(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if !(Rex{Bin: plain}).Available() {
		t.Error("an executable file reported unavailable")
	}
	// A directory is not a binary; a path under a file is a stat error
	// that is NOT IsNotExist, which degrades to UNKNOWN — true — rather
	// than to a "no" that would refuse a launch (ADR 0013 §1).
	if (Rex{Bin: dir}).Available() {
		t.Error("a directory reported available")
	}
	if !(Rex{Bin: filepath.Join(plain, "under-a-file")}).Available() {
		t.Error("a stat that failed for a reason other than absence read as a NO rather than as unknown")
	}
}

// ─── the slot itself ─────────────────────────────────────────────────────────

func TestQARexIsTheSecondOccupantOfTheSlot(t *testing.T) {
	t.Parallel()
	// ADR 0075 §2 and ADR 0076 §1: the interface is the boundary, and §4's
	// herdr-only verbs are reached by a type assertion that must FAIL here
	// — a failed assertion is "not available" and never "no".
	var slot Mux = RexMux{}
	if _, ok := slot.(HerdrMux); ok {
		t.Fatal("RexMux satisfied the herdr assertion, so §4's herdr-only verbs would be asked of Rex")
	}
	if _, ok := slot.(ManifestReader); ok {
		t.Error("RexMux answers the detection preflight, which is herdr's own (ADR 0076 §4)")
	}
}
