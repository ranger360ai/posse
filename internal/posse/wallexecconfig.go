package posse

import (
	"os"
	"path/filepath"
	"strings"
)

// claude's config directory is two things in one directory, and the wall has
// to tell them apart (ranger-base-6akzw, finding ranger-base-y438o).
//
// It is the runtime's STATE — the CLI writes `.claude.json`, history,
// projects/, sessions/, shell snapshots and caches there on every turn, and
// ADR 0080 D19 grants the whole directory at the bwrap mechanism because a
// directory grant is the only one that survives the atomic replace those
// writers make (probe B8).
//
// It is also EXECUTABLE CONFIG for every other claude on the box that shares
// the directory — uncaged seats, and the operator's own claude. Hooks in
// settings.json are commands; skills, agents, commands, output styles and
// CLAUDE.md are instructions; plugins/ holds marketplace clones and plugin
// code. A seat that can write those can plant something that runs, or is
// obeyed, outside its own wall, and keeps running after the seat is gone.
//
// So the members below are carved READ-ONLY over the grant, the way the git
// config chain is carved over a worktree grant (sessionGitConfigFiles).
// MEASURED 2026-11-11 on the D1 host, claude through mise's wrapper: with
// settings.json, skills/, plugins/ and `.local/share` all bound read-only, a
// `claude -p` turn exits 0 with its answer, and its debug log records no
// refused write. That is one print-mode turn, not every code path: the
// interactive marketplace refresh writes plugins/known_marketplaces.json and
// is expected to fail quietly here — which is the point, since that file
// names where plugin code is fetched from. ranger-base-y438o's re-verify
// from a live seat is the full-lifecycle reading.
//
// The names are the ones the CLI was MEASURED reading at user scope (its
// debug log stats settings.json, agents/, commands/, output-styles/ and
// skills/, and loads plugins/), plus the documented ones it reads when
// present (CLAUDE.md, rules/, hooks/) and settings.local.json, which costs
// nothing to cover and closes the next release that starts reading it.
//
// NOT HERE, AND STATED: `.claude.json` itself. It also carries commands —
// user-scope MCP servers — and it cannot be read-only, because it is the file
// the CLI rewrites every turn. Closing that needs a per-seat config directory,
// which is a design and not a carve: ranger-base-nv3el.
//
// The second half of the list is ranger-base-d0rnr's, and its census is
// claude's OWN: the 2.1.322 binary's sandbox write-deny names these members
// of the config dir beside the ones above, and its file tools refuse them.
// MEASURED 2026-11-11 on the D1 host, claude 2.1.322 under bwrap, a scratch
// CLAUDE_CONFIG_DIR, one unbilled `-p` turn against a local fake endpoint
// that logged every request body:
//
//   - workflows/: a script planted there is IN THE REQUEST, listed as an
//     invocable skill beside a planted command — a saved Workflow, a JS
//     script that spawns agents when it is named.
//   - routines/: claude lists it with commands/, skills/ and workflows/
//     among the config dirs its own permission rules guard. A planted .md
//     did not reach a `-p` request; its reader is UNMEASURED.
//   - daemon.json, daemon/: read from the binary's text, not exercised —
//     "the service that runs background sessions starts the workers listed
//     there without asking anyone", and "a file there can start a
//     background session without asking anyone". daemon.json also holds the
//     daemon's cron tasks (prompts and permission modes).
//   - scheduled_tasks.json, launch.json: claude reads both at
//     `<project>/.claude/`, so in the default config dir they are the HOME
//     project's: durable cron prompts a later session in `~` fires, and the
//     Preview launch config's commands. `{}` loads as no tasks (no `tasks`
//     array).
//   - With all six bound read-only (the dirs empty, the files `{}`), the
//     turn exits 0, its Bash call runs, the debug log names no refused
//     write, and the files are byte-identical afterwards. No member exists
//     in this box's live config dir, so no seat on it has written one.
//
// Residual: ranger-base-h5vij. Its caches are claudeManagedCache's.
var (
	claudeExecConfigDirNames  = []string{"agents", "commands", "daemon", "hooks", "output-styles", "plugins", "routines", "rules", "skills", "workflows"}
	claudeExecConfigFileNames = []string{"CLAUDE.md", "daemon.json", "launch.json", "scheduled_tasks.json", "settings.json", "settings.local.json"}
)

