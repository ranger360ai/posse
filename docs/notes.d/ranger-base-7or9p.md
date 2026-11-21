# The judge's wall comes from the launcher, so a verdict can move with no landing (ranger-base-7or9p)

*2026-11-21 · developer · code bead under ADR 0088 D7/D8, from
ranger-base-q8j8x's verify of ranger-base-ufd7z*

## What was wrong

ADR 0088 D1 is careful about where the judge's BYTES come from — the copy
that runs is the last tree recorded green's, never the tree under judgement —
and D5's filed bead is written as though that were the whole of it:

    The newest landing is simply the one whose tree first held them all.

One input to the verdict does not come from the window's trees. The judge's
WALL is the launcher's: the socket-wall step's binary is `wallWatchBin()`'s
answer, the seatbelt profile is rendered by the running process's own code,
and ADR 0097's venue comes out of the same `boxLandedCage`. So a landing that
changes the wall is judged by the OLD wall until somebody reinstalls, and on
that install one tree's verdict flips with no landing at all. Nothing in the
verdict line or in the bead named the binary that judged, so a reader could
not tell a tree red from a judge red.

## The measurement that is the whole bead

MEASURED 2026-11-20 (taken on ranger-base-ufd7z, re-read under
ranger-base-q8j8x; this box, 16 cores): hold the TREE constant at main's
`edfe7a79` and change only the binary on the step. Render the judge's cage
over a `git worktree add --detach` of that tip, judge = that tree's own
`scripts/posse-landed-check` (byte-identical to the last-recorded-green tree
`b07f5e4f`'s copy), then run the rendered argv with the step binary
substituted:

| step binary | verdict |
| --- | --- |
| posse built at `b29c95b5` (pre-REFER) | **exit 2 in 139s** — `FAIL github.com/ranger360ai/posse/internal/treepins 79.775s`, `make: *** [Makefile:1311: treepins-check] Error 1`, four bdargvgate pins failing on `branch -M` with `Invalid cross-device link` |
| the installed `0.5.3+edfe7a79` | **exit 0 in 132s**, `11 prerequisites of make test`, zero `--- SKIP` |

Production said the same thing from its side: the judge commented `still red
at 853242ca2dc2 (f1509bcfb970)` while f1509bcf's tree already contained the
fix. What it cost, both instances measured on ranger-base-ufd7z: one P1 filed
and dispatched against a window of eleven innocent landings and the persona of
the newest one, a seat's session to establish that no tree in the window could
clear it, and a close whose stated timeline is wrong in the record.

## What landed

Two rules, one mechanism — the identity of the launcher that ran the check
(`internal/posse/landedlauncher.go`), read once per judgement and memoized so
one judgement's line, record and bead cannot name three launchers.

- **D7** · every verdict line and every filed bead names it: version, path,
  mtime, size. Version AND mtime because they fail differently — the version
  is the running code that renders the cage and the profile, the mtime is the
  file at the step's path that the socket-wall step execs, and a reinstall
  under a running launcher moves the second and not the first.
- **D8** · a green→red window is not attributed when the launcher is not the
  one that judged the green tree. One more `git config` line per rule
  (`posse.landedCheckBy`, `posse.wallCheckBy`), written beside a GREEN verdict
  and only there, so it holds "who judged the tree on record green" — the far
  edge of every window D5 attributes — and is still readable from inside the
  red run that needs it. Three answers: `same` attributes as before, `moved`
  files UNASSIGNED and names both launchers, `unknown` attributes as before
  plus a sentence saying the comparison could not be made.

ADR 0097 D5's wall-check rule gets both through the shared parameter
(`landedRule.LauncherKey`), not a copy, which is what D5 requires there.

## Why not a third field on D4's record

`landedRecorded` reads a record in any other shape as NO record, deliberately
— a half-parsed verdict must never read green. A record read as absent makes
every red a green→red transition again, which is one P1 per landing: the
ranger-base-4v4r6 incident D5 exists to end. A separate key is the house
pattern for this (D6.3's counter is its own key for the same reason).

## MUTATION-CHECKED

Each half reverted on its own, 2026-11-21, `-tags posse_arm2`:

- the assignee gate (`by.Drift != landedLauncherMoved`) removed → reds the
  `a launcher that moved` arm alone, on `the create assigned somebody although
  the launcher moved under the window`;
- the withholding and unknown paragraphs removed → reds the `moved` and
  `unknown` arms, and not the `same` one;
- `c.launcherNote()` dropped from the green verdict line → reds
  `TestEveryVerdictLineNamesTheLauncherThatRanTheCheck/green` alone;
- the launcher record never written → reds every drift arm and
  `TestARedVerdictLeavesTheLauncherOnRecordAlone`, each naming the empty
  `posse.landedCheckBy`.

The pin's own answer to "what behaves differently if this is deleted?": the
filer would blame a landing for a wall that moved — a seat dispatched against
innocent work, with the bead asserting in its body that the newest landing is
the cause.
