# ranger-base-shf97 — the two arms, and the rule that was not taken

ranger-base-jhmm7's class, two beads on: `BwrapArgs` judges every write grant
against the per-seat wall root composed from `$HOME`
(`bwrapRefuseWallRootGrant`, ADR 0100 D1/D2(a)) and refuses a grant that
contains it. The refusal stands down only where the home is under `/tmp`
(`bwrapWallRoot`) — a property of the HOST and not of the test. jhmm7 pinned
the class over the render a test WRITES and said in its own head comment that
two live reds reach the render through a method and so are outside what that
pin can see. This bead is those two reds, and the open question of a rule that
covers them.

Environment for everything below: linux 7.2.15 x86\_64, go1.27.3, 16 cores, at
648a32d6 (`ranger-base-jhmm7`'s landing), 2026-11-22. The repro shape is
`TMPDIR=<a directory outside /tmp>`, which is the linux spelling of the darwin
default this class is red on.

## HALF 1 — the two arms, and the layer under them

MEASURED, red-before, `TMPDIR=/var/tmp/shf97-repro go test -run
TestQAWallSeedProjectConfig ./internal/posse`:

    --- FAIL: TestQAWallSeedProjectConfigCensusOfWhatThePrepCreates
        wallseedprojectconfig_qa_test.go:166: bwrap render for claude: bwrap
        cannot grant write on <root>/home: it contains the per-seat wall root
        <root>/home/.cache/posse-wall … (ADR 0100 D1/D2(a))
    --- FAIL: TestQAWallSeedProjectConfigLaunchInTheHomeItSeededIsClean
        wallseedprojectconfig_qa_test.go:301: the same sentence

With `TMPDIR` unset — this box's and ci.yml's ubuntu legs' shape — both are
`ok` in 0.9s.

Why these two and not the thirteen the widening flagged: `bwrapRender` drops
the blanket temp grants before `BwrapArgs` sees them (`bwrapDropTmpGrants`),
so on that path the home has to be granted by something ELSE. Here it is:
`prep` renders a wall whose SESSION DIR is the fixture's own home
(`f.prep(t, f.home)`), so the home is a write grant in its own right and a
real wall root under it is refused whatever else moves. That is why the answer
is `wallStandDownDir` and not `wallUngrantedDir`.

**And the fix uncovered the same accident one layer down.** With both arms on a
stand-down root the render succeeded and both went red at the PREP HOST FENCE
instead (`:225`, `:336`):

    prep for claude: … /tmp/posse-standdown-…/home/.cache/posse is under none
    of /var/tmp/shf97-repro/posse-testhome-… /var/tmp/shf97-repro /var/tmp

`prepHostFenceRootList` (prephostfence.go) enumerates the temp roots this
package's fixtures are legitimately cut from — `os.TempDir()`, `/var/tmp`, and
a seat's declared ungranted root — and `os.TempDir()` names `/tmp` only while
`TMPDIR` is unset or itself under it. So the entry that covered every
stand-down fixture was the SAME host accident `wallStandDownDir` exists to
escape. `bwrapTmpMount` is a fourth root there now, named rather than
inherited, and it widens the fence by no more than `/var/tmp` already does:
both are world-shared temp roots holding no live area, and the keep-out arm
still decides a passwd home strictly inside either.

Green-after, same command: `ok` in 0.821s under the repro and 0.877s with
`TMPDIR` unset. What landed:

- both arms take `wspFixtureIn(t, wallStandDownDir(t))`;
- `wspFixtureIn` itself refuses a root outside `bwrapTmpMount`, in the
  fixture's own words rather than the render's — it is the one place in the
  source that can still say no, because the tree-wide pin cannot see this
  render at all (HALF 2);
- `prepHostFenceRootList` names `bwrapTmpMount`;
- `TestQATheStandDownFixtureRootIsAPrepHostFenceRoot`
  (wallungrantedroot\_qa\_test.go) holds that entry, and holds it as NAMED: it
  drops one `os.TempDir()` occurrence from the list and asserts the step under
  a stand-down home is still admitted, which models a box whose `TMPDIR` is
  outside `/tmp` without setting `TMPDIR` in a process where every parallel
  test's `t.TempDir()` reads it.

## HALF 2 — every widening that could see them, measured

The instrument: jhmm7's own detector (`bhsRead`, `bhsReach`, the violation
join) driven from a scratch test in `internal/treepins`, over the live corpus
with the PRE-FIX `wallseedprojectconfig_qa_test.go` overlaid from
`git show HEAD:` — so the two arms are bare homes again and the question is
only whether a rule reaches them. Both halves of the rule were read back per
function to prove the overlay applied: both arms read `homeBare` at `:165` and
`:300`, i.e. **the HOME half of the pin already sees them** (`bhsOpenHomes`
attributes the open-fixture idiom at the call site). The missing half is the
render.

