//go:build !posse_arm2 && !posse_arm3

package posse

// The LIVE half of ranger-base-6rndn's pins, and the shape the bead asks for
// by name: an uncaged process binds a unix socket AFTER the seat launched,
// the seat tries to connect, and it gets EACCES — with a control arm, the
// same seat and the same socket without the step, that CONNECTS.
//
// WHY THE CONTROL ARM IS THE POINT. A connect that fails proves nothing on
// its own: it fails the same way when the path is wrong, when the listener
// died, when a mount hid the directory, and when the fixture never came up.
// ADR 0080 D21's T6 is this repo's worked example of a cell that reported
// `refused` and could not say which (ranger-base-bjk4k), so this pin measures
// the DIFFERENCE between two runs that differ in one word of argv.
//
// WHY THE SOCKET IS BOUND AFTER THE SEAT STARTS. That is the half of
// ranger-base-l2gk9 no bwrap mount can cover: a socket absent at launch has
// nothing to bind over, and one the host unlinks and re-binds detaches the
// mount that was over it (l2gk9's own readings). A pin that bound the socket
// first would pass against a wall that only holds for sockets present at
// launch, which is the wall we already had.
//
// WHY THE SCRATCH DIR IS A WRITE GRANT. Same reason: a socket inside a
// directory the seat must be able to WRITE is the residual this layer exists
// for, because a tmpfs over that directory would empty the grant. Every other
// host socket directory is already answered by ranger-base-ludpp's hide.
//
// THE SEAT IS THIS TEST BINARY RE-EXEC'D (cagehomelock_qa_test.go's shape),
// so the pin plants no executable: ADR 0085's cold-exec class is about a test
// that writes an executable and then execs it under a deadline, and a test
// binary that already exists is not one.

import (
	"bytes"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// wuLiveSkip is the one gate, written as bgLiveSkip is: the question is
// whether this host HAS the pieces, so the binary is the bare name.
func wuLiveSkip(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Landlock is a Linux LSM and bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16); the darwin half is its own bead and its own mechanism")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	if n := wallUnixABI(); n < WallUnixABIMin {
		t.Skipf("this kernel's Landlock ABI is %d and RESOLVE_UNIX needs %d, so there is no socket wall to measure here — which is exactly the case `posse gates` reports as DEGRADED", n, WallUnixABIMin)
	}
}

// wuEnvSock and wuEnvReady are how the parent tells the re-exec'd child what
// to wait for and what to dial.
const (
	wuEnvSock  = "RHQ_QA_WALLUNIX_SOCK"
	wuEnvReady = "RHQ_QA_WALLUNIX_READY"
)

func TestLiveWallUnixRefusesAHostSocketBoundAfterTheSeatLaunched(t *testing.T) {
	wuLiveSkip(t)
	self := posseBinForTest(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name string
		step []string
		want string
	}{
		// The control runs the seat with no step at all. If this arm ever
		// stops saying `connected`, the arm below it has stopped measuring
		// anything and must not be read as a pass.
		{"control: no socket wall on the line", nil, "connected"},
		{"the socket wall", WallUnixStepArgv(self), "EACCES"},
	} {
		t.Run(c.name, func(t *testing.T) {
			// A dir per arm: the second arm must not meet the first arm's
			// socket, and a fresh mktemp per iteration is cheaper than
			// reasoning about removing one (AGENTS.md).
			dir := t.TempDir()
			sock := filepath.Join(dir, "leader.sock")
			ready := filepath.Join(dir, "ready")

			argv := append([]string{
				"--die-with-parent",
				"--ro-bind", "/", "/",
				"--dev-bind", "/dev", "/dev",
				"--unshare-user",
				// The write grant the socket will appear inside.
				"--bind", dir, dir,
			}, c.step...)
			argv = append(argv, exe, "-test.run=^TestWallUnixSeatChild$", "-test.v")

			var buf bytes.Buffer
			cmd := exec.Command("bwrap", argv...)
			cmd.Env = wuSeatEnv(wuEnvSock+"="+sock, wuEnvReady+"="+ready)
			cmd.Stdout, cmd.Stderr = &buf, &buf
			if err := cmd.Start(); err != nil {
				t.Fatalf("the seat did not start (bwrap %s): %v", strings.Join(argv, " "), err)
			}

			// THE HOST BINDS NOW, uncaged: this test process, which no
			// Landlock domain covers and which is outside the seat's mount
			// namespace. The seat is already up and waiting on `ready`.
			l := wuBindAfter(t, sock, ready)
			defer l.Close()

			if err := cmd.Wait(); err != nil {
				t.Fatalf("the seat exited badly (%v). Seat output:\n%s\nargv: bwrap %s",
					err, buf.String(), strings.Join(argv, " "))
			}
			got := buf.String()
			if !strings.Contains(got, "SEAT-RESULT "+c.want) {
				t.Errorf("seat %q did not report %q.\nSeat output:\n%s\nargv: bwrap %s",
					c.name, c.want, got, strings.Join(argv, " "))
			}
		})
	}
}

