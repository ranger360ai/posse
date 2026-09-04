package posse

// A meter reader must never be the thing that re-arms the window it is
// waiting out (ranger-base-4rfw1).
//
// MEASURED 2026-09-02. `plan_guard_5h:`/`plan_guard_7d:` were commented out
// and the watch restarted, to drain a 429 window that had been re-arming
// since 03:30Z (ranger-base-uzyd2's quiet gap). The box was silent on the
// usage endpoint for 94 minutes — until the operator opened `posse cockpit`.
// Its plan tick asked the endpoint itself at 20:13:57Z, a second instance
// asked again at 21:15:39Z, both drew `429 Retry-After: 3600`, and the gap
// was over twice without anybody deciding it should be
// ($StateDir/plan-usage.log, caller `cockpit`).
//
// The bug is not in the cockpit. Three surfaces read this meter — the
// cockpit header, `posse cost`, the dispatch guard — and the ones that
// asked did so because nothing between them and the socket knew the shop
// had stopped metering. dispatch.planGuard and GovInputs.planReading each
// check the thresholds for themselves and return early; the cockpit and
// `posse cost` never had that line, and a rule every caller must remember
// is a rule the next caller forgets. So the refusal lives at the choke
// point instead: PlanCache is the one path to the endpoint (rangerhq-tdy8),
// and a quiet cache does not ask, whoever is holding it.
//
// Two things make a cache quiet, and they are the same state seen from two
// sides:
//
//   - The plan guard is OFF — no `plan_guard_<window>:` is set — AND
//     nothing on this box is spending (PlanMeterSpender). Nothing is
//     deciding anything on the reading and nothing is burning the window it
//     measures, so no request it costs is one anybody asked for. This is
//     the state the 09-02 incident was in.
//
//     The second half of that sentence is ranger-base-ddivo's, bought on
//     2026-09-03: without it an unarmed guard muted the METER as well as
//     the guard, and the shop spent two days of `--watch` under dollar caps
//     against a weekly window nobody had read since 09-01. Guard-off is a
//     ruling about the brake; it was never a ruling about the number.
//   - `plan_usage_quiet: true` — the operator's own ruling, which holds
//     with the guard armed. It is the flag the quiet gap actually needed:
//     commenting out two thresholds to stop the polling also switches off
//     the brake, and the operator should not have to trade one for the
//     other.
//
// Quiet is guard-OFF, not guard-blind, everywhere it lands: no clock
// starts, nothing parks, nothing degrades. Blind means "a meter exists and
// could not be read" (ADR 0018 §1) and this is "nobody may ask" — parking a
// fleet on a condition the operator declared would be a brake with no
// release.
//
// What quiet does NOT do is hide the last reading. Every surface still
// serves the snapshot at whatever age it has, and says the age out loud:
// the failure mode of a quiet meter is a stale number read as the present,
// and an age is what stops that.

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// PlanQuiet is the refusal to ask the usage endpoint at all. It is an error
// because Read has to return something when there is no snapshot to serve,
// and it is a TYPE because three callers fork on it — the header renders a
// different sentence, the guard treats it as off rather than blind, and
// `posse cost` exits on it (the rule NoPlanAdapter and NoSource each got a
// type to keep).
type PlanQuiet struct {
	// Flag is true when `plan_usage_quiet:` is what did it, false when the
	// guard is simply unarmed. The two states differ in what an operator
	// would do next, which is the only reason to keep them apart.
	Flag bool
}

func (e *PlanQuiet) Error() string {
	if e.Flag {
		return "plan meter quiet (config plan_usage_quiet: true) — the endpoint is not asked"
	}
	return "plan guard off (no plan_guard_<window>: set) — the endpoint is not asked"
}

// Why is the two words a display has room for.
func (e *PlanQuiet) Why() string {
	if e.Flag {
		return "meter quiet"
	}
	return "guard off"
}

// PlanQuietReason is the *PlanQuiet behind a failed read, or nil.
func PlanQuietReason(err error) *PlanQuiet {
	var q *PlanQuiet
	if err != nil && errors.As(err, &q) {
		return q
	}
	return nil
}

// PlanMeterQuiet is whether this instance may ask the usage endpoint at
// all, and why not. Nil = ask, under the TTL and the cooldown as before.
//
// It is planMeterState's first answer, kept under its own name because five
// callers fork on nothing else (plancache.go Read, dispatch.go, govern.go,
// cockpit.go) and a caller that wants only the verdict should not have to
// name the second value to get it.
func (a *App) PlanMeterQuiet(errw io.Writer) *PlanQuiet {
	q, _ := a.planMeterState(errw)
	return q
}

