package rhq

// Directory trust on claude is a LAUNCH fact, exactly the way
// `-c "projects={\"$PWD\"={trust_level=\"trusted\"}}"` is one on codex
// (ADR 0002 §4, amendment 2026-08-18) — rangerhq-w4uf, discovered while
// verifying the permission-mode landing of rangerhq-qs5r.
//
// MEASURED on claude 2.1.241, four herdr scratch panes, no API turn: a
// claude started in a directory it has never run in draws a full-screen
// modal before any prompt can land — "Quick safety check: Is this a
// project you created or one you trust?", footed `Enter to confirm · Esc
// to cancel`. Two facts about that screen, both measured rather than
// assumed:
//
//   - herdr reports it `blocked` (manifest rule live_blocked_form), NOT
//     idle. So dispatch does not type the work prompt into the menu the
//     way stock detection let it type into codex's "Hooks need review"
//     (rangerhq-7ia). It waits out its patience instead and the session
//     never becomes promptable — a launch that is dead either way, and
//     dead in a directory nobody is watching.
//   - The dirs that hit it are exactly the ones the fleet keeps growing:
//     a new repo, a scratch dir, a container's fresh HOME. The fleet's
//     long-lived dirs never show it because the operator answered the
//     dialog there once, by hand, months ago.
//
// THERE IS NO FLAG AND NO SETTINGS KEY, which was the first thing the
// bead asked to check. `claude --help` on 2.1.241 names none; `claude
// project` manages only `purge`; `--settings` takes no trust key; the one
// env var that skips the check outright (CLAUDE_CODE_SANDBOXED) is a
// claim about the process's confinement that a shims-tier launch has no
// business making. What the CLI does have is a documented non-interactive
// path, which it prints in its own words when it drops a project's hooks
// for want of trust: "Run Claude Code interactively here once and accept
// the trust dialog, or set projects[<dir>].hasTrustDialogAccepted: true
// in <config>". That is the key this file writes, and the reason it is a
// file and not a typed line is that claude offers no line to type it on.
//
// WHAT THE GRANT BUYS THE REPO, and why that is a divergence somebody has
// to rule on: while a workspace is untrusted claude DROPS the project's
// `hooks` and `mcpServers` entries out of <dir>/.claude/settings.json
// ("Dropped N project-scoped hooks entries — workspace not yet trusted",
// its own log line). Trusted, they load. That is the same class of
// channel ADR 0002's amendment made codex's launch check for, and the
// amendment's §1 rests on a premise this file makes false for claude —
// "empty everywhere else, including claude (posse types it no trust
// flag)". Wiring Runtime.ProjectConfig to `.claude/settings.json` is the
// follow-on that amendment parked, and it is now live work rather than a
// hypothetical: it belongs to the architect (rangerhq-w4uf handed it off)
// because it would refuse a launch in every repo that carries one, this
// one included.

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ClaudeConfigFile is the file claude keeps its `projects` map in,
// resolved the way claude 2.1.241 resolves it: <config dir>/.config.json
// when that file exists, else <CLAUDE_CONFIG_DIR or $HOME>/.claude.json,
// where the config dir is CLAUDE_CONFIG_DIR or ~/.claude.
//
// $HOME and not os.UserHomeDir(), for interstitial.go's reason: the path
// printed has to be the path read. Claude also suffixes the basename
// (`.claude-staging-oauth.json`) for a non-production deployment — that is
// not a fleet shape, and a session that sets CLAUDE_CODE_CUSTOM_OAUTH_URL
// gets the dialog rather than a guess.
func ClaudeConfigFile() string {
	cfgDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if cfgDir == "" {
		cfgDir = filepath.Join(os.Getenv("HOME"), ".claude")
	}
	if p := filepath.Join(cfgDir, ".config.json"); fileExists(p) {
		return p
	}
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		base = os.Getenv("HOME")
	}
	return filepath.Join(base, ".claude.json")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ClaudeTrustKey is the projects[] key for a session dir: the absolute
// path with symlinks resolved. Resolved because the cwd claude reads is
// the one the kernel hands it — on macOS a session dir under /tmp is
// /private/tmp to claude — and a key written under a spelling claude never
// looks up is a key that grants nothing.
func ClaudeTrustKey(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	return filepath.Clean(abs)
}

// ClaudeTrusted answers claude's own question — would this launch draw the
// dialog? — against an already-parsed config, so the launch can leave the
// operator's file alone when there is nothing to add to it.
//
// It mirrors claude 2.1.241's fallback walk: start at the session dir,
// take any hasTrustDialogAccepted == true on the way up, and STOP at the
// enclosing git repo root, because a trusted parent does not cover a repo
// underneath it (the same rule codex's per-exact-path trust has, and the
// reason ~/src being trusted does nothing for a repo inside it). Outside a
// repo the walk runs to the filesystem root.
//
// One hop is deliberately not followed: claude also keys the config on the
// CANONICAL root, so a linked worktree resolves to its main repo and
// inherits that repo's answer. Following a `.git` file through its
// commondir to decide whether to skip a write is more machinery than the
// write is worth, so a fresh worktree of a trusted repo gets an entry it
// did not strictly need. The error only ever runs that way: nothing this
// reports trusted would have drawn the dialog.
func ClaudeTrusted(state map[string]any, dir string) bool {
	projects, _ := state["projects"].(map[string]any)
	if projects == nil {
		return false
	}
	accepted := func(p string) bool {
		e, _ := projects[p].(map[string]any)
		v, _ := e["hasTrustDialogAccepted"].(bool)
		return v
	}
	n := ClaudeTrustKey(dir)
	root := gitRootOf(n)
	for {
		if accepted(n) {
			return true
		}
		if n == root {
			return false
		}
		parent := filepath.Dir(n)
		if parent == n {
			return false
		}
		n = parent
	}
}

