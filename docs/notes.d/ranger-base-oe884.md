# The one line over a set of verdicts says what they say (ranger-base-oe884)

*2026-11-22 · dinesh · discovered-from ranger-base-6r6ox*

## The defect

A wall-watch verdict file is prose for the human who comes to a seat after
its pane is gone (ranger-base-pfcci), and the two readers of it put ONE line
over a persona's whole set:

- `posse gates <persona>` (cmd/posse/main.go): `wall-watch: N verdict(s) — a
  seat of this persona was KILLED for a wall that came off`;
- `posse peek` (`WallWatchVerdictNotice`): `A seat whose wall came off is
  KILLED and its pane closes with it`.

Both asserted that over every verdict on disk, whatever each one said. Since
ranger-base-6r6ox a kill that did not confirm opens its line with `could NOT
confirm the kill of seat X` and carries `killed: NOT CONFIRMED — …`, so the
head asserted the opposite of the body printed under it — on the one reading
whose whole point is that a human must go and end that seat by hand. Two
other readings were reported as kills too: a REFUSAL verdict
(ranger-base-r9slg) names no kill at all, nine of its lines ending the seat
before it ran or refusing to start it; and the `SAID AND NOT SKIPPED` arm's
`⚠ this verdict is on disk and could not be read`, where nothing is known
about a kill either way.

## What changed

- `WallWatchVerdictsSay(vs)` (internal/posse/wallwatch.go) is the clause both
  readers now put over a set: the three readings counted apart. One class
  present gets the unnumbered sentence, more than one gets counted clauses
  joined by `; `. A file holding BOTH readings — `appendWallWatchVerdict`
  appends, so one file holds every block its watcher wrote — counts as
  unconfirmed, because a seat that may still be running is what its reader
  has to act on.
- `wallWatchKillHead`'s two heads are constants (`wallWatchKilledHead`,
  `wallWatchNotConfirmedHead`), and the reader matches on them. One spelling
  for the writer and the reader: a renamed head cannot leave the reader
  quietly finding nothing.
- The gates head is now `wall-watch: N verdict(s) for this persona — <clause>.
  The pane that printed each of them is gone:`, and peek's two sentences take
  the same clause. Neither reader's count, path, body or exit changed.

**Fixed here beyond the bead's own sentence**: peek's notice, which is the
same assertion over the same set one surface over — both are named "the two
READERS of a wall-watch verdict" by the QA file that pins them
(cmd/posse/wallwatchverdict_qa_test.go). Said in the commit message rather
than left silent.

## What drives it

The internal rows are a ROUND TRIP, not a hand-written fixture: a backstop
tick that really detached writes the verdict, `readWallWatchVerdicts` reads
it back, and `WallWatchVerdictsSay` says what it found — so the key the
reader matches on is pinned where it is read and not only where it is
written. The control is ranger-base-6r6ox's `fakeWallWatchProbe.killErr` /
`killNothing`. The cmd rows plant the file and go through the built binary,
beside that file's other two pins.

MEASURED 2026-11-22 (linux, this worktree, `go test -count=1`): seven mutants,
each red by at least one row.

| mutant | red |
|---|---|
| an unconfirmed kill counts as a kill (`case nc:`) | 4 internal rows + the cmd row |
| the head the reader matches is renamed | `…SayOverReadingsNoTickWrites`, cmd row |
| a verdict reporting no kill is called a kill | `…SayClaimsNoKillOverARefusal`, `…OverReadingsNoTickWrites` |
| a mixed set drops its unconfirmed count | `…SayCountsAMixedSetApart`, cmd row |
| peek's unnarrowed arm asserts KILLED again | `…NoticeDoesNotSayKilledOverAnUnconfirmedKill` |
| peek's persona arm asserts KILLED again | the same row |
| the gates head asserts KILLED again | `TestQAGatesReportDoesNotSayKilledOverAKillThatDidNotConfirm` |

The rename mutant is the one worth reading twice: renaming the constant moves
the writer and the reader together, so the round trip stays green and what
reds is the row holding a verdict file written BEFORE the rename — which is
every file already on disk.

## Not closed here

- ranger-base-kzk3u (gwart) — the record of a seat whose kill did not confirm.
  It edits `wallWatchBackstop` in the same file; nothing here touches that
  function.
