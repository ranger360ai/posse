package rhq

// Plan-guard overflow (ADR 0010) — a second pool when the guarded window is
// hot.
//
// The plan guard (planusage.go) reads ONE provider's rate windows and, above
// their `plan_guard_<window>:` thresholds, skipped the whole pass. Two things are
// wrong with that once the fleet runs more than one runtime: a lane whose
// runtime is not on that meter was skipped because somebody else's window was
// hot, and a pass that could have run at equal posture on a second pool ran
// nothing at all.
//
// So when the guard trips, the pass runs and the decision moves per bead:
//
//	resolved runtime not on the guarded meter  → launch as today, ungated
//	eligible (§2) and cap not reached (§3)     → launch on `plan_guard_overflow:`
//	otherwise                                  → the guard's skip line, per bead
//
// Two rules keep this from being a way to spend a pool nobody can meter.
// `plan_guard_overflow_cap:` is REQUIRED — an overflow runtime with no cap is
// overflow off, one stderr line, and on-meter beads park. And a *blind* guard
// never overflows (§5): with no reading to judge on, guessing that the other
// pool should pay is exactly the failure the cap exists to bound. Off-meter
// beads still launch through either state (ADR 0013 §3).

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// OverflowWindow is the rolling window the cap counts over. Seven days and
// beads, not dollars: neither candidate pool has a meter posse can read and
// `posse cost` cannot see their spend, so the ledger counts the only unit
// dispatch actually knows it spent. Rolling rather than calendar because a
// weekly pool's reset day is the provider's secret — a rolling window
// upper-bounds every calendar week without knowing it.
const OverflowWindow = 7 * 24 * time.Hour

// GuardedRuntime is the runtime whose plan windows the guard reads. It is
// the default runtime by construction: the meter, the credential and the
// endpoint in planusage.go all belong to that one provider.
const GuardedRuntime = DefaultRuntime

// OnGuardedMeter reports whether a launch on this runtime spends the meter
// the plan guard read (ADR 0010 §1). The other built-ins are their own
// pools and do not. A template-only `runtimes/<name>.yaml` is UNKNOWN, and
// unknown is gated: "this runtime is free" is the expensive guess to get
// wrong, and the operator who knows better has `runtime:` on the PID.
func OnGuardedMeter(name string) bool {
	if name == "" || name == GuardedRuntime {
		return true
	}
	for i := range builtinRuntimes {
		if builtinRuntimes[i].Name == name {
			return false
		}
	}
	return true
}

// Overflow is one pass's overflow configuration. The zero value is off, so
// on-meter beads park on a threshold trip while off-meter beads still run.
type Overflow struct {
	Runtime string
	Cap     int
}

// On reports whether a tripped guard may move anything. A runtime without a
// cap is deliberately NOT on: §3 makes the cap required, because the cap is
// the entire difference between this and draining a weekly pool in an
// afternoon. Neither is the guarded runtime itself on: the move's whole
// premise is that the guard's reading does not apply to the target, and it
// applies to that one by construction.
func (o Overflow) On() bool {
	return o.Runtime != "" && o.Runtime != GuardedRuntime && o.Cap > 0
}

// PlanGuardOverflow reads config `plan_guard_overflow:` (a runtime name) and
// `plan_guard_overflow_cap:` (beads per rolling 7 days). Unset — the default
// — is off and silent. Anything half-configured is named on errw and off:
// a guard that quietly stopped guarding is the failure mode this whole file
// is written against, and so is a pool that quietly started paying.
func (a *App) PlanGuardOverflow(errw io.Writer) Overflow {
	rt := strings.TrimSpace(YamlGet(a.ConfigPath, "plan_guard_overflow"))
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "plan_guard_overflow_cap"))
	if rt == "" {
		if raw != "" {
			fmt.Fprintf(errw, "plan guard: config plan_guard_overflow_cap: %q with no plan_guard_overflow: — overflow off\n", raw)
		}
		return Overflow{}
	}
	// The target has to be a SECOND pool. Naming the guarded runtime spends
	// the hot meter through the ladder built to spare it: the trip reading
	// says this pool is over threshold, and "so use this pool" is not a
	// judgement on it, it is the guard cancelling itself. Off and named,
	// same as any other half-configured overflow.
	if rt == GuardedRuntime {
		fmt.Fprintf(errw, "plan guard: config plan_guard_overflow: %s is the runtime the guard meters — a hot pool cannot be its own overflow — overflow off, on-meter beads park on a tripped guard\n", rt)
		return Overflow{}
	}
	n, err := strconv.Atoi(raw)
	if raw == "" || err != nil || n <= 0 {
		fmt.Fprintf(errw, "plan guard: config plan_guard_overflow: %s needs plan_guard_overflow_cap: N (beads per rolling 7 days, %q is not one) — overflow off, on-meter beads park on a tripped guard\n", rt, raw)
		return Overflow{}
	}
	return Overflow{Runtime: rt, Cap: n}
}

// OverflowLogPath is the ledger: `$StateDir/overflow.log`, append-only, one
// line per overflow launch.
func (a *App) OverflowLogPath() string { return filepath.Join(a.StateDir, "overflow.log") }

