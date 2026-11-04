//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-u4uwg, verifying the close of ranger-base-s69ep.
//
// s69ep's deliverable is a reading that ends a SILENCE: a policy-tier
// drop-in still pinning GIT_CONFIG_SYSTEM over the launch pin, which
// GitSystemScopeDegraded cannot see because resolution succeeded. The close
// shipped that reading twice over — a launch warning beside its sibling in
// planLaunch, and the `git: policy-tier pin` row in `posse gates` — and
// pinned the READING (policyGitSystemScopeFindings, policyPinDegraded,
// policyPinRowDetail) six ways in policypin_test.go.
//
// What no shipped pin reached was either WIRING. MEASURED under this bead:
// neutering the one line that adds the row —
//
//	p.Realized[PolicyPinGate] = RealizedGate{Detail: ...}  ->  _ = ...
//
// left the whole shipped suite green (internal/posse arm 1, and
// internal/treepins' LaunchCheckSources pin). The row's only existing
// witness, TestPolicyPinRowOnlyForSettingsPinRuntimes, asserts the row's
// ABSENCE for a runtime that carries no settings pin — an absence that is
// also true of a row that is never added at all. This is the positive
// control that absence needed.
//
// The row's own existence is not the only fact here. The close made a
// deliberate, reversible design choice and wrote it down: the row is
// PRINTED and never DEGRADING, because refusing every claude dispatch over
// a root-owned file no persona can replace would stop the fleet rather than
// fix the box. That choice is what the two assertions on Class and Degraded
// below hold, so that moving the row into p.Degraded is a decision someone
// takes on purpose and not a line that drifts.
//
// The sibling launch warning in planLaunch is NOT pinned here, and neither
// are its two neighbours (GitSystemScopeDegraded, GitGlobalScopeDegraded):
// deleting any of the three warn() calls leaves this package green too.
// That is a house-wide class rather than this close's slip, and it is filed
// for the code lane rather than invented here. IT IS HELD NOW, one file over:
// verify_zpk8r_qa_test.go's
// TestQAEveryZeroArgDegradedLaunchReadingIsPrintedAtItsCallSite censuses the
// derived class — an exported, zero-argument, string-returning `*Degraded`
// function in this package — for a guarded call site that prints it, so all
// three warn() lines have a witness and a fourth reading added tomorrow is in
// the class without anyone editing a list (ranger-base-zpk8r finding 1).

import (
	"runtime"
	"strings"
	"testing"
)

func TestQAU4uwgPolicyTierRowIsAddedForASettingsPinRuntime(t *testing.T) {
	t.Parallel()
	if claudePolicyDir(runtime.GOOS) == "" {
		t.Skipf("no known claude policy dir for GOOS=%s, so applyPolicyPin adds no row anywhere on this platform; the reading's own pins still hold", runtime.GOOS)
	}

	// Both ways a runtime can carry claude's settings pin, because
	// applyPolicyPin's guard is a disjunction and a pin that exercises one
	// arm of it cannot see the other go away.
	for _, c := range []struct {
		name string
		rt   *Runtime
	}{
		{"SettingsPin", &Runtime{Name: "claude", SettingsPin: func() string { return "{}" }}},
		{"FleetSettings", &Runtime{Name: "claude", FleetSettings: func([]string) string { return "{}" }}},
	} {
		p := Parity{Realized: map[string]RealizedGate{}}
		applyPolicyPin(&p, c.rt)
		g, ok := p.Realized[PolicyPinGate]
		if !ok {
			t.Errorf("%s: a runtime carrying claude's settings pin got NO %q row — the launch reads the policy tier and `posse gates` says nothing about it, which is the silence ranger-base-s69ep was filed about", c.name, PolicyPinGate)
			continue
		}
		// The detail is the reading, not a placeholder. GIT_CONFIG_SYSTEM is
		// the one token BOTH wordings carry — the clean row names the
		// variable it found nothing pinning, the DEGRADED row names the
		// variable the policy tier outranks — so this holds on a clean box
		// and on a stale one without asserting which this box is.
		if !strings.Contains(g.Detail, "GIT_CONFIG_SYSTEM") {
			t.Errorf("%s: %q row detail does not name the variable it reads: %q", c.name, PolicyPinGate, g.Detail)
		}
		// PRINTED, NEVER DEGRADING (the close's stated choice, matching the
		// sibling GitSystemScopeDegraded launch line). The empty class is
		// what makes `posse gates` print the detail bare instead of as an
		// adversarial gate claim, and staying out of Degraded is what keeps
		// a root-owned file off the fleet's dispatch path.
		if g.Class != "" {
			t.Errorf("%s: %q is classed %q; it is a printed reading, not a realized gate claim — classing it renders a root-owned file nobody can fix as a wall", c.name, PolicyPinGate, g.Class)
		}
		for _, d := range p.Degraded {
			if strings.Contains(d, PolicyPinGate) {
				t.Errorf("%s: %q reached p.Degraded (%q) — that refuses every claude dispatch on this box over a file only root can replace; promoting it is an operator call, not a drift", c.name, PolicyPinGate, d)
			}
		}
	}

	// THE CONTROL, in the same test and over the same predicate: a runtime
	// that carries neither settings payload must still get no row. Without
	// this, every assertion above is satisfied by an applyPolicyPin that
	// adds the row unconditionally — which would put a claude-only reading
	// on codex's and grok's blocks and read as a fact about them.
	p := Parity{Realized: map[string]RealizedGate{}}
	applyPolicyPin(&p, &Runtime{Name: "codex"})
	if g, ok := p.Realized[PolicyPinGate]; ok {
		t.Errorf("a runtime with no settings pin got the %q row (%q) — it does not read claude's policy tier, so the row would be a reading about a file it never opens", PolicyPinGate, g.Detail)
	}
	p2 := Parity{Realized: map[string]RealizedGate{}}
	applyPolicyPin(&p2, nil)
	if _, ok := p2.Realized[PolicyPinGate]; ok {
		t.Errorf("a nil runtime got the %q row", PolicyPinGate)
	}
}
