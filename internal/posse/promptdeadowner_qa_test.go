//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-6pew3: a prompt is REFUSED into a rex block whose own
// program_status record says nothing is holding its keyboard — muxrex.go's
// refuseAtDeadOwner, which is ADR 0076 §3's liveness reading spent where the
// keystrokes are instead of conceded to.
//
// THE FIXTURES ARE THE MEASUREMENT'S SHAPE, not its bytes. Every row below is
// one of the three states MEASURED 2026-11-11 on this box (rex CLI
// 0.0.1794069975-g6c6689, claude 2.1.322) in a scratch `rex new` block, and
// docs/notes.d/ranger-base-6pew3.md has the transcript:
//
//	`/exit`   → {"records":[]}                                    conceded
//	SIGTERM   → {"records":[]}                                    conceded
//	SIGKILL   → state=idle, owner claude(2292821), a DEAD pid,     REFUSED
//	            with the block's foreground back to its own shell
//
// and on that last row the same session typed a work prompt through this
// verb's exact argv and the shell RAN it (`make: *** No rule to make target
// 'the'.  Stop.`).

import (
	"strings"
	"testing"
)

// ─── the three verdicts ──────────────────────────────────────────────────────

func TestQARexOwnerVerdictNamesWhichOfSection3sGuardsRefused(t *testing.T) {
	t.Parallel()
	// The split the refusal rests on, and the reason it is a verdict rather
	// than a sentence: two of §3's three guards are EVIDENCE about the
	// keyboard and the rest is silence, and a delivery verb that matched on
	// the error's prose would stop refusing the day somebody improved it.
	for _, c := range []struct {
		name  string
		recs  []RexStatusRecord
		proc  RexProcess
		want  rexOwnerVerdict
		fails bool
	}{
		{"a live claude owns the record", []RexStatusRecord{{State: "idle", App: "claude-code",
			Owner: &RexOwner{PID: 20, Name: claudeRuntimeProcName}}}, claudeProcTable(20), rexOwnerAnswers, false},
		{"the record outlived its writer (the SIGKILL row)", []RexStatusRecord{{State: "idle", App: "claude-code",
			Owner: &RexOwner{PID: 999, Name: claudeRuntimeProcName}}}, rexShellProc(), rexOwnerDead, true},
		{"the owner's name no longer matches that pid", []RexStatusRecord{{State: "idle", App: "claude-code",
			Owner: &RexOwner{PID: 20, Name: "python3"}}}, claudeProcTable(20), rexOwnerDead, true},
		{"the block's own shell wrote it", []RexStatusRecord{{State: "idle", App: "claude-code",
			Owner: &RexOwner{PID: 10, Name: "zsh"}}}, claudeProcTable(20), rexOwnerShell, true},
		{"no record at all (the /exit and SIGTERM rows)", nil, rexShellProc(), rexOwnerUnknown, true},
		{"a record naming no owner", []RexStatusRecord{{State: "idle", App: "claude-code"}},
			rexShellProc(), rexOwnerUnknown, true},
		{"only a CHILD record, so the program reported nothing of its own",
			[]RexStatusRecord{{ID: "build", State: "working", App: "claude-code",
				Owner: &RexOwner{PID: 20, Name: claudeRuntimeProcName}}}, claudeProcTable(20), rexOwnerUnknown, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m, _ := newTestRexMux(t)
			rexSeedTestSeat(t)
			rexSeedProcess(t, rexTestBlock, c.proc)
			rexSeedStatus(t, rexTestBlock, c.recs)
			_, verdict, err := m.runtimeOwner(rexTestSeat())
			if verdict != c.want {
				t.Errorf("runtimeOwner verdict = %d, want %d (err %v)", verdict, c.want, err)
			}
			// The verdict never replaces the error: RuntimeOwner's own
			// contract is that an error is the only other answer, never a
			// zero owner, because the caller joins a file name on that pid.
			if (err != nil) != c.fails {
				t.Errorf("runtimeOwner err = %v, want failure = %v", err, c.fails)
			}
		})
	}
}

