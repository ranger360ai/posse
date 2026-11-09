# The stall verdict's work reading reads the tree too (ranger-base-x475y)

ADR 0084 D3, built. `committedWork` became `sessionWork`: work happened if
there are commits ahead of the branch the session was cut from **or**
`git status --porcelain` in its own tree is non-empty. Only "both halves
read, both empty" can still reach the unclaim.

## The measurement the reading rests on

The porcelain half decides, where every other reader of that status only
reports it. So a prepare path that leaves one file of posse's own in the tree
no longer files a spurious P1 (ranger-base-e01op) — it keeps every claim it
is ever asked about, forever, with nothing printed. rangerhq-81d, lost
silently. The bead made the measurement a precondition of the build, and it
is a pin rather than a paragraph:
`TestPreparedSessionTreeIsPorcelainCleanOnEveryPreparePath`
(internal/posse/worktree_test.go, arm 2).

MEASURED 2026-11-09, darwin 25.6.0 / git 2.50.1, `--untracked-files=all`
(stricter than the `normal` porcelain `sessionWork` itself takes, which
collapses an untracked directory to one line):

| prepare path | porcelain lines |
|---|---|
| `EnsureSessionTree` + `PrepareSessionHead(caged=false)` — every seat outside the container tier | 0 |
| `PrepareSessionHead(caged=true)` — the container tier's detached HEAD | 0 |
| four relaunches alternating the tier | 0 each |
| a live dispatched claude seat's own tree, uncaged, after a session's bd reads | 0 |

Nothing had to be excluded. `seedScaffoldExcludes` already hides the two
things seeding writes (`.beads/redirect` and a `worktree_link:` symlink), and
the two the caged prepare writes — `<common>/logs` and
`<gitdir>/config.worktree` — are inside the git dir, where no working-tree
status can reach them. That last part is the reason to *expect* zero and
exactly the kind of reasoning that is wrong the day somebody adds a third
prepare-time create, which is why the pin asserts both files exist before it
asserts the status is empty.

## The design, in one line each

- **A half that will not answer is ignorance only when the other half found
  NOTHING.** Evidence beats a failed read: a reading that found a commit does
  not need the tree's status to know a turn ran.
- **The shared-checkout arm is unchanged, `(none, answered)`** — and now for
  a second reason. Reading the *shared* checkout's status there would read
  every other persona's dirt as this bead's work.
- **`dirtyPaths` split into `dirtyPathsRead`.** The callers that REPORT the
  list keep today's "a failed read is nothing", which is safe for all of
  them; the one caller that DECIDES on the absence takes the readable form.
  One implementation, so the `--no-optional-locks` lesson (ranger-base-a8tqz)
  stays in one place.
- **The print and the D5 record NAME which evidence they found**, in one
  spelling (`workEvidence.String`), because the two kinds send the reader
  somewhere different: a commit is landed work `posse worktrees` can take,
  where a dirty tree is work only the seat itself will ever commit.

## Mutations run, 2026-11-09

Each reds the pin whose name is the behaviour; none survived.

| mutation | reds |
|---|---|
| `sessionWork` ignores the tree half (D3 reverted) | `…WhoseTreeHoldsUncommittedWorkKeepsItsClaim` |
| `dirtyPathsRead` reads a failed status as a clean tree | `TestSessionWorkIsATriState` |
| the `if w.found()` short-circuit removed | `TestSessionWorkIsATriState` |
| `found()` needs BOTH kinds of evidence | that arm, `…WhoseBranchHoldsACommit…`, tri-state, `TestWorkEvidenceNames…` |
| the tree half always reports one change (over-keep) | `…OverACleanSessionTreeStillHandsBack` and three more |
| `String()` names the commit as dirt and the dirt as a commit | that arm, `…WhoseBranchHoldsACommit…`, `TestWorkEvidenceNames…` |
| `PrepareSessionHead` writes `config.worktree` into the worktree root | `…PorcelainCleanOnEveryPreparePath` |
| `seedScaffoldExcludes` not called | that pin and `…PorcelainCleanBeforeTheSeatTypes` |

The over-keep mutant is the one worth keeping in mind: it reds the
hand-back arm, which is the only thing standing between this widening and a
queue where nothing is ever freed.

## Found on the way out

`EnsureSessionTree` refuses a tree posse itself detached, so a RELAUNCH into
a container-tier session worktree dies before the launch —
**ranger-base-p60mv**, filed with the fixture that measured it. The launcher
runs `EnsureSessionTree` first and `PrepareSessionHead` once the tier is
resolved, so a second launch into a caged session reaches `existingTree` over
a detached HEAD and gets the Die written for a path belonging to another repo.
Not reached in the field (no container-tier tree is live on this box) and not
reached by the suite either, because every caged arm calls
`PrepareSessionHead` directly. The relaunch loop of the new pin skips
`EnsureSessionTree` while the tree is detached, and says so.
