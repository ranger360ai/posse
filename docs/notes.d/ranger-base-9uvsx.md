# One notification, one verdict (ranger-base-9uvsx)

*2026-11-22 · gwart · why cmd/coldexec's report summed past its own
notification count, the uniform fix, and the measurement that decided where
the pin had to live.*

## The claim

`cmd/coldexec`'s `door` struct says "every notification lands on exactly one
verdict", and the report prints the two numbers side by side:

```
coldexec: 19 execve notifications, 1 cold first execs delayed 600ms each (0 warm, 18 older than the door, 0 absent)
coldexec: 5 notifications were withdrawn before the door could answer — …
```

Nineteen notifications; 1 + 0 + 18 + 0 + 5 = twenty-four verdicts. The claim
was not true.

`adjudicate` settled three of the nine verdicts itself — `old`, `warm`,
`absent`, the three it can decide with no question left for the kernel — and
recorded them on the way in, answering a `vNone` sentinel to say so. The
holes and the delays did not: both rode down to `answer()`, which records a
verdict only once `SECCOMP_IOCTL_NOTIF_ID_VALID` has said the notification
was still there (ranger-base-j0riv, ranger-base-4kars). So when the kernel
withdrew a notification whose verdict `adjudicate` had already written,
`answer()` recorded `vWithdrawn` **as well**: one notification, two verdicts.

MEASURED 2026-11-11 (this box, linux 7.2.15 x86_64, go1.27.3), D = 2.5s,
over all five `make coldexec-check` arms with `-verbose`:

| arm | notifs | sum(verdicts) | withdrawn |
|---|---|---|---|
| 1 (posse) | 129555 | 129559 | 4 |
| 2 (arm2) | 113769 | 113769 | 0 |
| 3 (arm3) | 83172 | 83172 | 0 |
| cmd/posse | 4111 | 4111 | 0 |
| treepins | 23841 | 23841 | 0 |

Four of five close exactly; arm 1 overshoots by exactly its withdrawn count.
An earlier arm 1 run the same day closed exactly with 1 withdrawn, because
that one notification's pending verdict was a HOLE — recorded only at the
answer, so the withdrawal replaced it instead of adding to it.

## Why it was worth fixing rather than documenting

Nothing here is false about coverage, which is what separates it from
ranger-base-4kars. That bead's defect was a delay the door REPORTED and did
not pay; this one's withdrawn `old` really was old, and nothing escaped the
door. What is wrong is the arithmetic, and the struct's own stated
invariant — a report a seat reconciling it cannot reconcile.

The fix is uniform and smaller than the comment that described the
exception: hand `old`, `warm` and `absent` down to `answer()` as pending
verdicts, exactly as the holes and delays already are, and let `answer()`
record either the pending verdict or the withdrawal. `vNone` goes with them,
and so does `statVerdict`'s second `hole bool` return — both existed only so
`adjudicate` could choose which verdicts it recorded itself. One verbose
`old` line now prints at the answer rather than at the stat, which is where
every other verdict's line already printed.

A withdrawal's `-verbose` line also names a file now. It used to read
`(the kernel took the notification back before the answer)` for exactly
these three verdicts, because `adjudicate` had kept the target for the line
it printed itself — that placeholder *was* the double-counted notification's
second line.

## Where the pin had to live, and why it is two pins

The bead asked for `notifs == sum(verdicts)` over a run that produced a
withdrawal. That is `doorRun.wantReportAccountsForEveryNotification`, and
`runDoor` makes it on every supervised run in the file — it costs a parse of
output already captured, and it holds the number a seat actually reconciles.

But it is not a pin for this invariant, and the measurement is the reason.

MEASURED 2026-11-22 (this box, linux 7.2.15 x86_64, go1.27.3, D = 600ms,
one-minute loadavg 2.3-6.8, no suite slot held by anyone), 16
`go test ./cmd/coldexec` runs against the pre-fix `door_linux.go` with the
new test file in place:

| batch | runs | redded on the arithmetic |
|---|---|---|
| A | 1 | TestAHookGitExecsPaysD, 19 notifications against 24 verdicts |
| B | 5 | none — all five closed exactly |
| C | 10 | 3 of 10: TestACopiedBinaryPaysD at 9 against 10, then TestATruncatedChargeLeavesItsPlantWarm at 16 against 17, twice |

Four of sixteen, in three different tests, on a quiet box. The withdrawals
land on `old` execs of `/bin/sh` and `/usr/bin/git`: git is a Go binary, so
its own preemption `SIGURG` is what takes the notification back — the wild
shape the note above `holes` in door_linux.go describes — and the window is
the microseconds the door spends in its `/proc` reads, which needs no D at
all. Which test draws it is whichever one the run queue hands it to.

That is the defect's SYMPTOM. It is ranger-base-j0riv's red exactly: a
failure that tracks the loadavg and not the code, which there cost a red arm
1 and three beads looking at the wrong thing. A pin for an invariant has to
red on the code, so `TestANotificationSpendsExactlyOneVerdict` (ARM 22)
reads it off `handle()` with the listener a closed fd — the arm where every
answer reads as withdrawn, which is the arm the defect lived on — and covers
all three verdicts with a positive control each that `adjudicate` reaches
the verdict and records nothing. ARM 19 is the same shape for the holes and
reached it from the other side: there the live window was unreachable, here
it is reachable a quarter of the time, and both answers are "read it off
handle()".

Two small things that are the pin's own and not the door's:

- the epoch is `time.Now().Add(-time.Second)`. A file's ctime comes from the
  kernel's COARSE clock, which can read up to a tick behind `time.Now()`, so
  a plant written straight after an epoch of `time.Now()` reads `old` — 3 of
  3 runs on tmpfs here. A supervised run gets that margin for free; a unit
  pin does not.
- `verdictName` spells an out-of-range verdict instead of panicking on it.
  That is for the mutant: reverting this fix reintroduces `vNone` (-1) for
  exactly the verdicts the pin reads, and the first run against the pre-fix
  door produced an index-out-of-range stack instead of the message. ARM 19
  has the same exposure and was left alone.
- the planted file is **0o644 and never exec'd**. `make execwrite-check`
  forces every `os.WriteFile` with an exec-bit literal in the tree through a
  fork-locked writer (golang/go#22315), and cmd/coldexec cannot reach one: it
  imports nothing from internal/posse on purpose (doc.go's exit hatch), and a
  package-local copy — internal/treepins' answer to the same import problem —
  would make cmd/coldexec the FOURTH planting package and red
  `TestQAEveryPlantingPackageHasAColdexecArm` until `coldexec-check` grew an
  arm over its own tests. None of that is needed, because the pin execs
  nothing: it hands `handle()` a notification and the door stats the inode,
  the kernel checking the mode only after the answer. Caught by the gate, not
  by review — `make test`'s first run on this bead redded there.

## Verified

| | |
|---|---|
| `make test` | exit 0 — all three arms, the gates and the silent-revert audit (arm 2 255.7s, arm 3 117.3s) |
| `make tree-check` | green, 98.1s at a one-minute loadavg of 7.5 (over the FINAL comment text) |
| `go test ./cmd/coldexec` | ok, `-count=1`, `-count=2` and `-count=5` (9.768s) |
| ARM 22 against the pre-fix door | FAIL on all three cases, in each of the 6 runs whose per-case output was read (every one of the 16 exited FAIL) |
| the report assertion against the pre-fix door | FAIL 4 of 16 runs (table above) |

## Not closed here

Nothing. ranger-base-4kars' "Not closed here" named this bead; its own
residual list is empty.
