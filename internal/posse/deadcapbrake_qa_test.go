package posse

// QA pin for ranger-base-vcqzb, verifying ranger-base-2eeb's close.
//
// countedBrake has three arms — grok with `grok_guard_week:` armed, grok
// unarmed, and every OTHER priced runtime — and 2eeb's four pins drive the
// first two plus the unpriced mirror (codex). The third arm was reachable
// and unmeasured: claude is priced (cost_claude.go Prices() is true), so
// `uncounted_cap_claude:` is dead exactly as grok's is, and the wallet
// clause it gets is the only thing the operator is sent to. Replacing that
// clause's whole text with a constant left every pin green.
//
// Two claims, and the second is why the arm exists at all: the line names
// the wallet caps, and it does NOT carry grok's pool clause. A brake table
// that collapsed to grok's wording would otherwise send a claude operator to
// a dial that meters a pool claude's dollars do not come out of.

import (
	"strings"
	"testing"
)

func TestQADeadCapOnAPricedNonPoolRuntimeNamesTheWalletBrake(t *testing.T) {
	f := uncountedPassOn(t, "claude", "uncounted_cap_claude: 1\n",
		`[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`,
		"ranger", "scout")

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out, errs := dispatcherOut(f.d), f.errb.String()
	// The dead key brakes nothing, and the launches really went to claude:
	// without the second half a fixture that fell back to codex would read
	// as two launches under a cap that is alive there.
	if n != 2 {
		t.Fatalf("a dead cap of 1 must brake neither bead, got n=%d:\n%s\n%s", n, out, errs)
	}
	if got := strings.Count(out, "creating session "); got != 2 || strings.Count(out, "claude/") != 2 {
		t.Fatalf("want two create lines, both on claude, got %d create lines:\n%s", got, out)
	}
	if got := strings.Count(errs, "uncounted_cap_claude:"); got != 1 {
		t.Errorf("want the dead key named exactly once a pass, got %d:\n%s", got, errs)
	}
	for _, want := range []string{
		`config uncounted_cap_claude: "1" does not apply`,
		"prices claude's spend",
		"the brake on claude is budget_pass:/budget_day: over those dollars",
		"(ADR 0010 §3)",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("the line must carry %q:\n%s", want, errs)
		}
	}
	// The pool clause belongs to grok alone. This is the assertion that
	// kills a brake table collapsed onto grok's wording.
	if strings.Contains(errs, "grok_guard_week:") || strings.Contains(errs, "over the pool") {
		t.Errorf("claude's dollars do not come out of the grok pool; the line must not offer that dial:\n%s", errs)
	}
	// Dead in both directions, as on grok: no degrade, no ledger.
	if strings.Contains(out, "account-degraded") {
		t.Errorf("claude is priced; the pass must not degrade it:\n%s", out)
	}
	if got := len(f.uncountedLedger(t)); got != 0 {
		t.Errorf("nothing may be ledgered for a priced runtime, got %d lines", got)
	}
}
