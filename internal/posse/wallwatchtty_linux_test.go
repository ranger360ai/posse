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
