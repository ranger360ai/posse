//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-r3pri — the git shim's hook-skip arm.
//
// Claim: from a seat, on any runtime and under any PID, no ordinary-path
// spelling makes git skip (or re-point) a hook, and the plain commit runs the
// hook. See hookskip.go for the arm and its stated residuals.
//
// These run REAL git in a scratch repo, through the launch line a seat is
// typed under (WrapWithGates), for two runtimes. The "managed" hook lives at
// a core.hooksPath set in the scratch HOME's global config — the ADR 0052
// managed-box shape — and its pre-commit appends to a marker file, so "the
// hook fired" is a line in that file and not an exit code. The scratch repo
// is not the tree, so this is not a tree-wide pin and owes the Makefile no
// door.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hsLeg is one command line typed after the gate prefix, the env it carries
// (as a typed `VAR=… ` prefix, which is how a seat would spell it), and what
// must happen: rule "" means not refused by this arm.
type hsLeg struct {
	env  string
	args string
	rule string
}

// hsRefused enumerates every spelling the bead names, plus the ones the
// implementation found on the way (am -n, include.*, scope overrides by
// content). Each must exit nonzero naming its rule.
func hsRefused(fx *hsFixture) []hsLeg {
	return []hsLeg{
		// hookskip:no-verify
		{"", "commit -q -n -m d -- f", "hookskip:no-verify"},
		{"", "commit -q --no-verify -m d -- f", "hookskip:no-verify"},
		{"", "commit -q --no-verif -m d -- f", "hookskip:no-verify"},
		{"", "commit -q --no-veri -m d -- f", "hookskip:no-verify"},
		{"", "commit -q --no-v -m d -- f", "hookskip:no-verify"},
		{"", "commit -q -nm d3 -- f", "hookskip:no-verify"},
		{"", "commit -qn -m d3 -- f", "hookskip:no-verify"},
		{"", "-C . commit -q -n -m d -- f", "hookskip:no-verify"},
		{"", "am -n x.mbox", "hookskip:no-verify"},
		{"", "am --no-verify x.mbox", "hookskip:no-verify"},
		{"", "merge --no-verify HEAD", "hookskip:no-verify"},
		{"", "pull --no-verify", "hookskip:no-verify"},
		{"", "rebase --no-verify HEAD", "hookskip:no-verify"},
		{"", "push --no-verify origin main", "hookskip:no-verify"},
		{"", "push --no-verif origin main", "hookskip:no-verify"},
		// hookskip:config-override, any verb
		{"", "-c core.hooksPath=/dev/null commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c CORE.HOOKSPATH=/dev/null commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c core.hooksPath= commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c core.hooksPath status", "hookskip:config-override"},
		{"", "-c hook.scan.command=true commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c include.path=/tmp/x commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c includeIf.gitdir:/.path=/tmp/x commit -q -m e -- f", "hookskip:config-override"},
		{"", "--config-env=core.hooksPath=HOME commit -q -m e -- f", "hookskip:config-override"},
		{"", "--config-env core.hooksPath=HOME commit -q -m e -- f", "hookskip:config-override"},
		{"", "-C . -c user.name=x -c core.hooksPath=/dev/null log -1", "hookskip:config-override"},
		// hookskip:config-env
		{`GIT_CONFIG_PARAMETERS="'core.hookspath'='/dev/null'" `, "commit -q -m f -- f", "hookskip:config-env"},
		{`GIT_CONFIG_PARAMETERS="'hook.scan.command'='true'" `, "commit -q -m f -- f", "hookskip:config-env"},
		{"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null ", "commit -q -m f -- f", "hookskip:config-env"},
		{"GIT_CONFIG_COUNT=2 GIT_CONFIG_KEY_0=user.name GIT_CONFIG_VALUE_0=x GIT_CONFIG_KEY_1=Core.HooksPath GIT_CONFIG_VALUE_1=" + shQuote(fx.otherHooks) + " ", "commit -q -m f -- f", "hookskip:config-env"},
		// A redirect-SHAPED value one level too deep is not the redirect.
		{"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=" + shQuote(filepath.Join(fx.hooksRoot, "s", "x")) + " ", "commit -q -m f -- f", "hookskip:config-env"},
		{"GIT_CONFIG_GLOBAL=/dev/null ", "commit -q -m f -- f", "hookskip:config-env"},
		{"GIT_CONFIG_SYSTEM=" + shQuote(fx.plantedSystem) + " ", "commit -q -m f -- f", "hookskip:config-env"},
		// hookskip:config-write
		{"", "config core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config --local core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config --global core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config --file .git/config core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config --fil .git/config core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config set core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config set --local core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config unset core.hooksPath", "hookskip:config-write"},
		{"", "config --unset core.hooksPath", "hookskip:config-write"},
		{"", "config --unse core.hooksPath", "hookskip:config-write"},
		{"", "config --unset-all core.hooksPath", "hookskip:config-write"},
		{"", "config --add hook.scan.command true", "hookskip:config-write"},
		{"", "config --replace-all core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config --remove-section core", "hookskip:config-write"},
		{"", "config remove-section hook.scan", "hookskip:config-write"},
		{"", "config --rename-section core x", "hookskip:config-write"},
		{"", "config include.path /tmp/x", "hookskip:config-write"},
		{"", "config --edit", "hookskip:config-write"},
		{"", "config -e", "hookskip:config-write"},
		{"", "config edit", "hookskip:config-write"},
		// ranger-base-z8kym: aliases. The bead's legs first — each exited 0
		// with the hook unrun before this arm resolved aliases.
		{"", `-c alias.ci="commit -n" ci -q -m a1 -- f`, "hookskip:config-override"},
		{"", `-c alias.ci="!git commit -n" ci -q -m a2 -- f`, "hookskip:config-override"},
		{"", "--config-env=alias.ci=HOME ci -q -m a2 -- f", "hookskip:config-override"},
		{`GIT_CONFIG_PARAMETERS="'alias.ci'='commit -n'" `, "ci -q -m a3 -- f", "hookskip:config-env"},
		{"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=alias.ci GIT_CONFIG_VALUE_0='commit --no-verify' ", "ci -q -m a4 -- f", "hookskip:config-env"},
		{"", `config alias.ci "commit -n"`, "hookskip:config-write"},
		{"", `config set alias.ci "commit -n"`, "hookskip:config-write"},
		{"", "config --remove-section alias", "hookskip:config-write"},
		// An alias already in config (planted by hsSetup, past the shim, the
		// way an operator's own gitconfig would carry one) is resolved and
		// its expansion judged.
		{"", "pci -q -m p1 -- f", "hookskip:no-verify"},
		{"", "PCI -q -m p1 -- f", "hookskip:no-verify"},
		{"", "-C . pci -q -m p1 -- f", "hookskip:no-verify"},
		{"", "pchain -q -m p2 -- f", "hookskip:no-verify"},
		{"", "pnv -q -m p3 -- f", "hookskip:no-verify"},
		{"", "phooks -q -m p4 -- f", "hookskip:config-override"},
		{"", "psh", "hookskip:alias"},
		{"", "pabs", "hookskip:alias"},
	}
}

