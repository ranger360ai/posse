# Two arms of a wall-watch kill head that no fixture could reach (ranger-base-0b6zo)

QA verify of four closes (ranger-base-6r6ox, ranger-base-x5zmk,
ranger-base-mklu0, ranger-base-6felj). Three of the four verified with nothing
to add; what landed here are the two pins that the verify of ranger-base-6r6ox
and ranger-base-6felj turned up, both of them arms whose mutant left the whole
of arm 1 green. Everything measured 2026-11-22 on the landed tip 81edb325
(linux 7.2.15-1-omarchy, go1.27.3, bwrap 0.13.1, 16 cores).

## 1. The reading of a verdict file is its LAST word, and two blocks cannot say so

`wallWatchVerdictSays` chooses among the three head constants by
`strings.LastIndex` (ranger-base-6felj). Both rows that pinned it were
TWO-block files — an alarm and the resolution that answers it, in each order —
and over two blocks a head's first occurrence and its last agree. MEASURED:
replacing `LastIndex` with `Index` left `./internal/posse ./cmd/posse` green,
exit 0, 0 FAIL lines, over 6m0s. No arm-2 or arm-3 file in internal/posse
reads that function, `WallWatchVerdictsSay` or any of the three heads, and
internal/treepins names none of them, so those two packages are the whole
suite's answer for that arm.

The readings part company at THREE blocks, which is the shape the file takes
when a seat is condemned, retried, found over and condemned again:
`appendWallWatchVerdict` appends, and `wallWatchRetryKill` clears the record's
retry mark rather than the file. Under `Index` such a file reads KILLED while
its last word is a kill that did not confirm — ranger-base-oe884's defect with
the sign flipped back, which is what ranger-base-6felj was filed on. Two rows
and one summary assertion in
`TestWallWatchVerdictSaysTakesTheLastWordOnTheKill` now drive it, composed
from the writers' own constants; the `Index` mutant reds them.

## 2. Two of the four render sites take their head from a call nothing graded

ranger-base-6r6ox's finding 2 was that the NOT-CONFIRMED arm of a wall-watch
kill was driven by no fixture at any of its four render sites. Its close drove
two — the backstop's detach and orphan-kill lines — and said plainly that the
watcher's own two in `wallwatch_linux.go` were not fixture-driven, a failing
kill needing a seat that survives twenty rounds of SIGKILL: "they call the
same pinned helper". The helper is driven. The CALL was not, and a site that
stops calling it is the shape the bead was filed on: MEASURED, replacing
either call with `wallWatchKilledHead + <seat>` — the pre-fix spelling, which
renders byte-identically whenever the kill confirms, so no live seat can tell
them apart either — left the whole of arm 1 green, exit 0, 0 FAIL lines.

So that arm is held by a rule over the source instead of a fixture.
`TestQAEveryWallWatchKillHeadIsReadInItsTwoHomesAndNowhereElse` parses this
package's 220 non-test files and requires each head constant to be read in its
one writer and its one reader and nowhere else, and its text to open no string
literal of its own. Both mutants above red it by name.

A PREFIX, AND IT IS ANCHORED ON WHAT THE DEFECT ACTUALLY SPELLED. The
pre-ranger-base-6r6ox lines at both sites were
`v.say("KILLED seat %s: the wall over %s came off…", d.Seat, …)` and
`refuse("KILLED seat %s: its bwrap (pid %d) ended…", r.Opts.Seat, …)` — a
literal with the head at its front — so the prefix arm reads the shape the
regression took, at both of the sites that took it, and the constant arm reads
the other route a new site has (naming the constant and skipping the helper).
What it cannot see: a line that reports a kill in words of its own, carrying
none of the three head phrases at all. Nothing can — the heads are the only
thing a reader of a verdict file has to go on, which is why they are constants.

A PREFIX and not a `Contains`, which the wide form taught: `wallWatchTitle`
carries the not-confirmed head's words mid-sentence
("security: wall-watch could NOT confirm the kill of seat %s"), which is a
second spelling of the same claim but not of a HEAD — a head opens the line it
reports, and no verdict file holds a title.
`TestQATheWallWatchKillHeadRuleSaysNo` drives both halves of the key over
planted source with a control that must report nothing, and a renamed home
reds the census's own vacuity arm rather than passing quietly.

## What this did not land

The three suite-lock arms of ranger-base-x5zmk, both findings of
ranger-base-mklu0 and the six rows of ranger-base-6r6ox's own fix all kill
un-combined; the readings are on the bead. One finding of ranger-base-mklu0's
close is a decision between a census rule and its key and went to the code
lane, and two findings are in other repositories — see ranger-base-0b6zo.
