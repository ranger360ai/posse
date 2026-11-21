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
// The source is under the seat's wall dir (SeatWallDir, ADR 0100 D1), so the
// worktree removal that takes the wall dir takes these too.
//
// bwrap only. A seatbelt profile cannot bind, and "deny write except the
// session's own name" cannot be written ahead of a random session id; the
// darwin side is its own bead.
//
// Two more members join for ranger-base-fswhs: backups/ and file-history/,
// which are also one file or one dir per session at one uid. Nothing posse
// runs reads either one, and a seat's own copies go with its wall dir.
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
//
// paste-cache/ joins for ranger-base-a544g, beside a FILE bind over
// history.jsonl (claudePrivateSessionState, below). history.jsonl gets one
// row per interactive submit, and a short paste rides INLINE in that row;
// paste-cache/ holds the body of a long paste a row names by hash. Two
// readers act on what they find, and both are what this protects:
//
//   - the operator's prompt recall (up-arrow, history search) in an UNCAGED
//     claude loads rows for its cwd and fills each paste from the row or the
//     cache file, re-hashing nothing. A seat that plants a row or rewrites a
//     cached paste changes what the operator submits under their own name.
//   - posse's sentline (lastSubmittedOfPID) reads the last row for a seat's
//     session as the echo-vs-hold verdict; a planted row makes a held draft
//     read as an echo, and posse types over it.
//
// A private copy means the seat appends its own rows and pastes privately,
// reads nobody else's (the operator's prompts included), and writes none.
// The file bind is a mountpoint inside the seat's namespace, so the seat
// cannot unlink or rename over it. The reader-side half is
// lastSubmittedOfPID: the private log first, found through /proc/<pid>/cwd.
// MEASURED, design ranger-base-tgc4b: claude 2.1.323 interactive, private
// file and dir binds, every turn answered and recall worked from the private
// copy. Not closed here: ranger-base-0qpsw.
//
// cache/ joins for ranger-base-7x3bj, beside a FILE bind over .last-cleanup
// (claudePrivateSessionState, below). They are the two members of the shared
// config dir that the four above left behind — named in no carve and on
// claude's own protected list nowhere (census ranger-base-drimd). MEASURED
// 2026-11-15 from a bwrap seat, claude 2.1.325: no bind covered either name,
// so both resolved to the shared host dir, a file created under cache/ landed
// there, and .last-cleanup passed W_OK. What a peer's write reaches, read out
// of the 2.1.325 binary's own JS and not exercised:
//
//   - cache/model-catalog/<account>-<hash>-cc.json is the SERVED catalog
//     cache: models, provider alias targets, per-model thinking and effort
//     defaults, state.model, min_claude_code_version. On read claude checks
//     the schema and the surface and nothing else — the file carries no
//     signature field, the signing covers the separate published catalog — so
//     a planted copy is what the next claude on the box believes about
//     models. cache/model-catalog/masked-ids.json is persisted masked model
//     ids, hydrated at every later start.
//   - .last-cleanup is read for its MTIME as a 24h throttle: within the
//     window, the background housekeeping and retention sweep is SKIPPED, and
//     the marker is only rewritten after a sweep runs. So one peer's future
//     mtime switches off box-wide transcript retention indefinitely. (A
//     symlink at the marker is not a redirect: claude treats ELOOP/EISDIR as
//     not-regular, deletes it and rewrites.)
//   - cache/my-closed-issues.json and cache/changelog.md are UI display only.
//     Low on their own, and the same carve covers them.
//
// A posse seat's --model is an exact id, so the alias route does not move a
// seat; the effort defaults, the masks and the operator's own alias use are
// UNMEASURED, and no code execution or credential exposure is known. P2
// hardening, and the same answer as the members above: a private copy per
// seat. Nothing posse reads either name back, so this one has no reader-side
// half.
//
// COSTS, both of them the carve working: the seat refetches the served
// catalog and the changelog instead of reading the box's copy, and it runs
// its own sweep against its own binds — which, with a private marker created
// empty at launch, means a fresh mtime and no sweep for the seat's life. That
// is the conservative direction: a seat's housekeeping is the suspected
// remover in ranger-base-jkpxq. The one place the carve writes the SHARED
// dir is the mount point: an absent .last-cleanup is created there as an
// empty 0600 file, whose mtime then throttles the box's next sweep by up to
// 24h, once, on a box where claude has never swept.
//
// Not closed here:
//
//	ranger-base-mbcbv
var claudeSessionStateDirNames = []string{"session-env", "shell-snapshots", "backups", "file-history", "debug", claudeSessionsDir, claudePasteCacheDir, claudeCacheDir}

