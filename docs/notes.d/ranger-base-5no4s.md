# The plan-guard blind gate has no clock of its own: it inherits one from the snapshot's provenance (ranger-base-5no4s)

Filed from ranger-base-awt98's one live finding — the plan-guard blind
*budget* is gated on a wall clock while the row justifying it measures awake
time (ADR 0064 D1). The finding is real. It is also narrower and worse than
it was filed as, and the measurement below is what the ruling turns on, so it
is worth having before anyone picks a side.

## What the bead said, and what is actually there

The bead reads the pair as "the row measures awake, the gate measures wall":

- `govern.go` `blindPast` (G5, the row) — `blind = now.Sub(at) -
  in.suspended(at, now)`, awake since ranger-base-szzoj.
- `dispatch.go:934` `blindGuard` (the gate) — `blind := now.Sub(d.blindSince)`,
  with no `suspended` caller anywhere in dispatch.go.

Both halves of that are true as written. But `now.Sub(d.blindSince)` is not a
wall difference *by construction*; it is a wall difference only when
`d.blindSince` carries no monotonic reading. `Time.Sub` prefers the monotonic
reading whenever **both** operands have one (suspend.go's head, which is where
this package already knows it), `now` is `d.now()` — real `time.Now()` in
production, monotonic-bearing — so the gate's denomination is decided entirely
by which of the three seed paths last wrote `d.blindSince`:

| seed | value | monotonic? | gate measures |
|---|---|---|---|
| `watch.go:123` loop start | `d.now()` | yes | **awake** |
| `dispatch.go:628` hand-run pass | `now` | yes | **awake** (unreachable: `past` needs `d.Unattended`) |
| `dispatch.go:681`, **fresh** read | `c.now()` via `PlanCache.Read` | yes | **awake** |
| `dispatch.go:681`, **cache hit** | `e.At`, JSON-parsed off the shared snapshot | no | **wall** |

MEASURED 2026-10-03, this box, go1.26.5 (`scripts`-free, a seven-line
`time`+`encoding/json` program; `t.String() != t.Round(0).String()` is the
monotonic-reading observable):

```
fresh readAt    mono=true    now.Sub => MONOTONIC (awake)-difference
cachehit readAt mono=false   now.Sub => WALL (includes suspend)-difference
fixture clock   mono=false   now.Sub => WALL (includes suspend)-difference

fresh.String() = 2026-10-03 19:06:49.867889 -0400 EDT m=+0.000090584
hit.String()   = 2026-10-03 19:06:49.867889 -0400 EDT
serialized     = {"at":"2026-10-03T19:06:49.867889-04:00"}
```

`PlanCache.load` always goes to disk (`os.ReadFile` + `json.Unmarshal`,
plancache.go:520), so **every** cache hit strips the reading. There is no
in-memory entry that keeps it.

## So the asymmetry is a coin flip, not a policy

`d.blindSince` is only written on a *successful* read (dispatch.go:681), so
whatever denomination the last good read left is frozen for the whole blind
window that follows. Which one that is depends on whether that read was
dispatch's own or served from the shared snapshot — and on the armed loop the
cache hit is the common case: `planGuardMaxAge` is `min(ttl, blind_max/2)`
(plancache.go:723), i.e. 5m at the default 10m `plan_guard_blind_max`, against
a 3m pass interval and the cockpit and `posse cost` writing the same file
(`PlanCache("cockpit")`, `PlanCache("cost")`).

Two consequences, and they are the whole point of this fragment:

1. **The pair AGREES on the fresh-read path and DISAGREES only on the cache
   hit.** On a fresh-read seed the gate is already awake-denominated and G5 is
   awake, so they match. On a cache-hit seed the gate is wall, G5 subtracts the
   witnessed sleep, and the bead's finding is exactly right. The row's own
   clock is wall either way (`LastReadAt()` re-reads the file), which is why
   ranger-base-szzoj's explicit subtraction was needed there and reads as the
   only deliberate clock in the pair.
