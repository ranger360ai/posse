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
	real := absResolve(cwd)
	sum := sha256.Sum256([]byte(real))
	return filepath.Join(home, seatCacheRootRel, filepath.Base(real)+"-"+hex.EncodeToString(sum[:])[:12])
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
