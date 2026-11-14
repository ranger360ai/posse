//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-heypq: whatever was typed at a watched seat and not
// read by it is discarded when the watcher returns, because the next reader
// of that terminal is the pane's own shell, outside the cage. Linux-only for
// the pty ioctls; the watcher itself is Linux-only too (wallwatch_other.go).

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// wwPty opens a pseudo-terminal: the master is what the multiplexer types
// into, the slave is the pane's terminal, which the watcher has as stdin.
func wwPty(t *testing.T) (master, slave *os.File) {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pty on this host: %v", err)
	}
	t.Cleanup(func() { m.Close() })
	if err := unix.IoctlSetPointerInt(int(m.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatalf("unlockpt: %v", err)
	}
	n, err := unix.IoctlGetInt(int(m.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("ptsname: %v", err)
	}
	s, err := os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open the pty's slave: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return m, s
}

// wwPending is how many bytes are waiting to be read on the terminal.
func wwPending(t *testing.T, tty *os.File) int {
	t.Helper()
	n, err := unix.IoctlGetInt(int(tty.Fd()), unix.TIOCINQ)
	if err != nil {
		t.Fatalf("TIOCINQ: %v", err)
	}
	return n
}

// wwType writes a line into the master the way `rex send` + Enter does, and
// waits until the line discipline has it, so the assertion after the watcher
// returns is about the flush and not about a write still in flight.
func wwType(t *testing.T, master, slave *os.File, line string) {
	t.Helper()
	if _, err := master.Write([]byte(line + "\r")); err != nil {
		t.Fatalf("type into the pty: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for wwPending(t, slave) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the typed line never reached the terminal's input queue")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The prompt the incident's shell evaluated carried a command substitution.
const wwTypedPrompt = "Work bead `id` $(id)"

// typingStore is the incident's timing: the keystrokes arrive while the
// watcher is filing its bead, i.e. after the seat is dead and before the
// watcher has returned the terminal to the shell.
type typingStore struct {
	fakeWallWatchStore
	onCreate func()
}

func (s *typingStore) Create(dir string, n BdNew) (string, error) {
	if s.onCreate != nil {
		s.onCreate()
	}
	return s.fakeWallWatchStore.Create(dir, n)
}

// THE INCIDENT, on a real bwrap seat. A wall comes off, the watcher kills the
// seat and goes to file its bead; dispatch types the work prompt in that
// window. When the watcher returns, nothing typed may be left for the shell.
func TestLiveWallWatchDiscardsWhatWasTypedIntoTheKilledSeat(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	master, slave := wwPty(t)
	typed := -1
	st := &typingStore{}
	st.onCreate = func() {
		wwType(t, master, slave, wwTypedPrompt)
		typed = wwPending(t, slave)
	}
	lock := filepath.Join(t.TempDir(), "wall-watch.lock")
	code, _ := s.launchOn(t, st, "seat-heypq", lock, slave)
	bg := s.bg(t)
	renameOver(t, s.deny, false)
	wwWaitKilled(t, code, "deny")
	wwGone(t, bg, "deny")
	if typed <= 0 {
		t.Fatalf("the fixture never typed while the watcher was filing (pending=%d); the pin proves nothing", typed)
	}
	if n := wwPending(t, slave); n != 0 {
		t.Errorf("%d bytes typed into the killed seat are still queued on its terminal, for the pane's uncaged shell to read and run (ranger-base-heypq)", n)
	}
}

// Every return after bwrap starts flushes, including the one where bwrap
// never got as far as a seat. No bwrap needed: a binary that exits at once
// stands in for one that refused its args.
func TestWallWatchDiscardsTypedInputWhenTheSeatNeverStarts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mounts := filepath.Join(dir, "bwrap.mounts")
	if err := os.WriteFile(mounts, wallWatchMountsBytes([]WallWatchMount{{Kind: WallWatchDeny, Path: filepath.Join(dir, "x")}}), 0o644); err != nil {
		t.Fatal(err)
	}
	af, err := os.Open(mounts)
	if err != nil {
		t.Fatal(err)
	}
	defer af.Close()
	master, slave := wwPty(t)
	wwType(t, master, slave, wwTypedPrompt)
	var out strings.Builder
	got := runWallWatch(wallWatchRun{
		Opts:  wallWatchOpts{Mounts: mounts, Seat: "x", Dir: dir, Bwrap: []string{"/bin/false"}},
		Args:  af,
		Stdin: slave, Stdout: &out, Stderr: &out,
		Store: &fakeWallWatchStore{},
	})
	if got == 0 {
		t.Errorf("exit = 0 for a bwrap that never started a seat:\n%s", out.String())
	}
	if n := wwPending(t, slave); n != 0 {
		t.Errorf("%d typed bytes are still queued on the pane's terminal after the watcher returned", n)
	}
}

// ranger-base-69vw5: the terminal's input queue holds 4096 bytes and every
// dispatch work prompt is larger, so a prompt typed at a dead seat is still
// mid-write when the watcher returns. A flush empties the queue and lets the
// rest in behind it; the watcher has to keep discarding until the sender
// stops.

// wwBigPrompt is a prompt five times the queue, its tail the part the
// incident's shell evaluated, its Enter included.
func wwBigPrompt() string {
	var b strings.Builder
	for b.Len() < 20000 {
		b.WriteString("a line of bead text, long enough to fill a queue fast\n")
	}
	b.WriteString("Done: `bd close x` $(id)\r")
	return b.String()
}

// wwTypeInFlight types p into the master from a goroutine, the way a sender
// mid-write does, with the master's output drained so echo cannot stall the
// line discipline. It returns once the terminal's queue is full and the rest
// is waiting behind it; the channel closes when the whole write has landed.
func wwTypeInFlight(t *testing.T, master, slave *os.File, p string) <-chan struct{} {
	t.Helper()
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := master.Read(buf); err != nil {
				return
			}
		}
	}()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = master.Write([]byte(p))
	}()
	deadline := time.Now().Add(5 * time.Second)
	for wwPending(t, slave) < 3900 {
		if time.Now().After(deadline) {
			t.Fatalf("the terminal's queue never filled (pending=%d): the pin would not be testing a prompt larger than the queue", wwPending(t, slave))
		}
		time.Sleep(2 * time.Millisecond)
	}
	return done
}

