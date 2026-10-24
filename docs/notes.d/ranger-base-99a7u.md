## The meta-harness landscape, 2026-10-24, and where posse sits in it (ranger-base-99a7u)

This is the evidence for ADR 0075 (posse is a thin meta-harness: five slots
over one foundation). Everything here was read on 2026-10-24.

**Labels.** MEASURED means run on this box (darwin 25.6.0). PRIMARY means
read today on the project's own repo, docs or site; GitHub numbers come from
`gh api repos/<owner>/<repo>` and `.../releases/latest`. SECONDARY means a
third-party page. NOT-FOUND means it was looked for and not found. A
NOT-FOUND is not a negative claim: like any reading of a third-party
product, it carries the date.

### This box (MEASURED)

| tool | version |
|---|---|
| herdr | 0.9.1 (latest is 0.9.5) |
| bob (Bob Shell) | 2.0.5 (latest is 2.0.6) |
| claude | 2.1.310 |
| codex | 0.153.4 |
| grok | 1.0.52 (see `notes-grok.md` on the pin) |
| `gt` (Gas Town) | prints 0.5.0. Gas Town's latest release is v1.2.1 and the Homebrew formula is v1.1.0, so this binary may not be Gas Town at all. Not resolved here. |
| tmux | installed; posse does not use it |
| zellij, rex | not installed |

### Entrants

Columns: **mux** = does it run agents in a terminal multiplexer, and could
that multiplexer be swapped; **Bob** = does it support IBM Bob as a runtime.

| entrant | layers | mux | runtimes · Bob | task store | connectors | license | momentum (2026-10-24) |
|---|---|---|---|---|---|---|---|
| **Paperclip** [1] | orchestration, board, task store, connectors, sandbox plugins | none. Runs agents headless on a heartbeat (`claude --print --output-format stream-json`); claude, codex, gemini and kimi default to ACP in source, while the docs say `engine: auto` (the two disagree). No tmux and no pty. | 13 adapter types in the docs, plus cursor-cloud and hermes-gateway in the repo · Bob NOT-FOUND | own Postgres (issues, goals, projects, routines) | 72 catalogue entries (badge says 76), some marked "setup unverified"; a tool gateway (experimental, off by default) that checks policy per call and keeps an audit trail; credentials are stored in Paperclip | MIT | 106,754★, 17,734 forks, about 236 contributors, created 2026-03-02; v2026.1023.0 published 2026-10-24T00:03Z; releases every 1–2 weeks |
| **herdr** [2] | multiplexer, with agent state | it is the multiplexer | 23 agents built in, 5 more report their own state · Bob NOT-FOUND natively (posse reports it, ADR 0060) | — | plugins | Apache-2.0, Rust | 45,619★; v0.9.5 2026-10-14; releases about weekly |
| **Superlogical Rex** [3] | multiplexer (terminal, server, CLI, API) | it is a multiplexer | — | — | — | not stated; no public repo; "always free", self-hostable | beta since 2026-10-05 (mailing-list waves, macOS app only); not GA |
| **Gas Town** [4] | orchestration (Mayor, Witnesses, Refineries, convoys), runtime adapter | tmux 3.0+ is required for its roles; a minimal mode with no tmux exists; no other multiplexer | claude (default), codex, copilot, gemini, cursor, kiro, amp, auggie, opencode, pi · Bob NOT-FOUND | **beads** (bd 0.57.0+) on Dolt | NOT-FOUND | MIT | 18,513★; repo moved to `gastownhall/gastown`; v1.2.1 2026-06-06, commits still landing |
| **Claude Code Agent Teams** [5] | orchestration inside one runtime | split panes need tmux or iTerm2; in-process by default | Claude only | file-locked shared task list under `~/.claude/tasks/` | project MCP | proprietary | experimental, behind `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`; launch date NOT-FOUND (the "April 2026" seed is unconfirmed) |
| **Agent Orchestrator** (formerly Composio) [6] | orchestration, runtime adapter, desktop and mobile apps | its own pty host by default; tmux is a legacy fallback | 35 harnesses, grok among them · Bob NOT-FOUND | tracker plugins (contents not read) | NOT-FOUND | Apache-2.0 | 13,553★; v0.14.6 2026-10-24 |
| **Claude Squad** [7] | TUI over tmux and a worktree per agent | tmux, required, not swappable | claude, codex, gemini, aider, any CLI via `-p` · Bob NOT-FOUND | none | none | AGPL-3.0 | 8,661★; v1.0.20 2026-08-20, also its last push |
| **Kilo Code Agent Manager** [8] | VS Code panel, worktrees under `.kilo/worktrees/` | no; its own terminal | Kilo's own agent only | NOT-FOUND | the extension's MCP | MIT | 27,732★; v7.8.17 2026-10-23 |
| **Emdash** [9] | Electron app, worktree per task, SSH to remote hosts | no | claude, codex, cursor, opencode, amp, devin, qwen, droid, copilot · Bob NOT-FOUND | local SQLite | Linear, GitHub, Jira, GitLab, Asana | Apache-2.0 | 6,068★; v1.2.9 2026-10-23 |
| **Conductor** [10] | macOS app, isolated workspaces | NOT-FOUND | claude, codex, cursor | NOT-FOUND | NOT-FOUND | not stated on the site (SECONDARY: closed source) | a free tier and paid tiers with cloud workspaces |
| **Vibe Kanban** [11] | web board, task store, runtime adapter | no; worktrees | 10 agents · Bob NOT-FOUND | its own, local only since the shutdown | has an MCP server | Apache-2.0 | "sunsetting" (README); shutdown post 2026-04-10 says community maintained; last release v0.1.44 2026-04-24 |

