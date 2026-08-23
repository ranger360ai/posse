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

- Close the bead, and commit **naming your own paths** (`git commit -F - --
  <paths>`) — every persona shares this checkout and its index, so an
  unqualified commit takes whatever another persona has staged.
- `bd sync`, so `.beads/issues.jsonl` matches the database.
- **Never push. The operator pushes.** Every persona's PID denies
  `Bash(git push:*)` and this repo's `pre-push` gate refuses it, so a push
  is a refused turn, not a landing. Work is complete when it is committed
  locally and the bead is closed.
