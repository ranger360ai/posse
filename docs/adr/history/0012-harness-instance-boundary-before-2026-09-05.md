# Historical snapshot — not current policy

Archived 2026-09-05. Current decision: [ADR 0012](../0012-harness-instance-boundary.md).

# ADR 0012 — the harness/instance boundary, and the public split

*Status: accepted 2026-08-20 · executed at publication 2026-08-22 ·
owner: architect · amended 2026-08-24 (ADR 0013: D4.1 dispatch delivery) ·
amended 2026-08-28 (ranger-base-cqbq: App.A 5 reaches code-tree comments)*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> This is the ADR that created this repository. The original's leak
> audits, file-by-file scrub verdicts, and extraction bead lists are that
> instance's facts and are not restated; the boundary, the flow-in rule,
> the runtime contract, and the continuity conventions are, in full.

## Context

The operator's principle (2026-08-20): a work laptop should be able to
pull the harness, shape an instance for work — new secrets, new crew,
different *work-authorized* inference engines — and the harness should be
agnostic to all of it, while the smarts built in any instance (skills
authoring, spike practice, the dispatch disciplines) flow *into* the
harness. They then raised the stakes — a **new public project** made of
the agnostic parts — and this ADR answers whether that repo *supersedes*
the private development repo or merely *splits* from it, on migration
cost.

Hard constraint from the instance's security review (ranger-base-3jg):
the development repo's history carries the instance's cost, plan, and
credential telemetry across its committed bead database; re-publicizing
it would be a re-publication project. **The public repo starts clean** —
no inherited history, no bead database carried over, no doc text quoting
the instance's numbers, accounts, plans, or credential topology.

## Decision 1 — the boundary

Three layers, two repos. There is no third "product" repo today: nothing
exists that is neither mechanism nor instance facts. Create a product
repo when the first product-only artifact exists, not before.

| layer | owns | lives |
|---|---|---|
| **harness** (this repo — posse, github.com/ranger360ai/posse) | mechanism: dispatch loop, gates/cages L0–L4, tier resolution, skills *binding*, parity/degraded, verify-after, work-prompt shape, cost *arithmetic* and the uncounted-honesty rule, plan-guard *mechanism* (thresholds, blind clock, fail-open policy), PID/ORDERS *format* and lint, `posse init` seeding, example crew/skills/recipes/config | `cmd/`, `internal/rhq/`, `examples/`, `plugin/`, `etc/`, `scripts/`, `docs/adr/`, README/NOTES/DIRECTION |
| **provider adapters** (harness code, pluggable seams) | the shipped usage-endpoint + credential-read adapter, the shipped price table + transcript decoder, each runtime's launch template/realizer/detection manifest | same repo, behind the seams D4 names |
| **instance** (a private repo per deployment) | facts: secrets/env sets, the cast PIDs and ORDERS, config values (thresholds, caps, beads list, autostart arming), which engines are authorized, plan/billing reality, skills content encoding the instance's incidents, state/ | the instance repo (= RHQ_HOME) |

The plan guard answers its own question: the **mechanism** (skip a pass
above a utilization threshold, blind-window fail-closed when unattended)
is harness; the **provider's usage endpoint and credential read** are an
adapter — one implementation the harness ships; the **thresholds and the
credential** are instance. Unset thresholds already mean "no request is
made"; an instance on an unadapted provider runs guard-off until that
provider has an adapter (D4).

## Decision 2 — how smarts flow in

The rule is the security review's test (ranger-base-3jg), transposed from
beads to capability:

> A smart is harness-worthy iff **any deployer could have written it** —
> it is mechanism or method, and it survives with the instance's facts
> removed.

Mechanically, "facts removed" means: measured numbers become config
defaults with the rationale restated (not the measurement quoted);
private-tracker ids become ADR-section citations; incident narratives
become the invariant the incident taught; persona names become roles.
CONTRIBUTING.md carries the operational form of this rule verbatim, under
"Upstreaming from a private instance".

Skills are the worked case. The harness ships `examples/skills/` seeded
with the **generic canon** of distributed-systems — in the development
instance's audit, roughly two-thirds of every reference file was portable
field canon, with the instance matter concentrated in a "this shop's
answer" tail per file. The canon ships; the tails stay instance-side as a
local appendix. The same split applies to any future skill: authoring
method (trigger-shaped description, index + references, primary-source
honesty rules) is harness doc; content citing private history is
instance.

The contribution path: a private instance discovers → generalizes by the
rule above → lands here by ordinary commit/PR. Two backstops against
leak-by-habit: the instance-side routing rule (ranger-base-3jg) and the
visibility guard (rangerhq-hrz — a pattern lint on bead databases bound
for public repos), which must be **live before any public bead database
exists**.

