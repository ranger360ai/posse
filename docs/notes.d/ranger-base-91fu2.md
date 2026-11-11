# A restarted execve pays the rest of its charge (ranger-base-91fu2)

*2026-11-11 · gwart · the half of the withdrawal ranger-base-4kars left
open: the kernel fact the fix rests on, the mechanism, what `truncated`
means now, and what it cost.*

## What was left

`cmd/coldexec` holds the first `execve` of every inode created after it
started, for D (ADR 0085 D2). ranger-base-j0riv established that a
notification the kernel takes back is not a coverage hole — seccomp's wait
is interruptible, and a caller whose handler carries `SA_RESTART` restarts
the `execve` and raises a fresh notification the door adjudicates normally.
ranger-base-4kars established that the DELAY in that case was not paid:
`adjudicate` marked `d.seen[key] = true` before it slept, so the restart
read the inode as warm and was answered at once, and the plant's first exec
completed in milliseconds against the D the report charged it. That bead
made the door say so — `truncated`, out of the delayed count — and measured
the rate: **12 of 2,453 charges over all five arms, 0.49%**, every one of
them a planted `bd` exec'd by `internal/posse/bdwriteshim.go`'s
`syscall.Exec`, the one live Go runtime in this tree that `execve`s a plant
directly.

It did not make the restart pay. This bead does.

## The fact it rests on, MEASURED rather than assumed

The mechanism keys a charge by the task that is stopped in the `execve`. It
is unique only if `seccomp_notif.pid` is the TASK id and not the
thread-group id — the kernel fills it from `task_pid_vnr(current)`, but that
is a reading of the source and not a reading of this kernel.

MEASURED 2026-11-11, this box (linux 7.2.15-1-omarchy x86_64, go1.27.3,
python 3.14.8). A python3 caller spawns a second thread and `execv`s a
plant from it, printing `os.getpid()` and `threading.get_native_id()` first:

| run | tgid | tid | the door's notification named |
|---|---|---|---|
| 1 | 2511336 | 2511364 | 2511364 |
| 2 | 2511745 | 2511762 | 2511762 |
| 3 | 2512072 | 2512094 | 2512094 |

Three of three, tid != tgid in all three. It is the task id. Pinned by
`TestANonLeaderThreadsExecIsAdjudicatedUnderItsOwnTid` (ARM 18), which skips
nothing and fails loudly if a run ever gets a thread that IS the group
leader, because that run could not tell the two readings apart.

## The mechanism

A second map, `inflight`, from `(pid, inode)` to the absolute deadline of a
charge in flight. `d.seen` is untouched.

- a cold first exec publishes `{deadline: now + D}` under that key and
  sleeps to the deadline, exactly as before;
- a notification that MATCHES a live entry is the restart of that charge —
  the kernel restarts a withdrawn syscall in place, on the same task, and a
  task can only be inside one `execve` at a time — so it sleeps the
  REMAINDER to the same deadline instead of being answered warm;
- the creator deletes the entry when it wakes. An entry that lingers costs
  nothing: a notification arriving after the deadline sleeps a negative
  duration.

The invariant is one line: **no notification for a charge is answered before
its deadline**, so the `execve` completes no earlier than D after it entered
the kernel, however many notifications it spends getting there. The charge
stays ONE charge, so ranger-base-in8dr's cost model (D once per cold exec,
additive within 5%) is unchanged, and because the `seen` mark did not move,
two concurrent first execs by DIFFERENT tasks still pay D once between them.

It is symmetric in one case worth naming: if the withdrawal lands before the
first handler has marked the inode, the RESTART is the one that reads cold
and creates the charge, and the original handler takes the restart branch.
One charge either way.

## The accounting: one charge, two notifications

| notification | before | after |
|---|---|---|
| the one cut short | `truncated` | `withdrawn`, naming the plant whose charge it handed on |
| its restart | `warm` | `delayed`, once its answer lands |

