## Ahead by sha is not ahead by work (ranger-base-g2xf)

`MergeSessionWork` counted a session's unlanded work with
`git rev-list --count <base>..<branch>`, which counts **shas**. A commit
cherry-picked onto the base keeps its own sha on the branch, so the branch
still counts as ahead by it.

Usually that costs nothing: the base moved, so the merge is not a
fast-forward, the launcher replays the branch onto it, and a non-interactive
`git rebase` **drops an already-upstream commit by patch-id** on its own.
The count collapses to zero and the branch fast-forwards.

It stops being free the moment the landing was **resolved by hand**. Then
the commit on the base is not the branch's patch: it is the branch's patch
plus whatever the human decided. The patch-ids differ, `git cherry` says
`+`, the rebase replays the commit into the same conflict every time, and
the pass reports

```
⚠ <bead>  1 commit(s) on <branch> did NOT reach main: main moved on and
  replaying <branch> onto it conflicts — the branch is untouched and still
  holds the work
```

which is **word-for-word the report for a real strand** (ranger-base-09b7's,
where the work genuinely is not on main). The operator could not tell "work
will be lost if this tree is retired" from "already landed, retire freely"
without cherry-picking by hand to see what happened, and the line came back
every pass forever.

Found in the field on `posse/gwart-posse-rangerhq-zag6`, whose one commit
`9b167c8` is on main in full as `2e9a9a8` — which says so, in git's own
words: `(cherry picked from commit 9b167c8b39cb45a…)`. The hand resolution
amended the ADR status line and kept main's newer flex row (`81bbcc7`,
ranger-base-uf3h), so the two patches differ by exactly that.

**The fix.** `equivalentOnBase(repo, base, tip)` in `worktree.go` asks
whether the base already holds the work of **every** commit `tip` is ahead
of it by, under other shas, and names the pairing when it does. Two ways a
commit's work gets there, and this bug needs both:

- **patch-id equivalence** — what `git cherry` measures and what the rebase
  drops unaided. One call for the whole branch, and the case that never
  reaches a human.
- **git's `-x` trailer**, `(cherry picked from commit <sha>)` — the only
  evidence left when the pick was resolved by hand, because the resolution
  amends the patch. Bounded to `<base> --not <tip>`, the only region a pick
  of that commit can be in.

It is asked **before** the rebase, so it also answers the arms the rebase
never reaches (a dirty tree, a checkout that moved), and it is asked of the
tree's HEAD rather than the branch for the same reason `landed()` is: the
branch is not always where the work sits (ranger-base-dybv).

**All-or-nothing, and nil is the honest default.** One commit it cannot
account for and the branch is a strand — the tree is still the only copy of
that commit — so the predicate refuses rather than returning the part it
recognised. `MergeOutcome.Equivalent` is non-empty only when the whole
branch is accounted for; `EquivalentNote()` is the one sentence that tells
the two reports apart, and every reporter — the landing sweep, `mergeBack`,
`posse worktrees --land`, the kill's `Line()` — prints ≡ for it instead of ⚠.

**The trailer is a record, not a measurement.** It says a human landed THIS
commit as THAT one; it cannot prove the resolution kept every hunk. So it is
only ever read as "the base holds this work", never as licence to throw the
branch away: `RemoveSessionTree` still asks its own question by sha and
still refuses, and the kill's line now prints both facts in that order so
the refusal reads as the belt it is. Retiring such a tree still wants
`--force`, or the by-sha guard taught the same question — filed separately.

Pins (`internal/rhq/worktree_test.go`):
`TestMergeSessionWorkTellsACherryPickedBranchFromAStrand` — four arms:
hand-resolved pick (trailer only), clean `-x` pick, plain pick with no
trailer (patch-id only), and the control, real unlanded work, which must
still strand in the unchanged words;
`TestMergeSessionWorkStillStrandsAPartlyLandedBranch` — one picked commit
and one that never left the tree is a strand;
`TestHandResolvedPickReallyDoesConflictOnReplay` — the fixture really is the
reported shape and not an easier one.

Mutation-checked, four ways: removing the arm reds the first three cases and
leaves the control green; removing the patch-id half reds only the
no-trailer arm; removing the trailer half reds only the hand-resolved arm;
turning the predicate's `return nil` into `continue` is invisible to every
single-commit arm and is caught only by the partly-landed one.

**A fixture trap worth keeping.** A cherry-pick onto a base that has *not*
moved rebuilds the **identical commit object** — same tree, same parent,
same message, same identity, therefore the same sha — which the base then
reaches outright, and no arm measures anything. Every arm moves the base
first.
