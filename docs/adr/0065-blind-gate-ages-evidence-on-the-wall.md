# ADR 0065 — The blind gate ages its evidence on the wall clock: a sleep never extends a hire on a stale reading

*Status: accepted 2026-10-03 · owner: architect · source bead
ranger-base-cxcv1 (the ruling half of ranger-base-5no4s, from
ranger-base-awt98) · sits under ADR 0010 §5 (the gate's decision table) and
beside ADR 0064 (the condition rows, which measure awake time) · amends
ADR 0064 D1's justification for `guard-blind` and nothing of its decision ·
material in `docs/notes.d/ranger-base-5no4s.md`*

## Context

`plan_guard_blind_max:` has two readers over one timestamp and one
threshold. G5's row (govern.go `blindPast`) subtracts the witnessed suspend
and is awake-denominated since ADR 0064. The hiring gate (dispatch.go
`blindGuard`) does not subtract, and ADR 0064 scoped itself out of it on
purpose — its header rules on what the shop may SAY, and the gate is what
the loop DOES. So the gate's clock was never ruled, and three records
describe a pair that does not exist: `blindPast`'s doc and ADR 0064 D1 argue
from the gate ("a suspended box hires nothing, so the budget was always
awake-denominated"), and `blindFork`'s doc says the degrade is "never
bounded by wall-clock" over a clock that includes the sleep.

**What the gate measures today, MEASURED 2026-10-03 on this box, go1.26.5
(ranger-base-5no4s):** nothing on purpose. `now.Sub(d.blindSince)` is a wall
difference only when `d.blindSince` carries no monotonic reading, and
`Time.Sub` prefers monotonic when both operands have one. `d.blindSince` is
written only on a successful read, from `readAt`, and `readAt` comes from
`c.now()` on a fresh read (monotonic: the gate is AWAKE) or from `e.At`
JSON-parsed off the shared snapshot on a cache hit (no monotonic: WALL).
`PlanCache.load` always reads the file, so every hit strips it; and on the
armed loop the hit is the common case, since `planGuardMaxAge` is
min(ttl, blind_max/2) = 5m at the default against a 3m pass, with the
cockpit and `posse cost` writing the same file. The denomination is frozen
for the whole blind window at whatever the last good read happened to be.
The pair agrees on the fresh path and disagrees on the hit: an accident, not
a policy, and either ruling costs a line of code.

No fixture can see the accident. Every blindGuard test drives `d.Now` off a
`time.Date` constant (planblind_helpers_test.go `blindT`), which carries no
monotonic reading, so every test takes the wall path; and a `Time` whose wall
and monotonic readings disagree cannot be synthesized (suspend.go's head —
`Add` moves both, only a real suspend separates them).

**The field's name for the question.** A blind budget is a lease: the gate
holds a right to hire on evidence it cannot refresh, for a bounded time. The
box sleeping is the unbounded process pause of the leases literature (Gray &
Cheriton 1989; Kleppmann, DDIA ch. 8 — cited through the distributed-systems
skill, verified there 2026-08-20), and its standard failure is **a paused
holder does not know it is dead**: it resumes believing its lease is still
good and "may go ahead and make some unsafe change". Awake-denominating the
gate's budget is that failure written as policy — the lease stops aging while
the holder is paused.

**The premise, already ruled once, the other way.** The plan window is an
ACCOUNT quota, not a box quota: planusage.go's head says the point of
watching it is "the operator's interactive headroom", and ADR 0018 rejected
"Expire the refusal at the window's own length" because "healed assumes
nothing else spent into the window meanwhile (the operator's interactive use
shares the account)". A suspended box hires nothing — true, and the reason
the ROW is awake — but the evidence the gate hires on is about a number other
spenders move while the lid is shut. 0018 refused to let a BRAKING reading
expire with time; the symmetric caution refuses to let a PERMISSIVE reading's
budget be EXTENDED by a sleep. ADR 0010 §5's table already says it for the
braking case: "neither can a clock".

What bounds the exposure, and keeps this a small ruling rather than a
reversal of 0064: `staleGate` is asked BEFORE the clock, so a last reading in
the braking band parks whatever the clock says, and the only passes either
ruling moves are those whose last reading LEFT ROOM. And the gate is reached
only when the probe FAILED — on a wake whose network is back in time for the
first pass, nothing here fires at all.

## Decision

**D1 — The gate's clock is wall time, and `plan_guard_blind_max:` has one
meaning: the maximum age of the evidence the guard will hire on with no
fresh reading.** Age of evidence is wall age, because the evidence is a
reading of an account that keeps moving while this box sleeps. The gate
subtracts no suspend. After a witnessed wake past the budget, the first
unattended pass whose probe fails forks exactly as a pass that was awake and
blind the whole time: park when the meter is the last armed brake or the
ledger is unreadable, degrade loud under the ledger otherwise (ADR 0010 §5).
The first successful reading clears it, as it always has — no sticky state,
no operator action. The cost is latency on one pass per wake whose network
is slower back than the pass, never correctness (ADR 0028 §1's rule).

**D2 — The denomination is made structural, not inherited.** `blindGuard`
takes the difference over `UnixNano`, the way suspend.go takes every wall gap
and for the reason its head gives: `UnixNano` has no monotonic variant, so
the class "this subtraction silently became awake because the seed carried a
monotonic reading" is impossible at the source rather than guarded by a line
no fixture can hold. The same difference feeds the "reading restored after
%s blind" line (dispatch.go planGuard), so the two numbers a reader sees for
one blind window are the same number. No seed is stripped and no `Round(0)`
is written: the seeds stay what they are and the subtraction stops caring.
No new field, state, config key or process.

**D3 — The row and the gate measure different quantities, and the record
says why.** G5's `guard-blind` is a statement about THIS shop: its monitoring
has been down for N awake hours, and a box with the lid shut is not a shop
with broken monitoring. That — not "a suspended box hires nothing" — is the
row's justification, and ADR 0064 D1 is amended to say so. The gate is
enforcement over a resource this shop does not solely own. They share a
threshold because the threshold is the moment the shop stops tolerating its
own ignorance; they share a timestamp because the snapshot is the instance's
one record of when it last knew. On a wake they diverge for the length of the
sleep, in the conservative direction on both sides: the enforcer errs toward
the operator's quota, the alarm errs toward not waking a human for a lid
closing.

**D4 — A braking pass inside the awake budget after a wake owes the
coordinator no pulse key.** "Deferred, never lost" (ADR 0064 D1) is the
row's promise and the brake does not defer; this is the named cost of the
divergence. It is not paid with a key, for three reasons in order of weight:
the key that used to arrive beside such a pass — `guard-blind:5h` one second
after a wake — IS the measured false positive 0064 removed, a P1 and an
evening (`docs/notes.d/ranger-base-sqxo1.md`); the brake is self-clearing on
the first good read, so a wake whose network returns inside a pass or two
costs one parked or degraded pass and nothing to prompt about; and a wake
whose network does NOT return reaches the row anyway, once awake quiet passes
`blind_max`, so the lag between brake and key is bounded by the budget and
the deferral covers the brake. What a braking pass does owe, and already
pays: its own line on the pass output, every pass, naming the wall age — the
park line per bead, the degraded line on `d.Out` — and the cockpit header's
blind clause. The SUSPENDED line sits in the same watch log within one
witness tick (`SuspendTick`, 30s), which is why the gate's line does not
restate the sleep: a second clock-reading in the brake's line would need the
pass to read the pair itself (ADR 0064 D3's call, a second caller) to be
reliably non-zero on the first pass after a wake, for a sentence the next
line in the log already carries. If a status or cockpit reading inside that
window ever draws a bead, the pair-read in the pass is the first build, not a
key.

## Consequences

- The 10-01 wake, replayed under this page and 0064 together: the pulse
  reads the pair, delivers only `question:…` (0064); the first pass's probe
  fails one second after the wake, `blindGuard` reads 5h30m of wall
  blindness, `staleGate` finds the last reading left room, the pass degrades
  loud under the ledger (caps armed) or parks (caps unset); the next pass's
  probe succeeds, prints "reading restored after 5h33m blind", and hires.
  One pass of latency, two log lines, no prompt.
- The build is ranger-base-5no4s's, one change in `blindGuard` and its two
  doc paragraphs, not a new bead: `blind` and the restored line computed
  over `UnixNano`; `blindGuard`'s "quiet tolerance … no amount of wall-clock
  is a reason to run, and none is a reason to stop" gains the sentence that
  the clock IS wall and why (D1, this page); `blindFork`'s "bounded by MONEY
  and never by wall-clock" is kept and qualified — the clock that opens the
  fork is wall and includes a sleep, the clock that never closes a degrade
  is any clock at all. The arm the finding specified — a blindGuard fixture
  on a Dispatcher whose `suspends` ledger covers the blind window — must
  reach `blindFork` and its line must name the wall age; the hand-driven
  clock pins the subtraction, and `UnixNano` carries the denomination (D2),
  exactly suspend.go's split.
- govern.go `blindPast`'s doc and ADR 0064 D1 no longer argue from the gate
  (amended with this page). ADR 0010 §5 gains one sentence pointing here so
  the gate's table reaches its clock in one hop.
- Price, ASSUMED one dinesh-sized slice already filed: two expressions, two
  doc paragraphs, one fixture. MEASURED cost of leaving it: zero incidents in
  the record, and the window is real — on the 10-01 wake the first reads
  FAILED (`plan-usage.log`: pulse `usage endpoint unreachable` at 04:08:57Z,
  cockpit the same at 04:08:58Z, cockpit `ok` at 04:09:24Z, 27s of dark
  network), and the gate was never reached only because no pass fired in the
  74s before the operator's TERM. A pass timer resumes with its remainder, so
  the chance a pass lands inside such a window is about its length over the
  interval — one wake in seven at 27s against 3m, ASSUMED from uniform
  phases. This page rules on the exposure, not on a loss.

## Alternatives rejected

**Nothing** (keep the inherited denomination). Priced first. Zero
incidents, as above; and still rejected, because three records state a
mechanism the code does not have, the gate's answer on a wake depends on
whether the last read was its own or the cockpit's, and a future reader of
`blindPast`'s doc would "fix" the gate toward awake on the strength of a
sentence this page finds false. The cost of the fix is two expressions.

**(a) — subtract the witnessed suspend in the gate too**, the resolution the
finding offered first, and the clever one: one call,
`d.suspendedSince(d.blindSince)`, off a ledger already in the process, and
every record becomes true as written. Rejected because it is the paused
holder renewing its own lease: a 79% reading before a 5h30m sleep hires
unmetered for a full `blind_max` of awake time afterwards, on an account the
operator may have spent into from another device the whole time. ASSUMED,
not measured: how far an account moves in a sleep. It is the direction that
is wrong, not the magnitude — 0018 already refused to let time heal a
reading, and this is time healing a reading. The row keeps (a)'s logic
because the row is about this shop, not about the account (D3).

**Strip the seed** (`readAt.Round(0)` at dispatch.go:681, and at the other
two seeds). Same outcome as D2 and worse: three sites to keep stripped
instead of one subtraction that cannot be anything else, and suspend.go's
head already says why `Round(0)` is "just as unkillable by a pin".

**A pulse key for the braking pass** (`guard-blind:…` restored, or a new
`guard-braked` key). Rejected under D4: the restored key is the measured
false positive; a new one forks the fingerprint, wakes monica URGENT for a
pass that clears itself on the next probe, and at 1184 sleeps a week is the
noisiest row the shop would have (0064's own count).

**Name the sleep in the brake's line.** One call and a format clause — the
reason it was tempting — but reliable only if the pass reads the pair first
(a second `suspendTick` caller beside the pulse's), for a sentence the
witness writes into the same log within 30s. Deferred, with its trigger
named in D4.

**Awake for both, and move the row's justification instead.** Rejected by
0064's evidence: the row measured on the wall cost a P1 and an evening, and
its awake reading was ruled on a record of 401 prompts. This page does not
reopen it.

## Claims

MEASURED (2026-10-03, this box, go1.26.5, ranger-base-5no4s): the four seed
paths and their denominations; `PlanCache.load` reads the file on every
call; `planGuardMaxAge` = 5m at the default; the fixture clock carries no
monotonic reading; the 10-01 wake's network was dark for 27s after the wake
and no pass fired before the loop was ended (`plan-usage.log`, read
2026-10-03 for this page). RULED, not measured: that account movement during
a sleep is the exposure (0018's premise, re-applied). ASSUMED: one pass of
latency per wake whose network is slower back than the pass, about one wake
in seven at the one dark window measured — the gate has never fired on one,
so the rate is a phase estimate and not a count.
