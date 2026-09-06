//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// workingForeignHolder is foreignHolder with the workspace's agent BUSY.
// A foreign row's Status is read straight off the workspace listing
// (herdrback.go:624-628 — an agent row plus its `agent_status`), so "working"
// is as reachable a shape as "idle": the operator's own conversation, meta
// wiped, mid-turn.
func workingForeignHolder(t *testing.T, fake, name string) {
	t.Helper()
	ws := append(fakeLoadWSFrom(t, fake), fakeWS{WorkspaceID: "wForeign", Label: name, AgentStatus: "working"})
	saveWSTo(t, fake, ws)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"working","pane_id":"wForeign:p1","workspace_id":"wForeign"}]`), 0o644)
}

// rangerhq-ynx8 verify (ranger-base-pcb1): the LaunchBead foreign guard had
// no arm of its own. TestLaunchBeadRefusesAForeignHolder asserts a message
// launchSession's backstop emits VERBATIM — same sentence, same session name
// — so deleting the guard at LaunchBead left every foreign pin green
// (measured: comment out `if held := d.foreignHeld(guard...)` and all four
// arms still pass). A pin that survives the mutation it names pins nothing.
//
// This is the arm that separates them: with the workspace BUSY, LaunchBead's
// own guard sits ABOVE the working/blocked refusal, so the guard's absence
// changes the sentence — "is working — not dispatched" instead of the
// foreign line — and the ownership answer is replaced by a liveness one.
// The distinction is not cosmetic: "working" reads as our session, busy now,
// retry later; the foreign line says the name belongs to someone else and
// names the two ways to get it back.
func TestQALaunchBeadRefusesAForeignHolderAboveTheStatusCheck(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}
	session := SessionForBead("ranger", repo, "a-1")
	workingForeignHolder(t, fake, session)

	_, err := d.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "held by a foreign workspace "+session) {
		t.Fatalf("ownership is answered before liveness: want the foreign refusal, got %v", err)
	}
	if strings.Contains(err.Error(), "is working") {
		t.Errorf("a foreign row was refused as OUR busy session, not as somebody else's: %v", err)
	}
	if !strings.Contains(err.Error(), "posse kill "+session) {
		t.Errorf("refusal does not say how to free the name: %v", err)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("a work prompt reached the foreign pane:\n%s", log)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("bead claimed onto a foreign workspace:\n%s", log)
	}
}
