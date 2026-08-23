package rhq

// Startup screens (rangerhq-7sbo): grok opens on a splash that owns the
// keyboard and never dismisses itself, so the wait-for-settled gate in
// dispatch — right, and now honestly answered `blocked` by our detection
// override — made grok undispatchable. The launcher clears the screens it
// knows by rule id, and nothing else: the levers here are wait-status (what
// herdr settles on), explain-rule (which rule produced it) and
// send-keys-clears (whether the key actually works).

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

// The whole point of the bead: a fresh grok pane is blocked on its splash,
// the launcher presses Esc, and the bead is dispatched.
func TestDispatchClearsGrokStartupSplash(t *testing.T) {
	b, fake := newTestBackend(t)
	d := splashRepo(t, b, fake, "startup_splash")
	os.WriteFile(filepath.Join(fake, "send-keys-clears"), nil, 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "agent send-keys w1:p1 esc") {
		t.Errorf("the splash was never cleared:\n%s", log)
	}
	if !strings.Contains(log, "agent prompt") || n != 1 {
		t.Errorf("want the bead dispatched after the splash cleared, got n=%d:\n%s\n%s", n, dispatcherOut(d), log)
	}
	if !strings.Contains(dispatcherOut(d), "clearing the startup screen") {
		t.Errorf("clearing a screen must be said out loud:\n%s", dispatcherOut(d))
	}
	// Esc, and only Esc: Enter is what the splash eats, and its menu and the
	// consent banner's [Opt in] are on that same screen (rangerhq-sz7u).
	if strings.Count(log, "agent send-keys") != 1 || strings.Contains(log, "send-keys w1:p1 enter") {
		t.Errorf("exactly one Esc may be sent, and never Enter:\n%s", log)
	}
}

// A blocker that is not a startup screen is the operator's: dispatch waits,
// fails loudly, and never touches the keyboard. Answering a permission
// dialog on the operator's behalf is the failure this fix must not become.
func TestDispatchLeavesUnknownBlockerAlone(t *testing.T) {
	b, fake := newTestBackend(t)
	d := splashRepo(t, b, fake, "permission_scope_selector")
	os.WriteFile(filepath.Join(fake, "send-keys-clears"), nil, 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	log := calls(t, fake)
	if strings.Contains(log, "agent send-keys") {
		t.Errorf("a dialog posse does not know must never be answered for the operator:\n%s", log)
	}
	if n != 0 || !strings.Contains(dispatcherOut(d), "never settled") {
		t.Errorf("want a loud failure, got n=%d:\n%s", n, dispatcherOut(d))
	}
	if strings.Contains(log, "agent prompt") {
		t.Errorf("prompt fired into a blocked agent:\n%s", log)
	}
}

// grok's splash does not undraw when its key is pressed: Esc moves the focus
// into the composer, the menu and banner stay on screen, and herdr goes on
// reporting the same rule for a pane that takes a prompt perfectly well
// (measured live, rangerhq-7sbo). The same screen after the key means ready.
func TestDispatchPromptsThroughAStillDrawnStartupScreen(t *testing.T) {
	b, fake := newTestBackend(t)
	d := splashRepo(t, b, fake, "startup_splash")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	log := calls(t, fake)
	if got := strings.Count(log, "agent send-keys"); got != 1 {
		t.Errorf("want exactly one keypress, got %d:\n%s", got, log)
	}
	if n != 1 || !strings.Contains(log, "agent prompt") {
		t.Errorf("a still-drawn startup screen must not cost the dispatch, got n=%d:\n%s\n%s", n, dispatcherOut(d), log)
	}
	if !strings.Contains(dispatcherOut(d), "still drawn") {
		t.Errorf("prompting past a screen herdr still calls blocked must be said out loud:\n%s", dispatcherOut(d))
	}
}

// What is behind the startup screen is not posse's to interpret. A different
// blocker after the keypress is the operator's, and the launch fails loudly
// rather than typing a work prompt into whatever that is.
func TestDispatchStopsWhenAnotherBlockerIsBehindTheStartupScreen(t *testing.T) {
	b, fake := newTestBackend(t)
	d := splashRepo(t, b, fake, "startup_splash")
	os.WriteFile(filepath.Join(fake, "send-keys-rule"), []byte("permission_scope_selector"), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	log := calls(t, fake)
	if got := strings.Count(log, "agent send-keys"); got != 1 {
		t.Errorf("want exactly one keypress, got %d:\n%s", got, log)
	}
	if n != 0 || !strings.Contains(dispatcherOut(d), "never settled") {
		t.Errorf("want a loud failure, got n=%d:\n%s", n, dispatcherOut(d))
	}
	if strings.Contains(log, "agent prompt") {
		t.Errorf("prompt fired into a dialog posse does not know:\n%s", log)
	}
}

// Every rule posse presses a key at must exist in the shipped manifest, and
// the key must be one herdr names. A renamed rule is a fix that silently
// stops working — the launch would go back to failing on the splash.
func TestStartupScreenRulesExistInTheManifests(t *testing.T) {
	for rule, keys := range startupScreenDismissals {
		if len(keys) == 0 {
			t.Errorf("%s: no keys to press", rule)
		}
		for _, k := range keys {
			if k != "esc" {
				t.Errorf("%s: %q — only Esc is safe on a screen nobody is watching", rule, k)
			}
		}
		found := false
		for _, m := range []string{"grok.toml", "codex.toml"} {
			b, err := os.ReadFile(filepath.Join("..", "..", "etc", "herdr", "agent-detection", m))
			if err != nil {
				continue
			}
			if strings.Contains(string(b), `id = "`+rule+`"`) {
				found = true
			}
		}
		if !found {
			t.Errorf("no rule %q in etc/herdr/agent-detection — dispatch presses keys at a screen no manifest reports", rule)
		}
	}
}
