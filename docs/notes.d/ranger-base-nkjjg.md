## A `no-db: true` store discards the defer date, so a parked question kept paging (ranger-base-nkjjg)

The operator ruled on hcn-4, hcn-7, hcn-11 and hcn-12 on 2026-10-03 and
monica ran `bd defer <id> --until 2026-10-09|2026-10-16` on each. `bd show`
printed ❄ DEFERRED and `bd ready` dropped all four. The next pulse still read
`question:hcn-11; question:hcn-12; question:hcn-4; question:hcn-7` and
prompted her, every tick.

Not a reading error and not a stale cache: **the date the operator typed does
not exist in that store.** ranger-base-5aln's fix and ranger-base-03ada's
sharpening both read `defer_until` and deliberately never the status string,
on a measurement taken against the shop's SQLite queue. HCN's store is the
other class — `no-db: true`, JSONL only — and on that class bd 0.50.3 writes
no date at all.

### MEASURED 2026-10-03, bd 0.50.3, `no-db: true` JSONL store

A hand-written scratch store (`issue-prefix: "scr"`, `no-db: true`, two open
`-l question` rows in `.beads/issues.jsonl`), one binary, three spellings:

| command | printed | `status` after | `defer_until` after |
|---|---|---|---|
| `bd defer scr-1 --until 2026-10-16` | `* Deferred scr-1` | `deferred` | **absent** |
| `bd defer scr-2 --until=tomorrow` | `* Deferred scr-2` | `deferred` | **absent** |
| `bd update scr-2 --defer 2026-10-16` | `✓ Updated issue: scr-2` | `deferred` | **absent** |

"Absent" is absent in four places, checked separately: the `.beads/issues.jsonl`
record, `bd list --json`, `bd show --json`, and `bd show`'s human frame, which
prints `❄ scr-1 · q one   [● P0 · DEFERRED]` with no date line under it. The
`--until` value is accepted, reported as success, and dropped.

The shop's own `~/src/hcn` reads the same way — hcn-4, hcn-7, hcn-11, hcn-12
and hcn-15 are all `status: deferred` with no `defer_until` key — and the
SQLite class does not: on the queue db the same day, three of four deferred
beads carry a date (`ranger-base-73tlu` 2026-09-07, `ranger-base-6wqe`
2026-10-17, `rangerhq-bnvk` 2026-09-26).

So neither field alone is the signal. **`defer_until` is the only signal on
one store class and the status is the only signal on the other**, and the two
classes sit side by side in one `beads:` list.

### The rule

`deferredNow` (internal/posse/beads.go) is the one reader, moved there from
interrupted.go to sit beside the field it reads:

> **The date decides when there is one; otherwise the status does.**

- a date, whatever the status — ranger-base-03ada's shape, a deferred bead
  reading back `status: "open"` with a date. A date in the **past** is the
  park expired with nobody revisiting it, which is unanswered again, and the
  status is still not consulted;
- no date and `status: "deferred"` — the shape above. `bd defer <id>` with no
  `--until` is bd's own documented status-based defer ("deliberately set
  aside"), so an indefinite park is still an answer: "not now";
- no date and any other status — silence, which is what the aging-question row
  exists to report.

Its callers are governance G3 (`govern.go`, and therefore the pulse key and
the cockpit panel, which are one computation) and the claimed half of the
dispatch queue (`interrupted.go`).

### The G3 row, not a G3 row that says "deferred"

The bead's description asks for both "no G3 row" (its Pin) and a row reading
`deferred to <date>` (its fix shape). Those are not the same change and the
Pin is the one implemented, because **a row is a condition**: the pulse
fingerprints `GovSet.Keys()` (pulse.go), so a G3 row that said "deferred to
2026-10-09" would page the coordinator with it every tick — the complaint
itself. `posse status` has no second question listing for such a row to live
in. A parked question is reported by `bd list`, which shows it ❄ with
whatever date survived.

### The dispatch skip line is a guard, not a reproduction

The bead also reports `– hcn-4 for the operator (question) — not dispatched`
per parked bead per pass. That line cannot be reached from a real store
today: dispatch's queue is `bd ready` plus `interruptedRuns`, and **`bd ready`
excludes a parked row on both store classes** — MEASURED the same day, the
no-db scratch store above answers `ready --json --limit 0` with its one open
row and lists the deferred one under `bd blocked`, which `Bd.Ready` subtracts
besides. The reported lines predate the defers landing.

The check is in the loop anyway, and the judgment is `OpenLabeledAny`'s about
closed rows: what this surface promises is this surface's to enforce, not
bd's. `TestDispatchQuestionSkipLineOmitsParkedBeads` reaches it through the
fake, which serves `fake-ready.json` whole.

### Shown able to fail

Three mutations of `deferredNow`, each restored:

| mutation | reds |
|---|---|
| `return is.DeferUntil != nil && is.DeferUntil.After(now)` (the predicate before this change) | exactly the three new arms — `TestGovG3StatusDeferredWithNoDateIsNotACondition`, `TestGovPulseLineOmitsParkedQuestions`, `TestDispatchQuestionSkipLineOmitsParkedBeads`; every dated arm stays green |
| `return true` (park everything) | 9, including both controls — `TestGovG3NoDeferUntilStillNags`, `TestGovG3DeferredUntilPastStillNags`, and the `q-e` / `q-live` / `q-expired` arms of the two new tests |
| the `&& !deferredNow(...)` dropped from the fire loop's question skip | `TestDispatchQuestionSkipLineOmitsParkedBeads` alone |

`TestGovG3NoDeferUntilStillNags` loses `"deferred"` from its status table and
gains `"in_progress"` and `"blocked"`: a dateless `deferred` is now the park,
and the honesty arm it was written to be is carried by the statuses that are
not one, plus the unparked control in each new test.