// claudeExecConfig names those members for the config directory a seat on
// this box would use — CLAUDE_CONFIG_DIR when it is in force, `<home>/.claude`
// otherwise, the same answer BwrapStateDirs grants — and only when the
// launching runtime's own state grant reaches that directory. That grant is
// what this carve narrows (ADR 0080 D19's, or seatbelt's `~/.claude`), and it
// is the only grant that reaches the directory on a box whose home is not
// itself under a granted path. A seat on another runtime gets no deny lines
// for a directory nothing granted it.
func claudeExecConfig(home string, stateGrants []string) (dirs, files []string) {
	if home == "" {
		return nil, nil
	}
	cfg := ClaudeConfigDirIn(home)
	if !filepath.IsAbs(cfg) {
		return nil, nil
	}
	cfg = absResolve(cfg)
	if !writeGranted(stateGrants, cfg) {
		return nil, nil
	}
	for _, n := range claudeExecConfigDirNames {
		dirs = append(dirs, filepath.Join(cfg, n))
	}
	for _, n := range claudeExecConfigFileNames {
		files = append(files, filepath.Join(cfg, n))
	}
	return dirs, files
}

// claude's config dir also holds CACHES of server-fetched policy, and one of
// them is obeyed as policy (ranger-base-l751c, census ranger-base-h5vij).
// remote-settings.json is the stored copy of the account's managed settings,
// loaded at POLICY tier for an account claude deems eligible: any API-key
// login against api.anthropic.com, team and enterprise OAuth. Policy tier
// carries env, permissions and hooks. policy-limits.json is the stored copy of
// the org's policy limits. The rest of the list is their signature, stamp and
// consent siblings, the same set claude 2.1.322's own sandbox write-denies
// (read from its binary, at the `remote-settings-helper-consent` loop and the
// `rEe`/`u4e`/`FQe`/`d_t` name helpers it applies to each cache).
//
// MEASURED 2026-11-12 on the D1 host, claude 2.1.322, bwrap 0.13.1, unbilled:
// a scratch CLAUDE_CONFIG_DIR, an API-key login with no base-URL override, run
// in a private user+net+mount namespace where api.anthropic.com resolves to a
// local TLS fake (the namespace has only lo, so nothing left the box). The
// fake fails the settings fetch, so a stored copy is the only source, and asks
// for one Bash call that prints the env it got.
//
//   - An UNSIGNED planted `{"env":{...}}` is OBEYED: its variable reaches the
//     Bash tool. The stored-copy gate is the server flag tengu_fizzy_widget,
//     and its served value is cached in `.claude.json`'s
//     cachedGrowthBookFeatures, which the seat writes every turn (it cannot
//     be carved: ranger-base-nv3el). Flag false: obeyed. Flag absent (a fresh
//     cache): obeyed. Flag true: refused. So the gate cannot protect this
//     cache against a seat, and this carve is the only control.
//   - With /dev/null bound over every member, the same planted copy is NOT
//     obeyed, the turn exits 0, and settings the server serves are still
//     applied for the run (claude holds them in memory; a `-p` run persists
//     nothing anyway).
//   - An EMPTY 0600 member reads as no stored copy ("no cached settings") to
//     an uncaged eligible claude, and that turn exits 0 too.
//
// So the members are NOT seeded. A `{}` seed would be a stored answer of
// posse's making ("no managed settings") that every claude on the box reads.
// At bwrap, /dev/null is bound over each one, present or absent
// (SeatbeltCarveOut.Mask), and an absent one gets an empty 0600 mount point
// first, because bwrap's own is 0444 and claude rewrites these caches IN
// PLACE (O_TRUNC, no rename), so a 0444 one would stop the operator's own
// claude from ever caching again. At seatbelt, the write deny needs no name.
//
// This box's own login is a Max OAuth account, which reads as ineligible, and
// none of these exists in its live config dir. The exposure is a box running
// posse on an API key or a team plan.
var claudeManagedCacheFileNames = []string{
	"policy-limits.json",
	"policy-limits.json.signature-iat.json",
	"policy-limits.json.signature.json",
	"policy-limits.json.stamp.json",
	"remote-settings-consent.json",
	"remote-settings-helper-consent",
	"remote-settings.json",
	"remote-settings.json.signature-iat.json",
	"remote-settings.json.signature.json",
	"remote-settings.json.synced-at",
}

