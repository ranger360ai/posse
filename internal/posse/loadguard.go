package posse

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
//     naming the load, a second naming who is holding it, and — when the
//     holders are posse's own leaked gate-shell children — a third naming
//     those one by one (below);
//   - no launch — `posse new`, `posse relaunch`, a recipe, a cockpit key —
//     starts a session while the box is over it.
//
// It gates LAUNCHING, and NOTHING A SESSION OWNS IS EVER TOUCHED: a
// saturated box needs its sessions to finish, not to be interrupted.
//
// The one exception is a process no session owns any more. Arm 1 of the
// orphan report (ranger-base-apwr) names leaked gate-shell children and
// kills none of them; arm 2 (ranger-base-gvp2p, loadguardkill.go) ends them,
// where the operator has armed it with `load_guard_kill: true` and where the
// process carries no declare-or-die marker. Everything else about the
// paragraph above still holds: an orphan is by definition reparented to
// init, so ending one interrupts no session, and a persona's DELIBERATE
// long-lived background process is spared by the marker it wrote — which is
// the explicit exception this comment used to say the operator owed us, and
// he ruled on it on 2026-08-31.

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
	// loadCulpritTimeout bounds the census. Past this the reading is
	// abandoned and the line is not printed: the guard's job is to skip
	// the pass promptly, not to find out who is at fault. It bounds BOTH
	// of the census's reads together (SysTopCPU), not each of them, so
	// adding the second one cannot lengthen the worst case.
	loadCulpritTimeout = 2 * time.Second

	// LoadOrphanMinAge is the age floor of the orphan predicate
	// (ranger-base-apwr). It is there to exclude the transient rather than
	// to measure anything: a child is briefly ppid 1 while the session
	// that forked it tears down, and the teau RCA measured a last-forked
	// pid outliving its own fork/exec window by about a second. A minute
	// clears both by a wide margin, and it is far under the age of
	// anything this report can be about — the guard only reads on a pass
	// it is already skipping, and teau's sixteen were 2h30m old when a
	// human finally found them.
	LoadOrphanMinAge = time.Minute
	// loadOrphanTop is how many orphans the report names outright, for the
	// reason loadCulpritTop exists: sixteen leaks are sixteen lines of the
	// same thing, and the rest are counted.
	loadOrphanTop = 3
	// loadOrphanPayload bounds the shown command. The point is to say what
	// the process WAS, not to reproduce it; the transcript has the rest.
	loadOrphanPayload = 90
)

// Proc is one row of the box's process table as the culprit reading needs
// it: who, whose child, how much of a core, how long, and what.
type Proc struct {
	PID  int
	PPID int
	CPU  float64       // pcpu: percent of one core, as ps reports it
	Age  time.Duration // etime: elapsed since start
	Comm string        // comm, NOT args — see SysTopCPU
	// Args is the UNTRUNCATED argv, and it is filled only for the rows the
	// orphan report could be about (orphanSuspect) — never for a row the
	// culprit line renders, which reads Comm and must go on reading Comm.
	// Empty is the normal case and the honest one: on a healthy box no row
	// qualifies and the census does not take the second read at all.
	Args string
}

// orphanSuspect is the half of the orphan predicate that the census's first
// `ps` can answer: reparented to init, burning real CPU, and old enough not
// to be a teardown or a fork/exec window. It is what decides whether the
// argv read below is taken at all, and the report then adds the half that
// needs argv — that the process came out of one of our gated commands.
//
// An orphan burning CPU is ALWAYS a leak by itself; what the argv adds is
// that it is OURS rather than the operator's (ranger-base-apwr).
func (p Proc) orphanSuspect() bool {
	return p.Orphaned() && p.CPU >= LoadCulpritOrphanCPU && p.Age >= LoadOrphanMinAge
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
//
// WHY THERE ARE TWO READS AND NOT ONE (ranger-base-apwr). The orphan report
// needs the UNTRUNCATED argv as well as comm, and darwin's ps cannot hand
// over both in one call: every column but the LAST is cut to a fixed width,
// measured on darwin 25.4.0 as 16 characters for comm and 63 for args, and
// `-ww` does not lift it — `pid=,comm=,args=` renders
// "/usr/libexec/com /usr/libexec/com.apple.cmio.videodriverkithostextension…",
// whose comm no longer has a basename to take. So one column set cannot
// serve both consumers, and the census takes a SECOND, pid-scoped read
// where args IS last and is therefore whole.
//
// That second read costs nothing on a healthy box: fillOrphanArgs only runs
// when the first read already found a reparented process burning CPU, which
// on every box that is not wedged is no rows and no fork. It shares this
// function's one deadline, so two reads cannot outlast what one was allowed,
// and it fails open to no argv at all — which renders no orphan report and
// leaves the culprit line exactly as ranger-base-0p6x shipped it.
func SysTopCPU() ([]Proc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), loadCulpritTimeout)
	defer cancel()
	procs, err := sysProcTable(ctx)
	if err != nil {
		return nil, err
	}
	fillOrphanArgs(ctx, procs)
	return procs, nil
}

