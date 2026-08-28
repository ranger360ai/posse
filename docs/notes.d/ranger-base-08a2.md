## The shared-index guard exempts where git refuses a pathspec, not where an operation is in progress (ranger-base-08a2)

`sharedIndexBody`'s git-driven exemption was written as *an operation is in
progress* — `MERGE_HEAD CHERRY_PICK_HEAD REVERT_HEAD rebase-merge
rebase-apply`, plus a `case "$2" in merge|squash) exit 0` arm ahead of it —
and justified as *git refuses a pathspec during those outright, so a refusal
would have no safe form to point at*.

**The premise holds for two of the five.** Measured on git 2.39.3 / macOS
26.4.1, each state probed for what git accepts and for what the hook is
handed:

| state | pathspec | git's own completion at the slot | verdict |
|---|---|---|---|
| `MERGE_HEAD` (conflict) | fatal | `$2=merge`, or `message` with `-m` | exempt |
| `MERGE_HEAD` (`--no-commit`) | fatal | `$2=merge` | exempt |
| `CHERRY_PICK_HEAD` | fatal | `$2=message` | exempt |
| `rebase-merge` | **accepted** | `$2=message`, `$GIT_DIR/index`, no marker | exempt, residual |
| `REVERT_HEAD` (conflict) | **accepted** | `$2=merge` | **refused** |
| `REVERT_HEAD` (`--no-commit`) | **accepted** | n/a | **refused** |
| `SQUASH_MSG` (`$2=squash`) | **accepted** | n/a | **refused** |

So during a revert the exemption bought nothing and cost the wall:

```sh
$ git revert --no-commit <sha>                       # REVERT_HEAD now present
$ git add <another persona's paths> && git commit -m x   # exempted, swept
```

— rangerhq-nyqj exactly, and the window is the one rangerhq-lrnp's own
blessed recipe opens. `git merge --squash` is the same hole one arm over.

**Nothing is stranded by removing them,** which was the open question on the
bead. `git revert --continue` reached the slot as `$2=merge`, so it was the
`case "$2"` arm carrying it, not `REVERT_HEAD`; and a path-limited commit
*finishes a conflicted revert outright* — verified end to end: `REVERT_HEAD`,
`MERGE_MSG` and `AUTO_MERGE` all cleared, tree clean, and a following
`git revert --continue` correctly answering `error: no cherry-pick or revert
in progress`. That is the same two-step way through the refusal already
names, so the verdict on `git revert --continue` is now consistent with the
verdict rangerhq-lrnp reached for a clean `git revert`.

**Merge and cherry-pick keep their exemption on its stated merits.** git's
own `fatal: cannot do a partial commit during a merge` / `during a
cherry-pick` is pinned rather than quoted, in both the conflicted and the
`--no-commit` state, so if it ever stops being fatal the exemption is owed a
re-measurement. Note the marker and not `"$2"` is what has to hold them: a
persona finishing a conflicted merge with `git commit -m mine` arrives as
`$2=message`.

**The residual, stated plainly: during a rebase the wall is down.** A
pathspec is accepted mid-rebase, but a rebase has commits left to replay, so
`git rebase --continue` is the only way on and it is indistinguishable from a
typed `git commit` at this slot. `GIT_REFLOG_ACTION=rebase (continue)` *does*
discriminate (measured: unset for a hand commit in the identical repo state)
and is unusable for the same reason `GIT_INDEX_FILE`'s spelling was — it is
the caller's to set (rangerhq-cqq1). It is bounded by the crew PIDs, which
forbid rewriting history in the shared checkout at all.

`REVERT_HEAD` is now free to *word* a refusal, which is all it was ever safe
for: a revert finished with a message of the persona's own gets an arm naming
what is staged and that the path-limited commit ends the revert.

Pinned in `internal/rhq/gateschain_qa_test.go`,
`TestQAGuardExemptsOnlyWhereGitRefusesAPathspec` — seven arms, one repo each,
mutation-checked: re-adding `REVERT_HEAD`, re-adding the `case "$2"` arm, and
dropping each of `MERGE_HEAD` / `CHERRY_PICK_HEAD` / the rebase loop / the
wording arm each fail their own subtest and nothing else.

This supersedes the NOTES.md paragraph beginning *"Commits git drives itself
are let through when git leaves a marker to see — merge (`$2` = `merge`),
cherry-pick, rebase, squash, and a revert being finished by hand"*: squash and
the hand-finished revert are no longer in that set.

ADR 0002's acceptance criteria still hold as written — §14's three
(`git merge`, `git cherry-pick`, `git rebase --continue`) are exactly the
exemptions that survive — but §15 now *understates*: it names a clean
`git revert`, and the refusal is every revert form, plus `git merge
--squash`. Widening it is the operator's call, not this bead's.
