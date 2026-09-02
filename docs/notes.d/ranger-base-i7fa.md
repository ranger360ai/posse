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

## 6. The remaining 63%: what the `newTestBackend` change actually is

Priced in §2 at 1431 tests / 743.2s / 75% of the wall, and left undone. It
is written out here because it is not a sketch — every unknown in it was
resolved while measuring §2, and the edit inventory below is complete.

> **Built since.** `ranger-base-aupee` landed this section at `b2f5b21`:
> `newTestBackend` no longer touches the environment and `RHQ_FAKE_HERDR` is
> set once in `TestMain` (`herdr_test.go:105`), process-wide. "Left undone"
> above is the state this was written in, kept so the plan still reads as one.

`newTestBackend` reaches for the process environment four times where it
wants per-test state:

```go
t.Setenv("HOME", t.TempDir())     // defensive: keep ~/.posse, ~/.claude, ~/.grok, ~/.codex off the operator's real home
t.Setenv("RHQ_FAKE_HERDR", "1")   // constant — read by the CHILD at startup, never by the parent
t.Setenv("RHQ_FAKE_DIR", fake)    // per-test — read by the CHILD, and by 9 parent-side assertions
t.Setenv(EnvPersona, "")          // constant — hermetic against the operator fence (ADR 0031 §2)
```

Three of the four are constants across all 698 call sites and belong in
`TestMain`, once, before `m.Run()`. Only `RHQ_FAKE_DIR` is genuinely
per-test, and only the child needs it.

**The child can be told without the environment: link, don't export.**
`fakeDir()` becomes `filepath.Dir(os.Args[0])` (with the `$RHQ_FAKE_DIR`
read kept as an override, which is what the 6 tests that redirect the fake's
state deliberately use). `newTestBackend` links the test binary into the
test's own fake dir as `herdr`, and points `Herdr{Bin: …}` at the link; a
child exec'd through it finds its state directory from argv[0]. Nothing
process-global is touched, and no product struct grows a test-only field.

A per-test registry (`sync.Map` keyed on `t.Name()`, written by
`newTestBackend`) gives the parent side the same value back, for the call
sites that discarded the returned `fake`.

### Edit inventory (counted, not estimated)

```
herdr_test.go  TestMain            set HOME (one temp dir for the binary), RHQ_FAKE_HERDR, EnvPersona
herdr_test.go  fakeDir()           argv[0] dir, $RHQ_FAKE_DIR still overriding
herdr_test.go  newTestBackend      drop the four t.Setenv; link + register
~30 sites      Bd{Bin: exe}        -> Bd{Bin: fakeBinFor(t, "bd")}; t is in scope at every one
  9 sites      fakeDir()           parent-side reads -> fakeDirOf(t)
  5 sites      exec.Command(exe|os.Args[0], "-test.run=…")  child env drops RHQ_FAKE_HERDR — done, 0410018
```

That last row is the one trap worth naming: those five tests re-exec the
binary to run a NAMED test in a child. With `RHQ_FAKE_HERDR=1` process-wide,
`TestMain` would dispatch that child to `fakeBd` on its `-test.run=…` argv
and exit before the test ran. They are `cagehomelock_qa_test.go`,
`launchlock_qa_test.go`, `trustlock_qa_test.go` (two sites) and
`watchlock_test.go`.

The count read 4 here until `ranger-base-cecvu`. `watchlock_test.go` spells
the binary `os.Args[0]` where the other four spell it `exe`, so a census
grepping `exec.Command(exe` walked straight past it — and it was the one of
the five carrying no filter at all. All five carry it as of `0410018`, so this
row is an edit already made rather than one to make.

