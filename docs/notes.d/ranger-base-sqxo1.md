## The 2026-10-01 "watch loop hung ~6h" was a suspend, and the second one (ranger-base-sqxo1)

ranger-base-sqxo1 was filed as a hang: "watch loop hung ~6h after pass 6
(18:26 local): 'prompt(s) in flight, gathering' never returned". It was not a
hang. The laptop went into **Low Power Sleep on a 1% battery** and was gone
for 19807 seconds. This is the measurement, because it is the second time this
shop has read a sleeping box as a hung loop — ranger-base-wj7e9 on 2026-09-03
was the first, and it was retracted on the same ground — and because the two
readings that *did* fire both times were correct and both pointed away from
the cause.

### 1. What happened, MEASURED 2026-10-02 on this box

Three independent witnesses, agreeing to the second.

```
pmset -g log      2026-10-01 18:22:44  BatteryHealth Warning level: 2 time: 10 cap: 10
                  2026-10-01 18:34:01  BatteryHealth Warning level: 3 time: 4 cap: 2
                  2026-10-01 18:38:44  Notification  Display is turned off
                  2026-10-01 18:38:49  Sleep  Entering Sleep state due to 'Low Power
                                       Sleep':TCPKeepAlive=inactive Using Batt
                                       (Charge:1%) 19807 secs
                  2026-10-02 00:08:56  Wake  Wake from Hibernate [CDNVA] : due to
                                       lid/UserActivity Assertion Using AC (Charge:100%)
                                       HibernateStats hibmode=3, WakeTime 2.054 sec

state/plan-usage.log   the PULSE's own plan reads stop at 2026-10-01T22:34:55Z and
                       resume at 2026-10-02T04:08:57Z — 18:34:55 and 00:08:57 local,
                       one second after the wake.

state/plan-usage.log   the COCKPIT's reads, a different process entirely, stop at
                       22:35:01Z and resume at 04:08:58Z. Two processes do not stall
                       together; a box does.
```

19807s is 5h30m07s. 18:38:49 + 19807s = 00:08:56.

### 2. What the watch log actually says

`state/dispatch-watch.log` lines 17189-17362, watch pid 60923, posse
0.5.0+1b8ffe2d, launched 17:46:55 with `--watch 3m --max-interval 3m -n 4
--resume`:

- **pass 6 COMPLETED.** Header at 18:26:12, then `– ranger-base-5jjtn
  dinesh-posse busy this pass`, the three skip lines, `… 1 prompt(s) in
  flight, gathering`, `◷ 1 bead(s) still with their agent — claims kept`, and
  `   1 dispatched · next pass in 3m0s (ctrl-c to stop)`. Those last lines
  landed in the ~65 seconds between the watchdog line at 18:37:44 and the
  sleep at 18:38:49. The gather returned and the carry line is absent because
  the set drained to zero — the pass did everything it is supposed to do.
- **The "hours of pulses" are two rounds in seventy-four seconds.** Everything
  after pass 6's summary — the `monica box previews` line, the
  `guard-blind:5h; loop-mute` pair, the load-guard reading at loadavg 115.32,
  and the final `pulse: skipped (working)` — is between the wake at 00:08:56
  and monica's TERM at ~00:10. It reads as 5h45m of pulses because **pulse
  lines carry no timestamp**; only pass headers do.
- **The one real finding was true and self-resolving.** `◷ watchdog: no pass
  has completed for 14m — past its 12m budget (last pass 18:23:44)` at
  18:37:44. Pass 6 genuinely took 12m30s, on a CPU macOS was throttling for a
  battery at 1-2% since 18:22. It completed ~65s later. The witness's
  once-per-stall rule (watchdog.go) was right here: a second line would have
  been about a pass that was already done.

### 3. Why the asks on the bead were already answered

- **"the prompt-gathering wait needs a bound"** — it has had one since
  ranger-base-3ryit: `gatherRound` runs under `d.GatherWindow` (the loop's
  base interval, 3m on the armed loop) and carries what is still outstanding
  into the next pass (`internal/posse/passcarry.go`). It is not where the
  12m30s went; `… 1 prompt(s) in flight, gathering` is the LAST thing pass 6
  printed before its summary, not the first.
- **"a regression pin that a pass with an in-flight prompt that never settles
  still completes within budget"** — `TestQAPassCompletesWithPromptsStillOutstanding`
  (`internal/posse/passcarry_qa_test.go`), whose fixture fires a leg with
  `prompt-delay-ms` two orders of magnitude outside every window it asserts
  and then waits for pass 2. It was green at the binary that ran this
  incident.
- **"surface `loop-mute` as URGENT in `posse status`"** — it has been G7's
  third key, `GovUrgent`, since ranger-base-n00wn (`internal/posse/govern.go`,
  `watchLogRow`), and the cockpit header renders it as `· loop mute`. It is
  how monica found this incident.

