# ranger-base-0whbv — a green rerun is not a verification of a flake

*2026-11-13 · gilfoyle · `scripts/reap-test-tmp.sh --self-test`, finding 4's rig,
and `scripts/reap-selftest-ambient.sh`.*

Every number below is MEASURED on this box (linux 7.2.15 x86_64, uid 1001,
`/tmp` a tmpfs, 11-13 real `claude` seats live, one-minute load average
1.7-2.1) on 2026-11-13, tree at `ae63ac00` plus this change.

## The reported defect was already fixed, for the fourth time

0whbv reported finding 4's two arms grading a DELTA in a box-wide count of
live `claude` processes — 3 of 8 back-to-back `make verify-reap-test-tmp`
runs red on 2026-11-12 over a tree whose only change to the script was
full-line comments. **ranger-base-0wvu6 closed exactly that at `7725ba75`**,
by the shape 0whbv's own description proposed: one census per arm, graded by
MEMBERSHIP of the pid the rig planted in the pid list the production counter
now reports from the same loop as its count.

This is the fourth bead on that one incident. ranger-base-om4i3 was closed as
a duplicate of 0wvu6 ("verified green x3 here"), ranger-base-bttuj found the
adjacent dead-fixture vacuity in the same arms, and 0whbv was filed from a
tree that predated the fix.

## Why four, and why "verified green x3" was the wrong instrument

A delta over a population the rig does not own is only wrong when the
population MOVES, and on a quiet minute it does not move. So the house's
verification habit for this flake — rerun the target a few times, see green —
cannot tell a fix from a quiet box. Graded here against the last tree that
carried the delta spelling (`7725ba75^`) and against this tree's rig:

| perturbation | pre-fix rig | this tree's rig |
|---|---|---|
| none | **0 of 8 red** | 0 of 8 red |
| ambient claude on/off, 1-3s cycles | 2 of 6 red | 0 of 8 red |
| ambient claude arriving once a second | **5 of 5 red** | 0 of 8 red |

The top-left cell is the finding: eight clean runs of the broken rig, all
green, on a box with 13 live seats — which is the evidence two of these four
beads were closed on. 23-35s per run throughout, either spelling.

## The two perturbations are not equally good, and the difference is monotonicity

An on/off churn only reds when a transition lands between the rig's two
samples, so it detects a third of the time — a harness that is green over a
broken tree two runs in three is worse than none. Arrivals that are
**monotonic** — one new process a second, none killed until the run ends —
make any two samples taken a second apart differ by construction, so a
delta-shaped verdict is wrong every time:

    FAIL a 'claude -p' run changed claude_procs from 19 to 23 -- it must be excluded
    FAIL control: a non-print claude process changed claude_procs from 19 to 27, want 20

That is the pre-fix rig under the ramp. The same ramp over this tree's
membership arms moves nothing, because the only pid in either verdict is one
the rig planted itself.

## What landed

`scripts/reap-selftest-ambient.sh` — the ramp, as a tool rather than a thing
the next bead rebuilds. It copies `tee(1)` to a `claude`-named path under its
own `mktemp -d` root (a real ELF: `/proc/<pid>/exe` resolves a symlink to its
target, so a link named `claude` is invisible to the counter), execs one per
second for as long as the self-test runs, reports each run's FAIL arms, and
kills every fake BY PID. It is nobody's prerequisite on purpose: it costs
half a minute and 25-35 processes to re-answer, probabilistically, what the
membership arms answer deterministically and for free. Type it when you have
edited that rig, or when judging a report that reads like a box-wide count.

Its own positive control is in the table above: it must red `7725ba75^`, and
it does, 5 of 5.

Two properties of it are load-bearing and neither is obvious:

- **The fakes cannot outlive it.** Each child gets the read end of a fifo and
  has the write end (fd 9, held in the harness shell alone) closed, so a
  SIGKILLed harness is EOF on their stdin and `tee` with no file operand
  exits. A fifo held open read-write by the children instead would have
  blocked them forever.
- **Its report is counted with bash's own `read`/`case`**, not `grep -c`. A
  matcher that could not be forked under the load this tool deliberately adds
  would report zero FAIL arms over a red run, which is the one answer it must
  never give (ranger-base-t07yx, and
  `internal/treepins/selftestforkarm_qa_test.go` holds that rule for arms in
  `scripts/`).

The Makefile's description of `verify-reap-test-tmp` also said the finding-4
fixtures were two cold execs of a copied `sleep` and the run was "a few
seconds"; it is `tee` since ranger-base-bttuj and 23-35s measured, and it now
says the arms grade membership and where the ramp is.

## Still open

Nothing in the tree asserts that the arms stay insensitive to the box. The
detector exists and is deterministic, but no door runs it, and wiring one is
a cost every seat on the box pays (half a minute and 25-35 planted
`claude`-exe processes inside `make test`, against a text pin whose rule —
"a verdict is not a delta of a population you do not own" — has no crisp
spelling in shell). That decision, and the pin, are the code lane's:

- ranger-base-tq5tj
