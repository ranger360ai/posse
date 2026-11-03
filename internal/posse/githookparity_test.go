//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-tpvkc: the managed-hook git config row. The fixture system file
// is injected as the operator's resolved system path, so nothing here reads
// this box's /etc/gitconfig (absent here, MEASURED 2026-11-03).

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const fixtureManagedHooks = "/opt/managed/hooks"

// hookParityFixture writes the fixture system file and returns it with a
// base env whose HOME and XDG_CONFIG_HOME are empty dirs, so global scope is
// empty in both views.
func hookParityFixture(t *testing.T, body string) (sys string, base []string, dir string) {
	t.Helper()
	root := t.TempDir()
	sys = filepath.Join(root, "gitconfig")
	if err := os.WriteFile(sys, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	dir = filepath.Join(root, "work")
	for _, d := range []string{home, dir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	base = envPut(os.Environ(), "HOME", home)
	base = envPut(base, "XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	// A stray GIT_CONFIG_* in the test's own env must not reach either view.
	base = append(base, "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	return sys, base, dir
}

func hookParityText(r gitHookParityResult) string { return strings.Join(r.Lines, "\n") }

// Arm a: today's inletPin value. RED, naming system scope, the fixture file
// and NOT CARRIED.
func TestHookParitySeatOnDevNullIsRed(t *testing.T) {
	t.Parallel()
	sys, base, dir := hookParityFixture(t, "[core]\n\thooksPath = "+fixtureManagedHooks+"\n")
	r := gitHookParity(gitHookParityInput{
		Base: base, OperatorSystem: sys, Dir: dir,
		Seat: []EnvVar{{"GIT_CONFIG_SYSTEM", os.DevNull}},
	})
	out := hookParityText(r)
	if !r.Red {
		t.Fatalf("a seat on /dev/null read clean over a system-scope managed hook:\n%s", out)
	}
	for _, want := range []string{"NOT CARRIED", "system file:" + sys, "core.hookspath=" + fixtureManagedHooks, "operator system scope " + sys} {
		if !strings.Contains(out, want) {
			t.Errorf("row does not say %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "no managed hooks") {
		t.Errorf("a not-carried scope read as an empty one:\n%s", out)
	}
}

// Arm b: the seat carries the resolved path (ranger-base-t301k's fix).
func TestHookParitySeatCarryingSystemPathIsClean(t *testing.T) {
	t.Parallel()
	sys, base, dir := hookParityFixture(t, "[core]\n\thooksPath = "+fixtureManagedHooks+"\n")
	r := gitHookParity(gitHookParityInput{
		Base: base, OperatorSystem: sys, Dir: dir,
		Seat: []EnvVar{{"GIT_CONFIG_SYSTEM", sys}},
	})
	if r.Red {
		t.Fatalf("a seat carrying the system file read red:\n%s", hookParityText(r))
	}
	if !strings.Contains(hookParityText(r), "operator's 1 hook config entries") {
		t.Errorf("clean row did not count the fixture entry, so it may be vacuous:\n%s", hookParityText(r))
	}
}

// Arm c: the redirect forwards somewhere other than the operator's
// effective hooksPath. The system entry stays listed in the seat's view, so
// only the redirect arm can see this.
func TestHookParityRedirectToTheWrongPlaceIsRed(t *testing.T) {
	t.Parallel()
	sys, base, dir := hookParityFixture(t, "[core]\n\thooksPath = "+fixtureManagedHooks+"\n")
	hooks := filepath.Join(t.TempDir(), "state", "hooks", "s")
	seat := []EnvVar{{"GIT_CONFIG_SYSTEM", sys}}
	seat = append(seat, gitConfigHooksPathVars(seat, hooks)...)

	wrong := gitHookParity(gitHookParityInput{
		Base: base, OperatorSystem: sys, Dir: dir, Seat: seat,
		Redirect: &l3Redirect{Hooks: hooks, Managed: "/opt/elsewhere/hooks"},
	})
	out := hookParityText(wrong)
	if !wrong.Red {
		t.Fatalf("a redirect forwarding to the wrong dir read clean:\n%s", out)
	}
	for _, want := range []string{"/opt/elsewhere/hooks", fixtureManagedHooks, "forwards into"} {
		if !strings.Contains(out, want) {
			t.Errorf("redirect finding does not name %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "missing from the seat") {
		t.Errorf("the listing arm fired too, so this arm is not what caught it:\n%s", out)
	}

	// Control: the same redirect forwarding to the operator's own path.
	right := gitHookParity(gitHookParityInput{
		Base: base, OperatorSystem: sys, Dir: dir, Seat: seat,
		Redirect: &l3Redirect{Hooks: hooks, Managed: fixtureManagedHooks},
	})
	if right.Red {
		t.Fatalf("a redirect forwarding to the operator's hooksPath read red:\n%s", hookParityText(right))
	}
}

// Arm d: GIT_CONFIG_NOSYSTEM in the seat env.
func TestHookParitySeatWithNoSystemIsRed(t *testing.T) {
	t.Parallel()
	sys, base, dir := hookParityFixture(t, "[core]\n\thooksPath = "+fixtureManagedHooks+"\n")
	r := gitHookParity(gitHookParityInput{
		Base: base, OperatorSystem: sys, Dir: dir,
		Seat: []EnvVar{{"GIT_CONFIG_NOSYSTEM", "1"}},
	})
	out := hookParityText(r)
	if !r.Red || !strings.Contains(out, "NOT CARRIED (GIT_CONFIG_NOSYSTEM") {
		t.Fatalf("GIT_CONFIG_NOSYSTEM did not read as not carried:\n%s", out)
	}
}

// An operator system file that does not exist is said, not passed silently.
func TestHookParityAbsentSystemFileIsSaid(t *testing.T) {
	t.Parallel()
	_, base, dir := hookParityFixture(t, "")
	missing := filepath.Join(t.TempDir(), "no-such-gitconfig")
	r := gitHookParity(gitHookParityInput{
		Base: base, OperatorSystem: missing, Dir: dir,
		Seat: []EnvVar{{"GIT_CONFIG_SYSTEM", os.DevNull}},
	})
	out := hookParityText(r)
	if r.Red {
		t.Fatalf("an absent system file read red:\n%s", out)
	}
	if !strings.Contains(out, "system scope "+missing+" absent: nothing to carry") {
		t.Errorf("absent system file not named:\n%s", out)
	}
	if !strings.Contains(out, "seat system scope NOT CARRIED") {
		t.Errorf("the seat's /dev/null was hidden behind the vacuous pass:\n%s", out)
	}
}

// Config-based hooks (hook.<name>.*) exist from git 2.54. ASSUMED from the
// operator's 2.54 run; this box's 2.50.1 has none, so the arm skips there.
func TestHookParityConfigHookMissingIsRed(t *testing.T) {
	t.Parallel()
	out, err := exec.Command("git", "version").Output()
	if err != nil {
		t.Skip("git version:", err)
	}
	m := regexp.MustCompile(`(\d+)\.(\d+)`).FindStringSubmatch(string(out))
	if m == nil {
		t.Skipf("cannot read %q", out)
	}
	maj, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if maj < 2 || (maj == 2 && minor < 54) {
		t.Skipf("%s has no config-based hooks (needs 2.54)", strings.TrimSpace(string(out)))
	}
	sys, base, dir := hookParityFixture(t, "[hook \"scan\"]\n\tevent = pre-commit\n\tcommand = /opt/managed/scan\n")
	r := gitHookParity(gitHookParityInput{
		Base: base, OperatorSystem: sys, Dir: dir,
		Seat: []EnvVar{{"GIT_CONFIG_SYSTEM", os.DevNull}},
	})
	if !r.Red || !strings.Contains(hookParityText(r), "hook.scan.command=/opt/managed/scan") {
		t.Fatalf("a system-scope config hook missing from the seat did not read red:\n%s", hookParityText(r))
	}
}

func TestParseGitHookEntries(t *testing.T) {
	t.Parallel()
	raw := "system\x00file:/etc/gitconfig\x00core.hookspath\n/opt/m\x00command\x00command line:\x00core.hookspath\n/s/h\x00"
	got := parseGitHookEntries([]byte(raw))
	want := []gitHookEntry{
		{"system", "file:/etc/gitconfig", "core.hookspath", "/opt/m"},
		{"command", "command line:", "core.hookspath", "/s/h"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if es := parseGitHookEntries(nil); len(es) != 0 {
		t.Errorf("empty output parsed to %+v", es)
	}
}

// The seat's git pin follows the runtime: claude renders inletPin's
// GIT_CONFIG rows, a runtime without the settings pin renders none.
func TestSeatGitPinFollowsTheRuntime(t *testing.T) {
	t.Parallel()
	claude := &Runtime{Name: "claude", FleetSettings: ClaudeFleetSettingsJSON}
	found := false
	for _, v := range seatGitPin(claude) {
		if !strings.HasPrefix(v.Key, "GIT_CONFIG") {
			t.Errorf("seat git pin carries a non-git row %s", v.Key)
		}
		if v.Key == "GIT_CONFIG_SYSTEM" {
			found = true
		}
	}
	if !found {
		t.Error("claude's seat git pin does not carry GIT_CONFIG_SYSTEM")
	}
	if pin := seatGitPin(&Runtime{Name: "codex"}); len(pin) != 0 {
		t.Errorf("a runtime with no settings pin rendered %+v", pin)
	}
}
