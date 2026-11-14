# ranger-base-4tsa7 — the private loop's fourth arm, and one watch instead of four

Finding 1 of four from the verify batch [ranger-base-zkuwa](ranger-base-zkuwa.md), the posse half.
`bwrapArgsInto`'s `for _, pb := range carve.Private` loop had four arms, each
making its own `watch(WallWatchPrivate, pb.Dst)` call, and the fourth — the
empty tmpfs that [ranger-base-6ob20](ranger-base-6ob20.md) added, selected by `pb.Src == ""` — was
held by no pin at all.

It is a live production shape: `bwrap.go` appends `PrivateBind{Dst: d}`, Src
empty, for every dir `runtimeEphemeralDirs` names, which is codex's `tmp/` and
`shell_snapshots/` and grok's `daemon/run/`. So a host-side detach of one of
those private tmpfs mounts went unnoticed and the seat wrote into the shared
dir underneath it — the hazard ranger-base-a544g closed for the FILE bind and
[ranger-base-99mvt](ranger-base-99mvt.md) pinned for three of the four arms.

## MEASURED

This box, 2026-11-14, go1.27.3, warm cache, `-run TestWallWatch` over
`./internal/posse` (1.79s each), each mutant planted from a golden copy and the
tree read back by md5. Before the fix, one `watch(WallWatchPrivate, pb.Dst)`
commented out at a time:

| arm | site | result |
| --- | --- | --- |
| FILE bind (ranger-base-a544g) | `bwrap.go:617` | 2 FAIL |
| dir bound in place (ranger-base-ajkqx) | `bwrap.go:641` | 2 FAIL |
| empty TMPFS (ranger-base-6ob20) | `bwrap.go:652` | **ok 1.789s** |
| dir copy (ranger-base-5632o) | `bwrap.go:666` | 2 FAIL |

holden's reading, reproduced here before anything was changed. The survivor's
two tests are `TestWallWatchRenderRecordsEveryWallAndNoOpening` and
`TestWallWatchEveryRenderedMountIsAWallOrANamedOpening`, and arm 1 is the whole
of the coverage: none of the 330 `posse_arm2`/`posse_arm3` test files under
`internal/posse` mentions `WallWatch`, `wallwatch`, `bwrapArgs` or
`PrivateBind`.

After the fix, three mutants, all killed:

| mutant | result |
| --- | --- |
| the loop's ONE watch call commented out | 2 FAIL, naming all four private dests |
| the tmpfs fixture entry turned into an in-place one (`{Src: eph, Dst: eph}`) | 1 FAIL — "no PrivateBind entry in any render here is an empty TMPFS" |
| `wwPrivateArms` one arm short | 1 FAIL — "bwrapPrivateArgs renders 4 arms and wwPrivateArms names 3" |

(The first spelling of the second mutant — commenting the fixture entry out —
only broke the build on an unused `eph`, which measures nothing. Never quote
it.)

## Why a field reflection could not have caught this

[ranger-base-99mvt](ranger-base-99mvt.md)'s close widened
`TestWallWatchFixturesCarryEveryCarveOutField` to reflect over `PrivateBind`'s
fields and demand each be NON-ZERO in some render, and claimed that a field
added there "cannot again be rendered by no fixture". That holds only for arms
a non-zero value selects. The tmpfs arm is selected by `Src == ""` — a field's
ZERO value — so no non-zero demand can ever reach it, and it was already
unrendered while the reflection read green. The generalisation is small and
worth keeping: **a reflection over fields holds a field, and an arm is not a
field.**

## What changed

| file | change |
| --- | --- |
| `internal/posse/bwrap.go` | the four arms move into `bwrapPrivateArgs`, which returns the mount options, the prep steps and whether it rendered; the loop makes the one `watch` call over whatever came back |
| `internal/posse/wallwatch_test.go` | a fourth `PrivateBind{Dst: eph}` in `wwEveryShape`, its dest in the kinds-and-order literal, `wwPrivateArms` + `TestWallWatchFixturesRenderEveryPrivateArm`, and the "three arms" comments corrected |

The rendered argv and prep are unchanged by the move — the arm bodies are
carried over statement for statement, with `bind` hoisted to the top (both arms
computed it identically) and each arm's `continue` becoming its return.

### The class, and why the exit point rather than the count alone

[ranger-base-zkuwa](ranger-base-zkuwa.md) named two candidate fixes and called the choice a design
call: assert the number of `watch(WallWatchPrivate` sites against the fixture's
arms, or give the loop one exit point whose watch cannot be deleted per arm.
Both are here, because they hold different halves and neither is the other's
superset:

- **One exit point** is what closes the escape. A fifth arm added to
  `bwrapPrivateArgs` is watched by writing no new line at all, so there is no
  longer a per-arm watch for an author to be written without. That is the
  mechanism, not a pin, which is why it is worth a refactor rather than a test.
- **The arm count** is what keeps the FIXTURE total. With one watch, an
  unrendered arm's mount is at least recorded as a wall — but nothing else
  would say that `TestWallWatchEveryRenderedMountIsAWallOrANamedOpening` never
  renders that arm, so a new arm emitting a bwrap option `bwrapRenderOpts` does
  not classify would still be invisible. `TestWallWatchFixturesRenderEveryPrivateArm`
  reads the arms off `bwrapPrivateArgs`' own returns — one
  `return <argv>, <prep>, true, nil` per arm — and holds `wwPrivateArms`, a
  closed mirror of the branch order, to be a partition over the fixture
  entries. A fifth arm reds it until it is named and given an entry.

The pin reads `bwrap.go` by its own name from the package directory, the
spelling `TestQAWatchTimerSelectWakesOnlyOnTheLocalCarry` uses, so it takes no
repo-root helper and joins no tree-wide door class; `-run TestWallWatch` reaches
it, which is the filter this area is already read under.

## Not closed here

The other three findings of [ranger-base-zkuwa](ranger-base-zkuwa.md) are
ranger-core and could not land from a posse worktree: ranger-base-7cl6d (the
two registry pins, on gilfoyle's branches) and ranger-base-rxqfv (the merge
queue item that reads #111 as carrying a fix it does not).
