# ADR 0001 — Persona Intent Documents (PIDs)

*Status: accepted 2026-08-15 · amended 2026-08-18 · amended 2026-08-29
(Consequences: the scaffold's `deny:` seeds the commit wall —
ranger-base-w1ny) · amended 2026-09-06 (the frontmatter schema table and
the transcribed worked example are struck for pointers at
`internal/posse/agents.go` and `examples/agents/architect.md` — they
named 8 of the parser's 19 keys; the shipped Consequences bullets are
struck; `## Intents` mode is recorded as governing nothing since ADR 0006
§4 — ranger-base-mppjc) · owner: architect*

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
- **Flat-YAML subset** (`internal/posse/yamlflat.go`): top-level scalars,
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

**The key schema is the parser's, not this page's** (amended 2026-09-06,
ranger-base-mppjc). The live list is the commented block at the top of
`internal/posse/agents.go`, beside `LoadAgent` which reads it; `posse
agent new` scaffolds the commonly authored subset of it with one-line
hints (not every key — `cage:`, `writable:`, `egress:`, `sockets:`,
`envs:` and `trust_project_config:` are authored when wanted). This ADR
decides only the four keys it introduced — `intents:`, `allow:`, `deny:`,
`metrics:` — and their semantics are below. Every other key is governed by the ADR
that decides its mechanism, checked 2026-09-06: `runtime:`, `cage:`,
`writable:`, `egress:`, `sockets:` and `trust_project_config:` by
[ADR 0002](0002-runtimes-and-gates.md) §5 (with `writable:`'s path
matrix in [ADR 0014](0014-path-scoped-writes.md) §5); `tier:` and
`tier_floor:` by [ADR 0003](0003-model-tiering.md) (Dials A–D);
`skills:` by [ADR 0007](0007-skills-binding.md) §1; `route_order:` by
[ADR 0011](0011-dispatch-model.md) §4; and `envs:` names env sets, whose
launch-order selection is [ADR 0039](0039-model-dial-follow-through.md)
D3d and whose store of record is [ADR 0019](0019-credential-architecture.md)
§1.

The table this section used to carry is struck rather than updated,
because a second copy of a parser's key list drifts and this one had:
MEASURED 2026-09-06 over `LoadAgent`, `CheckAgent` and `SkillDescription`,
the parser reads **19** frontmatter keys and the table named **8**; the
eleven listed above were live and unnamed here. (Eleven, not the twelve the
proposing bead listed: `overflow:` is a *config* key whose removal message
lives in `planusage.go`, and was never a PID key.) A pointer cannot drift;
if the count is wanted again, it is `git grep -oE 'yaml(Get|List)Lines\(front, "[a-z_]+"' internal/posse`.

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

`## Intents` is the heart, and it is heart-for-the-reader only. Each row
is one intent from the operator's inventory with the **mode** it runs in
and a one-line **done when** — the sentence a reviewer (QA, the operator)
can check the closed bead against. `mode` lives per intent, not per
persona, because the same persona works fleet for one intent and crew for
another.

**The mode column governs nothing, and neither does the rest of the
table** (amended 2026-09-06, ranger-base-mppjc). The original text said
"dispatch does not read mode *today*"; that `today` has been retired in
the other direction. [ADR 0006](0006-handoff-shapes.md) §4 ruled that a
guessed PID row may not stand in for a bead's own acceptance, and
ranger-base-0ezn7 deleted the last harness reader of this table —
`IntentDoneWhen`, `intentMatchesLabel`, `IntentRow`, `IntentRows` — on
2026-09-06. What is left is `pidcheck`, which requires the `## Intents`
*heading* and never parses a row. So no code reads `crew`/`fleet`/
`advisory`, an `advisory` persona is not gated from committing by
anything but its own `deny:` rules, and a wrong row here misleads a human
and nothing else. Write it for the reviewer; do not write it expecting
enforcement, and do not add a reader without an ADR that says why the
table beats the bead's own words.

