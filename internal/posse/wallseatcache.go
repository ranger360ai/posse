package posse

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

// The shared caches leave the caged write set (ranger-base-olr0g, the residual
// of ranger-base-6akzw / y438o).
//
// `~/.cache` and `~/.npm` were granted whole, because every seat builds and
// installs through them. Both hold content a LATER process executes without
// re-checking it:
//
//   - Go's build cache. An entry is keyed by the hash of its INPUTS, and the
//     go command trusts the output it finds there. MEASURED 2026-11-11 on the
//     D1 host (go1.27.3): an archive compiled from a `lib.Msg() = "evil"`
//     source, copied over the cached output of the `"good"` source, makes a
//     plain rebuild of the unchanged good source print `evil`. The operator's
//     builds of posse itself run through `~/.cache/go-build`, so a seat that
//     could write there could put code into the launcher.
//   - npm's cache, which npx runs packages out of; `~/.cache/beads/wasm`,
//     bd's compiled wasm; `~/.cache/mise`, the installer downloads mise
//     unpacks into XDG_DATA_HOME. Everything under the XDG cache dir is in
//     that class unless named otherwise, so the grant goes and named members
//     come back, rather than the other way round.
//
// What a seat gets instead is a cache of its OWN, one directory per session
// dir, that only that session's wall grants and only that session's tools
// are pointed at (GOCACHE, npm_config_cache). A seat can poison it and the
// only consumer is itself. The price is a cold first build per seat:
// MEASURED 2026-11-11, D1 host, 16 cores, load 2-5, this repo — `go build
// ./...` 7.2s cold against 0.6s warm, arm 1's test compile 8.4s, arms 2 and
// 3 another 4.9s and 6.1s; about 27s once per seat, 335M and 2.4k files.
// A layered cache (read the operator's, write the seat's) would save that
// and needs GOCACHEPROG and a program of posse's own; 27s did not buy it.
//
// `~/.cache/posse` stays writable: it is the box's lock dir (suite slots,
// ADR 0073's bd write locks), and both fail OPEN when they cannot be opened —
// an unqueued suite, an unlocked bd write. Its `plan-usage` member is carved
// read-only over that grant: it is a snapshot the dispatcher's usage guard
// trusts, and its readers already fail quiet on an unwritable dir
// (plancache.go).

// seatCacheEnvVar names the per-seat cache dir inside the wall, so a script
// that wants scratch space off /tmp's inode budget (test-times.sh's advice)
// can find a place the wall lets it write.
const seatCacheEnvVar = "POSSE_SEAT_CACHE"

// seatCacheRootRel is the per-seat caches' parent under the home. A sibling
// of `.cache/posse` and not a child of it: `.cache/posse` is granted to every
// seat, so a per-seat dir under it would be writable by every seat.
const seatCacheRootRel = ".cache/posse-seat"

// SeatCacheDir is the cache dir a walled session in cwd builds through:
// `<home>/.cache/posse-seat/<base of cwd>-<12 hex of its resolved path>`.
// The base is for a reader; the hash is what makes it one per session dir
// and not one per name, since two repos can hold a worktree of the same
// name. "" with no home or no session dir, which grants nothing.
func SeatCacheDir(home, cwd string) string {
	if home == "" || cwd == "" {
		return ""
	}
	return filepath.Join(home, seatCacheRootRel, seatDirKey(cwd))
}

// seatDirKey is the per-session-dir name both of this box's per-seat
// directories are built from — the seat CACHE above and the seat WALL below.
// One function, so the two cannot drift: the wall dir's key is the cache
// dir's key, which is what lets a reader pair them by eye and what
// RemoveSessionTree relies on when it names both from one session tree.
func seatDirKey(cwd string) string {
	real := absResolve(cwd)
	sum := sha256.Sum256([]byte(real))
	return filepath.Base(real) + "-" + hex.EncodeToString(sum[:])[:12]
}

// ─── THE SEAT'S WALL DIR (ADR 0100 D1) ──────────────────────────────────────

// seatWallRootRel is the per-seat WALL dirs' parent under the home, and
// NOTHING GRANTS IT — that is the whole of what the directory is for, and
// ADR 0100 D2(a) is the refusal that keeps it true.
//
// A SIBLING of `.cache/posse-seat` and `.cache/posse`, not a child of
// either, for the reason seatCacheRootRel gives one line up and then one
// turn further: the seat cache is granted to the seat that owns it, so a
// bind SOURCE under it is a path that seat can unlink and replace with a
// link between this render's component walk and bwrap's own resolution at
// mount time (ADR 0100 Context). A source under the wall root has no such
// component: every one of them lies under the read-only `/` of the seat's
// namespace, and the seat reaches the CONTENTS through the mount at Dst
// instead.
const seatWallRootRel = ".cache/posse-wall"

