//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ADR 0079 (ranger-base-32ll0, from ranger-base-z8kym item 3): the
// hook-skip arm judges git's GLOBAL scope against the launcher's resolved
// global files, for HOME, XDG_CONFIG_HOME and GIT_CONFIG_GLOBAL alike.
//
// Each pin protects the refusal on a hook verb: delete one and a seat skips
// the managed hook at rc 0. Pins 1-6 run REAL git through the seat launch line
// (WrapWithGates) on claude AND codex, with the render's env set to the
// fixture's (App.GitScopeEnv), so the scratch HOME is the launcher's. Pin 7
// renders the shim over a stub git and counts its execs. The scratch dirs are
// not the tree, so none of this is a tree-wide pin and it owes the Makefile no
// door.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// hsGlobalLeg is one typed line and whether the arm must refuse it.
type hsGlobalLeg struct {
	name, env string
	refused   bool
}

// hsWrapped renders the gates for rtName with the fixture's env as the
// launcher's, and returns the typed launch prefix.
func hsWrapped(t *testing.T, fx *hsFixture, rtName string) string {
	t.Helper()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), GitScopeEnv: hsEnv(fx)}
	rt, err := a.LoadRuntime(rtName)
	if err != nil {
		t.Fatal(err)
	}
	fx.hooksRoot = filepath.Join(a.StateDir, "hooks")
	wrapped, _, _, err := a.WrapWithGates("hg-"+rtName, rt, []string{"Bash(git push:*)", "Bash(git commit unless --)"}, "")
	if err != nil {
		t.Fatal(err)
	}
	return wrapped
}

// hsGlobalCheck runs each leg as a path-limited commit and asserts the
// verdict and the hook: refused means hookskip:config-env and no fire;
// allowed means rc 0 and exactly one fire.
func hsGlobalCheck(t *testing.T, fx *hsFixture, wrapped string, legs []hsGlobalLeg) {
	t.Helper()
	for i, leg := range legs {
		before := hsFired(fx)
		// hsRun writes the argv into f, so a distinct message per leg is
		// what gives each commit something to commit.
		errs, code := hsRun(t, fx, wrapped, hsLeg{env: leg.env, args: "commit -q -m g" + strconv.Itoa(i) + " -- f"})
		fired := hsFired(fx) - before
		if leg.refused {
			if code == 0 || !strings.Contains(errs, "rule: hookskip:config-env,") || !strings.Contains(errs, "ADR 0079") {
				t.Errorf("%s (%sgit commit): code=%d, want refused by hookskip:config-env naming ADR 0079: %q", leg.name, leg.env, code, errs)
			}
			if fired != 0 {
				t.Errorf("%s: the hook ran %d times — the refusal came after git", leg.name, fired)
			}
			continue
		}
		if code != 0 || fired != 1 || strings.Contains(errs, "hookskip:") {
			t.Errorf("%s (%sgit commit): code=%d fired=%d, want 0 and 1 (%q)", leg.name, leg.env, code, fired, errs)
		}
	}
}

