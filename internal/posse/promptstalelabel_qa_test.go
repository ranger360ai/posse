//go:build posse_arm3

package posse

// ranger-base-j687x: a typed prompt is REFUSED into a reported pane whose
// every foreground process is the pane's own shell — ADR 0061 D3.2's reading,
// moved into the DELIVERY VERB beside ranger-base-xdaw9's screen refusal,
// over reportedagent.go's refuseAtShellPrompt.
//
// WHAT THESE ARMS HOLD, and it is not the reading: ADR 0061 D3.2 built the
// reading and reportedreading_qa_test.go pins it. What this bead found is
// where it does NOT run. D3.2 said the liveness half "runs before every
// keystroke the promptable gate admits" — true, and the gate is not the only
// way to the keystrokes. TRACED 2026-11-11, three live callers reach
// `AgentPrompt` with no liveness reading in front of them: `posse prompt
// --now` (which the gate's own refusal used to offer as the way past itself),
// `landThePlane` (gateless on purpose — `AgentTarget` resolved a live PANE,
// which is not a live CLI) and the runtime probe (gates on `agent wait`
// alone, which a stale `idle` label settles instantly). So the refusal has to
// be where the keystrokes are, which is ADR 0076 Amendment 7 D7's argument
// one layer over.
//
// WHAT IS MEASURED (2026-11-11, herdr 0.9.1, claude 2.1.322, this box,
// docs/notes.d/ranger-base-xdaw9.md): a reported label OUTLIVES the CLI it
// describes on this herdr. A pane running claude, labelled through `pane
// report-agent --agent fakebob-xdaw9 --state idle`; the first probe's Enter
// actuated the folder-trust prompt's `No, exit` and claude EXITED; `agent get
// w1:p1` then still read `{"agent":"fakebob-xdaw9","agent_status":"idle"}`
// over a pane that was a bare shell prompt — ADR 0061 Claims fact 1
// reproduced on a live claude. And `pane send-text` + `pane send-keys enter`
// into a pane at a shell RUNS the text: the same probe launched claude that
// way on purpose and the shell executed it. Nothing prints red on that path —
// the keystrokes land, both calls succeed, and what runs is whatever a work
// prompt means to zsh.
//
// NO RUNTIME IS NAMED in any of it, and the labels here are deliberately of
// no significance: the reading is a pid compared to a pid, so what these arms
// are about is the REPORT and the pane, never the CLI behind them.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// staleReportedPane arms the measured shape: a pane carrying a reported label
// with a watched state on it, and a foreground that is nothing but the pane's
// own shell. The numbers are ADR 0061 Claims fact 2's (shell_pid 34661).
func staleReportedPane(t *testing.T, fake, label, status string) {
	t.Helper()
	reportedPane(t, fake, label, status, "claude", "codex", "grok")
	atShell(t, fake)
}

// ─── the bug ─────────────────────────────────────────────────────────────────

func TestQAReportedPromptRefusesAPaneThatWentBackToItsShellAndTypesNothing(t *testing.T) {
	t.Parallel()
	// The whole hazard in one arm: the reporter says idle, the pane is a bare
	// shell prompt, and the two keystrokes would have run the work prompt as
	// a shell command.
	b, fake := newTestBackend(t)
	staleReportedPane(t, fake, "plugged-j687x", "idle")

	_, err := b.H.AgentPrompt("w1:p1", "Work beads issue ranger-base-j687x", true, 10_000)
	if err == nil {
		t.Fatal("a prompt into a pane at a bare shell prompt was accepted")
	}
	if log := calls(t, fake); herdrTypedAnything(log) {
		t.Errorf("the refusal still typed — that text is a shell command now; calls were:\n%s", log)
	}
}

func TestQAReportedPromptRefusesAStaleLabelInEveryWatchedState(t *testing.T) {
	t.Parallel()
	// Every state the reporter can leave behind that reportedStateIsWatched
	// calls positive evidence. `blocked` is the one that reaches this guard
	// through a refusal of its own, and it must not be the one that slips
	// past: sendTextPrompt's `agent_blocked` arm refuses it first, with
	// nothing typed either way, which is the only property that matters here.
	for _, state := range []string{"idle", "working", "done", "blocked"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			staleReportedPane(t, fake, "plugged-j687x", state)
			if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err == nil {
				t.Fatalf("a prompt into a shell under a reported %q was accepted", state)
			}
			if log := calls(t, fake); herdrTypedAnything(log) {
				t.Errorf("the refusal still typed:\n%s", log)
			}
		})
	}
}

