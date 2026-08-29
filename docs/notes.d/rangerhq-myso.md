## The grok weekly pool has a meter after all (rangerhq-myso)

ADR 0010 §3 wrote `plan_guard_overflow_cap:` in **beads** because "the
overflow pool has no meter the harness can read", and §4 named a
provider-side usage endpoint as the loop closer. There still is no endpoint —
xAI shows utilisation on Settings -> Usage, to a human. The loop closed from
the other side: **grok writes its own cost per turn to disk**, so the pool is
computable locally with zero network calls, no credential and no keychain.

```
utilisation% = (USD since the last weekly reset) / (USD per point)
```

Three inputs, all **config**, none of them a constant in this repo:
`grok_guard_week:` (percent), `grok_pool_reset:` (`<weekday> <HH:MM>`, local),
`grok_pool_usd_per_point:`. The calibration figures behind the last two are
the operator's and live on the private db (ranger-base-toe, per
ranger-base-3jg); posse ships none, and logs the factor it used on every pass
that takes a reading — it is empirical, derived from grok's own list-price
accounting, and it goes stale the day xAI reprices with nothing failing.

**What the number is not.**

1. *Not the vendor's.* Every line says `estimated`. Nothing may present it
   as authoritative.
2. *Not complete.* It sees grok sessions written on THIS box — dispatch's and
   the operator's own CLI, which spend the same pool and are therefore both
   counted. It cannot see that pool spent from a phone or the web. So it is a
   **floor**, it under-reports, and a threshold must be sized knowing that.
3. *Not blindable.* The plan guard's whole blind-clock apparatus (ADR 0018)
   exists because a credential or an endpoint can stop answering. This reads
   local files: there is no transient outage to wait out, so there is no
   clock, and an armed guard missing its reset or its factor is **OFF with one
   stderr line**, never parked. Parking on a condition no retry can change is
   a brake with no release.

**Why it earns a second guard when a bead cap already bounds the same pool.**
The Claude 5h window heals in five hours. The SuperGrok week has no
intra-week reset, so exhaustion is *days* of nothing, and it takes the
operator's own Grok — Chat, Voice, Imagine, one bucket — down with the fleet.
`uncounted_cap_grok:` counts beads because nobody could count dollars; this
counts the dollars.

**Where it applies: per BEAD, not per pass.** ADR 0013 §3 — a meter gates only
the work that can spend it. Skipping a whole pass because grok is drained
would park claude lanes on somebody else's pool, the exact defect ADR 0010 §1
moved the plan guard's verdict per-bead to fix. The check sits beside the
account stage on the runtime the launch is *actually* going to, so a bead ADR
0010 moved onto grok is judged by grok's pool. It runs ahead of
`uncounted_cap_<runtime>:` deliberately: where a runtime has a reading, the
reading is what the operator wants named, and the bead cap is the stand-in for
the *absence* of one.

**Two traps the implementation had to get past.**

- *The 2× trap* is real but is **not** what the ordering bead described. grok's
  `usage` carries a `modelUsage` breakdown that sums to exactly the aggregate
  beside it, so a decoder walking every `costUsdTicks` doubles the total —
  measured 171/171 records equal, ratio 2.0000 (ranger-base-k7nb, cost_grok.go).
  The records are **not** cumulative snapshots: there is one `turn_completed`
  per `prompt_id` and the values are not monotonic within a session, so
  max-per-session would silently drop every turn but the priciest. Per-turn
  aggregates, summed. This guard inherits that decoder rather than re-deriving
  it, which is why it does not repeat the mistake in either direction.
- *The reading is lazy.* It is taken on the first bead that resolves onto grok
  and memoized for the pass, so a fleet with no grok work scans no transcripts
  and prints no line — uncountedReport's rule.

**One honest limit in the floor.** An unreadable transcript FILE is counted as
unread and named on the line. An unreadable session *directory* is not:
`grokCost.Transcripts` locates with `filepath.Glob`, which ignores I/O errors
by contract, so the directory vanishes with `errs` empty (measured 2026-08-29:
two session dirs, both readable → 2 matches; one `chmod 000` → 1 match, nil
error). Filed as ranger-base-yljd; it is the locator's property, not this
guard's.

Code: `internal/rhq/grokpool.go`, one call site in `dispatch.go` beside
`uncountedSkip`. Pins: `internal/rhq/grokpool_test.go`. Filed alongside:
ranger-base-qs0z (ADR 0010's "no meter" premise), ranger-base-0lg6 (grok
declares no `cost_adapter:` while a provider is registered for it),
ranger-base-yljd (the locator above).