Nor is it latent any more, now that `b2f5b21` has made the variable
process-wide. Measured at HEAD: take the `watchlock_test.go` filter back out
and `TestWatchLockDiesWithItsProcess` reds in 0.01s ("child never took the
lock"), with the scanner naming the site in the same run, while
`TestLaunchLockHoldsAcrossProcesses` — a site that does filter — stays green.
The filter is what holds this up today, not a precaution.

Do not re-derive the set from this paragraph: the census is now a pin.
`internal/posse/reexecenv_qa_test.go` scans the package's own `*_test.go` for
both spellings of the binary and fails on any site that hands its child a bare
`os.Environ()`, with a floor of 5 rather than a list, so a sixth site added
later is covered by it and by nothing else. The line numbers this paragraph
used to carry are gone too: they were right when written — still exact at
`0410018^` — and stale one commit later, because `b2f5b21` moved all four.
That is the very commit a reader following this section is standing on.

### The one open design question: a shared $HOME

Moving `HOME` to `TestMain` gives the whole binary one temp home instead of
one per test. That keeps the property `ranger-base-gvrh` bought — no test
cuts a worktree in the operator's live `~/.posse` — but it does NOT keep
per-test isolation of `$HOME/.posse/worktrees`, which is what
`DefaultWorktreeRoot()` returns and where `EnsureSessionTree` writes. Two
parallel tests that pick the same session name would collide.

`App.WorktreeRoot()` already reads config `worktrees:` first and only
enforces that the root be under `$HOME`, so a per-test root under the shared
home satisfies both. Whether that arrives as a config write in
`newTestBackend` or as an `App` field beside `ModelLister`/`Load1`/`TopCPU`
— which `hermetic()` documents as exactly the place for a fake-by-
construction default — is the call to make, and it is the reason this half
was not just typed in behind the first.

### Verify it against the second filter, not just the first

§4's lesson generalises: run the package-level-var filter again over the
newly-clean tests before adding `t.Parallel` to them. Both filters are
scripted and cheap; neither is a substitute for the other.

## 7. Sequencing note for whoever measures this

Top-level tests that call `t.Parallel` never overlap the serial ones — go
runs the sequential pass to completion first, then resumes the paused set
together. Two consequences worth having in hand:

  - the parallel set only ever races against ITSELF, which is why the two
    filters above are aimed there and not at the serial remainder; and
  - `t.Setenv` values are restored before the parallel phase begins, so the
    environment is stable while it runs — which is what makes the
    `$RHQ_FAKE_DIR` override in §6 safe to keep.

## 8. The controlled measurement, and what it says the other half is worth

§5 said no wall-clock verdict taken on this box is trustworthy, and that is
still true of any single number. But the comparison does not need a quiet
box — it needs the same box. `-parallel 1` forces the paused tests to resume
one at a time, so **the same test binary, in the same session, gives both
arms**:

```
go test ./internal/posse -count=1 -timeout 40m -parallel 1     1035.0s   ok
go test ./internal/posse -count=1 -timeout 40m                  697.0s   ok
                                                                817.2s   ok  (repeat)
```

**-32.6%** on the first pair. The two parallel runs differ by 17% from each
other, load having gone from ~8 to ~16 between them, which is the same point
§5 was making — take the ratio, not the number.

### The model predicted this to within 0.4%, so it can be trusted on the rest

§2 priced the parallel set at 37% of the wall. Parallelising a fraction *f*
at eight ways predicts `serial × (1 - f + f/8)`:

```
predicted   1035.0 x (1 - 0.37 + 0.37/8)  =  699.6s
measured                                     697.0s
```

Applying the same arithmetic to the `newTestBackend` half of §6, where
*f* = 0.75:

```
1 - 0.75 + 0.75/8  =  0.344 of serial

  on THIS box (serial 1035.0s, load 8-16)          356s
  on a quiet box (gilfoyle's serial 475.7s)        164s
  under 300s whenever serial is at or below        873s
```

So the bead's `SLOW_PACKAGE_SECONDS=300` line is reachable, and it is
reachable by the one change §6 describes — not by anything else on the
table. The first half alone does not reach it and was never going to: 63% of
a package left serial is 63% of the package.

### Flake evidence for the half that did land

Three full parallel runs (887.3s under load 18-36, 697.0s, 817.2s) plus the
serial control, **zero failures in any of them**, after the two filters of
§4. The one failure that ever appeared, `TestPreflightUNKNOWN…`, appeared on
the run *before* the package-level-var filter existed and has not recurred.
The rest of the repo is green too: `github.com/ranger360ai/posse` 169.8s,
`cmd/posse` 114.5s.
