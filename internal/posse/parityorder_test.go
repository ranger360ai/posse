//go:build posse_arm2

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
	t.Parallel()
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

// ranger-base-esa0j arm 1 (audit finding 13): ADR 0025 §3's push-effect note.
// `applyPushEffectNote` (parity.go) is reached only from CheckParityIn, sets
// RealizedGate.Effect on the `git push` deny at `cage: container` alone, and
// is the one field on that struct nothing in the suite read — a note the
// operator is meant to see, with no witness that it is still there or still
// says what the record ratified.
//
// The note is a NOTE: the launcher does not know the remote's host, so the
// sentence must stay hedged ("as far as this launch is configured") and the
// class beside it must stay Cooperative. A note that hardened into a claim
// would tell an operator the push cannot land when it can.
//
// MUTATIONS RUN (each reds this test): drop the `p.Cage != CageContainer`
// guard; drop the `deniesGitPush` filter so every deny gets the note; delete
// the assignment; drop the ` — ` join in RealizedGate.String.
func TestQAPushEffectNoteRidesTheContainerPushDenyOnly(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	const push = "Bash(git push:*)"
	ag := cageAgent(t, a, "cage: container\ndeny: ["+push+", Bash(security:*)]\n")

	p := a.CheckParityIn(ag, claude, CageContainer, TierStrong, dir)
	g, ok := p.Realized[push]
	if !ok {
		t.Fatalf("fixture: the push deny is not a realized row at all:\n%s", p.String())
	}
	// The note's load-bearing words. Spelled out because this IS the
	// deliverable: the record ratified a hedge, two named enforcement
	// layers, and the condition on the :ro one.
	for _, want := range []string{
		"at cage: container the push's EFFECT still dies at an enforced layer",
		"as far as this launch is configured",
		"a path remote inside the mounts is stopped by :ro",
		"granted when the PID also denies Edit/Write",
		"a network remote by the egress proxy unless egress: names its host",
		"(ADR 0025 §3)",
		"the verb gate itself stays cooperative",
	} {
		if !strings.Contains(g.Effect, want) {
			t.Errorf("the push-effect note must carry %q:\n%s", want, g.Effect)
		}
	}
	// The note is beside the class, not instead of it. ADR 0025 §3's whole
	// point is that the VERB gate is still only cooperative here.
	if g.Class != Cooperative {
		t.Errorf("the push deny stays cooperative at the container tier; class = %q", g.Class)
	}
	// And it reaches the printed block, trailing its row after an em dash.
	row := g.String()
	if i, j := strings.Index(row, string(Cooperative)), strings.Index(row, " — at cage: container"); i < 0 || j < i {
		t.Errorf("the note must trail the class on the printed row:\n%s", row)
	}
	if !strings.Contains(p.String(), g.Effect) {
		t.Errorf("the note must survive into the rendered parity block:\n%s", p.String())
	}

	// The two arms that keep this from being a note on everything.
	if other := p.Realized["Bash(security:*)"]; other.Effect != "" {
		t.Errorf("only the push deny carries the note; Bash(security:*) got %q", other.Effect)
	}
	for _, cage := range []string{CageShims, CageSeatbelt} {
		q := a.CheckParityIn(ag, claude, cage, TierStrong, dir)
		if e := q.Realized[push].Effect; e != "" {
			t.Errorf("cage %s is not the container tier, so no effect note belongs on it: %q", cage, e)
		}
	}
}