// SeatWallRoot is that parent for one home, composed from the RESOLVED home
// (ADR 0100 D2(a)): the grant a render is judged against is resolved too, so
// comparing the two spellings is comparing what the bind would expose. ""
// with no home, which names nothing and refuses nothing.
func SeatWallRoot(home string) string {
	if home == "" {
		return ""
	}
	return filepath.Join(absResolve(home), seatWallRootRel)
}

// SeatWallDir is the one directory every bwrap bind SOURCE that differs from
// its destination lies under (ADR 0100 D1): `<SeatWallRoot>/<the SeatCacheDir
// key>`, one per session dir. Its members are the seat's private /tmp
// (SeatTmpDir) and claude's private session state (claudePrivateStateDir);
// ADR 0100 D1's remaining member, the seat's own object store, joins them
// in a later slice.
//
// "" with no home or no session dir, the same answer and for the same reason
// as SeatCacheDir: a render that cannot name the dir binds nothing from it.
func SeatWallDir(home, cwd string) string {
	if home == "" || cwd == "" {
		return ""
	}
	return filepath.Join(SeatWallRoot(home), seatDirKey(cwd))
}

// bwrapTmpMount is the directory the bwrap wall replaces with the seat's own
// (ranger-base-7wdb7). Linux only, where /tmp is not a symlink.
const bwrapTmpMount = "/tmp"

// SeatTmpDir is the host directory a walled session sees as /tmp: `tmp` under
// its SeatWallDir, so no seat can write any component of it, and removed with
// the wall dir when the worktree goes. "" wherever SeatWallDir is "".
//
// IT LIVED UNDER THE SEAT CACHE DIR until ADR 0100 D1, and that was the
// non-race half of this class: the render checked the source with os.Stat,
// which FOLLOWS, so a seat that replaced its own `<cache>/tmp` with a link
// had the next launch bind the link's target read-write over /tmp — no
// window to hit, because the seat is granted its cache dir by name
// (MEASURED, ranger-base-p47q6). The walk the private binds already got
// (bwrapPrivateSrcStat) is this one's too now, and the source it walks has
// no seat-writable component left to walk over.
func SeatTmpDir(home, cwd string) string {
	d := SeatWallDir(home, cwd)
	if d == "" {
		return ""
	}
	return filepath.Join(d, seatWallTmpRel)
}

// seatWallTmpRel is that member's name under the wall dir. A constant
// because the render walks the source from its PARENT (bwrapSeatTmpRoot),
// and the two readings of "the tmp member" have to be one string.
const seatWallTmpRel = "tmp"

// bwrapSeatTmpRoot is the directory bwrap's render trusts every component of
// a private /tmp source ABOVE: the seat's wall dir, which is the parent
// SeatTmpDir composed it from. Derived from the source rather than carried
// beside it, because the two producers of a private /tmp — SeatTmpDir and
// landedCheckTmpDir — both join one `tmp` onto a directory the LAUNCHER
// owns, and a second field would be a second place for them to disagree.
func bwrapSeatTmpRoot(tmp string) string {
	if tmp == "" {
		return ""
	}
	return filepath.Dir(tmp)
}

// bwrapSeatTmp is SeatTmpDir as the bwrap render uses it: "" — the host's
// /tmp, as before — when the home or the session dir is itself under /tmp.
// A private /tmp over those would hide the seat's own tree and leave it
// nothing to chdir into, and a home under /tmp is in the shared directory
// already, so there is no seat on such a box for this mount to protect. No
// production launch is that shape (homes and worktrees live under the home);
// the fixtures of this package's own wall tests are.
func bwrapSeatTmp(home, cwd string) string {
	if home == "" || cwd == "" || underDir(bwrapTmpMount, home) || underDir(bwrapTmpMount, cwd) {
		return ""
	}
	return SeatTmpDir(home, cwd)
}

// bwrapDropTmpGrants takes out of a bwrap write set what the private /tmp
// replaces: /tmp itself, and the launcher's TMPDIR WHEREVER it lies. That
// TMPDIR is granted as a host path (SeatbeltWritable) and is one directory
// every seat on the box shares; the seat is handed TMPDIR=/tmp instead
// (bwrapRender), which is its own. Under /tmp, the grant would bind the
// shared dir back on top of the seat's /tmp; anywhere else (a TMPDIR under
// the home) it was a second shared temp dir nothing in the seat is pointed
// at any more (ranger-base-1c91x). A grant under /tmp that is NOT the
// TMPDIR is a named directory (a PID's `writable:` extra) and stays.
func bwrapDropTmpGrants(set []string, tmpdir string) []string {
	return bwrapDropTmpGrantsOver(set, bwrapTmpMount, tmpdir)
}

