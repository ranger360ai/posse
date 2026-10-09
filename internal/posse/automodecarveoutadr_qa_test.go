package posse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adrAutoModeCarveOutPath is the record that authorizes the carve-out's text.
const adrAutoModeCarveOutPath = "docs/adr/0070-allow-is-friction-under-auto-mode.md"

// The carve-out against the record it quotes (ranger-base-c768n finding 2,
// escaped from ranger-base-x2vt7).
//
// ClaudeAutoModeCarveOut is documented at its own declaration as "quoted
// verbatim from ADR 0070 D2", and until this pin nothing held that. It IS
// verbatim: MEASURED 2026-10-09, the D2 blockquote with `> ` stripped and the
// lines joined on a single space is byte-identical to the const — 862 bytes
// and 856 runes on both sides, the six-byte gap being the em dashes and the
// ellipsis, which is why the bead that filed this said 856 and this pin logs
// 862.
//
// MEASURED survivor, same date:
//
//	perl -0777 -pi -e 's/SIBLING terminal panes/sibling terminal panes/' internal/posse/agents.go
//
// then the five automodeallow pins plus
// TestQAClaudeAutoModeCarveOutMeetsTheRulesBar -> ok. That pin holds six
// phrases and each granted verb's last word, which is the RULES' bar and the
// right thing for it to hold; what no reading reached was the drift from the
// record. This is the one const in agents.go whose whole value is that a
// model reads exactly the text the ADR cleared — the statement is prose read
// by a classifier (D3), so the only thing that makes it posse's standing
// statement rather than a seat's paraphrase is that it is the cleared bytes.
//
// internal/treepins/adrtestcitation_qa_test.go is the house precedent for
// reading an ADR from a test; this one lives in internal/posse because the
// const is unexported there.
func TestQAClaudeAutoModeCarveOutIsADR0070D2Verbatim(t *testing.T) {
	t.Parallel()
	// qibRepoRoot, not a hand-rolled climb: the tree-wide door census in
	// internal/treepins derives this pin's class from that one helper
	// (ranger-base-sx2dq), and a pin outside the class gets no door.
	root := qibRepoRoot(t)

	b, err := os.ReadFile(filepath.Join(root, adrAutoModeCarveOutPath))
	if err != nil {
		t.Fatalf("%s: %v — that record is what authorizes the carve-out's text, and a pin that cannot read it holds nothing", adrAutoModeCarveOutPath, err)
	}

	quoted, line, err := adrD2Blockquote(string(b))
	if err != nil {
		t.Fatalf("%s: %v", adrAutoModeCarveOutPath, err)
	}
	t.Logf("%s:%d — D2's blockquote is %d bytes flowed (%d runes)", adrAutoModeCarveOutPath, line, len(quoted), len([]rune(quoted)))

	if quoted != ClaudeAutoModeCarveOut {
		t.Errorf("ClaudeAutoModeCarveOut is not ADR 0070 D2's blockquote verbatim (%s:%d).\n"+
			"  const (%d bytes): %q\n"+
			"  D2    (%d bytes): %q\n"+
			"  the declaration says the text is quoted verbatim from that record, and the whole value of this const is that a classifier reads exactly the text the ADR cleared — a paraphrase is a statement nobody cleared. Move BOTH, or neither (ranger-base-c768n finding 2).",
			adrAutoModeCarveOutPath, line, len(ClaudeAutoModeCarveOut), ClaudeAutoModeCarveOut, len(quoted), quoted)
	}
}

// adrD2Blockquote is the first blockquote inside ADR 0070's D2 decision,
// flowed: `> ` stripped and the lines joined on a single space, which is the
// one transformation between a wrapped markdown quote and a Go string
// constant.
//
// Bounded by the NEXT decision heading on purpose. An unbounded scan for the
// first `> ` after D2 would silently read D3's or D4's quote if D2 ever lost
// its own, and compare the const against the wrong paragraph — a pin that is
// green for a reason that has nothing to do with its subject.
func adrD2Blockquote(doc string) (flowed string, atLine int, err error) {
	lines := strings.Split(doc, "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "**D2 —") || strings.HasPrefix(ln, "**D2 -") {
			start = i
			break
		}
	}
	if start < 0 {
		return "", 0, fmt.Errorf("no `**D2 —` decision heading — the carve-out's record has been renumbered or rewritten, so this pin no longer knows which paragraph it quotes")
	}

	var quote []string
	for i := start + 1; i < len(lines); i++ {
		ln := lines[i]
		if quote == nil && strings.HasPrefix(ln, "**D") {
			return "", 0, fmt.Errorf("D2 (line %d) carries no `> ` blockquote before the next decision heading (line %d) — the cleared text is what this pin compares, and a decision that states it some other way needs this reader changed with it", start+1, i+1)
		}
		switch {
		case strings.HasPrefix(ln, ">"):
			if quote == nil {
				atLine = i + 1
			}
			quote = append(quote, strings.TrimSpace(strings.TrimPrefix(ln, ">")))
		case quote != nil:
			// The blockquote ended: one contiguous run, never two joined
			// across the prose between them.
			return strings.Join(quote, " "), atLine, nil
		}
	}
	if quote == nil {
		return "", 0, fmt.Errorf("D2 (line %d) carries no `> ` blockquote at all", start+1)
	}
	return strings.Join(quote, " "), atLine, nil
}
