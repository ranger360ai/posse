package rhq

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// plugin/autostart.sh under test. herdr runs it as `bash autostart.sh
// --startup` once per server start — including the start of a live-handoff
// import server, which is the case rangerhq-ct9 is about.
//
// Two halves, faked differently on purpose. Everything the hook does to
// herdr (`new`, `kill`) is scripted, so nothing real is created. The
// liveness decision is not faked at all: the hook asks
// `posse dispatch --watch-status`, and that reaches the real WatchStatus
// over a real flock in a temp RHQ_HOME. A live loop here is the test
// process holding that lock, and the hook's probe is a separate process
// asking the kernel about it — which is exactly the arrangement in
// production (rangerhq-gir5).

type hookRun struct {
	out   string
	code  int
	calls string // one `posse <argv>` per line
}

type hookWorld struct {
	home         string // RHQ_HOME
	exists       string // touched while the fake posse believes the session exists
	socket       string // HERDR_SOCKET_PATH; empty is the default herdr server
	herdrSession string // HERDR_SESSION; empty is the default herdr server
	deaf         string // touched to make the fake posse fail --watch-status
	noisy        string // touched to make it print an unrelated stderr notice first
	killRefused  string // touched to make `kill` refuse the way the reap guard does
}

func newHookWorld(t *testing.T, config string) *hookWorld {
	t.Helper()
	home := t.TempDir()
	w := &hookWorld{
		home:   home,
		exists: filepath.Join(home, "session-exists"),
		deaf:   filepath.Join(home, "probe-deaf"),
		noisy:  filepath.Join(home, "probe-noisy"),

		killRefused: filepath.Join(home, "kill-refused"),
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	// `new` and `kill` are scripted — they stand in for herdr. The liveness
	// question is not: `dispatch --watch-status` re-execs this test binary
	// as posse (TestMain's RHQ_FAKE_POSSE arm), which answers it from the
	// real lock in the real state dir. The seam the hook depends on is that
	// line, so the tests must cross it for real (rangerhq-gir5).
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fake := "#!/usr/bin/env bash\n" +
		"echo \"$*\" >> " + shq(filepath.Join(home, "calls.log")) + "\n" +
		"if [ \"$1 $2\" = 'dispatch --watch-status' ]; then\n" +
		"  if [ -e " + shq(w.deaf) + " ]; then echo 'posse: unknown flag: --watch-status' >&2; exit 1; fi\n" +
		"  if [ -e " + shq(w.noisy) + " ]; then echo 'posse: ~/.config/posse does not exist; using existing home ~/.config/rhq (nothing moved)' >&2; fi\n" +
		"  RHQ_FAKE_POSSE=1 exec " + shq(self) + " dispatch --watch-status\n" +
		"fi\n" +
		"case \"$1\" in\n" +
		"new)  if [ -e " + shq(w.exists) + " ]; then echo 'workspace already exists'; exit 1; fi\n" +
		"      : > " + shq(w.exists) + "; echo created; exit 0 ;;\n" +
		"kill) if [ -e " + shq(w.killRefused) + " ]; then\n" +
		"        printf 'NOT killed: dispatch still holds rb-1 (in_progress)\\nand ~/src/posse has uncommitted work\\n' >&2; exit 1\n" +
		"      fi\n" +
		"      rm -f " + shq(w.exists) + "; exit 0 ;;\n" +
		"esac\nexit 0\n"
	if err := os.WriteFile(filepath.Join(home, "posse"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	return w
}

func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// app is the hook's RHQ_HOME as posse sees it.
func (w *hookWorld) app() *App {
	return &App{Home: w.home, StateDir: filepath.Join(w.home, "state")}
}

// loopRunning arranges the live loop every carry-over test needs: the test
// itself takes the watch lock, exactly as `posse dispatch --watch` does, and
// the hook's probe is a separate process asking the kernel about it.
func (w *hookWorld) loopRunning(t *testing.T) *WatchLock {
	t.Helper()
	lock, held, err := lockWatch(w.app())
	if err != nil || held {
		t.Fatalf("could not arrange a running loop: held=%v err=%v", held, err)
	}
	t.Cleanup(lock.Release)
	return lock
}

// deafProbe is the "could not ask" environment: a posse too old to know the
// subcommand, or any other reason the question cannot be put. The hook must
// stand down on it rather than replace a loop it cannot see.
func (w *hookWorld) deafProbe(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(w.deaf, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// noisyProbe is the operator's own box: posse prints the config-home
// transition notice on stderr before it answers anything, on every
// invocation, for as long as the instance has only the old home.
func (w *hookWorld) noisyProbe(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(w.noisy, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// probedOnly reports whether the hook asked the liveness question and did
// nothing else — the shape of every stand-down.
func probedOnly(calls string) bool {
	for _, line := range strings.Split(strings.TrimSpace(calls), "\n") {
		if line != "" && line != "dispatch --watch-status" {
			return false
		}
	}
	return true
}

// sessionExists makes the fake posse refuse `new` the way herdr's does for a
// name that already resolves to a workspace — husk or live, it looks the same.
func (w *hookWorld) sessionExists(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(w.exists, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// killRefuses is a `posse kill` that says no — the ADR 0013 §4 reap guard, or
// the foreign-workspace refusal. The hook cannot overrule it; what it must not
// do is swallow the reason (ranger-base-oej).
func (w *hookWorld) killRefuses(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(w.killRefused, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (w *hookWorld) run(t *testing.T, args ...string) hookRun {
	t.Helper()
	hook, err := filepath.Abs(filepath.Join("..", "..", "plugin", "autostart.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", append([]string{hook}, args...)...)
	cmd.Env = append(os.Environ(),
		"RHQ_HOME="+w.home,
		"RHQ_BIN="+filepath.Join(w.home, "posse"),
		"HOME="+w.home,
		"HERDR_SOCKET_PATH="+w.socket,
		"HERDR_SESSION="+w.herdrSession,
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(filepath.Join(w.home, "calls.log"))
	return hookRun{out: string(out), code: code, calls: string(calls)}
}

const armed = "autostart_interval: 30s\n"

// herdr's plugin registry is global, so [[startup]] runs for named session
// servers as well as the default server. A scratch server that inherits the
// fleet RHQ_HOME must never become a second writer for the fleet queue or its
// dispatch-watch.pid (ranger-base-87q).
func TestAutostartNamedSocketNeverArmsTheFleet(t *testing.T) {
	w := newHookWorld(t, armed)
	w.socket = filepath.Join(w.home, ".config", "herdr", "sessions", "ug9b-qa", "herdr.sock")

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if r.calls != "" {
		t.Errorf("named herdr server invoked posse against the fleet:\n%s", r.calls)
	}
	if !strings.Contains(r.out, "not the default herdr server") || !strings.Contains(r.out, w.socket) {
		t.Errorf("stand-down must name the non-default socket:\n%s", r.out)
	}
}

// HERDR_SESSION is herdr's other way to select a named server. The plugin
// normally injects the exact socket, but an absent socket must not turn a
// named session into the default by accident.
func TestAutostartNamedSessionNeverArmsTheFleet(t *testing.T) {
	w := newHookWorld(t, armed)
	w.herdrSession = "ug9b-qa"

	r := w.run(t, "--startup")
	if r.code != 0 || r.calls != "" {
		t.Errorf("named herdr session touched posse (exit %d):\n%s\n%s", r.code, r.out, r.calls)
	}
	if !strings.Contains(r.out, "HERDR_SESSION=ug9b-qa") {
		t.Errorf("stand-down must name the named session selector:\n%s", r.out)
	}
}

// An explicit path to herdr's default socket is still the fleet server. This
// is the positive control for the non-default socket fence above.
func TestAutostartExplicitDefaultSocketStillArms(t *testing.T) {
	w := newHookWorld(t, armed)
	w.socket = filepath.Join(w.home, ".config", "herdr", "herdr.sock")

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.calls, "dispatch --watch 30s -n 3 --resume") {
		t.Errorf("default herdr server did not arm the fleet loop:\n%s", r.calls)
	}
}

// The ownership fence is for automatic server startup. Running the hook by
// hand is an explicit act and keeps its existing ability to target whatever
// herdr socket the operator selected.
func TestAutostartByHandCanTargetNamedServer(t *testing.T) {
	w := newHookWorld(t, armed)
	w.socket = filepath.Join(w.home, ".config", "herdr", "sessions", "staging", "herdr.sock")

	r := w.run(t)
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.calls, "dispatch --watch 30s -n 3 --resume") {
		t.Errorf("explicit hook run did not target the selected server:\n%s", r.calls)
	}
}

// The bug: herdr runs [[startup]] hooks on a live handoff too, where the
// workspace comes back WITH its command still running. The hook must leave
// it alone — killing it drops a claimed bead and a prompt in flight, and
// starts a second loop on the same queue (rangerhq-ct9).
func TestAutostartLeavesACarriedOverLoopAlone(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	w.loopRunning(t)
	// The identity half, which the hook quotes and never reasons from.
	if err := WriteWatchPid(WatchPidPath(w.app()), WatchPid{Pid: os.Getpid(), Started: time.Now()}); err != nil {
		t.Fatal(err)
	}

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Errorf("exit %d, want 0:\n%s", r.code, r.out)
	}
	if strings.Contains(r.calls, "kill") {
		t.Errorf("the live loop was killed:\n%s", r.calls)
	}
	if strings.Contains(r.calls, "new") {
		t.Errorf("a second loop was started against the same queue:\n%s", r.calls)
	}
	if !strings.Contains(r.out, "left alone") {
		t.Errorf("want the hook to say it stood down:\n%s", r.out)
	}
	if !strings.Contains(r.out, strconv.Itoa(os.Getpid())) {
		t.Errorf("the stand-down must name the holder from the pidfile:\n%s", r.out)
	}
	if !probedOnly(r.calls) {
		t.Errorf("a live loop must cost exactly one question and nothing else:\n%s", r.calls)
	}
}

// The pidfile is identity, not evidence. A held lock with no record beside
// it is still a running loop — the hook stands down, and says it cannot name
// the holder rather than deciding it therefore has none.
func TestAutostartLeavesACarriedOverLoopAloneWithNoRecord(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	w.loopRunning(t)

	r := w.run(t, "--startup")
	if r.code != 0 || !probedOnly(r.calls) {
		t.Errorf("exit %d, calls:\n%s\nout:\n%s", r.code, r.calls, r.out)
	}
	if !strings.Contains(r.out, "holder unrecorded") || !strings.Contains(r.out, "left alone") {
		t.Errorf("want a stand-down that admits it cannot name the holder:\n%s", r.out)
	}
}

// One loop per queue, not per session name: a --watch the operator started
// by hand in another workspace still owns the pidfile, so --startup must
// not create a second loop under autostart_session.
func TestAutostartStandsDownWhenLoopLivesUnderAnotherName(t *testing.T) {
	w := newHookWorld(t, armed)
	// no sessionExists: autostart_session is a name that is not running
	w.loopRunning(t)

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Errorf("exit %d, want 0:\n%s", r.code, r.out)
	}
	if !probedOnly(r.calls) {
		t.Errorf("a live loop in any session must not start another:\n%s", r.calls)
	}
}

// The case the hook was written for still works: a cold server start, where
// herdr restored the workspace without re-running the command.
func TestAutostartReplacesAHusk(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t) // restored by herdr, no pidfile: nothing posse launched survived

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Errorf("exit %d, want 0:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.calls, "kill dispatch") {
		t.Errorf("a husk must be killed:\n%s", r.calls)
	}
	if !strings.Contains(r.calls, "dispatch --watch 30s") {
		t.Errorf("the loop must be started after the kill:\n%s", r.calls)
	}
	// herdr server start is this path, not the cold `new`. ranger-base-f0g
	// only matters if the replacement command resumes.
	if !strings.Contains(r.calls, "dispatch --watch 30s -n 3 --resume") {
		t.Errorf("husk replacement did not arm --resume:\n%s", r.calls)
	}
}

// A loop killed with its pane never removes its pidfile. The record is not
// the evidence — the lock it left behind is free the instant it died, so
// the stale file must not keep the fleet unarmed.
func TestAutostartReplacesAHuskWithAStaleRecord(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	done := exec.Command("sh", "-c", "exit 0")
	if err := done.Run(); err != nil {
		t.Fatal(err)
	}
	WriteWatchPid(WatchPidPath(w.app()), WatchPid{Pid: done.Process.Pid})

	r := w.run(t, "--startup")
	if !strings.Contains(r.calls, "kill dispatch") || !strings.Contains(r.calls, "--watch 30s") {
		t.Errorf("stale record must read as no loop:\n%s\n%s", r.out, r.calls)
	}
}

// Pid reuse: the recorded number is alive, but it belongs to something else.
// Under the pidfile this needed an argv match to refute, and the refutation
// leaked (below). Under the lock there is nothing to refute — the recorded
// pid is never asked.
func TestAutostartIgnoresARecycledPid(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	WriteWatchPid(WatchPidPath(w.app()), WatchPid{Pid: liveProcess(t, "somebody-elses-sleeper")})

	r := w.run(t, "--startup")
	if !strings.Contains(r.calls, "kill dispatch") || !strings.Contains(r.calls, "--watch 30s") {
		t.Errorf("a recycled pid must not read as a running loop:\n%s\n%s", r.out, r.calls)
	}
}

// rangerhq-ppy9, unskipped: `grep -q dispatch` matched any argv substring, so
// a recycled pid of a one-shot `posse dispatch --persona` — or a claude
// whose --append-system-prompt says the word — read as the watch loop and
// the hook stood down, leaving the fleet unarmed until that pid died. The
// argv is now nobody's evidence: this process holds no watch lock, so it is
// not the watch loop, whatever it calls itself.
func TestAutostartIgnoresARecycledOneShotDispatch(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	if err := WriteWatchPid(WatchPidPath(w.app()), WatchPid{Pid: liveArgv(t, "posse dispatch --persona qa -n 1")}); err != nil {
		t.Fatal(err)
	}

	r := w.run(t, "--startup")
	if !strings.Contains(r.calls, "kill dispatch") || !strings.Contains(r.calls, "--watch 30s") {
		t.Errorf("a one-shot dispatch is not the watch loop:\n%s\n%s", r.out, r.calls)
	}
}

// rangerhq-mugy, ranger-base-rmc: three tests used to live here, one per
// way `ps` can decline to answer — exit 127, exit 0 with nothing, exit 0
// with a column of blanks — because the argv probe had to distinguish
// silence from refutation and got it wrong twice. The lock has no such
// arm: it is held or it is free. What survives is the case where the
// QUESTION cannot be put at all, and the answer to that is unchanged —
// stand down, and say which fact you stood down on (rangerhq-llse).
// Unarmed is visible and recoverable; double dispatch is neither.
func TestAutostartStandsDownWhenTheProbeCannotAnswer(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	w.loopRunning(t)
	w.deafProbe(t) // a posse too old to know --watch-status

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Errorf("exit %d, want 0:\n%s", r.code, r.out)
	}
	if !probedOnly(r.calls) {
		t.Errorf("an unanswerable probe must not replace a live loop:\n%s", r.calls)
	}
	if !strings.Contains(r.out, "left alone") {
		t.Errorf("want the hook to say it stood down:\n%s", r.out)
	}
	if !strings.Contains(r.out, "could not answer") {
		t.Errorf("want the stand-down to name the failed probe:\n%s", r.out)
	}
	if !strings.Contains(r.out, "unknown flag") {
		t.Errorf("want the probe's own words quoted, so the cause is diagnosable:\n%s", r.out)
	}
}

// Same deafness, but with no loop running at all. The hook cannot tell the
// two apart — that is the point of standing down — so it stands down here
// too, and the fleet stays unarmed until somebody looks. That cost is the
// one this direction accepts on purpose.
func TestAutostartStandsDownOnADeafProbeEvenWithNoLoop(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	w.deafProbe(t)

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Errorf("exit %d, want 0:\n%s", r.code, r.out)
	}
	if strings.Contains(r.calls, "kill") || strings.Contains(r.calls, "new") {
		t.Errorf("an unanswerable probe must never authorise a kill:\n%s", r.calls)
	}
	if !strings.Contains(r.out, "make link-plugin") {
		t.Errorf("want the recovery named, since this state does not clear itself:\n%s", r.out)
	}
}

// stdout is the answer; stderr is not part of it. posse writes unrelated
// notices there, and folding them in would put a line in front of
// `watch-loop: …`, read as "could not ask", and stand the hook down on
// every start of an instance that has not migrated its config home —
// permanently unarmed, for a reason nothing in the message would name.
func TestAutostartReadsTheAnswerPastAnUnrelatedStderrNotice(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	w.loopRunning(t)
	w.noisyProbe(t)

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.out, "loop already running") {
		t.Errorf("a stderr notice must not hide the answer:\n%s", r.out)
	}
	if !probedOnly(r.calls) {
		t.Errorf("the live loop was not left alone:\n%s", r.calls)
	}
}

// Same notice, no loop: the husk is still replaced. The noise must not push
// the hook into either standing decision.
func TestAutostartReplacesAHuskPastAnUnrelatedStderrNotice(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	w.noisyProbe(t)

	r := w.run(t, "--startup")
	if !strings.Contains(r.calls, "kill dispatch") || !strings.Contains(r.calls, "--watch 30s") {
		t.Errorf("a husk must still be replaced:\n%s\n%s", r.out, r.calls)
	}
}

// By hand, with no --startup, nothing is ever killed: the name may be worn by
// a workspace the operator is sitting in.
func TestAutostartByHandNeverKills(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)

	r := w.run(t)
	if strings.Contains(r.calls, "kill") {
		t.Errorf("a by-hand run killed something:\n%s", r.calls)
	}
}

// The defect (ranger-base-oej): with the name taken and NO loop behind it, the
// by-hand run reported "dispatch already running — left alone" and exited 0.
// The lock had already been asked and answered none two blocks earlier, so the
// line contradicted the hook's own measurement — during F8 the operator read it
// twice off a husk, with no loop and no pidfile, and it named neither the husk
// nor the lever. A report that armed nothing must not say a loop is running,
// must name `posse kill`, and must not exit 0.
func TestAutostartByHandNameTakenWithNoLoopIsNotAlreadyRunning(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t) // a husk: the workspace came back, the command did not

	r := w.run(t)
	if strings.Contains(r.out, "already running") {
		t.Errorf("no loop holds the lock, so nothing is already running:\n%s", r.out)
	}
	if r.code == 0 {
		t.Errorf("exit 0 reports an arm that did not happen:\n%s", r.out)
	}
	if !strings.Contains(r.out, "no dispatch loop holds the lock") {
		t.Errorf("want the measured reason:\n%s", r.out)
	}
	// The lever, and why nothing else will pull it: the workspace is created
	// by `posse new`, which stamps crew: true, so every sweep steps over it.
	if !strings.Contains(r.out, "posse kill dispatch") {
		t.Errorf("want the lever named:\n%s", r.out)
	}
	if !strings.Contains(r.out, CrewTag) {
		t.Errorf("want the crew mark named as why the husk survives:\n%s", r.out)
	}
	if strings.Contains(r.calls, "kill") {
		t.Errorf("naming the lever is not pulling it:\n%s", r.calls)
	}
}

// The positive control for the case above: by hand, over a loop that really is
// running, "already running — left alone" is the truth and exit 0 is right.
func TestAutostartByHandOverALiveLoopStillReportsSuccess(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	w.loopRunning(t)

	r := w.run(t)
	if r.code != 0 {
		t.Errorf("exit %d, want 0 over a live loop:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.out, "loop already running") || !strings.Contains(r.out, "left alone") {
		t.Errorf("want the conservative report:\n%s", r.out)
	}
	if !probedOnly(r.calls) {
		t.Errorf("a live loop must be left alone:\n%s", r.calls)
	}
}

// The startup twin of the same complaint: a kill CAN refuse — the reap guard,
// the foreign-workspace refusal — and the hook used to send that reason to
// /dev/null, leaving "still present after kill — not started" naming no lever.
func TestAutostartHuskReplacementQuotesARefusingKill(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	w.killRefuses(t)

	r := w.run(t, "--startup")
	if r.code == 0 {
		t.Errorf("exit 0 with nothing armed:\n%s", r.out)
	}
	if !strings.Contains(r.out, "still present after kill") {
		t.Errorf("want the failure said:\n%s", r.out)
	}
	if !strings.Contains(r.out, "NOT killed: dispatch still holds rb-1") {
		t.Errorf("want the kill's own reason quoted:\n%s", r.out)
	}
	// One line: the refusal is multi-line and the hook squashes it.
	for _, line := range strings.Split(r.out, "\n") {
		if strings.Contains(line, "still present after kill") && !strings.Contains(line, "uncommitted work") {
			t.Errorf("the quoted reason was cut off at the newline:\n%s", r.out)
		}
	}
}

// Disarmed is disarmed: no autostart_interval:, no posse call at all — not
// even the liveness probe's kill(2).
func TestAutostartDisarmed(t *testing.T) {
	w := newHookWorld(t, "beads:\n  - /nowhere\n")
	w.sessionExists(t)

	r := w.run(t, "--startup")
	if r.code != 0 || r.calls != "" {
		t.Errorf("disarmed hook touched posse (exit %d):\n%s\n%s", r.code, r.out, r.calls)
	}
	if !strings.Contains(r.out, "disarmed") {
		t.Errorf("want the disarmed report:\n%s", r.out)
	}
}

// A bare `autostart_interval:` is the one shape that is neither armed nor
// disarmed, and it used to read as disarmed — which made the seed config's
// "presence, not value, is the arm switch" paragraph a lie and made the one
// diagnostic the deployer gets ("no autostart_interval:") point away from the
// key sitting in their file (ranger-base-cxyk). It cannot be defaulted:
// `posse dispatch --watch` has no default interval (cmd/posse/main.go dies
// with "--watch needs an interval"), so the hook refuses it here rather than
// arming a session that dies in a log.
//
// Every variant below is the same shape after cfg()'s trailing-comment and
// whitespace strip — including the operator who commented out the VALUE and
// left the key, which is the likeliest way to reach this by hand.
func TestAutostartBareIntervalIsRefusedNotDisarmed(t *testing.T) {
	for name, line := range map[string]string{
		"bare":       "autostart_interval:\n",
		"whitespace": "autostart_interval:   \n",
		"comment":    "autostart_interval: # 5m\n",
	} {
		t.Run(name, func(t *testing.T) {
			w := newHookWorld(t, line)
			w.sessionExists(t)

			r := w.run(t, "--startup")
			if r.code == 0 {
				t.Errorf("a bare autostart_interval: was accepted (exit 0):\n%s", r.out)
			}
			if r.calls != "" {
				t.Errorf("the hook armed something off an empty interval:\n%s", r.calls)
			}
			if !strings.Contains(r.out, "autostart_interval:") || !strings.Contains(r.out, "present but empty") {
				t.Errorf("the refusal does not name the key and what is wrong with it:\n%s", r.out)
			}
			// The old message asserted the key was absent while the deployer
			// was looking at it. That sentence is the bug, not just the
			// exit code, so it is pinned separately.
			if strings.Contains(r.out, "no autostart_interval:") {
				t.Errorf("the hook still reports the present key as absent:\n%s", r.out)
			}
		})
	}
}

// One input shape over from the bare key, and the same broken arm: a value the
// hook cannot read. `posse dispatch --watch banana` dies with
// `bad interval "banana"` — but it dies inside the herdr session, after the
// hook has already told the deployer "dispatch started", so the operator has
// an unattended loop they believe is armed and is not (ranger-base-7rt5).
// Every other value key here names what it cannot read; this one cannot fall
// back on a default, so it stands down the way the empty key does.
//
// ParseInterval is asked about every fixture, so this table cannot quietly
// drift into pinning shapes posse would have accepted.
func TestAutostartMalformedIntervalIsRefusedNotArmed(t *testing.T) {
	for name, value := range map[string]string{
		"word":         "banana",
		"quoted empty": `""`,
		"zero":         "0",
		"zero units":   "0h0m",
		"negative":     "-5m",
		"wrong unit":   "5min",
		"units only":   "m",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseInterval(value); err == nil {
				t.Fatalf("fixture %q is one posse accepts — not a malformed value", value)
			}
			w := newHookWorld(t, "autostart_interval: "+value+"\n")

			r := w.run(t, "--startup")
			if r.code == 0 {
				t.Errorf("a malformed autostart_interval: was accepted (exit 0):\n%s", r.out)
			}
			if r.calls != "" {
				t.Errorf("the hook armed something off an interval posse refuses:\n%s", r.calls)
			}
			// "dispatch started" is the specific lie: a positive claim, in the
			// deployer's terminal, about a loop that dies in a session log.
			if strings.Contains(r.out, "started") {
				t.Errorf("the hook reported success having armed nothing:\n%s", r.out)
			}
			if !strings.Contains(r.out, "autostart_interval:") || !strings.Contains(r.out, value) {
				t.Errorf("the refusal names neither the key nor the value the operator typed:\n%s", r.out)
			}
		})
	}
}

// The other half of that check, and the reason it is a mirror of
// ParseInterval rather than a guess: refusing a value posse would have
// accepted is a disarmed fleet, which is the worse failure of the two. Every
// shape posse takes must still arm, and must reach the loop verbatim.
func TestAutostartArmsEveryIntervalPosseAccepts(t *testing.T) {
	for _, value := range []string{"30s", "5m", "45", "1h30m", "2m30s", "500ms", "1.5m", ".5s", "007", "2h45m30s500ms"} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseInterval(value); err != nil {
				t.Fatalf("fixture %q is not one posse accepts: %v", value, err)
			}
			w := newHookWorld(t, "autostart_interval: "+value+"\n")

			r := w.run(t, "--startup")
			if r.code != 0 {
				t.Fatalf("exit %d on an interval posse accepts:\n%s", r.code, r.out)
			}
			if !strings.Contains(r.calls, "dispatch --watch "+value+" ") {
				t.Errorf("the interval did not reach the loop verbatim:\n%s", r.calls)
			}
		})
	}
}

