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

// wuSunPath is AF_UNIX's cap, NUL included, so the longest pathname a bind
// takes is one byte less. wallchecklive_test.go's wallLiveSunPath is the same
// number for arm 2's two gnupg-socket members; it is written twice because
// the two files are in different build arms and neither compiles with the
// other.
const wuSunPath = 108

// wuSockDir is the scratch directory these two pins bind a unix socket
// inside, and it is shortTempDir and NOT t.TempDir() for one measured reason:
// a t.TempDir() name carries the TEST's own name — 73 bytes for the pin below
// — so the path is over wuSunPath as soon as TMPDIR is not a 4-byte /tmp, and
// it is the HOST's own net.Listen that fails:
//
//	listen unix <TMPDIR>/TestLiveWallUnixRefuses…cont<10 digits>/001/leader.sock: bind: invalid argument
//
// MEASURED 2026-11-21 (ranger-base-vbp5a), Linux 7.2.15-1-omarchy, go1.27.3:
// green with TMPDIR unset, and BOTH ARMS of both pins failed at that listen
// inside ADR 0097 D1's venue, whose TMPDIR is
// `<state>/landed-check/run-<10 digits>/tmp` (62 bytes on this box). That is
// worse than a red: the control arm dies with the step arm, so the pin cannot
// even read as a false pass — which is the one shape this file's header says
// the control arm exists to prevent. ADR 0096 D3 makes that venue where these
// live pins measure, and there they measured nothing.
//
// THE LENGTH IS ASSERTED RATHER THAN SKIPPED OVER, which is the one
// difference from wallchecklive_test.go's budget: that one guards a root an
// operator may pass in, while shortTempDir's root is a literal `/tmp`, so no
// environment can push this path over the cap and the only thing left that
// can is a rename inside this file. A skip would go dark for a tree bug.
func wuSockDir(t *testing.T, name string) (dir, sock string) {
	t.Helper()
	dir = shortTempDir(t)
	sock = filepath.Join(dir, name)
	if len(sock) >= wuSunPath {
		t.Fatalf("NOTHING MEASURED: the socket path %s is %d bytes and sun_path is %d, so the host's own listen would answer `bind: invalid argument` and BOTH arms of this pin would die with it — a red that is this fixture's path and not the wall (ranger-base-vbp5a)",
			sock, len(sock), wuSunPath)
	}
	return dir, sock
}

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
			// reasoning about removing one (AGENTS.md). It is wuSockDir's
			// short root because the socket below has to fit in sun_path
			// before anything else here can be true.
			dir, sock := wuSockDir(t, "leader.sock")
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
//   - and the seat still dies with its parent, with a control arm that has
//     no Landlock layer on the line at all.
//
// WHY THE DEATH ARM IS EXPECTED TO PASS, written correctly this time. The
// first draft said `--die-with-parent` is "a kernel delivery that does not
// pass the Landlock hook", and that is false: a pdeathsig goes through
// security_task_kill like any other signal (do_exit ->
// forget_original_parent -> group_send_sig_info -> check_kill_permission ->
// security_task_kill). What makes Landlock allow it is WHOSE signal it is.
// SCOPE_SIGNAL restricts signals sent FROM a domain, and the sender here is
// the DYING PARENT — the outer bwrap, which is outside the step and carries
// no Landlock domain — so the seat's own domain is never consulted. The seat
// is the TARGET, and a Landlock domain does not protect its holder from the
// outside.
//
// WHICH IS WHY THE DEATH ARM NEEDS A CONTROL AND A SELF-REPORT
// (ranger-base-8m9d0). The arm reds every ubuntu-26.04 arm-1 job
// (ranger-base-z56s9), and a one-shape arm that reds can only say "the seat
// outlived its parent" — it cannot say whether the pdeathsig was CLEARED
// before the seat ran or SET and never DELIVERED, nor whether a plain
// `bwrap --die-with-parent --unshare-user` seat on that runner dies either.
// So the arm runs twice over one body, the control differing by the step
// alone, and the seat writes what it can read about itself beside its pid:
// its PR_GET_PDEATHSIG, its ppid, its NoNewPrivs/Seccomp rows and its
// AppArmor label. The parent prints that for BOTH arms, pass or fail,
// because nobody has a shell on that runner and the CI log is the only
// instrument. wuSeatSelfReport names how to read the four outcomes.
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

	// And it still dies with its parent — in TWO arms over one body, the
	// control differing by the step alone, each failing on its own and naming
	// itself. A shell stands between this test and bwrap — the trailing `:`
	// keeps it from exec'ing bwrap in place — and is SIGKILLed; bwrap's
	// pdeathsig, and the one it sets on its own child, must take the seat down
	// with it. The seat is the test binary, which writes its pid and its
	// self-report and sleeps.
	//
	// BOTH ARMS' REPORTS ARE COLLECTED HERE AND DUMPED BY WHICHEVER ARM REDS,
	// which is not belt-and-braces over the t.Logf below: ci.yml runs `make
	// test-arm1` and `make test-arm1` passes no `-v`, so a PASSING arm's
	// t.Logf never reaches the CI log. Three of the four readings in
	// wuSeatSelfReport's table are about how the two arms DIFFER, so a log
	// carrying only the failing arm's report cannot reach them. The control
	// runs first, the arms are independent subtests, and the dump after the
	// loop fires once either of them has failed.
	var (
		deaths    []string
		anyArmRed bool
	)
	for _, c := range []struct {
		name string
		step []string
	}{
		// The control: a red here is not the Landlock layer, because this arm
		// has no Landlock layer. On a host where AppArmor moves bwrap's exec
		// into another label it cannot die with its parent at all (the
		// transition clears the pdeathsig, ranger-base-6ec6n), so there it
		// asserts that platform fact instead of waiting; elsewhere a red is
		// the runner — bwrap, or the credentials.
		{"control: no Landlock layer on the line, and it still dies with its parent", nil},
		{"the seat still dies with its parent", WallUnixStepArgv(self)},
	} {
		t.Run(c.name, func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "seat.pid")
			args := []string{"--die-with-parent", "--ro-bind", "/", "/", "--dev-bind", "/dev", "/dev",
				"--unshare-user", "--bind", filepath.Dir(pidFile), filepath.Dir(pidFile)}
			args = append(args, c.step...)
			args = append(args, exe, "-test.run=^TestWallUnixSignalSeatChild$")
			sh := exec.Command("sh", append([]string{"-c", `bwrap "$@"; :`, "sh"}, args...)...)
			sh.Env = wuSeatEnv(wuEnvSigPidFile + "=" + pidFile)
			if err := sh.Start(); err != nil {
				t.Fatal(err)
			}
			var (
				seat   int
				report string
			)
			for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
				b, err := os.ReadFile(pidFile)
				if err != nil || !strings.HasSuffix(string(b), wuSeatReportEnd) {
					continue
				}
				report = string(b)
				seat = wuSeatReportPid(report)
				break
			}
			if seat <= 0 {
				_ = sh.Process.Kill()
				_ = sh.Wait()
				// Before the Fatalf, which Goexits: an arm that measured
				// nothing is still an arm whose SIBLING's report is wanted.
				anyArmRed = true
				t.Fatalf("NOTHING MEASURED in %q: the seat never wrote a complete self-report (%d bytes so far):\n%s\nargv: bwrap %s",
					c.name, len(report), report, strings.Join(args, " "))
			}
			// Unconditionally, pass or fail: this is the instrument the CI log
			// is read with, and a `-v` run on a box with a shell is how its
			// rows are learnt.
			t.Logf("seat self-report (%s):\n%s", c.name, report)
			deaths = append(deaths, c.name+":\n"+report)
			// THE STEP RE-ARMS IT (ranger-base-6ec6n), so its leader reads 9
			// whatever the exec into it cleared. On an AppArmor host this row
			// is the measurement of that fix's ASSUMED half: that the step's
			// own exec into the runtime stays in the label and does not
			// clear it a second time.
			if c.step != nil {
				if got := wuLeaderPdeathsig(report); got != "9" {
					anyArmRed = true
					t.Errorf("%s: the seat's leader reads pdeathsig %q, want 9 — `posse %s` re-arms it after bwrap's exec, so a 0 here is the step's own exec into the runtime clearing it again (an AppArmor label change on that exec too) or a step that did not run (ranger-base-6ec6n)", c.name, got, WallUnixVerb)
				}
			}
			// THE CONTROL ASSERTS THE PLATFORM (ranger-base-6ec6n). Where
			// AppArmor moved bwrap's exec into another label,
			// apparmor_bprm_committing_creds zeroed the pdeathsig bwrap had
			// just set, and NOTHING on a bwrap-only line can put it back —
			// so this arm cannot die with its parent there, and a 10s red
			// would be a reading of the platform, every run. It says so and
			// asserts what the platform does instead: pdeathsig 0. A label
			// change that left 9 in place falls through to the death check,
			// which is the stronger claim.
			if c.step == nil {
				if from, into, moved := wuAppArmorTransition(report); moved {
					if got := wuLeaderPdeathsig(report); got == "0" {
						t.Logf("%s: PLATFORM — AppArmor moved bwrap's exec from %q into %q, and that transition cleared the pdeathsig bwrap set (apparmor_bprm_committing_creds, ranger-base-6ec6n): the leader reads 0, so a seat with no step cannot die with its parent on this host. Asserted as 0, not waited on; the step arm is the one that must die.", c.name, from, into)
						_ = sh.Process.Kill()
						_ = sh.Wait()
						_ = syscall.Kill(seat, syscall.SIGKILL) // the seat this arm started
						if outer := wuSeatReportInt(report, "ppid "); outer > 0 && !wuPidGone(outer) {
							_ = syscall.Kill(outer, syscall.SIGKILL)
						}
						return
					}
					t.Logf("%s: AppArmor moved bwrap's exec from %q into %q and the leader still reads pdeathsig %q — that is not the clear ranger-base-6ec6n read from source, so this arm is held to dying with its parent", c.name, from, into, wuLeaderPdeathsig(report))
				}
			}
			_ = sh.Process.Kill() // the one pid this test launched
			_ = sh.Wait()
			for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
				if wuPidGone(seat) {
					return
				}
			}
			// WHICH LINK BROKE (ranger-base-z56s9). There are two pdeathsigs
			// on this line and the seat can only report one: the sh's death
			// must take the outer bwrap (its own --die-with-parent, set after
			// raw_clone with no exec after it), and the outer bwrap's death
			// must take the seat. The seat's ppid when it reported IS that
			// outer bwrap, so read it now, before the cleanup kill below.
			outer := wuSeatReportInt(report, "ppid ")
			outerState := "<no ppid row>"
			if outer > 0 {
				outerState = "GONE: the sh's death took the outer bwrap, so the link that broke is the outer bwrap's death not taking the seat"
				if !wuPidGone(outer) {
					outerState = "STILL ALIVE: the outer bwrap outlived the SIGKILLed sh, so the link that broke is ONE LAYER UP, bwrap's own --die-with-parent, and the seat's rows are not where the fault is"
				}
			}
			seatPpidNow := wuProcPpid(seat)
			_ = syscall.Kill(seat, syscall.SIGKILL)
			if outer > 0 && !wuPidGone(outer) {
				// Ours: the sh this test launched forked it, and a seat-less
				// bwrap left behind would hold the box. Kill the pid, never a
				// pattern.
				_ = syscall.Kill(outer, syscall.SIGKILL)
			}
			anyArmRed = true
			t.Errorf("%s: the seat (pid %d) outlived its SIGKILLed parent by 10s — --die-with-parent no longer reaches it.\nouter bwrap (pid %d, the seat's ppid when it reported): %s\nthe seat's ppid after the 10s: %s\nRead the seat's own report before blaming a layer:\n%s\nargv: bwrap %s",
				c.name, seat, outer, outerState, seatPpidNow, report, strings.Join(args, " "))
		})
	}
	if anyArmRed {
		t.Errorf("a death arm failed, so here is EVERY arm's self-report, the passing ones included — `make test-arm1` passes no -v, and three of the four readings are about how the arms differ (wuSeatSelfReport):\n%s",
			strings.Join(deaths, "\n"))
	}
}