### 4. What was actually missing, both times

The reading that says the box was asleep. `watchdog.go` names it and declines
it in the same breath — *"Naming the wake is a DIFFERENT reading (wall elapsed
minus monotonic elapsed) and a different line; this file is not it"* — and
that was the right call for one incident and the wrong one for two.

Every clock inside the loop measures **awake** time, because `LastWrite`,
`lastPass` and `d.now()` all carry Go monotonic readings and the reading Go
takes on darwin does not advance across a suspend. MEASURED 2026-10-03 03:50Z
on this box, because it is the number the whole fix rests on:

| reading | value | behaviour |
|---|---|---|
| wall uptime (`now - sysctl kern.boottime`) | 2509051.5s | — |
| `CLOCK_MONOTONIC` | 2509051.2s | tracks wall — **includes** suspend on darwin |
| `CLOCK_UPTIME_RAW` == `mach_absolute_time` | 1352850.5s | **excludes** suspend |

1156201s between the last two: thirteen days and nine hours of suspend inside a
twenty-nine-day uptime. And it is `mach_absolute_time` that Go reads —
`runtime.nanotime1` for `GOOS=darwin` is the `mach_absolute_time` trampoline
(go1.26.5, `src/runtime/sys_darwin.go:304`), not `clock_gettime` — so a Go
monotonic difference on this platform is awake time, and darwin's
`CLOCK_MONOTONIC` is a trap rather than an alternative. The 2026-09-03 reading
in `watchdog.go`'s head measured the same thing less directly (wall uptime
323192s against a runtime monotonic of 296514s) and agrees.

So every clock inside the loop was correctly silent. Every reading **outside**
the loop is wall time, so all of them said something true and misleading:

| reading | what it said | what it meant |
|---|---|---|
| `guard-blind:5h` (pulse) | the plan reading is 5h30m old | the box was off for 5h30m |
| `loop-mute` (G7, URGENT) | the watch log went 5h30m untouched | the box was off for 5h30m |
| `--watch-status` log note | log written 5h ago | the box was off for 5h30m |

Three alarms, no cause. That is the whole of the operator cost here.

### 5. The fix: `internal/posse/suspend.go`

A fifth reading on the watchdog's goroutine, on a 30s ticker of its own
(`SuspendTick`), outside both watchdog budgets because neither budget arms it:

```
wallGap := time.Duration(wall.UnixNano() - prevWall.UnixNano())
monoGap := mono - prevMono
slept  := wallGap - monoGap        // >= SuspendFloor (30s) is a suspend
```

Three things in it are load-bearing and each is pinned
(`internal/posse/suspend_qa_test.go`):

1. **`UnixNano`, never `Time.Sub`.** `Sub` prefers the monotonic reading when
   both operands carry one, so `wall.Sub(prev)` *is* the monotonic gap and
   `slept` is permanently zero. It is one keystroke, and **no fixture can
   catch it**: there is no way to build a `time.Time` whose wall and monotonic
   readings disagree (`Add` moves both; only a real suspend separates them),
   so `Round(0)` would be just as unkillable. Pinned at the source instead —
   `TestQASuspendWallGapIsNotAMonotonicSubtraction` parses `suspendTick` and
   refuses any `.Sub` in its body.
2. **Its own seam (`Dispatcher.Clocks`), never `d.Now`.** Fixtures in this
   package age `d.Now` by hours, and `watchhang_qa_test.go` runs one at
   10000x real time to reach a budget — which *is* wall moving while monotonic
   does not. Reading `d.Now` here would print a suspend line into every one of
   them.
3. **A floor for clock STEPS, not for skew.** Both readings are taken in one
   call a nanosecond apart, so a tick delayed by a loaded box grows both gaps
   equally and is not a false positive. The only other thing that separates
   them is the wall clock being *set* (ntpd, an operator), and a one-second
   correction is not a suspend.

### 6. What it deliberately does not do

- **It does not shorten the pass the loop is waiting out.** The next-pass
  timer is monotonic too, so a wake lands with whatever was left of it still
  to run — 2m50s of a 3m interval on the night this was filed, against the
  5h30m it explains. ADR 0028 §1's rule holds: a late wake costs latency,
  never correctness.
- **It does not report a BACKWARD wall step**, which would make every
  wall-clock reading in the shop read fresher than it is. Pinned silent, not
  pinned absent.
- **It does not teach the governance set about a suspend.** `guard-blind` and
  `loop-mute` are built in `govern.go` from files, by any process that asks,
  and the pulse delivers the KEY and not the detail — so the token monica was
  handed can only change by changing the shop's condition contract. Handed off
  rather than decided here.