// ── the backoff cap (ranger-base-x8y8) ─────────────────────────────────────
//
// autostart_max_interval: has the same defect as the interval above — the
// hook appended whatever the key said to the argv unvalidated, so
// `--max-interval banana` died inside the session under a hook that had
// already said "dispatch started" — and deliberately NOT the same answer.
// The arm switch refuses because posse has no default interval to fall back
// on; this key's absent-flag case IS a default (8x the base, cmd/posse
// main.go), so it takes the autostart_max_beads / autostart_resume
// precedent: name the value, use the default, arm. Standing a fleet down
// over an unreadable backoff cap is the more expensive of the two failures,
// and the key is not the arm switch.

func TestAutostartMalformedMaxIntervalIsDroppedNotArmed(t *testing.T) {
	// Same fixtures as the interval table, asked of ParseInterval the same
	// way, so the two answers to one grammar cannot drift apart.
	for name, value := range map[string]string{
		"word":         "banana",
		"quoted empty": `""`,
		"zero":         "0",
		"zero units":   "0h0m",
		"negative":     "-5m",
		"wrong unit":   "5min",
		"units only":   "m",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseInterval(value); err == nil {
				t.Fatalf("fixture %q is one posse accepts — not a malformed value", value)
			}
			w := newHookWorld(t, armed+"autostart_max_interval: "+value+"\n")

			r := w.run(t, "--startup")
			// Armed, unlike the arm switch: the cap has a default and the
			// fleet is not stood down over one.
			if r.code != 0 {
				t.Fatalf("exit %d — a malformed cap stood the fleet down:\n%s", r.code, r.out)
			}
			if !strings.Contains(r.calls, "dispatch --watch 30s -n 3 --resume") {
				t.Errorf("the loop was not armed with posse's default cap:\n%s", r.calls)
			}
			// The flag is DROPPED, not passed through and not passed empty:
			// `--max-interval` with a value posse refuses is the whole bug,
			// and `--max-interval` with nothing after it eats the next flag.
			if strings.Contains(r.calls, "--max-interval") {
				t.Errorf("a cap posse refuses reached the loop:\n%s", r.calls)
			}
			if !strings.Contains(r.out, "autostart_max_interval:") || !strings.Contains(r.out, value) {
				t.Errorf("the fallback names neither the key nor the value the operator typed:\n%s", r.out)
			}
			// Named AND replaced: the operator has to be able to tell which
			// cap the armed loop is actually running under.
			if !strings.Contains(r.out, "default") {
				t.Errorf("the line does not say the default was used instead:\n%s", r.out)
			}
		})
	}
}

