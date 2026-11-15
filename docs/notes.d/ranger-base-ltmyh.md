## The seat store moves under the wall dir, and "pinned" becomes a predicate the renders are judged by (ranger-base-ltmyh)

2026-11-15. ADR 0100 D1's seat store and D2(b), after ranger-base-5f5kb
landed the ADR and moved the other three members of the class.

### What was wrong

A walled seat's object store was `<own worktree git dir>/seat-objects`,
bound read-write over `<common>/objects` (ADR 0099 D1). bwrap resolves a
bind SOURCE at mount time and follows links, and the render's component walk
judges the same source a moment earlier — so anything that can write a
component of the source between the two instants redirects the bind. The
per-worktree git dir is in the seat's own write set, so `seat-objects` was
exactly such a component, and the seat did not even need to win a race: a
leftover of it can toggle the name between the real directory and a link to
`<common>/objects` in a loop. A launch that loses gets the SHARED object
store read-write, which is the one write ADR 0099 exists to remove.

### What landed

**The store is `<SeatWallDir>/seat-objects`**, with `SrcRoot` the wall dir,
at L3 (`seatStoreBinds`) and L4 (`seatStoreCageMounts`) alike. Nothing
grants the wall root (D2(a)), so inside the seat's namespace every component
of the source is under the read-only `/`; the seat reaches the store's
CONTENTS at `<common>/objects` and has no other name for it. The side path
`<own>/seat-objects-base` and the `info/alternates` text are unchanged —
that one is a DESTINATION, and a destination inside a grant is pinned by
being this render's own mount.

A producer that cannot name a wall dir REFUSES rather than falling back to
the git dir (`seatStoreNoWall`), because the fallback is the hole. At L4,
which renders a mount list and returns no error, the fail-closed answer is
`<common>/objects` read-only over itself: history reads, nothing commits.

**The launcher writes `<wall>/origin`** — the resolved tree path and the
common dir — when it makes the dir. That file is what makes an orphaned wall
store findable: the dir's name hashes the RESOLVED tree path, so once the
tree is gone there is no path left to hash and no way to tell one repo's
abandoned store from another's.

**Two arms in every importer** (`importSeatObjects`,
`importOrphanedSeatStores`, `importAllSeatStores`, `pruneWorktrees`): the
wall arm — from the tree path for a live tree, from an origin sweep of the
wall root for an orphan — and the legacy arm, `seatStoreAt` over the admin
dir, unchanged. **The first launch that renders the wall store drains the
legacy one** (`importLegacySeatStore`, in `prepareSeatStore`), because the
old store holds committed work and nothing binds the old place any more: a
seat whose branch tip is readable only there would come up with a git that
cannot read its own HEAD. A legacy store that will not import REFUSES THE
LAUNCH. It is drained and not moved — a rename would be the uncaged launcher
writing through the seat-writable components this ADR exists to stop
trusting. The legacy arm's removal is ranger-base-tbg3h, gated on a census
of zero legacy stores.

**The retire order is import both arms → `worktree remove`/`prune` → RemoveAll
the wall dir**, and a failed import refuses the removal. `pruneWorktrees`
removes the orphaned wall dirs of THIS repo after a successful prune, keyed
on the origin file's common dir.

**D2(b) is one function.** `bwrapPinned(p, grants, dsts)`: p is pinned when
every component of it that lies strictly inside a write grant is itself a
mount destination. "Strictly inside a grant" is "its parent is writable",
which is what you need to create, rename or unlink a component; "is a mount
destination" is what refuses the rename (EBUSY, ADR 0100 P4/P4b). It is one
function because ADR 0100 D4(a) asks the same question of every
destination, and two implementations of "pinned" is two answers
about one wall. `bwrapRefuseUnpinnedSource` runs it over the finished argv
at the end of `BwrapArgs`, where all three of this package's renders meet.

### What the build found that the ADR did not

**The landed-check judge and the wall venue did not pass D2(b).** The ADR
asserted they did, on the grounds that no walled seat writes
`StateDir/landed-check/run-*`. That is an argument about WHO can write the
dir, and it is true; the predicate asks something else. MEASURED by running
the shipped predicate over the two rendered argvs: the judge's private /tmp
is `tmp` INSIDE the run dir, the venue's private /var/tmp is `var-tmp` in the
same place, and the run dir is the one thing either render grants — so both
components lie strictly inside a write grant and are mounts of neither
render.

The fix is the one the ADR itself prescribes for a D4(a) miss, applied to a
source: an in-place bind of the loose component, by the render that already
grants it (`BwrapArgs` step 2a-bis, and `wallVenueArgv` for the addition it
makes after `BwrapArgs` has returned). It grants nothing — in place is
read-write over a path the run-dir grant already made read-write — and it is
CONDITIONAL on that grant, because where no grant covers the source the same
line would hand out a write nobody asked for. A seat's own sources need none
of it. The alternative was an exemption ("a source under a dir no walled seat
writes is pinned by its lifetime"), which is a rule with a second rule to
maintain and a judgement call at every future source. ADR 0100's D2(b)
paragraph is amended with the measurement and the fix.

The venue's `--bind <run>/var-tmp <run>/var-tmp` is a fourth argv addition in
a render ADR 0097 D1 describes as "the judge plus exactly three things".
`TestQAWallVenueArgvIsTheJudgesPlusADR0097sThree` is amended to expect it,
and says why it is still one of the three: no capability is added, because
the path was already writable.

**The rule stands down where the wall root does** (`bwrapWallRoot`), i.e. on
a box whose home is itself under the shared `/tmp`. `/tmp` is granted
unconditionally, so there every source would be unpinned and every render
refused. No production launch is that shape and every fixture in
internal/posse is, which is why the D2(b) pins put their home under
`wallUngrantedDir` — a pin written over a `t.TempDir()` home would measure
the stand-down and read green for it.

**Stated residual in the predicate**: "write grant" is the render's grant
list, the paths bound read-write IN PLACE. A read-write bind whose source
differs from its destination also exposes its source host-side, so a
component lying inside another bind's rw source is seat-writable and
`bwrapPinned` does not say so. No render here has that shape (the only
aliased rw sources are the wall members, and nothing binds their parents);
a bead that adds one widens the function rather than exempting itself.

### Mutations run

Each applied to the tree, the named test run, then reverted:

| mutation | reds |
|---|---|
| `seatStoreFrom` puts the store back under the git dir | `TestQAWallPinnedSeatRenderPinsEverySource`, `TestQAWallPinnedRenderRefusesAStorePutBackUnderTheGitDir`, `TestLiveBwrapSeatStoreHostPathIsUnreachableInTheWall` |
| `BwrapArgs` step 2a-bis dropped | `TestQAWallPinnedJudgeAndVenueSourcesArePinnedToo` |
| `prepareSeatStore` no longer drains the legacy store | `TestQASeatImportFirstWallLaunchDrainsTheLegacyStore` |
| `RemoveSessionTree` removes the wall dir before the import | `TestQASeatImportRetireTakesTheWallDirOnlyAfterTheImport` |
| the wall arm dropped from `orphanedSeatStores` | `TestQASeatImportPruneImportsAnOrphanedWallStoreOfThisRepoOnly` |
| the origin sweep stops keying on the common dir | the same test (this repo's prune tries to import another repo's store) |

The live arm (`TestLiveBwrapSeatStoreHostPathIsUnreachableInTheWall`) runs a
real bwrap from the shipped render and measures that a walled seat can
neither write nor rename the store's host path, its wall dir, or any
component of either, while its writes at `<common>/objects` still land.

### Not closed here

- ranger-base-gn050
- ranger-base-tbg3h