func TestQAStaleLabelRefusalIsNotDeliveredSoAClaimIsHandedBack(t *testing.T) {
	t.Parallel()
	// ADR 0084 D1's allow-list, the same reason the screen refusal beside
	// this one borrows `agent_not_ready`: nothing was typed, so the claim
	// goes back rather than being kept over a pane nobody asked anything. A
	// plain untyped error would be read as INDETERMINATE and kept.
	b, fake := newTestBackend(t)
	staleReportedPane(t, fake, "plugged-j687x", "idle")

	_, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0)
	if err == nil {
		t.Fatal("a prompt into a shell was accepted")
	}
	if !isNotDelivered(err) {
		t.Errorf("the refusal = %v, which isNotDelivered does not recognize — dispatch would keep the claim over a prompt it never sent", err)
	}
}

func TestQAStaleLabelRefusalNamesTheLabelTheStateAndTheShell(t *testing.T) {
	t.Parallel()
	// The refusal is the whole surface an operator gets. It carries the one
	// copy of the stale-label clause (ReportedNotSeen), so the sentence here
	// and the sentence the gate prints cannot drift apart, plus this route's
	// own two calls and what the recovery is.
	b, fake := newTestBackend(t)
	staleReportedPane(t, fake, "plugged-j687x", "idle")

	_, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0)
	if err == nil {
		t.Fatal("a prompt into a shell was accepted")
	}
	for _, want := range []string{
		"nothing was sent",
		"plugged-j687x",
		"is STALE",
		"SHELL",
		"runs every line of it",
		"pane send-text",
		"ADR 0061 D3.2",
		"posse peek",
		"ranger-base-xdaw9",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q:\n%v", want, err)
		}
	}
}

func TestQAStaleLabelRefusalSentenceIsReportedNotSeensOwn(t *testing.T) {
	t.Parallel()
	// One copy, asserted as one copy: the clause in the refusal is the string
	// ReportedNotSeen returns for this reading, not a second spelling of it.
	b, fake := newTestBackend(t)
	staleReportedPane(t, fake, "plugged-j687x", "idle")

	_, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0)
	if err == nil {
		t.Fatal("a prompt into a shell was accepted")
	}
	want := AgentDetection{State: "idle", Reported: "plugged-j687x", ShellForeground: true}.ReportedNotSeen()
	if want == "" {
		t.Fatal("ReportedNotSeen says nothing about a stale label — the refusal has no sentence to borrow")
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("the refusal does not carry ReportedNotSeen's own clause:\nwant %q\nin   %v", want, err)
	}
}

// ─── the scope, which is the one way this differs from the screen refusal ────

func TestQAStaleLabelRefusalReachesThePaneThisRouteWasBuiltFor(t *testing.T) {
	t.Parallel()
	// The DESIGNED case of this route — a pane whose RUNTIME herdr has no
	// manifest for, a Bob under a plugin — is the case the screen refusal
	// beside this one CONCEDES, because the box it locates is claude's idiom
	// and herdr names no chrome for that pane. This guard reads herdr's
	// process table instead: a pid compared to a pid, no runtime named, so it
	// answers for that pane too. A stranded Bob is exactly the pane ADR 0061
	// D3.2 was written about.
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle", "claude", "codex", "grok")
	atShell(t, fake)
	write(t, filepath.Join(fake, "pane-text", "w1_p1"), "prompt$ \n")

	_, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0)
	if err == nil {
		t.Fatal("a stranded label over a pane herdr cannot explain at all was typed into")
	}
	if log := calls(t, fake); herdrTypedAnything(log) {
		t.Errorf("the refusal still typed:\n%s", log)
	}
}

