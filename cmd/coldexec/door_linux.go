//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// struct seccomp_notif, struct seccomp_notif_resp and struct seccomp_data,
// as the kernel lays them out (linux/seccomp.h). They are checked against
// the running kernel's own idea of their sizes before anything runs —
// checkKernel below — rather than trusted.
type seccompData struct {
	Nr                 int32
	Arch               uint32
	InstructionPointer uint64
	Args               [6]uint64
}

type seccompNotif struct {
	ID    uint64
	Pid   uint32
	Flags uint32
	Data  seccompData
}

type seccompNotifResp struct {
	ID    uint64
	Val   int64
	Error int32
	Flags uint32
}

// seccompNotifSizes is what SECCOMP_GET_NOTIF_SIZES fills in.
type seccompNotifSizes struct {
	Notif     uint16
	NotifResp uint16
	Data      uint16
}

// pathMax bounds the read out of the target's memory. A path the kernel
// would refuse anyway is not worth reading.
const pathMax = 4096

// door is one supervised run.
type door struct {
	listener int
	delay    time.Duration
	// started is the door's epoch: an inode whose ctime is later than this
	// is a plant (ADR 0085 D2). It is taken before the child is launched,
	// so nothing the run creates can predate it.
	started time.Time

	mu   sync.Mutex
	seen map[inode]bool
	// inflight is the charges being held right now, keyed by the TASK that
	// is stopped in the execve and the inode it is exec'ing. A notification
	// that matches one IS the restart of a charge the kernel cut short, and
	// it sleeps the REMAINDER of that charge rather than being answered
	// warm (ranger-base-91fu2) — see the note above recordWithdrawal.
	inflight map[charge]*held

	notifs atomic.Int64
	// reaped counts the orphaned descendants the supervisor waited on. It
	// is the subreaper arm's only witness in a real run: before
	// ranger-base-orgvt those processes were reparented away and their
	// execs read `unreadable`, so a non-zero count here is the hole that
	// is no longer there.
	reaped atomic.Int64
	// Almost every notification lands on exactly one verdict, and the ones
	// that mean "this exec was not covered" are printed at the end rather
	// than left silent: a door that has stopped being able to read its
	// targets must not read as a suite with no members of the class. One
	// verdict per NOTIFICATION, note, not per execve: a restarted execve
	// raises a fresh notification and spends two (see the note above holes).
	//
	// ALMOST, and here is the exception, because the honest version of this
	// comment is the one that names it. A notification the kernel takes
	// back after adjudicate has already recorded `old`, `warm` or `absent`
	// records vWithdrawn too, so it spends TWO verdicts and the report's
	// verdicts sum to more than its notification count. Holes and delays no
	// longer do this — both ride down to answer() and are recorded there
	// once (ranger-base-j0riv, ranger-base-4kars) — and a verdict adjudicate
	// settles itself is not FALSIFIED by the withdrawal the way a delay is:
	// the file really was old.
	//
	// MEASURED 2026-11-11, D = 2.5s, the five coldexec arms: four of the
	// five summed EXACTLY to their notification count, and arm 1 overshot by
	// exactly its 4 withdrawn — 129555 notifications against 129559 verdicts
	// (docs/notes.d/ranger-base-4kars.md). ranger-base-9uvsx holds the
	// decision about closing it, and the pin it would need is
	// notifs == sum(verdicts).
	verdicts [numVerdicts]atomic.Int64

	// verbose prints one line per notification. It is for triage — which
	// plants a run delayed, and why the rest were passed through — and it
	// is off unless asked for, because an arm takes tens of thousands of
	// these.
	verbose bool

	wg sync.WaitGroup
}

// The verdicts, in the order the report prints them.
type verdict int

const (
	// Covered, and the two of these are the door working.
	vDelayed verdict = iota // a plant's first execve: held D
	vWarm                   // a plant this run has already paid for
	vOld                    // an inode older than the door: the checkout, the toolchain
	// Not a plant and not a hole either: an execve that is going to fail
	// on its own. A shell searching PATH produces one of these per
	// directory it misses, which is why they are counted apart from the
	// holes below rather than lumped in with them.
	vAbsent
	// Also not a hole, and for a stronger reason than vAbsent's: an execve
	// the KERNEL did not run. See the note above holes.
	vWithdrawn
	// A charge the kernel cut short with NOTHING to restart it: the door
	// charged this exec D, the kernel took the notification back mid-hold,
	// and no second notification for the same task and inode ever arrived
	// to carry the remainder. The execve therefore did not happen — a
	// caller whose handler lacks SA_RESTART takes EINTR, one with no
	// handler dies — so nothing escaped the door and this is not a hole
	// either. What is gone is the charge: the plant is marked warm from
	// here on, so a LATER exec of it is not held. The PAYING shape is
	// `withdrawn` on the cut-short notification and `delayed` on its
	// restart. See the note above recordWithdrawal.
	vTruncated
	// The holes. Each one is an execve the door let through without
	// knowing what it was.
	vUnreadable // the path argument could not be read out of the caller
	vUnstattable
	vForeign
	numVerdicts int = iota
)

// vNone is not a verdict and is never recorded. adjudicate answers it to
// say "I have recorded this notification's verdict myself"; anything else
// is a verdict only answer() is in a position to decide.
const vNone verdict = -1

var verdictNames = [numVerdicts]string{
	vDelayed:     "delayed",
	vWarm:        "warm",
	vOld:         "old",
	vAbsent:      "absent",
	vWithdrawn:   "withdrawn",
	vTruncated:   "truncated",
	vUnreadable:  "unreadable",
	vUnstattable: "unstattable",
	vForeign:     "foreign-syscall",
}

