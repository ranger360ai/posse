# claude's cache/ and .last-cleanup, read- and write-denied in a caged seat's seatbelt profile (ranger-base-mbcbv)

*2026-11-21 · jian-yang · the darwin half of ranger-base-7x3bj*

## What changed

- **seatbelt**: `claudeCacheAndCleanupMarker`
  (`internal/posse/wallclaudesessionstate.go`) names
  `<CLAUDE_CONFIG_DIR>/cache` and `<cfg>/.last-cleanup`, each plus its
  resolved spelling when a link makes the two differ, under
  `claudeDebugDir`'s guard — so nothing is named unless the launching
  runtime's own state grant reaches claude's config dir, and a codex seat
  gets neither. `SeatbeltCarveOut` appends them to `DenyAll`, which is
  rendered as the LAST block of the profile (`deny file-read* file-write*`),
  so no allow above it — the ssh allowlist, the carve-out's `Keep`, the
  `projects/` reopen — can outvote it. `posse gates` prints each as
  `x <path> (read and write deny: claude's shared session state; …)`.
- **bwrap**: unchanged. `bwrapRender` clears `DenyAll`, and its spelling of
  these two members is the private dir bind and the private file bind
  ranger-base-7x3bj landed. A read deny there would hide the seat's own
  copies.

The shape is `claudeSessionsRegistry`'s (ranger-base-yaha3) and
`claudePromptHistory`'s (ranger-base-wj93u), down to the file member: a
`(subpath …)` naming a file matches that name and nothing beside it, so the
marker rides the same subpath block as the directory.

## The hole this closes

A caged DARWIN seat shared both names with every claude on the box. The
bwrap half gave a Linux seat a private copy of each; a profile cannot bind,
so the darwin answer is a deny. Two processes, one per member, and neither of
them is the seat whose profile this is:

- **an uncaged claude's served-catalog read.**
  `<cfg>/cache/model-catalog/<account>-<hash>-cc.json` is the served catalog
  cache — models, provider alias targets, per-model thinking and effort
  defaults, `state.model`, `min_claude_code_version` — and
  `model-catalog/masked-ids.json` is the persisted masked ids every later
  claude hydrates at start. Neither carries a signature field (the signing
  covers the separate published catalog) and claude checks the schema and
  the surface and nothing else, so a planted copy was what the next claude
  on the box believed about models. `cache/changelog.md` and
  `cache/my-closed-issues.json` are UI display only and ride the same deny.
- **the box's retention sweep.** `<cfg>/.last-cleanup` is read for its
  **mtime** as a 24h throttle: inside the window the background housekeeping
  and retention sweep is skipped, and the marker is rewritten only after a
  sweep runs. So one `touch -d <future>` switched box-wide transcript
  retention off indefinitely. Denying `file-write*` at the NAME is what that
  needs — claude treats a non-regular file there as not-regular, deletes it
  and rewrites — so unlink, create and a rename onto the name are refused
  too, not only a write through it.

Readers and impact are ranger-base-7x3bj's, read out of the claude binary's
own JS and not exercised; a posse seat's `--model` is an exact id, so the
alias route does not move a seat. P2 hardening.

## Pins

`internal/posse/wallclaudecacheseatbelt_qa_test.go`, the shape
ranger-base-wj93u's pin has:

- `TestQAWallClaudeCacheSeatbeltDeniesReadAndWriteForClaudeOnly`: both names
  are in the read-and-write deny block **by literal**, so renaming
  `claudeCacheDir` or `claudeLastCleanupFile` alone cannot satisfy the pin;
  both are in the carve's `DenyAll`; no `(allow` follows the block; the bwrap
  render's carve carries no `DenyAll` at all; and a codex seat's profile
  names neither.
- `TestQAWallClaudeCacheSeatbeltALinkIsDeniedAtBothSpellings`: with a link at
  each name the helper names the literal and the resolved spelling, in that
  order, and with no state grant reaching the config dir it names nothing.
- `TestQAWallClaudeCacheSeatbeltLiveRefusesTheNamesAndTheirContents`: the
  live arm, which runs only where `sandbox-exec` can apply a profile
  (darwin) and skips here, saying so. Fourteen rows under the rendered
  profile — the served catalog rewritten, appended, read and joined by a new
  catalog file; masked-ids rewritten and read; `model-catalog/` and `cache/`
  relinked; the marker touched into 2099, written, read, replaced by a link,
  renamed onto and unlinked — each refused, with a sibling write in the same
  granted config dir as the control that says the refusals are the deny's and
  not a missing grant. Then the host's catalog and masked ids are
  byte-identical and the marker is still a regular file with an mtime that
  has not moved into the future.

Control, MEASURED 2026-11-21 on this host (`go test ./internal/posse -run
WallClaudeCacheSeatbelt`, mutants fed in by `-overlay` so the worktree is
untouched):

| mutant | reds |
|---|---|
| the `DenyAll` append deleted from `SeatbeltCarveOut` | the render arm, on both members and on `DenyAll` |
| the resolved spelling dropped from the helper | the link arm |
| the config-dir guard bypassed | the codex arm and the no-grant arm |

## Cost

MEASURED 2026-11-21 on this Linux host, claude 2.1.330: a scratch
`CLAUDE_CONFIG_DIR` seeded with a `cache/model-catalog/masked-ids.json`, a
`cache/changelog.md` and a stale `.last-cleanup`, then `cache/` and
`.last-cleanup` at mode **0000** — EACCES, the refusal SBPL gives — and one
unbilled `claude -p --dangerously-skip-permissions` turn against a local fake
Messages endpoint (no egress, no billing). **The turn exits 0 with its
answer**, byte-identical to the control arm whose two names are open, and the
single refused read its debug log names is logged and dropped:

```
[DEBUG] [servedCatalog] mask file read failed: EACCES
```

That is why READ is denied and not only write. A write-only deny was the
fallback and is enough for the hazard; it is not needed.

What the seat pays: it refetches the served catalog and the changelog every
start and caches neither, and it reads the marker as absent, so it runs its
own sweep against names it cannot write. Both are what the bwrap half costs
with an empty private copy, minus the seat's own cache.

Two limits of that reading, named here because the pin names them:

- **ASSUMED, and not measurable in this instance**: that SBPL's refusal is
  handled the way this EACCES was — the same assumption ranger-base-yaha3 and
  ranger-base-wj93u carry, on the same ground (the code path is the one
  claude takes for any errno at these names), and that a subpath deny behaves
  as it does for `projects/` (ranger-base-r2uxm). There is no darwin box here.
- The **marker's own reader is not reached by a `-p` turn**: the measured
  run's debug log names no housekeeping sweep, and the control arm's stale
  marker was neither read nor rewritten. So what the measurement settles is
  that a turn completes with the marker denied, not what an interactive
  session's sweep does with it. The served-catalog fetch is the other half of
  that: against a fake endpoint claude logged
  `[servedCatalog] off (not_first_party)` and wrote no catalog cache, so the
  refetch cost is reasoned from ranger-base-7x3bj's reading and not measured
  here.
