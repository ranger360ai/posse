# A retry budget of twenty attempts that was one instant (ranger-base-6mpke)

2026-11-07. `pruneUntil` (internal/posse/reclaimdestroy_qa_test.go) is the
retry budget that stands between ranger-base-43mxx's reclaim pins and the
fork/flock false-busy the shop has ACCEPTED in production (ranger-base-a40d9,
operator ruling on ranger-base-5mulm: bounded, self-correcting, no fork
gate). It was `for i := 0; i < 20; i++` with nothing between the passes.

That bound read like twenty chances at a 5.31% event. It was one.

## 1. What was MEASURED

MEASURED 2026-11-07, this box (darwin 25.6.0, go1.26.5, 8 cores), over
`muxMetaRexBoard`'s own fixture — the board arm 2 of the reclaim pins runs on:

| probe | answer |
|---|---|
| one listing pass (`b.Sessions()`) | 167-196µs (three runs, `-count=3`) |
| the whole twenty-pass budget, back to back | 1.7-3.9ms |
| the launcher lock held 10ms from a second open file description | exhausted all 20 passes |
| the same at 50ms | exhausted all 20 |
| the same at 200ms | exhausted all 20 |

The hold is a deterministic stand-in for the real window and needs no forks
and no load: `flock` is per open file description, so a second `open` +
`LOCK_NB` from this very process conflicts with the one `reclaim` takes
(arm 1 of the same file already reads that fact from the other side). No
load-rate rig was run — the standing ruling is no load testing on this box
(AGENTS.md) — and none is needed: the exhaustion is deterministic at every
hold from 10ms up.

So the budget covered under four milliseconds of wall clock, and the thing it
exists to ride out is a **window in time**: fork to `execve` in a sibling
goroutine's child, on a box where that child has to be scheduled first. At
the one-minute load average of 49-75 the flake was observed at, that window
is not within two orders of magnitude of 4ms.

Twenty back-to-back passes do not sample twenty instants. They sample one.

## 2. What it cost, which is the reason this was a bead and not a note

ranger-base-43mxx section 8 recorded the same mechanism for this package and
chose a note, correctly, because in PRODUCTION the cost is one spared file and
the documented "the next quiet pass prunes it". The suite surface is where
that pricing stops holding. ranger-base-0g6zm's full `make test` reddened
`TestQAAFailedDestroyKeepsTheRecordAndRetries` with

    reclaimdestroy_qa_test.go:152: after 20 listing passes, the destroy was
    attempted once and never again

over a tree whose only `.go` delta from `main` touched nothing near reclaim.
Pass 1 reached the destroy; passes 2-20 were all spared on the launcher lock.
The cost was a red arm 1, a ~680s run discarded, and a reader aimed at a
merge defect that was not there — the pin that reddened sits over `reclaim`
in `herdrback.go`, which was the one file that merge-back had to merge.

## 3. The fix: the budget's unit, and what a pass has to have done to count

Two changes, in `pruneUntil` / `prunePasses`:

- **The budget is wall clock** — `pruneUntilBudget`, 15s — not a pass count.
- **A pass spared on the launcher lock is not a pass.** It never reached the
  branch under test, so it does not count against `pruneUntilReaches` (20,
  the old bound, now applied to the only passes it was ever a statement
  about). A contended lock can therefore exhaust nothing but the clock, and a
  genuine break still fails in milliseconds rather than at the deadline.

`lockSparedSentence` is the coupling that does this, and it is the one thing
here no build can check: `reclaim` writes "the launch lock was not taken" and
`prunePasses` reads it. A rename on reclaim's side would break nothing and
silently turn every spared pass back into a counted one — the budget back to
a count of twenty. It is pinned through the deadline message, which cannot be
reached except through that match.

`pruneUntilGap` (5ms) spaces the passes. It is **not** part of the argument
above and is not asserted as if it were: with the reach accounting in place a
zero gap still rides the window out, by spinning. It is there because the
alternative is burning a core flat out against contention on a box that is
already short of cores.

The other change is the hand-rolled twenty-pass witness loop in
`TestQAATheDestroyIsNotTakenOnEvidenceTheReProofWouldSpare`, which had the
same bug for the same reason and now goes through `pruneUntil`.

## 4. What this is not

- **Not a fork gate.** ranger-base-5mulm ruled that out on 2026-09-07 and
  this bead does not reopen it. The subject is the pins' robustness against a
  race the shop has accepted, not the race.
- **Not `t.Parallel()` dropped.** That is a real house mitigation for this
  family (`cmd/testparallel`'s `serial` map, forkflockrace_live_test.go's
  argument: Go never runs a non-parallel test concurrently with a running
  batch of parallel ones). It was rejected here because it is not sufficient
  for this file: `TestQAARexHuskIsDestroyedAndOnlyNotFoundCountsAsAlreadyDestroyed`
  has three `t.Parallel()` SUBTESTS, each forking its own fake rex while a
  sibling holds its own launcher lock, so the file self-races with its top
  level already serial — and the observed red was in a pin with no subtests at
  all. A time budget covers both shapes and costs the parallel phase nothing.