// TestLiveWallUnixStepReArmsAClearedDieWithParent is ranger-base-6ec6n's live
// pin, and it is the one half of that fix this box CAN measure: D1 has no
// AppArmor, so no exec here clears bwrap's pdeathsig. What AppArmor's
// transition leaves the step is a task with pdeath_signal 0 — and that is
// exactly what a step started by a plain fork+exec, with no bwrap in front of
// it, gets. So the launcher here is a shell and nothing else, and the arms
// differ by the step alone:
//
//   - control, no step: the agent's leader reads pdeathsig 0 and OUTLIVES its
//     SIGKILLed shell. That is the fixture modelling the wipe; if the control
//     ever dies, the step arm below is not measuring the re-arm.
//   - the step: the leader reads 9 and dies with the shell.
//
// The ubuntu leg reads the same re-arm through bwrap and the real transition
// (TestLiveWallUnixScopesSignalsToTheSeat's step arm).
func TestLiveWallUnixStepReArmsAClearedDieWithParent(t *testing.T) {
	wuSigLiveSkip(t)
	self := posseBinForTest(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name      string
		step      []string
		pdeathsig string
		dies      bool
	}{
		{"control: no step, so nothing set a pdeathsig and the seat outlives its launcher", nil, "0", false},
		{"the step re-arms it, and the seat dies with its launcher", WallUnixStepArgv(self), "9", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "seat.pid")
			args := append(append([]string{}, c.step...), exe, "-test.run=^TestWallUnixSignalSeatChild$")
			sh := exec.Command("sh", append([]string{"-c", `"$@"; :`, "sh"}, args...)...)
			sh.Env = wuSeatEnv(wuEnvSigPidFile + "=" + pidFile)
			if err := sh.Start(); err != nil {
				t.Fatal(err)
			}
			var report string
			seat := 0
			for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
				b, err := os.ReadFile(pidFile)
				if err == nil && strings.HasSuffix(string(b), wuSeatReportEnd) {
					report, seat = string(b), wuSeatReportPid(string(b))
					break
				}
			}
			if seat <= 0 {
				_ = sh.Process.Kill()
				_ = sh.Wait()
				t.Fatalf("NOTHING MEASURED in %q: the seat never wrote a complete self-report:\n%s\nargv: sh -c ... %s", c.name, report, strings.Join(args, " "))
			}
			t.Cleanup(func() { _ = syscall.Kill(seat, syscall.SIGKILL) }) // the one pid this arm's seat wrote
			if got := wuLeaderPdeathsig(report); got != c.pdeathsig {
				t.Errorf("%s: the seat's leader reads pdeathsig %q, want %q.\n%s", c.name, got, c.pdeathsig, report)
			}
			_ = sh.Process.Kill()
			_ = sh.Wait()
			gone := false
			for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
				if gone = wuPidGone(seat); gone {
					break
				}
			}
			switch {
			case c.dies && !gone:
				t.Errorf("%s: the seat (pid %d) outlived its SIGKILLed launcher by 5s — the step did not re-arm its die-with-parent (ranger-base-6ec6n).\n%s", c.name, seat, report)
			case !c.dies && gone:
				t.Errorf("%s: the seat (pid %d) died with its launcher although nothing set it a pdeathsig — this fixture no longer models a cleared die-with-parent, so the step arm is not measuring the re-arm.\n%s", c.name, seat, report)
			}
		})
	}
}

