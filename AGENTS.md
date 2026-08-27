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
- **In the shared checkout a revert is two steps**: `git revert --no-commit
  <sha>`, then `git commit -F - -- <the paths it touched>`. A plain
  `git revert` names no paths, so the same gate refuses it — after git has
  already staged it (rangerhq-lrnp); undo that path-limited with
  `git restore --source=HEAD --staged --worktree -- <those paths>`, never
  `git reset --hard`.
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
