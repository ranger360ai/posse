# ranger-base-dmp4x — the second recurring add/add site in a merge-back: `## Unreleased`

Replayed 2026-10-09, git 2.50.1 / darwin 25.4.0.

`ranger-base-wmaf9` closed with one commit on
`posse/gwart-posse-ranger-base-wmaf9` (`8268e364`) and `main` moved on under
it by seven commits. The launcher's replay conflicted, aborted, and filed this
bead. `docs/notes.d/ranger-base-xea2y.md` is the method — `merge-tree
--write-tree`, write the blobs, resolve, commit path-limited, no sequencer —
and it held unchanged. What this pass adds is a second conflict site and a
correction to xea2y's verification recipe.

## CHANGELOG.md's `## Unreleased` conflicts the same way README.md does, and is NOT generated

The whole conflict was two add/add hunks, both at the head of a list:

```
Auto-merging CHANGELOG.md
CONFLICT (content): Merge conflict in CHANGELOG.md
Auto-merging docs/notes.d/README.md
CONFLICT (content): Merge conflict in docs/notes.d/README.md
```

xea2y names the second and says it "will recur on every merge-back whose
branch carries a notes fragment, which is most of them". The first recurs on
the same footing and is the opposite kind of file. `docs/notes.d/README.md`
is generated, so it is regenerated (`python3 scripts/notes-index.py`, then
`--check`) and a hand-merge of it is a guess at the script's sort order.
`CHANGELOG.md` is prose nobody generates, so regenerating is not available and
the hand-merge is the only resolution there is.

**The rule for it is the one the file's own head states: keep both entries,
newest first.** Both sides add a paragraph directly under `## Unreleased`,
separated from its neighbour by a blank line, and the order between them is by
AUTHOR date — here the work at 12:32 above `main`'s at 11:06, both 2026-10-09.
Neither side's text is touched; the conflict is only about which block comes
first.

## xea2y's verification recipe reports a false difference once a block MOVES

xea2y checks that a textually clean auto-merge lost no hunk by diffing the
whole hunk text two ways: the merged file against the work commit should be
the main side's own change, and against main should be the work's own. That
recipe is exact for a file whose surrounding lines did not move. It is not
exact for the file you just reordered: a hunk's leading and trailing CONTEXT
is now the OTHER block, so both diffs report a difference that is entirely
context and no added line.

MEASURED here on `CHANGELOG.md`: three context lines differ in each direction
and the recipe exits 1 on a resolution that is correct.

**The invariant is the ADDED lines, so compare only those** — the one reading
that cannot be moved by a reorder:

```
diff <(git diff --no-ext-diff main HEAD -- <p> | grep '^+') \
     <(git diff --no-ext-diff <base> <work> -- <p> | grep '^+')
diff <(git diff --no-ext-diff <work> HEAD -- <p> | grep '^+') \
     <(git diff --no-ext-diff <base> main -- <p> | grep '^+')
```

Both identical means the file holds each side's additions verbatim and
nothing else. Use xea2y's full-hunk form on a file that auto-merged — there
the context is evidence too, and `internal/posse/gates.go`, the one `.go`
file on both sides of the base here (`+45/-?` from the work, `+92/-?` from
main), passed it identically in both directions.

A third reading is cheap and catches what neither diff does: the committed
tree against the merged tree.

```
git diff --stat HEAD <TREE>
```

It must name exactly the files you resolved by hand and nothing else —
here `CHANGELOG.md` and `docs/notes.d/README.md`, with the README's three
lines being the conflict markers alone.

## Verified

`git diff --diff-filter=D --name-only main <TREE>` empty (the replay loop
only writes and adds, so a deletion is the one thing it would miss),
`scripts/notes-index.py --check`, `make fmt-check`, `make tree-check`,
`go vet` clean under all three arm tags, and `make test` whole.
