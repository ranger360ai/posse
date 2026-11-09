//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ADR 0079 (ranger-base-32ll0, from ranger-base-z8kym item 3): the
// hook-skip arm judges git's GLOBAL scope against the launcher's resolved
// global files, for HOME, XDG_CONFIG_HOME and GIT_CONFIG_GLOBAL alike.
//
// Each pin protects the refusal on a hook verb: delete one and a seat skips
// the managed hook at rc 0. Pins 1-6, 8 and 9 run REAL git through the seat
// launch line (WrapWithGates) on claude AND codex, with the render's env set
// to the fixture's (App.GitScopeEnv), so the scratch HOME is the launcher's.
// Pin 7 renders the shim over a stub git and counts its execs. Pins 8 and 9
// are the ranger-base-o6xr6 amendment's: the `~` include (shipped with
// ranger-base-32ll0) and D2's newline count (ranger-base-ava0x). The scratch
// dirs are not the tree, so none of this is a tree-wide pin and it owes the
// Makefile no door.

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

// hsWrappedDir renders the gates for rtName under persona, with the fixture's
// env as the launcher's, and returns the typed launch prefix and the bin dir
// holding the rendered git shim. Pin 9's witness patches that shim; every
// other pin wants only the prefix (hsWrapped).
func hsWrappedDir(t *testing.T, fx *hsFixture, rtName, persona string) (string, string) {
	t.Helper()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), GitScopeEnv: hsEnv(fx)}
	rt, err := a.LoadRuntime(rtName)
	if err != nil {
		t.Fatal(err)
	}
	fx.hooksRoot = filepath.Join(a.StateDir, "hooks")
	wrapped, gatesDir, _, err := a.WrapWithGates(persona, rt, []string{"Bash(git push:*)", "Bash(git commit unless --)"}, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	return wrapped, filepath.Join(gatesDir, "bin")
}

