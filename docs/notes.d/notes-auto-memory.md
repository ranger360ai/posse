## The fleet writes no auto-memory (ranger-base-7uhip)

claude 2.1.263 keeps a per-project "auto-memory" — a directory of one-fact
markdown files plus a `MEMORY.md` index it loads into every session's context.
**It resolves that directory from the git WORKING-COPY root, not from the
session's cwd.** Every posse seat launches into
`~/.posse/worktrees/posse/<session>`, so every seat resolved to the *operator's* own
`~/.claude/projects/<sanitized MAIN checkout path>/memory/` and appended there.

MEASURED by monica 2026-09-06: 114 memory files under that one directory
carrying ~100 distinct `originSessionId` values, `MEMORY.md` at 199 lines
against a harness cap of **200 lines / 25,000 chars** past which the tail is cut
(the model is told so — the cut is not silent, but the memories are gone from
context either way). Not one of the 1470 per-worktree project dirs
(`~/.claude/projects/<sanitized WORKTREE path>-*`) holds a
`memory/` subdir at all, which is the whole proof: the transcript dir keys on
cwd, the memory dir does not.

So the fix is one key in the launch payload — `"autoMemoryEnabled": false` in
`ClaudeFleetSettings` (`internal/posse/agents.go`), which `{settings}` renders
as `--settings` on every claude launch line.

**Why off and not redirected.** claude also takes `autoMemoryDirectory`, so
pointing a seat at its own persona dir was available and was not taken
(ranger-base-bmr1c's ruling). `ORDERS.md` is the persona memory the constitution
names and the launcher already commits it (`memoryland.go`); the auto-memory was
a second channel beside it that no persona owned. A redirect keeps that second
channel and only moves it, and it makes a payload that is a `const` on purpose
into a per-persona render. The COST is real and is named here rather than
discovered later: a seat also stops READING that index, and what is in it today
is largely posse engineering lore written by earlier seats. `ORDERS.md`,
`AGENTS.md` and `docs/` are the channels that survive, and they are the ones
with an owner and a commit.

**Do not reach for `CLAUDE_CODE_DISABLE_AUTO_MEMORY`.** It exists, and a
launcher-exported variable loses to any settings scope naming the same key
(ranger-base-rq83c) — which is the reason the credential and inlet pins already
travel inside this payload. `--settings` is flagSettings. For the sibling key
`autoMemoryDirectory` the resolver's own scope list ranks flag above
local/project/user and below policy alone, and project scope is ignored for that
key outright ("Ignored if set in projectSettings … for security"). For
`autoMemoryEnabled` the arms below measured flag scope against the DEFAULT — the
operator's `~/.claude` names the key nowhere today. **Flag versus a user-scope
`true` — what the `/config` toggle writes — is UNMEASURED**, and it is not
measurable from a seat: the root-owned policy file on this box pins
`CLAUDE_CONFIG_DIR`, so there is no scratch user scope to plant in
(ranger-base-i7cy4). If auto-memory ever comes back on for the fleet, measure
that contest first.

**The measurement, reusable.** There is no zero-turn readout for this — `doctor`,
`mcp list` and `project` print nothing about the memory dir, and `mcp list` does
not even create it. It takes a headless turn, which a seat can have by handing
the child the session's own mint from the instance's env set — that recipe is
instance-ops content and stays in the instance tree (ranger-base-bm9cd) — then
one prompt, *"Reply with ONLY the absolute
path of your persistent file-based memory directory, or the single word NONE if
you have no memory directory"*, on `--model claude-haiku-4-5-20251001`, arms
differing in exactly one settings key. Four arms were run:

| arm | payload | cwd | answer |
|---|---|---|---|
| A | `autoMemoryDirectory` = a scratch dir | scratch repo | that scratch dir |
| B | A + `autoMemoryEnabled:false` | scratch repo | `NONE` |
| C | the real rendered payload, key removed | this worktree | the operator's `…/projects/<main checkout>/memory/` |
| D | the real rendered payload | this worktree | `NONE` |

A is the failing wrong arm for B (the rig can produce a directory) and C is the
defect reproduced by execution, in the real shape, with the payload the launcher
actually renders rather than a hand-written replica. Nothing was written to the
operator's memory dir by any arm: the prompt asks the session to *name* the
directory, not to save anything in it.
