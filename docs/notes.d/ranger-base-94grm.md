# A red gate whose red job never ran (ranger-base-94grm)

ci-watch filed ranger-base-94grm at P1 on 2026-10-05T19:21Z: `ci.yml` red on
`main` at `bab60e51`, streak 1. The gate WAS red and the reading WAS current —
ranger-base-m46kr's freshness guard had landed hours earlier and this page was
not stale. Nothing about `bab60e51` failed.

## What the run said

Run 37362765576, attempt 1, six jobs (MEASURED 2026-10-05 from
`/actions/runs/37362765576/jobs?filter=all`):

| job | conclusion | steps | runner_name | started → completed |
|---|---|---|---|---|
| test (macos-latest, 1) | success | 11 | assigned | 19:26:17 → 19:36:02 |
| test (macos-latest, 2) | success | 11 | assigned | 19:21:19 → 19:26:50 |
| test (macos-latest, 3) | success | 11 | assigned | 19:26:23 → 19:34:11 |
| test (ubuntu-latest, 1) | success | 11 | assigned | 19:26:05 → 19:31:10 |
| test (ubuntu-latest, 2) | success | 11 | assigned | 19:21:31 → 19:23:30 |
| **test (ubuntu-latest, 3)** | **cancelled** | **0** | **`""`** | 19:21:01 → 19:36:03 |

The run-level conclusion is `failure` because one job of six is not
`success`. That job never started: `started_at == created_at`, zero steps,
empty `runner_name`, and no log blob at all —

```
gh api repos/ranger360ai/posse/actions/jobs/111941056422/logs
→ BlobNotFound, HTTP 404
```

so the first move the bead template prints, `gh run view <id> --log-failed`,
answers with an **empty string**: no test named, no step named, nothing to
read. Fifteen minutes queued, then GitHub cancelled it one second after the
last running job finished. Arm 3's CONTENT passed on `macos-latest` in the
same run, so the only thing missing from the commit's verdict was one
platform's reading of one arm.

**This is the diagnosis to reach for when `--log-failed` prints nothing**, and
it is cheap: a job with zero steps and no `runner_name` never ran, and a job
that never ran is a statement about the queue.

## It is the first of its class in 300 runs

MEASURED 2026-10-05 over `ci.yml` on `main`, runs from 2026-09-06T14:56:19Z
to 2026-10-05T19:21:00Z — 300 runs, 244 `success`, 55 `failure`, 1 in flight.
Every one of the 55 failures carried at least one job whose conclusion was
`failure` with 11 executed steps (109 such jobs across the 55); the only
non-success job conclusion in the whole window is `failure`. Zero were caused
by a job that never started. Re-run it:

```
gh run list --repo ranger360ai/posse --workflow=ci.yml --branch main \
  --limit 300 --json conclusion,databaseId,createdAt
# then, per failure run id:
gh api "repos/ranger360ai/posse/actions/runs/<id>/jobs?filter=all&per_page=100"
# classify: any job conclusion in (failure, timed_out, startup_failure) → a
# real red; a sole `cancelled` job with len(steps)==0 → the queue.
```

So the remedy is a re-run and not a fix, and the frequency says so too.

## The gap this walked through

`ciVerdict` (`internal/posse/ciwatch.go`) reads the RUN-level conclusion and
nothing else. Its own doc comment states the rule this episode needed:

> `cancelled` is the case this function exists for. GitHub stops a run for
> reasons that are about the QUEUE and not about the code […] A stopped run
> has no verdict, so it gets none.

A cancelled RUN is skipped. A cancelled JOB that reddens an otherwise green
run is not — the run-level conclusion is `failure`, which `ciVerdict` reads
as red, so the mechanism filed a P1 and dispatched a session over a commit
that nothing was wrong with. The cost is the one ci-watch exists to avoid,
paid in the other direction: not an unread red, but a read one that was not a
verdict. Carried as **ranger-base-rdi79** with the shape of the fix — the
extra `/jobs` call is affordable because it is needed only on a reading that
is about to FILE, which is 55 readings in 300 and not one per pass.

## The remedy, and what it cost

```
gh run rerun 37362765576 --repo ranger360ai/posse --failed
```

Blast radius: one read-only runner job on a public repo — no token, no
artifact, nothing deployed or published; the only thing it changes is the
run's conclusion. Two operational facts worth keeping:

- **`--failed` does re-run a `cancelled` job.** GitHub's
  `rerun-failed-jobs` endpoint counts it as failed, and it prints NOTHING on
  success (empty stdout, exit 0) — read the run back rather than the command.
- **The new attempt does not appear immediately.** For ~70s the run read
  `status=queued, conclusion=null, run_attempt=1` and
  `/jobs?filter=latest` returned the ATTEMPT 1 rows. `run_attempt` went to 2
  before the attempt-2 job row existed.

Attempt 2's `test (ubuntu-latest, 3)` queued 19:42:43Z, got a runner
19:53:23Z — **10m40s queued**, against 15m00s before attempt 1 was cancelled,
so the starvation was real and the margin was thin — and ran green in 123s
(19:53:23 → 19:55:26). Run 37362765576 is `completed success`, attempt 2,
`head_sha bab60e51`, which is `main`'s tip.

## A stale list page, observed live

Between two `gh run list --workflow=ci.yml --branch main` calls 30 seconds
apart, the first returned a page whose newest run was `1b8ffe2d` from
2026-09-29T03:09:48Z — six days old — and the second returned `bab60e51`
at 2026-10-05T19:21:00Z. Same command, same session, same credential. That is
ranger-base-m46kr's index-lag, reproduced by accident while verifying this
fix, and it is why the verification here reads
`/actions/runs/<id>` directly: a run id resolves to that run, and a LIST page
resolves to whatever the index has. The freshness guard would have abstained
on the stale page rather than reporting the 09-29 green, which is the behavior
it was built for.
