# claude's own protected config members, carved read-only (ranger-base-d0rnr)

*2026-11-11 · dinesh · y438o class, outside the 6akzw carve · touches
claudeExecConfig's lists (internal/posse/wallexecconfig.go), so both walls*

## What this closes

claude's config dir is granted to a caged seat (ADR 0080 D19), and
ranger-base-6akzw carved its executable members read-only over that grant.
The claude 2.1.322 binary has its own list of config-dir members that it
refuses to let its sandboxed Bash or file tools write. Six of them were
outside the 6akzw carve, and each one holds something a LATER claude, or
claude's background-session daemon, starts or obeys:

- `workflows/`: saved Workflow scripts. JS that spawns agents when named.
- `routines/`: claude's own permission rules guard it with `commands/`,
  `skills/` and `workflows/`.
- `daemon.json` and `daemon/`: the background-session service starts the
  workers listed there, and runs its cron tasks, "without asking anyone"
  (the binary's own refusal text).
- `scheduled_tasks.json` and `launch.json`: claude reads both at
  `<project>/.claude/`, so in the default config dir they belong to the HOME
  project. The first holds durable cron prompts that a later session in `~`
  fires. The second holds the Preview launch config's commands.

All six are now on `claudeExecConfigDirNames` / `claudeExecConfigFileNames`,
so they get the 6akzw treatment at both mechanisms: a deny over the state
grant; at bwrap, dirs are made empty when absent and files are seeded `{}`.

## Measured

MEASURED 2026-11-11, D1 host, claude 2.1.322, bwrap 0.13.1, a scratch
CLAUDE_CONFIG_DIR, one unbilled `claude -p --dangerously-skip-permissions`
turn against a local fake Messages endpoint that asks for one Bash call and
logs every request body. This reuses the 5632o harness, with body logging
added.

- Plant, everything writable. A `workflows/tokwf.js` carrying a token was
  in both requests, listed as an invocable skill
  (`- tokwf: TOKWF… planted workflow`) beside a planted `commands/` control.
  A `.md` under `routines/` or `workflows/` was not in a `-p` request, so
  the reader of `routines/` is UNMEASURED. It is carved on claude's own word.
- Carve. `workflows/`, `routines/` and `daemon/` were empty and read-only,
  and the three files were `{}` and read-only. The turn exited 0 and its
  Bash call ran. The debug log names no refused write. The three files were
  byte-identical afterwards (sha256), and the dirs were still empty.
- `{}` as `scheduled_tasks.json` loads as no tasks (the loader wants a
  `tasks` array). `daemon.json`'s reader takes `{}` as no workers.
- None of the six exists in this box's live `~/.claude`, after months of
  seats, so no seat has needed to write one.
- Census: none of the new names is on any runtime's project-config list, so
  `WallSeedProjectConfig`'s census is unchanged (its pins are green).

Pins: `go test ./internal/posse -run 'WallExecConfig'`.
`TestQAWallExecConfigCarvesClaudesOwnProtectedMembers` holds the names on
the lists. The render pin walks the lists, so it cannot notice a name
dropped from them. `TestLiveWallExecConfigRefusesEveryRowUnderTheRenderedWall`
has seven new refused rows. Mutation: with the lists reverted to 6akzw's,
the membership pin reds on all six names, and the live pin reds on all
seven rows. The first cut of the three dir rows was green under that
mutation: the dirs are absent in the fixture, so `: >` died on ENOENT
either way. Each of them now makes its dir first.

## Not closed here

- ranger-base-h5vij