// wuAppArmorTransition reads whether the exec from the outer bwrap into the
// seat changed AppArmor label: the outer bwrap's label (the report's ppid,
// read live — the caller has not killed it yet) against the seat's own
// `apparmor` row. moved needs POSITIVE evidence on both sides — either label
// unreadable (D1 answers EINVAL: no LSM there answers the file) is "no
// transition", and so is a seat that landed unconfined, which
// apparmor_bprm_committing_creds leaves alone.
func wuAppArmorTransition(report string) (from, into string, moved bool) {
	outer := wuSeatReportInt(report, "ppid ")
	if outer <= 0 {
		return "", "", false
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(outer) + "/attr/current")
	if err != nil {
		return "", "", false
	}
	from = strings.TrimRight(string(b), "\x00\n")
	for _, line := range strings.Split(report, "\n") {
		if rest, ok := strings.CutPrefix(line, "apparmor "); ok && !strings.HasPrefix(rest, "<") {
			into = rest
		}
	}
	if from == "" || into == "" || from == into || strings.HasPrefix(into, "unconfined") {
		return from, into, false
	}
	return from, into, true
}

// wuLeaderPdeathsig is the number at the head of the report's LEADER row,
// `pdeathsig 9 (leader, read at init)` → "9"; "" when the row is missing or
// was not read on the leader, which no expected value matches.
func wuLeaderPdeathsig(report string) string {
	for _, line := range strings.Split(report, "\n") {
		if rest, ok := strings.CutPrefix(line, "pdeathsig "); ok {
			if !strings.Contains(rest, "(leader") {
				return ""
			}
			f := strings.Fields(rest)
			if len(f) == 0 {
				return ""
			}
			return f[0]
		}
	}
	return ""
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

// wuProcPpid is field 4 of /proc/<pid>/stat, read past the last ')' so a comm
// with spaces or parentheses cannot shift it, or a bracketed reason.
func wuProcPpid(pid int) string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "<" + err.Error() + ">"
	}
	s := string(b)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return "<unparsed stat>"
	}
	f := strings.Fields(s[i+1:])
	if len(f) < 2 {
		return "<unparsed stat>"
	}
	return f[1]
}