// planMeterState is the one place the quiet verdict is computed: whether the
// endpoint may be asked, and — when the guard is unarmed — the spender that
// keeps it awake. Both come from one read so no caller can hold two
// opinions.
//
// ONE read is the whole of ranger-base-67mdf. The spend state decays (a
// watch loop can start the instant after PlanMeterSpender returns "") and
// PlanStaleness used to ask it twice: once through this verdict for the
// quiet gate, once again for the sentence. Two reads of a decaying state can
// disagree, and the disagreeing shape is the loud one — let past the gate by
// a spender the second read no longer sees, the line then says "ruling on it
// under the headroom rule" over a box where no rule is running. Two
// questions with one answer have no window between them to disagree in.
//
// Read on every PlanCache construction, which is once per caller per tick —
// two YamlGets over a file the process has already read. The alternative is
// a cached decision, and a cached decision is one an operator's edit does
// not reach until something restarts: the whole point of the flag is that
// it takes effect on the next tick of a cockpit that is already open.
func (a *App) planMeterState(errw io.Writer) (quiet *PlanQuiet, spender string) {
	switch raw := strings.TrimSpace(a.CfgGet("plan_usage_quiet", "")); raw {
	case "":
	case "true":
		return &PlanQuiet{Flag: true}, ""
	case "false":
		// Said explicitly: the guard-off arm below still applies. `false`
		// turns off the flag, not the meter's other reasons for silence.
	default:
		// The plan-guard rule for a malformed setting: a typo is named and
		// the safe reading stands. Safe here is NOT quiet — a value nobody
		// can parse must not be able to switch off the shop's only meter.
		fmt.Fprintf(errw, "plan guard: config plan_usage_quiet: %q is not true or false — the meter stays readable\n", raw)
	}
	if len(a.PlanGuardThresholds(io.Discard)) == 0 {
		// The unarmed arm, and the only one with a second answer: a spender
		// is what keeps the meter awake with no guard on it
		// (ranger-base-ddivo), so it is both the reason this is not quiet
		// and the words the stale line says.
		if s := a.PlanMeterSpender(); s != "" {
			return nil, s
		}
		return &PlanQuiet{}, ""
	}
	return nil, ""
}

// PlanMeterSpender is why this box is SPENDING while its plan guard is
// unarmed, or "" when nothing is. It is the whole of what keeps an unarmed
// guard from muting the meter (ranger-base-ddivo).
//
// MEASURED 2026-09-03. `plan_guard_5h:`/`plan_guard_7d:` had been commented
// out since 09-01 under the operator's full-speed ruling, so every
// PlanCache on the box was quiet by the arm above; `budget_pass:` and
// `budget_day:` were set and `dispatch --watch` hired for two days against
// it. The last reading this box took is stamped 2026-09-01T23:23 local, two
// cockpit opens made no request, and the weekly window was found exhausted
// by hand.
//
// The defect is the COUPLING, not the quiet rule. "Nobody armed the guard,
// so nobody needs the number" is true only when nothing is spending;
// unarming the guard is a ruling about the BRAKE and has never been a
// ruling about the METER, and the two were one sentence. So an unarmed
// guard mutes the guard, and `plan_usage_quiet: true` stays the only full
// mute — that one is a ruling somebody typed.
//
// Two spenders, cheapest question first:
//
//   - A dollar cap is written. ADR 0018's ledger brake knows dollars and
//     knows nothing about the weekly window (ranger-base-c3vqe,
//     ranger-base-wkai3), so a cap is precisely the shape where the meter
//     nobody is reading is the one that runs out first.
//   - A `posse dispatch --watch` loop is running — the unattended hiring
//     the incident spent two days doing. Liveness is the kernel's
//     (WatchLoopRunning's flock, ADR 0011 §1): release IS process death, so
//     there is no staleness class and no pidfile to misread.
//
// A probe that cannot be ANSWERED reads as spending, the same way a
// malformed `plan_usage_quiet:` reads as readable above. Wrong that way
// costs one request per TTL on an idle shop; wrong the other way is this
// bead.
//
// It is a snapshot and it decays — a watch can start the instant after this
// returns "" — and nothing here locks against that. The answer is recomputed
// on every PlanCache construction, so a stale reading costs one skipped
// refresh that the next tick takes, which is the harmless end of the
// check-then-act class rather than a race worth a lock.
//
// What it does NOT do is decide anything. The guard is still off: no
// threshold to trip, no blind clock, no park, and the reading it keeps
// alive joins no comparison it did not join before (budget.go's Dial E
// reads the plan windows only where the guard armed them). This changes who
// ASKS, never what the answer rules on.
func (a *App) PlanMeterSpender() string {
	if a.BudgetCapsConfigured() {
		return "budget_pass:/budget_day: is set"
	}
	running, err := WatchLoopRunning(a)
	if err != nil {
		return "the watch-loop lock could not be read"
	}
	if running {
		return "a dispatch --watch loop is running"
	}
	return ""
}

// PlanQuietLine is the one line that says the meter has been muted on
// purpose, and what the last reading was — or "" where there is nothing to
// say.
//
// It fires only for the FLAG. An unarmed plan guard is the default on most
// shops and has never said anything about a meter, and a permanent line on
// every one of them is furniture — the failure `plan meter BLIND` was
// written in upper case to escape (planstale.go). `plan_usage_quiet: true`
// is different in the way that matters: it is a temporary ruling, it
// disarms a brake the operator has otherwise asked for, and a mute nobody
// can see is exactly what this file's own cooldown ceiling refuses.
//
// Files only. Like PlanStaleness it asks the endpoint nothing — a shop
// check that reported a quiet gap by breaking it would be the joke this
// bead is about.
func (a *App) PlanQuietLine(caller string, now time.Time) string {
	// The cache first, and the verdict off the cache: it carries the one
	// planMeterState this line needs (plancache.go), and asking for the
	// verdict separately would compute it twice for the same tick.
	c := a.PlanCache(caller)
	if c.Quiet == nil || !c.Quiet.Flag {
		return ""
	}
	line := "plan meter QUIET (plan_usage_quiet): no surface is asking the endpoint, and the guard is off for the duration"
	if u, at, ok := c.LastReading(); ok {
		line += fmt.Sprintf(" — last reading %s (%s), %s ago",
			at.UTC().Format("2006-01-02T15:04Z"), u.Line(), BlindFor(now.Sub(at)))
	}
	return line
}
