package posse

// Helpers lifted out of autostart_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
	plain        string // touched to make it answer with the pre-n00wn line, naming no log
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
		plain:  filepath.Join(home, "probe-plain"),

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
		"  if [ -e " + shq(w.plain) + " ]; then echo 'watch-loop: none (~/.config/posse/state/dispatch-watch.lock is free)'; exit 0; fi\n" +
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

// runArgv runs the hook over one config and returns everything it asked of
// posse, with the world's own temp paths folded away so two worlds are
// comparable. The hook must have armed something: a stand-down would compare
// equal to another stand-down and pin nothing.
func runArgv(t *testing.T, config string) string {
	t.Helper()
	w := newHookWorld(t, config)
	r := w.run(t, "--startup")
	if r.code != 0 {
		t.Fatalf("exit %d on config %q:\n%s", r.code, config, r.out)
	}
	if !strings.Contains(r.calls, "dispatch --watch ") {
		t.Fatalf("no loop was armed on config %q — nothing to compare:\n%s\n%s", config, r.out, r.calls)
	}
	// A value posse accepts is never complained about; a hook that both
	// warned and armed would otherwise compare equal on argv alone.
	for _, complaint := range []string{"is not an interval", "is not a count", "not true/false"} {
		if strings.Contains(r.out, complaint) {
			t.Errorf("hook complained on a value posse reads fine (%q):\n%s", complaint, r.out)
		}
	}
	return strings.ReplaceAll(strings.TrimSpace(r.calls), w.home, "$RHQ_HOME")
}

// probeGet is the YamlGet control for a one-line config whose key the caller
// does not want to spell twice: it reads back whichever autostart key the
// line names.
func probeGet(t *testing.T, line, prefix string) string {
	t.Helper()
	key, _, ok := strings.Cut(line, ":")
	if !ok || !strings.HasPrefix(key, prefix) {
		t.Fatalf("not a %s line: %q", prefix, line)
	}
	probe := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(probe, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	return YamlGet(probe, key)
}

// runStandDown is runArgv's other half: the hook over one config, with the
// world's temp paths folded away so two worlds compare, and no requirement
// that anything was armed. What it returns is the whole refusal — exit code,
// every line said, and the argv (empty, for a stand-down) — because a
// disagreement between the two readers shows up in the WORDS as often as in
// the exit code.
func runStandDown(t *testing.T, config string) hookRun {
	t.Helper()
	w := newHookWorld(t, config)
	r := w.run(t, "--startup")
	r.out = strings.ReplaceAll(r.out, w.home, "$RHQ_HOME")
	r.calls = strings.ReplaceAll(strings.TrimSpace(r.calls), w.home, "$RHQ_HOME")
	return r
}

// ranger-base-g7lt. Every test above injects RHQ_HOME, which is exactly why
// none of them could see this: the hook's default was a second, disagreeing
// copy of newApp's both-paths read (internal/posse/app.go). homeWorld runs the
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

// plainProbe is a posse from before the loop wrote its own log: its
// `--watch-status` line names no log, which is the hook's signal that the
// record still depends on the tee.
func (w *hookWorld) plainProbe(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(w.plain, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}
