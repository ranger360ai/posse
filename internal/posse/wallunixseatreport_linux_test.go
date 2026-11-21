//go:build !posse_arm2 && !posse_arm3

package posse

// The one row of wuSeatSelfReport that is a syscall, split out by GOOS for
// wallunix_linux.go's reason: internal/posse builds and its whole test suite
// runs on darwin, and `unix.Syscall` of a Linux-only number does not compile
// there. The seat itself is never off Linux — bwrap is the Linux mechanism of
// the seatbelt tier (ADR 0080 D16) — so the darwin twin is a compile-time
// answer and says so (ranger-base-8m9d0).
//
// THE SUFFIX IS THE CONSTRAINT, not a second `//go:build` clause: Go reads
// the GOOS off the filename, and internal/treepins/armtags_qa_test.go's
// armPlatformAxis represents exactly the two GOOSes ci.yml runs, so the pair
// reads as one arm-1 file per matrix leg rather than as a file nothing
// builds. The `//go:build` line carries the ARM and nothing else, which is
// why it is spelled the same as wallunixlive_test.go's.

import (
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"
)

// wuSeatPdeathsig is the seat's own PR_GET_PDEATHSIG, as the number a
// `kill -l` would name, or a bracketed errno.
//
// IT WRITES THROUGH arg2 AND IS NOT A RETURNING PRCTL, which is the one thing
// easy to get wrong here: unix.PrctlRetInt is for the options that return
// their value in the syscall's own result, and PR_GET_PDEATHSIG is not one of
// them — the kernel does `put_user(..., (int __user *)arg2)` and returns 0. So
// the argument is a pointer to a 32-bit int, which is what `int` is on the
// kernel's side of that put_user on every arch this tree builds for, and a Go
// `int` (64-bit) passed here would leave its upper half untouched garbage.
//
// unix.Syscall AND NOT unix.Prctl, for the unsafe.Pointer rule: the
// conversion to uintptr has to happen in the call expression to the syscall
// itself, which is the pattern wallunix_linux.go's two Landlock calls already
// use and the one `go vet`'s unsafeptr check reads.
func wuSeatPdeathsig() string {
	var sig int32
	if _, _, errno := unix.Syscall(unix.SYS_PRCTL, unix.PR_GET_PDEATHSIG,
		uintptr(unsafe.Pointer(&sig)), 0); errno != 0 {
		return "<" + errno.Error() + ">"
	}
	return strconv.Itoa(int(sig))
}