// A WITHDRAWN NOTIFICATION IS NOT A HOLE, and this is the note for why,
// because that classification has been wrong once (ranger-base-j0riv:
// `gone`, as it was called, was in holes, so a door redded its own ARM 3
// pin intermittently under `go test ./...` and more often the busier the
// box).
//
// The kernel's seccomp wait is interruptible. When the stopped caller takes
// a deliverable signal the kernel REMOVES the notification and sets the
// syscall's return to ERESTARTSYS, and what the caller does next is decided
// by the handler it had installed. All three endings were MEASURED here on
// 2026-11-11 (this box, linux 7.2.15 x86_64, go1.27.3), each one run under
// this door:
//
//	a handler WITH SA_RESTART — which is what the Go runtime installs, its
//	  own preemption SIGURG included: the execve is RESTARTED and raises a
//	  FRESH notification, and that one is adjudicated. One plant read
//	  `delayed` then `warm`, ran, and its caller exited 0.
//	a handler WITHOUT SA_RESTART (python3 installs these): the execve
//	  returns EINTR to the caller and never happens — Python raised
//	  InterruptedError, exit 1.
//	no handler and a default-fatal signal: the caller dies stopped and the
//	  execve never happens — exit 138 for a default SIGUSR1.
//
// The kernel never completes an execve whose notification it has taken
// back, so NOTHING escapes the door at full speed this way. What a
// withdrawal really costs is arithmetic: in the restart case one execve
// spends two notifications, and the door used to report the first of them
// as an exec it had failed to cover.
//
// This is why the supervisor's OWN Go child is the process that produces
// them — it is the one process under the door with an SA_RESTART handler on
// a signal the runtime sends itself — and why the rate tracks the box's run
// queue: the wider the queue, the more preemption signals that child takes
// while stopped in its execve of the command.
var holes = []verdict{vUnreadable, vUnstattable, vForeign}

func (d *door) record(v verdict, pid int, target string) {
	d.verdicts[v].Add(1)
	if d.verbose {
		fmt.Fprintf(os.Stderr, "coldexec: %-11s pid %d %s\n", verdictNames[v], pid, target)
	}
}

// inode is the identity a delay is charged against once. ctime is part of
// it on purpose: a file rewritten in place is a new (dev, ino, ctime) and is
// cold again, which is what the old door's per-write nonce did.
type inode struct {
	dev, ino    uint64
	csec, cnsec int64
}

// charge is one TASK's hold on one inode: the identity of a delay that is
// in flight. The pid is in the key because a restarted execve necessarily
// arrives on the same task — the kernel restarts the syscall in place — and
// a task can only be inside one execve at a time, so a notification that
// matches an in-flight charge IS that charge's restart and nothing else.
//
// MEASURED 2026-11-11 (this box, linux 7.2.15 x86_64, go1.27.3) that the
// pid in the key is the one the kernel hands out, which is the TASK id and
// not the thread-group id: a non-leader thread's execve of a plant was
// adjudicated under the thread's own tid in 3 of 3 runs, tid != tgid in all
// three. If it were the tgid, two threads of one process exec'ing one plant
// would collide on this key and the match would no longer be unique. Pinned
// by TestANonLeaderThreadsExecIsAdjudicatedUnderItsOwnTid.
type charge struct {
	pid int
	key inode
}

// held is a charge in flight.
type held struct {
	// deadline is charge start + D, and it is the whole invariant: no
	// notification for this charge is answered before it, so the execve
	// completes no earlier than D after it first entered the kernel —
	// however many notifications it spends getting there.
	deadline time.Time
	// restarted says a second notification for this charge arrived while it
	// was in flight, which can only be the restart of a withdrawn one. It
	// is what tells the handler holding the charge that its own withdrawal
	// handed the charge on rather than cutting it short.
	restarted bool
}

