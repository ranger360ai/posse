//go:build linux

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// delayEnv is where `make coldexec-check` puts D — the same variable the
// content-rewrite door read, so the Makefile's switch does not move
// (ADR 0085 D6).
const delayEnv = "POSSE_COLD_EXEC_DELAY"

// defaultDelay is ADR 0085 D4's D, above the measured darwin worst case of
// 2.17s: a deadline that survives 2.5s of cold exec survives darwin.
// ASSUMED that 2.17s tail is the true worst; if darwin ever measures worse,
// raise this. It is one constant.
const defaultDelay = 2500 * time.Millisecond

// Exit codes, which the pins read and a person reading `make` output should
// be able to tell apart:
//
//	2  the door refused to start, or could not stay up — NOTHING ran under
//	   it, or the run is no longer covered. Never the command's own code.
//	3  the run delayed nothing. ADR 0085 D5: zero delayed is a failed door
//	   and not a clean suite, because a door that injects nothing is
//	   indistinguishable from a suite with no members of the class.
//
// Anything else is the command's own exit code, forwarded.
const (
	exitRefused     = 2
	exitNoneDelayed = 3
)

// The listener-fd socket's slot in the child. os/exec dup2s ExtraFiles to
// fd 3 upward, and dup2 clears close-on-exec — which is why the child must
// close it by hand before exec'ing, or the whole suite would inherit it.
const childSockFD = 3

func main() {
	delay := flag.Duration("delay", 0, "D: how long to hold the first execve of an inode created after the door started (default "+delayEnv+", else "+defaultDelay.String()+")")
	verbose := flag.Bool("verbose", false, "print one line per execve notification: the verdict, the pid and the path (triage; an arm takes tens of thousands)")
	child := flag.Bool("child", false, "internal: install the filter, hand the listener up to the parent, exec the argv")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"usage: coldexec [-delay D] -- <argv…>\n\n"+
				"Runs <argv…> with the FIRST execve of every file created after this\n"+
				"moment delayed by D, for every process underneath it, at any depth\n"+
				"(ADR 0085 D2). D defaults to %s, else %s.\n\n",
			delayEnv, defaultDelay)
		flag.PrintDefaults()
	}
	flag.Parse()

	argv := flag.Args()
	if len(argv) == 0 {
		flag.Usage()
		os.Exit(exitRefused)
	}

	if *child {
		// Never returns: it execs argv or it says why and exits.
		runChild(argv)
	}

	d := *delay
	if d == 0 {
		var err error
		if d, err = envDelay(); err != nil {
			refuse("%v", err)
		}
	}
	if d < 0 {
		refuse("-delay %s is negative", d)
	}
	os.Exit(supervise(d, *verbose, argv))
}

// envDelay reads D from the environment. A value that is not a duration is
// REFUSED rather than replaced with the default, which is the opposite of
// what _suite_lock_slots does with POSSE_SUITE_SLOTS and for the opposite
// reason: that variable tunes a queue, and a typo in it should not stop a
// suite, while this one is the door's whole subject. A run whose injected
// delay nobody can read is a run whose verdict nobody can read.
func envDelay() (time.Duration, error) {
	v, ok := os.LookupEnv(delayEnv)
	if !ok || v == "" {
		return defaultDelay, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a duration", delayEnv, v)
	}
	if d < 0 {
		return 0, fmt.Errorf("%s=%q is negative", delayEnv, v)
	}
	return d, nil
}

// consumeDelayEnv takes POSSE_COLD_EXEC_DELAY out of THIS process's
// environment, which is how it is kept out of every process under the door:
// the child inherits this environment and hands it on, so there is no
// `cmd.Env` anywhere in the supervisor and no second copy of the
// environment to keep right.
//
// The strip is load-bearing while the old content-rewrite door is still in
// the tree (it goes under ranger-base-7zkxw, ADR 0085 D3). That door is
// installed by internal/posse's TestMain whenever it sees this variable, so
// leaving it in the environment would run BOTH doors over one suite: the
// shim rewriting the bytes of every plant and this supervisor delaying the
// shim's own exec. The variable names the supervisor's D, and the
// supervisor is the only process that should read it.
//
// Called after D has been read, which is the whole reason it is a separate
// line: there is no state in which the variable is gone and D is unknown.
func consumeDelayEnv() {
	os.Unsetenv(delayEnv)
}