// sysProcTable is the census's first `ps` — see SysTopCPU for why the
// columns are what they are. Shared with SysSelfOrphans (ranger-base-6mhxw),
// which needs the same table under a different predicate for which rows get
// their argv read.
func sysProcTable(ctx context.Context) ([]Proc, error) {
	out, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=,pcpu=,etime=,comm=").Output()
	if err != nil {
		return nil, err
	}
	return parseProcTable(string(out)), nil
}

// fillOrphanArgs reads the untruncated argv of the census rows the orphan
// report could be about, and of no others. It reports nothing: an argv it
// could not get is an empty Args, which the report reads as "not shown to be
// ours" and skips. The guard exists for a box that cannot fork, so this must
// never be able to fail or delay a pass — it takes the caller's deadline and
// every failure is silence.
func fillOrphanArgs(ctx context.Context, procs []Proc) {
	fillArgsForPIDs(ctx, procs, orphanSuspectPIDs(procs))
}

// fillArgsForPIDs reads the untruncated argv of exactly the given pids and
// writes it back onto the matching rows of procs — the half of the census
// that fillOrphanArgs (the load guard's own CPU-gated suspects) and
// SysSelfOrphans (no CPU floor: a leak that never uses much of a core is
// still a leak, ranger-base-6mhxw) share. It reports nothing: a read it
// could not take leaves those rows' Args empty, same as before the call.
func fillArgsForPIDs(ctx context.Context, procs []Proc, ids []string) {
	if len(ids) == 0 {
		return
	}
	// args is the only column and so the last one: whole on both platforms,
	// with -ww asking procps not to cut it to a screen width either.
	out, err := exec.CommandContext(ctx, "ps", "-ww", "-o", "pid=,args=", "-p", strings.Join(ids, ",")).Output()
	if err != nil {
		return
	}
	args := parseArgsTable(string(out))
	for i := range procs {
		if a, ok := args[procs[i].PID]; ok {
			procs[i].Args = a
		}
	}
}

// orphanSuspectPIDs is the scope of the census's second read, as ps wants it.
// It is a function of its own because "only the suspects" is the property
// that keeps the extra fork off a healthy box, and a property worth having
// is worth being able to pin.
func orphanSuspectPIDs(procs []Proc) []string {
	var ids []string
	for _, p := range procs {
		if p.orphanSuspect() {
			ids = append(ids, strconv.Itoa(p.PID))
		}
	}
	return ids
}

// parseArgsTable reads `pid=,args=` — pid, then the whole rest of the line,
// which is argv space-joined and may hold anything a persona typed. Lines it
// cannot read are dropped rather than guessed at, exactly as parseProcTable
// drops them.
func parseArgsTable(out string) map[int]string {
	args := map[int]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimLeft(line, " ")
		sp := strings.IndexByte(line, ' ')
		if sp < 0 {
			continue
		}
		pid, err := strconv.Atoi(line[:sp])
		if err != nil {
			continue
		}
		args[pid] = strings.TrimSpace(line[sp+1:])
	}
	return args
}

// gateShellForkPayload says whether an argv came out of one of our gated
// commands, and hands back the persona's own command with our preamble taken
// off the front.
//
// A forked subshell never execs, so it carries its parent's whole -c string
// — the ADR 0009 preamble included. That is what makes this test cheap and
// what makes it OURS: a busy orphan is always a leak, and the preamble is
// what says the leak came from a persona Bash line rather than from
// something the operator started (ranger-base-apwr).
//
// "OPENS with" is load-bearing and is enforced. The preamble must be the
// head of a command string — the whole of the argv, or whatever follows the
// shell's own `-c `. A preamble found deeper in a line is some other
// process's text ABOUT ours (an editor, a grep, a `ps` of this very report),
// and calling that a leak would be the teau misreading in a new costume.
func gateShellForkPayload(args string) (string, bool) {
	i := strings.Index(args, gateShellPreambleHead)
	if i < 0 {
		return "", false
	}
	if i != 0 && !strings.HasSuffix(args[:i], "-c ") {
		return "", false
	}
	rest := args[i+len(gateShellPreambleHead):]
	j := strings.Index(rest, gateShellPreambleTail)
	if j < 0 {
		return "", true // ours, but we cannot see where our own text ends
	}
	return strings.TrimSpace(rest[j+len(gateShellPreambleTail):]), true
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
	return "\n  " + line + a.orphanReport(busy)
}