// bwrapDropTmpGrantsOver is that drop with the MOUNT passed in, and the one
// thing it does differently from the four lines it replaced is RESOLVE that
// mount before comparing (ranger-base-aurb8).
//
// WHY IT MATTERS, and why no linux leg could say so. Every entry of the set
// is `SeatbeltWritable`'s, and that producer's own `add` is `absResolve` —
// "the path the sandbox will match on" (ranger-base-al4n9's sentence, the
// same class one mechanism over). So the set spells the host's temp root the
// way the kernel does, and the drop key was the producer's raw `"/tmp"`. On
// linux those are one string and the rule worked. On darwin `/tmp` IS
// `/private/tmp`: `seatbeltTempGrants` names both, `add` resolves both to
// `/private/tmp`, and the raw key matched NEITHER — so the blanket temp
// grant survived a render that had just bound a private /tmp over it, and
// every deny inside it was then a deny the render thought the seat could
// create. MEASURED by reading CI run 44974829306, job 137062263947 (macos
// arm 1, b2509fba): 13 bwrap-render pins red, all on one refusal, the prep
// host fence stopping a `mkdir -p /private/tmp/posse-upgrade` on the host —
// the staging-root deny (upgradePrivateDenied), reached through exactly that
// surviving grant. The fence was right; the grant should not have been there.
//
// THE MOUNT IS A PARAMETER FOR seatbeltTempGrants' REASON — a pin has to be
// able to read the darwin answer on a box that is not darwin. "One directory
// under two spellings" needs a symlink, and this box's `/tmp` is not one, so
// the pin plants its own and names it here
// (TestQABwrapDropTmpGrantsReadsTheMountsResolvedSpelling). Nothing in
// production passes anything but `bwrapTmpMount`.
//
// BOTH KEYS ARE RESOLVED AND BOTH ARE WITNESSED, which took two arms and not
// one (ranger-base-hhps5). The pin above plants a symlinked MOUNT and hands
// it a tmpdir needing no resolution, so it cannot produce the shape that
// grades the TMPDIR's key: MEASURED 2026-11-22, with `drop[tmpdir]` raw, that
// pin and every other drop pin in the tree were GREEN. The TMPDIR's key has
// its own arm now, the same route one key over
// (TestQABwrapDropTmpGrantsReadsTheTmpdirsResolvedSpelling), and the darwin
// reading it is for is the one ranger-base-1b8yv measured: a launcher TMPDIR
// of /var/folders/…/T resolves to /private/var/folders/…/T, and
// `SeatbeltWritable`'s `add` puts the resolved spelling in the set.
//
// It is also the ONE spelling of this rule now: BwrapArgs' step 1c took the
// grant the private /tmp replaces out of its own set with a second copy of
// the comparison, carrying the same raw literal and the same blind spot, and
// calls this with no TMPDIR instead.
func bwrapDropTmpGrantsOver(set []string, mount, tmpdir string) []string {
	drop := map[string]bool{absResolve(mount): true}
	if tmpdir != "" {
		drop[absResolve(tmpdir)] = true
	}
	var out []string
	for _, p := range set {
		if !drop[absResolve(p)] {
			out = append(out, p)
		}
	}
	return out
}

// seatCacheEnv is what points a seat's tools at its own cache, in the order
// the wall renders it. GOCACHE and npm_config_cache are the two the bead
// named and the two a seat on this box was measured writing; the rest of
// what used to be written under `~/.cache` (claude's MCP logs, mise's lock
// files) was measured to work read-only.
func seatCacheEnv(dir string) []EnvVar {
	if dir == "" {
		return nil
	}
	return []EnvVar{
		{"GOCACHE", filepath.Join(dir, "go-build")},
		{"npm_config_cache", filepath.Join(dir, "npm")},
		{seatCacheEnvVar, dir},
	}
}

// seatCacheEnvPrefix is seatCacheEnv as a shell word list for a mechanism
// with no env option of its own: `/usr/bin/env K=V … `, or "" for no vars.
func seatCacheEnvPrefix(vars []EnvVar) string {
	if len(vars) == 0 {
		return ""
	}
	s := "/usr/bin/env "
	for _, v := range vars {
		s += shellQuote(v.Key+"="+v.Value) + " "
	}
	return s
}

// posseCacheReadOnly names the members of the granted `~/.cache/posse` a
// seat may read and not write, as directories.
func posseCacheReadOnly(home string) []string {
	if home == "" {
		return nil
	}
	return []string{filepath.Join(home, ".cache", "posse", "plan-usage")}
}
