# ranger-base-nqtz5 — two G19 pins, and why one of them had to read source

Both findings came from a verify of ranger-base-p51zu's close (G19, a
dispatched seat that never got its work prompt). Neither is a defect in that
fix; both are holes in what witnesses it.

Environment for every reading: linux 7.2.15-1-omarchy, go1.27.3, `main` at
814776ed, in this bead's own worktree. Each mutant run alone, with the file
restored from a checksummed golden copy before the next.

## F1 · the hand-prompt stamp write

`hb.MarkPrompted(name, time.Now())` in `cmd/posse` `case "prompt"` is the
half of ranger-base-p51zu's fix that G19's own Detail prints as the remedy
(`posse prompt <session>`). `MarkPrompted` has exactly two call sites: the
one in `dispatch.go`, pinned by `TestRunRecordPersistsBeadAndPrompted`, and
this one, pinned by nothing — and the four G19 pins the same close added are
all in `internal/posse`, which cannot reach package `main`.

MEASURED 2026-11-22, with that one line deleted: `go vet ./cmd/posse` clean,
`go test -count=1 ./cmd/posse` ok in 103s, `make treepins-check` ok in 89.3s.

Two pins, at the two ends of the chain, because neither end can reach the
other's fixture:

- `cmd/posse/handpromptstamp_qa_test.go` drives the `prompt` verb against
  promptsubmit_qa_test.go's fake herdr, over the one record state no launch
  leaves behind on purpose — `bead:` set, `launched:` old, `prompted:`
  absent, which is G19's own row — and reads the record back off disk. It is
  over the RECORD and not over the call, so it holds the fact whichever
  writer puts it there.
- `internal/posse/unpromptedpredicate_qa_test.go`'s
  `TestMarkPromptedSatisfiesBothReadersOfTheStamp` holds that a stamp of that
  shape stands both readers down: `unpromptedSeat`/G19 over the record, and
  `promptedRecently` over a SECOND `Dispatcher`, so the answer cannot come
  from the first one's in-process map (the cross-process half is the whole
  reason the stamp is persisted, ADR 0011 §3).

Mutants, against the first pin's two arms:

| mutant | result |
| --- | --- |
| the `MarkPrompted` line deleted | RED — `left the record with no prompted: stamp` |
| the same line hoisted ABOVE the `Prompt` call | RED — the wrong arm: `a prompt herdr refused left prompted: …` |

The second mutant is why the wrong arm is there. A failed prompt is not a
prompt, and a record claiming one was delivered is worse than one claiming
nothing: `promptedRecently` would hold the next pass off re-prompting a seat
that never got its work, which is the repair G19 is counting on.

## F2 · one exclusion rule, two copies, two kinds of witness

The Foreign / Crew / beadless exclusion is written twice — in the G19 walk
(`govern.go`) and in the predicate (`unpromptedSeat`) — and the one pin
written for it, `TestGovG19CrewAndBeadlessSeatsRaiseNothing`, goes through
`ShopCheck`, so it grades the composite and neither copy alone. Measured on
the originating verify: deleting either copy left the suite green, while
`unpromptedGrace` 15m → 1ns reddened
`TestGovG19SeatInsideTheGraceIsNotACondition`, so the file was not simply
unread.

**The bead asked for one pin and one pin cannot do it.** Its prescription was
"a pin over `unpromptedSeat` directly … so that deleting either copy reds
something". A direct predicate pin reds the predicate's copy and leaves the
walk's exactly as green as it was measured: with the walk's guard deleted,
`readMeta` is called for every listed session, the predicate still excludes
all three populations, and every output byte is identical. Its whole effect is
the cost its own comment states (ONE FILE READ PER CANDIDATE), so no
behavioural pin anywhere can red it. A guard whose deletion is invisible to
every run is witnessed by reading the walk or by nothing — the same key as
this package's other source censuses under `$(QA_PKGCENSUS_PINS)`.

So, two pins in `internal/posse/unpromptedpredicate_qa_test.go`:

- `TestGovG19PredicateAloneExcludesTheThreePopulations` — the predicate
  called directly, with a foreign, a crew and a beadless row, and the
  POSITIVE row first as the discriminator: without it the three arms are
  three ways of printing nothing, which a predicate returning false for
  everything would satisfy. This is the test for `unpromptedSeat`'s own
  docstring claim that it "is the predicate alone, which is what makes it the
  same answer twice".
- `TestQAG19WalkTestsTheFreeClausesBeforeItOpensARecord` — the walk read as
  source: every clause the session LISTING already carries is tested, and
  skipped on, before a record is opened. The clauses are named as FIELDS and
  the guard as "names the field, then `continue`s, before the read", so one
  `if` with three `||`, three separate ifs and a switch all read as guarded,
  and the pin reds for the two ways the guard stops guarding — deleted, and
  hoisted under a read that moved above it. It carries its own fixtures,
  including the negative case of a block that names the fields and never
  skips.

| mutant | result |
| --- | --- |
| the predicate's `case s.Foreign, s.Crew:` and `case s.Bead == "":` arms deleted (14 lines) | RED — all three arms of the direct pin |
| `govern.go`'s `if s.Foreign \|\| s.Crew \|\| s.Bead == "" { continue }` deleted (3 lines) | RED — the source pin, and only it |
| control: `unpromptedGrace` 15m → 1ns | RED — `TestGovG19SeatInsideTheGraceIsNotACondition`, as before |

Neither copy was wrong and neither went. The walk's is a cost guard with a
stated reason; the predicate's exclusions are what make it safe for a second
caller. Refactoring them into one shared predicate was the other candidate
and was not taken: the finding says neither copy should go, and a production
refactor is a different deliverable from the pins it asked for.

## Two gates this work met on the way, worth knowing about

`make treepins-check` reds for a new test file twice over, from
`cmd/testparallel`, and neither reading is about this bead's subject:

1. the direct predicate pin took `t.Parallel()` and read `govNow`, a written
   package var whose only cleared reader is `govIn` — so `parallelOK` would
   have needed a new line naming this test. It takes its own clock instead,
   which it should anyway: `unpromptedSeat` reads nothing and takes `now` as
   a parameter, so a test of the predicate alone has no reason to reach a
   package var at all.
2. `TestMarkPromptedSatisfiesBothReadersOfTheStamp` was eligible for
   `t.Parallel()` and did not carry it, which the same tool reds for in the
   other direction.

No new Makefile door is owed. The source pin reads ONE named file in its own
package directory, which is outside both halves of the tree-wide
derivation — no `qibRepoRoot` climb and no directory enumeration — and
`make treepins-check` is green over it.
