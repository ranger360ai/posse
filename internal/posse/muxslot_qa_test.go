//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-phq39 — the `multiplexer:` instance key, and the
// two gates ADR 0076 §3 puts around a Rex launch.
//
// THE CLAIM, in three parts:
//
//  1. One key names the occupant, it defaults to herdr, and a value that is
//     neither occupant refuses the LAUNCH while the read paths keep working.
//     (App.Multiplexer / App.MultiplexerName, NewHerdrBackend.)
//  2. ADMISSION is a declaration read before the claim (D1): Runtime
//     .ReportsOnRex, unreachable from any yaml, and the refusal names herdr
//     as the way to run the runtime. Since Amendment 2's D9 it is TWO
//     declarations — the bool, and the release a ROUTED report was measured
//     on (RexVerified, codex) — both read at the same two sites and before
//     the same claim.
//  3. CONFIRMATION is the first-report wait after the launch (D2), and what a
//     miss COSTS is keyed on `prompt:` — a refusal with nothing typed for a
//     typed runtime, a HOLD for an argv one, and never a refusal when rex
//     itself could not be asked.
//
// The arm that is easiest to lose is 2: a try-and-see refusal would look
// identical on claude and would spend a claim, a worktree and a billed turn
// on grok before it said no. So the pins below assert the ORDER — that the
// refusal is reachable with no Mux call at all — and not only the message.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── the key (ADR 0076 §6) ───────────────────────────────────────────────────

