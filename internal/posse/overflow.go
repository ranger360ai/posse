package posse

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
//	eligible (§2), the pool's brakes have room → launch on `plan_guard_overflow:`
//	otherwise                                  → the guard's skip line, per bead
//
// Two rules keep this from being a way to spend a pool nobody can meter.
// The move needs an ARMED BRAKE on the target pool (§3, amended 2026-08-29):
// `plan_guard_overflow_cap:`, a bead count standing in for a meter that does
// not exist, or the target's own pool meter fully armed where it does exist —
// either arms it, both apply where both are set, and neither is overflow off,
// one stderr line, on-meter beads park. And a *blind* guard never overflows
// (§5): with no reading to judge on, guessing that the other pool should pay
// is exactly the failure these brakes exist to bound. Off-meter beads still
// launch through either state (ADR 0013 §3).

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
//
// Meter is the target pool's own meter, ARMED — the arming test, taken at
// config time and never a reading (PoolMeterArming). It is the second way to
// arm the move under §3's amendment, and it is carried rather than re-asked
// so one pass decides once.
type Overflow struct {
	Runtime string
	Cap     int
	Meter   bool
}

// On reports whether a tripped guard may move anything. §3's requirement, as
// amended 2026-08-29, is AT LEAST ONE ARMED BRAKE on the target pool: the
// bead cap, or the target's own pool meter. A runtime with neither is
// deliberately not on, because a brake is the entire difference between this
// and draining a weekly pool in an afternoon. Neither is the guarded runtime
// itself on: the move's whole premise is that the guard's reading does not
// apply to the target, and it applies to that one by construction.
func (o Overflow) On() bool {
	return o.Runtime != "" && o.Runtime != GuardedRuntime && (o.Cap > 0 || o.Meter)
}

// Capped reports whether the bead cap is one of the armed brakes — i.e.
// whether the ledger's rolling count is compared against anything. Overflow
// armed by the meter alone still WRITES the ledger on every move (§3: it
// feeds the metric, and a cap set later), it just has no number to park on.
func (o Overflow) Capped() bool { return o.Cap > 0 }

// PlanGuardOverflow reads config `plan_guard_overflow:` (a runtime name) and
// `plan_guard_overflow_cap:` (beads per rolling 7 days), and asks the target
// pool whether it has a meter of its own that is armed. Unset — the default
// — is off and silent. Anything half-configured is named on errw and off:
// a guard that quietly stopped guarding is the failure mode this whole file
// is written against, and so is a pool that quietly started paying.
//
// §3 as amended 2026-08-29: the requirement is AT LEAST ONE ARMED BRAKE on
// the target pool, not the cap specifically. Why the meter alone suffices
// for overflow — the thing it did not suffice for when this was written —
// is that every overflow launch spends the pool from THIS box, so a meter
// reading local transcripts sees all of the drain overflow itself causes;
// the floor's blind spot is other people's spend, which the threshold is
// sized for. Where both are set both apply, and no warning is printed for
// it: an operator who set both meant both (§3's rejected alternative), and
// the two fail differently — a bead count needs no calibration, a
// percentage needs the factor.
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
	// The arming test, never a reading: no transcript is scanned to decide
	// whether a brake exists, and none is scanned at all until a bead §2
	// would move actually asks the pool how full it is (overflowFor).
	meter, meterOff := a.PoolMeterArming(rt)
	n, err := strconv.Atoi(raw)
	if raw != "" && err == nil && n > 0 {
		return Overflow{Runtime: rt, Cap: n, Meter: meter}
	}
	if meter {
		// A cap that is SET and unusable is a typo, and a typo must be
		// visible whatever else is holding the pool — the alternative is a
		// brake the operator believes in and does not have. Overflow stays
		// armed on the brake that IS there, and the line says which.
		if raw != "" {
			fmt.Fprintf(errw, "plan guard: config plan_guard_overflow_cap: %q is not a bead count (N per rolling 7 days) — that cap is off; overflow stays armed on %s's own pool meter\n", raw, rt)
		}
		return Overflow{Runtime: rt, Meter: true}
	}
	// Neither brake. The line names BOTH ways to arm the move, because an
	// operator who reads only "set a cap" cannot tell that the pool they
	// pointed at has a meter that would have done — and where the target's
	// meter is half-configured, which input is missing, since that is a
	// three-key meter and the one that is set says nothing about the others.
	var alt string
	switch {
	case rt != GrokPoolRuntime:
		alt = fmt.Sprintf("; %s has no pool meter posse can read, so the cap is its only brake", rt)
	case meterOff != "":
		alt = fmt.Sprintf(", or %s's own pool meter fully armed — %s", rt, meterOff)
	default:
		alt = fmt.Sprintf(", or %s's own pool meter fully armed (grok_guard_week: + grok_pool_reset: + grok_pool_usd_per_point:)", rt)
	}
	fmt.Fprintf(errw, "plan guard: config plan_guard_overflow: %s needs an armed brake on the target pool: plan_guard_overflow_cap: N (beads per rolling 7 days, %q is not one)%s — overflow off, on-meter beads park on a tripped guard\n", rt, raw, alt)
	return Overflow{}
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
// A missing ledger is zero, not an error: the first launch creates it.
//
// A line that is not a ledger entry is an ERROR, not a skip (ranger-base-lasj).
// Skipping it reads as "that was not a launch", and the one thing a torn or
// hand-edited line is not is evidence that nothing was launched — it is a
// launch nobody can date, so the week's total is unknown. Both callers already
// fail closed on an unreadable ledger (overThreshold, uncountedSkip), which is
// the honest answer here too: an unknown count is not a licence to spend a pool
// with no meter. Whole-blank lines are the one exception: appendLedger writes a
// newline-terminated line in one call, so a torn write leaves a prefix and
// never an empty line, and an empty line carries no record to lose.
//
// The shape is the one appendLedger writes — RFC3339, runtime, bead, persona —
// so a short line is a truncated one and counts as corrupt on the same reading.
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
	ln := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		ln++
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 4 {
			return 0, fmt.Errorf("line %d is not a ledger entry (%d fields, want %s runtime bead persona)", ln, len(fields), time.RFC3339)
		}
		at, err := time.Parse(time.RFC3339, fields[0])
		if err != nil {
			return 0, fmt.Errorf("line %d is not dated: %v", ln, err)
		}
		if fields[1] != runtime || at.Before(cutoff) {
			continue
		}
		n++
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return n, nil
}

