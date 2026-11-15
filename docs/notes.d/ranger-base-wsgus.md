## A private bind's source was checked at its last component, and the seat owned every component above it (ranger-base-wsgus)

MEASURED 2026-11-14, from the code at `9293c431` and from the mutation runs
below. `bwrapPrivateArgs` (internal/posse/bwrap.go) renders each
`carve.Private` entry, and two of its arms take a SOURCE under the seat's own
cache dir: the dir copy of claude's `session-env`, `shell-snapshots`,
`backups`, `file-history`, `debug`, `sessions` and `paste-cache`
(ranger-base-5632o and successors), and ranger-base-a544g's FILE bind of
`history.jsonl`. Both asked one filesystem question, about one component:

- the dir arm asked `bwrapStat`, which is `os.Stat`, so a link at the
  member's own name resolved and bwrap bound the link's target read-write at
  `<cfg>/<member>`;
- both arms asked only about the LAST component. `<SeatCacheDir>/claude` is
  in the seat's own write set (`seatbelt.go`'s `add(SeatCacheDir(home, cwd))`,
  and `BwrapWritable` takes the same list), so the seat could remove that
  directory and plant a link in its place. `<seat>/claude/history.jsonl` then
  `lstat`s as the TARGET's own regular file, and a544g's lstat — which is a
  real check of a real file — passed on it.

The mount lands after step 2's grants and before the deny and read-mask
steps, and those steps name the paths the carve-out computed, not the alias a
link introduces. So the widening was invisible to every later narrowing in
the same render.

### The fix: the root is declared, and every component below it is walked

`PrivateBind` carries a `SrcRoot` — the exact prefix `Src` was composed from,
in the producer's own spelling. `bwrapPrivateSrcStat` lstats `SrcRoot` and
then every component below it, refuses a symlink or a non-directory anywhere
along the way, and hands the LAST component back unjudged so each arm keeps
its own rule (a regular file for the file bind, a directory for the dir
copy). An entry with a source and no `SrcRoot` is refused outright, as is one
whose `Src` is not lexically under the root it declares.

Nothing at or above the root is inspected, and that is the trust line: the
root is the launcher's own path, a seat cannot replace it because nothing
grants its parent, and demanding its ancestors be link-free would refuse
every launch on a host that reaches its home or its cache dir through a
symlink. `claudePrivateSessionState` passes `SeatCacheDir`'s answer
unresolved, because the containment test is lexical.

### The prep steps were the other half

`PrepMkdirAll` and `PrepMaskPoint` run in the UNCAGED launcher. Under the old
check a planted link also aimed those at the seat's chosen directory: the
mutation run below prints
`mask point <seat>/claude/history.jsonl` in a render whose `<seat>` was a
link, which is a 0600 file the launcher would create inside the target. They
are emitted on the walk's verdict now, and a refused render hands the
launcher no step at all — which the new pin asserts, because an error path
that came back with prep attached would be the same hole one caller away.

### What the pins hold, and what each mutation reds

`TestQAWallClaudeSessionStateRefusesALinkAtAnyComponentOfThePrivateSource`
plants a link at three components in turn — a dir member's own source,
`<seat>/claude` with a real `history.jsonl` and a real directory per member
in the target, and the seat cache dir itself — and asserts the render
refuses, names the planted component, and emits no prep.
`TestQAWallPrivateBindWithNoSrcRootRefuses` holds the declaration itself.

MEASURED, by replacing the walk with each of the two checks it replaced:

| mutation | result |
|---|---|
| the final component only, `os.Lstat` (a544g's) | the two parent-link cases red, and the member-link case stays GREEN — a544g's check really does catch a link at the member's own name |
| the final component only, `os.Stat` (the dir arm's) | all three cases red, and `TestQAWallClaudeSessionStateRefusesALinkAtHistory` reds with them |

The partition is the point: neither old check was redundant with the other,
and neither was enough.

### Not closed here

- ranger-base-p47q6
