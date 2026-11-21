# A darwin seat's objects live in its own store too (ranger-base-5blmt)

*2026-11-20 · dinesh · ADR 0099 D7 as amended, design ranger-base-hxens*

Builds on:

- ranger-base-21i98 (the per-seat store at L3/L4, and the import)
- ranger-base-hxens (the darwin measurement this adopts)

## The hole it closes

darwin's seatbelt tier granted a committing seat `<common>/objects` whole.
sandbox-exec cannot bind, so 21i98's mount had no equivalent there. A seat
could overwrite, unlink or chmod any object in the shared store. Every other
seat and the operator read those bytes as history, and git never re-hashes
on read.

## The fix, two halves that ship together

1. **The wall** (`seatbeltSets`, `internal/posse/seatbeltseatstore.go`). The
   seatbelt mechanism drops `<common>/objects` from the writable set, grants
   `<SeatWallDir>/seat-objects`, and puts `<common>/objects` in the
   carve-out's Deny explicitly. Being in Deny means a PID `writable:` extra
   over an ancestor cannot reopen it, and `renameSeal` seals that ancestor.
   `SeatbeltWritable` itself is unchanged, because bwrap reads it and mounts
   the store instead. The launch makes the store with no side path:
   `info/alternates` names the real `<common>/objects` (`prepareSeatStoreAt`,
   `seatStoreSideless`).
2. **The env** (`renderSeatObjectEnv`, in every git shim). The seatbelt
   prefix hands the seat `POSSE_SEAT_OBJECTS` and `POSSE_SEAT_COMMON`. The
   shim exports `GIT_OBJECT_DIRECTORY` and `GIT_ALTERNATE_OBJECT_DIRECTORIES`
   only when the git it is about to run resolves to the seat's common dir.
   It resolves the target the way git does: every `-C` in order,
   `--git-dir`, `--bare`, and `GIT_DIR` from the env, then it asks the real
   git. `clone` and `init` never get the env.

Wall without the env is a seat that cannot commit. Env without the wall is
no protection. The deny is what holds: a git that skips the shim gets no env,
and its object write is refused rather than landing in the shared store.

## Measured

- darwin, from hxens (operator's run, macOS 26.7.1, Apple git 2.50.1): the
  no-env write is refused. Overwrite, unlink, chmod, rename, hardlink, create
  and mkdir under the denied dir are refused, and so is a hardlink of a
  shared object into a writable dir. A commit through the scoped env lands
  in the seat store and fsck is clean.
- linux, 2026-11-20, git 2.56.0, no sandbox, for the shim half: with
  `GIT_OBJECT_DIRECTORY` set, `git clone` dies at `trying to write ref ...
  with nonexistent object`, and `git init` makes a repo with no objects dir.
  That is why both are carved out.
- `TestQASeatObjectEnvScopesToTheSeatsOwnRepo` runs the shim on any host.
  `TestQASeatbeltSeatStoreWallUnderTheRenderedProfile` runs (0), (1) and (2)
  under the RENDERED profile on the macos CI leg. The other darwin pins that
  commit under the shipped profile now commit as a seat does
  (`wgSeatProfile`). `TestQASeatbeltWalledSeatCommitsDetachedAndCannotWriteARef`
  also asserts that the splice's import lands the commit in the shared store,
  so the import is no longer a no-op on this tier.

## Not closed here

- A hook of the seat's own commit inherits the two variables, as it inherits
  `GIT_DIR`. A hook that commits into another repo must shed
  `git rev-parse --local-env-vars`, which it already has to do today.
  Measured on hxens, and nothing here strips them.
- The re-seal in `seatbeltObjectsDeny` is redundant today. The carve-out's
  hooks and config denies already seal every ancestor `<common>/objects` has
  (a mutation that drops it survives). It stays so the seal does not depend
  on those other denies.
