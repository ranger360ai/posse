# ranger-base-87p6i — the `-race` arm covered eight files, and the race landed in one of the fifteen it did not

`scripts/test-race.sh` is the on-demand `-race` arm over `internal/posse`'s
concurrency-carrying tests. Its candidate set was eight files. Its own comment
says what the set is for — "dispatch.go's gather fan-in, passcarry.go's
fan-out and wake channel, watch.go's pulse/backup/guard/watchdog goroutines" —
and on 2026-11-14 **fourteen other test files ran one of those watch.go loops
on a goroutine of their own, and the data race ranger-base-np54s fixed was in
one of the fourteen**: `suspend_qa_test.go`, where `go d.watchdogLoop(...)` sits
four lines above the poll that raced it.

So the instrument built for exactly that class was green over the file that
had the race. `make test` found it instead, once, by luck, under load — as a
panic that ended the whole arm-3 binary, so every other test in that run
reported nothing. That is `armtags_qa_test.go`'s rot #2 one level further out,
and the same shape as the defect `racearm_qa_test.go`'s arms 1-3 already hold:
an instrument that covers less than it says it does and exits 0.

2026-11-14.

## The census

MEASURED 2026-11-14 at `3943afc3`, by a `go/ast` walk over all 705
`internal/posse/*_test.go` files: a file is a launcher when it has a `go`
statement whose subtree calls `Watch`, `pulseLoop`, `backupLoop`, `guardLoop`
or `watchdogLoop`, or `Run` on one of the dispatcher's three variable
spellings, directly or through a test helper it calls by name.

| | files | tests |
| --- | --- | --- |
| launchers | 21 | — |
| of those, in `CANDIDATE_FILES` | 6 | — |
| of those, NOT in it | **15** | **119** |
| in `CANDIDATE_FILES`, launching nothing | 2 | — |
| the arm before | 8 | 108 |
| the arm after | 23 | 227 |

Fourteen of the fifteen launch one of the four loops; the fifteenth,
`launchlock_test.go`, launches the pass itself (`go func() { d.Run(…) }()`
with the test reading the dispatcher's output while it runs — np54s's shape
exactly, through a `syncBuf` that happens to be the right apparatus).

The two that launch nothing are `dispatch_qa_test.go` and
`dispatchparity_qa_test.go`, in the arm for dispatch.go's gather fan-in. They
are why the derivation below is a floor on the list and not the list.

The bead was filed saying the gap was two files (`suspend_qa_test.go` and
`backuploop_test.go`). Its census found files by the four loop NAMES and then
listed every launch site inside the files it had found — which is why a
`go func() { … d.Watch(…) }()` appears in it under `backuploop_test.go` while
twelve files whose only launch is `Watch` went unnamed. `Watch` starts all
four loops, so a test that runs IT on a goroutine is running them too:
`pulse_test.go` and
`pulse_verify_test.go` (whose subject IS `pulseLoop`), `guardclock_qa_test.go`
(`guardLoop`), `backupjoin_qa_test.go` (whether a backup tick in flight is
JOINED), `drain_qa_test.go` (cancelling a gather mid-pass), and seven that use
a background `Watch` as a rig for something else.

## What landed

| file | what |
| --- | --- |
| `scripts/test-race.sh` | the fifteen files added to `CANDIDATE_FILES`; the comment now says what the list's mechanical floor is and where it is held |
| `internal/treepins/racearm_qa_test.go` | arm 4, the file list against that floor; arm 5, the control over the derivation; `raceArmExcluded`, the priced escape hatch, empty |
| `Makefile` | the `test-race` comment's price (~17 min over 227 tests) and its "not gate-clean today" bead id |
| `docs/notes.d/ranger-base-d0xvw.md` | a pointer section: its eight-file list is the 2026-09-07 record, not the live set |

### Why the list is still a person's, with a derived floor under it

The script's comment is right that "concurrency-carrying" is a judgement about
the PRODUCT code, and a derived list would read the test instead — `-l
architecture` territory, and the bead said so. What the measurement shows is
that the judgement has a mechanical half that was never anybody's judgement:
a file that runs one of those loops on a goroutine is carrying that
concurrency whatever it is called and whatever it asserts. A race does not ask
what the file's subject is. `suspend_qa_test.go`'s subject is a suspend
witness, and the race was in its poll.

So arm 4 derives that half and holds the list against it, and the list stays a
list. Nothing is derived INTO the arm: a file the derivation proposes is in
`CANDIDATE_FILES` or carries a row in `raceArmExcluded`, both typed by hand.
This is ADR 0098 D1's shape — in the class unless excluded, so a file written
tomorrow is covered the day it lands rather than after an escape — with one
difference worth knowing before writing a row: **an ADR 0098 D2 treepins
exclusion moves a test to another route that still runs it, and this arm is
the only route in the tree that runs anything under `-race`. A row here is a
coverage hole, not a schedule.** That is why the table is empty and why "its
subject is not concurrency" is not a reason it accepts.

### Why all fifteen went in rather than two

Because the arm's cost is wall on an instrument that no gate runs, takes no
suite slot, is a prerequisite of nothing, and is typed by hand when a change
touches dispatch/passcarry/watch or a race is suspected — and because the
alternative was thirteen coverage holes chosen by nobody. The price is below.

## The price, MEASURED 2026-11-14

This box (D1 host, Linux 7.2.15-1-omarchy, go1.27.3, 16 cores, warm build
cache), `scripts/test-race.sh` with `POSSE_RACE_FILES` set to each set in
turn, run back to back in that order, one-minute load average noted per arm
because another seat's suite was on the box for part of it.