// supervise runs argv under the door and answers the exit code to use.
func supervise(d time.Duration, verbose bool, argv []string) int {
	if _, err := auditArch(); err != nil {
		refuse("%v", err)
	}
	if err := checkKernel(); err != nil {
		refuse("%v", err)
	}

	self, err := os.Executable()
	if err != nil {
		refuse("finding my own binary to re-exec as the supervised child: %v", err)
	}

	// A blocking SOCK_STREAM pair. CLOEXEC on both ends: the parent's end
	// must not reach the child, and the child's end is placed by os/exec
	// with dup2, which clears it again.
	sp, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		refuse("socketpair for the listener hand-off: %v", err)
	}
	kid := os.NewFile(uintptr(sp[1]), "coldexec-listener-handoff")

	cmd := exec.Command(self, append([]string{"-child", "--"}, argv...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.ExtraFiles = []*os.File{kid}
	// D has been read, so the variable leaves this process and therefore
	// every process under it. Not a cmd.Env: see consumeDelayEnv — and
	// BEFORE cmd.Start, which is where os/exec takes its snapshot of the
	// environment for a child whose Env is nil.
	consumeDelayEnv()

	// THE DOOR'S REACH IS ANCESTRY (readPath). On a box with
	// yama/ptrace_scope=1 the supervisor may read a descendant's memory
	// because it is its ancestor and for no other reason, so a descendant
	// REPARENTED away — its own parent exited — falls out of reach, and
	// its execs are un-adjudicated. MEASURED 2026-11-11 (ranger-base-orgvt,
	// Linux 7.2.15, ptrace_scope=1): 134 `/proc/<pid>/mem: permission
	// denied` in the internal/treepins arm's 29744 notifications,
	// clustered in the suite-lock self-tests, which orphan holder
	// processes on purpose — against none at all in cmd/posse's 8427,
	// whose tests do not orphan. A plant such a process execs is never
	// delayed and the test that planted it is not covered at all. (The
	// bead reads 145; the other 11 were `gone`, which ranger-base-j0riv
	// has since retired as a verdict and as a hole.)
	//
	// PR_SET_CHILD_SUBREAPER closes it: an orphaned descendant is
	// reparented to THIS process rather than past it, so it stays inside
	// the ptrace reach and its execs are adjudicated like any other's.
	// MEASURED the same day on the repro the bead carries: the orphan's
	// plant goes from 3ms and `unreadable` to 504ms and `delayed`, with no
	// holes left in the run.
	//
	// It is per-process (signal_struct), so no locked thread, and it is
	// NOT inherited as `is_child_subreaper` by the children — only the
	// hint that one exists above them — so every orphan below lands here
	// and not on an intermediate shell.
	//
	// REFUSED and not warned about: the flag is the difference between a
	// covered run and a run with silent holes in it, and ADR 0085 D5 is
	// that the door never runs a suite uncovered.
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		refuse("PR_SET_CHILD_SUBREAPER: %v — without it an orphaned descendant is reparented "+
			"out of this process's ptrace reach and its execs go un-adjudicated (ADR 0085 D5)", err)
	}

	dr := &door{delay: d, verbose: verbose, seen: map[inode]bool{}, inflight: map[charge]*held{}}
	// THE EPOCH, and it is here: after this line every file the run creates
	// is a plant, and every file it does not is the checkout, the toolchain
	// or the box.
	dr.started = time.Now()
	if err := cmd.Start(); err != nil {
		refuse("starting the supervised child: %v", err)
	}
	kid.Close()
	// The other half of the prctl, and it starts before the first
	// notification is read: see reapOrphans.
	go dr.reapOrphans(cmd.Process.Pid)

	lis, err := recvListener(sp[0])
	unix.Close(sp[0])
	if err != nil {
		// The child has already said why on stderr; it shares ours.
		_ = cmd.Wait()
		refuse("the child handed over no seccomp listener: %v", err)
	}
	dr.listener = lis

	fmt.Fprintf(os.Stderr, "coldexec: up — the first execve of every inode created from now on is held %s, "+
		"for every process under this one (ADR 0085 D2)\n", d)

	served := make(chan error, 1)
	go func() { served <- dr.serve() }()

	code := wait(cmd)
	dr.drain(served)
	// The listener is NOT closed here. serve may still be blocked in its
	// receive, and a notification that arrived between drain returning and
	// a close would be answered on a closed fd — which is the one error
	// answer() treats as fatal, so a clean run would end on exit 2. The fd
	// goes when the process does, which is two lines away.
	dr.report(d)
	if code != 0 {
		return code
	}
	// The PAID count, which is what vDelayed is since ranger-base-4kars: a
	// run whose every injection was truncated injected nothing, and ADR
	// 0085 D5 does not care why.
	if dr.verdicts[vDelayed].Load() == 0 {
		fmt.Fprintln(os.Stderr,
			"coldexec: the command passed and the door delayed NOTHING — a run that injected "+
				"nothing says nothing about the class, so this is a failed door and not a clean "+
				"suite (ADR 0085 D5)")
		if n := dr.verdicts[vTruncated].Load(); n > 0 {
			fmt.Fprintf(os.Stderr,
				"coldexec: and it is the truncation above that took them: %d charges were cut "+
					"short by a withdrawal that nothing restarted, so this run held none of them "+
					"to the end\n", n)
		}
		return exitNoneDelayed
	}
	return 0
}

// report is the run's own account of what it did. ADR 0085 D5 asks for the
// notifications and the delays; the rest is here because a door's silence
// and a door's clean bill of health have to look different.
func (d *door) report(delay time.Duration) {
	fmt.Fprintf(os.Stderr, "coldexec: %d execve notifications, %d cold first execs delayed %s each (%d warm, %d older than the door, %d absent)\n",
		d.notifs.Load(), d.verdicts[vDelayed].Load(), delay,
		d.verdicts[vWarm].Load(), d.verdicts[vOld].Load(), d.verdicts[vAbsent].Load())
	if n := d.verdicts[vWithdrawn].Load(); n > 0 {
		fmt.Fprintf(os.Stderr, "coldexec: %d notifications were withdrawn before the door could answer — "+
			"the kernel does not run an execve whose notification it took back, so none of those escaped "+
			"the door; a caller whose handler carries SA_RESTART restarts the execve and raises a fresh "+
			"notification, which is the one this run adjudicated (ranger-base-j0riv). One that was "+
			"holding a charge hands it to that restart, which sleeps the remainder, so the execve is "+
			"still held %s from where it started (ranger-base-91fu2)\n", n, delay)
	}
	if n := d.verdicts[vTruncated].Load(); n > 0 {
		fmt.Fprintf(os.Stderr, "coldexec: %d charges were TRUNCATED — the kernel withdrew the "+
			"notification while the door was holding it and NO restart arrived to carry the remainder, "+
			"so the caller was released at that signal and not at %s. Those execves did not happen — a "+
			"caller whose handler lacks SA_RESTART takes EINTR — so nothing escaped the door; what is "+
			"gone is the charge, and the plant is marked warm from here on. They are NOT in the delayed "+
			"count above; -verbose names each one (ranger-base-4kars, ranger-base-91fu2)\n", n, delay)
	}
	var total int64
	var parts []string
	for _, v := range holes {
		if n := d.verdicts[v].Load(); n > 0 {
			total += n
			parts = append(parts, fmt.Sprintf("%d %s", n, verdictNames[v]))
		}
	}
	if total > 0 {
		fmt.Fprintf(os.Stderr, "coldexec: %d execs went un-adjudicated (%s) — those were NOT covered; "+
			"-verbose names each one\n", total, strings.Join(parts, ", "))
	}
	if n := d.reaped.Load(); n > 0 {
		fmt.Fprintf(os.Stderr, "coldexec: %d orphaned descendants were reparented onto this process and reaped "+
			"— their execs were adjudicated, which before ranger-base-orgvt they were not\n", n)
	}
}

