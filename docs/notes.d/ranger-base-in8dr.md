# Two more arms for the cold-first-exec door, and the three things they found (ranger-base-in8dr)

2026-11-11, Linux 7.2.15 x86_64, go1.27.3, 16 cores, from this bead's
worktree. ADR 0085 D6's last slice: `make coldexec-check` reaches cmd/posse
and internal/treepins. From ranger-base-gdzu5 (the design), after
ranger-base-97w45 (the supervisor) and ranger-base-kiwvn (the first full run,
whose triage is what bought the supervisor).

**THE BEAD'S TITLE IS NO LONGER THE WORK**, and that is the amendment paying
off. It says "extend the injector to cmd/posse and internal/treepins' own
WriteExecutable", which was true of the content-rewrite door: each package
would have needed the hook and a shim arm in its own `TestMain`. ADR 0085
D2/D3 as amended (ranger-base-dacil) moved the cost into the `execve`, so
there is no injector to extend and nothing in either package to change —
internal/treepins keeps its own copy of the writer (`execwrite_test.go`,
because internal/posse's `init.go` imports the root package and Go refuses the
cycle back), cmd/posse calls `posse.WriteExecutable`, and both are untouched
by this bead. The deliverable is two Makefile lines, which is what richard's
NOTE on the bead said it would be.

## The arms are a census, not a choice

`make execwrite-check`'s tree-wide pin —
`TestQATreeGoFilesWriteExecutablesUnderTheForkLock`, which walks every `.go`
file in the repository and reds on an exec-bit `os.WriteFile` anywhere in it —
establishes that every exec-bit write in this tree goes through a
`WriteExecutable`. So counting those call sites counts the plants. MEASURED
2026-11-11:

| package | `WriteExecutable(` call sites | files holding them |
|---|---|---|
| internal/posse | 274 | 92 |
| internal/treepins | 54 | 24 |
| cmd/posse | 17 | 12 |

345 sites, three packages, and **no other package in the tree has one** — the
root package is a single `embed.go`, and nothing else under `cmd/` or
`internal/` plants anything. With these two arms the door covers all three.

What it does NOT do is derive that list. The five arms are five written
Makefile lines; a fourth package that starts planting tomorrow is covered by
nothing until somebody adds its line, and the pin above cannot notice,
because its per-directory floors are a FILE count and that is a different
claim. **ranger-base-s64a5** is the bead for deriving the arms from the
census.

## What the two arms cost, and that both are green

Each row is one arm's command line out of the `coldexec-check` recipe, run by
hand — `make coldexec-check` runs all five and internal/posse's three are
ranger-base-xpxo6's to price. Walls are `scripts/test-times.sh`'s package
time; the counts are the supervisor's own report, which it prints from the
PARENT of `go test` and therefore prints red or green. Warm build cache,
loadavg 5-15, one sibling seat holding the other suite slot throughout.

| arm | doored | undoored | ratio | notifications | delayed | warm | old | absent | holes |
|---|---|---|---|---|---|---|---|---|---|
| `./cmd/posse/...` | **ok 674.8s** | ok 103.5s | 6.5x | 8427 | 231 | 480 | 7433 | 283 | **0** |
| `./internal/treepins/...` | **ok 1581.8s** | ok 131.8s | 12.0x | 29744 | 608 | 914 | 21224 | 6864 | 145 |

**0 reds in the class, in either package.** Every test that plants an
executable and execs it under a deadline passed with that exec held 2.5s, and
neither arm needed a `--warm` pre-flight anywhere — the first pair added to
this door that didn't. The answer the bead asked for is "no reds".

The cmd/posse arm was run twice, before and over the finished tree, and the
column the door is about reproduced exactly: 231 cold execs delayed both
times, 8417 notifications then 8427. Every one of those ten is in the `old`
column (7423 then 7433) — execs of the checkout and the toolchain, which vary
with what `git` and `go` happen to do. The `delayed`, `warm` and `absent`
counts are identical across the two runs.

**THE COST IS ADDITIVE AND BARELY OVERLAPPED**, which is the reading worth
keeping. Each arm's doored wall is its undoored wall plus D once per cold
exec: +571.3s against 231 x 2.5s = 577.5s for cmd/posse (within 3%), and
+1450.0s against 608 x 2.5s = 1520.0s for internal/treepins (within 5%). So
an arm's doored wall can be predicted from the cold-exec count its report
prints, before the box is spent on it. Note also how little of either arm is
the class: 231 and 608 cold execs out of 8427 and 29744 notifications, the
rest being the toolchain and the checkout, which are older than the door.

The undoored walls are both well under this package's documented numbers
(AGENTS.md prices internal/treepins at a 393.5s median over 327 readings).
Those cover cold build caches and a loaded box; 131.8s is what it costs with
every `go build` and `go vet` its pins fork already cached, on a quiet box.
The ratio is the honest part of the table — both halves were taken minutes
apart under the same conditions.

