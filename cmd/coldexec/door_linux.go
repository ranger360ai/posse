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

	notifs atomic.Int64
	// reaped counts the orphaned descendants the supervisor waited on. It
	// is the subreaper arm's only witness in a real run: before
	// ranger-base-orgvt those processes were reparented away and their
	// execs read `unreadable`, so a non-zero count here is the hole that
	// is no longer there.
	reaped atomic.Int64
	// Every notification lands on exactly one verdict, and the ones that
	// mean "this exec was not covered" are printed at the end rather than
	// left silent: a door that has stopped being able to read its targets
	// must not read as a suite with no members of the class. One verdict
	// per NOTIFICATION, note, not per execve: a restarted execve raises a
	// fresh notification and spends two (see the note above holes).
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

	dr := &door{delay: d, verbose: verbose, seen: map[inode]bool{}}
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
	if dr.verdicts[vDelayed].Load() == 0 {
		fmt.Fprintln(os.Stderr,
			"coldexec: the command passed and the door delayed NOTHING — a run that injected "+
				"nothing says nothing about the class, so this is a failed door and not a clean "+
				"suite (ADR 0085 D5)")
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
			"notification, which is the one this run adjudicated (ranger-base-j0riv)\n", n)
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
	pending, pendingTarget := d.adjudicate(n)
	d.answer(n, pending, pendingTarget)
}

// adjudicate works out what this execve is about and sleeps D if it is the
// first exec of a file the run created.
//
// It records the COVERED verdicts itself and answers vNone for those. A
// HOLE it never records: a hole is an execve the door let through without
// knowing what it was, and whether this one was let through at all is
// decided at the answer and nowhere else — so the verdict rides down to
// answer(), which records it only if the kernel says the notification was
// still live. Every hole here is otherwise load-sensitive in the same way
// and for the same reason: the caller can be unreadable, or its pid gone,
// precisely BECAUSE the kernel withdrew the notification under us
// (ranger-base-j0riv).
func (d *door) adjudicate(n *seccompNotif) (pending verdict, pendingTarget string) {
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
		return vForeign, fmt.Sprintf("syscall %d", n.Data.Nr)
	}

	p, err := readPath(pid, addr)
	if err != nil {
		return vUnreadable, err.Error()
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
		return vNone, ""
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
			return v, target
		}
		d.record(v, pid, target)
		return vNone, ""
	}
	// The ctime, not the mtime: a copied binary keeps its source's mtime
	// and a `chmod +x` moves only the ctime, and both of those are plants.
	if !time.Unix(st.Ctim.Sec, st.Ctim.Nsec).After(d.started) {
		d.record(vOld, pid, target)
		return vNone, ""
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
	d.mu.Lock()
	first := !d.seen[key]
	d.seen[key] = true
	d.mu.Unlock()
	if !first {
		d.record(vWarm, pid, target)
		return vNone, ""
	}
	// Recorded BEFORE the sleep, and so before the answer: this charge is
	// against the inode and it stands even if the notification is withdrawn
	// under us, because the restart that follows reads the inode as warm.
	d.record(vDelayed, pid, target)
	time.Sleep(d.delay)
	return vNone, ""
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

// answer lets the stopped execve run. CONTINUE and not a return value: the
// kernel performs the original syscall, so the exec'd program, its pid and
// its argv are what they would have been with no door in the way.
//
// ID_VALID first, which is the kernel's own advice: a notification whose
// caller is gone must not be answered, and the id could by then belong to a
// different exec. It is also the ONLY authority on whether this
// notification still exists, which is why adjudicate's holes are referred
// here rather than recorded up there: a hole is recorded only once the
// kernel has said the notification was live AND the answer has landed,
// because only then has the door really let a live execve through without
// knowing what it was. A notification the kernel took back covered nothing
// and had nothing left to cover (see the note above holes).
func (d *door) answer(n *seccompNotif, pending verdict, pendingTarget string) {
	withdrawn := func() {
		d.record(vWithdrawn, int(n.Pid), "(the kernel took the notification back before the answer)")
	}
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
