# ADR 0018 — Blind meter, armed ledger: the last brake parks, a backed brake degrades

*Status: accepted 2026-08-26 · owner: architect · amends ADR 0013 §3
(blind row) and ADR 0003 §4 (Dial E gains a duty) · ranger-base-kld4*

## Context

2026-08-26: the plan guard could not read the keychain credential (the
item held a shape posse did not know — ranger-base-okbr), went blind,
and past `plan_guard_blind_max:` (10m) parked every on-meter bead. All
17 PIDs are `runtime: claude`, so the shop dispatched zero beads for an
hour. The ask from okbr: should an unreadable meter be BLIND (park) or
DEGRADED (run, loudly)?

The premise of that ask was the actual bug. On the day of the outage
`budget_pass:`/`budget_day:` (ADR 0003 Dial E) were **unset**, so the
plan guard was the only armed brake — config.yaml said so in as many
words — and "degrade" would have meant *no brake at all*, unmetered
until a human read a log. Blind is blind: an unreadable meter cannot
tell 0% used from 98% used, and a credential-shape change is silent and
permanent, not transient like a 429.

What changed the same day: the operator armed the ledger caps
(`budget_pass: 30`, `budget_day: 250`, from measured spend). Dial E
computes from posse's own transcripts and needs no credential — it
works exactly when the plan guard cannot. And as a stopgap,
`plan_guard_blind_max:` was raised to 24h, a hand-tuned coupling whose
own comment calls it a smell to delete. That knob had by then been
changed three times in two days, each time on a wrong diagnosis. The
coupling is real; it should be law, not a comment.

## Decision

**1. Unattended blindness past `plan_guard_blind_max:` parks on-meter
beads only when Dial E is unarmed.** The skip condition
(dispatch.go `blindGuard`) gains one clause: `… && !ledgerArmed`, where
ledger-armed is `BudgetState.Set()` (either cap configured). The last
armed brake still fails closed, unchanged. A brake with a floor under
it degrades instead:

- **Degraded pass**: on-meter beads launch under the ledger brake.
  Dial E's rungs apply as always — step-down at 80%, stop at 100% of
  the tightest window. The degrade is bounded by the ledger, **never by
  wall-clock**: run while something is still counting, never because
  the clock ran out.
- **Loud**: one line per pass in the pass output (`d.Out`, not stderr —
  a degraded pass is never quiet, extending rangerhq-llse) naming the
  blind duration, the read error, and the ledger state, e.g.
  `plan guard: blind 4h (…) — degraded, running under ledger brake
  (pass $8.20/$30, day $146/$250)`. The cockpit header must render
  degraded distinguishably from parked (today's `guard blind 14m`
  gains the ledger clause).
- Unchanged everywhere: attended passes (fail-open, stderr witness),
  blindness within `blind_max` (quiet tolerance), off-meter beads
  (launch through anything, ADR 0013 §3), `blind_max: 0` (never park —
  still the escape hatch for the unarmed case, and only that case).

**2. No policy fork by failure class.** A shape mismatch, a gate
refusal, a 401, a network error — for gating they are one state:
no reading. The classes exist for the *diagnostic* (okbr fixed that;
the next shape change names the keys it found in the first line) and
for cooldown (`RateLimit`), never for park-vs-degrade. Policy that
reads diagnosis strings rots when the diagnosis improves.

**3. The degraded brake must itself be honest.** `ScanCosts` swallows
read errors (`segs, _ := ScanTranscript(…)`) — an unreadable transcript
root today reads as $0 spent, i.e. an armed brake that counts nothing.
Precedent already in tree: "an unreadable ledger is not a licence to
spend" (overflow ledger, dispatch.go `overThreshold`). Same rule here:
the cost scan learns to distinguish *no records* from *cannot read*,
and in a degraded pass a cannot-read parks on-meter beads exactly as
an unarmed Dial E would. Outside degraded mode a cannot-read with caps
armed is named on stderr once per pass.

**4. No third on-meter state** (the ask's part c). "On-meter but
unmeterable" is a condition of the *pass*, not a property of the bead;
ADR 0013 §3's two-way split stands. The separate defect — the meter
gating passes whose runtime does not spend it — is already fixed at the
bead grain by 0013 §3 and its residue stays on its own beads
(ranger-base-3j8 neighbours), untouched here.

## Consequences

- The shop can no longer be halted by the optional meter alone while
  the ledger caps are set — the 2026-08-26 outage shape becomes a
  degraded-loud day bounded at $30/pass, $250/day.
- A blind week no longer risks the plan windows silently: the exposure
  is capped in dollars, which is coarser than window-percent. That
  trade is deliberate — a coarse honest brake over a precise invented
  one.
- `plan_guard_blind_max:` recovers a single meaning: how long quiet
  tolerance lasts before the policy fork (park or declared-degraded).
  The 24h workaround should be deleted when this lands (its own
  comment already says so); the default returns. Operator's move, not
  the crew's.
- Arming/disarming Dial E now also chooses the blind policy. That
  coupling is the point, and it is stated where the caps are documented
  (examples/config.yaml).

## Alternatives rejected

- **Degrade unconditionally** (the ask's plain reading): with caps
  unset that is an unmetered fleet until a human reads a log — a worse
  outage than a stopped shop, landing on the operator's interactive
  headroom, the thing the guard exists to protect.
- **Degrade transient classes, park shape mismatches**: blind is blind
  in every class; class-forked policy couples gating to diagnostic
  strings and pays twice for a diagnostic already fixed.
- **Wall-clock-bounded degrade** (the 24h stopgap as law): the clock
  knows nothing about spend; it is exactly "run because the clock ran
  out". Kept only as operator state, to be reverted.
- **Keep the coupling manual** (operator tunes `blind_max` when caps
  change): measured to fail — three changes in two days, coupling in a
  comment nobody re-reads under incident pressure.
- **Estimate the plan window while blind** (the clever one): fit a
  $/window-percent ratio from history and let the guard gate on the
  extrapolation. Rejected: the ratio is unmeasured, drifts with model
  mix and cache behaviour, and a wrong estimate wears the authority of
  the real meter. If it is ever wanted it is a display hint, never a
  gate.

## Claims

MEASURED: the outage timeline and zero-dispatch hour (okbr, plan-usage
.log); caps set from this instance's spend (config.yaml comments);
`blind_max` changed three times in two days (same); the skip condition
and the `segs, _ :=` swallow (dispatch.go:276, cost.go:303). ASSUMED:
local-FS transcript reads fail rarely (why §3 is a hardening, not a
redesign); $30/$250 acceptable ceilings for a blind day — the
operator's numbers, theirs to change.