// checkKernel asks the running kernel whether it has what the door needs,
// and whether the structs this binary was compiled with are the ones it
// will fill in. Both before the command starts: ADR 0085 D5's "it never
// runs a suite uncovered" is this function.
func checkKernel() error {
	avail := uint32(unix.SECCOMP_RET_USER_NOTIF)
	if _, _, errno := unix.Syscall(unix.SYS_SECCOMP, uintptr(unix.SECCOMP_GET_ACTION_AVAIL), 0, uintptr(unsafe.Pointer(&avail))); errno != 0 {
		return fmt.Errorf("seccomp(GET_ACTION_AVAIL, SECCOMP_RET_USER_NOTIF): %v — this kernel has no user "+
			"notification, which the door is built on (Linux 5.5+)", errno)
	}
	var sizes seccompNotifSizes
	if _, _, errno := unix.Syscall(unix.SYS_SECCOMP, uintptr(unix.SECCOMP_GET_NOTIF_SIZES), 0, uintptr(unsafe.Pointer(&sizes))); errno != 0 {
		return fmt.Errorf("seccomp(GET_NOTIF_SIZES): %v", errno)
	}
	for _, c := range []struct {
		what   string
		kernel uint16
		mine   uintptr
	}{
		{"seccomp_notif", sizes.Notif, unsafe.Sizeof(seccompNotif{})},
		{"seccomp_notif_resp", sizes.NotifResp, unsafe.Sizeof(seccompNotifResp{})},
		{"seccomp_data", sizes.Data, unsafe.Sizeof(seccompData{})},
	} {
		if uintptr(c.kernel) != c.mine {
			return fmt.Errorf("this kernel's struct %s is %d bytes and the one compiled in here is %d — "+
				"the door would be reading the wrong fields out of every notification", c.what, c.kernel, c.mine)
		}
	}
	return nil
}

// serve is the receive loop. It returns when the listener says every
// process that carried the filter is gone (ENOTCONN), or on an error that
// means the door is no longer up.
func (d *door) serve() error {
	for {
		// Zeroed per iteration, which the ioctl requires, and per
		// iteration ALSO because the goroutine below keeps the pointer.
		n := new(seccompNotif)
		if err := ioctlPtr(d.listener, unix.SECCOMP_IOCTL_NOTIF_RECV, unsafe.Pointer(n)); err != nil {
			switch {
			case errors.Is(err, unix.EINTR):
				// The Go runtime preempts with signals, so this is ordinary.
				continue
			case errors.Is(err, unix.ENOENT):
				// The caller died between the stop and our receive.
				continue
			default:
				return err
			}
		}
		d.notifs.Add(1)
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			d.handle(n)
		}()
	}
}

// drain waits for the handlers still sleeping once the command has exited.
// It is bounded: a descendant the run leaked can hold the filter open for
// as long as it likes, and the supervisor is not the process that reaps
// those (`go run ./cmd/checkorphans` is).
func (d *door) drain(served <-chan error) {
	done := make(chan struct{})
	go func() { d.wg.Wait(); close(done) }()
	select {
	case <-done:
	case err := <-served:
		// Every filter holder is gone, which is the clean end. Anything
		// else is the door having fallen over mid-run, and it is said out
		// loud because the run's remaining execs went uncovered.
		if err != nil && !errors.Is(err, unix.ENOTCONN) {
			fmt.Fprintf(os.Stderr, "coldexec: the notification loop stopped early: %v — execs after that point were NOT covered\n", err)
		}
	case <-time.After(d.delay + 5*time.Second):
		fmt.Fprintln(os.Stderr,
			"coldexec: the command exited with execs still held — something it started outlived it. "+
				"Answering nothing further; those execs get ENOSYS when this process goes "+
				"(`go run ./cmd/checkorphans` is the tool for what leaked)")
	}
}

// handle adjudicates one notification and ALWAYS answers it. A notification
// that goes unanswered is an execve blocked in the kernel forever, so every
// failure path here falls through to the answer.
func (d *door) handle(n *seccompNotif) {
	pending, taken, pendingTarget := d.adjudicate(n)
	d.answer(n, pending, taken, pendingTarget)
}

