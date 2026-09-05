# ADR 0010 — Plan-guard overflow: a second pool when the metered window is hot

*Status: accepted 2026-08-18 · owner: architect · amended 2026-08-24
(ADR 0013: the skip is per-bead, including when blind) · amended
2026-08-29 (ranger-base-qs0z: a local meter arrived — §3 arming, §4 loop
closer, §6 local-meter shape) · amended 2026-09-05 (0018 folded; §5 is the complete guard table)*

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

*(Amended 2026-08-29, ranger-base-qs0z.)* The premise half-expired: xAI
still publishes no endpoint, but grok writes its own per-turn cost to
disk, and rangerhq-myso turned that into a **local pool meter**
(`grokpool.go`): `grok_guard_week:` (percent) / `grok_pool_reset:` /
`grok_pool_usd_per_point:`, all three required together, utilisation% =
USD since the last weekly reset ÷ USD per point — an estimate and a
floor, per-bead beside the account stage. Three rules follow:

- **The requirement becomes "at least one armed brake on the target
  pool."** Overflow arms when `plan_guard_overflow_cap:` is set (as
  before) **or** when the target's own pool meter is fully armed (today
  only grok has one; the check keys on the meter's arming, not a
  reading). Both set = both apply. Neither = overflow off, one stderr
  line naming both ways to arm it. Why the meter alone suffices for
  overflow specifically: every overflow launch spends the pool from this
  box, so the meter sees **all** of the drain overflow itself causes —
  the floor's blind spot (the operator's phone/web share) is other
  people's spend, priced into the threshold. The residual risk is factor
  drift (a reprice staling the estimate with nothing failing);
  mitigations are the factor logged on every reading, and the cap
  remaining available as a belt.

  *(Built 2026-09-02, ranger-base-gxgc.)* `PlanGuardOverflow` asks the
  target pool for its arming (`PoolMeterArming`, keyed on
  `GrokPoolRuntime`) and `Overflow.On` is true on either brake;
  `Overflow.Capped` is the cap's own half, so the cap fires only where it
  was set. The arming test never takes a reading — it is three config
  keys, and the reading stays where §2's placement above puts it. Two
  consequences the sentences above did not spell out, decided here and
  recorded rather than left to the code. A cap that is SET and unusable
  is still named on stderr when the meter arms the move: the typo is a
  brake the operator believes in and does not have, and it stays visible
  whatever else is holding the pool. And a ledger fault — unreadable or
  unappendable `overflow.log`, the ranger-base-2y96 / ranger-base-af98
  refusals — now takes the CAP rather than the move, where the meter is
  armed: the fault is in the cap's instrument, the meter is a config key
  and a transcript scan that no unreadable file touches, and the ledger
  is no longer the only record of what the pool was spent on. With no
  meter armed it is the same whole refusal as before, unchanged. Pins:
  `internal/posse/overflowarming_test.go` (eight mutants run, listed in
  the file header).
- **Two brakes on one pool both fire; deferral is a config edit, not a
  mechanism.** The shipped ordering is ratified: where a reading exists
  it is named first — the bead cap is the stand-in for the *absence* of
  one — and the cap still fires second. No auto-defer and no both-set
  warning: an operator who set both meant both, and the brakes fail
  differently (a bead count needs no calibration; a percentage needs the
  factor).

  *(Amended 2026-09-02, ranger-base-dmzao / ranger-base-v62hj.)* The
  sentence above was ratified against the wrong pair. It was written
  over `grokPoolSkip`-before-`uncountedSkip` in the launch loop, which
  ADR 0013 §5 had already made unable to both fire, so the ratification
  measured nothing. The pair this bullet arms — the target's meter and
  the bead cap — ran the other way round: measured at be5077c, the cap
  was checked in `overflowFor` at the overflow **move** and the meter
  only afterwards in the launch loop, so a bead with both brakes tripped
  parked on the cap's bead count and the pool was never read. No spend
  escaped; the reading did. Ruled a code defect rather than a record
  defect, for three reasons — the code's own comment and this bullet
  already agreed on the intent and differed only in placement; §3 tells
  the operator to raise the cap "only on that evidence", the pool's own
  usage, and the shipped ordering hid that reading on exactly the line
  that prompts the calibration; and the meter is an *arming* brake for
  the move (bullet 1), so it belongs in the move's ladder rather than
  after it. The ordering is now realised at the move: `overflowFor`
  consults the target pool's meter after §2's eligibility and before the
  cap. A pool over threshold names its reading and the cap never speaks
  on the bead's line — the pass's trip header still announces the cap's
  count once, as the arming notice it always was — and a reached cap
  fires only once the reading has been taken and reported.
  Placement after §2 is load-bearing — a bead §2 refused is no candidate
  for the pool, and takes no reading. Pins:
  `TestQABothGrokBrakesOnOnePoolReadTheMeterThenTheCap` and
  `TestQAOverflowMeterIsReadOnlyForABeadSection2WouldMove`
  (`internal/posse/verifyesa0j_qa_test.go`).