func TestQARexOwnerVerdictConcedesEveryReadItCouldNotTake(t *testing.T) {
	t.Parallel()
	// Unknown is not no (ADR 0013 §1), on the two calls this reading is made
	// of. A rex that will not answer is ignorance about the keyboard and
	// never evidence against it — which is the direction that costs a prompt
	// typed into a seat posse could not read rather than a fleet that cannot
	// be prompted over a server hiccup.
	for _, verb := range []string{"program_status", "process"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			m, _ := newTestRexMux(t)
			rexSeedTestSeat(t)
			rexSeedProcess(t, rexTestBlock, claudeProcTable(20))
			rexSeedStatus(t, rexTestBlock, []RexStatusRecord{{State: "idle", App: "claude-code",
				Owner: &RexOwner{PID: 20, Name: claudeRuntimeProcName}}})
			rexSeedFail(t, verb, "block target not found")
			_, verdict, err := m.runtimeOwner(rexTestSeat())
			if err == nil {
				t.Fatalf("a failed `%s` answered without an error", verb)
			}
			if verdict != rexOwnerUnknown {
				t.Errorf("a failed `%s` read as verdict %d — a read posse could not take is being spent as evidence", verb, verdict)
			}
		})
	}
}

// ─── the refusal ─────────────────────────────────────────────────────────────

// rexSeatWithDeadOwner is the SIGKILL row: an `idle` record whose owner pid
// is not in the block's process table, over a block whose foreground has
// fallen back to its own shell. `app` is passed through so the non-claude
// arms can use the same seat.
func rexSeatWithDeadOwner(t *testing.T, app, proc string) RexMux {
	t.Helper()
	m, _ := newTestRexMux(t)
	rexSeedTestSeat(t)
	rexSeedProcess(t, rexTestBlock, rexShellProc())
	rexSeedStatus(t, rexTestBlock, []RexStatusRecord{{State: "idle", App: app,
		UpdatedAt: "2026-11-11T01:37:37-05:00", Owner: &RexOwner{PID: 2292821, Name: proc}}})
	// A composer on the screen, so nothing in these arms can be the other
	// refusal answering: the only thing wrong with this seat is that its
	// record outlived the process that wrote it.
	rexSeedCaptureQueue(t, rexTestBlock, rexIdleScreen)
	return m
}

func TestQARexPromptRefusesABlockWhoseRecordOutlivedItsWriterAndTypesNothing(t *testing.T) {
	t.Parallel()
	m := rexSeatWithDeadOwner(t, "claude-code", claudeRuntimeProcName)
	_, err := m.Prompt(rexTestSeat(), "make the LIVENESS refusal in RexMux.Prompt", true, 10_000)
	if err == nil {
		t.Fatal("a prompt into a block whose program_status record outlived its writer was accepted — that text would have been RUN by the block's shell (MEASURED 2026-11-11)")
	}
	if rexTypedAnything(t) {
		t.Errorf("the refusal still typed; calls were:\n%s", strings.Join(rexCalls(t), "\n"))
	}
	// The operator has to be told WHICH reading refused and what to do, and
	// the dead owner is the evidence: a pid and a name, out of the record.
	for _, want := range []string{"2292821", "outlived", "posse peek"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
}

func TestQARexPromptRefusesABlockWhoseOwnShellWroteTheRecord(t *testing.T) {
	t.Parallel()
	// §3's other positive case: a report written from the shell's own prompt
	// carries the shell as its owner, so the block IS a shell at a prompt.
	m, _ := newTestRexMux(t)
	rexSeedTestSeat(t)
	rexSeedProcess(t, rexTestBlock, rexShellProc())
	rexSeedStatus(t, rexTestBlock, []RexStatusRecord{{State: "idle", App: "claude-code",
		Owner: &RexOwner{PID: 10, Name: "zsh"}}})
	rexSeedCaptureQueue(t, rexTestBlock, rexIdleScreen)
	_, err := m.Prompt(rexTestSeat(), "do the thing", false, 0)
	if err == nil {
		t.Fatal("a prompt into a block whose own shell wrote the record was accepted")
	}
	if rexTypedAnything(t) {
		t.Errorf("the refusal still typed; calls were:\n%s", strings.Join(rexCalls(t), "\n"))
	}
	if !strings.Contains(err.Error(), "own shell") {
		t.Errorf("the refusal does not name the guard that refused: %v", err)
	}
}

func TestQARexPromptDeadOwnerRefusalIsNotDeliveredSoAClaimIsHandedBack(t *testing.T) {
	t.Parallel()
	// ADR 0084 D1's allow-list, for refuseWithoutComposer's reason exactly:
	// nothing was typed, so the claim goes back rather than being kept over a
	// seat that was never asked anything. A plain untyped error reads as
	// INDETERMINATE and is kept.
	m := rexSeatWithDeadOwner(t, "claude-code", claudeRuntimeProcName)
	_, err := m.Prompt(rexTestSeat(), "do the thing", true, 10_000)
	if err == nil {
		t.Fatal("a prompt into a block with a dead owner was accepted")
	}
	if !isNotDelivered(err) {
		t.Errorf("Prompt's refusal = %v, which isNotDelivered does not recognize — dispatch would keep the claim over a prompt it never sent", err)
	}
}

func TestQARexPromptDeadOwnerRefusalIsNotScopedToARuntime(t *testing.T) {
	t.Parallel()
	// The one place this differs from refuseWithoutComposer, which is bound
	// to claude by Amendment 5's limit because it reads claude's own chrome.
	// This compares a pid to a pid, which is true of every runtime — and the
	// runtimes that cannot clear their record on the way out are the ones it
	// protects most: posse's own reporter clears at `SessionEnd`
	// (CodexHookReport), an event a killed codex never fires.
	for app, proc := range map[string]string{"codex": "codex", "grok-build": "grok"} {
		t.Run(app, func(t *testing.T) {
			t.Parallel()
			m := rexSeatWithDeadOwner(t, app, proc)
			if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err == nil {
				t.Fatalf("a prompt into a dead %s's block was accepted — the liveness reading is scoped to a runtime it has no reason to be", app)
			}
			if rexTypedAnything(t) {
				t.Errorf("the refusal still typed; calls were:\n%s", strings.Join(rexCalls(t), "\n"))
			}
		})
	}
}

// ─── what it must NOT refuse ─────────────────────────────────────────────────

func TestQARexPromptConcedesABlockWithNoRecordAtAll(t *testing.T) {
	t.Parallel()
	// THE MEASURED COMMON CASE, and the reason this refusal is narrower than
	// the failure it answers: a claude ended with `/exit` or SIGTERM takes
	// its record with it, so the commonest bare-shell block on this instance
	// reads rexOwnerUnknown. Refusing on that would also refuse every seat in
	// its launch window, which is what AwaitFirstReport is for.
	m, _ := newTestRexMux(t)
	rexSeedTestSeat(t)
	rexSeedProcess(t, rexTestBlock, rexShellProc())
	rexSeedStatus(t, rexTestBlock, nil)
	rexSeedCaptureQueue(t, rexTestBlock, rexIdleScreen)
	if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err != nil {
		t.Fatalf("a block with no program_status record was refused on an absence: %v", err)
	}
	rexCallWith(t, "send", "do the thing")
}

func TestQARexPromptConcedesEveryOwnerReadItCouldNotTake(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"program_status", "process"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			m := rexSeatWithDeadOwner(t, "claude-code", claudeRuntimeProcName)
			rexSeedFail(t, verb, "block target not found")
			if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err != nil {
				t.Fatalf("a failed `%s` was read as evidence that nothing holds the keyboard: %v", verb, err)
			}
			rexCallWith(t, "send", "do the thing")
		})
	}
}

