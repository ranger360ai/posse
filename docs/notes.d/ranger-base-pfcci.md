# wall-watch's verdict is a file now, not only a pane line (ranger-base-pfcci)

ranger-base-pfcci, 2026-11-15. Discovered from ranger-base-1l1en: every seat's
launch line is exec'd, so the watcher IS the pane's process and the block
closes when the watcher returns.

## What was actually lost

`posse wall-watch` says three lines on its respond path
(`internal/posse/wallwatch_linux.go`), all of them to `r.Stderr`, which is the
seat's terminal:

```
KILLED seat <s>: the wall over <paths> came off at <t> (ranger-base-0qpsw D2)
filed <id>
⚠ the record is incomplete: <err> — this line is the record
```

MEASURED 2026-11-15 on this box (Linux 7.2.15-1-omarchy, rex
0.0.1794069975-g6c6689, scratch server): once the launch line is exec'd, a
`rex capture` of the watcher's block after it returns answers
`block target "<id>" not found`. The scrollback is gone that instant.

On the happy path nothing was lost — the filed bead is the record and it is
durable. The loss is the DOUBLE failure the third line was written for: the
wall came off AND bd could not be written, where the pane held the only
remaining copy of the reason. Before ranger-base-1l1en the pane stayed open
(ADR 0080 D9's "runtime exits, shell stays" row) and an operator could read
it.

## Where the copy lives, and why there

`<state>/gates/<persona>/wall-watch/<watcher pid>.verdict`, beside the
backstop record `<watcher pid>.json` that the same watcher already writes
(ranger-base-zd4d4, ADR 0qpsw D3).

The suffix is the whole trick. The backstop globs `*.json`
(`wallWatchRecordGlob`) and removes a record on a clean return and after a
kill; the watcher's own `defer os.Remove(record)` takes the same path. A
`.verdict` is matched by neither, so nothing that cleans up after a seat
removes it. **Nothing removes it at all** — a detach is a P1 security event,
and the operator who reads it is the one who takes it.

Two things that are not obvious:

- The verdict path is settled BEFORE the launch baseline, not beside the
  record write. A wall that is already off at launch responds before the
  record exists, and that response is exactly the one an operator has least
  other evidence for (the seat's command never ran).
- A render that is not a live launch writes its mounts file somewhere other
  than the gates dir (`renderBwrapWall`'s `dest`), so a probe's watcher
  writes its verdict into that temp dir and the readers never glob it. That
  is the right answer: the readers report what this instance's SEATS did.

## The backstop's two arms are not symmetric

The kill arm runs once per seat — after it the seat is dead — so it appends
like the watcher does. The dead-watcher arm (`walls intact, watcher gone,
bead could not be filed`) runs on EVERY tick for as long as the store is
down, so a plain append would turn a record into a log. It writes
`<pid>.orphan.verdict` with `O_EXCL` instead: first tick wins, later ticks
no-op, one line per seat however long the outage.

## Reading it back

- `posse gates <persona>` prints that persona's verdicts in full, after
  `refusals.log`. Not capped the way refusals.log is (a refusal is routine; a
  detach is not), and deliberately NOT part of that command's exit code — a
  detach that happened is history, and the exit says whether the persona is
  cleared to dispatch now.
- `posse peek <name>` appends a notice when it cannot read the pane it was
  asked for — the two failures an operator meets after a kill are
  `no such session` and `block target not found`. With a persona in hand (the
  session resolved, the pane did not) it prints the newest verdict in full;
  with none it prints the count and the path and no more, because peek is
  also what you type at a session name you mistyped, and a stranger persona's
  old detach must not bury that error.
