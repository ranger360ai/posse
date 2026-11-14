# The exemption moved to the other side of the call (ranger-base-n2veh)

ranger-base-4fx5x finding 1, fixed. Check 4's own-bead exemption
(ranger-base-ed07e, commit `e3b41b30`) dropped the committing tree's own open
`vuln-private` id from the ids handed to `vulnPrivateHits`. That took it out
of the PRESENCE test, which is the whole intent, and out of the STRIP that
runs before the bareness test, which was nobody's intent — so the own id's
own letters were read as a description of whatever OTHER open id shared its
unit.

MEASURED 2026-11-14 at `025d2e58`, through the shipped
`RunGatesVulnPrivateScan` in a real session worktree whose branch record
names the own bead, both beads open and labelled `vuln-private`
(own `ranger-base-l751c`, other `ranger-base-fixt2`):

| text | before | after |
|---|---|---|
| `Not closed here:\n- <own>, <other>\n` | HIT, names `<other>` | clean |
| `<own> <other>\n` | HIT, names `<other>` | clean |
| `Not closed here:\n- <own>\n- <other>\n` | clean (two units) | clean |
| `- <other> the mechanism` | HIT | HIT |
| `<own>: the fix this commit carries` | clean | clean |

A unit of nothing but bare ids was refused, naming an id the unit described
nowhere, over a remedy — "a flagged id names ANOTHER hole", "write the bare
id alone" — the writer had already performed. ADR 0068's class, and the class
`vulnprivate.go`'s own comments cite as the cost to avoid.

## The shape of the fix, and why it is not the shape 4fx5x suggested

4fx5x asked for two lists in the caller: flag only non-own ids, strip every
open id. Same effect, and this lands the equivalent on the other side of the
call — every open id goes IN, and `RunGatesVulnPrivateScan` drops its own id
from what comes OUT. Identical in every case, because stripping is what the
two lists differed about and `vulnPrivateHits` records a hit for every id in
a non-bare unit either way.

The reason to prefer it is that the broken invariant is
`vulnPrivateHits`' own: *every id present in a unit is stripped from it
before the bareness test runs*. A `flag`/`strip` pair keeps that sentence
true only while callers pass the pair correctly — the same sentence was true
of the one-list signature too, until a caller held an id back. Filtering the
RESULT leaves no way to hold one back, so the sentence is now a property of
the function and not a request of its callers. The doc comment says that in
as many words, because the next exemption will be written by someone reading
it.

It costs one git call less on the common path, too: `vulnPrivateOwnBead` is
now asked only when something was flagged, so a clean commit message pays no
git call at all.

Pinned by `TestRunGatesVulnPrivateScanStripsTheExemptedOwnBeadToo`
(internal/posse, arm 1), which carries both refused shapes and both controls
from the MEASURED run above. Re-breaking the fix to the pre-fix shape reds it
with exactly the recorded output.

## Not closed here

- ranger-base-li3up — a separate report, open and in this same check: a
  commit in a session worktree whose `posseBead` record read the own bead was
  refused NAMING THAT OWN BEAD. That is the exemption not applying at all,
  which the defect fixed here cannot produce (a held-back id could never be
  flagged), so the two are not the same bug and this fix does not close it.
  Not vuln-private, so it is named rather than left bare.

## Finding 2 needed nothing filed

4fx5x finding 2 was three verified, unpushed ranger-core branches that no
bead named. Between that finding and this bead, monica scanned for them,
pushed all three, and filed ranger-base-0m9pa — one P1 operator ask carrying
every ranger-core PR in merge order. Verified 2026-11-14: each branch's local
tip equals its `origin/` tip at the sha the finding recorded
(`jian-yang/arxiv-lane-reach` 298d317 as #116,
`jian-yang/extraction-budget-per-lane` cbdbd4f as #114,
`jian-yang/represent-stored-articles` 9419509 as #119).

The merge order was the author's to state and is confirmed on that bead: #114
before #119 is a hard constraint — #119 literally contains cbdbd4f — and #116
has no ordering constraint against either, its three files
(`specs/data-sources.md`, `src/ft/collectors/arxiv.py`,
`tests/collectors/test_arxiv.py`) overlapping neither branch's file set. The
"they all touch the same extraction budget" interaction is a RUNTIME one —
arXiv pacing changes what the shared cap then divides — and not a merge-order
one.

The standing lesson is ranger-base-un2no's and is recorded in this persona's
ORDERS.md: a ranger-core bead is not done until its branch has a push ask
filed, because the PID denies push and a closed bead with a local-only branch
is invisible work.
