package posse

// THE SEAT STORE AT darwin's SEATBELT (ranger-base-5blmt, design
// ranger-base-hxens; the darwin half of ranger-base-21i98).
//
// At L3 and L4 a committing seat's own object store is MOUNTED over
// `<common>/objects` (seatstore.go), so the shared store is out of reach by
// construction and no environment variable is involved. sandbox-exec cannot
// remap a path, so until this file the darwin tier granted
// `<common>/objects` WHOLE: a seat could overwrite, unlink or chmod any
// object of the shared store, and every other seat and the operator read
// those bytes as history (ADR 0099 D7's stated residual, ranger-base-dvw0w
// D5).
//
// Two halves, and they only work together:
//
//  1. THE WALL (seatbeltSets). The `objects` grant is taken out of the
//     writable set, the seat's store under its wall dir
//     (`<SeatWallDir>/seat-objects`, ADR 0100 D1) is granted instead, and
//     `<common>/objects` joins the carve-out's Deny — explicitly, not left
//     to the default deny, because a PID `writable:` extra covering an
//     ancestor would otherwise reopen it, and being in Deny is what makes
//     renameSeal seal every renamable ancestor of it. The store's
//     info/alternates names the REAL `<common>/objects` here (there is no
//     side path; prepareSeatStore).
//  2. THE ENV (renderSeatObjectEnv, the L1 git shim). The launcher hands the
//     seat POSSE_SEAT_OBJECTS and POSSE_SEAT_COMMON (seatStoreEnv); the shim
//     exports GIT_OBJECT_DIRECTORY=<store> and
//     GIT_ALTERNATE_OBJECT_DIRECTORIES=<common>/objects ONLY when the git it
//     is about to run targets this seat's repo — resolved the way git
//     resolves it — so bd's commits in the store of record, and any
//     `git -C <other>`, write their own store as before.
//
// FAIL-CLOSED BY CONSTRUCTION. The deny is what holds. A git that bypasses
// the shim (an absolute /usr/bin/git, an emptied PATH) or a shim that
// misresolves gets no env, and its object write is REFUSED under the deny —
// it never lands in the shared store. Wall without env is a seat that cannot
// commit; env without wall is no protection; so the two ship in one change.
//
// MEASURED on hxens (macOS 26.7.1, Apple git 2.50.1, operator's run, output
// on that bead): a no-env object write in the seat is refused; a detached
// commit through the scoped env lands in the seat store with the shared
// store's file count unchanged and fsck --connectivity-only clean;
// overwrite, unlink, chmod, rename, hardlink, create and mkdir under the
// denied dir are refused, and so is a hardlink of a shared object INTO a
// writable dir (seatbelt checks link(2)'s source path) and a rename of the
// objects dir itself. The env reaches a hook of the seat's commit — the same
// set as GIT_DIR (`git rev-parse --local-env-vars`), which git already
// exports to hooks, so a hook that touches another repo must shed that set
// today; nothing here strips it.
//
// What the seat writes reaches the shared store only through the re-hashing
// import (seatimport.go), which is mechanism-neutral and now finds a store on
// darwin too.

import (
	"os"
	"path/filepath"
)

// The two variables the launcher hands a darwin seat, read by the L1 git
// shim and by nothing else. Not GIT_OBJECT_DIRECTORY itself: that would reach
// every git in the seat, bd's commits in the store of record included, and
// write their objects into this seat's store.
const (
	seatObjectsEnvVar = "POSSE_SEAT_OBJECTS"
	seatCommonEnvVar  = "POSSE_SEAT_COMMON"
)

// seatbeltSeatStore is the store layout a seatbelt seat in cwd commits into,
// and whether this seat gets the wall at all: a linked worktree, and a PID
// that is granted a git dir of its session (ADR 0086 D3 — a PID that denies
// the whole tree has no `objects` grant to swap). Store is "" when no home
// can be named; the swap still takes the grant away and denies the shared
// store, which is the fail-closed answer (seatStoreNoWall).
func seatbeltSeatStore(ag *AgentFile, home, cwd string) (seatStoreLayout, bool) {
	if cwd == "" || (ag != nil && deniesFileWrite(ag.Deny)) {
		return seatStoreLayout{}, false
	}
	return seatStoreForHome(home, cwd)
}