// claudeManagedCache names those members for the config dir a seat on this box
// would use, under the same guard as claudeExecConfig: only when the
// launching runtime's state grant reaches that dir.
func claudeManagedCache(home string, stateGrants []string) (files []string) {
	if home == "" {
		return nil
	}
	cfg := ClaudeConfigDirIn(home)
	if !filepath.IsAbs(cfg) {
		return nil
	}
	cfg = absResolve(cfg)
	if !writeGranted(stateGrants, cfg) {
		return nil
	}
	for _, n := range claudeManagedCacheFileNames {
		files = append(files, filepath.Join(cfg, n))
	}
	return files
}

// codex and grok keep their command-bearing config in ONE file each, and
// unlike claude's `.claude.json` that file can be read-only (ADR 0090 D3,
// ranger-base-zmk4q). `config.toml` under the CLI's home declares MCP
// servers, codex's `notify`, grok's hooks, notification hooks, status-line
// and auth-provider commands — commands every later reader of the file
// starts outside the wall that wrote it.
//
// D3's rule is a launch override where the CLI has one, else a read-only
// carve if the CLI runs with it. MEASURED 2026-11-11 on the D1 host, codex
// 0.167.1 and grok 1.0.77, scratch homes, one unbilled turn each against a
// local fake endpoint:
//
//   - codex: `-c notify=[]` beats the file; `-c mcp_servers={}` does NOT —
//     codex merges tables, so the file's servers survive it. No override
//     covers the key, so the file is carved. The interactive TUI rewrites
//     the file mid-session (`[tui]` bookkeeping); bound read-only it logs
//     the refused write (EBUSY) and completes the turn.
//   - grok: no flag or env reaches `mcp_servers` (its `GROK_CONFIG` overlay
//     is allowlisted and drops it). It rewrites the file at startup in both
//     `-p` and the TUI (`[marketplace]` bookkeeping); bound read-only both
//     complete the turn.
//
// The carve covers every key in the file at once, including the ones no
// census here named, which is why the per-key override is not also typed:
// a caged seat that cannot write the file cannot plant `notify` in it.
//
// The other executable members of those homes are runtimeExecMembers'
// (ranger-base-zvgql).
func runtimeConfigFiles(home string, stateGrants []string) (files []string) {
	if home == "" {
		return nil
	}
	for _, h := range []string{codexHomeIn(home), grokHomeIn(home)} {
		if !filepath.IsAbs(h) {
			continue
		}
		p := absResolve(filepath.Join(h, "config.toml"))
		if writeGranted(stateGrants, p) {
			files = append(files, p)
		}
	}
	return files
}