func TestQAMultiplexerDefaultsToHerdrAndRefusesAThirdName(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())

	// Absent is herdr, which is what keeps every meta file and every home on
	// this box reading unchanged (§6, HerdrMeta.Mux).
	got, err := a.Multiplexer()
	if err != nil || got != MuxHerdr {
		t.Fatalf("an unset %s: = (%q, %v), want (%q, nil)", MultiplexerKey, got, err, MuxHerdr)
	}
	for _, want := range []string{MuxHerdr, MuxRex} {
		if err := os.WriteFile(a.ConfigPath, []byte(MultiplexerKey+": "+want+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := a.Multiplexer(); err != nil || got != want {
			t.Fatalf("%s: %s = (%q, %v), want (%q, nil)", MultiplexerKey, want, got, err, want)
		}
	}
	// A third name is a refusal and never a fallback: the fallback is herdr,
	// so an instance that believed it was on Rex would open a herdr workspace
	// and stamp the record with the occupant that really answered.
	if err := os.WriteFile(a.ConfigPath, []byte(MultiplexerKey+": rexx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = func() error { _, e := a.Multiplexer(); return e }()
	if err == nil {
		t.Fatal("an unrecognised multiplexer name was accepted — the fallback is herdr, so a typo would silently un-Rex the instance")
	}
	for _, want := range []string{"rexx", MuxHerdr, MuxRex, a.ConfigPath} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
	// And the read paths keep working over the same bad value, which is
	// App.Instance's rule: a listing must not die of a config typo.
	if got := a.MultiplexerName(); got != MuxHerdr {
		t.Errorf("MultiplexerName over a malformed key = %q, want %q — a listing has to keep working, and the launch is where it is refused", got, MuxHerdr)
	}
}

func TestQABackendTakesItsOccupantFromTheKeyAndLeavesHerdrUntouched(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := NewAppAt(home)

	// The default home: Slot stays NIL, which is the mechanical form of
	// "herdr's code paths do not change" — Mux() then builds HerdrMux over
	// the backend's OWN H, exactly as it did before the key existed.
	if b := NewHerdrBackend(a); b.Slot != nil {
		t.Fatalf("a home with no %s key got Slot %T — herdr must stay the nil default so every backend built without the key is byte-identical", MultiplexerKey, b.Slot)
	}
	if err := os.WriteFile(a.ConfigPath, []byte(MultiplexerKey+": "+MuxRex+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := NewHerdrBackend(a)
	rm, ok := b.Mux().(RexMux)
	if !ok {
		t.Fatalf("%s: %s gave the backend a %T — the key is the only thing that selects an occupant", MultiplexerKey, MuxRex, b.Mux())
	}
	if rm.R.Bin != a.RexBin() {
		t.Errorf("the Rex occupant was built with bin %q, want this instance's %q", rm.R.Bin, a.RexBin())
	}
	// And the herdr-only doors (§4) are shut, which is what makes the §4
	// callers' "not available" arms run rather than a nil-pointer panic.
	if _, ok := b.herdrMux(); ok {
		t.Error("a Rex backend still answered the herdr-only door — §4's verbs must read 'not available' there")
	}
}

// ─── admission: the declaration (ADR 0076 D1) ────────────────────────────────

func TestQARexAdmitsOnlyTheRuntimeMeasuredToReportThere(t *testing.T) {
	t.Parallel()
	// codex flipped under ranger-base-8gh58 (ADR 0076 Amendment 2, D9): its
	// record is posse's own reporter on codex's events, which is the same
	// evidence class claude's native one is (ADR 0013 §1). grok's own flip is
	// Amendment 3's and bob stays refused (D11).
	want := map[string]bool{"claude": true, "codex": true, "grok": false, "bob": false}
	seen := map[string]bool{}
	for i := range builtinRuntimes {
		// A COPY, never &builtinRuntimes[i]. Taking the table's address in a
		// test file marks the var written for cmd/testparallel, which would
		// strip t.Parallel's clearance off every other test that reads the
		// built-ins (fourteen of them, measured).
		rt := builtinRuntimes[i]
		seen[rt.Name] = true
		w, named := want[rt.Name]
		if !named {
			t.Fatalf("built-in %q is not in this pin's table — a new built-in has to be MEASURED in Rex before it is admitted or refused there (ADR 0076 §3)", rt.Name)
		}
		if got := RexAdmits(&rt); got != w {
			t.Errorf("RexAdmits(%s) = %v, want %v — the value is a measurement (ADR 0076 §3 and its Spike/Amendment), not a preference", rt.Name, got, w)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("this pin's table names %q and the built-in table does not — re-aim the pin", name)
		}
	}
	if RexAdmits(nil) {
		t.Error("a nil runtime was admitted on Rex")
	}
}

func TestQANoRuntimeYamlCanClaimItReportsOnRex(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	// Every spelling an author might reach for, declared at once. None is a
	// key, so none may set the bool: ReportsOnRex is a property of the
	// BINARY, measured by launching it in Rex, and a profile that could
	// claim it without that measurement would buy back the exact cost D1
	// removes.
	rt := writeRuntime(t, a, "mycli", "command: mycli {file}\n"+
		"reports_on_rex: true\nreports_on_rex_why: measured by me\nrex: true\nmultiplexer: rex\n"+
		"rex_route: "+RexRouteCodexHooks+"\nrex_verified: mycli 1.0\n")
	if rt.ReportsOnRex || RexAdmits(rt) {
		t.Fatal("a runtimes/*.yaml set ReportsOnRex — it is built-in only (ADR 0076 D1), because a declaration without the measurement is the try-and-see refusal in another costume")
	}
	// The two keys Amendment 2 added are built-in only for the same reason
	// and for one more each: a route would have posse typing hook
	// declarations at a CLI nobody asked `hooks/list`, and a version pin a
	// profile could move is the admission a profile could move (D9).
	if rt.RexRoute != "" || rt.RexVerified != "" {
		t.Fatalf("a runtimes/*.yaml set the Rex route/version: route=%q verified=%q — both are built-in only (ADR 0076 Amendment 2, D6/D9)", rt.RexRoute, rt.RexVerified)
	}
	// An OVERLAY onto a built-in keeps the built-in's measured answer: it is
	// still that binary.
	over := writeRuntime(t, a, "claude", "startup_wait: 30s\n")
	if !RexAdmits(over) {
		t.Error("an overlay onto the claude built-in lost ReportsOnRex — an overlay declares INSTANCE facts and the binary is the same one that was measured")
	}
	// And the same for the pair, on the one built-in that carries them: an
	// overlay that dropped the route would type no reporter onto a codex
	// line and leave the seat unreadable; one that dropped the version pin
	// would admit every future codex without a re-measurement.
	cx := writeRuntime(t, a, "codex", "startup_wait: 30s\n")
	if cx.RexRoute != RexRouteCodexHooks || cx.RexVerified == "" {
		t.Errorf("an overlay onto the codex built-in lost its Rex route/version: route=%q verified=%q", cx.RexRoute, cx.RexVerified)
	}
}

func TestQARexRuntimeRefusalNamesTheCauseAndHerdrAsTheDoor(t *testing.T) {
	t.Parallel()
	rt := builtinRuntime(t, "grok")
	err := RexRuntimeRefusal(rt)
	if err == nil {
		t.Fatal("RexRuntimeRefusal returned nil")
	}
	// ADR 0076 §3 in its own words: "a message that names herdr as the way
	// to run that runtime".
	for _, want := range []string{"grok", "herdr", MultiplexerKey + ": " + MuxRex, "ADR 0076", "before the claim"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the Rex launch refusal does not name %q:\n%v", want, err)
		}
	}
	// It never sends the operator to a session, because refusing here is
	// exactly the decision that no session exists yet.
	if strings.Contains(err.Error(), "posse peek") {
		t.Errorf("the pre-claim refusal points at a session, and the whole point of its placement is that there is none:\n%v", err)
	}
	if warn := RexRuntimeDegraded(rt); !strings.Contains(warn, "grok") || !strings.Contains(warn, "herdr") {
		t.Errorf("the interactive warning does not carry the same two facts:\n%s", warn)
	}
}

// builtinRuntime is one built-in by name, as a COPY — see the note in the
// admission table above for why the table's own address is never taken from
// a test file.
func builtinRuntime(t *testing.T, name string) *Runtime {
	t.Helper()
	for i := range builtinRuntimes {
		if builtinRuntimes[i].Name == name {
			rt := builtinRuntimes[i]
			return &rt
		}
	}
	t.Fatalf("no built-in runtime %q", name)
	return nil
}

// ─── confirmation: the first-report wait (ADR 0076 §3, D2) ───────────────────

// rexFirstReportBackend is a backend whose slot is the fake Rex, with one
// seeded seat. The status QUEUE is the sequence AwaitFirstReport walks.
func rexFirstReportBackend(t *testing.T, proc RexProcess, sets ...[]RexStatusRecord) (*HerdrBackend, Seat) {
	t.Helper()
	b, _ := newTestBackend(t)
	m, _ := newTestRexMux(t)
	b.Slot = m
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: "box:dev",
		Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Label: "shell", Flavor: RexTerminalFlavor}}})
	rexSeedProcess(t, rexTestBlock, proc)
	rexSeedStatus(t, rexTestBlock, sets...)
	return b, rexTestSeat()
}

