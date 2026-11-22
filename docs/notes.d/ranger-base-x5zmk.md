# Four unwitnessed arms of the bounded suite queue, and the one that re-admitted the symptom (ranger-base-x5zmk)

2026-11-22. ranger-base-gm54k verified ranger-base-og2zi's close and found the
behaviour right and three of its load-bearing branches unwitnessed: each could
be removed with `scripts/suite-lock.sh --self-test` still printing `all arms
ok`, and the first of them put the bead's own symptom back with the kernel's
authority attached. A fourth arm, in another repository, is tracked on this
bead and is not described here.

All numbers below are MEASURED 2026-11-22 on linux 7.2.15-1-omarchy, 16 cores,
bash 5.3.20, with python3 as the lock tool. The mutants are
ranger-base-gm54k's, re-run against the new arms by a grading script committed
beside this persona's other batteries (`ranger-base-x5zmk-suitelock-grade.sh`);
it mutates a COPY and never touches a worktree.

## The three mutants that survived, and what kills them now

| mutant | what it removes | before | after |
|---|---|---|---|
| M3b | the kernel reader's dev+ino confirmation, 8-entry cap lifted | 20 ok / 0 FAIL | arm 19 FAIL |
| M3 | the same, cap left at 8 | arm 18 FAIL, on a busy box only | arm 19 FAIL |
| M9 | the give-up branch's holder read, blanked | 20 ok / 0 FAIL | arms 18 and 22 FAIL |
| M7 | the same read, moved AFTER the say | 20 ok / 0 FAIL | arm 22 FAIL |
| M8b | `_suite_lock_wait_bound` accepts 0 | 20 ok / 0 FAIL | arms 20 and 21 FAIL |
| M8 | the same edit, aimed at `_suite_lock_slots` | arm 12 FAIL (misaimed) | arm 12 FAIL, new arms green |

The last row is the probe trap and the reason it is in this table: `[ "$n" -ge
1 ]` occurs TWICE in the file, at `_suite_lock_slots` and at
`_suite_lock_wait_bound`. A first-occurrence replace grades the wrong one and
reds a NEIGHBOURING arm, which reads exactly like a kill. A grading script
that does not assert which of the two it hit cannot tell those apart.

## Arm 19 — the kernel names the holders of THIS slot

`_suite_lock_kernel_holders` walks every `/proc/<pid>/fdinfo`, keeps the fds
carrying a `lock: … FLOCK` line, and then confirms by dev+ino through
`/proc/<pid>/fd/<fd>` that the fd is this slot file. Dropping that one
confirmation reports every process on the box holding any flock at all. Over
one real squatter it printed ten processes as the slot's holders, a desktop
audio daemon among them — the bead's own symptom, a waiter that misnames the
holder, except that this one says `per /proc fdinfo` while it does it.

**Arm 18 had no positive control for it.** It greps the log for the squatter's
real pid; the reader keeps the 8 lowest-numbered pids; a freshly forked
squatter sorts last. So on a box already holding more than 8 unrelated flocks
the cap drops the pid the arm looks for and the arm reds — and lifting the cap
in the same mutant leaves arm 18 GREEN with the ten-process line still
printed. What red there was the cap, and only on a busy box.

The fixture is a decoy: a forked process holding an flock on a file in the
rig's own tmp dir, taking no slot and reading no stamp. It is the one shape a
rig whose only flock is the slot's own physically cannot produce. Three halves,
because the cap is still in the way:

- the squatter IS named, or the two below are vacuous over an empty list;
- the decoy is NOT named — the defect on a box with few enough flocks to fit
  under the cap (measured: with the cap at 8 the decoy landed inside it);
- and the list is not TRUNCATED — the same defect on a box with more, where the
  cap is what hides it. `and N more` over one slot file in a dir the rig just
  made means the reader is answering about something else.

Between them there is no flock count at which the mutant survives.

## Arms 18 and 22 — the give-up line, not the log

An acquire that queued and then gave up says the holders TWICE: once at the
announce, once at the give-up. Arm 18's four naming assertions were all
answered by the earlier WAITING line, so the give-up branch's holder read could
be blanked with the arm green and the line reading `held by: nobody this seat
can name`. That is the line a reader sees when their suite went unserialized,
which is the half of the bead the bound is for. The fix is one assertion asked
of the give-up LINE — `log_line`, a bash-only line reader, for the no-exec rule
the self-test's own preamble gives.

**That is not enough on its own, and it is why arm 22 exists.** `held` is set
twice in one acquire, so MOVING the second read after the say leaves the first
one's value in scope: the line is populated, every naming assertion over it
passes, and all 23 other arms stay green. What it names is then the holder at
the announce, which on a real queue is one default bound — an hour — earlier,
under a sentence telling the reader to go check whether that pid is a suite.

Arm 22 discriminates on the STAMP rather than on the slot. Handing the slot
from one squatter to another cannot be done without the waiter winning the
sweep in between, so the squatter stays and its stamp changes from
`/first/stamp` to `/second/stamp`. The announce must read the first (the
control) and the give-up must read the second.

**The ordering is structural, not timed**, which is the ranger-base-stbh0 rule:
a rig that had to rewrite the stamp before a bound expired would be timing its
own assertion. At `POSSE_SUITE_LOCK_POLL=3` and `POSSE_SUITE_LOCK_WAIT=1` pass
1 sweeps, announces at `waited=0` and sleeps 3s; pass 2 sweeps and gives up at
`waited=3`. The rewrite goes in at the head of that sleep, a whole poll
interval before the branch that reads it can run. The arm costs the one sleep.

## Arms 20 and 21 — the bound's validator

The asymmetry is the argument. `POSSE_SUITE_SLOTS` has TWO arms for exactly
this validator (arms 12 and 13). `POSSE_SUITE_LOCK_WAIT` got the same validator
and neither, and no arm anywhere set a bad value, so its `is not a positive
integer — using 3600` line was a sentence no run on this box had printed.

0 is the value that costs, and it is the one a reader reaches for to mean "do
not wait". Taken, `waited` is 0 on the first pass, `[ 0 -ge 0 ]` holds, and the
acquire gives up the queue before it has ever announced that it was queued — so
nothing on the box queues again while every arm stays green. Arm 20 is the
"says so" half over a free dir; arm 21 is its control, a waiter at
`POSSE_SUITE_LOCK_WAIT=0` against a held single slot that must QUEUE.

Arm 20 asserts the message WHOLE — variable name, value, and the default it
fell back to — because `not a positive integer` on its own is the sibling
validator's sentence too. That is the M8/M8b distinction made into an
assertion.

## Cost

24 arms, all green, `make test`'s `verify-suite-lock` prerequisite unchanged.
Arms 19 through 22 add four forked holders, one decoy and one 3s poll.

## Not closed here

Nothing of this bead's suite-lock work. Its fourth finding is in another
repository and is tracked on ranger-base-x5zmk.
