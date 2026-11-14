# A mount SHAPE the fixture reflection could not see, one level down (ranger-base-99mvt)

The verify at ranger-base-7h7cs filed one live finding against
ranger-base-a544g's close: the FILE private bind it added
(`internal/posse/bwrap.go`, the `pb.File` arm) is rendered by no wall-watch
fixture, so the `watch(WallWatchPrivate, pb.Dst)` on that arm is held by no
pin. 2026-11-14, this box (linux 7.2.15 x86_64, go1.27.3, git 2.56.0, 16
cores), branched at 025d2e58.

## Why the two pins that exist for exactly this were green

`TestWallWatchEveryRenderedMountIsAWallOrANamedOpening` derives its
expectation from the rendered argv, so it can only see shapes the INPUTS
ask for — which is why ranger-base-mxbq0 gave it
`TestWallWatchFixturesCarryEveryCarveOutField` as its other end: every
field of `SeatbeltCarveOut` must have a value in one of the renders.

That reflection stops at `SeatbeltCarveOut`'s own fields, and
`SeatbeltCarveOut.Private` is a slice of a STRUCT whose own fields pick
which arm of the private loop renders. `Private: []PrivateBind{{Src: src,
Dst: priv}}` is non-zero, so the field pin was satisfied while
`PrivateBind.File` had no value anywhere in the tree (`File: true` appeared
in no `*_test.go` under internal/posse) and a whole rendered arm — its
lstat discipline, its `--bind`, its watch — was rendered by nothing. The
field pin's own failure message states the guarantee it was not delivering
for that arm.

## The fix

`wwEveryShape` carries three `PrivateBind` entries now, one per arm of that
loop: the dir COPY (`Src != Dst`), the FILE (ranger-base-a544g's shape, a
regular file at both ends), and a dir bound IN PLACE read-only
(claudeProjectsBinds', ranger-base-ajkqx). Their two new destinations are in
`TestWallWatchRenderRecordsEveryWallAndNoOpening`'s kinds-and-order literal.

And the reflection goes one level down: `PrivateBind`'s own fields must each
have a value in some render's `Private` entries, with the same "classify
what it mounts" message. That closes the class rather than the instance — a
field added to `PrivateBind` tomorrow is a new mount shape, and it cannot
again be rendered by no fixture while the carve field above it reads
non-zero. The nesting is deliberately one type wide: `SeatbeltNarrow` is the
other struct element type in that carve, and `bwrapRender` clears
`Carve.Narrow` before the render, so its fields select no arm and reflecting
over them would red for a mount that does not exist.

`PrivateBind.ReadOnly` had no value in any render either — it is why the
in-place entry is the read-only one — so the widened pin was red on two
fields when it was first run, not one.

## What the mutants said

| mutant | before | after |
|---|---|---|
| delete `watch(WallWatchPrivate, pb.Dst)` on the FILE arm only | **GREEN** | RED, both render pins |
| `File: true` dropped from the fixture's file entry | **GREEN** | RED, `PrivateBind.File` named |

The first row is the finding's own REPRO, reproduced at 025d2e58: the dir
arms at the three other `watch` call sites stay, and the focused run the
bead names (`-run 'Wall|Bwrap|Cage|Gates|Claude|Private|Watch|Sentline|Echo'
./internal/posse`) read `ok ... 83.735s` unmutated here. With the fixture
above, the same deletion reds
`TestWallWatchRenderRecordsEveryWallAndNoOpening` (the watch list is short
one `private` entry) and
`TestWallWatchEveryRenderedMountIsAWallOrANamedOpening` ("the render mounts
over .../cfg/history.jsonl and neither watches it nor names it an
opening").

## What this did not change

Nothing in `bwrap.go`. ranger-base-a544g's arm was correct — it watches
what it mounts — and the hole was that nothing would have noticed if it
stopped. The live half (`wallwatchlive_test.go`) still renders a dir bind
only, and does not need a file one for the mechanism it pins: `mountTable`
is keyed by mountinfo's mount POINT with no file/dir distinction anywhere
in `wallWatchBaseline` or `wallWatchDetached`, so the detach diff is
shape-agnostic by construction. What was shape-specific was the render, and
that is what now has a fixture.