## Decision 3 — supersede, not split (and the queue stays private)

**Supersede.** This public repo is the harness's one home; the private
development repo is archived read-only, private forever, its git history
the provenance of every pre-split decision. One deliberate simplification
removes most of the supersede's cost: **the public repo carries no bead
database at launch.** All beads moved to the originating instance's
private tracker; public participation starts as GitHub issues/PRs, and
the coordinator files instance-side beads from them. A public bead db is
an *additive later step*, taken only behind the hrz guard and the flow-in
rule.

Why supersede beats split, on cost structure rather than taste:

- **Split's cost is permanent and compounding.** The private repo stays
  the dev home, so every future commit and ADR needs a
  where-does-this-go decision; every public sync is a recurring scrub
  review; and the public repo is a mirror nobody works in — PRs land
  against stale code. Worst, it preserves the named root cause verbatim:
  harness development and instance operation conflated in one repo *is
  what leaked* the telemetry.
- **Supersede's cost is one-time and boundable.** Extraction + scrub was
  a bounded bead set; the queue migration, with no visibility split to
  adjudicate, was a mechanical export/import plus a config edit, with the
  id-preservation mechanics spiked before cut-over day.
- **Id continuity is a convention, not a migration.** The archive stays
  the resolver for its own ids: private-tracker ids in code comments and
  ADR citations remain as **inert provenance markers**, documented once
  in HISTORY.md. Nothing dangles because nothing promises to resolve.
  New public work cites public ids.
- **The point of no return is publication, nothing else.** Until the repo
  went public, every step reversed by repointing config back at the
  archive.

What would have changed the recommendation to **split**: harness
development itself needing to be private (then a deliberate mirror with a
per-release scrub is the honest shape); an import spike showing ids/deps
cannot be preserved faithfully; or the harness reaching maintenance-only
commit rates.

**Where dispatch lands** (amended 2026-08-22): dispatch's cwd follows the
bead's *database*, not the code — the `beads:` entry that answers a query
supplies the `Dir` that becomes the session's cwd, the session name, and
the cwd of every bd call for the bead. With the queue private and the
code public, pointing `beads:` at the instance repo would put the wrong
cwd under the dominant class of work. The decision: `beads:` lists the
public working copy *alone*, which carries a `.beads/redirect` file
holding the **absolute** path to the instance's `.beads` (relative forms
do not resolve across repos — verified). bd invoked in the public tree
then reads and writes the instance's database. The tracked `.gitignore`
covers `.beads/`, so neither the redirect (whose path is an instance
fact) nor any jsonl can reach the public repo; the hrz guard is the
second cover.

What the redirect buys: **one store of record** — the bd database,
versioned in the instance repo — with the redirect as a second mount
point, not a second store (the single-writer discipline ADR 0011 bought;
a second queue store was rejected there by name); the right cwd for code
work; and duplicate dispatch impossible by construction — one `beads:`
entry means one `Dir` per bead (`ReadyAll` does not de-duplicate across
entries, so two entries serving one database is structurally unsafe —
measured). Exit hatch: delete the redirect and list the instance repo in
`beads:` directly — two config lines, no state held hostage, the
databases never mixed. When the additive public db arrives, the redirect
gives way to a real public database.

Rejected for the queue: a gitignored *local* `.beads/` inside the public
tree (a second, unversioned queue store — the shape ADR 0011 rejected —
parking instance telemetry one `.gitignore` mistake from the exact leak
this ADR exists to end); and both repos in `beads:` (one bead, two
`Dir`s, dispatchable twice — structural, not fixable by discipline).

## Decision 4 — the runtime contract

A work-authorized engine must be addable **without patching the
harness**. The route is declarative-first: extend `runtimes/<name>.yaml`,
not a Go plugin API (rejected below). The minimum contract a runtime must
satisfy:

1. **Launch**: one typed command line, rendered from a template over the
   closed placeholder set (`{file} {memory} {model} {skills} {allow}
   {deny} {settings}`), that delivers the PID (system-prompt flag / config key /
   rules flag — template text either way) and, for interactive `posse
   new`, starts idle awaiting a typed prompt. **Dispatch** is a different
   delivery: when the runtime declares `prompt: argv` (ADR 0013 §2) the
   work prompt is appended as `"$(cat <file>)"` after that line — no new
   placeholder, because an unrendered `{prompt}` is a literal argument
   to the CLI. Headless flags (`-p` / `exec`) are not this path.
2. **Unattended approval**: template flags must make it approve tool calls
   with nobody watching, or the instance accepts permanently-blocked
   sessions.
3. **The wall**: tolerate PATH-prepended shims and the `SHELL=` gate shell
   (ADR 0009), or declare `gate_shell: false` and pay the parity price. L0
   native flags are politeness and therefore *optional* — a runtime with no
   realizer loses nothing the wall doesn't cover.
