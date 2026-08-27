package rhq

// Dial E (ADR 0003 §4) — budget caps and step-down.
//
// Two caps in API-equivalent dollars, `budget_pass:` and `budget_day:`.
// **Both unset by default**, and then nothing here runs at all: no
// transcript scan, no arithmetic, exactly today's behaviour. That dormancy
// is the point — the operator sets values after a week of `posse cost`
// numbers (the ADR's starting point: $25/pass, $100/day).
//
// With a cap set, dispatch computes the utilization of the *tightest*
// window before each launch and:
//
//   ≥ 80%  a session whose tier resolved to `standard` by default runs at
//          `fast` instead — parity permitting. `strong` never moves: the
//          ADR's Dial E is option (b), and quality of judged work is never
//          traded silently. Mechanical work slows first.
//   ≥ 100% dispatch stops, one clear line per bead it did not launch.
//
// The windows: `pass` is the spend of the beads *this pass* dispatched,
// measured from the moment the pass began (a fired bead is already burning
// tokens while the next one launches, so this grows within a pass and is
// not merely last pass's number); `day` is the local calendar day's bead
// spend, the same total the cockpit shows. Interactive sessions are in
// neither — Dial G keeps them visible and ungated.
//
// When the plan-utilization guard (rangerhq-jgm) has already read the
// plan's rate windows this pass, those percentages join the comparison: the
// plan window is the realer budget, and stepping down at 80% of it is the
// soft landing before the guard's hard skip. Nothing extra is fetched for
// this — no thresholds configured means no reading, and no caps set means
// Dial E is dormant whatever the plan says.
//
// Which windows those are is the provider adapter's business and never this
// file's (ADR 0012 D4). A window arrives named, and every window is the
// same thing to the arithmetic here whatever its provider calls it: a
// percentage with a label on it.

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// The two rungs of Dial E, as percentages of the tightest window.
const (
	BudgetStepDownPct = 80.0
	BudgetStopPct     = 100.0
)

// BudgetCaps reads config `budget_pass:` / `budget_day:` in API-equivalent
// dollars (a leading `$` is allowed). Unset — the default — is no cap. A
// value that is not a positive number is named on errw and treated as
// unset: a typo must be visible, not a silently disabled cap.
func (a *App) BudgetCaps(errw io.Writer) (pass, day float64) {
	return a.budgetDollars("budget_pass", errw), a.budgetDollars("budget_day", errw)
}

func (a *App) budgetDollars(key string, errw io.Writer) float64 {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, key))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimPrefix(raw, "$"), 64)
	if err != nil || v <= 0 {
		fmt.Fprintf(errw, "budget: config %s: %q is not a positive dollar amount — no cap for that window\n", key, raw)
		return 0
	}
	return v
}

// BudgetState is Dial E's view of the windows at one moment: the caps, what
// has been spent against them, and the tightest one — which is the only one
// the dials read.
type BudgetState struct {
	PassCap, DayCap     float64
	PassSpend, DaySpend float64
	// Plan is the plan-window reading this pass took, as its adapter named
	// it; nil = the guard was not consulted. Any number of windows.
	Plan PlanUsage

	Pct    float64 // the tightest window's utilization
	Window string  // which window that was ("" when no cap is set)

	// Unreadable is the cost scan's read failure, when it had one (ADR 0018
	// §3): part of the ledger could not be read, so PassSpend/DaySpend are a
	// FLOOR and not a total. The rungs below deliberately ignore it — a
	// floor over the cap is still over the cap — but a caller about to trust
	// this state as the only brake must not treat an uncountable ledger as
	// $0 spent. "An unreadable ledger is not a licence to spend."
	Unreadable error
}

// Set reports whether Dial E is armed at all.
func (b BudgetState) Set() bool { return b.PassCap > 0 || b.DayCap > 0 }

// Stop: the window is spent — dispatch launches nothing more this pass.
func (b BudgetState) Stop() bool { return b.Set() && b.Pct >= BudgetStopPct }

// StepDown: the window is nearly spent — standard-by-default drops to fast.
func (b BudgetState) StepDown() bool { return b.Set() && b.Pct >= BudgetStepDownPct }

// resolve picks the tightest window. Plan windows are already percentages;
// they count only when the guard read them this pass.
func (b *BudgetState) resolve() {
	b.Pct, b.Window = 0, ""
	consider := func(pct float64, w string) {
		if w != "" && pct > b.Pct {
			b.Pct, b.Window = pct, w
		}
	}
	if b.PassCap > 0 {
		consider(100*b.PassSpend/b.PassCap, "pass")
	}
	if b.DayCap > 0 {
		consider(100*b.DaySpend/b.DayCap, "day")
	}
	// In the adapter's order, though order does not decide anything here —
	// consider takes the largest, and a tie keeps the first, which is the
	// tighter window by the adapter's own reckoning.
	for _, w := range b.Plan {
		if w.Pct > 0 {
			consider(w.Pct, "plan "+w.Name)
		}
	}
}

// Short names the driving window and its utilization: "day 84%".
func (b BudgetState) Short() string {
	if b.Window == "" {
		return "no cap"
	}
	return fmt.Sprintf("%s %.0f%%", b.Window, b.Pct)
}

// Line is Short plus the numbers behind it, for the skipped-bead report.
func (b BudgetState) Line() string {
	switch b.Window {
	case "pass":
		return fmt.Sprintf("pass $%.2f of $%.2f (%.0f%%)", b.PassSpend, b.PassCap, b.Pct)
	case "day":
		return fmt.Sprintf("day $%.2f of $%.2f (%.0f%%)", b.DaySpend, b.DayCap, b.Pct)
	case "":
		return "no cap set"
	default: // a plan window — a percentage is all a usage endpoint gives
		return fmt.Sprintf("%s at %.0f%%", b.Window, b.Pct)
	}
}

// Ledger names both cap windows and what has been spent against them —
// "pass $8.20/$30, day $146/$250". Unlike Line it does not pick the
// tightest window: this is the receipt a degraded pass prints (ADR 0018
// §1), and the operator reading it wants every number the brake has.
func (b BudgetState) Ledger() string {
	var parts []string
	if b.PassCap > 0 {
		parts = append(parts, fmt.Sprintf("pass $%.2f/$%.2f", b.PassSpend, b.PassCap))
	}
	if b.DayCap > 0 {
		parts = append(parts, fmt.Sprintf("day $%.2f/$%.2f", b.DaySpend, b.DayCap))
	}
	if len(parts) == 0 {
		return "no cap set"
	}
	return strings.Join(parts, ", ")
}

// startOfDay is the local midnight that opens now's calendar day — the
// earliest transcript record the day window can be made of.
func startOfDay(now time.Time) time.Time {
	y, m, d := now.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}
