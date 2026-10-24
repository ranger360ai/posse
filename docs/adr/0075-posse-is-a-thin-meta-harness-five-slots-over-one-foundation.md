# ADR 0075 — posse is a thin meta-harness: five slots over one foundation, and a slot becomes code only when a second occupant is being built

*Status: accepted 2026-10-24 (ranger-base-99a7u, operator direction 2026-10-24) · owner: architect · amends ADR 0012 D4 at one sentence (the runtime's pane is the multiplexer occupant's, not herdr's by name); restates, and keeps, DIRECTION's 2026-08-17 exit-hatch ruling as the general rule below · landscape and sources in `docs/notes.d/ranger-base-99a7u.md`*

## Context

The operator's direction (2026-10-24): extreme swappability, fast motion,
as thin as possible. Meta-harnesses are moving faster than posse can follow,
so posse leverages what others build at every layer under it, and keeps one
thing: the user's intent to run a persistent team of agents with memory and
orders, founded on PIDs (ADR 0001).

What each layer looks like in this tree today (MEASURED 2026-10-24, this
worktree at `482a1256`, 150 non-test `.go` files under `cmd/` and `internal/`):

| layer | occupant | how wide the seam is |
|---|---|---|
| multiplexer | herdr 0.9.1 | 21 files hold a herdr client or backend; 32 name a herdr type; 73 mention herdr. The client has 25 verbs, `HerdrBackend` 22. DIRECTION said 13 files on 2026-09-10. |
| runtimes | claude, codex, grok, bob | already a slot: four built-ins behind declarations and overlays (ADR 0013 §8, ADR 0060) |
| sandbox | seatbelt (L2), with the L1/L3 shims | L4 container exists in code; unrun since Docker left this box 2026-08-30, and unrealizable over a SQLite store (ADR 0074) |
| tasks | beads (`bd`) | 11 files name the `Bd` client; 23 verbs |
| connectors | none | each runtime reads its own MCP config; posse only seeds trust around it (`trust.go`) |

Two counts above need care. The 13 of 2026-09-10 has no recorded method;
the 21 is "files that hold or build a `Herdr`/`HerdrBackend`", the nearest
reading of "names the seam". Either way the multiplexer seam has widened
since DIRECTION said the rewrite would be the herdr client. And lcxr3's
"67 files" counted mentions; today's mention count is 73.

## Decision

**1. Five slots, one foundation.** posse names five layers as slots. Each
has exactly one occupant today and may have others. The foundation is not
a slot: it is what posse is.

| slot | today | eligible next (not commitments) |
|---|---|---|
| multiplexer | herdr | Rex (gated on GA, ranger-base-jusig), tmux, others |
| sandbox | seatbelt | remote sandboxes off this laptop; never local Docker |
| runtimes | claude, codex, grok, bob | any CLI that meets ADR 0013 |
| connectors | none | bound per PID, below |
| tasks | beads | a tracker that meets the contract in 4 |

Foundation: PIDs (ADR 0001, from the DISCOVER framework's persona
document), persona identity as a task-store assignee, per-persona memory
and ORDERS, the constitution and its promotion (ADR 0015), and the dispatch
loop that binds them (ADR 0011). These stay posse's whatever fills the slots.

**2. The slot rule.** A slot is a layer with a named occupant, a list of
the verbs posse uses, and no posse state held hostage: everything durable
lives in the foundation or the task store, never in the multiplexer, the
sandbox or the runtime. A slot becomes a Go interface only when a bead is
building a second occupant, and the interface is what those two occupants
both need. No second backend is built ahead of its occupant.

This is DIRECTION's 2026-08-17 ruling, stated for every layer. That ruling
survives. Its premise did not: "the rewrite is the herdr client" stopped
being true when the seam reached 21 files. The operator's "herdr, tmux, Rex
and others" names who may fill the slot. It does not commit posse to build
them. tmux-reference stays retired. Rex is the second occupant because it
is the one the operator gated on, and ranger-base-lcxr3 extracts the
interface when it starts. Until then, nothing is done to the seam: there is
no second occupant to shape an interface against.

**3. The runtime seam stays a PID-rendered argv in a shell (ADR 0013),
not ACP.** ACP runs the agent as a stdio subprocess that a client drives
over JSON-RPC. Its `terminal/*` methods run commands the agent asks the
client for; nothing in it hosts the agent's own TUI (VERIFIED-FROM-PRIMARY
2026-10-24, agentclientprotocol.com). Posse would lose the terminal, the
multiplexer slot and every
pane reader in ADR 0013 §2 and ADR 0057 to adopt it. Bob is the fourth built-in through
the argv seam (ADR 0060). ACP may be adopted later as a reading channel
(turn outcome, agent state) if a measured need appears. It is not the
dispatch seam.

