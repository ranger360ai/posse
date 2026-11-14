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

// landlockAccessFSResolveUnix is LANDLOCK_ACCESS_FS_RESOLVE_UNIX
// (linux/landlock.h, ABI 9+): "Look up pathname UNIX domain sockets ... this
// restricts both calls to connect(2) as well as calls to sendmsg(2) with an
// explicit recipient address", and it "applies only to connections to UNIX
// server sockets which were created outside of the newly created Landlock
// domain". Spelled here because golang.org/x/sys v0.47.0 carries the ruleset
// structs and the syscall numbers but not this constant.
const landlockAccessFSResolveUnix = 1 << 16

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

// restrictResolveUnix applies the domain to the calling thread group: one
// layer handling RESOLVE_UNIX and no other right, with NO rule added, so
// every host pathname socket in this seat's view is refused and a socket the
// seat makes for itself is not.
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
func restrictResolveUnix() error {
	attr := unix.LandlockRulesetAttr{Access_fs: landlockAccessFSResolveUnix}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return errno
	}
	defer unix.Close(int(fd))
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, fd, 0, 0); errno != 0 {
		return errno
	}
	return nil
}