// refuse is every "the door is not up" exit: loud, on stderr, exit 2.
func refuse(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "coldexec: "+format+" — NOTHING RAN UNDER THE DOOR\n", a...)
	os.Exit(exitRefused)
}

// ─── the child: install the filter, hand it up, exec ─────────────────────────

// runChild is the supervised process before it becomes the command. It never
// returns.
//
// Everything here happens on ONE locked OS thread, and that is the whole
// reason no TSYNC flag is needed: a seccomp filter installed without TSYNC
// binds the calling thread alone, execve keeps only the calling thread, and
// the filter survives into the new program and into every descendant of it.
// runtime.LockOSThread is what makes "the calling thread" the same thread
// for the prctl, the seccomp and the exec — the Go runtime would otherwise
// be free to move this goroutine between them.
func runChild(argv []string) {
	runtime.LockOSThread()

	// Resolved BEFORE the filter goes in, so a typo in the command line is
	// an ordinary error message and not a door failure.
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "coldexec: %v\n", err)
		os.Exit(exitRefused)
	}

	arch, err := auditArch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "coldexec: %v\n", err)
		os.Exit(exitRefused)
	}

	// no_new_privs first: seccomp(SET_MODE_FILTER) requires it from a
	// process without CAP_SYS_ADMIN. It is per-task, inherited across fork
	// and exec, and on this tree's own measurement it changes nothing the
	// suite does — bwrap is not setuid on this host, and the bwrap tests
	// pass under the door (ADR 0085 D2's MEASURED list).
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		fmt.Fprintf(os.Stderr, "coldexec: PR_SET_NO_NEW_PRIVS: %v\n", err)
		os.Exit(exitRefused)
	}

	prog := filterProgram(arch)
	fd, errno := seccompSetFilter(unix.SECCOMP_FILTER_FLAG_NEW_LISTENER, &prog)
	if errno != 0 {
		switch errno {
		case unix.EBUSY:
			// MEASURED (ADR 0085 D5): the kernel allows ONE user-notification
			// listener per filter chain, so a coldexec under a coldexec —
			// or under anything else that installs one — fails here. Loudly,
			// and the command does not run: a second door that silently
			// shared the first one's listener would delay nothing and say
			// nothing.
			fmt.Fprintln(os.Stderr,
				"coldexec: seccomp(SET_MODE_FILTER, NEW_LISTENER): EBUSY — a seccomp "+
					"user-notification listener is already installed in this filter chain. "+
					"The kernel allows one. This is a coldexec inside a coldexec, or another "+
					"supervisor above it; run the command under exactly one (ADR 0085 D5).")
		case unix.EINVAL:
			fmt.Fprintln(os.Stderr,
				"coldexec: seccomp(SET_MODE_FILTER, NEW_LISTENER): EINVAL — this kernel does "+
					"not take a notification listener. The door needs Linux 5.5 or later "+
					"(ADR 0085 D5).")
		default:
			fmt.Fprintf(os.Stderr, "coldexec: seccomp(SET_MODE_FILTER, NEW_LISTENER): %v\n", errno)
		}
		os.Exit(exitRefused)
	}

	if err := sendListener(childSockFD, fd); err != nil {
		fmt.Fprintf(os.Stderr, "coldexec: handing the listener to the supervisor: %v\n", err)
		os.Exit(exitRefused)
	}
	// Closed by hand: dup2 cleared close-on-exec when os/exec placed it, so
	// without this the command and every process under it would carry an
	// open socket to its own supervisor on fd 3.
	unix.Close(childSockFD)
	unix.Close(fd)

	// The first execve of this process is the command's own, and it is
	// notified like any other. The supervisor is already in its receive
	// loop: the fd was sent before this line, so it is in the socket's
	// buffer whatever this exec does.
	err = unix.Exec(bin, argv, os.Environ())
	fmt.Fprintf(os.Stderr, "coldexec: exec %s: %v\n", bin, err)
	os.Exit(126)
}

// sendListener hands one fd up the socketpair with SCM_RIGHTS. The byte is
// there because a control message needs at least one byte of payload to
// travel with.
func sendListener(sock, fd int) error {
	rights := unix.UnixRights(fd)
	for {
		err := unix.Sendmsg(sock, []byte{0}, rights, nil, 0)
		if err == unix.EINTR {
			continue
		}
		return err
	}
}

