# The creator's lock, and the reaper that reads it as a death certificate (ranger-base-z056i)

2026-11-11, Linux 7.2.15 x86_64, go1.27.3, 16 cores, from this bead's
worktree. Implements ADR 0089 D1-D2 (from ranger-base-glogz, from
ranger-base-vw1cp). Builds on ranger-base-32bdv, which gave internal/posse's
test tree one temp HOME and shipped `scripts/reap-test-tmp.sh`.

The hole: the reaper could not see the one process that owns a temp HOME. Go's
`os.Setenv` does not rewrite `/proc/<pid>/environ`, so the `HOME=` the scan
reads off a live test binary is still the operator's; the binary's cwd is the
package dir; and it held no fd on the dir either. The only thing standing
between a live run's `$HOME` and an operator's `--apply` was the 60-minute
grace — and `coldexec-check`'s arms carry `-timeout 60m`, so a loaded box
outlives it.

The fix is ADR 0011 §1's argument one directory over: the creator holds an
exclusive `flock` on the dir for its whole life, the kernel releases it on
SIGKILL and on a `-timeout` panic alike, and the reaper takes that lock as
proof of death. Release *is* death, so there is no staleness class to detect.

## What landed

- **D1**, `internal/posse/herdr_test.go`: `lockTestHome` opens the dir
  `MkdirTemp` just returned and takes `LOCK_EX|LOCK_NB` on the directory fd.
  It is the next statement after `MkdirTemp`, inside the `ownHome` branch —
  an adopting re-exec child takes no lock, because the dir is not its own and
  the parent's lock is what it would collide with. The `*os.File` is held in a
  package var (`testHomeLock`), not a local: `os.File` carries a finalizer
  that closes the fd when the value becomes unreachable, and a local that
  nothing reads again is unreachable from the moment `m.Run` is called — the
  lock would then be dropped by a GC cycle in the middle of the run instead of
  at death. Every failure is one stderr line and no exit.
- **D2**, `scripts/reap-test-tmp.sh`: `flock_filter` runs `flock -n -E 9 "$d"
  true` over every candidate ahead of the `/proc` held-set filter. Busy (9)
  and unopenable (anything else) both keep the dir and are reported with the
  reason; only a clean acquisition lets a dir through. `--apply` removes each
  dir under a freshly taken lock (`flock -n -E 9 "$d" rm -rf -- "${d:?}"`),
  which closes the window between the walk and the delete, and says so per dir
  when a creator claimed one in between. The script now refuses outright
  (`exit 2`) when `flock(1)` is not on PATH rather than falling back to the two
  gates this bead was filed to stop relying on.
