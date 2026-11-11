# The first full run under the execve supervisor: the price, and the red that belonged to no family (ranger-base-xpxo6)

2026-11-11, Linux 7.2.15 x86_64, go1.27.3, 16 cores, from this bead's
worktree. ADR 0085 D6's price, for internal/posse's three arms — the three
`make coldexec-check` lines ranger-base-in8dr left to this bead. After
ranger-base-97w45 (the supervisor), ranger-base-kiwvn (the same run under the
content rewrite, whose triage bought the supervisor) and ranger-base-in8dr
(arms 4 and 5). Design: ranger-base-dacil.

**THE ANSWER IS THAT NOTHING IN THIS PACKAGE IS IN THE CLASS.** Five red rows
over two doored arm-3 runs, and not one of them is a test whose verdict
changes when the first exec of its own planted file is slow. One is a pin
whose real-time margin the door's slower box ate (fixed here, below); four
are a full `/tmp` (ranger-base-vw1cp). Arms 1 and 2 were green doored, first
time, and all three arms were green undoored. The 26 reds kiwvn triaged were
every one an artifact of rewriting a plant's BYTES, and the supervisor does
not touch bytes — so the coverage D2/D3 as amended bought, including the 49%
of plants the rewrite could never reach at all, cost zero new reds.

**MEASURED AT ef842dbe, WITH THE SUPERVISOR AS IT WAS THEN.** Said plainly
because one of these numbers has already been overtaken: the hole counts below
are the pre-ranger-base-orgvt supervisor's. orgvt landed
`PR_SET_CHILD_SUBREAPER` while this bead was running, which closes exactly the
edge those four `unreadable` execs are, so a run of today's door should report
no holes at all where this one reported 0.0013%. Nothing else here moves with
it — the walls, the ratios and the delayed counts are the cost of holding a
first execve, which the subreaper does not change — but the hole column is
history and is left as it was measured rather than quietly restated. The ADR's
own edge bullet is orgvt's now and this bead did not touch it.

## The price

Package seconds as `go test` reports them, which leaves the suite-slot wait
outside. One-minute loadavg before → after each arm. Warm build cache, `/tmp`
on its tmpfs as every earlier reading of this command had it. Undoored first,
then doored, back to back through the same `scripts/test-times.sh` wrapper and
the same suite slots; the whole sequence ran in 37 minutes with the box at a
loadavg of 1.7-10.7 throughout, so the two halves are comparable.

| arm | undoored | loadavg | doored, D=2.5s | loadavg | ratio |
|---|---|---|---|---|---|
| 1 (`./internal/posse/...`) | ok 106.615s | 3.38 → 7.16 | **ok 948.918s** | 10.71 → 8.54 | 8.90x |
| 2 (`-tags posse_arm2`) | ok 73.473s | 7.16 → 9.81 | **ok 476.279s** | 8.54 → 4.31 | 6.48x |
| 3 (`-tags posse_arm3`) | ok 95.615s | 9.81 → 10.71 | FAIL 538.873s | 4.31 → 1.69 | 5.64x |
| **all three** | **275.703s** | | **1964.070s** | | **7.12x** |

**+1688.4s over the three arms, which is what D6 hands the operator.** The
ASSUMED line it replaces said "at least" the content rewrite's +571.1s
(280.3s → 851.4s, 3.04x, ranger-base-kiwvn). It is at least that and then
some: **2.96x the added cost, 7.12x against 3.04x**, because the supervisor
delays everything the rewrite did plus every hook, gate shim, production
render and `#!`-less binary. The undoored baseline reproduced kiwvn's to
within 1.6% (275.703s against 280.289s), which is what makes the two
injected numbers comparable at all.

With in8dr's two arms at 674.8s and 1581.8s, **the unfiltered five-arm
`make coldexec-check` is 4220.7s of package time — seventy minutes of box**,
before five suite-slot waits. Nobody should run it out of curiosity.

## THE COST IS NOT ADDITIVE HERE, and in8dr's rule does not travel

in8dr found each arm's doored wall to be its undoored wall plus D once per
cold exec, within 3% on cmd/posse and 5% on internal/treepins, and said an
arm could therefore be priced from its cold-exec count. **That does not hold
for internal/posse, and the gap is not small:**

| arm | cold execs delayed | D x execs (nominal) | actual added wall | realised |
|---|---|---|---|---|
| 1 | 700 | 1750.0s | +842.3s | 48.1% |
| 2 | 323 | 807.5s | +402.8s | 49.9% |
| 3 | 599 | 1497.5s | +443.3s | 29.6% |
| **all three** | **1622** | **4055.0s** | **+1688.4s** | **41.6%** |

