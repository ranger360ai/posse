//go:build linux

package posse

// The two syscalls behind wallunix.go, and nothing else in this file. Split
// out by GOOS rather than guarded inside, because internal/posse builds and
// its whole test suite runs on darwin: a `unix.Syscall` of a Linux-only
// number does not compile there, and the render half has to stay readable
// from the box that cannot run it (wall.go's wallMechanismHost).

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// liveWallUnixABI asks the kernel for the highest Landlock ABI it supports,
// which is what `landlock_create_ruleset(NULL, 0, LANDLOCK_CREATE_RULESET_VERSION)`
// returns. A negative number means the syscall itself is absent — no
// CONFIG_SECURITY_LANDLOCK, or a kernel older than 5.13 — which is a
// different fact from "version 0" and is reported as such.
//
// NOT MEMOIZED, and deliberately unlike liveBwrapApplyRefusal's sync.Once:
// that probe forks a bwrap, and this is one syscall that allocates no fd when
// asked for the version. There is nothing to amortize and a memo would only
// be a thing a pin cannot re-arm.
func liveWallUnixABI() int {
	n, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0,
		uintptr(unix.LANDLOCK_CREATE_RULESET_VERSION))
	if errno != 0 {
		return -int(errno)
	}
	return int(n)
}

// liveArmWallUnixPdeathsig sets SIGKILL as the CALLING THREAD's parent-death
// signal — the same value bwrap's --die-with-parent sets, put back after an
// exec transition cleared it (RunWallUnix says when that is,
// ranger-base-6ec6n). Per-task, so RunWallUnix calls it on its locked leader.
func liveArmWallUnixPdeathsig() error {
	return unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGKILL), 0, 0, 0)
}

// landlockAccessFSRefer is spelled in wallunix.go so the ruleset pin reads it
// from darwin. This is the assertion that the hand-spelled bit is the kernel's,
// made where x/sys has the header's own value: a drift makes the Linux build
// fail to compile, rather than rendering a ruleset that handles a right the
// kernel does not know and refuses outright (EINVAL at create_ruleset, which
// RunWallUnix turns into a seat that will not open).
var _ [0]struct{} = [landlockAccessFSRefer - unix.LANDLOCK_ACCESS_FS_REFER]struct{}{}

// liveWallUnixTask is the calling thread's id and this process's id. They are
// equal on, and only on, the thread group leader: the kernel gives the leader
// a tid equal to the tgid, and `gettid` is the one way a thread can read which
// one it is.
//
// TWO SYSCALLS AND NOT os.Getpid()'S CACHE, deliberately: `getpid` here is the
// same number either way, and reading both through x/sys keeps the pair one
// kernel reading rather than one kernel reading compared against a Go runtime
// value. unix.Gettid is Linux-only in x/sys, which is why this seam is split
// by GOOS at all.
func liveWallUnixTask() (tid, pid int) { return unix.Gettid(), unix.Getpid() }

