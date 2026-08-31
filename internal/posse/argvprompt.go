package posse

// Argv prompt delivery — ADR 0013 §2, the promptable stage.
//
// A `prompt: typed` runtime is dispatched the claude-shaped way: create the
// session, wait until herdr can SEE a screen that takes input, claim, then
// type the work prompt into it. Every clause of that is an assumption, and
// two of the three built-in runtimes broke one in an evening: grok and
// codex both open on a first-run interstitial, and grok's pane matches no
// herdr rule at all until it has had a turn — so there is no number of
// seconds that turns "waiting" into "promptable" there (ranger-base-3j8).
//
// The sidestep is that all three CLIs take the prompt as a positional
// argument that starts an INTERACTIVE session (`claude [prompt]`,
// `codex [PROMPT]`, `grok [PROMPT]`; the headless `-p` / `exec` cousins are
// a different product and not this path). The work prompt rides in on the
// launch line, so the screen stops being the delivery channel: whatever the
// CLI draws first, the prompt is already the first user turn behind it.
//
// The prompt itself does not go on the line — it is a page of text with
// quotes, newlines and a bead's own title in it, and a pane's shell is a
// tty with a line limit (rangerhq-ybec). It goes in a file, and the line
// carries `"$(cat <file>)"`: 12 characters plus a path, expanded by the
// shell that runs the CLI. No new template placeholder — an unrendered
// `{prompt}` would be a literal argv (ADR 0001/0002's lesson, and ADR 0013
// rejects it by name), so this is appended to the already-rendered command.

import (
	"os"
	"path/filepath"
	"strings"
)

// WorkPromptDir is where a dispatched session's work prompt is written:
// one file per session name, which under Dial F means one per bead. It
// lives under $RHQ_HOME/state/ — posse's own dir, not the repo's and not
// the persona's memory, which the persona writes to.
func (a *App) WorkPromptDir() string { return filepath.Join(a.StateDir, "prompts") }

// WorkPromptFile is the path this session's argv prompt is written to.
func (a *App) WorkPromptFile(session string) string {
	return filepath.Join(a.WorkPromptDir(), session+".txt")
}

// WriteWorkPrompt puts the assembled work prompt where the launch line's
// `$(cat …)` will read it, and returns the path.
//
// The file is left behind on purpose. It is the only record of what a
// dispatched session was actually asked — a typed prompt at least echoes
// into the pane's scrollback, and an argv one is consumed by the exec
// before any screen exists. It is rewritten on the next dispatch of the
// same bead, so the directory grows by one small file per bead, not per
// pass.
func (a *App) WriteWorkPrompt(session, prompt string) (string, error) {
	// A positional prompt that starts with `-` is read as a flag, not as
	// text: codex died on exactly that when a PID's `---` frontmatter was
	// passed as its prompt (rangerhq-5oi). The work prompt is assembled by
	// posse and starts with "Work beads issue …", so this cannot fire
	// today — which is the point of asserting it here rather than
	// discovering it the next time the skeleton is edited.
	if strings.HasPrefix(prompt, "-") {
		return "", Die("work prompt starts with %q — a positional prompt that starts with a dash is parsed as a flag, not as text (ADR 0013 §2)", strings.SplitN(prompt, "\n", 2)[0])
	}
	if err := os.MkdirAll(a.WorkPromptDir(), 0o700); err != nil {
		return "", err
	}
	p := a.WorkPromptFile(session)
	if err := os.WriteFile(p, []byte(prompt), 0o600); err != nil {
		return "", err
	}
	return p, nil
}

// ArgvPromptSuffix is what gets appended to a rendered launch line so the
// CLI receives the file's contents as its first user turn. Double-quoted
// around a single-quoted path: the shell expands the substitution, and the
// whole page — newlines, quotes and all — arrives as ONE argument.
func ArgvPromptSuffix(file string) string { return ` "$(cat ` + shellQuote(file) + `)"` }
