# ADR 0006 — Handoff shapes: collaboration as beads, nothing else

*Status: accepted 2026-08-18 · owner: architect*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> Persona names are restated as roles; an instance's own cast is the
> operator's to map onto them.

## Context

Every crew PID has a `## Handoffs` section that says *who* takes from
whom (the developer from the architect, QA from the developer's closes,
security from anyone `-l security`…). None says *what a handoff looks
like* — which bead, which label, which relation — and nothing in dispatch
notices when a handoff that should exist doesn't. The visible symptom: a
QA persona whose raw material is closes is never dispatched, because a
close creates no work; security findings sit untriaged; a design that got
built with a divergence leaves no trace the metrics can count.

ADR 0005 gave every persona the HANDOFF rung: *create a bead for the
other persona, `discovered-from` yours, comment it, continue.* This ADR
decides the **shapes** that rung produces for the four collaborations,
where each kind of message goes, and the one dispatch affordance the
substrate is missing.

Checked: `bd mail` delegates to an external provider (`mail.delegate`,
`BEADS_MAIL_DELEGATE`); with none configured, a persona typing
`bd mail send` reaches nothing.

## Decision

**1. Three channels, by what the message is about — no fourth.**

| the message is… | goes to | why |
|---|---|---|
| about *this* bead (progress, a decision, `ASSUMED:` `BLOCKED:` `REFUSED:` `DIVERGED:`) | `bd comments add <id>` | stays with the work; ADR 0005 Context says "read the comments" |
| *work for someone* (a fix, a verification, a design, a question) | a **new bead**, `-a <persona>`, their label, `--deps discovered-from:<id>` (+ `blocks:` when order matters) | the only thing dispatch routes and `bd ready` shows |
| a conversation | **nowhere** — `bd mail` is unused | no provider, and mail is invisible to dispatch and cockpit; a chat that matters becomes a comment or a bead |

Collaboration state therefore lives entirely in beads: `discovered-from`
is the handoff edge, `blocks` is the ordering edge, labels route, and
comment prefixes are the greppable events (the metrics catalog counts
them).

**2. The four shapes.**

