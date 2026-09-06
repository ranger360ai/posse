package posse

import (
	"io/fs"
	"os"
	"syscall"
)

// Writing a file that something is about to exec is a race, and the race is
// ours. Between the open-for-write and the close, any goroutine in this
// process that forks hands the child a duplicate of that write descriptor.
// O_CLOEXEC closes it — but only AT execve, so for the child's whole
// fork-to-exec window the file still has a writer, and Linux answers execve
// on a file with a writer with ETXTBSY. That is golang/go#22315, and it is
// not hypothetical here: ci.yml run 34002511879 on ubuntu-latest reds with
//
//	fork/exec /tmp/TestProbeSessionExeSeesThroughTheGatePrefix.../herdr:
//	text file busy
//
// on a fake binary a test had just written (ranger-base-d26ak). darwin does
// not enforce the check at all — measured 2026-09-06, an execve succeeds
// there with our own write descriptor still open on the file — which is why
// ubuntu-latest is the only job that has ever shown it, and why no amount of
// running the suite on this box would have.
//
// syscall.ForkLock is the lock that answer already exists for. Its doc
// promises the fork is done holding it for writing; the two platforms keep
// that promise differently, and it is worth being exact, because the one
// that enforces ETXTBSY is the one that does not take it per fork
// (go1.26.5, syscall/exec_unix.go:199 acquireForkLock). darwin's
// forkpipe.go is a bare ForkLock.Lock() every time. linux's forkpipe2.go is
// reference-counted: it takes ForkLock.Lock() only when the count of forks
// in flight goes 0 -> 1, and a fork that overlaps one already running just
// increments the count and forks without touching the lock. Either way the
// lock is held for the whole span in which any fork is in flight, which is
// the property this file needs: a writer that holds it across open..close
// cannot be forked over, because every fork either completed before the
// descriptor existed or starts after it is gone. Once the file is closed no
// process holds a writer on it and it is safe to exec forever after, which
// is why the fix belongs at the write and nothing is needed at the exec.
//
// The cost is that no fork/exec in this process runs during the write. That
// is the point, and a write of a few hundred bytes is the shortest thing in
// the process worth queueing behind.
//
// linux's refcount has a second cost that nothing here has observed and
// nothing pins: acquireForkLock yields to waiting READERS only
// (hasWaitingReaders), never to a waiting writer, so an unbroken train of
// overlapping forks never lets the count reach 0 and a writer waiting on
// ForkLock.Lock() waits behind all of it. This tree's parallel test binary
// is exactly that traffic. If a write through here ever appears to hang on
// linux, this is the first thing to suspect (ranger-base-ctkhp).
func underForkLock(write func() error) error {
	syscall.ForkLock.Lock()
	defer syscall.ForkLock.Unlock()
	return write()
}

// WriteExecutable writes a file that will be exec'd, without leaving a window
// in which a sibling fork can inherit the write descriptor. It is os.WriteFile
// in every other respect — same truncate-or-create, same perm, same error.
//
// Use it for anything with an exec bit: a gate shim, a hook script, a fake
// binary a test writes and runs. os.WriteFile is still right for data.
//
// With one caveat, and it is ForkLock's own (syscall/exec_unix.go:36-41):
// "Some system calls that create new file descriptors can block for
// arbitrarily long times: open on a hung NFS server or named pipe ... We
// can't reasonably grab the lock across those operations." This function
// does exactly that, so the hold is only as bounded as the open and the
// write are, and while it is held no fork in the process can start. An
// ordinary file on local disk — every call site in this tree — is bounded.
// A named pipe is not, and that is not theoretical: the pins in
// execwrite_test.go park this call on a FIFO for as long as they like, both
// in open(2) and inside write(2). Do not point it at a path that can block.
func WriteExecutable(path string, content []byte, perm fs.FileMode) error {
	return underForkLock(func() error { return os.WriteFile(path, content, perm) })
}
