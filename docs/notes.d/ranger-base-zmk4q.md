# codex's and grok's config.toml, read-only in a caged seat (ranger-base-zmk4q)

*2026-11-11 · gwart · ADR 0090 D3, the codex and grok half of
ranger-base-nv3el*

## What changed

A caged seat on codex or grok gets its own CLI's `config.toml` read-only
over the state grant that reaches it, at both mechanisms. An absent file is
seeded empty first so the bwrap mount can cover it. It is the same carve
ranger-base-6akzw made for claude's executable config, through the same
producer (`SeatbeltCarveOut`), and the path follows `CODEX_HOME` /
`GROK_HOME` the way the CLI does. A seat on another runtime gets no deny for
a home nothing granted it. Code: `runtimeConfigFiles` in
`internal/posse/wallexecconfig.go`.

## How D3's rule came out

D3: a launch override that wins over the file where the CLI has one, else a
read-only carve if the CLI runs with the file read-only.

| CLI | key | override that wins over the file? | runs with the file read-only? | answer |
|---|---|---|---|---|
| codex 0.167.1 | `notify` | yes, `-c notify=[]` | yes | carve (covers it) |
| codex 0.167.1 | `mcp_servers` | **no**: `-c mcp_servers={}` merges, the file's servers still start | yes | carve |
| grok 1.0.77 | `mcp_servers`, hooks, notification hooks, status-line, auth-provider and feedback commands | none found (`GROK_CONFIG` overlay drops these tables by design) | yes | carve |

The carve covers every key in the file at once, including keys nobody has
listed yet. So the one override that exists (`notify`) is not also typed:
a seat that cannot write the file cannot plant the key. That departs from
the rule's per-key wording, and the bead says why.

## Measurements

MEASURED 2026-11-11, D1 host, scratch homes. One turn per run against a
local fake model endpoint (Responses API for codex, Chat Completions for
grok, configured as a custom provider with a dummy key). No login and no
billed turn: this is how the work ran on a host where neither CLI is signed
in, which ADR 0090 recorded as blocking.

- Both CLIs start a stdio MCP server declared in their `config.toml`. codex
  starts it before the first API call. Its `notify` command runs when the
  turn completes.
- Both CLIs rewrite `config.toml` during a seat's life. The codex TUI writes
  `[tui]` bookkeeping. grok writes `[marketplace]` bookkeeping at startup in
  both `-p` and the TUI. So "does the seat need the write" was a question
  worth asking.
- With the file bind-mounted read-only, codex `exec`, the codex TUI with the
  fleet's flags, grok `-p` and the grok TUI all complete the turn. codex logs
  the two refused writes (EBUSY, a rename onto a mount point) and carries
  on. The grok screen shows nothing.
- Under grok 1.0.77, an MCP server declared only in
  `$GROK_HOME/managed_config.toml` or in an unsigned
  `$GROK_HOME/requirements.toml` did not start. This host is not signed in,
  so logged-in console sync is unmeasured.

## Pins

`internal/posse/wallruntimeconfig_qa_test.go`, `-run WallRuntimeConfig`:

- render: seeded, file-shaped deny for each CLI's own file, in the seatbelt
  profile too, never for the other CLI's file or for a claude seat;
  `CODEX_HOME` moves the carve.
- live wall: under posse's bwrap render, an append and an atomic replace of
  `config.toml` are refused; `sessions/`, a new home member and a temp file
  beside it are allowed.
- live CLI: each CLI found on the host runs from posse's render against the
  fake endpoint, and must print the fake model's reply token and leave the
  file byte-identical. A release that stops running with the file read-only
  reds here.

Control: with the carve line removed, all three red. The CLI pin reds on
"config.toml changed under the wall", which shows both CLIs write the file
when they can.

## Not closed here

- The other executable members of both homes (codex hooks, plugins, skills;
  grok agents, plugins, workflows): ranger-base-zvgql.
- grok's compat layer reads claude's `$HOME/.claude.json` MCP servers and
  `~/.claude/settings.json` hooks by default. That reader is missing from
  ADR 0090's table: ranger-base-ksa79.
- A notice or first-run screen that a seat used to dismiss by writing the
  file now shows again on every launch. None blocked a turn in the runs
  above.