Left out: Bernstein (it calls itself a governance layer now) and Crystal
(SECONDARY: deprecated in February 2026).

### The protocols and pieces under them

- **ACP, the Agent Client Protocol** [12]. Its repo moved to
  `agentclientprotocol/agent-client-protocol` (Apache-2.0, 4,493★, v1.12.0
  2026-10-16, a v2 schema in alpha). Zed and JetBrains govern it for now,
  with the stated aim of an independent foundation. It is JSON-RPC 2.0 between
  a client and an agent that runs as a stdio subprocess. Calls go both ways.
  Its `terminal/*` methods let the agent ask the client to run a command.
  Nothing in it hosts the agent's own TUI. 46 agents are listed. claude and
  codex speak it through adapters. Bob is not listed, but Bob ships it
  natively.
- **Bob** [13]. `bob run` is headless (`--format stream-json`,
  `--max-cost`, `--max-turns`, `--workspace`, `--resume`), needs an API
  key in the environment, and pre-approves every tool. `bob acp` is native ACP over
  stdio (2026-08-31). 2.0.6 shipped in October.
- **Rex's automation surface** [3]. The CLI has `rex ls`, `send`,
  `capture`, `split`, `wait` and `run --wait`. The server API is
  self-describing (`rex api list/describe --json`) and has `session.create`,
  `session.new_split` (also callable from Lua), `com.superlogical.terminal.format`
  and `block.event`. `rex wait` returns a program's exit status. **No
  working, blocked or idle state is documented.** The beta post names
  "program activity status" as a feature, with no API for it.
- **Others are already putting new fronts on the multiplexer layer.**
  t-rex [14] is an installable web app over herdr, built on libghostty-vt
  in WASM, installed as a herdr plugin (MIT, created 2026-10-20, 0★).
  pi-subagentura #208 [15] (open, 2026-10-17) asks for Rex as a backend
  beside tmux, zellij and herdr, and says activity should stay `unknown`
  until Rex offers an API for it.

### What the landscape says about the slots

1. **Multiplexer: nobody else treats it as a slot.** Every entrant either
   is a multiplexer (herdr, Rex), hard-wires one (Gas Town and Claude Squad
   need tmux; Agent Teams needs tmux or iTerm2), or has none at all
   (Paperclip, Agent Orchestrator, Emdash and Kilo run agents headless or in
   their own pty). The only multi-multiplexer evidence is pi-subagentura
   #208. A swappable multiplexer is posse's distinctive position, and it is
   also the least proven one.
2. **Agent state is the hard verb.** herdr detects it for 23 agents. Rex
   documents none, and #208 reached the same answer on its own: report
   `unknown`. For ranger-base-lcxr3 this means a backend without detection
   needs state from the runtime (hooks, or `pane report-agent` as with Bob
   under ADR 0060), or it reads "unknown" and dispatch refuses (ADR 0013 §1).
3. **Runtimes: Bob is posse's to carry.** No entrant lists Bob. Paperclip
   has no adapter for it and herdr has no detection for it. The two routes
   in are the argv seam posse already uses and ACP, which Bob speaks
   natively. ADR 0075 §3 keeps the argv seam.