// config.toml is not the only member of a codex or grok home that a later
// reader starts or obeys (ranger-base-zvgql, the class 6akzw carved for
// claude). The members below are carved read-only over the same state grant,
// by the same rule: dirs whole, files seeded empty when absent so the mount
// can cover them.
//
// MEASURED 2026-11-11 on the D1 host, codex 0.167.1 and grok 1.0.77, scratch
// homes, one unbilled turn each against a local fake endpoint that logged
// every request body. "In the request" means a token planted in the member
// reached the model; "started" means a planted command touched a marker.
//
//   - codex: AGENTS.md, AGENTS.override.md and a skill's description under
//     skills/ are in the request. hooks.json is the CLI's hook file (feature
//     `hooks`, stable, on); a planted hook did not fire here because codex
//     gates hooks on a trust review — firing UNMEASURED. plugins/, prompts/
//     and rules/ (exec-policy approvals) are documented readers, not
//     exercised. codex installs its system skills into skills/.system at
//     startup, scripts included; read-only, it skips that install and
//     completes the turn. A
//     managed_config.toml or requirements.toml under CODEX_HOME started
//     nothing while the same server in config.toml did, so neither is here.
//   - grok: a hooks/*.json SessionStart and UserPromptSubmit hook ran, in
//     an untrusted folder. lsp.json's server command ran. A plugin under
//     plugins/ loads with its hooks and MCP server (`grok inspect`).
//     rules/*.md, a home AGENTS.md, skills/ and commands/ are in the
//     request, and so are all six instruction names it reads at home level.
//     managed_config.toml and requirements.toml are DELETED by grok's
//     startup when it can delete them, which is why ranger-base-zmk4q saw
//     them start nothing — read-only, both files' hooks ran and both MCP
//     servers started. They are carved because a seat must not be the one
//     to write them. trusted_folders.toml decides where project hooks run
//     for every later grok. Its seed is an empty file, which grants nothing:
//     ADR 0007 §6f's "posse does not seed the grant" still holds, and the
//     carve keeps the grant the operator's act, which is that section's
//     point. installed-plugins/ is where installs land.
//     agents/, personas/ and workflows/ are documented readers (agents can
//     declare MCP servers and hooks), not exercised.
//   - With every member here bound read-only, codex exec and grok -p (with
//     and without --trust) complete the turn and leave every member
//     byte-identical. An empty AGENTS.override.md does not shadow
//     AGENTS.md, and an empty hooks.json is accepted.
//
// A seat on another runtime gets no deny for a home nothing granted it.
//
// The second half of each list is ranger-base-6ob20's (finding
// ranger-base-74pqi, whose comments carry the per-member readings):
//
//   - grok's census is grok's OWN: its sandbox doc
//     (docs/user-guide/18-sandbox.md, "Direct global write protection", as
//     shipped with grok 1.0.77) write-denies bundled/ and bundles/ ("anything
//     placed there loads as a built-in skill, agent, or workflow"),
//     hooks-paths (a registry whose absolute targets load as hook JSON) and
//     sandbox.toml (the profiles a config.toml profile name selects). Beside
//     those: bin/, the managed install a running leader relaunches onto, with
//     channel-switch.json, which picks its channel; and memory-v2/, whose
//     index is injected into every later session when memory is on. A seat
//     has no business updating the operator's grok, so its line also carries
//     GROK_DISABLE_AUTOUPDATER=1 (the runtime's LineEnv). grok makes an empty
//     hooks-paths itself, so the empty seed is the vendor's own shape.
//   - codex: .tmp/ holds a git clone of the curated plugin marketplace and its
//     sha, and `codex plugin list` names plugin sources inside that clone, so
//     a writer's copy is installed under the curated label. memories/ is read
//     by an operator who turns the memories feature on.
//   - codex's tmp/ and shell_snapshots/ are a different class and are not in
//     these lists: codexEphemeralDirs has them.
//
// MEASURED 2026-11-12 on the D1 host, codex 0.167.2 and grok 1.0.77, the
// zvgql method (scratch homes, a local fake endpoint, no billed turn): with
// every member here bound read-only, `codex exec` and `grok -p` complete the
// turn and leave every seeded member byte-identical
// (TestLiveWallRuntimeConfigEachCLIRunsWithItsConfigReadOnly). The rest of
// the readings are in docs/notes.d/ranger-base-6ob20.md.
//
// Residual: ranger-base-qyq59.
var (
	codexExecDirNames  = []string{".tmp", "memories", "plugins", "prompts", "rules", "skills"}
	codexExecFileNames = []string{"AGENTS.md", "AGENTS.override.md", "hooks.json"}
	grokExecDirNames   = []string{"agents", "bin", "bundled", "bundles", "commands", "hooks", "installed-plugins", "memory-v2", "personas", "plugins", "rules", "skills", "workflows"}
	grokExecFileNames  = []string{
		"AGENT.md", "AGENTS.md", "Agents.md", "CLAUDE.local.md", "CLAUDE.md", "Claude.md",
		"channel-switch.json", "hooks-paths",
		"lsp.json", "managed_config.toml", "requirements.toml", "sandbox.toml", "settings.json", "trusted_folders.toml",
	}
)

