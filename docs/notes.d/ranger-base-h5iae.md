# ranger-base-h5iae — the wall-check self-test says which exit each arm asserted

`internal/treepins/wallcheckselftest_qa_test.go`'s arm 2 claimed to hold ADR
0097 D3's three exits. What it read was three LABEL PREFIXES — `green:`,
`red:`, `abstention:` — out of `scripts/posse-wall-check --self-test`'s `ok`
lines. A label is prose an author types in an `st_arm` call; the exit that arm
asserts is `st_arm`'s third positional argument and it reached the output
nowhere. So exit 3 could leave the contract entirely with the pin, the
self-test and `make verify-wall-check` all green. Found by the verify of
ranger-base-zpx1k (ranger-base-l50hz, FINDING 1).

The `ok` line carries the asserted exit in brackets now, printed from the same
`$want` `st_arm` compares the parser's answer against:

    ok    [3] abstention: a member SKIPped after the marker

and the pin parses that field instead of the label. The summary line is the
second half of the same hole and is derived the same way: it printed the
literal `(ADR 0097 D3 exits 0/1/3, D5 abstention)` whatever the arms drove, so
it stated the contract's range in prose while proving nothing about it, and
said it loudest in the one case it was wrong.

2026-11-14.

## What landed

| file | what |
| --- | --- |
| `scripts/posse-wall-check` | `st_arm` prints `[$want]` and accumulates `st_exits`; the summary line derives its range from the arms that PASSED; the mandated-sentence arm asserts its exit instead of discarding it with `\|\| true` |
| `internal/treepins/wallcheckselftest_qa_test.go` | `wcsOKLine`/`wcsPassing`/`wcsRange`/`wcsExitFindings` + `wcsFamilies`, arm 2's body, arm 5, the head comment |

Three things the pin asks of a run now, where it used to ask one:

1. every label in `wcsArms` is still proved by a passing arm, which is the
   half that already worked — `MUTANT B` below is its control;
2. each of D3's three exits is asserted by a passing arm, read off the
   bracket;
3. the label's family and the asserted exit agree. `wcsFamilies` maps
   `green:`→0, `red:`→1, `abstention:`→3, and an unknown family is passed
   over on purpose: a fourth exit on D3's table should arrive as a fourth
   family without this arm being the thing that refuses it.

The mandated-sentence arm asserts exit 0 now because it has to: the bracket it
prints has to mean what every other arm's means, and the mandated sentence
exists only on the green path, so an arm claiming `[0]` over a verdict the
parser answered 3 for would be reading the last line of the wrong run.

## Why not a pin that re-derives the wants from the shell source

It would be a second implementation of `st_arm`'s argument order, kept in sync
by hand, going quietly narrower than the script while both looked green. The
bracket costs one `printf` and makes the fact mechanical: an arm can print
`[3]` only by having asked the parser for 3 and been answered 3.

## Arm 5, and why the fix needed one

A closed-set check that has never been seen to red in the direction it exists
for is a comment — ranger-base-ietka is this repo's name for that, and arm 2
was an instance of it for its whole life. So arm 5 keeps the bead's own mutant:
drop the parser's two `sys.exit(3)` to 0, drop the four `st_arm` wants that
asked for 3 to match, and require the mutated self-test to stay GREEN, every
label to still be proved, the floor to still be met — and `wcsExitFindings`,
the exact function arm 2 runs, to complain, and to complain about exit 3.

MEASURED 2026-11-14, D1 host, Linux 7.2.15-1-omarchy, go1.27.3, 16 cores, warm
build cache, each mutant planted in the tracked file alone and restored by
digest afterwards. Each row is the self-test's own exit, then the pins:

| mutant | `--self-test` | the pins |
| --- | --- | --- |
| **A** — `sys.exit(3)`→0 in the parser and the four wants of 3→0 (ranger-base-l50hz's MUTANT A) | 10 arms clean, exit 0, summary now reads `exits 0/1` | arm 2 RED: *no passing arm asserted exit 3*, plus four label/exit disagreements; arm 5 fatals on its own fence, the mutant having already been applied |
| **B** — one label renamed, parser untouched (the positive control) | 10 arms clean, exit 0 | arm 2 RED on the label list; arm 5 RED saying it is measuring the label list and not the exit field |
| **C** — the bracket removed from the `ok` line, wants untouched | 10 arms clean, exit 0 | arm 2 RED on every label and the floor; arm 5 RED |

Before this landed, MUTANT A was `--self-test` 10 arms clean, exit 0, and the
pin PASSING in 0.25s at 61ec26f1. That is the escape, and it is the reading
the table's first row replaces.

## What it costs

MEASURED 2026-11-14, same box and cache. The five arms as one package run:
0.196s, 0.217s, 0.263s at a one-minute load average of 2-5, and 0.500s,
0.554s, 0.527s at a load of 21 with this bead's own `make test` on the box.
Arm 5 is 0.17-0.20s of that, the same as arm 3, which plants and runs the
script the same way. `--self-test` itself is 0.40s a run over five runs at a
load of 24 — one `printf` conversion and a ten-iteration shell loop more than
it was — so `make verify-wall-check` is unchanged in kind and remains a `make
test` prerequisite.

Neither arm pays the cold-exec cost ADR 0085 prices: `make coldexec-check
RUN='^TestQATheWallCheckSelfTest(CanFail|ExitFieldIsLoadBearing|StillReadsTheThreeExits)$'`
is green with no `--warm` pre-flight, because the plant is read by `bash` and
never exec'd as a file of its own.

## Not closed here

- ranger-base-np54s (P1) — found by this bead's own `make test` and nothing to
  do with its change: `internal/posse/suspend_qa_test.go:598` polls
  `dispatcherOut(d)` — a bare `strings.Builder` — while `d.watchdogLoop` writes
  the SUSPENDED line from a goroutine, so arm 3 can die on `panic: unsafe.
  String: ptr is nil and len is not zero` and take the whole package run with
  it. `go test -race -tags posse_arm3 -count=5 -run
  '^TestQASuspendWitnessRunsWithNoBudgetAndIsSeededByWatch$'` names the race on
  the first iteration; run alone and unloaded the test passes in 0.034s, which
  is the expensive half. Arms 1 and 2 were green in the same run and arm 3
  re-ran green at 102.0s.

- ranger-base-a7sqc — ranger-base-l50hz's FINDING 2, an ADR 0098 D2 reading
  that is architecture's to make and not a verifier's or a developer's: the
  `twdTreepinsExcluded` row for `TestQATheRaceArmCensusRefusesTheWrongShapes`
  is priced at 2.85s against D2's 2.0s floor, and run alone it costs
  1.67-1.76s on this box. Both are honest readings of different quantities and
  the row records no run shape, so neither can be told from the other by a
  later reader. Whether the floor means the contended in-door cost or the
  test's cheapest price decides whether the row keeps a sentence or leaves the
  table — and the second answer changes what `make treepins-check` runs.
