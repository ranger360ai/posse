# ADR 0010 — Park guarded work; keep off-meter work eligible

*Status: accepted; simplified 2026-09-05 by operator ruling · automatic-overflow removal pending deferred implementation · owner: architect.*

## Context

Doing nothing carries an automatic provider move, two overflow keys and a
rolling launch ledger. The review found neither key configured locally;
that is local non-use, not a census of supported instances. Prefer explicit
runtime choice and parking the work whose meter has tripped. Preserve the
independent spending ledger and the blind/headroom protection folded from 0018.

## Decision

**1. Remove automatic overflow.** A sighted threshold trip parks on-meter
beads; eligible off-meter beads continue under their own brakes. Do not
change a bead's runtime to continue paid work. Explicit operator runtime
selection remains. Remove `plan_guard_overflow`, `plan_guard_overflow_cap`,
the PID `overflow` eligibility switch, provider-choice/eligibility branches,
and the overflow-only rolling ledger and display state. Preserve
`OnGuardedMeter` semantics when its helper moves out of overflow.go.

**2. Local meters and dollar budgets remain independent.** A local-file
meter is armed when its inputs parse, otherwise off and loud; it does not
borrow the remote blind clock. Unreadable data is named rather than treated
as zero. Target-pool calibration and static-runtime brakes remain; removing
overflow does not remove local metering, uncounted caps, `budget_pass`,
`budget_day` or their transcript accounting. 0003 owns spending dials;
0013 §5 owns priced versus uncounted work. 0034 is telemetry only.

**3. The pass remains bounded.** 0011 owns rolling reconciliation and
watch backoff. A fully skipped pass can back off; no whole-pass guard
blocks unrelated work. No timeout or missed reading unclaims a bead.

**5. Remote guard: complete per-bead decision table** (folded from 0018).
The pass runs; only beads spending the guarded meter take its verdict.
Runtime meter membership is `OnGuardedMeter` (0013 §3); unknown membership
is treated as on-meter. Rows are evaluated in order. Other launch brakes
always remain in force.

| Condition | Guard outcome |
|---|---|
| Bead off the guarded meter, or guard disabled | No refusal from this guard |
| Valid reading over an operator threshold | Park on-meter work; do not substitute a provider |
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
The local-file meter shape is §2, never the remote blind clock.


## Consequences and alternatives

ASSUMED removal price: 4–7 source files plus tests/docs; two config keys,
one PID opt-out, overflow history and provider-choice state removed; no new
actor or flag. The **overflow** ledger is expendable historical telemetry;
Dial E's spending ledger remains the independent brake in §5. First
done-when on the deferred removal: census supported instances for configured
overflow and actual overflow launches, stating coverage and observation
window. Do not call one instance's absence a product-wide measurement.
If wrong, automatic paid continuity is lost for an instance that relies on it.

Rejected: doing nothing (unpriced automatic continuity); another per-pool
registry or no-cap target (more state or uncontrolled drain); deleting the
blind protection together with overflow (different ledger); timing out
headroom or estimating plan percentage from dollars (false authority).
The accepted smaller behavior is pending code; current overflow continues
until its deferred task lands. Existing local meter configuration is not
changed by this documentation execution.

## Lineage

| Was | Here |
|---|---|
| 0010 §§1–4 overflow move and target ledger | §1 removes the mechanism; independent meters retained in §2 |
| 0018 §§1–4 and headroom amendment; old 0013 §3 | §5 ordered guard table; 0013 points here |
| 0010 §6 local meter shape | §2 |

Historical overflow design and guard evidence: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0010-plan-guard-overflow.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
remain dated history.
