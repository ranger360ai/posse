## An indefinite park went silent forever, and 14 days is why it no longer does (ranger-base-pm5zo)

ranger-base-nkjjg stopped governance G3 paging the coordinator about a
question bead parked with no `defer_until` — the only park one store class can
express, because it discards the date `bd defer --until` is given
(ranger-base-bwp7h). That fix was right and it opened a hole: **nothing then
made such a park loud again, ever.**

### Nothing else re-surfaces a park. MEASURED 2026-10-03, bd 0.50.3, both store classes

| store class | record | result |
|---|---|---|
| SQLite queue | `ranger-base-73tlu`, `defer_until` four weeks past | absent from `bd ready` |
| `no-db: true` JSONL | hand-written past date | absent from `bd ready` |

So bd re-surfaces nothing on either class, and the only thing that has ever
made a park loud again is governance's own reader — which keys on the date,
the one field the no-db class drops. A dateless park had no clock at all.

And that class keeps producing them: `bd defer <id> --until <date>` there sets
the status and discards the date, and `--until` cannot even MOVE a date that
is already present (ranger-base-bwp7h §2). Every future park in such a store
arrives dateless unless a hand edits the JSONL.

### The clock is `updated_at`, and it is the only honest one

MEASURED 2026-10-03/04, bd 0.50.3, darwin 25.4.0, on an isolated git-init'd
`no-db` scratch store (canary: `bd where` named it before anything was
deferred) and against the live store that prompted ranger-base-nkjjg:

| write | `defer_until` after | `updated_at` after |
|---|---|---|
| `bd defer <id> --until <date>` | **absent** | **moved to the park** |
| `bd defer <id>` (no `--until`) | absent | moved to the park |
| `bd defer <id> --until <date>` on an already-parked record | absent | moved again |
| `bd comments add <id> …` | absent | **unchanged** |
| `bd update <id> --priority` / `--assignee` | absent | moved |

Four things follow, and they are the whole design:

1. **There IS a clock.** The park stamps `updated_at` at the moment it
   happens, in both spellings, even as it drops the date.
2. **`created_at` is the WRONG clock, and not marginally.** On the live store
   the five dateless parks carry `updated_at` inside the 43 minutes the
   operator spent parking them, over `created_at` values spanning the five and
   a half hours before that. G3 has already required the bead to be older than
   `attn_question_age` by the time this predicate is asked, so keyed on
   `created_at` every one of those five would be loud the instant it was
   parked — ranger-base-nkjjg un-fixed.
3. **The row has an EXIT, which is what the row ranger-base-nkjjg rejected
   did not.** `bd defer <id>` again re-stamps `updated_at` and buys another
   horizon; on a store that keeps the date, the re-park leaves one and the
   dated arm takes over. Answering or closing clears it like any G3 row.
4. **The one imprecision, named.** An unrelated field write (`--priority`,
   `--assignee`) also moves `updated_at`, so it postpones the row. A comment
   does not. The drift is in the safe direction — the row can be LATE, never
   early, and what postpones it is somebody writing to this very bead. Early
   is the symptom ranger-base-nkjjg was filed on; late is this row, a
   fortnight later.

A **zero** `updated_at` is quiet, matching what the same loop does three lines
above with a zero `created_at`: a clock that cannot be read does not date a
park, and a row nobody can clear is the defect this row exists to avoid. The
compatibility abstain is named rather than left implicit — MEASURED, no record
is on the old path: every record in both stores carries `updated_at`, in the
JSONL and in `bd list --json`.

### N = 14 days, argued rather than inherited

The bead asked for the number to be picked on the record. MEASURED
2026-10-03/04, every park horizon either of the shop's two stores can account
for — the dated ones from their own `defer_until`, the dateless ones from the
argv recovered on ranger-base-bwp7h:

```
5d   6d   6d   6d   6d   13d   14d   30d        (n = 8)
```

The five in the middle are the no-db store's: the dates bd accepted and
discarded. **They are the population this row exists for**, because they are
the parks that arrive dateless, and **13 days is the longest of them**. So 14
days is the smallest whole-day horizon that fires strictly after every intent
in that population, and more than twice the shortest. It coins no new period
either — 14 days is already this shop's unit for a reading gone stale
(`RefreshExpiryWindow`) and for a close that stuck (the `closed-no-reopen`
metric).