// LedgerEntry is one line of a launch ledger: `RFC3339 runtime bead persona`.
// Two ledgers share the shape because they count the same event for two
// different reasons — this one, which beads the plan guard MOVED to a second
// pool (ADR 0010 §3), and uncounted.log, which beads went to a runtime no
// cost adapter reads (ADR 0013 §5). A bead can be both, and then it is on
// both: neither number answers the other's question.
type LedgerEntry struct {
	At      time.Time
	Runtime string
	Bead    string
	Persona string
}

func (e LedgerEntry) line() string {
	return fmt.Sprintf("%s %s %s %s\n", e.At.UTC().Format(time.RFC3339), e.Runtime, e.Bead, e.Persona)
}

// appendLedger records one launch. Append-only and never rotated or pruned
// by posse: it is the only evidence of what a pool with no meter was spent
// on, and the metrics are read off it.
func (a *App) appendLedger(path string, e LedgerEntry) error {
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(e.line()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// countLedger is how many beads went to this runtime inside the window
// ending at now — the number a cap is compared against. Counted per runtime,
// so changing which runtime a cap names does not charge the new pool for the
// old one's week.
//
// A missing ledger is zero, not an error: the first launch creates it. A
// line that does not parse is skipped — the file is ours to write and a
// corrupt line is not a launch anyone can date.
func countLedger(path, runtime string, now time.Time, window time.Duration) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()
	cutoff := now.Add(-window)
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || fields[1] != runtime {
			continue
		}
		at, err := time.Parse(time.RFC3339, fields[0])
		if err != nil || at.Before(cutoff) {
			continue
		}
		n++
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return n, nil
}

// AppendOverflow records one overflow launch (ADR 0010 §3).
func (a *App) AppendOverflow(e LedgerEntry) error {
	return a.appendLedger(a.OverflowLogPath(), e)
}

// OverflowCount is the rolling-window count `plan_guard_overflow_cap:` is
// compared against.
func (a *App) OverflowCount(runtime string, now time.Time) (int, error) {
	return countLedger(a.OverflowLogPath(), runtime, now, OverflowWindow)
}

// overflowDecision is the per-bead ladder's answer for one bead: the runtime
// to launch on, whether that is the overflow pool (so a ledger line is owed
// once the launch succeeds), and — instead of both — the line this bead gets
// in place of a launch.
type overflowDecision struct {
	Runtime string
	Moved   bool
	Skip    string
}

// overflowFor walks ADR 0010 §1's ladder for one bead. rt is the runtime
// this launch would use with no guard in the way; pin names the reason this
// launch's runtime is not dispatch's to change ("" = it is). It is only
// called on a pass whose guard tripped over a threshold — §5 keeps a blind
// guard out of here entirely.
func (d *Dispatcher) overflowFor(is RepoIssue, persona string, ag *AgentFile, rt, tier, pin string) overflowDecision {
	// Rung 1. This launch does not spend the meter the guard read, so that
	// reading says nothing about it: launch as today, ungated. This is the
	// fix in passing — a `runtime: grok` lane was being skipped because a
	// window it cannot touch was hot.
	if !OnGuardedMeter(rt) {
		return overflowDecision{Runtime: rt}
	}
	// On the guarded meter, and something else already decided its runtime:
	// the guard's skip line, exactly as before the overflow existed.
	if pin != "" {
		return overflowDecision{Skip: fmt.Sprintf("%s, %s — skipped", d.planTrip, pin)}
	}
	ov := d.overflow
	if !ov.On() { // planGuard would have skipped the pass; belt and braces
		return overflowDecision{Skip: d.planTrip + " — skipped"}
	}
	skip := func(f string, a ...any) overflowDecision {
		return overflowDecision{Skip: fmt.Sprintf("%s, overflow %s: %s — skipped", d.planTrip, ov.Runtime, fmt.Sprintf(f, a...))}
	}
	// §2(b): judged work never moves. The tier table maps `strong` to
	// "runtime default" on the overflow targets, which is not the model the
	// tier meant — mirrors Dial E, which will not trade judged work either.
	if tier == TierStrong {
		return skip("%s work stays on %s", TierStrong, GuardedRuntime)
	}
	// §2(c): the PID's own opt-out, for lanes that drive through repo shell
	// scripts an overflow target's unattended mode is known to stall. A
	// parity check cannot see that, so the persona says it.
	if ag == nil {
		return skip("no PID for %s to check parity against", persona)
	}
	if ag.NoOverflow {
		return skip("%s says overflow: false", persona)
	}
	// §2(a): parity CLEAN on the target — not "equal to the guarded
	// runtime's". This is dispatch's own choice and dispatch never holds
	// --allow-degraded (the rule Dial E already uses for `fast`), which is
	// what excludes a cage the target cannot nest with no runtime
	// special-casing anywhere.
	ort, err := d.App.LoadRuntime(ov.Runtime)
	if err != nil {
		return skip("%v", err)
	}
	if p := d.App.CheckParityIn(ag, ort, ResolveCage(d.Cage, ag), tier, is.Dir); len(p.Degraded) > 0 {
		return skip("%s", p.Degraded[0])
	}
	// §3: the cap stands in for the meter the pool does not have.
	if d.overflowUsed >= ov.Cap {
		return skip("%d/%d in 7d", d.overflowUsed, ov.Cap)
	}
	return overflowDecision{Runtime: ov.Runtime, Moved: true}
}
