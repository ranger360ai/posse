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
	"strconv"
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

// ---------------------------------------------------------------------------
// ranger-base-e5un1: the SAME domain scopes signals.
//
// The seat shares the host's pid namespace (ADR 0080 D17), so this pin can
// name a host pid from inside it. Four readings from one seat, each against a
// control seat that differs by the step alone:
//
//   - kill(<a live host process outside the seat>, 0) is EPERM — and `ok` in
//     the control, so the EPERM is the scope and not a permission the uid
//     never had;
//   - kill(<a pid that names nobody>, 0) is ESRCH in both, so the errno still
//     reads liveness;
//   - the runtime still SIGTERMs a child it fork+exec'd itself, in both;
//   - and the seat still dies with its parent (--die-with-parent is a
//     pdeathsig, a kernel delivery that does not pass the Landlock hook).
//
// ONLY SIGNAL 0 IS AIMED OUTSIDE. The outside process is one this test
// started for the purpose; nothing here can reach a process it did not.
const (
	wuEnvSigOutside = "RHQ_QA_WALLUNIX_SIG_OUTSIDE"
	wuEnvSigAbsent  = "RHQ_QA_WALLUNIX_SIG_ABSENT"
	wuEnvSigPidFile = "RHQ_QA_WALLUNIX_SIG_PIDFILE"
)

// wuSigLiveSkip is wuLiveSkip at the signal half's floor.
func wuSigLiveSkip(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Landlock is a Linux LSM and bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	if n := wallUnixABI(); n < WallUnixSignalABIMin {
		t.Skipf("this kernel's Landlock ABI is %d and SCOPE_SIGNAL needs %d, so there is no signal scope to measure here — `posse gates` reports that as DEGRADED", n, WallUnixSignalABIMin)
	}
}

// wuOutsideAndAbsent starts a host process outside every seat and finds a pid
// that names nobody: one this test started and reaped, re-checked as ESRCH
// from here (pid reuse inside the run would make it name a stranger).
func wuOutsideAndAbsent(t *testing.T) (outside, absent int) {
	t.Helper()
	o := exec.Command("sleep", "120")
	if err := o.Start(); err != nil {
		t.Fatalf("cannot start the outside process: %v", err)
	}
	t.Cleanup(func() { _ = o.Process.Kill(); _ = o.Wait() })
	g := exec.Command("true")
	if err := g.Run(); err != nil {
		t.Fatalf("cannot start the process whose pid is to be absent: %v", err)
	}
	absent = g.ProcessState.Pid()
	if err := syscall.Kill(absent, 0); !errors.Is(err, syscall.ESRCH) {
		t.Skipf("NOTHING MEASURED: pid %d was reaped and is not ESRCH from the host (%v) — reused already", absent, err)
	}
	return o.Process.Pid, absent
}