// hsAllowed are the controls: none may be refused by this arm, and the
// commits among them must run the hook.
func hsAllowed(fx *hsFixture) []hsLeg {
	return []hsLeg{
		{"", "commit -q -m n -- f", ""},
		{"", "commit -q -mn -- f", ""},
		{"", "commit -q -m --no-verify -- f", ""},
		{"", "-c user.name=someone commit -q -m c -- f", ""},
		{"GIT_CONFIG_GLOBAL=" + shQuote(fx.globalCopy) + " ", "commit -q -m g -- f", ""},
		{"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=user.name GIT_CONFIG_VALUE_0=x ", "commit -q -m h -- f", ""},
		{`GIT_CONFIG_PARAMETERS="'user.name'='x'" `, "commit -q -m i -- f", ""},
		{"", "push -n origin main", ""},
		{"", "push --dry-run origin main", ""},
		{"", "config --get core.hooksPath", ""},
		{"", "config core.hooksPath", ""},
		{"", "config get core.hooksPath", ""},
		{"", "config --list", ""},
		{"", "config user.email a@b.c", ""},
		{"GIT_CONFIG_GLOBAL=/dev/null ", "status", ""},
		// An alias for a plain commit runs, and runs the hook.
		{"", "pcm -q -m a -- f", ""},
		{"", "config --get alias.pci", ""},
		{"", "config get alias.pci", ""},
		{"", "--version", ""},
		// An alias loop is git's to refuse ("alias loop detected"), and it
		// runs nothing.
		{"", "ploop", ""},
	}
}

type hsFixture struct {
	home, repo, marker, hooksRoot, otherHooks, plantedSystem, globalCopy string
	realGit                                                              string
}

