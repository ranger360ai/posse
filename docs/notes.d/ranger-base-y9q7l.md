# The deadline was sized for a cost that belongs outside it (ranger-base-y9q7l)

2026-11-02. `TestReadCLIVersionGivesUpOnACLIThatWillNotAnswer`
(internal/posse/runtimeprobe_test.go) reddened arm 3 of a full `make test`
for a seat whose diff added one test and two docs files and touched no
production code. The failure was the fixture's OWN witness assertion, firing
by design:

```
the planted sleeper never reached its first line under a 3s deadline
(stat ...sleepycli.ran: no such file or directory), so nothing forked,
nothing held the pipes, and cmd.WaitDelay had nothing to bound — this arm
measured the scheduler and not the code
```

The witness is ranger-base-metqp's correction and it worked: the arm said
"raise the limit" instead of passing over a tree where the thing it pins had
been deleted. Raising the limit is what the test's doc prescribed, and what
this bead was filed to do. The measurement says the limit was the wrong lever.

## It is not the scheduler, it is the first exec of a new binary

The fixture writes a `#!/bin/sh` script into a fresh `t.TempDir()` and execs
it. MEASURED 2026-11-02 on this box (darwin 25.6.0), timing the sleeper's
witness file from `cmd.Start()`, with the 1-minute load average per reading:

| reading | loadavg | cold: a new path each time | warm: the same path again |
|---|---|---|---|
| n=40 | 5.7 | mean 714ms, worst 2.17s | — |
| n=40 | 6.9 | mean 696ms, worst 1.58s | — |
| n=30 | 19.6 | mean 702ms, worst 1.97s | first 1.34s, then mean 8.8ms, worst 13.4ms |
| n=30 | 44.4 | mean 181ms, worst 471ms | first 145ms, then mean 6.5ms, worst 7.8ms |
| n=100 | 10.6 | mean 164ms, p50 150ms, p99 417ms | first 138ms, then mean 6.3ms, p99 21ms |

The first row polled for the witness file without sleeping and the rest poll at
1ms; its mean is 2.5% off the row below it, so the spin is not what any of this
measures.

Two things in that table:

**The cold exec's mean moves 4.4x between readings (164-714ms) and its worst
5.2x (417ms-2.17s), and nothing in the load column predicts where a reading
lands.** A single exec measured 150ms at one reading's p50 and 2.17s at
another's tail, 14x apart. The largest worst-case came at loadavg 5.7, and the
cheapest reading of all — mean 181ms, worst 471ms — at 44.4, the busiest box of
the five. So the load column this bead asked for, reasonably, since every
earlier reading of this fixture was labelled by load, would not have warned.
(A plausible cause is CPU frequency state: the fast readings follow a heavy
compile, the slow ones an idle box. Not measured; it does not change the fix.)

**The warm re-exec of the same path is 6.3-8.8ms mean and 7.8-21ms at the
tail, in every reading, across loadavg 5.7 to 44.4** — two orders of magnitude
cheaper and stable where the cold one is not. In both readings that have a
warm column, the FIRST exec of the warm path cost what a cold exec cost
(1.34s, 145ms) and every later exec of it cost ~7ms, so the charge is per-path
and a pre-flight of the same path pays it.

## So the fix is a pre-flight, and the limit is only the margin

The sleeper is now a two-invocation script: the first marks `$0.warm` and
exits before reaching its body, the second writes `$0.ran` and runs the body.
The fixture spends invocation 1 before the clock starts, through
`readCLIVersionWithin` itself so that what it warms is exactly the exec the
timed call makes (same `protocolCmd` resolution, same env). The timed call
then re-execs a warm path: ~8ms of startup inside the deadline, against a
14x-variable cost the old 3s had to cover.

The limit is 4s rather than 3s anyway. 4s is 190x the measured warm tail, and
1.8x the worst cold reading — so if the pre-flight is ever removed or warms
nothing, the arm degrades to slightly better than the 3s that reddened, not to
a coin toss. `bound` stays 12s: the arm reads limit+WaitDelay = 8s, a leak
reads 25s, and there is nothing between 8 and 25 for a loaded box to be
mistaken for. Cost: 8s of this arm's wall against 6s, both bodies in parallel.

The pre-flight is guarded on both sides, because a pre-flight that silently
does either thing wrong hands back exactly the green this bead is about — one
would leave the cold exec inside the deadline again, the other would make
`.ran` a witness to the pre-flight instead of to the timed run.

## The old matrix would have chosen 1600ms today

The reading being replaced was 3 runs per cell of "did the sleeper RAN at this
limit": 200ms 0/3, 800ms 1-2/3, 1600ms 3/3, 3s 3/3, at loadavg 5.9/21.1. It
was RE-TAKEN 2026-11-02 at loadavg 5.3-7.7, 3 runs per cell per arm, over
1600ms, 3s, 4s, 6s and 8s: **every cell read 3/3, both arms, nothing
distinguished.** Six samples cannot see a tail event, and the tail event is
the whole failure — so the matrix is green at every limit it was meant to
choose between, and on this box today it would have picked the cheapest one.
A RAN-ratio at 3 runs a cell is not an instrument for choosing this number;
the latency distribution is. That is the part worth carrying forward.

## Verification

- The pin still discriminates at the new limit: `cmd.WaitDelay = limit`
  deleted reds **3 of 3**, both arms, each run reporting a 25.0s wait under a
  4s deadline.
- Each new witness fires under a fixture mutant: the warm guard deleted reds
  the `.warm` assertion on both arms; a `.ran` moved ahead of that guard reds
  the `.ran` assertion on both arms.
- The empty-read check in front of those two has no mutant and is the weak
  one: no body here prints inside the pre-flight, so it guards the NEXT body
  rather than asserting anything today. Said so in the test.
