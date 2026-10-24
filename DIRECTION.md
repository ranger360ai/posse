# DIRECTION — where posse is going

*Status: architecture settled 2026-08-11; all three sequencing phases
below have landed (last refreshed 2026-08-17); the stack redrawn as slots
over a foundation 2026-10-24 (ADR 0075, ranger-base-99a7u). The
tmux/Ghostty implementation lives on the tmux-reference
branch (last commit 2026-08-11). Nothing here is a promise with a date on
it; dates below are history, not plans.*

posse started as a Ghostty + tmux session manager. It is becoming the
**business harness** of the Ranger work system: the layer that knows who the
operator's agents are, what they can do, what they should work on, and what
environment they run in — built on shared open-source substrates rather than
competing with them.

The thinking owes a lot to Steve Yegge's
[Continuous Thunderdome](https://yegge.ai/essays/the-shape-of-things-to-come/)
essay. Two of his conclusions shaped this plan: harnesses end up **bespoke**
(chemically bonded to your actual work — his Wheelhouse is closed for a
reason), while the substrates underneath are **shared**. posse is the
operator's bespoke harness. Everything below it is adopted, not built.

## Design philosophy: a thin meta-harness

*Operator direction, 2026-10-24. The decision is ADR 0075; the landscape
it was weighed against, sourced and dated, is
`docs/notes.d/ranger-base-99a7u.md`.*

posse aims for **extreme swappability, fast motion, and staying as thin as
possible.** Meta-harnesses are in rapid development and posse will not try
to keep up with them. It stays thin and uses what others build at every
layer below it:

- **Multiplexer** — stay thin; herdr today, and Rex, tmux and others may
  follow. In the end an agent runs in a shell.
- **Sandbox** — local or remote; stay thin and use existing technology.
- **Runtimes** — claude, codex, grok, bob, and others. Many runtimes,
  IBM Bob among them, is a hard requirement.
- **Connectors / MCP** — they could be pushed down into the runtime layer,
  but are likely better at the meta-harness layer. Either way, swappable.
- **Tasks** — beads today. The hardest one to move, but in principle
  swappable.

**What stays is intent, and it stays with the user:** a persistent team of
agents with memory and orders. The PID, from the DISCOVER framework
(ADR 0001), is the foundation. Everything else is a slot.

Two rules keep this thin (ADR 0075 §2). A slot holds no posse state
hostage: what lasts lives in the foundation or the task store. And a slot
becomes code (an interface) only when a bead is building its second real
occupant. Naming who may fill a slot commits posse to building none of them.

## The stack

```
posse — the foundation (bespoke, this repo; NOT a slot)
        PIDs · personas (a task-store assignee) · memory · orders ·
        constitution · dispatch loop
  │
  ├─ tasks         slot · beads (bd)                 github.com/steveyegge/beads (MIT)
  │                durable work graph, claim, mail, project memory
  ├─ multiplexer   slot · herdr                      github.com/herdrdev/herdr (Apache-2.0)
  │                panes, layout, live agent state (working/blocked/idle)
  ├─ runtimes      slot · claude · codex · grok · bob
  │                interchangeable labor; a PID-rendered argv in a shell (ADR 0013)
  ├─ sandbox       slot · seatbelt (L2) with the L1/L3 shims (ADR 0002)
  │                no local Docker (2026-08-30); a remote occupant goes off-laptop
  └─ connectors    slot · none yet
                   when needed: the PID names it, the launch binds it into the
                   runtime's own MCP config (ADR 0075 §5)
```

Each layer has a clean job:

- **herdr** is the presentation and oversight layer: it owns terminals,
  layout, and *ephemeral* state — which agent is in which pane and whether it
  is working, blocked, or idle right now — so the operator can see at a
  glance what is running, where, in parallel. Its CLI/socket API is the actuation surface
  (`agent start` / `agent prompt --wait` / `agent wait` / `pane read`,
  per-pane env injection).
- **beads** (`bd`) is the durable work substrate: a dependency-aware issue
  graph with atomic claiming (`bd ready` → `bd update --claim` → `bd close`),
  inter-agent mail with threading, and project memory
  (`bd remember` / `bd prime`).
- **posse** binds them: it decides which persona picks up which bead in
  which environment, launches it through herdr, and gives the operator the
  cockpit.

The tmux implementation was not thrown away, but it did not become a
second backend either: main is herdr-native, and the tmux/Ghostty version
(bash script, bubbletea launcher, 2×2 grid) is frozen on the
tmux-reference branch as of 2026-08-11.
**Decided 2026-08-17:** tmux-reference is retired — a dead-end reference,
kept but not maintained. It is not a fallback and there is no bead to keep
it building.

**Reconciled 2026-10-24 (ADR 0075).** The 2026-08-17 rule survives and is
now the rule for every slot: no second backend is built ahead of a second
occupant. Its premise did not survive. It said the rewrite for a new
multiplexer would be the herdr client (`herdr.go`, `herdrback.go`), and
13 non-test files named that seam (MEASURED 2026-09-10, method not
recorded). On 2026-10-24, 21 of 150 non-test files hold or build a herdr
client or backend, 32 name a herdr type, and 73 mention herdr (MEASURED).
The meta-file design under `state/herdr/` still keeps session state out of
the multiplexer. But the code seam is wider than one client now. Rex
(Superlogical's libghostty-based multiplexer) is the second occupant. The
interface is extracted when it starts, not before: ranger-base-lcxr3, gated
on Rex GA (ranger-base-jusig). tmux is eligible for the slot and is not
being built.

## The four pillars, and who owns what

**Env sets** — already built (see NOTES.md). The named, secret-masked
catalog and per-recipe binding stay in posse; only the injection mechanism
changes (per-pane `env` on herdr's API instead of `tmux new-session -e`).

**Persistent personas** — built. Identity is durable through beads, not
the multiplexer: herdr agent names are aliases that die with the process;
a beads **assignee** is forever. A persona is its assignee name
(`BD_ACTOR`, injected at launch) + its Persona Intent Document
(`agents/<name>.md`, ADR 0001 — frontmatter command template, intents,
guardrails as prose *and* tool rules; the markdown body is the prompt) +
a per-persona memory directory seeded with standing orders. Seven crew
personas are live. Its history is every bead it ever claimed, closed, or
was mailed about — provenance for free.

**Memory** splits three ways, and posse deliberately owns only one:

| kind | lives in |
|---|---|
| project knowledge | beads (`bd remember` / `bd prime`) |
| persona-private memory & standing orders | posse per-persona dirs |
| runtime working memory | the agent CLI's own mechanisms |

Building a posse project-memory feature would triple-implement what the
substrate already does. Resist.

**Tasks** — adopt `bd` outright. posse never grows an issue tracker; it
grows a *dispatcher* over one. bd is a slot, not the floor: a second
tracker would have to provide what ADR 0075 §4 lists (ready-with-deps,
claim, close, comments, edges, its own count, one writer and read-back, a
store bound by env). Nothing builds one.

**Skills** — the agent runtimes own skill loading; posse owns the
cross-agent, cross-project **binding**: persona X gets these skills and this
memory whether it launches as claude or codex, materialized at launch time
(seeded dirs, template args). The runtime half exists (ADR 0002: a persona
launches on claude, codex, or grok; the cage is model-agnostic); the skills
half ships for all three built-ins — a PID's `skills:` names dirs under
`RHQ_HOME/skills`, and the launch materializes them per runtime: claude
gets a rendered plugin dir and `--plugin-dir` typed at it; codex and grok
take no flag at all and read `<session dir>/.agents/skills/`, so the
launch symlinks them there and hides the dir in `.git/info/exclude`. A
runtime that can materialize neither refuses the launch (ADR 0007,
rangerhq-185 → d3u → 1qd).

## Crew, fleet, and the dispatch loop

Yegge reinvented the same shape twice (Gas Town, then Wheelhouse) without
trying: interactive **crew** agents that design and review, background
**fleet** workers that consume the work graph, mail between them, a
multiplexer under it all. posse's existing UI maps straight onto it:

- **crew** = sessions marked crew (`posse crew <name>`, and any session the
  operator starts a conversation in) — dispatch leaves them alone
- **fleet** = background sessions (~20 today) — workers grinding the queue
- **cockpit** = a herdr plugin pane (`posse cockpit`) showing `bd ready`
  alongside live sessions, with dispatch from the keyboard

The dispatch loop is the entire harness core. It is not small —
`internal/posse/dispatch.go` is 5,542 lines and 128 functions (MEASURED
2026-09-10) — and the sketch below is its shape, not its size:

```
bd ready
  → route bead to a persona (by label/type), then to a free seat in its lane
  → create the bead's own workspace <persona>-<repo>-<bead> · agent start
    <persona> (env set injected) · await the agent settling idle
  → bd update --claim · agent prompt "work bd-xxxx" --wait --until done|blocked
  → done:    bd close → the seat refills with the next ready bead
  → blocked: herdr sidebar flags it → the operator intervenes from the grid
```

## Sequencing

Ordered so every phase pays for itself even if the next never happens:

1. **Use beads bare.** `bd init` + `bd setup claude` in one real repo, work
   normally for a while. No orchestration — just the memory upgrade.
   *Landed 2026-08-11 (this repo's own graph is the proof).*
2. **herdr backend + personas.** Port the backend seam to herdr's CLI/socket
   API; bind personas to beads assignees; launcher shows herdr state instead
   of polling tmux. *Landed 2026-08-11 (herdr-native main, cockpit plugin,
   claim/close as the persona); PIDs 2026-08-15 (rangerhq-f8u/gqi).*
3. **Dispatch.** One fleet persona pulling `bd ready`, then more. Only now
   does the loop above exist. *Landed 2026-08-11 (rangerhq-b5p); QA'd
   2026-08-16 (rangerhq-beb); `--watch` and parallel passes 2026-08-16
   (rangerhq-bh8, tqr); seven personas dispatched since.*

## Cautions (learned from other people's postmortems)

- Gas Town died of harness self-refinement: agents polishing the harness
  instead of the work. Defense: keep posse thin; coordination lives in
  beads, oversight lives in herdr, and the harness stays too small to be an
  attractive nuisance.
- Yegge reports ~20–25% of all work going to harness upkeep. Budgeted.
- herdr is young (2026) and fast-moving; beads drags in Dolt. Both are
  acceptable because neither holds posse state hostage. The exit from any
  slot is a second occupant plus the interface the two need (ADR 0075 §2),
  not a shim kept ready ahead of time.
