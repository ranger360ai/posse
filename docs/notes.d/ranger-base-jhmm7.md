# ranger-base-jhmm7 — the door under the bwrap fixture HOME

The class, from the bead: `BwrapArgs` judges every write grant against the
per-seat wall root composed from `$HOME` and refuses a grant that contains it
(`bwrapRefuseWallRootGrant`, ADR 0100 D1/D2(a)). The refusal stands down only
where the home is under `/tmp` (`bwrapWallRoot`) — a property of the HOST, not
of the test: on linux `t.TempDir()` IS under `/tmp`, on darwin it is `$TMPDIR`
under `/var/folders/…`. So a fixture home from a plain `t.TempDir()` is green
on this box and red on macos arm 1, in a sentence that names a refusal instead
of the wall the arm came to measure. Three fixtures in two days, each found by
reading a macos CI log.

The pin is `internal/treepins/bwraphomestanddown_qa_test.go`, three arms, and
its own head comment is the record for the rule. What is here is the
measurements.

## The rule, in one line

A function that drives the bwrap render (`BwrapArgs` / `bwrapArgsWatched`, in
its own body or through identifier calls) from a `$HOME` that came from a bare
temp root rather than from one of the three DECLARED fixture roots —
`wallStandDownDir`, `wallUngrantedDir`, `sbRoot` — is the violation.

## Three declared roots, not one

The bead asked for the stand-down root. The corpus turned out to have three
ways of making a render's premise honest, and a rule stated over one of them
flags the other two:

| root | what it does | the pin's reading |
|---|---|---|
| `wallStandDownDir` | `os.MkdirTemp(bwrapTmpMount, …)`, resolved — the home is inside the one blanket grant both hosts spell `/tmp`, so the stand-downs cover it | renders and must SUCCEED |
| `wallUngrantedDir` | a root outside every blanket temp grant — the wall root is real and a grant over it is refused | refuses and must SAY SO |
| `sbRoot` | the grant MOVES instead: `TMPDIR` re-pointed at a sibling, `seatbeltTempGrants` stubbed to nil where the box cannot place the root outside them (ranger-base-u8xk4) | renders and must succeed, for a reason the host cannot take away |

MEASURED 2026-11-22 (linux 7.2.15 x86_64, go1.27.3, 16 cores, at 4f309bcb),
with `wallUngrantedDir` and `sbRoot` NOT yet in the vocabulary: 26 functions
flagged, 3 real. Of the 23 false reads, 11 were `wallUngrantedDir` fixtures
whose subject is the refusal itself (`wallseatwall_qa_test.go`,
`keepleaf_qa_test.go`, `wallcarvepins_qa_test.go`, `wallpinnedsource_qa_test
.go`) and 1 was `sbRoot` through `wgNewFixture`. Converting any of them would
have retired the pin they are.

## A method call is not an edge

MEASURED, same box and commit. With method-selector names as call-graph edges
the scan read 26 functions where 3 were real, and 10 of the 23 false reads
were one type's `render` method (or a `render` closure in `cost_test.go`)
reaching a `BwrapArgs` that belongs to neither. A receiver's type cannot be
resolved from source, and source is what has to be read: the package is three
build-tag arms and only `parser.ParseFile` sees all three at once. So the
graph is identifier calls, with locally-shadowed names dropped — which is
`temproot_qa_test.go`'s "drop a name the package binds twice rather than
guessing", applied to a whole namespace.

## The open-fixture idiom

`…FixtureIn(t, root)` sets `HOME` to a path under a root its CALLER chose, and
this corpus writes it eighteen ways over (`wgFixtureIn`, `wrcFixtureIn`,
`weFixtureIn`, `wspFixtureIn`, `cpFixtureIn`). Inside the fixture both a
`t.TempDir()` and a `wallUngrantedDir(t)` root read as the same untraceable
`f.home` — it reaches the `Setenv` through a struct field and a closure — so
the class is attributed at the CALL SITE instead, by the join over all
arguments. 27 of the 147 bare `$HOME` sites arm 1 counts are attributed that
way; `cpFixtureIn` alone has six `t.TempDir()` callers and two
`wallUngrantedDir(t)` ones, which is why the mechanism is pinned in both
directions in arm 2 rather than left to a floor.

## Red before, green after

The repro shape is ranger-base-1aayx's: `git apply -R` of d8acb142's two
files, which puts `wgFixtureAnyBox` back on `wtqaHome` + `gitTempDir`.

- PRE-FIX: arm 1 names exactly `TestQABwrapRendersNoWriteBindInTheStoreRepoGitDir`,
  `TestQAEveryWallResolvesTheStoreSpellingBeadsHomeAnswers` and
  `TestQAWholeTreeDenyRendersAtTheBwrapMechanism`, and nothing else. Arm 3
  reds too, correctly: `wallStandDownDir` is not declared in that tree.
  The first two are the two rows macos arm 1 carried at 2a50bcb5.
- POST-FIX: all three arms green, 0.4s.

Independently, with `TMPDIR=/var/tmp/<dir>` on this box, the same two tests
red with the macos log's sentence word for word — the shape that says the
detector and the runtime agree about which three those are.

## The production render path, and why it is a separate bead

`ranger-base-shf97`. Deriving the production functions that render bwrap on
EVERY host — a path to `BwrapArgs` along uniquely-named edges with every
`WallMechanism()` caller cut, because a path through the mechanism switch
renders bwrap only on linux where the class cannot bite — gives a clean set of
10 out of 3270 production declarations:

    BwrapArgs BwrapReport RenderBwrap bwrapArgsWatched bwrapRender
    mcpConfigWallRefusalOn mcpWriteSet renderBwrapWall renderWallInto
    renderWallOn

(91 ambiguous names dropped and 17 mechanism-asking functions cut; following
the ambiguous names instead puts 1651 of the 3270 in the set, which is the
measurement that says why they are dropped.)

Seeded in, that set flagged 13 live tests and **all 13 are green** under the
`TMPDIR=/var/tmp` repro — `bwrapRender` drops the blanket temp grants before
`BwrapArgs` sees them (`bwrapDropTmpGrants`), so on that path the refusal
needs the home to be granted by something else. And it **still missed the two
that are red there**, `TestQAWallSeedProjectConfigCensusOfWhatThePrepCreates`
and `TestQAWallSeedProjectConfigLaunchInTheHomeItSeededIsClean`, whose seed
call sits inside a method. 13 false and 2 missed is not a door. Those two
reds, and a sound rule for that path, are ranger-base-shf97's — filed with the
measurements rather than fixed here, because an unpinned fifth fix of this
class is the thing this bead exists to replace.

## Not closed here

- ranger-base-shf97
