# ADR 0100 — A bwrap bind source lies under no write grant, and a private destination is a mount wherever it can be written

*Status: accepted 2026-11-15 (architect sign-off ranger-base-6w88m) · owner:
architect · extends ADR 0080 D17's render and ADR 0099 D1's seat store; their
other lines stand · fix: ranger-base-5f5kb (D1's claude state and private
/tmp, D2(a), D3)*

## Context

bwrap resolves every mount SOURCE at mount time and follows links (MEASURED
P1). The render's lstat walk (ranger-base-wsgus) judges the source a moment
earlier. Any process that can write a COMPONENT of the source in between can
redirect the bind. That includes a leftover of the seat itself, which needs
no timing: it can toggle the component in a loop. The census of the class,
keyed on the invariant "a source that differs from its destination lies under
a write grant", finds four members:

- claude's private session-state dirs and its private history.jsonl, under
  `<SeatCacheDir>/claude`;
- the seat /tmp, `<SeatCacheDir>/tmp`. This one is NOT a race: the render
  checked it with os.Stat, which follows links, so a planted link is mounted
  rw at /tmp (MEASURED, ranger-base-p47q6);
- the seat store, `<own git dir>/seat-objects`, bound rw at
  `<common>/objects` (ADR 0099 D1). A launch that loses the race gets the
  SHARED object store rw, which is the write ADR 0099 exists to remove; the
  rate is ASSUMED, by the same toggle argument.

The DESTINATION side is a different attack, and it is not a race. MEASURED
2026-11-15, this box (Linux 7.2.15, bubblewrap 0.13.1), by a scratch probe
quoted on ranger-base-6w88m:

- A victim namespace binds a private dir over `cfg/m`.
- A namespace that holds no mount on `cfg/m` renames it away mid-session and
  creates a new `cfg/m`. Both steps succeed.
- The victim's next open of `cfg/m/who` reads the PLANTED file. Its mount is
  still alive, but on the renamed dentry.
- A namespace that holds the same bind gets EBUSY for the same mv.

So any walled seat that can write a private destination's parent, and does
not hold that destination as a mount, can swap it at any time in the victim's
session. For claude's shell-snapshots and session-env, a win means the victim
runs sibling-written code ahead of every Bash command: a walled-to-walled
lateral move. A check at launch (bwrap's final-link refusal, or a post-mount
verify) cannot close this, because the swap can come after the check.

## Decision

**D1. NO SOURCE THAT DIFFERS FROM ITS DESTINATION LIES WHERE A WALLED SEAT
CAN WRITE A COMPONENT OF IT.** All four members move to a per-seat WALL dir,
`SeatWallDir = <resolved home>/.cache/posse-wall/<the SeatCacheDir key>/`,
under a wall ROOT that no render grants (see D2). The new paths are
`<wall>/claude/…`, `<wall>/tmp` and `<wall>/seat-objects`, and SrcRoot is
`<wall>`. The seat still reads and writes the contents through the mount at
Dst. It cannot create, rename or unlink any component of the source, because
every component is under the read-only `/`. The walk stays: Tmp goes through
the component walk (`bwrapSrcWalk`) and never through `bwrapStat` again. The
same move applies to the L4 cage's seat-store mount (seatStoreCageMounts).
The launcher writes `<wall>/origin` (the resolved tree path and the common
dir) when it creates the dir. That file is trustworthy because no seat writes
under the wall root.

NO MIGRATION, with one exception. The old `<SeatCacheDir>/claude` and
`<SeatCacheDir>/tmp` are left as inert junk. Moving them would be a rename
through seat-writable components by the uncaged launcher, which is this hole
once more. Each seat's private history, debug logs and /tmp start empty once
(ASSUMED acceptable).

THE SEAT STORE IS THE EXCEPTION, because it holds committed work:

- The first launch that renders the wall store IMPORTS the legacy
  `<own>/seat-objects` first, through openSeatStore (ADR 0099 D3's os.Root
  path, which refuses a link that leaves the parent). Before that import, the
  seat's own branch tip may be readable only there.
- Every importer has TWO arms. The wall arm finds the store from admin dir →
  tree → SeatWallDir; for orphans, it sweeps the wall root for origin files
  that name this common dir and whose tree is gone. The legacy arm is today's
  seatStoreAt path, unchanged.