// claudePasteCacheDir is claude's long-paste store, a member of the list
// above (ranger-base-a544g).
const claudePasteCacheDir = "paste-cache"

// claudeCacheDir is claude's cache of server-fetched display and catalog
// state, a member of the list above, and claudeLastCleanupFile is the
// housekeeping throttle whose private copy is a file bind beside
// history.jsonl's (ranger-base-7x3bj).
const (
	claudeCacheDir        = "cache"
	claudeLastCleanupFile = ".last-cleanup"
)

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

// claudePromptHistory is the seatbelt reading of claude's prompt history
// (ranger-base-wj93u, the darwin half of ranger-base-a544g): <cfg>/history.jsonl
// and <cfg>/paste-cache, each plus its resolved spelling when that differs,
// under claudeDebugDir's guard. SeatbeltCarveOut appends these to DenyAll.
// The process it protects is the operator's prompt recall on darwin: an
// uncaged claude replays a history row's paste body, inline or from
// paste-cache/<hash>.txt, as the operator's own submitted turn.
//
// A `(subpath …)` naming a file matches that name and nothing beside it, so
// history.jsonl rides the same subpath block as the directories: one form,
// and a superset of the literal the design named. Denying file-write* at the
// NAME refuses unlink, create and a rename onto it as well as the append, so
// a seat cannot swap in a link the operator's claude would follow. Denying
// read keeps the operator's prompts and pastes out of the seat too.
//
// COST: a caged darwin seat appends no history, so sentline reads its
// composer as a hold rather than an echo. MEASURED 2026-11-13 on Linux,
// claude 2.1.323, both names unreadable and unwritable (EACCES): every turn
// completes, the failed write is logged and dropped (ranger-base-tgc4b).
// ASSUMED, no darwin box: SBPL's refusal is handled the same way.
func claudePromptHistory(home string, stateGrants []string) []string {
	dbg := claudeDebugDir(home, stateGrants)
	if dbg == "" {
		return nil
	}
	var out []string
	for _, n := range []string{claudeHistoryFile, claudePasteCacheDir} {
		p := filepath.Join(filepath.Dir(dbg), n)
		out = append(out, p)
		if real := absResolve(p); real != p {
			out = append(out, real)
		}
	}
	return out
}

// claudePrivateSessionStateRel is where those members live under the seat's
// wall dir.
const claudePrivateSessionStateRel = "claude"

// claudePrivateStateDir is that directory for one seat: `<SeatWallDir>/claude`,
// the parent of every private source this file renders and of the two an
// uncaged posse READS back (claudePrivateHistoryPath in sentline.go,
// claudePrivateSessionRowPath in runtimesession.go). "" wherever SeatWallDir
// is "".
//
// ONE HELPER AND FOUR READERS, which is ADR 0100 D1's "recompose every
// reader from the one helper". It used to hang off SeatCacheDir, a directory
// the seat is granted by name — so every one of those readers walked
// components the seat could replace, and the render's own walk had nothing
// above the member it could trust (ranger-base-p47q6). Under the wall dir
// the seat can write none of them.
func claudePrivateStateDir(home, cwd string) string {
	wall := SeatWallDir(home, cwd)
	if wall == "" {
		return ""
	}
	return filepath.Join(wall, claudePrivateSessionStateRel)
}

