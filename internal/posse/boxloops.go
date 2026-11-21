package posse

// The dispatch loops ON THIS BINARY — the box-wide half of G18's subject
// (ranger-base-is1qp, from ranger-base-nqaa1 finding 1; ADR 0092 D8).
//
// WHY THIS EXISTS. `posse upgrade` renames ONE file, `~/.local/bin/posse`,
// and rename(2) leaves every running process on its old inode (ADR 0092
// D3.5). That file is box-wide: every RHQ_HOME's `dispatch --watch` loop
// execs it, and the loop is what RENDERS a launch. Every reader of that fact
// was per-instance — `ReadWatchImage(a *App)` asks ONE home's watch lock and
// reads ONE home's `dispatch-watch.pid`, ShopCheck's G18 arm runs per
// instance, and the verb runs in the delegate's instance only — so after an
// install nothing on this box said a SIBLING instance's loop was on the old
// inode. ranger-base-nqaa1 made that reading say its own reach out loud
// (`watchScopeSentence`); this is the widening it deliberately did not do.
//
// WHAT IS COINED, AND WHAT IS REFUSED. The bead asked for "the instances on
// this box". That notion is refused, because it is the wrong derivation and
// not merely an absent one: `RHQ_HOME` is an environment variable and
// nothing constrains it to live under one root, so enumerating declared
// instances OVER-reports (an instance with no loop running is at no risk —
// there is nothing to leave on an old inode) and UNDER-reports (a loop whose
// home is outside the root is exactly the hole). What is coined instead is
// derived from the FILE the install renames:
//
//	the dispatch loops on this binary = every process on this box whose
//	image came from the path being replaced, and whose own RHQ_HOME's
//	`state/dispatch-watch.pid` names that process.
//
// That set needs no config, no homes root and no new key in anything. It
// sees loops nobody declared, and it cannot see an instance that is not
// running one.
//
// THE KEY IS THE EXE LINK, NEVER ARGV. Candidacy is `/proc/<pid>/exe`
// resolving to the target path — the kernel's own statement about which image
// a process runs, which survives the rename and says so (readlink appends
// " (deleted)"). It is NOT an argv match: every seat on this box runs
// byte-identical argv from a different instance, which is why AGENTS.md
// ("Ending anything") forbids an argv pattern as a target key, and
// `sysProcTable` (loadguard.go) — two bounded `ps` reads giving comm and
// argv — therefore cannot be the reader here even though it already walks
// the box. `ps` cannot report the (dev, inode) of a process's image at all.
//
// THE RECORD IS A FILTER, NOT AN ENUMERATOR, and that is the whole trick.
// Each candidate names its own instance (its `RHQ_HOME`), and a candidate is
// a loop only when THAT home's `dispatch-watch.pid` names THAT pid — the
// per-home invariant watchpid.go already keeps, read in the one direction
// that needs no homes root. Enumerating records would need one; filtering
// pids /proc already produced does not.
//
// MEASURED 2026-11-21, this box, linux 7.2.15-1-omarchy, uid 1001 at cage
// shims, target `~/.local/bin/posse` at dev:ino 54:14541392 — three
// processes ran that image:
//
//	1185112  ppid 1140     54:14541392  RHQ_HOME=~/homes/weller  record 1185112  LOOP
//	1185185  ppid 1140     54:14541392  RHQ_HOME=~/homes/ranger  record 1185185  LOOP
//	3285561  ppid 3285542  54:14541392  RHQ_HOME=~/homes/ranger  record 1185185  NOT a loop
//
// The third row is why the filter is load-bearing, and it is live rather than
// hypothetical: a seat's own `posse pulse wait --after 494` child runs the
// same image from the same path under the same RHQ_HOME as the ranger loop.
// Keyed on the exe link alone it reads as a loop; its home's record does not
// name it, so the filter rejects it. Sibling `exe` readlinks, their stats,
// `environ` and the sibling home's record were all readable from that seat.
// Neither loop was stale when it was read — both had restarted at 12:11:54Z
// onto the installed inode — so what is measured here is READABILITY, which
// is what the mechanism needs; a live stale sibling is still unmeasured.
//
// A READER, AND NOTHING ELSE. Restarting a live dispatcher stays the
// operator's or the coordinator's (ADR 0092 D6, guardrail 3), so nothing here
// restarts, signals or touches any process it finds. Liveness is
// "`/proc/<pid>` is there", never `kill -0`: under LANDLOCK_SCOPE_SIGNAL a
// caged seat's signal 0 to a process it did not fork is EPERM whether that
// process is alive or dead (AGENTS.md, ranger-base-e5un1), so a
// signal-based liveness check would read wrong from exactly the seats that
// most need it. The reading itself survives a cage by ruling rather than by
// luck: `--unshare-pid` is deliberately absent from the bwrap argv so a seat
// can read the box's process table (bwrap.go, ADR 0092-era D17), and /proc
// stays the host's procfs under `--ro-bind / /`.
//
// WHOSE READING IT IS: the upgrade verb's alone. G18 stays per-instance and
// unchanged. A box-wide row inside a per-instance report is a new category
// and an incoherent one — it would be raised once per instance that happens
// to have pulse state and silent in every instance that has none (the weller
// instance has no pulse state at all, so no tick there evaluates G18), so one
// box-wide condition would be both duplicated and missed depending on which
// instances tick. The verb is the one command whose subject is box-wide.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BoxProc is one process on this box running the image at the path an
// install replaces. Loop says whether it is a dispatch loop: its own
// instance's `dispatch-watch.pid` names it.
type BoxProc struct {
	Pid   int
	Image string // dev:ino of the image it runs, from /proc/<pid>/exe
	Home  string // the RHQ_HOME it declares; "" when the reader could not say
	Loop  bool   // this pid IS its home's recorded dispatch loop
	// Why is the reason Loop is false, for the one line that names it. A
	// candidate that could not be attributed is NAMED and never dropped:
	// silent under-reporting is the hole this reader exists to close.
	Why string
}

