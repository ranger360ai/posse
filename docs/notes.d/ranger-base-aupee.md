# The other 75%: `newTestBackend` stops touching the process environment

ranger-base-aupee, the second half of docs/notes.d/ranger-base-i7fa.md and
the build of ADR 0047. i7fa's §6 priced this change and left it: one helper,
four `t.Setenv` calls, 728 call sites, and three quarters of `internal/posse`'s
wall clock held out of any concurrency because `t.Parallel` panics in a test
that has set the environment.

## 1. What changed, against the inventory §6 wrote

| §6 said | what landed |
|---|---|
| `TestMain` sets HOME, `RHQ_FAKE_HERDR`, `EnvPersona` | as filed, plus `operatorHome` (below) |
| `fakeDir()` reads argv[0]'s dir, `$RHQ_FAKE_DIR` still overriding | as filed |
| `newTestBackend` drops the four `t.Setenv`; links + registers | as filed |
| ~30 `Bd{Bin: exe}` → `fakeBinFor(t, "bd")` | 31 sites, plus 2 `RHQ_BD_BIN` sites §6 did not count |
| 9 parent-side `fakeDir()` → `fakeDirOf(t)` | 9, plus 1 raw `os.Getenv("RHQ_FAKE_DIR")` §6 did not count |
| 4 `exec.Command(exe, "-test.run=…")` must clear `RHQ_FAKE_HERDR` | **3 of the 4 already did**; the one that did not is a FIFTH site §6 missed |

Plus ADR 0047 D2: `App.WorktreeRootDefault`, set by `hermetic(t, a)` to
`$HOME/worktrees/<t.Name()>`.

### The fifth re-exec site, and the three that were already right

`qaSeederEnv` (trustlock_qa_test.go:97) already stripped `RHQ_FAKE_HERDR`
from the child's environment, and so did `TestLaunchLockHoldsAcrossProcesses`
by hand — defensively, back when only the backend tests set the variable. So
three of §6's four rows were already done. The row it did not have is
`watchlock_test.go:84`, which built its child's environment with a plain
`append(os.Environ(), …)`: harmless while `RHQ_FAKE_HERDR` was per-test and
that test never called `newTestBackend`, and a child that exits as the fake
bd without running a line of the test the moment the variable is process-wide.

The lesson is the one worth keeping: **an inventory of what must change is
taken against the state of the code, and hoisting a variable from per-test to
per-binary changes who reads it.** The right question was not "which four
tests re-exec the binary" but "which tests re-exec the binary at all".

### `operatorHome`

`TestMain` replaces `$HOME`, so after D1 nothing in the binary can name the
home the operator actually has. Two things needed it:

  - `TestTheTestBinaryGetsATempHome` (was `TestNewTestBackendGetsATempHome`)
    is ranger-base-gvrh's pin — no test cuts a git worktree in the operator's
    live `~/.posse` — and it checks that by comparing `$HOME` against the real
    one. The guarantee moved from the helper to `TestMain`; the pin still
    measures it, by running `EnsureSessionTree` for real.
  - `qcRunbook` reads `~/src/ranger-base/docs/runbooks/queue-cutover.md`, a
    tree that lives outside the checkout, and skips when it is absent. On a
    temp `$HOME` that skip fires on every box — a green suite measuring
    nothing. It reads `operatorHome` now.

## 2. What the shared `$HOME` broke, and it was not concurrency

D1 gives the binary one temp home instead of one per test. ADR 0047 D3 says
to audit the `$HOME` readers before adding `t.Parallel`, on the theory that
the risk is two parallel tests in one directory. The audit found a defect
that is not about parallelism at all:

```
--- FAIL: TestGrokPoolCountsInteractiveSessionsToo
    the operator's own grok spends the same pool, got n=0:
    grok pool: estimated 100% of the weekly pool used
--- FAIL: TestGrokPoolOverThresholdSkipsTheBead      … 180%
--- FAIL: TestGrokPoolAtThresholdRunsAndLogsTheFactor … 200%
```

`grokPoolPassFull` planted a session transcript under `os.UserHomeDir()` and
the guard sums **every** session under `$HOME/.grok`. These five tests are
serial — they were never in the parallel set — and they still read each
other's spend, because a per-binary home is shared by serial tests too. Six
tests in all, counting `verifyesa0j_qa_test.go`'s pair.