4. **Tiering**: `model_strong/standard/fast` + `model_flag` (declarable
   today, but only in `--flag value` shape).
5. **Skills**: a flag surface (`skills_flag:`) or cwd discovery, or accept
   Degraded under ADR 0007's declared-means-required.
6. **Detectability**: herdr must recognize the exe — an agent-detection
   manifest keyed on argv0. Without it the session is `agent_not_found` and
   dispatch cannot address it. This is the hardest requirement and partly
   outside posse; it becomes a documented authoring step plus a preflight.

At the time of this decision the yaml could declare 6 keys; the audit
found 17 touchpoints a third party could not supply. The gap closes in
two code tracks plus the two provider seams:

- **yaml v2** — printf-shape `model_flag`/`skills_flag` (`-c model=%s`),
  `skills_cwd:`, `self_sandbox:`, `project_config:`, unknown-key warning,
  parity wired to each.
- **runtime preflight** — `state_dir:` (feeds the seatbelt writable list —
  and note what that grant is, amended 2026-09-03 by ranger-base-rq83c:
  claude's `state_dir:` is `~/.claude` whole, so every caged persona can
  write `~/.claude/settings.json`, and a user-scope settings file's `env`
  block is applied over `process.env` of every claude that starts on the
  box afterwards, the operator's own uncaged session included. The grant is
  not narrowable — sub-file granularity fails the self-sandbox runtimes —
  so what answers it is a launch that pins the keys that matter at a scope
  the persona cannot reach: `{settings}` above, ADR 0019 D2's second
  preventive bullet. The credential dirs are pinned there today; whether
  that pin should widen is open, and tracked in the private queue),
  `env_required:` (checked at launch), declarable startup-screen
  dismissals, and `posse runtime check <name>`: exe on PATH, herdr manifest
  present, plus a manifest-authoring doc. "exe on PATH" is amended
  2026-09-05 by ranger-base-8vys9: the lookup runs in the POSSE process, and
  the pane a launch opens is a child of the herdr daemon and resolves in the
  daemon's environment, so the two answer different questions and herdr
  publishes no route to the one that decides a launch. That gap therefore
  REPORTS and never refuses. It names the PATH it looked on and sends the
  reader to `posse runtime probe` (ADR 0032 §1), which opens a real pane and
  measures the CLI the session actually launches — the only reading here that
  asks the PATH a launch resolves in. An empty `command:` still blocks: it is
  not a PATH question.