MEASURED, 2026-11-22, same box:

| variant | reach a render | flagged | real (of 2) | false |
|---|---|---|---|---|
| the pin as it stands — seeds `BwrapArgs`/`bwrapArgsWatched`, identifier edges | 106 | 0 | 0 | 0 |
| + a method edge where the name is UNIQUE tree-wide | 109 | 0 | 0 | 0 |
| + a method edge keyed by NAME ALONE (unsound) | 142 | 2 | 0 | 2 |
| seeds = the 10 production entry points, identifier edges | 189 | 13 | 0 | 13 |
| the 10 + unique-name method edges | 192 | 13 | 0 | 13 |
| the 10 + name-alone method edges (unsound) | 230 | 17 | **2** | 15 |
| file-scoped (a file that renders flags every bare home in it), narrow seeds | — | 5 files / 8 funcs | 0 | 8 |
| file-scoped, the 10 seeds | — | 16 files / 25 funcs | **2** | 23 |

**Uniqueness was the one sound narrowing available and it does not reach.** The
rule would have been `temproot_qa_test.go`'s — admit a method edge whose name
has exactly one declaration in the tree and is bound nowhere else — and the
blocker is not the render but the intermediate hop: `bwrapRender` is unique
(1 declaration, never locally bound), while `prep` is declared twice in
internal/posse and bound as a local besides, so `f.prep(t, f.home)` is
inadmissible by exactly the rule that makes `bwrapRender` admissible. Over the
whole corpus the widening buys three more functions (106 → 109, 189 → 192) and
changes the flagged set not at all.

The two flagged by the narrow seeds with name-alone edges are
`canonicalwall_qa_test.go`'s, not the red ones — jhmm7's measured failure mode
("one type's `render` reaching a `BwrapArgs` that belongs to neither") landing
again at a different pair.

## The ground truth, and what the door for this class actually is

MEASURED 2026-11-22: `TMPDIR=/var/tmp/shf97-arm1 go test -count=1 -timeout 60m
./internal/posse` — arm 1, after HALF 1's fix — **263.032s, and two red test
functions, both of them this class and neither flagged by any variant above**:

    --- FAIL: TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate
        (both subtests: a linked worktree, a main checkout)
    --- FAIL: TestQAWallWitnessTakesTheRowAwayWhenTheWallCannotBeRenderedAtAll
        wallwitness_qa_test.go:382, :643 — "bwrap cannot grant write on
        /var/tmp/shf97-arm1: it contains the per-seat wall root
        /var/tmp/shf97-arm1/Test…/001/.cache/posse-wall"

That file is careful about the one root its subject is a PLACEMENT of
(`wwTempRoot`, a `/tmp` literal, with its own measurement in its own comment)
and takes its HOME from `wtqaHome` — `t.Setenv("HOME", t.TempDir())` — which
is the class. Its render is reached through `f.a.RenderWall`: a method, again.
Filed as its own bead rather than fixed here.

So the tally for the class in arm 1 is four members. The best static variant
flagged two of them and cost fifteen false reads; the RUN found four of four
and cost nothing false, at 263.0s on this box. The same run independently
re-measured jhmm7's thirteen as green, and the two `canonicalwall` reads as
green.

**No rule was taken.** Not one variant reaches zero false positives, the
cheapest that sees the bead's two arms at all costs fifteen, and that same
variant misses half the class. A rule that told the thirteen apart from the two
would have to model which grants reach the home after `bwrapDropTmpGrants` —
for these two, that the render's session-dir ARGUMENT is the home — which
needs parameter-to-argument matching across a method edge and a struct field,
three mechanisms, every one of them the kind of guess this pin's head comment
already refuses once. The honest door for the render-through-a-method path is a
run under a `TMPDIR` outside `/tmp`, and that is filed as a bead with this
price on it rather than invented here.

## Filed, not fixed here

- the two `wallwitness_qa_test.go` arms above (same class, `wtqaHome`'s bare
  home, a render reached through `RenderWall`)
- the venue door: run the wall pins with `TMPDIR` outside `/tmp`, priced at
  263.0s for arm 1 on this box, and the ADR 0098 question of whether that is a
  local door or a CI leg
