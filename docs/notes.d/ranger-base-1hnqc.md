# ranger-base-1hnqc — four `-race` reds with `DATA RACE` 0: a budget in wall seconds is a budget on a wall the build stretches

`scripts/test-race.sh` went from eight candidate files to twenty-three on
2026-11-14 (ranger-base-87p6i). The arm it then ran was red: **four tests
failed under `-race`, passed without it, and reported `DATA RACE` 0**, at the
same timings run alone on a quiet box as under load. None of them was a race.
Each was a `select` whose other arm was a `time.After` CONSTANT, against an
apparatus whose wall `-race` stretches — and the cost of `-race` here is WAIT
and not CPU (13-14% CPU at ~13x the wall, measured in the script's own header),
so every such constant is a budget on how much the instrumented build has time
to do.

Three of the four were in `launchlock_test.go`, which was outside this arm
until 87p6i, so nobody had seen them. This is `ranger-base-0dt50`'s class one
bead later: its close said the class was general and the next one would come.

2026-11-15.

## The two mechanisms were already in the tree, and which one each test wanted

| mechanism | bead | what it is |
| --- | --- | --- |
| a deadline derived from `t.Deadline()` | ranger-base-khhnd | the backstop races `go test`'s own timeout, so take that timeout and subtract the room to print |
| a budget counted in CALLS | ranger-base-0dt50 | `fakeCallCost`, the cost of one fake-herdr round trip: ~250ms ordinary, 2s under `-race` |

The two are not interchangeable, and the line between them is **whether the
deadline is the OBSERVABLE or a backstop.**

Five launcher-lock waits were backstops — "it resumed once the lock was free",
with nothing asserted depending on the resume being fast — and they took the
derived deadline. The rewait test's 2s ceiling was not: it is what the test
PINS. A bigger constant there would weaken the assertion, because what the
test needs is that the ceiling is reached *and* that it leaves room for more
than one leg. That one took the call-counted unit.

## `backstop(t)`, and why it moved out of `watch_test.go`

khhnd wrote the derived deadline as `watchBackstop` beside its first two
callers. `backoffhold_qa_test.go` took it next; this bead took it for five
launcher-lock tests, where "watch" names nothing in the fixture at all. The
helper and its pinned arithmetic (`backstopBefore`) are now
`internal/posse/backstop_test.go` — no build tag, so every arm compiles it —
and the file's head comment states the class and, more usefully, **states what
is not in it.**

A deadline BOUNDED BY SOMETHING ELSE IN THE FIXTURE is an observable and stays
a constant. Three live examples, each with its bound now written at the site:

- `launchlock_test.go`'s "the gather held the launcher lock" (2s) is bounded by
  the fake's 5s prompt delay. Past that there is no gather left to have been
  unlocked, and the take would succeed against a finished pass.
- `launchlock_qa_test.go`'s "`--watch` holds the launcher lock across its
  backoff" (10s) is bounded by that fixture's own 30s backoff. Past it the loop
  starts its next pass and takes the lock legitimately, so a derived deadline
  would read a lock the watch loop was entitled to hold as a pass.
- The 150ms and 100ms arms in `launchlock_test.go` assert that a blocked
  launcher has NOT run yet. The deadline is the observation; widening it only
  slows the suite.

That distinction is the reusable part of this bead. The wrong reading — "a
`time.After` constant in a test is a bug" — would have quietly voided three
assertions.

## What landed

| file | change |
| --- | --- |
| `internal/posse/backstop_test.go` | new, shared: `backstop`, `backstopBefore` and their pin, moved out of `watch_test.go` with the class and its non-members documented |
| `internal/posse/watch_test.go` | the helper removed; its two callers call `backstop(t)` |
| `internal/posse/backoffhold_qa_test.go` | one caller, and a doc pointer |
| `internal/posse/launchlock_test.go` | six backstops derived; the one constant that is an observable annotated; the file head says which are which and why |
| `internal/posse/launchlock_qa_test.go` | three backstops derived; the backoff-bounded constant annotated |
| `internal/posse/dispatch_qa_test.go` | the rewait ceiling counted in `fakeCallCost`, with why it is the observable and why `LegFloor` is not scaled |
| `scripts/test-race.sh`, `Makefile` | the arm's status and price |

Not one assertion moved. Every test pins what it pinned before.
