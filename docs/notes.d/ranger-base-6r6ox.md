# A kill that did not confirm no longer reports KILLED, and a fixture finally drives it (ranger-base-6r6ox)

*2026-11-22 · dinesh · two findings from ranger-base-evq89's verify*

## Finding 2 (this repo): wall-watch's NOT CONFIRMED arm

`wallWatchKilled` has rendered `NOT CONFIRMED — <err>` for a kill that did
not confirm since ranger-base-0qpsw D2, but no fixture could reach it:
`fakeWallWatchProbe.Kill` returned every pid and an empty error
unconditionally. Driving it showed what the bead predicted — the body said
NOT CONFIRMED while the TITLE and the verdict line's head said KILLED, so the
one P1 that most needs a human to end a seat by hand told its reader, in
`bd list` and in the pane, that the seat was already ended.

What changed (`internal/posse/wallwatch.go`, `wallwatch_linux.go`):

- `wallWatchKillHead(d)` opens every verdict line that reports a kill:
  `KILLED seat X`, or `could NOT confirm the kill of seat X` when `KillErr`
  is set. Used by the backstop's detach and orphan-kill lines and by the
  watcher's own two lines (the detach respond line, and ranger-base-6ec6n's
  leader-outlived-bwrap line). On an unconfirmed kill the detach lines add a
  `killed:` line, since they did not carry one.
- `wallWatchKillSentence(d, who)` is the bead description's sentence: "<who>
  killed the seat." or "<who> TRIED to kill the seat and could NOT confirm it
  ended — it may still be running, so end it by hand".
- `wallWatchTitle` and `wallWatchOrphanKillTitle` (now taking the detach)
  turn on `KillErr` the same way; `wallWatchOrphanKillComment` likewise.
- A kill that found NOTHING left is a seat that is over: it still reads as
  killed, and its body says "nothing was left to kill". That arm was equally
  undriven and has its own row now.
- `wallWatchOrphanKillMarker` keeps its one spelling ("seat KILLED") on
  purpose: it is the class's search key, and an unconfirmed kill is the one
  most worth finding by that search. A row pins that it survives.

The control is `fakeWallWatchProbe.killErr` / `killNothing`. Six rows:
detach bead + verdict, detach comment (`wallWatchComment`), orphan-kill
bead + verdict, orphan-kill comment on an already-filed dead-watcher bead,
and the found-nothing arm for detach and orphan. MEASURED 2026-11-22 (linux,
this worktree): seven mutants — each of the five KillErr branches set to
`false`, the NOT CONFIRMED return replaced, the nothing-left return replaced —
each red at least one of the new rows.

Not driven by a fixture: the watcher's own two lines in
`wallwatch_linux.go` reach the helper only from a live seat whose kill
fails; they call the same pinned helper and nothing else differs.

Filed, not fixed here: ranger-base-kzk3u, ranger-base-oe884.

## Finding 1 (another repository)

Finding 1 is in another repository and is tracked as ranger-base-es4ai.
