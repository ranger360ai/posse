# ranger-base-tq5tj — the delta needed a baseline, so the pin forbids a baseline

*2026-11-13 · dinesh · `internal/treepins/ambientcensus_qa_test.go`,
`scripts/reap-test-tmp.sh`'s finding-4 rig, `scripts/reap-selftest-ambient.sh`.*

Every number below is MEASURED on this box (linux 7.2.15 x86_64, 16 cores, uid
1001, `/tmp` a tmpfs, one-minute load average 2.0-2.9) on 2026-11-13, tree at
`a8575914`.

## The decision this bead held

Finding 4 of `reap-test-tmp.sh --self-test` asks whether a `claude -p` run is
excluded from the blindness control's live-claude count. That count is
BOX-WIDE on purpose, so a rig grading a DIFFERENCE of two such counts grades
whatever the shop did between its samples — which it did until
ranger-base-0wvu6 (`7725ba75`), reddening `make test` for seats that had
changed nothing. Six beads across four personas; the last two were each filed
by someone reading a red main had already fixed. ranger-base-0whbv left the
tree with the fix, a deterministic detector and **nothing asserting the fix
stays**, and tq5tj was filed to choose between:

- **(a)** door the detector (`scripts/reap-selftest-ambient.sh`) — behavioural,
  deterministic, and a cost every seat pays;
- **(b)** a shape pin over the finding-4 region — free, if the rule has a
  crisp spelling, which the bead said had not been found;
- **(c)** neither, prose only.

**(b) landed.** Three arms in `internal/treepins/ambientcensus_qa_test.go`.

## Why (a) was declined, with the number the bead rested on corrected

The bead priced (a) at "~25-35s plus 25-35 planted claude-exe processes per
run". That figure predates ranger-base-hxenz, which cut the self-test from
23-35s to 7-9s; the harness plants **one process per second of the run it
perturbs**, so both halves of the price fell with that wall. Re-measured here,
`--runs 3` each time:

| tree | verdict | wall/run | arrivals/run |
|---|---|---|---|
| `a8575914` (membership arms) | 3 of 3 green | 8s | 8 |
| `7725ba75^` (delta spelling) | 3 of 3 **RED** | 24-25s | 24-25 |

(`make verify-reap-test-tmp` itself: 9s, 7s, 7s over three runs. The pre-fix
column is slower because that tree predates hxenz, not because the ramp is.)

So (a) is three times cheaper than anyone in this family believed, and the
detector's positive control still holds at a third of the arrivals. It is
still not the pin, and the cost is only half the reason:

- 8s and 8 planted claude-exe processes on **every** seat's `make test`, to
  re-answer what the membership arms already answer for free. The fakes are
  visible to a sibling seat's reaper, which can only push that run toward
  REFUSING the claude-scratch class — the control's own fail-safe direction —
  but it is still an effect on seats that did not ask for one.
- A behavioural pin reds the **behaviour**, and the behaviour is already
  right. What was unprotected is the **edit**, and an edit is what each of the
  four re-discoveries began with. A shape pin reds that, in milliseconds, off
  one `os.ReadFile`.

## The rule, and why it is not a roster of variable names

The bead's objection to (b) was that no crisp shell spelling had been found:
the old verdicts named `$n0`/`$n1`, the new ones name `$pfake`, and the
region's first arm legitimately names no pid at all. Both of those are
spellings. The **property** is one step back:

> A delta needs a BASELINE — a value read from one census and still readable
> after a later one.

That is the one thing every spelling of this defect must do, and the one thing
the fixed spelling never does: each arm captures a census, derives its fields,
decides, and the next capture destroys them. So arm 1 asks nothing about names
and everything about program order — **no census-derived value is read across
a later census** — and a rename of any variable changes nothing.

A census is derived too, not spelled: a function in the region whose body
invokes `"$0"`. That is what lets the same rule read the pre-fix spelling,
whose census was `count_claude_procs` and not `claude_census`.

The reading is **may-hold, not must-hold**: a non-census assignment does not
clear the taint. The pre-fix code resets its baseline with `n0=0` on the
unreadable-census branch only, so on every other path `n0` still dates from
the census two arms back — and that is the path the verdict is wrong on. A
must-hold reading passes the broken spelling, which is how the first draft of
this pin was caught.

### The three arms

1. `TestQANoFindingFourVerdictReadsACensusValueAcrossALaterCensus` — the rule
   above, with non-vacuity guards (at least two censuses, at least one derived
   name: ranger-base-bttuj's lesson one layer down in these same arms).
2. `TestQAFindingFoursCensusRunsTheProductionCounterExactlyOnce` — one census
   is one run of the counter, so the count and the pid list are each other's
   evidence. Two runs inside one census are two populations and need no
   surviving name to be a delta.
3. `TestQAEachFindingFourFixtureArmGradesMembershipOfItsOwnPlantedPid` — each
   `start_fake`…`stop_fake` span decides through a `case` over the census's own
   pid list with a pattern naming a pid the rig planted; the only test in a
   span that may read the census COUNT is the unreadable-census sentinel; and
   no arithmetic or numeric comparison in a span touches a census value. The
   planted-pid names are derived from `$!` assignments, not spelled.

### Its own positive control

`TestQATheCrossCensusReadRuleRedsTheDeltaSpellingItWasWrittenFor` runs arm 1's
analysis over the `7725ba75^` region, embedded in the file. It must red, and it
reds on `n0` at both verdicts:

    line 32 reads n0 across the census at line 27 — if [ "$n1" = "$n0" ]
    line 46 reads n0 across the census at line 41 — if [ "$n2" = "$((n0 + 1))" ]

This is the arm that matters most, because a clean rerun of the target passes
the broken rig **8 times out of 8** on a quiet minute (ranger-base-0whbv's
table) — the evidence two beads in this family were closed on. A rule that
does not red the thing that red the shop is decoration.

End to end, the live arms were also run against a delta planted in
`scripts/reap-test-tmp.sh` itself (a `baseline=$cn` before the `-p` arm and a
`[ "$cn" -eq "$((baseline + 1))" ]` in the control). Arms 1 and 3 both red it,
naming `scripts/reap-test-tmp.sh:681` and the line; the probe was reverted with
`git restore`.

## Not tree-wide, so no door row

Both pins read one named file with `os.ReadFile` and enumerate no directory, so
neither is in the class `treewidedoor_qa_test.go` derives — whose own negative
case is `os.ReadFile("Makefile")` — and neither needs a Makefile door row. They
run in arm 1's `./...`, where `internal/treepins` already runs whole.

## What this does not reach

A verdict that reads the box **without going through the rig's census function
at all** — a `pgrep -c claude` spelled straight into an arm. Arm 3 covers the
fixture arms, whose verdict must be a membership test over the census's own pid
list; beyond that, the harness is the answer. `scripts/reap-selftest-ambient.sh`
and the Makefile's `verify-reap-test-tmp` comment both say so and both name the
pin now, with the corrected price.

Nothing pins against the other half of this family's cost — filing a bead from
a stale tree, which is how two of the six arrived. A pin reds an edit; it
cannot red a report.
