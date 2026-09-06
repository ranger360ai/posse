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
// syscall.ForkLock is the lock that answer already exists for. Its own doc
// says the fork is done holding it for writing, so a writer that holds it
// across open..close cannot be forked over: every fork either completed
// before the descriptor existed or starts after it is gone. Once the file is
// closed no process holds a writer on it and it is safe to exec forever
// after, which is why the fix belongs at the write and nothing is needed at
// the exec.
//
// The cost is that no fork/exec in this process runs during the write. That
// is the point, and a write of a few hundred bytes is the shortest thing in
// the process worth queueing behind.
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
func WriteExecutable(path string, content []byte, perm fs.FileMode) error {
	return underForkLock(func() error { return os.WriteFile(path, content, perm) })
}
