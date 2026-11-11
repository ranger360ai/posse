# codex's and grok's other executable home members, read-only in a caged seat (ranger-base-zvgql)

*2026-11-11 · gwart · follows ranger-base-zmk4q (ADR 0090 D3) and
ranger-base-6akzw*

## What changed

A caged seat on codex or grok now gets the rest of its CLI home's executable
members read-only over its state grant, beside `config.toml`. Directories are
denied whole. Files are seeded empty first (`{}` for JSON), so the bwrap mount
can cover them. The path follows `CODEX_HOME` / `GROK_HOME`, and a seat on
another runtime gets no deny for a home nothing granted it. Code:
`runtimeExecMembers` in `internal/posse/wallexecconfig.go`, wired into
`SeatbeltCarveOut`.

| CLI | dirs | files |
|---|---|---|
| codex | `plugins/ prompts/ rules/ skills/` | `AGENTS.md AGENTS.override.md hooks.json` |
| grok | `agents/ commands/ hooks/ installed-plugins/ personas/ plugins/ rules/ skills/ workflows/` | `AGENT.md AGENTS.md Agents.md CLAUDE.local.md CLAUDE.md Claude.md lsp.json managed_config.toml requirements.toml settings.json trusted_folders.toml` |

The bwrap seeder now makes a seed appear whole (`seedFileWhole`). A body
that is not empty is written to a temp file and hard-linked into place, so
no reader on the box ever sees a `{}`-seeded JSON file with zero bytes. The
old create-then-write left that window open, and this bead adds two JSON
seeds to homes that other CLIs read.

`TestPersonaLaunchRuntime` was red on main before this bead, from
ranger-base-zmk4q, which was mine. MEASURED: arm 1 of an unmodified tree
failed with `invalid JSON: unexpected end of JSON input`. The test's grok
recipe named no `dir:`, so it launched in `$HOME`, the test binary's shared
home. A parallel test's real bwrap prep had seeded an empty
`~/.grok/config.toml` there. That is a name on grok's project-config list,
which the classifier parses as JSON. The recipe now gets its own session
dir. The same thing happens outside tests: once any caged grok seat has run,
a grok seat launched in `$HOME` degrades on `~/.grok`. That is the old
verdict for any home whose `config.toml` exists, so it is not filed.

## Measurements

MEASURED 2026-11-11 on the D1 host, codex 0.167.1 and grok 1.0.77, scratch
homes. Each run was one turn against a local fake model endpoint that logged
every request body, with no login and no billed turn (the zmk4q method).
"In the request" means a token planted in the member reached the model.
"Ran" means a planted command touched a marker.

- **codex.** AGENTS.md, AGENTS.override.md and a skill's description under
  `skills/` are in the request. An empty AGENTS.override.md does not shadow
  AGENTS.md, and an empty `hooks.json` is accepted. A planted `hooks.json`
  hook did not fire, because codex gates hooks on a trust review. Firing is
  UNMEASURED: the trust-bypass flag was not run in this session. `plugins/`,
  `prompts/` and `rules/` are documented readers that were not exercised.
  A prompt under `prompts/` is not in the request until it is invoked.
  codex writes its system skills, scripts included, into `skills/.system` at
  startup. With `skills/` read-only it skips that write and completes the
  turn.
- **codex, not a reader.** An MCP server and `notify` declared in
  `$CODEX_HOME/managed_config.toml` or `$CODEX_HOME/requirements.toml`
  started nothing. The same declarations in `config.toml` both ran (the
  control).
- **grok.** A `hooks/*.json` SessionStart hook and a UserPromptSubmit hook ran
  in an untrusted folder. `lsp.json`'s server command ran. A plugin under
  `plugins/` loads with its hooks and MCP server (`grok inspect`). `rules/*.md`,
  a home AGENTS.md, `skills/` and `commands/` are in the request. `grok
  inspect` lists all six instruction file names at home level as global
  instructions. `agents/`, `personas/` and `workflows/` are documented readers
  that were not exercised. `--trust` writes `trusted_folders.toml`, and that
  file decides where project hooks run for every later grok.
- **grok's managed_config.toml and requirements.toml.** When the files are
  writable, grok's startup deletes them before they are read. That is why
  zmk4q saw them start nothing. When they are read-only, the hooks in both
  ran and the MCP servers in both started. They are carved so that a seat is
  never the one to write them. The seed is an empty file, which starts
  nothing.
- **Read-only.** With every member above bound read-only, `codex exec` and
  `grok -p` (with and without `--trust`) complete the turn and leave every
  member byte-identical.

## Pins

`internal/posse/wallruntimeconfig_qa_test.go`, `-run WallRuntimeConfig`:

- render: every member is a deny at both mechanisms. Dirs are dir-shaped,
  files are seeded and file-shaped. Never the other CLI's home, never for a
  claude seat. `CODEX_HOME` moves the carve. A list edit that drops a member
  the measurement found read reds the pin.
- live wall: under posse's bwrap render, a create in each dir and an append
  and an atomic replace of each file are refused.
- live CLI: each CLI completes a turn from posse's render and leaves every
  seeded member byte-identical.

Control: with the carve given no grants, all three red (82 failure lines).

## Not closed here

- Residual: ranger-base-74pqi.
- A caged codex no longer installs its system skills. They appear only after
  an uncaged codex runs. A caged grok cannot `plugin install` or `--trust` a
  folder persistently.
