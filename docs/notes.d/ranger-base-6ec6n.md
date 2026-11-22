# bwrap's --die-with-parent does not survive an AppArmor exec transition, so the step re-arms it and wall-watch kills what is left (ranger-base-6ec6n)

2026-11-21, dinesh. The finding is ranger-base-evp5k's.

## What breaks

bwrap sets `PR_SET_PDEATHSIG, SIGKILL` on its child as the last thing before
`execvp`. On a host where AppArmor puts that exec into a different label —
ubuntu resolute's `bwrap-userns-restrict` moves bwrap's children into
`bwrap//&unpriv_bwrap` — `apparmor_bprm_committing_creds` zeroes
`pdeath_signal` on the way in. Nothing bwrap does can survive that exec, so a
seat whose launcher dies keeps running, reparented, with its cage on and
nothing supervising it. That is why `TestLiveWallUnixScopesSignalsToTheSeat`
was red in both death arms on every ubuntu-26.04 arm-1 job.

## What changed

1. **The step re-arms it.** `posse wall-unix` is the first exec under the new
   label. After the Landlock apply, on its locked leader thread, it calls
   `prctl(PR_SET_PDEATHSIG, SIGKILL)`, then re-reads `getppid()` against the
   value it read at its first statement, and refuses if the parent changed (the
   standard race guard). Then it execs the runtime. A failed prctl refuses the
   seat too.
2. **wall-watch kills an orphaned seat.** The watcher is bwrap's parent, so it
   sees bwrap exit. If the seat's leader is still alive in the seat's mount
   namespace at that moment, bwrap was killed under it and its
   die-with-parent did not fire. The watcher then kills every process in the
   namespace and writes a `KILLED seat …` line to the verdict file. A seat that
   ended by itself has no leader left, so nothing it left behind is touched.
   This covers seat lines with no step (a kernel below the signal-scope ABI),
   and a parent that died before the step's first statement.
3. **The control arm asserts the platform.** When the outer bwrap's AppArmor
   label differs from the seat's, and the seat's label is not unconfined, the
   no-step death arm expects leader `pdeathsig 0`, logs why, and passes
   without the 10s wait. A label change that left 9 in place falls through
   to the ordinary death check. The step arm must read leader `pdeathsig 9`
   on every host.

## Measured, and what is not

MEASURED 2026-11-21 on D1 (Linux 7.2.15-1-omarchy, bubblewrap 0.13.1,
Landlock ABI 10, no AppArmor):

- `TestLiveWallUnixStepReArmsAClearedDieWithParent` models the cleared
  pdeathsig by starting the step from a plain shell with no bwrap. The
  control (no step) reads leader `pdeathsig 0` and outlives its SIGKILLed
  shell. The step arm reads 9 and dies.
- `TestLiveWallWatchKillsASeatItsBwrapLeftRunning` models it by taking
  `--die-with-parent` off the bwrap line. SIGKILLing bwrap leaves the leader
  running, and the watcher kills both the leader and its backgrounded job.
  The control seat exits 7 by itself, and its backgrounded job survives.
- The mutants are listed on the bead.

ASSUMED, not measured: the step's own exec into the runtime stays in the
`unpriv_bwrap` label and so does not clear the re-armed value a second time.
D1 cannot read this. The step arm of `TestLiveWallUnixScopesSignalsToTheSeat`
reds on the ubuntu leg if it is wrong (leader pdeathsig not 9).

## Not covered here

- If the watcher itself is SIGKILLed on an AppArmor host whose seat line has
  no step, bwrap dies through its own pdeathsig, which holds because no exec
  follows it, and the seat is orphaned with no watcher left to kill it. The
  dispatch backstop's dead-watcher arm (ranger-base-0qpsw D3) still files its
  P1 for that seat and keeps diffing its walls, but it does not kill it.
