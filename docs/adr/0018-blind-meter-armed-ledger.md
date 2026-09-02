# ADR 0018 — Blind meter, armed ledger: the last brake parks, a backed brake degrades

*Status: accepted 2026-08-26 · owner: architect · amends ADR 0013 §3
(blind row) and ADR 0003 §4 (Dial E gains a duty) · ranger-base-kld4 ·
scope note 2026-08-29 (ranger-base-qs0z): this ADR's blind state and
clock belong to REMOTE meters, whose no-reading may be transient; a
meter over local files is armed or off-loud, never blind — ADR 0010 §6 ·
amended 2026-09-01 (ranger-base-bp224, from ranger-base-c3vqe): §1's
degrade runs from HEADROOM, not from a cap — a last reading over a
threshold or in the braking band parks with the caps armed; the
exposure line in Consequences was the sentence the 2026-08-31 incident
cashed in, and is rewritten · amended 2026-09-02 (ranger-base-jwcxu,
folding ranger-base-ch6re): §1's rendered brake line carries the same
placeholders NOTES.md's copy of it does, and Consequences says the
blind-day bound is not the pair first written, citing where the pair in
force is recorded (ranger-base-vi67) rather than quoting it*

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

**2026-08-31 measured what the floor is made of** *(amended,
ranger-base-c3vqe)*. The meter credential went stale at 23:09 and every
read after it was a 401 or a 429: nineteen hours blind. Dial E was armed
and counted correctly the whole way, so every unattended pass took §1's
degraded arm and kept hiring. The ledger counts dollars; the thing
running out was the account's weekly window, which the ledger has never
been able to see and does not know the ceiling of. The last reading the
meter managed said 89% of that window — already in Dial E's braking band
— and the fleet's own snapshot said 89% for nineteen hours while the
account climbed to 96%. The operator caught it by hand with 4% left.
This ADR's first premise held ("Dial E works exactly when the plan guard
cannot"); its conclusion did not: a brake that measures a different
quantity is not a floor under the one at risk. ADR 0011's diagnosis, one
store further out — one store's momentary reading taken as evidence
about another store's durable fact.

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
  (pass $X.XX/$Y.YY, day $X.XX/$Y.YY)`. The cockpit header must render
  degraded distinguishably from parked (today's `guard blind 14m`
  gains the ledger clause).
- Unchanged everywhere: attended passes (fail-open, stderr witness),
  blindness within `blind_max` (quiet tolerance), off-meter beads
  (launch through anything, ADR 0013 §3), `blind_max: 0` (never park —
  still the escape hatch for the unarmed case, and only that case).

**…and the degrade runs from headroom, not from a cap** *(amended
2026-09-01, ranger-base-bp224, from ranger-base-c3vqe; code
`internal/posse/blindheadroom.go`, one call site in dispatch.go
`blindFork`)*. Armed is necessary for the degrade and is no longer
sufficient. The licence is asked of the meter that went blind, from the
last successful reading on this machine (`PlanCache.LastReading`, the
same instance-wide snapshot G5's blind clock reads). Two refusals, both
numbers already in force on a sighted pass — no new knob, no new number:

- **Over a threshold**: the reading is strictly above one of the
  operator's own `plan_guard_<window>:` thresholds. A sighted pass would
  have skipped on it (`planGuard`, same comparison, same adapter order);
  going blind is not a promotion from skipped to running.
- **In the braking band**: the reading is at or past `BudgetStepDownPct`
  (80%), the rung at which the plan windows — which join Dial E's
  tightest-window comparison whenever the guard read them — had the
  ledger already braking. Blind, those windows drop out of `resolve()`
  silently and a pass that was braking becomes one that is not. Sighted
  at 89% the shop *steps down* and watches for 100%; blind it cannot see
  100% arrive, so the brake's first rung becomes its last: **park**.
  Stricter than sighted in the band, deliberately.

Thresholds are asked before the rung (the operator's own line is the
stronger statement, and the refusal names the config key they would
edit); both walk the adapter's order so the window named is the one
whose exhaustion hurts most; the refusal is asked *before* the ledger
scan, so a park costs no transcript walk. Under either refusal the
on-meter beads park with the caps armed and counting, the park line
names the window, its percentage, the reading's age and the sentence
"a dollar cap is not a brake on the plan window", and the cockpit
header gains a fourth blind clause — `no headroom at last reading,
parked` — on the ranger-base-3nvt rule that a header must not name a
brake that is not holding (it said "ledger brake" for nineteen hours).

What the reading is NOT: a number about now. It is never aged forward,
scaled by spend, or extrapolated — that is "estimate the plan window
while blind", rejected below and still rejected. It is asked one
question about the past that the past can answer: *when the lights went
out, was there room?* A 429 moves only the cooldown and keeps the
reading; a snapshot holding only a cooldown is not a reading.

Unchanged: a reading with room degrades exactly as above; **no reading
ever taken on this machine degrades too** — that is the 2026-08-26
shape (a credential posse could not read from the first pass) and
parking it cost a measured hour of zero dispatch. The fork is between
evidence of a ceiling and no evidence, not between known-safe and
unknown: park on evidence, never on ignorance. `blind_max: 0`, attended
passes, off-meter beads, the first good reading clearing everything —
all untouched.

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
- *(amended 2026-09-01)* …only while its last reading left room. With
  the caps set, a last reading over a threshold or in the braking band
  halts the shop anyway; that halt is the point, and the header says
  which halt it is.
- *(amended 2026-09-02, ranger-base-ch6re, from ranger-base-vi67)* …and
  the pair the first bullet names is the pair armed on 2026-08-26, not
  the pair in force. On ranger-base-vi67 the operator re-affirmed the
  caps this deployment carries rather than changing them to match the
  pair above, on the ground that the caps are anomaly stops and the
  brake on a blind day is §1's headroom refusal (amended into §1 above,
  ranger-base-bp224), not the cap. So read the first bullet for the
  SHAPE of the bound — two dollar ceilings, one over the pass and one
  over the day, and never a clock — and read ranger-base-vi67 for the
  pair that bounds it here. The figures themselves are a live guard
  value and stay in the instance record: ADR 0024 D1 rules that class
  instance content, and D3's restate-and-cite is what this line is. The
  bless on ranger-base-axft licensed the pair the first bullet names,
  not whichever pair is live, and this ADR does not read the config —
  so citing beats quoting here twice over, once for visibility and once
  because a quoted pair goes stale the day the operator moves it.
- *(rewritten 2026-09-01 — the sentence below as first written was the
  one the 2026-08-31 incident cashed in.)* A blind window risks the plan
  windows only from a reading that showed room, and there the exposure
  is capped in dollars — coarser than window-percent, a coarse honest
  brake over a precise invented one, still the trade. From a reading in
  the braking band the exposure is zero dispatch. The residue is real
  and named: a reading at 79% followed by a long blind window runs on
  dollars all the way, because nothing here knows the dollar-to-percent
  ratio and nothing will pretend to. That residue is bounded per day by
  `budget_day:` and is cured only by a reading — the credential that
  rots in hours is ranger-base-wkai3 / ADR 0019, and this rule makes the
  fleet safe while blind, not less blind.
- `plan_guard_blind_max:` recovers a single meaning: how long quiet
  tolerance lasts before the policy fork (park or declared-degraded).
  The 24h workaround should be deleted when this lands (its own
  comment already says so); the default returns. Operator's move, not
  the crew's.
- Arming/disarming Dial E now also chooses the blind policy — where the
  meter's last reading left room. That coupling is the point, and it is
  stated where the caps are documented (examples/config.yaml, which since
  2026-09-01 also says what the caps do not buy; INSTALL.md and NOTES.md
  carry the same fork).
- *(amended 2026-09-01)* On re-arm after a blind stretch that ended in
  the braking band, the unattended shop parks until the meter reads
  live — no hiring on a frozen 97%. That is the catch that was missing.
  The escape hatch if the operator must move anyway is `blind_max: 0`,
  unchanged, and it is a decision they make, not one the caps make for
  them.

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
  gate. *(2026-09-01: still rejected, and the headroom rule above is
  not it — it reads the snapshot as a fact about the past and computes
  nothing about now. The next reader should not mistake one for the
  other.)*
- **Park whenever blind and armed** (the incident's own ask: AND the
  two conditions instead of OR-ing them). Rejected 2026-09-01. It
  parks on ignorance: a fresh machine, a wiped state dir, or a
  credential unreadable from the first pass has no evidence of a
  ceiling, and that shape already cost a measured hour of zero dispatch
  on 2026-08-26. It also makes arming Dial E irrelevant to the blind
  policy, which reverts the coupling this ADR exists to make law.
- **Expire the refusal at the window's own length** (a 5h reading is
  surely healed after five hours; a 7d reading after a week). Rejected
  2026-09-01. It is the first step of aging the snapshot: "healed"
  assumes nothing else spent into the window meanwhile (the operator's
  interactive use shares the account), and some providers' week has no
  intra-week reset at all. The cost is named and paid: a reading in the
  5h window's braking band parks the on-meter lanes for the whole blind
  stretch, however long. The cure is a reading, the hatch is
  `blind_max: 0`.
- **A maximum age on the reading** (refuse the degrade once the
  snapshot is older than N): wall-clock in a new coat, and this ADR's
  own rule is that the degrade is never bounded by the clock. Age is
  said out loud on the park line; it decides nothing.
- **Teach the ledger the plan ceiling** (the incident's other ask:
  convert `budget_day:` into window-percent). It needs the
  dollar-to-percent ratio, which is the estimate alternative wearing
  the ledger's clothes; rejected for the same reasons.

## Claims

MEASURED: the outage timeline and zero-dispatch hour (okbr, plan-usage
.log); caps set from this instance's spend (config.yaml comments);
`blind_max` changed three times in two days (same); the skip condition
and the `segs, _ :=` swallow (dispatch.go:276, cost.go:303). ASSUMED:
local-FS transcript reads fail rarely (why §3 is a hardening, not a
redesign); $30/$250 acceptable ceilings for a blind day — the
operator's numbers, theirs to change.

*Amendment claims (2026-09-01).* MEASURED: the 2026-08-31 timeline —
last successful reading 23:09 at 89% of the weekly window, nineteen
hours of 401/429 with the snapshot's `At` frozen at 23:09, the account
at 96% when caught by hand (ranger-base-c3vqe, plan-usage.log); the
2026-08-26 hour of zero dispatch that the no-reading arm protects; the
rule's two boundaries pinned to the code that owns each (strictly-above
matches `planGuard`, at-or-above matches `BudgetState.StepDown`); the
incident replayed end to end under the fix parks (commit a98ed0e's
tests, mutation-checked 8/8); a 429 keeps the last reading and a
cooldown-only snapshot is not a reading (`plancache.go`, pinned).
ASSUMED: that a reading under the rung followed by a long blind window
cannot exhaust the weekly ceiling inside one `budget_day:` — unknowable
without the ratio this ADR refuses to estimate, so it is stated as the
residue, not as a bound; that a wiped `plan-usage.json` (which demotes a
braking reading to no-reading) is the operator's own act on their own
state dir and as rare as a fresh install.

*Amendment claims (2026-09-02, ranger-base-jwcxu, folding
ranger-base-ch6re).* MEASURED: §1's rendered brake line quoted this
instance's own pass and day spend — the two halves before the slashes —
and the shipped ops-pattern cost class saw them, which is why the same
line in NOTES.md has always rendered placeholders instead; both copies
of that render now agree, pinned at the repo root by
`adr0018scrub_qa_test.go`. RULED, not measured: that the bound
Consequences first stated is not the bound in force. That is the
operator's re-affirmation on ranger-base-vi67, and the pair itself is
NOT restated here — a live cap is instance content under ADR 0024 D1,
the bless on ranger-base-axft covered the pair the first bullet names
and not whichever pair is live, and this ADR cannot read the config, so
a quoted pair would be both a disclosure and a staleness bug. D3's
restate-and-cite is the shape (asked as ranger-base-1gak4, taken at its
stated default rather than waiting on it, because not publishing is the
reversible direction). LEFT UNDONE, deliberately: the ASSUMED line
above still quotes the 2026-08-26 pair as the acceptable ceiling for a
blind day. ranger-base-ch6re names the Consequences section only, so
restating it there too is the operator's call and not this amendment's
licence.
