## A second superseded branch, and the reading that ends it in one direction (ranger-base-gn0ch)

`posse/jian-yang-posse-ranger-base-9a53x` has produced two merge-back blocks
against one untouched, one-commit branch:

  ranger-base-fpqj4   closed do-not-land, comment only   2026-08-31
  ranger-base-gn0ch   re-filed by a later sweep          2026-09-04

This is the class `docs/notes.d/ranger-base-4ri4n.md` named and
`docs/notes.d/ranger-base-avq12.md` measured on
`posse/gilfoyle-posse-ranger-base-nw9zg`; the cause is **ranger-base-j8qmj**
(the dedupe in `noteMergeBlocked` reads only OPEN beads, so closing a block
with a do-not-land verdict is exactly what lets the next sweep re-file it).
Recorded here because the two branches fail differently and this one is the
cleaner specimen: a **single** commit, a **single** path, and a deliverable
`main` holds in full.

### The branch's whole content is on `main`; three instruments still say it is not

`ranger-base-9a53x` fixed two reds by appending nine `sha256` digests to
`internal/posse/exampledigests.go`. gwart's **ranger-base-ez7s4** fixed the
identical defect first and landed on `main` as `318e1d9`. Both append the same
nine digest values.

The line-level reading of avq12 gives nine branch-only lines:

```sh
B=posse/jian-yang-posse-ranger-base-9a53x
for p in $(git diff --name-only $(git merge-base main $B)..$B); do
    comm -23 <(git show $B:$p | sort -u) <(git show main:$p | sort -u)
done
```

The token-level reading gives **zero**:

```sh
P=internal/posse/exampledigests.go
comm -23 <(git show $B:$P   | grep -o '"[0-9a-f]\{64\}"' | sort -u) \
         <(git show main:$P | grep -o '"[0-9a-f]\{64\}"' | sort -u)
```

| | branch | `main` | branch-only |
|---|---|---|---|
| digest **lines** differing | 9 | — | 9 |
| digest **tokens** | 78 | 106 | 0 |

All nine differences are the same digest carrying a comment `main` spells
differently — a twelve-character sha and a semicolon where `main` writes eight
and a parenthetical:

```
branch: "a556a7ad…", // 9c00e1926551 2026-08-31 rename internal/rhq -> internal/posse; $RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR
main:   "a556a7ad…", // 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)
```

That trailing comment is the entire delta, and it is what makes every ancestry
instrument report a strand: `merge-base --is-ancestor` says NOT-IN-MAIN,
`git cherry main $B` prints `+`, the two patch-ids differ
(`1bd09ede` vs `85bdd808`), and the replay the block quotes conflicts. **A
comment-only difference is enough to defeat all four.** Only the two readings
above answer the question the block is actually asking.

### The deliverable, checked where it has to be true

Both tests `ranger-base-9a53x` was opened for pass on `main`
(this worktree, HEAD `0e78fae`):

```
go test ./internal/posse/ -count=1 -run \
  'TestEveryEmbeddedExamplePIDIsInTheShippedTable|TestShippedExampleTableCoversEveryVersionInGitHistory'
--- PASS: TestEveryEmbeddedExamplePIDIsInTheShippedTable (0.00s)
--- PASS: TestShippedExampleTableCoversEveryVersionInGitHistory (1.89s)
```

### The verdict is monotone here too, and for a sharper reason

At `fpqj4` the two files held the same nine digests. Today `main` holds
**twenty-eight** digests the branch has never seen — `e045c0d`, `a5a7cbc` and
`899548b` each appended more since. The branch is a strict subset that shrinks
with every seeded-PID change, so a do-not-land verdict on it can never flip.
Landing it gains nothing and would re-spell nine comments backwards; merge-back
is `--ff-only`, so it cannot happen by accident.

### The exit is the operator's

The tree is clean (`git status --porcelain` empty, HEAD `c8df48d`).
`RemoveSessionTree` asks `measuredOnBase()` for a **patch-id** account of every
commit in `base..tip`, and `c8df48d`'s patch-id differs from `318e1d9`'s for
the comment reason above — so the refusal to retire is permanent, exactly as it
is for `nw9zg` (avq12), by a different route: there it was an unpicked base
commit, here it is a landed commit whose bytes were re-spelled. `posse
worktrees` will keep printing `1 commit(s) not on main, for ranger-base-9a53x`
until the tree is removed by hand. Filed for the operator as
**ranger-base-enaij**, sibling to **ranger-base-s0eo2**.

Fourteen of the trees `posse worktrees` lists carry an unlanded-looking commit
today; if the operator would rather rule once than twice, the two beads are the
same ruling.