## `-timeout 25m` was the door's cost arriving as a hang

The treepins arm's FIRST doored run died: `panic: test timed out after
25m0s`, `FAIL ... 1502.514s`, with `TestQASuiteLockSelfTestIgnoresAnAmbient
PosseEnvironment` 1s in and a dozen goroutines parked in `t.Parallel` for 24
minutes. 25m is what `make test` gives the same package, and it is not enough
under the door: `go test -timeout` bounds the TEST BINARY's own wall, and 608
cold execs at 2.5s is 1520s of that wall before a single test does anything.

That red is in neither of the two families a reader would reach for. It is
not ADR 0085's class — no test's verdict changed, the binary ran out of
clock. And it is not the splitting advice `scripts/test-times.sh` prints over
a slow package ("a package this big tests as one binary and one clock; it
wants splitting, not a bigger -timeout"), which is sound and does not apply:
the package did not grow, the door charged it. So every arm carries `-timeout
60m` now. At 60m the same arm is `ok 1581.8s`, 43% of budget. What it costs is
that a REAL hang under the door takes an hour to surface, which is the price
of not reading the door's own cost as a hang.

**And the first spelling of that fix was wrong in a way the tree caught.** It
was `-timeout $(COLDEXEC_TIMEOUT)`, one variable for five arms, which is how
`COLDEXEC_DELAY` is done one line above. `internal/treepins`'
`TestQANoEntryPointRunsGoTestOnTheDefaultTimeout` red it mid-flight —
`Makefile:1181: -timeout does not parse: time: invalid duration
"$(COLDEXEC_TIMEOUT)"`, five times, once per arm. That pin scans the Makefile,
scripts and workflows for any `go test` entry point and requires a parseable
`-timeout` above a 15m floor, and it is right to refuse a make variable: it
cannot tell 60m from 6m. So the value is a literal on all five lines, and the
pin that would have been routed around is the reason.

## The one coverage hole: an orphaned process is out of reach

internal/treepins is the first arm to report holes — 145 of 29744
notifications un-adjudicated, 134 `unreadable` and 11 `gone`, 0.49% of the arm
— where cmd/posse reports 0 of 8427 and ranger-base-97w45's filtered
internal/posse arms reported 0 of 1602. Every unreadable one is the same
error, `open /proc/<pid>/mem: permission denied`, and in the arm they cluster
in the suite-lock self-tests, which orphan holder processes on purpose.

The cause is parentage, not the plant, and it reproduces in isolation. Under
`./bin/coldexec -verbose -delay 500ms`, with two identical fresh `#!/bin/sh`
plants in one `mktemp -d`:

| who execs the plant | verdict |
|---|---|
| a child still in the supervisor's tree | `delayed` |
| a child whose parent exited, so it was reparented to init | `unreadable`, and the plant is **never delayed** |

Reading the path argument needs `PTRACE_MODE_ATTACH` on the caller, which the
supervisor has by being its ancestor; `yama/ptrace_scope=1` on this host
allows an ancestor and nothing else. An orphan has left that reach.
`cmd/coldexec/doc.go`'s `readPath` comment states this edge already — what is
new is that it is measured, located, and has a consequence inside the class:
a test whose planted executable is exec'd by an orphan is not covered by the
door, and nothing in that test's own output says so. It is counted and named
rather than silently called `old`, which is exactly what D5's verdict taxonomy
was built for (ranger-base-97w45, decision 3).

Filed as **ranger-base-orgvt**, with the repro and an unmeasured candidate
fix: `prctl(PR_SET_CHILD_SUBREAPER, 1)` on the supervisor before it forks, so
an orphaned descendant reparents to IT instead of to init and stays in reach.
ADR 0085 D5's stated-edge list now names this one with these numbers.

## What was run, and what is left

- `make coldexec-check`'s two new arms, doored and undoored, twice for
  cmd/posse and twice for internal/treepins (the first treepins run is the
  25m red above). Logs are this session's scratchpad.
- The verbose hole probe:
  `./bin/coldexec -verbose -- go test ./internal/treepins -run
  '^TestQA(SuiteLock|EverySuiteArmTypeChecks|SelfTestForkArm)' -v`, 28.7s,
  1908 notifications, 7 delayed, 5 unreadable — all five the same EACCES.
- `make fmt-check`, `make tree-check` (rc=0, 38.5s, and again over the
  finished tree) and `make test` (rc=0, 643s, three arms, with cmd/posse at
  140.7s and internal/treepins at 151.3s inside it). `go run
  ./cmd/checkorphans` clean.
- NOT run: the unfiltered five-arm `make coldexec-check`. internal/posse's
  three arms are ranger-base-xpxo6's measurement, and the additive rule above
  is what that bead can price them with in advance.
