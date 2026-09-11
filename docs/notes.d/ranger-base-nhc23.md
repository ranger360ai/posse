## The `-race` arm is a target now, not a recipe (ranger-base-nhc23)

2026-09-10. Findings bundle from ranger-base-7npp8's verify of
ranger-base-d0xvw. Four findings; what follows is the one that was code.

### What was live

1. the priced arm ran 20 of the 106 tests it names — RECORD, corrected in
   `docs/notes.d/ranger-base-d0xvw.md`'s CORRECTION section (laurie, b8feb943).
2. **the deliverable is a RECIPE, and the recipe runs 19% of the arm it
   describes** — this bead.
3. the (c) arithmetic rests on the wrong price and the wrong KIND of price —
   a re-decision, handed off.
4. three arm-2 dispatch tests fail under `-race` — already filed as
   ranger-base-0dt50.

### The fix: `make test-race`

`scripts/test-race.sh`, and it is three commands because `internal/posse` is
three build-tag binaries and the candidate set straddles all three. Two things
it does that no hand-typed recipe can:

- **the names come from the FILES, every run.** The eight candidate files are
  the list a person maintains — "concurrency-carrying" is a judgement about
  `dispatch.go`'s gather fan-in, `passcarry.go`'s fan-out and wake channel, and
  `watch.go`'s pulse/backup/guard/watchdog goroutines, not something to infer
  from a test's name. The 107 test names inside them are read fresh. A test
  added to `dispatch_qa_test.go` is in the arm the day it lands, and no list of
  106 names ages in a document. (It aged in four days: 106 on 2026-09-07, 107
  on 2026-09-10 — `watch_test.go` gained one.)
- **it censuses before it spends a minute.** Every candidate name must be
  compiled into at least one of the three arms; a name compiled into none
  fails in ~8 seconds, naming the test. That is the trap that ate d0xvw and
  ate ranger-base-y3x6n's recorded repro before it — rot #4 in
  `armtags_qa_test.go`'s own header, "a `go test -run` that matches no test
  exits 0" — and the run itself carries the same check a second time, because
  `[no tests to run]` is also a zero exit.

MEASURED 2026-09-10 by `scripts/test-race.sh --census`, HEAD e4341e56:
**107 candidate tests — arm 1 (default) 21, `-tags posse_arm2` 76,
`-tags posse_arm3` 28**, every one of them reachable. (Laurie's 20/75/27 three
days earlier, plus the one test `watch_test.go` gained; the shared files are
counted in each arm they compile into.)

### Pinned, because nothing runs it

`internal/treepins/racearm_qa_test.go`. An on-demand instrument nobody's gate
executes is the state in which an instrument goes quiet — `armtags_qa_test.go`'s
rot #2, one level out — so the suite holds the two halves that can rot while
the arm sits unused: the target still exists and still runs the script (arm 1),
and the census still reaches every candidate test in the tree as it stands
(arm 2). Arm 3 is the control, because arm 2 over a blind census is a green
that measures nothing — the same defect one level in.

MUTATION-CHECKED: renaming the `test-race:` target reds arm 1 alone; taking
the `missing` verdict out of the script's census reds arm 3 alone, over a
planted candidate file whose tests are in no arm at all.

### What it costs, and why it is not in `make test`

Inherited from the CORRECTION, MEASURED 2026-09-10 on darwin/arm64, go1.26.5,
`-count=1`: 51.4s without `-race`, 670.9s with — ~13x, ~11.4 min in three
commands, at 13-14% CPU. The cost is WAIT, not the detector's per-access tax:
fixed intervals, backstops and barrier timeouts stretching under the `-race`
build. So it is typed by hand when a change touches dispatch/passcarry/watch or
a race is suspected, it takes no suite slot (a `-run` filter is not queued,
ranger-base-uvzjk), and it is a prerequisite of nothing.

VERIFIED end to end 2026-09-10 at this bead's HEAD, `make test-race`, on a
box carrying other seats (load average 24→74, so the wall figures are upper
bounds):

| arm | tests | wall | verdict | `DATA RACE` |
|---|---|---|---|---|
| 1 (default) | 21 | 278s | `ok` | 0 |
| 2 (`-tags posse_arm2`) | 76 | 418s | FAIL — ranger-base-0dt50's three, and nothing else | 0 |
| 3 (`-tags posse_arm3`) | 28 | 96s | `ok` | 0 |

792s, exit 1. The three reds are `TestDispatchParallelPass`,
`TestDispatchParallelPassGathersDespiteCreateStagger` and
`TestStatusAfterTimeoutRidesOutABlink` — the same three ranger-base-khhnd's
after-column names, which is the arm reproducing a filed bug rather than
finding a new one.

**It is not green today, and the script does not hide that**: three arm-2
dispatch tests fail under `-race` and pass without it, `DATA RACE` 0
(ranger-base-0dt50). Arms 1 and 3 are `ok` as of ranger-base-khhnd, which
landed the same day and took the watch-loop backstop off its 30s constant and
onto the binary's own deadline. A red is a result; the summary names the log.

### What this bead did NOT do

Finding 3's re-decision. The corrected price does not overturn the SHAPE of
d0xvw's answer — a narrow on-demand arm is still the proposal, and this is not
a reopen (ADR 0006 §2) — but the reasoning that rejected (c), a periodic
full-package arm, extrapolated a per-access CPU tax package-wide, and the
measurement says this arm's cost is wall clock. Redoing that arithmetic needs a
number nobody has: what `internal/posse` whole costs under `-race`, on a quiet
box. That is a design call with an experiment in front of it, not a script.