// The other half, and the reason valid_interval is a mirror of ParseInterval
// rather than a guess: dropping a cap posse would have accepted is a loop
// backing off to 8x when the operator asked for 40m. Every shape posse takes
// must reach the loop verbatim, and quietly.
func TestAutostartArmsEveryMaxIntervalPosseAccepts(t *testing.T) {
	for _, value := range []string{"30s", "5m", "45", "1h30m", "2m30s", "500ms", "1.5m", ".5s", "007", "2h45m30s500ms"} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseInterval(value); err != nil {
				t.Fatalf("fixture %q is not one posse accepts: %v", value, err)
			}
			w := newHookWorld(t, armed+"autostart_max_interval: "+value+"\n")

			r := w.run(t, "--startup")
			if r.code != 0 {
				t.Fatalf("exit %d on a cap posse accepts:\n%s", r.code, r.out)
			}
			if !strings.Contains(r.calls, "--max-interval "+value+" ") {
				t.Errorf("the cap did not reach the loop verbatim:\n%s", r.calls)
			}
			if strings.Contains(r.out, "autostart_max_interval:") {
				t.Errorf("a cap posse accepts was warned about:\n%s", r.out)
			}
		})
	}
}

// An empty value is not a malformed one. Absent and empty both mean "no cap
// given" — the flag is omitted and posse's default applies — which is what
// they already meant here and what an empty autostart_max_beads: means. Only
// a value that says something posse cannot read earns a line, or every
// deployer who left the key as a placeholder gets a warning about nothing.
func TestAutostartEmptyMaxIntervalIsSilentlyTheDefault(t *testing.T) {
	for name, line := range map[string]string{
		"absent":     "",
		"bare":       "autostart_max_interval:\n",
		"whitespace": "autostart_max_interval:   \n",
		"comment":    "autostart_max_interval: # 40m\n",
	} {
		t.Run(name, func(t *testing.T) {
			w := newHookWorld(t, armed+line)

			r := w.run(t, "--startup")
			if r.code != 0 {
				t.Fatalf("exit %d:\n%s", r.code, r.out)
			}
			if strings.Contains(r.calls, "--max-interval") {
				t.Errorf("an empty cap reached the loop as a flag:\n%s", r.calls)
			}
			if strings.Contains(r.out, "autostart_max_interval:") {
				t.Errorf("an empty cap was warned about:\n%s", r.out)
			}
		})
	}
}