`grokPoolHome(t)` is the fix and it is ADR 0047 D3's own prescription: the
test takes its own `$HOME`, which also keeps it serial by the runtime's rule
with no list to maintain.

## 3. The three filters

i7fa's two were scripted and not committed, so they were rewritten with
`go/ast` — and committed this time, as `cmd/testparallel`: call-graph taint
over the test files, package-level var writes, D3's roots, and the
hand-named serial set. Run against HEAD as a control they
reproduce i7fa's shape — 872 env-clean where i7fa counted 802 — and the 31
tests in the gap are static assertions i7fa's pass left serial.

```
                                    HEAD    this bead
top-level tests                     2120         2120
env-tainted (stay serial)           1248          597
env-clean                             872        1523
  less: name a written package var     66         136
  less: named serial by hand            0          23
= eligible for t.Parallel             806        1364
```

In the tree: **1389 tests carry `t.Parallel()`, up from 800** — 599 added, 10
of i7fa's taken away (the lock tests of §4 and the cross-process children
below). The 25 that are parallel without being eligible are i7fa's, kept
because they shipped green: this bead's filter reads them as var-tainted off
`blindT` and `backupAt`, package-level `time.Time` fixtures that count as
written because they are given `.Add`.

### The 23 named serial

Five are the child halves of the cross-process tests —
`TestQASeedTrustChildSeeder`, `TestQASeedTrustChildHoldLock`,
`TestQASeedCageHomeChildSeeder`, `TestLaunchLockChildHolder`,
`TestWatchLockHolderChild` — re-exec'd with `-test.run=<name>` and read for
one line on stdout against a deadline. Pausing them into the parallel phase
buys nothing and moves a line the parent is waiting for.

The other eighteen assert on flock acquisition; §4 is why.

### Two exemptions from the package-var filter, both named

The filter is a taint heuristic, and two of the vars it flags are the
harness's own:

  - `fakeDirs`, the per-test registry this bead adds: written only as
    `Store(t.Name(), …)` / `Delete(t.Name())`, read only as
    `Load(t.Name())`. A key space partitioned by the test's own name over a
    `sync.Map` — no two tests touch one entry. Left in, it taints 663 of the
    tests this bead exists to free, so it is named here rather than waived
    quietly.
  - `operatorHome`: written once in `TestMain`, before `m.Run`, read-only for
    the rest of the run.

Both are the argument ADR 0047 D3 already makes for `SeedClaudeTrust`'s
per-repo key: a write under a key no other test can name is not shared state.

### D3, and what reading it cost

D3's root set is the 15 `$HOME`-reading functions outside a test file (ADR
0047 counts 17 `os.Getenv`/`os.UserHomeDir` sites across the same files).
Propagated to tests, that is 1235 of the eligible set — every launch reaches
`planLaunch` — so the list is only tractable narrowed to what the ADR's own
reasoning says matters: the one writer, and the tests that reach the file it
writes.

```
eligible AND touch the claude config in test code        29
eligible AND read $HOME from their own test code          2
```

All 31 were read. Every one points at a config path it built from its own
`t.TempDir()`, compares path strings, or does arithmetic on `$HOME` without
touching the disk. **None asserts the shared file whole or absent, so none
moved to serial.** The write itself is `SeedClaudeTrust` under a flock, keyed
on the per-test repo path — concurrent seeds serialize and do not clobber,
exactly as the ADR said.

The class D3 was aimed at did fire, but through a root the claude-config pass
did not cover: a TEST that writes under `$HOME` itself (§2). The second
filter row above is that one, and it is now 2 — both read-only.

## 4. The lock tests, and why 23 of them are serial by name

The first full parallel run of the finished tree came back with

```
--- FAIL: TestLaunchLockFreeAfterRelease
    a free lock made the next launcher wait:
    ⏳ launcher lock held by this process (pid 30608, another goroutine)
```

and, in another run, `TestWatchLoopRunningTracksTheLock` saying the same
thing in its own words. Both are per-test lock files — `LaunchLockPath` and
`WatchLockPath` are `a.StateDir` joined, and every App here is a `t.TempDir`
— so a lock released by `Close` was read back as held by a process that
holds nothing.

It reproduces, and the shape of the repro is the finding:

```
-test.run '^TestLaunchLockFreeAfterRelease$'  -count=60 -parallel 8    0 failures
-test.run '^TestLaunchLock'                   -count=60 -parallel 8    3 and 6
-test.run over launchlock + watchlock          -count=25 -parallel 8   4, three of
                                                                       them watchlock
-test.run over the watchlock family alone     -count=400 -parallel 8   0
```

