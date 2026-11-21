# ranger-base-a59w7 — the carve-out's denies and masks get their loose parents bound in place

ADR 0100 D2(b) judges every in-place bind and D4(a) every destination that
differs from its source. `ranger-base-dot6w` gave the GRANT producer the
remedy both arms prescribe for a loose component — bind it in place — and
said in as many words that it reached no further. It did not: a deny is an
in-place `--ro-bind` and a read mask is a `/dev/null` over a name, and the
directory above one of those holds no grant of its own, so no grant-keyed
list can name it.

## What was red

Five of `internal/posse`'s own pins, each a render REFUSED for a loose
component above a deny or a mask rather than above a grant:

| pin | the nest |
|---|---|
| `TestBwrapArgsGitConfigRefusalIsKeyedNarrowly` (4 rows) | `<repo>` granted, the deny `<repo>/.git/config.lock`, `config.worktree`, `<repo>/.gitdir/config`, `<repo>/etc/config` |
| `TestBwrapArgsGitConfigRefusalIsNotANestedRepositorysConfig` (2 rows) | a nested repository's `.git/config` under the cwd grant, and under a grant that holds only the nested repo — three and two components deep, so these are the rows where more than one directory is pinned |
| `TestBwrapArgsGitConfigRefusalResolvesTheSessionDirsSpelling` | the same under a symlinked cwd |
| `TestBwrapArgsGitConfigRefusalAsksTheSeatsOwnGrants` (1 row) | a whole-tree-deny PID (ADR 0086 D3), whose `<repo>/.git/config` deny is loose at `<repo>/.git` under a grant that is not the seat's own — which is that pin's whole subject, read from the other end |
| `TestBwrapArgsSkipsAMaskOverAWriteGrantedCredentialStore` ("the grant is above the store") | a grant above a CLI home: the store mask stands down inside a grant, the credential file keeps its `/dev/null`, and the store directory between the two is a mount of nothing |

It fails CLOSED. A seat on a box in that configuration does not launch; it is
never mis-walled.

## Reproduction without a venue, ~3s

The rules stand down where the wall root does (`bwrapWallRoot`), and this
package's `TestMain` puts `HOME` under `os.TempDir()` — which is why an
ordinary run is green. Take the fixture home out from under the shared `/tmp`:

    mkdir -p "$HOME/p/tmp"
    TMPDIR="$HOME/p/tmp" go test -count=1 \
      -run 'BwrapArgsGitConfigRefusal|BwrapArgsSkipsAMaskOverAWriteGrantedCredentialStore' \
      ./internal/posse

MEASURED 2026-11-21 (Linux 7.2.15, bwrap 0.13.1, 16 cores): five top-level
tests FAIL at `65d90a5e`, `ok` with the fix.

None of the five is a live wall pin, so ADR 0097 D1's venue reads green over
all of them — a filtered run with a moved TMPDIR is the only thing that says
it.

## The fix

`bwrapCarvePins` (internal/posse/bwrap.go, BwrapArgs step 5d): for every
mount the render makes from the deny producer down — a deny's in-place
`--ro-bind`, a `/dev/null` mask, a tmpfs, a Keep file, an allow bound back
under a mask — every ANCESTOR of its source and of its destination that lies
strictly inside a write grant and is a mount destination of nothing, bound in
place, shallowest first.

It is the third instance of one sentence and not a new mechanism: step 2a-bis
does it for the private `/tmp` source, `runtimeEphemeralDirParents` for the
CLI homes' ephemeral dirs, `bwrapGrantMounts` for the grants.

**It widens nothing.** Every path it names lies strictly inside a grant that
has already made it read-write, and a path inside no grant produces no entry
— which is most of them, since the masks a seat is not granted lie under the
read-only base.

**Ancestors only.** A bind source that differs from its destination can be
loose on its own account, and the remedy for THAT is D1's — put the source
under the per-seat wall dir — so this list does not quietly bind it in place
and step 6 still refuses it by name.

### Where the pins land, which is the whole of the ordering argument

Composed at step 5d, where every mount of the render is known, and **spliced
in at step 2d's mark**: after shape 1's grants, before every other mount.
Three things follow.

- A pin cannot shadow the statement it exists for. The deny it pins is a
  read-only bind, and the mask is a `/dev/null`, MOUNTED INSIDE the directory
  the pin binds — so a pin emitted after one would cover that name with the
  host's own directory again and take the wall off.
- Nor any other wall it happens to contain. 2c's private members are mounts
  over paths inside the same grants; a pin above one, emitted after it, would
  bury a wall the seat's siblings are held out by. Here it lands first and
  the private mount lands on top.
- And it cannot widen anything wherever it lands, because of the grant
  condition above. The only mounts BEFORE it are step 1's base and the 1b/1c
  hides, and a hide inside a write grant is already shadowed by that grant's
  own in-place bind at step 2, so a pin under one shows the seat nothing the
  grant did not.

The prep list is spliced at the same mark, because `PrepMaskPoint` and
`PrepTouch` make no parents of their own: a pin whose directory is not there
yet needs its `mkdir -p` to run before the mask point the launcher creates
inside it.

### A link at a pin's own name refuses the render

