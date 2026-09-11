## The gather was gathering: `-race` moved the fixture's unit, not the dispatcher (ranger-base-0dt50)

ranger-base-7npp8 found three arm-2 tests failing under `-race` and green
without it — the only three reds in the arm `make test-race` now runs
(ranger-base-nhc23) — with `grep -c 'DATA RACE'` = 0 across every log. It named two
readings and said only measurement separates them:

- **(a) a fixture defect** — something in the test rig serialises the two
  `fire` legs and the dispatcher is gathering correctly;
- **(b) a real dispatch defect** — the gather fan-in does not overlap its
  prompt legs when every memory access is instrumented, in which case a
  gathered pass is a property of timing rather than of structure, and ADR
  0011's pass model is weaker than it reads.

It is **(a)**, and the measurement below excludes (b) rather than arguing
against it.

### What was measured

Every fake-herdr process was made to record its own start, and every call
its argv, in both builds. Same command otherwise.

MEASURED 2026-09-10, darwin/arm64 (MacBookPro18,3, 8 cores), macOS 26.4.1,
go1.26.5, HEAD e0c7a529, `-tags posse_arm2 -count=1`,
`TestDispatchParallelPass` run **alone** on a near-idle box:

| | `go test` | `go test -race` |
|---|---|---|
| fake-herdr calls the pass made | **50** | **50** |
| their argv sequence | identical | identical |
| median gap between consecutive calls | **8.3ms** | **1.02s** (~123x) |
| the fake's own work inside one call | 1-3ms | 1-3ms |
| `agent prompt` A -> `agent prompt` B | **0.44s** | **23.94s** |
| calls between those two prompts | 24 | 24 |
| verdict | PASS 1.15s | FAIL 66.09s |

The dispatcher issued **the same fifty calls in the same order** in both
builds. Nothing about the fan-in changed; what changed is that the fake is
the test binary re-exec'ing ITSELF (herdr_test.go), so under `-race` the
child is the instrumented binary too and pays instrumented startup and
teardown once per `herdr ...` call. The fake's own work is unaffected —
1-3ms in both — which is what pins the cost to the process and not to the
code under test.

That is the whole failure. `fire(A)` puts prompt A in flight in a goroutine
and returns; `fireLoop` then fires B, and B's launch is 24 fake-herdr calls.
At 8.3ms each that is 0.44s and prompt A is still in flight when B arrives —
gathered. At 1.02s each it is 23.94s, and `fakeBarrierWait` — 10 seconds —
had already released prompt A alone, at which point the fixture reports
"awaited serially rather than gathering" about a pass that did no such
thing.

### The same lie, one test over

`TestStatusAfterTimeoutRidesOutABlink` is the third failure and the same
shape. Its subject is a blink: the FIRST status poll must miss the agent and
a later one find it, so its `StatusGrace` has to buy more than one poll. A
poll is two fake-herdr calls (`workspace list`, `agent list`).

MEASURED, same box and build, instrumenting each poll:

| | `go test` | `go test -race` |
|---|---|---|
| one poll | 25.3ms, then 18.6ms | 2.05s |
| polls inside a 2s `StatusGrace` | ~70 (2 needed) | 0 — the first overran it by 45ms |
| result | PASS, `st="working"` | FAIL, `st=""` |

The control: with the grace raised to 30s and nothing else changed, the
`-race` run polls twice — `""` then `"working"` — and passes. The blink is
intact; there was simply never a second poll to see past it.

### The fix: budgets in the unit the work is done in

A budget spelled in seconds is really a budget on how many fake-herdr calls
the build has time for, and the two builds do not have the same answer: ten
seconds is ~1200 calls without `-race` and ~10 with it. So the unit is now
named and measured —

    fakecallcost_test.go       //go:build !race    fakeCallCost = 250ms
    fakecallcost_race_test.go  //go:build race     fakeCallCost = 2s

— and the two budgets that bound work measured in calls are written in it:

    fakeBarrierWait = 40 * fakeCallCost   (herdr_test.go; was 10s)
    d.StatusGrace   =  8 * fakeCallCost   (the blink test; was 2s)

