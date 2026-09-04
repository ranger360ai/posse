## The do-not-land verdict is monotone: the same block, filed a third time (ranger-base-avq12)

`docs/notes.d/ranger-base-4ri4n.md` records the class — a merge-back block
re-filed against a branch an earlier rescue already landed — and named the
thing that would repeat: *"a branch cut from a base `main` later rewrote will
re-file its merge-back block on every pass until the operator retires it."*

This is that prediction with a measured instance. Same branch,
`posse/gilfoyle-posse-ranger-base-nw9zg`, same five commits, same untouched
tree, third block: the original strand (rescued by **ranger-base-2yaud**,
2026-09-02), **ranger-base-4ri4n** (closed do-not-land, 2026-09-03), and this
one, filed the same day 4ri4n closed. The cause is filed as
**ranger-base-j8qmj**: the dedupe in `noteMergeBlocked`
(`internal/posse/dispatch.go:4688`) reads only OPEN beads, so closing a block
with a do-not-land verdict is exactly what lets the next sweep re-file it.

### What is new: the branch is drifting further from `main`, not closer

4ri4n compared the deliverable over the four paths above the branch's base and
found three byte-identical and one (`Makefile`) a strict superset on `main`.
Re-run today over all six paths in `<merge-base>..<branch>`, three now differ:

| path | branch vs `main` today | at 4ri4n |
|---|---|---|
| `docs/notes.d/ranger-base-nw9zg.md` | identical | identical |
| `gotestreuse_qa_test.go` | identical | identical |
| `scripts/gotest.sh` | identical | identical |
| `Makefile` | +15 / −2 on `main` | +15 on `main` |
| `internal/posse/memoryland.go` | +91 / −10 on `main` | not compared |
| `internal/posse/memoryland_credshapes_qa_test.go` | +7 / −4 on `main` | not compared |

The last two are the branch's **base** commit `34a27b4` (ranger-base-vd1bo),
which `main` re-landed as `6a230eb` and has since built on — so they were
never the deliverable and were correctly never picked.

### The reading that ends the question, in one direction

Ancestry, `git cherry` and patch-id all give a mixed or wrong answer here
(4ri4n has the table). The question that has a single answer is: **which lines
does the branch hold that `main` does not?** Sixteen non-blank lines, and every
one of them is a line `main` deliberately replaced:

```sh
B=posse/gilfoyle-posse-ranger-base-nw9zg
for p in $(git diff --name-only $(git merge-base main $B)..$B); do
    comm -23 <(git show $B:$p   | sort -u) <(git show main:$p | sort -u)
done
```

* `Makefile` — two lines, and **zero** branch-only *tokens*: `.PHONY` and
  `test:` both gained `verify-parallel` on `main` and lost nothing. A strict
  superset.
* `internal/posse/memoryland.go` — the pre-rename `rhq/personas` /`rhq/agents`
  comment, and the four-line `+++ b/` header reader that
  **ranger-base-y7i7k replaced with `diffHeaderPath`** — including the comment
  y7i7k's own fix quotes and calls wrong ("a reset would be a line no fixture
  can reach").
* `internal/posse/memoryland_credshapes_qa_test.go` — four digit-shaped Slack
  fixtures `main` re-spelled with `A`s.

So landing this branch is not a no-op that costs nothing. It would revert a
landed fix, a landed rename and a landed fixture decision. **Merge-back is
`--ff-only`, so it cannot actually happen** — but it settles the verdict in the
direction that matters: the answer is not "nothing to gain", it is "something
to lose", and it gets more true with every commit `main` takes. A do-not-land
verdict on a superseded branch never flips back, which is the argument for
j8qmj's closed-aware dedupe: re-deriving it costs a seat and can only ever
reach the same conclusion.

### Retiring the tree is the operator's, and it is the only exit

`RemoveSessionTree` asks `measuredOnBase()` — is EVERY commit in `base..tip`
accounted for by a **patch-id** match? — and ranger-base-as19 deliberately
honours only that arm, because a hand-resolved pick can drop a hunk. `34a27b4`
belongs to another bead and was never picked, so it has no account of either
kind and never will. The tree therefore refuses to retire without `--force`,
and `git worktree remove` is denied outright by Claude Code's auto-mode
classifier from a seat. Filed for the operator as **ranger-base-s0eo2**; the
tree is clean (`git status --porcelain` empty, HEAD `df434d8`), so the blast
radius of removing it is zero lines.