// Pins 1, 3, 4 and 6, over the managed hooksPath in ~/.gitconfig (the one-file
// box obdnk pins GIT_CONFIG_GLOBAL on).
func TestQAHookSkipMovedHomeJudgedAgainstLauncher(t *testing.T) {
	t.Parallel()
	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			wrapped := hsWrapped(t, fx, rtName)
			pin := filepath.Join(fx.home, ".gitconfig")

			// A HOME holding no hooksPath; an "evil" HOME holding its own
			// .gitconfig; a HOME holding a byte-identical copy of the managed
			// config; and an empty one.
			bare := filepath.Join(fx.home, "bare-home")
			evil := filepath.Join(fx.home, "evil-home")
			same := filepath.Join(fx.home, "same-home")
			empty := filepath.Join(fx.home, "empty-home")
			for _, d := range []string{bare, evil, same, empty} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			write(t, filepath.Join(bare, ".gitconfig"), "[user]\n\tname = t\n\temail = t@example.com\n")
			write(t, filepath.Join(evil, ".gitconfig"), "[core]\n\thooksPath = "+fx.otherHooks+"\n[user]\n\tname = t\n\temail = t@example.com\n")
			managed, err := os.ReadFile(pin)
			if err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(same, ".gitconfig"), string(managed))

			// The control: the plain commit, nothing moved.
			hsGlobalCheck(t, fx, wrapped, []hsGlobalLeg{{name: "control: nothing moved", env: ""}})

			hsGlobalCheck(t, fx, wrapped, []hsGlobalLeg{
				// Pin 1: z8kym item 3's HOME leg.
				{"pin 1: HOME lacking the managed hooksPath", "HOME=" + shQuote(bare) + " ", true},
				// Pin 3.
				{"pin 3: env -u HOME", "env -u HOME ", true},
				// Pin 4: ADR 0079 Context 2's two measured bypasses of the
				// unset-baseline arm.
				{"pin 4: HOME=evil GIT_CONFIG_GLOBAL=evil/.gitconfig", "HOME=" + shQuote(evil) + " GIT_CONFIG_GLOBAL=" + shQuote(filepath.Join(evil, ".gitconfig")) + " ", true},
				{"pin 4: HOME=empty GIT_CONFIG_GLOBAL=/dev/null", "HOME=" + shQuote(empty) + " GIT_CONFIG_GLOBAL=/dev/null ", true},
				// Pin 6: equal content is allowed, and the hook runs.
				{"pin 6: HOME at an identical config", "HOME=" + shQuote(same) + " ", false},
				{"pin 6: pinned GIT_CONFIG_GLOBAL plus HOME=<empty>", "HOME=" + shQuote(empty) + " GIT_CONFIG_GLOBAL=" + shQuote(pin) + " ", false},
				// The pinned value with nothing else moved (a claude seat as
				// launched): allowed.
				{"pinned GIT_CONFIG_GLOBAL alone", "GIT_CONFIG_GLOBAL=" + shQuote(pin) + " ", false},
			})
		})
	}
}

// Pins 2 and 5, over the managed hooksPath in the XDG file with ~/.gitconfig
// beside it — the two-file box, where obdnk pins nothing.
func TestQAHookSkipXDGMoveJudgedAgainstLauncher(t *testing.T) {
	t.Parallel()
	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			dotfile := filepath.Join(fx.home, ".gitconfig")
			managed, err := os.ReadFile(dotfile)
			if err != nil {
				t.Fatal(err)
			}
			xdgFile := filepath.Join(fx.home, ".config", "git", "config")
			if err := os.MkdirAll(filepath.Dir(xdgFile), 0o755); err != nil {
				t.Fatal(err)
			}
			// The hooksPath moves to the XDG file; ~/.gitconfig keeps only the
			// identity. git reads both, XDG first.
			write(t, xdgFile, string(managed))
			write(t, dotfile, "[user]\n\tname = t\n\temail = t@example.com\n")
			wrapped := hsWrapped(t, fx, rtName)

			other := filepath.Join(fx.home, "other-xdg")
			if err := os.MkdirAll(other, 0o755); err != nil {
				t.Fatal(err)
			}

			hsGlobalCheck(t, fx, wrapped, []hsGlobalLeg{
				// The control, which also proves the XDG hook is read at all.
				{"control: nothing moved, XDG hook fires", "", false},
				// Pin 2.
				{"pin 2: XDG_CONFIG_HOME=<other>", "XDG_CONFIG_HOME=" + shQuote(other) + " ", true},
				// Pin 5: ADR 0079 Context 3. `git config --global` reads only
				// ~/.gitconfig here, so a compare made through it calls this
				// equal; the --show-scope listing does not.
				{"pin 5: GIT_CONFIG_GLOBAL=~/.gitconfig hides the XDG hook", "GIT_CONFIG_GLOBAL=" + shQuote(dotfile) + " ", true},
				// XDG pointed at the same dir it resolves to anyway: equal.
				{"XDG_CONFIG_HOME at its own default", "XDG_CONFIG_HOME=" + shQuote(filepath.Join(fx.home, ".config")) + " ", false},
			})
		})
	}
}