// ledgerAppendable reports whether appendLedger could write to path right
// now, and writes nothing to it either way. It is the precondition for
// spending a pool the ledger is the only record of: a cap counted off a file
// nothing can be added to is a cap of zero that reads as room forever.
//
// The probe is an OPEN, never the mode bits: a 0444 file is a promise about
// a uid, and root — or an ACL — defeats it in both directions
// (ranger-base-c00). O_CREATE is deliberately absent so a pass that overflows
// nothing leaves no ledger behind; when there is no ledger yet, what has to
// be writable is the directory the first append will create it in, so that is
// what gets opened instead, and the probe file is taken away again.
func (a *App) ledgerAppendable(path string) error {
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		return f.Close()
	}
	if !os.IsNotExist(err) {
		return err
	}
	probe, err := os.CreateTemp(a.StateDir, ".ledger-probe-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Remove(name)
}

// AppendOverflow records one overflow launch (ADR 0010 §3).
func (a *App) AppendOverflow(e LedgerEntry) error {
	return a.appendLedger(a.OverflowLogPath(), e)
}

// OverflowAppendable is the ledger-writability half of the cap's reading:
// whether the line this pass would owe for a move can be written at all.
func (a *App) OverflowAppendable() error {
	return a.ledgerAppendable(a.OverflowLogPath())
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
	// SkipKind is the refill class Skip is counted under, "" meaning the
	// ladder's own. Only the target pool's meter differs: that park is the
	// pool's brake, not the plan guard's, and it counts where grokPoolSkip's
	// other caller counts it.
	SkipKind string
}

// kind is the class this decision's skip is counted under in a refill.
func (o overflowDecision) kind() string {
	if o.SkipKind != "" {
		return o.SkipKind
	}
	return skipPlanGuard
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
	// §3, the ordering, ratified 2026-08-29 and realised here (ranger-base-v62hj):
	// where the target pool HAS a reading, the reading is named first and the
	// bead cap is the stand-in for the *absence* of one. The meter belongs at
	// the move rather than after it, for two reasons. It decides whether this
	// bead may move at all, so it is a rung of this ladder like the ones
	// above. And the cap's line is a calibration prompt — §3 says raise the
	// cap "only on that evidence", the pool's own usage — so the operator who
	// reads "2/2 in 7d" must already have been told the pool is at 80%,
	// which the shipped ordering hid on exactly that line.
	//
	// Placed after §2 deliberately: the reading is a transcript scan, taken
	// lazily and memoised by grokPoolGuard, and a bead §2 already refused is
	// no candidate for the pool — a pass whose only on-meter beads are
	// ineligible prints no "! grok pool:" line at all.
	//
	// Keyed on the one runtime that has a meter; ADR 0010 §3's tripwire says
	// a registry waits for a second one.
	if line := d.grokPoolSkip(ov.Runtime); line != "" {
		return overflowDecision{Skip: line, SkipKind: skipRuntimeCap}
	}
	// §3: the cap stands in for the meter the pool does not have — so where
	// the pool DOES have one and the operator set no cap, there is nothing
	// here to stand in for and nothing to compare against. What armed the
	// move then is the meter, and what brakes it is the reading taken
	// immediately above; the ledger line is still owed and still written on
	// every move, it is simply not a brake.
	if ov.Capped() && d.overflowUsed >= ov.Cap {
		return skip("%d/%d in 7d", d.overflowUsed, ov.Cap)
	}
	return overflowDecision{Runtime: ov.Runtime, Moved: true}
}