// Stale is the condition, against what the path holds now: this process runs
// an image that is no longer the one at the path it came from.
//
// A PATH THAT HOLDS NOTHING IS STALE, which is `WatchImage.Stale`'s predicate
// exactly (watchimage.go) and is deliberate rather than inherited. An empty
// OnDisk means nothing is at that path at all — mid-swap, or a window where
// `install(1)` has unlinked and not yet written (ADR 0092 Context) — and a
// loop whose image is not the one at its path is in the condition this
// reading is about whether the replacement is there yet or not. Excluding it
// would render as "all already on the installed image" over a path with no
// installed image on it, which is the one sentence that must never be
// printable; the rendering says the path holds nothing instead.
func (p BoxProc) Stale(onDisk string) bool {
	return p.Image != "" && p.Image != onDisk
}

// BoxReading is one box-wide reading over one path.
type BoxReading struct {
	Path   string    // the binary the reading is about
	OnDisk string    // dev:ino of Path now; "" when nothing is there
	Procs  []BoxProc // every candidate found, pid order, loops and not
	// Err is why there is no reading at all — not linux, no readable /proc.
	// An ABSTENTION, never an all-clear: the renderings say so.
	Err error
}

// Loops are the confirmed dispatch loops, which is the subject.
func (r BoxReading) Loops() []BoxProc {
	var out []BoxProc
	for _, p := range r.Procs {
		if p.Loop {
			out = append(out, p)
		}
	}
	return out
}

// StaleLoops are the loops a rename left behind: the ones whose restart ADR
// 0092 D6 is about.
func (r BoxReading) StaleLoops() []BoxProc {
	var out []BoxProc
	for _, p := range r.Loops() {
		if p.Stale(r.OnDisk) {
			out = append(out, p)
		}
	}
	return out
}

// Unattributed are the candidates that are not loops: a process running the
// same image whose instance's record names someone else, or whose instance
// the reader could not name. They are reported as a count and their pids,
// because naming them is honest and dropping them would be the failure this
// reader exists to prevent — but briefly, because they render no launch.
func (r BoxReading) Unattributed() []BoxProc {
	var out []BoxProc
	for _, p := range r.Procs {
		if !p.Loop {
			out = append(out, p)
		}
	}
	return out
}

