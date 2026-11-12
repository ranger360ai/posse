# claude's projects/, read-only in a caged seat but for its own key dir (ranger-base-ajkqx)

*2026-11-12 · dinesh · touches the bwrap render (BwrapArgs step 2c,
bwrapRender) and nothing at seatbelt*

## What this closes

Every claude seat shares one config dir, and the caged write set grants it
(ADR 0080 D19). Inside it, `projects/<key>/` is one dir per working
directory (`claudeProjectDir` is the key), holding the session transcripts
(`<uuid>.jsonl` and `<uuid>/`) and `memory/`. Other claude processes read
both back: `memory/` at every interactive session start for that key with
no flag needed, and the transcripts on `--resume` and through posse's own
cost and turn-outcome readers. So a caged seat could write instructions into
another key's memory or transcripts, the operator's own keys included.

## The fix

Three binds IN PLACE, not private, so every reader keeps its path
(`claudeProjectsBinds`, internal/posse/wallclaudeprojects.go), rendered in
BwrapArgs' step 2c after the config-dir grant, in this order:

1. `--ro-bind projects projects`;
2. `--bind <own key> <own key>`, only when cwd is the launch's own session
   worktree (`claudeOwnSessionTree`: a linked git worktree at exactly
   `<WorktreeRoot>/<repo>/<session>`);
3. `--ro-bind <own key>/memory <own key>/memory`. The fleet writes no
   auto-memory, and a seat's own key can equal the operator's.

A seat whose cwd is not its own session worktree (the shared checkout, a
probe in a scratch dir, a subdirectory of the tree) gets 1 only.

`PrivateBind` gained `ReadOnly`, and `Src == Dst` now means "in place": the
destination's lstat is the source's too, and an absent dir is made once in
prep. A link at any of the three refuses the render, as 5632o's members do.

## Measured

- Step 0, MEASURED 2026-11-12, D1 Linux host, claude 2.1.322, bwrap 0.13.1,
  scratch `CLAUDE_CONFIG_DIR`, a local fake Messages endpoint, no billed
  turn: `claude -p` with `projects/` wholly read-only (EROFS) exits 0 and
  prints its reply. It writes no transcript and says nothing about it. With
  the three binds above it exits 0 and writes its transcript in the own key
  dir. So a seat outside its own worktree loses only its transcript, and no
  `--session-id` file grant is needed.
- Pins: `go test ./internal/posse -run WallClaudeProjects -v`. Render (order
  after the grant; nothing reopened for the main checkout, a plain dir or a
  subdirectory; nothing for a codex seat), link refusal and single prep, and
  the live join under posse's rendered wall and the real bwrap: twelve plant
  shapes into a peer key's `memory/MEMORY.md` and `*.jsonl`, a new key, the
  own `memory/` and the mount points themselves, each refused and the peer's
  files byte-identical on the host; the own transcript written and appended
  by a shell; then the real claude under the same argv against a dead
  loopback endpoint, which writes its transcript into the own key dir.
- Mutation, by `go test -overlay`, five mutants, every one red: all three
  binds dropped; the own key never reopened; `memory/` read-write;
  `projects/` read-write; `ReadOnly` ignored by the renderer.
- 5632o's live pin had a "projects still write" row. It now writes a plain
  config-dir member instead, since its fixture's cwd is no session worktree.

## Not closed here

- ranger-base-r2uxm
- ranger-base-w95hl