// Both interval keys malformed at once. The arm switch decides: it refuses
// first and nothing is armed, so the cap's fallback never runs and never
// says anything — one diagnostic, naming the key that actually stopped it.
func TestAutostartMalformedIntervalWinsOverMalformedMax(t *testing.T) {
	w := newHookWorld(t, "autostart_interval: banana\nautostart_max_interval: kumquat\n")

	r := w.run(t, "--startup")
	if r.code == 0 {
		t.Fatalf("a malformed arm switch was accepted (exit 0):\n%s", r.out)
	}
	if r.calls != "" {
		t.Errorf("the hook armed something off a refused interval:\n%s", r.calls)
	}
	if !strings.Contains(r.out, "autostart_interval: 'banana'") {
		t.Errorf("the refusal does not name the key that stopped the arm:\n%s", r.out)
	}
	if strings.Contains(r.out, "autostart_max_interval:") {
		t.Errorf("the cap was diagnosed on a run that armed nothing:\n%s", r.out)
	}
}

// ── the resume arm (ranger-base-f0g) ────────────────────────────────────────
//
// The armed loop is the one the OPERATOR gets, and before this it was
// permanently the non-resuming one: a bead whose persona settled idle without
// closing it got a `◑ … settled but open — review` line in a log nobody was
// reading, and then sat there. `autostart_resume:` is therefore the one key in
// this hook that defaults ON, which is exactly the kind of inversion that
// wants pinning rather than remembering.