The one thing this producer could not inherit from shape 1's. A pin's path is
by construction a directory the seat may write — that is the whole reason it
needs pinning — so a link at that name is one a seat or its leftover planted,
and bwrap follows a link at a bind's source and at its destination alike.
Binding it in place would resolve BOTH ends to the target and mount that
target read-write at its own name: the very attack D2(b) refuses, let through
by the mount meant to refuse it, and step 6 then sees a component that IS a
mount destination and says nothing. So the pin loop `os.Lstat`s and refuses a
non-directory, naming what is there in words (`fileTypeName`, not
`Mode().Type()`'s `L---------`).

Following it and checking the target is not the other answer: the seat can
repoint the link between this render and `mount(2)`, which is the
check-then-use ADR 0100 rejects A2 for.

`TestQAWallCarvePinsRefuseALinkAtThePinsOwnName` holds both directions, with
the real-directory control beside it, because a refusal that fires on
everything is not a refusal. The link target there is INSIDE the grant on
purpose: a link out of the grant makes the deny itself resolve outside it, and
the deny arm then renders nothing at all — no mount, no pin, no refusal, which
is that arm's own documented behaviour and not this one's.

The same question at the other producer is filed and not fixed here. Its id is
in "Not closed here" below and is all this page says about it.

### Which mounts are asked about, and why that is not the same question

Step 5d reads its candidates from step 2e's mark — the deny producer — and
not from 2d's. The two producers between those marks each pin their own loose
component and are pinned for doing it, and
`TestQAWallPinnedDstTheCensusMissParentIsBoundInPlace` works by taking the
ephemeral parent's entry OUT of the carve-out and asking for the refusal: a
general list that re-emitted it answers that pin green with its producer
gone. MEASURED while building this — the first draft did exactly that and red
that pin. Retiring either producer in favour of this list is a bead of its
own, not a side effect of the one that needed the denies pinned.

## The census

**0 carve-out pins over all five argvs** this package renders (claude, codex,
grok, the landed-check judge, the wall venue), MEASURED 2026-11-21 by
`TestQAWallCarvePinsCensusOverEveryRender`, which LOGS the count and the
paths per render rather than asserting a number — a render that grows a pin
is correct, so a count asserted at 0 would red the day somebody fixed a wall.
What holds the property is the two censuses that were already there
(`TestQAWallPinnedSourceEveryRenderPinsItsInPlaceBindsToo`, 150 in-place
binds; `TestQAWallPinnedDstEveryRenderPinsEveryDestination`), both of which
red if a pin fails to be emitted.

So the list is inert in every shipped render until one carries such a nest.
The two shapes that do are a main-checkout seat with a `.git/config*` deny
(which ADR 0095 D1 refuses one line earlier, for a reason of its own) and a
grant above a CLI home whose credential store is masked.

## Mutations

Applied one at a time to `internal/posse/bwrap.go` by a script under the
session scratchpad, each restored before the next, the file asserted
byte-identical to its baseline at the end. Filter
`-run 'WallCarvePins|WallPinned|BwrapArgs'`, `TMPDIR="$HOME/p/tmp"`.

A mutant that leaves `slices` unused is a compile error and not a mutant, so
the two splice mutants keep a `slices` call that emits no mount.

| mutant | reds |
|---|---|
| step 5d's splice dropped (pins computed, nothing emitted) | all five pins above, plus all three of this bead's: `...ADenysLooseParentIsBoundInPlace`, `...AMasksLooseParentIsBoundInPlace`, `...RefuseALinkAtThePinsOwnName` (its control arm) |
| spliced at 5d's own position instead of 2d's mark | `...ADenysLooseParentIsBoundInPlace` and `...AMasksLooseParentIsBoundInPlace`, both at their order arms and nothing else |
| candidates read from 2d's mark instead of 2e's | `TestQAWallPinnedDstTheCensusMissParentIsBoundInPlace` — the pin that takes `runtimeEphemeralDirParents`' entry out and asks for the refusal |
| the grant condition dropped (every ancestor pinned, granted or not) | `TestQAWallCarvePinsNamesTheDirectoriesBetweenAGrantAndACarveOutMount` (12 of 12 rows) and `TestQAWallPinnedSourceWholeCommonGrantPinsTheSeatsOwnGitDir` ("a pin inside no grant hands out a write nobody asked for") |
| the ancestors-only guard dropped (a loose bind source pinned in place) | `...NamesTheDirectoriesBetweenAGrantAndACarveOutMount` (1 row, the bind source) |
| the pin's `os.Lstat` replaced by `os.Stat`, which follows | `TestQAWallCarvePinsRefuseALinkAtThePinsOwnName` |

Every mutant reds at least one pin whose subject it broke, and the splice
mutant reds only the order arms — which it did not on the first run, where it
red the PRESENCE arm instead, because a pin spliced at the very end of the
argv has no trailing space for a `--bind p p ` string search to find. The
order arms read argv indices now (`wcpMountAt`), so a mutant that moves a
mount is reported as having moved it.

The grant-condition mutant reds every row, which is the point of a condition
that says "inside a grant": without it the list pins `/` and everything down
to the deny.

## Not closed here

- ranger-base-pg251
- The mounts the wall venue adds AFTER `BwrapArgs` (`wallVenueArgv`) get no
  pin from this list — `boxLandedCage` judges them with the same two
  predicates and REFUSES, which is where they stood before this bead. Nothing
  in that render has a loose component today (the census above reads the
  finished venue argv and answers 0), and a venue addition that grew one
  would want the pin emitted beside it, the way every other producer here
  emits its own.
- The four producers that bind a loose component in place are still four —
  step 2a-bis, `runtimeEphemeralDirParents`, `bwrapGrantMounts` and now
  `bwrapCarvePins`. ADR 0100 says why each stays where it is (a source, a
  tmpfs destination, the grants, the carve-out), and step 2e says what breaks
  if one swallows another. Consolidating them is a design question and not a
  hole.
