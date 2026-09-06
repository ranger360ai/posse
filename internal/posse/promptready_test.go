//go:build posse_arm3

package posse

// ranger-base-3p0: `posse prompt` typed into a pane herdr had only GUESSED
// was idle, and the CLI — not yet holding the keyboard — turned the work
// prompt into `/Work` plus arguments. herdr reported success. These pin the
// gate that now stands between AgentTarget and AgentPrompt (promptready.go).
//
// The levers are fakeExplain's, and the difference they drive is the whole
// bug: `explain-fallback` is herdr's guess (matched_rule null, visible_idle
// false, default_known_agent_idle_fallback), the default is a seen screen.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// promptReadySession gives a session a runtime with a short declared
// patience, so a test that measures the REFUSAL takes a fraction of a
// second instead of the claude-shaped 45. It is the same lever a slow CLI
// uses in production, not a test-only door into the gate.
func promptReadySession(t *testing.T, b *HerdrBackend, name, wait string) {
	t.Helper()
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuntime(t, b.App, "fastcli", "command: fastcli --sys {file}\nstartup_wait: "+wait+"\n")
	if err := os.MkdirAll(b.metaDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.metaPath(name),
		[]byte("name: "+name+"\nworkspace: w1\npane: w1:p1\nruntime: fastcli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The bug itself. A pane herdr never recognizes is not promptable, and the
// gate says so with nothing typed — the opposite of the incident, where the
// text went in and the call returned success.
func TestPromptGateRefusesAPaneHerdrOnlyGuessesAt(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	promptReadySession(t, b, "fresh", "400ms")
	// Guess forever: the CLI never draws a screen posse knows.
	if err := os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// herdr's own working, which the failure line owes the next reader.
	if err := os.WriteFile(filepath.Join(fake, "explain-rules"),
		[]byte(`[{"id":"live_prompt_box","matched":false,"region":"osc_title","state":"idle",`+
			`"evidence":{"region_bytes":0,"region_preview":""}}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, note, err := b.AwaitPromptable("fresh", "w1:p1")
	if err == nil {
		t.Fatalf("a guessed screen must not be prompted; got note %q", note)
	}
	for _, want := range []string{"nothing was sent", "fresh", "400ms", "default_known_agent_idle_fallback", "posse peek fresh", "--now"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q:\n%v", want, err)
		}
	}
	if !strings.Contains(err.Error(), "osc_title") {
		t.Errorf("the refusal must carry herdr's working (WhatHerdrSaw):\n%v", err)
	}
	// It kept reading until the runtime's own patience was spent — a gate
	// that refused on the first guess would refuse every cold start. The
	// last poll is not started when it cannot finish inside the deadline
	// (awaitSettled's loop shape), so the floor is one poll short of it.
	if waited := time.Since(start); waited < 400*time.Millisecond-promptReadyPoll {
		t.Errorf("gave up after %s, on a runtime that declared 400ms of patience", waited)
	}
}

// The wrong arm: with no lever, the fake answers the shape a settled pane
// really has — a named rule — and the gate opens at once. A gate that
// refused here would break every ordinary prompt, so this is what says the
// test above measured the guess and not the mechanism.
func TestPromptGatePassesAPaneHerdrHasSeen(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	promptReadySession(t, b, "live", "400ms")

	start := time.Now()
	_, note, err := b.AwaitPromptable("live", "w1:p1")
	if err != nil {
		t.Fatalf("a seen screen must be promptable: %v", err)
	}
	if note != "" {
		t.Errorf("a prompt that waited for nothing has nothing to report: %q", note)
	}
	if waited := time.Since(start); waited >= promptReadyPoll {
		t.Errorf("an established session paid %s for the gate — it must cost one explain", waited)
	}
}

// The gate waits for a SEEN screen, not for `idle`. Nudging a session that
// is mid-turn is what `posse prompt` is for half the time, and holding that
// text until the turn ended would be a behaviour nobody asked for: a
// working pane herdr recognizes is a pane whose CLI has the keyboard.
func TestPromptGateDoesNotWaitForIdle(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	promptReadySession(t, b, "busy", "400ms")
	if err := os.WriteFile(filepath.Join(fake, "explain-state"), []byte("working"), 0o644); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if _, _, err := b.AwaitPromptable("busy", "w1:p1"); err != nil {
		t.Fatalf("a working agent herdr can see must still take a prompt: %v", err)
	}
	if waited := time.Since(start); waited >= promptReadyPoll {
		t.Errorf("the gate held a working session for %s — it waits for a seen screen, not for idle", waited)
	}
}

// The boot race as it actually resolves: two guesses, then the CLI draws
// and herdr recognizes it. The prompt goes in, late and out loud — the
// second `posse prompt` the operator had to make by hand on the incident.
func TestPromptGateWaitsOutABootThenPrompts(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	promptReadySession(t, b, "booting", "5s")
	if err := os.WriteFile(filepath.Join(fake, "explain-fallback"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, note, err := b.AwaitPromptable("booting", "w1:p1")
	if err != nil {
		t.Fatalf("the CLI drew its screen on the third read; the prompt must go: %v", err)
	}
	for _, want := range []string{"waited", "booting", "fake_idle"} {
		if !strings.Contains(note, want) {
			t.Errorf("a gate that held must say so and name what opened it (%q): %q", want, note)
		}
	}
}

// A diagnostic that will not answer is not evidence about the pane. The
// gate prompts anyway and names the error — and it does NOT spend the
// runtime's whole patience finding out, because an `agent explain` that has
// never once answered is a verb this herdr does not have, not a
// measurement in progress. 30s of silence per hand prompt against an older
// herdr would be a worse regression than the bug.
func TestPromptGateProceedsWhenHerdrCannotExplain(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	promptReadySession(t, b, "opaque", "30s")
	if err := os.WriteFile(filepath.Join(fake, "explain-error"),
		[]byte("bad_request|unknown subcommand explain"), 0o644); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, note, err := b.AwaitPromptable("opaque", "w1:p1")
	if err != nil {
		t.Fatalf("a failed diagnostic must not refuse the prompt: %v", err)
	}
	for _, want := range []string{"cannot explain", "opaque", "unknown subcommand explain"} {
		if !strings.Contains(note, want) {
			t.Errorf("the concession must name %q out loud: %q", want, note)
		}
	}
	if waited := time.Since(start); waited >= 30*time.Second {
		t.Errorf("a herdr with no explain cost the full patience (%s) — the concession is bounded", waited)
	}
}

// The patience is the session's own runtime's, the same number its launch
// got (Runtime.Wait). Unknown falls back to the ordinary patience, never to
// none: a session with no meta must not be gated to zero and refused.
func TestPromptGateWaitIsTheSessionsRuntimePatience(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	promptReadySession(t, b, "declared", "400ms")

	if got := b.promptReadyWait("declared"); got != 400*time.Millisecond {
		t.Errorf("the runtime's declared startup_wait must be the gate's patience: %s", got)
	}
	if got := b.promptReadyWait("no-such-session"); got != DefaultStartupWait {
		t.Errorf("a session with no meta must get the default patience, not %s", got)
	}
}
