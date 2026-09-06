//go:build posse_arm2

package posse

// rangerhq-6bbz: herdr's workspace-id allocator is max(live id)+1, recomputed
// from the live set at every server process start — a restart and a
// live-handoff both (measured rangerhq-6bg7, re-probed rangerhq-6bbz).
// scripts/verify-id-recycle.sh is the live pin: a scratch --session server,
// never the fleet. This file pins that the script still asserts the table
// that makes WorkspaceAlive(id) a liveness answer, not an identity.
//
// Live run (scratch session, fleet is only snapshotted):
//
//	scripts/verify-id-recycle.sh
//	RHQ_LIVE_IDRECYCLE=1 go test ./internal/rhq -run TestQAIdRecycleScriptPassesAgainstAScratchServer -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func idRecycleScript(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "scripts", "verify-id-recycle.sh")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestQAIdRecycleScriptPinsAllocatorTable(t *testing.T) {
	t.Parallel()
	s := idRecycleScript(t)
	// Safety: the script must refuse the fleet socket, not just prefer a name.
	for _, needle := range []string{
		"Never aims at the fleet default server",
		"REFUSING handoff",
		"FLEET_SOCK",
		"unset HERDR_ENV HERDR_SOCKET_PATH",
		"session delete",
	} {
		if !strings.Contains(s, needle) {
			t.Errorf("verify-id-recycle.sh no longer contains %q — the pin can aim at the fleet or leak a session", needle)
		}
	}
	// The table rangerhq-6bg7 measured and rangerhq-6bbz re-probed.
	for _, needle := range []string{
		"fresh-create-x4",
		"w1 w2 w3 w4",
		"same-process-no-reuse",
		"w5 w6",
		"same-process-does-not-fill-hole",
		"want=w7 not=w2",
		"restart-recycles-id-above-live-high-water",
		"want=w8",
		"close-everything-restart-resets-to-w1",
		"handoff-recycles-closed-max",
		"want=w3",
		"live-handoff --import-exe",
		"fleet-workspace-ids-untouched",
		"workspace_not_found",
	} {
		if !strings.Contains(s, needle) {
			t.Errorf("verify-id-recycle.sh no longer asserts %q — the 6bg7 table is what posse's identity fence is for", needle)
		}
	}
}

func TestQAIdRecycleScriptPassesAgainstAScratchServer(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_IDRECYCLE") == "" {
		t.Skip("set RHQ_LIVE_IDRECYCLE=1 to run scripts/verify-id-recycle.sh against a scratch herdr session")
	}
	script := filepath.Join("..", "..", "scripts", "verify-id-recycle.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "HERDR_SESSION=", "HERDR_SOCKET_PATH=", "HERDR_ENV=")
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatalf("verify-id-recycle.sh: %v", err)
	}
	if !strings.Contains(string(out), "verify-id-recycle: PASS") {
		t.Fatalf("script exited 0 without PASS:\n%s", out)
	}
}