// readOverflowCount is the rolling-window reading with its fail-closed half
// attached. An unreadable ledger is not a licence to spend a pool with no
// meter: overflow goes off for the rest of the pass — the pre-overflow
// behaviour, which costs a skipped pass and heals itself, rather than an
// uncounted week. ok is false exactly when that happened.
//
// An UNWRITABLE one is the same refusal for a sharper reason (ranger-base-2y96).
// Reading is only half of what the cap needs; the other half is the line the
// move owes afterwards. A ledger that can be read but not appended to counts
// every pass at whatever it already says — zero, for an empty one — so cap 1
// admits one launch per pass forever and records none of them. The count is
// the number, but the append is what makes the number mean anything, so both
// are checked before the ladder is allowed to spend, not after.
//
// What this pass launched but could not record is added back: the file will
// under-count those beads for as long as it exists, so a re-read that trusted
// it alone would hand their room out twice (ranger-base-af98). The probe does
// not retire that arithmetic — it cannot see a write that will fail for a
// reason an open does not, a full disk being the obvious one — it only stops
// the case where the failure was knowable before anything was spent.
//
// Order matters: the count is taken first, so a ledger that is both corrupt
// and unwritable is named by the fault an operator has to fix first.
func (d *Dispatcher) readOverflowCount() (int, bool) {
	n, err := d.App.OverflowCount(d.overflow.Runtime, d.now())
	if err != nil {
		left := d.dropCap()
		d.eprintf("plan guard: overflow ledger %s unreadable (%v) — %s\n",
			AbbrevHome(d.App.OverflowLogPath()), err, left)
		return 0, false
	}
	if err := d.App.OverflowAppendable(); err != nil {
		left := d.dropCap()
		d.eprintf("plan guard: overflow ledger %s cannot be appended to (%v) — %s\n",
			AbbrevHome(d.App.OverflowLogPath()), err, left)
		return 0, false
	}
	return n + d.overflowUnlogged, true
}

// dropCap disarms the bead cap for the rest of this pass and returns the
// clause that says what that leaves, for the line naming the fault.
//
// A ledger fault is a fault in the CAP's instrument, and §3's rule is at
// least one armed brake on the target pool. Where the pool's own meter is
// armed, the brake that armed the move is a config key and a transcript
// scan: neither is touched by a file this pass cannot read, and the ledger
// is no longer the only record of what the pool was spent on — the meter
// reads that spend at the source. So the cap goes and overflow stays.
//
// Where the meter is not armed, this is the pre-existing refusal unchanged
// (ranger-base-2y96, ranger-base-af98): an unknown or unrecordable count is
// not a licence to spend a pool with no meter, and overflow goes off for the
// pass — the pre-overflow behaviour, which costs a skipped pass and heals
// itself, rather than an uncounted week.
func (d *Dispatcher) dropCap() string {
	d.overflow.Cap = 0
	if d.overflow.On() {
		return fmt.Sprintf("the bead cap is off for this pass; overflow stays armed on %s's own pool meter, which reads the spend at the source", d.overflow.Runtime)
	}
	d.overflow = Overflow{}
	return "overflow off this pass, on-meter beads park"
}

// refreshOverflowUsed re-takes the cap's reading inside the launcher critical
// section (ADR 0011 §1), and it is what makes `plan_guard_overflow_cap:` a cap
// on the POOL rather than one per concurrent dispatcher (ranger-base-af98).
//
// The count and the launch are a check-then-act pair against overflow.log, a
// file every launcher sharing this StateDir appends to. overThreshold takes
// its reading when the guard trips, which is before this loop's lock and
// therefore before any other launcher has had to finish: two passes that trip
// together both read 0/1, both queue on the flock, and both then act on a
// number the first one has already spent. Reading again here — after the wait,
// where nobody else may be launching — is the whole fix, because the first
// holder's ledger line is on disk by the time the second holder gets in.
//
// It is a no-op unless the guard tripped with overflow configured; nothing
// else reads the ledger, and a pass under threshold must not touch it. It runs
// per fireLoop call rather than per Run, so a refire (ADR 0028 §2) re-reads
// too: it holds the lock separately, so it can go stale separately.
func (d *Dispatcher) refreshOverflowUsed() {
	// Capped, not On: with the meter as the only armed brake the count is a
	// metric and not a number anything is compared against, so re-reading it
	// under the lock would buy nothing and cost a scan. The meter itself is
	// taken per bead inside the critical section already.
	if d.planTrip == "" || !d.overflow.Capped() {
		return
	}
	was := d.overflowUsed
	n, ok := d.readOverflowCount()
	if !ok {
		return
	}
	d.overflowUsed = n
	if n != was {
		// Somebody else's launches, or entries aging out of the rolling
		// window while this pass waited. Either way the pass reported one
		// number and is acting on another, and unexplained is the one thing
		// a cap line must never be.
		d.printf("plan guard: overflow %s now %d/%d in 7d, read under the launcher lock (this pass reported %d)\n",
			d.overflow.Runtime, n, d.overflow.Cap, was)
	}
}