- **Not a home no sibling fork can contend.** There is none. The lock path is
  already per-test (`newTestBackend`'s own home); the fd that reads busy is
  **this test's own**, duplicated into a sibling's child by `fork`. A
  different path changes nothing.

## 5. The mutation table

MUTATION-CHECKED 2026-11-07, each mutant applied to a clean tree and the tree
asserted clean again after. Run:
`go test -run 'TestQAA(ThePruneRetryBudget|FailedDestroy|TheDestroyIsNot|ProvenDead|RexHusk|ThePruneNeverDestroys|TheFakeRex|RexSessionGone)' ./internal/posse`

| mutant | reds |
|---|---|
| `time.Sleep(pruneUntilGap)` removed (the passes unspaced) | the ride-out arm, on its pass count: 1212-1418 passes against a ceiling of 600 |
| a spared pass counts against the reach guard | the ride-out arm |
| `lockSparedSentence` no longer matches what reclaim writes | the ride-out arm AND the held-throughout arm |
| the deadline guard removed | the held-throughout arm, by the package timeout (`panic: test timed out`) |
| the whole old helper restored — a bare count of 20, back to back | the ride-out arm AND the held-throughout arm |
| the deadline's give-up claims `NOT ONE ... reached` whatever it counted | the reach-guard arm's second give-up (at 15.18s, the budget) |

The ride-out arm's hold is **300ms and that number is load-bearing**: longer
than `pruneUntilReaches` passes can span even with the gap (20 × 5ms =
100ms), so the only thing that can ride it out is the reach accounting. At
the 10ms of section 1 the arm would pass with that accounting deleted, which
is the one mutant it exists to catch.

## 6. The arm's own probe was subject to the race the arm pins

Found by running the mutation table, not by reading: one run in nine reddened
the ride-out arm in 0.01s with "could not take the launcher lock to hold it".
The arms hold the lock by taking it, and **that** probe reads the same
false-busy — a sibling goroutine's fork duplicating some other test's
launcher-lock fd into its child. The arms ride the window out for the
*prune's* probe and would have flaked on their own for want of doing it for
their own. `holdLaunchLock` retries the take over the same budget, so a lock a
real launcher holds for the whole of it still fails, and says which it was.
12 consecutive runs of the file green after it (`-count=12`).

And then the same thing a second time, in the arm that pins the deadline's
own give-up. Written as ONE probe with a 1ms budget it reddened the second
full `make test` of this bead:

    the deadline reported "after 1 listing passes over 1ms, NOT ONE of which
    reached reclaim's own decision — ..." over passes that reached

One pass, spared, so the give-up was the other branch and correctly so. A
short budget is by construction a budget that one window can swallow — this
bead's whole finding, applied to the pin that pins it. It repeats the probe
now until the box gives it a pass that got through.

Both of these are the same lesson and it is worth stating plainly: **a pin
over this race cannot have a single-shot step anywhere in it**, including in
its own setup and its own probes, not only in the loop it is pinning.

The two give-up messages are worth one sentence each, because this bead is
about where a message sends its reader. The reach guard's names the passes
that reached reclaim's own decision — that is a claim about the prune. The
deadline's, when none of them did, says so and names the lock — that is a
claim about the box. The second branch of the deadline, over passes that did
reach before the clock ran out, exists so that neither message is ever said
over the other one's evidence.

## 7. The ride-out arm's numbers

The pass-count ceiling is ten times the ~60 passes a spaced ride-out of 300ms
actually costs. The spaced run is sleep-dominated and cannot approach 600
whatever the box is doing, so the arm cannot flake; the unspaced mutant is
pass-cost dominated, so its own margin narrows as the box gets busier.

## 8. The suite runs

Three full `make test` runs on this tree, this box, 2026-11-07, at one-minute
load averages of 12-26 with a sibling seat holding a suite slot:

| run | result |
|---|---|
| 1, over the fix as first written | GREEN, exit 0, zero FAIL. arm 1 internal/posse 482.4s + cmd/posse 215.9s, arm 2 414.3s, arm 3 395.8s |
| 2, after the deadline-message fix | **RED** — the 1ms single-shot probe above. arm 1 internal/posse 622.5s |
| 3, final | GREEN, `MAKE_TEST_EXIT=0`, zero FAIL. arm 1 internal/posse 456.3s + cmd/posse 223.1s, arm 2 362.6s, arm 3 365.3s, treepins 466.8s |

`go run ./cmd/checkorphans` clean after each. The reclaim pins themselves also
ran `-count=15` focused, green, and the file costs ~0.5s of the arm (the
300ms hold and the ~0.13s reach-guard arm are most of it).
