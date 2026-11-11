# A withdrawn notification is not a coverage hole (ranger-base-j0riv)

*2026-11-11 · gwart · why `gone` redded cmd/coldexec's ARM 3 pin
load-sensitively inside arm 1, what the kernel is actually doing, and the
one rule that closes the whole family rather than the one instance.*

## The question the bead asked

`TestShRunningThePlantIsNotDelayed` redded on `wantNoHoles` when cmd/coldexec
ran as one package among five under `go test ./...`, and passed standalone —
MEASURED by gilfoyle on the bead: FAIL at a one-minute loadavg of 12.95, ok
three times at 7.32 and below. arm 1 is `make test`'s first test line and
nobody's optional door, so the cost was a red package and a non-zero exit
for a door whose injection was never in question.

The bead left the judgement open: is `gone` a hole at all? A notification
whose caller died before the answer covered nothing — but there was nothing
left to cover.

## What is actually happening, MEASURED

The answer is not "the caller died". It is that **the kernel took the
notification back**, and it does that to a caller that is very much alive.

seccomp's wait for the supervisor's answer is interruptible. When the stopped
caller takes a deliverable signal, the kernel REMOVES the notification from
the listener's list and sets the syscall's return to `ERESTARTSYS`. What
happens next is decided by the handler the caller had installed, and all
three endings were MEASURED here on 2026-11-11 (this box, linux 7.2.15
x86_64, go1.27.3), each one run under this door with a signal sent into the
door's own delay window:

| the caller's handler | what the execve does | what was seen |
|---|---|---|
| **with `SA_RESTART`** — the Go runtime's shape, its preemption `SIGURG` included | **restarts**, raising a FRESH notification which is adjudicated | one plant read `delayed` then `warm`, ran, caller exit 0 |
| with a handler but no `SA_RESTART` (python3's) | returns `EINTR`; the exec never happens | Python raised `InterruptedError`, exit 1 |
| no handler, default-fatal | the caller dies stopped; the exec never happens | exit 138 for a default `SIGUSR1` |

**The kernel never completes an execve whose notification it has taken
back.** So nothing escapes the door at full speed in any of the three. What a
withdrawal really costs is arithmetic: in the restart case one execve spends
two notifications, and the door was reporting the first of them as an exec it
had failed to cover. The bead's own evidence shows that double count without
naming it — 12 notifications against 12 `old` plus 7 `gone`, and each `gone`
line paired with an `old` line of the same pid.

### Why it is the supervisor's own child, and why it tracks load

The pid in every one of those pairs is the door's `-child`: the one process
under the door with an `SA_RESTART` handler on a signal its own runtime
sends. It is a Go program calling `unix.Exec` from a live runtime, so it is
interruptible — unlike a plant exec'd through `os/exec`, whose child blocks
every signal across fork+exec, and unlike `sh`, which resets handled
dispositions before `exec` so the signal is fatal instead. The wider the
box's run queue, the more preemption signals that child takes while stopped
in its execve, which is the whole of the load sensitivity.

Standalone, on a quiet box (loadavg 1-2), the ARM 3 fixture produced a
withdrawal in 3 of 100 runs, and 9 and 8 of 1500 in two later batches — so
the race is not rare and not concurrency-only; concurrency just raises it to
where a suite notices.

## The decision

`withdrawn` (renamed from `gone`, because the old name asserted something
about the process that the door cannot know and that is usually false) is
**not** a hole. It is reported, with its own sentence saying why it is not
one, so a seat whose run spent two notifications on one execve has something
to read.

The rename is the point rather than a tidy-up: the verdict's name was the
claim, and the claim was wrong.

## The rule that closes the family

Reclassifying one verdict would have left the same bug twice over. The first
1500-run batch after the narrow fix still produced one `un-adjudicated` run,
and the reason generalizes: **the caller can be unreadable, or its pid gone,
precisely BECAUSE the kernel withdrew the notification under us.** Every hole
`adjudicate` can reach is load-sensitive in the same way and for the same
reason.

So the rule is now one line, and it is the taxonomy's own words made true: *a
hole is an execve the door let through without knowing what it was, and
whether it was let through at all is decided at the ANSWER and nowhere else.*
`adjudicate` records the covered verdicts itself and records no hole; every
hole rides down to `answer`, which records it only once `ID_VALID` has said
the notification was live and the answer has landed. One verdict per
notification, always — which is what the `door` struct claimed and did not
have.

`statVerdict` loses its `gone` branch entirely as a result: an absent pid is
a guess (a pid is reusable, and a caller that exec'd and exited reads exactly
like one that died stopped) and `ID_VALID` is the only authority, so an
absent pid is now an unstattable like any other and the answer decides it.

MEASURED after the change: 1500 ARM 3 fixture runs, 8 withdrawals exercised,
**0 holes**.

## The pin

`TestAWithdrawnNotificationIsNotAHole` (ARM 14). Nothing redded when a door
called its own race a coverage hole, which is how this shipped.

The fixture is a python3 caller — `signal.siginterrupt(sig, False)` is the
shortest way to install an `SA_RESTART` handler, the Go runtime's shape — and
the nudging is a FORKED child of that caller guarded by the caller's own
`/proc/<pid>/exe`, not a sleep in the script. The window to hit is D and this
box reaches a loadavg of 50-100, so a timed sleep would be the flake
cmd/coldexec's test header warns about. The loop is deterministic in both
directions: a signal that arrives too early is simply handled and retried,
and the instant the exe link changes the plant is running under that pid and
must not be signalled. 40/40 identical on a quiet box; 10/10 under `go test
-count=10`. It reds on the one-line mutation that puts `vWithdrawn` back into
`holes`.

What it deliberately does NOT assert is wall time — see below.

## Handed on

**ranger-base-4kars** (`-l code`, filed not fixed): a withdrawal
**truncates** the injection. `adjudicate` marks `d.seen[key] = true` before
it sleeps, so the restart reads WARM and is answered at once — the plant's
exec goes through in ~10-20ms against the 600ms the report charges it
(MEASURED, 40/40), while the original handler sleeps out the full D against a
notification nobody is waiting for. The accounting says the injection
happened; the wall time says it did not.

That is a different claim from this bead's — it is about whether the delay
the door REPORTS was paid, not about coverage — and the fix is a design
choice with a cost (moving the `seen` mark is also what stops two concurrent
first execs of one inode from both paying D), resting on a number nobody has
measured: how many plant execs are both interruptible and delayed. ARM 14
asserts no wall time so that it stays green whichever way that lands.

## Not an ADR change

ADR 0085 says nothing about `gone` or about holes; the verdict taxonomy lives
entirely in `cmd/coldexec/door_linux.go`. D5's claim — "the door covers every
first `execve`, by any process under it, of an inode created after it
started" — is unchanged and still true: a withdrawn notification creates no
uncovered edge, because the restart re-covers it. So there was nothing to
amend.