The other half is ASSUMED and is policy, not measurement: that a deliberate
`bd defer <id>` with no `--until` — an indefinite park, whose intent is
unmeasurable by construction — wants reviewing fortnightly.
`attn_parked_age:` is the dial.

**REJECTED: `attn_question_age` (4h)**, the shortest honest floor, and the
shape the bead itself suggested — "a dateless park no quieter than a question
nobody deferred at all". MEASURED consequence: the five parked questions would
have gone loud again the same afternoon they were parked, ~4h later, and then
every `pulse_renag` (30m) until somebody closed them. That is
ranger-base-nkjjg's reported symptom with a four-hour delay. A park IS an
answer for as long as it was meant to last, and what the operator typed said
days.

### The row: its own key and its own sentence

`parked:<id>`, not `question:<id>`. `GovCondition.Key` is the whole identity
to a machine reader and must change when a different instance appears, and the
pulse fingerprints these: "no answer has ever been given" and "somebody parked
this and nobody came back" are different conditions with different remedies.

The sentence differs for a harder reason — the ordinary row is *unreadable*
here. It would say `open 32d unanswered` about a bead `bd list` shows ❄
DEFERRED with no date beside it, and the operator would read that as
ranger-base-nkjjg come back rather than as the finding it is. The row names
the park and the missing date, or it cannot be acted on.

Its age renders through `expiryAgo` rather than `BlindFor`, by `BlindFor`'s own
doc: that one renders a BLIND duration for a log line and the cockpit header,
a minutes-to-hours quantity with no day arm, so a 14-day park reads
`336h00m`. Same question, same units, one function — not a fourth renderer.

The URGENT/LANE grading is the row above's, computed once for both: a park
nobody revisited that is holding beads out of `bd ready` is the shop stopped
just as surely as a question nobody wrote, and a second copy of that judgement
is a second thing to keep in sync.

### The config key has a trap in it

`attn_parked_age:` goes through the same `attnAge` grammar as the other two
attention keys — which is Go's, and **Go has no day unit**. So the horizon's
own natural spelling, `14d`, is a TYPO: named on stderr, default standing,
like any other typo in these keys. A fortnight is `336h`.
`examples/config.yaml` ships `336h` with that said on the line, and
`TestGovG3ConfigurableParkedAge` carries an arm for it whose park age is on the
*other* side of the default from the typed value, so "the default stood" and
"the typed value was read" cannot be confused. Zero means every tick, which is
deliberately spellable: it is the behaviour before ranger-base-nkjjg.

### Shown able to fail

Six mutations, each restored, baseline asserted clean between each, over
`-run 'TestGovG3|TestGovPulseLine'`:

| mutation | arms red |
|---|---|
| `staleIndefinitePark` → `return false` (the predicate before this change) | 9 — every LOUD arm, and no quiet or dated arm |
| the clock → `is.Created` (the other available field) | 7 — every QUIET arm and the pulse-line control, the opposite set |
| `>= max` → `> max` | 1 — `exactly_the_horizon` |
| the `Updated.IsZero()` abstain dropped | 1 — `…WithNoClockStaysQuiet` |
| the `DeferUntil != nil` guard → `if false` | 1 — `…DatedParkIsOutOfTheParkHorizonsReach/future_date/deferred` |
| row key `parked:` → `question:` | 9 — every key assertion |

Two of these are worth reading twice. The first two mutate in **opposite
directions** and are caught by **disjoint** arm sets, which is the pair that
says the horizon is a window and not a one-sided skip. And the `DeferUntil`
guard reds only the `deferred` half of the dated arm: a dated park whose
status reads `open` is already excluded by the status test beside it, so that
guard is load-bearing for exactly one shape — a dated park whose status DID
move, which is the shape ranger-base-03ada measured. Overlapping guards, and
the mutation says which one covers what.

The fixture (`datelessPark`) pins `created_at` 99 days back on **every** arm,
including the arms that must stay quiet. That is the one condition under which
the two available clocks disagree, and a reader keyed on `created_at` passes
every arm if the fixture puts them close together.

### Not in scope, deliberately

The store's mode (ranger-base-bwp7h, blocked on an operator ruling) and
ranger-base-nkjjg's reader change itself. Dispatch's own use of `deferredNow`
is untouched: a stale indefinite park is still not handed to a persona as
interrupted work, because "should a forgotten park be dispatched" is a routing
question and this bead is the attention surface's.
