package posse

import (
	"os"
	"path/filepath"
	"strings"
)

// claude's config dir holds projects/<key>/, one dir per working directory
// (claudeProjectDir is the key). Inside it are the session transcripts
// (<uuid>.jsonl, plus a <uuid>/ dir) and memory/. Other claude processes read
// both back: the transcripts on --resume and through posse's own readers, and
// memory/ at every interactive session start for that key, with no flag
// needed. The caged write set grants the whole config dir (ADR 0080 D19), so
// one seat could write into another key's transcripts or memory, including
// the operator's (ranger-base-ajkqx).
//
// The bwrap answer, in place and not private, so every reader keeps its path:
//
//  1. projects/ is bound read-only over the config-dir grant;
//  2. the seat's OWN key dir is bound read-write again on top of that, but
//     only when cwd is the launch's own session worktree (claudeOwnSessionTree).
//     MEASURED 2026-11-11 over 40 recent seat key dirs: they hold only
//     <uuid>.jsonl and <uuid>/, so this is everything a seat writes there;
//  3. memory/ inside the own key is bound read-only again. The fleet writes no
//     auto-memory, and the own key can equal the operator's.
//
// A seat whose cwd is not its own session worktree (the shared checkout, a
// probe in a scratch dir) gets 1 and nothing else. MEASURED 2026-11-12, claude
// 2.1.322: `claude -p` with projects/ wholly read-only exits 0 and answers;
// it writes no transcript and prints nothing about it.
//
// The seatbelt answer (ranger-base-r2uxm) is the SAME list read as SBPL rules
// in the same order, deny for ReadOnly and allow otherwise
// (seatbeltProjectsNarrow), so the two mechanisms cannot name different dirs.
// ASSUMED, not measured from this host: SBPL's last matching rule wins, as
// SeatbeltCarveOut's Deny already relies on (MEASURED there, macOS 26.4).
const claudeProjectsDirName = "projects"

// seatbeltProjectsNarrow turns claudeProjectsBinds into the profile's ordered
// write rules: deny projects/, allow the own key, deny its memory/.
//
// A link (or a file) at any of those names drops the reopen, so projects/ is
// read-only whole. bwrap refuses to render over one instead, because a mount
// would follow it; SBPL matches the resolved path, so a link at the own key
// would only make the allow match nothing, but a link at memory/ pointing
// back into the own key would make the memory claude reads writable under
// another name. Failing closed costs the seat its transcript and nothing else
// (MEASURED 2026-11-12, see the top of this file). A link at projects/ itself
// gets its resolved spelling denied too, since the profile matches that one.
func seatbeltProjectsNarrow(binds []PrivateBind) []SeatbeltNarrow {
	if len(binds) == 0 {
		return nil
	}
	for _, pb := range binds {
		if fi, err := os.Lstat(pb.Dst); err == nil && !fi.IsDir() {
			projects := binds[0].Dst
			out := []SeatbeltNarrow{{Path: projects}}
			if real := absResolve(projects); real != projects {
				out = append(out, SeatbeltNarrow{Path: real})
			}
			return out
		}
	}
	out := make([]SeatbeltNarrow, 0, len(binds))
	for _, pb := range binds {
		out = append(out, SeatbeltNarrow{Path: pb.Dst, Allow: !pb.ReadOnly})
	}
	return out
}

// claudeMemoryDirName is the auto-memory dir inside a key dir.
const claudeMemoryDirName = "memory"

// claudeProjectsBinds names the in-place binds over the config dir's
// projects/, for a seat whose runtime state grant reaches that dir. ownTree
// is the resolved cwd when it is the launch's own session worktree, and ""
// otherwise. The order is the mount order and it matters: each one narrows or
// reopens the one before.
func claudeProjectsBinds(home string, stateGrants []string, ownTree string) []PrivateBind {
	cfg := ClaudeConfigDirIn(home)
	if !filepath.IsAbs(cfg) {
		return nil
	}
	cfg = absResolve(cfg)
	if !writeGranted(stateGrants, cfg) {
		return nil
	}
	projects := filepath.Join(cfg, claudeProjectsDirName)
	out := []PrivateBind{{Src: projects, Dst: projects, ReadOnly: true}}
	if ownTree == "" {
		return out
	}
	own := filepath.Join(projects, claudeProjectDir(ownTree))
	mem := filepath.Join(own, claudeMemoryDirName)
	return append(out,
		PrivateBind{Src: own, Dst: own},
		PrivateBind{Src: mem, Dst: mem, ReadOnly: true},
	)
}

// claudeOwnSessionTree is cwd, resolved, when it is a session worktree this
// App would have made: a linked git worktree at exactly
// <WorktreeRoot>/<repo>/<session> (SessionTreePath). "" for anything else,
// and then the seat's projects/ is read-only whole. A main checkout is not a
// linked worktree, and its key is the one the operator's own sessions use.
func (a *App) claudeOwnSessionTree(cwd string) string {
	root, err := a.WorktreeRoot()
	if err != nil || cwd == "" {
		return ""
	}
	real := absResolve(cwd)
	rel, err := filepath.Rel(absResolve(root), real)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	if parts := strings.Split(filepath.ToSlash(rel), "/"); len(parts) != 2 {
		return ""
	}
	if LinkedGitDirs(real) == nil {
		return ""
	}
	return real
}
