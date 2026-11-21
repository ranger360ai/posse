# The git-global snapshot's readers refused a fifo and its writer hung on one (ranger-base-40vo2)

2026-11-21. `GIT_CONFIG_GLOBAL` names a read-only, content-addressed snapshot
under `SeatWallRoot($HOME)` (ranger-base-161t2). ranger-base-r9slg gave
`gitGlobalSnapshotVerify` a shape check — `Lstat` plus `IsRegular`, beside the
0444 mode — so that a symlink or a fifo planted at the snapshot's name degrades
every READER instead of being trusted.

The WRITER never got that arm. `gitGlobalSnapshot` decided whether to rewrite by
comparing bytes with `os.ReadFile(snap)`, before asking what was at that name:

```go
if got, rerr := os.ReadFile(snap); rerr != nil || !bytes.Equal(got, want) {
```

A fifo there opens `O_RDONLY` only once a writer arrives, so that read sat in
the kernel. `PrepareGitGlobalSnapshot` is the LAUNCH path, so the launch never
answered — and `GitGlobalScopeDegraded()`'s line, which exists for exactly this
plant, was never printed, because nothing returned to print it. Of this
family's tampers it was the only one with no line for an operator to read: the
mode-only tamper degraded and healed, the content tamper had always healed,
this one hung.

MEASURED 2026-11-21 (ranger-base-suozl, verifying ranger-base-r9slg), a 0444
fifo at `<SeatWallRoot(HOME)>/git-global/<hash>/read-only-snapshot.gitconfig`:
`gitGlobalSnapshotVerify` answers in microseconds with "is not a regular file";
`PrepareGitGlobalSnapshot` does not answer in 5s.

WHO CAN PLANT IT: not a caged seat — the path is under no caged write grant
(`TestQAGitGlobalSnapshotIsUnderNoCagedWriteGrant`) — but at the shims tier the
whole home is writable by design. Same class and blast radius as the mode-only
tamper: a durability and detection defect in posse's own self-healing, not a
security finding.

## The fix

One function, `gitGlobalSnapshotRead`, between the writer and the bytes. An
error from it means "not the bytes this launch wants", which is the rewrite
arm — so the rename lands over a fifo, a symlink or a device the way it lands
over tampered bytes and the launch HEALS the plant. Where it cannot (a
directory at that name) the rename's error comes back and the caller prints the
degraded line, which is the answer every other unusable snapshot gets. Hanging
is what is ruled out.

Two checks, and they answer different questions:

- `Lstat` + `IsRegular` is the shape, and it is also what refuses a SYMLINK —
  whose target would otherwise be read and could equal `want`, which is the hat
  ranger-base-r9slg took off the verify.
- `O_NONBLOCK` on the open plus an `fstat` of the fd closes the window between
  that `Lstat` and that open. Whoever can plant once can plant in a loop, and a
  window that still hangs the launch is this defect with a smaller mouth.
  `O_NONBLOCK` is inert on a regular file; on a fifo it is what lets the open
  return at all, and the `fstat` is what refuses what it opened.

`gitGlobalSnapshotVerify`'s own `os.ReadFile(snap)` goes through the helper
too, and that is a ONE-LINE widening of the bead, done out loud rather than
left behind. The reader never hung on a planted fifo — its `Lstat` is what
ranger-base-r9slg added and what the row above tests — but the window between
that `Lstat` and that read is the writer's window exactly, in the function two
lines below the one being fixed. It changes no answer: the helper's shape check
is a subset of the arms already there, and an error from it reaches the same
"does not read back as a copy" refusal a short read always did.

## The door, and the one arm that has none

`TestQAAGitGlobalSnapshotLaunchAnswersOverWhatIsNotAFileAtThatName`
(`internal/posse/gitglobalsnapshotmode_qa_test.go`, arm 3) is the writer's half
of the reader's door one test above it. Two rows: a fifo, which must heal, and
a directory, which must come back with the error and the degraded line. Both
go through `ggsAnswered`, for the reason that helper already names — an
unbounded call over the mutant hangs the whole arm-3 binary to its timeout and
prints no test name.

MUTATION RUN, 2026-11-21, this box (Linux 7.2.15-1-omarchy, go1.27.3, 16
cores): restoring `os.ReadFile` in `gitGlobalSnapshot` fails the new test BY
NAME on the fifo row at 10.01s, leaves its directory row green, and leaves
`TestQAAGitGlobalSnapshotVerifyRefusesWhatIsNotAFileAtThatName` and the rest of
that file green — which is the state the tree was in before this landed.

THE `fstat` ARM CARRIES NO DOOR, said here so the next reader does not spend
the derivation: witnessing it needs the path to change BETWEEN the `Lstat` and
the open, and no test can drive that interleaving without a seam in production
code for the test's benefit. The `Lstat` arm is what the fifo row reds. The
second arm is kept because it is three lines and closes a window a local
adversary can grind, not because anything measures it. The same is true of the
helper's use inside `gitGlobalSnapshotVerify`: the planted fifo there is
already doored by the reader's own test, and the raced one is the same
un-drivable interleaving.

## Not closed here

- ranger-base-6zw05
