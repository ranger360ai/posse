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

## MEASURED 2026-11-15, D1 host (Linux 7.2.15-1-omarchy, go1.27.3, 16 cores)

Warm build cache, `-count=1`. Load and slots are given with every reading
because this arm's wall moves with them and nothing else about it does.

The four, filtered, each the command the bead recorded — at a one-minute load
of 7.74 with no suite slot held:

| test(s) | before | after |
| --- | --- | --- |
| arm 3's three launchlock tests | FAIL 108.259s | **ok 200.513s** |
| `TestDispatchRewaitServerGoneKeepsTheClaim` | FAIL 43.732s | **ok 59.187s** |

Then the whole arm, end to end through `make test-race`, at a load swinging
4.5-22 with BOTH suite slots held by two sibling seats throughout:

| arm | tests | result | wall |
| --- | --- | --- | --- |
| 1 (default build) | 64 | `ok` | 366s |
| 2 (`-tags posse_arm2`) | 106 | `ok` | 302s |
| 3 (`-tags posse_arm3`) | 112 | `ok` | 438s |
| **total** | **226** | **`ok`, `DATA RACE` 0 in all three** | **1106s — 18.4 min** |

**THE GREEN ARM COSTS MORE THAN THE RED ONE DID**, and that is the shape of
the defect rather than a regression: 1106s against 1011s on 2026-11-14. A
budget that fires ends its test early, so every one of these four was *cheaper*
while it was broken — 152s of the 95s difference is the four tests now running
to completion, and the rest is the box. Read the two totals as the same arm,
and neither against the other's load.

The candidate set reads 226 tests where 2026-11-14 read 227: moving the
backstop's arithmetic pin out of `watch_test.go` (a candidate file) and into
`backstop_test.go` (not one) took it out of this arm. It is a table of
`backstopBefore` inputs with no concurrency in it, and `raceArmFloor` is 90, so
nothing was lost and nothing reds.

Also run: `make fmt-check`, `make notes-check`, `go vet` in all three arms,
`make tree-check`, `make test`, and `go run ./cmd/checkorphans` after the
backgrounded arm (clean).
