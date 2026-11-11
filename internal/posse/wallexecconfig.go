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