The sleeps overlap, because this package's plants are exec'd from tests that
run in parallel — kiwvn measured the same thing under the rewrite (443 execs,
1107s of nominal sleep, a far smaller wall) and wrote "nobody should price
this door as D x plants". Both readings are right about their own package.
**What decides it is how much of the arm is parallel**, so the honest rule is
the one neither bead can state alone: D x cold execs is an upper bound
everywhere, it is tight where the plants are exec'd serially (cmd/posse,
internal/treepins), and it is roughly 2x the truth where they are not
(internal/posse). Price a new arm from the bound and be pleased.

## What the door reached, by its own counters

| | arm 1 | arm 2 | arm 3 | total |
|---|---|---|---|---|
| execve notifications | 122697 | 113511 | 83362 | 319570 |
| cold first execs delayed | 700 | 323 | 599 | **1622** |
| warm | 24961 | 19417 | 11823 | 56201 |
| older than the door | 95533 | 87106 | 69218 | 251857 |
| absent | 1500 | 6664 | 1722 | 9886 |
| withdrawn before the answer | 2 | 1 | 0 | 3 |
| **holes (unreadable)** | **3** | **1** | **0** | **4** |

**Four un-adjudicated execs in 319570 notifications, 0.0013%** — against
in8dr's 145 of 29744 (0.49%) on internal/treepins, whose suite-lock
self-tests orphan holders on purpose, and 0 of 8427 on cmd/posse. All four
are the `unreadable` kind ADR 0085 D5 names and ranger-base-orgvt holds the
candidate fix for; this package has almost nothing that orphans. The three
withdrawn notifications are ranger-base-j0riv's reading and not holes.

Note how little of this is the class: 1622 cold execs out of 319570
notifications, the rest being the toolchain and the checkout, which are older
than the door.

**The counters are stable.** Arm 3 was run twice doored: 538.873s then
550.744s (2.2% apart), 83362 then 83004 notifications, 599 then 596 delayed,
1722 absent both times. The `absent` count being identical to the unit across
two runs is the same thing in8dr saw — what moves between runs is the `old`
column, which is whatever `git` and `go` happen to do.

## The one red of ours, and why it is a third thing

`TestQAWatchStartsTheWatchdogAndItNamesAStalledPass`
(internal/posse/watchhang_qa_test.go), doored arm 3, run 1:

```
--- FAIL: TestQAWatchStartsTheWatchdogAndItNamesAStalledPass (7.53s)
    watchhang_qa_test.go:711: a pass stalled in a hung bd child past the
    budget was never named — Watch did not start the watchdog:
```

The bead's instruction was a fork: a red is either IN CLASS (fix it with a
`--warm` pre-flight) or a supervisor bug (fix the supervisor, never a
pass-through list). This is neither, and saying so is the point.

**It is not in the class.** Its planted executable is `hangingBin`'s, which
already pays its own first exec in a `--warm` run before the timed window —
the pre-flight the class's fix would add is there. MEASURED: focused under the
door, 10 of 10 PASS at 4.53s, which is the undoored 2.03s plus exactly one
2.5s hold, and the supervisor's `-verbose` names the two execs it held:
`posse.test` and the `hang` plant's warm run. No planted exec happens inside
this test's deadline at all.

**It is not a supervisor bug.** The door held the first execve of every inode
created after it started, which is all it claims to do.

**What it is: a pin whose margin was four ticks wide.** Measured by a
throwaway in-package probe (`zz_xpxo6_margin_test.go`, the pin's body with the
ctx as the only variable, deleted before the commit):

| ctx | silence line | verdict |
|---|---|---|
| 50ms, 100ms, 150ms, 200ms | absent | **FAIL, deterministically** |
| 300ms, 400ms, 500ms, 1500ms | present | PASS |

and the line it prints is the tell: `written nothing for 33m — past its 32m
budget (last line 04:53:19)`. `04:53:19` is one fake second past `t0`, which
is **`Watch`'s own `noteWrite` seed** — so the silence this pin reads was
never the bd stall it is written about. The loop's header block writes bare,
so nothing touches `LastWrite` until pass 1's header, and the window in which
the watchdog goroutine must get a 20ms tick is

