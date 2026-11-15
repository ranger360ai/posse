# The socket wall took cross-directory rename with it (ranger-base-p5i2c)

2026-11-15. A Landlock domain that handles ANY filesystem access right and
does not handle `LANDLOCK_ACCESS_FS_REFER` denies every `rename(2)` and
`link(2)` whose source and destination directories differ, with `EXDEV`. The
seat's socket wall (`posse wall-unix`, ADR 0080 D27) handles exactly one
filesystem right — `RESOLVE_UNIX` — so it inherited that default-deny, for
every process in the domain, everywhere in its view, whatever the mounts say.

The step's own header said the opposite, and so did the sentence `posse gates`
printed from it: "The mounts above are unchanged by it."

## What it cost

`git branch -m` behind the step is exit 128: `unable to move logfile
logs/refs/heads/main to logs/refs/.tmp-renamed-log: Invalid cross-device
link`. ranger-base-i92tj had just put the step on the landed-check judge's
line, so four `internal/treepins/bdargvgate_qa_test.go` pins reddened on their
fixture's `git branch -M`, `make treepins-check` reddened, and ADR 0088's
judge reported main RED on **every** tree — including the tree last recorded
green, which is the control that proved no landing had caused it
(ranger-base-ufd7z, filed as "main red at 925c90bf297f").

## Why the probe read green over it

`docs/adr/0080-resolve-unix.probe.sh`'s U11 row — "ordinary file
read/write/mkdir/rename in it" — renamed `W/d -> W/d2`. One directory. The
class is cross-directory refer and nothing else, and this domain never touched
the same-directory case.

Two ways to measure it wrong, both of which were taken:

- **`mv` reads green.** coreutils falls back to copy+unlink on `EXDEV`, so
  every shell probe of this class passes over a wall that has already taken
  rename away. It takes `python3`/`os.rename`, `ln`, or git.
- **Renaming within one directory reads green**, which is what U11 did.

## The fix

Handle `REFER` on exactly the arms that handle `RESOLVE_UNIX`, and add one
rule granting `REFER` on `/`. Handled-and-granted is a no-op, and it is the
only spelling Landlock has for "leave refer alone": the right has to be
handled, or the kernel's refer default-deny applies, and it has to be granted,
or Landlock's allow-only rule denies it everywhere. Both halves are needed and
neither is enough — a mutant that handles `REFER` and grants nothing is the
same `EXDEV` with an extra syscall.

The grant confers no access: `rename(2)` and `link(2)` still need a writable
mount and a permitted uid, so it restores exactly the status quo this layer
was never meant to change. `RESOLVE_UNIX` stays handled and granted nowhere,
which is the whole of the socket wall. ADR 0080's new amendment carries D33-D36.

`/` and not the seat's grant list, because the grant has to reach everywhere
the seat can already write — which the bwrap mounts decide and this layer does
not. A narrower rule brings the same bug back at whatever path the next PID
grants.

## MEASURED 2026-11-15, D1 (Linux 7.2.15-1-omarchy, Landlock ABI 10, git 2.56.0)

By syscall, inside `posse wall-unix -- python3` and outside it, before the fix:

| case | inside | outside |
|---|---|---|
| rename, same directory | OK | OK |
| rename, across directories | EXDEV | OK |
| link, same directory | OK | OK |
| link, across directories | EXDEV | OK |
| rename a DIRECTORY across directories | EXDEV | OK |
| write / mkdir / unlink | OK | OK |
| connect(2) to a host pathname socket | EACCES | OK |
| kill(2) a host process outside the seat | EPERM | OK |

After the fix, every row reads as the control except the two the wall is for:
`connect` stays EACCES and `kill` stays EPERM. `git branch -m` behind the step
goes from exit 128 to exit 0.

The ADR probe carries it as rows U14 (the bug), U15 (the fix) and U16 (the
socket wall still holds under the grant): 17 rows, 0 mismatched.

End to end: the four symptom pins behind the step, `go test -run` over
`./internal/treepins` — FAIL on all four with the pre-fix binary, `ok 1.292s`
with the fixed one — and `make treepins-check` behind the fixed step,
`ok 111.171s` at a one-minute load average of 18.9.

## The pins, and what they red on

- `TestQAWallUnixRulesetHandsReferBackWhereItHandlesAnyFSRight` (plan, every
  GOOS): REFER is handled wherever any filesystem right is, the grant is
  exactly REFER, and the grant never carries `RESOLVE_UNIX`.
- `TestLiveWallUnixLeavesCrossDirectoryRename` (kernel's side, real bwrap
  seat): four operations — same-directory rename, cross-directory rename,
  cross-directory link, directory rename across directories — against a
  control seat that differs by the step alone, and the socket wall read in the
  same breath so that deleting the step cannot make it green.

MEASURED, two mutants, both red in both pins:

- handles `RESOLVE_UNIX` only (the bug as shipped): the live pin reports
  `same=ok cross=EXDEV link=EXDEV dir=EXDEV`.
- handles `REFER`, grants nothing: the same `EXDEV`, and the plan pin names
  the missing grant.

## The lesson that generalizes

A right added to `handled_access_fs` restricts everything it covers, whether
or not restricting it was the point. `REFER` is the only `LANDLOCK_ACCESS_FS_*`
bit documented to work by default-deny on a *pair* of paths, so it is the one
that surprises — but the probe is where that gets read before it ships, and a
probe row has to name the operation the kernel names, not a neighbour of it.