- **The bead cap's lifetime is the account-degraded column's** (ADR
  0013 §5). That day came (ranger-base-0lg6, ratified ranger-base-mykq):
  the column's predicate is now `Runtime.CostPriced()`, grok's adapter
  carries provider-reported dollars, and `uncounted_cap_grok:` is dead
  by §5's existing law — `uncountedFor` returns nil for a priced runtime
  before it ever reads the key. codex's cap SURVIVES its adapter,
  because that adapter prices nothing it reads (read-but-unpriced keeps
  the column). The dead key must still die **loudly**: a set
  `uncounted_cap_<runtime>:` on a priced runtime is named once per pass
  as not applying, pointing at the brake that does — built 2026-09-01
  (ranger-base-2eeb, `uncounted.go` countedCapDead; cut twice, the other
  id is ranger-base-ql08): one stderr line per pass naming the key and
  the brake that holds the runtime instead (`grok_guard_week:` where
  armed, else `budget_pass:`/`budget_day:` over the dollars `posse cost`
  can now see). A silently dead key is the cap-that-stopped-capping
  failure `uncounted.go` is written against.

**4. Pool exhausted underneath us: skip, as today — plus the cap.** There
is no reading to take; a launch that the runtime rate-limits shows up as
the existing "held by persona, idle" line. That is the price of
open-loop, stated: a provider-side usage endpoint, if one appears, is
the loop closer.

*(Amended 2026-08-29.)* The loop closed — from the consumer side. No
endpoint appeared; the pool's own client writes its cost to disk and
rangerhq-myso reads it. "There is no reading to take" is no longer true
of grok: exhaustion-underneath-us now shows as an estimate over the
threshold and skips *before* the launch, not as an idle pane after it.
The open-loop price is repealed only where a meter exists, and only to
that meter's precision — the reading is an estimate and a floor, never
the vendor's number.

**5. Remote guard: complete per-bead decision table** (folded from 0018).
The pass runs; only beads spending the guarded meter take its verdict.
Runtime meter membership is the declaration in 0013; unknown membership
is treated as on-meter. Rows are evaluated in order. Other launch brakes
always remain in force.

| Condition | Guard outcome |
|---|---|
| Bead off the guarded meter, or guard disabled | No refusal from this guard |
| Valid reading over an operator threshold | Threshold trip; currently §§1–3 overflow/park, pending the approved overflow removal |
| Valid reading at or below thresholds | Continue, subject to Dial E and other brakes |
| No reading; attended, or `blind_max: 0`, or not past `plan_guard_blind_max` | Continue with the existing diagnostic/tolerance; never infer overflow from blindness |
| Unattended blindness past the limit, no `budget_pass` or `budget_day` cap armed | Park on-meter work |
| Same, caps armed, last successful reading strictly over an operator threshold | Park; name threshold, window, percentage and reading age |
| Same, caps armed, last reading at or above `BudgetStepDownPct` (80%) | Park; a spending cap cannot bound a plan window |
| Same, caps armed, last reading below both boundaries **or no successful reading ever**, ledger unreadable | Park; cannot-read is not an empty ledger |
| Same, caps armed, last reading below both boundaries **or no successful reading ever**, ledger readable | Degraded and loud on each pass; continue only within Dial E's step-down/stop brakes |

Arming is checked first, then the last reading, then the ledger scan.
Thresholds precede the braking-band test and use adapter window order.
The reading is evidence about the past: never extrapolate it, age it into
headroom, or erase a refusal on a timer. A 429 retains the reading; a
cooldown-only cache is no reading. A fresh successful reading restores the
sighted policy. No-reading-ever with a readable armed ledger deliberately
degrades: parking that shape caused the measured zero-dispatch outage.
Failure classes affect diagnostics/cooldown, never park-versus-degrade.
The degraded line reports blind duration, error and actual ledger state;
the parked line and cockpit name the brake that really holds.

MEASURED in 0018: a credential-shape outage caused an hour of zero dispatch
with no fallback brake; later, nineteen blind hours with a last reading
already at 89% let the weekly window reach 96% despite an armed spending
ledger (ranger-base-c3vqe). Therefore ledger arming alone is insufficient.
ASSUMED residue: a reading below the braking band can still exhaust a plan
window during a long blind period; dollars and window percentage are
different quantities. No new estimate or policy fork is introduced.
The local-file meter shape remains §6, never the remote blind clock.

## Lineage

| Was | Here |
|---|---|
| 0018 §§1–4, including the headroom amendment and ledger-read refusal | §5 complete table; dated incidents and alternatives retained in 0018 |
| 0013 §3 per-bead guard table | §5; 0013 retains only runtime membership and the pointer |

