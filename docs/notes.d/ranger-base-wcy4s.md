# ranger-base-wcy4s — the launch verify's cost, measured as work instead of wall clock

`TestVerifyPromotedCostIsNegligible` (internal/posse/promote_test.go) is the
measurement ADR 0015 asks for on the row "launch-time hashing of the promoted
set is negligible". It used to assert that reading, and that was the defect:
the statistic was **worst of 20 wall-clock readings against a 250ms bound**,
taken inside a test binary whose other hundreds of tests are `t.Parallel`. A
maximum is the one estimator a single descheduling moves, so the pin read the
scheduler.

## What was observed (not by me — ranger-base-wcy4s's filing)

MEASURED 2026-10-02, gwart worktree, tree aa9190ca, darwin 25.4.0, go1.26.5,
one `make test` sharing the box with another suite. promote_test.go carries no
build tag, so that one invocation ran this test three times over an identical
tree:

| arm | package result | this test |
|---|---|---|
| arm 1 | ok 1034.603s | passed |
| arm 2 | ok 881.933s | passed |
| arm 3 | FAIL 712.476s | **FAILED at 348.6ms** |

Two greens and a red from the same code. Same class as ranger-base-1jgch
(open, a different test and a different remedy), ranger-base-ze9p and
ranger-base-5i4c, under the ranger-base-2jl5 precedent whose lesson is that
the fix is not a bigger number.

## The reading that replaced it

`promoteWork` (internal/posse/promote.go) is what one promoted-set walk DID:
regular files opened, bytes read through the hash. `hashPromotedSet` tallies
it, `VerifyPromoted` carries its own walk's tally on the verdict (unexported
`work` — it is a diagnostic, never a verdict), and `HashPromotedSet` keeps its
exported signature over a throwaway tally.

The pin now asserts, every iteration: **one verify opens each regular file the
manifest names exactly once and reads exactly its bytes.** That is what the
two regressions the test was always for actually change — hashing per launch
in a loop multiplies `Files`, reading the whole tree twice doubles both — and
neither count moves with the box's load.

The clock stays, reported and loosely bounded, for the blowup no read count can
see (an O(n²) compare, a per-file exec, a sleep): the **median** of 20 under a
**1s** ceiling. For the median to breach it the box must be slow for the whole
test, where one unlucky iteration in twenty used to be enough.

## Numbers

MEASURED 2026-10-02, this worktree, darwin 25.4.0 / go1.26.5, 8 cores,
`go test -run '^TestVerifyPromotedCostIsNegligible$' -count=5`, fixture of 121
files / ~1205KB (twice the live constitution's size):

| box state | median of 20 | worst of 20 |
|---|---|---|
| loadavg ~70, old pin | — | 47.8 / 58.7 / 59.9 / 66.4 / 120.1ms |
| loadavg ~48-70, new pin | 11.0 / 11.6 / 12.3 / 12.7 / 21.8ms | 19.7 - 40.4ms |

Work, all 100 iterations: 121 files / 1234200 bytes per verify, exactly.

The margin that was lost is visible in the first row: the old bound was 250ms
and a single reading on a merely busy box already reached 120ms — 2x of margin
on the one statistic contention cannot average out. The 1s ceiling sits ~50x
above the median.

## Mutation readings

Both arms were made to fire, and the mutants reverted:

- a second `hashPromotedSet` inside one `VerifyPromoted` ("reading the whole
  tree twice") → `one verify read 242 files / 2468400 bytes, want 121 /
  1234200`, red in 0.09s on iteration 1 of 20.
- `time.Sleep(1100 * time.Millisecond)` before `v.Elapsed` is stamped (a cost
  with no reads behind it) → `median of 1.130439792s ... ceiling 1s`, red.