// seatbeltObjectsSwap is half 1's writable set: every entry that IS the
// shared store dropped, and the seat's own store appended (LAST, as
// bwrapGitSwap appends what its mechanism added, so a reader comparing the
// two walls finds it where it was added). An entry that merely COVERS the
// shared store — a PID extra over an ancestor — is left alone; the deny below
// it is what takes the store back out of that grant.
func seatbeltObjectsSwap(writable []string, l seatStoreLayout) []string {
	var out []string
	for _, w := range writable {
		if absResolve(w) == l.Objects {
			continue
		}
		out = append(out, w)
	}
	if l.Store != "" {
		out = append(out, absResolve(l.Store))
	}
	return dedupeStrings(out)
}

// seatbeltObjectsDeny is half 1's carve-out: the shared store in Deny, and
// the seal recomputed over the set it now follows.
func seatbeltObjectsDeny(c *SeatbeltCarveOut, writable []string, l seatStoreLayout) {
	c.Deny = dedupeStrings(append(c.Deny, l.Objects))
	c.Seal = renameSeal(c.Deny, writable)
}

// seatbeltSets is the seatbelt mechanism's writable set and carve-out for
// one launch: SeatbeltWritable and SeatbeltCarveOut, with the seat store's
// wall applied. Every seatbelt-arm reader of the two lists asks this — the
// render, `posse gates`, and the MCP config refusal — so what is reported and
// what is enforced are one answer.
//
// SeatbeltWritable itself is unchanged, for BwrapWritable's reason in the
// other direction: bwrap reads the same producer and puts the store up by a
// mount, and narrowing the shared producer would take `objects` out of the
// bwrap set too.
func (a *App) seatbeltSets(ag *AgentFile, cwd, gatesDir string, stateDirs ...string) ([]string, SeatbeltCarveOut) {
	writable := a.SeatbeltWritable(ag, cwd, gatesDir, stateDirs...)
	home, _ := os.UserHomeDir()
	l, ok := seatbeltSeatStore(ag, home, cwd)
	if ok {
		writable = seatbeltObjectsSwap(writable, l)
	}
	carve := a.SeatbeltCarveOut(ag, cwd, gatesDir, writable, stateDirs...)
	if ok {
		seatbeltObjectsDeny(&carve, writable, l)
	}
	return writable, carve
}

// seatStoreEnv is half 2's launch env: the two variables the shim reads, for
// a seat the wall above applies to and that has a store. nil otherwise — and
// a seat with no store and the deny anyway is the fail-closed launch the
// launcher refuses before it gets here (prepareSeatStore).
func seatStoreEnv(ag *AgentFile, home, cwd string) []EnvVar {
	l, ok := seatbeltSeatStore(ag, home, cwd)
	if !ok || l.Store == "" {
		return nil
	}
	return []EnvVar{
		{seatObjectsEnvVar, absResolve(l.Store)},
		{seatCommonEnvVar, filepath.Dir(l.Objects)},
	}
}

