# The shared caches leave the caged write set; each seat builds through its own (ranger-base-olr0g)

*2026-11-11 · gilfoyle · the residual of ranger-base-6akzw (y438o's last row)
· touches `homeCacheGrants`, the write set and the carve at both mechanisms*

## Severity: P1, as filed

`~/.cache` and `~/.npm` were granted whole to every walled seat. Both hold
content a later process executes without re-checking it, and the operator's
own builds of posse read the same Go build cache. So a seat could put code
into the launcher binary.

MEASURED 2026-11-11, D1 host, go1.27.3, in a scratch `GOCACHE`: build a
`lib.Msg() = "good"` program, compile a `"evil"` twin into another cache,
copy the twin's archive over the cached output for `good` (same size, so the
index entry still matches), then rebuild the **unchanged** good source. It
prints `evil`. The go command trusts the outputs it finds in the cache and
does not hash them again.

`~/.cache` holds more of the same: `beads/wasm` (bd's compiled wasm),
`mise/downloads-cas` (the installers mise unpacks into XDG_DATA_HOME),
`sigstore-rust` (the trust root mise checks those against). Whatever lands
there next joins the class, so the fix takes the grant away and adds back
named members. It does not try to carve out each bad member.

## What changed

| was writable | now | where |
|---|---|---|
| `~/.cache` whole, `~/.npm` whole; darwin also `~/Library/Caches` | not granted | `homeCacheGrants` |
| — | `~/.cache/posse` (suite slots, bd write locks) | `homeCacheGrants` |
| `~/.cache/posse/plan-usage` | read-only over that grant | `SeatbeltCarveOut` via `posseCacheReadOnly` |
| — | the seat's own cache, `~/.cache/posse-seat/<base>-<12 hex of the session dir>` | `SeatbeltWritable` via `SeatCacheDir` |
| — | `GOCACHE`, `npm_config_cache`, `POSSE_SEAT_CACHE` pointed at it | bwrap: `--setenv` in the argv; sandbox-exec: `/usr/bin/env K=V` before the prefix |
| — | the cache is removed when the worktree is retired | `RemoveSessionTree` |

`~/.cache/posse` stays writable because both of its locks fail **open** when
they cannot be opened. A suite run goes unqueued, and a bd write goes
unlocked, which is ADR 0073's lost-write class back again.

## The cost, measured before choosing

A per-seat cache means one cold build per seat. On this repo:

| D1 host, 16 cores | cold | warm |
|---|---|---|
| `go build ./...` (load 2) | 7.2s | 0.6s |
| arm 1 test compile, `-run '^$' ./...` | 8.4s | 0.6s |
| arm 2 / arm 3 compile, `./internal/posse` | 4.9s / 6.1s | — |
| `go build ./...` under the new shape (load 26) | 10.6s | 0.6s |

That is roughly 27s once per seat, plus 215-335M and about 2.4k files on
/home, not on /tmp. The other design was a layered cache: the seat reads the
operator's cache and writes its own. That needs `GOCACHEPROG` and a program
posse would ship. 27s per seat is not worth that.

## What else a seat wrote under ~/.cache, and works without

MEASURED 2026-11-11, real tools under bwrap with `~/.cache` and `~/.npm`
read-only and the new shape otherwise:

- `claude -p`: exits 0 with its answer. It does not need its MCP-log dir
  (`claude-cli-nodejs`).
- the mise shims (`node`): run. mise's `lockfiles/` write is not needed.
- `bd list`: works. Writing `beads/wasm` is refused.
- `npm cache verify`: fails with EROFS on the shared `~/.npm`, and is fine on
  the seat's own cache.
- `scripts/suite-lock.sh --status`: the slots read UNOPENABLE without the
  `.cache/posse` grant, and normally with it.
- `make fmt-check` and a focused `go test`: green.

## Pins

`internal/posse/wallseatcache_qa_test.go`:

- The render pins cover both GOOS lists, the write set and carve, one cache
  per session dir, and the env at both mechanisms.
- The live pin hands posse's render to the real bwrap. It tries 10 plants
  (Go cache, npx, cacache, mise downloads, bd wasm, a new cache dir,
  plan-usage, another seat's cache, and a write through `GOCACHE` pointed back
  at the shared cache) and 6 writes the seat needs.
- The pin is green with the fix. With `.cache` and `.npm` put back in the
  Linux list, 9 of the 10 plants are ALLOWED (plan-usage stays held by the
  carve).

## Not closed here

- **darwin is ASSUMED.** `Library/Caches` goes for the Linux reason, and its
  env prefix is pinned by render only. This box has no darwin seat to run.
- **`/tmp` is the host's and shared.** bwrap does not `--tmpfs /tmp`, by
  ADR 0080 D17. An uncaged `go build` compiles and links in
  `/tmp/go-build*` (`$WORK`, mode 0700, but the same uid as every seat), so a
  seat that wins a race there can plant into an operator build. This is the
  same class over a different directory, and a separate design:
  ranger-base-7wdb7.
- The `~/.cache/posse` lock files carry an informational argv stamp that
  `--status` prints. A seat can write it. That is integrity of a status line,
  not execution.