func TestAutostartArmsResumeByDefault(t *testing.T) {
	w := newHookWorld(t, armed)

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.calls, "dispatch --watch 30s -n 3 --resume") {
		t.Errorf("armed loop does not resume — a settled-but-open bead sits forever:\n%s", r.calls)
	}
}

// Off is available, and only by saying so. An operator who wants the warning
// and nothing else says false; nothing else in the file turns it off.
func TestAutostartResumeFalseDisarmsResume(t *testing.T) {
	for _, off := range []string{"false", "no", "0"} {
		t.Run(off, func(t *testing.T) {
			w := newHookWorld(t, armed+"autostart_resume: "+off+"\n")

			r := w.run(t, "--startup")
			if r.code != 0 {
				t.Fatalf("exit %d:\n%s", r.code, r.out)
			}
			if strings.Contains(r.calls, "--resume") {
				t.Errorf("autostart_resume: %s still armed --resume:\n%s", off, r.calls)
			}
			if !strings.Contains(r.calls, "dispatch --watch") {
				t.Errorf("the loop was not armed at all:\n%s", r.calls)
			}
		})
	}
}

// A typo is not an off-switch. `autostart_max_beads` sets the precedent: a
// value the hook cannot read is named on stderr and replaced with the default
// rather than silently obeyed — and here the default is the safe direction,
// so the one thing a misspelling must not do is put the loop back in the
// broken shape while looking configured.
func TestAutostartResumeGarbageKeepsResumeAndSaysSo(t *testing.T) {
	w := newHookWorld(t, armed+"autostart_resume: flase\n")

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.calls, "--resume") {
		t.Errorf("a malformed autostart_resume read as off:\n%s", r.calls)
	}
	if !strings.Contains(r.out, "not true/false") {
		t.Errorf("the malformed value was obeyed silently:\n%s", r.out)
	}
}

