# ranger-base-orgvt — an orphaned descendant's execs, and what closing them cost

The hole ranger-base-in8dr's two new arms exposed, its fix, the two prices
the fix carries, and the one route that looked right and was measured wrong.
ADR 0085 D5 carries the decision; this is the measurement.

## The hole

`cmd/coldexec`'s supervisor reads the path argument of every stopped
`execve` out of `/proc/<pid>/mem`. That needs `PTRACE_MODE_ATTACH`, and
under `yama/ptrace_scope=1` — this host — it is granted to an **ancestor**
and to nothing else. A descendant whose own parent exited is reparented
past the supervisor, so it stops being a descendant and the read is
refused. Its exec is then recorded `unreadable` and let through
un-adjudicated: **a plant such a process execs is never delayed**, and the
test that planted it is not covered at all, with nothing in that test's own
output to say so.

MEASURED 2026-11-11, Linux 7.2.15 x86_64, `ptrace_scope=1`, the two arms
in8dr added:

| arm | notifications | un-adjudicated | |
|---|---|---|---|
| internal/treepins | 29744 | 145 (134 unreadable, 11 gone) | 0.49% |
| cmd/posse | 8427 | 0 | — |

**134 of that 145 is this edge, not all of it.** ranger-base-j0riv landed
between the measurement and this fix and retired the `gone` verdict
altogether — an absent pid is a guess, `ID_VALID` is the authority, and a
notification the kernel withdrew is not a hole — so the other 11 were never
this and are no longer counted as holes by anything. Read the arm's
remaining holes as 134, all unreadable, all of them orphans.

Every unreadable one is `open /proc/<pid>/mem: permission denied`. The
cause is parentage and not the plant: in treepins they cluster in the
suite-lock self-tests, which orphan holder processes on purpose. cmd/posse's
tests do not orphan, which is why its arm reads 0 and why the class looked
closed until in8dr widened the door.

Minimal repro, two identical fresh plants under
`./bin/coldexec -delay 500ms -verbose`:

```
a child still in the supervisor's tree execs its plant → delayed     503ms
a child whose parent exited execs its plant            → unreadable,   3ms
```

The reparent target here is not pid 1. MEASURED: `systemd --user`, pid
1140 — a systemd user session sets itself as a subreaper, so the orphan
lands there. Either way it is outside the reach.

## The route that does not work

`prctl(PR_SET_PTRACER, <supervisor pid>)` in `coldexec -child`, before the
filter goes in. It looks like the smaller change — it leaves the process
tree entirely alone. MEASURED 2026-11-11 on the repro above: the prctl
itself succeeds and **nothing changes**, 5 unreadable of 5. yama's
exception is per-task: it admits the declared tracer for the declaring task
and does not extend to that task's descendants, and the exec that is
refused belongs to the orphan's own child. There is no way to run a prctl
in an arbitrary descendant, so this route cannot be rescued.

## The fix, and its two prices

`prctl(PR_SET_CHILD_SUBREAPER, 1)` on the supervisor, before it forks. An
orphaned descendant is then reparented **to** the supervisor rather than
past it, so it stays a descendant and its execs are adjudicated like any
other's. MEASURED on the repro: the orphan's plant goes from 3ms and
`unreadable` to 504ms and `delayed`, and the run's holes go to 0. It is
per-process (`signal_struct`), so no locked thread; `is_child_subreaper` is
not copied to children, only the `has_child_subreaper` hint, so every
orphan below lands on the supervisor and not on an intermediate shell.

The bead's ASSUMED estimate was "that is one prctl". It is not — it carries
two prices, and the first is a regression if it is left unpaid.

**Price 1, paid: zombies.** Being the subreaper makes the supervisor the
only process that can wait on those orphans. An orphan that exits
unwaited-on stays a zombie: it answers `kill -0`, it holds a pid, it shows
in `ps`. That is not cosmetic here — `kill -0` is this tree's own liveness
predicate (`internal/posse/codextrust_test.go` reads ESRCH as "ended and
reaped" and anything else as a leaked zombie; queuecutover's guard is the
same call). MEASURED with the prctl and no reaper: 20 orphans that exec
`/bin/true` and exit leave 20 `Z (defunct)` children on the supervisor, and
on the pin's script the orphan's pid still answered `kill -0` ten seconds
after it exited. `reapOrphans` closes it — a 200ms tick that reads
`/proc/self/task/*/children` and `wait4(pid, WNOHANG)` each one, skipping
the supervised child, whose status belongs to `os/exec`.

A poll and not a `SIGCHLD` handler, deliberately: `os/exec` is already
waiting on the supervised child with its own pidfd, and a signal-driven
reaper calling `wait4(-1)` would race it for that status and break the one
thing ARM 9 pins — the command's own exit code reaching `make`. A tick that
skips one pid by name cannot.

Which thread adopts them: MEASURED, 6 of 6 landed on the group leader. The
kernel's `find_new_reaper` walks the thread group from its leader and a Go
process's leader does not exit, so that is expected to hold. The all-task
glob is the read that does not depend on it, at the cost of a few extra
reads of `/proc/self`. Nothing pins the difference — a belt, not a claim.

