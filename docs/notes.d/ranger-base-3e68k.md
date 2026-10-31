# ranger-base-3e68k — merging two beads that both appended to the door register's head comment

Landed 2026-10-31, git 2.50.1 / macOS 26.4.1, go 1.26.10.

`ranger-base-pyl22` closed with one commit on
`posse/gwart-posse-ranger-base-pyl22` and `main` moved on underneath it:
`ranger-base-36nyh` and `ranger-base-bm6j8` landed in between. The launcher's
replay conflicted, aborted, and filed this bead. The replay recipe is
`ranger-base-xea2y`'s and is not repeated here — `merge-tree --write-tree`,
`cat-file blob` per path, no sequencer, nothing to leak. What is worth keeping
is what the conflict WAS, because it is the shape every future bead that
doors a pin will hit.

## The conflict was two beads appending to the same list, and one sentence that cannot be appended to

`git merge-tree --write-tree --merge-base=8077fef3 main 0271a7c0` named two
files:

```
CONFLICT (content): Merge conflict in Makefile
CONFLICT (content): Merge conflict in internal/treepins/treewidedoor_qa_test.go
```

Both are the SAME edit made twice. `ranger-base-bm6j8` doored two new
tree-wide pins and `ranger-base-pyl22` doored five, and a door is three
coupled places:

- **`$(QA_CORPUS_PINS)` in the Makefile.** A pure append on main's side and a
  pure prepend on the branch's, so the resolution is both: five names in
  front of main's twenty-three. Mechanical, and the only check that it was
  done right is arm 2 — a name dropped here leaves a pin undoored and arm 2
  says which.
- **`corpus-check`'s Makefile comment.** Both sides rewrote the same `~4s`
  line: main re-measured the door at 11.7s over twenty-three pins, the branch
  added a paragraph about its five. Both are true and they are about
  different things, so both stay — main's reading keeps the pin count it was
  taken at, with a clause saying the five landed after it.
- **The register's head comment.** Each side appended its own `and N more
  into <door>'s door` block, which merges by keeping both blocks, in landing
  order.

The part that does NOT merge is the sentence under those blocks:

> and `make tree-check` is all of them — 81-89s on this box over three warm
> runs at **eighty-three pins and fourteen doors**

`TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles` (arm 4) matches
**exactly one** `<n> pins and <n> doors` claim in the head comment and derives
both numerals from the Makefile. So taking either side's sentence is a red
suite, and taking either side's WALL figure under a corrected numeral is
worse: a green suite over a reading of a class that was never run. Each side
measured a class the other had not landed into — 78 pins on main, 81 on the
branch — and the merged class is 83.

**So the reading is re-taken here, and both sides' readings are demoted into
the history list.** That is the same resolution `ranger-base-v3tt4` reached
one merge-back earlier, for the same reason, and it is now the rule: a
merge-back that lands a doored pin re-measures `make tree-check`.

## The reading, taken twice, which is the useful part

MEASURED 2026-10-31, this worktree, warm cache, 83 pins behind 14 doors.

| set | run | wall | 1-min load before | suite slots held by siblings |
|---|---|---|---|---|
| A | 1 | 62s | 23.45 | 1 |
| A | 2 | 127s | 13.98 | 1 |
| A | 3 | 146s | 91.60 | 1 |
| B | 1 | 87s | 25.12 | 2 |
| B | 2 | 81s | 27.90 | 2 |
| B | 3 | 89s | 26.08 | 1 |

**Set B is the live reading — 81-89s — and set A is why.** A is a reading of
this exact code, and it was taken before the AGENTS.md sentence below went in,
which reds arm 4. A reading taken on a tree whose suite is red is not a
reading of this tree, so the three were re-taken. Nothing in the code moved
between them.

What the pair buys is worth more than the hour it cost. **Set A's spread is
84s and set B's is 8s, over the same class, the same doors and the same box,
an hour apart.** The difference between them is not the LEVEL of the load —
A's runs sat at 14-92 and B's at 25-28, overlapping — it is how much the load
moved while the command ran. Every "ordered by nothing in it" reading in the
head comment's history list says this from inside one set of three; this is
the first pair that says it directly. Price the command at ninety seconds
when the box is steady, whatever level it is steady at, and at two and a half
minutes when it is not.

Seven of the 83 pins are new and cost under a second between them (0.43s for
`ranger-base-pyl22`'s five, 0.24s and 0.21s for `ranger-base-bm6j8`'s two),
so neither spread is theirs.

This supersedes `docs/notes.d/ranger-base-pyl22.md`'s 74.5-172.1s, which is
a correct reading of that branch's class and not of main's.

## The trap that cost set A: the re-measured number has exactly one home

Arm 4 forbids AGENTS.md to COUNT this class — any numeral up to two plain
words before a plural `pins` or `doors` on that page reds it, because the
count is said in one place on purpose. A merge-back's job includes rewriting
that page's reading sentence, and the natural way to write it is

> the merge-back that landed ranger-base-pyl22's **five pins** onto a main
> that had meanwhile landed ranger-base-bm6j8's two … the **seven pins** those
> two beads added cost under a second between them

— two forbidden counts in one paragraph, neither of them a count of the class
and both matched anyway, because the rule reads the shape and not the
intent. Say `new pins` and `the pins those two beads added`. The wall figures
belong on that page; no numeral adjacent to `pins` or `doors` does.

This is a gate a merge-back hits and an ordinary bead does not, since an
ordinary bead has no reason to retype that sentence.

## One correction to prose this merge falsified

`ranger-base-bm6j8`'s demoted entry called its 22s spread "the SECOND widest
in this list". It is now sixth, and the merge is what made the list longer.
The entry keeps its finding — an idle box with no sibling suite still gave a
22s spread, which is the point — and drops the ranking. A comparative claim
about a growing list is held by nothing; the wall figures are.

## Verified

`make fmt-check`, `make register-check` (34.0s, the arm that holds the count),
`make tree-check` three times green at 87s/81s/89s on the tree as landed
(and three times before that, at 62s/127s/146s, on the same code), `python3
scripts/notes-index.py --check`, and `make test` whole.
