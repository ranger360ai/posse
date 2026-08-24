package rhq

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
	"strings"
	"time"
)

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
