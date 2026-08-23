# ADR 0010 — Plan-guard overflow: a second pool when the metered window is hot

*Status: accepted 2026-08-18 · owner: architect*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> The specific subscriptions, pool sizes, and prices that motivated this
> design are that instance's facts and are not restated; the mechanism
> assumes only the pool *shapes* described below.

## Context

The plan guard gave dispatch a whole-pass stop: above `plan_guard_5h:` /
`plan_guard_7d:` utilization the pass is skipped (`d.planGuard`, first
thing in `Run`). The guard reads **one provider's meter** — the shipped
adapter is the Claude subscription usage endpoint (ADR 0012 D4). It skips
every launch, including ones whose runtime is not on that meter — a
persona on a different runtime is skipped because the guarded provider
is hot.

Why a second pool at all, restated from the development instance's
numbers: on a subscription-metered provider the marginal cost of fleet
work inside the plan is zero; the binding constraint is the short
(5-hour) window, which the operator also shares interactively during
working hours. A second subscription pool on another runtime buys that
window back.

What makes the move dangerous, and drives every rule below: candidate
pools differ in **shape**. Some heal every few hours; some are **one
weekly bucket with no intra-week reset**, possibly shared with the
operator's own interactive use of that provider. None of them expose a
meter the harness can read. And their sandbox/parity properties differ —
a caged PID may be clean on one target and degraded on another.

Dial E (ADR 0003) is a *tier* ladder inside one runtime, triggered by
max(pass$, day$, plan5h, plan7d), stepping quality down. What this ADR
adds is a *pool* move at **equal** posture and tier. Different axis,
different trigger, different failure (a drained weekly bucket).

## Decision

**1. The step-down exists, and it is the plan guard's — not Dial E's.**
Config `plan_guard_overflow: <runtime>` (unset = skip, the prior
behaviour). When the guard trips, the pass **runs**; per bead, at launch:

```
resolved runtime not on the guarded meter               → launch as today, ungated
--runtime given / PID runtime: not the guarded runtime  → same (explicit wins, ADR 0002)
eligible (§2) and cap not reached (§3)                  → launch with runtime = overflow
otherwise                                               → the guard's skip line, per bead
```

The guard's meter belongs to one provider; a template-only
`runtimes/<name>.yaml` is treated as on that meter (unknown → gated).
Only sessions **this pass creates** are moved; a found session keeps its
runtime (Dial F makes that the common case). Dial E is untouched: it
still resolves the tier, and on an overflow launch its step-down applies
on the overflow runtime (harmless — `fast` is unmapped there and falls
back to `standard`).

**2. Eligibility — parity decides, and judged work never moves.** A bead
may overflow only if
  (a) `CheckParityIn(ag, overflow, ResolveCage(cage, ag), tier, dir)` has
      **zero** `Degraded` entries — not "equal to the guarded runtime's",
      *clean*: this is dispatch's own choice and dispatch never holds
      `--allow-degraded` (the same rule Dial E uses for `fast`); this is
      what excludes a caged PID from a target that cannot nest its cage,
      with no runtime special-casing; and
  (b) the resolved tier is not `strong` — the tier table maps `strong`
      to "runtime default" on overflow targets, which is not the model
      the tier meant; mirrors Dial E (b). The judged lanes (architect,
      security, product) stay on the guarded runtime.
  (c) the PID has not said `overflow: false` — the opt-out for lanes
      that drive through repo shell scripts a target's unattended
      approval mode is known to stall (a parity check cannot see that).
      Default: eligible.
Static routing (`runtime: <x>` on a PID) is the operator's decision and
is not touched by this: routing a lane to the second pool permanently is
that, zero code.

