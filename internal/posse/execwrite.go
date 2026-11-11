package posse

import (
	"io/fs"
	"os"
	"runtime"
	"strings"
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
// ordinary file on local disk is bounded; a named pipe is not, and that is
// not theoretical. Every call site in this tree writes an ordinary file
// except this function's own FIFO pins in execwrite_test.go, which park
// it on a pipe for as long as they like — deliberately, both in open(2) and
// inside write(2). Do not point it at a path that can block.
func WriteExecutable(path string, content []byte, perm fs.FileMode) error {
	if coldExecRewrite != nil && callerIsTestFile(2) {
		content = coldExecRewrite(path, content)
	}
	return underForkLock(func() error { return os.WriteFile(path, content, perm) })
}

// coldExecRewrite is the cold-first-exec injector's one seam into this file
// (ADR 0085 D2), and it is nil in every shipped binary. Nothing in cmd/ or
// internal/ assigns it; the only assignment in the tree is in
// herdr_test.go's TestMain, under POSSE_COLD_EXEC_DELAY, and `make
// coldexec-check` is the one command that sets that variable.
//
// WHY THE INJECTION IS HERE AND NOT IN A CENSUS. The execwrite pins
// (execwritegates_qa_test.go, execwritesiblings_qa_test.go) already force
// every executable write in the tree through this function, so the set of
// files a test can plant and then exec is closed by a gate that already
// exists. A hook here reaches all of them and needs no census of its own —
// which is the whole of ADR 0085's argument against a static matcher, whose
// census left 10 of 14 rows undecidable (docs/notes.d/ranger-base-aru4c.md).
//
// The rewrite runs OUTSIDE underForkLock, and must: it only builds bytes —
// no open, no stat, no fork — so it adds nothing to the span in which no
// fork in this process may start (the paragraph above). The marker file the
// injected shim keys on is created by the SHIM, in the child, at exec time.
var coldExecRewrite func(path string, content []byte) []byte

// callerIsTestFile reports whether the frame `skip` levels above this
// function was compiled from a _test.go file. It is the whole of ADR 0085
// D3's "test-only by construction" on the write side: a shim that PRODUCTION
// code writes keeps its exact bytes even with the hook installed, so the
// content read-back pins over rendered gates, hooks and probes cannot red
// for the injector's reason rather than their own.
//
// skip is counted from this function, so WriteExecutable passes 2: 0 is
// here, 1 is WriteExecutable, 2 is whoever called it. runtime.Caller
// expands inlined frames, so it answers about the source line and not about
// what the compiler did with it.
//
// It is the IMMEDIATE caller and deliberately not a walk of the stack. A
// test that plants through a production helper is writing production bytes
// at a production call site, and the file that site writes is the file the
// product writes — if its first exec needs warming, that is the product's
// deadline to fix and not a fixture's.
func callerIsTestFile(skip int) bool {
	_, file, _, ok := runtime.Caller(skip)
	return ok && strings.HasSuffix(file, "_test.go")
}