**The tests interfere across files.** One of them alone cannot fail; the
watch-lock tests are green over 400 iterations until the launcher-lock tests
run beside them. One theory was tried and killed: `runtime.KeepAlive(f)` in
the shared `flock` helper (launchlock.go:185) — the documented remedy for
`syscall.Flock(int(f.Fd()), …)` — measured **6 failures patched against 3 on
the control**, same arm, so it is not the `os.File` lifetime.

Filed as ranger-base-9l77f with the numbers and the dead theory. Here, the
23 tests that assert on flock *acquisition* are named serial in
`cmd/testparallel`, which is where the next pass will look before it adds
`t.Parallel` back. The hundreds of tests that merely TAKE the launcher lock
on their way through a dispatch pass are untouched and stay parallel — it is
the assertion about who holds it that cannot be trusted beside another one.

## 5. The controlled measurement

i7fa §8's method, unchanged: one `go test -c` binary, both arms in the same
session, `-parallel 1` forcing the paused tests to resume one at a time.
Taken twice, over two trees a few commits apart — the second is the shipped
one, with the 23 lock tests serial:

```
tree           arm            seconds  result   load (1/5/15)
first          -parallel 1       2072   ok      40 42 45
               default            753   FAIL*   43 36 38
               default            931   ok      43 55 47
               default            660   ok      37 31 34
shipped        -parallel 1       1818   ok      19 37 41
               default            940   FAIL†   111 85 59
               default           1212   ok      57 53 60
               default            803   ok      34 30 39
               default           1069   ok      33 41 42
```

`*` the lock flake of §4, before it was contained.
`†` ranger-base-ehllm, below: a pin that flakes at ~5% whether or not
anything runs in parallel. The fourth run is the third green one on the
shipped tree, which is the three this bead owed.

```
ratio to serial   first tree   0.36  0.45  0.32        mean 0.38
                  shipped      0.52  0.67  0.44  0.59  mean 0.55
i7fa's model (f = 0.75)        1 - f + f/8       =       0.344
```

**The first tree's pair matches the model; the shipped tree's does not, and
the reason is in the load column.** The shipped serial arm ran at load 19–41
and its parallel arms at 34, 57 and 111 — eight-way parallelism on a box
already carrying five other suites is a queue, not a speedup, which is
exactly what i7fa §5 measured and warned would make any absolute number
useless here. The one pair taken with both arms at comparable load (the
first tree, 40 against 37–43) is the one that lands on 0.344.

So the honest statement is i7fa's, unchanged: the change is worth between a
half and two thirds of the wall on a box like this one, and **no
`SLOW_PACKAGE_SECONDS=300` verdict is available without a quiet box.** On
gilfoyle's quiet serial of 475.7s the model's 0.344 is 164s and the measured
0.38 is 181s; 300s is reached for any serial at or below 790–870s. Nothing
in this session can check that, and this bead does not claim it.

### The other failure, and it is not this bead's either

```
--- FAIL: TestQABackupExitHatchIsPlainTar
    a flipped byte extracted into a fully working store — this pin measures nothing
```

The shown-able-to-fail arm flips `body[len(body)/2]` of the archive and
requires the extracted db to stop answering. Measured base rate on this tree:
**1 failure in 40 at `-parallel 8`, 3 in 40 at `-parallel 1`** — the serial
arm is the worse one, which is the control that says this bead's `t.Parallel`
did not cause it. What the archive holds (a git bundle whose pack bytes are
not byte-stable, and a sqlite db) moves the midpoint between its members, and
a flip landing past the db leaves the db extractable. Filed as
ranger-base-ehllm with the fix: flip a byte the archive cannot survive.

## 6. What is NOT done

  - Each `-parallel 1` control was taken once, on a loaded box. It is a
    control, not a promise.
  - The 56 tests the package-var filter over-reads (a package-level
    `time.Time` given `.Add` counts as written; `blindT`, `backupAt`) could
    take `t.Parallel` and do not. Measured, not taken — a parallelisation
    bead is the wrong place to relax the filter that keeps it honest.
  - ranger-base-9l77f: 23 lock tests are serial because the product's
    behaviour under in-process concurrency is not understood, not because
    they are slow. That is a product question, still open.
  - `cmd/testparallel` is the tool, committed this time so the next bead does
    not re-derive it. It answers for any package, not just this one.
