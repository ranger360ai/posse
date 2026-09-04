## A merge-back block re-filed on a branch a previous rescue already landed (ranger-base-4ri4n)

`docs/notes.d/ranger-base-tb90m.md` records three outcomes for a merge-back
block: re-land the strand (ranger-base-0qiny), find the block spurious, or
find the strand **superseded** and refuse to land it. This is a fourth, and
it is the one that repeats:

**the strand was already re-landed by an earlier rescue bead, and the block
was filed again against the same untouched branch.**

Nothing is wrong on `main` and nothing is wrong with the reaper. The branch
still exists because a rescue picks onto its own branch and leaves the old
tree for the operator, and a left-standing branch is re-read at every pass.

### The instance

`posse/gilfoyle-posse-ranger-base-nw9zg` — 5 commits, none an ancestor of
`main`, `posse worktrees` saying *"5 commit(s) not on main, for
ranger-base-nw9zg"*. Its whole deliverable is on `main`, put there on
2026-09-02 by **ranger-base-2yaud** (`d889cc7 4ddf15d fc835aa 55d0e8e`),
which is itself the rescue that ranger-base-169ft's verify asked for. The
fifth commit is the branch's **base**, `34a27b4` (ranger-base-vd1bo), a
shared base three worktrees were cut from that `main` re-landed as `6a230eb`
— the 169ft fleet incident. 2yaud skipped it on purpose: `main`'s copy is a
superset (the `rhq`→`posse` rename, added `t.Parallel`, changed fixtures).

So the branch holds five commits `main` does not have by sha and zero lines
`main` does not have by content.

### The reading that settles it costs one command per path

Ancestry says STRAND for a branch whose content is fully landed, and
`git cherry` is worse than useless here — it gives a **mixed** answer:

```
+ 34a27b4   the superseded base (another bead's)
- c663a9a   patch-id survived
+ 54ed42b   patch-id moved
- 51ca0a7   patch-id survived
+ df434d8   patch-id moved
```

Compare the DELIVERABLE, over the branch's own touched paths, before
picking anything:

```sh
B=posse/gilfoyle-posse-ranger-base-nw9zg
for p in $(git diff --name-only <base>..$B); do
    echo "== $p"; git diff --stat $B main -- "$p"
done
```

Three of four printed nothing — byte-identical. The fourth, `Makefile`,
printed 15 insertions **`main` has and the branch does not** (`verify-parallel`,
which landed later): a strict superset, so a pick advances nothing and
reverts a landed decision. That is the whole question, and it needs no
worktree, no rebase and no merge-base.

### Why the reaper cannot see it, measured twice

`equivalentOnBase` (`internal/posse/worktree.go:1080`) reads exactly two
kinds of evidence per commit in `base..tip` — a patch-id match from
`git cherry`, else git's `(cherry picked from commit <sha>)` trailer — and
on the first commit with neither it `return nil`s the whole branch. That
all-or-nothing is deliberate and pinned
(`TestMergeSessionWorkStillStrandsAPartlyLandedBranch`, worktree_test.go:1601):
accounting for *some* commits would let a tree holding the only copy of the
rest be deleted. Two things defeat it here, and both are the rescue's shape,
not a defect:

**1. The hand resolution moves the patch-id of the commits that touch the
resolved file — and only those.** 2yaud's pick hit one conflict, `Makefile`'s
`.PHONY` list, resolved as a UNION. Per-file patch-ids across the pick:

| commit → twin | Makefile | every other file |
|---|---|---|
| `54ed42b` → `4ddf15d` | DRIFT | SAME |
| `df434d8` → `55d0e8e` | DRIFT | SAME |
| `c663a9a` → `d889cc7` | — | SAME (whole commit) |
| `51ca0a7` → `fc835aa` | — | SAME (whole commit) |

One conflicting file out of four cost two of four commits their patch-id
evidence. Base drift alone did not: the two commits that never touch
`Makefile` kept theirs across the rebase, because `patch-id --stable`
already ignores line numbers and context offsets.

**2. None of the four re-landed commits carries an `-x` trailer.** The
sequencer markers have to be deleted by hand in a posse worktree
(packed-refs is EPERM), so the pick gets committed by hand and git's own
trailer is dropped — the ranger-base-0qiny amend. Census:
`git log -1 --fixed-strings --grep="cherry picked from commit <sha>" main`
is empty for all five. With no trailer and a moved patch-id, two commits
have no account at all.

**And the amend would not have been enough.** `34a27b4` is in
`main..tip`, belongs to another bead, and was correctly never picked, so it
can never have either kind of evidence. **One superseded commit anywhere in
`base..tip` makes the branch permanently unaccountable**, no matter what is
done to the others. A branch cut from a base that `main` later rewrote will
re-file its merge-back block on every pass until the operator retires it.

### What to do with one

Nothing to `main`. Merge-back is `--ff-only`, so a conflicting branch cannot
regress anything; it costs a line in `posse worktrees` and a re-filed bead.
Read the deliverable, say so on the bead naming the rescue that landed it and
the commit that superseded the base, and close. The tree and branch are the
operator's to remove (`git worktree remove` + `git branch -D`; Claude Code's
auto-mode classifier denies both from a seat, and `RemoveSessionTree`'s own
refusal counts commits, not content, so it refuses this tree too).

The tell that saves the reading, in git's own words — the merge-back bead
quotes the failed rebase, and it is already the answer:

```
warning: skipped previously applied commit c663a9a
warning: skipped previously applied commit 51ca0a7
error: could not apply 34a27b4... the memory credential scan reads … (ranger-base-vd1bo)
```

A genuinely stranded branch skips nothing. Two skips mean part of this
branch is demonstrably on `main` already, and the commit it choked on names
a **different bead** than the branch's own — which is the base, not the
deliverable. Both are visible without running anything.
