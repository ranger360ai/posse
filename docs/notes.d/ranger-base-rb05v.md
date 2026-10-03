## How long a healthy pass takes, and what every long one was (ranger-base-rb05v)

*bead: ranger-base-rb05v (P1, architecture) · from ranger-base-sqxo1 · record:
ADR 0063 · everything below is MEASURED 2026-10-03 on this box (darwin 25.4.0,
posse 0.5.0 generations 8fa107db through 1b8ffe2d) unless marked ASSUMED*

## 1. The question the census answers

ADR 0063 had to defend a threshold with numbers: an acting watchdog must clear
the slowest healthy pass this shop produces, and nobody had measured that
distribution. The bead named the instrument — a census of
`state/dispatch-watch.log` pass headers — so this is that census, committed
as `scripts/pass-census.py`.

## 2. How it reads the log

Only two kinds of line in the watch log carry a timestamp: the generation
banner (`== dispatch --watch armed <date> <time> · pid N ==`) and the pass
header (`── pass N · HH:MM:SS`). The pass summary (`N dispatched · next pass
in X`) carries the wait but no time. So for each pass with a successor:

- **U** = next header minus this header = the pass plus the wait. This is
  exactly what the pass clock reads (`notePass` to `notePass`, shifted by one
  pass), so the 12m budget is compared against U.
