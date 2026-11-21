# ADR 0099 — A walled seat writes objects into its own store, and they enter the shared store only by a re-hashing import

*Status: accepted 2026-11-14 (architect sign-off ranger-base-dvw0w) · owner:
architect · supersedes the `objects` line of the grant sets in ADR 0014 §4,
ADR 0059 and ADR 0086; their other lines stand · fix: ranger-base-21i98,
ranger-base-pe5g5, ranger-base-xcucy*

## Context

git names an object by the hash of its bytes, but it never re-hashes on read:
an existing object's file is trusted by its name. A committing seat was
granted `<common>/objects` read-write, so it could overwrite the file of an
object `main` already names, and every uncaged reader (checkout, worktree add,
cat-file, the land check, the operator) would then serve the seat's bytes as
history. The builders already re-hash through `scripts/clean-build.sh`
(ranger-base-la61c); nothing else did. No create-only grant exists: git's own
object write unlinks a tmp file and renames it onto the final name inside
`objects/`, and neither bwrap nor Landlock distinguishes new names from
existing ones (MEASURED gilfoyle, 2026-11-14, git 2.56.0, linux 7.2.15).

## Decision

**D1 Seat store.** At L3 (bwrap) and L4 (container), a committing seat's
`<common>/objects` is its own store, `<own worktree git dir>/seat-objects`,
bound read-write over that name. The real store is bound read-only at a side
path, which the seat store's `info/alternates` names. No environment variable
is used, because GIT_OBJECT_DIRECTORY would leak into bd's commits in another
repo. The bind must be the deepest mount over `<common>/objects` even when
another grant (for example a store of record inside the same repo) covers
`<common>` whole.
*Scope, confirmed 2026-11-20 (ranger-base-kfrpv, ranger-base-gdgbb):* D1
covers the SESSION's repo only, and that scope is complete. It does not
leave out the store of record's repo. Since ranger-base-kfrpv, no seat is
granted any git dir there, so nothing writes objects there and there is no
store to put over them. The launcher commits the queue (ADR 0015 §4). Do
not reopen this as "D1 needs a store-repo arm". If a seat ever needs write
access into that repo's git, that grant is what to question first. The
store-repo half of the rejection of GIT_OBJECT_DIRECTORY above ("bd's
commits in another repo") has the same false premise. The rejection still
stands, on the same leak without the commit: git honours the variable for
every repository it is run in (git(1), ENVIRONMENT VARIABLES). A seat's
read-only `git -C <store repo> log` would resolve objects in the seat store
and fail. The darwin arm (As built, below) scopes it per invocation for
exactly this reason.

**D2 Detached HEAD at every walled tier** (pe5g5). A seat never writes a
shared ref, so the branch ref pair and the create-only `refs/heads` ancestor
grants are gone. The launcher moves the branch.

**D3 The import is quarantine migration that re-hashes** (xcucy). This is the
standard shape of git's receive-pack quarantine: objects land in a side store
and migrate into the main store only after the checks pass
(git-receive-pack(1), QUARANTINE ENVIRONMENT). git can migrate without
re-hashing because index-pack already named every object it received. Here
the writer is the adversary, so migration re-hashes. Every object in the seat
store itself that common lacks is packed from a fresh, launcher-made bare view
and indexed into common with `index-pack --stdin --strict`. Forged bytes land
only under their true name, and the name they claimed stays absent. An object
common already holds is never read from the seat.

**D4 One import function and three callers:** the splice, before `branch -f`;
retire and reap, before `worktree remove/prune` (which deletes the store),
refusing the removal if the import fails; and the launcher's watch pass, once
per live seat store.

**D5 Landing gate.** After the import, `rev-list --objects --missing=print
<tip> --not main` in common prints nothing, or the landing refuses.

**D6 Readers before the import.** SHA-only reads (rev-parse, symbolic-ref)
work. Object reads (log, merge-base --is-ancestor, branch -f) exit 128. A
reader of objects runs after an import in the same pass, or it treats 128 as
unknown and fails toward keeping work: never "landed", "empty" or "safe to
remove".

**D7 darwin.** D1 is the L3 and L4 wall. D2, D3 (a no-op without a store) and
D5 apply on every tier.

*Amended by ranger-base-5blmt (design ranger-base-hxens).* darwin's
sandbox-exec cannot bind, so it gets the same store by a grant and a deny
instead of a mount. The seatbelt writable set drops `<common>/objects` and
grants `<SeatWallDir>/seat-objects`, whose `info/alternates` names the real
`<common>/objects` (no side path). `<common>/objects` is in the carve-out's
Deny explicitly, so a PID `writable:` extra over an ancestor cannot reopen it
and the rename seal covers that ancestor. The L1 git shim exports
`GIT_OBJECT_DIRECTORY` and `GIT_ALTERNATE_OBJECT_DIRECTORIES` only when the
git it runs resolves to the seat's own common dir. The deny is what holds: a
git that skips the shim gets no env, and its object write is refused rather
than landing in the shared store. D3's import finds the store on this tier
too. `internal/posse/seatbeltseatstore.go` has the measurement.

## As built (ranger-base-21i98)

- The layout is one function, `seatStoreFor` / `seatStoreFrom`
  (`internal/posse/seatstore.go`). The side path is
  `<own worktree git dir>/seat-objects-base`, a sibling of the store and not
  inside it, so the import's listing of the store never meets it.
- The launcher makes both directories and (re)writes the store's
  `info/alternates` on every walled launch whose mechanism can bind
  (`prepareSeatStore`, called from `PrepareSessionHead`). It writes through an
  `os.Root` opened from the store's parent, and removes and recreates the file
  rather than truncating it, so a link or hard link the seat planted in its
  store is never written through.
- L3: `seatStoreBinds` adds two private binds to the render
  (`bwrapRender`), the side path read-only first and the store second. They
  render in BwrapArgs step 2c, after every write grant, which is what makes
  the store the deepest mount over `<common>/objects`.
- L4: `seatStoreCageMounts` replaces the `<common>/objects` overlay in place
  with the store's bind and adds the side path read-only. The engine sorts
  binds by destination depth, so the store is the deepest mount there too.
- A PID that denies the whole tree gets neither, at either tier (ADR 0086 D3,
  D8).

MEASURED 2026-11-15, git 2.56.0, bwrap, linux 7.2.15, by the live pins in
`internal/posse/bwrapseatstorelive_test.go`:

- Under the shipped render, a committing seat writes new objects (into its
  store, never the shared one), commits, and reads history through the
  alternates. An unlink, a chmod, a rename over and an in-place rewrite of an
  existing shared object are each refused, by its real name and by the side
  path, and the file's bytes and mode read back unchanged from outside. The
  same holds with `<common>` granted whole. Under the render this replaces,
  the same unlink succeeds.
- A seat that rewrites one of its own objects after committing lands nothing.
  The tree that names the claimed blob travels in the same pack, so
  `index-pack --strict` refuses the whole pack ("did not receive expected
  object"), main does not move, and neither the claimed name nor the forged
  bytes' true name reach common. An honest seat's commit lands unchanged.
- An operator `gc` fails over a blob staged in a walled seat's store, and
  succeeds right after the watch-pass import.

## Consequences

MEASURED by dvw0w, 2026-11-14, git 2.56.0, linux 7.2.15, in a scratch repo,
with the seat simulated by GIT_OBJECT_DIRECTORY pointing at a store with
alternates to common:

- One staged blob in a seat's index makes operator `git gc` exit 128 and
  `fsck` exit 2 in common.
- A seat-only HEAD also breaks uncaged `log` and `status` in the tree.
- Importing only the tip clears gc (with a warning), but fsck stays red on
  reflog-only commits.
- Importing the whole store clears all four checks. That is why D3 imports
  the whole store and why D4 runs it on the watch pass.
- Forged bytes under a predicted name and under an existing name both left
  common honest.

Residuals:

- A gc that starts between a seat's write and the next pass fails. It can be
  re-run. An auto-gc failure writes gc.log, which suppresses auto-gc for
  gc.logExpiry (1 day by default). The cadence is ASSUMED to be short enough.
  If the measured rate says otherwise, the fix is an inotify fast path on
  seat stores with the pass as the floor, never a seat store in common's
  alternates.
- A seat's own `gc --auto` walks every worktree's index, so it fails while
  another seat holds un-imported objects. This is ASSUMED harmless to the
  seat's commits (auto-gc runs detached from them); it is unmeasured.
- A hostile seat can already make the operator's gc fail by pointing its own
  worktree HEAD at a missing name. That is true before this ADR and is not
  changed by it.

Exit hatch: the store holds nothing hostage. Import everything and drop the
bind, and the layout is the one before this ADR.

## Alternatives rejected

- List seat stores in common's `info/alternates`, which keeps gc and readers
  whole. An uncaged writer's has_object check is then satisfied by a
  seat-planted object under a predicted name, so the writer skips its own
  write and history serves the seat's bytes. That reopens the hole.
- Route every uncaged reader through a land view: about 18 sites (gwart's
  census), and every new reader is one more place to forget it.
- Import only the tip, by fetching through a land view (21i98's first
  landing): sound for the tip, but measured above to leave gc broken by staged
  blobs and fsck red by reflogs. It is also a second path doing what D3 does.
- Each seat as a separate repository that borrows common through alternates
  (`clone --shared`): nothing of a seat is under `<common>`, so gc never sees
  it and needs no remap. This was the attractive option. Rejected because
  every wall over a seat's git (ADR 0038's worktree-config binds, 0052's hooks
  redirect, 0086, 0095) is built on the linked-worktree layout. A separate
  repo's own config is writable by the seat and read by uncaged git in the
  tree, so all of those walls would have to be solved again to close a gc
  window. It would also need a launcher-held base ref per seat, or common's
  prune deletes objects the seat borrows. It is the fallback if D7's residual
  cannot be closed AND the measured gc rate is bad.
- Hold the whole fix for a spike in a venue this lane does not have: holding
  for it never converges, and shipping L3 and L4 regresses nothing elsewhere.
