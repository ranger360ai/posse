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
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// promptSubmitFake is what the arms below vary: the composer region
// `agent explain` previews, and the two verbs the retirements behind
// PaneHolding ask NEXT (ranger-base-um1bi). Empty means the verb is
// unanswered, which is this fake's own shape for "herdr would not say" — and
// is what the two original arms run with, so the reading they pin is
// unchanged.
type promptSubmitFake struct {
	// box is the `prompt_box_body` preview, inside a JSON document, so a
	// U+00A0 is spelled `\u00a0`.
	box string
	// ansi is `agent read --format ansi`, raw bytes, for the dim-run reading
	// that tells claude's own suggestion from typed text (ghostbox.go).
	ansi string
	// session is `pane get`'s agent_session value — the runtime's own id for
	// the conversation, which is the join the submit log is read through
	// (sentline.go).
	session string
}

// promptSubmitHerdr writes a fake herdr whose `agent explain` reports an
// idle pane whose composer holds box, and returns the home it was set up
// in. The shape of the explain answer is a real capture's (herdr 0.8.2):
// state, the matched rule, and evaluated_rules carrying each rule's screen
// region and a preview of it.
func promptSubmitHerdr(t *testing.T, box string) (home, bin string) {
	t.Helper()
	return promptSubmitHerdrWith(t, promptSubmitFake{box: box})
}

