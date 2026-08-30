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
//   - a dispatch pass over the limit is skipped whole, with a witness line
//     naming the load and a second naming who is holding it (below);
//   - no launch — `posse new`, `posse relaunch`, a recipe, a cockpit key —
//     starts a session while the box is over it.
//
// It gates LAUNCHING only. Nothing already running is touched: a saturated
// box needs its sessions to finish, not to be interrupted.

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
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

// ─── who is burning it (ranger-base-0p6x) ───────────────────────────────────
//
// The guard above measures the symptom perfectly and says nothing about the
// cause. On 2026-08-30 it printed nine identical witness lines over 45
// minutes while sixteen orphaned shells burned ~30% CPU each, and the fleet
// sat frozen for 2.5h until the operator ran `ps` by hand at 07:00
// (ranger-base-teau). The culprit line below is that `ps`, run for them.
//
// It obeys the first paragraph of this file. A box that cannot fork cannot
// be asked to fork much, so this is at most ONE `ps`, bounded by a context
// timeout, taken ONLY on a pass the guard is already skipping, and it
// degrades to printing NOTHING at all — on a missing `ps`, a timeout, an
// unparseable table, an idle one. It can neither delay nor fail a pass.
// Fail-open, exactly like the load reading above it.
//
// ppid 1 is the high-signal field and the reason this pays for itself. An
// orphan burning CPU is always a leak and is always safe to name; a busy
// child of a live session is ordinary fleet work, so it is never flagged,
// only listed if it is genuinely one of the top burners. (On darwin a
// launchd-managed agent also shows ppid 1. The flag is a witness on a line
// nobody acts on blind, not a verdict, and the alternative — inferring
// session ancestry from a table taken on a sick box — is the kind of
// cleverness that produced teau's first wrong reading.)

const (
	// loadCulpritTop is how many burners the line names outright. Three
	// fits a log line; the rest are counted, not listed.
	loadCulpritTop = 3
	// LoadCulpritOrphanCPU is the floor for the "N more orphaned
	// processes" tail: an orphan over this much of one core is a leak
	// worth counting, below it is noise on any busy box.
	LoadCulpritOrphanCPU = 20.0
	// loadCulpritTimeout bounds the one fork. Past this the reading is
	// abandoned and the line is not printed: the guard's job is to skip
	// the pass promptly, not to find out who is at fault.
	loadCulpritTimeout = 2 * time.Second
)

// Proc is one row of the box's process table as the culprit reading needs
// it: who, whose child, how much of a core, how long, and what.
type Proc struct {
	PID  int
	PPID int
	CPU  float64       // pcpu: percent of one core, as ps reports it
	Age  time.Duration // etime: elapsed since start
	Comm string        // comm, NOT args — see SysTopCPU
}

// Orphaned is "reparented to init", the leak signal (ppid 1).
func (p Proc) Orphaned() bool { return p.PPID == 1 }

// SysTopCPU reads the box's process table with one bounded `ps`.
//
// The column set `pid=,ppid=,pcpu=,etime=,comm=` is what darwin's ps and
// procps both answer, with every header suppressed by the trailing `=`.
//
// comm, NOT args, and that is a finding rather than a preference: a forked
// subshell carries its PARENT's argv, so reading args named teau's sixteen
// spinners as the gate preamble that had merely spawned them and cost the
// RCA its first hour. If args are ever wanted here they must be
// untruncated, and the RCA says why.
//
// Fields are indexed from the FRONT for a neighbouring reason: only the
// last column may contain spaces, so comm is the whole remainder of the
// line and nothing ahead of it can shift under a field-index that counted
// on a fixed width.
func SysTopCPU() ([]Proc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), loadCulpritTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=,pcpu=,etime=,comm=").Output()
	if err != nil {
		return nil, err
	}
	return parseProcTable(string(out)), nil
}

// parseProcTable turns `ps` output into rows, dropping any line it cannot
// read whole rather than guessing at it. darwin's comm is a full
// executable path and procps' is a bare name; both render as the basename.
func parseProcTable(out string) []Proc {
	var procs []Proc
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		pid, errPID := strconv.Atoi(f[0])
		ppid, errPPID := strconv.Atoi(f[1])
		cpu, errCPU := strconv.ParseFloat(f[2], 64)
		if errPID != nil || errPPID != nil || errCPU != nil {
			continue
		}
		procs = append(procs, Proc{
			PID:  pid,
			PPID: ppid,
			CPU:  cpu,
			Age:  parseEtime(f[3]),
			Comm: filepath.Base(strings.Join(f[4:], " ")),
		})
	}
	return procs
}

// parseEtime reads ps's elapsed time, `[[dd-]hh:]mm:ss`. An age it cannot
// read is 0, which renders as "0s" — an unknown age must not cost the line
// the pid beside it.
func parseEtime(s string) time.Duration {
	var days int
	if i := strings.IndexByte(s, '-'); i >= 0 {
		d, err := strconv.Atoi(s[:i])
		if err != nil || d < 0 {
			return 0
		}
		days, s = d, s[i+1:]
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	secs := 0
	for _, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return 0
		}
		secs = secs*60 + v
	}
	return time.Duration(days)*24*time.Hour + time.Duration(secs)*time.Second
}

// LoadCulpritLine is the second line of a load-guard refusal — the top few
// CPU consumers, orphans flagged — ready to append to whatever sentence the
// caller printed the LoadHigh witness under:
//
//	load guard: top CPU: 36.9% pid 49235 zsh [ORPHANED 2h30m], 31.2% pid 49241 zsh [ORPHANED 2h30m], 4.1% pid 812 node — 13 more orphaned processes over 20% CPU
//
// It carries its own leading newline and indent, and it is "" whenever it
// has nothing honest to say, so a caller appends it unconditionally and
// silence costs an empty string. Callers append it INSIDE a refusal — where
// LoadHigh returned a witness — because that is the only place its fork is
// paid for by a pass that is being skipped anyway.
func (a *App) LoadCulpritLine() string {
	read := a.TopCPU
	if read == nil {
		read = SysTopCPU
	}
	procs, err := read()
	if err != nil {
		return ""
	}
	busy := make([]Proc, 0, len(procs))
	for _, p := range procs {
		if p.CPU > 0 {
			busy = append(busy, p)
		}
	}
	if len(busy) == 0 {
		return ""
	}
	sort.SliceStable(busy, func(i, j int) bool {
		if busy[i].CPU != busy[j].CPU {
			return busy[i].CPU > busy[j].CPU
		}
		return busy[i].PID < busy[j].PID // ties render the same way twice
	})
	named := busy
	if len(named) > loadCulpritTop {
		named = named[:loadCulpritTop]
	}
	shown := make([]string, 0, len(named))
	for _, p := range named {
		one := fmt.Sprintf("%.1f%% pid %d %s", p.CPU, p.PID, p.Comm)
		if p.Orphaned() {
			one += fmt.Sprintf(" [ORPHANED %s]", BlindFor(p.Age))
		}
		shown = append(shown, one)
	}
	line := "load guard: top CPU: " + strings.Join(shown, ", ")
	rest := 0
	for _, p := range busy[len(named):] {
		if p.Orphaned() && p.CPU >= LoadCulpritOrphanCPU {
			rest++
		}
	}
	if rest > 0 {
		plural := "es"
		if rest == 1 {
			plural = ""
		}
		line += fmt.Sprintf(" — %d more orphaned process%s over %g%% CPU", rest, plural, LoadCulpritOrphanCPU)
	}
	return "\n  " + line
}
