# A walled seat's objects live in its own store (ranger-base-21i98)

*2026-11-15 · gwart · ADR 0099 D1, design ranger-base-dvw0w*

Builds on:

- ranger-base-pe5g5 (every walled seat commits on a detached HEAD)
- ranger-base-xcucy (the re-hashing import and the landing gate)

## What this closes

A committing seat at L3 (bwrap) and L4 (container) was granted the shared
`<common>/objects` read-write. git trusts an object file by its name and never
re-hashes on read, so a seat could overwrite the file of an object `main`
already names. Every uncaged reader would then serve the seat's bytes as
history. That grant is gone. The seat's `<common>/objects` is now its own store,
`<own worktree git dir>/seat-objects`. The shared store is read-only at
`<own worktree git dir>/seat-objects-base`, and the store's `info/alternates`
names that path.

## What changed

- `internal/posse/seatstore.go` (new) holds the layout, the launcher's
  preparation (`prepareSeatStore`), the L3 binds (`seatStoreBinds`) and the L4
  mounts (`seatStoreCageMounts`).
- `PrepareSessionHead` (`worktree.go`) makes the store and rewrites its
  alternates on every walled launch whose mechanism can bind: the container
  tier, and the seatbelt tier where bwrap renders it.
- `bwrapRender` (`bwrap.go`) adds the two binds as private binds. They render
  after every write grant (BwrapArgs step 2c).
- `CageMounts` (`cage.go`) replaces the `<common>/objects` overlay with the
  store's bind and adds the side path read-only. The old residual comment there
  said "inert until a ref names it". That was wrong for an object that already
  exists, and it is replaced.
- `docs/adr/0099-a-walled-seat-writes-objects-into-its-own-store.md` is the
  architect's ADR, transcribed, with an "As built" section.

## Pins

- `bwrapseatstorelive_test.go`, live bwrap and arm 1:
  - Refuses rewriting a shared object: unlink, chmod, rename over and in-place
    rewrite are each refused, by the real name and by the side path. It holds
    as rendered and with `<common>` granted whole. A new object goes to the
    seat's store, a commit works, and history reads. The control is the
    pre-store render, where the same unlink succeeds.
  - Honest work lands unchanged.
  - A tampered store lands nothing. MEASURED: `index-pack --strict` refuses the
    whole pack, because the tree that names the claimed blob is in the same
    pack.
  - Operator `gc` fails over a staged blob and passes right after
    `importAllSeatStores`.
- `seatstore_qa_test.go`:
  - the L4 mount list, and its deepest-mount rule under a whole-`<common>`
    mount;
  - the `bwrapRender` wiring;
  - the alternates rewrite;
  - no link followed out of the store at the store, `info`, or
    `info/alternates`.
- `cmd/testparallel`: the three live rows read the bwrap nest gate, as their
  siblings do.

Mutants, each run one at a time, with the file restored and compared after
each. Every one turned at least one pin red:

- no store bind at L3
- no side bind at L3
- no store mount at L4
- `CageMounts` not calling it
- `bwrapRender` not appending the binds
- `PrepareSessionHead` not making the store

## Not closed here

- ranger-base-hxens