func promptSubmitHerdrWith(t *testing.T, f promptSubmitFake) (home, bin string) {
	t.Helper()
	home = t.TempDir()
	binDir := t.TempDir()
	herdr := filepath.Join(binDir, "herdr")
	box := f.box
	extra := ""
	if f.ansi != "" {
		extra += `"agent read")
  printf '%s' '` + f.ansi + `'
  exit 0;;
`
	}
	if f.session != "" {
		extra += `"pane get")
  printf '%s\n' '{"pane":{"agent":"claude","agent_session":{"kind":"id","value":"` + f.session + `"}}}'
  exit 0;;
`
	}
	script := `#!/bin/sh
case "$1 $2" in
` + extra + `
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
	if err := posse.WriteExecutable(herdr, []byte(script), 0o755); err != nil {
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

// promptText is the text every arm sends, named because the echo arm needs
// claude's submit log to carry exactly it.
const promptText = "commit and close the bead once the suite is green"

func runPrompt(t *testing.T, home, herdr string) string {
	t.Helper()
	return runPromptEnv(t, home, herdr)
}

func runPromptEnv(t *testing.T, home, herdr string, extra ...string) string {
	t.Helper()
	cmd := exec.Command(buildRhq(t), "prompt", "fresh", promptText)
	cmd.Env = append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+herdr,
		"HERDR_SOCKET_PATH="+filepath.Join(home, "no-such.sock"))
	cmd.Env = append(cmd.Env, extra...)
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

// ─── the two retirements the warning never had (ranger-base-um1bi) ───────────
//
// Both warnings above read the composer, and the pre-type one read it off the
// readiness gate's own detection — herdr's region preview, and nothing else.
// That is wrong twice. On a `multiplexer: rex` instance `agent explain` is ADR
// 0076 §4, the gate concedes and hands back the ZERO detection, so the
// warning was silent on every seat for the life of the instance; and on herdr
// it fired over two boxes that nobody is holding text in. The reading that
// knows the difference is PaneHolding (panework.go), which routes per occupant
// and carries both retirements, so the call site asks it instead.

// suggestText is claude's own next-prompt suggestion, and suggestBox is the
// region preview of it — identical, character for character, to a prompt
// somebody typed. What tells them apart is that claude draws its own FAINT,
// which herdr's ANSI-stripped preview flattens away (ranger-base-6o7wm).
const (
	suggestText = "keep waiting, then commit and close"
	suggestBox  = "❯\\u00a0" + suggestText + "\\n"
	suggestANSI = "❯ \x1b[0m\x1b[2m" + suggestText + "\x1b[0m\n"
)

// A box claude DREW is not a prompt anybody is holding, so neither warning
// fires — and the prompt still goes out. Before this bead the pre-type line
// fired on every settled seat whose box carried a suggestion, which is most
// of them, and a warning that is usually wrong is a warning the operator
// learns to skip past.
func TestQAPromptDoesNotWarnAboutClaudesOwnSuggestion(t *testing.T) {
	home, herdr := promptSubmitHerdrWith(t, promptSubmitFake{box: suggestBox, ansi: suggestANSI})
	out := runPrompt(t, home, herdr)
	if strings.Contains(out, "this text is typed after it") {
		t.Errorf("the pre-type warning called claude's own suggestion a prompt sitting unsent:\n%s", out)
	}
	if strings.Contains(out, "typed but not submitted") {
		t.Errorf("the read-back called claude's own suggestion an unsubmitted prompt:\n%s", out)
	}
	if !strings.Contains(out, "agent_prompted") {
		t.Errorf("the prompt did not reach herdr:\n%s", out)
	}
}

// THE WRONG ARM, and the reason the one above is a discriminator rather than
// a third way of printing nothing: the same characters in the box with NO dim
// run are a prompt that really is sitting there, and both warnings fire
// exactly as ranger-base-htafy measured them.
func TestQAPromptStillWarnsWhenTheSameBoxIsNotDrawnFaint(t *testing.T) {
	home, herdr := promptSubmitHerdrWith(t, promptSubmitFake{
		box: suggestBox, ansi: "prompt$ claude\n\n❯ " + suggestText + "\n"})
	out := runPrompt(t, home, herdr)
	if !strings.Contains(out, "this text is typed after it") || !strings.Contains(out, suggestText) {
		t.Errorf("a prompt sitting unsent in the box raised no pre-type warning:\n%s", out)
	}
}

// The other retirement: a box previewing the line this pane last SUBMITTED is
// echoing it, which claude's own submit log says (ranger-base-2hvtv). The
// pre-type warning is silent — there is nothing for this text to be typed
// after.
//
// The POST-submit read-back still speaks here, and that is not this bead's to
// change: ConfirmSubmitted's herdr arm has never had the echo check, it is
// asked straight after posse submitted this very line, and the box
// legitimately holds it for a moment. The capture route does run it, because
// there the echo join is inside the one reading (panework.go,
// confirmSubmittedCaptured).
func TestQAPromptDoesNotWarnAboutABoxEchoingTheLastSubmit(t *testing.T) {
	const session = "f4987ef2-86df-48c7-8946-86a5c03e31f9"
	home, herdr := promptSubmitHerdrWith(t, promptSubmitFake{
		box:     "❯\\u00a0" + promptText + "\\n",
		session: session,
	})
	configDir := t.TempDir()
	row := map[string]any{
		"display":        promptText,
		"pastedContents": map[string]any{},
		"project":        "/tmp/proj",
		"sessionId":      session,
		"timestamp":      1788000000000,
	}
	line, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "history.jsonl"), append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	out := runPromptEnv(t, home, herdr, "CLAUDE_CONFIG_DIR="+configDir)
	if strings.Contains(out, "this text is typed after it") {
		t.Errorf("the pre-type warning called a box echoing the line this pane last submitted a prompt sitting unsent:\n%s", out)
	}
	if !strings.Contains(out, "agent_prompted") {
		t.Errorf("the prompt did not reach herdr:\n%s", out)
	}
}

// THE WRONG ARM for the echo: the same box and the same store, with the log's
// last row for this session naming something else — so nothing was echoed and
// the warning stands.
func TestQAPromptStillWarnsWhenTheStoreDoesNotKnowTheBox(t *testing.T) {
	const session = "f4987ef2-86df-48c7-8946-86a5c03e31f9"
	home, herdr := promptSubmitHerdrWith(t, promptSubmitFake{
		box:     "❯\\u00a0" + promptText + "\\n",
		session: session,
	})
	configDir := t.TempDir()
	line, err := json.Marshal(map[string]any{
		"display": "ok goign to bed", "project": "/tmp/proj", "sessionId": session,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "history.jsonl"), append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	out := runPromptEnv(t, home, herdr, "CLAUDE_CONFIG_DIR="+configDir)
	if !strings.Contains(out, "this text is typed after it") {
		t.Errorf("a prompt nobody has sent raised no pre-type warning:\n%s", out)
	}
}