- **plan windows seam** — `PlanUsage` becomes `[]Window{Name, Pct}`;
  budget.go is label-agnostic arithmetic; the shipped usage-endpoint and
  credential-read adapter is one implementation; no adapter ⇒ guard
  cleanly off with one explicit line. (Routing around a tripped meter is
  ADR 0010's overflow.)
- **cost seam** — provider surface = price table + transcript locator +
  record decoder; everything downstream of `[]*Segment` is arithmetic.
  Uncounted-never-zero stays the default for a provider with no adapter.
  For providers that report *cumulative* cost snapshots, take the max,
  never sum.

Out of scope, deliberately: container-cage image baking and first-run
seeding for third-party CLIs (the cage is optional for a work install;
document, don't generalize yet), and per-runtime egress allowlists (ADR
0002's table, still design-only).

## Venue restrictions (folded from 0037)

A dimension is public harness material; a deployment fact stays in its
authorized private instance. Publish key schemas, probe mechanics and
anonymous mechanism fixes. Keep vendor identity, command strings, model
ids, dialog text, pane captures, filled runtime declarations, fixtures and
probe records in the authorized venue, even if someone offers to scrub them.
An authoring instance that is not authorized to run the engine must not
register its runtime profile; a private runbook may carry a skeleton for
use only at the authorized instance. Capability is not venue permission.

The authorized instance owns its measured grid and probe record. No record
transport is required: only a permitted generalized bug or coarse verdict
can return, under the same audience boundary. Its work waits for the venue
and an installed release containing required mechanisms. [0013 §9](../0013-runtime-dispatch-contract.md)
owns onboarding mechanics. Rejected: testing in a forbidden venue, a filled
public example, or a second record store across the boundary. This fold
removes no code, state, key, actor or flag; transport value is ASSUMED with
no consumer. Dated venue evidence remains in 0037.

## Lineage

| Was | Here |
|---|---|
| 0037 §§1–3 dimension/fact split, anchored evidence, venue sequencing | Venue restrictions; runtime checklist belongs to 0013 |

## Decision 5 — what the public project ships

- **README that is not development notes** (product voice, install,
  architecture), **CONTRIBUTING** with issue expectations and the D2
  flow-in rule, and a provable install.
- **Licence: Apache-2.0** — the operator's decision at publication
  (patent grant; NOTICE overhead accepted).
- **beads is a hard dependency** of dispatch, stated plainly. posse is a
  dispatcher over bd by design (DIRECTION: "posse never grows an issue
  tracker"); an adapter layer would be speculative generality. Exit hatch
  named per this shop's own rule: the bd surface posse uses is small
  (`ready`, `show`, `update --claim`, `close`, `comments`, `dep`,
  `create`, all via `internal/rhq/beads.go` — one file to shim if the
  substrate ever must be replaced), and durable state lives in bd's
  jsonl, hostage to nothing.
- **Distribution**: public repo + release binary with embedded examples.
  Repo access alone is enough for a machine that may build Go;
  binary-only installs need the examples embedded in the binary (`posse
  init` seeds from them).
- **Secrets shape**: a work install makes the secrets architecture
  load-bearing — at work the keychain question stops being a preference.
  This ADR does not decide it; it records the dependency: a work install
  is blocked on that decision, which starts on the operator's word only.

## Decision 6 — migration and continuity

- **ADR numbering carries over.** This repo restates the archive's ADRs
  under their existing numbers, each with a provenance header.
  Cross-references like "ADR 0009 §1" — which saturate NOTES and the
  code — survive unchanged. New ADRs continue the sequence. HISTORY.md
  records which numbers are restated and which are reserved pending
  restatement.
- **Private-tracker ids in text**: inert provenance markers; HISTORY.md
  documents the convention (D3). No mass sweep.
- **Crew names in the shipped code tree** (amended 2026-08-28,
  ranger-base-cqbq): App.A 5 reaches every line cmd/, internal/, and
  etc/ ship — comments included, not string literals alone — because
  D2's "persona names become roles" carries no code/prose carve-out.
  The edge is the tree, not the syntax: docs/ and the root narrative
  files are the development record, where the crew are historical
  actors and the no-mass-sweep convention above governs them as it
  governs ids. In the code tree an archive id stays either way; the
  name beside it becomes a role or goes. The two rules are not in
  tension: D6 grandfathers *ids* (nothing promises to resolve), D2
  depersonalizes *names* (any deployer could have written it) — a
  comment reading "measured (rangerhq-lrnp)" satisfies both.
- **Cut-over sequencing** (an instance-side runbook owns the detail):
  land the visibility guard and the scrubs; independent pre-publication
  clearance of the extraction file set; create and publish the repo —
  *the point of no return*; freeze the dispatch loop; export **all**
  beads to the instance tracker (the live set alone does not move —
  most dependency edges point at closed beads, and bd silently drops any
  edge whose target is absent from the import); repoint config; rearm;
  archive the old repo read-only. The freeze means the crew survives
  cut-over as a quiet evening, not a live migration.

## Consequences

- A new instance becomes: clone this repo (or a binary), `posse init`,
  author PIDs from examples, declare a work-authorized runtime in
  `runtimes/<name>.yaml` + a detection manifest, point `beads:` at a work
  repo. No fork, no patch.
- The originating instance's harness work continues in its private
  tracker — invisible to this repo, which is the deliberate trade:
  transparency later, safety now.
- The example layer is load-bearing (it seeds every instance), which is
  why much of the extraction work was examples/defaults work.
- The restated ADRs shed the instance's telemetry; the archive keeps the
  full-fat originals, so nothing is lost, only re-homed.

## Alternatives rejected

- **Public bead db at launch, per-bead visibility split** (the
  transparency ideal, and the one the architect wanted): forces
  classifying every live bead before cut-over and puts a publication
  judgment inside every future `bd create`. The hrz guard makes adopting
  it *later* cheap; front-loading it buys risk, not capability.
- **Split (private dev home + public mirror)**: recurring scrub cost,
  dead-mirror contribution story, and it preserves the exact conflation
  that leaked. Right only under the three conditions in D3.
- **Three repos now (harness/product/instance)**: the product repo would
  be empty. YAGNI with a named trigger: first product-only artifact.
- **Go plugin API for runtime realizers**: L0 is politeness; the wall is
  L1/L3. A plugin ABI (build coupling, version skew) to generalize
  politeness is cleverness with no payload. Revisit only if a runtime
  arrives whose native flags are OS-enforced and must count toward parity.
- **Re-publicizing the archive after a history purge**: rejected by the
  security review (a purge cannot recall pre-flip caches, and the
  committed jsonl still carries the telemetry). The clean-repo constraint
  stands.
- **Fresh ADR numbering in the public repo**: breaks every "ADR NNNN §"
  cross-reference in NOTES and code comments for zero gain; the
  provenance header carries the same information.
