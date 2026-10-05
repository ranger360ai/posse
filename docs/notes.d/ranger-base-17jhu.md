# A red-gate bead filed from a 24-day-old reading (ranger-base-17jhu)

`ci.yml` on `main` in `ranger360ai/posse` was **green** on 2026-10-05 when
this bead was filed, and had been green for 24 days. The bead was real; the
reading behind it was not.

## What the bead said, and what main actually was

ranger-base-17jhu was filed by the dispatch pass's ci-watch
(`internal/posse/ciwatch.go`, ranger-base-x9e34) at 2026-10-05T12:45:29Z,
P1, naming:

    red now      9b24d3c6  2026-09-11T08:59:43Z  runs/34581919232
    red since    a40b2ce3  2026-09-11T08:55:51Z  runs/34581610397
    streak       2 consecutive failed run(s)

MEASURED 2026-10-05T12:47-12:56Z, `gh run list --repo ranger360ai/posse
--workflow=ci.yml --branch main --limit 100`: every one of the 100 runs in
the scan window is `success`. The window spans 2026-09-20T21:44:38Z to
2026-10-05T05:42:26Z — ~14.3 days of history at this repo's run cadence.
The newest run, 37268965993 at 8bbca6b6, is green in all six jobs
(`test (ubuntu-latest, 1|2|3)`, `test (macos-latest, 1|2|3)`), and 8bbca6b6
is `main`'s tip.

The two runs the bead names are a real, closed episode: the
`verify-shell-syntax` self-test's arm 1 asserting a darwin bash 3.2 verdict
on an ubuntu bash 5 runner. It was filed as **ranger-base-ftr3e** on
2026-09-11T09:07:23-04:00, diagnosed and fixed at 7b35b24b the same hour,
and `scripts/shell-syntax.sh` carries both run ids in its header to this
day. ranger-base-17jhu is that episode refiled 24 days later, at P1, onto a
dispatched session.

## The reading was stale, and so was one of mine

`ReadCI` sorts the page it is handed by `createdAt` descending and takes
`verdicts[0]` as the verdict. It never asks whether the page it was handed
is current. GitHub's Actions run-list endpoint is served from an index, and
that morning it was handing out old pages:

MEASURED 2026-10-05, 38 hand readings of the identical query, this box,
`gh 2.98.0`:

| reading | newest row returned | verdict |
|---|---|---|
| #1, ~12:47Z | 2026-09-28T20:59:08Z (d8809472) — 81 rows behind | **stale** |
| #2-#38, 12:48-12:56Z | 2026-10-05T05:42:26Z (8bbca6b6) | fresh, all identical |

So 1 of 38, and the stale one landed within ~2 minutes of ci-watch's own —
a different stale snapshot (81 rows behind, not 24 days), which is what an
index settling looks like rather than a fixed cache. The depth is not
bounded by anything: ci-watch's page topped out at 9b24d3c6, which is the
only top that yields `streak 2, since a40b2ce3`.

ASSUMED, not measured: that the staleness is GitHub's index and not `gh`.
`~/Library/Caches/gh` does not exist on this box and no `GH_*` or
`RHQ_GH_BIN` was set in the session, so there was no local cache to serve
it — but a single unreproduced reading cannot distinguish an index replica
from anything else upstream of it.

## The dedupe cannot stop this, by design

One bead per episode is ci-watch's load-bearing invariant, and all three of
its enforcement arms read the STORE, never the clock. A stale reading walks
past every one of them:

- `ciOpenBeads` is OPEN-only. All 37 prior ci-red beads are closed.
- `ciDupeFiled` reads closed beads back, scoped to the marker and the
  streak's own `Since` sha — but it is *gated on the closed bead lacking*
  `ciAlreadyCleared`'s comment, and it spends its one free pass by marking
  the bead with `ciRaceAbstainPrefix` and reading that mark back.

MEASURED 2026-10-05 over all 37 closed `ci-red` beads in this store, by
walking `bd list --label-any ci-red --status all --limit 100000 --json` and
`bd comments <id> --json` and matching each comment's text by **prefix**, the
way `ciAlreadyCleared` does:

    closed ci-red beads                     : 37
      a comment STARTING 'ci-red cleared: ' : 13
      the one-pass abstain already spent    : 7
      no cleared AND abstain spent          : 7

The 7 are ranger-base-bg1wm, -ftr3e, -f5l0d, -a8tqz, -s16kv, -487x1,
-vr7eb. Together with the 13 cleared ones that are 20 of 37 episodes a
stale reading refiles on the FIRST pass; the remaining 17 refile on the
second consecutive one. ranger-base-ftr3e is in the 7, and that is the whole
of why ranger-base-17jhu exists.

Match by prefix and not substring: the abstain comment quotes the string
`ci-red cleared:` in its own body, so a substring census reads 20 cleared
where 13 are. A pin for any fix here has to get that right or it measures
the wrong set.

## What is not decided here

A freshness guard needs a number nobody has measured — how far behind the
run-list index can legitimately be, against how long after a push a run
first appears — and the alternative shapes (compare `Latest.Sha` against the
local `origin/main` tip, or against the commit count between them) each rest
on their own. That is a distinct deliverable and is filed separately rather
than invented under a bead whose own premise was stale.

What is settled: `ci.yml` is green on `main`, in all six jobs, at `main`'s
tip, and nothing in CI needed a change.
