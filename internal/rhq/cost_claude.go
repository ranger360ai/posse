package rhq

// The shipped Claude Code cost adapter (ADR 0012 D4) — one implementation of
// CostProvider, not the thing cost.go does.
//
// Its three parts are the three the seam names, and all three already existed
// in cost.go as the only way of counting: PriceTable/PriceFor is the price
// table, transcriptFiles is the locator (~/.claude/projects/*/*.jsonl), and
// ScanTranscript is the record decoder. The method is unchanged and stays
// documented at the top of cost.go; what changed is that it now registers
// itself and ScanCosts reaches it through the interface.
//
// Those package-level names stay exported with their meaning intact: they are
// this adapter's surface, and every caller that only ever counted Claude
// keeps working untouched.

import "time"

type claudeCost struct{}

func (claudeCost) Runtime() string { return "claude" }

// Reads/Prices: the reading this repo was built around, and the only one
// that ends in dollars from a rate card of our own (PriceTable).
func (claudeCost) Reads() string {
	return "transcript scanner (~/.claude/projects/*.jsonl, ADR 0003 §4)"
}
func (claudeCost) Prices() bool { return true }

func (claudeCost) PriceFor(model string) (Price, bool) { return PriceFor(model) }

// Transcripts uses the locator that distinguishes "no transcripts here" from
// "cannot read where the transcripts are" (ADR 0018 §3) — the quiet
// TranscriptFiles would collapse both to an empty list, and an unreadable
// root would read as $0 spent.
func (claudeCost) Transcripts(project string) ([]string, []error) { return transcriptFiles(project) }

func (claudeCost) Decode(path string, since time.Time) ([]*Segment, error) {
	return ScanTranscript(path, since)
}

func init() { RegisterCostProvider(claudeCost{}) }
