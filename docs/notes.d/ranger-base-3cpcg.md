# ranger-base-3cpcg — the treepins door is the package minus a priced table

ADR 0098 D1-D3, implemented. `internal/treepins`' tree-wide pins used to be
reached by six `-run` doors on `make tree-check`'s line, and a pin none of
those six named was a pin no door ran. That key had just produced its own
verdict: nine hand-written exemptions in `twdDoorHolders`, seven of them for
one cause — a pin whose subject is a tracked file it reads BY NAME, which no
derivation over Go source can see. The door is now the complement:

    go test ./internal/treepins -timeout 15m -count=1 \
      -skip '^($(QA_TREEPINS_EXCLUDED))$'

Twelve names on that line, each with a priced row in
`twdTreepinsExcluded`. A pin written tomorrow is doored the moment it is
written.

2026-11-14.

## What landed

| file | what |
| --- | --- |
| `Makefile` | `QA_TREEPINS_EXCLUDED`, the `treepins-check` target, `.PHONY`, and `tree-check`'s prerequisite line |
| `internal/treepins/treewidedoor_qa_test.go` | `twdTreepinsExcluded` + its floor and row type, arm 5, arm 1's covered-door rule, the head comment |
| `internal/treepins/README.md` | the door table (held to that line by arm 4) and the six subset doors |
| `AGENTS.md` | the door block (held the same way) and what the complement door is |

