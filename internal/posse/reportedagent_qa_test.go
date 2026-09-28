//go:build posse_arm3

package posse

// QA pins for ranger-base-8eqaa — typed prompt delivery to a pane herdr
// LABELS but does not DETECT (reportedagent.go).
//
// THE MEASUREMENT these stand in for (2026-09-28, herdr 0.9.1, this box, a
// scratch workspace reported under the label `fakebob` and closed after):
//
//	pane report-agent --source posse-probe --agent fakebob --state idle <p>
//	agent get <p>      → {"agent":"fakebob","agent_status":"idle",…}
//	agent explain <p>  → agent_explain_unavailable: "does not have a
//	                     detected agent label"
//	agent prompt <p>   → agent_not_ready: "is not an active named agent"
//	pane send-text <p> "…" ; pane send-keys <p> enter   → lands, every time
//	agent wait <p> --until working --timeout 5000       → returns on the
//	                     reported transition, in the result.agent shape
//
// The label is deliberately of no significance: the same three answers came
// back for a live Bob pane under the MartinLoeper/herdr-bob plugin
// (ranger-base-p8afi), and what they are about is the report, not the
// runtime. So is every test here — none of them names bob, and a pin that
// did would go green the day the mechanism started keying on a name, which
// is the ADR 0017 §3 class ADR 0060 rejected a report-agent adapter over.
//
// WHAT WOULD FAIL SILENTLY WITHOUT THESE. All of it. A prompt that takes the
// wrong route does not print anything: `agent prompt` answers agent_not_ready
// and dispatch unclaims the bead as a failed prompt, while a send-text route
// taken over a pane herdr CAN address would bypass herdr's own blocked
// rejection and type into a permission dialog (rangerhq-ejf). Both are one
// branch apart, and only the call log says which one ran.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reportedPane arms the fake herdr the way the box was armed: `agent get`
// answers with a label and a state, and `herdr-kinds` names the labels this
// herdr has manifests for — so a label absent from it is exactly the shape
// `pane report-agent` leaves behind.
func reportedPane(t *testing.T, fake, label, status string, kinds ...string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fake, "reported-agent"), []byte(label+"|"+status), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "herdr-kinds"), []byte(strings.Join(kinds, " ")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The bug. A pane herdr labels but has no manifest for is typed into, and
// `agent prompt` — the verb that answers agent_not_ready for it — is never
// reached.
func TestQAAPromptToAReportedAgentGoesInByPaneSendText(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle", "claude", "codex", "grok")

	if _, err := b.H.AgentPrompt("w1:p1", "Work beads issue ranger-base-8eqaa", false, 0); err != nil {
		t.Fatalf("a reported pane must take a prompt: %v", err)
	}
	log := calls(t, fake)
	if strings.Contains(log, "agent prompt") {
		t.Errorf("`agent prompt` answers agent_not_ready for a reported pane — it must not be the route:\n%s", log)
	}
	if !strings.Contains(log, "pane send-text w1:p1 Work beads issue ranger-base-8eqaa") {
		t.Errorf("the prompt text must be typed into the pane:\n%s", log)
	}
	if !strings.Contains(log, "pane send-keys w1:p1 enter") {
		t.Errorf("typed text that is never submitted is the whole failure — Enter must follow:\n%s", log)
	}
}

// The control arm, and the one that says the test above measured the REPORT
// and not the mechanism: a label herdr has a manifest for is addressed by
// herdr's own verb, on the argv it always used. Reverting nothing but the
// manifest reading would make both tests pass; reverting the route would
// make both fail.
func TestQAAPromptToADetectedKindStillUsesHerdrsOwnVerb(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "claude", "idle", "claude", "codex", "grok")

	if _, err := b.H.AgentPrompt("w1:p1", "hello", false, 0); err != nil {
		t.Fatalf("a detected kind must take a prompt: %v", err)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "agent prompt w1:p1 hello") {
		t.Errorf("a pane herdr detects is addressed by `agent prompt`:\n%s", log)
	}
	if strings.Contains(log, "pane send-text") {
		t.Errorf("typing into a pane herdr can address bypasses its own blocked rejection — never the route here:\n%s", log)
	}
}

