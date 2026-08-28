package rhq

// Startup screens, retired (rangerhq-6723). grok opened on a splash our
// detection override called `blocked` (rangerhq-37c/7sbo), so the launcher
// pressed Esc at it by rule id — startupScreenDismissals, ADR 0013 §2 layer
// 3, "declared keystrokes". Two later measurements took the ground out from
// under that: the splash is decoration over a live composer and reports
// `idle` now (rangerhq-1xsj), and it is not even drawn until 0.6s after the
// readiness gate opens, so the branch never fired in a launch (rangerhq-3hb5).
// The machinery went with it; what remains here is the fence it left behind.
//
// THE FENCE, and why it is still executable. security ruled on rangerhq-4mzt
// that the launcher may never answer a drawn dialog: "1/Enter" at claude's
// trust dialog is a capability grant made blind, and that dialog matches
// herdr's GENERIC `live_blocked_form`, so a dismissal entry for it would
// answer every form claude ever draws. The old test carried that ruling as
// two assertions over the dismissal table (only Esc; only rule ids posse's
// own manifests carry). With no table there is nothing to constrain, so the
// ruling is pinned here as behaviour instead, and the harder way: dispatch
// answers NO blocked screen, including the one it used to answer. If ADR
// 0013 layer 3 is ever taken up again for another agent, security's two
// assertions come back with the table.
//
// The levers are the same fake-herdr ones: wait-status (what herdr settles
// on) and explain-rule (which rule produced it).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// splashRepo sets up one ready bead, a persona for it, and a session whose
// agent herdr reports blocked on the given detection rule.
func splashRepo(t *testing.T, b *HerdrBackend, fake, rule string) *Dispatcher {
	t.Helper()
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"grok","agent_status":"blocked","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)
	os.WriteFile(filepath.Join(fake, "wait-status"), []byte("blocked"), 0o644)
	os.WriteFile(filepath.Join(fake, "explain-rule"), []byte(rule), 0o644)
	return d
}

// A blocked agent is the operator's, whatever screen it is sitting on — the
// dialog posse never knew (a permission prompt), the generic form claude's
// trust dialog matches, and `startup_splash`, the one screen posse used to
// press a key at. Dispatch waits, fails loudly, and never touches the
// keyboard. Answering a dialog on the operator's behalf is the failure this
// code must not grow back into (rangerhq-4mzt).
func TestDispatchAnswersNoBlockedScreen(t *testing.T) {
	for _, rule := range []string{
		"permission_scope_selector",
		"live_blocked_form",
		"startup_splash",
	} {
		t.Run(rule, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := splashRepo(t, b, fake, rule)
			// The fake would clear on a keypress. Nothing presses one.
			os.WriteFile(filepath.Join(fake, "send-keys-clears"), nil, 0o644)

			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			log := calls(t, fake)
			if strings.Contains(log, "agent send-keys") {
				t.Errorf("a key was pressed at %s — no screen is the launcher's to answer:\n%s", rule, log)
			}
			if strings.Contains(log, "agent prompt") {
				t.Errorf("prompt fired into a blocked agent:\n%s", log)
			}
			if n != 0 || !strings.Contains(dispatcherOut(d), "never settled") {
				t.Errorf("want a loud failure, got n=%d:\n%s", n, dispatcherOut(d))
			}
		})
	}
}

// The retirement itself, so it cannot be half-undone. A dismissal table is
// only safe with security's two assertions attached (see the file comment), and
// those assertions are gone because the table is. Bringing back a key press
// in the launch path without them is the regression this pins: `AgentSendKeys`
// stays a herdr binding with no caller in dispatch.
func TestDispatchPathPressesNoKeys(t *testing.T) {
	src, err := os.ReadFile("dispatch.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, dead := range []string{"AgentSendKeys", "startupScreenDismissals", "clearStartupScreen"} {
		if strings.Contains(string(src), dead) {
			t.Errorf("dispatch.go names %s again — rangerhq-6723 retired it, and rangerhq-4mzt's two assertions (only Esc, only a rule id from posse's own manifests) must come back with any table that presses a key", dead)
		}
	}
}
