# The verify-chain bound, built to bind at any batch size (ranger-base-fswbr)

2026-11-09. Darwin 25.6.0 arm64, this worktree at af448c76 (parent e2499fcc).
The decision is ADR 0006 §3 as amended by ranger-base-rffxq, whose
measurements this implements; the defect is ranger-base-bu8rz. This file is
what the build measured, not what the design argued.

## What shipped

Both halves of the amendment, in `internal/posse/verifyafter.go`:

- **The walk** (`verifyChainRounds`, now a `verifyChainWalk` method). At a
  verify bead answering N closes it counts the hop and continues through the
  SHALLOWEST source: `1 + min` over the sources not already on the path. The
  hops returned are the minimum's path; a batched hop renders as
  `verify Q of S (one of N)` where it used to render `(a batch)`.
- **The grouping** (`verifyRoundGroups`). The pending closes are grouped by
  the rounds their lineages have spent; the existing batch loop runs inside
  each group, in close order, and each group's tail is held on its own until
  `verify_batch_age:`.
- `verifyChainBound` and its stderr line are deleted. The combination is no
  longer refused, so nothing is printed about it.

## Two things the branching walk needed that a linear one did not

**A read cache, per pass.** `verifyChainWalk.edges` holds one `bd dep list`
per BEAD per repo pass, so the bill is a function of the graph and not of how
many candidates' lineages run through it. The deepest fixture measures this:
at `verify_chain_max: 3` over a lineage batched four ways, the walk makes
**21 reads and 9 bd calls** — the leaf, four closes one level up, four the
level below that, each read once. 21 is the number ADR 0006 §3 prices the
shipped shape at (1 + 4 + 16), and the test asserts both the 9 and that no id
is read twice.

ASSUMED, and stated because it is a deviation from the implementation bead's
letter: the MEMO is of the reads, not of the counts. A count here depends on
the set of beads already on the path (the amendment says "over the sources
not yet seen"), so a path-blind count memo would have to drop the repeat
check — and then a lineage that loops back on itself counts to the bound and
escalates, which is an overcount, the one direction this rule refuses. The
bead's stated purpose for the memo — "so no bead's `bd dep list` is read
twice" — is met exactly, and measured above.

**A read budget, per candidate.** `verifyChainReadMax = 50` counts every
dependency list a candidate's walk reaches for, cache hits included, because
it bounds the WORK: the cache already bounds the bd calls, and an unbounded
re-walk of cached edges spends the launcher lock just as surely. Past it the
count is a floor.

**And the floor rule is now explicit.** While the walk was a line, a bd
failure left strictly fewer than `limit` hops by construction, so
`len(hops) >= limit` was itself proof that every edge had been read. That
reading is gone, so the walk returns whether the count is whole and the
caller escalates only on a whole one. A minimum is whole when some branch
that ATTAINS it is whole — every other branch's true count is at least what
it reported — so a tie between a whole branch and a floored one is whole.

It still holds structurally, by a longer route: a path is floored only where
it GAVE UP, a read that failed or the budget spent, and both need depth still
to spend, so a floored path is strictly shorter than `limit`. **No fixture
can red the caller's `whole` check on its own**, and that is said in the
code: it realizes the ADR's sentence for the change that breaks the argument
without noticing.

## The measurements, re-run

`scripts/chain-depth-census.py` now prints the MIN key as its first report
(LINEAGE is kept beside it, because the amendment rests on the two agreeing
about the closes that reach a bound) and takes `--replay`. Both ports come
from the two scripts in `docs/notes.d/ranger-base-rffxq.md`.

Over this instance's store, 3,368 beads (15 more than when rffxq measured),
2,647 of them not verify beads:

```
MIN      0: 2066  1: 521  2: 49  3: 8  4: 3   >=3: 11 (0.42%)
LINEAGE  0: 2066  1: 529  2: 41  3: 8  4: 3   >=3: 11 (0.42%)
FAN-OUT                                       >=3: 202 (7.63%)
```

The same eleven closes at or above 3 under either shipped key, the same
deepest of 4, and the same eleven ids rffxq names — so the default of 3 does
not move. FAN-OUT, the rejected repair, is 18x that.

`--replay`, which is where the GROUPING's cost is read:

```
batched verify beads: homogeneous 88 {0: 83, 1: 5}, mixed 264

1673 closes a verify bead answers, rounds (MIN) {0: 1146, 1: 469, 2: 47, 3: 8, 4: 3}
  verify_batch: 4  one group            verify beads  424  wait h median 0.3 p90  1.5
  verify_batch: 4  by rounds (SHIPPED)  verify beads  439  wait h median 0.5 p90  3.9
  verify_batch: 4  round 0 vs >=1       verify beads  430  wait h median 0.5 p90  3.2
  verify_batch: 4  deep unbatched       verify beads  818  wait h median 0.0 p90  1.7
```

The grouping costs **+3.5% verify beads** and takes the p90 wait from 1.5h to
3.9h; filing each deep close on its own costs +93%. At `verify_batch: 1`
every strategy files 1,673 and waits 0.0, which is the control: the grouping
is invisible where there is nothing to group.

The flaw rffxq's `sim.py` names is fixed rather than carried: its
"deep unbatched" row gave each deep close a group of its own and then waited
for a batch of four, so those closes flushed at the 24h age (median 1.0h, p90
24.0h). A group that cannot fill files on ARRIVAL, which is what `n_for`
expresses, and the row now reads 0.0/1.7 — its COUNT is unchanged, which is
the only number the ADR quotes from it.

## The guards, seen red

Four mutants, applied one at a time to a clean tree at af448c76, each
followed by `git checkout --` and a clean `git status`
(`-run 'VerifyChain|VerifyBatches'`, arm 3):

| mutant | what reds |
|---|---|
| the batched hop STOPS again (bu8rz's walk) | the six-round repro, the mixed-batch control, and the continues-through pin |
| the DEEPEST source instead of the shallowest | the mixed-batch control, alone |
| one group for every depth (the grouping deleted) | the like-with-like pin, alone |
| the read cache off | the six-round repro's read count |

The mixed-batch control is the only pin that tells the shipped rule from the
rejected one, which is why it names its sources deep-first: the walk
short-circuits on a source with no round above it, so a fixture that lists
the shallow source first would never compare two depths at all. It asserts
that both branches were read.

The like-with-like pin puts its round-1 close in the MIDDLE of the close
order for the same reason. Put it last and close order alone batches the four
round-0 closes and holds it, so the fixture passes with the grouping deleted.

## What this did not change

`TestVerifyChainStopsCountingAtABatchedVerifyBead` is
`TestVerifyChainContinuesThroughABatchedVerifyBeadsShallowestSource`, and it
escalates where it used to file. Every other test at `verify_batch: 1` and
with the bound off is untouched and green: the round line, the escalation
body, the title dedupe, the floor on a failed walk, the `0` that walks
nothing. A first-round filing's description is still byte for byte what this
rule has always written.