// wwNothingReachesTheShell asserts the whole prompt is gone: the write has
// landed, and nothing of it is queued for the next reader, now or a moment
// later.
func wwNothingReachesTheShell(t *testing.T, done <-chan struct{}, slave *os.File) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("the prompt is still being written after the watcher returned; the pane's uncaged shell is its next reader (ranger-base-69vw5), %d bytes queued", wwPending(t, slave))
	}
	for end := time.Now().Add(300 * time.Millisecond); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if n := wwPending(t, slave); n != 0 {
			t.Fatalf("%d bytes of a prompt larger than the terminal's queue reached the pane's shell after the watcher returned (ranger-base-69vw5)", n)
		}
	}
}

func TestWallWatchDiscardsAPromptLargerThanTheTerminalQueue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mounts := filepath.Join(dir, "bwrap.mounts")
	if err := os.WriteFile(mounts, wallWatchMountsBytes([]WallWatchMount{{Kind: WallWatchDeny, Path: filepath.Join(dir, "x")}}), 0o644); err != nil {
		t.Fatal(err)
	}
	af, err := os.Open(mounts)
	if err != nil {
		t.Fatal(err)
	}
	defer af.Close()
	master, slave := wwPty(t)
	before, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	done := wwTypeInFlight(t, master, slave, wwBigPrompt())
	var out strings.Builder
	runWallWatch(wallWatchRun{
		Opts:  wallWatchOpts{Mounts: mounts, Seat: "x", Dir: dir, Bwrap: []string{"/bin/false"}},
		Args:  af,
		Stdin: slave, Stdout: &out, Stderr: &out,
		Store: &fakeWallWatchStore{},
	})
	wwNothingReachesTheShell(t, done, slave)
	after, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	if after.Lflag != before.Lflag || after.Cc != before.Cc {
		t.Errorf("the drain left the pane's terminal changed: lflag %#x → %#x, cc %v → %v", before.Lflag, after.Lflag, before.Cc, after.Cc)
	}
}

// THE INCIDENT at a real prompt's size, on a real bwrap seat: the prompt is
// typed while the watcher files its bead and is still mid-write when it
// returns.
func TestLiveWallWatchDiscardsAPromptLargerThanTheTerminalQueue(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	master, slave := wwPty(t)
	var done <-chan struct{}
	st := &typingStore{}
	st.onCreate = func() { done = wwTypeInFlight(t, master, slave, wwBigPrompt()) }
	lock := filepath.Join(t.TempDir(), "wall-watch.lock")
	code, _ := s.launchOn(t, st, "seat-69vw5", lock, slave)
	bg := s.bg(t)
	renameOver(t, s.deny, false)
	wwWaitKilled(t, code, "deny")
	wwGone(t, bg, "deny")
	if done == nil {
		t.Fatal("the fixture never typed while the watcher was filing; the pin proves nothing")
	}
	wwNothingReachesTheShell(t, done, slave)
}
