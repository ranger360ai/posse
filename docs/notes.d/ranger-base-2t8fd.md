# ranger-base-2t8fd — the merge-back whose conflict was the stale figure itself

Landed 2026-11-22, linux 7.2.15-1-omarchy, git 2.56.0, go1.27.3.

`ranger-base-yem73` closed with two commits on
`posse/dinesh-posse-ranger-base-yem73` and `main` moved on underneath it. The
launcher's replay conflicted, aborted, and filed this bead. Both sides touched
one file — `internal/treepins/wallmemberaccount_qa_test.go` — and `main`'s side
was `ranger-base-x1ofd` (`0687d302`), which added arm 2's FOURTH account route.

Replayed per the block's recipe: `git merge-tree --write-tree
--merge-base=$C^ HEAD $C` per original, oldest first, one commit out per
commit in, each carrying its original's author identity, AUTHOR date and
subject. No rebase and no cherry-pick, so no sequencer marker to leak
(AGENTS.md, "Landing the plane"). The pairing set was read back with the
block's own `diff <(git log --format='%ae%x1f%aI%x1f%s' …)` and matches.

## git reported two conflicts; the file had three

The marked conflicts were both the obvious kind — two Logf lines each side had
edited, where `yem73` turned a COUNT into a LIST and `x1ofd` added a route to
the same call. Those resolve by taking both: the parameter route's `%d` and the
list's `%v` in arm 2, the listed `inHelper` with `9 of 66` in arm 6.

**The third conflict git merged cleanly and it was a contradiction.** `x1ofd`
inserted a head-comment paragraph ending *"it is the NINTH of the nine messages
the paragraph below charges"*; `yem73` rewrote that very paragraph, and its
rewrite says *"EIGHT of the 65 messages arm 6 already grades"*. The two
paragraphs do not overlap by a single line, so there was no marker and no
prompt — the merged file simply said eight in one place and nine in the other,
one paragraph apart.

That is this file's own subject taken a fourth time. `yem73` was filed because
two figures were read at one tip and typed at another; the merge that landed it
produced a third figure false at the tip it landed on, by the mechanism neither
bead could see: **each side's figure was true of its own parent and of no tree
that would ever exist.** A textually clean merge is not a true one when the
conflicting content is a number.

What it cost to find: nothing but reading the merged head comment with the two
diffs beside it. What it would have cost not to: the fifth instance.

## The live reading is the resolution, and it was available the whole time

Both figures came back from one `-v` run on the merged tree, which is what
`yem73` added the lists for:

```
go test -v -run 'TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine|TestQAEveryWallMemberAccountFitsInsideTheEvidenceBudget' ./internal/treepins
```

MEASURED 2026-11-22 at the merged tip (`418b49c0`): 24 member files, 8 carrying
an account, **66** account messages — 32 runner local, 8 folded local, 25
inline fold, 1 parameter — **6** deferring prose to a second line, **9 of 66**
rendered inside a `t.Helper()`, and the widest head 209 at
`internal/posse/bwrapgitrefslive_test.go:624`. Every one of those is a number
one of the two sides had typed as prose; the run settles which without
arithmetic and without a call graph.

Arm 1's paragraph (the second original's whole content) merged clean and its
figures were re-read the same way: 24 files, 33 launching functions, 32
argv-entry, 2 wrapper, 40 child errors — identical at the merged tip to what
`yem73` read at `814776ed`, so that paragraph needed no edit at all.

**So the rule a merge-back inherits from this file is: resolve the markers,
then re-read every figure the merged prose claims, from the arms' own Logf.**
A figure is the one kind of conflict that does not announce itself.

## What was left

One paragraph on `main` is stale by one and this landing did not touch it:
*"AND THE TERM IS STILL WITNESSED HERMETICALLY"* charges *"those eight 27 to
41"* and reads *"nine of the 65 rather than eight"* for the sloppier helper
reading. `x1ofd` added a ninth helper-rendered message and a 66th graded one
and left that paragraph alone, so it is `x1ofd`'s residue and not this merge's
— neither original touched it, and widening a merge-back to fix it would put a
third party's prose under this bead's id. Filed instead.

## Not closed here

- ranger-base-vyz61
