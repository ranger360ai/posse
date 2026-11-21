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
// most need it.
//
// WHAT A CAGED READER GETS, AND THE CLAIM THIS FILE FIRST MADE WRONGLY
// (ranger-base-rph5z, the security review). The first version of this header
// said the
// reading "survives a cage by ruling rather than by luck", on the grounds
// that `--unshare-pid` is deliberately absent from the bwrap argv and /proc
// stays the host's procfs under `--ro-bind / /` (bwrap.go). That is true and
// it is not enough: **it opens the pid LIST, not the links.** A seat in a
// child user namespace is REFUSED a same-uid peer's `exe` and `environ`,
// because `cap_ptrace_access_check` compares user namespaces and not just
// uids (the ranger-base-rfyth mechanism). MEASURED 2026-11-21 by the security
// persona from
// such a seat: `/proc` listed 460 pids, and `readlink /proc/1185185/exe` and
// `cat /proc/1185185/environ` were both EACCES while a child that seat had
// started itself was readable. The reading in the MEASURED block above was
// taken from a `cage shims` seat, which is not in a child userns — so the
// reader class this census most needs to work from is the class that first
// reading did not cover.
//
// The consequence is the whole reason `BoxReading.Unreadable` exists.
// `procExe` answers the same "not ok" for a pid that exited and for a link
// the kernel refused, so the first version dropped both silently and rendered
// "no dispatch loop on this box runs …" — a false all-clear over loops it was
// simply not allowed to see, which is the one sentence this file says must
// never be printable. A refused read of a process that is still there and
// still OURS is counted, and every rendering that would otherwise read as an
// all-clear says so instead.
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
	// Unreadable is how many processes of THIS uid the reader could not read
	// at all — their /proc/<pid>/exe is refused, so they never became
	// candidates and nothing here can say whether a dispatch loop is among
	// them (ranger-base-rph5z). It is a count and not a list because from a
	// child-userns seat it is every same-uid process on the box, which is
	// hundreds; what the reader needs is that the number is not zero.
	//
	// IT IS THE DIFFERENCE BETWEEN "no stale loop" AND "no stale loop I
	// could see", and before it existed the census printed the first while
	// meaning the second.
	Unreadable int
	// Err is why there is no reading at all — not linux, no readable /proc,
	// or not one readable process among the ones this uid owns. An
	// ABSTENTION, never an all-clear: the renderings say so.
	Err error
}

// Partial is "there is a reading, but it does not cover the box": something
// of ours was refused. Every rendering that would otherwise read as an
// all-clear has to say this.
func (r BoxReading) Partial() bool { return r.Unreadable > 0 }

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
	// Owned says whether a pid ExeOf could not read is nonetheless a process
	// of this uid — the difference between "refused" and "gone", which
	// ExeOf's single bool cannot carry (ranger-base-rph5z).
	Owned func(pid int) bool
	// CommOf is the kernel's program name for a pid, readable where the exe
	// link is refused. It narrows which refusals are worth reporting and it
	// never confirms a loop (boxloops_linux.go says why that distinction is
	// not the argv matching AGENTS.md forbids).
	CommOf func(pid int) (string, bool)
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
		Owned:  boxPidOwned,
		CommOf: boxCommOf,
		Self:   os.Getpid(),
	}
}

// boxRecordedLoopPid is the pid an instance's own record names as its
// dispatch loop. The filter's right-hand side, and the only thing this
// reader asks of another instance's state dir.
func boxRecordedLoopPid(home string) (int, bool) {
	path := WatchPidPath(&App{StateDir: filepath.Join(home, "state")})
	// LSTAT FOR A REGULAR FILE BEFORE READING (ranger-base-cwboz (b)). The
	// directory this reaches into is named by another process, so the file at
	// the end of it is not ours: `ReadWatchPid` stats and then reads, and an
	// open(2) of a FIFO there blocks forever with no timeout — in `posse
	// upgrade`, after the swap, which is the worst place on this box to hang.
	// Lstat and not Stat so a symlink to a FIFO is refused as well.
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() {
		return 0, false
	}
	rec, ok := ReadWatchPid(path)
	if !ok {
		return 0, false
	}
	return rec.Pid, true
}

