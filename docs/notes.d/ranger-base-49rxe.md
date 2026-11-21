# ranger-base-49rxe — the seat store pin's wide-grant arm asserted a refusal the render had just stopped making

`wall pins` was red in ADR 0097 D1's venue from 2026-11-21 02:45 onward, in
one member, at three consecutive landings. The red is a test arm whose
premise expired between two landings that were each green on their own base.
Nothing in the shipped render changed and nothing needed to.

## The red

    FAIL TestLiveBwrapSeatStoreRefusesRewritingASharedObject
    bwrapseatstorelive_test.go:221: the render bound the seat's own git dir in
    place under a whole-<common> grant with <common>/worktrees loose — anything
    this seat leaves behind renames that component and the next launch binds
    what it points at, read-write (ADR 0100 D4(a)/D2(b))

That is `bsWideGrantRefusedUnpinned`'s `err == nil` arm: it rendered the
fixture's wide grant set — the writable set plus `<common>`, with no grant on
`<common>/worktrees` — and asserted `BwrapArgs` REFUSED it, naming that
component as the loose one.

## The two halves

- **ranger-base-4gyib** (f7ca0aa6) added the arm. At its base the refusal was
  real: shape 1 bound every write grant in place and nothing else, so
  `<common>` granted with `<common>/worktrees/<name>` granted two components
  inside it left `worktrees` a mount of nothing, and ADR 0100's source and
  destination arms both refused the render. 4gyib's fix was to GRANT the
  component in the wide set, and this arm was the proof that the grant was
  the ADR's prescribed pin rather than a widening added to make a test pass.
- **ranger-base-dot6w** (one landing later) generalized D2(b)'s remedy:
  `bwrapGrantMounts` binds every directory STRICTLY BETWEEN a grant and the
  grant above it, in place. This fixture's shape is exactly that shape — it
  is the shape dot6w's list was written for — so the component is pinned by
  the render now and the set 4gyib expected to be refused is renderable BY
  DESIGN.

dot6w's branch was cut from 3c16c00c, one landing before 4gyib's f7ca0aa6, so
its venue run never saw this arm; 4gyib's run never saw the list. ADR 0088
D5's attribution rule reads as it says it does — the newest landing is the one
whose tree first held them both.

The 4gyib fragment's "The fix" section is superseded on exactly this point:
the wide shape does not need the `<common>/worktrees` grant, because the
render pins the component for itself. Its reading of WHICH arm refuses an
unpinned set still holds and is re-measured below.

## Reproduction, 0.26s, no venue needed

ADR 0100's two refusals stand down when `bwrapWallRoot()` is empty, which it
is when this uid's home lies under `/tmp` — and this package's `TestMain`
puts `HOME` under `os.TempDir()`. So the arm is dormant under an uncaged
`make test` and live in the venue, whose TMPDIR is under the posse state dir
(4gyib's fragment has the why). Move the fixture home out from under the
shared `/tmp` and the venue is not needed:

    mkdir -p "$HOME/.cache/posse-49rxe/tmp"
    TMPDIR="$HOME/.cache/posse-49rxe/tmp" go test -count=1 \
      -run 'TestLiveBwrapSeatStore' ./internal/posse

MEASURED 2026-11-21 (Linux 7.2.15-1-omarchy, bubblewrap 0.13.1, go1.27.3, 16
cores): FAIL at `c7744a04` with that TMPDIR, `ok 0.28s` with the fix, and the
rendered argv at the red carries `--bind <common>/worktrees
<common>/worktrees` — the render's own pin, which is the whole of the
diagnosis.

## The fix, in the test and not in the render

`bsWideGrantRefusedUnpinned` becomes
`bsWideGrantPinsTheLooseComponent`, and the wide set loses the now-decorative
`<common>/worktrees` grant — so the live arm renders the set a production
producer would hand `BwrapArgs`, the same set
`TestQAWallPinnedSourceWholeCommonGrantPinsTheSeatsOwnGitDir` reads off the
argv. The claim worth holding survives the merge one step further in: the pin
EXISTS, and it is LOAD-BEARING. The arm

1. renders the wide set and reads the in-place mount at
   `<common>/worktrees` off the finished argv,
2. CUTS that one `--bind p p` triple out of the argv
   (`bsArgvWithoutInPlaceBind`, keyed on both operands so a later `--ro-bind`
   or `--tmpfs` over the same path cannot be taken instead), and
3. asserts `bwrapRefuseUnpinnedSource` else `bwrapRefuseUnpinnedDst` refuses
   the cut argv, naming that component with its clause — `"<path> lies inside
   a write grant"` and never a bare substring of the path, which is a prefix
   of both paths such a refusal names.

Step 2 asserts the cut removed something, so a bind spelling it misses is a
CONTROL failure and never a green.

This is the statement the QA pin does not make. That pin asserts the wide
render is not refused, that the component is a mount destination, and that
every bind source is pinned; it does not say what happens when the pin is
gone, which is what keeps a producer from dropping the mount with every pin
green.

## Mutations

Applied one at a time by `mutate.sh` under the session scratchpad, each
restored from a baseline copy before the next, both files asserted
byte-identical to their baselines at the end. Filter
`-run 'TestLiveBwrapSeatStore|TestQAWallPinned|TestQABwrapSeatStore'`,
`TMPDIR="$HOME/.cache/posse-49rxe/tmp"`. Baseline: `ok 2.887s`.

| mutant | reds |
|---|---|
| shape 1's pin arm dropped from `bwrapGrantMounts` | this arm (the wide render is refused outright, at its `t.Fatalf`) and `TestQAWallPinnedGrantMountsNamesTheDirectoriesBetweenNestedGrants` (4 rows) |
| the `m.Src == m.Dst` skip restored in the source arm (dw2du's) | `TestQAWallPinnedSourceBindsATwoDeepNestsLooseComponentInPlace` — **and NOT this arm** |
| the cut made a no-op (`bsArgvWithoutInPlaceBind` returns argv) | this arm, at its CONTROL |

The second row is 4gyib's reading, re-measured at this tree and now about the
cut argv rather than an ungranted set: with dw2du's skip back in place the cut
argv is still refused, by D4(a)'s destination arm over the store's side bind
at `<common>/worktrees/<name>/seat-objects-base`, with the same loose
component. So this arm measures "the pin is load-bearing under ADR 0100" and
not one of that ADR's two decisions over the other — which is why it reads
the two refusals in sequence and asserts on whichever answers.

## Verification

    bash docs/adr/0097-wall-venue.probe.sh venue+check

MEASURED 2026-11-21, same box: `rc=0 wall=106s load=16.12`, **`wall check:
green, 46 pins measured`**. Before: `rc=1`, `wall pins RED: 1 of 46
member(s) FAILed`, the one member this bead's arm.

The run carries the same two `note:` lines 4gyib's did, and they are the same
pre-existing class: `TestLiveWallUnixLeavesCrossDirectoryRename` and
`TestLiveWallUnixRefusesAHostSocketBoundAfterTheSeatLaunched` cannot `bind`
their fixture socket — `invalid argument` — because the probe's run-dir path
spends most of `sun_path`'s 108 bytes before the test's name starts. They
print no ADR 0097 D4 marker, so the body counts them as "printed no marker"
rather than as members and exits 0 — a red for `make test` and not for this
check. Red at `c7744a04` too, and nothing to do with this bead:
ranger-base-vbp5a.

`make fmt-check` and `make tree-check` green (2m2s warm, load 7-16, one
treepins arm at 87.3s).
