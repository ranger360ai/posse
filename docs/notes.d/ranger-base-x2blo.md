# The reaper's probe created the inode it was asked to find (ranger-base-x2blo)

2026-11-12, Linux 7.2.15 x86_64, flock from util-linux 2.42.4, from this
bead's worktree. The last live finding of verify bead ranger-base-9rbfq,
over the close of ranger-base-z056i (ADR 0089 D2). The other three findings
of that bundle are carried by ranger-base-w9w37, which has landed, and by
ranger-base-jk6av.

`scripts/reap-test-tmp.sh`'s `flock_filter` is ADR 0089 D2's step 1: the one
gate that is a positive proof of LIFE rather than a failure to find death.
It runs `flock -n -E 9 "$d" true` over every candidate and keeps the dir
unless the lock comes free. The header two decisions above it states what
that is allowed to cost: *"a busy lock can only ever make this script
collect LESS."*

## The hole

`flock(1)` opens its lock path with `O_CREAT`. MEASURED, the four answers
the probe can get:

```
free dir            -> rc=0
busy dir            -> rc=9
mode-000 dir        -> rc=66, "cannot open lock file: Permission denied"
NONEXISTENT path    -> rc=0, and flock CREATED it as -rw-r--r--
```

The first three are exactly the three the `case` claims to distinguish, and
they hold. The fourth is the finding: a candidate directory removed between
the walk and the probe — the `find`/`sort -u` upstream of this function and
the `flock` inside it — was **re-created by the reaper as a 0-byte regular
FILE at that path**, read back as a clean acquisition, and offered as
collectable. In a dry run nothing then removes it, and because every class's
`find` is `-type d`, no later run can ever collect it. The sweep leaked an
inode it made itself, on the quota it exists to protect, and against its own
claim: a busy lock collects less, but a vanished one collected *more* than
there was.

Severity is low and stated as such: the window is small and the artifact is
one 0-byte file.

## The fix

`[ -d "$d" ] || continue` ahead of the probe, so a vanished candidate is
dropped rather than re-created. `-d` and not `-e`: a candidate is a
directory or it is not a candidate. Dropped silently, in neither stream —
there is no dir to keep and nothing for the next reader to act on.

The removal loop at the foot of the script opens with the same `O_CREAT` and
is left alone, because its own `rm -rf` takes back whatever `flock` just
made.

## The arm calls the function directly, and that is why the dispatch moved

The window the guard closes is between the `find` and the probe inside one
pipeline. Nothing a fixture can plant reopens it on demand, so an arm that
tried would be pinning a race's timing. Feeding `flock_filter` the one input
that race produces — a candidate path that is not there — pins the guard
itself: the path is not re-created, it is not offered as collectable, it is
not reported as kept, and (the control, so the guard cannot pass by
rejecting everything) a real free dir still comes through.

That is the only arm in `--self-test` that calls a function of the script
rather than re-invoking `"$0"`, and a shell function has to be *defined*
before it is called, not merely before it is read. So the `--self-test`
dispatch now sits below `flock_filter` instead of up beside the argument
parsing. It passes two preflights on the way, `/proc` and `flock(1)`, which
every arm's `"$0"` sub-invocation needs anyway: a box missing either now
gets one clear line instead of a dozen failing arms.

MEASURED: `make verify-reap-test-tmp` green, 27.2s, 23 arms. Against a copy
of the script with the one guard line removed, the two behavioural arms red
and the "not reported as kept" arm stays green — pre-fix the path is emitted
as collectable rather than recorded, which is the stream that arm exists to
pin.

## Found beside it, filed not fixed

`--self-test`'s ambient `claude_procs` control arm reads a count twice, a
second apart, and asserts the difference is exactly the one process it
planted. On this box every seat is a real `claude` process, so a sibling
seat starting or ending in that second moves the count and reddens the arm —
MEASURED once in two runs here, `12 -> 14, want 13`. The arm is a
prerequisite of `make test`, so the flake is a red suite for whoever is next.
Filed as ranger-base-om4i3, not fixed here: the assertion is the defect, the
production code the two arms cover is correct and was re-verified green.

## Not closed here

- ranger-base-jk6av