// Pin 7 (ADR 0079 D3): the plain commit with HOME, XDG_CONFIG_HOME and
// GIT_CONFIG_GLOBAL as rendered execs no `git config` — on claude, whose seat
// carries the obdnk pin, and on codex, whose seat carries none. A stub git
// records every exec, as TestQAPIDDenyAliasLookupIsTheOnlyExec's does. The
// witness: a moved HOME on the same shim DOES exec the compare, so an empty
// count is the gate and not a compare that never renders.
func TestQAHookSkipGlobalGateExecsNothingAsRendered(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	// One global file, so the reference carries a pin.
	write(t, filepath.Join(home, ".gitconfig"), "[user]\n\tname = t\n")
	execs := filepath.Join(dir, "execs")
	stub := filepath.Join(dir, "git")
	script := "#!/bin/sh\necho \"$*\" >> " + shQuote(execs) + "\ncase \"$1\" in var) exit 1 ;; config) exit 1 ;; esac\n"
	if err := WriteExecutable(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GIT_CONFIG") || strings.HasPrefix(kv, "RHQ_GATE_") || strings.HasPrefix(kv, "HOME=") || strings.HasPrefix(kv, "XDG_CONFIG_HOME=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "HOME="+home)
	ref := resolveHookSkipGlobalRef(stub, env)
	if want := filepath.Join(home, ".gitconfig"); ref.Pin != want {
		t.Fatalf("the reference must carry obdnk's pin %q, got %+v", want, ref)
	}
	shim := filepath.Join(dir, "shim")
	body := renderShim("hg-canary", "git", stub, filepath.Join(dir, "refusals.log"), "/bin/date", nil, nil, "", filepath.Join(dir, "hooks"), ref)
	if err := WriteExecutable(shim, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(extra ...string) []string {
		os.Remove(execs)
		cmd := exec.Command(shim, "commit", "-m", "x", "--", "f")
		cmd.Env = append(append([]string{}, env...), extra...)
		cmd.CombinedOutput()
		b, _ := os.ReadFile(execs)
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
	for _, c := range []struct {
		name  string
		extra []string
	}{
		{"claude: GIT_CONFIG_GLOBAL at the pin", []string{"GIT_CONFIG_GLOBAL=" + ref.Pin}},
		{"codex: no GIT_CONFIG_GLOBAL", nil},
	} {
		if got := run(c.extra...); strings.Join(got, "|") != "commit -m x -- f" {
			t.Errorf("%s: the plain commit must exec git once, for itself; got %q", c.name, got)
		}
	}
	got := run("HOME=" + dir)
	n := 0
	for _, l := range got {
		if strings.HasPrefix(l, "config ") {
			n++
		}
	}
	// 2 for the have side, one per candidate (2) for the want side.
	if n != 2+len(ref.Cands) {
		t.Errorf("WITNESS: a moved HOME must run the compare (%d git config execs), got %q", 2+len(ref.Cands), got)
	}
}

// The two places the build goes past ADR 0079's text (hookskip.go,
// renderHookSkipGlobalScope and renderHookSkipGlobalGate), both over an
// operator global file whose hooksPath sits behind `include.path =
// ~/.gitconfig.inc`. git expands that `~` from the CALL's HOME, so a moved
// HOME holding a byte-identical .gitconfig and its own .gitconfig.inc follows
// the attacker's include:
//   - want read under the call's HOME would follow it too and call the two
//     equal. The want reads run under the launcher's HOME.
//   - D3 as written skips the compare when GIT_CONFIG_GLOBAL is the pin,
//     yet the pinned file's include still expands from the moved HOME.
func TestQAHookSkipMovedHomeFollowsNoTildeInclude(t *testing.T) {
	t.Parallel()
	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			dotfile := filepath.Join(fx.home, ".gitconfig")
			managed, err := os.ReadFile(dotfile)
			if err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(fx.home, ".gitconfig.inc"), string(managed))
			top := "[include]\n\tpath = ~/.gitconfig.inc\n"
			write(t, dotfile, top)
			wrapped := hsWrapped(t, fx, rtName)

			evil := filepath.Join(fx.home, "evil-home")
			if err := os.MkdirAll(evil, 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(evil, ".gitconfig"), top)
			write(t, filepath.Join(evil, ".gitconfig.inc"), "[user]\n\tname = t\n\temail = t@example.com\n")

			hsGlobalCheck(t, fx, wrapped, []hsGlobalLeg{
				{"control: the included hook fires", "", false},
				{"HOME at an identical top file, its own include", "HOME=" + shQuote(evil) + " ", true},
				{"pinned GIT_CONFIG_GLOBAL, HOME moved under its include", "HOME=" + shQuote(evil) + " GIT_CONFIG_GLOBAL=" + shQuote(dotfile) + " ", true},
			})
		})
	}
}
