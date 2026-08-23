# ADR 0001 — Persona Intent Documents (PIDs)

*Status: accepted 2026-08-15 · amended 2026-08-18 · owner: architect*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> Persona names are restated as roles; the amendment that aligned this
> contract with that instance's signed-off PIDs is folded into the body.
>
> **Provenance.** The artifact this ADR defines — the Persona Intent
> Document, both the name and the binding of persona, intent, tools,
> guardrails, and metrics into one document — follows the Specify-phase
> artifact of the DISCOVER framework (<https://discover-framework.ai/>,
> "ICPD v2.0 — Intent-Centered Persona Design", published anonymously).
> The intent inventory this ADR's Context cites was produced with that
> framework's Discover and Identify phases. The document format itself —
> flat-YAML frontmatter, the fixed body headings, the plain-file
> constraint, the tool-rule semantics — is this project's own.

## Context

Personas are `agents/<name>.md`: a flat-YAML frontmatter block (`name`,
`description`, `command`, `labels`) and a markdown body that *is* the
prompt (`command:` renders `{file}` and `{memory}`; the whole file is
what `cat {file}` hands the CLI). The first crew skeletons were written
from job titles. An intent inventory (kept instance-side) now tells us
what each persona is actually *for*, which hard risk lines every persona
must carry, and that some intents want tool restrictions that a prose
guardrail cannot enforce (no autonomous spending, no publishing under
the operator's name, no unpermitted changes to deployed systems).

Downstream work needs a stable shape to build on: PID authoring,
work-prompt blueprints (ADR 0005), handoff shapes (ADR 0006), and
outcome metrics computed from bd history.

Constraints, all inherited and kept:

- **Plain files.** A PID must remain something a persona command can
  `cat`. No sidecar files required, no build step, no JSON.
- **Flat-YAML subset** (`internal/rhq/yamlflat.go`): top-level scalars,
  inline/block lists, one-level maps, `#` comments, double quotes
  stripped. No nesting, no multiline scalars.
- **`{file}` / `{memory}` rendering** and the launch precedence
  (persona command > recipe/`--cmd`) are unchanged.
- **Backward compatible.** Today's parser reads known keys and ignores the
  rest, so a PID with the new *keys* launches on the current binary; they
  gain behaviour when the developer beads land. The one exception is the
  new `{allow}`/`{deny}` placeholders inside `command:` — today's renderer
  would pass them through literally — so a template may only use them
  once the launcher bead has shipped (it is P1 for that reason).
- **Thin harness.** The harness binds; it does not grow a policy engine.

## Decision

A PID is the same file with more said in it. Two halves, one contract:

**Frontmatter carries what the harness reads.** Every key is optional
except `name`; every list is a flat-YAML list (block form preferred —
permission rules can contain commas; inline form is accepted, and only
an item showing a bad split — unbalanced parentheses — is warned about).

| key | form | read by | meaning |
|---|---|---|---|
| `name` | scalar | launcher, dispatch | beads assignee = durable identity (unchanged) |
| `description` | scalar | cockpit | one line, shown in listings (unchanged) |
| `command` | scalar | launcher | template; `{file}` `{memory}` **`{allow}` `{deny}`** |
| `labels` | list | dispatch | bead labels this persona picks up (unchanged) |
| `intents` | list | humans, ADR 0005/0006, metrics | slugs of the intents this persona serves, matching the operator's intent vocabulary (e.g. `design`, `review-design`, `spec-beads`) |
| `allow` | list | launcher | permission rules **added** to the repo-global allowlist |
| `deny` | list | launcher | permission rules **removed** regardless of any allowlist |
| `metrics` | list | scorecard | ids of the metrics (below) this persona is judged by |

`allow`/`deny` items use the Claude Code permission-rule syntax verbatim
(`Edit`, `Bash(bd:*)`, `Bash(git push:*)`, `WebFetch`) — one syntax, the
same one already in `.claude/settings.json`, so a rule can be moved
between the fleet floor and a persona without translation.

**Tool binding is a delta, not a replacement.** The repo's committed
`.claude/settings.json` is the fleet floor every persona in that repo
gets. `{allow}` renders to `--allowedTools <rules…>` (union with the
floor), `{deny}` to `--disallowedTools <rules…>` (deny wins over every
allow, including the floor and the operator's local settings). Both
render to the empty string when the list is empty, so a template that
mentions them costs nothing. Place them **last** in the template — the
CLI's `<tools...>` flags are variadic and would swallow a trailing
positional. This is deliberately claude-shaped: the default command is
already `claude …`; another runtime's template simply omits the
placeholders, and the launcher also exports the lists as
`RHQ_TOOLS_ALLOW` / `RHQ_TOOLS_DENY` (newline-separated) so a wrapper
script for any runtime can apply them its own way. That env export is
the exit hatch.

**Body carries what the model reads**, under fixed headings so authors,
reviewers, and downstream beads can rely on where things are. Order and
names are part of the contract; a section may be short but not absent:

```
You are <Name>, the <role> of the crew.           # identity line, first

## Who you are     what you decide/produce; bias; what you don't do
## Intents         table: intent · mode (crew|fleet|advisory) · done when
## How you work    the working method — bd first, read before write, outputs
## Guardrails      hard risk lines (verbatim, all four) + persona-specific
## Handoffs        take from / hand to whom, in what form (ADR 0006 refines)
## Done            definition of done; the closing bd commands
## Blocked         what to say and that you stop
## Memory          read $RHQ_PERSONA_DIR/ORDERS.md at start; what to append
## Metrics         the ids from frontmatter, in words, with the bd query idea
```

Extra sections beyond the contract are fine; the linter checks presence
and order of the contract headings only. (`## Who you are` was first
drafted as `## Role`; the signed-off instance PIDs said it better, and
by this contract's own rule — the PIDs are the contract; the document
and code follow them — the heading is `## Who you are`, with no alias.)

`## Intents` is the heart. Each row is one intent from the operator's
inventory with the **mode** it runs in and a one-line **done when** —
the sentence a reviewer (QA, the operator) can check the closed bead
against. `mode` lives per intent, not per persona, because the same
persona works fleet for one intent and crew for another. Dispatch does
not read mode today; when it does, `advisory` is the first thing it will
gate (advisory personas never commit).

`## Guardrails` always restates the four hard risk lines (money ·
publishing under the operator's name · deployed systems · visibility) —
verbatim, so an audit can grep for them — and then the persona's own.
Where a guardrail can be expressed as a tool rule, it appears **twice**:
prose in the body and a rule in `deny:`. The prose explains; the rule
enforces.

**Metric catalog is derived, not declared.** The catalog is the union of
every loaded PID's `metrics:` plus config `metric_ids:` — a persona
declaring how it is judged is the source of truth, and the linter flags
*near-duplicate* ids across PIDs so the vocabulary stays one. The
scorecard reports each declared id as either computed (it has an
answerer) or `declared, not yet computable: <what bd would need>`.
Starter ids, honest and derivable from `bd` history:

| id | reads as |
|---|---|
| `closed-no-reopen` | beads this persona closed that were not reopened within 14 days |
| `findings-surviving-triage` | beads this persona filed that were accepted (not closed as invalid/duplicate) |
| `designs-implemented-unchanged` | design beads whose implementation beads closed without a design-divergence comment |
| `blocked-honestly` | dispatches that ended blocked *with* a stated need vs. silently idle |
| `spec-clarity` | beads this persona specced that the implementer closed without a "clarify" comment |

Metrics are honest, few (1–2 per persona) — no persona is measured on
anything the substrate can't see.

**Runtime variants are not personas.** `codex.md` / `grok.md` are a
`command:` choice, not a mindset. They are retired as agent files; a
runtime is chosen per PID (since ADR 0002, `runtime:` rather than a
hand-written `command:` — PIDs that still carry a claude-shaped
`command:` should drop the line and inherit the built-in template). If
two runtimes of one persona must coexist, that is one PID and two
recipes.

## Worked example

`examples/agents/architect.md` in this repo is the reference PID
(generic; an operator's own PIDs live instance-side). Its frontmatter:

```yaml
---
name: architect
description: software architect — designs before the crew builds
command: claude --append-system-prompt "$(cat {file})" --add-dir {memory} {allow} {deny}
labels: [architecture, design, adr]
intents:
  - design
  - review-design
  - cut-implementation-beads
allow:
  - Bash(bd:*)
  - Bash(git log:*)
  - Bash(git show:*)
deny:
  - Bash(git push:*)
  - Bash(git push --force:*)
metrics:
  - designs-implemented-unchanged
  - closed-no-reopen
---
```

Note what the file does *not* have: no per-project paths, no secrets, no
mention of the operator's projects — those bind at launch through env
sets, `--add-dir {memory}`, and the bead itself.

## Consequences

- **Today**: `intents`, `allow`, `deny`, `metrics` are inert-but-safe on
  the current binary; `{allow}`/`{deny}` in `command:` need the launcher
  bead first. Authoring can start immediately — write the lists now, add
  the placeholders to `command:` when the launcher lands (or leave
  `command:` out and inherit the default).
- **Launcher** grows ~30 lines: parse three lists, render two placeholders,
  export two env vars. Tests in `agents_test.go`. `DefaultAgentCommand`
  gains `--add-dir {memory} {allow} {deny}`: the memory dir is always
  materialized anyway, and the placeholders render empty for legacy
  files, so no existing agent changes behaviour beyond seeing its own
  memory dir.
- **`posse agent new`** scaffolds the PID shape (frontmatter keys present
  and empty, headings present with one-line hints), so a new persona
  starts as a PID rather than a job title.
- **Guardrails become partly enforceable** at zero policy-engine cost:
  the CLI already does deny-wins. What cannot be a tool rule (e.g. "no
  publishing under the operator's name" — the model can still draft)
  stays prose and is checked by review, which is what the `## Handoffs`
  section and ADR 0006 are for.
- **Prompt cost**: frontmatter is in the prompt (`cat {file}`). Fine at
  this size; a PID that grows past ~150 lines should move detail into
  the persona's `ORDERS.md` (memory), not the PID.
- **Vocabulary coupling**: `intents:` slugs and metric ids are strings
  matched by convention with the operator's inventory and the scorecard.
  Deliberate — a registry would be harness self-refinement. If drift
  hurts, `posse agent check` can lint against a list in config.

## Alternatives rejected

- **Sidecar `settings.json` per persona, `--settings {memory}/settings.json`.**
  Enforces the same rules with zero parser change, but breaks "one plain
  file": tool bindings would live outside the PID and out of the diff a
  reviewer reads. Kept as a *migration* trick if a rule ever needs a
  settings feature the CLI flags lack (hooks, env).
- **Full YAML / nested frontmatter** (`tools: {allow: [...], deny: [...]}`).
  Would need a real YAML parser in both Go and the bash fallback; the
  flat subset is a documented product decision.
- **A `mode:` key in frontmatter and dispatch gating on it.** Mode is per
  intent, not per persona; and dispatch already dispatches crew personas
  (this ADR was written by one, dispatched). Body table instead; revisit
  when dispatch grows a policy.
- **Machine-readable intents with routing** (route beads by intent, not
  label). Beads speak labels; inventing a second routing vocabulary is
  triple-implementing the substrate. `labels:` routes; `intents:`
  describes.
- **A separate `pids/` directory or `.pid.md` extension.** Same file
  format, one directory, no migration; the "I" is what's *in* the file.
- **A declared metric catalog the linter enforces** (the first draft).
  Rejected by the amendment's ruling: the signed-off PIDs are the
  contract, and a linter that rejects a persona's own declaration of how
  it is judged has the authority backwards. Derived catalog,
  near-duplicate warnings only.
