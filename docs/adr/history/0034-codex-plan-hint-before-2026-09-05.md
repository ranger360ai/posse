# Historical snapshot — not current policy

Archived 2026-09-05. Current decision: [ADR 0034](../0034-codex-plan-hint.md).

# ADR 0034 — Codex plan hint: the on-disk meter informs, the guard still reads one live meter

*Status: accepted 2026-08-28 · owner: architect · amends ADR 0010 §3/§4
(the cap gains an advisory) · leaves ADR 0012 D4 and ADR 0018 unamended ·
ranger-base-lmvr, from rangerhq-0va item 4*

## Context

Every codex `token_count` event in `~/.codex/sessions/*/rollout-*.jsonl`
carries `payload.rate_limits`: per-window `used_percent`,
`window_minutes`, `resets_at`, plus a `credits` block that says whether
an exhausted pool starts billing (this account: no — a hard ceiling).
That is the same reading `planusage_anthropic.go` fetches for Claude —
on disk, free, no credential. Item 4 of rangerhq-0va asks for both pools
in the cockpit header.

The seam runs one adapter per instance (`PlanAdapter()`), and adding
codex to `planAdapters` makes it a fallback, not a second pool. Wiring
it as a second *guard* raises the bead's five questions: cross-provider
tightest-window, half-blind, stale-file gating, a provider-keyed
snapshot, and the overflow ladder's meaning.

