# ADR 0089 — A test home's creator holds a lock on it, and the reaper takes that lock as proof of death

*Status: accepted 2026-11-11 (ranger-base-glogz, from ranger-base-vw1cp) ·
owner: architect · builds on ranger-base-32bdv (one temp HOME per test tree,
`scripts/reap-test-tmp.sh`) and ADR 0011 §1–§2 (flock as liveness; prune
proves death)*

## Context

internal/posse's `TestMain` makes `posse-testhome-*` under `$TMPDIR` and
removes it when `m.Run` returns. The bead asked about two kinds of leftover
dir. ranger-base-32bdv landed an answer for both an hour before this bead was
picked up (commit 8fd56c6b), so this ADR reviews that answer. It does not
redesign it.

- **Shape (a), re-exec'd children.** Fixed by 32bdv. A child adopts its
  parent's home through `RHQ_TEST_HOME`, and only the creator removes the
  dir (`herdr_test.go`, `ownHome`). The hermeticity question the bead raised
  is answered there: the shared credential fixture was the one race, and the
  `ownHome` guard is the whole of its fix. `testhome_qa_test.go` pins both.
  Nothing more is needed here.
- **Shape (b), a parent killed by SIGKILL or a `-timeout` panic.** Nothing
  inside the process can clean up after either, so collection has to come
  from outside. 32bdv shipped `scripts/reap-test-tmp.sh`. It removes a dir
  only when two things hold. First, no live process names the dir through
  `HOME`/`TMPDIR`/`RHQ_HOME`/`RHQ_QA_HOLD_HOME` in its environ, through its
  cwd, or through an open fd. Second, the dir's mtime is older than
  `--min-age` (default 60m). This is the field's standard shape
  (safe-reclamation: a hazard-pointer scan plus an RCU grace period).

**The scan cannot see the process that owns the dir.** MEASURED 2026-11-11,
linux 7.2.15, go1.27.3, using a 20-line probe that does what TestMain does
(`MkdirTemp`, then `os.Setenv("HOME", dir)`, then sleep):

- `/proc/<pid>/environ` still reads `HOME=/home/posse`. It is the environment
  the process was exec'd with, and Go's `os.Setenv` never rewrites it. Only
  CHILDREN exec'd after the Setenv carry the temp path.
- The probe holds no fd on the dir, and its cwd is the package dir.
- Running `reap-test-tmp.sh --root <scratch>` (dry run) against the probe
  while it was alive, with the dir's mtime set back two hours, printed
  `live temp HOMEs seen: 0` and `collectable: 1 dirs`.

So the creator is protected only by the 60-minute grace, and only while some
child of it happens to be running. That grace is measured on the dir's own
mtime, which changes only when a top-level entry is added. For a long run it
stays near the start time (ASSUMED, not measured over a full arm).
`coldexec-check` arms carry `-timeout 60m`, and ranger-base-in8dr priced one
doored arm at 26 minutes on a quiet box. A loaded box runs 2–3x slower. A run
older than the grace that has no child alive at that moment is in the sweep
set. The script defaults to a dry run, and the person who runs `--apply` is
the person who removes a live run's HOME.

**Namespaces, so the lock works where the scan does.** L3 cages share the
host's pid namespace and `/tmp` (`bwrap.go` leaves out `--unshare-pid` and
`--tmpfs /tmp` on purpose). A flock is held on the inode, so it is visible
from any namespace that sees the same dir. A container seat's home lives in
the container's own `/tmp`, not the host's, so the host reaper never sees it
as a candidate (ASSUMED from the L4 mount list, not probed).

## Decision