// claudePrivateSessionState names the binds for the config dir a seat on this
// box would use (ClaudeConfigDirIn, the claudeExecConfig answer), and only
// when THE RENDER'S OWN WRITE SET reaches that dir — claudePrivateStateGate
// is which list the caller hands it and the one place that is not the write
// set. A launch with no seat wall dir gets nothing, and so does a render that
// cannot write `<cfg>`.
//
// THE GATE IS THE GRANT AND NOT THE RUNTIME (ADR 0100 D4(b)). It read the
// launching runtime's state grants until 2026-11-20, which made the table a
// property of WHICH CLI the seat runs; the attack it defends against is a
// property of who can WRITE the directory. A namespace that can write
// `<cfg>` and holds no mount on `<cfg>/<member>` renames that member away
// mid-session and plants its own, and the victim's claude then reads the
// planted directory while its own bind rides the renamed one (MEASURED
// 2026-11-15, ADR 0100's Context probe). For shell-snapshots and session-env
// that is code the victim sources ahead of every Bash command. So any bwrap
// render whose write set reaches `<cfg>` binds every member of the table,
// whichever runtime it launches, and gets EBUSY for the rename — the
// box-wide half of D4, where D4(a)'s per-render PINNED predicate is the
// other.
//
// WHAT THAT COSTS A NON-CLAUDE SEAT: nothing it was using. A codex or grok
// seat reaches `<cfg>` only through a PID's `writable:` extra, and the
// members here are claude's own per-session scratch — snapshots, session-env,
// debug logs, its peer registry, its prompt history. A seat granted the dir
// for some other file in it keeps that file; what it loses is the ability to
// read or write another seat's copy of these.
//
// Its SEATBELT siblings in this file (claudeDebugDir, claudeSessionsRegistry,
// claudePromptHistory) still read the runtime's state grants, and that is
// not an oversight to tidy later: D4's whole subject is the mount that gives
// EBUSY, SCOPE says the guarantee is for walled seats, and those three are
// the darwin mechanism's spelling, where there is no mount and no EBUSY to
// be had. claudeProjectsBinds keeps the same gate for the same reason — its
// binds are in place, so no destination of its own can be swapped.
func claudePrivateSessionState(home, cwd string, grants []string) []PrivateBind {
	wall, own := SeatWallDir(home, cwd), claudePrivateStateDir(home, cwd)
	if wall == "" {
		return nil
	}
	cfg := ClaudeConfigDirIn(home)
	if !filepath.IsAbs(cfg) {
		return nil
	}
	cfg = absResolve(cfg)
	if !writeGranted(grants, cfg) {
		return nil
	}
	var out []PrivateBind
	// SrcRoot IS THE SEAT WALL DIR, in the spelling Src is composed from
	// (ranger-base-wsgus, ADR 0100 D1). Everything below it — <wall>/claude
	// and each member under it — is walked: the render lstats every one of
	// those components and refuses a link at any of them. The walk is what
	// it always was; what changed is that the components are no longer in
	// the seat's write set at all, so there is no longer a writer who can
	// swap one between this walk and bwrap's own resolution at mount time.
	// The wall dir itself is the launcher's path, and no render grants its
	// parent (D2(a), bwrapRefuseWallRootGrant).
	for _, n := range claudeSessionStateDirNames {
		out = append(out, PrivateBind{
			Src:     filepath.Join(own, n),
			SrcRoot: wall,
			Dst:     filepath.Join(cfg, n),
		})
	}
	// history.jsonl is a FILE, so its private copy is a file bind; why it is
	// private at all is above claudeSessionStateDirNames. The source is the
	// path lastSubmittedOfPID reads, from the one helper.
	out = append(out, PrivateBind{
		Src:     claudePrivateHistoryPath(home, cwd),
		SrcRoot: wall,
		Dst:     filepath.Join(cfg, claudeHistoryFile),
		File:    true,
	})
	// .last-cleanup is a FILE too, and the same file bind; why it is private
	// is above claudeSessionStateDirNames. Its source is composed HERE and
	// not behind a helper of its own, which is the difference from
	// history.jsonl: nothing outside this file reads the marker back, so
	// there is no second reader to recompose from one helper (ADR 0100 D1).
	out = append(out, PrivateBind{
		Src:     filepath.Join(own, claudeLastCleanupFile),
		SrcRoot: wall,
		Dst:     filepath.Join(cfg, claudeLastCleanupFile),
		File:    true,
	})
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

// claudePrivateStateGate is which list D4(b)'s gate reads — the render's own
// write set, or, where this mechanism's wall stands down, the launching
// runtime's state grants.
//
// IT STANDS DOWN WHERE EVERY OTHER ADR 0100 RULE DOES, off the same helper
// and for the same reason (bwrapRefuseUnpinnedSource's last paragraph): on a
// box whose home is itself under the shared `/tmp` there is no ungranted
// place for a bind source to live, so nothing is pinned by anything and both
// of the ADR's refusals are disabled. There, EVERY render's write set
// reaches `<cfg>` through the blanket `/tmp` grant — so the grant-keyed gate
// would hand every seat a private table whose sources that seat can swap,
// which is a mount with no EBUSY behind it and no wall in it. The runtime's
// state grants are what the gate read before D4(b), so where the wall is not
// real the behaviour is exactly what it was.
//
// No production launch is that shape — homes and worktrees live under the
// home, and SeatWallRoot is composed from it — and every fixture in this
// package is, which is why the pins for the grant-keyed half put their home
// under wallUngrantedDir.
func claudePrivateStateGate(set, stateGrants []string) []string {
	if bwrapWallRoot() == "" {
		return stateGrants
	}
	return set
}
