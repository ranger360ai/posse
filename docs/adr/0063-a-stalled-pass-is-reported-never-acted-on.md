# ADR 0063 — A stalled pass is reported and never acted on: no acting threshold has a target in the record

*Status: accepted 2026-10-03 · owner: architect · source bead
ranger-base-rb05v (from ranger-base-sqxo1) · sits under ADR 0028 §4 (every
refill originates in the one watch process) and ADR 0028 §1 (a late wake
costs latency, never correctness) · leaves watchdog.go's pass budget and its
once-per-stall rule as they are · measurements in
`docs/notes.d/ranger-base-rb05v.md`, instrument `scripts/pass-census.py`*

## Context

The ask, in monica's words for the operator after the second occurrence on
ranger-base-sqxo1: "the bound on the gather and a watchdog that acts, not
logs". The gather has been bounded since ranger-base-3ryit; the operator's
own stop has been read at every stage boundary of a pass since sqxo1. This
page decides the other half — what a loop may DO about its own pass beyond
saying so — and it decides it from the record rather than from the two
nights that prompted the ask, because those two nights pointed opposite ways
(one a sleep, one a pass still inside its budget).

The question has a shape the field named: a watchdog that acts on a timeout
is a failure detector, and a failure detector on one box cannot tell a slow
process from a paused one — "no timeout value is correct" (Kleppmann, DDIA
ch. 8 and the 2016 locking essay; cited through the distributed-systems
skill, verified there 2026-08-20). A suspended laptop is the unbounded pause
in its purest form, and the clocks that would fire the detector are exactly
the ones the pause stops.

**The record, MEASURED 2026-10-03** by `scripts/pass-census.py` over both
generations of `state/dispatch-watch.log` (713 pass headers, 15 loop
generations, from before 2026-09-07 to 2026-10-03; 697 passes have a
successor to measure against):

| reading (3m-wait passes, awake, n=650) | p50 | p90 | p99 | max |
|---|---|---|---|---|
| the pass itself (header to summary) | 2m20s | 5m44s | 8m02s | 10m37s |
| header to header (what the pass clock reads) | 5m20s | 8m44s | 11m02s | 13m37s |

- Over the pass clock's 12m budget: 4 of 650. Over 24m: 0.
- The four passes set aside are 09-20 passes 18–21, 33–60m each. powerd
  logs clamshell-closed Maintenance Sleep from 14:30:48 across their window,
  and the loop's three independent tickers (2m pulse, 3m guard clock, 3m
  watchdog) wrote at about one sixth of their awake rate through all four.
  That is occurrence 1's shape three weeks earlier, unnamed because
  suspend.go did not yet exist.
- The pass-stall witness has fired four times in the whole record. Three
  named a healthy pass that completed 65–100s later (two in the rotated log,
  one on 10-01 on a CPU macOS was throttling at 1% battery). The fourth, on
  10-02 at 23:55, named pass 6 on a box at loadavg 95 that the operator was
  already draining for a reboot; the pass that then hung was the NEXT one,
  and it never reached the budget.
- Awake passes that did not come round on their own: **0 of 697**. The one
  pass in the record with no summary line was ended by a reboot.
- Every firing with a load reading beside it sat over `load_guard: 60`
  (113 and 95); the other two were a throttled battery and a refill storm
  re-offering 109 ready beads to every free seat.

## Decision

**D1 — Nothing acts.** Both watchdog readings and the suspend witness stay
reports. No pass is ended by the loop, no loop ends itself, and no hire is
refused on pass age. An acting threshold needs a target, and every candidate
in the record is one of three things: a sleep, where acting is a false
positive by construction; a box over the load line, where the acting brake
already exists and is denominated in the cause (D2); or a pass still inside
its budget, which no threshold reaches. Zero awake unresolved stalls in 697
passes is the whole argument, and the number that reopens it is one (D4).

