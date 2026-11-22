# ranger-base-mlw6q — the stand-down root is the wrong answer for a fixture that drives the PRODUCTION render

Two `wallWitness` arms, plus a third arm's control, red from a `wtqaHome()`
home on a linux seat whose `TMPDIR` points outside `/tmp`. Same class as
ranger-base-jhmm7, ranger-base-shf97 and ranger-base-1aayx — a fixture home
from a bare temp root, green here by an accident of where this box puts its
temp files — and the first of the family whose answer is NOT
`wallStandDownDir`.

The fix is in `internal/posse/wallwitness_qa_test.go`: `$HOME` and the App
home both move out of the per-arm fixture root into one `wallUngrantedDir`
root (`wwSeatRoot`), leaving each arm's root to be the TREE and nothing else.

## Why the stand-down root does not work here

`wallStandDownDir` is the declared answer for a fixture home whose render must
succeed: a root under `/tmp`, where the wall-root containment refusal stands
down (`bwrapWallRoot`). Every fixture that has used it drives the PURE render
— `BwrapArgs` or `bwrapArgsWatched` — which emits an argv and runs nothing.

This pair drives `App.RenderWall`, the production entry point, which runs
`runBwrapPrep`: real writes on the HOST, in the seat's name, at paths the
render chose. A test binary fences those (`prepHostFenceWhy`,
ranger-base-bncbk) to the binary's temp home, `os.TempDir()`, `/var/tmp` and
this seat's declared ungranted root. **`/tmp` is none of those unless the
box's `TMPDIR` happens to be `/tmp`** — which is this box's and ci.yml's
ubuntu legs' shape, and exactly why neither layer is visible here by default.

So the two refusals are a vice: a home under `TMPDIR` hits the wall-root
refusal, a home under `/tmp` hits the prep fence, and only a root that is
*both* outside every blanket grant *and* named by the fence satisfies both.
`/var/tmp` is that root, and `wallUngrantedDir` is how this package cuts one.

It needs no stand-down at all, which is what makes it an answer rather than a
third guess: the wall-root refusal fires on an OVERLAP between a write grant
and the wall root, and `/var/tmp` is the one directory on both hosts that no
blanket temp grant covers. The wall root composed from this home is REAL and
nothing is granted over it. `ungrantedGrantRefusal`, inside that helper,
checks it rather than assuming it.

## MEASURED

2026-11-22, this box (linux 7.2.15, go1.27.3, 16 cores), at e17ea911, over
`-run 'TestQAWallWitness|TestLiveWallWitness' ./internal/posse`, with the
repro ranger-base-1aayx uses. Three seat placements against two `TMPDIR`
shapes. The movers are
`TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate` (both subtests)
and `TestQAWallWitnessTakesTheRowAwayWhenTheWallCannotBeRenderedAtAll`'s
control at the foot of the arm:

| seat placement | `TMPDIR` unset | `TMPDIR=/var/tmp/posse-mlw6q-repro` |
|---|---|---|
| `wtqaHome()`, App home at `<root>/home` (as shipped) | ok | `bwrap cannot grant write on /var/tmp/posse-mlw6q-repro: it contains the per-seat wall root …/001/.cache/posse-wall … (ADR 0100 D1/D2(a))` |
| `wallStandDownDir`, App home unchanged | ok | `cage seatbelt (bwrap) refuses to prepare /tmp/posse-standdown-N/.cache …: the bwrap prep host fence is up … is under none of <binary home> /var/tmp/posse-mlw6q-repro /var/tmp` |
| `wallStandDownDir`, App home unchanged — one step on | ok | the same fence, next step: `refuses to prepare /tmp/posse-wwtmp-N/home/runtimes` |
| `wwSeatRoot` (`/var/tmp`), App home with it | ok | **ok** |

Rows 2 and 3 are the same placement: fixing the `$HOME` moved the refusal onto
the App home, which also lived under the `/tmp` tree root. That is why the fix
moves BOTH halves of the seat and not just `$HOME`.

Final state, both shapes, 11 tests, no skips and no `NOTHING MEASURED`:
`ok 1.318s` with `TMPDIR` unset, `ok 2.224s` under the repro.

## The fence is not widened to `/tmp`, and that is measured too

The other candidate was one row in `prepHostFenceRootList` — add
`bwrapTmpMount`, since the package deliberately cuts fixture homes under
`/tmp` (`wallStandDownDir`, `bwsHome`) and the fence's own comment enumerates
only "the two temp roots this package's fixtures are cut from".

It is refused by a reading already in the tree. The 13 macos arm-1 reds
ranger-base-aurb8 diagnosed were this same fence refusing a
`mkdir -p /private/tmp/posse-upgrade`, and that bead's verdict was *"the fence
was right; the grant should not have been there"*
(`bwrapDropTmpGrantsOver`'s comment, `wallseatcache.go`). A fence admitting
all of `/tmp` would have passed those 13 silently. The fence stays as it is
and the fixture moves.

## What the fix does not perturb, read rather than assumed

- **The premise.** `bwrapSeatTmp` answers `""` — no private `/tmp`, blanket
  `/tmp` grant intact, tree writable — when the home OR the cwd is under
  `/tmp`, and the cwd is `wwTempRoot`'s root. The premise was over-determined
  before, by both halves; it is carried by the cwd alone now, which is what
  the helper's comment always said it rested on. An arm that stopped
  measuring because `bwrapSeatTmp`'s cwd arm broke reds now instead of
  passing on the home. `wwRequireTheWallAllowsTheCreate` reads the premise
  through the real render before any row is judged, so a green arm is a
  measured one.
- **The live arm.** `TestLiveWallWitnessHoldsUnderTheRenderedWall` keeps its
  tree under `wwRoot` and is green under the repro before and after. Its seat
  was never what made its wall hold: its cwd under `/var/tmp` gets the seat a
  private tmp, so `bwrapDropTmpGrants` takes out both grants that could have
  overlapped a `TMPDIR` home. What it loses is that second, redundant reason.
- **`wtqaHome`'s own count.** Its comment says "the two left are … sbRoot …
  and one init test". Two `wallwitness` calls had been added since; removing
  them makes that sentence true again.

## The cost, stated

The `/tmp` arms now carry `wallUngrantedDir`'s abstention through
`wwSeatRoot`: a seat whose `/var/tmp` is read-only — every seat running inside
its own bwrap wall — skips them unless it declares
`POSSE_TEST_UNGRANTED_ROOT` (ranger-base-ydw7p). The abstention is COUNTED and
fails the run undeclared (`wallUngrantedAbstain`), and the live arm of this
same pair already took it, so what changes is that the whole file goes dark on
such a seat instead of half of it — loudly, and with the same one variable to
set. That variable is also the fence's third root, so a seat that sets it
fixes both halves at once.

## Not visible to the tree-wide pin

`internal/treepins/bwraphomestanddown_qa_test.go` reads a render a test writes
as `BwrapArgs`/`bwrapArgsWatched` in its own identifier call graph; this one
arrives through the METHOD `f.a.RenderWall`, and a method edge keyed by name
alone is unsound in a package parsed per-arm from source. ranger-base-shf97
measured every widening that would reach it. Nothing here widens that pin.

The gap this bead *adds* to that pin's subject is worth naming: the three
declared roots answer "does the render refuse itself", and they do not
distinguish a fixture that drives the PURE render from one that drives the
production entry point and so must also satisfy the prep host fence. For the
pure-render fixtures the pin covers today, `wallStandDownDir` remains right.

## Not closed here

- ranger-base-shf97
