## The wall clock was already gone — this is the verification, and the one arm it could not run (ranger-base-ci9e)

`TestQALateExplainErrorStillFailsLoudlyNamingTheGuess` was filed as a test
that inverts on a slow box: it built the ordering it needs out of three wall
clocks (a 900ms `StartupWait`, a 100ms `Poll`, and a goroutine that slept
700ms before planting the explain error), so on anything ~3x slower the error
landed *after* the window had already been decided, the launch took the
"prompting on its idle anyway" concession path, and all three assertions fired
at once. Measured 3/3 red on emulated `linux/amd64` against 3/3 green on
native `linux/arm64`; then 3/30 red on **this** box on an ordinary day, and
once inside a full `make test` at loadavg 14-29.

**The fix landed before this bead was picked up**, in four commits none of
which is this bead's:

| commit | what it did |
|---|---|
| `0ebdbce` (ranger-base-4pjw) | armed the late error by call count in the `bootrace` twin |
| `ccc6d8c` + `6ee039e` (ranger-base-9mwa) | the same lever here, in `verify_nx85` |
| `ff1779e` (ranger-base-t1aq) | sized the window at 4s off a measured load, not a guess |
| `e584224` (ranger-base-3wc7) | the early-error arm, errored by count instead of deleted by timer |
| `f59bffd` (ranger-base-vstc) | same class, the prompt-ready gate's waited note |

The ordering is now a signal: `explain-error-after: 2` makes the first two
`agent explain` calls answer and every one after that fail, so the last
explain of the window is the failed one *by construction* and nothing about
it is timed. What is left time-driven is only the **margin** — the window has
to be wide enough to serve more than the countdown consumes — and that failure
mode is a loud `fixture unmet: N explains ...` naming the box, not an
inversion into the concession path.

So this bead's remaining work was to measure the claim, at HEAD, rather than
to read it.

### VERIFIED AT `bf9d503`, darwin 25.4.0 / go1.26.5 / arm64, 8 cpu

Built once with `go test -c ./internal/posse` and run from the package dir.

**Baseline.** `-test.run ExplainError -test.count=30`, loadavg 8-14 throughout:

    TestQAExplainErrorOnStderrStillPromptsOutLoud                30/30 PASS
    TestQAGuessesForTheWholeWindowAreLostToOneLateExplainError   30/30 PASS
    TestQALateExplainErrorStillFailsLoudlyNamingTheGuess         30/30 PASS
    TestQAAnEarlyExplainErrorDoesNotOutliveALaterGuess           30/30 PASS
    rc=0, 120 of 120

Against 3 of 30 **red** on the timer fixture, measured on this box on
2026-08-30 on a quieter one than this run had.

