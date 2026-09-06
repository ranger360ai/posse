# ADR 0010 — Park guarded work; keep off-meter work eligible

*Status: accepted; simplified 2026-09-05 by operator ruling · §1's removal
executed 2026-09-06 in ranger-base-6xx37, built in that bead's seat tree and
not on main at this stamp — `git log --grep ranger-base-6xx37` on main is the
record of whether it landed, this sentence is a dated snapshot (ADR 0038
shape, ranger-base-w5xu7) · owner: architect.*

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

MEASURED removal price (ranger-base-6xx37): 33 files, +890 / −2552. Ten
source files plus the CLI help; two config keys, one PID opt-out and
`$StateDir/overflow.log` removed; no new actor or flag. Against the ASSUMED
4–7 source files, so the ASSUMPTION was low — the mechanism reached further
into dispatch and the record than the estimate allowed for, and the extra
files are one-line citation and comment repairs rather than logic. Two
things were RETAINED rather than deleted, both because a surviving brake
consumes them: `OnGuardedMeter` (the §5 table's membership test, ADR 0013
§3) moves to `planusage.go`, and the ledger shape and its three helpers move
to `ledger.go`, where `uncounted.log` — ADR 0013 §5's independent brake — is
their one remaining reader. `PoolMeterArming` is deleted: it existed to
answer this ADR's pre-simplification §3 arming question and had no other
caller. Old `overflow.log` files are left in place, unread and unwritten.

MEASURED census for the first done-when, 2026-09-06, ONE instance
(`~/.config/posse`) plus every other `config.yaml` reachable on that box (7
files). Configured overflow: ZERO — no `plan_guard_overflow:` or
`plan_guard_overflow_cap:` is set anywhere, and the shipped seed has always
carried both commented out. Actual overflow launches: ZERO — no
`overflow.log` exists anywhere on the box, and the first move would have
created one; across 154,329 lines of dispatch output in six logs (2026-08-25
→ 2026-09-06) the string "overflow" appears zero times, marker included.
COVERAGE, stated because the absence of a key is not the absence of a user:
the 13 threshold trips in that record are all from 2026-08-25 and all print
the pre-ladder whole-pass skip, so they are not observations of an armed
overflow decision; in the window where the ladder existed (the two --watch
logs, 2026-08-30 15:16 → 2026-09-06 02:11) the guard never tripped a
threshold at all — 94 blind lines, 9 rate-limit, 8 reading-restored, 0 trips
— so the ladder had no opportunity to run and this instance's zero is
consistent with both "nobody uses it" and "nobody could have". posse has
been published since 2026-08-23 with both keys documented in
`examples/config.yaml` and sends no telemetry, so how many other instances
set them is UNKNOWN and unmeasurable from here. This is one instance's
non-use, not a product-wide obsolescence finding, and the operator ruling to
remove was taken knowing that.

The **overflow** ledger is expendable historical telemetry; Dial E's
spending ledger remains the independent brake in §5.
If wrong, automatic paid continuity is lost for an instance that relies on
it: that instance's on-meter beads park on a trip instead of moving, and the
fix is an explicit `runtime:` on the PID or `--runtime` on the pass. A
config that still sets either removed key gets one stderr line per pass
naming it, rather than having it read as a threshold for a window named
"overflow".

Rejected: doing nothing (unpriced automatic continuity); another per-pool
registry or no-cap target (more state or uncontrolled drain); deleting the
blind protection together with overflow (different ledger); timing out
headroom or estimating plan percentage from dollars (false authority).
Existing local meter configuration is not changed by this removal, and
neither are Dial E's caps, the uncounted ledger and its writability rules,
tier safety or rolling dispatch: §5's table, §2's meters and §3's bounded
pass are byte-for-byte the behaviour that shipped before it.

## Lineage

| Was | Here |
|---|---|
| 0010 §§1–4 overflow move and target ledger | §1 removes the mechanism; independent meters retained in §2 |
| 0018 §§1–4 and headroom amendment; old 0013 §3 | §5 ordered guard table; 0013 points here |
| 0010 §6 local meter shape | §2 |

Code citing a section number this page no longer has was repointed by that
same removal: old §6 (local meter armed-or-off) is §2, old §3's `uncounted_cap_`
consequence is ADR 0013 §5's, and the dead-key rule for a cap on a runtime
whose dollars ARE priced (ranger-base-2eeb) was ratified as old §3's third
amendment and now cites ADR 0013 §5, which owns the key and carries the rule's
own text since ranger-base-ubqcw — filed from this bead's execution and landed
before it.

Historical overflow design and guard evidence: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0010-plan-guard-overflow.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record). The removed mechanism's own code is `git show 495d2a6:internal/posse/overflow.go`.
