# ADR 0026 — Research before invention, within the task when bounded

*Status: accepted 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · mandatory task multiplication removed; prompt/skill implementation deferred.*

## Context

Three attempts at a concurrency invariant and an incident evening produced
standard answers the field already knew. Research remains required when the
design lacks evidence. The review found no measured benefit from requiring
a separate research bead and downstream implementation beads for every gap.

## Decision

Read the relevant bound skill and references when about to invent a mechanism
or name, try a third repair of one invariant, commit to an expensive interface
or rely on an unmeasured number. A useful reference answers the question;
a missing answer calls for bounded research, normally within the existing task.
Research may share that task with implementation after the decision is sound.

Create a separate `spike: <question>` only for a distinct dependency or
deliverable: work another lane must do, an experiment needing its own venue,
or findings substantial enough to require an independent handoff. State its
time box (normally one session), question and stopping condition. Create
implementation beads only for actual remaining implementation slices, not
as mandatory proof that research occurred. No automatic trigger detector.

Every research result belongs in a committed ADR section or notes artifact,
not only a transcript. An empirical result labels numbers MEASURED or ASSUMED,
records environment/date, provides a repeatable probe when the experiment
needs one and leaves its venue clean. Literature work names the known problem,
standard answers and their failure modes, and records adopt/reject calls.
Reusable general findings can update the relevant skill's reference shelf.

Prefer named primary sources; label papers, docs and opinion. Verify outside
claims against accessible sources on the research date; model recall is not
evidence. Mark unavailable links, use a DOI or dated archive where appropriate,
and state what could not be checked. Existing dated evidence remains dated;
it is not silently upgraded to a fresh measurement.

## Queue mechanics, only when separate work is needed

The SPIKE rung in [ADR 0005](0005-work-prompt-blueprints.md) points here for the research
decision. A separate spike inherits its deciding task's priority and class,
uses the runner's lane label, and blocks the deciding task with
`bd --no-daemon dep add <deciding> <spike>`. Use a provenance comment
`discovered-from: <deciding>` on the spike, not a reverse dependency edge.
Confirm the block by reading the deciding task's dependencies. Readiness is
the dispatcher contract's ready-minus-blocked view, not raw `bd ready` alone.

The 2026-08-30/09-01 experiments found that cycle handling differs by store:
SQLite refused a reverse provenance/block cycle while a no-db store accepted
it without excluding the deciding task from ready. Keep the acyclic graph and
check the actual block. Continue independent work while a separate dependency
is pending; do not build a decision on an unresolved load-bearing question.

## Consequences and deferred acceptance

Price: approximately 2–5 text/prompt/skill surfaces, including the rendered
SPIKE rung in `internal/posse/dispatch.go` and its checks; fewer automatically
created tasks and block transitions. No runtime key, ticker, store or flag.
No files outside `docs/adr/` change in this session; the code task owns the
bound instructions and renderer. Instance-owned instructions require their
normal source/promotion path, not edits to a generated live copy.

First done-when row: measure the benefit claimed for mandatory task splitting
by a bounded task census: which separate spikes supplied a distinct dependency
or deliverable, and which only duplicated research that fit the deciding task.
Record window, sample and uncertainty; the review established no causal benefit.
Verify the revised rung still requires evidence, creates/blocks distinct work
correctly and does not invent acceptance by multiplying beads.

What breaks if wrong: small research loses an independently tracked artifact
and unresolved questions can disappear inside implementation. Committed
findings and explicit acceptance remain the controls. The historical estimate
that a spike cost half an implementation task is not a universal threshold.

## Alternatives rejected

- Mandatory spike plus downstream beads: task count is not evidence quality.
- Skip research on a shelf miss: preserves the repeated-invention failure.
- Research only in PID prose: the shared work-prompt rung must carry the rule.

## Lineage

| Record | Surviving decision |
|---|---|
| 0026, 2026-08-27/30 | Triggers, sourcing and acyclic separate-spike mechanics |
| Operator ruling 2026-09-05 | Bounded research may stay in its existing task |

Prior practice: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0026-research-spikes.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