func TestLiveWallUnixScopesSignalsToTheSeat(t *testing.T) {
	wuSigLiveSkip(t)
	self := posseBinForTest(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	outside, absent := wuOutsideAndAbsent(t)

	for _, c := range []struct {
		name    string
		step    []string
		outside string
	}{
		// The control: if this ever stops saying `ok`, the arm below is not
		// measuring the scope and must not be read as a pass.
		{"control: no Landlock layer on the line", nil, "ok"},
		{"the signal scope", WallUnixStepArgv(self), "EPERM"},
	} {
		t.Run(c.name, func(t *testing.T) {
			argv := append([]string{
				"--die-with-parent",
				"--ro-bind", "/", "/",
				"--dev-bind", "/dev", "/dev",
				"--unshare-user",
			}, c.step...)
			argv = append(argv, exe, "-test.run=^TestWallUnixSignalSeatChild$", "-test.v")
			cmd := exec.Command("bwrap", argv...)
			cmd.Env = wuSeatEnv(
				wuEnvSigOutside+"="+strconv.Itoa(outside),
				wuEnvSigAbsent+"="+strconv.Itoa(absent))
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("the seat exited badly (%v). Seat output:\n%s\nargv: bwrap %s", err, out, strings.Join(argv, " "))
			}
			for _, want := range []string{
				"SEAT-OUTSIDE " + c.outside,
				"SEAT-ABSENT ESRCH",
				"SEAT-OWN-CHILD terminated",
			} {
				if !strings.Contains(string(out), want) {
					t.Errorf("seat %q did not report %q.\nSeat output:\n%s\nargv: bwrap %s", c.name, want, out, strings.Join(argv, " "))
				}
			}
		})
	}

	// And it still dies with its parent. A shell stands between this test and
	// bwrap — the trailing `:` keeps it from exec'ing bwrap in place — and is
	// SIGKILLed; bwrap's pdeathsig, and the one it sets on its own child, must
	// take the seat down with it. The seat is the step-wrapped test binary,
	// which writes its own (host) pid and sleeps.
	t.Run("the seat still dies with its parent", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "seat.pid")
		args := []string{"--die-with-parent", "--ro-bind", "/", "/", "--dev-bind", "/dev", "/dev",
			"--unshare-user", "--bind", filepath.Dir(pidFile), filepath.Dir(pidFile)}
		args = append(args, WallUnixStepArgv(self)...)
		args = append(args, exe, "-test.run=^TestWallUnixSignalSeatChild$")
		sh := exec.Command("sh", append([]string{"-c", `bwrap "$@"; :`, "sh"}, args...)...)
		sh.Env = wuSeatEnv(wuEnvSigPidFile + "=" + pidFile)
		if err := sh.Start(); err != nil {
			t.Fatal(err)
		}
		var seat int
		for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if b, err := os.ReadFile(pidFile); err == nil && strings.HasSuffix(string(b), "\n") {
				seat, _ = strconv.Atoi(strings.TrimSpace(string(b)))
				break
			}
		}
		if seat <= 0 {
			_ = sh.Process.Kill()
			_ = sh.Wait()
			t.Fatal("NOTHING MEASURED: the seat never wrote its pid")
		}
		_ = sh.Process.Kill() // the one pid this test launched
		_ = sh.Wait()
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if wuPidGone(seat) {
				return
			}
		}
		_ = syscall.Kill(seat, syscall.SIGKILL)
		t.Errorf("the seat (pid %d) outlived its parent by 10s with the Landlock layer on the line — --die-with-parent no longer reaches it", seat)
	})
}

// wuPidGone: ESRCH, or a zombie nobody has reaped yet, which is dead.
func wuPidGone(pid int) bool {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return true
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	s := string(b)
	if i := strings.LastIndexByte(s, ')'); i >= 0 && len(s) > i+2 {
		return s[i+2] == 'Z'
	}
	return false
}

// TestWallUnixSignalSeatChild is the seat for the pin above.
func TestWallUnixSignalSeatChild(t *testing.T) {
	if pf := os.Getenv(wuEnvSigPidFile); pf != "" {
		if err := os.WriteFile(pf, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(60 * time.Second)
		return
	}
	o, err1 := strconv.Atoi(os.Getenv(wuEnvSigOutside))
	a, err2 := strconv.Atoi(os.Getenv(wuEnvSigAbsent))
	if err1 != nil || err2 != nil {
		t.Skip("child of TestLiveWallUnixScopesSignalsToTheSeat")
	}
	t.Logf("SEAT-OUTSIDE %s", wuSigErrName(syscall.Kill(o, 0)))
	t.Logf("SEAT-ABSENT %s", wuSigErrName(syscall.Kill(a, 0)))
	own := exec.Command("sleep", "120")
	if err := own.Start(); err != nil {
		t.Fatalf("cannot fork+exec a child inside the seat: %v", err)
	}
	if err := own.Process.Signal(syscall.SIGTERM); err != nil {
		t.Logf("SEAT-OWN-CHILD %s (%v)", wuSigErrName(err), err)
		_ = own.Process.Kill()
		_ = own.Wait()
		return
	}
	_ = own.Wait()
	if ws, ok := own.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() && ws.Signal() == syscall.SIGTERM {
		t.Logf("SEAT-OWN-CHILD terminated")
		return
	}
	t.Logf("SEAT-OWN-CHILD exited without SIGTERM: %v", own.ProcessState)
}

func wuSigErrName(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, syscall.EPERM):
		return "EPERM"
	case errors.Is(err, syscall.ESRCH):
		return "ESRCH"
	}
	return "other"
}

