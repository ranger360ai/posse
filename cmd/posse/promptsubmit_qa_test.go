package main

// QA pin for ranger-base-htafy's second half, on the entry point the
// incident actually used: `posse prompt`.
//
// MEASURED three times on 2026-09-02: herdr returned
// agent_prompted, the text was typed into the composer, and the submit
// never happened — on the last occurrence it sat there for four hours while
// every store reported the agent idle, and `herdr agent send-keys <pane>
// enter` did not submit it either. A caller that types and reads the return
// value has no way to know. So the command reads the box back afterwards
// and says what is still in it, and warns before typing when the box
// already holds someone else's unsent prompt — the two texts would
// otherwise be typed into one message.
//
// Neither is a refusal, deliberately: the measured RECOVERY from this state
// was a hand `posse prompt`, and a gate in front of it would block the fix
// along with the mistake.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// promptSubmitHerdr writes a fake herdr whose `agent explain` reports an
// idle pane whose composer holds box, and returns the home it was set up
// in. The shape of the explain answer is a real capture's (herdr 0.8.2):
// state, the matched rule, and evaluated_rules carrying each rule's screen
// region and a preview of it.
func promptSubmitHerdr(t *testing.T, box string) (home, bin string) {
	t.Helper()
	home = t.TempDir()
	binDir := t.TempDir()
	herdr := filepath.Join(binDir, "herdr")
	script := `#!/bin/sh
case "$1 $2" in
"workspace list")
  printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1","label":"fresh","agent_status":"idle"}]}}'
  exit 0;;
"agent list")
  printf '%s\n' '{"result":{"agents":[{"agent":"claude","agent_status":"idle","pane_id":"p1","workspace_id":"w1"}]}}'
  exit 0;;
"agent prompt")
  printf '%s\n' '{"result":{"type":"agent_prompted","agent":{"agent_status":"idle"}}}'
  exit 0;;
"agent explain")
  printf '%s\n' '{"state":"idle","matched_rule":{"id":"live_prompt_box"},"visible_idle":true,"fallback_reason":null,"evaluated_rules":[{"id":"live_blocked_form","matched":false,"region":"after_last_horizontal_rule","state":"blocked","evidence":{"region_bytes":52,"region_preview":"  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents\n"}},{"id":"live_prompt_box","matched":true,"region":"prompt_box_body","state":"idle","evidence":{"region_bytes":8,"region_preview":"` + box + `"}}]}'
  exit 0;;
esac
printf '%s\n' '{"error":{"code":"no","message":"unexpected"}}'
exit 1
`
	if err := os.WriteFile(herdr, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	metaDir := filepath.Join(home, "state", "herdr")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "fresh.yaml"),
		[]byte("name: fresh\nworkspace: w1\npane: p1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return home, herdr
}

func runPrompt(t *testing.T, home, herdr string) string {
	t.Helper()
	cmd := exec.Command(buildRhq(t), "prompt", "fresh", "commit and close the bead once the suite is green")
	cmd.Env = append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+herdr,
		"HERDR_SOCKET_PATH="+filepath.Join(home, "no-such.sock"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("posse prompt: %v\n%s", err, out)
	}
	return string(out)
}

// The incident. The prompt is typed, herdr says agent_prompted, and the box
// still holds text — which the operator is told, with the text, and where
// to look.
func TestQAPromptReadsTheComposerBackAfterSubmitting(t *testing.T) {
	home, herdr := promptSubmitHerdr(t, "❯\\u00a0close 1gak4 and then close jwcxu\\n")
	out := runPrompt(t, home, herdr)
	if !strings.Contains(out, "typed but not submitted") {
		t.Errorf("a prompt that never left the composer reported as sent:\n%s", out)
	}
	if !strings.Contains(out, "close 1gak4 and then close jwcxu") {
		t.Errorf("the warning does not show what is in the box:\n%s", out)
	}
	// The other warning, before anything was typed: this prompt is about to
	// go in AFTER text somebody else's prompt left there.
	if !strings.Contains(out, "this text is typed after it") {
		t.Errorf("nothing warned that the box already held an unsent prompt:\n%s", out)
	}
	// And it is a warning, not a refusal: the prompt went to herdr.
	if !strings.Contains(out, "agent_prompted") {
		t.Errorf("the prompt was not sent — the measured recovery for this state is a hand prompt:\n%s", out)
	}
}

// The control: an empty box is silent. Without it "no warning was printed"
// is also what a build that never reads the box prints.
func TestQAPromptIntoAnEmptyBoxSaysNothing(t *testing.T) {
	home, herdr := promptSubmitHerdr(t, "❯\\n")
	out := runPrompt(t, home, herdr)
	if strings.Contains(out, "not submitted") || strings.Contains(out, "typed after it") {
		t.Errorf("an empty composer raised a warning:\n%s", out)
	}
	if !strings.Contains(out, "agent_prompted") {
		t.Errorf("the prompt did not reach herdr:\n%s", out)
	}
}
