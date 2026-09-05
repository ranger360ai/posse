# ADR 0051 — Stable citations and an on-demand SHA audit

*Status: accepted 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · commit-time SHA policing removed from the decision; implementation deferred.*

## Decision

Prefer a stable bead id for a change, with the commit subject when it selects
one of several commits. A citation helps find work; it does not prove landing.
`git log --grep <id>` can locate a message whose contents still need review.
The provenance limits of shared files belong to
[ADR 0022](0022-shared-file-single-writer.md), and verification's landing block belongs
solely to [ADR 0006](0006-handoff-shapes.md). A matching subject alone cannot
satisfy that block or establish that every named change reached the base.

SHAs may identify a measurement or historical evidence. State repository,
branch/base and date when claiming ancestry; a session commit is a session
commit until landing is checked. Rebase can change its name. In an incident
about stale names, retain the original and show a verified landed twin beside
it when one is available. Do not fabricate a twin when the object is absent.
Preserve verbatim evidence and date the limits of what can be resolved.

Remove SHA checks from `prepare-commit-msg` assembly. Committing an ADR must
not invoke citation-specific object lookup, ancestry classification or
patch-ID equivalence. Keep the existing **on-demand** `posse gates adr-census`
audit; its predicate and reporting can remain scoped to that explicit command.
No replacement commit marker, admission hook or automatic background audit.

The optional census reports the actual base, judged tokens, ancestors,
equivalent-patch cases and findings. Missing objects and an unavailable base
mean unjudged evidence, not clean or landed. Empty patch IDs are not twins.
This page does not demand that every dated historical token be made to resolve
in every clone. Audit findings are review inputs, with their coverage stated.

All executable, write-scope, visibility and data-ceiling commit boundaries
remain. [ADR 0054](0054-silent-revert-triage-survives-the-rebase.md) uses patch-ID evidence for a separate
operational silent-revert detector and is unchanged. Removing this editorial
gate does not authorize deleting patch-ID logic by searching for its name.

## Deferred deletion and acceptance

Delete `adrShaGuardBody` and its assembly path in `internal/posse/gates.go`,
plus hook-only wrappers/remedy prose and hook/census-equality tests that no
longer express a shared contract. Retain `AdrCensusScript` and what it actually
uses, including the existing predicate if appropriate. Adjust focused fixtures
to show stale/session/absent-object citations do not block commits, while the
on-demand audit still distinguishes findings and unjudged coverage. Never
remove a shared diff reader used by the visibility guard.

Price: approximately 1–3 source files plus tests; all citation ancestry and
equivalent-patch branches leave the commit path. No key, store, background
actor or new flag. Audit code is retained, so this is not a promise to delete
every SHA helper. Hook rendering/install follows its normal later promotion;
no installed hook is changed during documentation execution.

First done-when row: **number of commit refusals in the last 30 days that
prevented a materially misleading landing claim which ordinary review would
have accepted.** Record cases and the limits of that counterfactual judgment;
zero observed is not a claim of universal zero benefit. The review measured
stale names, not the necessity of blocking every commit.

What breaks if wrong: stale or misleading references survive until review or
an explicit audit. Citation/landing discipline remains, but its enforcement
is editorial at that point. Verify unrelated commit protections and 0006's
landing evidence still operate, without rebuilding another citation gate.

## Dated evidence and alternatives

The 2026-09-02 census found 12 non-main names among 32 resolving commits,
all with equivalent patches; 48/134 recorded landings had rebased. This
establishes a changing-name problem, not lost work. The 2026-09-03 census
reported 58 judged tokens, 12 admitted by twins and zero refused at its
recorded base. Those records are archived with their twin table; no census
was rerun here. A future clone may lack their pruned objects.

Reject blanket SHA stripping, since exact historical identities can matter;
reject a suite-wide ancestry pin, whose base and object availability differ
from a writer's; reject mandatory commit-time equivalence, whose machinery
and failure modes are the removal under this ruling.

## Lineage

| Record | Surviving decision |
|---|---|
| 0051 and its census/twin amendments | Citation convention, dated evidence and optional audit |
| 0006 | Sole verification/landing contract |
| Operator ruling 2026-09-05 | Removes editorial SHA admission from commits |

Prior gate design and stale-to-landed evidence: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0051-landed-is-a-bead-id.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
