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

// The GIT_CONFIG_GLOBAL row's value (ranger-base-obdnk, from
// ranger-base-3m6xy).
//
// Three env names steer git's GLOBAL scope — GIT_CONFIG_GLOBAL itself,
// XDG_CONFIG_HOME (git reads $XDG_CONFIG_HOME/git/config) and HOME (git reads
// $HOME/.gitconfig) — and a lower-scope settings `env` naming any of them at
// an attacker's config ran that config's core.hooksPath. MEASURED 2026-11-03
// (git 2.50.1 Apple Git-155): all three FIRED under the pin as it stood, and
// all three went QUIET with GIT_CONFIG_GLOBAL set to an absolute path,
// because that variable replaces git's whole global-file lookup.
//
// ranger-base-37y0z left the row out because every CLOSING constant
// (/dev/null, "") drops ~/.gitconfig — where user.name and user.email live —
// and git then commits under a gecos ident at rc 0 while the identity wall
// (DeriveIdentityLiterals) comes out with no e-mail in it. That ruling
// rested on there being no neutral value a constant could carry. Since
// ranger-base-t301k the launcher resolves a row per launch, so the value can
// be the file git itself would read with the variable unset:
//
//   - exactly one global file exists: that file.
//   - none exists: the LAST candidate, $HOME/.gitconfig, which is where
//     `git config --global` writes when the variable is unset.
//   - BOTH exist: NO row. git reads both, XDG first, and no single value
//     reads both (MEASURED, fixture HOME: pinning either loses the other's
//     keys; an [include] file breaks `git config --global --get-all` and
//     moves --global writes). The inlet stays open, as it was under
//     ranger-base-37y0z, and the launch says so (GitGlobalScopeDegraded).
//
// THE FAIL DIRECTION IS THE OPPOSITE OF THE SYSTEM ROW'S, on purpose. An
// unresolved GIT_CONFIG_SYSTEM falls back to /dev/null: what that loses is
// system scope, said on the launch line. An unresolved GIT_CONFIG_GLOBAL
// falls back to NO ROW, never /dev/null or "": what those lose is the
// operator's identity, so every commit is misattributed and the identity
// wall is emptied, at rc 0, with nothing said by git. Leaving an inlet open
// and naming it is the lesser cost.
//
// Stated, not fixed:
//   - The value is resolved from the LAUNCHER's HOME and XDG_CONFIG_HOME. A
//     launcher whose own env is attacker-steered is out of scope here, as it
//     is for every row.
//   - A second global file created after launch is not read by that session;
//     the next launch degrades.
//   - HOME's reach beyond git (a shell's rc files, other tools' dotfiles) is
//     not covered by this row.
//   - The container tier renders the same pin, and a container seat's HOME is
//     its cage HOME, not the launcher's: there the row names a host path the
//     container does not mount. Neither file exists inside either way (the
//     cage HOME is seeded with no git config), so what git READS is the same;
//     what moves is where a `git config --global` write lands.

// gitGlobalScopeCandidates asks gitBin which files it reads as global scope
// with GIT_CONFIG_GLOBAL unset, in the order git reads them.
//
// `git var GIT_CONFIG_GLOBAL` (git 2.42+) names every candidate whether or
// not it exists — MEASURED: always two lines, XDG first, when HOME is set.
// An older git answers it with an error, and so does any git with HOME unset;
// either way the answer falls back to git's documented rule over the same
// environment, which is the whole of what git does.
func gitGlobalScopeCandidates(gitBin string, env []string) ([]string, error) {
	if gitBin != "" {
		ctx, cancel := context.WithTimeout(context.Background(), GitTimeout)
		defer cancel()
		cmd := protocolCmd(exec.CommandContext(ctx, gitBin, "var", "GIT_CONFIG_GLOBAL"))
		bindGitDeadline(ctx, cmd)
		cmd.Env = protocolEnv(env)
		var out bytes.Buffer
		cmd.Stdout = &out
		if cmd.Run() == nil {
			var paths []string
			for _, l := range strings.Split(out.String(), "\n") {
				if l = strings.TrimRight(l, "\r"); l != "" {
					paths = append(paths, l)
				}
			}
			if len(paths) > 0 {
				return paths, nil
			}
		}
	}
	return gitGlobalScopeDocumented(env)
}