// boxHomeIsWellFormed is the fence on a value this census lifts out of
// ANOTHER process's environment (ranger-base-cwboz).
//
// WHY A VALUE FROM THERE NEEDS ONE. `RHQ_HOME` is whatever a same-uid process
// exported, and the census then trusts it twice: it RENDERS it, into a line
// the install delegate pastes onto an install bead — so arbitrary text,
// newlines included, would arrive in that persona's context under its own
// authorship, which is prompt injection into the one persona holding the
// install verb — and it OPENS it, as `<home>/state/dispatch-watch.pid`. It is
// not a planted-attacker story: a caged seat's own child qualifies, and this
// census's first live reading found exactly such a child (a `posse pulse
// wait` under the ranger home).
//
// So: an absolute, already-clean path of printable bytes, or nothing. A value
// that fails is NOT rendered in any form — the reader falls through to the
// abstention path, which names the pid and no text. The check is deliberately
// about SHAPE and not about existence: a home that is well-formed and absent
// is a different answer (the record read says so), and a reader that stat'ed
// here would do the untrusted open this function exists to gate.
func boxHomeIsWellFormed(v string) bool {
	if v == "" || !filepath.IsAbs(v) || filepath.Clean(v) != v {
		return false
	}
	for i := 0; i < len(v); i++ {
		if v[i] < 0x20 || v[i] == 0x7f {
			return false
		}
	}
	return true
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
// A process whose exe link this reader cannot read is NOT silently absent,
// and the boundary is the USER NAMESPACE and not the uid — which is what the
// first version of this comment got wrong (ranger-base-rph5z). It said "a
// process of ANOTHER uid is invisible here … ASSUMED harmless on this box,
// where every instance's loop runs as the one account", and the uid was the
// wrong fence to be reassured by: same-uid-but-cross-userns is refused too,
// so on a caged reader the "harmless" case was every sibling loop on the box.
// Now: a refused pid that `Owned` says is ours is counted in Unreadable and
// the renderings cannot claim a clear box; a refused pid of another account
// stays excluded, because root's processes and non-dumpable ones are refused
// to everyone and are not subjects of this rename — counting them would make
// every reading on every box permanently partial, which is a caveat nobody
// can act on and therefore a caveat nobody reads.
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
		if !ok {
			// A FAILED READ IS NOT AN ABSENCE (ranger-base-rph5z). `procExe`
			// answers the same "not ok" for a pid that exited and for a link
			// the kernel refused, and from a seat in a child user namespace
			// EVERY same-uid peer is refused — so dropping both silently is
			// what let this census render "no dispatch loop on this box"
			// over loops it was simply not allowed to see. If the process is
			// still there and is OURS, the honest answer is that we do not
			// know what it runs.
			if boxRefusalWorthReporting(p, pid, path) {
				r.Unreadable++
			}
			continue
		}
		if exe != path {
			continue
		}
		proc := BoxProc{Pid: pid, Image: id}
		// `declaredHome` and NOT `home`, which is not style: internal/treepins'
		// temp-root census resolves an identifier across the whole package,
		// so a literal assigned to `home` anywhere in internal/posse joins
		// the set of parents that `os.MkdirTemp(home, …)` in app_test.go
		// could be writing under. An `ok, home = false, ""` here put `""`
		// into that set and reddened a file this bead never touched. The
		// census is right to work that way — a prefix nobody can resolve is a
		// prefix nobody can check — and the cheap half of keeping it
		// resolvable is not reusing its names for a local.
		declaredHome, ok := p.HomeOf(pid)
		// VALIDATED AGAIN HERE, and boxHomeOf already refused a malformed
		// value before returning one (ranger-base-cwboz). Two checks on
		// purpose: the platform read refuses early so nothing malformed ever
		// leaves the environ, and this is the portable chokepoint that every
		// probe and every future platform reader goes through — so the fence
		// holds for a reader written after this file, which is the one a
		// comment cannot reach.
		if ok && !boxHomeIsWellFormed(declaredHome) {
			ok = false
			declaredHome = ""
		}
		switch {
		case !ok || declaredHome == "":
			proc.Why = "the reader could not name this process's instance (no readable, well-formed RHQ_HOME in its environ)"
		default:
			proc.Home = declaredHome
			recPid, ok := p.RecPid(declaredHome)
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
	// NOT ONE READABLE PROCESS OF OURS, AND SOME REFUSED: there is no
	// reading here, only a permission boundary, so this is an abstention and
	// not an empty box. That is the caged-reader case exactly, and it is the
	// one the renderings must not be allowed to dress up (ranger-base-rph5z).
	if len(r.Procs) == 0 && r.Unreadable > 0 {
		r.Err = Die("none of the %d process(es) of this account on this box could be read from this reader — /proc/<pid>/exe is refused, which is what a seat in a child user namespace gets for a same-uid peer — so whether any dispatch loop is still on the replaced image is UNKNOWN, not no", r.Unreadable)
	}
	return r
}

// boxRefusalWorthReporting decides whether a pid we were refused counts
// against the reading's completeness.
//
// IT HAS TO BE NARROWER THAN "ours and refused", which is what the first fix
// for ranger-base-rph5z used and what this corrects. MEASURED 2026-11-21 from
// a `cage shims` seat on this box: four processes are permanently in that set
// and none could ever be a dispatch loop — the user `systemd`, `(sd-pam)`,
// `sshd-session` and `gpg-agent`, all non-dumpable, all EACCES on `exe`. So
// the caveat fired on every reading forever, and a caveat that is always
// there is one a reader learns to skip, which would have left the false
// all-clear this field exists to prevent hiding behind a sentence nobody
// finishes.
//
// The three answers, in the order they are asked:
//
//   - not ours: EXCLUDED. Another account's process is refused to everyone
//     and is not a subject of this rename.
//   - ours, and its comm names a different program: EXCLUDED. It is not
//     running the binary this reading is about, so it cannot be a loop on it.
//   - ours, and its comm matches, or its comm could not be read either:
//     COUNTED. Both are "we do not know what this process of ours is
//     running", which is the whole content of the caveat.
//
// comm is truncated by the kernel to 15 bytes, so a longer basename is
// compared on that prefix: the truncation is the kernel's and reading it as a
// mismatch would exclude exactly the processes the caveat is for.
func boxRefusalWorthReporting(p boxProbe, pid int, path string) bool {
	if p.Owned == nil || !p.Owned(pid) {
		return false
	}
	if p.CommOf == nil {
		return true
	}
	comm, ok := p.CommOf(pid)
	if !ok {
		return true
	}
	want := filepath.Base(path)
	if len(want) > boxCommLen {
		want = want[:boxCommLen]
	}
	return comm == want
}

// boxCommLen is TASK_COMM_LEN-1: what the kernel keeps of a program name in
// /proc/<pid>/comm.
const boxCommLen = 15

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
	case len(loops) == 0 && r.Partial():
		// NOT "no dispatch loop on this box". The reader saw some of the box
		// and was refused the rest, and the two sentences differ by exactly
		// the thing an operator would act on (ranger-base-rph5z).
		b.WriteString(fmt.Sprintf("no dispatch loop runs %s among the processes this reader COULD read, and %s of this account it could NOT read may hold one — so this is UNKNOWN and not a clear box",
			AbbrevHome(r.Path), boxCount(r.Unreadable, "process", "processes")))
	case len(loops) == 0:
		b.WriteString("no dispatch loop on this box runs " + AbbrevHome(r.Path) +
			" — nothing a rename of it left behind, and nothing it renders either")
	case len(stale) == 0 && r.Partial():
		b.WriteString(fmt.Sprintf("%s this reader could read on this box, all already on the installed image — %s; but %s of this account it could NOT read may hold a loop that is not, so this is not a clear box",
			boxCount(len(loops), "dispatch loop", "dispatch loops"), boxProcList(loops, r.OnDisk, false),
			boxCount(r.Unreadable, "process", "processes")))
	case len(stale) == 0:
		b.WriteString(fmt.Sprintf("%s on this box, all already on the installed image — %s",
			boxCount(len(loops), "dispatch loop", "dispatch loops"), boxProcList(loops, r.OnDisk, false)))
	default:
		b.WriteString(fmt.Sprintf("%s on this box, %d still on the image the rename replaced — %s; launches those render the old walls until they restart, and restarting a live dispatcher is the operator's or the coordinator's (ADR 0092 D6, guardrail 3)",
			boxCount(len(loops), "dispatch loop", "dispatch loops"), len(stale), boxProcList(loops, r.OnDisk, true)))
		if r.OnDisk == "" {
			b.WriteString(" — and " + AbbrevHome(r.Path) + " holds NOTHING right now, so every one of them is stale against an absent file: read this as a swap in flight or a missing install before reading it as loops to restart")
		}
		if r.Partial() {
			b.WriteString(fmt.Sprintf(" — and these are only the loops this reader could read: %s of this account it could not read may hold more",
				boxCount(r.Unreadable, "process", "processes")))
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

// boxRefusedNote is the one phrase that must appear whenever a reading did
// not cover the box, in whichever branch rendered it. Kept as a predicate
// over the finished sentence rather than as a string the branches share,
// because what a pin needs to hold is that the SENTENCE says it — three
// branches assemble it three ways and a shared constant would let one of
// them drop the clause while still owning the constant.
func boxSentenceAdmitsItDidNotCoverTheBox(s string) bool {
	return strings.Contains(s, "could NOT read") || strings.Contains(s, "could not read") ||
		strings.Contains(s, "UNKNOWN, not no")
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
