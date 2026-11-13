# claude's debug/, private per seat at bwrap and write-denied at seatbelt (ranger-base-4cmqw)

*2026-11-13 · gwart · closes ranger-base-ntt1w; finding ranger-base-gab5i; follows ranger-base-dlw20*

## What changed

`<CLAUDE_CONFIG_DIR>/debug` is no longer a directory every seat on the box
shares and every seat can write.

- **bwrap**: `debug` joins `claudeSessionStateDirNames`
  (`internal/posse/wallclaudesessionstate.go`), so each seat gets a private
  copy of it under its seat cache, bound over the shared name after the
  config-dir grant. A seat writes and reads only its own debug logs and sees
  no other seat's. The render already refused a link at a private member's
  name — `BwrapArgs` lstats every `Dst` and will not mount on wherever
  another seat pointed it — so `<cfg>/debug` as a symlink refuses the launch
  rather than redirecting it.
- **seatbelt**: a profile cannot bind and cannot say "every name but this
  session's own" ahead of a random session id, so `claudeDebugDir` adds the
  directory to `SeatbeltCarveOut.Deny` and it renders as
  `(deny file-write* (subpath "<cfg>/debug"))`. A subpath write deny covers
  CREATING and UNLINKING a name under it, not only writing one already
  there, which is what a planted link would need.
- `bwrapRender` takes that one deny back out again, the same two-step
  `codexEphemeralDirs` makes: the deny step mounts after the private one, so
  a read-only bind from the host would put the shared directory straight
  back over the seat's own copy.

Both arms are guarded the way every other member of this carve is: nothing is
denied or bound unless the launching runtime's own state grant reaches
claude's config dir, so a codex or grok seat gets no line for a directory
nothing granted it.

## The hole this closes

MEASURED 2026-11-13 by ranger-base-gab5i (claude 2.1.323, scratch
`CLAUDE_CONFIG_DIR`, local fake Messages endpoint, unbilled): a claude running
with debug on writes `<cfg>/debug/<session id>.txt` **by path** and follows a
symlink there — appending to an existing target, creating a missing one — and
it reopens the log by path on every write, so swapping the live log for a link
mid-session redirects the rest of it. The session id is readable out of
`sessions/*.json`, so nothing has to be guessed. If `<cfg>/debug` itself is a
link, every log goes into the link's target dir. `debug/latest` is inert:
claude unlinks and recreates it rather than following it.

From a bwrap seat `<cfg>/debug` was W_OK, so any caged seat could plant there.
The payoff was claude-authored debug text appended into any uid-writable file
outside the planting seat's wall, plus file creation at any path it named —
integrity and DoS, and it fires only on a debug-on session, which is rare.

## Measurements

**The seat's own claude still runs with its debug dir write-denied, so no
`--debug-file` reroute is needed.** MEASURED 2026-11-13 on this Linux host,
claude 2.1.323, with `<cfg>/debug` at mode 0555 — EACCES, the refusal SBPL
gives — under a scratch `CLAUDE_CONFIG_DIR` with `CLAUDECODE` and
`CLAUDE_CODE_CHILD_SESSION` stripped (gab5i's trap):

| leg | `<cfg>/debug` | rc | log written | stderr |
|---|---|---|---|---|
| control | writable | 0 | 42,929 B plus `latest` | nothing about debug |
| denied | 0555, already holds a log | 0 | none, no new `latest` | nothing about debug |
| fresh denied | 0555 from the start, empty | 0 | none | nothing about debug |

Each leg was a full turn with debug on, not a local subcommand. The write
failure is swallowed: the turn completes with its answer and says nothing.
ASSUMED that SBPL's refusal is swallowed the same way, on the same ground
ranger-base-wj93u assumes it for `history.jsonl` — the host measured is Linux,
and the code path is the one claude takes for any errno at this name.

Unbilled was the intent and three short turns were billed instead: `--debug`
takes an OPTIONAL argument, so `claude --debug mcp list` parses as
`--debug=mcp` with `list` as the prompt. Whoever re-runs this should spell it
`--debug=` or put the subcommand first.

## Pins

`internal/posse/wallclaudesessionstate_qa_test.go`,
`-run WallClaudeSessionState`:

- **bwrap render**: the existing loop over `claudeSessionStateDirNames` now
  covers `debug`, and a new arm asserts the shared dir is NOT in
  `Carve.Deny`.
- **seatbelt render**
  (`TestQAWallClaudeSessionStateSeatbeltDeniesTheSharedDebugDir`): the
  subpath deny is in the profile, below the carve-out's deny block opener and
  not in the allow block, and a codex seat's profile has no such line.
- **live wall**: from under the real bwrap, with another seat's live debug
  log on the host — the peer's log is not visible; appending to it, replacing
  it with a link, creating a new `<id>.txt` link beside it, moving
  `<cfg>/debug` and replacing it with a link all leave the host's log
  byte-identical, still a regular FILE, the shared dir still a DIRECTORY, the
  redirect target byte-identical, and no new name under the shared dir. The
  seat's own debug log and its own `latest` are written, and land in its
  private dir and never in the shared one.

Control, MEASURED 2026-11-13: each half dropped on its own reds.

| mutation | reds |
|---|---|
| `debug` out of `claudeSessionStateDirNames` | live pin: the seat destroys the shared dir, the peer's log vanishes, its own writes fail |
| the `SeatbeltCarveOut` deny dropped | seatbelt pin: no deny line, `Carve.Deny` does not name the dir |
| `bwrapRender` keeps the deny | render pin names the dir in `Carve.Deny`; live pin: the peer's log is visible again and the seat's own debug write is `Read-only file system` |

## Costs

- A seat's own debug log now lives under its seat cache
  (`<seat cache>/claude/debug/`), not in `~/.claude/debug`, and goes when the
  worktree's cache goes. Reading a seat's debug log means reading it there.
- At seatbelt a caged seat keeps no debug log at all. The turn is unaffected
  (measured above); the log is simply lost.
- A hostile seat can still stop the NEXT launch by leaving a link at
  `<cfg>/debug`, because the render refuses rather than mounting over it.
  That is the fail-closed direction this carve already takes for every other
  private member: loud beats silent.
- Shims-tier seats are unwalled, so neither arm binds them. This route gave
  them nothing they did not already have.

## Not closed here

- ranger-base-wj93u
- ranger-base-a544g