// Both switches at once, in the shape an operator actually arms first: dry
// passes that also resume. The two are independent flags on one command line.
func TestAutostartResumeAndDryRunCompose(t *testing.T) {
	w := newHookWorld(t, armed+"autostart_dry_run: true\nautostart_resume: true\n")

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	for _, flag := range []string{"--resume", "--dry-run", "-n 3"} {
		if !strings.Contains(r.calls, flag) {
			t.Errorf("armed command is missing %s:\n%s", flag, r.calls)
		}
	}
}

// INSTALL's first arm: dry-run on, resume key still commented. The inverted
// default has to survive that shape or the observation loop is the old
// warn-and-sit loop with a dry-run flag on it.
func TestAutostartDryRunStillResumesByDefault(t *testing.T) {
	w := newHookWorld(t, armed+"autostart_dry_run: true\n")

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.calls, "dispatch --watch 30s -n 3 --resume --dry-run") {
		t.Errorf("dry first-arm lost default --resume:\n%s", r.calls)
	}
}

// ── the fan-out cap (rangerhq-v83) ─────────────────────────────────────────
//
// The unattended loop's -n is always present. Absent autostart_max_beads
// used to omit the flag, and Dispatcher's 0 default is no cap — a whole
// ready queue in one pass. The key raises or lowers a cap; it does not
// switch it on. 0 is still unbounded, and only by saying so.

