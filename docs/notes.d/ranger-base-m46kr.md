# The freshness guard on ci-watch's reading, and the number behind it (ranger-base-m46kr)

ci-watch filed ranger-base-17jhu at P1 on 2026-10-05T12:45:29Z off a `gh run
list` page whose newest run was 24 days old. `ci.yml` was green on `main` in
all six jobs at `main`'s tip and had been for 24 days; the episode the bead
named had been diagnosed and fixed at 7b35b24b on 2026-09-11. The reading,
not the bead, was wrong — `docs/notes.d/ranger-base-17jhu.md` has that
measurement. This fragment is the guard and the number it rests on.

## The defect is symmetrical and only one half leaves a trace

`ReadCI` sorted the page it was handed by `createdAt` and took `verdicts[0]`.
Nothing asked whether the page was current.

- A stale page topped **RED** files a false P1 bead. Cost: one dispatched
  session, observed once.
- A stale page topped **GREEN** says nothing while `main` is red — which is
  ci-watch's founding incident exactly (191 consecutive reds over five days,
  ~120 commits, nobody looked), because the pass "says something only when
  it ACTS" and a stale green pass is byte-identical to a current one.

Nothing in the store, in stderr or in `posse status` distinguishes the second
from a clean pass. So the guard abstains on the READING rather than deciding
either verdict, and the abstention is the "there IS a gate and this pass
could not READ it" kind — said once per process, never silence.

The dedupe cannot cover it: all three arms (`ciOpenBeads`, `ciDupeFiled`,
`ciAlreadyCleared`) read the STORE, never the clock. MEASURED 2026-10-05 over
all 37 closed `ci-red` beads in this store, matching comment text by PREFIX
the way `ciAlreadyCleared` does: 20 of those episodes refile on the FIRST
stale pass and the other 17 on the second consecutive one.

## A wall-clock bound cannot do this job

MEASURED 2026-10-05, this box, `gh 2.98.0`, `ranger360ai/posse` ci.yml on
main, 689 runs over 2026-08-28..10-05: the longest legitimately quiet stretch
between two runs is **8.80 days** (211.2h, 2026-09-11T19:30Z to
2026-09-20T14:40Z; second longest 174.3h). One of the two stale pages
observed on 2026-10-05 was topped only **7 days** back.

The legitimate quiet OVERLAPS the staleness, so no bound on the newest run's
age separates them. An age bound long enough to stay quiet over a real
weekend is longer than the staleness it exists to catch — and a repo nobody
pushed to for a week is the case where a false abstention costs the most,
because that is when a red `main` sits unlooked-at.

## Commit distance does separate them

The guard counts how many commits sit on the branch ahead of the newest
verdict-bearing run:

    git rev-list --count <newest verdict-bearing run>..refs/remotes/origin/<branch>

It reads **0** over a quiet repo however long the quiet lasts, which is the
property the clock does not have.

The reference point is the REMOTE-TRACKING ref and not the local branch:
`refs/remotes/origin/main` moves when something PUSHES, which is the event
that creates the run, while this box's local `main` routinely carries
merge-backs the operator has not pushed yet — MEASURED here at 13:04Z it was
1 ahead, and 0 nine minutes later when that commit was pushed and every
distance below moved by one, which is the ref tracking the push exactly the
way the guard needs. Nothing in a dispatch pass fetches, so that ref can only LAG what
GitHub has — which makes the count a lower bound on how far behind the page
is, and every error path falls toward letting the reading through.

### The number: 64

The legitimate value is not 0 and not 1. A run in flight carries no verdict
(`ciVerdict`), so every commit pushed while CI is running sits ahead of the
newest run that has one, and one push can carry a batch.

MEASURED 2026-10-05 (this box, `gh 2.98.0`, 689 runs of ci.yml on main across
38 days, 2026-08-28..10-05), reconstructed at every instant a run was created
or completed — 1,357 breakpoints — with the count spelled exactly as the
guard spells it:

| | legitimate `d` |
|---|---|
| median | 0 |
| p95 | 3 |
| p99 | 5 |
| max | **57** |
| max, 2026-09-07..10-05 (524 breakpoints) | **17** |

| the two stale pages observed 2026-10-05 | `d` |
|---|---|
| `9b24d3c6`, the page ranger-base-17jhu was filed off | **220** |
| `d8809472`, a hand reading ~2 minutes later | **150** |

The whole deep tail is 2026-08-30..09-06, the era when pushes to `main` shared
a concurrency group and 20 runs were **cancelled**; a cancelled run carries no
verdict, so a superseded batch accumulates. Since ranger-base-sfb3 keyed
main's group on the commit there have been 0 cancelled runs and the max is 17.