> [ the budget, 2 x (PromptWaitMS 15m + HerdrWaitGrace 1m) = 32m fake = **192ms real** ,
> pass 1's header, whose arrival closes it ]

On a quiet box the header lands at ~359ms (fake stamp `05:52:21`-`05:53:18`
across eight probe runs), so the window is ~167ms, eight ticks. **In the
failing doored arm the header landed at fake `05:30:36` = 223.8ms, and the
window was 31.8ms — one or two ticks — which the loaded box missed.** The
failure direction is the surprising one: the pin is fragile when the startup
block is FAST, and it would fail deterministically if that block ever came in
under 192ms.

Fixed by moving the ctx from 500ms to 2s, so the quiet the dog reads is the 2s
stall the test was always written about. Both bounds are now stated in the
test: above the header plus the budget (2s against a header measured at
223-359ms leaves ~1.45s, seventy ticks, where 500ms left one), and below the
stall's own end at header+2s, so the loop does not come round for a second
pass and pay another stall. It costs nothing — undoored 2.03s before and
after, doored 4.53-4.60s over 5 runs, 5 of 5 PASS.

**And green in the condition that broke it**, which is the row that matters:
the whole doored arm 3, with the fix, on the disk TMPDIR described below —
`FAIL 534.141s` whose ONLY two reds are the two that the undoored control on
that same TMPDIR also produced, with 598 cold first execs held and the
watchdog pin's name appearing nowhere in the log. Before the fix that arm red
it; after, under 598 held execs and a 534s wall, it does not. The substrate
cost nothing either: 534.141s against 538.873s and 550.744s on tmpfs doored,
and 96.076s against 95.615s undoored, so moving TMPDIR changed those two
seatbelt pins' verdicts and not the price.

**Mutation-checked**, because a guard never seen red is not a guard. With
`Watch`'s `watchdogLoop` call replaced by `<-dogCtx.Done()` — the loop
started, the watchdog not — the fixed pin fails with its own fatal, the one
quoted above. `internal/posse/watch.go` was restored and the tree re-read
clean afterwards; the mutation went in a file this bead does not otherwise
touch, so restoring it could not revert the fix.

## The other four reds were a full /tmp, and `df -h` said it was fine

Doored arm 3's second run redded four different tests and the watchdog pin
passed. All four name the same cause:

| test | what it said |
|---|---|
| `TestClaudeProjectConfigTrustIsKeyedAndFailsClosed` | `parity_test.go:655: write .../state/gates/dev/bin/git: disk quota exceeded` |
| `TestQueueRollbackVerificationFiresOnARollbackThatWorked` | git `failed to write new configuration file .../config.lock`, exit 128 |
| `TestQueueCutoverAbortInTheMoveWindowNamesTheHalfStateAndItsUndo` | git `copy-fd: write returned: Disk quota exceeded` |
| `TestQueueRollbackIsWrittenForAStateStepEightRemoves` | the same |

**What runs out is INODES.** `/tmp` here is a tmpfs mounted
`nr_inodes=1048576`. MEASURED at the moment of failure: `df -i` said 1041640
of 1048576 used — **100%, 6936 free** — while `df -h` said 80% with 2.8G
available. Every write then fails as `EDQUOT`, which reads as a quota problem
and is an inode problem. Afterwards even `go test -c ./internal/posse` could
not link: `compile: writing output: write $WORK/b132/_pkg_.a: disk quota
exceeded`.

The inodes were eaten by **425 abandoned `/tmp/posse-testhome-*` trees holding
5.2G**, the oldest from 2026-11-10T20:34Z — fourteen hours of accumulation
across every seat, not this bead's. internal/posse's `TestMain` makes that
temp HOME and removes it after `m.Run()` with the error discarded, so a run
that never reaches that line leaves it; the bulk of each tree is a mise
toolchain install the fake HOME triggers (`.cache/mise` 1.3G and
`.local/share` 1.0G in the largest). No read-only directory inside any of
them, so `RemoveAll` was not blocked — those runs simply never got there.

`scripts/test-times.sh` has a disk preflight for exactly this and read none
of it: with the inode table full and 2.8G of bytes free it printed its
"Running anyway" line and the arm ran into the wall.

**Half of that is already somebody's, and the better half of the reading is
theirs too** — ranger-base-akk9f landed on main while this bead was running
(cf6506e4), from ranger-base-r5tnw, over the same errno from the other
direction. It is worth reading before anyone acts on this: the mount carries
`usrquota`, that quota is per UID, and **every seat on this box runs as
`posse` — so it is one box-wide budget that all concurrent suites spend
together**, which is why a sibling seat can red your arm with writes you never
made. akk9f fixed the explainer (`explain_disk` grepped only `no space left on
device`, so an EDQUOT red named no cause at all, in either of the two
spellings this box produces) and made the DISK line say that a free-space
number cannot see a quota.

What akk9f deliberately did NOT do is probe the budget — "there is no `quota`
binary on this box". **The inode half of that budget needs no such binary:
`df -iP` is right there, and it read 100% at the moment of failure while the
byte column read fine.** MEASURED after akk9f landed: `df -kP` is still the
only `df` the script runs, and nothing in it reads an inode count. So what is
left of this finding is one reading the preflight could take and does not, and
the 425 leaked trees that spend the budget in the first place — which akk9f's
fragment does not name. Filed as **ranger-base-vw1cp** (P1, `-l code`),
narrowed by a comment to those two halves so nobody re-does akk9f's.

**Nothing was deleted from `/tmp`**, and that is the script's own ruling, not
a preference: "What to clear on a shared box is the operator's call, not this
script's: ... deleting from $TMPDIR can take a live test's TempDir out from
under it." No session can tell a leaked `posse-testhome-*` from a live one —
nothing inside names the worktree that made it, which was checked. **So /tmp
needs the operator before any seat can run a full arm again**, and the
remaining runs in this bead moved to `TMPDIR=/home/posse/tmp-xpxo6` on the
930G disk instead.

## Two things verification turned up that are nobody's door

**A relocated TMPDIR is not a substitute substrate for arm 3.** With `/tmp`
unusable, the fix's "green in the condition that broke it" run moved to
`TMPDIR=/home/posse/tmp-xpxo6` on the 930G disk — and the UNDOORED control on
that TMPDIR red two tests of its own:
`TestQAGrantStillReachesTheGitConfigInEveryShape/deniesFiles` and
`TestSeatbeltWritableReachesGoTelemetryCountersAndNotTheModeSwitch`, 96.076s,
neither of them red on tmpfs minutes earlier. Both reason about writable roots
and the seat's granted paths, and `$TMPDIR` is an input to that — the hazard
AGENTS.md and this shop's standing notes already name from the other side
("seatbelt grants `$TMPDIR` unconditionally, so a `t.TempDir()` fixture is
vacuous"). Running the control FIRST is what makes the doored run on the same
TMPDIR readable at all: the pair is read for whether the watchdog pin is among
the reds, not for a wall or a total. Nothing here is a finding about the door,
and the two reds are not filed — they are an artifact of a substrate this bead
invented for one run.

**`make tree-check` is red on main, and it is not this bead's.** rc=2 in 7.6s
at ef842dbe, `TestQAColdexecArmRegisterIsNotStale`: internal/treepins'
register still waives cmd/posse and internal/treepins from needing a
coldexec-check arm, and in8dr landed both arms. The register's comment
anticipated the red and addressed it to in8dr's seat — but ranger-base-s64a5
landed the register AFTER in8dr, so the rows were false on arrival and the
seat the message was for had gone. It reds `corpus-check`, so `tree-check`,
so `make test`, for every seat on the box. Attributed by `git diff
--name-only HEAD`, which names only `docs/` and
`internal/posse/watchhang_qa_test.go` — neither the Makefile nor that pin's
file. Filed as **ranger-base-byc2i** (P1, `-l code`) with the two-row deletion
the pin itself prints and the derivation run that has to pass after it.

Every other door is green over this change: `fmt-check`, `crew-check`,
`seed-check`, `history-check`, `doc-check`, `identity-check`, `ops-check`,
`execwrite-check`, the three `pkgcensus` arms, `notes-check` and `adr-check`
inside the `tree-check` run before it stopped, and `register-check`,
`scripts-check` and `pid-check` run individually afterwards because
`corpus-check`'s red had stopped make before them.

## What was run

- The three arms undoored and doored, back to back, through the
  `coldexec-check` recipe's own command lines rather than `make
  coldexec-check` — for the reason kiwvn gives, that make stops at the first
  failing arm and a red arm 1 would have hidden arms 2 and 3 for the whole
  bead. Walls and counters above.
- Arm 3 doored twice more. The second is the `/tmp` run above. The third was
  stopped mid-flight, not reported: by then the inode table was full and its
  numbers would have been the box's, not the door's.
- The focused probes: the pin under the door `-count=10` (10 PASS), the
  verbose single run naming the two held execs, the ctx table above, the
  no-watchdog mutant, and the fixed pin doored `-count=5` (5 PASS) and
  undoored `-count=3` (3 PASS).
- Arm 3 undoored and doored on `TMPDIR=/home/posse/tmp-xpxo6`, as the pair
  above: 96.076s and 534.141s, two reds each and the same two. That scratch
  dir was removed afterwards.
- `go run ./cmd/checkorphans` clean after every background run.
- NOT run: the unfiltered five-arm `make coldexec-check`. It cannot pass on
  this box until `/tmp` is cleared, and at 4220.7s of package time it is not
  something to retry against a full inode table. Every red it would still
  carry is accounted for: one fixed here, four on ranger-base-vw1cp.