// leakRow is one leaked gate-shell child as the report has it: the census
// row, the persona's own command with our preamble taken off, and — when
// the declare-or-die marker was on the line — the reason it named.
type leakRow struct {
	p       Proc
	payload string
	reason  string
}

// orphanReport is the leaked gate-shell children this box is holding, named.
// It rides under the culprit line, off the same single census.
//
// ARM 1 (ranger-base-apwr) KILLS NOTHING and is still the shipped default.
// The report-only floor was the operator's call to lift, not ours: a persona
// that deliberately starts a long-lived process with a trailing ampersand
// and lets the tool call return produces exactly this signature, so a reaper
// that guessed would eventually kill something somebody meant. What arm 1
// buys is the 2.5 hours teau spent with sixteen of these visible to any `ps`
// and named by nothing: the wedge becomes loud without anything becoming
// destroyable.
//
// ARM 2 (ranger-base-gvp2p, loadguardkill.go) is that lift, ruled on
// 2026-08-31: a process is spared when it carries the declare-or-die marker
// and ended when it does not — but only where `load_guard_kill: true` says
// so, and the shipped default is false. Read loadguardkill.go for the
// marker, the measured signal and the two directions this pair of predicates
// is allowed to be wrong in.
//
// The DECLARATION is honoured in both modes and the sparing is printed in
// both. While the arm is off that is not decoration: the ruling's first bar
// for the live flip is arm-1 field data with no false positive in it, and a
// declared process reported as a leak would be precisely that false
// positive.
//
// It is "" whenever it has nothing to say, which is every healthy box and
// every box whose orphans are not ours.
func (a *App) orphanReport(busy []Proc) string {
	var leaks, spared []leakRow
	for _, p := range busy {
		if !p.orphanSuspect() {
			continue
		}
		payload, ours := gateShellForkPayload(p.Args)
		if !ours {
			continue
		}
		if reason, declared := orphanDeclared(p.Args); declared {
			spared = append(spared, leakRow{p, payload, reason})
			continue
		}
		leaks = append(leaks, leakRow{p, payload, ""})
	}
	if len(leaks) == 0 {
		return declaredLine(spared)
	}
	armed, warn := a.LoadGuardKill()
	kids := "children"
	if len(leaks) == 1 {
		kids = "child"
	}
	head := fmt.Sprintf("\n  load guard: %d orphaned gate-shell %s (ppid 1, over %g%% CPU, over %s)",
		len(leaks), kids, LoadCulpritOrphanCPU, BlindFor(LoadOrphanMinAge))
	if !armed {
		out := head + " — REPORT ONLY, nothing was killed:"
		shown := leaks
		if len(shown) > loadOrphanTop {
			shown = shown[:loadOrphanTop]
		}
		for _, l := range shown {
			out += fmt.Sprintf("\n    %.1f%% pid %d %s: %s", l.p.CPU, l.p.PID, BlindFor(l.p.Age), leakWhat(l))
		}
		if n := len(leaks) - len(shown); n > 0 {
			out += fmt.Sprintf("\n    %d more like these", n)
		}
		if warn != "" {
			out += "\n  " + warn
		}
		return out + declaredLine(spared)
	}
	targets := make([]Proc, 0, len(leaks))
	for _, l := range leaks {
		targets = append(targets, l.p)
	}
	tail, body := killArmLine(leaks, a.reapOrphans(targets))
	return head + tail + body + declaredLine(spared)
}

// leakWhat is the persona's command as a line can show it. The head of our
// preamble matching with no tail behind it means where our text stops is
// unknown, and saying so beats printing a slice of our own guard as if it
// were the persona's command.
func leakWhat(l leakRow) string {
	if what := ellipsize(l.payload, loadOrphanPayload); what != "" {
		return what
	}
	return "(command not readable behind the preamble)"
}

// ─── did *I* just leak (ranger-base-6mhxw) ──────────────────────────────────
//
// Arm 1 above only reads the process table on a pass the load guard is
// already skipping — a box already over the limit. That leaves the ordinary
// case unanswered: a persona backgrounds something, the Bash call returns,
// and it wants to know whether that leaked, on a box whose load never went
// anywhere near the guard. The incident this closes (ranger-base-k6csq) was
// exactly that: the session's own cleanup check read clean while forty
// spinners it had started kept running for two hours, and it failed for two
// structural reasons neither of which the guard above has:
//
//   - `jobs -l` is scoped to the CURRENT shell process. A gate session's
//     Bash tool calls each fork their own shell (ADR 0009 preamble), so a
//     job backgrounded in an earlier call is already invisible to a later
//     call's `jobs -l`, alive or dead.
//   - A per-process %CPU floor is blind to a leak that fans out wide: forty
//     spinners at ~1% each sit under any threshold worth setting, the same
//     way LoadCulpritOrphanCPU would miss them here.
//
// So this predicate drops the CPU floor entirely: orphaned, old enough not
// to be a fork/exec teardown window, and ours (the ADR 0009 preamble is the
// head of its argv) is leak enough on its own, however little CPU it burns.