**3. The cap stands in for the meter.** `plan_guard_overflow_cap: N` —
max beads sent to the overflow runtime in any **rolling 7 days**. It is
**required**: overflow set without a cap = overflow off, one stderr line
per pass. Why 7-day rolling and beads, not dollars: the overflow pool
has no meter the harness can read and `posse cost` cannot see its spend; a
rolling window upper-bounds any calendar week without knowing the pool's
reset day; and a weekly pool with no intra-week reset is exactly the
shape a per-pass trigger over-drains. Ledger: append-only
`$StateDir/overflow.log` (`RFC3339 runtime bead persona`), read once per
pass. Cap reached → the skip line names it (`plan 5h at 78% > 70%,
overflow <runtime> 20/20 in 7d — skipped`). Starting value: single
digits for a calibration week — read the pool provider's own usage
display before and after, and raise only on that evidence.

**4. Pool exhausted underneath us: skip, as today — plus the cap.** There
is no reading to take; a launch that the runtime rate-limits shows up as
the existing "held by persona, idle" line. That is the price of
open-loop, stated: a provider-side usage endpoint, if one appears, is
the loop closer.

**5. A blind guard skips; it never overflows.** *(Amendment,
2026-08-20.)* The guard has a second way to stop a pass: `--watch` plus
no successful meter reading for `plan_guard_blind_max:` (10m default).
That skip is **not** an over-threshold trip, so §1's per-bead ladder
does not run and nothing is moved to the overflow runtime, cap or no
cap. The reason is the same one that makes §1 a *per-bead* decision at
all: every rung above is a judgement made **on a reading** — which
window, how far over, which beads are on that meter. Blind, there is no
reading to judge on, so "the guarded window is hot, use the other pool"
is a guess, and a guess that spends a weekly bucket with no intra-week
reset (§3) is exactly the failure this ADR was written to avoid. The
blind skip is a *park*: nothing claimed, nothing spent on any pool, and
the first good reading resumes normal service including overflow.
Ledger consequence: a blind pass writes nothing to `overflow.log`.

## Consequences

- `dispatch.go`: runtime becomes per-launch (`fire`/`launchSession`/
  `promptContext`/`tierRefusal` take it); `planGuard` returns the
  reading, the per-bead decision moves into the loop; `--dry-run` shows
  `[<runtime> ← overflow]`.
- Config: `plan_guard_overflow:`, `plan_guard_overflow_cap:`; PID
  `overflow: false`. Dispatch docs, `examples/config.yaml`.
- Fixes in passing: launches on runtimes off the guarded meter are no
  longer skipped by that meter.
- Not enabled by anything here: no value set until the operator has
  sized the overflow pool and confirmed that any pay-per-use billing on
  it is off — a cap in beads is a brake, not a bill guard.
- Metric: overflow launches / closes / reopens by runtime — the same
  judge as Dial E, on the ledger.

## Alternatives rejected

- **A rung in Dial E's ladder** (`80% standard→fast`, `90% → <overflow
  runtime>`). The ladder is quality-down inside one runtime, driven by
  dollar caps too; moving pools does not reduce API-equiv spend, so a $
  cap must never fire it, and a pool move at equal posture is not a
  degradation. Two axes on one ladder is the clever one — I wanted it.
  It would have put a weekly bucket behind a 5-hour trigger.
- **`plan_guard_action: skip|<runtime>`, whole pass, no cap.** The
  literal ask. On a weekly pool it drains a week in an afternoon and
  takes the operator's own interactive share of that pool with it; on a
  pool that heals every few hours it is fine only if the pool is large,
  which — with no readable meter — nobody knows. The cap is the whole
  difference and it costs one key.
- **Per-pool budget model** (each runtime: window shape, meter, cap;
  dispatch bin-packs). Beautiful, no meters to feed it, three keys of
  config nobody has numbers for. File it when a second meter exists.
- **Eligibility "equal to the guarded runtime's posture".** That
  runtime's own launch can be degraded; "clean on the target" is
  stricter, simpler, and already the rule for `fast`.
- **Two overflow targets** (one per lane class). Static `runtime: <x>`
  on the PIDs that need the second target *is* that half; overflow then
  has one target, sized before it gets a cap.
