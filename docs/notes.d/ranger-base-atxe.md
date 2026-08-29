## `--land` read no record before it merged (ranger-base-atxe)

`LandSessionTrees` (`internal/rhq/worktree.go`) looped every tree
`SessionTreesIn` found and called `MergeSessionWork` on it unconditionally.
It consulted no record at all — neither the issue store nor the branch —
unlike its automatic sibling `landClosedTrees` (`landsweep.go`), which reads
`branch.<branch>.posseBead` and **reports** rather than lands a tree no
record accounts for.

**What that costs, measured.** A session tree left over from `rangerhq-vojc`
held one commit `6217c9f` that main did not have *by sha*, and whose
content is byte-identical to `2418bde` already on main: the historic
stranded close, re-landed by hand under a different bead id
(`ranger-base-k7nb`). Nothing in the merge path can see that. The patch-ids differ because of base drift, so
`git cherry` prints `+`; the re-land carries no `(cherry picked from commit
…)` trailer because it was not a `-x` pick; so `equivalentOnBase` — the
predicate ranger-base-g2xf added for exactly this class — returns nil. A
blind `posse worktrees --land` would have replayed 778 stale lines onto main
or conflicted trying.

And the listing the operator reads first said only

```
1 commit(s) not on main
```

which is true, and identical for a real strand and for that duplicate.

**Three content tests that do NOT work here**, all measured on the vojc
branch against main at `4345b51` (the tree is the one `ranger-base-u67g`
asks the operator to remove):

- `git cherry main <branch>` → `+`. Patch-ids differ; that is the g2xf case
  and this is not it.
- `git merge-tree --write-tree main <branch>` → **conflicts** (`add/add` on
  five files). The merge base is old and the same content arrived on main by
  an unrelated route, so a three-way merge cannot tell they are the same.
- tree comparison on the paths the branch touched — all 8 still differ.
  Correctly: the branch's content matches main **at 2418bde**, and main has
  moved on since. Landing it would not be a no-op, it would be a silent
  revert of the later work.

So there is no cheap content predicate that recognises this shape, and the
sharpest true statement about it is not "already landed" but "landing it
would undo later work" — a much bigger question. The record is the cheap
guard, and it is the one that was missing.

**The fix.** `unaccountedFor(t, force)` gates the loop: a tree holding
commits its base does not have, whose branch names no bead, is printed and
skipped. `posse worktrees --land --force` lands it anyway.

Only a tree with something to land is gated — nothing ahead of the base is
nothing to get wrong, and a base that cannot be read at all lands nothing
either and gets `MergeSessionWork`'s own words instead, which say more.

**Empty is not "nothing to answer for".** A crew session's tree has no bead
by design (`CreateSession` stamps `o.Bead`, which is empty for one), and so
does every tree cut before the stamp landed on 2026-08-27. Both are
legitimate, and both still get the refusal, because from git alone they are
indistinguishable from the vojc shape. `--force` is the operator saying
which one it is.

**The listing says it too.** `treeState` now prints `N commit(s) not on
<base>, for <id>` or `…, no record says which bead`, so the judgement is
available before the command is typed rather than after it has run.

**And the sweep's prescription was updated with it.** `landClosedTrees` told
the operator that "`posse worktrees --land` decides it" for exactly these
trees. Once `--land` reads the same record, that sentence named a command
that would refuse the tree it was printed about; it now says `posse
worktrees` shows it and `--land --force` decides it, pinned in
`TestSweepWillNotGuessWhichBeadATreeHolds`.

Pins (`internal/rhq/worktree_test.go`):
`TestLandWillNotTakeWorkNoBeadRecordAccountsFor` — three arms, a stamped
tree that must still land (the control that keeps the gate narrow), an
unstamped one that must not, and `--force`, which must;
`TestListSessionTreesNamesWhichBeadTheUnlandedWorkIsFor` — both halves of
the listing phrase, plus the negative that a stamped branch stops reading as
unrecorded.

Mutation-checked four ways: forcing `unaccountedFor` to `""` reds the land
pin and nothing else; dropping its `force ||` reds the `--force` arm;
reverting `treeState` to the bare count reds the listing pin; restoring the
old sweep prescription reds the sweep pin.

**Not fixed here.** `RemoveSessionTree` still counts commits by sha, so it
refuses to retire the vojc tree even though nothing in it would be lost —
that removal is `ranger-base-u67g`, and it needs the operator's hand.
