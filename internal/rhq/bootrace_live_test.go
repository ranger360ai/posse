package rhq

// Live pin for rangerhq-3hb5: the readiness gate must not return on herdr's
// idle GUESS. The hermetic cases in bootrace_test.go drive a fake herdr that
// this repo also writes, so they prove the gate's logic and nothing about the
// window it exists for. This one launches the CLI itself and runs awaitAgent
// across the boot race, which is the only way to see it.
//
// Unlike livesplash_test.go, which is handed a pane that has already settled,
// this test does the `pane run` — the racy half. Recipe:
//
//	herdr --session <s> server &                       # scratch, not the fleet
//	export HERDR_SOCKET_PATH=~/.config/herdr/sessions/<s>/herdr.sock
//	herdr workspace create --cwd <scratch> --label qalive --no-focus  # ROOT pane id
//	RHQ_LIVE_SHELL_PANE=<pane> go test ./internal/rhq -run TestLiveAwaitAgentHolds -v
//
// RHQ_LIVE_CMD overrides the command (default: the fleet's grok line, which
// is where the race was measured). It never prompts and never presses Enter,
// so it costs no agent turn — grok's menu and the consent banner's [Opt in]
// share that screen (rangerhq-sz7u). Tear down with `herdr workspace close`
// + `herdr server stop`.
//
// What it asserts is the fix in one line: the state the gate opened on is one
// herdr could SEE. Before the fix the gate opened at ~0.2s on
// default_known_agent_idle_fallback over the shell's own prompt line, and
// that assertion is what fails. Measured both ways on grok 1.0.5
// (rangerhq-3hb5): HEAD's gate 0.26s / no rule, this one 0.49s /
// osc_title_idle.
//
// It drives awaitSettled, the gate itself, and not awaitAgent: the accepted
// detection is the thing under test, and re-reading the pane a moment later
// asks a different question — a fresh claude drew a dialog 30ms after the
// gate opened, which is a real thing that happens and not this bug.
// awaitAgent's other half, finding the agent, is pinned by livesplash_test.

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveAwaitAgentHoldsThroughTheBootRace(t *testing.T) {
	pane := os.Getenv("RHQ_LIVE_SHELL_PANE")
	if pane == "" {
		t.Skip("set RHQ_LIVE_SHELL_PANE=<ws:pane> at a shell (+ HERDR_SOCKET_PATH, RHQ_HERDR_BIN) — see the file comment")
	}
	cmd := os.Getenv("RHQ_LIVE_CMD")
	if cmd == "" {
		cmd = "grok --permission-mode auto"
	}
	b := liveBackend(t, pane)

	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.StartupWait = 30 * time.Second
	d.Poll = 200 * time.Millisecond

	start := time.Now()
	if err := b.H.PaneRun(pane, cmd); err != nil {
		t.Fatalf("pane run %s %q: %v", pane, cmd, err)
	}
	deadline := start.Add(d.StartupWait)

	// The dispatch-shaped launch races detection too: herdr has no agent in
	// this pane until the CLI execs. awaitAgent's loop, inlined.
	var target string
	for {
		t2, err := b.AgentTarget("qalive")
		if err == nil {
			target = t2
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no agent detected in %s after %s: %v", pane, d.StartupWait, err)
		}
		time.Sleep(d.Poll)
	}

	status, det, err := d.awaitSettled("live-3hb5", "qalive", target, []string{"idle", "done", "blocked"}, deadline, d.StartupWait)
	elapsed := time.Since(start)
	t.Logf("the gate opened after %s on %q: rule=%q visible_idle=%v fallback=%q err=%v\n%s",
		elapsed, status, det.Rule.ID, det.VisibleIdle, det.FallbackReason, err, out.String())
	if err != nil {
		t.Fatalf("the gate refused a pane that booted normally: %v", err)
	}
	if !det.Seen() {
		t.Fatalf("the gate opened on a guess after %s: %q with no rule (%s) — the screen may still be the shell's",
			elapsed, status, det.FallbackReason)
	}
}
