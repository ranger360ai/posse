# ranger-base-mlw6q — the wallWitness home, and the two layers under it

Two `wallWitness` arms, plus a third arm's control, red from a `wtqaHome()`
home on a linux seat whose `TMPDIR` points outside `/tmp`. ranger-base-jhmm7's
class — a fixture home from a bare temp root, green on this box only because
linux's `t.TempDir()` IS under `/tmp`, where the wall-root containment refusal
stands down.

The fix is one line in `internal/posse/wallwitness_qa_test.go`: `wwTempRoot`
takes its `$HOME` from `wallStandDownDir` instead of `wtqaHome`. That is the
answer ranger-base-shf97, ranger-base-zbmxc and ranger-base-1aayx took, and
this bead is the fourth in the class to take it.

What is worth recording is not the line. It is that **this fixture needed a
second thing to be true, and at the commit this bead started from it was
not** — so the obvious fix was red, and the reading that explains why is the
deliverable.

## Layer 1: the wall-root refusal

`BwrapArgs` judges every write grant against the per-seat wall root composed
from `$HOME` and refuses the render outright where one OVERLAPS it
(`bwrapRefuseWallRootGrant`, ADR 0100 D1/D2(a)). The refusal stands down where
the home is itself under `/tmp` (`bwrapWallRoot`).

This fixture is the one shape that KEEPS the launcher's `TMPDIR` grant:
`bwrapSeatTmp` answers `""` for a cwd under `/tmp` — which is the arm's whole
premise, since that is what leaves the blanket `/tmp` grant covering the tree —
so `bwrapDropTmpGrants` never takes `TMPDIR` back out. A `$HOME` cut from
`TMPDIR` is therefore a wall root the render walks straight into.

## Layer 2: the prep host fence, and why `wallStandDownDir` alone was not enough

These arms drive the full PRODUCTION render (`f.a.RenderWall`), not the pure
`BwrapArgs`. The production render runs `runBwrapPrep`: real writes on the
HOST, in the seat's name, at paths the render chose. A test binary fences
those (`prepHostFenceWhy`, ranger-base-bncbk).

At this bead's base (e17ea911) that fence named the binary's temp home,
`os.TempDir()`, `/var/tmp` and this seat's declared ungranted root — and
`/tmp` only by way of `os.TempDir()`, which covers it just while `TMPDIR` is
unset or itself under `/tmp`. **That is the same host accident
`wallStandDownDir` exists to escape.** So moving the home onto a stand-down
root traded a wall-root refusal for a fence refusal, one layer down, and two
of this fixture's steps are under `/tmp`: the seat's own `.cache`, and the App
home's `runtimes` at `<fixture root>/home`.

`ranger-base-shf97` landed `bwrapTmpMount` as a NAMED entry of that fence
(`prepHostFenceRootList`), pinned by
`TestQATheStandDownFixtureRootIsAPrepHostFenceRoot`, having hit the identical
two layers in `wallSeedProjectConfig`'s arms. With that entry in, the
one-line fix here is correct and sufficient.

## MEASURED

2026-11-22, this box (linux 7.2.15, go1.27.3, 16 cores), over
`-run 'TestQAWallWitness|TestLiveWallWitness' ./internal/posse` with the repro
ranger-base-1aayx uses. The movers are
`TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate` (both subtests)
and `TestQAWallWitnessTakesTheRowAwayWhenTheWallCannotBeRenderedAtAll`'s
control.

| home | fence | `TMPDIR=/var/tmp/posse-mlw6q-repro` |
|---|---|---|
| `wtqaHome()` | either | `bwrap cannot grant write on /var/tmp/posse-mlw6q-repro: it contains the per-seat wall root …/001/.cache/posse-wall … (ADR 0100 D1/D2(a))` |
| `wallStandDownDir` | at e17ea911, pre-shf97 | `cage seatbelt (bwrap) refuses to prepare /tmp/posse-standdown-N/.cache …: the bwrap prep host fence is up … under none of <binary home> /var/tmp/posse-mlw6q-repro /var/tmp` |
| `wallStandDownDir`, `$HOME` only | at e17ea911, pre-shf97 | the same fence one step on: `refuses to prepare /tmp/posse-wwtmp-N/home/runtimes` |
| `wallStandDownDir` | with the shf97 entry (main today) | **ok** |