// TestWallUnixSeatChild is the seat. It waits for the host to bind, dials
// once, and prints one line the parent greps for.
//
// IT REPORTS THE ERRNO BY NAME and not merely "refused", for the reason the
// control arm exists: EACCES is this layer, ENOENT is a mount that hid the
// directory, and ECONNREFUSED is a listener that went away. A pin that could
// not tell them apart would go green on the wrong wall.
func TestWallUnixSeatChild(t *testing.T) {
	sock, ready := os.Getenv(wuEnvSock), os.Getenv(wuEnvReady)
	if sock == "" {
		t.Skip("child of TestLiveWallUnixRefusesAHostSocketBoundAfterTheSeatLaunched")
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	conn, err := net.Dial("unix", sock)
	if err == nil {
		conn.Close()
		t.Logf("SEAT-RESULT connected")
		return
	}
	t.Logf("SEAT-RESULT %s (%v)", wuErrName(err), err)
}

// wuSeatEnv is this process's environment with the SUITE's own markers shed,
// plus what the seat is told.
//
// A SEAT IS NOT A SUITE, and this is the one place in this pin where that
// matters (landedchecklive_test.go's reading, same cause): this package's
// TestMain turns ANY posse test binary carrying RHQ_FAKE_HERDR into a fake
// bd, and the seat IS this test binary re-exec'd. Inherited, the marker
// crosses into the seat, and the child answers `fake bd: unhandled
// -test.run=…` and exits 1 — which it did on this pin's first run, in both
// arms, so the control arm would have reported a connect that never happened.
// Shed for the CHILD and not process-wide, because the arms need this process
// to stay a test.
func wuSeatEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "RHQ_FAKE_") || strings.HasPrefix(kv, "RHQ_QA_WALLUNIX_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

// wuErrName names the errno the way the probe's rows do.
//
// errors.Is AND NOT os.IsPermission, which is what this was first written
// with: net.Dial returns a *net.OpError wrapping the syscall error, and the
// legacy os.Is* predicates do not unwrap, so a real EACCES read as "other"
// and the pin failed over a wall that was working. The errnos are compared
// directly because they are what the kernel documents for this right —
// "a resolution attempt is denied [with] an EACCES error" (linux/landlock.h)
// — and because telling EACCES from ENOENT and ECONNREFUSED is the whole job
// of this function (a mount that hid the directory, and a listener that went
// away, are the two other ways a connect fails in this fixture).
func wuErrName(err error) string {
	switch {
	case errors.Is(err, syscall.EACCES):
		return "EACCES"
	case errors.Is(err, syscall.ENOENT):
		return "ENOENT"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "ECONNREFUSED"
	}
	return "other"
}

// wuBindAfter binds the socket and raises the flag the seat is waiting on.
// The settle is not a race the pin depends on: the seat waits for `ready`,
// which is written after the bind, so binding late is safe and binding early
// would only weaken the measurement.
func wuBindAfter(t *testing.T, sock, ready string) net.Listener {
	t.Helper()
	time.Sleep(300 * time.Millisecond)
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("the host could not bind the socket the seat is waiting for: %v", err)
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	if err := os.WriteFile(ready, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return l
}

// posseBinForTest builds this tree's posse into the test's own dir. The pin
// needs the binary under measurement and not whatever `make install` last put
// on PATH: the verb may not be in that one at all, and a pin that passed
// against an installed binary would be measuring another tree.
func posseBinForTest(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "posse")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/ranger360ai/posse/cmd/posse")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build this tree's posse for the seat (%v): %s", err, out)
	}
	return bin
}
