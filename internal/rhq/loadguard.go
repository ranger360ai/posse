package rhq

// The load guard (ranger-base-innx).
//
// A box whose load average is far above its core count cannot schedule
// fork(). The failure that produces is unusually nasty and unusually quiet:
// every command a session tries to spawn hangs with no output at all, while
// anything in-process carries on working, so the shop looks alive and is
// not. Process limits, ptys and disk are all fine while it happens, which is
// exactly why the elimination tree wastes hours before anyone reads the one
// cheap number that names it.
//
// So this is a belt and it is honest about being one. Load a bug inside
// posse generates is not fixed by declining to dispatch — only by fixing the
// bug. What the guard earns is the load posse does NOT control, an OS update
// storm or a neighbour build, where launching a session into a box that
// cannot fork it is strictly worse than waiting.
//
// Two rules follow, and they are why the reading is taken in two places:
//   - a dispatch pass over the limit is skipped whole, with one witness line;
//   - no launch — `posse new`, `posse relaunch`, a recipe, a cockpit key —
//     starts a session while the box is over it.
//
// It gates LAUNCHING only. Nothing already running is touched: a saturated
// box needs its sessions to finish, not to be interrupted.

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// LoadGuardDefault is the 1-minute load average above which nothing new is
// launched, absent config.
//
// It assumes a machine of roughly eight cores: several times what a busy
// fleet costs there, and well under what a box in fork starvation shows.
// Load is NOT normalised by core count, and neither is this number — on
// hardware that is not roughly that size, set `load_guard:` from your own
// quiet baseline rather than inheriting this one. The shape to copy is
// "several times the quiet number, well under the broken one".
const LoadGuardDefault = 25.0

// LoadGuard reads config `load_guard:` — the 1-minute load average above
// which this instance launches nothing. Unset = LoadGuardDefault. **0 is
// the operator's escape hatch**: guard off, launch into anything, which is
// pre-innx behaviour. A value that is not a non-negative number is named on
// errw and the default stands, the house rule for a malformed ceiling: a
// typo must be visible, and here the visible failure is the safe one.
func (a *App) LoadGuard(errw io.Writer) float64 {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "load_guard"))
	if raw == "" {
		return LoadGuardDefault
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 {
		fmt.Fprintf(errw, "load guard: config load_guard: %q is not a load average (a number, or 0 to disable) — using %g\n",
			raw, LoadGuardDefault)
		return LoadGuardDefault
	}
	return v
}

// SysLoad1 is the box's 1-minute load average, read without forking.
func SysLoad1() (float64, error) { return sysLoad1() }

// LoadHigh returns the witness half of a refusal — "load guard: 1-min
// loadavg 112.34 is over load_guard: 25" — when the box is too loaded to
// launch into, and "" when it is not. The caller supplies what it is
// declining to do, so the pass and the launch print the same measurement
// under two different sentences.
//
// It fails OPEN: a reading it cannot take is named on errw and gates
// nothing. A monitoring failure must not be able to stop the shop, and this
// guard is a belt over ceilings — `budget_*`, `plan_guard_*` — that are
// still counting.
func (a *App) LoadHigh(errw io.Writer) string {
	limit := a.LoadGuard(errw)
	if limit <= 0 {
		return ""
	}
	read := a.Load1
	if read == nil {
		read = SysLoad1
	}
	load, err := read()
	if err != nil {
		fmt.Fprintf(errw, "load guard: %v — load not gated this time\n", err)
		return ""
	}
	if load <= limit {
		return ""
	}
	return fmt.Sprintf("load guard: 1-min loadavg %.2f is over load_guard: %g", load, limit)
}