// gitGlobalScopeDocumented is git's documented global-file rule (git-config
// FILES): ${XDG_CONFIG_HOME:-$HOME/.config}/git/config, then $HOME/.gitconfig.
// An empty XDG_CONFIG_HOME counts as unset, as it does in git. With no HOME
// and no XDG_CONFIG_HOME git reads no global file at all, and there is
// nothing to pin.
func gitGlobalScopeDocumented(env []string) ([]string, error) {
	get := func(k string) (string, bool) {
		for i := len(env) - 1; i >= 0; i-- {
			if v, ok := strings.CutPrefix(env[i], k+"="); ok {
				return v, true
			}
		}
		return "", false
	}
	home, hasHome := get("HOME")
	var out []string
	if xdg, _ := get("XDG_CONFIG_HOME"); xdg != "" {
		out = append(out, xdg+"/git/config")
	} else if hasHome {
		out = append(out, home+"/.config/git/config")
	}
	if hasHome {
		out = append(out, home+"/.gitconfig")
	}
	if len(out) == 0 {
		return nil, errors.New("neither HOME nor XDG_CONFIG_HOME is set, so git reads no global file")
	}
	return out, nil
}

// errGitGlobalScopeTwoFiles is the one resolution that has no neutral value.
var errGitGlobalScopeTwoFiles = errors.New("git global scope has two files")

// chooseGitGlobalScope is the row's decision over the candidates, with
// existence asked fresh: one existing file is that file, none is the last
// candidate (git's write target), and more than one is no value at all.
func chooseGitGlobalScope(cands []string, exists func(string) bool) (string, error) {
	if len(cands) == 0 {
		return "", errors.New("git named no global-scope file")
	}
	var have []string
	for _, c := range cands {
		if exists(c) {
			have = append(have, c)
		}
	}
	switch len(have) {
	case 0:
		return cands[len(cands)-1], nil
	case 1:
		return have[0], nil
	}
	return "", fmt.Errorf("%w: %s", errGitGlobalScopeTwoFiles, strings.Join(have, " and "))
}

// gitGlobalScopeCache keys the candidate list on the git binary and the two
// env values git derives it from. Existence is NOT cached — it is two stats,
// and a file that appears between launches must move the next answer.
var gitGlobalScopeCache struct {
	sync.Mutex
	m map[string]gitGlobalScopeEntry
}

type gitGlobalScopeEntry struct {
	size  int64
	mtime time.Time
	cands []string
}

// gitGlobalScope is the GIT_CONFIG_GLOBAL row's value for this launch, or
// the error that says why there is no row.
func gitGlobalScope() (string, error) {
	env := gitSystemScopeEnv() // every GIT_CONFIG* scrubbed, gates-free PATH
	bin := resolveOutside("git", "")
	key := bin
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME"} {
		v, set := os.LookupEnv(k) // unset and empty resolve differently in git
		key += fmt.Sprintf("\x00%t:%s", set, v)
	}
	var cands []string
	fi, statErr := os.Stat(bin)
	if bin != "" && statErr == nil {
		gitGlobalScopeCache.Lock()
		e, ok := gitGlobalScopeCache.m[key]
		gitGlobalScopeCache.Unlock()
		if ok && e.size == fi.Size() && e.mtime.Equal(fi.ModTime()) {
			cands = e.cands
		}
	}
	if cands == nil {
		c, err := gitGlobalScopeCandidates(bin, env)
		if err != nil {
			return "", err
		}
		cands = c
		if bin != "" && statErr == nil {
			gitGlobalScopeCache.Lock()
			if gitGlobalScopeCache.m == nil {
				gitGlobalScopeCache.m = map[string]gitGlobalScopeEntry{}
			}
			gitGlobalScopeCache.m[key] = gitGlobalScopeEntry{fi.Size(), fi.ModTime(), cands}
			gitGlobalScopeCache.Unlock()
		}
	}
	return chooseGitGlobalScope(cands, func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	})
}

// GitGlobalScopeDegraded is the launch line for a launch whose pin carries
// no GIT_CONFIG_GLOBAL row, "" when it carries one. The launch proceeds: the
// session reads exactly the global scope it would outside posse, and the
// line says which inlets that leaves open.
func GitGlobalScopeDegraded() string {
	_, err := gitGlobalScope()
	if err == nil {
		return ""
	}
	if errors.Is(err, errGitGlobalScopeTwoFiles) {
		files := strings.TrimPrefix(err.Error(), errGitGlobalScopeTwoFiles.Error()+": ")
		return fmt.Sprintf("DEGRADED: %v; GIT_CONFIG_GLOBAL inlet NOT pinned (%s). No single value reads both, and /dev/null would misattribute every commit, so GIT_CONFIG_GLOBAL, XDG_CONFIG_HOME and HOME stay open to git in this session; merge the two files to close them (ranger-base-obdnk)", errGitGlobalScopeTwoFiles, files)
	}
	return fmt.Sprintf("DEGRADED: GIT_CONFIG_GLOBAL inlet NOT pinned because %v. GIT_CONFIG_GLOBAL, XDG_CONFIG_HOME and HOME stay open to git in this session (ranger-base-obdnk)", err)
}
