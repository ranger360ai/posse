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
//
// Two more members join for ranger-base-fswhs: backups/ and file-history/,
// which are also one file or one dir per session at one uid. Nothing posse
// runs reads either one, and a seat's own copies go with its seat cache.
//
// debug/ joins for ranger-base-ntt1w, and the direction of that one is the
// other way round: the seat is the PLANTER and an uncaged claude the writer.
// MEASURED 2026-11-13 (ranger-base-gab5i, claude 2.1.323): a claude running
// with debug on writes <cfg>/debug/<session id>.txt BY PATH and follows a
// symlink there — appending to an existing target and creating a missing one
// — and it reopens the log by path on every write, so swapping the live log
// for a link mid-session redirects the rest of it. The session id is readable
// out of sessions/*.json, so nothing has to be guessed. If <cfg>/debug ITSELF
// is a link, every log goes into the link's target dir. So a caged seat that
// can write this directory gets claude-authored text appended into any
// uid-writable file outside its wall, and a file created at any path it
// names. (debug/latest is inert: claude unlinks and recreates it rather than
// following it.)
//
// Same answer, same shape: a private copy per seat. The seat's own claude
// writes and reads its own debug/ and no other seat's, and the name an
// uncaged claude opens is one no seat can reach. The render already refuses a
// link at the member's name (BwrapArgs step 2c lstats every Dst), which is
// what keeps the bind off a mount point another seat chose.
//
// Nothing posse runs reads <cfg>/debug, so a private copy costs the box
// nothing.
//
// sessions/ joins for ranger-base-lx5jk. It is claude's peer registry: one
// <pid>.json row per live session, carrying the socket other sessions connect
// to, beside <pid>.<hash>.key peer tokens. Two readers trust it, and both are
// what this protects. EVERY claude on the box reads it for peer discovery
// (ListAgents/SendMessage) and connects where a row points, an uncaged one
// included. And posse's own rex join (runtimesession.go) reads a row's
// sessionId, which feeds sentline's echo verdict. A private copy means the
// seat registers into its own dir, sees no peer's row, and cannot write one.
//
// Unlike the members above, posse DOES read this one, so the bind has a
// reader-side half: claudeSessionOfPID reads the seat's private row (found
// through /proc/<pid>/cwd) before the shared one. A caged claude survives the
// bind with its own registration intact (MEASURED, ranger-base-tii0u).
var claudeSessionStateDirNames = []string{"session-env", "shell-snapshots", "backups", "file-history", "debug", claudeSessionsDir}

// claudeDebugDirName is the member above whose carve the two mechanisms spell
// differently: bwrap gives the seat a private copy (it is in the list above),
// seatbelt has no bind and write-denies the directory instead.
const claudeDebugDirName = "debug"

// claudeDebugDir is <cfg>/debug for a seat on this box, under the same guard
// as claudePrivateSessionState: "" unless the launching runtime's own state
// grant reaches claude's config dir, since a directory nothing granted needs
// no deny.
//
// SeatbeltCarveOut adds it to Deny, which is the SEATBELT reading: a profile
// cannot bind, and "deny every name but this session's own" cannot be written
// ahead of a random session id. bwrapRender takes it back out again, because
// the deny step's read-only bind FROM THE HOST would land on top of the
// private bind and put the shared directory back — the same two-step
// codexEphemeralDirs makes (ranger-base-6ob20).
//
// COST at seatbelt: the seat's own claude loses its debug log. MEASURED
// 2026-11-13 on this Linux host, claude 2.1.323, with <cfg>/debug chmod 0555
// (EACCES, the refusal SBPL gives) — both over a config dir whose debug/
// already held a log and over a fresh one: a full turn with debug on exits 0
// with its answer, writes no log and no `latest`, and says nothing about it
// on stderr. The failure is swallowed, so no --debug-file reroute is needed.
// ASSUMED that SBPL's refusal is swallowed the same way, on the same ground
// ranger-base-wj93u assumes it for history.jsonl: the host is Linux and the
// code path is the one claude takes for any errno here.
func claudeDebugDir(home string, stateGrants []string) string {
	if home == "" {
		return ""
	}
	cfg := ClaudeConfigDirIn(home)
	if !filepath.IsAbs(cfg) {
		return ""
	}
	cfg = absResolve(cfg)
	if !writeGranted(stateGrants, cfg) {
		return ""
	}
	return filepath.Join(cfg, claudeDebugDirName)
}

// claudeSessionsRegistry is the seatbelt reading of the sessions/ member
// (ranger-base-yaha3, the darwin half of ranger-base-lx5jk): <cfg>/sessions,
// plus its resolved spelling when that differs (a link there is matched by
// SBPL at its target), under claudeDebugDir's guard. SeatbeltCarveOut puts
// these in DenyAll, which denies READ as well as write: a profile cannot
// bind a private copy, and the read half also keeps a seat off its peers'
// .key tokens, which on darwin no runtime-dir mask is known to hide.
//
// COST at seatbelt: the seat's own claude cannot register, so no peer can
// discover it, and posse's rex join has no row to read. The join is already
// off on darwin (procStartOf answers "not available" off linux), so that
// half costs nothing. MEASURED 2026-11-13 on Linux, claude 2.1.323, with
// sessions/ unreadable and unwritable (EACCES): the turn completes, exit 0
// (ranger-base-tii0u). ASSUMED, no darwin box: SBPL's refusal is handled the
// same way, and a subpath deny behaves as it does for projects/
// (ranger-base-r2uxm).
//
// bwrapRender clears DenyAll; its spelling of the same member is the private
// bind claudeSessionStateDirNames already gives it.
func claudeSessionsRegistry(home string, stateGrants []string) []string {
	dbg := claudeDebugDir(home, stateGrants)
	if dbg == "" {
		return nil
	}
	p := filepath.Join(filepath.Dir(dbg), claudeSessionsDir)
	out := []string{p}
	if real := absResolve(p); real != p {
		out = append(out, real)
	}
	return out
}

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
