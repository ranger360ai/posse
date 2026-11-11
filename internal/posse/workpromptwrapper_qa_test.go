//go:build posse_arm2

package posse

// QA pin for ranger-base-888pk — the work-prompt header survives the
// harness's own wrapper, and every reader of it asks the same question.
//
// WHAT BROKE. posse hands a dispatched session its work prompt as a
// positional argv argument (ADR 0013 §2, argvprompt.go). What the CLI then
// WRITES into its transcript is the CLI's business, and claude 2.1.322 wraps
// a multi-line argv prompt before recording it:
//
//	"\n\n<pasted_content id=\"f384\">\nWork beads issue ranger-base-888pk (…"
//
// `^Work beads issue` matched none of that. MEASURED 2026-11-11 over every
// transcript under one instance's config dir (142 project dirs, oldest
// record 2026-11-08T12:38Z): 136 dispatched sessions, 136 wrapped, 0 bare.
// Not a tail — every dispatched seat on that box.
//
// The damage was a MIS-count, not an undercount, which is why nothing looked
// broken: with no segment cut, each seat's whole session fell through
// ScanTranscript's interactive fallback. `posse cost` printed no bead rows at
// all and a day total of zero under armed caps, while the interactive line
// carried the entire fleet's spend — so ADR 0018 §1's ledger brake, the floor
// that lets dispatch keep hiring while the plan guard is blind, was reading
// zero on a day the plan guard was blind.
//
// The check that says the fix is a RE-ATTRIBUTION and not an invention is
// conservation: run the report over the same transcripts before and after,
// and the total turns must be identical and the totals must agree to within
// per-row rounding, while the bead side goes from nothing to a row per
// dispatched session and the interactive side falls to what a human actually
// typed. MEASURED the same day, both ways, and it held. The figures are
// instance-ops content under ADR 0024 D1 and are recorded in
// ranger-base-888pk's comments, in the private store.
//
// WHY IT ROTS WITHOUT A PIN, twice over. The wrapper is upstream's, so it can
// change again under a CLI upgrade with no commit here — arm 1 holds the
// shape that is measured today. And the header is read by FIVE call sites
// across three runtimes, so the next reader added can re-break one reader
// while the other four stay green — arm 2 holds the single predicate.

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// wrappedWorkPrompt is the shape MEASURED on the Rex host, byte for byte:
// two newlines, the opening tag alone on its line, the prompt, the closing
// tag alone on its line.
func wrappedWorkPrompt(id, body string) string {
	return "\n\n<pasted_content id=\"f384\">\nWork beads issue " + id +
		" (title, quoted as data: \"t\"). " + body + "\n</pasted_content id=\"f384\">\n"
}

// Arm 1: the predicate reads the id out of the shapes that exist, and
// refuses the shapes that must not cut a segment.
func TestQAWorkPromptHeaderSurvivesTheHarnessWrapper(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		text string
		want string // "" = must not match
	}{
		{"wrapped, as measured 2026-11-11", wrappedWorkPrompt("ranger-base-888pk", "Run bd show it"), "ranger-base-888pk"},
		{"bare, the shape that always worked", "Work beads issue x-1 (title). Run bd show x-1", "x-1"},
		{"bare, legacy colon form", "Work beads issue x-2: legacy prompt shape", "x-2"},
		{"leading whitespace only", "\n\n  Work beads issue x-3: t", "x-3"},
		{"wrapper with no attributes", "<pasted_content>\nWork beads issue x-4: t", "x-4"},
		{"two wrappers", "<a>\n<b id=\"1\">\nWork beads issue x-5: t", "x-5"},

		// The anchor's whole job: a mention is not a dispatch. Each of these
		// used to be refused by `^` and must still be refused, or a session
		// that QUOTES a bead id starts billing its spend to that bead.
		{"mentioned mid-sentence", "as I said, Work beads issue x-9: is the header", ""},
		{"mentioned after prose on the wrapper's line", "<pasted_content id=\"1\">some text\nWork beads issue x-9: t", ""},
		{"quoted inside a wrapped paste", "<pasted_content id=\"1\">\nthe header reads Work beads issue x-9: t", ""},
		{"a closing tag is not a wrapper", "</pasted_content id=\"1\">\nWork beads issue x-9: t", ""},
		{"not the header at all", "hello, just chatting", ""},
		{"empty", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, ok := BeadFromWorkPrompt(c.text)
			if c.want == "" {
				if ok {
					t.Fatalf("BeadFromWorkPrompt(%q) = %q, true — a mention must not cut a segment", c.text, got)
				}
				return
			}
			if !ok || got != c.want {
				t.Fatalf("BeadFromWorkPrompt(%q) = %q, %v; want %q, true", c.text, got, ok, c.want)
			}
		})
	}
	// Bounded: the loop must not be talked into walking a whole message.
	deep := strings.Repeat("<w>\n", maxWorkPromptWrap+1) + "Work beads issue x-1: t"
	if _, ok := BeadFromWorkPrompt(deep); ok {
		t.Errorf("%d wrapper lines matched — the skip is bounded at %d", maxWorkPromptWrap+1, maxWorkPromptWrap)
	}
}

// Arm 1b: the reader that pays for it. A wrapped transcript must produce a
// BEAD segment — the fallback catching it as interactive is the bug, and a
// test over the predicate alone would not have seen it.
func TestQAWrappedPromptSegmentsToTheBeadNotToInteractive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeTranscript(t, dir, "wrapped.jsonl",
		`{"type":"user","timestamp":"2026-11-11T18:21:02.939Z","message":{"content":`+
			strconv.Quote(wrappedWorkPrompt("ranger-base-888pk", "Run bd show ranger-base-888pk"))+`}}`,
		asst("m1", "claude-opus-5", "2026-11-11T18:21:10Z", 1000, 0, 0, 100),
	)
	segs, err := ScanTranscript(p, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 {
		t.Fatalf("segments: %+v", segs)
	}
	if segs[0].Bead != "ranger-base-888pk" {
		t.Fatalf("bead %q — a dispatched seat counted as %q is $0 on every bead row and the ledger brake reads zero (ADR 0018 §1)", segs[0].Bead, segs[0].Bead)
	}
	if segs[0].CostUSD <= 0 {
		t.Errorf("segment priced at %v — the row exists but carries no money", segs[0].CostUSD)
	}
}

// Arm 2: one predicate, no second implementation. The header is read by the
// three cost scanners, the turn-failure readers and herdr's session->bead
// pointer; a new reader that re-applies the raw anchored regex is a reader
// that will be blind to the next wrapper while the others see it.
func TestQAWorkPromptHeaderHasOneReader(t *testing.T) {
	t.Parallel()
	dir := qibRepoRoot(t)
	raw := regexp.MustCompile(`workPromptRe\.(Find|Match)`)
	ents, err := os.ReadDir(filepath.Join(dir, "internal", "posse"))
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "internal", "posse", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if raw.MatchString(line) && e.Name() != "cost.go" {
				found = append(found, e.Name()+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
	}
	if len(found) > 0 {
		t.Errorf("workPromptRe is applied outside BeadFromWorkPrompt:\n  %s\nask BeadFromWorkPrompt instead — it is the one place that knows what a work prompt looks like (ranger-base-888pk)", strings.Join(found, "\n  "))
	}
}