func TestQAStaleLabelRefusalConcedesAPaneWithACLIInItsForeground(t *testing.T) {
	t.Parallel()
	// The pane this route exists to type into. The fake's DEFAULT foreground
	// is a live one (shell_pid 34661, the CLI at 82630 — ADR 0061 Claims fact
	// 2's own numbers), so this is the arm that says the guard refuses the
	// measured shape and not the ordinary one.
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle", "claude", "codex", "grok")

	if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err != nil {
		t.Fatalf("a prompt into a pane with a CLI in its foreground was refused: %v", err)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "pane send-text w1:p1 do the thing") || !strings.Contains(log, "pane send-keys w1:p1 enter") {
		t.Errorf("the route must still deliver:\n%s", log)
	}
}

func TestQAStaleLabelRefusalConcedesAProcessReadingItCannotTake(t *testing.T) {
	t.Parallel()
	// UNKNOWN IS NOT A NO, the direction this whole file is built on
	// (paneShellForeground's own note): a `process-info` that cannot be TAKEN
	// is not evidence that the pane is at a shell. The label stands on its
	// own as it did before ADR 0061, because the other way round means a
	// herdr that will not answer makes every reported pane unpromptable —
	// a box that has stopped dispatching rather than one running a measured
	// risk.
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle", "claude", "codex", "grok")
	write(t, filepath.Join(fake, "process-info-error"), "bad_request|no such subcommand")

	if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err != nil {
		t.Fatalf("a process reading that failed was read as a pane at a shell: %v", err)
	}
	if log := calls(t, fake); !strings.Contains(log, "pane send-text w1:p1 do the thing") {
		t.Errorf("the prompt must still go in:\n%s", log)
	}
}

func TestQAStaleLabelRefusalConcedesAForegroundHerdrDidNotReport(t *testing.T) {
	t.Parallel()
	// "Every" over an empty set is vacuously true, and a missing shell_pid
	// has nothing to compare against — both are UNKNOWN inside AtShellPrompt,
	// and this is that rule reached through the delivery verb rather than
	// through the predicate. A herdr whose output shape moved must not turn
	// into a box where every reported label reads stale.
	for name, foreground := range map[string]string{
		"no foreground processes": "34661|",
		"no shell_pid":            "0|82630",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			reportedPane(t, fake, "plugged", "idle", "claude", "codex", "grok")
			write(t, filepath.Join(fake, "pane-foreground"), foreground)

			if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err != nil {
				t.Fatalf("%s was spent as a refusal: %v", name, err)
			}
			if log := calls(t, fake); !strings.Contains(log, "pane send-text w1:p1 do the thing") {
				t.Errorf("the prompt must still go in:\n%s", log)
			}
		})
	}
}

// ─── order, which is about cost and about which diagnosis the operator gets ──

func TestQAStaleLabelIsReadBeforeAnyScreenAndCostsOneCall(t *testing.T) {
	t.Parallel()
	// A pane at a shell has no CLI on it, so the screen refusal beside this
	// one would concede it anyway (`agent explain` names no claude there) and
	// the operator would be told nothing. Reading liveness FIRST buys the
	// right diagnosis for one `process-info` — measured at 0.00s — and spends
	// neither the scope `agent explain` nor the `pane read` behind it.
	b, fake := newTestBackend(t)
	herdrClaudeOnAReportedLabel(t, fake, herdrIdleScreen)
	atShell(t, fake)

	if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err == nil {
		t.Fatal("a prompt into a shell under a label herdr detects as claude was accepted")
	}
	log := calls(t, fake)
	if !strings.Contains(log, "pane process-info --pane w1:p1") {
		t.Errorf("the liveness reading is the whole of this refusal and must be taken:\n%s", log)
	}
	if strings.Contains(log, "agent explain w1:p1 --json") || strings.Contains(log, "pane read") {
		t.Errorf("a stale label is answered before the screen is read at all:\n%s", log)
	}
	if n := strings.Count(log, "pane process-info"); n != 1 {
		t.Errorf("%d process-info reads for one prompt — this reading needs no second look, a screen does:\n%s", n, log)
	}
}

