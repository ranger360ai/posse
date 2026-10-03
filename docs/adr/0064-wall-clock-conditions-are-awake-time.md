# ADR 0064 — A wall-clock shop condition measures awake time: the process that witnessed a suspend subtracts it

*Status: accepted 2026-10-03 · owner: architect · source bead
ranger-base-brt3x (from ranger-base-sqxo1) · sits under ADR 0029 (the
condition view and its scopes) and ADR 0027 (delivery) · re-rules the
"one interval wide and transient" clause of `WatchLogStaleAfter`'s doc
(watchlog.go) · amends nothing in ADR 0063, which decided what a loop may
DO; this page decides what the shop may SAY*

## Context

Two G-rows are built from wall-clock ages of files: G7's `loop-mute`
(watch-log mtime past `WatchLogStaleAfter`) and G5's `guard-blind:Nh` (plan
snapshot past `plan_guard_blind_max`). Both thresholds are derived from
**awake** guarantees — `WatchLogStaleAfter` is the watchdog's budget, and the
watchdog's clock is Go monotonic, which on darwin is `mach_absolute_time` and
does not advance across a suspend (suspend.go's head, MEASURED 2026-10-03);
`plan_guard_blind_max` bounds how long the shop may *hire* without a reading,
and a suspended box hires nothing. The third age-based row, G4
`guard-stuck`, is already awake-denominated by construction: its streak clock
is an in-process `time.Time`, so `now.Sub(GuardTrippedSince)` is a monotonic
difference. G5 and G7 compare an awake threshold to a wall reading; G4 does
not. That is the defect, in one sentence.

On 2026-10-01 the box slept 19807s on a 1% battery and the pulse fired one
second after the wake: `pulse: guard-blind:5h; loop-mute; question:… →
prompted monica`. Both keys were literally true and neither was the cause; a
P1 was filed as a six-hour hang and the evening went to triage
(`docs/notes.d/ranger-base-sqxo1.md`). sqxo1 then landed the suspend witness
(suspend.go): the watch loop measures wall-minus-monotonic every 30s and
writes one line naming the suspend. That fixes the log and cannot reach the
prompt, because the prompt carries keys (pulse.go `pulsePromptText`) and the
keys are built in govern.go from files.

**The record, MEASURED 2026-10-03 on this box:**

| reading | value |
|---|---|
| pulses that prompted, both watch-log generations (since the loop began writing its own log, ranger-base-n00wn 2026-09-03) | 401 |
| of those carrying `loop-mute` | 1 (the 10-01 wake) |
| of those carrying a bare `guard-blind:Nh` (no class token — a dead socket, which is what a wake's first read gets) | 1 (the same line) |
| `guard-blind:Nh:429` / `:unreadable` key occurrences across the record | 594 / 201, every one a real blind meter (one 88h `unreadable` episode, one long 429 storm) |
| sleeps in `pmset -g log` (09-26 → 10-02, the log's whole retention) | 1184 |
| longer than `plan_guard_blind_max` (10m default) | 326, almost all 10–17m Maintenance/standby cycles |
| longer than `WatchLogStaleAfter` at this box's arm (5m base, 40m cap → 85m) | 1 (the 10-01 Low Power Sleep) |

So the false positive has fired once in 401 prompts, and cost a P1 and an
evening; the predecessor reading (ranger-base-wj7e9, 2026-09-03) cost the same
before `loop-mute` existed. The exposure is structural, not rare in kind: every
wake after a long sleep races the pulse's 2m timer against the witness's 30s
one (both resume with whatever was left, so the pulse lands first about one
wake in eight, ASSUMED from uniform phases), and the first plan read after a
wake fails whenever the network is slower back than the pulse.

```
grep -c 'pulse: .*→ prompted' ~/.config/posse/state/dispatch-watch.log*
grep -h 'pulse: .*loop-mute.*→ prompted' ~/.config/posse/state/dispatch-watch.log*
grep -ho 'guard-blind:[0-9]*h[:0-9a-z-]*' ~/.config/posse/state/dispatch-watch.log* | sed -E 's/[0-9]+h/Nh/' | sort | uniq -c
pmset -g log | grep 'Entering Sleep' | grep -oE '[0-9]+ secs' | awk '$1>600'   # re-run; pmset rotates weekly
```

## Decision

**D1 — The contract: a wall-clock condition is a statement about awake
time.** `loop-mute` and `guard-blind` keep their keys, their urgency, their
thresholds and their rows. What changes is the reading they are compared to:
a process that can prove the box was suspended for part of the window
subtracts that part before comparing. Same key, same fingerprint, no new
G-row, no new token, nothing for the cockpit switch (cockpit.go `govSegment`)
or the pulse prompt to learn. A row fires exactly when the shop was *awake*
and quiet past its budget — which is what both thresholds always meant, and
what G4 already does. The "this instrument is blind" signal is **deferred,
never lost**: a real outage that straddles a sleep fires once awake quiet
accumulates past the threshold, and the sleep cannot hide it beyond that.

**D2 — The plumbing: the watch process only, through `GovInputs`.** One new
field, `Suspended func(since time.Time) time.Duration` — "how much of the
window from `since` to now this process witnessed the box suspended" — set by
the pulse's `govInputs` from the witness and nil everywhere else, exactly the
shape `GuardTrippedSince` already has (govern.go: present in the pulse's
inputs, deliberately absent from `StatusInputs`). `watchLogRow` and
`blindPast` subtract it from the age before the threshold compare; the bucket
in `guard-blind:Nh` is the awake age. The witness keeps a small ledger of
`(back, slept)` pairs (suspend.go) and answers the field by summing each
pair's overlap with the window. A detail string that still fires after a
non-zero subtraction says so — the line names the awake age and the suspended
span it discounted — because the next thing a reader does with it is check
`pmset -g log`.

**D3 — The pulse reads the pair itself, before it reads anything else.**
After a wake the 30s witness ticker and the 2m pulse ticker resume with
whatever was left on each, so the witness is not guaranteed to have seen the
gap when the pulse computes its set. `pulseOnce` therefore calls the same
`suspendTick` first; it is under `mu`, re-seeds the pair, and whichever
goroutine sees a gap records it, so two callers cannot double-count. The
SUSPENDED line is printed by whichever saw it. Note that the write also
freshens the log's mtime, which would clear `loop-mute` on its own — that is
incidental, not the mechanism, and the subtraction is what the pin holds.

**D4 — `posse status` and the cockpit keep the wall reading, and the cost is
named.** They are separate processes with no witness, and the ask was for a
store. Not built: their exposure is `loop-mute` for at most one witness tick
after a wake (the witness's own line moves the mtime at `SuspendTick`, 30s,
and the pulse's read under D3 shortens it) and `guard-blind` until the first
successful plan read after the wake. Both are views; neither prompts anyone
and neither persists a verdict. A store — one `suspended:` line per event in
`dispatch-watch.pid`, the carrier ADR 0063 already named for watch-process
facts, read by govern.go beside the lock — is the first build if an operator
ever files a bead on a status or cockpit reading taken inside that window.
Zero such beads in the record.

**D5 — `WatchLogStaleAfter`'s clause is re-ruled.** "The window is one
interval wide and transient — a cost worth naming, not one worth a second
clock" was the right call before the clock existed. The clock exists
(sqxo1), the window was tested and cost a P1, and the price of using the
clock here is one field and one subtraction. The doc comment is amended to
point here.

**D6 — A backward wall step is reported, not corrected.** The same pair
reading with the sign flipped: every wall age then reads *fresher* than it is
by the step, so `loop-mute` and `guard-blind` go quiet over a real outage for
that long. The witness stops being silent on it and prints one line naming
the step (over the same `SuspendFloor`, which is sized for a clock being SET),
and nothing subtracts or adds it. Two reasons, in order of weight: zero
backward steps in the record (ntp slews under the floor; a 30s+ step back is
an operator setting the clock); and the correction is not computable. A
forward gap — suspend or step — leaves a hole in wall time that no timestamp
falls into, so overlap-with-window is exact. A backward step makes two frames
overlap for `S` of wall time, and a timestamp inside the overlap cannot be
told pre-step from post-step. The hazard is bounded by `S` and self-heals as
the overlap passes. The first backward line in the log reopens this clause;
the line is what makes it countable.

## Consequences

- The 10-01 wake, replayed: the pulse reads the pair, records 5h30m07s,
  computes a watch-log awake age of seconds and a plan awake-blind of one
  second, and delivers nothing but `question:ranger-base-6wqe` — which is what
  was actually owed.
- No key moves, so `state/pulse.yaml` fingerprints written by the older
  binary still compare; no cockpit, status or prompt text changes.
- The 09-20 clamshell afternoon (ADR 0063) would have read the same either
  way: no `loop-mute` reached a pulse that day, because the loop's own
  tickers kept the log fresh through the dark wakes.
- The watch-status line's "written N ago" (watchlog.go `WatchLogNote`) stays
  wall: it is a one-shot process's view, and the SUSPENDED line sits in the
  log it points at.
- Price, ASSUMED one to two dinesh sessions: suspend.go (ledger, the window
  sum, the backward line), pulse.go (one call, one field), govern.go (field,
  two subtractions, two detail strings), watchlog.go (one doc clause), pins
  in suspend_qa_test.go and govern_test.go. No new config key, state file,
  process, G-row or token.

## Alternatives rejected

**Nothing** (the current state; option 4 on the bead). Priced first because
the standing order says so. MEASURED cost: one false URGENT in 401 prompts,
a P1 and an evening each time it has happened, plus every operator's next
read of `loop-mute` being "or the lid was closed". The log line sqxo1 added
is read after the prompt, by someone who was already woken. Rejected on the
two incidents, not on the rate.

**Qualify the detail** (option 1). Cheapest, and reaches nothing: the pulse
delivers keys. It also qualifies a row that, under D1, should not have fired
— a true measurement of the wrong quantity.

**A key of its own** — `loop-mute:suspend`, `guard-blind:suspend:5h`
(option 2). Reaches the prompt and moves the fingerprint, so monica is woken
URGENT to be told nothing is wrong. The shape `guard-blind:10h:429` forks the
*cause* of a real condition; this would fork its *absence*. And the cockpit
switch needs an arm for a row whose content is "ignore this row".

**Suppress and replace with `box-suspended:5h30m`** (option 3). A lid
closing is not a shop condition and nothing is owed on it; at 1184 sleeps a
week the row would be the shop's noisiest. The one case argued for it — the
battery died — is a box fact, and G10 (live-box checks) is where box facts
live if anyone wants one. The signal it was feared to lose is not lost under
D1 (deferred, see there).

**A wake grace** — suppress every wall-clock row for one interval after a
witnessed wake. Same code size as D1 and strictly worse: a blind spot by
construction, where the subtraction has none, and a real outage inside the
grace is hidden rather than deferred.

**A store for every process** now (D4's first build). A file format, a
writer, a reader, a staleness class for a record left by a dead loop, and
pins, to remove a 30-second cockpit flicker nobody has filed. Two of the
racing signals in the standing order with no bead demanding them.

**Order the tickers** — make the witness fire before the pulse after a wake.
Timers resume with their remainders; there is no ordering to arrange short of
D3, which is D3.

**Correct backward steps symmetrically.** Rejected under D6: not computable
in the overlap, and zero observations. Reporting them costs one line and
makes the first one a measurement.