func TestQARexFirstReportWaitReturnsOnTheRuntimesOwnRecord(t *testing.T) {
	t.Parallel()
	// Two silences and then the record: the wait has to poll, not sample.
	m, _ := newTestRexMux(t)
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: "box:dev",
		Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Label: "shell", Flavor: RexTerminalFlavor}}})
	rexSeedProcess(t, rexTestBlock, rexRuntimeProc())
	rexSeedStatus(t, rexTestBlock, nil, nil, []RexStatusRecord{rexRuntimeRec(MuxIdle)})
	read, reported, err := m.AwaitFirstReport(rexTestSeat(), 5*time.Second)
	if err != nil || !reported {
		t.Fatalf("AwaitFirstReport over a seat that reports on the third poll = (%+v, %v, %v)", read, reported, err)
	}
	if read.State != MuxIdle || !read.Seen {
		t.Errorf("the reading handed back is %+v, want the runtime's own idle with Seen set", read)
	}
}

func TestQARexFirstReportWaitIsNotSatisfiedByTheBlocksOwnShell(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: "box:dev",
		Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Label: "shell", Flavor: RexTerminalFlavor}}})
	// The shell is at its prompt and wrote the record itself — ADR 0076 §3's
	// "owner must not be the pane's shell", which is the one way a seat can
	// hold a record and still have nothing to say about a runtime.
	rexSeedProcess(t, rexTestBlock, rexShellProc())
	rexSeedStatus(t, rexTestBlock, []RexStatusRecord{{State: MuxIdle, App: "zsh", Owner: &RexOwner{PID: 10, Name: "zsh"}}})
	read, reported, err := m.AwaitFirstReport(rexTestSeat(), 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if reported {
		t.Fatal("a record the block's own SHELL wrote satisfied the launch gate — that is positive evidence about the plumbing, not about a runtime (ADR 0076 §3)")
	}
	if read.State != MuxUnknown {
		t.Errorf("the last reading is %+v, want MuxUnknown", read)
	}
}