**D2 — The brake is the load guard, not pass age.** Option 3 ("refuse to hire
while a pass is over budget") would have fired on nothing the load guard was
not already holding: every over-budget awake pass with a reading sat over
the line, and the guard clock refuses launches off the pass on every tick it
stands. A brake is a floor only under the quantity it measures (standing
order, from ranger-base-bp224): pass age measures fork latency times stage
count, and `load_guard:` measures the thing that sets it. A second brake
under the symptom adds false positives where the first has nothing to say
and nothing where it does.

**D3 — The operator's stop stays stage-bounded.** sqxo1's seven boundaries
honour TERM within one stage; a stage is its children times their
deadlines — `BdTimeout` 3m per beads dir (2 configured), `GitTimeout` 2m
per repo (3 configured), `HerdrControlTimeout` 2m plus 1m grace — so the
worst residual is ASSUMED about 6m, and no stage has been measured at its
ceiling. Option 4 (a context through every bd and git call site) buys
seconds off a bound that was reached once, for at most 10m18s, on a box at
loadavg 95 that was being rebooted. Not built; it is the second build when
D4 trips.

**D4 — The reopen condition, and the instrument.** This page is wrong the
first time the log shows a pass-stall line followed by no pass header for
two budgets, with no `SUSPENDED` line and no `over load_guard` line between
them — an awake loop under the load line that is not coming round. One such
observation reopens this ADR; the first build is then option 2 as priced
below, and option 4 after it, never option 5. The recipe:

```
grep -n 'watchdog: no pass has completed\|── pass \|was SUSPENDED\|over load_guard' \
    ~/.config/posse/state/dispatch-watch.log
python3 scripts/pass-census.py        # the distribution; re-run it, the corpus grows
```

**D5 — The report's budget and its once-per-stall rule stand.** At 3m/3m
the 12m budget sits at the tail of the awake header-to-header distribution
(p99 11m02s, max 13m37s), so the witness names about four healthy passes in
650 and each resolved within 100 seconds. That is the right price for one
log line and the wrong price for an act — D1 in a single number. Noted, not
changed: under the autostart defaults (5m base, 8x cap) the same formula is
90m, and the reading carries the backed-off wait inside it; a budget
denominated in the pass alone, from its header, would be tighter. ASSUMED,
no incident behind it, no bead.

## Consequences

- On a sleeping laptop the operator's cost is naming, which suspend.go now
  does; on a loaded box it is the load, which the guard clock names every
  tick. Neither is a cost this page could remove by acting, and the two
  incidents that prompted the ask were found by `loop-mute` and by the
  operator at the keyboard, not by a pass clock.
- The 09-20 afternoon reads as four 33–60m passes in the log and will keep
  reading that way; the same afternoon today would carry four suspend lines.
- No code beads are cut and nothing goes to dinesh. This ADR constrains by
  absence, and a pin that scanned for the absence of a kill would name no
  process (ADR 0006 §7); the census script is the instrument, not a pin.
- `scripts/pass-census.py` is committed beside this page because the box's
  other records rotate (pmset from 09-26, plan-usage from 09-28, the unified
  log patchy by category) and the loop's own log is the longest record on
  the box.

## Alternatives rejected

**Option 2 — escalate the report** (re-log every budget; a `pass-stall` key
on G7 built from a `LastPass` field in `dispatch-watch.pid`, read by
govern.go the way `loop-mute` is). Price ASSUMED half a session: one field
written per pass, one governance row, one pin. Rejected because three of the
four firings on record would have raised URGENT to the coordinator for a pass
that completed within 100 seconds, and the once-per-stall rule in watchdog.go
was right on all four. It is the first build when D4 trips, because it is
the only option whose false positive costs a line and not a loop.

**Option 3 — refuse to hire while the pass is over budget.** Price small: one
predicate beside the last `stopHere` gate. Rejected under D2: zero awake
over-budget passes under the load line in the record.

**Option 4 — a cancellable pass.** Price ASSUMED several sessions, mechanical:
a context parameter through every bd and git call site in the eight stages,
each one a chance to make a one-shot Run interruptible by mistake
(`stopping()`'s nil rule). Buys the stop in seconds instead of within one
stage. Rejected for now under D3; second build when D4 trips.

**Option 5 — the loop ends itself and autostart replaces it.** Smallest
code, rejected hardest. `plugin/autostart.sh` arms only at herdr startup
(`[[startup]]` in herdr-plugin.toml), so nothing replaces a loop that ended
itself until the next herdr start; G7 then reads `loop-dead` URGENT, and the
shop goes from a slow loop to no loop plus an alarm. On a sleeping box the
replacement does not run; on a loaded box it forks into the same load. The
carried legs lose nothing — `stopClaim` keeps every claim and ADR 0028 §4
survives — but nothing is gained either.

**Re-derive the pass budget from the census** (raise it to clear 13m37s).
Rejected: it is a report, its false-positive price is one line and 100
seconds, and raising it delays the one true line by the same margin.

**The clever one** — act on the stall only when the suspend witness and the
load guard are both silent, so the detector is "awake and under the line and
late". Rejected because that predicate is D4 written as code, and in 697
passes it has never been true: a mechanism with no observed input is a path
with no caller, and the first time the predicate is true the right response
is to read the log, not to have killed the loop before anyone could.
