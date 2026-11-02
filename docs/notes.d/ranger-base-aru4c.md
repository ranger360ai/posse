# The cold-first-exec census, and the two sites it found that the grep did not (ranger-base-aru4c)

Tree `37bd9cbe`, 2026-11-02, darwin 25.6.0. discovered-from
ranger-base-y9q7l, which fixed the same shape in
`TestReadCLIVersionGivesUpOnACLIThatWillNotAnswer`.

**THE CLASS.** A test plants a brand-new executable (`WriteExecutable` into a
`t.TempDir()`) and then execs it under a deadline of a few hundred ms. macOS
assesses a freshly written executable on its FIRST exec, and that assessment
costs two orders of magnitude more than the exec that follows it, so a small
deadline over a cold exec decides its verdict on process startup rather than
on the code.

The class was already known here. `hangingBin`'s header
(`internal/posse/watchhang_qa_test.go`) has carried the doctrine and a
MEASURED reading since 2026-09-03 — "more than 300ms and less than 2s" — and
two fixture helpers already implement the answer. ranger-base-y9q7l
rediscovered it independently; this bead is the third pass. What was missing
was the census.

## The measurement

ranger-base-y9q7l's readings (five, loadavg 5.7-44.4): first exec mean
164-714ms, worst **2.17s**; re-exec of the same path mean 6.3-8.8ms, tail
7.8-21ms. The largest worst case came at the quietest loadavg of the five
and the cheapest reading of all at the busiest, so **load average does not
predict where a cold exec lands.**

Independently MEASURED 2026-11-02 for this bead, n=12 pairs, one fresh
`t.TempDir()`-shaped script per pair, loadavg(1m) 21.94:

| | mean | median | worst |
|---|---|---|---|
| FIRST exec of a freshly written script | 156.3ms | 149.5ms | 222.4ms |
| RE-exec of the SAME path | 10.9ms | 10.2ms | 16.4ms |

cold readings, sorted (ms): 144 145 148 149 149 149 150 153 154 155 157 222.

That **confirms the ratio — 14x — and the ~150ms floor, and did not
reproduce the 2.17s tail in 12 samples.** Consistent with the finding above:
the tail is rare and does not track load, so a dozen samples is not the
instrument that finds it. The floor alone is what makes the class real; the
tail is what makes a 2s bound unsafe rather than merely tight.

## The census

Method, re-derivable: split every tracked `*_test.go` into top-level
functions, take the functions that call `WriteExecutable`, and collect the
duration literals at or under 3s in each one **and in its callers in the
same file** — because the planting is in a helper and the deadline is in the
test. That produced 14 candidate rows, and **10 of the 14 needed a judgement
no matcher made**: whether the duration bounds an `execve` of the planted
file at all.

Two adjudication rules did most of the work, and both are reusable:

1. **A fixture driven through `Herdr`/`HerdrMux` pays its cold exec outside
   every deadline the test sets.** `mux.Start` → `Herdr.PaneRun` → `Run` →
   `capture` → `cmd.Run()` is *synchronous*, bounded by
   `HerdrControlTimeout` (2 minutes), and every caller computes its own
   `deadline := time.Now().Add(...)` **after** it returns. So the first exec
   is already paid, and the test's deadline covers only warm re-execs.
2. **`sh <path>` is not an exec of `<path>`.** The kernel execs `/bin/sh`,
   long warm, and the planted file is opened as data. No first-exec
   assessment happens at all.

### In class — fixed under this bead

Both in `internal/posse/bdwritelock_test.go`, and **neither was named by the
bead**, which grepped `WriteExecutable` against small durations.

- `TestRunOnceHoldsWriteLockAcrossChild` — `waitForFile(t, marker,
  3*time.Second)` over the first exec of a planted `bd`. A 0.83s margin on a
  2.17s worst case, and the fatal it would print, "`started` never appeared
  within 3s", names `runOnce` for Gatekeeper's cost.
- `TestRunOnceDoesNotLockOnARead` — `time.After(2 * time.Second)` over
  `runOnce`'s first exec of a planted `bd`. **This bound sat BELOW the
  measured worst case**, and it is the worse of the two for a second reason:
  its fatal reads "a read verb waited on a write lock held by somebody
  else" — the defect under test, reported over code that is fine. A flake
  with an honest message costs a re-run; this one costs a bead.