**`ciFreshMaxBehind = 64`** is the measured legitimate maximum (57) rounded
up: zero false abstentions across all 1,357 breakpoints, and 2.3x inside the
shallower of the two stale readings. Erring low is the cheap direction — too
low costs one loud abstention that the next completed run clears, too high
costs a P1 bead filed off a page that was never about the branch it names.

It is a BOUND and not a proof: a page stale by fewer commits than this reads
as current. Both pages actually observed were 2.3x and 3.4x past it.

### Plain `rev-list --count`, not `--first-parent`

**CORRECTED 2026-10-05 by ranger-base-1ump9 F1.** Both halves of what this
section first said were wrong, and the shipped spelling is still the right one
for a different and better reason.

The reason is the ON-chain shas, not the off-chain ones. Plain counts the
side-branch commits a merge brought in and `--first-parent` does not, so plain
never UNDERCOUNTS how far behind the page is — and undercounting is the
fail-open direction this guard exists to refuse, since a stale page read as
current leaves no trace at all.

**MEASURED 2026-10-05** over this repo's whole first-parent chain, at
`origin/main` = `540cc22c`, git 2.50.1 (Apple Git-155), by
`git rev-list --count <sha>..refs/remotes/origin/main` with and without
`--first-parent` for all 1,985 on-chain shas. This is the reading to re-run: it
needs no `gh` page at all.

| | |
|---|---|
| on-chain shas | 1,985 |
| the two spellings agree | 731 |
| they disagree | **1,254** |
| plain smaller than `--first-parent` | **0** |
| gap sizes (plain − first-parent) | 2, 4, 6, 7, 9, 14, 19, 22, 30, 31, 42, 44, 45, 47, **53** |
| shas with `--first-parent` ≤ 64 < plain | **0** |
| merges on the chain | 15, newest 730 commits back (`f9e50bbf`, 2026-09-05) |

A gap of 53 against a bound of 64 is a page whose true distance is outside the
bound and whose `--first-parent` distance is inside it. That no such straddle
exists today is a DISTANCE and not a property: every sha shallower than the
newest merge agrees under both spellings, the newest merge is 730 back, so
nothing within 64 of the tip disagrees. The fail-open is therefore
CONSTRUCTIBLE rather than present — which is why F1 was a debt line and not a
P1 — and it is one merge away, not one era away. Fifteen of these commits are
merges, each a `merge main into <branch>` taken so a bead could fast-forward
(read their subjects), and the next one puts the following 64 commits' worth of
readings inside the gap.

The gh side says the same thing, and is where F1 measured it first: over the
672 run head shas of `ci.yml` that are on the chain, 409 agree and **263 do
not**, gaps of 4, 6, 7, 9, 42 and 53. Prefer the chain census when re-checking.
A sample only 700 commits deep reads **zero** disagreements here — the nearest
merge is 730 back — which is the same trap as the unstable `gh` pagination
recorded further down: assert the corpus depth before trusting the answer.

What this section first claimed, and why each half is wrong:

| claim | verdict |
|---|---|
| `--first-parent` "never reaches" the off-chain shas, "so it walks the whole chain and answers ~1,415 for a run that is current" | wrong about the mechanism. `--first-parent A..B` limits the NEGATIVE traversal to first parents too, so `A`'s own first-parent ancestry is still excluded and the walk is not the whole chain. Of the 19 off-chain shas, the two spellings AGREE for 17 against the tip of their own time — the only ref that bears on "a run that is current" — and differ by 2 and by 10 for the other two; in **0 of 19** would `--first-parent` have abstained where plain would not. The ~1,415 is what `--first-parent` answers for those 19 against TODAY's ref, where plain answers 1,409-1,468 for the same shas: both enormous because all 19 are a month old, and `--first-parent` is SMALLER than plain in every one of the 19, never larger. |
| "On every sha that IS on the chain the two spellings answer identically" | false as counted: 263 of 672 disagree, per the measurement above. |

Until F1 this rested on nothing the suite could check: swapping the shipped
line for `--first-parent` **red nothing**, because every fixture in
ciwatch_test.go was a chain `ciChain` built — one `-p` per commit — and two
spellings of a first-parent walk cannot differ on a chain that has only first
parents. The pin is now
`TestCIFreshnessCountsEveryCommitBehindAndNotTheFirstParentChainAlone`
(ciwatch_test.go), on a fixture `ciMergeChain` builds:

```
b0 ─ b1 ──────── M        refs/remotes/origin/main = M
 \              /
  s1 ─ … ─ s68
```