// adjudicate works out what this execve is about and sleeps D if it is the
// first exec of a file the run created.
//
// It records the verdicts it can settle on its own — old, warm, absent —
// and answers vNone for those. Two kinds it never records, and both for the
// same reason: whether this execve was let through at all, and at what
// speed, is decided at the answer and nowhere else, so the verdict rides
// down to answer() which records it once the kernel has spoken.
//
//	a HOLE is an execve the door let through without knowing what it was,
//	  and every hole here is load-sensitive in the same way: the caller can
//	  be unreadable, or its pid gone, precisely BECAUSE the kernel withdrew
//	  the notification under us (ranger-base-j0riv).
//	a DELAY is a charge the door claims to have paid, and a notification
//	  withdrawn mid-sleep released its caller at the signal instead
//	  (ranger-base-4kars; see the note above recordWithdrawal). Since
//	  ranger-base-91fu2 the charge outlives the notification that was
//	  holding it: a restart inherits the remainder, and the delay is
//	  recorded by whichever notification is still live at the deadline.
func (d *door) adjudicate(n *seccompNotif) (pending, taken verdict, pendingTarget string) {
	pid := int(n.Pid)
	var addr uint64
	var flags uint64
	dirfd := int(unix.AT_FDCWD)

	switch int(n.Data.Nr) {
	case unix.SYS_EXECVE:
		addr = n.Data.Args[0]
	case unix.SYS_EXECVEAT:
		dirfd = int(int32(uint32(n.Data.Args[0])))
		addr = n.Data.Args[1]
		flags = n.Data.Args[4]
	default:
		// The filter names no other syscall. If one ever arrives, let it
		// through rather than guess which argument is a path — and say so,
		// because it means the filter and this switch have drifted apart.
		return vForeign, vWithdrawn, fmt.Sprintf("syscall %d", n.Data.Nr)
	}

	p, err := readPath(pid, addr)
	if err != nil {
		return vUnreadable, vWithdrawn, err.Error()
	}

	// Resolved the way the kernel will resolve it, and through the
	// CALLER's cwd and fds — not this process's, which is the
	// supervisor's and is not where the test is.
	var target string
	switch {
	case p == "" && flags&unix.AT_EMPTY_PATH != 0:
		// execveat(fd, "", …, AT_EMPTY_PATH): the fd IS the file.
		target = fmt.Sprintf("/proc/%d/fd/%d", pid, dirfd)
	case p == "":
		d.record(vAbsent, pid, "(empty path)")
		return vNone, vWithdrawn, ""
	case filepath.IsAbs(p):
		target = p
	case dirfd == int(unix.AT_FDCWD):
		target = fmt.Sprintf("/proc/%d/cwd/%s", pid, p)
	default:
		target = fmt.Sprintf("/proc/%d/fd/%d/%s", pid, dirfd, p)
	}

	var st unix.Stat_t
	if err := unix.Stat(target, &st); err != nil {
		v, hole := d.statVerdict(pid, err)
		if hole {
			return v, vWithdrawn, target
		}
		d.record(v, pid, target)
		return vNone, vWithdrawn, ""
	}
	// The ctime, not the mtime: a copied binary keeps its source's mtime
	// and a `chmod +x` moves only the ctime, and both of those are plants.
	if !time.Unix(st.Ctim.Sec, st.Ctim.Nsec).After(d.started) {
		d.record(vOld, pid, target)
		return vNone, vWithdrawn, ""
	}
	// The ctime again, and for a SECOND decision rather than the same one
	// twice: this key tells a first exec of this inode-at-this-version from
	// a repeat, so the `chmod +x` case above happening one exec LATER — a
	// metadata-only change to a file already exec'd — has to read cold and
	// not warm. Witnessed by
	// TestAReExecAfterAMetadataOnlyChangeIsColdAgain, which is the only pin
	// these two fields have; the epoch test above has its own
	// (ranger-base-8do8p finding 2).
	key := inode{dev: st.Dev, ino: st.Ino, csec: st.Ctim.Sec, cnsec: st.Ctim.Nsec}
	ch := charge{pid: pid, key: key}
	d.mu.Lock()
	first := !d.seen[key]
	d.seen[key] = true
	h := d.inflight[ch]
	switch {
	case first:
		h = &held{deadline: time.Now().Add(d.delay)}
		d.inflight[ch] = h
	case h != nil:
		// THE RESTART of a charge the kernel cut short, and it is claimed
		// under the same lock that published it, so the handler holding the
		// charge cannot wake between this lookup and this mark.
		h.restarted = true
	}
	d.mu.Unlock()

	switch {
	case first:
		// NOT recorded before the sleep (ranger-base-4kars). The delay
		// rides down to answer() like a hole does, and for the same reason:
		// whether this exec was really held is decided at the answer and
		// nowhere else. A notification the kernel takes back mid-sleep
		// released its caller at the signal rather than at D, so recording
		// the charge here would be the door reporting a delay it did not
		// pay. See the note above recordWithdrawal.
		time.Sleep(time.Until(h.deadline))
		d.mu.Lock()
		delete(d.inflight, ch)
		handedOver := h.restarted
		d.mu.Unlock()
		// What a withdrawal of THIS notification means is now decided by
		// whether a restart claimed the charge: handed on, and the restart
		// is the notification that records the delay — or cut short with
		// nothing to carry it (ranger-base-91fu2). The live verdict is
		// unaffected: if the answer lands after all, this handler held the
		// charge for D and let the exec through, which is a delay whatever
		// arrived behind it.
		taken := vTruncated
		if handedOver {
			taken = vWithdrawn
		}
		return vDelayed, taken, target
	case h != nil:
		// The REMAINDER of the charge this restart inherited, never a fresh
		// D: the charge stays ONE charge — the door's cost model is D once
		// per cold exec (ranger-base-in8dr) — and the execve completes no
		// earlier than its original start plus D.
		time.Sleep(time.Until(h.deadline))
		return vDelayed, vWithdrawn, target
	}
	d.record(vWarm, pid, target)
	return vNone, vWithdrawn, ""
}

