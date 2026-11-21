# ranger-base-dot6w — shape 1 pins the directories between nested write grants

ADR 0100 D2(b) judges every in-place bind since ranger-base-dw2du, and the
census that landed with that widening read "no render has a grant nested two
or more components inside another with nothing bound between". That was a
reading of five argvs, not a property of the shape. A sixth shape was already
in the tree and the live seat-store pin builds it.

## What was red

`TestLiveBwrapSeatStoreRefusesRewritingASharedObject`
(`bwrapseatstorelive_test.go:209`), 1 of 46 wall-pin members in ADR 0097 D1's
venue. Its second arm renders the production shape "a store of record inside
the same repo grants `<common>` whole": the grants are
`<common>/worktrees/s-1`, `<common>/objects`, `<common>/logs` and `<common>`.
`objects` and `logs` are ONE component inside the `<common>` grant, so their
parent IS that grant and they pin. `worktrees/s-1` is TWO components inside
it with `worktrees` loose between them — a mount of nothing inside a
directory the seat may write — so `BwrapArgs` refused the whole render:

    bwrap cannot bind <...>/.git/worktrees/s-1 in place:
    <...>/.git/worktrees lies inside a write grant of this render and is no
    mount of it ... (ADR 0100 D1/D2(b))

It fails CLOSED. A seat is not mis-walled; on a box in that configuration it
does not launch.

## Reproduction without the venue, 0.07s

The rule stands down where the wall root does (`bwrapWallRoot`), and this
package's `TestMain` puts `HOME` under `os.TempDir()` — which is why an
ordinary run is green and the venue (TMPDIR under the state dir) is red. Take
the fixture home out from under the shared `/tmp` and no venue is needed:

    mkdir -p "$HOME/p/tmp"
    TMPDIR="$HOME/p/tmp" go test -count=1 \
      -run '^TestLiveBwrapSeatStoreRefusesRewritingASharedObject$' ./internal/posse

MEASURED 2026-11-21 (Linux 7.2.15, bwrap 0.13.1, 16 cores): FAIL with that
TMPDIR at `3c16c00c`, PASS with the fix.

## The fix

`bwrapGrantMounts` (internal/posse/bwrap.go) is shape 1's mount list now:
every write grant, plus every directory STRICTLY BETWEEN a grant and the
grant that contains it, shallowest first so the deeper mount lands last. Each
of those directories is bound IN PLACE, read-write, which is what makes it a
mount destination and so refuses the rename (P4/P4b: EBUSY in the namespace
that holds it).

It is the remedy ADR 0100 already prescribes for a miss, said once for the
grants: step 2a-bis does it for the private /tmp SOURCE and
`runtimeEphemeralDirParents` for the CLI homes' tmpfs DESTINATIONS. Three
instances of one sentence, and this is the third, not a new mechanism.

**It widens nothing.** Every path it names lies strictly inside a grant that
has already made it read-write, and a path inside no grant produces no entry
— which is why the list is computed from the grant set and not from the path
shape.

**Keyed to the grants and not to the whole render's mounts**, one notch wider
than `bwrapPinned` asks. A later tmpfs or private bind over the same path
would pin it too, and the predicate at step 6 reads it that way — but shape 1
is rendered FIRST, so a pin computed from mounts that are not composed yet
would be a pin whose correctness depends on which step appends what. Where a
later mount does cover one of these paths it lands on top, as every narrower
statement in this render does.

## The census

Unchanged, and that is the finding: the same **150 in-place binds** over the
same five argvs (claude 55, codex 31, grok 50, judge 7, venue 7), MEASURED
2026-11-21 by `TestQAWallPinnedSourceEveryRenderPinsItsInPlaceBindsToo`,
which now LOGS its per-render count so `-v` re-derives the number instead of
leaving it to be retyped. No shipped render carries a nest for the new list to
pin: it is inert until a grant set has one, and the shape that has one is
built by the live pin and by a store of record in the same repository.

ADR 0097 D1's venue, with the fix: `rc=0`, 98s, **46 of 46 pins green**.
(Two tests inside the venue's filter fail for a reason of their own and are
counted as "printed no marker" rather than as members:
`TestLiveWallUnixLeavesCrossDirectoryRename` and
`TestLiveWallUnixRefusesAHostSocketBoundAfterTheSeatLaunched` cannot `bind`
their fixture socket — `invalid argument` — under the probe's long run-dir
path. Red at `3c16c00c` too, and nothing to do with this bead.)

## Mutations

Applied one at a time to `internal/posse/bwrap.go` by a script under the
session scratchpad, each restored before the next, with the file asserted
byte-identical to its baseline at the end. Filter:
`-run 'WallPinned|BwrapArgs|LiveBwrapSeatStore'`, `TMPDIR="$HOME/p/tmp"`.

| mutant | reds |
|---|---|
| shape 1's pin arm dropped (the list names them, nothing binds them) | `TestQAWallPinnedSourceBindsATwoDeepNestsLooseComponentInPlace`, `TestQAWallPinnedSourceWholeCommonGrantPinsTheSeatsOwnGitDir`, `TestLiveBwrapSeatStoreRefusesRewritingASharedObject` |
| the `Src == Dst` skip restored in the source arm (dw2du's) | `...BindsATwoDeepNestsLooseComponentInPlace` (its second arm) |
| pins emitted after the grants instead of shallowest-first over both | `...BindsATwoDeepNestsLooseComponentInPlace`, `TestQAWallPinnedGrantMountsNamesTheDirectoriesBetweenNestedGrants` (3 rows) |
| the grant condition dropped (every ancestor pinned, granted or not) | `...WholeCommonGrantPinsTheSeatsOwnGitDir`, the row table (7 rows), and 9 other `BwrapArgs`/`LiveBwrapSeatStore` tests |

Every mutant red at least one pin whose subject it broke; the fourth reds
widely, which is the point of a condition that says "inside a grant".

## What this does NOT close: ranger-base-a59w7

The same reproduction, run wider, reds five more of this package's pins — and
none of them is this bead's class. A DENY (`--ro-bind d d`) and a read MASK
(`/dev/null` over `d`) are bound in place too, and where `d` lies two or more
components inside a write grant with no mount between, the render is refused
for the deny's loose ancestor rather than a grant's: `<repo>` granted with
the deny `<repo>/.git/config.lock`, so `<repo>/.git` is loose; and the same
one directory lower, where a grant above a CLI home leaves that home's
credential-store directory loose under the mask of a file inside it
(`TestBwrapArgsSkipsAMaskOverAWriteGrantedCredentialStore`, "the grant is
above the store" — the store's own path is the test's and this page does not
restate it, ADR 0024 D3).

MEASURED 2026-11-21 under the recipe above: 8 top-level tests red at
`3c16c00c`, 6 with this fix. One of the two that went green is this bead's
live pin; the other is
`TestBwrapArgsGitConfigRefusalIsTheSessionDirsOwnGitDir`, where a grant
happened to be nested under the same `.git` the deny needed pinned — which is
the clearest statement of how far a grant-keyed list reaches. Of the 6 that
remain, five are that class and the sixth is an artifact of the recipe rather
than a class member: `TestBwrapArgsRendersAClaudeSeatOnlyThroughD19` grants
`os.TempDir()`, which contains the wall root when `HOME` is under it, so
D2(a) refuses it — green under an ordinary TMPDIR.

None of the five is a live wall pin, so the venue reads green over them and
only a filtered run with a moved TMPDIR says so. Same remedy, a different
producer at a different step of the render, its own census: ranger-base-a59w7.
