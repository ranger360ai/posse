## Fact 2's second instrument: a whitespace-exact patch-id twin (ranger-base-06y60)

ADR 0058 D1 fact 2 says a session tree may be retired only when the base
holds the branch's bytes. As built on 2026-09-05 that was one question —
`contentNotOnBase`, the blob walk ranger-base-x8jp added — and the amendment
of 2026-09-06 (ranger-base-lwd29) measured what it covered: **nothing**.

The reason is structural, not a corner. A tree only reaches the `≡` row
because its landing was **not a fast-forward**, and a landing that is not a
fast-forward writes a NEW blob for every file the base moved in the meantime.
For an append-heavy file (CHANGELOG.md, INSTALL.md, NOTES.md) the base has
always moved it, so the branch's blob is on the base **nowhere, on any
commit**, and the tree is kept on every pass forever — for bytes that are on
main line for line. That every-pass line is the thing ADR 0058 was written to
end.

**The second instrument** (`baseHoldsBytes`, internal/posse/worktree.go):
every commit in `base..tip` has a **whitespace-exact** patch-id twin among
the base's own commits in `tip..base`.

```
git log -p --no-ext-diff --no-renames <range> | git patch-id --verbatim
```

One process pair per side, one id per non-merge commit, `"<id> <sha>"` per
line. `--verbatim` is the whole point: plain `git patch-id` **normalises
whitespace**, which is exactly the hole x8jp opened the blob walk over, and
`--verbatim` is git's own flag for not doing that. Context lines are hashed,
so a twin is the same hunks against the same neighbours, byte for byte; lines
the branch never touched are not its to lose, the rule the blob walk already
states for paths.

**Either licenses**, and the blob walk stays the first one asked: it is the
cheap common answer, and the refusal's per-path `git -C … diff` pointer is
its. The twin walk is paid ONLY by a tree that already passed patch-id and
failed the blob walk — 970 ids over a 987-commit range in 5.5s wall
(measured 2026-09-06 on olwk).

**Both arms fail closed.** A commit the range form prints no id for is a
merge (or an empty commit) and is unmeasured, never paired — `git log -p`
prints no patch for a merge, and a merge's own resolution is the one thing on
a branch that exists nowhere else by construction. A git that rejects
`--verbatim` (it is **2.39+**, and it cannot be combined with `--stable`) is
an error, and both callers read an error as a keep.

**One helper, two callers.** `heldByTip` (RemoveSessionTree's refusal) and
`treeHolds` (the same refusal asked as a question by the reap, the retire
sweep and `posse worktrees --retire`) both ask `baseHoldsBytes`, so their
answers about one tree cannot drift —
`TestRetireGuardsSeeADetachedTreesWork` and the arms in
`internal/posse/verbatimtwin_test.go` hold them to it. Both refusals now name
the commit the base has no exact twin for **and** the paths whose bytes
differ.

### The wrong arms are the point

`internal/posse/verbatimtwin_test.go` pins the eight arms the ADR measured,
and two of them are the ones that decide whether any of it means anything: a
hand landing that **re-indented** (x8jp's own shape) and one that added
**trailing whitespace** must both be KEPT. A twin measured with `--verbatim`
dropped calls both equivalent and deletes the last copy of those bytes.
Mutation-checked on the way in, and each mutant reds only its own arms:

| mutant | reds |
|---|---|
| `--verbatim` dropped from `patchIDsVerbatim` | the re-indent and trailing-whitespace arms, and nothing else |
| the twin arm removed from `baseHoldsBytes` | the two arms that retire on it (the clean pick past the hunk's context, and the same under later base edits) |
| a missing id read as "nothing to compare, carry on" | the merge arm |
| an error from `verbatimUnpaired` read as held | the git-floor arm |
| the twin lookup written as a set instead of a count | the add/revert/add arm |

The count is the one rule here that is not the ADR's. One id can belong to
two commits ahead — add X, take X back, add X again — and a base holding ONE
copy of it holds one of them; a set pairs both against that single twin. A
twin is consumed by the commit it pairs.

The git floor has its own arm and it cannot be overlaid: the flag goes to a
binary found on PATH, so a PATH shim rejecting `--verbatim` (and exec'ing the
real git for every other argv) is the only seam. It is the one non-parallel
test in the file for that reason.

A note for the next reader of `docs/notes.d/ranger-base-f6lk.md` and
NOTES.md's `residueHolds` paragraph: both say the licence is patch-id
equivalence AND the base holding the branch's bytes, which is still true —
what changed is that "holding the branch's bytes" is now two measurements
and either answers it.
