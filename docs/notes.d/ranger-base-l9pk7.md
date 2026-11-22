# A pin's stated reach is not its reach (ranger-base-l9pk7)

*2026-11-22 · gwart · cmd/coldexec's report assertion said "every supervised
run" and held 17 of 20; routing the other three, and the census that holds the
membership so a fourth cannot be written in silence.*

## What was wrong

`wantReportAccountsForEveryNotification` reads the one arithmetic a seat
reconciles by hand off the end-of-run report: the notification count against
the sum of the nine verdict counts. It is the report-side half of
ranger-base-9uvsx's fix, and its own doc comment ended "held on every
supervised run against every future way of spending a verdict twice or
spending none."

It was held on the runs `runDoor` makes. Three supervised runs that print a
reconcilable report were outside it, each for a reason that was good at the
time:

- **ARM 11**, `TestTheDelayVariableReachesTheSupervisorAndNothingUnderIt`,
  builds its own `exec.Command(builtDoor(t), …)` because it must set
  `POSSE_COLD_EXEC_DELAY` and `runDoor` scrubs that variable.
- **`runRoom`'s two arms where the door runs to the end**,
  `TestNoFloorAsksForNoRoomCheck` and
  `TestTheRoomIsReadWhereTheSuiteWillWrite`. `runRoom`'s comment justified
  bypassing `runDoor` with "a refusal prints no report at all — which is the
  thing being pinned", which is true of its **third** arm only.

Nothing pinned that a supervised run goes through `runDoor`, so the next one
written outside it was unasserted silently. That is how three accumulated.

The invariant itself was fine: ARM 22, `TestANotificationSpendsExactlyOneVerdict`,
reads it off `handle()` where it is deterministic. This was reach, not defect.

## The fix

The assertion is a free function over the output rather than a method on
`doorRun` — a supervised run assembled without that struct is still a run
whose report has to close — and the parse it needs is `reportArithmetic`.
ARM 11 calls it; `runRoom` calls it where the witness file says the command
ran, which is also what says the door got past the filter and printed a
report. The refusal arm is unchanged and still pins that there is no report.

`TestQAEverySupervisedRunHoldsTheReportArithmetic`
(`cmd/coldexec/reportmembership_linux_test.go`) is the structural half: it
parses this package's `*_test.go`, and a function that calls
`exec.Command(builtDoor(t), …)` must either call the assertion or sit in
`rmExempt` with a reason. One entry is in that table — ARM 10,
`TestNoCommandRefuses`, which runs the door with no argv, refuses before it
installs a filter and prints no report at all. A table entry whose function
has stopped starting a run, or has started holding the assertion, reds as
stale.

It reads the direct shape and says so. ARM 8's inner door is a string inside a
supervised script, and that run is a child of a run `runDoor` already holds.

## MEASURED 2026-11-22

This box, linux 7.2.15 x86_64, go1.27.3, 16 cores, at main 5a5ff36a. Each
mutant planted alone and the tree restored between arms
(`scripts`-less battery, driven from the session scratchpad).

**The real defect shape, and why the routing changes what the pins prove.**
A door that double-spends every notification — `d.record(pending, int(n.Pid),
pendingTarget)` written twice at the end of `answer()` — makes the arithmetic
wrong deterministically instead of a quarter of the time. Before this bead it
passed all three runs above (ranger-base-x64hk's finding 1 measured that; 20
tests red with 19 distinct messages from this assertion, and those three among
the green). After:

| run | verdict under the mutant |
|---|---|
| `TestTheDelayVariableReachesTheSupervisorAndNothingUnderIt` | red, 3 notifications against 6 verdicts (`supervisor_linux_test.go:694`) |
| `TestNoFloorAsksForNoRoomCheck` | red, 2 against 4 (`room_linux_test.go:108`) |
| `TestTheRoomIsReadWhereTheSuiteWillWrite` | red, 2 against 4 (`room_linux_test.go:131`) |

**The census, graded by mutation.** Five mutants, each run against
`-run '^TestQAEverySupervisedRunHoldsTheReportArithmetic$'`; all five red, none
survived.

| mutant | what it said |
|---|---|
| ARM 11 drops its assertion call | names ARM 11 and the site, `supervisor_linux_test.go:671` |
| `runRoom` drops its assertion call | names `runRoom` and `room_linux_test.go:58` |
| the exempt run starts holding it | "calls … and is also in rmExempt" — take it out of the table |
| `rmStarter` renamed out from under the census | "found no call to `exec.Command(builtDoorRenamed(…), …)` … so it is asserting nothing" |
| a stale exemption | the stale entry **and** the run it was shielding, both named |

The fourth is the one worth having: a census that reads one source shape goes
quiet when that shape is renamed, and every other red above would be
unreachable with nothing to say so.

`go test -count=1 ./cmd/coldexec` is green, 1.991s.
