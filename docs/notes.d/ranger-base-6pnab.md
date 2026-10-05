# The gate had been green for an hour and fifty-two minutes when this bead was filed, and the filer was 101 commits behind (ranger-base-6pnab)

ci-watch filed ranger-base-6pnab at P1 on 2026-10-05T23:30:38Z: `ci.yml` red
on `main` at `eebe9737`, streak 1, naming run
[37367869180](https://github.com/ranger360ai/posse/actions/runs/37367869180).
Its description is byte-identical to
[ranger-base-hxcqe](ranger-base-hxcqe.md)'s, which was filed over that same
run at 20:49:44Z and closed GREEN at `64f18288` hours before this one was
written.

Nothing was wrong with the gate and nothing is fixed here. **The filer is a
launcher 101 commits behind `main`**, old enough to have none of this week's
four ci-watch fixes — including the one that demotes this exact run. The
remedy is an install and a restart, which is the operator's
(**ranger-base-xxgp3**).

## The gate, read at the run and not off a list page

MEASURED 2026-10-05T23:35Z, `/actions/workflows/ci.yml/runs?branch=main` and
`/actions/runs/37380736552/jobs?per_page=100` (`total_count` 6, six rows, so
the page is complete). `main`'s tip is `82c08653`, local and remote.

| run | head | created | conclusion |
|---|---|---|---|
| [37380736552](https://github.com/ranger360ai/posse/actions/runs/37380736552) | `82c08653` **= tip** | 22:10:26Z | success |
| 37380093175 | `e93f6bb4` | 22:04:34Z | success |
| 37378723466 | `3d5daf0b` | 21:52:09Z | success |
| 37378458990 | `484b185e` | 21:49:41Z | success |
| 37372180663 | `64f18288` | 20:50:06Z | success (attempt 2) |
| **37367869180** | **`eebe9737`** | **20:08:21Z** | **failure — the run this bead names** |

All six jobs of the tip run are `success` with 11 executed steps and a runner
assigned, queue waits 4-78s; the run completed at **22:20:55Z, seventy
minutes before the bead was filed**. Five completed `success` runs sit
between the run this bead names and the tip, the oldest of them green since
**21:38:52Z** — an hour and fifty-two minutes before the filing.

## The filer

MEASURED 2026-10-05T23:40Z on this box:

```
ps -Ao pid,lstart,args | grep 'dispatch --watch'
  22611  Sat Oct  3 15:28:08 2026  posse dispatch --watch 3m --max-interval 3m -n 4 --resume
ls -l ~/.local/bin/posse                      13,573,426 bytes, Oct  3 15:10
posse version                                 posse 0.5.0+193c8782 (herdr-native)
git log -1 --format=%ci 193c8782              2026-10-03 14:38:17 -0400
git rev-list --count 193c8782..origin/main    101
```

and `posse status` says it itself, in the sentence launcherlag.go
(ranger-base-z3hx6) exists to print:

> launcher · 0.5.0+193c8782 is 101 commit(s) behind main in ~/src/posse —
> every one of them is a fix this fleet is not getting, and it keeps running
> the defects they fixed

Four of the 101 are ci-watch, and `git log --oneline 193c8782..origin/main --
internal/posse/ciwatch.go` is the whole list:

| commit | bead | what the running binary does not have |
|---|---|---|
| `540cc22c` | ranger-base-m46kr | the freshness guard — a page the index served from behind is not a verdict |
| `f5ccb1a8` | ranger-base-1ump9 | the freshness spelling's pin |
| `64f18288` | ranger-base-rdi79 | a cancelled JOB is the queue too, so it gets no verdict |
| `e93f6bb4` | ranger-base-r5ksj | the jobs-call cap bounds calls and not the verdict's direction |

## What the shipped rule does with the run this bead names

MEASURED 2026-10-05T23:42Z, `/actions/runs/37367869180/jobs?per_page=100`,
`total_count` 6 and six rows:

| job | status | conclusion | steps | runner_name |
|---|---|---|---|---|
| test (ubuntu-latest, 1) | completed | cancelled | 0 | `""` |
| test (ubuntu-latest, 3) | completed | cancelled | 0 | `""` |
| test (macos-latest, 3) | completed | cancelled | 0 | `""` |
| test (macos-latest, 1) | completed | success | 11 | assigned |
| test (macos-latest, 2) | completed | success | 11 | assigned |
| test (ubuntu-latest, 2) | completed | success | 11 | assigned |

That is `ciJobsSayQueue`'s demotion arm exactly — complete page, no job
`failure`/`timed_out`/`startup_failure`, at least one `cancelled` with zero
steps and no runner — so the shipped reading sets the run aside and the next
verdict down answers. The next verdict down is 37362765576 (`bab60e51`),
`success` since **19:55:27Z**, three and a half hours before the filing. So
on `main`'s code this reading is GREEN and no bead is filed, whatever page
the index served. **The old binary is not one cause among several; it is the
cause.**

## The page it must have been served, and the index caught in the act

For `eebe9737` to be the newest verdict-bearing run of a page, that page
cannot have carried the five newer runs above as `completed`. Two shapes do
that — the newer runs absent, or present and not yet `completed` in that
snapshot — and nothing distinguishable survives after the fact. Both count
the same distance, spelled as `ciFreshness` spells it:

```
git rev-list --count eebe9737..refs/remotes/origin/main   →  8
```

**8 is INSIDE `ciFreshMaxBehind = 64`**, so the freshness guard would have
read that page as current. It never ran — the running binary predates it —
but the number is the point and it is new evidence, below.

The index was caught in the act one minute after the filing. MEASURED
2026-10-05, this box, `gh 2.98.0`, ci-watch's own query
(`gh run list --repo ranger360ai/posse --workflow ci.yml --branch main
--limit 100 --json conclusion,status,createdAt,headSha,url`):

| when | page topped by | created | `d` |
|---|---|---|---|
| ~23:31Z, first hand reading of the session | `dd7822ae` success | **2026-09-11T12:06:03Z** | **216** |
| 23:35:21–23:36:01Z, 12 consecutive polls | `82c08653` success | 2026-10-05T22:10:26Z | 0 |
| 23:36:17–23:36:26Z, limits 8 / 15 / 30 / 100 | `82c08653` success | 2026-10-05T22:10:26Z | 0 |

A third depth for one day, beside m46kr's 150 and 220, and the same 24-day
snapshot that cost ranger-base-17jhu a session. Page size is not the lever:
the stale reading was taken at `--limit 15` and the next one at the same
limit was current. Note which way that page is topped — `success` — which on
a launcher with no freshness guard is the SILENT half of the defect, a green
verdict over a month-old page.

## New evidence for ciFreshMaxBehind, and what does not change

m46kr's const comment ends: *"It is a BOUND and not a proof. A page stale by
fewer commits than this reads as current, and nothing here would catch it;
both pages actually observed were 2.3x and 3.4x past it."* This episode is
the first page observed **inside** the bound — `d` = 8 — and it is the first
to have cost a P1 and a dispatched session.

The bound is not changed and no bead is filed to change it:

- The number rests on 1,357 reconstructed breakpoints with a measured
  legitimate max of 57. Lowering it to 8 would abstain on legitimate
  readings; a run in flight carries no verdict, so every commit pushed while
  CI runs sits ahead of the newest run that has one.
- What covered this class on `main`'s code is a different guard — rdi79's
  job-level vet — and it covers it completely here, for a reason that
  generalizes only to the queue-only class: a stale page topped by a
  GENUINELY red run inside 64 commits would still file a bead naming a run
  that is not the branch's current state. No episode has cost that yet. When
  one does, the reading to take is m46kr's census re-run against the pages
  the index actually served, not a smaller constant chosen from one episode.

## The second finding: the view that says both things

`posse status` prints the 101-behind sentence, ending "only installing closes
it", and then prints **`nothing needs a human`** six lines later. The summary
is `GovReport`'s answer for an empty `GovSet` (govern.go:1222) and no
governance condition reads the lag — `grep -n 'Lag\|Launcher'
internal/posse/govern.go` finds nothing at `82c08653`. Four days of that
reading is how the launcher reached 101. Filed as **ranger-base-y13h7**.

## What to re-run

```
ps -Ao pid,lstart,args | grep 'dispatch --watch'      # the image's age
posse status | head -2                                # the lag, in one line
git rev-list --count $(posse version | sed 's/.*+//;s/ .*//')..origin/main
gh api "repos/ranger360ai/posse/actions/workflows/ci.yml/runs?branch=main&per_page=8" \
  --jq '.workflow_runs[] | "\(.id) \(.head_sha[0:8]) \(.conclusion)"'
gh api "repos/ranger360ai/posse/actions/runs/37367869180/jobs?per_page=100" \
  --jq '.jobs[] | "\(.name) \(.conclusion) steps=\(.steps|length) runner=\"\(.runner_name)\""'
```

The poll loop that caught the stale page is five lines of shell around the
`gh run list` above with `git rev-list --count <top sha>..refs/remotes/origin/main`
after it; it takes ~4s per reading and the depth it reports is the whole
signal.
