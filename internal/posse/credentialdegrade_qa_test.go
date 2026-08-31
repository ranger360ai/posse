package posse

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// ADR 0018 §2 — no policy fork by failure class — with the class rangerhq-ytyj
// added, on the arm this instance actually sits on.
//
// WHY THIS FILE EXISTS (found verifying rangerhq-ytyj's close, ranger-base-uon7).
// TestBlindDegradeDoesNotForkOnFailureClass pins §2 by pairing a refused
// keychain read against a dead socket — the two classes that predate ytyj.
// ytyj introduced a third, *AuthFailure, and with it a new predicate anything
// could branch on: AuthFailureReason(err). Nothing was added to the §2 pin, so
// the rule is enforced over a class set that no longer covers the code.
//
// MEASURED, not argued. Planting the fork the close's own writeup claims its
// mutation (e) catches —
//
//	func (d *Dispatcher) blindFork(blind time.Duration, err error) {
//		park := func(why string) { ... }
//		if AuthFailureReason(err) != nil {   // <- the fork
//			park(", credential condition")
//			return
//		}
//
// — leaves `go test ./internal/rhq -run 'TestBlind|TestQABlind|Degrade|
// Credential|TestGovG5|TestPulseDelivers|TestPlanReader'` GREEN: ok, 114.6s,
// against an 81.6s green baseline. Every one of the six pins that close
// shipped passes under it, because all six either park anyway (the ledger is
// unarmed in their rigs) or never reach blindFork at all (they sit under the
// budget).
//
// AND IT IS THE LIVE ARM. `budget_pass:`/`budget_day:` are armed on this
// instance, so past `plan_guard_blind_max:` a credential failure must DEGRADE
// under the ledger's own rungs — never park. A fork that parks it stops the
// shop on a condition ADR 0018 says is not a reason to stop, and does it
// silently: the park line and the degrade line are both "loud", so a log
// reader sees a plausible sentence either way.
//
// The mutation that must red this pin is the fork above; the mutation that
// must red its sibling below is the same fork with the arms swapped.
func TestQABlindDegradeDoesNotForkOnTheCredentialClass(t *testing.T) {
	r := newBlindRig(t, ledgerArmedCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.ps.status = http.StatusUnauthorized
	r.ps.body = "unauthorized"
	r.at(4 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("a 401 past the budget degrades exactly like a dead socket — an armed ledger is the floor: %d dispatched\n%s", n, r.out())
	}
	out := r.out()
	if strings.Contains(out, "— skipped") {
		t.Errorf("parking a credential failure IS a policy fork by failure class (ADR 0018 §2):\n%s", out)
	}
	// The class still shapes the DIAGNOSTIC — that is the whole of what ytyj
	// changed, and the degraded line has to carry it too, not only the park
	// line the close's pins measured.
	for _, want := range []string{
		"degraded, running under ledger brake",
		"401",
		"credential stale",
		"epoch $8.20/$30.00",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the degraded line must carry %q, got:\n%s", want, out)
		}
	}
}

// The other direction of the same fork, so a future edit cannot flip it by
// accident: with Dial E UNSET the plan guard is the last armed brake and a
// credential failure parks, exactly like every other blind read. Without this
// arm the pin above passes over a blindFork that degrades unconditionally.
func TestQABlindCredentialFailureStillParksWhenLedgerUnarmed(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.ps.status = http.StatusUnauthorized
	r.ps.body = "unauthorized"
	r.at(4 * time.Hour)

	if n := r.run(t); n != 0 {
		t.Fatalf("with no ledger armed the guard is the last brake and parks, 401 or not: %d dispatched\n%s", n, r.out())
	}
	out := r.out()
	if !strings.Contains(out, "— skipped") {
		t.Errorf("a parked pass must say so:\n%s", out)
	}
	if strings.Contains(out, "degraded, running under ledger brake") {
		t.Errorf("nothing is counting, so there is no floor to run under:\n%s", out)
	}
}
