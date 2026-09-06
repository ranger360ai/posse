//go:build posse_arm3

package posse

// QA pin for the leg of the cost seam that used to have no consumer
// (ranger-base-8tut, found verifying the ranger-base-k7nb close).
//
// costseam.go says an adapter answers four questions — what it reads, price
// table, transcript locator, record decoder. Before this bead three of the
// four were wired: the scan called Transcripts and Decode, and the account
// stage called Reads/Prices. Nothing in production ever called PriceFor —
// MEASURED 2026-08-29, and the measurement was a compile: deleting the
// PriceFor line from the CostProvider interface left `go build ./...`
// clean, while `go vet ./...` named ONE call site — cost_codex_test.go:435,
// a test. Segment.Total() priced every segment that was not ProviderPriced
// through Usage.Priced()/Usage.Cost(), and both called the package-level
// PriceFor: claude's table, plus its substring family fallback, applied
// regardless of Segment.Runtime. (Those two methods no longer exist: routing
// Total() past them left them with no production caller and the same compile
// proof, so ranger-base-xqtgv deleted them.)
//
// Fixed by routing Segment.Total() through Segment.priceFor(model), which
// asks CostProviderFor(s.Runtime) first and falls back to the global table
// only when no adapter is registered for that runtime. claude's own adapter
// still delegates to that same global, so the claude reading is byte-for-
// byte unchanged; the three decoders (cost.go's ScanTranscript, cost_codex.go,
// cost_grok.go) now stamp Segment.Runtime at construction so the resolver
// has an answer the moment Total() runs, not only after scanProvider labels
// the segment post-decode.

import (
	"testing"
	"time"
)

// tablePriced is an adapter that prices its OWN model — the case the seam
// documents and the harness does not call. Its rate is deliberately absurd
// so a number sourced from it cannot be confused with one from claude's
// table or with an arithmetic slip.
type tablePricedCost struct{}

func (tablePricedCost) Runtime() string { return "qa-tablepriced" }

func (tablePricedCost) Reads() string { return "qa fixture (a rate card of its own)" }
func (tablePricedCost) Prices() bool  { return true }

func (tablePricedCost) PriceFor(model string) (Price, bool) {
	if model == "qa-model-1" {
		return Price{In: 1000, Out: 2000}, true
	}
	return Price{}, false
}

func (tablePricedCost) Transcripts(string) ([]string, []error) { return nil, nil }

func (tablePricedCost) Decode(string, time.Time) ([]*Segment, error) { return nil, nil }

// A segment on a runtime whose adapter prices its own models must be priced
// by THAT adapter (ranger-base-8tut: Segment.Total now resolves through
// Segment.priceFor, which asks CostProviderFor(s.Runtime) before falling
// back to claude's global table).
//
// SERIAL, and not by omission: RegisterCostProvider writes the package-level
// map costProviders and the cleanup deletes from it. Every parallel test that
// prices a Segment reads that map through Segment.priceFor -> CostProviderFor,
// so one writer beside them is a concurrent map read/write — a fatal throw of
// the whole binary, not a FAIL line (ranger-base-btdvw).
func TestQACostAdapterPriceTableIsConsulted(t *testing.T) {
	p := tablePricedCost{}
	RegisterCostProvider(p)
	t.Cleanup(func() { delete(costProviders, p.Runtime()) })

	s := &Segment{
		Bead:    "a-1",
		Runtime: p.Runtime(),
		Msgs: map[string]*Usage{
			"m1": {Model: "qa-model-1", In: 1_000_000, Out: 1_000_000},
		},
	}
	_, cost := s.Total()

	// 1 MTok in at $1000 + 1 MTok out at $2000. Any other number means the
	// adapter's table was not the one consulted.
	if cost != 3000 {
		t.Fatalf("a segment on %s must be priced by its own adapter: $%.2f, want $3000.00", p.Runtime(), cost)
	}
	if s.Unpriced != 0 {
		t.Errorf("the adapter priced this model; it must not count as unpriced: %d", s.Unpriced)
	}
}