// recvListener is the other end: the parent's blocking receive of that fd.
func recvListener(sock int) (int, error) {
	buf := make([]byte, 1)
	oob := make([]byte, unix.CmsgSpace(4))
	for {
		n, oobn, _, _, err := unix.Recvmsg(sock, buf, oob, 0)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return -1, err
		}
		if n == 0 && oobn == 0 {
			return -1, fmt.Errorf("the child closed the socket without sending a listener")
		}
		scms, err := unix.ParseSocketControlMessage(oob[:oobn])
		if err != nil {
			return -1, fmt.Errorf("parsing the control message: %w", err)
		}
		if len(scms) != 1 {
			return -1, fmt.Errorf("got %d control messages, want 1", len(scms))
		}
		fds, err := unix.ParseUnixRights(&scms[0])
		if err != nil {
			return -1, fmt.Errorf("parsing SCM_RIGHTS: %w", err)
		}
		if len(fds) != 1 {
			return -1, fmt.Errorf("got %d file descriptors, want 1", len(fds))
		}
		return fds[0], nil
	}
}

// ─── the filter ──────────────────────────────────────────────────────────────

// The AUDIT_ARCH_* values seccomp_data.arch carries. x/sys/unix does not
// export them, and they are the kernel's own numbers (linux/audit.h).
const (
	auditArchX8664   = 0xc000003e // AUDIT_ARCH_X86_64
	auditArchAArch64 = 0xc00000b7 // AUDIT_ARCH_AARCH64
)

// userNotifFlagContinue is SECCOMP_USER_NOTIF_FLAG_CONTINUE (Linux 5.5),
// the answer that tells the kernel to run the syscall it stopped. Not in
// x/sys/unix either.
const userNotifFlagContinue = 1

// Offsets into struct seccomp_data, which the filter addresses as a byte
// array: nr first, arch second.
const (
	offsetNr   = 0
	offsetArch = 4
)

// auditArch answers the arch value this binary's syscall numbers belong to,
// and REFUSES on anything else rather than installing a filter whose
// execve number means a different call (ADR 0085 D5: it never runs a suite
// uncovered).
func auditArch() (uint32, error) {
	switch runtime.GOARCH {
	case "amd64":
		return auditArchX8664, nil
	case "arm64":
		return auditArchAArch64, nil
	default:
		return 0, fmt.Errorf("GOARCH=%s is an architecture this door does not name: "+
			"the filter has to match seccomp_data.arch against the arch whose execve "+
			"number it tests, and naming the wrong one would delay some other syscall "+
			"(ADR 0085 D5)", runtime.GOARCH)
	}
}

// filterProgram builds the classic-BPF program the filter runs for every
// syscall:
//
//	if arch != ours                   -> ALLOW
//	if nr == execve || nr == execveat -> USER_NOTIF
//	otherwise                         -> ALLOW
//
// The arch test is not decoration. A 32-bit process under a 64-bit kernel
// arrives with a different arch and a different numbering — i386's syscall
// 59 is not execve — so a filter without it would hold the wrong call. Such
// a process is ALLOWed through untouched, which is the edge ADR 0085 D5
// names: its execs are not covered, and nothing silently misreads them.
func filterProgram(arch uint32) unix.SockFprog {
	// Jump targets are relative to the instruction AFTER the jump, so each
	// `jf`/`jt` below counts the instructions to skip.
	insns := []unix.SockFilter{
		// 0: A = seccomp_data.arch
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: offsetArch},
		// 1: if A != arch, skip to 5 (ALLOW)
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: arch, Jt: 0, Jf: 3},
		// 2: A = seccomp_data.nr
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: offsetNr},
		// 3: if A == execve, skip to 6 (USER_NOTIF)
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(unix.SYS_EXECVE), Jt: 2, Jf: 0},
		// 4: if A == execveat, skip to 6 (USER_NOTIF)
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: uint32(unix.SYS_EXECVEAT), Jt: 1, Jf: 0},
		// 5: ALLOW
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW},
		// 6: USER_NOTIF
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_USER_NOTIF},
	}
	return unix.SockFprog{Len: uint16(len(insns)), Filter: &insns[0]}
}

// seccompSetFilter is seccomp(2) with SECCOMP_SET_MODE_FILTER. With
// NEW_LISTENER the return value is the listener fd. x/sys/unix has the
// constants and no wrapper.
func seccompSetFilter(flags uintptr, prog *unix.SockFprog) (int, unix.Errno) {
	r, _, errno := unix.Syscall(unix.SYS_SECCOMP, uintptr(unix.SECCOMP_SET_MODE_FILTER), flags, uintptr(unsafe.Pointer(prog)))
	return int(r), errno
}