// statVerdict tells a path that is not there from a path the door cannot
// see, which look the same through /proc and are not the same thing.
//
// A shell searching PATH execs one candidate per directory and misses most
// of them, so ENOENT is the single commonest answer a door like this gets:
// it is an execve that was going to fail anyway, there is nothing to delay,
// and counting it as a coverage hole would put tens of thousands of
// non-events in the report and bury the real ones. ENOENT on a
// /proc/<pid>/… path, though, can ALSO mean the caller is no longer there
// to resolve it against, so the pid is checked before the verdict is handed
// out.
//
// A pid that is not there is NOT a verdict of its own, which is why there
// is no `gone` here any more: a pid is reusable, and a caller that exec'd
// and exited while we resolved reads exactly like one that died stopped.
// The kernel's answer about THIS notification is ID_VALID and nothing else.
// So an absent pid is an unstattable like any other and goes to answer() as
// a hole — which answer() will record only if the notification was still
// live, i.e. only if the door really did let a live execve through without
// knowing what it was.
func (d *door) statVerdict(pid int, err error) (v verdict, hole bool) {
	if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENOTDIR) {
		if unix.Access(fmt.Sprintf("/proc/%d", pid), unix.F_OK) == nil {
			return vAbsent, false
		}
	}
	return vUnstattable, true
}

// A WITHDRAWAL DURING THE DELAY TRUNCATES THE INJECTION, and this is the
// note for the one verdict that is neither coverage nor a hole
// (ranger-base-4kars).
//
// Read the note above `holes` first: a withdrawal is the kernel taking a
// notification back from a caller that is usually alive, and a caller whose
// handler carries SA_RESTART then RESTARTS the execve and raises a FRESH
// notification. Now put that on a cold plant. adjudicate marks
// `d.seen[key] = true` BEFORE it sleeps, so UNTIL ranger-base-91fu2 the
// restart's notification read the inode as WARM and was answered at once:
// the plant's exec completed in milliseconds while the first handler went
// on sleeping out the full D against a notification nobody was waiting for.
//
// MEASURED 2026-11-11 (this box, linux 7.2.15 x86_64, go1.27.3), D = 600ms,
// by withdrawalFixture with a nudge every 10ms: the plant's exec went
// through at roughly the first nudge, ~10-20ms, against the 600ms the door
// charged it. 40/40 runs identical. Without the nudger the same fixture
// paid the full 600ms. The mark is still where it was and the paragraph
// below is still why the delay is counted at the answer; what changed is
// that the restart no longer reads that mark as the whole story.
//
// So the delay is charged at the ANSWER and nowhere else, which is the rule
// j0riv wrote for the holes applied to the one verdict that was still
// recorded on the way in. `delayed` now counts delays that were HELD and
// let through, and a delay the kernel cut short with nothing to carry it is
// `truncated` — one verdict per notification, and a count that means what
// the report says it means. The exit-3 rule (ADR 0085 D5: zero delayed is a failed door) reads
// the paid count, so a run whose every injection was truncated is a failed
// door and not a clean suite.
//
// AND SINCE ranger-base-91fu2 THE RESTART PAYS IT. The charge is no longer
// the sleeping handler's — it is an entry in `inflight`, keyed by the task
// and the inode and carrying the absolute deadline the handler is sleeping
// to. A notification that matches a live entry IS the restart of that
// charge: the kernel restarts a withdrawn syscall in place, on the same
// task, and a task can only be inside one execve at a time. So the restart
// sleeps the REMAINDER to the original deadline instead of being answered
// warm, and the execve completes no earlier than charge start + D however
// many notifications it spends getting there. The charge stays ONE charge,
// which is the door's cost model (D once per cold exec, additive within 5%
// — ranger-base-in8dr), and the mark stays where it was, so two concurrent
// first execs by DIFFERENT tasks still pay D once between them and nothing
// about that changes.
//
// It is the pid in the key that makes the match unique, so what it rests on
// is the pid being the TASK id and not the thread-group id. MEASURED 2026-11-11
// (same box): a non-leader thread's execve of a plant is adjudicated under
// that thread's own tid, 3 of 3, tid != tgid in all three — the kernel fills
// the field from task_pid_vnr(current). Pinned by
// TestANonLeaderThreadsExecIsAdjudicatedUnderItsOwnTid.
//
// THE ACCOUNTING, which is now one charge over two notifications: the
// cut-short one reads `withdrawn`, naming the plant whose charge it handed
// on, and the restart reads `delayed` once its answer lands. `truncated` is
// what is LEFT — a charge nothing restarted, which is a caller whose
// handler lacks SA_RESTART (EINTR) or none at all (death), and in both of
// those the execve never happens. So after this bead a truncation is no
// longer an exec let through at full speed; it is a charge spent on an exec
// that did not occur, and the one thing it leaves behind is a plant marked
// warm.
//
// MEASURED 2026-11-11 on the arm the census named — the internal/treepins
// family of bd-shim plants, filtered, D = 2.5s, the two doors run back to
// back from this tree: truncations 11 -> 0, delays paid 47 -> 57, and the
// arm's own wall 118.371s -> 145.762s. That +27.4s is 10.96 D, which is the
// injection those charges were supposed to deliver and did not.
//
// WHAT IT COSTS IS NOTIFICATIONS, and the number is bigger than it looks:
// 421 -> 3082 over that run, all 2,661 of the new ones withdrawals of the
// same 11 charges — 243 each, one every ~10ms for the whole 2.5s hold.
// Before this the first restart was answered warm and the execve completed
// at once; now the task stays stopped for D and the Go runtime goes on
// preempting it the whole time, each cycle costing a notification, a
// goroutine, a /proc read and a stat. The deadline is what ends it: the
// storm cannot outlast the charge, and the door answers a restart in ~100µs
// against signals arriving every few ms, so it converges. Over the whole
// door it is some 2,900 extra notifications and at most 30s.
//
// MEASURED over all five arms before the fix: 12 of 2179 charges, 0.55%,
// every one of them a planted `bd` exec'd by
// internal/posse/bdwriteshim.go's syscall.Exec — the one live Go runtime in
// this tree that execve's a plant directly, which is why that family and no
// other can be withdrawn at all (docs/notes.d/ranger-base-4kars.md). That
// is the rate this mechanism converts from `truncated` to `withdrawn` plus
// `delayed`.
//
// WHAT IS STILL NOT HELD, stated rather than left silent: a restart that
// arrives after the deadline has passed finds no entry and is answered warm
// — correctly, since by then the execve has already been in the kernel for
// D. The window for that is the withdrawal happening within microseconds of
// the deadline, and the cost is one line of over-reporting (a `truncated`
// and a `warm` for an exec that did take D), never an exec let through
// early.
func (d *door) recordWithdrawal(n *seccompNotif, taken verdict, pendingTarget string) {
	if pendingTarget == "" {
		// A verdict adjudicate settled itself, so there is no target left
		// to name: the line is the second one this notification spends.
		pendingTarget = "(the kernel took the notification back before the answer)"
	}
	d.record(taken, int(n.Pid), pendingTarget)
}

