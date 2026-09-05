package posse

// A runtime process being alive is not proof that a turn ran. Claude writes
// subscription/allotment refusals as synthetic assistant messages: herdr
// correctly sees the CLI settle back to idle, but no model ever handled the
// prompt. The transcript is the runtime-owned fact that distinguishes that
// terminal outcome from an ordinary settled turn.

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TurnOutcomeReader reads the runtime-owned outcome of one settled turn:
// the message a refusal carried (empty = the turn was answered normally)
// and whether an outcome was READ AT ALL. The two are separate because
// "nothing to report" and "nothing readable" are different facts, and only
// the first one may clear a failure marker.
type TurnOutcomeReader func(dir, bead string, since time.Time) (message string, observed bool)

// TurnOutcomeClaudeTranscript reads claude's own JSONL transcript under
// ~/.claude/projects. A runtime whose CLI writes that same shape declares
// `turn_outcome: claude-transcript` and is read by it; anything else needs a
// reader here first (ADR 0012 D4's adapter seam). grok's store is the second
// one built — TurnOutcomeGrokSessionStore, turnfailure_grok.go.
const TurnOutcomeClaudeTranscript = "claude-transcript"

// turnOutcomeReaders maps a runtime's declared turn_outcome: adapter to the
// code that implements it. The map is the whole registry: a name that is
// not a key here is refused at load, so a declaration can never promise a
// reading nothing performs.
//
// grok's store is read since ranger-base-fc8go: its refusal artifact was
// captured first and pinned as a fixture, which is the order ADR 0013 §1's
// promotion rule sets (docs/adr/0013-turn-outcome-refusal-probe.md).
//
// codex writes ~/.codex/sessions/*.jsonl (MEASURED, ranger-base-xaev) and is
// reachable in principle, but its refusal artifact has NOT been captured —
// its account was alive when the probe ran and the probe was told not to
// spend to force one — so it has no reader yet, and a reader built over a
// guessed shape is exactly what the promotion rule refuses. That is a
// declared blindness on codex, printed on the settle line, not a silence.
var turnOutcomeReaders = map[string]TurnOutcomeReader{
	TurnOutcomeClaudeTranscript: FindClaudeTurnOutcome,
	TurnOutcomeGrokSessionStore: FindGrokTurnOutcome,
}

// TurnOutcomeAdapters is every registered adapter name, sorted — what a
// refused declaration lists so the operator can see what is on offer.
func TurnOutcomeAdapters() []string {
	names := make([]string, 0, len(turnOutcomeReaders))
	for name := range turnOutcomeReaders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TurnOutcomeReaderFor is the reader a runtime's declaration resolves to,
// or nil when it declares none. nil is the honest answer for codex today:
// posse cannot tell an exhausted account from an ordinary settle there, and
// saying so is this seam's whole point (ranger-base-02zr).
func TurnOutcomeReaderFor(rt *Runtime) TurnOutcomeReader {
	if rt == nil {
		return nil
	}
	return turnOutcomeReaders[rt.TurnOutcomeAdapter]
}

// FindClaudeTurnOutcome finds the first assistant outcome for this dispatch prompt.
// It scans only the Claude project directory for dir, only files touched by
// this turn, and only an assistant record after the matching bead prompt.
// User-authored bead text can quote the provider message without becoming a
// false positive. observed distinguishes a healthy first answer ("", true)
// from a transcript that is not readable yet ("", false).
func FindClaudeTurnOutcome(dir, bead string, since time.Time) (message string, observed bool) {
	project := strings.ReplaceAll(filepath.ToSlash(filepath.Clean(dir)), "/", "-")
	for _, path := range TranscriptFiles(project) {
		st, err := os.Stat(path)
		if err != nil || st.ModTime().Before(since.Add(-time.Second)) {
			continue
		}
		if message, observed := scanClaudeTurnOutcome(path, bead, since); observed {
			return message, observed
		}
	}
	return "", false
}

func scanClaudeTurnOutcome(path, bead string, since time.Time) (message string, observed bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()

	inTurn := false
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, readErr := r.ReadBytes('\n')
		if len(line) > 0 {
			var d struct {
				Type      string `json:"type"`
				Timestamp string `json:"timestamp"`
				Message   struct {
					Model   string          `json:"model"`
					Content json.RawMessage `json:"content"`
				} `json:"message"`
			}
			if json.Unmarshal(line, &d) == nil {
				ts, _ := time.Parse(time.RFC3339Nano, d.Timestamp)
				switch d.Type {
				case "user":
					m := workPromptRe.FindStringSubmatch(userText(d.Message.Content))
					inTurn = m != nil && m[1] == bead && !ts.Before(since)
				case "assistant":
					if inTurn && !ts.Before(since) {
						text := assistantText(d.Message.Content)
						if d.Message.Model == "<synthetic>" && claudeAllotmentLimit(text) {
							return strings.Join(strings.Fields(text), " "), true
						}
						// The first assistant answer was not an allotment refusal:
						// this is positive evidence that a prior failure marker can
						// be cleared. Anything later belongs to work that started.
						return "", true
					}
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", false
		}
	}
	return "", false
}

func assistantText(raw json.RawMessage) string {
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var out []string
	for _, p := range parts {
		if p.Type == "text" && p.Text != "" {
			out = append(out, p.Text)
		}
	}
	return strings.Join(out, "\n")
}

func claudeAllotmentLimit(text string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, "you've reached your ") &&
		strings.Contains(text, " limit") &&
		(strings.Contains(text, "/usage-credits") || strings.Contains(text, "switch models with /model"))
}
