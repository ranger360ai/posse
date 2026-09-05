# ADR 0012 — Public harness, private instance

*Status: accepted 2026-08-20; publication executed 2026-08-22; simplified 2026-09-05 by operator ruling · owner: architect.*

## Context

Doing nothing leaves the executed extraction and old instance-home layout
beside today's write policy. Keep the ownership and publication boundary;
archive the one-time procedure. A new deployment can use the harness with
its own crew, authorized runtimes, credentials and operating values.

## Decision 1 — ownership and write policy

| Surface | Owns | Write policy / governing record |
|---|---|---|
| Public harness (`cmd/`, `internal/posse/`, examples, docs, scripts) | Generic mechanisms, adapter code, formats and portable methods | Any-deployer test below; 0024 visibility, 0050 separate data ceiling |
| Private constitution source (`posse/` subtree) | Versioned instance PIDs, config, recipes, runtimes and skills | Draft then operator promotion; source resolution and promoted set in 0015 |
| Runtime home | Promoted law, separate memory and private state/env sets | 0015 §§2–3/5/7; home is not the constitution repo |
| Private queue store | Durable work and dependency graph | 0015 §4; work repos resolve this same store, 0055 binds it in session env |
| Runtime-owned state | CLI configuration and rotating credentials | 0019 provider ownership; no posse-authored foreign-home files (0035) |

The public split is complete. This repo is the single harness development
home; the private archive stays private and resolves its historical ids.
Do not replay an extraction/import cutover or republish that history.

## Decision 2 — what can flow into the harness

An artifact is public only if **any deployer could have written it**:
mechanism or method that survives removal of deployment facts. Restate the
invariant, rather than scrub a deployment narrative. Instance values,
secrets, filled profiles and telemetry remain private. Generic skill canon
can travel; incident-specific appendices cannot. Code-tree names become
roles; archive ids remain inert provenance. When audience is unclear, keep
the artifact in the narrower venue. 0024 owns the artifact-routing details.

## Decision 3 — one work store, correct working directory

`beads:` names the working repo in which work should run. A redirect can
point it at the private store, without creating a second queue or listing
the same store twice under two launch directories. A redirect alone is not
a universal bd discovery guarantee: [0055](0055-store-of-record-rides-the-session-env.md)
requires the launcher to bind `BEADS_DIR` to the resolved store for every
session, consistently with its write grants. The queue's own tree and
commit policy belong to [0015 §4](0015-constitution-promotion.md).

A local database beside a redirect is the rejected second store, and it is
invisible while the redirect resolves: bd reads the target, every surface
is correct, and `bd ready` answers at exit 0 with a weeks-old graph the day
the redirect file is lost (the public checkout held one from 2026-08-24;
September 2026 adherence audit, finding 6). Amended 2026-09-05
(ranger-base-dj3k2): that rejection is **reported, not enforced**. `posse
status` and every `dispatch --watch` pass sweep each configured `beads:`
directory and print one line per finding naming the path, what is in it and
the fix (`internal/posse/secondstore.go`) — no refusal, no delete, no exit
code, because the exit hatch is two config lines and one `rm` the operator
types, and posse deleting a database nobody asked it to delete is a worse
incident than the stale graph it prevents. A `.beads/` with its own
database and no redirect is every ordinary bd repo and is silent; a
redirect bd will not follow takes a different sentence, because there bd is
already reading the local store.

## Decision 4 — runtime and provider seams

A runtime remains a CLI in a pane that herdr recognizes. Named declarations
and validated overlays are the extension surface; no plugin ABI or internal
inference loop. The sole executable contract/checklist, including argv-first
dispatch, safe unattended delivery and launcher-versus-pane PATH, is
[0013](0013-runtime-dispatch-contract.md). [0002](0002-runtimes-and-gates.md)
owns enforcement; [0019](0019-credential-architecture.md) owns credentials
and the launch settings pin protecting store resolution.

Plan adapters provide named percentage windows; policy is in 0010. No
adapter means guard off with witness. Cost adapters locate/decode transcripts
and price segments; downstream arithmetic is provider-independent. For
cumulative cost snapshots take the maximum, never sum. Missing pricing is
explicitly uncounted/unpriced, never zero. Authorized venues follow below.

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
and an installed release containing required mechanisms. [0013 §9](0013-runtime-dispatch-contract.md)
owns onboarding mechanics. Rejected: testing in a forbidden venue, a filled
public example, or a second record store across the boundary. This fold
removes no code, state, key, actor or flag; transport value is ASSUMED with
no consumer. Dated venue evidence remains in 0037.


## Decision 5 — distribution and dependencies

Keep Apache-2.0, source and release-binary distribution, embedded examples
for `posse init`, and bd as dispatch's work substrate. Its small command
surface is isolated in beads.go; the existing export/records preserve work
if an adapter replaces it. Runtime adapters and cage engines are replaceable
behind their contracts; none takes ownership of the queue.

## Decision 6 — continuity

Existing ADR numbers and provenance stay stable; 0040 owns consolidation.
Code-tree role naming includes comments, strings and paths; docs and root
narratives retain dated actors and inert ids under the publication boundary.
The executed publication/migration sequence is history, not an active task.

## Consequences and alternatives

MEASURED: publication and the later source/env-binding amendments are in
the repository record. This simplification removes no runtime files, keys,
states, actors or flags. ASSUMED: a short ownership table prevents replay
and misrouting. Rejected: doing nothing (contradictory directory accounts),
a recurring private-to-public mirror (ongoing scrub), a second queue copy,
or a product repo before a product artifact exists. Losing this boundary
would disclose private facts; shortening its migration history does not.

## Lineage

| Was | Here |
|---|---|
| 0012 original boundary, flow-in, distribution and continuity | Decisions 1–6 |
| 0012 executed extraction and queue cutover | Pre-simplification page in git history (below); not replayable instructions |
| 0037 §§1–3 | Venue restrictions |
| Later constitution source / explicit store environment amendments | 0015 / 0055, directly referenced above |

Executed migration and dated evidence: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0012-harness-instance-boundary.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
