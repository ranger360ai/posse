## The fourth operand: a base that moved FORWARD past the conflict (ranger-base-c6ohn)

`ranger-base-ejju3` made `standingMergeBlock` re-read the operands a standing
merge-back block was an answer about, and deliberately left one: a base that
moved **forward** such that the replay would now succeed — the operator
answering the handoff by REVERTING the conflicting commit with a new commit
rather than resetting `main`. Nothing the sweep can read sees it. The base is
not an ancestor of the branch (no fast-forward), the work is on the base under
no sha at all (no equivalence), the tree is clean (not the dirt arm). Only the
replay answers it, and the replay is the session-tree write the whole skip
exists to stop (`ranger-base-9u5zy`, ADR 0058 fact 4).

It is closed as a **filter plus a record**, because either half alone is one of
the two bugs the file is about:

- **the filter** — `mergesCleanly` (`worktree.go`), `git merge-tree
  --write-tree <base> <tip>`, which merges in the repo's object store with **no
  worktree at all**. A whole-branch merge is not a commit-by-commit replay, so
  a clean answer is "worth asking git for real" and never "this will land".
- **the record** — `probedBaseKey` (`worktree.go`), `branch.<b>.posseProbedBase`,
  the base sha the replay last actually ran against, written by
  `MergeSessionWork` immediately before each `git rebase`. Without it, a branch
  where the filter and the rebase disagree is probed on **every pass forever**,
  which is `ranger-base-9u5zy`'s bug back wearing the other operand's clothes.

`blockStillStands` (`landsweep.go`) asks them in that order, last, after the
three arms `ranger-base-ejju3` added. So the answer costs one tree write per
base **MOVEMENT** and not one per pass.

### What clean is read as, and why not the exit status alone

MEASURED 2026-09-11, git 2.50.1 / macOS 26.4.1 — re-measuring
`ranger-base-ejju3`'s 2026-09-10 reading and agreeing with it:

| case | exit | stdout |
|---|---|---|
| clean merge | 0 | the tree oid, and nothing else |
| real conflict | 1 | a tree oid, then the conflicted stages and `CONFLICT (add/add): …` |
| revision git cannot resolve | 1 | nothing (message on stderr) |

A git before 2.38 has no `--write-tree` and reads it as a tree-ish, failing the
same way. So clean is **exit 0 AND a single object name** — the exit status
separates clean from conflicted, and the lone object name separates both from
every version and argument accident. An unanswerable question comes back false,
which leaves the block standing rather than letting a probe through on a
reading nobody got.

### What the filter costs, asked every pass over every blocked tree

Git's objects are content-addressed, so re-merging an unmoved pair writes the
oids the store already holds. MEASURED 2026-09-11 (same box), 10 consecutive
calls over one unmoved pair: **2 loose objects on the first conflicted call and
zero on the nine after it**; zero on all ten of the clean case. Only a base that
MOVES is a new question, and only a new question costs anything. None of it
lands in the session's git dir, which is the reading `lastTreeWrite` takes.

### How often the question is even live — the census ranger-base-c6ohn asked for

The bead said nobody had counted how often a blocked branch's base moves in a
way that would make the replay succeed. Counted 2026-09-11 over this repo's
whole history of the class: **57 merge-back-blocked beads across 47 distinct
branches, every one of them closed, and every one of those 47 branches now
gone** from `refs/heads` — so there is no live population to measure at all.
Of the 39 tips ADR 0058's retire kept under `refs/posse/retired/`, **35 belong
to a branch that once carried a merge-back block, and 35 of 35 still conflict
with `main` today**, hundreds of commits later.

Read it as a **cost** result and not as a frequency one. These are branches
whose block a human ANSWERED, and answering it usually means main now carries a
hand-resolved version of the same work — which is itself a reason the replay
still conflicts. What it does say, with no caveat, is that the filter answers
"no" for the entire population history has produced: over that class
`mergesCleanly` returns false, no probe runs, and the tree stays quiet. The
branch this exists for is the one that never reached a human, and the whole
point of the arm is that it is the one nobody has ever seen — because until now
it stood until a human closed it or `posse worktrees --land` was run.

### The residual, stated rather than hidden

A branch where the filter and the replay disagree — net diff empty, own commits
conflicting one at a time — over a base that moves every pass, still probes
every pass. Nothing cheaper than the replay can tell that branch from one that
would now land. `TestAFilterThatDisagreesWithTheReplayProbesOncePerBaseMove`
(`mergeblocked_qa_test.go`) is the bound: one probe per base move, and the tree
quiet on every pass between moves.