**D1. The creator holds an exclusive `flock` on its temp home for its whole
lifetime.** Right after `MkdirTemp`, and only when `ownHome` is true,
`TestMain` opens the directory and calls `flock(LOCK_EX|LOCK_NB)` on it. It
keeps that fd open until the process exits, which is after the
`RemoveAll`. The kernel releases the lock on any death, SIGKILL and a
watchdog panic included. The release IS the death, so the lock cannot go
stale (ADR 0011 §1's argument, unchanged). The lock is on the directory fd,
not on a file inside it, so the home's contents do not change and no test
that reads `$HOME` sees anything new. Go opens files with `O_CLOEXEC`, so
children do not inherit the fd. They are covered by D2's scan. A flock
error is printed once on stderr and is not fatal. The dir is then exactly as
protected as it is today: by the scan and the grace.

MEASURED 2026-11-11 (same probe, with the lock): `flock -n <dir> true` from
outside the process reports busy while the probe is alive, and acquires the
lock within 0.5s of `kill -9`.

**D2. The reaper requires all three conditions, and takes the lock first.**
For each candidate older than `--min-age`:

1. `flock -n` on the dir. If it is busy, the dir is HELD by its live creator,
   and the scan is not needed.
2. Run the existing `/proc` scan, unchanged. A descendant left alive after
   its parent was SIGKILLed still names the dir in its environ, and the lock
   cannot see it.
3. The existing grace, unchanged. It covers the microseconds between
   `MkdirTemp` and the `flock`, and it covers dirs made by binaries built
   before D1, which hold no lock. Those cannot be told from a dead creator,
   so the grace is still their only protection.

The `rm -rf` runs while the reaper still holds the lock it acquired (for
example `flock -n "$d" rm -rf -- "$d"`). A creator never reopens its dir by
path, so holding the lock during removal costs nothing. It also closes the
window between the check and the delete for any later process that
announces itself by taking the lock.

**D3. The reaper's default and its owner do not change.** It is still a dry
run by default, still run by the operator, and still linux-only (`/proc`).
D1 makes `--apply` safe to run while suites are in flight. Whether anything
should run it automatically is a separate question, and it belongs to the
operator (ranger-base-awnb2).

## Consequences

- The case where a live creator is collected is closed for every binary built
  after D1. It stays open for binaries built before D1 that are still
  running, until they finish, which is at most one `-timeout` later.
- One extra fd stays open per test-tree root process. No test counts the
  test process's own fds (grep for `/proc/self/fd`/`NumFD`, 2026-11-11).
- On darwin, a flock on a directory fd is ASSUMED to work, because BSD flock
  locks the vnode, and this was not probed. If it fails, D1 prints its line
  and the dir is protected as it is today. The reaper does not run on darwin.

## Alternatives rejected

- **A stamp file holding pid, worktree and start time** (the bead's own
  suggestion). This is a pidfile. A file that names a process outlives that
  process, and the pid it names gets reused (liveness-and-identity). Making
  it trustworthy would need the identity check as well: compare the pid's
  start time and argv with what the stamp recorded. That is a staleness layer
  rebuilt by hand, when flock already gives the kernel's answer. A worktree
  name would help a person read the dir, but it adds no safety. The scan
  already names the live holders, and nothing reads that field to make a
  decision.
- **Keep an fd open on the dir without a lock, so the existing fd scan sees
  the creator.** This is the zero-mechanism version, and I wanted it. It
  works only where `/proc/<pid>/fd` can be read: same uid, and a reaper that
  can see the creator's pid namespace. The reaper's blindness check exists
  precisely because that cannot be assumed. Taking the lock costs one more
  syscall on the same fd and makes the answer independent of `/proc`.
- **Re-exec `os.Setenv` into the visible environ** (re-exec self with the temp
  HOME, so `/proc/<pid>/environ` names it). This doubles TestMain's startup
  and puts a liveness signal in the wrong place. The environ is a snapshot
  taken at exec, not a held resource.
- **A longer grace (say 6h) instead of a lock.** This narrows the window and
  does not close it, because no number bounds a run whose `-timeout` the
  caller picks. Each hour of grace also leaves another hour of genuinely dead
  homes on a quota every seat shares, which is the cost ranger-base-vw1cp
  measured.
- **A signal handler or `t.Cleanup` in the parent.** SIGKILL cannot be
  caught, and a `-timeout` panic exits from the watchdog goroutine. Both
  were MEASURED by 32bdv.
- **A per-run `TMPDIR` that the wrapper (`test-times.sh`) removes.** This
  covers only runs launched through the wrapper. A bare `go test` is 25% of
  suite entries (AGENTS.md, suite-entry census), and those leak exactly as
  before. ranger-base-akk9f showed that moving TMPDIR off `/tmp` reds 12
  tests in 4 packages. Reject until that class is fixed.

Implementation: ranger-base-z056i (dinesh), D1 and D2 in one bead.

## Exit hatch

D1 is about a dozen lines in `TestMain` plus one `flock -n` in a shell
script. It holds no state hostage: delete both and the reaper is back to
today's scan-plus-grace. If Go ever gives a temp dir a lifetime tied to the
process (it has no such API today), D1 goes and D2's step 1 goes with it.
