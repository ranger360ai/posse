## A close only reports a landing it measured (ranger-base-dybv)

`MergeOutcome.Merged` said the base had the session's work. It did not
measure that. It was **inferred**, three different ways:

- from `git merge --ff-only`'s exit status on the happy path — sound;
- from `!branchExists(repo, branch)`, read as "nothing was ever committed
  on it" — a guess;
- from `rev-list --count base..branch == 0`, read as "nothing to land" —
  a guess.

Both guesses ask the **branch**. A persona's commit lands wherever the
**worktree's HEAD** is, which is not always the branch: a detached HEAD in
the session tree takes the commits with it and leaves the branch at the
base. Every branch-shaped question then answers *nothing unlanded*, and the
close reports success over work that is on no base and that no ref anyone
will check out reaches.

Measured twice in the field before this landed — rangerhq-81y0/eb03495, and
rangerhq-vojc/6217c9f, an eight-file cost-provider seam invisible for a day
until a verify pass went looking (re-landed by hand as ranger-base-k7nb).

The false `Merged` is also what makes it **destructive** rather than merely
misleading: `posse kill` retires the tree and deletes the branch on the
strength of that flag, and `RemoveSessionTree`'s own refusal asks the
branch too, so it agrees there is nothing to lose. The only copy goes.

**The fix.** `landed()` in `worktree.go` is now the single place the file is
allowed to say a merge happened, and it says it from
`git merge-base --is-ancestor <work head> <base>` — the fast-forward
precondition, asked of git rather than assumed from an exit status
elsewhere. `workHead()` supplies the subject in the order the answer
survives: the worktree's HEAD first, the branch second (for a tree already
retired), nothing third. When the base does not reach it, `Merged` is false,
`Commits` is re-counted from the *work head* so the report is not "0
commit(s) did NOT reach main", and `Reason` names the sha and prescribes
`git -C <tree> branch -f <branch> HEAD`. Every existing caller — `mergeBack`,
the landing sweep, `posse worktrees --land`, the kill — already had a loud
arm for `!Merged`, so nothing new had to be invented to be heard: the pass
prints ⚠ and `mergeBack` files the merge-blocked bead.

**Ask it of HEAD as it is now, never of a sha captured on the way in.** A
rebase legitimately rewrites the work: afterwards the pre-rebase sha is an
ancestor of nothing and the tree's HEAD is on the base, and only the second
is the fact anyone cares about. A postcondition written against the entry
sha would refuse every rebase-then-fast-forward, which is the *normal* path.

Pins: `TestMergeSessionWorkRefusesWorkTheBranchDoesNotReach` (three arms —
control, head-off-branch, no-branch-reaches-it) and
`TestKillKeepsATreeWhoseWorkIsOnNoBranch`, which is the consequence.
