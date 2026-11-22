# ranger-base-evx07 — a walled seat SIGKILLed from outside, and why the fix is not finding the sender

*from ranger-base-s9h52 · the fifth bead of the class ranger-base-a688i,
-w6kke, -axt1f, -s9h52 opened · probe:
`docs/notes.d/ranger-base-evx07.probe.sh` — arms A/C/D by default,
`… nsino` and `… pdeathsig` for the two slow ones*

## What the bead asked and what this answers

The bead asked: **what sends SIGKILL to a bwrap seat in the ADR 0097 D1 venue
when the box is loaded by a concurrent sibling suite**, and named three
candidates to read in order — the fleet load guard's kill arm, the cgroup the
venue runs the seat in, and the kernel pressure path.

**The sender is still unnamed, and this bead argues it is the wrong
deliverable.** Linux keeps no record of who sent a signal. The one log that
would name `systemd-oomd` is the system journal, which a seat cannot read
(see below); `auditd` and an eBPF `signal_generate` probe both need privilege
a seat does not have. So "name the killer" cannot be closed from this lane by
investigation — a sixth bead would be a sixth instrument, which is the cost
the bead itself objected to.

What **is** fixable, and is the defect, is the line the bead's own instrument
already writes and then throws away: `bgSeatAccount` names the shape — *"the
shape of a seat ended from outside rather than one the wall turned away"* —
and every caller turns it into a member FAIL anyway. See *The fix*.

## The structural discriminator (the reading the earlier four beads lacked)

`exit status 137` is **bwrap exiting NORMALLY** with 128+9, bubblewrap's
`propagate_exit_status` for a signalled sandbox child. So the victim is the
process **inside** the sandbox, and the `go test` that forked bwrap survived to
report it.

That rules out an entire family in one step. **bwrap creates no cgroup of its
own**, so a seat shares its `go test`'s cgroup. Any cgroup-scoped killer —
kernel memcg OOM group-kill, `systemd-oomd`'s `cgroup.kill`, a slice-level kill
— would have taken the reporter with it, and there would be no member line at
all. The killer selected an individual pid, or the death was senderless
(pdeathsig, a pid-namespace init death).

MEASURED 2026-11-21, linux 7.2.15, bubblewrap 0.13.1, go1.27.3, 16 cores
(probe arm D). bwrap propagates its **sandbox child's** status and nothing
deeper, so what the member reads depends on what that child did with the death
below it — and **137 arrives by two different routes**:

| command inside the render | bwrap rc | |
|---|---|---|
| `exec` of a self-killing script | **137** | signalled child; `propagate_exit_status` renders 128+WTERMSIG |
| no `exec`, the killed child goes last | **137** | the shell SURVIVED and reported 128+9 as its own ordinary exit |
| no `exec`, the shell carries on (`; exit 7`) | 7 | the kill is **masked entirely** |
| `exit 128` | 128 | git's own answer at `cannot lock ref` |

None of the four printed anything. The first two are why
`bgSeatEndedFromOutside` reads `ExitCode` and not `WaitStatus.Signaled` alone:
one is a signal death and the other a normal exit carrying the same number, and
a member cannot tell them apart nor needs to. The third is why
`packUnderALiveSeat`'s inner line execs — so the commit's own death is the
sandbox child's death and cannot be masked. The fourth is the row that must
keep redding.

*(An earlier draft of this page and of the code comment said row 2 read `rc 0`.
It does not; that reading came from a mis-quoted `$$` that expanded in the
wrong shell. Both are corrected, and the probe now runs the arms from a script
file so the quoting cannot drift again.)*

## Ruled out, each with its reading

**1. The fleet load guard's kill arm — ARMED on this instance, and it still
cannot select a seat.** The bead's first candidate, and the code is misleading
about it: `loadguardkill.go`'s header says *"SHIPPED OFF.
`load_guard_kill:` defaults to false and this bead does not flip it"*, while
`~/homes/ranger/config.yaml:271` reads `load_guard_kill: true`. The comment is
true of its own bead and false of this box, which is a trap for the next
reader of an incident (filed: see *Handed off*). Ruled out by **predicate**,
which is stronger than a log read — a log can be stale or rotated, a predicate
cannot select what it cannot match. The arm requires all of: **ppid 1**
(`Proc.Orphaned`), the **ADR 0009 gate-shell preamble anchored at the head of
the `-c` string**, over a **%CPU floor**, over an **age floor**, and no
`POSSE_KEEP` marker. A nested seat forked by a test binary has ppid = that
binary; its argv is `bwrap --args … -- sh -c "echo up > …"` with no preamble;
it is **idle at a ready file** at the moment it is killed; and it is seconds
old. Four independent misses.

