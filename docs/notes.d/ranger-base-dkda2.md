## One window reporting nothing voids the whole reading, including the window that still reads (ranger-base-dkda2)

The shipped plan adapter asks the provider's usage endpoint for two windows
and `Read()` returns the first `pct()` error, so a body carrying a perfectly
good reading of the second window is discarded whole:

```go
five, err := body.FiveHour.pct(anthropicWindow5h)
if err != nil {
	return nil, err
}
seven, err := body.SevenDay.pct(anthropicWindow7d)
```

On 2026-10-31 this instance's endpoint began answering `five_hour` as JSON
**null** — the whole object, not an object with a null `utilization` — while
`seven_day` kept arriving with a present, whole-percent `utilization`. Every
pulse and dispatch read failed `usage response is not the expected JSON: no 5h
utilization` and the guard went blind over a window it could still read, with
a threshold armed against that window and that window climbing.

**No config reaches it.** `pct()` runs for both windows whatever thresholds
are configured, so removing the 5h threshold changes nothing, and
`plan_guard_blind_max` set to zero disarms the blind brake rather than
restoring a sighted one. A code change is the only route back to a sighted
guard on the surviving window.

### MEASURED — the capture, and where it lives

The raw body shape, this instance's guard values, the account's own numbers
and the probe method are **instance-ops and are on the bead**
(`ranger-base-dkda2`, private store), per ADR 0024 D3's restate-and-cite:
NOTES.md "Privacy model" routes operational measurements and one deployment's
configuration to the private tree, and the first draft of this fragment was
refused by the prepare-commit-msg visibility gate for exactly that — it had
quoted the live thresholds. What is mechanism is here; what describes this
deployment is there.

What the capture settles, stated without the numbers:

- `five_hour` arrives as **JSON null**, not as an object with a null
  `utilization`, on a **200** response with a **live** credential — so this
  is neither a credential-shape outage (ranger-base-okbr) nor an auth failure
  wearing a different hat, and not an error envelope from a middlebox.
- It is **not a brief gap**: 82 minutes of the identical failure at a 2-minute
  read cadence, with a successful 200 at each end of that span still carrying
  a null `five_hour`. The surviving window's utilization MOVED across the
  span, so the adapter reads that one correctly.
- The body carries its own **enumeration of the account's windows**, and in
  this capture every window in it resets on a weekly horizon — nothing on a
  five-hour one. That is the positive evidence; see the bead for the shape.
- **UNKNOWN, and not claimed**: whether the rest of the body changed shape at
  the same moment. Nothing on this box had ever captured this response raw —
  the adapter decodes into a two-field struct and discards everything else —
  so there is no before-picture, and the decisive fact needs none.

**The bead's own hypothesis is falsified.** It was: the 5h window rolled over
and the endpoint answers null until the new window sees a request — a
transient reset, smallest fix "present-key-null = fresh window 0%". A reset
answers `utilization: 0`, not a null object, and 82 minutes is not a
rollover. Reading null as 0% would be a **floor** — a guess wearing the
meter's authority, which is `Read()`'s own comment's rejected case
("never a floor (it would blind the guard at every window reset)") read the
wrong way round. Under the proposal below the window is reported as
*nothing*, which is what it is.

### Why the current behaviour is deliberate, and why that is still the bug

`{"five_hour":null,"seven_day":{"utilization":0}}` is a pinned case
(`internal/posse/planusage_test.go`) expecting a `5h` error, so this is not an
oversight. It was written against ranger-base-65s: a body of the wrong shape
(`{}`, an error envelope, a renamed key) decoded to two zero values and came
back as a **successful** reading of 0%, the guard opened, and the blind clock
never armed — fail-open in the one direction the interlock must never fail.
Absent-is-an-error is the fix that bought the ADR 0018 blind machine.

What it cannot do is tell **"the response is garbage"** from **"this provider
stopped reporting one of the two windows I ask for."** Both are silence to
`pct()`, and only one of them should void a reading that is otherwise sound.

### Proposal — the smallest rule (RULED AND APPLIED; see below)

**A was ruled and shipped** (operator, 2026-11-01, on `ranger-base-z5snh`;
`docs/notes.d/ranger-base-z5snh.md` is the decision and the pins). What
follows is the proposal as this bead wrote it, kept as written: the "NOT
APPLIED" it carried was true of this bead, which measured and proposed only.
B was not chosen and is still live.


ADR 0010 / ADR 0018 plan_guard semantics are the operator's, so this bead
measured and proposed only. The decision is filed as `ranger-base-z5snh`
(P1, `-l question`), which carries the full version of what follows.

**A.** In the adapter only: a window key that is **absent or null** drops out
of the returned `PlanUsage` instead of failing the read, **provided at least
one window remains**. A body that yields **zero** windows stays an error
exactly as today — `{}`, an error envelope and a renamed `utilization` all
land there, so ranger-base-65s's class keeps the blind machine. Every value
check is unchanged and still fails the whole read: a window that IS reported
and implausible is drift (ranger-base-cb0s, ranger-base-959m6). **Only
silence is absence.**

The harness half already handles the consequence and needs no change.
`Dispatcher.unmatchedThresholds` exists for exactly a `plan_guard_<window>:`
aimed at a window the provider does not report, and would say so once per
process, naming the windows it does report.

Cost: three pinned cases move, and the renamed-key and `{}` cases must gain
explicit arms asserting they still fail — under A they are the whole of what
holds the blind machine.

**The exposure A buys, named:** a provider that silently stops reporting the
TIGHTER window degrades the guard to the looser one, announced by one stderr
line per process and nothing else. That is ADR 0010's "dollars and window
percentage are different quantities" residue one notch over — a brake
measuring a different window rather than a different quantity — and it is why
this is a question and not a commit. If a louder surface is wanted, the honest
place is a per-pass line, never a refusal.

**B,** named so A is not mistaken for the end of the road: read the windows
out of the body's own window enumeration instead of two fixed keys. It is
self-describing and would have survived this change with no edit. It is also a
new provider grammar — window NAMES would come from strings that would rename
`plan_guard_<window>:` keys out from under an operator's config, and the
enumeration carries an active/inactive state the harness has no vocabulary
for. It needs its own measurement first, and that measurement needs its own
venue: re-asking this endpoint at its rate-limit boundary draws a fresh 429
(ranger-base-dvxac, 2026-09-02), so capturing the vocabulary wants an hour in
which nothing else on the box asks.

### The probe, and the cost of running it

Method and output are on the bead. Nothing was shipped: the probe was a
throwaway program inside the module, so it could reach the credential seam,
and it is deleted. A credential-reading shape dumper is its own design
question — but note that this is the fourth time the adapter has been taught
a fact about its provider's grammar by an incident (ranger-base-65s shape,
ranger-base-cb0s range, ranger-base-959m6 scale, this one absence), and each
time the diagnosis began with having no capture of what the endpoint actually
sent.

**Declared, because it is the kind of cost that should be:** the endpoint's
own rate limiting was already in force before any probe here, so those
refusals are not this bead's — but probe reads did collide with the pulse and
pushed one cooldown up a streak step. The cache heals that and the reading it
protects is unchanged. Probing stopped there rather than buy the window
vocabulary with another read.
