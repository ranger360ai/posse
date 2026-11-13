# The rest of claude's own protected config members, carved read-only (ranger-base-ka66w)

*2026-11-12 · dinesh · follows ranger-base-d0rnr · touches
claudeExecConfig's lists (internal/posse/wallexecconfig.go), so both walls*

## What this closes

ranger-base-d0rnr carved six of the config-dir members that claude 2.1.322's
own sandbox write-denies. This bead carves the rest of that list. Each is now
a read-only deny over the config-dir grant at both mechanisms: at bwrap, dirs
are made empty when absent and files are seeded.

- Dirs: `access-audit`, `agent-memory`, `agent-memory-local`,
  `cowork_plugins`, `jobs`, `logs`, `mcp-discovery-cache`,
  `mcp-skill-archives`, `seed-admin`, `shares`, `state`, `statsig`, `todos`.
  `todos`, `statsig`, `logs` and `access-audit` are the binary's `Qgo` list.
- File: `loop.md`, the default `/loop` prompt.

From the binary's text (none of this was exercised): `cowork_plugins/` is the
alternate plugin root. `seed-admin/` is the plugin-seed admin dir, which claude
makes itself, 0700. `agent-memory/` is user-scope subagent memory.
`agent-memory-local/` is the local scope at `<cwd>/.claude/`, so in the
default config dir it belongs to the home project, the same reason d0rnr
carved `scheduled_tasks.json`. `jobs/` is background-session state. The
readers of the rest are unmeasured, and they are carved on claude's word.

## loop.md: seeded empty, not masked (a divergence from the bead's text)

The bead preferred a `/dev/null` mask over a `{}` seed, because a seeded
`loop.md` is a prompt. Two readings changed that:

- `seedBody` gives a non-`.json` name an EMPTY seed, never `{}`. claude
  2.1.322 trims `loop.md` and treats an empty one as "no loop file": the
  reader returns null when the trimmed length is 0 (read from the binary). So
  the seed is no prompt, and to an uncaged claude the empty file is the same
  as an absent one.
- MEASURED 2026-11-12, D1 host, bwrap 0.13.1, through
  `TestLiveWallExecConfigRefusesEveryRowUnderTheRenderedWall`: a `/dev/null`
  mask reads as `Permission denied` (EACCES) inside the wall, because bwrap
  binds nodev. claude's loop reader returns null on a not-found test and on
  EISDIR, and rethrows anything else. Whether that not-found test admits
  EACCES is unmeasured, so a mask could break a seat's own `/loop`.

So `loop.md` is an ordinary seeded member of `claudeExecConfigFileNames`, and
it reads as empty inside the wall.

## Measured

- None of the members exists in this box's live config dir (MEASURED
  2026-11-12, `ls -a ~/.claude`), so no seat has needed to write one.
- Pins: `go test ./internal/posse -run WallExecConfig`, green in all three
  arms. `TestQAWallExecConfigCarvesTheRestOfClaudesProtectedDirs` is the
  hand-typed membership list. `TestQAWallExecConfigSeedsClaudesLoopFileEmpty`
  checks the seed is empty and that the file is not masked. The seatbelt
  pin's hand-typed list has every new name. The live pin has a refused row
  per member (each dir row makes its dir first, as in d0rnr), and it checks
  that `loop.md` reads as an empty file under the wall.
- Mutation, through `go test -overlay` (the tree untouched): dropping `jobs`,
  `todos`, `seed-admin` or `loop.md` from the tables reds three tests each.

## Not closed here

- ranger-base-66k22
