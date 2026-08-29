package rhq

// QA pin for the leg of the cost seam that has no consumer
// (ranger-base-8tut, found verifying the ranger-base-k7nb close).
//
// costseam.go says an adapter answers four questions — what it reads, price
// table, transcript locator, record decoder. Three of the four are wired:
// the scan calls Transcripts and Decode, and the account stage calls
// Reads/Prices. Nothing in production ever calls PriceFor.
//
// MEASURED 2026-08-29, and the measurement is a compile: delete the
// PriceFor line from the CostProvider interface and `go build ./...` stays
// clean, while `go vet ./...` names ONE call site — cost_codex_test.go:435,
// a test. Segment.Total() prices every segment that is not ProviderPriced
// through Usage.Priced()/Usage.Cost(), and both call the package-level
// PriceFor: claude's table, plus its substring family fallback, applied
// regardless of Segment.Runtime.
//
// Latent today (claude's adapter delegates to that same global; codex and
// grok price nothing and are ProviderPriced or unpriced), which is why this
// is a skip and not a red build. It stops being latent the first time an
// adapter ships a rate card of its own, or the first time a non-claude
// model id contains "opus"/"sonnet"/"haiku"/"fable" and is silently billed
// at claude list rates.

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
// by THAT adapter. Read this skip as a red: it is the seam's third leg, and
// today the pricing path never asks.
func TestQACostAdapterPriceTableIsConsulted(t *testing.T) {
	t.Skip("ranger-base-8tut: CostProvider.PriceFor has no production caller — Segment.Total prices every runtime through claude's table")

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