// answer lets the stopped execve run. CONTINUE and not a return value: the
// kernel performs the original syscall, so the exec'd program, its pid and
// its argv are what they would have been with no door in the way.
//
// ID_VALID first, which is the kernel's own advice: a notification whose
// caller is gone must not be answered, and the id could by then belong to a
// different exec. It is also the ONLY authority on whether this
// notification still exists, which is why adjudicate's holes and its delays
// are referred here rather than recorded up there: a hole is recorded only
// once the kernel has said the notification was live AND the answer has
// landed, because only then has the door really let a live execve through
// without knowing what it was. A notification the kernel took back covered
// nothing and had nothing left to cover (see the note above holes).
//
// And a withdrawal discovered here is TWO different facts depending on what
// this handler was holding — recordWithdrawal above has them.
func (d *door) answer(n *seccompNotif, pending, taken verdict, pendingTarget string) {
	withdrawn := func() { d.recordWithdrawal(n, taken, pendingTarget) }
	id := n.ID
	if err := ioctlPtr(d.listener, unix.SECCOMP_IOCTL_NOTIF_ID_VALID, unsafe.Pointer(&id)); err != nil {
		withdrawn()
		return
	}
	resp := seccompNotifResp{ID: n.ID, Flags: userNotifFlagContinue}
	if err := ioctlPtr(d.listener, unix.SECCOMP_IOCTL_NOTIF_SEND, unsafe.Pointer(&resp)); err != nil {
		if errors.Is(err, unix.ENOENT) {
			// The caller is gone. Nothing to let through.
			withdrawn()
			return
		}
		// The door cannot answer, so every exec from here on would hang in
		// the kernel. Go out loudly instead: closing the listener makes
		// those execs fail with ENOSYS, which a suite reports, while a hang
		// is a 25m timeout with no reason attached.
		fmt.Fprintf(os.Stderr, "coldexec: NOTIF_SEND: %v — the door cannot answer the kernel, so it is going down "+
			"rather than hanging every exec under it\n", err)
		os.Exit(exitRefused)
	}
	if pending != vNone {
		d.record(pending, int(n.Pid), pendingTarget)
	}
}

// readPath reads a NUL-terminated string out of another process's memory.
//
// Chunked at page boundaries because /proc/<pid>/mem answers per page: a
// read that reaches into an unmapped page returns the bytes before it, or
// EIO if the very first page is unmapped, and a path that sits at the end of
// a mapping is an ordinary case rather than a strange one.
//
// It needs PTRACE_MODE_ATTACH on the target, which this process has because
// it is its ancestor — and that is the one thing it is: on a box with
// yama/ptrace_scope=1 (this one) an ancestor is allowed and an unrelated
// process is not. A descendant REPARENTED away when its own parent died
// used to fall out of that reach, which is the hole ranger-base-orgvt
// closed with PR_SET_CHILD_SUBREAPER in supervise: the orphan lands on the
// supervisor instead of past it, so it stays a descendant and its execs
// are adjudicated like any other's. MEASURED 2026-11-11, Linux 7.2.15:
// before the flag, 134 of the internal/treepins arm's 29744 notifications
// were `permission denied` and nothing in cmd/posse's 8427 was; on the
// repro the orphan's plant went from 2-3ms and `unreadable` to 504ms and
// `delayed`.
//
// PR_SET_PTRACER in the child is NOT the fix, and was MEASURED not to be
// (same day, same box): the prctl succeeds, and yama's exception is still
// per-task — it does not extend to the declaring process's descendants, so
// an orphan's own child is refused exactly as before, 5 unreadable of 5.
//
// Whatever is still unread is counted, named in the report, and never
// silently treated as old.
func readPath(pid int, addr uint64) (string, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/mem", pid))
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 0, 256)
	for len(buf) < pathMax {
		off := addr + uint64(len(buf))
		chunk := 4096 - (off % 4096)
		if rem := uint64(pathMax - len(buf)); chunk > rem {
			chunk = rem
		}
		page := make([]byte, chunk)
		n, err := f.ReadAt(page, int64(off))
		if i := bytes.IndexByte(page[:n], 0); i >= 0 {
			return string(append(buf, page[:i]...)), nil
		}
		buf = append(buf, page[:n]...)
		if err != nil {
			return "", fmt.Errorf("reading the path at %#x: %w", addr, err)
		}
	}
	return "", fmt.Errorf("no NUL in %d bytes at %#x", pathMax, addr)
}

