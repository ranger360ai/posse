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
	"encoding/binary"
	"os"
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
//
// IT READS THE CALLING THREAD, AND THAT IS NOT THE ROW THE REPORT WANTS
// (ranger-base-z56s9). pdeath_signal is per-task: bwrap sets it on the one
// thread that then execs the seat, which becomes the seat's thread-group
// leader, and copy_process zeroes it on every thread the Go runtime clones
// afterwards. A test function runs on whatever M its goroutine landed on, so
// a read from there says 0 on any non-leader thread while the leader still
// holds 9. MEASURED 2026-11-21 on D1 (Linux 7.2.15, go1.27.3, bubblewrap
// 0.13.1): a Go seat under `bwrap --die-with-parent --unshare-user` read 9 on
// its leader and 0 on each of three locked non-leader threads. The first two
// ubuntu-26.04 logs carrying this report read 0 in BOTH arms, and nothing in
// them said which thread that was. wuSeatLeaderPdeathsig is the leader's row;
// this one stays beside it, labelled with its tid, so a log can show the two
// disagreeing.
func wuSeatPdeathsig() string {
	return wuPrGetPdeathsig() + " (tid " + strconv.Itoa(unix.Gettid()) + ", pid " + strconv.Itoa(unix.Getpid()) + ")"
}

func wuPrGetPdeathsig() string {
	var sig int32
	if _, _, errno := unix.Syscall(unix.SYS_PRCTL, unix.PR_GET_PDEATHSIG,
		uintptr(unsafe.Pointer(&sig)), 0); errno != 0 {
		return "<" + errno.Error() + ">"
	}
	return strconv.Itoa(int(sig))
}

// wuSeatLeaderPdeathsig is PR_GET_PDEATHSIG read during package
// initialisation, which runtime.main runs with the main goroutine locked to
// the main OS thread — the thread-group leader, the one bwrap's prctl and
// execve ran on. The row carries its own proof: it says `leader` only when
// gettid()==getpid() at the moment of the read, and `NOT THE LEADER` with
// both ids otherwise, so a runtime that ever stops locking init cannot pass
// off a non-leader read as this row. Every arm-1 test binary pays one prctl
// for it; only the seat prints it.
var wuSeatLeaderPdeathsig = func() string {
	v := wuPrGetPdeathsig()
	if tid, pid := unix.Gettid(), unix.Getpid(); tid != pid {
		return v + " (NOT THE LEADER: read on tid " + strconv.Itoa(tid) + " of pid " + strconv.Itoa(pid) + ")"
	}
	return v + " (leader, read at init)"
}()

// wuSeatATSecure is the seat's AT_SECURE auxv entry: 1 means the kernel ran
// this exec as a secure exec, and begin_new_exec zeroes pdeath_signal on
// exactly that condition. On ubuntu the only candidate is AppArmor's exec
// transition into bwrap//&unpriv_bwrap: security/apparmor/domain.c sets
// secureexec when the matched x-rule lacks AA_X_UNSAFE, and the resolute
// profile's rule is a lowercase (unsafe) `pix`, which by that source does not.
// This row says whether the source and the runner agree.
//
// Read from /proc/self/auxv because the pinned x/sys has no Getauxval: the
// file is (type, value) pairs of native words, AT_SECURE is type 23
// (include/uapi/linux/auxvec.h) and AT_NULL (0) ends it.
func wuSeatATSecure() string {
	const atSecure = 23
	b, err := os.ReadFile("/proc/self/auxv")
	if err != nil {
		return "<" + err.Error() + ">"
	}
	w := int(unsafe.Sizeof(uintptr(0)))
	word := func(p []byte) uint64 {
		if w == 4 {
			return uint64(binary.NativeEndian.Uint32(p))
		}
		return binary.NativeEndian.Uint64(p)
	}
	for i := 0; i+2*w <= len(b); i += 2 * w {
		switch word(b[i:]) {
		case 0:
			return "<absent from auxv>"
		case atSecure:
			return strconv.FormatUint(word(b[i+w:]), 10)
		}
	}
	return "<absent from auxv>"
}
