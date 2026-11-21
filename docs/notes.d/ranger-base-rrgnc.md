# An install does not reach a running dispatch loop: G18 (ranger-base-rrgnc)

`posse upgrade` swaps the binary by rename(2) (ADR 0092 D3.5), and rename
leaves every running process on its old inode. The `dispatch --watch` loop is
such a process and it renders every launch, so after an install the file on
disk has the fixes and every relaunch through that loop renders the old walls.
G11 cannot see it: it reads the on-disk binary, and clears when the install
lands, which is the moment this gap opens.

## The evidence

- MEASURED 2026-11-20 (hoover, ranger-base-cye32): a launch six minutes after
  the edfe7a79..3c0e3c5e install read `rendered by posse 0.5.3+edfe7a79`.
- MEASURED 2026-11-21 (gilfoyle, ranger-base-76esp): after
  3c0e3c5e..0e1cb6c0, `/proc/<pid>/exe` of both loops read `(deleted)`, and a
  fresh seat still carried ranger-base-hgcmw's pre-fix `--add-dir` line.
- MEASURED 2026-11-21 00:25 EST, this box, with this change: the live loop
  (restarted 00:08:52) and `~/.local/bin/posse` are both dev:ino
  54:12591441, and `posse status` from this tree raises no G18. That is the
  live negative arm; the positive arm is the pin below, because making one
  live means leaving a fleet loop stale on purpose.

## What changed

- `internal/posse/watchimage.go`: the reading. On linux it stats
  `/proc/<pid>/exe` (the image, even unlinked) against the path that link
  names, so it reads loops started before it existed. Elsewhere it reads the
  image identity, path and version the loop now stamps into
  `dispatch-watch.pid` at start; an unstamped loop there is an abstention.
- **G18** in `ShopCheck`, LANE: `dispatcher pid N runs <old>; <path> is <new>;
  launches render the old walls until this loop restarts — restart the
  dispatch loop, then relaunch seats`.
- G18 is an install subject in `upgradeConditionSource`: the rename creates it
  by construction, so as binary-rendered it would roll back every good
  install with a loop running.
- `posse upgrade` prints the same sentence on the line before D6's.
- ADR 0092 D6 amended: the remedy is restart the loops, then relaunch seats.
  Restarting a live dispatcher stays the operator's or coordinator's
  (guardrail 3). Nothing here restarts anything.

## Pins

`internal/posse/watchimage_qa_test.go`: a real process exec'd from a file
holding the watch lock and record, the file swapped by rename, G18 asserted
before (absent) and after (present, through `GovReport` with an all-clear
control), and cleared by a loop started on the new file; the record road with
no /proc; the stamp; the upgrade line.

## Not measured

The darwin road is pinned against the record only. Nobody has read a live
darwin loop across an install.
