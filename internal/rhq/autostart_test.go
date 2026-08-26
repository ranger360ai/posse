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
}

func newHookWorld(t *testing.T, config string) *hookWorld {
	t.Helper()
	home := t.TempDir()
	w := &hookWorld{
		home:   home,
		exists: filepath.Join(home, "session-exists"),
		deaf:   filepath.Join(home, "probe-deaf"),
		noisy:  filepath.Join(home, "probe-noisy"),
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
		"kill) rm -f " + shq(w.exists) + "; exit 0 ;;\n" +
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

// By hand, with no --startup, nothing is ever killed.
func TestAutostartByHandNeverKills(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)

	r := w.run(t)
	if r.code != 0 || strings.Contains(r.calls, "kill") {
		t.Errorf("exit %d, calls:\n%s", r.code, r.calls)
	}
	if !strings.Contains(r.out, "already running — left alone") {
		t.Errorf("want the conservative report:\n%s", r.out)
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
