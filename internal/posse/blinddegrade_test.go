package posse

// ADR 0018 — blind meter, armed ledger. The 2026-08-26 outage in one
// sentence: the plan guard went blind, it was the ONLY armed brake, and an
// hour of the shop's dispatch was spent proving that a brake with nothing
// under it must fail closed. The fix is not "degrade" — it is a fork on
// whether anything else is still counting.
//
// Hermetic like its neighbours: blindRig's dead endpoint for the blind
// state, an injected Spend for the dollars, an injected clock for the age.
// Nothing here reads the operator's transcripts or credentials.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ledgerArmedCfg is guardOn with Dial E armed — the ADR's own numbers, so a
// failure message reads like the config it is about.
const ledgerArmedCfg = guardOn + "\nbudget_pass: 30\nbudget_day: 250"

// spendReport is spendStub's shape as a value, so a test can hand back a
// report that also carries a read failure.
func spendOf(usd float64, readErr error) *CostReport {
	rep := &CostReport{Beads: []*Segment{{Bead: "spent", Start: time.Now(), CostUSD: usd}}}
	if readErr != nil {
		rep.noteUnread(readErr)
	}
	return rep
}

// §1, the whole point: past the blind budget with the ledger armed, the
// on-meter bead LAUNCHES, and the pass says out loud what is holding it.
func TestBlindDegradesUnderArmedLedger(t *testing.T) {
	r := newBlindRig(t, ledgerArmedCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.blind()
	r.at(4 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("an armed ledger is a floor under the blind meter: %d dispatched\n%s", n, r.out())
	}
	out := r.out()
	if strings.Contains(out, "— skipped") {
		t.Errorf("a degraded pass parks nothing:\n%s", out)
	}
	// The loud line, in the ADR's own shape: blind duration, read error,
	// ledger state — on the pass output, not stderr.
	for _, want := range []string{
		"plan guard: blind 4h00m",
		"usage endpoint unreachable",
		"degraded, running under ledger brake",
		"epoch $8.20/$30.00",
		"day $8.20/$250.00",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the degraded line must carry %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(r.err(), "degraded") {
		t.Errorf("a degraded pass is an outcome, not a warning — it belongs on d.Out: %q", r.err())
	}
}

// The other half of the fork, and the behaviour the outage says is right:
// with Dial E unset the plan guard is the last armed brake, so it still
// parks. This is TestBlindOverBudgetSkips' rule restated as a fork, so a
// future edit cannot flip it by accident.
func TestBlindParksWhenLedgerUnarmed(t *testing.T) {
	for _, cfg := range []struct{ name, extra string }{
		{"no caps", ""},
		// A cap that is not a positive dollar amount is not a cap (Dial E
		// already refuses it and says so). Unarmed means unarmed.
		{"malformed cap", "\nbudget_day: lots"},
	} {
		t.Run(cfg.name, func(t *testing.T) {
			r := newBlindRig(t, guardOn+cfg.extra)
			r.d.Unattended = true
			r.d.Spend = func(time.Time) *CostReport { t.Fatal("an unarmed ledger must not be scanned"); return nil }
			r.blind()
			r.at(4 * time.Hour)

			if n := r.run(t); n != 0 {
				t.Fatalf("the last armed brake fails closed: %d dispatched\n%s", n, r.out())
			}
			if !strings.Contains(r.out(), "plan guard: blind 4h00m") || !strings.Contains(r.out(), "— skipped") {
				t.Errorf("want today's park line, got:\n%s", r.out())
			}
			if strings.Contains(r.out(), "degraded") {
				t.Errorf("nothing is holding this pass — it must not claim to be degraded:\n%s", r.out())
			}
		})
	}
}

// "Bounded by the ledger, never by wall-clock." A week blind is not a
// reason to stop, and it is not a reason to run either — the dollars decide.
func TestBlindDegradeIsBoundedByMoneyNotTheClock(t *testing.T) {
	for _, tc := range []struct {
		name     string
		blind    time.Duration
		spend    float64
		want     int
		wantLine string
	}{
		{"a week blind, ledger quiet", 7 * 24 * time.Hour, 1.00, 1, "degraded, running under ledger brake"},
		{"a minute past the budget, ledger spent", 11 * time.Minute, 40, 0, "budget: epoch $40.00 of $30.00"},
		{"a week blind, ledger spent", 7 * 24 * time.Hour, 40, 0, "budget: epoch $40.00 of $30.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newBlindRig(t, ledgerArmedCfg)
			r.d.Unattended = true
			r.d.Spend = func(time.Time) *CostReport { return spendOf(tc.spend, nil) }
			r.blind()
			r.at(tc.blind)

			if n := r.run(t); n != tc.want {
				t.Fatalf("want %d dispatched, got %d:\n%s", tc.want, n, r.out())
			}
			if !strings.Contains(r.out(), tc.wantLine) {
				t.Errorf("want %q in the pass output, got:\n%s", tc.wantLine, r.out())
			}
			// A bead stopped by the ledger is stopped by the LEDGER, and its
			// line says so: the pass still declares itself degraded (the
			// brake IS the ledger), but no bead wears the blind guard's
			// park, or a spend ceiling reads as a monitoring outage.
			if tc.want == 0 && strings.Contains(r.out(), "plan guard: blind 11m —") {
				t.Errorf("unexpected shape:\n%s", r.out())
			}
			if tc.want == 0 && strings.Contains(r.out(), "(usage endpoint unreachable) — skipped") {
				t.Errorf("the ledger's stop must not be reported as a blind park:\n%s", r.out())
			}
		})
	}
}

// Dial E's rungs apply as always, so the 80% step-down is what a degraded
// pass gets before the stop — the ADR's "the ledger's rungs apply as
// always", checked rather than assumed.
func TestBlindDegradeStepsDownAtEightyPercent(t *testing.T) {
	// The day cap alone: the pass window is the tighter one under
	// ledgerArmedCfg and would stop this bead outright, which is a
	// different rung than the one under test.
	r := newBlindRig(t, guardOn+"\nbudget_day: 250")
	budgetPersona(t, r.d.App, "ranger", "[go]", "tier: standard\n") // only a default standard can step
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(210, nil) } // 84% of $250
	r.blind()
	r.at(4 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("84%% is a step-down, not a stop: %d dispatched\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "fast via budget step-down at day 84%, was standard via PID") {
		t.Errorf("the degraded launch must carry the stepped-down tier:\n%s", r.out())
	}
	if got := delivered(t, r.d.App, r.fake); !strings.Contains(got, "runtime/tier: claude/fast") {
		t.Errorf("and the persona must be told the tier it is running at:\n%s", got)
	}
}

// §3: an armed cap over an unreadable ledger counts nothing, which is the
// unarmed case wearing the armed case's clothes. Park, and name both
// failures — the blind meter AND the ledger that could not stand in for it.
func TestBlindDegradeParksWhenTheLedgerCannotBeRead(t *testing.T) {
	r := newBlindRig(t, ledgerArmedCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport {
		return spendOf(8.20, fmt.Errorf("open transcripts: permission denied"))
	}
	r.blind()
	r.at(4 * time.Hour)

	if n := r.run(t); n != 0 {
		t.Fatalf("an unreadable ledger is not a licence to spend: %d dispatched\n%s", n, r.out())
	}
	out := r.out()
	for _, want := range []string{"plan guard: blind 4h00m", "ledger unreadable", "permission denied", "— skipped"} {
		if !strings.Contains(out, want) {
			t.Errorf("the park line must carry %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "degraded") {
		t.Errorf("a pass with nothing counting must not claim a brake:\n%s", out)
	}
	if calls := bdCalls(t, r.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a park claims nothing, got: %s", calls)
	}
}

// §3's other half: outside a degraded pass, an unreadable ledger with caps
// armed is named on stderr — once per pass, not once per bead — and the
// pass still runs on the floor it could read.
func TestSightedPassNamesAnUnreadableLedgerOncePerPass(t *testing.T) {
	r := newBlindRig(t, ledgerArmedCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(1.00, fmt.Errorf("open transcripts: permission denied")) }

	if n := r.run(t); n != 1 {
		t.Fatalf("a sighted pass runs on the floor it could read: %d dispatched\n%s", n, r.out())
	}
	if got := strings.Count(r.err(), "the ledger counts less than was spent"); got != 1 {
		t.Errorf("want one stderr line for the pass, got %d: %q", got, r.err())
	}
	if !strings.Contains(r.err(), "permission denied") {
		t.Errorf("the line must name the read failure: %q", r.err())
	}
	// Per BEAD, within the pass: silent. The check runs before every
	// launch, and a wall of the same line is what this dedupe exists for.
	r.d.budget()
	if got := strings.Count(r.err(), "the ledger counts less than was spent"); got != 1 {
		t.Errorf("the same pass must not say it twice: %d lines\n%q", got, r.err())
	}
	// A new pass is a new reading, so a fault that outlives one pass keeps
	// being visible. (The bead itself is held by the session the first pass
	// made, so the second pass never reaches a launch — ask the ledger
	// directly for the witness.)
	r.at(time.Minute)
	r.run(t)
	r.d.budget()
	if got := strings.Count(r.err(), "the ledger counts less than was spent"); got != 2 {
		t.Errorf("a new pass takes a fresh reading and says so: %d lines\n%q", got, r.err())
	}
}

// §2: no policy fork by failure class. A refused keychain read and a dead
// socket are one state — no reading — and both degrade under an armed
// ledger. (The classes still shape the DIAGNOSTIC, which is why the line
// below must still name the keychain.)
func TestBlindDegradeDoesNotForkOnFailureClass(t *testing.T) {
	r := newBlindRig(t, ledgerArmedCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	keychainOnly(planReaderOf(r.d), func() (string, error) {
		return "", Die("keychain item %q unreadable", KeychainService)
	})
	r.at(4 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("a keychain failure degrades exactly like a dead socket: %d dispatched\n%s", n, r.out())
	}
	if !strings.Contains(r.out(), "degraded, running under ledger brake") || !strings.Contains(r.out(), "keychain") {
		t.Errorf("the degraded line must still carry the real diagnosis:\n%s", r.out())
	}
}

// Loud means loud: every pass, for as long as it lasts. The hourly quiet
// window belongs to the fail-open note and must not swallow this.
func TestBlindDegradeIsLoudEveryPass(t *testing.T) {
	r := newBlindRig(t, ledgerArmedCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.blind()

	for i, at := range []time.Duration{12 * time.Minute, 20 * time.Minute, 40 * time.Minute} {
		r.at(at)
		r.run(t)
		if got := strings.Count(r.out(), "degraded, running under ledger brake"); got != i+1 {
			t.Fatalf("pass %d: %d degraded lines, want %d\n%s", i+1, got, i+1, r.out())
		}
	}
	// …each carrying its own age, not the crossing's.
	for _, age := range []string{"blind 12m", "blind 20m", "blind 40m"} {
		if !strings.Contains(r.out(), age) {
			t.Errorf("each degraded line carries the current age (%s):\n%s", age, r.out())
		}
	}
}

// The fork is the unattended path only. An attended pass fails open before
// it, so an armed ledger changes nothing a human is watching.
func TestBlindDegradeIsUnattendedOnly(t *testing.T) {
	r := newBlindRig(t, ledgerArmedCfg)
	// Inject the spend, as every sibling here does. Without it this rig falls
	// through to the real ScanCosts over the operator's own ~/.claude and
	// ~/.grok, so what it asserts depends on how much was spent on the machine
	// that day — it passed only while grok was invisible to the scanner.
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.blind()
	r.at(4 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("an attended pass fails open at any blind age: %d\n%s", n, r.out())
	}
	if strings.Contains(r.out(), "degraded") {
		t.Errorf("attended blindness never reaches the fork:\n%s", r.out())
	}
	if !strings.Contains(r.err(), "pass not gated") {
		t.Errorf("want the attended stderr line, got %q", r.err())
	}
}

// `plan_guard_blind_max: 0` is quiet tolerance without end — the escape
// hatch, unchanged by the fork. It is not a third policy: nothing is
// declared, because nothing has been decided.
func TestBlindMaxZeroIsUntouchedByAnArmedLedger(t *testing.T) {
	r := newBlindRig(t, ledgerArmedCfg+"\nplan_guard_blind_max: 0")
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.blind()
	r.at(6 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("the escape hatch still never fails closed: %d\n%s", n, r.out())
	}
	if strings.Contains(r.out(), "degraded") || strings.Contains(r.out(), "— skipped") {
		t.Errorf("under the hatch there is no fork to declare:\n%s", r.out())
	}
}

// ADR 0013 §3 is untouched: an off-meter bead launches through any of this,
// and a degraded pass does not start claiming one differently.
func TestBlindDegradeLeavesOffMeterBeadsAlone(t *testing.T) {
	r := newBlindRig(t, ledgerArmedCfg)
	r.d.Unattended = true
	r.d.Runtime = "grok"
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.blind()
	r.at(4 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("--runtime grok launches through a blind guarded meter: %d\n%s", n, r.out())
	}
	if got := delivered(t, r.d.App, r.fake); !strings.Contains(got, "runtime/tier: grok/") {
		t.Errorf("the off-meter bead must still go to its own runtime:\n%s", got)
	}
}

// The recovery path still works through the fork: the first good reading
// clears the clock, and the degraded declaration goes with it.
func TestBlindDegradeStopsOnTheFirstGoodReading(t *testing.T) {
	r := newBlindRig(t, ledgerArmedCfg)
	r.d.Unattended = true
	r.d.Spend = func(time.Time) *CostReport { return spendOf(8.20, nil) }
	r.blind()
	r.at(4 * time.Hour)
	r.run(t)
	before := strings.Count(r.out(), "degraded")
	if before == 0 {
		t.Fatal("setup: want a degraded pass first")
	}

	r.sighted()
	r.at(4*time.Hour + 2*time.Minute)
	r.run(t)
	if got := strings.Count(r.out(), "degraded"); got != before {
		t.Errorf("a sighted pass declares nothing:\n%s", r.out())
	}
	if !strings.Contains(r.err(), "reading restored after") {
		t.Errorf("want the recovery line, got %q", r.err())
	}
}

// ── §3 at the source: the cost scan ────────────────────────────────────────

// The swallow this ADR names (cost.go's `segs, _ :=`): a transcript that
// cannot be read used to contribute $0 and say nothing, so an armed cap
// over a broken ledger read as an empty day.
func TestScanCostsDistinguishesUnreadableFromEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	proj := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}

	// A project dir with no transcripts is an honest empty scan.
	if rep := ScanCosts("", time.Time{}); rep.ReadErr != nil || rep.Unread != 0 {
		t.Errorf("no records is not a read failure: %v (%d unread)", rep.ReadErr, rep.Unread)
	}

	// A transcript that cannot be opened is spend of unknown size.
	f := filepath.Join(proj, "s.jsonl")
	if err := os.WriteFile(f, []byte("{}\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; the permission arm needs an unprivileged uid")
	}
	rep := ScanCosts("", time.Time{})
	if rep.ReadErr == nil || rep.Unread != 1 {
		t.Fatalf("an unreadable transcript must be counted as unread, got %v (%d)", rep.ReadErr, rep.Unread)
	}
	if !strings.Contains(rep.ReadErr.Error(), "s.jsonl") {
		t.Errorf("the error must name the file it could not read: %v", rep.ReadErr)
	}
	// And it is still a floor, not a refusal: whatever else was readable is
	// reported, because a partial ledger beats no ledger.
	if err := os.Chmod(f, 0o644); err != nil {
		t.Fatal(err)
	}
	if rep := ScanCosts("", time.Time{}); rep.ReadErr != nil {
		t.Errorf("a readable scan must be clean again: %v", rep.ReadErr)
	}
}

// A transcript ROOT that cannot be read is the ADR's own example. A root
// that does not exist is not that: a machine that never ran the CLI has
// nothing to count, and calling that unreadable would park a fresh
// instance on its first blind pass.
func TestScanCostsUnreadableRootIsNotAQuietDay(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; this needs an unprivileged uid")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	if rep := ScanCosts("", time.Time{}); rep.ReadErr != nil {
		t.Errorf("a machine with no ~/.claude/projects has no records, not a fault: %v", rep.ReadErr)
	}

	claude := filepath.Join(home, ".claude")
	if err := os.MkdirAll(filepath.Join(claude, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(claude, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(claude, 0o755) })
	rep := ScanCosts("", time.Time{})
	if rep.ReadErr == nil {
		t.Fatal("an unreadable transcript root must not read as $0 spent")
	}
}

// Ledger is the degraded pass's receipt: both windows, whichever are armed,
// never the tightest-window summary Line gives the skip report.
func TestBudgetStateLedger(t *testing.T) {
	for _, tc := range []struct {
		st   BudgetState
		want string
	}{
		{BudgetState{PassCap: 30, PassSpend: 8.2, DayCap: 250, DaySpend: 146}, "epoch $8.20/$30.00, day $146.00/$250.00"},
		{BudgetState{DayCap: 250, DaySpend: 146}, "day $146.00/$250.00"},
		{BudgetState{PassCap: 30}, "epoch $0.00/$30.00"},
		{BudgetState{}, "no cap set"},
	} {
		if got := tc.st.Ledger(); got != tc.want {
			t.Errorf("Ledger() = %q, want %q", got, tc.want)
		}
	}
}
