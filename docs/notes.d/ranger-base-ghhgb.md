# ranger-base-ghhgb — the nudge that outlived the execve, and the signal that cannot

`withdrawalFixture` nudges a stopped execve until it lands, guarded by the
caller's own `/proc/<pid>/exe`. A poll has a window on the far side of the fact
it polls: the execve completes, the plant is running under that same pid, and
the nudger's next `readlink` is up to 10ms away. The signal lands on the
**plant**.

The plant's answer was its first line, `trap "" USR1` — a trap it has to be
RUNNING to install. It covered most of those microseconds and could not cover
all of them, and when it lost, SIGUSR1 at its default disposition killed the
plant after its exec had already been adjudicated and paid for: `CALLER EXIT
138` on a run where the door had done everything the pin asserts about it.

Environment for everything below: this box, linux 7.2.15 x86_64, go1.27.3,
D = 600ms (`pinDelay`), 2026-11-12.

## The fix is the choice of signal, and it closes the window rather than narrowing it

Two kernel facts, both MEASURED here (the script is in the bead's session, the
shape is ARM 21):

1. `execve` resets a **handled** disposition to `SIG_DFL`, so nothing the
   caller installed survives into the plant. Whatever the plant faces, it
   faces the default.
2. SIGURG's default disposition is **Ignore**. SIGUSR1's is terminate.

So a nudge spelled SIGURG is a no-op on the plant from its first instruction,
with no window at all — and the plant needs no `trap`, which also means no
handler to interrupt its own `echo`. Same script, same deliberately post-exec
nudge, under each signal:

| nudge | plant | caller |
|---|---|---|
| SIGURG | printed its marker | exit 0 |
| SIGUSR1 | printed nothing | **exit 138** |

SIGURG is not a workaround picked for its disposition alone. It is the signal
the Go runtime preempts with, which is what produces withdrawals under this
door in the wild (the note above `holes` in `door_linux.go`), so the fixture is
now the real shape rather than an analogue of it. A nudge that lands on a
**reused** pid — the loop breaks on `readlink`'s `OSError`, but only at its
next tick — is harmless for the same reason.

What it costs: nothing in the mechanism. A handler is still installed before
the fork, and for a sharper reason than before — an *ignored* signal does not
interrupt the stopped execve at all, so a run that nudged before
`signal.signal` would reach none of the mechanism. Both endings still happen:
`siginterrupt(False)` restarts the execve (ARM 14, ARM 16) and python3's own
default disposition takes EINTR (ARM 17, ARM 20).

## The signal is a constant, and ARM 21 is the pin on it

`nudgeSignal` is read by the fixture's caller and by ARM 21
(`TestANudgeThatLandsAfterTheExecveLeavesThePlantRunning`), so a seat that
spells it SIGUSR1 again — or SIGTERM, SIGINT, SIGHUP, any default that kills —
reds in one place deterministically instead of getting a sub-percent flake in
arm 1 whose message points at the door.

ARM 21 is deterministic because it **inverts the guard**: its nudger waits for
the exe link to CHANGE and signals then, so the post-exec window is reached
every run rather than 2 times in 150. The handler is installed exactly as the
fixture installs it, because the composition is the claim. And the nudger
writes its `nudged` file 50ms AFTER the `kill`, so the plant cannot see that
file — and cannot print — until long after a fatal signal would have ended it.
MEASURED: 3/3 pass at SIGURG, 3/3 red at SIGUSR1 with `CALLER EXIT 138`, which
is the verbatim symptom this bead was filed for.

## The fixture family's flake rate after the fix, MEASURED

150 executions of each of the five pins that now read `withdrawalFixture` or
`nudgeSignal` — ARM 14, ARM 16, ARM 17, ARM 20, ARM 21 — one `go test -run`,
252.9s, one-minute loadavg 9.5 at the start:

| pin | ending | failures |
|---|---|---|
| ARM 14 `…NotificationIsNotAHole` | SA_RESTART | **0** / 150 |
| ARM 16 `…PaysTheRestOfItsCharge` | SA_RESTART | **0** / 150 |
| ARM 17 `…RestartsReadsTruncated` | EINTR | **0** / 150 |
| ARM 20 `…LeavesItsPlantWarm` | EINTR | **0** / 150 |
| ARM 21 `…LeavesThePlantRunning` | post-exec | **0** / 150 |

Against ranger-base-dgbys' census of the same four pins eleven days earlier —
0, **10**, 1, 1 — the family's three known mechanisms are now all answered and
this was the last of them open: the deadline-edge accounting is
ranger-base-k9xsd's two honest spellings in ARM 16 (closed), the nudge that
beats the door to the notification is ranger-base-9n4y9's retried positive
control in ARM 17 and ARM 20 (closed), and the nudge that outlives the execve
is this one.

Nothing about the door changed here. `cmd/coldexec` holds one more test file
line and one fewer `trap`.