// wuSeatReportEnd terminates the seat's self-report, and it is what the
// parent waits for. The report is more than one line now, so a trailing
// newline no longer says "complete" — the seat writes the whole buffer in one
// os.WriteFile and the parent reads nothing until this sentinel is there, so a
// torn read is a retry and never a parse of half a report.
const wuSeatReportEnd = "END\n"

// wuSeatSelfReport is what the seat can read about ITSELF, and it exists
// because a death arm that reds on a runner nobody has a shell on has to say
// WHICH of the two readings it is (ranger-base-8m9d0). How to read it, in the
// next ubuntu arm-1 log:
//
// FIRST, WHICH THREAD (ranger-base-z56s9). pdeath_signal is per-task, and
// until ranger-base-z56s9 the `pdeathsig` row was read on whatever OS thread
// the test goroutine happened to be on — any non-leader thread reads 0 there
// while the leader holds 9 (MEASURED on D1, see wuSeatPdeathsig). The first
// two ubuntu-26.04 logs read `pdeathsig 0` in both arms through that row, so
// they did NOT establish the "cleared in BOTH arms" reading below; they are
// equally what a leader holding 9 looks like from another thread. The
// `pdeathsig` row is now the LEADER's, read at init and self-certified; the
// calling thread's value is `pdeathsig-this-thread`, tid beside it. Two more
// rows decide the rest without a shell on the runner: `AT_SECURE` (1 is the
// only exec-time clear — begin_new_exec zeroes pdeath_signal on a secure
// exec, and AppArmor's transition is the only thing on ubuntu that could make
// this one secure), and the parent's `outer bwrap` line on a red, which says
// whether the sh's death even took the outer bwrap or the break is one layer
// up from the seat.
//
//   - pdeathsig=0 in BOTH arms: the runner clears it before the seat runs —
//     bwrap, an AppArmor exec transition, or the credentials. The Landlock
//     premise above stands and the residual is "die-with-parent is a no-op on
//     that platform", which goes back to ranger-base-z56s9.
//   - pdeathsig=0 in the step arm ONLY: the step clears it, and the step is
//     this repo's code to fix. ONE CAUSE OF THIS ROW HAS SINCE BEEN FOUND AND
//     FIXED (ranger-base-vxz74, ADR 0080 D37/D38): the step applied its layers
//     to whatever thread the goroutine was on, and a pdeathsig is per-task and
//     zero on every thread the Go runtime clones, so an exec from a non-leader
//     thread read 0 here. It locks its OS thread and refuses off the thread
//     group leader now. On this box the row read 9 both before and after (600
//     execs each), so that fix does not predict what an ubuntu log says — but
//     if this row is still 0 in the step arm there, the cause is a second one,
//     and if the step arm instead fails to START, read its stderr: a seat that
//     refuses now says which thread it was on.
//   - pdeathsig=9 and the seat survives anyway: a DELIVERY problem, not a
//     clearing one, and ADR ranger-base-sv89t's premise needs amending.
//   - both arms pass: the original red came from the environment.
//
// THE ROWS BESIDE THE PDEATHSIG ARE THE SUSPECTS. ppid says whether the seat
// is still parented to the bwrap that set the pdeathsig or was reparented
// already; NoNewPrivs and Seccomp say whether this is the step's own
// PR_SET_NO_NEW_PRIVS or something the runner did; and the AppArmor label is
// the one difference between that runner and this box that could clear a
// pdeathsig through an exec transition (a kernel with no AppArmor answers
// ENOENT or EINVAL here, and that IS the answer, not a failure).
//
// MEASURED 2026-11-21 on this box (Linux 7.2.15-1-omarchy, go1.27.3,
// bubblewrap 0.13.1, LSMs lockdown,capability,landlock,yama,bpf,ima,evm with
// no AppArmor), both death arms PASS and both report:
//
//	pdeathsig 9 · NoNewPrivs: 1 · Seccomp: 0 · Seccomp_filters: 0
//	apparmor <read /proc/self/attr/current: invalid argument>
//
// which is the baseline an ubuntu log is read against, and it carries one
// reading worth having before the log arrives: **NoNewPrivs is 1 in the
// CONTROL arm too**, so that row is bwrap's `--unshare-user` and not the
// step's PR_SET_NO_NEW_PRIVS, and it cannot tell the two arms apart. The
// EINVAL on the AppArmor label is this kernel having no LSM that answers that
// file — the valid answer the paragraph above promises, and the row most
// likely to differ on ubuntu-26.04.
//
// EVERY ROW IS BEST-EFFORT AND NONE OF THEM CAN FAIL THE SEAT. An
// unreadable row prints its error and the report still ends with its
// sentinel: the pin's subject is whether the seat dies, and a diagnostic that
// can turn a death arm into a fixture red would be worse than no diagnostic.
func wuSeatSelfReport() string {
	var b strings.Builder
	// pid FIRST and on its own line: it is the one row the parent parses, and
	// everything after it is for a human reading a log.
	b.WriteString("pid " + strconv.Itoa(os.Getpid()) + "\n")
	b.WriteString("pdeathsig " + wuSeatLeaderPdeathsig + "\n")
	b.WriteString("pdeathsig-this-thread " + wuSeatPdeathsig() + "\n")
	b.WriteString("AT_SECURE " + wuSeatATSecure() + "\n")
	b.WriteString("ppid " + strconv.Itoa(os.Getppid()) + "\n")
	if s, err := os.ReadFile("/proc/self/status"); err != nil {
		b.WriteString("status <unreadable: " + err.Error() + ">\n")
	} else {
		for _, want := range []string{"NoNewPrivs:", "Seccomp:", "Seccomp_filters:"} {
			b.WriteString(want + " " + wuStatusField(string(s), want) + "\n")
		}
	}
	// The AppArmor label. ENOENT (no securityfs row) and EINVAL (a kernel
	// with no LSM that answers this file) are both valid answers and are
	// printed as such.
	if s, err := os.ReadFile("/proc/self/attr/current"); err != nil {
		b.WriteString("apparmor <" + err.Error() + ">\n")
	} else {
		b.WriteString("apparmor " + strings.TrimRight(string(s), "\x00\n") + "\n")
	}
	b.WriteString(wuSeatReportEnd)
	return b.String()
}

