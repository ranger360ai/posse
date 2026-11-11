package posse

import "path/filepath"

// Two members of claude's config directory are executed by claude ITSELF,
// ahead of every Bash command, and the seat that wrote them need not be the
// seat that runs them (ranger-base-5632o, the y438o class outside the 6akzw
// carve).
//
//   - shell-snapshots/: the Bash tool runs every command as `bash -c …
//     source <CLAUDE_CONFIG_DIR>/shell-snapshots/snapshot-bash-<id>.sh …`.
//     MEASURED 2026-11-11 from /proc/$$/cmdline in a live seat, claude
//     2.1.322.
//   - session-env/<session id>/: the scripts hooks leave there are run ahead
//     of Bash commands. Read 2026-11-11 out of the 2.1.322 binary's own text,
//     the refusal its permission checker gives for a write there: "Claude
//     Code runs the scripts that hooks leave there ahead of Bash commands",
//     beside the helper that names them `<event>-hook-<n>.sh`. Not exercised
//     with a planted hook.
//
// Every claude seat on the box shares the directory, and both members are
// one file or one dir per session, all at one uid. So a seat that can write
// the directory could append to another live session's snapshot and have the
// text run in that seat's shell on its next Bash call, with no race. If that
// seat is uncaged (the operator's own claude, a shims-tier seat), the text
// runs outside every wall.
//
// They cannot be carved read-only the way 6akzw's members are: the seat's own
// claude writes its snapshot and its session-env dir at startup. The answer
// is a PRIVATE copy per seat. bwrap binds a per-seat dir over each member, so
// a seat writes and reads only its own and sees none of anyone else's. claude
// only ever reads its own session's entry, so nothing it needs is behind the
// mount.
//
// The source is under the seat's cache dir (SeatCacheDir), so the worktree
// removal that takes the cache takes these too.
//
// bwrap only. A seatbelt profile cannot bind, and "deny write except the
// session's own name" cannot be written ahead of a random session id; the
// darwin side is its own bead.
var claudeSessionStateDirNames = []string{"session-env", "shell-snapshots"}

// claudePrivateSessionStateRel is where those members live under the seat's
// cache dir.
const claudePrivateSessionStateRel = "claude"

// claudePrivateSessionState names the binds for the config dir a seat on this
// box would use (ClaudeConfigDirIn, the claudeExecConfig answer), and only
// when the launching runtime's state grant reaches that dir. A seat on another
// runtime gets nothing, and so does a launch with no seat cache dir.
func claudePrivateSessionState(home, cwd string, stateGrants []string) []PrivateBind {
	seat := SeatCacheDir(home, cwd)
	if seat == "" {
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
	var out []PrivateBind
	for _, n := range claudeSessionStateDirNames {
		out = append(out, PrivateBind{
			Src: filepath.Join(seat, claudePrivateSessionStateRel, n),
			Dst: filepath.Join(cfg, n),
		})
	}
	return out
}

// runtimeStateGrants is a runtime's state dirs as the wall grants them:
// tilde-expanded, absolute and resolved. Anything not absolute after the
// expansion is not a grant.
func runtimeStateGrants(stateDirs []string) []string {
	var out []string
	for _, d := range stateDirs {
		if e := ExpandTilde(d); filepath.IsAbs(e) {
			out = append(out, absResolve(e))
		}
	}
	return out
}