func TestQARexPromptTypesIntoABlockALiveRuntimeStillHolds(t *testing.T) {
	t.Parallel()
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("idle")})
	rexSeedCaptureQueue(t, rexTestBlock, rexIdleScreen)
	if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err != nil {
		t.Fatalf("a prompt into a live claude's block was refused: %v", err)
	}
	rexCallWith(t, "send", "do the thing")
	rexCallWith(t, "send-key", "enter")
}

// ─── order and cost ──────────────────────────────────────────────────────────

func TestQARexPromptJudgesTheKeyboardBeforeItJudgesTheScreen(t *testing.T) {
	t.Parallel()
	// ORDER, and it is not about cost. A block that has gone back to its
	// shell has no composer to be judged for, and the screen it leaves
	// behind is whatever the dead runtime last drew — so a seat with a dead
	// owner and claude's own box still on its screen must refuse on the
	// keyboard, not be licensed by the picture. rexSeatWithDeadOwner seeds
	// exactly that screen.
	m := rexSeatWithDeadOwner(t, "claude-code", claudeRuntimeProcName)
	if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err == nil {
		t.Fatal("a stale composer licensed a prompt into a block nothing is holding")
	}
	for _, c := range rexCalls(t) {
		if strings.Contains(c, "capture") {
			t.Errorf("the liveness refusal paid for a capture it cannot use: %s", c)
		}
	}
}

func TestQARexPromptTakesTheOwnerReadingOnceForBothRefusals(t *testing.T) {
	t.Parallel()
	// The reading is two rex calls, and before ranger-base-6pew3 there was
	// one refusal taking it. There are two now and there is still one
	// reading: Prompt takes it and hands it down (Prompt's own note).
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("idle")})
	rexSeedCaptureQueue(t, rexTestBlock, rexIdleScreen)
	if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	for _, verb := range []string{"program_status", "process"} {
		n := 0
		for _, c := range rexCalls(t) {
			if strings.Contains(c, verb) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("one prompt took %d `%s` calls, want 1 — the two refusals are reading the block twice", n, verb)
		}
	}
}