- The legacy arm stays until a census reads no admin dir holding a
  seat-objects. Its removal is a later bead, not this one.
- Retire, reap and prune run in this order: import both arms, then `git
  worktree remove`/prune, then RemoveAll the wall dir. An import that fails
  keeps the wall dir and refuses the removal, as pruneWorktrees already does.

**D2. TWO INVARIANTS, checked over EVERY bwrap argv the package renders:**
bwrapRender, the landed-check judge and the wall venue.

(a) Box-wide: refuse any write grant at or above the wall ROOT. The grant
side is compared RESOLVED (absResolve), because that is what the bind will
expose. A seat could plant a link at a grant path of its own for its next
launch, and only the resolved grant shows where it points. The wall root and
every source are compared as COMPOSED, from the one resolved home. Compare
with insideDir, never underDir: underDir resolves both sides, so it would
follow a planted source link and judge its target.

(b) Per render: a path P is PINNED in render R when every component of P that
lies strictly inside a write grant of R is itself a mount destination of R.
Every source that differs from its destination must be pinned. The old Tmp,
claude and seat-store sources all fail this, which is the pin. The seat
store's side bind passes legitimately: its source `<common>/objects` is
inside a `<common>` grant but is the store's own mount.

Today the landed-check and venue sources lie under `StateDir/landed-check/
run-*`, which no walled seat writes. They pass both invariants, and the pin
says so.

**D3. PREP STAYS IN THE LAUNCHER, unchanged in shape.** Its source-side paths
now have no component a walled seat can write. Its destination-side paths are
covered by D4.

**D4. A PRIVATE DESTINATION IS A MOUNT IN EVERY WALLED NAMESPACE THAT CAN
WRITE ITS PARENT.**

(a) Per render: every destination that differs from its source is PINNED in R
(the D2(b) predicate, one function). This covers the seat's own namespace and
its leftovers. `<cfg>/<member>` passes, because the member is the mount.
`<cfg>/projects/<key>` passes, because projects is an in-place ro bind.
`<common>/objects` and `<own>/seat-objects-base` pass: their components
inside a grant are mounts, and outside a grant they sit under the ro `/`.
That answers the review's "the store's own side bind" case without an
exemption. A miss is fixed by an in-place bind of the parent, never by an
exemption: grok's `daemon/run` is ASSUMED to miss, and codex's tmp and
shell_snapshots need a census.

(b) Box-wide: THE PRIVATE SET FOLLOWS THE GRANT, NOT THE RUNTIME. Any bwrap
render whose final write set reaches a runtime config dir that has a private
table binds that table, whichever runtime the seat runs. Today the only such
table is claude's: claudeSessionStateDirNames plus history.jsonl.
claudePrivateSessionState's gate moves from the runtime's state grants to the
render's final write set. A sibling that can write `<cfg>` therefore holds
every member as a mount, and gets EBUSY.

