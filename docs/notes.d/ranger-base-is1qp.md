# The dispatch loops on this binary (ranger-base-is1qp)

The decision is ADR 0092 D8; the mechanism is `internal/posse/boxloops.go`
with `boxloops_linux.go` / `boxloops_other.go` and the pins in
`boxloops_qa_test.go`. This page holds the measurement it rests on and the
alternatives that were weighed and dropped, which do not belong in either.

## The question, as it arrived

`posse upgrade` renames one file, `~/.local/bin/posse`, and rename(2) leaves
every running process on its old inode (ADR 0092 D3.5). That file is box-wide:
every `RHQ_HOME`'s `dispatch --watch` loop execs it, and the loop is what
RENDERS a launch. Every reader of that fact was per-instance —
`ReadWatchImage(a *App)` asks ONE instance's watch lock and reads ONE
instance's `dispatch-watch.pid`, ShopCheck's G18 arm runs per instance, and
the verb runs in the delegate's instance only. So after an install, nothing on
this box said a SIBLING instance's loop was on the old inode.

ranger-base-nqaa1 fixed the READING OF THE REMEDY — `watchScopeSentence`, on
every branch of both renderings — and deliberately did not widen the reading,
because widening meant coining a notion of "the instances on this box" that no
shipped code in this tree had.

## What was measured

2026-11-21, this box, linux 7.2.15-1-omarchy, from a gilfoyle seat at cage
shims, uid 1001. `~/.local/bin/posse` at dev:ino 54:14541392. Walking `/proc`
for every pid whose `exe` link resolves to that path found three:

```
PID       PPID     DEV:INO        DELETED  RHQ_HOME        RECORD   VERDICT
1185112   1140     54:14541392    no       ~/homes/weller  1185112  LOOP
1185185   1140     54:14541392    no       ~/homes/ranger  1185185  LOOP
3285561   3285542  54:14541392    no       ~/homes/ranger  1185185  not a loop
```

argv, for the record and never as a key: the first two are `posse dispatch
--watch 3m -n 2 --resume` and `posse dispatch --watch 5m -n 6 --resume`; the
third is `posse pulse wait --after 494`.

Sibling `exe` readlinks, their stats, their `environ` and the sibling
instance's `state/dispatch-watch.pid` were ALL readable from that seat.

**What moved since the bead was filed**, and it is worth saying because the
bead's own description carries the older numbers: the install inode and both
pids (54:12591441, pids 2066610 and 1991667). An install landed and both loops
restarted at 12:11:54Z onto the new inode. So neither loop was stale when the
box was read, and what this measurement establishes is READABILITY — which is
what the mechanism needs. **A live stale sibling is still unmeasured.** The
fixture pin (`theMeasuredBox`) holds this exact arrangement with the weller
loop left on the pre-install inode, which is the state the census exists to
report; the live arm (`TestLiveBoxCensusReadsThisBox`) holds that the
arrangement is readable, and logs the table so a suite run carries the
measurement rather than this page's word for it.

Reproduce it: `scripts/` holds nothing for this, on purpose — the live arm is
the reproduction, and it runs from `make test`. For a reading outside a suite,
`go test -run TestLiveBoxCensusReadsThisBox ./internal/posse/ -v`.

The class the third row belongs to is recurrent, not a one-off: two separate
runs of the live arm, minutes apart, each found a different transient `posse`
child of a seat under the ranger instance (pids 3285561 and 594507). So the
filter is exercised on this box on essentially every reading, which is the
best thing that can be said about a filter.

## The one finding that shaped the mechanism

**The third row is a live false positive, not a hypothetical one.** A seat's
own `posse pulse wait --after 494` child runs the same image, from the same
path, under the same `RHQ_HOME` as the ranger loop. Keyed on the exe link
alone — which is the only key the census has for finding candidates at all —
it is indistinguishable from a dispatch loop.

What separates them is the per-instance record, read as a FILTER rather than
as an enumerator: the process names its own instance from its `environ`, and
that instance's `dispatch-watch.pid` either names the process back or does
not. Enumerating records is what needs a homes root; filtering pids `/proc`
already produced does not. That inversion is the whole of why this needed no
config key.

## Why "the instances on this box" was refused

The bead asked for that notion and the answer is that it is the wrong
derivation, not merely an absent one.

- `RHQ_HOME` is an environment variable. Nothing constrains it to live under
  one root, so a declared-instance enumeration UNDER-reports a loop whose home
  is elsewhere — which is exactly the hole.
- It OVER-reports: an instance running no loop has nothing a rename could have
  left behind. `~/homes` holds `ranger` and `weller` today, and only one of
  them has pulse state at all.
