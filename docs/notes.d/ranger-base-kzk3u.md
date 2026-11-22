# A kill the tick could not confirm keeps its seat's record, and is retried (ranger-base-kzk3u)

2026-11-22. `internal/posse/wallwatch.go`, `wallWatchBackstop`.

## What was wrong

Both of the backstop's kill arms dropped the seat's record once they had
acted: the detach arm when the watcher was dead, the ranger-base-5k7vf
orphan arm always. They read a killed seat as over. A kill that returned a
`KillErr` left a seat that was NOT over, and with the record gone no later
tick read its mount table or tried the kill again. The P1 bead was the only
trace left, and nothing came back to it.

## What it does now

- On a `KillErr`, either arm keeps the record and marks it:
  `kill_pending` (the failure) and `kill_beads` (the beads the kill was
  filed or recorded on). Each of those beads gets one comment saying the
  tick is retrying.
- A marked record takes its own arm on every later tick, ahead of the
  diff: nothing is re-diffed or re-filed. The seat is killed again, or
  found over (empty mount namespace).
- A retry that fails again prints one pane line and nothing else. No
  comment, because that would be one per tick.
- A retry that confirms, or a seat found over, writes one verdict line and
  one comment per bead. The record is dropped (dead watcher) or unmarked
  (live watcher, which owns it) only once every bead has been told. A bead
  the store refused stays in `kill_beads`, and the next tick tells it
  again.

The bead offered two options: retry and comment, or say on the bead that
the tick had stopped watching. This takes the first. The second would have
left a seat that had escaped a kill with nothing watching it.

## Verified

MEASURED 2026-11-22 on this box: four new tests in
`internal/posse/wallwatch_test.go`, `-run TestWallWatchBackstop` ok. Four
mutants each turn at least one of them red: the record dropped as before;
no retry arm; the record dropped with a bead untold; the live watcher's
mark not cleared.

## Overlap

ranger-base-6r6ox (unlanded at the time of writing) adds a `killErr` field
of the same shape to `fakeWallWatchProbe` and rewrites the KILLED wording
at the same render sites. The two merge by hand.
