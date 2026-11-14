# ADR 0095 — A bwrap seat is never rendered over a main checkout's writable git dir

*Status: accepted 2026-11-13 (ranger-base-nk45q, discovered from
ranger-base-166va) · owner: architect · builds on
[ADR 0080](0080-posse-on-an-always-on-linux-host-rex-first.md) (the bwrap
render) and [ADR 0086](0086-a-cage-map-lowers-the-wall-never-a-pids-rule.md)
D3 (who gets a git grant) · the watcher's kill-not-repair rule is
ranger-base-0qpsw's and is unchanged*

## Context

A bwrap seat's Deny over an existing file is a `--ro-bind` of the file over
its own name. When the host renames a new file over that name, the kernel
detaches the mount in every namespace, and `posse wall-watch` kills the seat
(ranger-base-0qpsw D2). ranger-base-166va stopped rendering a Deny that no
write grant overlaps, because the read-only base already refuses those
writes. That ended the false kills for linked-worktree seats.

One shape is left. In a **main checkout** (`.git` is a directory, so
`LinkedGitDirs` returns one entry) the seat's grant is the cwd, which contains
the whole `.git`. `BwrapWritable` returns that set unchanged. So
`.git/config`'s Deny is a mount inside a writable directory. After a host
rename removes it, the seat can replace `.git/config` itself (hooksPath,
fsmonitor, core.sshCommand all run as the operator). The kill is correct in
this case, and it fires on every host write to that file.

Which host writes replace `.git/config`. MEASURED 2026-11-13 on git 2.56.0,
Linux 7.2.15, by comparing the inode before and after each verb in a scratch
repo:

| host verb | replaces `.git/config` | who runs it |
|---|---|---|
| `git worktree add -b` / `add --detach` | no | dispatch hire |
| `git config branch.<b>.bead <id>` (`recordBead`) | **yes** | dispatch hire, every one |
| `git worktree remove --force` | no | dispatch reap |
| `git branch -D <b>`, even with no `branch.<b>` section | **yes** | dispatch reap, every one |
| `git config posse.landcheck …` | **yes** | landing check |
| `git worktree prune` | no | hire |

So a bwrap seat in the main checkout of a repo that dispatch serves dies at
the next hire, reap or land check there. This matches the triage's two timings on
ranger-base-166va (21:02:04 hire, 21:08:36 reap). The operator's own
`git config` and `git branch -D` kill it too.

The shape comes from two places (`NewSessionOpts.Worktree`, herdrback.go):
`posse new` and recipes, which never take a worktree (ADR 0008: a crew
session works in the operator's checkout), and dispatch when
`PlanSessionTree` answers nil, which happens for a repo on a detached HEAD
with no session branch to return to. Every `cage: seatbelt` persona renders as
bwrap on this host.

## Decision

**D1. The render refuses the shape.** `BwrapArgs` refuses a render in which
a watched Deny mount sits at `<dir>/.git/config` and a write grant contains
it. The refusal says why and what to do:

> bwrap cannot wall `<repo>/.git/config` inside a writable `<repo>/.git`: every
> host git config write (each dispatch hire and reap in this repo, the landing
> check, the operator's own `git config` / `branch -D`) replaces that file by
> rename, which takes the wall off, and wall-watch then kills the seat
> (ranger-base-0qpsw D2, ADR 0095). Run this seat in a linked worktree:
> dispatch it, or launch it from one.

The check reads the render, not the launch path. That way `posse new`, a
recipe, dispatch's detached-HEAD fallback and any future caller all meet the
same refusal at the same place. It is keyed to `.git/config` because that is
the file the table shows host writers replace. Other Denies inside a grant
are not changed here (see Not decided here).

**D2. Dispatch's fallback takes the refusal as "not launchable now".** A
detached-HEAD repo already makes dispatch fall back to the shared checkout.
For a bwrap seat that fallback now fails at render. It must go through the
existing launch-failure path (`unclaimAfterLaunchFailure`), with the bead
handed back and one reason line per pass, and must not become a new
claim/unclaim storm every tick. ASSUMED that the existing path already
behaves this way for any render refusal. The implementation bead checks it.

**D3. `posse gates` shows it before a launch does.** The planned-render row
for a session whose dir is a main checkout names this refusal, so an
operator reading the wall sees it before typing `posse new`.

## Consequences

- No bwrap seat ever holds a whole `.git` writable. Every caged commit on
  Linux goes through a linked worktree's narrowed grant (ADR 0080 D20), which
  never contains the common dir's top. One git-write shape is left to reason
  about instead of two.
- A caged persona can no longer be a crew session in the operator's own
  checkout on Linux. Today such a seat launches and is then killed within
  minutes by the next hire or reap, filing a P1 security bead each time
  (ranger-base-166va, -uppgy). After this change it fails at launch with the
  reason, files nothing and costs no triage.
- darwin is unchanged. Seatbelt carves by path rule and has no mount for a
  rename to remove.
- Exit hatch: if ADR 0008 is ever amended to give crew sessions worktrees, D1
  never fires and can be deleted with its pin. It holds no state.

## Alternatives rejected

- **Narrow a main checkout's git grant the way `bwrapGitSwap` narrows a
  linked one** (grant `objects`, `refs`, `logs`, never `.git`'s top). This is
  the clever one the bead offered, and it does not build. MEASURED 2026-11-13
  (bwrap 0.13.1, git 2.56.0, `.git` read-only with `objects`/`refs`/`logs`
  bound writable): `git add` and `git write-tree` fail on `.git/index.lock`,
  and `update-ref HEAD` and `checkout -b` fail on `.git/HEAD.lock`, all with
  "Read-only file system". A main checkout keeps its index, HEAD, ORIG_HEAD
  and every sequencer state at the git dir's top, and git creates each one by
  lock-and-rename in that directory. A linked worktree can be narrowed only
  because git moved exactly those files into `worktrees/<name>/`. Getting that
  shape means creating a worktree, which is D1's remedy.
- **Stop posse's own rewrites** (`git update-ref -d` instead of
  `branch -D`, and move `branch.<b>.bead` and `posse.landcheck` out of
  `.git/config`). That fixes dispatch's writers and leaves the operator's.
  The seat in question sits in the operator's own checkout, where the
  operator runs `git config` and `branch -D`. The kill would become
  rarer and still be guaranteed, and it would cost moving two
  pieces of persisted state with readers across landsweep and landcheck.
- **Re-bind instead of kill when the rename is a known host writer.** In this
  shape the seat can write `.git`. During the gap it can rename its own file
  over the host's, or keep an fd open, and either one survives a re-bind.
  That is exactly why ranger-base-0qpsw D2 kills, and it applies here with
  more force than anywhere else.
- **Host self-bind (rename becomes EBUSY).** Needs init-namespace root, and
  makes the dispatcher's own `recordBead` and `branch -D` fail in that repo
  (ranger-base-0qpsw's pricing).
- **Accept the kills as they are.** The cost is one P1 security bead per
  host config write, each needing triage to rule out a seat write. That is
  the noise ranger-base-166va was filed to remove.

## Not decided here

- A Deny inside a write grant on some file other than `.git/config` (for
  example a project's own `.claude/settings.json` under the cwd grant) has the
  same mechanism. Nothing posse runs is measured replacing one while a seat
  is live, so D1 does not refuse them. If wall-watch files one, it extends
  D1's key list, still read from the render.

## Lineage

| date | bead | change |
|---|---|---|
| 2026-11-13 | ranger-base-nk45q | written |
