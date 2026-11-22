# A verdict file's reading is its LAST word on the kill (ranger-base-6felj)

*2026-11-22 · jian-yang · discovered-from ranger-base-repzh*

## The defect

ranger-base-oe884 made the one line `posse gates <persona>` and `posse peek`
put over a persona's verdicts say what those verdicts SAY, instead of
asserting a kill over all of them. It did that with `strings.Contains` over
the whole file for two head constants, and resolved a file matching both to
the unconfirmed reading on purpose — "the louder reading wins ... a seat that
may still be running is what the reader has to act on". That rule was written
for a file holding two INDEPENDENT events: a refusal and then a detach,
neither of which answers the other.

ranger-base-kzk3u, landing beside it, added a third block type that ANSWERS
the second. A kill that does not confirm now keeps the seat's record, and
every later tick retries it; the tick that finds the seat over appends its
verdict — `seat <s> is over — …` — to the same file that holds the `could NOT
confirm the kill of seat <s>` head it resolves. That block carried neither
head, so the reader could not see it at all: the summary went on saying

> a wall came off and the kill of the seat could NOT be confirmed
> (ranger-base-0qpsw D2) — it may still be RUNNING, so end it by hand

for the life of the file, several ticks after the tick had confirmed the seat
was over and told its beads so. Same defect oe884 was filed to fix, sign
flipped: oe884's head asserted an action was done that was not, and this one
asks for an action that is already done. Either way the only way to find out
is to read the whole body the head exists to summarize — and here the body's
own last line says the seat is over.

## What changed

internal/posse/wallwatch.go, two things:

- **`wallWatchOverHead`**, a third constant beside `wallWatchKilledHead` and
  `wallWatchNotConfirmedHead`, written by `wallWatchRetryKill`'s two arms
  (the retry's own kill confirmed; the seat's mount namespace is empty). A
  resolution has to be as findable as the alarm it resolves. Arm 1's prose is
  unchanged; arm 2 now reads `… is over — the kill an earlier tick could not
  confirm (<reason>) was retried, and this one did: killed <pids>`.
- **`wallWatchVerdictSays` chooses by LAST occurrence**, not by `Contains`,
  and returns one `wallWatchReading` of three instead of two independent
  bools. `appendWallWatchVerdict` appends, so a file's blocks are in the order
  they happened and its last head is the current state of that seat's kill.
  The over head reads as `wallWatchReadsKilled` — the same reading
  `wallWatchKillHead` gives a kill that found nothing left, for the same
  reason: a seat that is over is over.

The "louder reading wins" case survives as the thing it was written for — a
refusal carries no head at all, so a refusal-then-detach file still reads as
the detach — and the case it was never written for is gone. Nothing else
moved: no caller, no count, no path, no exit, and the two readers take the
same clause they took before.

## What drives it

Three rows through the real tick
(`TestWallWatchVerdictsSaySeesTheRetryThatConfirmedTheSeatIsOver`), the
bead's repro: the detach arm whose retry kills, the orphan arm whose seat
ends between ticks, and a CONTROL of one tick with nothing retried, which
must still say "may still be RUNNING". Round trip in oe884's sense — the tick
writes the file, `readWallWatchVerdicts` reads it back, the summary says what
it found — so a head the writer stops writing reds here.

Order is the other half, and `Contains` is what made it invisible:
`TestWallWatchVerdictSaysTakesTheLastWordOnTheKill` puts the two blocks in
BOTH orders, composed from the writers' own constants. Alarm then resolution
reads killed; resolution then a FRESH alarm reads unconfirmed, because that
is a new unconfirmed kill and not a stale one.

MEASURED 2026-11-22 (linux 7.2.15-1-omarchy, go1.27.3, 16 cores, this
worktree, `go test -count=1 ./internal/posse`):

| mutant | red |
|---|---|
| the reader back to the old rule (`Contains` of the not-confirmed head wins) | both new pins; the detach and orphan rows, control green |
| the writer drops the head from the retry's confirming arm | the detach row |

The second mutant is the pre-fix code exactly: the retry's block with no head
in it. The control staying green under the first is what says the rows are
not measuring a summary that never says "may still be RUNNING".
