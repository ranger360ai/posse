package rhq

// QA pin for rangerhq-cmfj / rangerhq-gmnm (verified under rangerhq-o0el).
//
// §9 tells a cold installer that the work prompt states the push precedence in
// its own voice, and quotes the line — because the file-side fix cannot reach
// `bd prime`'s copy of the mandate, so the prompt is the only wall left. A
// doc that quotes a constant drifts from it silently: workprompt_test.go pins
// the constant, nothing pinned the quote. This is that half.
//
// The rest of the pins for cmfj (§9's cut-and-append recipe, run rather than
// read, and this repo's own AGENTS.md) live in ../../pushmandate_qa_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSection9QuotesThePushPrecedenceLineVerbatim(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "INSTALL.md"))
	if err != nil {
		t.Fatalf("read INSTALL.md: %v", err)
	}
	doc := string(b)
	i := strings.Index(doc, "## 9. The work repo and its queue")
	j := strings.Index(doc, "## 10. ")
	if i < 0 || j < 0 || j < i {
		t.Fatal("INSTALL.md: §9 not found — the pin has stopped reading its subject")
	}

	// The quote is an indented block: four spaces, starting at `guardrails:`,
	// ending at the first line that is not indented.
	var quoted []string
	for _, line := range strings.Split(doc[i:j], "\n") {
		if len(quoted) == 0 {
			if strings.HasPrefix(line, "    guardrails: ") {
				quoted = append(quoted, strings.TrimSpace(line))
			}
			continue
		}
		if !strings.HasPrefix(line, "    ") {
			break
		}
		quoted = append(quoted, strings.TrimSpace(line))
	}
	if len(quoted) == 0 {
		t.Fatal("INSTALL.md §9 no longer quotes the work prompt's `guardrails:` line (rangerhq-gmnm)")
	}

	if got := strings.Join(quoted, " "); got != pushPrecedence {
		t.Errorf("INSTALL.md §9's quote has drifted from pushPrecedence:\n doc %q\nsrc %q", got, pushPrecedence)
	}
}

// TestPushPrecedenceNamesNoSourceAsItsBoundary is the reason the wording
// changed at all: the earlier rider said guardrails outrank push instructions
// "in repo docs", which by its own terms did not cover `bd prime`'s
// session-start checklist — and a persona pushed into the gate under it.
func TestPushPrecedenceNamesNoSourceAsItsBoundary(t *testing.T) {
	if strings.Contains(pushPrecedence, "in repo docs") {
		t.Errorf("pushPrecedence bounded itself to repo docs again — bd prime's checklist is not one (rangerhq-gmnm): %q", pushPrecedence)
	}
	for _, want := range []string{"bd prime", "tool output", "this prompt", "git push"} {
		if !strings.Contains(pushPrecedence, want) {
			t.Errorf("pushPrecedence no longer names %q as covered (rangerhq-gmnm): %q", want, pushPrecedence)
		}
	}
}
