# internal/posse runs its tests one at a time — where the wall clock is, and what moves it

ranger-base-i7fa, handed off from ranger-base-7xla (gilfoyle) and
ranger-base-2ggb. Both halves of the *gate* were already done: `make test`
carries `-timeout 25m` (pinned by `suitetimeout_qa_test.go`) and
`scripts/test-times.sh` prints per-package seconds against a
`SLOW_PACKAGE_SECONDS=300` line. Neither makes the package faster. This is
the half that does.

Two names in the parent bead are stale and worth fixing here once: the
package is `internal/posse`, not `internal/rhq` (renamed since), and it has
grown from the 1448 tests gilfoyle measured to **1975**.

## 1. The baseline, re-measured

darwin 25.4.0 / go1.26, 8 cpu, 2026-09-02, `go test -json ./internal/posse
-count=1 -timeout 40m`:

```
991.5s package elapsed      310.3s user + 367.8s sys  (= 0.68 of one core, on eight)
1975 top-level tests        sum of every top-level Elapsed = 990.6s
```

That last line is the same finding gilfoyle recorded and it still holds: the
per-test times sum to the package's own elapsed time, so **nothing in this
package runs concurrently with anything else**. `grep -c 't.Parallel()'`
across the test files was 0.

The distribution has no hot spot to attack:

```
tests >= 10s      5     70.3s    7% of the wall
tests >=  5s     23    191.3s   19%
tests >=  2s    124    485.0s   49%
tests >=  1s    242    645.3s   65%
tests >= 0.5s   566    865.9s   87%
```

Slowest single test 19.4s. It is a hundred-plus tests each paying half a
second, and 0.68 of one core says most of that half-second is spent waiting
on `git`, `sandbox-exec` and the fake-herdr re-exec rather than computing.
The re-exec itself is not the cost: a warm run of the 22MB test binary with
no test selected returns in under 10ms.

## 2. Which tests *can* be parallel, in wall seconds

`t.Parallel` panics in a test that has called `t.Setenv`, so the question is
which tests reach the process environment. Answered by a call-graph taint
over every function in the test files (`t.Setenv`/`os.Setenv`/`Chdir` at the
root, closure bodies included), joined to the per-test elapsed times above:

```
                                   tests    wall     share   serial left
env-clean today                      802   364.1s     37%       626.5s
if newTestBackend were env-clean    1431   743.2s     75%       247.4s
```

**That second row is the lever.** gilfoyle's correction already named the
shape — one helper, not 56 scattered decisions — and this prices it: the
`newTestBackend` helper (`herdr_test.go`, 698 call sites) calls `t.Setenv`
four times, and those four calls hold **three quarters of this package's
wall clock** out of any concurrency. What is left after it is a long tail,
led by `wtApp` (48.2s), `wtqaPassWithWork` (26.0s), `hwsFixture` (20.0s),
and direct `t.Setenv("PATH")` (15.7s) and `HERDR_SOCKET_PATH` (11.9s).

Splitting the package instead — the other lever the bead named — was priced
and rejected: a file-level coupling graph over the 93 non-test files has
**exactly one weakly-connected component**. Every file references a
declaration in another. There is no seam to cut without an architecture
change, which is richard's to make, not a code bead's.

## 3. What this bead changed

### `t.Parallel()` on the 758 tests that can take it

The 802 env-clean tests, less 44 excluded in §4. No product behaviour
changes; the tests that touch the environment are untouched and still run
one at a time.

### `resolveOutside` no longer swaps the process $PATH (`gates.go`)

It resolved a binary by setting `PATH`, calling `exec.LookPath`, and
restoring it under a `defer`. The process environment is one variable shared
by every goroutine, so that swap was a window in which any concurrent caller
resolved against a PATH it never asked for. Harmless while this package's
tests ran strictly one at a time; a race the moment any of them calls
`t.Parallel`. It now walks `PathOutsideGates`'s list by hand and touches no
global. Same answer, and `PathOutsideGates` was already a read.

## 4. The class the env taint does not catch: process-global state

The first parallel run came back with one failure, and it is the
instructive one:

```
--- FAIL: TestPreflightUNKNOWNSaysOurGateRefusedUsOnce (2.38s)
    gatedkeychain_test.go:173: the silent UNKNOWN must say our gate refused us: ""
```

That test asserts a **once per process** notice: it deletes a key from the
package-level `gateRefusalNotices` sync.Map, provokes the notice, and reads
it back. Its own comment already says "another test in this binary may have
spent this key already" — and deleting the key first is a complete answer
while tests are serial and no answer at all when they are not. Another test
spent the key between the delete and the read.

So an env-clean test is not thereby a parallel-safe test. A second filter
was run: every package-level `var` in the package (139 of them, enumerated
with `go/ast`, not grep), narrowed to the 37 that are written somewhere —
assigned, indexed, appended to, or given a `Store`/`Delete`/`Do` — and then
every candidate test that so much as names one. That is 44 tests, and they
keep running serially. They cost 11.8s of the 364.1s, so the filter is
nearly free:

```
parallel set   802 tests / 364.1s   ->   758 tests / 352.3s
```

The 37 are the usual suspects and they read as a map of what this package
shares: injected clocks (`backupAt`, `govNow`, `expiryNow`, `grokPoolNow`),
once-per-process notice sets (`gateRefusalNotices`, `legacyHomeNotices`,
`cwdFallbackNotices`, `runtimeKeyNotices`), `sync.Once` probes
(`sandboxApplyOnce`, `innerProbe`, `imagePosse`), and the mutable tables
(`AvailableCages`, `OpsPatterns`, `planAdapters`, `shippedExampleDigests`).

## 5. What is NOT done, and the number that says so

The package is still over the 300s line, and this bead does not get it
under. The arithmetic is in §2: parallelising the env-clean 37% leaves the
env-tainted 63% serial, and 63% of a 991s package is 626s on its own. **Only
the `newTestBackend` row reaches 300s**, and that is a separate change with
real design content in it — see the follow-on bead.

### The box could not measure it either

Every wall-clock number in this note beyond the baseline is unusable as an
absolute. Measured during this session, on the machine that runs every
session's suite:

```
load averages: 33.18 21.36 18.80      9 concurrent `go test` processes, 8 cores
```

Under that, the 758 parallel tests' summed elapsed rose from 352s to 1784s —
not because they got slower but because eight-way parallelism on a box with
nine other suites on it is a queue, not a speedup. The serial portion was
unmoved (626.5s -> 622.7s), which is the control that says the inflation is
contention and not the change.

**A `SLOW_PACKAGE_SECONDS` verdict on this package is only honest on a quiet
box.** That is worth knowing independently of this bead: the same load makes
`make test` itself unrepresentative, and ranger-base-0uth is the run where
it took the package past the 25m budget at 1981s.
