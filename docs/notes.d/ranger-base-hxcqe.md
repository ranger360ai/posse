# The red gate was a GitHub outage, and the third run was the tell (ranger-base-hxcqe)

ci-watch filed ranger-base-hxcqe at P1 on 2026-10-05T20:49:44Z: `ci.yml` red on
`main` at `eebe9737`, streak 1. It is the same class as
[ranger-base-94grm](ranger-base-94grm.md) — a run whose red jobs never ran —
and the two were 47 minutes apart. 94grm's fragment calls its episode "the
FIRST of its class in 300 runs". This is the second and the third, and the
reason they came in a cluster is the thing neither earlier bead could see from
one run: **GitHub Actions was in a declared major outage the whole time.**

## Look outward only after ruling out inward

In order, and all three before reaching for the status page:

- Nothing in this tree cancels a workflow run.
  `grep -rn -E 'run cancel|actions/runs/[^ ]*cancel|gh run rerun' --include='*.go'
  --include='*.sh' --include='*.py' --include='*.yml' --include='*.md' .` finds
  exactly one hit, and it is 94grm's fragment quoting `gh run rerun` as a
  remedy.
- `ci.yml` sets no `timeout-minutes`, at job or step level, so the 15-minute
  cut is not a budget this repo wrote.
- The concurrency group for a push is `ci-${{ github.sha }}` — a group of one
  (deliberately, ranger-base-sfb3), which supersedes nothing and cancels
  nothing.

Then:

```
curl -sS https://www.githubstatus.com/api/v2/summary.json
```

`"Actions": "major_outage"`. Incident **"Incident with Actions"**, impact
`critical`, created `2026-10-05T19:11:58Z`, still `investigating` at its
`20:47:22Z` update. The `20:39:27Z` update is GitHub naming this exact failure:

> We're continuing to investigate job failures and delays affecting
> GitHub-hosted runner assignment and workflow start times.

## The timeline is the incident's

| time (UTC) | run | head | starved jobs | cut at |
|---|---|---|---|---|
| 19:11:58 | — | — | incident opens | — |
| 19:21:00 | [37362765576](https://github.com/ranger360ai/posse/actions/runs/37362765576) | `bab60e51` | 1 of 6 | 15m00s |
| 20:08:21 | [37367869180](https://github.com/ranger360ai/posse/actions/runs/37367869180) | `eebe9737` | 3 of 6 | 15m03s |
| 20:50:06 | [37372180663](https://github.com/ranger360ai/posse/actions/runs/37372180663) | `64f18288` | see below | — |

Two cuts at fifteen minutes to the second is a mechanism rather than a
coincidence, and it is GitHub's. Nothing was wrong with `bab60e51`,
`eebe9737` or `64f18288`.

## Run 37367869180, the one this bead names

MEASURED 2026-10-05 from `/actions/runs/37367869180/jobs?per_page=100`,
`total_count` 6 and six rows, so the page is complete:

| job | conclusion | steps | runner_name | started → completed |
|---|---|---|---|---|
| test (macos-latest, 1) | success | 11 | assigned | 20:08:32 → 20:16:20 |
| test (macos-latest, 2) | success | 11 | assigned | 20:08:32 → 20:13:25 |
| test (ubuntu-latest, 2) | success | 11 | assigned | 20:14:38 → 20:16:40 |
| **test (ubuntu-latest, 1)** | **cancelled** | **0** | **`""`** | 20:08:21 → 20:23:24 |
| **test (ubuntu-latest, 3)** | **cancelled** | **0** | **`""`** | 20:08:21 → 20:23:24 |
| **test (macos-latest, 3)** | **cancelled** | **0** | **`""`** | 20:08:21 → 20:23:25 |

No job of the six carries `failure`, `timed_out` or `startup_failure`. The
run-level conclusion is `failure` because three of six are not `success`.

**The scarce pool was `ubuntu-latest`, which is backwards from the usual.**
Queue waits in this run: macos 1 and macos 2 got a runner in 11s, ubuntu 2 in
6m17s, and ubuntu 1 and ubuntu 3 never did. 94grm's single starved job was
also ubuntu. Whatever GitHub was mitigating, it was not the macOS pool that
a free public repo normally has to wait for — so "the slow pool is the one
you expect" is not a diagnosis, and reading the per-job `started_at` is.

What the commit actually lost: arm 3 ran on **neither** platform, and arm 1
ran on macos only (where it is also the job that carries the gates and the
silent-revert audit, and it passed, 437s). `ci.yml`'s own header answers the
coverage question — fast-forward-only `main` means an earlier commit's
CONTENT is covered at a later green tip, so nothing is unverified forever.

## Why the remedy was not 94grm's remedy

94grm's was `gh run rerun --failed`, and it worked: its one starved job
queued 19:42:43Z, got a runner 19:53:23Z and ran green in 123s. That margin
was thin and it was inside the same incident.

Re-running three jobs here, with main's tip run also in flight and starving,
would have queued six jobs into a pool GitHub was saying it could not assign
from, and the three re-runs would have competed with the tip's for whatever
came free. The tip is the run the bead's DONE WHEN turns on. **Inside a
declared Actions incident the move is to wait for the incident and then
re-run**, not to re-run into it.

## What this exposed in the reading

ranger-base-rdi79's `ciJobsSayQueue` landed at `64f18288` — pushed at
20:50:06Z, **22 seconds after this bead was filed at 20:49:44Z**. The
pre-fix `ciVerdict` read the run-level `failure` and filed; the shipped rule
reads this run's jobs as queue-only and gives it no verdict. So the fix was
right and this bead is the last false P1 of its class rather than a counter-
example to it.

What the outage did expose is the sizing. `ciJobsVetCap = 3`, and its doc
comment imagines precisely this page — "a queue broken for everybody tops the
list with phantom failure after phantom failure" — then lets the 4th
consecutive phantom STAND as the verdict, which asserts the opposite of the
sentence that justifies the cap ("three in a row is already a fact about
GitHub rather than about this branch"). rdi79's census sized the class at 1
run in 300; inside the outage it was 2 of the next 2. Filed as
**ranger-base-r5ksj**.

Not filed, because `ci.yml` already answers it: a run set aside on an
otherwise-GREEN gate leaves no trace anywhere — `CIState.QueueOnly` is read
only on the red and abstain paths, so no bead is filed and nothing names the
commit whose arms never ran. That is a real hole in attribution and it is the
one `ci.yml`'s fast-forward-only argument covers at the next green tip.
