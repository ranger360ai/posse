# The exact age two pins asserted was a statement about a clock they did not own (ranger-base-dsuk7)

MEASURED 2026-11-06 on this box (darwin 25.6.0). Two `posse status` QA pins in
`cmd/posse` seeded a plan-usage snapshot exactly three hours old and asserted
the age the binary rendered from it, to the minute and with no bracket:

- `planquiet_qa_test.go`, `TestQAStatusSaysTheMeterIsQuiet`: `"3h00m ago"`.
- `planmeterspend_qa_test.go`, `TestQAStatusSaysTheGuardIsUnarmedOverAnOldReading`:
  `"plan meter BLIND 3h00m"`.

Neither had been seen red. Both were one interruption away from it, for the
reason ranger-base-gt0q9 measured in the same package: the seed is a
wall-clock timestamp written by the test process and the age is rendered by
ANOTHER process out of `time.Now()`, while the test's own notion of elapsed
time is monotonic — and on darwin Go's monotonic clock is
`mach_absolute_time`, which does not tick while the box is asleep. This box
slept 2h51m02s at 1% battery inside one `cmd/posse` test on 2026-11-04 and a
sibling pin printed `2h54m` against a monotonic bracket of `[3m, 3m]`.
Independently of any clock jump, an exact age also reds on a launch slower
than one minute, in a package measured at 590.639s on a loaded box.

## Why not "stop the clock"

The bead offered it: seed the snapshot at an age the binary is TOLD about.
`posse status` reads `time.Now()` and nothing else — there is no clock
injection anywhere in the shipped binary, and the whole env surface is eight
variables (`grep -rn 'os.Getenv("RHQ_' cmd internal` over non-test sources:
`RHQ_HOME`, `RHQ_GATE_LOG`, `RHQ_BD_BIN`, `RHQ_TOOLS_DENY`, `RHQ_GH_BIN`,
`RHQ_HERDR_BIN`, `RHQ_PLAN_USAGE_URL`, `RHQ_CAGE`). Stopping the clock means
adding a ninth that a production binary believes about what time it is, so
that two tests can assert two strings. The fix is the bracket gt0q9 already
landed in this package.

## Why the bracket is not the weaker assertion

What `3h00m` pinned is that the line carries the age of the reading it is not
refreshing — `BlindFor` handed `now - At`. `agedAgesOf(at, quietSeedAge)`
names every age that is, from the write to the moment the test checks, one per
whole minute (`BlindFor` is minute-resolution), and anything outside it is
still red. The one thing the exact string caught and the bracket does not is a
binary that rendered a minute too much on a launch that really did take a
minute — the false positive, not a defect, and not a thing the test can tell
apart from the launch's own duration.

`skipIfTheBoxSleptMidTest` beside it is what keeps the bracket tight rather
than belt-and-braces: with a wall jump of over a minute skipped, the bracket
can only ever span the launch's own wall time, so it is two members wide at
worst. MEASURED on every run here: one member, `3h00m` — the same single
string the pin asserted before.

## What changed

- `costplan_test.go`: `agedAges(at)` is now `agedAgesOf(at, agedSeed)`, and
  `agedAgesOf` takes the seed age, because these two seed three hours and not
  `agedSeed`'s three and a half minutes. No behaviour change for the four
  existing callers.
- `planquiet_qa_test.go`: `quietSeedAge` (3h) and `seedAgedQuietSnapshot`,
  shared by both pins — seeds the snapshot and hands back the reference with
  its monotonic reading intact, which `skipIfTheBoxSleptMidTest` requires and
  the old `time.Now().UTC().Add(-3*time.Hour)` stripped. Plus `planLine`, a
  diagnostic: `posse status` in a fixture home prints forty lines of bd errors
  and sandbox warnings around the one sentence, and `wantOneOf` quotes what it
  is given with `%q`.
- The BLIND pin's control arm now asserts on the prefix `plan meter BLIND`
  rather than on `plan meter BLIND 3h00m`. An idle unarmed shop is QUIET and
  nothing there is stale at any age, so that is the claim — and it is stronger
  than what it replaced, which a BLIND line bearing any other age walked past.

## Three arms, each run

- The guard is reached and fires from both new call sites: tolerance
  temporarily negative, all five subtests SKIP with the measured jump printed.
- The bracket is not vacuous: the fixture temporarily seeded an hour older
  than the bracket's reference, both pins red naming `4h00m` against a bracket
  of exactly `["3h00m"]`.
- The BLIND control is not vacuous: a `budget_pass:` cap temporarily given to
  the idle row, which reds with `said it, want silence`. (Named without its
  value: a guard key with a threshold in a tracked markdown file is ops
  residue, and `make ops-check` is right to refuse it — the cap lives in
  planmeterspend_qa_test.go's own table, which that census does not read.)
