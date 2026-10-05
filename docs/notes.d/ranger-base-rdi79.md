# A cancelled JOB is the queue too (ranger-base-rdi79)

`ciVerdict` (`internal/posse/ciwatch.go`) read the RUN-level conclusion and
nothing else, and GitHub applies `cancelled` at the JOB level as well. A run
whose six jobs are five greens and one job that never got a runner concludes
`failure`, so ci-watch filed **ranger-base-94grm** at P1 on 2026-10-05 and
dispatched a session over `bab60e51`, which nothing was wrong with. The
per-job table, the `BlobNotFound` log blob and the re-run are in
`docs/notes.d/ranger-base-94grm.md`; this fragment is the fix and its
measurement.

## The rule that shipped

`ciJobsSayQueue`, asked only of a `failure` that is **about to be the
verdict** — the head of the sorted page, and only while it is red. Over the
jobs of the run's latest attempt:

| job conclusion | reading |
|---|---|
| `failure`, `timed_out`, `startup_failure` | a real red; the run stands |
| `success`, `skipped`, `neutral` | not the cause; ignored |
| `cancelled`, 0 steps, no `runner_name` | never started: the queue |
| `cancelled` with steps or a runner | stopped after working: the run stands |
| anything else, or a job not `completed` | the run stands |

A run is set aside only when **every** non-success job is of the fourth kind
and there is at least one of them. Demotion takes positive evidence; an
unreadable page, a paginated page (`total_count` past the rows), an empty
page, an unknown conclusion and a `failure` no job accounts for all leave the
run RED. That direction is the whole safety of it: a false P1 costs one
dispatched session and leaves a bead saying so, while a suppressed genuine
red is this mechanism's founding incident (191 reds over five days, nobody
looked) and leaves no trace in the store, on stderr or in `posse status`.

A set-aside run lands in `CIState.QueueOnly` with the sentence that said so,
and a bead filed while one sits above the verdict names it — the bead's own
`Reproduce:` block sends a seat to a `gh run list` whose top row is a red run
the bead is deliberately not about.

## What it costs

`ciJobsVetCap = 3` runs per reading, `ciJobsReadTimeout = 10s` each. The call
is made only at a red head, so a green pass forks nothing (MEASURED: a live
`ReadCI` against this repo on 2026-10-05 read green in 4.54s with zero jobs
calls) and an ordinary red pass forks one. The cap is what a pathological
page costs — a queue broken for everybody tops the list with phantom after
phantom, and walking 100 of them would put 100 children on a dispatch pass.
Past the cap the run stands as `gh run list` reported it.

## MEASURED 2026-10-05

`ci.yml` on `main`, the 300 runs gh serves, 2026-09-06T14:56:19Z to
2026-10-05T19:21:00Z: **244 success, 55 failure, 1 in flight**. All 55
failures carry a job whose conclusion is `failure` — 109 such jobs, **100
with 11 executed steps and 9 with 9** — so this rule demotes none of them and
the red half of the gate is unchanged. Across all 302 jobs of those 55 runs
the only two conclusions in the window are `success` (193) and `failure`
(109), and **every page was complete** (`total_count == len(jobs)`, no
pagination). The episode is still the first of its class in 300 runs.

Re-run it (the counts drift as the window slides; the shape is the claim):

```
gh run list --repo ranger360ai/posse --workflow=ci.yml --branch main \
  --limit 300 --json conclusion,status,databaseId,createdAt,headSha
# then, per failure run id:
gh api "repos/ranger360ai/posse/actions/runs/<id>/jobs?per_page=100"
# classify: any job conclusion in (failure, timed_out, startup_failure) → a
# real red; every non-success job `cancelled` with 0 steps and no runner → the
# queue.
```

**The first page that command handed back was seven days stale** — newest run
2026-09-28T12:59:56Z, 300 runs ending a week back — and three later calls all
answered the current window. That is ranger-base-m46kr's index lag again,
hit by accident twice now while measuring this gate (94grm's notes have the
other). A census that does not print the window it actually counted is
counting whatever the index felt like serving.

## `filter=latest`, not `filter=all`

The shipped reader asks `/actions/runs/<id>/jobs?per_page=100` and takes the
default filter, which is the **latest attempt's** jobs — and the latest
attempt is what the run's conclusion is about. MEASURED 2026-10-05: run
37362765576 after its re-run answers 6 jobs, all `success`, under the
default, and 12 under `filter=all`, attempt 1's cancelled row among them.
`filter=all` would hold a re-run run red on the strength of the attempt it
was re-run to replace. 94grm's forensics used `filter=all` deliberately,
which is the one place that fragment's commands and the shipped reading
differ.

## Scoped to the head, deliberately

Runs **below** the head keep their conclusions as gh reported them, so a
phantom failure buried inside a red streak still counts toward `Streak` and
can still be `Since`. That is a number slightly too large on a bead that was
going to be filed anyway, against one `gh` child per run of the streak to
fix — the founding incident's own streak was 191. The false P1 this bead is
about is a reading, not a count.

## Pins

`internal/posse/ciwatch_test.go` (arm 1): the rule as a table over jobs pages
(`TestCIJobsSayQueueDemandsPositiveEvidence`), the episode end to end through
the shipped argv, a real red untouched, an unreadable page leaving the run
red, no jobs call on a green pass or a stopped run, the cap, the
all-set-aside abstention, and the set-aside block in the bead description.
`ciwatch_qa_test.go`'s neighbour at the pass level:
`TestCIWatchFilesNothingOverAFailureWhoseRedJobNeverRanAndStillFilesARealOne`
— no bead, no session, nothing on stderr, and then a real red files one.

`ciwatch_live_test.go` (arm 3, `TestLiveCIJobsVet`) is the half the fakes
cannot pin: that `ciJobsPage` parses the bytes GitHub actually sends. It
reads **attempt 1 of 37362765576** — the episode's own payload, reachable
only through the attempts path now that the run was re-run green — and
asserts the rule says "the queue" over it, then reads the ordinary red
**34632088265** (2026-09-11, `904059b2`, two `failure` jobs with 11 steps
each) through the shipped `ghRunJobs` and asserts it says "a verdict". Both
run ids are fixed on purpose: a live pin keyed on "whatever is red right now"
asserts nothing on the days main is green, which is most days. It needs
`GH_TOKEN` and `-count=1` for reasons its own header measures — the temp
`$HOME` hides gh's credential, and go's test cache does not key on an env var
no Go code reads.