// UNKNOWN IS NEVER A "NO" — both readings, one test each way. A herdr that
// will not describe the pane, and a pane with no label at all, both deliver
// the way posse always has. A route that read either as "reported" would
// type work prompts at shells, which is ranger-base-3p0.
func TestQAAnUnreadableRouteDeliversTheWayPosseAlwaysHas(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		arm  func(fake string)
	}{
		{"agent get fails", func(fake string) {
			os.WriteFile(filepath.Join(fake, "agent-get-error"), []byte("timeout|fake herdr: no response"), 0o644)
		}},
		{"no label on the pane", func(fake string) {}}, // no reported-agent lever: `agent get` 404s
		{"the manifest probe will not answer", func(fake string) {
			os.WriteFile(filepath.Join(fake, "reported-agent"), []byte("plugged|idle"), 0o644)
			os.WriteFile(filepath.Join(fake, "explain-error"), []byte("timeout|fake herdr: cannot explain"), 0o644)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			tc.arm(fake)
			if _, err := b.H.AgentPrompt("w1:p1", "hello", false, 0); err != nil {
				t.Fatalf("an unreadable route must not refuse the prompt: %v", err)
			}
			if log := calls(t, fake); !strings.Contains(log, "agent prompt w1:p1 hello") {
				t.Errorf("unknown must degrade to herdr's own verb, never to typing:\n%s", log)
			}
		})
	}
}

// ADR 0060 D5, now that the route has a caller. A leading '/' opens the
// CLI's command picker, the picker eats the Enter, and the text sits in the
// composer while the call reports success — with no herdr envelope and no
// `agent explain` behind this route, nothing else would ever say so.
func TestQAAReportedPromptRefusesALeadingSlashWithNothingSent(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle")

	_, err := b.H.AgentPrompt("w1:p1", "/clear", false, 0)
	if err == nil {
		t.Fatal("a '/' prompt on the typed route must be refused")
	}
	for _, want := range []string{"nothing was sent", "command picker", "eats the Enter", "posse peek"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q:\n%v", want, err)
		}
	}
	if log := calls(t, fake); strings.Contains(log, "send-text") || strings.Contains(log, "send-keys") {
		t.Errorf("refused means nothing was typed:\n%s", log)
	}
}

// herdr rejects a submission to a blocked agent BEFORE any input, because
// 0.8.0 typed into the dialog and the text was swallowed (rangerhq-ejf).
// The typed route is the same keystroke, so it owes the same rejection with
// the same code — a caller that branches on agent_blocked must not stop
// working because the delivery changed underneath it.
func TestQAAReportedPromptRefusesABlockedPaneWithHerdrsOwnCode(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "blocked")

	_, err := b.H.AgentPrompt("w1:p1", "carry on", false, 0)
	if !IsHerdrCode(err, "agent_blocked") {
		t.Fatalf("a blocked reported pane must be refused as agent_blocked: %v", err)
	}
	if log := calls(t, fake); strings.Contains(log, "send-text") {
		t.Errorf("herdr sends nothing to a blocked agent, and neither does this route:\n%s", log)
	}
}

// `agent prompt --wait` requires an OBSERVED working-or-blocked state within
// 5000ms of an accepted submission, else agent_prompt_stalled — herdr's own
// contract, quoted in herdrPromptStallMS. Without that leg, `agent wait
// --until idle` answers instantly with the state that was already there and
// a turn that never started reads as one that settled.
func TestQAAReportedWaitAsksForTheTurnBeforeItAsksForTheSettle(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle")

	if _, err := b.H.AgentPrompt("w1:p1", "go", true, 0); err != nil {
		t.Fatalf("a --wait prompt on the typed route must settle: %v", err)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "agent wait w1:p1 --until working --until blocked --timeout 5000") {
		t.Errorf("the turn must be observed before the settle is waited for:\n%s", log)
	}
	if !strings.Contains(log, "agent wait w1:p1 --until idle --until done --until blocked") {
		t.Errorf("and then the settle, in herdr's own default set:\n%s", log)
	}
}

