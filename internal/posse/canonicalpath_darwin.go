//go:build darwin

package posse

// kernelPath on darwin is ADR 0082 D1's canonical spelling: open the path
// and ask the kernel what it just opened. `fcntl(fd, F_GETPATH)` fills a
// MAXPATHLEN buffer with ONE name per vnode, which is three normalizations
// in a single reading and is why the ADR chose it over EvalSymlinks:
//
//	symlinks   resolved, as EvalSymlinks also does.
//	CASE       the on-disk spelling. EvalSymlinks keeps the CALLER's case, so
//	           on a case-INsensitive volume (the APFS default) `~/src/proj`
//	           and `~/src/PROJ` resolved to two strings for one directory and
//	           rendered two identity walls — ranger-base-99gww's axis, the
//	           one samePath exists to answer for the COMPARISON.
//	firmlinks  folded. `/System/Volumes/Data/Users/<u>/src/<repo>` is the
//	           same vnode as `/Users/<u>/src/<repo>` and EvalSymlinks hands
//	           the first one straight back (MEASURED, ADR 0082's table).
//
// MEASURED 2026-11-08 (this bead, darwin 25.6.0, go1.26.10 darwin/arm64,
// APFS case-insensitive) over the ADR's own fixtures, reproducing its table
// row for row: `link/Repo`, `real/repo` and `LINK/REPO` all answer
// `<cwd>/real/Repo`; `/tmp` answers `/private/tmp`; `/var/folders` answers
// `/private/var/folders`; an absolute path with no symlink in it comes back
// unchanged; a missing path fails at the open.
//
// THE FCNTL IS STDLIB'S Syscall AND THE REST IS x/sys/unix, which looks
// mixed and is not a choice: F_GETPATH's third argument is a BUFFER, and
// x/sys/unix exports only FcntlInt/FcntlFlock/FcntlFstore on darwin — no
// raw Syscall at all (there is no `func Syscall` in its darwin build). The
// pointer-to-uintptr conversion is written inline in the call, which is the
// one form unsafe's own rules and `go vet`'s unsafeptr check sanction.
//
// O_NONBLOCK IS LOAD-BEARING, not tidiness: this is reached from
// DeriveIdentityLiterals on the launch path, and `open` of a FIFO for read
// with no writer never returns — the exact wedge ranger-base-fvfve fixed one
// call below, at .beads/redirect. identityRedirectTarget's isRegularFile
// guard protects the redirect FILE; nothing protects the DIRECTORY this
// opens, so the flag does.

import (
	"bytes"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func kernelPath(p string) (string, error) {
	fd, err := unix.Open(p, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return "", err
	}
	defer unix.Close(fd)
	buf := make([]byte, unix.PathMax)
	if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(unix.F_GETPATH), uintptr(unsafe.Pointer(&buf[0]))); errno != 0 {
		return "", errno
	}
	if i := bytes.IndexByte(buf, 0); i >= 0 {
		buf = buf[:i]
	}
	return string(buf), nil
}
