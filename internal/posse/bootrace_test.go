//go:build !posse_arm2 && !posse_arm3

package posse

// The boot race (rangerhq-3hb5): herdr answers `idle` for a pane it has
// identified as a known agent even when no rule matched anything, and during
// a launch that guess arrives BEFORE the CLI does — measured on a
// dispatch-shaped grok launch, herdr said agent=grok/idle at 0.20s over the
// shell's prompt line, and grok did not take the screen until 0.39s. The old
// gate (`agent wait --until idle|done|blocked`) returned on that guess, so
// the work prompt was typed at a shell, buffered through the exec, and
// delivered somewhere nobody chose. That is the failure behind rangerhq-37c
// and the lost dispatch in rangerhq-5on; calling grok's splash `blocked`
// never covered it, because the splash is not drawn until 0.6s later
// (rangerhq-1xsj).
//
// The lever is explain-fallback: how many `agent explain` calls answer with
// herdr's guess (matched_rule null, visible_idle false,
// default_known_agent_idle_fallback) before it can see the screen.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// raceRepo is splashRepo's settled sibling: one ready bead, a persona, and a
// pane herdr calls idle — the common path, which is where the race lives.
func raceRepo(t *testing.T, b *HerdrBackend, fake string) *Dispatcher {
	t.Helper()
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"grok","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)
	return d
}

// The bead itself: the launch holds through herdr's guess and prompts on the
// first state herdr can actually see. Nothing is typed in between — the race
// is lost by typing, not by waiting.
func TestDispatchWaitsThroughHerdrsIdleGuess(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	os.WriteFile(filepath.Join(fake, "explain-fallback"), []byte("3"), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	log := calls(t, fake)
	if n != 1 || !strings.Contains(log, "agent prompt") {
		t.Fatalf("the bead must still be dispatched once herdr sees the screen, got n=%d:\n%s\n%s", n, dispatcherOut(d), log)
	}
	prompt := strings.Index(log, "agent prompt")
	guesses := strings.Count(log[:prompt], "agent explain")
	if guesses < 4 { // three guesses, then the seen state that opened the gate
		t.Errorf("the gate returned on a guess: only %d explains before the prompt:\n%s", guesses, log)
	}
	if strings.Contains(log[:prompt], "agent send-keys") {
		t.Errorf("nothing may be typed at a pane whose screen herdr cannot read:\n%s", log)
	}
}

// A pane herdr only ever guesses about is the launch that must not happen:
// the screen is unrecognized, so nobody knows what the prompt would be typed
// into. It fails loudly, and it says which pane and why.
func TestDispatchRefusesAPaneHerdrOnlyGuessesIdleFor(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	d.StartupWait = 300 * time.Millisecond
	os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644) // guess forever

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	log := calls(t, fake)
	if strings.Contains(log, "agent prompt") {
		t.Errorf("a work prompt was typed at a screen herdr never recognized:\n%s", log)
	}
	out := dispatcherOut(d)
	if n != 0 || !strings.Contains(out, "never saw a screen it recognizes") {
		t.Errorf("want a loud failure naming the guess, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "default_known_agent_idle_fallback") {
		t.Errorf("herdr's own word for the guess is the diagnosis — say it:\n%s", out)
	}
}

// Detection that cannot be read is not evidence of unreadiness. A launcher
// that refuses to launch because a diagnostic call failed is worse than the
// race it guards, so the prompt goes out — out loud, naming the error.
func TestDispatchPromptsOutLoudWhenExplainCannotBeRead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	d.StartupWait = 300 * time.Millisecond
	os.WriteFile(filepath.Join(fake, "explain-error"), []byte("internal|no detection for w1:p1"), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || !strings.Contains(calls(t, fake), "agent prompt") {
		t.Fatalf("a broken `agent explain` must not strand the fleet, got n=%d:\n%s", n, dispatcherOut(d))
	}
	if !strings.Contains(dispatcherOut(d), "herdr cannot explain") {
		t.Errorf("prompting without detection must be said out loud:\n%s", dispatcherOut(d))
	}
}

// `agent wait` and `agent explain` are one detection sampled twice, and they
// disagree for real: measured live, a fresh claude settled idle and had drawn
// its trust dialog 30ms later, by the time explain answered. The fresher read
// wins — typing a work prompt into a dialog is the failure the gate exists to
// prevent, not a launch worth saving.
func TestDispatchTakesExplainOverTheWaitWhenTheyDisagree(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	d.StartupWait = 300 * time.Millisecond
	os.WriteFile(filepath.Join(fake, "explain-state"), []byte("blocked"), 0o644)
	os.WriteFile(filepath.Join(fake, "explain-rule"), []byte("live_blocked_form"), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	log := calls(t, fake)
	if strings.Contains(log, "agent prompt") {
		t.Errorf("a work prompt was typed at a dialog herdr had already drawn:\n%s", log)
	}
	if strings.Contains(log, "agent send-keys") {
		t.Errorf("a dialog posse does not know must never be answered for the operator:\n%s", log)
	}
	if n != 0 || !strings.Contains(dispatcherOut(d), "never settled") {
		t.Errorf("want a loud failure, got n=%d:\n%s", n, dispatcherOut(d))
	}
}

// The decoder, pinned against real `agent explain --json` output. Both
// captures are herdr 0.8.0: the guess is the 0.20s window of the launch
// timeline, the seen one is a settled claude pane. `matched_rule: null` is
// the shape that must not read as readiness.
func TestAgentDetectionSeenVersusGuess(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		json string
		seen bool
	}{
		{"guess", `{"fallback_reason":"default_known_agent_idle_fallback","matched_rule":null,` +
			`"state":"idle","visible_blocker":false,"visible_idle":false,"visible_working":false}`, false},
		{"seen", `{"fallback_reason":null,"matched_rule":{"id":"live_prompt_box","priority":950,` +
			`"region":"prompt_box_body","state":"idle"},"state":"idle","visible_blocker":false,` +
			`"visible_idle":true,"visible_working":false}`, true},
	} {
		var det AgentDetection
		if err := json.Unmarshal([]byte(tc.json), &det); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if det.State != "idle" {
			t.Errorf("%s: state %q", tc.name, det.State)
		}
		if det.Seen() != tc.seen {
			t.Errorf("%s: Seen()=%v want %v (rule %q visible_idle=%v)", tc.name, det.Seen(), tc.seen, det.Rule.ID, det.VisibleIdle)
		}
	}
}