`## Guardrails` always restates the four hard risk lines (money ·
publishing under the operator's name · deployed systems · visibility) —
verbatim, so an audit can grep for them — and then the persona's own.
Where a guardrail can be expressed as a tool rule, it appears **twice**:
prose in the body and a rule in `deny:`. The prose explains; the rule
enforces.

**A deny rule naming a FLAG walls one spelling, not one effect** (amended
2026-08-29, ranger-base-zs6b; residual corrected 2026-08-31,
ranger-base-e7eo). `Bash(git push --force:*)` reads like a force-push wall
and is not one: `git push -f` (a separately registered short name, not an
abbreviation of `--force`), `--force-with-lease`, `git push origin +main`
and `git push --mirror origin` all force-push and none of them carries the
token the rule names — **MEASURED**, git-push(1), git 2.50.1 — a floor on
the spelling set, not a count of it. `+main` has no option to spell at
all; `--mirror` is worse, because under `remote.<remote>.mirror` the same
force-update is what a bare `git push origin` does, with no option and no
refspec in the argv for any matcher to read. No amount of widening the
matcher reaches either. Where the *effect* is what must not happen, deny
the verb: `Bash(git push:*)`, which every PID in `examples/agents` carries
and posse's own tests require of them. Keep the flag rule for the sentence
it puts in the refusal, never as the wall. This generalizes: before
writing a flag rule, ask which other spellings of the same effect the tool
accepts, and if the answer is more than one, deny the verb.

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

`examples/agents/architect.md` in this repo is the reference PID (generic;
an operator's own PIDs live instance-side). **Read it there** — the
transcription this section used to carry is struck for the same reason as
the schema table (amended 2026-09-06, ranger-base-mppjc): it was a second
copy of a file the suite already pins byte-for-byte through the
shipped-example digest, so only the copy nobody tested could drift, and it
had — the quoted `command:` line is the escape hatch a PID no longer
writes, since ADR 0002 made `runtime:` the way to choose a launch profile
and this ADR's own body says so two sections up.

What the example is here to teach is what the file does *not* have: no
per-project paths, no secrets, no mention of the operator's projects —
those bind at launch through env sets, the memory dir the runtime's
template renders, and the bead itself.

## Consequences

*(The three bullets this list opened with — "today these keys are
inert-but-safe", "the launcher grows ~30 lines", "authoring can start
before the launcher lands" — are struck as of 2026-09-06,
ranger-base-mppjc. All of it shipped: `LoadAgent` reads every key,
`{allow}`/`{deny}` render through each runtime's own realizer rather than
a claude-shaped `command:`, and `RHQ_TOOLS_ALLOW`/`RHQ_TOOLS_DENY` are
exported. A consequence written in the future tense stops being a
consequence and becomes a false claim about the binary; the shipped shape
is `internal/posse/agents.go` and `agents_test.go`. What follows is the
part still live as consequence rather than history.)*

- **`posse agent new`** scaffolds the PID shape (frontmatter keys present
  and empty, headings present with one-line hints), so a new persona
  starts as a PID rather than a job title. One deliberate exception
  (amended 2026-08-29, ranger-base-w1ny): `deny:` ships one real rule,
  `Bash(git commit unless --)` — the L1 half of the commit wall
  (ranger-base-09b7). The rationale: L3 is a repo hook that exists only
  where something installed it, while a typed-line deny reaches every
  repo and runtime; deny-wins means a seeded rule can never grant
  anything; and the rule leaves open exactly the path-limited form
  (`git commit … -- <paths>`) that the single-writer convention
  requires (ADR 0022). "Shape but no policy" was measured as the gap,
  not the purity: a persona created from the all-empty scaffold carried
  no commit wall at all in a repo with no hook. This is the only rule
  the scaffold seeds — anything narrower than a crew-wide invariant
  still belongs to the authored PID, not the scaffold.
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
