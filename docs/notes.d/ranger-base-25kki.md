# The landed check's parallel body and its made tail (ranger-base-25kki)

ADR 0088's amendment "the body runs four doors at a time" (D6.7,
ranger-base-hzh0w) decided the shape; this is the build of it, measured.
`scripts/posse-landed-check` now runs the derived goals under one
`make -j4 --output-sync=target` with no `-k`, and on a red re-runs the FIRST
failed door alone, last, so that the last lines of combined output are that
door's own complaint rather than a green sibling's.

Dated 2026-11-22, on the D1 host (linux 7.2.15-1-omarchy, 16 cores, GNU Make
4.4.1), from this bead's worktree, judging that worktree — `main` at
`81edb325` plus this bead's two files.

## The wall, uncaged, through the script

| run | cache | loadavg before → after | wall |
|---|---|---|---|
| 1 | warm (the box's own `GOCACHE`) | 5.66 → 8.98 | **105s green** |
| 2 | fresh private `GOCACHE`, as D3.2 gives the judge | 9.64 → 11.30 | **124s green** |
| 3 | warm, quieter box | 4.40 → — | **92s green** |
| 4 | warm, over the finished tree | 2.35 → — | **107s green** |

One suite slot was held by a sibling seat throughout runs 1 and 2. Eleven
prerequisites, four at a time — and the wall is the longest door plus
contention, exactly as the amendment's growth law says: `treepins-check`
alone is 82.5s of run 1 and 93.8s of run 4, so the whole judgement costs
between 11 and 25 seconds more than its slowest single door. That also says
where the next over-ceiling bead comes from, and it is not this runner. The amendment's own serial and `-j4`/`-j8` readings stay where it
said them and are not copied here; what these three add is the same shape
through the SCRIPT that production execs rather than over the derived goals by
hand. All three land below D6.7's ASSUMED 130-150s production band, uncaged
and with the box in use; what is between them and that band is the cage, and
the verify bead reads that off the dispatch log.

## The tail is made, and what that cost to pin

Under `-j` D1's "the last lines are the cause" stops being free. MEASURED over
a planted Makefile: make prints the dead door's error line, then `Waiting for
unfinished jobs....`, then every already-running green door's output, last. So
the check reads the failed doors back out of make's own
`*** [Makefile:N: <door>] Error` lines and runs the first one again alone.

Three things that only turned up in the building:

- **The pin's first draft was satisfied by the script's own narration.** The
  check says `red under -j4 … re-running <door> alone` BEFORE the second make,
  and that line names the door — so an arm asserting "the last line names the
  dead door" passed with the whole re-run deleted. The arm asserts make's own
  complaint now (`*** [` and the door, plus the door's own output inside the
  three lines the runner keeps), which is what D6.7 actually promises.
- **`--output-sync=target` is held as text and not as behaviour.** Removing it
  reds only the four plan arms. What it buys is each door's output in one
  block instead of four interleaved: legibility, not the verdict. Said in the
  pin so the next reader does not take it for a behavioural pin.
- **A red from which no door can be parsed is a live shape, not just a broken
  parser.** A goal with no rule prints `No rule to make target` and no
  bracketed error line at all, so the parser now sits between the body's red
  and the verdict — a place a green could come back from. That arm exists, and
  the script's refusal names the output above it rather than blaming itself.
- **The parser's one way to produce a WRONG finding is an ignored error.** A
  recipe line prefixed `-` whose command fails is reported `make:
  [Makefile:N: door] Error 1 (ignored)` and the door did not fail (MEASURED).
  Read as the cause, it re-runs a door that passed and reports it as "red
  under -j4 and green alone" — a finding on a bead about a door that is fine,
  which is worse than missing one. Note the line carries no `***`, so the
  `***` already in the pattern excludes it; the `(ignored)` clause beside it
  is belt-and-braces, and the mutation table below prices both.

`--output-sync` needs a writable TMPDIR. The judge is given one inside its own
run dir (`landedRunEnv`), and where it is NOT writable make says `cannot open
output-sync lock file, suppressing output-sync` and carries on interleaved
(MEASURED): every door still runs and the error lines the re-run is chosen
from are untouched, so the degradation costs legibility and never the verdict.

## Mutations

MEASURED 2026-11-22 by planting one line at a time in the script and running
`-run LandedCheck` over arm 2 (18 arms, 2.04s). Every mutation reds, and the
column says which arms:

| mutation | reds |
|---|---|
| drop the serial re-run from the red path | the tail arm, and the green-alone arm |
| exit with the re-run's own status (serial green reads green) | the green-alone arm |
| put the body back to a serial `make`, re-run kept | the four plan arms, and the tail arm at its fixture guard |
| `-j4` with no `--output-sync` | the four plan arms |
| drop the `MAKEFLAGS`/`MAKELEVEL` scrub | the three environment arms |
| a red that names no door exits 0 | the no-door arm |
| drop EITHER ignored-error guard alone | nothing — the other still excludes the line |
| drop BOTH ignored-error guards | the ignored-error arm, and nothing else |

The serial mutation reds the tail arm at a `t.Fatalf` and not at its claim,
and that is the honest reading: with no `-j` the slow green sibling never runs
at all, so the shape that arm is about does not exist to assert. The pair of
ignored-error guards is the one row where a single mutation survives, and the
arm is written to pin the PAIR rather than either, which is what that row
says out loud instead of leaving it to be rediscovered.

## What the parallel body changed about two old arms

Both are kept, with the claim the new shape supports.
`…StopsAtTheFirstRedAndTheTailNamesIt` said "a door after the failing one must
not run"; under `-j4` three siblings are already running when the first door
dies, so what survives of fail-fast is that make STARTS nothing further — the
five doors behind the first batch of four leave no witness.
`…IgnoresAnInheritedKeepGoing` asserted the tail turned on `-k`; it no longer
does, because the tail is made. What an inherited `-k` costs now is the body's
own fail-fast: every remaining door would run, for no verdict, on a box the
judge shares with the seats it exists to protect.