| set | files | tests | arm 1 | arm 2 | arm 3 | total | load (1m) |
| --- | --- | --- | --- | --- | --- | --- | --- |
| the arm as it stood | 8 | 108 | `ok` 315s | FAIL 319s | `ok` 53s | 707s | 4.9 → 7.4 |
| the two files the bead named | 2 | 23 | — | — | `ok` 22s | 27s | 6.0 → 4.5 |
| the fourteen it did not | 14 | 107 | `ok` 120s | FAIL 74s | FAIL 106s | 301s | 4.5 → 19.9 |
| **the arm as it stands now** | **23** | **227** | `ok` 317s | FAIL 348s | FAIL 345s | **1011s** | 13.5 → 9.2 |

`DATA RACE` 0 in every arm of every run. **So the arm costs 16.9 minutes now
against 11.8 for the eight files — about 40% more wall for 2.1x the tests**,
because what the old set spent was concentrated in a few long-backstop tests
rather than spread over its 108. The sets are additive on this box to within
3s (707 + 301 against the combined 1011), which is what the arm being
three `-run` filters over three binaries predicts.

That is the whole cost decision the bead left open, and it is paid: an
on-demand instrument that no gate runs, that takes no suite slot, that is a
prerequisite of nothing, and that a person types when they are already waiting
for its answer, is worth five more minutes to stop being green over fifteen
files.

One thing to know before pricing a subset with `POSSE_RACE_FILES` yourself:
the two-file run above reports `RAN NOTHING` for arms 1 and 2 and exits 1.
Both files are `posse_arm3`, so those two arms genuinely had no test to run,
and the script's rot-#4 check fires exactly as designed. On the full set it
cannot happen, and that is the case the check is for.

## Mutation-checked

One mutant at a time, each reverted before the next, and the two files back to
the pre-battery bytes at the end (`cmp`, both identical) with arms 4 and 5
green again.

| mutant | reds |
| --- | --- |
| `suspend_qa_test.go` out of `CANDIDATE_FILES` — the tree exactly as it was when the race landed | arm 4, naming the file and `watchdogLoop` at `suspend_qa_test.go:592` |
| `watchhang_qa_test.go` out — the transitive case, reached only through `runBackupLoop` | arm 4, naming the helper and the site inside it |
| a `raceArmExcluded` row for a file the derivation does not propose | arm 4 |
| a `raceArmExcluded` row for a file `CANDIDATE_FILES` also runs | arm 4 |
| the launcher names renamed, so the derivation finds nothing | arm 5 (every shape), which is why arm 5 exists |
| `CANDIDATE_FILES` renamed in the script, so the reader finds no list | arm 4, on the reader rather than the list |

The last two are the ones worth having: a derivation that finds nothing and a
reader that reads nothing both say "every file is covered", and arm 4 alone
cannot tell either from a tree that is genuinely clean.

## Known limits

- **The transitive half follows a helper called by name.** All five of today's
  are plain funcs (`runBackupLoop`, `drainFixture`, `guardClockFixture`,
  `runLagWatch`, `carryFixture`). A helper with a RECEIVER is called through a
  selector and arm 4 would not follow it. `raceArmLaunchFloor` (15 against
  today's 21) is what catches a derivation that has gone blind, which is the
  failure that reads as "every file is covered".
- **A candidate file constrained by `//go:build !race` would be censused and
  not run.** The census builds without `-race` on purpose — the `-race` build
  is the same partition and ten minutes dearer — so a `race`-sensitive
  constraint is the one thing it cannot see. No candidate file carries one
  today (the only pair in the package is `fakecallcost_test.go` /
  `fakecallcost_race_test.go`, neither a candidate), and the script's
  `no tests to run` check catches only the total wipeout. Noted, not filed:
  there is no live instance.

## What the bigger arm found, on its first run at this size

- **ranger-base-1hnqc** — four tests that fail under `-race`, pass without it,
  and report `DATA RACE` 0: three in `launchlock_test.go` (34.17s, 10.00s,
  64.07s; `ok` 1.379s together without `-race`) and
  `TestDispatchRewaitServerGoneKeepsTheClaim` (43.71s; `ok` 3.791s without).
  Each is a budget spelled in wall seconds — a `time.After` constant, or a 2s
  `WaitCeiling` that is the test's own observable — against an apparatus
  `-race` stretches. That is ranger-base-0dt50's class, whose close predicted
  this ("the class is general and `fakeCallCost` is now there for the next
  one"), and three of the four are in a file this arm could not see until
  today. A fifth, `TestLaunchBeadLocksBeforeItsGuardsRead`, failed twice at a
  load of 19.9 and passes alone with 0.25s of its 30s deadline to spare — same
  class, nearer the edge, named on that bead and not separately.
- **`ranger-base-0dt50` was closed and two comments still said it was open.**
  `scripts/test-race.sh`'s "THE ARM IS NOT GREEN TODAY … filed as
  ranger-base-0dt50" and the Makefile's "the arm is not gate-clean today
  (ranger-base-0dt50)" were both false from 2026-09-10. That bead's own close
  recorded them as false and left them for whoever owns the target; both now
  say what today's run measured.

## Not closed here

- ranger-base-1hnqc — the four reds above. The arm being red is the arm
  working; the fix is somebody's judgement about four budgets, not a sed.
- Running the three arms concurrently. d0xvw left it unfiled at 13-14% CPU
  each, on the grounds that nobody had asked for the six minutes back. At
  1011s the sum is seventeen minutes and the longest arm is 348s, so the
  saving is now about eleven — still unmeasured under contention, still
  nobody's ask, and still not filed. Noted here because the number moved.