// ---------------------------------------------------------------------------
// ranger-base-p5i2c: the same domain leaves rename(2) and link(2) ALONE.
//
// THE PIN THE BEAD ASKS FOR BY NAME, and the one the 2026-11-13 probe's U11
// row should have been: a Landlock domain that handles any filesystem right
// and not LANDLOCK_ACCESS_FS_REFER denies every rename and link whose source
// and destination directories DIFFER, with EXDEV. U11 renamed within ONE
// directory and read green, so the class shipped — `git branch -m` inside the
// step is exit 128, and through the landed-check judge that reddened every
// tree (ranger-base-ufd7z).
//
// FOUR OPERATIONS AND NOT ONE. Same-directory rename is the row that was
// already green and must stay green; cross-directory rename, cross-directory
// link and the rename of a DIRECTORY across directories are the three the
// class took, and they are listed separately because a partial fix would show
// up as one of them.
//
// AND IT READS THE SOCKET WALL IN THE SAME BREATH, which is what stops the
// cheap false fix: deleting the step, or dropping Access_fs, makes every refer
// row green. So the step arm must ALSO report EACCES on a host socket while
// the control arm connects — the grant is REFER on `/` and nothing anywhere
// grants RESOLVE_UNIX, so restoring rename must not have restored connect.
//
// THE SOCKET IS BOUND BEFORE THE SEAT STARTS, deliberately unlike
// TestLiveWallUnixRefusesAHostSocketBoundAfterTheSeatLaunched: that pin's
// subject is the bind-after case no mount can cover and it pays a handshake
// for it. Here the socket is only the wall's liveness check (U1's case), so
// the cheaper shape is the right one and the handshake would be a second
// thing that can hang.
const (
	wuEnvRefDir  = "RHQ_QA_WALLUNIX_REFER_DIR"
	wuEnvRefSock = "RHQ_QA_WALLUNIX_REFER_SOCK"
)

func TestLiveWallUnixLeavesCrossDirectoryRename(t *testing.T) {
	wuLiveSkip(t)
	self := posseBinForTest(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	const wantRefer = "same=ok cross=ok link=ok dir=ok"
	for _, c := range []struct {
		name string
		step []string
		sock string
	}{
		// The control measures the FIXTURE: a bwrap seat with no Landlock
		// layer at all renames across directories inside its write grant and
		// connects to the socket. If this arm ever stops saying so, the arm
		// below it is measuring the mounts and not the step.
		{"control: no socket wall on the line", nil, "connected"},
		{"the socket wall", WallUnixStepArgv(self), "EACCES"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			sock := filepath.Join(dir, "wall.sock")
			// Bound HERE, by this uncaged process, before the seat exists.
			l, err := net.Listen("unix", sock)
			if err != nil {
				t.Fatalf("the host could not bind the socket the seat will dial: %v", err)
			}
			defer l.Close()
			go func() {
				for {
					conn, err := l.Accept()
					if err != nil {
						return
					}
					conn.Close()
				}
			}()

			argv := append([]string{
				"--die-with-parent",
				"--ro-bind", "/", "/",
				"--dev-bind", "/dev", "/dev",
				"--unshare-user",
				// The write grant the renames happen inside. A seat's
				// cross-directory rename is only ever interesting where the
				// seat can write, and that is a bind and not the ro-bind.
				"--bind", dir, dir,
			}, c.step...)
			argv = append(argv, exe, "-test.run=^TestWallUnixReferSeatChild$", "-test.v")

			var buf bytes.Buffer
			cmd := exec.Command("bwrap", argv...)
			cmd.Env = wuSeatEnv(wuEnvRefDir+"="+dir, wuEnvRefSock+"="+sock)
			cmd.Stdout, cmd.Stderr = &buf, &buf
			if err := cmd.Run(); err != nil {
				t.Fatalf("the seat exited badly (%v). Seat output:\n%s\nargv: bwrap %s",
					err, buf.String(), strings.Join(argv, " "))
			}
			got := buf.String()
			if want := "SEAT-REFER " + wantRefer; !strings.Contains(got, want) {
				t.Errorf("seat %q did not report %q — a cross-directory rename or link was denied behind this seat's wall. A Landlock domain handling any filesystem right without LANDLOCK_ACCESS_FS_REFER answers EXDEV for all three cross-directory rows, and `git branch -m` then fails with `Invalid cross-device link` (ranger-base-p5i2c, ADR 0080 D33).\nSeat output:\n%s\nargv: bwrap %s",
					c.name, want, got, strings.Join(argv, " "))
			}
			if want := "SEAT-REFER-SOCK " + c.sock; !strings.Contains(got, want) {
				t.Errorf("seat %q did not report %q — the refer rows above are only worth reading while the socket wall is still the wall, and this arm says it is not. REFER is granted on %s; RESOLVE_UNIX is granted nowhere (ADR 0080 D28).\nSeat output:\n%s\nargv: bwrap %s",
					c.name, want, wallUnixReferPath, got, strings.Join(argv, " "))
			}
		})
	}
}

