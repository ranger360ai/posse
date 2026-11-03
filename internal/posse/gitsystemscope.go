package posse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// The GIT_CONFIG_SYSTEM row's value (ranger-base-t301k, from
// ranger-base-zwkgb / gh ranger360ai/posse#15).
//
// The row closes an inlet: a lower-scope settings `env` pointing
// GIT_CONFIG_SYSTEM at an attacker's config naming core.hooksPath ran the
// attacker's hook. Until t301k the row closed it with /dev/null, which also
// emptied system scope — and system scope is where an employer installs a
// managed commit scanner. MEASURED 2026-11-03 (git 2.50.1 Apple Git-155,
// scratch repo, a stand-in system config naming core.hooksPath with a marker
// pre-commit): the hook fired with the variable at that file, and did not
// fire under /dev/null, rc 0, no word on either stream. ADR 0052's rule is
// that posse never causes the managed hook to be skipped.
//
// The neutral value is the file git reads when the variable is UNSET: any
// constant shuts the inlet, so the constant may as well be the one git
// already chose. That file is the git BINARY's build-time sysconfdir —
// /etc/gitconfig for /usr/bin/git and a distro git,
// /opt/homebrew/etc/gitconfig for a Homebrew git on apple silicon — so it
// is resolved per launch, in the launcher, against the git a seat runs
// (resolveOutside over the gates-free PATH: the binary every git shim execs
// and the first git a shimless seat finds).

// gitSystemScopeFallback is what the row carries when the path cannot be
// resolved: the inlet stays shut, and system scope is NOT carried. It is
// never taken silently — GitSystemScopeDegraded names it at launch.
const gitSystemScopeFallback = os.DevNull

// gitSystemScopeEnv is the environment the question is asked in: this
// process's, with every GIT_CONFIG* name removed. A launcher that itself
// runs under the pin (a dispatcher inside a claude pane) carries
// GIT_CONFIG_SYSTEM already, and git would answer with that value — /dev/null
// pinned onward as if git had chosen it. GIT_CONFIG_NOSYSTEM goes for the
// same reason: under it `git var` answers nothing at all. PATH is the
// gates-free one, so git's own children cannot reach a shim either.
func gitSystemScopeEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GIT_CONFIG") {
			continue
		}
		env = append(env, e)
	}
	return protocolEnv(env)
}

// resolveGitSystemScope asks gitBin which file it reads as system scope with
// GIT_CONFIG_SYSTEM unset.
//
// `git var GIT_CONFIG_SYSTEM` (git 2.42+) is asked first, because it names
// the path whether the file exists, is missing, or is empty. Older gits
// answer it with an error, and get the second question: `git config
// --system --list --show-origin`, whose output names the path in a `file:`
// origin when the file holds something and in the fatal when it is absent.
// An existing EMPTY file prints neither on that older route, and that is a
// resolution failure rather than a guess.
func resolveGitSystemScope(gitBin string) (string, error) {
	if gitBin == "" {
		return "", errors.New("no git on the gates-free PATH")
	}
	run := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), GitTimeout)
		defer cancel()
		cmd := protocolCmd(exec.CommandContext(ctx, gitBin, args...))
		bindGitDeadline(ctx, cmd)
		cmd.Env = gitSystemScopeEnv()
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		return out.String(), err
	}
	if out, err := run("var", "GIT_CONFIG_SYSTEM"); err == nil {
		if p := strings.TrimSpace(out); p != "" && !strings.ContainsAny(p, "\n\r") {
			return p, nil
		}
	}
	out, _ := run("config", "--system", "--list", "--show-origin") // rc 128 when absent; that output is the answer
	if p, ok := parseGitSystemScopePath(out); ok {
		return p, nil
	}
	return "", fmt.Errorf("%s named no system-scope path (git var GIT_CONFIG_SYSTEM failed, and `git config --system --list --show-origin` printed %q)", gitBin, strings.TrimSpace(out))
}

// parseGitSystemScopePath reads the path out of whichever of the two shapes
// `git config --system --list --show-origin` produced: a `file:` origin when
// the file exists and holds something, the fatal when it does not. An
// existing but EMPTY system config prints nothing at rc 0, and reports
// not-ok. (Promoted from inletpingitconfig_qa_test.go, where it was the
// probe's reader; that file still holds it against the three shapes.)
func parseGitSystemScopePath(out string) (string, bool) {
	if _, rest, ok := strings.Cut(out, "unable to read config file '"); ok {
		if path, _, ok := strings.Cut(rest, "'"); ok {
			return path, true
		}
	}
	if _, rest, ok := strings.Cut(out, "file:"); ok {
		path, _, _ := strings.Cut(rest, "\t")
		return strings.TrimRight(path, "\r\n"), true
	}
	return "", false
}

// gitSystemScopeCache keys a resolved path on the git binary and its stat.
// The answer is compiled into that binary, so it moves only when the binary
// does; a render path that runs per persona (reachability, the cockpit)
// would otherwise exec git once per line. Failures are not cached.
var gitSystemScopeCache struct {
	sync.Mutex
	m map[string]gitSystemScopeEntry
}

type gitSystemScopeEntry struct {
	size  int64
	mtime time.Time
	path  string
}

// gitSystemScope is the resolved path for the git a seat runs, or the error
// that says why there is none.
func gitSystemScope() (string, error) {
	bin := resolveOutside("git", "")
	if bin == "" {
		return "", errors.New("no git on the gates-free PATH")
	}
	fi, statErr := os.Stat(bin)
	if statErr == nil {
		gitSystemScopeCache.Lock()
		e, ok := gitSystemScopeCache.m[bin]
		gitSystemScopeCache.Unlock()
		if ok && e.size == fi.Size() && e.mtime.Equal(fi.ModTime()) {
			return e.path, nil
		}
	}
	p, err := resolveGitSystemScope(bin)
	if err != nil {
		return "", err
	}
	if statErr == nil {
		gitSystemScopeCache.Lock()
		if gitSystemScopeCache.m == nil {
			gitSystemScopeCache.m = map[string]gitSystemScopeEntry{}
		}
		gitSystemScopeCache.m[bin] = gitSystemScopeEntry{fi.Size(), fi.ModTime(), p}
		gitSystemScopeCache.Unlock()
	}
	return p, nil
}

// gitSystemScopePinValue is the GIT_CONFIG_SYSTEM row's value: the resolved
// path, else the named fallback.
func gitSystemScopePinValue() string {
	if p, err := gitSystemScope(); err == nil {
		return p
	}
	return gitSystemScopeFallback
}

// GitSystemScopeDegraded is the launch line for a failed resolution, "" when
// the row carries git's own path. A launch still proceeds — the inlet is
// shut either way — but the line says what the seat is missing, because the
// failure it describes is the one ranger-base-t301k exists to end: a
// managed hook in system scope skipped with no word said.
func GitSystemScopeDegraded() string {
	_, err := gitSystemScope()
	if err == nil {
		return ""
	}
	return fmt.Sprintf("DEGRADED: git system scope is NOT carried into this session — GIT_CONFIG_SYSTEM is pinned to %s because %v. A commit scanner or hook configured only in system scope will not run here (ranger-base-t301k)", gitSystemScopeFallback, err)
}