**4. Tasks: bd is a slot, not the floor. posse still never grows a
tracker.** A second occupant would have to provide: ready-with-deps, an
atomic claim by assignee, close, comments, dependency edges, a count of its
own to check an empty answer against (ADR 0071), one writer per store and a
read-back of every write (ADR 0073), and a store of record that the session
env binds (ADR 0055). That list is the contract. Nothing implements it now.

**5. Connectors: when one is needed, posse binds and the runtime runs it.**
A PID names the connector, and the launch writes it into that runtime's own
MCP config, as ADR 0007 does for skills. That puts the choice in the
meta-harness layer, where the operator leans, without posse running a
gateway. No PID needs a connector today, so no key is added.

**6. Sandbox: a remote occupant must reach the store of record.** A
sandbox off this laptop would need the worktree, write access to the store
of record (ADR 0074 shows what happens without it), credentials minted per
ADR 0019, and L1/L3 realized inside it. No local Docker (operator ruling
2026-08-30).

**7. ADR 0012 D4, amended in place.** "A runtime remains a CLI in a pane
that herdr recognizes" now reads "a CLI in a pane that the multiplexer
occupant recognizes (herdr today, ADR 0075)".

## Consequences

- DIRECTION.md draws the stack as slots plus a foundation, and its 13-file
  claim is replaced by the measurement above.
- ranger-base-lcxr3 starts from the 21/32/73 counts and the slot rule. The
  verb it will find hardest is agent state: Rex documents none (see the
  notes fragment).
- No implementation bead is cut by this record. Slots 3–6 change no code,
  and slot 2's first application is already filed and gated.
- The cost is that the multiplexer seam keeps widening until Rex is GA.
  That is ASSUMED cheaper than shaping an interface now against one
  occupant.

## Alternatives rejected

- **Do nothing.** DIRECTION would keep a 13-file claim that is now 21, no
  sandbox or connector layer, and an open conflict between 2026-08-17 and
  today's direction. lcxr3 would start from both. One record and one doc
  edit cost less than that.
- **Extract all five interfaces now** (the one I wanted: one `Backend`
  per layer, the stack made explicit in code). Four slots have one
  occupant, and an interface written against one occupant is a guess. The
  multiplexer alone is 47 verbs. The simplicity order (2026-09-03) refuses it.
- **ACP as the one runtime protocol.** One protocol for every runtime is
  the elegant answer, and it is Paperclip's (its claude, codex, gemini and
  kimi adapters default to ACP in source). The client owns the session and
  no TUI is hosted, so posse would trade its terminal surface for it and
  become a second Paperclip.
- **Become Paperclip-shaped** (own task database, web board, connector
  gateway, budgets). Each part contradicts a standing decision: no tracker,
  no board, no dollar budgets (lcxr3 scope).
- **A connector gateway that holds credentials.** It is a new long-lived
  actor and a second credential owner beside the provider (ADR 0019).
  Binding into each runtime's MCP config gives the same swap with neither.
- **Revive tmux-reference as the second backend.** Frozen since
  2026-08-11, it has no agent-state detection, and the operator gated the
  second occupant on Rex, not tmux.