// TestWallUnixReferSeatChild is the seat. It renames and links inside its own
// write grant and prints two lines the parent greps for.
//
// IT REPORTS THE ERRNO BY NAME, for wuErrName's reason: EXDEV is this layer,
// EPERM or EACCES would be the mount or the uid, and ENOENT would be a
// fixture that never came up. A pin that could not tell them apart would go
// green on the wrong wall.
//
// `mv` IS NOT USED ANYWHERE HERE, and that is the reason this class shipped:
// coreutils falls back to copy+unlink on EXDEV, so every shell probe of this
// reads green. The syscalls are called directly.
func TestWallUnixReferSeatChild(t *testing.T) {
	dir, sock := os.Getenv(wuEnvRefDir), os.Getenv(wuEnvRefSock)
	if dir == "" {
		t.Skip("child of TestLiveWallUnixLeavesCrossDirectoryRename")
	}
	base := filepath.Join(dir, "ref")
	sub := filepath.Join(base, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("the seat cannot make its own fixture under its write grant: %v", err)
	}
	for _, n := range []string{"f1", "g1"} {
		if err := os.WriteFile(filepath.Join(sub, n), []byte("x"), 0o600); err != nil {
			t.Fatalf("the seat cannot write under its write grant: %v", err)
		}
	}
	if err := os.Mkdir(filepath.Join(sub, "dd"), 0o755); err != nil {
		t.Fatalf("the seat cannot mkdir under its write grant: %v", err)
	}
	var out []string
	for _, r := range []struct {
		name string
		do   func() error
	}{
		{"same", func() error { return os.Rename(filepath.Join(sub, "f1"), filepath.Join(sub, "f2")) }},
		{"cross", func() error { return os.Rename(filepath.Join(sub, "f2"), filepath.Join(base, "f3")) }},
		{"link", func() error { return os.Link(filepath.Join(sub, "g1"), filepath.Join(base, "g2")) }},
		{"dir", func() error { return os.Rename(filepath.Join(sub, "dd"), filepath.Join(base, "dd2")) }},
	} {
		if err := r.do(); err != nil {
			out = append(out, r.name+"="+wuReferErrName(err))
			continue
		}
		out = append(out, r.name+"=ok")
	}
	t.Logf("SEAT-REFER %s", strings.Join(out, " "))

	conn, err := net.Dial("unix", sock)
	if err == nil {
		conn.Close()
		t.Logf("SEAT-REFER-SOCK connected")
		return
	}
	t.Logf("SEAT-REFER-SOCK %s (%v)", wuErrName(err), err)
}

// wuReferErrName is wuErrName for the refer rows: the errnos a rename or link
// can fail with here, each of which means something different about where the
// denial came from.
func wuReferErrName(err error) string {
	switch {
	case errors.Is(err, syscall.EXDEV):
		return "EXDEV"
	case errors.Is(err, syscall.EACCES):
		return "EACCES"
	case errors.Is(err, syscall.EPERM):
		return "EPERM"
	case errors.Is(err, syscall.ENOENT):
		return "ENOENT"
	}
	return "other"
}