| handoff | trigger | shape | closes when |
|---|---|---|---|
| **architect → developer** (design → build) | ADR committed | implementation beads `-l code -a <developer> --deps discovered-from:<design>`, `blocks:` between them for order, ADR path in each description; the design bead closes when the beads exist (not when they're built) | each build bead: on the closer's word + the verify bead (below). A build that must diverge: comment `DIVERGED: <what/why>` on the *build* bead; if it changes the design, HANDOFF `-l architecture -a <architect>` |
| **developer/ops → QA** (build → verify) | a bead with a label in config `verify_labels:` (default `code, devops`) is **closed** | one verify bead `verify: <title>` `-l qa -a <config verify_assignee:> --deps discovered-from:<closed id>`, description = closer, `close_reason`, commits (`git log --grep <id>`), and the closer's PID "done when" row for the bead's intent *(at `verify_batch:` N > 1: one bead per N closes — shape in the §3 amendment)* | QA closes it "verified" (comment `VERIFIED: <how>`), or files a bug bead `-l code -a <closer>` with a repro and closes theirs `escape` — the closed bead is never reopened by a persona (operator's call) |
| **anyone → security** (finding → triage) | anything that smells like exposure, at any time | bead `-l security -a <security persona> --deps discovered-from:<id>`, **priority = severity**: P0 exploitable now · P1 credential/exposure reachable · P2 hardening · P3 note; the security persona never edits — its output is beads: fixes `-l code` / `-l devops`, accepted-risk decisions ASK the operator (`-l risk`, ADR 0005) | fix bead closes → verify shape applies (it's `-l code`); a P0/P1 finding also comments `SECURITY:` on the origin bead so its holder sees it |
| **operator/product grooming** (cadence) | one `-l groom` bead per week assigned to the product persona, filed by the operator or their scheduling automation (posse does not schedule; `--watch` dispatches, it doesn't create) | the product persona re-prioritises, splits, labels (`tier:` per ADR 0003), files `-l architecture` beads where design precedes build, closes with `bd comments add` listing what moved | close = queue is honest for the week; the `queue-honesty` metric reads it |

**3. The one dispatch affordance: verify-after.** Convention alone would
leave the QA persona idle the first time a closer forgets. So the harness
files the verify bead: each dispatch pass (and `posse ready`) scans beads
with a `verify_labels` label closed since the last pass and, if none has
a `-l qa` child `discovered-from` it, creates one as above (comment on
the closed bead: `verify filed: <qid>`). A closer that files it first is
seen and not duplicated. Config `verify_labels:` (empty = off),
`verify_assignee:` (the QA persona). This is config-driven and one rule;
it is not a workflow engine. *(Amended 2026-08-20: a `--dry-run` pass
files nothing — dry-run shows routing without acting, and filing a bead
is acting. The code said so from the start; the record now does too.)*

*(Amended 2026-08-27, from ranger-base-f7pk/bh7q.)* Config gains
`verify_batch: N` (default 1) and `verify_batch_age:` (default 24h). At
N=1 the rule is exactly as written above, byte for byte; the seed config
ships the key commented out, so N > 1 is the operator's move
(ranger-base-bah7). At N > 1 one verify bead answers N closes:

- title `verify N closes: <id>, <id>, …` (a single close keeps
  `verify: <title>` unchanged);
- one `discovered-from` edge per close; priority = the batch's most
  urgent close;
- description = N sections, each exactly the block §2 quotes, under one
  trailer; `verify filed: <qid>` is commented back on every close.

The dedupe is per close, not per bead: each section opens with the
harness's own marker line, so a batched orphan from a timed-out create
still answers every close in it next pass. A trailing partial batch is
**held, bounded**: filed only when its oldest close reaches
`verify_batch_age:`. Both obvious alternatives fail — filing every
pass's leftovers makes N a ceiling, not a quantum (most passes see one
close, so `verify_batch: 4` would still file 1:1 and buy nothing), and
holding unbounded means a shop that goes quiet three closes into a
batch of four never verifies those three. Held closes need no new
store: the watermark does not advance past one, so the pending set is
exactly the closes it has not passed.

Batching is not a coverage cut — the same work is verified, in one
session instead of N; what N divides is the *filing* amplification.
Measured (ranger-base-1t7r): half of verify runs file real follow-up
work (`qa → code` = 0.49), so the 1:1 gate put the queue's branching
factor at ρ = 1.14, 90% CI [1.02, 1.25], which grows without bound at
any headcount; N=4 puts ρ at 0.875. The alternative rejected — dropping
`code` from `verify_labels:` — gets the same ρ for free but buys it by
cutting the catch, and the 0-reopens record is what this gate pays for.
§5 stands unchanged: batched or not, the verify bead never holds any
close.

**4. PID `## Handoffs` sections say the shape, not just the name.** Each
row becomes `who · label · what the bead must contain`, e.g. the
developer's: *hand to QA — nothing to do: verify is filed on your close;
hand to security `-l security` P≤1 when a change touches secrets, auth,
or egress.* Recommended rows for the whole example crew ship in
`examples/agents/*`; an instance's own PIDs are the operator's to update.

**5. Out of scope, named.** Persona-to-persona chat; approvals or sign-off
gates beyond "closed" (the verify is a bead, not a gate that holds the
close); a scheduler in posse; multi-repo handoffs (a bead lives in the
repo of the work — a handoff to another repo is `bd create` in that
repo's `beads:` dir with the origin id in the description; the `beads:`
list aggregates them already).

## Consequences

- `dispatch.go`/`beads.go`: verify-after — `ListAll` filtered by
  `closed_at > last pass` and label ∈ `verify_labels`, child check via
  `dependents`, `bd create` with the assembled description;
  fixture-tested; last-pass watermark in `RHQ_HOME/state/`.
- ADR 0005's Context makes every shape legible at the receiving end:
  `from:` names the origin bead, `design:` the ADR, and comments carry
  the prefixes.
- The metrics catalog gains the events it needs: `VERIFIED:`/`escape`
  for QA (`escapes-caught`), `DIVERGED:` for
  `designs-implemented-unchanged`, `SECURITY:` for
  `findings-surviving-triage`.
- Instance PIDs need their `## Handoffs` and `## Done` rows updated by
  the operator (the developer's stops saying "hand to QA" — it's
  automatic).

## Alternatives rejected

- **`bd mail` for persona-to-persona.** Unconfigured by default, and even
  configured it's a side channel dispatch can't see; the crew already
  has a queue that routes.
- **A `handoff:` frontmatter key or a routing table in config.** The
  edges are already in beads (`discovered-from`, labels, assignee);
  a second vocabulary is triple-implementing the substrate (ADR 0001).
- **Verify as a gate that holds the close** (bead stays open until QA
  passes). Turns every builder's close into a wait and makes `bd ready`
  lie about what's done; a verify *bead* keeps both honest.
- **The closer files the verify bead (convention only).** Silent failure
  mode is an idle QA persona; the harness rule costs one query per pass.
- **A dedicated `security` severity field.** bd has priority; mapping it
  is one line in the security persona's PID and this ADR.
- **posse schedules grooming.** A scheduler is a product; a weekly bead is
  a habit the operator already has.
