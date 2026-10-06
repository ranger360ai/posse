# The page this bead was filed off was topped 25 days and 243 commits back, and it was topped with a real red (ranger-base-ut512)

ci-watch filed ranger-base-ut512 at P1 on 2026-10-06T11:11:34-04:00: `ci.yml`
red on `main`, streak 2, naming
[34581919232](https://github.com/ranger360ai/posse/actions/runs/34581919232)
(`9b24d3c6`) as **red now** and
[34581610397](https://github.com/ranger360ai/posse/actions/runs/34581610397)
(`a40b2ce3`) as **red since**, both stamped **2026-09-11**.

Nothing was wrong with the gate and nothing is fixed here. Both runs are
genuine failures — from twenty-five days ago, cleared the same morning. The
filer is the launcher **112 commits behind `main`**, the same binary and the
same cause as [ranger-base-6pnab](ranger-base-6pnab.md) one day earlier, and
the remedy is the same install and restart, still unanswered at
**ranger-base-xxgp3**.

What is new here is the shape of the staleness. 6pnab's page was topped with a
phantom — a run whose jobs page says no job of it ever failed — so
`ciJobsSayQueue` (ranger-base-rdi79) demoted it and the shipped code would
have filed nothing on that ground alone. **This page's top run really did
fail.** No jobs vet can touch it. The only fix in main that rejects this
reading is `ciFreshness` (ranger-base-m46kr), and that is the measurement
below: the two fixes are not redundant, and this is the first observed case
that needs the freshness one specifically.

## The gate, read at the run and at its jobs page

MEASURED 2026-10-06T15:45Z, by `head_sha` on `/actions/runs` rather than off a
list page, because a list page is the thing under suspicion. `main`'s tip is
`f5e40f19`, local and remote (`git ls-remote origin main`).

```
run 37402325341 · f5e40f19 = tip · main · push
  created 2026-10-06T02:04:13Z   completed/success 02:13:20Z
/actions/runs/37402325341/jobs?per_page=100 -> total_count=6, six rows
  test (ubuntu-latest, 1..3)  completed/success  11 steps  runner assigned
  test (macos-latest,  1..3)  completed/success  11 steps  runner assigned
```

Six of six jobs `success`, 11 executed steps each, a runner on every one. Over
the 100-run window gh serves for `ci.yml` on `main`: **99 success, 1 failure,
0 in flight** — the one failure being `eebe9737`, which is 6pnab's run and was
superseded before that bead was filed.

## The page the bead was filed off, and how far back it was topped

The two runs the bead names are real reds. `9b24d3c6`'s jobs page is complete
and one job of it genuinely failed:

```
/actions/runs/34581919232/jobs?per_page=100 -> total_count=6
  test (ubuntu-latest, 1): completed/failure  11 steps  runner 1000001651
  the other five:          completed/success  11 steps  runner assigned
```

So this is not a queue phantom. It is a true verdict about `9b24d3c6`, served
as a verdict about now, and `main` had moved **243 commits** past it:

```
git rev-list --count 9b24d3c6..refs/remotes/origin/main    243
git rev-list --count a40b2ce3..refs/remotes/origin/main     248
ciFreshMaxBehind                                             64
```

That red was cleared the same morning it happened — `4c23ac59` green at
2026-09-11T11:47:17Z and `dd7822ae` at 12:06:03Z, 2h06m after the run the bead
calls "red now". The gate has not been red on that commit's account since
2026-09-11.

**This is the deepest stale page observed so far.** `ciFreshness`'s own
paragraph records 150 and 220 commits behind (2026-10-05, 38 hand readings,
one stale) and ranger-base-xxgp3 records 216 one minute after 6pnab was filed.
243 and 248 are 3.8x past the bound that was written to catch them.

## The index served it twice while this bead was being worked

MEASURED 2026-10-06T15:20-15:50Z on this box, gh 2.98.0, **29 readings** of
the gate's own query and its `gh api` equivalent, both scoped to `ci.yml` on
`main`:

| reading | n | topped at | behind |
|---|---|---|---|
| current | 27 | `f5e40f19` 2026-10-06T02:04:13Z success | 0 |
| **stale** | **2** | `dd7822ae`/`9b24d3c6` **2026-09-11** | **243-248** |

Both stale readings were the same snapshot, and it is the snapshot the bead
names. The first was the first `gh run list` of the session; the second was a
`gh api .../workflows/ci.yml/runs?branch=main` **in the same `bash` invocation
as a current `gh run list`**, which is the cleanest evidence available that
this is an index serving past pages and not a client cache: two spellings of
one question, seconds apart, one current and one twenty-five days behind. A
20-reading census immediately afterwards was 20/20 current.

This is `ciFreshness`'s "it is a BOUND and not a proof" paragraph holding
exactly as written — an index settling, with its depth bounded by nothing.

## The filer, and why main's guard did not run

MEASURED 2026-10-06T15:30Z:

```
posse status      launcher · 0.5.0+193c8782 is 112 commit(s) behind main in ~/src/posse
ls -l ~/.local/bin/posse                 13,573,426 bytes, Oct  3 15:10
ps -o lstart= -p 22611                   Sat Oct  3 15:28:08 2026  (up 2d20h)
lsof -p 22611 | awk '$4=="txt"'          ~/.local/bin/posse  inode 475260281
stat -f %i ~/.local/bin/posse            475260281        <- same inode, so the
                                                             running loop IS this build
plugin/bin/posse -> ~/.local/bin/posse   one binary, not two
```

`ciFreshness` landed in `540cc22c` on **2026-10-05 09:52**, thirty-four hours
before this bead was filed, and the live binary was built from `193c8782` on
**2026-10-03 14:38**. Three independent readings agree it is not in the thing
that ran:

```
git cat-file -p 193c8782:internal/posse/ciwatch.go  | grep -c ciFreshMaxBehind   0
git cat-file -p origin/main:internal/posse/ciwatch.go | grep -c ciFreshMaxBehind 5
strings ~/.local/bin/posse | grep -c 'the reading is not current'                 0
```

Of the 112 commits the live binary is missing, 66 touch `internal/posse` or
`cmd/`, and **five** touch `ciwatch.go` or `launcherlag.go` — the four
ranger-base-xxgp3 already names, plus `6b330c13` (ranger-base-y13h7), the
governance condition that stops `posse status` printing `nothing needs a
human` over a launcher this far behind. That last one is the sharp edge: the
condition written to make this visible is itself in the gap it describes.

Had the shipped code made this reading, `ciFreshness` would have returned
non-empty at `243 > 64` and `ciAbstain` would have printed the recipe instead
of filing a P1. The arithmetic is deterministic and is the three lines above.

## What this bead cost, and what closes it

Second dispatched session on a green gate in two days, same cause. The gate's
own header already says what that buys: *what a red gate costs is not the
reds, it is the ATTRIBUTION* — and a false red spends attribution in the
opposite direction, on a branch that was never red.

`launcherlag.go` is explicit that the remedy is not ours: *"Installing over a
binary that is dispatching a live fleet is a live change and stays the
operator's (guardrail 3); this file is the signal, not the remedy."* The
signal is firing in so many words, on every pass, and has been for 16 hours at
**ranger-base-xxgp3** (open, assignee `dave`, P1), which carries the exact
commands and the blast radius. No second ask is filed here; this bead's
reading is added to that one as its second cost.

The directional point ranger-base-xxgp3 makes is worth repeating with today's
number attached: with no freshness guard, **every** past page the index serves
is a verdict in whichever direction it happens to be topped. Today it was
topped red on a green branch, which is loud and costs a session. Topped
**green on a red branch** it is silent, and that is ci-watch's founding
incident.