// renderSeatObjectEnv is half 2's shim arm, emitted into every git shim with
// a resolved real binary: inert unless both launcher variables are set, which
// only a darwin seatbelt seat's launch does.
//
// THE TARGET IS RESOLVED THE WAY GIT RESOLVES IT, in a subshell so nothing it
// does reaches the command: every leading `-C` applied in order (an empty
// one is git's no-op), `--git-dir <p>`/`--git-dir=<p>` and `--bare` setting
// GIT_DIR as git's own option parser does (the last one wins, and a relative
// value is read against the final directory), GIT_DIR and GIT_COMMON_DIR
// from the env as git reads them — and then the real git is ASKED for its
// common dir, resolved, and compared with the seat's. Anything git cannot
// answer is "not ours": no env, and the deny decides.
//
// THE WALK IS A FUNCTION AND THE SUBSHELL ONLY CALLS IT (ranger-base-i36bf).
// It was written inline, `posse_so_c=$(… case "$1" in -C) … esac …)`, and
// that is a SYNTAX ERROR under darwin's /bin/sh: bash 3.2 scans `$( )` for
// its closing paren while tracking quote state, so the first case pattern's
// `)` ends the substitution and the `;;` after it is left outside any case.
// MEASURED macos-latest, Darwin 25.6.0 arm64, bash 3.2.57, CI run
// 44927635225: `syntax error near unexpected token ';;'` at the `-C)` line of
// the rendered shim, so NOTHING in the shim parsed and every git through a
// darwin seat's gates PATH exited 2. Green at edfe7a79, red at 9193d3b1
// (ranger-base-5blmt) and every macos arm-3 leg between.
//
// The function is the fix rather than the POSIX-optional leading paren
// (`(-C)`, which renderHookSkipGlobalScope uses and which parses — the same
// shim carried it green on darwin through edfe7a79): a function body is
// parsed at the top level, where this hazard does not exist, so an arm added
// later cannot reintroduce it, and it is what the other five argv walks in
// this shim already do (posse_verb_match, posse_hs_readverb, posse_hs_am,
// posse_cw_hit, posse_seq_scan). This arm was the one that inlined it.
// `exit` becomes `return`: the function still runs inside the substitution's
// subshell, so a refusal still yields empty output and no env — and the cd
// still reaches nothing but that subshell.
//
// `clone` and `init` are never ours, whatever the cwd: their target is the
// repository they create, and GIT_OBJECT_DIRECTORY would put that new repo's
// objects in this seat's store.
//
// Placed after the PID's deny rules and the alias arm, so a refused command
// is not handed to git for this, and before the hook-skip env arm's last
// exec, the sequencer audit and the final exec, all of which run git for the
// command's own effect and need the env.
func renderSeatObjectEnv(real string) string {
	if real == "" {
		return ""
	}
	return "# The seat's own object store (ranger-base-5blmt): set only for this seat's repo.\n" +
		"if [ -n \"${" + seatObjectsEnvVar + ":-}\" ] && [ -n \"${" + seatCommonEnvVar + ":-}\" ]; then\n" +
		// The walk is a function, not the body of the substitution below:
		// darwin's bash 3.2 would end the substitution at the first case
		// pattern's `)` (ranger-base-i36bf, the block above).
		"  posse_so_target() {\n" +
		"    unset CDPATH # a CDPATH hit makes cd print, and this call's output is the answer\n" +
		"    posse_so_gd=${GIT_DIR:-}\n" +
		"    while [ $# -gt 0 ]; do\n" +
		"      case \"$1\" in\n" +
		"        -C) [ $# -ge 2 ] || return 1; if [ -n \"$2\" ]; then cd \"$2\" 2>/dev/null || return 1; fi; shift 2 ;;\n" +
		"        --git-dir) [ $# -ge 2 ] || return 1; posse_so_gd=$2; shift 2 ;;\n" +
		"        --git-dir=*) posse_so_gd=${1#--git-dir=}; shift ;;\n" +
		"        --bare) posse_so_gd=$(pwd); shift ;;\n" +
		"        -c|--work-tree|--namespace|--super-prefix|--config-env|--attr-source) [ $# -ge 2 ] || return 1; shift 2 ;;\n" +
		"        -*) shift ;;\n" +
		"        clone|init) return 1 ;;\n" +
		"        *) break ;;\n" +
		"      esac\n" +
		"    done\n" +
		"    if [ -n \"$posse_so_gd\" ]; then GIT_DIR=$posse_so_gd; export GIT_DIR; fi\n" +
		"    posse_so_r=$(" + shQuote(real) + " rev-parse --path-format=absolute --git-common-dir 2>/dev/null) || return 1\n" +
		"    [ -n \"$posse_so_r\" ] && cd \"$posse_so_r\" 2>/dev/null && pwd -P\n" +
		"  }\n" +
		// The subshell is still what contains the cd and the GIT_DIR export:
		// the function is called inside it and nowhere else.
		"  posse_so_c=$(posse_so_target \"$@\")\n" +
		"  if [ -n \"$posse_so_c\" ] && [ \"$posse_so_c\" = \"$" + seatCommonEnvVar + "\" ]; then\n" +
		"    GIT_OBJECT_DIRECTORY=$" + seatObjectsEnvVar + "\n" +
		"    GIT_ALTERNATE_OBJECT_DIRECTORIES=$" + seatCommonEnvVar + "/objects\n" +
		"    export GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES\n" +
		"  fi\n" +
		"fi\n"
}