func hsSetup(t *testing.T) *hsFixture {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	realGit := resolveOutside("git", "")
	if realGit == "" {
		t.Skip("no git")
	}
	fx := &hsFixture{home: gitTempDir(t), realGit: realGit}
	fx.repo = filepath.Join(fx.home, "repo")
	fx.marker = filepath.Join(fx.home, "fired")
	managed := filepath.Join(fx.home, "managed-hooks")
	fx.otherHooks = filepath.Join(fx.home, "other-hooks")
	for _, d := range []string{fx.repo, managed, fx.otherHooks} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	hook := "#!/bin/sh\necho fired >> " + shQuote(fx.marker) + "\n"
	if err := WriteExecutable(filepath.Join(managed, "pre-commit"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	// The managed hooks path, in global scope — ADR 0052's managed box.
	global := "[core]\n\thooksPath = " + managed + "\n[user]\n\tname = t\n\temail = t@example.com\n"
	write(t, filepath.Join(fx.home, ".gitconfig"), global)
	fx.globalCopy = filepath.Join(fx.home, "global-copy")
	write(t, fx.globalCopy, global)
	fx.plantedSystem = filepath.Join(fx.home, "planted-system")
	write(t, fx.plantedSystem, "[core]\n\thooksPath = "+fx.otherHooks+"\n")

	remote := filepath.Join(fx.home, "remote.git")
	for _, c := range [][]string{
		{"init", "-q", "--bare", remote},
		{"-C", fx.repo, "init", "-q", "-b", "main"},
		{"-C", fx.repo, "remote", "add", "origin", remote},
	} {
		hsRealGit(t, fx, c...)
	}
	write(t, filepath.Join(fx.repo, "f"), "0\n")
	hsRealGit(t, fx, "-C", fx.repo, "add", "--", "f")
	hsRealGit(t, fx, "-C", fx.repo, "commit", "-q", "-m", "seed", "--", "f")
	hsRealGit(t, fx, "-C", fx.repo, "push", "-q", "origin", "main")
	// Persistent aliases, planted past the shim (ranger-base-z8kym).
	for k, v := range map[string]string{
		"pci":    "commit -n",
		"pchain": "pci",
		"pnv":    "commit --no-verify",
		"phooks": "-c core.hooksPath=/dev/null commit",
		"psh":    "!git commit -n -q -m x -- f",
		"pabs":   "!" + realGit + " commit -n -q -m x -- f",
		"ploop":  "ploop2",
		"ploop2": "ploop",
		"pcm":    "commit",
	} {
		hsRealGit(t, fx, "-C", fx.repo, "config", "alias."+k, v)
	}
	os.Remove(fx.marker)
	return fx
}

// hsEnv is the env every leg starts from: this process's, with every
// GIT_CONFIG_* the seat running this suite carries shed, and HOME the scratch
// one — so the default global scope is the managed one above.
func hsEnv(fx *hsFixture) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GIT_CONFIG") || strings.HasPrefix(kv, "HOME=") || strings.HasPrefix(kv, "XDG_CONFIG_HOME=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "HOME="+fx.home)
}

func hsRealGit(t *testing.T, fx *hsFixture, args ...string) {
	t.Helper()
	cmd := exec.Command(fx.realGit, args...)
	cmd.Env = hsEnv(fx)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func hsFired(fx *hsFixture) int {
	b, _ := os.ReadFile(fx.marker)
	return strings.Count(string(b), "fired")
}

// hsRun types `<prefix><leg.env>git <leg.args>` into sh in the repo, which is
// how a seat's own shell would run it, and returns stderr and the exit code.
func hsRun(t *testing.T, fx *hsFixture, prefix string, leg hsLeg) (string, int) {
	t.Helper()
	// Every leg edits f first, so a commit has something to commit.
	write(t, filepath.Join(fx.repo, "f"), leg.args+"\n")
	cmd := exec.Command("sh", "-c", prefix+leg.env+"git "+leg.args)
	cmd.Dir = fx.repo
	cmd.Env = hsEnv(fx)
	var errb strings.Builder
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return errb.String(), code
}

// The bead's execution pin: through a rendered shim, on claude AND codex,
// each refused form exits nonzero with a named reason and a refusals.log
// line, the hook does not fire, and the plain commit fires it.
func TestQAHookSkipRefusedOnEveryRuntime(t *testing.T) {
	t.Parallel()
	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			home := t.TempDir()
			a := &App{Home: home, StateDir: filepath.Join(home, "state")}
			rt, err := a.LoadRuntime(rtName)
			if err != nil {
				t.Fatal(err)
			}
			fx.hooksRoot = filepath.Join(a.StateDir, "hooks")
			persona := "hs-" + rtName
			// The deny list every crew PID carries.
			wrapped, gatesDir, _, err := a.WrapWithGates(persona, rt, []string{"Bash(git push:*)", "Bash(git commit unless --)"}, "")
			if err != nil {
				t.Fatal(err)
			}
			if rt.Name != rtName {
				t.Fatalf("loaded runtime %q, want %q", rt.Name, rtName)
			}

			// The plain commit runs the hook — the control the whole arm is for.
			if errs, code := hsRun(t, fx, wrapped, hsLeg{args: "commit -q -m plain -- f"}); code != 0 || hsFired(fx) != 1 {
				t.Fatalf("plain commit: code=%d fired=%d, want 0 and 1 (%s)", code, hsFired(fx), errs)
			}

			refused := hsRefused(fx)
			for _, leg := range refused {
				before := hsFired(fx)
				errs, code := hsRun(t, fx, wrapped, leg)
				line := leg.env + "git " + leg.args
				if code == 0 {
					t.Errorf("%s: exited 0, want refused by %s", line, leg.rule)
					continue
				}
				if !strings.Contains(errs, "rule: "+leg.rule+",") || !strings.Contains(errs, "refused by posse gate") {
					t.Errorf("%s: refusal must name %s: %q", line, leg.rule, errs)
				}
				if hsFired(fx) != before {
					t.Errorf("%s: the hook ran — the refusal came after git, not before", line)
				}
			}
			logb, err := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(string(logb), "(hookskip: "); n != len(refused) {
				t.Errorf("refusals.log must carry one hookskip line per refusal: got %d, want %d\n%s", n, len(refused), logb)
			}
			if !strings.Contains(string(logb), "git commit -q -n -m d -- f (hookskip: hookskip:no-verify)") {
				t.Errorf("a log line must carry the argv and the rule:\n%s", logb)
			}

			for _, leg := range hsAllowed(fx) {
				before := hsFired(fx)
				errs, code := hsRun(t, fx, wrapped, leg)
				line := leg.env + "git " + leg.args
				if strings.Contains(errs, "hookskip:") {
					t.Errorf("%s: refused by the hook-skip arm, want allowed: %q", line, errs)
					continue
				}
				if strings.HasPrefix(leg.args, "commit") || strings.Contains(leg.args, " commit ") || strings.HasPrefix(leg.args, "pcm ") {
					if code != 0 || hsFired(fx) != before+1 {
						t.Errorf("%s: code=%d, hook fired %d times, want 0 and once (%s)", line, code, hsFired(fx)-before, errs)
					}
				}
			}
		})
	}
}