The fix is the one that worked on ranger-base-y9q7l: a pre-flight invocation
that pays the cold exec OUTSIDE the deadline, so the timed call re-execs a
warm path. **Both halves are asserted** — that the pre-flight ran (or it
warmed nothing and the deadline is back over a cold exec) and that it did
*not* run the body (or the witness is the pre-flight's own mark and not the
child's). In the first site `--warm` guards the sleep and not the touch, so
the mark proves the body's first line ran, and it is cleared behind the
pre-flight; in the second the body has no side effect and its own answer is
the witness, so it needs no guard.

MUTATION-CHECKED, three mutations, each reproducing its own witness and
nothing else:

| mutation | witness that fired |
|---|---|
| drop the `--warm` guard | `the --warm run fell through its guard and ran the body ("{}\n")` |
| move the guard above the touch | `the --warm run never reached the body's first line (stat …: no such file or directory)` |
| read-arm body answers nothing | `the --warm run answered "", not the body's own line` |

### Not in class — and why

- `TestProbeSessionExeRefusesWhatItCannotName` (`runtimeprobe_test.go`,
  300ms) — **the bead's site 1, and it is safe**, by rule 1 above:
  `probeSessionExe` computes its deadline after a synchronous `mux.Start`,
  so the fake herdr's cold exec is paid under the 2-minute control timeout.
  Arms 1 and 2 find the answer file already written at the first `ReadFile`.
  The bead asked whether **arm 3** (`exit 0`, expecting "did not say which
  binary") would pass over a fixture that never ran at all: **it would not.**
  An exec that fails returns from `mux.Start` as a `herdr pane run: …` Die,
  `probeSessionExe` returns it unchanged, and the arm's
  `strings.Contains(err, "did not say which binary")` does not match — so
  the arm reds. Its green is not a green that measured nothing.
- `RuntimeProbe` with `ProbeOpts{Timeout: 2 * time.Second}` (two sites) —
  same rule 1: `mux.Start` precedes `deadline := time.Now().Add(o.Timeout)`.
- `hangingGit` (`githang_qa_test.go`, 250ms) — **the bead's site 2, and it
  was already fixed** — warmed by a `--warm` pre-flight since the file's
  first commit, `9393e5f2` (ranger-base-zfza8). The bead predicted a red
  that the fixture already prevents.
- `hangingBin` / `hangingBinWith` (250ms-2s) — warmed; the doctrine's home.
- `TestBdBinUnresolvableRefusesWithoutWaitingOnTheStoreLock` (150ms) — the
  duration bounds a `bdWriteLock` flock probe, not an exec, and that
  fixture's planted `bd` is deliberately never run.
- `liveProcess` (`watchpid_test.go`) — rule 2: it runs `sh script`. The
  10/20/40ms in its caller are `Watch` poll intervals.
- `egress_test.go` — the bound is `egressWait = 30s`; the 20ms is a poll
  sleep.
- `hookwallsweep_qa_test.go` — 10/20ms are `Watch` poll intervals; the hang
  guard is 90s.
- `cockpitbeadscan_test.go` — the 3s is a `sleep` inside the fake bd, used
  as a separation, and the wait on it is bounded at 60s.
- `TestReadCLIVersionGivesUpOnACLIThatWillNotAnswer` (3s) —
  ranger-base-y9q7l's own site, in flight there. Not touched here.

## Why this bead leaves no pin

The class has been found three times and the obvious answer is a pin that
reds a new unwarmed fixture. **The subject is not mechanically decidable at
the precision the class needs**, which is the finding, not an excuse: 10 of
the 14 candidate rows turned on whether a duration bounds an `execve` of the
planted path, and answering that took reading `Herdr.capture` to find a
synchronous `cmd.Run` two calls down, and knowing that `sh <path>` execs
`/bin/sh`. A matcher that cannot make those two distinctions either reds the
whole `runtimeprobe`/`watchpid` family or stays green on the two sites this
census found. Filed as debt, ranger-base-gdzu5, rather than invented here:
its done-when takes either answer, a decidable pin or a recorded decision
that the class is held by prose.