With `TMPDIR` unset every row above reads `ok`, which is the whole reason the
class is invisible on this box and on ci.yml's ubuntu legs.

Final state, 11 tests, no skips and no `NOTHING MEASURED` on either shape:
`ok 0.915s` with `TMPDIR` unset, `ok 1.207s` under the repro.

Rows 2 and 3 were taken at e17ea911, on the real pre-entry tree, and not from
a mutant — so they are also the red-before for the coupling shf97's pin now
holds: take `bwrapTmpMount` out of that fence and these arms red at the render
again.

## The route not taken, and why it is recorded rather than argued

While this bead was in flight and before shf97 landed, the fix here was built
the other way: move the whole SEAT — `$HOME` and the App home — out of the
`/tmp` tree root into one `wallUngrantedDir` root, on the reasoning that
`/var/tmp` is outside every blanket grant (so nothing can overlap the wall
root) *and* is a root the fence already named, needing no fence change at all.
That version was green on both shapes too (`ok 1.318s` / `ok 2.224s`,
commit `6374b814`, superseded).

It was abandoned on a comparison and not on a preference:

- It is a SECOND answer to a class that now has one. `bhsDeclaredRoots` reads
  `wallStandDownDir` as "the render must succeed" and `wallUngrantedDir` as
  "the refusal is the subject"; using the second for a render that must
  succeed works, but reads as the opposite of what it is.
- It costs coverage. `wallUngrantedDir` abstains where `/var/tmp` is
  read-only — every seat inside its own bwrap wall — so the whole file would
  have gone dark on such a seat instead of half of it, unless that seat
  declares `POSSE_TEST_UNGRANTED_ROOT` (ranger-base-ydw7p).
- It is a larger diff: a new helper plus a moved App home, against one
  identifier.

The argument it rested on — that the fence should not name `/tmp`, because
ranger-base-aurb8's 13 macos reds were this same fence catching a
`mkdir -p /private/tmp/posse-upgrade` and its verdict was "the fence was
right; the grant should not have been there" — is a **real cost of shf97's
entry and is noted here as one**: that refusal would now be admitted
silently. But it is a cost to a different invariant. The fence's subject is
"do not write into somebody's live area", and `/private/tmp/posse-upgrade` is
not one; catching a surviving grant was a side effect it was never designed
for. shf97's own reasoning — the entry widens the fence no more than the
`/var/tmp` entry already does, both are world-shared temp roots holding no
live area of this box's operator or of rex, and the keep-out arm still decides
a passwd home strictly inside either — holds on its own terms.

## The guard

`wwTempRoot` now checks its root against the ANSWER and not the spelling, which
is `wspFixtureIn`'s guard (ranger-base-shf97) applied to this fixture's second
host-prep step: the App home it carries is `<root>/home`, admitted by the fence
only because the root is under `bwrapTmpMount`. A root that resolved elsewhere
— darwin spells `/tmp` `/private/tmp` — would red at the fence with a sentence
about somebody's live area, so the fixture says so itself instead.

`wallStandDownDir` makes the same check over the home, so both halves of the
seat are checked where they are chosen.

## Not visible to the tree-wide pin

`internal/treepins/bwraphomestanddown_qa_test.go` reads a render a test writes
as `BwrapArgs`/`bwrapArgsWatched` in its own identifier call graph; this one
arrives through the METHOD `f.a.RenderWall`. ranger-base-shf97 measured eight
widenings over the live corpus — none with zero false positives, the cheapest
that sees a method-reached render costing 15 false reads — and
ranger-base-m33n5 holds the venue door for that path. Nothing here widens that
pin.
