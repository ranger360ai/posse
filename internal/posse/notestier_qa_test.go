//go:build posse_arm2

package posse

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// NOTES.md states the built-in tier table and quotes the preflight's loud
// line, and until ranger-base-1kvfr nothing coupled either sentence to the
// code: 9e51e72 flipped claudeModels[TierStrong] and the identical sentences
// in ADR 0003, and NOTES.md went on naming claude-fable-5 in both places for
// three days — one of them a quotation of program output the program does not
// produce. Every _test.go reader of NOTES.md was for some other paragraph.
//
// Both pins below are TWO-WAY on purpose. "the new id appears somewhere" goes
// green the moment any other sentence mentions it, which is exactly the state
// the defect was found in: the tier paragraph held the old id while the new
// one appeared nowhere in it.

func notesText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "NOTES.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// notesFlat collapses NOTES.md's hard wrapping and drops the backticks it
// wears around ids, so a phrase can be matched as it READS rather than as it
// happens to be broken across lines — the sentence this file pins is split
// mid-quotation by the wrap, and a per-line scan would never see it.
func notesFlat(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "`", "")), " ")
}

// notesParagraph returns the paragraph opening with marker, flattened.
func notesParagraph(t *testing.T, notes, marker string) string {
	t.Helper()
	i := strings.Index(notes, marker)
	if i < 0 {
		t.Fatalf("NOTES.md no longer carries the paragraph opening %q — if it was reworded, re-aim this pin (ranger-base-1kvfr)", marker)
	}
	rest := notes[i:]
	if j := strings.Index(rest, "\n\n"); j >= 0 {
		rest = rest[:j]
	}
	return notesFlat(rest)
}

// claudeModelID matches a claude model id the way the ids are actually
// spelled. Greedy on the trailing segments, so a stale claude-fable-5 next to
// the current claude-fable-5-1 is a token of its own and not a prefix hiding
// inside it.
var claudeModelID = regexp.MustCompile(`claude-[a-z0-9]+(?:[-.][a-z0-9]+)*`)

// codexModelID and grokModelID are claudeModelID's twins for the other two
// runtimes in the same sentence — ranger-base-1kvfr's own two-way check only
// ever built one of these, which is finding 4 of ranger-base-nn33e: a stale
// or invented codex or grok id in the SAME paragraph was pinned by nothing.
var codexModelID = regexp.MustCompile(`gpt-[a-z0-9]+(?:[-.][a-z0-9]+)*`)
var grokModelID = regexp.MustCompile(`grok-[a-z0-9]+(?:[-.][a-z0-9]+)*`)

// tierTripleID captures a runtime's strong/standard/fast ids IN THE ORDER
// the paragraph spells them — "<runtime> <id> / <id> / <id>". The set check
// below cannot tell a permuted mapping from the true one (ranger-base-nn33e
// finding 3: swapping the claude triple to sonnet/opus/fable-5-1 still
// passes a set comparison), so the per-tier assertions read this instead.
func tierTripleID(runtime string) *regexp.Regexp {
	return regexp.MustCompile(runtime + ` ([a-z0-9.-]+) / ([a-z0-9.-]+) / ([a-z0-9.-]+)`)
}

// notesTierParagraphNamesBuiltInIds is ARM 1 of ranger-base-1kvfr, run once
// per runtime (claude, codex, grok — ranger-base-nn33e finding 4 widened it
// from claude-only): the built-in tier table paragraph is a restatement of
// models, and must name that map's three ids, no other id of the runtime,
// and in the tier order — strong/standard/fast — the map itself holds.
func notesTierParagraphNamesBuiltInIds(t *testing.T, runtime string, id *regexp.Regexp, models map[string]string) {
	t.Helper()
	para := notesParagraph(t, notesText(t), "**Tiers (ADR 0003 §1–2).**")

	// Compared as SETS of whole ids, not with Contains: claude-fable-5 is a
	// prefix of claude-fable-5-1, so a containment check reads the stale id
	// as present in the current one and cannot tell the two apart.
	want := map[string]string{} // id -> tier, for the message
	for tier, mid := range models {
		want[mid] = tier
	}
	got := map[string]bool{}
	var named []string
	for _, mid := range id.FindAllString(para, -1) {
		if !got[mid] {
			named = append(named, mid)
		}
		got[mid] = true
		if _, ok := want[mid]; !ok {
			t.Errorf("the tier paragraph names %q, which is not a %s models id (have %v) — a stale or invented id (ranger-base-1kvfr)", mid, runtime, want)
		}
	}
	for mid, tier := range want {
		if !got[mid] {
			// The paragraph itself is ~4KB of prose; the ids it named are
			// the whole of what this arm is about.
			t.Errorf("the tier paragraph does not name %s models[%s] = %q — NOTES.md and the built-in table disagree; it names %v (ranger-base-1kvfr)", runtime, tier, mid, named)
		}
	}

	// ORDER, not just membership (ranger-base-nn33e finding 3): the triple
	// must read strong/standard/fast in that position, or the paragraph
	// tells a reader the wrong tier gets the wrong model even though every
	// id it names is a real one.
	tri := tierTripleID(runtime).FindStringSubmatch(para)
	if tri == nil {
		t.Fatalf("the tier paragraph has no %q triple in the shape %q — this pin cannot judge the mapping (ranger-base-nn33e)", runtime, runtime+" <id> / <id> / <id>")
	}
	for i, tier := range Tiers {
		if got, want := tri[i+1], models[tier]; got != want {
			t.Errorf("the %s triple's %s position reads %q, %s models[%s] = %q — the paragraph and the table disagree about WHICH tier gets WHICH model (ranger-base-nn33e)",
				runtime, tier, got, runtime, tier, want)
		}
	}
}

