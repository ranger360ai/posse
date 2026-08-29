## A refill that does not say it is one (ranger-base-59jd)

Two cosmetic defects from the first live refill, 2026-08-28 ~09:15. Both
are about what the shop SAYS, and both were the kind that costs an
operator's morning rather than a bead.

**The refill's enumeration was unowned.** ADR 0028 §1 as amended makes the
settle the level-triggered tick: every settle re-runs the *whole* fire path,
and the fire path has always enumerated per bead. Under a `--persona gwart`
watch that meant a wall of `– <bead> … lane busy` lines followed by
`– 131 ready bead(s) outside gwart's lane — skipped by --persona`, reprinted
at every settle, attributed to nothing. Every line was true. None of them
said who was speaking, and repeated on a loop they read as a rogue
persona-filtered process holding the watch — which is what the operator read
them as, and went to an alarm footing over.

The fix is attribution plus arithmetic, not silence. A refill prints
`↻ refill for settled seat <seat> (<bead> settled) — re-offering N ready
bead(s) to every free seat` before it enumerates, and one
`↻ refill for settled seat <seat>: N launched, M skipped (<counts by
reason>)` after it. The per-bead `– ` lines are counted by a short reason
instead of printed, because under a rolling `Run` they are the *same lines
every time* — the queue is the same queue. Nothing else is quieted:
launches, `✗` errors and every other report print exactly as they did, and a
fire pass that is not a refill (the head of a `Run`, every one-shot
`posse dispatch`, every `--dry-run`) still enumerates per bead, unchanged.
`internal/rhq/refillreport.go`; the control arm is asserted, so the
summarising cannot be the fixture's doing.

The load-guard refusal on the same path now names the seat too
(`◷ refill for settled seat <seat> skipped: <why>`). It was the one refill
line that already announced itself and it still could not be traced to a
seat.

**The arm stamp was a constant.** ADR 0028 §5 observable 1 shipped first and
alone, so `seatidle.go` stamped every line and every summary
`no refill has shipped, this is the control arm`. True the day it was
written; a lie from the moment §1 went live; unnoticed because nothing was
keyed on anything — the string could not become false. An observable whose
arm label cannot change is an observable with one arm, and the before/after
comparison it exists to support would have read as all-before.

The arm is now read off the call path that closed the window: `d.refilling`
is set only for the duration of a refill's own `fireLoop`, so
`SeatRefill.Rolling` means *this launch was made by a refill* and not *this
build has refills in it*. Per-window, the line reads
`[ADR 0028 §5 obs.1 rolling]` or `[… baseline]`; per report, the pass line
counts them — `no window here was closed by a refill — control arm`, or
`K of N window(s) closed by a refill — treatment arm`.

Per-window and not per-process on purpose: a rolling `Run`'s FIRST launch
into each seat still comes from the head of its pass, and the window it
closes opened before any refill existed. That is a baseline window and is
stamped as one. Keying the arm on `d.Refill` — the flag — would have folded
those into the treatment figure, which is "before" data inside the "after"
number, in the direction that flatters the change. This is the same trap
`ranger-base-293j` names twice for the same instrument, in the same
direction, for the third time.

**The general shape, for the next instrument.** A measurement's own metadata
is measurement: if the arm, the units, or the denominator are written as a
literal, they are pinned to the day the code was written and nothing will
red when they stop being true. Derive them from the call path, and give the
test both arms — an assertion that only ever sees one is a sticker.
