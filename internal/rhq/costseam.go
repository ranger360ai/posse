package rhq

// The cost seam (ADR 0012 D4).
//
// A provider adapter answers four questions and nothing else:
//
//	what it reads      — Reads()/Prices(): the account stage's own facts,
//	                     including whether this reading ends in dollars
//	price table        — what does this model id cost per MTok
//	transcript locator — where does this provider leave its records
//	record decoder     — how does one of those files become []*Segment
//
// The first is here rather than on the runtime table because a hand-kept
// second declaration drifts: grok gained an adapter and its `cost_adapter:`
// line was never written, so for two days the dispatch pass said "no cost
// adapter reads grok" about spend that was already in `posse cost`
// (ranger-base-0lg6). The registry is the only answer now.
//
// Everything downstream of []*Segment is arithmetic: Total/Sum, ByBead,
// DayTotal/PassTotal, the groupings and the printing all live in cost.go and
// never learn a provider's name. This is the shape the plan-window seam took
// one commit earlier — budget.go is label-agnostic over []Window — and for
// the same reason: the harness owns the arithmetic, an adapter owns the
// provider's facts.
//
// A runtime with **no adapter registered is uncounted, never $0**. That is
// ADR 0018 §3's rule ("nothing was spent" and "nothing could be counted" are
// two different facts) applied to providers rather than to unreadable files:
// such sessions land in CostReport.Uncounted and their spend is absent from
// every total, which the report states rather than silently meaning zero.

import (
	"sort"
	"time"
)

// CostProvider is one runtime's cost adapter — the whole provider surface.
type CostProvider interface {
	// Runtime is the runtime name this adapter counts, spelled the way
	// runtimes/<name>.yaml and herdr's session records spell it. It is the
	// key that decides whether a live session is counted or uncounted.
	Runtime() string

	// Reads describes, for a human, WHAT this adapter reads: the value the
	// `account` row of `posse runtime check` prints and the reason clause
	// the dispatch degrade prints. Never empty — an account stage that
	// cannot say what stands behind it is one nobody can audit.
	Reads() string

	// Prices reports whether what Reads() describes ends in DOLLARS —
	// whether a segment this adapter decodes ever carries money, from a
	// rate table or from the provider's own reported number.
	//
	// False is neither "free" nor "unreadable". It is the third state the
	// cockpit already prints as `$unpriced` beside `$uncounted` and a
	// figure (cmd/posse/cockpit.go): turns, tokens and per-bead
	// attribution are all read, and no dollar ever is — the
	// subscription-seat shape, where the provider reports no cost and no
	// list rate applies to a plan seat. A runtime in that state keeps ADR
	// 0013 §5's brake and its loud line, because the fact the brake stands
	// in for — no dollar meter on this pool — still holds; only the reason
	// clause changes, and it has to, because "no adapter reads this
	// runtime" is false there (ranger-base-0lg6).
	Prices() bool

	// PriceFor resolves a model id to list rates per MTok. ok is false when
	// the id is not recognised, and then the zero Price is returned rather
	// than an assumed one: an invented number lands in the same total as real
	// money with nothing marking it. The caller counts a false as unpriced —
	// unknown, not zero (ADR 0012 D4). A provider that reports its own money
	// has no rate card at all and returns false for everything.
	PriceFor(model string) (Price, bool)

	// Transcripts lists this provider's transcript files, optionally filtered
	// by a project-path substring. Errors mean records could not be looked
	// for, which is not the same fact as finding none — the caller must not
	// read it as a quiet day. It is a slice because a locator that walks
	// keeps walking: three unreadable project dirs are three unknown piles of
	// spend, not one (ADR 0018 §3), and the files it did find are still
	// returned alongside them.
	Transcripts(project string) ([]string, []error)

	// Decode segments one transcript into the work the dispatcher sent it,
	// dropping records older than since. A decoder for a provider that
	// reports cumulative cost snapshots calls Segment.NoteCumulative rather
	// than filling Msgs; the max-not-sum rule lives there, so no decoder has
	// to remember it.
	Decode(path string, since time.Time) ([]*Segment, error)
}

// costProviders is the adapter registry, keyed by runtime name.
var costProviders = map[string]CostProvider{}

// RegisterCostProvider adds an adapter, replacing any already registered for
// that runtime. Shipped adapters register from init(); an instance that must
// override one registers after them.
func RegisterCostProvider(p CostProvider) { costProviders[p.Runtime()] = p }

// CostProviderFor returns the adapter for a runtime, if one is registered.
// The comma-ok is the uncounted test: no adapter, no number.
func CostProviderFor(runtime string) (CostProvider, bool) {
	p, ok := costProviders[runtime]
	return p, ok
}

// CostProviders lists the registered adapters in runtime-name order, so a
// scan over them is deterministic and a report built from one is stable.
func CostProviders() []CostProvider {
	out := make([]CostProvider, 0, len(costProviders))
	for _, p := range costProviders {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Runtime() < out[j].Runtime() })
	return out
}

// CountedRuntimes names the runtimes that have an adapter — exactly the
// ones `posse cost` reads at all. Anything else is uncounted, its sessions
// absent from every total rather than counted as zero.
//
// Read, not priced: an adapter here may still return false from Prices(),
// and then the runtime's turns and tokens are counted while its dollars
// never are. `Runtime.CostPriced()` is that narrower question.
func CountedRuntimes() []string {
	out := make([]string, 0, len(costProviders))
	for _, p := range CostProviders() {
		out = append(out, p.Runtime())
	}
	return out
}
