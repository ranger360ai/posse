# claude's shell snapshots and session-env, private to each seat (ranger-base-5632o)

*2026-11-11 · dinesh · y438o class, outside the 6akzw carve · touches the
bwrap render (BwrapArgs step 2c, bwrapRender) and nothing at seatbelt*

## What this closes

Every claude seat on the box shares one config dir, and the caged write set
grants it (ADR 0080 D19). Two of its members are run by claude ITSELF, ahead
of each Bash command:

- `shell-snapshots/snapshot-bash-<id>.sh`: the Bash tool runs every command
  as `bash -c … source <CLAUDE_CONFIG_DIR>/shell-snapshots/snapshot-bash-<id>.sh …`.
  MEASURED 2026-11-11 from `/proc/$$/cmdline` in a live seat, claude 2.1.322.
- `session-env/<session id>/`: hook-written scripts. The 2.1.322 binary's own
  permission refusal for a write there says it "runs the scripts that hooks
  leave there ahead of Bash commands". Read from the binary's text, not
  exercised with a planted hook.

Both are one file or dir per session, all at one uid. So one seat could
append to another live session's snapshot, and the text would run in that
session's shell on its next Bash call, outside the cage if that session is
uncaged. That held without a race.

## The fix

A carve cannot do it: the seat's own claude writes both members at startup.
So each seat gets a PRIVATE copy. `claudePrivateSessionState`
(internal/posse/wallclaudesessionstate.go) names two binds:
`<SeatCacheDir>/claude/{session-env,shell-snapshots}` over the config dir's
members. It names them only when the runtime's state grant reaches that dir.
BwrapArgs renders them after the grants and before the denies (step 2c), so
the bind lands on top of the config-dir grant. The seat writes its own copy
and sees nobody else's.

- The destination is `lstat`'d. It sits in a dir every seat can write, so a
  link there may be another seat's, and bwrap would mount where it points. A
  non-directory refuses the render. The grant check comes after the lstat,
  because `writeGranted` resolves its argument and would skip the link
  rather than refuse it. The first cut had that order backwards, and
  `TestQAWallClaudeSessionStateRefusesALinkAtTheMember` caught it.
- The members are mount points inside the seat, so `mv` of either one, or of
  the config dir, is EBUSY. The live pin tries all three.
- The source lives under the seat cache, so the worktree removal that takes
  the cache takes these too.

## Measured

- Pins: `go test ./internal/posse -run WallClaudeSessionState`. Render (bind
  present, after the grant, none for a codex seat), link refusal plus prep,
  and the live join under the real bwrap: a peer's snapshot and hook env
  byte-identical on the host after seven plant shapes, neither visible, the
  seat's own snapshot written and sourced by a later shell, and stored in its
  private dir on the host.
- Mutation: with `r.Carve.Private` set to nil, the render pin and the live
  pin both red, the live one on every plant row.
- A real claude under the binds, unbilled. MEASURED 2026-11-11, D1 host,
  claude 2.1.322, bwrap 0.13.1, scratch CLAUDE_CONFIG_DIR, a local fake
  Messages endpoint that asks for one Bash call:
  `claude -p … --dangerously-skip-permissions` exits 0. The Bash call ran (its
  marker file was written). Its own `ls $CLAUDE_CONFIG_DIR/shell-snapshots`
  answered exactly one name, `snapshot-bash-1794437340182-n5yw9t.sh`, its
  own, with the peer's planted snapshot absent. Its session-env dir landed in
  the private copy, and the shared dir held only the peer's entries afterwards.
  That was a bwrap line with the two private binds and the config-dir grant,
  not posse's full render. The full render's join is the live pin above.

## Not closed here

- ranger-base-f30f6
- ranger-base-d0rnr
- ranger-base-w95hl