func TestAutostartMaxBeadsAlwaysPresent(t *testing.T) {
	cases := []struct {
		name, config, wantN string
		warn                bool
	}{
		{name: "unset", config: armed, wantN: "3"},
		{name: "explicit-3", config: armed + "autostart_max_beads: 3\n", wantN: "3"},
		{name: "unbounded-0", config: armed + "autostart_max_beads: 0\n", wantN: "0"},
		{name: "raised-7", config: armed + "autostart_max_beads: 7\n", wantN: "7"},
		{name: "empty-value", config: armed + "autostart_max_beads:\n", wantN: "3"},
		{name: "commented-out", config: armed + "# autostart_max_beads: 99\n", wantN: "3"},
		{name: "word", config: armed + "autostart_max_beads: three\n", wantN: "3", warn: true},
		{name: "neg", config: armed + "autostart_max_beads: -1\n", wantN: "3", warn: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newHookWorld(t, c.config)
			r := w.run(t, "--startup")
			if r.code != 0 {
				t.Fatalf("exit %d:\n%s", r.code, r.out)
			}
			want := " -n " + c.wantN + " "
			if !strings.Contains(r.calls, want) {
				t.Errorf("want %q in the armed command:\n%s", want, r.calls)
			}
			warned := strings.Contains(r.out, "is not a count")
			if warned != c.warn {
				t.Errorf("warn=%v, want %v:\n%s", warned, c.warn, r.out)
			}
		})
	}
}

// The shape an operator actually types first: interval, max-interval, an
// explicit 3, dry-run. The cap has to survive composition, not only the
// absent-key default.
func TestAutostartFirstArmBlockCarriesTheCap(t *testing.T) {
	w := newHookWorld(t, ""+
		"autostart_interval: 5m\n"+
		"autostart_max_interval: 40m\n"+
		"autostart_max_beads: 3\n"+
		"autostart_dry_run: true\n")

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.calls, "dispatch --watch 5m --max-interval 40m -n 3 --resume --dry-run") {
		t.Errorf("first-arm block lost the cap:\n%s", r.calls)
	}
}

// ranger-base-g7lt. Every test above injects RHQ_HOME, which is exactly why
// none of them could see this: the hook's default was a second, disagreeing
// copy of newApp's both-paths read (internal/rhq/app.go). homeWorld runs the
// hook with RHQ_HOME UNSET and a scratch HOME, and its fake posse records the
// home each child inherited beside the argv — the arm decision and the
// session's home are two facts, and the bug was that they disagreed.
type homeWorld struct {
	user string // HOME
	home string // RHQ_HOME, when the test sets one; empty = unset
	log  string // calls.log: one `RHQ_HOME=<home> <argv>` per line
}