`truncated` is what is LEFT: a charge nothing restarted. That is a caller
whose handler lacks `SA_RESTART` (EINTR out of `execv` — MEASURED:
python3's own default disposition, `InterruptedError`, exit 1) or has none
at all (death). In both of those the `execve` never happens, so after this
bead a truncation is no longer an exec that went through at full speed. What
it still costs is the charge, and the one thing it leaves behind is a plant
marked warm — a LATER exec of that inode is not held. That is why the
verdict is still counted and named rather than folded into `withdrawn`.

The live verdict did not move either way: if a handed-over notification's
answer lands after all, that handler really did hold the charge for D and
let the exec through, so it records `delayed`. Every notification still
records exactly one verdict.

## What it cost, MEASURED

Two runs of the same command, back to back on this box (loadavg 0.5-4, one
sibling seat holding a suite slot), D = 2.5s, both doors built from this
tree — the internal/treepins arm filtered to the family the census named:

```
go test ./internal/treepins/... -count=1 -run \
  'TestShippedPIDsLetBeadsOwnHooksRun|TestBroadHookDenyWallsTheCommitAndTheCheckout|TestNarrowedHookDenyStillRefusesInstall'
```

| | before | after |
|---|---|---|
| charges (delayed + truncated) | 58 | 57 |
| **paid** (`delayed`) | 47 | **57** |
| **`truncated`** | **11** | **0** |
| `withdrawn` | 0 | 2672 |
| `warm` | 61 | 51 |
| `old` | 302 | 302 |
| notifications | 421 | 3082 |
| the arm's own wall | 118.371s | 145.762s |

**The eleven are paid and the wall says so.** +27.391s against 11 charges
at D = 2.5s is 10.96 D: the arm got slower by exactly the injection it was
supposed to be delivering and was not. (The 58/57 is run-to-run variance in
how many plants these tests make; `old` is identical in both.)

**What it cost in notifications is the surprise, and it is the fixture's
storm in the wild.** All 2,672 withdrawals name `stub/bd`, and they fall on
**11 distinct charges — 243 each, one every ~10ms for the whole 2.5s hold**.
Before this bead a handed-over charge spent two notifications, because the
first restart was answered warm and the execve completed at once; now the
task stays stopped for D, and the Go runtime goes on preempting it the whole
time. Each cycle costs a notification, a goroutine, a `/proc/<pid>/mem` read
and a stat, and the deadline is what ends it. Priced over the whole door:
ranger-base-4kars' census found 12 such charges across all five arms, so
paying them costs at most 12 x 2.5s = 30s of wall and some 2,900 extra
notifications against that census's third of a million — a per cent of one
arm's wall and well under one of the notifications. (That census's own
totals are quoted two ways in two places, 2,453 charges in ADR 0085 and
2,179 on the bead, because two arms' denominators were cut short by the
/tmp tmpfs running out mid-run; the 12 is the same 12 in both, and the
count of truncations is the only figure this bead builds on.)

## What is still not held, stated rather than left silent

- **A restart that arrives after the deadline** finds no entry and is
  answered warm. That is correct, not a hole: by then the `execve` has
  already been in the kernel for D. The cost is one line of over-reporting
  — a `truncated` and a `warm` for an exec that did take D — and the window
  is a withdrawal landing within microseconds of the deadline.
- **A charge nothing restarts** still loses its injection, and the plant is
  marked warm from there on. Nothing escaped the door, because that `execve`
  did not happen; what is unheld is some LATER exec of the same inode.
- **Pid reuse inside D** could in principle make an unrelated task's first
  exec of the same inode look like a restart, which would make it sleep
  somebody else's remainder. It cannot make an exec go through EARLY, which
  is the only direction that matters, and it requires the charged task to
  die, its pid to be reused and the new task to exec the same inode-at-the
  same-ctime, all inside D.
- **The storm converges at the deadline and not before.** A caller that is
  signalled repeatedly is withdrawn and restarted repeatedly, each cycle
  costing one notification; the last restart is the one that completes,
  after the entry is gone. The door answers a restart in ~100µs against
  signals arriving every few ms, so it converges — and the deadline bounds
  how long it can go on.

## The pins

- **ARM 14** `TestAWithdrawnNotificationIsNotAHole` — j0riv's, unchanged in
  subject. Its "the restart was adjudicated too" reading now counts
  `withdrawn` among the spellings, because a withdrawal that handed a charge
  on names the plant it handed on.
- **ARM 16** `TestARestartedExecvePaysTheRestOfItsCharge` — this bead's. Two
  cold plants, both in the delayed count, no truncation, and a wall-time
  FLOOR on the nudged exec. The floor is new: before this bead there was
  nothing to time, and a floor is the one direction load cannot flake (the
  header's objection is to a RATIO of two wall times). MEASURED: the mutant
  that answers the restart warm reds this pin at `nudged took 10ms, want at
  least 400ms` and leaves ARMs 14, 17 and 18 green.
- **ARM 17** `TestAChargeNothingRestartsReadsTruncated` — what is left of
  ranger-base-4kars' subject. Same fixture, `saRestart=false`, so the caller
  takes EINTR: `truncated` names the plant, `delayed == 1` (the second plant,
  which nothing signals), the plant does NOT run, and the caller exits 1. No
  wall time: "released at the signal and not at D" is a CEILING, and this
  file's header is right about ceilings on this box.
- **ARM 18** `TestANonLeaderThreadsExecIsAdjudicatedUnderItsOwnTid` — the
  kernel fact the key rests on. It fails loudly rather than passing
  vacuously if the exec'ing thread turns out to be the group leader.