func TestQARexLaunchRefusesATypedRuntimeThatNeverReportedAndTypesNothing(t *testing.T) {
	t.Parallel()
	// A seat whose only record is its own shell's: the runtime's line was
	// typed and nothing of the runtime ever answered.
	b, seat := rexFirstReportBackend(t, rexShellProc(),
		[]RexStatusRecord{{State: MuxIdle, App: "zsh", Owner: &RexOwner{PID: 10, Name: "zsh"}}})
	rt := builtinRuntime(t, "claude")
	if rt.PromptMode() != PromptTyped {
		t.Fatalf("this pin is the TYPED arm and claude reads %q — re-aim it", rt.PromptMode())
	}
	o := NewSessionOpts{Name: "seat-x", Bead: "ranger-base-x", Runtime: "claude"}
	err := b.confirmReports(o, &launchPlan{Runtime: "claude"}, seat)
	if err == nil {
		t.Fatal("a dispatched launch whose runtime never reported was allowed to proceed — on Rex that is a seat posse cannot read at all (ADR 0076 §3)")
	}
	for _, want := range []string{"claude", "seat-x", "NOTHING WAS TYPED", "herdr", "posse peek"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the first-report refusal does not name %q:\n%v", want, err)
		}
	}
}

func TestQARexLaunchHoldsAnArgvRuntimeInsteadOfRefusingIt(t *testing.T) {
	t.Parallel()
	b, seat := rexFirstReportBackend(t, rexShellProc(),
		[]RexStatusRecord{{State: MuxIdle, App: "zsh", Owner: &RexOwner{PID: 10, Name: "zsh"}}})
	// The arm D2 writes for the runtimes D1 does not admit today: the work
	// prompt rode the launch line, so the claim is KEPT and nothing is
	// judged. Refusing here would judge a turn that may well be running.
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuntime(t, b.App, "argvcli", "command: argvcli {file}\nprompt: argv\n")
	warn := warnBuf(t, b)
	o := NewSessionOpts{Name: "s-argv", Bead: "ranger-base-x", Runtime: "argvcli"}
	if err := b.confirmReports(o, &launchPlan{Runtime: "argvcli"}, seat); err != nil {
		t.Fatalf("an argv runtime's missing report refused the launch; ADR 0076 D2 makes it a HOLD, because the prompt has already been delivered: %v", err)
	}
	if got := warn.String(); !strings.Contains(got, "HOLD") || !strings.Contains(got, "s-argv") {
		t.Errorf("the hold said nothing legible:\n%s", got)
	}
}

func TestQAInteractiveRexLaunchWarnsWhereADispatchedOneRefuses(t *testing.T) {
	t.Parallel()
	b, seat := rexFirstReportBackend(t, rexShellProc(),
		[]RexStatusRecord{{State: MuxIdle, App: "zsh", Owner: &RexOwner{PID: 10, Name: "zsh"}}})
	warn := warnBuf(t, b)
	// No Bead and no Recreate: `posse new`. ADR 0013 §1 rule 5 — the
	// operator is at the keyboard, and looking at the screen is how a
	// runtime nobody has measured on Rex gets measured.
	o := NewSessionOpts{Name: "scratch", Runtime: "claude"}
	if err := b.confirmReports(o, &launchPlan{Runtime: "claude"}, seat); err != nil {
		t.Fatalf("an interactive launch was refused: %v", err)
	}
	if got := warn.String(); !strings.Contains(got, "DEGRADED") || !strings.Contains(got, "scratch") {
		t.Errorf("the interactive warning said nothing legible:\n%s", got)
	}
}