4. **Tasks: beads is in good company.** Gas Town is also built on beads.
   Every other entrant with a task store uses its own database
   (Paperclip's Postgres, Emdash's SQLite, Vibe Kanban's local store). None
   offers a store that another harness could share.
5. **Connectors: the reference shape is a gateway, and posse should not
   copy it yet.** Paperclip's gateway is experimental and off by default,
   and it makes Paperclip a credential owner. Binding a connector into each
   runtime's own MCP config (ADR 0075 §5) gives the same swap with no new
   long-running process.
6. **Sandbox: Paperclip treats it as a plugin slot**, with Cloudflare,
   Daytona, E2B, exe.dev, Kubernetes, Modal and Novita in the docs, and
   createos in the repo. That is the eligible-occupant list for a remote
   posse sandbox. ADR 0074's lesson, that the store of record must be
   reachable from inside, is the first test any of them would face.
7. **Defaults diverge on permissions.** Paperclip's claude adapter
   defaults to the CLI's skip-permissions flag, codex to bypassing
   approvals and its sandbox, cursor to `--yolo`, and opencode to allow
   every tool. posse renders a wall from the PID at every launch
   (ADR 0002). That divergence is worth keeping.

**Where posse sits.** It is the one entrant that keeps the multiplexer, the
runtimes and the task store as separate, substitutable pieces under a
persistent team defined by PIDs. That is narrower than Paperclip, which owns
every layer, and wider than Claude Squad, which is a tmux front end. It
holds the position by staying thin. Each slot gets a second occupant only
when there is a real one to build against.

### Sources

1. https://github.com/paperclipai/paperclip (README, `packages/adapters/*/src/index.ts`, `packages/adapters/claude-local/src/server/execute.ts`, `packages/db/src/schema/`, `doc/experimental/issue-worktree-support.md`, `docs/guides/board-operator/costs-and-budgets.md`); https://docs.paperclip.ing/reference/adapters/overview/ ; https://docs.paperclip.ing/connectors/ ; https://docs.paperclip.ing/reference/api/tool-gateway/ ; https://docs.paperclip.ing/administration/plugins/ — PRIMARY
2. https://github.com/herdrdev/herdr ; https://github.com/herdrdev/herdr/releases/tag/v0.9.5 ; https://herdr.dev/docs/agents/ — PRIMARY
3. https://superlogical.com/ ; https://superlogical.com/updates/public-testing-beginning ; https://superlogical.com/rex/docs/automate/cli ; https://superlogical.com/rex/docs/automate/server-api — PRIMARY
4. https://github.com/gastownhall/gastown (README, provider-integration doc) — PRIMARY
5. https://code.claude.com/docs/en/agent-teams — PRIMARY
6. https://github.com/OrchestratorInc/agent-orchestrator ; https://docs.orchestrator.inc/plugins/agents ; https://docs.orchestrator.inc/plugins/runtimes — PRIMARY
7. https://github.com/smtg-ai/claude-squad — PRIMARY; https://www.augmentcode.com/tools/open-source-agent-orchestrators (updated 2026-08-12) — SECONDARY
8. https://kilo.ai/docs/automate/agent-manager ; https://github.com/Kilo-Org/kilocode — PRIMARY
9. https://github.com/generalaction/emdash — PRIMARY
10. https://www.conductor.build ; https://www.conductor.build/pricing — PRIMARY; licence from augmentcode.com — SECONDARY
11. https://github.com/BloopAI/vibe-kanban ; https://www.vibekanban.com/blog/shutdown — PRIMARY
12. https://github.com/agentclientprotocol/agent-client-protocol ; https://agentclientprotocol.com/protocol/overview.md ; https://agentclientprotocol.com/overview/agents.md ; https://agentclientprotocol.com/community/governance.md — PRIMARY
13. https://bob.ibm.com/docs/shell/getting-started/start-bobshell-non-interactive ; https://bob.ibm.com/docs/shell/features/acp ; https://bob.ibm.com/docs/shell/changelog — PRIMARY
14. https://github.com/alexanderkreidich/t-rex — PRIMARY
15. https://github.com/lmn451/pi-subagentura/issues/208 — PRIMARY for the issue; its claims about Rex are SECONDARY

The seed blogs at amux.io, parallelcode.app and agentsroom.dev were not
read. Nothing above rests on them.