// hsWrapped renders the gates for rtName with the fixture's env as the
// launcher's, and returns the typed launch prefix.
func hsWrapped(t *testing.T, fx *hsFixture, rtName string) string {
	t.Helper()
	wrapped, _ := hsWrappedDir(t, fx, rtName, "hg-"+rtName)
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
	// 2 for the have side; for the want side 1 system read (ranger-base-87zme)
	// and one per candidate (2).
	if n != 3+len(ref.Cands) {
		t.Errorf("WITNESS: a moved HOME must run the compare (%d git config execs), got %q", 3+len(ref.Cands), got)
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
//
// What this pin does NOT witness, named here because its title reads as if it
// did (ranger-base-y9i3o, from ranger-base-pr6p8 finding 4): the want side
// following the include at all. What refuses the fixture below is the `~` in
// its include.path expanding differently under the two HOMEs, so dropping
// `--includes` from the want's candidate read leaves every leg green —
// MEASURED 2026-11-06. ADR 0079 pin 15,
// TestQAHookSkipGlobalIncludeIsFollowedOnTheWantSide, carries the no-`~`
// shape that needs the flag.
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

// Amendment pin (ranger-base-87zme, the bead's item 4): a HOME-relative
// hooksPath reached through SYSTEM scope, plus a moved HOME holding a
// byte-identical global file. The system arm only runs when GIT_CONFIG_SYSTEM
// or GIT_CONFIG_NOSYSTEM is set, and the global compare used to keep only
// `global` lines, so this call reached no compare and git ran
// <moved HOME>/managed-hooks/pre-commit at rc 0.
//
// The system file cannot be planted for real (it is the box's), so the shim
// is rendered over a git wrapper that points GIT_CONFIG_SYSTEM at a scratch
// file on EVERY exec — the shim's reads and the commit alike — which is what
// a box whose /etc/gitconfig carries the line looks like. The call itself sets
// no GIT_CONFIG_SYSTEM, so the system arm stays out of it.
func TestQAHookSkipSystemTildeHooksPathJudgedAgainstLauncher(t *testing.T) {
	t.Parallel()
	fx := hsSetup(t)
	user := "[user]\n\tname = t\n\temail = t@example.com\n"
	write(t, filepath.Join(fx.home, ".gitconfig"), user)
	sys := filepath.Join(fx.home, "system-config")
	write(t, sys, "[core]\n\thooksPath = ~/managed-hooks\n")

	dir := t.TempDir()
	wrapper := filepath.Join(dir, "git-wrapper")
	if err := WriteExecutable(wrapper, []byte("#!/bin/sh\nGIT_CONFIG_SYSTEM="+shQuote(sys)+"; export GIT_CONFIG_SYSTEM\nexec "+shQuote(fx.realGit)+" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ref := resolveHookSkipGlobalRef(wrapper, hsEnv(fx))
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	body := renderShim("hg-sys", "git", wrapper, filepath.Join(dir, "refusals.log"), "/bin/date", nil, nil, "", filepath.Join(dir, "hooks"), ref)
	if err := WriteExecutable(filepath.Join(bin, "git"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := "PATH=" + shQuote(bin) + ":\"$PATH\" "

	evil := filepath.Join(fx.home, "evil-home")
	if err := os.MkdirAll(filepath.Join(evil, "managed-hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(evil, ".gitconfig"), user)
	seatHook := filepath.Join(fx.home, "seat-hook-ran")
	if err := WriteExecutable(filepath.Join(evil, "managed-hooks", "pre-commit"),
		[]byte("#!/bin/sh\necho ran >> "+shQuote(seatHook)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	hsGlobalCheck(t, fx, prefix, []hsGlobalLeg{
		// The control: the managed hook is reached through the system tilde.
		{"control: nothing moved, the system tilde names the managed hook", "", false},
		{"HOME=evil, byte-identical global file, system hooksPath = ~/managed-hooks", "HOME=" + shQuote(evil) + " ", true},
	})
	if b, _ := os.ReadFile(seatHook); len(b) != 0 {
		t.Errorf("the seat's own pre-commit ran under a refused commit: %q", b)
	}

	// The system compare is not a refusal of every HOME move: the same
	// system file spelled ABSOLUTE reads the same under both HOMEs.
	write(t, sys, "[core]\n\thooksPath = "+filepath.Join(fx.home, "managed-hooks")+"\n")
	// hsGlobalCheck numbers its messages from g0 again, and the g0 commit
	// above already landed that content: commit something else first, past
	// the shim and with no hook (the real box's system scope, no hooksPath).
	write(t, filepath.Join(fx.repo, "f"), "between\n")
	hsRealGit(t, fx, "-C", fx.repo, "-c", "core.hooksPath=/dev/null", "commit", "-q", "-m", "between", "--", "f")
	hsGlobalCheck(t, fx, prefix, []hsGlobalLeg{
		{"HOME=evil, byte-identical global file, system hooksPath absolute", "HOME=" + shQuote(evil) + " ", false},
	})
}

// Amendment pin (ranger-base-87zme, the bead's item 2): --type=path makes git
// exit 128, printing nothing, on a `~user` it cannot expand. The launcher's
// global file here carries one in an alias value, which the launcher's own
// git never path-expands and so never trips on. A moved HOME at a config with
// no hook keys then reads have = "" and, if the failed want read were taken
// as "no lines", want = "" too: equal, allowed, and the managed hooksPath
// skipped. A want read that fails must refuse.
func TestQAHookSkipUnexpandableTildeFailsClosed(t *testing.T) {
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
			bad := string(managed) + "[alias]\n\tzz87 = ~nosuchuser-posse-87zme/x\n"
			write(t, dotfile, bad)
			wrapped := hsWrapped(t, fx, rtName)

			empty := filepath.Join(fx.home, "empty-home")
			same := filepath.Join(fx.home, "same-home")
			for _, d := range []string{empty, same} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			write(t, filepath.Join(empty, ".gitconfig"), "[user]\n\tname = t\n\temail = t@example.com\n")
			write(t, filepath.Join(same, ".gitconfig"), bad)

			hsGlobalCheck(t, fx, wrapped, []hsGlobalLeg{
				{"control: nothing moved runs no compare", "", false},
				// want fails, have reads nothing.
				{"HOME at a config with no hook keys, launcher's want read fails", "HOME=" + shQuote(empty) + " ", true},
				// both fail: two empties must not compare equal.
				{"HOME at a byte-identical config, both reads fail", "HOME=" + shQuote(same) + " ", true},
			})
		})
	}
}

// hsGitOut runs the real git with the fixture's env (the launcher's) and
// returns its stdout — the want side, read exactly as the shim reads it.
func hsGitOut(t *testing.T, fx *hsFixture, args ...string) string {
	t.Helper()
	cmd := exec.Command(fx.realGit, args...)
	cmd.Env = hsEnv(fx)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// hsCountTest is D2's count test, as renderHookSkipGlobalScope writes it, and
// the mutant that drops it while leaving the subshell parseable. Pin 9's
// witness swaps the first for the second in a rendered shim.
const (
	hsCountTest   = "[ \"$posse_hs_gl\" -eq \"$posse_hs_gn\" ])"
	hsCountMutant = "[ 0 -eq 0 ])"
)

// Pin 9 (ADR 0079 D2's count, owed by the ranger-base-o6xr6 amendment): a
// config value carrying a NEWLINE forges a `global<TAB>…` line in the
// non -z `--show-scope` listing, so a call that HIDES the launcher's global
// scope can still make the have side read back the want side.
//
// The attack, MEASURED 2026-11-06 against git 2.50.1 (Apple Git-155) — a
// LOCAL alias (matched by hookSkipRegexp) whose value is
// `a\nglobal<TAB>core.hookspath <the launcher's managed path>`, planted past
// the shim the way hsSetup plants its own aliases and the way an operator's
// gitconfig or a cloned repo would carry one, then
// `GIT_CONFIG_GLOBAL=/dev/null git commit`:
//
//	local<TAB>alias.pforge a
//	global<TAB>core.hookspath <managed>
//
// git reads no global file under that variable, so the managed hook is
// unread — yet the shim's pure-sh filter keeps the second line, which is byte
// for byte what the want reads return from the launcher's own files. have ==
// want, the compare calls the call clean, and the commit lands at rc 0 with
// the hook never run. The same forgery works through a moved HOME, which is
// the second leg here.
//
// D2's `--name-only` count is what refuses both: a KEY cannot hold a newline,
// so that listing's line count is an honest entry count, and the forged
// listing carries one line more than there are entries. (-z would also settle
// it and is out: sh command substitution drops NULs.)
//
// THE WITNESS is the same two legs through a shim whose count test is mutated
// to `[ 0 -eq 0 ]`, which must be ALLOWED at rc 0 with the hook unfired.
// Without it this pin cannot tell D2's count from the plain
// `GIT_CONFIG_GLOBAL=/dev/null` refusal that hsRefused already carries, nor
// from a HOME compare that happens to refuse for the ordinary reason.
func TestQAHookSkipNewlineForgedGlobalLineIsRefused(t *testing.T) {
	t.Parallel()
	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			// The want side, read the way the shim reads it, so the forged
			// line is built from git's own output and not from this test's
			// idea of the fixture's hooksPath.
			want := hsGitOut(t, fx, "config", "--includes", "--file", filepath.Join(fx.home, ".gitconfig"), "--get-regexp", hookSkipRegexp)
			if !strings.HasPrefix(want, "core.hookspath ") {
				t.Fatalf("the fixture's global file must carry the managed hooksPath; got %q", want)
			}
			forged := "a"
			for _, l := range strings.Split(strings.TrimRight(want, "\n"), "\n") {
				forged += "\nglobal\t" + l
			}
			hsRealGit(t, fx, "-C", fx.repo, "config", "alias.pforge", forged)

			// A HOME carrying the identity but no hooksPath: the second way
			// to hide the launcher's global scope from the call.
			bare := filepath.Join(fx.home, "bare-home-9")
			if err := os.MkdirAll(bare, 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(bare, ".gitconfig"), "[user]\n\tname = t\n\temail = t@example.com\n")

			legs := []hsGlobalLeg{
				{"pin 9: GIT_CONFIG_GLOBAL=/dev/null, forged global line", "GIT_CONFIG_GLOBAL=/dev/null ", true},
				{"pin 9: HOME lacking the hooksPath, forged global line", "HOME=" + shQuote(bare) + " ", true},
			}

			wrapped, _ := hsWrappedDir(t, fx, rtName, "hg9-"+rtName)
			// The control leads, in the SAME call as the legs: with nothing
			// moved the D3 gate does not fire, so the forged value changes
			// nothing about the ordinary commit and the managed hook runs.
			// One call because hsGlobalCheck numbers its commit messages from
			// zero — split across two calls the control and the first leg
			// write f identically, and the leg then dies at "nothing to
			// commit" (rc 1, on stdout) instead of landing the bypass, which
			// is a mutant that reds this pin for the wrong reason.
			hsGlobalCheck(t, fx, wrapped, append([]hsGlobalLeg{{name: "control: forged value, nothing moved", env: ""}}, legs...))

			// The witness: the same legs, count mutated away.
			mwrapped, mbin := hsWrappedDir(t, fx, rtName, "hg9mut-"+rtName)
			shim := filepath.Join(mbin, "git")
			body, err := os.ReadFile(shim)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), hsCountTest) {
				t.Fatalf("the rendered shim no longer carries D2's count test %q — this witness is stale, not passing", hsCountTest)
			}
			mutant := strings.Replace(string(body), hsCountTest, hsCountMutant, 1)
			if err := WriteExecutable(shim, []byte(mutant), 0o755); err != nil {
				t.Fatal(err)
			}
			for i, leg := range legs {
				before := hsFired(fx)
				errs, code := hsRun(t, fx, mwrapped, hsLeg{env: leg.env, args: "commit -q -m w" + strconv.Itoa(i) + " -- f"})
				if code != 0 || hsFired(fx) != before {
					t.Errorf("WITNESS %s: with D2's count mutated away the forged line must compare EQUAL — the commit lands at rc 0 with the hook unfired; got code=%d fired=%d (%q)", leg.name, code, hsFired(fx)-before, errs)
				}
			}
		})
	}
}