func TestQARexThatCannotBeAskedIsIgnoranceAndNeverARefusal(t *testing.T) {
	t.Parallel()
	b, seat := rexFirstReportBackend(t, rexRuntimeProc())
	rexSeedFail(t, "program_status", "server unreachable")
	warn := warnBuf(t, b)
	o := NewSessionOpts{Name: "s-err", Bead: "ranger-base-x", Runtime: "claude"}
	if err := b.confirmReports(o, &launchPlan{Runtime: "claude"}, seat); err != nil {
		t.Fatalf("a rex call that FAILED was read as 'the runtime did not report' and refused the launch — unknown is not no (ADR 0013 §1): %v", err)
	}
	if got := warn.String(); !strings.Contains(got, "could not be confirmed") {
		t.Errorf("the gate stood down without saying so:\n%s", got)
	}
}

func TestQAHerdrLaunchReachesTheFirstReportGateAtAll(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	// Slot nil = herdr. The gate's door is the type assertion, so a herdr
	// launch pays nothing: no rex call, no extra second, no line.
	if err := b.confirmReports(NewSessionOpts{Name: "s", Bead: "b", Runtime: "claude"}, &launchPlan{Runtime: "claude"}, Seat{Session: "w1", Pane: "p1"}); err != nil {
		t.Fatalf("a herdr launch was refused by Rex's first-report gate: %v", err)
	}
	if calls := rexCalls(t); len(calls) != 0 {
		t.Errorf("a herdr launch made %d rex call(s):\n%s", len(calls), strings.Join(calls, "\n"))
	}
	if got := warnBuf(t, b).String(); got != "" {
		t.Errorf("a herdr launch printed a multiplexer line:\n%s", got)
	}
}

// ─── the WIRE, which no pin above holds (ranger-base-metqp, finding 3) ───────