// ioctlPtr is ioctl(2) with a pointer argument. x/sys/unix has the request
// numbers and no wrapper that takes a pointer.
func ioctlPtr(fd int, req uint, arg unsafe.Pointer) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}

// reapTick is how often the supervisor looks for orphans to wait on.
//
// A POLL AND NOT A SIGCHLD HANDLER, and the reason is the skip rather than
// the signal: the supervised child's status belongs to os/exec, and the
// only way to leave it there is to name the pids to wait on one at a time,
// which a scan can do and `wait4(-1)` cannot. SIGCHLD would tell us WHEN
// without telling us WHO.
//
// How much that skip is worth was MEASURED and it is less than it looks:
// removing it — reaping the supervised child too — left ARM 9 (the
// command's own exit code reaching `make`) and ARM 15 green. os/exec is
// already blocked in its own wait when the child exits, so the window in
// which a tick could take that status first is microseconds wide and did
// not open in any run here. It is kept anyway, and it is a guard and not a
// pinned behaviour: the door's exit code should not rest on os/exec
// winning a race nothing told it about.
//
// 200ms against a D of 2.5s: the window in which a finished orphan is
// visible as a zombie is a fraction of what the door already adds, and the
// scan below is a handful of reads of /proc/self.
const reapTick = 200 * time.Millisecond

// reapOrphans is the other half of PR_SET_CHILD_SUBREAPER, and it is not
// housekeeping: without it the flag would trade a silent coverage hole for
// a VISIBLE change in what the suite under the door can observe about
// itself.
//
// Being the subreaper makes this process the parent of every orphaned
// descendant, which makes it the only process that can wait on one. An
// orphan that exits unwaited-on is a zombie — it answers `kill -0`, it
// holds a pid, and it shows in `ps` — where under init it would have been
// reaped and gone. That matters here because `kill -0` is this tree's own
// liveness predicate: internal/posse/codextrust_test.go reads ESRCH as
// "ended and reaped" and anything else as a leaked zombie, and
// queuecutover's guard is the same call. A door that turned those reads
// around would red tests whose code is fine, which is the failure ADR 0085
// D1 is written against.
//
// It skips the supervised child, whose status belongs to os/exec — a
// guard, and reapTick says how narrow the race behind it measured.
//
// WHAT IT CANNOT RESTORE is the orphan's ppid, and that is a stated edge
// rather than a bug — see doc.go's "WHAT IT DOES NOT COVER".
func (d *door) reapOrphans(childPid int) {
	t := time.NewTicker(reapTick)
	defer t.Stop()
	for range t.C {
		for _, pid := range ourChildren() {
			if pid == childPid {
				continue
			}
			var ws unix.WaitStatus
			for {
				got, err := unix.Wait4(pid, &ws, unix.WNOHANG, nil)
				if err == unix.EINTR {
					continue
				}
				// got == 0 is "still running", which is every tick until
				// it is not. An error is a pid that is not ours any more.
				if err == nil && got == pid {
					d.reaped.Add(1)
				}
				break
			}
		}
	}
}

// ourChildren lists this process's children, and `children` is a per-TASK
// file, so it reads every thread of it rather than one.
//
// Not because the orphans were measured landing on more than one: the
// kernel reparents onto whichever alive thread of the subreaper
// find_new_reaper() picks, which it finds by walking the thread group from
// its leader, and the leader of a Go process is the main thread and does
// not exit while the process lives. MEASURED 2026-11-11 (ranger-base-orgvt,
// six orphans in one run): 6 of 6 landed on the group leader. The all-task
// glob is here because it is the read that does not DEPEND on that, at the
// cost of a handful of extra reads of /proc/self — and nothing pins the
// difference, so it is a belt and not a claim.
//
// Reading /proc rather than keeping a set of our own, because the whole
// point is the children this process never started.
func ourChildren() []int {
	tasks, err := filepath.Glob("/proc/self/task/*/children")
	if err != nil {
		return nil
	}
	var out []int
	for _, t := range tasks {
		// A thread that exits between the glob and the read is ordinary.
		b, err := os.ReadFile(t)
		if err != nil {
			continue
		}
		for _, f := range strings.Fields(string(b)) {
			if pid, err := strconv.Atoi(f); err == nil {
				out = append(out, pid)
			}
		}
	}
	return out
}

// wait reaps the supervised command and answers the exit code to forward:
// its own, or 128+signal for a death by signal, which is the shell's
// convention and what `make` would have reported without the door.
func wait(cmd *exec.Cmd) int {
	err := cmd.Wait()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
			if ws.Signaled() {
				return 128 + int(ws.Signal())
			}
			return ws.ExitStatus()
		}
		if c := ee.ExitCode(); c >= 0 {
			return c
		}
		return 1
	}
	fmt.Fprintf(os.Stderr, "coldexec: waiting for the supervised command: %v\n", err)
	return 1
}
