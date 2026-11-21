# ranger-base-a688i — a wall-pin member's fatal has ONE line to say why, and this one spent it on the symptom

`wall pins` was red once in ADR 0097 D1's venue, at tree `c56c697c9247`
(commit `09732acfd290`), and green again at `e3aeefb3c4c4` (`8ee960e5140f`)
one landing later. It is not reproducible and its cause is not recoverable
from any record this shop keeps. That second sentence is the finding, and the
fix is to the instrument, not to the wall.

## What the red said, in full

    FAIL TestLiveWallWatchKillsASeatWhoseWallComesOffAndFilesOneBead:
    wallwatchlive_test.go:275: the seat ended before it ran (exit 1):

That is the whole of it. Line 275 is stage 3 of that member — the launch of
`seat-c`, the arm that re-detaches the deny wall inside ten minutes — and the
message comes from `launchOn`'s wait loop, which fataled because
`runWallWatch` returned before the seat's shell wrote its background pid.

`exit 1` from that function has four causes, and nothing above distinguishes
them:

1. a wall MISSING AT LAUNCH — `respond(missing, true)`, which also files a
   bead and writes a `KILLED seat …` verdict;
2. one of the watcher's own refusals — the seat not finishing setup inside
   `wallWatchSeatReadyWait`, an unreadable `/proc/<pid>/mountinfo`, or a
   backstop record it could not write, each of which kills the seat and says
   `killing the seat rather than running it unwatched`;
3. a bwrap that died before it reported a child, where the 1 is bwrap's own
   status and bwrap's own stderr is the only account of it;
4. a `cmd.Wait()` error that is not an `*exec.ExitError`.

All four write their words to the buffer the fixture had captured — and the
fixture put them on line 2.

## Why line 2 is nowhere

`scripts/posse-wall-check` reports a failed member by `evidence()`: the first
`file.go:NN:` line of that test's output that is not the gate marker,
truncated to 300 characters, and **nothing after it** (ADR 0097 D3). The
launcher then puts that one line on the bead. Both halves are working as
designed — a per-member tail would drown the verdict the check exists to give
— so the budget is one line per member, and it is the MEMBER's job to spend
it on a cause.

This member spent it on `the seat ended before it ran (exit 1):`, a symptom
with a colon where the reason should be. `~/homes/ranger/state/dispatch-watch.log`
holds the same truncated line and nothing more; the venue's run dir, with the
durable verdict file that would have named the cause outright, is cut fresh
per run and was gone. So the P1 arrived unanswerable, and no amount of care
at this end would have changed that.

## What changed

`wallwatchlive_test.go` only. The two fatals in `launchOn` carry what the
watcher and its bwrap said, folded onto their own first line by `wwOneLine`
(`strings.Fields` + `strconv.Quote`, so the reader also sees where the
message ends), with a said-NOTHING case that reads as a finding rather than
as empty quotes — because cause 3 above is the one that leaves no words, and
an absence has to be legible as one.

The captured buffer became `wwSaid`, a mutex around a `strings.Builder`,
because one of the two readers is the DEADLINE branch: it fires with the
watcher's goroutine still writing, so the `strings.Builder` shared across the
two was a race. The other branch reads after a receive on `code` and was
always safe. Nothing in the suite runs this package with `-race`, so the race
was real and silent; `go test -race -run TestLiveWallWatch` is green now.

`TestQAWallWatchFixtureFoldsWhatTheWatcherSaidOntoOneLine` pins the fold.
It needs no bwrap and no venue, and it is deliberately not named like a
member (`^Test(QA)?Live(Bwrap|Wall)`): it measures no wall.

## MEASURED 2026-11-21 (linux 7.2.15, bwrap 0.13.1, 16 cores, load 5-9)

- the member alone, venue-shaped `TMPDIR` under `$HOME`, `-count=100`:
  100 PASS, 0 FAIL, ~0.3s per run
- the whole member set (`^Test(QA)?Live(Bwrap|Wall)`) uncaged with that
  `TMPDIR`, 3 rounds: rc=0 at 89s, 84s, 84s
- `bash docs/adr/0097-wall-venue.probe.sh venue+check`: rc=0,
  `wall check: green, 47 pins measured`
- `go test -race -run 'TestQAWallWatchFixtureFolds…|TestLiveWallWatch'`: ok

So the red is not reproducible by repetition, by the member set, or in the
venue, and this fix does not claim to have closed it. What it closes is the
reason nobody could tell: the next occurrence names its cause on the line
that reaches the bead.

## The class, for whoever reads the next one of these

A wall-pin member's failure message gets ONE line. A first line that names a
symptom and defers the reason to a captured buffer is a first line that
reaches the bead empty. The shape to look for is a format string ending in
`:\n%s` where the `%s` is the only account of WHY — as distinct from the many
in this package whose first line is already a claim and whose line 2 is
supporting output, which are fine. `wwWaitKilled`'s `watcher exit = %d, want 1`
is the nearest remaining instance in this same fixture: an exit of 2 there is
a refusal whose words are in the same buffer, and the helper does not take
it. Filed as ranger-base-w6kke, not fixed here — it needs a helper signature
and a second file, and this bead's scope was the two fatals whose first line
named no cause at all.
