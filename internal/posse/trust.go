package posse

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
// WHAT THE GRANT BUYS THE REPO — REMEASURED 2026-08-27 on 2.1.247 and
// re-run unchanged on 2.1.241/.245/.246 (ranger-base-i0s8), because the
// first telling of this paragraph was wrong in its detail:
//
//   - The "Dropped N project-scoped <key> entries — workspace not yet
//     trusted" line is real, but its template is dynamic and the shipped
//     bundle passes it exactly two keys: `permissions.allow` and
//     `permissions.additionalDirectories`. It never says `hooks`. The
//     earlier quote here attributed a permissions message to hooks.
//   - `hooks` are not dropped at settings load at all. They are gated one
//     layer down, at execution: "Skipping <event> hook execution —
//     workspace trust not accepted". Interactive and untrusted, the
//     session parks on the dialog and the project's SessionStart hook
//     never runs (SessionEnd is skipped with that line). Interactive and
//     trusted — the state this file seeds — it runs.
//   - Headless is the shape that does NOT hold: `claude -p` in an
//     UNTRUSTED directory runs the project's hooks, in the same run that
//     drops that file's `permissions.allow`, and writes no trust. So trust
//     is what enables repo hooks for a posse LAUNCH, which is interactive;
//     it is not what gates hooks in general.
//   - `mcpServers` under .claude/settings.json is inert on these builds:
//     never listed by `claude mcp list`, never named in a trusted
//     session's debug log, never spawned. The live project-MCP channel is
//     `.mcp.json`, and it sits behind its own "Pending approval" gate that
//     reads identically trusted and untrusted.
//
// The launch check is unchanged by all of this, and is MORE load-bearing
// for it: it fires on settings content, so it does not depend on which of
// claude's gates is holding. Claude's built-in runtime names this file, its
// local sibling `.claude/settings.local.json` (the same scope and the same
// pre-turn exec channel — measured, rangerhq-9u8, ADR 0002 amendment
// 2026-08-28), and the top-level keys above; permission-only settings stay
// clean, while a match or an unclassifiable file degrades the launch before
// this trust seed is written. Naming `mcpServers` there is deliberately conservative
// — a key claude ignores today is a key it may honor tomorrow.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
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
// inherits that repo's answer. MEASURED 2026-08-27 on 2.1.247
// (ranger-base-i0s8): with only a repo root trusted, both a subdirectory
// and a `git worktree add` linked worktree of it opened on a live
// composer, while an untrusted sibling repo drew the dialog, and claude
// added no projects entry for either. Following a `.git` file through its
// commondir to decide whether to skip a write is more machinery than the
// write is worth, so a fresh worktree of a trusted repo gets an entry it
// did not strictly need — belt, not load-bearing. The error only ever runs
// that way: nothing this reports trusted would have drawn the dialog.
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
// lock and a read.
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
//
// AND THE MERGE IS CHECK-THEN-ACT, so it runs under a lock (ranger-base-5qnt).
// Read-amend-rename by two launches at once is a textbook lost update: both
// read the operator's state, each adds its own dir, and the second rename
// wins with a projects map that never held the first one's key. The sibling
// launch then opens on the dialog this file exists to answer — rangerhq-w4uf
// again, one launcher over. The atomic rename is no defence: it makes the
// file whole, not the merge correct. Measured 20/20 lost at N=2 before the
// lock (verifying ranger-base-s83).
//
// The window is not narrowable, so it is closed: the lock is held across
// read, check and write, which is the standard answer for a check-then-act
// on state another actor mutates (CWE-367). It is per config FILE and not
// per RHQ_HOME, because the launcher lock (ADR 0011 §1) already serializes
// one RHQ_HOME's dispatch and is not the failing shape: `posse new` takes
// no launcher lock, and a fleet RHQ_HOME and a scratch one write this same
// file with two locks between them.
func SeedClaudeTrust(cfg string, rt *Runtime, dir string) (string, error) {
	if cfg == "" || rt == nil || rt.Name != "claude" || dir == "" {
		return "", nil
	}
	p := cfg
	unlock, err := lockClaudeConfig(p)
	if err != nil {
		return "", err
	}
	defer unlock()
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
	// Two questions now, and a launch has to answer BOTH or it has answered
	// neither: a trusted dir whose config has never seen the outside-read
	// notice is a session that opens on a composer and stops on a dialog at
	// its first read past the worktree (ranger-base-d3fwo). So the
	// nothing-to-do return is the conjunction, and each key is written only
	// when it is the one missing.
	trusted, seen := ClaudeTrusted(state, dir), claudeOutsideReadSeen(state)
	if trusted && seen {
		return "", nil
	}
	if !trusted {
		claudeSeedProject(state, ClaudeTrustKey(dir))
	}
	state[ClaudeOutsideReadSeenKey] = true
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

// ─── one writer at a time on the operator's config ───────────────────────────

// claudeConfigLockFile is the sidecar the read-amend-rename above is held
// under: <cfg>.posse-lock, beside the config and deliberately not the
// config itself.
//
// NOT the config, because the write ends in a rename and a rename replaces
// the inode under the path. A holder that flocked the old inode and a
// waiter that opened the new one are two holders of one path with no error
// anywhere — the same trap launchlock.go names for unlinking its lock
// file. The sidecar is created once and never renamed, truncated or
// removed, so every process locking it locks the same inode.
//
// Keyed on the config path so the two callers that matter meet on it: a
// dispatch launch under one RHQ_HOME's launcher lock and a hand-run `posse
// new` under none, both writing the operator's one file.
func claudeConfigLockFile(cfg string) string { return cfg + ".posse-lock" }

// claudeConfigLockWait bounds the wait. flock is held by the open file
// description, so a dead holder's lock is already released by the kernel
// and there is no staleness to reap (ADR 0011 §1) — the only thing this
// deadline can hit is a live holder wedged inside a critical section that
// is one read and one rename of a small file. Waiting past that is not
// patience, it is a launch hanging silently in a pane nobody is watching,
// which is the failure class this file exists to prevent. So it fails
// loudly instead, naming the file to look at.
const claudeConfigLockWait = 10 * time.Second

// lockClaudeConfig takes the exclusive lock on cfg's sidecar and returns
// the release. Callers must not nest it: flock is per open file
// description, so a second Open+Flock in this process waits on the first
// forever (launchlock.go, same reason). The one holder is SeedClaudeTrust.
func lockClaudeConfig(cfg string) (func(), error) {
	path := claudeConfigLockFile(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, Die("claude directory trust: cannot make %s (%v) — the seed writes projects[].hasTrustDialogAccepted there and will not write it unlocked (ranger-base-5qnt)", AbbrevHome(filepath.Dir(path)), err)
	}
	// 0600 like the file it guards: it sits in the operator's home beside
	// a config that also holds account state.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, Die("claude directory trust: cannot open %s (%v) — posse serializes the merge of %s on it, and an unserialized merge drops a concurrent launch's trusted directory (ranger-base-5qnt)", AbbrevHome(path), err, AbbrevHome(cfg))
	}
	deadline := time.Now().Add(claudeConfigLockWait)
	for {
		err := flock(f, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { f.Close() }, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			f.Close()
			return nil, Die("claude directory trust: cannot lock %s (%v)", AbbrevHome(path), err)
		}
		if !time.Now().Before(deadline) {
			f.Close()
			return nil, Die("claude directory trust: %s held by another posse launch for %s — this launch will not merge %s unserialized (that is how a sibling launch's trusted directory goes missing, ranger-base-5qnt). Check for a wedged `posse new` or dispatch; the lock frees itself when its holder exits",
				AbbrevHome(path), claudeConfigLockWait, AbbrevHome(cfg))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// claudeTrustProbe backs the interstitial row in `posse runtime check`:
// would a claude launched in the CWD draw the trust dialog today? It reads
// the operator's config and nothing else, like every other probe there.
func claudeTrustProbe() Silence {
	p := ClaudeConfigFile()
	b, err := os.ReadFile(p)
	if err != nil {
		return Silence{Unknown: true, Why: "unreadable " + AbbrevHome(p) + " — cannot tell whether this directory is trusted"}
	}
	state := map[string]any{}
	if err := json.Unmarshal(b, &state); err != nil {
		return Silence{Unknown: true, Why: "unparseable " + AbbrevHome(p) + " — cannot tell whether this directory is trusted"}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return Silence{Unknown: true, Why: "no cwd to ask about"}
	}
	key := ClaudeTrustKey(cwd)
	if ClaudeTrusted(state, cwd) {
		return Silence{Silenced: true, Why: key + " is already trusted in " + AbbrevHome(p) + " — this launch writes nothing"}
	}
	return Silence{Why: key + " is not trusted in " + AbbrevHome(p) + " — the launch seeds it (a dir the operator has never run claude in draws the modal)"}
}

// ClaudeOutsideReadSeenKey is claude's record that the operator has already
// been shown the one-time "Allow reads outside the working directories?"
// notice. It is a TOP-LEVEL key of the same config file directory trust
// lives in — not a per-project one — because the question it settles is
// asked once per config dir, not once per repo.
//
// ranger-base-d3fwo. MEASURED on claude 2.1.258, by reading the installed
// binary (the arming condition is one expression; a live pane arm would
// have cost a turn and an unanswered config dir, and could not have said
// anything this does not):
//
//	function Fon(e,n,r,o){ … if(!hy(d.mode)||d.shouldAvoidPermissionPrompts
//	  ||d.blockReadsOutsideWorkingDirectories===!0)return!1; …
//	  return !o.session.outsideReadPrompt.isOpenElsewhere(o.toolUseId)
//	    && !o.session.outsideReadPrompt.answered
//	    && !ie().hasSeenAutoModeOutsideReadPrompt }
//
// and the dialog's own accept arm calls `Aqn`, whose whole body is
// `hasSeenAutoModeOutsideReadPrompt: true`. So "1. Yes, keep allowing" —
// what monica typed by hand into gilfoyle's pane at 07:02Z — writes this
// key and nothing else. Seeding it is therefore not a permission decision
// at all: it reproduces, per launch, the answer the operator already gave,
// and an outside read behaves exactly as it does on this box today.
//
// WHY NOT --settings, which is what the bead asked for. The key the dialog
// names is `permissions.blockReadsOutsideWorkingDirectories`, and neither
// value settles the question:
//
//   - `false` does NOTHING. The guard above tests `===!0` — strictly true —
//     so a false, or the absence the CLI already has, leaves the prompt
//     armed. Shipping it would have looked like a fix in the launch line
//     and changed no behaviour at all, which is worse than shipping
//     nothing: the next session to sit blocked would have a settled-looking
//     flag to disbelieve.
//   - `true` does suppress the prompt, by REFUSING the read it was asking
//     about — "the file tools refuse reads outside the working directories
//     in every project", claude's own words. gilfoyle would not have been
//     blocked on the runbook; gilfoyle would have been unable to read it.
//     It is also restrictive-wins across every settings source, so a
//     session cannot take it back.
//
// The third lever is working directories (`--add-dir`, or
// `permissions.additionalDirectories`): a read inside one is not an outside
// read, so the prompt never arms for those paths. That fixes the paths a
// launch can enumerate and no others, and it widens what the file tools may
// touch — it is a permission decision, and this is not. Left to whoever
// prices it; the seed is what makes the session survive the general case.
//
// WHAT SEEDING WIDENS, said plainly: on the host path this is the first
// GLOBAL key posse writes into the operator's config (directory trust is
// per project). The operator's own next interactive session skips the
// notice too. That is the state this box has been in since 07:02Z anyway,
// and the alternative — a per-persona CLAUDE_CONFIG_DIR — is a much larger
// change to dodge a one-line notice.
const ClaudeOutsideReadSeenKey = "hasSeenAutoModeOutsideReadPrompt"

// claudeOutsideReadSeen reads the key the way claude does: strictly true.
// Anything else — absent, false, a string — is a config that still arms the
// prompt, so it is one the launch has something to write.
func claudeOutsideReadSeen(state map[string]any) bool {
	v, _ := state[ClaudeOutsideReadSeenKey].(bool)
	return v
}

// claudeOutsideReadProbe backs the second interstitial row in `posse
// runtime check`: would a claude launched here today draw the outside-read
// notice? Like every probe there it reads the operator's config and
// nothing else.
func claudeOutsideReadProbe() Silence {
	p := ClaudeConfigFile()
	b, err := os.ReadFile(p)
	if err != nil {
		return Silence{Unknown: true, Why: "unreadable " + AbbrevHome(p) + " — cannot tell whether the outside-read notice has been seen"}
	}
	state := map[string]any{}
	if err := json.Unmarshal(b, &state); err != nil {
		return Silence{Unknown: true, Why: "unparseable " + AbbrevHome(p) + " — cannot tell whether the outside-read notice has been seen"}
	}
	if claudeOutsideReadSeen(state) {
		return Silence{Silenced: true, Why: ClaudeOutsideReadSeenKey + " is already set in " + AbbrevHome(p) + " — this launch writes nothing"}
	}
	return Silence{Why: ClaudeOutsideReadSeenKey + " is not set in " + AbbrevHome(p) + " — the launch seeds it (an auto-mode session reading outside its worktree would otherwise stop on the notice)"}
}
