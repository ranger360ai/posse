//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin filed while verifying the close of ranger-base-5qe6 under
// ranger-base-w7h58.
//
// That close fixed BOTH halves of pulse's addressing — the AgentPrompt at
// pulse.go's delivery and the AgentExplain underneath
// AwaitPromptable — but only the prompt half is pinned: reverting
// `d.HB.AwaitPromptable(name, pane)` to the pre-fix `(name, name)` leaves
// the whole pulse suite green (measured). The fake herdr answers `agent
// explain <anything>`, which is the same blindness that let the original
// bug ship: real herdr answers agent_not_found for a session LABEL and
// only resolves a pane id.
//
// Measured against the real herdr on 2026-09-01 at this tree's HEAD:
//   herdr agent explain <session-label> -> {"error":{"code":"agent_not_found"}}
//   herdr agent explain <workspace>:p1  -> agent: claude / state: working
//
// Live, the unpinned half is not a lost pulse but a stalled one:
// AwaitPromptable treats an erroring `agent explain` as "never answered"
// and burns its whole promptReadyWait window on every tick before
// prompting anyway.
import (
	"strings"
	"testing"
	"time"
)

func TestQAPulseAddressesAgentExplainByPaneNotSessionLabel(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	name := "coordinator-work"
	id := personaSession(t, b, fake, name, "coordinator", "idle", false)
	pane := id + ":p1"
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	d.pulseOnce(PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute})

	log := calls(t, fake)
	if !strings.Contains(log, "agent explain "+pane) {
		t.Errorf("promptability must be asked about the PANE — real herdr answers agent_not_found for a session label (ranger-base-5qe6):\n%s", log)
	}
	if strings.Contains(log, "agent explain "+name) {
		t.Errorf("`agent explain %s` addresses the session label; real herdr 404s it and AwaitPromptable then burns its whole wait window every tick:\n%s", name, log)
	}
	// The prompt half, asserted here too so this pin names the whole
	// contract in one place rather than half of it.
	if !strings.Contains(log, "agent prompt "+pane) || strings.Contains(log, "agent prompt "+name+" ") {
		t.Errorf("the prompt must be addressed to the pane, not the session label:\n%s", log)
	}
}