Both constants are the measurement doubled — 250ms is the slowest single
call observed without `-race` (a `workspace create`, which the fake does
real work for) rather than the 8.3ms median, and 2s doubles the 1.02s — so
each carries its own load margin. The multipliers are chosen so **every
budget comes out at the number the tree already carried in the ordinary
build**: 40 x 250ms = 10s, 8 x 250ms = 2s. The change moves what `-race`
allows and nothing else.

`joinWait` (60s) was already generous enough for both builds and is left at
its value; only its comment was corrected, from "under -race that fork is
tens of seconds slow" to the measured ~1.02s per call. Tens of seconds is a
whole launch's worth of calls, not one fork's.

### The arm census had no room for a build mode

`make test` went red on the pair before anything else ran, in all three
arms:

    internal/posse/fakecallcost_test.go is built into arm 1 and this census
    does not put it there — the two readers of the partition have drifted

`internal/treepins/armtags_qa_test.go` keeps internal/posse's three-binary
partition total, and it decides which arm a file is in by reading its
`//go:build` line against a table of the three arm expressions. `!race` is
in none of them, so arm 1 read the file as one nothing runs and arm 6 read
the toolchain building a file the census placed nowhere — two wrong answers
about a file that is in fact in every arm.

The gap is real and not the pair's: the census assumed every constraint in
the package is an ARM, and `race` is a **build mode** — chosen by a flag,
orthogonal to the tags, and a file behind one is in all three arms in the
mode that builds it. `armModeAxis` now names that axis (both spellings,
mapped to whether the census's own default build compiles them) and
`armPlaces` answers "which arm, and does this mode build it" for all three
readers. It narrows nothing: a constraint that is neither an arm nor a
listed mode is still a file nothing runs, and a mode-axis file that carries
TESTS the census's mode does not build is now reported too — `make test`
passes no `-race`, so a test behind `race` would run nowhere, which is
rot class 1 in a legal tag. The pair carries a constant and no tests, which
is the shape that axis is for.

### Why this is the third time

Two earlier beads set a wall-clock margin here and had it eaten by a loaded
box, each time accusing a dispatcher that was gathering: rangerhq-g6lx (a
900ms ceiling on the whole pass) and rangerhq-3ig1 (500ms of allowed stagger
between the prompts). The barrier was the answer to both, and its own
comment claimed a gathered pass "reaches two-in-flight whatever the machine
is doing". It does — but only for as long as the barrier waits, and that
deadline was itself a stopwatch on the inter-fire interval. The rule the
three of them make:

> A fixture budget that bounds work done in forked child processes must be
> written as a count of those processes. Spelled in seconds it is a
> different budget in every build, and the build it was not measured in is
> the one where it accuses the code.

### Controls

- **The widened barrier can still refuse.** A guard never seen red is not a
  guard, so the gather was removed and the test re-run in both builds:
  `fire`'s prompt leg called synchronously instead of in a `go func`, which
  is a pass that fires and awaits, fires and awaits. Both arms FAIL, and
  the message is the one the bead reported:

  | build | prompt 0 released | test |
  |---|---|---|
  | `-race`, `fakeBarrierWait` = 80s | `"timeout"` after 1m20.002s | FAIL 153.35s |
  | plain, `fakeBarrierWait` = 10s | `"timeout"` after 10.001s | FAIL 11.24s |

  So the fix moved the deadline off the fixture's own launch cost without
  moving it off the invariant. `dispatch.go` was restored and the tree
  asserted clean before and after.
- **Green in the condition that broke it.** All three tests pass under
  `-race -tags posse_arm2`, run TOGETHER so their fake-herdr forks contend
  for the box (71.3s package clock: blink 7.47s, parallel pass 68.12s,
  create-stagger 69.82s).
- **And the whole arm is green.** ranger-base-nhc23 landed `make test-race`
  while this was in flight, which is the gate these three were the only
  reds in. MEASURED here at b6903aed + this fix, 2026-09-10: 107 of 107
  candidate tests, `arm 1 ok 273s · arm 2 ok 401s · arm 3 ok 78s`, **DATA
  RACE 0** in every arm, exit 0. `make test` green without `-race`.
