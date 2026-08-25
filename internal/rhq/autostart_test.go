package rhq

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// plugin/autostart.sh under test. herdr runs it as `bash autostart.sh
// --startup` once per server start — including the start of a live-handoff
// import server, which is the case rangerhq-ct9 is about. The hook is
// driven here against a fake posse, so nothing real is created.

type hookRun struct {
	out   string
	code  int
	calls string // one `posse <argv>` per line
}

type hookWorld struct {
	home   string // RHQ_HOME
	exists string // touched while the fake posse believes the session exists
	path   string // prepended to PATH, where a test shadows a system tool
}

func newHookWorld(t *testing.T, config string) *hookWorld {
	t.Helper()
	home := t.TempDir()
	w := &hookWorld{home: home, exists: filepath.Join(home, "session-exists")}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := "#!/usr/bin/env bash\n" +
		"echo \"$*\" >> " + shq(filepath.Join(home, "calls.log")) + "\n" +
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

// breakPs shadows `ps` with one that cannot answer, which is what the hook
// meets on a slim image, under a sandbox that denies process inspection, or
// with a restricted PATH. Shadowing beats un-setting PATH: the hook still
// needs sed, head and the rest to get as far as the probe.
func (w *hookWorld) breakPs(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte("#!/bin/sh\nexit 127\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.path = dir
}

// psCanReadArgv reports whether this box's `ps` can name a process by pid.
// Only the pid-reuse guard needs that — refuting a live pid is the one
// question an unanswerable probe cannot decide, so the test for it is
// skipped rather than failed where the answer is unavailable (rangerhq-mugy).
func psCanReadArgv(t *testing.T) bool {
	t.Helper()
	out, err := exec.Command("ps", "-p", strconv.Itoa(os.Getpid()), "-o", "command=").Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
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
	)
	if w.path != "" {
		cmd.Env = append(cmd.Env, "PATH="+w.path+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
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

// The bug: herdr runs [[startup]] hooks on a live handoff too, where the
// workspace comes back WITH its command still running. The hook must leave
// it alone — killing it drops a claimed bead and a prompt in flight, and
// starts a second loop on the same queue (rangerhq-ct9).
func TestAutostartLeavesACarriedOverLoopAlone(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	pid := liveProcess(t, "posse-dispatch-watch-stub")
	if err := WriteWatchPid(WatchPidPath(&App{StateDir: filepath.Join(w.home, "state")}), WatchPid{Pid: pid}); err != nil {
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
	if r.calls != "" {
		t.Errorf("a live loop must not invoke posse at all:\n%s", r.calls)
	}
}

// Same carry-over, but the process argv is the production shape
// (`posse dispatch --watch …`), not a stub whose *path* happens to contain
// "dispatch". The hook greps `ps -o command=`, so a test that only puts
// the word in the filename would pass a matcher that never sees real args.
func TestAutostartLeavesACarriedOverWatchArgvAlone(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	pid := liveArgv(t, "posse dispatch --watch 5m --dry-run")
	if err := WriteWatchPid(WatchPidPath(&App{StateDir: filepath.Join(w.home, "state")}), WatchPid{Pid: pid}); err != nil {
		t.Fatal(err)
	}

	r := w.run(t, "--startup")
	if r.code != 0 || strings.Contains(r.calls, "kill") || strings.Contains(r.calls, "new") {
		t.Errorf("exit %d, calls:\n%s\nout:\n%s", r.code, r.calls, r.out)
	}
	if !strings.Contains(r.out, "left alone") {
		t.Errorf("want the hook to say it stood down:\n%s", r.out)
	}
}

// One loop per queue, not per session name: a --watch the operator started
// by hand in another workspace still owns the pidfile, so --startup must
// not create a second loop under autostart_session.
func TestAutostartStandsDownWhenLoopLivesUnderAnotherName(t *testing.T) {
	w := newHookWorld(t, armed)
	// no sessionExists: autostart_session is a name that is not running
	pid := liveArgv(t, "posse dispatch --watch 30s")
	if err := WriteWatchPid(WatchPidPath(&App{StateDir: filepath.Join(w.home, "state")}), WatchPid{Pid: pid}); err != nil {
		t.Fatal(err)
	}

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Errorf("exit %d, want 0:\n%s", r.code, r.out)
	}
	if r.calls != "" {
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
}

// A loop killed with its pane never removes its pidfile. Existence is not
// liveness — the stale record must not keep the fleet unarmed.
func TestAutostartReplacesAHuskWithAStaleRecord(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	done := exec.Command("sh", "-c", "exit 0")
	if err := done.Run(); err != nil {
		t.Fatal(err)
	}
	WriteWatchPid(WatchPidPath(&App{StateDir: filepath.Join(w.home, "state")}), WatchPid{Pid: done.Process.Pid})

	r := w.run(t, "--startup")
	if !strings.Contains(r.calls, "kill dispatch") || !strings.Contains(r.calls, "--watch 30s") {
		t.Errorf("stale record must read as no loop:\n%s\n%s", r.out, r.calls)
	}
}

// Pid reuse: the recorded number is alive, but it belongs to something else.
// Matching the argv is what keeps that from reading as a running loop.
func TestAutostartIgnoresARecycledPid(t *testing.T) {
	if !psCanReadArgv(t) {
		t.Skip("no usable `ps -p -o command=`: pid reuse cannot be refuted here")
	}
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	pid := liveProcess(t, "somebody-elses-sleeper")
	WriteWatchPid(WatchPidPath(&App{StateDir: filepath.Join(w.home, "state")}), WatchPid{Pid: pid})

	r := w.run(t, "--startup")
	if !strings.Contains(r.calls, "kill dispatch") || !strings.Contains(r.calls, "--watch 30s") {
		t.Errorf("a recycled pid must not read as a running loop:\n%s\n%s", r.out, r.calls)
	}
}

// rangerhq-ppy9: grep -q dispatch matches any argv substring, so a recycled
// pid of a one-shot `posse dispatch --persona` (or a claude PID that says
// "dispatch") reads as the watch loop and the hook stands down.
func TestAutostartIgnoresARecycledOneShotDispatch(t *testing.T) {
	t.Skip("rangerhq-ppy9: grep -q dispatch matches a one-shot `posse dispatch --persona` as if it were the watch loop")
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	pid := liveArgv(t, "posse dispatch --persona qa -n 1")
	if err := WriteWatchPid(WatchPidPath(&App{StateDir: filepath.Join(w.home, "state")}), WatchPid{Pid: pid}); err != nil {
		t.Fatal(err)
	}

	r := w.run(t, "--startup")
	if !strings.Contains(r.calls, "kill dispatch") || !strings.Contains(r.calls, "--watch 30s") {
		t.Errorf("a one-shot dispatch is not the watch loop:\n%s\n%s", r.out, r.calls)
	}
}

// rangerhq-mugy: the argv probe can only REFUTE liveness. Where `ps` cannot
// answer at all, an alive pid must still read as a live loop — the old
// `ps ... | grep -q dispatch || return 1` read that silence as "no loop",
// killed the workspace and started a second loop against the queue the
// first one was still claiming beads from. Unarmed is visible; double
// dispatch is not.
func TestAutostartStandsDownWhenPsCannotAnswer(t *testing.T) {
	w := newHookWorld(t, armed)
	w.sessionExists(t)
	pid := liveArgv(t, "posse dispatch --watch 30s")
	if err := WriteWatchPid(WatchPidPath(&App{StateDir: filepath.Join(w.home, "state")}), WatchPid{Pid: pid}); err != nil {
		t.Fatal(err)
	}
	w.breakPs(t)

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Errorf("exit %d, want 0:\n%s", r.code, r.out)
	}
	if r.calls != "" {
		t.Errorf("an unanswerable probe must not replace a live loop:\n%s", r.calls)
	}
	if !strings.Contains(r.out, "left alone") {
		t.Errorf("want the hook to say it stood down:\n%s", r.out)
	}
	// A failed scan says so (rangerhq-llse): standing down on a pid nothing
	// could identify is not the same fact as standing down on a confirmed
	// loop, and the operator is the one who can tell them apart.
	if !strings.Contains(r.out, "could not identify") {
		t.Errorf("want the stand-down to name the failed probe:\n%s", r.out)
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
	if !strings.Contains(r.calls, "--resume") {
		t.Errorf("armed loop does not resume — a settled-but-open bead sits forever:\n%s", r.calls)
	}
}

// Off is available, and only by saying so. An operator who wants the warning
// and nothing else says false; nothing else in the file turns it off.
func TestAutostartResumeFalseDisarmsResume(t *testing.T) {
	w := newHookWorld(t, armed+"autostart_resume: false\n")

	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if strings.Contains(r.calls, "--resume") {
		t.Errorf("autostart_resume: false still armed --resume:\n%s", r.calls)
	}
	if !strings.Contains(r.calls, "dispatch --watch") {
		t.Errorf("the loop was not armed at all:\n%s", r.calls)
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