**Price 2, stated and not paid: parentage.** Under the door an orphaned
descendant's ppid is **the supervisor's and not 1**. `Orphaned()` in
internal/posse is `PPID == 1`, so a test that plants a real orphan and
reads its parentage back sees the door and not itself. Nothing in the three
arms does that today: `orphancontrol_test.go` is the one place that asserts
ppid 1 on a planted orphan, and it runs only inside
`scripts/verify-orphan-report.sh`'s throwaway container, skipped on this
box; `go run ./cmd/checkorphans` runs after the door has exited, so its
ppid-1 reading is untouched. The trade is knowing: one visible, enumerated
change in parentage against 145 silent un-adjudicated execs. It is a new
line in ADR 0085 D5's edge list, in the half that says what the door
CHANGES rather than what it fails to cover.

## The pin

`TestAnOrphanedDescendantsPlantPaysDAndLeavesNoZombie`, ARM 15 of
`cmd/coldexec/supervisor_linux_test.go`. One supervised `sh` plants two
identical executables, execs one from a child that stays in the tree and
one from a child whose parent exits, then waits for that orphan's pid to
stop answering `kill -0`. It asserts the arm is set up (the orphan's ppid
IS the supervisor's, so it really was reparented and really did land in
reach), both plants cold, 2 delayed, no holes, and the orphan reaped.

The kept half is not decoration: it separates this pin's red from a door
that has stopped delaying anything at all. A run where neither child pays D
is ARM 1's failure; a run where only the orphan does not is this one.

Three mutants, each red on its own message (MEASURED 2026-11-11):

| mutant | what reds |
|---|---|
| `PR_SET_CHILD_SUBREAPER` removed | `the orphan's ppid is 1140 and the supervisor is …` |
| `go dr.reapOrphans(…)` removed | `still answered kill -0 after it exited` + the missing report line |
| the glob narrowed to `/proc/self/children` | the same two (that path does not exist, so the scan finds nothing) |
| the `pid == childPid` skip removed | **nothing** — ARM 9 and ARM 15 both stay green |

The third is a witness for the path spelling and not for the all-task scan,
which is unpinned on purpose — see above.

The fourth is the honest result and it is worth writing down: the skip that
keeps the supervised child's status with `os/exec` is a GUARD and not a
pinned behaviour. `os/exec` is already blocked in its own wait when the
child exits, so the window in which a 200ms tick could take that status
first is microseconds wide and did not open in any run here. The skip stays
— the door's forwarded exit code should not rest on `os/exec` winning a race
nothing told it about — but no pin defends it, and the first draft of
`reapTick`'s comment claimed it would break ARM 9, which this measurement
contradicts.

## The merge

This landed on a base that did not have ranger-base-in8dr (the two new arms
that produced the measurement) or ranger-base-j0riv (which restructured
`adjudicate`/`answer` in the same file). Merged `main` in rather than
leaving the launcher a non-fast-forward: `door_linux.go` and `doc.go`
auto-merged, and the two conflicts were both "we each appended at the end" —
ADR 0085 D5's edge bullet, where in8dr had filed this edge as OPEN and named
this bead as its resolution, and `supervisor_linux_test.go`, where j0riv's
arm took the number 14 first, so this one is ARM 15.

## End to end, on the arm where the holes were

The repro is a repro. This is the real door over the real tests the 134
clustered in — `internal/treepins`, `-run SuiteLock`, `D = 300ms` — run
twice on the same box minutes apart, once with the door `main` ships at
ef842dbe and once with this change (MEASURED 2026-11-11):

| door | notifications | un-adjudicated | orphans reaped | delayed | tests |
|---|---|---|---|---|---|
| main, pre-fix | 1790 | **6 unreadable** | — | 7 | ok, 12.059s |
| this change | 1816 | **0** | **31** | 7 | ok, 12.302s |

Six of them, not 134, because this is one `-run` filter and not the whole
arm — the 134 is the arm entire. What the two rows show is the mechanism
end to end at the scale it was found: the un-adjudicated execs go to zero,
31 orphans really are adopted and waited on, the suite still passes, and
the wall time is inside the run-to-run noise.

Worth saying plainly: the delayed count is **7 in both**. Closing a blind
spot is not the same as finding a plant in it — once those six execs became
readable, none of them turned out to be a plant. The value is that the door
now KNOWS that, where before it could not tell those six from six missed
injections, and the suite-lock tests it could not see into are the ones
whose own fixtures are the orphans.

## Not measured

The full `make coldexec-check` was not re-run: it is five arms now, each
taking a suite slot, and ~851s for the three internal/posse ones alone
(ranger-base-kiwvn's price, for the content rewrite). Nothing here changes
what is delayed except by ADDING the orphans that used to escape, and the
filtered run above is the shape of that. What a full arm costs in wall time
under the fix, and whether the treepins arm's 134 go to 0 at full scale
rather than at this filter's 6, is unmeasured.

`make tree-check` and `make test` could not be read green on this branch,
and not because of this change: `TestQAColdexecArmRegisterIsNotStale` is
red on `main` itself (reproduced from a clean `git archive main`), because
ranger-base-in8dr added the cmd/posse and internal/treepins arms and left
both packages' `cxaUncoveredPlanters` waivers in place. Filed as
ranger-base-8vk23, not fixed here. Every other door passes, and so do the
other 33 of `corpus-check`'s 34 pins.