// The other half of the same contract: a submission that never becomes a
// turn is agent_prompt_stalled, not a bare timeout. dispatch branches on
// that code as "the prompt never reached the agent" (rangerhq-1z0) and hands
// the claim back; a timeout means the opposite — herdr took the text and
// stopped watching — and the bead stays claimed.
func TestQAAReportedPromptThatNeverStartsATurnIsStalledNotTimedOut(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle")
	if err := os.WriteFile(filepath.Join(fake, "wait-error"),
		[]byte("timeout|fake herdr: no state observed"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := b.H.AgentPrompt("w1:p1", "go", true, 0)
	if !IsHerdrCode(err, "agent_prompt_stalled") {
		t.Fatalf("a typed prompt that starts no turn is agent_prompt_stalled: %v", err)
	}
	if log := calls(t, fake); !strings.Contains(log, "pane send-text") {
		t.Errorf("the text WAS typed — the stall is about the turn, not the delivery:\n%s", log)
	}
}

// …and the clause right after it in the same help text: "a caller timeout
// that expires first returns timeout". A caller who asked for less patience
// than herdr's own stall window gets their own answer back — which is the
// code that means "herdr took the text and stopped watching", so the bead
// stays claimed instead of being handed back (dispatch.go's gather). The two
// codes are one `if` apart and drive opposite verdicts over live work.
func TestQAAReportedPromptHonoursACallerTimeoutShorterThanTheStallWindow(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle")
	if err := os.WriteFile(filepath.Join(fake, "wait-error"),
		[]byte("timeout|fake herdr: no state observed"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := b.H.AgentPrompt("w1:p1", "go", true, 1000)
	if IsHerdrCode(err, "agent_prompt_stalled") {
		t.Fatalf("the CALLER's timeout expired first, not herdr's stall window — that is a timeout: %v", err)
	}
	if !IsHerdrCode(err, "timeout") {
		t.Fatalf("want herdr's own timeout code: %v", err)
	}
	if log := calls(t, fake); !strings.Contains(log, "--timeout 1000") {
		t.Errorf("the shorter of the two patiences is the one that is waited:\n%s", log)
	}
}

// The gate half (promptready.go). `agent explain` refuses a reported pane
// outright, so every poll of the screen gate errors and it falls into its
// never-answered concession — prompting with no readiness reading at all.
// The reported state IS the reading, and it opens the gate at once.
func TestQAThePromptGateReadsAReportedStateAsPromptable(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	promptReadySession(t, b, "plugged", "400ms")
	reportedPane(t, fake, "plugged", "idle")

	det, note, err := b.AwaitPromptable("plugged", "w1:p1")
	if err != nil {
		t.Fatalf("a reported idle pane must be promptable: %v", err)
	}
	if note != "" {
		t.Errorf("a gate that waited for nothing has nothing to report: %q", note)
	}
	if det.State != "idle" || det.Reported != "plugged" {
		t.Errorf("the detection must carry the reported state and the label that stated it: %+v", det)
	}
	// Seen() is what the pulse re-applies its own idle|done rule to
	// (pulse.go). A reported state that read as unseen would let the shop
	// check prompt a persona mid-turn, which is the one thing that rule is
	// for.
	if !det.Seen() {
		t.Errorf("an authority reporting a lifecycle state is positive evidence: %+v", det)
	}
	if log := calls(t, fake); strings.Contains(log, "agent explain w1:p1") {
		t.Errorf("herdr refuses to explain a reported pane — asking it is a wasted startup wait every time:\n%s", log)
	}
}

// And the refusal arm: `unknown` is the reported way of saying nothing is
// known about this CLI, and it holds exactly as herdr's own guess does. A
// gate that opened on it would type into a pane nobody has watched start.
func TestQAThePromptGateRefusesAReportedUnknownState(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	promptReadySession(t, b, "plugged", "400ms")
	reportedPane(t, fake, "plugged", "unknown")

	_, _, err := b.AwaitPromptable("plugged", "w1:p1")
	if err == nil {
		t.Fatal("a reported `unknown` is not a promptable state")
	}
	for _, want := range []string{"nothing was sent", "plugged", "unknown", "posse peek plugged", "--now"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q:\n%v", want, err)
		}
	}
}