// wuStatusField pulls one `Name:\tvalue` row out of /proc/self/status, and
// says `<absent>` rather than empty when the kernel does not have it:
// Seccomp_filters: is a 4.17+ row and Seccomp: needs CONFIG_SECCOMP, so an
// empty string here would read as "0" to someone skimming the log.
func wuStatusField(status, key string) string {
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, key) {
			return strings.TrimSpace(strings.TrimPrefix(line, key))
		}
	}
	return "<absent>"
}

// wuSeatReportPid reads back the one row the parent needs. It takes the
// report and not the raw file so that a report whose sentinel arrived is
// parsed exactly once.
func wuSeatReportPid(report string) int {
	return wuSeatReportInt(report, "pid ")
}

// wuSeatReportInt reads one integer row of the report by its prefix, 0 when
// the row is missing or not a number.
func wuSeatReportInt(report, prefix string) int {
	for _, line := range strings.Split(report, "\n") {
		if rest, ok := strings.CutPrefix(line, prefix); ok {
			n, err := strconv.Atoi(strings.TrimSpace(rest))
			if err != nil {
				return 0
			}
			return n
		}
	}
	return 0
}

// TestWallUnixSignalSeatChild is the seat for the pin above.
func TestWallUnixSignalSeatChild(t *testing.T) {
	if pf := os.Getenv(wuEnvSigPidFile); pf != "" {
		if err := os.WriteFile(pf, []byte(wuSeatSelfReport()), 0o600); err != nil {
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
			dir, sock := wuSockDir(t, "wall.sock")
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
