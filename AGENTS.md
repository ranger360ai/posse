# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Landing the plane

- **Know which tree you are in.** A dispatched session works in its OWN git
  worktree of this repo (under `~/.posse/worktrees/`), on a branch
  `posse/<session>` — its own index, its own HEAD, nobody else's. The work
  prompt names it. An operator session, and any session in the checkout at
  `~/src/posse` itself, shares that checkout with everyone.
- Close the bead, and commit. **In the shared checkout, name your own
  paths** (`git commit -F - -- <paths>`): there the index is shared and an
  unqualified commit takes whatever another persona has staged, and the
  `prepare-commit-msg` gate refuses one. In your own worktree nothing is
  shared, the gate stands down, and the ordinary form is fine.
- **A NEW file needs two steps** (rangerhq-4pbt): `git add -- <the new
  paths>`, then `git commit -F - -- <all your paths>`. A pathspec only matches
  a file git already has an index entry for, so the path-limited form on its
  own answers `did not match any file(s) known to git`. Scope that add with
  `--`; never `git add -A` or `git add .`, which stage every persona's file
  into the shared index.
- **`MM .beads/issues.jsonl` after a clean commit is not work** (rangerhq-be7k).
  bd's `pre-commit` hook flushes the beads db and stages it, but a path-limited
  commit only refreshes the index for the paths you NAMED — so the commit takes
  the flushed file and the index is left holding the version before it.
  `git status` then reads `MM` over a tree that already matches HEAD.
  Check with `git diff HEAD -- <paths>`, which compares the tree to the COMMIT:
  empty means there is nothing there, and `git restore --staged -- <paths>`
  clears it. Never unstage without that check first — if the diff is not empty,
  the entry is someone's real work.
- **In the shared checkout a revert is two steps**: `git revert --no-commit
  <sha>`, then `git commit -F - -- <the paths it touched>`. A plain
  `git revert` names no paths, so the same gate refuses it — after git has
  already staged it (rangerhq-lrnp); undo that path-limited with
  `git restore --source=HEAD --staged --worktree -- <those paths>`, never
  `git reset --hard`.
- **The bead id in the subject is this shop's provenance; the
  `Co-Authored-By` runtime trailer is the harness's** (ranger-base-5aks).
  599 of 608 commits on `main` name a bead, and everything that asks "why is
  this here" reads that. The trailer is typed by the model and enforced by
  nothing: 60% of the same commits carry one, in runs, since the repo's first
  week. No gate adds it and no gate removes it — `prepare-commit-msg` can
  refuse a commit but never opens its message file for write, so a
  trailer-less commit is a message that was written without one, never a
  route that ate one. Type it; do not rewrite history that lacks it.
- **Commit everything you want kept.** Only commits move: the launcher
  fast-forwards your branch onto `main` when the bead closes, and uncommitted
  files stay behind in a tree that is eventually retired.
- `bd sync`, so `.beads/issues.jsonl` matches the database. All worktrees
  share one beads database — the graph does not fork.
- **Never push, and never merge to `main` yourself. The operator pushes and
  the launcher merges.** Every persona's PID denies `Bash(git push:*)` and
  this repo's `pre-push` gate refuses it, so a push is a refused turn, not a
  landing. Work is complete when it is committed locally and the bead is
  closed; `posse worktrees` shows anything that has not landed.
