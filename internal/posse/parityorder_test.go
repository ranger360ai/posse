package posse

import (
	"strings"
	"testing"
)

// rangerhq-epes: the ✓ half of a parity block came out of a Go map, so two
// runs of `posse gates <persona>` could not be diffed — the lines moved
// while the content stood still, and `sort | diff` was the only honest
// check. The order is fixed now: deny rules by rule text, then egress:,
// then skills:, then the record gate.
func TestParityStringOrdersTheRealizedLines(t *testing.T) {
	p := Parity{
		Runtime: "claude", Cage: CageShims, Tier: TierStrong,
		Realized: map[string]RealizedGate{
			"Write":                       {Class: Enforced, Detail: "L2 seatbelt"},
			"Bash(git push:*)":            {Class: Cooperative, Detail: "L1 shim (subcommand, option-aware)"},
			"mcp__deploy":                 {Class: Enforced, Detail: "L4 container"},
			"Edit":                        {Class: Enforced, Detail: "L2 seatbelt"},
			"skills: distributed-systems": {Detail: "claude --plugin-dir (session-only, additive)"},
			RecordReachGate:               {Detail: "cage shims has no file wall"},
			"egress: api.anthropic.com":   {Class: Enforced, Detail: "L4 --internal network + CONNECT proxy"},
			"Bash(security:*)":            {Class: Cooperative, Detail: "L1 shim (whole verb)"},
		},
		Degraded: []string{"WebFetch — runtime-native only below cage: container"},
	}
	want := []string{
		"Bash(git push:*)", "Bash(security:*)", "Edit", "Write", "mcp__deploy",
		"egress: api.anthropic.com",
		"skills: distributed-systems",
		RecordReachGate,
	}

	// Map iteration randomizes per range, so one call proves little: the
	// pin is that thirty of them agree, and agree with the order above.
	first := p.String()
	for i := 0; i < 30; i++ {
		out := p.String()
		if out != first {
			t.Fatalf("two renders of the same matrix differ (run %d):\n%s\n---\n%s", i, first, out)
		}
		at := -1
		for _, gate := range want {
			j := strings.Index(out, "    ✓ "+gate)
			if j < 0 {
				t.Fatalf("gate %q is missing from the block:\n%s", gate, out)
			}
			if j <= at {
				t.Fatalf("gate %q is out of order:\n%s", gate, out)
			}
			at = j
		}
	}
	// The ✗ half is unchanged, and still follows the ✓ half.
	if i, j := strings.Index(first, "    ✗ WebFetch"), strings.LastIndex(first, "    ✓ "); i < j {
		t.Errorf("the ✗ lines must follow the ✓ lines:\n%s", first)
	}
}
