# ADR 0090 — A config file a caged seat must rewrite is made inert by removing its readers, not its writer

*Status: accepted 2026-11-11 (ranger-base-nv3el, the residual of
ranger-base-6akzw) · owner: architect · builds on ADR 0080 D19 (claude's
config lives in a dir grant) and ADR 0077 D1 (`CLAUDE_CONFIG_DIR` is typed on
the line)*

## Context

ranger-base-6akzw carved claude's executable config read-only inside the
caged write set. One file of the same class could not be carved:
`$CLAUDE_CONFIG_DIR/.claude.json`. It holds MCP server definitions at user
scope and per project, plus the approvals for a repo's `.mcp.json`. An MCP
server definition is a command that every claude reading the file starts at
session start, outside any wall the reader runs under. The CLI replaces the
file by atomic rename every turn, so it must stay writable (D19; probe B8
rules out binding the file itself).

So a write the runtime needs and a write that plants a command land in the
same file. No carve can split them. Something other than the write set has to
decide.

Who reads that file today:

| reader | file it reads | can a caged seat write that file? |
|---|---|---|
| every posse-launched claude, caged or not | `~/.claude/.claude.json` (the line types `CLAUDE_CONFIG_DIR`) | yes, at both mechanisms |
| the operator's own `claude`, no variable set | `~/.claude.json` | bwrap: no (D19 dropped it). seatbelt: **yes**, it is still granted (`SeatbeltSiblings`) |
| claude started inside a seat | either | inside the same cage, so no escape |
| every posse-launched grok, through its harness-compatibility layer (added by ranger-base-ksa79) | `$HOME/.claude.json` for MCP servers, never `CLAUDE_CONFIG_DIR`'s; `~/.claude/settings.json` for hooks | the same answers as the operator's `claude` row for `.claude.json`; `settings.json` is carved read-only (ranger-base-6akzw) |

## Decision

**D1. Every claude line posse renders carries `--strict-mcp-config`.** The
flag tells claude to start only the servers named by `--mcp-config` and to
ignore every other source: user scope, project scope and a repo's
`.mcp.json`. Writes to `.claude.json` still land, but no posse claude reads
the keys that start commands. The flag is a launch guarantee of the
`EnsureSettingsPin` kind. It is in the built-in template, and it is appended to
any line that starts claude and lacks it, through the same door, so a declared
`command:` gets it too. `posse runtime check` reports a line without it the way
it reports a missing unattended flag. A line may still name its own
`--mcp-config`, and that is how the operator gives seats a server. The launch
refuses, at both mechanisms, if any file that flag names is inside that seat's
write set.

**D2. On seatbelt, `~/.claude.json` leaves the write set whenever the line
types `CLAUDE_CONFIG_DIR`.** This is D19's rule, which bwrap already follows.
A seat with the variable set never writes the home file, so granting it only
gives the seat the operator's MCP list. The comment above `SeatbeltSiblings`
that rejects `CLAUDE_CONFIG_DIR=$HOME/.claude` because it "moves the operator's
file" is stale against ADR 0077 D1 and ranger-base-rwcqg: the variable is
already typed. The render half can be pinned from any host. The live half
needs a darwin host.

*As built (ranger-base-ntnst, 2026-11-11).* `SeatbeltStateDirs` in
`internal/posse/seatbelt.go`, the sandbox-exec twin of `BwrapStateDirs`. Every
seatbelt reader takes its state dirs from it: the profile and report
(`wall.go`), the reach probe (`reachability.go`) and D1's MCP-file refusal
(`mcpconfig.go`). One narrowing beyond the letter: `CLAUDE_CONFIG_DIR=$HOME`
puts the seat's file back at `$HOME/.claude.json`, so the entry stays there.
Pinned by `internal/posse/seatbeltclaudeconfigdir_qa_test.go`. The live half
is not done: no darwin host was available, so it is the verify lane's.

**D3. Codex and grok follow the same rule, decided per CLI by measurement.**
For each command-bearing key (codex: `mcp_servers`, `notify`; grok: whatever
its config holds), use a launch override that wins over the file when the CLI
has one. Otherwise carve the file read-only, if the CLI still runs with it
read-only. This host runs neither CLI (MEASURED below), so the work waits for a
host that does.