`tree-check`'s prerequisites went from nineteen to fourteen: `treepins-check`
replaces `notes-check`, `orientation-check`, `adr-check`, `corpus-check`,
`register-check` and `scripts-check`, all six still targets a seat can type,
each a subset of the one that replaced them. `pid-check` stays on the line as
well — its two pins measure 0.44s and 0.00s on this box, so they are inside
the complement too and the door above them is a second, cheaper way to ask
the same question. (Its own Makefile comment prices it at ~10s. That is not
this box's price, and re-pricing it was not this bead.)

## The exclusion table, measured

`scripts/test-times.sh go test ./internal/treepins -timeout 25m -count=1
-json`, twice, D1 host, Linux 7.2.15-1-omarchy, go1.27.3, 16 cores, warm
build cache, both suite slots free at the start of each.

| run | wall | summed test time | load (1m) start → end |
| --- | --- | --- | --- |
| 1 | 146s | 238.0s | 3.17 → 8.11 |
| 2 | 137s | 167.3s | 5.37 → 5.83 |

481 top-level tests in both. **The twelve tests at or above the 2.0s floor
with no door of their own are the same twelve in both readings**, which is
what the table needed before it could be written: one reading cannot tell a
price from a mood.

| test | run 1 | run 2 (the row) |
| --- | --- | --- |
| `TestQATheQueuedArmsFailOnEvidenceAndNotOnASlowFork` | 25.26s | 25.20s |
| `TestQATheSelfTestsDiskArmsDoNotContradictEachOtherWhenAForkFails` | 11.60s | 11.78s |
| `TestQASuiteLockStrictArmFailsWhenTheQueuedPathKillsTheWrapper` | 6.10s | 6.11s |
| `TestQATheWrapperStillExplainsAFullDiskAndABrokenStdBuild` | 5.78s | 5.98s |
| `TestQASuiteLockSelfTestIgnoresAnAmbientPosseEnvironment` | 3.86s | 4.06s |
| `TestQATheFilteredRunArmFailsOnEvidenceAndNotOnATimeout` | 4.05s | 4.03s |
| `TestQATheSuiteQueueStillProvesItQueues` | 4.05s | 4.01s |
| `TestQATheSuiteQueueSelfTestCanFail` | 3.01s | 3.00s |
| `TestNoTrackedFileCarriesADigitRealisticDetectorShape` | 3.00s | 2.95s |
| `TestQAEveryRaceArmCandidateTestIsCompiledIntoSomeArm` | **27.91s** | 2.91s |
| `TestQATheRaceArmCensusRefusesTheWrongShapes` | **27.72s** | 2.85s |
| `TestQABdArgvGateResolvesTheVerb` | 2.56s | 2.39s |
| total | 124.9s | 75.3s |

The two bold rows are the race-arm census, which compiles `internal/posse`'s
three arms: run 1 paid for those builds and run 2 read them out of the cache.
**The rows carry the warm figures on purpose** — a row has to clear the floor
at the test's cheapest measured price, not at its most expensive, or the floor
is doing no work.

Ten of the twelve are the suite apparatus testing itself: `suite-lock.sh`'s
queue and strict arms, `test-times.sh`'s and `gotest.sh`'s self-tests and
their disk and fork arms, and that race-arm census. Each forks a real wrapper
or a real `go`, which is where the seconds go. The other two are the
tracked-file digit census and the bd argv gate's verb arm. ADR 0098's Context
read fifteen tests and 172s at `37a94cd9`; the membership is the same family
and the difference is the cold arm builds plus the landings in between.

Nearest test under the floor: `TestQATheWrapperSelfTestCanFail` at 1.56s.
Most expensive test the door still RUNS:
`TestQATheTreeWideDoorsReportRealDrift` at 22.2s — the drift arm is the
strongest thing the register does, so it stays inside the door rather than
being priced out of it. What ADR 0098 bought was a door over the pins nobody
names, not a cheaper register.

## `make tree-check` before and after

Three warm runs of each, same box, same window, `before` being this tree with
`tree-check`'s old nineteen-door prerequisite line restored, so neither half
is measured against a different day's load.

**`before` cannot be measured by swapping the prerequisite line.** Arm 5 pins
`treepins-check` onto `tree-check`'s line, so a tree carrying the new register
and the old line reds at ~40s — which is the arm working (it is this bead's
own M7 mutation), and it is also why the first battery's `before` column is
four RED lines. The old door SET is what gets measured instead, named as one
`make` invocation over the nineteen targets, with the Makefile untouched. One
serial invocation is what that line was, so the two columns are the same kind
of thing.

MEASURED 2026-11-14, D1 host, Linux 7.2.15-1-omarchy, go1.27.3, 16 cores,
warm build cache, **no suite slot held by anyone at any point in the
battery**, one warm-up run discarded before each set:

| command | three warm runs | load (1m) per run |
| --- | --- | --- |
| `make tree-check`, nineteen doors (**before**) | 42.4s, 43.0s, 42.0s | 7.86, 6.39, 6.36 |
| `make tree-check`, fourteen doors (**after**) | 91.5s, 83.1s, 91.0s | 12.53, 11.76, 7.01 |
| `make treepins-check` alone | 85.3s, 69.6s, 67.7s | 9.99, 17.56, 10.28 |
| the six doors it replaces, one invocation | 29.2s, 29.6s, 29.2s | 7.97, 6.71, 5.45 |

So **+40 to +49s on `tree-check`**, and the figure is internally consistent:
the thirteen doors that stay are 42.0s − 29.2s ≈ 13s, and 13s + 68-85s is the
83-91s the after column reads. ADR 0098 predicted about +36s from one reading
of each; the shape of the answer is the same and the number is this box's.

A fourth reading, taken as this landed rather than under the protocol above:
**78s** for the whole of `make tree-check` at a one-minute load average of
2.58 falling to 3.10 — the quietest box any reading here was taken on, and
`treepins-check` was 63.0s of it. That is five seconds under the fastest of
the three above and its load is a quarter of theirs, which is this chain's
standing lesson one more time.

The six-door bundle is the steadiest reading in this fragment — a 0.45s spread
over three runs — and the complement is the widest, 17.6s, which is the thing
every wall reading in this repository says: what moves is the load, and the
widest spread sits on the run whose load average moved from 10.0 to 17.6 and
back.

## Does it need a suite slot?

**No, and ADR 0098's assumption stands — with a corrected ratio.** The
Consequences section ASSUMED about 1.6 cores and asked the implementation to
measure it under load and correct it if wrong.

| run | wall | user+sys | cores | load (1m) |
| --- | --- | --- | --- | --- |
| 1 | 85.3s | 146.3 + 25.8 = 172.1s | 2.02 | 9.99 |
| 2 | 69.6s | 129.6 + 25.8 = 155.4s | 2.23 | 17.56 |
| 3 | 67.7s | 125.1 + 26.9 = 152.0s | 2.25 | 10.28 |

**2.0-2.25 cores of sixteen**, the busiest reading taken at a one-minute load
average of 17.6 — the battery's own serial runs plus whatever the sibling
seats were doing, and `scripts/suite-lock.sh --status` reported both slots
free on every line. That is a seventh of the box for seventy seconds, against
a queued arm's median of 393.5s and most of it. It is not a suite by cost and
it takes no slot.

The 2.0-2.25 figure is **process CPU** — `time`'s user+sys over the whole
`make` subtree, so it counts `go build`, `make` itself and the drift arm's
four forked doors. ADR 0098's 1.6 was summed TEST time over wall, a different
quantity measured a different way (the complement's summed test time in this
bead's reading 2 is 92.0s, which over 69.6s of wall is 1.32). Both are
honest; neither changes the verdict. The ADR's number is left as it was — it
is a dated reading of its own metric, and the decision it supports holds.

## Arm 5, and that it can fail

The complement buys one property and pays for it with a list of names that
silence the door. Nothing else in the register can see that list: arm 1 reads
what a door RUNS, arm 2 reads what a door NAMES, and a test added to
`$(QA_TREEPINS_EXCLUDED)` is named by nothing and run by nothing — which is
ranger-base-ik44f arriving again through the one gap the new spelling opens.
So `TestQATheTreepinsComplementDoorIsThePackageMinusItsPricedTable` holds the
table four ways (D2, D3), and the door's shape besides. It was MEASURED by
mutation rather than asserted — the battery edits `Makefile` and the register
in place, runs the one pin, and restores both:

| mutation | verdict |
| --- | --- |
| control, unmutated | PASS |
| a name dropped from the Makefile variable, still priced | FAIL — set mismatch |
| a row priced at 1.39s, under the floor | FAIL — names the floor and the row |
| a row's route changed to `make adr-check`, which skips it | FAIL — the route is false |
| a row's route changed to `make nosuchdoor-check` | FAIL — no door has that target |
| a DERIVED test excluded, with its own door (D3's legal case) | PASS, and logs the door |
| the same exclusion with its door taken away | FAIL — D3 |
| the recipe's `-skip` narrowed to one hand-spelled name | FAIL — the door and the table disagree |
| `treepins-check` off `tree-check`'s line | FAIL |

And arm 1's new rule, which is DERIVED rather than listed — a treepins door
whose every pin is outside the table is a subset of the complement, so
`tree-check` reaching `treepins-check` is what reaches it:

| mutation | verdict |
| --- | --- |
| `treepins-check` off the line | FAIL, once per covered door — six of them |
| `adr-check`'s first pin priced into the table, with `adr-check` off the line | FAIL, naming `adr-check` |

A listed answer would have been the failure this file keeps finding: a row
reading "covered" for a door whose pin was priced into the table later, and a
prerequisite line a seat reads as complete while one pin is a package-tree run
away.

## The acceptance criterion, run

The bead's DONE WHEN was "a new undoored test planted in internal/treepins
runs under treepins-check without any register edit". MEASURED 2026-11-14 by
planting `internal/treepins/twd_plant_probe_test.go` — one failing test that
no door variable names and no derivation reaches — making no edit to the
Makefile or the register, and then running the doors:

| command | verdict |
| --- | --- |
| `make treepins-check` | **RED in 63.2s, naming `TestPlantProbeNobodyDoors`** |
| `make notes-check` | green |
| `make orientation-check` | green |
| `make adr-check` | green |
| `make corpus-check` | green |
| `make register-check` | green |
| `make scripts-check` | green |

Six green doors over a red tree is the escape ADR 0098 closes, and it is the
shape every one of those doors would have had before this bead: a `-run`
filter names tests, and nothing named the plant. The plant was removed
afterwards; this fragment and the battery scripts are the only record of it.

## Residuals

- `twdDoorHolders`' nine rows are still there. Retiring them for this package
  is ADR 0098 D4 and ranger-base-gyz7a, which this bead blocks. Under the
  complement they are no longer load-bearing for membership — only for arm
  2's two-way check, which still reads the old key.
- The exclusion table is the only hand-kept state ADR 0098 leaves, and its
  exit hatch is the floor: raising it makes the door cheaper and the escape
  window wider, one row at a time.
- `scripts/posse-landed-check` derives its body from `make test`'s
  prerequisites, so the landed judge picked this up with no edit. Its price
  goes up by the same net figure as `tree-check`'s, against ADR 0088 D6.2's
  240s ceiling; the headroom is in that record and in
  `docs/notes.d/ranger-base-r3pqf.md`, not here.