Two facts decide all five. First, the reading is a **snapshot outside
its store of record** (Helland, CIDR 2005 — data on the outside "is
clearly from the past"): it is as stale as the last codex turn on this
box. Second, the staleness is **unbounded in the dangerous direction**:
the pool is account-wide, the rollouts are box-local, so codex on any
other device drains the pool without this file moving. A local-only
argument ("any turn here refreshes it, idle utilization only decays")
is true and insufficient. A fact like that may inform; it must not gate
a fleet, and it must never be handed a blind clock — blindness means "a
meter exists and could not be read", and this file being absent means
codex was never used here, which parks nothing.

## Decision

**1. The guard's seam stays singular; codex enters as a typed hint.**
A new provider file (`planhint_codex.go`) reads the newest `rate_limits`
event across the rollouts and returns a `PlanHint`: duration-named
windows each with `used_percent` and its own `resets_at`, the reading's
own event timestamp as `At`, and the credits block. Nil when no rollout
holds one. It does **not** implement `PlanReader`, is never listed in
`planAdapters`, is never written to `plan-usage.json` (a local file
needs no request-rate cache; a second provider in that snapshot is a
third copy of a fact that already has two stores), never starts a blind
clock, and never parks a pass. The type split *is* the policy: what can
gate and what can only inform are different Go types, so no future
caller promotes the hint by accident.

**2. Windows are named by duration, never by slot.** `codex_5h`,
`codex_7d`, else `codex_<N>m`, derived from `window_minutes`. Measured
on this box: primary was the 5h window Jan–Jun 2026 and the weekly one
in Aug, so a slot-named threshold silently changes meaning when the
provider reshuffles; a duration-named one at worst goes unmatched,
which is already said out loud. The vocabulary lives in the one
provider file, per ADR 0012 D4.

**3. Display: a hint is never shown without its age.** The cockpit
header and `posse cost --plan` render a codex segment when a reading
exists (`codex 7d 62%, as of 3h ago`); no reading, no line — absence of
codex use is not a fact worth a header row. Past its window's
`resets_at` the stale percent is never shown: the window renders as
reset. The claude segment is unchanged.

**4. The overflow ladder gains an advisory refusal; the cap remains the
brake.** ADR 0010 §3's cap stands unamended as the thing that bounds
spend, because it counts what *this instance* sent — the only quantity
this box actually knows. On top of it, when `plan_guard_overflow:
codex` and the operator sets a `plan_guard_codex_<win>:` threshold, the
per-bead ladder consults the hint before moving beads: a reading still
inside its `resets_at` and over the bar turns overflow off for the pass,
named with its age (`overflow codex: 7d 96% as of 2h ago — overflow off
this pass`). Fail-direction precedent is the unreadable-ledger rule:
refusing overflow costs one skipped pass and heals itself. No reading,
or past reset → cap-only, exactly today. The hint can only refuse,
never license. Two hardcoded floors, not knobs: overflow refuses when
`spend_control_reached` is set, or when a current reading shows ≥100%
with billing possible (`credits.has_credits` or `unlimited`) — an
overflow onto a pool that bills past its limit is autonomous spending
(hard risk line 1). The overflow print quotes the credits state either
way (`hard ceiling` / `bills past limit`).

**5. Threshold matching learns one carve-out.** `plan_guard_codex_*`
keys are the hint's, so `unmatchedThresholds` must not report them
against the live adapter's reading; with no codex reading on the
machine they still get the honest once-per-process line ("no codex
reading — that threshold gates nothing yet").

## Consequences

- Half-blind never becomes a state; ADR 0018's machine is untouched and
  stays sized for the one meter that carries last-brake duty.
- A static `runtime: codex` lane is *not* newly gated: its brakes stay
  the uncounted/cap machinery and 0va's cost accounting. If the shop
  ever routes a lane to codex statically, that — not this ADR — is the
  trigger to revisit gating, and the revisit starts from the
  cross-machine staleness argument above.
- ADR 0010 §3's calibration week gets cheaper: read the hint before and
  after instead of the provider's web UI. §4's open loop ("a usage
  endpoint, if one appears, is the loop closer") is half-closed:
  closed for refusal, open for licensing, deliberately.
- grok stays out: there is no on-disk meter anywhere in `~/.grok`
  (jared, rangerhq-aim), so the two-pool header is claude+codex and the
  hint is one concrete file, not a registry. Exit hatch: if a second
  on-disk meter ever exists, lift `PlanHint` production behind an
  interface then — speculative generality rejected now for ADR 0012's
  own reason.

## Alternatives rejected

- **Second entry in `planAdapters`** (the obvious one): first-available
  makes codex a fallback on credential-less boxes, not a second pool;
  and it hands a file with unbounded staleness the guard's gating
  authority and blind-clock semantics that are nonsense for it.
- **Full multi-meter guard** (the clever one: N adapters, per-provider
  blind clocks, per-runtime on-meter gating): symmetric and general —
  and general over an N that is provably 2, where the second member can
  never gate honestly (cross-machine drain) and has zero gating
  customers today (no static codex lane). Price: ADR 0018's state
  machine multiplied by provider, bought for a header line.
- **Provider-keyed `plan-usage.json`**: the snapshot exists to
  rate-limit a metering endpoint; a local read needs none of it, and
  each extra store of a fact is N−1 new ways to disagree.
- **Clamp-to-zero past `resets_at` and let the hint gate the main
  guard**: the monotone-upper-bound argument holds only per box; the
  operator's codex use on another device breaks it silently, and a
  guard that is usually right wears authority it does not have (the
  ADR 0018 estimate rejection, one layer up).
- **Probe for a live OpenAI usage endpoint** and wire it as a real
  second adapter: nobody has measured one, it needs a codex credential
  read posse does not do, and item 4's actual ask — the header — is
  answered by the file. Becomes worth a spike only if licensing
  overflow off a live reading is ever wanted.

## Claims

MEASURED (jian-yang, rangerhq-0va→lmvr, 163 rollouts on this box):
the `rate_limits` shape incl. per-window `resets_at`; `window_minutes`
drift (primary 300→10080 between Jan–Jun and Aug 2026); `plan_type`
drift (team→plus); credits `false/false/"0"` — hard ceiling on this
account; grok has no on-disk meter (jared, rangerhq-aim). ASSUMED: the
newest-rollout scan is cheap enough to run per header tick and per
overflow decision (walk date dirs newest-first; verify while building);
event timestamps are usable as the reading's `At`; the pool is shared
account-wide across devices (unverified, assumed in the safe
direction — it is why the hint never gates).