- It needs a config key the operator would have to maintain for a reading to
  stay correct, which is a worse trade than deriving the set from the file the
  install actually renames.

## Alternatives weighed and dropped

- **Reuse `sysProcTable`** (loadguard.go), which already walks the box. It is
  two bounded `ps` reads giving pid, ppid, %CPU, etime, comm and argv. `ps`
  cannot report the (dev, inode) of a process's image at all, and argv is the
  one key AGENTS.md ("Ending anything") forbids as a target key because every
  seat's is byte-identical. So the census reads `/proc` directly and forks
  nothing — which is also what lets it run from a caged seat.
- **Make it a G-row (box-wide G18, or a G19).** Dropped on coherence and not
  on cost. Every G-row is per-instance, and a ShopCheck tick runs per
  instance, so a box-wide row would be raised once per instance that happens
  to have pulse state and silent in every instance that has none — one
  condition, both duplicated and missed, depending on which instances tick.
  `posse upgrade` is the one command whose subject is box-wide because it is
  the one that does the rename.
- **Signal 0 for liveness** instead of "`/proc/<pid>` is there". Under
  `LANDLOCK_SCOPE_SIGNAL` a caged seat's signal 0 to a process it did not fork
  is EPERM whether that process is alive or dead (AGENTS.md,
  ranger-base-e5un1), so it would read wrong from exactly the seats that most
  need the census.
- **A darwin route.** The process table `cmd/checkorphans` reads through
  `sysctl kern.proc.all` carries comm and argv and neither an image inode nor
  an environment. A real route needs `proc_pidinfo`'s PROC_PIDVNODEPATHINFO
  and a `KERN_PROCARGS2` parse, neither measured from inside a seatbelt
  sandbox. `boxloops_other.go` refuses and says why, on
  `procstart_other.go`'s rule — an abstention costs a line in a report, while
  a census compared against an unmeasured spelling passes for the wrong
  reason.
- **Replace the per-instance line with this one.** Dropped: the per-instance
  reading is STRONGER for the one loop it reads — a lock the kernel holds,
  against a pid record a process names itself by — and it is the loop the
  delegate's next seat comes through. Both print, each stating its own reach.

## What this reader touches that a security reader should know about

`boxHomeOf` reads `/proc/<pid>/environ` — another process's whole
environment, which on this box carries credentials. One variable leaves that
function and it is a path (`RHQ_HOME`); the return type is one string rather
than a map on purpose, the raw bytes are never logged, rendered or attached to
a `BoxProc`, and the census line an operator pastes on an install bead cannot
carry them. The alternative keys were worse on this axis, not better: argv is
forbidden for a different reason and is equally sensitive, and there is no
other way for a process to tell a reader which instance it belongs to. Filed
for hoover's read on its own bead rather than asserted here.

## Two things that bit during the build, both worth keeping

**The reader is a subject of its own census.** `posse upgrade` runs FROM the
path it replaces, so the verb's own process is a candidate by the census's own
key, and after its swap its own image is the old inode. Unskipped, every
single run would report the reader as a stale process on this binary — true,
and useless, because it is about to exit and renders no launch. `boxProbe.Self`
is the skip, and it is deliberately narrow: a transient CHILD of the verb is
still reported, among the unattributed, because a reader that silently drops
processes it thinks it recognises is the shape this file is a correction of.

**A path that holds nothing had to be called stale.** The first predicate
here read `Image != "" && onDisk != "" && Image != onDisk` — which looks
careful and is the wrong answer, because it makes an absent file "nothing to
compare against" rather than "not the image this loop runs". `WatchImage.Stale`
had it right all along (`Image != "" && Image != OnDisk`), and the divergence
would have surfaced as the one sentence this reading must never print: "all
already on the installed image", over a path with no installed image on it.
Mid-swap and the `install(1)` unlink/write window (ADR 0092 Context) are the
states that reach it. Both predicates are now the same shape, a pin asserts
the census's answer and `WatchImage.Stale`'s answer over the same facts, and
the rendering says the path is empty.

**`os.UserHomeDir()` in a live arm of internal/posse skips silently.** This
package's `TestMain` replaces `HOME` with a per-tree temp home
(herdr_test.go), so the obvious spelling found no installed posse and the live
arm skipped with a message that read exactly like a CI runner's — on the one
box the arm exists for, on its first run. `operatorHome` is the home this
binary started with and is what `ciwatch_live_test.go` reaches for, for the
same reason. A live arm in this package that looks for anything in the real
home has to use it.