// rexLaunchBackend is a `multiplexer: rex` instance with a persona and a
// claude overlay, ready for a real planLaunch/startPlanned pass.
//
// THE OVERLAY IS `startup_wait:` AND NOTHING ELSE, for two reasons. It is
// what makes a pin of the refusal cost 0.5s instead of the 45s default
// (DefaultStartupWait) that the direct-call pins above each pay. And it
// leaves the ADMISSION untouched: an overlay onto a built-in keeps that
// built-in's measured ReportsOnRex, which is pinned next door in
// TestQANoRuntimeYamlCanClaimItReportsOnRex — so claude is still admitted
// here, and the gate under test is D2's and not D1's.
func rexLaunchBackend(t *testing.T, wait string) *HerdrBackend {
	t.Helper()
	b, _ := newTestBackend(t)
	if err := os.WriteFile(b.App.ConfigPath, []byte(MultiplexerKey+": "+MuxRex+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.Slot, _ = newTestRexMux(t)
	writePersona(t, b.App, "ranger", "[go]")
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuntime(t, b.App, "claude", "startup_wait: "+wait+"\n")
	return b
}

// Every pin above calls b.confirmReports DIRECTLY, so all fifteen of them
// stay green over a tree whose ONE call site (herdrback.go, startPlanned) has
// been deleted. MEASURED by ranger-base-grj2i, 2026-10-31: with the call
// replaced by `if err := error(nil); err != nil {`, the fifteen muxslot pins
// and the whole of internal/posse arm 1 both came back green, in 493.2s. A
// Rex launch would then never wait for a first report and never refuse a seat
// that wrote none, which is the entire defect ADR 0076 D2 exists for.
// RE-MEASURED under ranger-base-metqp with that same mutant: this pin's
// first arm reds.
//
// So this one drives a LAUNCH: plan it, start it, and read the verdict off
// startPlanned. It is the only pin in this file that creates a Rex session.
//
// IT ALSO PINS THE ORDER, which the direct calls cannot see: the gate's
// program_status read has to come after the launch line was typed, because
// the record is written by a process that does not exist until then
// (confirmReports' own doc). A gate hoisted above the Start would read the
// seat before the runtime could possibly have reported and refuse every
// launch on this instance.
func TestQARexFirstReportGateIsWiredIntoStartPlanned(t *testing.T) {
	t.Parallel()
	t.Run("a seat that never reports refuses the launch it just opened", func(t *testing.T) {
		t.Parallel()
		b := rexLaunchBackend(t, "50ms")
		// Nothing seeded: the fake's program_status answers `{"records":[]}`
		// for a block with no queue, which is a seat that wrote nothing.
		o := NewSessionOpts{Name: "s-wire", Dir: t.TempDir(), Agent: "ranger", Runtime: "claude", Bead: "ranger-base-x"}
		p, err := b.planLaunch(o)
		if err != nil {
			t.Fatalf("planLaunch refused an ADMITTED runtime on Rex — this pin is D2's and D1 has already said yes: %v", err)
		}
		if p.Cmd == "" {
			t.Fatal("the plan carries no command, so startPlanned types nothing and the gate is behind a branch this fixture never takes")
		}
		ws, err := b.startPlanned(o, p)
		if err == nil {
			t.Fatal("startPlanned opened a Rex seat for a runtime that never reported and returned no error — confirmReports is not on this path, so ADR 0076 D2's gate is disconnected and every pin of it above is passing over a dead wire")
		}
		for _, want := range []string{"claude", "s-wire", "NOTHING WAS TYPED", "herdr", "posse peek"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal startPlanned returned does not name %q — it may be some other launch failure standing in for the gate:\n%v", want, err)
			}
		}
		// rangerhq-v52t's contract: the workspace is handed back WITH the
		// error, because "a seat is up and the gate refused it" is not the
		// same state as "nothing exists" and must not be written over.
		if ws == "" {
			t.Error("startPlanned refused and returned no workspace id — a caller cleaning up cannot then tell a refused launch from one that never opened a seat")
		}
		// The order. `send` is the typed line, and every program_status read
		// must follow it.
		calls := rexCalls(t)
		typed, read := -1, -1
		for i, c := range calls {
			if strings.Contains(c, " send ") && typed < 0 {
				typed = i
			}
			if strings.Contains(c, "program_status") && read < 0 {
				read = i
			}
		}
		if typed < 0 || read < 0 || read < typed {
			t.Errorf("the first program_status read is at %d and the typed line at %d — the gate CONFIRMS a launch and so reads after it (ADR 0076 §3); above the Start there is nothing that could have reported yet:\n%s", read, typed, strings.Join(calls, "\n"))
		}
	})

	t.Run("a seat that reports is launched", func(t *testing.T) {
		t.Parallel()
		b := rexLaunchBackend(t, "5s")
		// The block the fake's `new` will mint for the first session of this
		// home, seeded before the create so the record is already there when
		// the gate reads it. This is the two-way half: without it a gate
		// wired to refuse EVERYTHING would pass the arm above.
		rexSeedProcess(t, rexTestBlock, rexRuntimeProc())
		rexSeedStatus(t, rexTestBlock, []RexStatusRecord{rexRuntimeRec(MuxIdle)})
		o := NewSessionOpts{Name: "s-ok", Dir: t.TempDir(), Agent: "ranger", Runtime: "claude", Bead: "ranger-base-x"}
		p, err := b.planLaunch(o)
		if err != nil {
			t.Fatal(err)
		}
		ws, err := b.startPlanned(o, p)
		if err != nil {
			t.Fatalf("a Rex launch whose runtime DID report was refused — the gate is a confirmation and a seat that answered is a launch: %v", err)
		}
		if ws != rexTestSession {
			t.Errorf("the launch opened %q, want the fake's first session %q — the arm above seeds %q, so a different id means the two halves are not reading the same seat", ws, rexTestSession, rexTestBlock)
		}
	})
}

// ─── the launch's own refusal of a malformed key ─────────────────────────────

func TestQAPlanLaunchRefusesAMalformedMultiplexerKey(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	if err := os.WriteFile(b.App.ConfigPath, []byte(MultiplexerKey+": herder\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte("---\nname: ranger\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"})
	if err == nil || !strings.Contains(err.Error(), MultiplexerKey) {
		t.Fatalf("planLaunch accepted %s: herder — every create plans first, so this is the one gate that keeps a home from opening a seat under a name it ignores: %v", MultiplexerKey, err)
	}
}

// ─── what the surfaces say (the bead's "gates/list show which Mux") ──────────

func TestQAListHeaderNamesTheOccupantAndHerdrsIsUnchanged(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	var buf syncBuf
	if err := b.CmdList(&buf); err != nil {
		t.Fatal(err)
	}
	// Byte-identical to what it has always printed, which is the whole of
	// "herdr instances unchanged" on this surface.
	if !strings.HasPrefix(buf.String(), "HERDR SESSIONS\n") {
		t.Fatalf("a herdr home's listing header moved:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), MultiplexerKey+":") {
		t.Errorf("a herdr home's listing grew a multiplexer line — the default prints what it always printed:\n%s", buf.String())
	}
	if err := os.WriteFile(b.App.ConfigPath, []byte(MultiplexerKey+": "+MuxRex+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.Slot, _ = newTestRexMux(t)
	var rex syncBuf
	if err := b.CmdList(&rex); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rex.String(), "REX SESSIONS\n") || !strings.Contains(rex.String(), "program_status") {
		t.Fatalf("a Rex home's listing does not say which multiplexer it runs:\n%s", rex.String())
	}
}

// ─── the placement, which IS the decision (ADR 0076 D1) ──────────────────────

// The arm that goes red if the refusal is ever rebuilt as a launch-then-wait.
// It asserts the three things a try-and-see cannot have: no bead claimed, no
// Rex session created, and no worktree cut — because the refusal is read off
// the runtime table before any of them is spent (ADR 0013 §1 rule 3).
//
// grok is the subject deliberately: it is `prompt: argv`, so a try-and-see
// here would mean a real billed turn that nobody ever observes (ADR 0076 D3,
// MEASURED — no record in 131 s of a live argv turn).
func TestQADispatchRefusesANonReportingRuntimeOnRexBeforeTheClaim(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	if err := os.WriteFile(b.App.ConfigPath, []byte(MultiplexerKey+": "+MuxRex+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.Slot, _ = newTestRexMux(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}
	prompt := func() string { return "work prompt" }
	_, err := d.launchSession(is, "ranger", "ranger-x-a-1", "grok", "fast", prompt, nil, false)
	if err == nil {
		t.Fatal("a grok launch on a Rex instance was allowed — grok writes no program_status record there (ADR 0076 D3, MEASURED), so its every state would be unknown")
	}
	for _, want := range []string{"grok", "herdr"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
	// A plain error, not a sessionFailure: fireLoop's busy-key split has to
	// bench the persona/runtime SLOT, because every bead routed here meets
	// the same silence (ADR 0013 §2 property 4).
	if _, isSession := err.(sessionFailure); isSession {
		t.Errorf("the refusal came back as a sessionFailure — that benches the pane and re-claims the next bead onto the same gap:\n%v", err)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("the bead was claimed before the runtime was refused:\n%s", log)
	}
	// Nothing was CREATED and nothing was TYPED. The reads that did happen
	// are Resolve's listing and the provenance triple, which is how the
	// launch learns there is no session yet — they create nothing and spend
	// nothing, and they are the same two reads a herdr launch makes.
	for _, c := range rexCalls(t) {
		for _, verb := range []string{" new ", " send ", " send-key ", " kill "} {
			if strings.Contains(" "+c+" ", verb) {
				t.Errorf("a refused launch ran rex%s— nothing may be created or typed before the runtime is admitted: %s", verb, c)
			}
		}
	}
	// And claude on the same instance is NOT refused by this gate: the pin
	// would pass just as well if the predicate had been inverted or wired to
	// the multiplexer alone.
	if rt := builtinRuntime(t, "claude"); !RexAdmits(rt) {
		t.Fatal("claude must be admitted on the same instance, or this gate is refusing the multiplexer rather than the runtime")
	}
}

// The D1 BACKSTOP, which the dispatch pin above does not reach
// (ranger-base-metqp, finding 4). planLaunch carries the same refusal for
// every bead-carrying path that is not the dispatch loop — a cockpit `d` on a
// session it must create, a recipe, a recreate — and nothing held it:
// MEASURED by ranger-base-grj2i, 2026-10-31: with the condition at
// herdrback.go disabled as `if false && ...`, the fifteen muxslot pins and
// the whole of internal/posse arm 1 both came back green, in 493.2s.
// RE-MEASURED under ranger-base-metqp over three mutants — that one, a
// RexAdmits that admits everything, and a RexAdmits that admits nothing —
// each of which reds an arm below.
// TestQAPlanLaunchRefusesAMalformedMultiplexerKey is next to that line and
// covers the malformed KEY, not the admission, which is why the gap read as
// covered.
//
// grok is the subject for the dispatch pin's reason: `prompt: argv`, so a
// refusal that arrived any later would be a billed turn nobody observes.
//
// FOUR ARMS, because the condition has two halves and each can fail open.
// Dispatched and recreate refuse; interactive warns and proceeds (ADR 0013 §1
// rule 5); and an ADMITTED runtime is not refused, which is what a
// `RexAdmits` that admitted nothing — or a refusal moved above the
// admission read — would red.
func TestQAPlanLaunchIsTheRexAdmissionBackstopForEveryBeadCarryingPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		o       NewSessionOpts
		refused bool
	}{
		{"a recipe or cockpit launch carrying a bead", NewSessionOpts{Runtime: "grok", Bead: "ranger-base-x"}, true},
		{"a recreate, which plans before it kills", NewSessionOpts{Runtime: "grok", Recreate: true}, true},
		{"the operator's own, at the keyboard", NewSessionOpts{Runtime: "grok"}, false},
		{"the runtime Rex admits", NewSessionOpts{Runtime: "claude", Bead: "ranger-base-x"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			if err := os.WriteFile(b.App.ConfigPath, []byte(MultiplexerKey+": "+MuxRex+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			// The slot is the FAKE rex, so that the zero-rex-calls
			// assertion below is a real reading and not a backend with no
			// occupant to call. planLaunch reads the admission off
			// a.MultiplexerName() and the runtime table, so it never needs
			// it — which is the property being asserted.
			b.Slot, _ = newTestRexMux(t)
			writePersona(t, b.App, "ranger", "[go]")
			o := tc.o
			o.Name, o.Dir, o.Agent = "s1", t.TempDir(), "ranger"
			_, err := b.planLaunch(o)
			warn := warnBuf(t, b).String()

			// ADR 0013 §1 rule 3, the half the dispatch pin next door calls
			// its strongest: the verdict is read off the runtime table, so
			// no seat is opened and no occupant is asked, on any of these
			// arms. A try-and-see rebuild would show up here as a call.
			if calls := rexCalls(t); len(calls) != 0 {
				t.Errorf("planLaunch made %d rex call(s) — the admission is a DECLARATION read before the claim, and a plan that asks the occupant has already spent the thing refusing was meant to save:\n%s", len(calls), strings.Join(calls, "\n"))
			}

			if !tc.refused {
				if err != nil {
					t.Fatalf("planLaunch refused this launch: %v", err)
				}
				// The interactive grok arm still has to SAY it, or the
				// escape hatch is a silent one.
				if tc.o.Runtime == "grok" && (!strings.Contains(warn, "DEGRADED") || !strings.Contains(warn, "grok")) {
					t.Errorf("an interactive launch on a runtime Rex cannot read proceeded without saying so:\n%s", warn)
				}
				return
			}
			if err == nil {
				t.Fatal("planLaunch planned a grok launch on a `multiplexer: rex` instance — this is D1's backstop for every bead-carrying path that is not the dispatch loop, and without it a cockpit `d`, a recipe or a recreate opens a seat whose every state is unknown (ADR 0076 D1)")
			}
			// It is THIS refusal and not another of planLaunch's: the same
			// four facts the pre-claim refusal next door is pinned on.
			for _, want := range []string{"grok", "herdr", MultiplexerKey + ": " + MuxRex, "ADR 0076"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not name %q, so it may be some other planLaunch gate standing in for the admission:\n%v", want, err)
				}
			}
			if warn != "" {
				t.Errorf("a refused launch also printed a line — the asymmetry is refuse OR warn, never both:\n%s", warn)
			}
		})
	}
}