// liveRestrictWallUnix applies the domain to the CALLING THREAD: one layer
// handling RESOLVE_UNIX and REFER and no other filesystem right, with one rule
// added — REFER on `/` — so every host pathname socket in this seat's view is
// refused, a socket the seat makes for itself is not, and rename(2) and link(2)
// are left exactly as the mounts leave them. It also scopes SIGNAL, so the seat
// can signal only what it forked (wallUnixRuleset says which of these this ABI
// gets).
//
// THE CALLING THREAD, AND NOT THE THREAD GROUP, and that correction is
// ranger-base-vxz74: this comment used to say "thread group", which is false
// of `landlock_restrict_self(fd, 0)` — the domain reaches the siblings only
// with LANDLOCK_RESTRICT_SELF_TSYNC. What makes a per-thread apply sound is
// RunWallUnix's lock and leader check, which hold this goroutine on the
// leader from before the restrict until the exec, and `execve` then keeps the
// one thread that is in the domain (de_thread). cmd/coldexec's runChild is the
// same shape for the same reason, one layer down, with seccomp in place of
// Landlock.
//
// NO TSYNC FLAG, and that is a decision and not an omission (ADR 0080 D39).
// It would be belt-and-braces over a window the lock has already closed, it
// cannot carry the pdeathsig — which is per-task and set on one task only, so
// no flag of any syscall can propagate it — and it is gated: the flag is
// (1U << 3) of `landlock_restrict_self`'s flags in this box's
// linux/landlock.h, this tree has never measured which Landlock ABI
// introduced it, and an unknown flag is EINVAL. Passed ungated, it would turn
// every seat on a kernel that does not have it into a seat that does not open
// — a refusal where today there is a working wall.
//
// THE RULE IS ADDED BEFORE restrict_self AND AFTER create_ruleset, which is the
// only window the syscall has: a rule is added to a ruleset FD, and the FD stops
// accepting rules the moment the domain is enforced. It is added only when
// wallUnixReferGrant is non-zero — the ABI 6-8 arm handles no filesystem right,
// so it has nothing to grant and `/` needs no open there.
//
// O_PATH|O_CLOEXEC ON THE PARENT, as landlock_add_rule's path_beneath attr
// requires: O_PATH because the rule needs a reference to the directory and not
// a readable handle on it, and O_CLOEXEC because this process is about to
// become the runtime and the runtime has no use for a descriptor on `/`.
//
// THE ATTR IS A CONVERSION OF wallUnixRuleset'S, never a second spelling of
// its fields: the two structs are identical by name and type, so there is no
// line here that could drop `Scoped` while the plan the pins read still
// carries it, and a change to x/sys's layout is a compile error rather than a
// ruleset that silently handles less. The size passed is the full v6+ struct
// (24 bytes), which every kernel this step is typed for knows.
//
// PR_SET_NO_NEW_PRIVS FIRST, because landlock_restrict_self requires it of an
// unprivileged caller and answers EPERM without it. It is also the right
// thing on its own terms for a process that is about to become the runtime.
//
// THE RULESET FD IS CLOSED ON EVERY PATH. The domain outlives it — it is held
// by the task, not by the descriptor — and leaking it into the exec'd runtime
// would hand the seat a handle to its own ruleset for no purpose. `unsafe` is
// here for the one thing it is for: the attr struct's address, as the syscall
// ABI defines the argument.
func liveRestrictWallUnix(abi int) error {
	attr := unix.LandlockRulesetAttr(wallUnixRuleset(abi))
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return errno
	}
	defer unix.Close(int(fd))
	if grant := wallUnixReferGrant(abi); grant != 0 {
		if err := wallUnixAddReferRule(int(fd), grant); err != nil {
			return err
		}
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, fd, 0, 0); errno != 0 {
		return errno
	}
	return nil
}

// wallUnixAddReferRule grants `grant` on wallUnixReferPath in the ruleset the
// fd names. Split out of the apply so the one syscall pair that can fail for a
// reason of its own — the open, and the add — reports which.
//
// THE PARENT FD IS CLOSED HERE and not deferred to the apply: it is wanted for
// the length of one syscall, and the apply's own `defer` on the ruleset fd is
// the only descriptor that has to outlive a statement.
//
// AN ERROR IS NOT SWALLOWED. RunWallUnix refuses the seat on any error from
// the apply (ADR 0080 D4), and that is the right answer here too: a ruleset
// that was created but could not be granted REFER, enforced anyway, is a seat
// whose rename(2) is silently EXDEV — the exact failure this function exists
// to end, arriving as a mystery instead of a refusal.
func wallUnixAddReferRule(fd int, grant uint64) error {
	parent, err := unix.Open(wallUnixReferPath, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	attr := unix.LandlockPathBeneathAttr{Allowed_access: grant, Parent_fd: int32(parent)}
	_, _, errno := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, uintptr(fd),
		uintptr(unix.LANDLOCK_RULE_PATH_BENEATH), uintptr(unsafe.Pointer(&attr)), 0, 0, 0)
	if cerr := unix.Close(parent); cerr != nil && errno == 0 {
		return cerr
	}
	if errno != 0 {
		return errno
	}
	return nil
}