- **L** = U minus the printed wait = the pass itself. Exact when the wait ran
  out; a lower bound when a carried leg's settle woke the loop early
  (watch.go's `settled()` arm), because then the real wait was shorter.

The rotated log's head has no banner (it rotated away), so its 149 passes are
dated 2000-01-01 and kept; midnight rollover is handled by a header reading
earlier than its predecessor. Passes with no successor (the last of each
generation, and the two October hung passes) are excluded: there is nothing
to measure them against.

## 3. The distribution

All 697 passes, every wait (3m, 5m, 10m, 20m):

```
U: median 5m33s  p90 9m35s  p95 10m40s  p99 21m44s  max 63m27s
L: median 2m25s  p90 6m06s  p95 7m04s  p99  9m31s  max 60m27s
U > 12m: 20   L > 12m: 4   (all four L > 12m are the 09-20 cluster, §4)
```

3m-wait passes only, with the 09-20 cluster set aside (n = 650):

```
U: p50 5m20s  p90 8m44s  p99 11m02s  max 13m37s     U > 12m: 4 of 650 · U > 24m: 0
L: p50 2m20s  p90 5m44s  p99  8m02s  max 10m37s
```

Per generation (passes · waits · max L · L over 6m / 9m / 12m):

```
log.1 head (undated)      149 · 3m          · 10m37s · 29 / 3 / 0
2026-09-07 pid38252        85 · 3m          ·  9m31s ·  5 / 1 / 0
2026-09-07 pid63181        76 · 3m          ·  7m39s ·  4 / 0 / 0
2026-09-10 pid10513        61 · 3m          ·  7m47s ·  6 / 0 / 0
2026-09-11 pid50179        83 · 3m          ·  7m43s ·  8 / 0 / 0
2026-09-20 pid49621       101 · 3m          · 60m27s ·  4 / 4 / 4   ← §4
2026-09-28 pid84335        76 · 3m          ·  6m48s ·  2 / 0 / 0
2026-09-28 pid39415        43 · 5m/10m/20m  ·  7m40s · 11 / 0 / 0
2026-10-01 pid60923         5 · 3m          ·  6m17s ·  1 / 0 / 0
2026-10-02 pid49599         6 · 3m          ·  7m40s ·  3 / 0 / 0
```

The four awake passes over the 12m budget at 3m/3m: rotated-log passes 291
(U 12m42s), 329 (13m37s) and 193 (12m16s), and 09-07 pass 76 (12m31s). The
first two are the two witness lines in the rotated log (§5); the other two
fell inside the 3m tick's granularity and were never named.

## 4. The 09-20 cluster was a lid-closed sleep, not four slow passes

Passes 18–21 of the 09-20 generation read as 60m27s, 38m21s, 44m05s and
33m15s. Three readings from the loop's own goroutines, then one from the box:

| witness | cadence | expected in pass 19's 41m | printed |
|---|---|---|---|
| pulse (`pulse_interval: 2m`, writes every tick) | 2m | ~20 ticks, ~40 lines | 6 lines |
| guard clock (level-triggered, every tick over the line) | 3m | ~13 lines at loadavg 76–131 | 1 line |
| pass-stall watchdog (12m budget, 3m tick) | 3m | one line by minute 15 | 0 lines |

A Go ticker on a box at loadavg 76 fires late by seconds, not by half an
hour, and the darwin monotonic clock does not advance across a suspend
(watchdog.go's head, MEASURED 09-03) — so three tickers writing at a sixth
of their rate is a process that was not awake, never a process that was
slow. Then powerd (`/usr/bin/log show --predicate 'process == "powerd" AND
category == "sleepWake"'`): `Entering Sleep state due to 'Maintenance
Sleep'` with `ClamshellState. Closed : 1` from 14:30:48, dark-waking about
once a minute, through at least 15:17 — inside passes 18, 19 and 20's
windows. The unified log holds nothing in that category between 13:40 and
14:30, so the sleep's onset inside pass 18 is not pinned to a minute
(ASSUMED earlier than 14:30 from the pulse count: pass 18's window holds
four pulse ticks, about eight awake minutes of sixty). pmset's own log
starts 09-26 and plan-usage.log starts 09-28, so neither reaches this
afternoon.

This is occurrence 1 of ranger-base-sqxo1 — the 10-01 Low Power Sleep —
three weeks earlier, unnamed because suspend.go did not exist. The same
afternoon today prints four `was SUSPENDED` lines.

## 5. Every pass-stall witness line in the record, and what followed

| where | line | what followed |
|---|---|---|
| log.1:16007 | 14m past 12m, last pass 16:59:34, 5 prompts in flight, loadavg 113.42 | pass 292 header at 17:15:16 — completed ~100s after the line |
| log.1:23423 | 13m past 12m, last pass 21:24:21, "nothing in flight", a refill storm re-offering 109 ready beads to every free seat | pass 330 at 21:38:59 — completed ~90s after the line |
| log:17345 | 10-01, 14m past 12m, last pass 18:23:44, CPU throttled at 1–2% battery | pass 6 completed 65s later, four seconds before Low Power Sleep |
| log:17518 | 10-02 23:55, 12m past 12m, last pass 23:43:38, loadavg 95.86 | pass 6 completed; pass 7 started 23:56:42 and never finished — the box was shut down at 00:07, 10m18s in, inside its budget |

Four lines, three healthy passes, one true finding that preceded a hang the
budget never reached. Awake passes in the record that did not come round on
their own: 0 of 697.

## 6. What an acting threshold would have done, option by option

- **Option 3, a hire brake on pass age:** every candidate above sat over
  `load_guard: 60` (113, 95), on a throttled CPU, or asleep. The load guard
  was already refusing on every one with a reading. Zero additional refusals,
  three false ones.
- **Option 4, ending the pass:** three healthy passes ended 65–100s before
  they would have completed; occurrence 2 untouched (inside budget).
- **Option 5, ending the loop:** the same three, each then `loop-dead`
  URGENT until the next herdr start (`[[startup]]` is autostart's only
  trigger); 09-20's four would have ended a loop on a sleeping laptop four
  times.

## 7. The residual the stop still has (D3's numbers)

After sqxo1 a TERM is honoured within one stage. A stage is its children
times their deadlines: `BdTimeout` 3m (beads.go) per beads dir, two
configured; `GitTimeout` 2m (githang.go) per repo, three configured;
`HerdrControlTimeout` 2m plus `HerdrWaitGrace` 1m (herdr.go). Worst stage
ASSUMED about 6m. No stage has been measured at its ceiling; the one hung
pass on record (10-02 pass 7) was between the reap sweep and the gather for
at least 10m18s with its context cancelled, which is consistent with two
stages at their ceilings on a box at loadavg 68–95 with 6.1G of swap, and
is not a measurement of either.
