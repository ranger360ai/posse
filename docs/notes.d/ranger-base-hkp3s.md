# ranger-base-hkp3s — the shared checkout gets no L1 wall for destruction, and the house stops prescribing it

ranger-base-6zw05 closed the COMMIT half of the sequencer wall (`--skip`)
and handed on the other half: `--abort` destroys what `--skip` destroys
and commits nothing, so the commit rule cannot reach it. The bead asked
the wider question as well: should L1 wall destruction of another persona's
uncommitted work in the shared checkout at all? `reset --hard`,
`checkout -f`, `restore` and `clean -fd` are all in that class, and today
only prose covers them.

**Decision** (ADR 0001, Consequences, the seeded rule's bullet, amended
2026-11-21): no. The class stays prose, at an accepted cost. What changes
is the one place the house PRESCRIBED a destroying verb: the continue
wall's refusal and AGENTS.md now point at `--quit` plus a path-limited
restore, not at `--abort`. That change is the code bead cut from this one.

## MEASURED 2026-11-21, this box

git 2.56.0, Linux 7.2.15-1-omarchy, worktree
`posse/richard-posse-ranger-base-hkp3s`. One fresh non-linked repo per row,
with every command typed through this seat's own L1 shim (so `rc=WALL` would
have shown a refusal; none fired). Another actor's work was planted before
each verb in four forms:

- N, a new `other.txt`, staged;
- M, tracked `b.txt`, modified and staged;
- W, tracked `c.txt`, modified and not staged;
- U, untracked `u.txt`.

`kept` means the content is in the index and on disk. `disk` means it is on
disk only, which is the intact state for W and U, and is a demotion (unstaged,
not lost) for N and M. `GONE` means the content is in neither.

```
verb                                     rc  N     M     W     U
reset --hard                             0   GONE  GONE  GONE  disk
reset (mixed) / reset --keep HEAD        0   disk  disk  disk  disk
checkout -f  /  checkout -f <branch>     0   GONE  GONE  GONE  disk
checkout <branch>   (no -f)              0   kept  kept  disk  disk
checkout -- .                            0   kept  kept  GONE  disk
checkout HEAD -- .                       0   kept  GONE  GONE  disk
switch --discard-changes <branch>        0   GONE  GONE  GONE  disk
restore .                                0   kept  kept  GONE  disk
restore --staged .                       0   disk  disk  disk  disk
restore -SW --source=HEAD .              0   GONE  GONE  GONE  disk
restore -SW --source=HEAD -- a.txt       0   kept  kept  disk  disk
clean -fd  /  clean -fdx                 0   kept  kept  disk  GONE
stash                                    0   GONE  GONE  GONE  disk   (into the shared stash stack)
stash -u                                 0   GONE  GONE  GONE  GONE   (into the shared stash stack)
rm -r --cached .                         0   disk  disk  disk  disk
cherry-pick --abort   (mid-conflict)     0   GONE  GONE  disk  disk
merge --abort         (mid-conflict)     0   GONE  GONE  disk  disk
reset --merge         (mid-conflict)     0   GONE  GONE  disk  disk
rebase --abort        (mid-conflict)     0   GONE  GONE  GONE  disk
cherry-pick/merge/rebase --quit          0   kept  kept  disk  disk
```

Two readings correct the bead's list. `clean -fd` never touches staged
work: a staged file is tracked. It destroys only files nobody has added.
And `rebase --abort` is worse than the other three `--abort`s, because it
also takes unstaged edits.

**Recoverability** (same box, same day). After `reset --hard`,
`checkout -f` and `restore -SW --source=HEAD .`, `git fsck --lost-found`
returned the victim's staged content (N and M) as dangling blobs in every
case. `git add` writes the blob, so the staged half survives until gc
prunes it. The unstaged half (W) was never an object and cannot be
recovered.

**The respelling.** A sequencer was conflicted and resolved, the victim's
N, M and W were planted, and then the seat ran `git <verb> --quit`, then
`git restore --source=HEAD --staged --worktree -- a.txt` (the operation's
own path), then for a rebase `git switch <branch>` without force:

```
verb         state left   HEAD/branch           N     M     W
merge        none         restored, main        kept  kept  kept
cherry-pick  none         restored, main        kept  kept  kept
rebase       none         restored, side        kept  kept  kept
```

Order matters for rebase. The restore must come BEFORE the switch: the
other way round, git refuses the switch over the dirty conflicted path,
which is safe but does not finish the exit. The revert row of that run is
a fixture artefact, because the revert did not conflict and committed
cleanly. Revert already has its own route in AGENTS.md (the two-step
path-limited commit, which finishes a conflicted revert outright), so it
needs no `--quit` prescription.

**Census** of every Claude transcript on the box (743 files, 84,375 Bash
`tool_use` calls, regex over the command text, so text inside heredocs
counts too and the SHARED column over-counts):

```
verb class            SHARED cwd    session worktree
reset --hard          3 (2 sess)    29 (9)
checkout -f/--force   0             6 (2)
restore (any)         5 (5)         98 (35)
restore whole-tree    0             4 (1)
clean -f              1 (1)         31 (15)
stash push/bare       2 (2)         15 (7)
sequencer --abort     4 (1)         36 (11)
sequencer --quit      0             1 (1)
```

Every SHARED hit read by hand was one of two things: text (ORDERS.md
heredocs, bead comments), or the operator's own session doing landing
surgery on `~/src/posse` (`reset --keep origin/main`, then a cherry-pick
with `--abort` on conflict). That session runs no crew PID, so a crew L1
wall would not have reached it. No crew seat typed a destroying verb in a
shared checkout. ASSUMED: transcripts outside `~/.claude/projects` (codex,
grok and other runtimes) would read the same. They were not censused.

## Why not a wall (the alternatives, priced)

- **Wall the whole class at L1.** Rejected. The arm has no PID rule to
  realize, because `renderContinueWall` renders only from the commit rule.
  And argv cannot draw the line that matters. `restore -- <path>` is the
  house's safe form, and it destroys a victim inside `<path>`, which is
  ADR 0022's same-file case. So the wall would be a list of whole-tree
  spellings (`.`, `:/`, no pathspec, `-f`, `--hard`), and that list is open
  to aliases, `--pathspec-from-file`, `read-tree -u --reset` and
  `update-index`. It would guard a route nobody walks (census above). The
  unrecoverable half (W) is also destroyed by editors, `cp` and `rm`, which
  no git wall can reach.
- **Wall only the sequencer `--abort`**, next to 6zw05's `--skip`. This is
  cheap, one more option per row. Rejected, though this was the one I
  wanted. `--skip` is walled because it COMMITS, so it is the commit rule's
  other half. `--abort` commits nothing. Walling it would make this the first
  L1 arm that originates policy no PID states. A typed `deny:` that a PID
  could state cannot stand down in a linked worktree, where `--abort` is
  used 36 times and is correct. And with the prescription fixed, nothing in
  the house sends anyone to `--abort` in the shared checkout any more.
- **Accept the class with nothing changed.** Rejected. An accepted cost is
  fine for a route the house does not recommend. Here, though, the continue
  wall's refusal names `--abort` in three of its four Way strings, and
  AGENTS.md says "otherwise `--abort` here". That sends a seat to the
  destroying verb at exactly the moment another persona's work is staged.

## Residuals, written down

- `git am --abort` and `git am --continue` are outside the continue wall's
  four verbs. `am` was not measured here.
- A seat that ignores the prose and types `--abort`, `reset --hard` and the
  rest in the shared checkout destroys the work, unrefused. Recovery for
  the staged half is `git fsck --lost-found`, before gc.
- `stash` moves the victim's work into the stash stack, which every
  worktree shares. That is recoverable, but only by whoever finds the
  entry.

The four probe scripts (destruction table, recoverability, respelling,
census) were session scratch and were not shipped. Each fixture is fully
described above, so the readings can be reproduced from this page.