// SelfCheckMinAge is SysSelfOrphans' age floor. It exists for the same
// reason LoadOrphanMinAge does — to clear the fork/exec teardown window a
// process is briefly ppid 1 in while the shell that forked it exits, which
// the teau RCA measured at about a second — and it is far shorter because
// the two checks run at a different distance from the fork: the load guard
// only reads on a pass it was already skipping, minutes into a stuck box,
// while a self-check runs seconds after the Bash call that might have
// leaked and should not have to wait a full minute to find out.
const SelfCheckMinAge = 3 * time.Second

// SysSelfOrphans is the reusable half of arm 1, without the load-spike
// framing: any caller can run it, at any time, to ask "did anything I just
// backgrounded leak past that call" — the question `jobs -l` and a CPU
// threshold cannot answer (see above). It returns one Proc per leak — ppid
// 1, old enough, argv matched against the ADR 0009 gate-shell preamble —
// with Args already trimmed to the persona's own command, preamble
// stripped, same as orphanReport's payload. Empty and nil on a clean box.
//
// It costs one `ps` on every call and a second, pid-scoped one only when the
// first turns up an orphan — the same shape as SysTopCPU, so a call that
// finds nothing costs what SysTopCPU costs a healthy box.
func SysSelfOrphans() ([]Proc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), loadCulpritTimeout)
	defer cancel()
	procs, err := sysProcTable(ctx)
	if err != nil {
		return nil, err
	}
	fillArgsForPIDs(ctx, procs, selfCheckSuspectPIDs(procs))
	return selfOrphansFrom(procs), nil
}

// selfOrphansFrom is the pure predicate over an already-read table (Args
// already filled for the rows selfCheckSuspectPIDs asked for) — the part of
// SysSelfOrphans a unit test can drive without a real `ps`.
func selfOrphansFrom(procs []Proc) []Proc {
	var leaks []Proc
	for _, p := range procs {
		if !selfCheckSuspect(p) {
			continue
		}
		payload, ours := gateShellForkPayload(p.Args)
		if !ours {
			continue
		}
		p.Args = payload
		leaks = append(leaks, p)
	}
	return leaks
}

// selfCheckSuspect is the CPU-agnostic half of the self-check predicate:
// orphaned and old enough. The argv-matched "ours" half is applied after the
// second read, in selfOrphansFrom, the same as orphanSuspect/orphanReport
// above.
func selfCheckSuspect(p Proc) bool {
	return p.Orphaned() && p.Age >= SelfCheckMinAge
}

// selfCheckSuspectPIDs scopes the second read to rows that could pass
// selfCheckSuspect — the reason a self-check on a clean box takes only one
// fork, the same property orphanSuspectPIDs keeps for the load guard.
func selfCheckSuspectPIDs(procs []Proc) []string {
	var ids []string
	for _, p := range procs {
		if selfCheckSuspect(p) {
			ids = append(ids, strconv.Itoa(p.PID))
		}
	}
	return ids
}

// FormatSelfOrphans renders SysSelfOrphans' leaks for a persona to read on a
// terminal or in a gate session's transcript. "" when there is nothing to
// report, so a caller can append it unconditionally.
func FormatSelfOrphans(leaks []Proc) string {
	if len(leaks) == 0 {
		return ""
	}
	kids := "children"
	if len(leaks) == 1 {
		kids = "child"
	}
	lines := make([]string, 0, len(leaks)+1)
	lines = append(lines, fmt.Sprintf("%d leaked gate-shell %s (ppid 1, over %s old):", len(leaks), kids, BlindFor(SelfCheckMinAge)))
	for _, p := range leaks {
		what := p.Args
		if what == "" {
			what = "(command not readable behind the preamble)"
		}
		// A DECLARED process is still listed — this is a census of what is
		// still running under you, and you asked — but it is marked, so the
		// caller does not tell a persona to declare what it already
		// declared, and so the marker can be seen to have been READ rather
		// than merely written (ranger-base-gvp2p).
		mark := ""
		if reason, ok := orphanDeclared(p.Args); ok {
			mark = fmt.Sprintf(" [declared %s%s]", LoadOrphanKeepMarker, reason)
		}
		lines = append(lines, fmt.Sprintf("  pid %d, %s old%s: %s", p.PID, BlindFor(p.Age), mark, ellipsize(what, loadOrphanPayload)))
	}
	return strings.Join(lines, "\n")
}

// ellipsize cuts s to at most n runes, marking that it did. Runes, because
// a persona command holds whatever the persona typed and half a UTF-8
// sequence in a log line is a mess someone else has to read.
func ellipsize(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
