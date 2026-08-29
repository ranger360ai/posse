## A path-limited commit does not refresh the index for paths it did not name (rangerhq-be7k)

gilfoyle hit this closing rangerhq-f2p5 in a shared checkout: a clean
`git commit -F - -- INSTALL.md` committed two files (INSTALL.md and
`.beads/issues.jsonl`, flushed by bd's pre-commit hook — correct), and left

```
git status --short          MM .beads/issues.jsonl
git diff --cached --stat    .beads/issues.jsonl | 4 +, 7 -    index != HEAD
git diff --stat             .beads/issues.jsonl | 7 +, 4 -    worktree != index
git diff HEAD --stat        <empty>                           worktree == HEAD
```

The worktree was already correct and committed. `git status` said otherwise.

**Attribution, which the bead asked for and which is not what it guessed.**
Nobody wrote that entry. git's *partial* commit builds two indexes: it writes
the REAL index — refreshed for the pathspec only — and holds its lock, then
commits from a separate `$GIT_DIR/next-index-<pid>.lock`. That second one is
what `GIT_INDEX_FILE` points at while the pre-commit hook runs, so the hook's
`git add` reaches the commit and never reaches the real index, which was
written before the hook was called. Measured directly (git 2.39.3, macOS 25.4,
2026-08-29) — the hook prints its `GIT_INDEX_FILE` as `.git/next-index-61429.lock`
and sees the pre-flush blob there. bd's hook is the trigger, not the writer,
and it is doing the only thing it can with the index git handed it.

**The producer is the form we prescribe, and it is the only form that produces
it.** Same hook, same edit, five forms:

| commit form | index left stale? |
|---|---|
| `git commit -m x -- <paths>` — the blessed form | **YES** |
| the same, with no hook installed | no |
| `git commit -a` | no |
| `git commit -i -- <paths>` | no |

`-a` and `-i` commit from a lock that BECOMES the real index, so the hook's add
lands in the index that survives. Only the pure pathspec commit keeps the two
apart. This retires the clearance `scripts/audit-silent-reverts.sh` used to
carry — "the blessed form does NOT have this property" — which was drawn from
a correct measurement (it refreshes the NAMED paths) and over-read into a
blanket one. rangerhq-8rtf named one producer of a stale index; this is a
second, and it is the one nobody was watching because it is the one we tell
personas to use.

**Sizing the harm: it is a status-line bug, not a data-loss bug.** Which later
form carries the stale blob back into HEAD:

| later form | hook removed | hook installed |
|---|---|---|
| unqualified `git commit -m` | **SILENT REVERT** | no — the hook re-flushes and re-stages |
| `git commit -i -- <paths>` | **SILENT REVERT** | no — same |
| `git commit -a` | no | no |
| `git commit -m x -- .` / `-- <named>` | no | no |
| unqualified / `-i`, plus `--no-verify` | — | **SILENT REVERT** |

Two corrections fall out. The bead expected `git commit -a` to be a carrier; it
is not — `-a` re-reads every modified tracked file from the working tree, and
the working tree is correct, so `-a` overwrites the stale entry instead of
committing it. And in a live bd repo the hook that made the entry also repairs
it, which leaves exactly one reachable route: a carrier run with `--no-verify`,
because that is what skips the repairing flush.

**That route is walled.** `--no-verify` skips pre-commit but still runs
prepare-commit-msg (measured), which is the slot the shared-index wall took —
for this reason, stated in `gates.go` and now pinned. So the wall refuses both
`--no-verify` carriers, for every shell in the checkout since rangerhq-lt2w.

**Where the fix went, and why not the three places it did not.** Not bd: its
hook stages into the index git gives it and has no other index to reach. Not
the wall: its slot runs before the commit, git's real-index lock is written
before that, and the form is the one we want — there is nothing to refuse. Not
git: refreshing only the pathspec is the documented behaviour of a partial
commit. What is left is that the state is invisible to a persona doing the
right thing, so the fix is to make it readable: a line in AGENTS.md's landing
checklist with the check (`git diff HEAD -- <paths>`, which compares the tree
to the COMMIT and so is empty exactly when the entry is content-free) and the
recovery (`git restore --staged -- <paths>`), plus the corrected fact in
`audit-silent-reverts.sh` and `gates.go`, plus
`internal/rhq/staleindex_qa_test.go` holding all of the above as measurements
rather than as sentences — which is the part that keeps the next over-read from
shipping.

**Do not copy the recovery without the check.** `git restore --staged` is safe
here only because `git diff HEAD` was empty first; if it is not, the entry is
somebody's real work and unstaging it in a shared index throws it away.
