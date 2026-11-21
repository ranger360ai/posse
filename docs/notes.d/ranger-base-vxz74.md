# The wall step's three layers are per-task, and it did not hold a thread (ranger-base-vxz74)

2026-11-21. `posse wall-unix` is the step inside the bwrap cage that applies
the seat's Landlock domain and then becomes the runtime (ADR 0080 D27, D31).
Three of the four things it delivers belong to a THREAD and not to the
process:

- `prctl(PR_SET_NO_NEW_PRIVS)` sets the calling thread's bit;
- `landlock_restrict_self(fd, 0)` puts the calling thread in the domain — the
  whole thread group only with `LANDLOCK_RESTRICT_SELF_TSYNC`, which this call
  does not pass;
- `pdeath_signal`, which bwrap's `--die-with-parent` set on the task it
  cloned, is **zero on every thread the Go runtime clones afterwards**.

`syscall.Exec` runs on whichever M the goroutine happens to be on, and nothing
in the step called `runtime.LockOSThread`. A park, a preemption or a syscall
handoff between the restrict and the exec would therefore have exec'd the
runtime on a thread with no NNP, no Landlock domain and no pdeathsig — while
the render and `posse gates` both still printed the sentence that says the
layer is up. ADR 0080 D4 is the rule that a wall posse reported and does not
have refuses rather than opens; this was that case, arriving as a fail OPEN.

The comment on `liveRestrictWallUnix` said the apply reached "the calling
thread group". That was false of `landlock_restrict_self(fd, 0)`, and the
false sentence is the reason nobody looked.

## Why it is a P2

An attacker cannot steer the Go scheduler. The hazard depends on a scheduling
decision, it was never reproduced, and the pdeathsig is the one observable
that can say afterwards which thread exec'd — after `execve` the surviving
task takes the leader's tid by number (`de_thread`), so `/proc/self/status`
inside the seat reads `Pid == Tgid` either way and cannot be asked.

MEASURED 2026-11-21, D1 host (Linux 7.2.15-1-omarchy, go1.27.3), hoover's
reading before the fix: 600 of 600 runs of

```
setpriv --pdeathsig KILL -- posse wall-unix -- python3 <PR_GET_PDEATHSIG>
```

read 9 — across all CPUs, under `taskset -c 0` with `GOMAXPROCS=1`, and on two
CPUs with `GOMAXPROCS=2`. Every one of those execs had already run on the
original thread. Latent, and fails open, which is why it was filed.

## The fix

`runtime.LockOSThread()` as `RunWallUnix`'s first statement, never unlocked,
and a refusal before the restrict unless `gettid() == getpid()`:

- the LOCK closes the window — nothing can move the goroutine once it is held,
  so the NNP, the domain and the exec all land on one task;
- the LEADER CHECK is what makes that task the one the pdeathsig is on, since
  no Go program can ask to be moved TO a particular thread. Off the leader the
  step refuses and names the thread, the process and the fact that a relaunch,
  not a kernel, is the answer.

`cmd/coldexec`'s `runChild` is the same shape one layer down — one locked
thread, seccomp in place of Landlock — and its comment is where the shape was
already written out in this tree.

### No TSYNC, and that is a decision (ADR 0080 D39)

`LANDLOCK_RESTRICT_SELF_TSYNC` would be belt-and-braces over a window the lock
has already closed; it cannot carry the pdeathsig, which is per-task and set
on ONE task, so no flag of any syscall can propagate it; and it is gated. The
flag is `(1U << 3)` of `landlock_restrict_self`'s flags in this box's
`linux/landlock.h` (linux-api-headers 7.3-1), this tree has never measured
which Landlock ABI introduced it, and an unknown flag is `EINVAL` — which
`RunWallUnix` turns into a seat that does not open. Passed ungated it would
trade a working wall for a refusal on every kernel that does not have it.

## After the fix

MEASURED 2026-11-21, same box, same three cells, 600 execs: **600 of 600 read
pdeathsig 9 and the leader check refused 0 times** (400 all-CPU + 100 `taskset
-c 0` `GOMAXPROCS=1` + 100 `taskset -c 0,1` `GOMAXPROCS=2`; 11.0s wall at a
one-minute load average of 9.65, 16 cores). So the check costs no seat on this
host, and the pdeathsig is where it was.

The three live bwrap pins pass behind the fix, both `--die-with-parent` arms
included: `go test -run TestLiveWallUnix ./internal/posse` green, control arms
and all.

## The pins, and the mutant each one is for

Four mutants were planted one at a time and the files restored byte-identical
after each (MEASURED 2026-11-21):

| mutant | pin that reds |
|---|---|
| the leader check never fires (`false && tid != pid`) | `TestQAWallUnixStepRefusesOffTheThreadGroupLeader`, and the live one |
| `runtime.LockOSThread()` replaced by a no-op | `TestQAWallUnixStepLocksItsOSThreadFirstAndNeverUnlocks` |
| the lock is no longer the first statement | the same source pin |
| `liveWallUnixTask` reads `getpid` twice, so everything is "the leader" | `TestQAWallUnixStepRefusesFromARealNonLeaderThread` |

Two of those deserve a note.

**The lock has no observable, so its pin reads the source.** A run without the
lock passes every behavioural pin in the package and passed 600 live execs on
this box. The first thing to notice a lost lock would otherwise be a seat that
came up unwalled, saying so nowhere. The pin parses `wallunix.go`, asserts the
first statement of `RunWallUnix` is `runtime.LockOSThread()` and that the file
calls no `UnlockOSThread`. It names its one file and enumerates nothing, so it
is outside internal/treepins' one-package-census class and needs no Makefile
door (`treewidedoor_qa_test.go`'s derived key: discovered file sets need a
door, named ones do not).

**The live pin takes the thread id from the kernel, not from the probe.**
Without that, the fourth mutant would not red — it would SKIP. A
`liveWallUnixTask` that answered `getpid` twice reads "always the leader"
everywhere; the stubbed pin stays green because it stubs the probe, and the
live pin would decide it had been handed no non-leader thread and abstain. A
skip is green. So `/proc/thread-self/status`'s `Pid:` row decides whether the
arm is set up, and a probe that disagrees with it is a failure in its own
right.

## Not closed here

- ranger-base-z56s9
