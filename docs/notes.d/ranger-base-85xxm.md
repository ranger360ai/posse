# ranger-base-85xxm — the sweep's own death, and a gate pinned by the abort below it

*2026-11-14 · gilfoyle · `scripts/reap-test-tmp.sh --self-test`, the KEEP/COLLECT rig.*

Every number below is MEASURED on this box (linux 7.2.15 x86_64, bash
5.3.20(1), `/tmp` a 14G tmpfs) on 2026-11-14, tree at `e3b41b30` plus this
change. Every mutant was applied to a **copy** of the script under a
scratchpad and run by path; the tracked file's md5 was printed before and
after each battery and never moved.

This is the same class ranger-base-bttuj closed, one apparatus up.
ranger-base-bttuj made the four background HOLDERS' death visible; the
SWEEP's own death was still not. Its lesson, verbatim from its commit
message, is the test that found this: *ask which way an arm fails if its
fixture dies.* The sweep is the biggest fixture in the rig. The finding is
holden's, on ranger-base-uc338; this is the fix and its grading.

## Half 1 — the sweep's exit status and output were both discarded

The rig's census was one line:

```sh
"$0" --root "$tmp/root" --class claude-scratch --apply --quiet \
  --claude-sessions "$tmp/sessions" --claude-projects "$tmp/projects" --no-bead \
  >/dev/null 2>&1 || true
```

so a sweep that aborted mid-run was indistinguishable from one that ran and
decided. With `exit 3` injected immediately after the census lines and before
the class walk:

```
  ok   A kept
  ok   D kept
  ok   E kept
  ok   F kept
  ok   H kept
  FAIL B: want COLLECTED, got KEPT
  FAIL C: want COLLECTED, got KEPT
  FAIL G: want COLLECTED, got KEPT
```

Five arms green over a population nothing ever collected from, three reporting
a reaper defect when what happened is that the reaper exited, and no line of
the output naming the abort. That is the mirror of the sentence the file
already carried about A, D and H, and it had no arm either.

**Found the hard way, for free.** The first battery run for this bead had a
mutator that wrote each copy with `open(dst, "w")` — mode 0644. The script
re-invokes itself as `"$0"`, so every sweep died with `Permission denied` into
a discarded stderr, and the control copy — no mutation at all — printed
*exactly* the eight lines above. Four mutants were read as findings before the
control was run. The defect demonstrating itself on the way to being fixed is
the strongest argument for the arm that now exists; the battery chmods its
copies and refuses to run one that is not executable.

### The fix, and the two observables

The sweep's output is captured and its status kept (`|| sweep_rc=$?`), and one
arm reports a broken RIG, before any KEEP/COLLECT verdict, when either
observable says the sweep did not finish:

- **the exit status** — what `set -euo pipefail` turns any abort in the sweep
  into. Repro: the `exit 3` mutant above.
- **the `worst: … -> …` line**, which the sweep prints *after* its removal
  loop. An `exit 0` planted early is the one abort an exit status cannot see.

It is that line and deliberately **not** `collected N of M dirs`, which is
printed only when the walk found something: a reaper that collected nothing
would then be reported as a broken rig — this bead's own defect with the blame
reversed. Both are read with bash parameter expansion, no forked matcher
(`TestQANoAssertionArmDecidesThroughAForkedMatcher`).

## Half 2 — the registered-pid liveness gate was pinned only by a pipefail abort

Dropping `[ -d "/proc/$rpid" ] || continue` red three arms, which is what
ranger-base-bttuj recorded — but it red through half 1, not through any arm's
verdict. With the gate gone, the `sed -E 's/.*\) //' "/proc/$rpid/stat"` two
lines below it is handed a dead pid's missing file and the script aborts. Two
mutants, one change at a time:

| mutation | result |
|---|---|
| **N0** that one pipeline made crash-proof (`{ sed … \|\| true; } \| awk …`) ALONE | all arms pass, exit 0 |
| **N1** N0 **plus** the gate deleted | all arms pass, exit 0 |

N0 is the control that says the crash-proofing changes no verdict of its own.
N1 is the finding: the gate's only killer was an abort one `|| true` away from
gone, and a plausible robustness edit made for an unrelated reason would have
taken it with it in silence.

**Why no fixture covered it.** The gate alone decides exactly one combination
— a registered pid that is DEAD and carries NO procStart — and the rig had no
such row. B is dead-with-procStart, so the procStart match just below the gate
excludes B whatever the gate does; D is live-without-procStart. The two halves
of B and D never met.

### The fix: row I, with its own uuid and its own registry file

`mk I yes no; reg I "$deadi" OMIT`, wanting COLLECTED. Beside B and never an
edit to it: that shape bolted onto B reaches `claude_live++` and the `for sd`
walk, which arms B through G and the three class rigs then read differently
(measured on ranger-base-uc338).

With the gate present, I's entry is skipped at the gate, nothing holds its
dir, and it is collected. With the gate gone, I reaches `claude_live++`, its
dir goes into `$livedirs`, and the dir is kept — so the gate is now pinned on
its semantics and not on the abort below it.

### And the dead pids are now derived and checked

B registered a hardcoded `999999`. linux's pid counter wraps at
`/proc/sys/kernel/pid_max`, and this box was already issuing pids past
4,000,000 against a `pid_max` of 4,194,304 — so `999999` is a number the
kernel will hand out again, and on that wrap B and I would both be registering
a LIVE process and reporting `want COLLECTED, got KEPT`: a reaper defect over
a rig that is what moved. Both rows now take `pid_max` and `pid_max + 1`,
which are one and two past the largest pid the kernel allocates, and one arm
checks that claim with `[ -d /proc/<pid> ]` rather than carrying it. Same
failure direction as the four holder rows, from the other side.

## Grading, all six rows of one battery

| mutant | before this change | after |
|---|---|---|
| none (control) | all arms pass | all arms pass, `ok the sweep ran to completion` |
| `exit 3` after the census, before the walk | 5 arms green, B/C/G red as a reaper defect | **sweep arm red first**, naming the rig and the exit status |
| `exit 0` planted in the same place | 5 arms green, B/C/G red as a reaper defect | **sweep arm red first**, naming the missing `worst:` line |
| N0 (crash-proofing alone) | all arms pass | all arms pass |
| N1 (N0 + gate deleted) | all arms pass, exit 0 | **1 arm red, and it is I** |
| gate deleted alone | B/C/G red as a reaper defect | sweep arm red first, naming exit 2 |
| `deadi=$$` (a dead-pid row made live) | — | **rig arm red**, naming arm I and the live pid |

`make verify-reap-test-tmp` green in 7.4s (the documented 7.3-7.5s band is
unchanged), and it is a prerequisite of both `make test` and `make test-arm1`,
so all of this is in the suite.

## What this did not touch

The production code. Every one of ranger-base-bttuj's three claims was
re-measured on ranger-base-uc338 and reproduced; nothing above is a reaper
defect. This was a pin that was wrong about what it measured, which ADR 0006
§6 names as live.