2. **Neither resolution is a no-op, and (b) is not documentation-only.** The
   bead offers (a) subtract in the gate too, or (b) the record says the
   enforcer keeps the wall clock deliberately. The enforcer does not keep a
   wall clock today — it keeps an accident — so (b) costs code as well: the
   seed has to be stripped (`readAt.Round(0)`, or the subtraction taken over
   `UnixNano` the way suspend.go does it) before a doc sentence about it is
   true. (a) costs the one call the bead names and makes all four rows above
   read awake.

**No pin can see any of this.** Every blindGuard fixture drives `d.Now` off
`blindT = time.Date(2026, 8, 19, …)` (planblind_helpers_test.go:14), which
carries no monotonic reading, so every test takes the wall path — the third
row of the measurement. This is the structural blind spot suspend.go's head
already names ("no way to synthesize a `Time` whose wall and monotonic
readings disagree … so the class is pinned at the source"). A behavioural pin
on a hand-driven clock can hold whichever subtraction the ruling picks; it
cannot hold the *denomination*, and a pin for that belongs at the source.

## The premise the ruling turns on, already ruled once

ADR 0064 D1's justification is about the gate — "`plan_guard_blind_max` bounds
how long the shop may HIRE without a reading, and a suspended box hires
nothing, so the budget was always awake-denominated" — while D1's decision and
D2's plumbing name only `watchLogRow` and `blindPast`, and the header scopes
the page to "what the shop may SAY" and not "what a loop may DO". That gap is
the live finding and it is not ranger-base-szzoj's miss.

What the bead did not carry is that the load-bearing clause of that
justification is contradicted by an earlier ruling. ADR 0018's rejected
alternative **"Expire the refusal at the window's own length"** was rejected
on this ground:

> "healed" assumes nothing else spent into the window meanwhile (**the
> operator's interactive use shares the account**), and some providers' week
> has no intra-week reset at all.

The plan window is an *account* quota, not a box quota (planusage.go's head:
"the point of watching them is the operator's interactive headroom"). A
suspended box hires nothing, so the sleep cannot be the shop's own exposure —
which is sound for the ROW, a statement about whether this shop was quiet and
unmonitored. It does not follow for the GATE, whose tolerance is tolerance for
*ignorance* about a number other spenders can move while the lid is shut.

Note the direction of the earlier caution: ADR 0018 refused to let a
**braking** reading expire with time. The symmetric caution here is to refuse
to let a **permissive** reading's blind budget be *extended* by a sleep. Both
are the conservative read, and they point at (b).

Two things keep that from being decisive on its own, and they are the rest of
the ruling's material:

- `staleGate` (dispatch.go:987) is asked **before** the clock, so a last
  reading already in the braking band parks the pass whatever the clock says.
  (a) therefore only changes passes whose last reading *left room* — the
  exposure is "the account crossed the threshold during the sleep, from
  spenders other than this shop", not "the shop resumed over its ceiling".
- A rolling 5h window ages the shop's own contribution out across a 5h30m
  sleep, so a stale reading is conservative with respect to this shop and
  non-conservative only with respect to the operator.

## What is undisputed either way

The brake does not defer. ADR 0064 D1's "deferred, never lost" is the ROW's
promise; `blindFork` parks or degrades on the pass in front of it. Before
ranger-base-szzoj the pulse delivered `guard-blind:5h` beside a braking pass;
after it the row is clear on a witnessed wake while the gate may still brake,
so the only coordinator-facing signal for a braking pass is gone. That is a
cost under (a) *and* under (b) — under (a) the brake also stops, under (b) it
brakes silently — and whichever is ruled, something has to say so out loud.

## Status

The build half is one call in internal/posse and is gated on the ruling; the
ruling is architecture's, because ADR 0064 scoped itself out of the gate on
purpose. Filed for the lane with this fragment as its material. Nothing in
dispatch.go changed under this bead.
