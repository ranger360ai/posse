# grok's compat layer is a reader of claude's and cursor's MCP servers and hooks, switched off on the line (ranger-base-ksa79)

*2026-11-12 · gwart · ADR 0090 D3; found under ranger-base-zmk4q*

## What changed

grok's harness-compatibility layer reads other CLIs' files. It starts the
MCP servers in `$HOME/.claude.json` and `~/.cursor/mcp.json`, and the hooks
in `~/.claude/settings.json` and `~/.cursor/hooks.json`. Every cell is on by
default. ADR 0090 listed only claude as a reader of `.claude.json`.

Each cell has an environment twin that wins over `config.toml`, so D3's
first branch applies: a launch override. grok's `Runtime.LineEnv` now types
these four variables beside `GROK_DISABLE_AUTOUPDATER=1`:

    GROK_CLAUDE_MCPS_ENABLED=0  GROK_CLAUDE_HOOKS_ENABLED=0
    GROK_CURSOR_MCPS_ENABLED=0  GROK_CURSOR_HOOKS_ENABLED=0

`RuntimeLineEnv` renders them. `WrapWithGates` puts them on every grok line:
planLaunch, RelaunchAgent and the runtime probe. ADR 0090's reader table
gets a grok row, and D3 gets an as-built paragraph.

Nothing is lost. MEASURED 2026-11-11 (ADR 0090): no MCP server is defined in
either claude file on this host. There is no `~/.cursor`.

## Measurement

MEASURED 2026-11-12, D1 host, grok 1.0.77 (50ca579fa996). Each arm used a
scratch HOME and one unbilled `grok -p` turn against a local fake
chat-completions endpoint. A planted command touches a marker when it runs.

| arm | env | markers |
|---|---|---|
| control | none | claude-mcp, cursor-mcp, sessionstart, promptsubmit, stop |
| mcps0 | `GROK_CLAUDE_MCPS_ENABLED=0` | sessionstart, promptsubmit, stop |
| hooks0 | `GROK_CLAUDE_HOOKS_ENABLED=0` | claude-mcp |
| all four | all four `=0` | none |

- The turn completed in every arm.
- `grok inspect` lists the planted entries `[disabled]`. It reads each cell
  `OFF (env)`, including when `config.toml` sets
  `[compat.claude]`/`[compat.cursor]` `mcps = hooks = true`.
- The PreToolUse hook had no tool call to fire on.
- No cursor hook fired in the control arm. The plant format may be wrong,
  or 1.0.77 may still refuse cursor hooks (runtime.go recorded them as
  "loaded, REFUSED" on 1.0.52). `GROK_CURSOR_HOOKS_ENABLED` is typed on the
  docs (`05-configuration.md`) and is not backed by a reading.
- These switches also cover the cwd-scoped sources the same cells name, such
  as `<cwd>/.cursor/mcp.json`. That narrows the project-config channel. It
  does not change it.

## Pins

- `TestQAWallRuntimeConfigGrokLineDisablesTheUpdater`: the exact line env.
  It covers grok only.
- `TestLiveGrokLineStartsNoCompatMCPOrHook`: live, with no wall needed.
  posse's wrapped grok line runs against the plants and starts nothing. The
  same line with `RuntimeLineEnv` removed must start all five. That control
  arm reds the pin if a grok release renames a switch or moves a reader.

      go test ./internal/posse -run 'GrokLine' -v
