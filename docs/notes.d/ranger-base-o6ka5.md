## A merge-back whose conflict was two sides of one map, and where the bead id goes (ranger-base-o6ka5)

Landed 2026-10-04. `ranger-base-r5546` closed with one commit, `eb130781`, on
`posse/gwart-posse-ranger-base-r5546`; `main` moved to `2c328504` under it and
the launcher's rebase conflicted and aborted. The replay mechanism is
`docs/notes.d/ranger-base-xea2y.md` and nothing here changes it. Three things
this pass adds.

### The conflict was twdDoorHolders, from both sides at once

`internal/treepins/treewidedoor_qa_test.go` holds `twdDoorHolders`, the one
way into a door variable that the enumeration rule does not derive — a map
literal of test name to reason. Both sides edited it in the same pass of the
shop:

- `ranger-base-hrf47` added a THIRD row,
  `TestQABoxCheckCensusCoversEveryVerifyTarget`, with a dozen lines of reason
  above it. It reached `main` through `ranger-base-yrag8`'s own merge-back.
- `eb130781` rewrote the SECOND row's description, because
  `ranger-base-r5546`'s F3 made arm 4 read the treepins README's door table
  as well as this file's head comment.

Adjacent lines of one literal, disjoint in meaning, so the resolution keeps
both — the added row and the rewritten description. What makes that a reading
rather than a guess is `ranger-base-xea2y`'s check, and it is worth repeating
how cheap the answer was. Diff the merged file against the WORK commit, and
diff that against the other side's own `base..main` diff of the same file:

```
diff <(git diff --no-ext-diff eb130781 -- internal/treepins/treewidedoor_qa_test.go | sed -n '/^diff --git/,$p') \
     <(git diff --no-ext-diff f354346c main -- internal/treepins/treewidedoor_qa_test.go | sed -n '/^diff --git/,$p')
```

Three lines of output: the blob hashes, one `@@` offset, and exactly one
context line — the arm-4 row, carrying the branch's description instead of
`main`'s. That is "the merged file is `main`'s version plus every hunk the
branch had, and nothing else", said by a tool rather than by a compile. A Go
file that auto-merges and builds says nothing about a dropped hunk; this says
it.

The merged tree's own doors were then the verification, per
`ranger-base-yrag8`'s rule: arm 4 and `TestQABoxCheckCensusCoversEveryVerifyTarget`
both green on the resolved map, which is the two-way check accepting three
holders and the new README table clause in one run.

### main moves during the replay, and a replay is cheap to redo

`main` went to `4384ac7f` — `ranger-base-zt45t`, `-cu0zg`, `-o1aoi` — between
the first replay being verified and its commit being written, so the branch
stopped fast-forwarding and the whole pass had to be done again on the new
base. That is not a race worth trying to win: three seats were working, and a
`make test` on this box is twenty-odd minutes during which `main` can move
twice.

What makes it survivable is that **a replay is not a decision, so redoing one
costs nothing it cost the first time.** `git reset --hard main` in the session
worktree (the old commits stay reachable by sha), the same `merge-tree
--write-tree` against the new base, the same `cat-file blob` loop — and the
seven replayed blobs came out BYTE-IDENTICAL to the first pass's, checked with
`git hash-object` against the first commit's entries. Only the generated index
differed, because only the generated index is a file `main`'s own commits also
touch. So the second pass cost one `notes-index.py` run and a re-measurement:
the two conflicts were resolved once, and the readings were re-taken because a
reading names a tree.

The ordering lesson is the cheap one. Commit the replay BEFORE the long suite,
not after — the commit is what stops the next launcher pass re-filing the
block, and a suite result is a claim about a tree that is still true if the
tree is already committed. The first pass committed after a 76.5s door run and
before the 20-minute suite, which is why `main` only cost it a redo and not
the work.

### The bead id goes in the BODY, and the subject stays the branch's

The block bead asks a replay to keep the original commit's author identity,
AUTHOR date and subject. That is not etiquette: `replayKey` in
`internal/posse/worktree.go` is `%ae`/`%aI`/`%s`, and it is the third of
`equivalentOnBase`'s three arms — the only one that can account for a
hand-resolved REBASE, because a rebase writes no `-x` trailer and a resolution
changes the patch, so `git cherry` says `+`. Change any of the three and the
branch reads as a strand and the block is re-filed.

But this shop's provenance is the bead id in the subject, and a replay has two
beads: the work's and the block's. They do not conflict, because
`git log --grep` reads the whole message. So the subject is the WORK bead's,
byte for byte, and the replay bead names itself in the body — which keeps the
pairing key intact and still answers `git log --grep ranger-base-o6ka5`. The
replay's own notes fragment is a separate commit under the replay bead's own
subject, so the faithful replay stays one commit that pairs.

### Verified

`make fmt-check`; `make tree-check` all fourteen doors green, 76.5s at a
one-minute load average of 8.3 with a sibling seat holding one suite slot;
`go vet` over both changed packages; `scripts/notes-index.py --check`; arm 1's
four touched readings pins 4/4 PASS; arm 2's
`TestQASettledHolderRacingTheScanIsSkipped` PASS. Everything below was read
TWICE, once per base, and both readings are here because the second is the
live one and the first is what says the redo changed nothing.

On `2c328504`: `make tree-check` fourteen doors green 76.5s at a one-minute
load average of 8.3 with a sibling seat holding one suite slot; `make test`
exit 0, no FAIL — arm 1 `cmd/posse` 216.702s, `cmd/testparallel` 1.591s,
`internal/posse` 430.817s, `internal/treepins` 516.024s; arm 2 268.399s; arm 3
298.849s; silent-revert audit 2011 commits, 0 untriaged; `cmd/checkorphans`
clean.

On `4384ac7f`, the base this landed on: `make tree-check` fourteen doors green
55.7s at a one-minute load average of 7.6 rising to 8.1, both suite slots
free; the touched pins green again at 1.1s and 1.4s; and `make test` whole,
exit 0, no FAIL — arm 1 `cmd/posse` 166.844s,
`internal/posse` 300.594s, `internal/treepins` 347.583s (`cmd/testparallel`
cached); arm 2 257.8s; arm 3 265.354s; silent-revert audit 2014 commits, 0
untriaged. `cmd/checkorphans` clean after both runs.

Every arm came in faster on the second base than the first — 166.8 against
216.7, 300.6 against 430.8, 347.6 against 516.0, 257.8 against 268.4, 265.4
against 298.8 — on a tree whose only difference in this branch's paths is the
generated index. That is the box, which is the same conclusion the head
comment of `internal/treepins/treewidedoor_qa_test.go` keeps re-reading over
`make tree-check`: the first run went up against a sibling seat's suite, the
second had both slots to itself.