**2. The kernel and memcg OOM killers.** `memory.max` is `max` at every level
from `rex.service` up through `user@1001.service`, `user-1001.slice` and
`user.slice`, and `memory.events` reads `oom 0 oom_kill 0 oom_group_kill 0` at
every one of them. Those counters are **cumulative since boot**, so this rules
the family out over the whole lifetime of the box, not just at the moment
somebody ran `dmesg`.

**3. `systemd-oomd` — active, and both wrong-shaped and far from its
thresholds.** It is `active` and `enabled`, with omarchy's drop-in
(`/etc/systemd/oomd.conf.d/10-omarchy.conf`) setting
`DefaultMemoryPressureLimit=50%` and `DefaultMemoryPressureDurationSec=20s`,
and `ManagedOOMMemoryPressure=auto` / `ManagedOOMSwap=auto` on `user.slice`.
Its swap arm kills over 90% swap used; swap read **4.4G of 54G (8%)**, all of
it zram. Its pressure arm needs 50% for 20s; `/proc/pressure/memory` read
`full avg10/60/300 = 0.00` with a cumulative `full total` of 127.6s since
boot. And it kills by `cgroup.kill`, which is the wrong shape per the section
above.
**The honest gap:** oomd's kills are logged only to the system journal, and
this seat's user is in none of `adm`, `systemd-journal`, `wheel` — `journalctl
-u systemd-oomd` answers *"not seeing messages from other users and the
system"*. **No seat-side instrument can ever name oomd.** That is a permanent
limit on this class of report, and it is the main reason the *Fix* below is
not "instrument harder".

**4. A sibling member of the same venue run.** The candidate the bead did not
list, and the best of them: the member set run in one `go test` includes
`TestLiveWallWatch*` arms whose whole job is to **SIGKILL bwrap seats**, via
`wallWatchSeatPids`' census over the real `/proc` — which, in the venue's own
pid namespace, contains every other member's nested bwrap. They are all
`t.Parallel()`. Two routes, both closed:

- *By namespace inode.* `wallWatchSeatPids(0, mntns)` selects pids purely by
  comparing `/proc/<pid>/ns/mnt`'s inode to a number read from a record. The
  code reasons explicitly about recycled **pids** (*"a bwrap pid read from a
  record whose watcher is gone may name a recycled, unrelated process"*) and
  says nothing about recycled namespace **inodes**. MEASURED (probe arm B):
  **200 sequential bwrap mount namespaces, 200 distinct inodes, 0 reuses**,
  monotonic over `1456586674 .. 1456613593`. nsfs inodes do not recycle here.
- *By pid.* Closed, but **not** by the "pid_max is 4M so pids never recycle"
  argument that first draft used — that argument is wrong on this box.
  `wallwatchbackstoplive_linux_test.go` ends with
  `t.Cleanup(func() { syscall.Kill(bg, syscall.SIGKILL) })`, a SIGKILL to a pid
  read from a file, by then already dead and reaped, with no liveness or
  identity re-check — an individual-pid kill from inside the same `go test`
  run, which is the shape exactly. And pid reuse here is **live and fast**:
  MEASURED 2026-11-21, `last_pid` was directly observed wrapping (3634783 →
  173230 between two readings minutes apart), and the rate ranges from
  **117 pid/s idle (wrap ≈ 10 h)** to **~9500 pid/s under a suite
  (wrap ≈ 7 min)**. Pids are issued sequentially, so a freed pid is reissued
  only after a full wrap — which at the loaded rate is minutes, not days.
  What closes it is **the venue, not `pid_max`**: ADR 0097 D1's venue is
  `--unshare-pid`, so its `go test` and every nested seat live in a **fresh pid
  namespace whose counter starts at 1**, and a ~120s run cannot consume
  4194304 pids. Inside one venue run a freed pid is never reissued.
  Worth keeping anyway, because it prices the guard the code already has:
  `wallWatchKillWith`'s `bwrapPid 0` arm exists for a record that outlives its
  watcher **on the host**, and these numbers say reuse there is a question of
  tens of minutes.

`TestLiveWallUnixScopesSignalsToTheSeat` was also read, because its subject is
signal delivery and its control arm asserts a seat **can** signal a pid outside
itself. It sends `signal 0` only — a liveness probe — and sends `SIGTERM` only
to a child it forked itself. Clean.

**5. `--die-with-parent` + per-task PDEATHSIG + Go thread churn** (probe arm E). A complete
self-contained candidate: pdeathsig is per-**task**, Go forks from whatever OS
thread a goroutine is on, and a `LockOSThread` goroutine that returns has its
thread terminated. It predicts exactly the thing the bead found strange — that
the victim was an *idle* seat, which is what a thread-exit race selects and
what no CPU-burning-leak heuristic would. **Ruled out, measured**: all three arms read `seat ALIVE after 3s`. bwrap sets its pdeathsig on the sandbox child relative to bwrap, not on
itself relative to the forking thread.

**6. Repetition under the live condition.** Re-read as the bead's FIRST STEP
asked, and the condition did not have to be manufactured: the bead's own
sibling seat, `gwart-posse-ranger-base-aurb8`, still held suite slot 1.
**3000 executions** of the two pack-refs pins (`-count=1500`) green in 145.3s
at a one-minute load of ~7, and one full 48-member pass green in 89.6s. No
137. Added to the bead's 700, this class has now gone 3700+ executions without
recurring — which is the other half of why the fix is the verdict and not the
hunt.

## The fix

`internal/posse/bwrapgitrefslive_test.go`. A member FAIL is ADR 0097 D3's
**exit 1**, which D5's ratchet files as one P1 `-l code` **attributed to the
newest landing's persona**. So a seat the box killed arrives as a wall
regression charged to whoever landed last — four times now, and none of those
landings had anything to do with it.

**ADR 0097 already has the right bucket, and it is not red.** A member that
abstains is **exit 3, "no answer"**, which D5 routes to the *watchdog*: filed
on the second consecutive one, `-l architecture`, attributed to nobody,
carrying the tails. D5 says *"an abstention is never a pass"*; this is its
converse, which nothing implemented — **a death that measured nothing was a
red**. So:

- `bgSeatEndedFromOutside(err, out)` — a narrow **conjunction**: the seat said
  nothing (`strings.Fields`, `bgSeatAccount`'s own emptiness test) **and** it
  was ended by a supervisor's signal (bwrap propagated 128+9 or 128+15, or
  bwrap itself was signalled). Everything else still reds. `exit status 128` is
  git's own answer and is one below the range. An in-sandbox fault that kills
  silently stays red too — a seccomp `SECCOMP_RET_KILL_PROCESS` is SIGSYS
  (128+31) and would be the wall's doing.
- `bgSeatAbstain` spends it in one place, as a `t.Skipf` carrying a
  `NOTHING MEASURED:` line. This **invents no mechanism**: the idiom is already
  in the house (`wuOutsideAndAbsent` skips that way over a reaped pid),
  `scripts/posse-wall-check` already counts a member that printed D4's marker
  and then went dark (*"can skip for its own reason … and still turns this
  verdict into exit 3"*), and `bgLiveSkip` prints that marker before any body
  runs. **No ADR changes.**
- Wired into `f.walled` and `packUnderALiveSeat`, not into the five arms that
  call them, because **both polarities are broken** by a silent outside kill:
  `walled`'s refusal arms (`if ok { fail }`) read it as the wall refusing — a
  **false pass** — and its two success arms (`if !ok { fail }`) read it as a
  red. One criterion in one place answers both.

**It discharges a named, open ADR obligation.** ADR 0097's Consequences
records this as still open: *"A pin whose own exec fails for some other reason
still reads as a refusal. That is the pin's ADR 0086 D5 obligation, and nothing
here discharges it."* The `walled` half of this change is that discharge.

And in `packUnderALiveSeat` the abstention is placed **before either arm reads
the result**, which matters more than the red it replaces: T4e's `!ok` branch
logs `MEASURED: the prune took the directory and the seat's commit failed` as a
reading of ADR 0080 D20's reasoning. A seat the box killed would have been
recorded as a kernel measurement this run never took.

