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
//   - The plan guard is OFF — no `plan_guard_<window>:` is set. Nothing on
//     this box is deciding anything on the reading, so no request it costs
//     is one anybody asked for. This is the state the incident was in.
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
// Read on every PlanCache construction, which is once per caller per tick —
// two YamlGets over a file the process has already read. The alternative is
// a cached decision, and a cached decision is one an operator's edit does
// not reach until something restarts: the whole point of the flag is that
// it takes effect on the next tick of a cockpit that is already open.
func (a *App) PlanMeterQuiet(errw io.Writer) *PlanQuiet {
	switch raw := strings.TrimSpace(a.CfgGet("plan_usage_quiet", "")); raw {
	case "":
	case "true":
		return &PlanQuiet{Flag: true}
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
		return &PlanQuiet{}
	}
	return nil
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
	q := a.PlanMeterQuiet(io.Discard)
	if q == nil || !q.Flag {
		return ""
	}
	line := "plan meter QUIET (plan_usage_quiet): no surface is asking the endpoint, and the guard is off for the duration"
	if u, at, ok := a.PlanCache(caller).LastReading(); ok {
		line += fmt.Sprintf(" — last reading %s (%s), %s ago",
			at.UTC().Format("2006-01-02T15:04Z"), u.Line(), BlindFor(now.Sub(at)))
	}
	return line
}