// codexEphemeralDirNames are the members of a codex home that hold
// per-PROCESS state another process then runs (ranger-base-6ob20, finding
// ranger-base-74pqi rows 1 and 2):
//
//   - tmp/: every codex process makes a fresh `tmp/arg0-linux/codex-arg0*`
//     dir and puts it FIRST on PATH for every command it runs. MEASURED
//     (74pqi): each process makes its own, and removes stale ones whose lock
//     is not held.
//   - shell_snapshots/: a `<thread>.sh` written at every start, shell code
//     the session sources before its commands.
//
// Nothing there persists past the process that made it, so the exposure is a
// LIVE window: while an uncaged codex sharing the home runs, its dir sits
// inside a caged seat's grant. The answer is the 5632o shape, not a carve: on
// bwrap each seat gets an empty private tmpfs over both (bwrapRender), so it
// writes its own and sees nobody else's; seatbelt cannot mount, so it carves
// them read-only (SeatbeltCarveOut). MEASURED (74pqi) codex prints `could not
// create PATH aliases` and proceeds when tmp/ is unwritable.
var codexEphemeralDirNames = []string{"shell_snapshots", "tmp"}

// codexEphemeralDirs names those dirs for the codex home a seat's state grant
// reaches, following CODEX_HOME, or nothing.
func codexEphemeralDirs(home string, stateGrants []string) (dirs []string) {
	if home == "" {
		return nil
	}
	h := codexHomeIn(home)
	if !filepath.IsAbs(h) {
		return nil
	}
	h = absResolve(h)
	if !writeGranted(stateGrants, h) {
		return nil
	}
	for _, n := range codexEphemeralDirNames {
		dirs = append(dirs, filepath.Join(h, n))
	}
	return dirs
}

// runtimeExecMembers names those members for each CLI home the launching
// runtime's state grant reaches, following CODEX_HOME / GROK_HOME the way
// runtimeConfigFiles does.
func runtimeExecMembers(home string, stateGrants []string) (dirs, files []string) {
	if home == "" {
		return nil, nil
	}
	for _, cli := range []struct {
		home        string
		dirs, files []string
	}{
		{codexHomeIn(home), codexExecDirNames, codexExecFileNames},
		{grokHomeIn(home), grokExecDirNames, grokExecFileNames},
	} {
		if !filepath.IsAbs(cli.home) {
			continue
		}
		h := absResolve(cli.home)
		if !writeGranted(stateGrants, h) {
			continue
		}
		for _, n := range cli.dirs {
			dirs = append(dirs, filepath.Join(h, n))
		}
		for _, n := range cli.files {
			files = append(files, filepath.Join(h, n))
		}
	}
	return dirs, files
}

// seedFileWhole creates p holding body, and fails with an IsExist error when
// p is already there, the way O_CREATE|O_EXCL does. A body that is not empty
// is written to a temp file beside p and hard-linked into place, so no reader
// ever sees p with zero bytes. A seed lands in a home that other CLIs read
// while it is written, and a zero-byte `settings.json` is invalid JSON to
// them (ranger-base-zvgql: a grok project-config classifier in a parallel
// test read one mid-seed). The link and not a rename, because a rename would
// replace a file that appeared since the render.
func seedFileWhole(p, body string) error {
	if body == "" {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		return f.Close()
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "."+filepath.Base(p)+".posse-seed-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.WriteString(body)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp.Name(), 0o644)
	}
	if werr != nil {
		return werr
	}
	return os.Link(tmp.Name(), p)
}

// wallHome is the home the wall resolves `~` against: $HOME, the value
// ExpandTilde reads, so a deny and the grant it narrows are spelled against
// one home (BwrapStateDirs' own note on the same choice).
func wallHome() string { return os.Getenv("HOME") }

// seedBody is what the launcher writes into a Seed file that is not there
// yet, so a read-only mount can cover it. It is the EMPTY value of the file's
// own format and nothing else: `{}` for JSON, which every settings reader
// takes as "no settings", and zero bytes for anything else, which is an empty
// CLAUDE.md. A seed never carries content of posse's choosing beyond that,
// because the file it creates is read by the operator's own claude too.
func seedBody(p string) string {
	if strings.HasSuffix(p, ".json") {
		return "{}\n"
	}
	return ""
}