(c) For the seat-store destination `<common>/objects`, the writers of
`<common>` are of two kinds. Linked seats of the repo hold it as their own
mount, (a). The other kind already writes the shared store by grant: an
unlinked seat in the main checkout, or a shims seat. A swap gives them
nothing they lack (ADR 0099's own scope).

(d) bwrap's final-link refusal (MEASURED P3) is check-then-use: 0.13.1 has no
move_mount, and path mount(2) follows a final link. It is kept as the refusal
a hand-made link gets. Nothing relies on it.

**SCOPE.** The guarantee is for WALLED seats (bwrap, L3/L4). A seat at `cage:
shims`, and a seat degraded from seatbelt to shims on linux, is a same-uid
crew process with no write wall. It reaches every wall dir, every
SeatCacheDir, every `<cfg>/<member>` and every gates dir. It is NOT the
operator. MEASURED 2026-11-15 from this instance's session records: 5 live
seats at shims, 3 degraded seatbelt→shims, 2 walled. That exposure predates
this record, and this record neither closes nor widens it.

## Consequences

- No launch-line change, no new entry step, no ADR 0080 D18 amendment.
- RESIDUAL, stale walls. A wall is fixed at launch. A walled seat launched
  before this landing (or before a member joined the private table) does not
  hold the new mounts. Until it relaunches it can still swap a
  `<cfg>/<member>` in a newer sibling, and its own sources still sit under
  its SeatCacheDir. The residual closes when every walled seat whose launch
  predates the landing has relaunched. Those seats can be listed from the
  launch times in the instance's session records against the landing commit.
- RESIDUAL, a home under the shared `/tmp`. D2(a) stands down there
  (`bwrapWallRoot`), for the reason the private /tmp already stands down
  (`bwrapSeatTmp`): `/tmp` is granted unconditionally, a home inside it is in
  the shared directory already, and there is no seat on such a box for the
  wall root to protect. No production launch is that shape — homes and
  worktrees live under the home — and every fixture of internal/posse is,
  which is why the pins for D2(a) place their home outside every blanket temp
  grant and would otherwise measure the stand-down.
- Every reader that composes the private paths recomposes from SeatWallDir:
  claudePrivateHistoryPath, claudePrivateSessionRowPath, the herdr backend's
  seat home, SeatTmpDir/bwrapSeatTmp, the seat store layout and the
  importers. As a side effect, the uncaged readers stop reading through
  seat-writable components. So does the launcher's own write of the store's
  info/alternates.
- Exit hatch: the wall dir holds only the seat's own state. Deleting it costs
  the D1 "start empty" bullet, plus a store that must be imported first. The
  importers do that.

## Alternatives rejected

**A1. posse holds the objects:** openat2(RESOLVE_NO_SYMLINKS|RESOLVE_BENEATH)
plus --bind-fd from an uncaged entry step. It WORKS (MEASURED P2/Q2/Q3).
Rejected: it races the writer instead of removing it, and it costs an ADR
0080 D18 amendment, an fd protocol and an *at copy of prep. It also does
nothing for the destination, which is not a launch race (Context). Revisit if
a source must live where the seat can write it by name.

**A2. Re-check just before the line is typed.** Rejected: the toggle loop
makes the width of the window irrelevant.

**A3. An in-cage post-mount verify of each destination's dev/ino.** Rejected
for BOTH arms, and for the destination arm now on a measurement and not on
cost. The destination swap is a rename that a sibling can make at any moment
of the victim's session (Context, MEASURED), so a launch-time check verifies
a state that can change a second later. It would also have to become
unconditional (wall-unix is typed only at Landlock ABI >= 6).

**A4. A read-only in-place bind of `<SeatCacheDir>/claude`** (the clever one).
Rejected: it is a mount whose only job is to guard a path that never needed a
grant.

**A5. --unshare-pid to end leftovers.** Rejected by ADR 0096, and it would
not help: the Tmp plant needs no leftover, and neither does the destination
swap.

**A6. Do not grant `<cfg>` whole:** grant only its shared members in place, so
no seat can rename a child of `<cfg>`. Rejected for now. claude writes new
top-level files and is ASSUMED to rename-replace files such as its
credentials in `<cfg>` itself. That would break every claude seat. D4(b) gets
the same EBUSY without that bet. Revisit if a census of claude's `<cfg>`
top-level writes shows none.

**A7. The review's box-wide wording** ("a render that grants a dir rw binds
every private Dst inside it that ANY current render binds"). Adopted in
substance as D4(b), but keyed on a static per-config-dir table, not on "any
current render". A render cannot read its siblings' renders, and a stale one
would make the rule undecidable at render time.

## MEASURED vs ASSUMED

MEASURED (2026-11-15, this box, Linux 7.2.15, bubblewrap 0.13.1):

- P1 a source link is followed.
- P2/Q2/Q3 --bind-fd pins a dir, a file and a renamed-away dir.
- P3 a final destination link is refused.
- Q1 an intermediate destination link is followed.
- P4/P4b a mountpoint gives EBUSY in the namespace that holds it, alias
  included.
- P4c a bind source can be renamed in-namespace.
- The Tmp link renders and mounts its target rw.
- A namespace without the mount renames a live private destination
  mid-session, the victim's name resolves to the planted dir, and a namespace
  with the mount gets EBUSY.
- The seat-store bind shape, the live seat tiers (5 shims, 3 degraded, 2
  walled), and that the gates dir is read-only in-seat.

ASSUMED:

- the seat-store toggle win rate;
- that grok's daemon/run and codex's members miss D4(a) (a later slice runs
  the census);
- that losing private history/debug/tmp once is acceptable;
- A6's premise about claude's top-level `<cfg>` writes.

## Not closed here

- ranger-base-ltmyh
- ranger-base-gn050
