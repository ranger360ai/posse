## Bob on herdr 0.8.2: three detection routes measured, one alive by half (ranger-base-v1yrt)

The design record is ADR 0060. This is the bench: what was typed, what came
back, on 2026-09-28, personal box, herdr 0.8.2 (`~/.local/bin/herdr`, the
client and the server the session's `HERDR_SOCKET_PATH` reaches), bobshell
2.0.5 under node 25.2.1. Public product facts only; the pane fixtures are the
recon's, redacted (`<work-email>`).

Two shapes of the 0.8.2 CLI that cost a probe each, so nobody re-learns them:
`herdr pane list` prints JSON with no flag (`--json` is "unknown option"), and
`pane report-agent` / `release-agent` take the PANE ID FIRST — the options
before the pane id are a 0.9.0 change per the release notes. `pane
process-info` takes `--pane <id>`.

### 1. A locally added alias does not resolve

Overrides installed one at a time under `~/.config/herdr/agent-detection/`,
each a copy of the remote manifest with `version = "2026.09.28.101"` and
`aliases = [..., "bob"]`, then `herdr server reload-agent-manifests`, then
`herdr server agent-manifests --json` to confirm the override was the ACTIVE
source before asking:

```
gemini override active: [('gemini', 'local override', '2026.09.28.101')]
  agent explain --file idle-signed-in.txt --agent bob   → bob unknown unknown_agent (no manifest)
  agent explain --file idle-signed-in.txt --agent gemini → gemini idle default_known_agent_idle_fallback 2026.09.28.101
claude override active: [('claude', 'local override', '2026.09.28.101')]
  --agent bob         → bob unknown unknown_agent (no manifest)
  --agent claude-code → claude idle live_prompt_box 2026.09.28.101   (the compiled alias resolves)
control, no override: --agent grok-build → grok idle default_known_agent_idle_fallback 2026.07.16.105
```

Both overrides were removed and the reload re-run in the same script; the
claude shadow lasted seconds and carried identical rules. The runbook's
alias route (`docs/runbooks/agent-detection-manifest.md`) was measured on
`grok-build`, an alias upstream's grok manifest carries too — so what
resolves is the compiled alias table, and a manifest cannot add to it.

### 2. A standalone manifest is still ignored

A four-line `bob.toml` (id, version, min_engine_version 3, one idle rule)
installed and reloaded: `agent explain --agent bob` → `unknown_agent`,
`manifest_source: null`, and `server agent-manifests` does not list it.
Same answer as rangerhq-tr8k on 0.8.0. Removed after.

### 3. The live pane gets no label from argv

`herdr workspace create --cwd <scratch> --no-focus --env SANDBOX=1 --env
BOB_GATEWAY_URL=http://127.0.0.1:9 …` (dead gateway: no turn can be spent;
`SANDBOX=1` suppresses the browser open), then `pane run <pane> "bob chat
--accept-license --trust -w <scratch>"`, 10s:

```
pane row:      agent None, agent_status unknown, title "bob chat --accept-license --trust -w"
process-info:  argv ["/opt/homebrew/Cellar/node/25.2.1/bin/node", "/opt/homebrew/bin/bob", "chat", ...]
agent explain: {"error":{"code":"agent_not_found"}}
screen tail:   "Complete sign-in in your browser… (Press ESC or Ctrl+C to exit)"
```

(With the gateway dead Bob falls to the sign-in screen even though a token
is stored; that is the screen's fixture for the filing, not a finding about
sign-in.)

### 4. report-agent: label yes, explain no, prompt no

On that pane, and again on a plain shell pane with no Bob at all:

```
report-agent <pane> --source posse --agent bob --state idle
  pane row → agent bob, agent_status idle; appears in `agent list`
agent explain <pane> → agent_explain_unavailable: "does not have a detected agent label"
report --state working; agent wait --until working (started 2s earlier, 8s timeout) → returns, status working
report --state idle after working; agent wait --until idle → timeout; `agent get` → status done
agent prompt <pane> "echo …" → agent_not_ready: "agent w2KF:p1 is not an active named agent"
release-agent → agent None, status unknown
```

So a reporter can make the pane addressable and its waits partly work, but
posse's readiness gate (`Seen()` over `agent explain`) and its delivery path
(`agent prompt`) both refuse a reported label. All probe workspaces were
closed by the script's trap; `pane list` shows none of them.

### 5. What the bundle says (read, not run)

`/opt/homebrew/lib/node_modules/bobshell/dist/bob.js`, grep with context:

- The program: `.enablePositionalOptions().allowExcessArguments().allowUnknownOption()`,
  with `-p, --prompt <prompt>` defined on the program. Subcommand actions
  read `optsWithGlobals()` and set `prompt` from the positionals
  (`r.join(" ")`) only when `-p` is unset. Hence `bob -p … chat …`, never
  `bob chat … -p …`.
  **Correction, MEASURED 2026-10-01 (ranger-base-5jjtn): the ordering is
  right and buys nothing — `bob chat` reads that `-p` back and SUBMITS NO
  TURN.** This section says "read, not run" and this is what that costs:
  the bundle shows where the value is parsed to, not that anything sends it.
  See `docs/notes.d/ranger-base-5jjtn.md` §1; `BobCommand` now carries no
  `-p` at all.
- Custom modes: `~/.bob/settings/custom_modes.yaml` is "available in all
  workspaces", `.bob/custom_modes.yaml` per workspace; a mode carries
  `roleDefinition`, `customInstructions`, `groups`.
- The status screen labels the cost `Bobcoins:` with the `🅞` glyph, beside
  `Monthly Budget:` and `Monthly Usage:`; the formatter is
  `e>=1?toFixed(2):e>=.01?toFixed(3):toFixed(4)`.
- Env names read: the API-key name (two spellings, the one `bob run`
  names in its refusal), `BOB_GATEWAY_URL`, `BOB_WEB_LOGIN_URL`,
  `BOB_LOG_LEVEL`, `BOB_EXTENSIONS`, `BOB_SUPERVISED`, `BOB_USE_MODEL_ENV`,
  a `BOB_TELEMETRY_*` family.
- `bob run [prompt...]` is the headless front; `-f json|stream-json`.

### 6. bob.db, read with the system sqlite3

`~/.bob/db/bob.db` (WAL). `tasks(id, project_id 'file:<dir>', status
running|active, first_message, directory, costs, last_error, …)`,
`messages(id, task_id, role system|user|assistant|tool, data JSON, created_at)`,
`task_pending_approvals` (empty while the Execute Command dialog was up —
blocked is pane-only). Per assistant message `data._meta.spend = {cost,
contextTokens}`; the task's `costs` is the running sum:

```
assistant 0.021024 / 10512 tokens · 0.021266 / 10633 · 0.021464 / 10732 · 0.02167 / 10835 · 0.021744 / 10872
tasks.costs {"cost":0.107168,"contextTokens":10872}
```

### 7. Arg-parse checks with the gateway dead

`bob chat -w <dir> zzz-positional` draws the TUI (no argument error);
`bob run zzz` → `Error: Bob API key is required.` (and it names the env
variable to set). Nothing here spent a turn.
