package rhq

// Live probe for rangerhq-ouf9: does herdr tolerate the instance separator
// in a workspace label at all?
//
//	RHQ_LIVE_HERDR=1 go test ./internal/rhq -run TestLiveHerdrKeepsASlashInALabel -v
//
// The design that introduced `instance:` left exactly one assumption open
// and said to probe it before building: labels are herdr's, not posse's, and
// a server that sanitized or split on '/' would silently give every tagged
// session a label nothing matches — which is worse than the collision the
// tag removes, because it breaks the fleet that has no second instance.
//
// MEASURED, herdr 0.8.0 (protocol 19), macOS, 2026-08-27: `workspace create
// --label 'probe-ouf9/slash'` answers with `"label":"probe-ouf9/slash"`
// verbatim; the same string comes back from `workspace list` and from
// `workspace get <id>`; and the workspace closes by id like any other. So
// the separator is '/' and nothing else in the design moves.
//
// It runs against the operator's own herdr, so it creates exactly one
// unfocused workspace and closes it in a defer — the same create/close pair
// `posse new` and `posse kill` make.

import (
	"os"
	"strings"
	"testing"
)

func TestLiveHerdrKeepsASlashInALabel(t *testing.T) {
	if os.Getenv("RHQ_LIVE_HERDR") == "" {
		t.Skip("set RHQ_LIVE_HERDR=1 (creates and closes one workspace on the real herdr)")
	}
	h := NewHerdr()
	if !h.Available() {
		t.Skip("no herdr on this host")
	}
	const label = "probe-ouf9" + InstanceSep + "slash"

	id, pane, err := h.CreateWorkspace(label, "", nil)
	if err != nil {
		t.Fatalf("create with a %q in the label: %v", InstanceSep, err)
	}
	t.Cleanup(func() {
		if err := h.CloseWorkspace(id); err != nil {
			t.Errorf("the probe workspace %s (%s) was left behind: %v", id, label, err)
		}
	})
	if pane == "" {
		t.Errorf("no root pane for %s", id)
	}

	// The label as the create itself reports it is not enough: what every
	// matching path reads is the listing and the per-id query.
	ws, found, err := h.WorkspaceGet(id)
	if err != nil || !found {
		t.Fatalf("workspace get %s: found=%v %v", id, found, err)
	}
	if ws.Label != label {
		t.Errorf("workspace get returned label %q, want %q — the separator does not survive", ws.Label, label)
	}

	wss, err := h.Workspaces()
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	for _, w := range wss {
		if w.WorkspaceID == id {
			if w.Label != label {
				t.Errorf("workspace list returned label %q, want %q", w.Label, label)
			}
			listed = append(listed, w.Label)
		}
	}
	if len(listed) != 1 {
		t.Errorf("the probe workspace is listed %d times: %s", len(listed), strings.Join(listed, ", "))
	}
}
