# ADR 0034 — Cached plan hints inform display, never the guard

*Status: accepted; simplified 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · display KEEP as built; abandoned guard clauses withdrawn.*

## Decision

Keep `PlanHint` and the concrete `planhint_codex.go` reader. Read the newest
`rate_limits` event in local rollouts, retaining its own timestamp, duration-
named windows, percentages, reset times and credits metadata. Name windows
`codex_5h`, `codex_7d`, otherwise `codex_<N>m`; provider slot order is not
identity. Missing data means no hint, not a blind meter or exhausted pool.

Cockpit and `posse cost --plan` display the hint with its age. Omit the segment
when no reading exists. After a window's reset time, show reset rather than
the stale percentage; that display does not assert current account headroom.
Preserve the existing live-meter segment separately.

A hint does not implement `PlanReader`, enter `planAdapters`, populate the
request-rate snapshot `plan-usage.json`, start a blind clock or gate dispatch.
The local file can lag activity on another machine; its age alone cannot
bound account-wide staleness. Keep the type distinction and concrete reader,
with a new interface only if an actual second reader justifies one.

Former D4 (overflow advisory/refusal and credits floors) and D5 (special
threshold matching for `plan_guard_codex_*`) are withdrawn, not pending
implementation. All launch/brake decisions belong to
[ADR 0010](0010-plan-guard-overflow.md); cached display creates no threshold exception.

## Disposition and evidence

The 2026-09-01 consolidation ruling on ranger-base-ay3dr dropped D3–D5.
The operator then resolved ranger-base-ntvtx: **KEEP D3 as built** on
ranger-base-ntvtx's cited display commit `088ddeb`; D4 and D5 remain DROP.
This records the final ruling, not an inference from code presence. The
2026-09-05 simplification leaves that resulting behavior intact.

The original 163-rollout census measured rate-limit shape, duration-slot
drift and credits on its own account. Newest-scan cost and cross-device
staleness were assumptions, not current guarantees. These dated findings
justify cautious telemetry, not a second guard. No probe was rerun here.

## Consequences and alternatives

Zero runtime keys, states, actors or flags removed; no code bead required.
Removing the reader would lose useful telemetry. Promoting it into a guard
would add provider-specific blindness and stale-file decisions without an
authoritative reading. A provider-keyed snapshot or full multi-meter guard
is not needed for the existing display.

## Lineage

| Record | Surviving decision |
|---|---|
| 0034 D1–D3 | Concrete age-bearing display hint |
| ranger-base-ay3dr, ranger-base-ntvtx | Display retained; guard/threshold clauses dropped |

Prior design and dated claims: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0034-codex-plan-hint.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