// newHomeWorld builds a scratch HOME. homes maps a directory name under
// ~/.config to its config.yaml body; an empty body creates the home empty.
func newHomeWorld(t *testing.T, homes map[string]string) *homeWorld {
	t.Helper()
	user := t.TempDir()
	for name, body := range homes {
		dir := filepath.Join(user, ".config", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if body == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := &homeWorld{user: user, log: filepath.Join(user, "calls.log")}
	// The liveness seam is crossed for real by the tests above; here the
	// probe only has to answer, so the fake says "none" and the hook goes on
	// to arm. Anything else and every case below would stand down on an
	// unanswerable probe instead of deciding a home.
	fake := "#!/usr/bin/env bash\n" +
		"echo \"RHQ_HOME=${RHQ_HOME-<unset>} $*\" >> " + shq(w.log) + "\n" +
		"if [ \"$1 $2\" = 'dispatch --watch-status' ]; then echo 'watch-loop: none'; exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(user, "posse"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	return w
}

// run invokes the hook with a built-from-nothing environment: the developer's
// own RHQ_HOME must not be able to reach a test about what happens when there
// is none.
func (w *homeWorld) run(t *testing.T) hookRun {
	t.Helper()
	hook, err := filepath.Abs(filepath.Join("..", "..", "plugin", "autostart.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", hook, "--startup")
	cmd.Env = []string{
		"HOME=" + w.user,
		"PATH=" + os.Getenv("PATH"),
		"RHQ_BIN=" + filepath.Join(w.user, "posse"),
	}
	if w.home != "" {
		cmd.Env = append(cmd.Env, "RHQ_HOME="+w.home)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(w.log)
	return hookRun{out: string(out), code: code, calls: string(calls)}
}

func (w *homeWorld) path(name string) string { return filepath.Join(w.user, ".config", name) }

// childHome is the RHQ_HOME every invoked posse saw, or "" if they disagreed
// or none ran. The disagreement is the point: an arm read from one home that
// launches the loop into another is the half of ranger-base-g7lt that a
// "which config did it read" assertion alone would miss.
func childHome(t *testing.T, calls string) string {
	t.Helper()
	seen := ""
	for _, line := range strings.Split(strings.TrimSpace(calls), "\n") {
		if line == "" {
			continue
		}
		got := strings.TrimPrefix(strings.Fields(line)[0], "RHQ_HOME=")
		if seen != "" && got != seen {
			t.Errorf("children disagreed about the home:\n%s", calls)
			return ""
		}
		seen = got
	}
	return seen
}

// The whole decision table, read against newApp's: RHQ_HOME wins; otherwise
// ~/.config/posse unless it does not exist and ~/.config/rhq does. Both
// halves are asserted every time — which config armed it, and which home the
// session it started will run out of.
func TestAutostartDefaultHomeMatchesNewApp(t *testing.T) {
	cases := []struct {
		name   string
		homes  map[string]string
		want   string // the home that must both arm and be exported
		armed  bool
		absent string // a home under ~/.config the hook must not create
	}{
		// The fresh install INSTALL.md describes: `posse init` seeded
		// ~/.config/posse, nothing in the profile. This stayed disarmed.
		{name: "fresh-posse-only", homes: map[string]string{"posse": armed}, want: "posse", armed: true, absent: "rhq"},
		// This operator, and every install that predates the rename.
		{name: "legacy-only", homes: map[string]string{"rhq": armed}, want: "rhq", armed: true, absent: "posse"},
		// Both present: posse wins, as newApp does — including the
		// arm switch living only in the home that lost.
		{name: "both-armed-in-posse", homes: map[string]string{"posse": armed, "rhq": ""}, want: "posse", armed: true},
		{name: "both-armed-in-legacy", homes: map[string]string{"posse": "", "rhq": armed}, want: "posse", armed: false},
		// Nothing installed at all: name the preferred home in the
		// disarm line and create neither.
		{name: "neither", homes: nil, want: "posse", armed: false, absent: "rhq"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newHomeWorld(t, c.homes)
			r := w.run(t)
			if r.code != 0 {
				t.Fatalf("exit %d:\n%s", r.code, r.out)
			}
			want := w.path(c.want)
			if c.armed {
				if strings.Contains(r.out, "disarmed") {
					t.Fatalf("%s is armed but the hook stood down:\n%s", want, r.out)
				}
				if !strings.Contains(r.calls, "dispatch --watch") {
					t.Fatalf("no watch loop launched:\n%s\n%s", r.calls, r.out)
				}
				if got := childHome(t, r.calls); got != want {
					t.Errorf("armed from %s but handed posse RHQ_HOME=%s:\n%s", want, got, r.calls)
				}
			} else {
				if !strings.Contains(r.out, "disarmed (no autostart_interval: in "+filepath.Join(want, "config.yaml")+")") {
					t.Errorf("disarm line must name %s:\n%s", want, r.out)
				}
				if strings.Contains(r.calls, "dispatch --watch ") {
					t.Errorf("disarmed run launched a loop:\n%s", r.calls)
				}
			}
			if c.absent != "" {
				if _, err := os.Stat(w.path(c.absent)); !os.IsNotExist(err) {
					t.Errorf("hook created %s: %v", w.path(c.absent), err)
				}
			}
		})
	}
}

// RHQ_HOME still wins over both directories, and is still what the children
// get — the case the whole suite above depends on.
func TestAutostartExplicitHomeWinsOverBoth(t *testing.T) {
	w := newHomeWorld(t, map[string]string{"posse": "", "rhq": ""})
	elsewhere := filepath.Join(w.user, "instances", "fleet")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(elsewhere, "config.yaml"), []byte(armed), 0o644); err != nil {
		t.Fatal(err)
	}
	w.home = elsewhere

	r := w.run(t)
	if r.code != 0 || strings.Contains(r.out, "disarmed") {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if got := childHome(t, r.calls); got != elsewhere {
		t.Errorf("RHQ_HOME=%s did not reach posse (got %s):\n%s", elsewhere, got, r.calls)
	}
	if strings.Contains(r.out, "does not exist; using existing home") {
		t.Errorf("explicit home printed the transition notice:\n%s", r.out)
	}
}

// The fallback is announced once on stderr, the way posse announces it, and
// it says nothing was moved — an operator who sees the fleet arm out of the
// old home has to be able to tell that from a migration.
func TestAutostartLegacyFallbackSaysSo(t *testing.T) {
	w := newHomeWorld(t, map[string]string{"rhq": armed})

	r := w.run(t)
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	want := w.path("posse") + " does not exist; using existing home " + w.path("rhq") + " (nothing moved)"
	if !strings.Contains(r.out, want) {
		t.Errorf("want the transition notice %q:\n%s", want, r.out)
	}
}

// os.Stat's two edges, which the shell test has to match exactly or the hook
// and newApp part company on the odd install. A dangling ~/.config/posse
// symlink is IsNotExist, so it falls back; a ~/.config/posse that is a plain
// file is not, so it does not — newApp keeps it and `posse init` fails there
// loudly, which is the outcome an operator can act on.
func TestAutostartHomeStatEdgesMatchNewApp(t *testing.T) {
	t.Run("dangling-symlink-falls-back", func(t *testing.T) {
		w := newHomeWorld(t, map[string]string{"rhq": armed})
		if err := os.Symlink(filepath.Join(w.user, "gone"), w.path("posse")); err != nil {
			t.Fatal(err)
		}
		r := w.run(t)
		if r.code != 0 || strings.Contains(r.out, "disarmed") {
			t.Fatalf("dangling posse symlink did not fall back (exit %d):\n%s", r.code, r.out)
		}
		if got := childHome(t, r.calls); got != w.path("rhq") {
			t.Errorf("armed with RHQ_HOME=%s, want %s:\n%s", got, w.path("rhq"), r.calls)
		}
	})
	t.Run("plain-file-does-not", func(t *testing.T) {
		w := newHomeWorld(t, map[string]string{"rhq": armed})
		if err := os.WriteFile(w.path("posse"), []byte("not a home\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		r := w.run(t)
		if r.code != 0 {
			t.Fatalf("exit %d:\n%s", r.code, r.out)
		}
		if !strings.Contains(r.out, "disarmed (no autostart_interval: in "+filepath.Join(w.path("posse"), "config.yaml")+")") {
			t.Errorf("a posse *file* must not fall back to the legacy home:\n%s", r.out)
		}
	})
}
