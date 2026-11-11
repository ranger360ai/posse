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
var (
	claudeExecConfigDirNames  = []string{"agents", "commands", "hooks", "output-styles", "plugins", "rules", "skills"}
	claudeExecConfigFileNames = []string{"CLAUDE.md", "settings.json", "settings.local.json"}
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
// Residual: ranger-base-74pqi. A seat on another runtime gets no deny for a
// home nothing granted it.
var (
	codexExecDirNames  = []string{"plugins", "prompts", "rules", "skills"}
	codexExecFileNames = []string{"AGENTS.md", "AGENTS.override.md", "hooks.json"}
	grokExecDirNames   = []string{"agents", "commands", "hooks", "installed-plugins", "personas", "plugins", "rules", "skills", "workflows"}
	grokExecFileNames  = []string{
		"AGENT.md", "AGENTS.md", "Agents.md", "CLAUDE.local.md", "CLAUDE.md", "Claude.md",
		"lsp.json", "managed_config.toml", "requirements.toml", "settings.json", "trusted_folders.toml",
	}
)

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
