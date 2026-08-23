# DIRECTION — where posse is going

*Status: architecture settled 2026-08-11; all three sequencing phases
below have landed (last refreshed 2026-08-17). The
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

## The stack

```
posse        the business harness (bespoke, this repo)
             personas · env sets · skills · dispatch · launcher UI
  │
  ├─ beads   work substrate — dependency-aware task graph, agent mail,
  │          project memory        github.com/steveyegge/beads (MIT)
  │
  ├─ herdr   presentation & oversight — panes, layout, live agent state
  │          (working/blocked/idle), remote     github.com/herdrdev/herdr (Apache-2.0)
  │
  └─ agent runtimes — claude code, codex, … (interchangeable labor)
```

Each layer has a clean job:

- **herdr** is the presentation and oversight layer: it owns terminals,
  layout, and *ephemeral* state — which agent is in which pane and whether it
  is working, blocked, or idle right now — so the operator can see at a
  glance what is running, where, in parallel. Its CLI/socket API is the actuation surface
  (`agent start` / `agent prompt --wait` / `agent wait` / `pane read` /
  `events.subscribe`, per-pane env injection).
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
tmux-reference branch as of 2026-08-11. The exit hatch DIRECTION originally
promised — a backend seam — turned out to be cheaper as data than as code:
everything posse knows about a session is a flat meta file under
`state/herdr/`, and all durable state lives in beads, never in the
multiplexer. If a better multiplexer ships (Superlogical's
libghostty-based one is still the obvious candidate), the rewrite is the
one file that talks to herdr, not the harness. **Decided 2026-08-17:**
tmux-reference is retired — a dead-end reference, kept but not maintained.
It is not a fallback and there is no bead to keep it building; the exit
hatch is the meta-file design above, not a second backend.

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
grows a *dispatcher* over one.

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

- **crew** = the 2×2 grid slots — personas the operator actively talks to
- **fleet** = background sessions (~20 today) — workers grinding the queue
- **cockpit** = a herdr plugin pane (`posse cockpit`) showing `bd ready`
  alongside live sessions, with dispatch from the keyboard

The dispatch loop is the entire harness core, and it is small because the
substrates do the hard parts:

```
bd ready
  → route bead to a persona (by label/type)
  → find-or-create workspace · agent start <persona> (env set injected)
  → bd update --claim · agent prompt "work bd-xxxx" --wait --until done|blocked
  → done:    bd close → next bead
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
  acceptable because neither holds posse state hostage — the exit from
  any substrate is a backend shim, by construction.