func TestQAStaleLabelRefusalCostsNothingOnTheArgumentRefusals(t *testing.T) {
	t.Parallel()
	// The two refusals that are properties of the ARGUMENT and of a reading
	// already in hand stay ahead of it, so neither costs a herdr call it does
	// not need.
	for name, tc := range map[string]struct{ status, text string }{
		"a leading '/'":   {"idle", "/clear"},
		"a blocked label": {"blocked", "do the thing"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			staleReportedPane(t, fake, "plugged-j687x", tc.status)

			if _, err := b.H.AgentPrompt("w1:p1", tc.text, true, 10_000); err == nil {
				t.Fatalf("%s was accepted", name)
			}
			if log := calls(t, fake); strings.Contains(log, "pane process-info") {
				t.Errorf("%s cost a process reading:\n%s", name, log)
			}
		})
	}
}

// ─── the gate no longer offers a door that is shut ───────────────────────────

func TestQAThePromptableGateStopsOfferingNowForAStaleLabel(t *testing.T) {
	t.Parallel()
	// The gate's reported refusal ends in its remedies, and `--now` was the
	// last of them: it skips the gate, and before this bead it also skipped
	// every reading of liveness there was. It does not any more, so naming it
	// on this clause would point the operator at a door that answers
	// agent_not_ready. The OTHER clause — a state nobody watched — is
	// unchanged, because that is a pane posse cannot judge, which is what
	// --now is for.
	for _, tc := range []struct {
		name, status  string
		stale, offers bool
	}{
		{"a stale label", "idle", true, false},
		{"a state nobody watched", "unknown", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			promptReadySession(t, b, "plugged", "200ms")
			reportedPane(t, fake, "plugged", tc.status, "claude", "codex", "grok")
			if tc.stale {
				atShell(t, fake)
			}

			_, _, err := b.AwaitPromptable("plugged", fakeHerdrSeat)
			if err == nil {
				t.Fatal("the gate opened on a reading that is not seen")
			}
			// The OFFER and not the token: the stale clause still says the
			// word, to say that it is not the way past this one.
			if got := strings.Contains(err.Error(), "send it anyway with --now"); got != tc.offers {
				t.Errorf("the refusal offers --now = %v, want %v:\n%v", got, tc.offers, err)
			}
			if !tc.offers && !strings.Contains(err.Error(), "the delivery verb takes the same reading") {
				t.Errorf("the refusal must say why --now is not the way past this one:\n%v", err)
			}
		})
	}
}

// ─── and the route it was traced through ─────────────────────────────────────

func TestQAStaleLabelIsRefusedOnTheOneTypedDeliveryDoor(t *testing.T) {
	t.Parallel()
	// `AgentPrompt` is the one door every typed delivery comes through —
	// `HerdrMux.Prompt` and therefore `posse prompt --now`, landThePlane, the
	// pulse, the probe and both dispatch call sites. Asserted through the mux
	// rather than the method, because a pin that called the refusal directly
	// would prove nothing about who reaches it, and the three callers this
	// bead traced reach it with no gate in front.
	b, fake := newTestBackend(t)
	staleReportedPane(t, fake, "plugged-j687x", "idle")

	if _, err := b.Mux().Prompt(fakeHerdrSeat, "do the thing", false, 0); err == nil {
		t.Fatal("the mux typed a work prompt into a bare shell")
	}
	if log := calls(t, fake); herdrTypedAnything(log) {
		t.Errorf("the refusal still typed:\n%s", log)
	}
}

// A guard on the arrangement: if the fake's default foreground ever becomes
// the shell, every "must still deliver" arm above would go green over a route
// that refuses everything, and every refusal arm would pass for the wrong
// reason.
func TestQAFakeHerdrsDefaultPaneIsLiveAndAtShellIsTheArmedOne(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle", "claude", "codex", "grok")

	if p, err := b.H.PaneProcessInfo("w1:p1"); err != nil || p.AtShellPrompt() {
		t.Fatalf("the fake's default pane must be LIVE: %+v %v", p, err)
	}
	atShell(t, fake)
	if p, err := b.H.PaneProcessInfo("w1:p1"); err != nil || !p.AtShellPrompt() {
		t.Fatalf("atShell must put the pane's own shell in the foreground: %+v %v", p, err)
	}
	if _, err := os.Stat(filepath.Join(fake, "pane-foreground")); err != nil {
		t.Errorf("the lever is not where the fake reads it: %v", err)
	}
}