// gitRootOf is the boundary of the walk above and nothing more: the
// nearest ancestor holding a `.git` (a directory in a checkout, a file in
// a linked worktree), "" outside a repo. It never shells out — this runs
// on every launch, and `git rev-parse` in a directory git dislikes is a
// process spawned to learn nothing.
func gitRootOf(dir string) string {
	for d := dir; ; {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		p := filepath.Dir(d)
		if p == d {
			return ""
		}
		d = p
	}
}

// claudeSeedProject sets, under projects[dir], the keys that decide what a
// fresh session dir draws. Shared with the cage's HOME seed (cage.go) so
// there is one statement of what posse writes into a claude config.
//
// Both keys are MEASURED, and they are not the same fact:
//   - hasTrustDialogAccepted is the one that matters — without it the
//     modal, with it a live composer.
//   - hasCompletedProjectOnboarding suppresses the "Welcome back!" panel
//     that a first run in a directory otherwise draws above the composer.
//     Measured harmless on its own (herdr read the pane `idle` with the
//     panel up, composer live, prompt box detected) — it is written
//     anyway, because "harmless splash" is what grok's startup menu was
//     called too, and every detection rule the fleet owns is anchored on
//     the shape of a screen.
func claudeSeedProject(state map[string]any, dir string) {
	projects, _ := state["projects"].(map[string]any)
	if projects == nil {
		projects = map[string]any{}
	}
	proj, _ := projects[dir].(map[string]any)
	if proj == nil {
		proj = map[string]any{}
	}
	proj["hasTrustDialogAccepted"] = true
	proj["hasCompletedProjectOnboarding"] = true
	projects[dir] = proj
	state["projects"] = projects
}

// SeedClaudeTrust makes the session dir trusted for the launch that is
// about to type its command, and returns the config file it wrote — "" for
// the launches with nothing to do: another runtime, or a directory claude
// already trusts. Idempotent, so RelaunchAgent re-asserting it costs a
// read.
//
// It is the host counterpart of SeedCageHome: same key, same merge, a
// different file, and the container tier calls that one instead because
// the config a caged claude reads is the cage HOME's, not the operator's.
//
// cfg is passed in rather than resolved here — HerdrBackend.ClaudeConfig,
// filled by NewHerdrBackend — for the reason App.ModelLister is a field:
// this is the one thing a launch writes OUTSIDE RHQ_HOME and the session
// dir, so a backend that never resolved one (every test backend) must
// write nothing rather than reach into the operator's real config. Hence
// the empty case is a no-op and not a fallback.
//
// THE MERGE IS THE WHOLE POINT. The file holds the operator's entire
// claude state — every project they have opened, their theme, their
// onboarding — so it is read, amended and written back through a temp file
// and a rename, never rewritten from a template. A file posse cannot parse
// is a file posse does not touch: that state refuses the launch instead,
// because an unreadable projects map is a map with no trusted directory in
// it, which means the dialog is exactly what the session would get.
func SeedClaudeTrust(cfg string, rt *Runtime, dir string) (string, error) {
	if cfg == "" || rt == nil || rt.Name != "claude" || dir == "" {
		return "", nil
	}
	p := cfg
	b, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return "", Die("claude directory trust: cannot read %s (%v) — posse seeds projects[%s].hasTrustDialogAccepted there so the session does not open on the trust dialog (rangerhq-w4uf). Fix the file, or run `claude` in %s once and accept the dialog by hand",
			AbbrevHome(p), err, ClaudeTrustKey(dir), dir)
	}
	state := map[string]any{}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &state); err != nil {
			return "", Die("claude directory trust: %s is not JSON posse can read (%v) — it holds the operator's whole claude state, so the launch will not rewrite it, and an unreadable projects map is one that trusts nothing: this session would open on the trust dialog. Repair the file, or run `claude` in %s once and accept the dialog by hand",
				AbbrevHome(p), err, dir)
		}
	}
	if ClaudeTrusted(state, dir) {
		return "", nil
	}
	claudeSeedProject(state, ClaudeTrustKey(dir))
	return p, writeJSONInPlace(p, state)
}

// writeJSONInPlace replaces a JSON file through a temp file in its own
// directory and a rename, keeping the mode it already had (0600 for a new
// one — this is the file claude also keeps account state in). Atomic
// because the alternative is a half-written ~/.claude.json, which costs
// the operator every project they have ever opened.
func writeJSONInPlace(p string, state map[string]any) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	mode := os.FileMode(0o600)
	if st, err := os.Stat(p); err == nil {
		mode = st.Mode().Perm()
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".posse-seed-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op once the rename has taken it
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// claudeTrustProbe backs the interstitial row in `posse runtime check`:
// would a claude launched in the CWD draw the trust dialog today? It reads
// the operator's config and nothing else, like every other probe there.
func claudeTrustProbe() (bool, string) {
	p := ClaudeConfigFile()
	b, err := os.ReadFile(p)
	if err != nil {
		return false, "unreadable " + AbbrevHome(p) + " — cannot tell whether this directory is trusted"
	}
	state := map[string]any{}
	if err := json.Unmarshal(b, &state); err != nil {
		return false, "unparseable " + AbbrevHome(p) + " — cannot tell whether this directory is trusted"
	}
	cwd, err := os.Getwd()
	if err != nil {
		return false, "no cwd to ask about"
	}
	key := ClaudeTrustKey(cwd)
	if ClaudeTrusted(state, cwd) {
		return true, key + " is already trusted in " + AbbrevHome(p) + " — this launch writes nothing"
	}
	return false, key + " is not trusted in " + AbbrevHome(p) + " — the launch seeds it (a dir the operator has never run claude in draws the modal)"
}