**Mutation — the pin must die when the loud failure does.** `dispatch.go:3927`,
the end of `awaitSettled`'s window, swapped for the quiet return the pin
exists to refuse:

    -   return "", AgentDetection{}, Die("agent in %s never became promptable ...
    +   _ = Die("agent in %s never became promptable ...
    +   return status, AgentDetection{State: status}, nil

`-test.count=3` against that binary:

    TestQAExplainErrorOnStderrStillPromptsOutLoud                3/3 PASS  (no window; correctly indifferent)
    TestQAGuessesForTheWholeWindowAreLostToOneLateExplainError   3/3 FAIL
    TestQALateExplainErrorStillFailsLoudlyNamingTheGuess         3/3 FAIL
    TestQAAnEarlyExplainErrorDoesNotOutliveALaterGuess           3/3 FAIL

and it dies on the right lines — `verify_nx85_qa_test.go:127` ("a window of
guesses is a real answer and must fail loudly") and `:130` ("herdr's own word
for the guess is the diagnosis"), 3 of 3 each. Not on the fixture witness,
which is the way a green pin over a starved window would have reported.

### THE MARGIN, MEASURED — because the platform arm could not be

A probe build (`t.Logf` of the explain count, next to the witness that already
computes it; the source was restored and the tree is clean) run at three load
levels, 5 repetitions each, `n=15` window observations per level. The
no-window sibling in the same `-run` set is the load control: it does the same
launch work with no 4s window in it, so its duration is what the load *costs*.

| spinners | explains in the 4s window (needs > 2) | no-window sibling | result |
|---|---|---|---|
| 0 | 33-35 | 0.79-0.81s | 20/20 PASS |
| 8 (1/cpu) | 31-32 | 1.09-1.42s | 20/20 PASS |
| 16 (2/cpu) | 30-31 | 1.15-1.91s | 20/20 PASS |

**Read the two columns together.** Sixteen spinners on eight cores cost the
sibling a factor of 2.4 in wall time and cost the window four explains out of
thirty-three. The window is poll-bound, not work-bound: `4s / 100ms` caps it
at 40 and it serves 30-35, so a whole iteration costs ~121ms of which 100ms is
the fixed sleep. For the fixture to go unmet an iteration would have to reach
`4s / 3 = 1.33s`, which is ~11x what it costs here — and ~13x the ~26ms of
actual work per iteration.

That is the number the emulated arm would have produced. The emulation factor
this bead measured for `linux/amd64` was ~3.6x (3.48-3.56s against 0.87-1.00s
native, on the 900ms fixture). At 3.6x an iteration is ~440ms and the window
still serves ~9, three times what the fixture needs.

### THE ARM THAT DID NOT RUN, AND WHY

The bead's literal control is `scripts/test-linux.sh go test ./internal/rhq
-run ExplainError -count=3 -v` on both `linux/amd64` (emulated) and
`linux/arm64` (native). **It is unrunnable on this box.** Docker was abandoned
here by operator ruling `ranger-base-6mz7` on 2026-08-30 — one day after this
bead was filed — and `scripts/test-linux.sh` carries that ruling in its own
`die()`: the binary is still on `PATH`, the daemon is down, and the script
refuses rather than telling anyone to start it. No other container engine is
installed (`podman`, `colima`, `lima`, `nerdctl`, `finch`: all absent).

So the platform half of the control is replaced by the margin table above,
which answers the question the platform arm was asked to answer — *does a
slower box starve this window?* — with a measured factor rather than with a
second architecture. The native-amd64 arm the release actually depends on runs
on every tag anyway: `.github/workflows/release.yml` runs `make test` on
`ubuntu-latest`.

The package also moved since the bead was written: `internal/rhq` is
`internal/posse` (`9c00e19`), so the control command's path needs updating
before anyone runs it elsewhere.

### THE CLASS SCREEN

The bead asked for a sweep of `internal/rhq`'s tests for "a `time.Sleep` in a
goroutine that has to land inside a `StartupWait` window". **None is left.**
Every `time.Sleep` in `internal/posse/*_test.go` was read; the survivors are
three shapes, none of them this one:

- **Poll-until-condition loops** — `dispatch_qa_test.go:671`,
  `herdrevents_test.go:721`, `watchlock_test.go:226`, `watchpid_test.go:141`.
  A slow box goes round the loop more times; there is no ordering claim to
  invert.
- **Sleeps whose slow direction is the safe one** — `launchlock_qa_test.go:367`
  waits 200ms before asserting an *absence*, and `dispatch_qa_test.go:402`
  sleeps 300ms deliberately longer than a 200ms `StartupWait` to prove the
  relaunch guard is a branch and not a stopwatch. That one rides a 45s
  `RelaunchGrace`: 150x margin, and slower only makes the point harder.
- **Live-only** — `bootrace_qa_test.go:201`'s 700ms is inside
  `TestQALiveGateOpensOnAScreenNotAShellPrompt`, which skips unless
  `RHQ_LIVE_SHELL_PANE` is set.

`time.AfterFunc` appears in no test in the package, and no test goroutine
plants or removes a fixture file at all any more.