// boxProbe is the platform's half of the reading, as a struct of functions so
// a pin can drive the filter with no /proc at all. ExeOf is `procExe`, the
// same spelling G18 reads an image identity through (watchimage.go), so the
// per-instance reading and this one cannot disagree about what a pid runs.
type boxProbe struct {
	Pids   func() ([]int, error)
	ExeOf  func(pid int) (id, path string, ok bool)
	HomeOf func(pid int) (string, bool)
	RecPid func(home string) (int, bool)
	Ident  func(path string) string
	Self   int // the reading process, which is skipped — see readBoxLoops
}

// liveBoxProbe is the real one.
func liveBoxProbe() boxProbe {
	return boxProbe{
		Pids:   boxPids,
		ExeOf:  procExe,
		HomeOf: boxHomeOf,
		RecPid: boxRecordedLoopPid,
		Ident:  fileIdentity,
		Self:   os.Getpid(),
	}
}

// boxRecordedLoopPid is the pid an instance's own record names as its
// dispatch loop. The filter's right-hand side, and the only thing this
// reader asks of another instance's state dir.
func boxRecordedLoopPid(home string) (int, bool) {
	rec, ok := ReadWatchPid(WatchPidPath(&App{StateDir: filepath.Join(home, "state")}))
	if !ok {
		return 0, false
	}
	return rec.Pid, true
}

// ReadBoxLoops reads every process on this box running the image at path.
// An abstention — off linux, or a /proc that will not answer — comes back as
// BoxReading.Err and never as an empty all-clear.
func ReadBoxLoops(path string) BoxReading {
	return readBoxLoops(path, liveBoxProbe())
}

// readBoxLoops is the filter, over whatever probe it is given.
//
// THE READING PROCESS IS SKIPPED, and it has to be said rather than noticed:
// `posse upgrade` runs FROM the path it replaces, so the verb is itself a
// process whose exe link names the target — and after its own swap its own
// image is the old inode, so an unskipped reader would report the reader as a
// stale process on this binary on every single run. True, and useless: this
// process is about to exit and renders no launch. A transient CHILD of the
// verb can still appear (the verify block execs the installed binary), and
// that one is NOT skipped: it is named among the unattributed, where the line
// says it is not a loop, because a reader that silently drops processes it
// thinks it recognises is the shape this whole file is a correction of.
//
// A process of ANOTHER uid running the same path is INVISIBLE here:
// readlink(/proc/<pid>/exe) is EACCES across uids, so such a process is not
// even a candidate. ASSUMED harmless on this box, where every instance's
// loop runs as the one account (MEASURED 2026-11-21: all three candidates
// uid 1001) — stated because it is a limit of the reading and not of the
// subject.
func readBoxLoops(path string, p boxProbe) BoxReading {
	r := BoxReading{Path: path}
	if path == "" {
		r.Err = Die("no path to read the box's loops against")
		return r
	}
	pids, err := p.Pids()
	if err != nil {
		r.Err = err
		return r
	}
	r.OnDisk = p.Ident(path)
	for _, pid := range pids {
		if pid == p.Self {
			continue
		}
		id, exe, ok := p.ExeOf(pid)
		if !ok || exe != path {
			continue
		}
		proc := BoxProc{Pid: pid, Image: id}
		home, ok := p.HomeOf(pid)
		switch {
		case !ok || home == "":
			proc.Why = "the reader could not name this process's instance (no readable RHQ_HOME in its environ)"
		default:
			proc.Home = home
			recPid, ok := p.RecPid(home)
			switch {
			case !ok:
				proc.Why = "that instance has no readable state/dispatch-watch.pid, so nothing says it is the loop"
			case recPid == pid:
				proc.Loop = true
			default:
				proc.Why = fmt.Sprintf("that instance's dispatch-watch.pid names pid %d, not this one", recPid)
			}
		}
		r.Procs = append(r.Procs, proc)
	}
	sort.Slice(r.Procs, func(i, j int) bool { return r.Procs[i].Pid < r.Procs[j].Pid })
	return r
}