The fold removes zero runtime mechanisms. Retaining the old slogan would
allow a blind brake to run on a last reading that already said stop.
Rejected: unconditional degrade, diagnosis-string policy, time-expired
headroom, guessed dollar-to-window conversion, or parking no-reading-ever
despite the independently armed readable ledger.

**6. A local meter is armed or off; it is never blind.** *(Added
2026-08-29, ranger-base-qs0z — the shape rangerhq-myso set, written down
so the next local meter does not inherit ADR 0018's clock by default.)*
The blind state and its clock (`plan_guard_blind_max:`, ADR 0018) exist
because a **remote** meter's no-reading may be transient — a credential,
an endpoint — so waiting is a strategy. A meter over local files has no
transient outage to wait out: its inputs are config keys and a
directory. It has exactly two states: **armed** (every input present and
parsing) or **off, loud** — one stderr line per pass naming the missing
input. Never parked: parking on a condition no retry can change is a
brake with no release. Two corollaries. An off meter is not a reading
and satisfies nothing — it gates no bead and does not arm overflow under
§3. And an armed meter whose store is unreadable still fails toward
naming, never toward $0: unread transcripts are counted as unread on the
line, and the floor only ever under-reports, stated. ADR 0018 keeps the
remote shape; its scope note points here.

## Consequences

- `dispatch.go`: runtime becomes per-launch (`fire`/`launchSession`/
  `promptContext`/`tierRefusal` take it); `planGuard` returns the
  reading, the per-bead decision moves into the loop; `--dry-run` shows
  `[<runtime> ← overflow]`.
- Config: `plan_guard_overflow:`, `plan_guard_overflow_cap:`; PID
  `overflow: false`. Dispatch docs, `examples/config.yaml`.
- Fixes in passing: launches on runtimes off the guarded meter are no
  longer skipped by that meter. *(ADR 0013: this was true of a
  threshold trip only; the blind skip is now the same grain.)*
- Not enabled by anything here: no value set until the operator has
  sized the overflow pool and confirmed that any pay-per-use billing on
  it is off — a cap in beads is a brake, not a bill guard.
- Metric: overflow launches / closes / reopens by runtime — the same
  judge as Dial E, on the ledger.
- *(2026-08-29)* `overflow.go` arming learns the either-brake rule (§3);
  `uncounted.go` learns the dead-key line (§3, live before 0lg6 lands —
  the condition is `Counted()` plus a set key, testable on a fixture
  runtime). Both cut as beads off ranger-base-qs0z. `overflow.log` is
  still written on every overflow launch, cap or no cap: it feeds the
  metric, and the cap if one is later set. *(Built 2026-09-02: the
  arming rule ranger-base-gxgc, the dead-key line ranger-base-2eeb. The
  trip header names whichever brake armed the move, so a meter-armed
  pass reports no bead count and invents no cap of zero.)*
- *(2026-08-29)* The ledger's WRITABILITY is a precondition of the move,
  not a warning after it (ranger-base-2y96): a readable-but-unwritable
  `overflow.log` counted every pass at whatever it already said, so a cap
  admitted its number of launches per pass forever and recorded none. It
  now fails closed with the unreadable case. `docs/notes.d/ranger-base-2y96.md`.
- *(2026-08-29)* Tripwire: the arming check keys on the grok meter by
  name, deliberately not a registry. When a **second** local pool meter
  appears, the rejected per-pool budget model's "file it when a second
  meter exists" comes due for real.
- *(2026-08-29)* Claims: MEASURED — grok per-turn cost on disk and the
  decoder's 2× trap (ranger-base-k7nb, 171/171 records); the shipped
  guard and its mutation-checked pins (f746ba5); `uncountedFor` nils out
  on `Counted()` before reading the cap key (uncounted.go:112);
  overflow-spend-is-on-box is by construction (dispatch launches
  locally). ASSUMED — the conversion factor holds between calibrations
  (mitigated: logged every reading); the operator's off-box grok share
  is small enough to price into the threshold (the operator sizes it).

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

*(2026-08-29, with the meter in hand:)*

- **The cap auto-defers when the meter is armed.** Silently disarming a
  brake the operator set, on the strength of an estimate with two silent
  under-report modes (factor drift, the floor). Deferring is one config
  edit, made by the person who owns the numbers.
- **The cap stays required alongside the meter forever.** Makes §4's
  loop-closer promise empty, and forces the operator to keep inventing a
  bead number the meter obsoletes — "single digits for a calibration
  week" was a crutch for not knowing, and now we know.
- **A warning when both brakes are set.** Both-set is a valid
  belt-and-braces posture, not a smell; warning on it nags the cautious.
- **Generalise now to a pool-meter registry.** The per-pool budget
  model again, still rejected for the same reason at smaller scale: one
  meter exists, and a registry of one is a name with no second member.