*As built for grok's compat layer (ranger-base-ksa79, 2026-11-12).* grok also
reads OTHER CLIs' command-bearing files: claude's MCP servers and hooks (the
table's last row) and cursor's MCP servers from `~/.cursor/mcp.json`. Every
cell is on by default. grok has a launch override for each one, and it wins
over `config.toml`, so the rule's first branch applies. Every grok line now
carries `GROK_CLAUDE_MCPS_ENABLED=0`, `GROK_CLAUDE_HOOKS_ENABLED=0`,
`GROK_CURSOR_MCPS_ENABLED=0` and `GROK_CURSOR_HOOKS_ENABLED=0`
(`Runtime.LineEnv`, rendered by `RuntimeLineEnv`). Pinned live by
`TestLiveGrokLineStartsNoCompatMCPOrHook`: posse's rendered line starts no
planted server or hook, and the same line without the variables starts all
five. That second arm is the control. No function is lost: this host has no
MCP server in either claude file, and it has no `~/.cursor`.

**D4. The pin is live, under the rendered wall, and carries its own
controls.** Inside posse's real bwrap render for a claude seat, the pin writes
a server definition into the seat's `.claude.json` and a repo `.mcp.json`. That
write must succeed, which is the control that the file stays writable. Then it
starts claude from posse's rendered line with one legitimate `--mcp-config`
server outside the wall. Pass: the legitimate server starts and the planted
ones do not. A second arm, the same line without the flag, must start the
planted server. That arm reds the pin if a CLI release moves where MCP is read
from. It also refuses a write to `$HOME/.claude.json`. No turn is billed: a
dummy key and a dead API endpoint, because servers start before the first API
call. A render pin covers every claude line posse can produce. hoover
re-verifies from a bwrap seat.

## Alternatives rejected

- **Give each seat its own `CLAUDE_CONFIG_DIR`, seeded from the shared one and
  never read back** (the bead's first shape). On Linux the credential is a
  file in that dir, so each seat would get its own copy, and the copies split
  token refresh across N files. ASSUMED that the refresh token rotates, so one
  seat's refresh could strand the others' copies. ranger-base-rwcqg priced
  this already and rejected it for the same reason. Posse's own readers of
  `projects/` transcripts would also lose the shared dir.
- **A per-seat overlay on the config dir inside the mount namespace** (the
  clever one; bwrap 0.13.1 has `--overlay`). It is the only option that closes
  the whole file, whatever keys a future CLI adds, which D1 does not. Rejected
  for three reasons. It is Linux-only, since seatbelt has no overlay, so it
  would mean two designs. It makes private every write the fleet must share:
  the credential refresh, transcripts and history. Those would need a list of
  binds back through the overlay, and a file bind fails on atomic replace (the
  same B8 result). That is a bigger list to rot than D1's, and it fails worse:
  a lost login, not an unread key. It is the exit hatch if D1's residual below
  turns into a measured key.
- **A launcher check that refuses a launch when the command-bearing keys
  changed since the last uncaged write** (the bead's second shape). It only
  detects; it does not prevent. It races any running seat that writes after the
  check. It refuses wrongly after the operator's own `claude mcp add`. It
  protects nothing posse did not launch.
- **Box-wide managed policy** (`managed-mcp.json` under `/etc`). It would also
  cover a bare `claude`, but it needs root, it is a host change the operator
  must approve change by change (risk line 3), and it takes MCP away from the
  operator's own sessions. Recorded as the operator's option, not posse's.
- **Scrub the keys from the file at launch.** That writes a file the CLI
  rewrites every turn, so it loses updates against running seats, and a
  scrubbed key can be planted again a second later.

## Consequences

- **No function is lost today.** MEASURED 2026-11-11: no MCP server is defined
  in either `~/.claude.json` or `~/.claude/.claude.json`, at user scope or in
  any project entry.
- **The residual is keys, not the file.** D1 makes MCP inert, and with it the
  `.mcp.json` approvals (`enabledMcpjsonServers`,
  `enableAllProjectMcpServers`), which the probe showed are ignored under the
  flag. The same key census found no other key that starts a process. A
  per-project `allowedTools` is an approval, not a command start. Whether
  2.1.322 still reads it from this file is UNMEASURED (a static read of the
  binary was inconclusive). PID deny lists and the gate shims bind either way.
  A CLI release that adds a key that starts something reopens the class, and
  D4's control arm catches only a move of MCP. Re-read the key list when the
  claude pin moves.
- **An operator `claude` run with `CLAUDE_CONFIG_DIR=~/.claude` would read the
  seats' file.** MEASURED: the variable is not set in `~/.bashrc`,
  `~/.profile`, `~/.bash_profile` or the systemd user environment. ASSUMED
  that the operator does not type it by hand. The build adds this to NOTES
  as one line of host procedure.

## Measurements

- MEASURED 2026-11-11, D1 host, claude 2.1.322 (mise), scratch config dir,
  dummy API key, API endpoint at a dead local port. Three planted servers
  (user scope, project scope, repo `.mcp.json` with project approval), each
  touching a marker when started. Without the flag, all three markers
  appeared. With `--strict-mcp-config`, and with that flag plus an empty
  `--mcp-config`, none appeared. No turn was billed.
- MEASURED the same day: `~/.codex` holds only `tmp/` and `~/.grok` holds only
  `docs/`. The CLIs are installed but nothing is logged in, so no codex or
  grok seat runs on this host.
- MEASURED 2026-11-11 (ranger-base-m0bfv), the same host and claude, against
  a local fake Messages API that asks for one `Bash` call, with no billed
  turn: a `projects.<cwd>.allowedTools` rule in `.claude.json` is NOT
  honoured. The call is refused as "needs approval", exactly as it is with no
  rule, while the same rule passed as `--allowedTools` runs the call. The
  Consequences entry above that says UNMEASURED is answered by this reading.
  docs/notes.d/ranger-base-m0bfv.md has the arms.
- MEASURED 2026-11-12 (ranger-base-ksa79), D1 host, grok 1.0.77, scratch
  HOME, one unbilled `grok -p` turn per arm against a local fake endpoint.
  Plants: a server in `$HOME/.claude.json`, claude hooks (SessionStart,
  UserPromptSubmit, Stop, PreToolUse) in `~/.claude/settings.json`, a server
  in `~/.cursor/mcp.json`, and cursor hooks in `~/.cursor/hooks.json`. With
  nothing set, both servers and claude's three turn hooks ran. PreToolUse had
  no tool call to fire on. Each claude variable at 0 stopped only its own
  reader. With all four at 0, nothing ran. The turn completed in every arm.
  `grok inspect` reads the cells `OFF (env)` even when `config.toml` sets them
  true. No cursor hook fired with the variable unset either, so that switch
  rests on grok's docs (`05-configuration.md`), not on a reading.
  docs/notes.d/ranger-base-ksa79.md has the arms.
