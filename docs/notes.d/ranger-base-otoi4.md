# ranger-base-otoi4 — ARM 21's guard asserted something about the box

`TestTwoTasksExecingOnePlantConcurrentlyChargeOnlyOneOfThem` (cmd/coldexec,
ARM 21) ended with

```go
if strings.Contains(r.out, "truncated") || strings.Contains(r.out, "withdrawn") {
```

over the WHOLE supervised run's output, with the reason "this run signalled
nothing, so there was nothing to cut short or hand on". The premise is about
the fixture; the assertion is about every process under the supervisor,
which includes every one the test did not start. The two differ by exactly
the withdrawals a busy box produces on its own — and the busier the box, the
more of them, because the window is the door's own /proc reads against a
caller taking preemption signals (the note above `holes` in door_linux.go:
the rate tracks the run queue).

So it reds on load, in arm 1, which is `make test`'s first test line and
nobody's optional door: the cost of one such red is a failed `make test` for
every seat on the box.

Environment for everything below: this box, linux 7.2.15-1-omarchy x86_64,
uid 1001, go1.27.3, `/bin/sh` -> bash, D = 600ms (`pinDelay`).

## The red (MEASURED 2026-11-21, ranger-base-nt5xd's full-suite run)

Inside `make test` at a one-minute loadavg of roughly 5-9, both suite slots
held by sibling seats:

```
FAIL cmd/coldexec 1.973s, one test
coldexec: 14 execve notifications, 1 cold first execs delayed 600ms each
          (1 warm, 12 older than the door, 0 absent)
coldexec: 6 notifications were withdrawn before the door could answer
ELAPSED first 2ms / ELAPSED second 602ms   <- both charges correct
```

Six withdrawals of a `/bin/sh` older than the door, one task repeatedly,
none of them the fixture's. The pin's own subject — one of the two
concurrent execs pays D and the other does not — was right in that run.

## It is not load-only, and a reproduction that needs no load

The standing ruling here is no load testing on this box, so the shape was
reproduced deterministically instead: ARM 21's own script plus a second,
UNRELATED plant exec'd by a python3 caller with an SA_RESTART handler on
SIGURG, nudged the way `withdrawalFixture` nudges (a forked child polling
`/proc/<pid>/exe`). MEASURED 2026-11-21, loadavg 4.67, one run:

```
coldexec: delayed     pid 4044752 /proc/4044752/cwd/./plant     <- ours
coldexec: warm        pid 4044753 /proc/4044753/cwd/./plant     <- ours
coldexec: withdrawn   pid 4044746 /proc/4044746/cwd/./other     <- x60
coldexec: 72 execve notifications, 2 cold first execs delayed 600ms each
          (1 warm, 9 older than the door, 0 absent)
ELAPSED first 602ms / ELAPSED second 1ms
```

60 withdrawals, 0 of them of either task this pin is about. A box under load
is one way to get that output; a sibling process with an SA_RESTART handler
is another, and neither is the fixture's doing.

The first attempt at this nudged a caller exec'ing `/bin/true` — an inode
older than the door, which is the shape the red above had. It produced no
withdrawal at all in a quiet run: a self-settled verdict is answered in
microseconds, so the window a 10ms nudge has to hit is the door's own /proc
reads and nothing else. That IS why the red tracks the run queue, and it is
why the live reproduction here withdraws a charge (600ms of window) rather
than an `old` verdict.

## The fix, and what it is scoped by

The guard is now "no charge of OUR OWN TASKS was cut short or handed on",
read off the `-verbose` lines: collect the tasks whose adjudication lines
name the plant, require exactly 2 of them, and red only on a
`withdrawn`/`truncated` line naming one of those. Scoped by PID and not by
target, because a withdrawal of a verdict `adjudicate` settled itself names
no target at all (`recordWithdrawal`'s `pendingTarget == ""` case), and one
of those on our own task is still a withdrawal of our own exec.

`reVerbose` is the one pattern for those lines, in the harness block beside
`reDelayed`. `door.record` is the only thing under this door that prints
`coldexec: … pid <n> …`, so it cannot pick up a line of the end-of-run
report.

The `len(ours) != 2` fatal is new coverage rather than scaffolding: two
DIFFERENT tasks on one fresh inode is the premise of every count in this
pin, and `1 delayed, 1 warm` off a single task would be ARM 1's claim. No
other assertion here said so.

## Verified (MEASURED 2026-11-21, same box)

| check | result |
|---|---|
| the captured 60-withdrawal run above, replayed through both guards | old guard reds, new guard does not |
| the bead's own log shape (unnamed withdrawal, foreign pid) | old guard reds, new guard does not |
| our own task's charge handed to a restart (`withdrawn … /plant` on a plant task) | new guard reds |
| the mutant ARM 21 exists for — `charge{pid: 0, key: key}` at door_linux.go:597 | reds, on `wantDelayed` and on the `paid D` count, with `len(ours)` still 2 |
| `go test ./cmd/coldexec` | ok, `-count=1` and `-count=5` (9.187s) |

The replay was done in a throwaway test in the package, over the real
captured output; it is not committed, because a pin over text this bead
captured would assert the capture and not the door.

## Not closed here

- ranger-base-9uvsx — the same TRIGGER (a withdrawal landing on a
  notification that already had a verdict) at a different site: the report's
  own arithmetic, where the verdict counts sum past the notification count.
  This bead is a test whose premise about withdrawals was wrong; that one is
  the door's accounting, and it is filed apart because the sites and the
  costs are different. Nothing here changes door_linux.go.