Pinned by `TestBgSeatEndedFromOutsideReadsOnlyASupervisorSignalOverSilence`
(the criterion, over real `*exec.ExitError` values from real commands rather
than hand-built `WaitStatus` structs, with every negative row that must keep
redding) and by `TestLiveBwrapGitRefsReadsAnOutsideKillAsNothingMeasured` (the
live half: the real propagation through the **rendered** argv, plus the exit-128
control). The live one **passes** rather than abstaining, deliberately — a
member that always skipped would hold the whole wall check at exit 3 forever.

## What is left open

- **The sender is unnamed**, and from a seat it is unnameable: no signal
  sender is recorded anywhere, oomd's log needs journal group membership this
  seat does not have, and audit/eBPF need privilege. If anyone wants it named,
  it is an operator-side reading (`journalctl -u systemd-oomd` around the
  window, or `auditctl -a exit,always -F arch=b64 -S kill`), not lane work.
  What this bead changes is that it no longer costs a P1 and an innocent
  attribution each time it happens.
- **Only the git-refs fixture is converted.** The other members in ADR 0097's
  set keep their own seat runners, so the same silent kill can still red them.
  Deliberately not widened here: those are other pins, and standing orders file
  adjacent work rather than fixing it silently.

## Handed off

- `loadguardkill.go`'s header reads "SHIPPED OFF … defaults to false" while
  this instance runs `load_guard_kill: true`. Filed — the trap cost this bead
  real time and would cost the next reader of an incident the same.
