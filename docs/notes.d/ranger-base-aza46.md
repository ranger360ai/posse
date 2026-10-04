# A three-commit merge-back where two twins land no part of the conflicted file (ranger-base-aza46)

Landed 2026-10-04, git 2.50.1 (Apple Git-155) / macOS 25.4.0. The third note in
the replay series: `docs/notes.d/ranger-base-xea2y.md` measured the replay over
ONE commit, `docs/notes.d/ranger-base-eawjq.md` over two, and this branch —
`posse/holden-posse-ranger-base-4mrmc`, pinned at
`refs/posse/merge-blocked/…`, tip 4dc3a47e — carried three. Both earlier notes
hold unchanged. What is new is that **a twin's share of the conflicted file can
be nothing**, and the branch said so itself.

## The branch's own round 3 had already rebased the prose by hand

The three commits are rounds 1-3 of the ADR 0062 bob probe, all touching the
same two files. Between round 2 and round 3, `main` landed ranger-base-se81d,
-er6mt, -mkcsy and -4w7rk, which rewrote ADR 0062's Claims and ASSUMED lists
**with rounds 1-2's findings in them, in their own words**. Round 3 was then
written against main's text rather than the branch's, deliberately, and said so
in two places that a replayer reads before touching anything:

- 4dc3a47e's commit message: *"Rounds 1-2's own ADR edits are superseded and
  should not be re-landed; notes §7.7 says so on the record."*
- the fragment's own §7.7: *"What should land: this fragment … and the round-3
  ADR amendment. What should NOT: be07ed31's and 7bd14afa's edits to the ADR."*

MEASURED, and this is the reading that decides it: `git diff --numstat main
4dc3a47e` over the ADR is **105/11**, against **402/128** for round 3's own
diff against its parent. The tip's ADR *is* main's ADR plus round 3's
amendment; the 11 removed lines are all rewrites in place (one-line summary,
three sentence extensions, four struck ASSUMED headers), nothing of main's
dropped. The branch's two earlier ADR versions carry none of main's amendments
at all (`git merge-base --is-ancestor 6bd1a949 4dc3a47e` → false).

So the resolution is **not a merge of the ADR at all**: take 4dc3a47e's version
verbatim and check it against main, rather than resolving six conflict hunks by
hand and hoping the result matches what the author already computed.

## What the twins carry

eawjq's pairing rule is unchanged — `replayKey` is `(%ae, %aI, %s)` and
`equivalentOnBase` collapses to nil on the first unaccounted sha — so three
stranded commits still need three twins. But pairing reads none of the content,
and the content that is honest here is lopsided:

| twin | original | ADR | fragment |
|---|---|---|---|
| 1 | be07ed31 | nothing — superseded on main | 171 lines, new file |
| 2 | 7bd14afa | nothing — superseded on main | +264/-7 |
| 3 | 4dc3a47e | 4dc3a47e's version verbatim (+105/-11 vs main) | +232 |

Twins 1 and 2 are fragment-only commits. That is not a collapse of the history
— each still says which round found what, which is eawjq's argument for
separate commits — and it is the only shape that does not re-land prose main
already supersedes.

## A cherry-pick of the TIP reports a conflict the branch does not have

The bead carried a prior measurement that **both** files conflict, taken by
`git cherry-pick 4dc3a47e` in a scratch worktree. Reproduced, with the base
made explicit:

```
$ git merge-tree --write-tree --merge-base=7bd14afa main 4dc3a47e   # the tip alone
CONFLICT (content): Merge conflict in docs/adr/0062-…md
CONFLICT (modify/delete): docs/notes.d/ranger-base-4mrmc.md deleted in main and
  modified in 4dc3a47e.

$ git merge-tree --write-tree --merge-base=4dff757a main 4dc3a47e   # the branch
CONFLICT (content): Merge conflict in docs/adr/0062-…md
```

The fragment's conflict is an artifact of the base, not a divergence: a
cherry-pick's base is the commit's PARENT, which holds rounds 1-2's fragment,
so a `main` that never had the file reads as having deleted it. Over the
branch's real merge-base the fragment is an add on one side only and merges
clean.

**So read the conflict set with `merge-tree --merge-base=$(git merge-base main
<tip>)`** — `mergesCleanly`'s own operand, which is what the block's recipe
prints — and treat a cherry-pick's extra `modify/delete` on a file main does
not have as noise. Believing it costs a hand-merge of a file with one author.

## The generated index was stale again

Third merge-back in a row: `docs/notes.d/README.md` is generated, the branch
added `ranger-base-4mrmc.md` without regenerating it, and `make notes-check`
was red on the replayed tree before `python3 scripts/notes-index.py` ran. It is
regenerated under THIS bead, with this fragment, not inside a twin — the twins
keep the originals' subjects, and a generated file rebuilt there would be
attributed to a commit that did not rebuild it.

## Verified

Pairing, with the sweep's own fields — all three `paired`:

```
for s in be07ed31 7bd14afa 4dc3a47e; do
  k=$(git log -1 --format='%ae%x1f%aI%x1f%s' $s)
  git log --format='%H%x1f%ae%x1f%aI%x1f%s' HEAD --not main | grep -Fq "$k" && …
done
```

Content: both replayed paths byte-identical to 4dc3a47e
(`diff <(git cat-file blob "4dc3a47e:$p") "$p"`); no conflict markers in the
tree; `git diff --diff-filter=D --name-only main $TREE` empty on all three
trees, so the write-and-add loop missed no deletion. `make fmt-check`,
`make notes-check`, `make tree-check` (all fourteen doors). No sequencer state
was created — no `cherry-pick`, `rebase` or `revert` ran in this worktree.
