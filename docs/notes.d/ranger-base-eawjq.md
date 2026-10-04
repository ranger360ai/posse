# Replaying a TWO-commit merge-back: every commit needs its own twin (ranger-base-eawjq)

Landed 2026-10-04, git 2.50.1 / macOS 25.4.0. The companion to
docs/notes.d/ranger-base-xea2y.md, which measured the replay itself over a
branch of ONE commit. This branch — `posse/gilfoyle-posse-ranger-base-bwp7h`,
pinned at c14e2092 — carried two, and that difference has a consequence the
one-commit note could not show.

## The block's recipe says "keep that commit's identity"; with two commits it is each of them

`equivalentOnBase` (internal/posse/worktree.go) loops over every sha in
`git rev-list <base>..<tip>` and returns **nil the moment one is
unaccounted-for** — nil is "the branch is ahead by real work", which is a
strand. A replay writes no `-x` trailer and its resolution changes the patch,
so for a replayed commit neither the patch-id arm nor the trailer arm fires:
the only arm left is `replayKey`, `(%ae, %aI, %s)`, paired against
`replayIndex`'s map over the same bound.

So a two-commit branch replayed into **one** commit lands all the content and
still reads as stranded, because the other sha pairs with nothing and the
whole answer collapses to nil. The fix is not subtle once seen — land one
commit per stranded commit, each carrying that commit's author identity,
AUTHOR date and subject:

```
git commit --date='<%aI of the original>' -F <msg> -- <paths>
```

`--date` sets the AUTHOR date, which is what `replayKey` reads; `%cI` is the
replay's own and is expected to differ. `--amend` ignores `GIT_AUTHOR_DATE`,
which is the other reason not to reach for it here (and in the shared checkout
it is forbidden outright — AGENTS.md, "Landing the plane").

Building the two in order costs one extra `merge-tree`, not a sequencer:

```
TREE1=$(git merge-tree --write-tree --merge-base=<base> main <first>  | head -1)
TREE2=$(git merge-tree --write-tree --merge-base=<base> main <second> | head -1)
```

`<base>` is `git merge-base main <tip>`. Write `$TREE1`'s blobs, resolve,
commit as the first twin; write `$TREE2`'s blobs, resolve, commit as the
second. Verify the pairing before closing — this is the check, and it reads
the same fields the sweep does:

```
for s in <first> <second>; do
  k=$(git log -1 --format='%ae%x1f%aI%x1f%s' $s)
  git log --format='%H%x1f%ae%x1f%aI%x1f%s' HEAD --not main | grep -Fq "$k" \
    && echo "$s paired" || echo "$s NO TWIN"
done
```

## The conflict was a .PHONY list, and the resolution is a union

One hunk, in the Makefile, both times: this branch adds `verify-nodb-defer` to
`.PHONY` while main's same-day landings added seven other targets there
(ranger-base-7zng1's `treepins`, 1a0hi's and g6sb1's tree-wide doors). Two
sides appending to one long line is the Makefile's version of the generated
index in ranger-base-xea2y — not a semantic conflict, and not a place to
choose a side.

Resolve it as a **set union**, computed rather than retyped: split both
sides on whitespace, assert that each side's exclusive targets are the ones
you expect, then splice. Doing it by eye over a 60-target line is how a
sibling's target goes missing silently, and ranger-base-emgdb records exactly
that failure on a shared `.PHONY` list. Two readings say it held:

```
git diff --no-ext-diff main -- Makefile | grep -c '^-'   # 2: the ---  header, and the one .PHONY line
```

and every other replayed path byte-identical to the work commit
(`diff <(git cat-file blob "<tip>:<p>") <p>`), which was true of all four here.

## The branch's own notes fragment was never in the index

`docs/notes.d/README.md` is generated, and this branch added
`ranger-base-bwp7h.md` without regenerating it — so `make notes-check` was red
on the replayed tree and would have been red on main. A merge-back inherits
that: the branch's `make test` predates main's current doors, so run
`make tree-check` on the replayed tree and fix what it finds under YOUR bead,
not the original's. Regenerating is `python3 scripts/notes-index.py`.

## The twins are not interchangeable, and running them proved it

The first twin's `make verify-nodb-defer` exits **2** — the door named no store
and the script's "finding nothing over zero stores is not a pass" rule fires.
That is the defect c14e2092 fixed with `--arm-a-only`, and the second twin
exits 0. Worth stating because it is the argument for two commits beyond the
pairing: collapsing them would have published a green door over a tree whose
first half could not produce one, and the history would no longer say which
half was which.

## Verified

`make fmt-check`, `make tree-check` (all fourteen doors, 52.6s),
`make verify-nodb-defer`, `make notes-check`,
`go test ./internal/treepins -run 'TestQAMake|Armtags|ArmTypeChecks|TestQATheWrapper|TestQAMakefile'`
(12 pins, 17.7s). The pins monica's note placed at the repo root live in
`internal/treepins` now; there are no `*_test.go` files at the root.
