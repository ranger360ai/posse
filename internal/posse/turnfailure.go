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

// TurnOutcome is what a runtime's own store says one settled turn did: the
// message a refusal carried (empty = the turn was answered normally), and how
// much of that turn had already RUN when the refusal landed.
//
// The second half is why this is a struct and not a string
// (ranger-base-qcu4c). An account does not only refuse before it serves: one
// of the seven refusals in this box's grok history landed 190,817 tokens and
// six model calls into a turn that had been running for a minute and a half,
// and a session that far in may well have edited files and commented on the
// bead. "no work ran" is exactly wrong about that turn, and the runtime's own
// record is the only thing that can tell the two apart.
type TurnOutcome struct {
	// Message is the refusal the runtime recorded, folded to one line.
	Message string
	// ModelCalls and OutputTokens are what the runtime's record says this
	// turn spent BEFORE the refusal — the units an operator decides in, not
	// the store's whole usage object, because the question the settle line
	// asks is only "is there work on the other side of this".
	//
	// They are read only beside a Message, and stay zero on an outcome that
	// carries none: nothing asks what a turn that was never refused spent,
	// and what it spent is already `posse cost`'s to read off the same
	// record. A reader filling them in on a healthy turn would be answering
	// a question this type is not the one to answer.
	ModelCalls   int
	OutputTokens int
}

// Worked reports that this turn had already run when it was refused, so a
// relaunch lands on top of work that may exist — a worktree with edits, a
// bead with comments.
//
// False carries a claim, so a reader owes it one: BOTH registered readers can
// make it, for different reasons — claude by construction (see
// FindClaudeTurnOutcome), grok off the `usage` object its store writes on
// every turn that ran anything (186/186, censused 2026-09-05). A third
// adapter that cannot tell must say so in its own docstring, because the
// settle line reads a zero here as "nothing ran".
func (o TurnOutcome) Worked() bool { return o.ModelCalls > 0 || o.OutputTokens > 0 }

// TurnOutcomeReader reads the runtime-owned outcome of one settled turn: what
// the turn did (TurnOutcome) and whether an outcome was READ AT ALL. The two
// are separate because "nothing to report" and "nothing readable" are
// different facts, and only the first one may clear a failure marker.
type TurnOutcomeReader func(dir, bead string, since time.Time) (out TurnOutcome, observed bool)

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
// false positive. observed distinguishes a healthy first answer (no message,
// true) from a transcript that is not readable yet (no message, false).
//
// The outcome's work fields stay zero, and here that is a MEASURED zero and
// not an unreported one: this reader reports a refusal only when the FIRST
// assistant record after the bead prompt is the synthetic one, so no tool
// call, no edit and no bead comment can have happened ahead of it — claude's
// arm of the settle line keeps the flat "no work ran" and is right to
// (ranger-base-qcu4c). What claude cannot see is the mirror of that: a
// refusal landing AFTER a first answer reads here as an outcome with no
// message and observed=true — a healthy turn — ranger-base-4ldma.
func FindClaudeTurnOutcome(dir, bead string, since time.Time) (out TurnOutcome, observed bool) {
	project := strings.ReplaceAll(filepath.ToSlash(filepath.Clean(dir)), "/", "-")
	for _, path := range TranscriptFiles(project) {
		st, err := os.Stat(path)
		if err != nil || st.ModTime().Before(since.Add(-time.Second)) {
			continue
		}
		if out, observed := scanClaudeTurnOutcome(path, bead, since); observed {
			return out, observed
		}
	}
	return TurnOutcome{}, false
}

func scanClaudeTurnOutcome(path, bead string, since time.Time) (out TurnOutcome, observed bool) {
	f, err := os.Open(path)
	if err != nil {
		return TurnOutcome{}, false
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
							// Work fields left zero on purpose: this record IS
							// the first answer, so there is nothing before it.
							return TurnOutcome{Message: strings.Join(strings.Fields(text), " ")}, true
						}
						// The first assistant answer was not an allotment refusal:
						// this is positive evidence that a prior failure marker can
						// be cleared. Anything later belongs to work that started.
						return TurnOutcome{}, true
					}
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return TurnOutcome{}, false
		}
	}
	return TurnOutcome{}, false
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