// boxScopeSentence is what this reading says about its own reach — the
// counterpart of watchScopeSentence, which says what the per-instance one
// does not cover. Each line states its own reach; neither claims the other's.
func boxScopeSentence(path string) string {
	return "scope: every process on this box whose /proc/<pid>/exe names " + AbbrevHome(path) +
		" — the kernel's statement about which image a process runs, never an argv match, " +
		"and a dispatch loop is one whose own instance's state/dispatch-watch.pid names it (ranger-base-is1qp)"
}

// Sentence is the rendering the upgrade verb prints: the box-wide companion
// to the per-instance `dispatch:` line above it.
func (r BoxReading) Sentence() string {
	if r.Err != nil {
		return fmt.Sprintf("dispatch (box): not read — %v. So the only loop reading in this report is the per-instance one above, and ADR 0092 D6's \"restart the dispatch loops\" still reaches instances nothing here looked at (ranger-base-is1qp).", r.Err)
	}
	loops, stale := r.Loops(), r.StaleLoops()
	var b strings.Builder
	b.WriteString("dispatch (box): ")
	switch {
	case len(loops) == 0:
		b.WriteString("no dispatch loop on this box runs " + AbbrevHome(r.Path) +
			" — nothing a rename of it left behind, and nothing it renders either")
	case len(stale) == 0:
		b.WriteString(fmt.Sprintf("%s on this box, all already on the installed image — %s",
			boxCount(len(loops), "dispatch loop", "dispatch loops"), boxProcList(loops, r.OnDisk, false)))
	default:
		b.WriteString(fmt.Sprintf("%s on this box, %d still on the image the rename replaced — %s; launches those render the old walls until they restart, and restarting a live dispatcher is the operator's or the coordinator's (ADR 0092 D6, guardrail 3)",
			boxCount(len(loops), "dispatch loop", "dispatch loops"), len(stale), boxProcList(loops, r.OnDisk, true)))
		if r.OnDisk == "" {
			b.WriteString(" — and " + AbbrevHome(r.Path) + " holds NOTHING right now, so every one of them is stale against an absent file: read this as a swap in flight or a missing install before reading it as loops to restart")
		}
	}
	if other := r.Unattributed(); len(other) > 0 {
		b.WriteString(fmt.Sprintf(". %s on this binary %s: %s",
			boxCount(len(other), "other process", "other processes"),
			plural(len(other), "is not a dispatch loop", "are not dispatch loops"),
			boxOtherList(other)))
	}
	b.WriteString(". " + boxScopeSentence(r.Path))
	return b.String()
}

// boxProcList names each loop by its instance and pid. perProcState is off
// when the branch above already said the state of all of them — repeating
// "on the installed image" once per loop made the ordinary line, the one an
// operator reads after every install, twice as long as the information in it.
// It is on the moment any of them differ, because then the per-loop word is
// the only thing that says WHICH.
func boxProcList(ps []BoxProc, onDisk string, perProcState bool) string {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		where := p.Home
		if where == "" {
			where = "an instance this reader could not name"
		} else {
			where = AbbrevHome(where)
		}
		row := fmt.Sprintf("%s pid %d", where, p.Pid)
		if perProcState {
			if p.Stale(onDisk) {
				row += " STALE, runs inode " + p.Image
			} else {
				row += " on the installed image"
			}
		}
		parts = append(parts, row)
	}
	return strings.Join(parts, "; ")
}

func boxOtherList(ps []BoxProc) string {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, fmt.Sprintf("pid %d (%s)", p.Pid, p.Why))
	}
	return strings.Join(parts, "; ")
}

// boxCount is "<n> <noun>", with BOTH spellings given rather than an "s"
// appended: "other process" pluralizes to "other processes", and a helper
// that guesses would have rendered "2 other processs" in the one branch a
// reader meets least often and would therefore proof-read least.
func boxCount(n int, one, many string) string {
	return fmt.Sprintf("%d %s", n, plural(n, one, many))
}