- **The pins**, `internal/posse/testhome_qa_test.go`:
  `TestTheCreatorHoldsAFlockOnItsTempHome` (D1, from outside the process, which
  is the reaper's only vantage) and
  `TestTheReaperKeepsALiveTempHomeAndTakesADeadOne` (D1+D2 round trip against
  the real script, on both sides of a `kill -9`).

## The before and after, MEASURED

Two arms of one run (`measure.sh`, this bead's scratchpad): arm A is the
pre-D1 shape — a dir owned by a live process that holds no lock and names it
in no environ, no argv, no cwd and no fd — and arm B is the shape as it ships.
Both dirs aged 3h with `touch -d`, both swept with `--root <arm's root>
--class posse-testhome --min-age 60`, dry run.

**ARM A — pre-D1 (a live owner, no lock).** This reproduces ADR 0089's own
measurement:

```
same-uid processes: 59   environs readable: 56   candidates a live process names: 0
class posse-testhome (posse-testhome-*, grace 60m, depth 1): 1 collectable
collectable: 1 dirs, 0M, 1 inodes (roots: …/rootA)
```

A live owner, and the sweep offers its dir for collection. Nothing in that
output looks wrong, which is the whole defect.

**ARM B — as it ships, while the creator lives.** `flock -n` probe: exit 9
(busy).

```
same-uid processes: 59   environs readable: 56   candidates a live process names: 1
  HELD …/rootB/posse-testhome-2630403472
class posse-testhome: 1 dir(s) kept by ADR 0089 D2 step 1 — a live creator's own flock, so the scan was not needed
  HELD (lock) …/rootB/posse-testhome-2630403472
class posse-testhome (posse-testhome-*, grace 60m, depth 1): 0 collectable
nothing to collect (roots: …/rootB)
```

**ARM B — after `kill -9`.** `flock -n` probe: exit 0, within one 0.5s poll.

```
same-uid processes: 58   environs readable: 55   candidates a live process names: 0
class posse-testhome (posse-testhome-*, grace 60m, depth 1): 1 collectable
collectable: 1 dirs, 1M, 3 inodes (roots: …/rootB)
```

and `--apply` over the same set:

```
collected 1 of 1 dirs (claimed since the walk: 0, failed: 0)
dir still there: no
```

So the mechanism reads a live run as live and a SIGKILLed one as collectable,
with no grace doing the work in either direction.

## The finding: D1's lock fd gives the rejected alternative for free

ADR 0089 rejected "keep an fd open on the dir without a lock, so the existing
fd scan sees the creator" because it works only where `/proc/<pid>/fd` can be
read. Taking the lock requires an open fd, and that fd is visible in the
holder's own `/proc/<pid>/fd` — so the scan arm the ADR declined now answers
too, as a side effect of the arm it chose. MEASURED: `flock -n -F <dir> sleep`
shows `3 -> <dir>` in `/proc/<pid>/fd`, and arm B above reads `candidates a
live process names: 1` where arm A reads 0.

This changes nothing about the decision — the environ arm is still blind and
the fd arm is still conditional on a readable `/proc`, which is exactly why
the lock and not the fd is the mechanism — but it does mean posse-testhome-*
now has two signals where it had none. The round-trip pin logs which ones
fired and asserts only the lock, in both directions, because a cage that masks
`/proc` takes the second away and must not red a pin whose subject is the
first.

## No build tag, and the check that says so

ADR 0089 D1 asked for a windows build to be ruled out before adding a
`GOOS` tag split for `syscall.Flock`. It is already ruled out, twice:
`internal/posse/launchlock.go` (a NON-test file, so every build of the
package) calls `syscall.Flock` with no constraint of any kind, and `ci.yml`'s
matrix is `[ubuntu-latest, macos-latest]`. A tag here would be the only one of
its kind in the package and would protect a build nobody makes. The pins skip
on `runtime.GOOS != "linux"` instead — the reaper reads `/proc` and ADR 0089
D3 leaves it linux-only — so the macOS legs skip rather than fail, and D1
itself still runs there (BSD `flock` locks the vnode; ADR 0089's consequences
record that as ASSUMED and unprobed).

## Mutation-checked, and one of the two pins had to be rewritten to pass it

- Deleting TestMain's `lockTestHome` call reds the round trip ("made … and did
  not lock it") and — on the first draft — left
  `TestTheCreatorHoldsAFlockOnItsTempHome` **SKIPPING green**. The draft keyed
  its adopting-child skip on `testHomeLock == nil`, which is nil for an
  adopting child *and* for a creator whose flock failed: the one case the pin
  exists for read as the one case it is meant to skip. Fixed by keeping
  TestMain's `ownHome` in its own package var (`testHomeOwned`) and skipping on
  that; both pins now red under the same mutation.
- Removing `flock_filter` from the candidate pipeline reds the round trip on
  the missing `HELD (lock)` line — and **not** on the collectable count, because
  the `/proc` fd arm above still keeps the dir. The two assertions failing
  apart is the point: the count alone would have gone green over a deleted D2.

## Not closed here

- A binary BUILT BEFORE D1 and still running holds no lock, cannot be told
  from a dead creator, and is protected by the grace alone. It closes itself
  at most one `-timeout` after this lands.
- No other class in the reaper's table has a creator we can make take a lock
  (ADR 0089 D2 step 3), so for them the grace is still the gate. The
  `flock_filter` step runs over every class anyway, because a busy lock can
  only ever make the sweep collect less, and a future creator that announces
  itself the same way is then covered without an edit.