func TestNotesTierParagraphNamesTheBuiltInClaudeIds(t *testing.T) {
	t.Parallel()
	notesTierParagraphNamesBuiltInIds(t, "claude", claudeModelID, claudeModels)
}

func TestNotesTierParagraphNamesTheBuiltInCodexIds(t *testing.T) {
	t.Parallel()
	notesTierParagraphNamesBuiltInIds(t, "codex", codexModelID, codexModels)
}

func TestNotesTierParagraphNamesTheBuiltInGrokIds(t *testing.T) {
	t.Parallel()
	notesTierParagraphNamesBuiltInIds(t, "grok", grokModelID, grokModels)
}

var notesLoudLine = regexp.MustCompile(`tier strong wants (claude-[a-z0-9.-]+) — unavailable([^\n]*)`)

// ARM 2 of ranger-base-1kvfr: NOTES.md quotes the preflight's loud line. A
// doc that quotes a rendering is worth its quote only while the renderer
// still produces it, so this RENDERS the line — same call the launch path
// makes — and requires NOTES.md to carry that sentence and no variant of it.
//
// The sentence changed shape when ADR 0003 §3 removed automatic
// substitution (ranger-base-hv2zr): it named a substitute and now names
// none. The regex below is deliberately open after "unavailable" so the
// two-way arm still CATCHES the old sentence rather than stopping matching
// it — a stale "falling back to …" left in the prose must red this pin, not
// slip past it.
func TestNotesPreflightLoudLineIsWhatPreflightPrints(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	// strong is off the account; standard and fast are on it.
	seedCatalog(t, a, time.Minute, claudeModels[TierStandard], claudeModels[TierFast])
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if pf.Line == "" {
		t.Fatalf("the fixture must produce a loud line, or this pin quotes nothing: %+v", pf)
	}
	// NOTES.md's copy names a persona of its own, and the shipped tree may
	// not name one back (ADR 0012 App.A §5) — so the coupled half is
	// everything after the "<who>: " prefix, which is where the ids are.
	sentence := pf.Line[strings.Index(pf.Line, ": ")+len(": "):]

	notes := notesFlat(notesText(t))
	if !strings.Contains(notes, sentence) {
		t.Errorf("NOTES.md does not quote the line preflight actually prints (ranger-base-1kvfr):\n  renders %q", sentence)
	}
	// Two-way: every such sentence in NOTES.md must be THIS pair. Without
	// this, the old sentence could stay beside a new one and read as fact.
	found := notesLoudLine.FindAllStringSubmatch(notes, -1)
	if len(found) == 0 {
		t.Errorf("NOTES.md no longer quotes the loud line at all — it is the sentence that says a session's tier asked for something the account will not serve (ranger-base-1kvfr)")
	}
	for _, m := range found {
		if m[1] != claudeModels[TierStrong] {
			t.Errorf("NOTES.md quotes %q; the built-in table says strong is %q (ranger-base-1kvfr)", m[1], claudeModels[TierStrong])
		}
		// The removal's own half: no quoted line may offer a substitute
		// (ADR 0003 §3, ranger-base-hv2zr). A tail naming another model is
		// prose describing a mechanism the code no longer has.
		if strings.Contains(m[2], "falling back") {
			t.Errorf("NOTES.md still quotes a substitution: %q — automatic fallback was removed (ranger-base-hv2zr)", m[0])
		}
	}
}
