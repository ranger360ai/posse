//go:build posse_arm2

package posse

// QA pins for the two arms of the ranger-base-8tut fix that its own
// regression test does not reach (filed under ranger-base-kvecg, verifying
// that close).
//
// TestQACostAdapterPriceTableIsConsulted pins the POSITIVE arm: an adapter
// with a rate card of its own is the one Segment.Total() asks. Two other
// arms carry the rest of the fix, and both were MEASURED to survive being
// mutated away at HEAD (2026-09-01): each mutation below was seeded into
// its own copy of the tree and the whole internal/posse package was run
// against it, beside an unmutated control tree seeded the same way. Neither
// mutant produced a single red the control did not also produce, and
// TestQACostAdapterPriceTableIsConsulted stayed green through both:
//
//	the REFUSAL arm — an adapter that prices nothing must END the lookup,
//	not fall through to the package-level claude table. This is the second
//	half of the bead's own diagnosis ("the family fallback is substring-only:
//	a non-claude model id containing opus/sonnet/haiku/fable is silently
//	priced at claude list rates"), and a `priceFor` written to fall through
//	on a miss looks correct, keeps the positive pin green, and puts that
//	defect straight back.
//
//	the STAMP arm — Segment.Runtime is set by the DECODER at construction,
//	not only by scanProvider after decode. Every decoder calls Total() on
//	its own segments before scanProvider ever sees them (cost.go:409,
//	cost_codex.go:296, cost_grok.go:227), and grokpool.go:274 calls Decode
//	and Total() without scanProvider at all — so an unstamped segment is
//	priced with Runtime "", which resolves no adapter and lands back on
//	claude's table.
//
// Both pins are written against the REAL codex adapter rather than a
// fixture, because the fixture cannot fail the way production would: the
// thing under test is that codex's own "I price nothing" is honoured.

import (
	"testing"
	"time"
)

// qaClaudePricedNonClaudeID is a model id no provider ships and claude's
// substring family fallback prices anyway. It is the whole hazard in one
// string: `PriceFor` matches "opus" as a substring of any id.
const qaClaudePricedNonClaudeID = "gpt-5.6-opus-preview"

// The refusal arm: codex prices nothing, so a codex segment carrying an id
// claude's table WOULD price must come out unpriced — not priced at claude
// list rates.
func TestQACostAdapterMissDoesNotFallBackToClaudesTable(t *testing.T) {
	t.Parallel()
	// The wrong arm must be able to fail first: if claude's table did not
	// price this id, the zero below would prove nothing.
	p, ok := PriceFor(qaClaudePricedNonClaudeID)
	if !ok || p.In == 0 || p.Out == 0 {
		t.Fatalf("this pin needs an id claude's table prices, or its zero measures nothing: PriceFor(%q) = %+v, %v",
			qaClaudePricedNonClaudeID, p, ok)
	}

	s := newCodexSegment("a-1", "s.jsonl", time.Now(), qaClaudePricedNonClaudeID)
	s.Msgs["m1"] = &Usage{Model: qaClaudePricedNonClaudeID, In: 1_000_000, Out: 1_000_000}
	_, cost := s.Total()

	// What the claude table would have charged, had the lookup fallen
	// through — the number this pin exists to keep out of the total.
	claudeWould := (1_000_000*p.In + 1_000_000*p.Out) / 1e6
	if cost != 0 {
		t.Errorf("a codex segment must not be priced by claude's table: $%.2f (claude's rate for %q would be $%.2f)",
			cost, qaClaudePricedNonClaudeID, claudeWould)
	}
	if s.Unpriced != 1 {
		t.Errorf("an adapter that prices nothing leaves the message UNPRICED, not $0: Unpriced = %d, want 1", s.Unpriced)
	}
	// Priced() is what the report reads to print a blank instead of 0.00.
	if s.Priced() {
		t.Errorf("segment reports priced dollars it does not have: CostUSD=%v Unpriced=%d", s.CostUSD, s.Unpriced)
	}
}

// The stamp arm: each decoder names its own runtime when it builds the
// segment, because it prices that segment before anything else labels it.
func TestQADecodersStampRuntimeAtConstruction(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	if got := newCodexSegment("a-1", "s.jsonl", now, "gpt-5.6-sol").Runtime; got != "codex" {
		t.Errorf("newCodexSegment Runtime = %q, want %q — Total() prices it before scanProvider labels it", got, "codex")
	}
	if got := newGrokSegment("a-1", "s.jsonl", now, "grok-4.6").Runtime; got != "grok" {
		t.Errorf("newGrokSegment Runtime = %q, want %q", got, "grok")
	}

	// claude's decoder builds its segments inline in ScanTranscript; the
	// only way to read one back is to scan a transcript.
	dir := t.TempDir()
	p := writeTranscript(t, dir, "s.jsonl",
		`{"type":"user","timestamp":"2026-08-17T10:00:00Z","message":{"content":"hello, just chatting"}}`,
		asst("m0", "claude-opus-5", "2026-08-17T10:00:05Z", 1000, 0, 0, 100),
		`{"type":"user","timestamp":"2026-08-17T10:01:00Z","message":{"content":"Work beads issue x-1: t. Run bd show x-1"}}`,
		asst("m1", "claude-opus-5", "2026-08-17T10:01:05Z", 1000, 0, 0, 100),
	)
	segs, err := ScanTranscript(p, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 2 {
		t.Fatalf("want the interactive stretch and the bead segment, got %d", len(segs))
	}
	for _, s := range segs {
		if s.Runtime != "claude" {
			t.Errorf("ScanTranscript segment %q Runtime = %q, want %q", s.Bead, s.Runtime, "claude")
		}
	}

	// And the resolver has to be the thing that reads it: an unstamped
	// segment falls back to the global table, which is the shape the two
	// assertions above exist to keep out of the decoders.
	unstamped := &Segment{Bead: "a-1", Msgs: map[string]*Usage{
		"m1": {Model: qaClaudePricedNonClaudeID, In: 1_000_000, Out: 1_000_000},
	}}
	if _, cost := unstamped.Total(); cost == 0 {
		t.Errorf("the fallback for an unregistered runtime is claude's table; if that is no longer true this pin's premise moved")
	}
}