`b1` is ON the first-parent chain with the merge between it and the ref, so
plain answers 69 and `--first-parent` answers 1 — opposite sides of the bound,
which is the arm that kills the mutant (MEASURED: it reds with "a page 69
commits behind origin/main produced a verdict", and nothing else in the file
moves). `s68` is off that chain and BOTH spellings answer 2 for it, which is
the row that says "off the first-parent chain" was the wrong discriminator to
have written the arm on. The straddle itself is asserted rather than commented:
a linear chain of the same depth would pass the ReadCI arm for the wrong reason
— plain is over the bound there too — so the pin fatals if the fixture ever
stops being able to tell the two spellings apart (MEASURED: `ciMergeChain(t,
dir, 2)` reds with "the fixture no longer straddles the bound").

### It assumes the workflow runs on every push to the branch

That is what makes a commit distance mean anything, and ci.yml satisfies it:
`on: push: branches: [main]`, no path filter, and the file header's whole
argument is that it is the ONLY gate a commit on main ever passes.

MEASURED 2026-10-05 through the shipped path against the live repo
(`posse.ReadCI` over `~/src/posse`, real `gh`), with `ci_workflow:` pointed at
this repo's other two workflows:

| workflow | reading |
|---|---|
| ci.yml | **known**, green, latest 95e1333e — `main`'s tip, d=0 |
| pages.yml | abstains — newest verdict-bearing run 2026-09-11, 100+ commits back |
| release.yml | abstains — newest verdict-bearing run 2026-09-20 |

The abstention on the other two is honest: a gate whose newest verdict is 100
commits old is not a statement about the tip either. But it is a
CONFIGURATION answer rather than a staleness one and it holds on every pass
forever, so the notice names both readings — a past page OR a workflow that
does not run on every push — instead of blaming the index. This is the one
thing running the code found that reading it did not; the first cut of the
notice asserted the index.

### The error paths

- **No `refs/remotes/origin/<branch>`** → abstain. There is no local view, so
  nothing can be proved either way; unlike a stale page this is a fact about
  the CHECKOUT that holds on every pass until somebody fetches, so saying it
  once per process is the whole of what it costs. Letting the reading through
  would leave the guard silently absent, which is the outcome this bead is
  about. `ghRepo`, the suite's own fixture, had no such ref — every reading
  test in `ciwatch_test.go` went red saying so, and the fixture now seeds one.
- **A head sha this checkout has never heard of** → the reading stands. The
  run is about a commit newer than anything local, so the page is AHEAD of the
  local view, not behind it. The cost is a shallow clone, where the old
  commits a stale page is topped by are also absent and the guard goes quiet;
  the beads repos are working checkouts, so that is a documented edge and not
  a check.
- **The notice names the BOUND and not the count.** `ciAbstain` keys its
  once-per-process notice on the Why text, so a moving number would
  re-announce the same fact on every pass that moved it — a gate whose runs
  have stopped entirely gains a commit a day forever — and that is how a
  visible line becomes an invisible one. The notice carries the `rev-list`
  recipe instead, so the reader gets the exact number.

## The pins, and that they can fail

`TestReadCIAbstainsOnARunListPageTheIndexServedFromBehind` and
`TestCIWatchFilesNothingOffAStalePageAndStillFilesOffACurrentOne`
(`internal/posse/ciwatch_test.go`, arm 1). Both build a real commit chain with
`commit-tree`/`update-ref` — not `git commit --allow-empty`, which every crew
PID denies — and drive the real `ReadCI` through the suite's fake `gh`.

Shown able to fail, 2026-10-05, each mutation applied alone:

| mutation | result |
|---|---|
| the guard not called from `ReadCI` | both red; the pass **filed `q-1`** off the stale page, which is the defect verbatim |
| `ciFreshMaxBehind = 100000` | both red on `ciSaneBound` in under a second (the fixtures are a chain as deep as the bound, so the pin's cost is the bound) |
| `ciFreshMaxBehind = 0` | red on the `d=5` row |

That last one is why the boundary table carries a literal `5` beside the two
rows derived from the const. A bound of 0 is the sha-equality shape — abstain
unless the newest run is the tip — and it passed every other row in the test,
because `{0, true}` and `{ciFreshMaxBehind, true}` collapse onto the same case
when the bound is 0. 5 is the census's p99: a reading that far behind is the
ordinary mid-flight state and must still be a reading.

The non-regression property has its own half of the second pin: a CURRENT red
page still files on the FIRST red. A freshness guard that bought silence with
that would be worse than the bead it prevents.

## Reproduce

    gh run list --repo ranger360ai/posse --workflow=ci.yml --branch main \
      --limit 1000 --json conclusion,status,createdAt,updatedAt,headSha,url
    git rev-list --count <a run's headSha>..refs/remotes/origin/main