// Independent of the PID: a persona that denies NOTHING in git still gets a
// git shim, and the arm in it. push -n is allowed there (it is --dry-run)
// and push --no-verify is refused by the arm and not by any deny.
func TestQAHookSkipNeedsNoPIDRule(t *testing.T) {
	t.Parallel()
	fx := hsSetup(t)
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	fx.hooksRoot = filepath.Join(a.StateDir, "hooks")
	_, binDir, _, err := a.RenderGates("hs-nodeny", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(binDir, "git")); err != nil {
		t.Fatalf("a PID with no git deny must still get a git shim: %v", err)
	}
	prefix := GatePrefix(binDir, "")
	if errs, code := hsRun(t, fx, prefix, hsLeg{args: "push -n origin main"}); code != 0 {
		t.Errorf("push -n is --dry-run and must run: code=%d %q", code, errs)
	}
	if errs, code := hsRun(t, fx, prefix, hsLeg{args: "push --no-verify origin main"}); code == 0 || !strings.Contains(errs, "rule: hookskip:no-verify,") {
		t.Errorf("push --no-verify must be refused by the arm with no PID rule: code=%d %q", code, errs)
	}
	// The env arm on push, where no deny refuses push first.
	if errs, code := hsRun(t, fx, prefix, hsLeg{env: "GIT_CONFIG_GLOBAL=/dev/null ", args: "push origin main"}); code == 0 || !strings.Contains(errs, "rule: hookskip:config-env,") {
		t.Errorf("GIT_CONFIG_GLOBAL hiding the managed hooksPath must refuse a push: code=%d %q", code, errs)
	}
	// And posse's own redirect is the one core.hooksPath the env may carry.
	redirect := filepath.Join(fx.hooksRoot, "persona-posse-x")
	if err := os.MkdirAll(redirect, 0o755); err != nil {
		t.Fatal(err)
	}
	leg := hsLeg{env: "GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=" + shQuote(redirect) + " ", args: "commit -q -m r -- f"}
	if errs, code := hsRun(t, fx, prefix, leg); code != 0 || strings.Contains(errs, "hookskip:") {
		t.Errorf("the ADR 0052 D2 redirect must pass: code=%d %q", code, errs)
	}
}

// With no git resolved at render, an override the arm cannot judge is
// refused, not waved through: the scope comparison fails closed.
func TestQAHookSkipScopeFailsClosedWithoutGit(t *testing.T) {
	t.Parallel()
	body := renderHookSkipArm("", "/bin/date", "/x/hooks")
	if !strings.Contains(body, "posse_hs_scope() {\n  posse_hs_have=$(false config") || !strings.Contains(body, "  return 1\n}\nposse_hs_env()") {
		t.Errorf("with no real git the scope comparison must return 1 (refuse):\n%s", body)
	}
}
